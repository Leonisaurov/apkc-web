package main

// `apkc dex`: la etapa de dex, suelta, sin `aapt2`, sin `javac` y sin `d8`.
//
// En el flujo normal de apkc el `.dex` lo escribe `d8` sobre los `.class` que dejaron `aapt2` y
// `javac`, y en wasm nada de eso existe (no hay proceso). Esta orden es la pieza que sí puede
// correr del lado del cliente: recibe `.class`, emite `classes.dex`, y se comprueba con oráculos
// que no comparten código con este fork (el propio `d8` re-dexando la salida, docs/evidencia.md
// §22).
//
// Lo que entra tienen que ser `.class` ya compilados: hasta que la Fase 1G escriba el compilador de
// Java en Go, el `javac` del equipo sigue siendo el que produce esos `.class`. Lo que sale es un dex
// del subconjunto medido (§14 y §21): fuera quedan lambdas, `invokedynamic` y cualquier cosa que
// necesite desugaring.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func dex() {
	set := flag.NewFlagSet("dex", flag.ExitOnError)
	set.Usage = func() {
		fmt.Print(`usage: apkc dex [-o classes.dex] carpeta-con-class [mas-carpetas...]

Turns the .class files of a directory tree into a classes.dex, in Go, with no d8 and no
subprocess anywhere. The input is whatever javac left on disk (build/classes, or the compiled
output of a jar); the output is the named file, because the progress lines share stdout with
the user and a dex does not belong among them.

Only the measured subset is dexed: an invokedynamic, a lambda or a call returning long/float/double
stops the command with its name, not a half-written dex.
`)
	}
	out := set.String("o", "build/outputs/classes.dex", "where the classes.dex goes")
	if err := set.Parse(os.Args[2:]); err != nil {
		LogF("dex", err.Error())
	}
	raices := set.Args()
	if len(raices) == 0 {
		LogF("dex", "apkc dex needs at least one directory with .class files; see `apkc dex -h`")
	}
	for _, r := range raices {
		// un switch escrito despues de las carpetas no es un switch para flag.FlagSet, y el
		// silencio convierte eso en un dex escrito donde nadie lo pidio: lo que hace `sign` con
		// su paquete, aqui lo hace el -o
		if strings.HasPrefix(r, "-") {
			LogF("dex", r, "was read as a folder: the switches go before the folders,",
				"as in apkc dex -o out.dex build/classes (and -- ends them)")
		}
	}
	clases, err := leeClases(raices)
	if err != nil {
		LogF("dex", err.Error())
	}
	if len(clases) == 0 {
		LogF("dex", "no .class file under", strings.Join(raices, ", "))
	}
	b, err := dexDeClases(clases)
	if err != nil {
		LogF("dex", err.Error())
	}
	LogI("dex", len(clases), "classes ->", len(b), "bytes of classes.dex")
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		LogF("dex", err.Error())
	}
	if err := os.WriteFile(*out, b, 0644); err != nil {
		LogF("dex", err.Error())
	}
	LogI("dex", "wrote", *out)
}

// dexDeClases es la etapa entera, sin disco: leer los .class ya está hecho aquí arriba, y lo que
// sale es el fichero. En dos líneas para que la página la pueda llamar sin reproducir el orden de
// internar, ordenar y ensamblar.
func dexDeClases(clases []classFull) ([]byte, error) {
	td := newDexTables()
	if err := td.collectClasses(clases); err != nil {
		return nil, err
	}
	td.orderTables()
	return td.armaDex(clases)
}

// leeClases baja todos los .class de los árboles dados, en un orden fijo: el internado depende del
// orden en que se recorren las clases (los índices de la tabla salen de ahí y luego se ordenan), y
// un `filepath.Walk` por carpeta de entrada no garantiza lo mismo en dos sistemas de ficheros.
func leeClases(raices []string) ([]classFull, error) {
	type entrada struct {
		ruta  string
		datos []byte
	}
	lista := []entrada{}
	for _, raiz := range raices {
		info, err := os.Stat(raiz)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if !strings.HasSuffix(raiz, ".class") {
				return nil, fmt.Errorf("%s is not a .class file and not a directory", raiz)
			}
			d, err := os.ReadFile(raiz)
			if err != nil {
				return nil, err
			}
			lista = append(lista, entrada{raiz, d})
			continue
		}
		err = filepath.Walk(raiz, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if i.IsDir() || !strings.HasSuffix(p, ".class") {
				return nil
			}
			d, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			lista = append(lista, entrada{p, d})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].ruta < lista[j].ruta })

	out := []classFull{}
	for _, e := range lista {
		// un ClassFile de un jar de dependencia no es código nuestro: los `.class` de `META-INF`
		// y el `module-info` se dejan pasar, porque d8 tampoco los dexa
		if strings.Contains(e.ruta, "META-INF") || filepath.Base(e.ruta) == "module-info.class" {
			continue
		}
		cf, err := readClassFull(e.datos)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.ruta, err)
		}
		out = append(out, cf)
	}
	return out, nil
}
