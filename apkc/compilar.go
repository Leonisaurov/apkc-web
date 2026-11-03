package main

// `apkc compile`: el `.class` sin `javac`. Es la pieza que quedaba del objetivo de la página: leer los
// fuentes que el editor escribe y dejar los ficheros que `apkc dex` convierte en `classes.dex`, todo dentro
// del mismo proceso y sin levantar nada.
//
// La resolución de tipos se hace contra el `android.jar` que se pase por bandera, y solo se leen las
// entradas que el fuente nombra: sobre la plantilla del parrafo 40 son cuatro ficheros de unos pocos KB, no
// los 26 MB del jar.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

Type names resolve against the android.jar passed in, reading only the entries the sources mention.
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
	clases, err := compilarProyectoJava(fuentes, *jar)
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

// revisarCompilerGo nombra lo que `--compiler go` todavia no mide. Se aislo del cuerpo de `build` para
// poder probarlo: las negaciones de un portero que solo vive dentro de un LogF no se comprueban.
func revisarCompilerGo(o buildOpts) error {
	if kt, _ := getFiles("src", ".kt", false); len(kt) > 0 {
		return fmt.Errorf("--compiler go no compila Kotlin: %d fuentes .kt en src", len(kt))
	}
	if jars := allJars(); len(jars) > 0 {
		return fmt.Errorf("--compiler go no dexa dependencias: hay %d jars en jar/ y eso lo hace d8", len(jars))
	}
	if o.useAAB {
		return fmt.Errorf("--compiler go no escribe bundles: d8 es quien produce el .aar de las dependencias")
	}
	return nil
}

// compilarFuentesEnGo es la etapa de `apkc build --compiler go` que sustituye a javac: lee src/*.java,
// escribe build/classes/*.class y dice los fuentes que paso, que es lo que la firma de la etapa espera.
// No hay incremental aqui: el emisor resuelve los nombres entre ficheros, así que un fuente que cambia
// obliga a compilarlos todos, y eso es lo que hace.
func compilarFuentesEnGo() ([]string, bool) {
	defer timeIt("compile go")()
	fuentes, err := fuentesJava([]string{"src"})
	if err != nil {
		LogF("build", err.Error())
	}
	if len(fuentes) == 0 {
		return nil, false
	}
	clases, err := compilarProyectoJava(fuentes, androidJar)
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
	LogI("build", len(nombres), "fuentes de Java en Go ->", len(clases), "clases")
	return nombres, true
}

// dexarClasesEnGo es la etapa que sustituye a d8 sobre las clases del proyecto: las lee de build/classes
// con el lector de la Fase 2 y escribe build/classes.dex, el sitio de donde lo toma el empaquetado.
func dexarClasesEnGo() {
	defer timeIt("dex go")()
	clases, err := leeClases([]string{filepath.Join("build", "classes")})
	if err != nil {
		LogF("build", err.Error())
	}
	if len(clases) == 0 {
		return
	}
	dex, err := dexDeClases(clases)
	if err != nil {
		LogF("build", err.Error())
	}
	ruta := filepath.Join("build", "classes.dex")
	if err := os.WriteFile(ruta, dex, 0644); err != nil {
		LogF("build", err.Error())
	}
	LogI("build", len(clases), "clases ->", len(dex), "bytes en", ruta)
}
