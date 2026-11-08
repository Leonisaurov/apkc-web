package main

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"testing"
)

// fuenteSeis es la sonda del párrafo 90: más de cinco palabras en una llamada. `llama` es un estático
// con seis enteros, `llamaInst` un virtual con receptor y seis enteros, y `llamaAncho` un estático con
// dos `long` (ocho palabras). Los tres tienen que salir con `invoke-*/range` 3rc.
const fuenteSeis = `public class Seis {
    static int seis(int a, int b, int c, int d, int e, int f) {
        return a + b + c + d + e + f;
    }
    static int llama() {
        return seis(1, 2, 3, 4, 5, 6);
    }
    int inst(int a, int b, int c, int d, int e, int f) {
        return a + b + c + d + e + f;
    }
    int llamaInst() {
        return inst(1, 2, 3, 4, 5, 6);
    }
    static long ancho(int a, int b, int c, int d, long e, long f) {
        return a + b + c + d + e + f;
    }
    static long llamaAncho() {
        return ancho(1, 2, 3, 4, 5L, 6L);
    }
}
`

// seisJavac: `javac --release 8 -g:lines,source Seis.java` y `base64 -w0 Seis.class`.
const seisJavac = "yv66vgAAADQAIQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEABFNlaXMBAARzZWlzAQAJKElJSUlJSSlJCgAIAA4MAA8ADAEABGluc3QFAAAAAAAAAAUFAAAAAAAAAAYKAAgAFQwAFgAXAQAFYW5jaG8BAAkoSUlJSUpKKUoBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAFbGxhbWEBAAMoKUkBAAlsbGFtYUluc3QBAApsbGFtYUFuY2hvAQADKClKAQAKU291cmNlRmlsZQEACVNlaXMuamF2YQAhAAgAAgAAAAAABwABAAUABgABABgAAAAdAAEAAQAAAAUqtwABsQAAAAEAGQAAAAYAAQAAAAEACAALAAwAAQAYAAAAJgACAAYAAAAOGhtgHGAdYBUEYBUFYKwAAAABABkAAAAGAAEAAAADAAgAGgAbAAEAGAAAACMABgAAAAAACwQFBgcIEAa4AAesAAAAAQAZAAAABgABAAAABgAAAA8ADAABABgAAAAnAAIABwAAAA8bHGAdYBUEYBUFYBUGYKwAAAABABkAAAAGAAEAAAAJAAAAHAAbAAEAGAAAACQABwABAAAADCoEBQYHCBAGtgANrAAAAAEAGQAAAAYAAQAAAAwACAAWABcAAQAYAAAAJwAEAAgAAAAPGhtgHGAdYIUWBGEWBmGtAAAAAQAZAAAABgABAAAADwAIAB0AHgABABgAAAAmAAgAAAAAAA4EBQYHFAAQFAASuAAUrQAAAAEAGQAAAAYAAQAAABIAAQAfAAAAAgAg"

var metodosSeis = []string{"seis", "llama", "inst", "llamaInst", "ancho", "llamaAncho", "<init>"}

func dosClasesSeis(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Seis.java", contenido: fuenteSeis}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(seisJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestLasLlamadasDeSeisArgumentosSonLasDeJavac: el `Code` de los siete métodos, con pila, locales y
// filas de línea. La forma de la llamada es lo que fija el 3rc; el índice del pool se normaliza porque
// el orden de internado de javac no es parte de la forma.
func TestLasLlamadasDeSeisArgumentosSonLasDeJavac(t *testing.T) {
	nuestra, suya := dosClasesSeis(t)
	sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }
	for _, nombre := range metodosSeis {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no está en la de javac", nombre)
		}
		if a.MaxStack != b.MaxStack || a.LocalCount != b.LocalCount {
			t.Errorf("%s: pila %d y locales %d, y los de javac %d y %d",
				nombre, a.MaxStack, a.LocalCount, b.MaxStack, b.LocalCount)
		}
		if x, y := sinPool(formaDeMetodo(t, nuestra, a)), sinPool(formaDeMetodo(t, suya, b)); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s: las filas nuestras son %q y las de javac %q", nombre, x, y)
		}
	}
}

// TestLasLlamadasDeSeisArgumentosSeDexanIgual: el dex byte a byte contra el de javac.
func TestLasLlamadasDeSeisArgumentosSeDexanIgual(t *testing.T) {
	nuestra, suya := dosClasesSeis(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 90: %d bytes", len(deUno))
	if !bytes.Equal(deUno, deOtro) {
		difiere := -1
		for i := range deUno {
			if i < len(deOtro) && deUno[i] != deOtro[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el dex nuestro son %d bytes y el de javac %d; el primer byte distinto está en el offset %d (%#02x contra %#02x)",
			len(deUno), len(deOtro), difiere, deUno[difiere], deOtro[difiere])
	}
}

// TestLasFormasRange: `invoke-static/range` 0x77 en `llama` y `llamaAncho`, `invoke-virtual/range` 0x74
// en `llamaInst`. Las palabras de `d8` están en el §90.
func TestLasFormasRange(t *testing.T) {
	nuestra, _ := dosClasesSeis(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{nuestra}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{"llama": 0x77, "llamaAncho": 0x77, "llamaInst": 0x74}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra clase", nombre)
		}
		ci, err := td.codeItemDe(nuestra, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := 0
		for _, u := range ci.insns {
			if byte(u) == opcode {
				visto++
			}
		}
		if visto == 0 {
			t.Errorf("%s: no aparece el opcode %#02x en %v", nombre, opcode, ci.insns)
		}
	}
}
