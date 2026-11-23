package main

// El reparto del dex de la app: hasta el §223 el reparto de buckets de dependencias estaba
// medido y el dex del proyecto seguía yendo por un solo fichero, así que un proyecto con más de
// 65 536 referencias propias reventaba con el error del índice de 16 bits (dexinstr.go). Estos
// tests miden las dos mitades de la frontera:
//
//   - la falsificación: con el reparto desactivado —la etapa de un solo dex, `dexDeClasesConCache`—
//     el mismo conjunto de clases falla con el error literal del índice, y con el reparto del
//     proyecto (`dexarAppEnGo`) sale en varios ficheros;
//   - la punta: `apkc build --linker go --compiler go` de una app generada con 70 000 métodos deja
//     `classes.dex`, `classes2.dex`, … en `build/` y en `app.apk`, firmados v1/v2/v3, alineados, y
//     el `dexdump` del SDK ve el método de la última clase en el último dex.
//
// Lo pequeño no cambia: el dex de una app que cabe en un fichero es el mismo byte a byte que el
// de la etapa de un solo dex, y su caché sigue siendo `build/dexcache.bin`.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// llamadorSintetico arma la clase que nombra un método de otra: un `invokestatic` con índice de
// pool 20 (Methodref #10.#21 -> destino.metodo:()I) y un `ireturn`. Es lo mínimo que hace que el
// traductor pida el `method_id` del destino, que es donde el índice de 16 bits se queda corto.
func llamadorSintetico(destino, metodo string) classFull {
	return classFull{
		Name:  "Lgrande/Uso;",
		Super: "Ljava/lang/Object;",
		p: &pool{
			utf:   map[int]string{1: destino, 2: metodo, 3: "()I", 4: "Lgrande/Uso;"},
			class: map[int]int{10: 1, 11: 4},
			tag:   map[int]byte{20: cpMethodref, 21: cpNameAndType},
			body: map[int][]byte{
				20: {0, 10, 0, 21},
				21: {0, 2, 0, 3},
			},
		},
		Methods: []classMethod{{
			Access: 0x9, Name: "usa", Desc: "()I",
			HasCode: true, MaxStack: 1,
			Code: []byte{0xb8, 0x00, 0x14, 0xac},
		}},
	}
}

// clasesDeLaAppSonGrandes devuelve el conjunto que no cabe en un dex: 70 clases con 1000 métodos
// declarados (70 000 method_id) y una que llama al último de ellos, para que el índice que se
// escribe en la instrucción pase de 0xffff y no solo la cuenta del fichero.
func clasesDeLaAppSonGrandes() []classFull {
	clases := clasesSinteticas(70, 1000, 0, 0)
	return append(clases, llamadorSintetico("grande/C69", "m999"))
}

// TestDexDeLaAppSeReparteYElViejoCaminoFalla es la falsificación: el mismo conjunto de clases
// dexado en un solo fichero tiene que fallar con el error literal del índice de 16 bits, y la
// etapa del proyecto tiene que repartirlo. Además comprueba lo que no puede cambiar: el dex de
// una app pequeña es el mismo byte a byte y su caché es `build/dexcache.bin`.
func TestDexDeLaAppSeReparteYElViejoCaminoFalla(t *testing.T) {
	clases := clasesDeLaAppSonGrandes()

	// desactivar el reparto es llamar a la etapa de un solo dex, que es la que estaba en
	// compilar.go:289 antes de esta frontera
	_, err := dexDeClasesConCache(clases, nil)
	if err == nil {
		t.Fatal("un solo dex con 70 000 métodos salió sin error: la falsificación no falsifica")
	}
	const literal = "does not fit a 16-bit index"
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("el camino sin reparto falló con %q, y el error del índice es %q", err, literal)
	}
	t.Logf("sin reparto: %v", err)

	// y con el reparto del proyecto sale en varios ficheros en build/
	setupProject(t)
	ultimoDexReuso = resumenCacheDex{}
	dexes, err := dexarAppEnGo(clases, 34)
	if err != nil {
		t.Fatalf("repartiendo el dex de la app: %v", err)
	}
	if len(dexes) < 2 {
		t.Fatalf("%d dex para 70 000 métodos: la app no se repartió", len(dexes))
	}
	for n := 1; n <= len(dexes); n++ {
		ruta := filepath.Join("build", dexEntryName(n))
		if _, err := os.Stat(ruta); err != nil {
			t.Fatalf("la etapa no dejó %s: %v", ruta, err)
		}
	}
	// la última clase tiene que estar en el último dex, y cada fichero se relee sin pasar del techo
	ultimo := dexes[len(dexes)-1]
	if _, err := readDex(ultimo); err != nil {
		t.Fatalf("el último dex no se relee: %v", err)
	}
	vistas := map[string]bool{}
	for _, b := range dexes {
		d, err := readDex(b)
		if err != nil {
			t.Fatalf("un dex del reparto no se relee: %v", err)
		}
		for _, c := range d.classes {
			vistas[c.name] = true
		}
	}
	if !vistas["Lgrande/C69;"] {
		t.Fatal("la última clase no quedó en ningún dex")
	}
	// cada grupo lleva su caché: el primero `build/dexcache.bin`, los demás con su índice
	primera := filepath.Join("build", "dexcache.bin")
	if _, err := os.Stat(primera); err != nil {
		t.Fatalf("el primer grupo no dejó la caché de siempre: %v", err)
	}
	if _, err := os.Stat(ficheroCacheAppGoDeGrupo(1)); err != nil {
		t.Fatalf("el segundo grupo no dejó su caché: %v", err)
	}

	// una app pequeña: el dex del reparto y el de un solo fichero son el mismo byte a byte
	pequena := clasesSinteticas(4, 5, 2, 1)
	esperado, err := dexDeClasesConCache(pequena, nil)
	if err != nil {
		t.Fatalf("dexando la app pequeña sin reparto: %v", err)
	}
	if _, err := dexarAppEnGo(pequena, 34); err != nil {
		t.Fatalf("dexando la app pequeña repartida: %v", err)
	}
	obtenido, err := os.ReadFile(filepath.Join("build", "classes.dex"))
	if err != nil {
		t.Fatalf("no hay build/classes.dex de la app pequeña: %v", err)
	}
	if !bytes.Equal(esperado, obtenido) {
		t.Fatalf("la app pequeña cambió: %d bytes sin reparto y %d con él", len(esperado), len(obtenido))
	}
	for n := 2; ; n++ {
		if !exists(filepath.Join("build", dexEntryName(n))) {
			break
		}
		t.Fatalf("la app pequeña dejó %s: sobra un dex del reparto anterior", dexEntryName(n))
	}
}

// fuenteDeMetodos escribe la clase con `cuantos` métodos estáticos, cada uno con su literal: es
// lo que el emisor de Java compila sin salirse del subconjunto medido.
func fuenteDeMetodos(clase string, cuantos int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package grande;\n\npublic class %s {\n", clase)
	for i := 0; i < cuantos; i++ {
		fmt.Fprintf(&b, "    static int m%d() { return %d; }\n", i, i)
	}
	b.WriteString("}\n")
	return b.String()
}

// fuenteLlamadora llama al último método de la última clase generada: el `method_id` de ese
// destino es el que no cabe en 16 bits cuando las 70 clases viajan en un solo dex.
func fuenteLlamadora(clases, metodos int) string {
	return fmt.Sprintf("package grande;\n\npublic class Uso {\n    static int usa() { return C%d.m%d(); }\n}\n",
		clases-1, metodos-1)
}

// herramientaDelSDK busca un binario de las build-tools instaladas (apksigner, zipalign,
// dexdump). Devuelve "" si el SDK no está o no trae ese binario, y la prueba decide qué hacer:
// el oráculo es local y no se inventa.
func herramientaDelSDK(t *testing.T, nombre string) string {
	t.Helper()
	base := os.Getenv("ANDROID_HOME")
	if base == "" {
		return ""
	}
	version, err := latestBuildTools(filepath.Join(base, "build-tools"))
	if err != nil {
		return ""
	}
	ruta := filepath.Join(base, "build-tools", version, nombre)
	if !exists(ruta) {
		return ""
	}
	return ruta
}

// TestCompilaAppConVariosDexEnElApk es la punta: una app generada con 70 000 métodos se compila
// con el compilador y el linker de Go, su dex se reparte, y el paquete lleva los dos ficheros,
// firmados v1/v2/v3, alineados y legibles por el `dexdump` del SDK.
func TestCompilaAppConVariosDexEnElApk(t *testing.T) {
	const clases, metodos = 70, 1000

	apksigner := herramientaDelSDK(t, "apksigner")
	zipalign := herramientaDelSDK(t, "zipalign")
	dexdump := herramientaDelSDK(t, "dexdump")
	if apksigner == "" || zipalign == "" || dexdump == "" {
		t.Skip("sin apksigner, zipalign y dexdump en las build-tools: los oráculos del SDK no se pueden inventar")
	}

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
    <string name="app_name">Grande</string>
</resources>`)
	for i := 0; i < clases; i++ {
		pon(t, raiz, fmt.Sprintf("src/grande/C%d.java", i), fuenteDeMetodos(fmt.Sprintf("C%d", i), metodos))
	}
	pon(t, raiz, "src/grande/Uso.java", fuenteLlamadora(clases, metodos))

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
		v4:          true,
	}
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(opciones)

	// el reparto se ve en build/ y el paquete lo numera seguido, antes de las dependencias
	appDexes := []string{}
	for n := 1; ; n++ {
		ruta := filepath.Join("build", dexEntryName(n))
		if !exists(ruta) {
			break
		}
		appDexes = append(appDexes, ruta)
	}
	if len(appDexes) < 2 {
		t.Fatalf("%d dex de la app en build/: no se repartió", len(appDexes))
	}
	t.Logf("la app dejó %d dex: %v", len(appDexes), appDexes)

	abierto, err := zip.OpenReader(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("el apk no abre como zip: %v", err)
	}
	enApk := map[string][]byte{}
	for _, f := range abierto.File {
		if !strings.HasSuffix(f.Name, ".dex") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("abriendo %s del apk: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("leyendo %s del apk: %v", f.Name, err)
		}
		enApk[f.Name] = b
	}
	abierto.Close()
	ultimoNombre := dexEntryName(len(appDexes))
	if _, ok := enApk["classes.dex"]; !ok {
		t.Fatalf("el apk no lleva classes.dex: %v", nombresDeMapa(enApk))
	}
	if _, ok := enApk[ultimoNombre]; !ok {
		t.Fatalf("el apk no lleva %s: %v", ultimoNombre, nombresDeMapa(enApk))
	}
	if !bytes.Equal(enApk[ultimoNombre], leerArchivo(t, appDexes[len(appDexes)-1])) {
		t.Fatalf("%s del apk no son los bytes de %s", ultimoNombre, appDexes[len(appDexes)-1])
	}

	// el oráculo del dex: la última clase y su último método están en el último fichero
	suelto := filepath.Join(t.TempDir(), ultimoNombre)
	if err := os.WriteFile(suelto, enApk[ultimoNombre], 0644); err != nil {
		t.Fatal(err)
	}
	salida, err := exec.Command(dexdump, suelto).CombinedOutput()
	if err != nil {
		t.Fatalf("dexdump falló sobre %s: %v\n%s", ultimoNombre, err, salida)
	}
	if !bytes.Contains(salida, []byte("Lgrande/C69;")) || !bytes.Contains(salida, []byte("m999")) {
		t.Fatalf("dexdump no ve Lgrande/C69;.m999 en %s", ultimoNombre)
	}

	// la firma del propio binario (v1+v2+v3) y los dos oráculos del SDK
	v, err := verificarApk(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("verificarApk: %v", err)
	}
	if !v.v1 || !v.v2 || !v.v3 {
		t.Fatalf("el apk no verifica v1/v2/v3: v1=%v v2=%v v3=%v", v.v1, v.v2, v.v3)
	}
	salida, err = exec.Command(apksigner, "verify", "--verbose", filepath.Join("build", "app.apk")).CombinedOutput()
	if err != nil {
		t.Fatalf("apksigner verify falló: %v\n%s", err, salida)
	}
	for _, esquema := range []string{
		"Verified using v1 scheme (JAR signing): true",
		"Verified using v2 scheme (APK Signature Scheme v2): true",
		"Verified using v3 scheme (APK Signature Scheme v3): true",
	} {
		if !bytes.Contains(salida, []byte(esquema)) {
			t.Fatalf("apksigner no confirma %q:\n%s", esquema, salida)
		}
	}
	salida, err = exec.Command(zipalign, "-c", "-p", "4", filepath.Join("build", "app.apk")).CombinedOutput()
	if err != nil {
		t.Fatalf("zipalign no da el apk por alineado: %v\n%s", err, salida)
	}
}

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

func leerArchivo(t *testing.T, ruta string) []byte {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leyendo %s: %v", ruta, err)
	}
	return b
}

func nombresDeMapa(m map[string][]byte) []string {
	nombres := make([]string, 0, len(m))
	for nombre := range m {
		nombres = append(nombres, nombre)
	}
	sort.Strings(nombres)
	return nombres
}
