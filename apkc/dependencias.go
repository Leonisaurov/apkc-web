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
// resuelven nombres de Java) y quedan fuera de esta pieza.
//
// Desde que `--compiler go` también dexa las dependencias, este fichero tiene la segunda mitad: los
// `.class` de cada entrada del classpath leídos en memoria (`clasesDeJar`) y la lista de lo que hay
// que dexar de `jar/` (`dependenciasDexeables`). Es la misma lista que recibe d8 en el camino javac,
// para que los dos caminos de `apkc build` dexen exactamente el mismo conjunto.

import (
	"archive/zip"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	return entradasDelClasspathEn("", rutaAndroidJar)
}

// entradasDelClasspathEn es entradasDelClasspath sobre la raíz de un proyecto: `jar/` se busca ahí y
// no en el directorio de trabajo. `compile` y `build` pasan vacío porque trabajan sobre el cwd;
// `check` diagnostica un fichero que puede estar fuera de él y tiene que ver las dependencias de su
// proyecto, no las del directorio desde el que se invocó.
// Antes de devolverlas comprueba que ninguna clase la definan dos dependencias, con la misma guarda
// que usa el build (`claseDuplicadaEntreJars`): d8 aborta en ese caso y el dexer de este fork se
// quedaba con una de las dos copias, así que `check` tampoco puede resolver al primero por orden
// léxico y decir que el proyecto compila. El `android.jar` queda fuera de la comprobación porque no
// es una entrada dexable: ni `build` lo abre ahí y su directorio central son miles de nombres en cada
// pasada del diagnóstico.
func entradasDelClasspathEn(raiz, rutaAndroidJar string) ([]entradaClasspath, error) {
	entradas := []entradaClasspath{{abrir: rutaAndroidJar, huella: huellaDelJarTexto(rutaAndroidJar)}}
	dependencias := []string{}
	for _, jar := range jarsEn(raiz) {
		entradas = append(entradas, entradaClasspath{abrir: jar, huella: huellaDelJarTexto(jar)})
		dependencias = append(dependencias, jar)
	}
	for _, aar := range aarsEn(raiz) {
		classes, err := classesJarDelAar(aar)
		if err != nil {
			return nil, err
		}
		if classes == "" {
			// un AAR sin clases (solo recursos) no aporta nombres que resolver
			continue
		}
		entradas = append(entradas, entradaClasspath{abrir: classes, huella: huellaDelJarTexto(aar)})
		dependencias = append(dependencias, classes)
	}
	if err := claseDuplicadaEntreJars(dependencias); err != nil {
		return nil, err
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
	return aarsEn("")
}

// jarsEn y aarsEn son las dependencias de la raíz que se les da (vacía: el directorio de trabajo).
func jarsEn(raiz string) []string {
	jars, _ := getFiles(filepath.Join(raiz, "jar"), ".jar", false)
	return jars
}

func aarsEn(raiz string) []string {
	aars, _ := getFiles(filepath.Join(raiz, "jar"), ".aar", false)
	return aars
}

// dependenciasDexeables es lo que hay que dexar de `jar/`, con la huella de la que sale cada entrada:
// cada `.jar` tal cual y, de cada `.aar`, su `classes.jar` extraído. La huella de un AAR es la del
// AAR entero y no la de su `classes.jar`, para que cambiar un recurso del AAR vuelva a dexar sus
// clases igual que invalida las del compilador. Es la lista que reciben los dos dexers —d8 en el
// camino javac y `dexDeClases` en el camino Go—, así que los dos empaquetan el mismo conjunto.
// Antes de devolverla comprueba que ninguna clase la definan dos entradas: con una colisión d8
// aborta y el dexer de este fork se quedaba con una de las dos copias sin decirlo.
func dependenciasDexeables() ([]string, []string) {
	rutas := []string{}
	digests := []string{}
	for _, jar := range allJars() {
		digest, err := fileSHA1(jar)
		if err != nil {
			LogF("build", err, jar)
		}
		rutas = append(rutas, jar)
		digests = append(digests, digest)
	}
	for _, aar := range allAars() {
		classes, err := classesJarDelAar(aar)
		if err != nil {
			LogF("build", err.Error())
		}
		if classes == "" {
			// un AAR de solo recursos no aporta clases que dexar
			continue
		}
		digest, err := fileSHA1(aar)
		if err != nil {
			LogF("build", err, aar)
		}
		rutas = append(rutas, classes)
		digests = append(digests, digest)
	}
	if err := claseDuplicadaEntreJars(rutas); err != nil {
		LogF("build", err.Error())
	}
	return rutas, digests
}

// claseDuplicadaEntreJars busca una clase que definan dos entradas dexables a la vez. d8 aborta en
// ese caso con «Type X is defined multiple times»; el dexer de este fork concatenaba las dos listas,
// así que el build terminaba en verde con una clase ambigua que el runtime resolvía a su gusto. Solo
// se lee el directorio central, como en `shadowedClass`, y el error nombra la clase y las dos rutas
// para que el diagnóstico sea el mismo que el de d8.
func claseDuplicadaEntreJars(rutas []string) error {
	if len(rutas) < 2 {
		// un solo jar no puede colisionar consigo mismo: no se abre nada
		return nil
	}
	vista := map[string]string{}
	for _, ruta := range rutas {
		clases, err := jarClasses(ruta)
		if err != nil {
			// un jar ilegible ya lo reporta el dexer; aquí no puede tapar la colisión del resto
			LogW("build", "could not read", ruta, err.Error())
			continue
		}
		sort.Strings(clases)
		for _, clase := range clases {
			// `jarClasses` ya descarta `META-INF/` y las formas con paquete, pero no el
			// `module-info.class` de la raíz: dos jars modulares no colisionan por él.
			if clase == "module-info" || clase == "package-info" {
				continue
			}
			if antes, visto := vista[clase]; visto {
				return fmt.Errorf("la clase %s está definida más de una vez: %s y %s",
					strings.ReplaceAll(clase, "/", "."), antes, ruta)
			}
			vista[clase] = ruta
		}
	}
	return nil
}

// clasesDeJar lee los `.class` de un jar o de un `classes.jar` en memoria, en orden de nombre de
// entrada, que es lo que hace determinista el internado del dex: el mismo jar da el mismo fichero.
// Se saltan `META-INF/` y los `module-info`/`package-info`, que d8 tampoco dexa. La huella de cada
// clase es el SHA-1 de sus bytes, que es la clave del dexado por clase (I4, dexreuso.go).
func clasesDeJar(ruta string) ([]classFull, error) {
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		return nil, fmt.Errorf("%s no abre: %w", ruta, err)
	}
	defer zr.Close()
	nombres := []string{}
	porNombre := map[string]*zip.File{}
	for _, f := range zr.File {
		nombre := f.Name
		if !strings.HasSuffix(nombre, ".class") || strings.HasPrefix(nombre, "META-INF/") ||
			strings.HasSuffix(nombre, "/module-info.class") || strings.HasSuffix(nombre, "/package-info.class") ||
			nombre == "module-info.class" || nombre == "package-info.class" {
			continue
		}
		nombres = append(nombres, nombre)
		porNombre[nombre] = f
	}
	sort.Strings(nombres)
	out := make([]classFull, 0, len(nombres))
	for _, nombre := range nombres {
		rc, err := porNombre[nombre].Open()
		if err != nil {
			return nil, err
		}
		datos, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		cf, err := readClassFull(datos)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", ruta, nombre, err)
		}
		cf.huella = sha1.Sum(datos)
		out = append(out, cf)
	}
	return out, nil
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
	tmp := fmt.Sprintf("%s.%s.tmp", destino, sufijoDeTemporal())
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
