package main

import (
	"encoding/base64"
	"testing"
)

// fuenteTry es la sonda del parrafo 71: un `try` que devuelve desde dentro, un `throw` y un `try`
// con un catch que se ejecuta.
const fuenteTry = `public class T {
    static int total = 0;
    static int lee(int[] v, int i) {
        try {
            return v[i];
        } catch (ArrayIndexOutOfBoundsException e) {
            return -1;
        }
    }
    static void lanza(int n) {
        if (n > 0) {
            throw new IllegalStateException("malo");
        }
    }
    static int atrapa(int n) {
        int r = 0;
        try {
            lanza(n);
            r = 1;
        } catch (IllegalStateException e) {
            r = 2;
        }
        return r;
    }
}
`

// tryJavac: `javac -g T.java` y `base64 -w0 T.class`.
const tryJavac = "yv66vgAAAEEALwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAoamF2YS9sYW5nL0FycmF5SW5kZXhPdXRPZkJvdW5kc0V4Y2VwdGlvbgcACgEAH2phdmEvbGFuZy9JbGxlZ2FsU3RhdGVFeGNlcHRpb24IAAwBAARtYWxvCgAJAA4MAAUADwEAFShMamF2YS9sYW5nL1N0cmluZzspVgoAEQASBwATDAAUABUBAAFUAQAFbGFuemEBAAQoSSlWCQARABcMABgAGQEABXRvdGFsAQABSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQADTFQ7AQADbGVlAQAGKFtJSSlJAQABZQEAKkxqYXZhL2xhbmcvQXJyYXlJbmRleE91dE9mQm91bmRzRXhjZXB0aW9uOwEAAXYBAAJbSQEAAWkBAA1TdGFja01hcFRhYmxlAQABbgEABmF0cmFwYQEABChJKUkBACFMamF2YS9sYW5nL0lsbGVnYWxTdGF0ZUV4Y2VwdGlvbjsBAAFyAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAGVC5qYXZhACEAEQACAAAAAQAIABgAGQAAAAUAAQAFAAYAAQAaAAAALwABAAEAAAAFKrcAAbEAAAACABsAAAAGAAEAAAABABwAAAAMAAEAAAAFAB0AHgAAAAgAHwAgAAEAGgAAAGEAAgADAAAAByobLqxNAqwAAQAAAAMABAAHAAMAGwAAAA4AAwAAAAUABAAGAAUABwAcAAAAIAADAAUAAgAhACIAAgAAAAcAIwAkAAAAAAAHACUAGQABACYAAAAGAAFEBwAHAAgAFAAVAAEAGgAAAEoAAwABAAAADxqeAA27AAlZEgu3AA2/sQAAAAMAGwAAAA4AAwAAAAsABAAMAA4ADgAcAAAADAABAAAADwAnABkAAAAmAAAAAwABDgAIACgAKQABABoAAACDAAEAAwAAABADPBq4ABAEPKcABk0FPBusAAEAAgAIAAsACQADABsAAAAeAAcAAAAQAAIAEgAGABMACAAWAAsAFAAMABUADgAXABwAAAAgAAMADAACACEAKgACAAAAEAAnABkAAAACAA4AKwAZAAEAJgAAAA8AAv8ACwACAQEAAQcACQIACAAsAAYAAQAaAAAAHQABAAAAAAAFA7MAFrEAAAABABsAAAAGAAEAAAACAAEALQAAAAIALg=="

// TestElTrySeDexaComoJavac: el `Code` del try (con su `athrow` y su exception_table) y la traducción
// a `try_item` + `move-exception`/`throw` dan el mismo cuerpo que javac. Se compara por método —los
// registros, la tabla de excepciones y las palabras— y no el fichero entero: la `LineNumberTable` de
// un `try` sigue la política de este emisor y javac anota además el `goto` de salida con la línea de
// la llave del catch (§71).
func TestElTrySeDexaComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/T.java", contenido: fuenteTry}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(tryJavac)
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
	uno, err := readDex(deUno)
	if err != nil {
		t.Fatalf("nuestro dex no se relee: %v", err)
	}
	otro, err := readDex(deOtro)
	if err != nil {
		t.Fatalf("el dex de javac no se relee: %v", err)
	}
	for _, metodo := range []string{"lee", "lanza", "atrapa"} {
		a, err := codeItemDeMetodo(uno, metodo)
		if err != nil {
			t.Fatalf("%s: %v", metodo, err)
		}
		b, err := codeItemDeMetodo(otro, metodo)
		if err != nil {
			t.Fatalf("%s: %v", metodo, err)
		}
		if a.registersSize != b.registersSize || a.insSize != b.insSize || a.outsSize != b.outsSize {
			t.Errorf("%s: la cabecera es %d/%d/%d y la de javac %d/%d/%d", metodo,
				a.registersSize, a.insSize, a.outsSize, b.registersSize, b.insSize, b.outsSize)
		}
		if a.triesSize != b.triesSize {
			t.Errorf("%s: %d try_item nuestros y %d de javac", metodo, a.triesSize, b.triesSize)
		}
		if len(a.insns) != len(b.insns) {
			t.Errorf("%s: %d palabras y las de javac %d", metodo, len(a.insns), len(b.insns))
			continue
		}
		for i := range a.insns {
			if a.insns[i] != b.insns[i] {
				t.Errorf("%s: la palabra %d es %#04x y la de javac %#04x", metodo, i, a.insns[i], b.insns[i])
				break
			}
		}
	}
}
