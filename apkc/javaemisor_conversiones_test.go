package main

// Las conversiones numéricas de 12x cuando la fuente o el destino caen por encima de v15: la fuente
// se copia a un temporal bajo con el move de su tipo y el destino se pide bajo (`tomaBajo`/
// `tomaParBajo`). En el caso normal los registros ya caben y sale el mismo `unidad12x` de siempre.

import "testing"

// TestDexConvierteEnAnchoConFuenteAlta: `i2l` con el int en v17 y v2 libre baja la fuente con
// `move/from16 v2, v17` y niega en el sitio: `int-to-long v2, v2`. Falla si se vuelve a comprobar el
// registro alto con `nibble` sin bajarlo.
func TestDexConvierteEnAnchoConFuenteAlta(t *testing.T) {
	p := &traduce{base: 30, techo: true, siguiente: 18}
	p.libres = []int{2}
	p.enPila = []int{17}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{false}
	p.pilaMove = []byte{opMueve}
	p.convierteEnAncho(jvmInstr{name: "i2l"}, opIntALargo)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if len(p.w) != 3 || p.w[0] != 0x0202 || p.w[1] != 17 || p.w[2] != 0x2281 {
		t.Errorf("esperaba move/from16 v2, v17 + int-to-long v2, v2; salió %04x", p.w)
	}
}

// TestDexConviertePalabraConFuenteAlta: `i2f` con el int en v17 baja la fuente y convierte en el sitio.
func TestDexConviertePalabraConFuenteAlta(t *testing.T) {
	p := &traduce{base: 30, techo: true, siguiente: 18}
	p.libres = []int{2, 3}
	p.enPila = []int{17}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{false}
	p.pilaMove = []byte{opMueve}
	p.conviertePalabra(jvmInstr{name: "i2f"}, 0x82)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if len(p.w) != 3 || p.w[0] != 0x0302 || p.w[1] != 17 || p.w[2] != 0x3282 {
		t.Errorf("esperaba move/from16 v3, v17 + int-to-float v2, v3; salió %04x", p.w)
	}
}

// TestDexConvierteDeAnchoConFuenteAlta: `l2i` con el long en el par v17 y el par (2,3) libre baja la
// fuente con `move-wide/from16 v2, v17` y convierte a un destino bajo.
func TestDexConvierteDeAnchoConFuenteAlta(t *testing.T) {
	p := &traduce{base: 30, techo: true, siguiente: 18}
	p.libres = []int{2, 3, 4}
	p.enPila = []int{17}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{true}
	p.pilaMove = []byte{opMueveWide}
	p.convierteDeAncho(jvmInstr{name: "l2i"}, opLargoAInt)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if len(p.w) != 3 || p.w[0] != 0x0205 || p.w[1] != 17 || p.w[2] != 0x2484 {
		t.Errorf("esperaba move-wide/from16 v2, v17 + long-to-int v4, v2; salió %04x", p.w)
	}
}
