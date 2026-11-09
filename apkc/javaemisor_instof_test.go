package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// fuenteInstanceof es la sonda del parrafo 79: el `instanceof` sobre una clase, sobre una interfaz,
// dentro de un bucle con un arreglo de objetos, el `checkcast` de siempre y una prueba contra el tipo de
// la propia clase.
const fuenteInstanceof = `public class Ins {
    static int esCadena(Object x) {
        if (x instanceof String) {
            return 1;
        }
        return 0;
    }

    static int esSecuencia(Object x) {
        if (x instanceof CharSequence) {
            return 2;
        }
        return 3;
    }

    static int conArreglo(Object[] vs) {
        int n = 0;
        for (int i = 0; i < vs.length; i++) {
            if (vs[i] instanceof String) {
                n = n + 1;
            }
        }
        return n;
    }

    static String baja(Object x) {
        return (String) x;
    }

    static int anidado(Object x) {
        Object o = x;
        if (o instanceof Ins) {
            return 4;
        }
        return 5;
    }
}
`

// instanceofJavac: `javac --release 8 -g:lines,source Ins.java` y `base64 -w0 Ins.class`.
const instanceofJavac = "yv66vgAAADQAGgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAQamF2YS9sYW5nL1N0cmluZwcACgEAFmphdmEvbGFuZy9DaGFyU2VxdWVuY2UHAAwBAANJbnMBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAIZXNDYWRlbmEBABUoTGphdmEvbGFuZy9PYmplY3Q7KUkBAA1TdGFja01hcFRhYmxlAQALZXNTZWN1ZW5jaWEBAApjb25BcnJlZ2xvAQAWKFtMamF2YS9sYW5nL09iamVjdDspSQEABGJhamEBACYoTGphdmEvbGFuZy9PYmplY3Q7KUxqYXZhL2xhbmcvU3RyaW5nOwEAB2FuaWRhZG8BAApTb3VyY2VGaWxlAQAISW5zLmphdmEAIQALAAIAAAAAAAYAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAgADwAQAAEADQAAADQAAQABAAAACyrBAAeZAAUErAOsAAAAAgAOAAAADgADAAAAAwAHAAQACQAGABEAAAADAAEJAAgAEgAQAAEADQAAADQAAQABAAAACyrBAAmZAAUFrAasAAAAAgAOAAAADgADAAAACgAHAAsACQANABEAAAADAAEJAAgAEwAUAAEADQAAAFwAAgADAAAAHwM8Az0cKr6iABYqHDLBAAeZAAcbBGA8hAIBp//qG6wAAAACAA4AAAAaAAYAAAARAAIAEgAKABMAEwAUABcAEgAdABcAEQAAAAsAA/0ABAEBEvoABQAIABUAFgABAA0AAAAdAAEAAQAAAAUqwAAHsAAAAAEADgAAAAYAAQAAABsACAAXABAAAQANAAAAPwABAAIAAAANKkwrwQALmQAFB6wIrAAAAAIADgAAABIABAAAAB8AAgAgAAkAIQALACMAEQAAAAgAAfwACwcAAgABABgAAAACABk="

var metodosInstanceof = []string{"esCadena", "esSecuencia", "conArreglo", "baja", "anidado"}

func dosClasesInstanceof(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Ins.java", contenido: fuenteInstanceof}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(instanceofJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestElInstanceofSeEscribeComoJavac: `x instanceof T` es `aload; instanceof #k; ifeq` —la condicion
// **sin invertir**, al reves que las comparaciones del parrafo 52—, y el `ifeq` va al `else` medido
// desde el opcode. Se compara metodo a metodo contra javac, con las filas de linea y los marcos.
func TestElInstanceofSeEscribeComoJavac(t *testing.T) {
	nuestra, suya := dosClasesInstanceof(t)
	for _, nombre := range metodosInstanceof {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no esta en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no esta en la de javac", nombre)
		}
		if a.MaxStack != b.MaxStack || a.LocalCount != b.LocalCount {
			t.Errorf("%s: pila %d y locales %d, y los de javac %d y %d", nombre,
				a.MaxStack, a.LocalCount, b.MaxStack, b.LocalCount)
		}
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s: las filas nuestras son %q y las de javac %q", nombre, x, y)
		}
	}
}

// TestElInstanceofSeDexaComoJavac: el dex que sale de nuestras clases es byte a byte el que sale de las
// de javac, con el `instance-of` 0x20 en su forma 22c (`op|BA`, `type@CCCC`) medida sobre el fichero de
// `d8` (parrafo 79).
func TestElInstanceofSeDexaComoJavac(t *testing.T) {
	nuestra, suya := dosClasesInstanceof(t)
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

// TestElInstanceofRechazaLoNoMedido: el `instanceof` sobre un entero sale por su nombre (la forma
// negada y el arreglo como tipo probado están medidos desde el §131).
func TestElInstanceofRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"sobre un entero", "package p;\npublic class A { public static int f(int n) { if (n instanceof Object) { return 1; } return 0; } }\n", "sobre I"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto lo que no esta medido", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.pista) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
