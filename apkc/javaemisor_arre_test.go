package main

// Los arreglos estrechos, medidos con `javac`, `d8` y `dexdump` sobre `Arre`. Los opcodes del JVMS
// `baload` 0x33, `saload` 0x35, `bastore` 0x54 y `sastore` 0x56 (y los ya medidos `caload` 0x34 y
// `castore` 0x55) y sus formas DEX `aget-boolean/byte/char/short` 0x47-0x4a y
// `aput-boolean/byte/char/short` 0x4e-0x51. El `baload`/`bastore` los comparte el `byte[]` y el
// `boolean[]`, y sólo el tipo del arreglo —el `[B` o el `[Z`— dice cuál de los dos opcodes sale; la
// sonda cubre las tres vías por las que llega: parámetro, `getstatic` y alias en un local.

import (
	"encoding/base64"
	"testing"
)

// arreJavac: `javac 21 --release 8 -g:lines,source Arre.java` y `base64 -w0 Arre.class`.
const arreJavac = "yv66vgAAADQAMwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABEFycmUBAAJmYgEAAltCCQAIAA4MAA8AEAEAAmZ6AQACW1oBAAJmcwEAAltTAQACZmMBAAJbQwEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAJsYgEABihbQkkpQgEAAnNiAQAHKFtCSUIpVgEAAmxzAQAGKFtTSSlTAQACc3MBAAcoW1NJUylWAQACbGMBAAYoW0NJKUMBAAJzYwEAByhbQ0lDKVYBAAJsegEABihbWkkpWgEAAnN6AQAHKFtaSVopVgEAAmdiAQAEKEkpQgEAAmd6AQAEKEkpWgEAAnBiAQAFKElCKVYBAAJwegEABShJWilWAQAHbGJMb2NhbAEAB2x6TG9jYWwBAApTb3VyY2VGaWxlAQAJQXJyZS5qYXZhACEACAACAAAABAAIAAsADAAAAAgADwAQAAAACAARABIAAAAIABMAFAAAAA8AAQAFAAYAAQAVAAAAHQABAAEAAAAFKrcAAbEAAAABABYAAAAGAAEAAAABAAgAFwAYAAEAFQAAABwAAgACAAAABCobM6wAAAABABYAAAAGAAEAAAAGAAgAGQAaAAEAFQAAAB0AAwADAAAABSobHFSxAAAAAQAWAAAABgABAAAABwAIABsAHAABABUAAAAcAAIAAgAAAAQqGzWsAAAAAQAWAAAABgABAAAACAAIAB0AHgABABUAAAAdAAMAAwAAAAUqGxxWsQAAAAEAFgAAAAYAAQAAAAkACAAfACAAAQAVAAAAHAACAAIAAAAEKhs0rAAAAAEAFgAAAAYAAQAAAAoACAAhACIAAQAVAAAAHQADAAMAAAAFKhscVbEAAAABABYAAAAGAAEAAAALAAgAIwAkAAEAFQAAABwAAgACAAAABCobM6wAAAABABYAAAAGAAEAAAAMAAgAJQAmAAEAFQAAAB0AAwADAAAABSobHFSxAAAAAQAWAAAABgABAAAADQAIACcAKAABABUAAAAeAAIAAQAAAAayAAcaM6wAAAABABYAAAAGAAEAAAAOAAgAKQAqAAEAFQAAAB4AAgABAAAABrIADRozrAAAAAEAFgAAAAYAAQAAAA8ACAArACwAAQAVAAAAHwADAAIAAAAHsgAHGhtUsQAAAAEAFgAAAAYAAQAAABAACAAtAC4AAQAVAAAAHwADAAIAAAAHsgANGhtUsQAAAAEAFgAAAAYAAQAAABEACAAvABgAAQAVAAAAHgACAAMAAAAGKk0sGzOsAAAAAQAWAAAABgABAAAAEgAIADAAJAABABUAAAAeAAIAAwAAAAYqTSwbM6wAAAABABYAAAAGAAEAAAATAAEAMQAAAAIAMg=="

func claseArreDeJavac(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(arreJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return cf
}

// TestDexWalkJvmCodeAceptaLosArreglosEstrechos: el caminante acepta los cuatro bytes que faltaban,
// cada uno de un byte y sin operandos, y también los dos que ya estaban. Falla si se quita cualquiera
// de las entradas de `jvmMeasured`: `walkJvmCode` para en el byte con el error medido.
func TestDexWalkJvmCodeAceptaLosArreglosEstrechos(t *testing.T) {
	casos := map[byte]string{
		0x33: "baload", 0x34: "caload", 0x35: "saload",
		0x54: "bastore", 0x55: "castore", 0x56: "sastore",
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

// TestDexWalkJvmCodeRechazaArreglosSinEntrada: la prueba contraria, opcode a opcode. Quitando cada
// entrada en caliente —y devolviéndola al terminar— el caminante vuelve a pararse en ese byte. Deja
// las seis filas atadas a la prueba.
func TestDexWalkJvmCodeRechazaArreglosSinEntrada(t *testing.T) {
	for _, op := range []byte{0x33, 0x34, 0x35, 0x54, 0x55, 0x56} {
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

// TestDexArreglosEstrechosDeUnClassDeJar: cada lector y escritor del `.class` de javac sale con el
// opcode 23x que mide `d8` para su tipo. Cubre el parámetro (`lb`/`lz`), el campo estático
// (`gb`/`gz`/`pb`/`pz`) y el alias en un local (`lbLocal`/`lzLocal`), que es donde el rastreo de
// tipo decide entre `aget-byte` 0x48 y `aget-boolean` 0x47.
func TestDexArreglosEstrechosDeUnClassDeJar(t *testing.T) {
	clase := claseArreDeJavac(t)
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class de javac: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{
		"lb": opAgetByte, "sb": opAputByte,
		"ls": opAgetShort, "ss": opAputShort,
		"lc": opAgetChar, "sc": opAputChar,
		"lz": opAgetBoolean, "sz": opAputBoolean,
		"gb": opAgetByte, "gz": opAgetBoolean, "pb": opAputByte, "pz": opAputBoolean,
		"lbLocal": opAgetByte, "lzLocal": opAgetBoolean,
	}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(clase, nombre)
		if !ok {
			t.Fatalf("%s: no está en el fixture de javac", nombre)
		}
		ci, err := td.codeItemDe(clase, m)
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
