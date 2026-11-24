//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_campo_local_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_campo_local_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestCampoDeLocalDeApkShareContraJavac17: el `App.java` del zip, entero y literal, por los dos
// caminos. Se compara el dex de **`App` solo**: `Apk.java` trae una diferencia ajena a estas dos ramas
// —no pliega `GET_ACTIVITIES | GET_META_DATA` en `129`— que este frente no toca. El resto de `App`
// entra igual: el constructor, `getApks`, el `for` con dos puntos y `toString`.
func TestCampoDeLocalDeApkShareContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "src/share/apps/util/App.java", fuenteAppApkShare)
	pon(t, raiz, "src/share/apps/util/Apk.java", fuenteApkApkShare)

	nuestras := nuestrasClases(t, jar, []fuenteJava{
		{nombre: "src/share/apps/util/App.java", contenido: fuenteAppApkShare},
		{nombre: "src/share/apps/util/Apk.java", contenido: fuenteApkApkShare},
	})
	suyas := clasesDeJavac17(t, jar, javac, raiz, "src/share/apps/util/App.java", "src/share/apps/util/Apk.java")
	compararDex(t, laClaseNuestra(t, nuestras, "share/apps/util/App"), laClaseDeJavac(t, suyas, "Lshare/apps/util/App;"))
}

// TestCampoDeLocalComoArgumentoContraJavac17: la rama del `App.java:28` aislada. El argumento
// `info.applicationInfo` se estima desde el tipo estático del parámetro `info`, sin emitir su receptor.
func TestCampoDeLocalComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Argumento.java", fuenteCampoDeLocal)

	nuestras := nuestrasClases(t, jar, []fuenteJava{{nombre: "Argumento.java", contenido: fuenteCampoDeLocal}})
	compararDex(t, nuestras, clasesDeJavac17(t, jar, javac, raiz, "Argumento.java"))
}

// TestArregloComparadoConNullContraJavac17: la rama del `App.java:34` aislada. javac escribe
// `aload_0; getfield splitSourceDirs:[Ljava/lang/String;; ifnonnull` — el mismo `ifnonnull` de una
// referencia `L`, porque un arreglo también lo es.
func TestArregloComparadoConNullContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Nulo.java", fuenteArregloNulo)

	nuestras := nuestrasClases(t, jar, []fuenteJava{{nombre: "Nulo.java", contenido: fuenteArregloNulo}})
	compararDex(t, nuestras, clasesDeJavac17(t, jar, javac, raiz, "Nulo.java"))
}
