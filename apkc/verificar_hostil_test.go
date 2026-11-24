package main

// Pruebas de paquete hostil en el verificador. Nacieron de la auditoría de seguridad:
//   - un bloque de firmas que anuncia un tamaño imposible hacía que `largo-24` desbordara y `make`
//     muriera con un pánico (rc=2 y traza), en vez de con un motivo y rc=1;
//   - el barrido de la v1 se quedaba con el último `.SF` y el último `.RSA`, así que un paquete con
//     una firma buena y un segundo `.SF` se aprobaba sin haber mirado al segundo firmante;
//   - el fuzz daba por bueno cualquier error, y verificarApk convierte los pánicos en un error: medía
//     «no se cae el test», no «no hay pánico». Ahora el prefijo del pánico recuperado es un fallo.
//
// El paquete se arma en el test con archive/zip y bytes: no se versiona ningún binario (regla 2).
//
// R6 (pendiente): el tope que la v4 pone a la lectura entera del paquete (topeApkParaV4, en
// v4verify.go) está medido en el binario nativo, pero no en el hospedador de navegador; este
// archivo no lo toca y no se baja la cota sin una medición en wasm.

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
//
// Un error no es un veredicto: verificarApk atrapa los pánicos y devuelve un error con el prefijo
// prefijoPaqueteHostil, así que el `return` que había aquí convertía el fuzz en una prueba de «no
// se cae el test». El prefijo se mira ahora en cada corrida —desde la fuente única de
// verificar.go—, y el detector se prueba con un pánico de sonda en la primera línea: un fuzz verde
// significa que no hubo pánico, no que hubo salida.
func FuzzVerificarApkNoMuere(f *testing.F) {
	f.Add([]byte{})
	f.Add(apkConBloqueDeFirmas(f, 16))
	f.Add(apkConBloqueDeFirmas(f, 1<<40))
	f.Fuzz(func(t *testing.T, datos []byte) {
		sonda := conLaGuardiaDeVerificarApk(func() { panic("sonda del fuzz") })
		if fallo := falloDelPaquete(nil, sonda); fallo == "" {
			t.Fatalf("el detector no ve un pánico recuperado: %v", sonda)
		}
		ruta := filepath.Join(t.TempDir(), "paquete.apk")
		if err := os.WriteFile(ruta, datos, 0o644); err != nil {
			t.Fatal(err)
		}
		v, err := verificarApk(ruta)
		if fallo := falloDelPaquete(v, err); fallo != "" {
			t.Fatal(fallo)
		}
	})
}

// falloDelPaquete dice qué tiene de malo lo que el verificador devolvió, o "" si no hay nada que
// reprocharle. Un error sin el prefijo es una salida legítima (un paquete que no se puede listar);
// el prefijo significa que el recover() de verificarApk atrapó un pánico, y eso el fuzz no lo
// puede dar por bueno. El prefijo es el de producción (prefijoPaqueteHostil), no una copia: si
// verificar.go cambia el literal, este detector cambia con él.
func falloDelPaquete(v *verificacion, err error) string {
	if err != nil {
		if strings.HasPrefix(err.Error(), prefijoPaqueteHostil) {
			return "el verificador recuperó un pánico: " + err.Error()
		}
		return ""
	}
	if v == nil {
		return "el verificador no devolvió verificación ni error"
	}
	return ""
}

// conLaGuardiaDeVerificarApk reproduce el recover() de verificarApk —el mismo prefijo, con el
// pánico dentro— para poder fabricar un pánico recuperado sin un paquete que lo provoque. No
// guarda una copia del literal: usa prefijoPaqueteHostil, la fuente única que también emplean
// verificarApk y falloDelPaquete.
func conLaGuardiaDeVerificarApk(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s %v", prefijoPaqueteHostil, r)
		}
	}()
	f()
	return nil
}

// TestElDetectorDePanicosDelFuzz: un pánico recuperado, que antes se contaba como «error y
// adelante», ahora falla con nombre. Es la prueba de que `falloDelPaquete` no es decorado, y de
// que el prefijo de verificar.go es el que este test vigila.
func TestElDetectorDePanicosDelFuzz(t *testing.T) {
	err := conLaGuardiaDeVerificarApk(func() {
		var nada []byte
		_ = nada[1]
	})
	if err == nil || !strings.HasPrefix(err.Error(), prefijoPaqueteHostil) {
		t.Fatalf("la guardia del test no devolvió el prefijo de pánico: %v", err)
	}
	if fallo := falloDelPaquete(nil, err); fallo == "" {
		t.Fatal("el detector dejó pasar un pánico recuperado como si fuera un veredicto")
	}
	// un error corriente sigue siendo una salida legítima: el fuzz no reprocha un paquete ilegible
	if fallo := falloDelPaquete(nil, fmt.Errorf("el paquete no se puede listar")); fallo != "" {
		t.Errorf("el detector reprocha un error normal: %q", fallo)
	}
	// y una verificación con error nulo y sin resultado tampoco es una salida válida
	if fallo := falloDelPaquete(nil, nil); fallo == "" {
		t.Error("el detector aceptó un veredicto nulo y un error nulo")
	}
}

// TestLaSondaSigueAlPrefijoDeProduccion: el detector del fuzz y su sonda no pueden guardar una
// copia del prefijo. Si la guardaran y producción cambiara el literal, el fuzz seguiría verde
// mirando un prefijo que ya no existe y quedaría ciego a los pánicos. Aquí se altera el prefijo
// de producción —la fuente única, prefijoPaqueteHostil— y se comprueba que la sonda y el detector
// lo siguen: con una copia, la sonda emitiría el literal viejo y el detector (o el HasPrefix de
// arriba) no lo reconocería, así que este test fallaría.
func TestLaSondaSigueAlPrefijoDeProduccion(t *testing.T) {
	anterior := prefijoPaqueteHostil
	prefijoPaqueteHostil = "paquete hostil alterado:"
	defer func() { prefijoPaqueteHostil = anterior }()

	err := conLaGuardiaDeVerificarApk(func() { panic("sonda del cableado") })
	if err == nil || !strings.HasPrefix(err.Error(), "paquete hostil alterado:") {
		t.Fatalf("la sonda no siguió al prefijo de producción: %v", err)
	}
	if fallo := falloDelPaquete(nil, err); fallo == "" {
		t.Fatal("el detector no siguió al prefijo de producción: quedaría ciego a los pánicos")
	}
}
