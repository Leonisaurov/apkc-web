package main

// Las pruebas del verificador. Lo que se compara no es "el verificador está de acuerdo consigo
// mismo" en abstracto: el paquete lo escribe `resignWithKey` (el mismo camino que usa `apkc sign`,
// y por tanto el de la pestaña), y el verificador lo vuelve a leer desde el disco con sus propios
// parseadores. Los tres casos que importan son el paquete bueno, el paquete alterado y el paquete
// sin firmar, y en cada uno se comprueba el VEREDICTO y el MOTIVO con nombre.

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paqueteFirmado deja el fixture de `sign_test.go` bajo una firma v1+v2 escrita aquí.
func paqueteFirmado(t *testing.T) string {
	t.Helper()

	key, cert := signingPair(t)
	salida := filepath.Join(t.TempDir(), "firmado.apk")
	if _, err := resignWithKey(fixturePackage(t), salida, key, cert); err != nil {
		t.Fatalf("no se pudo firmar el paquete de prueba: %v", err)
	}
	return salida
}

func TestVerificarApruebaLoQueEstaBienFirmado(t *testing.T) {
	v, err := verificarApk(paqueteFirmado(t))
	if err != nil {
		t.Fatalf("el verificador no abrió el paquete: %v", err)
	}
	if !v.v1 {
		t.Errorf("v1 debería aprobar: %s", strings.Join(v.v1Motivos, " · "))
	}
	if !v.v2 {
		t.Errorf("v2 debería aprobar: %s", strings.Join(v.v2Motivos, " · "))
	}
	// el fixture trae nueve entradas de contenido más las tres de la firma; el manifiesto cubre
	// todo lo que no sea META-INF, y los directorios viajan fuera del manifiesto por regla medida
	if v.entradasFirmas < 5 {
		t.Errorf("el manifiesto cubre %d entradas, y se esperaban al menos 5", v.entradasFirmas)
	}
	if v.firmantes != 1 {
		t.Errorf("firmantes = %d, se esperaba 1", v.firmantes)
	}
	if !strings.Contains(v.certificado, "apkc test") {
		t.Errorf("el certificado no dice quién firmó: %q", v.certificado)
	}
}

func TestVerificarAvisaCuandoUnByteCambia(t *testing.T) {
	ruta := paqueteFirmado(t)
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}

	// resources.arsc viaja almacenado (sin comprimir) en este paquete: cambiar un byte suyo no
	// rompe el zip, así que lo que tiene que saltar es la comprobación del digest, no la de lista
	marca := bytes.Repeat([]byte{0x01, 0x02, 0x03}, 40)
	sitio := bytes.Index(datos, marca)
	if sitio < 0 {
		t.Fatal("el fixture no trae el recurso almacenado que esta prueba necesita")
	}
	datos[sitio+7] ^= 0x02
	alterado := filepath.Join(t.TempDir(), "alterado.apk")
	if err := os.WriteFile(alterado, datos, 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := verificarApk(alterado)
	if err != nil {
		t.Fatalf("el verificador no abrió el paquete alterado: %v", err)
	}
	if v.v1 {
		t.Error("v1 aprobó un paquete con un byte cambiado")
	}
	if v.v2 {
		t.Error("v2 aprobó un paquete con un byte cambiado")
	}
	if !strings.Contains(strings.Join(v.v1Motivos, " · "), "resources.arsc") {
		t.Errorf("v1 no nombró la entrada alterada: %s", strings.Join(v.v1Motivos, " · "))
	}
	if !strings.Contains(strings.Join(v.v2Motivos, " · "), "cambió después de firmarse") {
		t.Errorf("v2 no dijo que el contenido ya no es el firmado: %s", strings.Join(v.v2Motivos, " · "))
	}
}

func TestVerificarSinFirmaLoDicePorEsquemas(t *testing.T) {
	// el mismo fixture sin las tres piezas de la firma v1 y sin bloque: un zip de verdad, al que hay
	// que poder decirle "sin v1" y "sin v2" por separado, no con un error único
	rutaCon := fixturePackage(t)
	source, err := os.Open(rutaCon)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	zr, err := zip.NewReader(source, tamenoDe(t, rutaCon))
	if err != nil {
		t.Fatal(err)
	}

	ruta := filepath.Join(t.TempDir(), "sin-firma.apk")
	destino, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(destino)
	for _, file := range zr.File {
		if strings.HasPrefix(file.Name, "META-INF/") {
			continue
		}
		nuevo, err := zw.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(nuevo, r); err != nil {
			t.Fatal(err)
		}
		r.Close()
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	destino.Close()

	v, err := verificarApk(ruta)
	if err != nil {
		t.Fatalf("el verificador no abrió el paquete sin firmar: %v", err)
	}
	if v.v1 || v.v2 {
		t.Errorf("un paquete sin firmar no puede aprobar nada (v1 %v, v2 %v)", v.v1, v.v2)
	}
	if motivos := strings.Join(v.v1Motivos, " · "); !strings.Contains(motivos, "faltan piezas") {
		t.Errorf("v1 no dijo que faltan las piezas de la firma: %s", motivos)
	}
	if motivos := strings.Join(v.v2Motivos, " · "); !strings.Contains(motivos, "no hay bloque de firmas") {
		t.Errorf("v2 no dijo que el bloque de firmas no está: %s", motivos)
	}
}

func tamenoDe(t *testing.T, ruta string) int64 {
	t.Helper()
	info, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func TestVerificarRechazaUnPaqueteConElValorDelParAlterado(t *testing.T) {
	// el byte se cambia dentro del VALOR del par de la v2 (los datos firmados, el certificado o la
	// firma): el zip sigue siendo legible y las entradas no cambian, así que v1 tiene que quedar
	// buena y caer v2. Medido contra `apksigner verify` sobre este mismo caso: DOES NOT VERIFY
	ruta := paqueteFirmado(t)
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	eocd := bytes.LastIndex(datos, []byte{'P', 'K', 0x05, 0x06})
	cd := int(leerUint32LE(datos, eocd+16))
	inicio := cd - int(leerUint64LE(datos, cd-24)) - 8
	if inicio <= 0 || inicio >= cd {
		t.Fatalf("el bloque de firmas no está donde debe: inicio %d, cd %d", inicio, cd)
	}
	tamPar := int(leerUint64LE(datos, inicio+8))
	valor := inicio + 8 + 8 + 4
	if valor+tamPar-4 > cd-24 {
		t.Fatalf("el valor del par v2 no cabe en el bloque: %d + %d", valor, tamPar)
	}
	datos[valor+(tamPar-4)/2] ^= 0x08
	roto := filepath.Join(t.TempDir(), "roto.apk")
	if err := os.WriteFile(roto, datos, 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := verificarApk(roto)
	if err != nil {
		t.Fatalf("el verificador no abrió el paquete alterado: %v", err)
	}
	if v.v2 {
		t.Error("v2 aprobó un paquete con el valor del par alterado")
	}
	if !v.v1 {
		t.Errorf("v1 no tiene por qué enterarse del bloque: %s", strings.Join(v.v1Motivos, " · "))
	}
	if motivos := strings.Join(v.v2Motivos, " · "); motivos == "" {
		t.Error("v2 cayó sin decir por qué")
	}
}

func TestVerificarIgnoraUnParDesconocidoYLoAnota(t *testing.T) {
	// el formato manda ignorar los pares que no se conocen, y así lo hace `apksigner`: medido con
	// este mismo caso (un par 0x11223344 añadido al bloque), apksigner dice v1 true y v2 true. Si
	// herejáramos un id ajeno como fallo, aprobaríamos menos que el oráculo, que es un falso negativo.
	// El bloque no entra en el digest del contenido, por eso el paquete sigue firmando bien.
	ruta := paqueteFirmado(t)
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	eocd := bytes.LastIndex(datos, []byte{'P', 'K', 0x05, 0x06})
	cd := int(leerUint32LE(datos, eocd+16))
	largo := int(leerUint64LE(datos, cd-24))
	inicio := cd - largo - 8
	if inicio <= 0 || inicio >= cd {
		t.Fatalf("el bloque de firmas no está donde debe: inicio %d, cd %d", inicio, cd)
	}
	pares := append([]byte{}, datos[inicio+8:cd-24]...)

	ajeno := []byte{0xde, 0xad, 0xbe}
	par := appendUint64(nil, uint64(4+len(ajeno)))
	par = appendUint32(par, 0x11223344)
	par = append(par, ajeno...)
	pares = append(pares, par...)

	bloque := appendUint64(nil, uint64(len(pares)+24))
	bloque = append(bloque, pares...)
	bloque = appendUint64(bloque, uint64(len(pares)+24))
	bloque = append(bloque, apkSigningBlockMagic...)

	salida := append([]byte{}, datos[:inicio]...)
	salida = append(salida, bloque...)
	salida = append(salida, datos[cd:]...)
	// el bloque creció, así que el directorio central se corrió: el registro final tiene que decir
	// dónde está, o el verificador leería el bloque como si fuera el final del fichero
	nuevoEOCD := len(salida) - (len(datos) - eocd)
	binary.LittleEndian.PutUint32(salida[nuevoEOCD+16:], uint32(inicio+len(bloque)))

	roto := filepath.Join(t.TempDir(), "con-par-ajeno.apk")
	if err := os.WriteFile(roto, salida, 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := verificarApk(roto)
	if err != nil {
		t.Fatalf("el verificador no abrió el paquete con el par ajeno: %v", err)
	}
	notas := strings.Join(v.notas, " · ")
	if !strings.Contains(notas, "par desconocido en el bloque de firmas: 0x11223344") {
		t.Errorf("el par ajeno no se anotó como desconocido: %q", notas)
	}
	if !v.v1 || !v.v2 {
		t.Errorf("un par desconocido no invalida lo medido (v1 %v: %s · v2 %v: %s)", v.v1,
			strings.Join(v.v1Motivos, " · "), v.v2, strings.Join(v.v2Motivos, " · "))
	}
}

func leerUint32LE(b []byte, i int) uint32 {
	return uint32(b[i]) | uint32(b[i+1])<<8 | uint32(b[i+2])<<16 | uint32(b[i+3])<<24
}

func leerUint64LE(b []byte, i int) uint64 {
	return uint64(leerUint32LE(b, i)) | uint64(leerUint32LE(b, i+4))<<32
}
