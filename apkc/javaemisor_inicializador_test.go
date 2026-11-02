package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

// fuenteInicializador es la sonda del parrafo 73: `new int[] {…}` como vuelta, como local, como
// argumento de una llamada y uno de referencia (`String[]`). javac no usa `filled-new-array`: escribe
// el largo, el `newarray`/`anewarray` y por cada elemento `dup`, indice, valor y
// `iastore`/`aastore`.
const fuenteInicializador = `public class A {
    static int[] tres() { return new int[] {1, 2, 3}; }
    static String[] dos() { return new String[] {"a", "b"}; }
    static void usa() {
        int[] v = new int[] {4, 5};
        System.out.println(v[0] + v[1]);
    }
    static int uno(int[] v) { return v[0]; }
    static int llama() { return uno(new int[] {7, 8}); }
}
`

// inicializadorJavac: `javac --release 8 -g A.java` y `base64 -w0 A.class`.
const inicializadorJavac = "yv66vgAAADQALwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAQamF2YS9sYW5nL1N0cmluZwgACgEAAWEIAAwBAAFiCQAOAA8HABAMABEAEgEAEGphdmEvbGFuZy9TeXN0ZW0BAANvdXQBABVMamF2YS9pby9QcmludFN0cmVhbTsKABQAFQcAFgwAFwAYAQATamF2YS9pby9QcmludFN0cmVhbQEAB3ByaW50bG4BAAQoSSlWCgAaABsHABwMAB0AHgEAAUEBAAN1bm8BAAUoW0kpSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQADTEE7AQAEdHJlcwEABCgpW0kBAANkb3MBABUoKVtMamF2YS9sYW5nL1N0cmluZzsBAAN1c2EBAAF2AQACW0kBAAVsbGFtYQEAAygpSQEAClNvdXJjZUZpbGUBAAZBLmphdmEAIQAaAAIAAAAAAAYAAQAFAAYAAQAfAAAALwABAAEAAAAFKrcAAbEAAAACACAAAAAGAAEAAAABACEAAAAMAAEAAAAFACIAIwAAAAgAJAAlAAEAHwAAACgABAAAAAAAEAa8ClkDBE9ZBAVPWQUGT7AAAAABACAAAAAGAAEAAAACAAgAJgAnAAEAHwAAACcABAAAAAAADwW9AAdZAxIJU1kEEgtTsAAAAAEAIAAAAAYAAQAAAAMACAAoAAYAAQAfAAAATAAEAAEAAAAaBbwKWQMHT1kECE9LsgANKgMuKgQuYLYAE7EAAAACACAAAAAOAAMAAAAFAAwABgAZAAcAIQAAAAwAAQAMAA4AKQAqAAAACAAdAB4AAQAfAAAALgACAAEAAAAEKgMurAAAAAIAIAAAAAYAAQAAAAgAIQAAAAwAAQAAAAQAKQAqAAAACAArACwAAQAfAAAAKQAEAAAAAAARBbwKWQMQB09ZBBAIT7gAGawAAAABACAAAAAGAAEAAAAJAAEALQAAAAIALg=="

// TestElInicializadorDeArregloSeEscribeComoJavac: mismos metodos, mismos flags y la misma sucesion
// de instrucciones (con el miembro resuelto por nombre) que el fixture de javac.
func TestElInicializadorDeArregloSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/A.java", contenido: fuenteInicializador}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(inicializadorJavac)
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
		if n, s := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); n != s {
			t.Errorf("%s:\n  nuestro: %s\n  javac:   %s", a.Name, n, s)
		}
	}
}

// TestElInicializadorRechazaLoNoMedido: `new T[]` sin inicializador, mas de una dimension y un
// elemento de otro tipo salen por su nombre.
func TestElInicializadorRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"sin inicializador", "class A { void f() { int[] v = new int[]; } }\n", "inicializador"},
		{"dos dimensiones", "class A { void f() { int[] v = new int[] {}[0]; } }\n", "dimension"},
		{"elemento de otro tipo", "class A { void f() { int[] v = new int[] {\"a\"}; } }\n", "elemento"},
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
