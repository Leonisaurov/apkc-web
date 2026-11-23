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
//     texto que cambia sin que nadie toque la pantalla no es un emulador. Los clics llegan por
//     `--pulsar id/pulsar`, en el orden en que se piden y dentro de la misma ejecución, así que el
//     estado que ve el `onClick` es el que dejó `onCreate` y los clics anteriores. Repetir la lista
//     reproduce la misma pantalla: el emulador no guarda estado entre comandos y no lo finge.
//   - la salida de la app (`System.out.println`, `print` y `Log.*`) se anota además en `consola`, en
//     orden de emisión y numerada con `secuencia`, que es el panel de consola de la pestaña. Las
//     escrituras de view siguen en `capturas`, que es el registro del emulador.
//
// Lo que NO hace: JNI, bibliotecas `.so`, multidex, excepciones (`try`/`catch` se rechazan por
// nombre), `fill-array-data` y cualquier API de framework fuera de los stubs. Una llamada que no se
// sabe resolver no se inventa: se devuelve el cero de su tipo y se anota un aviso en el JSON, que es
// lo único que impide que una aproximación se confunda con una ejecución completa.
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
	capturasMaximas         = 64
	avisosMaximos           = 24
	profundidadMaximaDalvik = 48
	// pulsacionesMaximas acota la lista de `--pulsar`: un clic de más no puede encadenar una
	// ejecución sin fin, y el tope se avisa en vez de recortar en silencio.
	pulsacionesMaximas = 32
	// pantallasMaximas es el tope simétrico del otro bucle: una actividad que en su `onCreate`
	// lanza a otra y esa a la primera encadenaría pantallas sin fin. Al llegar al tope se avisa y
	// se deja delante la última que sí arrancó.
	pantallasMaximas = 32
	// consolaMaximas acota las líneas de consola, con el mismo criterio que las capturas.
	consolaMaximas = 128
)

// capturaTexto es una escritura que el intérprete pudo ver: qué stub la recibió, sobre qué view (si
// es una view con id de recurso) y con qué texto.
type capturaTexto struct {
	Origen string `json:"origen"`
	ID     string `json:"id,omitempty"`
	IDRef  string `json:"idRef,omitempty"`
	Texto  string `json:"texto"`
}

// lineaConsola es una línea de la consola de la app: una llamada a `print`/`println` o a `Log.*`.
// `secuencia` es el orden de emisión dentro de la ejecución (1, 2, 3…) y es lo único que se puede
// medir sin reloj: la hora de verdad no es determinista y no se finge.
type lineaConsola struct {
	Origen    string `json:"origen"`
	Texto     string `json:"texto"`
	Secuencia int    `json:"secuencia"`
}

// oyenteVista es una vista que registró un `OnClickListener`: su id de recurso y su nombre, que es
// lo que la pestaña necesita para pintar el botón de pulsar y para mandar `--pulsar id/pulsar`.
type oyenteVista struct {
	ID    string `json:"id,omitempty"`
	IDRef string `json:"idRef,omitempty"`
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
	Avisos        []string       `json:"avisos,omitempty"`
	Error         string         `json:"error,omitempty"`
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

	// malformado separa dos clases de fallo que no son lo mismo: «la app llama a algo que el MVP no
	// cubre» (se sigue con el modelo y rc 0) y «estos bytes no son un dex ejecutable», que el comando
	// rechaza. No viaja al JSON: el consumidor ve `error` y el código de salida.
	malformado bool
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
}

// arregloDalvik es un arreglo: el descriptor de su tipo y sus huecos. Los de tipos primitivos
// guardan el valor ya extendido (un `byte` como int32 con signo).
type arregloDalvik struct {
	tipo  string
	datos []any
}

// metodoDalvik es un método del dex listo para ejecutarse. `item` es nil cuando el método no tiene
// cuerpo (abstracto o nativo), y entonces solo puede atenderse como stub.
type metodoDalvik struct {
	nombre  string
	proto   string
	retorno string
	acceso  uint32
	item    *codeItem
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
	prog        *programaDalvik
	tabla       *tablaRecursos
	pasos       int
	capturas    []capturaTexto
	consola     []lineaConsola
	avisos      []string
	vistos      map[string]bool
	layoutID    uint32
	oyentes     []oyenteRegistrado
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
	// estadoGuardado es lo que la última `onSaveInstanceState` dejó en el Bundle, para publicarlo
	estadoGuardado map[string]any
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

func nuevaMaquina(p *programaDalvik, tabla *tablaRecursos) *maquinaDalvik {
	return &maquinaDalvik{prog: p, tabla: tabla, vistos: map[string]bool{},
		vistas: map[uint32]*objetoDalvik{}, almacen: nuevoAlmacenPreferencias()}
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

// vista devuelve la vista que ese id ya tiene o la crea como la dejaría el inflado: la clase genérica
// de View y el `android:text` que el layout declare para ese id. El id 0 no se registra: no es una
// vista del layout y no se puede volver a encontrar.
func (m *maquinaDalvik) vista(id uint32) *objetoDalvik {
	if id == 0 {
		return &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{}}
	}
	if v := m.vistas[id]; v != nil {
		return v
	}
	v := &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{}, id: id,
		texto: m.textosDeLayout[id]}
	if m.vistas == nil {
		m.vistas = map[uint32]*objetoDalvik{}
	}
	m.vistas[id] = v
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
		// la consola no se recorta con el tope de las capturas: son dos acotas distintas
		m.anotaConsola(origen, texto)
	}
	if len(m.capturas) >= capturasMaximas {
		return
	}
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
	if len(m.consola) >= consolaMaximas {
		return
	}
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
// resultado es solo para la lista de clics (un id que no existe o una vista sin listener): la app
// ejecutada a medias no se devuelve como si fuera una pantalla buena.
func ejecutaOnCreateConPulsos(d *dexFile, tabla *tablaRecursos, actividad string, pulsos []string) (*resultadoEjecucion, uint32, error) {
	return ejecutaOnCreatePreparando(d, tabla, actividad, pulsos, nil)
}

// ejecutaOnCreatePreparando es el mismo arranque con un gancho entre `onCreate` y los clics. Lo usa
// `apkc preview` para registrar los oyentes declarativos del layout (`android:onClick`): el
// framework los enlaza al inflar, es decir, después de que la app pida su `setContentView` y antes
// de que llegue ningún toque. Sin lista de actividades declaradas, cualquier clase del dex puede ser
// destino de `startActivity` (es el camino de las pruebas que no leen manifiesto).
func ejecutaOnCreatePreparando(d *dexFile, tabla *tablaRecursos, actividad string, pulsos []string,
	preparar func(m *maquinaDalvik)) (*resultadoEjecucion, uint32, error) {
	return ejecutaOnCreateNavegando(d, tabla, actividad, pulsos, preparar, nil)
}

// ejecutaOnCreateNavegando es el arranque con ciclo de vida y navegación. `declaradas` son las
// actividades que el manifiesto declara, por descriptor: una que no esté ahí no se lanza, como en el
// aparato.
//
// La pila de actividades vive aquí y no en el dex: cada pantalla conserva su propia máquina (sus
// objetos, sus capturas y sus oyentes) mientras está cubierta, y `finish()` la descubre otra vez sin
// repetir su `onCreate`, que es lo que hace que el estado vuelva como estaba.
func ejecutaOnCreateNavegando(d *dexFile, tabla *tablaRecursos, actividad string, pulsos []string,
	preparar func(m *maquinaDalvik), declaradas map[string]bool) (*resultadoEjecucion, uint32, error) {
	return ejecutaOnCreateConOrdenes(d, tabla, actividad, ordenesDePulsos(pulsos), preparar, declaradas)
}

// ordenesDePulsos convierte la lista clásica de `--pulsar` en órdenes, para las envolturas y las
// pruebas que no necesitan recrear.
func ordenesDePulsos(pulsos []string) []ordenEjecucion {
	ordenes := make([]ordenEjecucion, 0, len(pulsos))
	for _, p := range pulsos {
		ordenes = append(ordenes, ordenEjecucion{Tipo: ordenPulsarTipo, Valor: p})
	}
	return ordenes
}

// ejecutaOnCreateConOrdenes es el arranque con ciclo de vida, navegación y la secuencia de órdenes
// (`--pulsar` y `--recrear`) entregada en el orden en que se escribió.
func ejecutaOnCreateConOrdenes(d *dexFile, tabla *tablaRecursos, actividad string, ordenes []ordenEjecucion,
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
			res.Ciclo = m.ciclo
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

	// las órdenes pedidas, en orden y solo si la app llegó viva a este punto. El estado que ve cada
	// `onClick` incluye los anteriores, porque comparten la misma máquina: es lo que hace que
	// `--pulsar a --pulsar a` deje el contador en 2 y no en 1. Entre orden y orden se estabiliza la
	// navegación: la siguiente va a la pantalla que quedó delante.
	var errPulsos error
	if res.Error == "" {
		entregados := 0
		for _, orden := range ordenes {
			if entregados >= pulsacionesMaximas {
				m.avisa("se dejaron órdenes sin entregar por el tope de %d", pulsacionesMaximas)
				break
			}
			if orden.Tipo == ordenRecrearTipo {
				if err := recreaActual(); err != nil {
					errPulsos = err
					break
				}
				entregados++
				estabiliza()
				continue
			}
			id, err := idDePulsacion(tabla, orden.Valor)
			if err != nil {
				errPulsos = err
				break
			}
			if !m.pulsa(id) {
				errPulsos = fmt.Errorf("no se puede pulsar %q: ninguna vista con ese id se registró "+
					"como OnClickListener%s", orden.Valor, listaDeOyentes(m))
				break
			}
			res.Pulsaciones = append(res.Pulsaciones, orden.Valor)
			entregados++
			estabiliza()
		}
	}

	// el resultado es el de la pantalla actual, con la suma de lo que hicieron las que se visitaron
	res.Actividad = claseCorta(m.actividad.clase)
	res.Capturas = m.capturas
	res.Oyentes = m.oyentesDeVista()
	for _, x := range todas {
		res.Pasos += x.pasos
		res.Interacciones += x.interaccion
		res.Consola = append(res.Consola, x.consola...)
		for _, a := range x.avisos {
			if !contieneTexto(res.Avisos, a) {
				res.Avisos = append(res.Avisos, a)
			}
		}
	}
	res.Ciclo = ciclo
	// el estado que sobrevive a la ejecución se publica tal cual: el Bundle de la última recreación
	// y el SharedPreferences de toda la ejecución, sin inventar lo que la app no escribió
	if m.estadoGuardado != nil {
		res.Bundle = m.estadoGuardado
	}
	res.Preferencias = m.almacen.volcado()
	// la consola se renumera al unir las pantallas: `secuencia` es el orden real de emisión
	for i := range res.Consola {
		res.Consola[i].Secuencia = i + 1
	}
	return res, m.layoutID, errPulsos
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

// idDePulsacion lleva lo que se escribe en `--pulsar` al id de recurso de la vista. Se aceptan las
// tres formas que el modelo publica: `id/pulsar` (nombreDeId al revés), `0x7f010000` (el `idRef` de
// los nodos) y el decimal del mismo número.
func idDePulsacion(tabla *tablaRecursos, pedido string) (uint32, error) {
	s := strings.TrimSpace(pedido)
	if s == "" {
		return 0, fmt.Errorf("--pulsar necesita un id: id/pulsar, 0x7f010000 o su decimal")
	}
	if tipo, clave, ok := strings.Cut(s, "/"); ok {
		if tabla == nil {
			return 0, fmt.Errorf("no se puede pulsar %q: sin resources.arsc no hay tabla que traduzca "+
				"el nombre; pasa el id numérico", pedido)
		}
		id, ok := tabla.idDeNombre(tipo, clave)
		if !ok {
			return 0, fmt.Errorf("no se puede pulsar %q: la tabla no tiene %s/%s", pedido, tipo, clave)
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
	return 0, fmt.Errorf("no se puede pulsar %q: se espera id/pulsar, 0x7f010000 o un decimal", pedido)
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

// ejecutaCuerpo interpreta el code_item de un método con los argumentos dados. `tipos` dice el tipo
// de cada argumento, que es lo que decide cuántas palabras ocupa en los registros.
func (m *maquinaDalvik) ejecutaCuerpo(clase string, met *metodoDalvik, args []any, tipos []string) (res any, err error) {
	if met == nil {
		return nil, fmt.Errorf("%s: método sin cuerpo que ejecutar", claseCorta(clase))
	}
	if met.item == nil {
		return m.stubSinCuerpo(clase, met, args)
	}
	it := met.item
	if it.triesSize > 0 {
		return nil, fmt.Errorf("%s.%s lleva tabla de excepciones y el intérprete MVP no la ejecuta",
			claseCorta(clase), met.nombre)
	}
	if m.profundidad >= profundidadMaximaDalvik {
		return nil, fmt.Errorf("llamadas anidadas por encima de %d en %s.%s",
			profundidadMaximaDalvik, claseCorta(clase), met.nombre)
	}
	m.profundidad++
	defer func() { m.profundidad-- }()
	// El decodificador comprueba cada acceso a las palabras del code_item y rechaza por nombre lo que
	// no cabe (payloads de switch incluidos). Este recover es la última red: si algo se escapa, se
	// dice en qué unidad y con qué opcode, y se marca como dex malformado para que el comando lo
	// rechace; no se disfraza de error de negocio ni se atribuye sin pruebas a los registros.
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

	for {
		if pc < 0 || pc >= len(ins) {
			return nil, fmt.Errorf("%s.%s se salió del código en la unidad %d",
				claseCorta(clase), met.nombre, pc)
		}
		m.pasos++
		if m.pasos > pasosMaximos {
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
		case 0x0d: // move-exception
			return nil, fmt.Errorf("%s.%s usa move-exception y el MVP no ejecuta excepciones",
				claseCorta(clase), met.nombre)

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
			if int(w) >= len(m.prog.dex.strings) {
				return nil, fmt.Errorf("const-string con la cadena %d fuera del pool", w)
			}
			poner(aa, m.prog.dex.strings[w])
			pc += 2
		case 0x1b: // const-string/jumbo
			if pc+2 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un const-string/jumbo")
			}
			idx := uint32(ins[pc+1]) | uint32(ins[pc+2])<<16
			if int(idx) >= len(m.prog.dex.strings) {
				return nil, fmt.Errorf("const-string/jumbo con la cadena %d fuera del pool", idx)
			}
			poner(aa, m.prog.dex.strings[idx])
			pc += 3
		case 0x1c: // const-class
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if int(w) >= len(m.prog.dex.types) {
				return nil, fmt.Errorf("const-class con el tipo %d fuera de type_ids", w)
			}
			poner(aa, &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
				"Ljava/lang/Class;.$tipo": m.prog.dex.types[w],
			}})
			pc += 2

		// --- objetos -----------------------------------------------------------------------
		case 0x1f: // check-cast
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if obj, ok := regs[aa].(*objetoDalvik); ok && int(w) < len(m.prog.dex.types) {
				obj.clase = m.prog.dex.types[w]
			}
			pc += 2
		case 0x20: // instance-of (22c)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			es := false
			if obj, ok := regs[int(u>>12)].(*objetoDalvik); ok && int(w) < len(m.prog.dex.types) {
				es = m.esDe(obj, m.prog.dex.types[w])
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
			if int(w) >= len(m.prog.dex.types) {
				return nil, fmt.Errorf("new-instance con el tipo %d fuera de type_ids", w)
			}
			poner(aa, &objetoDalvik{clase: m.prog.dex.types[w], campos: map[string]any{}})
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
			if int(w) < len(m.prog.dex.types) {
				tipo = m.prog.dex.types[w]
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
			if int(w) < len(m.prog.dex.types) {
				tipo = m.prog.dex.types[w]
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
			if int(w) < len(m.prog.dex.types) {
				tipo = m.prog.dex.types[w]
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
			return nil, fmt.Errorf("%s.%s lanza una excepción y el MVP no las ejecuta",
				claseCorta(clase), met.nombre)

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
			v, err := m.leeCampo(uint32(w), regs[int(u>>12)], false)
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
			if err := m.escribeCampo(uint32(w), regs[int(u>>12)], regs[int(u>>8)&0xf], false); err != nil {
				return nil, err
			}
			pc += 2
		case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66: // sget*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			v, err := m.leeCampo(uint32(w), nil, true)
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
			if err := m.escribeCampo(uint32(w), nil, regs[aa], true); err != nil {
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
			v, err := m.invoca(op, uint32(w), registros35c(u, t), regs, clase)
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
			v, err := m.invoca(op-6, uint32(w), regsArg, regs, clase)
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

func (m *maquinaDalvik) leeCampo(idx uint32, receptor any, estatico bool) (any, error) {
	d := m.prog.dex
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

func (m *maquinaDalvik) escribeCampo(idx uint32, receptor any, v any, estatico bool) error {
	d := m.prog.dex
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
func (m *maquinaDalvik) invoca(op byte, idx uint32, palabras []int, regs []any, claseActual string) (any, error) {
	d := m.prog.dex
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
	// `findViewById` invocado sobre MainActivity se declara en Activity: se prueban las dos.
	candidatos := []string{ref.owner}
	if obj, ok := receptor.(*objetoDalvik); ok {
		candidatos = append([]string{obj.clase}, candidatos...)
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

// intentaStub resuelve las llamadas de framework que la app toca. `candidatos` son las clases por
// las que se busca (la del objeto, la que declara el método); los sufijos bastan porque lo que
// distingue un `setText` es el nombre y su firma, no la jerarquía entera del framework.
func (m *maquinaDalvik) intentaStub(candidatos []string, nombre, proto string, vals []any, receptor any) (any, bool, error) {
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
			"Landroid/widget/ImageView;", "Landroid/widget/LinearLayout;",
			"Landroid/widget/FrameLayout;", "Landroid/widget/RelativeLayout;")
	}
	obj, _ := receptor.(*objetoDalvik)

	switch nombre {
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
		if obj != nil && proto == "(Ljava/lang/String;)V" && es("Ljava/lang/StringBuilder;") {
			obj.cadena = &strings.Builder{}
			obj.cadena.WriteString(textoDe(vals[0]))
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
			return m.vista(uint32(id)), true, nil
		}
	case "setOnClickListener":
		if len(vals) == 1 {
			if o := obj; o != nil {
				o.oyente = vals[0]
				m.oyentes = append(m.oyentes, oyenteRegistrado{oyente: vals[0], vista: o})
			}
			return nil, true, nil
		}
	case "setText":
		if len(vals) == 1 && es("Landroid/widget/TextView;", "Landroid/widget/Button;",
			"Landroid/widget/EditText;", "Landroid/widget/CheckBox;", "Landroid/widget/Switch;") {
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
			obj.cadena.WriteString(textoDe(vals[0]))
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
		if obj != nil && esVista() {
			return m.rechaza(nombre, proto,
				"Editable y Spannable no están en el subconjunto: solo se emula texto plano",
				"Landroid/text/Editable;")
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
	case "setTitle":
		if len(vals) == 1 && es("Landroid/app/Activity;") {
			m.captura("Activity.setTitle", obj, textoDe(vals[0]))
			return nil, true, nil
		}
	case "toString":
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
				return nil, true, fmt.Errorf("parseInt de %q: %w", textoDe(vals[0]), err)
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
			return 0, fmt.Errorf("división entera por cero")
		}
		return a / b, nil
	case 4:
		if b == 0 {
			return 0, fmt.Errorf("resto entero por cero")
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
			return 0, fmt.Errorf("división de long por cero")
		}
		return a / b, nil
	case 4:
		if b == 0 {
			return 0, fmt.Errorf("resto de long por cero")
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
			return 0, fmt.Errorf("división entera por cero")
		}
		return a / lit, nil
	case 4:
		if lit == 0 {
			return 0, fmt.Errorf("resto entero por cero")
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
