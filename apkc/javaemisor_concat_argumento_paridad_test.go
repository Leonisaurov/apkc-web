//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// La paridad con javac 17 de javaemisor_concat_argumento_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_concat_argumento_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestConcatComoArgumentoContraJavac17: la línea 179 tal cual. `File(File,String)` se resuelve y el
// `run()V` sale con las mismas instrucciones que javac; el dex entero, salvo el atributo del
// constructor de la anónima, también.
func TestConcatComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesConcatEnFile(fuenteConcatEnFile)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la concatenación como argumento de `File`: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/MainActivity.java", fuenteConcatEnFile)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestro := metodoDeClaseEmitida(t, nuestras, "p/MainActivity$1", "run", "()V")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/MainActivity$1;", "run", "()V")

	// la sobrecarga elegida: `File(File,String)` y no otro constructor. En `run()` hay dos `new File`:
	// el de la línea 178 (`File(String)`) y el de la 179 (`File(File,String)`).
	suFile := invocacionesDeFileInit(t, poolDeLeida(t, deJavac, "Lp/MainActivity$1;"), suyo.Code)
	if len(suFile) != 2 || suFile[1] != "(Ljava/io/File;Ljava/lang/String;)V" {
		t.Fatalf("javac llama a File.%v y se esperaba File(File,String) en la línea 179: el oráculo no vale", suFile)
	}
	nuestroFile := invocacionesDeFileInit(t, poolDeEmitida(t, nuestras, "p/MainActivity$1"), nuestro.Code)
	if len(nuestroFile) != 2 || nuestroFile[1] != suFile[1] {
		t.Fatalf("nuestro `run()` llama a File.%v y javac a File.%v", nuestroFile, suFile)
	}

	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("run()V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la anónima no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestConcatStringEnteroContraJavac17: `int + String` también se estima como `String` y el dex sale
// igual al de javac, con el `append(I)` del entero.
func TestConcatStringEnteroContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesConcatEnFile(fuenteConcatStringEntero)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `int + String` como argumento: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/MainActivity.java", fuenteConcatStringEntero)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestro := metodoDeClaseEmitida(t, nuestras, "p/MainActivity", "conEntero", "(I)Ljava/io/File;")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/MainActivity;", "conEntero", "(I)Ljava/io/File;")
	if d := invocacionesDeFileInit(t, poolDeLeida(t, deJavac, "Lp/MainActivity;"), suyo.Code); len(d) != 1 || d[0] != "(Ljava/io/File;Ljava/lang/String;)V" {
		t.Fatalf("javac llama a File.%v y se esperaba File(File,String): el oráculo de este test no vale", d)
	}
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("conEntero(I)Ljava/io/File; no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestCuentaEnteraEnFileSigueRechazando: el control negativo. `1 + 2` no es concatenación, javac
// rechaza el fuente y el emisor tampoco lo acepta: no se estima como cadena por estar en esa posición.
func TestCuentaEnteraEnFileSigueRechazando(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesConcatEnFile(fuenteCuentaEnFile)

	// primero, que javac lo rechaza: si algún día lo aceptara, el control cambia de sentido
	raiz := t.TempDir()
	pon(t, raiz, "p/MainActivity.java", fuenteCuentaEnFile)
	salida := filepath.Join(raiz, "javac-out")
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "p/MainActivity.java")}
	out, errJavac := exec.Command(javac, args...).CombinedOutput()
	if errJavac == nil {
		t.Fatalf("javac aceptó `File(File,int)`: el control negativo ya no mide nada")
	}
	if !strings.Contains(string(out), "int cannot be converted to String") &&
		!strings.Contains(string(out), "no suitable constructor") {
		t.Logf("javac rechaza con un mensaje distinto del esperado: %s", out)
	}

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("el emisor aceptó una cuenta entera donde va una cadena: la estimación adivina")
	}
	if strings.Contains(err.Error(), "Ljava/lang/String;") {
		t.Fatalf("la cuenta entera se estimó como cadena: %v", err)
	}
	t.Logf("cuenta entera en File: %v (javac: %s)", err, firstLine(string(out)))
}
