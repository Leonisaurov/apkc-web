package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fixtureAAB is a zip shaped like a bundle: BundleConfig.pb and a module with a dex
// and a manifest. It is not a real bundle built by bundletool (that needs the proto
// resource table), only the container the signature sees.
func fixtureAAB(t *testing.T) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "in.aab")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, nombre := range []string{
		"BundleConfig.pb",
		"base/dex/classes.dex",
		"base/manifest/AndroidManifest.xml",
	} {
		w, err := zw.Create(nombre)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("contenido de " + nombre)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// TestUnBundleSeFirmaComoJarYManifiestoPrimero: un .aab no lleva bloque v2/v3, y el
// manifiesto tiene que ser la primera entrada para que JarInputStream lo encuentre
// (jarsigner avisa «Manifest is missing when reading via JarInputStream» si va detrás).
func TestUnBundleSeFirmaComoJarYManifiestoPrimero(t *testing.T) {
	key, cert := signingPair(t)
	entrada := fixtureAAB(t)
	salida := filepath.Join(t.TempDir(), "out.aab")

	carried, err := resignBundle(entrada, salida, key, cert)
	if err != nil {
		t.Fatalf("el bundle no se firma: %v", err)
	}
	if carried != 3 {
		t.Errorf("%d entradas copiadas, se esperaban 3", carried)
	}

	datos, err := os.ReadFile(salida)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(datos, []byte("APK Sig Block 42")) {
		t.Error("un .aab no lleva bloque de firmas v2/v3 y este lo tiene")
	}
	zr, err := zip.OpenReader(salida)
	if err != nil {
		t.Fatalf("el bundle firmado no se abre: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 6 {
		t.Fatalf("%d entradas, se esperaban 3 de datos y 3 de firma", len(zr.File))
	}
	if zr.File[0].Name != v1ManifestEntry {
		t.Errorf("la primera entrada es %q, y JarInputStream busca %q", zr.File[0].Name, v1ManifestEntry)
	}

	v, err := verificarApk(salida)
	if err != nil {
		t.Fatalf("el bundle firmado no se verifica: %v", err)
	}
	if !v.bundle {
		t.Error("el verificador no reconoció el .aab como bundle")
	}
	if !v.v1 {
		t.Errorf("la v1 del bundle no verifica: %v", v.v1Motivos)
	}
	if v.v2 {
		t.Error("el bundle dijo tener v2, y no la tiene")
	}
}

// TestSignDistingueElBundlePorLaExtension: el mismo comando firma un apk (v1+v2+v3) o
// un .aab (solo v1) mirando la extensión, que es lo que la pestaña manda.
func TestSignDistingueElBundlePorLaExtension(t *testing.T) {
	ks := escribeFixtures(t, releaseP12Fixture, "release.p12")
	entrada := fixtureAAB(t)
	salida := filepath.Join(t.TempDir(), "out.aab")
	carried, err := resignPackage(entrada, salida, ks, "s3creto", "miclave")
	if err != nil {
		t.Fatalf("apkc sign no firmó el .aab: %v", err)
	}
	if carried != 3 {
		t.Errorf("%d entradas copiadas, se esperaban 3", carried)
	}
	v, err := verificarApk(salida)
	if err != nil {
		t.Fatalf("el .aab de apkc sign no se verifica: %v", err)
	}
	if !v.bundle || !v.v1 || v.v2 {
		t.Errorf("veredicto del .aab: bundle=%v v1=%v v2=%v (%v)", v.bundle, v.v1, v.v2, v.v1Motivos)
	}
}
