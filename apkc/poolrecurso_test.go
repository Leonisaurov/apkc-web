package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// TestEscribePoolSaleByteABiteComoAapt2: los tres pools del resources.arsc fijado en el parrafo
// 26 -global, nombres de tipo y claves- vuelven a salir idénticos cuando los escribe este
// fichero con las cadenas que el lector encontró en ellos. Es la pieza sobre la que se apoya el
// escritor de la tabla entera.
func TestEscribePoolSaleByteABiteComoAapt2(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString(arscTabla)
	if err != nil {
		t.Fatal(err)
	}
	// las posiciones estan medidas en el fichero: @12 global (UTF-8), @356 tipos (UTF-16),
	// @436 claves (UTF-8)
	for _, c := range []struct {
		nombre string
		off    int
		utf8   bool
	}{
		{"global", 12, true},
		{"nombres de tipo", 356, false},
		{"claves", 436, true},
	} {
		suyo := leePoolCrudo(b, c.off)
		p, err := leePool(b, c.off)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		nuestro := escribePool(p.cadenas, c.utf8)
		if !bytes.Equal(nuestro, suyo) {
			t.Errorf("%s pool: %d bytes written against %d of aapt2 (%q)",
				c.nombre, len(nuestro), len(suyo), strings.Join(p.cadenas, ", "))
		}
		if bool(p.utf8) != c.utf8 {
			t.Errorf("%s: the file says utf8=%v, the test expected %v", c.nombre, p.utf8, c.utf8)
		}
	}
}

// TestPoolIdayVuelta: una cadena larga (la forma de dos unidades de longitud) y una fuera del plano
// base (dos unidades utf-16) tienen que volver iguales por el lector.
func TestPoolIdayVuelta(t *testing.T) {
	larga := strings.Repeat("k", 200)
	for _, utf8 := range []bool{true, false} {
		b := escribePool([]string{"id", larga, "", "AppTheme"}, utf8)
		p, err := leePool(b, 0)
		if err != nil {
			t.Fatalf("utf8=%v: %v", utf8, err)
		}
		if len(p.cadenas) != 4 || p.cadenas[1] != larga || p.cadenas[2] != "" {
			t.Errorf("utf8=%v: round trip gave %d strings %q", utf8, len(p.cadenas), p.cadenas[1])
		}
	}
	astral := "\U0001F600fin"
	b := escribePool([]string{astral}, false)
	p, err := leePool(b, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.cadenas) != 1 {
		t.Fatalf("%d strings", len(p.cadenas))
	}
	// el lector devuelve las unidades sueltas (el par sustituto no es un rune de Go): lo que se
	// comprueba es que la longitud son DOS unidades por el caracter astral, no una
	if utf16Len(p.cadenas[0]) != utf16Len(astral) {
		t.Errorf("the astral string came back with %d units, written %d",
			utf16Len(p.cadenas[0]), utf16Len(astral))
	}
}
