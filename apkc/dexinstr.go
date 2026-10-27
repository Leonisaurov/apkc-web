package main

// Traducción del código JVM a palabras dalvik, para el subconjunto que este fork dexa.
//
// Cada formato que se emite aquí salió de las palabras que `d8` escribió sobre los mismos
// `.class` (docs/evidencia.md §21): no hay ni un número de opcode ni una disposición de operandos
// tomada de un manual. Lo que no está medido no se emite: se rechaza por nombre, que es la única
// forma de que un proyecto que necesita más formato lo diga en vez de compilar un dex a medias.
//
// El modelo es una pila sobre registros. Cada entrada de la pila JVM es un registro dalvik; los
// argumentos de un método viven arriba del todo (el formato los busca en `registers_size -
// ins_size`, medido en §20) y los temporales debajo. Un `dup` no cuesta una instrucción: empuja el
// mismo registro, igual que hizo `d8` con el `new`/`dup` del listener.

// Los opcodes dalvik que este traductor sabe escribir, con el número que tienen en las palabras de
// d8. Un formato 11x pone el registro en la mitad alta de la primera unidad; un 12x parte esa mitad
// en dos nibbles de cuatro bits; un 21c lleva el índice en la segunda unidad; un 22c hace lo mismo
// con dos registros en nibbles; un 35c cuenta los argumentos y reparte registros entre dos
// unidades. La lista es corta a propósito: lo que falta está dicho en los rechazos.
const (
	opRetorna        = 0x0f // 11x return vAA
	opRetornaV       = 0x0e // 10x return-void
	opMueveResultObj = 0x0c // 11x move-result-object vAA
	opConst4         = 0x12 // 11n const/4 vAA, #+B
	opConst16        = 0x13 // 21c const/16 vAA, #+BBBB
	opConstAlto16    = 0x15 // 21c const/high16 vAA, #+BBBB0000
	opConstString    = 0x1a // 21c const-string vAA, string@BBBB
	opNuevaInstancia = 0x22 // 21c new-instance vAA, type@BBBB
	opIget           = 0x52 // 22c iget vAA, vBB, field@CCCC
	opIgetObject     = 0x54 // 22c iget-object vAA, vBB, field@CCCC
	opIputObject     = 0x5b // 22c iput-object vAA, vBB, field@CCCC
	opInvocaVirtual  = 0x6e // 35c invoke-virtual
	opInvocaSuper    = 0x6f // 35c invoke-super
	opInvocaDirecto  = 0x70 // 35c invoke-direct
	opInvocaEstatico = 0x71 // 35c invoke-static
	opInvocaInterfaz = 0x72 // 35c invoke-interface
)

// traduce es el estado de un método mientras se traduce. Se corre dos veces: la primera con una
// base tan ancha como la pila máxima que midió javap, para saber cuántos temporales hacen falta de
// verdad; la segunda con esa cuenta como base, que es lo que fija dónde caen los argumentos y por
// tanto las palabras definitivas.
type traduce struct {
	t         *dexTables
	x         poolIndex
	clase     classFull
	m         classMethod
	base      int   // primer registro de los locales: cuántos temporales se usan
	locales   int   // m.LocalCount, medido sobre el .class
	enPila    []int // registro de cada entrada de la pila JVM
	libres    []int // temporales ya muertos
	siguiente int   // próximo temporal nuevo
	pico      int   // el mayor número de temporal usado
	w         []uint16
	outs      int // la llamada más ancha, que es el outs_size del §20
	err       error
}

func (p *traduce) falla(in jvmInstr, motivo string) {
	if p.err == nil {
		p.err = &traduceError{pc: in.pc, nombre: in.name, motivo: motivo}
	}
}

type traduceError struct {
	pc     int
	nombre string
	motivo string
}

func (e *traduceError) Error() string {
	return "at pc " + itoa(e.pc) + ", " + e.nombre + ": " + e.motivo
}

// toma: un registro temporal nuevo, o el último que murió. Reutilizar es lo que hace el marco más
// pequeño; la muerte la decide suelta, que mira la pila antes de devolver el registro.
func (p *traduce) toma(in jvmInstr) int {
	if n := len(p.libres); n > 0 {
		r := p.libres[n-1]
		p.libres = p.libres[:n-1]
		return r
	}
	r := p.siguiente
	p.siguiente++
	if p.siguiente > p.pico {
		p.pico = p.siguiente
	}
	if r >= p.base {
		p.falla(in, "it needs more temporaries than the max stack javap measured")
	}
	return r
}

// suelta: devuelve un temporal solo si ninguna otra entrada de la pila lo está usando. Comprobarlo
// cada vez cuesta un recorrido corto y es lo que impide que un dup pierda un valor por escribir dos
// vivos en el mismo registro. Y un local nunca se suelta: por debajo de base solo hay temporales, y
// el registro de un argumento se vuelve a leer en la llamada siguiente.
func (p *traduce) suelta(r int) {
	if r >= p.base {
		return
	}
	for _, vivo := range p.enPila {
		if vivo == r {
			return
		}
	}
	p.libres = append(p.libres, r)
}

func (p *traduce) pop(in jvmInstr) int {
	if len(p.enPila) == 0 {
		p.falla(in, "the JVM stack is empty: there is nothing to drop")
		return 0
	}
	r := p.enPila[len(p.enPila)-1]
	p.enPila = p.enPila[:len(p.enPila)-1]
	return r
}

func (p *traduce) push(r int) { p.enPila = append(p.enPila, r) }

// local: el registro dalvik del enésimo hueco JVM. Un long o un double ocupa dos huecos aquí y dos
// registros allá, así que el número de hueco se pasa tal cual.
func (p *traduce) local(n int) int { return p.base + n }

func (p *traduce) palabra(u uint16) { p.w = append(p.w, u) }

// nibble: los formatos 11n, 12x y 35c guardan el registro en CUATRO bits. Pasar de v15 no se
// recorta: se rechaza, porque un v16 mal codificado es otra instrucción. Los 11x y 21c usan un byte
// entero y sí llegan a 255.
func (p *traduce) nibble(r int, in jvmInstr) bool {
	if r < 0 || r > 15 {
		p.falla(in, "register v"+itoa(r)+" does not fit a four-bit format (the /range or /from16 form is not measured here)")
		return false
	}
	return true
}

// unidad11x: op en la mitad baja, registro en la alta. Medido con 0x030f (return v3) y 0x020c
// (move-result-object v2) de d8.
func unidad11x(op byte, a int) uint16 {
	return uint16(op) | uint16(a&0xff)<<8
}

// unidad11n: op abajo, y arriba el valor de cuatro bits junto al registro. Medido con 0x1312
// (const/4 v3, 1) y 0x0112 (const/4 v1, 0).
func unidad11n(op byte, a, b int) uint16 {
	return uint16(op) | uint16((b&0xf)<<4|a&0xf)<<8
}

// unidad12x: op abajo y dos registros de cuatro bits arriba. Medido en 0x2354 (iget-object v3, v2)
// y en 0x015b (iput-object v1, v0): el destino en el nibble bajo, la fuente en el alto.
func unidad12x(op byte, a, b int) uint16 {
	return uint16(op) | uint16((b&0xf)<<4|a&0xf)<<8
}

// indice: la segunda unidad de un 21c, de un 22c y de un 35c. Un índice fuera de u16 no se recorta:
// se rechaza, porque recortarlo es escribir un dex que apunta a otra cosa.
func (p *traduce) indice(n int, in jvmInstr, qué string) bool {
	if n < 0 || n > 0xffff {
		p.falla(in, qué+" does not fit a 16-bit index")
		return false
	}
	p.palabra(uint16(n))
	return true
}

// invoca escribe un 35c: primera unidad con el opcode, el recuento de argumentos en los bits 4..7 y
// el quinto registro en los 0..3; segunda unidad con el índice del método; tercera con los
// registros en nibbles, el primero en los bits bajos. Así están las nueve llamadas del fixture
// (§21): invoke-direct {v0}, method@16 sale 1070 0010 0000 y invoke-interface {v2, v0}, method@3
// sale 2072 0003 0002.
func (p *traduce) invoca(op byte, regs []int, metodo int, in jvmInstr) {
	n := len(regs)
	if n < 1 || n > 5 {
		p.falla(in, "a call of "+itoa(n)+" arguments: 35c goes up to five and invoke/range is not measured here")
		return
	}
	for _, r := range regs {
		if !p.nibble(r, in) {
			return
		}
	}
	g := 0
	if n == 5 {
		g = regs[4]
	}
	p.palabra(uint16(op) | uint16((n&0xf)<<4|g&0xf)<<8)
	if !p.indice(metodo, in, "the method index") {
		return
	}
	unidadRegs := uint16(regs[0] & 0xf)
	if n > 1 {
		unidadRegs |= uint16(regs[1]&0xf) << 4
	}
	if n > 2 {
		unidadRegs |= uint16(regs[2]&0xf) << 8
	}
	if n > 3 {
		unidadRegs |= uint16(regs[3]&0xf) << 12
	}
	p.palabra(unidadRegs)
}

// recorre traduce todo el Code del método. Se usa dos veces seguidas, con bases distintas.
func (p *traduce) recorre() error {
	lista, err := walkJvmCode(p.m.Code)
	if err != nil {
		return err
	}
	for k := 0; k < len(lista); k++ {
		in := lista[k]
		// mira: la siguiente instruccion JVM. Se usa para saber si el resultado de una llamada se
		// va a tirar: d8 no emite move-result-object cuando el pop lo consume, y las palabras del
		// listener del studio lo dejan ver (§21)
		seTira := k+1 < len(lista) && lista[k+1].name == "pop"
		switch in.name {
		case "iconst_0", "iconst_1":
			v := 0
			if in.name == "iconst_1" {
				v = 1
			}
			r := p.toma(in)
			if !p.nibble(r, in) {
				return p.err
			}
			p.palabra(unidad11n(opConst4, r, v))
			p.push(r)
		case "ldc":
			p.ldc(in)
		case "aload_0":
			r := p.local(0)
			if !p.nibble(r, in) {
				return p.err
			}
			p.push(r)
		case "aload_1":
			r := p.local(1)
			if !p.nibble(r, in) {
				return p.err
			}
			p.push(r)
		case "pop":
			p.suelta(p.pop(in))
		case "dup":
			if len(p.enPila) == 0 {
				p.falla(in, "dup with an empty JVM stack")
				return p.err
			}
			p.push(p.enPila[len(p.enPila)-1])
		case "return":
			p.palabra(uint16(opRetornaV))
		case "ireturn":
			r := p.pop(in)
			if !p.nibble(r, in) {
				return p.err
			}
			p.palabra(unidad11x(opRetorna, r))
		case "getfield":
			p.accesoCampo(in, false)
		case "putfield":
			p.accesoCampo(in, true)
		case "new":
			desc := p.x.classDescriptor(dexU16(in.ops))
			r := p.toma(in)
			p.palabra(uint16(opNuevaInstancia) | uint16(r&0xff)<<8)
			if !p.indice(p.t.typeIdx[desc], in, "the type_id of new") {
				return p.err
			}
			p.push(r)
		case "invokevirtual", "invokespecial", "invokestatic", "invokeinterface":
			p.invocaMetodo(in, seTira)
			if seTira && p.err == nil {
				// el pop se absorbio con la llamada: ni move-result ni descarga, como en las
				// palabras de d8 sobre el listener del studio (§21)
				k++
			}
		default:
			p.falla(in, "outside the measured subset")
		}
		if p.err != nil {
			return p.err
		}
	}
	return nil
}

func (p *traduce) ldc(in jvmInstr) {
	i := int(in.ops[0])
	r := p.toma(in)
	switch p.x.tagOf(i) {
	case cpString:
		s := p.x.stringValue(i)
		p.palabra(uint16(opConstString) | uint16(r&0xff)<<8)
		if !p.indice(p.t.stringIdx[s], in, "the string_id of ldc") {
			return
		}
	case cpInteger:
		v := int(p.x.intOf(i))
		switch {
		case v >= -8 && v <= 7:
			if !p.nibble(r, in) {
				return
			}
			p.palabra(unidad11n(opConst4, r, v))
		case v >= -32768 && v <= 32767:
			p.palabra(uint16(opConst16) | uint16(r&0xff)<<8)
			p.palabra(uint16(v))
		case v&0xffff == 0 && v>>16 >= -32768 && v>>16 <= 32767:
			p.palabra(uint16(opConstAlto16) | uint16(r&0xff)<<8)
			p.palabra(uint16(v >> 16))
		default:
			p.falla(in, "an int that fits neither const/4, const/16 nor const/high16: const (0x14) is not measured here")
			return
		}
	default:
		p.falla(in, "an ldc of a pool type that is neither string nor int")
		return
	}
	p.push(r)
}

func (p *traduce) accesoCampo(in jvmInstr, escribe bool) {
	owner, nombre, desc, _ := p.x.refOf(dexU16(in.ops))
	if desc == "" {
		p.falla(in, "a field without a descriptor")
		return
	}
	campo, ok := p.t.fieldIdx[fieldKey{owner: owner, typeOf: desc, name: nombre}]
	if !ok {
		p.falla(in, "a field that was never interned ("+owner+"."+nombre+" "+desc+")")
		return
	}
	var op byte
	switch {
	case escribe && esObjeto(desc):
		op = opIputObject
	case !escribe && esObjeto(desc):
		op = opIgetObject
	case !escribe && desc[0] == 'I':
		op = opIget
	default:
		p.falla(in, "a field access of type "+string(desc[0])+": only object fields and iget were measured")
		return
	}
	if escribe {
		// orden JVM: objeto, valor; el 22c escribe primero el valor y después el objeto, que es
		// como d8 dejó el this$0: iput-object v1, v0
		valor := p.pop(in)
		objeto := p.pop(in)
		if !p.nibble(valor, in) || !p.nibble(objeto, in) {
			return
		}
		p.palabra(unidad12x(op, valor, objeto))
	} else {
		objeto := p.pop(in)
		r := p.toma(in)
		if !p.nibble(objeto, in) || !p.nibble(r, in) {
			return
		}
		p.palabra(unidad12x(op, r, objeto))
		p.push(r)
	}
	p.indice(campo, in, "the field_id of the access")
}

func esObjeto(desc string) bool {
	return desc != "" && (desc[0] == 'L' || desc[0] == '[')
}

func (p *traduce) invocaMetodo(in jvmInstr, seTira bool) {
	owner, nombre, desc, _ := p.x.refOf(dexU16(in.ops))
	metodo, ok := p.t.methodIdx[methodKey{owner: owner, proto: protoKeyOf(desc), desc: desc, name: nombre}]
	if !ok {
		p.falla(in, "a method that was never interned ("+owner+"."+nombre+" "+desc+")")
		return
	}
	estatico := in.name == "invokestatic"
	total := countArgs(desc)
	if !estatico {
		total++
	}
	op := map[string]byte{
		"invokevirtual":   opInvocaVirtual,
		"invokeinterface": opInvocaInterfaz,
		"invokestatic":    opInvocaEstatico,
	}[in.name]
	if in.name == "invokespecial" {
		switch {
		case nombre == "<init>":
			op = opInvocaDirecto
		case owner == p.clase.Super:
			// medido: el super.onCreate del fixture sale como invoke-super (0x6f), no como
			// invoke-direct, así que el reparto de invokespecial depende del destino
			op = opInvocaSuper
		default:
			p.falla(in, "an invokespecial that is neither a constructor nor a super call: a private method, not measured here")
			return
		}
	}
	regs := make([]int, total)
	// se descargan en orden inverso: el último argumento es el primero que sale de la pila JVM, y
	// en el 35c el receptor va en el primer registro
	for i := total - 1; i >= 0; i-- {
		regs[i] = p.pop(in)
	}
	if p.err != nil {
		return
	}
	for _, r := range regs {
		p.suelta(r)
	}
	if total > p.outs {
		p.outs = total
	}
	p.invoca(op, regs, metodo, in)
	if p.err != nil {
		return
	}
	_, ret, _ := splitDescriptor(desc)
	switch {
	case ret == "V" || ret == "":
	case seTira:
		// el resultado se tira justo despues: ni move-result ni descarga, que es lo que hizo d8
		// con el MenuItem del listener (§21). Tampoco hace falta saber el tipo devuelto
	case esObjeto(ret):
		r := p.toma(in)
		p.palabra(unidad11x(opMueveResultObj, r))
		p.push(r)
	default:
		p.falla(in, "a call returning "+string(ret[0])+": move-result (0x0a) is not measured in this fork")
	}
}

// codeItemDe traduce el cuerpo de un método y devuelve el code_item con las palabras que este
// traductor emite, más lo que el §20 obliga a poner en la cabecera: ins_size son los huecos que
// toman los argumentos, outs_size la llamada más ancha y registers_size temporales más locales.
func (t *dexTables) codeItemDe(clase classFull, m classMethod) (codeItem, error) {
	if !m.HasCode {
		return codeItem{}, nil
	}
	x := poolIndex{p: clase.p}
	// la primera pasada necesita una base que no pise los locales: la pila máxima que midió javap
	// es un techo de entradas vivas, y por tanto de temporales
	prueba := &traduce{t: t, x: x, clase: clase, m: m, locales: m.LocalCount, base: m.MaxStack}
	if err := prueba.recorre(); err != nil {
		return codeItem{}, err
	}
	final := &traduce{t: t, x: x, clase: clase, m: m, locales: m.LocalCount, base: prueba.pico}
	if err := final.recorre(); err != nil {
		return codeItem{}, err
	}
	if final.pico > prueba.pico {
		return codeItem{}, &traduceError{nombre: m.Name,
			motivo: "the second pass asked for more temporaries than the first: translator bug"}
	}
	return codeItem{
		registersSize: uint32(final.base + final.locales),
		insSize:       uint32(m.ArgCount),
		outsSize:      uint32(final.outs),
		triesSize:     0,
		debugInfoOff:  0,
		insnsSize:     uint32(len(final.w)),
		insns:         final.w,
	}, nil
}
