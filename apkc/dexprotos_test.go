package main

import (
	"testing"
)

// TestLosProtosSeOrdenanPorSusParametros: dentro del mismo tipo de retorno, el `proto_id` se ordena por
// la LISTA DE PARÁMETROS comparada por los índices de tipo, no por el shorty. Medido en el aparato
// (Android 15, `dexdump`, 2026-11-03) con el dex de `D` y sus accesores: `(LD;J)J` va antes que
// `(LF;)J` —sus shortys son «JLJ» y «JL», que ordenarían al revés— y ART cortaba el fichero con
// `Out-of-order proto_id arguments`.
func TestLosProtosSeOrdenanPorSusParametros(t *testing.T) {
	td := newDexTables()
	// llegan en el orden del fuente, que no es el del formato
	for _, d := range []string{"(LF;)J", "(LD;J)J", "(LD;)J"} {
		td.internProto(d)
	}
	td.orderTables()
	querido := []string{"(LD;)J", "(LD;J)J", "(LF;)J"}
	if len(td.protos) != len(querido) {
		t.Fatalf("hay %d protos y se esperaban %d", len(td.protos), len(querido))
	}
	for i, q := range querido {
		p := td.protos[i]
		firma := "(" + p.params + ")" + p.ret
		if firma != q {
			t.Errorf("el proto %d es %s y tenía que ser %s", i, firma, q)
		}
	}
}
