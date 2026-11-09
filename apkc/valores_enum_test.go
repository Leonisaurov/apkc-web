package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestAttrEnumYFlagsConHijos: `format="enum"` y `format="flags"` con hijos producen el mapa del
// atributo (la marca de formato y un registro por hijo con el id de su recurso `id`) y los recursos
// `id`, byte a byte como aapt2 (§135).
func TestAttrEnumYFlagsConHijos(t *testing.T) {
	valores := []fuenteXML{{
		nombre: "res/values/attrs.xml",
		contenido: []byte(`<resources>
  <attr name="modo" format="enum">
    <enum name="uno" value="1" />
    <enum name="dos" value="2" />
  </attr>
  <attr name="banderas" format="flags">
    <flag name="alta" value="1" />
    <flag name="baja" value="2" />
  </attr>
</resources>`),
	}}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.attrs"><application/></manifest>`)}
	salida, err := enlazarProyecto(valores, nil, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString(arscEnumJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// arscEnumJavac: `aapt2 compile` + `aapt2 link --no-auto-version` del fuente del test y
// `unzip -p app.apk resources.arsc | base64 -w0`.
const arscEnumJavac = "AgAMAIADAAABAAAAAQAcABwAAAAAAAAAAAAAAAABAAAcAAAAAAAAAAACIAFYAwAAfwAAAGMAbwBtAC4AYQB0AHQAcgBzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAQAAAAAAAFgBAAAAAAAAAAAAAAEAHAA4AAAAAgAAAAAAAAAAAAAAJAAAAAAAAAAAAAAADAAAAAQAYQB0AHQAcgAAAAIAaQBkAAAAAQAcAGAAAAAGAAAAAAAAAAABAAA0AAAAAAAAAAAAAAALAAAAEgAAABkAAAAgAAAAJgAAAAgIYmFuZGVyYXMABARtb2RvAAQEYWx0YQAEBGJhamEAAwNkb3MAAwN1bm8AAgIQABgAAAABAAEAAgAAAAAAAAAAAAAAAQJUAMQAAAABAAAAAgAAAFwAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQAAAAQAAEAAAAAAAAAAAADAAAAAAAAAQgAABAAAAIAAAACfwgAABABAAAAAQACfwgAABACAAAAEAABAAEAAAAAAAAAAwAAAAAAAAEIAAAQAAABAAIAAn8IAAAQAgAAAAMAAn8IAAAQAQAAAAICEAAgAAAAAgABAAQAAAAAAAAAAAAAAAAAAAAAAAAAAQJUAKQAAAACAAAABAAAAGQAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAAAAAgABAACAAAACAAAEgAAAAAIAAQAAwAAAAgAABIAAAAACAAEAAQAAAAIAAASAAAAAAgABAAFAAAACAAAEgAAAAA="
