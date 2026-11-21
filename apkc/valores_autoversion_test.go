package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestAutoVersionLayoutV1: con --auto-version (el defecto de aapt2) cada layout base se duplica en
// la configuración v1, con sdkVersion 1 y la bandera 0x0400 en el spec (§171).
func TestAutoVersionLayoutV1(t *testing.T) {
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.v1"><application/></manifest>`)}
	layout := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`
	layouts := []fuenteXML{{nombre: "res/layout/main.xml", contenido: []byte(layout)}}
	salida, err := enlazarProyecto(nil, layouts, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14", autoVersion: true})
	if err != nil {
		t.Fatalf("el enlace no salió: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	fijado, err := base64.StdEncoding.DecodeString(arscAutoVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// arscAutoVersion: `aapt2 compile` de res/layout/main.xml y `aapt2 link` SIN --no-auto-version.
const arscAutoVersion = "AgAMALwCAAABAAAAAQAcAFQAAAACAAAAAAAAAAABAAAkAAAAAAAAAAAAAAAZAAAAFhZyZXMvbGF5b3V0LXYxL21haW4ueG1sABMTcmVzL2xheW91dC9tYWluLnhtbAAAAAIgAVwCAAB/AAAAYwBvAG0ALgB2ADEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACABAAAAAAAAUAEAAAAAAAAAAAAAAQAcADAAAAABAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAGAGwAYQB5AG8AdQB0AAAAAQAcACgAAAABAAAAAAAAAAABAAAgAAAAAAAAAAAAAAAEBG1haW4AAAICEAAUAAAAAQACAAEAAAAABAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwEAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADAAAAAA=="
