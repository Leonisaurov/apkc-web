package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestValuesEsRmxEsByteAByteComoAapt2: `values-es-rMX` es language 'e','s' en +8/+9 y country
// 'M','X' en +10/+11; el arsc de 680 bytes sale idéntico al de aapt2 (parrafo 107).
func TestValuesEsRmxEsByteAByteComoAapt2(t *testing.T) {
	valores := []fuenteXML{
		{nombre: "res/values/strings.xml", contenido: []byte(`<resources><string name="app_name">Region</string></resources>`)},
		{nombre: "res/values-es-rMX/strings.xml", contenido: []byte(`<resources><string name="app_name">Region MX</string></resources>`), config: "es-rMX"},
	}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.region"><application android:label="@string/app_name"/></manifest>`)}
	salida, err := enlazarProyecto(valores, nil, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString("AgAMAKgCAAABAAAAAQAcADwAAAACAAAAAAAAAAABAAAkAAAAAAAAAAAAAAAJAAAABgZSZWdpb24ACQlSZWdpb24gTVgAAAAAAAIgAWACAAB/AAAAYwBvAG0ALgByAGUAZwBpAG8AbgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACABAAAAAAAAUAEAAAAAAAAAAAAAAQAcADAAAAABAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAGAHMAdAByAGkAbgBnAAAAAQAcACwAAAABAAAAAAAAAAABAAAgAAAAAAAAAAAAAAAICGFwcF9uYW1lAAACAhAAFAAAAAEAAgABAAAABAAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMAAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAGVzTVgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwEAAAA=")
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d", len(nuestro), len(fijado))
	}
}
