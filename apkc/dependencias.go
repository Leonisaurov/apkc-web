package main

// El classpath del compilador Go: el `android.jar` y las dependencias que el proyecto ya trae en
// `jar/`.
//
// El emisor resuelve los nombres leyendo las clases que el fuente nombra, y el classpath es el mismo
// que javac recibe: el `android.jar` delante y después cada dependencia. Un `.jar` se abre tal cual;
// un `.aar` no es un zip de clases, sino un paquete con `classes.jar` dentro, así que ese
// `classes.jar` se extrae a `build/aar/` y se abre como cualquier otro jar. La extracción se nombra
// por la huella del AAR, así que un AAR que no cambió no se vuelve a escribir.
//
// Nada de esto resuelve Maven ni descarga: lo que hay en `jar/` es lo que se compila. El `res/` y el
// `lib/<abi>/` de un AAR no son entradas del compilador (los recursos y las bibliotecas nativas no
// resuelven nombres de Java) y quedan fuera de esta pieza; el dex de las dependencias tampoco lo
// escribe este camino (sigue siendo d8).

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// entradaClasspath es un fichero que se abre como zip de clases y la huella de la dependencia de la
// que sale. En un `.jar` las dos cosas son el mismo fichero; en un `.aar` la huella es la del AAR y
// lo que se abre es su `classes.jar` extraído, para que cambiar cualquier byte del AAR invalide las
// clases reutilizadas de I3 aunque el `classes.jar` de dentro no se moviera.
type entradaClasspath struct {
	abrir  string
	huella string
}

// entradasDelClasspath arma el classpath del compilador: el `android.jar` primero y las dependencias
// después, en el orden de `getFiles` (léxico y estable, para que dos builds del mismo proyecto abran
// los jars en el mismo orden).
func entradasDelClasspath(rutaAndroidJar string) ([]entradaClasspath, error) {
	entradas := []entradaClasspath{{abrir: rutaAndroidJar, huella: huellaDelJarTexto(rutaAndroidJar)}}
	for _, jar := range allJars() {
		entradas = append(entradas, entradaClasspath{abrir: jar, huella: huellaDelJarTexto(jar)})
	}
	for _, aar := range allAars() {
		classes, err := classesJarDelAar(aar)
		if err != nil {
			return nil, err
		}
		if classes == "" {
			// un AAR sin clases (solo recursos) no aporta nombres que resolver
			continue
		}
		entradas = append(entradas, entradaClasspath{abrir: classes, huella: huellaDelJarTexto(aar)})
	}
	return entradas, nil
}

// rutasDelClasspath es lo que se le pasa a `abrirClasspath`: los ficheros que se abren, en orden.
func rutasDelClasspath(entradas []entradaClasspath) []string {
	rutas := make([]string, 0, len(entradas))
	for _, e := range entradas {
		rutas = append(rutas, e.abrir)
	}
	return rutas
}

// huellaDelClasspath resume el classpath entero: la huella de cada entrada, en orden, incluido el
// `android.jar` y el AAR de origen de cada `classes.jar`. Es lo que se guarda en el estado del
// incremental: otro classpath resuelve los nombres contra otras clases, así que las de
// `build/classes` dejan de estar al día aunque no cambie ni un byte del proyecto.
func huellaDelClasspath(entradas []entradaClasspath) string {
	lineas := make([]string, 0, len(entradas))
	for _, e := range entradas {
		lineas = append(lineas, e.huella)
	}
	return strings.Join(lineas, "\n")
}

// allAars devuelve los `.aar` del proyecto. Es la mitad que `allJars` no ve: un AAR es un zip con
// `classes.jar`, `res/` y `lib/`, y el compilador solo consume su `classes.jar`.
func allAars() []string {
	aars, _ := getFiles("jar", ".aar", false)
	return aars
}

// dependenciasDelProyecto cuenta lo que hay en `jar/`: jars y aars. Lo lee el portero del compilador
// para nombrar la frontera del dex de las dependencias donde no hay procesos.
func dependenciasDelProyecto() []string {
	return append(append([]string{}, allJars()...), allAars()...)
}

// classesJarDelAar extrae el `classes.jar` de un AAR a `build/aar/` y devuelve su ruta. Si el AAR no
// tiene `classes.jar` devuelve vacío: es un AAR de solo recursos, y no hay nombres de Java que
// resolver. El nombre de la extracción lleva el SHA-1 de la ruta y el CRC del directorio central del
// AAR, así que un AAR con otros bytes deja otro fichero y uno con los mismos bytes reutiliza el que
// ya está.
func classesJarDelAar(rutaAar string) (string, error) {
	_, directorio, err := huellaDelJar(rutaAar)
	if err != nil {
		return "", fmt.Errorf("el AAR %s no abre: %w", rutaAar, err)
	}
	destino := filepath.Join("build", "aar", sha1Text(rutaAar)[:16]+fmt.Sprintf("-%08x.jar", directorio))
	if _, err := os.Stat(destino); err == nil {
		return destino, nil
	}
	datos, err := entradaDelZip(rutaAar, "classes.jar")
	if err != nil {
		return "", fmt.Errorf("el AAR %s no se puede leer: %w", rutaAar, err)
	}
	if datos == nil {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return "", err
	}
	// temporal y rename, como el índice del jar: un build que muere a mitad no puede dejar un
	// classes.jar corto con el nombre que dice ser el de este AAR
	tmp := fmt.Sprintf("%s.%d.tmp", destino, os.Getpid())
	if err := os.WriteFile(tmp, datos, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, destino); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return destino, nil
}

// entradaDelZip devuelve los bytes de la entrada `nombre` de un zip, o nil si no está. No usa el
// lector propio del índice porque aquí la entrada es una sola y archive/zip ya sabe andar cualquier
// forma de zip, incluidas las que el lector propio deja al respaldo.
func entradaDelZip(ruta, nombre string) ([]byte, error) {
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != nombre {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, nil
}
