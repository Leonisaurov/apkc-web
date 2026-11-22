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

// sobreVarargs: dos metodos con `...` (uno primitivo y otro de referencia) y una `t` que los llama con
// tres argumentos, con cero, con dos cadenas y con el arreglo escrito tal cual. Las lineas importan:
// javac anota `t` en la linea 4 y el dex la compara byte a byte (§194).
const sobreVarargs = `public class S {
static void var(int... xs) {}
static void ref(String... xs) {}
static void t() { var(1,2,3); var(); ref("a","b"); var(new int[]{7}); }
}
`

// sobreVarargsJavac es el `S.class` de javac 17 `--release 8 -g:lines,source` sobre `sobreVarargs`.
const sobreVarargsJavac = "yv66vgAAADQAHAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAN2YXIBAAUoW0kpVgcADgEAEGphdmEvbGFuZy9TdHJpbmcIABABAAFhCAASAQABYgoACAAUDAAVABYBAANyZWYBABYoW0xqYXZhL2xhbmcvU3RyaW5nOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACAACAAAAAAAEAAEABQAGAAEAFwAAAB0AAQABAAAABSq3AAGxAAAAAQAYAAAABgABAAAAAQCIAAsADAABABcAAAAZAAAAAQAAAAGxAAAAAQAYAAAABgABAAAAAgCIABUAFgABABcAAAAZAAAAAQAAAAGxAAAAAQAYAAAABgABAAAAAwAIABkABgABABcAAABNAAQAAAAAADUGvApZAwRPWQQFT1kFBk+4AAcDvAq4AAcFvQANWQMSD1NZBBIRU7gAEwS8ClkDEAdPuAAHsQAAAAEAGAAAAAYAAQAAAAQAAQAaAAAAAgAb"

// TestVarargsEnSobrecargas: `int... xs` y `String... xs` ya no son frontera. Las cuatro llamadas
// resuelven la firma `([I)V` / `([Ljava/lang/String;)V`, y el dex sale byte a byte: `iconst_3;
// newarray int` con `dup`/`iconst_i`/`iastore` por elemento, el arreglo vacio con `iconst_0`, y el
// `new int[]{7}` que se pasa tal cual; `LineNumberTable` de `t` con su unica `line 4: 0`, y los dos
// metodos con ACC_VARARGS (0x0088). Medido con javac 17 `--release 8 -g:lines,source` (§194).
func TestVarargsEnSobrecargas(t *testing.T) {
	nuestro, suyo := dexDeCaso(t, sobreVarargs, sobreVarargsJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}

// TestVarargsFlagsDelMethodInfo: el method_info de un `T... xs` lleva ACC_VARARGS (0x0080) además de
// static, que es lo que escribe javac 17 con --release 8 (§194). Sin el bit el dex no coincidiría,
// porque `accesoMetodoDex` conserva el 0x0080.
func TestVarargsFlagsDelMethodInfo(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: sobreVarargs}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("%d clases, una", len(salida))
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	visto := map[string]bool{}
	for _, m := range cf.Methods {
		if m.Name != "var" && m.Name != "ref" {
			continue
		}
		visto[m.Name] = true
		if m.Access&accVarargs == 0 {
			t.Errorf("%s no lleva ACC_VARARGS: flags 0x%04x", m.Name, m.Access)
		}
		if m.Access&accStatic == 0 {
			t.Errorf("%s no es static: flags 0x%04x", m.Name, m.Access)
		}
	}
	for _, n := range []string{"var", "ref"} {
		if !visto[n] {
			t.Errorf("no se emitió %s", n)
		}
	}
}

// TestVarargsTieneQueSerElUltimo: `int... xs, int y` no es Java (JLS 8.4.1) y el parser lo rechaza
// nombrando el parámetro varargs en vez de aceptar una firma que javac no admite.
func TestVarargsTieneQueSerElUltimo(t *testing.T) {
	fuente := `public class S { static void var(int... xs, int y) {} }`
	_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: fuente}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("se aceptó un `...` que no estaba en el último parámetro")
	}
	if !strings.Contains(err.Error(), "varargs xs") {
		t.Errorf("el rechazo no nombra el parámetro varargs: %v", err)
	}
}
