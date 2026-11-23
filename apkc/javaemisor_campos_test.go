package main

// La bajada de registros altos en los accesos a campo: `aseguraRegistro` copia un registro que no
// cabe en el formato a un temporal bajo con la forma 22v del move de su tipo, y `tomaParBajo` elige
// el par consecutivo más bajo. Es lo que usa `accesoCampo`/`accesoEstatico` para el objeto, el valor
// y el destino cuando caen por encima de v15 (o de v255 en la 21c).

import "testing"

// TestDexAseguraRegistroBajaUnAlto: un registro v16 con un temporal bajo libre se copia con
// `move-object/from16 v3, v16` y se devuelve v3. Falla si se revierte a usar el registro alto tal cual.
func TestDexAseguraRegistroBajaUnAlto(t *testing.T) {
	p := &traduce{base: 20, techo: true}
	p.libres = []int{3}
	nuevo, copia := p.aseguraRegistro(jvmInstr{name: "getfield"}, 16, 15, opMueveObjeto, false)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if !copia {
		t.Fatal("no copió un registro alto")
	}
	if nuevo > 15 {
		t.Errorf("devolvió v%d, que no cabe en cuatro bits", nuevo)
	}
	if len(p.w) != 2 || byte(p.w[0]) != 0x08 || p.w[1] != 16 {
		t.Errorf("el move-object/from16 no salió como se esperaba: %04x", p.w)
	}
}

// TestDexTomaParBajoEligeElParMasBajo: con los pares (2,3) y (9,10) libres, elige el más bajo.
func TestDexTomaParBajoEligeElParMasBajo(t *testing.T) {
	p := &traduce{base: 20, techo: true}
	p.libres = []int{9, 10, 2, 3}
	if got := p.tomaParBajo(jvmInstr{name: "iget-wide"}, 14); got != 2 {
		t.Errorf("eligió v%d, esperaba v2", got)
	}
}
