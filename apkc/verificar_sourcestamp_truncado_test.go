package main

// El SourceStamp puede llevar, después de las firmas por esquema, un bloque opcional con los
// datos del sello (la hora) y su firma. Una cola de 1 a 7 bytes no alcanza ni para el prefijo de
// longitud de ese bloque: es un sello truncado. El oráculo es `apksigner verify -v`, que en ese
// caso dice "Verified for SourceStamp: false" con "Malformed SourceStamp signature"; aquí se
// reproduce el corte sobre un apk firmado por apksigner y se exige que las dos herramientas
// coincidan en el rechazo.

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestColaDelSelloTruncadaEsMotivo fija la decisión sin depender de apksigner: la cola vacía es
// un sello sin hora y no es un motivo; de 1 a 7 bytes no cabe un elemento y sí lo es.
func TestColaDelSelloTruncadaEsMotivo(t *testing.T) {
	if motivos := motivosDeLaColaDelSello(nil, nil); len(motivos) != 0 {
		t.Errorf("un sello sin cola dio motivos: %v", motivos)
	}
	for k := 1; k <= 7; k++ {
		motivos := motivosDeLaColaDelSello(make([]byte, k), nil)
		if len(motivos) == 0 {
			t.Errorf("una cola de %d bytes se aceptó: verde falso en un sello truncado", k)
			continue
		}
		if !strings.Contains(motivos[0], "truncado") {
			t.Errorf("la cola de %d bytes dio el motivo %q, que no dice que está truncada", k, motivos[0])
		}
	}
}

// TestSourceStampTruncadoConOraculoApksigner corta la cola de un sello real y comprueba que
// apksigner y esta verificación lo rechazan. Se reescribe el bloque de firmas sin cambiar su
// tamaño total por más que el valor del sello, para que el digest v2/v3 siga siendo el mismo.
func TestSourceStampTruncadoConOraculoApksigner(t *testing.T) {
	oraculoDisponible(t)
	dir := t.TempDir()
	firmante := parParaOraculo(t, "Firmante", 0x6001)
	sello := parParaOraculo(t, "Sello", 0x6002)
	appKey, appCert := escribeParOraculo(t, dir, "app", firmante)
	stampKey, stampCert := escribeParOraculo(t, dir, "stamp", sello)
	base := apkParaApksigner(t, dir)
	stamped := filepath.Join(dir, "stamped.apk")
	correApksigner(t, "sign", "--key", appKey, "--cert", appCert,
		"--min-sdk-version", "28",
		"--stamp-signer", "--key", stampKey, "--cert", stampCert,
		"--out", stamped, base)

	datos, err := os.ReadFile(stamped)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(stamped)
	if err != nil {
		t.Fatal(err)
	}
	eocd, err := leerRegistroFinal(f, int64(len(datos)))
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	bloque, err := leerBloqueDeFirmas(f, eocd)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	var valor []byte
	for _, par := range bloque.pares {
		if par.id == idParSourceStamp {
			valor = par.valor
		}
	}
	if valor == nil {
		t.Fatal("apksigner no escribió un par SourceStamp en el apk de prueba")
	}
	// el sello de apksigner: [firmante] y, dentro, [certificado][firmas por esquema][cola]
	firmanteBytes, _, err := primerElemento(valor)
	if err != nil {
		t.Fatal(err)
	}
	_, resto, err := primerElemento(firmanteBytes)
	if err != nil {
		t.Fatal(err)
	}
	sigs, cola, err := primerElemento(resto)
	if err != nil {
		t.Fatal(err)
	}
	if len(cola) == 0 {
		t.Skip("apksigner no escribió datos de sello opcionales: no hay cola que cortar")
	}
	// el firmante sin la cola: certificado y firmas por esquema, con sus prefijos
	certificado, _, err := primerElemento(firmanteBytes)
	if err != nil {
		t.Fatal(err)
	}
	firmanteSinCola := appendUint32(nil, uint32(len(certificado)))
	firmanteSinCola = append(firmanteSinCola, certificado...)
	firmanteSinCola = appendUint32(firmanteSinCola, uint32(len(sigs)))
	firmanteSinCola = append(firmanteSinCola, sigs...)

	for k := 1; k <= 7; k++ {
		colaCorta := append(append([]byte{}, firmanteSinCola...), make([]byte, k)...)
		nuevo := appendUint32(nil, uint32(len(colaCorta)))
		nuevo = append(nuevo, colaCorta...)
		ruta := filepath.Join(dir, fmt.Sprintf("sello-truncado-%d.apk", k))
		if err := os.WriteFile(ruta, reescribeParDelSello(t, datos, bloque, eocd, nuevo), 0o644); err != nil {
			t.Fatal(err)
		}

		salida, err := exec.Command("apksigner", "verify", "-v", "--min-sdk-version", "28", ruta).CombinedOutput()
		if err != nil {
			t.Fatalf("apksigner no pudo verificar el apk truncado: %v\n%s", err, salida)
		}
		if !strings.Contains(string(salida), "Verified for SourceStamp: false") {
			t.Errorf("apksigner aprobó una cola de %d bytes; el oráculo no respalda el rechazo:\n%s", k, salida)
		}

		v, err := verificarApk(ruta)
		if err != nil {
			t.Fatal(err)
		}
		if v.sello {
			t.Errorf("la verificación aprobó una cola de %d bytes: verde falso en un sello truncado", k)
		}
		if len(v.selloMotivos) == 0 {
			t.Errorf("la cola de %d bytes se rechazó sin motivo", k)
		} else if !strings.Contains(v.selloMotivos[0], "truncado") {
			t.Errorf("la cola de %d bytes se rechazó por %q, no por el corte", k, v.selloMotivos[0])
		}
	}
}

// reescribeParDelSello devuelve el apk con el valor del par SourceStamp sustituido por nuevo,
// ajustando el tamaño del bloque de firmas y el desplazamiento del directorio central del
// registro final. El bloque empieza en el mismo sitio, así que el digest v2/v3 no cambia.
func reescribeParDelSello(t *testing.T, datos []byte, bloque *bloqueDeFirmas, eocd *registroFinal, nuevo []byte) []byte {
	t.Helper()
	pares := []byte{}
	for _, par := range bloque.pares {
		valor := par.valor
		if par.id == idParSourceStamp {
			valor = nuevo
		}
		pares = appendUint64(pares, uint64(4+len(valor)))
		pares = appendUint32(pares, par.id)
		pares = append(pares, valor...)
	}
	largo := uint64(len(pares) + 24)
	bloqueNuevo := appendUint64(nil, largo)
	bloqueNuevo = append(bloqueNuevo, pares...)
	bloqueNuevo = appendUint64(bloqueNuevo, largo)
	bloqueNuevo = append(bloqueNuevo, []byte(apkSigningBlockMagic)...)

	nuevoCD := int64(bloque.inicio) + int64(len(bloqueNuevo))
	salida := append([]byte{}, datos[:bloque.inicio]...)
	salida = append(salida, bloqueNuevo...)
	salida = append(salida, datos[eocd.cdOffset:]...)
	posRegistro := eocd.pos + int64(len(bloqueNuevo)) - bloque.largo
	binary.LittleEndian.PutUint32(salida[posRegistro+16:], uint32(nuevoCD))
	return salida
}
