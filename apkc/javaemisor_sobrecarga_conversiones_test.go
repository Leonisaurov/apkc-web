package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// Sondas de la frontera 5.26 medidas con javac 17 (`--release 8 -g:lines,source`). Cada fuente tiene
// dos sobrecargas y una llamada: sin conversion solo hay una candidata por subtipo y ya se resolvia;
// con boxing (`I` -> Integer), ensanchamiento (`I` -> long) o arreglos (`int[]` -> Object vs int[]) la
// eleccion y el codigo que javac escribe son estos.
const sobreBox = `public class S { static void box(Integer x) {} static void box(String s) {} static void t() { box(7); } }`
const sobreEns = `public class S { static void ens(long x) {} static void ens(String s) {} static void t() { ens(7); } }`
const sobreArr = `public class S { static void arr(Object o) {} static void arr(int[] a) {} static void t() { arr(new int[3]); } }`

const sobreBoxJavac = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAEWphdmEvbGFuZy9JbnRlZ2VyAQAHdmFsdWVPZgEAFihJKUxqYXZhL2xhbmcvSW50ZWdlcjsKAA4ADwcAEAwAEQASAQABUwEAA2JveAEAFihMamF2YS9sYW5nL0ludGVnZXI7KVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAVKExqYXZhL2xhbmcvU3RyaW5nOylWAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAOAAIAAAAAAAQAAQAFAAYAAQATAAAAHQABAAEAAAAFKrcAAbEAAAABABQAAAAGAAEAAAABAAgAEQASAAEAEwAAABkAAAABAAAAAbEAAAABABQAAAAGAAEAAAABAAgAEQAVAAEAEwAAABkAAAABAAAAAbEAAAABABQAAAAGAAEAAAABAAgAFgAGAAEAEwAAACEAAQAAAAAACRAHuAAHuAANsQAAAAEAFAAAAAYAAQAAAAEAAQAXAAAAAgAY"
const sobreEnsJavac = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBQAAAAAAAAAHCgAKAAsHAAwMAA0ADgEAAVMBAANlbnMBAAQoSilWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAFShMamF2YS9sYW5nL1N0cmluZzspVgEAAXQBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACgACAAAAAAAEAAEABQAGAAEADwAAAB0AAQABAAAABSq3AAGxAAAAAQAQAAAABgABAAAAAQAIAA0ADgABAA8AAAAZAAAAAgAAAAGxAAAAAQAQAAAABgABAAAAAQAIAA0AEQABAA8AAAAZAAAAAQAAAAGxAAAAAQAQAAAABgABAAAAAQAIABIABgABAA8AAAAfAAIAAAAAAAcUAAe4AAmxAAAAAQAQAAAABgABAAAAAQABABMAAAACABQ="
const sobreArrJavac = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAANhcnIBAAUoW0kpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABUoTGphdmEvbGFuZy9PYmplY3Q7KVYBAAF0AQAKU291cmNlRmlsZQEABlMuamF2YQAhAAgAAgAAAAAABAABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAEACAALAA8AAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAALAAwAAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAAQAAYAAQANAAAAHwABAAAAAAAHBrwKuAAHsQAAAAEADgAAAAYAAQAAAAEAAQARAAAAAgAS"

func dexDeCaso(t *testing.T, fuente, javac string) (nuestro, suyo []byte) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: fuente}}, jarDelSDK(t))
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
		t.Fatal(err)
	}
	suyo, err = dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatal(err)
	}
	return nuestro, suyo
}

// TestSobrecargaConBoxingEnsanchamientoYArreglos: las tres llamadas resuelven la misma sobrecarga que
// javac 17 y el dex sale byte a byte: `Integer.valueOf` para el boxing, `ldc2_w` para el literal
// promovido a long y `newarray int` para la candidata con arreglo.
func TestSobrecargaConBoxingEnsanchamientoYArreglos(t *testing.T) {
	casos := []struct{ nombre, fuente, javac string }{
		{"boxing", sobreBox, sobreBoxJavac},
		{"ensanchamiento", sobreEns, sobreEnsJavac},
		{"arreglos", sobreArr, sobreArrJavac},
	}
	for _, c := range casos {
		nuestro, suyo := dexDeCaso(t, c.fuente, c.javac)
		if !bytes.Equal(nuestro, suyo) {
			t.Errorf("%s: el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
				c.nombre, len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
		}
	}
}

// FRONTERA 5.26: los varargs siguen sin medirse. El parser solo acepta `tipo nombre` en un parametro,
// así que `int... xs` se rechaza por nombre antes de llegar a la resolucion; javac 17 lo baja a un
// `newarray int` con los elementos y una llamada a `var:([I)V` (medido con `javap -c`).
func TestLosVarargsSiguenSiendoFrontera(t *testing.T) {
	fuente := `public class S { static void var(int... xs) {} static void t() { var(1,2,3); } }`
	_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: fuente}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("se aceptó un parametro varargs que no está medido")
	}
	if !strings.Contains(err.Error(), "nombre") {
		t.Errorf("el rechazo no nombra el parametro: %v", err)
	}
}
