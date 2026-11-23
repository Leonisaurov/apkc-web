package main

import "testing"

// TestDexLosTryItemsVanOrdenados: la `exception_table` del `.class` no tiene por qué venir ordenada
// por `start_pc`; el DEX exige los `try_item` en orden ascendente de `start_addr`. Con dos rangos
// disjuntos en orden inverso, la tabla emitida tiene que salir ordenada. Falla si se quita el
// `sort.Slice` de `armaTries` (el `dexdump` del jar real lo cortaba con
// `Out-of_order try_item with start_addr`).
func TestDexLosTryItemsVanOrdenados(t *testing.T) {
	p := &traduce{
		t: &dexTables{typeIdx: map[string]int{"Ljava/lang/Exception;": 1}},
		m: classMethod{Name: "m", Desc: "()V", Tries: []classTry{
			{start: 20, end: 30, handler: 31, catchType: "Ljava/lang/Exception;"},
			{start: 0, end: 10, handler: 11, catchType: "Ljava/lang/Exception;"},
		}},
		pcs: map[int]int{0: 0, 10: 10, 20: 20, 30: 30, 11: 11, 31: 31},
	}
	items, _, err := armaTries(p, p.m)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(items) != 2 || items[0].start != 0 || items[1].start != 20 {
		t.Errorf("try_items desordenados: %+v", items)
	}
}
