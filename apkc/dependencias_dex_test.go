package main

// El dex de las dependencias por el camino Go: `jar/` dexado con el dexer de este fork, en su
// propio `classesN.dex`, sin arrancar d8. Es la frontera que dejaba `--compiler go` a medias en la
// pestaña: el proyecto compilaba contra el jar y el build moría al dexarlo. El jar de la prueba
// lleva varias clases, como `android-support-v4.jar` del proyecto real, no una sola.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fuentesDepSaludo son las clases de la dependencia: una con el método que se invoca, otra que la
// usa y una tercera con estado, para que el jar no sea el caso trivial de una clase suelta.
var fuentesDepSaludo = []fuenteJava{
	{nombre: "dep/saludo/Saludo.java", contenido: `package saludo;
public class Saludo {
    public static String saludo() { return "hola"; }
}
`},
	{nombre: "dep/saludo/SaludoUtil.java", contenido: `package saludo;
public class SaludoUtil {
    public static String dosVeces() { return Saludo.saludo(); }
}
`},
	{nombre: "dep/saludo/Contador.java", contenido: `package saludo;
public class Contador {
    public static final int BASE = 2;
    public static int doble(int n) { return n * BASE; }
}
`},
}

// fuenteUsoSaludo nombra dos de las clases del jar: el emisor resuelve las dos contra `jar/`.
const fuenteUsoSaludo = `package com.hola;
import saludo.Saludo;
import saludo.Contador;
public class Uso {
    public String llama() { return Saludo.saludo(); }
    public int cuenta() { return Contador.doble(2); }
}
`

// jarDeVariasClases emite los fuentes de la dependencia con el propio emisor y los mete en el jar
// del proyecto, como haría apkc-deps con un `android-support-v4.jar` local.
func jarDeVariasClases(t *testing.T, jarSDK string) string {
	t.Helper()
	raiz := t.TempDir()
	salida, err := compilarProyectoJava(fuentesDepSaludo, jarSDK)
	if err != nil {
		t.Fatalf("el emisor no compiló la dependencia de la prueba: %v", err)
	}
	clases := map[string][]byte{}
	for _, c := range salida {
		clases[c.interno+".class"] = c.datos
	}
	if len(clases) < 3 {
		t.Fatalf("la dependencia de la prueba salió con %d clases, medidas tres", len(clases))
	}
	return jarDeDependencia(t, raiz, clases)
}

// TestBuildGoDexaLaDependenciaSinD8 corre las etapas de `apkc build --compiler go` sobre un
// proyecto con `jar/`: el dex de la app, el de la dependencia y ninguna llamada a d8. El d8 de la
// prueba es un guion que deja un marcador y falla: si alguien lo arranca, la prueba lo dice.
func TestBuildGoDexaLaDependenciaSinD8(t *testing.T) {
	jarSDK := jarDelSDK(t)
	anteriorJar, anteriorD8 := androidJar, d8Path
	androidJar = jarSDK
	// el d8 que delata su ejecución: no es el del SDK, así que si el camino Go lo arranca, el
	// build falla y el marcador queda en disco
	sonda := t.TempDir()
	falso := filepath.Join(sonda, "d8")
	marcador := filepath.Join(sonda, "d8-invocado")
	guion := "#!/bin/sh\ntouch \"" + marcador + "\"\nexit 1\n"
	if err := os.WriteFile(falso, []byte(guion), 0o755); err != nil {
		t.Fatal(err)
	}
	d8Path = falso
	DataTable = Data{}
	t.Cleanup(func() {
		androidJar, d8Path = anteriorJar, anteriorD8
		DataTable = Data{}
	})

	jar := jarDeVariasClases(t, jarSDK)
	raiz := filepath.Dir(filepath.Dir(jar))
	pon(t, raiz, "src/com/hola/Uso.java", fuenteUsoSaludo)
	t.Chdir(raiz)

	if _, ran := compilarFuentesEnGo(false); !ran {
		t.Fatal("la etapa de compilación no corrió")
	}
	dexarClasesEnGo()

	jars, digests := dependenciasDexeables()
	if len(jars) != 1 {
		t.Fatalf("%d dependencias dexables, medida una (jar/dep.jar)", len(jars))
	}
	dexes, ran := dexDeps(21, false, true, jars, digests)
	if !ran {
		t.Fatal("la primera vuelta tenía que dexar el bucket de la dependencia")
	}
	if len(dexes) != 1 {
		t.Fatalf("%d dexes de dependencia, medido uno", len(dexes))
	}
	if exists(marcador) {
		t.Fatal("el camino Go arrancó d8 para dexar la dependencia")
	}

	// el dex del proyecto y el de la dependencia son dos ficheros distintos, y cada uno lleva lo
	// suyo: la app invoca la clase del jar, y la dependencia lleva el método que se invoca
	app, err := os.ReadFile(filepath.Join("build", "classes.dex"))
	if err != nil {
		t.Fatalf("no hay build/classes.dex: %v", err)
	}
	appDex, err := readDex(app)
	if err != nil {
		t.Fatalf("el dex de la app no se relee: %v", err)
	}
	if !tieneClase(appDex, "Lcom/hola/Uso;") {
		t.Errorf("el dex de la app no lleva Lcom/hola/Uso;")
	}
	if tieneClase(appDex, "Lsaludo/Saludo;") {
		t.Errorf("el dex de la app lleva la clase de la dependencia: no se separaron")
	}

	dep, err := os.ReadFile(dexes[0])
	if err != nil {
		t.Fatalf("el dex de la dependencia no está en disco: %v", err)
	}
	depDex, err := readDex(dep)
	if err != nil {
		t.Fatalf("el dex de la dependencia no se relee: %v", err)
	}
	if !tieneClase(depDex, "Lsaludo/Saludo;") {
		t.Errorf("el dex de la dependencia no lleva Lsaludo/Saludo;")
	}
	if !tieneMetodo(depDex, "Lsaludo/Saludo;", "saludo", "()Ljava/lang/String;") {
		t.Errorf("el dex de la dependencia no lleva Lsaludo/Saludo;.saludo")
	}
	// las tres clases del jar, no solo la que se invoca
	for _, quiero := range []string{"Lsaludo/Saludo;", "Lsaludo/SaludoUtil;", "Lsaludo/Contador;"} {
		if !tieneClase(depDex, quiero) {
			t.Errorf("el dex de la dependencia no lleva %s", quiero)
		}
	}

	// la vuelta siguiente la contesta la caché del bucket: ni d8 ni el dexer vuelven a trabajar
	otraVez, ranOtra := dexDeps(21, false, true, jars, digests)
	if ranOtra {
		t.Fatal("la segunda vuelta re-dexó un bucket que la caché ya tenía")
	}
	if len(otraVez) != 1 {
		t.Fatalf("%d dexes en la vuelta cacheada, medido uno", len(otraVez))
	}
	iguales, err := os.ReadFile(otraVez[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(iguales) != string(dep) {
		t.Fatal("la vuelta cacheada devolvió otros bytes que la que dexó")
	}
}

// TestClasesDeJarNombraLaEntradaQueNoDexa: cuando el dexer rechaza una clase de la dependencia, el
// error tiene que decir el jar y la entrada, no un «falló el bucket» a secas: es lo que convierte
// una frontera en un diagnóstico.
func TestClasesDeJarNombraLaEntradaQueNoDexa(t *testing.T) {
	raiz := t.TempDir()
	ruta := filepath.Join(raiz, "roto.jar")
	jarDePrueba(t, ruta, map[string][]byte{
		"dep/Rota.class": {0xca, 0xfe, 0xba, 0xbe, 0, 0},
	})
	_, err := clasesDeJar(ruta)
	if err == nil {
		t.Fatal("un .class que no se puede leer pasó como si nada")
	}
	for _, quiero := range []string{"roto.jar", "dep/Rota.class"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("el rechazo no nombra %q: %v", quiero, err)
		}
	}
}

// TestDependenciasDexeablesIncluyeElClassesJarDeUnAar: de un AAR no se dexa el AAR, sino su
// `classes.jar` extraído, y la huella que decide la caché es la del AAR entero.
func TestDependenciasDexeablesIncluyeElClassesJarDeUnAar(t *testing.T) {
	jarSDK := jarDelSDK(t)
	raiz := t.TempDir()
	t.Chdir(raiz)
	if err := os.MkdirAll("jar", 0o755); err != nil {
		t.Fatal(err)
	}
	clases := clasesDeFuente(t, jarSDK, "src/dep/Suma.java", fuenteDepSuma)
	jarDePrueba(t, filepath.Join(raiz, "classes.jar"), clases)
	datos, err := os.ReadFile(filepath.Join(raiz, "classes.jar"))
	if err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "dep.aar"), map[string][]byte{"classes.jar": datos})

	rutas, digests := dependenciasDexeables()
	if len(rutas) != 1 || len(digests) != 1 {
		t.Fatalf("%d rutas y %d huellas, medidas una de cada", len(rutas), len(digests))
	}
	if !strings.HasPrefix(filepath.ToSlash(rutas[0]), "build/aar/") {
		t.Fatalf("no se dexaría el classes.jar extraído del AAR: %s", rutas[0])
	}
	// la huella es la del AAR: cambiar el AAR tiene que invalidar el bucket
	antes := digests[0]
	jarDePrueba(t, filepath.Join(raiz, "jar", "dep.aar"), map[string][]byte{
		"classes.jar":            datos,
		"res/values/strings.xml": []byte("<resources/>"),
	})
	_, digests = dependenciasDexeables()
	if digests[0] == antes {
		t.Fatal("la huella del AAR no cambió al cambiar el AAR")
	}
}

func tieneClase(d *dexFile, descriptor string) bool {
	for _, c := range d.classes {
		if c.name == descriptor {
			return true
		}
	}
	return false
}
