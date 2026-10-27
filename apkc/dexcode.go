package main

// Reading what lives inside a dex: the class_data_item of each class and the code_item of
// each method.
//
// The index tables say *what* exists; these say what belongs to whom, and what the machine
// runs. They are the other half of the comparison the writer needs: when our classes.dex and
// d8's differ, the difference is either in a table (dextabla.go, checked already) or in a
// code_item's words, and only the second kind can hide a translation error that installs.
//
// The encodings here are the delta-coded ones of the format: an encoded_field and an
// encoded_method name their item by the difference from the previous one, and a code_item is
// 16 bytes of header followed by insns_size 16-bit words, with the handler table behind the
// code and padded back to four bytes.

import (
	"encoding/binary"
	"fmt"
)

// Los tipos de un encoded_value, con los valores del formato: VALUE_NULL es 0x1e, no 0x02 (que es
// VALUE_SHORT). Importa tenerlos bien porque el ancho del resto del valor depende del tipo.
const (
	valueByte       = 0x00
	valueShort      = 0x02
	valueChar       = 0x03
	valueInt        = 0x04
	valueLong       = 0x06
	valueFloat      = 0x10
	valueDouble     = 0x11
	valueMethodType = 0x15
	valueMethodHand = 0x16
	valueString     = 0x17
	valueType       = 0x18
	valueField      = 0x19
	valueMethod     = 0x1a
	valueEnum       = 0x1b
	valueArray      = 0x1c
	valueAnnotation = 0x1d
	valueNull       = 0x1e
	valueBool       = 0x1f
)

// anchoValor: cuantos bytes llevan detras del byte de tipo. Los tipos cortos codifican su tamano
// en los bits 5..7 como "size - 1"; VALUE_NULL y VALUE_BOOLEAN no llevan ninguno. Lo que seria un
// indice o una estructura variable devuelve -1: se rechaza, no se adivina.
func anchoValor(vf byte) int {
	size := int(vf>>5&7) + 1
	switch vf & 0x1f {
	case valueByte:
		return 1
	case valueShort, valueChar, valueInt, valueLong, valueFloat, valueDouble,
		valueMethodType, valueMethodHand:
		return size
	case valueNull, valueBool:
		return 0
	}
	return -1
}

// codeItem is a code_item: sixteen header bytes and the code words behind them. Every field here
// has a name because it was MEASURED on d8's file against `javap -v` on the same classes
// (docs/evidencia.md §20), not because a header file says so:
//
//	+0  u2  registers_size  never below ins_size nor below outs_size, and above javap's locals
//	                          exactly when the method needs temps
//	+2  u2  ins_size        equals javap's args_size in all nine methods of the fixture
//	+4  u2  outs_size       the widest call site; never above javap's max_stack, which has to
//	                          hold the same arguments on the operand stack
//	+6  u2  tries_size      zero in the nine, none of which carries an exception table
//	+8  u32 debug_info_off  always inside the debug_info_item area the map_list reports (1160..1192)
//	+12 u32 insns_size      16 + 2·this is the item, and it reaches the next one up to padding
//
// The twelve-byte reading that some text gives (size at +8, code at +12) contradicts the file: it
// would call the debug offsets a code length and start the instructions two words too early, where
// the constructors plainly do not begin with 0x70.
type codeItem struct {
	registersSize uint32 // +0
	insSize       uint32 // +2, el args_size de javap
	outsSize      uint32 // +4
	triesSize     uint32 // +6, cero sin tabla de excepciones
	debugInfoOff  uint32 // +8, cae dentro del area debug_info_item del mapa
	insnsSize     uint32 // +12
	insns         []uint16
}

// tamanoEnBytes: la cabecera mas sus palabras. Sin tries por ahora, que es el caso de los nueve
// metodos del fixture; un try anyadido cambia la cuenta y se medira cuando haga falta.
func (c codeItem) tamanoEnBytes() int {
	return 16 + 2*int(c.insnsSize)
}

// readCodeItem reads the item at off, words at +16 and the size as the u32 it is (+12, whose high
// half is zero in every method measured).
func (d *dexFile) readCodeItem(off uint32) (codeItem, error) {
	if off == 0 {
		return codeItem{}, nil
	}
	if int(off)+16 > len(d.raw) {
		return codeItem{}, fmt.Errorf("the code_item at %d does not fit a %d byte file", off, len(d.raw))
	}
	var c codeItem
	c.registersSize = uint32(binary.LittleEndian.Uint16(d.raw[off:]))
	c.insSize = uint32(binary.LittleEndian.Uint16(d.raw[off+2:]))
	c.outsSize = uint32(binary.LittleEndian.Uint16(d.raw[off+4:]))
	c.triesSize = uint32(binary.LittleEndian.Uint16(d.raw[off+6:]))
	c.debugInfoOff = dexLittleEndian.Uint32(d.raw[off+8:])
	c.insnsSize = dexLittleEndian.Uint32(d.raw[off+12:])
	if uint64(off)+16+uint64(c.insnsSize)*2 > uint64(len(d.raw)) {
		return c, fmt.Errorf("the code_item at %d claims %d words, past the end of the file", off, c.insnsSize)
	}
	c.insns = make([]uint16, c.insnsSize)
	for i := uint32(0); i < c.insnsSize; i++ {
		c.insns[i] = binary.LittleEndian.Uint16(d.raw[off+16+i*2:])
	}
	return c, nil
}

// classData is what a class declares about itself: which fields and methods belong to it, named by
// the delta encodings of the format.
type classData struct {
	staticFields   []encodedField
	instanceFields []encodedField
	directs        []encodedMethod
	virtuals       []encodedMethod
}

type encodedField struct {
	fieldIdx    uint32
	accessFlags uint32
	// la constante de un campo estatico no vive aqui: viaja en el encoded_array_item que anuncia
	// class_defs.static_values_off (medido en §22). El par (valueType, value) lo llena
	// valoresEstaticos, no el recorrido del class_data.
	valueType byte
	value     uint64
}

type encodedMethod struct {
	methodIdx   uint32
	accessFlags uint32
	codeOff     uint32
}

// readClassData reads the class_data_item at off. The delta coding is why this keeps a running
// previous index: reading one item without the ones before it is not possible.
func (d *dexFile) readClassData(off uint32) (classData, error) {
	var cd classData
	if off == 0 {
		return cd, nil
	}
	at := off
	n, at, err := d.uleb(at)
	if err != nil {
		return cd, fmt.Errorf("static field count: %w", err)
	}
	statics := n
	if n, at, err = d.uleb(at); err != nil {
		return cd, fmt.Errorf("instance field count: %w", err)
	}
	instances := n
	if n, at, err = d.uleb(at); err != nil {
		return cd, fmt.Errorf("direct method count: %w", err)
	}
	directs := n
	if n, at, err = d.uleb(at); err != nil {
		return cd, fmt.Errorf("virtual method count: %w", err)
	}
	virtuals := n

	prev := uint32(0)
	for i := uint32(0); i < statics; i++ {
		f, next, err := d.readEncodedField(at, prev)
		if err != nil {
			return cd, fmt.Errorf("static field #%d: %w", i, err)
		}
		prev = f.fieldIdx
		at = next
		cd.staticFields = append(cd.staticFields, f)
	}
	prev = 0
	for i := uint32(0); i < instances; i++ {
		f, next, err := d.readEncodedField(at, prev)
		if err != nil {
			return cd, fmt.Errorf("instance field #%d: %w", i, err)
		}
		prev = f.fieldIdx
		at = next
		cd.instanceFields = append(cd.instanceFields, f)
	}
	prev = 0
	for i := uint32(0); i < directs; i++ {
		m, next, err := d.readEncodedMethod(at, prev)
		if err != nil {
			return cd, fmt.Errorf("direct method #%d: %w", i, err)
		}
		prev = m.methodIdx
		at = next
		cd.directs = append(cd.directs, m)
	}
	prev = 0
	for i := uint32(0); i < virtuals; i++ {
		m, next, err := d.readEncodedMethod(at, prev)
		if err != nil {
			return cd, fmt.Errorf("virtual method #%d: %w", i, err)
		}
		prev = m.methodIdx
		at = next
		cd.virtuals = append(cd.virtuals, m)
	}
	return cd, nil
}

func (d *dexFile) readEncodedField(at, prev uint32) (encodedField, uint32, error) {
	var f encodedField
	diff, next, err := d.uleb(at)
	if err != nil {
		return f, at, err
	}
	at = next
	if f.accessFlags, at, err = d.uleb(at); err != nil {
		return f, at, err
	}
	f.fieldIdx = prev + diff
	// El encoded_field NO lleva el valor en este dex. Medido sobre el fichero de d8 (§22): el campo
	// estatico de R$id son los dos bytes `01 19` (diff y access_flags) y la constante esta en el
	// encoded_array_item que anuncia class_defs.static_values_off. La lectura que plegaba el byte
	// del valor en los bits 16 de access_flags era un artefacto de como escribiamos nosotros, y
	// hacia crecer access_flags cinco ordenes de magnitud.
	return f, at, nil
}

func (d *dexFile) readEncodedMethod(at, prev uint32) (encodedMethod, uint32, error) {
	var m encodedMethod
	diff, next, err := d.uleb(at)
	if err != nil {
		return m, at, err
	}
	at = next
	if m.accessFlags, at, err = d.uleb(at); err != nil {
		return m, at, err
	}
	m.methodIdx = prev + diff
	if m.codeOff, at, err = d.uleb(at); err != nil {
		return m, at, err
	}
	return m, at, nil
}

// methodsDeCadaClase ata cada class_data_item con la clase que lo declara, que es lo que un
// diff necesita: "a MainActivity le falta un virtual" no se puede decir sin esa relacion.
func (d *dexFile) methodsDeCadaClase() (map[string]classData, error) {
	out := map[string]classData{}
	for _, c := range d.classes {
		cd, err := d.readClassData(c.dataOff)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.name, err)
		}
		if c.staticValuesOff != 0 {
			valores, err := d.valoresEstaticos(c.staticValuesOff)
			if err != nil {
				return nil, fmt.Errorf("%s: static values: %w", c.name, err)
			}
			// el encoded_array_item va en el orden de la lista de campos estaticos; si la cuenta
			// no cuadra, el fichero esta roto y se dice, no se rellena con ceros
			if len(valores) != len(cd.staticFields) {
				return nil, fmt.Errorf("%s: %d static values for %d static fields",
					c.name, len(valores), len(cd.staticFields))
			}
			for i := range cd.staticFields {
				cd.staticFields[i].valueType = valores[i].valueType
				cd.staticFields[i].value = valores[i].value
			}
		}
		out[c.name] = cd
	}
	return out, nil
}

// valoresEstaticos lee un encoded_array_item. El tamano esta medido como uleb128, no como u32:
// las tres areas de R empiezan en 2465, 2471 y 2477, seis bytes cada una, y un `uint size` de
// cuatro mas el VALUE_INT de cuatro ocuparian nueve. Con el u32 el primer item diria que tiene
// 25601 valores, que es la senal de que la lectura es otra (§22).
func (d *dexFile) valoresEstaticos(off uint32) ([]encodedField, error) {
	n, at, err := d.uleb(off)
	if err != nil {
		return nil, fmt.Errorf("array size: %w", err)
	}
	out := []encodedField{}
	for i := uint32(0); i < n; i++ {
		if int(at) >= len(d.raw) {
			return nil, fmt.Errorf("value #%d runs past the end of the file", i)
		}
		vf := d.raw[at]
		at++
		f := encodedField{valueType: vf & 0x1f}
		ancho := anchoValor(vf)
		if ancho < 0 {
			return nil, fmt.Errorf("value #%d of type %#x is not read by this fork yet", i, f.valueType)
		}
		if int(at)+ancho > len(d.raw) {
			return nil, fmt.Errorf("value #%d claims %d bytes past the end of the file", i, ancho)
		}
		for k := 0; k < ancho; k++ {
			f.value |= uint64(d.raw[at]) << (8 * k) // little endian, truncado como el formato
			at++
		}
		out = append(out, f)
	}
	return out, nil
}

// desmontaPalabras convierte las palabras de un code_item en instrucciones legibles. Nace de la
// misma medicion que el traductor (docs/evidencia.md §21): solo conoce los quince formatos que
// d8 escribio sobre el proyecto del studio, y un opcode fuera de esa lista es un error con nombre,
// no un misterio. Es lo que permite decir "nuestras palabras y las de d8 son las mismas
// instrucciones sobre los mismos ids" sin comparar registros, que es donde un asignador distinto
// separa (documentado en el plan 2G.4).
func desmontaPalabras(palabras []uint16) ([]string, error) {
	out := []string{}
	for i := 0; i < len(palabras); {
		u := palabras[i]
		op := byte(u)
		alta := uint8(u >> 8)
		nombre, formato, ok := formatoDalvik(op)
		if !ok {
			return nil, fmt.Errorf("at word %d: opcode %#02x is outside the fifteen formats measured on d8's file", i, op)
		}
		switch formato {
		case '0': // 10x: sin operandos
			out = append(out, nombre)
			i++
		case 'x': // 11x: vAA
			out = append(out, fmt.Sprintf("%s v%d", nombre, alta))
			i++
		case 'n': // 11n: vAA, #+B (el registro en el nibble bajo, el valor con signo en el alto)
			out = append(out, fmt.Sprintf("%s v%d, %d", nombre, alta&0xf, int(int8(alta)>>4)))
			i++
		case '2': // 12x: vA, vB
			out = append(out, fmt.Sprintf("%s v%d, v%d", nombre, alta&0xf, alta>>4))
			i++
		case 'c': // 21c: vAA, indice
			if i+1 >= len(palabras) {
				return nil, fmt.Errorf("at word %d: %s wants its index word and the item ends", i, nombre)
			}
			out = append(out, fmt.Sprintf("%s v%d, %d", nombre, alta, palabras[i+1]))
			i += 2
		case 'f': // 22c: vA, vB, indice
			if i+1 >= len(palabras) {
				return nil, fmt.Errorf("at word %d: %s wants its index word and the item ends", i, nombre)
			}
			out = append(out, fmt.Sprintf("%s v%d, v%d, %d", nombre, alta&0xf, alta>>4, palabras[i+1]))
			i += 2
		case 'i': // 35c: conteo y registros repartidos, con el indice en medio
			if i+2 >= len(palabras) {
				return nil, fmt.Errorf("at word %d: %s wants three words and the item ends", i, nombre)
			}
			n := int(alta >> 4)
			regs := []int{}
			u2 := palabras[i+2]
			for k := 0; k < n && k < 4; k++ {
				regs = append(regs, int((u2>>(4*k))&0xf))
			}
			if n == 5 {
				regs = append(regs, int(alta&0xf))
			}
			texto := ""
			for k, r := range regs {
				if k > 0 {
					texto += ", "
				}
				texto += fmt.Sprintf("v%d", r)
			}
			out = append(out, fmt.Sprintf("%s {%s}, %d", nombre, texto, palabras[i+1]))
			i += 3
		}
	}
	return out, nil
}

// formatoDalvik da el nombre y la forma de los quince opcodes medidos en el fichero de d8. El
// nombre es el que usa la herramienta, para que un diff se lea sin tabla al lado.
func formatoDalvik(op byte) (string, byte, bool) {
	switch op {
	case 0x0e:
		return "return-void", '0', true
	case 0x0c:
		return "move-result-object", 'x', true
	case 0x0f:
		return "return", 'x', true
	case 0x12:
		return "const/4", 'n', true
	case 0x13:
		return "const/16", 'c', true
	case 0x15:
		return "const/high16", 'c', true
	case 0x1a:
		return "const-string", 'c', true
	case 0x22:
		return "new-instance", 'c', true
	case 0x52:
		return "iget", 'f', true
	case 0x54:
		return "iget-object", 'f', true
	case 0x5b:
		return "iput-object", 'f', true
	case 0x6e:
		return "invoke-virtual", 'i', true
	case 0x6f:
		return "invoke-super", 'i', true
	case 0x70:
		return "invoke-direct", 'i', true
	case 0x71:
		return "invoke-static", 'i', true
	case 0x72:
		return "invoke-interface", 'i', true
	}
	return "", 0, false
}
