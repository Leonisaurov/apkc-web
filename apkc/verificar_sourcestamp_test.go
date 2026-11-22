package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El oráculo de SourceStamp es `apksigner sign --stamp-signer` aprobado por
// `apksigner verify -v` con "Verified for SourceStamp: true". Sin la herramienta
// la prueba se salta. Las firmas se comprueban contra los digests ya calculados
// por verificarApk, no contra un fichero intermedio del firmante.

// TestVerificarSourceStampConOraculoApksigner: el sello de un apk firmado por
// apksigner debe aprobarse, y dejar de aprobarse si cambia la huella del ZIP.
func TestVerificarSourceStampConOraculoApksigner(t *testing.T) {
	oraculoDisponible(t)
	dir := t.TempDir()
	firmante := parParaOraculo(t, "Firmante", 0x5001)
	sello := parParaOraculo(t, "Sello", 0x5002)
	appKey, appCert := escribeParOraculo(t, dir, "app", firmante)
	stampKey, stampCert := escribeParOraculo(t, dir, "stamp", sello)

	base := apkParaApksigner(t, dir)
	stamped := filepath.Join(dir, "stamped.apk")
	correApksigner(t, "sign", "--key", appKey, "--cert", appCert,
		"--min-sdk-version", "28",
		"--stamp-signer", "--key", stampKey, "--cert", stampCert,
		"--out", stamped, base)

	aprobado := correApksigner(t, "verify", "-v", "--min-sdk-version", "28", stamped)
	if !strings.Contains(aprobado, "Verified for SourceStamp: true") {
		t.Fatalf("el oráculo no aprobó el SourceStamp:\n%s", aprobado)
	}

	v, err := verificarApk(stamped)
	if err != nil {
		t.Fatal(err)
	}
	if !v.sello || len(v.selloMotivos) > 0 {
		t.Errorf("SourceStamp: sello=%v motivos=%v", v.sello, v.selloMotivos)
	}

	// cambiar un byte del nombre común del certificado dentro del bloque del sello: el
	// DER sigue siendo parseable, pero su SHA-256 ya no es el de stamp-cert-sha256. El
	// bloque de firmas no entra en el digest del contenido, así que v2/v3 no se tocan.
	datos, err := os.ReadFile(stamped)
	if err != nil {
		t.Fatal(err)
	}
	aguja := []byte("Sello")
	pos := -1
	for i := 0; i+len(aguja) <= len(datos); i++ {
		if string(datos[i:i+len(aguja)]) == string(aguja) {
			pos = i
			break
		}
	}
	if pos < 0 {
		t.Fatal("no se encontró el certificado de sello en el bloque de firmas")
	}
	datos[pos] ^= 0x01
	roto := filepath.Join(dir, "stamp-roto.apk")
	if err := os.WriteFile(roto, datos, 0644); err != nil {
		t.Fatal(err)
	}
	vRoto, err := verificarApk(roto)
	if err != nil {
		t.Fatal(err)
	}
	if vRoto.sello {
		t.Error("el SourceStamp se aprobó con el certificado de sello cambiado")
	}
	if len(vRoto.selloMotivos) == 0 {
		t.Error("no se dio ningún motivo al rechazar el SourceStamp roto")
	} else if !strings.Contains(vRoto.selloMotivos[0], "stamp-cert-sha256") {
		t.Errorf("motivo inesperado al rechazar el sello: %v", vRoto.selloMotivos)
	}
}
