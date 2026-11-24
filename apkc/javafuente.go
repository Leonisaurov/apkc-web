package main

// La Fase 1G, primera pieza: leer el Java que escribe la pagina y dejarlo en un arbol. El objetivo medido
// es la plantilla que genera `apkc create` (apkc/template/MainActivity.java), cuyo oráculo es javap sobre
// el .class que deja el javac nativo del telefono (docs/evidencia.md 40): dos metodos, `ldc` de la
// constante inlinada de `R.layout.main`, `invokespecial` al super y `invokevirtual` al metodo heredado.
//
// La regla de este fichero es la del repo: lo que no se ha medido NO se acepta en silencio. Cada forma de
// Java que el parser no sabe emitir —y por lo tanto el emisor no puede comprobar contra javap— se rechaza
// con el nombre de lo que aparece, igual que hace el dexer con los opcodes que no verifico. La lista crece
// con su medicion: un constructo entra cuando entra su prueba contra `javap -c -p`, no antes.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// pos es una posicion en el fuente, para los mensajes y para la LineNumberTable que escribe el emisor.
type pos struct {
	linea int
	// columna es la del token que abrio la expresion, contada en caracteres y desde 1. Vale cero
	// cuando la posicion es la de una sentencia, que solo lleva la linea: el mensaje lo dice asi en
	// vez de inventarse una columna.
	columna int
}

// sitio escribe la posicion para un mensaje: `linea 15, columna 17` si se sabe la columna (viene de un
// token) y `linea 15` si solo se sabe la linea. Es lo que `apkc check` convierte despues en
// `archivo:linea:columna`, para que el editor subraye el token y no la linea entera.
func (p pos) sitio() string {
	if p.columna > 0 {
		return fmt.Sprintf("linea %d, columna %d", p.linea, p.columna)
	}
	return fmt.Sprintf("linea %d", p.linea)
}

// jexpr es una expresion del subconjunto.
type jexpr interface{ jexprPuede() }

// jlit es un literal: cadena, entero, booleano o nulo.
type jlit struct {
	tipoJ    byte // 's' cadena, 'i' entero, 'j' entero largo, 'f' flotante, 'd' doble, 'b' booleano, 'n' nulo
	texto    string
	entero   int32
	largo    int64
	flotante float64
	booleano bool
	donde    pos
}

func (jlit) jexprPuede() {}

// jclase: el literal `X.class` del párrafo 89. `nombre` es el nombre simple del fuente (o el
// primitivo) y `dimensiones` cuenta los `[]`; el emisor resuelve el nombre interno contra el fuente y
// el jar. Un primitivo sin dimensiones no es `ldc`: javac escribe el `getstatic Wrapper.TYPE`.
type jclase struct {
	nombre      string
	dimensiones int
	primitivo   bool
	donde       pos
}

func (jclase) jexprPuede() {}

// jthis: `this`, y `Tipo.this` cuando nombre no esta vacio (captura del exterior en una interna).
type jthis struct {
	nombre string
	donde  pos
}

func (jthis) jexprPuede() {}

// jacceso: un acceso a campo, con la base vacia si es un nombre suelto (`R`, `menu`, `MainActivity`).
type jacceso struct {
	base   jexpr
	nombre string
	donde  pos
}

func (jacceso) jexprPuede() {}

// jllamada: una invocacion. `super.` va en receptorNil con super puesto a verdadero; con base vacia el
// receptor es `this`, que es lo que javac anota como clase actual (parrafo 40: setContentView sale
// Methodref contra MainActivity, no contra Activity, aunque se declare en Activity).
type jllamada struct {
	base  jexpr
	super bool
	// constructorPropio: `this(...)`, la delegacion a otro constructor de la misma clase (parrafo 91).
	// `super(...)` usa `super` con nombre `<init>`.
	constructorPropio bool
	nombre            string
	args              []jexpr
	donde             pos
}

func (jllamada) jexprPuede() {}

// jcast: un molde entre parentesis. Solo se acepta sobre un tipo que el emisor conoce.
type jcast struct {
	tipo   string
	dentro jexpr
	donde  pos
}

// jinstanceof: `x instanceof T`. Medido en el parrafo 79 sobre la sonda `Ins`: javac escribe
// `aload_0; instanceof #7 // class java/lang/String; 4: ifeq 9` —deja un 0/1 en la pila y la condicion
// del `if` es el `ifeq` **sin invertir** (al revés que las comparaciones, que salen invertidas:
// parrafo 52)—.
type jinstanceof struct {
	tipo   string
	dentro jexpr
	donde  pos
}

func (jinstanceof) jexprPuede() {}

func (jcast) jexprPuede() {}

// jnuevo: `new Tipo(args)` y, si cuerpo no es nil, la clase anonima `new Tipo(args) { ... }`.
// jlambda es una lambda de un parametro sin tipo y cuerpo de expresion (`apk -> shareApp(apk)`). El
// tipo destino no viaja en la expresion: lo pone el parametro formal de la llamada que la recibe, que
// es quien declara la interface funcional (JLS 15.27.3). Un cuerpo de bloque (`x -> { … }`) queda
// fuera de este subconjunto y se rechaza con su nombre al analizarlo.
type jlambda struct {
	param  string
	cuerpo jexpr
	donde  pos
}

func (jlambda) jexprPuede() {}

type jnuevo struct {
	tipo   string
	args   []jexpr
	cuerpo *jclas
	donde  pos
}

func (jnuevo) jexprPuede() {}

// jnuevoArreglo: `new Tipo[n]` o `new Tipo[] {…}`. Con inicializador, javac escribe el largo, el
// `newarray`/`anewarray` y una cadena de `dup` + indice + valor + `iastore`/`aastore` por elemento
// (medido en el parrafo 73), asi que no hace falta `filled-new-array`.
type jnuevoArreglo struct {
	tipo        string
	largo       jexpr
	dimensiones []jexpr // más de una: `new int[2][3]`, bajado por d8 a Array.newInstance
	elementos   []jexpr
	donde       pos
}

func (jnuevoArreglo) jexprPuede() {}

// jindice: `v[i]`, para leer y para asignar (el destino de un `jasign` puede ser uno).
type jindice struct {
	base   jexpr
	indice jexpr
	donde  pos
}

func (jindice) jexprPuede() {}

// jincr: `i++`, `--i`, `i += 1` y el paso de un `for`. Como sentencia se emite `iinc` en un local
// —medido en el párrafo 67— o la lectura-cuenta-escritura de un campo. Como valor, `prefijo` distingue
// las dos formas medidas en el párrafo 151: `++i` cuenta antes de dejar el valor, `i++` deja el valor
// anterior y cuenta después.
type jincr struct {
	destino jexpr
	delta   int32
	prefijo bool
	donde   pos
}

func (jincr) jexprPuede() {}

// jstmt es una sentencia del subconjunto.
type jstmt interface{ jstmtPuede() }

type jstmtExpr struct{ e jexpr }

func (jstmtExpr) jstmtPuede() {}

type jretorno struct {
	e     jexpr
	donde pos
}

func (jretorno) jstmtPuede() {}

// jdecl: declaracion de variable local, con inicializador opcional.
type jdecl struct {
	tipo   string
	nombre string
	valor  jexpr
	donde  pos
	final  bool
}

func (jdecl) jstmtPuede() {}

// jasign: asignacion a una variable local o a un campo (`t = ...`, `this.x = ...`).
type jasign struct {
	destino jexpr
	valor   jexpr
	donde   pos
}

func (jasign) jstmtPuede() {}

// jasignExp: la asignacion simple (`=`) **como expresion** (JLS 15.26.1), la forma que pide una
// condicion como `while ((length = in.read(buffer)) > 0)` de la linea 187 de `MainActivity.java` del
// zip de ApkShare. El arbol es el de javac —destino y valor, y la expresion vale lo asignado— y el
// emisor escribe el valor, lo duplica y lo guarda (`dup`/`dup2` antes del `istore`/`lstore`). El
// `jasign` de siempre es solo sentencia: no deja nada en la pila.
type jasignExp struct {
	destino jexpr
	valor   jexpr
	donde   pos
}

func (jasignExp) jexprPuede() {}

type jbloque struct {
	sentencias []jstmt
	donde      pos
}

func (jbloque) jstmtPuede() {}

type jif struct {
	cond     jexpr
	entonces jstmt
	siNo     jstmt
	donde    pos
}

func (jif) jstmtPuede() {}

// jmientras: `while (cond) cuerpo`. `for` no tiene nodo propio: se desdobla en el analizador como
// javac lo escribe —`init; while (cond) { cuerpo; paso }`— medido en el parrafo 63.
type jmientras struct {
	cond   jexpr
	cuerpo jstmt
	donde  pos
}

func (jmientras) jstmtPuede() {}

// jhacer: `do { cuerpo } while (cond);`. La forma de javac es el cuerpo primero y después la
// condición **directa** saltando hacia atrás a su cabecera (medido en el parrafo 69: `if_icmplt 2`),
// al revés que el `while`, que invierte la condición para salir.
type jhacer struct {
	cuerpo jstmt
	cond   jexpr
	donde  pos
}

func (jhacer) jstmtPuede() {}

// jrompe y jcontinua: `break;` y `continue;`. El destino lo pone el emisor con la pila de contextos
// (el fin del bucle o del switch; la cabecera, la condición o el paso del bucle), que es lo que
// escribe javac: medido en el parrafo 70, `continue` en un `for` salta al paso y en un `do-while` a
// la condición.
type jrompe struct{ donde pos }

func (jrompe) jstmtPuede() {}

type jcontinua struct{ donde pos }

func (jcontinua) jstmtPuede() {}

// jpara: `for (init; cond; paso) cuerpo`. El `init` queda fuera (el analizador lo devuelve en un
// `jbloque`); el `paso` vive aquí y no dentro del cuerpo para que `continue` salte a él, que es lo
// que hace javac (medido en el parrafo 70: el `continue` es un `goto` al `iinc` del paso).
type jpara struct {
	cuerpo jstmt
	cond   jexpr
	paso   jstmt
	donde  pos
}

func (jpara) jstmtPuede() {}

// jporCada: `for (T v : coleccion) cuerpo`, el `for` con dos puntos. Se guarda tal cual y lo baja el
// emisor en las dos formas que escribe javac, medidas en el §225: sobre un arreglo, copiando el
// arreglo y su largo a locales sintéticos y contando con un índice; sobre un `Iterable`, con
// `iterator()`/`hasNext()`/`next()` y el `checkcast` del elemento.
type jporCada struct {
	tipo      string
	nombre    string
	coleccion jexpr
	cuerpo    jstmt
	donde     pos
	// lineaCierre es la del `}` del cuerpo (o la del `;` de una sentencia suelta): javac anota con
	// ella el `goto` de vuelta del camino del iterador, y no con la del `for` (medido en el §225)
	lineaCierre int
}

func (jporCada) jstmtPuede() {}

// capturaTry: un `catch (T e) { … }` de un `try`.
type capturaTry struct {
	tipos  []string
	excep  string
	atrapa jstmt
	// lineaAtrapa es el renglon del `catch`: javac lo anota en el pc del handler (medido en el
	// parrafo 71) y no anota el del `try`
	lineaAtrapa int
}

// jtry: `try { … }` con cero o mas `catch` y un `finally` opcional. La forma de javac, medida en los
// parrafos 71 y 75: el cuerpo protegido y despues un bloque por camino —cada `catch` con su `astore`
// y su cuerpo—; con `finally` la copia del bloque va al pie de cada camino, el normal tambien, y el
// ultimo manejador es el `any`, que guarda la excepcion, copia el bloque y la re-lanza.
type jtry struct {
	cuerpo  jstmt
	atrapas []capturaTry
	// finalmente es el bloque que se copia en cada camino, o nil si el fuente no lo escribe
	finalmente jstmt
	// recursos son los del `try (…)`, en el orden del fuente (vacio en un try sin recursos). El
	// emisor escribe la forma anidada de javac con `Throwable.addSuppressed`, un nivel por recurso
	recursos    []jdecl
	lineaAviso  int
	lineaCierre int
	donde       pos
}

func (jtry) jstmtPuede() {}

// jlanza: `throw expr;`.
type jlanza struct {
	e     jexpr
	donde pos
}

func (jlanza) jstmtPuede() {}

// jswitch: `switch (selector) { case k: cuerpo break; … default: cuerpo }`. El selector tiene que ser
// un entero y las claves literales enteros; cada caso **tiene que terminar en `break` o en `return`**
// (la caída de un caso al siguiente no está medida y se rechaza por nombre). Se midió en el §64 qué
// forma elige javac; el emisor usa la suya, escrita en `cambio`.
type jswitch struct {
	selector jexpr
	casos    []jcaso
	defecto  []jstmt
	esCadena bool // `switch (String)`: se baja a hashCode + equals + switch del índice
	// orden es el orden del fuente de los brazos: un indice de `casos`, o -1 para el `default`.
	// javac emite los cuerpos en ese orden, y la caída al siguiente brazo depende de él.
	orden []int
	donde pos
}

func (jswitch) jstmtPuede() {}

type jcaso struct {
	valor  int32
	cadena string // clave de un switch de String
	cuerpo []jstmt
	rompe  bool // terminó en `break;`, así que el emisor escribe el salto al final del switch
	donde  pos
}

// jparam, jcampo y jmetodo son las partes declarativas.
type jparam struct {
	tipo   string
	nombre string
	// generico es el parámetro declarado con argumentos de tipo (`List<Apk>`): su firma va al atributo
	// Signature del método y el descriptor sigue siendo el del borrado
	generico *tipoArgumento
	donde    pos
}

// anotValorJ es un valor de elemento de una anotación. El subconjunto cubre los «elementos simples»
// (JVMS 4.7.16.1): literales enteros, `char`, booleanos y cadenas, literales de clase (`c`),
// constantes de enum (`e`) y arreglos de cualquiera de ellos (`a`). `texto` guarda la cadena o el
// nombre del tipo, `nombre` el de la constante de enum y `lista` los hijos de un arreglo.
type anotValorJ struct {
	kind   byte
	texto  string
	nombre string
	entero int64
	lista  []anotValorJ
}

type anotElemJ struct {
	nombre string
	valor  anotValorJ
}

// anotacionJ es `@Nombre(elemento = valor, …)` tal como lo escribe el fuente. El nombre se resuelve en
// el emisor contra los imports y el paquete, igual que cualquier otro tipo.
type anotacionJ struct {
	nombre    string
	elementos []anotElemJ
	donde     pos
}

type jcampo struct {
	nombre string
	tipo   string
	// generico es el tipo declarado con sus argumentos, cuando los trae (`List<Apk>`): el descriptor
	// sale de tipo (el borrado) y la `Signature` genérica del `.class` de aquí.
	generico    *tipoArgumento
	esEstatico  bool
	esFinal     bool
	esPublico   bool
	esPrivado   bool
	esProtegido bool
	esSintetico bool // val$x de una captura de anonima
	valor       jexpr
	anotaciones []anotacionJ
	donde       pos
}

type jmetodo struct {
	nombre  string
	retorno string // "" si constructor
	// retornoGenerico es el tipo de vuelta con sus argumentos (`List<Apk> f()`); va a la Signature
	// genérica del method_info y no cambia el descriptor, que sigue siendo el del borrado.
	retornoGenerico *tipoArgumento
	params          []jparam
	cuerpo          []jstmt
	esEstatico      bool
	esPublico       bool
	esPrivado       bool
	esProtegido     bool
	esFinal         bool
	// locales: parametros y locales declarados, para resolver una captura de una anonima (5.14).
	locales map[string]string
	// abstracto: el metodo no tiene cuerpo; en una interface es implicito y en una clase viene del
	// modificador `abstract` (parrafo 92).
	abstracto bool
	// deprecado: @Deprecated deja un atributo Deprecated y un RuntimeVisibleAnnotations (§170).
	deprecado bool
	// lanza: las clases del atributo Exceptions de un `throws A, B` (parrafo 92).
	lanza []string
	// varargs: el ultimo parametro venia con `...` (`int... xs`). El tipo guardado ya es el arreglo
	// (`int[]`), que es lo que javac describe en el method_info, y la firma lleva ACC_VARARGS (§194).
	varargs     bool
	anotaciones []anotacionJ
	linea       int
	// lineaFin es la llave que cierra el cuerpo: javac anota ahi el `return` implicito de un metodo void,
	// medido en el parrafo 40 (line 16 en la plantilla, que es donde esta el `}` de onCreate).
	lineaFin int
}

// jclas es una clase, anidada o no. interior guarda las anidadas en el orden del fuente, y nombreExterno
// el prefijo con el que javac las nombra en el binario (R y R$layout).
type jclas struct {
	nombre string
	super  string
	// superArgs e implementaArgs son los argumentos de tipo escritos en la cláusula `extends` o
	// `implements` (`ArrayAdapter<Apk>` deja el crudo en super y el `Apk` aquí). El descriptor de la
	// superclase no cambia con ellos —el borrado los tira—, pero javac los escribe en la `Signature`
	// genérica del `.class`, y el dex los lleva como la anotación `dalvik.annotation.Signature`.
	superArgs      []tipoArgumento
	implementa     []string
	implementaArgs [][]tipoArgumento
	campos         []jcampo
	metodos        []jmetodo
	interior       []*jclas
	abstracta      bool
	esInterface    bool
	final          bool
	esPublica      bool
	esPrivada      bool
	esEstatica     bool // solo cambia los flags del InnerClasses: en su propio ClassFile una anidada no es ACC_STATIC
	// esEnum y constantes: un `enum Color { RED, GREEN }` simple. El emisor escribe el andamiaje que
	// javac genera para el tipo (java/lang/Enum, $VALUES, values, valueOf y <clinit>).
	esEnum        bool
	constantes    []string
	linea         int
	paquete       string
	imports       []string // los del fuente donde se declaró la clase: su alcance, no el del llamador
	// unidad es el índice del fichero del que salió la clase. JLS 6.5.5.1 resuelve un nombre simple
	// primero contra el mismo compilation unit y solo después contra los imports: sin este dato, una
	// clase de otro fichero del proyecto se colaba por delante del import explícito. El nombre base no
	// sirve porque dos `Helper.java` de paquetes distintos se llaman igual.
	unidad        int
	nombreExterno string

	// anonima: la clase que crea un `new Tipo(args) { ... }`. javac le pone el campo this$0, un constructor
	// con la captura y el atributo EnclosingMethod, y en el InnerClasses su fila va sin nombre ni exterior
	// (parrafo 42). encierre y metodoEncierre son de donde salio, y se guardan punteros porque el descriptor
	// del exterior se calcula despues, en el emisor.
	anonima bool
	// siguienteAnonima cuenta las anonimas creadas dentro de esta clase: javac las nombra
	// Exterior$1$1, Exterior$1$2… cuando la clase que las crea ya es anonima (medido con javac 17)
	siguienteAnonima int
	encierre         *jclas
	metodoEncierre   *jmetodo
	// capturas: las locales/parámetros effectively-final que la anonima usa y que javac baja a
	// campos `val$x` y parámetros del constructor (5.14).
	capturas []captura

	anotaciones []anotacionJ
}

type captura struct {
	nombre string
	tipo   string
}

// tipoArgumento es un tipo dentro de unos argumentos de tipo, y también el tipo declarado de un campo:
// el nombre tal como lo escribe el fuente, los corchetes del arreglo y, si a su vez es genérico, sus
// propios argumentos. Se guarda entero porque la `Signature` genérica de javac los escribe anidados
// (`List<List<String>>`) y con los corchetes delante (`List<String>[]` es `[Ljava/util/List<…>;`),
// aunque el descriptor siga siendo el del borrado. Un comodín (`?`, `? extends X`, `? super X`) llega
// con `comodin` puesto y su cota —si la trae— en `cota` (§241).
type tipoArgumento struct {
	nombre   string
	arreglos int
	args     []tipoArgumento
	// comodin es el signo con el que la `Signature` escribe el comodín: `*` para `?`, `+` para
	// `? extends T` y `-` para `? super T`. Cero quiere decir que el tipo no es un comodín.
	comodin byte
	// cota es el tipo de `? extends T` o `? super T`; nil en el `?` a secas.
	cota *tipoArgumento
	// esFinal marca el `final` que precede a una declaración local (`final int n = 6`,
	// `final List<Apk> apks = new ArrayList<>()`). No viaja en el descriptor ni en la `Signature`:
	// javac no lo escribe en el `.class` con `-g:lines,source`, así que `crudo()` lo ignora y solo
	// lo usa la sentencia para su `jdecl`.
	esFinal bool
}

// crudo es el nombre tal como lo lee el descriptor: los corchetes pegados y sin argumentos, que el
// borrado tira (`List<Apk>[]` -> `List[]`, que es `[Ljava/util/List;`). Un comodín borra a
// `java.lang.Object`, salvo `? extends T`, que borra a su cota (JLS 4.6); solo se llega aquí con uno
// si se usara como tipo declarado, que Java no admite, pero el descriptor no depende de eso.
func (t tipoArgumento) crudo() string {
	if t.comodin != 0 {
		if t.comodin == '+' && t.cota != nil {
			return t.cota.crudo()
		}
		return "java.lang.Object"
	}
	return t.nombre + strings.Repeat("[]", t.arreglos)
}

// archivoJava es un fuente entero: el paquete, los imports y la clase principal con sus anidadas.
type archivoJava struct {
	paquete string
	imports []string
	clases  []*jclas
}

// ----- el léxico -----

// tokenJava es una unidad léxica. valor trae el texto de identificadores, literales y operadores.
type tokenJava struct {
	kind  byte // 'i' identificador, 'n' numero, 's' cadena, 'k' palabra clave, 'p' puntuacion, 'f' fin
	valor string
	donde pos
}

// palabrasJava: las que este parser reconoce. Lo demas es identificador, y si aparece donde va una
// declaracion el parser lo nombra y rechaza.
var palabrasJava = map[string]bool{
	"package": true, "import": true, "public": true, "private": true, "protected": true, "static": true,
	"final": true, "class": true, "interface": true, "extends": true, "implements": true, "abstract": true,
	"void": true, "return": true, "if": true, "else": true, "new": true, "this": true, "super": true,
	"true": true, "false": true, "null": true, "for": true, "while": true, "try": true, "catch": true,
	"boolean": true, "int": true, "long": true, "float": true, "double": true, "byte": true, "short": true,
	"char": true, "enum": true, "switch": true, "case": true, "default": true, "break": true, "continue": true, "throw": true, "native": true,
	"synchronized": true, "volatile": true, "transient": true, "instanceof": true, "do": true,
	"finally": true, "throws": true,
}

// simbolesJava, de mas largo a mas corto para que `>>` no se lea como dos `>`.
var simbolesJava = []string{
	"...", ">>>=", ">>=", "<<=", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=",
	// `->` va antes que `-`: con el guion suelto delante, la flecha de una lambda se leia como un
	// menos seguido de un mayor y el cuerpo de la lambda se perdia (medido en el frente de ApkShare)
	"&=", "|=", "^=", ">>>", ">>", "<<", "++", "--", "->",
	"{", "}", "(", ")", "[", "]", ";", ",", ".", "=", ">", "<", "!", "+", "-", "*", "/", "&", "|", "^", ":", "@", "%", "?",
}

// tokenizarJava parte el fuente. Hay una garantia en el final del bucle: si una vuelta no consumio
// bytes, se rechaza. Sin esa garantia un byte multibyte que no es letra ASCII (un `¥`, que es un
// simbolo monetario y no una letra) hacia append de un identificador vacio sin avanzar y el bucle
// exterior no paraba nunca: memoria hasta que el kernel mataba el proceso.
func tokenizarJava(fuente string) ([]tokenJava, error) {
	out := []tokenJava{}
	linea := 1
	// inicio es el byte donde empieza la linea actual: la columna de un token es cuantos caracteres
	// hay entre inicio y su primer byte, mas uno. Se cuenta en caracteres y no en bytes para que el
	// subrayado del editor (que mide en caracteres) caiga sobre el token aunque la linea lleve tildes.
	inicio := 0
	columnaDe := func(desde int) int { return utf8.RuneCountInString(fuente[inicio:desde]) + 1 }
	i := 0
	for i < len(fuente) {
		antes := i
		c := fuente[i]
		switch {
		case c == '\n':
			linea++
			i++
			inicio = i
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(fuente) && fuente[i+1] == '/':
			for i < len(fuente) && fuente[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(fuente) && fuente[i+1] == '*':
			i += 2
			for i+1 < len(fuente) && !(fuente[i] == '*' && fuente[i+1] == '/') {
				if fuente[i] == '\n' {
					linea++
					inicio = i + 1
				}
				i++
			}
			if i+1 >= len(fuente) {
				return nil, fmt.Errorf("linea %d: un comentario de bloque sin cerrar", linea)
			}
			i += 2
		case c == '"':
			texto, nuevo, err := leerCadenaJava(fuente, i, linea)
			if err != nil {
				return nil, err
			}
			out = append(out, tokenJava{kind: 's', valor: texto, donde: pos{linea: linea, columna: columnaDe(antes)}})
			i = nuevo
		case c == '\'':
			// el literal `char` del párrafo 87: un caracter entre comillas simples, con sus escapes. El
			// valor que sale es el **punto de código**, que es lo que javac escribe con `iconst`/`bipush`/
			// `sipush`/`ldc` medido en `Letras` (`'\n'` es `bipush 10`, `'á'` es `sipush 225` y `'\uFFFF'`
			// es `ldc` con la entrada `Integer` 65535).
			valor, nuevo, err := leerCharJava(fuente, i, linea)
			if err != nil {
				return nil, err
			}
			out = append(out, tokenJava{kind: 'c', valor: itoa(int(valor)), donde: pos{linea: linea, columna: columnaDe(antes)}})
			i = nuevo
		case esLetraJava(c) || c == '_' || c == '$':
			// ASCII solo: Java permite letras de otros alfabetos en un identificador, pero la
			// plantilla de la pagina y los fuentes que genera son ASCII, y leer un byte suelto de una
			// secuencia multibyte como si fuera un caracter es lo que hacia girar a este bucle.
			desde := i
			for i < len(fuente) && (esLetraJava(fuente[i]) || esDigito(fuente[i]) || fuente[i] == '_' || fuente[i] == '$') {
				i++
			}
			palabra := fuente[desde:i]
			kind := byte('i')
			if palabrasJava[palabra] {
				kind = 'k'
			}
			out = append(out, tokenJava{kind: kind, valor: palabra, donde: pos{linea: linea, columna: columnaDe(desde)}})
		case esDigito(c):
			desde := i
			for i < len(fuente) && (fuente[i] >= '0' && fuente[i] <= '9' || fuente[i] == 'x' || fuente[i] == 'X' ||
				fuente[i] == 'a' || fuente[i] == 'b' || fuente[i] == 'c' || fuente[i] == 'd' || fuente[i] == 'e' || fuente[i] == 'f' ||
				fuente[i] == 'A' || fuente[i] == 'B' || fuente[i] == 'C' || fuente[i] == 'D' || fuente[i] == 'E' || fuente[i] == 'F' ||
				fuente[i] == '_') {
				i++
			}
			if i < len(fuente) && (fuente[i] == 'L' || fuente[i] == 'l') {
				i++
			}
			// el punto decimal: `FyD` (parrafo 77) trae `2.5` y `1.5f`, y el bucle de arriba partia el
			// numero en `2`, `.` y `5`. Se aceptan punto con digitos y despues las letras que el mismo
			// bucle admitia (el exponente con `e` y el sufijo `f`/`d`); con un signo en el exponente
			// (`1e-3`) el token sale partido y el lector de literales lo rechaza por su nombre.
			if i+1 < len(fuente) && fuente[i] == '.' && esDigito(fuente[i+1]) {
				i++
				for i < len(fuente) && (esDigito(fuente[i]) || fuente[i] == 'x' || fuente[i] == 'X' ||
					fuente[i] == 'a' || fuente[i] == 'b' || fuente[i] == 'c' || fuente[i] == 'd' || fuente[i] == 'e' || fuente[i] == 'f' ||
					fuente[i] == 'A' || fuente[i] == 'B' || fuente[i] == 'C' || fuente[i] == 'D' || fuente[i] == 'E' || fuente[i] == 'F' ||
					fuente[i] == '_') {
					i++
				}
			}
			out = append(out, tokenJava{kind: 'n', valor: fuente[desde:i], donde: pos{linea: linea, columna: columnaDe(desde)}})
		default:
			encontrado := ""
			for _, s := range simbolesJava {
				if strings.HasPrefix(fuente[i:], s) {
					encontrado = s
					break
				}
			}
			if encontrado == "" {
				r, _ := utf8.DecodeRuneInString(fuente[i:])
				return nil, fmt.Errorf("linea %d: el caracter %q no es de este subconjunto", linea, r)
			}
			out = append(out, tokenJava{kind: 'p', valor: encontrado, donde: pos{linea: linea, columna: columnaDe(antes)}})
			i += len(encontrado)
		}
		if i == antes {
			// la garantia de que este bucle avanza: si una vuelta no consumio nada, esto era un bucle
			// infinito comiéndose memoria
			r, _ := utf8.DecodeRuneInString(fuente[i:])
			return nil, fmt.Errorf("linea %d: el caracter %q no es de este subconjunto", linea, r)
		}
	}
	// El fin de fichero se queda en la **ultima linea con contenido**: si el fuente acaba en salto
	// de linea, la linea siguiente esta vacia y el error de una llave sin cerrar salia una linea por
	// debajo de la que el usuario ve (§200).
	finLinea, finColumna := linea, columnaDe(i)
	if recortado := strings.TrimRight(fuente, "\r\n"); len(recortado) < len(fuente) {
		finLinea = strings.Count(recortado, "\n") + 1
		finColumna = utf8.RuneCountInString(recortado[strings.LastIndex(recortado, "\n")+1:]) + 1
	}
	out = append(out, tokenJava{kind: 'f', donde: pos{linea: finLinea, columna: finColumna}})
	return out, nil
}

// esLetraJava y esDigito: ASCII, que es lo que cabe en este subconjunto.
func esLetraJava(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func esDigito(c byte) bool { return c >= '0' && c <= '9' }

// leerCadenaJava: una cadena con sus escapes. \u no se interpreta: las cadenas que emite la pagina son
// ASCII y el parrafo 40 lo deja medido asi.
func leerCadenaJava(fuente string, i, linea int) (string, int, error) {
	sb := strings.Builder{}
	i++ // la comilla de apertura
	for i < len(fuente) {
		c := fuente[i]
		if c == '"' {
			return sb.String(), i + 1, nil
		}
		if c == '\n' {
			return "", 0, fmt.Errorf("linea %d: una cadena sin cerrar", linea)
		}
		if c != '\\' {
			sb.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(fuente) {
			return "", 0, fmt.Errorf("linea %d: un escape a medio escribir", linea)
		}
		switch fuente[i] {
		case 'n':
			sb.WriteByte('\n')
		case 't':
			sb.WriteByte('\t')
		case 'r':
			sb.WriteByte('\r')
		case '"':
			sb.WriteByte('"')
		case '\'':
			sb.WriteByte('\'')
		case '\\':
			sb.WriteByte('\\')
		default:
			return "", 0, fmt.Errorf("linea %d: el escape \\%c no esta medido en este subconjunto", linea, fuente[i])
		}
		i++
	}
	return "", 0, fmt.Errorf("linea %d: una cadena sin cerrar", linea)
}

// ----- el parser -----

// leerCharJava lee un literal `char` desde la comilla simple abre y devuelve su punto de código. Las
// formas medidas con `javap -c` sobre la sonda `Letras` (párrafo 87): el caracter tal cual (`'A'` es
// `bipush 65`), los escapes simples `\n \t \r \b \f \' \" \\` (`'\n'` es `bipush 10`, `'\”` es
// `bipush 39`, `'\\'` es `bipush 92`), el escape octal (`'\101'` es `bipush 65`), el escape unicode
// escrito en el fuente (`'\u0008'` es `bipush 8`, `'\uFFFF'` es `ldc` con la entrada `Integer` 65535)
// y el caracter UTF-8 suelto (`'á'` es `sipush 225`). Con cualquier otra forma se rechaza por nombre.
func leerCharJava(fuente string, i int, linea int) (int32, int, error) {
	j := i + 1 // la comilla abre
	if j >= len(fuente) {
		return 0, 0, fmt.Errorf("linea %d: una comilla de char sin cerrar", linea)
	}
	var valor int32
	if fuente[j] != '\'' {
		v, nuevo, err := escapeJava(fuente, j, linea)
		if err != nil {
			return 0, 0, err
		}
		valor = v
		j = nuevo
	}
	if j >= len(fuente) || fuente[j] != '\'' {
		return 0, 0, fmt.Errorf("linea %d: un literal de char con mas de un caracter, o sin comilla de cierre", linea)
	}
	return valor, j + 1, nil
}

// escapeJava devuelve el punto de código de lo que hay detrás de la abre de un `char`: el caracter
// mismo (con su longitud en bytes si es UTF-8) o su escape. `leerCadenaJava` usa la misma tabla de
// escapes simples, medida ahí; lo que suma este camino es el octal, el unicode y el valor numérico.
func escapeJava(fuente string, i int, linea int) (int32, int, error) {
	if fuente[i] != '\\' {
		r, tam := utf8.DecodeRuneInString(fuente[i:])
		if r == utf8.RuneError && tam <= 1 {
			return 0, 0, fmt.Errorf("linea %d: un byte que no es UTF-8 dentro de un literal de char", linea)
		}
		return int32(r), i + tam, nil
	}
	j := i + 1
	if j >= len(fuente) {
		return 0, 0, fmt.Errorf("linea %d: un escape truncado dentro de un literal de char", linea)
	}
	switch e := fuente[j]; e {
	case 'n':
		return '\n', j + 1, nil
	case 't':
		return '\t', j + 1, nil
	case 'r':
		return '\r', j + 1, nil
	case 'b':
		return '\b', j + 1, nil
	case 'f':
		return '\f', j + 1, nil
	case '"':
		return '"', j + 1, nil
	case '\'':
		return '\'', j + 1, nil
	case '\\':
		return '\\', j + 1, nil
	case 'u':
		// el escape unicode: cuatro cifras hexadecimales. Medido `'\u0008'` como `bipush 8` y
		// `'\uFFFF'` como `ldc` con el entero 65535
		if j+4 >= len(fuente) {
			return 0, 0, fmt.Errorf("linea %d: un escape \\u sin sus cuatro cifras hexadecimales", linea)
		}
		v, err := strconv.ParseUint(fuente[j+1:j+5], 16, 16)
		if err != nil {
			return 0, 0, fmt.Errorf("linea %d: %q no es un escape \\u de cuatro cifras", linea, fuente[j+1:j+5])
		}
		return int32(v), j + 5, nil
	default:
		// el octal: hasta tres cifras entre 0 y 7, medido `'\101'` como `bipush 65`
		if e < '0' || e > '7' {
			return 0, 0, fmt.Errorf("linea %d: el escape \\%c no esta medido en un literal de char", linea, e)
		}
		cifras := ""
		for j < len(fuente) && len(cifras) < 3 && fuente[j] >= '0' && fuente[j] <= '7' {
			cifras += string(fuente[j])
			j++
		}
		v, err := strconv.ParseUint(cifras, 8, 16)
		if err != nil || v > 0o177 {
			return 0, 0, fmt.Errorf("linea %d: %q no es un escape octal de char (llega hasta \\177)", linea, cifras)
		}
		return int32(v), j, nil
	}
}

// analizadorJava lleva la posicion de lectura y el archivo que va armando. anonima cuenta las clases
// internas anonimas del fuente, que javac numera por orden de aparicion (MainActivity$1).
type analizadorJava struct {
	toks     []tokenJava
	i        int
	fuente   string
	paquete  string
	imports  []string
	anonima  int
	exterior string // nombre interno de la clase de primera nivel, para nombrar a sus anonimas
	// claseActual y metodoActual son el contexto del `new ... { ... }`: javac anota en la anonima la clase y
	// el metodo que la crean (EnclosingMethod, parrafo 42) y le pasa la clase como captura.
	claseActual  *jclas
	metodoActual *jmetodo
}

func (a *analizadorJava) actual() tokenJava { return a.toks[a.i] }

func (a *analizadorJava) avanzar() tokenJava {
	t := a.toks[a.i]
	if t.kind != 'f' {
		a.i++
	}
	return t
}

func (a *analizadorJava) es(kind byte, valor string) bool {
	t := a.actual()
	return t.kind == kind && t.valor == valor
}

// mira: el token n posiciones delante del actual (0 es el actual). Devuelve el fin de fichero si se
// pasa, para que las comprobaciones de forma no tengan que repetir los límites.
func (a *analizadorJava) mira(n int) tokenJava {
	i := a.i + n
	if i < 0 || i >= len(a.toks) {
		return tokenJava{kind: 'f'}
	}
	return a.toks[i]
}

// puedeAbrirExpresion: si el token puede empezar una expresión unaria. Hace falta para deshacer la
// ambigüedad de `(x)`: `tipo()` acepta un nombre suelto como molde, pero un molde tiene que llevar
// algo detrás. Sin esta comprobación, `-(x)` se leía como el molde de `x` y el `;` siguiente rompía
// el parser; medido con javac 17, `-(x)` es un paréntesis y `(int) x` es un molde.
func puedeAbrirExpresion(t tokenJava) bool {
	switch t.kind {
	case 'i', 'n', 's', 'c':
		return true
	case 'k':
		switch t.valor {
		case "this", "new", "true", "false", "null", "super":
			return true
		}
		return false
	case 'p':
		switch t.valor {
		case "(", "!", "-", "++", "--":
			return true
		}
	}
	return false
}

func (a *analizadorJava) comer(kind byte, valor string) error {
	if !a.es(kind, valor) {
		return fmt.Errorf("linea %d: se esperaba %q y hay %q", a.actual().donde.linea, valor, a.actual().valor)
	}
	a.avanzar()
	return nil
}

func (a *analizadorJava) comerSimbolo(v string) error { return a.comer('p', v) }

func (a *analizadorJava) comerPalabra(v string) error { return a.comer('k', v) }

// parsearJava lee un fuente completo. Devuelve el archivo con sus clases, en el orden del fuente.
func parsearJava(fuente string) (*archivoJava, error) {
	toks, err := tokenizarJava(fuente)
	if err != nil {
		return nil, err
	}
	a := &analizadorJava{toks: toks, fuente: fuente}
	salida := &archivoJava{}
	for a.es('k', "package") {
		a.avanzar()
		nombre, err := a.nombrePunto()
		if err != nil {
			return nil, err
		}
		a.paquete, salida.paquete = nombre, nombre
		if err := a.comerSimbolo(";"); err != nil {
			return nil, err
		}
	}
	for a.es('k', "import") {
		a.avanzar()
		nombre, err := a.nombreImport()
		if err != nil {
			return nil, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, err
		}
		a.imports = append(a.imports, nombre)
	}
	salida.imports = a.imports
	for a.actual().kind == 'k' || a.actual().kind == 'i' || a.es('p', "@") {
		// una anotación delante de una clase de primer nivel también es modificador: se lee aquí y se
		// entrega a la clase, igual que las lee miembroDeClase para los miembros.
		anots := []anotacionJ{}
		for a.es('p', "@") {
			an, err := a.anotacion()
			if err != nil {
				return nil, err
			}
			if !esAnotacionSinHuella(an.nombre) {
				anots = append(anots, an)
			}
		}
		c, err := a.clase(nil)
		if err != nil {
			return nil, err
		}
		c.anotaciones = append(anots, c.anotaciones...)
		salida.clases = append(salida.clases, c)
	}
	if len(salida.clases) == 0 {
		return nil, fmt.Errorf("el fuente no declara ninguna clase")
	}
	return salida, nil
}

// nombrePunto: `android.view.View`, `R.layout`, con los puntos que hagan falta.
func (a *analizadorJava) nombrePunto() (string, error) {
	sb := strings.Builder{}
	for {
		t := a.actual()
		if t.kind != 'i' && !(t.kind == 'k' && esNombreDeTipo(t.valor)) {
			return "", fmt.Errorf("linea %d: se esperaba un nombre y hay %q", t.donde.linea, t.valor)
		}
		sb.WriteString(t.valor)
		a.avanzar()
		if !a.es('p', ".") {
			return sb.String(), nil
		}
		a.avanzar()
		sb.WriteString(".")
	}
}

// nombreImport lee el nombre de un import, que es un nombrePunto y puede terminar en `.*`.
func (a *analizadorJava) nombreImport() (string, error) {
	sb := strings.Builder{}
	for {
		t := a.actual()
		if t.kind != 'i' && !(t.kind == 'k' && esNombreDeTipo(t.valor)) {
			return "", fmt.Errorf("linea %d: se esperaba un nombre de import y hay %q", t.donde.linea, t.valor)
		}
		sb.WriteString(t.valor)
		a.avanzar()
		if !a.es('p', ".") {
			return sb.String(), nil
		}
		a.avanzar()
		if a.es('p', "*") {
			a.avanzar()
			return sb.String() + ".*", nil
		}
		sb.WriteString(".")
	}
}

// esNombreDeTipo: `int`, `boolean` y compañía son palabras clave pero van en un nombre de tipo.
func esNombreDeTipo(v string) bool {
	switch v {
	case "int", "boolean", "long", "float", "double", "byte", "short", "char", "void", "var":
		return true
	}
	return false
}

// esAnotacionSinHuella: las tres que no escriben RuntimeVisibleAnnotations. @Deprecated la decide
// miembroDeClase aparte, porque escribe el atributo Deprecated y una anotación sin pares (§170).
func esAnotacionSinHuella(n string) bool {
	return n == "Override" || n == "java.lang.Override" ||
		n == "SuppressWarnings" || n == "java.lang.SuppressWarnings" ||
		n == "Deprecated" || n == "java.lang.Deprecated"
}

// clase lee una declaracion de clase, con sus anidadas.
func (a *analizadorJava) clase(exteriores *jclas) (*jclas, error) {
	c := &jclas{paquete: a.paquete, linea: a.actual().donde.linea, encierre: exteriores}
	anterior := a.claseActual
	a.claseActual = c
	defer func() { a.claseActual = anterior }()
	for {
		t := a.actual()
		switch {
		case a.es('k', "public"):
			c.esPublica = true
		case a.es('k', "static"):
			// una clase anidada estatica se emite igual que una de primera nivel: lo que cambia
			// son los flags, y aun no esta medido contra javap
			return nil, fmt.Errorf("linea %d: una clase anidada con `static` no esta medida en este compilador", t.donde.linea)
		case a.es('k', "final"):
			c.final = true
		case a.es('k', "abstract"):
			c.abstracta = true
		case a.es('k', "private"):
			return nil, fmt.Errorf("linea %d: un miembro `private` escrito aqui no esta medido; falta comparar sus flags con javap", t.donde.linea)
		case a.es('k', "protected"):
			return nil, fmt.Errorf("linea %d: `protected` no esta medido en este compilador", t.donde.linea)
		case a.es('k', "class"):
			a.avanzar()
			goto nombre
		case a.es('k', "enum"):
			// un enum simple: constantes sin argumentos ni cuerpo. javac lo baja a una clase final que
			// extiende java/lang/Enum, con $VALUES, values(), valueOf() y un <clinit> (medido con javac 17).
			c.esEnum = true
			c.final = true
			a.avanzar()
			goto nombre
		case a.es('k', "interface"):
			// una interface propia (parrafo 92): flags ACC_INTERFACE|ACC_ABSTRACT y metodos sin cuerpo
			c.esInterface = true
			c.abstracta = true
			a.avanzar()
			goto nombre
		case a.es('p', "@"):
			an, err := a.anotacion()
			if err != nil {
				return nil, err
			}
			if !esAnotacionSinHuella(an.nombre) {
				c.anotaciones = append(c.anotaciones, an)
			}
			continue
		default:
			return nil, fmt.Errorf("linea %d: %q no es un modificador que este parser conozca", t.donde.linea, t.valor)
		}
		a.avanzar()
	}
nombre:
	{
		t := a.actual()
		if t.kind != 'i' {
			return nil, fmt.Errorf("linea %d: la clase no tiene nombre", t.donde.linea)
		}
		c.nombre = t.valor
		if exteriores != nil {
			c.nombre = exteriores.nombre + "$" + t.valor
		} else {
			a.exterior = t.valor // las anonimas de esta clase se llaman Nombre$1, como las nombra javac
		}
		a.avanzar()
	}
	if c.esInterface && exteriores != nil {
		// una interface anidada cambia los flags del InnerClasses y el acceso; queda fuera del
		// subconjunto hasta que una sonda los mida (parrafo 92)
		return nil, fmt.Errorf("linea %d: una interface anidada no esta medida", c.linea)
	}
	if a.es('k', "extends") {
		a.avanzar()
		if c.esInterface {
			// una interface puede extender varias; van en el mismo `interfaces` del ClassFile
			for {
				nombre, args, err := a.tipoDeClausula()
				if err != nil {
					return nil, err
				}
				c.implementa = append(c.implementa, nombre)
				c.implementaArgs = append(c.implementaArgs, args)
				if !a.es('p', ",") {
					break
				}
				a.avanzar()
			}
		} else {
			nombre, args, err := a.tipoDeClausula()
			if err != nil {
				return nil, err
			}
			c.super = nombre
			c.superArgs = args
		}
	}
	if a.es('k', "implements") {
		a.avanzar()
		for {
			nombre, args, err := a.tipoDeClausula()
			if err != nil {
				return nil, err
			}
			c.implementa = append(c.implementa, nombre)
			c.implementaArgs = append(c.implementaArgs, args)
			if !a.es('p', ",") {
				break
			}
			a.avanzar()
		}
	}
	if err := a.comerSimbolo("{"); err != nil {
		return nil, err
	}
	if c.esEnum {
		for a.actual().kind == 'i' {
			c.constantes = append(c.constantes, a.avanzar().valor)
			if a.es('p', ",") {
				a.avanzar()
				continue
			}
			break
		}
		if len(c.constantes) == 0 {
			return nil, fmt.Errorf("linea %d: un enum sin constantes no esta medido", c.linea)
		}
		if a.es('p', ";") {
			a.avanzar()
		}
		if !a.es('p', "}") {
			return nil, fmt.Errorf("linea %d: solo se miden enums con constantes simples, sin argumentos ni cuerpo", a.actual().donde.linea)
		}
	}
	for !a.es('p', "}") {
		if a.es('p', ";") { // un punto y coma suelto dentro de la clase es legal en Java
			a.avanzar()
			continue
		}
		linea := a.actual().donde.linea
		miembro, err := a.miembroDeClase(c, linea)
		if err != nil {
			return nil, err
		}
		if miembro == nil {
			continue
		}
		if m, esMetodo := miembro.(jmetodo); esMetodo {
			c.metodos = append(c.metodos, m)
			continue
		}
		if campo, esCampo := miembro.(jcampo); esCampo {
			c.campos = append(c.campos, campo)
			continue
		}
		if anidada, esClase := miembro.(*jclas); esClase {
			anidada.paquete = c.paquete
			c.interior = append(c.interior, anidada)
		}
	}
	a.avanzar() // el } final
	return c, nil
}

// miembroDeClase lee un campo, un metodo o una clase anidada.
func (a *analizadorJava) miembroDeClase(c *jclas, linea int) (interface{}, error) {
	esEstatico, esFinal, esPublico, esAbstracta := false, false, false, false
	esPrivado, esProtegido := false, false
	esDeprecado := false
	anots := []anotacionJ{}
	for {
		switch {
		case a.es('k', "static"):
			esEstatico = true
		case a.es('k', "final"):
			esFinal = true
		case a.es('k', "public"):
			esPublico = true
		case a.es('k', "abstract"):
			esAbstracta = true
		case a.es('k', "private"):
			esPrivado = true
		case a.es('k', "protected"):
			esProtegido = true
		case a.es('p', "@"):
			an, err := a.anotacion()
			if err != nil {
				return nil, err
			}
			if an.nombre == "Deprecated" || an.nombre == "java.lang.Deprecated" {
				esDeprecado = true
			} else if !esAnotacionSinHuella(an.nombre) {
				anots = append(anots, an)
			}
			continue
		case a.es('k', "class"), a.es('k', "interface"):
			if esAbstracta {
				return nil, fmt.Errorf("linea %d: una clase anidada abstracta no esta medida", linea)
			}
			if esProtegido {
				return nil, fmt.Errorf("linea %d: una clase anidada `protected` no esta medida: cambia los flags del ClassFile y del InnerClasses", linea)
			}
			anidada, err := a.clase(c)
			if anidada != nil {
				// los modificadores los consumio este bucle, no la declaracion: sin esto la anidada perderia
				// el `public static final` que javac deja en su ClassFile (0x0031) y en el InnerClasses
				anidada.esPublica = anidada.esPublica || esPublico
				anidada.esPrivada = anidada.esPrivada || esPrivado
				anidada.esEstatica = anidada.esEstatica || esEstatico
				anidada.final = anidada.final || esFinal
				anidada.anotaciones = append(anots, anidada.anotaciones...)
			}
			return anidada, err
		default:
			goto cuerpo
		}
		a.avanzar()
	}
cuerpo:
	// `private` y `protected` en un metodo son los flags ACC_PRIVATE (0x0002) y ACC_PROTECTED (0x0004),
	// medidos en el parrafo 72 con `javap -v`; en un campo ya venian del parrafo 49. Un miembro privado
	// del exterior que usa una anonima pasa por los metodos sinteticos `access$NNN`, que mide ese mismo
	// parrafo.
	leerMetodo := func(retorno string, retornoGenerico *tipoArgumento) (interface{}, error) {
		m, err := a.metodo(c, retorno, retornoGenerico, esEstatico, esFinal, esPublico, esPrivado, esProtegido, linea)
		if err != nil {
			return m, err
		}
		m.deprecado = esDeprecado
		m.anotaciones = anots
		if esAbstracta || c.esInterface {
			m.abstracto = true
		}
		if c.esInterface {
			// en una interface los metodos son public abstract aunque el fuente no lo diga
			m.esPublico = true
			if len(m.cuerpo) > 0 {
				return m, fmt.Errorf("linea %d: un metodo de interface con cuerpo (default) no esta medido", linea)
			}
		}
		return m, nil
	}
	if a.es('k', "void") {
		a.avanzar()
		return leerMetodo("void", nil)
	}
	// un constructor: `Nombre(`. El nombre lo consume `metodo`, que es quien distingue el `<init>`
	// por `retorno == c.nombre`; avanzarlo tambien aqui se comia el parentesis (parrafo 91).
	if a.actual().kind == 'i' && a.actual().valor == sinPaquete(c.nombre) && a.pegaParentesis() {
		return leerMetodo(c.nombre, nil)
	}
	decl, err := a.tipoConArgumentos()
	if err != nil {
		return nil, err
	}
	tipo := decl.crudo()
	if a.actual().kind != 'i' {
		return nil, fmt.Errorf("linea %d: se esperaba un nombre de miembro y hay %q", a.actual().donde.linea, a.actual().valor)
	}
	if a.pegaParentesis() {
		if len(decl.args) > 0 {
			// un método que devuelve `List<Apk>` lleva la `Signature` genérica en el method_info; el
			// descriptor sigue siendo el del borrado
			generico := decl
			return leerMetodo(tipo, &generico)
		}
		return leerMetodo(tipo, nil)
	}
	nombre := a.avanzar().valor
	campo := jcampo{nombre: nombre, tipo: tipo, esEstatico: esEstatico, esFinal: esFinal, esPublico: esPublico,
		esPrivado: esPrivado, esProtegido: esProtegido, anotaciones: anots, donde: pos{linea: linea}}
	if len(decl.args) > 0 {
		// la Signature genérica del campo: el descriptor sigue siendo el del borrado, `List<Apk>` es
		// `Ljava/util/List;`, y los argumentos solo viajan en el atributo
		generico := decl
		campo.generico = &generico
	}
	// un campo con inicializador: `public static final int main=0x7f020000;`, que es justo la forma en
	// que el enlazador de la Fase 3 escribe la R.java que despues hay que compilar
	if a.es('p', "=") {
		a.avanzar()
		e, err := a.expresion()
		if err != nil {
			return nil, err
		}
		campo.valor = e
	}
	if err := a.comerSimbolo(";"); err != nil {
		return nil, err
	}
	return campo, nil
}

// metodo lee la firma y el cuerpo. `retorno` viene vacio para el constructor.
func (a *analizadorJava) metodo(c *jclas, retorno string, retornoGenerico *tipoArgumento, esEstatico, esFinal, esPublico, esPrivado, esProtegido bool, linea int) (jmetodo, error) {
	m := jmetodo{retorno: retorno, retornoGenerico: retornoGenerico, esEstatico: esEstatico, esFinal: esFinal,
		esPublico: esPublico, esPrivado: esPrivado, esProtegido: esProtegido, linea: linea}
	if retorno == c.nombre {
		m.nombre = "<init>"
		// el descriptor de un constructor vuelve V: el nombre de la clase no es su tipo de vuelta
		m.retorno = ""
		a.avanzar() // el nombre, que ya sabemos cual es
	} else {
		m.nombre = a.avanzar().valor
	}
	if err := a.comerSimbolo("("); err != nil {
		return m, err
	}
	if !a.es('p', ")") {
		for {
			decl, err := a.tipoConArgumentos()
			if err != nil {
				return m, err
			}
			tipo := decl.crudo()
			esVarargs := false
			if a.es('p', "...") {
				// `tipo... nombre` es azucar del ultimo parametro: el tipo que se guarda es el arreglo
				// que javac escribe en el descriptor (`[I`, `[Ljava/lang/String;`) y la firma lleva
				// ACC_VARARGS, que es lo que fija el bit 0x0080 del method_info (§194).
				a.avanzar()
				esVarargs = true
				decl.arreglos++
				tipo = decl.crudo()
			}
			if a.actual().kind != 'i' {
				return m, fmt.Errorf("linea %d: un parametro necesita nombre", a.actual().donde.linea)
			}
			nombre := a.avanzar().valor
			p := jparam{tipo: tipo, nombre: nombre, donde: pos{linea: linea}}
			if len(decl.args) > 0 {
				// el parámetro genérico (`List<Apk>`) lleva su firma al atributo Signature del método;
				// el descriptor del parámetro sigue siendo el del borrado (`Ljava/util/List;`)
				generico := decl
				p.generico = &generico
			}
			m.params = append(m.params, p)
			if esVarargs {
				// JLS 8.4.1: solo el ultimo parametro puede ser varargs. Si sigue otro, se rechaza
				// nombrando el que lo era en vez de aceptar una firma que javac no admite.
				if a.es('p', ",") {
					return m, fmt.Errorf("linea %d: el parametro varargs %s tiene que ser el ultimo",
						a.actual().donde.linea, nombre)
				}
				m.varargs = true
			}
			if !a.es('p', ",") {
				break
			}
			a.avanzar()
		}
	}
	if err := a.comerSimbolo(")"); err != nil {
		return m, err
	}
	m.locales = map[string]string{}
	for _, p := range m.params {
		m.locales[p.nombre] = p.tipo
	}
	if a.es('k', "throws") {
		// `throws A, B`: cada nombre va al atributo Exceptions del method_info (parrafo 92). Se guarda
		// el nombre tal como lo escribe el fuente; el emisor lo resuelve contra el fuente y el jar.
		a.avanzar()
		for {
			nombre, err := a.nombrePunto()
			if err != nil {
				return m, err
			}
			m.lanza = append(m.lanza, nombre)
			if !a.es('p', ",") {
				break
			}
			a.avanzar()
		}
	}
	if a.es('p', ";") { // un metodo abstracto o de interface: sin cuerpo
		a.avanzar()
		return m, nil
	}
	// el metodo se apunta antes del cuerpo: una anonima que aparezca dentro necesita saber donde nacio para
	// su EnclosingMethod (parrafo 42), y aqui el nombre, la vuelta y los parametros ya estan leidos
	antes := a.metodoActual
	a.metodoActual = &m
	cuerpo, err := a.bloque()
	a.metodoActual = antes
	if err != nil {
		return m, err
	}
	m.cuerpo = cuerpo
	// bloque dejo atras la llave de cierre: el token anterior es ese `}`, y su linea es la que javac
	// anota en el `return` implicito del metodo (parrafo 40)
	if a.i > 0 && a.i <= len(a.toks) {
		m.lineaFin = a.toks[a.i-1].donde.linea
	}
	return m, nil
}

// pegaParentesis: si despues del token actual viene `(`, esto es un metodo y no un campo.
func (a *analizadorJava) pegaParentesis() bool {
	return a.i+1 < len(a.toks) && a.toks[a.i].kind == 'i' && a.toks[a.i+1].valor == "("
}

// anotacion consume `@Nombre(elemento = valor, …)` o `@Nombre`. @Override y @SuppressWarnings tienen
// retención SOURCE y no dejan huella en el .class (parrafo 40), así que se leen y se descartan.
// @Deprecated se descarta aquí también: el que decide es miembroDeClase, que pone el atributo
// Deprecated y el RuntimeVisibleAnnotations sin pares que mide el parrafo 170. Cualquier otra anotación
// se guarda con sus elementos para que el emisor escriba su RuntimeVisibleAnnotations.
func (a *analizadorJava) anotacion() (anotacionJ, error) {
	an := anotacionJ{donde: a.actual().donde}
	a.avanzar() // @
	if a.actual().kind == 'i' || (a.actual().kind == 'k' && esNombreDeTipo(a.actual().valor)) {
		an.nombre, _ = a.nombrePunto()
	} else {
		return an, fmt.Errorf("linea %d: la anotación no nombra su tipo", an.donde.linea)
	}
	if a.es('p', "(") {
		a.avanzar()
		if !a.es('p', ")") {
			for {
				if a.actual().kind != 'i' {
					return an, fmt.Errorf("linea %d: se esperaba el nombre de un elemento de la anotación @%s", a.actual().donde.linea, an.nombre)
				}
				nombreElem := a.avanzar().valor
				if err := a.comerSimbolo("="); err != nil {
					return an, err
				}
				valor, err := a.anotValor()
				if err != nil {
					return an, err
				}
				an.elementos = append(an.elementos, anotElemJ{nombre: nombreElem, valor: valor})
				if !a.es('p', ",") {
					break
				}
				a.avanzar()
			}
		}
		if err := a.comerSimbolo(")"); err != nil {
			return an, err
		}
	}
	return an, nil
}

// anotValor lee el valor de un elemento de anotación. Un arreglo `{a, b}` se lee aquí porque la
// expresión ordinaria no sabe de llaves; lo demás se apoya en el parser de expresiones y se baja al
// subconjunto de element_value de JVMS 4.7.16.1.
func (a *analizadorJava) anotValor() (anotValorJ, error) {
	v := anotValorJ{}
	if a.es('p', "{") {
		v.kind = 'a'
		a.avanzar()
		if !a.es('p', "}") {
			for {
				hijo, err := a.anotValor()
				if err != nil {
					return v, err
				}
				v.lista = append(v.lista, hijo)
				if !a.es('p', ",") {
					break
				}
				a.avanzar()
			}
		}
		return v, a.comerSimbolo("}")
	}
	// `ElementType.TYPE`: un nombre con puntos no es una expresión de este parser (sería un acceso
	// a campo), así que se reconoce antes. Es el element_value de tipo `e` (constante de enum).
	if a.actual().kind == 'i' && a.mira(1).kind == 'p' && a.mira(1).valor == "." && a.mira(2).kind == 'i' {
		nombre, err := a.nombrePunto()
		if err != nil {
			return v, err
		}
		i := strings.LastIndex(nombre, ".")
		v.kind, v.texto, v.nombre = 'e', nombre[:i], nombre[i+1:]
		return v, nil
	}
	e, err := a.expresion()
	if err != nil {
		return v, err
	}
	switch x := e.(type) {
	case jlit:
		switch x.tipoJ {
		case 'i', 'c':
			return anotValorJ{kind: 'i', entero: int64(x.entero)}, nil
		case 'b':
			n := int64(0)
			if x.booleano {
				n = 1
			}
			return anotValorJ{kind: 'b', entero: n}, nil
		case 's':
			return anotValorJ{kind: 's', texto: x.texto}, nil
		case 'j':
			return v, fmt.Errorf("linea %d: un elemento de anotación `long` no está medido", a.actual().donde.linea)
		}
	case jclase:
		return anotValorJ{kind: 'c', texto: x.nombre}, nil
	case jnegacion:
		if lit, ok := x.dentro.(jlit); ok && lit.tipoJ == 'b' {
			n := int64(1)
			if lit.booleano {
				n = 0
			}
			return anotValorJ{kind: 'b', entero: n}, nil
		}
	}
	return v, fmt.Errorf("linea %d: el valor de este elemento de anotación no es un elemento simple medido", a.actual().donde.linea)
}

// tipo lee un nombre de tipo con sus corchetes. Un array no esta medido en el emisor.
func (a *analizadorJava) tipo() (string, error) {
	nombre, err := a.nombrePunto()
	if err != nil {
		return "", err
	}
	// los corchetes del arreglo van pegados al nombre, en el orden del fuente: `String[]`,
	// `int[][]`. El descriptor los lee el emisor (`descriptorDe`), que ya sabe escribirlos.
	for a.es('p', "[") {
		a.avanzar()
		if err := a.comerSimbolo("]"); err != nil {
			return "", err
		}
		nombre += "[]"
	}
	return nombre, nil
}

// tipoDeClausula lee el tipo de un `extends` o un `implements`: el nombre crudo y sus argumentos de
// tipo. `extends ArrayAdapter<Apk>` deja `android.widget.ArrayAdapter` en el nombre —que es lo que va
// al `super_class` del `.class` y al descriptor del dex, porque el borrado tira `Apk`— y el argumento
// aparte, que solo se usa para escribir la `Signature` genérica.
func (a *analizadorJava) tipoDeClausula() (string, []tipoArgumento, error) {
	nombre, err := a.nombrePunto()
	if err != nil {
		return "", nil, err
	}
	args, err := a.argumentosDeTipo()
	if err != nil {
		return "", nil, err
	}
	return nombre, args, nil
}

// argumentosDeTipo lee `<>` si está: el nombre del fuente, los corchetes del arreglo y los suyos
// propios, que van anidados (`T<U,V>`, `List<List<String>>`). Los comodines entran como argumento
// desde §241 (`AdapterView<?>`); los tipos primitivos se rechazan en el emisor al resolverlos.
func (a *analizadorJava) argumentosDeTipo() ([]tipoArgumento, error) {
	if !a.es('p', "<") {
		return nil, nil
	}
	a.avanzar()
	args := []tipoArgumento{}
	for {
		arg, err := a.argumentoDeTipo()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !a.es('p', ",") {
			break
		}
		a.avanzar()
	}
	if err := a.cierraAngulo(); err != nil {
		return nil, err
	}
	return args, nil
}

// leeArgumentosDeTipoDelNuevo es el interruptor de la sonda del `new` con argumentos de tipo: apagado,
// el parser vuelve a la ruta de antes y rechaza `new ArrayList<>()` con el mensaje literal del usuario.
// Lo usa la falsificación de `javaemisor_diamante_test.go`; encendido en el binario que se publica.
var leeArgumentosDeTipoDelNuevo = true

// argumentosDeTipoDelNuevo consume los argumentos de tipo de un `new` si los trae: el diamante
// `new ArrayList<>()`, cuyo argumento se infiere de la declaración, y la forma explícita
// `new ArrayList<Apk>()`. En los dos casos el `.class` de javac no lleva `Signature` para el `new`
// —el argumento es del tipo de la variable, no de la expresión— y el descriptor es el del borrado,
// así que lo único que hace falta es consumirlos y dejar el nombre crudo en `tipo`.
func (a *analizadorJava) argumentosDeTipoDelNuevo() error {
	if !leeArgumentosDeTipoDelNuevo || !a.es('p', "<") {
		return nil
	}
	// el diamante `<>`: los dos signos van seguidos y no hay tipo que leer
	if a.i+1 < len(a.toks) && a.toks[a.i+1].kind == 'p' && a.toks[a.i+1].valor == ">" {
		a.avanzar()
		a.avanzar()
		return nil
	}
	_, err := a.argumentosDeTipo()
	return err
}

// argumentoDeTipo lee un tipo con sus corchetes y, si los tiene, sus propios argumentos, o un comodín.
func (a *analizadorJava) argumentoDeTipo() (tipoArgumento, error) {
	// el comodín solo cabe como argumento de tipo (`AdapterView<?>`); como tipo declarado Java no lo
	// admite, así que no hace falta leerlo en ningún otro sitio
	if a.es('p', "?") {
		return a.comodinDeTipo()
	}
	return a.argumentoDeTipoSinComodin()
}

// leeComodinesDeTipo es el interruptor de la sonda de los comodines: apagado, el parser rechaza el
// `?` con el mensaje literal que el usuario leyó antes de §241 —«el caracter '?' no es de este
// subconjunto»—. Lo usa la falsificación de `javaemisor_comodin_test.go`; encendido en el binario que
// se publica.
var leeComodinesDeTipo = true

// leeFinalEnDeclaracion es el interruptor de la sonda del `final` en declaraciones locales: apagado,
// la declaración `final Tipo nombre` vuelve a la ruta de la expresión y el parser rechaza con el
// mensaje literal que el usuario leyó —«"final" no es una expresion que este parser reconozca»—. Lo
// usa la falsificación de `javaemisor_final_local_test.go`; encendido en el binario que se publica.
var leeFinalEnDeclaracion = true

// comodinDeTipo lee `?`, `? extends T` o `? super T`. El signo que se guarda es el que la `Signature`
// escribe delante de la cota (JVMS 4.7.9.1): `*` sin cota, `+` para `extends` y `-` para `super`.
func (a *analizadorJava) comodinDeTipo() (tipoArgumento, error) {
	t := a.actual()
	a.avanzar()
	if !leeComodinesDeTipo {
		return tipoArgumento{}, fmt.Errorf("linea %d: el caracter '?' no es de este subconjunto", t.donde.linea)
	}
	arg := tipoArgumento{comodin: '*'}
	if a.es('k', "extends") || a.es('k', "super") {
		signo := byte('+')
		if a.actual().valor == "super" {
			signo = '-'
		}
		a.avanzar()
		// la cota es un tipo, nunca otro comodín: `? extends ?` no es Java
		cota, err := a.argumentoDeTipoSinComodin()
		if err != nil {
			return tipoArgumento{}, err
		}
		arg.comodin = signo
		arg.cota = &cota
	}
	return arg, nil
}

// argumentoDeTipoSinComodin lee un tipo con sus corchetes y, si los tiene, sus propios argumentos.
func (a *analizadorJava) argumentoDeTipoSinComodin() (tipoArgumento, error) {
	nombre, err := a.nombrePunto()
	if err != nil {
		return tipoArgumento{}, err
	}
	arg := tipoArgumento{nombre: nombre}
	if a.es('p', "<") {
		arg.args, err = a.argumentosDeTipo()
		if err != nil {
			return tipoArgumento{}, err
		}
	}
	for a.es('p', "[") {
		a.avanzar()
		if err := a.comerSimbolo("]"); err != nil {
			return tipoArgumento{}, err
		}
		arg.arreglos++
	}
	return arg, nil
}

// tipoConArgumentos lee un tipo declarado —el de un campo, un parámetro o una variable local—: el
// nombre crudo con sus corchetes y, si los trae, sus argumentos de tipo. El crudo es lo que va al
// descriptor —el borrado tira los argumentos, así que `List<Apk>` es `Ljava/util/List;`— y los
// argumentos solo se usan para escribir la `Signature` genérica, que es lo que javac deja en el
// `.class` del campo o del método.
func (a *analizadorJava) tipoConArgumentos() (tipoArgumento, error) {
	// el `final` de una declaración local se consume aquí, antes del tipo. En un campo o en un
	// parámetro el modificador ya lo leyó el bucle de modificadores; en una local solo lo veía
	// `pareceDeclaracion`, y el tipo se paraba en la palabra clave.
	esFinal := false
	if leeFinalEnDeclaracion && a.es('k', "final") {
		a.avanzar()
		esFinal = true
	}
	t, err := a.argumentoDeTipo()
	t.esFinal = esFinal
	return t, err
}

// cierraAngulo come el `>` que cierra unos argumentos de tipo. El tokenizador junta los `>` seguidos
// (`List<List<String>>` trae un solo token `>>`), así que se quita un carácter al token en vez de
// avanzar: el `>` que sobra tiene que verlo el que cierra los argumentos de fuera.
func (a *analizadorJava) cierraAngulo() error {
	t := a.actual()
	if t.kind != 'p' || t.valor == "" || t.valor[0] != '>' {
		return fmt.Errorf("linea %d: se esperaba «>» y hay %q", t.donde.linea, t.valor)
	}
	if t.valor == ">" {
		a.avanzar()
		return nil
	}
	a.toks[a.i].valor = t.valor[1:]
	return nil
}

// bloque lee `{ ... }` y devuelve sus sentencias.
func (a *analizadorJava) bloque() ([]jstmt, error) {
	if err := a.comerSimbolo("{"); err != nil {
		return nil, err
	}
	out := []jstmt{}
	for !a.es('p', "}") {
		if a.es('p', ";") {
			a.avanzar()
			continue
		}
		antes := a.i
		s, producida, err := a.sentencia()
		if err != nil {
			return nil, err
		}
		if a.i == antes {
			// una sentencia que no consume nada seria un bucle infinito: se prefiere el mensaje
			return nil, fmt.Errorf("linea %d: no se sabe leer lo que sigue de %q", a.actual().donde.linea, a.actual().valor)
		}
		if !producida {
			continue
		}
		out = append(out, s)
	}
	a.avanzar()
	return out, nil
}

// sentencia lee una sentencia. El segundo valor dice si produjo una: `if` sin cuerpo devuelve false
// despues de haber consumido, y el bloque no anade nada.
// cambio lee `switch (selector) { case k: … break; default: … }`. Las claves tienen que ser literales
// enteros y cada caso termina en `break` o en `return`: la caída al caso siguiente no está medida y se
// rechaza con su nombre (parrafo 64).
func (a *analizadorJava) cambio(linea int) (jstmt, bool, error) {
	a.avanzar() // switch
	if err := a.comerSimbolo("("); err != nil {
		return nil, false, err
	}
	sel, err := a.expresion()
	if err != nil {
		return nil, false, err
	}
	if err := a.comerSimbolo(")"); err != nil {
		return nil, false, err
	}
	if err := a.comerSimbolo("{"); err != nil {
		return nil, false, err
	}
	cam := jswitch{selector: sel, donde: pos{linea: linea}}
	vistas := map[int32]bool{}
	for !a.es('p', "}") {
		esDefecto := false
		var valor int32
		cadena := ""
		switch {
		case a.es('k', "case"):
			a.avanzar()
			e, err := a.expresion()
			if err != nil {
				return nil, false, err
			}
			lit, esLit := e.(jlit)
			if !esLit || (lit.tipoJ != 'i' && lit.tipoJ != 'c' && lit.tipoJ != 's') {
				return nil, false, fmt.Errorf("linea %d: una clave de `case` que no es un literal entero, char o cadena no esta medida", a.actual().donde.linea)
			}
			if lit.tipoJ == 's' {
				cam.esCadena = true
				cadena = lit.texto
				valor = int32(len(cam.casos))
			} else {
				// una clave `char` es el punto de codigo: medido en `Letras2`, `switch (c) { case 'a': … }`
				// sale `lookupswitch { 97: 20 … }` (parrafo 87)
				valor = lit.entero
			}
		case a.es('k', "default"):
			a.avanzar()
			esDefecto = true
		default:
			return nil, false, fmt.Errorf("linea %d: se esperaba `case` o `default` dentro del switch", a.actual().donde.linea)
		}
		if err := a.comerSimbolo(":"); err != nil {
			return nil, false, err
		}
		cuerpo := []jstmt{}
		terminado := false
		rompe := false
		for !a.es('p', "}") && !a.es('k', "case") && !a.es('k', "default") {
			if a.es('k', "break") {
				a.avanzar()
				if err := a.comerSimbolo(";"); err != nil {
					return nil, false, err
				}
				terminado = true
				rompe = true
				break
			}
			s, _, err := a.sentencia()
			if err != nil {
				return nil, false, err
			}
			cuerpo = append(cuerpo, s)
		}
		if !terminado && a.es('p', "}") {
			// el último brazo puede acabar sin `break`: cae en el final del switch, que es lo mismo
			terminado = true
		}
		if !terminado && (a.es('k', "case") || a.es('k', "default")) {
			// caída al siguiente brazo: javac no escribe `goto` y el control sigue en el cuerpo
			// siguiente, que es el que viene después en el fuente (§129)
			terminado = true
		}
		if !terminado && len(cuerpo) > 0 {
			if _, esRetorno := cuerpo[len(cuerpo)-1].(jretorno); esRetorno {
				terminado = true
			}
		}
		if !terminado {
			return nil, false, fmt.Errorf("linea %d: un `case` que no termina en `break` ni en `return` no esta medido", linea)
		}
		if esDefecto {
			cam.defecto = cuerpo
			cam.orden = append(cam.orden, -1)
			continue
		}
		if vistas[valor] {
			motivo := fmt.Sprintf("la clave %d aparece dos veces en el switch", valor)
			if cam.esCadena {
				motivo = "la cadena " + cadena + " aparece dos veces en el switch"
			}
			return nil, false, fmt.Errorf("linea %d: %s", linea, motivo)
		}
		vistas[valor] = true
		cam.casos = append(cam.casos, jcaso{
			valor: valor, cadena: cadena, cuerpo: cuerpo, rompe: rompe, donde: pos{linea: linea},
		})
		cam.orden = append(cam.orden, len(cam.casos)-1)
	}
	if err := a.comerSimbolo("}"); err != nil {
		return nil, false, err
	}
	if len(cam.casos) == 0 {
		return nil, false, fmt.Errorf("linea %d: un `switch` sin ningun `case` no esta medido", linea)
	}
	return cam, true, nil
}

// admitePorCada es el interruptor de la sonda del `for` con dos puntos: apagado, el parser vuelve a la
// ruta de antes y rechaza `for (String path: x)` con el mensaje literal del usuario. Lo usa la
// falsificación de `javaemisor_porcada_test.go`; encendido en el binario que se publica.
var admitePorCada = true

// esPorCada: si el encabezado del `for` que empieza en el token actual es el de la forma con dos
// puntos, `for (T v : expr)`. Se distingue del clasico por el `:` de nivel cero del encabezado: el
// subconjunto no tokeniza el `?`, asi que no hay `?:` que lo confunda, y el `;` de nivel cero es la
// firma del clasico. La llave, el corchete y el parentesis suben el nivel para no leer el `:` de un
// `new` con inicializador ni el de un indice.
func (a *analizadorJava) esPorCada() bool {
	if !a.es('p', "(") {
		return false
	}
	// el `(` del encabezado ya esta delante: el nivel cero es su interior y el `)` que lo cierra
	// termina la busqueda
	hondo := 0
	for i := a.i + 1; i < len(a.toks); i++ {
		t := a.toks[i]
		if t.kind == 'f' {
			return false
		}
		if t.kind != 'p' {
			continue
		}
		switch t.valor {
		case "(", "[", "{":
			hondo++
		case ")", "]", "}":
			if hondo == 0 {
				return false
			}
			hondo--
		case ";":
			if hondo == 0 {
				return false
			}
		case ":":
			if hondo == 0 {
				return true
			}
		}
	}
	return false
}

// cadaUno lee el `for` con dos puntos ya abierto: `for (T v : expr) cuerpo`. La variable entra en los
// locales del metodo como una declaracion mas, para que una anonima del cuerpo la capture.
func (a *analizadorJava) cadaUno(linea int) (jstmt, bool, error) {
	if a.es('k', "final") {
		// el `final` del encabezado no deja huella en el `.class` sin `-g:vars`: la LineNumberTable
		// no lo mira y el descriptor del local es el del tipo
		a.avanzar()
	}
	decl, err := a.tipoConArgumentos()
	if err != nil {
		return nil, false, err
	}
	tipo := decl.crudo()
	if a.actual().kind != 'i' {
		return nil, false, fmt.Errorf("linea %d: la variable del `for` con dos puntos no tiene nombre", a.actual().donde.linea)
	}
	nombre := a.avanzar().valor
	if err := a.comerSimbolo(":"); err != nil {
		return nil, false, err
	}
	coleccion, err := a.expresion()
	if err != nil {
		return nil, false, err
	}
	if err := a.comerSimbolo(")"); err != nil {
		return nil, false, err
	}
	cuerpo, _, err := a.sentencia()
	if err != nil {
		return nil, false, err
	}
	if a.metodoActual != nil {
		if a.metodoActual.locales == nil {
			a.metodoActual.locales = map[string]string{}
		}
		a.metodoActual.locales[nombre] = tipo
	}
	return jporCada{tipo: tipo, nombre: nombre, coleccion: coleccion, cuerpo: cuerpo,
		donde: pos{linea: linea}, lineaCierre: a.toks[a.i-1].donde.linea}, true, nil
}

func (a *analizadorJava) sentencia() (jstmt, bool, error) {
	linea := a.actual().donde.linea
	switch {
	case a.es('p', "{"):
		b, err := a.bloque()
		return jbloque{sentencias: b, donde: pos{linea: linea}}, err == nil, err
	case a.es('k', "return"):
		a.avanzar()
		if a.es('p', ";") {
			a.avanzar()
			return jretorno{donde: pos{linea: linea}}, true, nil
		}
		e, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jretorno{e: e, donde: pos{linea: linea}}, true, nil
	case a.es('k', "if"):
		a.avanzar()
		if err := a.comerSimbolo("("); err != nil {
			return nil, false, err
		}
		cond, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(")"); err != nil {
			return nil, false, err
		}
		entonces, _, err := a.sentencia()
		if err != nil {
			return nil, false, err
		}
		siNo := jstmt(nil)
		if a.es('k', "else") {
			a.avanzar()
			s, _, err := a.sentencia()
			if err != nil {
				return nil, false, err
			}
			siNo = s
		}
		return jif{cond: cond, entonces: entonces, siNo: siNo, donde: pos{linea: linea}}, true, nil
	case a.es('k', "while"):
		a.avanzar()
		if err := a.comerSimbolo("("); err != nil {
			return nil, false, err
		}
		cond, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(")"); err != nil {
			return nil, false, err
		}
		cuerpo, _, err := a.sentencia()
		if err != nil {
			return nil, false, err
		}
		return jmientras{cond: cond, cuerpo: cuerpo, donde: pos{linea: linea}}, true, nil
	case a.es('k', "for"):
		a.avanzar()
		porCada := admitePorCada && a.esPorCada()
		if err := a.comerSimbolo("("); err != nil {
			return nil, false, err
		}
		if porCada {
			return a.cadaUno(linea)
		}
		var inicio jstmt
		if a.es('p', ";") {
			a.avanzar()
		} else {
			s, _, err := a.sentencia() // se come su propio `;`
			if err != nil {
				return nil, false, err
			}
			inicio = s
		}
		if a.es('p', ";") {
			return nil, false, fmt.Errorf("linea %d: un `for` sin condicion no esta medido", linea)
		}
		cond, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		piezas := []jstmt{}
		if !a.es('p', ")") {
			// el paso puede ser una llamada, `i++` o una asignacion (tambien `+=`), que en este
			// subconjunto es sentencia y no expresion: se lee el destino y, si viene un operador de
			// asignacion, se arma la sentencia
			dst, err := a.expresion()
			if err != nil {
				return nil, false, err
			}
			// `for (;; i = i + 1)` llega aqui como `jasignExp`: es el paso, una sentencia, y no deja
			// el valor en la pila
			if asig, esAsig := dst.(jasignExp); esAsig {
				piezas = append(piezas, jasign{destino: asig.destino, valor: asig.valor, donde: pos{linea: linea}})
			} else if op, esAsig := operadorDeAsignacion(a.actual()); esAsig {
				a.avanzar()
				valor, err := a.expresion()
				if err != nil {
					return nil, false, err
				}
				piezas = append(piezas, asignacion(dst, op, valor, pos{linea: linea}))
			} else {
				piezas = append(piezas, jstmtExpr{e: dst})
			}
		}
		if err := a.comerSimbolo(")"); err != nil {
			return nil, false, err
		}
		cuerpo, _, err := a.sentencia()
		if err != nil {
			return nil, false, err
		}
		// el paso se guarda aparte del cuerpo: `continue` salta a él (parrafo 70)
		paso := jstmt(nil)
		if len(piezas) == 1 {
			paso = piezas[0]
		} else if len(piezas) > 1 {
			return nil, false, fmt.Errorf("linea %d: mas de una sentencia en el paso de un `for`", linea)
		}
		p := jpara{cuerpo: cuerpo, cond: cond, paso: paso, donde: pos{linea: linea}}
		if inicio == nil {
			return p, true, nil
		}
		return jbloque{sentencias: []jstmt{inicio, p}, donde: pos{linea: linea}}, true, nil
	case a.es('k', "switch"):
		return a.cambio(linea)
	case a.es('k', "break"):
		a.avanzar()
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jrompe{donde: pos{linea: linea}}, true, nil
	case a.es('k', "continue"):
		a.avanzar()
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jcontinua{donde: pos{linea: linea}}, true, nil
	case a.es('k', "do"):
		a.avanzar()
		cuerpo, _, err := a.sentencia()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerPalabra("while"); err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo("("); err != nil {
			return nil, false, err
		}
		cond, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(")"); err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jhacer{cuerpo: cuerpo, cond: cond, donde: pos{linea: linea}}, true, nil
	case a.es('k', "try"):
		a.avanzar()
		// try-with-resources: `try (T r = expr; …) { … }`. Los recursos se guardan en orden y es el
		// emisor quien escribe la forma de javac (un nivel anidado por recurso, con
		// `Throwable.addSuppressed`), igual que en el caso de uno solo
		recursos := []jdecl{}
		if a.es('p', "(") {
			a.avanzar()
			for {
				tipo, err := a.tipo()
				if err != nil {
					return nil, false, err
				}
				if a.actual().kind != 'i' {
					return nil, false, fmt.Errorf("linea %d: el recurso del try no tiene nombre", a.actual().donde.linea)
				}
				nombre := a.avanzar().valor
				if err := a.comerSimbolo("="); err != nil {
					return nil, false, err
				}
				valor, err := a.expresion()
				if err != nil {
					return nil, false, err
				}
				if a.metodoActual != nil {
					if a.metodoActual.locales == nil {
						a.metodoActual.locales = map[string]string{}
					}
					a.metodoActual.locales[nombre] = tipo
				}
				recursos = append(recursos, jdecl{tipo: tipo, nombre: nombre, valor: valor, final: true, donde: pos{linea: linea}})
				if !a.es('p', ";") {
					break
				}
				a.avanzar()
			}
			if err := a.comerSimbolo(")"); err != nil {
				return nil, false, err
			}
		}
		cuerpo, _, err := a.sentencia()
		if err != nil {
			return nil, false, err
		}
		var atrapas []capturaTry
		for a.es('k', "catch") {
			lineaAtrapa := a.actual().donde.linea
			a.avanzar()
			if err := a.comerSimbolo("("); err != nil {
				return nil, false, err
			}
			tipo, err := a.tipo()
			if err != nil {
				return nil, false, err
			}
			// `catch (A | B e)`: javac escribe una fila de la tabla por tipo y un solo manejador,
			// con la excepcion guardada en el mismo hueco (§132)
			tipos := []string{tipo}
			for a.es('p', "|") {
				a.avanzar()
				otro, err := a.tipo()
				if err != nil {
					return nil, false, err
				}
				tipos = append(tipos, otro)
			}
			if a.actual().kind != 'i' {
				return nil, false, fmt.Errorf("linea %d: el catch no nombra su excepcion", linea)
			}
			excep := a.avanzar().valor
			if err := a.comerSimbolo(")"); err != nil {
				return nil, false, err
			}
			atrapa, _, err := a.sentencia()
			if err != nil {
				return nil, false, err
			}
			atrapas = append(atrapas, capturaTry{tipos: tipos, excep: excep, atrapa: atrapa, lineaAtrapa: lineaAtrapa})
		}
		var finalmente jstmt
		lineaAviso, lineaCierre := 0, 0
		if a.es('k', "finally") {
			a.avanzar()
			if !a.es('p', "{") {
				return nil, false, fmt.Errorf("linea %d: el `finally` no abre bloque", linea)
			}
			// el renglon del primer renglon del bloque es el que javac anota en la entrada del
			// manejador excepcional, y el de la llave que lo cierra el que anota en la re-lanza:
			// medido en el parrafo 75 sobre `Fin.simple` (9 y 10) y `Fin.lanza` (43 y 44)
			lineaAviso = a.toks[a.i+1].donde.linea
			b, err := a.bloque()
			if err != nil {
				return nil, false, err
			}
			finalmente = jbloque{sentencias: b, donde: pos{linea: linea}}
		}
		// la llave que cerro el ultimo bloque del constructo es la linea con la que javac anota el
		// `goto` de salida: medido en `Fini.dosCatch` (la del `catch`) y en `Fini.multi` (la del
		// `finally`)
		lineaCierre = a.toks[a.i-1].donde.linea
		if len(atrapas) == 0 && finalmente == nil && len(recursos) == 0 {
			return nil, false, fmt.Errorf("linea %d: un `try` sin `catch`, `finally` ni recursos no es Java valido", linea)
		}
		return jtry{cuerpo: cuerpo, atrapas: atrapas, finalmente: finalmente,
			recursos: recursos, lineaAviso: lineaAviso, lineaCierre: lineaCierre,
			donde: pos{linea: linea}}, true, nil
	case a.es('k', "throw"):
		a.avanzar()
		e, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jlanza{e: e, donde: pos{linea: linea}}, true, nil
	}
	// declaracion local o expresion
	tipoActual := a.actual()
	// el `final` de una declaración local (`final Dialog dialog = new Dialog(this);`) abre tipo igual
	// que un nombre de tipo. `pareceDeclaracion` ya lo saltaba, pero este guardia se paraba antes de
	// llegar y la sentencia caía en la ruta de la expresión.
	esFinalLocal := leeFinalEnDeclaracion && tipoActual.kind == 'k' && tipoActual.valor == "final"
	if (tipoActual.kind == 'i' || (tipoActual.kind == 'k' && (esNombreDeTipo(tipoActual.valor) || esFinalLocal))) && !a.es('k', "void") {
		if a.pareceDeclaracion() {
			decl, err := a.tipoConArgumentos()
			if err != nil {
				return nil, false, err
			}
			tipo := decl.crudo()
			if a.actual().kind != 'i' {
				return nil, false, fmt.Errorf("linea %d: la declaracion no tiene nombre", a.actual().donde.linea)
			}
			nombre := a.avanzar().valor
			if a.metodoActual != nil {
				if a.metodoActual.locales == nil {
					a.metodoActual.locales = map[string]string{}
				}
				a.metodoActual.locales[nombre] = tipo
			}
			d := jdecl{tipo: tipo, nombre: nombre, donde: pos{linea: linea}, final: decl.esFinal}
			if a.es('p', "=") {
				a.avanzar()
				e, err := a.expresion()
				if err != nil {
					return nil, false, err
				}
				d.valor = e
			}
			if a.es('p', ",") {
				return nil, false, fmt.Errorf("linea %d: varias variables en una misma declaracion no estan medidas", linea)
			}
			if err := a.comerSimbolo(";"); err != nil {
				return nil, false, err
			}
			return d, true, nil
		}
	}
	e, err := a.expresion()
	if err != nil {
		return nil, false, err
	}
	// una asignacion simple llega ya armada como **expresion** (`jasignExp`), porque tambien vale
	// dentro de parentesis; como sentencia no deja el valor en la pila, asi que se baja al `jasign` de
	// siempre con la linea de la sentencia, que es la que marca javac.
	if asig, esAsig := e.(jasignExp); esAsig {
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jasign{destino: asig.destino, valor: asig.valor, donde: pos{linea: linea}}, true, nil
	}
	if op, esAsig := operadorDeAsignacion(a.actual()); esAsig {
		a.avanzar()
		valor, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return asignacion(e, op, valor, pos{linea: linea}), true, nil
	}
	if err := a.comerSimbolo(";"); err != nil {
		return nil, false, err
	}
	return jstmtExpr{e: e}, true, nil
}

// operadorDeAsignacion: los cinco que acepta este subconjunto. `>>=`, `<<=`, `&=` y los demás no
// están medidos y el tokenizador ni siquiera los arma.
func operadorDeAsignacion(t tokenJava) (string, bool) {
	if t.kind != 'p' {
		return "", false
	}
	switch t.valor {
	case "=", "+=", "-=", "*=", "/=", "<<=", ">>=", ">>>=", "&=", "|=", "^=":
		return t.valor, true
	}
	return "", false
}

// asignacion arma la sentencia. `x = v` es un `jasign`; `x += v` con un literal entero es `jincr`
// cuando el operador es `+` o `-`, porque javac lo baja a `iinc` si el literal cabe en un byte
// (medido en `Mas`: `i += 2` es `iinc 0, 2`, `i -= 3` es `iinc 0, -3`, `i *= 3` es `iload;
// iconst_3; imul; istore`). Lo demás se compone como `x = x op v`, que es exactamente la forma de
// javac para un campo (`getstatic; iconst_2; iadd; putstatic`).
func asignacion(destino jexpr, op string, valor jexpr, donde pos) jstmt {
	if op == "=" {
		return jasign{destino: destino, valor: valor, donde: donde}
	}
	base := strings.TrimSuffix(op, "=")
	if base == "+" || base == "-" {
		if lit, esLit := valor.(jlit); esLit && lit.tipoJ == 'i' {
			delta := lit.entero
			if base == "-" {
				delta = -delta
			}
			return jstmtExpr{e: jincr{destino: destino, delta: delta, donde: donde}}
		}
	}
	return jasign{
		destino: destino,
		valor:   joperador{izq: destino, op: base, der: valor, donde: donde},
		donde:   donde,
	}
}

// leeTipoCualificadoDeclaracion es el interruptor de la sonda del tipo cualificado en una declaración
// local: apagado, `pareceDeclaracion` solo acepta un nombre de tipo suelto y la línea 218 del zip de
// ApkShare —`AlertDialog.Builder builder = new AlertDialog.Builder(this);`— vuelve a fallar con el
// mensaje literal que leyó el usuario. Lo usa la falsificación de
// `javaemisor_tipo_cualificado_local_test.go`; encendido en el binario que se publica.
var leeTipoCualificadoDeclaracion = true

// pareceDeclaracion: `Tipo nombre` (dos identificadores seguidos, con el tipo suelto o cualificado) o
// `final Tipo nombre`.
func (a *analizadorJava) pareceDeclaracion() bool {
	i := a.i
	if a.toks[i].kind == 'k' && a.toks[i].valor == "final" {
		i++
	}
	primer := a.toks[i]
	if primer.kind != 'i' && !(primer.kind == 'k' && esNombreDeTipo(primer.valor)) {
		return false
	}
	i++
	// el tipo cualificado (`AlertDialog.Builder builder`, `android.os.Bundle estado`): los segmentos
	// del tipo van unidos por puntos y el nombre de la variable es el que va detrás del último. Lo que
	// separa la declaración del acceso a un campo es el contexto: en Java `a.b c;` no es ninguna
	// expresión, y un acceso es receptor de una llamada o de una asignación —`System.out.println(…)`
	// trae `(` detrás del último segmento y `builder.setView(x)` no trae un nombre detrás—, así que ni
	// uno ni otro llega a esta rama.
	if leeTipoCualificadoDeclaracion {
		for i+1 < len(a.toks) && a.toks[i].kind == 'p' && a.toks[i].valor == "." &&
			(a.toks[i+1].kind == 'i' || (a.toks[i+1].kind == 'k' && esNombreDeTipo(a.toks[i+1].valor))) {
			i += 2
		}
	}
	// los argumentos de tipo del tipo declarado (`List<Apk> xs`, `Map<String,List<Apk>> xs`). Se cuentan
	// los `<` y los `>`, y el `>>` del tokenizador cierra dos; si no cierran, no es una declaración
	// (`a < b` es una comparación) y se vuelve a la ruta de la expresión.
	if i < len(a.toks) && a.toks[i].kind == 'p' && a.toks[i].valor == "<" {
		hondo := 0
		for ; i < len(a.toks); i++ {
			t := a.toks[i]
			if t.kind != 'p' {
				continue
			}
			switch t.valor {
			case "<":
				hondo++
			case ">":
				hondo--
			case ">>":
				hondo -= 2
			case ">>>":
				hondo -= 3
			}
			if hondo <= 0 {
				i++
				break
			}
		}
		if hondo != 0 {
			return false
		}
	}
	// los corchetes del tipo de arreglo: `int[] v` o `String[][] v`. Solo cuentan los vacios; un
	// `v[i]` no es una declaracion, es un indice, y aqui se distingue por eso.
	for i+1 < len(a.toks) && a.toks[i].kind == 'p' && a.toks[i].valor == "[" &&
		a.toks[i+1].kind == 'p' && a.toks[i+1].valor == "]" {
		i += 2
	}
	if i < len(a.toks) && a.toks[i].kind == 'i' {
		return true
	}
	return false
}

// expresion lee una expresion con su cadena de accesos y llamadas.
func (a *analizadorJava) expresion() (jexpr, error) {
	e, err := a.expresionBinaria(0)
	if err != nil {
		return nil, err
	}
	// `=` como expresion (JLS 15.26.1), la forma de `while ((length = in.read(buffer)) > 0)`: es el
	// operador de menor precedencia y asocia a la derecha, asi que el lado izquierdo ya esta leido y el
	// derecho se lee entero por esta misma ruta (`a = b = c` se anida igual que en javac). Solo el `=`
	// simple: `+=` y los demas compuestos siguen siendo sentencia y el llamador los arma alli. El lado
	// izquierdo tiene que ser una variable —un acceso o un indice—, no una llamada ni un literal.
	if t := a.actual(); t.kind == 'p' && t.valor == "=" {
		switch e.(type) {
		case jacceso, jindice:
		default:
			return nil, fmt.Errorf("linea %d: el lado izquierdo de `=` tiene que ser una variable", t.donde.linea)
		}
		a.avanzar()
		valor, err := a.expresion()
		if err != nil {
			return nil, err
		}
		return jasignExp{destino: e, valor: valor, donde: t.donde}, nil
	}
	// el `?` que sigue a una expresión es el del ternario, que no es de este subconjunto. El de los
	// argumentos de tipo no llega aquí: lo consume `argumentoDeTipo` dentro de los `<>` (§241).
	if t := a.actual(); t.kind == 'p' && t.valor == "?" {
		return nil, fmt.Errorf("linea %d: el ternario no es de este subconjunto", t.donde.linea)
	}
	return e, nil
}

// precedencias: las que usa este subconjunto, medidas con el emisor. El `%` entra porque su `irem` esta
// leido de `javap -c` (parrafo 49), no porque la precedencia se suponga.
// Precedencia de Java de menor a mayor, con huecos para los operadores de bits y el desplazamiento
// medidos en el párrafo 157. Los valores viejos se desplazaron pero no cambió su orden relativo.
var precedenciasJava = map[string]int{
	"||": 1, "&&": 2, "|": 3, "^": 4, "&": 5,
	"==": 6, "!=": 6, "<": 7, ">": 7, "<=": 7, ">=": 7,
	"<<": 8, ">>": 8, ">>>": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "%": 10,
}

func (a *analizadorJava) expresionBinaria(min int) (jexpr, error) {
	izq, err := a.expresionUnaria()
	if err != nil {
		return nil, err
	}
	for {
		t := a.actual()
		// `instanceof` liga como las relacionales (nivel 4) y no es asociativo: se consume aqui y el
		// tipo que sigue va tal cual
		if t.kind == 'k' && t.valor == "instanceof" {
			if precedenciasJava["<"] < min {
				return izq, nil
			}
			a.avanzar()
			tipo, err := a.tipo()
			if err != nil {
				return nil, err
			}
			izq = jinstanceof{dentro: izq, tipo: tipo, donde: t.donde}
			continue
		}
		op := ""
		if t.kind == 'p' {
			op = t.valor
		}
		p, esOp := precedenciasJava[op]
		if !esOp || p < min {
			return izq, nil
		}
		a.avanzar()
		der, err := a.expresionBinaria(p + 1)
		if err != nil {
			return nil, err
		}
		izq = joperador{izq: izq, op: op, der: der, donde: t.donde}
	}
}

func (a *analizadorJava) expresionUnaria() (jexpr, error) {
	if a.es('p', "-") {
		// el menos de un literal lo dobla javac en el fuente (`-1` es `iconst_m1`, `-2.5` una
		// constante negativa del pool, `-'a'` un `bipush -97`); sobre cualquier otra expresión es una
		// negación que se emite en su sitio después de su operando (`iload_1; ineg` para `-total`,
		// medido con javac 17).
		donde := a.actual().donde
		a.avanzar()
		if t := a.actual(); t.kind == 'n' || t.kind == 'c' {
			a.avanzar()
			if t.kind == 'c' {
				// el literal `char` pliega como el entero que es: javac lo escribe `bipush -97`, y el
				// resultado de la cuenta es un `int`, no un `char` (medido con javac 17)
				n, err := enteroJava(t.valor)
				if err != nil {
					return nil, fmt.Errorf("linea %d: %w", donde.linea, err)
				}
				return jlit{tipoJ: 'i', entero: -n, donde: donde}, nil
			}
			if esLiteralFlotante(t.valor) {
				v, tipo, err := flotanteJava(t.valor)
				if err != nil {
					return nil, fmt.Errorf("linea %d: %w", donde.linea, err)
				}
				return jlit{tipoJ: tipo, flotante: -v, donde: donde}, nil
			}
			if esLiteralLargo(t.valor) {
				n, err := enteroLargoJava(t.valor)
				if err != nil {
					return nil, fmt.Errorf("linea %d: %w", donde.linea, err)
				}
				return jlit{tipoJ: 'j', largo: -n, donde: donde}, nil
			}
			n, err := enteroJava(t.valor)
			if err != nil {
				return nil, fmt.Errorf("linea %d: %w", donde.linea, err)
			}
			return jlit{tipoJ: 'i', entero: -n, donde: donde}, nil
		}
		e, err := a.expresionUnaria()
		if err != nil {
			return nil, err
		}
		return jnegativo{dentro: e, donde: donde}, nil
	}
	if a.es('p', "++") || a.es('p', "--") {
		donde := a.actual().donde
		delta := int32(1)
		if a.actual().valor == "--" {
			delta = -1
		}
		a.avanzar()
		e, err := a.expresionUnaria()
		if err != nil {
			return nil, err
		}
		return jincr{destino: e, delta: delta, prefijo: true, donde: donde}, nil
	}
	if a.es('p', "!") {
		donde := a.actual().donde
		a.avanzar()
		e, err := a.expresionUnaria()
		if err != nil {
			return nil, err
		}
		return jnegacion{dentro: e, donde: donde}, nil
	}
	if a.es('p', "(") {
		donde := a.actual().donde
		marca := a.i
		a.avanzar()
		tipo, err := a.tipo()
		if err == nil && a.es('p', ")") && puedeAbrirExpresion(a.mira(1)) {
			a.avanzar()
			e, err := a.expresionUnaria()
			if err != nil {
				return nil, err
			}
			return jcast{tipo: tipo, dentro: e, donde: donde}, nil
		}
		// no era un molde: una expresion entre parentesis, que solo cambia la precedencia y deja el
		// mismo arbol. Se deshace lo leido y se vuelve por la ruta de la expresion. Medido en el
		// parrafo 53 con `"s" + (x + 1)`, donde javac escribe la cuenta entera y un solo append(I).
		a.i = marca
		a.avanzar() // el (
		e, err := a.expresion()
		if err != nil {
			return nil, err
		}
		if err := a.comerSimbolo(")"); err != nil {
			return nil, err
		}
		return e, nil
	}
	e, err := a.atomo()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case a.es('p', "."):
			a.avanzar()
			t := a.actual()
			if t.kind != 'i' {
				return nil, fmt.Errorf("linea %d: tras el punto se esperaba un nombre y hay %q", t.donde.linea, t.valor)
			}
			a.avanzar()
			if a.es('p', "(") {
				args, err := a.argumentos()
				if err != nil {
					return nil, err
				}
				e = jllamada{base: e, nombre: t.valor, args: args, donde: t.donde}
				continue
			}
			e = jacceso{base: e, nombre: t.valor, donde: t.donde}
		case a.es('p', "["):
			donde := a.actual().donde
			a.avanzar()
			indice, err := a.expresion()
			if err != nil {
				return nil, err
			}
			if err := a.comerSimbolo("]"); err != nil {
				return nil, err
			}
			e = jindice{base: e, indice: indice, donde: donde}
		case a.es('p', "++") || a.es('p', "--"):
			donde := a.actual().donde
			delta := int32(1)
			if a.actual().valor == "--" {
				delta = -1
			}
			a.avanzar()
			e = jincr{destino: e, delta: delta, donde: donde}
		case a.es('p', "("):
			args, err := a.argumentos()
			if err != nil {
				return nil, err
			}
			base := jexpr(nil)
			if acc, esAcceso := e.(jacceso); esAcceso {
				base = acc.base
				e = jllamada{base: base, nombre: acc.nombre, args: args, donde: acc.donde}
				continue
			}
			return nil, fmt.Errorf("linea %d: solo se mide llamar a un metodo por nombre o por acceso", a.actual().donde.linea)
		default:
			return e, nil
		}
	}
}

// argumentos lee `(a, b, c)`.
func (a *analizadorJava) argumentos() ([]jexpr, error) {
	if err := a.comerSimbolo("("); err != nil {
		return nil, err
	}
	out := []jexpr{}
	if a.es('p', ")") {
		a.avanzar()
		return out, nil
	}
	for {
		e, err := a.expresion()
		if err != nil {
			return nil, err
		}
		out = append(out, e)
		if !a.es('p', ",") {
			break
		}
		a.avanzar()
	}
	if err := a.comerSimbolo(")"); err != nil {
		return nil, err
	}
	return out, nil
}

// literalDeClase reconoce `X.class`, `int.class` y `String[].class` antes de que el nombre se lea como
// un acceso. Medido en el párrafo 89: el de un tipo de referencia (o un arreglo) es `ldc` de una
// CONSTANT_Class; el de un primitivo suelto es el `getstatic` del `TYPE` de su envoltorio.
func (a *analizadorJava) literalDeClase() (jexpr, bool, error) {
	t := a.actual()
	if t.kind != 'i' && !(t.kind == 'k' && esNombreDeTipo(t.valor)) {
		return nil, false, nil
	}
	n := 1
	for a.mira(n).kind == 'p' && a.mira(n).valor == "[" &&
		a.mira(n+1).kind == 'p' && a.mira(n+1).valor == "]" {
		n += 2
	}
	if !(a.mira(n).kind == 'p' && a.mira(n).valor == "." &&
		a.mira(n+1).kind == 'k' && a.mira(n+1).valor == "class") {
		return nil, false, nil
	}
	a.avanzar() // el nombre
	dimensiones := 0
	for a.es('p', "[") {
		a.avanzar()
		if err := a.comerSimbolo("]"); err != nil {
			return nil, false, err
		}
		dimensiones++
	}
	a.avanzar() // el punto
	a.avanzar() // `class`
	return jclase{nombre: t.valor, dimensiones: dimensiones, primitivo: t.kind == 'k', donde: t.donde}, true, nil
}

// atomo lee un literal, this, super, un nombre o un `new`.
func (a *analizadorJava) atomo() (jexpr, error) {
	if c, ok, err := a.literalDeClase(); err != nil {
		return nil, err
	} else if ok {
		return c, nil
	}
	t := a.actual()
	switch {
	case t.kind == 's':
		a.avanzar()
		return jlit{tipoJ: 's', texto: t.valor, donde: t.donde}, nil
	case t.kind == 'n':
		a.avanzar()
		if esLiteralFlotante(t.valor) {
			v, tipo, err := flotanteJava(t.valor)
			if err != nil {
				return nil, fmt.Errorf("linea %d: %w", t.donde.linea, err)
			}
			return jlit{tipoJ: tipo, flotante: v, donde: t.donde}, nil
		}
		if esLiteralLargo(t.valor) {
			n, err := enteroLargoJava(t.valor)
			if err != nil {
				return nil, fmt.Errorf("linea %d: %w", t.donde.linea, err)
			}
			return jlit{tipoJ: 'j', largo: n, donde: t.donde}, nil
		}
		n, err := enteroJava(t.valor)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", t.donde.linea, err)
		}
		return jlit{tipoJ: 'i', entero: n, donde: t.donde}, nil
	case t.kind == 'c':
		// el literal `char` ya leído como punto de código por el léxico (parrafo 87)
		a.avanzar()
		n, err := enteroJava(t.valor)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", t.donde.linea, err)
		}
		return jlit{tipoJ: 'c', entero: n, donde: t.donde}, nil
	case a.es('k', "true"):
		a.avanzar()
		return jlit{tipoJ: 'b', booleano: true, donde: t.donde}, nil
	case a.es('k', "false"):
		a.avanzar()
		return jlit{tipoJ: 'b', booleano: false, donde: t.donde}, nil
	case a.es('k', "null"):
		a.avanzar()
		return jlit{tipoJ: 'n', donde: t.donde}, nil
	case a.es('k', "this"):
		a.avanzar()
		if a.es('p', "(") {
			// `this(...)`: delegacion a otro constructor de la misma clase (parrafo 91)
			args, err := a.argumentos()
			if err != nil {
				return nil, err
			}
			return jllamada{constructorPropio: true, nombre: "<init>", args: args, donde: t.donde}, nil
		}
		if a.es('p', ".") && a.i+1 < len(a.toks) && a.toks[a.i+1].kind == 'i' {
			// `this.x`: acceso al campo propio
			a.avanzar()
			nombre := a.avanzar().valor
			return jacceso{base: jthis{donde: t.donde}, nombre: nombre, donde: t.donde}, nil
		}
		return jthis{donde: t.donde}, nil
	case a.es('k', "super"):
		a.avanzar()
		if a.es('p', "(") {
			// `super(...)`: la llamada al constructor del padre, primera sentencia de un constructor
			args, err := a.argumentos()
			if err != nil {
				return nil, err
			}
			return jllamada{super: true, nombre: "<init>", args: args, donde: t.donde}, nil
		}
		if !a.es('p', ".") {
			return nil, fmt.Errorf("linea %d: `super` solo se usa para llamar a un metodo del padre", t.donde.linea)
		}
		a.avanzar()
		nombre := a.avanzar().valor
		if !a.es('p', "(") {
			return nil, fmt.Errorf("linea %d: `super.%s` sin llamada no esta medida", t.donde.linea, nombre)
		}
		args, err := a.argumentos()
		if err != nil {
			return nil, err
		}
		return jllamada{super: true, nombre: nombre, args: args, donde: t.donde}, nil
	case a.es('k', "new"):
		a.avanzar()
		tipo, err := a.nombrePunto()
		if err != nil {
			return nil, err
		}
		if err := a.argumentosDeTipoDelNuevo(); err != nil {
			return nil, err
		}
		if a.es('p', "[") {
			a.avanzar()
			if a.es('p', "]") {
				a.avanzar()
				if !a.es('p', "{") {
					return nil, fmt.Errorf("linea %d: `new %s[]` sin inicializador no es Java; escriba `new %s[n]` o `new %s[] {…}`", t.donde.linea, tipo, tipo, tipo)
				}
				a.avanzar() // el {
				elementos := []jexpr{}
				for !a.es('p', "}") {
					elem, err := a.expresion()
					if err != nil {
						return nil, err
					}
					elementos = append(elementos, elem)
					if !a.es('p', ",") {
						break
					}
					a.avanzar()
				}
				if err := a.comerSimbolo("}"); err != nil {
					return nil, err
				}
				if a.es('p', "[") || a.es('p', "{") {
					return nil, fmt.Errorf("linea %d: un arreglo de mas de una dimension no esta medido", t.donde.linea)
				}
				return jnuevoArreglo{tipo: tipo, elementos: elementos, donde: t.donde}, nil
			}
			dims := []jexpr{}
			for {
				d, err := a.expresion()
				if err != nil {
					return nil, err
				}
				if err := a.comerSimbolo("]"); err != nil {
					return nil, err
				}
				dims = append(dims, d)
				if !a.es('p', "[") {
					break
				}
				a.avanzar()
			}
			if a.es('p', "{") {
				return nil, fmt.Errorf("linea %d: un arreglo con inicializador y más de una dimensión no está medido", t.donde.linea)
			}
			if len(dims) == 1 {
				return jnuevoArreglo{tipo: tipo, largo: dims[0], donde: t.donde}, nil
			}
			return jnuevoArreglo{tipo: tipo, dimensiones: dims, donde: t.donde}, nil
		}
		if !a.es('p', "(") {
			return nil, fmt.Errorf("linea %d: un `new %s` sin constructor no esta medido", t.donde.linea, tipo)
		}
		args, err := a.argumentos()
		if err != nil {
			return nil, err
		}
		cuerpo := (*jclas)(nil)
		if a.es('p', "{") {
			// la clase anonima: javac la emite como Tipo$N y aqui se nombra igual, con el indice
			// que le toca en el orden de aparicion (parrafo 41)
			a.anonima++
			cuerpo, err = a.claseAnonima(tipo)
			if err != nil {
				return nil, err
			}
		}
		return jnuevo{tipo: tipo, args: args, cuerpo: cuerpo, donde: t.donde}, nil
	}
	if t.kind == 'i' {
		a.avanzar()
		nombre := t.valor
		if a.es('p', "->") {
			// `x -> expr`: la lambda de un parametro sin tipo. El cuerpo es una expresion entera, que
			// termina donde termina el argumento que la lleva (la coma o el parentesis que cierra).
			a.avanzar()
			if a.es('p', "{") {
				return nil, fmt.Errorf("linea %d: una lambda con cuerpo de bloque no esta medida; escriba su cuerpo como una expresion", t.donde.linea)
			}
			cuerpo, err := a.expresion()
			if err != nil {
				return nil, err
			}
			return jlambda{param: nombre, cuerpo: cuerpo, donde: t.donde}, nil
		}
		if !a.es('p', ".") && a.registraCaptura(nombre) {
			// una local/parámetro effectively-final del método que crea la anónima: javac la baja a
			// `val$nombre` y el emisor la lee como `this.val$nombre` (5.14)
			return jacceso{base: jthis{donde: t.donde}, nombre: "val$" + nombre, donde: t.donde}, nil
		}
		if a.es('p', ".") {
			// R.layout.main: se lee como acceso y el emisor decide si es una constante inlinable.
			// MainActivity.this: la captura del exterior, que javac baja a un campo this$0.
			// La cadena entera lleva la posicion de su **cabeza**, que es donde empieza el token que el
			// editor tiene que subrayar: `R.layout.main` se señala en la `R`, no en `main` (§200).
			inicio := t.donde
			base := jexpr(jacceso{nombre: nombre, donde: inicio})
			for a.es('p', ".") {
				a.avanzar()
				if a.es('k', "this") {
					a.avanzar()
					if acc, esAcceso := base.(jacceso); esAcceso && acc.base == nil {
						return jthis{nombre: acc.nombre, donde: inicio}, nil
					}
					return nil, fmt.Errorf("%s: `.this` solo se mide detras de un nombre de clase suelto", inicio.sitio())
				}
				siguiente := a.actual()
				if siguiente.kind != 'i' {
					return nil, fmt.Errorf("%s: tras el punto se esperaba un nombre", siguiente.donde.sitio())
				}
				a.avanzar()
				base = jacceso{base: base, nombre: siguiente.valor, donde: inicio}
			}
			return base, nil
		}
		return jacceso{nombre: nombre, donde: t.donde}, nil
	}
	if t.kind == 'k' && esNombreDeTipo(t.valor) {
		return nil, fmt.Errorf("linea %d: el tipo %q no va suelto en una expresion", t.donde.linea, t.valor)
	}
	if t.kind == 'p' && (t.valor == "+" || t.valor == "-") {
		// el caso real es `i++` dentro de un `for`: se nombra para que el portero diga que falta (5.1)
		return nil, fmt.Errorf("linea %d: un `++`/`--` no esta medido en este compilador", t.donde.linea)
	}
	return nil, fmt.Errorf("linea %d: %q no es una expresion que este parser reconozca", t.donde.linea, t.valor)
}

// registraCaptura: si `nombre` es una local/parámetro del método que crea la anónima y no una
// local/parámetro/campo de la propia anónima, la añade a `capturas` y a los campos `val$nombre`.
func (a *analizadorJava) registraCaptura(nombre string) bool {
	c := a.claseActual
	if c == nil || !c.anonima || c.metodoEncierre == nil {
		return false
	}
	if a.metodoActual != nil {
		if _, ok := a.metodoActual.locales[nombre]; ok {
			return false
		}
	}
	for _, f := range c.campos {
		if f.nombre == nombre {
			return false
		}
	}
	tipo, ok := c.metodoEncierre.locales[nombre]
	if !ok {
		return false
	}
	for _, cap := range c.capturas {
		if cap.nombre == nombre {
			return true
		}
	}
	c.capturas = append(c.capturas, captura{nombre: nombre, tipo: tipo})
	c.campos = append(c.campos, jcampo{
		nombre: "val$" + nombre, tipo: tipo, esFinal: true, esSintetico: true,
		donde: pos{linea: c.linea},
	})
	return true
}

// claseAnonima lee el cuerpo de un `new Interface() { ... }`. Lo que no sea implementar metodos no esta
// medido: javac le pone el campo this$0 y el constructor con la captura, y eso se comprueba aparte.
func (a *analizadorJava) claseAnonima(nombre string) (*jclas, error) {
	if a.claseActual == nil {
		return nil, fmt.Errorf("linea %d: una clase anonima fuera de una declaracion de clase no es Java", a.actual().donde.linea)
	}
	nombreAnonima := a.exterior + "$" + strconv.Itoa(a.anonima)
	if a.claseActual.anonima {
		// una anonima dentro de otra lleva el nombre de la que la crea y su propio contador:
		// Capa$1$1, no Capa$2 (medido con javac 17 --release 8 sobre Capa.java)
		nombreAnonima = a.claseActual.nombre + "$" + strconv.Itoa(a.claseActual.siguienteAnonima+1)
		a.claseActual.siguienteAnonima++
	} else if a.claseActual.nombre != a.exterior {
		return nil, fmt.Errorf("linea %d: una anonima creada dentro de una clase anidada no esta medida (javac la nombra Exterior$Anidada$1)", a.actual().donde.linea)
	}
	c := &jclas{
		nombre:         nombreAnonima,
		implementa:     []string{nombre},
		paquete:        a.paquete,
		linea:          a.actual().donde.linea,
		anonima:        true,
		encierre:       a.claseActual,
		metodoEncierre: a.metodoActual,
	}
	if a.metodoActual == nil {
		return nil, fmt.Errorf("linea %d: una anonima creada fuera de un metodo (en un campo o un inicializador) no esta medida: falta el EnclosingMethod", c.linea)
	}
	if err := a.comerSimbolo("{"); err != nil {
		return nil, err
	}
	// dentro de la anonima `a.claseActual` es la anonima: sin esto `registraCaptura` miraba la clase
	// exterior y no veia las locales del metodo que la crea (5.14)
	exteriorActual := a.claseActual
	a.claseActual = c
	defer func() { a.claseActual = exteriorActual }()
	for !a.es('p', "}") {
		linea := a.actual().donde.linea
		miembro, err := a.miembroDeClase(c, linea)
		if err != nil {
			return nil, err
		}
		switch v := miembro.(type) {
		case jmetodo:
			c.metodos = append(c.metodos, v)
		case jcampo:
			c.campos = append(c.campos, v)
		case nil:
		default:
			return nil, fmt.Errorf("linea %d: dentro de una clase anonima solo se miden metodos", linea)
		}
	}
	a.avanzar()
	return c, nil
}

// joperador y jnegacion: las expresiones binarias y la negacion, que el emisor baja a if_icmpne / ifne.
type joperador struct {
	izq, der jexpr
	op       string
	donde    pos
}

func (joperador) jexprPuede() {}

type jnegacion struct {
	dentro jexpr
	donde  pos
}

func (jnegacion) jexprPuede() {}

// jnegativo: la `-` unaria de la aritmética. Va aparte de `jnegacion` (que es la `!` booleana y se
// branquea con `ifeq`/`ifne`) porque se baja a `ineg`/`lneg`/`fneg`/`dneg` **después** de emitir el
// operando. Los literales numéricos siguen doblándose en el analizador (`-1` es `iconst_m1`) y no
// llegan aquí; esto es la negación de todo lo demás (`-total`, `-(a+b)`, `-f()`).
type jnegativo struct {
	dentro jexpr
	donde  pos
}

func (jnegativo) jexprPuede() {}

// sinPaquete quita el paquete a un nombre de clase interno (com/miapp/R$layout -> R$layout).
func sinPaquete(nombre string) string {
	if i := strings.LastIndex(nombre, "/"); i >= 0 {
		nombre = nombre[i+1:]
	}
	if i := strings.LastIndex(nombre, "."); i >= 0 {
		nombre = nombre[i+1:]
	}
	return nombre
}

// esLiteralLargo y enteroLargoJava: el sufijo `L`/`l` que el lexico ya se llevo dentro del token
// numerico. Medido con `javap -c` sobre la sonda `L` (parrafo 74): `2L` sale como `ldc2_w #7` con una
// entrada `Long` del pool, y `1L` como `lconst_1`.
func esLiteralLargo(texto string) bool {
	if texto == "" {
		return false
	}
	c := texto[len(texto)-1]
	return c == 'L' || c == 'l'
}

func enteroLargoJava(texto string) (int64, error) {
	limpio := strings.ReplaceAll(texto, "_", "")
	limpio = limpio[:len(limpio)-1]
	base := 10
	if strings.HasPrefix(limpio, "0x") || strings.HasPrefix(limpio, "0X") {
		base = 16
		limpio = limpio[2:]
	}
	n, err := strconv.ParseInt(limpio, base, 64)
	if err != nil {
		return 0, fmt.Errorf("%s no es un entero de 64 bits", texto)
	}
	return n, nil
}

// esLiteralFlotante: el token numerico lleva punto o exponente, o el sufijo `f`/`d`. Medido con
// `javap -c` sobre la sonda `FyD` (parrafo 77): `3.14159` sale `ldc2_w` con una entrada `Double` del
// pool, `1.5f` sale `ldc` con una entrada `Float` (una sola posicion) y `0.0f` es `fconst_0`.
func esLiteralFlotante(texto string) bool {
	if texto == "" {
		return false
	}
	if strings.HasPrefix(texto, "0x") || strings.HasPrefix(texto, "0X") {
		// los hexos con `p` de exponente no estan medidos
		return strings.ContainsAny(texto, ".pP")
	}
	if strings.ContainsAny(texto, ".") {
		return true
	}
	if i := strings.LastIndexAny(texto, "fFdD"); i >= 0 {
		return true
	}
	return strings.ContainsAny(texto, "eE")
}

// flotanteJava: el valor y el tipo del literal. Con `f` el tipo es `float` (32 bits, entrada `Float`
// del pool y `ldc`); sin sufijo o con `d` es `double` (64 bits, entrada `Double` con dos posiciones y
// `ldc2_w`).
func flotanteJava(texto string) (float64, byte, error) {
	limpio := strings.ReplaceAll(texto, "_", "")
	tipo := byte('d')
	if c := limpio[len(limpio)-1]; c == 'f' || c == 'F' {
		tipo = 'f'
		limpio = limpio[:len(limpio)-1]
	} else if c == 'd' || c == 'D' {
		limpio = limpio[:len(limpio)-1]
	}
	if strings.HasPrefix(limpio, "0x") || strings.HasPrefix(limpio, "0X") {
		return 0, 0, fmt.Errorf("un literal flotante hexadecimal (%s) no esta medido", texto)
	}
	anchura := 64
	if tipo == 'f' {
		anchura = 32
	}
	v, err := strconv.ParseFloat(limpio, anchura)
	if err != nil {
		return 0, 0, fmt.Errorf("%s no es un literal flotante", texto)
	}
	return v, tipo, nil
}

// enteroJava: los literales numericos de 32 bits del subconjunto; el largo va por enteroLargoJava.
func enteroJava(texto string) (int32, error) {
	limpio := strings.ReplaceAll(texto, "_", "")
	if strings.HasSuffix(limpio, "L") || strings.HasSuffix(limpio, "l") {
		return 0, fmt.Errorf("%s es un literal long, que se lee por otro camino", texto)
	}
	base := 10
	if strings.HasPrefix(limpio, "0x") || strings.HasPrefix(limpio, "0X") {
		base = 16
		limpio = limpio[2:]
	}
	n, err := strconv.ParseInt(limpio, base, 64)
	if err != nil {
		return 0, fmt.Errorf("%s no es un entero de 32 bits", texto)
	}
	return int32(n), nil
}
