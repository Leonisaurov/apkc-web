package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// buildAAB turns build/bundle.zip into build/app.aab without bundletool. `depDexes` es la
// lista de dex de las dependencias que produjo el plan del build, para que la rama del
// enlazador Go numere los dex con dexEntries y no recorra build/.
//
// The bundle.zip that the aapt2 backend leaves is already protobuf: manifest/
// AndroidManifest.xml, dex/, res/ and resources.pb. What bundletool adds is the
// BundleConfig.pb and the `base/` prefix of the base module, and that is what this
// writes. The signature is JAR v1, because a bundle has no APK Signing Block.
func buildAAB(depDexes []string) {
	salida := filepath.Join("build", "app.aab")
	entrada := filepath.Join("build", "bundle.zip")
	if _, err := os.Stat(entrada); err != nil && os.IsNotExist(err) {
		// Sin bundle.zip no hay salida de aapt2: el enlazador Go dejó AXML y resources.arsc.
		// Esta es la rama que cierra la frontera 5.33 (§128) sin bundletool ni proto de aapt2.
		if _, err := os.Stat(filepath.Join("build", "resources.arsc")); err == nil {
			if err := ensamblarAABDesdeGo("build", dexEntries("build", depDexes), salida); err != nil {
				LogF("build", err.Error(), "(el bundle se arma desde la salida del enlazador Go)")
			}
			LogI("build", salida, "ensamblado desde el enlazador Go, sin aapt2 ni bundletool")
			return
		}
	}
	if err := ensamblarAAB(entrada, salida); err != nil {
		LogF("build", err.Error(), "(el bundle se arma con --aab y el enlazador aapt2)")
	}
	LogI("build", salida, "ensamblado desde", entrada, "sin bundletool")
}

// ensamblarAAB is the assembly separated from the command so a test can drive it with a
// synthetic bundle.zip. It returns the error instead of ending the process.
func ensamblarAAB(entrada, salida string) error {
	zr, err := zip.OpenReader(entrada)
	if err != nil {
		return fmt.Errorf("no se puede abrir %s: %w", entrada, err)
	}
	defer zr.Close()

	tmp := salida + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)

	escribe := func(nombre string, datos []byte) error {
		w, err := zw.Create(nombre)
		if err != nil {
			return err
		}
		_, err = w.Write(datos)
		return err
	}
	if err := escribe("BundleConfig.pb", bundleConfigPB()); err != nil {
		return fmt.Errorf("BundleConfig.pb: %w", err)
	}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		datos, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		if err := escribe("base/"+f.Name, datos); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, salida)
}

// bundleConfigPB es el mensaje que escribió bundletool 1.18.3 en el §127: field 1
// (Bundletool) con field 2 (version) y la cadena. Es información de la herramienta, no
// del contenido; se reproduce para que el fichero sea el mismo formato.
func bundleConfigPB() []byte {
	const version = "1.18.3"
	inner := append([]byte{0x12, byte(len(version))}, version...)
	return append([]byte{0x0a, byte(len(inner))}, inner...)
}

// signAAB signs the bundle with the v1 signer of this fork. A bundle is not an apk: it
// is never zipaligned and never carries an APK Signing Block, so the JAR signature is
// the only one it has; `resignBundleCon` writes it without starting jarsigner.
func signAAB(keyStore, storePass, keyAlias, sigAlg string) {
	LogI("build", "signing aab")
	_ = sigAlg // el algoritmo lo decide la clave: RSA SHA-256 o ECDSA (§123)

	signer, cert, err := signingKey(keyStore, storePass, keyAlias)
	if err != nil {
		LogF("build", err.Error())
	}
	f, err := firmanteDe(signer, cert)
	if err != nil {
		LogF("build", err.Error())
	}
	ruta := filepath.Join("build", "app.aab")
	if _, err := resignBundleCon(ruta, ruta, f); err != nil {
		LogF("build", "no se pudo firmar el .aab:", err.Error())
	}
}

// runOrAbort runs a tool and shows its whole answer when it fails: a refusal is a wall of
// text about the module set, and the last three lines of it are not the reason. The tool
// is named by the caller because the path it was resolved to is sometimes the empty
// string, and "did not run" without a name leaves nothing to act on.
func runOrAbort(tool string, cmd *exec.Cmd) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		LogF("build", toolFailure(tool, out, err), cmd.Args[0])
	}
	if len(out) > 0 {
		LogD("build", string(out))
	}
}
