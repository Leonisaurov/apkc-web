package main

// Fase 2 de la vista previa: un intérprete Dalvik mínimo en Go, sobre el lector de dex que este fork
// ya tenía (dexreader.go, dexcode.go) y sin ningún proceso ni runtime ajeno. Ejecuta el `onCreate`
// de la actividad lanzadora y recoge el texto que la app escribe en pantalla.
//
// Lo que hace, dicho con precisión para que la frontera no se lea como un fallo:
//   - decodifica las instrucciones sobre las palabras del code_item que ya leía dexcode.go, con una
//     máquina de registros de 32 bits (un `long` o un `double` ocupa dos, como en Dalvik);
//   - resuelve llamadas a las clases del propio dex (constructores incluidos) y stubs de framework
//     para lo que la app toca: `Activity.onCreate`, `setContentView`, `findViewById`,
//     `View.setOnClickListener`, `TextView.setText`, `StringBuilder`, `Log` y `println`;
//   - la app arranca con `onCreate` y nada más: NO se pulsa ningún listener por su cuenta, porque un
//     texto que cambia sin que nadie toque la pantalla no es un emulador. Las órdenes de entrada
//     llegan por `--pulsar id/pulsar`, `--escribir id/texto`, `--marcar id` y `--desmarcar id` —y
//     `--recrear`, que rehace la actividad con su Bundle—, en el
//     orden en que se piden y dentro de la misma ejecución, así que el estado que ve cada una es el
//     que dejó `onCreate` y las órdenes anteriores. Teclear entrega el texto entero de una vez y
//     avisa a los `TextWatcher` del proyecto; marcar avisa al `OnCheckedChangeListener` y un
//     RadioGroup desmarca el resto de su grupo. Repetir el guion reproduce la misma pantalla: el
//     emulador no guarda estado entre comandos y no lo finge.
//   - la salida de la app (`System.out.println`, `print` y `Log.*`) se anota además en `consola`, en
//     orden de emisión y numerada con `secuencia`, que es el panel de consola de la pestaña. Las
//     escrituras de view siguen en `capturas`, que es el registro del emulador.
//
// Lo que NO hace: JNI, bibliotecas `.so`, multidex, `fill-array-data` y cualquier API de framework
// fuera de los stubs. Las excepciones SÍ se ejecutan cuando las lanza el propio intérprete (un
// `throw` del dex, el `parseInt` que no parsea, una división entera por cero): se busca el handler
// del `try_item` que cubre la unidad, se respeta el orden de sus filas y el catch-all, y si no hay
// ninguno el error se propaga con su tipo y su mensaje. Lo que todavía no se mide: el `finally` y el
// multi-catch, que el compilador emite como catch-all y como varias filas del mismo handler. Una
// llamada que no se sabe resolver no se inventa: se devuelve el cero de su tipo y se anota un aviso
// en el JSON, que es lo único que impide que una aproximación se confunda con una ejecución completa.
//
// Es código propio (MIT). NekoDroid se leyó como referencia de comportamiento, no de código, y
// WineDroid (GPL) no se ha traducido ni copiado.

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	pasosMaximos            = 200000
	avisosMaximos           = 24
	profundidadMaximaDalvik = 48
	// pantallasMaximas no es un límite de uso: es la única salvaguarda ante una app que se abre a
	// sí misma en onCreate sin parar. Un dispositivo acabaría agotando memoria; aquí se avisa y se
	// deja delante la última que arrancó. Los clics del usuario no tienen tope.
	pantallasMaximas = 1024
	// filasMaximas acota las filas que se piden al adaptador de un ListView: una lista de cientos
	// de elementos no es lo que esta fase pinta, y el recorte se avisa en vez de callarse.
	filasMaximas = 50
	// tareasMaximas no es un tope de uso: es la salvaguarda de la cola del reloj lógico ante una
	// tarea que se reprograma a sí misma sin mover el instante, que encadenaría tareas sin fin. Es
	// alta a propósito —100000 tareas cubren un `--avanzar` de un día con un tic por segundo— y, si
	// se alcanza, se avisa y el reloj no llega al destino: se queda en el instante de la última
	// tarea ejecutada, porque publicar una hora que no corrió sería mentir.
	tareasMaximas = 100000
)

// eventoUsuario es una orden de entrada del guion —un clic, un texto tecleado, una marca o un
// avance del reloj lógico— con el nombre del flag que la pidió. Se entregan en el orden de la línea
// de comandos, dentro de la misma ejecución, así que cada una ve el estado que dejó la anterior.
type eventoUsuario struct {
	// Tipo es el flag que la pidió: "pulsar", "escribir", "marcar", "desmarcar", "recrear",
	// "avanzar", "menu", "menu-item" o "intercambiar".
	Tipo string
	// Dato lleva el id de la vista, el texto pegado tras el primer «/» de «escribir», los
	// milisegundos de «avanzar» o la forma escrita del intercambio; en «recrear» va vacío.
	Dato string
	// Un evento «intercambiar» lleva además el dex del código nuevo ya compilado y el método que
	// instala en la sesión viva. Viajan en el evento, no en una ruta: dentro de la pestaña no hay
	// ficheros que leer, y así el guion es el mismo con bytes de un lado y de otro. `Metodo` vacío
	// significa la clase entera.
	Dex    []byte
	Clase  string
	Metodo string
	Proto  string
}

// tareaReloj es una tarea diferida: su instante en milisegundos del reloj lógico, el orden en que se
// encoló (dos tareas del mismo instante corren en el orden en que se pidieron) y el cuerpo que la
// ejecuta. No hay hilos: la tarea corre cuando el guion llega a su instante.
type tareaReloj struct {
	instante int64
	orden    int
	origen   string
	// runnable es el objeto que el `post`/`postDelayed` encoló, para que `removeCallbacks` pueda
	// retirar exactamente esa tarea; las tareas que no vienen de un Runnable (el AsyncTask) lo dejan
	// nulo y no se cancelan con él.
	runnable *objetoDalvik
	ejecuta  func() error
}

// capturaTexto es una escritura que el intérprete pudo ver: qué stub la recibió, sobre qué view (si
// es una view con id de recurso) y con qué texto.
type capturaTexto struct {
	Origen string `json:"origen"`
	ID     string `json:"id,omitempty"`
	IDRef  string `json:"idRef,omitempty"`
	Texto  string `json:"texto"`
}

// marcaVista es el estado de una vista marcable (CheckBox, RadioButton, Switch) con el estado que
// tiene al final del guion.
type marcaVista struct {
	ID      string `json:"id,omitempty"`
	IDRef   string `json:"idRef,omitempty"`
	Marcado bool   `json:"marcado"`
}

// pistaVista es el `setHint` que la app puso en una vista: no es su texto y no se mezcla con él.
type pistaVista struct {
	ID    string `json:"id,omitempty"`
	IDRef string `json:"idRef,omitempty"`
	Pista string `json:"pista"`
}

// lineaConsola es una línea de la consola de la app: una llamada a `print`/`println` o a `Log.*`.
// `secuencia` es el orden de emisión dentro de la ejecución (1, 2, 3…) y es lo único que se puede
// medir sin reloj: la hora de verdad no es determinista y no se finge.
type lineaConsola struct {
	Origen    string `json:"origen"`
	Texto     string `json:"texto"`
	Secuencia int    `json:"secuencia"`
}

// dialogoVista es la capa de un `AlertDialog` propio tal como se ve: título, mensaje y los botones
// que se pueden pulsar. No modela el fondo ni la posición; eso lo pinta el consumidor encima de la
// pantalla, que es lo único que hace falta para reconocer un diálogo.
type dialogoVista struct {
	Titulo  string         `json:"titulo,omitempty"`
	Mensaje string         `json:"mensaje,omitempty"`
	Botones []botonDialogo `json:"botones,omitempty"`
}

// botonDialogo es un botón de un diálogo: su papel (`positivo`, `negativo` o `neutro`, que es el
// que distingue el listener que le toca) y el texto que muestra. Se pulsa con `--pulsar dialogo/…`.
type botonDialogo struct {
	Tipo  string `json:"tipo"`
	Texto string `json:"texto"`
}

// menuVista es el menú de opciones que `onCreateOptionsMenu` construyó, tal como se ve al
// desplegarlo: si está abierto, sus ítems visibles y el último que se pulsó. Es un dato PROPIO del
// modelo (el framework no deja rastro del menú en el layout), y por eso viaja aquí y no en el árbol.
type menuVista struct {
	Abierto bool `json:"abierto,omitempty"`
	// Items son los ítems visibles, en el orden en que la app los añadió. Su `id` es `menu/N`, donde
	// N es la posición en el menú completo: la pestaña manda ese id a `--menu-item` y el índice no se
	// corre cuando un ítem oculto se salta.
	Items []itemMenuVista `json:"items,omitempty"`
	// Seleccionado es el `id` del último ítem que el guion pulsó, ya cerrado el menú.
	Seleccionado string `json:"seleccionado,omitempty"`
}

// itemMenuVista es un ítem de ese menú: lo que se ve y el id con el que la app lo reconoce en
// `onOptionsItemSelected`.
type itemMenuVista struct {
	ID      string `json:"id"`
	Titulo  string `json:"titulo"`
	ItemID  int32  `json:"itemId,omitempty"`
	Orden   int32  `json:"orden,omitempty"`
	Marcado bool   `json:"marcado,omitempty"`
}

// oyenteVista es una vista que registró un `OnClickListener`: su id de recurso y su nombre, que es
// lo que la pestaña necesita para pintar el botón de pulsar y para mandar `--pulsar id/pulsar`.
type oyenteVista struct {
	ID    string `json:"id,omitempty"`
	IDRef string `json:"idRef,omitempty"`
}

// filaPintada es una fila que devolvió el `getView` del adaptador del proyecto: el layout del que
// salió cuando lo infló (`layoutID`) o el tipo de la vista creada en código, y los textos que el
// propio `getView` escribió en las vistas con id de la fila. El modelo arma con esto los hijos del
// `ListView`, y por eso la medida sale del layout que la app devuelve y no de uno inventado.
type filaPintada struct {
	LayoutID uint32
	Tipo     string
	Texto    string
	Textos   map[uint32]string
}

// listaPintada es una lista del modelo con sus filas ya ejecutadas, en el orden del adaptador.
type listaPintada struct {
	ID    uint32
	Filas []filaPintada
}

// resultadoEjecucion viaja dentro del modelo de la vista. `error` no tumba el comando: un layout sin
// intérprete sigue siendo útil, y el consumidor tiene que poder distinguir «no hay clases.dex» de
// «el onCreate falló».
type resultadoEjecucion struct {
	Actividad     string         `json:"actividad,omitempty"`
	Metodo        string         `json:"metodo,omitempty"`
	Pasos         int            `json:"pasos"`
	Interacciones int            `json:"interacciones,omitempty"`
	Capturas      []capturaTexto `json:"capturas,omitempty"`
	Consola       []lineaConsola `json:"consola,omitempty"`
	Oyentes       []oyenteVista  `json:"oyentes,omitempty"`
	Pulsaciones   []string       `json:"pulsaciones,omitempty"`
	// Escrituras es el guion de `--escribir` entregado, en orden; Marcas y Pistas son el estado que
	// dejó en las vistas que lo declaran o lo tocaron, para que el modelo y el render lo enseñen.
	Escrituras []string     `json:"escrituras,omitempty"`
	Marcas     []marcaVista `json:"marcas,omitempty"`
	Pistas     []pistaVista `json:"pistas,omitempty"`
	Avisos     []string     `json:"avisos,omitempty"`
	Error      string       `json:"error,omitempty"`
	// Pantallas es el recorrido de actividades que la ejecución abrió, en orden, con la actual al
	// final: es la prueba de que `startActivity` y `finish()` movieron la app de pantalla.
	Pantallas []string `json:"pantallas,omitempty"`
	// Ciclo es la secuencia de métodos de ciclo de vida que se llamaron (`onCreate`, `onStart`,
	// `onResume`, `onPause`, `onStop`, `onDestroy`, `onRestart`), en orden.
	Ciclo []string `json:"ciclo,omitempty"`
	// Bundle es el último estado que la actividad guardó en `onSaveInstanceState`, ya volcado: es lo
	// que recibe la instancia recreada y lo que deja reproducible la rotación.
	Bundle map[string]any `json:"bundle,omitempty"`
	// Preferencias es el SharedPreferences de la ejecución, por nombre de fichero y clave. Dura toda
	// la ejecución (también a través de una recreación) y se publica para poder reproducirlo.
	Preferencias map[string]map[string]any `json:"preferencias,omitempty"`
	// Dialogos es la pila de diálogos propios que quedaron abiertos sobre la pantalla actual, del
	// más antiguo al que está delante: es la capa que el modelo pinta encima de la actividad.
	Dialogos []dialogoVista `json:"dialogos,omitempty"`
	// Menu es el menú de opciones de la actividad tal como quedó: los ítems que la app añadió y si
	// el guion lo dejó abierto. Sin `onCreateOptionsMenu` que ejecutar se queda nulo.
	Menu *menuVista `json:"menu,omitempty"`
	// Toast es el último aviso efímero que la app mostró (`Toast.show`): no deja vista, pero sí el
	// texto que se ve y la línea de consola que se anota al mostrarlo.
	Toast string `json:"toast,omitempty"`
	// Reloj es el tiempo lógico alcanzado, en milisegundos, y Tareas cuántas tareas diferidas
	// ejecutó la cola. Los dos son deterministas: salen del guion, no de un reloj de pared.
	Reloj  int64 `json:"reloj,omitempty"`
	Tareas int   `json:"tareas,omitempty"`
	// Intercambios es lo que los hot swaps del guion cambiaron en la tabla de despacho, en orden:
	// qué método se sustituyó o se añadió, para que la pestaña pueda enseñarlo.
	Intercambios []resultadoIntercambio `json:"intercambios,omitempty"`

	// malformado separa dos clases de fallo que no son lo mismo: «la app llama a algo que el MVP no
	// cubre» (se sigue con el modelo y rc 0) y «estos bytes no son un dex ejecutable», que el comando
	// rechaza. No viaja al JSON: el consumidor ve `error` y el código de salida.
	malformado bool
	// maquina es la que quedó delante al terminar: el modelo se la pide para ejecutar el adaptador
	// de cada `ListView` después de los clics, que es el único momento en que la lista está armada.
	maquina *maquinaDalvik
}

// errorDexMalformado marca los fallos estructurales: un payload de switch que no cabe en el cuerpo o
// un registro fuera del marco no son una app fuera del subconjunto, son bytes que no se pueden
// ejecutar. Se distinguen para que `preview` los rechace en vez de imprimir un modelo a medias.
type errorDexMalformado struct{ motivo string }

func (e *errorDexMalformado) Error() string { return e.motivo }

// malformado arma uno de esos errores con el formato de siempre.
func malformado(formato string, args ...any) error {
	return &errorDexMalformado{motivo: fmt.Sprintf(formato, args...)}
}

// esMalformado dice si el error, o algo que envuelve, es de los que invalidan los bytes. Se recorre
// con Unwrap en vez de `errors.As` para no arrastrar la reflexión de ese helper al binario de wasm.
func esMalformado(err error) bool {
	for err != nil {
		if _, ok := err.(*errorDexMalformado); ok {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// falla anota el error de la ejecución y recuerda si es de los que invalidan los bytes.
func (r *resultadoEjecucion) falla(err error) {
	r.Error = err.Error()
	r.malformado = esMalformado(err)
}

// marcaAncha ocupa el segundo registro de un valor de 64 bits, para que un `move-wide` copie los dos
// y una lectura del registro alto no devuelva un valor que parece un entero normal.
type marcaAncha struct{}

// objetoDalvik es una instancia: su clase, sus campos (con la clave dueño.nombre, para que un campo
// heredado no se confunda con otro), el id de recurso si vino de `findViewById`, el texto que ya se
// le puso, la etiqueta de `setTag` y el listener que se registró sobre una view.
type objetoDalvik struct {
	clase  string
	campos map[string]any
	id     uint32
	texto  string
	// textoEscrito dice que alguien llamó a `setText`/`append` sobre esta vista: con eso la siembra
	// del layout no pisa un texto vacío que la app escribió a propósito
	textoEscrito bool
	tag          any
	cadena       *strings.Builder
	oyente       any
	// marcado es el estado de un CompoundButton y `marcadoPuesto` recuerda si alguien lo fijó (la
	// siembra del layout, un `setChecked` o un toque); sin ese segundo campo no se distingue un
	// «sin marcar» de un «nadie dijo nada» a la hora de sembrar.
	marcado       bool
	marcadoPuesto bool
	// vigilantesTexto son los TextWatcher de `addTextChangedListener`: se avisan al teclear
	vigilantesTexto []any
	// oyenteMarca es el OnCheckedChangeListener de un CompoundButton; el último registrado gana,
	// como en el framework
	oyenteMarca any
	// pista es el `setHint` de un EditText: no es su texto y no se mezcla con él
	pista string
	// menuItems es lo que `Menu.add` dejó en un menú del framework, en orden: el estado vive en la
	// instancia, así que dos pantallas no comparten menú. Los campos `item*` son el estado de un
	// MenuItem: su id de aplicación, su texto, si se ve y si está habilitado.
	menuItems      []*objetoDalvik
	itemID         int32
	itemGrupo      int32
	itemOrden      int32
	itemTitulo     string
	itemVisible    bool
	itemHabilitado bool
	itemMarcado    bool
	itemOyente     any
	// intentClase es el destino de un `Intent(Context, Class)` de una actividad propia: es lo que
	// `startActivity` lee para abrir la pantalla siguiente
	intentClase string
	// bundle es el contenido de un `Bundle` del framework: sus claves y valores, ya normalizados. El
	// editor de SharedPreferences usa `editor` para los cambios pendientes hasta apply/commit.
	bundle map[string]any
	// prefsFichero nombra el fichero de SharedPreferences que este objeto representa; el editor lo
	// hereda para saber dónde aplicar sus cambios.
	prefsFichero string
	editor       map[string]any
	// adaptador es lo que `ListView.setAdapter` dejó en la lista: el modelo lo ejecuta al pintar.
	adaptador any
	// elementos es lo que un `ArrayAdapter` del framework recibió en su constructor; sirve de
	// respaldo cuando la subclase del proyecto no reescribe getCount/getItem/getView.
	elementos []any
	// layoutID es el recurso que infló una vista: el `getView` de un adaptador devuelve una vista
	// con esto y el modelo mide la fila a partir del layout de verdad.
	layoutID uint32
	// vistas son las vistas de ESTA instancia inflada, por id: dos filas del mismo layout no
	// comparten el `findViewById`, como no lo comparten en el aparato.
	vistas map[uint32]*objetoDalvik
	// mensajeExcepcion es el texto que un `Throwable` recibió en su constructor: es lo que devuelve
	// `getMessage()` y lo que el aviso de una excepción sin capturar enseña.
	mensajeExcepcion string
	// lista es el contenido de un `java.util.ArrayList` del framework: el estado vive en la
	// instancia, así que dos listas no comparten sus elementos y `get` respeta su índice. Se
	// distingue de «sin lista» por no ser nulo: `clear` la deja vacía, no nula.
	lista []any
	// conjunto es el `java.util.Set` que devuelve `getBondedDevices`: aquí siempre vacío, porque la
	// vista previa no tiene radio ni equipos reales. `iterando` e `indice` son el cursor de su
	// `iterator()`, que recorre esa misma lista.
	conjunto []any
	iterando []any
	indice   int
	// bluetoothActivo es el estado del adaptador de `getDefaultAdapter`: se modela encendido para
	// que la UI de la app pueda recorrer su camino completo sin un dispositivo.
	bluetoothActivo bool
	// lo que un `Paint` lleva para el dibujo de una vista propia: color, grosor y tamaño del texto.
	// Se guarda en el objeto del framework porque un onDraw puede tener varios Paint a la vez.
	paintColor  uint32
	paintPuesto bool
	paintGrosor float64
	paintTamano float64
	// anchoVista y altoVista son el rect que el layout ya midió para esta vista, en píxeles del
	// modelo. Un `onDraw` que pregunta por su tamaño los necesita: sin ellos `getWidth`/`getHeight`
	// caerían en cero y todas las figuras se concentrarían en el origen.
	anchoVista int32
	altoVista  int32
}

// arregloDalvik es un arreglo: el descriptor de su tipo y sus huecos. Los de tipos primitivos
// guardan el valor ya extendido (un `byte` como int32 con signo).
type arregloDalvik struct {
	tipo  string
	datos []any
}

// metodoDalvik es un método del dex listo para ejecutarse. `item` es nil cuando el método no tiene
// cuerpo (abstracto o nativo), y entonces solo puede atenderse como stub. `dex` es el fichero de
// donde salieron sus `insns`: un code_item no es autónomo, sus índices apuntan a los pools de SU dex,
// así que tras un hot swap cada método se interpreta contra el suyo y no contra el de la sesión.
type metodoDalvik struct {
	nombre  string
	proto   string
	retorno string
	acceso  uint32
	item    *codeItem
	dex     *dexFile
}

type claseDalvik struct {
	nombre    string
	super     string
	metodos   map[string]*metodoDalvik
	estaticos map[string]any
}

// programaDalvik es el dex cargado como programa: clases con sus métodos y sus estáticos.
type programaDalvik struct {
	dex    *dexFile
	clases map[string]*claseDalvik
}

// cargaPrograma convierte el dex leído por dexreader.go en un programa ejecutable. Los estáticos
// toman su constante del encoded_array_item que ya lee dexcode.go: las de R son lo que hace que
// `setContentView(R.layout.main)` sea un `const` con el id dentro.
func cargaPrograma(d *dexFile) (*programaDalvik, error) {
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	p := &programaDalvik{dex: d, clases: make(map[string]*claseDalvik, len(d.classes))}
	for _, c := range d.classes {
		cl := &claseDalvik{
			nombre:    c.name,
			super:     c.super,
			metodos:   map[string]*metodoDalvik{},
			estaticos: map[string]any{},
		}
		cd := datos[c.name]
		metodos := append(append([]encodedMethod{}, cd.directs...), cd.virtuals...)
		for _, m := range metodos {
			if int(m.methodIdx) >= len(d.methods) {
				return nil, fmt.Errorf("%s: el método %d no está en method_ids", c.name, m.methodIdx)
			}
			ref := d.methods[m.methodIdx]
			met := &metodoDalvik{
				nombre:  ref.name,
				proto:   descDeProto(ref.proto),
				retorno: ref.proto.returnType,
				acceso:  m.accessFlags,
				dex:     d,
			}
			if m.codeOff != 0 {
				item, err := d.readCodeItem(m.codeOff)
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %w", c.name, ref.name, err)
				}
				met.item = &item
			}
			cl.metodos[ref.name+met.proto] = met
		}
		for _, f := range cd.staticFields {
			if int(f.fieldIdx) >= len(d.fields) {
				continue
			}
			cl.estaticos[claveCampo(d.fields[f.fieldIdx])] = valorCodificado(f.valueType, f.value)
		}
		p.clases[c.name] = cl
	}
	return p, nil
}

// descDeProto arma el descriptor de un proto: es la firma con la que se busca un método.
func descDeProto(p dexProto) string {
	return "(" + strings.Join(p.params, "") + ")" + p.returnType
}

// claveCampo nombra un campo por dueño y nombre. El tipo no entra: un mismo nombre en dos clases de
// la misma jerarquía sí tiene que distinguirse, y eso lo da el dueño.
func claveCampo(f dexField) string { return f.owner + "." + f.name }

// valorCodificado traduce el encoded_value de un estático a un valor de la máquina.
func valorCodificado(vf byte, v uint64) any {
	switch vf & 0x1f {
	case valueInt:
		return int32(v)
	case valueLong:
		return int64(v)
	case valueFloat:
		return math.Float32frombits(uint32(v))
	case valueDouble:
		return math.Float64frombits(v)
	case valueShort:
		return int32(int16(v))
	case valueChar:
		return int32(uint16(v))
	case valueByte:
		return int32(int8(v))
	case valueBool:
		return v != 0
	}
	return nil
}

// valorCero es el cero del tipo de un campo o de un retorno: así una lectura de algo que nadie
// escribió no revienta y tampoco devuelve un valor que parezca medido.
func valorCero(tipo string) any {
	if tipo == "" {
		return nil
	}
	switch tipo[0] {
	case 'Z', 'B', 'S', 'C', 'I':
		return int32(0)
	case 'J':
		return int64(0)
	case 'F':
		return float32(0)
	case 'D':
		return float64(0)
	}
	return nil
}

// anchoDe son las palabras que ocupa un valor de ese tipo: dos para `long` y `double`.
func anchoDe(tipo string) int {
	if tipo != "" && (tipo[0] == 'J' || tipo[0] == 'D') {
		return 2
	}
	return 1
}

// maquinaDalvik es el estado de una ejecución.
type maquinaDalvik struct {
	prog     *programaDalvik
	tabla    *tablaRecursos
	pasos    int
	capturas []capturaTexto
	consola  []lineaConsola
	avisos   []string
	vistos   map[string]bool
	layoutID uint32
	oyentes  []oyenteRegistrado
	// las listas que `setAdapter` dejó armadas y los `OnItemClickListener` que se registraron
	// sobre ellas: es lo que convierte `--pulsar id/lista:fila` en un clic por posición.
	listas      []*objetoDalvik
	oyentesItem []oyenteItemRegistrado
	interaccion int
	profundidad int
	// la instancia de la actividad que corre `onCreate`: es el receptor de un `android:onClick`
	// declarado en el layout, que el framework resuelve contra el contexto de la vista
	actividad *objetoDalvik
	// las vistas que `findViewById` ya devolvió, por id: un `android:onClick` recibe la misma
	// instancia que la app tocó, no una copia con el id puesto
	vistas map[uint32]*objetoDalvik
	// el `android:text` que el layout declara para cada id, por si la vista se crea después del
	// inflado (un `findViewById` dentro del `onClick`, por ejemplo): el framework no devuelve una
	// vista vacía, devuelve la que ya trae su texto
	textosDeLayout map[uint32]string
	// lo que el layout dice de cada vista con id: su tipo (para rechazar por nombre una orden que no
	// le corresponde), su `android:checked` y los RadioButton de cada RadioGroup
	tiposVista map[uint32]string
	marcas     map[uint32]bool
	pistas     map[uint32]string
	// marcasConocidas son las vistas cuyo estado se sabe (lo declaró el layout o alguien las tocó):
	// las únicas que el resultado publica
	marcasConocidas map[uint32]bool
	// gruposRadio lleva, por id del RadioGroup, los ids de sus RadioButton: marcar uno desmarca el
	// resto del grupo, como el framework. grupoDe hace el camino inverso.
	gruposRadio map[uint32][]uint32
	grupoDe     map[uint32]uint32
	// el recorrido de actividades: cada `startActivity` deja aquí la clase destino y el bucle de
	// `ejecutaOnCreateNavegando` abre la siguiente pantalla; `finish()` pide volver a la anterior
	pendiente string
	terminada bool
	arrancada bool
	// las actividades que el manifiesto declara, por descriptor: fuera de esta lista no se lanza
	// nada, porque en el aparato tampoco se puede arrancar una actividad no declarada
	declaradas map[string]bool
	// ciclo de vida que esta máquina ejecutó, para el registro del resultado
	ciclo []string
	// registro es el ciclo compartido por todas las pantallas, en orden real: una actividad que se
	// pausa anota antes de que la siguiente arranque, y al volver sigue donde tocaba
	registro *[]string
	// el Intent con el que se abrió esta actividad, para `getIntent`; nil en la lanzadora
	intent *objetoDalvik
	// el Intent que un `startActivity` acaba de entregar: el bucle de navegación lo pasa a la
	// máquina de la actividad nueva
	intentoSaliente *objetoDalvik
	// almacen es el SharedPreferences de la ejecución: en memoria, sin ficheros, y compartido por
	// todas las actividades y por la instancia recreada
	almacen *almacenPreferencias
	// bundleInicial es el `savedInstanceState` con el que arranca esta máquina: nil salvo que venga
	// de una recreación, que es cuando el framework lo entrega no nulo
	bundleInicial *objetoDalvik
	// dialogos es la pila de `AlertDialog` propios abiertos en esta pantalla, del más antiguo al
	// que está delante; pulsar un botón cierra el de arriba, como el framework
	dialogos []*objetoDalvik
	// menu es el menú de opciones que `onCreateOptionsMenu` construyó para esta pantalla; nil hasta
	// que el guion pide abrirlo. `menuAbierto` es si quedó desplegado y `menuSeleccionado`, el id del
	// último ítem que se pulsó (ya cerrado).
	menu             *objetoDalvik
	menuAbierto      bool
	menuSeleccionado string
	// toast es el último texto que la app mostró con `Toast.show`, el aviso efímero del modelo
	toast string
	// estadoGuardado es lo que la última `onSaveInstanceState` dejó en el Bundle, para publicarlo
	estadoGuardado map[string]any
	// reloj es el tiempo lógico de la app, compartido por todas las pantallas: no hay hilos ni reloj
	// de pared, y solo el guion (`--avanzar N`) lo mueve
	reloj *int64
	// cola son las tareas diferidas que esta pantalla posteó. Al cubrir una actividad sus tareas
	// dejan de correr, como el Handler de una pantalla que ya no está delante.
	cola []tareaReloj
	// tareas es cuántas tareas ejecutó la cola y encoladas el orden de encolado siguiente
	tareas    int
	encoladas int
	// cortada recuerda que la cola se detuvo por la salvaguarda: el reloj no puede publicar un
	// destino que no se ejecutó
	cortada bool
	// pasosBase es el valor de `pasos` al arrancar la tarea en curso: el límite de pasos se mide
	// desde ahí, así que cada tarea de la cola tiene su propio presupuesto
	pasosBase int
	// dibujo es el lienzo que recoge las operaciones del `onDraw` de una vista propia mientras
	// corre; fuera de una llamada de dibujo es nil y los stubs de Canvas no anotan nada.
	dibujo *dibujoVista
}

type oyenteRegistrado struct {
	oyente any
	vista  any
	// `metodo` solo lo llevan los oyentes declarativos (`android:onClick="sumar"`): el nombre del
	// método a llamar en `oyente`. Un `setOnClickListener` lo deja vacío y se usa `onClick`.
	metodo string
}

// almacenPreferencias es el SharedPreferences de la ejecución: un mapa por nombre de fichero. Vive
// en memoria y no toca el disco; la ejecución lo publica en el modelo para que otra la reproduzca.
type almacenPreferencias struct {
	ficheros map[string]map[string]any
}

func nuevoAlmacenPreferencias() *almacenPreferencias {
	return &almacenPreferencias{ficheros: map[string]map[string]any{}}
}

// de devuelve las claves de un fichero y lo crea si no existe, como hace `getSharedPreferences`.
func (a *almacenPreferencias) de(fichero string) map[string]any {
	if a == nil {
		return map[string]any{}
	}
	if a.ficheros == nil {
		a.ficheros = map[string]map[string]any{}
	}
	claves := a.ficheros[fichero]
	if claves == nil {
		claves = map[string]any{}
		a.ficheros[fichero] = claves
	}
	return claves
}

// volcado devuelve el almacén para publicarlo; nil cuando no se escribió ninguna preferencia, para
// no inventar un campo vacío en el modelo.
func (a *almacenPreferencias) volcado() map[string]map[string]any {
	if a == nil || len(a.ficheros) == 0 {
		return nil
	}
	salida := make(map[string]map[string]any, len(a.ficheros))
	for nombre, claves := range a.ficheros {
		copia := make(map[string]any, len(claves))
		for k, v := range claves {
			copia[k] = v
		}
		salida[nombre] = copia
	}
	return salida
}

// oyenteItemRegistrado es un `AdapterView.OnItemClickListener` sobre una lista: el clic llega con
// la posición de la fila, que es lo que el `OnClickListener` de una vista no puede dar.
type oyenteItemRegistrado struct {
	oyente any
	vista  any
}

func nuevaMaquina(p *programaDalvik, tabla *tablaRecursos) *maquinaDalvik {
	reloj := int64(0)
	return &maquinaDalvik{prog: p, tabla: tabla, vistos: map[string]bool{},
		vistas: map[uint32]*objetoDalvik{}, almacen: nuevoAlmacenPreferencias(),
		tiposVista: map[uint32]string{}, marcas: map[uint32]bool{}, pistas: map[uint32]string{},
		marcasConocidas: map[uint32]bool{}, gruposRadio: map[uint32][]uint32{},
		grupoDe: map[uint32]uint32{}, reloj: &reloj}
}

// ahora lee el reloj lógico en milisegundos. Sin reloj (una máquina armada a mano en una prueba) el
// tiempo es cero, que es el arranque de la app.
func (m *maquinaDalvik) ahora() int64 {
	if m.reloj == nil {
		return 0
	}
	return *m.reloj
}

// avanza mueve el reloj lógico hacia adelante. Solo lo llama el guion: ninguna tarea puede moverlo
// por su cuenta, y por eso dos ejecuciones del mismo guion dan el mismo JSON. Un avance que no cabe
// en el reloj devuelve error en vez de envolver: un tiempo negativo no es una pantalla que se pueda
// leer, y el comando entero se rechaza como cualquier otra orden que no se puede entregar.
func (m *maquinaDalvik) avanza(ms int64) error {
	if m.reloj == nil {
		return nil
	}
	if ms < 0 || ms > math.MaxInt64-*m.reloj {
		return fmt.Errorf("--avanzar %d desborda el reloj lógico: está en %d ms y el máximo es %d",
			ms, *m.reloj, int64(math.MaxInt64))
	}
	*m.reloj += ms
	return nil
}

// programa encola una tarea para un instante del reloj lógico. No la ejecuta: la cola se drena en
// `vence`, cuando el reloj llega a ese instante.
func (m *maquinaDalvik) programa(instante int64, origen string, r *objetoDalvik, ejecuta func() error) {
	m.encoladas++
	m.cola = append(m.cola, tareaReloj{instante: instante, orden: m.encoladas,
		origen: origen, runnable: r, ejecuta: ejecuta})
}

// quitaTareas retira de la cola las tareas que encoló ese Runnable, que es lo que hacen
// `Handler.removeCallbacks` y su variante con token: no se ejecutan cuando el reloj llega a su
// instante. Deja intactas las que no vienen de un Runnable —las del AsyncTask— porque no son
// callbacks de un Handler. Es una aproximación por Runnable sobre la cola de la pantalla: no hay un
// buzón por Handler, así que también retira un `View.post` del mismo objeto, que la app no mezcla.
func (m *maquinaDalvik) quitaTareas(r *objetoDalvik) {
	if r == nil || len(m.cola) == 0 {
		return
	}
	quedan := m.cola[:0]
	for _, t := range m.cola {
		if t.runnable == r {
			continue
		}
		quedan = append(quedan, t)
	}
	m.cola = quedan
}

// vence ejecuta, si hay alguna, la tarea que toca de las que vencen hasta `limite`: la de menor
// instante y, entre las del mismo instante, la que se encoló antes. Mientras corre, el reloj vale el
// instante de la tarea —no el destino del avance—, porque es la hora que la app lee si vuelve a
// postear. Devuelve falso cuando no queda ninguna tarea vencida.
func (m *maquinaDalvik) vence(limite int64) bool {
	i := -1
	for j := range m.cola {
		if m.cola[j].instante > limite {
			continue
		}
		if i < 0 || m.cola[j].instante < m.cola[i].instante ||
			(m.cola[j].instante == m.cola[i].instante && m.cola[j].orden < m.cola[i].orden) {
			i = j
		}
	}
	if i < 0 {
		return false
	}
	// la salvaguarda solo corta si hay una tarea que ejecutar: con la cola vacía hasta el límite, el
	// reloj puede llegar al destino sin mentir
	if m.tareas >= tareasMaximas {
		m.cortada = true
		m.avisa("la cola del reloj lógico se detuvo a las %d tareas ejecutadas", tareasMaximas)
		return false
	}
	t := m.cola[i]
	m.cola = append(m.cola[:i], m.cola[i+1:]...)
	if m.reloj != nil {
		*m.reloj = t.instante
	}
	m.tareas++
	// cada tarea tiene su propio presupuesto de pasos: el contador es acumulado para publicarlo, pero
	// el límite se mide desde el arranque de esta tarea, así que una cola larga no agota el
	// presupuesto de las siguientes ni deja tics sin ejecutar
	m.pasosBase = m.pasos
	if t.ejecuta != nil {
		if err := t.ejecuta(); err != nil {
			m.avisa("la tarea de %s falló: %v", t.origen, err)
		}
	}
	return true
}

// difiere encola el `run()` de un Runnable, que es lo que hacen `Handler.post`/`postDelayed` y
// `View.post`/`postDelayed`: la tarea corre cuando el guion llega a su instante, no antes. Un
// retraso negativo se trata como cero, como hace Android: no puede adelantar el reloj ni dejar una
// tarea antes del arranque. Un retraso que no cabe en el reloj se avisa y deja la tarea en el último
// instante representable, en vez de envolver a un tiempo pasado que la ejecutaría de inmediato.
func (m *maquinaDalvik) difiere(origen string, runnable any, espera int64) any {
	r, ok := runnable.(*objetoDalvik)
	if !ok {
		m.avisa("%s con un Runnable que no es un objeto del dex: no hay código que ejecutar", origen)
		return nil
	}
	if espera < 0 {
		espera = 0
	}
	instante := m.ahora()
	if espera > math.MaxInt64-instante {
		m.avisa("%s con un retraso que no cabe en el reloj lógico: la tarea queda en %d ms",
			origen, int64(math.MaxInt64))
		instante = math.MaxInt64
	} else {
		instante += espera
	}
	m.programa(instante, origen, r, func() error { return m.ejecutaRunnable(r, origen) })
	return nil
}

// ejecutaRunnable llama al `run()V` de un Runnable del dex. Si el objeto no lo declara, se avisa:
// no se inventa un cuerpo.
func (m *maquinaDalvik) ejecutaRunnable(r *objetoDalvik, origen string) error {
	met := m.buscaMetodo(r.clase, "run", "()V")
	if met == nil {
		m.avisa("%s con un %s que no declara run()V", origen, claseCorta(r.clase))
		return nil
	}
	_, err := m.ejecutaCuerpo(r.clase, met, []any{r}, []string{r.clase})
	return err
}

// difiereTarea encola las tres fases de un AsyncTask en el mismo instante, en el orden que pide el
// plan: `onPreExecute`, `doInBackground` y `onPostExecute`. En el aparato la primera y la última
// corren en el hilo de la interfaz y la del medio en un hilo de trabajo; aquí no hay hilos y el
// orden es el único dato que se puede medir.
func (m *maquinaDalvik) difiereTarea(t *objetoDalvik, params any) {
	m.programa(m.ahora(), "AsyncTask.execute", nil, func() error { return m.ejecutaAsyncTask(t, params) })
}

// ejecutaAsyncTask corre las fases del AsyncTask sobre la misma instancia, pasando el resultado de
// `doInBackground` a `onPostExecute`, que es lo que hace la clase del framework.
func (m *maquinaDalvik) ejecutaAsyncTask(t *objetoDalvik, params any) error {
	if met := m.metodoPorNombre(t.clase, "onPreExecute"); met != nil {
		if _, err := m.ejecutaCuerpo(t.clase, met, []any{t}, []string{t.clase}); err != nil {
			return err
		}
	}
	var resultado any
	met := m.metodoPorNombre(t.clase, "doInBackground")
	if met == nil {
		m.avisa("%s no declara doInBackground: el AsyncTask no hace nada", claseCorta(t.clase))
	} else {
		args, ok := m.argumentosDeAsyncTask(t, met, params)
		if !ok {
			return nil
		}
		res, err := m.ejecutaCuerpo(t.clase, met, args, tiposDeArgumentos(args, met))
		if err != nil {
			return err
		}
		resultado = res
	}
	if met := m.metodoPorNombre(t.clase, "onPostExecute"); met != nil {
		args, ok := m.argumentosDeAsyncTask(t, met, resultado)
		if !ok {
			return nil
		}
		if _, err := m.ejecutaCuerpo(t.clase, met, args, tiposDeArgumentos(args, met)); err != nil {
			return err
		}
	}
	return nil
}

// metodoPorNombre busca un método de la clase del objeto que declare ese nombre, con la firma que
// sea: `AsyncTask` parametriza sus tres métodos y el tipo borrado de cada proyecto cambia.
func (m *maquinaDalvik) metodoPorNombre(clase, nombre string) *metodoDalvik {
	for i := 0; i < 32; i++ {
		cl, ok := m.prog.clases[clase]
		if !ok {
			return nil
		}
		claves := make([]string, 0, len(cl.metodos))
		for k := range cl.metodos {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		for _, k := range claves {
			if met := cl.metodos[k]; met.nombre == nombre {
				return met
			}
		}
		if cl.super == "" || cl.super == clase {
			return nil
		}
		clase = cl.super
	}
	return nil
}

// camposDelProto parte la lista de parámetros de un descriptor: `(Ljava/lang/String;I)V` da
// `Ljava/lang/String;` e `I`. Es lo que dice cuántos argumentos tiene un método cuyo proto no se
// conoce de antemano, como el `doInBackground` de un AsyncTask.
func camposDelProto(proto string) []string {
	if len(proto) < 2 || proto[0] != '(' {
		return nil
	}
	campos := []string{}
	for i := 1; i < len(proto) && proto[i] != ')'; {
		ini := i
		for i < len(proto) && proto[i] == '[' {
			i++
		}
		if i < len(proto) && proto[i] == 'L' {
			for i < len(proto) && proto[i] != ';' {
				i++
			}
		}
		if i < len(proto) {
			i++
		}
		campos = append(campos, proto[ini:i])
	}
	return campos
}

// argumentosDeAsyncTask arma los argumentos de una fase del AsyncTask: el receptor más el valor que
// le toca (el arreglo de parámetros de `execute` o el resultado de `doInBackground`), si su firma
// declara uno. Una firma con más de un parámetro es un `AsyncTask` de varios parámetros, que no está
// en el subconjunto: se avisa y la fase no se ejecuta.
func (m *maquinaDalvik) argumentosDeAsyncTask(t *objetoDalvik, met *metodoDalvik, valor any) ([]any, bool) {
	switch len(camposDelProto(met.proto)) {
	case 0:
		return []any{t}, true
	case 1:
		return []any{t, valor}, true
	}
	m.avisa("el %s de %s declara %d parámetros y el subconjunto solo ejecuta AsyncTask de uno",
		met.nombre, claseCorta(t.clase), len(camposDelProto(met.proto)))
	return nil, false
}

// tiposDeArgumentos arma los tipos de los argumentos de una llamada a una fase del AsyncTask: el
// primero es la clase del objeto y los demás, los del proto del método.
func tiposDeArgumentos(args []any, met *metodoDalvik) []string {
	tipos := make([]string, 0, len(args))
	if len(args) > 0 {
		if obj, ok := args[0].(*objetoDalvik); ok {
			tipos = append(tipos, obj.clase)
		} else {
			tipos = append(tipos, "")
		}
	}
	campos := camposDelProto(met.proto)
	for i := 1; i < len(args); i++ {
		if i-1 < len(campos) {
			tipos = append(tipos, campos[i-1])
		} else {
			tipos = append(tipos, "")
		}
	}
	return tipos
}

// siembraTextosDelLayout deja en la máquina el `android:text` que el layout declara para cada id y
// lo copia a las vistas que ya se habían pedido con `findViewById`: el inflado entrega la vista con
// su texto puesto, no vacía. Solo rellena las que nadie escribió, porque una vista a la que el
// `onCreate` ya le puso texto —aunque sea vacío— no puede volver al del layout.
func (m *maquinaDalvik) siembraTextosDelLayout(textos map[uint32]string) {
	if len(textos) == 0 {
		return
	}
	m.textosDeLayout = textos
	for id, vista := range m.vistas {
		if !vista.textoEscrito {
			vista.texto = textos[id]
		}
	}
}

// siembraMarcas deja en la máquina el estado que el layout declara —el `android:checked` de cada
// vista y el `android:checkedButton` de un RadioGroup— y la forma de cada grupo. No pisa lo que la
// app ya fijó en `onCreate`, con el mismo criterio que la siembra de textos, y no avisa a ningún
// oyente: el inflado tampoco lo hace.
func (m *maquinaDalvik) siembraMarcas(marcas map[uint32]bool, grupos map[uint32][]uint32, tipos map[uint32]string) {
	for id, tipo := range tipos {
		m.tiposVista[id] = tipo
	}
	for grupo, hijos := range grupos {
		m.gruposRadio[grupo] = hijos
		for _, hijo := range hijos {
			m.grupoDe[hijo] = grupo
		}
	}
	for id, valor := range marcas {
		m.marcas[id] = valor
		m.marcasConocidas[id] = true
		if vista := m.vistas[id]; vista != nil && !vista.marcadoPuesto {
			vista.marcado = valor
			vista.marcadoPuesto = true
		}
	}
}

// escribe entrega texto a una vista como si el usuario lo tecleara: lo deja en el EditText, lo
// captura y avisa a los TextWatcher registrados, en el orden del framework (`beforeTextChanged`,
// `onTextChanged` y `afterTextChanged`). El texto llega entero de una vez: el comando no finge un
// IME, un cursor ni una pulsación por carácter.
func (m *maquinaDalvik) escribe(id uint32, texto string) {
	vista := m.vista(id)
	anterior := vista.texto
	m.captura("usuario.escribir", vista, texto)
	m.avisaTecleo(vista, anterior, texto)
}

// avisaTecleo llama a los tres métodos de cada TextWatcher registrado, si el proyecto los declara.
// El texto nuevo viaja como cadena del intérprete, que es lo que un `toString` o un `length` de dentro
// del vigilante leen, y los enteros llevan los recuentos de antes y de después. Un `setText` que el
// propio vigilante haga sobre la misma vista no vuelve a disparar el aviso: el comando no finge una
// cascada que en el aparato depende de las marcas del Editable.
func (m *maquinaDalvik) avisaTecleo(v *objetoDalvik, anterior, nuevo string) {
	if len(v.vigilantesTexto) == 0 {
		return
	}
	antes := int32(len(unidadesDeCadena(anterior)))
	despues := int32(len(unidadesDeCadena(nuevo)))
	for _, vigilante := range v.vigilantesTexto {
		obj, ok := vigilante.(*objetoDalvik)
		if !ok {
			m.avisa("un TextWatcher de %#08x no es un objeto del dex", v.id)
			continue
		}
		s := &objetoDalvik{clase: "Ljava/lang/CharSequence;", campos: map[string]any{}, texto: nuevo}
		// beforeTextChanged(CharSequence s, int start, int count, int after)
		m.llamaVigilante(obj, "beforeTextChanged", "(Ljava/lang/CharSequence;III)V",
			[]any{obj, s, int32(0), antes, despues},
			[]string{obj.clase, "Ljava/lang/CharSequence;", "I", "I", "I"})
		// onTextChanged(CharSequence s, int start, int before, int count)
		m.llamaVigilante(obj, "onTextChanged", "(Ljava/lang/CharSequence;III)V",
			[]any{obj, s, int32(0), antes, despues},
			[]string{obj.clase, "Ljava/lang/CharSequence;", "I", "I", "I"})
		// afterTextChanged(Editable s)
		m.llamaVigilante(obj, "afterTextChanged", "(Landroid/text/Editable;)V",
			[]any{obj, s}, []string{obj.clase, "Landroid/text/Editable;"})
	}
}

// llamaVigilante ejecuta el método si el objeto lo declara. Un vigilante puede implementar solo
// alguno de los tres (es una interfaz con métodos vacíos por defecto en Kotlin y una clase abstracta
// en Java, pero aquí llegan los tres como métodos normales): el que falte no se inventa.
func (m *maquinaDalvik) llamaVigilante(obj *objetoDalvik, nombre, proto string, args []any, tipos []string) {
	met := m.buscaMetodo(obj.clase, nombre, proto)
	if met == nil {
		return
	}
	if _, err := m.ejecutaCuerpo(obj.clase, met, args, tipos); err != nil {
		m.avisa("el %s de %s falló: %v", nombre, claseCorta(obj.clase), err)
	}
}

// marcaVista fija el estado de una vista marcable como `CompoundButton.setChecked`: si el valor
// cambia, avisa a su OnCheckedChangeListener. Marcar un RadioButton desmarca el resto de su grupo,
// que es lo que hace un RadioGroup. No cuenta como interacción por sí sola: `--marcar` la cuenta y un
// `setChecked` de la app no, igual que el framework.
func (m *maquinaDalvik) marcaVista(v *objetoDalvik, valor bool) {
	if v == nil {
		return
	}
	// el aviso sale solo si el valor cambia, como en `CompoundButton.setChecked`: desmarcar lo que ya
	// estaba desmarcado no dispara nada
	cambia := v.marcado != valor
	v.marcado = valor
	v.marcadoPuesto = true
	if v.id != 0 {
		m.marcas[v.id] = valor
		m.marcasConocidas[v.id] = true
	}
	if !cambia {
		return
	}
	if valor && v.id != 0 {
		if grupo, ok := m.grupoDe[v.id]; ok {
			for _, otroID := range m.gruposRadio[grupo] {
				if otroID == v.id {
					continue
				}
				if m.marcas[otroID] {
					// el grupo desmarca al que estaba: su oyente también recibe el cambio
					m.marcaVista(m.vista(otroID), false)
				}
			}
		}
	}
	m.avisaMarca(v, valor)
}

// avisaMarca entrega el cambio al OnCheckedChangeListener registrado, con la firma de
// CompoundButton.OnCheckedChangeListener. Si el proyecto declara otra cosa, se avisa en vez de
// inventar una llamada.
func (m *maquinaDalvik) avisaMarca(v *objetoDalvik, valor bool) {
	if v.oyenteMarca == nil {
		return
	}
	oyente, ok := v.oyenteMarca.(*objetoDalvik)
	if !ok {
		m.avisa("el OnCheckedChangeListener de %#08x no es un objeto del dex", v.id)
		return
	}
	met := m.buscaMetodo(oyente.clase, "onCheckedChanged", "(Landroid/widget/CompoundButton;Z)V")
	if met == nil {
		m.avisa("%s se registró como OnCheckedChangeListener y no tiene "+
			"onCheckedChanged(CompoundButton, boolean)", claseCorta(oyente.clase))
		return
	}
	if _, err := m.ejecutaCuerpo(oyente.clase, met, []any{oyente, v, boolAInt(valor)},
		[]string{oyente.clase, "Landroid/widget/CompoundButton;", "Z"}); err != nil {
		m.avisa("el onCheckedChanged de %s falló: %v", claseCorta(oyente.clase), err)
	}
}

// marcasDeVista publica el estado final de las vistas marcables conocidas, en orden de id para que
// el JSON sea determinista.
func (m *maquinaDalvik) marcasDeVista() []marcaVista {
	ids := make([]uint32, 0, len(m.marcasConocidas))
	for id := range m.marcasConocidas {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	salida := make([]marcaVista, 0, len(ids))
	for _, id := range ids {
		valor := m.marcas[id]
		if v := m.vistas[id]; v != nil && v.marcadoPuesto {
			valor = v.marcado
		}
		entrada := marcaVista{IDRef: fmt.Sprintf("%#08x", id), Marcado: valor}
		if tipo, clave, ok := m.nombreDeID(id); ok {
			entrada.ID = tipo + "/" + clave
		}
		salida = append(salida, entrada)
	}
	return salida
}

// pistasDeVista publica los `setHint` que la app puso, en orden de id.
func (m *maquinaDalvik) pistasDeVista() []pistaVista {
	ids := make([]uint32, 0, len(m.pistas))
	for id := range m.pistas {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	salida := make([]pistaVista, 0, len(ids))
	for _, id := range ids {
		entrada := pistaVista{IDRef: fmt.Sprintf("%#08x", id), Pista: m.pistas[id]}
		if tipo, clave, ok := m.nombreDeID(id); ok {
			entrada.ID = tipo + "/" + clave
		}
		salida = append(salida, entrada)
	}
	return salida
}

// nombreDeID traduce un id de recurso a su tipo y su clave con la tabla del apk; sin tabla no hay
// nombre que dar y el resultado se queda con el número.
func (m *maquinaDalvik) nombreDeID(id uint32) (string, string, bool) {
	if m.tabla == nil {
		return "", "", false
	}
	return m.tabla.nombreDeId(id)
}

// vista devuelve la vista que ese id ya tiene o la crea como la dejaría el inflado: la clase genérica
// de View, el `android:text` que el layout declare para ese id, y el estado marcado o la pista que ya
// se conozcan. El id 0 no se registra: no es una vista del layout y no se puede volver a encontrar.
func (m *maquinaDalvik) vista(id uint32) *objetoDalvik {
	if id == 0 {
		return &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{}}
	}
	if v := m.vistas[id]; v != nil {
		return v
	}
	v := &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{}, id: id,
		texto: m.textosDeLayout[id], marcado: m.marcas[id], marcadoPuesto: m.marcasConocidas[id],
		pista: m.pistas[id]}
	if m.vistas == nil {
		m.vistas = map[uint32]*objetoDalvik{}
	}
	m.vistas[id] = v
	return v
}

// vistaDeInstancia devuelve la vista con ese id DENTRO de una vista inflada: es lo que hace
// `fila.findViewById(R.id.nombre)` en el `getView`, donde cada fila tiene sus propias vistas y no
// las comparte con las demás ni con la pantalla.
func (m *maquinaDalvik) vistaDeInstancia(padre *objetoDalvik, id uint32) *objetoDalvik {
	if padre == nil || padre.vistas == nil {
		return m.vista(id)
	}
	if id == 0 {
		return &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{}}
	}
	if v := padre.vistas[id]; v != nil {
		return v
	}
	v := &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{}, id: id}
	padre.vistas[id] = v
	return v
}

func (m *maquinaDalvik) avisa(formato string, args ...any) {
	texto := fmt.Sprintf(formato, args...)
	if m.vistos[texto] || len(m.avisos) >= avisosMaximos {
		return
	}
	m.vistos[texto] = true
	m.avisos = append(m.avisos, texto)
}

// captura anota una escritura de texto. El id se resuelve contra la tabla para que el modelo diga
// `id/total` y no un número; sin tabla se queda su número, que es lo que hay.
func (m *maquinaDalvik) captura(origen string, obj *objetoDalvik, texto string) {
	c := capturaTexto{Origen: origen, Texto: texto}
	if obj != nil {
		obj.texto = texto
		obj.textoEscrito = true
		if obj.id != 0 {
			c.IDRef = fmt.Sprintf("%#08x", obj.id)
			if m.tabla != nil {
				if tipo, clave, ok := m.tabla.nombreDeId(obj.id); ok {
					c.ID = tipo + "/" + clave
				}
			}
		}
	}
	if esOrigenDeConsola(origen) {
		m.anotaConsola(origen, texto)
	}
	// Sin tope: cada escritura se conserva, como en un dispositivo. El estado que ve el usuario
	// siempre es el último, y el registro permite reconstruir toda la ejecución.
	m.capturas = append(m.capturas, c)
}

// esOrigenDeConsola separa lo que escribe la app de lo que escribe en una view: `print`, `println` y
// `Log.*` son la salida estándar de la app y van también al panel de consola. El registro del
// emulador los conserva, porque ahí se ve todo en el orden en que ocurrió.
func esOrigenDeConsola(origen string) bool {
	switch origen {
	case "print", "println", "log":
		return true
	}
	return strings.HasPrefix(origen, "Log.")
}

// anotaConsola apila una línea con su secuencia de emisión. No se inventa la hora: el orden es lo
// que se puede medir dentro de una ejecución y lo que hace determinista el JSON.
func (m *maquinaDalvik) anotaConsola(origen, texto string) {
	m.consola = append(m.consola, lineaConsola{Origen: origen, Texto: texto, Secuencia: len(m.consola) + 1})
}

// textoDeId resuelve un id de recurso de tipo cadena contra la tabla: es lo que el framework hace
// con `@string/x`. El segundo valor dice si el id era de verdad una cadena de este paquete; con
// otro tipo (o sin tabla) no hay texto que devolver.
func (m *maquinaDalvik) textoDeId(v any) (string, bool) {
	id, ok := v.(int32)
	if !ok || m.tabla == nil {
		return "", false
	}
	enlace, ok := m.tabla.valorDeId(uint32(id))
	if !ok || enlace.tipo != tipoCadena || int(enlace.datos) >= len(m.tabla.global.cadenas) {
		return "", false
	}
	return m.tabla.global.cadenas[enlace.datos], true
}

// textoDe es cómo un stub imprime un valor: lo que haría `String.valueOf`.
func textoDe(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case *objetoDalvik:
		// una cadena del intérprete se imprime vacía si está vacía: devolver el nombre de la clase
		// («java.lang.String») es lo que hacía que un `"" + getText()` se leyera como la clase
		if x.clase == "Ljava/lang/String;" || x.clase == "Ljava/lang/CharSequence;" {
			return x.texto
		}
		if x.texto != "" {
			return x.texto
		}
		return claseCorta(x.clase)
	case *arregloDalvik:
		return x.tipo
	}
	return fmt.Sprint(v)
}

// claseCorta convierte un descriptor en el nombre con el que se lee un aviso.
func claseCorta(descriptor string) string {
	s := strings.TrimSuffix(strings.TrimPrefix(descriptor, "L"), ";")
	return strings.ReplaceAll(s, "/", ".")
}

// cadenaDe devuelve el texto de un receptor de cadena: un literal del dex o un objeto String que
// devolvió un stub (`getText`, `getString`, `toString`). Es lo que comparten `length`, `charAt`,
// `equals` e `isEmpty`.
func cadenaDe(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case *objetoDalvik:
		if x.clase == "Ljava/lang/String;" || x.clase == "Ljava/lang/CharSequence;" {
			return x.texto, true
		}
	}
	return "", false
}

// unidadesDeCadena es la cadena en unidades de 16 bits, que es lo que cuentan `length()` y
// `charAt()`: ni bytes ni runes. «á» es una unidad y dos bytes, y un emoji son dos unidades.
func unidadesDeCadena(s string) []uint16 { return utf16.Encode([]rune(s)) }

// claseDeLiteral lee el descriptor de un `X.class`: el `const-class` del intérprete guarda el tipo
// en el campo `$tipo` del objeto de `java.lang.Class`, que es lo que el constructor de `Intent`
// necesita para saber a qué actividad apunta.
func claseDeLiteral(v any) string {
	o, ok := v.(*objetoDalvik)
	if !ok || o.clase != "Ljava/lang/Class;" {
		return ""
	}
	tipo, _ := o.campos["Ljava/lang/Class;.$tipo"].(string)
	return tipo
}

// esReflexion dice si alguno de los candidatos es de reflexión (java.lang.Class o su paquete
// java.lang.reflect): ahí el intérprete no puede saber qué se va a llamar, así que se rechaza por
// nombre.
func esReflexion(candidatos []string) bool {
	for _, c := range candidatos {
		if c == "Ljava/lang/Class;" || strings.HasPrefix(c, "Ljava/lang/reflect/") {
			return true
		}
	}
	return false
}

// ejecutaOnCreate carga el programa y corre la actividad, sin pulsar nada. Es el camino de siempre:
// el emulador arranca en el estado que deja `onCreate` y espera un clic de verdad.
func ejecutaOnCreate(d *dexFile, tabla *tablaRecursos, actividad string) (*resultadoEjecucion, uint32) {
	res, layoutID, _ := ejecutaOnCreateConPulsos(d, tabla, actividad, nil)
	return res, layoutID
}

// ejecutaOnCreateConPulsos es el mismo arranque con una lista de clics: cada entrada se entrega al
// `onClick` de la vista con ese id, en orden y en la MISMA ejecución. Devolver un error fuera del
// resultado es solo para el guion (un id que no existe, una vista sin listener o una orden que no le
// corresponde): la app ejecutada a medias no se devuelve como si fuera una pantalla buena.
func ejecutaOnCreateConPulsos(d *dexFile, tabla *tablaRecursos, actividad string, pulsos []string) (*resultadoEjecucion, uint32, error) {
	eventos := make([]eventoUsuario, 0, len(pulsos))
	for _, p := range pulsos {
		eventos = append(eventos, eventoUsuario{Tipo: "pulsar", Dato: p})
	}
	return ejecutaOnCreateConEventos(d, tabla, actividad, eventos, nil, nil)
}

// ejecutaOnCreatePreparando es el mismo arranque con un gancho entre `onCreate` y los clics. Lo usa
// `apkc preview` para registrar los oyentes declarativos del layout (`android:onClick`): el
// framework los enlaza al inflar, es decir, después de que la app pida su `setContentView` y antes
// de que llegue ningún toque. Sin lista de actividades declaradas, cualquier clase del dex puede ser
// destino de `startActivity` (es el camino de las pruebas que no leen manifiesto).
func ejecutaOnCreatePreparando(d *dexFile, tabla *tablaRecursos, actividad string, pulsos []string,
	preparar func(m *maquinaDalvik)) (*resultadoEjecucion, uint32, error) {
	eventos := make([]eventoUsuario, 0, len(pulsos))
	for _, p := range pulsos {
		eventos = append(eventos, eventoUsuario{Tipo: "pulsar", Dato: p})
	}
	return ejecutaOnCreateConEventos(d, tabla, actividad, eventos, preparar, nil)
}

// ejecutaOnCreateNavegando es el nombre anterior del arranque con navegación: se queda como
// envoltorio fino para las llamadas que solo piden entregar el guion.
func ejecutaOnCreateNavegando(d *dexFile, tabla *tablaRecursos, actividad string, eventos []eventoUsuario,
	preparar func(m *maquinaDalvik), declaradas map[string]bool) (*resultadoEjecucion, uint32, error) {
	return ejecutaOnCreateConEventos(d, tabla, actividad, eventos, preparar, declaradas)
}

// ejecutaOnCreateConEventos es el arranque con el guion de entrada completo —clics, texto tecleado,
// marcas, desmarcas, recreaciones y avances del reloj lógico—, en el orden en que se pidieron, con
// ciclo de vida y navegación.
//
// `eventos` es el guion del emulador, en orden: cada `--pulsar` entrega un clic y cada `--avanzar N`
// mueve el reloj lógico y drena las tareas diferidas que vencen. El orden entre unos y otros es el
// de la línea de comandos, para que el mismo guion reproduzca el mismo JSON.
//
// La pila de actividades vive aquí y no en el dex: cada pantalla conserva su propia máquina (sus
// objetos, sus capturas y sus oyentes) mientras está cubierta, y `finish()` la descubre otra vez sin
// repetir su `onCreate`, que es lo que hace que el estado vuelva como estaba. `declaradas` son las
// actividades que el manifiesto declara, por descriptor: una que no esté ahí no se lanza, como en el
// aparato.
func ejecutaOnCreateConEventos(d *dexFile, tabla *tablaRecursos, actividad string, eventos []eventoUsuario,
	preparar func(m *maquinaDalvik), declaradas map[string]bool) (*resultadoEjecucion, uint32, error) {
	res := &resultadoEjecucion{Metodo: "onCreate"}
	prog, err := cargaPrograma(d)
	if err != nil {
		res.falla(err)
		return res, 0, nil
	}
	m := nuevaMaquina(prog, tabla)
	m.declaradas = declaradas
	ciclo := []string{}
	m.registro = &ciclo
	clase, conMain := eligeClaseLanzadora(prog, actividad)
	if clase == "" {
		res.Error = "el dex no trae ninguna clase con main ni onCreate"
		return res, 0, nil
	}
	res.Actividad = claseCorta(clase)

	// todas guarda cada máquina que se abrió, incluida la que `finish()` dejó atrás: el resultado
	// suma sus pasos y sus avisos, pero las capturas y los oyentes son los de la pantalla actual.
	todas := []*maquinaDalvik{}
	pila := []*maquinaDalvik{m}

	if conMain {
		res.Metodo = "main"
		cl := prog.clases[clase]
		m.actividad = &objetoDalvik{clase: clase, campos: map[string]any{}}
		if ctor := cl.metodos["<init>()V"]; ctor != nil {
			if _, err := m.ejecutaCuerpo(clase, ctor, []any{m.actividad}, []string{clase}); err != nil {
				res.falla(err)
			}
		}
		if res.Error == "" {
			main := cl.metodos["main([Ljava/lang/String;)V"]
			vacio := &arregloDalvik{tipo: "[Ljava/lang/String;", datos: []any{}}
			if _, err := m.ejecutaCuerpo(clase, main, []any{vacio}, []string{"[Ljava/lang/String;"}); err != nil {
				res.falla(err)
			}
		}
	} else {
		if err := m.abreActividad(clase, preparar, false); err != nil {
			res.falla(err)
			res.Pasos = m.pasos
			res.Capturas = m.capturas
			res.Consola = m.consola
			res.Avisos = m.avisos
			res.Marcas = m.marcasDeVista()
			res.Pistas = m.pistasDeVista()
			res.Ciclo = m.ciclo
			res.maquina = m
			return res, m.layoutID, nil
		}
		res.Pantallas = append(res.Pantallas, res.Actividad)
	}
	todas = append(todas, m)

	// estabiliza lleva la app al estado que pide su código: abre la actividad que dejó pendiente un
	// `startActivity` y vuelve a la anterior si alguien llamó a `finish()`. Se repite hasta que no
	// quede nada pendiente, porque una pantalla nueva puede navegar a otra en su propio `onCreate`.
	estabiliza := func() {
		for {
			if m.pendiente != "" {
				destino := m.pendiente
				m.pendiente = ""
				if len(todas) >= pantallasMaximas {
					// el tope simétrico de los clics: una navegación circular se corta aquí en vez
					// de encadenar pantallas sin fin; se avisa y se queda delante la última viva
					m.avisa("startActivity a %s no se abrió: el tope de %d pantallas", claseCorta(destino),
						pantallasMaximas)
					continue
				}
				if _, ok := prog.clases[destino]; !ok {
					m.avisa("startActivity a %s: la clase no está en el dex", claseCorta(destino))
					continue
				}
				if m.declaradas != nil && !m.declaradas[destino] {
					m.avisa("startActivity a %s: no está declarada en el manifiesto", claseCorta(destino))
					continue
				}
				m.pausa()
				nueva := nuevaMaquina(prog, tabla)
				nueva.declaradas = m.declaradas
				nueva.registro = m.registro
				nueva.almacen = m.almacen
				nueva.reloj = m.reloj
				nueva.intent = m.intentoSaliente
				if err := nueva.abreActividad(destino, preparar, true); err != nil {
					m.avisa("la actividad %s no arrancó: %v", claseCorta(destino), err)
					m.reanuda()
					continue
				}
				todas = append(todas, nueva)
				pila = append(pila, nueva)
				m = nueva
				res.Pantallas = append(res.Pantallas, claseCorta(destino))
				continue
			}
			if m.terminada {
				m.terminada = false
				m.cierra()
				if len(pila) > 1 {
					pila = pila[:len(pila)-1]
					m = pila[len(pila)-1]
					m.reanuda()
					res.Pantallas = append(res.Pantallas, claseCorta(m.actividad.clase))
					continue
				}
				m.avisa("finish() en la única actividad: la app se queda sin pantalla")
			}
			return
		}
	}
	estabiliza()

	// recreaActual mata y vuelve a crear la actividad que está delante con su Bundle, como una
	// rotación: guarda el estado, ejecuta la secuencia de cierre y arranca una máquina nueva que
	// recibe ese Bundle no nulo. Las actividades cubiertas no se tocan: la pila conserva su sitio y
	// solo la recreada repite su `onCreate`.
	recreaActual := func() error {
		if m.actividad == nil || m.actividad.clase == "" {
			return fmt.Errorf("--recrear sin actividad que recrear")
		}
		claseRecreada := m.actividad.clase
		bundle := m.despideParaRecrear()
		nueva := nuevaMaquina(prog, tabla)
		nueva.declaradas = m.declaradas
		nueva.registro = m.registro
		nueva.almacen = m.almacen
		nueva.reloj = m.reloj
		nueva.intent = m.intent
		nueva.bundleInicial = bundle
		nueva.estadoGuardado = m.estadoGuardado
		if err := nueva.abreActividad(claseRecreada, preparar, true); err != nil {
			m.avisa("la recreación de %s no arrancó: %v", claseCorta(claseRecreada), err)
			return fmt.Errorf("--recrear de %s: %w", claseCorta(claseRecreada), err)
		}
		todas = append(todas, nueva)
		if len(pila) == 0 {
			pila = append(pila, nueva)
		} else {
			pila[len(pila)-1] = nueva
		}
		m = nueva
		res.Pantallas = append(res.Pantallas, claseCorta(claseRecreada))
		return nil
	}

	// el guion pedido, en orden y solo si la app llegó viva a este punto. El estado que ve cada orden
	// incluye las anteriores, porque comparten la misma máquina: es lo que hace que
	// `--pulsar a --pulsar a` deje el contador en 2 y no en 1, y que teclear antes de pulsar cambie lo
	// que el onClick lee. Entre orden y orden se estabiliza la navegación —la siguiente va a la
	// pantalla que quedó delante— y se drenan las tareas diferidas que ya vencieron. Ni los clics ni
	// los tecleos ni las marcas ni los avances del reloj tienen tope: los pone el usuario y el
	// emulador los entrega todos.
	var errEventos error
	if res.Error == "" {
		// drena corre, por orden de instante, las tareas que vencen hasta `limite`: primero la de
		// menor instante y, a igual instante, la que se encoló antes. Se repite porque una tarea
		// puede navegar a otra pantalla: la nueva trae su propia cola y es esa la que sigue. Al
		// terminar, el reloj queda en el límite, que es la hora que pidió el guion; si la cola se
		// cortó por la salvaguarda, se queda en el instante de la última tarea ejecutada.
		drena := func(limite int64) {
			for m.vence(limite) {
				estabiliza()
			}
			if !m.cortada && m.ahora() < limite && m.reloj != nil {
				*m.reloj = limite
			}
		}
		// lo que la app posteó en el arranque sin retraso corre antes de cualquier orden
		drena(m.ahora())
		for _, orden := range eventos {
			if orden.Tipo == ordenIntercambiarTipo {
				// el hot swap del guion: el dex nuevo entra en la tabla de despacho de la sesión
				// viva, sin recrear la actividad ni sus objetos. Va donde se escribió, así que un
				// clic anterior ve el cuerpo viejo y uno posterior el nuevo.
				nuevo, err := readDex(orden.Dex)
				if err != nil {
					errEventos = fmt.Errorf("--intercambiar %s: %w", orden.Dato, err)
					break
				}
				cambios, err := deltaDeDexes(d, nuevo)
				if err != nil {
					errEventos = fmt.Errorf("--intercambiar %s: %w", orden.Dato, err)
					break
				}
				hechos, err := m.aplicaDelta(cambios, nuevo)
				if err != nil {
					errEventos = fmt.Errorf("--intercambiar %s: %w", orden.Dato, err)
					break
				}
				// la base del siguiente delta es el dex recién instalado; en fallo no se tocó
				d = nuevo
				res.Intercambios = append(res.Intercambios, hechos...)
				estabiliza()
				drena(m.ahora())
				continue
			}
			if orden.Tipo == ordenAvanzarTipo {
				ms, err := strconv.ParseInt(orden.Dato, 10, 64)
				if err != nil || ms < 0 {
					errEventos = fmt.Errorf("--avanzar %q no son milisegundos enteros no negativos",
						orden.Dato)
					break
				}
				if err := m.avanza(ms); err != nil {
					errEventos = err
					break
				}
				drena(m.ahora())
				continue
			}
			if orden.Tipo == ordenRecrearTipo {
				if err := recreaActual(); err != nil {
					errEventos = err
					break
				}
				estabiliza()
				drena(m.ahora())
				continue
			}
			pedido := orden.Dato
			if orden.Tipo == ordenPulsarTipo && esOrdenDeDialogo(pedido) {
				if err := m.pulsaBotonDeDialogo(pedido); err != nil {
					errEventos = err
					break
				}
				res.Pulsaciones = append(res.Pulsaciones, pedido)
				estabiliza()
				drena(m.ahora())
				continue
			}
			// `id/lista:1` es un clic en una fila: la forma lleva la posición después de los dos
			// puntos, y es la única que le entrega al `onItemClick` el número que tocó.
			if vista, fila, esFila := cortaFila(pedido); orden.Tipo == ordenPulsarTipo && esFila {
				id, err := idDeEvento(tabla, ordenPulsarTipo, vista)
				if err != nil {
					errEventos = err
					break
				}
				if !m.pulsaFila(id, fila) {
					errEventos = fmt.Errorf("no se puede pulsar la fila %d de %q: ninguna lista con ese "+
						"id registró un OnItemClickListener%s", fila, vista, listaDeOyentesDeItem(m))
					break
				}
				res.Pulsaciones = append(res.Pulsaciones, pedido)
				estabiliza()
				drena(m.ahora())
				continue
			}
			if err := m.entregaEvento(tabla, orden, res); err != nil {
				errEventos = err
				break
			}
			estabiliza()
			drena(m.ahora())
		}
	}

	// el resultado es el de la pantalla actual, con la suma de lo que hicieron las que se visitaron
	res.Actividad = claseCorta(m.actividad.clase)
	res.Capturas = m.capturas
	res.Oyentes = m.oyentesDeVista()
	res.Marcas = m.marcasDeVista()
	res.Pistas = m.pistasDeVista()
	for _, x := range todas {
		res.Pasos += x.pasos
		res.Interacciones += x.interaccion
		res.Tareas += x.tareas
		res.Consola = append(res.Consola, x.consola...)
		for _, a := range x.avisos {
			if !contieneTexto(res.Avisos, a) {
				res.Avisos = append(res.Avisos, a)
			}
		}
	}
	res.Reloj = m.ahora()
	res.Ciclo = ciclo
	// el estado que sobrevive a la ejecución se publica tal cual: el Bundle de la última recreación
	// y el SharedPreferences de toda la ejecución, sin inventar lo que la app no escribió
	if m.estadoGuardado != nil {
		res.Bundle = m.estadoGuardado
	}
	res.Preferencias = m.almacen.volcado()
	// la máquina que quedó delante es la que sabe qué listas tienen adaptador: el modelo se la
	// pide para ejecutar cada `getView` cuando ya se entregaron los clics
	res.maquina = m
	// la capa de diálogos y el último aviso efímero son los de la pantalla que quedó delante
	res.Dialogos = m.dialogosDeVista()
	res.Menu = m.menuVista()
	res.Toast = m.toast
	// la consola se renumera al unir las pantallas: `secuencia` es el orden real de emisión
	for i := range res.Consola {
		res.Consola[i].Secuencia = i + 1
	}
	return res, m.layoutID, errEventos
}

// entregaEvento ejecuta una orden de entrada sobre la pantalla que está delante. Cada tipo comprueba
// que la vista exista y que la orden le corresponda: un id que no resuelve, un clic sin oyente, texto
// a una vista que no es un EditText o una marca sobre algo que no se marca devuelven error, porque un
// modelo a medias se leería como si la orden hubiera funcionado.
func (m *maquinaDalvik) entregaEvento(tabla *tablaRecursos, e eventoUsuario, res *resultadoEjecucion) error {
	switch e.Tipo {
	case "pulsar":
		id, err := idDeEvento(tabla, e.Tipo, e.Dato)
		if err != nil {
			return err
		}
		if !m.pulsa(id) {
			return fmt.Errorf("no se puede pulsar %q: ninguna vista con ese id se registró "+
				"como OnClickListener%s", e.Dato, listaDeOyentes(m))
		}
		res.Pulsaciones = append(res.Pulsaciones, e.Dato)
	case "escribir":
		idParte, texto := partirEscribir(e.Dato)
		id, err := idDeEvento(tabla, e.Tipo, idParte)
		if err != nil {
			return err
		}
		if tipo := m.tiposVista[id]; tipo != "" && !admiteTecleo(tipo) {
			return fmt.Errorf("no se puede escribir en %q: el layout dice %s y solo se teclea en un "+
				"EditText", idParte, tipo)
		}
		m.interaccion++
		m.escribe(id, texto)
		res.Escrituras = append(res.Escrituras, e.Dato)
	case "marcar", "desmarcar":
		id, err := idDeEvento(tabla, e.Tipo, e.Dato)
		if err != nil {
			return err
		}
		if tipo := m.tiposVista[id]; tipo != "" && !esTipoMarcable(tipo) {
			return fmt.Errorf("no se puede %s %q: el layout dice %s y solo se marcan CheckBox, "+
				"RadioButton o Switch", e.Tipo, e.Dato, tipo)
		}
		m.interaccion++
		m.marcaVista(m.vista(id), e.Tipo == "marcar")
	case ordenMenuTipo:
		// desplegar el menú de opciones: construye lo que la app añada con `menu.add` y deja los
		// ítems en el modelo; el clic de cada uno llega después con `--menu-item`
		if err := m.abreMenuDeOpciones(); err != nil {
			return err
		}
	case ordenMenuItemTipo:
		if err := m.pulsaItemDeMenu(e.Dato); err != nil {
			return err
		}
	}
	return nil
}

// contieneTexto dice si la lista ya tiene ese aviso, para no repetirlo al unir pantallas.
func contieneTexto(lista []string, texto string) bool {
	for _, s := range lista {
		if s == texto {
			return true
		}
	}
	return false
}

// abreActividad crea la instancia, ejecuta su constructor, su `onCreate` y —si no navegó ni se
// cerró en él— su `onStart` y su `onResume`. `exigirLayout` distingue la lanzadora (que puede
// quedarse sin `setContentView` y usar el layout que pidió el comando) de una actividad abierta por
// `startActivity`, que sin layout propio no tiene nada que enseñar.
func (m *maquinaDalvik) abreActividad(clase string, preparar func(*maquinaDalvik), exigirLayout bool) error {
	cl, ok := m.prog.clases[clase]
	if !ok {
		return fmt.Errorf("la actividad %s no está en el dex", claseCorta(clase))
	}
	obj := &objetoDalvik{clase: clase, campos: map[string]any{}}
	m.actividad = obj
	if ctor := cl.metodos["<init>()V"]; ctor != nil {
		if _, err := m.ejecutaCuerpo(clase, ctor, []any{obj}, []string{clase}); err != nil {
			return err
		}
	}
	onCreate := cl.metodos["onCreate(Landroid/os/Bundle;)V"]
	if onCreate == nil {
		return fmt.Errorf("%s no tiene onCreate(Landroid/os/Bundle;)V", claseCorta(clase))
	}
	// en una recreación el framework entrega un Bundle no nulo; en el arranque normal, null
	var savedInstanceState any
	if m.bundleInicial != nil {
		savedInstanceState = m.bundleInicial
	}
	m.anotaCiclo("onCreate")
	if _, err := m.ejecutaCuerpo(clase, onCreate, []any{obj, savedInstanceState},
		[]string{clase, "Landroid/os/Bundle;"}); err != nil {
		return err
	}
	// una actividad abierta por `startActivity` que no llama a `setContentView` no tiene nada que
	// enseñar: en el aparato se queda en blanco. Sin este rechazo el modelo caía al layout `main`
	// de la lanzadora y la pantalla mentía. `estabiliza` ya avisa con este error y reanuda la
	// actividad anterior. No es un defecto si la pantalla navega o se cierra en su `onCreate`
	// (un redirect de arranque): en ese caso nunca llega a pintarse y la pila sigue su curso.
	if exigirLayout && m.layoutID == 0 && m.pendiente == "" && !m.terminada {
		return fmt.Errorf("%s no llamó a setContentView: una actividad abierta no hereda el layout "+
			"de la lanzadora", claseCorta(clase))
	}
	// el layout ya se sabe (lo dijo `setContentView`): sus `android:onClick` se enlazan aquí, como
	// haría el inflado, antes de que la app lea nada en `onStart` o `onResume`
	if preparar != nil && (!exigirLayout || m.layoutID != 0) {
		preparar(m)
	}
	if m.pendiente == "" && !m.terminada {
		if err := m.cicloDe("onStart", "()V"); err != nil {
			return err
		}
		// el framework llama a `onRestoreInstanceState` entre `onStart` y `onResume`, con el mismo
		// Bundle que recibió `onCreate`
		if m.bundleInicial != nil {
			if err := m.cicloDeCon("onRestoreInstanceState", "(Landroid/os/Bundle;)V",
				[]any{m.bundleInicial}, []string{"Landroid/os/Bundle;"}); err != nil {
				return err
			}
		}
		if err := m.cicloDe("onResume", "()V"); err != nil {
			return err
		}
		m.arrancada = true
	}
	return nil
}

// cicloDe llama a un método del ciclo de vida si la actividad lo declara. No lo inventa: si no está
// en el dex, no se anota ni se ejecuta.
func (m *maquinaDalvik) cicloDe(nombre, proto string) error {
	return m.cicloDeCon(nombre, proto, nil, nil)
}

// cicloDeCon es el mismo ciclo con argumentos extra, para `onRestoreInstanceState(Bundle)`.
func (m *maquinaDalvik) cicloDeCon(nombre, proto string, extra []any, tipos []string) error {
	if m.actividad == nil {
		return nil
	}
	met := m.buscaMetodo(m.actividad.clase, nombre, proto)
	if met == nil {
		return nil
	}
	m.anotaCiclo(nombre)
	args := append([]any{m.actividad}, extra...)
	argTipos := append([]string{m.actividad.clase}, tipos...)
	if _, err := m.ejecutaCuerpo(m.actividad.clase, met, args, argTipos); err != nil {
		return err
	}
	return nil
}

func (m *maquinaDalvik) anotaCiclo(nombre string) {
	clase := ""
	if m.actividad != nil {
		clase = claseCorta(m.actividad.clase) + "."
	}
	m.ciclo = append(m.ciclo, clase+nombre)
	if m.registro != nil {
		*m.registro = append(*m.registro, clase+nombre)
	}
}

// pausa deja la actividad cubierta por otra: `onPause` y `onStop`, si los declara.
func (m *maquinaDalvik) pausa() {
	if !m.arrancada {
		return
	}
	if err := m.cicloDe("onPause", "()V"); err != nil {
		m.avisa("onPause de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
	if err := m.cicloDe("onStop", "()V"); err != nil {
		m.avisa("onStop de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
	m.arrancada = false
}

// reanuda vuelve a poner delante una actividad que estaba cubierta: `onRestart`, `onStart` y
// `onResume`, si los declara.
func (m *maquinaDalvik) reanuda() {
	if m.actividad == nil || m.arrancada {
		return
	}
	if err := m.cicloDe("onRestart", "()V"); err != nil {
		m.avisa("onRestart de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
	if err := m.cicloDe("onStart", "()V"); err != nil {
		m.avisa("onStart de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
	if err := m.cicloDe("onResume", "()V"); err != nil {
		m.avisa("onResume de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
	m.arrancada = true
}

// cierra una actividad que se va: `onPause`/`onStop` si estaba delante y `onDestroy` siempre.
func (m *maquinaDalvik) cierra() {
	if m.actividad == nil {
		return
	}
	if m.arrancada {
		if err := m.cicloDe("onPause", "()V"); err != nil {
			m.avisa("onPause de %s falló: %v", claseCorta(m.actividad.clase), err)
		}
		if err := m.cicloDe("onStop", "()V"); err != nil {
			m.avisa("onStop de %s falló: %v", claseCorta(m.actividad.clase), err)
		}
		m.arrancada = false
	}
	if err := m.cicloDe("onDestroy", "()V"); err != nil {
		m.avisa("onDestroy de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
}

// despideParaRecrear ejecuta la secuencia con la que el framework mata la actividad para volver a
// crearla: `onPause`, luego `onSaveInstanceState` y solo después `onStop` y `onDestroy`. Devuelve el
// Bundle que la app llenó; nunca nil, porque al recrear el framework entrega uno no nulo.
func (m *maquinaDalvik) despideParaRecrear() *objetoDalvik {
	if m.actividad == nil {
		return m.guardaEstado()
	}
	if m.arrancada {
		if err := m.cicloDe("onPause", "()V"); err != nil {
			m.avisa("onPause de %s falló: %v", claseCorta(m.actividad.clase), err)
		}
	}
	bundle := m.guardaEstado()
	if m.arrancada {
		if err := m.cicloDe("onStop", "()V"); err != nil {
			m.avisa("onStop de %s falló: %v", claseCorta(m.actividad.clase), err)
		}
		m.arrancada = false
	}
	if err := m.cicloDe("onDestroy", "()V"); err != nil {
		m.avisa("onDestroy de %s falló: %v", claseCorta(m.actividad.clase), err)
	}
	return bundle
}

// guardaEstado llama a `onSaveInstanceState` con un Bundle vacío, guarda lo que la app dejó en él y
// lo devuelve. El Bundle se publica después en `ejecucion.bundle`, para que la recreación se pueda
// reproducir sin volver a ejecutar la app.
func (m *maquinaDalvik) guardaEstado() *objetoDalvik {
	bundle := &objetoDalvik{clase: "Landroid/os/Bundle;", campos: map[string]any{},
		bundle: map[string]any{}}
	if m.actividad != nil {
		if met := m.buscaMetodo(m.actividad.clase, "onSaveInstanceState", "(Landroid/os/Bundle;)V"); met != nil {
			m.anotaCiclo("onSaveInstanceState")
			if _, err := m.ejecutaCuerpo(m.actividad.clase, met, []any{m.actividad, bundle},
				[]string{m.actividad.clase, "Landroid/os/Bundle;"}); err != nil {
				m.avisa("onSaveInstanceState de %s falló: %v", claseCorta(m.actividad.clase), err)
			}
		}
	}
	m.estadoGuardado = make(map[string]any, len(bundle.bundle))
	for k, v := range bundle.bundle {
		m.estadoGuardado[k] = v
	}
	return bundle
}

// oyentesDeVista resume los listeners registrados con un id de recurso, sin repetir: es la lista con
// la que la pestaña sabe qué se puede pulsar y con qué nombre pedirlo.
func (m *maquinaDalvik) oyentesDeVista() []oyenteVista {
	oídos := make(map[uint32]bool, len(m.oyentes))
	salida := []oyenteVista{}
	for _, reg := range m.oyentes {
		vista, ok := reg.vista.(*objetoDalvik)
		if !ok || vista.id == 0 || oídos[vista.id] {
			continue
		}
		oídos[vista.id] = true
		o := oyenteVista{IDRef: fmt.Sprintf("%#08x", vista.id)}
		if m.tabla != nil {
			if tipo, clave, ok := m.tabla.nombreDeId(vista.id); ok {
				o.ID = tipo + "/" + clave
			}
		}
		salida = append(salida, o)
	}
	return salida
}

// listaDeOyentes arma el final del mensaje de error con lo que sí se puede pulsar; sin nada, lo dice.
func listaDeOyentes(m *maquinaDalvik) string {
	o := m.oyentesDeVista()
	if len(o) == 0 {
		return " (el onCreate no registró ningún listener con id)"
	}
	nombres := make([]string, 0, len(o))
	for _, oy := range o {
		if oy.ID != "" {
			nombres = append(nombres, oy.ID)
		} else {
			nombres = append(nombres, oy.IDRef)
		}
	}
	return " (los listeners registrados son " + strings.Join(nombres, ", ") + ")"
}

// idDeEvento lleva lo que se escribe en una orden de entrada (`--pulsar`, `--escribir`, `--marcar`,
// `--desmarcar`) al id de recurso de la vista. Se aceptan las tres formas que el modelo publica:
// `id/pulsar` (nombreDeId al revés), `0x7f010000` (el `idRef` de los nodos) y el decimal del mismo
// número. `orden` es el nombre del flag, para que el error diga cuál falló.
func idDeEvento(tabla *tablaRecursos, orden, pedido string) (uint32, error) {
	s := strings.TrimSpace(pedido)
	if s == "" {
		return 0, fmt.Errorf("--%s necesita un id: id/pulsar, 0x7f010000 o su decimal", orden)
	}
	if tipo, clave, ok := strings.Cut(s, "/"); ok {
		if tabla == nil {
			return 0, fmt.Errorf("no se puede %s %q: sin resources.arsc no hay tabla que traduzca "+
				"el nombre; pasa el id numérico", orden, pedido)
		}
		id, ok := tabla.idDeNombre(tipo, clave)
		if !ok {
			return 0, fmt.Errorf("no se puede %s %q: la tabla no tiene %s/%s", orden, pedido, tipo, clave)
		}
		return id, nil
	}
	if n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 32); err == nil &&
		(strings.HasPrefix(strings.ToLower(s), "0x") || strings.ContainsAny(s, "abcdefABCDEF")) {
		return uint32(n), nil
	}
	if n, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(n), nil
	}
	return 0, fmt.Errorf("no se puede %s %q: se espera id/pulsar, 0x7f010000 o un decimal", orden, pedido)
}

// partirEscribir separa `id/texto` en el id de la vista y el texto que se le entrega. El id puede ser
// `id/nombre`, que ya lleva una barra, o un número; y el texto puede llevar barras, así que se corta
// por la barra que cierra el id y no por la última. Un `id/nombre` sin texto entrega el texto vacío,
// que es vaciar el campo.
func partirEscribir(dato string) (string, string) {
	if resto := strings.TrimPrefix(dato, "id/"); resto != dato {
		if clave, texto, ok := strings.Cut(resto, "/"); ok {
			return "id/" + clave, texto
		}
		return dato, ""
	}
	if id, texto, ok := strings.Cut(dato, "/"); ok {
		return id, texto
	}
	return dato, ""
}

// pulsa entrega el clic al `onClick` de la última vista con ese id que se registró, como un
// `setOnClickListener` que sustituye al anterior. Devuelve falso cuando ninguna vista con ese id
// tiene oyente, que es lo que distingue «no hay a quién pulsar» de «se pulsó y no hizo nada».
func (m *maquinaDalvik) pulsa(id uint32) bool {
	for i := len(m.oyentes) - 1; i >= 0; i-- {
		reg := m.oyentes[i]
		vista, ok := reg.vista.(*objetoDalvik)
		if !ok || vista.id != id {
			continue
		}
		obj, ok := reg.oyente.(*objetoDalvik)
		if !ok {
			m.avisa("el listener de %#08x no es un objeto del dex", id)
			return true
		}
		nombre := "onClick"
		if reg.metodo != "" {
			nombre = reg.metodo
		}
		met := m.buscaMetodo(obj.clase, nombre, "(Landroid/view/View;)V")
		if met == nil {
			m.avisa("%s se registró como listener y no tiene %s", claseCorta(obj.clase), nombre)
			return true
		}
		m.interaccion++
		if _, err := m.ejecutaCuerpo(obj.clase, met, []any{obj, reg.vista},
			[]string{obj.clase, "Landroid/view/View;"}); err != nil {
			m.avisa("el onClick de %s falló: %v", claseCorta(obj.clase), err)
		}
		return true
	}
	return false
}

// cortaFila separa la forma `id/lista:1` (o `0x7f010000:1`) en el id de la lista y la posición.
// El id puede llevar `/` —`id/lista`—, así que se corta por el último `:`.
func cortaFila(pedido string) (string, int, bool) {
	i := strings.LastIndex(pedido, ":")
	if i <= 0 || i == len(pedido)-1 {
		return "", 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(pedido[i+1:]))
	if err != nil || n < 0 {
		return "", 0, false
	}
	return strings.TrimSpace(pedido[:i]), n, true
}

// listaDeOyentesDeItem arma el final del error con las listas que sí tienen OnItemClickListener.
func listaDeOyentesDeItem(m *maquinaDalvik) string {
	nombres := []string{}
	for _, reg := range m.oyentesItem {
		vista, ok := reg.vista.(*objetoDalvik)
		if !ok || vista.id == 0 {
			continue
		}
		nombre := fmt.Sprintf("%#08x", vista.id)
		if m.tabla != nil {
			if tipo, clave, ok := m.tabla.nombreDeId(vista.id); ok {
				nombre = tipo + "/" + clave
			}
		}
		nombres = append(nombres, nombre+":fila")
	}
	if len(nombres) == 0 {
		return " (ninguna lista registró un OnItemClickListener)"
	}
	return " (las listas pulsables son " + strings.Join(nombres, ", ") + ")"
}

// pulsaFila entrega el `onItemClick` de la fila pedida, como el toque en un `ListView`: el receptor
// es el listener, el primer argumento la lista y los demás el hijo que el adaptador devolvió para
// esa posición, el índice y su id. Devuelve falso cuando no hay lista con ese id y listener, que es
// lo que distingue «no hay esa fila» de «se pulsó y no hizo nada».
func (m *maquinaDalvik) pulsaFila(id uint32, posicion int) bool {
	for i := len(m.oyentesItem) - 1; i >= 0; i-- {
		reg := m.oyentesItem[i]
		lista, ok := reg.vista.(*objetoDalvik)
		if !ok || lista.id != id {
			continue
		}
		obj, ok := reg.oyente.(*objetoDalvik)
		if !ok {
			m.avisa("el OnItemClickListener de %#08x no es un objeto del dex", id)
			return true
		}
		if lista.adaptador == nil {
			m.avisa("la lista %#08x tiene OnItemClickListener pero ningún adaptador con filas", id)
			return true
		}
		hijo, err := m.elementoDelAdaptador(lista.adaptador, posicion, lista)
		if err != nil {
			m.avisa("la fila %d de %#08x no se pudo pintar: %v", posicion, id, err)
			return true
		}
		const protoItem = "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V"
		met := m.buscaMetodo(obj.clase, "onItemClick", protoItem)
		if met == nil {
			m.avisa("%s se registró como OnItemClickListener y no tiene onItemClick%s",
				claseCorta(obj.clase), protoItem)
			return true
		}
		m.interaccion++
		args := []any{obj, lista, hijo, int32(posicion), int64(posicion)}
		tipos := []string{obj.clase, "Landroid/widget/AdapterView;", "Landroid/view/View;", "I", "J"}
		if _, err := m.ejecutaCuerpo(obj.clase, met, args, tipos); err != nil {
			m.avisa("el onItemClick de %s falló: %v", claseCorta(obj.clase), err)
		}
		return true
	}
	return false
}

// esOrdenDeDialogo dice si un `--pulsar` pide un botón de diálogo (`dialogo/positivo`,
// `dialogo/negativo` o `dialogo/neutro`) en vez de una vista con id.
func esOrdenDeDialogo(pedido string) bool {
	return strings.HasPrefix(strings.TrimSpace(pedido), "dialogo/")
}

// campos de un diálogo dentro de su `objetoDalvik`: el título, el mensaje y, por cada botón, su
// texto y su oyente. Se guardan en el objeto para que `create()` pueda copiarlos tal cual.
const (
	campoDialogoTitulo  = "dialogo.titulo"
	campoDialogoMensaje = "dialogo.mensaje"
)

func campoBotonDialogo(tipo, parte string) string {
	return "dialogo.boton." + tipo + "." + parte
}

// tipoDeBotonDialogo es el papel del botón, que es lo que decide qué listener se ejecuta y con qué
// `which`. Es también el nombre que se escribe en `--pulsar dialogo/<tipo>`.
var tipoDeBotonDialogo = map[string]string{
	"setPositiveButton": "positivo",
	"setNegativeButton": "negativo",
	"setNeutralButton":  "neutro",
}

// whichDeBotonDialogo son las constantes de `DialogInterface` con las que el framework llama al
// `onClick(DialogInterface, int)`: BUTTON_POSITIVE = -1, BUTTON_NEGATIVE = -2 y BUTTON_NEUTRAL = -3.
var whichDeBotonDialogo = map[string]int32{"positivo": -1, "negativo": -2, "neutro": -3}

// textoDeArgumento resuelve el texto de un argumento que puede ser una cadena o el id de un recurso
// de cadena (`R.string.x`), que es lo que el framework acepta en `setTitle`, `setMessage`,
// `setPositiveButton` y `Toast.makeText`.
func (m *maquinaDalvik) textoDeArgumento(v any) string {
	if s, ok := m.textoDeId(v); ok {
		return s
	}
	return textoDe(v)
}

// creaDialogo copia los campos de un `AlertDialog.Builder` a una instancia de `AlertDialog`, que es
// lo que hace `create()` en el framework: los cambios posteriores del builder no se reflejan en el
// diálogo ya creado.
func creaDialogo(b *objetoDalvik) *objetoDalvik {
	d := &objetoDalvik{clase: "Landroid/app/AlertDialog;", campos: make(map[string]any, len(b.campos))}
	for k, v := range b.campos {
		d.campos[k] = v
	}
	return d
}

// dialogoAbierto dice si ese diálogo ya está en la pila, para no apilarlo dos veces cuando la app
// llama a `show()` sobre un diálogo que ya se mostraba.
func (m *maquinaDalvik) dialogoAbierto(d *objetoDalvik) bool {
	for _, x := range m.dialogos {
		if x == d {
			return true
		}
	}
	return false
}

// ejecutaDelAdaptador llama a un método del adaptador del proyecto. El despacho es el de siempre:
// si la subclase lo reescribe se ejecuta su cuerpo, y si no, el stub del framework responde.
func (m *maquinaDalvik) ejecutaDelAdaptador(adaptador *objetoDalvik, nombre, proto string,
	args []any, tipos []string) (any, error) {
	met := m.buscaMetodo(adaptador.clase, nombre, proto)
	if met == nil {
		return nil, fmt.Errorf("%s no tiene %s%s", claseCorta(adaptador.clase), nombre, proto)
	}
	return m.ejecutaCuerpo(adaptador.clase, met, append([]any{adaptador}, args...),
		append([]string{adaptador.clase}, tipos...))
}

// cuentaDelAdaptador pregunta cuántas filas hay. Un getCount negativo es un adaptador roto y se
// avisa en vez de recortarlo a cero como si fuera una lista vacía.
func (m *maquinaDalvik) cuentaDelAdaptador(adaptador any) (int, error) {
	obj, ok := adaptador.(*objetoDalvik)
	if !ok || obj == nil {
		return 0, fmt.Errorf("el adaptador no es un objeto del dex")
	}
	// sin getCount propio, el ArrayAdapter del framework cuenta los elementos que recibió,
	// también cuando son cero: el respaldo es del adaptador, no de un método del dex (§335).
	if m.buscaMetodo(obj.clase, "getCount", "()I") == nil &&
		m.esDe(obj, "Landroid/widget/ArrayAdapter;") {
		return len(obj.elementos), nil
	}
	v, err := m.ejecutaDelAdaptador(obj, "getCount", "()I", nil, nil)
	if err != nil {
		return 0, err
	}
	n, err := aInt32(v)
	if err != nil {
		return 0, fmt.Errorf("getCount: %w", err)
	}
	if n < 0 {
		return 0, fmt.Errorf("getCount devolvió %d", n)
	}
	return int(n), nil
}

// elementoDelAdaptador ejecuta el getView del proyecto para esa posición y devuelve la vista que
// armó. La vista usada (`convertida`) va nula porque esta fase no recicla filas: pedirla una vez
// por posición es lo que se promete, no el reciclado de un ListView real.
func (m *maquinaDalvik) elementoDelAdaptador(adaptador any, posicion int, lista *objetoDalvik) (any, error) {
	obj, ok := adaptador.(*objetoDalvik)
	if !ok || obj == nil {
		return nil, fmt.Errorf("el adaptador no es un objeto del dex")
	}
	const protoVista = "(ILandroid/view/View;Landroid/view/ViewGroup;)Landroid/view/View;"
	if met := m.buscaMetodo(obj.clase, "getView", protoVista); met != nil {
		var padre any
		if lista != nil {
			padre = lista
		}
		return m.ejecutaCuerpo(obj.clase, met,
			[]any{obj, int32(posicion), nil, padre},
			[]string{obj.clase, "I", "Landroid/view/View;", "Landroid/view/ViewGroup;"})
	}
	// sin getView propio, el ArrayAdapter del framework pinta el elemento con una fila de texto
	if obj.elementos != nil {
		if posicion < 0 || posicion >= len(obj.elementos) {
			return nil, fmt.Errorf("la fila %d está fuera de las %d del adaptador", posicion,
				len(obj.elementos))
		}
		return &objetoDalvik{clase: "Landroid/widget/TextView;", campos: map[string]any{},
			texto: textoDe(obj.elementos[posicion])}, nil
	}
	return nil, fmt.Errorf("%s no tiene getView para la fila %d", claseCorta(obj.clase), posicion)
}

// filaPintadaDe traduce la vista que devolvió getView a lo que el modelo necesita: el layout del
// que salió (si la infló), los textos que escribió en sus vistas con id, o el tipo y el texto de
// una vista creada en código. La medida la hace el modelo con ese layout, no con una tabla fija.
func filaPintadaDe(v any) filaPintada {
	obj, ok := v.(*objetoDalvik)
	if !ok || obj == nil {
		return filaPintada{Tipo: "View"}
	}
	f := filaPintada{LayoutID: obj.layoutID}
	if len(obj.vistas) > 0 {
		f.Textos = map[uint32]string{}
		for id, sub := range obj.vistas {
			if sub != nil && (sub.textoEscrito || sub.texto != "") {
				f.Textos[id] = sub.texto
			}
		}
	}
	if f.LayoutID == 0 {
		f.Tipo = tipoDeVista(obj.clase)
		f.Texto = obj.texto
	}
	return f
}

// tipoDeVista deja el nombre corto de un descriptor para el nodo (`TextView`, no
// `android.widget.TextView`), que es la forma en que el modelo nombra los tipos del layout.
func tipoDeVista(descriptor string) string {
	s := claseCorta(descriptor)
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// listasPintadas ejecuta el adaptador de cada lista armada y devuelve sus filas. Corre después de
// los clics, que es cuando la app ya hizo su `notifyDataSetChanged` y el estado es el que se pinta.
func (m *maquinaDalvik) listasPintadas() []listaPintada {
	if len(m.listas) == 0 {
		return nil
	}
	salida := make([]listaPintada, 0, len(m.listas))
	vistos := map[uint32]bool{}
	for _, lista := range m.listas {
		if lista == nil || lista.id == 0 || vistos[lista.id] {
			continue
		}
		vistos[lista.id] = true
		if lista.adaptador == nil {
			m.avisa("la lista %#08x no tiene adaptador: se pinta sin filas", lista.id)
			continue
		}
		n, err := m.cuentaDelAdaptador(lista.adaptador)
		if err != nil {
			m.avisa("el getCount de la lista %#08x falló: %v", lista.id, err)
			continue
		}
		if n > filasMaximas {
			m.avisa("la lista %#08x tiene %d filas: se pintan las %d primeras", lista.id, n, filasMaximas)
			n = filasMaximas
		}
		l := listaPintada{ID: lista.id}
		for i := 0; i < n; i++ {
			v, err := m.elementoDelAdaptador(lista.adaptador, i, lista)
			if err != nil {
				m.avisa("el getView(%d) de la lista %#08x falló: %v", i, lista.id, err)
				break
			}
			l.Filas = append(l.Filas, filaPintadaDe(v))
		}
		salida = append(salida, l)
	}
	sort.Slice(salida, func(i, j int) bool { return salida[i].ID < salida[j].ID })
	return salida
}

// apilaDialogo pone un diálogo delante de la pantalla. Si ya estaba abierto, no lo duplica.
func (m *maquinaDalvik) apilaDialogo(d *objetoDalvik) {
	if m.dialogoAbierto(d) {
		return
	}
	m.dialogos = append(m.dialogos, d)
}

// cierraDialogo quita el diálogo de la pila; si no estaba, no hace nada. Es lo que hacen `dismiss()`,
// `cancel()` y pulsar cualquiera de sus botones.
func (m *maquinaDalvik) cierraDialogo(d *objetoDalvik) {
	for i, x := range m.dialogos {
		if x == d {
			m.dialogos = append(m.dialogos[:i], m.dialogos[i+1:]...)
			return
		}
	}
}

// dialogoDeObjeto arma la capa publicable de un diálogo: título, mensaje y los botones que tiene, en
// el orden del framework. Un botón sin texto no se publica; uno sin oyente sí, porque se ve y se
// puede pulsar aunque solo cierre.
func dialogoDeObjeto(d *objetoDalvik) dialogoVista {
	v := dialogoVista{}
	if s, ok := d.campos[campoDialogoTitulo].(string); ok {
		v.Titulo = s
	}
	if s, ok := d.campos[campoDialogoMensaje].(string); ok {
		v.Mensaje = s
	}
	for _, tipo := range []string{"positivo", "negativo", "neutro"} {
		texto, ok := d.campos[campoBotonDialogo(tipo, "texto")].(string)
		if !ok {
			continue
		}
		v.Botones = append(v.Botones, botonDialogo{Tipo: tipo, Texto: texto})
	}
	return v
}

// dialogosDeVista publica la pila de diálogos de la pantalla actual, en orden.
func (m *maquinaDalvik) dialogosDeVista() []dialogoVista {
	if len(m.dialogos) == 0 {
		return nil
	}
	salida := make([]dialogoVista, 0, len(m.dialogos))
	for _, d := range m.dialogos {
		salida = append(salida, dialogoDeObjeto(d))
	}
	return salida
}

// pulsaBotonDeDialogo entrega el clic al botón del diálogo que está delante. Como el framework,
// cerrar el diálogo es parte del clic: se quita de la pila antes de correr el oyente, para que un
// `dismiss()` dentro del listener no lo intente dos veces. El oyente recibe el diálogo y el `which`
// de su papel.
func (m *maquinaDalvik) pulsaBotonDeDialogo(pedido string) error {
	tipo := strings.TrimPrefix(strings.TrimSpace(pedido), "dialogo/")
	if _, ok := whichDeBotonDialogo[tipo]; !ok {
		return fmt.Errorf("no se puede pulsar %q: un diálogo solo tiene dialogo/positivo, "+
			"dialogo/negativo o dialogo/neutro", pedido)
	}
	if len(m.dialogos) == 0 {
		return fmt.Errorf("no se puede pulsar %q: no hay ningún diálogo abierto", pedido)
	}
	d := m.dialogos[len(m.dialogos)-1]
	if _, ok := d.campos[campoBotonDialogo(tipo, "texto")]; !ok {
		return fmt.Errorf("no se puede pulsar %q: el diálogo no tiene botón %s", pedido, tipo)
	}
	m.cierraDialogo(d)
	m.interaccion++
	oyente, _ := d.campos[campoBotonDialogo(tipo, "oyente")].(*objetoDalvik)
	if oyente == nil {
		// el botón se ve y cierra el diálogo, pero nadie pidió que hiciera nada más
		return nil
	}
	met := m.buscaMetodo(oyente.clase, "onClick", "(Landroid/content/DialogInterface;I)V")
	if met == nil {
		return fmt.Errorf("%s se registró como onClick del diálogo y no tiene "+
			"onClick(DialogInterface, int)", claseCorta(oyente.clase))
	}
	if _, err := m.ejecutaCuerpo(oyente.clase, met, []any{oyente, d, whichDeBotonDialogo[tipo]},
		[]string{oyente.clase, "Landroid/content/DialogInterface;", "I"}); err != nil {
		m.avisa("el onClick de %s falló: %v", claseCorta(oyente.clase), err)
	}
	return nil
}

// Las dos firmas que el framework llama para el menú de opciones: una para construirlo y otra para
// entregar el ítem pulsado. `onCreateOptionsMenu` y `onOptionsItemSelected` viven en la actividad;
// `onMenuItemClick` es el oyente que un ítem puede llevar puesto.
const (
	protoOpcionesMenu = "(Landroid/view/Menu;)Z"
	protoItemDeMenu   = "(Landroid/view/MenuItem;)Z"
)

// itemDeMenu arma el objeto de un `Menu.add`, con el estado por instancia: su título, su id si lo
// trae y los valores por defecto del framework (visible y habilitado). Las dos formas que usa una app
// son `add(CharSequence)` y `add(int grupo, int id, int orden, CharSequence | int)`.
func (m *maquinaDalvik) itemDeMenu(vals []any) (*objetoDalvik, error) {
	it := &objetoDalvik{clase: "Landroid/view/MenuItem;", campos: map[string]any{},
		itemVisible: true, itemHabilitado: true}
	switch len(vals) {
	case 1:
		it.itemTitulo = m.textoDeArgumento(vals[0])
	case 4:
		grupo, err := aInt32(vals[0])
		if err != nil {
			return nil, err
		}
		id, err := aInt32(vals[1])
		if err != nil {
			return nil, err
		}
		orden, err := aInt32(vals[2])
		if err != nil {
			return nil, err
		}
		it.itemGrupo = grupo
		it.itemID = id
		it.itemOrden = orden
		it.itemTitulo = m.textoDeArgumento(vals[3])
	default:
		return nil, fmt.Errorf("Menu.add con %d argumentos: el framework solo tiene "+
			"add(CharSequence) y add(int, int, int, CharSequence|int)", len(vals))
	}
	return it, nil
}

// abreMenuDeOpciones construye el menú de opciones como el framework al desplegar el
// desbordamiento: llama a `onCreateOptionsMenu` con un menú nuevo y, si existe, a
// `onPrepareOptionsMenu`. El menú queda en la máquina y `--menu-item` entrega sus ítems.
func (m *maquinaDalvik) abreMenuDeOpciones() error {
	if m.actividad == nil || m.actividad.clase == "" {
		return fmt.Errorf("--menu sin actividad sobre la que abrir el menú")
	}
	clase := m.actividad.clase
	crea := m.buscaMetodo(clase, "onCreateOptionsMenu", protoOpcionesMenu)
	if crea == nil {
		return fmt.Errorf("%s no tiene onCreateOptionsMenu(Menu): no hay menú de opciones que abrir",
			claseCorta(clase))
	}
	menu := &objetoDalvik{clase: "Landroid/view/Menu;", campos: map[string]any{}}
	m.menu = menu
	m.menuAbierto = false
	m.menuSeleccionado = ""
	res, err := m.ejecutaCuerpo(clase, crea, []any{m.actividad, menu},
		[]string{clase, "Landroid/view/Menu;"})
	if err != nil {
		return fmt.Errorf("onCreateOptionsMenu de %s falló: %w", claseCorta(clase), err)
	}
	m.menuAbierto = boolDe(res)
	// el framework llama a `onPrepareOptionsMenu` cada vez que el menú se va a mostrar, después de
	// crearlo; si la app lo usa para retocar ítems, la vista previa ve el mismo menú que el aparato
	if prepara := m.buscaMetodo(clase, "onPrepareOptionsMenu", protoOpcionesMenu); prepara != nil {
		if _, err := m.ejecutaCuerpo(clase, prepara, []any{m.actividad, menu},
			[]string{clase, "Landroid/view/Menu;"}); err != nil {
			m.avisa("onPrepareOptionsMenu de %s falló: %v", claseCorta(clase), err)
		}
	}
	if !m.menuAbierto && len(menu.menuItems) == 0 {
		m.avisa("onCreateOptionsMenu de %s devolvió false: el menú no se muestra",
			claseCorta(clase))
	}
	return nil
}

// indiceDeItemDeMenu lee la forma `menu/N` (o el entero N) que publica el modelo y que la pestaña
// manda en `--menu-item`.
func indiceDeItemDeMenu(pedido string) (int, error) {
	s := strings.TrimSpace(pedido)
	s = strings.TrimPrefix(s, "menu/")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("--menu-item %q: se espera menu/N con N entero no negativo", pedido)
	}
	return n, nil
}

// pulsaItemDeMenu entrega el clic de un ítem del menú, en el orden del framework: primero el
// `OnMenuItemClickListener` del propio ítem y, si no lo consume, `onOptionsItemSelected` de la
// actividad. Elegir un ítem cierra el menú, como en el aparato.
func (m *maquinaDalvik) pulsaItemDeMenu(pedido string) error {
	indice, err := indiceDeItemDeMenu(pedido)
	if err != nil {
		return err
	}
	if m.menu == nil {
		if err := m.abreMenuDeOpciones(); err != nil {
			return err
		}
	}
	if indice >= len(m.menu.menuItems) {
		return fmt.Errorf("no se puede pulsar %q: el menú tiene %d ítems",
			pedido, len(m.menu.menuItems))
	}
	item := m.menu.menuItems[indice]
	if !item.itemVisible {
		return fmt.Errorf("no se puede pulsar %q: el ítem %d no es visible", pedido, indice)
	}
	if !item.itemHabilitado {
		return fmt.Errorf("no se puede pulsar %q: el ítem %d está deshabilitado", pedido, indice)
	}
	m.interaccion++
	consumido := false
	// 1) el oyente del ítem, que es lo que registra `setOnMenuItemClickListener`
	if oyente, ok := item.itemOyente.(*objetoDalvik); ok {
		met := m.buscaMetodo(oyente.clase, "onMenuItemClick", protoItemDeMenu)
		if met == nil {
			return fmt.Errorf("%s se registró como OnMenuItemClickListener y no tiene "+
				"onMenuItemClick(MenuItem)", claseCorta(oyente.clase))
		}
		res, err := m.ejecutaCuerpo(oyente.clase, met, []any{oyente, item},
			[]string{oyente.clase, "Landroid/view/MenuItem;"})
		if err != nil {
			return fmt.Errorf("onMenuItemClick de %s falló: %w", claseCorta(oyente.clase), err)
		}
		consumido = boolDe(res)
	}
	// 2) `onOptionsItemSelected`, si el ítem no lo consumió
	if !consumido {
		if met := m.buscaMetodo(m.actividad.clase, "onOptionsItemSelected", protoItemDeMenu); met != nil {
			res, err := m.ejecutaCuerpo(m.actividad.clase, met, []any{m.actividad, item},
				[]string{m.actividad.clase, "Landroid/view/MenuItem;"})
			if err != nil {
				return fmt.Errorf("onOptionsItemSelected de %s falló: %w",
					claseCorta(m.actividad.clase), err)
			}
			consumido = boolDe(res)
		}
	}
	if !consumido {
		m.avisa("el ítem %d del menú (%q) no lo atendió ni su OnMenuItemClickListener ni "+
			"onOptionsItemSelected", indice, item.itemTitulo)
	}
	m.menuSeleccionado = fmt.Sprintf("menu/%d", indice)
	m.menuAbierto = false
	return nil
}

// menuVista publica el menú que quedó: solo los ítems visibles, con su posición real como id.
func (m *maquinaDalvik) menuVista() *menuVista {
	if m.menu == nil {
		return nil
	}
	vista := &menuVista{Abierto: m.menuAbierto, Seleccionado: m.menuSeleccionado}
	for i, item := range m.menu.menuItems {
		if item == nil || !item.itemVisible {
			continue
		}
		vista.Items = append(vista.Items, itemMenuVista{
			ID: fmt.Sprintf("menu/%d", i), Titulo: item.itemTitulo, ItemID: item.itemID,
			Orden: item.itemOrden, Marcado: item.itemMarcado,
		})
	}
	return vista
}

// registraOnClickDeclarativo enlaza el `android:onClick="sumar"` del layout, como hace el inflado
// del framework: el receptor es la actividad (el contexto de la vista) y el método se busca por
// nombre con la firma `(View)V`. Devuelve el motivo cuando no se puede enlazar, y "" cuando sí.
func (m *maquinaDalvik) registraOnClickDeclarativo(id uint32, nombre string) string {
	if m.actividad == nil {
		return fmt.Sprintf("android:onClick=%q no tiene actividad donde buscar el método", nombre)
	}
	if m.buscaMetodo(m.actividad.clase, nombre, "(Landroid/view/View;)V") == nil {
		return fmt.Sprintf("android:onClick=%q y %s no tiene %s(View)", nombre,
			claseCorta(m.actividad.clase), nombre)
	}
	// precedencia de Android: el inflado enlaza el `android:onClick` y el `setOnClickListener` que
	// corre después en `onCreate` lo sustituye. Este enlace se hace con la actividad ya creada, así
	// que un oyente programático sobre la misma vista es el que manda y el del XML no se enlaza
	// (`metodo` vacío distingue el programático del declarativo).
	for _, reg := range m.oyentes {
		if reg.metodo != "" {
			continue
		}
		if vista, ok := reg.vista.(*objetoDalvik); ok && vista.id == id {
			return ""
		}
	}
	vista := m.vista(id)
	m.oyentes = append(m.oyentes, oyenteRegistrado{oyente: m.actividad, vista: vista, metodo: nombre})
	return ""
}

// eligeClaseLanzadora busca a quién ejecutar: primero la actividad del manifiesto si tiene un punto
// de entrada, después cualquier clase sin `$` con `main` y luego con `onCreate`. Ignorar las
// anónimas es lo que evita elegir `MainActivity$1`, que también es una clase con código.
func eligeClaseLanzadora(p *programaDalvik, preferida string) (string, bool) {
	tipoDe := func(nombre string) byte {
		cl, ok := p.clases[nombre]
		if !ok {
			return 0
		}
		if cl.metodos["main([Ljava/lang/String;)V"] != nil {
			return 'm'
		}
		if cl.metodos["onCreate(Landroid/os/Bundle;)V"] != nil {
			return 'o'
		}
		return 0
	}
	if t := tipoDe(preferida); t != 0 {
		return preferida, t == 'm'
	}
	nombres := make([]string, 0, len(p.clases))
	for n := range p.clases {
		if strings.Contains(n, "$") {
			continue
		}
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	for _, n := range nombres {
		if p.clases[n].metodos["main([Ljava/lang/String;)V"] != nil {
			return n, true
		}
	}
	for _, n := range nombres {
		if p.clases[n].metodos["onCreate(Landroid/os/Bundle;)V"] != nil {
			return n, false
		}
	}
	return "", false
}

// jerarquiaDelDex sube por una clase y sus padres mientras estén en el dex, y termina en el primero
// que no está: el padre del framework. Esa cadena es la que dice qué API de Android hay que emular
// para un objeto de una clase del proyecto.
func (m *maquinaDalvik) jerarquiaDelDex(clase string) []string {
	cadena := []string{}
	for i := 0; i < 32; i++ {
		cadena = append(cadena, clase)
		cl, ok := m.prog.clases[clase]
		if !ok || cl.super == "" || cl.super == clase {
			return cadena
		}
		clase = cl.super
	}
	return cadena
}

// buscaMetodo sube por la jerarquía de clases del dex hasta encontrar nombre+firma.
func (m *maquinaDalvik) buscaMetodo(clase, nombre, proto string) *metodoDalvik {
	for i := 0; i < 32; i++ {
		cl, ok := m.prog.clases[clase]
		if !ok {
			return nil
		}
		if met := cl.metodos[nombre+proto]; met != nil {
			return met
		}
		if cl.super == "" || cl.super == clase {
			return nil
		}
		clase = cl.super
	}
	return nil
}

// excepcionDalvik es una excepción de la app en vuelo: el descriptor de la clase lanzada y el objeto,
// que es lo que `move-exception` deja en el registro del handler. Implementa error para subir por las
// mismas vías que un fallo del intérprete, pero solo ella se busca en la tabla del método: el resto de
// errores son fallos duros (código que no cabe, un opcode sin decodificar) y no se finge que un catch
// los atrape.
type excepcionDalvik struct {
	tipo   string
	objeto *objetoDalvik
}

func (e *excepcionDalvik) Error() string {
	if e.objeto != nil && e.objeto.mensajeExcepcion != "" {
		return claseCorta(e.tipo) + ": " + e.objeto.mensajeExcepcion
	}
	return claseCorta(e.tipo)
}

// nuevaExcepcion arma la excepción de un tipo con su mensaje, que es lo que `getMessage()` devuelve si
// un catch la captura y lo que enseña el aviso cuando nadie la captura.
func nuevaExcepcion(tipo, formato string, args ...any) *excepcionDalvik {
	obj := &objetoDalvik{clase: tipo, campos: map[string]any{},
		mensajeExcepcion: fmt.Sprintf(formato, args...)}
	return &excepcionDalvik{tipo: tipo, objeto: obj}
}

// excepcionDe busca una excepción de la app en la cadena de errores. Se recorre con Unwrap, como
// `esMalformado`, para no arrastrar `errors.As` al binario de wasm.
func excepcionDe(err error) *excepcionDalvik {
	for err != nil {
		if ex, ok := err.(*excepcionDalvik); ok {
			return ex
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil
		}
		err = u.Unwrap()
	}
	return nil
}

// padresDeExcepcion es la jerarquía del framework que hace falta para despachar un catch: el dex solo
// trae las clases del proyecto, y un `catch (Exception e)` nombra un padre del framework mientras el
// objeto lanzado es un hijo suyo. Las claves y los valores son descriptores del dex.
var padresDeExcepcion = map[string]string{
	"Ljava/lang/NumberFormatException;":           "Ljava/lang/IllegalArgumentException;",
	"Ljava/lang/IllegalArgumentException;":        "Ljava/lang/RuntimeException;",
	"Ljava/lang/IllegalStateException;":           "Ljava/lang/RuntimeException;",
	"Ljava/lang/NullPointerException;":            "Ljava/lang/RuntimeException;",
	"Ljava/lang/ArithmeticException;":             "Ljava/lang/RuntimeException;",
	"Ljava/lang/ClassCastException;":              "Ljava/lang/RuntimeException;",
	"Ljava/lang/IndexOutOfBoundsException;":       "Ljava/lang/RuntimeException;",
	"Ljava/lang/ArrayIndexOutOfBoundsException;":  "Ljava/lang/IndexOutOfBoundsException;",
	"Ljava/lang/StringIndexOutOfBoundsException;": "Ljava/lang/IndexOutOfBoundsException;",
	"Ljava/lang/NegativeArraySizeException;":      "Ljava/lang/RuntimeException;",
	"Ljava/lang/UnsupportedOperationException;":   "Ljava/lang/RuntimeException;",
	"Ljava/lang/SecurityException;":               "Ljava/lang/RuntimeException;",
	"Ljava/lang/RuntimeException;":                "Ljava/lang/Exception;",
	"Ljava/io/IOException;":                       "Ljava/lang/Exception;",
	"Ljava/io/FileNotFoundException;":             "Ljava/io/IOException;",
	"Ljava/lang/InterruptedException;":            "Ljava/lang/Exception;",
	"Ljava/lang/ReflectiveOperationException;":    "Ljava/lang/Exception;",
	"Ljava/lang/ClassNotFoundException;":          "Ljava/lang/ReflectiveOperationException;",
	"Ljava/lang/InstantiationException;":          "Ljava/lang/ReflectiveOperationException;",
	"Ljava/lang/IllegalAccessException;":          "Ljava/lang/ReflectiveOperationException;",
	"Ljava/lang/NoSuchMethodException;":           "Ljava/lang/ReflectiveOperationException;",
	"Ljava/lang/NoSuchFieldException;":            "Ljava/lang/ReflectiveOperationException;",
	"Ljava/lang/CloneNotSupportedException;":      "Ljava/lang/Exception;",
	"Ljava/text/ParseException;":                  "Ljava/lang/Exception;",
	"Ljava/lang/Exception;":                       "Ljava/lang/Throwable;",
	"Ljava/lang/Error;":                           "Ljava/lang/Throwable;",
	"Ljava/lang/AssertionError;":                  "Ljava/lang/Error;",
	"Ljava/lang/VirtualMachineError;":             "Ljava/lang/Error;",
	"Ljava/lang/OutOfMemoryError;":                "Ljava/lang/VirtualMachineError;",
	"Ljava/lang/StackOverflowError;":              "Ljava/lang/VirtualMachineError;",
}

// esThrowableDe dice si un descriptor es una excepción o un error: lo que un `catch` puede nombrar y
// lo que un `throw` puede lanzar. Deciden la jerarquía del dex y la de `padresDeExcepcion`, no el
// nombre: una excepción propia llamada `MiExcepcion` o `FalloApp` no lleva el sufijo y aun así hereda
// de `Exception`. El sufijo queda como último recurso para una clase del framework que no esté en la
// tabla de padres.
func (m *maquinaDalvik) esThrowableDe(descriptor string) bool {
	return m.tipoAcepta("Ljava/lang/Throwable;", descriptor) || esThrowable(descriptor)
}

// esThrowable es el respaldo sin máquina: el sufijo `Exception;`/`Error;` de las clases del framework.
// No sirve para las del proyecto —`MiExcepcion`, `FalloApp`—, así que dentro del intérprete se usa
// `esThrowableDe`, que sí sube por el dex.
func esThrowable(descriptor string) bool {
	return strings.HasSuffix(descriptor, "Exception;") || strings.HasSuffix(descriptor, "Error;") ||
		descriptor == "Ljava/lang/Throwable;"
}

// tipoAcepta dice si un catch de `atrapa` recibe una excepción de tipo `lanzado`: sube por la
// jerarquía del dex y, cuando la clase no está en el dex, por `padresDeExcepcion`.
func (m *maquinaDalvik) tipoAcepta(atrapa, lanzado string) bool {
	for i := 0; i < 32; i++ {
		if lanzado == atrapa {
			return true
		}
		if cl, ok := m.prog.clases[lanzado]; ok && cl.super != "" && cl.super != lanzado {
			lanzado = cl.super
			continue
		}
		padre, ok := padresDeExcepcion[lanzado]
		if !ok {
			return false
		}
		lanzado = padre
	}
	return false
}

// leeUlebEn y leeSlebEn recorren un uleb128/sleb128 dentro de la lista de handlers, que ya está
// extraída del fichero: `d.uleb` trabaja sobre el dex entero y aquí solo quedan esos bytes.
func leeUlebEn(b []byte, at int) (uint32, int, bool) {
	var v uint32
	var shift uint
	for {
		if at >= len(b) || shift > 28 {
			return 0, at, false
		}
		c := b[at]
		at++
		v |= uint32(c&0x7f) << shift
		if c&0x80 == 0 {
			return v, at, true
		}
		shift += 7
	}
}

func leeSlebEn(b []byte, at int) (int32, int, bool) {
	var v int32
	var shift uint
	for {
		if at >= len(b) || shift > 28 {
			return 0, at, false
		}
		c := b[at]
		at++
		v |= int32(c&0x7f) << shift
		shift += 7
		if c&0x80 == 0 {
			if shift < 32 && c&0x40 != 0 {
				v |= -1 << shift
			}
			return v, at, true
		}
	}
}

// buscaHandler devuelve la unidad del primer handler que cubre `pc` y acepta el tipo de la excepción,
// o -1 cuando ninguno lo hace. Los try_item se recorren en el orden del fichero, que es el que decide
// qué catch gana cuando dos rangos se solapan, y dentro de un handler manda el orden de sus filas con
// el catch-all al final, como en ART.
func (m *maquinaDalvik) buscaHandler(d *dexFile, it *codeItem, ex *excepcionDalvik, pc int) (int, error) {
	for _, tr := range it.tries {
		if pc < int(tr.start) || pc >= int(tr.start)+int(tr.count) {
			continue
		}
		unidad, ok, err := m.handlerQueAcepta(d, it, tr, ex)
		if err != nil {
			return -1, err
		}
		if ok {
			return unidad, nil
		}
	}
	return -1, nil
}

// handlerQueAcepta recorre el encoded_catch_handler de un try_item: su tamaño en sleb (negativo cuando
// lleva catch-all), los pares (tipo, dirección) en orden y la dirección del catch-all. Devuelve la
// primera dirección cuyo tipo recibe la excepción.
func (m *maquinaDalvik) handlerQueAcepta(d *dexFile, it *codeItem, tr tryItem, ex *excepcionDalvik) (int, bool, error) {
	b := it.handlers
	at := int(tr.handlerOff)
	if at < 0 || at >= len(b) {
		return 0, false, malformado("un try_item apunta al handler %d y la lista mide %d bytes",
			tr.handlerOff, len(b))
	}
	n, pos, ok := leeSlebEn(b, at)
	if !ok {
		return 0, false, malformado("el handler del try_item en %d no cabe en la lista", tr.start)
	}
	pares := n
	if pares < 0 {
		pares = -pares
	}
	for i := int32(0); i < pares; i++ {
		tipoIdx, p, ok := leeUlebEn(b, pos)
		if !ok {
			return 0, false, malformado("el tipo del handler del try_item en %d no cabe", tr.start)
		}
		direccion, p2, ok := leeUlebEn(b, p)
		if !ok {
			return 0, false, malformado("la dirección del handler del try_item en %d no cabe", tr.start)
		}
		pos = p2
		if int(tipoIdx) >= len(d.types) {
			return 0, false, malformado(
				"el handler del try_item en %d nombra el tipo %d, fuera de type_ids", tr.start, tipoIdx)
		}
		if m.tipoAcepta(d.types[tipoIdx], ex.tipo) {
			return int(direccion), true, nil
		}
	}
	if n <= 0 {
		direccion, _, ok := leeUlebEn(b, pos)
		if !ok {
			return 0, false, malformado("el catch-all del try_item en %d no cabe", tr.start)
		}
		return int(direccion), true, nil
	}
	return 0, false, nil
}

// colocaArgumentos escribe cada argumento en su registro de entrada a partir de `pos`, con una
// marca ancha detrás de `long` y `double`. Un fallo aquí es de la llamada, no del dex: se devuelve
// como error normal (sin `malformado`) y con su propio recover, para que un índice inesperado nunca
// se confunda con bytes rotos del code_item. La guarda de `ejecutaCuerpo` ya comprobó que caben.
func colocaArgumentos(regs []any, pos int, args []any, tipos []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("los argumentos no caben en el marco de %d registros: %v", len(regs), r)
		}
	}()
	for i, a := range args {
		tipo := ""
		if i < len(tipos) {
			tipo = tipos[i]
		}
		regs[pos] = a
		if anchoDe(tipo) == 2 && pos+1 < len(regs) {
			regs[pos+1] = marcaAncha{}
		}
		pos += anchoDe(tipo)
	}
	return nil
}

// ejecutaCuerpo interpreta el code_item de un método con los argumentos dados. `tipos` dice el tipo
// de cada argumento, que es lo que decide cuántas palabras ocupa en los registros.
func (m *maquinaDalvik) ejecutaCuerpo(clase string, met *metodoDalvik, args []any, tipos []string) (res any, err error) {
	if met == nil {
		return nil, fmt.Errorf("%s: método sin cuerpo que ejecutar", claseCorta(clase))
	}
	if met.item == nil {
		return m.stubSinCuerpo(clase, met, args)
	}
	// El code_item se interpreta contra el dex del que salió: tras un hot swap la sesión viva tiene
	// métodos de dos ficheros y sus índices no se mezclan.
	d := met.dex
	if d == nil {
		d = m.prog.dex
	}
	it := met.item
	if m.profundidad >= profundidadMaximaDalvik {
		return nil, fmt.Errorf("llamadas anidadas por encima de %d en %s.%s",
			profundidadMaximaDalvik, claseCorta(clase), met.nombre)
	}
	m.profundidad++
	defer func() { m.profundidad-- }()
	// El decodificador comprueba cada acceso a las palabras del code_item y rechaza por nombre lo que
	// no cabe (payloads de switch incluidos). Este recover es la última red: si algo se escapa, se
	// dice en qué unidad y con qué opcode, y se marca como dex malformado para que el comando lo
	// rechace; no se disfraza de error de negocio ni se atribuye sin pruebas a los registros. Los
	// argumentos no llegan aquí: se validan contra `insSize` y su fallo es error de llamada.
	pcEnCurso, opEnCurso := 0, byte(0)
	defer func() {
		if r := recover(); r != nil {
			res = nil
			err = malformado("%s.%s: el intérprete se detuvo en la unidad %d (opcode %#02x) por un fallo interno: %v",
				claseCorta(clase), met.nombre, pcEnCurso, opEnCurso, r)
		}
	}()

	regs := make([]any, it.registersSize)
	pos := int(it.registersSize) - int(it.insSize)
	// Los argumentos no son bytes del dex: si no caben en el marco de entrada, la llamada está mal
	// armada por quien invoca. Se comprueba antes del bucle para decirlo con el método y el conteo,
	// sin caer en el recover de arriba, que marcaría el dex como malformado y taparía la causa real.
	palabras := 0
	for i := range args {
		tipo := ""
		if i < len(tipos) {
			tipo = tipos[i]
		}
		palabras += anchoDe(tipo)
	}
	if pos < 0 || int(it.insSize) > len(regs) || palabras > int(it.insSize) {
		return nil, fmt.Errorf(
			"el método %s.%s declara %d registros de entrada (insSize) y la llamada pasa %d argumentos que ocupan %d palabras sobre un marco de %d registros: la llamada no cabe",
			claseCorta(clase), met.nombre, it.insSize, len(args), palabras, it.registersSize)
	}
	if err := colocaArgumentos(regs, pos, args, tipos); err != nil {
		return nil, fmt.Errorf("el método %s.%s: %v", claseCorta(clase), met.nombre, err)
	}

	ins := it.insns
	pc := 0
	var resultado any
	var hayResultado bool
	poner := func(i int, v any) { regs[i] = v }
	ponerAncho := func(i int, v any) {
		regs[i] = v
		if i+1 < len(regs) {
			regs[i+1] = marcaAncha{}
		}
	}

	// pendiente es la excepción que acaba de entrar a un handler: `move-exception` la copia a su
	// registro y la consume. Vive aquí, fuera del bucle, para sobrevivir al salto.
	var pendiente *excepcionDalvik
	// corre ejecuta el cuerpo desde `pc`. Devuelve cuando el método retorna o cuando algo falla; si el
	// fallo es una excepción de la app, el despacho de abajo busca su handler y reanuda en él. Los
	// fallos duros (bytes que no caben, un opcode sin decodificar) no se atrapan.
	corre := func() (any, error) {
		for {
			if pc < 0 || pc >= len(ins) {
				return nil, fmt.Errorf("%s.%s se salió del código en la unidad %d",
					claseCorta(clase), met.nombre, pc)
			}
			m.pasos++
			if m.pasos-m.pasosBase > pasosMaximos {
				return nil, fmt.Errorf("el intérprete se detuvo a los %d pasos (posible bucle sin salida)",
					pasosMaximos)
			}
			u := ins[pc]
			op := byte(u)
			aa := int(u >> 8)
			pcEnCurso, opEnCurso = pc, op
			segunda := func() (uint16, error) {
				if pc+1 >= len(ins) {
					return 0, fmt.Errorf("%s.%s: el opcode %#02x pide una palabra y el código termina",
						claseCorta(clase), met.nombre, op)
				}
				return ins[pc+1], nil
			}
			tercera := func() (uint16, error) {
				if pc+2 >= len(ins) {
					return 0, fmt.Errorf("%s.%s: el opcode %#02x pide tres palabras y el código termina",
						claseCorta(clase), met.nombre, op)
				}
				return ins[pc+2], nil
			}

			switch op {
			case 0x00: // nop
				pc++

			// --- movimientos -------------------------------------------------------------------
			case 0x01, 0x04, 0x07: // move / move-wide / move-object (12x)
				dst, src := int(u>>8)&0xf, int(u>>12)
				if op == 0x01 || op == 0x07 {
					poner(dst, regs[src])
				} else {
					ponerAncho(dst, regs[src])
				}
				pc++
			case 0x02, 0x05, 0x08: // move/from16, move-wide/from16, move-object/from16 (22x)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if op == 0x02 || op == 0x08 {
					poner(aa, regs[int(w)])
				} else {
					ponerAncho(aa, regs[int(w)])
				}
				pc += 2
			case 0x03, 0x06, 0x09: // move/16, move-wide/16, move-object/16 (32x)
				if pc+2 >= len(ins) {
					return nil, fmt.Errorf("código cortado en un move/16")
				}
				src := int(ins[pc+2])
				if op == 0x03 || op == 0x09 {
					poner(aa, regs[src])
				} else {
					ponerAncho(aa, regs[src])
				}
				pc += 3
			case 0x0a: // move-result
				if !hayResultado {
					m.avisa("move-result sin llamada previa en %s.%s", claseCorta(clase), met.nombre)
				}
				poner(aa, resultado)
				pc++
			case 0x0b: // move-result-wide
				ponerAncho(aa, resultado)
				pc++
			case 0x0c: // move-result-object
				poner(aa, resultado)
				pc++
			case 0x0d: // move-exception: la excepción que entró al handler
				if pendiente == nil {
					return nil, fmt.Errorf("%s.%s hace move-exception sin excepción en vuelo",
						claseCorta(clase), met.nombre)
				}
				poner(aa, pendiente.objeto)
				pendiente = nil
				pc++

			// --- retornos ----------------------------------------------------------------------
			case 0x0e:
				return nil, nil
			case 0x0f, 0x10, 0x11: // return, return-wide, return-object
				return regs[aa], nil

			// --- constantes --------------------------------------------------------------------
			case 0x12: // const/4: el registro es el nibble bajo de AA y el literal el alto, con signo
				v := aa >> 4
				if v >= 8 {
					v -= 16
				}
				poner(aa&0xf, int32(v))
				pc++
			case 0x13: // const/16
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				poner(aa, int32(int16(w)))
				pc += 2
			case 0x14: // const (31i)
				if pc+2 >= len(ins) {
					return nil, fmt.Errorf("código cortado en un const")
				}
				poner(aa, int32(uint32(ins[pc+1])|uint32(ins[pc+2])<<16))
				pc += 3
			case 0x15: // const/high16
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				poner(aa, int32(uint32(w)<<16))
				pc += 2
			case 0x16: // const-wide/16
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				ponerAncho(aa, int64(int16(w)))
				pc += 2
			case 0x17: // const-wide/32
				if pc+2 >= len(ins) {
					return nil, fmt.Errorf("código cortado en un const-wide/32")
				}
				ponerAncho(aa, int64(int32(uint32(ins[pc+1])|uint32(ins[pc+2])<<16)))
				pc += 3
			case 0x18: // const-wide (51l)
				if pc+4 >= len(ins) {
					return nil, fmt.Errorf("código cortado en un const-wide")
				}
				v := uint64(ins[pc+1]) | uint64(ins[pc+2])<<16 | uint64(ins[pc+3])<<32 | uint64(ins[pc+4])<<48
				ponerAncho(aa, int64(v))
				pc += 5
			case 0x19: // const-wide/high16
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				ponerAncho(aa, int64(int16(w))<<48)
				pc += 2
			case 0x1a: // const-string
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if int(w) >= len(d.strings) {
					return nil, fmt.Errorf("const-string con la cadena %d fuera del pool", w)
				}
				poner(aa, d.strings[w])
				pc += 2
			case 0x1b: // const-string/jumbo
				if pc+2 >= len(ins) {
					return nil, fmt.Errorf("código cortado en un const-string/jumbo")
				}
				idx := uint32(ins[pc+1]) | uint32(ins[pc+2])<<16
				if int(idx) >= len(d.strings) {
					return nil, fmt.Errorf("const-string/jumbo con la cadena %d fuera del pool", idx)
				}
				poner(aa, d.strings[idx])
				pc += 3
			case 0x1c: // const-class
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if int(w) >= len(d.types) {
					return nil, fmt.Errorf("const-class con el tipo %d fuera de type_ids", w)
				}
				poner(aa, &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
					"Ljava/lang/Class;.$tipo": d.types[w],
				}})
				pc += 2

			// --- objetos -----------------------------------------------------------------------
			case 0x1f: // check-cast
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if obj, ok := regs[aa].(*objetoDalvik); ok && int(w) < len(d.types) {
					obj.clase = d.types[w]
				}
				pc += 2
			case 0x20: // instance-of (22c)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				es := false
				if obj, ok := regs[int(u>>12)].(*objetoDalvik); ok && int(w) < len(d.types) {
					es = m.esDe(obj, d.types[w])
				}
				poner(aa&0xf, boolAInt(es))
				pc += 2
			case 0x21: // array-length
				arr, ok := regs[int(u>>12)].(*arregloDalvik)
				if !ok {
					return nil, fmt.Errorf("array-length sobre un registro que no es arreglo")
				}
				poner(int(u>>8)&0xf, int32(len(arr.datos)))
				pc++
			case 0x22: // new-instance
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if int(w) >= len(d.types) {
					return nil, fmt.Errorf("new-instance con el tipo %d fuera de type_ids", w)
				}
				poner(aa, &objetoDalvik{clase: d.types[w], campos: map[string]any{}})
				pc += 2
			case 0x23: // new-array (22c)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				n, err := aInt32(regs[int(u>>12)])
				if err != nil {
					return nil, fmt.Errorf("new-array: %w", err)
				}
				if n < 0 || n > 1<<20 {
					return nil, fmt.Errorf("new-array con longitud %d", n)
				}
				tipo := ""
				if int(w) < len(d.types) {
					tipo = d.types[w]
				}
				poner(int(u>>8)&0xf, &arregloDalvik{tipo: tipo, datos: make([]any, n)})
				pc += 2
			case 0x24: // filled-new-array (35c)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				t, err := tercera()
				if err != nil {
					return nil, err
				}
				tipo := ""
				if int(w) < len(d.types) {
					tipo = d.types[w]
				}
				regsArg := registros35c(u, t)
				datos := make([]any, 0, len(regsArg))
				for _, r := range regsArg {
					datos = append(datos, regs[r])
				}
				resultado = &arregloDalvik{tipo: tipo, datos: datos}
				hayResultado = true
				pc += 3
			case 0x25: // filled-new-array/range (3rc)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				t, err := tercera()
				if err != nil {
					return nil, err
				}
				tipo := ""
				if int(w) < len(d.types) {
					tipo = d.types[w]
				}
				primero, cuenta := int(t), int(u>>8)
				datos := make([]any, 0, cuenta)
				for i := 0; i < cuenta; i++ {
					datos = append(datos, regs[primero+i])
				}
				resultado = &arregloDalvik{tipo: tipo, datos: datos}
				hayResultado = true
				pc += 3
			case 0x26: // fill-array-data
				return nil, fmt.Errorf("%s.%s usa fill-array-data y el intérprete MVP no lo ejecuta",
					claseCorta(clase), met.nombre)
			case 0x27: // throw
				obj, ok := regs[aa].(*objetoDalvik)
				if !ok {
					if regs[aa] == nil {
						return nil, nuevaExcepcion("Ljava/lang/NullPointerException;",
							"throw sobre un objeto nulo en %s.%s", claseCorta(clase), met.nombre)
					}
					return nil, fmt.Errorf("%s.%s lanza algo que no es un objeto (tipo %T)",
						claseCorta(clase), met.nombre, regs[aa])
				}
				if !m.esThrowableDe(obj.clase) {
					return nil, fmt.Errorf("%s.%s lanza un %s, que no es un Throwable",
						claseCorta(clase), met.nombre, claseCorta(obj.clase))
				}
				return nil, &excepcionDalvik{tipo: obj.clase, objeto: obj}

			// --- saltos ------------------------------------------------------------------------
			case 0x28: // goto (10t)
				pc += int(int8(aa))
			case 0x29: // goto/16 (20t)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				pc += int(int16(w))
			case 0x2a: // goto/32 (30t)
				if pc+2 >= len(ins) {
					return nil, fmt.Errorf("código cortado en un goto/32")
				}
				pc += int(int32(uint32(ins[pc+1]) | uint32(ins[pc+2])<<16))
			case 0x2b, 0x2c: // packed-switch / sparse-switch
				salto, err := m.saltoDeSwitch(ins, pc, op == 0x2b, regs[aa])
				if err != nil {
					return nil, err
				}
				pc += salto
			case 0x2d, 0x2e, 0x2f, 0x30, 0x31: // cmp*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				a, b := regs[int(w)&0xff], regs[int(w)>>8]
				cmp, err := m.compara(op, a, b)
				if err != nil {
					return nil, err
				}
				poner(aa, int32(cmp))
				pc += 2
			case 0x32, 0x33, 0x34, 0x35, 0x36, 0x37: // if-* (22t)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				cumple, err := m.comparaIf(op, regs[int(u>>8)&0xf], regs[int(u>>12)])
				if err != nil {
					return nil, err
				}
				if cumple {
					pc += int(int16(w))
				} else {
					pc += 2
				}
			case 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d: // if-*z (21t)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				cumple, err := m.comparaIf(op-6, regs[aa], int32(0))
				if err != nil {
					return nil, err
				}
				if cumple {
					pc += int(int16(w))
				} else {
					pc += 2
				}

			// --- arreglos ----------------------------------------------------------------------
			case 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a: // aget*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				arr, ok := regs[int(w)&0xff].(*arregloDalvik)
				if !ok {
					return nil, fmt.Errorf("aget sobre un registro que no es arreglo")
				}
				idx, err := aInt32(regs[int(w)>>8])
				if err != nil {
					return nil, err
				}
				if int(idx) < 0 || int(idx) >= len(arr.datos) {
					return nil, fmt.Errorf("aget con el índice %d fuera del arreglo de %d", idx, len(arr.datos))
				}
				v := arr.datos[idx]
				if op == 0x45 {
					ponerAncho(aa, v)
				} else {
					poner(aa, v)
				}
				pc += 2
			case 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51: // aput*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				arr, ok := regs[int(w)&0xff].(*arregloDalvik)
				if !ok {
					return nil, fmt.Errorf("aput sobre un registro que no es arreglo")
				}
				idx, err := aInt32(regs[int(w)>>8])
				if err != nil {
					return nil, err
				}
				if int(idx) < 0 || int(idx) >= len(arr.datos) {
					return nil, fmt.Errorf("aput con el índice %d fuera del arreglo de %d", idx, len(arr.datos))
				}
				arr.datos[idx] = regs[aa]
				pc += 2

			// --- campos ------------------------------------------------------------------------
			case 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58: // iget*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				v, err := m.leeCampo(d, uint32(w), regs[int(u>>12)], false)
				if err != nil {
					return nil, err
				}
				if op == 0x53 {
					ponerAncho(int(u>>8)&0xf, v)
				} else {
					poner(int(u>>8)&0xf, v)
				}
				pc += 2
			case 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f: // iput*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if err := m.escribeCampo(d, uint32(w), regs[int(u>>12)], regs[int(u>>8)&0xf], false); err != nil {
					return nil, err
				}
				pc += 2
			case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66: // sget*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				v, err := m.leeCampo(d, uint32(w), nil, true)
				if err != nil {
					return nil, err
				}
				if op == 0x61 {
					ponerAncho(aa, v)
				} else {
					poner(aa, v)
				}
				pc += 2
			case 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d: // sput*
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				if err := m.escribeCampo(d, uint32(w), nil, regs[aa], true); err != nil {
					return nil, err
				}
				pc += 2

			// --- llamadas ----------------------------------------------------------------------
			case 0x6e, 0x6f, 0x70, 0x71, 0x72: // invoke-* (35c)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				t, err := tercera()
				if err != nil {
					return nil, err
				}
				v, err := m.invoca(op, d, uint32(w), registros35c(u, t), regs, clase)
				if err != nil {
					return nil, err
				}
				resultado, hayResultado = v, true
				pc += 3
			case 0x74, 0x75, 0x76, 0x77, 0x78: // invoke-*/range (3rc)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				t, err := tercera()
				if err != nil {
					return nil, err
				}
				primero, cuenta := int(t), int(u>>8)
				regsArg := make([]int, cuenta)
				for i := 0; i < cuenta; i++ {
					regsArg[i] = primero + i
				}
				v, err := m.invoca(op-6, d, uint32(w), regsArg, regs, clase)
				if err != nil {
					return nil, err
				}
				resultado, hayResultado = v, true
				pc += 3

			// --- unarios y conversiones --------------------------------------------------------
			case 0x7b, 0x7c, 0x7d, 0x7e, 0x7f, 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86,
				0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f:
				dst, src := int(u>>8)&0xf, int(u>>12)
				v, ancho, err := unaria(op, regs[src])
				if err != nil {
					return nil, err
				}
				if ancho == 2 {
					ponerAncho(dst, v)
				} else {
					poner(dst, v)
				}
				pc++

			// --- binarias ----------------------------------------------------------------------
			case 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a,
				0x9b, 0x9c, 0x9d, 0x9e, 0x9f, 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5,
				0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf:
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				v, ancho, err := binaria(op, regs[int(w)&0xff], regs[int(w)>>8])
				if err != nil {
					return nil, err
				}
				if ancho == 2 {
					ponerAncho(aa, v)
				} else {
					poner(aa, v)
				}
				pc += 2
			case 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba,
				0xbb, 0xbc, 0xbd, 0xbe, 0xbf, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5,
				0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf: // */2addr (12x)
				dst, src := int(u>>8)&0xf, int(u>>12)
				v, ancho, err := binaria(op-0x20, regs[dst], regs[src])
				if err != nil {
					return nil, err
				}
				if ancho == 2 {
					ponerAncho(dst, v)
				} else {
					poner(dst, v)
				}
				pc++
			case 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7: // lit16 (22s)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				lit := int32(int16(w))
				v, err := enteraLit(op-0xd0, regs[aa], lit)
				if err != nil {
					return nil, err
				}
				poner(aa, v)
				pc += 2
			case 0xd8, 0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0, 0xe1, 0xe2: // lit8 (22b)
				w, err := segunda()
				if err != nil {
					return nil, err
				}
				lit := int32(int8(w >> 8))
				v, err := enteraLit(op-0xd8, regs[int(w)&0xf], lit)
				if err != nil {
					return nil, err
				}
				poner(aa, v)
				pc += 2
			default:
				return nil, fmt.Errorf("%s.%s usa el opcode %#02x, que el intérprete MVP no decodifica",
					claseCorta(clase), met.nombre, op)
			}
		}
	}

	for {
		v, err := corre()
		if err == nil {
			return v, nil
		}
		ex := excepcionDe(err)
		if ex == nil {
			return nil, err
		}
		unidad, errBusca := m.buscaHandler(d, it, ex, pc)
		if errBusca != nil {
			return nil, errBusca
		}
		if unidad < 0 {
			// sin handler en este método: se propaga con su tipo y su mensaje, sin fingir éxito
			return nil, fmt.Errorf("%s.%s: %w", claseCorta(clase), met.nombre, ex)
		}
		pendiente = ex
		pc = unidad
	}
}

// registros35c reconstruye la lista de registros del formato 35c: el contador en la mitad alta de la
// primera palabra, los cuatro primeros repartidos en nibbles de la tercera y el quinto en la
// segunda. Nuestro emisor usa 3rc para más de cinco palabras, así que aquí nunca hay más de cinco.
func registros35c(u, tercera uint16) []int {
	n := int(u>>12) & 0xf
	g := int(u>>8) & 0xf
	out := make([]int, 0, 5)
	for k := 0; k < n && k < 4; k++ {
		out = append(out, int(tercera>>(4*k))&0xf)
	}
	if n == 5 {
		out = append(out, g)
	}
	return out
}

// saltoDeSwitch lee el payload de un switch y devuelve el desplazamiento que toca desde la
// instrucción del switch, o 3 si el valor no cae en ningún brazo (el tamaño de la propia
// instrucción: la ejecución sigue detrás).
//
// Un payload malformado se RECHAZA con error y no se ejecuta a medias. Las dos formas no miden lo
// mismo: la packed lleva cabecera de dos palabras (ident, tamaño), la primera clave y n destinos de
// dos palabras (2 + 2 + 2n), y la sparse lleva la cabecera, n claves y n destinos (2 + 4n). Antes se
// validaba solo base+2+n, que deja fuera la primera clave y los destinos: un packed-switch con dos
// brazos declarados y un único par de destinos leía ins[dst+1] fuera del cuerpo.
func (m *maquinaDalvik) saltoDeSwitch(ins []uint16, pc int, compacto bool, valor any) (int, error) {
	if pc+1 >= len(ins) {
		return 0, malformado("código cortado en un switch: la instrucción en la unidad %d no tiene su desplazamiento", pc)
	}
	base := pc + int(int16(ins[pc+1]))
	if base < 0 || base+1 >= len(ins) {
		return 0, malformado("el payload del switch de la unidad %d cae fuera del código de %d palabras", pc, len(ins))
	}
	ident := ins[base]
	nombre := "packed-switch"
	if compacto && ident != 0x0100 {
		return 0, malformado("un packed-switch apunta al payload %#04x y no a 0x0100", ident)
	}
	if !compacto {
		nombre = "sparse-switch"
		if ident != 0x0200 {
			return 0, malformado("un sparse-switch apunta al payload %#04x y no a 0x0200", ident)
		}
	}
	clave, err := aInt32(valor)
	if err != nil {
		return 0, fmt.Errorf("switch: %w", err)
	}
	n := int(ins[base+1])
	necesarias := base + 4 + 2*n
	if !compacto {
		necesarias = base + 2 + 4*n
	}
	if necesarias > len(ins) {
		return 0, malformado("payload de %s truncado: declara %d brazos y necesita %d palabras, el cuerpo tiene %d",
			nombre, n, necesarias, len(ins))
	}
	if compacto {
		primera := int32(uint32(ins[base+2]) | uint32(ins[base+3])<<16)
		indice := int(clave - primera)
		if indice < 0 || indice >= n {
			return 3, nil
		}
		dst := base + 4 + 2*indice
		if dst+1 >= len(ins) {
			return 0, malformado("el destino del brazo %d de un packed-switch cae fuera del cuerpo", indice)
		}
		// los destinos del payload son relativos a la INSTRUCCIÓN del switch, no al payload
		return int(int32(uint32(ins[dst]) | uint32(ins[dst+1])<<16)), nil
	}
	for k := 0; k < n; k++ {
		kpos := base + 2 + 2*k
		claveK := int32(uint32(ins[kpos]) | uint32(ins[kpos+1])<<16)
		if claveK != clave {
			continue
		}
		dpos := base + 2 + 2*n + 2*k
		if dpos+1 >= len(ins) {
			return 0, malformado("el destino del brazo %d de un sparse-switch cae fuera del cuerpo", k)
		}
		return int(int32(uint32(ins[dpos]) | uint32(ins[dpos+1])<<16)), nil
	}
	return 3, nil
}

// esDe responde a `instance-of` subiendo por la jerarquía del dex.
func (m *maquinaDalvik) esDe(obj *objetoDalvik, tipo string) bool {
	clase := obj.clase
	for i := 0; i < 32; i++ {
		if clase == tipo {
			return true
		}
		cl, ok := m.prog.clases[clase]
		if !ok {
			break
		}
		if cl.super == "" || cl.super == clase {
			break
		}
		clase = cl.super
	}
	return false
}

// --- campos ---------------------------------------------------------------------------------

func (m *maquinaDalvik) leeCampo(d *dexFile, idx uint32, receptor any, estatico bool) (any, error) {
	if d == nil {
		d = m.prog.dex
	}
	if int(idx) >= len(d.fields) {
		return nil, fmt.Errorf("campo %d fuera de field_ids", idx)
	}
	f := d.fields[idx]
	if estatico {
		return m.leeEstatico(f)
	}
	if receptor == nil {
		return nil, fmt.Errorf("se leyó el campo %s.%s de un objeto nulo", claseCorta(f.owner), f.name)
	}
	obj, ok := receptor.(*objetoDalvik)
	if !ok {
		return valorCero(f.typeOf), nil
	}
	if v, ok := obj.campos[claveCampo(f)]; ok {
		return v, nil
	}
	return valorCero(f.typeOf), nil
}

func (m *maquinaDalvik) leeEstatico(f dexField) (any, error) {
	// `System.out` y `System.err` son el destino de println; se representan como el flujo que son
	if f.owner == "Ljava/lang/System;" && (f.name == "out" || f.name == "err") {
		return &objetoDalvik{clase: "Ljava/io/PrintStream;", campos: map[string]any{}}, nil
	}
	if cl, ok := m.prog.clases[f.owner]; ok {
		if v, ok := cl.estaticos[claveCampo(f)]; ok {
			return v, nil
		}
	}
	return valorCero(f.typeOf), nil
}

func (m *maquinaDalvik) escribeCampo(d *dexFile, idx uint32, receptor any, v any, estatico bool) error {
	if d == nil {
		d = m.prog.dex
	}
	if int(idx) >= len(d.fields) {
		return fmt.Errorf("campo %d fuera de field_ids", idx)
	}
	f := d.fields[idx]
	if estatico {
		cl, ok := m.prog.clases[f.owner]
		if !ok {
			m.avisa("se escribió el estático %s.%s sin cargar su clase", claseCorta(f.owner), f.name)
			return nil
		}
		cl.estaticos[claveCampo(f)] = v
		return nil
	}
	if receptor == nil {
		return fmt.Errorf("se escribió el campo %s.%s en un objeto nulo", claseCorta(f.owner), f.name)
	}
	obj, ok := receptor.(*objetoDalvik)
	if !ok {
		return fmt.Errorf("se escribió el campo %s sobre un registro que no es objeto", f.name)
	}
	obj.campos[claveCampo(f)] = v
	return nil
}

// --- llamadas -------------------------------------------------------------------------------

// invoca resuelve una instrucción invoke: primero las clases del propio dex, y si el método no está
// ahí, los stubs de framework. `op` es el opcode del invoke (0x6e..0x72) y `palabras` los registros
// que llevan los argumentos, contados en palabras.
func (m *maquinaDalvik) invoca(op byte, d *dexFile, idx uint32, palabras []int, regs []any, claseActual string) (any, error) {
	if d == nil {
		d = m.prog.dex
	}
	if int(idx) >= len(d.methods) {
		return nil, fmt.Errorf("invoke con el método %d fuera de method_ids", idx)
	}
	ref := d.methods[idx]
	proto := descDeProto(ref.proto)

	esEstatico := op == 0x71
	var receptor any
	base := 0
	if !esEstatico {
		if len(palabras) == 0 {
			return nil, fmt.Errorf("invoke-virtual de %s.%s sin registros", claseCorta(ref.owner), ref.name)
		}
		receptor = regs[palabras[0]]
		if receptor == nil {
			return nil, fmt.Errorf("se llamó a %s.%s sobre un objeto nulo", claseCorta(ref.owner), ref.name)
		}
		base = 1
	}
	vals := make([]any, 0, len(ref.proto.params))
	tipos := make([]string, 0, len(ref.proto.params))
	pos := base
	for _, p := range ref.proto.params {
		if pos >= len(palabras) {
			return nil, fmt.Errorf("invoke de %s.%s: faltan registros para sus argumentos",
				claseCorta(ref.owner), ref.name)
		}
		vals = append(vals, regs[palabras[pos]])
		tipos = append(tipos, p)
		pos += anchoDe(p)
	}

	// 1) las clases del propio dex. La resolución no es la misma para las cinco formas: un
	// `invoke-direct` y un `invoke-super` van a la clase que DECLARA el método (el constructor de
	// Activity, no el de MainActivity, aunque el receptor sea la actividad); solo virtual e
	// interface despachan por la clase del receptor.
	en := func(clase string, met *metodoDalvik, args []any, argTipos []string) (any, error) {
		v, err := m.ejecutaCuerpo(clase, met, args, argTipos)
		if err != nil {
			return nil, fmt.Errorf("en %s.%s: %w", claseCorta(clase), met.nombre, err)
		}
		return v, nil
	}
	if esEstatico {
		if met := m.buscaMetodo(ref.owner, ref.name, proto); met != nil {
			return en(ref.owner, met, vals, tipos)
		}
	} else if op == 0x6f || op == 0x70 {
		if met := m.buscaMetodo(ref.owner, ref.name, proto); met != nil {
			args := append([]any{receptor}, vals...)
			argTipos := append([]string{ref.owner}, tipos...)
			return en(ref.owner, met, args, argTipos)
		}
	} else if obj, ok := receptor.(*objetoDalvik); ok {
		if met := m.buscaMetodo(obj.clase, ref.name, proto); met != nil {
			args := append([]any{obj}, vals...)
			argTipos := append([]string{obj.clase}, tipos...)
			return en(obj.clase, met, args, argTipos)
		}
	}

	// 2) los stubs de framework. La clase que declara el método es la del ref, pero un
	// `findViewById` invocado sobre MainActivity se declara en Activity: se prueban las dos, y la
	// cadena de padres del dex, porque el emisor declara un método heredado en la clase del
	// receptor (`new Tarea().execute()` llega como `Tarea.execute`, no como `AsyncTask.execute`).
	candidatos := []string{ref.owner}
	if obj, ok := receptor.(*objetoDalvik); ok {
		candidatos = append(m.jerarquiaDelDex(obj.clase), candidatos...)
	}
	if v, ok, err := m.intentaStub(candidatos, ref.name, proto, vals, receptor); ok || err != nil {
		return v, err
	}

	// 3) sin resolver: cero del tipo de retorno y aviso, que es lo que evita confundir una
	// aproximación con una ejecución
	m.avisa("%s.%s%s no está en el dex ni en los stubs: se devuelve el cero de %s",
		claseCorta(ref.owner), ref.name, proto, ref.proto.returnType)
	return valorCero(ref.proto.returnType), nil
}

// stubSinCuerpo atiende los métodos que el dex declara sin código: los `printInt`, `print` y
// `println` que una app usa como salida capturan su argumento; el resto se avisa.
func (m *maquinaDalvik) stubSinCuerpo(clase string, met *metodoDalvik, args []any) (any, error) {
	switch strings.ToLower(met.nombre) {
	case "printint", "print", "println", "log":
		if len(args) > 0 {
			m.captura(met.nombre, nil, textoDe(args[len(args)-1]))
		}
		return valorCero(met.retorno), nil
	}
	m.avisa("%s.%s no tiene cuerpo y no es un stub conocido", claseCorta(clase), met.nombre)
	return valorCero(met.retorno), nil
}

// rechaza deja el aviso de una API que el intérprete NO emula y devuelve su cero. Se usa para las
// que no tienen una aproximación honesta (formato, Editable/Spannable, reflexión): el aviso dice el
// nombre y el motivo, y el cero hace que la app se pare donde el valor nulo la pare, en vez de
// seguir con un dato inventado.
func (m *maquinaDalvik) rechaza(nombre, proto, motivo, retorno string) (any, bool, error) {
	if retorno != "" {
		m.avisa("%s%s: %s; se devuelve el cero de %s", nombre, proto, motivo, retorno)
	} else {
		m.avisa("%s%s: %s", nombre, proto, motivo)
	}
	return valorCero(retorno), true, nil
}

// esAlmacenDeEstado dice si un objeto representa un Bundle o un SharedPreferences, que son los dos
// sitios donde la app guarda estado con clave.
func esAlmacenDeEstado(obj *objetoDalvik) bool {
	return obj != nil && (obj.bundle != nil || obj.prefsFichero != "")
}

// valorDeEstado lee una clave del Bundle o del SharedPreferences del objeto.
func (m *maquinaDalvik) valorDeEstado(obj *objetoDalvik, clave string) (any, bool) {
	if obj == nil {
		return nil, false
	}
	if obj.bundle != nil {
		v, ok := obj.bundle[clave]
		return v, ok
	}
	if obj.prefsFichero != "" {
		if m.almacen == nil {
			return nil, false
		}
		v, ok := m.almacen.ficheros[obj.prefsFichero][clave]
		return v, ok
	}
	return nil, false
}

// guardaValor pone un valor con clave en el Bundle o en el editor del SharedPreferences, ya
// normalizado (texto, entero, booleano o long), para que el volcado a JSON sea el mismo en cada
// ejecución. Devuelve falso si la firma no es la de un `put*` que emulamos.
func guardaValor(destino map[string]any, nombre string, vals []any) bool {
	if len(vals) != 2 {
		return false
	}
	clave := textoDe(vals[0])
	switch nombre {
	case "putString":
		destino[clave] = textoDe(vals[1])
	case "putInt":
		n, err := aInt32(vals[1])
		if err != nil {
			return false
		}
		destino[clave] = n
	case "putBoolean":
		destino[clave] = boolDe(vals[1])
	case "putLong":
		n, err := aInt64(vals[1])
		if err != nil {
			return false
		}
		destino[clave] = n
	default:
		return false
	}
	return true
}

// boolDe lleva a booleano el valor de un registro, que en Dalvik es un entero.
func boolDe(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int32:
		return x != 0
	case int64:
		return x != 0
	}
	return false
}

// enteroDeEstado lleva a int64 un valor guardado, sin inventar: lo que no es un número vale cero,
// que es lo que devuelve una lectura de una clave ausente.
func enteroDeEstado(v any) int64 {
	switch x := v.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

// cadenaObjeto arma el String que devuelve una lectura de texto, como el que devuelve `getText`.
func cadenaObjeto(s string) *objetoDalvik {
	return &objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{}, texto: s}
}

// stubsFrameworkActivos es un interruptor de prueba: a false simula el emulador anterior a estos
// stubs, para que la falsificación pueda reproducir el error del ejecutor nulo. En producción vale
// siempre true; ninguna orden del preview lo cambia.
var stubsFrameworkActivos = true

// intentaStub resuelve las llamadas de framework que la app toca. `candidatos` son las clases por
// las que se busca (la del objeto, la que declara el método); los sufijos bastan porque lo que
// distingue un `setText` es el nombre y su firma, no la jerarquía entera del framework.
func (m *maquinaDalvik) intentaStub(candidatos []string, nombre, proto string, vals []any, receptor any) (any, bool, error) {
	if !stubsFrameworkActivos {
		switch nombre {
		case "getMainLooper", "getPackageManager", "getApplicationContext",
			"getInstalledPackages", "newSingleThreadExecutor", "setView", "setCancelable",
			"checkSelfPermission", "invalidate":
			return nil, false, nil
		}
	}
	es := func(sufijos ...string) bool {
		for _, c := range candidatos {
			for _, s := range sufijos {
				if strings.HasSuffix(c, s) {
					return true
				}
			}
		}
		return false
	}
	// las clases a las que se les reconoce texto y etiqueta de view: la View genérica que crea
	// `findViewById` (que un `check-cast` especializa) y los widgets que heredan de TextView
	esVista := func() bool {
		return es("Landroid/view/View;", "Landroid/widget/TextView;", "Landroid/widget/Button;",
			"Landroid/widget/EditText;", "Landroid/widget/CheckBox;", "Landroid/widget/Switch;",
			"Landroid/widget/RadioButton;", "Landroid/widget/CompoundButton;",
			"Landroid/widget/ToggleButton;", "Landroid/widget/CheckedTextView;",
			"Landroid/widget/ImageView;", "Landroid/widget/LinearLayout;",
			"Landroid/widget/FrameLayout;", "Landroid/widget/RelativeLayout;")
	}
	// esMarcable es el subconjunto de vistas con estado de marcado: el CompoundButton y sus hijos
	// (CheckBox, RadioButton, Switch, ToggleButton) y el CheckedTextView de las listas
	esMarcable := func() bool {
		return es("Landroid/widget/CheckBox;", "Landroid/widget/RadioButton;",
			"Landroid/widget/Switch;", "Landroid/widget/ToggleButton;",
			"Landroid/widget/CompoundButton;", "Landroid/widget/CheckedTextView;")
	}
	obj, _ := receptor.(*objetoDalvik)
	// esLista reconoce la instancia de `java.util.ArrayList` que creó `new ArrayList()`: el estado
	// vive en el objeto, así que cada lista lleva sus elementos y `get` respeta su índice. Se mira
	// la clase del receptor y no la del invoke porque un `List.add` llega declarado en la interfaz.
	esLista := func() bool {
		return obj != nil && strings.HasSuffix(obj.clase, "Ljava/util/ArrayList;")
	}
	// esConjunto reconoce el conjunto que devolvió `getBondedDevices`: no nulo aunque esté vacío.
	esConjunto := func() bool { return obj != nil && obj.conjunto != nil }
	// esIterador reconoce el cursor que devolvió `iterator()`, que recorre esa misma lista.
	esIterador := func() bool {
		return obj != nil && strings.HasSuffix(obj.clase, "Ljava/util/Iterator;")
	}
	// esThrowableObj reconoce una excepción ya construida: su mensaje y su texto son lo que un catch
	// puede leer del objeto que recibió. Se decide por la jerarquía real —una excepción del proyecto
	// como `MiExcepcion` no lleva el sufijo en el descriptor— y no por el nombre de la clase.
	esThrowableObj := func() bool { return obj != nil && m.esThrowableDe(obj.clase) }
	// esContexto reconoce la actividad que corre y el Application que devuelve
	// `getApplicationContext`: son los dos objetos sobre los que el framework atiende
	// `getPackageManager`, `getSystemService` y compañía. La actividad se reconoce por identidad;
	// el Application se marca al crearlo para que las llamadas posteriores sobre él también casen.
	esContexto := func() bool {
		if obj == nil {
			return false
		}
		if obj == m.actividad {
			return true
		}
		_, marcado := obj.campos["esContexto"]
		return marcado || strings.HasSuffix(obj.clase, "Landroid/app/Application;") ||
			strings.HasSuffix(obj.clase, "Landroid/content/Context;")
	}

	switch nombre {
	case "getMainLooper":
		// `Looper.getMainLooper()` es estático y devuelve siempre el mismo objeto: el Handler que
		// se construye con él solo necesita una referencia no nula para existir.
		if len(vals) == 0 && es("Landroid/os/Looper;") {
			return &objetoDalvik{clase: "Landroid/os/Looper;", campos: map[string]any{}}, true, nil
		}
	case "getPackageManager":
		// el PackageManager de la app: un objeto con identidad que se pasa a los adaptadores y desde
		// el que se piden los paquetes instalados (una lista vacía, sin sistema, más abajo).
		if len(vals) == 0 && esContexto() {
			return &objetoDalvik{clase: "Landroid/content/pm/PackageManager;",
				campos: map[string]any{}}, true, nil
		}
	case "getApplicationContext":
		// el Application es distinto de la Activity, como en el framework; se marca como contexto
		// para que las llamadas posteriores sobre él se reconozcan igual que sobre la actividad.
		if len(vals) == 0 && esContexto() {
			return &objetoDalvik{clase: "Landroid/app/Application;",
				campos: map[string]any{"esContexto": true}}, true, nil
		}
	case "checkSelfPermission":
		// PERMISSION_GRANTED es cero en Android. La vista previa no tiene sistema de permisos;
		// conceder es lo que ya hacía el cero implícito del intérprete, así que el valor explícito
		// quita el aviso sin cambiar la rama que hoy se ejecuta. Se atiende tanto la llamada sobre
		// la actividad (Context) como la estática de ContextCompat.
		if len(vals) == 1 && (esContexto() || es("Landroidx/core/content/ContextCompat;")) {
			return int32(0), true, nil
		}
	case "getInstalledPackages":
		// no hay sistema de paquetes en la vista previa: la lista es vacía, pero no nula, para que
		// un `for (PackageInfo info : pm.getInstalledPackages(...))` termine sin un receptor nulo.
		if len(vals) <= 1 && obj != nil && es("Landroid/content/pm/PackageManager;") {
			return &objetoDalvik{clase: "Ljava/util/ArrayList;", campos: map[string]any{},
				lista: []any{}}, true, nil
		}
	case "newSingleThreadExecutor":
		// el ejecutor de un solo hilo: un objeto con identidad sobre el que `execute` encola el
		// Runnable. No hay hilos reales, pero la cola lógica da el mismo orden y es determinista.
		if len(vals) == 0 && es("Ljava/util/concurrent/Executors;") {
			return &objetoDalvik{clase: "Ljava/util/concurrent/ExecutorService;",
				campos: map[string]any{}}, true, nil
		}
	case "startActivity":
		if len(vals) == 1 && obj != nil && obj == m.actividad {
			if o, ok := vals[0].(*objetoDalvik); ok && o.clase == "Landroid/content/Intent;" && o.intentClase != "" {
				m.pendiente = o.intentClase
				m.intentoSaliente = o
				return nil, true, nil
			}
			return m.rechaza(nombre, proto,
				"solo se emula un Intent de clase propia, `new Intent(this, Otra.class)`", "")
		}
	case "startActivityForResult":
		if obj != nil && obj == m.actividad {
			return m.rechaza(nombre, proto,
				"startActivityForResult no está en el subconjunto: usa startActivity", "")
		}
	case "finish":
		if len(vals) == 0 && obj != nil && obj == m.actividad {
			m.terminada = true
			return nil, true, nil
		}
	case "getIntent":
		if len(vals) == 0 && obj != nil && obj == m.actividad {
			if m.intent != nil {
				return m.intent, true, nil
			}
			return &objetoDalvik{clase: "Landroid/content/Intent;", campos: map[string]any{}}, true, nil
		}
	case "setClass":
		if len(vals) == 2 && es("Landroid/content/Intent;") && obj != nil {
			obj.intentClase = claseDeLiteral(vals[1])
			return obj, true, nil
		}
	case "<init>":
		// el constructor de una clase del framework (Activity, Object, la vista que sea): no hay
		// cuerpo que ejecutar y no hay nada que capturar. La excepción es `StringBuilder(String)`,
		// que javac usa con el primer trozo ya dentro (`new StringBuilder("Pulsos: ")`): sin esto,
		// el `+` de javac salía sin su parte literal y el texto se quedaba en el número (medido:
		// «1»). Solo esa forma: `StringBuilder(int)` es la CAPACIDAD, no contenido.
		//
		// `Intent(Context, Class)` guarda el destino: es lo único que `startActivity` necesita para
		// abrir una actividad propia, y el literal `X.class` llega como objeto de `java.lang.Class`.
		if obj != nil && es("Landroid/content/Intent;") &&
			proto == "(Landroid/content/Context;Ljava/lang/Class;)V" && len(vals) == 2 {
			obj.intentClase = claseDeLiteral(vals[1])
		}
		// el constructor de un ArrayAdapter recibe sus elementos: se guardan para el respaldo de
		// getCount/getItem/getView de una subclase del proyecto que no los reescriba. Un arreglo
		// vacío es el estado inicial de una lista (§335): `append` sobre nil devolvería nil y
		// entonces getCount no tendría respaldo; `make` deja una lista vacía NO nula.
		if obj != nil && es("Landroid/widget/ArrayAdapter;") {
			for _, v := range vals {
				if arr, ok := v.(*arregloDalvik); ok {
					elementos := make([]any, len(arr.datos))
					copy(elementos, arr.datos)
					obj.elementos = elementos
					break
				}
			}
		}
		if obj != nil && proto == "(Ljava/lang/String;)V" && es("Ljava/lang/StringBuilder;") {
			obj.cadena = &strings.Builder{}
			obj.cadena.WriteString(textoDe(vals[0]))
		}
		// un Throwable guarda el mensaje que recibe en su constructor: es lo que `getMessage()`
		// devuelve en el catch y lo que enseña el aviso cuando nadie la captura
		if esThrowableObj() && len(vals) >= 1 &&
			(proto == "(Ljava/lang/String;)V" || proto == "(Ljava/lang/String;Ljava/lang/Throwable;)V") {
			obj.mensajeExcepcion = textoDe(vals[0])
		}
		// el ArrayList del framework nace con su lista vacía: así `add` puede empezar y `clear`
		// puede dejarla vacía sin que se confunda con «este objeto no es una lista»
		if obj != nil && es("Ljava/util/ArrayList;") && obj.lista == nil {
			obj.lista = []any{}
		}
		// un Paint nace negro y sin trazo, como el del framework: es el color que un onDraw
		// publica si no llama a setColor
		if obj != nil && es("Landroid/graphics/Paint;") && !obj.paintPuesto {
			obj.paintColor = 0xff000000
		}
		return nil, true, nil
	case "onCreate", "onStart", "onResume", "onPause", "onStop", "onDestroy", "onRestart",
		"onSaveInstanceState", "onRestoreInstanceState", "onPostCreate", "onPostResume":
		if es("Landroid/app/Activity;", "Landroid/app/ListActivity;", "Landroid/app/ActivityGroup;") {
			return nil, true, nil
		}
	case "getSharedPreferences":
		// el SharedPreferences de la app: un almacén en memoria por nombre de fichero, compartido por
		// todas las pantallas y por la instancia recreada. El fichero no se crea hasta que se escribe,
		// para que el volcado no publique un almacén que la app solo leyó.
		if len(vals) == 2 && obj != nil && obj == m.actividad {
			nombre := textoDe(vals[0])
			return &objetoDalvik{clase: "Landroid/content/SharedPreferences;",
				campos: map[string]any{}, prefsFichero: nombre}, true, nil
		}
	case "edit":
		if len(vals) == 0 && obj != nil && obj.prefsFichero != "" {
			return &objetoDalvik{clase: "Landroid/content/SharedPreferences$Editor;",
				campos: map[string]any{}, prefsFichero: obj.prefsFichero,
				editor: map[string]any{}}, true, nil
		}
	case "putString", "putInt", "putBoolean", "putLong":
		// los mismos cuatro `put*` sirven para el Bundle y para el Editor de SharedPreferences; se
		// distinguen por el destino que lleva el objeto
		if obj != nil {
			if obj.editor != nil && guardaValor(obj.editor, nombre, vals) {
				return obj, true, nil
			}
			if obj.bundle != nil && guardaValor(obj.bundle, nombre, vals) {
				return nil, true, nil
			}
		}
	case "apply", "commit":
		// `apply` y `commit` vuelcan el editor en el almacén: commit devuelve true, apply no devuelve
		// nada, como en el framework. El almacén no toca el disco.
		if obj != nil && obj.editor != nil {
			destino := m.almacen.de(obj.prefsFichero)
			for k, v := range obj.editor {
				destino[k] = v
			}
			obj.editor = map[string]any{}
			if nombre == "commit" {
				return int32(1), true, nil
			}
			return nil, true, nil
		}
	case "getInt", "getLong", "getBoolean":
		if len(vals) >= 1 && len(vals) <= 2 && esAlmacenDeEstado(obj) {
			v, ok := m.valorDeEstado(obj, textoDe(vals[0]))
			if !ok {
				if len(vals) == 2 {
					switch nombre {
					case "getInt":
						n, _ := aInt32(vals[1])
						return n, true, nil
					case "getLong":
						n, _ := aInt64(vals[1])
						return n, true, nil
					}
					return boolAInt(boolDe(vals[1])), true, nil
				}
				if nombre == "getLong" {
					return int64(0), true, nil
				}
				return int32(0), true, nil
			}
			switch nombre {
			case "getInt":
				return int32(enteroDeEstado(v)), true, nil
			case "getLong":
				return enteroDeEstado(v), true, nil
			}
			return boolAInt(boolDe(v)), true, nil
		}
	case "setContentView":
		if len(vals) == 1 {
			if id, err := aInt32(vals[0]); err == nil {
				m.layoutID = uint32(id)
				return nil, true, nil
			}
			if o, ok := vals[0].(*objetoDalvik); ok && o.id != 0 {
				m.layoutID = o.id
				return nil, true, nil
			}
		}
	case "findViewById":
		if len(vals) == 1 {
			id, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			// dentro de una vista inflada, el findViewById es el de esa instancia: dos filas del
			// mismo layout no comparten sus vistas, como no las comparten en el aparato
			return m.vistaDeInstancia(obj, uint32(id)), true, nil
		}
	case "setAdapter":
		// la lista guarda el adaptador y se registra para que el modelo le pida sus filas después
		// de los clics, que es cuando su contenido está estabilizado
		if len(vals) == 1 && obj != nil {
			obj.adaptador = vals[0]
			repetida := false
			for _, l := range m.listas {
				if l == obj {
					repetida = true
					break
				}
			}
			if !repetida {
				m.listas = append(m.listas, obj)
			}
			return nil, true, nil
		}
	case "setOnItemClickListener":
		if len(vals) == 1 && obj != nil {
			m.oyentesItem = append(m.oyentesItem, oyenteItemRegistrado{oyente: vals[0], vista: obj})
			return nil, true, nil
		}
	case "notifyDataSetChanged":
		// el modelo vuelve a pedir getCount/getView al pintar, así que el aviso de cambio solo
		// tiene que existir: no hay caché que invalidar
		if len(vals) == 0 && obj != nil &&
			(m.esDe(obj, "Landroid/widget/BaseAdapter;") || m.esDe(obj, "Landroid/widget/ArrayAdapter;")) {
			return nil, true, nil
		}
	case "getCount":
		// solo el respaldo del ArrayAdapter del framework: una subclase del proyecto que lo
		// reescriba ya lo ejecutó el despacho normal
		if len(vals) == 0 && obj != nil && obj.elementos != nil {
			return int32(len(obj.elementos)), true, nil
		}
	case "getItem":
		if len(vals) == 1 && obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			if i < 0 || int(i) >= len(obj.menuItems) {
				return nil, true, fmt.Errorf("Menu.getItem(%d) fuera de los %d ítems",
					i, len(obj.menuItems))
			}
			return obj.menuItems[int(i)], true, nil
		}
		if len(vals) == 1 && obj != nil && obj.elementos != nil {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			if i < 0 || int(i) >= len(obj.elementos) {
				return nil, true, fmt.Errorf("getItem(%d) fuera de las %d filas del adaptador",
					i, len(obj.elementos))
			}
			return obj.elementos[i], true, nil
		}
	case "getItemId":
		if len(vals) == 0 && obj != nil && es("Landroid/view/MenuItem;") {
			return obj.itemID, true, nil
		}
		if len(vals) == 1 && obj != nil && obj.elementos != nil {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			return int64(i), true, nil
		}
	case "getChildCount":
		// la aproximación documentada: los hijos de la lista son las filas del adaptador
		if len(vals) == 0 && obj != nil && obj.adaptador != nil {
			n, err := m.cuentaDelAdaptador(obj.adaptador)
			if err != nil {
				m.avisa("getChildCount de la lista %#08x: %v", obj.id, err)
				return int32(0), true, nil
			}
			return int32(n), true, nil
		}
	case "getChildAt":
		if len(vals) == 1 && obj != nil && obj.adaptador != nil {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			v, err := m.elementoDelAdaptador(obj.adaptador, int(i), obj)
			if err != nil {
				m.avisa("getChildAt(%d) de la lista %#08x: %v", i, obj.id, err)
				return nil, true, nil
			}
			return v, true, nil
		}
	case "getLayoutInflater":
		if len(vals) == 0 && obj != nil && obj == m.actividad {
			return &objetoDalvik{clase: "Landroid/view/LayoutInflater;", campos: map[string]any{}}, true, nil
		}
	case "inflate":
		// `inflate(layout, padre, attach)`: la fila nace con el layout que pide y un mapa de vistas
		// propio, que es lo que hace que su findViewById no devuelva las de otra fila
		if len(vals) >= 1 && es("Landroid/view/LayoutInflater;") {
			id, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			return &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{},
				layoutID: uint32(id), vistas: map[uint32]*objetoDalvik{}}, true, nil
		}
		// `MenuInflater.inflate`: los menús XML de `res/menu` no están en el subconjunto del
		// intérprete; se dice al llamarlos en vez de devolver un menú vacío que parecería que no hay
		// ítems en la app
		if len(vals) == 2 && obj != nil && es("Landroid/view/MenuInflater;") {
			return m.rechaza(nombre, proto,
				"los menús XML (`res/menu`) no se inflan en la vista previa: añade los ítems con "+
					"`menu.add` en onCreateOptionsMenu", "Landroid/view/Menu;")
		}
	case "from":
		if len(vals) == 1 && es("Landroid/view/LayoutInflater;") {
			return &objetoDalvik{clase: "Landroid/view/LayoutInflater;", campos: map[string]any{}}, true, nil
		}
	case "getContext":
		// el contexto de una view es la actividad que la infló: es lo que el adaptador necesita
		// para construir sus filas
		if len(vals) == 0 && obj != nil && m.actividad != nil {
			return m.actividad, true, nil
		}
	case "invalidate":
		// `View.invalidate()` solo pide al framework un dibujado futuro. El modelo ejecuta el
		// `onDraw` una vez al construir la pantalla, con una instancia nueva de la vista, y no tiene
		// ciclo de fotogramas: no hay redibujado que programar ni estado de la instancia que viaje.
		// Se silencia el aviso y no se deja huella (ni captura ni contador), porque publicar un
		// efecto que el modelo no puede dibujar sería inventar lo que la app no ha medido.
		if len(vals) == 0 && esVista() {
			return nil, true, nil
		}
	case "onDraw":
		// `super.onDraw(canvas)` de una vista propia. El paso 1 ya ejecutó el `onDraw` del dex
		// cuando existe: lo único que llega aquí es el `View.onDraw` del framework, que en Android
		// dibuja nada. Es un no-op honesto, no un silencio: el cuerpo propio sigue corriendo en
		// `dibujaVista` y publicando sus operaciones.
		if len(vals) == 1 && es("Landroid/view/View;") {
			return nil, true, nil
		}
	case "getWidth", "getHeight":
		// El tamaño de la vista es el que el layout ya midió y se sembró al construir la instancia
		// del `onDraw`. Sin él se devuelve cero, que es el valor que el modelo daba antes: se sigue
		// publicando el cero (con su aviso) para no inventar una medida donde no hay rect.
		if len(vals) == 0 && obj != nil && esVista() && obj.anchoVista > 0 {
			if nombre == "getWidth" {
				return obj.anchoVista, true, nil
			}
			return obj.altoVista, true, nil
		}
	case "setOnClickListener":
		if len(vals) == 1 {
			if o := obj; o != nil {
				o.oyente = vals[0]
				m.oyentes = append(m.oyentes, oyenteRegistrado{oyente: vals[0], vista: o})
			}
			return nil, true, nil
		}
	case "setOnMenuItemClickListener":
		// el oyente del ítem; el último gana, como en el framework, y se devuelve el ítem para no
		// romper la cadena de `menu.add(...).setOnMenuItemClickListener(...)`
		if len(vals) == 1 && obj != nil && es("Landroid/view/MenuItem;") {
			obj.itemOyente = vals[0]
			return obj, true, nil
		}
	case "getTitle", "getTitleCondensed":
		if len(vals) == 0 && obj != nil && es("Landroid/view/MenuItem;") {
			return cadenaObjeto(obj.itemTitulo), true, nil
		}
	case "getOrder":
		if len(vals) == 0 && obj != nil && es("Landroid/view/MenuItem;") {
			return obj.itemOrden, true, nil
		}
	case "setVisible":
		if len(vals) == 1 && obj != nil && es("Landroid/view/MenuItem;") {
			obj.itemVisible = boolDe(vals[0])
			return obj, true, nil
		}
	case "isVisible":
		if len(vals) == 0 && obj != nil && es("Landroid/view/MenuItem;") {
			return boolAInt(obj.itemVisible), true, nil
		}
	case "setEnabled":
		if len(vals) == 1 && obj != nil && es("Landroid/view/MenuItem;") {
			obj.itemHabilitado = boolDe(vals[0])
			return obj, true, nil
		}
	case "setCheckable":
		if len(vals) == 1 && obj != nil && es("Landroid/view/MenuItem;") {
			// el marco del check se modela con `itemMarcado`; `setCheckable(false)` lo apaga
			if !boolDe(vals[0]) {
				obj.itemMarcado = false
			}
			return obj, true, nil
		}
	case "setIcon", "setShowAsAction", "setShowAsActionFlags", "setAlphabeticShortcut",
		"setNumericShortcut", "setShortcut", "setIntent":
		// el icono, la acción en la barra y los atajos no cambian lo que se ve en el desplegable de la
		// vista previa; se aceptan y se devuelve el ítem para conservar la cadena
		if obj != nil && es("Landroid/view/MenuItem;") {
			return obj, true, nil
		}
	case "findItem":
		if len(vals) == 1 && obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
			id, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			for _, item := range obj.menuItems {
				if item.itemID == id {
					return item, true, nil
				}
			}
			return nil, true, nil
		}
	case "removeItem", "removeGroup":
		if len(vals) == 1 && obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
			if nombre == "removeItem" {
				id, err := aInt32(vals[0])
				if err != nil {
					return nil, true, err
				}
				for i, item := range obj.menuItems {
					if item.itemID == id {
						obj.menuItems = append(obj.menuItems[:i], obj.menuItems[i+1:]...)
						break
					}
				}
			} else {
				grupo, err := aInt32(vals[0])
				if err != nil {
					return nil, true, err
				}
				quedan := obj.menuItems[:0]
				for _, item := range obj.menuItems {
					if item.itemGrupo != grupo {
						quedan = append(quedan, item)
					}
				}
				obj.menuItems = quedan
			}
			return nil, true, nil
		}
	case "hasVisibleItems":
		if len(vals) == 0 && obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
			for _, item := range obj.menuItems {
				if item.itemVisible {
					return boolAInt(true), true, nil
				}
			}
			return boolAInt(false), true, nil
		}
	case "getMenuInflater":
		// el inflador existe para que una app que lo pida no reciba nulo; `inflate` de un menú XML no
		// está en el subconjunto y lo dice al llamarlo
		if len(vals) == 0 && obj != nil && obj == m.actividad {
			return &objetoDalvik{clase: "Landroid/view/MenuInflater;", campos: map[string]any{}}, true, nil
		}
	case "invalidateOptionsMenu", "openOptionsMenu", "closeOptionsMenu", "setQwertyMode":
		// el guion reconstruye el menú en cada `--menu` y `--menu-item` cierra solo: lo que la app
		// pide con estas llamadas no tiene nada más que hacer aquí
		if obj != nil && obj == m.actividad {
			return nil, true, nil
		}
	case "post", "postDelayed":
		// el reloj lógico: `Handler.post`/`postDelayed` y `View.post`/`postDelayed` encolan el
		// `run()` del Runnable con su instante y no lo ejecutan. `post` es un `postDelayed` de cero.
		if len(vals) >= 1 && (es("Landroid/os/Handler;") || esVista()) {
			espera := int64(0)
			if nombre == "postDelayed" {
				if len(vals) != 2 {
					return nil, true, fmt.Errorf("%s%s: se esperaban dos argumentos, el Runnable y el retraso",
						nombre, proto)
				}
				if n, err := aInt64(vals[1]); err == nil {
					espera = n
				} else {
					return nil, true, err
				}
			}
			return m.difiere(nombre, vals[0], espera), true, nil
		}
	case "removeCallbacks":
		// retira de la cola las tareas que ese mismo Runnable dejó pendientes: es el «parar» de un
		// cronómetro. El Runnable tiene que ser el objeto del dex que se posteó.
		if len(vals) >= 1 && es("Landroid/os/Handler;") {
			r, ok := vals[0].(*objetoDalvik)
			if !ok {
				return nil, true, fmt.Errorf("%s%s con un Runnable que no es un objeto del dex",
					nombre, proto)
			}
			m.quitaTareas(r)
			return nil, true, nil
		}
	case "removeCallbacksAndMessages":
		// el token nulo vacía la cola, como el framework; un token no nulo no casa con nada porque
		// aquí no se modelan mensajes con token, y entonces no cancela nada. Un nulo llega al
		// registro como el entero cero, no como un objeto: cualquier valor que no sea un objeto del
		// dex es el token nulo.
		if len(vals) == 1 && es("Landroid/os/Handler;") {
			if _, esObjeto := vals[0].(*objetoDalvik); !esObjeto {
				m.cola = nil
			}
			return nil, true, nil
		}
	case "elapsedRealtime":
		// el reloj monotónico de `SystemClock` es el reloj lógico de la vista previa: no hay reloj de
		// pared, así que dos ejecuciones del mismo guion leen los mismos milisegundos
		if len(vals) == 0 && es("Landroid/os/SystemClock;") {
			return m.ahora(), true, nil
		}
	case "getDefaultAdapter":
		// el adaptador de Bluetooth se modela presente y encendido, con cero equipos vinculados: es
		// lo que la UI puede enseñar sin un dispositivo. La radio no existe aquí.
		if len(vals) == 0 && es("Landroid/bluetooth/BluetoothAdapter;") {
			return &objetoDalvik{clase: "Landroid/bluetooth/BluetoothAdapter;",
				campos: map[string]any{}, bluetoothActivo: true}, true, nil
		}
	case "isEnabled":
		if len(vals) == 0 && obj != nil && es("Landroid/view/MenuItem;") {
			return boolAInt(obj.itemHabilitado), true, nil
		}
		if len(vals) == 0 && obj != nil && es("Landroid/bluetooth/BluetoothAdapter;") {
			return boolAInt(obj.bluetoothActivo), true, nil
		}
	case "getBondedDevices":
		if len(vals) == 0 && obj != nil && es("Landroid/bluetooth/BluetoothAdapter;") {
			return &objetoDalvik{clase: "Ljava/util/Set;", campos: map[string]any{},
				conjunto: []any{}}, true, nil
		}
	case "startDiscovery":
		// el descubrimiento se acepta y devuelve true, pero no entrega resultados: no hay receptor
		// ni radio. Es la frontera documentada de `chatbt`.
		if len(vals) == 0 && obj != nil && es("Landroid/bluetooth/BluetoothAdapter;") {
			return boolAInt(true), true, nil
		}
	case "createRfcommSocketToServiceRecord", "listenUsingRfcommWithServiceRecord", "getRemoteDevice":
		if obj != nil && es("Landroid/bluetooth/BluetoothAdapter;") {
			// el cero que se devuelve es el del tipo de retorno de cada método: un socket, un
			// servidor de sockets o un dispositivo, no siempre el mismo
			retorno := "Landroid/bluetooth/BluetoothSocket;"
			if nombre == "listenUsingRfcommWithServiceRecord" {
				retorno = "Landroid/bluetooth/BluetoothServerSocket;"
			} else if nombre == "getRemoteDevice" {
				retorno = "Landroid/bluetooth/BluetoothDevice;"
			}
			return m.rechaza(nombre, proto,
				"los sockets y los dispositivos Bluetooth reales no están en el subconjunto: no hay "+
					"radio ni red", retorno)
		}
	case "iterator":
		if len(vals) == 0 {
			if esConjunto() {
				return &objetoDalvik{clase: "Ljava/util/Iterator;", campos: map[string]any{},
					iterando: obj.conjunto}, true, nil
			}
			// el `for (X x : lista)` que emite javac llama a `List.iterator()` aunque el receptor
			// sea un ArrayList: sin esto el iterador llegaba nulo y el bucle reventaba en hasNext.
			if esLista() {
				return &objetoDalvik{clase: "Ljava/util/Iterator;", campos: map[string]any{},
					iterando: append([]any(nil), obj.lista...)}, true, nil
			}
		}
	case "hasNext":
		if len(vals) == 0 && esIterador() {
			return boolAInt(obj.indice < len(obj.iterando)), true, nil
		}
	case "next":
		if len(vals) == 0 && esIterador() {
			if obj.indice >= len(obj.iterando) {
				return nil, true, nuevaExcepcion("Ljava/util/NoSuchElementException;",
					"next() sin elementos: el conjunto está vacío")
			}
			v := obj.iterando[obj.indice]
			obj.indice++
			return v, true, nil
		}
	case "add":
		// `Menu.add`: el ítem nace con su título (o su id de cadena) y se publica en el menú de la
		// pantalla; devuelve el propio ítem para la cadena `menu.add(...).setOnMenuItemClickListener(...)`
		if obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
			item, err := m.itemDeMenu(vals)
			if err != nil {
				return nil, true, err
			}
			obj.menuItems = append(obj.menuItems, item)
			return item, true, nil
		}
		// `add(E)` y `add(int, E)` del ArrayList: el segundo respeta el índice e inserta delante de
		// lo que ya estaba, como el framework
		if esLista() {
			if len(vals) == 1 {
				obj.lista = append(obj.lista, vals[0])
				return boolAInt(true), true, nil
			}
			if len(vals) == 2 {
				i, err := aInt32(vals[0])
				if err != nil {
					return nil, true, err
				}
				if int(i) < 0 || int(i) > len(obj.lista) {
					return nil, true, nuevaExcepcion("Ljava/lang/IndexOutOfBoundsException;",
						"Index: %d, Size: %d", i, len(obj.lista))
				}
				obj.lista = append(obj.lista, nil)
				copy(obj.lista[int(i)+1:], obj.lista[int(i):])
				obj.lista[int(i)] = vals[1]
				return nil, true, nil
			}
		}
	case "get":
		if esLista() && len(vals) == 1 {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			if int(i) < 0 || int(i) >= len(obj.lista) {
				return nil, true, nuevaExcepcion("Ljava/lang/IndexOutOfBoundsException;",
					"Index %d out of bounds for length %d", i, len(obj.lista))
			}
			return obj.lista[int(i)], true, nil
		}
	case "size":
		if len(vals) == 0 {
			if obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
				return int32(len(obj.menuItems)), true, nil
			}
			if esLista() {
				return int32(len(obj.lista)), true, nil
			}
			if esConjunto() {
				return int32(len(obj.conjunto)), true, nil
			}
		}
	case "clear":
		if len(vals) == 0 && obj != nil && es("Landroid/view/Menu;", "Landroid/view/SubMenu;") {
			obj.menuItems = nil
			return nil, true, nil
		}
		if len(vals) == 0 && esLista() {
			obj.lista = []any{}
			return nil, true, nil
		}
	case "runOnUiThread":
		// `Activity.runOnUiThread` desde el hilo de la interfaz corre en el acto, y aquí no hay otro
		// hilo: la ejecución es inmediata, sin pasar por la cola
		if len(vals) == 1 && obj != nil && obj == m.actividad {
			r, ok := vals[0].(*objetoDalvik)
			if !ok {
				return nil, true, fmt.Errorf("runOnUiThread con un Runnable que no es un objeto del dex")
			}
			if err := m.ejecutaRunnable(r, nombre); err != nil {
				return nil, true, err
			}
			return nil, true, nil
		}
	case "execute":
		if len(vals) == 1 && obj != nil && es("Landroid/os/AsyncTask;") {
			m.difiereTarea(obj, vals[0])
			return obj, true, nil
		}
		// `ExecutorService.execute(Runnable)` no tiene hilos aquí, así que el Runnable entra en la
		// cola lógica como un `post` y corre cuando el guion avanza el reloj. Es lo mismo que hace el
		// AsyncTask y conserva el orden y el determinismo del emulador.
		if len(vals) == 1 && obj != nil && es("Ljava/util/concurrent/ExecutorService;",
			"Ljava/util/concurrent/Executor;") {
			return m.difiere("ExecutorService.execute", vals[0], 0), true, nil
		}
	case "executeOnExecutor":
		if obj != nil && es("Landroid/os/AsyncTask;") {
			return m.rechaza(nombre, proto,
				"los ejecutores no están en el subconjunto: usa execute()", "Landroid/os/AsyncTask;")
		}
	case "setOnCheckedChangeListener":
		// el framework guarda un solo oyente: el último registrado sustituye al anterior
		if len(vals) == 1 && obj != nil && esMarcable() {
			obj.oyenteMarca = vals[0]
			return nil, true, nil
		}
	case "addTextChangedListener":
		// los vigilantes se acumulan, como en TextView; el que se registra dos veces se avisa dos
		// veces, que es lo que hace el framework con una lista de listeners
		if len(vals) == 1 && obj != nil && esVista() {
			obj.vigilantesTexto = append(obj.vigilantesTexto, vals[0])
			return nil, true, nil
		}
	case "isChecked":
		if len(vals) == 0 && obj != nil && es("Landroid/view/MenuItem;") {
			return boolAInt(obj.itemMarcado), true, nil
		}
		if len(vals) == 0 && obj != nil && esMarcable() {
			return boolAInt(obj.marcado), true, nil
		}
	case "setChecked":
		// `setChecked` cambia el estado y avisa al oyente si el valor cambió; `toggle` es su
		// inverso. En los dos el receptor tiene que ser una vista marcable.
		if len(vals) == 1 && obj != nil && es("Landroid/view/MenuItem;") {
			obj.itemMarcado = boolDe(vals[0])
			return obj, true, nil
		}
		if len(vals) == 1 && obj != nil && esMarcable() {
			n, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			m.marcaVista(obj, n != 0)
			return nil, true, nil
		}
	case "toggle":
		if len(vals) == 0 && obj != nil && esMarcable() {
			m.marcaVista(obj, !obj.marcado)
			return nil, true, nil
		}
	case "setHint":
		if len(vals) == 1 && obj != nil && esVista() {
			pista := textoDe(vals[0])
			if resuelto, ok := m.textoDeId(vals[0]); ok {
				pista = resuelto
			}
			obj.pista = pista
			if obj.id != 0 {
				m.pistas[obj.id] = pista
			}
			return nil, true, nil
		}
	case "getHint":
		if len(vals) == 0 && obj != nil && esVista() {
			return &objetoDalvik{clase: "Ljava/lang/CharSequence;", campos: map[string]any{},
				texto: obj.pista}, true, nil
		}
	case "setText":
		if len(vals) == 1 && es("Landroid/widget/TextView;", "Landroid/widget/Button;",
			"Landroid/widget/EditText;", "Landroid/widget/CheckBox;", "Landroid/widget/Switch;",
			"Landroid/widget/RadioButton;", "Landroid/widget/CompoundButton;") {
			texto := textoDe(vals[0])
			// `setText(R.string.x)` llega como su número: se resuelve contra la tabla, que es lo
			// que el layout hace con `@string/x`. Un literal cualquiera no cae aquí porque la
			// tabla solo resuelve ids de SU paquete.
			if resuelto, ok := m.textoDeId(vals[0]); ok {
				texto = resuelto
			}
			m.captura("TextView.setText", obj, texto)
			return nil, true, nil
		}
	case "getText":
		// `TextView.getText()` devuelve el CharSequence que la vista tiene AHORA, que es lo que
		// `setText` escribió en `obj.texto` o lo que el layout declaró. Se devuelve un objeto nuevo
		// de clase String y no la vista: si se devolviera la vista, el `append` de la concatenación
		// le escribiría en su `cadena` y mezclaría el texto de la vista con el del StringBuilder.
		// Con un objeto aparte, cada llamada lleva su propio estado y `toString` lo lee tal cual. La
		// identidad no se reproduce —dos `getText()` no son el mismo objeto, como sí lo son en el
		// framework—; el texto sí, que es lo que se puede medir en una pantalla.
		if len(vals) == 0 && obj != nil && esVista() {
			return &objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{},
				texto: obj.texto}, true, nil
		}
	case "append":
		// dos `append` distintos: el de StringBuilder, que es el que emite el `+` de javac, y el
		// de TextView, que añade al texto de la vista. Se distinguen por la clase del receptor.
		if len(vals) == 1 && obj != nil {
			if esVista() {
				m.captura("TextView.append", obj, obj.texto+textoDe(vals[0]))
				return nil, true, nil
			}
			if obj.cadena == nil {
				obj.cadena = &strings.Builder{}
			}
			// un `append(boolean)` llega como 0/1 y el framework lo imprime «true»/«false»: la firma
			// del invoke es lo único que distingue un booleano de un entero
			if strings.HasPrefix(proto, "(Z)") {
				obj.cadena.WriteString(strconv.FormatBool(textoDe(vals[0]) != "0"))
			} else {
				obj.cadena.WriteString(textoDe(vals[0]))
			}
			return obj, true, nil
		}
	case "getString":
		// getString(clave, porDefecto) del Bundle o del SharedPreferences devuelve el texto guardado,
		// y getString(clave) devuelve null si no está, como el framework
		if len(vals) == 2 && esAlmacenDeEstado(obj) {
			if v, ok := m.valorDeEstado(obj, textoDe(vals[0])); ok {
				return cadenaObjeto(textoDe(v)), true, nil
			}
			return textoDe(vals[1]), true, nil
		}
		if len(vals) == 1 && esAlmacenDeEstado(obj) {
			if v, ok := m.valorDeEstado(obj, textoDe(vals[0])); ok {
				return cadenaObjeto(textoDe(v)), true, nil
			}
			return nil, true, nil
		}
		// `getString(R.string.x)` lo resuelve la actividad contra la tabla. El dueño del invoke es
		// la clase de la actividad (el emisor no escribe `android/content/Context;`), así que se
		// reconoce por el receptor y no por el nombre de la clase.
		if len(vals) == 1 && obj != nil && obj == m.actividad {
			texto, ok := m.textoDeId(vals[0])
			if !ok {
				return nil, true, fmt.Errorf("getString(%s): la tabla no resuelve ese id como una cadena",
					textoDe(vals[0]))
			}
			return &objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{},
				texto: texto}, true, nil
		}
		if len(vals) > 1 && obj != nil && obj == m.actividad {
			// con argumentos es formato (`getString(id, args)`), y el formato de cadenas no está
			// en el subconjunto: se dice por su nombre en vez de devolver el texto sin sustituir
			return m.rechaza(nombre, proto, "el formato de cadenas de getString no está en el subconjunto",
				"Ljava/lang/String;")
		}
	case "getId":
		if len(vals) == 0 && obj != nil {
			return int32(obj.id), true, nil
		}
	case "setTag":
		if len(vals) == 1 && obj != nil && esVista() {
			obj.tag = vals[0]
			return nil, true, nil
		}
	case "getTag":
		// la etiqueta puede no estar puesta y entonces el framework devuelve null: aquí también,
		// porque encadenar `getTag().toString()` sin mirarla revienta igual en el aparato
		if len(vals) == 0 && obj != nil && esVista() {
			return obj.tag, true, nil
		}
	case "getEditableText":
		// EditText.getEditableText() devuelve un Editable; aquí se emula como el texto plano de la
		// vista, que es lo único que una app usa de él (`toString`, `length`, `charAt`). Los tramos y
		// los spans siguen fuera del subconjunto: no hay nada que medir en ellos y no se finge.
		if len(vals) == 0 && obj != nil && esVista() {
			return &objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{},
				texto: obj.texto}, true, nil
		}
	case "fromHtml":
		if es("Landroid/text/Html;") {
			return m.rechaza(nombre, proto,
				"el formato con Html no está en el subconjunto: solo se emula texto plano",
				"Landroid/text/Spanned;")
		}
	case "forName", "getMethod", "getDeclaredMethod", "newInstance", "invoke":
		if esReflexion(candidatos) {
			return m.rechaza(nombre, proto,
				"la reflexión no está en el subconjunto", "")
		}
	case "setView", "setCancelable":
		// los dos devuelven el propio builder en el framework: sin esto, la cadena
		// `builder.setView(...).setCancelable(...).create().show()` recibía el cero y la app se
		// paraba. La vista del `setView` se guarda y `setCancelable` solo se acepta: ni una ni otra
		// cambian el título, el mensaje o los botones que publica el modelo.
		if len(vals) == 1 && obj != nil && es("Landroid/app/AlertDialog$Builder;") {
			if nombre == "setView" {
				obj.campos["dialogo.vista"] = vals[0]
			}
			return obj, true, nil
		}
	case "setTitle", "setMessage":
		// el título y el mensaje de un `AlertDialog.Builder`: se guardan para publicarlos como capa.
		// El `setTitle` de la actividad sigue siendo otro: lo distingue la clase del receptor.
		if nombre == "setTitle" && len(vals) == 1 && obj != nil && es("Landroid/view/MenuItem;") {
			// `MenuItem.setTitle(CharSequence | int res)`: el texto del ítem, que es lo que se ve en
			// el desplegable y lo que publica el modelo
			obj.itemTitulo = m.textoDeArgumento(vals[0])
			return obj, true, nil
		}
		if len(vals) == 1 && obj != nil && es("Landroid/app/AlertDialog$Builder;") {
			clave := campoDialogoTitulo
			if nombre == "setMessage" {
				clave = campoDialogoMensaje
			}
			obj.campos[clave] = m.textoDeArgumento(vals[0])
			return obj, true, nil
		}
		if nombre == "setTitle" && len(vals) == 1 && es("Landroid/app/Activity;") {
			m.captura("Activity.setTitle", obj, textoDe(vals[0]))
			return nil, true, nil
		}
	case "setPositiveButton", "setNegativeButton", "setNeutralButton":
		// el texto y el `DialogInterface.OnClickListener` de cada botón; el builder se devuelve a sí
		// mismo para que la cadena de `setTitle().setMessage().show()` siga viva
		if len(vals) == 2 && obj != nil && es("Landroid/app/AlertDialog$Builder;") {
			tipo := tipoDeBotonDialogo[nombre]
			obj.campos[campoBotonDialogo(tipo, "texto")] = m.textoDeArgumento(vals[0])
			obj.campos[campoBotonDialogo(tipo, "oyente")] = vals[1]
			return obj, true, nil
		}
	case "create":
		if len(vals) == 0 && obj != nil && es("Landroid/app/AlertDialog$Builder;") {
			return creaDialogo(obj), true, nil
		}
	case "show":
		// `show()` sobre el builder crea el diálogo y lo pone delante; sobre un Toast deja su línea
		// de consola y el aviso efímero; sobre un diálogo ya creado solo lo apila.
		if obj != nil && es("Landroid/widget/Toast;") {
			texto := ""
			if s, ok := obj.campos["toast.texto"].(string); ok {
				texto = s
			}
			m.anotaConsola("Toast", texto)
			m.toast = texto
			return nil, true, nil
		}
		if len(vals) == 0 && obj != nil && es("Landroid/app/AlertDialog$Builder;") {
			d := creaDialogo(obj)
			m.apilaDialogo(d)
			return d, true, nil
		}
		if len(vals) == 0 && obj != nil && es("Landroid/app/AlertDialog;", "Landroid/app/Dialog;") {
			m.apilaDialogo(obj)
			return nil, true, nil
		}
	case "dismiss", "cancel", "hide":
		// `hide` deja el diálogo vivo en el framework, pero en el modelo no hay diferencia visible
		// entre ocultarlo y cerrarlo: los dos lo quitan de la capa que se pinta.
		if len(vals) == 0 && obj != nil && es("Landroid/app/AlertDialog;", "Landroid/app/Dialog;") {
			m.cierraDialogo(obj)
			return nil, true, nil
		}
	case "isShowing":
		if len(vals) == 0 && obj != nil && es("Landroid/app/AlertDialog;", "Landroid/app/Dialog;") {
			return boolAInt(m.dialogoAbierto(obj)), true, nil
		}
	case "makeText":
		// `Toast.makeText(Context, CharSequence | int, int)`: se guarda el texto en el Toast y su
		// `show()` lo publica; la duración no cambia lo que se mide
		if len(vals) == 3 && es("Landroid/widget/Toast;") {
			t := &objetoDalvik{clase: "Landroid/widget/Toast;", campos: map[string]any{}}
			t.campos["toast.texto"] = m.textoDeArgumento(vals[1])
			return t, true, nil
		}
	case "getMessage", "getLocalizedMessage":
		if len(vals) == 0 && esThrowableObj() {
			return obj.mensajeExcepcion, true, nil
		}
	case "printStackTrace":
		if len(vals) == 0 && esThrowableObj() {
			return nil, true, nil
		}
	case "toString":
		// el `toString` de una excepción es su clase con el mensaje: es lo que un catch imprime
		if len(vals) == 0 && esThrowableObj() {
			if obj.mensajeExcepcion == "" {
				return claseCorta(obj.clase), true, nil
			}
			return claseCorta(obj.clase) + ": " + obj.mensajeExcepcion, true, nil
		}
		if len(vals) == 0 && es("Ljava/lang/StringBuilder;") && obj != nil && obj.cadena != nil {
			return obj.cadena.String(), true, nil
		}
		// un String devuelto por `getText`/`getString` se convierte en su texto aunque esté vacío:
		// el `textoDe` de más abajo devolvería el nombre de la clase, que es lo que hacía que un
		// «toString» de una cadena vacía se leyera como «java.lang.String»
		if len(vals) == 0 {
			if s, ok := cadenaDe(receptor); ok {
				return s, true, nil
			}
		}
		if len(vals) == 0 && obj != nil {
			return textoDe(obj), true, nil
		}
	case "length":
		if len(vals) == 0 && es("Ljava/lang/StringBuilder;") && obj != nil && obj.cadena != nil {
			return int32(obj.cadena.Len()), true, nil
		}
		// `length()` de String y de CharSequence cuenta unidades de 16 bits, no bytes: «á» es una
		// unidad y dos bytes
		if len(vals) == 0 {
			if s, ok := cadenaDe(receptor); ok {
				return int32(len(unidadesDeCadena(s))), true, nil
			}
		}
	case "isEmpty":
		if len(vals) == 0 {
			if s, ok := cadenaDe(receptor); ok {
				return boolAInt(s == ""), true, nil
			}
			if esLista() {
				return boolAInt(len(obj.lista) == 0), true, nil
			}
			if esConjunto() {
				return boolAInt(len(obj.conjunto) == 0), true, nil
			}
		}
	case "equals":
		// el `equals` de String compara contenido; con cualquier otra cosa (o con null) es falso,
		// que es lo que dice su contrato
		if len(vals) == 1 {
			if s, ok := cadenaDe(receptor); ok {
				otro, esCadena := cadenaDe(vals[0])
				return boolAInt(esCadena && s == otro), true, nil
			}
		}
	case "setColor":
		// `Paint.setColor(int)`: el color con el que se pintará el trazo siguiente. Sin esto, el
		// dibujo de una vista propia saldría con el negro por defecto y no con lo que pide la app.
		if len(vals) == 1 && obj != nil && es("Landroid/graphics/Paint;") {
			if c, ok := enteroDeDibujo(vals[0]); ok {
				obj.paintColor = c
				obj.paintPuesto = true
			}
			return nil, true, nil
		}
	case "rgb", "argb":
		// `Color.rgb` y `Color.argb` comparten el entero ARGB que `Paint.setColor` guarda y que el
		// modelo publica como `#rrggbb` (o `#aarrggbb` cuando el alfa no es opaco). Sin esto los
		// colores de la app caían en cero y el pintor los convertía en `rgba(0,0,0,0)`.
		cuantos := 3
		if nombre == "argb" {
			cuantos = 4
		}
		if es("Landroid/graphics/Color;") && len(vals) == cuantos {
			partes := make([]uint32, cuantos)
			valido := true
			for i, v := range vals {
				n, ok := enteroDeDibujo(v)
				if !ok || n > 0xff {
					valido = false
					break
				}
				partes[i] = n & 0xff
			}
			if valido {
				argb := uint32(0xff000000)
				if cuantos == 4 {
					argb = partes[0] << 24
					partes = partes[1:]
				}
				return int32(argb | partes[0]<<16 | partes[1]<<8 | partes[2]), true, nil
			}
		}
	case "getColor":
		if len(vals) == 0 && obj != nil && es("Landroid/graphics/Paint;") {
			return int32(colorDePaint(obj)), true, nil
		}
	case "setStrokeWidth":
		if len(vals) == 1 && obj != nil && es("Landroid/graphics/Paint;") {
			obj.paintGrosor = flotanteDeDibujo(vals[0])
			return nil, true, nil
		}
	case "getStrokeWidth":
		if len(vals) == 0 && obj != nil && es("Landroid/graphics/Paint;") {
			return float32(obj.paintGrosor), true, nil
		}
	case "setTextSize":
		// `Paint.setTextSize(float)`: el tamaño que `drawText` ha de publicar con el texto. Antes
		// caía al stub genérico, el aviso lo delataba y el modelo dibujaba sin tamaño ninguno.
		if len(vals) == 1 && obj != nil && es("Landroid/graphics/Paint;") {
			obj.paintTamano = flotanteDeDibujo(vals[0])
			return nil, true, nil
		}
	case "getTextSize":
		if len(vals) == 0 && obj != nil && es("Landroid/graphics/Paint;") {
			return float32(obj.paintTamano), true, nil
		}
	case "setStyle", "setAntiAlias", "setStrokeCap", "setStrokeJoin", "setDither", "setFilterBitmap":
		// propiedades de Paint que no cambian el trazo que el modelo publica: se aceptan sin
		// inventar un efecto que no se dibuja
		if obj != nil && es("Landroid/graphics/Paint;") {
			return nil, true, nil
		}
	case "drawColor":
		if len(vals) == 1 && obj != nil && es("Landroid/graphics/Canvas;") {
			if c, ok := enteroDeDibujo(vals[0]); ok {
				m.anotaDibujo(operacionDibujo{Operacion: "color", Color: formatoColor(c)})
			}
			return nil, true, nil
		}
	case "drawRGB":
		// `Canvas.drawRGB(int r, int g, int b)` compone un color opaco. Antes compartía el `case` de
		// `drawColor`, que exige un argumento: una llamada real de tres caía al stub genérico y el
		// color no se publicaba. Cada canal se recorta a ocho bits, como hace el framework.
		if len(vals) == 3 && obj != nil && es("Landroid/graphics/Canvas;") {
			r, rok := enteroDeDibujo(vals[0])
			g, gok := enteroDeDibujo(vals[1])
			b, bok := enteroDeDibujo(vals[2])
			if rok && gok && bok {
				c := 0xff000000 | (r&0xff)<<16 | (g&0xff)<<8 | (b & 0xff)
				m.anotaDibujo(operacionDibujo{Operacion: "color", Color: formatoColor(c)})
			}
			return nil, true, nil
		}
	case "drawARGB":
		// `Canvas.drawARGB(int a, int r, int g, int b)`: la misma composición con alfa explícito.
		if len(vals) == 4 && obj != nil && es("Landroid/graphics/Canvas;") {
			a, aok := enteroDeDibujo(vals[0])
			r, rok := enteroDeDibujo(vals[1])
			g, gok := enteroDeDibujo(vals[2])
			b, bok := enteroDeDibujo(vals[3])
			if aok && rok && gok && bok {
				c := (a&0xff)<<24 | (r&0xff)<<16 | (g&0xff)<<8 | (b & 0xff)
				m.anotaDibujo(operacionDibujo{Operacion: "color", Color: formatoColor(c)})
			}
			return nil, true, nil
		}
	case "drawLine":
		if len(vals) == 5 && obj != nil && es("Landroid/graphics/Canvas;") {
			m.anotaDibujo(operacionDibujo{
				Operacion: "linea",
				X1:        flotanteDeDibujo(vals[0]),
				Y1:        flotanteDeDibujo(vals[1]),
				X2:        flotanteDeDibujo(vals[2]),
				Y2:        flotanteDeDibujo(vals[3]),
				Color:     colorDeArgumento(vals[4]),
				Grosor:    grosorDeArgumento(vals[4]),
			})
			return nil, true, nil
		}
	case "drawRect":
		if len(vals) == 5 && obj != nil && es("Landroid/graphics/Canvas;") {
			m.anotaDibujo(operacionDibujo{
				Operacion: "rectangulo",
				X1:        flotanteDeDibujo(vals[0]),
				Y1:        flotanteDeDibujo(vals[1]),
				X2:        flotanteDeDibujo(vals[2]),
				Y2:        flotanteDeDibujo(vals[3]),
				Color:     colorDeArgumento(vals[4]),
				Grosor:    grosorDeArgumento(vals[4]),
			})
			return nil, true, nil
		}
	case "drawCircle":
		if len(vals) == 4 && obj != nil && es("Landroid/graphics/Canvas;") {
			m.anotaDibujo(operacionDibujo{
				Operacion: "circulo",
				X1:        flotanteDeDibujo(vals[0]),
				Y1:        flotanteDeDibujo(vals[1]),
				Radio:     flotanteDeDibujo(vals[2]),
				Color:     colorDeArgumento(vals[3]),
				Grosor:    grosorDeArgumento(vals[3]),
			})
			return nil, true, nil
		}
	case "drawText":
		// `drawText(String, x, y, Paint)` y su forma con CharSequence: el texto se publica como
		// operación, con el tamaño que su Paint lleva, sin medirlo con una fuente que el modelo
		// no tiene.
		if len(vals) == 4 && obj != nil && es("Landroid/graphics/Canvas;") {
			m.anotaDibujo(operacionDibujo{
				Operacion: "texto",
				Texto:     textoDe(vals[0]),
				X1:        flotanteDeDibujo(vals[1]),
				Y1:        flotanteDeDibujo(vals[2]),
				Color:     colorDeArgumento(vals[3]),
				Grosor:    grosorDeArgumento(vals[3]),
				Tamano:    tamanoDeArgumento(vals[3]),
			})
			return nil, true, nil
		}
	case "d", "i", "w", "e", "v", "wtf":
		if es("Landroid/util/Log;") {
			texto := ""
			if len(vals) >= 2 {
				texto = textoDe(vals[1])
			}
			m.captura("Log."+nombre, nil, texto)
			return int32(0), true, nil
		}
	case "println", "print":
		if es("Ljava/io/PrintStream;", "Ljava/io/OutputStream;") {
			if len(vals) >= 1 {
				m.captura("println", nil, textoDe(vals[0]))
			}
			return nil, true, nil
		}
	case "valueOf":
		if len(vals) == 1 && es("Ljava/lang/String;") {
			return textoDe(vals[0]), true, nil
		}
	case "parseInt":
		if len(vals) >= 1 && es("Ljava/lang/Integer;", "Ljava/lang/Long;") {
			n, err := strconv.ParseInt(strings.TrimSpace(textoDe(vals[0])), 10, 32)
			if err != nil {
				// el framework lanza NumberFormatException, que un `try` del proyecto puede
				// capturar: se lanza con su tipo, no como un fallo del intérprete
				return nil, true, nuevaExcepcion("Ljava/lang/NumberFormatException;",
					`For input string: %q`, textoDe(vals[0]))
			}
			if proto == "(Ljava/lang/String;)J" {
				return n, true, nil
			}
			return int32(n), true, nil
		}
	case "charAt":
		if len(vals) == 1 && es("Ljava/lang/String;", "Ljava/lang/CharSequence;") {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			s, ok := cadenaDe(receptor)
			if !ok {
				return nil, true, fmt.Errorf("charAt sobre un receptor que no es una cadena")
			}
			unidades := unidadesDeCadena(s)
			if int(i) < 0 || int(i) >= len(unidades) {
				return nil, true, fmt.Errorf("charAt con el índice %d fuera de la cadena", i)
			}
			return int32(unidades[i]), true, nil
		}
	}
	return nil, false, nil
}

// --- operaciones ----------------------------------------------------------------------------

func aInt32(v any) (int32, error) {
	switch x := v.(type) {
	case int32:
		return x, nil
	case int64:
		return int32(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un entero")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un entero", v)
}

func aInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int32:
		return int64(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un long")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un long", v)
}

func aFloat32(v any) (float32, error) {
	switch x := v.(type) {
	case float32:
		return x, nil
	case int32:
		// un registro no está tipado: el patrón de bits de un `const` puede ser un float
		return math.Float32frombits(uint32(x)), nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un float")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un float", v)
}

func aFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case int64:
		return math.Float64frombits(uint64(x)), nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un double")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un double", v)
}

func boolAInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// esReferencia: lo que se compara por identidad en Dalvik.
func esReferencia(v any) bool {
	switch v.(type) {
	case nil, *objetoDalvik, *arregloDalvik, string:
		return true
	}
	return false
}

// igualesDalvik es el `==` de Dalvik: identidad para objetos y arreglos, contenido para cadenas.
func igualesDalvik(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch x := a.(type) {
	case *objetoDalvik:
		y, ok := b.(*objetoDalvik)
		return ok && x == y
	case *arregloDalvik:
		y, ok := b.(*arregloDalvik)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	}
	return false
}

func (m *maquinaDalvik) comparaIf(op byte, a, b any) (bool, error) {
	if op == 0x32 || op == 0x33 { // if-eq / if-ne
		eq := false
		// en Dalvik el registro nulo y el cero entero son el mismo patrón de bits: `if-eqz` sobre una
		// referencia compara con null e `if (x != null)` llega aquí con el cero como otro operando
		if (a == nil && b == int32(0)) || (b == nil && a == int32(0)) {
			eq = true
		} else if esReferencia(a) || esReferencia(b) {
			eq = igualesDalvik(a, b)
		} else {
			av, err := aInt64(a)
			if err != nil {
				return false, err
			}
			bv, err := aInt64(b)
			if err != nil {
				return false, err
			}
			eq = av == bv
		}
		if op == 0x32 {
			return eq, nil
		}
		return !eq, nil
	}
	av, err := aInt64(a)
	if err != nil {
		return false, err
	}
	bv, err := aInt64(b)
	if err != nil {
		return false, err
	}
	switch op {
	case 0x34: // if-lt
		return av < bv, nil
	case 0x35: // if-ge
		return av >= bv, nil
	case 0x36: // if-gt
		return av > bv, nil
	case 0x37: // if-le
		return av <= bv, nil
	}
	return false, fmt.Errorf("comparación %#02x desconocida", op)
}

func (m *maquinaDalvik) compara(op byte, a, b any) (int, error) {
	switch op {
	case 0x2d, 0x2e: // cmpl-float / cmpg-float
		av, err := aFloat32(a)
		if err != nil {
			return 0, err
		}
		bv, err := aFloat32(b)
		if err != nil {
			return 0, err
		}
		if math.IsNaN(float64(av)) || math.IsNaN(float64(bv)) {
			if op == 0x2d {
				return -1, nil
			}
			return 1, nil
		}
		return cmpFloat(float64(av), float64(bv)), nil
	case 0x2f, 0x30: // cmpl-double / cmpg-double
		av, err := aFloat64(a)
		if err != nil {
			return 0, err
		}
		bv, err := aFloat64(b)
		if err != nil {
			return 0, err
		}
		if math.IsNaN(av) || math.IsNaN(bv) {
			if op == 0x2f {
				return -1, nil
			}
			return 1, nil
		}
		return cmpFloat(av, bv), nil
	case 0x31: // cmp-long
		av, err := aInt64(a)
		if err != nil {
			return 0, err
		}
		bv, err := aInt64(b)
		if err != nil {
			return 0, err
		}
		switch {
		case av < bv:
			return -1, nil
		case av > bv:
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("comparación %#02x desconocida", op)
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// unaria ejecuta las conversiones y los negados/notados de 12x.
func unaria(op byte, v any) (any, int, error) {
	switch op {
	case 0x7b: // neg-int
		x, err := aInt32(v)
		return -x, 1, err
	case 0x7c: // not-int
		x, err := aInt32(v)
		return ^x, 1, err
	case 0x7d: // neg-long
		x, err := aInt64(v)
		return -x, 2, err
	case 0x7e: // not-long
		x, err := aInt64(v)
		return ^x, 2, err
	case 0x7f: // neg-float
		x, err := aFloat32(v)
		return -x, 1, err
	case 0x80: // neg-double
		x, err := aFloat64(v)
		return -x, 2, err
	case 0x81: // int-to-long
		x, err := aInt32(v)
		return int64(x), 2, err
	case 0x82: // int-to-float
		x, err := aInt32(v)
		return float32(x), 1, err
	case 0x83: // int-to-double
		x, err := aInt32(v)
		return float64(x), 2, err
	case 0x84: // long-to-int
		x, err := aInt64(v)
		return int32(x), 1, err
	case 0x85: // long-to-float
		x, err := aInt64(v)
		return float32(x), 1, err
	case 0x86: // long-to-double
		x, err := aInt64(v)
		return float64(x), 2, err
	case 0x87: // float-to-int
		x, err := aFloat32(v)
		return int32(x), 1, err
	case 0x88: // float-to-long
		x, err := aFloat32(v)
		return int64(x), 2, err
	case 0x89: // float-to-double
		x, err := aFloat32(v)
		return float64(x), 2, err
	case 0x8a: // double-to-int
		x, err := aFloat64(v)
		return int32(x), 1, err
	case 0x8b: // double-to-long
		x, err := aFloat64(v)
		return int64(x), 2, err
	case 0x8c: // double-to-float
		x, err := aFloat64(v)
		return float32(x), 1, err
	case 0x8d: // int-to-byte
		x, err := aInt32(v)
		return int32(int8(x)), 1, err
	case 0x8e: // int-to-char
		x, err := aInt32(v)
		return int32(uint16(x)), 1, err
	case 0x8f: // int-to-short
		x, err := aInt32(v)
		return int32(int16(x)), 1, err
	}
	return nil, 1, fmt.Errorf("unario %#02x desconocido", op)
}

// binaria ejecuta las operaciones de 23x. El desplazamiento del int va enmascarado a cinco bits y el
// del long a seis, como manda Dalvik, no como deja el hardware.
func binaria(op byte, a, b any) (any, int, error) {
	switch {
	case op >= 0x90 && op <= 0x9a: // int
		x, err := aInt32(a)
		if err != nil {
			return nil, 1, err
		}
		y, err := aInt32(b)
		if err != nil {
			return nil, 1, err
		}
		v, err := entera(op-0x90, x, y)
		return v, 1, err
	case op >= 0x9b && op <= 0xa5: // long
		x, err := aInt64(a)
		if err != nil {
			return nil, 2, err
		}
		y, err := aInt64(b)
		if err != nil {
			return nil, 2, err
		}
		v, err := larga(op-0x9b, x, y)
		return v, 2, err
	case op >= 0xa6 && op <= 0xaa: // float
		x, err := aFloat32(a)
		if err != nil {
			return nil, 1, err
		}
		y, err := aFloat32(b)
		if err != nil {
			return nil, 1, err
		}
		v, err := flotante(op-0xa6, x, y)
		return v, 1, err
	case op >= 0xab && op <= 0xaf: // double
		x, err := aFloat64(a)
		if err != nil {
			return nil, 2, err
		}
		y, err := aFloat64(b)
		if err != nil {
			return nil, 2, err
		}
		v, err := doble(op-0xab, x, y)
		return v, 2, err
	}
	return nil, 1, fmt.Errorf("binaria %#02x desconocida", op)
}

// Las operaciones se nombran por su índice dentro de la familia: 0 suma, 1 resta, 2 producto, 3
// división, 4 resto, 5 and, 6 or, 7 xor, 8 desplazamiento a la izquierda, 9 a la derecha con signo
// y 10 a la derecha sin signo.
func entera(k byte, a, b int32) (int32, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		if b == 0 {
			return 0, nuevaExcepcion("Ljava/lang/ArithmeticException;", "/ by zero")
		}
		return a / b, nil
	case 4:
		if b == 0 {
			return 0, nuevaExcepcion("Ljava/lang/ArithmeticException;", "/ by zero")
		}
		return a % b, nil
	case 5:
		return a & b, nil
	case 6:
		return a | b, nil
	case 7:
		return a ^ b, nil
	case 8:
		return a << (uint32(b) & 31), nil
	case 9:
		return a >> (uint32(b) & 31), nil
	case 10:
		return int32(uint32(a) >> (uint32(b) & 31)), nil
	}
	return 0, fmt.Errorf("operación entera %d desconocida", k)
}

func larga(k byte, a, b int64) (int64, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		if b == 0 {
			return 0, nuevaExcepcion("Ljava/lang/ArithmeticException;", "/ by zero")
		}
		return a / b, nil
	case 4:
		if b == 0 {
			return 0, nuevaExcepcion("Ljava/lang/ArithmeticException;", "/ by zero")
		}
		return a % b, nil
	case 5:
		return a & b, nil
	case 6:
		return a | b, nil
	case 7:
		return a ^ b, nil
	case 8:
		return a << (uint64(b) & 63), nil
	case 9:
		return a >> (uint64(b) & 63), nil
	case 10:
		return int64(uint64(a) >> (uint64(b) & 63)), nil
	}
	return 0, fmt.Errorf("operación de long %d desconocida", k)
}

func flotante(k byte, a, b float32) (float32, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		return a / b, nil
	case 4:
		return float32(math.Mod(float64(a), float64(b))), nil
	}
	return 0, nil
}

func doble(k byte, a, b float64) (float64, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		return a / b, nil
	case 4:
		return math.Mod(a, b), nil
	}
	return 0, nil
}

// enteraLit es el `lit16`/`lit8`: los mismos índices que `entera`, con la resta invertida
// (`rsub-int` es el literal menos el registro) y los desplazamientos al final.
func enteraLit(k byte, valor any, lit int32) (int32, error) {
	a, err := aInt32(valor)
	if err != nil {
		return 0, err
	}
	switch k {
	case 0:
		return a + lit, nil
	case 1:
		return lit - a, nil
	case 2:
		return a * lit, nil
	case 3:
		if lit == 0 {
			return 0, nuevaExcepcion("Ljava/lang/ArithmeticException;", "/ by zero")
		}
		return a / lit, nil
	case 4:
		if lit == 0 {
			return 0, nuevaExcepcion("Ljava/lang/ArithmeticException;", "/ by zero")
		}
		return a % lit, nil
	case 5:
		return a & lit, nil
	case 6:
		return a | lit, nil
	case 7:
		return a ^ lit, nil
	case 8:
		return a << (uint32(lit) & 31), nil
	case 9:
		return a >> (uint32(lit) & 31), nil
	case 10:
		return int32(uint32(a) >> (uint32(lit) & 31)), nil
	}
	return 0, fmt.Errorf("operación con literal %d desconocida", k)
}
