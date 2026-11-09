package main

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// TestV1SHA1FirmaYVerifica: con v1SHA1 el manifiesto lleva `SHA1-Digest` y el .SF
// `SHA1-Digest-Manifest`; el verificador propio lo lee y la firma PKCS#7 (sha1) verifica.
func TestV1SHA1FirmaYVerifica(t *testing.T) {
	anterior := v1SHA1
	v1SHA1 = true
	defer func() {
		v1SHA1 = anterior
		digestsV1 = nil
	}()

	key, cert := signingPair(t)
	salida := filepath.Join(t.TempDir(), "sha1.apk")
	if _, err := resignWithKey(fixturePackage(t), salida, key, cert); err != nil {
		t.Fatalf("no se firma con SHA-1: %v", err)
	}

	zr, err := zip.OpenReader(salida)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var manifiesto, sf string
	for _, f := range zr.File {
		switch {
		case f.Name == v1ManifestEntry:
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			manifiesto = string(b)
		case strings.HasSuffix(f.Name, ".SF"):
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			sf = string(b)
		}
	}
	if !strings.Contains(manifiesto, "SHA1-Digest:") {
		t.Error("el manifiesto no lleva SHA1-Digest")
	}
	if !strings.Contains(sf, "SHA1-Digest-Manifest:") {
		t.Error("el .SF no lleva SHA1-Digest-Manifest")
	}
	v, err := verificarApk(salida)
	if err != nil {
		t.Fatalf("el apk SHA-1 no se verifica: %v", err)
	}
	if !v.v1 {
		t.Errorf("la v1 SHA-1 no verifica: %v", v.v1Motivos)
	}
}
