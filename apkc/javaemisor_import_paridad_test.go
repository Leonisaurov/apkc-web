//go:build !js

package main

// La paridad con javac 17 del §229: el dex entero byte a byte para los dos órdenes que el frente midió,
// y el caso del mismo paquete que javac resuelve por el import (JLS 6.5.5.1). Bajo `GOOS=js` no hay
// proceso que lanzar, así que el fichero lleva `//go:build !js`; los casos que solo miran lo emitido
// viven en `javaemisor_import_test.go`, sin tag. Estas pruebas **no se saltan**: sin javac 17 fallan
// (ver `dependencias_test.go:5-8`).

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestEmisorImportExplicitoContraJavac17 fija el dex entero byte a byte para el orden en que el bug se
// veía de verdad (el homónimo `dos` se registra antes que el importado `uno`).
func TestEmisorImportExplicitoContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	main := fuenteMainImport("com.x.app", "com.x.uno")
	fuentes := []fuenteJava{
		{nombre: "src/com/x/app/Main.java", contenido: main},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperImportDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperImportUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "src/com/x/app/Main.java", main)
	pon(t, raiz, "src/com/x/dos/Helper.java", fuenteHelperImportDos)
	pon(t, raiz, "src/com/x/uno/Helper.java", fuenteHelperImportUno)
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

// TestEmisorImportExplicitoMismoPaqueteContraJavac17 fija el caso que el mensaje del commit afirma y
// que los dos casos anteriores no cubrían: `Main` está en el mismo paquete que un `Helper` e importa
// el de otro. javac se queda con el import (JLS 6.5.5.1), así que `Helper.doble` tiene que ser
// com/x/uno/Helper y el dex entero salir byte a byte.
func TestEmisorImportExplicitoMismoPaqueteContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	main := fuenteMainImport("com.x.dos", "com.x.uno")
	fuentes := []fuenteJava{
		{nombre: "src/com/x/dos/Main.java", contenido: main},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperImportDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperImportUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}
	mainEmitida := porInterno(nuestras, "com/x/dos/Main")
	if mainEmitida == nil {
		t.Fatalf("no salió Main: %s", nombresDe(nuestras))
	}
	if !bytes.Contains(mainEmitida.datos, []byte("com/x/uno/Helper")) {
		t.Errorf("el import com.x.uno.Helper no ganó al homónimo del mismo paquete: %s", volcadoDeClases(nuestras))
	}
	if bytes.Contains(mainEmitida.datos, []byte("com/x/dos/Helper")) {
		t.Errorf("el import com.x.uno.Helper perdió contra el Helper del mismo paquete: %s", volcadoDeClases(nuestras))
	}

	raiz := t.TempDir()
	pon(t, raiz, "src/com/x/dos/Main.java", main)
	pon(t, raiz, "src/com/x/dos/Helper.java", fuenteHelperImportDos)
	pon(t, raiz, "src/com/x/uno/Helper.java", fuenteHelperImportUno)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "src/com/x/dos/Main.java"),
		filepath.Join(raiz, "src/com/x/dos/Helper.java"),
		filepath.Join(raiz, "src/com/x/uno/Helper.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	for _, nombre := range []string{"Lcom/x/dos/Main;", "Lcom/x/dos/Helper;", "Lcom/x/uno/Helper;"} {
		nuestra, suya := buscarClaseEmitida(t, nuestras, nombre, deJavac)
		compararMetodos(t, nuestra, suya)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
