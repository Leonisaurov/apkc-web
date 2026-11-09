package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"
)

// fuenteConstantes: la sonda del §130 para el `ConstantValue` que no es `int`: `long`, `float`,
// `double` y `String` se doblan en el atributo (tags 5, 4, 6 y 8) y no bajan al `<clinit>`.
const fuenteConstantes = `package p;
public class Constantes {
    static final long L = 6L;
    static final float F = 2.5f;
    static final double D = 3.5;
    static final String S = "hola";
    static final int I = 7;
}
`

// constantesJavac: `javac --release 8 -g:lines,source Constantes.java` y `base64 -w0`.
const constantesJavac = "yv66vgAAADQAHQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAMcC9Db25zdGFudGVzAQABTAEAAUoBAA1Db25zdGFudFZhbHVlBQAAAAAAAAAGAQABRgRAIAAAAQABRAZADAAAAAAAAAEAAVMBABJMamF2YS9sYW5nL1N0cmluZzsIABYBAARob2xhAQABSQMAAAAHAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNvdXJjZUZpbGUBAA9Db25zdGFudGVzLmphdmEAIQAHAAIAAAAFABgACQAKAAEACwAAAAIADAAYAA4ADgABAAsAAAACAA8AGAAQABAAAQALAAAAAgARABgAEwAUAAEACwAAAAIAFQAYABcAFwABAAsAAAACABgAAQABAAUABgABABkAAAAdAAEAAQAAAAUqtwABsQAAAAEAGgAAAAYAAQAAAAIAAQAbAAAAAgAc"

// fuenteAncha3: nueve parámetros `double` (y nueve `float`) dejan el último por encima del nibble, y
// la suma ya no cabe en la forma `/2addr`: `d8` escribe la de tres registros (23x).
const fuenteAncha3 = `package p;
public class Ancha3 {
    static double total;
    public static double f(double a, double b, double c, double d,
                           double e, double f, double g, double h, double i) {
        return a + i;
    }
    public static float g(float a, float b, float c, float d,
                          float e, float f, float g, float h, float i) {
        return a + i;
    }
    public static double s(double a, double c) {
        return a - (a - c);
    }
    public static float t(float a, float c) {
        return a - (a - c);
    }
}
`

// ancha3Javac: `javac --release 8 -g:lines,source Ancha3.java` y `base64 -w0`.
const ancha3Javac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAIcC9BbmNoYTMBAAV0b3RhbAEAAUQBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABZgEADChEREREREREREQpRAEAAWcBAAwoRkZGRkZGRkZGKUYBAAFzAQAFKEREKUQBAAF0AQAFKEZGKUYBAApTb3VyY2VGaWxlAQALQW5jaGEzLmphdmEAIQAHAAIAAAABAAgACQAKAAAABQABAAUABgABAAsAAAAdAAEAAQAAAAUqtwABsQAAAAEADAAAAAYAAQAAAAIACQANAA4AAQALAAAAHQAEABIAAAAFJhgQY68AAAABAAwAAAAGAAEAAAAGAAkADwAQAAEACwAAAB0AAgAJAAAABSIXCGKuAAAAAQAMAAAABgABAAAACgAJABEAEgABAAsAAAAeAAYABAAAAAYmJihnZ68AAAABAAwAAAAGAAEAAAANAAkAEwAUAAEACwAAAB4AAwACAAAABiIiI2ZmrgAAAAEADAAAAAYAAQAAABAAAQAVAAAAAgAW"

// dexDeFuente compila el fuente con el emisor, lo confronta con el `.class` de javac y devuelve el
// dex de las dos versiones para que la prueba compare byte a byte.
func dexDeFuente(t *testing.T, nombre, fuente, javac string) (nuestro, suyo []byte) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/" + nombre + ".java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("%d clases, una", len(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(javac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
	}
	nuestro, err = dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	suyo, err = dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	os.WriteFile("/data/data/com.termux/files/usr/tmp/dbg-nuestro-"+t.Name()+".dex", nuestro, 0644)
	os.WriteFile("/data/data/com.termux/files/usr/tmp/dbg-suyo-"+t.Name()+".dex", suyo, 0644)
	return nuestro, suyo
}

// TestElConstantValueLargoFlotanteDobleYCadena: los cuatro tipos se doblan en el atributo y el dex
// (que lleva los `static_values`) sale byte a byte el de javac.
func TestElConstantValueLargoFlotanteDobleYCadena(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Constantes", fuenteConstantes, constantesJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}

// TestLaCuentaAnchaDeTresRegistros: con el operando fuera del nibble, la suma de `double` (y la de
// `float` cuando no cabe en `/2addr`) se escribe en la forma 23x, y el dex es el de javac.
func TestLaCuentaAnchaDeTresRegistros(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Ancha3", fuenteAncha3, ancha3Javac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	d, err := readDex(nuestro)
	if err != nil {
		t.Fatal(err)
	}
	for metodo, opcode := range map[string]byte{"s": 0xac /* sub-double, 23x */, "t": 0xa7 /* sub-float, 23x */} {
		ci, err := codeItemDeMetodo(d, metodo)
		if err != nil {
			t.Fatalf("%s: %v", metodo, err)
		}
		tres := false
		for _, w := range ci.insns {
			if byte(w) == opcode {
				tres = true
			}
		}
		if !tres {
			t.Errorf("`%s` no lleva la forma 0x%02x de tres registros en el cuerpo (%04x)", metodo, opcode, ci.insns)
		}
	}
}
