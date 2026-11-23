package main

import "testing"

// TestDexEnSitioMarcaLaMitadAlta: `int-to-long vA, vA` deja un par ancho vivo (vA, vA+1). La mitad
// alta tiene que quedar en `altos` y fuera de `libres`: `toma` devuelve de `libres` sin mirar
// `altos`, así que si se queda en la bolsa el siguiente temporal la reutiliza y corrompe el long.
// Falla si se quita `marcaPar`/la retirada de `libres` (el padre de `6a23468` usaba `tomaPar`).
func TestDexEnSitioMarcaLaMitadAlta(t *testing.T) {
	p := &traduce{base: 20, techo: true, siguiente: 18}
	p.libres = []int{3}
	p.enPila = []int{2}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{false}
	p.pilaMove = []byte{opMueve}
	p.convierteEnAncho(jvmInstr{name: "i2l"}, opIntALargo)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if !p.altos[3] {
		t.Errorf("la mitad alta v3 no quedó marcada: altos=%v libres=%v", p.altos, p.libres)
	}
	if got := p.toma(jvmInstr{name: "siguiente"}); got == 3 {
		t.Errorf("el siguiente toma devolvió v3, que es la mitad alta del par vivo")
	}
}
