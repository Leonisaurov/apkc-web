package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestLayoutLandYDrawableHdpiSonByteAByteComoAapt2: la segunda configuracion de orientacion y la de
// densidad. El fixture es el resources.arsc de `aapt2 link --no-auto-version` (1204 bytes, parrafo
// 103): `layout-land` es orientation=2 y `drawable-hdpi-v4` es density=240 con sdkVersion=4; el spec
// del layout lleva 0x0080 y el del drawable 0x0500.
func TestLayoutLandYDrawableHdpiSonByteAByteComoAapt2(t *testing.T) {
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.cal"><application android:label="@string/app_name"/></manifest>`)}
	valores := []fuenteXML{{nombre: "res/values/strings.xml", contenido: []byte(`<resources><string name="app_name">Cal</string></resources>`)}}
	layouts := []fuenteXML{
		{nombre: "res/layout/main.xml", contenido: []byte(`<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`)},
		{nombre: "res/layout-land/main.xml", contenido: []byte(`<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`), config: "land"},
	}
	png := []byte{0x89, 'P', 'N', 'G'}
	ficheros := []fuenteXML{
		{nombre: "res/drawable/icono.png", contenido: png},
		{nombre: "res/drawable-hdpi-v4/icono.png", contenido: png, config: "hdpi"},
	}
	salida, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString("AgAMALQEAAABAAAAAQAcAKQAAAAFAAAAAAAAAAABAAAwAAAAAAAAAAAAAAAGAAAAJwAAAEAAAABbAAAAAwNDYWwAHh5yZXMvZHJhd2FibGUtaGRwaS12NC9pY29uby5wbmcAFhZyZXMvZHJhd2FibGUvaWNvbm8ucG5nABgYcmVzL2xheW91dC1sYW5kL21haW4ueG1sABMTcmVzL2xheW91dC9tYWluLnhtbAAAAAAAAiABBAQAAH8AAABjAG8AbQAuAGMAYQBsAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAEAAAAAAAB8AQAAAAAAAAAAAAABABwAXAAAAAMAAAAAAAAAAAAAACgAAAAAAAAAAAAAABQAAAAkAAAACABkAHIAYQB3AGEAYgBsAGUAAAAGAGwAYQB5AG8AdQB0AAAABgBzAHQAcgBpAG4AZwAAAAEAHABEAAAAAwAAAAAAAAAAAQAAKAAAAAAAAAAAAAAACAAAAA8AAAAFBWljb25vAAQEbWFpbgAICGFwcF9uYW1lAAAAAgIQABQAAAABAAIAAQAAAAAFAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADAgAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAADwAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMBAAAAAgIQABQAAAACAAIAAQAAAIAAAAABAlQAaAAAAAIAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAEAAAAIAAADBAAAAAECVABoAAAAAgAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAQAAAAgAAAMDAAAAAgIQABQAAAADAAEAAQAAAAAAAAABAlQAaAAAAAMAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAIAAAAIAAADAAAAAA==")
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d", len(nuestro), len(fijado))
	}
}
