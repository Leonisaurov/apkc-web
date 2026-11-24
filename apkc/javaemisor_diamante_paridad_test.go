//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_diamante_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_diamante_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestDiamanteDelNuevoContraJavac17: nuestro dex del caso real y el de javac 17 con `--release 8`
// tienen que ser el mismo byte a byte. Si el diamante no se consumiera, `compilarProyectoJava` ni
// llegaría aquí; si se consumiera mal, el constructor del borrado o la pila dejarían de coincidir.
func TestDiamanteDelNuevoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteApkDelDiamante)
	pon(t, raiz, "App.java", fuenteAppDiamante)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Apk.java", contenido: fuenteApkDelDiamante},
		{nombre: "App.java", contenido: fuenteAppDiamante},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `new ArrayList<>()`: %v", err)
	}
	emitida := porInterno(nuestras, "com/hola/apkcstudio/App")
	if emitida == nil {
		t.Fatalf("no salió App (%s)", nombresDe(nuestras))
	}
	nuestra, err := readClassFull(emitida.datos)
	if err != nil {
		t.Fatalf("nuestra App no se relee: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"), filepath.Join(raiz, "App.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	_, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/App;", deJavac)
	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
