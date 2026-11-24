//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_porcada_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_porcada_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestPorCadaDeAppContraJavac17: el `getApks` de `App.java` con la línea 37 verbatim, compilado por
// nosotros y por javac 17, tiene que dar el mismo dex byte a byte. Es la forma exacta que el usuario
// estaba compilando: `for (String path: info.splitSourceDirs)` sobre un `String[]` que viene de un
// campo de un parámetro, dentro de un método de instancia.
func TestPorCadaDeAppContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{
		{nombre: "Apk.java", contenido: fuenteApkDeApkShare},
		{nombre: "App.java", contenido: fuenteAppPorCada},
	}
	nuestras, deJavac := compilaLosDos(t, jar, javac, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/App;", deJavac)
	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
	// y la fila del `for` cae donde javac la pone: en el `astore` del arreglo copiado
	lineaFor := lineaDeLaSonda(t, fuenteAppPorCada, "for (String path:")
	if _, ok := mapaDeLineas(t, suya, "getApks")[lineaFor]; !ok {
		t.Fatalf("javac no anota la línea %d (la del `for`) en `getApks` (filas: %v)", lineaFor, mapaDeLineas(t, suya, "getApks"))
	}
}

// TestPorCadaDeLasDosFormasContraJavac17: la sonda de las formas que App.java no tiene —arreglo de
// primitivos, `Iterable` por interface y por clase, `continue`, `break` y elemento `Object`— contra
// javac 17, byte a byte.
func TestPorCadaDeLasDosFormasContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	nuestras, deJavac := compilaLosDos(t, jar, javac,
		[]fuenteJava{{nombre: "Cada.java", contenido: fuenteCada}})

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Cada;", deJavac)
	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
