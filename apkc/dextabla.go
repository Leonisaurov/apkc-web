package main

// Interning: the five index tables of a dex, derived from the classes that go into it.
//
// A dex does not carry pool indices. It has one table of each kind of item, deduplicated across the
// whole file, in an order the format fixes, and every reference is an index into one of those
// tables. So the writer's first job is not bytes but *sets and order*, and the sets are what this
// file computes: it walks the classes, takes the descriptors and names they use, follows the
// operands of each instruction to whatever the code refers to, and interns everything it finds.
//
// The order rules are claimed here and then MEASURED against d8's own tables in the test
// (dextabla_test.go), which reads the real classes.dex with the reader of dexreader.go. Where a
// rule is a guess, the test is the thing that says so; the comment in front of each sort is what
// was verified, not what was assumed.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type protoKey struct {
	shorty string
	ret    string
	params string // the descriptors joined, the only form the key needs
}

type fieldKey struct{ owner, typeOf, name string }
type methodKey struct {
	owner string
	proto protoKey // the interned key, so the sort never has to rebuild it from a descriptor
	desc  string   // the method descriptor, for messages
	name  string
}

// dexTables is the interned state of one classes.dex being built.
type dexTables struct {
	strings []string
	types   []string
	protos  []protoKey
	fields  []fieldKey
	methods []methodKey

	stringIdx map[string]int
	typeIdx   map[string]int
	protoIdx  map[protoKey]int
	fieldIdx  map[fieldKey]int
	methodIdx map[methodKey]int
}

func newDexTables() *dexTables {
	return &dexTables{
		stringIdx: map[string]int{}, typeIdx: map[string]int{}, protoIdx: map[protoKey]int{},
		fieldIdx: map[fieldKey]int{}, methodIdx: map[methodKey]int{},
	}
}

func (t *dexTables) internString(s string) {
	if _, ok := t.stringIdx[s]; ok {
		return
	}
	t.stringIdx[s] = len(t.strings)
	t.strings = append(t.strings, s)
}

func (t *dexTables) internType(desc string) {
	if desc == "" {
		return
	}
	if _, ok := t.typeIdx[desc]; !ok {
		t.typeIdx[desc] = len(t.types)
		t.types = append(t.types, desc)
	}
	t.internString(desc)
}

// internProto takes a method descriptor apart and interns the three pieces a proto_id_item names:
// the shorty, the return type and the parameter list.
func (t *dexTables) internProto(desc string) protoKey {
	shorty, ret, params := splitDescriptor(desc)
	k := protoKey{shorty: shorty, ret: ret, params: strings.Join(params, "")}
	if _, ok := t.protoIdx[k]; !ok {
		t.protoIdx[k] = len(t.protos)
		t.protos = append(t.protos, k)
	}
	t.internString(shorty)
	t.internType(ret)
	for _, p := range params {
		t.internType(p)
	}
	return k
}

func (t *dexTables) internField(owner, typeOf, name string) {
	k := fieldKey{owner: owner, typeOf: typeOf, name: name}
	if _, ok := t.fieldIdx[k]; !ok {
		t.fieldIdx[k] = len(t.fields)
		t.fields = append(t.fields, k)
	}
	t.internType(owner)
	t.internType(typeOf)
	t.internString(name)
}

func (t *dexTables) internMethod(owner, desc, name string) {
	k := methodKey{owner: owner, proto: t.internProto(desc), desc: desc, name: name}
	if _, ok := t.methodIdx[k]; !ok {
		t.methodIdx[k] = len(t.methods)
		t.methods = append(t.methods, k)
	}
	t.internType(owner)
	t.internProto(desc)
	t.internString(name)
}

// collectClasses interns everything the six (or one, or a hundred) classes refer to: their own
// names, supertypes, interfaces, member names and descriptors, and every pool entry the code
// points at.
func (t *dexTables) collectClasses(classes []classFull) error {
	for _, cf := range classes {
		t.internType(cf.Name)
		t.internType(cf.Super)
		for _, in := range cf.Interfaces {
			t.internType(in)
		}
		if cf.SourceFile != "" {
			t.internString(cf.SourceFile)
		}
		for _, f := range cf.Fields {
			t.internField(cf.Name, f.Desc, f.Name)
		}
		for _, m := range cf.Methods {
			t.internMethod(cf.Name, m.Desc, m.Name)
			if !m.HasCode {
				continue
			}
			if err := t.collectCode(m, cf.p); err != nil {
				return fmt.Errorf("%s%s: %w", cf.Name, m.Name, err)
			}
		}
		// d8 convierte los atributos de anidamiento y de parametros en anotaciones de
		// dalvik, y una anotacion es un type_id mas: esto se midio contra sus tablas
		// (§19), no se supuso.
		for _, in := range cf.Inners {
			t.internType(in.inner)
			t.internType(in.outer)
			if in.name != "" {
				t.internString(in.name)
			}
			if in.inner == cf.Name {
				t.internType("Ldalvik/annotation/InnerClass;")
				t.internString("accessFlags")
				t.internString("name")
			}
			if in.outer == cf.Name {
				t.internType("Ldalvik/annotation/MemberClasses;")
				t.internString("value")
				t.internString("name")
			}
		}
		if cf.EnclosingMethod {
			t.internType("Ldalvik/annotation/EnclosingMethod;")
		}
		for _, in := range cf.Inners {
			// una clase miembro con nombre se anuncia con @EnclosingClass; la anonima
			// no la lleva y usa @EnclosingMethod, que es lo que sale de las tablas de
			// d8 sobre estas seis clases (§19): R$id, R$layout y R$style la piden,
			// MainActivity$1 no
			if in.inner == cf.Name && in.outer != "" && in.name != "" {
				t.internType("Ldalvik/annotation/EnclosingClass;")
			}
		}
		for _, m := range cf.Methods {
			if m.Parameters {
				t.internType("Ldalvik/annotation/MethodParameters;")
				t.internString("accessFlags")
				t.internString("names")
			}
		}
	}
	return nil
}

// collectCode follows the operands of the instructions the walker knows. Each of the sixteen has a
// different kind of operand — a pool index for a field, a type for new, a string for ldc — and the
// table below is what the dexer needs to know which one, exactly like the translation does.
func (t *dexTables) collectCode(m classMethod, p *pool) error {
	if p == nil {
		return errors.New("the class came without its constant pool")
	}
	x := poolIndex{p: p}
	list, err := walkJvmCode(m.Code)
	if err != nil {
		return err
	}
	for _, in := range list {
		switch in.name {
		case "ldc":
			if len(in.ops) < 1 {
				return fmt.Errorf("ldc without its operand")
			}
			t.followConstant(x, int(in.ops[0]))
		case "getfield", "putfield":
			owner, name, desc, _ := x.refOf(dexU16(in.ops))
			t.internField(owner, desc, name)
		case "invokevirtual", "invokespecial", "invokestatic", "invokeinterface":
			owner, name, desc, iface := x.refOf(dexU16(in.ops))
			_ = iface
			t.internMethod(owner, desc, name)
		case "new":
			t.internType(x.classDescriptor(dexU16(in.ops)))
		}
	}
	return nil
}

// followConstant interns what a pool entry named by ldc stands for: a string, an int type marker, a
// class descriptor or a method type.
func (t *dexTables) followConstant(x poolIndex, idx int) {
	switch x.tagOf(idx) {
	case cpString:
		t.internString(x.stringValue(idx))
	case cpClass:
		t.internType(x.classDescriptor(idx))
	case cpMethodType:
		if b := x.p.body[idx]; len(b) >= 2 {
			t.internProto(x.p.utf[int(b[0])<<8|int(b[1])])
		}
	case cpInteger, cpLong, cpFloat, cpDouble:
		// the value itself travels in the code, not in the pool of strings
	}
}

// orderTables puts every table in the order the format asks for. Each sort below was checked
// against the tables d8 wrote: the string order by utf-16 units (measured in 2G-B), types by their
// descriptor's position in the string table, protos by shorty then return then parameter list,
// fields by class then type then name, methods by class then proto then name.
func (t *dexTables) orderTables() {
	sortByMUTF8(t.strings)
	for i, s := range t.strings {
		t.stringIdx[s] = i
	}

	sort.Slice(t.types, func(i, j int) bool { return t.stringIdx[t.types[i]] < t.stringIdx[t.types[j]] })
	for i, s := range t.types {
		t.typeIdx[s] = i
	}

	sort.Slice(t.protos, func(i, j int) bool {
		a, b := t.protos[i], t.protos[j]
		if t.stringIdx[a.shorty] != t.stringIdx[b.shorty] {
			return t.stringIdx[a.shorty] < t.stringIdx[b.shorty]
		}
		if t.typeIdx[a.ret] != t.typeIdx[b.ret] {
			return t.typeIdx[a.ret] < t.typeIdx[b.ret]
		}
		return a.params < b.params
	})
	for i, k := range t.protos {
		t.protoIdx[k] = i
	}

	sort.Slice(t.fields, func(i, j int) bool {
		a, b := t.fields[i], t.fields[j]
		if t.typeIdx[a.owner] != t.typeIdx[b.owner] {
			return t.typeIdx[a.owner] < t.typeIdx[b.owner]
		}
		if t.stringIdx[a.name] != t.stringIdx[b.name] {
			return t.stringIdx[a.name] < t.stringIdx[b.name]
		}
		return t.typeIdx[a.typeOf] < t.typeIdx[b.typeOf]
	})
	for i, k := range t.fields {
		t.fieldIdx[k] = i
	}

	sort.Slice(t.methods, func(i, j int) bool {
		a, b := t.methods[i], t.methods[j]
		if t.typeIdx[a.owner] != t.typeIdx[b.owner] {
			return t.typeIdx[a.owner] < t.typeIdx[b.owner]
		}
		// el orden real de d8 es clase, NOMBRE, proto: measured, y contradice el orden en
		// que se suele citar el formato (clase, proto, nombre). setContentView(VI) sale
		// despues de onCreateOptionsMenu(ZL) en el fichero de d8, y solo con este orden.
		if t.stringIdx[a.name] != t.stringIdx[b.name] {
			return t.stringIdx[a.name] < t.stringIdx[b.name]
		}
		return t.protoIdx[a.proto] < t.protoIdx[b.proto]
	})
	for i, k := range t.methods {
		t.methodIdx[k] = i
	}
}

// protoKeyOf rebuilds the key of a stored descriptor, so the method sort can compare proto indices.
func protoKeyOf(desc string) protoKey {
	shorty, ret, params := splitDescriptor(desc)
	return protoKey{shorty: shorty, ret: ret, params: strings.Join(params, "")}
}

// splitDescriptor takes (Landroid/os/Bundle;)V apart into the shorty d8 writes, the return type and
// the parameter descriptors. The shorty is one letter per item: L for objects and arrays, the
// primitive letter otherwise, which is why "LLI" appears in the string table of the studio project.
func splitDescriptor(desc string) (shorty string, ret string, params []string) {
	if desc == "" || desc[0] != '(' {
		return "", "", nil
	}
	end := strings.IndexByte(desc, ')')
	if end < 0 {
		return "", "", nil
	}
	body := desc[1:end]
	ret = desc[end+1:]
	carta := []byte{}
	for i := 0; i < len(body); {
		switch {
		case body[i] == 'L':
			j := strings.IndexByte(body[i:], ';')
			if j < 0 {
				return "", ret, params
			}
			params = append(params, body[i:i+j+1])
			carta = append(carta, 'L')
			i += j + 1
		case body[i] == '[':
			j := i
			for j < len(body) && body[j] == '[' {
				j++
			}
			if j < len(body) && body[j] == 'L' {
				k := strings.IndexByte(body[j:], ';')
				params = append(params, body[i:j+k+1])
				i = j + k + 1
			} else {
				params = append(params, body[i:j+1])
				i = j + 1
			}
			carta = append(carta, 'L')
		default:
			params = append(params, string(body[i]))
			carta = append(carta, body[i])
			i++
		}
	}
	// el shorty de un dex empieza por el tipo devuelto: makeText(...)Toast es "LLLI" y
	// onCreateOptionsMenu(Menu)Z es "ZL", y los dos estan asi en la tabla de d8 (§13)
	shorty = ret
	if len(ret) != 1 || ret != "V" {
		if strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "[") {
			shorty = "L" + string(carta)
		} else {
			shorty = ret + string(carta)
		}
	} else {
		shorty = "V" + string(carta)
	}
	return shorty, ret, params
}

func dexU16(b []byte) int {
	if len(b) < 2 {
		return 0
	}
	return int(b[0])<<8 | int(b[1])
}

// String is the debug form, and it is what a failing comparison prints: a diff of these tables is
// how a byte-level difference in the dex gets attributed to one rule.
func (t *dexTables) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d strings, %d types, %d protos, %d fields, %d methods\n",
		len(t.strings), len(t.types), len(t.protos), len(t.fields), len(t.methods))
	return b.String()
}
