package main

// The JVM opcodes this fork knows, and the walker that counts instructions with them.
//
// The table is MEASURED, not recalled. Each entry came from the six classes of the studio
// project: for every instruction javap lists, the width is (next pc - this pc - 1) and the byte at
// that pc is the opcode of that name. The script that derives it is recorded in docs/evidencia.md
// §17, and it printed these sixteen pairs and nothing else. That matters because a width taken
// from memory is a claim nobody checked, and a wrong width does not merely miscount: it walks into
// the middle of the next instruction and reports a method that parses.
//
// Anything outside the measured table is refused by name. That includes tableswitch, lookupswitch and
// wide, whose widths are not fixed and were not needed by the subset: the day a project uses a
// switch, the entry gets added with its own measurement, not with a guess about padding.

// jvmOpcode is one measured instruction: the name javap gives it and how many operand bytes follow.
type jvmOpcode struct {
	name string
	ops  int
}

// jvmMeasured es toda la tabla. Las primeras dieciseis salieron del proyecto del studio (parrafo 17); las
// demas se midieron en el parrafo 50 sobre un fuente con campos, locales y cuentas enteras, y el ancho de
// cada una es la diferencia de desplazamientos que imprime `javap -c` (ningun numero de memoria). Son
// opcodes del JVMS: en DEX la cuenta entera vive en otra parte (0xb0..), y confundirlas fue el primer
// intento de esta tabla.
var jvmMeasured = map[byte]jvmOpcode{
	0x01: {"aconst_null", 0},
	0x02: {"iconst_m1", 0},
	0x03: {"iconst_0", 0},
	0x04: {"iconst_1", 0},
	0x05: {"iconst_2", 0},
	0x06: {"iconst_3", 0},
	0x07: {"iconst_4", 0},
	0x08: {"iconst_5", 0},
	0x10: {"bipush", 1},
	0x11: {"sipush", 2},
	0x12: {"ldc", 1},
	0x13: {"ldc_w", 2},
	0x15: {"iload", 1},
	0x19: {"aload", 1},
	0x1a: {"iload_0", 0},
	0x1b: {"iload_1", 0},
	0x1c: {"iload_2", 0},
	0x1d: {"iload_3", 0},
	0x2a: {"aload_0", 0},
	0x2b: {"aload_1", 0},
	0x2c: {"aload_2", 0},
	0x2d: {"aload_3", 0},
	0x36: {"istore", 1},
	0x3a: {"astore", 1},
	0x3b: {"istore_0", 0},
	0x3c: {"istore_1", 0},
	0x3d: {"istore_2", 0},
	0x3e: {"istore_3", 0},
	0x4b: {"astore_0", 0},
	0x4c: {"astore_1", 0},
	0x4d: {"astore_2", 0},
	0x4e: {"astore_3", 0},
	0x57: {"pop", 0},
	0x59: {"dup", 0},
	0x5a: {"dup_x1", 0},
	0x60: {"iadd", 0},
	0x64: {"isub", 0},
	0x68: {"imul", 0},
	0x6c: {"idiv", 0},
	0x70: {"irem", 0},
	// `iinc` (parrafo 67): index de local y constante de un byte con signo, medido en `Inc`
	// (`iinc 1, 1` y `iinc 1, -1`)
	0x84: {"iinc", 2},
	// arreglos (parrafo 66): el ancho sale de `javap -c` de la sonda `Arreglos`, donde
	// `newarray int` va de 1 a 3, `anewarray #7` de 1 a 4 y `arraylength` de 1 a 2
	0x2e: {"iaload", 0},
	0x32: {"aaload", 0},
	0x4f: {"iastore", 0},
	0x53: {"aastore", 0},
	0xbc: {"newarray", 1},
	0xbd: {"anewarray", 2},
	0xbe: {"arraylength", 0},
	0xac: {"ireturn", 0},
	0xb0: {"areturn", 0}, // visto en la sonda del parrafo 53: `2: areturn` con el siguiente
	// desplazamiento en 3, o sea un byte y ningun operando. El traductor ya lo escribia desde el
	// parrafo 50, pero la tabla del caminante no lo tenia y el dexer se paraba en el 0xb0.
	0xbf: {"athrow", 0}, // medido en el parrafo 71: `13: athrow` con el siguiente desplazamiento en 14
	0xb1: {"return", 0},
	0xb2: {"getstatic", 2},
	0xb3: {"putstatic", 2},
	0xb4: {"getfield", 2},
	0xb5: {"putfield", 2},
	0xb6: {"invokevirtual", 2},
	0xb7: {"invokespecial", 2},
	0xb8: {"invokestatic", 2},
	0xb9: {"invokeinterface", 4}, // index u16, then count u8 and a reserved u8
	0xbb: {"new", 2},
	0xc0: {"checkcast", 2},
	// las branquias, con el ancho de dos bytes que mide `javap -c` en el parrafo 52: el numero que
	// imprime detras del mnemonic es el desplazamiento, y el siguiente desplazamiento de la lista
	// esta tres bytes mas adelante que el de la branquia
	0x99: {"ifeq", 2},
	0x9a: {"ifne", 2},
	0x9b: {"iflt", 2},
	0x9c: {"ifge", 2},
	0x9d: {"ifgt", 2},
	0x9e: {"ifle", 2},
	0x9f: {"if_icmpeq", 2},
	0xa0: {"if_icmpne", 2},
	0xa1: {"if_icmplt", 2},
	0xa2: {"if_icmpge", 2},
	0xa3: {"if_icmpgt", 2},
	0xa4: {"if_icmple", 2},
	0xa7: {"goto", 2},
	0xc6: {"ifnull", 2},
	0xc7: {"ifnonnull", 2},
}

// jvmInstr is one instruction of a Code array: where it starts, what it is, its operand bytes.
type jvmInstr struct {
	pc   int
	op   byte
	name string
	ops  []byte
	// relleno: los bytes de alineación que van entre el opcode y los operandos en `tableswitch` y
	// `lookupswitch`, contados desde el principio del Code (medido en el parrafo 64: la tabla de la
	// sonda está en el pc 1 y sus operandos empiezan en el 4, o sea tres bytes de relleno).
	relleno int
}

var jvmSwitch = map[byte]string{0xaa: "tableswitch", 0xab: "lookupswitch"}

// anchoDeSwitch: cuántos bytes de operandos tiene un switch empezando en `base` (ya sin el relleno).
func anchoDeSwitch(op byte, code []byte, base int) (int, error) {
	if op == 0xaa {
		if base+12 > len(code) {
			return 0, &jvmWalkError{pc: base, op: op, what: "the switch header runs past the end of the code"}
		}
		alto := int(int32(uint32(code[base+8])<<24 | uint32(code[base+9])<<16 |
			uint32(code[base+10])<<8 | uint32(code[base+11])))
		bajo := int(int32(uint32(code[base+4])<<24 | uint32(code[base+5])<<16 |
			uint32(code[base+6])<<8 | uint32(code[base+7])))
		if alto < bajo-1 || alto-bajo > 1<<16 {
			return 0, &jvmWalkError{pc: base, op: op, what: "a tableswitch with a key range that makes no sense"}
		}
		return 12 + 4*(alto-bajo+1), nil
	}
	if base+8 > len(code) {
		return 0, &jvmWalkError{pc: base, op: op, what: "the switch header runs past the end of the code"}
	}
	n := int(int32(uint32(code[base+4])<<24 | uint32(code[base+5])<<16 |
		uint32(code[base+6])<<8 | uint32(code[base+7])))
	if n < 0 || n > 1<<16 {
		return 0, &jvmWalkError{pc: base, op: op, what: "a lookupswitch with a pair count that makes no sense"}
	}
	return 8 + 8*n, nil
}

// destinoDe: el pc al que salta una branquia JVM. El desplazamiento va en bytes desde el principio
// de la instruccion —desde su opcode, no desde sus operandos, que es como lo escribio el emisor y
// como lo lee javap: `2: if_icmpne 7` lleva por operandos `00 05`—, y son un entero con signo.
func destinoDe(in jvmInstr) int {
	return in.pc + int(int16(uint16(in.ops[0])<<8|uint16(in.ops[1])))
}

// walkJvmCode reads a Code array as far as the measured table allows and stops at the first byte
// it does not know, naming it. An incomplete walk is an error, never a count: a method that
// half-parsed is exactly the kind of almost-right that reaches a device.
func walkJvmCode(code []byte) ([]jvmInstr, error) {
	out := []jvmInstr{}
	for i := 0; i < len(code); {
		op := code[i]
		if nombre, esSwitch := jvmSwitch[op]; esSwitch {
			relleno := (4 - (i+1)%4) % 4
			base := i + 1 + relleno
			ancho, err := anchoDeSwitch(op, code, base)
			if err != nil {
				return nil, err
			}
			if base+ancho > len(code) {
				return nil, &jvmWalkError{pc: i, op: op, what: "the switch runs past the end of the code"}
			}
			out = append(out, jvmInstr{pc: i, op: op, name: nombre, ops: code[base : base+ancho], relleno: relleno})
			i = base + ancho
			continue
		}
		k, ok := jvmMeasured[op]
		if !ok {
			return nil, &jvmWalkError{pc: i, op: op,
				what: "opcode outside the table this fork measured"}
		}
		instr := jvmInstr{pc: i, op: op, name: k.name}
		i++
		if i+k.ops > len(code) {
			return nil, &jvmWalkError{pc: i, op: op, what: "its operand runs past the end of the code"}
		}
		instr.ops = code[i : i+k.ops]
		i += k.ops
		out = append(out, instr)
	}
	return out, nil
}

// countJvmInstructions answers what javap lists, so the two can be compared method by method.
func countJvmInstructions(code []byte) (int, error) {
	list, err := walkJvmCode(code)
	if err != nil {
		return 0, err
	}
	return len(list), nil
}

type jvmWalkError struct {
	pc   int
	op   byte
	what string
}

func (e *jvmWalkError) Error() string {
	return "at pc " + itoa(e.pc) + ", byte " + hexByte(e.op) + ": " + e.what
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func hexByte(b byte) string {
	const dig = "0123456789abcdef"
	return "0x" + string([]byte{dig[b>>4], dig[b&0x0f]})
}
