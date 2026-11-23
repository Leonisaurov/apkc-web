package main

// `apkc check` lee un fuente Java —y, con `--proyecto`, también el resto de fuentes del proyecto,
// para que sus clases resuelvan— y devuelve en stdout un JSON con los errores que el parser y el
// emisor encuentran, sin escribir ni un .class. Es la pieza que la pestaña llama mientras se
// escribe: un comando corto, sin procesos y con un contrato que la página pueda leer.
//
// El parser y el emisor se paran en el primer error (no hay recuperación), así que el JSON trae
// ese error con su línea. Lo que sí se distingue es si el emisor pudo correr: sin android.jar
// solo se comprueba la sintaxis y el aviso lo dice, para no fingir que se comprobó el classpath.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type errorDeCheck struct {
	Linea   int `json:"linea"`
	Columna int `json:"columna,omitempty"`
	// Posicion es `fichero:linea:columna` —o `fichero:linea` cuando el error no nacio en un token—,
	// la forma en la que el editor lo enseña y con la que enlaza al sitio exacto.
	Posicion string `json:"posicion,omitempty"`
	Mensaje  string `json:"mensaje"`
}

type resultadoDeCheck struct {
	Tipo     string         `json:"tipo"`
	Fichero  string         `json:"fichero"`
	Lineas   int            `json:"lineas"`
	Emisor   bool           `json:"emisor"`
	Aviso    string         `json:"aviso,omitempty"`
	Recursos string         `json:"recursos,omitempty"`
	Errores  []errorDeCheck `json:"errores"`
}

// posicionEnMensaje reconoce la posicion que el parser y el emisor escriben delante de su explicacion.
// La columna solo aparece cuando el error nacio en un token; las sentencias llevan la linea sola.
var posicionEnMensaje = regexp.MustCompile(`linea ([0-9]+)(?:, columna ([0-9]+))?:`)

// prefijoDeMensaje y prefijoDeLinea son las capas que se apilan al propagar el error: el nombre del
// fichero que anade `compilarProyectoJava` y un `linea N:` por cada `%w`. Se quitan del texto para que
// la lista del editor enseñe la explicacion y no cinco posiciones seguidas.
var (
	prefijoDeMensaje = regexp.MustCompile(`^[^\s:]+\.java: `)
	prefijoDeLinea   = regexp.MustCompile(`^linea [0-9]+(?:, columna [0-9]+)?: `)
)

// agregar traduce el error de Go, que trae la posicion dentro del texto (a veces con el nombre del
// fichero delante y varias capas de %w), a la forma que la pagina subraya. Se toma la ultima
// mencion porque es la mas cercana al sitio donde el parser o el emisor pararon, y se deja el resto
// del texto como explicacion, sin las capas de posicion repetidas.
func (r *resultadoDeCheck) agregar(err error) {
	texto := strings.TrimSpace(err.Error())
	linea, columna := 0, 0
	for _, m := range posicionEnMensaje.FindAllStringSubmatch(texto, -1) {
		linea, columna = 0, 0
		if n, e := fmt.Sscanf(m[1], "%d", &linea); e != nil || n != 1 {
			linea = 0
		}
		if m[2] != "" {
			if n, e := fmt.Sscanf(m[2], "%d", &columna); e != nil || n != 1 {
				columna = 0
			}
		}
	}
	r.Errores = append(r.Errores, errorDeCheck{Linea: linea, Columna: columna, Posicion: r.posicion(linea, columna), Mensaje: limpiarMensaje(texto)})
}

// limpiarMensaje quita del texto las capas de posicion que se apilan al propagar el error: el nombre
// del fichero que anade el emisor y un `linea N:` por cada `%w`. Queda la explicacion sola.
func limpiarMensaje(texto string) string {
	texto = strings.TrimSpace(texto)
	for {
		quitado := false
		for _, re := range []*regexp.Regexp{prefijoDeMensaje, prefijoDeLinea} {
			if donde := re.FindStringIndex(texto); donde != nil {
				texto = texto[donde[1]:]
				quitado = true
			}
		}
		if !quitado {
			break
		}
	}
	return texto
}

// posicion: `fichero:linea:columna`, o `fichero:linea` si el error no trae columna.
func (r *resultadoDeCheck) posicion(linea, columna int) string {
	if linea <= 0 {
		return r.Fichero
	}
	if columna > 0 {
		return fmt.Sprintf("%s:%d:%d", r.Fichero, linea, columna)
	}
	return fmt.Sprintf("%s:%d", r.Fichero, linea)
}

// fuentesDeRecursos: las clases que el proyecto genera a partir de sus recursos, leidas **sin escribir
// nada**. `check` compila un solo fichero contra el android.jar, y la R no viaja en el jar: sin esto,
// `setContentView(R.layout.main)` no resolvia `R` y el diagnostico acusaba un error que la
// compilacion nunca da (§200). Se prefiere la R que el proyecto ya tiene escrita —la deja `apkc build`,
// con aapt2 o con el enlazador de Go— y, si no hay, se enlaza en memoria con el mismo enlazador que
// usa `build --linker go`: los ids salen de res/ y no se inventan.
func fuentesDeRecursos(raiz, rutaJar string, version int, codename string) ([]fuenteJava, string) {
	if enDisco := rDelProyectoEnDisco(raiz); len(enDisco) > 0 {
		return enDisco, "R.java del proyecto"
	}
	opts, err := opcionesDelProyecto(rutaJar, version, codename)
	if err != nil {
		return nil, "sin R: " + err.Error()
	}
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(raiz)
	if err != nil {
		return nil, "sin R: " + err.Error()
	}
	if len(valores) == 0 && len(layouts) == 0 && len(ficheros) == 0 {
		return nil, "sin R: el proyecto no tiene res/ que enlazar"
	}
	m, err := marcoDelAndroidJar(rutaJar)
	if err != nil {
		return nil, "sin R: la tabla de ids del framework: " + err.Error()
	}
	enlazado, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, m, opts)
	if err != nil {
		return nil, "sin R: " + err.Error()
	}
	paquete := strings.ReplaceAll(enlazado.tabla.nombrePaquete, ".", "/")
	salida := []fuenteJava{{nombre: "src/" + paquete + "/R.java", contenido: enlazado.r}}
	if enlazado.manifestJava != "" {
		salida = append(salida, fuenteJava{nombre: "src/" + paquete + "/Manifest.java", contenido: enlazado.manifestJava})
	}
	return salida, "R enlazada de res/"
}

// rDelProyectoEnDisco busca la R que el proyecto ya tiene escrita: `src/<paquete>/R.java` es donde la
// deja `apkc build` (con cualquiera de los dos enlazadores) y `gen/` es la carpeta clasica de aapt2.
func rDelProyectoEnDisco(raiz string) []fuenteJava {
	salida := []fuenteJava{}
	for _, base := range []string{"src", "gen"} {
		carpeta := filepath.Join(raiz, base)
		_ = filepath.WalkDir(carpeta, func(ruta string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			nombre := d.Name()
			if nombre != "R.java" && nombre != "Manifest.java" {
				return nil
			}
			datos, err := os.ReadFile(ruta)
			if err != nil {
				return nil
			}
			rel, err := filepath.Rel(raiz, ruta)
			if err != nil {
				rel = ruta
			}
			salida = append(salida, fuenteJava{nombre: filepath.ToSlash(rel), contenido: string(datos)})
			return nil
		})
	}
	return salida
}

// otrosFuentesDelProyecto lee los `.java` del proyecto que no son el fuente pedido. El emisor resuelve
// los nombres entre ficheros: una clase del propio proyecto escrita en otro `.java` tiene que estar
// delante o `check` la acusa de no existir (el mismo defecto que ya se corrigió para la R). Se saltan
// el fuente pedido —por ruta si se conoce y, siempre, por su nombre, que es el que lleva la pestaña
// cuando copia el buffer a un temporal fuera de `src/`— y las clases generadas (R y Manifest), que ya
// aporta fuentesDeRecursos. Cada fichero se lee una sola vez y no se escribe nada.
func otrosFuentesDelProyecto(proyecto, rutaPedida, nombreInforme string) []fuenteJava {
	salida := []fuenteJava{}
	pedida := ""
	if rutaPedida != "" {
		pedida = filepath.Clean(rutaPedida)
	}
	informe := baseDeNombre(nombreInforme)
	for _, base := range []string{"src", "gen"} {
		carpeta := filepath.Join(proyecto, base)
		_ = filepath.WalkDir(carpeta, func(ruta string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			nombre := d.Name()
			if !strings.HasSuffix(nombre, ".java") || nombre == "R.java" || nombre == "Manifest.java" {
				return nil
			}
			if pedida != "" && filepath.Clean(ruta) == pedida {
				return nil
			}
			if nombre == informe {
				return nil
			}
			datos, err := os.ReadFile(ruta)
			if err != nil {
				return nil
			}
			rel, err := filepath.Rel(proyecto, ruta)
			if err != nil {
				rel = ruta
			}
			salida = append(salida, fuenteJava{nombre: filepath.ToSlash(rel), contenido: string(datos)})
			return nil
		})
	}
	return salida
}

// fuenteDelError devuelve el fichero en el que nació un fallo del emisor, o nil si el error no viene
// de un fuente (por ejemplo, un classpath que no abre).
func fuenteDelError(err error) *errorDeFuente {
	var de *errorDeFuente
	if errors.As(err, &de) {
		return de
	}
	return nil
}

// raizDelProyecto sube desde la carpeta del fichero hasta la que tiene AndroidManifest.xml, que es la
// raiz del proyecto. Devuelve "" cuando no hay ninguna: entonces no se inventa una R.
func raizDelProyecto(dir string) string {
	if dir == "" {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AndroidManifest.xml")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return ""
		}
		dir = padre
	}
}

// revisarFuenteJava es el cuerpo de `check`, separado del comando para poder comprobarlo con
// cualquier fuente sin pasar por los argumentos. Nunca escribe clases: compilarProyectoJava solo
// devuelve bytes en memoria. `proyecto` es la raiz del proyecto, de donde salen la R y las
// dependencias de `jar/`; vacio significa que solo se comprueba el fichero.
func revisarFuenteJava(contenido, nombre, rutaJar, proyecto string) resultadoDeCheck {
	return revisarFuenteJavaComo(contenido, nombre, nombre, rutaJar, proyecto, 0, "")
}

// revisarFuenteJavaComo separa el nombre del fuente (el SourceFile, que es el del fichero que se lee)
// del nombre que va en las posiciones del informe. La pestaña comprueba un temporal y quiere que el
// error diga `MainActivity.java:15:24`, no `.apkc-check.java`.
func revisarFuenteJavaComo(contenido, nombreFuente, nombreInforme, rutaJar, proyecto string, version int, codename string) resultadoDeCheck {
	return revisarFuenteJavaEn(contenido, nombreFuente, nombreInforme, "", rutaJar, proyecto, version, codename)
}

// revisarFuenteJavaEn es lo mismo con la ruta real del fuente pedido. Sirve para no compilar dos veces
// el mismo fichero cuando el comando lee un `.java` de dentro del proyecto: los demás fuentes se
// recogen del árbol y el pedido se salta por su ruta.
func revisarFuenteJavaEn(contenido, nombreFuente, nombreInforme, rutaFuente, rutaJar, proyecto string, version int, codename string) resultadoDeCheck {
	r := resultadoDeCheck{
		Tipo: "check", Fichero: nombreInforme, Lineas: strings.Count(contenido, "\n") + 1,
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
	fuentes := []fuenteJava{{nombre: nombreFuente, contenido: contenido}}
	if proyecto != "" {
		// el índice del android.jar y el marco de recursos ya parseado que dejó el build viven en
		// <proyecto>/build/…, no en el cwd: la pestaña lanza `check` con el cwd que quedó del último
		// comando y no hay ninguna razón para que sea la raíz del proyecto. Fijarlos aquí hace que
		// el marco del framework salga de la caché en vez de recorrer el zip y parsear
		// resources.arsc en cada pasada.
		directorioDeIndicesDelJar = filepath.Join(proyecto, dirIndiceDelJar)
		directorioDeMarcoCacheado = filepath.Join(proyecto, dirMarcoCacheado)
		defer func() {
			directorioDeIndicesDelJar = ""
			directorioDeMarcoCacheado = ""
		}()
		extras, nota := fuentesDeRecursos(proyecto, rutaJar, version, codename)
		fuentes = append(fuentes, extras...)
		r.Recursos = nota
		// el resto de fuentes del proyecto va después del pedido: el emisor registra todas las
		// clases antes de emitir nada, y al emitir el pedido primero su diagnóstico no queda detrás
		// de un fallo de otro fichero.
		fuentes = append(fuentes, otrosFuentesDelProyecto(proyecto, rutaFuente, nombreInforme)...)
	}
	// el classpath es el mismo que el de `compile` y `build`: el `android.jar` y las dependencias del
	// proyecto (cada `jar/*.jar` y el `classes.jar` de cada `.aar`). Antes se compilaba contra el
	// `android.jar` solo, y un tipo que vive en un jar se subrayaba como error que la compilación no da.
	// El `classes.jar` de un AAR que aún no se extrajo deja el fichero de `build/aar/`, que es la caché
	// que `build` ya usa; el diagnóstico no escribe ni un `.class` ni una `R.java`.
	entradas, err := entradasDelClasspathEn(proyecto, rutaJar)
	if err != nil {
		r.agregar(err)
		return r
	}
	_, err = compilarProyectoJavaConClasspath(fuentes, rutasDelClasspath(entradas))
	if err == nil {
		r.Emisor = true
		return r
	}
	if strings.Contains(err.Error(), "no abre") {
		r.Aviso = "sin android.jar: solo se comprobó el parser"
		return r
	}
	// el fuente pedido es el que se diagnostica. Si el primer fallo es de otro `.java` del proyecto,
	// no se cuela en su lista: se dice en `aviso` y no se da el pedido por comprobado, salvo que el
	// fallo sea de emisión, porque entonces el pedido —que va primero— ya se emitió sin error.
	if de := fuenteDelError(err); de != nil && baseDeNombre(de.fuente) != baseDeNombre(nombreFuente) {
		if de.etapa == "emisor" {
			r.Emisor = true
		}
		r.Aviso = "otro fuente del proyecto no compila: " + baseDeNombre(de.fuente) + ": " + de.err.Error()
		return r
	}
	r.agregar(err)
	return r
}

func check() {
	set := flag.NewFlagSet("check", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: apkc check [--android-jar android.jar] [--proyecto raiz] fichero.java

Read one Java source and print one line of JSON on stdout with the errors the parser and the
emitter find, without writing any .class. The emitter needs the android.jar; without it the
report says so and only the parser is checked.

--proyecto is the root of the project the file belongs to: its res/ (or the R.java that a build
already wrote) is linked in memory so that R.layout.main resolves, and the other .java files under
its src/ and gen/ are compiled together with the requested one so that its classes resolve. Only the
requested file is reported: an error in another file never becomes a row for it. Without --proyecto
no R is available, no other source is read, and any resource access is reported as the unresolved
name it is. When the flag is missing, the directory of the file and its ancestors are searched for
an AndroidManifest.xml.

--sdk-version and --codename say which platform the resources are linked against when the path of
the android.jar does not carry it, which is the case in the tab (/sdk/android.jar).
`)
	}
	jar := set.String("android-jar", androidJar, "the android.jar whose classes the sources link against")
	proyecto := set.String("proyecto", "", "the project root whose resources give the R class; by default the file's directory and its ancestors")
	nombre := set.String("nombre", "", "the file name the positions are reported with; by default the name of the file being read")
	sdkVersion := set.Int("sdk-version", 0, "the API level the resources are linked against; by default it is read from the android.jar path")
	sdkCodename := set.String("codename", "", "the Android version name of that platform (\"14\" for API 34); by default it is read from --sdk-version when it is measured")
	if err := set.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	fuentes := set.Args()
	if len(fuentes) != 1 {
		fmt.Fprintln(os.Stderr, "check: se espera un fichero .java")
		os.Exit(2)
	}
	raiz := *proyecto
	if raiz == "" {
		raiz = raizDelProyecto(filepath.Dir(fuentes[0]))
	}
	informe := *nombre
	if informe == "" {
		informe = baseDeNombre(fuentes[0])
	}
	d, err := os.ReadFile(fuentes[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}
	datos, err := json.Marshal(revisarFuenteJavaEn(string(d), fuentes[0], informe, fuentes[0], *jar, raiz, *sdkVersion, *sdkCodename))
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}
	fmt.Println(string(datos))
}
