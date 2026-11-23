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
// del subconjunto medido (§14, §21 y §110): una lambda estática sin capturas se desugra a una clase
// sintética; las capturas, las referencias de instancia y la concatenación dinámica se rechazan por
// nombre.

import (
	"crypto/sha1"
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

Only the measured subset is dexed: a static lambda without captures is desugared to a synthetic
class; captures, instance method references or a dynamic concatenation stop the command with their
name, not a half-written dex.
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
	return dexDeClasesConCache(clases, nil)
}

// dexDeClasesConCache es lo mismo con la caché del dexado por clase (I4, dexreuso.go). Con `c` a
// nil —`apkc dex` y las pruebas de la etapa— se traduce todo, que es el camino de referencia.
func dexDeClasesConCache(clases []classFull, c *cacheDex) ([]byte, error) {
	clases, err := expandirLambdas(clases)
	if err != nil {
		return nil, err
	}
	return dexDeClasesExpandidas(clases, c)
}

// dexDeClasesExpandidas es la etapa con las lambdas ya expandidas: es la pieza que el reparto de
// un bucket necesita, porque expandir dos veces añadiría dos veces las clases sintéticas.
func dexDeClasesExpandidas(clases []classFull, c *cacheDex) ([]byte, error) {
	td := newDexTables()
	if err := td.collectClasses(clases); err != nil {
		return nil, err
	}
	td.orderTables()
	if c != nil {
		// la caché se cierra contra las tablas ya ordenadas: hasta aquí no se sabe si los índices
		// que guarda la vuelta anterior siguen nombrando lo mismo
		c.activa(td)
		td.cache = c
	}
	return td.armaDex(clases)
}

// gruposDeClasesRepartidas expande las lambdas una sola vez y parte las clases en tantos grupos
// como haga falta para que ninguno pase del techo de los índices de 16 bits (reparto.go). Es la
// mitad de `dexDeClasesRepartidas` que también necesita quien tiene que decidir si el reparto es
// admisible **antes** de escribir ningún fichero (dexarAppEnGo, compilar.go).
func gruposDeClasesRepartidas(clases []classFull, fijas map[string]bool) ([]grupoDex, error) {
	clases, err := expandirLambdas(clases)
	if err != nil {
		return nil, err
	}
	// las fijas se nombran antes de expandir las lambdas, que solo pueden añadir clases sintéticas
	// detrás de su clase dueña: una lambda de una clase del proyecto no se queda fuera del dex
	// primario por no estar en la lista de nombres originales
	grupos, err := reparteEnDexesConFijas(clases, fijas, refsPorDex)
	if err != nil {
		return nil, err
	}
	if len(grupos) == 0 {
		return nil, fmt.Errorf("there are no classes to dex")
	}
	return grupos, nil
}

// armaGrupos ensambla los grupos en orden, con la caché I4 que `cacheDe` dé para cada uno —nil sin
// caché—, porque una caché se cierra contra las tablas de un dex concreto y un reparto tiene una
// por fichero. Devuelve los dex en el orden en que el apk los numera: classes.dex, classes2.dex, …
func armaGrupos(grupos []grupoDex, cacheDe func(grupo, total int) *cacheDex) ([][]byte, error) {
	dexes := make([][]byte, 0, len(grupos))
	for i, g := range grupos {
		var c *cacheDex
		if cacheDe != nil {
			c = cacheDe(i, len(grupos))
		}
		b, err := armaGrupo(g, c)
		if err != nil {
			if len(grupos) > 1 {
				// el fichero que no se pudo escribir, para que el rechazo no se lea como el del
				// bucket entero: con un solo grupo el mensaje es el de siempre, carácter a carácter
				return nil, fmt.Errorf("el dex %d de %d (%d clases): %w", i+1, len(grupos), len(g.clases), err)
			}
			return nil, err
		}
		if c != nil {
			c.guarda()
			if r := c.resumen(); r.total > 0 {
				LogI("build", r.reutilizados, "de", r.total, "code_item reutilizados de", c.fichero)
			}
		}
		dexes = append(dexes, b)
	}
	return dexes, nil
}

// dexDeClasesRepartidas es la etapa de un bucket de dependencias: los grupos de
// `gruposDeClasesRepartidas` ensamblados con la caché de cada uno. Devuelve los dex en el orden en
// que el apk los numera: classes.dex, classes2.dex, …
func dexDeClasesRepartidas(clases []classFull, cacheDe func(grupo, total int) *cacheDex) ([][]byte, error) {
	grupos, err := gruposDeClasesRepartidas(clases, nil)
	if err != nil {
		return nil, err
	}
	return armaGrupos(grupos, cacheDe)
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
		cf.huella = sha1.Sum(e.datos)
		out = append(out, cf)
	}
	return out, nil
}
