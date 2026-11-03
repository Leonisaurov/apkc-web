package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteFlotante es la sonda del parrafo 77: cuentas de `float` y de `double`, la promocion binaria de
// Java (doble > flotante > largo > entero), las conversiones, los locales anchos, los campos estaticos
// de cada tipo y las constantes —cortas cuando hay `0.0`/`1.0`/`2.0`, por el pool por lo demas—.
const fuenteFlotante = `public class FyD {
    static double dd = 2.5;
    static float ff = 1.5f;

    public static double suma(double a, double b) { return a + b; }

    public static float resta(float a, float b) { return a - b; }

    public static double por(float a, double b) { return a * b; }

    public static double divide(double a, int b) { return a / b; }

    public static double constante() { return 3.14159; }

    public static float constanteCorta() { return 0.0f; }

    public static double deEntero(int n) { double x = n; return x + 1.0; }

    public static int aEntero(double d) { return (int) d; }

    public static float aFlotante(double d) { return (float) d; }

    public static double aDoble(float f) { return f; }

    public static double local(int n) {
        double r = 0.0;
        for (int i = 0; i < n; i++) {
            r = r + i * 1.5;
        }
        return r;
    }

    public static double campoEstatico() { return dd + ff; }

    public static double negativo() { return -2.5; }
}
`

// flotanteJavac: `javac --release 8 -g:lines,source FyD.java` y `base64 -w0 FyD.class`.
const flotanteJavac = "yv66vgAAADQANwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBkAJIfnwG4ZuBj/4AAAAAAAACQAMAA0HAA4MAA8AEAEAA0Z5RAEAAmRkAQABRAkADAASDAATABQBAAJmZgEAAUYGwAQAAAAAAAAGQAQAAAAAAAAEP8AAAAEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAARzdW1hAQAFKEREKUQBAAVyZXN0YQEABShGRilGAQADcG9yAQAFKEZEKUQBAAZkaXZpZGUBAAUoREkpRAEACWNvbnN0YW50ZQEAAygpRAEADmNvbnN0YW50ZUNvcnRhAQADKClGAQAIZGVFbnRlcm8BAAQoSSlEAQAHYUVudGVybwEABChEKUkBAAlhRmxvdGFudGUBAAQoRClGAQAGYURvYmxlAQAEKEYpRAEABWxvY2FsAQANU3RhY2tNYXBUYWJsZQEADWNhbXBvRXN0YXRpY28BAAhuZWdhdGl2bwEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEACEZ5RC5qYXZhACEADAACAAAAAgAIAA8AEAAAAAgAEwAUAAAADwABAAUABgABABoAAAAdAAEAAQAAAAUqtwABsQAAAAEAGwAAAAYAAQAAAAEACQAcAB0AAQAaAAAAHAAEAAQAAAAEJihjrwAAAAEAGwAAAAYAAQAAAAUACQAeAB8AAQAaAAAAHAACAAIAAAAEIiNmrgAAAAEAGwAAAAYAAQAAAAcACQAgACEAAQAaAAAAHQAEAAMAAAAFIo0na68AAAABABsAAAAGAAEAAAAJAAkAIgAjAAEAGgAAAB0ABAADAAAABSYch2+vAAAAAQAbAAAABgABAAAACwAJACQAJQABABoAAAAcAAIAAAAAAAQUAAevAAAAAQAbAAAABgABAAAADQAJACYAJwABABoAAAAaAAEAAAAAAAILrgAAAAEAGwAAAAYAAQAAAA8ACQAoACkAAQAaAAAAHwAEAAMAAAAHGodIJw9jrwAAAAEAGwAAAAYAAQAAABEACQAqACsAAQAaAAAAGwACAAIAAAADJo6sAAAAAQAbAAAABgABAAAAEwAJACwALQABABoAAAAbAAIAAgAAAAMmkK4AAAABABsAAAAGAAEAAAAVAAkALgAvAAEAGgAAABsAAgABAAAAAyKNrwAAAAEAGwAAAAYAAQAAABcACQAwACkAAQAaAAAAUgAGAAQAAAAaDkgDPh0aogASJx2HFAAJa2NIhAMBp//vJ68AAAACABsAAAAWAAUAAAAaAAIAGwAJABwAEgAbABgAHgAxAAAACgAC/QAEAwH6ABMACQAyACUAAQAaAAAAIQAEAAAAAAAJsgALsgARjWOvAAAAAQAbAAAABgABAAAAIQAJADMAJQABABoAAAAcAAIAAAAAAAQUABWvAAAAAQAbAAAABgABAAAAIwAIADQABgABABoAAAAoAAIAAAAAAAwUABezAAsSGbMAEbEAAAABABsAAAAKAAIAAAACAAYAAwABADUAAAACADY="

var metodosFlotante = []string{"suma", "resta", "por", "divide", "constante", "constanteCorta",
	"deEntero", "aEntero", "aFlotante", "aDoble", "local", "campoEstatico", "negativo"}

func dosClasesFlotante(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/FyD.java", contenido: fuenteFlotante}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(flotanteJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestElFlotanteSeEscribeComoJavac: instruccion a instruccion, con `max_stack`, los huecos de local, la
// exception_table y las filas de linea. Vale lo que no se puede adivinar: el `float` ocupa UN hueco y
// UNA palabra, el `double` dos de cada, y la conversion va **en el sitio del operando** (`FyD.por` es
// `fload_0; f2d; dload_1; dmul`).
func TestElFlotanteSeEscribeComoJavac(t *testing.T) {
	nuestra, suya := dosClasesFlotante(t)
	for _, nombre := range metodosFlotante {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no esta en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no esta en la de javac", nombre)
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

// TestElFlotanteSeDexaComoJavac: el dex que escribe este fork sobre nuestras clases es byte a byte el
// que escribe sobre las de javac, con las formas dalvik del punto flotante medidas en el parrafo 80
// (`const`/`const-wide/high16`, `add-double/2addr` 0xcb, `float-to-double` 0x89, `sget` para el
// `float`). La comprobacion en ART la hace el banco, que ademas ejecuta.
func TestElFlotanteSeDexaComoJavac(t *testing.T) {
	nuestra, suya := dosClasesFlotante(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		difiere := -1
		for i := range deUno {
			if i < len(deOtro) && deUno[i] != deOtro[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el dex nuestro son %d bytes y el de javac %d; el primer byte distinto esta en el offset %d (%#02x contra %#02x)",
			len(deUno), len(deOtro), difiere, deUno[difiere], deOtro[difiere])
	}
}

// TestElFlotanteRechazaLoNoMedido: leer un elemento de un arreglo de flotantes sale por su nombre.
// (La comparación de dobles entró en el párrafo 81 con su propio test.)
func TestElFlotanteRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"lectura de un arreglo de flotantes", "package p;\npublic class A { public static float f(float[] v) { return v[0]; } }\n", "arreglo de F"},
		{"escritura en un arreglo de dobles", "package p;\npublic class A { public static void f(double[] v) { v[0] = 1.5; } }\n", "arreglo de D"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto lo que no esta medido", c.nombre)
			continue
		}
		if !bytes.Contains([]byte(err.Error()), []byte(c.pista)) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
