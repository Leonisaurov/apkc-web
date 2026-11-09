package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestDensidadesYOrientacionPort: las densidades ldpi/mdpi/xhdpi/xxhdpi y la orientación `port`,
// además de land, con el arsc byte a byte el de aapt2 (§137). El spec del drawable lleva 0x0500 y el
// del layout 0x0080; la densidad va en +14 y sdkVersion 4 en +24, y port es orientation=1 en +12.
func TestDensidadesYOrientacionPort(t *testing.T) {
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.dens"><application/></manifest>`)}
	layout := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`
	layouts := []fuenteXML{
		{nombre: "res/layout/main.xml", contenido: []byte(layout)},
		{nombre: "res/layout-land/main.xml", contenido: []byte(layout), config: "land"},
		{nombre: "res/layout-port/main.xml", contenido: []byte(layout), config: "port"},
	}
	png := []byte{0x89, 'P', 'N', 'G'}
	ficheros := []fuenteXML{
		{nombre: "res/drawable/i.png", contenido: png},
		{nombre: "res/drawable-ldpi-v4/i.png", contenido: png, config: "ldpi"},
		{nombre: "res/drawable-mdpi-v4/i.png", contenido: png, config: "mdpi"},
		{nombre: "res/drawable-xhdpi-v4/i.png", contenido: png, config: "xhdpi"},
		{nombre: "res/drawable-xxhdpi-v4/i.png", contenido: png, config: "xxhdpi"},
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
	fijado, err := base64.StdEncoding.DecodeString(arscDensidades)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// arscDensidades: `aapt2 compile` + `aapt2 link --no-auto-version` del fuente del test (base,
// ldpi/mdpi/xhdpi/xxhdpi y layout port/land) y `unzip -p app.apk resources.arsc | base64 -w0`.
const arscDensidades = "AgAMACAGAAABAAAAAQAcABQBAAAIAAAAAAAAAAABAAA8AAAAAAAAAAAAAAAdAAAAOgAAAFgAAAB3AAAAjAAAAKcAAADCAAAAGhpyZXMvZHJhd2FibGUtbGRwaS12NC9pLnBuZwAaGnJlcy9kcmF3YWJsZS1tZHBpLXY0L2kucG5nABsbcmVzL2RyYXdhYmxlLXhoZHBpLXY0L2kucG5nABwccmVzL2RyYXdhYmxlLXh4aGRwaS12NC9pLnBuZwASEnJlcy9kcmF3YWJsZS9pLnBuZwAYGHJlcy9sYXlvdXQtbGFuZC9tYWluLnhtbAAYGHJlcy9sYXlvdXQtcG9ydC9tYWluLnhtbAATE3Jlcy9sYXlvdXQvbWFpbi54bWwAAAIgAQAFAAB/AAAAYwBvAG0ALgBkAGUAbgBzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACABAAAAAAAAaAEAAAAAAAAAAAAAAQAcAEgAAAACAAAAAAAAAAAAAAAkAAAAAAAAAAAAAAAUAAAACABkAHIAYQB3AGEAYgBsAGUAAAAGAGwAYQB5AG8AdQB0AAAAAQAcADAAAAACAAAAAAAAAAABAAAkAAAAAAAAAAAAAAAEAAAAAQFpAAQEbWFpbgAAAgIQABQAAAABAAUAAQAAAAAFAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADBAAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAB4AAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMAAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAKAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwEAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAQAEAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADAgAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAADgAQAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMDAAAAAgIQABQAAAACAAMAAQAAAIAAAAABAlQAaAAAAAIAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAEAAAAIAAADBwAAAAECVABoAAAAAgAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAQAAAAgAAAMGAAAAAQJUAGgAAAACAAAAAQAAAFgAAABAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAABAAAACAAAAwUAAAA="
