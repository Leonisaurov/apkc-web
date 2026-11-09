package main

// El comando `apkc keystore` y la lista de identidades que enseña la pestaña antes de
// firmar. El fixture es un PKCS#12 de usar y tirar (alias `miclave`, contraseña `s3creto`,
// CN=Mi App), generado con `keytool` para que la prueba no dependa del keystore de una
// máquina concreta y corra igual dentro del hospedador de navegador.

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const releaseP12Fixture = "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"

func escribeFixtures(t *testing.T, b64, nombre string) string {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	ruta := filepath.Join(t.TempDir(), nombre)
	if err := os.WriteFile(ruta, datos, 0600); err != nil {
		t.Fatalf("no se pudo escribir el fixture: %v", err)
	}
	return ruta
}

// TestInspectarKeystoreListaAliasSujetoYHuella: el comando que la pestaña usa para enseñar
// el certificado antes de firmar. El SHA-256 es el mismo que imprime `apksigner --print-certs`.
func TestInspectarKeystoreListaAliasSujetoYHuella(t *testing.T) {
	ruta := escribeFixtures(t, releaseP12Fixture, "release.p12")
	info, err := inspectarKeystore(ruta, "s3creto")
	if err != nil {
		t.Fatalf("el keystore de release no se inspecciona: %v", err)
	}
	if info.Keys != 1 || len(info.Entries) != 1 {
		t.Fatalf("%d claves y %d entradas, se esperaba una de cada", info.Keys, len(info.Entries))
	}
	e := info.Entries[0]
	if e.Alias != "miclave" {
		t.Errorf("alias %q, se esperaba miclave", e.Alias)
	}
	if !strings.Contains(e.Subject, "CN=Mi App") {
		t.Errorf("sujeto %q, se esperaba CN=Mi App", e.Subject)
	}
	const huella = "A7:21:72:B6:F4:00:5D:B7:62:FD:04:F9:1F:E2:86:C1:2C:7B:75:EE:37:52:7D:B5:AF:38:23:24:31:96:B0:7F"
	if e.SHA256 != huella {
		t.Errorf("huella %q, se esperaba la que imprime apksigner: %q", e.SHA256, huella)
	}
	if e.NotAfter != "2054-03-26" {
		t.Errorf("validez %q, se esperaba 2054-03-26", e.NotAfter)
	}
}

// TestInspectarKeystoreNombraLaContrasenaMal: una contraseña que no abre el fichero tiene que
// decirlo como contraseña, no como un keystore corrupto.
func TestInspectarKeystoreNombraLaContrasenaMal(t *testing.T) {
	ruta := escribeFixtures(t, releaseP12Fixture, "release.p12")
	if _, err := inspectarKeystore(ruta, "mala"); err == nil || !strings.Contains(err.Error(), "storepass") {
		t.Fatalf("la contraseña mala no se nombró: %v", err)
	}
}

// TestInspectarKeystoreLeeJKSYListaSuAlias: el mismo comando sobre el JKS que keytool escribía
// antes de JDK 9.
func TestInspectarKeystoreLeeJKSYListaSuAlias(t *testing.T) {
	ruta := escribeFixtures(t, jksFixture, "debug.keystore")
	info, err := inspectarKeystore(ruta, "android")
	if err != nil {
		t.Fatalf("el JKS no se inspecciona: %v", err)
	}
	if info.Keys != 1 || len(info.Entries) != 1 {
		t.Fatalf("%d claves y %d entradas, se esperaba una de cada", info.Keys, len(info.Entries))
	}
	if info.Entries[0].Alias != "androiddebugkey" {
		t.Errorf("alias %q, se esperaba androiddebugkey", info.Entries[0].Alias)
	}
	if !strings.Contains(info.Entries[0].Subject, "Test JKS") {
		t.Errorf("sujeto %q, se esperaba CN=Test JKS", info.Entries[0].Subject)
	}
}
