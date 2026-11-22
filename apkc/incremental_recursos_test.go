package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// El incremental tiene que notar que `res/` cambió, no solo que se borró un fichero: con
// `--linker go` el enlazador lee los recursos directamente, y `compileRes` devolvía siempre false
// ("no hay flats que compilar"), así que `bundleRes` no corría y se servía el apk anterior. Este
// test lo mide por el contenido: la cadena nueva tiene que estar en `resources.arsc`, y el apk
// tiene que ser otro y seguir verificando.
func TestIncrementalReenlazaCuandoUnRecursoEsNuevoOEditado(t *testing.T) {
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

	writeManifest(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.myapp">
	<uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name"/>
</manifest>`)
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Recursos</string>
    <string name="saludo">Hola</string>
</resources>`)
	pon(t, raiz, "res/layout/main.xml", `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="match_parent">
    <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" />
</LinearLayout>`)
	// un fuente que no lee la R: así el cambio de recursos no puede colarse por una recompilación
	pon(t, raiz, "src/com/myapp/Hola.java", "package com.myapp;\n\npublic class Hola {\n    static int n() { return 1; }\n}\n")

	opciones := buildOpts{
		incremental: true,
		linker:      linkerGo,
		compiler:    compilerGo,
		signer:      signerGo,
		keyStore:    keystore,
		storePass:   "android",
		keyAlias:    "androiddebugkey",
		jar:         androidJar,
		sdkVersion:  34,
		codename:    "14",
		// la v4 encendida porque es el valor por defecto del proceso: `_build` deja
		// `escribirV4` como se lo den, y apagarlo aquí cambiaría el estado global que leen
		// las pruebas de firma que corren después
		v4: true,
	}
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(opciones)
	frio := huellaApk(t)
	if contieneEnArsc(t, "CRITICO_NUEVA_CADENA_7391") {
		t.Fatal("la cadena de la prueba ya estaba antes de añadirla")
	}

	// alta: una cadena que no existía
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Recursos</string>
    <string name="saludo">Hola</string>
    <string name="critico_marcador">CRITICO_NUEVA_CADENA_7391</string>
</resources>`)
	_build(opciones)
	alta := huellaApk(t)
	if !contieneEnArsc(t, "CRITICO_NUEVA_CADENA_7391") {
		t.Fatal("con --inc, una cadena nueva no llegó a resources.arsc: no se reenlazó")
	}
	if alta == frio {
		t.Fatal("con --inc, añadir una cadena dejó el mismo apk: el paquete no se rehizo")
	}
	verificada(t)

	// edición: el valor de una cadena que ya existía, sin tocar su nombre
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Recursos</string>
    <string name="saludo">Hola editado 5521</string>
    <string name="critico_marcador">CRITICO_NUEVA_CADENA_7391</string>
</resources>`)
	_build(opciones)
	edicion := huellaApk(t)
	if !contieneEnArsc(t, "Hola editado 5521") {
		t.Fatal("con --inc, editar una cadena no llegó a resources.arsc: no se reenlazó")
	}
	if edicion == alta {
		t.Fatal("con --inc, editar una cadena dejó el mismo apk: el paquete no se rehizo")
	}
	verificada(t)

	// y una vuelta sin tocar nada no rehace el paquete, que es lo que el incremental viene a hacer
	_build(opciones)
	if huellaApk(t) != edicion {
		t.Fatal("una vuelta sin cambios rehizo el apk")
	}
}

// huellaApk es el SHA-256 del apk: dos builds con el mismo contenido de entrada tienen que dar el
// mismo fichero, y uno con un recurso nuevo tiene que dar otro.
func huellaApk(t *testing.T) string {
	t.Helper()
	datos, err := os.ReadFile(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("no hay build/app.apk: %v", err)
	}
	suma := sha256.Sum256(datos)
	return string(suma[:])
}

// contieneEnArsc busca el texto tal cual en la tabla de recursos: el enlazador de Go escribe el
// pool de cadenas en UTF-8, así que un valor nuevo es un trozo de bytes que está o no está.
func contieneEnArsc(t *testing.T, texto string) bool {
	t.Helper()
	datos, err := os.ReadFile(filepath.Join("build", "resources.arsc"))
	if err != nil {
		t.Fatalf("no hay build/resources.arsc: %v", err)
	}
	return bytes.Contains(datos, []byte(texto))
}

// verificada exige que el apk firmado en Go siga pasando el verificador del propio binario.
func verificada(t *testing.T) {
	t.Helper()
	v, err := verificarApk(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("verificarApk: %v", err)
	}
	if !v.v1 || !v.v2 {
		t.Fatalf("el apk incremental no verifica: v1=%v v2=%v", v.v1, v.v2)
	}
}
