package main

// El compilador Go contra las dependencias de `jar/`: que una clase del fuente que nombra un tipo de
// un jar se resuelva leyendo el jar (y no se invente el descriptor), que un `.aar` aporte su
// `classes.jar`, que una cadena de padres cruce dos jars hasta el `android.jar`, que la huella del
// classpath cubra cada dependencia y que cambiar una de ellas abra el atajo de I3. El oráculo de
// paridad es javac 17, y esa mitad vive en `dependencias_paridad_test.go` con `//go:build !js`
// porque bajo `GOOS=js` no hay proceso que lanzar. Estas pruebas **no se saltan**: sin SDK —o sin
// javac 17 en la mitad nativa— fallan, porque un SKIP deja la suite en verde sin haber medido nada
// (el crítico de 5bc1e8c vio cuatro SKIP y «ok … 0.034s»).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jarDelSDKExigido es jarDelSDK sin el SKIP: si no hay android.jar que abrir, la prueba falla y dice
// cómo darlo. Se usa en todas las pruebas de este fichero.
func jarDelSDKExigido(t *testing.T) string {
	t.Helper()
	candidatas := rutasDelAndroidJar()
	for _, r := range candidatas {
		b, err := abrirBuscador(r)
		if err != nil {
			continue
		}
		b.cerrar()
		return r
	}
	t.Fatalf("sin android.jar que abrir: exporta APKC_ANDROID_JAR (probadas: %s); estas pruebas miden la resolución de nombres contra el SDK y no se pueden saltar",
		strings.Join(candidatas, ", "))
	return ""
}

// javac17Exigido es javac17 sin el SKIP: la paridad byte a byte con javac 17 es la medida, no un
// extra, así que un entorno sin javac 17 tiene que verse. Se queda en este fichero, sin tag, porque
// también la usan las pruebas de `javaemisor_*` (también sin tag) que compilan contra javac; la
// prueba de paridad que la consume vive en `dependencias_paridad_test.go` con `//go:build !js`.
func javac17Exigido(t *testing.T) string {
	t.Helper()
	if ruta := buscarJavac17(); ruta != "" {
		return ruta
	}
	t.Fatalf("sin javac 17: exporta JAVAC17 con la ruta de su javac; la paridad con javac es la medida de esta prueba y no se puede saltar")
	return ""
}

const (
	// fuenteDepSuma es la dependencia mínima: una clase pública con un método estático.
	fuenteDepSuma = `package dep;
public class Suma {
    public static int suma(int a, int b) { return a + b; }
}
`
	// fuenteUsoSuma nombra la dependencia por un import y la llama: sin el jar, el emisor no puede
	// resolver ni el tipo ni el descriptor del método.
	fuenteUsoSuma = `package com.hola;
import dep.Suma;
public class Uso {
    public int llama() { return Suma.suma(2, 3); }
}
`
	// La cadena de padres que cruza dos jars: `Base` (b.jar) extiende Activity del android.jar y
	// `A` (a.jar) extiende `Base`. Resolver `A` obliga a seguir su padre en b.jar y el de `Base` en
	// el SDK, que es el orden real de un classpath con dependencias.
	fuenteDepBase = `package depb;
import android.app.Activity;
public class Base extends Activity {
    public int base() { return 7; }
}
`
	fuenteDepA = `package depa;
import depb.Base;
public class A extends Base {
    public int suma() { return base() + 1; }
}
`
	fuenteUsoCadena = `package com.hola;
import depa.A;
public class Uso {
    public int llama() { return new A().suma(); }
}
`
)

// mapaDeClases convierte lo emitido en lo que jarDePrueba monta: entrada del jar -> bytes.
func mapaDeClases(clases []claseEmitida) map[string][]byte {
	salida := map[string][]byte{}
	for _, c := range clases {
		salida[c.interno+".class"] = c.datos
	}
	return salida
}

// clasesDeFuente emite el fuente con el propio emisor y devuelve un mapa entrada-de-jar -> bytes,
// que es lo que jarDePrueba necesita para montar la dependencia.
func clasesDeFuente(t *testing.T, jarSDK, nombre, fuente string) map[string][]byte {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: nombre, contenido: fuente}}, jarSDK)
	if err != nil {
		t.Fatalf("el emisor no compiló la dependencia de la prueba: %v", err)
	}
	return mapaDeClases(salida)
}

// jarDeDependencia monta `jar/dep.jar` con la clase de la dependencia y deja el proyecto con el
// fuente que la usa. Devuelve la ruta del jar.
func jarDeDependencia(t *testing.T, raiz string, clases map[string][]byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(raiz, "jar", "dep.jar")
	jarDePrueba(t, ruta, clases)
	return ruta
}

// TestCompilarEnGoResuelveUnaClaseDeJar: con el jar en `jar/`, el emisor resuelve `dep.Suma` y el
// descriptor `(II)I`; sin el jar, el mismo fuente se rechaza. Las dos mitades importan: la primera
// dice que se leyó el jar, la segunda que no se acepta por casualidad.
func TestCompilarEnGoResuelveUnaClaseDeJar(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	raiz := t.TempDir()
	jarDeDependencia(t, raiz, clasesDeFuente(t, jarSDK, "src/dep/Suma.java", fuenteDepSuma))
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSuma)
	t.Chdir(raiz)

	fuentes, err := fuentesJava([]string{"src"})
	if err != nil {
		t.Fatal(err)
	}
	entradas, err := entradasDelClasspath(jarSDK)
	if err != nil {
		t.Fatalf("armar el classpath: %v", err)
	}
	if len(entradas) != 2 {
		t.Fatalf("%d entradas de classpath, medidas dos (android.jar y jar/dep.jar)", len(entradas))
	}
	clases, err := compilarProyectoJavaConClasspath(fuentes, rutasDelClasspath(entradas))
	if err != nil {
		t.Fatalf("el emisor no resolvió la dependencia: %v", err)
	}
	var uso *claseEmitida
	for i := range clases {
		if clases[i].interno == "com/hola/Uso" {
			uso = &clases[i]
		}
	}
	if uso == nil {
		t.Fatalf("no se emitió com/hola/Uso: %v", clases)
	}
	// el Methodref que javac escribe: el tipo dueño, el nombre y el descriptor del método
	for _, quiero := range []string{"dep/Suma", "suma", "(II)I"} {
		if !strings.Contains(string(uso.datos), quiero) {
			t.Errorf("la clase emitida no nombra %q: la dependencia no se resolvió", quiero)
		}
	}
	cf, err := readClassFull(uso.datos)
	if err != nil {
		t.Fatalf("la clase emitida no se relee: %v", err)
	}
	if _, err := dexDeClases([]classFull{cf}); err != nil {
		t.Fatalf("la clase que resolvió una dependencia no se dexa: %v", err)
	}

	// el control negativo: sin el jar, el mismo fuente no se puede resolver
	if _, err := compilarProyectoJavaConClasspath(fuentes, []string{jarSDK}); err == nil {
		t.Fatal("sin el jar el emisor aceptó un tipo que no puede resolver")
	} else if !strings.Contains(err.Error(), "Suma") || !strings.Contains(err.Error(), "classpath") {
		t.Errorf("el rechazo no nombra la clase que falta ni el classpath: %v", err)
	}
}

// dexarUnaClase saca del vector la clase pedida, la relee y la dexa.
func dexarUnaClase(t *testing.T, clases []claseEmitida, interno string) ([]byte, error) {
	t.Helper()
	for _, c := range clases {
		if c.interno != interno {
			continue
		}
		cf, err := readClassFull(c.datos)
		if err != nil {
			return nil, err
		}
		return dexDeClases([]classFull{cf})
	}
	t.Fatalf("%s no está en lo emitido", interno)
	return nil, nil
}

// TestCompilarEnGoLeeElClassesJarDeUnAar: un `.aar` no es un jar de clases; lo que el emisor abre es
// su `classes.jar`, extraído a build/aar/. Se comprueba que la extracción es la que se abre y que el
// fuente resuelve la clase de dentro.
func TestCompilarEnGoLeeElClassesJarDeUnAar(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	raiz := t.TempDir()
	clases := clasesDeFuente(t, jarSDK, "src/dep/Suma.java", fuenteDepSuma)
	jarDePrueba(t, filepath.Join(raiz, "classes.jar"), clases)
	datos, err := os.ReadFile(filepath.Join(raiz, "classes.jar"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "dep.aar"), map[string][]byte{
		"classes.jar":            datos,
		"AndroidManifest.xml":    []byte("<manifest/>"),
		"res/values/strings.xml": []byte("<resources/>"),
	})
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSuma)
	t.Chdir(raiz)

	entradas, err := entradasDelClasspath(jarSDK)
	if err != nil {
		t.Fatalf("armar el classpath con un AAR: %v", err)
	}
	if len(entradas) != 2 {
		t.Fatalf("%d entradas, medidas dos (android.jar y el classes.jar del AAR)", len(entradas))
	}
	extraido := entradas[1].abrir
	if !strings.HasPrefix(filepath.ToSlash(extraido), "build/aar/") {
		t.Fatalf("el AAR no se abrió por su classes.jar extraído: %s", extraido)
	}
	if _, err := os.Stat(extraido); err != nil {
		t.Fatalf("el classes.jar extraído no está en disco: %v", err)
	}
	fuentes, err := fuentesJava([]string{"src"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compilarProyectoJavaConClasspath(fuentes, rutasDelClasspath(entradas)); err != nil {
		t.Fatalf("el emisor no resolvió la clase del AAR: %v", err)
	}

	// el mismo AAR no vuelve a extraerse: el nombre sale de su huella
	otraVez, err := entradasDelClasspath(jarSDK)
	if err != nil {
		t.Fatal(err)
	}
	if otraVez[1].abrir != extraido {
		t.Fatalf("el mismo AAR se extrajo en otro sitio: %s frente a %s", otraVez[1].abrir, extraido)
	}
}

// TestCompilarEnGoResuelveUnaCadenaDePadresEntreJars: la clase que usa el fuente (A, en a.jar)
// extiende otra que solo está en b.jar, y esa extiende Activity del android.jar. El emisor tiene que
// seguir los dos saltos —si solo leyera el primer jar, el padre quedaría sin resolver—, dexar el
// resultado y, con el proyecto delante, dejar `check` en verde. El control negativo quita b.jar: el
// error tiene que nombrar `Base`, no `A`, que sí está.
func TestCompilarEnGoResuelveUnaCadenaDePadresEntreJars(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	raiz := t.TempDir()
	t.Chdir(raiz)
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}

	// b.jar: Base, que extiende Activity y se compila solo contra el SDK
	base, err := compilarProyectoJavaConClasspath(
		[]fuenteJava{{nombre: "src/depb/Base.java", contenido: fuenteDepBase}}, []string{jarSDK})
	if err != nil {
		t.Fatalf("el emisor no compiló la base de la cadena: %v", err)
	}
	jarB := filepath.Join(raiz, "jar", "b.jar")
	jarDePrueba(t, jarB, mapaDeClases(base))

	// a.jar: A extiende Base, que ya no está en el fuente sino en b.jar
	a, err := compilarProyectoJavaConClasspath(
		[]fuenteJava{{nombre: "src/depa/A.java", contenido: fuenteDepA}}, []string{jarSDK, jarB})
	if err != nil {
		t.Fatalf("el emisor no compiló A contra b.jar: %v", err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "a.jar"), mapaDeClases(a))
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoCadena)

	entradas, err := entradasDelClasspathEn(raiz, jarSDK)
	if err != nil {
		t.Fatalf("armar el classpath del proyecto: %v", err)
	}
	if len(entradas) != 3 {
		t.Fatalf("%d entradas de classpath, medidas tres (android.jar, a.jar y b.jar)", len(entradas))
	}
	fuentes, err := fuentesJava([]string{"src"})
	if err != nil {
		t.Fatal(err)
	}
	clases, err := compilarProyectoJavaConClasspath(fuentes, rutasDelClasspath(entradas))
	if err != nil {
		t.Fatalf("el emisor no siguió la cadena A -> Base -> Activity: %v", err)
	}
	if _, err := dexarUnaClase(t, clases, "com/hola/Uso"); err != nil {
		t.Fatalf("la clase que resolvió la cadena no se dexa: %v", err)
	}

	// el diagnóstico en vivo ve las mismas dependencias del proyecto
	r := revisarFuenteJava(fuenteUsoCadena, "Uso.java", jarSDK, raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("check da %d errores con la cadena resoluble: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("check no llegó al emisor: aviso %q", r.Aviso)
	}

	// el control negativo: sin b.jar, Base no está en ningún sitio y el error lo nombra
	if err := os.Remove(jarB); err != nil {
		t.Fatal(err)
	}
	r = revisarFuenteJava(fuenteUsoCadena, "Uso.java", jarSDK, raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores sin b.jar, medido uno: %+v", len(r.Errores), r.Errores)
	}
	if !strings.Contains(r.Errores[0].Mensaje, "Base") {
		t.Errorf("el error no nombra la clase que falta: %q", r.Errores[0].Mensaje)
	}
}

// TestCheckResuelveElClassesJarDeUnAar: el diagnóstico en vivo arma el mismo classpath que `build`,
// así que un tipo que solo vive en el `classes.jar` de un `.aar` resuelve. Antes se compilaba contra
// el android.jar solo y ese tipo se subrayaba como un error que la compilación nunca da.
func TestCheckResuelveElClassesJarDeUnAar(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	raiz := t.TempDir()
	t.Chdir(raiz)
	jarDePrueba(t, filepath.Join(raiz, "classes.jar"),
		clasesDeFuente(t, jarSDK, "src/dep/Suma.java", fuenteDepSuma))
	datos, err := os.ReadFile(filepath.Join(raiz, "classes.jar"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "dep.aar"), map[string][]byte{"classes.jar": datos})
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSuma)

	r := revisarFuenteJava(fuenteUsoSuma, "Uso.java", jarSDK, raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("check no vio la clase del AAR: %+v", r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("check no llegó al emisor: aviso %q", r.Aviso)
	}
}

// TestCheckResuelveElJarDelProyectoDesdeOtroCwd: `check` diagnostica un fichero que puede estar
// fuera del directorio de trabajo, así que las dependencias de `jar/` tienen que salir de la raíz
// del proyecto (`--proyecto`), no del cwd. Las demás pruebas de este fichero hacen `t.Chdir(raiz)` y
// pasan la misma `raiz` como proyecto, de modo que su verde no distingue una cosa de la otra; esta
// corre desde un directorio ajeno —que además no tiene `jar/`—, exige que la clase del jar resuelva
// y, al quitar el jar de la raíz, que el error nombre la clase.
func TestCheckResuelveElJarDelProyectoDesdeOtroCwd(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	raiz := t.TempDir()
	jar := jarDeDependencia(t, raiz, clasesDeFuente(t, jarSDK, "src/dep/Suma.java", fuenteDepSuma))
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSuma)

	// el cwd no es el proyecto y no tiene `jar/`: si `check` leyera el cwd, no vería la dependencia
	otro := t.TempDir()
	t.Chdir(otro)
	aqui, err := os.Getwd()
	if err != nil || aqui != otro {
		t.Fatalf("el cwd no es el directorio ajeno: %q (%v)", aqui, err)
	}
	if deAqui := jarsEn(""); len(deAqui) != 0 {
		t.Fatalf("el cwd tiene jars y la prueba no distinguiría la raíz del cwd: %v", deAqui)
	}
	if enLaRaiz := jarsEn(raiz); len(enLaRaiz) != 1 {
		t.Fatalf("la raíz del proyecto no tiene el jar: %v", enLaRaiz)
	}

	r := revisarFuenteJava(fuenteUsoSuma, "Uso.java", jarSDK, raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("check no resolvió dep.Suma con la raíz del proyecto y un cwd ajeno: %+v (aviso %q)", r.Errores, r.Aviso)
	}
	if !r.Emisor {
		t.Fatalf("check no llegó al emisor: aviso %q", r.Aviso)
	}

	// el control negativo: sin el jar de la raíz la clase no está en ningún sitio y el error la nombra
	if err := os.Remove(jar); err != nil {
		t.Fatal(err)
	}
	r = revisarFuenteJava(fuenteUsoSuma, "Uso.java", jarSDK, raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores sin el jar de la raíz, medido uno: %+v", len(r.Errores), r.Errores)
	}
	if !strings.Contains(r.Errores[0].Mensaje, "Suma") {
		t.Errorf("el error no nombra la clase del jar que falta: %q", r.Errores[0].Mensaje)
	}
}

// TestCompilarEnGoInvalidaElReusoSiCambiaUnaDependencia cierra I3 con dependencias: la vuelta sin
// cambios reutiliza, y cambiar un byte de `jar/` —sin tocar ni un fuente— abre la compilación otra
// vez, porque la huella del classpath entra en el estado.
func TestCompilarEnGoInvalidaElReusoSiCambiaUnaDependencia(t *testing.T) {
	jarSDK := jarDelSDKExigido(t)
	anterior := androidJar
	androidJar = jarSDK
	DataTable = Data{}
	t.Cleanup(func() {
		androidJar = anterior
		DataTable = Data{}
	})
	raiz := t.TempDir()
	clases := clasesDeFuente(t, jarSDK, "src/dep/Suma.java", fuenteDepSuma)
	jar := jarDeDependencia(t, raiz, clases)
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSuma)
	t.Chdir(raiz)

	if _, ran := compilarFuentesEnGo(true); !ran {
		t.Fatal("con el estado vacío la etapa tiene que compilar")
	}
	saveTable()
	if _, ran := compilarFuentesEnGo(true); ran {
		t.Fatal("sin fuentes cambiados ni classpath distinto la etapa no puede volver a compilar")
	}

	// el jar cambia en un byte que no es la clase que se resuelve: la huella cubre el jar entero,
	// así que el reuse de I3 tiene que abrirse igual
	clases["META-INF/MANIFEST.MF"] = []byte("Manifest-Version: 1.0\nDependencia: otra\n")
	jarDePrueba(t, jar, clases)
	if _, ran := compilarFuentesEnGo(true); !ran {
		t.Fatal("un jar de dependencias con otros bytes tiene que invalidar las clases reutilizadas")
	}
}

// TestHuellaDelClasspathCubreCadaDependencia: la huella no es la del android.jar con las otras
// rutas: cambiar cualquiera de los dos jars de `jar/` cambia la huella, que es lo que decide el
// reuse. No necesita el SDK: el android.jar de la prueba es un zip cualquiera.
func TestHuellaDelClasspathCubreCadaDependencia(t *testing.T) {
	raiz := t.TempDir()
	t.Chdir(raiz)
	jarDePrueba(t, filepath.Join(raiz, "android.jar"), map[string][]byte{"p/A.class": {1}})
	if err := os.MkdirAll("jar", 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "a.jar"), map[string][]byte{"p/B.class": {2}})
	jarDePrueba(t, filepath.Join(raiz, "jar", "b.jar"), map[string][]byte{"p/C.class": {3}})

	entradas, err := entradasDelClasspath(filepath.Join(raiz, "android.jar"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 3 {
		t.Fatalf("%d entradas, medidas tres (android.jar, a.jar y b.jar)", len(entradas))
	}
	antes := huellaDelClasspath(entradas)

	// cambiar el segundo jar, el que no es el primero del classpath, también cuenta
	jarDePrueba(t, filepath.Join(raiz, "jar", "b.jar"), map[string][]byte{"p/C.class": {9}})
	entradas, err = entradasDelClasspath(filepath.Join(raiz, "android.jar"))
	if err != nil {
		t.Fatal(err)
	}
	despues := huellaDelClasspath(entradas)
	if antes == despues {
		t.Fatal("cambiar un jar de jar/ no cambió la huella del classpath")
	}
	if !strings.Contains(despues, "jar/b.jar") || !strings.Contains(despues, "jar/a.jar") {
		t.Fatalf("la huella no nombra cada dependencia: %q", despues)
	}
}

// TestIndiceDelJarPorJar: I5 con más de un jar. Abrir el android.jar y después una dependencia no
// puede tirar el índice del primero: cada jar tiene su fichero en build/jarindice/.
func TestIndiceDelJarPorJar(t *testing.T) {
	raiz := t.TempDir()
	t.Chdir(raiz)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(raiz, "android.jar")
	b := filepath.Join(raiz, "dep.jar")
	jarDePrueba(t, a, map[string][]byte{"p/A.class": {1, 2, 3}})
	jarDePrueba(t, b, map[string][]byte{"p/B.class": {4, 5, 6}})

	for _, ruta := range []string{a, b} {
		busc, err := abrirBuscador(ruta)
		if err != nil {
			t.Fatal(err)
		}
		busc.cerrar()
	}
	antes := parseosDelDirectorio
	for _, ruta := range []string{a, b} {
		busc, err := abrirBuscador(ruta)
		if err != nil {
			t.Fatal(err)
		}
		busc.cerrar()
	}
	if parseosDelDirectorio != antes {
		t.Fatalf("con dos jars la vuelta siguiente volvió a parsear %d directorios centrales", parseosDelDirectorio-antes)
	}
	if ficheroIndiceDelJar(a) == ficheroIndiceDelJar(b) {
		t.Fatal("los dos jars comparten fichero de índice")
	}
}
