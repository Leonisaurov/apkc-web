package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteCorte es la sonda del parrafo 70: `break` en un `while`, `continue` en un `for` y `continue`
// en un `do-while`.
const fuenteCorte = `public class Corte {
    static int busca(int[] v, int x) {
        int i = 0;
        while (i < v.length) {
            if (v[i] == x) { break; }
            i++;
        }
        return i;
    }
    static int salta(int n) {
        int s = 0;
        for (int i = 0; i < n; i++) {
            if (i == 2) { continue; }
            s = s + i;
        }
        return s;
    }
    static int sigue(int n) {
        int s = 0;
        int i = 0;
        do {
            i++;
            if (i == 2) { continue; }
            s = s + i;
        } while (i < n);
        return s;
    }
}
`

// corteJavac: `javac -g Corte.java` y `base64 -w0 Corte.class`.
const corteJavac = "yv66vgAAAEEAHQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAFQ29ydGUBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQASTG9jYWxWYXJpYWJsZVRhYmxlAQAEdGhpcwEAB0xDb3J0ZTsBAAVidXNjYQEABihbSUkpSQEAAXYBAAJbSQEAAXgBAAFJAQABaQEADVN0YWNrTWFwVGFibGUBAAVzYWx0YQEABChJKUkBAAFuAQABcwEABXNpZ3VlAQAKU291cmNlRmlsZQEACkNvcnRlLmphdmEAIQAHAAIAAAAAAAQAAQAFAAYAAQAJAAAALwABAAEAAAAFKrcAAbEAAAACAAoAAAAGAAEAAAABAAsAAAAMAAEAAAAFAAwADQAAAAgADgAPAAEACQAAAHYAAgADAAAAGgM9HCq+ogATKhwuG6AABqcACYQCAaf/7RysAAAAAwAKAAAAFgAFAAAAAwACAAQACAAFABIABgAYAAgACwAAACAAAwAAABoAEAARAAAAAAAaABIAEwABAAIAGAAUABMAAgAVAAAACAAD/AACAQ8FAAgAFgAXAAEACQAAAIEAAgADAAAAHQM8Az0cGqIAFRwFoAAGpwAHGxxgPIQCAaf/7BusAAAAAwAKAAAAGgAGAAAACwACAAwACQANABEADgAVAAwAGwAQAAsAAAAgAAMABAAXABQAEwACAAAAHQAYABMAAAACABsAGQATAAEAFQAAAAwABP0ABAEBDAP6AAUACAAaABcAAQAJAAAAfwACAAMAAAAaAzwDPYQCARwFoAAGpwAHGxxgPBwaof/vG6wAAAADAAoAAAAeAAcAAAATAAIAFAAEABYABwAXAA8AGAATABkAGAAaAAsAAAAgAAMAAAAaABgAEwAAAAIAGAAZABMAAQAEABYAFAATAAIAFQAAAAkAA/0ABAEBCgMAAQAbAAAAAgAc"

// TestElCorteSeDexaComoJavac: `break` sale al fin del bucle (o del switch), `continue` va a la
// cabecera del `while`, al paso del `for` y a la condición del `do-while`; con eso el dex de nuestras
// clases es byte a byte el de javac.
func TestElCorteSeDexaComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Corte.java", contenido: fuenteCorte}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(corteJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
	}
}

// TestRompeYContinuaFueraDeSitio: los dos se rechazan con su nombre cuando no hay contexto.
func TestRompeYContinuaFueraDeSitio(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"break", "package p;\npublic class A { public static void f() { break; } }\n", "break"},
		{"continue", "package p;\npublic class A { public static void f() { continue; } }\n", "continue"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto fuera de un bucle", c.nombre)
			continue
		}
		if !bytes.Contains([]byte(err.Error()), []byte(c.pista)) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
