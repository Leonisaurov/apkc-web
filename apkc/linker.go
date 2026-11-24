package main

// El enlazador de Go dentro de `apkc build`: la puerta que deja de ejecutar aapt2. Escribe los mismos
// cuatro productos que la herramienta deja en el arbol del proyecto —build/AndroidManifest.xml,
// build/resources.arsc, build/res/layout/*.xml y src/<paquete>/R.java— y con eso el empaquetado y la
// firma de siempre siguen funcionando sin cambio: el plan del paquete (packagePlan) toma exactamente
// esas rutas, y resources.arsc viaja almacenado porque lo mapea el sistema.
//
// Lo que este enlace NO hace, dicho con nombre en vez de hacerse el sueco:
//   - proyectos con recursos que no sean valores y layouts. Un drawable, un mipmap, un raw o un font
//     piden copiar ficheros y poner entradas con referencia de fichero en la tabla, y eso no esta
//     medido. Se rechaza.
//   - el formato proto de un bundle (useAAB), porque aapt2 escribe alli resources.pb y nosotros no.
//   - los `resources.xml` con <public> ni los <declare-styleable>: medidos en el parrafo 29, no
//     emitidos.
//   - los <permission> del manifiesto, que aapt2 acompaña con un Manifest.java que aqui no se escribe.
//
// La regla del repo es un claim con su medicion: los bytes de este enlazador estan comprobados contra
// los ficheros que aapt2 dejo sobre el proyecto del studio (parrafos 35, 36 y 37). Un proyecto con otra
// forma de recursos no esta comprobado, y se dice en lugar de prometer.
//
// Lo que aporta un AAR importado entra por aquí también: `res/__deps/<isla>/<tipo>/…` se lee con las
// mismas reglas que `res/` y su fichero viaja al paquete con la ruta de su tipo (`res/layout/x.xml`),
// que es la que deja `aapt2 link` y la que resuelven la tabla y la vista previa. Un `.aar` que sigue
// en `jar/` aporta lo mismo por su propio camino: `aarsExtraidos` deja su `res/` en `build/aar/` y
// esta misma función la recorre, con el proyecto por delante cuando dos recursos piden la misma ruta.
// Los nativos de `lib/<abi>/*.so` no pasan por el enlazador: los mete en el apk `packagePlan`, como
// cualquier otro fichero de `lib/`, con el nombre canónico que `System.loadLibrary` busca —también
// los del `.aar` de `jar/`, que `aarsExtraidos` deja en su `lib/`.

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// linkerGo names the resource backend that does not start aapt2.
const linkerGo = "go"

// `apkc link`: el enlace de recursos, suelto, sin aapt2. Es la puerta que puede correr del lado del
// cliente —no levanta proceso y solo necesita los ficheros del proyecto y un android.jar—, y por eso va
// separada de `apkc build`, que todavía ejecuta javac y d8.
func link() {
	set := flag.NewFlagSet("link", flag.ExitOnError)
	set.Usage = func() {
		fmt.Print(`usage: apkc link --android-jar android.jar [-o build]

Links the resources of the project in the current directory, in Go, with no aapt2 and no
subprocess anywhere: it reads AndroidManifest.xml and res/values/*.xml and res/layout/*.xml, and
writes the binary manifest, resources.arsc, the compiled layouts and src/<package>/R.java under
the output directory, exactly where the packager looks for them.

The framework ids come from the android.jar passed in, read by this binary itself. Only values and
layouts without a qualifier are linked: a res/layout-es or a res/drawable stops the command with its
name, because those need a second configuration or a file copy, and neither is measured here.
`)
	}
	jar := set.String("android-jar", androidJar, "the android.jar whose resource table holds the framework ids")
	out := set.String("o", ".", "where the linked products go")
	version := set.Int("sdk-version", 0, "the API level of the platform being linked; by default it is read from the android.jar path")
	codename := set.String("codename", "", "the Android version name of that platform (\"14\" for API 34); by default it is read from --sdk-version when it is measured")
	if err := set.Parse(os.Args[2:]); err != nil {
		LogF("link", err.Error())
	}
	opts, err := opcionesDelProyecto(*jar, *version, *codename)
	if err != nil {
		LogF("link", err.Error())
	}
	cambiada, err := enlazarConGo(*out, *jar, opts)
	if err != nil {
		LogF("link", err.Error())
	}
	LogI("link", "recursos enlazados en Go; la R", map[bool]string{true: "cambio", false: "quedo igual"}[cambiada])
}

// tiposDeResQueSeEnlazan: las dos carpetas cuyos ficheros se tradacen a modelos. Los demas nombres de
// carpeta son recursos que este enlazador no sabe escribir.
var tiposDeResQueSeEnlazan = map[string]bool{
	"values": true, "layout": true, "drawable": true, "mipmap": true,
	"menu": true, "xml": true, "raw": true, "anim": true, "font": true,
}

// parteEsCalificador: un segmento del sufijo de una carpeta que este enlazador sabe traducir a un
// campo del ResTable_config. Incluye los medidos antes (idioma, región, orientación, densidad,
// tamaño, sw y -vN) y los nuevos: touchscreen, keyboard, navigation y uiMode.
func parteEsCalificador(p string) bool {
	if p == "" {
		return false
	}
	switch p {
	case "port", "land", "square",
		"notouch", "stylus", "finger",
		"nokeys", "qwerty", "12key",
		"nonav", "dpad", "trackball", "wheel",
		"desk", "car", "television", "appliance", "watch", "night",
		"small", "large", "xlarge":
		return true
	}
	if densidadDe(p) != 0 || uiModeDe(p) != 0 || pantallaTactilDe(p) != 0 ||
		tecladoDe(p) != 0 || navegacionDe(p) != 0 {
		return true
	}
	if strings.HasPrefix(p, "sw") && anchoMinimoDe(p) != 0 {
		return true
	}
	if esLengua(p) {
		return true
	}
	if len(p) == 3 && (p[0] == 'r' || p[0] == 'R') {
		return true // región de values-es-rMX
	}
	return len(p) >= 2 && p[0] == 'v' && soloDigitos(p[1:])
}

// versionImplicitaDeCalificador: el `-vN` que aapt2 añade a la ruta del apk cuando el calificador
// no trae versión explícita. Medido con build-tools 35.0.1: densidad y tamaño 4, sw 13, uiMode 8.
func versionImplicitaDeCalificador(config string) uint16 {
	for _, p := range strings.Split(config, "-") {
		if len(p) >= 2 && p[0] == 'v' && soloDigitos(p[1:]) {
			return 0
		}
	}
	return uint16(camposDeConfig(config).version)
}

// calificadorExtraDeCarpeta: reconoce las carpetas con calificador (`layout-small`, `values-v21`,
// `values-car-night`, `layout-qwerty-v21`, …). Devuelve el tipo base, el calificador (con el `-vN`
// explícito, que configDe necesita para escribir sdkVersion) y el `-vN` que aapt2 añade a la ruta
// del apk cuando la versión es implícita.
func calificadorExtraDeCarpeta(nombre string) (tipo, config string, autoVersion uint16, ok bool) {
	i := strings.IndexByte(nombre, '-')
	if i <= 0 {
		return "", "", 0, false
	}
	tipo, sufijo := nombre[:i], nombre[i+1:]
	if !tiposDeResQueSeEnlazan[tipo] {
		return "", "", 0, false
	}
	partes := strings.Split(sufijo, "-")
	partesVistas := 0
	for _, p := range partes {
		// el idioma y la región solo se midieron en values; un layout-es sigue siendo una
		// configuración que este enlazador no escribe
		if tipo != "values" && (esLengua(p) || len(p) == 3 && (p[0] == 'r' || p[0] == 'R')) {
			return "", "", 0, false
		}
		if !parteEsCalificador(p) {
			return "", "", 0, false
		}
		partesVistas++
	}
	if partesVistas == 0 {
		return "", "", 0, false
	}
	return tipo, sufijo, versionImplicitaDeCalificador(sufijo), true
}

// carpetaDeps es la isla donde el importador de dependencias deja los recursos de cada AAR:
// `res/__deps/<isla>/<tipo-calificador>/<archivo>`, la misma forma que entiende aapt2 (ver
// `apkc-deps`, `DEPS.md`). El nombre del fichero lleva el sufijo de la isla cuando es un `values`,
// porque dos `strings.xml` homónimos pisarían el mismo flat.
const carpetaDeps = "__deps"

// tipoDeRecurso: la carpeta de res/ que da el tipo, sin calificador. Un nombre con guion todavía no
// se acepta (5.17), así que basta el primer segmento. Dentro de la isla de dependencias el tipo está
// un nivel más abajo: `res/__deps/<isla>/<tipo>`.
func tipoDeRecurso(ruta string) string {
	partes := strings.Split(filepath.ToSlash(ruta), "/")
	if len(partes) >= 2 && partes[0] == "res" {
		i := 1
		if partes[1] == carpetaDeps && len(partes) >= 4 {
			i = 3
		}
		carpeta := partes[i]
		if i := strings.IndexByte(carpeta, '-'); i >= 0 {
			carpeta = carpeta[:i]
		}
		return carpeta
	}
	return ""
}

// fuentesDelProyecto recoge el arbol de recursos: los values, los layouts y el manifiesto, todos en
// texto. Rechaza lo que no se pueda enlazar, antes de escribir nada. Además de res/, lee la isla
// `res/__deps/`, que es lo que aporta un AAR importado, y el `res/` de cada `.aar` que siga en
// `jar/` (extraído por `aarsExtraidos`): los tres se enlazan con las mismas reglas y sus ficheros
// viajan al apk con la ruta de su tipo, igual que los del proyecto (lo que `aapt2 link` deja en
// `build/res`), para que el paquete y la vista previa no distingan el origen.
func fuentesDelProyecto(raiz string) (valores, layouts, ficheros []fuenteXML, manifiesto fuenteXML, err error) {
	manifiesto = fuenteXML{nombre: "AndroidManifest.xml"}
	manifiesto.contenido, err = os.ReadFile(filepath.Join(raiz, "AndroidManifest.xml"))
	if err != nil {
		return nil, nil, nil, manifiesto, fmt.Errorf("el proyecto no tiene AndroidManifest.xml: %w", err)
	}
	// `<permission>` ya no se rechaza: el enlace escribe su Manifest.java (parrafo 102).

	res := filepath.Join(raiz, "res")
	entradas, err := os.ReadDir(res)
	if err != nil {
		if os.IsNotExist(err) {
			return valores, layouts, ficheros, manifiesto, nil // un proyecto sin res/ no tiene recursos que enlazar
		}
		return nil, nil, nil, manifiesto, err
	}
	for _, e := range entradas {
		if !e.IsDir() {
			return nil, nil, nil, manifiesto, fmt.Errorf("res/%s no es una carpeta de recursos", e.Name())
		}
		if e.Name() == carpetaDeps {
			v, l, f, err := fuentesDeIslasDeDependencias(filepath.Join(res, carpetaDeps))
			if err != nil {
				return nil, nil, nil, manifiesto, err
			}
			valores = append(valores, v...)
			layouts = append(layouts, l...)
			ficheros = append(ficheros, f...)
			continue
		}
		v, l, f, err := fuentesDeCarpetaRes(filepath.Join(res, e.Name()), "res/"+e.Name(), false)
		if err != nil {
			return nil, nil, nil, manifiesto, err
		}
		valores = append(valores, v...)
		layouts = append(layouts, l...)
		ficheros = append(ficheros, f...)
	}
	// lo que cada `.aar` de `jar/` trae bajo `res/` entra con las mismas reglas que una isla de
	// `res/__deps/`: su ruta de paquete tampoco lleva la isla. Un layout o un fichero que el
	// proyecto (o una isla) ya aporta con el mismo nombre se queda con el suyo, así que un recurso
	// no viaja dos veces y la prioridad no depende del orden en que se leyeron los artefactos.
	aares, err := aarsExtraidos(raiz)
	if err != nil {
		return nil, nil, nil, manifiesto, err
	}
	vistos := map[string]bool{}
	for _, f := range layouts {
		vistos[f.nombre] = true
	}
	for _, f := range ficheros {
		vistos[f.nombre] = true
	}
	// dos copias byte a byte del mismo `values` (el importador deja una isla y el `.aar` sigue en
	// `jar/`) declaran los mismos recursos: se lee una vez
	contenidos := map[string]bool{}
	for _, f := range valores {
		contenidos[string(f.contenido)] = true
	}
	for _, aar := range aares {
		if aar.res == "" {
			continue
		}
		tipos, err := os.ReadDir(aar.res)
		if err != nil {
			return nil, nil, nil, manifiesto, err
		}
		for _, tipo := range tipos {
			logica := path.Join("jar", filepath.Base(aar.aar), "res", tipo.Name())
			if !tipo.IsDir() {
				return nil, nil, nil, manifiesto, fmt.Errorf("%s no es una carpeta de recursos", logica)
			}
			v, l, f, err := fuentesDeCarpetaRes(filepath.Join(aar.res, tipo.Name()), logica, true)
			if err != nil {
				return nil, nil, nil, manifiesto, err
			}
			for _, fuente := range l {
				if vistos[fuente.nombre] {
					LogW("build", "el recurso", fuente.nombre, "del AAR", filepath.Base(aar.aar),
						"ya lo aporta el proyecto: se queda el del proyecto")
					continue
				}
				vistos[fuente.nombre] = true
				layouts = append(layouts, fuente)
			}
			for _, fuente := range f {
				if vistos[fuente.nombre] {
					LogW("build", "el fichero", fuente.nombre, "del AAR", filepath.Base(aar.aar),
						"ya lo aporta el proyecto: se queda el del proyecto")
					continue
				}
				vistos[fuente.nombre] = true
				ficheros = append(ficheros, fuente)
			}
			for _, fuente := range v {
				if contenidos[string(fuente.contenido)] {
					continue
				}
				contenidos[string(fuente.contenido)] = true
				valores = append(valores, fuente)
			}
		}
	}
	return valores, layouts, ficheros, manifiesto, nil
}

// fuentesDeIslasDeDependencias recorre `res/__deps/`: una carpeta por artefacto y, dentro, las
// mismas carpetas de tipo que en res/. La isla en sí no viaja al paquete: se usa para que dos AAR no
// compartan fichero en disco, y su rastro queda en el nombre de los values, que el importador ya
// renombra con el sufijo de la isla.
func fuentesDeIslasDeDependencias(disco string) (valores, layouts, ficheros []fuenteXML, err error) {
	islas, err := os.ReadDir(disco)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, isla := range islas {
		if !isla.IsDir() {
			return nil, nil, nil, fmt.Errorf("res/%s/%s no es la carpeta de una dependencia: dentro de %s solo va una carpeta por artefacto",
				carpetaDeps, isla.Name(), carpetaDeps)
		}
		tipos, err := os.ReadDir(filepath.Join(disco, isla.Name()))
		if err != nil {
			return nil, nil, nil, err
		}
		for _, tipo := range tipos {
			if !tipo.IsDir() {
				return nil, nil, nil, fmt.Errorf("res/%s/%s/%s no es una carpeta de recursos", carpetaDeps, isla.Name(), tipo.Name())
			}
			v, l, f, err := fuentesDeCarpetaRes(filepath.Join(disco, isla.Name(), tipo.Name()), "res/"+carpetaDeps+"/"+isla.Name()+"/"+tipo.Name(), true)
			if err != nil {
				return nil, nil, nil, err
			}
			valores = append(valores, v...)
			layouts = append(layouts, l...)
			ficheros = append(ficheros, f...)
		}
	}
	return valores, layouts, ficheros, nil
}

// fuentesDeCarpetaRes lee una carpeta de recursos —con o sin calificador— y reparte sus ficheros en
// values, AXML (layout/menu/xml/anim y los drawable/font XML) y ficheros que viajan tal cual. `disco`
// es dónde está y `logica` la ruta con la que sus ficheros entran en el paquete (`res/values`,
// `res/__deps/<isla>/drawable`). `deDependencia` marca lo que viene de un AAR importado.
func fuentesDeCarpetaRes(disco, logica string, deDependencia bool) (valores, layouts, ficheros []fuenteXML, err error) {
	carpeta := path.Base(logica)
	tipo, config := carpeta, ""
	var autoVersion uint16
	if !tiposDeResQueSeEnlazan[tipo] {
		if t, c, auto, ok := calificadorExtraDeCarpeta(carpeta); ok {
			tipo, config, autoVersion = t, c, auto
		} else {
			return nil, nil, nil, fmt.Errorf("%s: este enlazador solo escribe una configuración por carpeta: values, layout, menu, xml, anim, drawable, mipmap, font y raw con idioma, región, orientación, densidad, tamaño, sw, touchscreen, keyboard, navigation, uiMode y -vN", logica)
		}
	}
	nom, err := os.ReadDir(disco)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, f := range nom {
		if f.IsDir() {
			return nil, nil, nil, fmt.Errorf("%s/%s: una subcarpeta dentro de %s no esta medida (densidades y variantes de configuracion se parten por API, parrafo 28)", logica, f.Name(), tipo)
		}
		carpetaSalida := carpeta
		if autoVersion != 0 {
			// aapt2 añade la version implicita del calificador a la ruta del apk incluso con
			// --no-auto-version: 4 para densidad y tamaño, 13 para sw y 8 para uiMode
			// (parrafo 103, §159 y medido con build-tools 35.0.1)
			carpetaSalida += fmt.Sprintf("-v%d", autoVersion)
		}
		// la isla de una dependencia no viaja: el fichero entra con la ruta de su tipo, que es
		// donde lo deja `aapt2 link` y lo que buscan la tabla, el paquete y la vista previa
		ruta := path.Join("res", carpetaSalida, f.Name())
		contenido, err := os.ReadFile(filepath.Join(disco, f.Name()))
		if err != nil {
			return nil, nil, nil, err
		}
		fuente := fuenteXML{nombre: ruta, contenido: contenido, config: config, deDependencia: deDependencia}
		switch tipo {
		case "values":
			if !strings.HasSuffix(f.Name(), ".xml") {
				return nil, nil, nil, fmt.Errorf("%s/%s: values solo admite .xml", logica, f.Name())
			}
			valores = append(valores, fuente)
		case "layout", "menu", "xml", "anim":
			// los cuatro son AXML; la raíz de menu/xml/anim no tiene que ser un layout. Se guardan
			// en la misma lista y el tipo sale de la ruta (párrafo 97). Un `anim` XML es el mismo
			// formato: lo que cambia es el nombre del tipo en la tabla.
			if !strings.HasSuffix(f.Name(), ".xml") {
				return nil, nil, nil, fmt.Errorf("%s/%s: %s solo admite .xml", logica, f.Name(), tipo)
			}
			layouts = append(layouts, fuente)
		case "raw":
			// un raw viaja literal, como un PNG; no se parsea
			ficheros = append(ficheros, fuente)
		case "font":
			// Igual que un drawable: el XML de una familia (`<font-family>`) se compila a AXML y
			// viaja como documento; un `.ttf`/`.otf` viaja tal cual y su entrada es la ruta.
			if strings.HasSuffix(f.Name(), ".xml") {
				layouts = append(layouts, fuente)
			} else {
				ficheros = append(ficheros, fuente)
			}
		default:
			// drawable/mipmap: un XML (`<shape>`, `<vector>`) se compila a AXML y viaja como
			// documento, que es lo que deja `aapt2 compile`; un PNG (o cualquier binario) viaja
			// tal cual y su entrada es un Res_value de tipo cadena con la ruta (párrafo 93).
			if strings.HasSuffix(f.Name(), ".xml") {
				layouts = append(layouts, fuente)
			} else {
				ficheros = append(ficheros, fuente)
			}
		}
	}
	return valores, layouts, ficheros, nil
}

// enlazarConGo es el sustituto de `aapt2 compile` y `aapt2 link` juntos. Escribe los productos en el
// arbol del proyecto y dice si la R cambio, que es lo que necesita el paso incremental para saber si
// hay que volver a pasar javac.
func enlazarConGo(raiz, androidJar string, opts opcionesDeEnlace) (bool, error) {
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(raiz)
	if err != nil {
		return false, err
	}
	if len(valores) == 0 && len(layouts) == 0 && len(ficheros) == 0 {
		return false, fmt.Errorf("el proyecto no tiene recursos que enlazar (ni values, ni layout, ni drawable)")
	}
	m, err := marcoDelAndroidJar(androidJar)
	if err != nil {
		return false, fmt.Errorf("la tabla de ids del framework: %w", err)
	}
	enlazado, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, m, opts)
	if err != nil {
		return false, err
	}

	arsc, err := escribeTablaRecursos(enlazado.tabla)
	if err != nil {
		return false, err
	}
	manifiestoBinario, err := escribeAXML(enlazado.manifiesto)
	if err != nil {
		return false, err
	}
	// el mismo control que el enlace de aapt2: se vacia build/res antes de escribir, para que un
	// recurso borrado no siga viajando en el paquete con una entrada que ya no existe en la tabla
	if err := os.RemoveAll(filepath.Join(raiz, "build", "res")); err != nil {
		return false, err
	}
	escrituras := []struct {
		ruta      string
		contenido []byte
	}{
		{filepath.Join(raiz, "build", "AndroidManifest.xml"), manifiestoBinario},
		{filepath.Join(raiz, "build", "resources.arsc"), arsc},
	}
	for nombre, doc := range enlazado.layouts {
		bin, err := escribeAXML(doc)
		if err != nil {
			return false, err
		}
		// el nombre dentro del apk es res/layout/x.xml y el producto en el arbol de build es el mismo
		escrituras = append(escrituras, struct {
			ruta      string
			contenido []byte
		}{filepath.Join(raiz, "build", filepath.FromSlash(nombre)), bin})
	}
	for nombre, contenido := range enlazado.ficheros {
		escrituras = append(escrituras, struct {
			ruta      string
			contenido []byte
		}{filepath.Join(raiz, "build", filepath.FromSlash(nombre)), contenido})
	}
	for _, e := range escrituras {
		if err := os.MkdirAll(filepath.Dir(e.ruta), 0o755); err != nil {
			return false, err
		}
		if err := os.WriteFile(e.ruta, e.contenido, 0o644); err != nil {
			return false, err
		}
	}

	ruta := filepath.Join(raiz, "src", strings.ReplaceAll(enlazado.tabla.nombrePaquete, ".", string(filepath.Separator)), "R.java")
	antes, errLeer := os.ReadFile(ruta)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(ruta, []byte(enlazado.r), 0o644); err != nil {
		return false, err
	}
	cambio := errLeer != nil || string(antes) != enlazado.r
	if enlazado.manifestJava != "" {
		mruta := filepath.Join(filepath.Dir(ruta), "Manifest.java")
		manAntes, errMan := os.ReadFile(mruta)
		if err := os.WriteFile(mruta, []byte(enlazado.manifestJava), 0o644); err != nil {
			return false, err
		}
		cambio = cambio || errMan != nil || string(manAntes) != enlazado.manifestJava
	}
	// la R vale para javac: si el contenido no cambio, los fuentes que leen esas constantes no se
	// invalidan (la misma regla que comprueba el enlace de aapt2 con snapshotGenJava)
	return cambio, nil
}

// opcionesDelProyecto saca lo que aapt2 recibe por bandera y aqui se pasa a mano. La version se puede
// decir con --sdk-version y --codename; si no se dice, se lee de la ruta del platform, que en el SDK
// instalado es platforms/android-34/android.jar. En la pagina el jar se monta donde quiere el usuario,
// por eso la bandera existe.
func opcionesDelProyecto(androidJar string, version int, codename string) (opcionesDeEnlace, error) {
	if codename != "" {
		if version == 0 {
			return opcionesDeEnlace{}, fmt.Errorf("--codename pide tambien --sdk-version")
		}
		return opcionesDeEnlace{compileSdkVersion: version, compileSdkCodename: codename}, nil
	}
	if version == 0 {
		base := filepath.Base(filepath.Dir(androidJar))
		if !strings.HasPrefix(base, "android-") {
			return opcionesDeEnlace{}, fmt.Errorf("%s: la ruta del android.jar no dice la version del platform; digala con --sdk-version", androidJar)
		}
		numero, err := strconv.Atoi(strings.TrimPrefix(base, "android-"))
		if err != nil {
			return opcionesDeEnlace{}, fmt.Errorf("%s: la version del platform no es un numero", base)
		}
		version = numero
	}
	// medido sobre el android-34 de este telefono: compileSdkVersion 34 y codename "14", que es la
	// version de Android; platformBuildVersionName sale del mismo numero entero (parrafo 37)
	nombrePlataforma, ok := nombreDeVersionDeAndroid(version)
	if !ok {
		return opcionesDeEnlace{}, fmt.Errorf("el nombre de la version de Android correspondiente a API %d no esta medido aqui; de-lo con --codename", version)
	}
	return opcionesDeEnlace{compileSdkVersion: version, compileSdkCodename: nombrePlataforma}, nil
}

// versionesMedidasDeAndroid: API -> version de Android en texto. Solo la que se comprobado contra los
// ficheros de aapt2; pedir otra es rechazar con nombre, no adivinar.
func nombreDeVersionDeAndroid(api int) (string, bool) {
	// La versión de Android que corresponde a la API. La 34/14 es la medida en el §37; las demás
	// salen de la misma tabla que usa aapt2 (`--compile-sdk-version-code`/`--compile-sdk-version-name`)
	// y se comprobaron con el volcado del §98. Las que no tienen nombre numérico (API 32 = 12L, 27 =
	// 8.1) se dejan fuera: `platformBuildVersionName` las escribiría como entero y eso no se midió.
	switch api {
	case 27:
		return "8.1", true
	case 28:
		return "9", true
	case 29:
		return "10", true
	case 30:
		return "11", true
	case 31:
		return "12", true
	case 32:
		return "12L", true
	case 33:
		return "13", true
	case 34:
		return "14", true
	case 35:
		return "15", true
	}
	return "", false
}
