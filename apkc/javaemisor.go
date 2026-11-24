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
	"hash/crc32"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// versionDeClassEmitida: Java 8, dicho arriba.
const versionDeClassEmitida = 52

// accEnum: el flag ACC_ENUM de JVMS 4.1, medido con `javap -v` sobre el enum de javac 17 (0x4031).
const accEnum = 0x4000

// Los opcodes que este emisor escribe, con el numero del JVMS.
const (
	opAconstNull = 0x01
	opLdc        = 0x12
	opLdcW       = 0x13
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
	// desplazamientos y operadores de bits, medidos en el párrafo 157 con `javap -c` de `Asig`
	opIshl  = 0x78
	opIshr  = 0x7a
	opIushr = 0x7c
	opIand  = 0x7e
	opIor   = 0x80
	opIxor  = 0x82
	// las negaciones unarias de la aritmética: `ineg` 0x74, `lneg` 0x75, `fneg` 0x76 y `dneg` 0x77,
	// un byte y ningún operando. Medidas con javac 17 sobre `-total`, `-(a+b)`, `-x` en los cuatro
	// anchos y `-f()`; el dexer las baja al 12x de DEX (`neg-int` 0x7b, `neg-long` 0x7d,
	// `neg-float` 0x7f y `neg-double` 0x80), medido con `d8` y `dexdump`
	opIneg = 0x74
	opLneg = 0x75
	opFneg = 0x76
	opDneg = 0x77
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

// metodoTipo: la entrada MethodType (tag 16) del JVMS 4.4.9: un indice a la utf8 del descriptor. Es
// lo que llevan los argumentos del bootstrap de una lambda.
func (p *poolBuilder) metodoTipo(desc string) int {
	u := p.utf8(desc)
	b := []byte{16, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(u))
	return p.agregar("mt\x00"+desc, b)
}

// metodoManejador: la entrada MethodHandle (tag 15) del JVMS 4.4.8: el `ref_kind` (1..9) y el indice
// de la referencia que apunta (un Methodref para los sitios de lambda). `REF_invokeStatic` es 6 y
// `REF_invokeSpecial` 7.
func (p *poolBuilder) metodoManejador(refKind byte, ref int) int {
	b := []byte{15, refKind, 0, 0}
	binary.BigEndian.PutUint16(b[2:], uint16(ref))
	return p.agregar(fmt.Sprintf("mh\x00%d|%d", refKind, ref), b)
}

// invocaDinamico: el sitio `invokedynamic` (tag 18) del JVMS 4.4.10: el indice del bootstrap y el
// nombre y tipo de la fabrica. El opcode que lo usa es `0xba` con dos bytes reservados a cero.
func (p *poolBuilder) invocaDinamico(bootstrap int, nombre, desc string) int {
	nt := p.nombreYTipo(nombre, desc)
	b := []byte{18, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(bootstrap))
	binary.BigEndian.PutUint16(b[3:], uint16(nt))
	return p.agregar(fmt.Sprintf("id\x00%d|%s|%s", bootstrap, nombre, desc), b)
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
// entradas del jar, que son ficheros de pocos KB: el android.jar entero no se toca. El índice de entradas
// sale de build/jarindice.bin cuando la huella del jar cuadra (jarindice.go), y si no se recorre el
// directorio central; en los dos casos se lee con `archivo`, que es el jar abierto para ReadAt.
type buscadorDeClases struct {
	ruta    string                // el fichero que se abrió, para nombrarlo en el diagnóstico
	archivo *os.File              // el jar abierto para leer las entradas por su offset
	zr      *zip.ReadCloser       // el respaldo: cuando el directorio central no se pudo recorrer
	indice  map[string]entradaJar // nombre internal -> entrada, para no andar el listado en cada lectura
	cache   map[string]*classFull
}

func abrirBuscador(rutaJar string) (*buscadorDeClases, error) {
	// una sola lectura del directorio central: su digest decide si la caché vale, y si no vale sus
	// bytes son los que se parsean
	directorio, tamano, cuenta, err := leeDirectorioCentral(rutaJar)
	if err == nil {
		huella := crc32.ChecksumIEEE(directorio)
		indice, cacheado := leerIndiceDelJar(rutaJar, tamano, huella)
		if !cacheado {
			propio, err := parseaDirectorioCentral(directorio, cuenta)
			if err == nil {
				indice = propio
				escribirIndiceDelJar(rutaJar, tamano, huella, indice)
			}
		}
		if indice != nil {
			if archivo, err := os.Open(rutaJar); err == nil {
				return &buscadorDeClases{ruta: rutaJar, archivo: archivo, indice: indice, cache: map[string]*classFull{}}, nil
			}
		}
	}
	// zip64, un zip con prefijo o un directorio central roto: lo lee el archive/zip de siempre, que ya
	// sabe de esas formas
	return abrirBuscadorConZip(rutaJar)
}

// abrirBuscadorConZip es el camino de siempre, el que archive/zip resuelve solo. Se queda como
// respaldo de las formas de zip que el lector propio no sigue, y es también el oráculo de las pruebas.
func abrirBuscadorConZip(rutaJar string) (*buscadorDeClases, error) {
	zr, err := zip.OpenReader(rutaJar)
	if err != nil {
		return nil, fmt.Errorf("el jar %s no abre: %w", rutaJar, err)
	}
	b := &buscadorDeClases{ruta: rutaJar, zr: zr, indice: map[string]entradaJar{}, cache: map[string]*classFull{}}
	for _, f := range zr.File {
		internal, ok := recortaClase(f.Name)
		if !ok {
			continue
		}
		if _, ya := b.indice[internal]; !ya {
			// sin offset: en este camino la entrada se abre con archive/zip, que lee su cabecera
			// local por su cuenta, y sacar el offset de cada una cuesta más que recorrer el zip
			b.indice[internal] = entradaJar{
				comprimido: uint32(f.CompressedSize64),
				crc:        f.CRC32,
				metodo:     f.Method,
				f:          f,
			}
		}
	}
	return b, nil
}

func (b *buscadorDeClases) cerrar() {
	if b.zr != nil {
		_ = b.zr.Close()
	}
	if b.archivo != nil {
		_ = b.archivo.Close()
	}
}

// classpathDeClases es el classpath del emisor: el `android.jar` y las dependencias de `jar/`, en
// orden. Se pregunta en ese orden y gana el primero que tenga la clase, que es lo que hace javac con
// `-classpath android.jar:jar/*.jar`. Cada fuente es un buscador con su propio índice y su propia
// caché, así que abrir seis jars no cuesta más entradas leídas que abrir uno: del jar de dependencias
// solo se leen las clases que el fuente nombra, igual que del `android.jar`.
type classpathDeClases struct {
	fuentes []*buscadorDeClases
}

func abrirClasspath(rutas []string) (*classpathDeClases, error) {
	c := &classpathDeClases{fuentes: make([]*buscadorDeClases, 0, len(rutas))}
	for _, ruta := range rutas {
		b, err := abrirBuscador(ruta)
		if err != nil {
			c.cerrar()
			return nil, err
		}
		c.fuentes = append(c.fuentes, b)
	}
	return c, nil
}

func (c *classpathDeClases) cerrar() {
	for _, b := range c.fuentes {
		b.cerrar()
	}
}

func (c *classpathDeClases) hayEnElJar(internal string) bool {
	_, ok := c.jarDe(internal)
	return ok
}

// jarDe devuelve la ruta del primer artefacto del classpath que define la clase: el diagnóstico de
// una clase redefinida tiene que nombrar el jar concreto, no decir solo que «está en el classpath».
func (c *classpathDeClases) jarDe(internal string) (string, bool) {
	for _, b := range c.fuentes {
		if b.hayEnElJar(internal) {
			return b.ruta, true
		}
	}
	return "", false
}

// claseDelJar lee la clase del primer jar que la tenga. El error nombra el classpath entero: la clase
// puede faltar porque el fuente nombra algo que no está en ninguna dependencia, y decir «no está en el
// android.jar» cuando hay seis jars detrás esconde dónde mirar.
func (c *classpathDeClases) claseDelJar(internal string) (*classFull, error) {
	for _, b := range c.fuentes {
		if b.hayEnElJar(internal) {
			return b.claseDelJar(internal)
		}
	}
	return nil, fmt.Errorf("%s no esta en el classpath (android.jar ni jar/)", internal)
}

func (b *buscadorDeClases) hayEnElJar(internal string) bool {
	_, ok := b.indice[internal]
	return ok
}

// claseDelJar lee una entrada del jar y la pasa por el lector de la Fase 2, que es el que ya sabe andar un
// constant pool de verdad.
func (b *buscadorDeClases) claseDelJar(internal string) (*classFull, error) {
	if c, ok := b.cache[internal]; ok {
		return c, nil
	}
	e, ok := b.indice[internal]
	if !ok {
		return nil, fmt.Errorf("%s no esta en el android.jar", internal)
	}
	var data []byte
	var err error
	if e.f != nil {
		// el respaldo ya tiene la entrada abierta: archive/zip lee la cabecera local por su cuenta
		rc, e2 := e.f.Open()
		if e2 != nil {
			return nil, e2
		}
		data, err = io.ReadAll(rc)
		_ = rc.Close()
	} else {
		data, err = datosDeLaEntrada(b.archivo, e)
	}
	if err != nil {
		return nil, err
	}
	c, err := readClassFull(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", internal, err)
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
	busc    *classpathDeClases
	paquete string
	imports []string
	orden   []*jclas // en el orden del fuente, para resolver sin depender del azar de un mapa
	propias map[string]*jclas
	archivo string // el SourceFile que anota javac: el nombre del fuente, sin rutas
	// unidad: el índice del fuente que se está emitiendo, para resolver un nombre simple primero contra
	// su propio compilation unit (JLS 6.5.5.1).
	unidad int

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

	// ctorSintetico: por cada anidada privada que se instancia desde fuera, el nombre interno de la
	// clase marcador que javac añade (Exterior$1). La clave es el nombre interno de la anidada.
	ctorSintetico map[string]string
	registrando   *jclas // la clase cuyo arbol se esta recorriendo en registrar
	// origen guarda, por nombre interno de clase, la ruta del fuente que la declaró: dos fuentes que
	// declaren la misma clase se quedaban con la última en `propias`, así que al emitir el fuente
	// pedido sus nombres se resolvían contra la clase del otro fichero; javac rechaza esa doble
	// definición y el build no puede escribir dos veces el mismo nombre. `duplicada` es la primera que
	// aparece y el llamador la convierte en error antes de emitir nada. La clave lleva la ruta
	// completa (`fuente`) y no el nombre base (`archivo`), que es el SourceFile y no distingue dos
	// homónimos de carpetas distintas.
	origen    map[string]string
	duplicada *claseRepetida
	// fuente es la ruta con la que entró el fuente que se está registrando, para poder nombrar los
	// dos ficheros de una clase repetida.
	fuente string
	// objeto son los métodos que declara `java/lang/Object`, por `nombre descriptor` (ver
	// `metodoDeObject`). Se llena una sola vez, la primera vez que una llamada sobre una interfaz
	// necesita saber si el método es de `Object`.
	objeto map[string]bool
}

// claseRepetida: la clase que declararon dos fuentes del proyecto, con las dos rutas para nombrarlas.
type claseRepetida struct {
	clase string // nombre interno
	antes string // el fuente que la declaró primero
	ahora string // el que la vuelve a declarar
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
	// el alcance (paquete e imports) del fuente donde se declaró la clase viaja con ella: al resolver un
	// descriptor suyo desde otro fichero se usa éste, no el del llamador
	c.imports = ctx.imports
	c.unidad = ctx.unidad
	interno := internoDe(c.paquete, c.nombre)
	if ctx.origen == nil {
		ctx.origen = map[string]string{}
	}
	if antes, visto := ctx.origen[interno]; visto && antes != ctx.fuente && ctx.duplicada == nil {
		ctx.duplicada = &claseRepetida{clase: interno, antes: antes, ahora: ctx.fuente}
	}
	ctx.origen[interno] = ctx.fuente
	ctx.propias[interno] = c
	antes := ctx.registrando
	ctx.registrando = c
	for _, m := range c.metodos {
		for _, s := range m.cuerpo {
			ctx.registrarSentencia(s)
		}
	}
	for _, d := range c.interior {
		ctx.registrar(d)
	}
	ctx.registrando = antes
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
	case jporCada:
		ctx.registrarExpresion(v.coleccion)
		ctx.registrarSentencia(v.cuerpo)
	case jhacer:
		ctx.registrarSentencia(v.cuerpo)
		ctx.registrarExpresion(v.cond)
	case jtry:
		for i := range v.recursos {
			ctx.registrarSentencia(v.recursos[i])
		}
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
		if v.selector != nil {
			ctx.registrarExpresion(v.selector)
		}
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
		// un `new Hija()` desde la clase que la contiene, siendo Hija private, pide el constructor
		// sintetico y la clase marcador de javac; se anota aqui porque las hijas se emiten ANTES que
		// la exterior y el emisor necesita saberlo al escribir la anidada
		if ctx.registrando != nil && v.cuerpo == nil {
			for _, h := range ctx.registrando.interior {
				nombreCorto := h.nombre
				if i := strings.LastIndex(nombreCorto, "$"); i >= 0 {
					nombreCorto = nombreCorto[i+1:]
				}
				if !h.esPrivada || (v.tipo != nombreCorto && !strings.HasSuffix(v.tipo, "."+nombreCorto)) {
					continue
				}
				if h.encierre != nil {
					ctx.ctorSintetico[internoDe(h.paquete, h.nombre)] = internoDe(h.encierre.paquete, h.encierre.nombre) + "$1"
				}
			}
		}
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
		for _, dim := range v.dimensiones {
			ctx.registrarExpresion(dim)
		}
		for _, el := range v.elementos {
			ctx.registrarExpresion(el)
		}
	case jindice:
		ctx.registrarExpresion(v.base)
		ctx.registrarExpresion(v.indice)
	case jincr:
		ctx.registrarExpresion(v.destino)
	case jasignExp:
		// la asignacion como valor del frente de la linea 187: sus dos lados son expresiones mas
		ctx.registrarExpresion(v.destino)
		ctx.registrarExpresion(v.valor)
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
	case jnegativo:
		ctx.registrarExpresion(v.dentro)
	case jlambda:
		// el cuerpo de la lambda es una expresion mas: un `new Hija()` privada dentro de ella pide el
		// mismo constructor sintetico que fuera, y las anonimas que cree se registran igual
		ctx.registrarExpresion(v.cuerpo)
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
		case jporCada:
			expr(v.coleccion)
			sent(v.cuerpo)
		case jhacer:
			sent(v.cuerpo)
			expr(v.cond)
		case jswitch:
			if v.selector != nil {
				expr(v.selector)
			}
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
		case jnegativo:
			expr(v.dentro)
		case jnuevoArreglo:
			if v.largo != nil {
				expr(v.largo)
			}
			for _, dim := range v.dimensiones {
				expr(dim)
			}
			for _, el := range v.elementos {
				expr(el)
			}
		case jindice:
			expr(v.base)
			expr(v.indice)
		case jincr:
			expr(v.destino)
		case jasignExp:
			expr(v.destino)
			expr(v.valor)
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

// resolverSimple sigue el orden de JLS 6.5.5.1, que es el de javac: una clase declarada en el mismo
// fichero, un import concreto, una clase del mismo paquete, los imports con estrella y por último
// java.lang. El import explícito gana al homónimo del paquete y, sobre todo, al de otro fichero del
// proyecto: antes se barrían todas las clases del proyecto primero y un `Helper` de otro paquete se
// comía al `import com.x.Helper` (medido con javac 17: el import gana incluso al del mismo paquete).
func (ctx *contextoJava) resolverSimple(nombre string) (string, error) {
	// 1. el mismo compilation unit: javac lo pone por delante de los imports y, si además hay un import
	//    del mismo nombre simple, lo rechaza como «already defined in this compilation unit»
	for _, c := range ctx.orden {
		if c.unidad != ctx.unidad {
			continue
		}
		if simpleDeClase(c) == nombre {
			return internoDe(c.paquete, c.nombre), nil
		}
	}
	// 2. un import concreto
	for _, imp := range ctx.imports {
		if strings.HasSuffix(imp, ".*") || sinPaquete(imp) != nombre {
			continue
		}
		if c := strings.ReplaceAll(imp, ".", "/"); ctx.hay(c) {
			return c, nil
		}
	}
	// 3. una clase del mismo paquete, del proyecto o del classpath
	for _, c := range ctx.orden {
		if c.paquete != ctx.paquete {
			continue
		}
		if simpleDeClase(c) == nombre {
			return internoDe(c.paquete, c.nombre), nil
		}
	}
	if c := internoDe(ctx.paquete, nombre); ctx.hay(c) {
		return c, nil
	}
	// 4. los imports con estrella
	for _, imp := range ctx.imports {
		if !strings.HasSuffix(imp, ".*") {
			continue
		}
		c := strings.ReplaceAll(strings.TrimSuffix(imp, ".*"), ".", "/") + "/" + nombre
		if ctx.hay(c) {
			return c, nil
		}
	}
	// 5. java.lang
	if ctx.hay("java/lang/" + nombre) {
		return "java/lang/" + nombre, nil
	}
	return "", fmt.Errorf("el tipo %s no se resuelve: ni en el fuente, ni en su paquete, ni en los imports, ni en java.lang, ni en el classpath (android.jar ni jar/)", nombre)
}

// simpleDeClase: el nombre simple con el que se nombra una clase del proyecto, sin paquete y, si es
// anidada (`Exterior$Anidada`), sin el prefijo del exterior.
func simpleDeClase(c *jclas) string {
	simple := sinPaquete(c.nombre)
	if i := strings.LastIndex(simple, "$"); i >= 0 {
		simple = simple[i+1:]
	}
	return simple
}

// tipoAInterno resuelve `MenuItem.OnMenuItemClickListener` contra el jar: la cabeza por resolverSimple y
// cada segmento siguiente como clase anidada, con el `$` con el que javac nombra las anidadas.
func (ctx *contextoJava) tipoAInterno(nombre string) (string, error) {
	bits := strings.Split(nombre, ".")
	if _, ok := descPrimitivo[bits[0]]; ok {
		return "", fmt.Errorf("el tipo primitivo %q no puede nombrar una clase", bits[0])
	}
	// un nombre con su paquete (`android.widget.ArrayAdapter`, `android.os.Parcelable.Creator`) no
	// llega por resolverSimple: su primer segmento es un paquete, no una clase. Se prueba el prefijo
	// más largo que sí sea una clase del classpath y los segmentos que quedan como anidadas con `$`.
	for corte := len(bits) - 1; corte >= 1; corte-- {
		base := strings.Join(bits[:corte+1], "/")
		if !ctx.hay(base) {
			continue
		}
		for _, seg := range bits[corte+1:] {
			if !ctx.hay(base + "$" + seg) {
				return "", fmt.Errorf("%s no es una clase anidada de %s en este classpath", seg, base)
			}
			base += "$" + seg
		}
		return base, nil
	}
	base, err := ctx.resolverSimple(bits[0])
	if err != nil {
		if len(bits) > 1 {
			// el error tiene que nombrar el tipo entero, no su primer segmento
			return ctx.resolverSimple(nombre)
		}
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

// tipoDeReferencia parte una cadena sin receptor (`com.x.dos.Helper.doble`, `Outer.Inner.CAMPO`) en el
// nombre interno del tipo y los segmentos que quedan, que ya son campos. Toma el prefijo más largo que
// resuelva a una clase, que es lo que javac hace con un nombre cualificado: `pkg.Clase.campo` es la
// clase del paquete, no la clase `pkg` con un campo `Clase`. Las clases anidadas se dejan a
// `accesoEstatico`, que anota la `Class` de cada una: `R.layout.main` tiene que seguir pasando por
// `R$layout` para que quede su fila de InnerClasses. ok=false cuando ni el primer segmento es un tipo,
// y entonces el llamador devuelve el error de siempre con la referencia entera.
func (ctx *contextoJava) tipoDeReferencia(segs []string) (string, []string, bool) {
	for corte := len(segs); corte >= 2; corte-- {
		if c := strings.Join(segs[:corte], "/"); ctx.hay(c) {
			return c, segs[corte:], true
		}
	}
	if len(segs) > 0 {
		if interno, err := ctx.resolverSimple(segs[0]); err == nil {
			return interno, segs[1:], true
		}
	}
	return "", nil, false
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

// firmaDeClase escribe la `Signature` genérica de la clase: la del padre y las de sus interfaces, con
// el `Ljava/lang/Object;` delante cuando el padre declarado no la lleva. Devuelve "" cuando no hay
// ningún argumento de tipo, que es cuando javac no escribe el atributo.
//
// Medido con javac 17 `--release 8` sobre `class ApkAdapter extends ArrayAdapter<Apk>`:
// `super_class` es el crudo `android/widget/ArrayAdapter` y el atributo es
// `Signature: Landroid/widget/ArrayAdapter<Lcom/hola/apkcstudio/Apk;>;`. El argumento no toca el
// descriptor: es el borrado (type erasure), y sólo viaja en esta firma.
func (ctx *contextoJava) firmaDeClase(c *jclas) (string, error) {
	generica := len(c.superArgs) > 0
	for _, args := range c.implementaArgs {
		if len(args) > 0 {
			generica = true
		}
	}
	if !generica {
		return "", nil
	}
	// el padre del fuente es un nombre (`Activity`, `android.view.View`); cuando no declara ninguno el
	// ClassFile dice `java/lang/Object`, que ya está en forma interna y no se resuelve contra el classpath
	var firma string
	if c.super == "" {
		firma = "Ljava/lang/Object;"
	} else {
		var err error
		firma, err = ctx.firmaDeClausula(c.super, c.superArgs)
		if err != nil {
			return "", fmt.Errorf("linea %d: el padre %s: %w", c.linea, c.super, err)
		}
	}
	sb := strings.Builder{}
	sb.WriteString(firma)
	for i, im := range c.implementa {
		var args []tipoArgumento
		if i < len(c.implementaArgs) {
			args = c.implementaArgs[i]
		}
		firma, err := ctx.firmaDeClausula(im, args)
		if err != nil {
			return "", fmt.Errorf("linea %d: la interface %s: %w", c.linea, im, err)
		}
		sb.WriteString(firma)
	}
	return sb.String(), nil
}

// firmaDeClausula: un tipo con sus argumentos, como se escribe dentro de una `Signature`
// (`Landroid/widget/ArrayAdapter<Lcom/hola/apkcstudio/Apk;>;`). Sin argumentos es sólo su descriptor
// de clase, que es lo que la firma pone del padre cuando la parte genérica la trae una interfaz.
func (ctx *contextoJava) firmaDeClausula(nombre string, args []tipoArgumento) (string, error) {
	interno, err := ctx.tipoAInterno(nombre)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "L" + interno + ";", nil
	}
	sb := strings.Builder{}
	sb.WriteString("L" + interno + "<")
	for _, arg := range args {
		firma, err := ctx.firmaDeArgumento(arg)
		if err != nil {
			return "", err
		}
		sb.WriteString(firma)
	}
	sb.WriteString(">;")
	return sb.String(), nil
}

// firmaDeArgumento: un tipo declarado o un argumento de tipo. Un arreglo se escribe con sus `[`
// delante, como en el descriptor (`List<Apk>[]` es `[Ljava/util/List<…>;`); un argumento genérico
// anida sus propios argumentos (`List<List<String>>`). Un comodín se escribe como manda la JVMS
// 4.7.9.1: `*` para `?`, `+` delante de la cota de `? extends T` y `-` delante de la de `? super T`.
func (ctx *contextoJava) firmaDeArgumento(t tipoArgumento) (string, error) {
	if t.comodin != 0 {
		if t.cota == nil {
			return "*", nil
		}
		cota, err := ctx.firmaDeArgumento(*t.cota)
		if err != nil {
			return "", err
		}
		return string(t.comodin) + cota, nil
	}
	firma, err := ctx.firmaDeClausula(t.nombre, t.args)
	if err != nil {
		return "", err
	}
	return strings.Repeat("[", t.arreglos) + firma, nil
}

// firmaDeMetodo escribe la `Signature` genérica de un método: los tipos de los parámetros, el de
// vuelta y los `throws`, con los argumentos de tipo que traigan. Devuelve "" cuando ni el retorno ni
// ningún parámetro son genéricos, que es cuando javac no escribe el atributo. Medido con javac 17
// `--release 8`: `void guardar(List<Apk> copia)` deja el descriptor `(Ljava/util/List;)V` y la firma
// `(Ljava/util/List<Lcom/hola/apkcstudio/Apk;>;)V`.
func (ctx *contextoJava) firmaDeMetodo(m jmetodo) (string, error) {
	generica := m.retornoGenerico != nil
	for _, p := range m.params {
		if p.generico != nil {
			generica = true
		}
	}
	if !generica {
		return "", nil
	}
	sb := strings.Builder{}
	sb.WriteString("(")
	for _, p := range m.params {
		var firma string
		var err error
		if p.generico != nil {
			firma, err = ctx.firmaDeArgumento(*p.generico)
		} else {
			firma, err = ctx.descriptorDe(p.tipo)
		}
		if err != nil {
			return "", fmt.Errorf("el parámetro %s: %w", p.nombre, err)
		}
		sb.WriteString(firma)
	}
	sb.WriteString(")")
	var vuelta string
	var err error
	switch {
	case m.nombre == "<init>":
		vuelta = "V"
	case m.retornoGenerico != nil:
		vuelta, err = ctx.firmaDeArgumento(*m.retornoGenerico)
		if err != nil {
			return "", err
		}
	default:
		vuelta, err = ctx.descriptorDe(m.retorno)
		if err != nil {
			return "", err
		}
	}
	sb.WriteString(vuelta)
	// los `throws` NO van aquí: en una `Signature` solo caben los de tipo variable (`^TT;`), que están
	// fuera de este subconjunto; los concretos viajan en el atributo `Exceptions` del method_info.
	// Medido con javac 17 `--release 8` y por defecto: `void guardar(List<Apk>) throws IOException,
	// IllegalStateException` deja la firma `(Ljava/util/List<…>;)V`, sin los dos tipos detrás.
	return sb.String(), nil
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
	var interno string
	err := ctx.enAlcanceDe(c, func() error {
		var e error
		interno, e = ctx.tipoAInterno(c.super)
		return e
	})
	return interno, err
}

// enAlcanceDe ejecuta f con el paquete y los imports del fuente donde se declaró c. Los tipos de la
// firma de otra clase se resuelven con **su** alcance: `new Lienzo(this)` desde una clase que no
// importa `Context` tiene que resolver el parámetro de `Lienzo(Context)`, que sí lo importa.
func (ctx *contextoJava) enAlcanceDe(c *jclas, f func() error) error {
	paquete, imports, unidad := ctx.paquete, ctx.imports, ctx.unidad
	ctx.paquete, ctx.imports, ctx.unidad = c.paquete, c.imports, c.unidad
	defer func() {
		ctx.paquete, ctx.imports, ctx.unidad = paquete, imports, unidad
	}()
	return f()
}

// descriptorDeEn resuelve un tipo escrito en c con el alcance de c, no con el del llamador.
func (ctx *contextoJava) descriptorDeEn(c *jclas, tipo string) (string, error) {
	var desc string
	err := ctx.enAlcanceDe(c, func() error {
		var e error
		desc, e = ctx.descriptorDe(tipo)
		return e
	})
	return desc, err
}

// descDeMetodoEn: el descriptor `(tipos)vuelta` de un método declarado en c, resuelto con el alcance de
// su fuente. Es el que usa la resolución de llamadas cuando el método es de otra clase del proyecto.
func (ctx *contextoJava) descDeMetodoEn(c *jclas, m jmetodo) (string, error) {
	var desc string
	err := ctx.enAlcanceDe(c, func() error {
		var e error
		desc, e = ctx.descDeMetodo(m)
		return e
	})
	return desc, err
}

// objetoEnInterfazVirtual es el interruptor de la falsificación: apagado, una llamada a un método de
// `java/lang/Object` sobre un receptor de interfaz vuelve a ser `invokeinterface <interfaz>`, que es
// la forma vieja y deja de coincidir con javac. Encendido en el binario que se publica. Lo usa
// `javaemisor_objeto_interfaz_test.go`.
var objetoEnInterfazVirtual = true

// metodoDeObject: ¿`java/lang/Object` declara ese método con ese descriptor? Es la pregunta que decide
// si una llamada sobre un receptor de tipo interfaz se emite como `invokevirtual java/lang/Object.*`
// en vez de `invokeinterface` (ver `llamada`). Los métodos se cargan una sola vez por compilación; si
// el classpath no trae `java/lang/Object` la respuesta es «no» y se emite lo de antes.
func (ctx *contextoJava) metodoDeObject(nombre, desc string) bool {
	if ctx.objeto == nil {
		ctx.objeto = map[string]bool{}
		if c, err := ctx.busc.claseDelJar("java/lang/Object"); err == nil {
			for _, m := range c.Methods {
				ctx.objeto[m.Name+" "+m.Desc] = true
			}
		}
	}
	return ctx.objeto[nombre+" "+desc]
}

// declaraElMismo: ¿el tipo `interno` declara él mismo un método con ese nombre y descriptor? No sube a
// los padres ni a las superinterfaces a propósito: javac resuelve el `toString()` de un receptor
// `Hereda extends CharSequence` contra `java/lang/Object` aunque `CharSequence` lo declare, y solo se
// queda en la interfaz cuando el tipo estático del receptor lo declara él mismo (medido con javac 17
// `--release 8 -g:lines,source`, tabla en `javaemisor_objeto_interfaz_test.go`).
func (ctx *contextoJava) declaraElMismo(interno, nombre, desc string) bool {
	if c, ok := ctx.clasePropia(interno); ok {
		for _, m := range c.metodos {
			if m.nombre != nombre {
				continue
			}
			if d, err := ctx.descDeMetodoEn(c, m); err == nil && d == desc {
				return true
			}
		}
		return false
	}
	c, err := ctx.busc.claseDelJar(interno)
	if err != nil {
		return false
	}
	for _, m := range c.Methods {
		if m.Name == nombre && m.Desc == desc {
			return true
		}
	}
	return false
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
		if c.esPrivada {
			acc |= accPrivate
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
	// varargs: la declaracion trae `...` (en el fuente) o ACC_VARARGS (en el jar). empaquetar: esta
	// llamada concreta baja los argumentos sobrantes a un arreglo nuevo, que es la forma que escribe
	// javac; una llamada varargs con un arreglo del tipo declarado no empaqueta y se pasa tal cual.
	varargs    bool
	empaquetar bool
	// decl es la declaracion del fuente cuando la clase es de esta pasada: por ella se sabe el tipo
	// generico del parametro formal (`Consumer<Apk>`), que es lo que instancia el tipo de una lambda.
	// Una candidata del classpath lo deja a nil.
	decl *jmetodo
}

// buscarMetodo recorre la clase y sus padres por nombre y aridad, y elige la sobrecarga en las tres
// fases de JVMS 15.12.2. Con **una** candidata devuelve esa, que es lo que este codigo habia hecho
// siempre; con varias:
//
//	fase 1 (15.12.2.2): identidad, ensanchamiento primitivo, subida de referencias y `null`, sin boxing
//	fase 2 (15.12.2.3): lo anterior mas boxing y desenvuelto (con su ensanchamiento detras)
//	fase 3 (15.12.2.4): las varargs, solo si ninguna de aridad fija fue aplicable
//
// En cada fase, si hay varias aplicables manda `masEspecificaDe` de esa fase; si ninguna es la mas
// especifica, la llamada es ambigua y se rechaza en vez de elegir por el orden de la jerarquia.
// Medido el 2026-11-01 (parrafo 62) con `System.out.println(que)`: paraba en el primer nombre con la
// aridad debida y bajaba `println:(Z)V`. Medido despues con javac 17 sobre `f(null)` con `f(String)` y
// `f(Object)` —gana `(Ljava/lang/String;)V`—, `f(1)` con `f(Integer)` y `f(long)` —gana `(J)V`— y
// `f('a')` con `f(Character)` y `f(int)` —gana `(I)V`—.
func (ctx *contextoJava) buscarMetodo(interno, nombre string, argc int, tipos []string) (candidato, error) {
	cands, err := ctx.candidatosDe(interno, nombre, argc, true)
	if err != nil {
		return candidato{}, err
	}
	return ctx.elegirSobrecarga(interno, nombre, cands, argc, tipos)
}

// elegirSobrecarga resuelve entre las candidatas ya juntadas por nombre y aridad con las tres fases de
// `buscarMetodo`. Está separada para que el estimador de tipo de una llamada sin receptor
// (`unaCandidataPorTipos`) elija con la misma regla que la emisión en vez de contar candidatas, que era
// lo que rechazaba un fuente con dos métodos de igual aridad en la clase exterior.
func (ctx *contextoJava) elegirSobrecarga(interno, nombre string, cands []candidato, argc int, tipos []string) (candidato, error) {
	if len(cands) == 0 {
		return candidato{}, fmt.Errorf("el metodo %s.%s con %d argumentos no esta en %s ni en sus padres",
			interno, nombre, argc, interno)
	}
	if len(cands) == 1 {
		c := cands[0]
		if c.varargs {
			// con una sola candidata no hay eleccion, pero si hay que decidir la forma: un arreglo del
			// tipo declarado se pasa tal cual y cualquier otra cosa se empaqueta. El `null` cabe en el
			// arreglo por la fase estricta, y javac lo pasa **como el arreglo** (`aconst_null;
			// invokestatic f:([Ljava/lang/String;)V`), no envuelto en un `new String[]{null}`.
			_, _, ps := splitDescriptor(c.desc)
			c.empaquetar = len(ps) == 0 || len(tipos) != len(ps) ||
				!ctx.asignableEstricto(tipos[len(ps)-1], ps[len(ps)-1])
		}
		return c, nil
	}
	// aridad fija: las de aridad exacta, varargs incluidas (una `T...` con la aridad del arreglo se
	// puede llamar como fija y pasa el arreglo tal cual).
	fijas := make([]candidato, 0, len(cands))
	for _, c := range cands {
		if _, _, ps := splitDescriptor(c.desc); len(ps) == len(tipos) {
			fijas = append(fijas, c)
		}
	}
	fases := []struct {
		nombre string
		cabe   func(desde, hasta string) bool
	}{
		{"estricta", ctx.asignableEstricto},
		{"laxa", ctx.asignable},
	}
	for _, fase := range fases {
		aplicables := ctx.aplicablesDe(fijas, tipos, fase.cabe)
		if len(aplicables) == 0 {
			continue
		}
		if len(aplicables) == 1 {
			return aplicables[0], nil
		}
		if unica, ok := ctx.masEspecificaDe(aplicables); ok {
			return unica, nil
		}
		posibles := make([]string, 0, len(aplicables))
		for _, c := range aplicables {
			posibles = append(posibles, c.desc)
		}
		return candidato{}, fmt.Errorf("%s.%s tiene %d sobrecargas aplicables en la fase %s a (%s) (%s) y ninguna es la mas especifica para este subconjunto",
			interno, nombre, len(aplicables), fase.nombre, strings.Join(posibles, " "), strings.Join(tipos, " "))
	}
	// fase 3 (JVMS 15.12.2.4): solo si no habia ninguna candidata de aridad fija aplicable. Los
	// `len(ps)-1` primeros argumentos van a los parametros fijos y el resto, cada uno convertible al
	// tipo ELEMENTO del ultimo parametro, se empaqueta en un arreglo nuevo.
	varargs := []candidato{}
	for _, c := range cands {
		if !c.varargs {
			continue
		}
		_, _, ps := splitDescriptor(c.desc)
		if len(ps) == 0 || len(ps[len(ps)-1]) == 0 || ps[len(ps)-1][0] != '[' {
			continue
		}
		n := len(ps) - 1
		if argc < n {
			continue
		}
		elemento := ps[n][1:]
		cabe := true
		for i := 0; i < n; i++ {
			if !ctx.asignable(tipos[i], ps[i]) {
				cabe = false
				break
			}
		}
		for i := n; cabe && i < argc; i++ {
			if !ctx.asignable(tipos[i], elemento) {
				cabe = false
			}
		}
		if cabe {
			c.empaquetar = true
			varargs = append(varargs, c)
		}
	}
	if len(varargs) == 1 {
		return varargs[0], nil
	}
	if len(varargs) > 1 {
		if unica, ok := ctx.masEspecificaDe(varargs); ok {
			return unica, nil
		}
	}
	posibles := make([]string, 0, len(cands))
	for _, c := range cands {
		posibles = append(posibles, c.desc)
	}
	return candidato{}, fmt.Errorf("%s.%s tiene %d sobrecargas de %d argumentos (%s) y los tipos llamados son (%s): ninguna es la mas especifica para este subconjunto",
		interno, nombre, len(cands), argc, strings.Join(posibles, " "), strings.Join(tipos, " "))
}

// aplicablesDe: las candidatas de aridad exacta cuyos parametros aceptan los tipos llamados con la
// conversion de invocacion que toca (`cabe` es `asignableEstricto` en la fase 1 y `asignable` en la 2).
func (ctx *contextoJava) aplicablesDe(cands []candidato, tipos []string, cabe func(desde, hasta string) bool) []candidato {
	aplicables := make([]candidato, 0, len(cands))
	for _, c := range cands {
		_, _, ps := splitDescriptor(c.desc)
		if len(ps) != len(tipos) {
			continue
		}
		todas := true
		for i := range ps {
			if !cabe(tipos[i], ps[i]) {
				todas = false
				break
			}
		}
		if todas {
			aplicables = append(aplicables, c)
		}
	}
	return aplicables
}

// masEspecificaDe: la unica candidata que cabe en todas las demas (JVMS 15.12.2.5). Devuelve falso si
// no hay ninguna o si hay empate.
//
// Con varias máximas por parámetros —lo normal es que los parámetros sean idénticos, que es el caso de
// un método heredado y redefinido— se desempata por el tipo de vuelta, como javac: de las máximas se
// queda la que devuelve un tipo que cabe en la vuelta de todas las demás (la más estrecha). Medido con
// javac 17 sobre `ListView.getAdapter()` de ApkShare, que ve dos declaraciones sin parámetros: la de
// `AbsListView`, que devuelve `ListAdapter`, y la genérica de `AdapterView`, que borra a `Adapter`; javac
// escribe `()Landroid/widget/ListAdapter;`. Si los retornos no están emparentados no hay desempate y la
// llamada sigue siendo ambigua.
func (ctx *contextoJava) masEspecificaDe(cands []candidato) (candidato, bool) {
	mas := ctx.maximasPorParametros(cands)
	if len(mas) == 1 {
		return mas[0], true
	}
	if len(mas) > 1 && desempataPorVuelta {
		if unica, ok := ctx.masEstrictaPorVuelta(mas); ok {
			return unica, true
		}
	}
	return candidato{}, false
}

// desempataPorVuelta es el interruptor de la sonda del desempate por retorno covariante. Apagado, dos
// máximas de parámetros iguales vuelven a ser ambiguas y el emisor devuelve el error literal de
// `ListView.getAdapter` de ApkShare; lo usa la falsificación de
// `javaemisor_vuelta_covariante_test.go`. Encendido en el binario que se publica.
var desempataPorVuelta = true

// maximasPorParametros: las candidatas que caben en todas las demás por su lista de parámetros
// (JVMS 15.12.2.5, la regla de siempre). Con parámetros idénticos las deja todas: el desempate por
// vuelta es otro paso.
func (ctx *contextoJava) maximasPorParametros(cands []candidato) []candidato {
	mas := []candidato{}
	for _, a := range cands {
		especifica := true
		for _, b := range cands {
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
	return mas
}

// masEstrictaPorVuelta: entre máximas de parámetros iguales, la única cuya vuelta cabe en la de todas
// las demás (JVMS 4.6.5, `R1 <: R2`). Es el caso de un método redefinido con retorno covariante: el
// `getAdapter` de `AbsListView` (`ListAdapter`) gana al de `AdapterView` (`Adapter`), y ese es el
// descriptor que javac escribe en el `invokevirtual` porque es el único que lleva la firma del DEX.
// Solo entre referencias: `int` no es más estrecho que `long` para esto, y además dos sobrecargas de
// parámetros iguales no pueden diferir en un primitivo sin colisionar en la firma.
func (ctx *contextoJava) masEstrictaPorVuelta(cands []candidato) (candidato, bool) {
	mas := []candidato{}
	for _, a := range cands {
		ra := vueltaDe(a.desc)
		especifica := true
		for _, b := range cands {
			rb := vueltaDe(b.desc)
			if a.desc == b.desc || (ra == rb && esTipoDeReferencia(ra)) {
				continue
			}
			if !esTipoDeReferencia(ra) || !esTipoDeReferencia(rb) || !ctx.asignableEstricto(ra, rb) {
				especifica = false
				break
			}
		}
		if especifica {
			mas = append(mas, a)
		}
	}
	if len(mas) == 1 {
		return mas[0], true
	}
	return candidato{}, false
}

// masEspecifico: si cada parametro de `a` cabe en el de `b` (JVMS 15.12.2.5: `Si <: Ti`, sin boxing;
// el ensanchamiento primitivo si cuenta, y por eso `f(int)` gana a `f(long)` y `f(long)` a `f(float)`).
func (ctx *contextoJava) masEspecifico(a, b string) bool {
	_, _, pa := splitDescriptor(a)
	_, _, pb := splitDescriptor(b)
	if len(pa) != len(pb) {
		return false
	}
	for i := range pa {
		if !ctx.asignableEstricto(pa[i], pb[i]) {
			return false
		}
	}
	return true
}

// tipoNulo: el tipo que se pone en la lista de una llamada por cada argumento que es el literal
// `null`. No es un descriptor de la JVM —no existe ningun tipo `N`— justamente para que no coincida
// con ninguna firma: la fase de coincidencia exacta no puede elegirlo y la especificidad se queda con
// la sobrecarga mas estrecha (JVMS 15.12.2.2 y 15.12.2.5). Lo que se emite sigue siendo un
// `aconst_null`, sin tipo propio; este marcador solo viaja en las listas de tipos.
const tipoNulo = "N"

// ampliaciones y boxing que javac admite en la resolucion de sobrecargas (JVMS 5.3): el emisor ya
// sabia escribir las conversiones en `expA`, aqui solo se dice que son aplicables para elegir candidata.
// La tabla es la cerradura del ensanchamiento primitivo de JLS 5.1.2: `byte` llega a `short`, `int`,
// `long`, `float` y `double`; `char` a `int`, `long`, `float` y `double`; y `boolean` **no** ensancha
// a nada (`f(1)` con `f(boolean)` no es Java).
var ampliacionPrimitiva = map[string]bool{
	"I>J": true, "I>F": true, "I>D": true, "J>F": true, "J>D": true, "F>D": true,
	"B>S": true, "B>I": true, "B>J": true, "B>F": true, "B>D": true,
	"S>I": true, "S>J": true, "S>F": true, "S>D": true,
	"C>I": true, "C>J": true, "C>F": true, "C>D": true,
}

var boxingDe = map[string]string{
	"I": "Ljava/lang/Integer;", "J": "Ljava/lang/Long;", "F": "Ljava/lang/Float;",
	"D": "Ljava/lang/Double;", "Z": "Ljava/lang/Boolean;", "C": "Ljava/lang/Character;",
	"S": "Ljava/lang/Short;", "B": "Ljava/lang/Byte;",
}

// primDeEnvoltorio: la vuelta de `boxingDe`, el primitivo que devuelve el `<x>Value()` de cada
// envoltorio (JLS 5.1.8). Sirve para el desenvuelto de la fase 2 y para escribirlo en `expA`.
var primDeEnvoltorio = map[string]string{
	"Ljava/lang/Integer;": "I", "Ljava/lang/Long;": "J", "Ljava/lang/Float;": "F",
	"Ljava/lang/Double;": "D", "Ljava/lang/Boolean;": "Z", "Ljava/lang/Character;": "C",
	"Ljava/lang/Short;": "S", "Ljava/lang/Byte;": "B",
}

// metodoDesenvoltura: el nombre del metodo que desenvuelve cada primitivo. Medido con `javap -c` sobre
// `void g(int)` y `void h(long)` llamados con un `Integer`: `invokevirtual Integer.intValue:()I`, y
// detras `i2l` cuando el hueco es mas ancho.
var metodoDesenvoltura = map[string]string{
	"I": "intValue", "J": "longValue", "F": "floatValue", "D": "doubleValue",
	"Z": "booleanValue", "C": "charValue", "S": "shortValue", "B": "byteValue",
}

// asignable: si un valor del tipo `desde` cabe en un parámetro del tipo `hasta` en la fase 2 de
// JVMS 15.12.2.3: lo estricto (identidad, ensanchamiento primitivo, subida de referencias y `null`) y
// además el boxing y el desenvuelto con su ensanchamiento detrás.
func (ctx *contextoJava) asignable(desde, hasta string) bool {
	if ctx.asignableEstricto(desde, hasta) {
		return true
	}
	// Boxing **seguido del ensanchamiento de referencia** (JVMS 5.3 y la conversión de invocación de
	// 15.12.2.4): un `int` no cabe solo en `Integer`, también en `Object`, `Number` o `Comparable`
	// pasando por `Integer`. Es lo que hacía falta para que `String.format("%d %s", 1, "x")` viera
	// aplicable `format(String, Object...)`: sin esto `asignable("I", "Ljava/lang/Object;")` era falso
	// y la fase 3 descartaba la única candidata varargs (§194). La recursión no puede morderse la cola
	// porque el envoltorio ya es de referencia: esa rama no vuelve a entrar en el boxing.
	if box, ok := boxingDe[desde]; ok {
		return ctx.asignableEstricto(box, hasta)
	}
	// Desenvuelto **seguido del ensanchamiento primitivo**: `Integer` cabe en un `int` por
	// `intValue()` y en un `long` por `intValue(); i2l`. Solo hacia un primitivo: hacia una referencia
	// ya lo resolvió (o no) el recorrido de padres de `asignableEstricto`, y volver a entrar aquí
	// giraría en el vacío.
	if prim, ok := primDeEnvoltorio[desde]; ok && len(hasta) == 1 {
		return ctx.asignableEstricto(prim, hasta)
	}
	return false
}

// cabeEn: si un valor del descriptor `dv` se guarda en un hueco del descriptor `hasta` sin que javac
// escriba conversión. Es la asignación de JLS 5.2 con lo estricto de 5.1.1-5.1.5: identidad, el
// literal `null` en cualquier referencia y la subida de referencias —clase, interfaz y la covarianza
// de arreglos—. Medido en ApkShare `AppAdapter.java:33`: `List<App> apps_filter` y `new ArrayList<>()`
// conviven porque `ArrayList` es `List` y el verificador admite el subtipo sin `checkcast`; javac solo
// escribe `new ArrayList; dup; aload; invokespecial; putfield List`. Un primitivo **no** cabe por aquí:
// su ensanchamiento necesita la instrucción de conversión que ya escribe `expA`.
func (ctx *contextoJava) cabeEn(dv, hasta string) bool {
	if dv == hasta {
		return true
	}
	if !cabeSubidaDeReferencia {
		return false
	}
	return esTipoDeReferencia(dv) && esTipoDeReferencia(hasta) && ctx.asignableEstricto(dv, hasta)
}

// cabeSubidaDeReferencia es el interruptor de la sonda de la subida de referencias al escribir un
// campo. Apagado, `cabeEn` vuelve a exigir el descriptor exacto —la comparación `dv != cr.desc` de
// antes—, que es lo que devuelve el error literal del usuario sobre `AppAdapter.java:33`. Encendido en
// el binario que se publica; lo usa la falsificación de `javaemisor_appadapter_cast_test.go`.
var cabeSubidaDeReferencia = true

// asignableEstricto: la conversión de invocación de la fase 1 (JVMS 15.12.2.2): identidad,
// ensanchamiento primitivo, subida de referencias —los arreglos suben a `Object`, `Cloneable` y
// `Serializable`— y el literal `null` a cualquier referencia. **Sin** boxing ni desenvuelto: con
// `f(Integer)` y `f(long)`, un `f(1)` no ve aplicable la primera en la fase estricta y javac se queda
// con `(J)V`. El `null` se reconoce por `tipoNulo`, que no coincide con ningún descriptor real.
func (ctx *contextoJava) asignableEstricto(desde, hasta string) bool {
	if desde == tipoNulo {
		return esTipoDeReferencia(hasta)
	}
	if desde == hasta {
		return true
	}
	if ampliacionPrimitiva[desde+">"+hasta] {
		return true
	}
	if !esTipoDeReferencia(desde) || !esTipoDeReferencia(hasta) {
		return false
	}
	if desde[0] == '[' {
		// JVMS 4.10.3: todo arreglo es subtipo de Object, Cloneable y Serializable
		if hasta == "Ljava/lang/Object;" || hasta == "Ljava/lang/Cloneable;" ||
			hasta == "Ljava/io/Serializable;" {
			return true
		}
		// y la covarianza de arreglos (JLS 4.10.3) es solo entre elementos de REFERENCIA:
		// `String[]` es `Object[]`, pero `int[]` no es `long[]` ni `Object[]`. Medido con javac 17:
		// `f(new String[1])` con `f(Object[])` y `f(Object)` elige `(Ljava/lang/Object;)V`, porque
		// `String[] <: Object[]` y por eso `Object[]` es la candidata mas especifica.
		if hasta[0] == '[' {
			de, a := desde[1:], hasta[1:]
			if esTipoDeReferencia(de) && esTipoDeReferencia(a) {
				return ctx.asignableEstricto(de, a)
			}
		}
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

// interfacesDeLaPropiaEnAsignable es el interruptor de la sonda de las interfaces de una clase
// propia. Apagado, `superEInterfaces` vuelve a devolver solo el padre —la forma que daba el error
// literal del usuario sobre `MainActivity.java:99` de ApkShare—; encendido en el binario que se
// publica. Lo usa la falsificación de `javaemisor_interfaz_anonima_test.go`.
var interfacesDeLaPropiaEnAsignable = true

// superEInterfaces: el padre y las interfaces de un tipo, venga del fuente o del `android.jar`.
//
// Una clase propia devuelve también las interfaces que declara implementar. Sin ellas,
// `Runnable r = new Runnable() { … }` se leía como un tipo ajeno y el `access$` de escritura del
// campo rechazaba la asignación con «runnable es Ljava/lang/Runnable; y se le asigna un
// Lshare/apps/util/MainActivity$4$1;» (ApkShare, `MainActivity.java:99`). Los nombres viajan como
// se escribieron en el fuente (`Runnable`), así que se resuelven con el alcance de su fichero —el
// mismo que usa `padreDe`— y no con el del llamador.
func (ctx *contextoJava) superEInterfaces(interno string) (padre string, interfaces []string, err error) {
	if propia, ok := ctx.clasePropia(interno); ok {
		p, err := ctx.padreDe(propia)
		if err != nil {
			return "", nil, err
		}
		if !interfacesDeLaPropiaEnAsignable {
			return p, nil, nil
		}
		resolver := func() error {
			for _, im := range propia.implementa {
				nombre, e := ctx.tipoAInterno(im)
				if e != nil {
					return fmt.Errorf("linea %d: la interfaz %s: %w", propia.linea, im, e)
				}
				interfaces = append(interfaces, nombre)
			}
			return nil
		}
		if e := ctx.enAlcanceDe(propia, resolver); e != nil {
			return "", nil, e
		}
		return p, interfaces, nil
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

// aridadSirve: si una sobrecarga de `n` parametros puede servir a una llamada de `argc`. La aridad
// exacta sirve siempre; una varargs tambien con menos argumentos, porque los que falten pueden venir
// en un arreglo vacio (JVMS 15.12.2.4, `argc >= n-1`). La comprobacion de tipos la hace buscarMetodo.
func aridadSirve(n int, varargs bool, argc int) bool {
	if varargs && n > 0 {
		return argc >= n-1
	}
	return n == argc
}

// candidatos: las sobrecargas de nombre y aridad exactos, en el orden de la jerarquia (la clase propia
// primero, luego el `android.jar`) y deduplicadas por descriptor, que es como javac ve un mismo metodo
// declarado y heredado. Es la vista estricta que usan los estimadores de tipo; la resolucion de una
// llamada usa `candidatosDe` con los varargs incluidos.
func (ctx *contextoJava) candidatos(interno, nombre string, argc int) ([]candidato, error) {
	return ctx.candidatosDe(interno, nombre, argc, false)
}

// candidatosDe: lo mismo, y con `conVarargs` tambien las declaraciones `...` para las que alcanza esta
// aridad. Las varargs se marcan para que buscarMetodo decida en su fase 3 y para que la emision sepa
// que hay que empaquetar (§194).
func (ctx *contextoJava) candidatosDe(interno, nombre string, argc int, conVarargs bool) ([]candidato, error) {
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
			cs, err := ctx.busc.candidatosDelJar(actual, nombre, argc, conVarargs)
			if err != nil {
				return nil, err
			}
			for _, c := range cs {
				poner(c)
			}
			return sal, nil
		}
		for mi := range propia.metodos {
			m := &propia.metodos[mi]
			if m.nombre != nombre {
				continue
			}
			if !(len(m.params) == argc || (conVarargs && aridadSirve(len(m.params), m.varargs, argc))) {
				continue
			}
			d, err := ctx.descDeMetodoEn(propia, *m)
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
			poner(candidato{desc: d, acceso: acceso, varargs: m.varargs, decl: m})
		}
		if nombre == "<init>" && propia.encierre != nil && !propia.anonima {
			// el constructor de una anidada no estatica lo sintetiza el emisor con el exterior delante;
			// el AST no lo tiene, así que lo ofrecemos aquí a la resolución de `new Hija()`.
			explicito := false
			for _, m := range propia.metodos {
				if m.nombre == "<init>" {
					explicito = true
				}
			}
			if !explicito && !propia.esEstatica {
				ext := internoDe(propia.encierre.paquete, propia.encierre.nombre)
				poner(candidato{desc: "(L" + ext + ";)V", acceso: 0})
			}
			// la anidada estatica privada que se instancia desde fuera: javac le pone `private` al
			// constructor que el emisor sintetiza, y esa es la firma que la resolucion tiene que ver
			if !explicito && propia.esEstatica && ctx.ctorSintetico[interno] != "" {
				poner(candidato{desc: "()V", acceso: accPrivate})
			}
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

// candidatosDelJar: lo mismo sobre las clases del classpath, subiendo por la cadena de padres y, después,
// por las superinterfaces. La cadena puede cruzar de un jar a otro (una dependencia que extiende una
// clase del `android.jar`), y por eso la búsqueda de la clase siguiente también pasa por el classpath y
// no por un solo jar.
//
// Las interfaces se recorren en anchura y se añaden detrás de la cadena de padres: un método heredado de
// una superinterfaz es tan candidato como uno heredado de una superclase, y javac los ve todos. Sin este
// recorrido, `ListAdapter.getItem(posicion)` no encontraba nada —`ListAdapter` no lo declara, lo hereda
// de `Adapter`—, que es el eslabón siguiente al desempate de `getAdapter` (§253).
func (cp *classpathDeClases) candidatosDelJar(interno, nombre string, argc int, conVarargs bool) ([]candidato, error) {
	sal := []candidato{}
	vistas := map[string]bool{}
	visitadas := map[string]bool{}
	// recorre lee una clase una sola vez: apunta sus candidatas y devuelve su padre y sus interfaces.
	recorre := func(actual string) (padre string, interfaces []string, err error) {
		if visitadas[actual] {
			return "", nil, nil
		}
		visitadas[actual] = true
		c, err := cp.claseDelJar(actual)
		if err != nil {
			return "", nil, err
		}
		for _, m := range c.Methods {
			varargs := m.Access&accVarargs != 0
			match := cuentaArgumentosJava(m.Desc) == argc
			if !match && conVarargs {
				match = aridadSirve(cuentaArgumentosJava(m.Desc), varargs, argc)
			}
			if m.Name != nombre || !match || vistas[m.Desc] {
				continue
			}
			vistas[m.Desc] = true
			sal = append(sal, candidato{desc: m.Desc, acceso: m.Access, varargs: varargs})
		}
		for _, i := range c.Interfaces {
			interfaces = append(interfaces, internoDeDescriptor(i))
		}
		if c.Super != "" {
			padre = internoDeDescriptor(c.Super)
		}
		return padre, interfaces, nil
	}

	// primero la cadena de padres, como antes de este cambio
	pendientes := []string{}
	actual := interno
	for visits := 0; actual != "" && visits < 32; visits++ {
		padre, interfaces, err := recorre(actual)
		if err != nil {
			return nil, err
		}
		pendientes = append(pendientes, interfaces...)
		if padre == "" || visitadas[padre] {
			break
		}
		actual = padre
	}
	// y después las superinterfaces, en anchura
	for len(pendientes) > 0 {
		actual := pendientes[0]
		pendientes = pendientes[1:]
		padre, interfaces, err := recorre(actual)
		if err != nil {
			return nil, err
		}
		// el padre de una interfaz es `java/lang/Object`, que ya trae la cadena de arriba
		if padre != "" && !visitadas[padre] {
			pendientes = append(pendientes, padre)
		}
		for _, i := range interfaces {
			if !visitadas[i] {
				pendientes = append(pendientes, i)
			}
		}
	}
	return sal, nil
}

type campoResuelto struct {
	desc      string
	acceso    int
	dueño     string // el tipo contra el que se escribe el Fieldref, que es el que nombra javac
	constante bool
	valor     int32
	// esCadena y cadena: un `static final String` con `ConstantValue` (tag 8). javac lo sustituye por
	// un `ldc` de la cadena y no deja ni `getstatic` ni el Fieldref, así que el valor viaja como el
	// literal, no como entero. Es el `Intent.ACTION_SEND` de ApkShare (§130 para el atributo).
	esCadena bool
	cadena   string
	// generico es el tipo declarado del campo con sus argumentos (`List<Apk>`), cuando lo trae. Es lo
	// que permite sustituir el tipo variable de la vuelta de un método genérico sin emitir el
	// receptor: `app.apks.get(0)` con `apks` un `List<Apk>` devuelve `Apk` y no el borrado `Object`.
	// Un campo del classpath lo deja a nil, y eso solo quiere decir «el descriptor y nada más».
	generico *tipoArgumento
}

// pliegaCadenaEstatica es el interruptor de la falsificación de la constante `String` del classpath:
// apagado, `Intent.ACTION_SEND` vuelve a rechazarse con «la constante ... no es un entero: doblarla no
// esta medida», que es el error literal que leyó el usuario en `ApkShare` (`MainActivity.java:198`;
// §130 midió el `ConstantValue` del atributo). Encendido en el binario que se publica; solo lo apaga
// `TestCadenaEstaticaFalsificacion`.
var pliegaCadenaEstatica = true

// pliegaCadenaHeredada es el interruptor de la falsificación de la constante `String` heredada sin
// receptor: un nombre suelto (`V`) que resuelve a un `static final String` con `ConstantValue` de un
// padre del classpath. Encendido emite `ldc` de la cadena; apagado revive el defecto anterior —un
// `iconst` con el descriptor de referencia— y el análisis del marco lo para con «se esperaba una
// referencia y hay entero». Solo lo apaga `TestCadenaEstaticaHeredadaFalsificacion`.
var pliegaCadenaHeredada = true

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
				d, err := ctx.descriptorDeEn(propia, f.tipo)
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
				cr := campoResuelto{desc: d, acceso: acc, dueño: dueño, constante: f.esFinal && f.esEstatico, generico: f.generico}
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
				switch f.ConstTag {
				case cpInteger:
					cr.constante = true
					cr.valor = f.ConstInt
				case cpString:
					if !pliegaCadenaEstatica {
						return campoResuelto{}, fmt.Errorf("la constante %s.%s no es un entero: doblarla no esta medida", actual, nombre)
					}
					// un `static final String` con `ConstantValue` es constante de compilación: javac la
					// sustituye por su literal y el valor tiene que viajar como cadena, no como entero
					cr.constante = true
					cr.esCadena = true
					cr.cadena = f.ConstStr
				default:
					return campoResuelto{}, fmt.Errorf("la constante %s.%s no es un entero ni una cadena: doblarla no esta medida", actual, nombre)
				}
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
	return compilarProyectoJavaConClasspath(fuentes, []string{rutaJar})
}

// errorDeFuente dice en qué fichero y en qué etapa del emisor nació un fallo del proyecto. El mensaje
// conserva la forma que ya tenían estos errores (`<fichero>: <explicacion>`), porque la consumen
// `compile`, `build` y `check`; la estructura existe para que `check` distinga un fallo del fuente
// pedido de uno de otro fichero del proyecto sin adivinarlo por el texto.
type errorDeFuente struct {
	fuente string // el nombre con el que entró el fuente, tal cual
	etapa  string // "parser", "registro" o "emisor"
	// antes cuenta los fuentes que ya se emitieron enteros cuando nació el fallo. El pedido va
	// primero en la lista, así que un fallo de emisión con `antes` > 0 no es suyo; sin este dato,
	// `check` daba por comprobado un fuente por el mero hecho de que el fallo fuera de emisión.
	antes int
	err   error
}

func (e *errorDeFuente) Error() string { return e.fuente + ": " + e.err.Error() }
func (e *errorDeFuente) Unwrap() error { return e.err }

// compilarProyectoJavaConClasspath es el emisor con el classpath entero: el `android.jar` primero y
// después las dependencias, en el orden en que se le pasan. El proyecto se registra antes de resolver
// nada, así que una clase del fuente gana a la del mismo nombre en una dependencia, como en javac.
func compilarProyectoJavaConClasspath(fuentes []fuenteJava, rutas []string) ([]claseEmitida, error) {
	busc, err := abrirClasspath(rutas)
	if err != nil {
		return nil, err
	}
	defer busc.cerrar()
	ctx := &contextoJava{busc: busc, propias: map[string]*jclas{},
		accesores: map[string][]accesorSintetico{}, accesorID: map[string]accesorSintetico{},
		accesorNumero: map[string]int{}, ctorSintetico: map[string]string{}}
	type parseado struct {
		a      *archivoJava
		nomb   string
		unidad int
	}
	var todos []parseado
	for i, f := range fuentes {
		a, err := parsearJava(f.contenido)
		if err != nil {
			return nil, &errorDeFuente{fuente: f.nombre, etapa: "parser", err: err}
		}
		// el paquete y los imports se ponen antes de registrar para que cada clase se quede con los de su
		// fuente: un descriptor suyo se resuelve después con ese alcance aunque lo pida otro fichero
		ctx.paquete, ctx.imports, ctx.archivo, ctx.unidad = a.paquete, a.imports, baseDeNombre(f.nombre), i
		ctx.fuente = f.nombre
		for _, c := range a.clases {
			ctx.registrar(c)
		}
		todos = append(todos, parseado{a, baseDeNombre(f.nombre), i})
	}
	// Dos fuentes que declaren la misma clase no llegan a emitirse: el mapa de clases propias se
	// quedaba con la última, y el diagnóstico del fuente pedido resolvía sus nombres contra la clase
	// del otro fichero. javac rechaza la doble definición y el build tampoco puede escribirla dos
	// veces, así que el fallo es del proyecto y el fuente pedido no se da por comprobado.
	if d := ctx.duplicada; d != nil {
		return nil, &errorDeFuente{fuente: d.ahora, etapa: "registro",
			err: fmt.Errorf("la clase %s ya está definida en %s: dos fuentes del proyecto no pueden declarar la misma clase",
				strings.ReplaceAll(d.clase, "/", "."), d.antes)}
	}
	// Una clase del proyecto que además viaje en el classpath: `hay` prefiere la propia, así que el
	// diagnóstico daba por bueno un fuente que llamaba a un miembro que solo existe en la copia local,
	// mientras el build dexa esa copia y el runtime resuelve contra el jar. Se contrasta antes de
	// emitir con el índice de cada artefacto ya abierto, sin leer una entrada de más.
	for _, c := range ctx.orden {
		interno := internoDe(c.paquete, c.nombre)
		if ruta, enJar := busc.jarDe(interno); enJar {
			fuente := ctx.origen[interno]
			return nil, &errorDeFuente{fuente: fuente, etapa: "registro",
				err: fmt.Errorf("la clase %s también está en %s: el fuente la redefine y la resolución usa la copia local",
					strings.ReplaceAll(interno, "/", "."), ruta)}
		}
	}
	var salida []claseEmitida
	for i, p := range todos {
		ctx.paquete = p.a.paquete
		ctx.imports = p.a.imports
		ctx.archivo = p.nomb
		ctx.unidad = p.unidad
		for _, c := range p.a.clases {
			hechas, err := ctx.emmitirArbol(c)
			if err != nil {
				return nil, &errorDeFuente{fuente: p.nomb, etapa: "emisor", antes: i, err: err}
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
	interno := internoDe(c.paquete, c.nombre)
	salida = append(salida, claseEmitida{interno: interno, datos: datos})
	// la marcador la escribe la clase exterior, despues de que sus metodos hayan pedido el
	// constructor sintetico; el orden final lo fija el sort de compilarProyectoJava
	for _, hija := range c.interior {
		if marcador, ok := ctx.ctorSintetico[internoDe(hija.paquete, hija.nombre)]; ok {
			datosM, err := ctx.emmitirMarcador(c, marcador)
			if err != nil {
				return nil, err
			}
			salida = append(salida, claseEmitida{interno: marcador, datos: datosM})
			break
		}
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
	estatico bool
	desc     string
	// esCtor marca el metodo que se emite como el constructor de la clase: los marcos de antes de su
	// `super`/`this` llevan `uninitializedThis`, y eso cambia la forma comprimida que javac elige.
	esCtor    bool
	vuelta    string
	locales   map[string]localVar
	siguiente int
	// ambitos es la pila de bloques abiertos: al cerrar cada uno se devuelve `siguiente` a lo que
	// valía al abrirlo, que es el reparto de javac (ver `ambito`).
	ambitos []ambito
	// muertes son los puntos donde un bloque del fuente se cierra, con los huecos que declaró: el
	// marco de javac los poda a `top` desde ahí aunque las dos ramas de una unión traigan el mismo
	// tipo (ver `muerteAmbito`).
	muertes []muerteAmbito
	// filaDeCierre es la del `}` del último bloque emitido cuando ese bloque declaró algún local con
	// valor, y cero si no: es la fila que javac escribe en el pc donde cierra el ámbito y que cae en el
	// `goto` que salta el `else` (ver `branca` y `jbloque.lineaCierre`).
	filaDeCierre int
	// picos son los altos que hay que vigilar aunque los bloques devuelvan su hueco: cada `try` con
	// `finally` deja abierto uno para saber dónde pone javac el local del manejador `any`, que va en
	// el registro más alto que alcanzó el cuerpo (ver `intenta`).
	picos   []int
	codigo  []byte
	lineas  []parLinea
	prof    int
	maxProf int
	maxLoc  int
	// truncado avisa que un indice del pool no cupo donde se escribio: el `ldc` corto con el
	// interruptor `ldcSinFormaLarga`, o un pool que ni en los dos bytes de `ldc_w` cabe. El fichero ya
	// no es correcto y el metodo se rechaza por nombre en vez de escribir un operando roto.
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

	// de la clase, para las lambdas del fuente: los `method_info` sinteticos que se escriben al final
	// del ClassFile, cuantos van, y la tabla de BootstrapMethods con su indice por clave (un
	// `invokedynamic` no se puede deduplicar sin ella). `metodo` es el nombre del metodo que se emite
	// ahora, para nombrar el implementador `lambda$metodo$N` como javac. `lambdaN` es el contador del
	// nombre: javac lo lleva POR CLASE y no por metodo, asi que dos sobrecargas del mismo nombre sacan
	// `lambda$f$0` y `lambda$f$1` en vez de dos `lambda$f$0` (que serian dos `method_info` identicos,
	// medido con `javap -p` sobre dos `f` con lambda en la misma clase).
	lambdaMetodos [][]byte
	// lambdaPendientes son los implementadores que aún no se han emitido: se vacían al cerrar la
	// clase, detrás del código de todos los métodos, como javac (ver `escribeImplementador`).
	lambdaPendientes []implementadorPendiente
	bootstraps       []classBootstrap
	bootIdx          map[string]int
	metodo           string
	lambdaN          int
}

type parcheSalto struct {
	en       int
	etiqueta int
	op       byte
	// largo avisa que el hueco ya se ensanchó: el `goto` ocupa 5 bytes (0xc8) o el condicional
	// 8 (la condición invertida y un `goto_w` detrás). Lo fija la pasada de colocación.
	largo bool
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
	// generico es el tipo declarado del local con sus argumentos (`List<Apk>`), cuando se conoce: es lo
	// que estrecha la vuelta de un método genérico cuando el receptor es esta local
	// (`m.get(0)` sobre un parámetro `List<Apk>`), igual que `campoResuelto.generico` para los campos.
	// nil quiere decir «no se sabe si traía argumentos»: con eso `vueltaSinEmitir` no estima, en vez de
	// devolver el borrado en silencio.
	generico *tipoArgumento
}

// ambito es un bloque abierto del fuente: el reparto de locales con el que empezó y los nombres que
// declaró dentro, para devolver las dos cosas al cerrarlo. Es lo que javac hace con `endScopes`: el
// registro de una local se libera al cerrar el bloque donde se declaró, y el siguiente local del
// mismo método vuelve a caer en ese hueco. Medido en el `Apk.<init>` de ApkShare, donde `String name`
// (`Apk.java:27` del zip) ocupa el hueco que deja `ApplicationInfo appInfo` (`Apk.java:19`) al cerrar
// el `if` de la línea 18 (`astore 4` de javac frente a nuestro `astore 5`).
type ambito struct {
	siguiente int
	// previos guarda, por cada nombre que este bloque volvió a declarar, lo que había antes: una
	// local exterior del mismo nombre o nada. Se devuelve en orden inverso al cerrar.
	previos []nombreAnterior
	// declarados son los huecos que este bloque declaró, con las dos mitades de un ancho: al cerrar
	// el bloque dejan de estar en ámbito y el marco los poda a `top` (ver `muerteAmbito`).
	declarados []int
	// almacena dice si alguno de esos huecos recibió un valor. javac solo abre el rango vivo del local
	// al guardarlo (`setDefined`), y `endScope` escribe la fila del cierre del bloque solo con un rango
	// abierto: sin local con valor no hay fila (ver `branca` y `jbloque.lineaCierre`).
	almacena bool
}

// muerteAmbito es el punto del código donde un bloque del fuente se cierra. javac marca `top` los
// huecos que declaró en el marco siguiente aunque las dos ramas de una unión traigan el MISMO tipo
// (medido con javac 17: un `IOException x` dentro de un `try` y el parámetro de su `catch` caen los
// dos en el hueco 2, y el marco del `goto` de salida los escribe `top`). Sin esa poda, la unión de
// dos referencias distintas daba `Object` donde javac escribe un hueco sin asignar: era la cuarta
// entrada del `StackMapTable` de `MainActivity$6`, `append Object` frente a `same`.
type muerteAmbito struct {
	pc      int
	indices []int
}

// nombreAnterior: lo que la tabla de locales decía de ese nombre antes de que el bloque lo declarara.
type nombreAnterior struct {
	nombre   string
	anterior localVar
	habia    bool
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

// inserta abre un hueco de `n` bytes en `en` y corre todo lo que viene detrás: etiquetas, parches
// pendientes, filas de línea y tabla de excepciones. Lo usa `resuelveSaltos` al ensanchar una rama.
func (e *emisor) inserta(en, n int) {
	e.codigo = append(e.codigo, make([]byte, n)...)
	copy(e.codigo[en+n:], e.codigo[en:len(e.codigo)-n])
	desplaza := func(v int) int {
		// `>=` y no `>`: un destino o un parche que empiece justo en el hueco se corre con él. La
		// vuelta de un bucle termina exactamente donde se cierra la etiqueta del fin, y dejarla sin
		// correr apuntaba dos bytes dentro del `goto_w`.
		if v >= en {
			return v + n
		}
		return v
	}
	for i := range e.parches {
		e.parches[i].en = desplaza(e.parches[i].en)
	}
	for i := range e.parches4 {
		e.parches4[i].en = desplaza(e.parches4[i].en)
		e.parches4[i].base = desplaza(e.parches4[i].base)
	}
	for k, v := range e.etiquetas {
		e.etiquetas[k] = desplaza(v)
	}
	for i := range e.lineas {
		e.lineas[i].desplazamiento = desplaza(e.lineas[i].desplazamiento)
	}
	for i := range e.excepciones {
		t := &e.excepciones[i]
		t.inicio, t.fin, t.handler = desplaza(t.inicio), desplaza(t.fin), desplaza(t.handler)
	}
	for i := range e.muertes {
		e.muertes[i].pc = desplaza(e.muertes[i].pc)
	}
}

// inversaCondicion: javac no alarga una branquia condicional; la invierte para que salte un goto_w
// corto y pone el salto larguísimo en ese goto (medido en §169: `ifgt 11; goto_w 48016`).
var inversaCondicion = map[byte]byte{
	0x99: 0x9a, 0x9a: 0x99, 0x9b: 0x9c, 0x9c: 0x9b, 0x9d: 0x9e, 0x9e: 0x9d,
	0x9f: 0xa0, 0xa0: 0x9f, 0xa1: 0xa2, 0xa2: 0xa1, 0xa3: 0xa4, 0xa4: 0xa3,
	0xa5: 0xa6, 0xa6: 0xa5, 0xc6: 0xc7, 0xc7: 0xc6,
}

// resuelveSaltos coloca la forma definitiva de cada salto y escribe su operando. Va después de
// emitir todo el método y antes de calcular el marco: mientras el cuerpo crece, cualquier rama
// puede quedarse corta, y ensanchar una corre las demás, así que los saltos que ya cabían también
// pueden pasarse de rosca. La pasada repite el ensanchado hasta que nadie cambia y solo entonces
// escribe los desplazamientos, ya sobre posiciones definitivas. Así la vuelta de un bucle a más de
// 32 KB (`goto_w` hacia atrás) y la rama que se ensancha delante del cuerpo no se pisan: antes,
// resolver una y ensanchar la otra dejaba el desplazamiento de la primera corto.
func (e *emisor) resuelveSaltos() error {
	for {
		cambiado := false
		for i := range e.parches {
			p := &e.parches[i]
			destino, ok := e.etiquetas[p.etiqueta]
			if !ok {
				return fmt.Errorf("un salto a una etiqueta que no se cerro")
			}
			if p.largo {
				continue
			}
			rel := destino - p.en
			if rel >= -32768 && rel <= 32767 {
				continue
			}
			if p.op == opGOTO {
				// `goto` de 3 bytes -> `goto_w` de 5, con el opcode en el mismo sitio
				e.inserta(p.en+3, 2)
				e.codigo[p.en] = 0xc8
			} else {
				inv, ok := inversaCondicion[p.op]
				if !ok {
					return fmt.Errorf("no se sabe invertir la condicion %#02x", p.op)
				}
				// la condición se invierte y salta por encima del `goto_w` que lleva el destino
				e.inserta(p.en+3, 5)
				e.codigo[p.en] = inv
				e.codigo[p.en+1], e.codigo[p.en+2] = 0, 8
				e.codigo[p.en+3] = 0xc8
			}
			p.largo = true
			cambiado = true
		}
		if !cambiado {
			break
		}
	}
	for i := range e.parches {
		p := &e.parches[i]
		destino := e.etiquetas[p.etiqueta]
		if p.largo {
			base := p.en + 1
			desde := p.en
			if p.op != opGOTO {
				// el `goto_w` vive detrás de la condición invertida: su desplazamiento se cuenta
				// desde el principio de ese `goto_w`, no desde la condición
				base, desde = p.en+4, p.en+3
			}
			rel := uint32(int32(destino - desde))
			e.codigo[base] = byte(rel >> 24)
			e.codigo[base+1] = byte(rel >> 16)
			e.codigo[base+2] = byte(rel >> 8)
			e.codigo[base+3] = byte(rel)
			continue
		}
		rel := destino - p.en
		e.codigo[p.en+1] = byte(rel >> 8)
		e.codigo[p.en+2] = byte(rel)
	}
	for i := range e.parches4 {
		p := &e.parches4[i]
		destino := e.etiquetas[p.etiqueta]
		rel := uint32(int32(destino - p.base))
		e.codigo[p.en] = byte(rel >> 24)
		e.codigo[p.en+1] = byte(rel >> 16)
		e.codigo[p.en+2] = byte(rel >> 8)
		e.codigo[p.en+3] = byte(rel)
	}
	return nil
}

// cierra anota la posicion actual como destino de la etiqueta. El operando de cada salto se
// completa más tarde, en `resuelveSaltos`, cuando ya se sabe si el método entero obliga a una forma
// larga en alguna rama.
func (e *emisor) cierra(id int) {
	if e.etiquetas == nil {
		e.etiquetas = map[int]int{}
	}
	e.etiquetas[id] = len(e.codigo)
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
//
// Solo cambia a qué etiqueta apunta el salto: los desplazamientos los escribe `resuelveSaltos`
// cuando ya están colocadas las formas largas.
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
	for i := range e.parches {
		p := &e.parches[i]
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
			p.etiqueta = siguiente
		}
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
	for i := range e.picos {
		if e.siguiente > e.picos[i] {
			e.picos[i] = e.siguiente
		}
	}
	if nombre != "" {
		e.apuntaNombre(nombre)
		e.locales[nombre] = localVar{indice: i, desc: desc}
		if len(e.ambitos) > 0 {
			// el hueco queda apuntado en el bloque que lo declara: al cerrarse, el marco lo poda.
			// Un ancho ocupa DOS huecos y los dos dejan de estar en ámbito a la vez (parrafo 74).
			a := &e.ambitos[len(e.ambitos)-1]
			a.declarados = append(a.declarados, i)
			if paso == 2 {
				a.declarados = append(a.declarados, i+1)
			}
		}
	}
	return i
}

// reservaConGenerico: `reserva` y, además, el tipo declarado del local con sus argumentos
// (`List<Apk>`). Solo lo usan los sitios donde el fuente lo dice de verdad: los parámetros del método,
// que lo traen en el AST, y las locales cuyo inicializador lo delata. Sin él, la vuelta de un método
// genérico encadenado sobre esa local no se puede estrechar y la ruta no estima.
func (e *emisor) reservaConGenerico(nombre, desc string, generico *tipoArgumento) int {
	i := e.reserva(nombre, desc)
	if propagaGenericoDeLocal {
		if lv, ok := e.locales[nombre]; ok {
			lv.generico = generico
			e.locales[nombre] = lv
		}
	}
	return i
}

// genericoDeLocal: el tipo declarado con argumentos de una local, deducido del valor que se le da. El
// analizador guarda el borrado (`List<Apk> apks` llega como `List`), así que los argumentos de tipo de
// una local solo quedan en su inicializador: `List<Apk> apks = app.apks` los trae en el campo. Si el
// valor no los da —o da otra clase— la local se queda sin tipo declarado conocido, y eso hace que la
// llamada encadenada sobre ella se rechace en vez de emitirse con el borrado.
func (e *emisor) genericoDeLocal(valor jexpr, desc string) *tipoArgumento {
	if valor == nil {
		return nil
	}
	_, generico, ok := e.tipoSinEmitirCompleto(valor)
	if !ok || generico == nil || len(generico.args) == 0 {
		return nil
	}
	propio, err := e.ctx.descriptorDe(generico.crudo())
	if err != nil || propio != desc {
		return nil
	}
	return generico
}

// apuntaNombre deja anotado, en el bloque abierto más interno, lo que la tabla de locales decía de ese
// nombre antes de declararlo aquí: al cerrar el bloque se devuelve, para que una local exterior del
// mismo nombre vuelva a verse y una declarada solo aquí desaparezca.
func (e *emisor) apuntaNombre(nombre string) {
	if len(e.ambitos) == 0 {
		return
	}
	a := &e.ambitos[len(e.ambitos)-1]
	for _, p := range a.previos {
		if p.nombre == nombre {
			// ya se declaró en este mismo bloque: lo que hay que devolver es lo de más afuera, que es
			// lo que ya está apuntado
			return
		}
	}
	anterior, habia := e.locales[nombre]
	a.previos = append(a.previos, nombreAnterior{nombre: nombre, anterior: anterior, habia: habia})
}

// abreAmbito marca el reparto de locales con el que empieza un bloque.
func (e *emisor) abreAmbito() {
	e.ambitos = append(e.ambitos, ambito{siguiente: e.siguiente})
}

// abrePico y cierraPico vigilan el alto que alcanza el reparto desde aquí aunque los bloques de
// dentro devuelvan su hueco. Lo necesita el manejador `any` de un `try` con `finally`: javac lo pone
// en el registro más alto que usó el cuerpo (ver `intenta`).
func (e *emisor) abrePico() { e.picos = append(e.picos, e.siguiente) }

// pico devuelve el alto vigilado más interno, que nunca baja.
func (e *emisor) pico() int {
	if len(e.picos) == 0 {
		return 0
	}
	return e.picos[len(e.picos)-1]
}

// cierraPico devuelve el alto vigilado y lo saca de la pila.
func (e *emisor) cierraPico() int {
	p := e.pico()
	if len(e.picos) > 0 {
		e.picos = e.picos[:len(e.picos)-1]
	}
	return p
}

// reutilizaHuecos es el interruptor de la falsificación: apagado, el bloque no devuelve su reparto de
// locales al cerrarse y el siguiente local del método vuelve a subir el contador, que es lo que hacía
// `reserva` antes. La prueba de la frontera mide con él las dos caras: el dex de las seis clases de
// ApkShare byte a byte con él encendido y distinto (y con `locals=6` en vez de 5) apagado.
var reutilizaHuecos = true

// anotaLineaDelElse es el interruptor de la falsificación de la fila del `else`: apagado, el `goto`
// que salta el `then` se queda sin la fila del `}` del bloque y el dex de `MainActivity` vuelve a
// separarse del de javac en esa fila (ver `branca` y `TestFalsificacionDeLaLineaDelElse`).
var anotaLineaDelElse = true

// almacenaEnBloque apunta que el hueco `indice` recibió un valor: javac abre ahí el rango vivo del
// local, y sin rango abierto el cierre del bloque no deja fila (ver `jbloque.lineaCierre`). Se marca el
// ámbito más interno que declaró ese hueco, que es el del bloque que lo declara: es el que `endScopes`
// cierra. Un hueco de un parámetro o de un local de fuera del bloque no está en `declarados` y no
// marca nada, como en javac.
func (e *emisor) almacenaEnBloque(indice int) {
	for i := len(e.ambitos) - 1; i >= 0; i-- {
		for _, d := range e.ambitos[i].declarados {
			if d == indice {
				e.ambitos[i].almacena = true
				return
			}
		}
	}
}

// ambitoAlmacena: el ámbito que se está cerrando declaró algún local con valor.
func (e *emisor) ambitoAlmacena() bool {
	if len(e.ambitos) == 0 {
		return false
	}
	return e.ambitos[len(e.ambitos)-1].almacena
}

// cierraAmbito devuelve el reparto y los nombres al punto en que el bloque se abrió. `maxLoc` no se
// toca: es el alto que alcanzó el método, y los marcos apilados lo necesitan tal cual.
func (e *emisor) cierraAmbito() {
	if len(e.ambitos) == 0 {
		return
	}
	a := e.ambitos[len(e.ambitos)-1]
	// la poda del marco no depende del reparto: javac marca `top` el hueco en cuanto el bloque se
	// cierra, aunque el interruptor de los huecos esté apagado (la falsificación mueve el reparto,
	// no el marco)
	if len(a.declarados) > 0 {
		e.muertes = append(e.muertes, muerteAmbito{pc: len(e.codigo), indices: a.declarados})
	}
	if !reutilizaHuecos {
		return
	}
	e.ambitos = e.ambitos[:len(e.ambitos)-1]
	e.siguiente = a.siguiente
	for i := len(a.previos) - 1; i >= 0; i-- {
		p := a.previos[i]
		if p.habia {
			e.locales[p.nombre] = p.anterior
			continue
		}
		delete(e.locales, p.nombre)
	}
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

// ldcSinFormaLarga es el interruptor de la falsificación del `ldc_w`. Apagado (el binario que se
// publica), el índice del pool decide la forma: hasta 255 `ldc` (0x12) y a partir de 256 `ldc_w`
// (0x13). Encendido, todo sale por `ldc` corto y el método se rechaza con «pasa de 255 entradas»,
// que es la regresión que se quiere poder ver. Lo usa `javaemisor_ldcw_escritor_test.go`.
var ldcSinFormaLarga = false

// ldc: carga una constante del pool con la forma que le toca por su índice. `ldc` (0x12) lleva el
// índice en un byte y llega hasta 255; a partir de 256 javac escribe `ldc_w` (0x13), con dos bytes
// (JVMS 6.5), y la instrucción crece uno. El índice ya es definitivo al emitir porque el pool solo
// crece por el final: ninguna entrada escrita se mueve. El byte de más corre las posiciones que se
// anotan después —filas de línea, etiquetas, `exception_table`—, y todas se toman de `len(e.codigo)`
// en el momento de emitir, así que quedan solas; los saltos se resuelven luego en `resuelveSaltos`.
// Un índice de más de 65535 tampoco cabe en `ldc_w`, y ahí el método se rechaza por nombre.
func (e *emisor) ldc(idx int) {
	if idx > 255 {
		if idx <= 0xffff && !ldcSinFormaLarga {
			e.emite(opLdcW, byte(idx>>8), byte(idx))
			return
		}
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
	if !e.cabeElInicializador(f.valor, d, desc) {
		return fmt.Errorf("linea %d: el campo %s es %s y su inicializador deja %s", f.donde.linea, f.nombre, desc, d)
	}
	if f.esEstatico {
		e.putstaticDe(idx, desc)
	} else {
		e.putfieldDe(idx, desc)
	}
	return nil
}

// cabeElInicializador: la política de asignación de JLS 5.2 al bajar el valor de un campo, que es la
// misma de `cabeEn` —identidad y subida de referencias, con el interruptor `cabeSubidaDeReferencia`
// que ya usa la escritura de campo— más el literal `null`. El `null` se comprueba aparte porque
// `exp` lo devuelve como `Ljava/lang/Object;`: en la pila es un `aconst_null` sin tipo propio y javac
// lo deja en cualquier hueco de referencia sin escribir nada (`List<App> l = null` es `aconst_null;
// putfield`), así que compararlo por descriptor contra el del campo lo rechazaría por ser más general.
// Sin esto, `private List<App> lista = new ArrayList<>();` daba «el campo lista es Ljava/util/List; y
// su inicializador deja Ljava/util/ArrayList;» aunque javac no escriba ninguna conversión.
func (e *emisor) cabeElInicializador(x jexpr, dv, hasta string) bool {
	if esLiteralNulo(x) && esTipoDeReferencia(hasta) {
		return true
	}
	return e.ctx.cabeEn(dv, hasta)
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
func lineaDeExpr(x jexpr) int { return posDeExpr(x).linea }

// posDeExpr: la posicion de una expresion. La columna solo viene cuando la expresion la abrio un
// token (`R.layout.main`, un literal, una llamada); las posiciones de sentencia salen con cero.
func posDeExpr(x jexpr) pos {
	switch v := x.(type) {
	case jlit:
		return v.donde
	case jclase:
		return v.donde
	case jthis:
		return v.donde
	case jacceso:
		return v.donde
	case jllamada:
		return v.donde
	case jcast:
		return v.donde
	case jinstanceof:
		return v.donde
	case jnuevo:
		return v.donde
	case jnuevoArreglo:
		return v.donde
	case jindice:
		return v.donde
	case jincr:
		return v.donde
	case jasignExp:
		return v.donde
	case joperador:
		return v.donde
	case jnegacion:
		return v.donde
	case jnegativo:
		return v.donde
	case jlambda:
		return v.donde
	}
	return pos{}
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
			// this$0:Lcom/hola/apkcstudio/MainActivity;` dentro de MainActivity$1. En una anonima
			// anidada el exterior nombrado puede estar dos saltos arriba y se emite la cadena entera.
			destino := ""
			for _, c := range exteriores(e.c) {
				interno := internoDe(c.paquete, c.nombre)
				if sinPaquete(interno) == v.nombre {
					destino = interno
					break
				}
			}
			if destino == "" {
				return "", fmt.Errorf("linea %d: `%s.this` solo se mide dentro de la anonima que ese exterior crea", v.donde.linea, v.nombre)
			}
			if err := e.cargaDelExteriorHasta(destino); err != nil {
				return "", fmt.Errorf("linea %d: al leer `%s.this`, %w", v.donde.linea, v.nombre, err)
			}
			return "L" + destino + ";", nil
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
		if destino, ok := descPrimitivo[v.tipo]; ok && strings.IndexByte("BCS", destino[0]) >= 0 {
			// `(char) x`, medido en `Letras` (parrafo 87): `resta` es `iload_0; iload_1; isub; i2c;
			// ireturn` y `cambia` `iload_0; iconst_1; iadd; i2c; istore_0`, con `i2c` = 0x92. Es una
			// palabra dentro y una fuera, asi que la pila no se mueve. Desde un ancho (`long`, `float`,
			// `double`) primero la conversion del parrafo 77 y despues el `i2c`.
			// Lo mismo para `i2b` 0x91 y `i2s` 0x93, medidos en `Estrecha` con la sonda javac 17:
			// `(byte) n` es `iload_0; i2b; ireturn`, `(short) n` `iload_0; i2s; ireturn`, y desde un
			// anchojavac escribe primero `l2i`/`d2i` y luego la conversion estrecha.
			op := map[byte]byte{'B': 0x91, 'C': 0x92, 'S': 0x93}[destino[0]]
			d, err := e.exp(v.dentro)
			if err != nil {
				return "", err
			}
			if d == destino {
				return destino, nil
			}
			if len(d) != 1 || strings.IndexByte("IJFD", d[0]) < 0 {
				return "", fmt.Errorf("linea %d: se convierte %s a `%s`, y eso no es una cuenta numerica medida", v.donde.linea, d, v.tipo)
			}
			if d != "I" {
				if !e.convierte(d, "I") {
					return "", fmt.Errorf("linea %d: la conversion de %s a entero antes de `%s` no esta medida", v.donde.linea, d, v.tipo)
				}
			}
			e.emite(op)
			return destino, nil
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
		return e.incrementoValor(v)
	case jasignExp:
		return e.asignacionValor(v)
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
		codes := map[string]byte{
			"+": opIadd, "-": opIsub, "*": opImul, "/": opIdiv, "%": opIrem,
			"<<": opIshl, ">>": opIshr, ">>>": opIushr, "&": opIand, "|": opIor, "^": opIxor,
		}
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
		if plegado, esConstante := e.plegarConstante(v); esConstante {
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
	case jnegativo:
		// la `-` de la aritmetica: el operando va primero y debajo suyo la negacion de su ancho,
		// **sin conversion previa** (medido con javac 17: `-total` es `iload_1; ineg`, `-(a+b)` es
		// `iload_0; iload_1; iadd; ineg` y `-f()` `invokestatic; ineg`; `lneg`, `fneg` y `dneg` salen
		// igual de `-x` en cada ancho). `char`, `byte` y `short` viajan como enteros de una palabra y
		// su negacion es `ineg` con resultado `int` (`-c` medido: `iload_0; ineg`).
		d, err := e.exp(v.dentro)
		if err != nil {
			return "", err
		}
		switch {
		case len(d) == 1 && strings.IndexByte("IBCS", d[0]) >= 0:
			e.emite(opIneg)
			return "I", nil
		case d == "J":
			e.emite(opLneg)
			return "J", nil
		case d == "F":
			e.emite(opFneg)
			return "F", nil
		case d == "D":
			e.emite(opDneg)
			return "D", nil
		}
		return "", fmt.Errorf("linea %d: la negacion de un %s no esta medida (solo los numeros)", v.donde.linea, d)
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

// pliegaBits es el interruptor de la falsificación: apagado, `&`, `|` y `^` de constantes vuelven a
// emitirse como la cuenta, y el constructor de `Apk` de ApkShare vuelve a salir con
// `iconst_1; bipush 128; ior` en vez del `sipush 129` de javac. Lo enciende el binario; solo lo apaga
// `TestFalsificacionDelPlegadoDeBits`.
var pliegaBits = true

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
//
// La familia de bits entra igual, con el ancho que da la promocion binaria: medido con javac 17
// `--release 8 -g:lines,source`, `5 | 2` → iconst_7, `5 & 6` → iconst_4, `5 ^ 3` → bipush 6,
// `1L | 128L` y `1 | 128L` → ldc2_w 129l (los dos, porque el `long` promociona al otro lado).
//
// Y no solo pliega el literal suelto: un `static final` de tipo entero nombrado por su tipo es una
// constante de compilacion para javac, que la sustituye por el valor y no deja en el `.class` ni el
// `getstatic` ni el Fieldref. Medido con el `Apk.java` real de ApkShare,
// `PackageManager.GET_ACTIVITIES | PackageManager.GET_META_DATA` (1 | 128) sale `const/16 v0, #int
// 129` en el dex de javac, sin `or-int/2addr`. La `Class` de las anidadas por las que pasa la cadena
// si queda en el pool aunque el valor se doble (medido: `R.layout.a` deja `p/R$layout` en el pool y
// en `InnerClasses`), asi que al plegar se anota igual que lo hace `accesoEstatico`.
func (e *emisor) plegarConstante(x jexpr) (constNum, bool) {
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
		case "+", "-", "*", "/", "%", "&", "|", "^":
		default:
			return constNum{}, false
		}
		a, ok := e.plegarConstante(v.izq)
		if !ok {
			return constNum{}, false
		}
		b, ok := e.plegarConstante(v.der)
		if !ok {
			return constNum{}, false
		}
		t := a.tipo
		if nivelNum(b.tipo) > nivelNum(t) {
			t = b.tipo
		}
		esDeBits := v.op == "&" || v.op == "|" || v.op == "^"
		if esDeBits && !pliegaBits {
			return constNum{}, false
		}
		if esDeBits && t != 'I' && t != 'J' {
			// `&`, `|` y `^` no existen para `float` ni `double`: un fuente asi no compila, y plegarlo
			// como si fuera una cuenta aritmetica inventaria un valor.
			return constNum{}, false
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
				case "&":
					r = a.entero & b.entero
				case "|":
					r = a.entero | b.entero
				case "^":
					r = a.entero ^ b.entero
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
			case "&":
				r = ia & ib
			case "|":
				r = ia | ib
			case "^":
				r = ia ^ ib
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
	case jnegativo:
		// javac también dobla la `-` sobre una cuenta de constantes, antes de emitir nada: medido con
		// javac 17, `-(1 + 2)` es `bipush -3`, `-(1L + 2L)` un `ldc2_w -3l`, y `-(1.0 + 2.0)` y
		// `-(1.5f + 2.5f)` las constantes negativas del pool. La negación es la del ancho de dentro
		// (un `char` pliega como entero, como en su suma); el desborde de `int` envuelve, igual que en
		// las cuentas.
		c, ok := e.plegarConstante(v.dentro)
		if !ok {
			return constNum{}, false
		}
		if c.tipo == 'I' {
			// la negación de `int` envuelve como la de la JVM: `-(-2147483648)` sigue siendo el mismo
			// entero (medido: javac dobla la constante y no protesta)
			c.entero = int64(-int32(c.entero))
		} else if c.tipo == 'J' {
			c.entero = -c.entero
		} else {
			c.real = -c.real
		}
		return c, true
	case jacceso:
		// la constante de compilación nombrada por su tipo (`PackageManager.GET_META_DATA`,
		// `R.layout.a`). Solo vale sin receptor —`esto.CAMPO` es una lectura de campo— y solo si el
		// campo es un `static final` entero con valor, que es lo que `buscarCampo` trae ya doblado.
		segs, base := aplanar(v)
		if base != nil || len(segs) == 0 {
			return constNum{}, false
		}
		// una constante suelta (`AutoResearchDelay`): su nombre no es el de un tipo, así que el camino
		// de abajo no la resuelve. javac la dobla sobre el hueco al que va —medido con javac 17.0.21
		// `--release 8`, `long x = INT500` es `ldc2_w 500l` y no `sipush 500; i2l`—, y aquí tiene que
		// doblarse igual para que `expA` escriba la constante ya promovida. Una local del método o una
		// captura `val$x` con el mismo nombre tapan al campo y no pliegan.
		if len(segs) == 1 {
			if _, esLocal := e.locales[segs[0]]; !esLocal && !e.tapaLaCaptura(segs[0]) {
				if _, cr, encontrado := e.duenioDelCampo(segs[0]); encontrado &&
					cr.constante && !cr.esCadena && cr.acceso&accStatic != 0 && esConstanteEntera(cr.desc) {
					if pliegaCampoSuelto {
						return constNum{tipo: 'I', entero: int64(cr.valor)}, true
					}
				}
			}
		}
		interno, resto, ok := e.ctx.tipoDeReferencia(segs)
		if !ok || len(resto) == 0 {
			return constNum{}, false
		}
		// las anidadas de la cadena se comprueban sin tocar el pool: si al final no es una constante,
		// el camino de siempre (`accesoEstatico`) las anota igual y no queremos dos anotaciones por
		// una expresión que se va a emitir como acceso.
		tipo := interno
		for _, seg := range resto[:len(resto)-1] {
			anidada := tipo + "$" + seg
			if !e.ctx.hay(anidada) {
				return constNum{}, false
			}
			tipo = anidada
		}
		cr, err := e.ctx.buscarCampo(tipo, resto[len(resto)-1])
		// una constante de `String` no pliega como número: se emite por `accesoEstatico`, que la
		// escribe como `ldc` del literal
		if err != nil || !cr.constante || cr.esCadena || cr.acceso&accStatic == 0 {
			return constNum{}, false
		}
		// el pool se anota solo al final, cuando el plegado es seguro: es lo mismo que hace
		// `accesoEstatico` con cada anidada por la que pasa la cadena (medido: `R.layout.a` deja
		// `p/R$layout` en el pool y en `InnerClasses` aunque el valor se doble).
		tipo = interno
		for _, seg := range resto[:len(resto)-1] {
			tipo += "$" + seg
			e.usar(tipo)
		}
		return constNum{tipo: 'I', entero: int64(cr.valor)}, true
	}
	return constNum{}, false
}

// pliegaCampoSuelto es el interruptor de la falsificación del plegado de una constante suelta. Apagado,
// `postDelayed(r, AutoResearchDelay)` vuelve a escribir `sipush 500; i2l` donde javac 17 escribe
// `ldc2_w 500l`, que es la divergencia que mide `javaemisor_ldc2w_campo_test.go`. Encendido en el
// binario que se publica.
var pliegaCampoSuelto = true

// esConstanteEntera: los descriptores de un `static final` que javac dobla como una constante entera.
// `Z` queda fuera: `long x = BOOL` no es Java y plegarlo como entero lo aceptaría (JLS 5.1.2).
func esConstanteEntera(desc string) bool {
	return len(desc) == 1 && strings.IndexByte("IBCS", desc[0]) >= 0
}

// tapaLaCaptura: la clase actual, o alguno de sus exteriores, declara el campo `val$nombre`, así que
// el nombre suelto es una local capturada y no una constante de la clase. Sin esta comprobación, una
// local que se llamara como un `static final` del exterior se doblaría con el valor del campo.
func (e *emisor) tapaLaCaptura(nombre string) bool {
	for c := e.c; c != nil; {
		for _, f := range c.campos {
			if f.nombre == "val$"+nombre {
				return true
			}
		}
		if !tieneExterior(c) {
			break
		}
		c = c.encierre
	}
	return false
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
	donde := a.donde
	if len(segs) == 0 {
		return "", fmt.Errorf("%s: acceso sin nombre", donde.sitio())
	}
	if base != nil {
		d, err := e.exp(base)
		if err != nil {
			return "", err
		}
		return e.cadenaDeCampos(d, segs, donde)
	}
	if v, ok := e.locales[segs[0]]; ok {
		e.cargaDe(v.indice, v.desc)
		return e.cadenaDeCampos(v.desc, segs[1:], donde)
	}
	// una local capturada por una anonima que encierra a esta: `adapter` dentro de `MainActivity$2$1`
	// no es una local del `run()` de `MainActivity$2` ni un campo escrito en el fuente —el `val$adapter`
	// lo puso javac en la anonima exterior—, asi que javac la lee por la cadena `this$N` hasta ese
	// campo: `aload_0; getfield this$1:LMainActivity$2; getfield val$adapter:LAppAdapter;` (medido con
	// javac 17 --release 8 sobre ApkShare). Va antes que el campo de la clase actual porque el nombre
	// del fuente es el de la local, no el del campo sintetico.
	if dueño, cr, ok := e.capturaDeExterior(segs[0]); ok {
		if err := e.cargaDelExteriorHasta(dueño); err != nil {
			return "", fmt.Errorf("%s: al leer %s, %w", donde.sitio(), strings.Join(segs, "."), err)
		}
		e.getfieldDe(e.pool.campo(dueño, "val$"+segs[0], cr.desc), cr.desc)
		return e.cadenaDeCampos(cr.desc, segs[1:], donde)
	}
	if len(segs) == 1 {
		d, ok, err := e.campoDeLaClaseActual(segs[0], donde.linea)
		if err != nil {
			return "", fmt.Errorf("%s: al leer %s, %w", donde.sitio(), segs[0], err)
		}
		if ok {
			return d, nil
		}
	}
	// `com.x.dos.Helper.CAMPO` y `Outer.Inner.CAMPO`: el tipo es el prefijo más largo que resuelve, no
	// el primer segmento. `R.layout.main` también entra aquí y sale con `R$layout` + `main`, que es el
	// mismo acceso estático que daba antes por el camino de `resolverSimple("R")`.
	if interno, resto, ok := e.ctx.tipoDeReferencia(segs); ok {
		return e.accesoEstatico(interno, resto, donde)
	}
	// el error tiene que nombrar la referencia entera, no su primer segmento
	_, err := e.ctx.tipoAInterno(strings.Join(segs, "."))
	return "", fmt.Errorf("%s: al leer %s, %w", donde.sitio(), strings.Join(segs, "."), err)
}

// cadenaDeCampos: campos de instancia sobre el valor que ya esta en la pila.
func (e *emisor) cadenaDeCampos(desc string, segs []string, donde pos) (string, error) {
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
			return "", fmt.Errorf("%s: %s no es un objeto, no se le puede sacar %s", donde.sitio(), actual, seg)
		}
		dueño := actual[1 : len(actual)-1]
		cr, err := e.ctx.buscarCampo(dueño, seg)
		if err != nil {
			return "", fmt.Errorf("%s: %w", donde.sitio(), err)
		}
		if cr.constante {
			return "", fmt.Errorf("%s: %s es una constante y sacarla de una instancia no esta medido", donde.sitio(), seg)
		}
		if cr.acceso&accStatic != 0 {
			return "", fmt.Errorf("%s: %s.%s es estatico: sacarlo de una instancia no esta medido", donde.sitio(), dueño, seg)
		}
		idx := e.pool.campo(dueño, seg, cr.desc)
		e.getfieldDe(idx, cr.desc)
		actual = cr.desc
	}
	return actual, nil
}

// escribeCampoDe: `appInfo.sourceDir = valor`, el campo escrito a través de un receptor que no es
// `this`: una local, un parámetro o un campo —también el que captura una anónima—, todos con tipo
// conocido. El receptor se emite primero con el mismo camino que la lectura (`acceso`), y el campo se
// busca en el tipo que ese camino devuelve, que es el que javac pone en el Fieldref: medido con javac
// 17 en una sonda propia, `Sub s; s.n = v` con `n` declarado en `Base` sale `putfield
// com/hola/apkcstudio/Sub.n:I`, y `b.n = v` con `b` de tipo `Base` sale contra `Base`. La fila de la
// sentencia la marca el `jasign` antes de llamar aquí y el receptor no añade ninguna: la
// LineNumberTable no se mueve.
func (e *emisor) escribeCampoDe(segs []string, base jexpr, donde pos, valor jexpr) error {
	nombre := strings.Join(segs, ".")
	campo := segs[len(segs)-1]
	if base == nil && len(segs) > 1 {
		// el receptor empieza por un nombre suelto: una local o un parámetro, un campo de la clase (o del
		// exterior que captura la anónima), o el nombre de un tipo. Si no es ninguna de las tres cosas no
		// se adivina su tipo.
		if _, ok := e.locales[segs[0]]; !ok {
			_, cr, esCampo := e.duenioDelCampo(segs[0])
			if esCampo {
				if cr.acceso&accStatic == 0 && e.estatico {
					return fmt.Errorf("linea %d: asignar a %s: %s es un campo de instancia y el metodo es estatico, sin `this` no es Java",
						donde.linea, nombre, segs[0])
				}
			} else if _, err := e.ctx.resolverSimple(segs[0]); err == nil {
				// `MainActivity.cuenta = v`: el receptor sería el nombre del tipo y el campo el estático
				// que javac escribe con `putstatic`; esa forma no está medida aquí.
				return fmt.Errorf("linea %d: asignar a %s por el nombre del tipo no esta medido", donde.linea, nombre)
			} else {
				return fmt.Errorf("linea %d: al asignar a %s: no se conoce el tipo del receptor %s", donde.linea, nombre, segs[0])
			}
		}
	}
	var dr string
	var err error
	if len(segs) > 1 {
		receptor := jacceso{base: base, nombre: segs[0], donde: donde}
		for _, seg := range segs[1 : len(segs)-1] {
			receptor = jacceso{base: receptor, nombre: seg, donde: donde}
		}
		dr, err = e.acceso(receptor)
		if err != nil {
			return fmt.Errorf("linea %d: al asignar a %s: no se conoce el tipo del receptor %s (%v)",
				donde.linea, nombre, strings.Join(segs[:len(segs)-1], "."), err)
		}
	} else {
		// `this.datos.campo = v` y `getDatos().campo = v`: el receptor es la base tal cual
		dr, err = e.exp(base)
		if err != nil {
			return fmt.Errorf("linea %d: al asignar a %s: no se conoce el tipo del receptor (%v)", donde.linea, nombre, err)
		}
	}
	if len(dr) == 0 || dr[0] != 'L' {
		return fmt.Errorf("linea %d: al asignar a %s: el receptor es %s y no un objeto", donde.linea, nombre, dr)
	}
	dueño := dr[1 : len(dr)-1]
	cr, err := e.ctx.buscarCampo(dueño, campo)
	if err != nil {
		return fmt.Errorf("linea %d: al asignar a %s: %w", donde.linea, nombre, err)
	}
	if cr.constante {
		return fmt.Errorf("linea %d: %s es una constante final y javac no la deja reasignar", donde.linea, campo)
	}
	if cr.acceso&accStatic != 0 {
		// `appInfo.CUENTA = v` con un campo estático: javac emite `aload; pop; putstatic`, y esa forma
		// todavía no está medida aquí
		return fmt.Errorf("linea %d: asignar a %s: %s es un campo estatico y escribirlo por una instancia no esta medido",
			donde.linea, nombre, campo)
	}
	dv, err := e.expA(valor, cr.desc)
	if err != nil {
		e.baja(1) // el receptor que ya estaba contado
		return err
	}
	if !e.ctx.cabeEn(dv, cr.desc) {
		e.baja(1)
		return fmt.Errorf("linea %d: %s.%s es %s y se le asigna un %s", donde.linea, dueño, campo, cr.desc, dv)
	}
	e.putfieldDe(e.pool.campo(dueño, campo, cr.desc), cr.desc)
	return nil
}

// accesoEstatico: `R.layout.main` desde el nombre del tipo. Las anidadas intermedias no emiten nada; el
// ultimo segmento es un campo estatico o una constante doblada.
func (e *emisor) accesoEstatico(interno string, segs []string, donde pos) (string, error) {
	if len(segs) == 0 {
		return "", fmt.Errorf("%s: el tipo %s suelto no es un valor", donde.sitio(), interno)
	}
	tipo := interno
	for i, seg := range segs {
		ultimo := i == len(segs)-1
		if anidada := tipo + "$" + seg; e.ctx.hay(anidada) {
			if ultimo {
				return "", fmt.Errorf("%s: %s es un tipo, no un valor", donde.sitio(), seg)
			}
			// javac deja la Class de la anidada en el pool aunque el valor se doble despues: medido en el
			// parrafo 40 (#11 = Class com/hola/j1g/R$layout en MainActivity, que no usa ese tipo en codigo)
			e.usar(anidada)
			tipo = anidada
			continue
		}
		cr, err := e.ctx.buscarCampo(tipo, seg)
		if err != nil {
			return "", fmt.Errorf("%s: %w", donde.sitio(), err)
		}
		if !ultimo {
			return "", fmt.Errorf("%s: tras el campo %s no hay nada medido", donde.sitio(), seg)
		}
		if cr.constante {
			if cr.esCadena {
				// `Intent.ACTION_SEND`: javac escribe `ldc` de la cadena y no `getstatic` del campo
				e.ldc(e.pool.cadena(cr.cadena))
				e.sube()
				return cr.desc, nil
			}
			e.cargaEntero(cr.valor)
			return "I", nil
		}
		if cr.acceso&accStatic == 0 {
			return "", fmt.Errorf("%s: %s.%s no es estatico y no se puede nombrar por el tipo", donde.sitio(), tipo, seg)
		}
		idx := e.pool.campo(tipo, seg, cr.desc)
		e.getstaticDe(idx, cr.desc)
		return cr.desc, nil
	}
	return "", fmt.Errorf("%s: no se resuelve %s.%s", donde.sitio(), interno, strings.Join(segs, "."))
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
		return e.accesoEstatico(envoltorio, []string{"TYPE"}, v.donde)
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

// capturaDeLocalEnAnonimaAnidada es el interruptor de la sonda de la captura a traves de una anonima
// anidada: apagado, `adapter` dentro de `MainActivity$2$1` se resuelve como un nombre de tipo y el
// emisor se para con el error literal del usuario. Lo usa la falsificacion de
// `javaemisor_captura_anidada_test.go`; encendido en el binario que se publica.
var capturaDeLocalEnAnonimaAnidada = true

// capturaDeExterior: `nombre` es una local que capturo una anonima que encierra a esta y devuelve la
// clase que la guarda en su campo `val$nombre`, con el campo. Es la ruta de la anonima anidada: la
// local no esta en los locales del metodo que crea la anonima interior —el de la exterior— sino en los
// campos `val$x` de la exterior, que javac alcanza por la cadena `this$N`. Se mira primero la clase
// actual (su propio `val$x`, que el fuente nombra igual) y despues los exteriores de dentro afuera.
//
// Puede pasar que la anonima exterior todavia no tenga el `val$x`: javac lo crea cuando la local se
// nombra en cualquier punto de su cuerpo, tambien dentro de una anonima interior, y el analizador solo
// lo daba de alta en la anonima donde se ve la referencia. Por eso, si la local esta en los locales del
// metodo que creo esa anonima exterior —una exterior, no la clase actual: ahi la captura la registra el
// analizador—, aqui se le da de alta antes de leerla (`registraCapturaEn`).
func (e *emisor) capturaDeExterior(nombre string) (string, campoResuelto, bool) {
	if !capturaDeLocalEnAnonimaAnidada {
		return "", campoResuelto{}, false
	}
	campo := "val$" + nombre
	for c := e.c; c != nil; {
		interno := internoDe(c.paquete, c.nombre)
		if cr, err := e.ctx.buscarCampo(interno, campo); err == nil {
			return interno, cr, true
		}
		if c != e.c && c.anonima && c.metodoEncierre != nil && !tieneCampoPropio(c, nombre) {
			if tipo, ok := c.metodoEncierre.locales[nombre]; ok {
				registraCapturaEn(c, nombre, tipo)
				if cr, err := e.ctx.buscarCampo(interno, campo); err == nil {
					return interno, cr, true
				}
			}
		}
		if !tieneExterior(c) {
			return "", campoResuelto{}, false
		}
		c = c.encierre
	}
	return "", campoResuelto{}, false
}

// tieneCampoPropio: la clase ya declara un campo con ese nombre del fuente o con su `val$`, asi que el
// nombre suelto se resuelve en ella y no hay captura que bajar.
func tieneCampoPropio(c *jclas, nombre string) bool {
	for _, f := range c.campos {
		if f.nombre == nombre || f.nombre == "val$"+nombre {
			return true
		}
	}
	return false
}

// registraCapturaEn: da de alta en `c` la captura de la local `nombre` del metodo que la creo, con su
// campo `val$nombre`, igual que hace el analizador con `registraCaptura` cuando ve la referencia.
// Hace falta aqui porque la unica referencia puede vivir en una anonima interior, que se emite antes:
// javac baja `val$adapter` a MainActivity$2 aunque `adapter` solo se nombre en MainActivity$2$1.
// Límite conocido: el campo se anade al final de los `val$` ya registrados, asi que si en la anonima
// exterior una captura posterior en el fuente se registro antes, el orden de los campos no seria el de
// javac; el oraculo del dex lo destaparia. En las capturas de ApkShare el orden coincide.
func registraCapturaEn(c *jclas, nombre, tipo string) {
	for _, cap := range c.capturas {
		if cap.nombre == nombre {
			return
		}
	}
	c.capturas = append(c.capturas, captura{nombre: nombre, tipo: tipo})
	c.campos = append(c.campos, jcampo{
		nombre: "val$" + nombre, tipo: tipo, esFinal: true, esSintetico: true,
		donde: pos{linea: c.linea},
	})
}

// duenioDelCampo: donde vive un nombre suelto de asignacion o de lectura. Primero la clase actual; y
// dentro de una anonima, si no es propio, el exterior: medido en el parrafo 54 sobre el contador, cuyo
// `onClick` escribe `pulsos = pulsos + 1` y `total = total + pulsos` en campos de la Activity que creó
// la anonima. Recorre la cadena entera de exteriores, no solo el inmediato: una anonima anidada puede
// leer un campo de la clase de dos saltos arriba.
func (e *emisor) duenioDelCampo(nombre string) (string, campoResuelto, bool) {
	if cr, err := e.ctx.buscarCampo(e.interno, nombre); err == nil {
		return e.interno, cr, true
	}
	for _, ext := range clasesEnAlcance(e.c) {
		interno := internoDe(ext.paquete, ext.nombre)
		if cr, err := e.ctx.buscarCampo(interno, nombre); err == nil {
			// si el campo del exterior es privado, javac no escribe el getfield contra el campo sino contra
			// sus metodos sinteticos `access$NNN` (parrafo 72: `access$000` para leer, `access$002` para
			// escribir, y el de escritura devuelve el valor, de ahi el `pop`); los dos llamadores lo
			// distinguen por el flag y pasan por el accesor
			return interno, cr, true
		}
	}
	return "", campoResuelto{}, false
}

// cargaDelDuenio: el receptor de un campo de instancia. En la clase actual es `aload_0`; dentro de una
// anonima, si el campo es del exterior, javac pasa por la captura: `aload_0; getfield this$0:LExterior;`,
// y en una anonima anidada por la cadena entera (`this$1` y luego `this$0`). Medido dos veces en la
// misma linea del contador (`pulsos = pulsos + 1`), una por el receptor del `putfield` y otra por la
// lectura del valor: javac no duplica el receptor con `dup`, lo vuelve a cargar.
func (e *emisor) cargaDelDuenio(dueño string) {
	// el dueño salio de `duenioDelCampo`, que ya comprobo que se alcanza; el `aload_0` suelto es la
	// clase actual, que es lo unico que quedo medido antes de la cadena
	if err := e.cargaDelExteriorHasta(dueño); err != nil {
		e.cargaDe(0, "L")
	}
}

// campoDeLaClaseActual: `menu` suelto cuando fuera un campo de esta clase o de un padre suyo. Devuelve
// el descriptor del valor que dejó en la pila. Un `static final String` con `ConstantValue` heredado de
// un padre del classpath (`Hijo extends Base` y `Base.V`) se dobla como `ldc` del literal, igual que el
// cualificado; nunca como entero con descriptor de referencia.
func (e *emisor) campoDeLaClaseActual(nombre string, linea int) (string, bool, error) {
	dueño, cr, encontrado := e.duenioDelCampo(nombre)
	if !encontrado {
		return "", false, nil
	}
	if cr.constante {
		if cr.esCadena {
			if !pliegaCadenaHeredada {
				// la falsificación: el defecto anterior, un entero con descriptor de referencia
				e.cargaEntero(cr.valor)
				return cr.desc, true, nil
			}
			e.ldc(e.pool.cadena(cr.cadena))
			e.sube()
			return cr.desc, true, nil
		}
		switch cr.desc {
		case "I", "C", "S", "B", "Z":
			e.cargaEntero(cr.valor)
			return cr.desc, true, nil
		}
		// una constante que no viaja como entero no tiene forma de plegarse por esta ruta: emitir un
		// `iconst` con el descriptor de referencia produce bytecode inválido, así que se rechaza
		return "", false, fmt.Errorf("la constante %s no se puede doblar con el descriptor %s", nombre, cr.desc)
	}
	estatico := cr.acceso&accStatic != 0
	if dueño != e.interno && cr.acceso&accPrivate != 0 {
		// el campo privado del exterior: la anonima pasa por `access$NN0` (parrafo 72), y el accesor de
		// un campo ancho es `getfield` + `lreturn`/`dreturn`, sin nada mas (párrafo 83)
		nombreAcc, descAcc := e.ctx.accesorPara(dueño, nombre, cr.desc, true, false, estatico)
		enPila := 0
		if !estatico {
			if e.estatico {
				return "", false, nil
			}
			e.cargaDelDuenio(dueño)
			enPila = 1
		}
		e.invoca(opInvokestat, dueño, nombreAcc, descAcc, enPila)
		return cr.desc, true, nil
	}
	if estatico {
		idx := e.pool.campo(dueño, nombre, cr.desc)
		e.getstaticDe(idx, cr.desc)
		return cr.desc, true, nil
	}
	if e.estatico {
		return "", false, nil
	}
	e.cargaDelDuenio(dueño)
	e.getfieldDe(e.pool.campo(dueño, nombre, cr.desc), cr.desc)
	return cr.desc, true, nil
}

// llamada: los cuatro casos del subconjunto, con el Methodref en el sitio que pone javac (parrafo 40).
func (e *emisor) llamada(l jllamada) (string, error) {
	argc := len(l.args)
	if l.constructorPropio {
		// `this(...)`: javac escribe `aload_0; argumentos; invokespecial <init>` contra la MISMA
		// clase (parrafo 91). La delegacion no baja los inicializadores de campo; eso lo hace el
		// constructor al que se delega.
		e.cargaDe(0, "L")
		if e.hayEmpaquetadoEn(e.interno, "<init>", l.args) || e.necesitaConversionEn(e.interno, "<init>", l.args) {
			// un `this(...)` con un constructor varargs: la aridad del fuente no es la del descriptor,
			// asi que se resuelve con los tipos estimados antes de bajar los argumentos (§194). El
			// ensanchamiento entra por la misma puerta: `S(long)` llamado con `this(n)` es
			// `aload_0; iload_1; i2l; invokespecial` y el `int` ocupa una palabra y no dos.
			cand, n, err := e.argumentosResueltos(e.interno, "<init>", l.args)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
			}
			e.invoca(opInvokespec, e.interno, "<init>", cand.desc, n+1)
			return "V", nil
		}
		tipos, err := e.argumentos(l.args)
		if err != nil {
			return "", err
		}
		cand, err := e.ctx.buscarMetodo(e.interno, "<init>", argc, tipos)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		if err := revisaAnchosDeArgumentos(tipos, cand.desc, l.donde.linea); err != nil {
			return "", err
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
		if e.hayEmpaquetadoEn(dueño, l.nombre, l.args) || e.necesitaConversionEn(dueño, l.nombre, l.args) {
			// `super(...)` con un constructor varargs del padre: mismo orden que `this(...)`, y por la
			// misma cuenta el ensanchamiento de un argumento también se resuelve antes de bajarlo
			cand, n, err := e.argumentosResueltos(dueño, l.nombre, l.args)
			if err != nil {
				return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
			}
			e.invoca(opInvokespec, dueño, l.nombre, cand.desc, n+1)
			return vueltaDe(cand.desc), nil
		}
		tipos, err := e.argumentos(l.args)
		if err != nil {
			return "", err
		}
		cand, err := e.ctx.buscarMetodo(dueño, l.nombre, argc, tipos)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		if err := revisaAnchosDeArgumentos(tipos, cand.desc, l.donde.linea); err != nil {
			return "", err
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
			n, err := e.argumentosDeLlamada(l.args, cand)
			if err != nil {
				return "", err
			}
			e.invoca(opInvokestat, e.interno, l.nombre, cand.desc, n)
			return vueltaDe(cand.desc), nil
		}
		// Una anonima resuelve el nombre suelto primero en si misma y, si no hay ninguna candidata, en la
		// clase que la creo (parrafo 72): un metodo no privado del exterior se llama con la captura
		// (`aload_0; getfield this$0; invokevirtual`) y uno privado por su `access$NN0`, que es un
		// `invokestatic` con la captura como primer argumento. Un estatico del exterior no lleva receptor.
		// La busqueda sube por la cadena entera de exteriores: dentro de MainActivity$2$1 el
		// `hide_loading_dialog()` es de la Activity, dos saltos arriba, y javac emite `aload_0; getfield
		// this$1:LMainActivity$2; getfield this$0:LMainActivity; invokevirtual` (medido con javac 17
		// --release 8 sobre ApkShare).
		if e.c.anonima || e.esInnerNoEstatica() {
			cands, err := e.ctx.candidatos(e.interno, l.nombre, argc)
			if err == nil && len(cands) == 0 {
				tipos := make([]string, 0, argc)
				for _, arg := range l.args {
					d, err := e.tipoEstimado(arg)
					if err != nil {
						return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
					}
					tipos = append(tipos, d)
				}
				externo := ""
				var cand candidato
				var primero error
				for _, c := range clasesEnAlcance(e.c) {
					interno := internoDe(c.paquete, c.nombre)
					encontrado, err := e.ctx.buscarMetodo(interno, l.nombre, argc, tipos)
					if err != nil {
						if primero == nil {
							primero = err
						}
						continue
					}
					cand, externo = encontrado, interno
					break
				}
				if externo == "" {
					return "", fmt.Errorf("linea %d: no existe %s con %d argumentos en %s ni en el exterior %s: %w",
						l.donde.linea, l.nombre, argc, e.interno, e.encierreInterno(), primero)
				}
				if cand.acceso&accStatic != 0 {
					n, err := e.argumentosDeLlamada(l.args, cand)
					if err != nil {
						return "", err
					}
					if cand.acceso&accPrivate != 0 {
						// tambien un estatico privado pasa por su accessor, que no lleva receptor
						nombreAcc, descAcc := e.ctx.accesorPara(externo, l.nombre, cand.desc, false, false, true)
						e.invoca(opInvokestat, externo, nombreAcc, descAcc, n)
					} else {
						e.invoca(opInvokestat, externo, l.nombre, cand.desc, n)
					}
					return vueltaDe(cand.desc), nil
				}
				if err := e.cargaDelExteriorHasta(externo); err != nil {
					return "", fmt.Errorf("linea %d: al llamar a %s, %w", l.donde.linea, l.nombre, err)
				}
				n, err := e.argumentosDeLlamada(l.args, cand)
				if err != nil {
					return "", err
				}
				if cand.acceso&accPrivate != 0 {
					nombreAcc, descAcc := e.ctx.accesorPara(externo, l.nombre, cand.desc, false, false, false)
					e.invoca(opInvokestat, externo, nombreAcc, descAcc, n+1)
				} else {
					e.invoca(opInvokevirt, externo, l.nombre, cand.desc, n+1)
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
			n, err := e.argumentosDeLlamada(l.args, cand)
			if err != nil {
				return "", err
			}
			e.invoca(opInvokestat, e.interno, l.nombre, cand.desc, n)
			return vueltaDe(cand.desc), nil
		}
		e.cargaDe(0, "L")
		n, err := e.argumentosDeLlamada(l.args, cand)
		if err != nil {
			return "", err
		}
		opcode := byte(opInvokevirt)
		if cand.acceso&accPrivate != 0 {
			// un metodo privado de la propia clase se invoca con `invokespecial`, medido en el parrafo 72
			opcode = opInvokespec
		}
		e.invoca(opcode, e.interno, l.nombre, cand.desc, n+1)
		return vueltaDe(cand.desc), nil
	}
	// una llamada estática sobre un nombre de tipo, simple (`Helper.doble`) o cualificado
	// (`com.x.dos.Helper.doble`): el tipo es el prefijo más largo que resuelve y no puede quedar nada
	// detrás, porque lo que sigue es el nombre del método. Si queda un campo (`Helper.instancia.m()`),
	// el receptor es una expresión y se emite por el camino normal.
	if acc, esAcceso := l.base.(jacceso); esAcceso {
		segs, base := aplanar(acc)
		if base == nil && len(segs) > 0 && e.locales[segs[0]].desc == "" {
			interno, resto, ok := e.ctx.tipoDeReferencia(segs)
			if ok && len(resto) == 0 {
				// el mismo orden que en el receptor de instancia: con un primitivo que cae en un
				// parametro de referencia hay que estimar y boxear antes de invocar (§194)
				if e.hayEmpaquetadoEn(interno, l.nombre, l.args) || e.necesitaConversionEn(interno, l.nombre, l.args) {
					cand, n, err := e.argumentosResueltos(interno, l.nombre, l.args)
					if err != nil {
						return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
					}
					if cand.acceso&accStatic == 0 {
						return "", fmt.Errorf("linea %d: %s.%s no es estatico y no se puede llamar sobre el tipo", l.donde.linea, interno, l.nombre)
					}
					e.invoca(opInvokestat, interno, l.nombre, cand.desc, n)
					return vueltaDe(cand.desc), nil
				}
				tipos, err := e.argumentos(l.args)
				if err != nil {
					return "", err
				}
				cand, err := e.ctx.buscarMetodo(interno, l.nombre, argc, tipos)
				if err != nil {
					return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
				}
				if err := revisaAnchosDeArgumentos(tipos, cand.desc, l.donde.linea); err != nil {
					return "", err
				}
				if cand.acceso&accStatic == 0 {
					return "", fmt.Errorf("linea %d: %s.%s no es estatico y no se puede llamar sobre el tipo", l.donde.linea, interno, l.nombre)
				}
				e.invoca(opInvokestat, interno, l.nombre, cand.desc, argc)
				return vueltaDe(cand.desc), nil
			}
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
	var cand candidato
	n := argc
	if e.hayEmpaquetadoEn(receptor, l.nombre, l.args) || e.necesitaConversionEn(receptor, l.nombre, l.args) {
		// una candidata varargs del receptor, o una de aridad fija con un parametro de referencia
		// donde el fuente pone un primitivo: los tipos hay que estimarlos antes, porque la aridad
		// del `invoke` la fija el descriptor y no el fuente, y porque el boxing (`Integer.valueOf`)
		// se escribe entre el valor y la llamada. Bajar primero el literal deja un `int` que el
		// descriptor rechaza. El receptor va delante en los dos caminos (§194)
		cand, n, err = e.argumentosResueltos(receptor, l.nombre, l.args)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
	} else {
		tipos, err := e.argumentos(l.args)
		if err != nil {
			return "", err
		}
		cand, err = e.ctx.buscarMetodo(receptor, l.nombre, argc, tipos)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
		}
		if err := revisaAnchosDeArgumentos(tipos, cand.desc, l.donde.linea); err != nil {
			return "", err
		}
	}
	accesosReceptor, err := e.ctx.accesosDe(receptor)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	var opcode byte = opInvokevirt
	dueño := receptor
	if accesosReceptor&accInterface != 0 {
		opcode = opInvointf
		// Un método que declara `java/lang/Object` y se llama sobre un receptor de tipo interfaz es
		// `invokevirtual java/lang/Object.x`, no `invokeinterface`: javac solo se queda en la
		// interfaz cuando el tipo estático del receptor declara el método **él mismo**. Lo que lo
		// hereda de una superinterfaz sigue yendo a `Object`: medido con javac 17 `--release 8` sobre
		// `CharSequence` (declara `toString` y sale `invokeinterface CharSequence.toString`) frente a
		// `Editable` (lo hereda de `CharSequence` y sale `invokevirtual Object.toString`), y sobre una
		// interfaz propia sin `toString` (sale `Object`) y otra que lo redeclara (sale la interfaz).
		// El oráculo y la tabla de la regla viven en `javaemisor_objeto_interfaz_test.go`.
		if objetoEnInterfazVirtual && e.ctx.metodoDeObject(l.nombre, cand.desc) && !e.ctx.declaraElMismo(receptor, l.nombre, cand.desc) {
			opcode = opInvokevirt
			dueño = "java/lang/Object"
		}
	}
	if cand.acceso&accPrivate != 0 {
		if receptor != e.interno {
			return "", fmt.Errorf("linea %d: %s.%s es privado y se llama desde %s", l.donde.linea, receptor, l.nombre, e.interno)
		}
		// `this.metodoPrivado()`: javac tambien escribe `invokespecial` (parrafo 72)
		opcode = opInvokespec
		dueño = receptor
	}
	e.invoca(opcode, dueño, l.nombre, cand.desc, n+1)
	return vueltaDe(cand.desc), nil
}

// argumentosEsperados: baja los argumentos con el tipo que la sobrecarga ya elegida espera. Es lo
// que permite escribir el boxing y el ensanchamiento como javac: la promocion de un literal se decide
// sobre la constante (`ldc2_w 7l`), no con una conversion detras (`bipush 7; i2l`).
func (e *emisor) argumentosEsperados(args []jexpr, desc string) ([]string, error) {
	_, _, ps := splitDescriptor(desc)
	tipos := make([]string, 0, len(args))
	for i, arg := range args {
		esperado := ""
		if i < len(ps) {
			esperado = ps[i]
		}
		d, err := e.expA(arg, esperado)
		if err != nil {
			return nil, err
		}
		tipos = append(tipos, d)
	}
	return tipos, nil
}

// argumentosDeLlamada: baja los argumentos con la forma que javac escribe para la candidata elegida.
// Con aridad fija cada argumento ocupa el parametro que le toca (`argumentosEsperados`, que es lo que
// permite bajar el literal ya promovido). Con una candidata varargs empaquetada, los fijos van uno a
// uno y el resto entra en un arreglo nuevo, elemento a elemento: `cargaEntero(n); newarray/anewarray;
// dup; indice; valor; *astore`, la misma cadena de `new T[] {…}` (§194). Devuelve la aridad que ve el
// `invoke`, que es la del descriptor y no la del fuente.
func (e *emisor) argumentosDeLlamada(args []jexpr, cand candidato) (int, error) {
	_, _, ps := splitDescriptor(cand.desc)
	if !cand.empaquetar || len(ps) == 0 {
		if _, err := e.argumentosEsperados(args, cand.desc); err != nil {
			return 0, err
		}
		return len(args), nil
	}
	fijos := len(ps) - 1
	if len(args) < fijos {
		return 0, fmt.Errorf("la candidata %s necesita %d argumentos y se llamaron %d", cand.desc, fijos, len(args))
	}
	for i := 0; i < fijos; i++ {
		if _, err := e.expA(args[i], ps[i]); err != nil {
			return 0, err
		}
	}
	if err := e.empaquetaVarargs(args[fijos:], ps[fijos]); err != nil {
		return 0, err
	}
	return len(ps), nil
}

// empaquetaVarargs: el arreglo que javac crea para una llamada varargs. Es la cadena de `nuevoArreglo`
// con los elementos del fuente: la cuenta, el `newarray` (0xbc) o `anewarray` (0xbd) del tipo ELEMENTO
// y, por cada argumento, `dup`, su indice, el valor convertido y `iastore`/`aastore`. Medido con javac
// 17 sobre `var(1,2,3)` y `ref("a","b")` (§194).
func (e *emisor) empaquetaVarargs(rest []jexpr, arreglo string) error {
	if len(arreglo) == 0 || arreglo[0] != '[' || len(arreglo) < 2 {
		return fmt.Errorf("el parametro varargs %s no es un arreglo", arreglo)
	}
	elem := arreglo[1:]
	e.cargaEntero(int32(len(rest)))
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
			return fmt.Errorf("un arreglo varargs de %s no esta medido", elem)
		}
		e.emite(0xbc, atype)
	}
	e.baja(1) // la cuenta
	e.sube()  // el arreglo
	guarda, ok := guardadoDeArreglo(elem[0])
	if !ok {
		return fmt.Errorf("un arreglo varargs de %s no esta medido", elem)
	}
	for i, x := range rest {
		e.emite(opDup)
		e.sube()
		e.cargaEntero(int32(i))
		d, err := e.expA(x, elem)
		if err != nil {
			return err
		}
		// El valor no tiene que ser **idéntico** al elemento: `Integer` cabe en un `Object[]` (y un
		// `int` llega aquí ya envuelto por `expA`), igual que `String` cabe en `Object[]` o `int[]` en
		// `Object[][]`. La igualdad de descriptores rechazaba justo el boxing hacia `Object` del §194.
		if !e.ctx.asignable(d, elem) {
			return fmt.Errorf("el parametro varargs es %s y el argumento %d es %s", elem, i, d)
		}
		e.emite(guarda)
		e.baja(2)
		e.bajaPor(elem)
	}
	return nil
}

// guardadoDeArreglo: el `*astore` que guarda un elemento del tipo dado. `castore` para `char`, que es
// lo que javac escribe en `new char[] {'a','b'}` (parrafo 87), y `aastore` para referencias y arreglos.
func guardadoDeArreglo(elem byte) (byte, bool) {
	switch elem {
	case 'I':
		return 0x4f, true
	case 'J':
		return 0x50, true
	case 'F':
		return 0x51, true
	case 'D':
		return 0x52, true
	case 'C':
		return 0x55, true
	case 'L', '[':
		return 0x53, true
	}
	return 0, false
}

// hayEmpaquetado: si alguna candidata varargs obliga a empaquetar esta llamada. Es la pregunta que
// decide el orden de emision: con empaquetado hay que estimar los tipos y resolver ANTES de bajar los
// argumentos; sin el, el camino de siempre (bajarlos y resolver con sus descriptores) vale igual.
func (e *emisor) hayEmpaquetado(args []jexpr, cands []candidato) bool {
	for _, c := range cands {
		if !c.varargs {
			continue
		}
		_, _, ps := splitDescriptor(c.desc)
		if len(ps) == 0 {
			continue
		}
		n := len(ps) - 1
		if len(args) < n {
			continue
		}
		if len(args) == len(ps) {
			// si el ultimo argumento ya es el arreglo declarado, javac no empaqueta: lo pasa tal cual
			if d, ok := e.tipoPrevisto(args[n]); ok && d == ps[n] {
				continue
			}
		}
		return true
	}
	return false
}

// hayEmpaquetadoEn: lo mismo, mirando las candidatas de `clase.nombre` con esta aridad.
func (e *emisor) hayEmpaquetadoEn(clase, nombre string, args []jexpr) bool {
	cands, err := e.ctx.candidatosDe(clase, nombre, len(args), true)
	if err != nil {
		return false
	}
	return e.hayEmpaquetado(args, cands)
}

// necesitaConversionEn: si la candidata de `clase.nombre` para estos tipos estimados pide una conversion
// que se escribe entre el valor y el `invoke`: boxing, desenvuelto o el ensanchamiento primitivo que
// cambia de ancho o escribe un opcode (`i2l`, `i2f`, `i2d`…). Es la pregunta que decide, en una llamada
// con receptor explicito, si hay que resolver ANTES de bajar los argumentos: con boxing el
// `Integer.valueOf` se escribe entre el valor y el `invoke`, y con desenvuelto pasa lo mismo con el
// `intValue()`. Bajar primero el literal dejaria sobre la pila un valor que el descriptor rechaza (§194).
// El ensanchamiento entra por la misma puerta y por una razón de cuenta: el `int` ocupa UNA palabra y el
// parámetro `long` DOS, así que bajar los argumentos primero y descontar después el descriptor deja el
// saldo en -1 —«quedan -1 valores en la pila al cerrar el metodo»— en vez del `i2l` de javac (JLS 5.1.2).
// Lo que no se puede prever sin emitir devuelve falso y deja el camino de siempre, que baja los
// argumentos y resuelve con sus descriptores reales; allí `revisaAnchosDeArgumentos` impide que ese
// desajuste se cuele en silencio.
func (e *emisor) necesitaConversionEn(clase, nombre string, args []jexpr) bool {
	tipos := make([]string, 0, len(args))
	for _, arg := range args {
		d, err := e.tipoEstimado(arg)
		if err != nil {
			return false
		}
		tipos = append(tipos, d)
	}
	cand, err := e.ctx.buscarMetodo(clase, nombre, len(args), tipos)
	if err != nil {
		return false
	}
	if cand.empaquetar {
		return true
	}
	_, _, ps := splitDescriptor(cand.desc)
	for i := range ps {
		if i >= len(tipos) {
			break
		}
		if tipos[i] == ps[i] {
			continue
		}
		// el ensanchamiento primitivo es de la fase estricta, asi que no lo detecta el `asignable`
		// laxo de abajo: se pregunta aparte y solo cuando de verdad escribe algo
		if ajustaAnchosDeArgumento && conversionPrimitiva(tipos[i], ps[i]) {
			return true
		}
		if e.ctx.asignableEstricto(tipos[i], ps[i]) {
			continue
		}
		// cabe en la fase laxa pero no en la estricta: hay boxing o desenvuelto que escribir
		if e.ctx.asignable(tipos[i], ps[i]) {
			return true
		}
	}
	return false
}

// booleanEnsanchaEnArgumentos es el interruptor de la falsificación del rechazo de `boolean` como
// argumento que ensancha. Apagado (el binario que se publica), `boolean` **no** tiene ensanchamiento
// numérico (JLS 5.1.2) y `f(long)` con `f(true)` se rechaza con su línea. Encendido, `Z` vuelve a
// tratarse como `int` y el programa inválido se acepta con `iload; i2l`, que es la regresión que midió
// el crítico. Lo usa `javaemisor_boolean_ensancha_test.go`.
var booleanEnsanchaEnArgumentos = false

// subeAIntParaAncho: si un descriptor de una palabra sube a `int` para escribir después la conversión
// al ancho. `byte`, `short` y `char` sí (JVMS 2.11.1: viven en la pila como int y su subida no escribe
// opcode). `boolean` ocupa una palabra igual, pero no ensancha a nada; solo entra con el interruptor de
// la falsificación. Sin él, esta función es la lista `BCS`.
func subeAIntParaAncho(d string) bool {
	if len(d) != 1 {
		return false
	}
	if strings.IndexByte("BCS", d[0]) >= 0 {
		return true
	}
	return d == "Z" && booleanEnsanchaEnArgumentos
}

// nombrePrimitivo: cómo se llama en el fuente un descriptor primitivo, para los errores de conversión.
// Un descriptor que no sea primitivo se devuelve tal cual.
func nombrePrimitivo(d string) string {
	switch d {
	case "Z":
		return "boolean"
	case "B":
		return "byte"
	case "C":
		return "char"
	case "S":
		return "short"
	case "I":
		return "int"
	case "J":
		return "long"
	case "F":
		return "float"
	case "D":
		return "double"
	}
	return d
}

// conversionPrimitiva: si pasar un valor del descriptor `desde` a un parámetro `hasta` escribe una
// conversión numérica (JLS 5.1.2). El `byte`, el `short` y el `char` suben a `int` sin opcode y desde
// ahí a los anchos; el resto es la rejilla de `codigosConversion`. Identidad y la subida B/C/S a `I`
// devuelven falso. `boolean` **no** ensancha a nada: `conversionPrimitiva("Z", "J")` es falso.
func conversionPrimitiva(desde, hasta string) bool {
	if len(desde) != 1 || len(hasta) != 1 {
		return false
	}
	base := desde[0]
	if subeAIntParaAncho(desde) {
		base = 'I'
	}
	if base == hasta[0] {
		return false
	}
	_, ok := codigosConversion[[2]byte{base, hasta[0]}]
	return ok
}

// ajustaAnchosDeArgumento es el interruptor de la sonda del ensanchamiento en los argumentos. Apagado,
// la llamada vuelve a bajar los argumentos antes de resolver y a descontar después el descriptor ancho,
// que es el defecto medido —«quedan -1 valores en la pila al cerrar el metodo»—: el `int` deja una
// palabra, el `long` descuenta dos y el `i2l` de javac no se escribe. La guarda de palabras se apaga
// con el para que el saldo llegue al cierre del método y el error sea el original. Encendido en el
// binario que se publica; lo apaga `javaemisor_ancho_argumento_test.go`.
var ajustaAnchosDeArgumento = true

// palabrasDeTipos: las palabras de pila que ocupan unos descriptores que ya se emitieron, con la misma
// cuenta que `palabrasDeArgs` (un `long` o un `double` cuentan por dos).
func palabrasDeTipos(tipos []string) int {
	n := 0
	for _, t := range tipos {
		if esAncho(t) {
			n += 2
			continue
		}
		n++
	}
	return n
}

// revisaAnchosDeArgumentos: el descriptor elegido pide las mismas palabras de pila que dejaron los
// argumentos ya emitidos. El camino que baja los argumentos antes de resolver la sobrecarga no puede
// escribir una conversión detrás de un valor ya emitido —la conversión va en el sitio de javac, no al
// final de la lista—, así que un parámetro más ancho que su argumento se rechaza aquí, con la línea, en
// vez de dejar el saldo descuadrado en `-1` o `-2` al cerrar el método.
func revisaAnchosDeArgumentos(tipos []string, desc string, linea int) error {
	if !ajustaAnchosDeArgumento {
		return nil
	}
	_, _, ps := splitDescriptor(desc)
	// un `boolean` contra un hueco numérico no es Java (JLS 5.1.2) aunque ocupe las mismas palabras
	// que un `float`: se nombra el tipo aquí en vez de dejar que lo pare el verificador de la pila con
	// un mensaje que no habla de `boolean`.
	for i := 0; i < len(ps) && i < len(tipos); i++ {
		if tipos[i] == "Z" && len(ps[i]) == 1 && ps[i][0] != 'Z' {
			return fmt.Errorf("linea %d: el argumento %d es boolean y el parámetro %s no lo acepta: boolean no ensancha a ningún tipo numérico (JLS 5.1.2)", linea, i+1, nombrePrimitivo(ps[i]))
		}
	}
	quiere, dejan := palabrasDeArgs(desc), palabrasDeTipos(tipos)
	if quiere == dejan {
		return nil
	}
	// el primer argumento cuyo ancho no cuadra se nombra en el error; antes decía «no se pudo estimar»
	// aunque el tipo sí se hubiera estimado.
	detalle := ""
	for i := 0; i < len(ps) && i < len(tipos); i++ {
		if esAncho(ps[i]) == esAncho(tipos[i]) {
			continue
		}
		detalle = fmt.Sprintf(": el argumento %d es %s y el parámetro %s pide una conversión que no se pudo escribir antes de emitirlo", i+1, nombrePrimitivo(tipos[i]), nombrePrimitivo(ps[i]))
		break
	}
	return fmt.Errorf("linea %d: %s pide %d palabras y los argumentos dejan %d%s", linea, desc, quiere, dejan, detalle)
}

// argumentosResueltos: estima los tipos de los argumentos (sin escribir nada), resuelve la sobrecarga
// y baja los argumentos con la forma de la candidata. Devuelve la candidata y la aridad del `invoke`.
func (e *emisor) argumentosResueltos(clase, nombre string, args []jexpr) (candidato, int, error) {
	tipos := make([]string, 0, len(args))
	for _, arg := range args {
		d, err := e.tipoEstimado(arg)
		if err != nil {
			return candidato{}, 0, err
		}
		tipos = append(tipos, d)
	}
	cand, err := e.ctx.buscarMetodo(clase, nombre, len(args), tipos)
	if err != nil {
		return candidato{}, 0, err
	}
	n, err := e.argumentosDeLlamada(args, cand)
	if err != nil {
		return candidato{}, 0, err
	}
	return cand, n, nil
}

// argumentos: baja los argumentos al pico y devuelve sus descriptores, que es lo que necesita
// buscarMetodo para elegir entre sobrecargas (parrafo 62). El literal `null` se apunta como
// `tipoNulo`: lo que queda en la pila es un `aconst_null` sin tipo propio, y con el descriptor
// `Object` que devuelve `exp` la fase de coincidencia exacta elegía `f(Object)` frente a `f(String)`
// cuando javac se queda con la mas estrecha.
func (e *emisor) argumentos(args []jexpr) ([]string, error) {
	tipos := make([]string, 0, len(args))
	for _, arg := range args {
		d, err := e.exp(arg)
		if err != nil {
			return nil, err
		}
		if esLiteralNulo(arg) {
			d = tipoNulo
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
	// `new Hija()` dentro de la clase exterior (o de otra anidada suya) pasa el exterior como primer
	// argumento del constructor, que es lo que javac escribe para una anidada con nombre no estatica.
	externo, externoDesc := "", ""
	if inner, ok := e.ctx.esInnerNoEstatica(interno); ok {
		externo = internoDe(inner.encierre.paquete, inner.encierre.nombre)
		externoDesc = "L" + externo + ";"
	}
	// el descriptor del constructor es el de la DECLARACION, no el de los tipos que se le pasan: con
	// `new TextView(this)` el argumento es la actividad y el constructor es
	// `<init>(Landroid/content/Context;)V`; firmarlo con el tipo del argumento compila y ART corta con
	// `NoSuchMethodError` (§136)
	tipos := make([]string, 0, len(n.args)+1)
	if externoDesc != "" {
		tipos = append(tipos, externoDesc)
	}
	// Una lambda no trae tipo propio: lo pone el parametro formal que la recibe (JLS 15.27.3), asi que
	// con una en los argumentos la sobrecarga se elige mirando el parametro de la declaracion y no el
	// tipo de la expresion, que no se puede estimar antes de haber elegido (medido en el frente de
	// ApkShare)
	conLambda := false
	for _, arg := range n.args {
		if _, es := arg.(jlambda); es {
			conLambda = true
			tipos = append(tipos, tipoLambda)
			continue
		}
		d, err := e.tipoEstimado(arg)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", n.donde.linea, err)
		}
		tipos = append(tipos, d)
	}
	cantArgs := len(tipos)
	var cand candidato
	if conLambda {
		cand, err = e.ctx.buscarCtorConLambdas(interno, tipos)
	} else {
		cand, err = e.ctx.buscarMetodo(interno, "<init>", cantArgs, tipos)
	}
	if err != nil {
		return "", fmt.Errorf("linea %d: %s: %w", n.donde.linea, interno, err)
	}
	// Un constructor privado no se puede llamar desde fuera de su clase: javac 17 con --release 8
	// escribe un constructor sintetico con un ultimo parametro de una clase marcador (Exterior$1)
	// y la llamada le pasa `null` (medido con `javap -v` sobre Capa.java).
	marcador := ""
	descLlamada := cand.desc
	if cand.acceso&accPrivate != 0 && e.interno != interno {
		inner, ok := e.ctx.clasePropia(interno)
		if !ok || inner.encierre == nil {
			return "", fmt.Errorf("linea %d: el constructor privado de %s no se puede llamar desde fuera", n.donde.linea, interno)
		}
		if cand.desc != "()V" {
			return "", fmt.Errorf("linea %d: el constructor sintetico de la anidada privada %s con argumentos no esta medido", n.donde.linea, interno)
		}
		marcador = internoDe(inner.encierre.paquete, inner.encierre.nombre) + "$1"
		e.ctx.ctorSintetico[interno] = marcador
		i := strings.LastIndex(cand.desc, ")")
		if i < 0 {
			return "", fmt.Errorf("linea %d: el descriptor %s no es de un constructor", n.donde.linea, cand.desc)
		}
		descLlamada = cand.desc[:i] + "L" + marcador + ";" + cand.desc[i:]
	}
	e.usar(interno)
	idx := e.pool.clase(interno)
	e.emite(opNew, byte(idx>>8), byte(idx))
	e.sube()
	e.emite(opDup)
	e.sube()
	if externoDesc != "" {
		if e.interno == externo {
			e.cargaDe(0, "L")
		} else if err := e.cargaDelExteriorHasta(externo); err != nil {
			return "", fmt.Errorf("linea %d: no se puede construir %s desde %s sin el exterior %s", n.donde.linea, interno, e.interno, externo)
		}
	}
	if cand.empaquetar {
		// un `new X(...)` que tenga que empaquetar varargs no está medido: se rechaza con nombre en vez
		// de escribir bytes que no serían los de javac
		return "", fmt.Errorf("linea %d: el constructor varargs de %s no esta medido", n.donde.linea, interno)
	}
	// Los argumentos se bajan con el descriptor del parametro que les toca: una lambda se convierte en
	// su clase sintetica con el tipo funcional del parametro formal, y el resto pasa por `expA` como
	// siempre. El exterior de una anidada ocupa el primer hueco del descriptor.
	_, _, params := splitDescriptor(descLlamada)
	base := 0
	if externoDesc != "" {
		base = 1
	}
	for i, arg := range n.args {
		if lam, es := arg.(jlambda); es {
			if _, err := e.lambdaEnParametro(lam, cand, base+i); err != nil {
				return "", err
			}
			continue
		}
		deseado := ""
		if base+i < len(params) {
			deseado = params[base+i]
		}
		if _, err := e.expA(arg, deseado); err != nil {
			return "", err
		}
	}
	if marcador != "" {
		e.emite(0x01) // aconst_null
		e.sube()
	}
	// el `dup` deja dos referencias: el constructor se come una con sus argumentos y la otra es el valor de
	// la expresion, que es el hueco que sube de la pila al terminar
	e.invoca(opInvokespec, interno, "<init>", descLlamada, cantArgs+1)
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
	if len(v.dimensiones) > 1 {
		// d8 baja `multianewarray` a `Array.newInstance` con un int[] de dimensiones y un
		// check-cast al tipo final. Medido en §168 sobre `new int[2][3]`.
		e.marcaLinea(v.donde.linea)
		idxTipo := e.pool.campo("java/lang/Integer", "TYPE", "Ljava/lang/Class;")
		e.emite(opGetstatic, byte(idxTipo>>8), byte(idxTipo))
		e.sube()
		dims := jnuevoArreglo{tipo: "int", elementos: v.dimensiones, donde: v.donde}
		d, err := e.exp(dims)
		if err != nil {
			return "", err
		}
		if d != "[I" {
			return "", fmt.Errorf("linea %d: las dimensiones dieron %s y no [I", v.donde.linea, d)
		}
		e.invoca(opInvokestat, "java/lang/reflect/Array", "newInstance",
			"(Ljava/lang/Class;[I)Ljava/lang/Object;", 2)
		desc := strings.Repeat("[", len(v.dimensiones)) + elem
		idxClase := e.pool.clase(desc)
		e.emite(opCheckcast, byte(idxClase>>8), byte(idxClase))
		return desc, nil
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
	guarda, ok := guardadoDeArreglo(elem[0])
	if !ok {
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
		// igual que en `empaquetaVarargs`: `Integer` cabe en un `Object[]`, y `expA` ya envolvió el
		// primitivo si hacía falta (§194).
		if !e.ctx.asignable(d, elem) {
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
	// `expA` devuelve el descriptor que queda en la pila: con `a[0] = 7` sobre un `Object[]` es el
	// envoltorio (`Ljava/lang/Integer;`), que javac escribe con `Integer.valueOf` antes del `aastore`.
	// Lo que no cabe es lo de siempre: identidad, ensanchamiento o boxing con su subida a referencia.
	if !e.ctx.asignable(dv, elem) {
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

// resuelveCampoDeLocal es el interruptor de la sonda del tipo de un campo de una local: apagado, la
// cadena `info.applicationInfo` no se resuelve desde el tipo estático del receptor y `tipoEstimado`
// vuelve al rechazo de antes. Lo usa la falsificación de `javaemisor_campo_local_test.go`; encendido en
// el binario que se publica.
var resuelveCampoDeLocal = true

// estimaMoldeSinEmitir es el interruptor de la sonda del molde como argumento: apagado, `tipoEstimado`
// no reconoce un `jcast` y devuelve el rechazo de antes («el tipo de una expresion main.jcast no se
// puede estimar sin emitirla»). Lo usa la falsificación de `javaemisor_molde_argumento_test.go`;
// encendido en el binario que se publica.
var estimaMoldeSinEmitir = true

// estimaLlamadaEncadenada es el interruptor de la sonda de la llamada encadenada como argumento:
// apagado, `tipoEstimado` no reconoce un `jllamada` con receptor y devuelve el rechazo de antes («no se
// puede estimar el tipo de get() sin emitir el receptor de la llamada»). Lo usa la falsificación de
// `javaemisor_llamada_argumento_test.go`; encendido en el binario que se publica.
var estimaLlamadaEncadenada = true

// estimaEstaticaCualificada es el interruptor de la sonda de la llamada estática calificada por el
// nombre del tipo (`Intent.createChooser(...)`) como argumento de otra llamada: apagado, `tipoEstimado`
// no la reconoce y devuelve el rechazo de antes («no se puede estimar el tipo de createChooser() sin
// emitir el receptor de la llamada»). Lo usa la falsificación de
// `javaemisor_estatica_cualificada_test.go`; encendido en el binario que se publica.
var estimaEstaticaCualificada = true

// propagaGenericoDeLocal es el interruptor de la sonda del tipo declarado del local. Apagado, un local
// no apunta sus argumentos de tipo y la ruta vuelve al comportamiento de `c0b40e9`: el borrado en
// silencio —`usa(Object)` y sin `check-cast`—, que es justo el defecto que mide esta ruta. Lo usa la
// falsificación de `javaemisor_llamada_argumento_test.go`; encendido en el binario que se publica.
var propagaGenericoDeLocal = true

// estimaConcatEnArgumento es el interruptor de la sonda de la concatenación como argumento: apagado,
// `tipoEstimado` no reconoce un `joperador` de `+` con cadena y devuelve el rechazo de antes («el tipo
// de una expresion main.joperador no se puede estimar sin emitirla»). Lo usa la falsificación de
// `javaemisor_concat_argumento_test.go`; encendido en el binario que se publica.
var estimaConcatEnArgumento = true

// descDeCampos: el descriptor del último campo de una cadena `a.b.c` sobre un receptor cuyo tipo ya se
// conoce, sin escribir el `aload`/`getfield` que emitiría `cadenaDeCampos`. `length` sobre un arreglo da
// `I`, como el `arraylength` de 0xbe. Devuelve además el tipo declarado del último campo cuando trae
// argumentos (`List<Apk>`), que es lo que estrecha la vuelta de un método genérico. Lo que no se
// resuelve sin código sale con ok falso.
func (e *emisor) descDeCampos(actual string, segs []string) (string, *tipoArgumento, bool) {
	var generico *tipoArgumento
	for _, seg := range segs {
		if seg == "length" && len(actual) > 0 && actual[0] == '[' {
			actual = "I"
			generico = nil
			continue
		}
		if len(actual) == 0 || actual[0] != 'L' {
			return "", nil, false
		}
		cr, err := e.ctx.buscarCampo(actual[1:len(actual)-1], seg)
		if err != nil {
			return "", nil, false
		}
		actual = cr.desc
		generico = cr.generico
	}
	return actual, generico, true
}

// tipoSinEmitir: el descriptor estático de una expresión que solo se lee, sin escribir su código. Es lo
// que resuelve `info.applicationInfo` —el campo de una local— cuando hay que estimar su tipo como
// argumento de una llamada y el receptor no se puede emitir todavía: la cadena de campos se apoya en el
// tipo estático del receptor, que se conoce (la tabla de locales, `this` o un campo de la clase), y no
// en un `aload`/`getfield` de más (§244). Devuelve ok falso en cuanto algo no se sabe sin código; el
// llamador elige el mensaje.
func (e *emisor) tipoSinEmitir(x jexpr) (string, bool) {
	d, _, ok := e.tipoSinEmitirCompleto(x)
	return d, ok
}

// tipoSinEmitirCompleto: `tipoSinEmitir` y, además, el tipo declarado del último campo de la cadena
// cuando trae argumentos (`app.apks` es un `List<Apk>`). Es lo que necesita la vuelta de un método
// genérico encadenado: `List.get` declara `(I)TE;` y el argumento de tipo lo pone el campo.
func (e *emisor) tipoSinEmitirCompleto(x jexpr) (string, *tipoArgumento, bool) {
	switch v := x.(type) {
	case jacceso:
		segs, base := aplanar(v)
		if len(segs) == 0 {
			return "", nil, false
		}
		if !resuelveCampoDeLocal && (base != nil || len(segs) > 1) {
			// apagado, la cadena de campos no se toca: la resuelve el llamador como pueda, que es el
			// rechazo que medía el usuario antes de §242
			return "", nil, false
		}
		var actual string
		var generico *tipoArgumento
		if base != nil {
			d, ok := e.tipoSinEmitir(base)
			if !ok {
				return "", nil, false
			}
			actual = d
		} else if lv, ok := e.locales[segs[0]]; ok {
			// una local tapa al tipo que se llame igual, y con ella se lleva sus argumentos de tipo
			// (`List<Apk> m`): sin ellos `m.get(0)` devolvería el borrado
			actual = lv.desc
			generico = lv.generico
			segs = segs[1:]
		} else if _, cr, ok := e.capturaDeExterior(segs[0]); ok {
			// una local capturada por una anonima que encierra a esta: el tipo es el del campo
			// `val$nombre`, igual que al emitirla. Su `Signature` no viaja en el `.class` —javac
			// declara el campo con el borrado—, así que si traía argumentos la ruta no los inventa.
			actual = cr.desc
			generico = cr.generico
			segs = segs[1:]
		} else if len(segs) == 1 {
			// un campo suelto de la clase actual o del exterior que captura la anónima
			_, cr, esCampo := e.duenioDelCampo(segs[0])
			if !esCampo {
				return "", nil, false
			}
			return cr.desc, cr.generico, true
		} else if interno, resto, ok := e.ctx.tipoDeReferencia(segs); ok {
			d, err := e.descDeAccesoEstatico(interno, resto)
			if err != nil {
				return "", nil, false
			}
			return d, nil, true
		} else {
			return "", nil, false
		}
		if len(segs) == 0 {
			// la expresión es la local misma (`m.get(0)`), así que el tipo declarado es el suyo
			return actual, generico, true
		}
		return e.descDeCampos(actual, segs)
	case jnuevo:
		// `f(new S())`: el tipo es el de la clase construida, igual que en `tipoEstimado`
		if v.cuerpo != nil {
			return "L" + internoDe(v.cuerpo.paquete, v.cuerpo.nombre) + ";", nil, true
		}
		interno, err := e.ctx.tipoAInterno(v.tipo)
		if err != nil {
			return "", nil, false
		}
		return "L" + interno + ";", nil, true
	}
	// literales, `this`, `X.class`, elementos de arreglo, negaciones y cuentas ya se prevén sin código
	d, ok := e.tipoPrevisto(x)
	return d, nil, ok
}

// estimaMetodoDeExterior es el interruptor de la sonda del método heredado sin receptor escrito dentro
// de una anónima: apagado, `tipoPrevisto` y `tipoEstimado` solo miran la clase actual y vuelven al
// rechazo de antes («no se puede estimar el tipo de getExternalFilesDir() sin emitir el receptor de la
// llamada»). Lo usa la falsificación de `javaemisor_receptor_heredado_test.go`; encendido en el binario
// que se publica.
var estimaMetodoDeExterior = true

// candidatasSinReceptor: las candidatas de una llamada sin receptor escrito. Primero la clase actual —y
// su cadena de padres, que `candidatos` ya recorre— y, si ahí no hay ninguna, las clases que la encierran,
// de dentro afuera, que es el mismo orden con el que la emisión resuelve el nombre suelto (parágrafo 72).
// Dentro de la anónima de `Runnable` de `MainActivity.java:179` ni `getApplicationContext()` ni
// `getExternalFilesDir(null)` están en la anónima: son métodos de `Context`/`Activity` que javac alcanza
// por el campo `this$0`, y su descriptor se resuelve aquí leyendo la jerarquía, sin escribir ninguna
// instrucción. Mientras la clase actual traiga candidatas se devuelve su lista y nada cambia para los
// casos que ya se medían; el error de `candidatos` se conserva para que el llamador lo nombre.
//
// Con más de una candidata en la aridad manda el tipo de los argumentos, igual que en la emisión: ver
// `unaCandidataPorTipos`.
func (e *emisor) candidatasSinReceptor(nombre string, args []jexpr) ([]candidato, error) {
	argc := len(args)
	cands, err := e.ctx.candidatos(e.interno, nombre, argc)
	if err != nil || len(cands) > 0 {
		return e.unaCandidataPorTipos(e.interno, nombre, cands, args), err
	}
	if !estimaMetodoDeExterior || !(e.c.anonima || e.esInnerNoEstatica()) {
		return cands, err
	}
	for _, c := range clasesEnAlcance(e.c) {
		interno := internoDe(c.paquete, c.nombre)
		cs, errC := e.ctx.candidatos(interno, nombre, argc)
		if errC != nil || len(cs) == 0 {
			continue
		}
		return e.unaCandidataPorTipos(interno, nombre, cs, args), nil
	}
	return cands, err
}

// filtraCandidatasPorTipos es el interruptor de la sonda de la sobrecarga de igual aridad: apagado,
// `unaCandidataPorTipos` devuelve la primera lista no vacía tal cual y el estimador vuelve al rechazo de
// antes («hay 2 candidatas con 1 argumentos»). Lo usa la falsificación de
// `javaemisor_receptor_heredado_test.go`; encendido en el binario que se publica.
var filtraCandidatasPorTipos = true

// unaCandidataPorTipos: cuando la llamada sin receptor tiene más de una candidata en su aridad, la
// sobrecarga la decide el tipo de los argumentos, con la misma resolución que la emisión
// (`elegirSobrecarga`). Sin esto, dos `f` de igual aridad en la clase exterior —`f(String)` y `f(int)`—
// hacían que el estimador contara dos candidatas y rechazara un fuente que javac acepta. Si los tipos no
// se pueden estimar o las fases no dejan una sola candidata, se devuelve la lista tal cual y el llamador
// sigue rechazando en vez de adivinar: el fail-safe es no elegir, nunca elegir la primera.
func (e *emisor) unaCandidataPorTipos(interno, nombre string, cands []candidato, args []jexpr) []candidato {
	if !filtraCandidatasPorTipos || len(cands) < 2 {
		return cands
	}
	tipos := make([]string, 0, len(args))
	for _, a := range args {
		d, err := e.tipoEstimado(a)
		if err != nil {
			return cands
		}
		tipos = append(tipos, d)
	}
	cand, err := e.ctx.elegirSobrecarga(interno, nombre, cands, len(args), tipos)
	if err != nil {
		return cands
	}
	return []candidato{cand}
}

// vueltaSinEmitir: el descriptor de lo que deja en la pila una llamada cuyo receptor **no se emite**.
// Es el caso de `shareApp(app.apks.get(0))` de ApkShare (`MainActivity.java:163`), donde el argumento
// de una llamada es otra llamada encadenada: para resolver la sobrecarga de `shareApp` hace falta el
// tipo de `get()`, y `tipoEstimado` se negaba a estimarlo sin escribir antes el receptor. El tipo sale
// del tipo estático del receptor (`tipoSinEmitirCompleto`, §244) y del descriptor del método, que trae
// la aridad y la vuelta borrada; el argumento de tipo del `Signature` la estrecha cuando el receptor
// declara los suyos (`app.apks` es un `List<Apk>` y `get` declara `(I)TE;`), que es con lo que resuelve
// javac. Sin genérico se devuelve el borrado, que es lo que la llamada deja de verdad en la pila: el
// `check-cast` al tipo estrecho lo escribe `expA` al llenar el hueco, igual que javac.
//
// ok falso cuando el receptor no se sabe sin código: el llamador devuelve el rechazo de siempre, sin
// adivinar. El orden de evaluación no cambia, porque aquí no se escribe ninguna instrucción.
func (e *emisor) vueltaSinEmitir(v jllamada) (string, bool, error) {
	if !estimaLlamadaEncadenada || v.base == nil || v.super {
		return "", false, nil
	}
	d, generico, ok := e.tipoSinEmitirCompleto(v.base)
	if !ok || len(d) == 0 || d[0] != 'L' {
		return "", false, nil
	}
	receptor := d[1 : len(d)-1]
	// la sobrecarga se resuelve con los tipos de los argumentos de la propia llamada, estimados como
	// los de cualquier otra; si alguno no se sabe sin código, el rechazo es el de siempre
	tipos := make([]string, 0, len(v.args))
	for _, arg := range v.args {
		da, err := e.tipoEstimado(arg)
		if err != nil {
			return "", false, nil
		}
		tipos = append(tipos, da)
	}
	cand, err := e.ctx.buscarMetodo(receptor, v.nombre, len(v.args), tipos)
	if err != nil {
		return "", true, fmt.Errorf("%s: al estimar el tipo de %s(), %w", v.donde.sitio(), v.nombre, err)
	}
	if vuelta, ok := e.vueltaGenerica(receptor, v.nombre, cand.desc, generico); ok {
		return vuelta, true, nil
	}
	if !propagaGenericoDeLocal {
		// la sonda: el borrado en silencio de `c0b40e9`, que elige `usa(Object)` y se come el `check-cast`
		return vueltaDe(cand.desc), true, nil
	}
	if e.vueltaConTipoVariable(receptor, v.nombre, cand.desc) {
		// la vuelta declarada es un tipo variable (`List.get` declara `(I)TE;`) y el argumento que lo
		// sustituye no se conoce —una local cuyo tipo declarado con argumentos no viaja en el fuente,
		// o un método heredado de otra clase—. Se devuelve el rechazo explícito de antes en vez del
		// borrado: `Object` no es el tipo con el que resuelve javac y la sobrecarga elegida sería otra.
		return "", false, nil
	}
	return vueltaDe(cand.desc), true, nil
}

// vueltaEstaticaCualificada: el descriptor de la vuelta de una llamada **estática calificada por el
// nombre del tipo** (`Intent.createChooser(sharedIntent, "Compartir con")`), sin escribir ninguna
// instrucción. Es la misma resolución que la emisión (`exp`, rama «una llamada estática sobre un nombre
// de tipo»): el prefijo más largo que resuelve como tipo tiene que ser el receptor entero y la candidata
// tiene que ser estática, o no se estima. La cadena de `MainActivity.java:212` es
// `startActivity(Intent.createChooser(...))`: para elegir la sobrecarga de `startActivity` hace falta el
// tipo del argumento, y `Intent` no es una local ni un campo sino un nombre de tipo, que la estimación
// trataba como instancia y por eso pedía emitir el receptor. El orden de evaluación no cambia: aquí solo
// se lee el `android.jar`, y la emisión sigue bajando los argumentos y escribiendo el `invokestatic`
// contra la clase (no hay receptor en la pila).
func (e *emisor) vueltaEstaticaCualificada(v jllamada) (string, bool, error) {
	if !estimaEstaticaCualificada {
		return "", false, nil
	}
	acc, esAcceso := v.base.(jacceso)
	if !esAcceso {
		return "", false, nil
	}
	segs, base := aplanar(acc)
	// el mismo guardián que la emisión: con una local del mismo nombre delante, el receptor es esa
	// local —una instancia— y el camino de instancia no se toca
	if base != nil || len(segs) == 0 || e.locales[segs[0]].desc != "" {
		return "", false, nil
	}
	interno, resto, ok := e.ctx.tipoDeReferencia(segs)
	if !ok || len(resto) != 0 {
		return "", false, nil
	}
	tipos := make([]string, 0, len(v.args))
	for _, arg := range v.args {
		d, err := e.tipoEstimado(arg)
		if err != nil {
			// un argumento que no se estima sin código deja la llamada al rechazo de siempre
			return "", false, nil
		}
		tipos = append(tipos, d)
	}
	cand, err := e.ctx.buscarMetodo(interno, v.nombre, len(v.args), tipos)
	if err != nil {
		// el receptor sí es un nombre de tipo y la candidata no existe: se devuelve **su** motivo, que
		// es el del argumento de tipo incorrecto, y no un «no se puede estimar» que escondería la causa
		return "", true, fmt.Errorf("%s: al estimar el tipo de %s(), %w", v.donde.sitio(), v.nombre, err)
	}
	if cand.acceso&accStatic == 0 {
		// un método de instancia llamado sobre el tipo lo rechaza la emisión con su propio mensaje;
		// aquí se devuelve el rechazo de antes en vez de inventar un tipo
		return "", false, nil
	}
	if e.vueltaConTipoVariable(interno, v.nombre, cand.desc) {
		// la vuelta declara un tipo variable y el argumento que lo sustituye no viaja en la llamada
		// (`Collections.emptyList()`): el borrado elegiría otra sobrecarga, así que se rechaza
		return "", false, nil
	}
	return vueltaDe(cand.desc), true, nil
}

// vueltaConTipoVariable: si la `Signature` del método declara la vuelta con un tipo variable en
// cualquier posición (`TE;`, `[TE;`, `Ljava/util/List<TE;>;`). En los dos últimos el descriptor
// borrado acierta la sobrecarga —Java no resuelve por argumentos de tipo—, pero el llamador no puede
// distinguirlo sin mirar la firma, así que se rechaza igual: es más barato que adivinar.
func (e *emisor) vueltaConTipoVariable(receptor, nombre, desc string) bool {
	cf, err := e.ctx.busc.claseDelJar(receptor)
	if err != nil {
		return false
	}
	firma, ok := firmaDelMetodoDelJar(cf, nombre, desc)
	if !ok {
		return false
	}
	return vueltaConVariable(firma)
}

// vueltaConVariable: recorre el tipo de vuelta de una `Signature` de método y dice si nombra algún
// tipo variable. Una `T` solo abre un tipo variable en un comienzo de tipo (el principio, detrás de
// `[` o detrás de `<`), así que un nombre de clase que empiece por T no confunde.
func vueltaConVariable(firma string) bool {
	cierre := strings.IndexByte(firma, ')')
	if cierre < 0 {
		return false
	}
	principio := true
	for i := cierre + 1; i < len(firma); i++ {
		switch firma[i] {
		case 'L':
			// el nombre de la clase no lleva tipos hasta sus argumentos
			for i < len(firma) && firma[i] != '<' && firma[i] != ';' {
				i++
			}
			if i < len(firma) && firma[i] == '<' {
				principio = true
				continue
			}
			principio = false
		case '[':
			principio = true
		case 'T':
			if principio {
				return true
			}
			principio = false
		default:
			principio = false
		}
	}
	return false
}

// vueltaGenerica: estrecha la vuelta borrada de un método con el argumento de tipo que declara el
// receptor (`List<Apk>.get` → `(I)TE;` con `E` = `Apk`). Solo sustituye cuando el tipo declarado del
// receptor es la propia clase del método y el número de parámetros de tipo de su `Signature` es el de
// argumentos declarados: con una herencia de por medio (`ArrayList<Apk>` con `get` en `List`) el mapeo
// posicional no es una regla de Java y se devuelve el borrado en vez de adivinar.
func (e *emisor) vueltaGenerica(receptor, nombre, desc string, decl *tipoArgumento) (string, bool) {
	if decl == nil || len(decl.args) == 0 || decl.arreglos > 0 || decl.comodin != 0 {
		return "", false
	}
	propio, err := e.ctx.descriptorDe(decl.crudo())
	if err != nil || propio != "L"+receptor+";" {
		return "", false
	}
	cf, err := e.ctx.busc.claseDelJar(receptor)
	if err != nil {
		return "", false
	}
	nombres := parametrosDeTipoDeFirma(cf.Signature)
	if len(nombres) == 0 || len(nombres) != len(decl.args) {
		return "", false
	}
	vars := make(map[string]string, len(nombres))
	for i, n := range nombres {
		da, err := e.ctx.descriptorDe(decl.args[i].crudo())
		if err != nil {
			return "", false
		}
		vars[n] = da
	}
	firma, ok := firmaDelMetodoDelJar(cf, nombre, desc)
	if !ok {
		return "", false
	}
	return vueltaDeFirma(firma, vars)
}

// firmaDelMetodoDelJar: la `Signature` del método `nombre` con el descriptor `desc` en esa clase. Solo
// en la propia clase: si el método se hereda, el tipo variable puede ser el del padre y no el del
// receptor, y entonces no se sustituye.
func firmaDelMetodoDelJar(cf *classFull, nombre, desc string) (string, bool) {
	for _, m := range cf.Methods {
		if m.Name == nombre && m.Desc == desc && m.Signature != "" {
			return m.Signature, true
		}
	}
	return "", false
}

// parametrosDeTipoDeFirma: los nombres de los parámetros de tipo de la `Signature` de una clase
// (JVMS 4.7.9.1), en orden de declaración: `<E:Ljava/lang/Object;>Ljava/lang/Object;` da `["E"]`. La
// firma de una clase sin parámetros (`Ljava/lang/Object;`) no los tiene.
func parametrosDeTipoDeFirma(firma string) []string {
	if len(firma) == 0 || firma[0] != '<' {
		return nil
	}
	nombres := []string{}
	i := 1
	for i < len(firma) && firma[i] != '>' {
		j := i
		for j < len(firma) && firma[j] != ':' {
			j++
		}
		if j >= len(firma) {
			return nil
		}
		nombres = append(nombres, firma[i:j])
		i = j
		// los límites del parámetro: `:` con el tipo de clase (puede faltar) y `:` con cada interfaz
		for i < len(firma) && firma[i] == ':' {
			i++
			if i < len(firma) && firma[i] == ':' {
				continue
			}
			siguiente, ok := saltaFirmaTipo(firma, i)
			if !ok {
				return nil
			}
			i = siguiente
		}
	}
	if len(nombres) == 0 {
		return nil
	}
	return nombres
}

// saltaFirmaTipo: avanza un tipo de una Signature sin interpretarlo. Basta con contar los `<>`
// anidados y parar en el `;` del nivel de fuera; un primitivo es una sola letra.
func saltaFirmaTipo(s string, i int) (int, bool) {
	if i >= len(s) {
		return i, false
	}
	if strings.IndexByte("BCDFIJSZV", s[i]) >= 0 {
		return i + 1, true
	}
	dentro := 0
	for ; i < len(s); i++ {
		switch s[i] {
		case '<':
			dentro++
		case '>':
			dentro--
		case ';':
			if dentro == 0 {
				return i + 1, true
			}
		}
	}
	return i, false
}

// vueltaDeFirma: el descriptor de vuelta de una `Signature` de método (`(I)TE;` con `E` = `Apk` da
// `Lshare/apps/util/Apk;`), con los tipos variables sustituidos. ok falso si la firma nombra una
// variable que no se conoce o una forma que este subconjunto no lee: el llamador se queda con el
// borrado del descriptor en vez de adivinar.
func vueltaDeFirma(firma string, vars map[string]string) (string, bool) {
	cierre := strings.IndexByte(firma, ')')
	if cierre < 0 {
		return "", false
	}
	d, _, ok := tipoDeFirma(firma, cierre+1, vars)
	if !ok || d == "V" {
		return "", false
	}
	return d, true
}

// tipoDeFirma: lee un `JavaTypeSignature` de una Signature y devuelve su descriptor borrado, con los
// tipos variables sustituidos por lo que diga `vars`, y el índice siguiente. Las anidadas detrás de
// argumentos (`LOuter<...>.Inner;`) se rechazan: no están medidas y su descriptor no es el del prefijo.
func tipoDeFirma(s string, i int, vars map[string]string) (string, int, bool) {
	if i >= len(s) {
		return "", i, false
	}
	switch s[i] {
	case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z', 'V':
		return string(s[i]), i + 1, true
	case '[':
		d, siguiente, ok := tipoDeFirma(s, i+1, vars)
		if !ok {
			return "", i, false
		}
		return "[" + d, siguiente, true
	case 'T':
		fin := strings.IndexByte(s[i:], ';')
		if fin < 0 {
			return "", i, false
		}
		d, ok := vars[s[i+1:i+fin]]
		if !ok {
			return "", i, false
		}
		return d, i + fin + 1, true
	case 'L':
		fin := i + 1
		for fin < len(s) && s[fin] != '<' && s[fin] != ';' {
			fin++
		}
		if fin >= len(s) {
			return "", i, false
		}
		nombre := strings.ReplaceAll(s[i+1:fin], ".", "$")
		if s[fin] == ';' {
			return "L" + nombre + ";", fin + 1, true
		}
		// los argumentos del tipo no cambian el descriptor (es el del borrado), pero se saltan
		dentro := 0
		j := fin
		for ; j < len(s); j++ {
			if s[j] == '<' {
				dentro++
			}
			if s[j] == '>' {
				dentro--
				if dentro == 0 {
					j++
					break
				}
			}
		}
		if dentro != 0 || j >= len(s) || s[j] != ';' {
			return "", i, false
		}
		return "L" + nombre + ";", j + 1, true
	}
	return "", i, false
}

// tipoEstimado: el descriptor de un argumento, sin volver a emitirlo. En el subconjunto medido los
// argumentos de un `new` son literales o variables, y ambos se saben aqui; tambien el `new` de una
// clase propia (`f(new S())`) y el literal `null`, que llega como `tipoNulo`.
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
			// `null` no tiene tipo: se apunta el marcador para que la fase de coincidencia exacta no
			// pueda elegir `f(Object)` cuando javac se queda con `f(String)` por especificidad
			return tipoNulo, nil
		}
		return "", fmt.Errorf("linea %d: un literal de clase %q no esta medido", v.donde.linea, string(v.tipoJ))
	case jnuevo:
		// `f(new S())`: el tipo de la expresion es el de la clase construida, no el de los argumentos
		// del constructor. Se resuelve igual que en `nuevo`, para que la estimacion y la emision
		// coincidan con el `new`/`dup`/`<init>` que escribe `exp`.
		if v.cuerpo != nil {
			return "L" + internoDe(v.cuerpo.paquete, v.cuerpo.nombre) + ";", nil
		}
		interno, err := e.ctx.tipoAInterno(v.tipo)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		return "L" + interno + ";", nil
	case jclase:
		return "Ljava/lang/Class;", nil
	case jcast:
		// el molde fija el tipo de la expresión, no su receptor: `(App) x` es
		// `Lshare/apps/util/App;` aunque `x` sea una llamada encadenada que no se puede estimar sin
		// emitirla. Es el argumento de `openAppDetailsDialog((App) apps.getAdapter().getItem(positon))`
		// de ApkShare (`MainActivity.java:55`), que llegaba aquí desde una anónima y se rechazaba con
		// «el tipo de una expresion main.jcast no se puede estimar sin emitirla». El descriptor es el
		// mismo que devuelve `exp` al escribir el `check-cast`, así que estimar y emitir coinciden.
		if estimaMoldeSinEmitir {
			d, err := e.ctx.descriptorDe(v.tipo)
			if err != nil {
				return "", fmt.Errorf("%s: al estimar el tipo del molde (%s), %w", v.donde.sitio(), v.tipo, err)
			}
			return d, nil
		}
	case jacceso:
		segs, base := aplanar(v)
		// `info.applicationInfo` llega aquí con `base == nil` y dos segmentos, porque `aplanar` no
		// distingue el campo de una local del nombre cualificado de un tipo. El tipo se resuelve desde
		// el tipo estático del receptor —la tabla de locales, `this` o la cadena de campos— sin emitirlo
		// (§244).
		if d, ok := e.tipoSinEmitir(v); ok {
			return d, nil
		}
		if base != nil {
			return "", fmt.Errorf("%s: no se puede estimar el tipo de %s sin emitir su receptor: eso solo pasa con accesos de un nombre, variables locales, campos de una local y `Clase.campo` estaticos", v.donde.sitio(), strings.Join(segs, "."))
		}
		// `R.layout.main` y cualquier `Clase.campo` / `Clase.campo.campo`, también con el tipo
		// cualificado (`com.x.dos.Helper.CAMPO`): el tipo es el prefijo más largo que resuelve y el
		// resto son campos. Cuando no se resuelve se devuelve **su** motivo (que suele ser una `R`
		// que el classpath no trae) y no un «no esta medido» que escondia la causa real (§200).
		interno, resto, ok := e.ctx.tipoDeReferencia(segs)
		if !ok {
			_, err := e.ctx.tipoAInterno(strings.Join(segs, "."))
			return "", fmt.Errorf("%s: al estimar el tipo de %s, %w", v.donde.sitio(), strings.Join(segs, "."), err)
		}
		d, err := e.descDeAccesoEstatico(interno, resto)
		if err != nil {
			return "", fmt.Errorf("%s: al estimar el tipo de %s, %w", v.donde.sitio(), strings.Join(segs, "."), err)
		}
		return d, nil
	case jthis:
		return "L" + e.interno + ";", nil
	case jindice:
		if acc, ok := v.base.(jacceso); ok && acc.base == nil {
			if lv, ok := e.locales[acc.nombre]; ok && len(lv.desc) > 0 && lv.desc[0] == '[' {
				return lv.desc[1:], nil
			}
		}
		return "", fmt.Errorf("%s: no se puede estimar el tipo de un elemento de arreglo si el arreglo no es una variable local", v.donde.sitio())
	case jnuevoArreglo:
		d, err := e.ctx.descriptorDe(v.tipo)
		if err != nil {
			return "", fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		n := 1
		if len(v.dimensiones) > 1 {
			n = len(v.dimensiones)
		}
		return strings.Repeat("[", n) + d, nil
	case jnegativo:
		// `-x` conserva el ancho de su operando (`-l` es largo, `-f` flotante) salvo el estrecho,
		// `char`/`byte`/`short`, que se niega como entero y devuelve `int` (medido con javac 17:
		// `-c` es `iload_0; ineg` y la vuelta es `I`). Así `imprime(-n)` elige la misma sobrecarga
		// que javac.
		d, err := e.tipoEstimado(v.dentro)
		if err != nil {
			return "", err
		}
		if len(d) == 1 && strings.IndexByte("IJFD", d[0]) >= 0 {
			return d, nil
		}
		if len(d) == 1 && strings.IndexByte("BCS", d[0]) >= 0 {
			return "I", nil
		}
		return "", fmt.Errorf("linea %d: la negacion de un %s no esta medida (solo los numeros)", v.donde.linea, d)
	case joperador:
		// `new File(dir, apk.name + ".apk")`: el `+` es concatenación en cuanto uno de sus lados es
		// cadena (JLS 15.18.1), y el tipo del resultado es `Ljava/lang/String;`, que es lo que necesita
		// la sobrecarga `File(File,String)`. Estimar y emitir coinciden: `exp` manda el nodo a
		// `concatenar`, que devuelve el mismo descriptor y no cambia el orden de evaluación. Una cuenta
		// sin cadena (`int + int`) no se estima: sigue cayendo en el rechazo de siempre, que es lo que
		// hace javac al no compilarla donde se espera una cadena.
		if estimaConcatEnArgumento && v.op == "+" && e.esConcatenacion(v) {
			return "Ljava/lang/String;", nil
		}
		return "", fmt.Errorf("%s: el tipo de una expresion %T no se puede estimar sin emitirla", posDeExpr(x).sitio(), x)
	case jllamada:
		// una llamada encadenada (`app.apks.get(0)`) como argumento de otra: la vuelta sale del tipo
		// estático del receptor, sin emitirlo (`vueltaSinEmitir`)
		if d, ok, err := e.vueltaSinEmitir(v); ok {
			return d, err
		}
		// una llamada sin receptor escrito: la vuelta de su unica candidata en esa aridad (medido al
		// pasar `local()` y `cuenta(2L, 3L)` como argumento de `imprime` en la sonda `L`, parrafo 74).
		// Dentro de una anonima la candidata puede ser un metodo heredado del exterior
		// (`getApplicationContext()` en `MainActivity.java:179`), que se busca en la cadena de clases
		// que la encierran sin emitir nada (`candidatasSinReceptor`).
		if v.base != nil || v.super {
			// una llamada estática calificada por el nombre del tipo (`Intent.createChooser(...)`): el
			// receptor no es una instancia que emitir, y su vuelta se lee del `android.jar` sin escribir
			// ninguna instrucción (`vueltaEstaticaCualificada`).
			if d, ok, err := e.vueltaEstaticaCualificada(v); ok {
				return d, err
			}
			return "", fmt.Errorf("%s: no se puede estimar el tipo de %s() sin emitir el receptor de la llamada", v.donde.sitio(), v.nombre)
		}
		cands, err := e.candidatasSinReceptor(v.nombre, v.args)
		if err != nil {
			return "", fmt.Errorf("%s: al estimar el tipo de %s(), %w", v.donde.sitio(), v.nombre, err)
		}
		if len(cands) != 1 {
			return "", fmt.Errorf("%s: no se puede estimar el tipo de %s(): hay %d candidatas con %d argumentos y el tipo lo decide la sobrecarga", v.donde.sitio(), v.nombre, len(cands), len(v.args))
		}
		return vueltaDe(cands[0].desc), nil
	case jlambda:
		// una lambda no tiene descriptor propio: lo pone el parametro formal que la recibe (JLS
		// 15.27.3), y por eso la resolucion de la llamada que la lleva va por su propio camino
		return "", fmt.Errorf("%s: una lambda no tiene tipo propio sin el parametro formal que la recibe", v.donde.sitio())
	}
	return "", fmt.Errorf("%s: el tipo de una expresion %T no se puede estimar sin emitirla", posDeExpr(x).sitio(), x)
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
		n := 1
		if len(v.dimensiones) > 1 {
			n = len(v.dimensiones)
		}
		return strings.Repeat("[", n) + d, true
	case jnegativo:
		// `-x` tiene el ancho de su operando, y el estrecho (`char`, `byte`, `short`) se niega como
		// entero: `-c` es `iload_0; ineg` y deja `I` (medido con javac 17). Así `l + (-x)` sabe que
		// tiene que ensanchar el lado de la negación antes de sumar.
		d, ok := e.tipoPrevisto(v.dentro)
		if !ok {
			return "", false
		}
		if d == "C" || d == "B" || d == "S" {
			return "I", true
		}
		if d == "I" || d == "J" || d == "F" || d == "D" {
			return d, true
		}
		return "", false
	case jasignExp:
		// `(x = f())` como valor de una comparación: la pila lleva el tipo del local, y saberlo antes
		// de emitir es lo que elige entre `if_icmp*` y el `lcmp`/`fcmpl`/`dcmpl` de la comparación
		// ancha. Es lo que mide el caso `long x; while ((x = f()) > 0)` de
		// `javaemisor_asignacion_valor_test.go` contra javac 17.
		acc, esAcceso := v.destino.(jacceso)
		if !esAcceso {
			return "", false
		}
		segs, base := aplanar(acc)
		if base != nil || len(segs) != 1 {
			return "", false
		}
		lv, ok := e.locales[segs[0]]
		if !ok {
			return "", false
		}
		return lv.desc, true
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
		if interno, resto, ok := e.ctx.tipoDeReferencia(segs); ok {
			d, err := e.descDeAccesoEstatico(interno, resto)
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
		cands, err := e.candidatasSinReceptor(v.nombre, v.args)
		if err != nil || len(cands) != 1 {
			return "", false
		}
		return vueltaDe(cands[0].desc), true
	}
	return "", false
}

// expA: emite la expresion y, si el hueco que hay que llenar es de otro tipo, escribe la conversion de
// javac en su sitio: `i2l` (0x85) para un entero que llena un `long`, `i2f`/`i2d` para los flotantes y
// la subida de `byte`/`short`/`char`, que en la pila ya es un entero, sin opcode de por medio.
func (e *emisor) expA(x jexpr, deseado string) (string, error) {
	// La promocion de un literal se hace **sobre la constante**, antes de elegir su forma: medido en el
	// parrafo 85, `d + 1` es `dconst_1`, `l + 2` es `ldc2_w 2l` y `f * -2` es `ldc -2.0f`, ninguno con la
	// conversion del parrafo 77 detras. Si el operando no es una constante plegable, la conversion si va
	// en su sitio (`d + i` con `i` variable es `iload_0; i2d`).
	if len(deseado) == 1 && strings.IndexByte("IJFD", deseado[0]) >= 0 {
		if c, ok := e.plegarConstante(x); ok && nivelNum(c.tipo) <= nivelNum(deseado[0]) {
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
		if c, ok := e.plegarConstante(x); ok && (c.tipo == 'I' || c.tipo == 'J' || c.tipo == 'C') {
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
	// el literal `null` no trae tipo propio: dentro de un hueco de referencia es un `null` de ese
	// tipo, que es lo que queda en la pila con el mismo `aconst_null` que ya se emitio.
	if esLiteralNulo(x) && esTipoDeReferencia(deseado) {
		return deseado, nil
	}
	if d == deseado {
		return d, nil
	}
	// boxing: javac 17 escribe `Integer.valueOf` (y el resto de envoltorios) antes de la llamada a la
	// sobrecarga que toma un Objeto; medido con `javap -c` sobre `box(7)`.
	if box, ok := boxingDe[d]; ok && box == deseado {
		dueño := box[1 : len(box)-1]
		e.invoca(opInvokestat, dueño, "valueOf", "("+d+")"+box, 1)
		return box, nil
	}
	// boxing y después ensanchamiento de referencia: el hueco es `Object` (o `Number`, o el elemento
	// `Object` de un varargs) y lo que sale es un primitivo. javac escribe el mismo `Integer.valueOf` y
	// deja sobre la pila el envoltorio; el descriptor que se devuelve es el del envoltorio, que es el
	// que de verdad hay arriba, y quien compara lo acepta por `asignable` (§194). El hueco tiene que
	// ser de referencia: `asignable(Byte, "I")` ya es cierto por el desenvuelto, y envolver un `byte`
	// para dejarlo donde va un `int` no es lo que hace javac.
	if box, ok := boxingDe[d]; ok && esTipoDeReferencia(deseado) && e.ctx.asignable(box, deseado) {
		dueño := box[1 : len(box)-1]
		e.invoca(opInvokestat, dueño, "valueOf", "("+d+")"+box, 1)
		return box, nil
	}
	// Desenvuelto, que es la vuelta del boxing: el hueco es primitivo y lo que hay en la pila es un
	// envoltorio. javac escribe el `<x>Value()` del envoltorio y, si el hueco es más ancho, la
	// conversión detrás: `Integer.intValue:()I` para un `int` y `Integer.intValue:()I; i2l` para un
	// `long` (medido con `javap -c` sobre `g(i)` y `h(i)`).
	if e.desenvuelve(d, deseado) {
		prim := primDeEnvoltorio[d]
		e.invoca(opInvokevirt, d[1:len(d)-1], metodoDesenvoltura[prim], "()"+prim, 1)
		base := prim
		if base == "B" || base == "C" || base == "S" {
			base = "I" // la subida de byte, short y char a int no escribe ninguna conversion
		}
		if base != deseado {
			e.convierte(base, deseado)
		}
		return deseado, nil
	}
	// `boolean` no ensancha a ningún tipo numérico (JLS 5.1.2): `long x = true` y `f(true)` con
	// `f(long)` no son Java. Si un hueco primitivo distinto de `Z` llega aquí con un `boolean` en la
	// pila, se rechaza con su línea en vez de emitir el `i2l`/`i2d` que deja el saldo descuadrado —o,
	// peor, un `.class` que ART ejecuta sobre un programa que javac no compila—. El interruptor solo
	// existe para la falsificación.
	if d == "Z" && !booleanEnsanchaEnArgumentos && len(deseado) == 1 && deseado[0] != 'Z' {
		return "", fmt.Errorf("linea %d: el argumento es boolean y el parámetro %s no lo acepta: boolean no ensancha a ningún tipo numérico (JLS 5.1.2)", lineaDeExpr(x), nombrePrimitivo(deseado))
	}
	// Enschar un `char` a `int` no escribe nada: `int n = c;` es `iload_0; istore_1` y `aEntero(char)` es
	// `iload_0; ireturn`, medidos en el parrafo 87. Lo mismo vale para `byte` y `short`: la sonda
	// javac 17 de `Estrecha.e` es `iload_0; iload_1; iadd`, sin conversiones.
	if subeAIntParaAncho(d) && deseado == "I" {
		return "I", nil
	}
	// `byte`, `short` y `char` viven en la pila como enteros de una palabra (JVMS 2.11.1): su subida a
	// `int` no escribe nada y hacia un ancho el opcode es el mismo que desde `int`, sin
	// `i2b`/`i2s`/`i2c` de por medio. Medido con javac 17: `f(long)` con un `byte b` es `iload; i2l`,
	// con un `short` lo mismo, y `g(double)` con un `char c` `iload; i2d`. `boolean` no entra (el
	// guardia de arriba lo rechaza).
	if subeAIntParaAncho(d) && len(deseado) == 1 &&
		strings.IndexByte("JFD", deseado[0]) >= 0 && e.convierte("I", deseado) {
		return deseado, nil
	}
	if len(d) == 1 && len(deseado) == 1 && e.convierte(d, deseado) {
		return deseado, nil
	}
	// `check-cast`: el hueco es de referencia y la pila trae una referencia MAS GENERAL. Es lo que
	// javac escribe detrás de una llamada borrada: medido con javac 17 sobre `AppAdapter.java:73` del
	// zip de ApkShare, `return apps_filter.get(position)` con la vuelta declarada `App` sale
	// `aload_0; getfield; iload_1; invokeinterface List.get:(I)Ljava/lang/Object;` y después
	// `checkcast #44 // class share/apps/util/App;`, y lo mismo en `App info = apps_filter.get(...)`
	// (la 43) y en `TextView txt1 = convertView.findViewById(...)` (la 47). Sin el `check-cast` la
	// pila queda en `Object` donde la firma declara `App` y el verificador de ART rechaza el método.
	// La dirección es la única que lo dispara: `Object o = list.get(0)` no lleva nada (el hueco ES
	// `Object`), y `List x = new ArrayList<>()` tampoco (ahí el valor ya es más estrecho y esto no
	// escribe conversión; la asignación de subtipo a campo la sigue mediando `cabeEn`).
	if esTipoDeReferencia(d) && esTipoDeReferencia(deseado) && d != deseado && e.ctx.asignableEstricto(deseado, d) && insertaCheckcast {
		e.checkcastDe(deseado)
		return deseado, nil
	}
	return d, nil
}

// insertaCheckcast es el interruptor de la sonda del `check-cast` de `expA`. Apagado, el emisor deja
// sobre la pila la referencia borrada —`Object` donde la firma declara `App`—, que es el defecto que
// el verificador rechaza; lo usan las falsificaciones de `javaemisor_appadapter_cast_test.go`.
// Encendido en el binario que se publica.
var insertaCheckcast = true

// checkcastDe: el `check-cast` de un descriptor de referencia, con la forma que ya se midió en el
// `for` mejorado (párrafo 129) y en `X.class`: la clase va al pool por su nombre interno —o por el
// descriptor entero si es un arreglo— y el índice ocupa dos bytes. Media palabra dentro y media
// fuera, así que no mueve el alto de la pila.
func (e *emisor) checkcastDe(desc string) {
	interno := desc
	if desc[0] == 'L' {
		interno = desc[1 : len(desc)-1]
		e.usar(interno)
	}
	idx := e.pool.clase(interno)
	e.emite(opCheckcast, byte(idx>>8), byte(idx))
}

// codigosConversion: la conversion numerica entre los tipos medidos, con la palabra que la escribe.
// La rejilla sale de `i2l` 0x85 (parrafo 74) y de las leidas en `FyD` (parrafo 77): `f2d` en `por`,
// `i2d` en `divide` y `deEntero`, `d2i` en `aEntero`, `d2f` en `aFlotante`.
var codigosConversion = map[[2]byte]byte{
	{'I', 'J'}: 0x85, {'I', 'F'}: 0x86, {'I', 'D'}: 0x87,
	{'J', 'I'}: 0x88, {'J', 'F'}: 0x89, {'J', 'D'}: 0x8a,
	{'F', 'I'}: 0x8b, {'F', 'J'}: 0x8c, {'F', 'D'}: 0x8d,
	{'D', 'I'}: 0x8e, {'D', 'J'}: 0x8f, {'D', 'F'}: 0x90,
}

// desenvuelve: si el envoltorio `desde` puede llenar el hueco primitivo `hasta` con su `<x>Value()` y,
// cuando hace falta, una conversion detras. `byte`, `short` y `char` suben a `int` sin escribir nada
// (JVMS 2.11.1) y de ahi a los anchos; `Boolean` solo vale como `Z` y no ensancha a nada.
func (e *emisor) desenvuelve(desde, hasta string) bool {
	prim, ok := primDeEnvoltorio[desde]
	if !ok || len(hasta) != 1 {
		return false
	}
	if hasta == prim {
		return true
	}
	base := prim
	if base == "B" || base == "C" || base == "S" {
		base = "I"
	}
	if base == hasta {
		return true
	}
	_, ok = codigosConversion[[2]byte{base[0], hasta[0]}]
	return ok
}

// convierte: escribe la conversion numerica entre los tipos medidos. Devuelve false si el par no esta.
func (e *emisor) convierte(de, a string) bool {
	peso := map[byte]int{'I': 1, 'F': 1, 'J': 2, 'D': 2}
	c, ok := codigosConversion[[2]byte{de[0], a[0]}]
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

// comparaArregloConNulo es el interruptor de la sonda del arreglo contra null: apagado, un `[` deja de
// contar como referencia y `condicionBranca` vuelve al rechazo de antes. Lo usa la falsificación de
// `javaemisor_campo_local_test.go`; encendido en el binario que se publica.
var comparaArregloConNulo = true

// condicionBooleanaSuelta es el interruptor de la sonda del booleano suelto en una condicion: apagado,
// `if (llama())` vuelve al rechazo de antes («una condicion que no sea una comparacion no esta
// medida»), que es el error literal que daba ApkShare en `AppAdapter.java:64`. Encendido en el binario
// que se publica; lo usa la falsificacion de `javaemisor_appadapter_test.go`.
var condicionBooleanaSuelta = true

// esComparacion: si la expresion es ya una comparacion de las que `condicionBranca` sabe branquear
// por sí sola. Lo usa la rama de la `!` para no robarle el caso a la tabla de abajo: `!(a == b)` se
// pliega en la comparación, como hace javac, y no se emite como un booleano materializado seguido de
// `ifne`; el salto que se escribe es el opuesto del invertido de la tabla (`opuestoDeSalto`).
func esComparacion(x jexpr) bool {
	op, es := x.(joperador)
	if !es {
		return false
	}
	switch op.op {
	case "==", "!=", "<", ">=", ">", "<=":
		return true
	}
	return false
}

// opuestoDeSalto: el salto condicional que branquea lo contrario. `condicionBranca` devuelve, para
// cada comparacion, el salto que aplica javac con la condicion **invertida** (salta cuando NO se
// cumple); la `!` de una comparacion entera necesita el salto que salta cuando SÍ se cumple, que es
// justo el opuesto. Medido con javac 17 en la sonda de la negación: `if (!(a == b))` es `if_icmpne`,
// `if (!(x != null))` es `ifnull` y `if (!(d > 0.0))` es `dcmpl; ifgt`
// (`TestLaNegacionDeUnBooleanoSueltoNoMaterializa`).
var opuestoDeSalto = map[byte]byte{
	opIFEQ: opIFNE, opIFNE: opIFEQ,
	opIFLT: opIFGE, opIFGE: opIFLT,
	opIFGT: opIFLE, opIFLE: opIFGT,
	opIFICMPEQ: opIFICMPNE, opIFICMPNE: opIFICMPEQ,
	opIFICMPLT: opIFICMPGE, opIFICMPGE: opIFICMPLT,
	opIFICMPGT: opIFICMPLE, opIFICMPLE: opIFICMPGT,
	opIFACMPEQ: opIFACMPNE, opIFACMPNE: opIFACMPEQ,
	opIFNULL: opIFNONNULL, opIFNONNULL: opIFNULL,
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
		// `if (!b)` con `b` un booleano suelto: javac escribe la expresion y salta con `ifne`, que es
		// el `ifeq` de `if (b)` invertido. La `!` de una comparacion (`!(a == b)`) NO se materializa:
		// javac la pliega y salta con el opuesto de la tabla, que es lo que se hace justo debajo.
		if !esComparacion(neg.dentro) {
			if !condicionBooleanaSuelta {
				return 0, 0, fmt.Errorf("linea %d: una condicion que no sea una comparacion no esta medida (sobre un booleano javac usa `ifeq`, y eso no lo leimos)",
					lineaDeExpr(cond))
			}
			if _, esLit := neg.dentro.(jlit); esLit {
				return 0, 0, fmt.Errorf("linea %d: una condicion que no sea una comparacion no esta medida sobre un literal booleano: `if (true)` lo pliega javac y ese plegado no lo leimos", lineaDeExpr(cond))
			}
			d, err := e.expA(neg.dentro, "Z")
			if err != nil {
				return 0, 0, err
			}
			if d != "Z" {
				return 0, 0, fmt.Errorf("linea %d: la condicion es %s y no un booleano", lineaDeExpr(cond), d)
			}
			return opIFNE, 1, nil
		}
		// `!(a == b)` y compania: los operandos son los de la comparacion y el salto es el opuesto del
		// invertido que devuelve la tabla, que es el que salta cuando la comparacion SI se cumple.
		op, consume, err := e.condicionBranca(neg.dentro)
		if err != nil {
			return 0, 0, err
		}
		contrario, hay := opuestoDeSalto[op]
		if !hay {
			return 0, 0, fmt.Errorf("linea %d: el salto %#02x de una comparacion negada no tiene opuesto medido", lineaDeExpr(cond), op)
		}
		return contrario, consume, nil
	}
	op, esComp := cond.(joperador)
	if !esComp {
		// El booleano suelto: `if (b)`, `if (isEmpty())`, `if (app.toString().contains(q))`. javac
		// escribe la expresion —la carga del local o la llamada— y branquea con `ifeq` **sin
		// invertir**, porque en el hueco no hay nada que comparar: medido en ApkShare
		// `AppAdapter.java:64` (`if (... .contains(...))`), con `javap -c` sobre la misma clase. Un
		// literal booleano se queda rechazado: javac pliega `if (true)` y borra la branca entera, y
		// aqui no hay plegado de condiciones.
		if _, esLit := cond.(jlit); esLit {
			return 0, 0, fmt.Errorf("linea %d: una condicion que no sea una comparacion no esta medida sobre un literal booleano: `if (true)` lo pliega javac y ese plegado no lo leimos", lineaDeExpr(cond))
		}
		if !condicionBooleanaSuelta {
			return 0, 0, fmt.Errorf("linea %d: una condicion que no sea una comparacion no esta medida (sobre un booleano javac usa `ifeq`, y eso no lo leimos)",
				lineaDeExpr(cond))
		}
		d, err := e.expA(cond, "Z")
		if err != nil {
			return 0, 0, fmt.Errorf("linea %d: una condicion que no sea una comparacion no esta medida (%v)", lineaDeExpr(cond), err)
		}
		if d != "Z" {
			return 0, 0, fmt.Errorf("linea %d: la condicion es %s y no un booleano", lineaDeExpr(cond), d)
		}
		return opIFEQ, 1, nil
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
		// un arreglo es una referencia: `info.splitSourceDirs == null` compara `[Ljava/lang/String;`
		// con null y javac branquea con `ifnonnull` sobre el objeto (§244)
		if !esObjeto(d) || (!comparaArregloConNulo && d[0] == '[') {
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
		// el `goto` que salta el `else` lleva la línea del `}` del `then` cuando ese bloque declaró
		// algún local con valor: es la fila que javac escribe al cerrar el ámbito, en el pc donde va el
		// `goto` (medido en el párrafo de la sonda `ElseGoto`). Sin local con valor javac no la escribe
		// y el `goto` se queda con la línea de la última sentencia del `then`; cuando el `then` no es
		// un bloque (`if (c) b(2);`) tampoco hay fila.
		cierre := 0
		if anotaLineaDelElse {
			if _, esBloque := v.entonces.(jbloque); esBloque {
				cierre = e.filaDeCierre
			}
		}
		if cierre > 0 {
			e.marcaLinea(cierre)
		}
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

// porCada: `for (T v : coleccion)`, el `for` con dos puntos. javac lo baja de dos maneras, las dos
// medidas contra javac 17 `--release 8` en el §225:
//
//   - sobre un arreglo, copia el arreglo a un local sintético y su largo a otro, y cuenta con un
//     índice: `astore arr$; aload arr$; arraylength; istore len$; iconst_0; istore i$` y después
//     `iload i$; iload len$; if_icmpge fin`, con `aload arr$; iload i$; *aload; *store v` delante del
//     cuerpo y `iinc i$, 1` al pie. El `iinc` lleva la línea del `for` y el `goto` la hereda.
//   - sobre un `Iterable`, llama `iterator()` una vez y en cada vuelta `hasNext()`/`next()` con el
//     `checkcast` del elemento. El `goto` de vuelta lleva la línea del cierre del cuerpo, no la del
//     `for`, y no hay `checkcast` cuando el elemento declarado es `Object`.
//
// `continue` salta al `iinc` (camino del arreglo) o a la condición (camino del iterador, donde javac
// pliega el paso vacío), y `break` al final: es la misma pila de contextos que el `for` clásico.
func (e *emisor) porCada(v jporCada) error {
	declarado, err := e.ctx.descriptorDe(v.tipo)
	if err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	e.marcaLinea(v.donde.linea)
	dCol, err := e.exp(v.coleccion)
	if err != nil {
		return err
	}
	if len(dCol) > 0 && dCol[0] == '[' {
		return e.porCadaArreglo(v, dCol, declarado)
	}
	return e.porCadaIterable(v, dCol, declarado)
}

// porCadaArreglo: la forma del arreglo, medida en el §225 sobre `String[]` e `int[]`. Los tres locales
// sintéticos van en el orden de javac —arreglo, largo, índice— y la variable del `for` después; el
// `iinc` del pie lleva la línea del `for`, que es lo que deja la LineNumberTable igual.
func (e *emisor) porCadaArreglo(v jporCada, dCol, declarado string) error {
	elem := dCol[1:]
	if elem == "" {
		return fmt.Errorf("linea %d: un arreglo sin tipo de elemento no esta medido", v.donde.linea)
	}
	if elem != declarado && !((elem[0] == 'L' || elem[0] == '[') && e.ctx.asignable(elem, declarado)) {
		return fmt.Errorf("linea %d: el arreglo es %s y el elemento se declara %s", v.donde.linea, dCol, declarado)
	}
	arr := e.reserva("", dCol)
	e.guardaDe(arr, dCol)
	largo := e.reserva("", "I")
	e.cargaDe(arr, dCol)
	e.emite(0xbe) // arraylength
	e.baja(1)
	e.sube()
	e.guardaDe(largo, "I")
	indice := e.reserva("", "I")
	e.cargaEntero(0)
	e.guardaDe(indice, "I")
	cabeza := e.nuevaEtiqueta()
	fin := e.nuevaEtiqueta()
	paso := e.nuevaEtiqueta()
	e.cierra(cabeza)
	e.cargaDe(indice, "I")
	e.cargaDe(largo, "I")
	e.saltoEmite(opIFICMPGE, fin, 2)
	e.quiebres = append(e.quiebres, fin)
	e.continuar = append(e.continuar, paso)
	e.cargaDe(arr, dCol)
	e.cargaDe(indice, "I")
	if err := e.cargaDeArreglo(elem); err != nil {
		e.quiebres = e.quiebres[:len(e.quiebres)-1]
		e.continuar = e.continuar[:len(e.continuar)-1]
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	variable := e.reserva(v.nombre, declarado)
	e.guardaDe(variable, declarado)
	err := e.stm(v.cuerpo)
	e.quiebres = e.quiebres[:len(e.quiebres)-1]
	e.continuar = e.continuar[:len(e.continuar)-1]
	if err != nil {
		return err
	}
	e.cierra(paso)
	e.marcaLinea(v.donde.linea)
	if indice > 255 {
		return fmt.Errorf("linea %d: el indice del `for` no cabe en el `iinc`", v.donde.linea)
	}
	e.emite(0x84, byte(indice), 1) // iinc
	e.saltoEmite(opGOTO, cabeza, 0)
	if err := e.resuelveAtras(cabeza); err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	e.cierra(fin)
	return nil
}

// porCadaIterable: la forma del `Iterable`, medida en el §225 sobre `List<String>` y `ArrayList<Apk>`.
// `iterator()` se resuelve contra el tipo estático de la colección, así que sale `invokeinterface` o
// `invokevirtual` según sea; `hasNext`/`next` son de `java/util/Iterator` y siempre de interface. El
// `checkcast` del elemento no se escribe cuando el tipo declarado es `Object`, y el `goto` de vuelta
// lleva la línea del cierre del cuerpo.
func (e *emisor) porCadaIterable(v jporCada, dCol, declarado string) error {
	if len(dCol) == 0 || dCol[0] != 'L' {
		return fmt.Errorf("linea %d: un `for` con dos puntos sobre %s no esta medido", v.donde.linea, dCol)
	}
	if len(declarado) == 0 || (declarado[0] != 'L' && declarado[0] != '[') {
		return fmt.Errorf("linea %d: un `for` con dos puntos sobre un `Iterable` con elemento %s no esta medido", v.donde.linea, declarado)
	}
	receptor := dCol[1 : len(dCol)-1]
	cand, err := e.ctx.buscarMetodo(receptor, "iterator", 0, nil)
	if err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	accesos, err := e.ctx.accesosDe(receptor)
	if err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	opcode := byte(opInvokevirt)
	if accesos&accInterface != 0 {
		opcode = opInvointf
	}
	dIt := vueltaDe(cand.desc)
	if dIt != "Ljava/util/Iterator;" {
		return fmt.Errorf("linea %d: `iterator()` de %s devuelve %s y no un `Iterator`", v.donde.linea, receptor, dIt)
	}
	e.invoca(opcode, receptor, "iterator", cand.desc, 1)
	it := e.reserva("", dIt)
	e.guardaDe(it, dIt)
	cabeza := e.nuevaEtiqueta()
	fin := e.nuevaEtiqueta()
	e.cierra(cabeza)
	e.cargaDe(it, dIt)
	e.invoca(opInvointf, "java/util/Iterator", "hasNext", "()Z", 1)
	e.saltoEmite(opIFEQ, fin, 1)
	e.quiebres = append(e.quiebres, fin)
	e.continuar = append(e.continuar, cabeza)
	e.cargaDe(it, dIt)
	e.invoca(opInvointf, "java/util/Iterator", "next", "()Ljava/lang/Object;", 1)
	if declarado != "Ljava/lang/Object;" {
		interno := declarado
		if declarado[0] == 'L' {
			interno = declarado[1 : len(declarado)-1]
			e.usar(interno)
		}
		idx := e.pool.clase(interno)
		e.emite(opCheckcast, byte(idx>>8), byte(idx))
	}
	variable := e.reserva(v.nombre, declarado)
	e.guardaDe(variable, declarado)
	err = e.stm(v.cuerpo)
	e.quiebres = e.quiebres[:len(e.quiebres)-1]
	e.continuar = e.continuar[:len(e.continuar)-1]
	if err != nil {
		return err
	}
	e.marcaLinea(v.lineaCierre)
	e.saltoEmite(opGOTO, cabeza, 0)
	if err := e.resuelveAtras(cabeza); err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	e.cierra(fin)
	return nil
}

// cargaDeArreglo: el `*aload` del elemento, con la cuenta de pila del §66. Es la tabla de `leeIndice`,
// que no se toca para no mover lo ya medido.
func (e *emisor) cargaDeArreglo(elem string) error {
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
		e.emite(0x34) // caload, medido en `Letras.lee` (parrafo 87)
	case 'L', '[':
		e.emite(0x32)
	default:
		return fmt.Errorf("un arreglo de %s no esta medido", elem)
	}
	e.baja(2)
	if esAncho(elem) {
		e.subeDos()
	} else {
		e.sube()
	}
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

// cambioCadena baja un `switch (String)` a la forma que javac mide: compara `hashCode` y luego
// `equals`, guarda el índice del brazo en un local y deja que `cambio` emita el switch de enteros.
// Aquí se compara directamente con `equals` (sin `hashCode`), equivalente y más corto; el dex que
// se compara con javac es el del switch de índices, que sí es idéntico.
func (e *emisor) cambioCadena(v jswitch) error {
	d, err := e.exp(v.selector)
	if err != nil {
		return err
	}
	if d != "Ljava/lang/String;" {
		return fmt.Errorf("linea %d: un switch de cadenas necesita String y recibió %s", v.donde.linea, d)
	}
	cad := e.reserva("", d)
	e.guardaDe(cad, d)
	indice := e.reserva("", "I")
	e.cargaEntero(-1)
	e.guardaDe(indice, "I")
	for i, c := range v.casos {
		siguiente := e.nuevaEtiqueta()
		e.cargaDe(cad, d)
		if _, err := e.exp(jlit{tipoJ: 's', texto: c.cadena, donde: c.donde}); err != nil {
			return err
		}
		e.invoca(opInvokevirt, "java/lang/String", "equals", "(Ljava/lang/Object;)Z", 1)
		e.saltoEmite(0x99, siguiente, 1) // ifeq: esta cadena no es la clave
		e.cargaEntero(int32(i))
		e.guardaDe(indice, "I")
		e.cierra(siguiente)
	}
	e.cargaDe(indice, "I")
	entero := jswitch{defecto: v.defecto, orden: v.orden, donde: v.donde}
	for i, c := range v.casos {
		entero.casos = append(entero.casos, jcaso{
			valor: int32(i), cuerpo: c.cuerpo, rompe: c.rompe, donde: c.donde,
		})
	}
	return e.cambio(entero)
}

// cambio: `switch` del fuente a `tableswitch` o `lookupswitch`, con la forma medida en el §64: el
// relleno va del opcode al siguiente múltiplo de cuatro **contado desde el principio del `Code`**, y
// cada destino es un entero de 32 bits con signo desde el principio de la instrucción. La elección de
// forma es de este emisor —`tableswitch` cuando el rango no pasa del doble de los casos—: el criterio
// exacto de javac 21 se sondeó y no es una desigualdad sencilla (con dos casos y rango 4 elige
// `lookupswitch`, con cuatro casos y rango 10 elige `tableswitch`), y las dos formas son correctas.
func (e *emisor) cambio(v jswitch) error {
	e.marcaLinea(v.donde.linea)
	if v.esCadena {
		return e.cambioCadena(v)
	}
	if v.selector != nil {
		if _, err := e.exp(v.selector); err != nil {
			return err
		}
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

// resuelveAtras valida el `goto` del final de un bucle, que apunta a una etiqueta **ya cerrada**. El
// desplazamiento no se escribe aquí: `resuelveSaltos` lo coloca con la forma que pida el método
// entero, corta o `goto_w` de 32 bits, cuando ya no se va a mover ningún byte.
func (e *emisor) resuelveAtras(id int) error {
	if _, ok := e.etiquetas[id]; !ok {
		return fmt.Errorf("un salto hacia atras a una etiqueta que no se cerro")
	}
	// el desplazamiento se escribe en `resuelveSaltos`, con el ancho que pida el método entero:
	// una vuelta de bucle de más de 32 KB cabe igual que cualquier otra rama larga.
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

// incrementoValor: `++`/`--` usados como valor, medido en el párrafo 151 contra javap 17. En un
// local javac cuenta con `iinc` y pone la carga antes (postfijo) o después (prefijo). En un campo
// escribe `dup`/`dup_x1` alrededor de la cuenta: `dup_x1` antes de sumar deja el valor anterior y
// `dup` después deja el nuevo. Solo entero de esta clase: el resto sigue rechazado por nombre.
func (e *emisor) incrementoValor(v jincr) (string, error) {
	e.marcaLinea(v.donde.linea)
	if acc, esAcceso := v.destino.(jacceso); esAcceso {
		segs, base := aplanar(acc)
		if base == nil && len(segs) == 1 {
			if lv, ok := e.locales[segs[0]]; ok {
				if !esEnteroDesc(lv.desc) {
					return "", fmt.Errorf("linea %d: `++`/`--` como valor sobre %s, que no es un entero", v.donde.linea, lv.desc)
				}
				if v.delta < -128 || v.delta > 127 {
					return "", fmt.Errorf("linea %d: la cuenta de `++`/`--` como valor no cabe en `iinc`", v.donde.linea)
				}
				if lv.indice > 255 {
					return "", fmt.Errorf("linea %d: el local %s no cabe en el índice de un `iinc`", v.donde.linea, segs[0])
				}
				if v.prefijo {
					if err := e.incremento(v); err != nil {
						return "", err
					}
					e.cargaDe(lv.indice, lv.desc)
				} else {
					e.cargaDe(lv.indice, lv.desc)
					if err := e.incremento(v); err != nil {
						return "", err
					}
				}
				return lv.desc, nil
			}
		}
	}
	acc, esAcceso := v.destino.(jacceso)
	if !esAcceso {
		return "", fmt.Errorf("linea %d: `++`/`--` como valor solo sobre una variable o un campo", v.donde.linea)
	}
	segs, base := aplanar(acc)
	if base != nil || len(segs) != 1 {
		return "", fmt.Errorf("linea %d: `++`/`--` como valor sobre un campo ajeno no está medido", v.donde.linea)
	}
	dueño, cr, encontrado := e.duenioDelCampo(segs[0])
	if !encontrado {
		return "", fmt.Errorf("linea %d: %s no es una variable local ni un campo conocido", v.donde.linea, segs[0])
	}
	if cr.desc != "I" {
		return "", fmt.Errorf("linea %d: `++`/`--` como valor sobre %s, que es %s: solo entero",
			v.donde.linea, segs[0], cr.desc)
	}
	idx := e.pool.campo(dueño, segs[0], cr.desc)
	op := byte(opIadd)
	delta := v.delta
	if delta < 0 {
		op = opIsub
		delta = -delta
	}
	if cr.acceso&accStatic != 0 {
		d, err := e.exp(v.destino)
		if err != nil {
			return "", err
		}
		if d != "I" {
			return "", fmt.Errorf("linea %d: el campo %s no es entero", v.donde.linea, segs[0])
		}
		if v.prefijo {
			e.cargaEntero(delta)
			e.emite(op)
			e.baja(1)
			e.emite(opDup)
			e.sube()
		} else {
			e.emite(opDup)
			e.sube()
			e.cargaEntero(delta)
			e.emite(op)
			e.baja(1)
		}
		e.putstatic(idx)
		return "I", nil
	}
	if e.estatico {
		return "", fmt.Errorf("linea %d: `++`/`--` sobre un campo de instancia sin `this`", v.donde.linea)
	}
	e.cargaDelDuenio(dueño)
	e.emite(opDup)
	e.sube()
	e.getfieldDe(idx, cr.desc)
	// En el campo de instancia javac usa `dup_x1` en los dos sentidos: antes de la cuenta en el
	// postfijo (deja el valor anterior) y después en el prefijo (deja el nuevo), porque el receptor
	// queda debajo del valor.
	if v.prefijo {
		e.cargaEntero(delta)
		e.emite(op)
		e.baja(1)
		e.emite(opDupX1)
		e.sube()
	} else {
		e.emite(opDupX1)
		e.sube()
		e.cargaEntero(delta)
		e.emite(op)
		e.baja(1)
	}
	e.putfield(idx)
	return "I", nil
}

// asignacionValor: `x = valor` usado como **valor** (JLS 15.26.1), la forma de
// `while ((length = in.read(buffer)) > 0)`. La expresion vale lo asignado, asi que javac escribe el
// valor y lo deja duplicado antes de guardarlo: medido con javac 17 sobre la linea 187 de
// `MainActivity.java` del zip de ApkShare, `aload_1; aload_2; invokevirtual read([B)I; dup; istore 4;
// iconst_0; if_icmple`. En un `long` el duplicado es `dup2` y el guardado ocupa dos palabras.
// Solo se mide sobre una variable local: en un campo el duplicado es `dup_x1`/`dup2_x1` con el receptor
// debajo y en un elemento de arreglo `dup_x2`, formas que nadie ha medido todavia y que se rechazan por
// su nombre. El valor va por `expA` contra el descriptor del local, igual que la asignacion sentencia.
func (e *emisor) asignacionValor(v jasignExp) (string, error) {
	e.marcaLinea(v.donde.linea)
	acc, esAcceso := v.destino.(jacceso)
	if !esAcceso {
		return "", fmt.Errorf("linea %d: una asignacion como valor solo esta medida sobre una variable local", v.donde.linea)
	}
	segs, base := aplanar(acc)
	if base != nil || len(segs) != 1 {
		return "", fmt.Errorf("linea %d: una asignacion como valor solo esta medida sobre una variable local, y el destino es %s",
			v.donde.linea, strings.Join(segs, "."))
	}
	lv, ok := e.locales[segs[0]]
	if !ok {
		return "", fmt.Errorf("linea %d: una asignacion como valor solo esta medida sobre una variable local, y %s no lo es",
			v.donde.linea, segs[0])
	}
	dv, err := e.expA(v.valor, lv.desc)
	if err != nil {
		return "", err
	}
	if dv[0] != lv.desc[0] {
		return "", fmt.Errorf("linea %d: %s es %s y se le asigna un %s", v.donde.linea, segs[0], lv.desc, dv)
	}
	if esAncho(lv.desc) {
		e.emite(opDup2)
	} else {
		e.emite(opDup)
	}
	e.subePor(lv.desc)
	e.guardaDe(lv.indice, lv.desc)
	e.almacenaEnBloque(lv.indice)
	return lv.desc, nil
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
	case jporCada:
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
		anidado := bajoOtroTry || len(v.atrapas) > 0 || len(v.recursos) > 0
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

// intentaRecurso: `try (R r = expr; …) { cuerpo }` en la forma de javac 17 (medida con
// `javap -c -p -v` sobre una sonda de dos recursos, §194). Cada recurso abre un nivel anidado: se
// declara fuera del rango protegido, y ese rango empieza justo después de la declaración y cubre el
// nivel interior entero (las declaraciones de dentro, el cuerpo, los cierres y los manejadores
// internos). El manejador de cada nivel guarda la excepción primaria en un local, cierra su recurso
// dentro de un segundo rango protegido y, si `close()` también falla, le añade la segunda con
// `Throwable.addSuppressed` antes de re-lanzar la primera. Con dos recursos `x` e `y`:
//
//	0  a; astore x
//	2  b; astore y
//	4  cuerpo                                [4,12) -> 25 Throwable
//	12 aload y; ifnull salida
//	16 aload y; close; goto salida
//	25 astore primary
//	27 aload y; ifnull relanza
//	31 aload y; close                        [31,37) -> 40 Throwable
//	37 goto relanza
//	40 astore suppressed
//	42 primary.addSuppressed(suppressed)
//	49 primary; athrow
//	52 aload x; ifnull salida                [2,52) -> 65 Throwable
//	56 aload x; close; goto salida
//	65 astore el hueco de `y` (javac lo reutiliza)
//	66 aload x; ifnull relanza
//	70 aload x; close                        [70,76) -> 79 Throwable
//	76 goto relanza
//	79 astore suppressed
//	81 primary.addSuppressed(suppressed)
//	87 primary; athrow
//	89 salida: …
//
// Las filas de la tabla salen de dentro hacia fuera, que es el orden en que javac las escribe. Solo
// se mide sin `catch` ni `finally` propios y con un cuerpo que cae: las salidas abruptas siguen
// fuera del subconjunto.
func (e *emisor) intentaRecurso(v jtry) error {
	if len(v.atrapas) > 0 || v.finalmente != nil {
		return fmt.Errorf("linea %d: un try-with-resources con `catch` o `finally` propios no esta medido", v.donde.linea)
	}
	return e.recursoAnidado(v, v.recursos, v.cuerpo)
}

// recursoAnidado emite el nivel del primer recurso de `recursos` y, dentro de su rango protegido, el
// nivel siguiente (o el cuerpo, en el más interno).
func (e *emisor) recursoAnidado(v jtry, recursos []jdecl, cuerpo jstmt) error {
	r := &recursos[0]
	desc, err := e.ctx.descriptorDe(r.tipo)
	if err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	if len(desc) < 3 || desc[0] != 'L' || desc[len(desc)-1] != ';' {
		return fmt.Errorf("linea %d: el recurso %s no es de referencia", v.donde.linea, r.tipo)
	}
	dueño := desc[1 : len(desc)-1]
	accesos, err := e.ctx.accesosDe(dueño)
	if err != nil {
		return fmt.Errorf("linea %d: %w", v.donde.linea, err)
	}
	opCierre := byte(opInvokevirt)
	if accesos&accInterface != 0 {
		opCierre = opInvointf
	}
	// la declaración va antes del rango protegido, como en javac
	if err := e.stm(*r); err != nil {
		return err
	}
	lv, ok := e.locales[r.nombre]
	if !ok {
		return fmt.Errorf("linea %d: el recurso %s no quedó en un local", v.donde.linea, r.nombre)
	}
	base := e.siguiente
	salida := e.nuevaEtiqueta()

	// el cuerpo vacío del recurso más interno no tiene nada que proteger: javac no abre rango ni
	// manejador y cierra con una guarda suelta. Un recurso solo deja la declaración, esa guarda y el
	// `return` del método, sin tabla de excepciones ni `addSuppressed`; con más recursos, el nivel de
	// dentro también se cierra así y los de fuera conservan su manejador. Medido con `javap -c -p -v`
	// de javac 17 sobre `try (Closeable x = a) { }` y `try (Closeable x = a; Closeable y = b) { }`.
	if len(recursos) == 1 && bloqueVacio(cuerpo) {
		e.cargaDe(lv.indice, desc)
		e.saltoEmite(opIFNULL, salida, 1)
		e.cargaDe(lv.indice, desc)
		e.invoca(opCierre, dueño, "close", "()V", 1)
		e.cierra(salida)
		return nil
	}

	relanza := e.nuevaEtiqueta()

	inicio := len(e.codigo)
	if len(recursos) > 1 {
		if err := e.recursoAnidado(v, recursos[1:], cuerpo); err != nil {
			return err
		}
	} else if err := e.stm(cuerpo); err != nil {
		return err
	}
	fin := len(e.codigo)

	// camino normal: `if (r != null) r.close();` y salto a la salida
	e.cargaDe(lv.indice, desc)
	e.saltoEmite(opIFNULL, salida, 1)
	e.cargaDe(lv.indice, desc)
	e.invoca(opCierre, dueño, "close", "()V", 1)
	e.saltoEmite(opGOTO, salida, 0)

	// manejador de la excepcion del cuerpo: la guarda como primaria. El local sale del hueco que el
	// nivel interior ya soltó: javac reutiliza el del recurso y, si el cuerpo declaró locales, el del
	// primero de ellos (`astore_2` en la sonda con `int local`, `astore_3` en la de dos recursos), y
	// `reserva` es monótona, así que hay que retroceder a mano
	e.siguiente = base
	hPrimary := len(e.codigo)
	e.marcaLinea(v.lineaAviso)
	indicePrimary := e.reserva("", "Ljava/lang/Throwable;")
	e.sube()
	e.guardaDe(indicePrimary, "Ljava/lang/Throwable;")

	// cierre dentro del rango protegido: si falla, la segunda excepcion es suprimida
	e.cargaDe(lv.indice, desc)
	e.saltoEmite(opIFNULL, relanza, 1)
	inicioCierre := len(e.codigo)
	e.cargaDe(lv.indice, desc)
	e.invoca(opCierre, dueño, "close", "()V", 1)
	finCierre := len(e.codigo)
	e.saltoEmite(opGOTO, relanza, 0)
	hClose := len(e.codigo)
	indiceSuprimida := e.reserva("", "Ljava/lang/Throwable;")
	e.sube()
	e.guardaDe(indiceSuprimida, "Ljava/lang/Throwable;")
	e.cargaDe(indicePrimary, "Ljava/lang/Throwable;")
	e.cargaDe(indiceSuprimida, "Ljava/lang/Throwable;")
	e.invoca(opInvokevirt, "java/lang/Throwable", "addSuppressed", "(Ljava/lang/Throwable;)V", 2)
	e.cierra(relanza)
	e.marcaLinea(v.lineaCierre)
	e.cargaDe(indicePrimary, "Ljava/lang/Throwable;")
	e.emite(0xbf) // athrow
	e.baja(1)

	e.cierra(salida)
	e.excepciones = append(e.excepciones,
		entradaTry{inicio: inicio, fin: fin, handler: hPrimary, tipo: "Ljava/lang/Throwable;"},
		entradaTry{inicio: inicioCierre, fin: finCierre, handler: hClose, tipo: "Ljava/lang/Throwable;"},
	)
	return nil
}

// intenta: `try { … } catch (T e) { … } [catch …] [finally { … }]`. La forma de javac, medida en el
// parrafo 71 sobre `T` y en el 75 sobre `Fin`, `Fin2` y `Fin3`: el cuerpo protegido y despues un bloque
// por camino —cada `catch` con su `astore` de la excepción y su cuerpo—; con `finally` la copia del
// bloque va al pie de cada camino que pueda caer (el normal también) y el ultimo manejador es el `any`,
// que guarda la excepción en un local que nadie mas usa, copia el bloque, y la re-lanza. Las filas de
// la tabla salen en ese orden: los tipos del try, el `any` del try, y el `any` de cada cuerpo del catch.
func (e *emisor) intenta(v jtry) error {
	if len(v.recursos) > 0 {
		return e.intentaRecurso(v)
	}
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
	// el `try` entero abre un ámbito: la excepción de cada `catch` y lo que quede del cuerpo dejan
	// de estar en ámbito al cerrarse el constructo, y el local siguiente vuelve a caer en su hueco.
	// Es el `astore_3` de `MainActivity$6`, donde javac reutiliza el hueco del `InputStream` para el
	// `Uri` que se declara justo después del `try`.
	e.abreAmbito()
	e.abrePico()
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
	// `any` exterior escribe `astore_3`). El pico se consulta y no `e.siguiente`, porque los bloques
	// de dentro ya han devuelto su hueco al cerrarse y el alto de javac no baja.
	if p := e.pico(); p > alto {
		alto = p
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
	// Con `catch` y `finally`, un cuerpo que sale por más de un trozo reparte TAMBIÉN los tipos
	// por trozos: javac escribe una fila por tipo y trozo, y después la fila `any` de cada trozo
	// (medido en §167 con `SalidaCatch.f`). La copia del `finally` delante del `return` no está
	// protegida, igual que en el caso de un solo trozo.
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
		if p := e.pico(); p > alto {
			alto = p
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
	trozosConTipo := filasCuerpo
	if finalmente == nil {
		trozosConTipo = e.trozosDelCuerpo(inicio, fin)
	}
	for i := range v.atrapas {
		for _, desc := range descs[i] {
			for _, trozo := range trozosConTipo {
				filas = append(filas, entradaTry{
					inicio: trozo.inicio, fin: trozo.fin, handler: manejadores[i], tipo: desc,
				})
			}
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
	// el pico de este `try` se cierra aquí: el reparto de lo que venga después va por su cuenta
	e.cierraPico()
	e.cierra(salida)
	e.excepciones = append(e.excepciones, filas...)
	e.cierraAmbito()
	return nil
}

// cortaRangoEnTodoGoto es el interruptor de la falsificación del rango: encendido, el troceado
// vuelve a partir en cualquier `goto`, que es lo que hacía antes de medir que javac solo corta
// cuando el salto sale del rango protegido. Apagado en producción.
var cortaRangoEnTodoGoto = false

// trozosDelCuerpo: sin `finally` no hay pendiente que haya anotado los trozos, pero javac también
// parte el rango protegido en cada salida del cuerpo: un `return` (medido en §167 con
// `SalidaCatch.h`: dos filas, [0,7) y [8,11), con el `ireturn` de la rama fuera de ambas) y un
// `goto` que salta FUERA del rango, como el `break` que sale del `try` o el `continue` de un bucle
// que lo envuelve (medido con javac 17 en este frente). Un `goto` que se queda dentro no parte
// nada: ni la vuelta de un bucle del propio cuerpo, aunque su cabecera sea el primer pc del rango,
// ni un `break` de un bucle interior. El `athrow` sí queda dentro, como en §71.
func (e *emisor) trozosDelCuerpo(inicio, fin int) []entradaTry {
	lista, err := walkJvmCode(e.codigo[:fin])
	if err != nil || len(lista) == 0 {
		return []entradaTry{{inicio: inicio, fin: fin}}
	}
	trozos := []entradaTry{}
	actual := inicio
	for _, in := range lista {
		if in.pc < inicio {
			continue
		}
		corta := false
		switch in.name {
		case "return", "ireturn", "lreturn", "freturn", "dreturn", "areturn":
			// un retorno sale del método y del rango: no puede lanzar y javac lo deja fuera
			corta = true
		case "goto", "goto_w":
			destino := destinoDe(in)
			corta = cortaRangoEnTodoGoto || destino < inicio || destino >= fin
		}
		if !corta {
			continue
		}
		if in.pc > actual {
			trozos = append(trozos, entradaTry{inicio: actual, fin: in.pc})
		}
		actual = in.pc + 1 + len(in.ops) + in.relleno
	}
	if actual < fin {
		trozos = append(trozos, entradaTry{inicio: actual, fin: fin})
	}
	if len(trozos) == 0 {
		trozos = append(trozos, entradaTry{inicio: inicio, fin: fin})
	}
	return trozos
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
		if v.tipo == "var" {
			// Java 10 infiere el tipo del inicializador; aquí se emite primero y se reserva el local
			// con el descriptor que dejó en la pila (medido en §179).
			e.marcaLinea(v.donde.linea)
			if v.valor == nil {
				return fmt.Errorf("linea %d: `var` necesita inicializador", v.donde.linea)
			}
			dv, err := e.exp(v.valor)
			if err != nil {
				return err
			}
			indice := e.reserva(v.nombre, dv)
			e.guardaDe(indice, dv)
			e.almacenaEnBloque(indice)
			return nil
		}
		d, err := e.ctx.descriptorDe(v.tipo)
		if err != nil {
			return fmt.Errorf("linea %d: %w", v.donde.linea, err)
		}
		// el analizador guarda el borrado también para una local (`List<Apk> apks` llega como `List`),
		// así que los argumentos de tipo solo se saben por el valor que se le da: `List<Apk> apks =
		// app.apks` los trae en el campo. Sin ellos la local queda sin tipo declarado conocido y la
		// ruta de la llamada encadenada no estima, en vez de devolver el borrado en silencio.
		indice := e.reservaConGenerico(v.nombre, d, e.genericoDeLocal(v.valor, d))
		if v.valor == nil {
			// una declaracion sin inicializador no escribe ni un byte, y javac no le pone fila en la
			// LineNumberTable: medido contra javac 17 en la copia de la linea 187 de ApkShare, donde
			// `int length;` no deja entrada y la primera fila del bucle es la del `while` (`pc 26,
			// linea 18`). Antes se marcaba la linea de la declaracion y quedaba una fila de mas con el
			// mismo `pc` que la sentencia siguiente.
			return nil
		}
		e.marcaLinea(v.donde.linea)
		dv, err := e.expA(v.valor, d)
		if err != nil {
			return err
		}
		if d[0] != dv[0] {
			return fmt.Errorf("linea %d: se declara %s y se le da un %s", v.donde.linea, d, dv)
		}
		e.guardaDe(indice, d)
		e.almacenaEnBloque(indice)
		return nil
	case jasign:
		e.marcaLinea(v.donde.linea)
		if ind, esIndice := v.destino.(jindice); esIndice {
			return e.escribeIndice(ind, v.valor)
		}
		acc, esAcceso := v.destino.(jacceso)
		segs, base := aplanar(acc)
		// `this.campo = valor` se resuelve contra la clase del receptor, igual que el nombre suelto: javac
		// escribe el mismo `aload_0; …; putfield` y pone la linea de la sentencia en el primer byte, asi
		// que la LineNumberTable no se mueve. Medido contra javac 17 en
		// `javaemisor_argumento_estatico_test.go` (`TestAsignacionCualificadaContraJavac17`).
		porThis := false
		if t, esThis := base.(jthis); esThis && t.nombre == "" && len(segs) == 1 {
			porThis = true
		}
		// `appInfo.sourceDir = valor` y `this.datos.ruta = valor`: el receptor no es `this` sino un
		// valor con tipo conocido (una local, un parámetro o un campo, también el que captura una
		// anónima), y el campo se busca en su clase. Va antes del error genérico y delega en
		// `escribeCampoDe`, que es quien sabe decirlo cuando el receptor no tiene tipo.
		if esAcceso && !porThis && (base != nil || len(segs) > 1) {
			return e.escribeCampoDe(segs, base, v.donde, v.valor)
		}
		if !esAcceso || len(segs) != 1 || (base != nil && !porThis) {
			return fmt.Errorf("linea %d: una asignacion con mas de un nombre en el destino no esta medida", v.donde.linea)
		}
		if porThis && e.estatico {
			return fmt.Errorf("linea %d: `this` en un metodo estatico no es Java", v.donde.linea)
		}
		if !porThis {
			if lv, ok := e.locales[segs[0]]; ok {
				dv, err := e.expA(v.valor, lv.desc)
				if err != nil {
					return err
				}
				if dv[0] != lv.desc[0] {
					return fmt.Errorf("linea %d: %s es %s y se le asigna un %s", v.donde.linea, acc.nombre, lv.desc, dv)
				}
				e.guardaDe(lv.indice, lv.desc)
				e.almacenaEnBloque(lv.indice)
				return nil
			}
		}
		// asignar a un campo: `pulsos = pulsos + 1`, medido como el receptor ANTES del valor, el valor, y
		// `putfield`; en un campo estatico no hay receptor. Desde una anonima el campo puede ser del
		// exterior, y entonces el receptor es la captura (parrafo 54). Con receptor `this` solo se mira
		// la clase del receptor: no es un local y no alcanza al exterior que captura una anonima, donde
		// `this` es la propia anonima.
		dueño, cr, encontrado := "", campoResuelto{}, false
		if porThis {
			if c, err := e.ctx.buscarCampo(e.interno, segs[0]); err == nil {
				dueño, cr, encontrado = e.interno, c, true
			}
		} else {
			dueño, cr, encontrado = e.duenioDelCampo(segs[0])
		}
		if !encontrado {
			if porThis {
				return fmt.Errorf("linea %d: al asignar a this.%s: no es un campo de esta clase", v.donde.linea, segs[0])
			}
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
			dv, err := e.expA(v.valor, cr.desc)
			if err != nil {
				e.baja(enPila - 1)
				return err
			}
			if !e.ctx.cabeEn(dv, cr.desc) {
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
		dv, err := e.expA(v.valor, cr.desc)
		if err != nil {
			if !esEstatico {
				e.baja(1) // el receptor que ya se habia contado
			}
			return err
		}
		if !e.ctx.cabeEn(dv, cr.desc) {
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
		// el bloque del fuente abre un ámbito: lo que declare dentro devuelve su hueco al cerrar, que
		// es el reparto de javac (`endScopes`). Un `return` dentro del bloque deja el método emitido y
		// el ámbito abierto no se cierra, pero ya no se emite nada más del método.
		e.abreAmbito()
		for _, st := range v.sentencias {
			if err := e.stm(st); err != nil {
				return err
			}
		}
		// la fila del cierre: javac la escribe en el pc donde cierra el ámbito, y solo si algún local
		// del bloque llegó a tener valor (`endScope` llama a `curCP`, que vuelca el `statBegin(endpos)`
		// del bloque). La lee `branca` al terminar de emitir el `then`, que es donde esa fila cae.
		e.filaDeCierre = 0
		if e.ambitoAlmacena() {
			e.filaDeCierre = v.lineaCierre
		}
		e.cierraAmbito()
		return nil
	case jif:
		return e.branca(v)
	case jmientras:
		return e.mientras(v)
	case jpara:
		return e.para(v)
	case jporCada:
		return e.porCada(v)
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
			// la constante entera se dobla como `int`; la de `String` conserva su descriptor y se
			// emite como el literal, que es lo que hace javac
			if cr.esCadena {
				return cr.desc, nil
			}
			return "I", nil
		}
		return cr.desc, nil
	}
	return "", fmt.Errorf("no se resuelve %s.%s", interno, strings.Join(segs, "."))
}

// ----- escribir el ClassFile -----

// emmitirEnum escribe el ClassFile que javac 17 genera para un enum sin constantes con cuerpo ni
// argumentos (medido con `javac --release 8 -g:lines,source Color.java`): una clase final con ACC_ENUM
// que extiende java/lang/Enum, un campo por constante, $VALUES, values(), valueOf(String), el constructor
// privado (String,int), $values() y el <clinit> que arma el arreglo.
func (ctx *contextoJava) emmitirEnum(c *jclas) ([]byte, error) {
	interno := internoDe(c.paquete, c.nombre)
	e := &emisor{ctx: ctx, pool: nuevoPool(), c: c, interno: interno, vistas: map[string]bool{}}
	desc := "L" + interno + ";"
	arr := "[" + desc
	linea := c.linea
	if linea == 0 {
		linea = 1
	}

	// los campos van en el orden del fuente y $VALUES al final
	campos := []byte{}
	for _, nombre := range c.constantes {
		b := u2be(accPublic | accStatic | accFinal | accEnum)
		b = append(b, u2be(uint16(e.pool.utf8(nombre)))...)
		b = append(b, u2be(uint16(e.pool.utf8(desc)))...)
		campos = append(campos, append(b, u2be(0)...)...)
	}
	b := u2be(accPrivate | accStatic | accFinal | accSynthetic)
	b = append(b, u2be(uint16(e.pool.utf8("$VALUES")))...)
	b = append(b, u2be(uint16(e.pool.utf8(arr)))...)
	campos = append(campos, append(b, u2be(0)...)...)

	metodos := []byte{}
	conte := 0
	agregar := func(nombre, mdesc string, acc int, extra []byte, extraN int, cuerpo func()) error {
		e.preparaMetodo(mdesc, acc&accStatic != 0)
		cuerpo()
		codigo, err := e.codeAttr()
		if err != nil {
			return err
		}
		attrs := attrInfo(e.pool, "Code", codigo)
		attrs = append(attrs, extra...)
		metodos = append(metodos, e.methodInfo(nombre, mdesc, acc, attrs, 1+extraN)...)
		conte++
		return nil
	}

	if err := agregar("values", "()"+arr, accPublic|accStatic, nil, 0, func() {
		e.marcaLinea(linea)
		idx := e.pool.campo(interno, "$VALUES", arr)
		e.getstaticDe(idx, arr)
		e.invoca(opInvokevirt, arr, "clone", "()Ljava/lang/Object;", 1)
		ci := e.pool.clase(arr)
		e.emite(opCheckcast, byte(ci>>8), byte(ci))
		e.emite(retorno('L'))
		e.baja(1)
	}); err != nil {
		return nil, err
	}

	if err := agregar("valueOf", "(Ljava/lang/String;)"+desc, accPublic|accStatic, nil, 0, func() {
		e.reserva("nombre", "Ljava/lang/String;")
		e.marcaLinea(linea)
		e.ldc(e.pool.clase(interno))
		e.sube()
		e.cargaDe(0, "Ljava/lang/String;")
		e.invoca(opInvokestat, "java/lang/Enum", "valueOf", "(Ljava/lang/Class;Ljava/lang/String;)Ljava/lang/Enum;", 2)
		ci := e.pool.clase(interno)
		e.emite(opCheckcast, byte(ci>>8), byte(ci))
		e.emite(retorno('L'))
		e.baja(1)
	}); err != nil {
		return nil, err
	}

	firmaCtor := attrInfo(e.pool, "Signature", u2be(uint16(e.pool.utf8("()V"))))
	if err := agregar("<init>", "(Ljava/lang/String;I)V", accPrivate, firmaCtor, 1, func() {
		e.reserva("", "L")
		e.reserva("", "Ljava/lang/String;")
		e.reserva("", "I")
		e.marcaLinea(linea)
		e.cargaDe(0, "L")
		e.cargaDe(1, "Ljava/lang/String;")
		e.cargaDe(2, "I")
		e.invoca(opInvokespec, "java/lang/Enum", "<init>", "(Ljava/lang/String;I)V", 3)
		e.emite(retorno('V'))
	}); err != nil {
		return nil, err
	}

	if err := agregar("$values", "()"+arr, accPrivate|accStatic|accSynthetic, nil, 0, func() {
		e.marcaLinea(linea)
		e.cargaEntero(int32(len(c.constantes)))
		ci := e.pool.clase(interno)
		e.emite(0xbd, byte(ci>>8), byte(ci)) // anewarray: se come el conteo y deja el arreglo
		for i, nombre := range c.constantes {
			e.emite(opDup)
			e.sube()
			e.cargaEntero(int32(i))
			idx := e.pool.campo(interno, nombre, desc)
			e.emite(opGetstatic, byte(idx>>8), byte(idx))
			e.subePor(desc)
			e.emite(0x53) // aastore
			e.baja(3)
		}
		e.emite(retorno('L'))
		e.baja(1)
	}); err != nil {
		return nil, err
	}

	if err := agregar("<clinit>", "()V", accStatic, nil, 0, func() {
		e.marcaLinea(linea)
		for i, nombre := range c.constantes {
			ci := e.pool.clase(interno)
			e.emite(opNew, byte(ci>>8), byte(ci))
			e.sube()
			e.emite(opDup)
			e.sube()
			e.ldc(e.pool.cadena(nombre))
			e.sube()
			e.cargaEntero(int32(i))
			e.invoca(opInvokespec, interno, "<init>", "(Ljava/lang/String;I)V", 3)
			idx := e.pool.campo(interno, nombre, desc)
			e.emite(opPutstatic, byte(idx>>8), byte(idx))
			e.baja(1)
		}
		e.invoca(opInvokestat, interno, "$values", "()"+arr, 0)
		idx := e.pool.campo(interno, "$VALUES", arr)
		e.emite(opPutstatic, byte(idx>>8), byte(idx))
		e.baja(1)
		e.emite(retorno('V'))
	}); err != nil {
		return nil, err
	}

	attrs := attrInfo(e.pool, "SourceFile", u2be(uint16(e.pool.utf8(ctx.archivo))))
	conteAttrs := 1
	firmaClase := "Ljava/lang/Enum<L" + interno + ";>;"
	attrs = append(attrs, attrInfo(e.pool, "Signature", u2be(uint16(e.pool.utf8(firmaClase))))...)
	conteAttrs++
	if len(c.anotaciones) > 0 {
		an, err := e.anotacionesAttr(c.anotaciones)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, an...)
		conteAttrs++
	}

	access := accSuper | accFinal | accEnum
	if c.esPublica {
		access |= accPublic
	}
	thisIdx := e.pool.clase(interno)
	superIdx := e.pool.clase("java/lang/Enum")

	out := []byte{0xca, 0xfe, 0xba, 0xbe}
	out = append(out, u2be(0)...)
	out = append(out, u2be(versionDeClassEmitida)...)
	out = append(out, e.pool.serializar()...)
	out = append(out, u2be(uint16(access))...)
	out = append(out, u2be(uint16(thisIdx))...)
	out = append(out, u2be(uint16(superIdx))...)
	out = append(out, u2be(0)...) // interfaces
	out = append(out, u2be(uint16(len(c.constantes)+1))...)
	out = append(out, campos...)
	out = append(out, u2be(uint16(conte))...)
	out = append(out, metodos...)
	out = append(out, u2be(uint16(conteAttrs))...)
	return append(out, attrs...), nil
}

// emmitirClase escribe el ClassFile de una declaracion. El pool se arma emitiendo los miembros, asi que el
// InnerClasses se calcula al final, cuando ya se sabe que anidadas menciona el codigo.
func (ctx *contextoJava) emmitirClase(c *jclas) ([]byte, error) {
	if c.esEnum {
		return ctx.emmitirEnum(c)
	}
	interno := internoDe(c.paquete, c.nombre)
	e := &emisor{ctx: ctx, pool: nuevoPool(), c: c, interno: interno, vistas: map[string]bool{}}

	thisIdx := e.pool.clase(interno)
	// el `Object` por defecto solo se añade cuando el fuente no declara padre: javac no deja la
	// entrada en el pool de una clase que extiende algo (medido en `MainActivity`, donde es el
	// único par `Utf8`/`Class` de `java/lang/Object` que sobraba y desplazaba las constantes).
	superIdx := 0
	if c.super != "" {
		s, err := ctx.tipoAInterno(c.super)
		if err != nil {
			return nil, err
		}
		e.usar(s)
		superIdx = e.pool.clase(s)
	} else {
		superIdx = e.pool.clase("java/lang/Object")
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

	esInner := !c.anonima && c.encierre != nil && !c.esEstatica
	campos := []byte{}
	nCampos := 0
	if esInner {
		// una anidada con nombre no estatica baja el exterior a this$0, delante de sus campos
		campos = append(campos, e.campoCaptura()...)
		nCampos++
	}
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
	} else if esInner {
		if constructorEscrito {
			return nil, fmt.Errorf("linea %d: un constructor escrito en una anidada no estatica no esta medido; javac le añade el parametro del exterior", c.linea)
		}
		b, err := e.initInner()
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
	if marcador, ok := e.ctx.ctorSintetico[interno]; ok {
		b, err := e.initMarcador(marcador)
		if err != nil {
			return nil, err
		}
		metodos = append(metodos, b...)
		conte++
	}
	// los puentes del retorno covariante. javac los escribe entre los metodos declarados y los
	// `access$NNN` (medido con javac 17 sobre una clase que sobrescribe con `String` un `Object` y
	// lleva a la vez un `access$002`: el orden del ClassFile es `m()Ljava/lang/String;`,
	// `m()Ljava/lang/Object;`, `access$002`, `static {}`)
	puentes, err := e.puentesCovariantes()
	if err != nil {
		return nil, err
	}
	for _, b := range puentes {
		metodos = append(metodos, b...)
		conte++
	}
	// Los implementadores de las lambdas, ahora que el código de todos los métodos está escrito: es
	// el punto en el que javac los genera, y por eso sus constantes caen al final del pool. Cada
	// vaciado puede encolar otros (una lambda dentro de otra), de ahí el bucle.
	for len(e.lambdaPendientes) > 0 {
		p := e.lambdaPendientes[0]
		e.lambdaPendientes = e.lambdaPendientes[1:]
		if err := e.escribeImplementador(p); err != nil {
			return nil, err
		}
	}
	// los implementadores de las lambdas del fuente, con `ACC_PRIVATE|ACC_SYNTHETIC`, en el sitio que
	// les da javac: entre los puentes del retorno covariante y los `access$NNN`. javac los escribe en
	// el orden INVERSO al que los crea (medido con `javap -p` sobre una clase con seis lambdas: crea
	// `lambda$f$0`…`lambda$h$5` y el ClassFile los lista `lambda$h$5`…`lambda$f$0`), y ese orden es el
	// que ven el `class_data_item` del dex y su comparación byte a byte. El sitio entre los puentes y
	// los `access$` está medido con javac 17 en `W`, una clase con puente covariante, lambda, anónima
	// que pide un `access$002` y `static {}`: el ClassFile sale `f()Ljava/lang/String;`,
	// `f()Ljava/lang/Object;` (el puente), `lambda$conLambdaYAnonima$0`, `access$002` y `static {}`,
	// que es el orden de las pasadas de javac —TransTypes crea el puente, LambdaToMethod la lambda y
	// Lower el `access$`—. Antes iban detrás de los `access$` y el dex de `MainActivity` no coincidía.
	for i := len(e.lambdaMetodos) - 1; i >= 0; i-- {
		metodos = append(metodos, e.lambdaMetodos[i]...)
	}
	conte += len(e.lambdaMetodos)
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
	// y las filas del BootstrapMethods, cuyos MethodHandle y MethodType se acaban de añadir. El
	// índice de la fila ya estaba reservado en el sitio del `invokedynamic`.
	for i := range e.bootstraps {
		if f := e.bootstraps[i].pendiente; f != nil {
			e.bootstraps[i].handle, e.bootstraps[i].args = f(e.pool)
			e.bootstraps[i].pendiente = nil
			// el descriptor del bootstrap nombra `MethodHandles$Lookup`, y javac lo anota al escribir
			// el atributo, con el código ya cerrado: por eso su fila del InnerClasses va la última,
			// detrás de las clases que mencionan los métodos posteriores al sitio de la lambda.
			e.usar("java/lang/invoke/MethodHandles$Lookup")
		}
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
	if c.anonima && c.encierre != nil && c.encierre.anonima {
		// la anonima interior lista primero a su padre anonimo y despues a si misma (medido en el
		// InnerClasses de Capa$1$1 con javac 17: `#23; // Capa$1` y `#2; // Capa$1$1`)
		anadir(internoDe(c.encierre.paquete, c.encierre.nombre))
	}
	if strings.Contains(c.nombre, "$") {
		anadir(interno)
	}
	// javac escribe tambien la fila de la clase marcador en la exterior y en la anidada
	if m := e.ctx.ctorSintetico[interno]; m != "" {
		anadir(m)
	}
	for _, m := range e.ctx.ctorSintetico {
		if strings.HasPrefix(m, interno+"$") {
			anadir(m)
		}
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
	if firma, err := ctx.firmaDeClase(c); err != nil {
		// un argumento de tipo que no está entre los fuentes de esta pasada no impide escribir la clase:
		// `check` comprueba un fichero suelto y el `Apk` de `ArrayAdapter<Apk>` puede ser otro fuente del
		// proyecto. El descriptor es el del borrado y no cambia; sólo se pierde la firma genérica.
		if !strings.Contains(err.Error(), "no se resuelve") {
			return nil, err
		}
	} else if firma != "" {
		// javac escribe la Signature aunque el borrado no cambie el descriptor; sin ella, el dex de
		// javac lleva la anotación `dalvik.annotation.Signature` y el nuestro no, y deja de coincidir
		attrs = append(attrs, attrInfo(e.pool, "Signature", u2be(uint16(e.pool.utf8(firma))))...)
		conteAttrs++
	}
	if c.anonima {
		cuerpo, err := e.cuerpoEnclosingMethod()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attrInfo(e.pool, "EnclosingMethod", cuerpo)...)
		conteAttrs++
	}
	if len(e.bootstraps) > 0 {
		// javac escribe BootstrapMethods entre la Signature y el InnerClasses; el atributo lleva una
		// fila por sitio, con el MethodHandle del bootstrap y sus argumentos como indices del pool
		attrs = append(attrs, attrInfo(e.pool, "BootstrapMethods", tablaDeBootstraps(e.bootstraps))...)
		conteAttrs++
	}
	if len(nombres) > 0 {
		attrs = append(attrs, attrInfo(e.pool, "InnerClasses", entradas)...)
		conteAttrs++
	}
	if len(c.anotaciones) > 0 {
		b, err := e.anotacionesAttr(c.anotaciones)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, b...)
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

// emmitirMarcador: la clase sintetica vacia `Exterior$1` que javac 17 con --release 8 añade cuando una
// anidada privada se instancia desde fuera. No tiene campos ni metodos; lleva SourceFile,
// EnclosingMethod sin metodo y una fila `static #N;` en su propio InnerClasses (medido con javap).
func (ctx *contextoJava) emmitirMarcador(externo *jclas, marcador string) ([]byte, error) {
	e := &emisor{ctx: ctx, pool: nuevoPool(), c: externo, interno: marcador, vistas: map[string]bool{}}
	thisIdx := e.pool.clase(marcador)
	superIdx := e.pool.clase("java/lang/Object")
	padre := internoDe(externo.paquete, externo.nombre)
	enclosing := u2be(uint16(e.pool.clase(padre)))
	enclosing = append(enclosing, u2be(0)...)
	fila := u2be(uint16(e.pool.clase(marcador)))
	fila = append(fila, u2be(0)...)
	fila = append(fila, u2be(0)...)
	fila = append(fila, u2be(uint16(accStatic|accSynthetic))...)
	inner := u2be(1)
	inner = append(inner, fila...)
	attrs := attrInfo(e.pool, "SourceFile", u2be(uint16(e.pool.utf8(ctx.archivo))))
	attrs = append(attrs, attrInfo(e.pool, "EnclosingMethod", enclosing)...)
	attrs = append(attrs, attrInfo(e.pool, "InnerClasses", inner)...)

	out := []byte{0xca, 0xfe, 0xba, 0xbe}
	out = append(out, u2be(0)...)
	out = append(out, u2be(versionDeClassEmitida)...)
	out = append(out, e.pool.serializar()...)
	out = append(out, u2be(uint16(accSuper|accSynthetic))...)
	out = append(out, u2be(uint16(thisIdx))...)
	out = append(out, u2be(uint16(superIdx))...)
	out = append(out, u2be(0)...) // interfaces
	out = append(out, u2be(0)...) // campos
	out = append(out, u2be(0)...) // metodos
	out = append(out, u2be(3)...)
	return append(out, attrs...), nil
}

// filaInner: inner, outer, nombre simple y access_flags, como los escribe javac. La fila de una anonima va
// sin exterior y sin nombre, y con flags ninguno: medido en el parrafo 42 (`InnerClasses: #2;`).
func (e *emisor) filaInner(internal string) ([]byte, error) {
	esMarcador := false
	for _, m := range e.ctx.ctorSintetico {
		if m == internal {
			esMarcador = true
		}
	}
	if esMarcador {
		// `static #N;` sin exterior ni nombre simple, con ACC_SYNTHETIC: medido en el InnerClasses de
		// Capa.class, Capa$Hija.class y Capa$1.class con javac 17 --release 8 (flags 0x1008)
		b := u2be(uint16(e.pool.clase(internal)))
		b = append(b, u2be(0)...)
		b = append(b, u2be(0)...)
		return append(b, u2be(uint16(accStatic|accSynthetic))...), nil
	}
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

// campoCaptura: el `final synthetic Tipo this$N;` con el que javac baja el exterior dentro de la anonima
// (parrafo 42: flags 0x1010, descriptor del exterior y ningun atributo). El N no es siempre 0: una
// anonima anidada guarda su exterior en `this$1` (ver `nivelDelExterior`).
func (e *emisor) campoCaptura() []byte {
	desc := "L" + e.encierreInterno() + ";"
	b := u2be(uint16(accFinal | accSynthetic))
	b = append(b, u2be(uint16(e.pool.utf8(nombreDelExterior(e.c))))...)
	b = append(b, u2be(uint16(e.pool.utf8(desc)))...)
	return append(b, u2be(0)...)
}

func (e *emisor) encierreInterno() string {
	if e.c.encierre == nil {
		return "java/lang/Object"
	}
	return internoDe(e.c.encierre.paquete, e.c.encierre.nombre)
}

// tieneExterior: la clase guarda en un campo `this$N` la instancia que la creo. Es una anonima de un
// metodo de instancia o una anidada con nombre no estatica; la anonima de un metodo `static` no tiene
// exterior (parrafo 100).
func tieneExterior(c *jclas) bool {
	if c == nil || c.encierre == nil {
		return false
	}
	if c.anonima {
		return c.metodoEncierre != nil && !c.metodoEncierre.esEstatico
	}
	return !c.esEstatica
}

// nivelDelExterior: el N de `this$N` de `c`. No es la profundidad del fuente sino cuantos exteriores
// encadenados hay por debajo: `MainActivity$2` guarda la Activity en `this$0` y `MainActivity$2$1`
// guarda `MainActivity$2` en `this$1`; en cambio `Capa$1$1`, cuyo exterior se creo en un metodo
// `static` y no tiene ninguno, lo guarda en `this$0`. Medido con javac 17 `--release 8 -g:lines,source`
// sobre ApkShare (`javap -p` de MainActivity$2 y MainActivity$2$1) y sobre la sonda de
// `javaemisor_anonima_anidada_test.go`.
func nivelDelExterior(c *jclas) int {
	if !tieneExterior(c) || !tieneExterior(c.encierre) {
		return 0
	}
	return 1 + nivelDelExterior(c.encierre)
}

// nombreDelExterior: el nombre del campo sintetico con el que `c` guarda el exterior que la creo.
func nombreDelExterior(c *jclas) string { return "this$" + itoa(nivelDelExterior(c)) }

// exteriores: las clases que encierran a `c`, de dentro afuera, mientras haya una instancia que
// alcanzar por `this$N`. Una anonima de un metodo `static` corta la cadena: no tiene exterior.
func exteriores(c *jclas) []*jclas {
	sal := []*jclas{}
	for c != nil && tieneExterior(c) {
		c = c.encierre
		sal = append(sal, c)
	}
	return sal
}

// clasesEnAlcance: todas las clases que encierran a `c`, de dentro afuera, tenga o no instancia que
// alcanzar. El ambito lexico de un nombre suelto las incluye todas —un `static` del exterior se llama
// sin receptor aunque la anonima se creara en un metodo `static` (medido en `Capa$1$1`, que llama a
// `Capa.getM()`)—; para un miembro de instancia, en cambio, hace falta la cadena `this$N` y
// `cargaDelExteriorHasta` falla si se corta.
func clasesEnAlcance(c *jclas) []*jclas {
	sal := []*jclas{}
	for c != nil && c.encierre != nil {
		c = c.encierre
		sal = append(sal, c)
	}
	return sal
}

// cargaDelExteriorHasta: deja en la pila la instancia de `destino` que se alcanza desde `this`:
// `aload_0` y un `getfield this$N` por cada salto, con el nombre del campo y el tipo del exterior que
// tocan en cada uno. En una anonima anidada son dos saltos: medido con javac 17 --release 8, el `run()`
// de MainActivity$2$1 lee `aload_0; getfield this$1:LMainActivity$2; getfield
// this$0:LMainActivity;` para llamar a un metodo de la Activity.
func (e *emisor) cargaDelExteriorHasta(destino string) error {
	if destino == "" {
		return fmt.Errorf("no hay exterior que cargar en %s", e.interno)
	}
	if e.estatico {
		return fmt.Errorf("`this` en un metodo estatico no es Java: no se alcanza %s desde %s", destino, e.interno)
	}
	dueño := e.interno
	c := e.c
	e.cargaDe(0, "L")
	for dueño != destino {
		if !tieneExterior(c) {
			return fmt.Errorf("no se alcanza %s desde %s: la cadena de exteriores se corta", destino, e.interno)
		}
		ext := c.encierre
		extInterno := internoDe(ext.paquete, ext.nombre)
		e.getfield(e.pool.campo(dueño, nombreDelExterior(c), "L"+extInterno+";"))
		dueño = extInterno
		c = ext
	}
	return nil
}

// esInnerNoEstatica: una clase anidada con nombre que no es static ni anonima. javac le baja el exterior
// a un campo `this$0` y le añade ese parámetro al constructor; no escribe MethodParameters (medido con
// javac 17 sobre `Inner$Hija`).
func (e *emisor) esInnerNoEstatica() bool {
	return e.c.encierre != nil && !e.c.esEstatica && !e.c.anonima
}

func (ctx *contextoJava) esInnerNoEstatica(internal string) (*jclas, bool) {
	c, ok := ctx.clasePropia(internal)
	return c, ok && c.encierre != nil && !c.esEstatica && !c.anonima
}

// initCaptura: el constructor de la anónima. Lleva la captura como único parámetro, le pone el campo antes
// de subir al padre y sube al `super()`, medido en el párrafo 42 sobre MainActivity$1. No escribe
// `MethodParameters`: ningún JDK lo pone en el constructor de una anónima (ver la nota del final).
func (e *emisor) initCaptura() ([]byte, error) {
	// Una anónima captura el exterior (`this$0`, si el método no es static) y las locales
	// effectively-final (`val$x`). Javac escribe primero las asignaciones de captura y después el
	// `super()`.
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
	e.esCtor = true
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
		idx := e.pool.campo(e.interno, nombreDelExterior(e.c), externo)
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
	// Sin `MethodParameters` en el constructor de una anónima: javac 17.0.21 —el oráculo del repo— no lo
	// escribe en ninguna, ni con `this$0` (método de instancia) ni con capturas de locales (medido en
	// cuatro sondas y en las 21 clases de ApkShare: 0 apariciones). javac 21.0.13 lo escribe solo en la
	// anónima de un método de instancia, con el `this$0` como `final mandated` (0x8010) —la medición del
	// párrafo 42 se hizo con ese JDK— y nunca en la de un `static`. Como el oráculo es 17 y el atributo no
	// cambia la ejecución, se omite: era la única diferencia del constructor con javac 17.
	// `Signature: ()V`: javac la escribe —17 y 21 por igual— si y solo si el constructor recibe
	// capturas de locales (`val$x`); con solo el exterior (`this$0`/`this$1`) no aparece. Medido con
	// javac 17.0.21 sobre ApkShare (`MainActivity$1..$6` y `ApkAdapter$1` la llevan, `$2$1` y `$6$1`
	// no), sobre `Anon$2` (captura en un metodo static), `Anon$3` (solo el exterior) y `Captura3$1`.
	// Antes se ponia solo en la anonima creada dentro de otra, que es justo donde no va.
	if len(e.c.capturas) > 0 {
		attrs = append(attrs, attrInfo(e.pool, "Signature", u2be(uint16(e.pool.utf8("()V"))))...)
		conte++
	}
	return e.methodInfo("<init>", desc, 0, attrs, conte), nil
}

// initInner: el constructor sintetizado de una anidada con nombre no estática. Guarda su exterior antes
// de subir al padre (javap lo pone en la línea de la declaración de la clase) y tampoco lleva
// MethodParameters: ni esta ni la anónima lo llevan con el oráculo de javac 17.
func (e *emisor) initInner() ([]byte, error) {
	externo := "L" + e.encierreInterno() + ";"
	desc := "(" + externo + ")V"
	e.preparaMetodo(desc, false)
	e.esCtor = true
	e.reserva("", "L")
	e.reserva("", externo)
	e.marcaLinea(e.c.linea)
	e.cargaDe(0, "L")
	e.cargaDe(1, externo)
	idx := e.pool.campo(e.interno, nombreDelExterior(e.c), externo)
	e.emite(opPutfield, byte(idx>>8), byte(idx))
	e.baja(2)
	e.cargaDe(0, "L")
	padre, err := e.superDe()
	if err != nil {
		return nil, err
	}
	e.invoca(opInvokespec, padre, "<init>", "()V", 1)
	e.emite(retorno('V'))
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	acc := 0
	if e.c.esPrivada {
		acc |= accPrivate
	}
	return e.methodInfo("<init>", desc, acc, attrInfo(e.pool, "Code", codigo), 1), nil
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
	if len(f.anotaciones) > 0 {
		b, err := e.anotacionesAttr(f.anotaciones)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, b...)
		conte++
	}
	if f.generico != nil {
		// la Signature genérica del campo, que es lo que javac pone en el field_info: el descriptor sigue
		// siendo el del borrado. Si un argumento no se resuelve en esta pasada (un fuente suelto de
		// `check`), se omite la firma sin falsificar el error: el campo sigue siendo válido.
		firma, err := e.ctx.firmaDeArgumento(*f.generico)
		if err != nil {
			if !strings.Contains(err.Error(), "no se resuelve") {
				return nil, fmt.Errorf("el campo %s: %w", f.nombre, err)
			}
		} else {
			attrs = append(attrs, attrInfo(e.pool, "Signature", u2be(uint16(e.pool.utf8(firma))))...)
			conte++
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
	e.esCtor = false
	e.desc = desc
	e.vuelta = vueltaDe(desc)
	e.locales = map[string]localVar{}
	e.siguiente = 0
	e.ambitos = nil
	e.picos = nil
	e.codigo = nil
	e.lineas = nil
	e.parches = nil
	e.parches4 = nil
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
	if err := e.resuelveSaltos(); err != nil {
		return nil, err
	}
	marcos, alto, err := framesDeMetodo(e.codigo, e.desc, e.estatico, e.interno, e.pool, e.maxLoc, e.excepciones, e.muertes, e.esCtor)
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
	if m.varargs {
		// javac 17 marca el method_info de un `T... xs` con ACC_VARARGS (§194); el dex conserva el
		// bit, asi que sin esto el method_info no coincide byte a byte.
		acc |= accVarargs
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

// anotacionesAttr escribe el atributo RuntimeVisibleAnnotations (JVMS 4.7.16): cuenta, descriptor del
// tipo y pares nombre/element_value en el orden del fuente. Los tres que no dejan huella (@Override,
// @SuppressWarnings y @Deprecated) se descartan en el parser, así que aquí solo llegan anotaciones con
// elementos simples.
func (e *emisor) anotacionesAttr(lista []anotacionJ) ([]byte, error) {
	body := u2be(uint16(len(lista)))
	for _, an := range lista {
		desc, err := e.ctx.descriptorDe(an.nombre)
		if err != nil {
			return nil, err
		}
		body = append(body, u2be(uint16(e.pool.utf8(desc)))...)
		body = append(body, u2be(uint16(len(an.elementos)))...)
		for _, par := range an.elementos {
			body = append(body, u2be(uint16(e.pool.utf8(par.nombre)))...)
			v, err := e.valorDeAnotacion(par.valor)
			if err != nil {
				return nil, err
			}
			body = append(body, v...)
		}
	}
	return attrInfo(e.pool, "RuntimeVisibleAnnotations", body), nil
}

// valorDeAnotacion codifica un element_value. `b` (booleano) usa la misma entrada Integer que `i` pero
// con tag 'Z', tal como sale en el javap de Annotated; `e` lleva el descriptor del enum y el nombre de
// la constante, y `a` el conteo seguido de sus elementos.
func (e *emisor) valorDeAnotacion(v anotValorJ) ([]byte, error) {
	switch v.kind {
	case 'i', 'b':
		tag := byte('I')
		if v.kind == 'b' {
			tag = 'Z'
		}
		return append([]byte{tag}, u2be(uint16(e.pool.entero(int32(v.entero))))...), nil
	case 's':
		return append([]byte{'s'}, u2be(uint16(e.pool.utf8(v.texto)))...), nil
	case 'c':
		desc, err := e.ctx.descriptorDe(v.texto)
		if err != nil {
			return nil, err
		}
		return append([]byte{'c'}, u2be(uint16(e.pool.utf8(desc)))...), nil
	case 'e':
		tipo, err := e.ctx.descriptorDe(v.texto)
		if err != nil {
			return nil, err
		}
		out := []byte{'e'}
		out = append(out, u2be(uint16(e.pool.utf8(tipo)))...)
		return append(out, u2be(uint16(e.pool.utf8(v.nombre)))...), nil
	case 'a':
		out := []byte{'['}
		out = append(out, u2be(uint16(len(v.lista)))...)
		for _, hijo := range v.lista {
			b, err := e.valorDeAnotacion(hijo)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		}
		return out, nil
	}
	return nil, fmt.Errorf("el valor de anotación de tipo %q no está medido", string(rune(v.kind)))
}

// firmaDeMetodoAttr: el atributo Signature genérico del método, o nil si no lo lleva. Si un argumento
// de tipo no está entre los fuentes de esta pasada (un fuente suelto de `check`), se omite sin
// falsificar el error: el descriptor sigue siendo el del borrado.
func (e *emisor) firmaDeMetodoAttr(m jmetodo) ([]byte, error) {
	firma, err := e.ctx.firmaDeMetodo(m)
	if err != nil {
		if strings.Contains(err.Error(), "no se resuelve") {
			return nil, nil
		}
		return nil, fmt.Errorf("linea %d: %w", m.linea, err)
	}
	if firma == "" {
		return nil, nil
	}
	return attrInfo(e.pool, "Signature", u2be(uint16(e.pool.utf8(firma)))), nil
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
		if len(m.anotaciones) > 0 {
			b, err := e.anotacionesAttr(m.anotaciones)
			if err != nil {
				return nil, err
			}
			attrs = append(attrs, b...)
			conte++
		}
		if b, err := e.firmaDeMetodoAttr(m); err != nil {
			return nil, err
		} else if b != nil {
			attrs = append(attrs, b...)
			conte++
		}
		return e.methodInfo(m.nombre, desc, acc, attrs, conte), nil
	}
	// Un cuerpo vacio `{ }` no es abstracto: emite el `return` implicito con la linea de la llave.
	e.preparaMetodo(desc, m.esEstatico)
	e.metodo = m.nombre
	e.esCtor = m.nombre == "<init>"
	if !m.esEstatico {
		e.reserva("", "L") // this en el hueco 0
	}
	for _, p := range m.params {
		d, err := e.ctx.descriptorDe(p.tipo)
		if err != nil {
			return nil, fmt.Errorf("linea %d: %w", p.donde.linea, err)
		}
		// el parámetro trae su tipo declarado del AST: `List<Apk> m` guarda los argumentos en
		// `p.generico` y el descriptor se queda en el borrado. Un parámetro sin argumentos se apunta
		// como lo que es —un tipo crudo— para que la estimación de `m.get(0)` sepa que el borrado es
		// de verdad el tipo de Java.
		generico := p.generico
		if generico == nil {
			generico = &tipoArgumento{nombre: p.tipo}
		}
		e.reservaConGenerico(p.nombre, d, generico)
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
		return nil, fmt.Errorf("linea %d: el pool de %s pasa de 255 entradas y el indice no cabe en el `ldc` que se escribio", m.linea, e.interno)
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
	if m.deprecado {
		// javac escribe primero `Deprecated` (longitud 0) y después RuntimeVisibleAnnotations con
		// una sola entrada sin pares, medido en §170.
		attrs = append(attrs, attrInfo(e.pool, "Deprecated", nil)...)
		anot := u2be(1)
		anot = append(anot, u2be(uint16(e.pool.utf8("Ljava/lang/Deprecated;")))...)
		anot = append(anot, u2be(0)...)
		attrs = append(attrs, attrInfo(e.pool, "RuntimeVisibleAnnotations", anot)...)
		conte += 2
	}
	if len(m.anotaciones) > 0 {
		b, err := e.anotacionesAttr(m.anotaciones)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, b...)
		conte++
	}
	if b, err := e.firmaDeMetodoAttr(m); err != nil {
		return nil, err
	} else if b != nil {
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
	e.esCtor = true
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
	if e.c.esPrivada {
		// javac marca `private` el constructor sintetizado de una anidada privada (medido con
		// javap 17 --release 8 sobre Capa$Hija)
		acc |= accPrivate
	}
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	return e.methodInfo("<init>", "()V", acc, attrInfo(e.pool, "Code", codigo), 1), nil
}

// initMarcador: el constructor sintetico que javac escribe en una anidada privada instanciada desde
// fuera. Recibe la clase marcador como ultimo parametro y delega en el constructor original:
// `aload_0; [args...]; invokespecial this.<init>(args); return` (medido con javap 17 sobre Capa.java).
func (e *emisor) initMarcador(marcador string) ([]byte, error) {
	desc := "(L" + marcador + ";)V"
	e.preparaMetodo(desc, false)
	e.esCtor = true
	e.reserva("", "L")
	e.reserva("", "L"+marcador+";")
	e.marcaLinea(e.c.linea)
	e.cargaDe(0, "L")
	e.invoca(opInvokespec, e.interno, "<init>", "()V", 1)
	e.emite(retorno('V'))
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	return e.methodInfo("<init>", desc, accSynthetic, attrInfo(e.pool, "Code", codigo), 1), nil
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

// sintetizaPuentesCovariantes es el interruptor de la sonda del puente del retorno covariante.
// Apagado, el emisor escribe solo el metodo estrecho —el defecto original: `grep -c getItem` daba 1
// donde javac escribe 2— y el dex de la clase deja de coincidir con el de javac. Encendido en el
// binario que se publica; lo usa la falsificación de `javaemisor_appadapter_cast_test.go`.
var sintetizaPuentesCovariantes = true

// puentesCovariantes: los `method_info` sinteticos que javac añade cuando una clase sobrescribe un
// metodo heredado con un retorno MAS ESTRECHO que su descriptor borrado. Es el caso de `AppAdapter`
// de ApkShare: `public App getItem(int)` sobrescribe `ArrayAdapter<App>.getItem`, cuyo borrado
// devuelve `Object`, así que javac sintetiza `getItem:(I)Ljava/lang/Object;`. Sin el puente, quien
// llama por la firma heredada (`invokevirtual ArrayAdapter.getItem:(I)Ljava/lang/Object;`) no
// encuentra en la clase el método que el descriptor promete: el verificador de ART rechaza el fichero.
// Medido con `javac 17 --release 8 -g:lines,source` sobre los fuentes reales: `grep -c getItem` da 2
// en `javap -p` (nosotros escribíamos 1) y el puente sale con `0x1041`, `aload_0`, los parámetros,
// `invokevirtual` contra la propia clase, `areturn`, sin StackMapTable y con una sola fila de
// LineNumberTable en 0 apuntando a la línea de la declaración de la clase (la 21 de
// `AppAdapter.java`).
func (e *emisor) puentesCovariantes() ([][]byte, error) {
	salida := [][]byte{}
	if e.c.esInterface || !sintetizaPuentesCovariantes {
		return salida, nil
	}
	for i := range e.c.metodos {
		m := &e.c.metodos[i]
		if m.nombre == "<init>" || m.nombre == "<clinit>" || m.abstracto || m.esEstatico || m.esPrivado {
			// una privada o una estatica no se sobrescribe, y un abstracto no tiene cuerpo que delegar
			continue
		}
		desc, err := e.ctx.descDeMetodo(*m)
		if err != nil {
			// el tipo que no resuelve ya lo dice el emisor del metodo al escribirlo
			continue
		}
		vuelta := vueltaDe(desc)
		if !esTipoDeReferencia(vuelta) {
			continue
		}
		heredada, hay, err := e.ctx.retornoHeredado(e.interno, m, desc)
		if err != nil {
			return nil, err
		}
		if !hay {
			continue
		}
		params := desc[1 : strings.Index(desc, ")")+1]
		b, err := e.puenteCovariante(m.nombre, "("+params+heredada, desc, e.flagsDeMetodo(*m))
		if err != nil {
			return nil, fmt.Errorf("linea %d: el puente de %s: %w", m.linea, m.nombre, err)
		}
		salida = append(salida, b)
	}
	return salida, nil
}

// retornoHeredado: el retorno del metodo que `m` sobrescribe con un tipo mas estrecho, si lo hay. Se
// recorre la jerarquia con `candidatosDe`, que ya sabe cruzar de los fuentes al classpath, y se queda
// con la primera firma heredada con los MISMOS parametros borrados, otro retorno y al que el nuestro
// se le pueda asignar —la covarianza de JLS 8.4.5; `App` es `Object`—. Las privadas y las estaticas
// quedan fuera porque no se sobrescriben, y una vuelta primitiva no puede estrecharse a referencia.
func (ctx *contextoJava) retornoHeredado(interno string, m *jmetodo, desc string) (string, bool, error) {
	cands, err := ctx.candidatosDe(interno, m.nombre, len(m.params), false)
	if err != nil {
		return "", false, err
	}
	_, vuelta, params := splitDescriptor(desc)
	firma := strings.Join(params, "|")
	for _, c := range cands {
		if c.decl == m {
			// la declaracion propia, que `candidatosDe` lista primero
			continue
		}
		if c.acceso&(accStatic|accPrivate) != 0 {
			continue
		}
		_, heredada, paramsHeredados := splitDescriptor(c.desc)
		if strings.Join(paramsHeredados, "|") != firma || heredada == vuelta {
			continue
		}
		if esTipoDeReferencia(heredada) && esTipoDeReferencia(vuelta) && ctx.asignableEstricto(vuelta, heredada) {
			return heredada, true, nil
		}
	}
	return "", false, nil
}

// puenteCovariante: el cuerpo del puente: `aload_0`, los huecos de los parametros, `invokevirtual`
// del metodo estrecho contra la propia clase y el `return` de su vuelta. Los flags son los del
// metodo que delega mas `ACC_BRIDGE` y `ACC_SYNTHETIC` (0x1041 para un `public`), y la unica fila de
// la LineNumberTable es la de la declaracion de la clase, que es lo que javac pone en un sintetico.
func (e *emisor) puenteCovariante(nombre, desc, delegado string, acc int) ([]byte, error) {
	e.preparaMetodo(desc, false)
	e.reserva("", "L")
	_, _, params := splitDescriptor(desc)
	for _, p := range params {
		e.reserva("", p)
	}
	e.marcaLinea(e.c.linea)
	e.cargaDe(0, "L")
	for i, p := range params {
		e.cargaDe(1+i, p)
	}
	e.invoca(opInvokevirt, e.interno, nombre, delegado, 1+len(params))
	vuelta := vueltaDe(desc)
	e.emite(retorno(vuelta[0]))
	e.bajaPor(vuelta)
	if e.prof != 0 {
		return nil, fmt.Errorf("el puente de %s deja %d valores en la pila al cerrar", nombre, e.prof)
	}
	codigo, err := e.codeAttr()
	if err != nil {
		return nil, err
	}
	return e.methodInfo(nombre, desc, acc|accBridge|accSynthetic, attrInfo(e.pool, "Code", codigo), 1), nil
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
		// `catch`. Un try-with-resources cae por el camino normal, así que se mira el cuerpo
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
