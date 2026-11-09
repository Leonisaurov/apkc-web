package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestV4FirmaYVerifica: el `.idsig` que escribe el firmante se relee y se verifica con
// el verificador propio (árbol, digest del bloque v2/v3, clave y firma RSA). El oráculo
// de verdad es `apksigner verify --v4-signature-file` en el e2e; este test sujeta la
// estructura y el acuerdo entre el escritor y el lector.
func TestV4FirmaYVerifica(t *testing.T) {
	key, cert := signingPair(t)
	entrada := fixturePackage(t)
	salida := filepath.Join(t.TempDir(), "out.apk")

	if _, err := resignWithKey(entrada, salida, key, cert); err != nil {
		t.Fatalf("el apk no se firma: %v", err)
	}
	if _, err := os.Stat(salida + ".idsig"); err != nil {
		t.Fatalf("no se escribió el .idsig: %v", err)
	}
	v, err := verificarApk(salida)
	if err != nil {
		t.Fatalf("el apk firmado no se verifica: %v", err)
	}
	if !v.v4Presente {
		t.Fatal("el verificador no encontró el .idsig")
	}
	if !v.v4 {
		t.Errorf("la v4 no verifica: %v", v.v4Motivos)
	}
}

// TestV4RechazaUnArbolCambiado: un byte cambiado en el árbol del `.idsig` tiene que
// romper la comparación con el árbol recalculado del apk.
func TestV4RechazaUnArbolCambiado(t *testing.T) {
	key, cert := signingPair(t)
	entrada := fixturePackage(t)
	salida := filepath.Join(t.TempDir(), "out.apk")
	if _, err := resignWithKey(entrada, salida, key, cert); err != nil {
		t.Fatalf("el apk no se firma: %v", err)
	}
	datos, err := os.ReadFile(salida + ".idsig")
	if err != nil {
		t.Fatal(err)
	}
	datos[len(datos)-1] ^= 0xff
	if err := os.WriteFile(salida+".idsig", datos, 0644); err != nil {
		t.Fatal(err)
	}
	v, err := verificarApk(salida)
	if err != nil {
		t.Fatalf("el apk se puede leer: %v", err)
	}
	if v.v4 {
		t.Error("un árbol cambiado pasó la verificación v4")
	}
}

// TestV4DesactivadoNoEscribeIdsig: `--v4=false` deja el apk sin sidecar, y `verify`
// entonces solo lo dice como nota, no como fallo.
func TestV4DesactivadoNoEscribeIdsig(t *testing.T) {
	key, cert := signingPair(t)
	entrada := fixturePackage(t)
	salida := filepath.Join(t.TempDir(), "out.apk")
	anterior := escribirV4
	escribirV4 = false
	defer func() { escribirV4 = anterior }()
	if _, err := resignWithKey(entrada, salida, key, cert); err != nil {
		t.Fatalf("el apk no se firma: %v", err)
	}
	if _, err := os.Stat(salida + ".idsig"); !os.IsNotExist(err) {
		t.Fatalf("con --v4=false se escribió un .idsig: %v", err)
	}
	v, err := verificarApk(salida)
	if err != nil {
		t.Fatal(err)
	}
	if v.v4Presente || v.v4 {
		t.Error("el verificador dijo que había v4 sin .idsig")
	}
}
