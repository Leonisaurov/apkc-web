package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteArreglos es la sonda del parrafo 66, que es la misma que produjo arreglosJavac: el alta con
// `new int[n]` y `new String[n]`, la lectura y la escritura de un elemento, `.length`, y dos metodos
// con bucle que copian y suman.
const fuenteArreglos = `public class Arreglos {
    static int[] enteros() { return new int[3]; }
    static String[] textos(int n) { return new String[n]; }
    static int largo(int[] v) { return v.length; }
    static int lee(int[] v, int i) { return v[i]; }
    static void pon(int[] v, int i, int x) { v[i] = x; }
    static String leeTexto(String[] v, int i) { return v[i]; }
    static void ponTexto(String[] v, int i, String x) { v[i] = x; }
    static int suma(int[] v) {
        int s = 0;
        int i = 0;
        while (i < v.length) { s = s + v[i]; i = i + 1; }
        return s;
    }
    static int[] copia(int[] v) {
        int[] w = new int[v.length];
        int i = 0;
        while (i < v.length) { w[i] = v[i]; i = i + 1; }
        return w;
    }
}
`

// arreglosJavac: `javac -g Arreglos.java` (el mismo javac 21 de los demas fixtures) y
// `base64 -w0 Arreglos.class`.
const arreglosJavac = "yv66vgAAAEEALwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAQamF2YS9sYW5nL1N0cmluZwcACgEACEFycmVnbG9zAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAEkxvY2FsVmFyaWFibGVUYWJsZQEABHRoaXMBAApMQXJyZWdsb3M7AQAHZW50ZXJvcwEABCgpW0kBAAZ0ZXh0b3MBABYoSSlbTGphdmEvbGFuZy9TdHJpbmc7AQABbgEAAUkBAAVsYXJnbwEABShbSSlJAQABdgEAAltJAQADbGVlAQAGKFtJSSlJAQABaQEAA3BvbgEAByhbSUlJKVYBAAF4AQAIbGVlVGV4dG8BACgoW0xqYXZhL2xhbmcvU3RyaW5nO0kpTGphdmEvbGFuZy9TdHJpbmc7AQATW0xqYXZhL2xhbmcvU3RyaW5nOwEACHBvblRleHRvAQApKFtMamF2YS9sYW5nL1N0cmluZztJTGphdmEvbGFuZy9TdHJpbmc7KVYBABJMamF2YS9sYW5nL1N0cmluZzsBAARzdW1hAQABcwEADVN0YWNrTWFwVGFibGUBAAVjb3BpYQEABihbSSlbSQEAAXcHABkBAApTb3VyY2VGaWxlAQANQXJyZWdsb3MuamF2YQAhAAkAAgAAAAAACgABAAUABgABAAsAAAAvAAEAAQAAAAUqtwABsQAAAAIADAAAAAYAAQAAAAEADQAAAAwAAQAAAAUADgAPAAAACAAQABEAAQALAAAAHAABAAAAAAAEBrwKsAAAAAEADAAAAAYAAQAAAAIACAASABMAAQALAAAALwABAAEAAAAFGr0AB7AAAAACAAwAAAAGAAEAAAADAA0AAAAMAAEAAAAFABQAFQAAAAgAFgAXAAEACwAAAC0AAQABAAAAAyq+rAAAAAIADAAAAAYAAQAAAAQADQAAAAwAAQAAAAMAGAAZAAAACAAaABsAAQALAAAAOAACAAIAAAAEKhsurAAAAAIADAAAAAYAAQAAAAUADQAAABYAAgAAAAQAGAAZAAAAAAAEABwAFQABAAgAHQAeAAEACwAAAEMAAwADAAAABSobHE+xAAAAAgAMAAAABgABAAAABgANAAAAIAADAAAABQAYABkAAAAAAAUAHAAVAAEAAAAFAB8AFQACAAgAIAAhAAEACwAAADgAAgACAAAABCobMrAAAAACAAwAAAAGAAEAAAAHAA0AAAAWAAIAAAAEABgAIgAAAAAABAAcABUAAQAIACMAJAABAAsAAABDAAMAAwAAAAUqGyxTsQAAAAIADAAAAAYAAQAAAAgADQAAACAAAwAAAAUAGAAiAAAAAAAFABwAFQABAAAABQAfACUAAgAIACYAFwABAAsAAABxAAMAAwAAABkDPAM9HCq+ogAQGyocLmA8HARgPaf/8BusAAAAAwAMAAAAEgAEAAAACgACAAsABAAMABcADQANAAAAIAADAAAAGQAYABkAAAACABcAJwAVAAEABAAVABwAFQACACgAAAAIAAL9AAQBARIACAApACoAAQALAAAAdgAEAAMAAAAcKr68CkwDPRwqvqIAECscKhwuTxwEYD2n//ArsAAAAAMADAAAABIABAAAABAABQARAAcAEgAaABMADQAAACAAAwAAABwAGAAZAAAABQAXACsAGQABAAcAFQAcABUAAgAoAAAACgAC/QAHBwAsARIAAQAtAAAAAgAu"

// TestLosArreglosSeDexanIgualQueLosDeJavac: el oraculo doble aplicado a los arreglos. Si el
// emisor escribe el mismo `Code` que javac y el traductor traduce igual `newarray`, `anewarray`,
// `arraylength`, `iaload`, `iastore`, `aaload` y `aastore`, los dos dex salen byte a byte iguales.
func TestLosArreglosSeDexanIgualQueLosDeJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Arreglos.java", contenido: fuenteArreglos}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(arreglosJavac)
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

// TestLosMarcosDeUnArregloSonLosDeJavac: con arreglos, el tipo de verificacion de un local es su
// descriptor (`class "[I"`), medido en `Arreglos.copia`. Se compara con el fixture expandido.
func TestLosMarcosDeUnArregloSonLosDeJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Arreglos.java", contenido: fuenteArreglos}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(arreglosJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	for _, metodo := range []string{"suma", "copia"} {
		nuestra, err := marcosDeClase(salida[0].datos, metodo)
		if err != nil {
			t.Fatalf("%s: nuestros marcos no se leen: %v", metodo, err)
		}
		suya, err := marcosDeClase(datos, metodo)
		if err != nil {
			t.Fatalf("%s: los marcos de javac no se leen: %v", metodo, err)
		}
		if len(nuestra.marcos) != len(suya.marcos) {
			t.Errorf("%s: %d marcos nuestros y %d de javac", metodo, len(nuestra.marcos), len(suya.marcos))
		}
		for pc, ea := range suya.marcos {
			eb, ok := nuestra.marcos[pc]
			if !ok {
				t.Errorf("%s: falta el marco del pc %d (%s)", metodo, pc, describeMarco(ea))
				continue
			}
			if !igualesTipos(ea.locales, eb.locales) || !igualesTipos(ea.pila, eb.pila) {
				t.Errorf("%s: en el pc %d javac dice %s y nosotros %s", metodo, pc, describeMarco(ea), describeMarco(eb))
			}
		}
	}
}

// TestElArregloDelFuenteRechazaLoNoMedido: el inicializador, mas de una dimension y el indice que no
// es entero salen por su nombre, no por un mensaje generico.
func TestElArregloDelFuenteRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"inicializador", "class A { static void f() { int[] v = new int[] {1, 2}; } }", "inicializador"},
		{"dos dimensiones con inicializador", "class A { static void f() { int[][] v = new int[2][3]; } }", "dimension"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto algo que no esta medido", c.nombre)
			continue
		}
		if !bytes.Contains([]byte(err.Error()), []byte(c.pista)) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
