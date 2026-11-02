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
}

// jexpr es una expresion del subconjunto.
type jexpr interface{ jexprPuede() }

// jlit es un literal: cadena, entero, booleano o nulo.
type jlit struct {
	tipoJ    byte // 's' cadena, 'i' entero, 'b' booleano, 'n' nulo
	texto    string
	entero   int32
	booleano bool
	donde    pos
}

func (jlit) jexprPuede() {}

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
	base   jexpr
	super  bool
	nombre string
	args   []jexpr
	donde  pos
}

func (jllamada) jexprPuede() {}

// jcast: un molde entre parentesis. Solo se acepta sobre un tipo que el emisor conoce.
type jcast struct {
	tipo   string
	dentro jexpr
	donde  pos
}

func (jcast) jexprPuede() {}

// jnuevo: `new Tipo(args)` y, si cuerpo no es nil, la clase anonima `new Tipo(args) { ... }`.
type jnuevo struct {
	tipo   string
	args   []jexpr
	cuerpo *jclas
	donde  pos
}

func (jnuevo) jexprPuede() {}

// jnuevoArreglo: `new Tipo[n]`. El inicializador `new Tipo[] {…}` no esta medido (pide
// `filled-new-array`, que todavia no se ha medido) y se rechaza por nombre al leerlo.
type jnuevoArreglo struct {
	tipo  string
	largo jexpr
	donde pos
}

func (jnuevoArreglo) jexprPuede() {}

// jindice: `v[i]`, para leer y para asignar (el destino de un `jasign` puede ser uno).
type jindice struct {
	base   jexpr
	indice jexpr
	donde  pos
}

func (jindice) jexprPuede() {}

// jincr: `i++`, `--i` y el paso de un `for`. Como sentencia se emite `iinc` en un local —medido en
// el parrafo 67— o la lectura-cuenta-escritura de un campo; como **valor** no esta medido y el emisor
// lo rechaza por nombre.
type jincr struct {
	destino jexpr
	delta   int32
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

// jswitch: `switch (selector) { case k: cuerpo break; … default: cuerpo }`. El selector tiene que ser
// un entero y las claves literales enteros; cada caso **tiene que terminar en `break` o en `return`**
// (la caída de un caso al siguiente no está medida y se rechaza por nombre). Se midió en el §64 qué
// forma elige javac; el emisor usa la suya, escrita en `cambio`.
type jswitch struct {
	selector jexpr
	casos    []jcaso
	defecto  []jstmt
	donde    pos
}

func (jswitch) jstmtPuede() {}

type jcaso struct {
	valor  int32
	cuerpo []jstmt
	rompe  bool // terminó en `break;`, así que el emisor escribe el salto al final del switch
	donde  pos
}

// jparam, jcampo y jmetodo son las partes declarativas.
type jparam struct {
	tipo   string
	nombre string
	donde  pos
}

type jcampo struct {
	nombre      string
	tipo        string
	esEstatico  bool
	esFinal     bool
	esPublico   bool
	esPrivado   bool
	esProtegido bool
	valor       jexpr
	donde       pos
}

type jmetodo struct {
	nombre     string
	retorno    string // "" si constructor
	params     []jparam
	cuerpo     []jstmt
	esEstatico bool
	esPublico  bool
	esFinal    bool
	linea      int
	// lineaFin es la llave que cierra el cuerpo: javac anota ahi el `return` implicito de un metodo void,
	// medido en el parrafo 40 (line 16 en la plantilla, que es donde esta el `}` de onCreate).
	lineaFin int
}

// jclas es una clase, anidada o no. interior guarda las anidadas en el orden del fuente, y nombreExterno
// el prefijo con el que javac las nombra en el binario (R y R$layout).
type jclas struct {
	nombre        string
	super         string
	implementa    []string
	campos        []jcampo
	metodos       []jmetodo
	interior      []*jclas
	abstracta     bool
	final         bool
	esPublica     bool
	esEstatica    bool // solo cambia los flags del InnerClasses: en su propio ClassFile una anidada no es ACC_STATIC
	linea         int
	paquete       string
	imports       []string
	nombreExterno string

	// anonima: la clase que crea un `new Tipo(args) { ... }`. javac le pone el campo this$0, un constructor
	// con la captura y el atributo EnclosingMethod, y en el InnerClasses su fila va sin nombre ni exterior
	// (parrafo 42). encierre y metodoEncierre son de donde salio, y se guardan punteros porque el descriptor
	// del exterior se calcula despues, en el emisor.
	anonima        bool
	encierre       *jclas
	metodoEncierre *jmetodo
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
	"char": true, "switch": true, "case": true, "default": true, "break": true, "continue": true, "throw": true, "native": true,
	"synchronized": true, "volatile": true, "transient": true, "instanceof": true, "do": true,
}

// simbolesJava, de mas largo a mas corto para que `>>` no se lea como dos `>`.
var simbolesJava = []string{
	"...", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", ">>", "<<", "++", "--",
	"{", "}", "(", ")", "[", "]", ";", ",", ".", "=", ">", "<", "!", "+", "-", "*", "/", "&", "|", "^", ":", "@", "%",
}

// tokenizarJava parte el fuente. Hay una garantia en el final del bucle: si una vuelta no consumio
// bytes, se rechaza. Sin esa garantia un byte multibyte que no es letra ASCII (un `¥`, que es un
// simbolo monetario y no una letra) hacia append de un identificador vacio sin avanzar y el bucle
// exterior no paraba nunca: memoria hasta que el kernel mataba el proceso.
func tokenizarJava(fuente string) ([]tokenJava, error) {
	out := []tokenJava{}
	linea := 1
	i := 0
	for i < len(fuente) {
		antes := i
		c := fuente[i]
		switch {
		case c == '\n':
			linea++
			i++
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
			out = append(out, tokenJava{kind: 's', valor: texto, donde: pos{linea}})
			i = nuevo
		case c == '\'':
			return nil, fmt.Errorf("linea %d: un literal de char no esta medido en este compilador", linea)
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
			out = append(out, tokenJava{kind: kind, valor: palabra, donde: pos{linea}})
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
			out = append(out, tokenJava{kind: 'n', valor: fuente[desde:i], donde: pos{linea}})
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
			out = append(out, tokenJava{kind: 'p', valor: encontrado, donde: pos{linea}})
			i += len(encontrado)
		}
		if i == antes {
			// la garantia de que este bucle avanza: si una vuelta no consumio nada, esto era un bucle
			// infinito comiéndose memoria
			r, _ := utf8.DecodeRuneInString(fuente[i:])
			return nil, fmt.Errorf("linea %d: el caracter %q no es de este subconjunto", linea, r)
		}
	}
	out = append(out, tokenJava{kind: 'f', donde: pos{linea}})
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
	for a.actual().kind == 'k' || a.actual().kind == 'i' {
		c, err := a.clase(nil)
		if err != nil {
			return nil, err
		}
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
	case "int", "boolean", "long", "float", "double", "byte", "short", "char", "void":
		return true
	}
	return false
}

// clase lee una declaracion de clase, con sus anidadas.
func (a *analizadorJava) clase(exteriores *jclas) (*jclas, error) {
	c := &jclas{paquete: a.paquete, linea: a.actual().donde.linea}
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
		case a.es('k', "interface"):
			return nil, fmt.Errorf("linea %d: una `interface` escrita a mano no esta medida (la del source generado si: se implementa con una clase anonima, parrafo 41)", t.donde.linea)
		case a.es('p', "@"):
			if err := a.anotacion(); err != nil {
				return nil, err
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
	if a.es('k', "extends") {
		a.avanzar()
		nombre, err := a.nombrePunto()
		if err != nil {
			return nil, err
		}
		c.super = nombre
	}
	if a.es('k', "implements") {
		a.avanzar()
		for {
			nombre, err := a.nombrePunto()
			if err != nil {
				return nil, err
			}
			c.implementa = append(c.implementa, nombre)
			if !a.es('p', ",") {
				break
			}
			a.avanzar()
		}
	}
	if err := a.comerSimbolo("{"); err != nil {
		return nil, err
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
			if err := a.anotacion(); err != nil {
				return nil, err
			}
			continue
		case a.es('k', "class"), a.es('k', "interface"):
			if esAbstracta {
				return nil, fmt.Errorf("linea %d: una clase anidada abstracta no esta medida", linea)
			}
			anidada, err := a.clase(c)
			if anidada != nil {
				// los modificadores los consumio este bucle, no la declaracion: sin esto la anidada perderia
				// el `public static final` que javac deja en su ClassFile (0x0031) y en el InnerClasses
				anidada.esPublica = anidada.esPublica || esPublico
				anidada.esEstatica = anidada.esEstatica || esEstatico
				anidada.final = anidada.final || esFinal
			}
			return anidada, err
		default:
			goto cuerpo
		}
		a.avanzar()
	}
cuerpo:
	_ = esAbstracta
	// `private` y `protected` se midieron en los flags de un campo (0x0002 y 0x0004, parrafo 49); en un
	// metodo todavia no se leyeron, así que se rechazan aqui y no en el emisor, que es mas lejos del fuente
	leerMetodo := func(retorno string) (interface{}, error) {
		if esPrivado || esProtegido {
			modificador := "private"
			if esProtegido {
				modificador = "protected"
			}
			return nil, fmt.Errorf("linea %d: un metodo %s no esta medido contra javap: los unicos flags leidos son los de un campo",
				linea, modificador)
		}
		return a.metodo(c, retorno, esEstatico, esFinal, esPublico, linea)
	}
	if a.es('k', "void") {
		a.avanzar()
		return leerMetodo("void")
	}
	// un constructor: `Nombre(`
	if a.actual().kind == 'i' && a.actual().valor == sinPaquete(c.nombre) && a.pegaParentesis() {
		a.avanzar()
		return leerMetodo(c.nombre)
	}
	tipo, err := a.tipo()
	if err != nil {
		return nil, err
	}
	if a.actual().kind != 'i' {
		return nil, fmt.Errorf("linea %d: se esperaba un nombre de miembro y hay %q", a.actual().donde.linea, a.actual().valor)
	}
	if a.pegaParentesis() {
		return leerMetodo(tipo)
	}
	nombre := a.avanzar().valor
	campo := jcampo{nombre: nombre, tipo: tipo, esEstatico: esEstatico, esFinal: esFinal, esPublico: esPublico,
		esPrivado: esPrivado, esProtegido: esProtegido, donde: pos{linea}}
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
func (a *analizadorJava) metodo(c *jclas, retorno string, esEstatico, esFinal, esPublico bool, linea int) (jmetodo, error) {
	m := jmetodo{retorno: retorno, esEstatico: esEstatico, esFinal: esFinal, esPublico: esPublico, linea: linea}
	if retorno == c.nombre {
		m.nombre = "<init>"
		a.avanzar() // el nombre, que ya sabemos cual es
	} else {
		m.nombre = a.avanzar().valor
	}
	if err := a.comerSimbolo("("); err != nil {
		return m, err
	}
	if !a.es('p', ")") {
		for {
			tipo, err := a.tipo()
			if err != nil {
				return m, err
			}
			if a.actual().kind != 'i' {
				return m, fmt.Errorf("linea %d: un parametro necesita nombre", a.actual().donde.linea)
			}
			m.params = append(m.params, jparam{tipo: tipo, nombre: a.avanzar().valor, donde: pos{linea}})
			if !a.es('p', ",") {
				break
			}
			a.avanzar()
		}
	}
	if err := a.comerSimbolo(")"); err != nil {
		return m, err
	}
	if a.es('k', "throws") {
		return m, fmt.Errorf("linea %d: un `throws` no esta medido: cambia el atributo Exceptions del method_info", a.actual().donde.linea)
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

// anotacion consume `@Nombre(...)` o `@Nombre`. Solo se tolera @Override, y por un motivo medido: el
// javap -v del parrafo 40 sobre la plantilla no muestra ningun atributo de anotaciones en la clase ni en
// sus metodos (attributes: 2, SourceFile e InnerClasses), o sea que esa anotacion no deja huella en el
// .class. Las demas si la dejan, en un atributo que este emisor todavia no escribe: se rechazan por
// nombre en vez de tirarlas.
func (a *analizadorJava) anotacion() error {
	linea := a.actual().donde.linea
	a.avanzar() // @
	nombre := ""
	if a.actual().kind == 'i' {
		nombre, _ = a.nombrePunto()
	}
	if a.es('p', "(") {
		profundidad := 0
		for {
			t := a.actual()
			if t.kind == 'f' {
				break
			}
			if t.valor == "(" {
				profundidad++
			}
			if t.valor == ")" {
				profundidad--
			}
			a.avanzar()
			if profundidad == 0 {
				break
			}
		}
	}
	if nombre == "Override" || nombre == "java.lang.Override" {
		return nil
	}
	return fmt.Errorf("linea %d: la anotacion @%s deja huella en el .class y este emisor no la escribe; solo se tolera @Override (parrafo 40)", linea, nombre)
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
	cam := jswitch{selector: sel, donde: pos{linea}}
	vistas := map[int32]bool{}
	for !a.es('p', "}") {
		esDefecto := false
		var valor int32
		switch {
		case a.es('k', "case"):
			a.avanzar()
			e, err := a.expresion()
			if err != nil {
				return nil, false, err
			}
			lit, esLit := e.(jlit)
			if !esLit || lit.tipoJ != 'i' {
				return nil, false, fmt.Errorf("linea %d: una clave de `case` que no es un literal entero no esta medida", a.actual().donde.linea)
			}
			valor = lit.entero
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
		if !terminado && len(cuerpo) > 0 {
			if _, esRetorno := cuerpo[len(cuerpo)-1].(jretorno); esRetorno {
				terminado = true
			}
		}
		if !terminado {
			return nil, false, fmt.Errorf("linea %d: un `case` que no termina en `break` ni en `return` no esta medido (la caida al siguiente tampoco)", linea)
		}
		if esDefecto {
			cam.defecto = cuerpo
			continue
		}
		if vistas[valor] {
			return nil, false, fmt.Errorf("linea %d: la clave %d aparece dos veces en el switch", linea, valor)
		}
		vistas[valor] = true
		cam.casos = append(cam.casos, jcaso{valor: valor, cuerpo: cuerpo, rompe: rompe, donde: pos{linea}})
	}
	if err := a.comerSimbolo("}"); err != nil {
		return nil, false, err
	}
	if len(cam.casos) == 0 {
		return nil, false, fmt.Errorf("linea %d: un `switch` sin ningun `case` no esta medido", linea)
	}
	return cam, true, nil
}

func (a *analizadorJava) sentencia() (jstmt, bool, error) {
	linea := a.actual().donde.linea
	switch {
	case a.es('p', "{"):
		b, err := a.bloque()
		return jbloque{sentencias: b, donde: pos{linea}}, err == nil, err
	case a.es('k', "return"):
		a.avanzar()
		if a.es('p', ";") {
			a.avanzar()
			return jretorno{donde: pos{linea}}, true, nil
		}
		e, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jretorno{e: e, donde: pos{linea}}, true, nil
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
		return jif{cond: cond, entonces: entonces, siNo: siNo, donde: pos{linea}}, true, nil
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
		return jmientras{cond: cond, cuerpo: cuerpo, donde: pos{linea}}, true, nil
	case a.es('k', "for"):
		a.avanzar()
		if err := a.comerSimbolo("("); err != nil {
			return nil, false, err
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
			if op, esAsig := operadorDeAsignacion(a.actual()); esAsig {
				a.avanzar()
				valor, err := a.expresion()
				if err != nil {
					return nil, false, err
				}
				piezas = append(piezas, asignacion(dst, op, valor, pos{linea}))
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
		p := jpara{cuerpo: cuerpo, cond: cond, paso: paso, donde: pos{linea}}
		if inicio == nil {
			return p, true, nil
		}
		return jbloque{sentencias: []jstmt{inicio, p}, donde: pos{linea}}, true, nil
	case a.es('k', "switch"):
		return a.cambio(linea)
	case a.es('k', "break"):
		a.avanzar()
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jrompe{donde: pos{linea}}, true, nil
	case a.es('k', "continue"):
		a.avanzar()
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return jcontinua{donde: pos{linea}}, true, nil
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
		return jhacer{cuerpo: cuerpo, cond: cond, donde: pos{linea}}, true, nil
	case a.es('k', "try"):
		return nil, false, fmt.Errorf("linea %d: la sentencia %q no esta medida en este compilador", linea, a.actual().valor)
	}
	// declaracion local o expresion
	tipoActual := a.actual()
	if (tipoActual.kind == 'i' || (tipoActual.kind == 'k' && esNombreDeTipo(tipoActual.valor))) && !a.es('k', "void") {
		if a.pareceDeclaracion() {
			tipo, err := a.tipo()
			if err != nil {
				return nil, false, err
			}
			final := esFinalDeClaracion(tipo)
			if a.actual().kind != 'i' {
				return nil, false, fmt.Errorf("linea %d: la declaracion no tiene nombre", a.actual().donde.linea)
			}
			nombre := a.avanzar().valor
			d := jdecl{tipo: tipo, nombre: nombre, donde: pos{linea}, final: final}
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
	if op, esAsig := operadorDeAsignacion(a.actual()); esAsig {
		a.avanzar()
		valor, err := a.expresion()
		if err != nil {
			return nil, false, err
		}
		if err := a.comerSimbolo(";"); err != nil {
			return nil, false, err
		}
		return asignacion(e, op, valor, pos{linea}), true, nil
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
	case "=", "+=", "-=", "*=", "/=":
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

func esFinalDeClaracion(tipo string) bool { return strings.HasPrefix(tipo, "final ") }

// pareceDeclaracion: `Tipo nombre` (dos identificadores seguidos) o `final Tipo nombre`.
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
	return a.expresionBinaria(0)
}

// precedencias: las que usa este subconjunto, medidas con el emisor. El `%` entra porque su `irem` esta
// leido de `javap -c` (parrafo 49), no porque la precedencia se suponga.
var precedenciasJava = map[string]int{"||": 1, "&&": 2, "==": 3, "!=": 3, "<": 4, ">": 4, "<=": 4, ">=": 4, "+": 5, "-": 5, "*": 6, "/": 6, "%": 6}

func (a *analizadorJava) expresionBinaria(min int) (jexpr, error) {
	izq, err := a.expresionUnaria()
	if err != nil {
		return nil, err
	}
	for {
		t := a.actual()
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
		return jincr{destino: e, delta: delta, donde: donde}, nil
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
		if err == nil && a.es('p', ")") {
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

// atomo lee un literal, this, super, un nombre o un `new`.
func (a *analizadorJava) atomo() (jexpr, error) {
	t := a.actual()
	switch {
	case t.kind == 's':
		a.avanzar()
		return jlit{tipoJ: 's', texto: t.valor, donde: t.donde}, nil
	case t.kind == 'n':
		a.avanzar()
		n, err := enteroJava(t.valor)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", t.donde.linea, err)
		}
		return jlit{tipoJ: 'i', entero: n, donde: t.donde}, nil
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
		if a.es('p', ".") && a.i+1 < len(a.toks) && a.toks[a.i+1].kind == 'i' {
			// `this.x`: acceso al campo propio
			a.avanzar()
			nombre := a.avanzar().valor
			return jacceso{base: jthis{donde: t.donde}, nombre: nombre, donde: t.donde}, nil
		}
		return jthis{donde: t.donde}, nil
	case a.es('k', "super"):
		a.avanzar()
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
		if a.es('p', "[") {
			a.avanzar()
			if a.es('p', "]") {
				return nil, fmt.Errorf("linea %d: el inicializador `new %s[] {…}` no esta medido; escriba `new %s[n]`", t.donde.linea, tipo, tipo)
			}
			largo, err := a.expresion()
			if err != nil {
				return nil, err
			}
			if err := a.comerSimbolo("]"); err != nil {
				return nil, err
			}
			if a.es('p', "[") || a.es('p', "{") {
				return nil, fmt.Errorf("linea %d: un arreglo de mas de una dimension o con inicializador no esta medido", t.donde.linea)
			}
			return jnuevoArreglo{tipo: tipo, largo: largo, donde: t.donde}, nil
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
		if a.es('p', ".") {
			// R.layout.main: se lee como acceso y el emisor decide si es una constante inlinable.
			// MainActivity.this: la captura del exterior, que javac baja a un campo this$0.
			base := jexpr(jacceso{nombre: nombre, donde: t.donde})
			for a.es('p', ".") {
				a.avanzar()
				if a.es('k', "this") {
					a.avanzar()
					if acc, esAcceso := base.(jacceso); esAcceso && acc.base == nil {
						return jthis{nombre: acc.nombre, donde: t.donde}, nil
					}
					return nil, fmt.Errorf("linea %d: `.this` solo se mide detras de un nombre de clase suelto", t.donde.linea)
				}
				siguiente := a.actual()
				if siguiente.kind != 'i' {
					return nil, fmt.Errorf("linea %d: tras el punto se esperaba un nombre", siguiente.donde.linea)
				}
				a.avanzar()
				base = jacceso{base: base, nombre: siguiente.valor, donde: siguiente.donde}
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

// claseAnonima lee el cuerpo de un `new Interface() { ... }`. Lo que no sea implementar metodos no esta
// medido: javac le pone el campo this$0 y el constructor con la captura, y eso se comprueba aparte.
func (a *analizadorJava) claseAnonima(nombre string) (*jclas, error) {
	if a.claseActual == nil {
		return nil, fmt.Errorf("linea %d: una clase anonima fuera de una declaracion de clase no es Java", a.actual().donde.linea)
	}
	if strings.Contains(a.claseActual.nombre, "$") {
		return nil, fmt.Errorf("linea %d: una anonima dentro de otra anonima no esta medida: javac nombra Exterior$1$2 y aqui el contador es por clase de primera nivel", a.actual().donde.linea)
	}
	if a.claseActual != nil && a.claseActual.nombre != a.exterior {
		return nil, fmt.Errorf("linea %d: una anonima creada dentro de una clase anidada no esta medida (javac la nombra Exterior$Anidada$1)", a.actual().donde.linea)
	}
	c := &jclas{
		nombre:         a.exterior + "$" + strconv.Itoa(a.anonima),
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

// enteroJava: los literales numericos del subconjunto, con hexo y con la L de long rechazada si no cabe.
func enteroJava(texto string) (int32, error) {
	limpio := strings.ReplaceAll(texto, "_", "")
	if strings.HasSuffix(limpio, "L") || strings.HasSuffix(limpio, "l") {
		return 0, fmt.Errorf("un literal long (%s) no esta medido en el emisor", texto)
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
