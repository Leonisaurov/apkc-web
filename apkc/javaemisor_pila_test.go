package main

// Las permutaciones de pila del JVMS: `dup` 0x59, `dup_x1` 0x5a, `dup_x2` 0x5b, `dup2` 0x5c,
// `dup2_x1` 0x5d, `dup2_x2` 0x5e y `swap` 0x5f. En DEX ninguna emite instrucción —los valores
// comparten registro—, pero la pila virtual tiene que quedar con la forma del JVMS, y para no
// confundir una pareja ancha con dos valores sueltos hay que saber cuántas ranuras ocupa cada
// entrada. Medido con `javac`, `d8` y `dexdump` sobre `Pila`: `oa[0] = oa[1] = null` deja `dup_x2`
// forma 1 y `la[0] = la[1] = 7L` deja `dup2_x2` con el tope ancho.

import (
	"encoding/base64"
	"testing"
)

// pilaJavac: `javac 21 --release 8 -g:lines,source Pila.java` y `base64 -w0 Pila.class`.
const pilaJavac = "yv66vgAAADQAKAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABFBpbGEBAAJpYQEAAltJCQAIAA4MAA8AEAEAAm9hAQATW0xqYXZhL2xhbmcvT2JqZWN0OwkACAASDAATABQBAAJsYQEAAltKBQAAAAAAAAAHAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAA2R4MQEABShJSSlWAQAKZHgyT2JqZXRvcwEACGR4MkFuY2hvAQAHZDJBbmNobwEABChKKVYBAAVkMkxlZQEABChJKUoBAAZ2YXJpb3MBACcoTGphdmEvbGFuZy9PYmplY3Q7TGphdmEvbGFuZy9PYmplY3Q7KVYBAAphbmNob0RvYmxlAQAGKEpKSSlWAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAJUGlsYS5qYXZhACEACAACAAAAAwAIAA8AEAAAAAgAEwAUAAAACAALAAwAAAAJAAEABQAGAAEAFwAAAB0AAQABAAAABSq3AAGxAAAAAQAYAAAABgABAAAAAQAIABkAGgABABcAAAAfAAMAAgAAAAeyAAcDGk+xAAAAAQAYAAAABgABAAAABQAIABsABgABABcAAAAlAAYAAAAAAA2yAA0DsgANBAFbU1OxAAAAAQAYAAAABgABAAAABgAIABwABgABABcAAAAnAAgAAAAAAA+yABEDsgARBBQAFV5QULEAAAABABgAAAAGAAEAAAAHAAgAHQAeAAEAFwAAAB8ABAACAAAAB7IAEQMeULEAAAABABgAAAAGAAEAAAAIAAgAHwAgAAEAFwAAAB4AAgABAAAABrIAERovrQAAAAEAGAAAAAYAAQAAAAkACAAhACIAAQAXAAAAJQADAAIAAAANsgANAypTsgANBCtTsQAAAAEAGAAAAAYAAQAAAAoACAAjACQAAQAXAAAAKQAEAAUAAAARsgARFQQeULIAERUEBGAgULEAAAABABgAAAAGAAEAAAALAAgAJQAGAAEAFwAAADQAAQAAAAAAFAe9AAKzAA0HvAuzABEHvAqzAAexAAAAAQAYAAAADgADAAAAAgAHAAMADQAEAAEAJgAAAAIAJw=="

func clasePilaDeJavac(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(pilaJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return cf
}

// TestDexWalkJvmCodeAceptaPermutacionesDePila: el caminante acepta las siete, cada una de un byte y
// sin operandos. Falla si se quita cualquiera de las entradas de `jvmMeasured`.
func TestDexWalkJvmCodeAceptaPermutacionesDePila(t *testing.T) {
	casos := map[byte]string{
		0x59: "dup", 0x5a: "dup_x1", 0x5b: "dup_x2", 0x5c: "dup2",
		0x5d: "dup2_x1", 0x5e: "dup2_x2", 0x5f: "swap",
	}
	for op, nombre := range casos {
		lista, err := walkJvmCode([]byte{op, 0xb1})
		if err != nil {
			t.Fatalf("%#02x (%s): el caminante lo rechaza: %v", op, nombre, err)
		}
		if len(lista) != 2 || lista[0].op != op || lista[0].name != nombre || len(lista[0].ops) != 0 {
			t.Errorf("%#02x: esperaba %s de cero operandos, salió %+v", op, nombre, lista)
		}
	}
}

// TestDexWalkJvmCodeRechazaPermutacionesSinEntrada: la prueba contraria, opcode a opcode. Quitando
// cada entrada en caliente —y devolviéndola al terminar— el caminante vuelve a pararse en ese byte.
func TestDexWalkJvmCodeRechazaPermutacionesSinEntrada(t *testing.T) {
	for _, op := range []byte{0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f} {
		guardada, habia := jvmMeasured[op]
		delete(jvmMeasured, op)
		_, err := walkJvmCode([]byte{op, 0xb1})
		if habia {
			jvmMeasured[op] = guardada
		}
		if err == nil {
			t.Errorf("%#02x: sin su entrada, el caminante lo aceptó como si estuviera medida", op)
		}
	}
}

// pilaDeTraduce arma un traductor con la pila virtual dada: registros y si cada entrada es ancha.
func pilaDeTraduce(regs []int, anchos []bool) *traduce {
	p := &traduce{}
	for i, r := range regs {
		p.pushValor(r, 0, anchos[i])
	}
	return p
}

// TestDexPermutaLaPilaComoElJvms: las siete formas sobre la pila virtual, incluidas las cuatro del
// `dup2_x2` y la del `dup_x2` con un valor ancho debajo. El resultado tiene que ser el orden de
// ranuras que describe el JVMS (de abajo a arriba).
func TestDexPermutaLaPilaComoElJvms(t *testing.T) {
	casos := []struct {
		nombre   string
		regs     []int
		anchos   []bool
		quiero   []int
		anchoFin []bool
	}{
		// dup: [1] -> [1, 1]
		{"dup", []int{1}, []bool{false}, []int{1, 1}, []bool{false, false}},
		// dup_x1: [1, 2] -> [2, 1, 2]
		{"dup_x1", []int{1, 2}, []bool{false, false}, []int{2, 1, 2}, []bool{false, false, false}},
		// dup_x2 forma 1 (tres de una palabra): [1, 2, 3] -> [3, 1, 2, 3]
		{"dup_x2", []int{1, 2, 3}, []bool{false, false, false}, []int{3, 1, 2, 3}, []bool{false, false, false, false}},
		// dup_x2 forma 2 (ancho debajo del tope estrecho): [W1, 2] -> [2, W1, 2]
		{"dup_x2", []int{1, 2}, []bool{true, false}, []int{2, 1, 2}, []bool{false, true, false}},
		// dup2 forma ancha: [W1] -> [W1, W1]
		{"dup2", []int{1}, []bool{true}, []int{1, 1}, []bool{true, true}},
		// dup2 forma dos palabras: [1, 2] -> [1, 2, 1, 2]
		{"dup2", []int{1, 2}, []bool{false, false}, []int{1, 2, 1, 2}, []bool{false, false, false, false}},
		// dup2_x1 forma 2 (tres de una palabra): [1, 2, 3] -> [2, 3, 1, 2, 3]
		{"dup2_x1", []int{1, 2, 3}, []bool{false, false, false}, []int{2, 3, 1, 2, 3}, []bool{false, false, false, false, false}},
		// dup2_x1 forma 1 (tope ancho): [1, W2] -> [W2, 1, W2]
		{"dup2_x1", []int{1, 2}, []bool{false, true}, []int{2, 1, 2}, []bool{true, false, true}},
		// dup2_x2 forma 1 (cuatro de una palabra): [1, 2, 3, 4] -> [3, 4, 1, 2, 3, 4]
		{"dup2_x2", []int{1, 2, 3, 4}, []bool{false, false, false, false},
			[]int{3, 4, 1, 2, 3, 4}, []bool{false, false, false, false, false, false}},
		// dup2_x2 forma 2 (tope ancho, dos de una palabra debajo): [1, 2, W3] -> [W3, 1, 2, W3]
		{"dup2_x2", []int{1, 2, 3}, []bool{false, false, true},
			[]int{3, 1, 2, 3}, []bool{true, false, false, true}},
		// dup2_x2 forma 3 (dos de una palabra arriba, ancho debajo): [W1, 2, 3] -> [2, 3, W1, 2, 3]
		{"dup2_x2", []int{1, 2, 3}, []bool{true, false, false},
			[]int{2, 3, 1, 2, 3}, []bool{false, false, true, false, false}},
		// dup2_x2 forma 4 (dos anchos): [W1, W2] -> [W2, W1, W2]
		{"dup2_x2", []int{1, 2}, []bool{true, true}, []int{2, 1, 2}, []bool{true, true, true}},
		// swap: [1, 2] -> [2, 1]
		{"swap", []int{1, 2}, []bool{false, false}, []int{2, 1}, []bool{false, false}},
	}
	for _, c := range casos {
		p := pilaDeTraduce(c.regs, c.anchos)
		p.permutaPila(jvmInstr{name: c.nombre})
		if p.err != nil {
			t.Errorf("%s: %v", c.nombre, p.err)
			continue
		}
		if len(p.enPila) != len(c.quiero) {
			t.Errorf("%s: %d entradas, esperaba %d (%v)", c.nombre, len(p.enPila), len(c.quiero), p.enPila)
			continue
		}
		for i := range c.quiero {
			if p.enPila[i] != c.quiero[i] || p.pilaAncho[i] != c.anchoFin[i] {
				t.Errorf("%s: entrada %d es (v%d, ancho %v), esperaba (v%d, ancho %v)",
					c.nombre, i, p.enPila[i], p.pilaAncho[i], c.quiero[i], c.anchoFin[i])
			}
		}
	}
}

// TestDexElJarDePilaSeDexa: el `.class` de javac, con `dup_x2` y `dup2_x2`, se dexa sin error y las
// asignaciones encadenadas salen como las escribe `d8`: `dx2Objetos` con dos `aput-object` y
// `dx2Ancho`/`d2Ancho` con `aput-wide`.
func TestDexElJarDePilaSeDexa(t *testing.T) {
	clase := clasePilaDeJavac(t)
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class de javac: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	casos := []struct {
		metodo string
		opcode byte
		veces  int
	}{
		{"dx2Objetos", opAputObjeto, 2},
		{"dx2Ancho", opAputAncho, 2},
		{"d2Ancho", opAputAncho, 1},
	}
	for _, c := range casos {
		m, ok := buscaMetodo(clase, c.metodo)
		if !ok {
			t.Fatalf("%s: no está en el fixture de javac", c.metodo)
		}
		ci, err := td.codeItemDe(clase, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", c.metodo, err)
		}
		visto := 0
		for _, u := range ci.insns {
			if byte(u) == c.opcode {
				visto++
			}
		}
		if visto < c.veces {
			t.Errorf("%s: %d veces el opcode %#02x, esperaba al menos %d en %04x",
				c.metodo, visto, c.opcode, c.veces, ci.insns)
		}
	}
}
