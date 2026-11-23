package main

// Cuando un salto no cabe en dieciséis bits, dalvik tiene `goto/32` 0x2a (30t) para el
// `goto` y ninguna forma larga para el `if-*`: una condicional lejana se invierte y salta por encima
// de un `goto/32`. Medido con javac 17, d8 y dexdump sobre `Largo32`, con 4300 llamadas
// `consume(<long>L)` (7 bytes de JVM, 8 unidades de dex cada una):
//
//	condi:      3c02 0500              if-gtz v2, +5      y  2a00 6586 0000  goto/32 +34405
//	atras:      3d02 0500              if-lez v2, +5      y  2a00 9b79 ffff  goto/32 -34405
//	salta:      3c02 0500 / 2a00 ...   la guarda del while y 2a00 9879 ffff  goto/32 -34408 de vuelta
//	larguisimo: goto_w del .class a goto/32 +40009 (40010 unidades)
//
// El `.class` lo escribe nuestro emisor: en `condi`, `atras` y `salta` el desplazamiento de JVM cabe
// en 16 bits (30106, -30108 y -30110 bytes) y es el dexer el que tiene que promocionar; en
// `larguisimo` javac y nuestro emisor ya usan `goto_w` (35011 bytes) y el dexer lo baja a `goto/32`.

import (
	"fmt"
	"strings"
	"testing"
)

// fuenteSalto32: `grande` llamadas en `larguisimo` y 4300 en los otros tres. `consume` escribe un
// campo estático para que la llamada no sea eliminable; con `y = consume(<long>L)` la sentencia mide
// 7 bytes de JVM y 8 unidades de dex en d8 (y 10 en este traductor, que no funde el resultado con el
// local), así que 4300 dejan el salto de JVM en 30106 bytes —cabe— y el de dex en más de 32767
// unidades —no cabe—. `larguisimo` con 5000 pasa de 32767 bytes de JVM y fuerza el `goto_w`.
func fuenteSalto32(grande int) string {
	cuerpo := func(n, base int) string {
		var b strings.Builder
		for k := 0; k < n; k++ {
			fmt.Fprintf(&b, "      y = consume(%dL);\n", base+k)
		}
		return b.String()
	}
	const n = 4300
	return fmt.Sprintf(`public class Largo32 {
  static long ULTIMO;
  static long consume(long v) { ULTIMO = v; return v; }

  static long condi(int n) {
    long y = 0;
    if (n > 0) {
%s    }
    return y;
  }

  static long atras(int n) {
    long y = 0;
    int i = n;
    do {
%s    } while (--i > 0);
    return y;
  }

  static long salta(int n) {
    long y = 0;
    while (n > 0) {
%s      n = n - 1;
    }
    return y;
  }

  static long larguisimo(int n) {
    long y = 0;
    if (n > 0) {
%s    } else {
      y = 7;
    }
    return y;
  }
}
`, cuerpo(n, 1000000000001), cuerpo(n, 2000000000001), cuerpo(n, 3000000000001), cuerpo(grande, 4000000000001))
}

// dexDeFuenteSalto32 compila el fuente con nuestro emisor y lo dexa, que es lo que luego se mira.
func dexDeFuenteSalto32(t *testing.T, n int) *dexFile {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Largo32.java", contenido: fuenteSalto32(n)}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó los métodos largos: %v", err)
	}
	clases := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee: %v", c.interno, err)
		}
		clases = append(clases, cf)
	}
	datos, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("el dexer rechazó los saltos largos: %v", err)
	}
	d, err := readDex(datos)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	return d
}

// esGoto32: la primera unidad del goto/32 es 0x002a exacta; el byte alto es cero porque el
// desplazamiento va en las dos unidades siguientes.
func esGoto32(insns []uint16, i int) bool {
	return i+2 < len(insns) && insns[i] == 0x002a
}

func desplazamiento30(insns []uint16, i int) int32 {
	return int32(uint32(insns[i+1]) | uint32(insns[i+2])<<16)
}

// busquedaGoto32 devuelve el primer goto/32 que cumpla el predicado, o -1.
func busquedaGoto32(insns []uint16, cumple func(int32) bool) int {
	for i := 0; i+2 < len(insns); i++ {
		if esGoto32(insns, i) && cumple(desplazamiento30(insns, i)) {
			return i
		}
	}
	return -1
}

// TestDexSaltoLargoSePromueveAGoto32: el bucle `while` de `salta` tiene el `goto` de vuelta a más de
// 32767 unidades y no hay `if` de por medio, así que el dexer tiene que emitirlo como `goto/32` 0x2a
// con el desplazamiento negativo en las dos unidades siguientes. Si se quita la forma larga, dexar
// el fuente falla con «a branch that needs a long form at pc 3» y la prueba muere antes de mirar el
// opcode.
func TestDexSaltoLargoSePromueveAGoto32(t *testing.T) {
	d := dexDeFuenteSalto32(t, 4300)
	ci, err := codeItemDeMetodo(d, "salta")
	if err != nil {
		t.Fatalf("salta: %v", err)
	}
	if len(ci.insns) <= 32767 {
		t.Fatalf("salta mide %d unidades: la sonda no fuerza el goto/32", len(ci.insns))
	}
	i := busquedaGoto32(ci.insns, func(rel int32) bool { return rel < -32768 })
	if i < 0 {
		t.Fatalf("no hay un goto/32 de vuelta fuera de 16 bits en %d unidades", len(ci.insns))
	}
	t.Logf("salta: goto/32 en la unidad %d con desplazamiento %d", i, desplazamiento30(ci.insns, i))
}

// TestDexSaltoCondicionalLargoSeInvierte: en `condi` el `ifle` del `.class` cabe en 16 bits de JVM
// (30106 bytes) pero no en unidades de dex (34411), y dalvik no tiene `if-*` larga. El dexer invierte
// la condición —`ifle` es `if-gtz`— con el desplazamiento fijo +5 y pone el destino en un `goto/32`
// detrás, que es exactamente lo que mide d8.
func TestDexSaltoCondicionalLargoSeInvierte(t *testing.T) {
	d := dexDeFuenteSalto32(t, 4300)
	ci, err := codeItemDeMetodo(d, "condi")
	if err != nil {
		t.Fatalf("condi: %v", err)
	}
	i := busquedaGoto32(ci.insns, func(rel int32) bool { return rel > 32767 })
	if i < 0 {
		t.Fatalf("condi: no hay goto/32 adelante fuera de 16 bits en %d unidades", len(ci.insns))
	}
	if i < 2 {
		t.Fatalf("condi: el goto/32 de la unidad %d no tiene la condicional delante", i)
	}
	cond := ci.insns[i-2]
	op := byte(cond)
	if op < opIfEq || op > opIfLez {
		t.Errorf("condi: la unidad %d antes del goto/32 es %#04x y no un if-*", i-2, cond)
	}
	if got := int(int16(ci.insns[i-1])); got != 5 {
		t.Errorf("condi: el if invertido salta %d y tiene que saltar 5 (2 del if y 3 del goto/32)", got)
	}
	t.Logf("condi: if-%#02x +5 en la unidad %d y goto/32 %d en la %d", op, i-2, desplazamiento30(ci.insns, i), i)
}

// TestDexSaltoCondicionalAtrasLargoSeInvierte: el `do { ... } while` de `atras` deja un `ifgt` de JVM
// hacia atrás que cabe en bytes pero no en unidades de dex. La condicional se invierte y el salto de
// vuelta va en un `goto/32` negativo, como el `if-lez v2, +5; goto/32 -34405` que mide d8.
func TestDexSaltoCondicionalAtrasLargoSeInvierte(t *testing.T) {
	d := dexDeFuenteSalto32(t, 4300)
	ci, err := codeItemDeMetodo(d, "atras")
	if err != nil {
		t.Fatalf("atras: %v", err)
	}
	i := busquedaGoto32(ci.insns, func(rel int32) bool { return rel < -32768 })
	if i < 0 {
		t.Fatalf("atras: no hay goto/32 de vuelta fuera de 16 bits en %d unidades", len(ci.insns))
	}
	if i < 2 {
		t.Fatalf("atras: el goto/32 de la unidad %d no tiene la condicional delante", i)
	}
	cond := ci.insns[i-2]
	op := byte(cond)
	if op < opIfEq || op > opIfLez {
		t.Errorf("atras: la unidad %d antes del goto/32 es %#04x y no un if-*", i-2, cond)
	}
	if got := int(int16(ci.insns[i-1])); got != 5 {
		t.Errorf("atras: el if invertido salta %d y tiene que saltar 5", got)
	}
	t.Logf("atras: if-%#02x +5 en la unidad %d y goto/32 %d en la %d", op, i-2, desplazamiento30(ci.insns, i), i)
}

// TestDexGotoWLargoSePromueveAGoto32: en `larguisimo` el salto del `.class` mide 35011 bytes, así que
// tanto javac como nuestro emisor escriben `goto_w`; el dexer lo baja primero a `goto/16` y, como
// tampoco cabe en 16 bits, a `goto/32` 0x2a.
func TestDexGotoWLargoSePromueveAGoto32(t *testing.T) {
	d := dexDeFuenteSalto32(t, 5000)
	ci, err := codeItemDeMetodo(d, "larguisimo")
	if err != nil {
		t.Fatalf("larguisimo: %v", err)
	}
	i := busquedaGoto32(ci.insns, func(rel int32) bool { return rel > 32767 })
	if i < 0 {
		t.Fatalf("larguisimo: no hay goto/32 fuera de 16 bits en %d unidades", len(ci.insns))
	}
	t.Logf("larguisimo: goto/32 en la unidad %d con desplazamiento %d", i, desplazamiento30(ci.insns, i))
}
