//go:build !js

package main

// La paridad con javac 17 del §228: la forma de la referencia cualificada, método a método y en el dex
// entero byte a byte. Bajo `GOOS=js` no hay proceso que lanzar, así que el fichero lleva
// `//go:build !js`; la resolución del tipo, que no necesita javac, vive en
// `javaemisor_cualificado_test.go`, sin tag. Esta prueba **no se salta**: sin javac 17 falla (ver
// `dependencias_test.go:5-8`).

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestEmisorReferenciaCualificadaContraJavac17 fija la forma: los métodos y el dex entero, byte a byte.
func TestEmisorReferenciaCualificadaContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{
		{nombre: "src/com/x/app/Main.java", contenido: fuenteMainCualificado},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperCualificadoDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperCualificadoUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "src/com/x/app/Main.java", fuenteMainCualificado)
	pon(t, raiz, "src/com/x/dos/Helper.java", fuenteHelperCualificadoDos)
	pon(t, raiz, "src/com/x/uno/Helper.java", fuenteHelperCualificadoUno)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "src/com/x/app/Main.java"),
		filepath.Join(raiz, "src/com/x/dos/Helper.java"),
		filepath.Join(raiz, "src/com/x/uno/Helper.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	for _, nombre := range []string{"Lcom/x/app/Main;", "Lcom/x/dos/Helper;", "Lcom/x/uno/Helper;"} {
		nuestra, suya := buscarClaseEmitida(t, nuestras, nombre, deJavac)
		compararMetodos(t, nuestra, suya)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
