package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAsignacionNulaAUnCampoContraJavac17: `icon = null;` con icon de tipo Drawable. La ruta de campo
// usaba `exp`, que tipa el null como Object; debe usar `expA` con el descriptor del campo, como ya
// hacía la asignación a una variable local. El dex tiene que salir byte a byte el de javac 17.
func TestAsignacionNulaAUnCampoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteNuloEnCampo)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteNuloEnCampo}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `icon = null;`: %v", err)
	}
	nuestra, err := readClassFull(nuestras[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	dexNuestro, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	dexJavac, err := dexDeClases(deJavac)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s", i, dexNuestro[i], dexJavac[i], volcadoDeClases(nuestras))
		}
	}
}
