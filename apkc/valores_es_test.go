package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestValuesEsEsByteAByteComoAapt2: la segunda configuración de las cadenas. El fixture es el
// `resources.arsc` que dejó aapt2 para un manifiesto con dos strings base y uno en `values-es`
// (728 bytes, parrafo 94). Lo que fija: language 'e','s' en +8/+9, la bandera 0x0004 del spec, el
// offset 0xffffffff de la clave que falta, y el orden del pool global (base ordenada y después es).
func TestValuesEsEsByteAByteComoAapt2(t *testing.T) {
	base := []byte(`<resources><string name="app_name">Icono</string><string name="otro">Uno</string></resources>`)
	es := []byte(`<resources><string name="app_name">Icono ES</string></resources>`)
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.idioma"><application android:label="@string/app_name"/></manifest>`)}
	valores := []fuenteXML{
		{nombre: "res/values/strings.xml", contenido: base},
		{nombre: "res/values-es/strings.xml", contenido: es, config: "es"},
	}
	salida, err := enlazarProyecto(valores, nil, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString("AgAMANgCAAABAAAAAQAcAEQAAAADAAAAAAAAAAABAAAoAAAAAAAAAAAAAAAIAAAADgAAAAUFSWNvbm8AAwNVbm8ACAhJY29ubyBFUwAAAAAAAiABiAIAAH8AAABjAG8AbQAuAGkAZABpAG8AbQBhAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAEAAAAAAABQAQAAAAAAAAAAAAABABwAMAAAAAEAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAYAcwB0AHIAaQBuAGcAAAABABwAOAAAAAIAAAAAAAAAAAEAACQAAAAAAAAAAAAAAAsAAAAICGFwcF9uYW1lAAQEb3RybwAAAAICEAAYAAAAAQACAAIAAAAEAAAAAAAAAAECVAB8AAAAAQAAAAIAAABcAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAACAAAAAAAAAAIAAADAAAAAAgAAAABAAAACAAAAwEAAAABAlQAbAAAAAEAAAACAAAAXAAAAEAAAAAAAAAAZXMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA/////wgAAAAAAAAACAAAAwIAAAA=")
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		difiere := -1
		for i := range nuestro {
			if i >= len(fijado) || nuestro[i] != fijado[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primer byte distinto es %d (%#02x contra %#02x)",
			len(nuestro), len(fijado), difiere, nuestro[difiere], fijado[difiere])
	}
}
