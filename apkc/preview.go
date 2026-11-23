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
//     apilado de FrameLayout, con el tamaño del texto estimado (el dinámico si lo hay) y escalado por
//     su `textSize`. La Fase E4 añade visibilidad (`gone` no ocupa), color de texto y de fondo,
//     estilos propios de la tabla, `ImageView` con un PNG del apk decodificado en Go y `gravity` fino;
//     RelativeLayout con reglas, líneas de texto partidas y el tema del sistema siguen fuera.
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
	"image/png"
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
	// TYPE_INT_COLOR_ARGB8: un `#aarrggbb` del layout o de un `@color/x` de la tabla
	tipoColor byte = 0x1c
	// TYPE_INT_COLOR_RGB8: lo que aapt2 escribe para un `#rrggbb` de res/values/colors.xml
	// (medido con aapt2 dump resources sobre `<color name="probe">#336699</color>`)
	tipoColorRGB8 byte = 0x1d
)

// Valores de View.VISIBLE, View.INVISIBLE y View.GONE, que son los que el enlazador escribe como
// entero en android:visibility. `invisible` ocupa su sitio y no se pinta; `gone` no ocupa nada.
const (
	vistaVisible   = 0
	vistaInvisible = 4
	vistaGone      = 8
)

// Ids de los atributos del framework que esta fase entiende. Se necesitan para leerlos también de un
// `style="@style/x"`, donde el registro del estilo nombra el atributo por id y no por nombre.
const (
	idVistaGravity    = 0x010100af
	idVistaTextSize   = 0x01010095
	idVistaTextStyle  = 0x01010097
	idVistaTextColor  = 0x01010098
	idVistaBackground = 0x010100d4
	idVistaVisibility = 0x010100dc
	idVistaSrc        = 0x01010119
	idVistaPadding    = 0x010100d5
	// los lados del relleno, para leerlos también de un estilo propio por su id
	idVistaPaddingH = 0x0101012c
	idVistaPaddingV = 0x0101012d
	idVistaPaddingS = 0x0101010e
	idVistaPaddingE = 0x0101010f
	idVistaPaddingL = 0x010100d6
	idVistaPaddingR = 0x010100d8
	idVistaPaddingT = 0x010100d7
	idVistaPaddingB = 0x010100d9
	// dpTextoReferencia es el textSize al que corresponde dpPorCaracter: el ancho por carácter y el
	// alto de la línea se escalan en proporción al textSize declarado.
	dpTextoReferencia = 16
)

// pixelesDeImagenMaximos es el máximo que se vuelca de un PNG: una imagen mayor se recorta para que
// el JSON no crezca sin límite; el tamaño intrínseco se conserva siempre.
const pixelesDeImagenMaximos = 64 * 64

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

// imagenVista es lo que el pintor necesita de un `ImageView`: el tamaño intrínseco del PNG y sus
// píxeles como `#rrggbbaa`. Se decodifica en Go con image/png (biblioteca estándar, sin dependencias)
// para que la pestaña no tenga que entender el PNG ni pedir nada fuera del origen.
type imagenVista struct {
	Ancho     int      `json:"ancho"`
	Alto      int      `json:"alto"`
	Pixeles   []string `json:"pixeles,omitempty"`
	Recortada bool     `json:"recortada,omitempty"`
}

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
	// Pista es el `setHint` de un EditText: lo que se lee cuando está vacío. Marcado es el estado de
	// un CheckBox, RadioButton o Switch; viaja como puntero porque «sin marcar» también es un dato y
	// no puede confundirse con «esta vista no se marca».
	Pista   string `json:"pista,omitempty"`
	Marcado *bool  `json:"marcado,omitempty"`
	// Oyente marca la vista que registró un OnClickListener: es la que responde a un clic en el
	// render y la única a la que se le puede pedir `--pulsar`.
	Oyente  bool        `json:"oyente,omitempty"`
	Estilo  string      `json:"estilo,omitempty"`
	Ancho   string      `json:"ancho,omitempty"`
	Alto    string      `json:"alto,omitempty"`
	Gravity string      `json:"gravity,omitempty"`
	Peso    float64     `json:"peso,omitempty"`
	Padding *ladosVista `json:"padding,omitempty"`
	Margen  *ladosVista `json:"margen,omitempty"`
	// Visibilidad es `invisible` o `gone` cuando el layout lo declara; vacío es visible. Un `gone` se
	// queda en el árbol (para que su id siga resolviéndose) con el rectángulo a cero.
	Visibilidad string       `json:"visibilidad,omitempty"`
	TamTexto    string       `json:"tamTexto,omitempty"`
	Color       string       `json:"color,omitempty"`
	EstiloTexto string       `json:"estiloTexto,omitempty"`
	Fondo       string       `json:"fondo,omitempty"`
	Imagen      *imagenVista `json:"imagen,omitempty"`
	// Dibujo es lo que la vista propia dibujó al ejecutar su `onDraw`: las operaciones que el
	// intérprete pudo medir y, si no se pudo, la frontera con su motivo. Vacío es una vista que no
	// es propia o cuyo `onDraw` no se pidió.
	Dibujo *dibujoVista `json:"dibujo,omitempty"`
	Rect   rectVista    `json:"rect"`
	Hijos  []*nodoVista `json:"hijos,omitempty"`

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
	visibilidad   int    // vistaVisible, vistaInvisible o vistaGone
	tamTextoDp    int    // android:textSize ya en dp, o 0 cuando no se declara
	srcRef        uint32 // el id de `android:src` si es una referencia, o 0
	imagenAncho   int    // el tamaño intrínseco del PNG ya en dp
	imagenAlto    int
	fondoRef      uint32 // el id de `android:background` si es una referencia, o 0
}

// estaOculta: `gone` no se mide ni se coloca; `invisible` sí ocupa su sitio.
func (n *nodoVista) estaGone() bool { return n.visibilidad == vistaGone }

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
	// Dialogo es la capa que la app dejó encima de la pantalla con un `AlertDialog` propio: título,
	// mensaje y botones. Es el mismo dato que `ejecucion.dialogos`, en la forma que el consumidor
	// necesita para pintarlo sobre la actividad.
	Dialogo *dialogoVista `json:"dialogo,omitempty"`
	Raiz    *nodoVista    `json:"raiz"`
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
		fmt.Fprint(os.Stderr, `usage: apkc preview [--layout nombre] [--sin-ejecutar] [--pulsar id]… [--recrear]… [--menu]… [--menu-item menu/N]… [--escribir id/texto]… [--marcar id]… [--desmarcar id]… [--avanzar ms]… app.apk

Read one apk and print, as one line of JSON on stdout, the model of its first screen: type, id,
text, children and an approximate rectangle for every node. The layout is chosen by name (main by
default) unless the app's onCreate calls setContentView with a resource id, which the Dalvik
interpreter of Phase 2 reads from classes.dex and follows. The interpreter runs onCreate and stops
there: it does NOT touch anything on its own. An android:onClick="metodo" declared in the layout is
bound before any click, as the framework does on inflate, so --pulsar reaches it. The orders are
delivered in the order given and inside the same run (id/pulsar, 0x7f010000 or its decimal):
--pulsar runs the OnClickListener registered on that view, --escribir gives id/text to an EditText
as if the user typed it (the project's TextWatchers run), and --marcar/--desmarcar set the state of
a CheckBox, RadioButton or Switch (its OnCheckedChangeListener runs; a RadioGroup keeps one option
checked); an AlertDialog that the app shows is published as a layer (dialogo and ejecucion.dialogos)
and its buttons are pressed with --pulsar dialogo/positivo|negativo|neutro, which runs their
OnClickListener and closes the dialog; --menu runs the activity's onCreateOptionsMenu (and
onPrepareOptionsMenu) and publishes the resulting menu in ejecucion.menu with its items visible, and
--menu-item menu/N presses item N in the same run, running its OnMenuItemClickListener first and
onOptionsItemSelected after, so a programmatic options menu can be opened and used from the tab; a
Toast leaves a line in ejecucion.consola and the last one in
ejecucion.toast. The text the app writes (TextView.setText) comes back in ejecucion.capturas and in
each node's textoDinamico, the check state in each node's marcado and in ejecucion.marcas, and the
app's own output (print, println, Log.*) in ejecucion.consola. Each --recrear destroys and recreates
the current activity with its Bundle, as a rotation does, so onSaveInstanceState feeds
savedInstanceState in the new onCreate and SharedPreferences survives the run; both stores come back
in ejecucion.bundle and ejecucion.preferencias. --avanzar ms moves the logical clock forward and runs
the tasks the app posted with Handler.post/postDelayed, View.post/postDelayed, runOnUiThread and
AsyncTask.execute whose time has come: the queue runs the earliest task first and, at the same
instant, the one queued before, and while a task runs the clock is its instant, not the destination;
there are no threads and no wall clock, so the clock and the task count come back in ejecucion.reloj
and ejecucion.tareas and the same command reproduces the same JSON. --pulsar, --escribir, --marcar,
--desmarcar, --recrear and --avanzar are delivered in the order they are written. A ListView whose
adapter is the project's own is painted with one child per row, texts included: --pulsar id/lista:1
delivers onItemClick for row 1 of that list. Repeating the same list reproduces the same screen. No
process is started and nothing is written, so it runs inside the tab.
`)
	}
	layout := set.String("layout", "", "nombre del recurso layout que se lee (por defecto main)")
	sinEjecutar := set.Bool("sin-ejecutar", false, "no interpreta classes.dex: solo el modelo del layout")
	// el guion conserva el orden de la línea de comandos: flag llama a Set en ese orden, así que
	// `--pulsar a --escribir b/hola --recrear --pulsar c` se entrega como se escribió, y la lista de
	// clics, textos, marcas, recreaciones y avances del reloj no pierde su sitio
	guion := []eventoUsuario{}
	set.Var(guionDeEntrada{tipo: "pulsar", eventos: &guion}, "pulsar",
		"id de la vista (o dialogo/positivo|negativo|neutro, o id/lista:fila) cuyo oyente se ejecuta "+
			"(repetible, en orden)")
	set.Var(guionDeEntrada{tipo: "escribir", eventos: &guion}, "escribir",
		"id/texto: entrega texto a un EditText como si el usuario lo tecleara (repetible)")
	set.Var(guionDeEntrada{tipo: "marcar", eventos: &guion}, "marcar",
		"id de la vista que se marca: CheckBox, RadioButton o Switch (repetible)")
	set.Var(guionDeEntrada{tipo: "desmarcar", eventos: &guion}, "desmarcar",
		"id de la vista que se desmarca (repetible)")
	set.Var(ordenDeRecrear{eventos: &guion}, "recrear",
		"recrea la actividad actual con su Bundle, como una rotación (repetible, en orden)")
	set.Var(ordenDeMenu{eventos: &guion}, "menu",
		"abre el menú de opciones: ejecuta onCreateOptionsMenu y publica sus ítems (repetible, en orden)")
	set.Var(guionDeEntrada{tipo: ordenMenuItemTipo, eventos: &guion}, "menu-item",
		"menu/N: pulsa el ítem N del menú, por su OnMenuItemClickListener o onOptionsItemSelected "+
			"(repetible, en orden)")
	set.Var(guionDeEntrada{tipo: "avanzar", eventos: &guion}, "avanzar",
		"milisegundos que avanza el reloj lógico y ejecuta las tareas vencidas (repetible, en orden)")
	if err := set.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	rutas := set.Args()
	if len(rutas) != 1 {
		fmt.Fprintln(os.Stderr, "preview: se espera un apk")
		os.Exit(2)
	}
	if *sinEjecutar && len(guion) > 0 {
		fmt.Fprintln(os.Stderr, "preview: --pulsar, --escribir, --marcar, --desmarcar, --recrear y "+
			"--avanzar necesitan ejecutar el dex; quita --sin-ejecutar")
		os.Exit(2)
	}
	datos, err := os.ReadFile(rutas[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "preview:", err)
		os.Exit(2)
	}
	modelo, err := modeloVistaDeAPKConEventos(datos, *layout, !*sinEjecutar, guion)
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

// ordenEjecucion es una orden del emulador en el orden en que se escribió: `pulsar` lleva el id de
// la vista y `recrear` no lleva valor. Es la forma clásica que usan las pruebas de la recreación y
// de los diálogos; el comando construye eventos y esta se convierte en `modeloVistaDeAPKConOrdenes`.
type ordenEjecucion struct {
	Tipo  string
	Valor string
}

const (
	ordenPulsarTipo   = "pulsar"
	ordenRecrearTipo  = "recrear"
	ordenAvanzarTipo  = "avanzar"
	ordenMenuTipo     = "menu"
	ordenMenuItemTipo = "menu-item"
)

// guionDeEntrada es el flag de una orden de entrada. Todas las órdenes van a la misma lista, en el
// orden en que se leen de la línea de comandos: es lo que permite intercalar un clic entre dos
// tecleos, un avance del reloj entre dos clics, y que el estado que ve cada orden sea el que dejó la
// anterior.
type guionDeEntrada struct {
	tipo    string
	eventos *[]eventoUsuario
}

func (g guionDeEntrada) String() string {
	partes := make([]string, 0, len(*g.eventos))
	for _, e := range *g.eventos {
		if e.Tipo == g.tipo {
			partes = append(partes, e.Dato)
		}
	}
	return strings.Join(partes, ",")
}

func (g guionDeEntrada) Set(v string) error {
	dato := strings.TrimSpace(v)
	if dato == "" {
		return fmt.Errorf("--%s necesita un id: id/nombre, 0x7f010000 o su decimal", g.tipo)
	}
	if g.tipo == "escribir" {
		// `id/texto`: el id puede ser `id/nombre` y el texto llevar barras, así que aquí solo se exige
		// que haya barra; `partirEscribir` corta por la que cierra el id
		if id, _, ok := strings.Cut(dato, "/"); !ok || strings.TrimSpace(id) == "" {
			return fmt.Errorf("--escribir necesita id/texto: el id de la vista y el texto, " +
				"separados por «/»")
		}
	}
	if g.tipo == ordenAvanzarTipo {
		// el reloj lógico solo va hacia adelante y en milisegundos enteros: un valor que no lo sea se
		// rechaza al parsear, no a mitad de la ejecución
		if ms, err := strconv.ParseInt(dato, 10, 64); err != nil || ms < 0 {
			return fmt.Errorf("--avanzar necesita milisegundos enteros no negativos, no %q", v)
		}
	}
	*g.eventos = append(*g.eventos, eventoUsuario{Tipo: g.tipo, Dato: dato})
	return nil
}

// ordenDeRecrear es la bandera sin valor de `--recrear`: entra en el mismo guion que las demás
// órdenes, así que `--pulsar id/a --recrear` recrea el estado que dejó el clic y no el del arranque.
type ordenDeRecrear struct{ eventos *[]eventoUsuario }

func (o ordenDeRecrear) String() string { return "false" }

func (o ordenDeRecrear) Set(v string) error {
	if v == "true" {
		*o.eventos = append(*o.eventos, eventoUsuario{Tipo: ordenRecrearTipo})
	}
	return nil
}

// IsBoolFlag deja que `--recrear` se escriba sin valor, como una bandera.
func (o ordenDeRecrear) IsBoolFlag() bool { return true }

// ordenDeMenu es la bandera sin valor de `--menu`: desplegar el menú de opciones entra en el mismo
// guion que las demás órdenes, así que puede ir entre dos clics y cada ítem ve el estado anterior.
type ordenDeMenu struct{ eventos *[]eventoUsuario }

func (o ordenDeMenu) String() string { return "false" }

func (o ordenDeMenu) Set(v string) error {
	if v == "true" {
		*o.eventos = append(*o.eventos, eventoUsuario{Tipo: ordenMenuTipo})
	}
	return nil
}

// IsBoolFlag deja que `--menu` se escriba sin valor, como una bandera.
func (o ordenDeMenu) IsBoolFlag() bool { return true }

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
// la pantalla a medias se leería como si el clic hubiera funcionado. Se queda como envoltorio para no
// cambiar la forma de llamada de las pruebas de la Fase 2.
func modeloVistaDeAPKConPulsos(datos []byte, layout string, ejecutar bool, pulsos []string) (*modeloVista, error) {
	eventos := make([]eventoUsuario, 0, len(pulsos))
	for _, p := range pulsos {
		eventos = append(eventos, eventoUsuario{Tipo: ordenPulsarTipo, Dato: p})
	}
	return modeloVistaDeAPKConEventos(datos, layout, ejecutar, eventos)
}

// modeloVistaDeAPKConOrdenes es la misma llamada con la forma clásica de órdenes (`--pulsar` y
// `--recrear`): convierte cada una en un evento del guion y deja una sola ruta de ejecución.
func modeloVistaDeAPKConOrdenes(datos []byte, layout string, ejecutar bool, ordenes []ordenEjecucion) (*modeloVista, error) {
	eventos := make([]eventoUsuario, 0, len(ordenes))
	for _, o := range ordenes {
		eventos = append(eventos, eventoUsuario{Tipo: o.Tipo, Dato: o.Valor})
	}
	return modeloVistaDeAPKConEventos(datos, layout, ejecutar, eventos)
}

// modeloVistaDeAPKConEventos es el mismo cuerpo con el guion de entrada completo (`--pulsar`,
// `--escribir`, `--marcar`, `--desmarcar`, `--recrear` y `--avanzar`), en el orden en que se pidió.
// Una orden que no se puede entregar devuelve error, por el mismo motivo: no se publica una pantalla
// a medias.
func modeloVistaDeAPKConEventos(datos []byte, layout string, ejecutar bool, eventos []eventoUsuario) (*modeloVista, error) {
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
	declaradas := actividadesDeclaradas(arbolManifiesto, ctxManifiesto)
	if ejecutar {
		if dexBytes, err := entradaDeZip(zr, "classes.dex"); err == nil {
			if d, err := readDex(dexBytes); err == nil {
				preferida := descriptorDeActividad(modelo.Paquete, nombreActividad)
				var errEventos error
				modelo.Ejecucion, layoutID, errEventos = ejecutaOnCreateConEventos(d, tabla, preferida, eventos,
					func(m *maquinaDalvik) { preparaVistasDelLayout(m, zr, tabla, layout) }, declaradas)
				if errEventos != nil {
					return nil, errEventos
				}
				// la pantalla que se pinta es la que quedó delante tras la navegación, no la lanzadora
				if modelo.Ejecucion != nil && len(modelo.Ejecucion.Pantallas) > 1 &&
					modelo.Ejecucion.Actividad != "" {
					modelo.Actividad = modelo.Ejecucion.Actividad
				}
				// un dex mal formado no se ejecuta a medias ni se tapa con un modelo que parecería
				// medido: el comando lo rechaza. Una API fuera del subconjunto, en cambio, solo deja
				// avisos y el modelo sale igual.
				if modelo.Ejecucion != nil && modelo.Ejecucion.malformado {
					return nil, fmt.Errorf("el classes.dex no es ejecutable: %s", modelo.Ejecucion.Error)
				}
				// el diálogo propio que quedó abierto se publica como capa; el último de la pila es
				// el que está delante
				if modelo.Ejecucion != nil && len(modelo.Ejecucion.Dialogos) > 0 {
					capa := modelo.Ejecucion.Dialogos[len(modelo.Ejecucion.Dialogos)-1]
					modelo.Dialogo = &capa
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
	// las imágenes se decodifican antes de medir: el tamaño del PNG es el contenido del ImageView
	cargaImagenes(raiz, zr, tabla)
	coloca(raiz, 0, 0, dpPantallaAncho, dpPantallaAlto)
	anotaTextoDinamico(raiz, modelo.Ejecucion)
	anotaOyentes(raiz, modelo.Ejecucion)
	anotaEstados(raiz, modelo.Ejecucion)
	// las vistas propias ejecutan su onDraw con un Canvas del modelo: el rectángulo ya está y el
	// trazo se publica en cada nodo; si no se puede, la frontera lleva el motivo
	pintaVistasPropias(raiz, modelo.Ejecucion, modelo.Paquete)
	// las listas se expanden después de los clics: el adaptador ya está armado y es su getView el
	// que decide qué filas y qué textos se ven
	expandeListas(raiz, modelo.Ejecucion, tabla, zr)
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

// preparaVistasDelLayout lee el layout que la app pidió con `setContentView` y deja la máquina como
// la dejaría el inflado: cada vista con su `android:text` y cada `android:onClick` registrado. El
// texto se siembra porque `findViewById` devuelve la vista que ya lo trae, no una vacía: sin eso,
// `getText()` sobre `id/total` devolvía "" y el primer `total.getText().toString()` del onClick
// empezaba la cuenta en «, 1». El clic declarado se enlaza aquí porque el inflado resuelve el nombre
// contra el contexto de la vista (la actividad); sin esto, una app que declara el clic en el XML —el
// patrón de los contadores de tutorial— se ejecutaba con `oyentes` vacío y el emulador no tenía a
// quién pulsar, aunque la app funcionara en el aparato.
func preparaVistasDelLayout(m *maquinaDalvik, zr *zip.Reader, tabla *tablaRecursos, layout string) {
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
	textos := map[uint32]string{}
	var recorre func(*elementoLeido)
	recorre = func(e *elementoLeido) {
		// los hijos primero: el orden en que se enlazan no cambia el resultado, pero deja el registro
		// con las vistas de dentro antes que su contenedor, como las recorre el inflado
		for _, h := range e.hijos {
			recorre(h)
		}
		id, conID := e.atributo(uriDeRecursos, "id")
		if conID && id.datos != 0 {
			// el texto se resuelve igual que en el modelo: un literal del pool del AXML o una
			// referencia a `@string/x` de la tabla
			if texto, ok := e.atributo(uriDeRecursos, "text"); ok {
				if resuelto := ctx.texto(texto); resuelto != "" {
					textos[id.datos] = resuelto
				}
			}
		}
		valor, ok := e.atributo(uriDeRecursos, "onClick")
		if !ok {
			return
		}
		metodo := strings.TrimSpace(ctx.texto(valor))
		if metodo == "" {
			return
		}
		if !conID || id.datos == 0 {
			m.avisa("android:onClick=%q en un %s sin android:id: un clic no puede llegar a una vista sin id",
				metodo, e.nombre)
			return
		}
		if motivo := m.registraOnClickDeclarativo(id.datos, metodo); motivo != "" {
			m.avisa("%s", motivo)
		}
	}
	recorre(raiz)
	m.siembraTextosDelLayout(textos)
	tipos, marcas, grupos := estadoDeMarcasDelLayout(raiz)
	m.siembraMarcas(marcas, grupos, tipos)
}

// estadoDeMarcasDelLayout lee del layout lo que el inflado deja en las vistas marcables: el tipo de
// cada vista con id —para poder rechazar por nombre una orden que no le corresponde—, el
// `android:checked` de cada una y los RadioButton de cada RadioGroup, con su `android:checkedButton`.
func estadoDeMarcasDelLayout(raiz *elementoLeido) (map[uint32]string, map[uint32]bool, map[uint32][]uint32) {
	tipos := map[uint32]string{}
	marcas := map[uint32]bool{}
	grupos := map[uint32][]uint32{}
	var recorre func(e *elementoLeido, grupo uint32)
	recorre = func(e *elementoLeido, grupo uint32) {
		propio := uint32(0)
		if id, ok := e.atributo(uriDeRecursos, "id"); ok && id.datos != 0 {
			propio = id.datos
			tipos[propio] = e.nombre
		}
		if v, ok := e.atributo(uriDeRecursos, "checked"); ok && propio != 0 {
			marcas[propio] = v.datos != 0
		}
		if grupo != 0 && propio != 0 && e.nombre == "RadioButton" {
			grupos[grupo] = append(grupos[grupo], propio)
		}
		if e.nombre == "RadioGroup" && propio != 0 {
			if v, ok := e.atributo(uriDeRecursos, "checkedButton"); ok && v.datos != 0 {
				marcas[uint32(v.datos)] = true
			}
			grupo = propio
		}
		for _, h := range e.hijos {
			recorre(h, grupo)
		}
	}
	recorre(raiz, 0)
	return tipos, marcas, grupos
}

// esTipoMarcable dice si un widget del layout lleva estado de marcado, por su nombre de elemento:
// lo que el render pinta con una casilla o un botón de radio y lo que `--marcar` puede tocar.
func esTipoMarcable(tipo string) bool {
	switch tipo {
	case "CheckBox", "RadioButton", "Switch", "ToggleButton", "CheckedTextView":
		return true
	}
	return false
}

// admiteTecleo dice si una vista del layout acepta texto de usuario. `--escribir` sobre otra cosa se
// rechaza por su nombre en vez de meter texto donde el aparato no dejaría teclearlo.
func admiteTecleo(tipo string) bool {
	switch tipo {
	case "EditText", "AutoCompleteTextView", "MultiAutoCompleteTextView", "TextInputEditText":
		return true
	}
	return false
}

// anotaTextoDinamico copia a cada nodo la última escritura que el intérprete capturó sobre su id.
// El match es por el id de recurso, que es lo que comparten el layout y el `findViewById`. Cuenta
// tanto `setText` como `append`, porque las dos cambian lo que la pantalla enseña, y el texto que
// entregó `--escribir`, que es lo que el usuario tecleó.
func anotaTextoDinamico(n *nodoVista, ejecucion *resultadoEjecucion) {
	if n == nil || ejecucion == nil {
		return
	}
	for _, c := range ejecucion.Capturas {
		if c.IDRef == "" || c.IDRef != n.IDRef ||
			(c.Origen != "TextView.setText" && c.Origen != "TextView.append" &&
				c.Origen != "usuario.escribir") {
			continue
		}
		n.TextoDinamico = c.Texto
	}
	for _, h := range n.Hijos {
		anotaTextoDinamico(h, ejecucion)
	}
}

// anotaEstados copia al árbol el estado de marcado y la pista que dejó la ejecución. El match es por
// id de recurso, como el de los oyentes. Una vista marcable sin entrada en `marcas` conserva el
// estado que declaró el layout (o el «sin marcar» del framework que puso `nodoDe`).
func anotaEstados(n *nodoVista, ejecucion *resultadoEjecucion) {
	if n == nil || ejecucion == nil {
		return
	}
	for _, m := range ejecucion.Marcas {
		if m.IDRef != "" && m.IDRef == n.IDRef {
			valor := m.Marcado
			n.Marcado = &valor
		}
	}
	for _, p := range ejecucion.Pistas {
		if p.IDRef != "" && p.IDRef == n.IDRef {
			n.Pista = p.Pista
		}
	}
	for _, h := range n.Hijos {
		anotaEstados(h, ejecucion)
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

// expandeListas sustituye cada nodo de lista por las filas que devolvió el adaptador del proyecto.
// El nodo conserva su rectángulo, sus atributos y su id; lo que cambia son sus hijos, que pasan de
// ninguno a una fila por elemento visible. Solo toca los nodos que la ejecución registró como lista
// con `setAdapter`: un `ListView` sin adaptador se pinta vacío, como en el aparato.
func expandeListas(raiz *nodoVista, ejecucion *resultadoEjecucion, tabla *tablaRecursos, zr *zip.Reader) {
	if raiz == nil || ejecucion == nil || ejecucion.maquina == nil {
		return
	}
	listas := ejecucion.maquina.listasPintadas()
	if len(listas) == 0 {
		return
	}
	porID := map[string]*nodoVista{}
	var recorre func(*nodoVista)
	recorre = func(n *nodoVista) {
		if n == nil {
			return
		}
		if n.IDRef != "" {
			porID[n.IDRef] = n
		}
		for _, h := range n.Hijos {
			recorre(h)
		}
	}
	recorre(raiz)
	for _, lista := range listas {
		n := porID[fmt.Sprintf("%#08x", lista.ID)]
		if n == nil {
			continue
		}
		n.Hijos = nil
		for _, fila := range lista.Filas {
			n.Hijos = append(n.Hijos, nodoDeFila(fila, tabla, zr))
		}
		// el ListView apila sus filas: sin orientación, la medida caería al apilado de un
		// FrameLayout y las tres filas quedarían una encima de otra
		if n.orientacion == "" {
			n.orientacion = "vertical"
		}
	}
}

// nodoDeFila arma el nodo de una fila a partir del layout que el getView infló, con los textos que
// ese getView escribió en sus vistas con id. Sin layout (una vista creada en código) el nodo es la
// vista tal cual: su tipo y su texto, que es lo único que se puede medir sin inventar un XML.
func nodoDeFila(fila filaPintada, tabla *tablaRecursos, zr *zip.Reader) *nodoVista {
	if fila.LayoutID != 0 && tabla != nil && zr != nil {
		if ruta, err := rutaDeLayoutPorId(tabla, fila.LayoutID); err == nil {
			if datos, err := entradaDeZip(zr, ruta); err == nil {
				if axml, err := leeAXML(datos); err == nil {
					ctx := &contextoVista{axml: axml, tabla: tabla}
					if arbol, err := ctx.arbol(); err == nil {
						if n := ctx.nodoDe(arbol); n != nil {
							// en una lista, una fila ocupa el ancho del padre y solo pide el alto
							// que necesita: sin medidas declaradas, es lo que haría el ListView
							if n.declAncho == medidaWrap {
								n.declAncho = medidaMatch
							}
							aplicaTextosDeFila(n, fila.Textos)
							return n
						}
					}
				}
			}
		}
	}
	return &nodoVista{Tipo: fila.Tipo, Texto: fila.Texto, declAncho: medidaMatch,
		declAlto: medidaWrap, layoutGravity: -1}
}

// aplicaTextosDeFila copia a cada nodo el texto que el getView escribió en la vista de su id. Es el
// equivalente de `anotaTextoDinamico`, pero por instancia: cada fila trae los suyos y no el último
// que se escribió en la pantalla.
func aplicaTextosDeFila(n *nodoVista, textos map[uint32]string) {
	if n == nil || len(textos) == 0 {
		return
	}
	if n.IDRef != "" {
		id := strings.TrimPrefix(strings.ToLower(n.IDRef), "0x")
		if v, err := strconv.ParseUint(id, 16, 32); err == nil {
			if texto, ok := textos[uint32(v)]; ok {
				n.TextoDinamico = texto
			}
		}
	}
	for _, h := range n.Hijos {
		aplicaTextosDeFila(h, textos)
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

// actividadesDeclaradas recoge los `android:name` de cada `<activity>` del manifiesto, ya como
// descriptor de type_id. `startActivity` solo puede abrir una de estas, como en el aparato.
func actividadesDeclaradas(arbol *elementoLeido, ctx *contextoVista) map[string]bool {
	declaradas := map[string]bool{}
	app := arbol.hijo("application")
	if app == nil {
		return declaradas
	}
	paquete := ctx.textoDeAtributo(arbol, "", "package")
	for _, h := range app.hijos {
		if h.nombre != "activity" && h.nombre != "activity-alias" {
			continue
		}
		nombre := ctx.textoDeAtributo(h, uriDeRecursos, "name")
		if nombre == "" {
			continue
		}
		declaradas[descriptorDeActividad(paquete, nombre)] = true
	}
	return declaradas
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
	// El estilo propio se lee antes que los atributos: sus valores son los defectos y lo que el
	// elemento declara encima manda, como en el inflado del framework.
	estilo := map[uint32]valor{}
	if ref, ok := e.atributo("", "style"); ok {
		n.Estilo = c.texto(ref)
		if ref.tipo == tipoReferencia {
			estilo = c.valoresDeEstilo(ref.datos, 0)
		}
	}
	if id, ok := e.atributo(uriDeRecursos, "id"); ok {
		n.ID = c.texto(id)
		n.IDRef = fmt.Sprintf("%#08x", id.datos)
	}
	if texto, ok := e.atributo(uriDeRecursos, "text"); ok {
		n.Texto = c.texto(texto)
	}
	if pista, ok := e.atributo(uriDeRecursos, "hint"); ok {
		n.Pista = c.texto(pista)
	}
	// el estado que declara el layout: `android:checked` en una vista marcable y, para las que no
	// declaran nada, el «sin marcar» del framework. Un RadioGroup marca su opción con
	// `android:checkedButton`, que se resuelve al ejecutar (hace falta la tabla de ids).
	if esTipoMarcable(e.nombre) {
		marcado := false
		if v, ok := e.atributo(uriDeRecursos, "checked"); ok {
			marcado = v.datos != 0
		}
		n.Marcado = &marcado
	}
	if ancho, ok := e.atributo(uriDeRecursos, "layout_width"); ok {
		n.Ancho, n.declAncho = c.valorDeMedida(ancho)
	}
	if alto, ok := e.atributo(uriDeRecursos, "layout_height"); ok {
		n.Alto, n.declAlto = c.valorDeMedida(alto)
	}
	if gravity, ok := valorDeVista(e, estilo, "gravity", idVistaGravity); ok {
		if bits, ok := enteroDeValor(gravity); ok {
			n.gravityBits = bits
			n.Gravity = nombreDeGravity(bits)
		} else if texto := c.texto(gravity); texto != "" {
			// un `gravity="center_vertical|end"` escrito como texto no lo compila aapt2, pero el
			// enlazador en Go puede dejarlo así: se lee con la misma tabla de bits
			if bits, ok := bitsDeGravityDeTexto(texto); ok {
				n.gravityBits = bits
				n.Gravity = nombreDeGravity(bits)
			} else {
				n.Gravity = texto
			}
		}
	}
	if gravity, ok := e.atributo(uriDeRecursos, "layout_gravity"); ok {
		if bits, ok := enteroDeValor(gravity); ok {
			n.layoutGravity = bits
		} else if texto := c.texto(gravity); texto != "" {
			if bits, ok := bitsDeGravityDeTexto(texto); ok {
				n.layoutGravity = bits
			}
		}
	}
	if visibilidad, ok := valorDeVista(e, estilo, "visibility", idVistaVisibility); ok {
		if bits, ok := enteroDeValor(visibilidad); ok {
			n.visibilidad = int(bits)
		} else {
			n.visibilidad = visibilidadDeTexto(c.texto(visibilidad))
		}
		if nombre := nombreDeVisibilidad(n.visibilidad); nombre != "" {
			n.Visibilidad = nombre
		}
	}
	if tam, ok := valorDeVista(e, estilo, "textSize", idVistaTextSize); ok {
		n.TamTexto, n.tamTextoDp = c.tamanoDeTexto(tam)
	}
	if color, ok := valorDeVista(e, estilo, "textColor", idVistaTextColor); ok {
		n.Color = c.colorDe(color)
	}
	if estiloTexto, ok := valorDeVista(e, estilo, "textStyle", idVistaTextStyle); ok {
		if bits, ok := enteroDeValor(estiloTexto); ok {
			n.EstiloTexto = nombreDeEstiloTexto(bits)
		}
	}
	if fondo, ok := valorDeVista(e, estilo, "background", idVistaBackground); ok {
		n.Fondo = c.colorDe(fondo)
		if fondo.tipo == tipoReferencia {
			n.fondoRef = fondo.datos
		}
	}
	if src, ok := valorDeVista(e, estilo, "src", idVistaSrc); ok && src.tipo == tipoReferencia {
		n.srcRef = src.datos
	}
	if peso, ok := e.atributo(uriDeRecursos, "layout_weight"); ok {
		n.Peso = c.numeroDe(peso)
	}
	if suma, ok := e.atributo(uriDeRecursos, "weightSum"); ok {
		n.pesoSuma = c.numeroDe(suma)
	}
	if pad, ok := c.ladosDe(e, estilo, "padding"); ok {
		n.pad = pad
		copia := pad
		n.Padding = &copia
	}
	if margen, ok := c.ladosDe(e, nil, "layout_margin"); ok {
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
// `start` es el lado izquierdo y `end` el derecho. Un `estilo` no nulo aporta los defectos del
// `style="@style/x"`; lo que el elemento declara gana.
func (c *contextoVista) ladosDe(e *elementoLeido, estilo map[uint32]valor, prefijo string) (ladosVista, bool) {
	l := ladosVista{}
	declarado := false
	pon := func(nombre string, idEstilo uint32, aplica func(int)) {
		v, ok := e.atributo(uriDeRecursos, nombre)
		if !ok && estilo != nil && idEstilo != 0 {
			v, ok = estilo[idEstilo]
		}
		if !ok {
			return
		}
		if dp, ok := c.dpDe(v); ok {
			aplica(dp)
			declarado = true
		}
	}
	pon(prefijo, idVistaPadding, func(dp int) { l = ladosVista{Izq: dp, Arr: dp, Der: dp, Aba: dp} })
	pon(prefijo+"Horizontal", idVistaPaddingH, func(dp int) { l.Izq, l.Der = dp, dp })
	pon(prefijo+"Vertical", idVistaPaddingV, func(dp int) { l.Arr, l.Aba = dp, dp })
	pon(prefijo+"Start", idVistaPaddingS, func(dp int) { l.Izq = dp })
	pon(prefijo+"End", idVistaPaddingE, func(dp int) { l.Der = dp })
	pon(prefijo+"Left", idVistaPaddingL, func(dp int) { l.Izq = dp })
	pon(prefijo+"Right", idVistaPaddingR, func(dp int) { l.Der = dp })
	pon(prefijo+"Top", idVistaPaddingT, func(dp int) { l.Arr = dp })
	pon(prefijo+"Bottom", idVistaPaddingB, func(dp int) { l.Aba = dp })
	if !declarado {
		return ladosVista{}, false
	}
	return l, true
}

// valorDeVista busca una propiedad visual en el elemento y, si no la declara, en el estilo propio:
// lo que el elemento escribe encima gana, como el inflado del framework.
func valorDeVista(e *elementoLeido, estilo map[uint32]valor, nombre string, idEstilo uint32) (valor, bool) {
	if v, ok := e.atributo(uriDeRecursos, nombre); ok {
		return v, true
	}
	if estilo != nil {
		if v, ok := estilo[idEstilo]; ok {
			return v, true
		}
	}
	return valor{}, false
}

// valoresDeEstilo aplana un `style="@style/x"` de la tabla: primero su padre (si es del mismo
// paquete) y después sus propios registros. Un padre del framework se deja estar: los temas del
// sistema quedan fuera de esta fase. La profundidad acota un linaje que se refiera a sí mismo.
func (c *contextoVista) valoresDeEstilo(id uint32, profundidad int) map[uint32]valor {
	valores := map[uint32]valor{}
	if c.tabla == nil || profundidad >= profundidadMaxima {
		return valores
	}
	_, e, ok := c.tabla.entradaDeId(id)
	if !ok || e.ausente || !e.compleja {
		return valores
	}
	if e.padre != 0 && e.padre>>24 == c.tabla.idPaquete&0xff {
		valores = c.valoresDeEstilo(e.padre, profundidad+1)
	}
	for k := 0; k+12 <= len(e.crudo); k += 12 {
		nombre := u32re(e.crudo, k)
		valores[nombre] = valor{
			tam:   int(u16re(e.crudo, k+4)),
			res0:  e.crudo[k+6],
			tipo:  e.crudo[k+7],
			datos: u32re(e.crudo, k+8),
		}
	}
	return valores
}

// colorDe lleva un color a `#rrggbb` (o `#aarrggbb` si tiene alfa) venga como entero ARGB8, como
// referencia a un `@color/x` de la tabla o como cadena `"#rrggbb"` del enlazador en Go.
func (c *contextoVista) colorDe(v valor) string { return c.colorDeCon(v, false, 0) }

func (c *contextoVista) colorDeCon(v valor, enTabla bool, profundidad int) string {
	switch v.tipo {
	case tipoColor, tipoColorRGB8, tipoEnteroHex, tipoEnteroDec:
		return formatoColor(v.datos)
	case tipoCadena:
		texto := ""
		if enTabla {
			if c.tabla != nil && int(v.datos) < len(c.tabla.global.cadenas) {
				texto = c.tabla.global.cadenas[v.datos]
			}
		} else {
			texto = c.texto(v)
		}
		if col, ok := colorDeCadena(texto); ok {
			return col
		}
	case tipoReferencia:
		if c.tabla == nil || profundidad >= profundidadMaxima {
			return ""
		}
		enlace, ok := c.tabla.valorDeId(v.datos)
		if !ok {
			return ""
		}
		return c.colorDeCon(enlace, true, profundidad+1)
	}
	return ""
}

// formatoColor imprime un ARGB de 32 bits como el `#rrggbb` del fuente; con alfa distinto de ff se
// dice también, para no perder transparencia.
func formatoColor(argb uint32) string {
	alfa := argb >> 24
	if alfa == 0xff {
		return fmt.Sprintf("#%06x", argb&0xffffff)
	}
	return fmt.Sprintf("#%08x", argb)
}

// colorDeCadena lee la forma escrita `#rgb`, `#rrggbb` o `#aarrggbb`.
func colorDeCadena(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return "", false
	}
	digitos := s[1:]
	for _, r := range digitos {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return "", false
		}
	}
	switch len(digitos) {
	case 3:
		r, g, b := digitos[0:1], digitos[1:2], digitos[2:3]
		return "#" + strings.ToLower(r+r+g+g+b+b), true
	case 6:
		return "#" + strings.ToLower(digitos), true
	case 8:
		color := strings.ToLower(digitos)
		if strings.HasPrefix(color, "ff") {
			return "#" + color[2:], true
		}
		return "#" + color, true
	}
	return "", false
}

// nombreDeVisibilidad traduce el entero de View.VISIBLE/INVISIBLE/GONE; `visible` no se escribe en
// el JSON (el campo vacío ya lo dice) y un valor desconocido se queda sin nombre.
func nombreDeVisibilidad(bits int) string {
	switch bits {
	case vistaInvisible:
		return "invisible"
	case vistaGone:
		return "gone"
	}
	return ""
}

// visibilidadDeTexto entiende la forma escrita, que no compila aapt2 pero puede dejar el enlazador.
func visibilidadDeTexto(s string) int {
	switch strings.TrimSpace(s) {
	case "invisible":
		return vistaInvisible
	case "gone":
		return vistaGone
	}
	return vistaVisible
}

// nombreDeEstiloTexto: TYPE_INT con las constantes de Typeface (0 normal, 1 bold, 2 italic, 3 las dos).
func nombreDeEstiloTexto(bits int32) string {
	switch bits {
	case 1:
		return "bold"
	case 2:
		return "italic"
	case 3:
		return "bold|italic"
	}
	return ""
}

// tamanoDeTexto devuelve lo que el modelo enseña (`24sp`) y el tamaño en dp que usa la medición.
// dpDeComplejo ya lleva `sp` con fontScale 1 y `px` a la densidad del modelo (160 dpi).
func (c *contextoVista) tamanoDeTexto(v valor) (string, int) {
	if v.tipo == tipoDimension {
		if dp, ok := dpDeComplejo(v.datos); ok {
			return strconv.Itoa(dp) + unidadDeDimension(v.datos), dp
		}
	}
	if v.tipo == tipoCadena {
		if dp, ok := c.dpDe(v); ok {
			if texto := strings.TrimSpace(c.texto(v)); texto != "" {
				return texto, dp
			}
		}
	}
	if dp, ok := c.dpDe(v); ok {
		return strconv.Itoa(dp) + "dp", dp
	}
	return "", 0
}

// unidadDeDimension nombra el sufijo de un TYPE_DIMENSION por su byte bajo.
func unidadDeDimension(datos uint32) string {
	switch datos & 0xf {
	case 0:
		return "px"
	case 2:
		return "sp"
	case 3:
		return "pt"
	case 4:
		return "in"
	case 5:
		return "mm"
	}
	return "dp"
}

// bitsDeGravityDeTexto lee `center`, `center_vertical|end`, `top|start`… con la misma tabla de bits
// que usa la colocación. `start` y `end` llevan la bandera de dirección relativa, como el framework.
func bitsDeGravityDeTexto(s string) (int32, bool) {
	var bits int32
	encontrado := false
	for _, trozo := range strings.Split(s, "|") {
		switch strings.TrimSpace(trozo) {
		case "left":
			bits |= gravityIzquierda
		case "start":
			bits |= gravityIzquierda | gravityBanderaDireccion
		case "right":
			bits |= gravityDerecha
		case "end":
			bits |= gravityDerecha | gravityBanderaDireccion
		case "center_horizontal":
			bits |= gravityCentroHorizontal
		case "fill_horizontal":
			bits |= gravityRellenoHorizontal
		case "center_vertical":
			bits |= gravityCentroVertical
		case "top":
			bits |= gravityArriba
		case "bottom":
			bits |= gravityAbajo
		case "fill_vertical":
			bits |= gravityRellenoVertical
		case "center":
			bits |= gravityCentroHorizontal | gravityCentroVertical
		case "fill":
			bits |= gravityRellenoHorizontal | gravityRellenoVertical
		default:
			continue
		}
		encontrado = true
	}
	return bits, encontrado
}

// cargaImagenes decodifica el PNG de cada `android:src`/`@drawable/x`. Se llama antes de medir: el
// tamaño intrínseco de la imagen es el contenido de un ImageView con wrap_content.
func cargaImagenes(n *nodoVista, zr *zip.Reader, tabla *tablaRecursos) {
	if n == nil {
		return
	}
	if n.srcRef != 0 {
		if img, ancho, alto := imagenDeRecurso(zr, tabla, n.srcRef); img != nil {
			n.Imagen = img
			n.imagenAncho, n.imagenAlto = ancho, alto
		}
	}
	// un `android:background="@drawable/x"` XML: `<shape>` con `<solid android:color>` se publica
	// como el color de relleno. Es lo que el preview puede dibujar de un drawable XML; un
	// `<vector>`, un `<selector>` o un `<layer-list>` no se interpretan y el fondo se queda vacío,
	// sin inventarlo.
	if n.fondoRef != 0 && n.Fondo == "" {
		n.Fondo = fondoDeDrawableXML(zr, tabla, n.fondoRef)
	}
	for _, h := range n.Hijos {
		cargaImagenes(h, zr, tabla)
	}
}

// imagenDeRecurso resuelve el id de un `@drawable/x`, lee sus bytes del zip y decodifica el PNG con
// image/png. Devuelve también el tamaño en dp con la densidad del modelo (160 dpi), que es el que
// ocupa un ImageView sin medidas declaradas.
func imagenDeRecurso(zr *zip.Reader, tabla *tablaRecursos, id uint32) (*imagenVista, int, int) {
	if tabla == nil {
		return nil, 0, 0
	}
	bl, e, ok := tabla.entradaDeId(id)
	if !ok || e.ausente || e.compleja || e.valor.tipo != tipoCadena {
		return nil, 0, 0
	}
	if bl.nombre != "drawable" && bl.nombre != "mipmap" {
		return nil, 0, 0
	}
	if int(e.valor.datos) >= len(tabla.global.cadenas) {
		return nil, 0, 0
	}
	ruta := tabla.global.cadenas[e.valor.datos]
	datos, err := entradaDeZip(zr, ruta)
	if err != nil {
		return nil, 0, 0
	}
	decodificada, err := png.Decode(bytes.NewReader(datos))
	if err != nil {
		return nil, 0, 0
	}
	limites := decodificada.Bounds()
	ancho, alto := limites.Dx(), limites.Dy()
	if ancho <= 0 || alto <= 0 {
		return nil, 0, 0
	}
	img := &imagenVista{Ancho: ancho, Alto: alto}
	volcados := 0
	for y := limites.Min.Y; y < limites.Max.Y && volcados < pixelesDeImagenMaximos; y++ {
		for x := limites.Min.X; x < limites.Max.X && volcados < pixelesDeImagenMaximos; x++ {
			r, g, b, a := decodificada.At(x, y).RGBA()
			img.Pixeles = append(img.Pixeles, fmt.Sprintf("#%02x%02x%02x%02x", r>>8, g>>8, b>>8, a>>8))
			volcados++
		}
	}
	img.Recortada = volcados < ancho*alto
	// el tamaño intrínseco se lleva a dp con la densidad del modelo y la del calificador de la ruta
	densidad := densidadDeRuta(ruta)
	anchoDp := int(math.Round(float64(ancho) * dpiPantalla / float64(densidad)))
	altoDp := int(math.Round(float64(alto) * dpiPantalla / float64(densidad)))
	if anchoDp < 1 {
		anchoDp = 1
	}
	if altoDp < 1 {
		altoDp = 1
	}
	return img, anchoDp, altoDp
}

// fondoDeDrawableXML resuelve el relleno de un `android:background="@drawable/x"` cuando el recurso
// es un XML compilado: un `<shape>` en la raíz con un `<solid android:color>`. Es la parte de un
// drawable XML que el modelo puede enseñar de verdad; un `<vector>`, un `<selector>` o un
// `<layer-list>` no se interpretan aquí y devuelven vacío, para no pintar un color que el fichero
// no declara como fondo único (en un layer-list el primer `<solid>` puede ser una capa de sombra).
func fondoDeDrawableXML(zr *zip.Reader, tabla *tablaRecursos, id uint32) string {
	if tabla == nil {
		return ""
	}
	bl, e, ok := tabla.entradaDeId(id)
	if !ok || e.ausente || e.compleja || e.valor.tipo != tipoCadena {
		return ""
	}
	if bl.nombre != "drawable" && bl.nombre != "mipmap" {
		return ""
	}
	if int(e.valor.datos) >= len(tabla.global.cadenas) {
		return ""
	}
	ruta := tabla.global.cadenas[e.valor.datos]
	if !strings.HasSuffix(ruta, ".xml") {
		return ""
	}
	datos, err := entradaDeZip(zr, ruta)
	if err != nil {
		return ""
	}
	axml, err := leeAXML(datos)
	if err != nil {
		return ""
	}
	ctx := &contextoVista{axml: axml, tabla: tabla}
	arbol, err := ctx.arbol()
	if err != nil || arbol == nil || arbol.nombre != "shape" {
		return ""
	}
	var andar func(*elementoLeido) string
	andar = func(e *elementoLeido) string {
		if e.nombre == "solid" {
			if v, ok := e.atributo(uriDeRecursos, "color"); ok {
				if col := ctx.colorDe(v); col != "" {
					return col
				}
			}
		}
		for _, h := range e.hijos {
			if col := andar(h); col != "" {
				return col
			}
		}
		return ""
	}
	return andar(arbol)
}

// densidadDeRuta lee el calificador de densidad de `res/drawable-xhdpi/x.png`; sin calificador (o
// con `nodpi`/`anydpi`, que no escalan) se queda en mdpi, la densidad que declara el modelo.
func densidadDeRuta(ruta string) int {
	carpeta := ruta
	if i := strings.LastIndex(carpeta, "/"); i >= 0 {
		carpeta = carpeta[:i]
	}
	for _, parte := range strings.Split(carpeta, "-") {
		switch parte {
		case "ldpi":
			return 120
		case "mdpi":
			return 160
		case "tvdpi":
			return 213
		case "hdpi":
			return 240
		case "xhdpi":
			return 320
		case "xxhdpi":
			return 480
		case "xxxhdpi":
			return 640
		}
	}
	return dpiPantalla
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
// alto por su tipo, escalados por el `android:textSize` si lo declara. Un ImageView con PNG mide lo
// que mide la imagen (ya en dp con la densidad del modelo). Es la parte declarada como aproximada.
func contenidoHoja(n *nodoVista) (int, int) {
	if n.imagenAncho > 0 && n.imagenAlto > 0 {
		return n.imagenAncho + n.pad.horizontal(), n.imagenAlto + n.pad.vertical()
	}
	visible := n.Texto
	if n.TextoDinamico != "" {
		visible = n.TextoDinamico
	}
	alto := dpAltoTexto
	porCaracter := dpPorCaracter
	if n.tamTextoDp > 0 {
		// el ancho de un carácter y el alto de la línea crecen con el tamaño del texto: la
		// referencia es 16 sp para el ancho por carácter y una línea de vez y media el tamaño
		porCaracter = int(math.Round(float64(dpPorCaracter) * float64(n.tamTextoDp) / dpTextoReferencia))
		if porCaracter < 1 {
			porCaracter = 1
		}
		alto = int(math.Round(float64(n.tamTextoDp) * 1.5))
		if alto < 1 {
			alto = 1
		}
	}
	if n.Tipo == "Button" && alto < dpAltoBoton {
		alto = dpAltoBoton
	}
	// el relleno del texto: el declarado o, sin él, el que trae el tema
	relleno := n.pad.horizontal()
	if relleno == 0 {
		relleno = dpRellenoTexto
	}
	ancho := len([]rune(visible))*porCaracter + relleno
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
	case "ListView", "AbsListView":
		// una lista apila sus filas; la orientación la fija el modelo al expandirla
		return "lineal"
	}
	if strings.HasSuffix(tipo, "Layout") {
		return "apilado"
	}
	return ""
}

// medir calcula el tamaño de un nodo y deja medidos sus hijos, como el `onMeasure` del framework:
// un contenedor mide a sus hijos con SU espec y después decide cuánto ocupa él.
func (n *nodoVista) medir(espAncho, espAlto espec) {
	if n.estaGone() {
		// GONE no se mide: ni su tamaño ni sus hijos ocupan sitio
		n.medAncho, n.medAlto = 0, 0
		return
	}
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
		if h.estaGone() {
			continue
		}
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
		if h.estaGone() {
			continue
		}
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
		if h.estaGone() || (h.declAncho != medidaMatch && h.declAlto != medidaMatch) {
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
	if n.estaGone() {
		n.Rect = rectVista{X: x, Y: y}
		return
	}
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
			if h.estaGone() {
				h.Rect = rectVista{}
				continue
			}
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
		if h.estaGone() {
			continue
		}
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
		if h.estaGone() {
			h.Rect = rectVista{}
			continue
		}
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
		if h.estaGone() {
			continue
		}
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
		if h.estaGone() {
			h.Rect = rectVista{}
			continue
		}
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
		if h.estaGone() {
			h.Rect = rectVista{}
			continue
		}
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
