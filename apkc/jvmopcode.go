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
// Anything outside the sixteen is refused by name. That includes tableswitch, lookupswitch and
// wide, whose widths are not fixed and were not needed by the subset: the day a project uses a
// switch, the entry gets added with its own measurement, not with a guess about padding.

// jvmOpcode is one measured instruction: the name javap gives it and how many operand bytes follow.
type jvmOpcode struct {
	name string
	ops  int
}

// jvmMeasured is the whole table. Sixteen entries, in the order the measurement printed them.
var jvmMeasured = map[byte]jvmOpcode{
	0x03: {"iconst_0", 0},
	0x04: {"iconst_1", 0},
	0x12: {"ldc", 1},
	0x2a: {"aload_0", 0},
	0x2b: {"aload_1", 0},
	0x57: {"pop", 0},
	0x59: {"dup", 0},
	0xac: {"ireturn", 0},
	0xb1: {"return", 0},
	0xb4: {"getfield", 2},
	0xb5: {"putfield", 2},
	0xb6: {"invokevirtual", 2},
	0xb7: {"invokespecial", 2},
	0xb8: {"invokestatic", 2},
	0xb9: {"invokeinterface", 4}, // index u16, then count u8 and a reserved u8
	0xbb: {"new", 2},
}

// jvmInstr is one instruction of a Code array: where it starts, what it is, its operand bytes.
type jvmInstr struct {
	pc   int
	op   byte
	name string
	ops  []byte
}

// walkJvmCode reads a Code array as far as the measured table allows and stops at the first byte
// it does not know, naming it. An incomplete walk is an error, never a count: a method that
// half-parsed is exactly the kind of almost-right that reaches a device.
func walkJvmCode(code []byte) ([]jvmInstr, error) {
	out := []jvmInstr{}
	for i := 0; i < len(code); {
		op := code[i]
		k, ok := jvmMeasured[op]
		if !ok {
			return nil, &jvmWalkError{pc: i, op: op,
				what: "opcode outside the sixteen this fork measured"}
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
