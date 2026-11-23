package main

// El dex primario por debajo de API 21: el contrato de bytes de la vía de multidex.
//
// Ese runtime no abre `classes2.dex` por su cuenta; el que lo abre es el cargador del soporte de
// multidex, y para eso las clases del proyecto y el propio cargador tienen que viajar en
// `classes.dex`. El reparto (reparto.go) recibe por eso una lista de clases fijas: esto mide
//
//   - la falsificación: sin la lista, una clase pequeña que llega al final cae en el segundo dex
//     (o más allá), y con la lista queda en el primero;
//   - la punta: `apkc build --linker go --compiler go` de un proyecto con 70 000 métodos en `jar/`
//     y `minSdkVersion` 19 sale en verde, deja las clases fijas en `classes.dex` y el resto en
//     `classes2.dex`, y los oráculos del SDK (`dexdump`, `apksigner`, `zipalign`) lo dan por bueno.
//
// Lo que no se mide aquí, porque no está en el árbol: el AAR de `androidx.multidex` y el gancho de
// `Application.attachBaseContext` (pasos siguientes) y el arranque real en un Dalvik de API ≤ 20,
// que no hay dónde ejecutar.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepartoArrastraLaLambdaDeSuClaseFija: después de expandir las lambdas, la clase sintética de
// una clase del proyecto vive al final de la lista; para que la app arranque tiene que viajar con
// su dueña en el dex primario. Se comprueba el arrastre y se falsifica apagándolo: la sintética se
// va a classes2.dex mientras la dueña sigue en classes.dex.
func TestRepartoArrastraLaLambdaDeSuClaseFija(t *testing.T) {
	clases := clasesSinteticas(70, 1000, 0, 0)
	dueña := classFull{Name: "Lcom/hola/Arranque;", Super: "Ljava/lang/Object;"}
	lambda := classFull{Name: "Lcom/hola/Arranque$$Lambda$1;", Super: "Ljava/lang/Object;"}
	clases = append(clases, dueña, lambda)

	fijas := map[string]bool{dueña.Name: true}

	// con el arrastre encendido, la sintética encabeza el primer dex junto a su dueña
	conArrastre, err := reparteEnDexesConFijas(clases, fijas, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo con arrastre: %v", err)
	}
	if !contieneClase(conArrastre[0], lambda.Name) {
		t.Fatalf("la lambda de una clase fija no cayó en el primer dex: %v", nombresDeGrupo(conArrastre[0])[:4])
	}
	if !contieneClase(conArrastre[0], dueña.Name) {
		t.Fatal("la clase dueña no cayó en el primer dex")
	}

	// la falsificación: sin arrastre, la misma lista deja la sintética en el último dex
	antes := arrastraLambdasDeFijas
	arrastraLambdasDeFijas = false
	t.Cleanup(func() { arrastraLambdasDeFijas = antes })
	sinArrastre, err := reparteEnDexesConFijas(clases, fijas, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo sin arrastre: %v", err)
	}
	if len(sinArrastre) != 2 {
		t.Fatalf("%d dex para 70 000 métodos: la falsificación esperaba classes.dex y classes2.dex", len(sinArrastre))
	}
	if contieneClase(sinArrastre[0], lambda.Name) {
		t.Fatal("sin arrastre la lambda siguió en classes.dex: la falsificación no falsifica")
	}
	if !contieneClase(sinArrastre[1], lambda.Name) {
		t.Fatal("sin arrastre la lambda no cayó en classes2.dex: el reparto no la movió")
	}
	if !contieneClase(sinArrastre[0], dueña.Name) {
		t.Fatal("sin arrastre la clase dueña dejó de estar en el primer dex")
	}
}

// TestRepartoFijaClasesEnElDexPrimario es la falsificación en el reparto: con el mismo conjunto y
// el mismo techo, la clase pequeña que llega al final se queda en el último dex cuando no hay
// lista de fijas, y encabeza `classes.dex` cuando la hay.
func TestRepartoFijaClasesEnElDexPrimario(t *testing.T) {
	// 70 clases de 1000 métodos no caben en un dex; detrás van la clase de arranque y el cargador
	// de multidex, que es el orden en que llegan cuando el proyecto compila y `jar/` aporta el resto
	clases := clasesSinteticas(70, 1000, 0, 0)
	arranque := classFull{Name: "Lcom/hola/Arranque;", Super: "Ljava/lang/Object;"}
	cargador := classFull{Name: "Landroidx/multidex/MultiDex;", Super: "Ljava/lang/Object;"}
	clases = append(clases, arranque, cargador)

	fijas := map[string]bool{arranque.Name: true, cargador.Name: true}

	// sin fijas: la clase de arranque no está en el primer dex, que es justo el contrato que se
	// rompe y lo que la falsificación tiene que enseñar
	sinFijas, err := reparteEnDexes(clases, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo sin fijas: %v", err)
	}
	if len(sinFijas) < 2 {
		t.Fatalf("%d grupos para 70 000 métodos: la falsificación no reparte", len(sinFijas))
	}
	if contieneClase(sinFijas[0], arranque.Name) {
		t.Fatal("sin fijas la clase de arranque ya cae en el primer dex: la falsificación no falsifica")
	}
	if !contieneClase(sinFijas[len(sinFijas)-1], arranque.Name) {
		t.Fatal("sin fijas la clase de arranque no está en el último dex: el reparto no la movió")
	}

	// con fijas: las dos encabezan el primer grupo, en su orden, y el resto conserva el suyo
	conFijas, err := reparteEnDexesConFijas(clases, fijas, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo con fijas: %v", err)
	}
	if len(conFijas) < 2 {
		t.Fatalf("%d grupos con fijas: el reparto dejó de partir", len(conFijas))
	}
	if got := nombresDeGrupo(conFijas[0]); len(got) < 2 || got[0] != arranque.Name || got[1] != cargador.Name {
		t.Fatalf("el primer dex no encabeza con las fijas: %v", got[:min(4, len(got))])
	}
	for _, g := range conFijas {
		if c := cuentaDe(g.tablas); c.mayor() > refsPorDex {
			t.Errorf("un grupo se pasa del techo: %s", c)
		}
	}
	// cobertura: todas las clases siguen en algún dex, y ninguna dos veces
	vistas := map[string]int{}
	for _, g := range conFijas {
		for _, cf := range g.clases {
			vistas[cf.Name]++
		}
	}
	if len(vistas) != len(clases) {
		t.Fatalf("%d clases repartidas, %d de entrada", len(vistas), len(clases))
	}
	for nombre, veces := range vistas {
		if veces != 1 {
			t.Fatalf("%s aparece %d veces en el reparto", nombre, veces)
		}
	}
}

// TestRepartoFijasQueNoCabenEsUnRechazo: si las fijas no caben en un dex, el reparto no las deja
// caer en `classes2.dex` en silencio: el error nombra el dex primario (main-dex), el techo y el
// fichero que ese runtime no abriría solo.
func TestRepartoFijasQueNoCabenEsUnRechazo(t *testing.T) {
	clases := clasesSinteticas(70, 1000, 0, 0)
	fijas := map[string]bool{}
	for _, cf := range clases {
		fijas[cf.Name] = true
	}
	_, err := reparteEnDexesConFijas(clases, fijas, refsPorDex)
	if err == nil {
		t.Fatal("70 clases fijas de 1000 métodos cupieron en un dex: el rechazo no existe")
	}
	for _, quiero := range []string{"main-dex", "65536", "classes2.dex"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("el rechazo no nombra %q: %v", quiero, err)
		}
	}
	t.Logf("rechazo del dex primario: %v", err)
}

func contieneClase(g grupoDex, nombre string) bool {
	for _, cf := range g.clases {
		if cf.Name == nombre {
			return true
		}
	}
	return false
}

// claseFixtureGrande escribe una clase con `metodos` métodos estáticos `iconst_0; ireturn`: son los
// `method_id` que miden el reparto y el código mínimo que `dexdump` acepta, sin pasar por javac.
func claseFixtureGrande(t *testing.T, interno string, metodos int) []byte {
	t.Helper()
	f := classFixture{name: interno, super: "java/lang/Object", access: 0x0021}
	for i := 0; i < metodos; i++ {
		f.methods = append(f.methods, memberFixture{
			name:       "m" + itoa(i),
			descriptor: "()I",
			access:     0x0009,
			code:       []byte{0x03, 0xac},
		})
	}
	return f.bytes()
}

// TestCompilaAppPre21FijaElDexPrimario es la punta: `minSdkVersion` 19 con 70 000 métodos en `jar/`
// y un proyecto pequeño. El build sale en verde porque las clases fijas caben en el dex primario:
// `classes.dex` lleva las del proyecto y el cargador de multidex, `classes2.dex` el resto, y el
// paquete firmado y alineado lo confirman los oráculos del SDK. Después se quita la fijación y se
// enseña que la clase del cargador cae en el segundo dex.
func TestCompilaAppPre21FijaElDexPrimario(t *testing.T) {
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
	<uses-sdk android:minSdkVersion="19" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name"/>
</manifest>`)
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Pre21</string>
</resources>`)
	// el proyecto: lo mínimo que compila el emisor de Go y que cabe, con su R, en el dex primario
	pon(t, raiz, "src/grande/Arranque.java", `package grande;

public class Arranque {
    public static int arranca() { return 1; }
}
`)

	// las dependencias: 70 clases de 1000 métodos (70 000 method_id) y, detrás en el orden de jar/,
	// el cargador de multidex. El nombre del jar decide el orden, y el cargador tiene que quedar
	// detrás para que la falsificación se vea.
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
	// el build deja `escribirV4` y la sal como los encontró: son estado del paquete, y sin esto
	// las pruebas de la v4 que corren después se quedan sin `.idsig`
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(opciones)

	// el reparto en build/: dos ficheros como mínimo, con las fijas en el primero
	appDexes := []string{}
	for n := 1; ; n++ {
		ruta := filepath.Join("build", dexEntryName(n))
		if !exists(ruta) {
			break
		}
		appDexes = append(appDexes, ruta)
	}
	if len(appDexes) < 2 {
		t.Fatalf("%d dex en build/ para 70 000 métodos: el dex de la app no se repartió", len(appDexes))
	}
	t.Logf("la app dejó %d dex: %v", len(appDexes), appDexes)

	primero := clasesDeDex(t, appDexes[0])
	if !primero["Lgrande/Arranque;"] {
		t.Fatalf("classes.dex no lleva la clase del proyecto: %v", nombresDeConjunto(primero))
	}
	if !primero["Landroidx/multidex/MultiDex;"] {
		t.Fatalf("classes.dex no lleva el cargador de multidex: %v", nombresDeConjunto(primero))
	}
	resto := map[string]bool{}
	for _, ruta := range appDexes[1:] {
		for nombre := range clasesDeDex(t, ruta) {
			resto[nombre] = true
		}
	}
	if !resto["Lgrande/C7;"] {
		t.Fatalf("classes2.dex no lleva el resto de las dependencias: %v", nombresDeConjunto(resto))
	}
	if resto["Lgrande/Arranque;"] || resto["Landroidx/multidex/MultiDex;"] {
		t.Fatal("una clase fija quedó también fuera del dex primario")
	}

	// el oráculo del dex: las clases fijas se ven en classes.dex y el resto en classes2.dex
	for _, caso := range []struct {
		ruta   string
		quiero []string
	}{
		{appDexes[0], []string{"Lgrande/Arranque;", "Landroidx/multidex/MultiDex;"}},
		{appDexes[1], []string{"Lgrande/C7;"}},
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

	// el paquete: los dos dex dentro, la firma del binario y los oráculos del SDK
	apk := filepath.Join("build", "app.apk")
	for _, nombre := range []string{"classes.dex", dexEntryName(len(appDexes))} {
		if _, err := os.Stat(filepath.Join("build", nombre)); err != nil {
			t.Fatalf("no hay %s en build/: %v", nombre, err)
		}
	}
	v, err := verificarApk(apk)
	if err != nil {
		t.Fatalf("verificarApk: %v", err)
	}
	if !v.v2 || !v.v3 {
		t.Fatalf("el apk no verifica v2/v3: v2=%v v3=%v", v.v2, v.v3)
	}
	salida, err := exec.Command(apksigner, "verify", "--verbose", apk).CombinedOutput()
	if err != nil {
		t.Fatalf("apksigner verify falló: %v\n%s", err, salida)
	}
	for _, esquema := range []string{
		"Verified using v2 scheme (APK Signature Scheme v2): true",
		"Verified using v3 scheme (APK Signature Scheme v3): true",
	} {
		if !bytes.Contains(salida, []byte(esquema)) {
			t.Fatalf("apksigner no confirma %q:\n%s", esquema, salida)
		}
	}
	salida, err = exec.Command(zipalign, "-c", "-p", "4", apk).CombinedOutput()
	if err != nil {
		t.Fatalf("zipalign no da el apk por alineado: %v\n%s", err, salida)
	}

	// la falsificación de punta: sin lista de fijas, el cargador de multidex se va al segundo dex
	clases, err := leeClases([]string{filepath.Join("build", "classes")})
	if err != nil {
		t.Fatalf("releyendo las clases del proyecto: %v", err)
	}
	for _, jar := range []string{filepath.Join("jar", "a-grande.jar"), filepath.Join("jar", "z-multidex.jar")} {
		delJar, err := clasesDeJar(jar)
		if err != nil {
			t.Fatalf("leyendo %s: %v", jar, err)
		}
		clases = append(clases, delJar...)
	}
	sinFijas, err := dexarAppEnGoCon(clases, nil, 19)
	if err != nil {
		t.Fatalf("repartiendo sin fijas: %v", err)
	}
	fuera := map[string]bool{}
	for _, b := range sinFijas[1:] {
		d, err := readDex(b)
		if err != nil {
			t.Fatalf("releyendo un dex sin fijas: %v", err)
		}
		for _, c := range d.classes {
			fuera[c.name] = true
		}
	}
	if !fuera["Landroidx/multidex/MultiDex;"] {
		t.Fatal("sin fijas el cargador de multidex siguió en el primer dex: la falsificación no falsifica")
	}
	t.Logf("sin fijas, el cargador de multidex cae en el dex 2 de %d", len(sinFijas))
}

// TestCompilaAppPre21PorD8FijaElDexPrimario es la punta del camino javac+d8, el del compilador por
// defecto. Con `minSdkVersion` 19 y un `jar/` que desborda un dex, d8 solo parte si `bundleJava` le
// escribe la lista de main-dex; sin ella aborta con «Cannot fit requested classes in a single dex
// file». El build tiene que volver con rc 0, dejar `classes.dex` y `classes2.dex`, y `dexdump -f`
// tiene que ver en el primero la clase del proyecto y el cargador de multidex.
//
// Es el único test del fichero que arranca javac y d8 (los demás van por el compilador de Go), así
// que se salta si faltan las herramientas. Suelto:
//
//	ANDROID_HOME=… APKC_ANDROID_JAR=… JAVAC17=… go test -run TestCompilaAppPre21PorD8FijaElDexPrimario -v .
func TestCompilaAppPre21PorD8FijaElDexPrimario(t *testing.T) {
	d8 := herramientaDelSDK(t, "d8")
	dexdump := herramientaDelSDK(t, "dexdump")
	if d8 == "" || dexdump == "" {
		t.Skip("sin d8 y dexdump en las build-tools: el camino javac+d8 no se puede medir")
	}
	javac := buscarJavac17()
	if javac == "" {
		t.Skip("sin javac 17: el camino javac+d8 necesita compilar los fuentes del proyecto")
	}
	// el d8 de las build-tools arranca la JVM de JAVA_HOME; el JDK de javac es el mismo
	t.Setenv("JAVA_HOME", filepath.Dir(filepath.Dir(javac)))

	antesJar, antesJavac, antesD8 := androidJar, javacPath, d8Path
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() {
		androidJar, javacPath, d8Path = antesJar, antesJavac, antesD8
		escribirV4, v4SaltHex = antesV4, antesSal
	})
	androidJar, javacPath, d8Path = jarDelSDK(t), javac, d8

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
	<uses-sdk android:minSdkVersion="19" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name"/>
</manifest>`)
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Pre21</string>
</resources>`)
	pon(t, raiz, "src/grande/Arranque.java", `package grande;

public class Arranque {
    public static int arranca() { return 1; }
}
`)
	// las dependencias: 70 clases de 1000 métodos y, detrás en el orden de jar/, el cargador de
	// multidex, que tiene que quedar fijo en el dex primario aunque venga de un jar
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

	// el camino por defecto: javac compila y bundleJava dexa el proyecto con jar/ en el mismo merge
	_build(buildOpts{
		incremental: true,
		linker:      linkerGo,
		compiler:    "javac",
		signer:      signerGo,
		keyStore:    keystore,
		storePass:   "android",
		keyAlias:    "androiddebugkey",
		jar:         androidJar,
		sdkVersion:  34,
		codename:    "14",
	})

	// la bandera que hace partir a d8: las fijas del proyecto y el cargador quedaron escritas
	reglas := leerArchivo(t, filepath.Join("build", "maindex.pro"))
	for _, quiero := range []string{"-keep class grande.Arranque { *; }", "-keep class androidx.multidex.MultiDex { *; }"} {
		if !bytes.Contains(reglas, []byte(quiero)) {
			t.Fatalf("build/maindex.pro no lleva %q:\n%s", quiero, reglas)
		}
	}

	primero := clasesDeDex(t, filepath.Join("build", "classes.dex"))
	segundo := clasesDeDex(t, filepath.Join("build", "classes2.dex"))
	if !primero["Lgrande/Arranque;"] || !primero["Landroidx/multidex/MultiDex;"] {
		t.Fatalf("classes.dex no lleva las fijas: %v", nombresDeConjunto(primero))
	}
	if !segundo["Lgrande/C0;"] {
		t.Fatalf("classes2.dex no lleva el resto de las dependencias: %v", nombresDeConjunto(segundo))
	}
	if primero["Lgrande/C0;"] {
		t.Fatal("classes.dex lleva una clase que no estaba fijada: d8 no siguió la lista")
	}

	// el oráculo del dex sobre los dos ficheros del merge
	for _, caso := range []struct {
		ruta   string
		quiero []string
	}{
		{filepath.Join("build", "classes.dex"), []string{"Lgrande/Arranque;", "Landroidx/multidex/MultiDex;"}},
		{filepath.Join("build", "classes2.dex"), []string{"Lgrande/C0;"}},
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
	t.Logf("javac+d8 con API 19: %d clases fijas en classes.dex y %d en classes2.dex",
		len(primero), len(segundo))
}

// clasesDeDex relee un dex de build/ y devuelve el conjunto de nombres de sus clases.
func clasesDeDex(t *testing.T, ruta string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leyendo %s: %v", ruta, err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("%s no se relee: %v", ruta, err)
	}
	nombres := map[string]bool{}
	for _, c := range d.classes {
		nombres[c.name] = true
	}
	return nombres
}

func nombresDeConjunto(m map[string]bool) []string {
	nombres := make([]string, 0, len(m))
	for nombre := range m {
		nombres = append(nombres, nombre)
	}
	return nombres
}
