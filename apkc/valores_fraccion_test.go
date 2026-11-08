package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestLasFraccionesSonByteAByteComoAapt2: 0%, 10%, 25%, 50%, 75%, 100%, 150%, 50%p y 100%p. El
// fixture es el arsc de aapt2 (796 bytes, parrafo 105): radix 3 para <1, 0x100 para 1.0, radix 2
// para >1 y el bit bajo de %p.
func TestLasFraccionesSonByteAByteComoAapt2(t *testing.T) {
	valores := []fuenteXML{{
		nombre: "res/values/f.xml",
		contenido: []byte(`<resources>
<fraction name="a">0%</fraction><fraction name="b">10%</fraction><fraction name="c">25%</fraction>
<fraction name="d">50%</fraction><fraction name="e">75%</fraction><fraction name="f">100%</fraction>
<fraction name="g">150%</fraction><fraction name="h">50%p</fraction><fraction name="i">100%p</fraction>
</resources>`),
	}}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.frac"><application/></manifest>`)}
	salida, err := enlazarProyecto(valores, nil, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString("AgAMABwDAAABAAAAAQAcABwAAAAAAAAAAAAAAAABAAAcAAAAAAAAAAACIAH0AgAAfwAAAGMAbwBtAC4AZgByAGEAYwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAQAAAAAAAFQBAAAAAAAAAAAAAAEAHAA0AAAAAQAAAAAAAAAAAAAAIAAAAAAAAAAAAAAACABmAHIAYQBjAHQAaQBvAG4AAAABABwAZAAAAAkAAAAAAAAAAAEAAEAAAAAAAAAAAAAAAAQAAAAIAAAADAAAABAAAAAUAAAAGAAAABwAAAAgAAAAAQFhAAEBYgABAWMAAQFkAAEBZQABAWYAAQFnAAEBaAABAWkAAgIQADQAAAABAAEACQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAECVAAIAQAAAQAAAAkAAAB4AAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAIAAAADAAAABAAAAAUAAAAGAAAABwAAAAgAAAAAgAAAAAAAAACAAABgAAAAAIAAAAAQAAAAgAAAYwzcwMCAAAAAIAAAAIAAAGMAAAIAgAAAADAAAACAAABjAAAEAIAAAABAAAAAgAAAYwAABgCAAAAAUAAAAIAAAGAAEAAAgAAAAGAAAACAAABiAAwAAIAAAABwAAAAgAAAYxAABACAAAAAgAAAAIAAAGAQEAAA==")
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d", len(nuestro), len(fijado))
	}
}
