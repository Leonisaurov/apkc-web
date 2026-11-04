package main

import (
	"testing"
)

// Fuentes del párrafo 86: marcos por encima de v15. Los argumentos van al FINAL de la frame (§57), así que
// con diecisiete `int` el último vive en v16 o más y ninguna forma de cuatro bits lo alcanza: el
// traductor lo baja a un temporal con la 22v que escribe `d8`. Cada fuente está recortada a una sola
// lectura para que el cuerpo entero sea predecible palabra por palabra.
const fuenteAltoEntero = `public class AltoE {
    static int marco(int a, int b, int c, int d, int e, int f, int g, int h,
                     int i, int j, int k, int l, int m, int n, int o, int q, int r) {
        return r;
    }
}
`

const fuenteAltoLargo = `public class AltoL {
    static long marco(long a, long b, long c, long d, long e, long f, long g, long h,
                      long i, long j, long k, long l, long m, long n, long o, long p,
                      long q, long r, long s) {
        return s;
    }
}
`

func insnsDelMarco(t *testing.T, fuente, clase string) codeItem {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/" + clase + ".java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	m, ok := buscaMetodo(cf, "marco")
	if !ok {
		t.Fatalf("%s: no está el método", clase)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	ci, err := td.codeItemDe(cf, m)
	if err != nil {
		t.Fatalf("traduciendo: %v", err)
	}
	return ci
}

// TestUnArgumentoAltoBajaConMoveFrom16: el cuerpo es leer el argumento alto y volverlo. La forma medida
// sobre `d8 --debug` (`0202 1200` = `move/from16 v2, v18`, parrafo 86) lleva el DESTINO en el byte alto de
// la primera palabra y la FUENTE de dieciséis bits en la segunda; el `return` sale del temporal bajo. Si
// el registro alto se hubiera recortado a nibble, la instrucción apuntaría a otro sitio y ART cortaría el
// método.
func TestUnArgumentoAltoBajaConMoveFrom16(t *testing.T) {
	ci := insnsDelMarco(t, fuenteAltoEntero, "AltoE")
	if ci.registersSize <= 17 {
		t.Fatalf("el marco salió de %d registros: la fuente no fuerza un argumento alto", ci.registersSize)
	}
	if byte(ci.insns[0]) != opMueveFrom16 {
		t.Fatalf("la primera palabra es %#04x y tenía que ser un `move/from16` (cuerpo %v)", ci.insns[0], ci.insns)
	}
	destino := int(ci.insns[0] >> 8)
	fuente := int(ci.insns[1])
	if destino > 15 {
		t.Errorf("la copia baja a v%d: el destino tenía que quedar dentro de las formas de cuatro bits", destino)
	}
	if fuente < 16 || fuente >= int(ci.registersSize) {
		t.Errorf("la fuente de la copia es v%d con un marco de %d registros", fuente, ci.registersSize)
	}
	if querido := uint16(opRetorna) | uint16(destino)<<8; ci.insns[2] != querido {
		t.Errorf("tras la copia viene %#04x y se esperaba `return v%d` (%#04x)", ci.insns[2], destino, querido)
	}
}

// TestUnaParejaAltaBajaConMoveWideFrom16: con diecinueve argumentos `long` (38 huecos) el último ocupa dos
// registros por encima de v14, así que la copia es `move-wide/from16` —medida en `D16c.guardaLargo` como
// `0501 1300`— y la pareja de destino tiene que caber entera en cuatro bits.
func TestUnaParejaAltaBajaConMoveWideFrom16(t *testing.T) {
	ci := insnsDelMarco(t, fuenteAltoLargo, "AltoL")
	if ci.registersSize <= 16 {
		t.Fatalf("el marco salió de %d registros: no fuerza una pareja alta", ci.registersSize)
	}
	if byte(ci.insns[0]) != opMueveWideFrom16 {
		t.Fatalf("la primera palabra es %#04x y tenía que ser un `move-wide/from16` (cuerpo %v)", ci.insns[0], ci.insns)
	}
	destino := int(ci.insns[0] >> 8)
	fuente := int(ci.insns[1])
	if destino > 14 {
		t.Errorf("move-wide/from16 v%d: la pareja de destino no cabría en cuatro bits", destino)
	}
	if fuente <= 14 || fuente+1 >= int(ci.registersSize) {
		t.Errorf("move-wide/from16 sobre v%d con un marco de %d registros", fuente, ci.registersSize)
	}
	if querido := uint16(opRetornaWide) | uint16(destino)<<8; ci.insns[2] != querido {
		t.Errorf("tras la copia viene %#04x y se esperaba `return-wide v%d` (%#04x)", ci.insns[2], destino, querido)
	}
}
