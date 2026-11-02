package main

import (
	"encoding/base64"
	"testing"
)

// TestElLargoSeDexaComoElDeJavacYComoD8: el dex que escribe este fork sobre nuestras clases `long` es
// el mismo byte a byte que el que escribe sobre las de javac (el oráculo doble de siempre), y el
// cuerpo de `imprime:(J)V` sale con las palabras exactas que midió `d8`: `6200 0100` (sget-object), `6e30 0600
// 1002` (invoke-virtual {v0, v1, v2} —el argumento largo aporta DOS registros—) y `0e00`, con
// `registers=3, ins=2, outs=3`. Donde este traductor elige otra forma se dice aquí y no se esconde:
// para `a + b * 2L` d8 escribe `mul-long` de tres registros (`9d04 0400`) y nosotros
// `mul-long/2addr` (`bd04`) encima del par muerto; y en `local` d8 pliega `1L + 300L` en una sola
// constante (parrafo 74).
func TestElLargoSeDexaComoElDeJavacYComoD8(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/L.java", contenido: fuenteLargo}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestras := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase no se lee: %v", err)
		}
		nuestras = append(nuestras, cf)
	}
	datos, err := base64.StdEncoding.DecodeString(largoJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	deNuestras, err := dexDeClases(nuestras)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deSuya, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if string(deNuestras) != string(deSuya) {
		t.Errorf("los dos dex difieren: %d bytes contra %d", len(deNuestras), len(deSuya))
	}
	d, err := readDex(deNuestras)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	ci, err := codeItemDeMetodo(d, "imprime")
	if err != nil {
		t.Fatalf("imprime: %v", err)
	}
	if ci.registersSize != 3 || ci.insSize != 2 || ci.outsSize != 3 {
		t.Errorf("imprime: la cabecera es %d/%d/%d y la que midio d8 es 3/2/3",
			ci.registersSize, ci.insSize, ci.outsSize)
	}
	quieren := []uint16{0x0062, 0x0001, 0x306e, 0x0006, 0x0210, 0x000e}
	if len(ci.insns) != len(quieren) {
		t.Fatalf("imprime: %d palabras (%#04x) y las de d8 %d", len(ci.insns), ci.insns, len(quieren))
	}
	for i, u := range quieren {
		if ci.insns[i] != u {
			t.Errorf("imprime: la palabra %d es %#04x y la que midio d8 %#04x", i, ci.insns[i], u)
		}
	}
	// y el `return-wide` (0x10) al final de `cuenta`, con los cuatro registros de los dos argumentos
	ci, err = codeItemDeMetodo(d, "cuenta")
	if err != nil {
		t.Fatalf("cuenta: %v", err)
	}
	if ci.insSize != 4 {
		t.Errorf("cuenta: ins=%d y dos argumentos largos valen cuatro palabras", ci.insSize)
	}
	if ultima := ci.insns[len(ci.insns)-1] & 0xff; ultima != 0x10 {
		t.Errorf("cuenta: la ultima palabra es %#04x y el return-wide es 0x10", ci.insns[len(ci.insns)-1])
	}
}
