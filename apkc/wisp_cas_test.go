package main

// Pruebas de la carga estricta de CAs para el TLS sobre WISP. Un bundle con
// basura, con un certificado corrupto o sin ningún X.509 válido tiene que
// fallar: añadir un certificado de más al almacén equivale a confiar en quien lo
// puso.

import (
	"encoding/pem"
	"strings"
	"testing"
)

func pemDePrueba(t *testing.T) []byte {
	t.Helper()
	cert, _ := certificadoDePrueba(t)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
}

func TestCargarCAsPEMAceptaBundleValido(t *testing.T) {
	valido := pemDePrueba(t)
	pool, err := CargarCAsPEM(valido)
	if err != nil {
		t.Fatalf("CargarCAsPEM con un certificado válido: %v", err)
	}
	if pool == nil {
		t.Fatal("CargarCAsPEM devolvió un almacén nil")
	}
	if n := len(pool.Subjects()); n != 1 {
		t.Fatalf("el almacén tiene %d certificados, se esperaba 1", n)
	}

	// Con la huella correcta pasa igual; con otra, no.
	huella := HuellaCAsPEM(valido)
	if _, err := CargarCAsPEMFijadas(valido, huella); err != nil {
		t.Fatalf("CargarCAsPEMFijadas con la huella correcta: %v", err)
	}
	otra := huella
	otra[0] ^= 0xff
	if _, err := CargarCAsPEMFijadas(valido, otra); err == nil {
		t.Fatal("CargarCAsPEMFijadas aceptó una huella que no coincide")
	}
}

func TestCargarCAsPEMRechazaBloqueBasura(t *testing.T) {
	valido := pemDePrueba(t)
	basura := []byte("-----BEGIN BASURA-----\nAAAA\n-----END BASURA-----\n")
	bundle := append(append([]byte(nil), basura...), valido...)
	pool, err := CargarCAsPEM(bundle)
	if err == nil {
		t.Fatalf("se ignoró el bloque basura y el almacén quedó con %d certificados", len(pool.Subjects()))
	}
	if pool != nil {
		t.Fatal("CargarCAsPEM devolvió un almacén a pesar del error")
	}
	if !strings.Contains(err.Error(), "BASURA") {
		t.Fatalf("el error no nombra el bloque culpable: %v", err)
	}
	if _, err := CargarCAsPEMFijadas(bundle, HuellaCAsPEM(bundle)); err == nil {
		t.Fatal("CargarCAsPEMFijadas aceptó un bundle con un bloque basura")
	}
}

func TestCargarCAsPEMRechazaCertificadoCorrupto(t *testing.T) {
	valido := pemDePrueba(t)
	corrupto := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x30, 0x00}})
	bundle := append(append([]byte(nil), corrupto...), valido...)
	pool, err := CargarCAsPEM(bundle)
	if err == nil {
		t.Fatalf("se ignoró un certificado corrupto y el almacén quedó con %d certificados", len(pool.Subjects()))
	}
	if !strings.Contains(err.Error(), "X.509") {
		t.Fatalf("el error no describe el certificado inválido: %v", err)
	}
}

func TestCargarCAsPEMRechazaBloqueQueNoEsCertificado(t *testing.T) {
	valido := pemDePrueba(t)
	clave := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{0x01, 0x02}})
	bundle := append(append([]byte(nil), clave...), valido...)
	_, err := CargarCAsPEM(bundle)
	if err == nil || !strings.Contains(err.Error(), "PRIVATE KEY") {
		t.Fatalf("se esperaba un error por el bloque PRIVATE KEY, llegó %v", err)
	}
}

func TestCargarCAsPEMRechazaTextoFueraDeBloques(t *testing.T) {
	valido := pemDePrueba(t)
	bundle := append([]byte("esto no es un PEM\n"), valido...)
	_, err := CargarCAsPEM(bundle)
	if err == nil || !strings.Contains(err.Error(), "no son PEM") {
		t.Fatalf("se esperaba un error por el texto suelto, llegó %v", err)
	}

	// Y al final, que es donde `pem.Decode` lo dejaría como resto.
	bundle = append(append([]byte(nil), valido...), []byte("cola suelta")...)
	_, err = CargarCAsPEM(bundle)
	if err == nil || !strings.Contains(err.Error(), "no son PEM") {
		t.Fatalf("se esperaba un error por la cola suelta, llegó %v", err)
	}
}

func TestCargarCAsPEMRechazaBundleVacio(t *testing.T) {
	if _, err := CargarCAsPEM(); err == nil {
		t.Fatal("CargarCAsPEM sin bundles debía fallar")
	}
	if _, err := CargarCAsPEM([]byte("  \n\t ")); err == nil {
		t.Fatal("CargarCAsPEM con un bundle en blanco debía fallar")
	}
}
