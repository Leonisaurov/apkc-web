package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestLosTiposDeValuesSonByteAByteComoAapt2: string-array, integer-array, plurals, bool y color en
// un solo values. El fixture es el resources.arsc de aapt2 (1152 bytes, parrafo 95). Fija los
// registros 0x01000001.. de los arrays, los ids de cantidad del plural (one 0x01000006, other
// 0x01000004) y el orden del fuente, no el ordenado por id.
func TestLosTiposDeValuesSonByteAByteComoAapt2(t *testing.T) {
	valores := []fuenteXML{{
		nombre: "res/values/tipos.xml",
		contenido: []byte(`<resources>
  <string-array name="nombres"><item>uno</item><item>dos</item></string-array>
  <integer-array name="numeros"><item>1</item><item>2</item></integer-array>
  <plurals name="cuenta"><item quantity="one">uno</item><item quantity="other">varios</item></plurals>
  <bool name="si">true</bool>
  <color name="rojo">#ff0000</color>
</resources>`),
	}}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.tipos"><application/></manifest>`)}
	salida, err := enlazarProyecto(valores, nil, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString("AgAMAIAEAAABAAAAAQAcAEAAAAADAAAAAAAAAAABAAAoAAAAAAAAAAAAAAAGAAAADAAAAAMDZG9zAAMDdW5vAAYGdmFyaW9zAAAAAAACIAE0BAAAfwAAAGMAbwBtAC4AdABpAHAAbwBzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAQAAAAAAAIgBAAAAAAAAAAAAAAEAHABoAAAABAAAAAAAAAAAAAAALAAAAAAAAAAAAAAADgAAABoAAAAoAAAABQBhAHIAcgBhAHkAAAAEAGIAbwBvAGwAAAAFAGMAbwBsAG8AcgAAAAcAcABsAHUAcgBhAGwAcwAAAAAAAQAcAFwAAAAFAAAAAAAAAAABAAAwAAAAAAAAAAAAAAAKAAAAFAAAABkAAAAgAAAABwdub21icmVzAAcHbnVtZXJvcwACAnNpAAQEcm9qbwAGBmN1ZW50YQAAAAACAhAAGAAAAAEAAQACAAAAAAAAAAAAAAABAlQArAAAAAEAAAACAAAAXAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKAAAABAAAQAAAAAAAAAAAAIAAAABAAABCAAAAwEAAAACAAABCAAAAwAAAAAQAAEAAQAAAAAAAAACAAAAAQAAAQgAABABAAAAAgAAAQgAABACAAAAAgIQABQAAAACAAEAAQAAAAAAAAABAlQAaAAAAAIAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAIAAAAIAAAS/////wICEAAUAAAAAwABAAEAAAAAAAAAAQJUAGgAAAADAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAADAAAACAAAHQAA//8CAhAAFAAAAAQAAQABAAAAAAAAAAECVACAAAAABAAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAEABAAAAAAAAAACAAAABgAAAQgAAAMBAAAABAAAAQgAAAMCAAAA")
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
