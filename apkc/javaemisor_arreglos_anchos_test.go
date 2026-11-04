package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteArreglosAnchos es la sonda del párrafo 84 ampliada con el alta y el inicializador: las seis
// operaciones medidas (`laload`/`lastore`, `faload`/`fastore`, `daload`/`dastore`, que en el dex son
// `aget-wide` 0x45 / `aput-wide` 0x4c para los anchos y `aget`/`aput` para el `float`), el
// `new long[n]`, el `new double[] {…}`, el `.length` de un `float[]` y una suma con bucle.
const fuenteArreglosAnchos = `public class ArreglosAnchos {
    static long leeLargo(long[] v, int i) { return v[i]; }
    static void escribeLargo(long[] v, int i, long x) { v[i] = x; }
    static float leeFlotante(float[] v, int i) { return v[i]; }
    static void escribeFlotante(float[] v, int i, float x) { v[i] = x; }
    static double leeDoble(double[] v, int i) { return v[i]; }
    static void escribeDoble(double[] v, int i, double x) { v[i] = x; }
    static long[] altaLargo(int n) { return new long[n]; }
    static double[] iniciaDoble() { double[] w = new double[] {1.5, 2.0}; return w; }
    static int largoDe(float[] v) { return v.length; }
    static float sumaFlotantes(float[] v) {
        float s = 0;
        int i = 0;
        while (i < v.length) { s = s + v[i]; i = i + 1; }
        return s;
    }
}
`

// arreglosAnchosJavac: `javac --release 8 -g:lines,source ArreglosAnchos.java` y
// `base64 -w0 ArreglosAnchos.class`.
const arreglosAnchosJavac = "yv66vgAAADQAJgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBj/4AAAAAAAABkAAAAAAAAAABwAMAQAOQXJyZWdsb3NBbmNob3MBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAIbGVlTGFyZ28BAAYoW0pJKUoBAAxlc2NyaWJlTGFyZ28BAAcoW0pJSilWAQALbGVlRmxvdGFudGUBAAYoW0ZJKUYBAA9lc2NyaWJlRmxvdGFudGUBAAcoW0ZJRilWAQAIbGVlRG9ibGUBAAYoW0RJKUQBAAxlc2NyaWJlRG9ibGUBAAcoW0RJRClWAQAJYWx0YUxhcmdvAQAFKEkpW0oBAAtpbmljaWFEb2JsZQEABCgpW0QBAAdsYXJnb0RlAQAFKFtGKUkBAA1zdW1hRmxvdGFudGVzAQAFKFtGKUYBAA1TdGFja01hcFRhYmxlAQAKU291cmNlRmlsZQEAE0FycmVnbG9zQW5jaG9zLmphdmEAIQALAAIAAAAAAAsAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAgADwAQAAEADQAAABwAAgACAAAABCobL60AAAABAA4AAAAGAAEAAAACAAgAEQASAAEADQAAAB0ABAAEAAAABSobIFCxAAAAAQAOAAAABgABAAAAAwAIABMAFAABAA0AAAAcAAIAAgAAAAQqGzCuAAAAAQAOAAAABgABAAAABAAIABUAFgABAA0AAAAdAAMAAwAAAAUqGyRRsQAAAAEADgAAAAYAAQAAAAUACAAXABgAAQANAAAAHAACAAIAAAAEKhsxrwAAAAEADgAAAAYAAQAAAAYACAAZABoAAQANAAAAHQAEAAQAAAAFKhsoUrEAAAABAA4AAAAGAAEAAAAHAAgAGwAcAAEADQAAABwAAQABAAAABBq8C7AAAAABAA4AAAAGAAEAAAAIAAgAHQAeAAEADQAAACoABQABAAAAEgW8B1kDFAAHUlkEFAAJUksqsAAAAAEADgAAAAYAAQAAAAkACAAfACAAAQANAAAAGwABAAEAAAADKr6sAAAAAQAOAAAABgABAAAACgAIACEAIgABAA0AAABLAAMAAwAAABkLRAM9HCq+ogAQIyocMGJEHARgPaf/8COuAAAAAgAOAAAAEgAEAAAADAACAA0ABAAOABcADwAjAAAACAAC/QAEAgESAAEAJAAAAAIAJQ=="

var metodosArreglosAnchos = []string{"leeLargo", "escribeLargo", "leeFlotante", "escribeFlotante",
	"leeDoble", "escribeDoble", "altaLargo", "iniciaDoble", "largoDe", "sumaFlotantes"}

func dosClasesArreglosAnchos(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/ArreglosAnchos.java", contenido: fuenteArreglosAnchos}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(arreglosAnchosJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestLosArreglosAnchosSeEscribenComoJavac: el `Code` de las seis operaciones, el alta y el
// inicializador. Lo que fija es el opcode propio de cada tipo (`laload` 0x2f, `faload` 0x30,
// `daload` 0x31, `lastore` 0x50, `fastore` 0x51, `dastore` 0x52) y que el `new` de un arreglo de
// primitivos es `newarray` con el atype, no `anewarray`.
func TestLosArreglosAnchosSeEscribenComoJavac(t *testing.T) {
	nuestra, suya := dosClasesArreglosAnchos(t)
	for _, nombre := range metodosArreglosAnchos {
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
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s: las filas nuestras son %q y las de javac %q", nombre, x, y)
		}
	}
}

// TestLosArreglosAnchosSeDexanIgualQueLosDeJavac: el oráculo doble. El traductor baja los seis opcodes
// del `.class` a `aget`/`aget-object`/`aget-wide` y `aput`/`aput-object`/`aput-wide` con el par de
// registros en los anchos, y si el `.class` coincide el dex tiene que salir byte a byte el mismo.
func TestLosArreglosAnchosSeDexanIgualQueLosDeJavac(t *testing.T) {
	nuestra, suya := dosClasesArreglosAnchos(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 84: %d bytes, idéntico escrito sobre nuestras clases que sobre las de javac", len(deUno))
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
