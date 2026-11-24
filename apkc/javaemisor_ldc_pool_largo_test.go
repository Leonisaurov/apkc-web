package main

// El pool de la lambda desplazaba el `ldc` de las constantes que vienen detrás. Medido en el
// `MainActivity` de ApkShare: nuestro emisor escribía el `Integer` de `R.layout.*` en la entrada #268
// del pool y javac en la #249, así que nosotros gastábamos `ldc_w` (0x13, tres bytes) donde javac
// escribe `ldc` (0x12, dos). No era la forma: con el índice por encima de 255 el JVMS 6.5 obliga a
// `ldc_w`. La causa era el orden del pool: el emisor compilaba el implementador de la lambda —y las
// constantes de su bootstrap— dentro del método que la declara, mientras que javac los genera al
// cerrar la clase (`lambda$…` va el último del `ClassFile`), de modo que esas constantes caen detrás
// de las de todos los métodos.
//
// La sonda reproduce el límite con el mínimo: un método con una lambda (que arrastra el Methodref del
// método que llama su cuerpo, dos MethodType, dos MethodHandle y las seis entradas de
// `LambdaMetafactory`) y, detrás, un método con dos `Integer` de `R`. Con `k = 114` el índice del
// primero queda en 254 —cabe en `ldc`— y con el interruptor de la falsificación, que devuelve el
// implementador al sitio, sube a 274 y pide `ldc_w`. El control del otro lado es `k = 116`: 258 y
// `ldc_w` en los dos caminos, porque ahí el índice ya no cabe.
//
// El oráculo es javac 17 `--release 8 -g:lines,source`: su `limite` escribe `ldc` y el dex entero
// sale byte a byte igual al nuestro. El dex no distingue las dos formas, así que la regla del índice
// se comprueba además sobre nuestro `.class` desarmado.

import (
	"fmt"
	"strings"
	"testing"
)

// fuentePoolLambda arma la sonda: una lambda en un constructor ajeno (el camino medido en
// `MainActivity.java:156`, `new ApkAdapter(this, apks, apk -> shareApp(apk))`), `k` métodos que
// llenan el pool con enteros que no caben en `sipush`, y detrás los dos `Integer` que se vigilan.
func fuentePoolLambda(k int) string {
	var b strings.Builder
	b.WriteString("package p;\nimport java.util.function.Consumer;\npublic class PoolLambda {\n")
	b.WriteString("    void usa(int n) {}\n")
	b.WriteString("    void conLambda() {\n        Ayuda a = new Ayuda(s -> usa(s.length()));\n        usa(2);\n    }\n")
	for i := 0; i < k; i++ {
		fmt.Fprintf(&b, "    static int c%d() { return %d; }\n", i, 40000+i)
	}
	b.WriteString("    static int limite(int n) {\n        if (n > 3) { return 2130903043; }\n        return 2130837511;\n    }\n")
	b.WriteString("}\nclass Ayuda { Ayuda(Consumer<String> c) {} }\n")
	return b.String()
}

// formaDelPrimerLdc: la forma y el índice del primer `ldc`/`ldc_w` de `limite`. Las dos cosas, porque
// el índice es lo que decide la forma y la forma es lo que el dex no ve.
func formaDelPrimerLdc(t *testing.T, cf classFull) (string, int) {
	t.Helper()
	for _, m := range cf.Methods {
		if m.Name != "limite" {
			continue
		}
		lista, err := walkJvmCode(m.Code)
		if err != nil {
			t.Fatalf("limite: el Code no se recorre: %v", err)
		}
		for _, in := range lista {
			switch in.name {
			case "ldc":
				return "ldc", int(in.ops[0])
			case "ldc_w":
				return "ldc_w", u2De(in.ops, 0)
			}
		}
	}
	t.Fatalf("limite no lleva ningún ldc: la sonda no sirve")
	return "", 0
}

// compilaLaSondaLaClase: emite la sonda con el emisor del fork y devuelve `p/PoolLambda` releída.
func compilaLaSonda(t *testing.T, fuente string) classFull {
	t.Helper()
	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "PoolLambda.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	emitida := porInterno(nuestras, "p/PoolLambda")
	if emitida == nil {
		t.Fatalf("no salió p/PoolLambda (%s)", nombresDe(nuestras))
	}
	cf, err := readClassFull(emitida.datos)
	if err != nil {
		t.Fatalf("nuestra clase no se relee: %v", err)
	}
	return cf
}

// TestElLdcDelPoolSeQuedaEnElLimite255: el control del otro lado del límite. Con dos métodos más de
// relleno el mismo `ldc` pasa a 258 y la forma larga es la correcta, así que la forma la decide el
// índice y no el interruptor.
func TestElLdcDelPoolSeQuedaEnElLimite255(t *testing.T) {
	nuestra := compilaLaSonda(t, fuentePoolLambda(116))
	forma, idx := formaDelPrimerLdc(t, nuestra)
	if forma != "ldc_w" {
		t.Fatalf("con el índice %d la constante sale con %q y toca `ldc_w`", idx, forma)
	}
	if idx <= 255 {
		t.Errorf("`ldc_w` con el índice %d: cabe en un byte y la forma correcta es `ldc`", idx)
	}
}
