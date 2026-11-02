package main

import "testing"

// TestLosPrivadosYLosAccessEnElDex: en DEX un metodo privado de instancia es un metodo **directo**
// (`invoke-direct` lo llama y el verificador espera verlo en esa lista: sin esto `dexdump` corta con
// `Direct/virtual method ... not in expected list`), y los accesores salen con las formas medidas en
// el parrafo 72: `iget` en el de lectura, `iput` + `return` en el de escritura, `invoke-direct` para
// el metodo privado de instancia y `invoke-static` para los estaticos y los campos estaticos.
func TestLosPrivadosYLosAccessEnElDex(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/P.java", contenido: fuentePrivada}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	clases := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", c.interno, err)
		}
		clases = append(clases, cf)
	}
	datos, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("dexando: %v", err)
	}
	d, err := readDex(datos)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("leyendo el class_data: %v", err)
	}
	directos := map[string]bool{}
	for _, m := range porClase["LP;"].directs {
		directos[d.methods[m.methodIdx].name] = true
	}
	for _, n := range []string{"<init>", "<clinit>", "oculto", "estatico", "lee",
		"access$002", "access$000", "access$102", "access$202", "access$200", "access$300", "access$400", "access$500"} {
		if !directos[n] {
			t.Errorf("%s no esta en la lista directa del class_data", n)
		}
	}
	for _, n := range []string{"publico", "protegido", "anonima"} {
		if directos[n] {
			t.Errorf("%s es virtual y aparece en la lista directa", n)
		}
	}
	palabras := func(nombre string) []uint16 {
		ci, err := codeItemDeMetodo(d, nombre)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		return ci.insns
	}
	// lectura: iget vA, vB (0x52) y return vA (0x0f)
	if w := palabras("access$000"); len(w) < 3 || w[0]&0xff != 0x52 || w[len(w)-1]&0xff != 0x0f {
		t.Errorf("access$000 no es iget+return: %#04x", w)
	}
	// escritura: iput v1, v0 (0x59) y return v1 (0x0f), como las palabras de d8 sobre el access$002
	if w := palabras("access$002"); len(w) < 3 || w[0]&0xff != 0x59 ||
		int(w[0]>>8)&0xf != 1 || int(w[0]>>12)&0xf != 0 ||
		w[len(w)-1]&0xff != 0x0f || int(w[len(w)-1]>>8) != 1 {
		t.Errorf("access$002 no es iput v1, v0 + return v1: %#04x", w)
	}
	// el metodo privado de instancia, por invoke-direct (0x70): oculto y lee
	for _, n := range []string{"access$300", "access$500"} {
		if w := palabras(n); len(w) < 3 || w[0]&0xff != 0x70 {
			t.Errorf("%s no llama con invoke-direct: %#04x", n, w)
		}
	}
	// el privado estatico, por invoke-static (0x71) y sin receptor
	if w := palabras("access$400"); len(w) < 3 || w[0]&0xff != 0x71 {
		t.Errorf("access$400 no llama con invoke-static: %#04x", w)
	}
	// el campo estatico: sget (0x60) y return
	if w := palabras("access$200"); len(w) < 3 || w[0]&0xff != 0x60 || w[len(w)-1]&0xff != 0x0f {
		t.Errorf("access$200 no es sget+return: %#04x", w)
	}
}
