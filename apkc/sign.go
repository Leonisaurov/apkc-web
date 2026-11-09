package main

// apkc sign: a package that already exists, put through the packager inside this
// binary.
//
// The signing half of a build was rewritten in Go — zipwriter.go, v1sign.go,
// apksig.go — and it is the one stage that starts no external program. That makes it
// reachable from a place where no program can start, which is the point of the wasm
// build: inside a build it stays locked behind javac, d8 and aapt2 agreeing first.
//
// As a command it is also what a person holding an apk from somewhere else needs: the
// same entries, byte for byte as they were carried, aligned as they go, under a
// v1+v2+v3 signature of their own. The signature files of whoever signed it before are
// dropped rather than kept, because two signers in one package is what a verifier
// reports as corrupt instead of as old.

import (
	"archive/zip"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func sign() {
	set := flag.NewFlagSet("sign", flag.ExitOnError)
	keyStore := set.String("keystore", keyStorePath, "path to the keystore")
	storePass := set.String("storepass", "android", "keystore password")
	keyAlias := set.String("keyalias", "androiddebugkey", "key alias to use")
	out := set.String("out", "", "where to write the signed package (default: over the input, as apksigner does); the switches go before the package, and -- ends them")
	v4 := set.Bool("v4", true, "write the .idsig sidecar of the v4 signature for incremental installs")
	lineage := set.String("lineage", "", "lineage file from apkc rotate: sign a key rotation (v1/v2 with the old key, v3/v4 with this one)")
	oldStore := set.String("old-keystore", "", "old keystore, the one that signs v1/v2 when --lineage is used")
	oldPass := set.String("old-storepass", "android", "password of the old keystore")
	oldAlias := set.String("old-keyalias", "androiddebugkey", "alias in the old keystore")
	rotMin := set.Int("rotation-min-sdk", rotacionMinSdkPorDef, "first API level that uses the new key in a rotation (33 = v3.1, <=32 = v3.0)")
	set.Parse(os.Args[2:])
	escribirV4 = *v4

	packages := set.Args()
	if len(packages) == 0 {
		LogF("sign", "apkc sign takes the package to sign: apkc sign app.apk",
			"[--keystore ... --storepass ... --keyalias ... --out ...]")
	}
	// a switch typed after the package is not a switch to flag.FlagSet, and the
	// silence about it is the problem: `apkc sign app.apk --out other.apk` would
	// otherwise sign app.apk in place and leave other.apk unwritten, saying nothing.
	// Saying so is cheaper than guessing, and the fix is one keystroke away.
	for _, p := range packages {
		if strings.HasPrefix(p, "-") && p != "-" {
			LogF("sign", p, "was read as a file name: the switches go before the package,",
				"as in apkc sign --out signed.apk app.apk (and -- ends them)")
		}
	}
	if len(packages) > 1 {
		LogF("sign", "apkc sign takes one package, and got another name after", packages[0]+":", packages[1:])
	}
	final := *out
	if final == "" {
		final = packages[0]
	}
	requireKeystore(*keyStore)

	var carried int
	var err error
	if *lineage != "" {
		if strings.HasSuffix(strings.ToLower(packages[0]), ".aab") {
			LogF("sign", "un .aab no lleva bloque v3/v3.1: --lineage es para apk")
		}
		if *oldStore == "" {
			LogF("sign", "--lineage necesita --old-keystore:",
				"la v1/v2 las firma la clave vieja, y la nueva va en --keystore")
		}
		blob, errR := os.ReadFile(*lineage)
		if errR != nil {
			LogF("sign", "lineage:", errR.Error())
		}
		inner, errR := innerLinaje(blob)
		if errR != nil {
			LogF("sign", errR.Error())
		}
		newSigner, newCert, errR := signingKey(*keyStore, *storePass, *keyAlias)
		if errR != nil {
			LogF("sign", "new keystore:", errR.Error())
		}
		newKey, errR := rsaKey(newSigner)
		if errR != nil {
			LogF("sign", errR.Error())
		}
		oldSigner, oldCert, errR := signingKey(*oldStore, *oldPass, *oldAlias)
		if errR != nil {
			LogF("sign", "old keystore:", errR.Error())
		}
		oldKey, errR := rsaKey(oldSigner)
		if errR != nil {
			LogF("sign", errR.Error())
		}
		rotacionActiva = &rotacion{nuevaLlave: newKey, nuevoCert: newCert, linaje: inner, minSdk: *rotMin}
		defer func() { rotacionActiva = nil }()
		carried, err = resignWithKey(packages[0], final, oldKey, oldCert)
	} else {
		carried, err = resignPackage(packages[0], final, *keyStore, *storePass, *keyAlias)
	}
	if err != nil {
		LogF("sign", err.Error())
	}
	if strings.HasSuffix(strings.ToLower(packages[0]), ".aab") {
		LogI("sign", final, "carries a v1 JAR signature of its own,", carried,
			"entries taken over as their bytes already were")
	} else if *lineage != "" {
		LogI("sign", final, "carries a rotation signature (v1+v2 old key, v3+v4 new key),", carried,
			"entries taken over as their bytes already were")
	} else {
		LogI("sign", final, "carries a v1+v2+v3 signature of its own,", carried,
			"entries taken over as their bytes already were")
	}
}

// resignPackage is the whole command in one call, so the CLI and a test drive the
// same code and the same keystore reading a build uses.
func resignPackage(inPath, finalPath, keyPath, storePass, keyAlias string) (int, error) {
	signer, cert, err := signingKey(keyPath, storePass, keyAlias)
	if err != nil {
		return 0, err
	}
	key, err := rsaKey(signer)
	if err != nil {
		return 0, err
	}
	// Un .aab no es un apk: no lleva bloque de firmas v2/v3 y no se zipaligna. Se firma
	// como JAR, que es la única firma que un bundle tiene.
	if strings.HasSuffix(strings.ToLower(inPath), ".aab") {
		return resignBundle(inPath, finalPath, key, cert)
	}
	return resignWithKey(inPath, finalPath, key, cert)
}

// resignBundle writes an .aab with a v1 JAR signature and nothing else. The entries are
// copied as they already are, the three signature files go in, and the central directory
// follows them directly: a bundle has no APK Signing Block, so there is no digest of the
// whole file and no v2/v3 pair. `jarsigner` and `apkc verify` (bundle mode) are the
// oracles; `apksigner` is not, because it expects an apk.
func resignBundle(inPath, finalPath string, key *rsa.PrivateKey, cert *x509.Certificate) (int, error) {
	pkg, err := openPreviousPackage(inPath)
	if err != nil {
		return 0, err
	}
	defer pkg.Close()
	if pkg.reader == nil {
		return 0, fmt.Errorf("%s is not a package this reader can list: a file that is not there, or a zip that does not open", inPath)
	}

	plan, digests, err := carryPlan(pkg, inPath)
	if err != nil {
		return 0, err
	}
	if len(plan) == 0 {
		return 0, fmt.Errorf("%s holds nothing outside its own signature", inPath)
	}
	reuse := make([]bool, len(plan))
	for i := range reuse {
		reuse[i] = true
	}

	tmpPath := finalPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	// El manifiesto va primero: `JarInputStream` lo busca en la primera entrada, y un
	// bundle que se lee así tiene que encontrarlo. Los datos van detrás, con sus bytes tal
	// como estaban; los digests del manifiesto no dependen del orden.
	signed, err := writeV1EntriesManifestFirst(out, plan, digests, key, cert)
	if err != nil {
		return 0, err
	}
	data, carried, _, err := writePackageEntries(out, plan, reuse, pkg, digests, true)
	if err != nil {
		return 0, err
	}
	records := append(signed, data...)

	offset, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	cd, eocd := centralDirectory(records, uint32(offset))
	if _, err := out.Write(cd); err != nil {
		return 0, err
	}
	if _, err := out.Write(eocd); err != nil {
		return 0, err
	}
	if err := out.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return 0, err
	}
	return carried, nil
}

// resignWithKey writes finalPath from inPath. Every entry is copied over as its
// stored bytes are inside the package, under a fresh local header carrying the
// padding this entry needs, so a re-sign costs no inflate and no deflate — the same
// reuse a one line source edit gets in an incremental build, applied to all of it.
func resignWithKey(inPath, finalPath string, key *rsa.PrivateKey, cert *x509.Certificate) (int, error) {
	pkg, err := openPreviousPackage(inPath)
	if err != nil {
		return 0, err
	}
	defer pkg.Close()
	if pkg.reader == nil {
		return 0, fmt.Errorf("%s is not a package this reader can list: a file that is not there, or a zip that does not open", inPath)
	}

	plan, digests, err := carryPlan(pkg, inPath)
	if err != nil {
		return 0, err
	}
	if len(plan) == 0 {
		return 0, fmt.Errorf("%s holds nothing outside its own signature", inPath)
	}

	// every entry is a copy, and the copy is only honest when this reader knows
	// where its bytes are: an entry whose offset it could not work out would
	// otherwise be written from inPath as if that file were its content
	reuse := make([]bool, len(plan))
	for i := range reuse {
		reuse[i] = true
	}

	tmpPath := finalPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	records, carried, written, err := writePackageEntries(out, plan, reuse, pkg, digests, true)
	if err != nil {
		return 0, err
	}
	if err := finishSignedPackage(out, tmpPath, finalPath, records, plan, written, key, cert); err != nil {
		return 0, err
	}
	return carried, nil
}

// carryPlan is the package as a plan: one entry per entry of the archive, in the
// order the archive lists them, with the SHA-256 of the bytes each entry *holds*
// rather than of how it was carried, which is what a JAR manifest digests.
//
// Those digests are the one pass a re-sign costs that a build does not: the packager
// computes them as it deflates an entry out of the project tree, and here they are
// already inside an archive and have to be read back out to be written down.
func carryPlan(pkg *previousPackage, inPath string) ([]bundleEntry, map[string]string, error) {
	plan := []bundleEntry{}
	digests := map[string]string{}
	seen := map[string]bool{}

	for _, file := range pkg.reader.File {
		name := file.Name
		if oldSignatureEntry(name) {
			continue
		}
		if seen[name] {
			return nil, nil, fmt.Errorf("%s lists %s twice, and a signature cannot say which of them the platform is meant to read", inPath, name)
		}
		seen[name] = true
		if pkg.index[name] == nil {
			return nil, nil, fmt.Errorf("%s cannot be copied out of %s: its data offset is not readable", name, inPath)
		}
		digest, err := entryDigest(file)
		if err != nil {
			return nil, nil, fmt.Errorf("the content of %s in %s: %w", name, inPath, err)
		}
		plan = append(plan, bundleEntry{
			src:      inPath,
			name:     name,
			compress: file.Method == zip.Deflate,
			digest:   digest,
		})
		digests[name] = digest
	}
	return plan, digests, nil
}

// entryDigest is the base64 SHA-256 of the uncompressed bytes of one entry.
func entryDigest(file *zip.File) (string, error) {
	content, err := file.Open()
	if err != nil {
		return "", err
	}
	defer content.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, content); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(digest.Sum(nil)), nil
}

// oldSignatureEntry names the files a JAR signature lives under, whatever the signer
// was called. The name is not a constant: this phone's debug keystore makes apksigner
// write ANDROIDD.SF, and another key writes ANDROID.SF (both measured here, on the
// same project), so it is the extension that says what a file is — and it is matched
// as a suffix rather than with filepath.Ext, because Ext stops at the last dot and a
// library marker called META-INF/androidx.core_core.version would be taken for a
// signature file, dropped, and then written back again under its own name on the next
// re-sign. Anything else under META-INF travels: version markers and service
// declarations are content, not signature.
func oldSignatureEntry(name string) bool {
	if !strings.HasPrefix(name, "META-INF/") {
		return false
	}
	for _, ext := range []string{".SF", ".RSA", ".DSA", ".EC", ".MF"} {
		if strings.HasSuffix(strings.ToUpper(name), ext) {
			return true
		}
	}
	return false
}
