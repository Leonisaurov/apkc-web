package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// TestValoresSinEntradaBase mide cómo aapt2 de build-tools 35.0.1 escribe la tabla cuando un tipo
// de values solo existe en una configuración secundaria. El bool y el integer que solo están en
// values-es se descartan («removing resource … without required default value»), pero aapt2 ya
// había reservado sus ids: deja los nombres de relleno `?1` y `?2` en la tabla de nombres de tipo y
// `string` queda con id 3 y resource 0x7f030000. El arsc de 672 bytes sale byte a byte.
func TestValoresSinEntradaBase(t *testing.T) {
	man := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.sinbase"><application/></manifest>`)}
	valores := []fuenteXML{
		{nombre: "res/values/strings.xml", contenido: []byte(`<resources><string name="app_name">Base</string></resources>`)},
		{nombre: "res/values-es/strings.xml", contenido: []byte(`<resources><string name="app_name">ES</string><bool name="soloes">true</bool><integer name="soloentero">7</integer></resources>`), config: "es"},
	}
	salida, err := enlazarProyecto(valores, nil, nil, man, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatal(err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	fijado, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(arscSinBase, "\n", ""))
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// arscSinBase: `aapt2 compile --dir res -o c.zip` + `aapt2 link --no-auto-version` de build-tools
// 35.0.1 sobre el proyecto del test y `unzip -p out.apk resources.arsc | base64 -w0`.
const arscSinBase = `AgAMALQCAAABAAAAAQAcADAAAAACAAAAAAAAAAABAAAkAAAAAAAAAAAAAAAHAAAABARCYXNlAAICRVMAAAIgAXgCAAB/AAAA
YwBvAG0ALgBzAGkAbgBiAGEAcwBlAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACABAAAAAAAAaAEAAAAAAAAAAAAAAQAcAEgAAAADAAAA
AAAAAAAAAAAoAAAAAAAAAAAAAAAIAAAAEAAAAAIAPwAxAAAAAgA/ADIAAAAGAHMAdAByAGkAbgBnAAAAAQAcACwAAAABAAAA
AAAAAAABAAAgAAAAAAAAAAAAAAAICGFwcF9uYW1lAAACAhAAFAAAAAMAAgABAAAABAAAAAECVABoAAAAAwAAAAEAAABYAAAA
QAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAA
AAAAAAgAAAMAAAAAAQJUAGgAAAADAAAAAQAAAFgAAABAAAAAAAAAAGVzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwEAAAA=`
