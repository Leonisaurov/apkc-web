package main

// La Fase 1 de la vista previa: leer un APK ya construido y describir su primera pantalla con un
// modelo JSON que la pestaña pueda pintar. Es el camino inverso de axmlwriter.go y arscwriter.go
// sobre los lectores que ya existían en recursos.go (leeAXML y leeTabla), más la resolución de
// referencias que hasta ahora no hacía falta: un `@+id/total` del layout es un id de recurso y su
// nombre está en la tabla.
//
// Lo que esta fase NO hace, dicho para que no se lea como un fallo:
//   - No interpreta DEX. El layout se elige por nombre (por defecto el que se llama `main`) porque
//     la referencia de verdad vive en el `setContentView` de la actividad, y eso es la Fase 2.
//   - El rectángulo es una aproximación declarada: cuenta las views de un LinearLayout, estima el
//     tamaño del texto y recorta a la pantalla. No es una medida del framework; estilos fieles,
//     imágenes, `gravity` fino y tema del sistema quedan fuera de la fase.
//   - Solo se lee una configuración por tipo: la base. Un `layout-land` se resuelve solo si se
//     pide por nombre.
//
// Todo el código es propio (MIT). No hay ninguna petición de red ni proceso: el comando lee bytes
// del zip con archive/zip, que ya usa el resto del fork para firmar y verificar.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
)

// rectVista es el rectángulo aproximado de un nodo, en dp y relativo a la pantalla.
type rectVista struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Ancho int `json:"ancho"`
	Alto  int `json:"alto"`
}

// nodoVista es una view del modelo: su tipo, su identificador resuelto, el texto resuelto, lo que
// declara de tamaño, el rectángulo y sus hijos. Los campos sin namespace no se serializan.
type nodoVista struct {
	Tipo   string       `json:"tipo"`
	ID     string       `json:"id,omitempty"`
	IDRef  string       `json:"idRef,omitempty"`
	Texto  string       `json:"texto,omitempty"`
	Estilo string       `json:"estilo,omitempty"`
	Ancho  string       `json:"ancho,omitempty"`
	Alto   string       `json:"alto,omitempty"`
	Rect   rectVista    `json:"rect"`
	Hijos  []*nodoVista `json:"hijos,omitempty"`

	orientacion string // "vertical", "horizontal" o "" (se apilan en el origen)
}

// modeloVista es el JSON que sale por stdout. `aproximado` viaja en verdadero siempre: los
// rectángulos son una estimación y el consumidor no tiene que adivinarlo.
type modeloVista struct {
	Paquete    string     `json:"paquete"`
	Version    string     `json:"version,omitempty"`
	Actividad  string     `json:"actividad,omitempty"`
	Layout     string     `json:"layout"`
	Pantalla   rectVista  `json:"pantalla"`
	Aproximado bool       `json:"aproximado"`
	Nodos      int        `json:"nodos"`
	Raiz       *nodoVista `json:"raiz"`
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
		fmt.Fprint(os.Stderr, `usage: apkc preview [--layout nombre] app.apk

Read one apk and print, as one line of JSON on stdout, the model of its first screen: type, id,
text, children and an approximate rectangle for every node. The layout is chosen by name (main by
default) because the real reference lives in the activity's setContentView, which is Phase 2. No
process is started and nothing is written, so it runs inside the tab.
`)
	}
	layout := set.String("layout", "", "nombre del recurso layout que se lee (por defecto main)")
	if err := set.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	rutas := set.Args()
	if len(rutas) != 1 {
		fmt.Fprintln(os.Stderr, "preview: se espera un apk")
		os.Exit(2)
	}
	datos, err := os.ReadFile(rutas[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "preview:", err)
		os.Exit(2)
	}
	modelo, err := modeloVistaDeAPK(datos, *layout)
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

// modeloVistaDeAPK es el cuerpo del comando, separado de los argumentos para poder probarlo con los
// bytes de un apk sin pasar por el sistema de ficheros.
func modeloVistaDeAPK(datos []byte, layout string) (*modeloVista, error) {
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
	if paquete, ok := arbolManifiesto.atributo("", "package"); ok {
		modelo.Paquete = ctxManifiesto.texto(paquete)
	}
	if version, ok := arbolManifiesto.atributo(uriDeRecursos, "versionName"); ok {
		modelo.Version = ctxManifiesto.texto(version)
	}
	if actividad, ok := actividadLanzadora(arbolManifiesto, ctxManifiesto); ok {
		modelo.Actividad = ctxManifiesto.textoDeAtributo(actividad, uriDeRecursos, "name")
	}

	rutaLayout, err := rutaDelLayout(tabla, zr, layout)
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
	modelo.Layout = rutaLayout
	modelo.Raiz = raiz
	modelo.Nodos = contarNodos(raiz)
	return modelo, nil
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
// como los declara el layout para que el consumidor sepa de dónde sale la aproximación.
func (c *contextoVista) nodoDe(e *elementoLeido) *nodoVista {
	if e == nil {
		return nil
	}
	n := &nodoVista{Tipo: e.nombre}
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
		n.Ancho = valorDeMedida(ancho)
	}
	if alto, ok := e.atributo(uriDeRecursos, "layout_height"); ok {
		n.Alto = valorDeMedida(alto)
	}
	if orientacion, ok := e.atributo(uriDeRecursos, "orientation"); ok {
		// el enlace escribe el enum como entero: 0 es horizontal y 1 vertical, que son las dos
		// constantes de LinearLayout; si viniera como texto se usa tal cual
		if orientacion.tipo == tipoEnteroDec {
			switch int32(orientacion.datos) {
			case 0:
				n.orientacion = "horizontal"
			case 1:
				n.orientacion = "vertical"
			}
		} else {
			n.orientacion = c.texto(orientacion)
		}
	}
	if n.orientacion == "" && n.Tipo == "ScrollView" {
		n.orientacion = "vertical"
	}
	for _, h := range e.hijos {
		if hijo := c.nodoDe(h); hijo != nil {
			n.Hijos = append(n.Hijos, hijo)
		}
	}
	return n
}

// valorDeMedida: el layout_width/height llega como número (dp) o como referencia a match_parent
// (-1) o wrap_content (-2); se devuelve la palabra que el modelo enseña.
func valorDeMedida(v valor) string {
	switch v.tipo {
	case tipoEnteroDec:
		switch int32(v.datos) {
		case -1:
			return "match_parent"
		case -2:
			return "wrap_content"
		}
		return strconv.FormatInt(int64(int32(v.datos)), 10) + "dp"
	case tipoReferencia:
		return "wrap_content"
	}
	return ""
}

// coloca reparte los hijos dentro del rectángulo del padre. `disponible` es lo que le queda al
// padre; un hijo con match_parent lo ocupa entero. Lo que un wrap_content no quepa se recorta.
func coloca(n *nodoVista, x, y, disponibleAncho, disponibleAlto int) {
	ancho, alto := tamanoDe(n, disponibleAncho, disponibleAlto)
	if ancho > disponibleAncho && disponibleAncho > 0 {
		ancho = disponibleAncho
	}
	if alto > disponibleAlto && disponibleAlto > 0 {
		alto = disponibleAlto
	}
	n.Rect = rectVista{X: x, Y: y, Ancho: ancho, Alto: alto}
	if len(n.Hijos) == 0 {
		return
	}
	switch n.orientacion {
	case "vertical":
		cursor := 0
		for _, h := range n.Hijos {
			coloca(h, x, y+cursor, ancho, alto-cursor)
			cursor += h.Rect.Alto
		}
	case "horizontal":
		cursor := 0
		for _, h := range n.Hijos {
			coloca(h, x+cursor, y, ancho-cursor, alto)
			cursor += h.Rect.Ancho
		}
	default:
		// un contenedor sin orientación (FrameLayout, RelativeLayout) apila en el origen
		for _, h := range n.Hijos {
			coloca(h, x, y, ancho, alto)
		}
	}
}

// tamanoDe: el tamaño que ocupa un nodo. Un contenedor con wrap_content mide la suma de sus hijos
// como los pediría cada uno; una view hoja mide su texto y el alto de su tipo.
func tamanoDe(n *nodoVista, disponibleAncho, disponibleAlto int) (int, int) {
	contenidoAncho, contenidoAlto := contenidoDelNodo(n, disponibleAncho, disponibleAlto)
	ancho := medidaDeclarada(n.Ancho, disponibleAncho, contenidoAncho)
	alto := medidaDeclarada(n.Alto, disponibleAlto, contenidoAlto)
	if ancho < 0 {
		ancho = 0
	}
	if alto < 0 {
		alto = 0
	}
	return ancho, alto
}

func contenidoDelNodo(n *nodoVista, disponibleAncho, disponibleAlto int) (int, int) {
	if len(n.Hijos) > 0 || esContenedor(n.Tipo) {
		vertical := n.orientacion == "vertical"
		if n.orientacion == "" && n.Tipo != "LinearLayout" {
			// apilado en el origen: el contenedor mide el mayor de sus hijos
			ancho, alto := 0, 0
			for _, h := range n.Hijos {
				hw, hh := tamanoDe(h, disponibleAncho, disponibleAlto)
				if hw > ancho {
					ancho = hw
				}
				if hh > alto {
					alto = hh
				}
			}
			return ancho, alto
		}
		ancho, alto := 0, 0
		for _, h := range n.Hijos {
			hw, hh := tamanoDe(h, disponibleAncho, disponibleAlto)
			if vertical {
				if hw > ancho {
					ancho = hw
				}
				alto += hh
				continue
			}
			if hh > alto {
				alto = hh
			}
			ancho += hw
		}
		return ancho, alto
	}
	// una view hoja: el ancho lo estima el texto y el alto su tipo
	alto := dpAltoTexto
	if n.Tipo == "Button" {
		alto = dpAltoBoton
	}
	ancho := len([]rune(n.Texto))*dpPorCaracter + dpRellenoTexto
	if ancho < 2*dpRellenoTexto {
		ancho = 2 * dpRellenoTexto
	}
	return ancho, alto
}

// esContenedor: los tipos de layout del framework se reconocen por el sufijo, que es la única
// forma de no llevar dentro la lista de clases de la plataforma.
func esContenedor(tipo string) bool {
	return strings.HasSuffix(tipo, "Layout") || tipo == "ScrollView"
}

func medidaDeclarada(decl string, disponible, contenido int) int {
	switch decl {
	case "", "wrap_content":
		return contenido
	case "match_parent", "fill_parent":
		return disponible
	}
	if dp, err := strconv.Atoi(strings.TrimSuffix(decl, "dp")); err == nil {
		return dp
	}
	return contenido
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
