package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

// fuenteLargo es la sonda del parrafo 74: un campo estatico `long`, una cuenta con dos argumentos
// largos, un local largo que se reasigna y se suma al estatico, y una llamada a
// `System.out.println(long)`. Con ella se midio que el `long` ocupa DOS huecos de local y DOS
// palabras de pila, que el pool guarda la constante `Long` en dos posiciones y que javac escribe
// `lconst_0`/`lconst_1` antes de recurrir a `ldc2_w`.
const fuenteLargo = `public class L {
    static long total = 5L;
    static long cuenta(long a, long b) { return a + b * 2L; }
    static long local() {
        long x = 1L;
        x = x + 300L;
        total = total + x;
        return total;
    }
    static void imprime(long a) { System.out.println(a); }
    public static void main(String[] args) {
        imprime(local());
        imprime(cuenta(2L, 3L));
    }
}
`

// largoJavac: `javac --release 8 -g:lines,source L.java` y `base64 -w0 L.class`.
const largoJavac = "yv66vgAAADQAMwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBQAAAAAAAAACBQAAAAAAAAEsCQAMAA0HAA4MAA8AEAEAAUwBAAV0b3RhbAEAAUoJABIAEwcAFAwAFQAWAQAQamF2YS9sYW5nL1N5c3RlbQEAA291dAEAFUxqYXZhL2lvL1ByaW50U3RyZWFtOwoAGAAZBwAaDAAbABwBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChKKVYKAAwAHgwAHwAgAQAFbG9jYWwBAAMoKUoKAAwAIgwAIwAcAQAHaW1wcmltZQUAAAAAAAAAAwoADAAnDAAoACkBAAZjdWVudGEBAAUoSkopSgUAAAAAAAAABQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAARtYWluAQAWKFtMamF2YS9sYW5nL1N0cmluZzspVgEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEABkwuamF2YQAhAAwAAgAAAAEACAAPABAAAAAGAAEABQAGAAEALAAAAB0AAQABAAAABSq3AAGxAAAAAQAtAAAABgABAAAAAQAIACgAKQABACwAAAAgAAYABAAAAAgeIBQAB2lhrQAAAAEALQAAAAYAAQAAAAMACAAfACAAAQAsAAAAOAAEAAIAAAAUCj8eFAAJYT+yAAseYbMAC7IAC60AAAABAC0AAAASAAQAAAAFAAIABgAIAAcAEAAIAAgAIwAcAAEALAAAACAAAwACAAAACLIAER62ABexAAAAAQAtAAAABgABAAAACgAJAC4ALwABACwAAAAzAAQAAQAAABO4AB24ACEUAAcUACS4ACa4ACGxAAAAAQAtAAAADgADAAAADAAGAA0AEgAOAAgAMAAGAAEALAAAAB8AAgAAAAAABxQAKrMAC7EAAAABAC0AAAAGAAEAAAACAAEAMQAAAAIAMg=="

// TestElLargoSeEscribeComoJavac: los cuerpos de la sonda son los de javac —mismos opcodes en el mismo
// orden, mismo `max_stack` en palabras y mismo `max_locals` en huecos, con el `long` contando por dos
// en los dos (medido con `javap -v -c` sobre las dos clases).
func TestElLargoSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/L.java", contenido: fuenteLargo}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(largoJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	if len(nuestra.Methods) != len(suya.Methods) {
		t.Fatalf("%d metodos y javac %d", len(nuestra.Methods), len(suya.Methods))
	}
	for i := range suya.Methods {
		a, b := nuestra.Methods[i], suya.Methods[i]
		if a.Name != b.Name || a.Desc != b.Desc || a.Access != b.Access {
			t.Errorf("el metodo %d es %s%s %#04x y el de javac %s%s %#04x",
				i, a.Name, a.Desc, a.Access, b.Name, b.Desc, b.Access)
			continue
		}
		if a.MaxStack != b.MaxStack {
			t.Errorf("%s: max_stack %d y el de javac %d (un largo cuenta por dos palabras)", a.Name, a.MaxStack, b.MaxStack)
		}
		if a.LocalCount != b.LocalCount {
			t.Errorf("%s: max_locals %d y el de javac %d (un largo ocupa dos huecos)", a.Name, a.LocalCount, b.LocalCount)
		}
		if n, s := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); n != s {
			t.Errorf("%s:\n  nuestro: %s\n  javac:   %s", a.Name, n, s)
		}
	}
}

// TestElLargoRechazaLoNoMedido: punto flotante, arreglo de largos y el campo privado ancho de la
// anonima salen por su nombre.
func TestElLargoRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"double", "class A { static void f() { double d = 1.0; } }\n", "punto"},
		{"arreglo de largos", "class A { static void f() { long[] v = new long[2]; v[0] = 1L; } }\n", "no esta medido"},
		{"comparacion de largos", "class A { static long f(long n) { if (n > 0L) { return 1L; } return 2L; } }\n", "entre J y J"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto lo que no esta medido", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.pista) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
