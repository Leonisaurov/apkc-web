package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuentePlega es la sonda del párrafo 85: un literal numérico que la promoción binaria de Java
// convierte en otro tipo. Lo que fija cada método:
//
//	porDos      `f * 2`        → fconst_2   (el 2 llega siendo flotante, sin i2f)
//	masCero     `f + 0.0f`     → fconst_0
//	masTres     `f + 3`        → ldc con una entrada Float (3.0f no es forma corta)
//	menosDos    `f * -2`       → ldc con -2.0f  (el signo entra en la constante, no hay fneg)
//	dobleLiteral `2.5f * 2`    → una sola ldc 5.0f, sin fmul
//	asignacion  `float g = 2`  → fconst_2 en la asignación
//	porDosD     `d * 2`        → ldc2_w 2.0d  (javac NUNCA usa dconst_2)
//	masDosD     `d + 2.0`      → el mismo ldc2_w compartido
//	masIntUno   `d + 1`        → dconst_1     (la forma corta depende del valor, no de cómo está escrito)
//	masI        `d + i`        → iload_0; i2d (una variable sí se convierte en su sitio)
//	masDos      `l + 2`        → ldc2_w 2l    (no existe lconst_2)
//	masUno      `l + 1`        → lconst_1     (entero literal promocionado a largo)
//	porTresL    `l * 3`        → ldc2_w 3l
//	mayorQueDos `f > 2`        → fconst_2; fcmpl; ifle (la comparación promociona igual)
//	divEntero   `n / 2`        → iconst_2; idiv (ninguna cuenta con una variable se pliega)
//	retornaDoble `return 3` de un `double` → ldc2_w 3.0d (la regla no es solo de las cuentas)
//	retornaLargo `return 1` de un `long`   → lconst_1
//	guardaFlotante `float g = 5` → ldc 5.0f (5 no es forma corta de flotante)
//	masCeroEntero `f + 0`       → fconst_0 (el cero escrito como entero llega igual)
//	menosCero     `return -0.0f` → ldc con -0.0f (el signo es parte de la constante: fconst_0 es el cero positivo)
const fuentePlega = `public class Plega {
    static float porDos(float f) { return f * 2; }
    static float masCero(float f) { return f + 0.0f; }
    static float masTres(float f) { return f + 3; }
    static float menosDos(float f) { return f * -2; }
    static float dobleLiteral() { return 2.5f * 2; }
    static float asignacion() { float g = 2; return g; }
    static double porDosD(double d) { return d * 2; }
    static double masDosD(double d) { return d + 2.0; }
    static double masIntUno(double d) { return d + 1; }
    static double masI(int i, double d) { return d + i; }
    static long masDos(long l) { return l + 2; }
    static long masUno(long l) { return l + 1; }
    static long porTresL(long l) { return l * 3; }
    static int mayorQueDos(float f) { if (f > 2) { return 1; } return 0; }
    static int divEntero(int n) { return n / 2; }
    static double retornaDoble() { return 3; }
    static long retornaLargo() { return 1; }
    static float guardaFlotante() { float g = 5; return g; }
    static float masCeroEntero(float f) { return f + 0; }
    static float menosCero() { return -0.0f; }
}
`

// plegaJavac: `javac --release 8 -g:lines,source Plega.java` y `base64 -w0 Plega.class`.
const plegaJavac = "yv66vgAAADQANwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBEBAAAAEwAAAAARAoAAABkAAAAAAAAAABQAAAAAAAAACBQAAAAAAAAADBkAIAAAAAAAABIAAAAAHABQBAAVQbGVnYQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAZwb3JEb3MBAAQoRilGAQAHbWFzQ2VybwEAB21hc1RyZXMBAAhtZW5vc0RvcwEADGRvYmxlTGl0ZXJhbAEAAygpRgEACmFzaWduYWNpb24BAAdwb3JEb3NEAQAEKEQpRAEAB21hc0Rvc0QBAAltYXNJbnRVbm8BAARtYXNJAQAFKElEKUQBAAZtYXNEb3MBAAQoSilKAQAGbWFzVW5vAQAIcG9yVHJlc0wBAAttYXlvclF1ZURvcwEABChGKUkBAA1TdGFja01hcFRhYmxlAQAJZGl2RW50ZXJvAQAEKEkpSQEADHJldG9ybmFEb2JsZQEAAygpRAEADHJldG9ybmFMYXJnbwEAAygpSgEADmd1YXJkYUZsb3RhbnRlAQANbWFzQ2Vyb0VudGVybwEACW1lbm9zQ2VybwEAClNvdXJjZUZpbGUBAApQbGVnYS5qYXZhACEAEwACAAAAAAAVAAEABQAGAAEAFQAAAB0AAQABAAAABSq3AAGxAAAAAQAWAAAABgABAAAAAQAIABcAGAABABUAAAAcAAIAAQAAAAQiDWquAAAAAQAWAAAABgABAAAAAgAIABkAGAABABUAAAAcAAIAAQAAAAQiC2KuAAAAAQAWAAAABgABAAAAAwAIABoAGAABABUAAAAdAAIAAQAAAAUiEgdirgAAAAEAFgAAAAYAAQAAAAQACAAbABgAAQAVAAAAHQACAAEAAAAFIhIIaq4AAAABABYAAAAGAAEAAAAFAAgAHAAdAAEAFQAAABsAAQAAAAAAAxIJrgAAAAEAFgAAAAYAAQAAAAYACAAeAB0AAQAVAAAAHAABAAEAAAAEDUMirgAAAAEAFgAAAAYAAQAAAAcACAAfACAAAQAVAAAAHgAEAAIAAAAGJhQACmuvAAAAAQAWAAAABgABAAAACAAIACEAIAABABUAAAAeAAQAAgAAAAYmFAAKY68AAAABABYAAAAGAAEAAAAJAAgAIgAgAAEAFQAAABwABAACAAAABCYPY68AAAABABYAAAAGAAEAAAAKAAgAIwAkAAEAFQAAAB0ABAADAAAABScah2OvAAAAAQAWAAAABgABAAAACwAIACUAJgABABUAAAAeAAQAAgAAAAYeFAAMYa0AAAABABYAAAAGAAEAAAAMAAgAJwAmAAEAFQAAABwABAACAAAABB4KYa0AAAABABYAAAAGAAEAAAANAAgAKAAmAAEAFQAAAB4ABAACAAAABh4UAA5prQAAAAEAFgAAAAYAAQAAAA4ACAApACoAAQAVAAAAKwACAAEAAAAKIg2VngAFBKwDrAAAAAIAFgAAAAYAAQAAAA8AKwAAAAMAAQgACAAsAC0AAQAVAAAAHAACAAEAAAAEGgVsrAAAAAEAFgAAAAYAAQAAABAACAAuAC8AAQAVAAAAHAACAAAAAAAEFAAQrwAAAAEAFgAAAAYAAQAAABEACAAwADEAAQAVAAAAGgACAAAAAAACCq0AAAABABYAAAAGAAEAAAASAAgAMgAdAAEAFQAAAB0AAQABAAAABRIJQyKuAAAAAQAWAAAABgABAAAAEwAIADMAGAABABUAAAAcAAIAAQAAAAQiC2KuAAAAAQAWAAAABgABAAAAFAAIADQAHQABABUAAAAbAAEAAAAAAAMSEq4AAAABABYAAAAGAAEAAAAVAAEANQAAAAIANg=="

var metodosPlega = []string{"porDos", "masCero", "masTres", "menosDos", "dobleLiteral", "asignacion",
	"porDosD", "masDosD", "masIntUno", "masI", "masDos", "masUno", "porTresL", "mayorQueDos", "divEntero",
	"retornaDoble", "retornaLargo", "guardaFlotante", "masCeroEntero", "menosCero"}

func dosClasesPlega(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Plega.java", contenido: fuentePlega}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(plegaJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestElLiteralPromocionadoSeEscribeComoJavac: el `Code` método por método, con `max_stack`, los huecos
// de local y las filas de línea. Lo que fija es que la promoción se hace **sobre la constante**, antes
// de elegir su forma: `d + 1` es `dconst_1` y `l + 2` es `ldc2_w`, no `iconst` seguido de una conversión.
func TestElLiteralPromocionadoSeEscribeComoJavac(t *testing.T) {
	nuestra, suya := dosClasesPlega(t)
	for _, nombre := range metodosPlega {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no está en la de javac", nombre)
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

// TestElLiteralPromocionadoSeDexaComoJavac: el dex que escribe este fork sobre nuestras clases es byte
// a byte el que escribe sobre las de javac. Con el plegado hecho en el `.class` no hay que traducir
// ninguna conversión, así que los cuerpos salen con las mismas palabras.
func TestElLiteralPromocionadoSeDexaComoJavac(t *testing.T) {
	nuestra, suya := dosClasesPlega(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 85: %d bytes, idéntico escrito sobre nuestras clases que sobre las de javac", len(deUno))
	if !bytes.Equal(deUno, deOtro) {
		difiere := -1
		for i := range deUno {
			if i < len(deOtro) && deUno[i] != deOtro[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el dex nuestro son %d bytes y el de javac %d; el primer byte distinto está en el offset %d (%#02x contra %#02x)",
			len(deUno), len(deOtro), difiere, deUno[difiere], deOtro[difiere])
	}
}

// TestLaAsignacionYElRetornoPromocionanSuLiteral: la regla no es solo de las cuentas —un `return 3;` de
// un método `double` o un `long l = 1;` salen también con la constante ya promocionada—.
func TestLaAsignacionYElRetornoPromocionanSuLiteral(t *testing.T) {
	nuestra, suya := dosClasesPlega(t)
	for _, nombre := range []string{"retornaDoble", "retornaLargo", "guardaFlotante"} {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no está en la de javac", nombre)
		}
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
	}
}
