package main

// El error de presupuesto (`toma` agota la base) no puede quedar sustituido por el «no low register is
// free» de `tomaBajo`: `falla` solo guarda el PRIMER error, así que el de presupuesto, que se fija
// antes, sobrevive y `codeItemConPc` sube la base en vez de rendirse. No hay ningún `p.err = nil` en
// el traductor, así que la sustitución no se puede reproducir por la vía de `tomaBajo`; estos tests
// fijan el comportamiento.

import (
	"strings"
	"testing"
)

// TestDexTomaBajoConservaElErrorDePresupuesto: con la base agotada, `tomaBajo` devuelve el error de
// presupuesto del primer `toma`, no el suyo.
func TestDexTomaBajoConservaElErrorDePresupuesto(t *testing.T) {
	p := &traduce{base: 16, techo: true, siguiente: 16}
	if v := p.tomaBajo(jvmInstr{name: "iload"}, 15); v != 0 {
		t.Fatalf("devolvió v%d con la base agotada", v)
	}
	if p.err == nil || !esPresupuesto(p.err) {
		t.Fatalf("el error no es de presupuesto: %v", p.err)
	}
	if strings.Contains(p.err.Error(), "no low register") {
		t.Errorf("el error de presupuesto se pisó: %v", p.err)
	}
}

// TestDexLiberaRegistroConservaElErrorDePresupuesto: `liberaRegistro` pide un temporal alto para
// desalojar; si la base está agotada, deja el error de presupuesto (y no libera nada).
func TestDexLiberaRegistroConservaElErrorDePresupuesto(t *testing.T) {
	p := &traduce{base: 18, techo: true, siguiente: 18}
	p.enPila = []int{3}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{false}
	p.pilaMove = []byte{opMueve}
	if p.liberaRegistro(jvmInstr{name: "iload"}, 3) {
		t.Fatal("liberó un registro cuando no quedaba presupuesto")
	}
	if p.err == nil || !esPresupuesto(p.err) {
		t.Fatalf("el error no es de presupuesto: %v", p.err)
	}
	if strings.Contains(p.err.Error(), "no low register") {
		t.Errorf("el error de presupuesto se pisó: %v", p.err)
	}
}
