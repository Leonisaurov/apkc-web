package main

// Pruebas de paquete hostil en el verificador. Las dos nacieron de la auditoría de seguridad:
//   - un bloque de firmas que anuncia un tamaño imposible hacía que `largo-24` desbordara y `make`
//     muriera con un pánico (rc=2 y traza), en vez de con un motivo y rc=1;
//   - el barrido de la v1 se quedaba con el último `.SF` y el último `.RSA`, así que un paquete con
//     una firma buena y un segundo `.SF` se aprobaba sin haber mirado al segundo firmante.
//
// El paquete se arma en el test con archive/zip y bytes: no se versiona ningún binario (regla 2).

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// apkConBloqueDeFirmas mete 24 bytes de bloque falso justo antes del directorio central y mueve el
// registro final como lo haría un paquete con bloque: los 8 primeros bytes son el tamaño que el
// bloque dice de sí mismo y los 16 últimos, la magia. Con largo=16 el tamaño declarado no llega ni
// a cubrir la magia, que es el paquete que hacía pánico.
func apkConBloqueDeFirmas(t testing.TB, largo uint64) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	w, err := zw.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("<manifest/>")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	datos := buf.Bytes()
	eocd := len(datos) - tamanoRegistroFinal
	cd := int(binary.LittleEndian.Uint32(datos[eocd+sitioDesplazamiento:]))

	bloque := make([]byte, 24)
	binary.LittleEndian.PutUint64(bloque[:8], largo)
	copy(bloque[8:], apkSigningBlockMagic)

	salida := append([]byte{}, datos[:cd]...)
	salida = append(salida, bloque...)
	salida = append(salida, datos[cd:]...)
	// el directorio central se ha movido 24 bytes con el bloque
	binary.LittleEndian.PutUint32(salida[eocd+24+sitioDesplazamiento:], uint32(cd+24))
	return salida
}

// TestVerificarRechazaBloqueDeFirmasConTamanoImposible: el tamaño se mira antes de reservar. Los
// tres valores por debajo de 24 son los que hacían que largo-24 desbordara; el último es mayor que
// el fichero entero y tampoco puede caber. En los cuatro el veredicto es el mismo motivo, sin pánico.
func TestVerificarRechazaBloqueDeFirmasConTamanoImposible(t *testing.T) {
	const motivo = "el bloque de firmas no tiene tamaño válido"
	for _, largo := range []uint64{16, 20, 23, 1 << 40} {
		ruta := filepath.Join(t.TempDir(), "hostil.apk")
		if err := os.WriteFile(ruta, apkConBloqueDeFirmas(t, largo), 0o644); err != nil {
			t.Fatal(err)
		}
		v, err := verificarApk(ruta)
		if err != nil {
			t.Fatalf("largo %d: el verificador devolvió error en vez de un veredicto: %v", largo, err)
		}
		if v.v2 {
			t.Errorf("largo %d: la v2 se aprobó con un bloque de tamaño imposible", largo)
		}
		dicho := false
		for _, m := range v.v2Motivos {
			if m == motivo {
				dicho = true
			}
		}
		if !dicho {
			t.Errorf("largo %d: los motivos no dicen que el tamaño no vale: %v", largo, v.v2Motivos)
		}
	}
}

// TestVerificarRechazaDosFirmantesV1: el paquete bueno más un segundo .SF. El .SF extra va delante
// a propósito: con el barrido que se quedaba con el último, el verificador elegía la firma buena y
// aprobaba la v1 ignorando al otro firmante. Ahora hay dos y no se mide ninguno.
func TestVerificarRechazaDosFirmantesV1(t *testing.T) {
	doble := paqueteConFirmaExtra(t, paqueteFirmado(t),
		"META-INF/SEGUNDO.SF", []byte("Signature-Version: 1.0\r\nSHA-256-Digest-Manifest: AAAA\r\n\r\n"))

	v, err := verificarApk(doble)
	if err != nil {
		t.Fatalf("el paquete no se puede listar: %v", err)
	}
	if v.v1 {
		t.Errorf("la v1 se aprobó con dos firmantes: %v", v.v1Motivos)
	}
	const motivo = "varios firmantes v1: no medido"
	for _, m := range v.v1Motivos {
		if m == motivo {
			return
		}
	}
	t.Errorf("el motivo no dice que hay varios firmantes: %v", v.v1Motivos)
}

// paqueteConFirmaExtra reescribe un paquete con una entrada más delante de todas, que es la
// posición que hacía caer al barrido en la firma buena. Las entradas se copian por su contenido
// descomprimido, así que los digests del manifiesto siguen cuadrando.
func paqueteConFirmaExtra(t *testing.T, origen, nombre string, datos []byte) string {
	t.Helper()
	lector, err := zip.OpenReader(origen)
	if err != nil {
		t.Fatal(err)
	}
	defer lector.Close()

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	escribir := func(nombre string, contenido []byte) {
		w, err := zw.Create(nombre)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(contenido); err != nil {
			t.Fatal(err)
		}
	}
	escribir(nombre, datos)
	for _, file := range lector.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		contenido, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		escribir(file.Name, contenido)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	salida := filepath.Join(t.TempDir(), "doble.apk")
	if err := os.WriteFile(salida, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return salida
}

// FuzzVerificarApkNoMuere: cualquier cosa que se llame paquete tiene que salir por un veredicto o
// por un error con nombre, nunca por una traza. Las semillas son el bloque de 16 bytes de la
// auditoría y su variante enorme; el resto lo pone el fuzz cuando se le invoca con -fuzz.
func FuzzVerificarApkNoMuere(f *testing.F) {
	f.Add([]byte{})
	f.Add(apkConBloqueDeFirmas(f, 16))
	f.Add(apkConBloqueDeFirmas(f, 1<<40))
	f.Fuzz(func(t *testing.T, datos []byte) {
		ruta := filepath.Join(t.TempDir(), "paquete.apk")
		if err := os.WriteFile(ruta, datos, 0o644); err != nil {
			t.Fatal(err)
		}
		v, err := verificarApk(ruta)
		if err != nil {
			return
		}
		if v == nil {
			t.Fatal("el verificador no devolvió verificación ni error")
		}
	})
}
