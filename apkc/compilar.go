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

func compilar() {
	set := flag.NewFlagSet("compile", flag.ExitOnError)
	set.Usage = func() {
		fmt.Print(`usage: apkc compile --android-jar android.jar [-o build/classes] [carpeta-con-java ...]

Turns the .java files of a directory tree into .class files, in Go, with no javac and no subprocess
anywhere. The output goes where the dexer looks for it, one file per class, named after the internal name
(R and R$layout are separate files, exactly like javac leaves them).

Only the measured subset compiles: declarations, @Override (which leaves no trace in the class file),
calls on this, on a written receiver, on a type and on super, string and integer literals, inlined
"static final int" constants like R.layout.main, and anonymous classes with the this$0 field javac gives
them. Anything else - an if, a long, an annotation other than @Override, a class the classpath does not
name - stops the command with its line, before writing a single file.

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
