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
	"sort"
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
	opDup        = 0x59
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
	opIFEQ      = 0x99
	opIFNE      = 0x9a
	opIFLT      = 0x9b
	opIFGE      = 0x9c
	opIFGT      = 0x9d
	opIFLE      = 0x9e
	opIFICMPEQ  = 0x9f
	opIFICMPNE  = 0xa0
	opIFICMPLT  = 0xa1
	opIFICMPGE  = 0xa2
	opIFICMPGT  = 0xa3
	opIFICMPLE  = 0xa4
	opGOTO      = 0xa7
	opIFNULL    = 0xc6
	opIFNONNULL = 0xc7
	// aritmetica entera, leida de `javap -c` sobre `a*b`, `a/b`, `a%b`, `a+b`, `a-b` (parrafo 49)
	opIadd = 0x60
	opIsub = 0x64
	opImul = 0x68
	opIdiv = 0x6c
	opIrem = 0x70
)

// descPrimitivo: el descriptor de cada tipo escrito en el fuente. `long`, `float` y `double` se rechazan
// con nombre en descriptorDe: el primero ocupa dos huecos de variable y los otros dos piden arithmetic
// widening que nadie midio aqui.
var descPrimitivo = map[string]string{
	"int": "I", "boolean": "Z", "short": "S", "byte": "B", "char": "C",
	"long": "J", "float": "F", "double": "D", "void": "V",
}

var deDosHuecosOPuntoFlotante = map[byte]bool{'J': true, 'F': true, 'D': true}

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
	return 0x2a, 0x19
}

func guarda(desc byte) (breve byte, conIndice byte) {
	if desc == 'I' || desc == 'B' || desc == 'C' || desc == 'S' || desc == 'Z' {
		return 0x3b, 0x36
	}
	return 0x4b, 0x3a
}

func retorno(desc byte) byte {
	switch desc {
	case 'V':
		return 0xb1
	case 'I', 'B', 'C', 'S', 'Z':
		return 0xac
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
	if d, ok := descPrimitivo[tipo]; ok {
		if deDosHuecosOPuntoFlotante[d[0]] {
			return "", fmt.Errorf("el tipo %s ocupa dos huecos de variable o es de punto flotante: no esta medido en este emisor", tipo)
		}
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

// buscarMetodo recorre la clase y sus padres por nombre y aridad. Devuelve la descripcion, los accesos del
// duenio (para el estatico y el interface) y el duenio, que es lo unico que se lee del jar.
func (ctx *contextoJava) buscarMetodo(interno, nombre string, argc int) (desc string, acceso int, err error) {
	actual := interno
	for visits := 0; actual != "" && visits < 32; visits++ {
		propia, esPropia := ctx.clasePropia(actual)
		if !esPropia {
			d, a, err := ctx.busc.enLaCadena(actual, nombre, argc)
			if err != nil {
				return "", 0, fmt.Errorf("%s: %w", interno, err)
			}
			return d, a, nil
		}
		for _, m := range propia.metodos {
			if m.nombre == nombre && len(m.params) == argc {
				d, err := ctx.descDeMetodo(m)
				if err != nil {
					return "", 0, err
				}
				acc := 0
				if m.esPublico {
					acc |= accPublic
				}
				if m.esEstatico {
					acc |= accStatic
				}
				if m.esFinal {
					acc |= accFinal
				}
				return d, acc, nil
			}
		}
		if actual == "java/lang/Object" {
			break
		}
		if actual, err = ctx.padreDe(propia); err != nil {
			return "", 0, err
		}
	}
	return "", 0, fmt.Errorf("el metodo %s.%s con %d argumentos no esta en %s ni en sus padres", interno, nombre, argc, interno)
}

// enLaCadena busca un metodo en una clase del jar y sus padres.
func (b *buscadorDeClases) enLaCadena(interno, nombre string, argc int) (desc string, accesoDuenio int, err error) {
	actual := interno
	for visits := 0; actual != "" && visits < 32; visits++ {
		c, err := b.claseDelJar(actual)
		if err != nil {
			return "", 0, err
		}
		for _, m := range c.Methods {
			if m.Name == nombre && cuentaArgumentosJava(m.Desc) == argc {
				return m.Desc, m.Access, nil
			}
		}
		if c.Super == "" {
			break
		}
		actual = internoDeDescriptor(c.Super)
	}
	return "", 0, fmt.Errorf("%s no declara %s con %d argumentos ni lo hereda", interno, nombre, argc)
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
				if f.esEstatico {
					acc |= accStatic
				}
				if f.esFinal {
					acc |= accFinal
				}
				cr := campoResuelto{desc: d, acceso: acc, dueño: dueño, constante: f.esFinal}
				if cr.constante {
					lit, esLit := f.valor.(jlit)
					if !esLit {
						return campoResuelto{}, fmt.Errorf("el campo %s.%s dice ser final pero su inicializador no es un literal", actual, nombre)
					}
					if lit.tipoJ != 'i' {
						return campoResuelto{}, fmt.Errorf("doblar una constante %s.%s que no es un entero no esta medida", actual, nombre)
					}
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
	ctx := &contextoJava{busc: busc, propias: map[string]*jclas{}}
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

// emmitirArbol escribe la clase y, despues, sus anidadas: cada una es su propio fichero.
func (ctx *contextoJava) emmitirArbol(c *jclas) ([]claseEmitida, error) {
	datos, err := ctx.emmitirClase(c)
	if err != nil {
		return nil, fmt.Errorf("linea %d: %w", c.linea, err)
	}
	salida := []claseEmitida{{interno: internoDe(c.paquete, c.nombre), datos: datos}}
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
	return salida, nil
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
	etiquetas         map[int]int
	siguienteEtiqueta int
}

type parcheSalto struct {
	en       int
	etiqueta int
	op       byte
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
	e.siguiente++
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
	e.sube()
}

func (e *emisor) guardaDe(indice int, desc string) {
	breve, conIndice := guarda(desc[0])
	if indice <= 3 {
		e.emite(breve + byte(indice))
	} else {
		e.emite(conIndice, byte(indice))
	}
	e.baja(1)
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

// putfield se come el receptor y el valor; putstatic solo el valor. Medido en `javap -c`: el `<init>` de
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
		e.putstatic(idx)
	} else {
		e.putfield(idx)
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
		e.emite(byte(enPila), 0)
	}
	e.baja(enPila)
	if v := vueltaDe(desc); v != "" && v != "V" {
		e.sube()
	}
}

// ----- expresiones -----

// lineaDeExpr: donde estaba la expresion, para la LineNumberTable y para los mensajes.
func lineaDeExpr(x jexpr) int {
	switch v := x.(type) {
	case jlit:
		return v.donde.linea
	case jthis:
		return v.donde.linea
	case jacceso:
		return v.donde.linea
	case jllamada:
		return v.donde.linea
	case jcast:
		return v.donde.linea
	case jnuevo:
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
		case 'b':
			if v.booleano {
				e.emite(0x04)
			} else {
				e.emite(0x03)
			}
			e.sube()
			return "Z", nil
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
	case jcast:
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
	case joperador:
		// aritmetica entera, leida de `javap -c` sobre `a*b`, `a/b`, `a%b`, `a+b`, `a-b` (parrafo 49):
		// dos palabras dentro, una fuera. Concatenar cadenas con `+` es StringBuilder y es otro incremento.
		codes := map[string]byte{"+": opIadd, "-": opIsub, "*": opImul, "/": opIdiv, "%": opIrem}
		code, esAritmetica := codes[v.op]
		if !esAritmetica {
			return "", fmt.Errorf("linea %d: el operador %q no esta medido en este emisor", v.donde.linea, v.op)
		}
		izq, err := e.exp(v.izq)
		if err != nil {
			return "", err
		}
		der, err := e.exp(v.der)
		if err != nil {
			return "", err
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
		e.getfield(idx)
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
		e.emite(opGetstatic, byte(idx>>8), byte(idx))
		e.sube()
		return cr.desc, nil
	}
	return "", fmt.Errorf("linea %d: no se resuelve %s.%s", linea, interno, strings.Join(segs, "."))
}

// campoDeLaClaseActual: `menu` suelto cuando fuera un campo de esta clase.
func (e *emisor) campoDeLaClaseActual(nombre string, linea int) (string, bool) {
	cr, err := e.ctx.buscarCampo(e.interno, nombre)
	if err != nil {
		return "", false
	}
	if cr.constante {
		e.cargaEntero(cr.valor)
		return cr.desc, true
	}
	if cr.acceso&accStatic != 0 {
		idx := e.pool.campo(e.interno, nombre, cr.desc)
		e.emite(opGetstatic, byte(idx>>8), byte(idx))
		e.sube()
		return cr.desc, true
	}
	if e.estatico {
		return "", false
	}
	e.cargaDe(0, "L")
	e.getfield(e.pool.campo(e.interno, nombre, cr.desc))
	return cr.desc, true
}

// llamada: los cuatro casos del subconjunto, con el Methodref en el sitio que pone javac (parrafo 40).
func (e *emisor) llamada(l jllamada) (string, error) {
	argc := len(l.args)
	if l.super {
		if e.estatico {
			return "", fmt.Errorf("linea %d: `super` en un metodo estatico no es Java", l.donde.linea)
		}
		dueño, err := e.superDe()
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		desc, _, err := e.ctx.buscarMetodo(dueño, l.nombre, argc)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		e.cargaDe(0, "L")
		if err := e.argumentos(l.args); err != nil {
			return "", err
		}
		e.invoca(opInvokespec, dueño, l.nombre, desc, argc+1)
		return vueltaDe(desc), nil
	}
	if l.base == nil {
		if e.estatico {
			return "", fmt.Errorf("linea %d: llamar %s sin receptor desde un metodo estatico no esta medido", l.donde.linea, l.nombre)
		}
		// receptor implicito: el Methodref va contra la clase actual, medido en el parrafo 40
		desc, _, err := e.ctx.buscarMetodo(e.interno, l.nombre, argc)
		if err != nil {
			return "", fmt.Errorf("linea %d: no existe %s con %d argumentos en %s ni en sus padres", l.donde.linea, l.nombre, argc, e.interno)
		}
		e.cargaDe(0, "L")
		if err := e.argumentos(l.args); err != nil {
			return "", err
		}
		e.invoca(opInvokevirt, e.interno, l.nombre, desc, argc+1)
		return vueltaDe(desc), nil
	}
	if acc, esAcceso := l.base.(jacceso); esAcceso && acc.base == nil && e.locales[acc.nombre].desc == "" {
		if interno, err := e.ctx.resolverSimple(acc.nombre); err == nil {
			desc, accesoDuenio, err := e.ctx.buscarMetodo(interno, l.nombre, argc)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
			}
			if accesoDuenio&accStatic == 0 {
				return "", fmt.Errorf("linea %d: %s.%s no es estatico y no se puede llamar sobre el tipo", l.donde.linea, interno, l.nombre)
			}
			if err := e.argumentos(l.args); err != nil {
				return "", err
			}
			e.invoca(opInvokestat, interno, l.nombre, desc, argc)
			return vueltaDe(desc), nil
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
	desc, _, err := e.ctx.buscarMetodo(receptor, l.nombre, argc)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	if err := e.argumentos(l.args); err != nil {
		return "", err
	}
	accesosReceptor, err := e.ctx.accesosDe(receptor)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	var opcode byte = opInvokevirt
	if accesosReceptor&accInterface != 0 {
		opcode = opInvointf
	}
	e.invoca(opcode, receptor, l.nombre, desc, argc+1)
	return vueltaDe(desc), nil
}

func (e *emisor) argumentos(args []jexpr) error {
	for _, arg := range args {
		if _, err := e.exp(arg); err != nil {
			return err
		}
	}
	return nil
}

func (e *emisor) superDe() (string, error) { return e.ctx.padreDe(e.c) }

// nuevo: `new Tipo(args)`. Una clase anonima baja el exterior a un campo this$0 y ese camino se comprueba
// aparte, con su propio parrafo de medicion.
func (e *emisor) nuevo(n jnuevo) (string, error) {
	if n.cuerpo != nil {
		// la anonima se nombra Exterior$N como la nombra javac y su constructor recibe primero la captura
		// del exterior: medido en el parrafo 42 (`new #28; dup; aload_0; invokespecial <init>
		// :(Lcom/hola/apkcstudio/MainActivity;)V`)
		if e.estatico {
			return "", fmt.Errorf("linea %d: una anonima creada en un metodo estatico no tiene capturado un this y ese caso no esta medido", n.donde.linea)
		}
		interno := internoDe(n.cuerpo.paquete, n.cuerpo.nombre)
		e.usar(interno)
		idx := e.pool.clase(interno)
		e.emite(opNew, byte(idx>>8), byte(idx))
		e.sube()
		e.emite(opDup)
		e.sube()
		e.cargaDe(0, "L")
		if err := e.argumentos(n.args); err != nil {
			return "", err
		}
		sb := strings.Builder{}
		sb.WriteString("(L" + e.interno + ";")
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
		e.invoca(opInvokespec, interno, "<init>", sb.String(), len(n.args)+2)
		return "L" + interno + ";", nil
	}
	interno, err := e.ctx.tipoAInterno(n.tipo)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", n.donde.linea, err)
	}
	e.usar(interno)
	idx := e.pool.clase(interno)
	e.emite(opNew, byte(idx>>8), byte(idx))
	e.sube()
	e.emite(opDup)
	e.sube()
	if err := e.argumentos(n.args); err != nil {
		return "", err
	}
	sb := strings.Builder{}
	sb.WriteString("(")
	for _, arg := range n.args {
		d, err := e.tipoEstimado(arg)
		if err != nil {
			return "", err
		}
		sb.WriteString(d)
	}
	sb.WriteString(")V")
	// el `dup` deja dos referencias: el constructor se come una con sus argumentos y la otra es el valor de
	// la expresion, que es el hueco que sube de la pila al terminar
	e.invoca(opInvokespec, interno, "<init>", sb.String(), len(n.args)+1)
	return "L" + interno + ";", nil
}

// tipoEstimado: el descriptor de un argumento que va en un constructor, sin volver a emitirlo. En el
// subconjunto medido los argumentos de un `new` son literales o variables, y ambos se saben aqui.
func (e *emisor) tipoEstimado(x jexpr) (string, error) {
	switch v := x.(type) {
	case jlit:
		switch v.tipoJ {
		case 'i':
			return "I", nil
		case 'b':
			return "Z", nil
		case 's':
			return "Ljava/lang/String;", nil
		case 'n':
			return "Ljava/lang/Object;", nil
		}
		return "", fmt.Errorf("linea %d: un literal de clase %q no esta medido", v.donde.linea, string(v.tipoJ))
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
	}
	return "", fmt.Errorf("linea %d: el tipo de un argumento de %T no esta medido", lineaDeExpr(x), x)
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
	if !esEnteroDesc(dizq) || !esEnteroDesc(dder) {
		return 0, 0, fmt.Errorf("linea %d: `%s` entre %s y %s: solo se midieron enteros y la referencia contra null", linea, op.op, dizq, dder)
	}
	return pares.dos, 2, nil
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

func (e *emisor) stm(s jstmt) error {
	switch v := s.(type) {
	case jstmtExpr:
		e.marcaLinea(lineaDeExpr(v.e))
		d, err := e.exp(v.e)
		if err != nil {
			return err
		}
		if d != "V" && d != "" {
			e.emite(opPop)
			e.baja(1)
		}
		return nil
	case jretorno:
		e.marcaLinea(v.donde.linea)
		if v.e == nil {
			if e.vuelta != "V" {
				return fmt.Errorf("linea %d: un `return` suelto en un metodo que devuelve %s", v.donde.linea, e.vuelta)
			}
			e.emite(retorno('V'))
			return nil
		}
		d, err := e.exp(v.e)
		if err != nil {
			return err
		}
		if e.vuelta == "V" {
			return fmt.Errorf("linea %d: se devuelve algo desde un metodo void", v.donde.linea)
		}
		if len(d) > 0 && len(e.vuelta) > 0 && d[0] != e.vuelta[0] && !(d == "Ljava/lang/String;" && e.vuelta[0] == 'L') {
			return fmt.Errorf("linea %d: el metodo devuelve %s y se devuelve un %s", v.donde.linea, e.vuelta, d)
		}
		e.emite(retorno(d[0]))
		e.baja(1)
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
		dv, err := e.exp(v.valor)
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
		acc, esAcceso := v.destino.(jacceso)
		segs, base := aplanar(acc)
		if !esAcceso || base != nil || len(segs) != 1 {
			return fmt.Errorf("linea %d: una asignacion con mas de un nombre en el destino no esta medida", v.donde.linea)
		}
		if lv, ok := e.locales[segs[0]]; ok {
			dv, err := e.exp(v.valor)
			if err != nil {
				return err
			}
			if dv[0] != lv.desc[0] {
				return fmt.Errorf("linea %d: %s es %s y se le asigna un %s", v.donde.linea, acc.nombre, lv.desc, dv)
			}
			e.guardaDe(lv.indice, lv.desc)
			return nil
		}
		// asignar a un campo de la clase actual: `pulsos = pulsos + 1`, medido como `aload_0` (el
		// receptor, ANTES del valor), el valor, y `putfield`; en un campo estatico no hay receptor
		cr, err := e.ctx.buscarCampo(e.interno, segs[0])
		if err != nil {
			return fmt.Errorf("linea %d: asignar a %s, que no es ni una variable local ni un campo de esta clase, no esta medido",
				v.donde.linea, segs[0])
		}
		if cr.constante {
			return fmt.Errorf("linea %d: %s es una constante final y javac no la deja reasignar", v.donde.linea, segs[0])
		}
		esEstatico := cr.acceso&accStatic != 0
		if esEstatico && e.c.anonima {
			return fmt.Errorf("linea %d: asignar al estatico %s desde una anonima no esta medido", v.donde.linea, segs[0])
		}
		if !esEstatico {
			if e.estatico {
				return fmt.Errorf("linea %d: asignar a %s sin `this`, en un metodo estatico, no es Java", v.donde.linea, segs[0])
			}
			e.cargaDe(0, "L")
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
		idx := e.pool.campo(e.interno, segs[0], cr.desc)
		if esEstatico {
			e.putstatic(idx)
		} else {
			e.putfield(idx)
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
	if len(c.implementa) > 0 && !c.anonima {
		return nil, fmt.Errorf("linea %d: una clase escrita a mano que implementa interfaces no esta medida: faltan sus metodos abstractos y sus flags", c.linea)
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
	if c.esPublica {
		access |= accPublic
	}
	if c.final {
		access |= accFinal
	}
	if c.abstracta {
		access |= accAbstract
	}

	campos := []byte{}
	nCampos := 0
	if c.anonima {
		// javac abre la anonima con su campo de captura, antes de lo que el fuente escriba (parrafo 42:
		// `final com.hola.apkcstudio.MainActivity this$0;` con flags 0x1010, ACC_FINAL y ACC_SYNTHETIC)
		campos = append(campos, e.campoCaptura()...)
		nCampos++
	}
	for _, f := range c.campos {
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
	} else if !constructorEscrito {
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
	externo := "L" + e.encierreInterno() + ";"
	desc := "(" + externo + ")V"
	e.preparaMetodo(desc, false)
	e.reserva("", "L")
	e.reserva("", externo)
	e.marcaLinea(e.c.linea)
	e.cargaDe(0, "L")
	e.cargaDe(1, externo)
	idx := e.pool.campo(e.interno, "this$0", externo)
	e.emite(opPutfield, byte(idx>>8), byte(idx))
	e.baja(2)
	e.cargaDe(0, "L")
	e.invoca(opInvokespec, "java/lang/Object", "<init>", "()V", 1)
	e.emite(retorno('V'))
	attrs := attrInfo(e.pool, "Code", e.codeAttr())
	// MethodParameters: conteo u1, y por cada uno nombre u2 y flags u2. El de la captura no tiene nombre.
	mp := []byte{1}
	mp = append(mp, u2be(0)...)
	mp = append(mp, u2be(uint16(accFinal|accMandated))...)
	attrs = append(attrs, attrInfo(e.pool, "MethodParameters", mp)...)
	return e.methodInfo("<init>", desc, 0, attrs, 2), nil
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
	desc, err := e.ctx.descriptorDe(f.tipo)
	if err != nil {
		return nil, err
	}
	attrs := []byte{}
	conte := 0
	if f.valor != nil {
		// medido con `javap -v` (parrafo 49): `static final int x = 6` se dobla y lleva ConstantValue sin
		// codigo; `final int fijo = 6` lleva ConstantValue *y* su `bipush 6; putfield` en el <init>; y uno
		// que no es final solo el codigo. Un `static final` con algo que no es un literal entero no esta
		// medido: se rechaza en vez de suponer donde lo pondria javac.
		lit, esLit := f.valor.(jlit)
		constante := esLit && lit.tipoJ == 'i'
		if f.esFinal && !constante {
			return nil, fmt.Errorf("el ConstantValue de un campo `final` cuyo inicializador no es un entero literal no esta medido")
		}
		if f.esFinal {
			attrs = append(attrs, attrInfo(e.pool, "ConstantValue", u2be(uint16(e.pool.entero(lit.entero))))...)
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
	e.vuelta = vueltaDe(desc)
	e.locales = map[string]localVar{}
	e.siguiente = 0
	e.codigo = nil
	e.lineas = nil
	e.parches = nil
	e.etiquetas = nil
	e.siguienteEtiqueta = 0
	e.prof = 0
	e.maxProf = 0
	e.maxLoc = 0
	e.truncado = false
}

// codeAttr: el atributo Code con su LineNumberTable, en el orden que javac los escribe.
func (e *emisor) codeAttr() []byte {
	e.encadenaGotos()
	code := u2be(uint16(e.maxProf))
	code = append(code, u2be(uint16(e.maxLoc))...)
	code = append(code, u4be(uint32(len(e.codigo)))...)
	code = append(code, e.codigo...)
	code = append(code, u2be(0)...) // exception_table_length: el subconjunto no lanza ni atrapa
	tabla := u2be(uint16(len(e.lineas)))
	for _, l := range e.lineas {
		tabla = append(tabla, u2be(uint16(l.desplazamiento))...)
		tabla = append(tabla, u2be(uint16(l.linea))...)
	}
	code = append(code, u2be(1)...)
	return append(code, attrInfo(e.pool, "LineNumberTable", tabla)...)
}

// methodInfo: access_flags, nombre, descriptor y sus atributos.
func (e *emisor) methodInfo(nombre, desc string, acc int, attrs []byte, conte int) []byte {
	b := u2be(uint16(acc))
	b = append(b, u2be(uint16(e.pool.utf8(nombre)))...)
	b = append(b, u2be(uint16(e.pool.utf8(desc)))...)
	b = append(b, u2be(uint16(conte))...)
	return append(b, attrs...)
}

func (e *emisor) metodoDeClassFile(m jmetodo) ([]byte, error) {
	if m.nombre == "<init>" {
		return nil, fmt.Errorf("un constructor escrito a mano no esta medido: javac baja ahi los inicializadores de los campos")
	}
	if len(m.cuerpo) == 0 {
		return nil, fmt.Errorf("un metodo sin cuerpo (%s) pide ACC_ABSTRACT y ese camino no esta medido", m.nombre)
	}
	desc, err := e.ctx.descDeMetodo(m)
	if err != nil {
		return nil, err
	}
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
	for _, s := range m.cuerpo {
		if err := e.stm(s); err != nil {
			return nil, err
		}
	}
	if e.vuelta == "V" {
		// el return implicito, con la linea de la llave que cierra (parrafo 40: line 16: 11)
		e.marcaLinea(m.lineaFin)
		e.emite(retorno('V'))
	} else if !terminaEnRetorno(m.cuerpo) {
		return nil, fmt.Errorf("linea %d: un metodo que devuelve %s y no termina en `return` no esta medido", m.linea, e.vuelta)
	}
	if e.prof != 0 {
		return nil, fmt.Errorf("linea %d: quedan %d valores en la pila al cerrar el metodo, y eso no lo produce javac", m.linea, e.prof)
	}
	if e.truncado {
		return nil, fmt.Errorf("linea %d: el pool de %s pasa de 255 entradas y `ldc_w` no esta medido en este emisor", m.linea, e.interno)
	}
	acc := 0
	if m.esPublico {
		acc |= accPublic
	}
	if m.esEstatico {
		acc |= accStatic
	}
	if m.esFinal {
		acc |= accFinal
	}
	return e.methodInfo(m.nombre, desc, acc, attrInfo(e.pool, "Code", e.codeAttr()), 1), nil
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
	return e.methodInfo("<init>", "()V", acc, attrInfo(e.pool, "Code", e.codeAttr()), 1), nil
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
	return e.methodInfo("<clinit>", "()V", accStatic, attrInfo(e.pool, "Code", e.codeAttr()), 1), nil
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
	}
	return false
}
