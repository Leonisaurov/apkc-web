package main

// La forma `wide` del `.class` (0xc4), que este caminante no conocía y que destapó el fixture
// `tries` del banco de ART: `iinc 1, 1000` —javac la escribe así en cuanto la constante no cabe en
// un byte con signo— llega como `c4 84 00 01 03 e8` y salía como «opcode outside the table this
// fork measured». El oráculo del destino es `d8`: la traduce a `add-int/lit16 vA, vA, #+lit16`
// (`d000 e803` en su fichero, medido con build-tools 36.0.0).

import (
	"testing"
)

// TestElCaminanteLeeElWideDeIinc: los cinco bytes del `.class` salen como una instrucción `iinc_w`
// con el índice y la constante en dos bytes cada uno. Sin la rama del 0xc4 el caminante falla con
// «opcode outside the table», que es el defecto medido.
func TestElCaminanteLeeElWideDeIinc(t *testing.T) {
	lista, err := walkJvmCode([]byte{0xc4, 0x84, 0x00, 0x01, 0x03, 0xe8})
	if err != nil {
		t.Fatalf("el caminante rechaza el wide: %v", err)
	}
	if len(lista) != 1 {
		t.Fatalf("%d instrucciones, esperaba una", len(lista))
	}
	in := lista[0]
	if in.name != "iinc_w" || in.op != 0x84 || !in.ancho {
		t.Errorf("instrucción %q (op %#02x, ancho %v), esperaba iinc_w", in.name, in.op, in.ancho)
	}
	if len(in.ops) != 4 || int(indiceLocal(in)) != 1 ||
		int(int16(uint16(in.ops[2])<<8|uint16(in.ops[3]))) != 1000 {
		t.Errorf("operandos % x, esperaba índice 1 y constante 1000", in.ops)
	}
}

// TestElIincAnchoSaleComoAddIntLit16: una constante que no cabe en un byte con signo va con la 22s
// (`add-int/lit16`), no con la 22b (`add-int/lit8`). Es la forma que escribe `d8` en `aritmetica`.
func TestElIincAnchoSaleComoAddIntLit16(t *testing.T) {
	p := &traduce{techo: true, m: classMethod{ArgCount: 1}, locales: 1, base: 10}
	p.incrementa(jvmInstr{name: "iinc_w", ancho: true, ops: []byte{0x00, 0x01, 0x03, 0xe8}})
	if p.err != nil {
		t.Fatalf("no se tradujo: %v", p.err)
	}
	// el local 1 de este método de un argumento cae en v10: la 22s lleva el registro en el byte bajo
	if len(p.w) != 2 || p.w[0] != 0x0ad0 || p.w[1] != 0x03e8 {
		t.Errorf("palabras %04x, esperaba 0ad0 03e8", p.w)
	}
	// la forma corta sigue siendo lit8: con el mismo local, `d80a 010a`
	q := &traduce{techo: true, m: classMethod{ArgCount: 1}, locales: 1, base: 10}
	q.incrementa(jvmInstr{name: "iinc", ops: []byte{0x01, 0x01}})
	if len(q.w) != 2 || q.w[0] != 0x0ad8 || q.w[1] != 0x010a {
		t.Errorf("palabras %04x, esperaba 0ad8 010a", q.w)
	}
}
