//go:build !js

package main

// El lado de javac 17 del rechazo de `boolean` como argumento que ensancha: las cinco sondas de
// `booleanSondas` mal compiladas por él, que es lo que fija que el subconjunto y la JVM están de
// acuerdo en que el fuente es inválido. Bajo `GOOS=js` no hay proceso que lanzar, así que el fichero
// lleva `//go:build !js`; el rechazo de nuestro emisor vive en `javaemisor_boolean_ensancha_test.go`,
// sin tag. Esta prueba **no se salta**: sin javac 17 falla (ver `dependencias_test.go:5-8`).

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestElBooleanNoEnsanchaContraJavac17: javac 17 rechaza las cinco sondas con `cannot be converted` o
// `incompatible types`, y nunca las acepta.
func TestElBooleanNoEnsanchaContraJavac17(t *testing.T) {
	javac := javac17Exigido(t)
	for _, c := range booleanSondas {
		t.Run(c.nombre, func(t *testing.T) {
			raiz := t.TempDir()
			pon(t, raiz, "S.java", c.fuente)
			salida, errJavac := exec.Command(javac, "--release", "8", "-d", raiz, filepath.Join(raiz, "S.java")).CombinedOutput()
			if errJavac == nil {
				t.Fatalf("javac 17 aceptó %s: el oráculo no es el esperado", c.nombre)
			}
			if !strings.Contains(string(salida), "cannot be converted") && !strings.Contains(string(salida), "incompatible types") {
				t.Logf("javac rechazó con: %s", salida)
			}
		})
	}
}

// TestElControlDeLosAnchosQueSiEnsanchan: `int`/`byte`/`short`/`char` siguen compilando y el dex sale
// byte a byte como el de javac 17. Es el control de que quitar `Z` no se llevó por delante los anchos
// de verdad.
func TestElControlDeLosAnchosQueSiEnsanchan(t *testing.T) {
	_, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/S.java", contenido: controlEnsanchaFuente}})
	comparaDexByteAByte(t, nuestro, suyo, "control_anchos")
}
