package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestDebugKeystoreSeEscribeYFirma(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "debug.keystore")
	if err := escribirDebugKeystore(ruta, "android", "androiddebugkey", 10000); err != nil {
		t.Fatalf("escribirDebugKeystore: %v", err)
	}
	blob, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if len(blob) < 12 || blob[0] != 0xFE || blob[1] != 0xED || blob[2] != 0xFE || blob[3] != 0xED {
		t.Fatalf("el fichero no empieza con el magic de JKS: %x", blob[:4])
	}
	firmante, cert, err := signingKey(ruta, "android", "androiddebugkey")
	if err != nil {
		t.Fatalf("signingKey no abre el keystore generado: %v", err)
	}
	if cert.Subject.CommonName != "Android Debug" {
		t.Fatalf("CN = %q, se esperaba Android Debug", cert.Subject.CommonName)
	}
	if len(cert.Subject.Organization) != 1 || cert.Subject.Organization[0] != "Android" ||
		len(cert.Subject.Country) != 1 || cert.Subject.Country[0] != "US" {
		t.Fatalf("sujeto inesperado: %v", cert.Subject)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("la clave pública no es RSA: %T", cert.PublicKey)
	}
	datos := []byte("firma de prueba")
	digest := sha256.Sum256(datos)
	firma, err := firmante.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("la clave generada no firma: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], firma); err != nil {
		t.Fatalf("la firma no verifica con el certificado generado: %v", err)
	}
}

func TestDebugKeystoreRechazaOtraContrasena(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "debug.keystore")
	if err := escribirDebugKeystore(ruta, "android", "androiddebugkey", 30); err != nil {
		t.Fatal(err)
	}
	if _, _, err := signingKey(ruta, "otra", "androiddebugkey"); err == nil {
		t.Fatal("una contraseña equivocada abrió el keystore")
	}
}
