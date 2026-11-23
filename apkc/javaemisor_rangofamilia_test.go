package main

// La familia `/range` de las invocaciones, completa: `invoke-virtual/range` 0x74 y
// `invoke-static/range` 0x77 ya estaban cubiertos por `TestLasFormasRange` (parrafo 90); esta sonda
// anade los tres que faltaban por forzar —`invoke-super/range` 0x75, `invoke-direct/range` 0x76 e
// `invoke-interface/range` 0x78— para que el siguiente opcode de la familia no sea una sorpresa.
// Las cinco formas salen del mismo `invocaRango` (3rc): la unidad del opcode con el recuento de
// PALABRAS y el primer registro del bloque contiguo.

import "testing"

const fuenteIfaceRango = `interface SeisIface {
    int seis(int a, int b, int c, int d, int e, int f);
}
`

const fuenteBaseRango = `class BaseRango {
    int sup(int a, int b, int c, int d, int e, int f) {
        return a + b + c + d + e + f;
    }
}
`

// priv (invokespecial a un metodo privado) llama a invoke-direct/range; super.sup a invoke-super/range;
// `SeisIface yo = this; yo.seis(...)` fuerza el invokeinterface del receptor estatico de interface, que
// con seis enteros pasa de cinco palabras y va por /range.
const fuenteRangoRango = `public class RangoRango extends BaseRango implements SeisIface {
    private int priv(int a, int b, int c, int d, int e, int f) {
        return a + b + c + d + e + f;
    }
    public int llamaPriv() {
        return priv(1, 2, 3, 4, 5, 6);
    }
    public int llamaSuper() {
        return super.sup(1, 2, 3, 4, 5, 6);
    }
    public int llamaIface() {
        SeisIface yo = this;
        return yo.seis(1, 2, 3, 4, 5, 6);
    }
    public int seis(int a, int b, int c, int d, int e, int f) {
        return a + b + c + d + e + f;
    }
}
`

// TestInvokeFormasRangeCompletas: cada llamada de siete palabras (receptor mas seis enteros) sale
// con el opcode 3rc de su forma. Si `invocaRango` dejara de convertir una, o el reparto de
// `invokespecial` dejara de distinguir el padre del metodo privado, la prueba lo nombra.
func TestInvokeFormasRangeCompletas(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/SeisIface.java", contenido: fuenteIfaceRango},
		{nombre: "src/BaseRango.java", contenido: fuenteBaseRango},
		{nombre: "src/RangoRango.java", contenido: fuenteRangoRango},
	}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestras := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase %s no se lee: %v", c.interno, err)
		}
		nuestras = append(nuestras, cf)
	}
	rango, ok := clasePorNombre(nuestras, "LRangoRango;")
	if !ok {
		t.Fatalf("no salió LRangoRango; de %d clases", len(nuestras))
	}
	td := newDexTables()
	if err := td.collectClasses(nuestras); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{
		"llamaPriv":  0x76, // invoke-direct/range
		"llamaSuper": 0x75, // invoke-super/range
		"llamaIface": 0x78, // invoke-interface/range
	}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(rango, nombre)
		if !ok {
			t.Fatalf("%s: no está en RangoRango", nombre)
		}
		ci, err := td.codeItemDe(rango, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := false
		for _, u := range ci.insns {
			if byte(u) == opcode {
				visto = true
				break
			}
		}
		if !visto {
			t.Errorf("%s: no aparece el opcode %#02x en %04x", nombre, opcode, ci.insns)
		}
	}
}
