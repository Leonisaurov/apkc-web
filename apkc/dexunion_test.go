package main

import (
	"encoding/base64"
	"math"
	"strings"
	"testing"
)

// El punto de unión de la sonda: `int r = c ? a : b; return r;` con los cinco tipos de valor, y el
// ternario con las ramas al revés. Es el defecto que destapó la investigación del 2026-11-23 y que
// antes de `dexunion.go` salía con las dos ramas devolviendo `b`: el dex pasaba `dexdump` y ART
// devolvía el valor equivocado. El `.class` lo escribió `javac 17 -g` sobre `U.java`:
//
//	public class U {
//	    static int ternario(boolean c, int a, int b) { int r = c ? a : b; return r; }
//	    static int inverso(boolean c, int a, int b) { int r = c ? b : a; return r; }
//	    static long ternarioL(boolean c, long a, long b) { long r = c ? a : b; return r; }
//	    static double ternarioD(boolean c, double a, double b) { double r = c ? a : b; return r; }
//	    static float ternarioF(boolean c, float a, float b) { float r = c ? a : b; return r; }
//	    static String ternarioS(boolean c, String a, String b) { String r = c ? a : b; return r; }
//	}
const fixtureTernario = "yv66vgAAAD0AKAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClW" +
	"BwAIAQABVQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0" +
	"aGlzAQADTFU7AQAIdGVybmFyaW8BAAYoWklJKUkBAAFjAQABWgEAAWEBAAFJAQABYgEAAXIBAA1T" +
	"dGFja01hcFRhYmxlAQAHaW52ZXJzbwEACXRlcm5hcmlvTAEABihaSkopSgEAAUoBAAl0ZXJuYXJp" +
	"b0QBAAYoWkREKUQBAAFEAQAJdGVybmFyaW9GAQAGKFpGRilGAQABRgEACXRlcm5hcmlvUwEAOSha" +
	"TGphdmEvbGFuZy9TdHJpbmc7TGphdmEvbGFuZy9TdHJpbmc7KUxqYXZhL2xhbmcvU3RyaW5nOwEA" +
	"EkxqYXZhL2xhbmcvU3RyaW5nOwcAJQEAEGphdmEvbGFuZy9TdHJpbmcBAApTb3VyY2VGaWxlAQAG" +
	"VS5qYXZhACEABwACAAAAAAAHAAEABQAGAAEACQAAAC8AAQABAAAABSq3AAGxAAAAAgAKAAAABgAB" +
	"AAAAAQALAAAADAABAAAABQAMAA0AAAAIAA4ADwABAAkAAABfAAEABAAAAAwamQAHG6cABBw+HawA" +
	"AAADAAoAAAAGAAEAAAACAAsAAAAqAAQAAAAMABAAEQAAAAAADAASABMAAQAAAAwAFAATAAIACgAC" +
	"ABUAEwADABYAAAAFAAIIQAEACAAXAA8AAQAJAAAAXwABAAQAAAAMGpkABxynAAQbPh2sAAAAAwAK" +
	"AAAABgABAAAAAwALAAAAKgAEAAAADAAQABEAAAAAAAwAEgATAAEAAAAMABQAEwACAAoAAgAVABMA" +
	"AwAWAAAABQACCEABAAgAGAAZAAEACQAAAGEAAgAHAAAADhqZAAcfpwAEITcFFgWtAAAAAwAKAAAA" +
	"BgABAAAABAALAAAAKgAEAAAADgAQABEAAAAAAA4AEgAaAAEAAAAOABQAGgADAAsAAwAVABoABQAW" +
	"AAAABQACCEAEAAgAGwAcAAEACQAAAGEAAgAHAAAADhqZAAcnpwAEKTkFGAWvAAAAAwAKAAAABgAB" +
	"AAAABQALAAAAKgAEAAAADgAQABEAAAAAAA4AEgAdAAEAAAAOABQAHQADAAsAAwAVAB0ABQAWAAAA" +
	"BQACCEADAAgAHgAfAAEACQAAAF8AAQAEAAAADBqZAAcjpwAEJEYlrgAAAAMACgAAAAYAAQAAAAYA" +
	"CwAAACoABAAAAAwAEAARAAAAAAAMABIAIAABAAAADAAUACAAAgAKAAIAFQAgAAMAFgAAAAUAAghA" +
	"AgAIACEAIgABAAkAAABhAAEABAAAAAwamQAHK6cABCxOLbAAAAADAAoAAAAGAAEAAAAHAAsAAAAq" +
	"AAQAAAAMABAAEQAAAAAADAASACMAAQAAAAwAFAAjAAIACgACABUAIwADABYAAAAHAAIIQAcAJAAB" +
	"ACYAAAACACc="

// entradaTernario es un caso de la sonda: el método, los argumentos con los que se llama y lo que
// devuelve `javac`. Los `String` viajan como identificadores, porque la máquina de abajo no tiene
// objetos y lo único que hay que comprobar es QUE referencia sale.
type entradaTernario struct {
	metodo string
	args   []any
	quiere any
}

func tablaDelTernario() (*dexTables, classFull, error) {
	b, err := base64.StdEncoding.DecodeString(fixtureTernario)
	if err != nil {
		return nil, classFull{}, err
	}
	cf, err := readClassFull(b)
	if err != nil {
		return nil, classFull{}, err
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		return nil, classFull{}, err
	}
	td.orderTables()
	return td, cf, nil
}

// TestLosPuntosDeUnionIgualanElReparto es el candado del defecto: cada rama del ternario tiene que
// dejar el valor en el MISMO registro, y el valor que sale de las palabras emitidas tiene que ser el
// que da `javac`. La falsificación es quitar la reconciliación: `ternario(true, 1, 2)` pasa a dar 2,
// `ternarioL(true, 10, 20)` 20 y `inverso(true, 7, 9)` 7, sin un solo error del dex.
func TestLosPuntosDeUnionIgualanElReparto(t *testing.T) {
	td, cf, err := tablaDelTernario()
	if err != nil {
		t.Fatalf("la sonda del ternario no se lee: %v", err)
	}
	casos := []entradaTernario{
		{"ternario", []any{true, 1, 2}, 1},
		{"ternario", []any{false, 1, 2}, 2},
		{"inverso", []any{true, 7, 9}, 9},
		{"inverso", []any{false, 7, 9}, 7},
		{"ternarioL", []any{true, int64(10), int64(20)}, int64(10)},
		{"ternarioL", []any{false, int64(10), int64(20)}, int64(20)},
		{"ternarioD", []any{true, 1.5, 2.5}, 1.5},
		{"ternarioD", []any{false, 1.5, 2.5}, 2.5},
		{"ternarioF", []any{true, float32(3.5), float32(4.5)}, float32(3.5)},
		{"ternarioF", []any{false, float32(3.5), float32(4.5)}, float32(4.5)},
		{"ternarioS", []any{true, 101, 102}, 101},
		{"ternarioS", []any{false, 101, 102}, 102},
	}
	porNombre := map[string]classMethod{}
	for _, m := range cf.Methods {
		if m.HasCode {
			porNombre[m.Name] = m
		}
	}
	vistos := 0
	for _, c := range casos {
		m, ok := porNombre[c.metodo]
		if !ok {
			t.Fatalf("%s no está en la sonda", c.metodo)
		}
		ci, err := td.codeItemDe(cf, m)
		if err != nil {
			t.Fatalf("%s: %v", c.metodo, err)
		}
		regs := bancoDeRegistros(t, m, ci, c.args)
		r, ok := ejecutaDalvik(t, ci.insns, regs, c.metodo)
		if !ok {
			t.Fatalf("%s: las palabras no retornan", c.metodo)
		}
		obtenido := valorDevuelto(m.Desc, regs, r)
		if obtenido != c.quiere {
			t.Errorf("%s%v devuelve %v y javac da %v (registro v%d, %d unidades)",
				c.metodo, c.args, obtenido, c.quiere, r, len(ci.insns))
		}
		vistos++
	}
	if vistos != len(casos) {
		t.Fatalf("%d casos de %d", vistos, len(casos))
	}
	// las palabras del `int`: el camino que cae deja el valor en su registro y el que salta pasa por
	// el bloque de fuera de línea, que lo copia y vuelve al cuerpo
	ci, err := td.codeItemDe(cf, porNombre["ternario"])
	if err != nil {
		t.Fatalf("ternario: %v", err)
	}
	quiere := []uint16{
		0x0138, 0x0003, // if-eqz v1, +3
		0x0328,         // goto +3
		0x3001, 0x000f, // move v0, v3 / return v0
		0x2301,                 // move v3, v2
		0x002a, 0xfffd, 0xffff, // goto/32 -3
	}
	if len(ci.insns) != len(quiere) {
		t.Fatalf("ternario escribe %d unidades y la medida dice %d: %v", len(ci.insns), len(quiere), ci.insns)
	}
	for i := range quiere {
		if ci.insns[i] != quiere[i] {
			t.Fatalf("ternario unidad %d: %04x contra %04x de la medida: %v", i, ci.insns[i], quiere[i], ci.insns)
		}
	}
}

// bancoDeRegistros arma los registros de la frame con los argumentos colocados donde los pone el
// formato: los `ins_size` de arriba del todo, y los anchos ocupando dos.
func bancoDeRegistros(t *testing.T, m classMethod, ci codeItem, args []any) []uint32 {
	t.Helper()
	regs := make([]uint32, ci.registersSize)
	base := int(ci.registersSize) - int(ci.insSize)
	i := 0
	pon := func(a any) {
		switch v := a.(type) {
		case bool:
			if v {
				regs[base+i] = 1
			}
		case int:
			regs[base+i] = uint32(v)
		case int64:
			regs[base+i] = uint32(uint64(v))
			regs[base+i+1] = uint32(uint64(v) >> 32)
			i++
		case float32:
			regs[base+i] = math.Float32bits(v)
		case float64:
			b := math.Float64bits(v)
			regs[base+i] = uint32(b)
			regs[base+i+1] = uint32(b >> 32)
			i++
		default:
			t.Fatalf("argumento de tipo %T en el banco", a)
		}
		i++
	}
	for _, a := range args {
		pon(a)
	}
	if base+i != int(ci.registersSize) {
		t.Fatalf("%s: los argumentos ocupan %d registros y ins_size dice %d", m.Name, i, ci.insSize)
	}
	return regs
}

// valorDevuelto lee el registro que nombró el `return*` según el tipo de vuelta del descriptor.
func valorDevuelto(desc string, regs []uint32, r int) any {
	ret := desc[strings.IndexByte(desc, ')')+1:]
	switch ret[0] {
	case 'J':
		return int64(uint64(regs[r]) | uint64(regs[r+1])<<32)
	case 'D':
		return math.Float64frombits(uint64(regs[r]) | uint64(regs[r+1])<<32)
	case 'F':
		return math.Float32frombits(regs[r])
	case 'L', '[':
		return int(regs[r])
	}
	return int(int32(regs[r]))
}

// ejecutaDalvik corre las palabras de un método sobre el banco de registros. Entiende solo los
// formatos que escribe la sonda de puntos de unión —los `move`, los dos `goto`, el `if-eqz` y los
// `return*`— y falla por nombre si aparece otro, para que el test no se vuelva un intérprete a medias
// que dice que sí a todo.
func ejecutaDalvik(t *testing.T, insns []uint16, regs []uint32, quien string) (int, bool) {
	t.Helper()
	pc := 0
	for pasos := 0; pasos < 64; pasos++ {
		if pc < 0 || pc >= len(insns) {
			t.Fatalf("%s: el pc %d se sale de las %d unidades", quien, pc, len(insns))
		}
		u := insns[pc]
		op := byte(u & 0xff)
		switch op {
		case 0x01, 0x04, 0x07: // move, move-wide, move-object (12x)
			a := int(u>>8) & 0xf
			b := int(u>>12) & 0xf
			regs[a] = regs[b]
			if op == 0x04 {
				regs[a+1] = regs[b+1]
			}
			pc++
		case 0x02, 0x05, 0x08: // las mismas en 22v
			a := int(u>>8) & 0xff
			b := int(insns[pc+1])
			regs[a] = regs[b]
			if op == 0x05 {
				regs[a+1] = regs[b+1]
			}
			pc += 2
		case 0x28: // goto 10t
			pc += int(int8(u >> 8))
		case 0x29: // goto/16
			pc += int(int16(insns[pc+1]))
		case 0x2a: // goto/32
			pc += int(int32(insns[pc+1]) | int32(insns[pc+2])<<16)
		case 0x38, 0x39: // if-eqz / if-nez
			a := int(u>>8) & 0xff
			salta := regs[a] == 0
			if op == 0x39 {
				salta = !salta
			}
			if salta {
				pc += int(int16(insns[pc+1]))
			} else {
				pc += 2
			}
		case 0x0f, 0x10, 0x11: // return, return-wide, return-object
			return int(u>>8) & 0xff, true
		default:
			t.Fatalf("%s: el opcode 0x%02x no está en el banco de la sonda", quien, op)
		}
	}
	return 0, false
}

// traduceSuelto corre el barrido sobre un cuerpo escrito a mano. Los cuerpos de estas dos pruebas no
// nombran el pool —solo enteros y saltos—, así que no hace falta ni clase ni tablas.
func traduceSuelto(t *testing.T, nombre string, pila, locales int, code []byte) *traduce {
	t.Helper()
	m := classMethod{Access: accStatic, Name: nombre, Desc: "()I", HasCode: true,
		MaxStack: pila, LocalCount: locales, ArgCount: 0, Code: code}
	p := &traduce{m: m, locales: locales, base: pila + locales + 2}
	if err := p.recorre(); err != nil {
		t.Fatalf("%s: el cuerpo no se traduce: %v", nombre, err)
	}
	return p
}

// TestElBordeDeVueltaConOtraAlturaNoAborta fija el comportamiento del cuerpo que sintetiza el
// desugrado de records para `$record$hashCode`: su `goto` de vuelta apunta al `istore_1` de la
// cabecera (pc 3) y no al `iload_1` que le sigue, así que el camino que cae llega con un valor en la
// pila y el borde de vuelta con ninguno. El plan de registros no puede cambiar una altura —no hay
// registro que inventar ni que tirar sin una instrucción—, así que esa llegada se deja como estaba y
// se anota como frontera: el método se traduce y el dex se escribe.
//
//	0: iconst_0        2: iconst_0        4: iload_1       7: if_icmpge 16
//	1: istore_0        3: istore_1        5: bipush 10    10: iinc 1, 1
//	                    ^ cabecera                          13: goto 3   <- borde de vuelta
func TestElBordeDeVueltaConOtraAlturaNoAborta(t *testing.T) {
	codigo := []byte{
		0x03, 0x3b, // iconst_0 / istore_0
		0x03, 0x3c, // iconst_0 / istore_1  (pc 3: la cabecera)
		0x1b, 0x10, 0x0a, // iload_1 / bipush 10
		0xa2, 0x00, 0x09, // if_icmpge 16
		0x84, 0x01, 0x01, // iinc 1, 1
		0xa7, 0xff, 0xf6, // goto 3
		0x1a, 0xac, // iload_0 / ireturn
	}
	p := traduceSuelto(t, "bordeConPila", 2, 2, codigo)
	if len(p.fronteras) != 1 {
		t.Fatalf("%d fronteras y la medida dice 1: %v", len(p.fronteras), p.fronteras)
	}
	f := p.fronteras[0]
	if f.pc != 13 || f.destino != 3 || f.cabecera != 1 || f.borde != 0 {
		t.Fatalf("la frontera es «%s», y la medida es pc 13 -> pc 3 con 1 valor en la cabecera y 0 en el borde",
			f.String())
	}
	if !strings.Contains(f.String(), "cannot change a stack height") {
		t.Errorf("la frontera no se nombra a sí misma: %s", f.String())
	}
	// no se inventa un bloque de reconciliación para ese borde: el único goto/32 sería el de un pad,
	// y aquí no hay ninguno
	for i, u := range p.w {
		if u&0xff == 0x2a {
			t.Fatalf("unidad %d: el borde de vuelta con otra altura sacó un goto/32 de pad: %v", i, p.w)
		}
	}
}

// TestElBordeDeVueltaQueCasaNoDejaFrontera es el control: un bucle corriente, con la cabecera a pila
// vacía en los dos caminos, se traduce sin frontera y el `goto` de vuelta apunta a la cabecera.
func TestElBordeDeVueltaQueCasaNoDejaFrontera(t *testing.T) {
	codigo := []byte{
		0x03, 0x3b, // iconst_0 / istore_0
		0x1a, 0x10, 0x0a, // iload_0 / bipush 10   (pc 2: la cabecera)
		0xa2, 0x00, 0x09, // if_icmpge 14
		0x84, 0x00, 0x01, // iinc 0, 1
		0xa7, 0xff, 0xf7, // goto 2
		0x1a, 0xac, // iload_0 / ireturn
	}
	p := traduceSuelto(t, "bordeVacio", 2, 1, codigo)
	if len(p.fronteras) != 0 {
		t.Fatalf("un bucle que casa deja %d fronteras: %v", len(p.fronteras), p.fronteras)
	}
	unidades, err := desmontaPalabras(p.w)
	if err != nil {
		t.Fatalf("las palabras no se releen: %v", err)
	}
	vuelta := false
	for _, u := range unidades {
		if strings.Contains(u, "goto/32") {
			t.Fatalf("el bucle corriente sacó un pad: %v", unidades)
		}
		if strings.HasPrefix(u, "goto") {
			vuelta = true
		}
	}
	if !vuelta {
		t.Fatalf("no hay goto de vuelta en las palabras: %v", unidades)
	}
}
