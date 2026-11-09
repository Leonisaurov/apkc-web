package main

// La segunda pieza de la Fase 1G: escribir el `.class`. El objetivo esta medido instruccion a instruccion
// en docs/evidencia.md, parrafo 40, con el `javap -v -p` del JDK 21 del telefono sobre la plantilla de
// `apkc create` y sobre la `R.java` que escribe nuestro propio enlazador de la Fase 3.
//
// Cuatro reglas salieron de esa medicion, no de un manual:
//   - `setContentView` se declara en Activity y el Methodref apunta igual a la CLASE ACTUAL: javac usa el
//     tipo estatico del receptor implicito. De la cadena de superclases se toma la DESCRIPCION, no el
//     duenio de la referencia. Con el receptor escrito (`Toast.makeText(...)`) el Methodref va contra el
//     tipo del receptor, y con `super.` contra el padre nombrado.
//   - `R.layout.main` no es un `getstatic`: un `static final int` con valor literal es una constante de
//     compilacion y javac la dobla en el llamador (`ldc int 2130771968`).
//   - una clase sin constructor escrito recibe uno sintetizado: `aload_0`, `invokespecial` al `<init>` del
//     padre (o a `java/lang/Object` si el fuente no dice padre) y `return`, con la LineNumberTable
//     apuntando a la linea de la DECLARACION de clase.
//   - el `return` que cierra un metodo `void` lleva la linea de la llave que lo cierra: en la plantilla es
//     la 16, que es donde esta el `}` de `onCreate`.
//
// La version emitida es la 52 (Java 8). javac 21 escribe 65 para estos mismos fuentes y no importa: lo que
// se comprueba es la secuencia de instrucciones del javap y el dex que sale de ahi, y 52 es la que aceptan
// sin discusion este dexer y `d8`. Con 52 tampoco viajan `NestHost` ni `NestMembers`, que son de Java 11 y
// que javac 21 si escribe: el parrafo 40 los midio y aqui se omiten a sabiendas.

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// versionDeClassEmitida: Java 8, dicho arriba.
const versionDeClassEmitida = 52

// accMandated: el flag con el que javac marca el parametro de la captura en el MethodParameters del
// constructor sintetizado de una anonima (parrafo 42, medido como `final mandated` = 0x8010).
const accMandated = 0x8000

// Los opcodes que este emisor escribe, con el numero del JVMS.
const (
	opAconstNull = 0x01
	opLdc        = 0x12
	opPop        = 0x57
	opPop2       = 0x58
	opDup        = 0x59
	opDupX1      = 0x5a
	// las formas que duplican un valor de dos palabras: el `dup2` del access$ de un estático ancho y el
	// `dup2_x1` del de instancia (medidos en el párrafo 83 sobre `E` y `D`)
	opDup2       = 0x5c
	opDup2X1     = 0x5d
	opGetstatic  = 0xb2
	opPutstatic  = 0xb3
	opGetfield   = 0xb4
	opPutfield   = 0xb5
	opInvokevirt = 0xb6
	opInvokespec = 0xb7
	opInvokestat = 0xb8
	opInvointf   = 0xb9
	opNew        = 0xbb
	opCheckcast  = 0xc0
	// las branquias, con la inversión que hace javac (medida en el parrafo 52 sobre `javap -c`):
	// `if (a == b)` sale `if_icmpne`, `if (a > 0)` sale `ifle`, `if (o == null)` sale `ifnonnull`
	opIFEQ     = 0x99
	opIFNE     = 0x9a
	opIFLT     = 0x9b
	opIFGE     = 0x9c
	opIFGT     = 0x9d
	opIFLE     = 0x9e
	opIFICMPEQ = 0x9f
	opIFICMPNE = 0xa0
	opIFICMPLT = 0xa1
	opIFICMPGE = 0xa2
	opIFICMPGT = 0xa3
	opIFICMPLE = 0xa4
	// la comparacion de REFERENCIAS (parrafo 89): `if_acmpeq` 0xa5 y `if_acmpne` 0xa6, con la misma
	// inversion que las de entero. `o.getClass() == String.class` de javac sale `if_acmpne`.
	opIFACMPEQ  = 0xa5
	opIFACMPNE  = 0xa6
	opGOTO      = 0xa7
	opIFNULL    = 0xc6
	opIFNONNULL = 0xc7
	// las comparaciones anchas (párrafo 81): `lcmp` y las cuatro que se distinguen por el signo que
	// dejan con un NaN. javac usa `…l` con `>`, `>=`, `==` y `!=`, y `…g` con `<` y `<=` (JVMS 2.9)
	opLcmp  = 0x94
	opFcmpl = 0x95
	opFcmpg = 0x96
	opDcmpl = 0x97
	opDcmpg = 0x98
	// aritmetica entera, leida de `javap -c` sobre `a*b`, `a/b`, `a%b`, `a+b`, `a-b` (parrafo 49)
	opIadd = 0x60
	opIsub = 0x64
	opImul = 0x68
	opIdiv = 0x6c
	opIrem = 0x70
)

// descPrimitivo: el descriptor de cada tipo escrito en el fuente. `long` y `double` ocupan dos huecos
// de variable y dos palabras de pila; `float`, uno y uno (párrafos 74 y 77).
var descPrimitivo = map[string]string{
	"int": "I", "boolean": "Z", "short": "S", "byte": "B", "char": "C",
	"long": "J", "float": "F", "double": "D", "void": "V",
}

// envoltorioDePrimitivo: el `Tipo.class` de un primitivo no es `ldc` sino el `getstatic` de
// `Wrapper.TYPE:Ljava/lang/Class;`, medido en el parrafo 89 con `int.class` y `void.class`.
var envoltorioDePrimitivo = map[string]string{
	"boolean": "java/lang/Boolean", "byte": "java/lang/Byte", "char": "java/lang/Character",
	"short": "java/lang/Short", "int": "java/lang/Integer", "long": "java/lang/Long",
	"float": "java/lang/Float", "double": "java/lang/Double", "void": "java/lang/Void",
}

// puntoFlotante era el guardia que rechazaba `F` y `D`: entraron medidos en el parrafo 77 y ya no
// queda nada de tipos primitivos fuera del subconjunto salvo el `char` (5.3).

// esAncho: los descriptores que ocupan DOS huecos de local y dos palabras de pila: `J` y `D`. Medido
// con `javap -v` sobre la sonda `F` (parrafo 74): `conRama(long a, int n)` con un local `long x` sale con
// `locals=5`, huecos 0-1 para `a`, 2 para `n` y 3-4 para `x`; y con `javap -c` sobre `FyD`: el `float`
// ocupa UN hueco (`por(float a, double b)` carga `a` en 0 con `fload_0` y `b` en 1 con `dload_1`) y el
// `double` dos (`suma(double a, double b)` es `dload_0; dload_2`).
func esAncho(desc string) bool {
	return desc != "" && (desc[0] == 'J' || desc[0] == 'D')
}

// ----- el constant pool -----

type poolBuilder struct {
	entradas [][]byte
	indices  map[string]int
}

func nuevoPool() *poolBuilder { return &poolBuilder{indices: map[string]int{}} }

// agregar: el indice es la posicion + 1 (el 0 esta reservado por el formato). Ninguna entrada que este
// emisor escribe ocupa dos huecos: `long` y `double` se rechazan antes de llegar aqui.
func (p *poolBuilder) agregar(clave string, e []byte) int {
	if i, ok := p.indices[clave]; ok {
		return i
	}
	p.entradas = append(p.entradas, e)
	i := len(p.entradas)
	p.indices[clave] = i
	return i
}

func (p *poolBuilder) utf8(s string) int {
	body := []byte{1, byte(len(s) >> 8), byte(len(s))}
	return p.agregar("u\x00"+s, append(body, []byte(s)...))
}

func (p *poolBuilder) entero(v int32) int {
	b := []byte{3, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[1:], uint32(v))
	return p.agregar(fmt.Sprintf("i\x00%d", v), b)
}

// largo8: la entrada `Long` (tag 5) del pool, que **ocupa dos posiciones**: medido en `javap -v` de la
// sonda `L`, donde `#7 = Long 2l` va seguido de `#9` (el #8 no existe). El hueco se reserva con una
// entrada sin bytes, que `serializar` no escribe pero sí cuenta.
func (p *poolBuilder) largo8(v int64) int {
	b := []byte{5, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint64(b[1:], uint64(v))
	return p.agregarDos(fmt.Sprintf("j\x00%d", v), b)
}

// doble8: la entrada `Double` —**tag 6**, medido en los bytes de `FyD.class`, donde el `2.5d` del campo
// estático va precedido del 0x06— y ocupa dos posiciones como la `Long`. Sale con `ldc2_w` (parrafo 77).
func (p *poolBuilder) doble8(v float64) int {
	b := []byte{6, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(v))
	return p.agregarDos(fmt.Sprintf("d\x00%v", v), b)
}

// flotante4: la entrada `Float` (**tag 4**, medido igual), de cuatro bytes y **una sola posicion**:
// `FyD` la escribe con `ldc #25 // float 1.5f` (parrafo 77).
func (p *poolBuilder) flotante4(v float64) int {
	b := []byte{4, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[1:], math.Float32bits(float32(v)))
	return p.agregar(fmt.Sprintf("f\x00%v", v), b)
}

// agregarDos: una entrada de ocho bytes seguida del hueco que ocupa su segunda posicion.
func (p *poolBuilder) agregarDos(clave string, e []byte) int {
	if i, ok := p.indices[clave]; ok {
		return i
	}
	p.entradas = append(p.entradas, e, nil)
	i := len(p.entradas) - 1
	p.indices[clave] = i
	return i
}

// cadena: tag 8 con el indice de su utf8. En el pool un String es el indice, no el texto.
func (p *poolBuilder) cadena(texto string) int {
	u := p.utf8(texto)
	b := []byte{8, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(u))
	return p.agregar("s\x00"+texto, b)
}

func (p *poolBuilder) clase(nombreInternal string) int {
	u := p.utf8(nombreInternal)
	b := []byte{7, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(u))
	return p.agregar("c\x00"+nombreInternal, b)
}

func (p *poolBuilder) nombreYTipo(nombre, desc string) int {
	ni := p.utf8(nombre)
	di := p.utf8(desc)
	b := []byte{12, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(ni))
	binary.BigEndian.PutUint16(b[3:], uint16(di))
	return p.agregar("nt\x00"+nombre+"|"+desc, b)
}

// referencia: tag 10 para metodo de clase, 11 para metodo de interface y 9 para campo.
func (p *poolBuilder) referencia(tag byte, claseInternal, nombre, desc string) int {
	ci := p.clase(claseInternal)
	nt := p.nombreYTipo(nombre, desc)
	b := []byte{tag, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(ci))
	binary.BigEndian.PutUint16(b[3:], uint16(nt))
	return p.agregar(fmt.Sprintf("r\x00%d%s|%s|%s", tag, claseInternal, nombre, desc), b)
}

func (p *poolBuilder) metodo(claseInternal, nombre, desc string) int {
	return p.referencia(10, claseInternal, nombre, desc)
}

func (p *poolBuilder) metodoInterface(claseInternal, nombre, desc string) int {
	return p.referencia(11, claseInternal, nombre, desc)
}

func (p *poolBuilder) campo(claseInternal, nombre, desc string) int {
	return p.referencia(9, claseInternal, nombre, desc)
}

// serializar: el conteo es uno mas que el ultimo indice, como pide el formato.
func (p *poolBuilder) serializar() []byte {
	total := len(p.entradas) + 1
	out := u2be(uint16(total))
	for _, e := range p.entradas {
		out = append(out, e...)
	}
	return out
}

// ----- escribir big endian -----

func u2be(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }

func u4be(v uint32) []byte { return append(u2be(uint16(v>>16)), u2be(uint16(v))...) }

// atributo: nombre, longitud y contenido, la forma que comparten todos los attributes_info.
func attrInfo(pool *poolBuilder, nombre string, cuerpo []byte) []byte {
	return append(append(u2be(uint16(pool.utf8(nombre))), u4be(uint32(len(cuerpo)))...), cuerpo...)
}

// ----- los opcodes de variable, por descriptor -----

// carga y guarda devuelven el opcode breve (el indice va en el propio opcode, 0 a 3) y el opcode con el
// indice detras, que es el que se usa a partir de 4. Son los numeros del JVMS: iload_0 0x1a, aload_0 0x2a,
// istore_0 0x3b, astore_0 0x4b, y las formas con indice 0x15, 0x19, 0x36 y 0x3a. El 0x4b esta leido de
// `javap -c` sobre `View v = findViewById(...)` (parrafo 49): con 0x47, que es dstore_0, el `astore_2` de
// javac salia como `dstore_2` y la ranura se contaba de a dos.
func carga(desc byte) (breve byte, conIndice byte) {
	if desc == 'I' || desc == 'B' || desc == 'C' || desc == 'S' || desc == 'Z' {
		return 0x1a, 0x15
	}
	// `long`: lload_0 0x1e y lload 0x16, medidos en `javap -c` de la sonda `W` (parrafo 74).
	// `float`/`double`: fload_0 0x22 con fload 0x17, y dload_0 0x26 con dload 0x18, leidos de `FyD`
	// en el parrafo 77 (`resta(float a, float b)` es `fload_0; fload_1` y `suma(double,double)` es
	// `dload_0; dload_2`)
	if desc == 'J' {
		return 0x1e, 0x16
	}
	if desc == 'F' {
		return 0x22, 0x17
	}
	if desc == 'D' {
		return 0x26, 0x18
	}
	return 0x2a, 0x19
}

func guarda(desc byte) (breve byte, conIndice byte) {
	if desc == 'I' || desc == 'B' || desc == 'C' || desc == 'S' || desc == 'Z' {
		return 0x3b, 0x36
	}
	// lstore_0 0x3f y lstore 0x37 (parrafo 74); fstore_0 0x43 con fstore 0x38 y dstore_0 0x47 con
	// dstore 0x39 (parrafo 77, `FyD.local`: `dconst_0; dstore_1`)
	if desc == 'J' {
		return 0x3f, 0x37
	}
	if desc == 'F' {
		return 0x43, 0x38
	}
	if desc == 'D' {
		return 0x47, 0x39
	}
	return 0x4b, 0x3a
}

func retorno(desc byte) byte {
	switch desc {
	case 'V':
		return 0xb1
	case 'I', 'B', 'C', 'S', 'Z':
		return 0xac
	case 'J':
		return 0xad
	case 'F':
		return 0xae // freturn, leido de `FyD.resta` (parrafo 77)
	case 'D':
		return 0xaf // dreturn, leido de `FyD.suma` (parrafo 77)
	default:
		return 0xb0
	}
}

// ----- resolucion de nombres contra el android.jar -----

// buscadorDeClases abre el jar y recuerda lo que ya leyó. Un enlace no deberia leer mas de una veintena de
// entradas del jar, que son ficheros de pocos KB: el android.jar entero no se toca.
type buscadorDeClases struct {
	zr      *zip.ReadCloser
	indice  map[string]*zip.File // nombre internal -> entrada, para no andar el listado en cada lectura
	nombres map[string]bool      // las entradas .class que hay dentro, para resolver los imports con *
	cache   map[string]*classFull
}

func abrirBuscador(rutaJar string) (*buscadorDeClases, error) {
	zr, err := zip.OpenReader(rutaJar)
	if err != nil {
		return nil, fmt.Errorf("el android.jar %s no abre: %w", rutaJar, err)
	}
	b := &buscadorDeClases{zr: zr, indice: map[string]*zip.File{}, nombres: map[string]bool{}, cache: map[string]*classFull{}}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".class") {
			continue
		}
		internal := strings.TrimSuffix(f.Name, ".class")
		b.nombres[internal] = true
		if _, ya := b.indice[internal]; !ya {
			b.indice[internal] = f
		}
	}
	return b, nil
}

func (b *buscadorDeClases) cerrar() { _ = b.zr.Close() }

func (b *buscadorDeClases) hayEnElJar(internal string) bool { return b.nombres[internal] }

// claseDelJar lee una entrada del jar y la pasa por el lector de la Fase 2, que es el que ya sabe andar un
// constant pool de verdad.
func (b *buscadorDeClases) claseDelJar(internal string) (*classFull, error) {
	if c, ok := b.cache[internal]; ok {
		return c, nil
	}
	f, ok := b.indice[internal]
	if !ok {
		return nil, fmt.Errorf("%s no esta en el android.jar", internal)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return nil, err
	}
	c, err := readClassFull(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	b.cache[internal] = &c
	return &c, nil
}

func cuentaArgumentosJava(desc string) int {
	abre := strings.Index(desc, "(")
	cierra := strings.Index(desc, ")")
	if abre < 0 || cierra < 0 {
		return 0
	}
	dentro := desc[abre+1 : cierra]
	n := 0
	for i := 0; i < len(dentro); {
		switch dentro[i] {
		case '[':
			i++
		case 'L':
			for i < len(dentro) && dentro[i] != ';' {
				i++
			}
			i++
			n++
		default:
			i++
			n++
		}
	}
	return n
}

// vueltaDe: el descriptor de lo que un descriptor de metodo devuelve.
func vueltaDe(desc string) string {
	cierra := strings.Index(desc, ")")
	if cierra < 0 {
		return ""
	}
	return desc[cierra+1:]
}

// ----- el contexto del fuente completo -----

// contextoJava lleva lo que todas las clases del mismo fuente comparten: el buscador del jar, las clases
// del propio fuente por su nombre internal (para doblar la constante de `R.layout.main`) y el paquete.
type contextoJava struct {
	busc    *buscadorDeClases
	paquete string
	imports []string
	orden   []*jclas // en el orden del fuente, para resolver sin depender del azar de un mapa
	propias map[string]*jclas
	archivo string // el SourceFile que anota javac: el nombre del fuente, sin rutas

	// accesores son los metodos sinteticos `access$NNN` que javac escribe en el exterior cuando una
	// anonima toca un miembro suyo privado (parrafo 72). Se guardan por dueño y en orden de creacion:
	// el numero que va en el nombre es esa posicion, y el orden es el que javac deja en el ClassFile
	// (cuerpos del exterior primero, clases anidadas despues).
	accesores map[string][]accesorSintetico
	accesorID map[string]accesorSintetico
	// accesorNumero da el indice de cada miembro privado (campos y metodos comparten la secuencia):
	// leer y escribir un mismo campo llevan el mismo indice y distinto ultimo digito, medido en el
	// parrafo 72 (`access$000` y `access$002` para el primer miembro).
	accesorNumero map[string]int
}

// accesorSintetico: un `access$NNN` del exterior. El cuerpo se deduce del miembro y del sentido: leer
// un campo, escribirlo (y devolver el valor: es lo que hace javac y por eso el llamador emite un `pop`)
// o llamar a un metodo privado.
type accesorSintetico struct {
	nombre      string
	desc        string
	duenio      string
	miembro     string
	miembroDesc string
	campo       bool
	escritura   bool
	estatico    bool
}

// internoDeDescriptor: el lector de la Fase 2 guarda `this_class` y `super_class` en forma de descriptor
// (`Landroid/widget/TextView;`), y el indice del jar va por nombre internal. Sin esta conversion el
// segundo eslabon de la cadena de padres responde que la clase "no esta en el android.jar", que es lo
// que pasaba al resolver `setText` de un `Button`, declarado en TextView.
func internoDeDescriptor(desc string) string {
	if len(desc) >= 2 && desc[0] == 'L' && desc[len(desc)-1] == ';' {
		return desc[1 : len(desc)-1]
	}
	return desc
}

func internoDe(paquete string, nombre string) string {
	if paquete == "" {
		return nombre
	}
	return strings.ReplaceAll(paquete, ".", "/") + "/" + nombre
}

func (ctx *contextoJava) registrar(c *jclas) {
	ctx.orden = append(ctx.orden, c)
	ctx.propias[internoDe(c.paquete, c.nombre)] = c
	for _, m := range c.metodos {
		for _, s := range m.cuerpo {
			ctx.registrarSentencia(s)
		}
	}
	for _, d := range c.interior {
		ctx.registrar(d)
	}
}

// registrarSentencia y registrarExpresion bajan por el arbol hasta los `new Tipo(...) { ... }`: la clase
// anonima que crean no esta en la lista de la clase que la rodea, y el emisor tiene que conocerla para
// resolver su nombre y para escribir su fichero (parrafo 42).
func (ctx *contextoJava) registrarSentencia(s jstmt) {
	switch v := s.(type) {
	case jstmtExpr:
		ctx.registrarExpresion(v.e)
	case jretorno:
		if v.e != nil {
			ctx.registrarExpresion(v.e)
		}
	case jdecl:
		if v.valor != nil {
			ctx.registrarExpresion(v.valor)
		}
	case jasign:
		ctx.registrarExpresion(v.destino)
		ctx.registrarExpresion(v.valor)
	case jbloque:
		for _, st := range v.sentencias {
			ctx.registrarSentencia(st)
		}
	case jif:
		ctx.registrarExpresion(v.cond)
		if v.entonces != nil {
			ctx.registrarSentencia(v.entonces)
		}
		if v.siNo != nil {
			ctx.registrarSentencia(v.siNo)
		}
	case jmientras:
		ctx.registrarExpresion(v.cond)
		ctx.registrarSentencia(v.cuerpo)
	case jpara:
		ctx.registrarExpresion(v.cond)
		ctx.registrarSentencia(v.cuerpo)
		if v.paso != nil {
			ctx.registrarSentencia(v.paso)
		}
	case jhacer:
		ctx.registrarSentencia(v.cuerpo)
		ctx.registrarExpresion(v.cond)
	case jtry:
		ctx.registrarSentencia(v.cuerpo)
		for _, c := range v.atrapas {
			ctx.registrarSentencia(c.atrapa)
		}
		if v.finalmente != nil {
			ctx.registrarSentencia(v.finalmente)
		}
	case jlanza:
		ctx.registrarExpresion(v.e)
	case jswitch:
		ctx.registrarExpresion(v.selector)
		for _, c := range v.casos {
			for _, s := range c.cuerpo {
				ctx.registrarSentencia(s)
			}
		}
		for _, s := range v.defecto {
			ctx.registrarSentencia(s)
		}
	}
}

func (ctx *contextoJava) registrarExpresion(x jexpr) {
	switch v := x.(type) {
	case jnuevo:
		for _, a := range v.args {
			ctx.registrarExpresion(a)
		}
		if v.cuerpo != nil {
			ctx.registrar(v.cuerpo)
		}
	case jnuevoArreglo:
		if v.largo != nil {
			ctx.registrarExpresion(v.largo)
		}
		for _, el := range v.elementos {
			ctx.registrarExpresion(el)
		}
	case jindice:
		ctx.registrarExpresion(v.base)
		ctx.registrarExpresion(v.indice)
	case jincr:
		ctx.registrarExpresion(v.destino)
	case jllamada:
		if v.base != nil {
			ctx.registrarExpresion(v.base)
		}
		for _, a := range v.args {
			ctx.registrarExpresion(a)
		}
	case jacceso:
		if v.base != nil {
			ctx.registrarExpresion(v.base)
		}
	case jcast:
		ctx.registrarExpresion(v.dentro)
	case jinstanceof:
		ctx.registrarExpresion(v.dentro)
	case joperador:
		ctx.registrarExpresion(v.izq)
		ctx.registrarExpresion(v.der)
	case jnegacion:
		ctx.registrarExpresion(v.dentro)
	}
}

// anonimasDe: las clases anonimas que crea el cuerpo de c, en el orden del fuente.
func anonimasDe(c *jclas) []*jclas {
	var out []*jclas
	var expr func(jexpr)
	var sent func(jstmt)
	sent = func(s jstmt) {
		switch v := s.(type) {
		case jstmtExpr:
			expr(v.e)
		case jretorno:
			if v.e != nil {
				expr(v.e)
			}
		case jdecl:
			if v.valor != nil {
				expr(v.valor)
			}
		case jasign:
			expr(v.destino)
			expr(v.valor)
		case jbloque:
			for _, st := range v.sentencias {
				sent(st)
			}
		case jif:
			expr(v.cond)
			sent(v.entonces)
			if v.siNo != nil {
				sent(v.siNo)
			}
		case jmientras:
			expr(v.cond)
			sent(v.cuerpo)
		case jpara:
			expr(v.cond)
			sent(v.cuerpo)
			if v.paso != nil {
				sent(v.paso)
			}
		case jhacer:
			sent(v.cuerpo)
			expr(v.cond)
		case jswitch:
			expr(v.selector)
			for _, c := range v.casos {
				for _, st := range c.cuerpo {
					sent(st)
				}
			}
			for _, st := range v.defecto {
				sent(st)
			}
		}
	}
	expr = func(x jexpr) {
		switch v := x.(type) {
		case jnuevo:
			for _, a := range v.args {
				expr(a)
			}
			if v.cuerpo != nil {
				out = append(out, v.cuerpo)
			}
		case jllamada:
			if v.base != nil {
				expr(v.base)
			}
			for _, a := range v.args {
				expr(a)
			}
		case jacceso:
			if v.base != nil {
				expr(v.base)
			}
		case jcast:
			expr(v.dentro)
		case jinstanceof:
			expr(v.dentro)
		case jnuevoArreglo:
			if v.largo != nil {
				expr(v.largo)
			}
			for _, el := range v.elementos {
				expr(el)
			}
		case jindice:
			expr(v.base)
			expr(v.indice)
		case jincr:
			expr(v.destino)
		}
	}
	for _, m := range c.metodos {
		for _, s := range m.cuerpo {
			sent(s)
		}
	}
	return out
}

func (ctx *contextoJava) hay(internal string) bool {
	if _, ok := ctx.propias[internal]; ok {
		return true
	}
	return ctx.busc.hayEnElJar(internal)
}

// resolverSimple sigue el orden de JLS que usa javac: una clase del propio fuente, el paquete del fuente,
// un import concreto, los imports con estrella y por ultimo java.lang.
func (ctx *contextoJava) resolverSimple(nombre string) (string, error) {
	for _, c := range ctx.orden {
		if sinPaquete(c.nombre) == nombre {
			return internoDe(c.paquete, c.nombre), nil
		}
	}
	if c := internoDe(ctx.paquete, nombre); ctx.hay(c) {
		return c, nil
	}
	for _, imp := range ctx.imports {
		if strings.HasSuffix(imp, ".*") || sinPaquete(imp) != nombre {
			continue
		}
		if c := strings.ReplaceAll(imp, ".", "/"); ctx.hay(c) {
			return c, nil
		}
	}
	for _, imp := range ctx.imports {
		if !strings.HasSuffix(imp, ".*") {
			continue
		}
		c := strings.ReplaceAll(strings.TrimSuffix(imp, ".*"), ".", "/") + "/" + nombre
		if ctx.hay(c) {
			return c, nil
		}
	}
	if ctx.hay("java/lang/" + nombre) {
		return "java/lang/" + nombre, nil
	}
	return "", fmt.Errorf("el tipo %s no se resuelve: ni en el fuente, ni en su paquete, ni en los imports, ni en java.lang, ni en el android.jar", nombre)
}

// tipoAInterno resuelve `MenuItem.OnMenuItemClickListener` contra el jar: la cabeza por resolverSimple y
// cada segmento siguiente como clase anidada, con el `$` con el que javac nombra las anidadas.
func (ctx *contextoJava) tipoAInterno(nombre string) (string, error) {
	bits := strings.Split(nombre, ".")
	if _, ok := descPrimitivo[bits[0]]; ok {
		return "", fmt.Errorf("el tipo primitivo %q no puede nombrar una clase", bits[0])
	}
	base, err := ctx.resolverSimple(bits[0])
	if err != nil {
		return "", err
	}
	for _, seg := range bits[1:] {
		if ctx.hay(base + "$" + seg) {
			base += "$" + seg
			continue
		}
		return "", fmt.Errorf("%s no es una clase anidada de %s en este classpath", seg, base)
	}
	return base, nil
}

// descriptorDe: como se escribe un tipo del fuente dentro de un descriptor.
func (ctx *contextoJava) descriptorDe(tipo string) (string, error) {
	if strings.HasSuffix(tipo, "[]") {
		// un arreglo se escribe con un corchete delante del descriptor del elemento: `int[]` -> `[I`,
		// `String[]` -> `[Ljava/lang/String;`, `int[][]` -> `[[I`. Es la forma que midio `javap -v`
		// sobre `Arreglos` (parrafo 66).
		elem, err := ctx.descriptorDe(strings.TrimSuffix(tipo, "[]"))
		if err != nil {
			return "", err
		}
		if elem == "V" {
			return "", fmt.Errorf("un arreglo de void no existe")
		}
		return "[" + elem, nil
	}
	if d, ok := descPrimitivo[tipo]; ok {
		return d, nil
	}
	i, err := ctx.tipoAInterno(tipo)
	if err != nil {
		return "", err
	}
	return "L" + i + ";", nil
}

func (ctx *contextoJava) clasePropia(internal string) (*jclas, bool) {
	c, ok := ctx.propias[internal]
	return c, ok
}

// descDeMetodo: el descriptor `(tipos)vuelta` de un metodo declarado en el fuente.
func (ctx *contextoJava) descDeMetodo(m jmetodo) (string, error) {
	sb := strings.Builder{}
	sb.WriteString("(")
	for _, p := range m.params {
		d, err := ctx.descriptorDe(p.tipo)
		if err != nil {
			return "", fmt.Errorf("linea %d: el parametro %s: %w", p.donde.linea, p.tipo, err)
		}
		sb.WriteString(d)
	}
	sb.WriteString(")")
	if m.retorno == "" || m.retorno == "void" {
		sb.WriteString("V")
		return sb.String(), nil
	}
	d, err := ctx.descriptorDe(m.retorno)
	if err != nil {
		return "", fmt.Errorf("linea %d: la vuelta %s: %w", m.linea, m.retorno, err)
	}
	sb.WriteString(d)
	return sb.String(), nil
}

// padreDe: el nombre internal del padre declarado, o java/lang/Object cuando el fuente no dice nada.
func (ctx *contextoJava) padreDe(c *jclas) (string, error) {
	if c.super == "" {
		return "java/lang/Object", nil
	}
	return ctx.tipoAInterno(c.super)
}

// accesosDe: los access_flags del tipo, como los veria javap. De los propios se toman los modificadores
// del fuente; de los del jar su atributo access_flags sin el ACC_SUPER, que es del ClassFile y nunca
// aparece en el InnerClasses.
func (ctx *contextoJava) accesosDe(internal string) (int, error) {
	if c, ok := ctx.clasePropia(internal); ok {
		acc := 0
		if c.esPublica {
			acc |= accPublic
		}
		if c.esEstatica {
			acc |= accStatic
		}
		if c.final {
			acc |= accFinal
		}
		if c.abstracta {
			acc |= accAbstract
		}
		if c.esInterface {
			// sin esto una llamada `I.doble()` se emitia como invokevirtual; medido en `Uso.usa`
			// del parrafo 92: javac escribe `invokeinterface I.doble:(I)I`
			acc |= accInterface
		}
		return acc, nil
	}
	c, err := ctx.busc.claseDelJar(internal)
	if err != nil {
		return 0, err
	}
	// en la fila del InnerClasses javac anade ACC_STATIC a la anidada del framework: medido sobre
	// MenuItem$OnMenuItemClickListener, donde el ClassFile de la interface dice 0x0601 y la fila dice 0x0609
	// (lo que aqui se usa para decidir `invokeinterface` es el 0x0601 del ClassFile, no esta maskara).
	accesos := c.Access &^ accSuper
	if strings.Contains(internal, "$") {
		accesos |= accStatic
	}
	return accesos, nil
}

// usarDescriptor apunta las anidadas que aparecen escritas dentro de un descriptor. javac las recuerda en el
// InnerClasses del que llama aunque el tipo solo salga ahi: medido en el parrafo 42, donde MainActivity lista
// MenuItem$OnMenuItemClickListener sin nombrarla en ningun opcode.
func (e *emisor) usarDescriptor(desc string) {
	for i := 0; i < len(desc); i++ {
		if desc[i] != 'L' {
			continue
		}
		j := strings.IndexByte(desc[i+1:], ';')
		if j < 0 {
			break
		}
		nombre := desc[i+1 : i+1+j]
		if strings.Contains(nombre, "$") {
			e.usar(nombre)
		}
		i += j + 1
	}
}

// candidato: una de las sobrecargas que sirven para una llamada, con los accesos de quien la declara.
type candidato struct {
	desc   string
	acceso int
}

// buscarMetodo recorre la clase y sus padres por nombre y aridad, y elige la sobrecarga. Con **una**
// candidata devuelve esa, que es lo que este codigo habia hecho siempre; con varias se queda con la que
// coincide **exactamente** con los descriptores de los argumentos. Medido el 2026-11-01 (parrafo 62):
// `System.out.println(que)` con `que` de tipo String bajaba `println:(Z)V` —la primera declarada en
// PrintStream— porque el recorrido paraba en el primer nombre con la aridad debida; javac escribe
// `println:(Ljava/lang/String;)V`, y el dex lo refleja palabra a palabra. La eleccion por **subtipo**
// (`setText(CharSequence)` con un String, que no tiene candidata exacta y hoy se resuelve por haber una
// sola), el `boxing` y los varargs **no estan medidos**: con varias candidatas y ninguna exacta se
// rechaza por nombre, con las candidatas a la vista.
func (ctx *contextoJava) buscarMetodo(interno, nombre string, argc int, tipos []string) (candidato, error) {
	cands, err := ctx.candidatos(interno, nombre, argc)
	if err != nil {
		return candidato{}, err
	}
	if len(cands) == 0 {
		return candidato{}, fmt.Errorf("el metodo %s.%s con %d argumentos no esta en %s ni en sus padres",
			interno, nombre, argc, interno)
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	exactas := []candidato{}
	for _, c := range cands {
		_, _, ps := splitDescriptor(c.desc)
		if len(ps) != len(tipos) {
			continue
		}
		todas := true
		for i := range ps {
			if ps[i] != tipos[i] {
				todas = false
				break
			}
		}
		if todas {
			exactas = append(exactas, c)
		}
	}
	if len(exactas) == 1 {
		return exactas[0], nil
	}
	// sin candidata exacta, la regla de javac es la aplicable mas especifica (JVMS 15.12.2.5), y lo que
	// se puede medir aquí es la parte de tipos de referencia: identidad o subida por la cadena de padres
	// y las interfaces que el `.class` del jar declara. El caso real que lo pidió es `Menu.add("...")`
	// del fixture: `add(CharSequence)` y `add(int)`, y el String solo cabe en la primera.
	aplicables := []candidato{}
	for _, c := range cands {
		_, _, ps := splitDescriptor(c.desc)
		if len(ps) != len(tipos) {
			continue
		}
		cabe := true
		for i := range ps {
			if !ctx.asignable(tipos[i], ps[i]) {
				cabe = false
				break
			}
		}
		if cabe {
			aplicables = append(aplicables, c)
		}
	}
	if len(aplicables) == 1 {
		return aplicables[0], nil
	}
	if len(aplicables) > 1 {
		mas := []candidato{}
		for _, a := range aplicables {
			especifica := true
			for _, b := range aplicables {
				if a.desc == b.desc || ctx.masEspecifico(a.desc, b.desc) {
					continue
				}
				especifica = false
				break
			}
			if especifica {
				mas = append(mas, a)
			}
		}
		if len(mas) == 1 {
			return mas[0], nil
		}
	}
	posibles := make([]string, 0, len(cands))
	for _, c := range cands {
		posibles = append(posibles, c.desc)
	}
	return candidato{}, fmt.Errorf("%s.%s tiene %d sobrecargas de %d argumentos (%s) y los tipos llamados son (%s): ninguna es la mas especifica para este subconjunto",
		interno, nombre, len(cands), argc, strings.Join(posibles, " "), strings.Join(tipos, " "))
}

// masEspecifico: si cada parametro de `a` cabe en el de `b` (y alguno no al revés, o son iguales).
func (ctx *contextoJava) masEspecifico(a, b string) bool {
	_, _, pa := splitDescriptor(a)
	_, _, pb := splitDescriptor(b)
	if len(pa) != len(pb) {
		return false
	}
	for i := range pa {
		if !ctx.asignable(pa[i], pb[i]) {
			return false
		}
	}
	return true
}

// asignable: si un valor del tipo `desde` cabe en un parámetro del tipo `hasta`. Identidad, y entre
// referencias la subida por la cadena de padres y por las interfaces. Los arrays y el ensanchamiento de
// primitivos no están medidos: salen `false`, y eso deja la llamada en el rechazo por nombre.
func (ctx *contextoJava) asignable(desde, hasta string) bool {
	if desde == hasta {
		return true
	}
	if !esTipoDeReferencia(desde) || !esTipoDeReferencia(hasta) {
		return false
	}
	if desde[0] != 'L' || hasta[0] != 'L' {
		return false
	}
	objetivo := hasta[1 : len(hasta)-1]
	vistos := map[string]bool{}
	pila := []string{desde[1 : len(desde)-1]}
	for len(pila) > 0 {
		actual := pila[len(pila)-1]
		pila = pila[:len(pila)-1]
		if actual == objetivo {
			return true
		}
		if vistos[actual] || actual == "java/lang/Object" {
			continue
		}
		vistos[actual] = true
		padre, interfaces, err := ctx.superEInterfaces(actual)
		if err != nil {
			continue
		}
		if padre != "" {
			pila = append(pila, padre)
		}
		pila = append(pila, interfaces...)
	}
	return false
}

// superEInterfaces: el padre y las interfaces de un tipo, venga del fuente o del `android.jar`.
func (ctx *contextoJava) superEInterfaces(interno string) (padre string, interfaces []string, err error) {
	if propia, ok := ctx.clasePropia(interno); ok {
		p, err := ctx.padreDe(propia)
		return p, nil, err
	}
	c, err := ctx.busc.claseDelJar(interno)
	if err != nil {
		return "", nil, err
	}
	if c.Super != "" {
		padre = internoDeDescriptor(c.Super)
	}
	for _, i := range c.Interfaces {
		interfaces = append(interfaces, internoDeDescriptor(i))
	}
	return padre, interfaces, nil
}

func esTipoDeReferencia(desc string) bool {
	return len(desc) > 0 && (desc[0] == 'L' || desc[0] == '[')
}

// candidatos: las sobrecargas de nombre y aridad dados, en el orden de la jerarquia (la clase propia
// primero, luego el `android.jar`) y deduplicadas por descriptor, que es como javac ve un mismo metodo
// declarado y heredado.
func (ctx *contextoJava) candidatos(interno, nombre string, argc int) ([]candidato, error) {
	sal := []candidato{}
	vistas := map[string]bool{}
	poner := func(c candidato) {
		if vistas[c.desc] {
			return
		}
		vistas[c.desc] = true
		sal = append(sal, c)
	}
	actual := interno
	for visits := 0; actual != "" && visits < 32; visits++ {
		propia, esPropia := ctx.clasePropia(actual)
		if !esPropia {
			cs, err := ctx.busc.candidatosDelJar(actual, nombre, argc)
			if err != nil {
				return nil, err
			}
			for _, c := range cs {
				poner(c)
			}
			return sal, nil
		}
		for _, m := range propia.metodos {
			if m.nombre != nombre || len(m.params) != argc {
				continue
			}
			d, err := ctx.descDeMetodo(m)
			if err != nil {
				return nil, err
			}
			acceso := 0
			if m.esPublico {
				acceso |= accPublic
			}
			if m.esPrivado {
				acceso |= accPrivate
			}
			if m.esProtegido {
				acceso |= accProtected
			}
			if m.esEstatico {
				acceso |= accStatic
			}
			if m.esFinal {
				acceso |= accFinal
			}
			poner(candidato{desc: d, acceso: acceso})
		}
		if actual == "java/lang/Object" {
			break
		}
		var err error
		if actual, err = ctx.padreDe(propia); err != nil {
			return nil, err
		}
	}
	return sal, nil
}

// candidatosDelJar: lo mismo sobre las clases del `android.jar`, subiendo por la cadena de padres.
func (b *buscadorDeClases) candidatosDelJar(interno, nombre string, argc int) ([]candidato, error) {
	sal := []candidato{}
	vistas := map[string]bool{}
	actual := interno
	for visits := 0; actual != "" && visits < 32; visits++ {
		c, err := b.claseDelJar(actual)
		if err != nil {
			return nil, err
		}
		for _, m := range c.Methods {
			if m.Name != nombre || cuentaArgumentosJava(m.Desc) != argc || vistas[m.Desc] {
				continue
			}
			vistas[m.Desc] = true
			sal = append(sal, candidato{desc: m.Desc, acceso: m.Access})
		}
		if c.Super == "" {
			break
		}
		actual = internoDeDescriptor(c.Super)
	}
	return sal, nil
}

type campoResuelto struct {
	desc      string
	acceso    int
	dueño     string // el tipo contra el que se escribe el Fieldref, que es el que nombra javac
	constante bool
	valor     int32
}

// buscarCampo recorre la clase y sus padres. Una constante de compilacion trae el valor: javac la dobla en
// el llamador y aqui no se escribe getstatic.
func (ctx *contextoJava) buscarCampo(dueño, nombre string) (campoResuelto, error) {
	actual := dueño
	for visits := 0; actual != "" && visits < 32; visits++ {
		if propia, esPropia := ctx.clasePropia(actual); esPropia {
			for _, f := range propia.campos {
				if f.nombre != nombre {
					continue
				}
				d, err := ctx.descriptorDe(f.tipo)
				if err != nil {
					return campoResuelto{}, err
				}
				acc := 0
				if f.esPublico {
					acc |= accPublic
				}
				if f.esPrivado {
					acc |= accPrivate
				}
				if f.esProtegido {
					acc |= accProtected
				}
				if f.esEstatico {
					acc |= accStatic
				}
				if f.esFinal {
					acc |= accFinal
				}
				// solo un `static final` es constante de compilación: un `val$x` final de instancia se
				// lee con `getfield` y no se dobla (5.14)
				cr := campoResuelto{desc: d, acceso: acc, dueño: dueño, constante: f.esFinal && f.esEstatico}
				if cr.constante {
					lit, esLit := f.valor.(jlit)
					if !esLit {
						return campoResuelto{}, fmt.Errorf("el campo %s.%s dice ser final pero su inicializador no es un literal", actual, nombre)
					}
					if lit.tipoJ != 'i' && lit.tipoJ != 'c' {
						return campoResuelto{}, fmt.Errorf("doblar una constante %s.%s que no es un entero no esta medida", actual, nombre)
					}
					// un `static final char` lleva `ConstantValue` con una entrada `Integer`: medido en
					// `Letras`, `CONSTE` es `ConstantValue: int 90` (parrafo 87)
					cr.valor = lit.entero
				}
				return cr, nil
			}
			if actual == "java/lang/Object" {
				break
			}
			p, err := ctx.padreDe(propia)
			if err != nil {
				return campoResuelto{}, err
			}
			actual = p
			continue
		}
		c, err := ctx.busc.claseDelJar(actual)
		if err != nil {
			return campoResuelto{}, err
		}
		for _, f := range c.Fields {
			if f.Name != nombre {
				continue
			}
			cr := campoResuelto{desc: f.Desc, acceso: f.Access, dueño: dueño}
			if f.HasConst {
				if f.ConstTag != 3 {
					return campoResuelto{}, fmt.Errorf("la constante %s.%s no es un entero: doblarla no esta medida", actual, nombre)
				}
				cr.constante = true
				cr.valor = f.ConstInt
			}
			return cr, nil
		}
		if c.Super == "" {
			break
		}
		actual = internoDeDescriptor(c.Super)
	}
	return campoResuelto{}, fmt.Errorf("el campo %s no esta en %s ni en sus padres", nombre, dueño)
}

// accesorPara devuelve el `access$NNN` del exterior con el que una anonima toca un miembro privado, y lo
// apunta si es la primera vez. Medido en el parrafo 72: el nombre es `access$` + posicion*100 + sentido
// (0 para leer un campo o llamar a un metodo, 2 para escribir un campo), con tres digitos
// (`access$000`, `access$102`); la posicion es el orden en que javac crea los accesores, compartido
// entre campos y metodos.
func (ctx *contextoJava) accesorPara(duenio, miembro, miembroDesc string, campo, escritura, estatico bool) (string, string) {
	clave := fmt.Sprintf("%s|%s|%s|%v|%v|%v", duenio, miembro, miembroDesc, campo, escritura, estatico)
	if a, ok := ctx.accesorID[clave]; ok {
		return a.nombre, a.desc
	}
	memoria := fmt.Sprintf("%s|%s|%s|%v|%v", duenio, miembro, miembroDesc, campo, estatico)
	numero, ok := ctx.accesorNumero[memoria]
	if !ok {
		// la posición cuenta los miembros del MISMO exterior, no el total del proyecto: medido con
		// `D` y `E` juntas en el párrafo 83, donde javac da `access$000`/`access$002` en cada una y el
		// contador global daba `access$200`/`access$202` en la segunda
		numero = 0
		for k := range ctx.accesorNumero {
			if strings.HasPrefix(k, duenio+"|") {
				numero++
			}
		}
		ctx.accesorNumero[memoria] = numero
	}
	// el ultimo digito es el sentido: 0 para leer un campo o llamar a un metodo, 2 para escribir un
	// campo. Medido: `access$300` es el metodo del tercer miembro, no `access$301` (parrafo 72).
	sentido := 0
	if campo && escritura {
		sentido = 2
	}
	nombre := fmt.Sprintf("access$%03d", numero*100+sentido)
	var desc string
	switch {
	case campo && estatico && escritura:
		desc = "(" + miembroDesc + ")" + miembroDesc
	case campo && estatico:
		desc = "()" + miembroDesc
	case campo && escritura:
		desc = "(L" + duenio + ";" + miembroDesc + ")" + miembroDesc
	case campo:
		desc = "(L" + duenio + ";)" + miembroDesc
	case estatico:
		desc = miembroDesc
	default:
		desc = "(L" + duenio + ";" + miembroDesc[1:]
	}
	a := accesorSintetico{nombre: nombre, desc: desc, duenio: duenio, miembro: miembro,
		miembroDesc: miembroDesc, campo: campo, escritura: escritura, estatico: estatico}
	ctx.accesores[duenio] = append(ctx.accesores[duenio], a)
	ctx.accesorID[clave] = a
	return nombre, desc
}

// ----- la entrada: un fuente, sus clases -----
type claseEmitida struct {
	interno string
	datos   []byte
}

// fuenteJava es un fichero del proyecto: su nombre, que va en el SourceFile, y su contenido.
type fuenteJava struct {
	nombre    string
	contenido string
}

// compilarJava traduce un solo fuente del subconjunto a sus `.class`.
func compilarJava(fuente, archivo, rutaJar string) ([]claseEmitida, error) {
	return compilarProyectoJava([]fuenteJava{{nombre: archivo, contenido: fuente}}, rutaJar)
}

// compilarProyectoJava traduce el proyecto entero de una. Hace falta que sean varios ficheros a la vez por
// la razon del parrafo 40: `R.layout.main` se dobla en el llamador, y para saber el valor hay que tener la
// declaracion de la R delante, que vive en otro fichero. Cada fichero aporta su paquete y sus imports, que
// es el alcance con el que se resuelven sus nombres; las clases de todos se registran antes de emitir nada.
func compilarProyectoJava(fuentes []fuenteJava, rutaJar string) ([]claseEmitida, error) {
	busc, err := abrirBuscador(rutaJar)
	if err != nil {
		return nil, err
	}
	defer busc.cerrar()
	ctx := &contextoJava{busc: busc, propias: map[string]*jclas{},
		accesores: map[string][]accesorSintetico{}, accesorID: map[string]accesorSintetico{},
		accesorNumero: map[string]int{}}
	type parseado struct {
		a    *archivoJava
		nomb string
	}
	var todos []parseado
	for _, f := range fuentes {
		a, err := parsearJava(f.contenido)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.nombre, err)
		}
		for _, c := range a.clases {
			ctx.registrar(c)
		}
		todos = append(todos, parseado{a, baseDeNombre(f.nombre)})
	}
	var salida []claseEmitida
	for _, p := range todos {
		ctx.paquete = p.a.paquete
		ctx.imports = p.a.imports
		ctx.archivo = p.nomb
		for _, c := range p.a.clases {
			hechas, err := ctx.emmitirArbol(c)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p.nomb, err)
			}
			salida = append(salida, hechas...)
		}
	}
	// El orden del vector es el de las rutas con el sufijo `.class` dentro, que es el orden en que `leeClases`
	// presentaria los ficheros y por tanto el orden de las class_defs del dex: MainActivity$1.class antes que
	// MainActivity.class (el `$` es 0x24 y el punto 0x2e), y la R despues de sus anidadas.
	sort.SliceStable(salida, func(i, j int) bool {
		return salida[i].interno+".class" < salida[j].interno+".class"
	})
	return salida, nil
}

// baseDeNombre: el SourceFile que javac anota es el nombre del fuente, sin la ruta del proyecto.
func baseDeNombre(nombre string) string {
	if i := strings.LastIndexAny(nombre, "/\\"); i >= 0 {
		return nombre[i+1:]
	}
	return nombre
}

// emmitirArbol escribe la clase y sus anidadas: cada una es su propio fichero. Las hijas van **antes** que
// la clase porque al emitirlas se apuntan los `access$NNN` del exterior que una anonima necesita, y esos
// metodos sinteticos se escriben al final del ClassFile del exterior (medido en el parrafo 72).
func (ctx *contextoJava) emmitirArbol(c *jclas) ([]claseEmitida, error) {
	salida := []claseEmitida{}
	// las anonimas que crea el cuerpo de esta clase son su propio fichero, con su captura
	for _, a := range anonimasDe(c) {
		hijas, err := ctx.emmitirArbol(a)
		if err != nil {
			return nil, err
		}
		salida = append(salida, hijas...)
	}
	for _, d := range c.interior {
		hijas, err := ctx.emmitirArbol(d)
		if err != nil {
			return nil, err
		}
		salida = append(salida, hijas...)
	}
	datos, err := ctx.emmitirClase(c)
	if err != nil {
		return nil, fmt.Errorf("linea %d: %w", c.linea, err)
	}
	return append(salida, claseEmitida{interno: internoDe(c.paquete, c.nombre), datos: datos}), nil
}

// ----- el emisor de una clase -----

// emisor lleva el pool de la clase, lo que es de la clase y lo que es del metodo que se esta emitiendo.
type emisor struct {
	ctx      *contextoJava
	pool     *poolBuilder
	c        *jclas
	interno  string
	menciona []string // nombres con `$` que el codigo usa, en el orden del primer uso
	vistas   map[string]bool

	// del metodo actual
	estatico  bool
	desc      string
	vuelta    string
	locales   map[string]localVar
	siguiente int
	codigo    []byte
	lineas    []parLinea
	prof      int
	maxProf   int
	maxLoc    int
	// truncado avisa que un indice del pool no cabia en el byte de un `ldc`: el fichero ya no es correcto y
	// el metodo se rechaza por nombre en vez de escribir un operando roto.
	truncado bool

	// las branquias dejan el operando en blanco y lo rellenan al cerrar la etiqueta: el desplazamiento
	// del JVMS se cuenta en bytes desde el principio de la instruccion, así que no se conoce hasta que
	// esta escrito el cuerpo del `if` (parrafo 52)
	parches           []parcheSalto
	parches4          []parcheSalto4
	etiquetas         map[int]int
	siguienteEtiqueta int
	// quiebres y continuar son las pilas de destinos de `break` y `continue`: cada bucle y cada
	// switch apila el suyo mientras emite su cuerpo (parrafo 70)
	quiebres  []int
	continuar []int
	// excepciones es la exception_table del Code (parrafo 71)
	excepciones []entradaTry
	// finalmente es la pila de `finally` vivos: un `return` o un `break` que los cruza copia el bloque
	// antes de salir (parrafo 76)
	finalmente []*pendienteTry
}

type parcheSalto struct {
	en       int
	etiqueta int
	op       byte
}

// parcheSalto4: un desplazamiento de 32 bits con signo, como el `default` y cada destino de un
// `tableswitch`/`lookupswitch`. Se cuenta desde `base`, que es el principio de la instrucción de
// switch, igual que en las branquias de dos bytes.
type parcheSalto4 struct {
	en       int
	base     int
	etiqueta int
}

type localVar struct {
	indice int
	desc   string
}

type parLinea struct {
	desplazamiento int
	linea          int
}

func (e *emisor) emite(b ...byte) { e.codigo = append(e.codigo, b...) }

func (e *emisor) sube() {
	e.prof++
	if e.prof > e.maxProf {
		e.maxProf = e.prof
	}
}

// subeDos: empujar un `long` (o un `double` cuando se mida), que ocupa DOS palabras de pila. La JVM lo
// exige: con `max_stack=1` y un `ldc2_w` escribe `VerifyError: Operand stack overflow … Exceeded max
// stack size` (medido al primera sonda `L` del parrafo 74, y javac pone `stack=2` para un solo largo).
func (e *emisor) subeDos() {
	e.prof += 2
	if e.prof > e.maxProf {
		e.maxProf = e.prof
	}
}

// subePor: una palabra, o dos si el descriptor es ancho.
func (e *emisor) subePor(desc string) {
	if esAncho(desc) {
		e.subeDos()
		return
	}
	e.sube()
}

// bajaPor: el contrario de subePor.
func (e *emisor) bajaPor(desc string) {
	if esAncho(desc) {
		e.baja(2)
		return
	}
	e.baja(1)
}

// palabrasDeArgs: cuanto ocupa en la pila la lista de argumentos de un descriptor, que es lo que
// `invoca` baja: un `long` cuenta por dos (parrafo 74).
func palabrasDeArgs(desc string) int {
	_, _, ps := splitDescriptor(desc)
	n := 0
	for _, p := range ps {
		if esAncho(p) {
			n += 2
			continue
		}
		n++
	}
	return n
}

func (e *emisor) baja(n int) {
	e.prof -= n
}

// nuevaEtiqueta: un destino de salto nuevo. Las etiquetas son del metodo y se rellenan al cerrarse.
func (e *emisor) nuevaEtiqueta() int {
	n := e.siguienteEtiqueta
	e.siguienteEtiqueta++
	return n
}

// saltoEmite escribe la instruccion de salto con sus dos bytes de operando reservados y los deja
// apuntados a la etiqueta. `consume` dice cuántas palabras baja de la pila simulada: dos en las
// comparaciones entre dos valores, una en las formas contra cero y contra null, ninguna en el `goto`.
func (e *emisor) saltoEmite(op byte, destino int, consume int) {
	e.emite(op, 0, 0)
	e.parches = append(e.parches, parcheSalto{en: len(e.codigo) - 3, etiqueta: destino, op: op})
	e.baja(consume)
}

// cierra anota la posicion actual como destino de la etiqueta y completa los saltos que la esperaban.
// El operando JVM es un desplazamiento en bytes contados desde el principio de la instruccion, no
// desde el operando: por eso se guarda `en` en la direccion de la branquia y se escribe en en+1.
func (e *emisor) cierra(id int) {
	if e.etiquetas == nil {
		e.etiquetas = map[int]int{}
	}
	e.etiquetas[id] = len(e.codigo)
	for _, p := range e.parches {
		if p.etiqueta != id {
			continue
		}
		rel := len(e.codigo) - p.en
		e.codigo[p.en+1] = byte(rel >> 8)
		e.codigo[p.en+2] = byte(rel)
	}
	quedan := e.parches4[:0]
	for _, p := range e.parches4 {
		if p.etiqueta != id {
			quedan = append(quedan, p)
			continue
		}
		rel := uint32(int32(len(e.codigo) - p.base))
		e.codigo[p.en] = byte(rel >> 24)
		e.codigo[p.en+1] = byte(rel >> 16)
		e.codigo[p.en+2] = byte(rel >> 8)
		e.codigo[p.en+3] = byte(rel)
	}
	e.parches4 = quedan
}

// cuatro: reserva un desplazamiento de 32 bits que se completa al cerrarse la etiqueta.
func (e *emisor) cuatro(etiqueta, base int) {
	e.parches4 = append(e.parches4, parcheSalto4{en: len(e.codigo), base: base, etiqueta: etiqueta})
	e.emite(0, 0, 0, 0)
}

// cuatroConstante: un entero de 32 bits con signo, como las claves y los extremos del switch.
func (e *emisor) cuatroConstante(v int32) {
	u := uint32(v)
	e.emite(byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

// encadenaGotos: un `goto` cuyo destino es una posicion que empieza por otro `goto` se resuelve
// contra el segundo. Lo hace javac, leido en el parrafo 52 con un `if` anidado al final del `then`
// de otro `if` con `else`: escribe `13: goto 27`, mientras que el primer borrador nuestro escribia
// `13: goto 20` y en 20 estaba el `goto 27` del bloque. La etiqueta intermedia no separa ninguna
// instruccion, es un salto vacio, y encadenarlo no cambia por donde pasa el programa.
func (e *emisor) encadenaGotos() {
	if len(e.parches) == 0 || len(e.etiquetas) == 0 {
		return
	}
	gotoEn := map[int]int{} // posicion de un goto -> etiqueta a la que apunta
	for _, p := range e.parches {
		if p.op == opGOTO {
			gotoEn[p.en] = p.etiqueta
		}
	}
	for i, p := range e.parches {
		if p.op != opGOTO {
			continue
		}
		donde, ok := e.etiquetas[p.etiqueta]
		if !ok {
			continue
		}
		for k := 0; k < len(e.parches); k++ { // la cota es el numero de saltos: no hay cadenas ciclicas
			siguiente, esGoto := gotoEn[donde]
			if !esGoto {
				break
			}
			posicion, cerrada := e.etiquetas[siguiente]
			if !cerrada {
				break
			}
			donde = posicion
			e.parches[i].etiqueta = siguiente
		}
		rel := donde - p.en
		e.codigo[p.en+1] = byte(rel >> 8)
		e.codigo[p.en+2] = byte(rel)
	}
}

// marcaLinea: la LineNumberTable de javac apunta al desplazamiento donde empieza cada sentencia, medido en
// el parrafo 40 (line 14:0, 15:5, 16:11 en el onCreate de la plantilla). Un metodo escrito en una sola
// linea del fuente lleva UNA fila, leido en el parrafo 52: javac no repite el numero de linea.
func (e *emisor) marcaLinea(linea int) {
	if linea <= 0 {
		return
	}
	if len(e.lineas) > 0 && e.lineas[len(e.lineas)-1].linea == linea {
		return
	}
	e.lineas = append(e.lineas, parLinea{desplazamiento: len(e.codigo), linea: linea})
}

func (e *emisor) reserva(nombre string, desc string) int {
	i := e.siguiente
	paso := 1
	if esAncho(desc) {
		// `long` ocupa DOS huecos: medido en `javap -v` de la sonda `F`, donde `conRama(long a, int n)`
		// declara `locals=5` y el local `long x` cae en el hueco 3 (parrafo 74)
		paso = 2
	}
	e.siguiente += paso
	if e.siguiente > e.maxLoc {
		e.maxLoc = e.siguiente
	}
	if nombre != "" {
		e.locales[nombre] = localVar{indice: i, desc: desc}
	}
	return i
}

func (e *emisor) cargaDe(indice int, desc string) {
	breve, conIndice := carga(desc[0])
	if indice <= 3 {
		e.emite(breve + byte(indice))
	} else {
		e.emite(conIndice, byte(indice))
	}
	e.subePor(desc)
}

func (e *emisor) guardaDe(indice int, desc string) {
	breve, conIndice := guarda(desc[0])
	if indice <= 3 {
		e.emite(breve + byte(indice))
	} else {
		e.emite(conIndice, byte(indice))
	}
	e.bajaPor(desc)
}

// cargaEntero: iconst, bipush, sipush o ldc con el indice de un Integer del pool, igual que javac.
func (e *emisor) cargaEntero(v int32) {
	switch {
	case v == -1:
		e.emite(0x02)
	case v >= 0 && v <= 5:
		e.emite(byte(0x03 + v))
	case v >= -128 && v <= 127:
		e.emite(0x10, byte(v))
	case v >= -32768 && v <= 32767:
		e.emite(0x11, byte(v>>8), byte(v))
	default:
		e.ldc(e.pool.entero(v))
	}
	e.sube()
}

// cargaConstante: la forma corta que le corresponde a un valor ya plegado y ya promocionado. Medida
// método por método en el párrafo 85: depende del **valor y del tipo promocionado**, no del token
// escrito —`d + 1` es `dconst_1`, `l + 1` es `lconst_1`, `l + 2` es `ldc2_w` porque no existe
// `lconst_2`, y `d * 2` es `ldc2_w` porque javac **nunca** usa `dconst_2` aunque exista (0x10)—. El
// signo es parte de la constante: `-0.0f` va por `ldc`, no por `fconst_0`.
func (e *emisor) cargaConstante(c constNum) {
	switch c.tipo {
	case 'I':
		e.cargaEntero(int32(c.entero))
	case 'J':
		switch c.entero {
		case 0:
			e.emite(0x09)
		case 1:
			e.emite(0x0a)
		default:
			idx := e.pool.largo8(c.entero)
			e.emite(0x14, byte(idx>>8), byte(idx))
		}
		e.subePor("J")
	case 'F':
		switch {
		case c.real == 0 && !math.Signbit(c.real):
			e.emite(0x0b)
		case c.real == 1:
			e.emite(0x0c)
		case c.real == 2:
			e.emite(0x0d)
		default:
			e.ldc(e.pool.flotante4(c.real))
		}
		e.sube()
	case 'D':
		switch {
		case c.real == 0 && !math.Signbit(c.real):
			e.emite(0x0e)
		case c.real == 1:
			e.emite(0x0f)
		default:
			idx := e.pool.doble8(c.real)
			e.emite(0x14, byte(idx>>8), byte(idx))
		}
		e.subePor("D")
	}
}

// ldc: `ldc` lleva el indice en un byte; a partir de 256 javac usa `ldc_w`. Un fichero de la pagina no
// llega, y si lo hiciera es mejor nombrarlo que escribir el indice truncado.
func (e *emisor) ldc(idx int) {
	if idx > 255 {
		e.truncado = true
		e.emite(opLdc, 0)
		return
	}
	e.emite(opLdc, byte(idx))
}

// usar anota una clase anidada mencionada, para que su entrada llegue al InnerClasses. El orden es el de
// primer uso: javac escribe las filas asi, medido en el parrafo 42 (R$layout, MainActivity$1, la interface).
func (e *emisor) usar(internal string) {
	if !strings.Contains(internal, "$") {
		return
	}
	if e.vistas[internal] {
		return
	}
	e.vistas[internal] = true
	e.menciona = append(e.menciona, internal)
}

// getfield: el receptor se come y sale el valor del campo, asi que la pila no cambia de alto. getstatic, en
// cambio, no come nada y sube uno.
func (e *emisor) getfield(idx int) {
	e.emite(opGetfield, byte(idx>>8), byte(idx))
	e.baja(1)
	e.sube()
}

// putstatic se come el receptor y el valor; putstatic solo el valor. Medido en `javap -c`: el `<init>` de
// `private int oculto = 3` es `aload_0; iconst_3; putfield #7`, y el `<clinit>` de `static int shared = 5`
// es `iconst_5; putstatic #53`, sin receptor.
func (e *emisor) putfield(idx int) {
	e.emite(opPutfield, byte(idx>>8), byte(idx))
	e.baja(2)
}

func (e *emisor) putstatic(idx int) {
	e.emite(opPutstatic, byte(idx>>8), byte(idx))
	e.baja(1)
}

// Las cuatro formas con descriptor: un campo `long` (o `double`) mueve DOS palabras, no una. Medido en
// `javap -v` de la sonda `L`, cuyo `<clinit>` es `ldc2_w #17; putstatic #14` con `stack=2` (parrafo 74).
func (e *emisor) getfieldDe(idx int, desc string) {
	e.getfield(idx)
	if esAncho(desc) {
		e.sube()
	}
}

func (e *emisor) putfieldDe(idx int, desc string) {
	e.putfield(idx)
	if esAncho(desc) {
		e.baja(1)
	}
}

func (e *emisor) getstaticDe(idx int, desc string) {
	e.emite(opGetstatic, byte(idx>>8), byte(idx))
	e.subePor(desc)
}

func (e *emisor) putstaticDe(idx int, desc string) {
	e.putstatic(idx)
	if esAncho(desc) {
		e.baja(1)
	}
}

// inicializador escribe el valor de un campo con su destino, en la forma que javac deja en el binario y con
// la LineNumberTable apuntando a la linea del campo (medido: `line 9: 4`, donde 4 es el `aload_0`).
func (e *emisor) inicializador(f jcampo) error {
	desc, err := e.ctx.descriptorDe(f.tipo)
	if err != nil {
		return fmt.Errorf("linea %d: %w", f.donde.linea, err)
	}
	e.marcaLinea(f.donde.linea)
	idx := e.pool.campo(e.interno, f.nombre, desc)
	if !f.esEstatico {
		e.cargaDe(0, "L")
	}
	d, err := e.exp(f.valor)
	if err != nil {
		return err
	}
	if d != desc {
		return fmt.Errorf("linea %d: el campo %s es %s y su inicializador deja %s", f.donde.linea, f.nombre, desc, d)
	}
	if f.esEstatico {
		e.putstaticDe(idx, desc)
	} else {
		e.putfieldDe(idx, desc)
	}
	return nil
}

// invoca escribe el opcode con su indice (y el conteo, si es invokeinterface) y ajusta la pila simulada.
func (e *emisor) invoca(opcode byte, dueño, nombre, desc string, enPila int) {
	esInterface := opcode == opInvointf
	var idx int
	if esInterface {
		idx = e.pool.metodoInterface(dueño, nombre, desc)
	} else {
		idx = e.pool.metodo(dueño, nombre, desc)
	}
	e.usar(dueño)
	e.usarDescriptor(desc)
	e.emite(opcode, byte(idx>>8), byte(idx))
	if esInterface {
		// el byte de `invokeinterface` son PALABRAS de argumento incluido el receptor (JVMS 6.5): un
		// `long` cuenta dos, que no es el `enPila` de los llamadores cuando hay un ancho de por medio
		e.emite(byte(palabrasDeArgs(desc)+1), 0)
	}
	e.baja(palabrasDeArgs(desc) + enPilaReceptor(opcode))
	if v := vueltaDe(desc); v != "" && v != "V" {
		e.subePor(v)
	}
}

// enPilaReceptor: el receptor de una llamada virtual, especializada o de constructor es una palabra;
// el estatico no lleva. `enPila` de los llamadores sigue contando argumentos y receptor por separado.
func enPilaReceptor(opcode byte) int {
	if opcode == opInvokestat {
		return 0
	}
	return 1
}

// ----- expresiones -----

// lineaDeExpr: donde estaba la expresion, para la LineNumberTable y para los mensajes.
func lineaDeExpr(x jexpr) int {
	switch v := x.(type) {
	case jlit:
		return v.donde.linea
	case jclase:
		return v.donde.linea
	case jthis:
		return v.donde.linea
	case jacceso:
		return v.donde.linea
	case jllamada:
		return v.donde.linea
	case jcast:
		return v.donde.linea
	case jinstanceof:
		return v.donde.linea
	case jnuevo:
		return v.donde.linea
	case jnuevoArreglo:
		return v.donde.linea
	case jindice:
		return v.donde.linea
	case jincr:
		return v.donde.linea
	case joperador:
		return v.donde.linea
	case jnegacion:
		return v.donde.linea
	}
	return 0
}

// exp emite la expresion y devuelve el descriptor de lo que deja en la pila.
func (e *emisor) exp(x jexpr) (string, error) {
	switch v := x.(type) {
	case jlit:
		switch v.tipoJ {
		case 'i':
			e.cargaEntero(v.entero)
			return "I", nil
		case 'c':
			// el `char` en la pila JVM ES un entero: medido en `Letras` (parrafo 87), `'\n'` sale
			// `bipush 10; ireturn`, `'á'` es `sipush 225` y `'\uFFFF'` es `ldc` con la entrada `Integer`
			// 65535. Lo único que lo distingue del `int` es el descriptor (`C`) y los opcodes de arreglo
			// (`caload`/`castore`) y del cast (`i2c`).
			e.cargaEntero(v.entero)
			return "C", nil
		case 'b':
			if v.booleano {
				e.emite(0x04)
			} else {
				e.emite(0x03)
			}
			e.sube()
			return "Z", nil
		case 'j':
			// medido en `javap -c` de `W`: `long x = 0` es `lconst_0` (0x09), `1L` es `lconst_1`
			// (0x0a) y cualquier otro va por `ldc2_w` (0x14) con el indice de una entrada `Long`
			e.cargaConstante(constNum{tipo: 'J', entero: v.largo})
			return "J", nil
		case 'f':
			// medido en `javap -c` de `FyD` (parrafo 77): `0.0f` es `fconst_0` (0x0b) y cualquier otro
			// va por `ldc` con una entrada `Float` del pool, que ocupa **una** posicion (`1.5f` sale
			// `ldc #25`). El signo entra en la constante: `-0.0f` es `ldc`, no `fconst_0` (parrafo 85)
			e.cargaConstante(constNum{tipo: 'F', real: float64(float32(v.flotante))})
			return "F", nil
		case 'd':
			// `1.0` sale `dconst_1` (0x0f) y `3.14159` es `ldc2_w #7` con una entrada `Double` que
			// reserva su segunda posicion, igual que la `Long` del parrafo 74
			e.cargaConstante(constNum{tipo: 'D', real: v.flotante})
			return "D", nil
		case 's':
			e.ldc(e.pool.cadena(v.texto))
			e.sube()
			return "Ljava/lang/String;", nil
		case 'n':
			e.emite(opAconstNull)
			e.sube()
			return "Ljava/lang/Object;", nil
		}
		return "", fmt.Errorf("linea %d: un literal de clase %q no esta medido", v.donde.linea, string(v.tipoJ))
	case jclase:
		return e.literalDeClase(v)
	case jthis:
		if v.nombre != "" {
			// `Exterior.this` es leer el campo de captura: medido en el parrafo 42, `aload_0; getfield
			// this$0:Lcom/hola/apkcstudio/MainActivity;` dentro de MainActivity$1
			if !e.c.anonima || sinPaquete(e.encierreInterno()) != v.nombre {
				return "", fmt.Errorf("linea %d: `%s.this` solo se mide dentro de la anonima que ese exterior crea", v.donde.linea, v.nombre)
			}
			externo := "L" + e.encierreInterno() + ";"
			e.cargaDe(0, "L")
			e.getfield(e.pool.campo(e.interno, "this$0", externo))
			return externo, nil
		}
		if e.estatico {
			return "", fmt.Errorf("linea %d: `this` en un metodo estatico no es Java", v.donde.linea)
		}
		e.cargaDe(0, "L")
		return "L" + e.interno + ";", nil
	case jinstanceof:
		// medido en `Ins.esCadena` (parrafo 79): `aload_0; instanceof #7 // class java/lang/String;`
		// y despues el `ifeq` de la condicion —el `instanceof` consume la referencia y deja un entero,
		// así que el alto de pila no cambia—. La clase va al pool como la de `checkcast`, con el
		// indice en dos bytes.
		d, err := e.exp(v.dentro)
		if err != nil {
			return "", err
		}
		if len(d) == 0 || d[0] != 'L' {
			return "", fmt.Errorf("linea %d: `instanceof` sobre %s: solo se midio sobre una referencia", v.donde.linea, d)
		}
		// `o instanceof String[]` y `o instanceof int[]`: el tipo del `instanceof` es el descriptor del
		// arreglo (`[Ljava/lang/String;`, `[I`), medido en la sonda `E` del §131
		interno := ""
		if strings.Contains(v.tipo, "[") {
			desc, err := e.ctx.descriptorDe(v.tipo)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
			}
			if !strings.HasPrefix(desc, "[") {
				return "", fmt.Errorf("linea %d: `instanceof %s` no es un arreglo ni una clase", v.donde.linea, v.tipo)
			}
			interno = desc
		} else {
			interno, err = e.ctx.tipoAInterno(v.tipo)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
			}
		}
		e.usar(interno)
		idx := e.pool.clase(interno)
		e.emite(0xc1, byte(idx>>8), byte(idx))
		return "Z", nil
	case jcast:
		if destino, ok := descPrimitivo[v.tipo]; ok && destino == "C" {
			// `(char) x`, medido en `Letras` (parrafo 87): `resta` es `iload_0; iload_1; isub; i2c;
			// ireturn` y `cambia` `iload_0; iconst_1; iadd; i2c; istore_0`, con `i2c` = 0x92. Es una
			// palabra dentro y una fuera, asi que la pila no se mueve. Desde un ancho (`long`, `float`,
			// `double`) primero la conversion del parrafo 77 y despues el `i2c`.
			d, err := e.exp(v.dentro)
			if err != nil {
				return "", err
			}
			if d == "C" {
				return "C", nil
			}
			if len(d) != 1 || strings.IndexByte("IJFD", d[0]) < 0 {
				return "", fmt.Errorf("linea %d: se convierte %s a `char`, y eso no es una cuenta numerica medida", v.donde.linea, d)
			}
			if d != "I" {
				if !e.convierte(d, "I") {
					return "", fmt.Errorf("linea %d: la conversion de %s a entero antes del `char` no esta medida", v.donde.linea, d)
				}
			}
			e.emite(0x92)
			return "C", nil
		}
		if destino, ok := descPrimitivo[v.tipo]; ok && strings.IndexByte("IJFD", destino[0]) >= 0 {
			// un cast numerico es la conversion de la rejilla, no un `checkcast`: medido en `FyD`,
			// donde `aEntero` es `dload_0; d2i` y `aFlotante` `dload_0; d2f` (parrafo 77)
			d, err := e.exp(v.dentro)
			if err != nil {
				return "", err
			}
			if d == destino {
				return d, nil
			}
			if strings.IndexByte("IJFD", d[0]) < 0 || len(d) != 1 {
				return "", fmt.Errorf("linea %d: se convierte %s a %s, y eso no es una cuenta numerica medida", v.donde.linea, d, v.tipo)
			}
			if !e.convierte(d, destino) {
				return "", fmt.Errorf("linea %d: la conversion de %s a %s no esta medida (el `byte` y el `short` no los escribió sonda alguna; el `char` tiene su propio camino con `i2c`, parrafo 87)", v.donde.linea, d, v.tipo)
			}
			return destino, nil
		}
		interno, err := e.ctx.tipoAInterno(v.tipo)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		if _, err := e.exp(v.dentro); err != nil {
			return "", err
		}
		e.usar(interno)
		idx := e.pool.clase(interno)
		e.emite(opCheckcast, byte(idx>>8), byte(idx))
		return "L" + interno + ";", nil
	case jacceso:
		return e.acceso(v)
	case jllamada:
		return e.llamada(v)
	case jnuevo:
		return e.nuevo(v)
	case jnuevoArreglo:
		return e.nuevoArreglo(v)
	case jindice:
		return e.leeIndice(v)
	case jincr:
		return "", fmt.Errorf("linea %d: `++`/`--` como valor no esta medido; uselo como sentencia", v.donde.linea)
	case joperador:
		// aritmetica entera, leida de `javap -c` sobre `a*b`, `a/b`, `a%b`, `a+b`, `a-b` (parrafo 49):
		// dos palabras dentro, una fuera. Antes de emitir se prueba el plegado de constantes, que javac
		// hace (`1 + 2` sale `iconst_3`, `1 + 2 * 3` sale `bipush 7`, medido en el parrafo 53).
		if v.op == "+" && e.esConcatenacion(v) {
			return e.concatenar(v)
		}
		if v.op == "==" || v.op == "!=" {
			// `return a == b` entre referencias: medido en `Clase.esCadena` del parrafo 89, javac
			// escribe `if_acmpne` 0xa6 (o `if_acmpeq` 0xa5), `iconst_1`, `goto`, `iconst_0`.
			return e.comparacionDeReferencias(v)
		}
		codes := map[string]byte{"+": opIadd, "-": opIsub, "*": opImul, "/": opIdiv, "%": opIrem}
		codesAnchas := map[string]map[string]byte{
			// la familia larga, medida en `javap -c` de `L` (parrafo 74)
			"J": {"+": 0x61, "-": 0x65, "*": 0x69, "/": 0x6d, "%": 0x71},
			// la de `float` y la de `double`, leidas de `FyD` en el parrafo 77: cada familia ocupa
			// cuatro opcodes seguidos despues del entero (fadd 0x62, dadd 0x63, fsub 0x66, ...)
			"F": {"+": 0x62, "-": 0x66, "*": 0x6a, "/": 0x6e, "%": 0x72},
			"D": {"+": 0x63, "-": 0x67, "*": 0x6b, "/": 0x6f, "%": 0x73},
		}
		code, esAritmetica := codes[v.op]
		if !esAritmetica {
			return "", fmt.Errorf("linea %d: el operador %q no esta medido en este emisor", v.donde.linea, v.op)
		}
		if plegado, esConstante := plegarConstante(v); esConstante {
			e.cargaConstante(plegado)
			return string(plegado.tipo), nil
		}
		// `int + long`: javac **ensancha el entero antes de la cuenta** y en el orden del fuente
		// (`W.suma` mide `iload_0; i2l; lload_1; ladd`, parrafo 74). Para poder meter ese `i2l` en su
		// sitio hay que saber el tipo de los dos lados **antes** de escribirlos, y eso solo da cuando
		// el tipo se puede prever sin emitir.
		tIzq, okIzq := e.tipoPrevisto(v.izq)
		tDer, okDer := e.tipoPrevisto(v.der)
		// El `char` (y el `byte`, el `short` y el `booleano`) entra en la cuenta **como entero**, sin
		// conversion: medido en `Letras`, `masUno(char c) { return c + 1; }` es `iload_0; iconst_1; iadd`
		// y `mayor(char a, char b)` usa `if_icmp*` de entero (parrafo 87).
		tIzq, tDer = enCuenta(tIzq), enCuenta(tDer)
		// `int + long`, `float * double`: javac **ensancha cada operando en su propio sitio** y en el
		// orden del fuente (`W.suma` mide `iload_0; i2l; lload_1; ladd`, parrafo 74, y `FyD.por` es
		// `fload_0; f2d; dload_1; dmul` con `FyD.divide` `dload_0; iload_2; i2d; ddiv`, parrafo 77).
		// Para meter esa conversion en su sitio hay que saber el tipo de los dos lados **antes** de
		// escribirlos, y eso solo da cuando el tipo se puede prever sin emitir. La promocion de Java es
		// doble > flotante > largo > entero.
		nivel := map[string]int{"": 0, "I": 0, "J": 1, "F": 2, "D": 3}
		ancho := ""
		if okIzq && nivel[tIzq] > nivel[ancho] {
			ancho = tIzq
		}
		if okDer && nivel[tDer] > nivel[ancho] {
			ancho = tDer
		}
		if ancho != "" {
			if !okIzq || !okDer {
				return "", fmt.Errorf("linea %d: `%s` con un %s y un operando cuyo tipo no se puede saber antes de escribirlo: la conversion va en el sitio de javac y adivinarlo aqui no esta medido",
					v.donde.linea, v.op, ancho)
			}
			di, err := e.expA(v.izq, ancho)
			if err != nil {
				return "", err
			}
			dd, err := e.expA(v.der, ancho)
			if err != nil {
				return "", err
			}
			if di != ancho || dd != ancho {
				return "", fmt.Errorf("linea %d: `%s` aplicado a %s y %s: la cuenta ancha solo se midio entre enteros, largos, flotantes y dobles",
					v.donde.linea, v.op, di, dd)
			}
			e.emite(codesAnchas[ancho][v.op])
			// dos elementos dentro y uno fuera: cuatro palabras a dos con `J`/`D`, dos a uno con `F`
			if ancho == "F" {
				e.baja(1)
			} else {
				e.baja(2)
			}
			return ancho, nil
		}
		// `expA` con el entero deseado: un operando `char` entra en la cuenta sin escribir ninguna
		// conversion (`masUno(char c) { return c + 1; }` es `iload_0; iconst_1; iadd`, parrafo 87) y la
		// constante del otro lado se pliega ya en el tipo de la cuenta (parrafo 53).
		izq, err := e.expA(v.izq, "I")
		if err != nil {
			return "", err
		}
		der, err := e.expA(v.der, "I")
		if err != nil {
			return "", err
		}
		if izq == "Ljava/lang/String;" || der == "Ljava/lang/String;" {
			return "", fmt.Errorf("linea %d: `%s` con una cadena cuyo tipo no se puede estimar sin emitirla (una llamada): la concatenacion esta medida sobre literales, campos, variables y `this`",
				v.donde.linea, v.op)
		}
		if izq != "I" || der != "I" {
			return "", fmt.Errorf("linea %d: `%s` aplicado a %s y %s: solo se midio la aritmetica de enteros",
				v.donde.linea, v.op, izq, der)
		}
		e.emite(code)
		e.baja(1)
		return "I", nil
	case jnegacion:
		return "", fmt.Errorf("linea %d: la negacion no esta medida en este emisor", v.donde.linea)
	}
	return "", fmt.Errorf("una expresion de %T no esta medida", x)
}

// internoStringBuilder: el tipo con el que javac pega cadenas en la version 52. No es un recuerdo del
// manual: del `javap -c` de la sonda del parrafo 53 sale `new #9 // class java/lang/StringBuilder`,
// `dup`, `invokespecial #11 <init>:()V`, una fila de `invokevirtual append` y `toString()Ljava/lang/String;`.
const internoStringBuilder = "java/lang/StringBuilder"

// esConcatenacion: un `+` es concatenacion cuando alguno de sus dos lados devuelve cadena. La
// agrupacion es la del fuente (izquierda asociativa): en `1 + 2 + x + "s"` el subarbol `(1+2)+x` NO es
// concatenacion y javac lo evalua como cuenta entera, para appendear su resultado con append(I).
func (e *emisor) esConcatenacion(v joperador) bool {
	return e.tieneCadena(v.izq) || e.tieneCadena(v.der)
}

// tieneCadena: el operando es cadena, o es el resultado de otra concatenacion. Hace falta porque
// `tipoEstimado` estima literales, campos, variables y `this`, pero no el resultado de un `+`: en
// `"hola" + " " + 3` el nodo de arriba tiene por izquierda a `"hola" + " "`, y esa rama ya devuelve
// cadena por si sola (parrafo 53).
func (e *emisor) tieneCadena(x jexpr) bool {
	if op, es := x.(joperador); es && op.op == "+" {
		// el subarbol se juzga con la misma regla, bajando por su espina izquierda
		if e.esConcatenacion(op) {
			return true
		}
	}
	d, err := e.tipoEstimado(x)
	return err == nil && d == "Ljava/lang/String;"
}

// segmentosDeConcat: los operandos en el orden en que javac los appendea, bajando por la espina
// izquierda mientras el hijo siga siendo concatenacion.
func (e *emisor) segmentosDeConcat(v joperador) []jexpr {
	if hijo, es := v.izq.(joperador); es && hijo.op == "+" && e.esConcatenacion(hijo) {
		return append(e.segmentosDeConcat(hijo), v.der)
	}
	return []jexpr{v.izq, v.der}
}

// textoLiteral: el texto con el que javac dobla un literal dentro de una concatenacion: la cadena tal
// cual, o el entero escrito en decimal. Medido con `"hola" + " " + 3` (una sola constante "hola 3") y
// `"x" + 2 + 3 + n` (`ldc "x23"` y despues `iload_1; append(I)`).
func textoLiteral(x jexpr) (string, bool) {
	lit, es := x.(jlit)
	if !es {
		return "", false
	}
	switch lit.tipoJ {
	case 's':
		return lit.texto, true
	case 'i':
		return strconv.Itoa(int(lit.entero)), true
	}
	return "", false
}

// concatenar escribe la cadena de StringBuilder. Tres reglas leidas del javap de la sonda: los literales
// adjacentes se doblan en UNA constante; si despues del plegado no queda mas que una cadena no hay
// StringBuilder (`"hola" + 3` es un solo `ldc`); y el primer operando se appendea como los demas, porque
// javac no usa el constructor `StringBuilder(String)`.
func (e *emisor) concatenar(v joperador) (string, error) {
	piezas := []jexpr{}
	texto := ""
	hayTexto := false
	for _, s := range e.segmentosDeConcat(v) {
		if t, esLiteral := textoLiteral(s); esLiteral {
			texto += t
			hayTexto = true
			continue
		}
		if hayTexto {
			piezas = append(piezas, jlit{tipoJ: 's', texto: texto, donde: v.donde})
			texto, hayTexto = "", false
		}
		piezas = append(piezas, s)
	}
	if hayTexto {
		piezas = append(piezas, jlit{tipoJ: 's', texto: texto, donde: v.donde})
	}
	if len(piezas) == 1 {
		if t, soloLiterales := textoLiteral(piezas[0]); soloLiterales {
			e.ldc(e.pool.cadena(t))
			e.sube()
			return "Ljava/lang/String;", nil
		}
	}
	appends := map[string]string{
		"I":                  "(I)Ljava/lang/StringBuilder;",
		"Ljava/lang/String;": "(Ljava/lang/String;)Ljava/lang/StringBuilder;",
	}
	idx := e.pool.clase(internoStringBuilder)
	e.emite(opNew, byte(idx>>8), byte(idx))
	e.sube()
	e.emite(opDup)
	e.sube()
	// el dup deja dos referencias: el constructor se come una y la otra es la cadena viva
	e.invoca(opInvokespec, internoStringBuilder, "<init>", "()V", 1)
	for _, p := range piezas {
		d, err := e.exp(p)
		if err != nil {
			return "", err
		}
		desc, medida := appends[d]
		if !medida {
			if d != "" && (d[0] == 'L' || d[0] == '[') {
				// medido con `String.valueOf(o)` implicito: `append:(Ljava/lang/Object;)`
				desc = "(Ljava/lang/Object;)Ljava/lang/StringBuilder;"
			} else {
				return "", fmt.Errorf("linea %d: concatenar una expresion de tipo %s: solo se midieron entero, cadena y referencia (append(Z), append(C), append(J) y append(D) no los leyimos)",
					v.donde.linea, d)
			}
		}
		e.invoca(opInvokevirt, internoStringBuilder, "append", desc, 2)
	}
	e.invoca(opInvokevirt, internoStringBuilder, "toString", "()Ljava/lang/String;", 1)
	return "Ljava/lang/String;", nil
}

// constNum: una expresion numerica que javac pliega a una constante, con el tipo que le da la
// promocion binaria ('I', 'J', 'F', 'D'). El valor entero va en `entero` y el del punto en `real`
// (almacenado ya con la precision del tipo: un `F` cabe en un float32).
type constNum struct {
	tipo   byte
	entero int64
	real   float64
}

// nivelNum: el orden de la promocion de Java, doble > flotante > largo > entero (parrafo 77).
func nivelNum(t byte) int {
	switch t {
	case 'J':
		return 1
	case 'F':
		return 2
	case 'D':
		return 3
	}
	return 0
}

// enCuenta: el descriptor con el que un tipo entra en una cuenta o en una comparación. El `char` (y el
// `byte`, el `short` y el `booleano`) entran como `int` y **sin conversion**: medido en `Letras`,
// `masUno(char c) { return c + 1; }` es `iload_0; iconst_1; iadd` y `mayor(char a, char b)` usa
// `if_icmp*` (parrafo 87). Lo contrario —estrechar— sí escribe `i2c` y es el cast del mismo párrafo.
func enCuenta(d string) string {
	if len(d) == 1 && strings.IndexByte("BCSZ", d[0]) >= 0 {
		return "I"
	}
	return d
}

// comoReal: el valor de la constante visto por una cuenta de punto flotante.
func (c constNum) comoReal() float64 {
	if c.tipo == 'I' || c.tipo == 'J' {
		return float64(c.entero)
	}
	return c.real
}

// promocionA: el mismo valor escrito en el tipo al que Java lo ensancha. El destino nunca es mas
// estrecho: quien llama comprueba el nivel antes.
func (c constNum) promocionA(t byte) constNum {
	if c.tipo == t {
		return c
	}
	r := constNum{tipo: t}
	switch t {
	case 'J':
		r.entero = c.entero
	case 'F':
		r.real = float64(float32(c.comoReal()))
	case 'D':
		r.real = c.comoReal()
	}
	return r
}

// plegarConstante: javac dobla las cuentas de literales ANTES de emitir (parrafo 53 para la entera,
// parrafo 85 para la del punto). Las formas medidas:
//
//	`1 + 2` → iconst_3, `1 + 2 * 3` → bipush 7 (iconst llega al 5), `7 - 10` → bipush -3,
//	`9 / 2` → iconst_4, `2147483647 + 1` → ldc -2147483648 (el desborde envuelve)
//	`1 / 0` y `1L / 0` NO se doblan: la excepcion tiene que saltar en ejecucion
//	`2.5f * 2` → ldc 5.0f, `1.0 / 0` → ldc2_w Infinity, `1.0 % 0` → ldc2_w NaN (el cero flotante
//	no es excepcion, es el resultado de la cuenta de IEEE)
//	`1.0f / 3.0f` → ldc 0.33333334f: la cuenta de `float` se hace en precision de float32
//	`1.5f + 2L` → ldc 3.5f y `1.0 + 2.5f` → ldc2_w 3.5d: cada operando entra ya promocionado
func plegarConstante(x jexpr) (constNum, bool) {
	switch v := x.(type) {
	case jlit:
		switch v.tipoJ {
		case 'i':
			return constNum{tipo: 'I', entero: int64(v.entero)}, true
		case 'c':
			// un `char` pliega como entero: la suma de dos `char` es `int` en Java, y javac la escribe
			// con los `iconst`/`bipush`/`sipush`/`ldc` de siempre (parrafo 87)
			return constNum{tipo: 'I', entero: int64(v.entero)}, true
		case 'j':
			return constNum{tipo: 'J', entero: v.largo}, true
		case 'f':
			return constNum{tipo: 'F', real: float64(float32(v.flotante))}, true
		case 'd':
			return constNum{tipo: 'D', real: v.flotante}, true
		}
		return constNum{}, false
	case joperador:
		switch v.op {
		case "+", "-", "*", "/", "%":
		default:
			return constNum{}, false
		}
		a, ok := plegarConstante(v.izq)
		if !ok {
			return constNum{}, false
		}
		b, ok := plegarConstante(v.der)
		if !ok {
			return constNum{}, false
		}
		t := a.tipo
		if nivelNum(b.tipo) > nivelNum(t) {
			t = b.tipo
		}
		if t == 'I' || t == 'J' {
			if (v.op == "/" || v.op == "%") && b.entero == 0 {
				return constNum{}, false
			}
			if t == 'J' {
				r := int64(0)
				switch v.op {
				case "+":
					r = a.entero + b.entero
				case "-":
					r = a.entero - b.entero
				case "*":
					r = a.entero * b.entero
				case "/":
					r = a.entero / b.entero
				case "%":
					r = a.entero % b.entero
				}
				return constNum{tipo: 'J', entero: r}, true
			}
			var r int32
			ia, ib := int32(a.entero), int32(b.entero)
			switch v.op {
			case "+":
				r = ia + ib
			case "-":
				r = ia - ib
			case "*":
				r = ia * ib
			case "/":
				r = ia / ib
			case "%":
				r = ia % ib
			}
			return constNum{tipo: 'I', entero: int64(r)}, true
		}
		fa, fb := a.comoReal(), b.comoReal()
		if t == 'F' {
			fa = float64(float32(fa))
			fb = float64(float32(fb))
		}
		var r float64
		switch v.op {
		case "+":
			r = fa + fb
		case "-":
			r = fa - fb
		case "*":
			r = fa * fb
		case "/":
			r = fa / fb
		case "%":
			r = math.Mod(fa, fb)
		}
		if t == 'F' {
			r = float64(float32(r))
		}
		return constNum{tipo: t, real: r}, true
	}
	return constNum{}, false
}

// aplanar: los segmentos de una cadena de puntos y su base, que viene vacia si empezo por un nombre.
func aplanar(a jacceso) ([]string, jexpr) {
	var segs []string
	base := jexpr(a)
	for {
		acc, esAcceso := base.(jacceso)
		if !esAcceso {
			break
		}
		segs = append([]string{acc.nombre}, segs...)
		base = acc.base
	}
	return segs, base
}

// acceso resuelve la cadena `R.layout.main`, `savedInstanceState`, `menu` o `this.x`. Se resuelve antes de
// emitir porque `R.layout.main` no emite nada: la constante se dobla.
func (e *emisor) acceso(a jacceso) (string, error) {
	segs, base := aplanar(a)
	linea := a.donde.linea
	if len(segs) == 0 {
		return "", fmt.Errorf("linea %d: acceso sin nombre", linea)
	}
	if base != nil {
		d, err := e.exp(base)
		if err != nil {
			return "", err
		}
		return e.cadenaDeCampos(d, segs, linea)
	}
	if v, ok := e.locales[segs[0]]; ok {
		e.cargaDe(v.indice, v.desc)
		return e.cadenaDeCampos(v.desc, segs[1:], linea)
	}
	if len(segs) == 1 {
		if d, ok := e.campoDeLaClaseActual(segs[0], linea); ok {
			return d, nil
		}
	}
	interno, err := e.ctx.resolverSimple(segs[0])
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", linea, err)
	}
	return e.accesoEstatico(interno, segs[1:], linea)
}

// cadenaDeCampos: campos de instancia sobre el valor que ya esta en la pila.
func (e *emisor) cadenaDeCampos(desc string, segs []string, linea int) (string, error) {
	actual := desc
	for _, seg := range segs {
		if seg == "length" && len(actual) > 0 && actual[0] == '[' {
			// `v.length` es `arraylength` (0xbe), medido en el §66; no es un campo, así que no pasa
			// por el pool ni por el jar
			e.emite(0xbe)
			actual = "I"
			continue
		}
		if len(actual) == 0 || actual[0] != 'L' {
			return "", fmt.Errorf("linea %d: %s no es un objeto, no se le puede sacar %s", linea, actual, seg)
		}
		dueño := actual[1 : len(actual)-1]
		cr, err := e.ctx.buscarCampo(dueño, seg)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", linea, err)
		}
		if cr.constante {
			return "", fmt.Errorf("linea %d: %s es una constante y sacarla de una instancia no esta medida", linea, seg)
		}
		if cr.acceso&accStatic != 0 {
			return "", fmt.Errorf("linea %d: %s.%s es estatico: sacarlo de una instancia no esta medido", linea, dueño, seg)
		}
		idx := e.pool.campo(dueño, seg, cr.desc)
		e.getfieldDe(idx, cr.desc)
		actual = cr.desc
	}
	return actual, nil
}

// accesoEstatico: `R.layout.main` desde el nombre del tipo. Las anidadas intermedias no emiten nada; el
// ultimo segmento es un campo estatico o una constante doblada.
func (e *emisor) accesoEstatico(interno string, segs []string, linea int) (string, error) {
	if len(segs) == 0 {
		return "", fmt.Errorf("linea %d: el tipo %s suelto no es un valor", linea, interno)
	}
	tipo := interno
	for i, seg := range segs {
		ultimo := i == len(segs)-1
		if anidada := tipo + "$" + seg; e.ctx.hay(anidada) {
			if ultimo {
				return "", fmt.Errorf("linea %d: %s es un tipo, no un valor", linea, seg)
			}
			// javac deja la Class de la anidada en el pool aunque el valor se doble despues: medido en el
			// parrafo 40 (#11 = Class com/hola/j1g/R$layout en MainActivity, que no usa ese tipo en codigo)
			e.usar(anidada)
			tipo = anidada
			continue
		}
		cr, err := e.ctx.buscarCampo(tipo, seg)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", linea, err)
		}
		if !ultimo {
			return "", fmt.Errorf("linea %d: tras el campo %s no hay nada medido", linea, seg)
		}
		if cr.constante {
			e.cargaEntero(cr.valor)
			return "I", nil
		}
		if cr.acceso&accStatic == 0 {
			return "", fmt.Errorf("linea %d: %s.%s no es estatico y no se puede nombrar por el tipo", linea, tipo, seg)
		}
		idx := e.pool.campo(tipo, seg, cr.desc)
		e.getstaticDe(idx, cr.desc)
		return cr.desc, nil
	}
	return "", fmt.Errorf("linea %d: no se resuelve %s.%s", linea, interno, strings.Join(segs, "."))
}

// literalDeClase emite `X.class` con la forma que midió el párrafo 89: un tipo de referencia o un
// arreglo es `ldc` de CONSTANT_Class (el nombre es el interno, o el descriptor si lleva `[]`), y un
// primitivo suelto es `getstatic Wrapper.TYPE:Ljava/lang/Class;` porque javac no lo dobla a `ldc`.
func (e *emisor) literalDeClase(v jclase) (string, error) {
	if v.primitivo && v.dimensiones == 0 {
		envoltorio, ok := envoltorioDePrimitivo[v.nombre]
		if !ok {
			return "", fmt.Errorf("linea %d: no hay envoltorio medido para `%s.class`", v.donde.linea, v.nombre)
		}
		return e.accesoEstatico(envoltorio, []string{"TYPE"}, v.donde.linea)
	}
	var nombre string
	if v.primitivo {
		d, ok := descPrimitivo[v.nombre]
		if !ok {
			return "", fmt.Errorf("linea %d: `%s` no es un primitivo medido en un literal de clase", v.donde.linea, v.nombre)
		}
		nombre = d
		for i := 0; i < v.dimensiones; i++ {
			nombre = "[" + nombre
		}
	} else {
		interno, err := e.ctx.resolverSimple(v.nombre)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		nombre = interno
		if v.dimensiones > 0 {
			nombre = "L" + interno + ";"
			for i := 0; i < v.dimensiones; i++ {
				nombre = "[" + nombre
			}
		}
	}
	e.ldc(e.pool.clase(nombre))
	e.sube()
	return "Ljava/lang/Class;", nil
}

// campoDeLaClaseActual: `menu` suelto cuando fuera un campo de esta clase.
// duenioDelCampo: donde vive un nombre suelto de asignacion o de lectura. Primero la clase actual; y
// dentro de una anonima, si no es propio, el exterior: medido en el parrafo 54 sobre el contador, cuyo
// `onClick` escribe `pulsos = pulsos + 1` y `total = total + pulsos` en campos de la Activity que creó
// la anonima.
func (e *emisor) duenioDelCampo(nombre string) (string, campoResuelto, bool) {
	if cr, err := e.ctx.buscarCampo(e.interno, nombre); err == nil {
		return e.interno, cr, true
	}
	if e.c.anonima {
		externo := e.encierreInterno()
		if cr, err := e.ctx.buscarCampo(externo, nombre); err == nil {
			// si el campo del exterior es privado, javac no escribe el getfield contra el campo sino contra
			// sus metodos sinteticos `access$NNN` (parrafo 72: `access$000` para leer, `access$002` para
			// escribir, y el de escritura devuelve el valor, de ahi el `pop`); los dos llamadores lo
			// distinguen por el flag y pasan por el accesor
			return externo, cr, true
		}
	}
	return "", campoResuelto{}, false
}

// cargaDelDuenio: el receptor de un campo de instancia. En la clase actual es `aload_0`; dentro de una
// anonima, si el campo es del exterior, javac pasa por la captura: `aload_0; getfield this$0:LExterior;`.
// Medido dos veces en la misma linea del contador (`pulsos = pulsos + 1`), una por el receptor del
// `putfield` y otra por la lectura del valor: javac no duplica el receptor con `dup`, lo vuelve a cargar.
func (e *emisor) cargaDelDuenio(dueño string) {
	e.cargaDe(0, "L")
	if dueño == e.interno {
		return
	}
	e.getfield(e.pool.campo(e.interno, "this$0", "L"+dueño+";"))
}

func (e *emisor) campoDeLaClaseActual(nombre string, linea int) (string, bool) {
	dueño, cr, encontrado := e.duenioDelCampo(nombre)
	if !encontrado {
		return "", false
	}
	if cr.constante {
		e.cargaEntero(cr.valor)
		return cr.desc, true
	}
	estatico := cr.acceso&accStatic != 0
	if dueño != e.interno && cr.acceso&accPrivate != 0 {
		// el campo privado del exterior: la anonima pasa por `access$NN0` (parrafo 72), y el accesor de
		// un campo ancho es `getfield` + `lreturn`/`dreturn`, sin nada mas (párrafo 83)
		nombreAcc, descAcc := e.ctx.accesorPara(dueño, nombre, cr.desc, true, false, estatico)
		enPila := 0
		if !estatico {
			if e.estatico {
				return "", false
			}
			e.cargaDelDuenio(dueño)
			enPila = 1
		}
		e.invoca(opInvokestat, dueño, nombreAcc, descAcc, enPila)
		return cr.desc, true
	}
	if estatico {
		idx := e.pool.campo(dueño, nombre, cr.desc)
		e.getstaticDe(idx, cr.desc)
		return cr.desc, true
	}
	if e.estatico {
		return "", false
	}
	e.cargaDelDuenio(dueño)
	e.getfieldDe(e.pool.campo(dueño, nombre, cr.desc), cr.desc)
	return cr.desc, true
}

// llamada: los cuatro casos del subconjunto, con el Methodref en el sitio que pone javac (parrafo 40).
func (e *emisor) llamada(l jllamada) (string, error) {
	argc := len(l.args)
	if l.constructorPropio {
		// `this(...)`: javac escribe `aload_0; argumentos; invokespecial <init>` contra la MISMA
		// clase (parrafo 91). La delegacion no baja los inicializadores de campo; eso lo hace el
		// constructor al que se delega.
		e.cargaDe(0, "L")
		tipos, err := e.argumentos(l.args)
		if err != nil {
			return "", err
		}
		cand, err := e.ctx.buscarMetodo(e.interno, "<init>", argc, tipos)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		e.invoca(opInvokespec, e.interno, "<init>", cand.desc, argc+1)
		return "V", nil
	}
	if l.super {
		if e.estatico {
			return "", fmt.Errorf("linea %d: `super` en un metodo estatico no es Java", l.donde.linea)
		}
		dueño, err := e.superDe()
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		e.cargaDe(0, "L")
		tipos, err := e.argumentos(l.args)
		if err != nil {
			return "", err
		}
		cand, err := e.ctx.buscarMetodo(dueño, l.nombre, argc, tipos)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		e.invoca(opInvokespec, dueño, l.nombre, cand.desc, argc+1)
		return vueltaDe(cand.desc), nil
	}
	if l.base == nil {
		if e.estatico {
			// sin receptor dentro de un metodo static: Java solo lo admite si el metodo es estatico, y
			// javac escribe `invokestatic` contra la clase actual, que es la misma forma ya medida (35c).
			// Los tipos se estiman **antes** de bajar los argumentos porque la resolucion tiene que elegir
			// la sobrecarga y aqui no hay receptor que cargar primero (parrafo 62).
			tipos := make([]string, 0, argc)
			for _, arg := range l.args {
				d, err := e.tipoEstimado(arg)
				if err != nil {
					return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
				}
				tipos = append(tipos, d)
			}
			cand, err := e.ctx.buscarMetodo(e.interno, l.nombre, argc, tipos)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
			}
			if cand.acceso&accStatic == 0 {
				return "", fmt.Errorf("linea %d: %s no es estatico y desde un metodo estatico hace falta receptor", l.donde.linea, l.nombre)
			}
			if _, err := e.argumentos(l.args); err != nil {
				return "", err
			}
			e.invoca(opInvokestat, e.interno, l.nombre, cand.desc, argc)
			return vueltaDe(cand.desc), nil
		}
		// Una anonima resuelve el nombre suelto primero en si misma y, si no hay ninguna candidata, en la
		// clase que la creo (parrafo 72): un metodo no privado del exterior se llama con la captura
		// (`aload_0; getfield this$0; invokevirtual`) y uno privado por su `access$NN0`, que es un
		// `invokestatic` con la captura como primer argumento. Un estatico del exterior no lleva receptor.
		if e.c.anonima {
			cands, err := e.ctx.candidatos(e.interno, l.nombre, argc)
			if err == nil && len(cands) == 0 {
				externo := e.encierreInterno()
				tipos := make([]string, 0, argc)
				for _, arg := range l.args {
					d, err := e.tipoEstimado(arg)
					if err != nil {
						return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
					}
					tipos = append(tipos, d)
				}
				cand, err := e.ctx.buscarMetodo(externo, l.nombre, argc, tipos)
				if err != nil {
					return "", fmt.Errorf("linea %d: no existe %s con %d argumentos en %s ni en el exterior %s: %w",
						l.donde.linea, l.nombre, argc, e.interno, externo, err)
				}
				if cand.acceso&accStatic != 0 {
					if _, err := e.argumentos(l.args); err != nil {
						return "", err
					}
					if cand.acceso&accPrivate != 0 {
						// tambien un estatico privado pasa por su accessor, que no lleva receptor
						nombreAcc, descAcc := e.ctx.accesorPara(externo, l.nombre, cand.desc, false, false, true)
						e.invoca(opInvokestat, externo, nombreAcc, descAcc, argc)
					} else {
						e.invoca(opInvokestat, externo, l.nombre, cand.desc, argc)
					}
					return vueltaDe(cand.desc), nil
				}
				e.cargaDe(0, "L")
				e.getfield(e.pool.campo(e.interno, "this$0", "L"+externo+";"))
				if _, err := e.argumentos(l.args); err != nil {
					return "", err
				}
				if cand.acceso&accPrivate != 0 {
					nombreAcc, descAcc := e.ctx.accesorPara(externo, l.nombre, cand.desc, false, false, false)
					e.invoca(opInvokestat, externo, nombreAcc, descAcc, argc+1)
				} else {
					e.invoca(opInvokevirt, externo, l.nombre, cand.desc, argc+1)
				}
				return vueltaDe(cand.desc), nil
			}
		}
		// receptor implicito: el Methodref va contra la clase actual, medido en el parrafo 40. Un
		// metodo `static` de la propia clase se llama sin receptor (`invokestatic`): javac lo escribe
		// asi desde un metodo de instancia, y emitir `invokevirtual` revienta en ART con
		// `IncompatibleClassChangeError` (§136)
		tipos := make([]string, 0, argc)
		for _, arg := range l.args {
			d, err := e.tipoEstimado(arg)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
			}
			tipos = append(tipos, d)
		}
		cand, err := e.ctx.buscarMetodo(e.interno, l.nombre, argc, tipos)
		if err != nil {
			return "", fmt.Errorf("linea %d: no existe %s con %d argumentos en %s ni en sus padres: %w",
				l.donde.linea, l.nombre, argc, e.interno, err)
		}
		if cand.acceso&accStatic != 0 {
			if _, err := e.argumentos(l.args); err != nil {
				return "", err
			}
			e.invoca(opInvokestat, e.interno, l.nombre, cand.desc, argc)
			return vueltaDe(cand.desc), nil
		}
		e.cargaDe(0, "L")
		if _, err := e.argumentos(l.args); err != nil {
			return "", err
		}
		opcode := byte(opInvokevirt)
		if cand.acceso&accPrivate != 0 {
			// un metodo privado de la propia clase se invoca con `invokespecial`, medido en el parrafo 72
			opcode = opInvokespec
		}
		e.invoca(opcode, e.interno, l.nombre, cand.desc, argc+1)
		return vueltaDe(cand.desc), nil
	}
	if acc, esAcceso := l.base.(jacceso); esAcceso && acc.base == nil && e.locales[acc.nombre].desc == "" {
		if interno, err := e.ctx.resolverSimple(acc.nombre); err == nil {
			tipos, err := e.argumentos(l.args)
			if err != nil {
				return "", err
			}
			cand, err := e.ctx.buscarMetodo(interno, l.nombre, argc, tipos)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
			}
			if cand.acceso&accStatic == 0 {
				return "", fmt.Errorf("linea %d: %s.%s no es estatico y no se puede llamar sobre el tipo", l.donde.linea, interno, l.nombre)
			}
			e.invoca(opInvokestat, interno, l.nombre, cand.desc, argc)
			return vueltaDe(cand.desc), nil
		}
	}
	d, err := e.exp(l.base)
	if err != nil {
		return "", err
	}
	if len(d) == 0 || d[0] != 'L' {
		return "", fmt.Errorf("linea %d: no se puede llamar %s sobre un %s", l.donde.linea, l.nombre, d)
	}
	receptor := d[1 : len(d)-1]
	tipos, err := e.argumentos(l.args)
	if err != nil {
		return "", err
	}
	cand, err := e.ctx.buscarMetodo(receptor, l.nombre, argc, tipos)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	accesosReceptor, err := e.ctx.accesosDe(receptor)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	var opcode byte = opInvokevirt
	if accesosReceptor&accInterface != 0 {
		opcode = opInvointf
	}
	if cand.acceso&accPrivate != 0 {
		if receptor != e.interno {
			return "", fmt.Errorf("linea %d: %s.%s es privado y se llama desde %s", l.donde.linea, receptor, l.nombre, e.interno)
		}
		// `this.metodoPrivado()`: javac tambien escribe `invokespecial` (parrafo 72)
		opcode = opInvokespec
	}
	e.invoca(opcode, receptor, l.nombre, cand.desc, argc+1)
	return vueltaDe(cand.desc), nil
}

// argumentos: baja los argumentos al pico y devuelve sus descriptores, que es lo que necesita
// buscarMetodo para elegir entre sobrecargas (parrafo 62).
func (e *emisor) argumentos(args []jexpr) ([]string, error) {
	tipos := make([]string, 0, len(args))
	for _, arg := range args {
		d, err := e.exp(arg)
		if err != nil {
			return nil, err
		}
		tipos = append(tipos, d)
	}
	return tipos, nil
}

func (e *emisor) superDe() (string, error) { return e.ctx.padreDe(e.c) }

// nuevo: `new Tipo(args)`. Una clase anonima baja el exterior a un campo this$0 y ese camino se comprueba
// aparte, con su propio parrafo de medicion.
func (e *emisor) nuevo(n jnuevo) (string, error) {
	if n.cuerpo != nil {
		// la anonima se nombra Exterior$N como la nombra javac y su constructor recibe primero la captura
		// del exterior: medido en el parrafo 42 (`new #28; dup; aload_0; invokespecial <init>
		// :(Lcom/hola/apkcstudio/MainActivity;)V`)
		interno := internoDe(n.cuerpo.paquete, n.cuerpo.nombre)
		e.usar(interno)
		idx := e.pool.clase(interno)
		e.emite(opNew, byte(idx>>8), byte(idx))
		e.sube()
		e.emite(opDup)
		e.sube()
		if !e.estatico {
			e.cargaDe(0, "L")
		}
		for _, cap := range n.cuerpo.capturas {
			lv, ok := e.locales[cap.nombre]
			if !ok {
				return "", fmt.Errorf("linea %d: la captura %s de la anonima no es una local viva", n.donde.linea, cap.nombre)
			}
			e.cargaDe(lv.indice, lv.desc)
		}
		if _, err := e.argumentos(n.args); err != nil {
			return "", err
		}
		sb := strings.Builder{}
		sb.WriteString("(")
		if !e.estatico {
			sb.WriteString("L" + e.interno + ";")
		}
		for _, cap := range n.cuerpo.capturas {
			d, err := e.ctx.descriptorDe(cap.tipo)
			if err != nil {
				return "", err
			}
			sb.WriteString(d)
		}
		for _, arg := range n.args {
			d, err := e.tipoEstimado(arg)
			if err != nil {
				return "", err
			}
			sb.WriteString(d)
		}
		sb.WriteString(")")
		// un descriptor de metodo lleva siempre la vuelta: sin la V del constructor el dexer internaria un
		// tipo que no existe
		sb.WriteString("V")
		enPila := 1 + len(n.cuerpo.capturas) + len(n.args)
		if !e.estatico {
			enPila++
		}
		e.invoca(opInvokespec, interno, "<init>", sb.String(), enPila)
		return "L" + interno + ";", nil
	}
	interno, err := e.ctx.tipoAInterno(n.tipo)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", n.donde.linea, err)
	}
	// el descriptor del constructor es el de la DECLARACION, no el de los tipos que se le pasan: con
	// `new TextView(this)` el argumento es la actividad y el constructor es
	// `<init>(Landroid/content/Context;)V`; firmarlo con el tipo del argumento compila y ART corta con
	// `NoSuchMethodError` (§136)
	tipos := make([]string, 0, len(n.args))
	for _, arg := range n.args {
		d, err := e.tipoEstimado(arg)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", n.donde.linea, err)
		}
		tipos = append(tipos, d)
	}
	cand, err := e.ctx.buscarMetodo(interno, "<init>", len(n.args), tipos)
	if err != nil {
		return "", fmt.Errorf("linea %d: %s: %w", n.donde.linea, interno, err)
	}
	e.usar(interno)
	idx := e.pool.clase(interno)
	e.emite(opNew, byte(idx>>8), byte(idx))
	e.sube()
	e.emite(opDup)
	e.sube()
	if _, err := e.argumentos(n.args); err != nil {
		return "", err
	}
	// el `dup` deja dos referencias: el constructor se come una con sus argumentos y la otra es el valor de
	// la expresion, que es el hueco que sube de la pila al terminar
	e.invoca(opInvokespec, interno, "<init>", cand.desc, len(n.args)+1)
	return "L" + interno + ";", nil
}

// atypeDe: el atype del JVMS 4.9 para el `newarray` de un elemento primitivo. Los ocho son la tabla
// entera; el subconjunto del fuente solo escribe `int`.
var atypeDe = map[byte]byte{'Z': 4, 'C': 5, 'F': 6, 'D': 7, 'B': 8, 'S': 9, 'I': 10, 'J': 11}

// nuevoArreglo: `new T[n]`. El largo se evalua primero y sale `newarray`/`anewarray`, como javac en
// la sonda `Arreglos` del parrafo 66 (`iconst_3; newarray int` y `iload_0; anewarray String`). El
// `anewarray` lleva la clase del ELEMENTO en el pool, no la del arreglo (`anewarray #7 // class
// java/lang/String`).
func (e *emisor) nuevoArreglo(v jnuevoArreglo) (string, error) {
	elem, err := e.ctx.descriptorDe(v.tipo)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	if v.largo == nil {
		// `new T[] {…}`: el largo es el numero de elementos y javac lo escribe con `cargaEntero`
		e.cargaEntero(int32(len(v.elementos)))
	} else {
		d, err := e.exp(v.largo)
		if err != nil {
			return "", err
		}
		if d != "I" {
			return "", fmt.Errorf("linea %d: el largo de un arreglo es %s y no un entero", v.donde.linea, d)
		}
	}
	if elem[0] == 'L' || elem[0] == '[' {
		interno := elem
		if elem[0] == 'L' {
			interno = elem[1 : len(elem)-1]
			e.usar(interno)
		}
		idx := e.pool.clase(interno)
		e.emite(0xbd, byte(idx>>8), byte(idx))
	} else {
		atype, ok := atypeDe[elem[0]]
		if !ok {
			return "", fmt.Errorf("linea %d: un arreglo de %s no esta medido", v.donde.linea, v.tipo)
		}
		e.emite(0xbc, atype)
	}
	e.baja(1) // el largo
	e.sube()  // el arreglo
	if v.largo != nil {
		return "[" + elem, nil
	}
	// la cadena de javac: por cada elemento, `dup`, indice, valor y `iastore`/`aastore` (parrafo 73)
	var guarda byte
	switch elem[0] {
	case 'I':
		guarda = 0x4f
	case 'J':
		guarda = 0x50
	case 'F':
		guarda = 0x51
	case 'D':
		guarda = 0x52
	case 'C':
		// `castore` 0x55 en la cadena del inicializador: `new char[] {'a','b'}` es `iconst_2; newarray
		// char; dup; iconst_0; bipush 97; castore; dup; iconst_1; bipush 98; castore` (parrafo 87)
		guarda = 0x55
	case 'L', '[':
		guarda = 0x53
	default:
		return "", fmt.Errorf("linea %d: un arreglo de %s no esta medido", v.donde.linea, v.tipo)
	}
	for i, x := range v.elementos {
		e.emite(opDup)
		e.sube()
		e.cargaEntero(int32(i))
		d, err := e.expA(x, elem)
		if err != nil {
			return "", err
		}
		if d != elem {
			return "", fmt.Errorf("linea %d: el arreglo es %s y el elemento %d es %s", v.donde.linea, elem, i, d)
		}
		e.emite(guarda)
		// el `*astore` se come el arreglo, el indice y el valor: el valor de un `long` o un `double`
		// son DOS palabras, así que la cuenta del `int[]` del §73 no sirve aquí
		e.baja(2)
		e.bajaPor(elem)
	}
	return "[" + elem, nil
}

// leeIndice: `v[i]`. El elemento sale por `iaload` (0x2e) o `aaload` (0x32), medidos en el §66.
func (e *emisor) leeIndice(v jindice) (string, error) {
	base, err := e.exp(v.base)
	if err != nil {
		return "", err
	}
	if len(base) == 0 || base[0] != '[' {
		return "", fmt.Errorf("linea %d: se indexa un %s, que no es un arreglo", v.donde.linea, base)
	}
	d, err := e.exp(v.indice)
	if err != nil {
		return "", err
	}
	if d != "I" {
		return "", fmt.Errorf("linea %d: el indice es %s y no un entero", v.donde.linea, d)
	}
	elem := base[1:]
	ancho := elem[0] == 'J' || elem[0] == 'D'
	switch elem[0] {
	case 'I':
		e.emite(0x2e)
	case 'J':
		e.emite(0x2f)
	case 'F':
		e.emite(0x30)
	case 'D':
		e.emite(0x31)
	case 'C':
		// `caload` 0x34, medido en `Letras.lee` (`aload_0; iload_1; caload; ireturn`, parrafo 87)
		e.emite(0x34)
	case 'L', '[':
		e.emite(0x32)
	default:
		return "", fmt.Errorf("linea %d: un arreglo de %s no esta medido", v.donde.linea, elem)
	}
	e.baja(2)
	if ancho {
		e.subeDos()
	} else {
		e.sube()
	}
	return elem, nil
}

// escribeIndice: `v[i] = x`, la sentencia. La pila JVM lleva receptor, indice y valor en ese orden,
// y sale `iastore`/`aastore` (medidos: `aput v2, v0, v1` en el §66).
func (e *emisor) escribeIndice(v jindice, valor jexpr) error {
	base, err := e.exp(v.base)
	if err != nil {
		return err
	}
	if len(base) == 0 || base[0] != '[' {
		return fmt.Errorf("linea %d: se indexa un %s, que no es un arreglo", v.donde.linea, base)
	}
	d, err := e.exp(v.indice)
	if err != nil {
		return err
	}
	if d != "I" {
		return fmt.Errorf("linea %d: el indice es %s y no un entero", v.donde.linea, d)
	}
	elem := base[1:]
	// `expA` y no `exp`: un literal entero cabe en un elemento `char` sin conversion —medido `v[0] = 65`
	// como `bipush 65; castore` (parrafo 87)— y una constante se pliega ya en el tipo del elemento.
	dv, err := e.expA(valor, elem)
	if err != nil {
		return err
	}
	if dv != elem {
		return fmt.Errorf("linea %d: el arreglo es %s y se le asigna un %s", v.donde.linea, elem, dv)
	}
	ancho := elem[0] == 'J' || elem[0] == 'D'
	switch elem[0] {
	case 'I':
		e.emite(0x4f)
	case 'J':
		e.emite(0x50)
	case 'F':
		e.emite(0x51)
	case 'D':
		e.emite(0x52)
	case 'C':
		// `castore` 0x55, medido en `Letras.escribe` (`aload_0; iload_1; iload_2; castore`, parrafo 87)
		e.emite(0x55)
	case 'L', '[':
		e.emite(0x53)
	default:
		return fmt.Errorf("linea %d: un arreglo de %s no esta medido", v.donde.linea, elem)
	}
	// el valor de dos palabras ocupa una mas en la pila simulada
	if ancho {
		e.baja(4)
	} else {
		e.baja(3)
	}
	return nil
}

// tipoEstimado: el descriptor de un argumento que va en un constructor, sin volver a emitirlo. En el
// subconjunto medido los argumentos de un `new` son literales o variables, y ambos se saben aqui.
func (e *emisor) tipoEstimado(x jexpr) (string, error) {
	switch v := x.(type) {
	case jlit:
		switch v.tipoJ {
		case 'i':
			return "I", nil
		case 'c':
			return "C", nil
		case 'j':
			return "J", nil
		case 'f':
			return "F", nil
		case 'd':
			return "D", nil
		case 'b':
			return "Z", nil
		case 's':
			return "Ljava/lang/String;", nil
		case 'n':
			return "Ljava/lang/Object;", nil
		}
		return "", fmt.Errorf("linea %d: un literal de clase %q no esta medido", v.donde.linea, string(v.tipoJ))
	case jclase:
		return "Ljava/lang/Class;", nil
	case jacceso:
		segs, base := aplanar(v)
		if base == nil && len(segs) == 1 {
			if lv, ok := e.locales[segs[0]]; ok {
				return lv.desc, nil
			}
			cr, err := e.ctx.buscarCampo(e.interno, segs[0])
			if err == nil {
				return cr.desc, nil
			}
		}
		if base == nil {
			if interno, err := e.ctx.resolverSimple(segs[0]); err == nil {
				return e.descDeAccesoEstatico(interno, segs[1:])
			}
		}
		return "", fmt.Errorf("linea %d: el tipo de ese argumento no esta medido", v.donde.linea)
	case jthis:
		return "L" + e.interno + ";", nil
	case jindice:
		if acc, ok := v.base.(jacceso); ok && acc.base == nil {
			if lv, ok := e.locales[acc.nombre]; ok && len(lv.desc) > 0 && lv.desc[0] == '[' {
				return lv.desc[1:], nil
			}
		}
		return "", fmt.Errorf("linea %d: el tipo de ese argumento no esta medido", v.donde.linea)
	case jnuevoArreglo:
		d, err := e.ctx.descriptorDe(v.tipo)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		return "[" + d, nil
	case jllamada:
		// una llamada sin receptor escrito: la vuelta de su unica candidata en esa aridad (medido al
		// pasar `local()` y `cuenta(2L, 3L)` como argumento de `imprime` en la sonda `L`, parrafo 74)
		if v.base != nil || v.super {
			return "", fmt.Errorf("linea %d: el tipo de ese argumento no esta medido", v.donde.linea)
		}
		cands, err := e.ctx.candidatos(e.interno, v.nombre, len(v.args))
		if err != nil || len(cands) != 1 {
			return "", fmt.Errorf("linea %d: el tipo de ese argumento no esta medido", v.donde.linea)
		}
		return vueltaDe(cands[0].desc), nil
	}
	return "", fmt.Errorf("linea %d: el tipo de un argumento de %T no esta medido", lineaDeExpr(x), x)
}

// tipoPrevisto: el descriptor que va a salir de una expresion, **sin escribir su codigo**. Hace falta
// para saber si una cuenta es larga antes de emitir el lado izquierdo, que es donde javac mete el
// `i2l` (parrafo 74). Lo que no se puede prever sin emitir sale con ok falso y el llamador lo dice.
func (e *emisor) tipoPrevisto(x jexpr) (string, bool) {
	switch v := x.(type) {
	case jlit:
		switch v.tipoJ {
		case 'i', 'c':
			// el `char` se prevé con su descriptor (`C`), que es lo que necesita la asignación y el
			// arreglo; para la cuenta se estrecha a `I` antes de promover (parrafo 87)
			if v.tipoJ == 'c' {
				return "C", true
			}
			return "I", true
		case 'j':
			return "J", true
		case 'f':
			return "F", true
		case 'd':
			return "D", true
		case 'b':
			return "Z", true
		case 's':
			return "Ljava/lang/String;", true
		}
		return "", false
	case jclase:
		return "Ljava/lang/Class;", true
	case jthis:
		if v.nombre != "" || e.estatico {
			return "", false
		}
		return "L" + e.interno + ";", true
	case jindice:
		if b, ok := v.base.(jacceso); ok && b.base == nil {
			if lv, ok := e.locales[b.nombre]; ok && len(lv.desc) > 0 && lv.desc[0] == '[' {
				return lv.desc[1:], true
			}
		}
		return "", false
	case jnuevoArreglo:
		d, err := e.ctx.descriptorDe(v.tipo)
		if err != nil {
			return "", false
		}
		return "[" + d, true
	case jacceso:
		segs, base := aplanar(v)
		if base != nil || len(segs) == 0 {
			return "", false
		}
		if lv, ok := e.locales[segs[0]]; ok {
			if len(segs) == 1 {
				return lv.desc, true
			}
			return "", false
		}
		if len(segs) == 1 {
			dueño, cr, ok := e.duenioDelCampo(segs[0])
			if !ok || dueño == "" {
				return "", false
			}
			return cr.desc, true
		}
		if interno, err := e.ctx.resolverSimple(segs[0]); err == nil {
			d, err := e.descDeAccesoEstatico(interno, segs[1:])
			if err != nil {
				return "", false
			}
			return d, true
		}
		return "", false
	case joperador:
		// una cuenta es ancha si alguno de sus lados lo es, con la promocion de Java (doble > flotante
		// > largo > entero); lo que mezcla tipos que no se pueden prever sale con ok falso (parrafos
		// 74 y 77)
		di, okI := e.tipoPrevisto(v.izq)
		dd, okD := e.tipoPrevisto(v.der)
		di, dd = enCuenta(di), enCuenta(dd)
		nivel := map[string]int{"": 0, "I": 0, "J": 1, "F": 2, "D": 3}
		ancho := ""
		if okI && nivel[di] > nivel[ancho] {
			ancho = di
		}
		if okD && nivel[dd] > nivel[ancho] {
			ancho = dd
		}
		if ancho != "" {
			return ancho, true
		}
		if okI && okD && di == "I" && dd == "I" {
			return "I", true
		}
		return "", false
	case jllamada:
		if v.base != nil || v.super {
			return "", false
		}
		cands, err := e.ctx.candidatos(e.interno, v.nombre, len(v.args))
		if err != nil || len(cands) != 1 {
			return "", false
		}
		return vueltaDe(cands[0].desc), true
	}
	return "", false
}

// expA: emite la expresion y, si el hueco que hay que llenar es ancho y lo que sale es un entero, mete
// el `i2l` (0x85) que javac pone en su lugar.
func (e *emisor) expA(x jexpr, deseado string) (string, error) {
	// La promocion de un literal se hace **sobre la constante**, antes de elegir su forma: medido en el
	// parrafo 85, `d + 1` es `dconst_1`, `l + 2` es `ldc2_w 2l` y `f * -2` es `ldc -2.0f`, ninguno con la
	// conversion del parrafo 77 detras. Si el operando no es una constante plegable, la conversion si va
	// en su sitio (`d + i` con `i` variable es `iload_0; i2d`).
	if len(deseado) == 1 && strings.IndexByte("IJFD", deseado[0]) >= 0 {
		if c, ok := plegarConstante(x); ok && nivelNum(c.tipo) <= nivelNum(deseado[0]) {
			c = c.promocionA(deseado[0])
			e.cargaConstante(c)
			return string(c.tipo), nil
		}
	}
	// El `char` se carga con el mismo `iconst`/`bipush`/`sipush`/`ldc` que el `int`, y la constante entera
	// entra en un hueco `char` sin conversion alguna: medido en `Letras2`, `char c = 65` es `bipush 65;
	// istore_0` y `v[0] = 65` es `bipush 65; castore` (parrafo 87). Fuera del rango javac da error, y aqui
	// se rechaza con el mismo nombre.
	if deseado == "C" {
		if c, ok := plegarConstante(x); ok && (c.tipo == 'I' || c.tipo == 'J' || c.tipo == 'C') {
			if c.entero < 0 || c.entero > 0xffff {
				return "", fmt.Errorf("linea %d: la constante %d no cabe en un `char` (llega a 65535)", lineaDeExpr(x), c.entero)
			}
			e.cargaEntero(int32(c.entero))
			return "C", nil
		}
	}
	d, err := e.exp(x)
	if err != nil {
		return "", err
	}
	if d == deseado {
		return d, nil
	}
	// Enschar un `char` a `int` no escribe nada: `int n = c;` es `iload_0; istore_1` y `aEntero(char)` es
	// `iload_0; ireturn`, medidos en el parrafo 87.
	if d == "C" && deseado == "I" {
		return "I", nil
	}
	if len(d) == 1 && len(deseado) == 1 && e.convierte(d, deseado) {
		return deseado, nil
	}
	return d, nil
}

// convierte: la conversion numerica entre los tipos medidos, con las palabras que cambia. La rejilla
// sale de `i2l` 0x85 (parrafo 74) y de las leidas en `FyD` (parrafo 77): `f2d` en `por`, `i2d` en
// `divide` y `deEntero`, `d2i` en `aEntero`, `d2f` en `aFlotante`. Devuelve false si el par no esta.
func (e *emisor) convierte(de, a string) bool {
	codes := map[[2]byte]byte{
		{'I', 'J'}: 0x85, {'I', 'F'}: 0x86, {'I', 'D'}: 0x87,
		{'J', 'I'}: 0x88, {'J', 'F'}: 0x89, {'J', 'D'}: 0x8a,
		{'F', 'I'}: 0x8b, {'F', 'J'}: 0x8c, {'F', 'D'}: 0x8d,
		{'D', 'I'}: 0x8e, {'D', 'J'}: 0x8f, {'D', 'F'}: 0x90,
	}
	peso := map[byte]int{'I': 1, 'F': 1, 'J': 2, 'D': 2}
	c, ok := codes[[2]byte{de[0], a[0]}]
	if !ok {
		return false
	}
	e.emite(c)
	if delta := peso[a[0]] - peso[de[0]]; delta > 0 {
		for i := 0; i < delta; i++ {
			e.sube()
		}
	} else {
		e.baja(-delta)
	}
	return true
}

// ----- sentencias -----

// esLiteralEntero / esLiteralNulo miran un literal concreto del fuente, sin emitirlo.
func esLiteralEntero(x jexpr, v int32) bool {
	lit, ok := x.(jlit)
	return ok && lit.tipoJ == 'i' && lit.entero == v
}

func esLiteralNulo(x jexpr) bool {
	lit, ok := x.(jlit)
	return ok && lit.tipoJ == 'n'
}

func esEnteroDesc(desc string) bool {
	if desc == "" {
		return false
	}
	return strings.IndexByte("IBCSZ", desc[0]) >= 0
}

// condicionBranca escribe los operandos de la condicion de un `if` y devuelve el opcode de salto con
// la INVERSION que aplica javac, leida en el parrafo 52 comparando las seis relaciones contra cero,
// contra una constante, contra otro entero y contra null:
//
//	`a == b` if_icmpne · `a != b` if_icmpeq · `a < b` if_icmpge · `a >= b` if_icmplt
//	`a > b` if_icmple · `a <= b` if_icmpgt · `a == 0` ifne · `a != 0` ifeq · `a < 0` ifge
//	`a >= 0` iflt · `a > 0` ifle · `a <= 0` ifgt · `o == null` ifnonnull · `o != null` ifnull
//
// Lo que no está en esa tabla se rechaza con nombre: el booleano suelto (`if b`), el literal a la
// izquierda y las referencias comparadas entre sí.
func (e *emisor) condicionBranca(cond jexpr) (byte, int, error) {
	// `x instanceof T` deja un 0/1 y javac branquea con el `ifeq` **sin invertir** (parrafo 79,
	// `Ins.esCadena`: `aload_0; instanceof #7; 4: ifeq 9`). La forma negada
	// (`if (!(x instanceof T))`) es el mismo `instanceof` y después `ifne`, medido en la sonda `E`
	// del §131: javac no invierte con `iconst`/`ixor`.
	if v, esInstance := cond.(jinstanceof); esInstance {
		if _, err := e.exp(v); err != nil {
			return 0, 0, err
		}
		return opIFEQ, 1, nil
	}
	if neg, esNegacion := cond.(jnegacion); esNegacion {
		if v, esInstance := neg.dentro.(jinstanceof); esInstance {
			if _, err := e.exp(v); err != nil {
				return 0, 0, err
			}
			return opIFNE, 1, nil
		}
	}
	op, esComp := cond.(joperador)
	if !esComp {
		return 0, 0, fmt.Errorf("linea %d: una condicion que no sea una comparacion no esta medida (sobre un booleano javac usa `ifeq`, y eso no lo leimos)",
			lineaDeExpr(cond))
	}
	linea := op.donde.linea
	invertidas := map[string]struct{ dos, uno byte }{
		"==": {opIFICMPNE, opIFNE},
		"!=": {opIFICMPEQ, opIFEQ},
		"<":  {opIFICMPGE, opIFGE},
		">=": {opIFICMPLT, opIFLT},
		">":  {opIFICMPLE, opIFLE},
		"<=": {opIFICMPGT, opIFGT},
	}
	pares, ok := invertidas[op.op]
	if !ok {
		return 0, 0, fmt.Errorf("linea %d: el operador %q en una condicion no esta medido", linea, op.op)
	}
	// Comparaciones anchas (párrafo 81): los dos lados se promocionan (doble > flotante > largo >
	// entero), javac emite el `cmp` y después la branquia contra cero que ya estaba medida. El
	// literal cero NO se ahorra aquí: `if (x > 0L)` es `lload_0; lconst_0; lcmp; ifle`.
	tIzq, okIzq := e.tipoPrevisto(op.izq)
	tDer, okDer := e.tipoPrevisto(op.der)
	// `if (c > '0')` es `iload_0; bipush 48; if_icmple`, sin conversion del `char` (parrafo 87)
	tIzq, tDer = enCuenta(tIzq), enCuenta(tDer)
	nivel := map[string]int{"": 0, "I": 0, "J": 1, "F": 2, "D": 3}
	ancho := ""
	if okIzq && nivel[tIzq] > nivel[ancho] {
		ancho = tIzq
	}
	if okDer && nivel[tDer] > nivel[ancho] {
		ancho = tDer
	}
	if ancho == "J" || ancho == "F" || ancho == "D" {
		if !okIzq || !okDer {
			return 0, 0, fmt.Errorf("linea %d: `%s` entre un %s y un operando cuyo tipo no se puede saber antes de escribirlo: la conversion va en el sitio de javac y adivinarlo aqui no esta medido",
				linea, op.op, ancho)
		}
		di, err := e.expA(op.izq, ancho)
		if err != nil {
			return 0, 0, err
		}
		dd, err := e.expA(op.der, ancho)
		if err != nil {
			return 0, 0, err
		}
		if di != ancho || dd != ancho {
			return 0, 0, fmt.Errorf("linea %d: `%s` entre %s y %s: la comparacion ancha solo se midio entre numeros", linea, op.op, di, dd)
		}
		invertida := op.op[0] == '<'
		cmp := byte(opLcmp)
		switch ancho {
		case "F":
			cmp = opFcmpl
			if invertida {
				cmp = opFcmpg
			}
		case "D":
			cmp = opDcmpl
			if invertida {
				cmp = opDcmpg
			}
		}
		e.emite(cmp)
		// cuatro palabras a una con `J`/`D`, dos a una con `F`
		if ancho == "F" {
			e.baja(1)
		} else {
			e.baja(3)
		}
		return pares.uno, 1, nil
	}
	if esLiteralNulo(op.der) {
		d, err := e.exp(op.izq)
		if err != nil {
			return 0, 0, err
		}
		if len(d) == 0 || d[0] != 'L' {
			return 0, 0, fmt.Errorf("linea %d: comparar con null algo que no es una referencia (%s) no esta medido", linea, d)
		}
		if op.op == "==" {
			return opIFNONNULL, 1, nil
		}
		return opIFNULL, 1, nil
	}
	if esLiteralNulo(op.izq) {
		return 0, 0, fmt.Errorf("linea %d: `null %s ...` con el null a la izquierda no esta medido", linea, op.op)
	}
	dizq, err := e.exp(op.izq)
	if err != nil {
		return 0, 0, err
	}
	if esLiteralEntero(op.der, 0) && esEnteroDesc(dizq) {
		return pares.uno, 1, nil
	}
	// el literal a la izquierda (`0 == a`) no se invierte ni se dobla: javac escribe el literal primero
	// y usa la misma branquia de dos operandos, leido en el parrafo 52 (`iconst_0; iload_1; if_icmpne`)
	dder, err := e.exp(op.der)
	if err != nil {
		return 0, 0, err
	}
	if esObjeto(dizq) || esObjeto(dder) {
		// `o.getClass() == String.class` del parrafo 89: las dos referencias se comparan con
		// `if_acmpeq`/`if_acmpne` (0xa5/0xa6), sin conversion. La igualdad es la unica relacion medida
		// entre referencias; `<`, `>` y las demas se rechazan por nombre.
		if !esObjeto(dizq) || !esObjeto(dder) {
			return 0, 0, fmt.Errorf("linea %d: `%s` entre %s y %s: no se midio comparar una referencia con algo que no lo es", linea, op.op, dizq, dder)
		}
		switch op.op {
		case "==":
			return opIFACMPNE, 2, nil
		case "!=":
			return opIFACMPEQ, 2, nil
		}
		return 0, 0, fmt.Errorf("linea %d: `%s` entre dos referencias no esta medido (solo `==` y `!=`)", linea, op.op)
	}
	if !esEnteroDesc(dizq) || !esEnteroDesc(dder) {
		return 0, 0, fmt.Errorf("linea %d: `%s` entre %s y %s: solo se midieron enteros, la referencia contra null y `==`/`!=` entre referencias", linea, op.op, dizq, dder)
	}
	return pares.dos, 2, nil
}

// comparacionDeReferencias: `a == b` / `a != b` como valor booleano. Los dos operandos se emiten, la
// branquia invertida salta al `iconst_0`, y el camino verdadero escribe `iconst_1` y un `goto` al final.
// Es la forma de `Clase.esCadena` (parrafo 89), con `if_acmpne` 0xa6 para `==` y `if_acmpeq` 0xa5
// para `!=`. No se midio ninguna otra relacion entre referencias.
func (e *emisor) comparacionDeReferencias(v joperador) (string, error) {
	di, err := e.exp(v.izq)
	if err != nil {
		return "", err
	}
	dd, err := e.exp(v.der)
	if err != nil {
		return "", err
	}
	if !esObjeto(di) || !esObjeto(dd) {
		return "", fmt.Errorf("linea %d: `%s` como valor solo se midio entre referencias (%s y %s)", v.donde.linea, v.op, di, dd)
	}
	op := byte(opIFACMPNE)
	if v.op == "!=" {
		op = opIFACMPEQ
	}
	falso := e.nuevaEtiqueta()
	fin := e.nuevaEtiqueta()
	e.saltoEmite(op, falso, 2)
	base := e.prof
	e.cargaEntero(1)
	e.saltoEmite(opGOTO, fin, 0)
	e.cierra(falso)
	// la simulacion es lineal: en el camino falso la pila vuelve a estar como antes de la branquia
	e.prof = base
	e.cargaEntero(0)
	e.cierra(fin)
	return "Z", nil
}

// branca: el `if` y su `else`, en el orden que escribe javac (parrafo 52): los operandos, el salto con
// la condicion invertida apuntando al `else` (o al despues si no lo hay), el cuerpo, y el `goto` al
// final solo cuando el cuerpo no sale ya por su cuenta — medido con `if (a == b) return 1;`, que no lo lleva.
func (e *emisor) branca(v jif) error {
	e.marcaLinea(v.donde.linea)
	op, consume, err := e.condicionBranca(v.cond)
	if err != nil {
		return err
	}
	if v.entonces == nil {
		return fmt.Errorf("linea %d: un `if` sin cuerpo no esta medido", v.donde.linea)
	}
	otroLado := e.nuevaEtiqueta()
	e.saltoEmite(op, otroLado, consume)
	if err := e.stm(v.entonces); err != nil {
		return err
	}
	if v.siNo == nil {
		e.cierra(otroLado)
		return nil
	}
	if !terminaEnSalida(v.entonces) {
		final := e.nuevaEtiqueta()
		e.saltoEmite(opGOTO, final, 0)
		e.cierra(otroLado)
		if err := e.stm(v.siNo); err != nil {
			return err
		}
		e.cierra(final)
		return nil
	}
	e.cierra(otroLado)
	return e.stm(v.siNo)
}

// mientras: `while (cond) cuerpo`, con la forma que se midió en el parrafo 63 (javac y `d8` escriben
// lo mismo): la cabeza con la condición **invertida** saltando al final, el cuerpo, y un `goto` a la
// cabeza. En la cabeza la pila JVM está vacía, que es lo que el verificador exige a los dos caminos, y
// los locales van clavados a su registro, así que el reparto no cambia entre vueltas.
func (e *emisor) mientras(v jmientras) error {
	e.marcaLinea(v.donde.linea)
	cabeza := e.nuevaEtiqueta()
	fin := e.nuevaEtiqueta()
	e.cierra(cabeza)
	op, consume, err := e.condicionBranca(v.cond)
	if err != nil {
		return err
	}
	e.saltoEmite(op, fin, consume)
	e.quiebres = append(e.quiebres, fin)
	e.continuar = append(e.continuar, cabeza)
	err = e.stm(v.cuerpo)
	e.quiebres = e.quiebres[:len(e.quiebres)-1]
	e.continuar = e.continuar[:len(e.continuar)-1]
	if err != nil {
		return err
	}
	e.saltoEmite(opGOTO, cabeza, 0)
	if err := e.resuelveAtras(cabeza); err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	e.cierra(fin)
	return nil
}

// para: `for (init; cond; paso) cuerpo`. Es el `while` con el paso delante del `goto` de vuelta, y
// `continue` apunta al paso (medido en el parrafo 70: `goto 21` con el `iinc` del paso en 21).
func (e *emisor) para(v jpara) error {
	e.marcaLinea(v.donde.linea)
	cabeza := e.nuevaEtiqueta()
	fin := e.nuevaEtiqueta()
	paso := e.nuevaEtiqueta()
	e.cierra(cabeza)
	op, consume, err := e.condicionBranca(v.cond)
	if err != nil {
		return err
	}
	e.saltoEmite(op, fin, consume)
	e.quiebres = append(e.quiebres, fin)
	e.continuar = append(e.continuar, paso)
	err = e.stm(v.cuerpo)
	e.quiebres = e.quiebres[:len(e.quiebres)-1]
	e.continuar = e.continuar[:len(e.continuar)-1]
	if err != nil {
		return err
	}
	e.cierra(paso)
	if v.paso != nil {
		if err := e.stm(v.paso); err != nil {
			return err
		}
	}
	e.saltoEmite(opGOTO, cabeza, 0)
	if err := e.resuelveAtras(cabeza); err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	e.cierra(fin)
	return nil
}

// hacer: `do { cuerpo } while (cond)`. La cabecera es la primera instrucción del cuerpo y la vuelta
// lleva la condición **directa**: `if (cond)` salta a la cabecera, que es lo que escribe javac
// (`if_icmplt 2`, medido en el parrafo 69). Como `condicionBranca` devuelve la forma invertida (la
// que salta cuando la condición NO se cumple), se le pasa la comparación negada.
func (e *emisor) hacer(v jhacer) error {
	cabeza := e.nuevaEtiqueta()
	cond := e.nuevaEtiqueta()
	fin := e.nuevaEtiqueta()
	e.cierra(cabeza)
	e.quiebres = append(e.quiebres, fin)
	e.continuar = append(e.continuar, cond)
	// javac no anota la linea del `do`: el primer renglon del cuerpo es el que cae en la cabecera
	err := e.stm(v.cuerpo)
	e.quiebres = e.quiebres[:len(e.quiebres)-1]
	e.continuar = e.continuar[:len(e.continuar)-1]
	if err != nil {
		return err
	}
	// y la condicion lleva la linea del `while` (medido: `line 8: 13` en `Doo.hasta`)
	if linea := lineaDeExpr(v.cond); linea > 0 {
		e.marcaLinea(linea)
	}
	e.cierra(cond)
	negada, err := niegaComparacion(v.cond, v.donde.linea)
	if err != nil {
		return err
	}
	op, consume, err := e.condicionBranca(negada)
	if err != nil {
		return err
	}
	e.saltoEmite(op, cabeza, consume)
	if err := e.resuelveAtras(cabeza); err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	e.cierra(fin)
	return nil
}

// niegaComparacion: el contrario de una comparación, para que `condicionBranca` (que escribe la
// forma que salta al NO cumplirse) salte al cumplirse. Solo comparaciones, como el `while`.
func niegaComparacion(x jexpr, linea int) (jexpr, error) {
	op, esOp := x.(joperador)
	if !esOp {
		return nil, fmt.Errorf("linea %d: un `do-while` solo se mide con una comparacion", linea)
	}
	contrarios := map[string]string{"<": ">=", ">": "<=", "==": "!=", "!=": "==", "<=": ">", ">=": "<"}
	contrario, ok := contrarios[op.op]
	if !ok {
		return nil, fmt.Errorf("linea %d: el operador %q en un `do-while` no esta medido", linea, op.op)
	}
	return joperador{izq: op.izq, op: contrario, der: op.der, donde: op.donde}, nil
}

// cambio: `switch` del fuente a `tableswitch` o `lookupswitch`, con la forma medida en el §64: el
// relleno va del opcode al siguiente múltiplo de cuatro **contado desde el principio del `Code`**, y
// cada destino es un entero de 32 bits con signo desde el principio de la instrucción. La elección de
// forma es de este emisor —`tableswitch` cuando el rango no pasa del doble de los casos—: el criterio
// exacto de javac 21 se sondeó y no es una desigualdad sencilla (con dos casos y rango 4 elige
// `lookupswitch`, con cuatro casos y rango 10 elige `tableswitch`), y las dos formas son correctas.
func (e *emisor) cambio(v jswitch) error {
	e.marcaLinea(v.donde.linea)
	if _, err := e.exp(v.selector); err != nil {
		return err
	}
	casos := append([]jcaso(nil), v.casos...)
	sort.Slice(casos, func(i, j int) bool { return casos[i].valor < casos[j].valor })
	bajo, alto := casos[0].valor, casos[len(casos)-1].valor
	rango := int(alto-bajo) + 1
	etiqueta := map[int32]int{}
	for _, c := range casos {
		etiqueta[c.valor] = e.nuevaEtiqueta()
	}
	fin := e.nuevaEtiqueta()
	defecto := fin
	if v.defecto != nil {
		defecto = e.nuevaEtiqueta()
	}
	inicio := len(e.codigo)
	tabla := rango <= 2*len(casos)
	if tabla {
		e.emite(0xaa)
	} else {
		e.emite(0xab)
	}
	for len(e.codigo)%4 != 0 {
		e.emite(0)
	}
	e.cuatro(defecto, inicio)
	if tabla {
		e.cuatroConstante(bajo)
		e.cuatroConstante(alto)
		for k := bajo; k <= alto; k++ {
			if et, ok := etiqueta[k]; ok {
				e.cuatro(et, inicio)
				continue
			}
			e.cuatro(defecto, inicio)
		}
	} else {
		e.cuatroConstante(int32(len(casos)))
		for _, c := range casos {
			e.cuatroConstante(c.valor)
			e.cuatro(etiqueta[c.valor], inicio)
		}
	}
	e.baja(1) // el switch consume el valor del selector
	// un `break` dentro de un brazo (aunque este anidado en un `if`) sale del switch: se apila su fin
	e.quiebres = append(e.quiebres, fin)
	// los cuerpos se emiten en el orden del fuente, no en el de la tabla: es el orden de javac y es
	// lo que hace que la caída de un brazo al siguiente caiga en el que sigue (§129)
	orden := v.orden
	if len(orden) == 0 {
		for i := range casos {
			orden = append(orden, i)
		}
		if v.defecto != nil {
			orden = append(orden, -1)
		}
	}
	for i, idx := range orden {
		if idx < 0 {
			e.cierra(defecto)
			if err := e.stm(jbloque{sentencias: v.defecto, donde: v.donde}); err != nil {
				e.quiebres = e.quiebres[:len(e.quiebres)-1]
				return err
			}
			continue
		}
		c := v.casos[idx]
		e.cierra(etiqueta[c.valor])
		if err := e.stm(jbloque{sentencias: c.cuerpo, donde: c.donde}); err != nil {
			e.quiebres = e.quiebres[:len(e.quiebres)-1]
			return err
		}
		// el `goto` al final solo lo escribe javac detrás del brazo que acaba en `break`; el que
		// termina en `return` no lo necesita, el que cae al siguiente tampoco y el último ya cae
		// en el final del switch
		if c.rompe && i != len(orden)-1 {
			e.saltoEmite(opGOTO, fin, 0)
		}
	}
	e.quiebres = e.quiebres[:len(e.quiebres)-1]
	e.cierra(fin)
	return nil
}

// resuelveAtras completa los saltos que apuntan a una etiqueta **ya cerrada**, que es el `goto` del
// final de un bucle: `cierra` solo parchea cuando se cierra, y la cabeza se cerró antes del cuerpo. El
// operando es un desplazamiento de 16 bits con signo contado desde el principio de la branquia.
func (e *emisor) resuelveAtras(id int) error {
	destino, ok := e.etiquetas[id]
	if !ok {
		return fmt.Errorf("un salto hacia atras a una etiqueta que no se cerro")
	}
	quedan := e.parches[:0]
	for _, p := range e.parches {
		if p.etiqueta != id {
			quedan = append(quedan, p)
			continue
		}
		rel := destino - p.en
		if rel < -32768 || rel > 32767 {
			return fmt.Errorf("un salto hacia atras de %d bytes: goto_w no esta medido", rel)
		}
		e.codigo[p.en+1] = byte(rel >> 8)
		e.codigo[p.en+2] = byte(rel)
	}
	e.parches = quedan
	return nil
}

// incremento: `i++`, `--i`, `i += n` y el paso de un `for`, como sentencia. En un local javac escribe
// `iinc` cuando la constante cabe en un byte (medido en el parrafo 67: `iinc 1, 1`, `iinc 1, -1`, y
// en `Mas`, `i += 2` → `iinc 0, 2`); si no cabe, usa `iload; iconst; iadd; istore`. En un campo la
// forma de javac es `destino = destino ± delta`, que el traductor ya sabe bajar.
func (e *emisor) incremento(v jincr) error {
	e.marcaLinea(v.donde.linea)
	if acc, esAcceso := v.destino.(jacceso); esAcceso {
		segs, base := aplanar(acc)
		if base == nil && len(segs) == 1 {
			if lv, ok := e.locales[segs[0]]; ok {
				if !esEnteroDesc(lv.desc) {
					return fmt.Errorf("linea %d: `++`/`--` sobre %s, que no es un entero", v.donde.linea, lv.desc)
				}
				if v.delta >= -128 && v.delta <= 127 {
					if lv.indice > 255 {
						return fmt.Errorf("linea %d: el local %s no cabe en el indice de un `iinc`", v.donde.linea, segs[0])
					}
					e.emite(0x84, byte(lv.indice), byte(int8(v.delta)))
					return nil
				}
			}
		}
	}
	op := "+"
	delta := v.delta
	if delta < 0 {
		op = "-"
		delta = -delta
	}
	lit := jlit{tipoJ: 'i', entero: delta, donde: v.donde}
	if d, ok := e.tipoPrevisto(v.destino); ok && d == "J" {
		// en un largo no hay `iinc`: javac escribe `lconst_1` y `ladd` (parrafo 74), no un entero que
		// luego hay que ensanchar
		lit = jlit{tipoJ: 'j', largo: int64(delta), donde: v.donde}
	}
	return e.stm(jasign{
		destino: v.destino,
		valor:   joperador{izq: v.destino, op: op, der: lit, donde: v.donde},
		donde:   v.donde,
	})
}

// entradaTry: una fila de la exception_table del Code (JVMS 4.7.3). `fin` es exclusivo y `tipo` es el
// descriptor de la clase atrapada; vacio es el `any` que escribe javac para el `finally` (parrafo 75).
type entradaTry struct {
	inicio, fin, handler int
	tipo                 string
}

// pendienteTry: un `finally` por delante de una salida abrupta. `trozo` es el pc donde empieza el tramo
// del cuerpo que todavia no cerró ninguna salida, y `registra` anota la fila `any` de ese tramo con el
// manejador por rellenar. Medido en el parrafo 76 sobre `Fin5` y `Fin6`: cada `return` o `break` que cruza
// el bloque cierra su tramo ([trozo, pc donde empieza la copia]), que puede estar mas adelante que el
// principio del `try` porque el tramo anterior ya lo cerró otra salida.
type pendienteTry struct {
	bloque      jstmt
	lineaCierre int
	trozo       int
	registra    func(inicio, fin int)
}

// finalmenteVivo: el `finally` mas cercano por delante de donde esta el emisor.
func (e *emisor) finalmenteVivo() *pendienteTry {
	if len(e.finalmente) == 0 {
		return nil
	}
	return e.finalmente[len(e.finalmente)-1]
}

// sueltaTrozo: anota la fila `any` del tramo abierto. Un tramo vacio no se anota: medido en
// `Fin6.soloVoid`, cuyo cuerpo es solamente el `return` y cuya tabla del `Code` sale sin filas.
func (e *emisor) sueltaTrozo(c *pendienteTry) {
	if len(e.codigo) > c.trozo {
		c.registra(c.trozo, len(e.codigo))
	}
}

// copiaElFinally: la copia del bloque delante de la salida abrupta. El trozo siguiente lo abre el que
// escribe la salida, porque en javac el siguiente rango empieza **despues** del `return` o del `goto`
// (medido en `Fin5.ramaVoid`: la copia en 4, el `return` en 9 y el trozo que sigue arranca en 10).
func (e *emisor) copiaElFinally(c *pendienteTry) error {
	e.sueltaTrozo(c)
	return e.stm(c.bloque)
}

// copiaLosFinally: la cadena de `finally` que cruza una salida, de dentro a fuera. Hoy el emisor
// solo llega aquí con un `finally` (el anidado se rechaza antes por nombre, §133); la cadena queda
// escrita para cuando esa forma entre medida, con el tramo exterior incluyendo la copia del interior
// como hace javac.
func (e *emisor) copiaLosFinally() error {
	for i := len(e.finalmente) - 1; i >= 0; i-- {
		c := e.finalmente[i]
		e.sueltaTrozo(c)
		if err := e.stm(c.bloque); err != nil {
			return err
		}
	}
	return nil
}

// reabreLosFinally: después de la salida abrupta, todos los tramos siguen en la instrucción
// siguiente; es el `trozo` que javac reabre detrás del `return`/`goto`.
func (e *emisor) reabreLosFinally() {
	for _, c := range e.finalmente {
		c.trozo = len(e.codigo)
	}
}

// motivoPendiente: que impide escribir la salida abrupta que hay dentro del cuerpo de un `try` con
// `finally`. Devuelve vacio cuando no hay ninguna, y es el nombre del caso cuando hay que rechazarlo:
// el `continue` que sale (no esta medido), un `return` o `break` que esta dentro de **otro** `try`
// anidado —con `Fin6.anidado` javac copia los dos bloques seguidos y la tabla queda con rangos que se
// solapan, y en el dex los `try_item` no pueden solaparse—, o el que sale desde dentro del propio
// `finally`.
func motivoPendiente(s jstmt, enBucle bool, bajoOtroTry bool, dentroDelFinally bool) string {
	motivo := ""
	apunta := func(c string) {
		if motivo == "" {
			motivo = c
		}
	}
	switch v := s.(type) {
	case jretorno:
		if dentroDelFinally {
			return "un `return` dentro del propio `finally` no esta medido"
		}
		if bajoOtroTry {
			return "un `return` que cruza otro `try` anidado no esta medido"
		}
		return ""
	case jrompe:
		if enBucle {
			return ""
		}
		if dentroDelFinally {
			return "un `break` dentro del propio `finally` no esta medido"
		}
		if bajoOtroTry {
			return "un `break` que cruza otro `try` anidado no esta medido"
		}
		return ""
	case jcontinua:
		if enBucle {
			return ""
		}
		if dentroDelFinally {
			return "un `continue` dentro del propio `finally` no esta medido"
		}
		if bajoOtroTry {
			return "un `continue` que cruza otro `try` anidado no esta medido"
		}
		return ""
	case jbloque:
		for _, x := range v.sentencias {
			apunta(motivoPendiente(x, enBucle, bajoOtroTry, dentroDelFinally))
		}
	case jif:
		apunta(motivoPendiente(v.entonces, enBucle, bajoOtroTry, dentroDelFinally))
		if v.siNo != nil {
			apunta(motivoPendiente(v.siNo, enBucle, bajoOtroTry, dentroDelFinally))
		}
	case jmientras:
		apunta(motivoPendiente(v.cuerpo, true, bajoOtroTry, dentroDelFinally))
	case jhacer:
		apunta(motivoPendiente(v.cuerpo, true, bajoOtroTry, dentroDelFinally))
	case jpara:
		apunta(motivoPendiente(v.cuerpo, true, bajoOtroTry, dentroDelFinally))
	case jswitch:
		for _, c := range v.casos {
			apunta(motivoPendiente(jbloque{sentencias: c.cuerpo}, true, bajoOtroTry, dentroDelFinally))
		}
		if v.defecto != nil {
			apunta(motivoPendiente(jbloque{sentencias: v.defecto}, true, bajoOtroTry, dentroDelFinally))
		}
	case jtry:
		// un `try` interior con `catch` sigue fuera; uno que solo tiene `finally` deja que la salida
		// cruce los dos bloques (javac los copia de dentro a fuera, §139)
		anidado := bajoOtroTry || len(v.atrapas) > 0
		apunta(motivoPendiente(v.cuerpo, enBucle, anidado, dentroDelFinally))
		for _, c := range v.atrapas {
			apunta(motivoPendiente(c.atrapa, enBucle, true, dentroDelFinally))
		}
		if v.finalmente != nil {
			apunta(motivoPendiente(v.finalmente, enBucle, true, true))
		}
	}
	return motivo
}

// terminaEnTry: el cuerpo acaba en un `try` (posiblemente dentro de un bloque). En ese caso la
// última instrucción emitida puede ser el `athrow` del manejador del `try` interior, que javac deja
// dentro del rango del exterior a propósito; no es el `throw` del fuente que rechaza `intenta` (§139).
func terminaEnTry(s jstmt) bool {
	switch v := s.(type) {
	case jtry:
		return true
	case jbloque:
		if len(v.sentencias) == 0 {
			return false
		}
		return terminaEnTry(v.sentencias[len(v.sentencias)-1])
	}
	return false
}

// bloqueVacio: un `finally { }` sin sentencias. javac lo suelta entero —ni copia, ni goto, ni fila en
// la tabla—, medido en `Fin3.vacioFinalmente`: el `Code` sale con el cuerpo del try y nada mas.
func bloqueVacio(s jstmt) bool {
	b, ok := s.(jbloque)
	return ok && len(b.sentencias) == 0
}

// intenta: `try { … } catch (T e) { … } [catch …] [finally { … }]`. La forma de javac, medida en el
// parrafo 71 sobre `T` y en el 75 sobre `Fin`, `Fin2` y `Fin3`: el cuerpo protegido y despues un bloque
// por camino —cada `catch` con su `astore` de la excepción y su cuerpo—; con `finally` la copia del
// bloque va al pie de cada camino que pueda caer (el normal también) y el ultimo manejador es el `any`,
// que guarda la excepción en un local que nadie mas usa, copia el bloque, y la re-lanza. Las filas de
// la tabla salen en ese orden: los tipos del try, el `any` del try, y el `any` de cada cuerpo del catch.
func (e *emisor) intenta(v jtry) error {
	// un `catch (A | B e)` lleva una entrada por tipo y un solo manejador, con la excepcion guardada
	// en un local del supertipo comun (§132). Aqui se usa `Throwable` como tipo del hueco: cualquier
	// excepcion es asignable a el, y el dex no guarda el tipo del local (sus lineas no traen tabla de
	// variables), asi que no cambia ni una instruccion.
	descs := make([][]string, len(v.atrapas))
	locales := make([]string, len(v.atrapas))
	for i, c := range v.atrapas {
		for _, t := range c.tipos {
			desc, err := e.ctx.descriptorDe(t)
			if err != nil {
				return fmt.Errorf("linea %d: %w", v.donde.linea, err)
			}
			descs[i] = append(descs[i], desc)
		}
		locales[i] = descs[i][0]
		if len(descs[i]) > 1 {
			locales[i] = "Ljava/lang/Throwable;"
		}
	}
	finalmente := v.finalmente
	if bloqueVacio(finalmente) {
		finalmente = nil
	}
	if finalmente != nil {
		caminos := append([]jstmt{v.cuerpo}, func() []jstmt {
			out := make([]jstmt, 0, len(v.atrapas))
			for _, c := range v.atrapas {
				out = append(out, c.atrapa)
			}
			return out
		}()...)
		for _, camino := range caminos {
			if motivo := motivoPendiente(camino, false, false, false); motivo != "" {
				return fmt.Errorf("linea %d: %s", v.donde.linea, motivo)
			}
		}
		// el bloque copiado tampoco puede tener una salida propia: javac la escribe, pero esa forma
		// no esta medida
		if motivo := motivoPendiente(finalmente, false, false, true); motivo != "" {
			return fmt.Errorf("linea %d: %s", v.donde.linea, motivo)
		}
	}
	base := e.siguiente
	alto := base
	salida := e.nuevaEtiqueta()
	// las filas `any` del cuerpo y de los cuerpos del catch, con el manejador por rellenar al final
	var filasCuerpo, filasCatch []entradaTry
	nuevoTrozo := func(registra func(int, int)) *pendienteTry {
		return &pendienteTry{bloque: finalmente, lineaCierre: v.lineaCierre, registra: registra}
	}
	ctxCuerpo := nuevoTrozo(func(a, b int) {
		filasCuerpo = append(filasCuerpo, entradaTry{inicio: a, fin: b, tipo: ""})
	})

	inicio := len(e.codigo)
	fin := inicio
	if finalmente != nil {
		ctxCuerpo.trozo = inicio
		e.finalmente = append(e.finalmente, ctxCuerpo)
	}
	if err := e.stm(v.cuerpo); err != nil {
		return err
	}
	if finalmente != nil {
		e.finalmente = e.finalmente[:len(e.finalmente)-1]
	}
	// un `try` anidado ya usó huecos por su cuenta: el temporal del `finally` de arriba va en el
	// mas alto que haya visto el metodo (medido en `Fini.anidado`, donde el interior usa el 2 y el
	// `any` exterior escribe `astore_3`)
	if e.siguiente > alto {
		alto = e.siguiente
	}
	fin = len(e.codigo)
	if finalmente == nil {
		// javac saca del rango protegido el `return`/`goto` final (no pueden lanzar) pero deja dentro
		// el `athrow`: medido en el parrafo 71 (`lee`: 0..3, con el `ireturn` en 3; `T.f`: 0..10, con el
		// `athrow` en 9)
		fin = e.finProtegido(inicio, fin)
	} else if e.ultimaEsLanza(inicio, fin) && !terminaEnTry(v.cuerpo) {
		// con `finally`, un `throw` como ultima sentencia del cuerpo deja en javac un rango que se
		// pasa a la primera instruccion de su propio manejador (medido: `Fin.lanza` es 0..9 con el
		// `astore` en 8, y `Fin3.soloLanzaConCatch` mezcla el cuerpo con el `catch` en 0..11). Esa
		// forma no la reproduce este emisor, asi que se rechaza por nombre
		return fmt.Errorf("linea %d: un `throw` como ultima sentencia de un `try` con `finally` no esta medido", v.donde.linea)
	}
	// las filas de la tabla salen en el orden de javac: los tipos del cuerpo, el `any` del cuerpo y
	// despues el `any` de cada cuerpo del catch (medido en `Fin.multi` y `Fin3.lanzaTyped`)
	tacha := func(cuerpo jstmt, c *pendienteTry) error {
		if terminaEnSalida(cuerpo) {
			return nil
		}
		if finalmente != nil {
			e.sueltaTrozo(c)
			if err := e.stm(finalmente); err != nil {
				return err
			}
			c.trozo = len(e.codigo)
			if terminaEnSalida(finalmente) {
				return nil
			}
		}
		e.marcaLinea(v.lineaCierre)
		e.saltoEmite(opGOTO, salida, 0)
		return nil
	}

	// camino normal: la copia del `finally` y el `goto` a la salida, solo si el cuerpo puede caer y
	// si queda algo despues (un `try` sin `catch` ni `finally` despues de recortar el bloque vacio
	// no deja nada que saltar, y javac no escribe el `goto`: `Fin3.vacioFinalmente`)
	if len(v.atrapas) > 0 || finalmente != nil {
		if err := tacha(v.cuerpo, ctxCuerpo); err != nil {
			return err
		}
	}
	// con `catch` y `finally` javac escribe un solo rango para el cuerpo; si el cuerpo sale por dos
	// trozos —un `return` en una rama y ademas la caida— hay que repartir los tipos por trozos, y esa
	// forma no esta medida. El rango del `any` que queda es el que manda sobre los tipos.
	if finalmente != nil && len(v.atrapas) > 0 {
		if len(filasCuerpo) != 1 {
			return fmt.Errorf("linea %d: un `try` con `catch` y `finally` cuyo cuerpo sale por dos trozos no esta medido", v.donde.linea)
		}
		fin = filasCuerpo[0].fin
	}
	if finalmente != nil && len(v.atrapas) == 0 && len(filasCuerpo) == 0 {
		// `Fin6.soloVoid` (`try { return; } finally { a = 5; }`): no queda nada protegido, y javac
		// escribe el manejador `any` **sin fila en la tabla** —codigo muerto—. Este emisor no lo
		// reproduce: el caminante de marcos llama la atencion sobre el pc inalcanzable
		return fmt.Errorf("linea %d: un cuerpo que no deja nada protegido dentro de un `try` con `finally` no esta medido", v.donde.linea)
	}
	// los `catch`, cada uno desde el hueco de local que deja el `try`: javac reutiliza el mismo
	// numero para la excepcion de cada brazo (medido en `Fin.multi`: los dos `astore_1`)
	manejadores := make([]int, len(v.atrapas))
	for i, c := range v.atrapas {
		e.siguiente = base
		h := len(e.codigo)
		manejadores[i] = h
		e.marcaLinea(c.lineaAtrapa)
		indice := e.reserva(c.excep, locales[i])
		e.sube() // la excepción que el verificador deja en la pila al entrar al handler
		e.guardaDe(indice, locales[i])
		ctxCatch := nuevoTrozo(func(a, b int) {
			filasCatch = append(filasCatch, entradaTry{inicio: a, fin: b, tipo: ""})
		})
		ctxCatch.trozo = h
		if finalmente != nil {
			e.finalmente = append(e.finalmente, ctxCatch)
		}
		if err := e.stm(c.atrapa); err != nil {
			return err
		}
		if finalmente != nil {
			e.finalmente = e.finalmente[:len(e.finalmente)-1]
		}
		if e.siguiente > alto {
			alto = e.siguiente
		}
		if finalmente == nil {
			// el ultimo `catch` cae a la salida del constructo: no lleva `goto`, medido en
			// `Fini.dosCatch` (javac para en `iload_1; ireturn` despues del segundo brazo)
			if i == len(v.atrapas)-1 {
				break
			}
			if err := tacha(c.atrapa, ctxCatch); err != nil {
				return err
			}
			continue
		}
		if e.ultimaEsLanza(h, len(e.codigo)) {
			return fmt.Errorf("linea %d: un `throw` como ultima sentencia de un `catch` con `finally` no esta medido", v.donde.linea)
		}
		if err := tacha(c.atrapa, ctxCatch); err != nil {
			return err
		}
	}
	filas := make([]entradaTry, 0, len(v.atrapas)+len(filasCuerpo)+len(filasCatch))
	for i := range v.atrapas {
		for _, desc := range descs[i] {
			filas = append(filas, entradaTry{inicio: inicio, fin: fin, handler: manejadores[i], tipo: desc})
		}
	}
	if finalmente != nil {
		// el manejador `any`: guarda la excepcion en el hueco mas alto que vio el metodo, copia el
		// bloque con su renglon, y la re-lanza con el del cierre (medido en `Fin.multi`: `astore_2`
		// en 42 con la linea del `a = 7;` y `aload_2` en 48 con la de la llave)
		e.siguiente = alto
		hAny := len(e.codigo)
		e.marcaLinea(v.lineaAviso)
		indice := e.reserva("", "Ljava/lang/Throwable;")
		e.sube()
		e.guardaDe(indice, "Ljava/lang/Throwable;")
		if err := e.stm(finalmente); err != nil {
			return err
		}
		if !terminaEnSalida(finalmente) {
			e.marcaLinea(v.lineaCierre)
			e.cargaDe(indice, "Ljava/lang/Throwable;")
			e.emite(0xbf)
			e.baja(1)
		}
		for _, f := range filasCuerpo {
			f.handler = hAny
			filas = append(filas, f)
		}
		for _, f := range filasCatch {
			f.handler = hAny
			filas = append(filas, f)
		}
	}
	e.cierra(salida)
	e.excepciones = append(e.excepciones, filas...)
	return nil
}

// ultimaEsLanza: si la ultima instruccion escrita del tramo [inicio,fin) es un `athrow`. Con `finally`
// javac deja ese caso en una forma que este emisor no reproduce (parrafo 75).
func (e *emisor) ultimaEsLanza(inicio, fin int) bool {
	lista, err := walkJvmCode(e.codigo[:fin])
	if err != nil {
		return false
	}
	for _, in := range lista {
		if in.pc >= inicio && in.name == "athrow" && in.pc+1 >= fin {
			return true
		}
	}
	return false
}

// finProtegido: el fin del rango de un try, recortando un `return`/`goto` final. Es lo que hace
// javac: el rango acaba donde empieza esa instrucción porque no puede lanzar, mientras que un
// `athrow` sí queda dentro (si no, el catch no vería su propio `throw`).
func (e *emisor) finProtegido(inicio, fin int) int {
	lista, err := walkJvmCode(e.codigo)
	if err != nil || len(lista) == 0 {
		return fin
	}
	ultima := lista[len(lista)-1]
	if ultima.pc < inicio {
		return fin
	}
	switch ultima.name {
	case "goto", "return", "ireturn", "areturn":
		return ultima.pc
	}
	return fin
}

// lanza: `throw expr;` es `athrow` (0xbf), medido en el parrafo 71 sobre `T.lanza`.
func (e *emisor) lanza(v jlanza) error {
	e.marcaLinea(v.donde.linea)
	d, err := e.exp(v.e)
	if err != nil {
		return err
	}
	if !esTipoDeReferencia(d) {
		return fmt.Errorf("linea %d: se lanza un %s, que no es una referencia", v.donde.linea, d)
	}
	e.emite(0xbf)
	e.baja(1)
	return nil
}

func (e *emisor) stm(s jstmt) error {
	switch v := s.(type) {
	case jstmtExpr:
		// `i++` como sentencia (y el paso de un `for`) no es una expresion que se evalue y se tire:
		// javac escribe `iinc` en un local, medido en el parrafo 67
		if inc, esInc := v.e.(jincr); esInc {
			return e.incremento(inc)
		}
		e.marcaLinea(lineaDeExpr(v.e))
		d, err := e.exp(v.e)
		if err != nil {
			return err
		}
		if d != "V" && d != "" {
			// un valor de dos palabras se tira con `pop2` (`D$1.run` tras el access$002, párrafo 83)
			if esAncho(d) {
				e.emite(opPop2)
				e.baja(2)
			} else {
				e.emite(opPop)
				e.baja(1)
			}
		}
		return nil
	case jretorno:
		c := e.finalmenteVivo()
		if c == nil {
			e.marcaLinea(v.donde.linea)
		}
		if v.e == nil {
			if e.vuelta != "V" {
				return fmt.Errorf("linea %d: un `return` suelto en un metodo que devuelve %s", v.donde.linea, e.vuelta)
			}
			if c != nil {
				// `return;` con un `finally` por delante: primero el bloque copiado y despues el
				// `return`, sin hueco pendiente y sin fila para el `return` hasta su instruccion
				// (medido en `Fin6.ramaVoid`: la copia en 4 con la linea del bloque, el `return` en 9
				// con la del `return;`); la cadena copia de dentro a fuera por si algún día hay dos
				if err := e.copiaLosFinally(); err != nil {
					return err
				}
			}
			e.marcaLinea(v.donde.linea)
			e.emite(retorno('V'))
			if c != nil {
				e.reabreLosFinally()
			}
			return nil
		}
		e.marcaLinea(v.donde.linea)
		d, err := e.expA(v.e, e.vuelta)
		if err != nil {
			return err
		}
		if e.vuelta == "V" {
			return fmt.Errorf("linea %d: se devuelve algo desde un metodo void", v.donde.linea)
		}
		if len(d) > 0 && len(e.vuelta) > 0 && d[0] != e.vuelta[0] && !(d == "Ljava/lang/String;" && e.vuelta[0] == 'L') {
			return fmt.Errorf("linea %d: el metodo devuelve %s y se devuelve un %s", v.donde.linea, e.vuelta, d)
		}
		if c != nil {
			// el valor espera en un hueco que no nombra nadie mientras se copia el bloque: medido en
			// `Fin5.rama` (`4 iconst_1; 5 istore_1`, la copia en 6, `10 iload_1; 11 ireturn` con la
			// linea del `return`)
			indice := e.reserva("", d)
			e.guardaDe(indice, d)
			if err := e.copiaLosFinally(); err != nil {
				return err
			}
			e.marcaLinea(v.donde.linea)
			e.cargaDe(indice, d)
		}
		e.emite(retorno(d[0]))
		e.bajaPor(d)
		if c != nil {
			e.reabreLosFinally()
		}
		return nil
	case jdecl:
		e.marcaLinea(v.donde.linea)
		d, err := e.ctx.descriptorDe(v.tipo)
		if err != nil {
			return fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		indice := e.reserva(v.nombre, d)
		if v.valor == nil {
			return nil
		}
		dv, err := e.expA(v.valor, d)
		if err != nil {
			return err
		}
		if d[0] != dv[0] {
			return fmt.Errorf("linea %d: se declara %s y se le da un %s", v.donde.linea, d, dv)
		}
		e.guardaDe(indice, d)
		return nil
	case jasign:
		e.marcaLinea(v.donde.linea)
		if ind, esIndice := v.destino.(jindice); esIndice {
			return e.escribeIndice(ind, v.valor)
		}
		acc, esAcceso := v.destino.(jacceso)
		segs, base := aplanar(acc)
		if !esAcceso || base != nil || len(segs) != 1 {
			return fmt.Errorf("linea %d: una asignacion con mas de un nombre en el destino no esta medida", v.donde.linea)
		}
		if lv, ok := e.locales[segs[0]]; ok {
			dv, err := e.expA(v.valor, lv.desc)
			if err != nil {
				return err
			}
			if dv[0] != lv.desc[0] {
				return fmt.Errorf("linea %d: %s es %s y se le asigna un %s", v.donde.linea, acc.nombre, lv.desc, dv)
			}
			e.guardaDe(lv.indice, lv.desc)
			return nil
		}
		// asignar a un campo: `pulsos = pulsos + 1`, medido como el receptor ANTES del valor, el valor, y
		// `putfield`; en un campo estatico no hay receptor. Desde una anonima el campo puede ser del
		// exterior, y entonces el receptor es la captura (parrafo 54).
		dueño, cr, encontrado := e.duenioDelCampo(segs[0])
		if !encontrado {
			return fmt.Errorf("linea %d: asignar a %s: no es una variable local ni un campo de esta clase ni del exterior que la anonima captura",
				v.donde.linea, segs[0])
		}
		if cr.constante {
			return fmt.Errorf("linea %d: %s es una constante final y javac no la deja reasignar", v.donde.linea, segs[0])
		}
		esEstatico := cr.acceso&accStatic != 0
		if dueño != e.interno && cr.acceso&accPrivate != 0 {
			// campo privado del exterior: la escritura pasa por `access$NN2`, que devuelve el valor y deja
			// un `pop` en el llamador (parrafo 72) —`pop2` cuando el campo es ancho (párrafo 83)—. El
			// accesor se apunta **antes** de bajar el valor porque
			// asi lo crea javac: en `secreto = secreto + 2` el de escritura es el 002 y el de lectura el 000.
			nombreAcc, descAcc := e.ctx.accesorPara(dueño, segs[0], cr.desc, true, true, esEstatico)
			enPila := 1
			if !esEstatico {
				if e.estatico {
					return fmt.Errorf("linea %d: asignar a %s sin `this`, en un metodo estatico, no es Java", v.donde.linea, segs[0])
				}
				e.cargaDelDuenio(dueño)
				enPila = 2
			}
			dv, err := e.exp(v.valor)
			if err != nil {
				e.baja(enPila - 1)
				return err
			}
			if dv != cr.desc {
				e.baja(enPila - 1)
				return fmt.Errorf("linea %d: %s es %s y se le asigna un %s", v.donde.linea, segs[0], cr.desc, dv)
			}
			e.invoca(opInvokestat, dueño, nombreAcc, descAcc, enPila)
			if esAncho(cr.desc) {
				e.emite(opPop2)
				e.baja(2)
			} else {
				e.emite(opPop)
				e.baja(1)
			}
			return nil
		}
		if !esEstatico {
			if e.estatico {
				return fmt.Errorf("linea %d: asignar a %s sin `this`, en un metodo estatico, no es Java", v.donde.linea, segs[0])
			}
			e.cargaDelDuenio(dueño)
		}
		dv, err := e.exp(v.valor)
		if err != nil {
			if !esEstatico {
				e.baja(1) // el receptor que ya se habia contado
			}
			return err
		}
		if dv != cr.desc {
			if !esEstatico {
				e.baja(1)
			}
			return fmt.Errorf("linea %d: %s es %s y se le asigna un %s", v.donde.linea, segs[0], cr.desc, dv)
		}
		idx := e.pool.campo(dueño, segs[0], cr.desc)
		if esEstatico {
			e.putstaticDe(idx, cr.desc)
		} else {
			e.putfieldDe(idx, cr.desc)
		}
		return nil
	case jbloque:
		for _, st := range v.sentencias {
			if err := e.stm(st); err != nil {
				return err
			}
		}
		return nil
	case jif:
		return e.branca(v)
	case jmientras:
		return e.mientras(v)
	case jpara:
		return e.para(v)
	case jhacer:
		return e.hacer(v)
	case jrompe:
		if len(e.quiebres) == 0 {
			return fmt.Errorf("linea %d: un `break` fuera de un bucle o de un `switch`", v.donde.linea)
		}
		if c := e.finalmenteVivo(); c != nil {
			// el `break` que sale de un `try` con `finally` copia el bloque delante del salto, y el
			// `goto` lleva la linea del `break` (medido en `Fin5.breakConFinally`: `11 iconst_3;
			// 12 putstatic; 15 goto 43`, y en `Fin2.ruptura` la fila 54 en el pc del `goto`). El
			// trozo que sigue al `try` arranca despues del salto: la fila de javac es `18..22`
			if err := e.copiaLosFinally(); err != nil {
				return err
			}
			e.marcaLinea(v.donde.linea)
			e.saltoEmite(opGOTO, e.quiebres[len(e.quiebres)-1], 0)
			e.reabreLosFinally()
			return nil
		}
		e.saltoEmite(opGOTO, e.quiebres[len(e.quiebres)-1], 0)
		return nil
	case jtry:
		return e.intenta(v)
	case jlanza:
		return e.lanza(v)
	case jcontinua:
		if len(e.continuar) == 0 {
			return fmt.Errorf("linea %d: un `continue` fuera de un bucle", v.donde.linea)
		}
		destino := e.continuar[len(e.continuar)-1]
		if c := e.finalmenteVivo(); c != nil {
			// el `continue` que sale de un `try` con `finally` es como el `break`: la copia del
			// bloque delante del salto, el `goto` con la linea del `continue` y el tramo siguiente
			// abierto despues del salto (§132)
			if err := e.copiaLosFinally(); err != nil {
				return err
			}
			e.marcaLinea(v.donde.linea)
			e.saltoEmite(opGOTO, destino, 0)
			if _, cerrada := e.etiquetas[destino]; cerrada {
				if err := e.resuelveAtras(destino); err != nil {
					return fmt.Errorf("linea %d: %w", v.donde.linea, err)
				}
			}
			e.reabreLosFinally()
			return nil
		}
		e.saltoEmite(opGOTO, destino, 0)
		// la vuelta de un `while` apunta a una etiqueta ya cerrada: se parchea ahora, no al cerrar
		if _, cerrada := e.etiquetas[destino]; cerrada {
			if err := e.resuelveAtras(destino); err != nil {
				return fmt.Errorf("linea %d: %w", v.donde.linea, err)
			}
		}
		return nil
	case jswitch:
		return e.cambio(v)
	}
	return fmt.Errorf("una sentencia de %T no esta medida", s)
}

// descDeAccesoEstatico: el descriptor de `R.layout.main` sin emitir nada, para saber que tipo lleva un
// argumento de un `new` antes de haberlo escrito.
func (e *emisor) descDeAccesoEstatico(interno string, segs []string) (string, error) {
	if len(segs) == 0 {
		return "", fmt.Errorf("el tipo %s suelto no es un valor", interno)
	}
	tipo := interno
	for i, seg := range segs {
		ultimo := i == len(segs)-1
		if anidada := tipo + "$" + seg; e.ctx.hay(anidada) && !ultimo {
			tipo = anidada
			continue
		}
		cr, err := e.ctx.buscarCampo(tipo, seg)
		if err != nil {
			return "", err
		}
		if cr.constante {
			return "I", nil
		}
		return cr.desc, nil
	}
	return "", fmt.Errorf("no se resuelve %s.%s", interno, strings.Join(segs, "."))
}

// ----- escribir el ClassFile -----

// emmitirClase escribe el ClassFile de una declaracion. El pool se arma emitiendo los miembros, asi que el
// InnerClasses se calcula al final, cuando ya se sabe que anidadas menciona el codigo.
func (ctx *contextoJava) emmitirClase(c *jclas) ([]byte, error) {
	interno := internoDe(c.paquete, c.nombre)
	e := &emisor{ctx: ctx, pool: nuevoPool(), c: c, interno: interno, vistas: map[string]bool{}}

	thisIdx := e.pool.clase(interno)
	superIdx := e.pool.clase("java/lang/Object")
	if c.super != "" {
		s, err := ctx.tipoAInterno(c.super)
		if err != nil {
			return nil, err
		}
		e.usar(s)
		superIdx = e.pool.clase(s)
	}
	iface := []int{}
	for _, im := range c.implementa {
		nombre, err := ctx.tipoAInterno(im)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", c.linea, err)
		}
		if c.anonima {
			// de una anonima solo se midio la que implementa una interface (parrafo 42); la que extiende una
			// clase va por el super y ahi el constructor cambia, asi que se nombra en vez de escribir mal
			accesos, err := ctx.accesosDe(nombre)
			if err != nil {
				return nil, fmt.Errorf("linea %d: %w", c.linea, err)
			}
			if accesos&accInterface == 0 {
				return nil, fmt.Errorf("linea %d: una anonima sobre %s, que no es una interface, no esta medida (javac la pone como super)", c.linea, im)
			}
		}
		e.usar(nombre)
		iface = append(iface, e.pool.clase(nombre))
	}

	access := accSuper
	if c.esInterface {
		// `javap -v` de la interface del parrafo 92: 0x0601 = public interface abstract; sin
		// ACC_SUPER y sin ACC_FINAL. Una interface package-private es 0x0600.
		access = accInterface | accAbstract
		if c.esPublica {
			access |= accPublic
		}
	} else {
		if c.esPublica {
			access |= accPublic
		}
		if c.final {
			access |= accFinal
		}
		if c.abstracta {
			access |= accAbstract
		}
	}

	campos := []byte{}
	nCampos := 0
	if c.anonima {
		// javac pone primero los `val$x` de las locales capturadas y después el `this$0` del exterior
		// (medido en `Captura3$1`, parrafo 100). Una anónima de un método static no tiene this$0.
		for _, f := range c.campos {
			if !strings.HasPrefix(f.nombre, "val$") {
				continue
			}
			b, err := e.campoDeClassFile(f)
			if err != nil {
				return nil, fmt.Errorf("linea %d: %w", f.donde.linea, err)
			}
			campos = append(campos, b...)
			nCampos++
		}
		if !(c.metodoEncierre != nil && c.metodoEncierre.esEstatico) {
			campos = append(campos, e.campoCaptura()...)
			nCampos++
		}
	}
	for _, f := range c.campos {
		if c.anonima && strings.HasPrefix(f.nombre, "val$") {
			continue
		}
		b, err := e.campoDeClassFile(f)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", f.donde.linea, err)
		}
		campos = append(campos, b...)
		nCampos++
	}

	// javac escribe el constructor primero, sintetizado o no: el orden va aqui, no detras
	metodos := []byte{}
	conte := 0
	constructorEscrito := false
	for _, m := range c.metodos {
		if m.nombre == "<init>" {
			constructorEscrito = true
		}
	}
	if c.anonima {
		b, err := e.initCaptura()
		if err != nil {
			return nil, err
		}
		metodos = append(metodos, b...)
		conte++
	} else if !constructorEscrito && !c.esInterface {
		b, err := e.initSintetico()
		if err != nil {
			return nil, err
		}
		metodos = append(metodos, b...)
		conte++
	}
	for _, m := range c.metodos {
		b, err := e.metodoDeClassFile(m)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", m.linea, err)
		}
		metodos = append(metodos, b...)
		conte++
	}
	// los `access$NNN` que pidieron las anidadas, en el orden en que se crearon; javac los escribe entre
	// los metodos declarados y el `<clinit>` (medido en el parrafo 72)
	for _, a := range e.ctx.accesores[e.interno] {
		b, err := e.accesorDeClassFile(a)
		if err != nil {
			return nil, err
		}
		metodos = append(metodos, b...)
		conte++
	}
	// el `static {}` va despues de lo declarado: es el orden que tiene javac en el ClassFile medido
	if !c.anonima && len(c.inicializadoresEstaticos()) > 0 {
		b, err := e.clinitSintetico()
		if err != nil {
			return nil, err
		}
		metodos = append(metodos, b...)
		conte++
	}
	// Las filas del InnerClasses van en el orden en que javac las escribe: las anidadas declaradas aqui, al
	// reves de como se declaran (medido: la R del estudio lista style, layout, id); la propia clase si es
	// anidada; y despues las que menciona el codigo, en orden de primer uso (medido: R$layout, MainActivity$1
	// y la interface, en ese orden, en el parrafo 42).
	var nombres []string
	ya := map[string]bool{}
	anadir := func(n string) {
		if n == "" || ya[n] {
			return
		}
		ya[n] = true
		nombres = append(nombres, n)
	}
	for i := len(c.interior) - 1; i >= 0; i-- {
		anadir(internoDe(c.interior[i].paquete, c.interior[i].nombre))
	}
	if strings.Contains(c.nombre, "$") {
		anadir(interno)
	}
	for _, m := range e.menciona {
		anadir(m)
	}
	entradas := []byte{}
	if len(nombres) > 0 {
		entradas = u2be(uint16(len(nombres)))
		for _, n := range nombres {
			fila, err := e.filaInner(n)
			if err != nil {
				return nil, err
			}
			entradas = append(entradas, fila...)
		}
	}

	attrs := attrInfo(e.pool, "SourceFile", u2be(uint16(e.pool.utf8(ctx.archivo))))
	conteAttrs := 1
	if c.anonima {
		cuerpo, err := e.cuerpoEnclosingMethod()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attrInfo(e.pool, "EnclosingMethod", cuerpo)...)
		conteAttrs++
	}
	if len(nombres) > 0 {
		attrs = append(attrs, attrInfo(e.pool, "InnerClasses", entradas)...)
		conteAttrs++
	}

	out := []byte{0xca, 0xfe, 0xba, 0xbe}
	out = append(out, u2be(0)...)
	out = append(out, u2be(versionDeClassEmitida)...)
	out = append(out, e.pool.serializar()...)
	out = append(out, u2be(uint16(access))...)
	out = append(out, u2be(uint16(thisIdx))...)
	out = append(out, u2be(uint16(superIdx))...)
	out = append(out, u2be(uint16(len(iface)))...)
	for _, i := range iface {
		out = append(out, u2be(uint16(i))...)
	}
	out = append(out, u2be(uint16(nCampos))...)
	out = append(out, campos...)
	out = append(out, u2be(uint16(conte))...)
	out = append(out, metodos...)
	out = append(out, u2be(uint16(conteAttrs))...)
	out = append(out, attrs...)
	return out, nil
}

// filaInner: inner, outer, nombre simple y access_flags, como los escribe javac. La fila de una anonima va
// sin exterior y sin nombre, y con flags ninguno: medido en el parrafo 42 (`InnerClasses: #2;`).
func (e *emisor) filaInner(internal string) ([]byte, error) {
	if propia, ok := e.ctx.clasePropia(internal); ok && propia.anonima {
		b := u2be(uint16(e.pool.clase(internal)))
		b = append(b, u2be(0)...) // outer
		b = append(b, u2be(0)...) // nombre
		return append(b, u2be(0)...), nil
	}
	outer := ""
	nombreSimplo := internal
	if i := strings.LastIndex(internal, "$"); i >= 0 {
		outer = internal[:i]
		nombreSimplo = internal[i+1:]
	}
	accesos, err := e.ctx.accesosDe(internal)
	if err != nil {
		return nil, err
	}
	oi := 0
	if outer != "" {
		oi = e.pool.clase(outer)
	}
	b := u2be(uint16(e.pool.clase(internal)))
	b = append(b, u2be(uint16(oi))...)
	b = append(b, u2be(uint16(e.pool.utf8(sinPaquete(nombreSimplo))))...)
	return append(b, u2be(uint16(accesos))...), nil
}

// campoCaptura: el `final synthetic Tipo this$0;` con el que javac baja el exterior dentro de la anonima
// (parrafo 42: flags 0x1010, descriptor del exterior y ningun atributo).
func (e *emisor) campoCaptura() []byte {
	desc := "L" + e.encierreInterno() + ";"
	b := u2be(uint16(accFinal | accSynthetic))
	b = append(b, u2be(uint16(e.pool.utf8("this$0")))...)
	b = append(b, u2be(uint16(e.pool.utf8(desc)))...)
	return append(b, u2be(0)...)
}

func (e *emisor) encierreInterno() string {
	if e.c.encierre == nil {
		return "java/lang/Object"
	}
	return internoDe(e.c.encierre.paquete, e.c.encierre.nombre)
}

// initCaptura: el constructor de la anonima. Lleva la captura como unico parametro, le pone el campo antes
// de subir al padre y anade el atributo MethodParameters marcando el parametro como `final mandated`
// (0x8010), todo medido en el parrafo 42 sobre MainActivity$1.
func (e *emisor) initCaptura() ([]byte, error) {
	// Una anonima captura el exterior (`this$0`, si el metodo no es static) y las locales
	// effectively-final (`val$x`). Javac escribe primero las asignaciones de captura y despues el
	// `super()`, y para capturas de un metodo static NO escribe `MethodParameters` (parrafo 100).
	estatica := e.c.metodoEncierre != nil && e.c.metodoEncierre.esEstatico
	desc := "("
	externo := ""
	if !estatica {
		externo = "L" + e.encierreInterno() + ";"
		desc += externo
	}
	descs := make([]string, len(e.c.capturas))
	for i, cap := range e.c.capturas {
		d, err := e.ctx.descriptorDe(cap.tipo)
		if err != nil {
			return nil, err
		}
		descs[i] = d
		desc += d
	}
	desc += ")V"
	e.preparaMetodo(desc, false)
	e.reserva("", "L")
	if !estatica {
		e.reserva("", externo)
	}
	for _, d := range descs {
		e.reserva("", d)
	}
	e.marcaLinea(e.c.linea)
	if !estatica {
		e.cargaDe(0, "L")
		e.cargaDe(1, externo)
		idx := e.pool.campo(e.interno, "this$0", externo)
		e.emite(opPutfield, byte(idx>>8), byte(idx))
		e.baja(2)
	}
	capturaBase := 1
	if !estatica {
		capturaBase++
	}
	for i, cap := range e.c.capturas {
		d := descs[i]
		e.cargaDe(0, "L")
		e.cargaDe(capturaBase+i, d)
		idx := e.pool.campo(e.interno, "val$"+cap.nombre, d)
		e.emite(opPutfield, byte(idx>>8), byte(idx))
		e.baja(2)
	}
	e.cargaDe(0, "L")
	e.invoca(opInvokespec, "java/lang/Object", "<init>", "()V", 1)
	e.emite(retorno('V'))
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	attrs := attrInfo(e.pool, "Code", codigo)
	conte := 1
	if !estatica {
		// MethodParameters: el `this$0` sale `final mandated` (0x8010) y cada `val$x`
		// `final synthetic` (0x1010); medido en `Captura3$1` (parrafo 100).
		mp := []byte{byte(1 + len(e.c.capturas))}
		mp = append(mp, u2be(0)...)
		mp = append(mp, u2be(uint16(accFinal|accMandated))...)
		for range e.c.capturas {
			mp = append(mp, u2be(0)...)
			mp = append(mp, u2be(uint16(accFinal|accSynthetic))...)
		}
		attrs = append(attrs, attrInfo(e.pool, "MethodParameters", mp)...)
		conte = 2
	}
	return e.methodInfo("<init>", desc, 0, attrs, conte), nil
}

// enclosingMethod: el atributo que dice en que metodo de que clase se creo la anonima. javac lo escribe y
// nuestro dexer lo convierte en la anotacion Ldalvik/annotation/EnclosingMethod; (parrafo 42).
func (e *emisor) cuerpoEnclosingMethod() ([]byte, error) {
	if e.c.encierre == nil || e.c.metodoEncierre == nil {
		return nil, fmt.Errorf("linea %d: la anonima no trae la clase y el metodo que la crean", e.c.linea)
	}
	externo := internoDe(e.c.encierre.paquete, e.c.encierre.nombre)
	desc, err := e.ctx.descDeMetodo(*e.c.metodoEncierre)
	if err != nil {
		return nil, err
	}
	e.usar(externo)
	b := u2be(uint16(e.pool.clase(externo)))
	return append(b, u2be(uint16(e.pool.nombreYTipo(e.c.metodoEncierre.nombre, desc)))...), nil
}

func (e *emisor) campoDeClassFile(f jcampo) ([]byte, error) {
	esPublico, esEstatico, esFinal := f.esPublico, f.esEstatico, f.esFinal
	if e.c.esInterface {
		// en una interface los campos son implicitamente public static final (JVMS 4.5); medido en
		// la sonda del parrafo 92: flags 0x0019 y ConstantValue
		esPublico, esEstatico, esFinal = true, true, true
	}
	acc := 0
	if esPublico {
		acc |= accPublic
	}
	if f.esPrivado {
		acc |= accPrivate
	}
	if f.esProtegido {
		acc |= accProtected
	}
	if esEstatico {
		acc |= accStatic
	}
	if esFinal {
		acc |= accFinal
	}
	if f.esSintetico {
		acc |= accSynthetic
	}
	desc, err := e.ctx.descriptorDe(f.tipo)
	if err != nil {
		return nil, err
	}
	attrs := []byte{}
	conte := 0
	if e.c.esInterface && f.valor == nil {
		return nil, fmt.Errorf("un campo de interface sin inicializador no esta medido (Java lo exige final y con valor)")
	}
	if f.valor != nil {
		// medido con `javap -v` (parrafo 49): `static final int x = 6` se dobla y lleva ConstantValue sin
		// codigo; `final int fijo = 6` lleva ConstantValue *y* su `bipush 6; putfield` en el <init>; y uno
		// que no es final solo el codigo. Un `static final` con algo que no es un literal entero no esta
		// medido: se rechaza en vez de suponer donde lo pondria javac.
		lit, esLit := f.valor.(jlit)
		// un `static final char` se dobla igual que un `static final int`: `ConstantValue` con una entrada
		// `Integer` y sin `putstatic` en el `<clinit>` (medido en `Letras`, `ConstantValue: int 90`,
		// parrafo 87)
		//
		// y el resto de la familia, medido en `G` (§130): `static final long L = 6L` lleva
		// `ConstantValue: long 6l` (tag 5), `float` tag 4, `double` tag 6 y `String` tag 8, todos sin
		// `putstatic` en el `<clinit>`.
		indice, constante := 0, false
		if esLit {
			switch {
			case (lit.tipoJ == 'i' || lit.tipoJ == 'c') && (desc == "I" || desc == "C" || desc == "S" || desc == "B" || desc == "Z"):
				indice, constante = e.pool.entero(lit.entero), true
			case lit.tipoJ == 'j' && desc == "J":
				indice, constante = e.pool.largo8(lit.largo), true
			case lit.tipoJ == 'f' && desc == "F":
				indice, constante = e.pool.flotante4(lit.flotante), true
			case lit.tipoJ == 'd' && desc == "D":
				indice, constante = e.pool.doble8(lit.flotante), true
			case lit.tipoJ == 's' && desc == "Ljava/lang/String;":
				indice, constante = e.pool.cadena(lit.texto), true
			}
		}
		if esFinal && !constante {
			return nil, fmt.Errorf("el ConstantValue de un campo `final` cuyo inicializador no es un literal del tipo del campo no esta medido")
		}
		if esFinal {
			attrs = append(attrs, attrInfo(e.pool, "ConstantValue", u2be(uint16(indice)))...)
			conte = 1
		}
	}
	b := u2be(uint16(acc))
	b = append(b, u2be(uint16(e.pool.utf8(f.nombre)))...)
	b = append(b, u2be(uint16(e.pool.utf8(desc)))...)
	b = append(b, u2be(uint16(conte))...)
	return append(b, attrs...), nil
}

// inicializadoresDeInstancia: los campos con valor que javac baja al <init>, en orden de declaracion. Un
// `static final` con literal se dobla en el ConstantValue y no baja a ningun sitio; por eso se excluye.
func (c *jclas) inicializadoresDeInstancia() []jcampo {
	out := []jcampo{}
	for _, f := range c.campos {
		if f.valor == nil || f.esEstatico {
			continue
		}
		out = append(out, f)
	}
	return out
}

// inicializadoresEstaticos: los campos `static` con un valor que no sea la constante doblada, que javac
// escribe en un <clinit> que va DESPUES de los metodos declarados (medido: <init>, onCreate, contar, {}).
func (c *jclas) inicializadoresEstaticos() []jcampo {
	out := []jcampo{}
	for _, f := range c.campos {
		if f.valor == nil || !f.esEstatico || f.esFinal {
			continue
		}
		out = append(out, f)
	}
	return out
}

// preparaMetodo deja el emisor como nuevo: el pool y las anidadas mencionadas son de la clase, todo lo demas
// es del metodo.
func (e *emisor) preparaMetodo(desc string, estatico bool) {
	e.estatico = estatico
	e.desc = desc
	e.vuelta = vueltaDe(desc)
	e.locales = map[string]localVar{}
	e.siguiente = 0
	e.codigo = nil
	e.lineas = nil
	e.parches = nil
	e.etiquetas = nil
	e.siguienteEtiqueta = 0
	e.quiebres = nil
	e.continuar = nil
	e.excepciones = nil
	e.finalmente = nil
	e.prof = 0
	e.maxProf = 0
	e.maxLoc = 0
	e.truncado = false
}

// codeAttr: el atributo Code con su LineNumberTable y, si hay ramas, su StackMapTable, en el orden
// que javac los escribe. El marco se calcula sobre el cuerpo ya parcheado, porque sus posiciones
// dependen de los anchos definitivos de los saltos.
func (e *emisor) codeAttr() ([]byte, error) {
	e.encadenaGotos()
	marcos, alto, err := framesDeMetodo(e.codigo, e.desc, e.estatico, e.interno, e.pool, e.maxLoc, e.excepciones)
	if err != nil {
		return nil, err
	}
	if alto > e.maxProf {
		return nil, fmt.Errorf("la pila simulada llega a %d y el metodo declara %d: el max_stack no cubre el cuerpo", alto, e.maxProf)
	}
	code := u2be(uint16(e.maxProf))
	code = append(code, u2be(uint16(e.maxLoc))...)
	code = append(code, u4be(uint32(len(e.codigo)))...)
	code = append(code, e.codigo...)
	code = append(code, u2be(uint16(len(e.excepciones)))...)
	for _, ex := range e.excepciones {
		code = append(code, u2be(uint16(ex.inicio))...)
		code = append(code, u2be(uint16(ex.fin))...)
		code = append(code, u2be(uint16(ex.handler))...)
		if ex.tipo == "" {
			code = append(code, u2be(0)...)
		} else {
			// catch_type es una Class con el nombre INTERNO (`java/lang/X`), no el descriptor:
			// `javap` imprime el Utf8 tal cual y con `L…;` salía `Class "Ljava/lang/X;"` (§71)
			code = append(code, u2be(uint16(e.pool.clase(internoDeDescriptor(ex.tipo))))...)
		}
	}
	tabla := u2be(uint16(len(e.lineas)))
	for _, l := range e.lineas {
		tabla = append(tabla, u2be(uint16(l.desplazamiento))...)
		tabla = append(tabla, u2be(uint16(l.linea))...)
	}
	if marcos == nil {
		code = append(code, u2be(1)...)
		return append(code, attrInfo(e.pool, "LineNumberTable", tabla)...), nil
	}
	code = append(code, u2be(2)...)
	code = append(code, attrInfo(e.pool, "LineNumberTable", tabla)...)
	return append(code, attrInfo(e.pool, "StackMapTable", marcos)...), nil
}

// methodInfo: access_flags, nombre, descriptor y sus atributos.
func (e *emisor) methodInfo(nombre, desc string, acc int, attrs []byte, conte int) []byte {
	b := u2be(uint16(acc))
	b = append(b, u2be(uint16(e.pool.utf8(nombre)))...)
	b = append(b, u2be(uint16(e.pool.utf8(desc)))...)
	b = append(b, u2be(uint16(conte))...)
	return append(b, attrs...)
}

// delegacionDeConstructor: la primera sentencia de un constructor es `super(...)` o `this(...)`.
// Medido en el parrafo 91: en los dos casos javac no baja los inicializadores de campo en ESTE
// constructor; el que recibe la delegacion es el que los escribe.
func delegacionDeConstructor(cuerpo []jstmt) (jllamada, bool) {
	if len(cuerpo) == 0 {
		return jllamada{}, false
	}
	st, ok := cuerpo[0].(jstmtExpr)
	if !ok {
		return jllamada{}, false
	}
	l, ok := st.e.(jllamada)
	if !ok {
		return jllamada{}, false
	}
	if l.constructorPropio || (l.super && l.nombre == "<init>") {
		return l, true
	}
	return jllamada{}, false
}

// flagsDeMetodo: los modificadores del fuente a los access_flags del method_info. El abstracto y el
// public de una interface se suman aparte, porque son implicitos (parrafo 92).
func (e *emisor) flagsDeMetodo(m jmetodo) int {
	acc := 0
	if m.esPublico {
		acc |= accPublic
	}
	if m.esPrivado {
		acc |= accPrivate
	}
	if m.esProtegido {
		acc |= accProtected
	}
	if m.esEstatico {
		acc |= accStatic
	}
	if m.esFinal {
		acc |= accFinal
	}
	return acc
}

// excepcionesAttr: el atributo Exceptions de un `throws A, B`: cuenta y un indice de Class por nombre.
func (e *emisor) excepcionesAttr(lanza []string) ([]byte, error) {
	body := u2be(uint16(len(lanza)))
	for _, t := range lanza {
		interno, err := e.ctx.tipoAInterno(t)
		if err != nil {
			return nil, err
		}
		e.usar(interno)
		body = append(body, u2be(uint16(e.pool.clase(interno)))...)
	}
	return attrInfo(e.pool, "Exceptions", body), nil
}

func (e *emisor) metodoDeClassFile(m jmetodo) ([]byte, error) {
	desc, err := e.ctx.descDeMetodo(m)
	if err != nil {
		return nil, err
	}
	if m.abstracto {
		// un metodo sin Code: `public abstract` en una clase abstracta o en una interface. Medido
		// en el parrafo 92: flags 0x0401 y solo el atributo Exceptions cuando hay `throws`.
		acc := e.flagsDeMetodo(m)
		acc |= accAbstract
		if e.c.esInterface {
			acc |= accPublic
		}
		attrs := []byte{}
		conte := 0
		if len(m.lanza) > 0 {
			b, err := e.excepcionesAttr(m.lanza)
			if err != nil {
				return nil, err
			}
			attrs = b
			conte = 1
		}
		return e.methodInfo(m.nombre, desc, acc, attrs, conte), nil
	}
	// Un cuerpo vacio `{ }` no es abstracto: emite el `return` implicito con la linea de la llave.
	e.preparaMetodo(desc, m.esEstatico)
	if !m.esEstatico {
		e.reserva("", "L") // this en el hueco 0
	}
	for _, p := range m.params {
		d, err := e.ctx.descriptorDe(p.tipo)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", p.donde.linea, err)
		}
		e.reserva(p.nombre, d)
	}
	cuerpo := m.cuerpo
	if m.nombre == "<init>" {
		if l, delega := delegacionDeConstructor(m.cuerpo); delega {
			// la primera sentencia ya es la llamada al padre o al otro constructor. Con `this(...)`
			// los inicializadores de campo NO se bajan aquí; con `super(...)` sí, después de la
			// llamada (medido en `Ctor` del parrafo 91: line 2: 5 tras el super de line 8).
			if err := e.stm(m.cuerpo[0]); err != nil {
				return nil, err
			}
			cuerpo = m.cuerpo[1:]
			if !l.constructorPropio {
				for _, f := range e.c.inicializadoresDeInstancia() {
					if err := e.inicializador(f); err != nil {
						return nil, err
					}
				}
			}
		} else {
			// `super()` implicito: javac lo escribe antes de los inicializadores y le pone la linea
			// de la firma del constructor (medido en `Simple` del parrafo 91: line 3: 0)
			dueño, err := e.superDe()
			if err != nil {
				return nil, err
			}
			e.marcaLinea(m.linea)
			e.cargaDe(0, "L")
			e.invoca(opInvokespec, dueño, "<init>", "()V", 1)
			for _, f := range e.c.inicializadoresDeInstancia() {
				if err := e.inicializador(f); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, s := range cuerpo {
		if err := e.stm(s); err != nil {
			return nil, err
		}
	}
	if e.vuelta == "V" {
		// el return implicito, con la linea de la llave que cierra (parrafo 40: line 16: 11). javac lo
		// suelta cuando el ultimo camino ya sale por su cuenta: medido en `Fin4.vacioConThrowFinal`,
		// cuyo `Code` termina en el `athrow` del `finally` y no lleva return (parrafo 75)
		if !terminaEnRetorno(m.cuerpo) {
			e.marcaLinea(m.lineaFin)
			e.emite(retorno('V'))
		}
	} else if !terminaEnRetorno(m.cuerpo) {
		return nil, fmt.Errorf("linea %d: un metodo que devuelve %s y no termina en `return` no esta medido", m.linea, e.vuelta)
	}
	if e.prof != 0 {
		return nil, fmt.Errorf("linea %d: quedan %d valores en la pila al cerrar el metodo, y eso no lo produce javac", m.linea, e.prof)
	}
	if e.truncado {
		return nil, fmt.Errorf("linea %d: el pool de %s pasa de 255 entradas y `ldc_w` no esta medido en este emisor", m.linea, e.interno)
	}
	acc := e.flagsDeMetodo(m)
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, fmt.Errorf("linea %d: %w", m.linea, err)
	}
	attrs := attrInfo(e.pool, "Code", codigo)
	conte := 1
	if len(m.lanza) > 0 {
		b, err := e.excepcionesAttr(m.lanza)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, b...)
		conte++
	}
	return e.methodInfo(m.nombre, desc, acc, attrs, conte), nil
}

// initSintetico: la clase no escribe constructor y javac lo pone: aload_0, invokespecial al <init> del padre
// y return, con una sola fila en la LineNumberTable, apuntando a la linea de la declaracion de clase.
func (e *emisor) initSintetico() ([]byte, error) {
	dueño, err := e.superDe()
	if err != nil {
		return nil, err
	}
	e.preparaMetodo("()V", false)
	e.reserva("", "L")
	e.marcaLinea(e.c.linea)
	e.cargaDe(0, "L")
	e.invoca(opInvokespec, dueño, "<init>", "()V", 1)
	for _, f := range e.c.inicializadoresDeInstancia() {
		if err := e.inicializador(f); err != nil {
			return nil, err
		}
	}
	e.emite(retorno('V'))
	acc := 0
	if e.c.esPublica {
		acc = accPublic
	}
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	return e.methodInfo("<init>", "()V", acc, attrInfo(e.pool, "Code", codigo), 1), nil
}

// clinitSintetico: el `static {}` que javac escribe al final, despues de los metodos declarados, solo
// cuando hay un campo estatico con un valor que no sea la constante doblada. Lleva ACC_STATIC, `()V`,
// `locals=0` (no hay `this`) y una fila por campo en su LineNumberTable.
func (e *emisor) clinitSintetico() ([]byte, error) {
	e.preparaMetodo("()V", true)
	for _, f := range e.c.inicializadoresEstaticos() {
		if err := e.inicializador(f); err != nil {
			return nil, err
		}
	}
	e.emite(retorno('V'))
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	return e.methodInfo("<clinit>", "()V", accStatic, attrInfo(e.pool, "Code", codigo), 1), nil
}

// accesorDeClassFile: el `access$NNN` con el que una anonima toca un miembro privado del exterior. La
// forma se midio en el parrafo 72 sobre `javap -c -p`: leer un campo es `aload_0; getfield; ireturn`, leer
// uno estatico `getstatic; ireturn`; escribir usa `dup_x1` (instancia) o `dup` (estatico) y devuelve el
// valor, de ahi el `pop` del llamador; un metodo privado se invoca con `invokespecial` y uno estatico con
// `invokestatic`. Los flags son ACC_STATIC|ACC_SYNTHETIC (0x1008) y la unica fila de la LineNumberTable
// apunta a la linea de la declaracion de la clase.
func (e *emisor) accesorDeClassFile(a accesorSintetico) ([]byte, error) {
	e.preparaMetodo(a.desc, true)
	// max_locals tiene que caber los argumentos aunque el cuerpo no reserve ninguno: la JVM rechaza el
	// fichero con `Arguments can't fit into locals` si el descriptor pide mas huecos (destapado por
	// `java -Xverify:all` sobre esta sonda antes de corregirlo). El accessor es siempre estatico, asi que
	// el descriptor dice todos los huecos; `a.estatico` habla del miembro, no del metodo sintetico.
	_, _, args := splitDescriptor(a.desc)
	// los HUECOS que piden los argumentos, no el número de argumentos: un `long` o un `double` ocupa
	// dos, y el traductor reparte los locales con `max_locals` (destapado en el aparato con
	// `D.access$002`, párrafo 83: `max_locals=2` para `(LD;J)J` dejaba a `aload_0` en v-1)
	e.maxLoc = 0
	for _, p := range args {
		if esAncho(p) {
			e.maxLoc += 2
		} else {
			e.maxLoc++
		}
	}
	e.marcaLinea(e.c.linea)
	// el `return` se lleva lo que haya en la pila: dos palabras si el miembro es ancho (J/D), una si no
	pierde := 1
	if esAncho(a.miembroDesc) {
		pierde = 2
	}
	switch {
	case a.campo && a.estatico && !a.escritura:
		idx := e.pool.campo(a.duenio, a.miembro, a.miembroDesc)
		e.getstaticDe(idx, a.miembroDesc)
		e.emite(retorno(a.miembroDesc[0]))
		e.baja(pierde)
	case a.campo && a.estatico:
		// `static long` privado: `lload_0; dup2; putstatic; lreturn` (medido en `E.access$002`,
		// párrafo 83); el `dup` de una palabra era el del parrafo 72
		idx := e.pool.campo(a.duenio, a.miembro, a.miembroDesc)
		e.cargaDe(0, a.miembroDesc)
		if esAncho(a.miembroDesc) {
			e.emite(opDup2)
			e.subeDos()
			e.putstaticDe(idx, a.miembroDesc)
		} else {
			e.emite(opDup)
			e.sube()
			e.putstatic(idx)
		}
		e.emite(retorno(a.miembroDesc[0]))
		e.baja(pierde)
	case a.campo && !a.escritura:
		idx := e.pool.campo(a.duenio, a.miembro, a.miembroDesc)
		e.cargaDe(0, "L")
		e.getfieldDe(idx, a.miembroDesc)
		e.emite(retorno(a.miembroDesc[0]))
		e.baja(pierde)
	case a.campo:
		// y el de instancia: `aload_0; lload_1; dup2_x1; putfield; lreturn` (`D.access$002`)
		idx := e.pool.campo(a.duenio, a.miembro, a.miembroDesc)
		e.cargaDe(0, "L")
		e.cargaDe(1, a.miembroDesc)
		if esAncho(a.miembroDesc) {
			e.emite(opDup2X1)
			e.subeDos()
			e.putfieldDe(idx, a.miembroDesc)
		} else {
			e.emite(opDupX1)
			e.sube()
			e.putfield(idx)
		}
		e.emite(retorno(a.miembroDesc[0]))
		e.baja(pierde)
	default:
		_, vuelta, params := splitDescriptor(a.miembroDesc)
		base := 0
		if !a.estatico {
			e.cargaDe(0, "L")
			base = 1
		}
		for i, p := range params {
			e.cargaDe(base+i, p)
		}
		opcode := byte(opInvokestat)
		if !a.estatico {
			opcode = opInvokespec
		}
		e.invoca(opcode, a.duenio, a.miembro, a.miembroDesc, base+len(params))
		if vuelta == "V" {
			e.emite(retorno('V'))
		} else {
			e.emite(retorno(vuelta[0]))
			e.baja(1)
		}
	}
	if e.prof != 0 {
		return nil, fmt.Errorf("el access$%s deja %d valores en la pila al cerrar", a.nombre, e.prof)
	}
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	return e.methodInfo(a.nombre, a.desc, accStatic|accSynthetic, attrInfo(e.pool, "Code", codigo), 1), nil
}

// terminaEnRetorno: la ultima sentencia del cuerpo es un return. Con `if` y bucles todavia no se puede
// saber si un metodo cae al final, y por eso el resto se rechaza con nombre.
func terminaEnRetorno(cuerpo []jstmt) bool {
	if len(cuerpo) == 0 {
		return false
	}
	return terminaEnSalida(cuerpo[len(cuerpo)-1])
}

// terminaEnSalida: la sentencia no deja continuar al final del bloque. Un `if` solo termina si su
// cuerpo y su `else` terminan: medido en el parrafo 52 con `if (a == 0) { return 6; } else { return 7; }`,
// del que javac no escribe ninguna salida al final.
func terminaEnSalida(s jstmt) bool {
	switch v := s.(type) {
	case jretorno:
		return true
	case jbloque:
		return terminaEnRetorno(v.sentencias)
	case jif:
		if v.siNo == nil {
			return false
		}
		return terminaEnSalida(v.entonces) && terminaEnSalida(v.siNo)
	case jswitch:
		// solo si TODOS los brazos y el defecto salen; con un `break` se sigue después del switch
		if v.defecto == nil || !terminaEnRetorno(v.defecto) {
			return false
		}
		for _, c := range v.casos {
			if !terminaEnRetorno(c.cuerpo) {
				return false
			}
		}
		return true
	case jlanza:
		return true
	case jtry:
		// con `finally`, el ultimo camino que se ejecuta es la copia del bloque: si ella sale (un
		// `throw`), el constructo sale y javac no escribe el `return` implicito —medido en
		// `Fin4.vacioConThrowFinal`—. Si no, el constructo cae solo si caen el cuerpo y todos los
		// `catch`
		if v.finalmente != nil && terminaEnSalida(v.finalmente) {
			return true
		}
		if !terminaEnSalida(v.cuerpo) {
			return false
		}
		for _, c := range v.atrapas {
			if !terminaEnSalida(c.atrapa) {
				return false
			}
		}
		return true
	}
	return false
}
