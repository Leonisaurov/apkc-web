package main

// El emisor de `.class` también tiene que ensanchar la vuelta de un bucle cuando el cuerpo pasa de
// 32 KB: el `goto` de tres bytes se queda corto y javac escribe `goto_w` (0xc8) con su
// desplazamiento de 32 bits. Aquí se mide contra javac 17 byte a byte en el `Code`, contra su dex
// byte a byte y se comprueba que el `goto_w` apunta hacia atrás, al principio del `while`.
//
// Antes de este frente el emisor rechazaba el método con `un salto hacia atras de N bytes: goto_w
// no esta medido`, y resolver la vuelta antes de cerrar el fin del bucle dejaba además el
// desplazamiento corto en cuanto la condición de cabecera se ensanchaba por delante.

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteGotoWAtras: el cuerpo del `while` son 9000 sentencias `x = x + n;`, 4 bytes de JVM cada una
// (36000 en total), así que el `goto` de vuelta y la condición de cabecera quedan a más de 32767
// bytes. Todo el cuerpo va en una línea para que la LineNumberTable no crezca con la sonda.
func fuenteGotoWAtras() string {
	cuerpo := strings.Repeat("x = x + n; ", 9000)
	return fmt.Sprintf(`public class Atras32 {
  static int atras(int n) {
    int x = 0;
    while (x >= 0) {
      %s
    }
    return x;
  }
}
`, cuerpo)
}

// metodoDeLaClase busca un método por nombre en una clase leída.
func metodoDeLaClase(cf classFull, nombre string) (classMethod, bool) {
	for _, m := range cf.Methods {
		if m.Name == nombre {
			return m, true
		}
	}
	return classMethod{}, false
}

// TestGotoWLargoHaciaAtras: nuestro `.class` coincide con el de javac 17 en el cuerpo del método y
// el dex que sale de las dos clases es el mismo byte a byte. Si el emisor no escribiera la forma
// larga, `compilarProyectoJava` fallaría antes de llegar aquí (falsificación en el informe).
func TestGotoWLargoHaciaAtras(t *testing.T) {
	jar := jarDelSDK(t)
	fuente := fuenteGotoWAtras()

	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Atras32.java", contenido: fuente}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la vuelta larga: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("salieron %d clases, una", len(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se relee: %v", err)
	}
	metodo, ok := metodoDeLaClase(nuestra, "atras")
	if !ok {
		t.Fatal("la clase no tiene `atras`")
	}
	if len(metodo.Code) <= 32767 {
		t.Fatalf("el método mide %d bytes: la sonda no fuerza el goto_w", len(metodo.Code))
	}
	// el `goto_w` de vuelta: opcode 0xc8 y un desplazamiento de 32 bits negativo que cae en la
	// cabecera del bucle. El prólogo es `iconst_0; istore_1` (dos bytes), así que la condición
	// `x >= 0` arranca en la posición 2.
	const cabeza = 2
	encontrado := false
	for i := 0; i+4 < len(metodo.Code); i++ {
		if metodo.Code[i] != 0xc8 {
			continue
		}
		rel := int32(uint32(metodo.Code[i+1])<<24 | uint32(metodo.Code[i+2])<<16 |
			uint32(metodo.Code[i+3])<<8 | uint32(metodo.Code[i+4]))
		if rel < -32768 && i+int(rel) == cabeza {
			encontrado = true
			t.Logf("goto_w en el byte %d con desplazamiento %d", i, rel)
			break
		}
	}
	if !encontrado {
		t.Fatal("no hay un goto_w de vuelta a la cabecera del bucle")
	}

	// oráculo 1: javac 17 sobre el mismo fuente, mismo `Code` y mismas cabeceras
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Atras32.java", fuente)
	dirJavac := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-d", dirJavac,
		filepath.Join(raiz, "Atras32.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó la sonda: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{dirJavac})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	var suya classFull
	for _, cf := range deJavac {
		if cf.Name == "LAtras32;" {
			suya = cf
		}
	}
	if suya.Name == "" {
		t.Fatal("javac no dejó LAtras32;")
	}
	esperado, ok := metodoDeLaClase(suya, "atras")
	if !ok {
		t.Fatal("javac no dejó `atras`")
	}
	if !bytes.Equal(metodo.Code, esperado.Code) {
		t.Fatalf("el Code difiere del de javac: %d bytes contra %d\nnuestro: %s\njavac:   %s",
			len(metodo.Code), len(esperado.Code), volcado(metodo.Code), volcado(esperado.Code))
	}
	if metodo.MaxStack != esperado.MaxStack || metodo.LocalCount != esperado.LocalCount {
		t.Errorf("max_stack/locals %d/%d y javac %d/%d", metodo.MaxStack, metodo.LocalCount,
			esperado.MaxStack, esperado.LocalCount)
	}

	// oráculo 2: el dex entero byte a byte (nuestro dexer sobre nuestras clases y sobre las suyas)
	nuestroDex, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	suDex, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(nuestroDex, suDex) {
		t.Fatalf("el dex difiere: %d bytes contra %d", len(nuestroDex), len(suDex))
	}

}

// volcado imprime los primeros bytes de un cuerpo para que un fallo diga dónde empieza a diferir.
func volcado(b []byte) string {
	const n = 48
	if len(b) <= n {
		return fmt.Sprintf("% x", b)
	}
	return fmt.Sprintf("% x …", b[:n])
}
