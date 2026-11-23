package main

// La v1 materializaba cada entrada que el MANIFEST nombraba con io.ReadAll: un paquete firmado
// por quien lo ataca, con una entrada de 1 a 4 GiB declarados, agotaba la memoria antes de que
// hubiera un veredicto. Ahora el tamaño declarado se mira antes de abrir y el digest se calcula
// en flujo. Esta prueba fabrica el ZIP a mano para poder declarar 4 GiB sin escribirlos.

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type entradaCruda struct {
	nombre    string
	datos     []byte
	metodo    uint16
	declarado uint32
}

func le16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func le32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }

// escribeZipCrudo arma un ZIP mínimo con los tamaños que diga cada entrada, sin comprimir ni
// validar nada: es la única forma de que una cabecera declare 4 GiB con cuatro bytes detrás.
func escribeZipCrudo(t *testing.T, ruta string, entradas []entradaCruda) {
	t.Helper()
	cuerpo := []byte{}
	central := []byte{}
	for _, e := range entradas {
		declarado := e.declarado
		if declarado == 0 {
			declarado = uint32(len(e.datos))
		}
		crc := crc32.ChecksumIEEE(e.datos)
		desplazamiento := uint32(len(cuerpo))

		cuerpo = le32(cuerpo, 0x04034b50)
		cuerpo = le16(cuerpo, 20)
		cuerpo = le16(cuerpo, 0)
		cuerpo = le16(cuerpo, e.metodo)
		cuerpo = le16(cuerpo, 0)
		cuerpo = le16(cuerpo, 0)
		cuerpo = le32(cuerpo, crc)
		cuerpo = le32(cuerpo, uint32(len(e.datos)))
		cuerpo = le32(cuerpo, declarado)
		cuerpo = le16(cuerpo, uint16(len(e.nombre)))
		cuerpo = le16(cuerpo, 0)
		cuerpo = append(cuerpo, e.nombre...)
		cuerpo = append(cuerpo, e.datos...)

		central = le32(central, 0x02014b50)
		central = le16(central, 20)
		central = le16(central, 20)
		central = le16(central, 0)
		central = le16(central, e.metodo)
		central = le16(central, 0)
		central = le16(central, 0)
		central = le32(central, crc)
		central = le32(central, uint32(len(e.datos)))
		central = le32(central, declarado)
		central = le16(central, uint16(len(e.nombre)))
		central = le16(central, 0)
		central = le16(central, 0)
		central = le16(central, 0)
		central = le16(central, 0)
		central = le32(central, 0)
		central = le32(central, desplazamiento)
		central = append(central, e.nombre...)
	}

	salida := append([]byte{}, cuerpo...)
	cdOffset := uint32(len(salida))
	salida = append(salida, central...)
	salida = le32(salida, 0x06054b50)
	salida = le16(salida, 0)
	salida = le16(salida, 0)
	salida = le16(salida, uint16(len(entradas)))
	salida = le16(salida, uint16(len(entradas)))
	salida = le32(salida, uint32(len(central)))
	salida = le32(salida, cdOffset)
	salida = le16(salida, 0)
	if err := os.WriteFile(ruta, salida, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestVerificarRechazaEntradaDeclaradaEnormeSinLeerla: el MANIFEST nombra una entrada que
// declara casi 4 GiB y cuyo flujo deflate es basura de cuatro bytes. Si la verificación la abriera,
// el motivo sería un error de lectura; el motivo tiene que ser el tope, y tiene que llegar sin
// que el flujo se toque.
func TestVerificarRechazaEntradaDeclaradaEnormeSinLeerla(t *testing.T) {
	manifiesto := "Manifest-Version: 1.0\r\n\r\n" +
		"Name: assets/grande.bin\r\n" +
		"SHA-256-Digest: AAAA\r\n\r\n"
	sf := "Signature-Version: 1.0\r\nSHA-256-Digest-Manifest: AAAA\r\n\r\n"
	apk := filepath.Join(t.TempDir(), "enorme.apk")
	escribeZipCrudo(t, apk, []entradaCruda{
		{nombre: "META-INF/MANIFEST.MF", datos: []byte(manifiesto)},
		{nombre: "META-INF/CERT.SF", datos: []byte(sf)},
		{nombre: "META-INF/CERT.RSA", datos: []byte("bloque")},
		// método 8 (deflate), cuatro bytes de basura y casi 4 GiB declarados
		{nombre: "assets/grande.bin", datos: []byte{0xff, 0xff, 0xff, 0xff}, metodo: 8, declarado: 1<<32 - 1},
	})

	v, err := verificarApk(apk)
	if err != nil {
		t.Fatalf("el paquete no se puede listar: %v", err)
	}
	tope := false
	for _, motivo := range v.v1Motivos {
		if strings.Contains(motivo, "por encima del tope") {
			tope = true
		}
		if strings.Contains(motivo, "no se puede digerir") {
			t.Errorf("la entrada se abrió y su flujo se leyó: %q", motivo)
		}
	}
	if !tope {
		t.Errorf("la entrada de casi 4 GiB no se rechazó por el tope: %v", v.v1Motivos)
	}
}

// TestElTopeDeLaV1SePuedeBajar: con el tope bajo, una entrada normal que lo supera también se
// rechaza por el tamaño declarado, sin depender del valor de 1 GiB.
func TestElTopeDeLaV1SePuedeBajar(t *testing.T) {
	anterior := topeEntradaV1
	topeEntradaV1 = 16
	defer func() { topeEntradaV1 = anterior }()

	manifiesto := "Manifest-Version: 1.0\r\n\r\n" +
		"Name: assets/normal.bin\r\n" +
		"SHA-256-Digest: AAAA\r\n\r\n"
	sf := "Signature-Version: 1.0\r\nSHA-256-Digest-Manifest: AAAA\r\n\r\n"
	apk := filepath.Join(t.TempDir(), "normal.apk")
	escribeZipCrudo(t, apk, []entradaCruda{
		{nombre: "META-INF/MANIFEST.MF", datos: []byte(manifiesto)},
		{nombre: "META-INF/CERT.SF", datos: []byte(sf)},
		{nombre: "META-INF/CERT.RSA", datos: []byte("bloque")},
		{nombre: "assets/normal.bin", datos: []byte("veinte bytes o más")},
	})

	v, err := verificarApk(apk)
	if err != nil {
		t.Fatal(err)
	}
	for _, motivo := range v.v1Motivos {
		if strings.Contains(motivo, "por encima del tope de 16") {
			return
		}
	}
	t.Errorf("la entrada de %d bytes no se rechazó con el tope en 16: %v",
		len("veinte bytes o más"), v.v1Motivos)
}
