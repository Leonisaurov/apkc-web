package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestBundleConfigPB: el mensaje medido en el §127 (bundletool 1.18.3).
func TestBundleConfigPB(t *testing.T) {
	quiero := []byte{0x0a, 0x08, 0x12, 0x06, '1', '.', '1', '8', '.', '3'}
	if got := bundleConfigPB(); !bytes.Equal(got, quiero) {
		t.Fatalf("BundleConfig.pb = %x, se esperaba %x", got, quiero)
	}
}

// TestElAABEnsamblaElBundleProto: un bundle.zip con la forma que deja aapt2 se convierte
// en un .aab con BundleConfig.pb y las entradas bajo base/; después se firma como bundle
// y el verificador propio lo reconoce.
func TestElAABEnsamblaElBundleProto(t *testing.T) {
	dir := t.TempDir()
	entrada := filepath.Join(dir, "bundle.zip")
	f, err := os.Create(entrada)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, nombre := range []string{
		"manifest/AndroidManifest.xml",
		"dex/classes.dex",
		"resources.pb",
		"res/layout/main.xml",
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

	salida := filepath.Join(dir, "app.aab")
	if err := ensamblarAAB(entrada, salida); err != nil {
		t.Fatalf("el .aab no se ensambla: %v", err)
	}
	zr, err := zip.OpenReader(salida)
	if err != nil {
		t.Fatal(err)
	}
	tenidos := map[string]bool{}
	for _, f := range zr.File {
		tenidos[f.Name] = true
	}
	zr.Close()
	for _, nombre := range []string{
		"BundleConfig.pb",
		"base/manifest/AndroidManifest.xml",
		"base/dex/classes.dex",
		"base/resources.pb",
		"base/res/layout/main.xml",
	} {
		if !tenidos[nombre] {
			t.Errorf("falta %s en el .aab", nombre)
		}
	}

	key, cert := signingPair(t)
	firmado := filepath.Join(dir, "firmado.aab")
	fm, err := firmanteDe(key, cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resignBundleCon(salida, firmado, fm); err != nil {
		t.Fatalf("el .aab no se firma: %v", err)
	}
	v, err := verificarApk(firmado)
	if err != nil {
		t.Fatal(err)
	}
	if !v.bundle || !v.v1 {
		t.Errorf("el .aab firmado no se reconoce: bundle=%v v1=%v (%v)", v.bundle, v.v1, v.v1Motivos)
	}
}
