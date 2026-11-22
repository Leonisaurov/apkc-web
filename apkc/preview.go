package main

// La Fase 1 de la vista previa: leer un APK ya construido y describir su primera pantalla con un
// modelo JSON que la pestaña pueda pintar. Es el camino inverso de axmlwriter.go y arscwriter.go
// sobre los lectores que ya existían en recursos.go (leeAXML y leeTabla), más la resolución de
// referencias que hasta ahora no hacía falta: un `@+id/total` del layout es un id de recurso y su
// nombre está en la tabla.
//
// Lo que esta fase NO hace, dicho para que no se lea como un fallo:
//   - El layout se elige por nombre (por defecto el que se llama `main`) salvo que la app lo diga:
//     desde la Fase 2 el intérprete de previewinterprete.go lee el id del `setContentView` y manda
//     sobre el nombre. Sin `classes.dex`, el criterio sigue siendo el nombre.
//   - El rectángulo es una colocación declarada, no una medida del framework: se aplican las reglas
//     de LinearLayout (orientación, gravity, layout_gravity, layout_weight, padding y márgenes) y el
//     apilado de FrameLayout, con el tamaño del texto estimado (el dinámico si lo hay). Estilos
//     fieles, imágenes, líneas de texto partidas, RelativeLayout y el tema del sistema quedan fuera.
//   - Un dex cuya ejecución se sale de sus propios bytes (un payload de switch que no cabe, un
//     registro fuera del marco) no se ejecuta a medias: el comando lo rechaza con error y código 2.
//     Lo que sí sigue saliendo es el modelo de un apk cuya app llama a una API fuera del subconjunto:
//     ahí el intérprete deja avisos y el `ejecucion.error` correspondiente, sin romper la vista.
//   - Solo se lee una configuración por tipo: la base. `--layout` recibe la clave de un recurso,
//     no un calificador, así que un `layout-land` con la misma clave que la base no se puede pedir
//     aparte: se toma la primera entrada que la tabla traiga para esa clave.
//
// Todo el código es propio (MIT). No hay ninguna petición de red ni proceso: el comando lee bytes
// del zip con archive/zip, que ya usa el resto del fork para firmar y verificar.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Medidas de la aproximación, en dp. La pantalla es la de un teléfono corriente de 360x640 dp y el
// alto de cada view sale de su tipo, no de un tema: es la parte declarada como aproximada.
const (
	dpPantallaAncho   = 360
	dpPantallaAlto    = 640
	dpAltoTexto       = 24
	dpAltoBoton       = 48
	dpPorCaracter     = 8
	dpRellenoTexto    = 16
	profundidadMaxima = 8

	// la densidad con la que un `px` del layout se lleva a dp: la pantalla del modelo es mdpi
	// (360x640 dp a 160 dpi), que es la que declara esta aproximación
	dpiPantalla = 160

	// los dos valores de espera de layout_width/layout_height, con el signo que les da aapt2
	medidaMatch = -1
	medidaWrap  = -2

	// tipos de Res_value que la colocación necesita y que axmlwriter.go no nombra
	tipoFlotante  byte = 0x04
	tipoDimension byte = 0x05
)

// Los trozos de android:gravity que se aplican, con los valores de Gravity del framework: los tres
// bits bajos son la gravedad horizontal y los del medio la vertical (CENTER es 0x11, medido en el
// xmltree del layout del studio).
const (
	gravityCentroHorizontal  = 0x01
	gravityIzquierda         = 0x03
	gravityDerecha           = 0x05
	gravityRellenoHorizontal = 0x07
	gravityCentroVertical    = 0x10
	gravityArriba            = 0x30
	gravityAbajo             = 0x50
	gravityRellenoVertical   = 0x70
	gravityMascaraHorizontal = 0x07
	gravityMascaraVertical   = 0x70
	gravityBanderaDireccion  = 0x00800000
)

// rectVista es el rectángulo de un nodo, en dp y relativo a la pantalla.
type rectVista struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Ancho int `json:"ancho"`
	Alto  int `json:"alto"`
}

// ladosVista es un relleno o un margen: los cuatro lados en dp, en el orden en que se leen.
type ladosVista struct {
	Izq int `json:"izq"`
	Arr int `json:"arr"`
	Der int `json:"der"`
	Aba int `json:"aba"`
}

func (l ladosVista) horizontal() int { return l.Izq + l.Der }
func (l ladosVista) vertical() int   { return l.Arr + l.Aba }

// nodoVista es una view del modelo: su tipo, su identificador resuelto, el texto resuelto, lo que
// declara de tamaño y de colocación, el rectángulo y sus hijos. Los campos sin namespace no se
// serializan. `textoDinamico` es la Fase 2: lo que el intérprete vio escribir a la app en esa view,
// separado del `texto` que declara el layout para que el modelo no confunda lo declarado con lo
// ejecutado. `gravity`, `padding`, `margen` y `peso` son lo que la colocación verificada contra la
// cuenta documentada de AOSP/aapt2 necesita (sin oráculo de aparato), y lo que el pintor puede
// consumir sin volver a leer el AXML.
type nodoVista struct {
	Tipo          string `json:"tipo"`
	ID            string `json:"id,omitempty"`
	IDRef         string `json:"idRef,omitempty"`
	Texto         string `json:"texto,omitempty"`
	TextoDinamico string `json:"textoDinamico,omitempty"`
	// Oyente marca la vista que registró un OnClickListener: es la que responde a un clic en el
	// render y la única a la que se le puede pedir `--pulsar`.
	Oyente  bool         `json:"oyente,omitempty"`
	Estilo  string       `json:"estilo,omitempty"`
	Ancho   string       `json:"ancho,omitempty"`
	Alto    string       `json:"alto,omitempty"`
	Gravity string       `json:"gravity,omitempty"`
	Peso    float64      `json:"peso,omitempty"`
	Padding *ladosVista  `json:"padding,omitempty"`
	Margen  *ladosVista  `json:"margen,omitempty"`
	Rect    rectVista    `json:"rect"`
	Hijos   []*nodoVista `json:"hijos,omitempty"`

	orientacion string // "vertical", "horizontal" o "" (se apilan en el origen)

	declAncho     int        // medidaMatch, medidaWrap o el dp declarado
	declAlto      int        // lo mismo para el alto
	gravityBits   int32      // android:gravity de la view, ya como bits
	layoutGravity int32      // android:layout_gravity, o -1 cuando no se declara
	pesoSuma      float64    // android:weightSum del contenedor, o 0 cuando no se declara
	pad           ladosVista // el relleno efectivo, en dp
	margen        ladosVista // el margen efectivo, en dp
	medAncho      int        // el tamaño medido, que es el que ocupa en la colocación
	medAlto       int
}

// modeloVista es el JSON que sale por stdout. `aproximado` viaja en verdadero siempre: los
// rectángulos son una estimación y el consumidor no tiene que adivinarlo.
type modeloVista struct {
	Paquete    string              `json:"paquete"`
	Version    string              `json:"version,omitempty"`
	Actividad  string              `json:"actividad,omitempty"`
	Layout     string              `json:"layout"`
	Pantalla   rectVista           `json:"pantalla"`
	Aproximado bool                `json:"aproximado"`
	Nodos      int                 `json:"nodos"`
	Ejecucion  *resultadoEjecucion `json:"ejecucion,omitempty"`
	Raiz       *nodoVista          `json:"raiz"`
}

// elementoLeido es un elemento del árbol del AXML con sus atributos ya resueltos a cadenas del
// pool y el valor crudo de cada uno. El lector de recursos.go deja una lista plana de nodos; esto
// le devuelve la forma de árbol que necesitan el manifiesto y el layout.
type elementoLeido struct {
	ns     string
	nombre string
	attrs  []atributoLeido
	hijos  []*elementoLeido
}

type atributoLeido struct {
	ns     string
	nombre string
	valor  valor
}

// contextoVista reúne lo que hace falta para resolver: el AXML del fichero que se está leyendo y la
// tabla de recursos del paquete. Sin tabla (un apk sin resources.arsc) las referencias se quedan
// escritas como @0x…, que es lo que son.
type contextoVista struct {
	axml  *axml
	tabla *tablaRecursos
}

func preview() {
	set := flag.NewFlagSet("preview", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: apkc preview [--layout nombre] [--sin-ejecutar] [--pulsar id]… app.apk

Read one apk and print, as one line of JSON on stdout, the model of its first screen: type, id,
text, children and an approximate rectangle for every node. The layout is chosen by name (main by
default) unless the app's onCreate calls setContentView with a resource id, which the Dalvik
interpreter of Phase 2 reads from classes.dex and follows. The interpreter runs onCreate and stops
there: it does NOT click anything on its own. An android:onClick="metodo" declared in the layout is
bound before any click, as the framework does on inflate, so --pulsar reaches it. Each --pulsar id runs the OnClickListener registered
on that view, in the order given and inside the same run (id/pulsar, 0x7f010000 or its decimal); the
text the app writes (TextView.setText) comes back in ejecucion.capturas and in each node's
textoDinamico, and the app's own output (print, println, Log.*) in ejecucion.consola. Repeating the
same list reproduces the same screen. No process is started and nothing is written, so it runs
inside the tab.
`)
	}
	layout := set.String("layout", "", "nombre del recurso layout que se lee (por defecto main)")
	sinEjecutar := set.Bool("sin-ejecutar", false, "no interpreta classes.dex: solo el modelo del layout")
	var pulsos listaRepetida
	set.Var(&pulsos, "pulsar", "id de la vista cuyo OnClickListener se ejecuta (repetible, en orden)")
	if err := set.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	rutas := set.Args()
	if len(rutas) != 1 {
		fmt.Fprintln(os.Stderr, "preview: se espera un apk")
		os.Exit(2)
	}
	if *sinEjecutar && len(pulsos) > 0 {
		fmt.Fprintln(os.Stderr, "preview: --pulsar necesita ejecutar el dex; quita --sin-ejecutar")
		os.Exit(2)
	}
	datos, err := os.ReadFile(rutas[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "preview:", err)
		os.Exit(2)
	}
	modelo, err := modeloVistaDeAPKConPulsos(datos, *layout, !*sinEjecutar, pulsos)
	if err != nil {
		fmt.Fprintln(os.Stderr, "preview:", err)
		os.Exit(2)
	}
	salida, err := json.Marshal(modelo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "preview:", err)
		os.Exit(2)
	}
	fmt.Println(string(salida))
}

// listaRepetida es un flag que acumula cada aparición: `--pulsar a --pulsar b` pide dos clics en ese
// orden, que es lo que hace reproducible una pantalla tras varios toques.
type listaRepetida []string

func (l *listaRepetida) String() string { return strings.Join(*l, ",") }

func (l *listaRepetida) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("--pulsar necesita un id: id/pulsar, 0x7f010000 o su decimal")
	}
	*l = append(*l, v)
	return nil
}

// modeloVistaDeAPK es el cuerpo del comando con el intérprete de la Fase 2 activo. Se queda como
// envoltorio para que las pruebas de la Fase 1 no cambien de forma de llamada.
func modeloVistaDeAPK(datos []byte, layout string) (*modeloVista, error) {
	return modeloVistaDeAPKCon(datos, layout, true)
}

// modeloVistaDeAPKCon es el cuerpo del comando, separado de los argumentos para poder probarlo con
// los bytes de un apk sin pasar por el sistema de ficheros. `ejecutar` decide si se interpreta
// classes.dex; sin él, el modelo es exactamente el de la Fase 1. No pulsa nada: el emulador arranca
// en el estado de `onCreate` y espera un clic.
func modeloVistaDeAPKCon(datos []byte, layout string, ejecutar bool) (*modeloVista, error) {
	return modeloVistaDeAPKConPulsos(datos, layout, ejecutar, nil)
}

// modeloVistaDeAPKConPulsos es el mismo cuerpo con la lista de clics de `--pulsar`, que se entrega en
// orden dentro de la misma ejecución. Un clic que no se puede entregar devuelve error: un modelo con
// la pantalla a medias se leería como si el clic hubiera funcionado.
func modeloVistaDeAPKConPulsos(datos []byte, layout string, ejecutar bool, pulsos []string) (*modeloVista, error) {
	zr, err := zip.NewReader(bytes.NewReader(datos), int64(len(datos)))
	if err != nil {
		return nil, fmt.Errorf("el apk no es un zip: %w", err)
	}
	manifiestoBytes, err := entradaDeZip(zr, "AndroidManifest.xml")
	if err != nil {
		return nil, fmt.Errorf("el AndroidManifest.xml: %w", err)
	}
	axmlManifiesto, err := leeAXML(manifiestoBytes)
	if err != nil {
		return nil, fmt.Errorf("el AndroidManifest.xml: %w", err)
	}

	// la tabla es opcional: sin ella se lee el layout igual, con las referencias sin resolver
	var tabla *tablaRecursos
	if arsc, err := entradaDeZip(zr, "resources.arsc"); err == nil {
		if tabla, err = leeTabla(arsc); err != nil {
			return nil, fmt.Errorf("el resources.arsc: %w", err)
		}
	}

	ctxManifiesto := &contextoVista{axml: axmlManifiesto, tabla: tabla}
	arbolManifiesto, err := ctxManifiesto.arbol()
	if err != nil {
		return nil, fmt.Errorf("el AndroidManifest.xml: %w", err)
	}
	modelo := &modeloVista{
		Aproximado: true,
		Pantalla:   rectVista{Ancho: dpPantallaAncho, Alto: dpPantallaAlto},
	}
	nombreActividad := ""
	if paquete, ok := arbolManifiesto.atributo("", "package"); ok {
		modelo.Paquete = ctxManifiesto.texto(paquete)
	}
	if version, ok := arbolManifiesto.atributo(uriDeRecursos, "versionName"); ok {
		modelo.Version = ctxManifiesto.texto(version)
	}
	if actividad, ok := actividadLanzadora(arbolManifiesto, ctxManifiesto); ok {
		nombreActividad = ctxManifiesto.textoDeAtributo(actividad, uriDeRecursos, "name")
		modelo.Actividad = nombreActividad
	}

	// Fase 2: la actividad se ejecuta antes de elegir el layout, porque el `setContentView` dice cuál
	// es. Si no hay classes.dex o el dex no se deja ejecutar, el modelo de la Fase 1 sigue saliendo.
	var layoutID uint32
	if ejecutar {
		if dexBytes, err := entradaDeZip(zr, "classes.dex"); err == nil {
			if d, err := readDex(dexBytes); err == nil {
				preferida := descriptorDeActividad(modelo.Paquete, nombreActividad)
				var errPulsos error
				modelo.Ejecucion, layoutID, errPulsos = ejecutaOnCreatePreparando(d, tabla, preferida, pulsos,
					func(m *maquinaDalvik) { enlazaOnClicksDelLayout(m, zr, tabla, layout) })
				if errPulsos != nil {
					return nil, errPulsos
				}
				// un dex mal formado no se ejecuta a medias ni se tapa con un modelo que parecería
				// medido: el comando lo rechaza. Una API fuera del subconjunto, en cambio, solo deja
				// avisos y el modelo sale igual.
				if modelo.Ejecucion != nil && modelo.Ejecucion.malformado {
					return nil, fmt.Errorf("el classes.dex no es ejecutable: %s", modelo.Ejecucion.Error)
				}
			} else {
				modelo.Ejecucion = &resultadoEjecucion{Error: "el classes.dex no se deja leer: " + err.Error()}
			}
		}
	}

	rutaLayout, err := rutaDelLayoutDeLaActividad(tabla, zr, layout, layoutID)
	if err != nil {
		return nil, err
	}
	layoutBytes, err := entradaDeZip(zr, rutaLayout)
	if err != nil {
		return nil, fmt.Errorf("el layout %s: %w", rutaLayout, err)
	}
	axmlLayout, err := leeAXML(layoutBytes)
	if err != nil {
		return nil, fmt.Errorf("el layout %s: %w", rutaLayout, err)
	}
	ctxLayout := &contextoVista{axml: axmlLayout, tabla: tabla}
	arbolLayout, err := ctxLayout.arbol()
	if err != nil {
		return nil, fmt.Errorf("el layout %s: %w", rutaLayout, err)
	}
	raiz := ctxLayout.nodoDe(arbolLayout)
	if raiz == nil {
		return nil, fmt.Errorf("el layout %s no tiene raíz", rutaLayout)
	}
	coloca(raiz, 0, 0, dpPantallaAncho, dpPantallaAlto)
	anotaTextoDinamico(raiz, modelo.Ejecucion)
	anotaOyentes(raiz, modelo.Ejecucion)
	// el rectángulo se calcula con el texto que se va a ver: si la app lo cambió, la medida del
	// layout declarado ya no es la de la pantalla que el modelo describe
	coloca(raiz, 0, 0, dpPantallaAncho, dpPantallaAlto)
	modelo.Layout = rutaLayout
	modelo.Raiz = raiz
	modelo.Nodos = contarNodos(raiz)
	return modelo, nil
}

// descriptorDeClase lleva el nombre del manifiesto (`.MainActivity` o `com.x.MainActivity`) al
// descriptor de un type_id del dex.
func descriptorDeActividad(paquete, nombre string) string {
	if nombre == "" {
		return ""
	}
	if strings.HasPrefix(nombre, ".") {
		nombre = paquete + nombre
	} else if !strings.Contains(nombre, ".") {
		nombre = paquete + "." + nombre
	}
	return "L" + strings.ReplaceAll(nombre, ".", "/") + ";"
}

// enlazaOnClicksDelLayout recorre el layout que la app pidió con `setContentView` y registra cada
// `android:onClick`: el inflado del framework resuelve el nombre contra el contexto de la vista (la
// actividad) y deja el listener puesto. Sin esto, una app que declara el clic en el XML —el patrón
// de los contadores de tutorial, `android:onClick="sumar"`— se ejecutaba con `oyentes` vacío y el
// emulador no tenía a quién pulsar, aunque la app funcionara en el aparato.
func enlazaOnClicksDelLayout(m *maquinaDalvik, zr *zip.Reader, tabla *tablaRecursos, layout string) {
	if m == nil {
		return
	}
	ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, layout, m.layoutID)
	if err != nil {
		return
	}
	datos, err := entradaDeZip(zr, ruta)
	if err != nil {
		return
	}
	axml, err := leeAXML(datos)
	if err != nil {
		return
	}
	ctx := &contextoVista{axml: axml, tabla: tabla}
	raiz, err := ctx.arbol()
	if err != nil {
		return
	}
	var recorre func(*elementoLeido)
	recorre = func(e *elementoLeido) {
		// los hijos primero: el orden en que se enlazan no cambia el resultado, pero deja el registro
		// con las vistas de dentro antes que su contenedor, como las recorre el inflado
		for _, h := range e.hijos {
			recorre(h)
		}
		valor, ok := e.atributo(uriDeRecursos, "onClick")
		if !ok {
			return
		}
		metodo := strings.TrimSpace(ctx.texto(valor))
		if metodo == "" {
			return
		}
		id, ok := e.atributo(uriDeRecursos, "id")
		if !ok || id.datos == 0 {
			m.avisa("android:onClick=%q en un %s sin android:id: un clic no puede llegar a una vista sin id",
				metodo, e.nombre)
			return
		}
		if motivo := m.registraOnClickDeclarativo(id.datos, metodo); motivo != "" {
			m.avisa("%s", motivo)
		}
	}
	recorre(raiz)
}

// anotaTextoDinamico copia a cada nodo la última escritura que el intérprete capturó sobre su id.
// El match es por el id de recurso, que es lo que comparten el layout y el `findViewById`.
func anotaTextoDinamico(n *nodoVista, ejecucion *resultadoEjecucion) {
	if n == nil || ejecucion == nil {
		return
	}
	for _, c := range ejecucion.Capturas {
		if c.IDRef == "" || c.IDRef != n.IDRef || c.Origen != "TextView.setText" {
			continue
		}
		n.TextoDinamico = c.Texto
	}
	for _, h := range n.Hijos {
		anotaTextoDinamico(h, ejecucion)
	}
}

// anotaOyentes marca en el árbol las vistas que responden a un clic. El emparejamiento es por id de
// recurso: es lo que comparten el layout y el `findViewById` del `OnClickListener`.
func anotaOyentes(n *nodoVista, ejecucion *resultadoEjecucion) {
	if n == nil || ejecucion == nil {
		return
	}
	for _, o := range ejecucion.Oyentes {
		if o.IDRef != "" && o.IDRef == n.IDRef {
			n.Oyente = true
			break
		}
	}
	for _, h := range n.Hijos {
		anotaOyentes(h, ejecucion)
	}
}

// rutaDelLayoutDeLaActividad elige el layout: `--layout` manda si se pidió uno, y si no, el id que
// el intérprete vio pasar por `setContentView`. Cuando ese id no se puede resolver (sin tabla, o una
// entrada compleja) se cae al criterio de la Fase 1, por nombre, en vez de fallar el comando.
func rutaDelLayoutDeLaActividad(tabla *tablaRecursos, zr *zip.Reader, pedido string, layoutID uint32) (string, error) {
	if pedido == "" && layoutID != 0 && tabla != nil {
		if ruta, err := rutaDeLayoutPorId(tabla, layoutID); err == nil {
			return ruta, nil
		}
	}
	return rutaDelLayout(tabla, zr, pedido)
}

// rutaDeLayoutPorId resuelve el id de un `R.layout.*` a la entrada del apk. La tabla guarda la ruta
// como cadena del pool global; si el enlace solo dejó la clave, se arma con la forma que usa aapt2.
func rutaDeLayoutPorId(tabla *tablaRecursos, id uint32) (string, error) {
	bl, e, ok := tabla.entradaDeId(id)
	if !ok || e.ausente || e.compleja || bl.nombre != "layout" {
		return "", fmt.Errorf("%#08x no es una entrada de layout con valor simple", id)
	}
	if e.valor.tipo == tipoCadena && int(e.valor.datos) < len(tabla.global.cadenas) {
		return tabla.global.cadenas[e.valor.datos], nil
	}
	return "res/layout/" + tabla.clave(e) + ".xml", nil
}

// entradaDeZip devuelve el contenido descomprimido de la entrada con ese nombre exacto.
func entradaDeZip(zr *zip.Reader, nombre string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != nombre {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		buf := &bytes.Buffer{}
		if _, err := buf.ReadFrom(rc); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return nil, fmt.Errorf("el apk no lleva %s", nombre)
}

// rutaDelLayout elige el fichero del layout: el que se pida por nombre, si no `main`, y si no el
// primero de la tabla por orden alfabético. La entrada del tipo `layout` de la tabla guarda la ruta
// dentro del apk como cadena del pool global, que es lo que se devuelve.
func rutaDelLayout(tabla *tablaRecursos, zr *zip.Reader, pedido string) (string, error) {
	type entrada struct {
		clave string
		ruta  string
	}
	encontradas := []entrada{}
	if tabla != nil {
		for _, bl := range tabla.bloques {
			if bl.nombre != "layout" {
				continue
			}
			for i := range bl.entradas {
				e := bl.entradas[i]
				if e.ausente || e.compleja {
					continue
				}
				clave := tabla.clave(e)
				ruta := ""
				if e.valor.tipo == tipoCadena && int(e.valor.datos) < len(tabla.global.cadenas) {
					ruta = tabla.global.cadenas[e.valor.datos]
				} else if esConfiguracionBase(bl.config) {
					ruta = "res/layout/" + clave + ".xml"
				}
				if ruta == "" {
					continue
				}
				encontradas = append(encontradas, entrada{clave: clave, ruta: ruta})
			}
		}
	}
	// sin tabla: lo que haya en el zip con la forma res/layout/<nombre>.xml
	if len(encontradas) == 0 {
		nombres := []string{}
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "res/layout/") && strings.HasSuffix(f.Name, ".xml") {
				nombres = append(nombres, f.Name)
			}
		}
		sort.Strings(nombres)
		if len(nombres) == 0 {
			return "", fmt.Errorf("el apk no lleva ningún res/layout/*.xml")
		}
		if pedido == "" {
			for _, n := range nombres {
				if n == "res/layout/main.xml" {
					return n, nil
				}
			}
			return nombres[0], nil
		}
		for _, n := range nombres {
			if n == "res/layout/"+pedido+".xml" {
				return n, nil
			}
		}
		return "", fmt.Errorf("el apk no lleva el layout %q", pedido)
	}
	sort.Slice(encontradas, func(i, j int) bool { return encontradas[i].clave < encontradas[j].clave })
	if pedido != "" {
		for _, e := range encontradas {
			if e.clave == pedido {
				return e.ruta, nil
			}
		}
		return "", fmt.Errorf("la tabla no tiene el layout %q", pedido)
	}
	for _, e := range encontradas {
		if e.clave == "main" {
			return e.ruta, nil
		}
	}
	return encontradas[0].ruta, nil
}

// actividadLanzadora busca la `activity` con el filtro MAIN+LAUNCHER; si ninguna lo lleva, la
// primera, que es lo que haría un `am start` sin nombre.
func actividadLanzadora(arbol *elementoLeido, ctx *contextoVista) (*elementoLeido, bool) {
	app := arbol.hijo("application")
	if app == nil {
		return nil, false
	}
	var primera *elementoLeido
	for _, h := range app.hijos {
		if h.nombre != "activity" && h.nombre != "activity-alias" {
			continue
		}
		if primera == nil {
			primera = h
		}
		if esLanzadora(h, ctx) {
			return h, true
		}
	}
	if primera != nil {
		return primera, true
	}
	return nil, false
}

func esLanzadora(actividad *elementoLeido, ctx *contextoVista) bool {
	for _, filtro := range actividad.hijos {
		if filtro.nombre != "intent-filter" {
			continue
		}
		main, launcher := false, false
		for _, h := range filtro.hijos {
			switch h.nombre {
			case "action":
				if ctx.textoDeAtributo(h, uriDeRecursos, "name") == "android.intent.action.MAIN" {
					main = true
				}
			case "category":
				if ctx.textoDeAtributo(h, uriDeRecursos, "name") == "android.intent.category.LAUNCHER" {
					launcher = true
				}
			}
		}
		if main && launcher {
			return true
		}
	}
	return false
}

// arbol arma el árbol a partir de la lista plana de nodos del lector. Los trozos de namespace no
// dejan nodo: su URI ya viaja en el campo ns de cada atributo.
func (c *contextoVista) arbol() (*elementoLeido, error) {
	var raiz *elementoLeido
	pila := []*elementoLeido{}
	for _, n := range c.axml.nodos {
		switch n.tipo {
		case trozoInicioEtiqueta:
			e := &elementoLeido{ns: c.uri(n.ns), nombre: c.axml.nombreDe(n.nombre)}
			for _, at := range n.attrs {
				e.attrs = append(e.attrs, atributoLeido{
					ns:     c.uri(at.ns),
					nombre: c.axml.nombreDe(at.nombre),
					valor:  at.valor,
				})
			}
			if len(pila) == 0 {
				if raiz != nil {
					return nil, fmt.Errorf("dos raíces")
				}
				raiz = e
			} else {
				pila[len(pila)-1].hijos = append(pila[len(pila)-1].hijos, e)
			}
			pila = append(pila, e)
		case trozoFinEtik:
			if len(pila) == 0 {
				return nil, fmt.Errorf("cierre de %q sin apertura", c.axml.nombreDe(n.nombre))
			}
			ultimo := pila[len(pila)-1]
			if ultimo.nombre != c.axml.nombreDe(n.nombre) {
				return nil, fmt.Errorf("se cierra %q dentro de %q", c.axml.nombreDe(n.nombre), ultimo.nombre)
			}
			pila = pila[:len(pila)-1]
		}
	}
	if raiz == nil {
		return nil, fmt.Errorf("sin raíz")
	}
	if len(pila) != 0 {
		return nil, fmt.Errorf("%q queda sin cerrar", pila[len(pila)-1].nombre)
	}
	return raiz, nil
}

// uri traduce el índice del pool que llevan los nodos y atributos: -1 es «sin namespace».
func (c *contextoVista) uri(i int32) string {
	if i < 0 {
		return ""
	}
	return c.axml.nombreDe(i)
}

func (e *elementoLeido) atributo(ns, nombre string) (valor, bool) {
	for _, a := range e.attrs {
		if a.ns == ns && a.nombre == nombre {
			return a.valor, true
		}
	}
	return valor{}, false
}

// attr devuelve el texto ya resuelto de un atributo, o "" si no está. El texto necesita la tabla,
// así que la resolución vive en contextoVista.textoDeAtributo.
func (e *elementoLeido) hijo(nombre string) *elementoLeido {
	for _, h := range e.hijos {
		if h.nombre == nombre {
			return h
		}
	}
	return nil
}

func (c *contextoVista) textoDeAtributo(e *elementoLeido, ns, nombre string) string {
	v, ok := e.atributo(ns, nombre)
	if !ok {
		return ""
	}
	return c.texto(v)
}

// texto resuelve un valor que viene del AXML (un atributo del layout o del manifiesto): su cadena
// está en el pool del propio fichero, no en el global de la tabla.
func (c *contextoVista) texto(v valor) string { return c.resuelve(v, c.axml.tabla.cadenas, 0) }

// resuelve lleva el pool del que salen los índices de cadena: el del AXML cuando el valor es un
// atributo, el global de la tabla cuando el valor es una entrada de recurso. Las referencias se
// siguen hasta una cadena; si el destino no es una cadena se queda su nombre, que es lo que el
// layout puede enseñar sin intérprete (`id/total`, `style/AppTheme`). La profundidad acota una
// tabla que se refiera a sí misma.
func (c *contextoVista) resuelve(v valor, pool []string, profundidad int) string {
	switch v.tipo {
	case tipoCadena:
		if int(v.datos) < len(pool) {
			return pool[v.datos]
		}
		return ""
	case tipoReferencia:
		if c.tabla == nil {
			return fmt.Sprintf("@%#08x", v.datos)
		}
		tipo, clave, ok := c.tabla.nombreDeId(v.datos)
		if !ok {
			return fmt.Sprintf("@%#08x", v.datos)
		}
		if enlace, ok := c.tabla.valorDeId(v.datos); ok && enlace.tipo == tipoCadena && profundidad < profundidadMaxima {
			return c.resuelve(enlace, c.tabla.global.cadenas, profundidad+1)
		}
		return tipo + "/" + clave
	case tipoEnteroDec:
		return strconv.FormatInt(int64(int32(v.datos)), 10)
	case tipoEnteroHex:
		return fmt.Sprintf("%#x", v.datos)
	case tipoBooleano:
		if v.datos != 0 {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf("%#08x", v.datos)
}

// nodoDe traduce un elemento del layout al modelo, sin colocarlo. El tipo es el nombre del elemento
// tal como lo escribió el fuente (LinearLayout, TextView, Button…); el ancho y el alto se guardan
// como los declara el layout para que el consumidor sepa de dónde sale la medida, y la colocación se
// queda con los mismos números para no volver a leer el AXML.
func (c *contextoVista) nodoDe(e *elementoLeido) *nodoVista {
	if e == nil {
		return nil
	}
	n := &nodoVista{Tipo: e.nombre, declAncho: medidaWrap, declAlto: medidaWrap, layoutGravity: -1}
	if id, ok := e.atributo(uriDeRecursos, "id"); ok {
		n.ID = c.texto(id)
		n.IDRef = fmt.Sprintf("%#08x", id.datos)
	}
	if texto, ok := e.atributo(uriDeRecursos, "text"); ok {
		n.Texto = c.texto(texto)
	}
	if estilo, ok := e.atributo("", "style"); ok {
		n.Estilo = c.texto(estilo)
	}
	if ancho, ok := e.atributo(uriDeRecursos, "layout_width"); ok {
		n.Ancho, n.declAncho = c.valorDeMedida(ancho)
	}
	if alto, ok := e.atributo(uriDeRecursos, "layout_height"); ok {
		n.Alto, n.declAlto = c.valorDeMedida(alto)
	}
	if gravity, ok := e.atributo(uriDeRecursos, "gravity"); ok {
		if bits, ok := enteroDeValor(gravity); ok {
			n.gravityBits = bits
			n.Gravity = nombreDeGravity(bits)
		} else {
			n.Gravity = c.texto(gravity)
		}
	}
	if gravity, ok := e.atributo(uriDeRecursos, "layout_gravity"); ok {
		if bits, ok := enteroDeValor(gravity); ok {
			n.layoutGravity = bits
		}
	}
	if peso, ok := e.atributo(uriDeRecursos, "layout_weight"); ok {
		n.Peso = c.numeroDe(peso)
	}
	if suma, ok := e.atributo(uriDeRecursos, "weightSum"); ok {
		n.pesoSuma = c.numeroDe(suma)
	}
	if pad, ok := c.ladosDe(e, "padding"); ok {
		n.pad = pad
		copia := pad
		n.Padding = &copia
	}
	if margen, ok := c.ladosDe(e, "layout_margin"); ok {
		n.margen = margen
		copia := margen
		n.Margen = &copia
	}
	if orientacion, ok := e.atributo(uriDeRecursos, "orientation"); ok {
		// el enlace escribe el enum como entero: 0 es horizontal y 1 vertical, que son las dos
		// constantes de LinearLayout; si viniera como texto se usa tal cual
		if bits, ok := enteroDeValor(orientacion); ok {
			switch bits {
			case 0:
				n.orientacion = "horizontal"
			case 1:
				n.orientacion = "vertical"
			}
		} else {
			n.orientacion = c.texto(orientacion)
		}
	}
	if n.orientacion == "" {
		// el defecto del framework: LinearLayout horizontal salvo RadioGroup y los dos ScrollView
		switch n.Tipo {
		case "LinearLayout":
			n.orientacion = "horizontal"
		case "RadioGroup", "ScrollView":
			n.orientacion = "vertical"
		case "HorizontalScrollView":
			n.orientacion = "horizontal"
		}
	}
	for _, h := range e.hijos {
		if hijo := c.nodoDe(h); hijo != nil {
			n.Hijos = append(n.Hijos, hijo)
		}
	}
	return n
}

// ladosDe lee un relleno o un margen completo: la forma corta (padding, layout_margin) y las de un
// lado, que se aplican encima. La dirección se toma como izquierda-a-derecha, que es la del modelo:
// `start` es el lado izquierdo y `end` el derecho.
func (c *contextoVista) ladosDe(e *elementoLeido, prefijo string) (ladosVista, bool) {
	l := ladosVista{}
	declarado := false
	pon := func(nombre string, aplica func(int)) {
		v, ok := e.atributo(uriDeRecursos, nombre)
		if !ok {
			return
		}
		if dp, ok := c.dpDe(v); ok {
			aplica(dp)
			declarado = true
		}
	}
	pon(prefijo, func(dp int) { l = ladosVista{Izq: dp, Arr: dp, Der: dp, Aba: dp} })
	pon(prefijo+"Horizontal", func(dp int) { l.Izq, l.Der = dp, dp })
	pon(prefijo+"Vertical", func(dp int) { l.Arr, l.Aba = dp, dp })
	pon(prefijo+"Start", func(dp int) { l.Izq = dp })
	pon(prefijo+"End", func(dp int) { l.Der = dp })
	pon(prefijo+"Left", func(dp int) { l.Izq = dp })
	pon(prefijo+"Right", func(dp int) { l.Der = dp })
	pon(prefijo+"Top", func(dp int) { l.Arr = dp })
	pon(prefijo+"Bottom", func(dp int) { l.Aba = dp })
	if !declarado {
		return ladosVista{}, false
	}
	return l, true
}

// enteroDeValor: los enum del framework (gravity, orientation) llegan como entero. El aapt2 del SDK
// escribe `gravity="center"` como 0x11 hexadecimal y el enlazador en Go de este fork deja el mismo
// valor con su tipo; las dos formas se aceptan.
func enteroDeValor(v valor) (int32, bool) {
	switch v.tipo {
	case tipoEnteroDec, tipoEnteroHex:
		return int32(v.datos), true
	}
	return 0, false
}

// numeroDe lee un número que puede venir como flotante (aapt2 escribe `layout_weight="1"` como el
// float32 0x3f800000), como entero (el enlazador en Go) o como cadena.
func (c *contextoVista) numeroDe(v valor) float64 {
	switch v.tipo {
	case tipoFlotante:
		return float64(math.Float32frombits(v.datos))
	case tipoEnteroDec, tipoEnteroHex:
		return float64(int32(v.datos))
	case tipoCadena:
		if c.axml == nil {
			return 0
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(c.texto(v)), 64); err == nil {
			return f
		}
	}
	return 0
}

// valorDeMedida: el layout_width/height llega como número (dp, o -1/-2 para match_parent y
// wrap_content), como dimensión o como referencia a un `@dimen`; se devuelve la palabra que el
// modelo enseña y su valor en dp, que es el que usa la colocación.
func (c *contextoVista) valorDeMedida(v valor) (string, int) {
	if bits, ok := enteroDeValor(v); ok {
		switch bits {
		case medidaMatch:
			return "match_parent", medidaMatch
		case medidaWrap:
			return "wrap_content", medidaWrap
		}
		if bits >= 0 {
			return strconv.Itoa(int(bits)) + "dp", int(bits)
		}
	}
	if dp, ok := c.dpDe(v); ok {
		return strconv.Itoa(dp) + "dp", dp
	}
	return "", medidaWrap
}

// valorDeMedida es la forma que enseña un layout_width/height sin tabla detrás: la usan las pruebas
// del lector, que leen el AXML sin resources.arsc.
func valorDeMedida(v valor) string {
	texto, _ := (&contextoVista{}).valorDeMedida(v)
	return texto
}

// dpDe lleva a dp el valor de una dimensión. Tres formas medidas sobre el APK del studio:
//   - TYPE_DIMENSION (0x05) con la mantisa y la unidad en un entero complejo, que es lo que escribe
//     el aapt2 del SDK (`16dp` = 0x00001001, `8dp` = 0x00000801, `200dp` = 0x0000c801);
//   - una cadena con sufijo (`"16dp"`), que es lo que deja el enlazador en Go de este fork —medido
//     con `aapt2 dump xmltree`, que la imprime como `"16dp" (Raw: "16dp")`;
//   - una referencia a `@dimen/…`, que se sigue en la tabla.
func (c *contextoVista) dpDe(v valor) (int, bool) { return c.dpDeCon(v, 0) }

func (c *contextoVista) dpDeCon(v valor, profundidad int) (int, bool) {
	switch v.tipo {
	case tipoDimension:
		return dpDeComplejo(v.datos)
	case tipoCadena:
		if c.axml == nil {
			return 0, false
		}
		return dpDeTexto(c.texto(v))
	case tipoEnteroDec, tipoEnteroHex:
		return int(int32(v.datos)), true
	case tipoFlotante:
		return int(math.Round(float64(math.Float32frombits(v.datos)))), true
	case tipoReferencia:
		if c.tabla == nil || profundidad >= profundidadMaxima {
			return 0, false
		}
		enlace, ok := c.tabla.valorDeId(v.datos)
		if !ok {
			return 0, false
		}
		// el enlace vive en el pool global de la tabla, no en el del AXML
		if enlace.tipo == tipoCadena && int(enlace.datos) < len(c.tabla.global.cadenas) {
			return dpDeTexto(c.tabla.global.cadenas[enlace.datos])
		}
		return c.dpDeCon(enlace, profundidad+1)
	}
	return 0, false
}

// dpDeComplejo decodifica un TYPE_DIMENSION con la cuenta de `complexToFloat` del framework: 8 bits
// de unidad, 4 de radix y 24 de mantisa. A 160 dpi (la densidad que declara el modelo) un `px` es un
// dp; `sp` se toma con fontScale 1 y `pt`, `in` y `mm` se llevan por su equivalencia.
func dpDeComplejo(datos uint32) (int, bool) {
	unidad := datos & 0xf
	radix := (datos >> 4) & 0x3
	multiplicador := [...]float64{1.0 / 256, 1.0 / (1 << 15), 1.0 / (1 << 23), 1.0 / (1 << 31)}[radix]
	valor := float64(int32(datos&0xffffff00)) * multiplicador
	dp := 0.0
	switch unidad {
	case 0: // px: a dpiPantalla, un px es un dp
		dp = valor * 160.0 / dpiPantalla
	case 1: // dip/dp
		dp = valor
	case 2: // sp: fontScale 1
		dp = valor
	case 3: // pt
		dp = valor * dpiPantalla / 72
	case 4: // in
		dp = valor * dpiPantalla
	case 5: // mm
		dp = valor * dpiPantalla / 25.4
	default:
		return 0, false
	}
	return int(math.Round(dp)), true
}

// dpDeTexto lee las dimensiones que viajan como cadena, con el sufijo que admite el formato.
func dpDeTexto(s string) (int, bool) {
	s = strings.TrimSpace(s)
	unidades := [...]struct {
		sufijo string
		factor float64
	}{
		{"dip", 1}, {"dp", 1}, {"sp", 1}, {"px", 1},
		{"pt", dpiPantalla / 72.0}, {"in", dpiPantalla}, {"mm", dpiPantalla / 25.4},
	}
	for _, u := range unidades {
		if !strings.HasSuffix(s, u.sufijo) {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, u.sufijo)), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(n * u.factor)), true
	}
	return 0, false
}

// nombreDeGravity: los bits de android:gravity como la palabra que enseña el modelo, para que el
// pintor sepa cómo alinear el texto dentro de su rectángulo.
func nombreDeGravity(bits int32) string {
	if bits == 0 {
		return ""
	}
	if bits&gravityMascaraHorizontal == gravityCentroHorizontal && bits&gravityMascaraVertical == gravityCentroVertical {
		return "center"
	}
	partes := []string{}
	switch {
	case bits&gravityMascaraHorizontal == gravityCentroHorizontal:
		partes = append(partes, "center_horizontal")
	case bits&gravityMascaraHorizontal == gravityIzquierda:
		if bits&gravityBanderaDireccion != 0 {
			partes = append(partes, "start")
		} else {
			partes = append(partes, "left")
		}
	case bits&gravityMascaraHorizontal == gravityDerecha:
		if bits&gravityBanderaDireccion != 0 {
			partes = append(partes, "end")
		} else {
			partes = append(partes, "right")
		}
	case bits&gravityMascaraHorizontal == gravityRellenoHorizontal:
		partes = append(partes, "fill_horizontal")
	}
	switch bits & gravityMascaraVertical {
	case gravityCentroVertical:
		partes = append(partes, "center_vertical")
	case gravityArriba:
		partes = append(partes, "top")
	case gravityAbajo:
		partes = append(partes, "bottom")
	case gravityRellenoVertical:
		partes = append(partes, "fill_vertical")
	}
	if len(partes) == 0 {
		return fmt.Sprintf("0x%02x", bits)
	}
	return strings.Join(partes, "|")
}

// verticalDeGravity y horizontalDeGravity dejan solo el trozo que manda en cada eje. La bandera de
// dirección relativa (start/end) no cambia la cuenta en un layout de izquierda a derecha.
func horizontalDeGravity(bits int32) int32 { return bits & gravityMascaraHorizontal }
func verticalDeGravity(bits int32) int32   { return bits & gravityMascaraVertical }

// espec es el límite con el que un padre mide a un hijo: `limita` en falso es el UNSPECIFIED del
// framework, que deja al hijo medir su contenido sin techo, y `exacto` es el EXACTLY, que manda
// aunque el layout declare wrap_content (es el que usa el reparto del peso).
type espec struct {
	tam    int
	limita bool
	exacto bool
}

// especDeHijo reproduce getChildMeasureSpec: un tamaño fijo manda, match_parent ocupa el hueco que
// el padre deja y wrap_content se queda en él como mucho.
func especDeHijo(esp espec, relleno, margenes, decl int) espec {
	disponible := esp.tam - relleno - margenes
	if disponible < 0 {
		disponible = 0
	}
	switch {
	case decl >= 0:
		return espec{tam: decl, limita: true, exacto: true}
	case decl == medidaMatch:
		// un match_parent sin techo del padre mide su contenido, como el UNSPECIFIED del framework
		if !esp.limita {
			return espec{limita: false}
		}
		return espec{tam: disponible, limita: true, exacto: true}
	}
	if esp.limita {
		return espec{tam: disponible, limita: true}
	}
	return espec{limita: false}
}

// tamanoPropio: un espec EXACTLY manda sobre lo declarado (es el que usa el reparto del peso para
// dejar a un hijo de `0dp` con el alto que le toca); si no, el tamaño declarado manda; wrap_content
// toma el contenido medido, recortado al techo del padre, y match_parent ocupa lo que el padre da.
func tamanoPropio(decl int, esp espec, contenido int) int {
	switch {
	case esp.exacto:
		return esp.tam
	case decl >= 0:
		return decl
	case decl == medidaMatch:
		if esp.limita {
			return esp.tam
		}
		return contenido
	}
	if contenido < 0 {
		contenido = 0
	}
	if esp.limita && contenido > esp.tam {
		return esp.tam
	}
	return contenido
}

// contenidoHoja estima lo que mide una view sin hijos: el ancho por el texto que se va a ver y el
// alto por su tipo. Es la parte declarada como aproximada del modelo.
func contenidoHoja(n *nodoVista) (int, int) {
	visible := n.Texto
	if n.TextoDinamico != "" {
		visible = n.TextoDinamico
	}
	alto := dpAltoTexto
	if n.Tipo == "Button" {
		alto = dpAltoBoton
	}
	// el relleno del texto: el declarado o, sin él, el que trae el tema
	relleno := n.pad.horizontal()
	if relleno == 0 {
		relleno = dpRellenoTexto
	}
	ancho := len([]rune(visible))*dpPorCaracter + relleno
	if ancho < 2*dpRellenoTexto {
		ancho = 2 * dpRellenoTexto
	}
	return ancho, alto + n.pad.vertical()
}

// tipoDeContenedor: los tipos del framework se reconocen por el sufijo y el nombre, que es la única
// forma de no llevar dentro la lista de clases de la plataforma. Lo que no es lineal se apila.
func tipoDeContenedor(tipo string) string {
	switch tipo {
	case "LinearLayout", "RadioGroup", "ScrollView", "HorizontalScrollView":
		return "lineal"
	case "FrameLayout":
		return "frame"
	}
	if strings.HasSuffix(tipo, "Layout") {
		return "apilado"
	}
	return ""
}

// medir calcula el tamaño de un nodo y deja medidos sus hijos, como el `onMeasure` del framework:
// un contenedor mide a sus hijos con SU espec y después decide cuánto ocupa él.
func (n *nodoVista) medir(espAncho, espAlto espec) {
	if tipoDeContenedor(n.Tipo) == "" && len(n.Hijos) == 0 {
		contenidoAncho, contenidoAlto := contenidoHoja(n)
		n.medAncho = tamanoPropio(n.declAncho, espAncho, contenidoAncho)
		n.medAlto = tamanoPropio(n.declAlto, espAlto, contenidoAlto)
		return
	}
	if tipoDeContenedor(n.Tipo) == "lineal" {
		n.medirLineal(n.orientacion != "horizontal", espAncho, espAlto)
		return
	}
	n.medirApilado(espAncho, espAlto)
}

// medirLineal es measureVertical/measureHorizontal de LinearLayout: recorre los hijos acumulando su
// tamaño y sus márgenes, reparte el hueco sobrante entre los que tienen peso y mide el contenedor
// por su contenido cuando es wrap_content.
func (n *nodoVista) medirLineal(vertical bool, espAncho, espAlto espec) {
	padAncho, padAlto := n.pad.horizontal(), n.pad.vertical()
	espPrincipal := espAlto
	if !vertical {
		espPrincipal = espAncho
	}
	total := 0
	sumaPesos := 0.0
	type hijoConPeso struct {
		n        *nodoVista
		espAncho espec
		espAlto  espec
	}
	conPeso := []hijoConPeso{}
	for _, h := range n.Hijos {
		espHijoAncho := especDeHijo(espAncho, padAncho, h.margen.horizontal(), h.declAncho)
		espHijoAlto := especDeHijo(espAlto, padAlto, h.margen.vertical(), h.declAlto)
		h.medir(espHijoAncho, espHijoAlto)
		if vertical {
			total += h.medAlto + h.margen.vertical()
		} else {
			total += h.medAncho + h.margen.horizontal()
		}
		if h.Peso > 0 {
			sumaPesos += h.Peso
			conPeso = append(conPeso, hijoConPeso{n: h, espAncho: espHijoAncho, espAlto: espHijoAlto})
		}
	}
	// el hueco que sobra (o falta) del techo del padre se reparte en proporción al peso, como el
	// bucle de measureVertical: cada hijo descuenta su parte del exceso y su peso del denominador,
	// el tamaño se remide EXACTLY y nunca queda negativo. El denominador es weightSum si el layout
	// lo declara; si no, la suma de los pesos de los hijos.
	if sumaPesos > 0 && espPrincipal.limita {
		restante := espPrincipal.tam - padAlto - total
		if !vertical {
			restante = espPrincipal.tam - padAncho - total
		}
		denominador := sumaPesos
		if n.pesoSuma > 0 {
			denominador = n.pesoSuma
		}
		if restante != 0 {
			for i := range conPeso {
				if denominador <= 0 {
					break
				}
				h := conPeso[i].n
				reparto := int(h.Peso * float64(restante) / denominador)
				restante -= reparto
				denominador -= h.Peso
				antes := h.medAlto
				if !vertical {
					antes = h.medAncho
				}
				tam := antes + reparto
				if tam < 0 {
					tam = 0
				}
				if vertical {
					h.medir(conPeso[i].espAncho, espec{tam: tam, limita: true, exacto: true})
					total += h.medAlto - antes
				} else {
					h.medir(espec{tam: tam, limita: true, exacto: true}, conPeso[i].espAlto)
					total += h.medAncho - antes
				}
			}
		}
	}
	contenidoAncho, contenidoAlto := 0, 0
	if vertical {
		contenidoAlto = total
		for _, h := range n.Hijos {
			if h.medAncho+h.margen.horizontal() > contenidoAncho {
				contenidoAncho = h.medAncho + h.margen.horizontal()
			}
		}
	} else {
		contenidoAncho = total
		for _, h := range n.Hijos {
			if h.medAlto+h.margen.vertical() > contenidoAlto {
				contenidoAlto = h.medAlto + h.margen.vertical()
			}
		}
	}
	n.medAncho = tamanoPropio(n.declAncho, espAncho, contenidoAncho+padAncho)
	n.medAlto = tamanoPropio(n.declAlto, espAlto, contenidoAlto+padAlto)
}

// medirApilado es el camino de FrameLayout y del resto de contenedores que no son lineales: todos
// los hijos se miden contra el padre y el contenedor mide el mayor. Un hijo con match_parent se
// vuelve a medir contra el tamaño ya decidido, que es lo que hace FrameLayout.
func (n *nodoVista) medirApilado(espAncho, espAlto espec) {
	padAncho, padAlto := n.pad.horizontal(), n.pad.vertical()
	maxAncho, maxAlto := 0, 0
	for _, h := range n.Hijos {
		h.medir(especDeHijo(espAncho, padAncho, h.margen.horizontal(), h.declAncho),
			especDeHijo(espAlto, padAlto, h.margen.vertical(), h.declAlto))
		if h.medAncho+h.margen.horizontal() > maxAncho {
			maxAncho = h.medAncho + h.margen.horizontal()
		}
		if h.medAlto+h.margen.vertical() > maxAlto {
			maxAlto = h.medAlto + h.margen.vertical()
		}
	}
	n.medAncho = tamanoPropio(n.declAncho, espAncho, maxAncho+padAncho)
	n.medAlto = tamanoPropio(n.declAlto, espAlto, maxAlto+padAlto)
	for _, h := range n.Hijos {
		if h.declAncho != medidaMatch && h.declAlto != medidaMatch {
			continue
		}
		espA := especDeHijo(espec{tam: n.medAncho, limita: true, exacto: true}, padAncho, h.margen.horizontal(), h.declAncho)
		espH := especDeHijo(espec{tam: n.medAlto, limita: true, exacto: true}, padAlto, h.margen.vertical(), h.declAlto)
		h.medir(espA, espH)
	}
}

// coloca mide la raíz contra la pantalla y reparte las posiciones: es lo que hace el framework en
// `onMeasure` y `onLayout`, en ese orden.
func coloca(n *nodoVista, x, y, disponibleAncho, disponibleAlto int) {
	n.medir(espec{tam: disponibleAncho, limita: true, exacto: true},
		espec{tam: disponibleAlto, limita: true, exacto: true})
	n.colocar(x, y)
}

// colocar deja el rectángulo del nodo y coloca a sus hijos según el contenedor que sea.
func (n *nodoVista) colocar(x, y int) {
	n.Rect = rectVista{X: x, Y: y, Ancho: n.medAncho, Alto: n.medAlto}
	if len(n.Hijos) == 0 {
		return
	}
	switch tipoDeContenedor(n.Tipo) {
	case "lineal":
		if n.orientacion == "horizontal" {
			n.colocarHorizontal()
		} else {
			n.colocarVertical()
		}
	case "frame":
		n.colocarFrame()
	default:
		// RelativeLayout y el resto: sin resolver las relaciones, se apilan en el origen
		for _, h := range n.Hijos {
			h.colocar(x+n.pad.Izq+h.margen.Izq, y+n.pad.Arr+h.margen.Arr)
		}
	}
}

// colocarVertical es layoutVertical: la gravedad mayor centra o pega abajo el bloque de hijos y la
// menor (o el layout_gravity de cada uno) coloca cada hijo en el eje horizontal. El bloque se centra
// en el hueco interior —descontado el relleno—, que es la lectura simétrica de la regla; el padding
// junto a gravity="center" no se pudo medir contra un aparato y queda como frontera declarada.
func (n *nodoVista) colocarVertical() {
	interiorAncho := n.medAncho - n.pad.horizontal()
	interiorAlto := n.medAlto - n.pad.vertical()
	total := 0
	for _, h := range n.Hijos {
		total += h.medAlto + h.margen.vertical()
	}
	cursor := n.Rect.Y + n.pad.Arr
	switch verticalDeGravity(n.gravityBits) {
	case gravityAbajo:
		cursor += interiorAlto - total
	case gravityCentroVertical:
		cursor += (interiorAlto - total) / 2
	}
	menor := horizontalDeGravity(n.gravityBits)
	for _, h := range n.Hijos {
		cursor += h.margen.Arr
		g := h.layoutGravity
		if g < 0 {
			g = menor
		}
		izq := n.Rect.X + n.pad.Izq + h.margen.Izq
		switch horizontalDeGravity(g) {
		case gravityCentroHorizontal:
			izq = n.Rect.X + n.pad.Izq + (interiorAncho-h.medAncho)/2 + h.margen.Izq - h.margen.Der
		case gravityDerecha:
			izq = n.Rect.X + n.pad.Izq + interiorAncho - h.medAncho + h.margen.Izq - h.margen.Der
		}
		h.colocar(izq, cursor)
		cursor += h.medAlto + h.margen.Aba
	}
}

// colocarHorizontal es layoutHorizontal, el espejo del anterior.
func (n *nodoVista) colocarHorizontal() {
	interiorAncho := n.medAncho - n.pad.horizontal()
	interiorAlto := n.medAlto - n.pad.vertical()
	total := 0
	for _, h := range n.Hijos {
		total += h.medAncho + h.margen.horizontal()
	}
	cursor := n.Rect.X + n.pad.Izq
	switch horizontalDeGravity(n.gravityBits) {
	case gravityDerecha:
		cursor += interiorAncho - total
	case gravityCentroHorizontal:
		cursor += (interiorAncho - total) / 2
	}
	menor := verticalDeGravity(n.gravityBits)
	for _, h := range n.Hijos {
		cursor += h.margen.Izq
		g := h.layoutGravity
		if g < 0 {
			g = menor
		}
		arr := n.Rect.Y + n.pad.Arr + h.margen.Arr
		switch verticalDeGravity(g) {
		case gravityCentroVertical:
			arr = n.Rect.Y + n.pad.Arr + (interiorAlto-h.medAlto)/2 + h.margen.Arr - h.margen.Aba
		case gravityAbajo:
			arr = n.Rect.Y + n.pad.Arr + interiorAlto - h.medAlto + h.margen.Arr - h.margen.Aba
		}
		h.colocar(cursor, arr)
		cursor += h.medAncho + h.margen.Der
	}
}

// colocarFrame es layoutChildren de FrameLayout: cada hijo va en el origen del relleno y su
// layout_gravity lo desplaza dentro del hueco del padre.
func (n *nodoVista) colocarFrame() {
	interiorAncho := n.medAncho - n.pad.horizontal()
	interiorAlto := n.medAlto - n.pad.vertical()
	for _, h := range n.Hijos {
		izq := n.Rect.X + n.pad.Izq + h.margen.Izq
		arr := n.Rect.Y + n.pad.Arr + h.margen.Arr
		switch horizontalDeGravity(h.layoutGravity) {
		case gravityCentroHorizontal:
			izq = n.Rect.X + n.pad.Izq + (interiorAncho-h.medAncho)/2 + h.margen.Izq - h.margen.Der
		case gravityDerecha:
			izq = n.Rect.X + n.pad.Izq + interiorAncho - h.medAncho + h.margen.Izq - h.margen.Der
		}
		switch verticalDeGravity(h.layoutGravity) {
		case gravityCentroVertical:
			arr = n.Rect.Y + n.pad.Arr + (interiorAlto-h.medAlto)/2 + h.margen.Arr - h.margen.Aba
		case gravityAbajo:
			arr = n.Rect.Y + n.pad.Arr + interiorAlto - h.medAlto + h.margen.Arr - h.margen.Aba
		}
		h.colocar(izq, arr)
	}
}

func contarNodos(n *nodoVista) int {
	if n == nil {
		return 0
	}
	total := 1
	for _, h := range n.Hijos {
		total += contarNodos(h)
	}
	return total
}

// esConfiguracionBase: el ResTable_config de 64 bytes es base cuando, tras su propio tamaño, todo
// es cero. Es la configuración que el escritor de este fork emite y la que elige un lector cuando
// no se le pide un idioma ni una densidad.
func esConfiguracionBase(config []byte) bool {
	if len(config) < 8 {
		return false
	}
	for _, b := range config[4:] {
		if b != 0 {
			return false
		}
	}
	return true
}
