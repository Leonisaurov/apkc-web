package main

// La colisión de una clase entre dos dependencias: d8 aborta con «Type X is defined multiple
// times», pero el dexer de este fork concatenaba las clases de cada jar y dejaba las dos copias en
// el dex, con el runtime eligiendo una sin que el build lo dijera. La guarda vive en
// `dependenciasDexeables`, que es la lista común de los dos caminos, así que se prueba ahí mismo:
// sin colisión el build sigue, y con colisión el proceso muere nombrando la clase y los dos jars.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claseDePrueba es un `.class` con la cabecera mínima: la guarda solo mira nombres de entrada del
// jar, así que no necesita bytes válidos y la prueba no depende de javac ni del SDK.
var claseDePrueba = []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x34}

// jarConClase monta un jar de `jar/` con una sola entrada.
func jarConClase(t *testing.T, raiz, nombre, clase string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", nombre), map[string][]byte{clase + ".class": claseDePrueba})
}

// TestClaseDuplicadaEntreJarsNombraLaClaseYLosDosJars mide la guarda sola: un jar no colisiona
// consigo mismo, dos jars con clases distintas tampoco, y dos con la misma clase devuelven un error
// que nombra la clase y las dos rutas.
func TestClaseDuplicadaEntreJarsNombraLaClaseYLosDosJars(t *testing.T) {
	raiz := t.TempDir()
	jarConClase(t, raiz, "a.jar", "dup/A")
	jarConClase(t, raiz, "b.jar", "dup/B")
	a := filepath.Join(raiz, "jar", "a.jar")
	b := filepath.Join(raiz, "jar", "b.jar")

	if err := claseDuplicadaEntreJars([]string{a}); err != nil {
		t.Fatalf("un solo jar colisionó consigo mismo: %v", err)
	}
	if err := claseDuplicadaEntreJars([]string{a, b}); err != nil {
		t.Fatalf("dos jars con clases distintas colisionaron: %v", err)
	}
	// el `module-info.class` de la raíz de dos jars modulares no es una colisión: d8 lo descarta
	for nombre, clase := range map[string]string{"a.jar": "dup/A", "b.jar": "dup/B"} {
		jarDePrueba(t, filepath.Join(raiz, "jar", nombre), map[string][]byte{
			"module-info.class":  claseDePrueba,
			"package-info.class": claseDePrueba,
			clase + ".class":     claseDePrueba,
		})
	}
	if err := claseDuplicadaEntreJars([]string{a, b}); err != nil {
		t.Fatalf("dos jars modulares colisionaron por module-info o package-info: %v", err)
	}

	jarConClase(t, raiz, "b.jar", "dup/A")
	err := claseDuplicadaEntreJars([]string{a, b})
	if err == nil {
		t.Fatal("dos jars que definen dup.A pasaron sin colisión")
	}
	for _, quiero := range []string{"dup.A", "a.jar", "b.jar"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("la colisión no nombra %q: %v", quiero, err)
		}
	}
}

// TestDependenciasDexeablesSigueConDosJarsSinColision: la guarda no puede estorbar al caso normal de
// un `jar/` con varias dependencias. Se ejecuta en proceso porque aquí no hay aborto.
func TestDependenciasDexeablesSigueConDosJarsSinColision(t *testing.T) {
	raiz := t.TempDir()
	jarConClase(t, raiz, "a.jar", "dup/A")
	jarConClase(t, raiz, "b.jar", "dup/B")
	t.Chdir(raiz)

	rutas, digests := dependenciasDexeables()
	if len(rutas) != 2 || len(digests) != 2 {
		t.Fatalf("%d rutas y %d huellas, medidas dos de cada", len(rutas), len(digests))
	}
}

// TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase vive en
// `dependencias_dup_proceso_test.go` con `//go:build !js`: conduce la guarda en un proceso hijo y en
// la pestaña no hay procesos que lanzar.
