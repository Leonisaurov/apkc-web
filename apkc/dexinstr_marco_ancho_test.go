package main

// El registro ancho tiene que caber DENTRO del marco. `convierteEnAncho` reutilizaba el registro del
// `int` como mitad baja del par (`int-to-long vA, vA`) sin comprobar que `vA+1` existiera: `ocupado`
// indexa rejillas dispersas y contesta «libre» para cualquier registro fuera de `registers_size`, así
// que un `long id(int p){return p;}` salía `registers: 2` con `int-to-long v1, v1` y ART rechazaba la
// clase entera con `wide register index out of range (1+1 >= 2)`. `d8` sobre el mismo `.class` da
// `registers=4` con `int-to-long v0, v3`.

import (
	"encoding/base64"
	"testing"
)

// anchoJavac17: `javac --release 17 -g Ancho.java` (mayor de clase 61) y `base64 -w0 Ancho.class`,
// con `class Ancho { long id(int p) { return p; } }`. Es el caso mínimo que reprodujo el defecto.
const anchoJavac17 = "yv66vgAAAD0ADwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAFQW5jaG8BAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQACaWQBAAQoSSlKAQAKU291cmNlRmlsZQEACkFuY2hvLmphdmEAIQAHAAIAAAAAAAIAAQAFAAYAAQAJAAAAHQABAAEAAAAFKrcAAbEAAAABAAoAAAAGAAEAAAABAAAACwAMAAEACQAAABsAAgACAAAAAxuFrQAAAAEACgAAAAYAAQAAAAIAAQANAAAAAgAO"

// TestDexConvierteEnAnchoNoUsaElSitioFueraDelMarco: con `fuente` en el ÚLTIMO registro del
// marco (v3 de un marco de cuatro), `v3+1` no existe y el par tiene que salir de un par bajo, no del
// sitio. Sin la comprobación `fuente+1 < base+locales` se emite `int-to-long v3, v3` (0x3381) y el
// par (v3,v4) se sale de `registers_size`; con ella sale `int-to-long v0, v3` (0x3081), que cabe.
func TestDexConvierteEnAnchoNoUsaElSitioFueraDelMarco(t *testing.T) {
	p := &traduce{base: 0, locales: 4, m: classMethod{ArgCount: 2}}
	p.enPila = []int{3}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{false}
	p.pilaMove = []byte{opMueve}
	p.convierteEnAncho(jvmInstr{name: "i2l"}, opIntALargo)
	if p.err != nil {
		t.Fatalf("no se tradujo: %v", p.err)
	}
	if len(p.w) != 1 {
		t.Fatalf("esperaba una palabra, salieron %d: %04x", len(p.w), p.w)
	}
	destino := int(p.w[0]>>8) & 0xf
	fuente := int(p.w[0]>>12) & 0xf
	if p.w[0]&0xff != 0x81 || destino != 0 || fuente != 3 {
		t.Fatalf("esperaba int-to-long v0, v3 (0x3081) y salió %04x", p.w[0])
	}
	if destino+1 >= p.base+p.locales {
		t.Errorf("el destino v%d deja la mitad alta v%d fuera de un marco de %d registros",
			destino, destino+1, p.base+p.locales)
	}
}

// TestDexAnchoDeJavacDejaElParDentroDelMarco: el `.class` de javac 17 dexado por nosotros deja el
// par (v0,v1) dentro de un marco de cuatro y la fuente en v3, el mismo `int-to-long v0, v3` que d8,
// con `return-wide v0`. Es la regresión del `AppAdapter` de ApkShare y del `Ancho.id` mínimo.
func TestDexAnchoDeJavacDejaElParDentroDelMarco(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(anchoJavac17)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("la clase de javac no se lee: %v", err)
	}
	dex, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatalf("dexando la clase de javac: %v", err)
	}
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	ci, err := codeItemDeMetodo(d, "id")
	if err != nil {
		t.Fatalf("id: %v", err)
	}
	if ci.registersSize != 4 || ci.insSize != 2 {
		t.Errorf("cabecera %d/%d y d8 mide 4/2", ci.registersSize, ci.insSize)
	}
	if len(ci.insns) != 2 {
		t.Fatalf("esperaba dos palabras (int-to-long + return-wide) y salieron %d: %04x",
			len(ci.insns), ci.insns)
	}
	ancho := ci.insns[0]
	if ancho&0xff != 0x81 {
		t.Fatalf("la primera palabra es %04x y no un int-to-long (0x81)", ancho)
	}
	destino := int(ancho>>8) & 0xf
	fuente := int(ancho>>12) & 0xf
	if destino != 0 || fuente != 3 {
		t.Errorf("esperaba int-to-long v0, v3 y salió v%d, v%d", destino, fuente)
	}
	if destino+1 >= int(ci.registersSize) || fuente >= int(ci.registersSize) {
		t.Errorf("el par (v%d,v%d) o la fuente v%d se salen de registers_size=%d",
			destino, destino+1, fuente, ci.registersSize)
	}
	// `readCodeItem` guarda las palabras en el orden del fichero, así que `dexdump 1000` es `0x0010`
	if ci.insns[1] != 0x0010 {
		t.Errorf("esperaba return-wide v0 (0x0010) y salió %04x", ci.insns[1])
	}
}
