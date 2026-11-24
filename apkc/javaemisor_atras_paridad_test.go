//go:build !js

package main

// La paridad con javac 17 de la vuelta larga de un bucle (§ del frente de `goto_w`): el `Code` y las
// cabeceras byte a byte y el dex entero. Bajo `GOOS=js` no hay proceso que lanzar, así que el fichero
// lleva `//go:build !js`; la mitad que solo necesita el emisor vive en `javaemisor_atras_test.go`, sin
// tag. Esta prueba **no se salta**: sin javac 17 falla (ver `dependencias_test.go:5-8`).

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGotoWLargoHaciaAtrasContraJavac17: nuestro `.class` coincide con el de javac 17 en el cuerpo del
// método y el dex que sale de las dos clases es el mismo byte a byte.
func TestGotoWLargoHaciaAtrasContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	fuente := fuenteGotoWAtras()

	// oráculo 1: javac 17 sobre el mismo fuente, mismo `Code` y mismas cabeceras
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

	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Atras32.java", contenido: fuente}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la vuelta larga: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se relee: %v", err)
	}
	metodo, ok := metodoDeLaClase(nuestra, "atras")
	if !ok {
		t.Fatal("la clase no tiene `atras`")
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
