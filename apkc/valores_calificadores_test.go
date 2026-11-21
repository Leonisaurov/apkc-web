package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestNodpiTvdpiYSw600dp: cierra la frontera de la fila 5.17. aapt2 guarda nodpi como
// DENSITY_ANY (0xffff) y tvdpi como 213, ambos con sdkVersion 4; `sw600dp` escribe 600 en
// smallestScreenWidthDp (+30 del ResTable_config) con sdkVersion 13. La tabla tiene que salir
// byte a byte la suya (§159).
func TestNodpiTvdpiYSw600dp(t *testing.T) {
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.qual"><application/></manifest>`)}
	layout := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`
	layouts := []fuenteXML{
		{nombre: "res/layout/main.xml", contenido: []byte(layout)},
		{nombre: "res/layout-sw600dp-v13/main.xml", contenido: []byte(layout), config: "sw600dp"},
	}
	png := []byte{0x89, 'P', 'N', 'G'}
	ficheros := []fuenteXML{
		{nombre: "res/drawable/i.png", contenido: png},
		{nombre: "res/drawable-nodpi-v4/i.png", contenido: png, config: "nodpi"},
		{nombre: "res/drawable-tvdpi-v4/i.png", contenido: png, config: "tvdpi"},
	}
	salida, err := enlazarProyecto(nil, layouts, ficheros, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString(arscCalificadores)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// arscCalificadores: `aapt2 compile --dir res` + `aapt2 link --no-auto-version` del fuente del test
// (base, nodpi-v4, tvdpi-v4 y layout-sw600dp-v13) y `unzip -p out.apk resources.arsc | base64 -w0`.
const arscCalificadores = "AgAMAJAEAAABAAAAAQAcALwAAAAFAAAAAAAAAAABAAAwAAAAAAAAAAAAAAAeAAAAPAAAAFEAAABzAAAAGxtyZXMvZHJhd2FibGUtbm9kcGktdjQvaS5wbmcAGxtyZXMvZHJhd2FibGUtdHZkcGktdjQvaS5wbmcAEhJyZXMvZHJhd2FibGUvaS5wbmcAHx9yZXMvbGF5b3V0LXN3NjAwZHAtdjEzL21haW4ueG1sABMTcmVzL2xheW91dC9tYWluLnhtbAAAAAAAAiAByAMAAH8AAABjAG8AbQAuAHEAdQBhAGwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAEAAAAAAABoAQAAAAAAAAAAAAABABwASAAAAAIAAAAAAAAAAAAAACQAAAAAAAAAAAAAABQAAAAIAGQAcgBhAHcAYQBiAGwAZQAAAAYAbABhAHkAbwB1AHQAAAABABwAMAAAAAIAAAAAAAAAAAEAACQAAAAAAAAAAAAAAAQAAAABAWkABARtYWluAAACAhAAFAAAAAEAAwABAAAAAAUAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMCAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAANUAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwEAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAA//8AAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADAAAAAAICEAAUAAAAAgACAAEAAAAAJAAAAQJUAGgAAAACAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAABAAAACAAAAwQAAAABAlQAaAAAAAIAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA0AAAAAAFgCAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAEAAAAIAAADAwAAAA=="
