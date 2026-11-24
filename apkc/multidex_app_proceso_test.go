//go:build !js

package main

// El aborto de la guarda del dex primario, del lado del build: las dos pruebas de aquí arman un
// proyecto de 70×1000 métodos y conducen `_build` en un hijo de la propia prueba —`exec.Command(os.Args[0])`—
// porque `LogF` termina el proceso y hay que leer su salida. Bajo `GOOS=js` no hay proceso que
// lanzar, así que la mitad nativa vive aquí con build tag y no con `t.Skip`, que es la doctrina de
// `dependencias_test.go`: un SKIP dejaría la suite en verde sin haber medido nada.
//
// La mitad wasm de lo mismo está en `multidex_app_test.go`, sin tag, con la contrapartida que sí se
// puede medir sin hijo: el manifiesto da la API que se pasa al reparto (`manifestMinAPI`) y
// `dexarAppEnGo` rechaza el mismo conjunto de clases con el mismo mensaje (main-dex, el techo,
// `classes2.dex` y la API), sin escribir ningún dex.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// construirAppGrandeEnHijo arma el proyecto de 70×1000 métodos con el manifiesto dado y conduce
// `_build` en un hijo de la propia prueba, porque `LogF` termina el proceso. Devuelve la raíz del
// proyecto (donde el padre ya está situado), la salida combinada del hijo y si esta ejecución es
// el hijo. En el hijo, un `_build` que **vuelve** (la guarda no abortó) termina la prueba en verde
// y el padre lo lee como «no abortó»; el `return` con `esHijo` de cada prueba corta ahí.
func construirAppGrandeEnHijo(t *testing.T, nombreTest, manifiesto string) (raiz string, salida []byte, esHijo bool) {
	t.Helper()
	const (
		envDir = "APKC_APP_GRANDE_DIR"
		envJar = "APKC_APP_GRANDE_JAR"
		envKS  = "APKC_APP_GRANDE_KS"
	)
	if dir := os.Getenv(envDir); dir != "" {
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		androidJar = os.Getenv(envJar)
		if androidJar == "" {
			t.Fatalf("el hijo no recibió %s", envJar)
		}
		_build(buildOpts{
			incremental: true,
			linker:      linkerGo,
			compiler:    compilerGo,
			signer:      signerGo,
			keyStore:    os.Getenv(envKS),
			storePass:   "android",
			keyAlias:    "androiddebugkey",
			jar:         androidJar,
			sdkVersion:  34,
			codename:    "14",
			v4:          true,
		})
		return "", nil, true
	}

	const clases, metodos = 70, 1000
	androidJar = jarDelSDK(t)
	setupProject(t)
	raiz, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	keystore := filepath.Join(raiz, "debug.keystore")
	if err := escribirDebugKeystore(keystore, "android", "androiddebugkey", 10000); err != nil {
		t.Fatalf("escribirDebugKeystore: %v", err)
	}
	writeManifest(t, manifiesto)
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Grande</string>
</resources>`)
	for i := 0; i < clases; i++ {
		pon(t, raiz, fmt.Sprintf("src/grande/C%d.java", i), fuenteDeMetodos(fmt.Sprintf("C%d", i), metodos))
	}
	pon(t, raiz, "src/grande/Uso.java", fuenteLlamadora(clases, metodos))

	cmd := exec.Command(os.Args[0], "-test.run=^"+nombreTest+"$")
	cmd.Env = append(os.Environ(), envDir+"="+raiz, envJar+"="+androidJar, envKS+"="+keystore)
	salida, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("el build con 70 000 referencias y este manifiesto no abortó; salida:\n%s", salida)
	}
	return raiz, salida, false
}

// TestCompilaAppPre21ConDemasiadasClasesAborta es la guarda del otro lado de la frontera: con
// `minSdkVersion` 19 las clases del proyecto son fijas al dex primario y aquí no caben —70 clases y
// su llamadora piden más de 65 536 referencias—, así que el reparto no puede existir: el build
// tiene que abortar con el motivo de main-dex, el techo y la API en el mensaje, y no dejar ningún
// apk. La falsificación —quitar la fijación— se nota en el código de salida y en el `app.apk` que
// entonces sí aparece con las clases del proyecto repartidas.
func TestCompilaAppPre21ConDemasiadasClasesAborta(t *testing.T) {
	raiz, salida, esHijo := construirAppGrandeEnHijo(t, "TestCompilaAppPre21ConDemasiadasClasesAborta",
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.myapp">
	<uses-sdk android:minSdkVersion="19" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name"/>
</manifest>`)
	if esHijo {
		return
	}
	for _, quiero := range []string{"main-dex", "65536", "API 19", "classes2.dex"} {
		if !bytes.Contains(salida, []byte(quiero)) {
			t.Errorf("el aborto no nombra %q; salida:\n%s", quiero, salida)
		}
	}
	if _, err := os.Stat(filepath.Join(raiz, "build", "app.apk")); err == nil {
		t.Fatal("el build abortado dejó build/app.apk: el paquete no puede existir")
	}
	if _, err := os.Stat(filepath.Join(raiz, "build", "classes2.dex")); err == nil {
		t.Fatal("el build abortado dejó build/classes2.dex: no se escribe un dex que ese runtime no abre")
	}
	t.Logf("aborto con rc != 0; salida del build:\n%s", salida)
}

// TestCompilaAppSinUsesSdkConDemasiadasClasesAborta cubre el hueco de la guarda: sin `<uses-sdk>`
// `manifestMinAPI` devuelve 0, y el build trata ese 0 como API 1 (`manifest.go`: d8 asume API 1
// cuando no está declarada). Por eso el cero también fija todas las clases del proyecto, y como no
// caben aborta, igual que en la llamada directa `dexarAppEnGo(…, 0)` con la que el crítico destapó
// que la guarda se eludía.
func TestCompilaAppSinUsesSdkConDemasiadasClasesAborta(t *testing.T) {
	raiz, salida, esHijo := construirAppGrandeEnHijo(t, "TestCompilaAppSinUsesSdkConDemasiadasClasesAborta",
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.myapp">
	<application android:label="@string/app_name"/>
</manifest>`)
	if esHijo {
		return
	}
	for _, quiero := range []string{"main-dex", "65536", "API 1", "classes2.dex"} {
		if !bytes.Contains(salida, []byte(quiero)) {
			t.Errorf("el aborto no nombra %q; salida:\n%s", quiero, salida)
		}
	}
	if _, err := os.Stat(filepath.Join(raiz, "build", "app.apk")); err == nil {
		t.Fatal("el build sin <uses-sdk> dejó build/app.apk: el paquete no puede existir")
	}
	if _, err := os.Stat(filepath.Join(raiz, "build", "classes2.dex")); err == nil {
		t.Fatal("el build sin <uses-sdk> dejó build/classes2.dex: la guarda se eludió con api cero")
	}
	// la sonda exacta del crítico, en proceso: api cero es API 1 y tiene que rechazar el reparto
	if _, err := dexarAppEnGo(clasesDeLaAppSonGrandes(), 0); err == nil {
		t.Fatal("dexarAppEnGo(…, 0) repartió: api cero no es «sin proyecto», es API 1")
	}
	t.Logf("aborto con rc != 0; salida del build:\n%s", salida)
}
