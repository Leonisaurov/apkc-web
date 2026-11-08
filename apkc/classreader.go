package main

// The class reader the dexer needs, which is more than the dependency index needs.
//
// classfile.go answers "what shape does this class present to its callers": names, descriptors,
// referenced classes. That is enough to decide what to recompile and nothing else, and dexing
// asks for the rest: the code of every method, the pool entries that code points at, the line
// numbers a stack trace reads, and the handlers a try/catch becomes an encoded_catch of.
//
// So this reads the whole of JVMS 4.5 through 4.7 and stops at the edge the plan declares: a
// class whose pool holds a dynamic call or a method handle is refused by name rather than
// half-dexed. The difference between "not supported" and "silently wrong" is that someone says
// which one it is.
//
// The oracle for all of it is javap: the numbers the tests assert are what javap -v -c printed
// over the six classes of the studio project on this phone, 2026-10-27.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// The access flags of JVMS 4.1 that a dex carries over in its own form. ACC_SUPER is kept because
// javac writes it and d8's class_def access_flags include it: dropping it would change bytes of
// output for no reason.
const (
	accPublic    = 0x0001
	accPrivate   = 0x0002
	accProtected = 0x0004
	accStatic    = 0x0008
	accFinal     = 0x0010
	accSuper     = 0x0020
	accSynthetic = 0x1000
	accInterface = 0x0200
	accAbstract  = 0x0400
)

// classFull is one compiled class, read for dexing.
type classFull struct {
	Name               string // descriptor form: Lcom/x/Y;
	Major              int
	Access             int
	Super              string // empty only for java/lang/Object
	Interfaces         []string
	SourceFile         string
	Fields             []classField
	Methods            []classMethod
	Inners             []classInner
	VisibleAnnotations []byte // raw, so the markers survive without interpreting them

	// The three attributes d8 turns into dalvik annotations. Presence is what the dexer needed
	// until paragraph 58; from there the content is read too, because the annotation carries it:
	// EnclosingMethod names the class and the method, MethodParameters lists one entry per
	// argument, and Signature is a string index.
	EnclosingMethod bool
	EnclosingClass  bool
	Enclosing       classEnclosing
	Signature       string // el attribute Signature de la clase, en texto MUTF-8

	// BootstrapMethods y los sitios desugrados: las lambdas (lambda.go) y la concatenación
	// dinámica (concat.go). Los mapas van por pc y solo llevan los invokedynamic que este fork
	// convierte; los demás se rechazan por nombre.
	BootstrapMethods []classBootstrap
	Lambdas          map[int]classLambda
	Concats          map[int]classConcat

	// p is the class's own constant pool. The dexer's interning follows operands that name
	// pool entries, so the pool belongs with the class it came from; it is not part of what
	// the shape of the class means.
	p *pool
}

// classEnclosing: el contenido del atributo EnclosingMethod. Nombre y descriptor vienen vacios
// cuando el atributo anuncia metodo ausente (una clase local dentro de un inicializador estatico).
type classEnclosing struct {
	clase  string
	nombre string
	desc   string
}

// classParametro: una fila del atributo MethodParameters, que d8 copia en la anotacion
// Ldalvik/annotation/MethodParameters; con el nombre y los flags que javac dejo.
type classParametro struct {
	nombre string // vacio cuando el nombre es 0, o sea no declarado
	access int
}

type classInner struct {
	inner  string
	outer  string
	name   string
	access int
}

type classField struct {
	Access int
	Name   string
	Desc   string

	// ConstantValue, resolved: javac inlines it into callers and a dex keeps it as an
	// encoded_value in the static field list
	HasConst  bool
	ConstTag  byte
	ConstInt  int32
	ConstLong int64
	ConstStr  string

	// Signature es el atributo del campo para los genéricos; d8 lo copia troceado en
	// `dalvik.annotation.Signature` (parrafo 101).
	Signature string
}

type classMethod struct {
	Access int
	Name   string
	Desc   string

	HasCode    bool
	MaxStack   int
	LocalCount int
	ArgCount   int
	Code       []byte
	Tries      []classTry
	Lines      []classLine
	Thrown     []string
	// Parameters marks a method that carries the MethodParameters attribute, which d8 keeps as
	// a dalvik annotation. Desde el parrafo 58 vienen tambien las filas: la anotacion lleva un
	// array de access_flags y otro de nombres (null cuando el nombre no esta declarado).
	Parameters bool
	Parametros []classParametro
	Signature  string // el attribute Signature del metodo, en texto
}

type classTry struct {
	start, end, handler int
	catchType           string // empty for a finally, which a dex writes as "catch anything"
}

type classLine struct {
	pc, line int
}

// readClassFull takes everything the dexer asks of one class file.
func readClassFull(data []byte) (classFull, error) {
	c := &cursor{b: data}
	if c.u4() != classMagic {
		return classFull{}, errors.New("not a class file")
	}
	var cf classFull
	c.u2() // minor_version: no dex cares
	cf.Major = c.u2()

	p, err := c.constantPool()
	if err != nil {
		return classFull{}, err
	}
	x := poolIndex{p: p}
	if err := x.refuseOutsideSubset(); err != nil {
		return classFull{}, err
	}

	cf.Access = c.u2()
	cf.Name = x.classDescriptor(c.u2())
	if superIdx := c.u2(); superIdx != 0 {
		cf.Super = x.classDescriptor(superIdx)
	}
	// the dependency index wants dotted names; a dex wants descriptors, so the interfaces are
	// resolved here rather than reused from cursor.classes
	for _, idx := range c.u2s() {
		cf.Interfaces = append(cf.Interfaces, x.classDescriptor(idx))
	}

	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		f := classField{Access: c.u2()}
		f.Name = p.utf[c.u2()]
		f.Desc = p.utf[c.u2()]
		for _, a := range c.attributeTable(p) {
			switch a.name {
			case "ConstantValue":
				f.readConstant(x, a.body)
			case "Signature":
				r := &cursor{b: a.body}
				f.Signature = p.utf[r.u2()]
			}
		}
		cf.Fields = append(cf.Fields, f)
	}

	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		m := classMethod{Access: c.u2()}
		m.Name = p.utf[c.u2()]
		m.Desc = p.utf[c.u2()]
		for _, a := range c.attributeTable(p) {
			switch a.name {
			case "Code":
				if err := m.readCode(x, a.body); err != nil {
					return classFull{}, fmt.Errorf("%s%s: %w", m.Name, m.Desc, err)
				}
			case "Exceptions":
				r := &cursor{b: a.body}
				for _, idx := range r.u2s() {
					m.Thrown = append(m.Thrown, x.classDescriptor(idx))
				}
			case "MethodParameters":
				// JVMS 4.7.10: el conteo es un u1 (medido sobre el .class de la anonima del studio:
				// el cuerpo sale 01 00 00 80 10, o sea un parametro, nombre ausente y flags 0x8010 =
				// final|mandated, que es el valor que d8 copia en la anotacion) y por cada parametro
				// el indice del nombre, que es 0 cuando no esta declarado, y sus flags: los del .class
				// y no los del dex
				m.Parameters = true
				r := &cursor{b: a.body}
				for i, n := 0, r.u1(); i < n && !r.failed(); i++ {
					var nombre string
					if idx := r.u2(); idx != 0 {
						nombre = p.utf[idx]
					}
					m.Parametros = append(m.Parametros, classParametro{nombre: nombre, access: r.u2()})
				}
			case "Signature":
				r := &cursor{b: a.body}
				m.Signature = p.utf[r.u2()]
			}
		}
		cf.Methods = append(cf.Methods, m)
	}

	for _, a := range c.attributeTable(p) {
		switch a.name {
		case "SourceFile":
			r := &cursor{b: a.body}
			cf.SourceFile = p.utf[r.u2()]
		case "InnerClasses":
			cf.readInners(x, a.body)
		case "RuntimeVisibleAnnotations":
			cf.VisibleAnnotations = a.body
		case "EnclosingMethod":
			// JVMS 4.7.7: dos u2, la clase que envuelve y UNA entrada del pool con el metodo. d8 lo
			// convierte en Ldalvik/annotation/EnclosingMethod; con un indice de method_ids, asi que
			// el nombre y el descriptor hacen falta aqui.
			cf.EnclosingMethod = true
			r := &cursor{b: a.body}
			cf.Enclosing.clase = x.classDescriptor(r.u2())
			cf.Enclosing.nombre, cf.Enclosing.desc = x.metodoDelPool(r.u2())
		case "EnclosingClass":
			cf.EnclosingClass = true
		case "Signature":
			r := &cursor{b: a.body}
			cf.Signature = p.utf[r.u2()]
		case "BootstrapMethods":
			// JVMS 4.7.23: conteo, y por cada fila el MethodHandle y su lista de argumentos.
			r := &cursor{b: a.body}
			for i, n := 0, r.u2(); i < n && !r.failed(); i++ {
				bm := classBootstrap{handle: r.u2()}
				bm.args = append(bm.args, r.u2s()...)
				cf.BootstrapMethods = append(cf.BootstrapMethods, bm)
			}
		}
	}
	cf.p = p
	if c.failed() {
		return classFull{}, fmt.Errorf("class %s ends inside %s", cf.Name, c.reason())
	}
	return cf, nil
}

func (f *classField) readConstant(x poolIndex, body []byte) {
	r := &cursor{b: body}
	idx := r.u2()
	f.HasConst = true
	f.ConstTag = x.tagOf(idx)
	switch f.ConstTag {
	case cpInteger:
		f.ConstInt = x.intOf(idx)
	case cpString:
		f.ConstStr = x.stringValue(idx)
	case cpLong:
		f.ConstLong = x.longOf(idx)
	case cpFloat:
		f.ConstInt = int32(math.Float32bits(x.floatOf(idx)))
	case cpDouble:
		f.ConstLong = int64(math.Float64bits(x.doubleOf(idx)))
	}
}

// attribute is one entry of an attribute table, kept with its bytes: the caller decides what to
// interpret, and StackMapTable or LocalVariableTable are not what a dex needs.
type attribute struct {
	name string
	body []byte
}

// attributeTable reads the count-and-list of JVMS 4.7, resolving the names against the pool.
func (c *cursor) attributeTable(p *pool) []attribute {
	n := c.u2()
	out := make([]attribute, 0, n)
	for i := 0; i < n && !c.failed(); i++ {
		nameIdx := c.u2()
		body := c.u4bytes()
		out = append(out, attribute{name: p.utf[nameIdx], body: body})
	}
	return out
}

// readCode parses JVMS 4.7.3: the two limits, the instructions, the handlers, and the inner
// attributes whose only job here is line numbers.
func (m *classMethod) readCode(x poolIndex, body []byte) error {
	r := &cursor{b: body}
	m.HasCode = true
	m.MaxStack = r.u2()
	m.LocalCount = r.u2()
	n := int(r.u4())
	m.Code = r.bytes(n)
	if r.failed() {
		return fmt.Errorf("the Code attribute claims %d bytes: %s", n, r.reason())
	}
	for i, total := 0, r.u2(); i < total && !r.failed(); i++ {
		t := classTry{start: r.u2(), end: r.u2(), handler: r.u2()}
		if idx := r.u2(); idx != 0 {
			t.catchType = x.classDescriptor(idx)
		}
		m.Tries = append(m.Tries, t)
	}
	// the receiver is a local slot too, and the dex wants args_size, not locals. `countArgs` ya cuenta
	// HUECOS: un `long` vale dos, medido en el `ins=4` de `V.cuenta(JJ)J` del parrafo 74.
	m.ArgCount = countArgs(m.Desc)
	if m.Access&accStatic == 0 {
		m.ArgCount++
	}
	for _, a := range r.attributeTable(x.p) {
		if a.name != "LineNumberTable" {
			continue
		}
		t := &cursor{b: a.body}
		for i, n := 0, t.u2(); i < n && !t.failed(); i++ {
			m.Lines = append(m.Lines, classLine{pc: t.u2(), line: t.u2()})
		}
	}
	return nil
}

func (cf *classFull) readInners(x poolIndex, body []byte) {
	r := &cursor{b: body}
	for i, n := 0, r.u2(); i < n && !r.failed(); i++ {
		inner, outer, name := r.u2(), r.u2(), r.u2()
		in := classInner{access: r.u2()}
		if inner != 0 {
			in.inner = x.classDescriptor(inner)
		}
		if outer != 0 {
			in.outer = x.classDescriptor(outer)
		}
		if name != 0 {
			in.name = x.p.utf[name]
		}
		cf.Inners = append(cf.Inners, in)
	}
}

// poolIndex resolves the pool entries the code points at. The pool of classfile.go kept the three
// things a shape needs; this adds what a translation needs, over the same bytes.
type poolIndex struct{ p *pool }

func (x poolIndex) nameAndType(i int) (string, string) {
	b := x.p.body[i]
	if len(b) < 4 {
		return "", ""
	}
	return x.p.utf[int(b[0])<<8|int(b[1])], x.p.utf[int(b[2])<<8|int(b[3])]
}

// metodoDelPool: nombre y descriptor de la entrada que nombra el segundo campo del atributo
// EnclosingMethod. Medido con `javap -v` sobre la anonima del studio (2026-11-01): javac 21 deja un
// NameAndType (#37 = NameAndType #38:#39 -> onCreateOptionsMenu, (Landroid/view/Menu;)Z), no dos
// indices separados; la especificacion habla de una referencia a metodo, que trae delante el indice
// de la clase. Se prueban las dos formas y lo que no resulte en un descriptor sale vacio.
func (x poolIndex) metodoDelPool(i int) (string, string) {
	nombre, desc := x.nameAndType(i)
	if nombre != "" && len(desc) > 0 && desc[0] == '(' {
		return nombre, desc
	}
	b := x.p.body[i]
	if len(b) >= 4 {
		nombre, desc = x.nameAndType(int(b[2])<<8 | int(b[3]))
		if nombre != "" && len(desc) > 0 && desc[0] == '(' {
			return nombre, desc
		}
	}
	return "", ""
}

// refOf takes a fieldref, methodref or interfaceMethodref apart: owner in descriptor form, plus
// the name and the type behind it. iface says interface method, which a dex calls invoke-interface.
func (x poolIndex) refOf(i int) (owner, name, desc string, iface bool) {
	b := x.p.body[i]
	if len(b) < 4 {
		return "", "", "", false
	}
	name, desc = x.nameAndType(int(b[2])<<8 | int(b[3]))
	if c, ok := x.p.class[int(b[0])<<8|int(b[1])]; ok {
		owner = descriptorDeClase(x.p.utf[c])
	}
	return owner, name, desc, x.p.tag[i] == cpInterfaceMethodref
}

func (x poolIndex) classDescriptor(i int) string {
	if c, ok := x.p.class[i]; ok {
		return descriptorDeClase(x.p.utf[c])
	}
	return ""
}

func (x poolIndex) stringValue(i int) string {
	b := x.p.body[i]
	if len(b) < 2 {
		return ""
	}
	return x.p.utf[int(b[0])<<8|int(b[1])]
}

func (x poolIndex) intOf(i int) int32  { return int32(binary.BigEndian.Uint32(x.p.body[i])) }
func (x poolIndex) longOf(i int) int64 { return int64(binary.BigEndian.Uint64(x.p.body[i])) }

func (x poolIndex) floatOf(i int) float32 {
	return math.Float32frombits(binary.BigEndian.Uint32(x.p.body[i]))
}
func (x poolIndex) doubleOf(i int) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(x.p.body[i]))
}

func (x poolIndex) tagOf(i int) byte { return x.p.tag[i] }

// refuseOutsideSubset names the one thing this fork still does not dex. An invokedynamic of a
// lambda without captures is desugared in lambda.go; the rest (captures, method references of an
// instance, altMetafactory, the string concatenation of JDK 9) is refused there with its name. A
// dynamic constant only exists to feed a bootstrap, and none of the measured bootstraps produce one.
func (x poolIndex) refuseOutsideSubset() error {
	for i, t := range x.p.tag {
		switch t {
		case cpDynamic:
			return fmt.Errorf("pool entry %d is a dynamic constant, which only a bootstrap method produces: not dexed here", i)
		}
	}
	return nil
}

// countArgs is how many local slots the parameters take: a long or a double takes two.
func countArgs(desc string) int {
	if desc == "" || desc[0] != '(' {
		return 0
	}
	end := 0
	for i := 0; i < len(desc); i++ {
		if desc[i] == ')' {
			end = i
			break
		}
	}
	n := 0
	for i := 1; i < end; {
		switch desc[i] {
		case 'J', 'D':
			n += 2
			i++
		case 'L':
			for i < end && desc[i] != ';' {
				i++
			}
			n++
			i++
		case '[':
			i++
			for i < end && desc[i] == '[' {
				i++
			}
			if i < end && desc[i] == 'L' {
				for i < end && desc[i] != ';' {
					i++
				}
			}
			n++
			i++
		default:
			n++
			i++
		}
	}
	return n
}

// descriptorDeClase: el Utf8 de una entrada `Class` es un nombre **interno**, no un descriptor; una
// clase llamada `B` o `I` en el paquete por defecto es legal y no es el primitivo del mismo nombre
// (lo destapó una sonda llamada `B`: el dex quedaba con el type_id "B" y `dexdump` lo rechazaba con
// `Invalid descriptor for class_idx`), y una llamada `L` empieza por la `L` de un descriptor sin
// serlo (la sonda `L` dio `Invalid type descriptor: 'L'`). Se reconoce un descriptor ya escrito solo
// por `L…;` cerrado o por el corchete de un arreglo.
func descriptorDeClase(name string) string {
	if name == "" || name[0] == '[' {
		return name
	}
	if name[0] == 'L' && name[len(name)-1] == ';' {
		return name
	}
	return "L" + name + ";"
}

// descriptorOf turns the internal name javac writes (com/x/Y) into what a dex holds (Lcom/x/Y;).
// A primitive or an array is already a descriptor and stays as it is.
func descriptorOf(name string) string {
	if name == "" {
		return ""
	}
	if name[0] == 'L' || name[0] == '[' {
		return name
	}
	if len(name) == 1 && stringsContainsAny("BCDFIJSVZ", name[0]) {
		return name
	}
	return "L" + name + ";"
}

func stringsContainsAny(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}
