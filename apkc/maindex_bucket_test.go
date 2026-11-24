package main

// El bucket de dependencias por debajo de API 21: qué se mide aquí y qué queda fuera.
//
// El crítico de dependencias dejó abierto que `dexarBucketEnGo` (build.go) dexara el bucket sin
// la lista de clases fijas de `fijasParaApi` (compilar.go), con el riesgo de que
// `androidx/multidex/…` cayera en el `classes2.dex` del bucket y el runtime de API < 21 no lo
// abriera. Esta prueba lo resuelve por el contrato de bytes:
//
//   - el build con `minSdkVersion` 19 no dexa ningún bucket, porque el reparto de dependencias
//     empieza en 21 (`splitDeps := api >= 21`, build.go:282): las dependencias entran en el merge
//     del dex de la app, que sí recibe las fijas, así que el cargador viaja en `classes.dex` y el
//     resto en `classes2.dex`;
//   - la falsificación enseña que el mecanismo existe en aislamiento: el mismo conjunto de `jar/`
//     dexado por `dexarBucketEnGo` sin fijas deja el cargador en su segundo dex. No llega al
//     paquete porque ese camino no se toma por debajo de 21;
//   - en 21 el bucket sí se dexa, sin fijas (que `fijasParaApi` devuelve nil), y el build sigue
//     en verde con cada clase en un dex del paquete.
//
// Lo que no se mide aquí: el AAR real de androidx.multidex y el arranque en un Dalvik de API ≤ 20,
// que no están en el árbol ni hay dónde ejecutarlos.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// dependenciasDeLaPrueba escribe los dos jar/ del caso: 70 clases de 1000 métodos (70 000
// method_id, más de un dex) y, detrás en el orden de `jar/`, el cargador de multidex. El orden y el
// nombre importan: con dos jar/ hay un solo bucket (`depBuckets`), el que desborda.
func dependenciasDeLaPrueba(t *testing.T, raiz string) []string {
	t.Helper()
	grande := map[string][]byte{}
	for i := 0; i < 70; i++ {
		grande["grande/C"+itoa(i)+".class"] = claseFixtureGrande(t, "grande/C"+itoa(i), 1000)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "a-grande.jar"), grande)
	jarDePrueba(t, filepath.Join(raiz, "jar", "z-multidex.jar"), map[string][]byte{
		"androidx/multidex/MultiDex.class": claseFixtureGrande(t, "androidx/multidex/MultiDex", 2),
	})
	return []string{
		filepath.Join(raiz, "jar", "a-grande.jar"),
		filepath.Join(raiz, "jar", "z-multidex.jar"),
	}
}

// proyectoPre21 deja un proyecto mínimo con `minSdkVersion` 19, que es el que manda en `api`.
func proyectoPre21(t *testing.T, raiz string) string {
	t.Helper()
	keystore := filepath.Join(raiz, "debug.keystore")
	if err := escribirDebugKeystore(keystore, "android", "androiddebugkey", 10000); err != nil {
		t.Fatalf("escribirDebugKeystore: %v", err)
	}
	writeManifest(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.myapp">
	<uses-sdk android:minSdkVersion="19" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name"/>
</manifest>`)
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Bucket</string>
</resources>`)
	pon(t, raiz, "src/grande/Arranque.java", `package grande;

public class Arranque {
    public static int arranca() { return 1; }
}
`)
	return keystore
}

// TestBucketDeDependenciasNoSeDexaBajoApi21 cierra el hallazgo por el lado negativo: bajo API 21
// no hay bucket que dexar, la app y sus dependencias comparten el merge que sí lleva las fijas, y
// la falsificación del dexado sin fijas deja el cargador fuera del primer dex del bucket.
func TestBucketDeDependenciasNoSeDexaBajoApi21(t *testing.T) {
	dexdump := herramientaDelSDK(t, "dexdump")
	if dexdump == "" {
		t.Skip("sin dexdump en las build-tools: el reparto del bucket no se puede leer")
	}
	setupProject(t)
	raiz, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	keystore := proyectoPre21(t, raiz)
	jars := dependenciasDeLaPrueba(t, raiz)

	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(buildOpts{
		incremental: true,
		linker:      linkerGo,
		compiler:    compilerGo,
		signer:      signerGo,
		keyStore:    keystore,
		storePass:   "android",
		keyAlias:    "androiddebugkey",
		jar:         jarDelSDK(t),
		sdkVersion:  34,
		codename:    "14",
	})

	// el bucket no se dexó: cualquier resto de su caché o de su directorio sería la prueba de que
	// el camino prohibido se tomó con API 19
	restos, err := filepath.Glob(filepath.Join("build", "dexcache", "deps-go*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(restos) != 0 {
		t.Fatalf("con minSdkVersion 19 el bucket de dependencias se dexó: %v", restos)
	}

	// la app: las fijas en el dex primario y el resto en el segundo
	primero := clasesDeDex(t, filepath.Join("build", "classes.dex"))
	segundo := clasesDeDex(t, filepath.Join("build", "classes2.dex"))
	if !primero["Lgrande/Arranque;"] {
		t.Fatalf("classes.dex no lleva la clase del proyecto: %v", nombresDeConjunto(primero))
	}
	if !primero["Landroidx/multidex/MultiDex;"] {
		t.Fatalf("classes.dex no lleva el cargador de multidex: %v", nombresDeConjunto(primero))
	}
	if !segundo["Lgrande/C7;"] {
		t.Fatalf("classes2.dex no lleva el resto de las dependencias: %v", nombresDeConjunto(segundo))
	}
	if primero["Lgrande/C7;"] {
		t.Fatal("classes.dex lleva una clase que no estaba fijada: el reparto no siguió las fijas")
	}
	if segundo["Landroidx/multidex/MultiDex;"] {
		t.Fatal("el cargador de multidex quedó también en classes2.dex")
	}

	// el oráculo del dex, sobre los dos ficheros que el paquete lleva
	for _, caso := range []struct {
		ruta   string
		quiero []string
	}{
		{filepath.Join("build", "classes.dex"), []string{"Lgrande/Arranque;", "Landroidx/multidex/MultiDex;"}},
		{filepath.Join("build", "classes2.dex"), []string{"Lgrande/C7;"}},
	} {
		salida, err := exec.Command(dexdump, "-f", caso.ruta).CombinedOutput()
		if err != nil {
			t.Fatalf("dexdump falló sobre %s: %v\n%s", caso.ruta, err, salida)
		}
		for _, quiero := range caso.quiero {
			if !bytes.Contains(salida, []byte(quiero)) {
				t.Fatalf("dexdump no ve %s en %s", quiero, caso.ruta)
			}
		}
	}

	// el paquete: dos dex, el de la app y su reparto, y ninguno más; y firmado, que es la prueba de
	// que el build llegó al final
	if exists(filepath.Join("build", "classes3.dex")) {
		t.Fatal("build/ dejó un tercer dex: un bucket se dexó pese a API 19")
	}
	entradas := dexEntries("build", nil)
	if len(entradas) != 2 || entradas[0].name != "classes.dex" || entradas[1].name != "classes2.dex" {
		t.Fatalf("el paquete no lleva los dos dex esperados: %v", entradas)
	}
	v, err := verificarApk(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("verificarApk: %v", err)
	}
	if !v.v2 || !v.v3 {
		t.Fatalf("el apk no verifica v2/v3: v2=%v v3=%v", v.v2, v.v3)
	}

	// la falsificación: el mismo conjunto dexado como bucket, sin fijas, deja el cargador en el
	// segundo dex. Es el mecanismo que el hallazgo describe, y no llega al paquete porque con API 19
	// el bucket no se dexa (arriba).
	staging := filepath.Join(raiz, "bucket-sin-fijas")
	if err := os.MkdirAll(staging, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := dexarBucketEnGo(jars, staging); err != nil {
		t.Fatalf("dexando el bucket sin fijas: %v", err)
	}
	bucketUno := clasesDeDex(t, filepath.Join(staging, dexEntryName(1)))
	bucketDos := clasesDeDex(t, filepath.Join(staging, dexEntryName(2)))
	if bucketUno["Landroidx/multidex/MultiDex;"] {
		t.Fatal("sin fijas el cargador siguió en el primer dex del bucket: la falsificación no falsifica")
	}
	if !bucketDos["Landroidx/multidex/MultiDex;"] {
		t.Fatal("sin fijas el cargador no cayó en el segundo dex del bucket: el reparto no lo movió")
	}
	salida, err := exec.Command(dexdump, "-f", filepath.Join(staging, dexEntryName(2))).CombinedOutput()
	if err != nil {
		t.Fatalf("dexdump falló sobre el segundo dex del bucket: %v\n%s", err, salida)
	}
	if !bytes.Contains(salida, []byte("Landroidx/multidex/MultiDex;")) {
		t.Fatal("dexdump no ve el cargador en el segundo dex del bucket")
	}
	t.Logf("sin fijas el bucket deja el cargador en %s; con API 19 ese bucket no se dexa",
		filepath.Base(filepath.Join(staging, dexEntryName(2))))
}

// TestBucketDeDependenciasEnApi21SeDexeaSinFijas comprueba el otro lado del cambio: en 21 el
// reparto de dependencias sí corre y `fijasParaApi` devuelve nil, así que el bucket se dexa igual
// que antes. El cargador queda en uno de sus dex y la app no lo repite.
func TestBucketDeDependenciasEnApi21SeDexeaSinFijas(t *testing.T) {
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
    <string name="app_name">Bucket</string>
</resources>`)
	pon(t, raiz, "src/grande/Arranque.java", `package grande;

public class Arranque {
    public static int arranca() { return 1; }
}
`)
	dependenciasDeLaPrueba(t, raiz)

	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(buildOpts{
		incremental: true,
		linker:      linkerGo,
		compiler:    compilerGo,
		signer:      signerGo,
		keyStore:    keystore,
		storePass:   "android",
		keyAlias:    "androiddebugkey",
		jar:         jarDelSDK(t),
		sdkVersion:  34,
		codename:    "14",
	})

	// el bucket sí se dexó: con API 21 el runtime abre cada dex y la lista de fijas es nil
	buckets, err := filepath.Glob(filepath.Join("build", "dexcache", "deps-go-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) == 0 {
		t.Fatal("con minSdkVersion 21 el bucket de dependencias no se dexó")
	}
	enBucket := false
	vistos := []string{}
	for _, dir := range buckets {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			continue
		}
		for _, dex := range dexFilesIn(dir) {
			vistos = append(vistos, dex)
			if clasesDeDex(t, dex)["Landroidx/multidex/MultiDex;"] {
				enBucket = true
			}
		}
	}
	if !enBucket {
		t.Fatalf("el cargador no está en ningún dex del bucket: %v", vistos)
	}

	// la app no repite las clases del bucket y el paquete conserva la numeración
	if clasesDeDex(t, filepath.Join("build", "classes.dex"))["Landroidx/multidex/MultiDex;"] {
		t.Fatal("con API 21 la app volvió a dexar las dependencias")
	}
	if exists(filepath.Join("build", "app.apk")) {
		if _, err := verificarApk(filepath.Join("build", "app.apk")); err != nil {
			t.Fatalf("verificarApk: %v", err)
		}
	}
}
