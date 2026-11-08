package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// TestAttrsYStyleable: dos <attr> con format integer/color producen entradas complejas idénticas a
// aapt2 (596 bytes) y el <declare-styleable> produce la clase styleable de R.java. El arreglo se
// escribe `new int[] {…}` (no `{…}`) para que lo compile el propio emisor del fork.
func TestAttrsYStyleable(t *testing.T) {
	valores := []fuenteXML{{
		nombre: "res/values/attrs.xml",
		contenido: []byte(`<resources>
  <attr name="modo" format="integer"/>
  <attr name="tono" format="color"/>
  <declare-styleable name="Vista">
    <attr name="modo"/>
    <attr name="tono"/>
  </declare-styleable>
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
	fijado, err := base64.StdEncoding.DecodeString("AgAMAFQCAAABAAAAAQAcABwAAAAAAAAAAAAAAAABAAAcAAAAAAAAAAACIAEsAgAAfwAAAGMAbwBtAC4AYQB0AHQAcgBzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAQAAAAAAAEwBAAAAAAAAAAAAAAEAHAAsAAAAAQAAAAAAAAAAAAAAIAAAAAAAAAAAAAAABABhAHQAdAByAAAAAQAcADQAAAACAAAAAAAAAAABAAAkAAAAAAAAAAAAAAAHAAAABARtb2RvAAQEdG9ubwAAAAICEAAYAAAAAQABAAIAAAAAAAAAAAAAAAECVACUAAAAAQAAAAIAAABcAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAcAAAAEAABAAAAAAAAAAAAAQAAAAAAAAEIAAAQBAAAABAAAQABAAAAAAAAAAEAAAAAAAABCAAAEBAAAAA=")
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d", len(nuestro), len(fijado))
	}
	r, err := rDeTabla(salida.tabla)
	if err != nil {
		t.Fatalf("la R no se escribe: %v", err)
	}
	for _, quiero := range []string{
		"public static final class styleable {",
		"public static int[] Vista=new int[] {0x7f010000, 0x7f010001};",
		"public static final int Vista_modo=0;",
		"public static final int Vista_tono=1;",
	} {
		if !strings.Contains(r, quiero) {
			t.Errorf("R.java no contiene %q:\n%s", quiero, r)
		}
	}
}
