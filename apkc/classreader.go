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

	// The three attributes d8 turns into dalvik annotations. They are read as presence,
	// not content: which annotation a class carries follows from having them at all, and
	// the content only matters when the annotations are emitted for real.
	EnclosingMethod bool
	EnclosingClass  bool

	// p is the class's own constant pool. The dexer's interning follows operands that name
	// pool entries, so the pool belongs with the class it came from; it is not part of what
	// the shape of the class means.
	p *pool
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
	// a dalvik annotation
	Parameters bool
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
			if a.name == "ConstantValue" {
				f.readConstant(x, a.body)
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
			case "Exceptions", "MethodParameters":
				if a.name == "MethodParameters" {
					m.Parameters = true
					continue
				}
				r := &cursor{b: a.body}
				for _, idx := range r.u2s() {
					m.Thrown = append(m.Thrown, x.classDescriptor(idx))
				}
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
			cf.EnclosingMethod = true
		case "EnclosingClass":
			cf.EnclosingClass = true
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
	// the receiver is a local slot too, and the dex wants args_size, not locals
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

// refOf takes a fieldref, methodref or interfaceMethodref apart: owner in descriptor form, plus
// the name and the type behind it. iface says interface method, which a dex calls invoke-interface.
func (x poolIndex) refOf(i int) (owner, name, desc string, iface bool) {
	b := x.p.body[i]
	if len(b) < 4 {
		return "", "", "", false
	}
	name, desc = x.nameAndType(int(b[2])<<8 | int(b[3]))
	if c, ok := x.p.class[int(b[0])<<8|int(b[1])]; ok {
		owner = descriptorOf(x.p.utf[c])
	}
	return owner, name, desc, x.p.tag[i] == cpInterfaceMethodref
}

func (x poolIndex) classDescriptor(i int) string {
	if c, ok := x.p.class[i]; ok {
		return descriptorOf(x.p.utf[c])
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

// refuseOutsideSubset names the two things this fork does not dex. An invokedynamic or a dynamic
// constant is a lambda or the string concatenation javac emits since JDK 9, and both need d8's
// desugaring plus the bootstrap table that goes with them.
func (x poolIndex) refuseOutsideSubset() error {
	for i, t := range x.p.tag {
		switch t {
		case cpInvokeDynamic:
			return fmt.Errorf("pool entry %d is an invokedynamic: lambdas and dynamic concatenation are not dexed here (docs/plan.md, 2G.5)", i)
		case cpDynamic:
			return fmt.Errorf("pool entry %d is a dynamic constant, which only a bootstrap method produces: not dexed here", i)
		case cpMethodHandle, cpMethodType:
			return fmt.Errorf("pool entry %d is a method handle or type, used by dynamic calls: not desugared in this fork", i)
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
