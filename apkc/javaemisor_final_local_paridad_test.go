//go:build !js

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_final_local_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_final_local_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestFinalEnDeclaracionLocalContraJavac17: el `.class` de las tres declaraciones locales con `final`
// tiene que decir lo mismo que el de javac 17 en `max_stack`/`locals`, en la LineNumberTable y en el
// dex entero. Y ni el suyo ni el nuestro pueden llevar tabla de variables locales: con
// `-g:lines,source` javac no la escribe, así que el `final` no viaja al fichero.
func TestFinalEnDeclaracionLocalContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Finales.java", fuenteFinalLocal)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Finales.java", contenido: fuenteFinalLocal},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el `final` de una declaración local: %v", err)
	}
	emitida := porInterno(nuestras, "com/hola/apkcstudio/Finales")
	if emitida == nil {
		t.Fatalf("no salió Finales (%s)", nombresDe(nuestras))
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Finales.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	nuestra, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/Finales;", deJavac)

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}

	// el `final` no deja tabla de locales: javac la omitiría con `-g:vars`, y aquí se comprueba que
	// tampoco la inventa nadie en la ruta nueva
	deJavacCrudo, err := os.ReadFile(filepath.Join(salida, "com/hola/apkcstudio/Finales.class"))
	if err != nil {
		t.Fatalf("leyendo el .class de javac: %v", err)
	}
	for _, aguja := range []string{"LocalVariableTable", "LocalVariableTypeTable"} {
		if bytes.Contains(deJavacCrudo, []byte(aguja)) {
			t.Fatalf("el .class de javac lleva %s: el fuente no es el medido", aguja)
		}
		if bytes.Contains(emitida.datos, []byte(aguja)) {
			t.Errorf("nuestro .class lleva %s y el `final` de una local no lo escribe", aguja)
		}
	}
}
