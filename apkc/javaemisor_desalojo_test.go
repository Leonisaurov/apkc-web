package main

// El desalojo de registros bajos: cuando los cuatro bits están todos ocupados y hace falta uno,
// `tomaBajo` devuelve un local atado a su hueco alto (o mueve un valor de la pila a un temporal alto)
// con la forma 22v de su move y usa el bajo que queda libre. d8 hace lo mismo; el caso normal —un
// bajo libre— no pasa por aquí.

import "testing"

// TestDexDesalojaUnLocalAtado: los dieciséis bajos están atados a locales y solo hay un temporal alto
// libre. `tomaBajo` desaloja el local 2 (v3) a su hueco canónico v21 con `move-object/from16 v21, v3`
// y devuelve v3. Falla si se quita el desalojo.
func TestDexDesalojaUnLocalAtado(t *testing.T) {
	p := &traduce{base: 20, techo: true, siguiente: 16, locales: 1, m: classMethod{ArgCount: 1}}
	p.fijos = map[int]bool{}
	for i := 0; i <= 15; i++ {
		p.fijos[i] = true
	}
	p.kindFijo = map[int]byte{3: opMueveObjeto}
	p.renom = map[int]int{2: 3}
	nuevo := p.tomaBajo(jvmInstr{name: "iload"}, 15)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if nuevo != 3 {
		t.Errorf("devolvió v%d, esperaba v3", nuevo)
	}
	if p.renom[2] != 21 {
		t.Errorf("el local 2 quedó en v%d, esperaba v21", p.renom[2])
	}
	if p.fijos[3] {
		t.Error("v3 sigue atado después de desalojarlo")
	}
	if len(p.w) != 2 || byte(p.w[0]) != 0x08 || p.w[1] != 3 {
		t.Errorf("el move-object/from16 no salió como se esperaba: %04x", p.w)
	}
}

// TestDexDesalojaUnValorDeLaPila: si el bajo lo ocupa un valor de la pila (una referencia marcada),
// `tomaBajo` lo mueve a un temporal alto con su move y usa el bajo.
func TestDexDesalojaUnValorDeLaPila(t *testing.T) {
	p := &traduce{base: 20, techo: true, siguiente: 16}
	p.enPila = []int{3}
	p.pilaTipo = []byte{0}
	p.pilaAncho = []bool{false}
	p.pilaMove = []byte{opMueveObjeto}
	nuevo := p.tomaBajo(jvmInstr{name: "ifne"}, 15)
	if p.err != nil {
		t.Fatalf("%v", p.err)
	}
	if nuevo != 3 {
		t.Errorf("devolvió v%d, esperaba v3", nuevo)
	}
	if p.enPila[0] != 16 {
		t.Errorf("la pila quedó en v%d, esperaba v16", p.enPila[0])
	}
	if len(p.w) != 2 || byte(p.w[0]) != 0x08 || p.w[1] != 3 {
		t.Errorf("el move-object/from16 no salió como se esperaba: %04x", p.w)
	}
}
