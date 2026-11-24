//go:build !js

package main

// La paridad byte a byte de `TestCompilarEnGoDexaIgualQueJavacConUnaDependencia` necesita un
// `javac` de verdad y un proceso hijo, y bajo `GOOS=js` ni existe el primero ni se puede lanzar el
// segundo: la prueba se quedaba en rojo dentro de la pestaña midiendo el entorno, no el emisor. La
// exclusión es por build tag y no por `t.Skip`, que es la doctrina de `dependencias_test.go`.
//
// La mitad wasm de lo mismo ya existe y no se toca: `TestCompilarEnGoResuelveUnaClaseDeJar`
// (`dependencias_test.go`) comprueba con las mismas aserciones que el descriptor se leyó del jar y
// que el resultado se dexa, sin javac.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCompilarEnGoDexaIgualQueJavacConUnaDependencia: el `.class` que emitimos para un fuente que
// llama a un jar, dexado, tiene que dar el mismo dex que el `.class` de javac 17 para el mismo fuente
// contra el mismo jar. Es la prueba de que el descriptor se leyó del jar y no se adivinó.
func TestCompilarEnGoDexaIgualQueJavacConUnaDependencia(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()

	// la dependencia, compilada por javac 17 con la versión que emite el subconjunto
	pon(t, raiz, "dep/dep/Suma.java", fuenteDepSuma)
	depOut := filepath.Join(raiz, "dep-out")
	if out, err := exec.Command(javac, "--release", "8", "-g:lines,source", "-d", depOut,
		filepath.Join(raiz, "dep", "dep", "Suma.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 no compiló la dependencia: %v\n%s", err, out)
	}
	datos, err := os.ReadFile(filepath.Join(depOut, "dep", "Suma.class"))
	if err != nil {
		t.Fatal(err)
	}
	jarDep := jarDeDependencia(t, raiz, map[string][]byte{"dep/Suma.class": datos})
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSuma)
	t.Chdir(raiz)

	// lo nuestro, contra el jar de dependencias que está en jar/
	fuentes, err := fuentesJava([]string{"src"})
	if err != nil {
		t.Fatal(err)
	}
	nuestras, err := compilarProyectoJavaConClasspath(fuentes, []string{jarSDK, jarDep})
	if err != nil {
		t.Fatalf("el emisor no resolvió el jar de javac: %v", err)
	}
	nuestroDex, err := dexarUnaClase(t, nuestras, "com/hola/Uso")
	if err != nil {
		t.Fatal(err)
	}

	// lo de javac 17, el mismo fuente contra el mismo jar
	javacOut := filepath.Join(raiz, "javac-out")
	if out, err := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath",
		classpath([]string{jarSDK, jarDep}), "-d", javacOut,
		filepath.Join(raiz, "src", "com", "hola", "Uso.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 no compiló el consumidor: %v\n%s", err, out)
	}
	datosJavac, err := os.ReadFile(filepath.Join(javacOut, "com", "hola", "Uso.class"))
	if err != nil {
		t.Fatal(err)
	}
	cf, err := readClassFull(datosJavac)
	if err != nil {
		t.Fatal(err)
	}
	javacDex, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatal(err)
	}
	if len(nuestroDex) != len(javacDex) {
		t.Fatalf("el dex del consumidor mide %d bytes y el de javac %d", len(nuestroDex), len(javacDex))
	}
	for i := range nuestroDex {
		if nuestroDex[i] != javacDex[i] {
			t.Fatalf("el dex difiere de javac en el byte %d: %#02x frente a %#02x", i, nuestroDex[i], javacDex[i])
		}
	}
}
