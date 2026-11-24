//go:build !js

package main

// El aborto de `dependenciasDexeables` con dos jars que definen la misma clase se mide en un proceso
// hijo: la guarda termina con LogF, así que la única forma de ver el aborto —y de que la
// falsificación quite la guarda y la prueba lo note— es mirar el código de salida y la salida. Bajo
// `GOOS=js` no hay procesos hijos, así que esta mitad va con build tag y no con un `t.Skip`, que
// dejaría la pestaña en verde sin medir. Las dos pruebas puras de la guarda siguen en
// `dependencias_dup_test.go` y corren en los dos entornos.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase conduce `dependenciasDexeables`
// en un proceso hijo.
func TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase(t *testing.T) {
	if dir := os.Getenv("APKC_DUP_JARS"); dir != "" {
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		dependenciasDexeables()
		return
	}

	raiz := t.TempDir()
	jarConClase(t, raiz, "a.jar", "dup/D")
	jarConClase(t, raiz, "b.jar", "dup/D")
	cmd := exec.Command(os.Args[0], "-test.run=^TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase$")
	cmd.Env = append(os.Environ(), "APKC_DUP_JARS="+raiz)
	salida, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("el build no abortó con dos jars que definen dup.D; salida:\n%s", salida)
	}
	for _, quiero := range []string{"dup.D", "a.jar", "b.jar"} {
		if !strings.Contains(string(salida), quiero) {
			t.Errorf("el aborto no nombra %q; salida:\n%s", quiero, salida)
		}
	}
}
