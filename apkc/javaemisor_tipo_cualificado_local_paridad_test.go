//go:build !js

package main

// La paridad con javac 17 del §253: el `.class` de la declaración local con el tipo cualificado dice lo
// mismo que el de javac 17 en los métodos, en la LineNumberTable y en el dex entero. Bajo `GOOS=js` no
// hay proceso que lanzar, así que el fichero lleva `//go:build !js`; el caso que solo mira lo emitido
// vive en `javaemisor_tipo_cualificado_local_test.go`, sin tag. Esta prueba **no se salta**: sin javac
// 17 falla (ver `dependencias_test.go:5-8`).

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTipoCualificadoLocalContraJavac17: el `.class` de la declaración local con el tipo cualificado
// tiene que decir lo mismo que el de javac 17 en los métodos, en la LineNumberTable y en el dex entero.
func TestTipoCualificadoLocalContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "src/share/apps/util/MainActivity.java", fuenteTipoCualificadoLocal)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteTipoCualificadoLocal},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el tipo cualificado de una declaración local: %v", err)
	}
	emitida := porInterno(nuestras, "share/apps/util/MainActivity")
	if emitida == nil {
		t.Fatalf("no salió MainActivity (%s)", nombresDe(nuestras))
	}
	// la variable es del tipo anidado, con el `$` con el que javac lo nombra: no el primer segmento
	if !bytes.Contains(emitida.datos, []byte("Landroid/app/AlertDialog$Builder;")) {
		t.Errorf("la declaración no apunta a android/app/AlertDialog$Builder: %s", volcadoDeClases(nuestras))
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "src/share/apps/util/MainActivity.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/MainActivity;", deJavac)

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
