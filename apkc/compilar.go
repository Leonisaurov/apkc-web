package main

// `apkc compile`: el `.class` sin `javac`. Es la pieza que quedaba del objetivo de la página: leer los
// fuentes que el editor escribe y dejar los ficheros que `apkc dex` convierte en `classes.dex`, todo dentro
// del mismo proceso y sin levantar nada.
//
// La resolución de tipos se hace contra el `android.jar` que se pase por bandera y contra las
// dependencias de `jar/` (cada `.jar`, y el `classes.jar` de cada `.aar`), y solo se leen las
// entradas que el fuente nombra: sobre la plantilla del parrafo 40 son cuatro ficheros de unos pocos
// KB, no los 26 MB del jar.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// compilerGo es el valor de `--compiler` que convierte las dos etapas de codigo de `apkc build` en este
// mismo binario: javac y d8 dejan de ejecutarse, y con `--linker go` tampoco se ejecuta aapt2. Es el camino
// que puede correr dentro de la pestaña, porque alli no arranca ningun proceso.
const compilerGo = "go"

func compilar() {
	set := flag.NewFlagSet("compile", flag.ExitOnError)
	set.Usage = func() {
		fmt.Print(`usage: apkc compile --android-jar android.jar [-o build/classes] [carpeta-con-java ...]

Turns the .java files of a directory tree into .class files, in Go, with no javac and no subprocess
anywhere. The output goes where the dexer looks for it, one file per class, named after the internal name
(R and R$layout are separate files, exactly like javac leaves them).

Only the measured subset compiles: declarations, @Override (which leaves no trace in the class file),
calls on this, on a written receiver, on a type and on super, string, integer, boolean and null
literals, inlined "static final int" constants like R.layout.main, fields (private/protected and
static included, with the access$NNN synthetics javac gives an anonymous class), local variables,
integer arithmetic, ++/-- and +=/-=, an if or if-else over one of the comparisons javac inverts
(docs/evidencia.md §49 and §52), while, for and do-while with break and continue (§63, §69, §70), a
switch of integer literals (§64), try/catch with several catches and finally (§71 and §75), arrays
(new T[n], v[i], .length, §66), the long with its two slots and its two stack words (§74), the float
and the double (§77) with the widened comparisons javac lowers through a cmp and a branch (§81), string
concatenation through the StringBuilder chain and the constant folding that javac does with it
(§53), and anonymous classes with the this$0 field javac gives them. Anything else -
a char, a condition that is a bare boolean,
a return, break or continue leaving a try with finally,
a nested private class, an annotation other than @Override, a class the classpath does not name -
stops the command with its line, before writing a single file.

Type names resolve against the android.jar passed in and the .jar/.aar files of jar/ (the
classes.jar inside an .aar), reading only the entries the sources mention.
`)
	}
	jar := set.String("android-jar", androidJar, "the android.jar whose classes the sources link against")
	out := set.String("o", "build/classes", "where the .class files go")
	if err := set.Parse(os.Args[2:]); err != nil {
		LogF("compile", err.Error())
	}
	raices := set.Args()
	if len(raices) == 0 {
		raices = []string{"src"}
	}
	for _, r := range raices {
		if strings.HasPrefix(r, "-") {
			LogF("compile", r, "was read as a folder: the switches go before the folders,",
				"as in apkc compile -o build/classes src (and -- ends them)")
		}
	}
	fuentes, err := fuentesJava(raices)
	if err != nil {
		LogF("compile", err.Error())
	}
	if len(fuentes) == 0 {
		LogF("compile", "no .java file under", strings.Join(raices, ", "))
	}
	entradas, err := entradasDelClasspath(*jar)
	if err != nil {
		LogF("compile", err.Error())
	}
	clases, err := compilarProyectoJavaConClasspath(fuentes, rutasDelClasspath(entradas))
	if err != nil {
		LogF("compile", err.Error())
	}
	if err := escribirClases(*out, clases); err != nil {
		LogF("compile", err.Error())
	}
	LogI("compile", len(fuentes), "fuentes ->", len(clases), "clases en", *out)
}

// escribirClases deja una clase por fichero, con el nombre internal y el sufijo .class, donde `apkc dex`
// mira. Lo que se escribe es siempre el resultado de un compile que no rechazo nada: el rechazo llega antes,
// asi que nunca quedan mezcladas clases nuevas con las que dej un fuente anterior.
func escribirClases(out string, clases []claseEmitida) error {
	for _, c := range clases {
		ruta := filepath.Join(out, filepath.FromSlash(c.interno)+".class")
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(ruta, c.datos, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// fuentesJava lee los `.java` de los arboles pedidos, en el orden de las rutas: el emisor resuelve los
// nombres entre ficheros, así que el conjunto entra junto. Se evita `os.Stat` para saber si la raiz es un
// fichero o un arbol: en el fs simulado del hospedador el Stat de un fichero montado responde que no existe
// mientras abrirlo va bien (parrafo 43), y aqui interesa lo segundo.
func fuentesJava(raices []string) ([]fuenteJava, error) {
	var out []fuenteJava
	for _, raiz := range raices {
		if strings.HasSuffix(raiz, ".java") {
			d, err := os.ReadFile(raiz)
			if err != nil {
				return nil, err
			}
			out = append(out, fuenteJava{nombre: raiz, contenido: string(d)})
			continue
		}
		err := filepath.Walk(raiz, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if i.IsDir() || !strings.HasSuffix(p, ".java") {
				return nil
			}
			d, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out = append(out, fuenteJava{nombre: p, contenido: string(d)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// revisarCompilerGo nombra lo que `--compiler go` todavía no mide. Se aisló del cuerpo de `build` para
// poder probarlo: las negaciones de un portero que solo vive dentro de un LogF no se comprueban.
//
// Las dependencias de `jar/` ya no son un tope, ni en el escritorio ni en wasm: el emisor las indexa
// y resuelve sus clases igual que las del `android.jar` (dependencias.go) y el dexer de este fork
// dexa sus `.class` en su propio `classesN.dex` (dexDeps con enGo, build.go), sin arrancar d8. Una
// dependencia que el dexer no mide —hoy, un `invokedynamic` cuyo bootstrap no es de los medidos— no
// se calla aquí: el rechazo sale del dexado, con el jar, la clase y el sitio concretos.
func revisarCompilerGo(o buildOpts) error {
	if kt, _ := getFiles("src", ".kt", false); len(kt) > 0 {
		return fmt.Errorf("--compiler go no compila Kotlin: %d fuentes .kt en src", len(kt))
	}
	// El bundle ya no depende de d8: aab_go.go convierte el arsc del enlazador y el dex de
	// este compilador en resources.pb y en el XmlNode del manifiesto (§186).
	return nil
}

// compilarFuentesEnGo es la etapa de `apkc build --compiler go` que sustituye a javac: lee src/*.java,
// escribe build/classes/*.class y dice los fuentes que pasó, que es lo que la firma de la etapa espera.
// El emisor resuelve los nombres entre ficheros, así que mientras haya un fuente cambiado los compila
// todos. Lo que sí se puede saltar es la vuelta en la que no cambió ninguno: no hay nada que resolver,
// las clases de build/classes son exactamente las que el emisor escribiría otra vez, y esa es la
// única forma de que la segunda pasada de la pestaña no pague el proyecto entero (no hay caché en
// memoria que sobreviva: cada `apkc build` de la página arranca una instancia nueva de Go).
func compilarFuentesEnGo(incremental bool) ([]string, bool) {
	defer timeIt("compile go")()
	// el classpath se arma una vez por vuelta: su huella es lo que decide si las clases de
	// build/classes siguen valiendo, y lo que se registra cuando se compilan. Incluye el
	// android.jar y cada dependencia de jar/ (un AAR por su classes.jar, con la huella del AAR).
	entradas, err := entradasDelClasspath(androidJar)
	if err != nil {
		LogF("build", err.Error())
	}
	huellaClasspath := huellaDelClasspath(entradas)
	if incremental {
		if clasesAlDia(huellaClasspath) {
			LogI("build", "ningun fuente cambio: se reutilizan las clases de", filepath.Join("build", "classes"))
			return nil, false
		}
	}
	fuentes, err := fuentesJava([]string{"src"})
	if err != nil {
		LogF("build", err.Error())
	}
	if len(fuentes) == 0 {
		return nil, false
	}
	clases, err := compilarProyectoJavaConClasspath(fuentes, rutasDelClasspath(entradas))
	if err != nil {
		LogF("build", err.Error())
	}
	if err := escribirClases(filepath.Join("build", "classes"), clases); err != nil {
		LogF("build", err.Error())
	}
	nombres := []string{}
	for _, f := range fuentes {
		nombres = append(nombres, f.nombre)
	}
	if incremental {
		// los fuentes del camino Go no los registraba nadie: sin esto no hay contra qué
		// comparar en la vuelta siguiente y el atajo de arriba no se abre nunca
		updateRecords(nombres)
		DataTable.ClassRoster = classRosterSHA()
		DataTable.AndroidJar = huellaClasspath
	}
	LogI("build", len(nombres), "fuentes de Java en Go ->", len(clases), "clases")
	return nombres, true
}

// clasesAlDia responde si las clases de build/classes son exactamente las que el emisor escribiría
// ahora: cada fuente de src/ conserva los bytes que vio la última compilación, el directorio de clases
// es el mismo, nombre por nombre y byte por byte, que el que esa compilación dejó, y el classpath es el
// mismo (el android.jar y cada dependencia de jar/). Sin la segunda mitad un build detenido a medias
// (o un `build/classes` tocado a mano) se creería hecho. La huella del classpath la calcula el
// llamador porque también la registra al compilar.
func clasesAlDia(huellaJar string) bool {
	loadTable()
	fuentes, _ := getFiles("src", ".java", false)
	if len(fuentes) == 0 {
		return false
	}
	// el classpath es un dato de la compilación igual que los fuentes: otro jar resuelve los
	// nombres contra otras clases, así que las de build/classes dejan de estar al día aunque no
	// haya cambiado ni un byte del proyecto. La huella es el tamaño y el digest del directorio
	// central, no la fecha: una fecha que cambia sola no puede costar una recompilación entera.
	if huellaJar != DataTable.AndroidJar {
		return false
	}
	for _, fuente := range fuentes {
		if recordedDigest(fuente) == "" || hasChanged(fuente) {
			return false
		}
	}
	roster := classRosterSHA()
	return roster != "" && roster == DataTable.ClassRoster
}

// classRosterSHA resume build/classes en un solo digest: la ruta relativa y el SHA-1 de cada fichero,
// en el orden léxico con el que filepath.Walk los entrega. Vacío cuando no hay ninguna clase.
func classRosterSHA() string {
	nombres, _ := getFiles(filepath.Join("build", "classes"), ".class", false)
	lineas := make([]string, 0, len(nombres))
	for _, nombre := range nombres {
		digest, err := fileSHA1(nombre)
		if err != nil {
			return ""
		}
		lineas = append(lineas, nombre+"="+digest)
	}
	if len(lineas) == 0 {
		return ""
	}
	return sha1Text(strings.Join(lineas, "\n"))
}

// dexarClasesEnGo es la etapa que sustituye a d8 sobre las clases del proyecto: las lee de build/classes
// con el lector de la Fase 2 y escribe build/classes.dex, el sitio de donde lo toma el empaquetado.
// Desde I4 (dexreuso.go) la vuelta se lleva la caché de la anterior y reutiliza el code_item y el
// debug_info_item de las clases cuyo fichero `.class` no cambió, siempre que las tablas del dex
// sigan siendo las mismas; el fichero que sale es idéntico byte a byte al de un dexado completo.
func dexarClasesEnGo() {
	dexarClasesEnGoCon(nil)
}

// dexarClasesEnGoCon es lo mismo con las dependencias dentro del mismo fichero. Por debajo de API 21
// el runtime no abre un segundo dex, así que las clases de `jar/` viajan en el `classes.dex` del
// proyecto, igual que en el merge de d8; por encima llegan en su propio `classesN.dex` (dexDeps) y
// aquí no se pasa ninguna. Las clases de las dependencias entran después de las del proyecto, en el
// orden de los jars, para que el mismo proyecto dé el mismo dex.
func dexarClasesEnGoCon(jars []string) {
	defer timeIt("dex go")()
	clases, err := leeClases([]string{filepath.Join("build", "classes")})
	if err != nil {
		LogF("build", err.Error())
	}
	for _, jar := range jars {
		delJar, err := clasesDeJar(jar)
		if err != nil {
			LogF("build", err.Error())
		}
		clases = append(clases, delJar...)
	}
	if len(clases) == 0 {
		return
	}
	if _, err := dexarAppEnGo(clases); err != nil {
		LogF("build", err.Error())
	}
}

// ficheroCacheAppGoDeGrupo es la caché I4 del grupo i del dex de la app: el primero conserva
// `build/dexcache.bin`, el fichero de siempre, y los demás llevan su índice detrás. Van fuera de
// `build/dexcache/` a propósito: `pruneDepsCache` borra lo que no empieza por `deps-` en ese
// directorio, y la caché del proyecto no es un bucket de dependencias.
func ficheroCacheAppGoDeGrupo(grupo int) string {
	if grupo == 0 {
		return cacheDexFichero
	}
	return strings.TrimSuffix(cacheDexFichero, ".bin") + "-g" + strconv.Itoa(grupo+1) + ".bin"
}

// dexarAppEnGo escribe el dex de las clases del proyecto (y, por debajo de API 21, las de `jar/`)
// en `build/`: `classes.dex`, `classes2.dex`, … Es el mismo reparto que un bucket de dependencias
// (`dexDeClasesRepartidas`, reparto.go), con una caché I4 por grupo, porque una caché se cierra
// contra las tablas de un dex concreto. Un proyecto que cabe en un dex deja exactamente el mismo
// `classes.dex` y la misma `build/dexcache.bin` que antes del reparto: hay un solo grupo y se mide
// en las mismas tablas.
//
// Los ficheros se escriben de atrás adelante —`classesN.dex` primero y `classes.dex` al final—:
// un build que muere a mitad deja la app sin su primer dex, así que la vuelta siguiente lo ve
// faltar (`dexMissing`, build.go) y repite la etapa, en vez de encontrar un `classes.dex` y dar
// por buenos los grupos que no llegaron a escribirse.
func dexarAppEnGo(clases []classFull) ([][]byte, error) {
	caches := []*cacheDex{}
	dexes, err := dexDeClasesRepartidas(clases, func(grupo, total int) *cacheDex {
		// dexDeClasesRepartidas guarda cada caché en su fichero y cuenta lo reutilizado; aquí
		// solo se recogen para sumar el resumen que mira la prueba del dexado por clase
		c := cargaCacheDexDe(ficheroCacheAppGoDeGrupo(grupo))
		caches = append(caches, c)
		return c
	})
	if err != nil {
		return nil, err
	}
	for n := len(dexes) - 1; n >= 0; n-- {
		ruta := filepath.Join("build", dexEntryName(n+1))
		if err := os.WriteFile(ruta, dexes[n], 0644); err != nil {
			return nil, err
		}
	}
	// un dex de una vuelta anterior que hoy no toca: sin esto el paquete seguiría metiendo
	// clases que ya no están en el proyecto
	for n := len(dexes) + 1; ; n++ {
		ruta := filepath.Join("build", dexEntryName(n))
		if !exists(ruta) {
			break
		}
		if err := os.Remove(ruta); err != nil {
			return nil, err
		}
	}
	total := 0
	for _, dex := range dexes {
		total += len(dex)
	}
	if len(dexes) == 1 {
		LogI("build", len(clases), "clases ->", total, "bytes en", filepath.Join("build", dexEntryName(1)))
	} else {
		LogI("build", len(clases), "clases ->", total, "bytes en", len(dexes), "dex")
	}
	resumen := resumenCacheDex{}
	for _, c := range caches {
		r := c.resumen()
		resumen.reutilizados += r.reutilizados
		resumen.total += r.total
	}
	ultimoDexReuso = resumen
	return dexes, nil
}

// ultimoDexReuso es lo que la última llamada a dexarClasesEnGo reutilizó: la prueba del dexado por
// clase lo mira para saber que hubo reutilización de verdad y no un dexado completo que casualmente
// da los mismos bytes.
var ultimoDexReuso resumenCacheDex
