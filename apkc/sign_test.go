package main

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The package a re-sign is handed is not the one a build writes: it comes from another
// tool, so it lists its directories, carries somebody else's signature files and may
// spell the signer name anything. Every test here starts from such an archive.

// signingPair is the key and certificate a re-sign signs with. Made once per package,
// because a 2048 bit key takes long enough to matter when the suite also runs in a
// browser-shaped host.
var (
	pairOnce  sync.Once
	pairKey   *rsa.PrivateKey
	pairCert  *x509.Certificate
	pairPanic any
)

func signingPair(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	pairOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			pairPanic = err
			return
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(0x51E77B0A),
			Subject:      pkix.Name{CommonName: "apkc test", Organization: []string{"apkc"}, Country: []string{"US"}},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(24 * time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			pairPanic = err
			return
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			pairPanic = err
			return
		}
		pairKey, pairCert = key, cert
	})
	if pairPanic != nil {
		t.Fatalf("no key to sign the fixtures with: %v", pairPanic)
	}
	return pairKey, pairCert
}

// fixturePackage writes an archive that looks like what aapt2 and apksigner leave
// behind: entries of both methods, the directories an archive lists and a build plan
// never mentions, a previous signer's files under other spellings, and a name long
// enough to fold in the manifest.
func fixturePackage(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "in.apk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	// the long name is not decoration: past 72 bytes a manifest section folds onto
	// continuation lines, and a re-sign that misjoins them signs a name that is not
	// in the package
	long := "res/drawable-land-animated-24dp-for-both-tablet-and-phone-screens/icon.png"
	entries := []struct {
		name    string
		method  uint16
		content []byte
	}{
		{"AndroidManifest.xml", zip.Deflate, []byte("<manifest android:versionCode=\"7\"/>")},
		{"classes.dex", zip.Deflate, []byte("dex\n035\x00 the whole dex of a minimal app")},
		{"resources.arsc", zip.Store, bytes.Repeat([]byte{0x01, 0x02, 0x03}, 300)},
		{long, zip.Store, []byte("a stored asset with a name that folds")},
		{"lib/arm64-v8a/native.so", zip.Store, []byte("an elf a runtime maps out of the package")},
		{"META-INF/androidx.core_core.version", zip.Store, []byte("1.12.0")},
		{"META-INF/ANDROIDD.SF", zip.Deflate, []byte("Signature-Version: 1.0\r\n\r\n")},
		{"META-INF/ANDROIDD.RSA", zip.Store, []byte("a pkcs7 block of the previous signer")},
		{"META-INF/MANIFEST.MF", zip.Deflate, []byte("Manifest-Version: 1.0\r\n\r\n")},
		{"res/", zip.Store, nil},
		{"res/layout/", zip.Store, nil},
	}
	for _, e := range entries {
		head := &zip.FileHeader{Name: e.name, Method: e.method}
		head.Modified = time.Date(2026, 10, 26, 3, 30, 0, 0, time.UTC)
		entry, err := w.CreateHeader(head)
		if err != nil {
			t.Fatal(err)
		}
		if len(e.content) > 0 {
			if _, err := entry.Write(e.content); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// readPackage opens an archive the way any tool would: for its list of names and for
// the bytes each entry holds.
func readPackage(t *testing.T, path string) map[string][]byte {
	t.Helper()

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("the signed package does not open: %v", err)
	}
	defer r.Close()

	content := map[string][]byte{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		content[f.Name] = body
	}
	return content
}

// TestResignCarriesEveryEntryAndDropsTheOldSignature is the promise of the command:
// the package that comes out holds what went in, under a signature of its own. The
// previous signer's files must not travel — a package listing two signers is read as
// corrupt, not as resigned — and its library markers must.
func TestResignCarriesEveryEntryAndDropsTheOldSignature(t *testing.T) {
	in := fixturePackage(t)
	out := filepath.Join(filepath.Dir(in), "signed.apk")
	key, cert := signingPair(t)

	carried, err := resignWithKey(in, out, key, cert)
	if err != nil {
		t.Fatalf("resignWithKey: %v", err)
	}

	want := map[string][]byte{
		"AndroidManifest.xml":                 []byte("<manifest android:versionCode=\"7\"/>"),
		"classes.dex":                         []byte("dex\n035\x00 the whole dex of a minimal app"),
		longFixtureName():                     []byte("a stored asset with a name that folds"),
		"lib/arm64-v8a/native.so":             []byte("an elf a runtime maps out of the package"),
		"META-INF/androidx.core_core.version": []byte("1.12.0"),
		"res/":                                nil,
		"res/layout/":                         nil,
	}
	got := readPackage(t, out)
	for name, content := range want {
		body, ok := got[name]
		if !ok {
			t.Errorf("%s did not travel into the signed package", name)
			continue
		}
		if !bytes.Equal(body, content) {
			t.Errorf("%s came out as %q, want the bytes that went in", name, body)
		}
	}
	if _, ok := got["resources.arsc"]; !ok {
		t.Error("resources.arsc did not travel")
	}
	// ANDROIDD.* are the previous signer's own names and must not survive; its
	// MANIFEST.MF is replaced under the same name, which is what the digest check
	// below reads back instead.
	for _, gone := range []string{"META-INF/ANDROIDD.SF", "META-INF/ANDROIDD.RSA"} {
		if _, ok := got[gone]; ok {
			t.Errorf("%s is still in the package, and two signers in one archive is what a verifier reports as corrupt", gone)
		}
	}
	for _, here := range []string{v1SFEntry, v1BlockEntry, v1ManifestEntry} {
		if _, ok := got[here]; !ok {
			t.Errorf("%s is missing from the signed package", here)
		}
	}
	// the count is the reuse this command exists for: nothing was written again
	if carried != len(want)+1 {
		t.Errorf("%d entries carried, want the %d of the fixture", carried, len(want)+1)
	}
}

func longFixtureName() string {
	return "res/drawable-land-animated-24dp-for-both-tablet-and-phone-screens/icon.png"
}

// manifestAttribute reads one attribute out of the middle of a section, which is
// where a digest sits: attrValue answers only for the first line, and a section puts
// its Name there. Continuation lines are joined because a folded value is one value.
func manifestAttribute(section, attr string) string {
	lines := strings.Split(section, "\r\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, attr+": ") {
			continue
		}
		value := line[len(attr)+2:]
		for _, next := range lines[i+1:] {
			if !strings.HasPrefix(next, " ") {
				break
			}
			value += next[1:]
		}
		return value
	}
	return ""
}

// TestResignManifestDigestsWhatTheEntriesHold is the v1 check done without apkc's own
// code: every name in the package outside the signature and the directories has a
// section, and the digest in it is the SHA-256 of the bytes read back out of the
// archive. A section that names what is not there verifies; a section whose digest is
// of the wrong thing does not, and that is the difference between a signature and a
// decoration.
func TestResignManifestDigestsWhatTheEntriesHold(t *testing.T) {
	in := fixturePackage(t)
	out := filepath.Join(filepath.Dir(in), "signed.apk")
	key, cert := signingPair(t)

	if _, err := resignWithKey(in, out, key, cert); err != nil {
		t.Fatal(err)
	}

	content := readPackage(t, out)
	manifest := string(content[v1ManifestEntry])
	if manifest == "" {
		t.Fatal("the package has no manifest to read")
	}
	sections := strings.SplitAfter(manifest, "\r\n\r\n")
	digests := map[string]string{}
	for _, section := range sections[1:] {
		name := attrValue(section, attrName)
		if name == "" {
			// the split leaves one empty piece behind the last section, and a
			// section that named nothing would be a bug in the writer rather than
			// something a reader of this file could digest
			continue
		}
		digests[name] = manifestAttribute(section, attrDigest)
	}
	if len(digests) == 0 {
		t.Fatal("the manifest has no sections to check")
	}
	for name, body := range content {
		switch {
		case strings.HasPrefix(name, "META-INF/"):
			continue
		case strings.HasSuffix(name, "/"):
			// java.util.jar leaves a directory out of the manifest while keeping it
			// in the archive, so its absence here is the correct answer
			if _, ok := digests[name]; ok {
				t.Errorf("%s is a directory and has a manifest section", name)
			}
			continue
		}
		have, ok := digests[name]
		if !ok {
			t.Errorf("%s is in the package and not in the manifest", name)
			continue
		}
		sum := sha256.Sum256(body)
		if want := base64.StdEncoding.EncodeToString(sum[:]); have != want {
			t.Errorf("%s: manifest digest %q, the bytes hold %q", name, have, want)
		}
	}
}

// TestResignLandsStoredDataOnTheBoundaryItPromises: alignment is decided as each
// entry is written, and a re-sign writes the headers itself even though it copies the
// bytes. A .so the manifest expects to map out of the package is the one that pays if
// the rule is wrong, so both the 4 and the 4096 are checked.
func TestResignLandsStoredDataOnTheBoundaryItPromises(t *testing.T) {
	in := fixturePackage(t)
	out := filepath.Join(filepath.Dir(in), "signed.apk")
	key, cert := signingPair(t)

	if _, err := resignWithKey(in, out, key, cert); err != nil {
		t.Fatal(err)
	}

	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Method == zip.Deflate {
			continue
		}
		at, err := f.DataOffset()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		boundary := int64(alignStandard)
		if isNativeLib(f.Name) {
			boundary = int64(alignPage)
		}
		if at%boundary != 0 {
			t.Errorf("%s data sits at %d, off the %d byte boundary it needs", f.Name, at, boundary)
		}
	}
}

// TestResignKeepsTheMethodOfWhatItCarries: an entry copied byte for byte comes with
// the method it was written with, and a flipped one is a package the loader reads
// wrongly — a deflated .so behind a manifest that promises to map it is worse than a
// rebuild.
func TestResignKeepsTheMethodOfWhatItCarries(t *testing.T) {
	in := fixturePackage(t)
	out := filepath.Join(filepath.Dir(in), "signed.apk")
	key, cert := signingPair(t)

	if _, err := resignWithKey(in, out, key, cert); err != nil {
		t.Fatal(err)
	}

	before := map[string]uint16{}
	for _, f := range mustOpen(t, in).File {
		before[f.Name] = f.Method
	}
	for _, f := range mustOpen(t, out).File {
		method, ok := before[f.Name]
		if !ok {
			continue
		}
		if f.Method != method {
			t.Errorf("%s changed method from %d to %d", f.Name, method, f.Method)
		}
	}
}

func mustOpen(t *testing.T, path string) *zip.Reader {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	r, err := zip.NewReader(f, info.Size())
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return r
}

// TestResignRefusesAPackageThatListsANameTwice: two entries under one name is a
// package whose signature cannot say what it covers. Reusing the name would sign the
// last copy and let the first one install.
func TestResignRefusesAPackageThatListsANameTwice(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "twice.apk")
	f, err := os.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for i, body := range [][]byte{[]byte("the first"), []byte("the second")} {
		head := &zip.FileHeader{Name: "classes.dex", Method: zip.Deflate}
		head.Modified = time.Now().Add(time.Duration(i) * time.Second)
		entry, err := w.CreateHeader(head)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out := filepath.Join(dir, "signed.apk")
	key, cert := signingPair(t)
	if _, err := resignWithKey(in, out, key, cert); err == nil {
		t.Fatal("a package listing classes.dex twice was signed")
	} else if !strings.Contains(err.Error(), "twice") {
		t.Errorf("the refusal says %q, want it to name the duplicate", err)
	}
}

// TestResignRefusesWhatItCannotList keeps the command from writing an empty signature
// over a file that is not a package: the message has to say which input is at fault.
func TestResignRefusesWhatItCannotList(t *testing.T) {
	dir := t.TempDir()
	key, cert := signingPair(t)

	notAZip := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notAZip, []byte("this is not an archive"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := resignWithKey(notAZip, filepath.Join(dir, "out.apk"), key, cert); err == nil {
		t.Error("a text file was signed")
	}

	if _, err := resignWithKey(filepath.Join(dir, "absent.apk"), filepath.Join(dir, "out.apk"), key, cert); err == nil {
		t.Error("a file that is not there was signed")
	}
}

// TestSignLeavesTheInputAloneUntilThePackageIsWhole: the output goes through a
// temporary file and a rename, so a refusal mid-way cannot replace the package the
// user came with.
func TestSignWritesToTheChosenOutputAndKeepsTheInput(t *testing.T) {
	in := fixturePackage(t)
	before, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(filepath.Dir(in), "signed.apk")
	key, cert := signingPair(t)

	if _, err := resignWithKey(in, out, key, cert); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the input package was rewritten by a sign that named another output")
	}
	if _, err := os.Stat(in + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary file outlived the command")
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary output outlived the command")
	}
}

// TestOldSignatureEntryReadsWhatApksignerSpells: the signer name is chosen from the
// key, not fixed, and this phone's debug keystore makes apksigner write ANDROIDD.SF
// while apkc writes ANDROID.SF. Matching only the name apkc uses would leave the old
// signature inside a package this command says it signed.
func TestOldSignatureEntryReadsWhatApksignerSpells(t *testing.T) {
	for _, e := range []struct {
		name string
		old  bool
	}{
		{"META-INF/ANDROID.SF", true},
		{"META-INF/ANDROIDD.SF", true},
		{"META-INF/ANDROID.RSA", true},
		{"META-INF/ANDROIDD.RSA", true},
		{"META-INF/MANIFEST.MF", true},
		{"META-INF/CERT.DSA", true},
		{"META-INF/ECLIPSE_.EC", true},
		{"META-INF/androidx.core_core.version", false},
		{"META-INF/services/javax.annotation.processing.Processor", false},
		{"META-INF/proguard/androidx-annotations.pro", false},
		{"META-INF/", false},
		{"res/layout/main.xml", false},
	} {
		if got := oldSignatureEntry(e.name); got != e.old {
			t.Errorf("oldSignatureEntry(%q) = %v, want %v", e.name, got, e.old)
		}
	}
}

// TestElBloqueV3LlevaSuParYSuSigner: APK Signature Scheme v3. El bloque lleva el par v3 además del
// v2; el signer tiene signed data, minSdk 24, maxSdk 0x7fffffff, las firmas y la clave pública, con
// los digests y certificados envueltos en su longitud (parrafo 108).
func TestElBloqueV3LlevaSuParYSuSigner(t *testing.T) {
	key, cert := signingPair(t)
	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i)
	}
	f, err := firmanteDe(key, cert)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := v2SignerBlock(digest, f)
	if err != nil {
		t.Fatal(err)
	}
	v3, err := v3SignerBlock(digest, f)
	if err != nil {
		t.Fatal(err)
	}
	bloque := apkSigningBlock(v2, v3)
	if !bytes.HasSuffix(bloque, []byte(apkSigningBlockMagic)) {
		t.Fatal("el bloque no termina en la magia")
	}
	leer := func(b []byte, o int) uint32 {
		return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
	}
	pares := map[uint32]bool{}
	fin := len(bloque) - 24
	for o := 8; o < fin; {
		l := int(uint64(leer(bloque, o)) | uint64(leer(bloque, o+4))<<32)
		o += 8
		pares[leer(bloque, o)] = true
		o += l
	}
	if !pares[apkSigningBlockIDv2] || !pares[apkSigningBlockIDv3] {
		t.Fatalf("faltan pares: v2=%v v3=%v", pares[apkSigningBlockIDv2], pares[apkSigningBlockIDv3])
	}
	// el value de v3: seq(seq(signer))
	if int(leer(v3, 0))+4 != len(v3) {
		t.Fatal("la longitud exterior de v3 no cierra")
	}
	inner := v3[4 : 4+leer(v3, 0)]
	if int(leer(inner, 0))+4 != len(inner) {
		t.Fatal("la longitud del signer no cierra")
	}
	signer := inner[4:]
	sdLen := int(leer(signer, 0))
	if sdLen+4 >= len(signer) {
		t.Fatal("signed data fuera del signer")
	}
	pos := 4 + sdLen
	if leer(signer, pos) != 24 || leer(signer, pos+4) != 0x7fffffff {
		t.Fatalf("min/max son %d/%#x", leer(signer, pos), leer(signer, pos+4))
	}
	pos += 8
	if int(leer(signer, pos))+4 > len(signer) {
		t.Fatal("las firmas se salen del signer")
	}
}
