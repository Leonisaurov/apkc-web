package main

// `apkc check` lee un fuente Java y devuelve en stdout un JSON con los errores que el parser y el
// emisor encuentran, sin escribir ni un .class. Es la pieza que la pestaña llama mientras se
// escribe: un comando corto, sin procesos y con un contrato que la página pueda leer.
//
// El parser y el emisor se paran en el primer error (no hay recuperación), así que el JSON trae
// ese error con su línea. Lo que sí se distingue es si el emisor pudo correr: sin android.jar
// solo se comprueba la sintaxis y el aviso lo dice, para no fingir que se comprobó el classpath.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type errorDeCheck struct {
	Linea   int    `json:"linea"`
	Mensaje string `json:"mensaje"`
}

type resultadoDeCheck struct {
	Tipo    string         `json:"tipo"`
	Fichero string         `json:"fichero"`
	Lineas  int            `json:"lineas"`
	Emisor  bool           `json:"emisor"`
	Aviso   string         `json:"aviso,omitempty"`
	Errores []errorDeCheck `json:"errores"`
}

var numeroDeLinea = regexp.MustCompile(`linea ([0-9]+):`)

// agregar traduce el error de Go, que trae la línea dentro del texto (a veces con el nombre del
// fichero delante y varias capas de %w), a la forma que la página subraya. Se toma la última
// mención porque es la más cercana al sitio donde el parser o el emisor pararon.
func (r *resultadoDeCheck) agregar(err error) {
	texto := strings.TrimSpace(err.Error())
	linea := 0
	for _, m := range numeroDeLinea.FindAllStringSubmatch(texto, -1) {
		if n, e := fmt.Sscanf(m[1], "%d", &linea); e != nil || n != 1 {
			linea = 0
		}
	}
	r.Errores = append(r.Errores, errorDeCheck{Linea: linea, Mensaje: texto})
}

// revisarFuenteJava es el cuerpo de `check`, separado del comando para poder comprobarlo con
// cualquier fuente sin pasar por los argumentos. Nunca escribe ficheros: compilarProyectoJava
// solo devuelve bytes en memoria.
func revisarFuenteJava(contenido, nombre, rutaJar string) resultadoDeCheck {
	r := resultadoDeCheck{
		Tipo: "check", Fichero: nombre, Lineas: strings.Count(contenido, "\n") + 1,
		Errores: []errorDeCheck{},
	}
	if _, err := parsearJava(contenido); err != nil {
		r.agregar(err)
		return r
	}
	if strings.TrimSpace(rutaJar) == "" {
		r.Aviso = "sin android.jar: solo se comprobó el parser"
		return r
	}
	_, err := compilarProyectoJava([]fuenteJava{{nombre: nombre, contenido: contenido}}, rutaJar)
	if err == nil {
		r.Emisor = true
		return r
	}
	if strings.Contains(err.Error(), "no abre") {
		r.Aviso = "sin android.jar: solo se comprobó el parser"
		return r
	}
	r.agregar(err)
	return r
}

func check() {
	set := flag.NewFlagSet("check", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: apkc check [--android-jar android.jar] fichero.java

Read one Java source and print one line of JSON on stdout with the errors the parser and the
emitter find, without writing any .class. The emitter needs the android.jar; without it the
report says so and only the parser is checked.
`)
	}
	jar := set.String("android-jar", androidJar, "the android.jar whose classes the sources link against")
	if err := set.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	fuentes := set.Args()
	if len(fuentes) != 1 {
		fmt.Fprintln(os.Stderr, "check: se espera un fichero .java")
		os.Exit(2)
	}
	d, err := os.ReadFile(fuentes[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}
	datos, err := json.Marshal(revisarFuenteJava(string(d), baseDeNombre(fuentes[0]), *jar))
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}
	fmt.Println(string(datos))
}
