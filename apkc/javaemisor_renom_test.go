package main

import "testing"

// TestDexElDesalojoNoRompeLosLocalesAliasados: dos locales fusionados al mismo registro por una
// asignación `y = x` lo comparten. Al desalojarlo, todos los alias tienen que pasar al nuevo
// registro; si `localDeFijo` devuelve uno solo y se mueve solo ese, el otro queda nombrando un
// registro que se reutiliza en cuanto se libera.
func TestDexElDesalojoNoRompeLosLocalesAliasados(t *testing.T) {
	p := &traduce{base: 20, techo: true, siguiente: 16, locales: 3, m: classMethod{ArgCount: 1}}
	p.fijos = map[int]bool{3: true}
	p.kindFijo = map[int]byte{3: opMueve}
	p.renom = map[int]int{1: 3, 2: 3}
	if !p.liberaRegistro(jvmInstr{name: "iload"}, 3) {
		t.Fatal("no desalojó el registro compartido")
	}
	if p.renom[1] != p.renom[2] {
		t.Errorf("los dos locales compartían v3 y quedan en v%d y v%d", p.renom[1], p.renom[2])
	}
	if p.fijos[3] {
		t.Error("v3 sigue atado después de desalojarlo")
	}
}
