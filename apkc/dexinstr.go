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

import (
	"math"
	"strings"
)

// Los opcodes dalvik que este traductor sabe escribir, con el número que tienen en las palabras de
// d8. Un formato 11x pone el registro en la mitad alta de la primera unidad; un 12x parte esa mitad
// en dos nibbles de cuatro bits; un 21c lleva el índice en la segunda unidad; un 22c hace lo mismo
// con dos registros en nibbles; un 35c cuenta los argumentos y reparte registros entre dos
// unidades. La lista es corta a propósito: lo que falta está dicho en los rechazos.
const (
	opRetorna     = 0x0f // 11x return vAA
	opRetornaV    = 0x0e // 10x return-void
	opMueveResult = 0x0a // 11x move-result vAA, medido en el parrafo 66 (`0a00` tras un
	// invoke-static que devuelve I)
	opMueveResultObj = 0x0c // 11x move-result-object vAA

	// los moves de registro, medidos en el parrafo 71: `move v0, v1` sale `0110` y
	// `move-object v0, v3` sale `0730` (12x, destino en el nibble bajo y fuente en el alto)
	opMueve       = 0x01 // 12x move vA, vB
	opMueveObjeto = 0x07 // 12x move-object vA, vB

	// las mismas tres copias en la forma 22v —destino de OCHO bits y fuente de DIECISÉIS—, medida sobre
	// el fichero que escribe `d8 --debug` (parrafo 86): `0202 1200` es `move/from16 v2, v18`,
	// `0501 1300` `move-wide/from16 v1, v19` y `0801 1300` `move-object/from16 v1, v19`. Es lo que hace
	// `d8` cuando una instrucción de cuatro bits tiene que leer un registro alto: lo baja.
	opMueveFrom16       = 0x02 // 22v move/from16 vAA, vBBBB
	opMueveWideFrom16   = 0x05 // 22v move-wide/from16 vAA, vBBBB
	opMueveObjetoFrom16 = 0x08 // 22v move-object/from16 vAA, vBBBB

	// el `long` en el dex, medido con `dexdump -d` sobre `d8 --release` en las sondas `L`, `V` y `M`
	// (parrafo 74): dos registros por valor, el de en medio de cada trío de acceso a campo
	opMueveWide       = 0x04 // 12x move-wide vA, vB
	opMueveResultWide = 0x0b // 11x move-result-wide vAA, `0b00`
	opRetornaWide     = 0x10 // 11x return-wide vAA, `1000`
	opConstWide16     = 0x16 // 21c const-wide/16 vAA, #+BBBB (`1600 0100`)
	opConstWide32     = 0x17 // const-wide/32 vAA, #+BBBBBBBB (`1700 7011 0100`)
	opConstWide       = 0x18 // const-wide vAA, #+BBBBBBBBBBBBBBBB, cuatro unidades (`1800 cb04 fb71 1f01`)
	opIGetWide        = 0x53 // 22c iget-wide vAA, vBB, field@CCCC (`5320 0000`)
	opIPutWide        = 0x5a // 22c iput-wide vAA, vBB, field@CCCC (`5a01 0000`)
	opSGetWide        = 0x61 // 21c sget-wide vAA, field@BBBB (`6102 0000`)
	opSPutWide        = 0x68 // 21c sput-wide vAA, field@BBBB (`6802 0000`)
	opIntALargo       = 0x81 // 12x int-to-long vA, vB (`8120`)
	opLargoAInt       = 0x84 // 12x long-to-int v1, v0 (`8401`)
	opConst4          = 0x12 // 11n const/4 vAA, #+B
	opConst16         = 0x13 // 21c const/16 vAA, #+BBBB
	opConstAlto16     = 0x15 // 21c const/high16 vAA, #+BBBB0000
	opConst           = 0x14 // 31u const vAA, #+BBBBBBBB —asi sale `1.0f` (`1400 0000 803f`, parrafo 80)
	opConstWideAlto16 = 0x19 // 21c const-wide/high16 vAA, #+BBBB000000000000 (`1902 f03f` = 1.0, parrafo 80)
	opConstString     = 0x1a // 21c const-string vAA, string@BBBB
	opNuevaInstancia  = 0x22 // 21c new-instance vAA, type@BBBB
	opIget            = 0x52 // 22c iget vAA, vBB, field@CCCC
	opIgetObject      = 0x54 // 22c iget-object vAA, vBB, field@CCCC
	opIputObject      = 0x5b // 22c iput-object vAA, vBB, field@CCCC
	opInvocaVirtual   = 0x6e // 35c invoke-virtual
	opInvocaSuper     = 0x6f // 35c invoke-super
	opInvocaDirecto   = 0x70 // 35c invoke-direct
	opInvocaEstatico  = 0x71 // 35c invoke-static
	opInvocaInterfaz  = 0x72 // 35c invoke-interface

	// leidos de `d8` sobre la sonda del parrafo 50: `sget v0, field@0` sale 0x60 y su indice,
	// `sput` 0x67, `check-cast v1, type@1` 0x1f y su indice, `return-object v1` 0x11. Las dos de
	// referencia se midieron el 2026-11-01 sobre `System.out.println` (parrafo 62):
	// `sget-object v0, field@0` es `6200 0000` y `sput-object v1, field@1` es `6901 0100`, o sea la
	// misma forma 21c y el mismo hueco de dos opcodes entre el entero y el de objeto (wide en medio).
	opSget       = 0x60 // 21c sget vAA, field@BBBB
	opSput       = 0x67 // 21c sput vAA, field@BBBB
	opSgetObject = 0x62 // 21c sget-object vAA, field@BBBB
	opSputObject = 0x69 // 21c sput-object vAA, field@BBBB

	// Los dos switch, medidos en el parrafo 64: `d8` escribe `packed-switch v0, +0x30` (0x2b) con el
	// payload `0100` + tamaño + first_key + destinos detrás de todos los cuerpos, y en `--debug`
	// `sparse-switch v0, +0x20` (0x2c) con `0200` + tamaño + pares (clave, destino) ascendentes. En los
	// dos, si ninguna clave casa se sigue por la instrucción siguiente, así que el defecto se escribe
	// como un `goto` de una unidad detrás del switch.
	opCambioEmpaquetado = 0x2b // 31t packed-switch vAA, +BBBBBBBB
	opCambioDisperso    = 0x2c // 31t sparse-switch vAA, +BBBBBBBB
	identEmpaquetado    = 0x0100
	identDisperso       = 0x0200
	opCheckcastDex      = 0x1f // 21c check-cast vAA, type@BBBB
	opInstanceDex       = 0x20 // 22c instance-of vAA, vBB, type@CCCC (`2010 0002` en `Ins2.f`, parrafo 79)
	opRetornaObjeto     = 0x11 // 11x return-object vAA
	opIput              = 0x59 // 22c iput vAA, vBB, field@CCCC (visto en el <init> de la sonda)

	// las comparaciones anchas, medidas en el párrafo 81 sobre dos sondas de `d8 --debug` leídas con
	// `readDex`: bloque contiguo 0x2d..0x31, formato 23x (`op|AA` y después `BB|CC<<8`), con el
	// destino en un registro suelto y las dos parejas de operandos detrás. El 0x32 (`if-eq`) estaba
	// medido desde el párrafo 52 y es con el que se consume el entero que dejan.
	opFcmplDex = 0x2d // 23x fcmpl vAA, vBB, vCC
	opFcmpgDex = 0x2e // 23x fcmpg
	opDcmplDex = 0x2f // 23x dcmpl
	opDcmpgDex = 0x30 // 23x dcmpg
	opLcmpDex  = 0x31 // 23x lcmp

	// los arreglos, medidos en el parrafo 66 sobre `Arreglos` dexado por `d8` y desarmado con
	// `dexdump -d` del aparato: `array-length v0, v0` sale `2100` (12x, destino en el nibble bajo y
	// fuente en el alto), `new-array v0, v0, [I` sale `2300 0500` (22c: registros en el byte alto y el
	// type_id detrás), `aget v2, v3, v1` sale `4402 0301` (23x: op | AA, después BB | CC<<8), y las
	// variantes de objeto están en el mismo hueco: `aget-object` 0x46, `aput` 0x4b, `aput-object` 0x4d.
	opLongitudArreglo = 0x21 // 12x array-length vA, vB
	opNuevoArreglo    = 0x23 // 22c new-array vA, vB, type@CCCC
	opAget            = 0x44 // 23x aget vAA, vBB, vCC
	opAgetAncho       = 0x45 // 23x aget-wide, medido en `Arr.leeLargo` (parrafo 84)
	opAgetObjeto      = 0x46 // 23x aget-object vAA, vBB, vCC
	opAput            = 0x4b // 23x aput vAA, vBB, vCC
	opAputAncho       = 0x4c // 23x aput-wide, medido en `Arr.escribeLargo` (parrafo 84)
	opAputObjeto      = 0x4d // 23x aput-object vAA, vBB, vCC

	// las excepciones, medidas en el parrafo 71 sobre `T` dexado por `d8`: `throw v1` sale `2701`
	// (11x) y el handler empieza con `move-exception vAA` (0x0d, 11x) cuando la excepcion se usa
	opMueveExcepcion = 0x0d // 11x move-exception vAA
	opLanza          = 0x27 // 11x throw vAA

	// la familia de cuentas enteras sobre dos registros, en el orden ad-sub-mul-div-rem que la
	// sonda verifico en cuatro de sus miembros (add 0xb0, sub 0xb1, div 0xb3, y el shl 0xb8 de la
	// misma tabla contigua) y que la otra familia, la de literal, reproduce en el mismo orden
	// (add-int/lit8 0xd8, mul 0xda, div 0xdb, rem 0xdc)
	opSumaInt2    = 0xb0 // 12x add-int/2addr
	opRestaInt2   = 0xb1 // 12x sub-int/2addr
	opMultiplica2 = 0xb2 // 12x mul-int/2addr
	opDivide2     = 0xb3 // 12x div-int/2addr
	opResiduo2    = 0xb4 // 12x rem-int/2addr

	// las branquias, leidas de las palabras de `d8` sobre la sonda del parrafo 52: `if (a == b)`
	// escribe 0x2133 = if-ne v1, v2 y su desplazamiento en la unidad siguiente; `ifne` sobre un
	// entero sale if-nez 0x39; `ifnull` sale if-eqz 0x38 (el nulo es cero para dalvik) y `goto`
	// 0x28 con el desplazamiento de ocho bits en la mitad alta de su propia unidad.
	opSalto = 0x28 // 10t goto +AAAA
	// familia de dos registros (22c con desplazamiento) y de uno (21t), en el mismo orden que la
	// de enteros del JVMS: eq, ne, lt, ge, gt, le
	opIfEq, opIfNe, opIfLt, opIfGe, opIfGt, opIfLe       = 0x32, 0x33, 0x34, 0x35, 0x36, 0x37
	opIfEqz, opIfNez, opIfLtz, opIfGez, opIfGtz, opIfLez = 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d
)

// las dos tablas de branquias. La inversion de la condicion ya la hizo javac al escribir el .class
// (el emisor la midio en el parrafo 52), asi que aqui solo se cambia de mnemonic: `if_icmpne` del
// JVMS es `if-ne` de dalvik, y `ifnonnull` es `if-nez`, porque el null es cero para la maquina.
var ifContraCero = map[string]byte{
	"ifeq": opIfEqz, "ifne": opIfNez, "iflt": opIfLtz, "ifge": opIfGez, "ifgt": opIfGtz, "ifle": opIfLez,
	"ifnull": opIfEqz, "ifnonnull": opIfNez,
}

var ifEntreDos = map[string]byte{
	"if_icmpeq": opIfEq, "if_icmpne": opIfNe, "if_icmplt": opIfLt,
	"if_icmpge": opIfGe, "if_icmpgt": opIfGt, "if_icmple": opIfLe,
}

// traduce es el estado de un método mientras se traduce. Se corre dos veces: la primera con una
// base tan ancha como la pila máxima que midió javap, para saber cuántos temporales hacen falta de
// verdad; la segunda con esa cuenta como base, que es lo que fija dónde caen los argumentos y por
// tanto las palabras definitivas.
type traduce struct {
	t         *dexTables
	x         poolIndex
	clase     classFull
	m         classMethod
	base      int           // primer registro de los locales: cuántos temporales se usan
	locales   int           // m.LocalCount, medido sobre el .class
	enPila    []int         // registro de cada entrada de la pila JVM
	libres    []int         // temporales ya muertos
	renom     map[int]int   // local JVM -> registro que lo guarda, cuando d8 lo fundió con otro
	fijos     map[int]bool  // temporal que quedó atado a un local: ya no es reusable
	altos     map[int]bool  // registro ALTO de un par vivo: no se libera hasta que muera el par
	pcs       map[int]int   // pc JVM -> unidad dalvik donde cae su instrucción, para resolver saltos
	saltos    []parcheDEX   // branquias abiertas: dónde va el desplazamiento y a qué pc apunta
	cargas    []cargaSwitch // payloads de switch, que se escriben al final del cuerpo
	siguiente int           // próximo temporal nuevo
	pico      int           // el mayor número de temporal usado
	techo     bool          // la base es un límite duro: solo en la pasada que escribe las palabras
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

// esPresupuesto: el fallo es del reparto de registros —la base se quedó corta—, y eso se arregla
// repitiendo la medida con más temporales. Cualquier otro motivo (un opcode sin medir, un tipo sin
// internar) no mejora subiendo la base y se devuelve tal cual, sin dar vueltas.
func esPresupuesto(err error) bool {
	te, ok := err.(*traduceError)
	if !ok {
		return false
	}
	return strings.HasPrefix(te.motivo, "it needs more temporaries") ||
		strings.HasPrefix(te.motivo, "it needs more wide temporaries")
}

// ocupado: el registro no se puede repartir. Está ocupado si es un local (`fijos`), si es la mitad
// alta de un par vivo (`altos`) o si lo usa un valor de la pila. Sin esta comprobación el par nuevo
// caía sobre la mitad alta de un par vivo: medido en ART con `ArrMain`, donde `move-result-wide v7`
// (par v7,v8) y `int-to-double` pedían (8,9) y los dos pares se solapaban en v8.
func (p *traduce) ocupado(r int) bool {
	if p.fijos[r] || p.altos[r] {
		return true
	}
	for _, v := range p.enPila {
		if v == r {
			return true
		}
	}
	return false
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
	for p.ocupado(r) {
		r = p.siguiente
		p.siguiente++
	}
	if p.siguiente > p.pico {
		p.pico = p.siguiente
	}
	if p.techo && r >= p.base {
		p.falla(in, "it needs more temporaries than the max stack javap measured")
	}
	return r
}

// tomaPar: dos registros **consecutivos** para un valor ancho. Las formas `*-wide` del dex nombran la
// mitad baja y la alta se sobreentiende, así que un par suelto no vale: medido en `d8` sobre `L` y `V`
// (parrafo 74), donde `const-wide/16 v0` va seguido de `add-long/2addr v0, v4` y del `return-wide v0`.
// La bolsa se recorre entera buscando el par consecutivo **más bajo**, no solo las dos últimas
// entradas: medido con `Lp/Ver;flotantes` después del §80, donde la bolsa `[0 2 3 4 5 6 7]` sí tenía
// par (2,3) y la regla de las dos últimas pedía dos registros nuevos, llevando el marco a v17.
func (p *traduce) tomaPar(in jvmInstr) int {
	bajo := -1
	for _, a := range p.libres {
		if bajo >= 0 && a >= bajo {
			continue
		}
		for _, b := range p.libres {
			if b == a+1 {
				bajo = a
				break
			}
		}
	}
	if bajo >= 0 {
		vivas := []int{}
		for _, r := range p.libres {
			if r != bajo && r != bajo+1 {
				vivas = append(vivas, r)
			}
		}
		p.libres = vivas
		p.marcaPar(bajo)
		return bajo
	}
	r := p.siguiente
	for p.ocupado(r) || p.ocupado(r+1) {
		r += 2
	}
	p.siguiente = r + 2
	if p.siguiente > p.pico {
		p.pico = p.siguiente
	}
	p.marcaPar(r)
	if p.techo && r+1 >= p.base {
		p.falla(in, "it needs more wide temporaries than the max stack javap measured")
	}
	return r
}

// marcaPar: la mitad alta del par queda protegida mientras el par viva en la pila (ver `altos`).
func (p *traduce) marcaPar(r int) {
	if p.altos == nil {
		p.altos = map[int]bool{}
	}
	p.altos[r+1] = true
}

// sueltaPar: devuelve las dos mitades, en el orden que `tomaPar` vuelve a reconocer, solo cuando el
// par ya no está en la pila; la marca de la mitad alta se retira con él.
func (p *traduce) sueltaPar(r int) {
	for _, vivo := range p.enPila {
		if vivo == r {
			return
		}
	}
	delete(p.altos, r+1)
	p.suelta(r + 1)
	p.suelta(r)
}

// cargaLargo: `lconst_0`, `lconst_1` y `ldc2_w` bajan a las tres formas de constante ancha que escribe
// d8, según el tamaño del valor: `const-wide/16` (`1600 0200`), `const-wide/32` (`1700 7011 0100`) o
// `const-wide` con los ocho bytes en little-endian (`1800 cb04 fb71 1f01` para 1234567890123L).
func (p *traduce) cargaLargo(in jvmInstr, v int64) {
	r := p.tomaPar(in)
	if p.err != nil {
		return
	}
	if !p.octeto(r, in) {
		return
	}
	switch {
	case v == int64(int16(v)):
		p.palabra(uint16(opConstWide16) | uint16(r&0xff)<<8)
		p.palabra(uint16(int16(v)))
	case v == int64(int32(v)):
		p.palabra(uint16(opConstWide32) | uint16(r&0xff)<<8)
		p.palabra(uint16(uint32(v)))
		p.palabra(uint16(uint32(v) >> 16))
	default:
		p.palabra(uint16(opConstWide) | uint16(r&0xff)<<8)
		for i := 0; i < 4; i++ {
			p.palabra(uint16(uint64(v) >> uint(16*i)))
		}
	}
	p.push(r)
}

// cuentaAncha: las cinco cuentas de `long`, `float` y `double`, con el mismo reparto que
// `cuentaEntera` (parrafo 66): `/2addr` sobre el operando muerto, conmutando si hace falta, y la forma
// de tres registros si los dos siguen vivos. Medido en `d8`: `add-long/2addr v0, v4` (`bb40`),
// `mul-long v4, v4, v0` (`9d04 0400`) y `div-long/2addr v0, v2` (`be20`) (parrafo 74); para el punto
// flotante, `sub-float/2addr v0, v1` = `c701` en `resta` y `add`/`mul`/`div` de `double` =
// 0xcb/0xcd/0xce en `suma`, `por` y `divide`, leidos sobre su fichero en el parrafo 80. Las formas de
// tres registros del `float` y del `double` no las escribio ninguna sonda, asi que ese camino se
// rechaza por nombre.
func (p *traduce) cuentaAncha(in jvmInstr) {
	dos := map[string]byte{
		"ladd": 0xbb, "lsub": 0xbc, "lmul": 0xbd, "ldiv": 0xbe, "lrem": 0xbf,
		"fadd": 0xc6, "fsub": 0xc7, "fmul": 0xc8, "fdiv": 0xc9, "frem": 0xca,
		"dadd": 0xcb, "dsub": 0xcc, "dmul": 0xcd, "ddiv": 0xce, "drem": 0xcf,
	}
	tres := map[string]byte{"ladd": 0x9b, "lsub": 0x9c, "lmul": 0x9d, "ldiv": 0x9e, "lrem": 0x9f}
	op, ok := dos[in.name]
	if !ok {
		p.falla(in, "a wide arithmetic opcode this translator does not know")
		return
	}
	conmuta := strings.HasSuffix(in.name, "add") || strings.HasSuffix(in.name, "mul")
	b := p.pop(in)
	a := p.pop(in)
	if p.err != nil {
		return
	}
	vivo := func(r int) bool {
		for _, v := range p.enPila {
			if v == r || v+1 == r {
				return true
			}
		}
		return false
	}
	switch {
	case !vivo(a):
		if !p.nibble(a, in) || !p.nibble(b, in) {
			return
		}
		p.palabra(unidad12x(op, a, b))
		p.push(a)
		p.sueltaPar(b)
	case conmuta && !vivo(b):
		if !p.nibble(b, in) || !p.nibble(a, in) {
			return
		}
		p.palabra(unidad12x(op, b, a))
		p.push(b)
		p.sueltaPar(a)
	default:
		tresOp, medida := tres[in.name]
		if !medida {
			p.falla(in, "the three-register form of a float or double arithmetic: only the /2addr was measured (parrafo 80)")
			return
		}
		r := p.tomaPar(in)
		if p.err != nil {
			return
		}
		if !p.octeto(r, in) || !p.octeto(a, in) || !p.octeto(b, in) {
			return
		}
		u1, u2 := unidad23x(tresOp, r, a, b)
		p.palabra(u1)
		p.palabra(u2)
		p.push(r)
		p.sueltaPar(a)
		p.sueltaPar(b)
	}
}

// comparaAncha: `lcmp`, `fcmpl`, `fcmpg`, `dcmpl` y `dcmpg` (párrafo 81). Consume las dos parejas
// —el largo o el flotante de cada lado— y deja el entero -1/0/1 en un temporal suelto, porque los
// dos operandos pueden seguir vivos por un `dup`. La rama que lo consume es la `if-*z` de siempre.
func (p *traduce) comparaAncha(in jvmInstr) {
	op, ok := map[string]byte{
		"fcmpl": opFcmplDex, "fcmpg": opFcmpgDex,
		"dcmpl": opDcmplDex, "dcmpg": opDcmpgDex,
		"lcmp":  opLcmpDex,
	}[in.name]
	if !ok {
		p.falla(in, "a wide comparison this translator does not know")
		return
	}
	b := p.pop(in)
	a := p.pop(in)
	if p.err != nil {
		return
	}
	r := p.toma(in)
	if p.err != nil {
		return
	}
	if !p.octeto(r, in) || !p.octeto(a, in) || !p.octeto(b, in) {
		return
	}
	u1, u2 := unidad23x(op, r, a, b)
	p.palabra(u1)
	p.palabra(u2)
	p.sueltaPar(b)
	p.sueltaPar(a)
	p.push(r)
}

// conviertePalabra: una conversion que no cambia el numero de palabras (`int-to-float` 0x82 y
// `float-to-int` 0x87, parrafo 80): un registro dentro y otro fuera.
func (p *traduce) conviertePalabra(in jvmInstr, op byte) {
	r := p.pop(in)
	if p.err != nil {
		return
	}
	d := p.toma(in)
	if p.err != nil {
		return
	}
	if !p.nibble(r, in) || !p.nibble(d, in) {
		return
	}
	p.palabra(unidad12x(op, d, r))
	p.suelta(r)
	p.push(d)
}

// cargaFlotante: un `float` en un temporal. Lo que va en el registro son los **bits** de 32, no el
// numero: `0.0f` sale `const/4 v0, #0` —asi lo escribe `d8` en `FyD.constanteCorta`— y `1.0f` es
// `const v0, #+0x3F800000` (parrafo 80).
func (p *traduce) cargaFlotante(in jvmInstr, v float64) {
	r := p.toma(in)
	if p.err != nil {
		return
	}
	if !p.nibble(r, in) {
		return
	}
	p.palabraFlotante(r, v, in)
	p.push(r)
}

// palabraFlotante: la instruccion de constante que cabe, con el destino ya elegido.
func (p *traduce) palabraFlotante(r int, v float64, in jvmInstr) {
	bits := uint32(math.Float32bits(float32(v)))
	switch {
	case bits == uint32(uint32(int8(bits))):
		p.palabra(unidad11n(opConst4, r, int(int8(bits))))
	case bits == uint32(uint32(int16(bits))):
		p.palabra(uint16(opConst16) | uint16(r&0xff)<<8)
		p.palabra(uint16(int16(bits)))
	default:
		// `const vAA, #+BBBBBBBB` (31u): dos unidades de literal
		p.palabra(uint16(opConst) | uint16(r&0xff)<<8)
		p.palabra(uint16(bits))
		p.palabra(uint16(bits >> 16))
	}
}

// cargaDoble: un `double` en un par de registros, con los bits de 64. `d8` escribe `1.0` como
// `const-wide/high16 v2, #+0x3FF0` (dos unidades) y `3.14159` como `const-wide` de cinco (parrafo 80);
// la eleccion es del emisor, no del formato, así que basta con que la forma sea valida.
func (p *traduce) cargaDoble(in jvmInstr, v float64) {
	r := p.tomaPar(in)
	if p.err != nil {
		return
	}
	if !p.octeto(r, in) {
		return
	}
	bits := math.Float64bits(v)
	switch {
	case bits == uint64(uint64(int64(int16(bits)))):
		p.palabra(uint16(opConstWide16) | uint16(r&0xff)<<8)
		p.palabra(uint16(int16(bits)))
	case bits<<16 == 0:
		// los dieciseis bits altos con signo y el resto ceros: `const-wide/high16` (0x19), dos
		// unidades —asi escribe `d8` el `1.0` de `deEntero` y el `-2.5` de `negativo` (parrafo 80)—
		p.palabra(uint16(opConstWideAlto16) | uint16(r&0xff)<<8)
		p.palabra(uint16(bits >> 48))
	case bits == uint64(uint64(int64(int32(bits)))):
		p.palabra(uint16(opConstWide32) | uint16(r&0xff)<<8)
		p.palabra(uint16(uint32(bits)))
		p.palabra(uint16(uint32(bits) >> 16))
	default:
		p.palabra(uint16(opConstWide) | uint16(r&0xff)<<8)
		for i := 0; i < 4; i++ {
			p.palabra(uint16(bits >> uint(16*i)))
		}
	}
	p.push(r)
}

// convierteEnAncho: `i2l` sale `int-to-long vA, vB` (`8120` para v0, v2), un registro suelto dentro de
// un par (parrafo 74).
func (p *traduce) convierteEnAncho(in jvmInstr, op byte) {
	r := p.pop(in)
	if p.err != nil {
		return
	}
	d := p.tomaPar(in)
	if p.err != nil {
		return
	}
	if !p.nibble(r, in) || !p.nibble(d, in) {
		return
	}
	p.palabra(unidad12x(op, d, r))
	p.suelta(r)
	p.push(d)
}

// convierteDeAncho: `l2i` sale `long-to-int v1, v0` (`8401`).
func (p *traduce) convierteDeAncho(in jvmInstr, op byte) {
	r := p.pop(in)
	if p.err != nil {
		return
	}
	p.sueltaPar(r)
	d := p.toma(in)
	if p.err != nil {
		return
	}
	if !p.nibble(r, in) || !p.nibble(d, in) {
		return
	}
	p.palabra(unidad12x(op, d, r))
	p.push(d)
}

// typeIdxDe: el `type_id` de un descriptor, con rechazo con nombre si la pasada de colecta no lo
// internó. Leer el mapa a secas devuelve cero para una clave ausente, y un cero mal escrito es un dex
// que `dexdump` traga y ART corta en ejecución: medido con `p.W2.dos` después del §79, donde la clase
// del `instanceof CharSequence` no se volvía a nombrar en todo el fichero y la instrucción salía
// `instance-of v5, v4, I` («instance-of on unexpected class Integer»).
func (p *traduce) typeIdxDe(desc string, in jvmInstr) int {
	i, ok := p.t.typeIdx[desc]
	if !ok {
		p.falla(in, "the type_id "+desc+" was never interned (the collector does not follow this operand)")
		return 0
	}
	return i
}

// suelta: devuelve un temporal solo si ninguna otra entrada de la pila lo está usando. Comprobarlo
// cada vez cuesta un recorrido corto y es lo que impide que un dup pierda un valor por escribir dos
// vivos en el mismo registro. Y un local nunca se suelta: por debajo de base solo hay temporales, y
// el registro de un argumento se vuelve a leer en la llamada siguiente.
func (p *traduce) suelta(r int) {
	if r >= p.base {
		return
	}
	if p.fijos[r] {
		// atado a un local: vive mientras viva el método
		return
	}
	for _, vivo := range p.enPila {
		if vivo == r {
			return
		}
	}
	// `altos`: r es la mitad ALTA de un par que sigue vivo. La pila guarda solo el registro bajo, así
	// que sin esta marca la mitad alta se liberaba con el par vivo y el plan la reutilizaba (medido en
	// ART con `ArrMain`: `mul-double/2addr v7, v8` con los pares v7,v8 y v8,v9 solapados,
	// `wide register v7 has type Double (Low Half)`).
	if p.altos[r] {
		return
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

// guardaLocal: `istore`/`astore`. d8 no copia: funde el registro del valor con el del local, como
// midió la sonda del parrafo 50 con `int x = a; x = x + 1; return x`, que sale en una sola
// instrucción. Al atar un temporal a un local hay que sacarlo de la bolsa de reutilización, porque
// si no el siguiente `toma` escribiría encima de una variable viva.
func (p *traduce) guardaLocal(in jvmInstr, n int) {
	p.guardaLocalEn(in, n, false)
}

// guardaLocalEn: `istore`/`astore`, y con `ancho` los `lstore` (parrafo 74). Un valor ancho vive en un
// PAR de registros consecutivos, así que la fusion y el `move-wide` atizan los dos.
func (p *traduce) guardaLocalEn(in jvmInstr, n int, ancho bool) {
	r := p.pop(in)
	destino := p.local(n)
	if r == destino {
		return
	}
	if p.renom == nil {
		p.renom = map[int]int{}
	}
	// La fusión (el local es el registro del valor, sin instrucción) solo vale en la PRIMERA
	// escritura de un local que no es argumento y cuando el valor viene de un temporal. Las demás
	// escrituras sueltan un `move`/`move-object` al registro del local: el registro de un argumento
	// puede seguir vivo, y dos asignaciones en ramas distintas tienen que caer en el mismo registro
	// (§71; `atrapa` daba `VerifyError` sin esto). d8 mueve cuando el valor viene de un local o de un
	// argumento vivo, medido en `Mv2.dos` (`0110`) y `Mv3.usa` (`0730`, `0720`).
	_, yaAtado := p.renom[n]
	if !yaAtado && n >= p.m.ArgCount && r < p.base {
		p.renom[n] = r
		if p.fijos == nil {
			p.fijos = map[int]bool{}
		}
		quitar := []int{r}
		if ancho {
			quitar = append(quitar, r+1)
		}
		for _, q := range quitar {
			p.fijos[q] = true
			for i, vivo := range p.libres {
				if vivo == q {
					p.libres = append(p.libres[:i], p.libres[i+1:]...)
					break
				}
			}
		}
		return
	}
	op := byte(opMueve)
	switch {
	case ancho:
		op = opMueveWide
	case in.name == "astore" || (len(in.name) == 8 && in.name[:6] == "astore"):
		op = opMueveObjeto
	}
	// El destino de la escritura es el registro del local; si es inalcanzable para cuatro bits se baja el
	// hueco a un temporal (parrafo 86) y el valor se copia sobre ese bajo. La otra dirección —escribir
	// SOBRE un registro alto— no la emite `d8` nunca: renombra (medido en `D16d.reasignaEntero`).
	destino = p.bajaAlto(in, n, op)
	if p.err != nil {
		return
	}
	if r > 15 {
		p.palabra(unidad22v(mueveFrom16(op), destino))
		p.palabra(uint16(r))
	} else {
		if !p.nibble(r, in) {
			return
		}
		p.palabra(unidad12x(op, destino, r))
	}
	if ancho {
		p.sueltaPar(r)
		return
	}
	p.suelta(r)
}

// cargaLocal: leer un hueco JVM que no es ancho. Con el registro del local por debajo de v16 se empuja
// el suyo propio, sin instrucción (la fusión que midió el parrafo 50); con v16 o más hay que bajarlo,
// porque las formas de cuatro bits no lo alcanzan y `d8` lo copia a un temporal con la forma 22v
// (medido en `D16.junta`, parrafo 86: `0202 1200` = `move/from16 v2, v18`, y de ahí `int-to-double v2,
// v2`). `op` es el move de cuatro bits que habría que emitir, y su pareja en 22v sale sola.
func (p *traduce) cargaLocal(in jvmInstr, n int, op byte) {
	r := p.bajaAlto(in, n, op)
	if p.err != nil {
		return
	}
	p.push(r)
}

// cargaLocalPar: lo mismo para un `long` o un `double`, cuyo valor ocupa DOS registros: la pareja de
// destino tiene que caber entera en cuatro bits, así que el techo es v14. Medido el
// `move-wide/from16` en `D16c.guardaLargo` (`0501 1300`, parrafo 86).
func (p *traduce) cargaLocalPar(in jvmInstr, n int) {
	p.cargaLocal(in, n, opMueveWide)
}

// mueveFrom16: la forma 22v del move que se habría emitido en 12x.
func mueveFrom16(op byte) byte {
	switch op {
	case opMueveObjeto:
		return opMueveObjetoFrom16
	case opMueveWide:
		return opMueveWideFrom16
	}
	return opMueveFrom16
}

// bajaAlto: atar a un temporal bajo un hueco cuyo registro no cabe en cuatro bits, y devolver ese
// registro. `d8` resuelve esto renombrando el local (medido en `D16d.reasignaEntero`, donde el `k` del
// argumento v16 se va a v4); aquí no hace falta ningún análisis de viveza: el hueco queda atado al
// temporal con el mismo mecanismo con el que `guardaLocal` ata la primera escritura, así que el
// registro alto no se vuelve a nombrar nunca. La copia se emite UNA sola vez por hueco, no por lectura:
// copiar en cada lectura costaba un temporal más por uso y el marco de `Ver.compara` se salía de v15.
func (p *traduce) bajaAlto(in jvmInstr, n int, op byte) int {
	ancho := op == opMueveWide
	techo := 15
	if ancho {
		techo = 14
	}
	r := p.local(n)
	if r <= techo {
		return r
	}
	var t int
	if ancho {
		t = p.tomaPar(in)
	} else {
		t = p.toma(in)
	}
	if p.err != nil {
		return 0
	}
	if t > techo {
		p.falla(in, "no low register is free to carry a high slot down")
		return 0
	}
	p.palabra(unidad22v(mueveFrom16(op), t))
	p.palabra(uint16(r))
	if p.renom == nil {
		p.renom = map[int]int{}
	}
	p.renom[n] = t
	if p.fijos == nil {
		p.fijos = map[int]bool{}
	}
	quitar := []int{t}
	if ancho {
		quitar = append(quitar, t+1)
	}
	for _, q := range quitar {
		p.fijos[q] = true
		for i, vivo := range p.libres {
			if vivo == q {
				p.libres = append(p.libres[:i], p.libres[i+1:]...)
				break
			}
		}
	}
	return t
}

// local: el registro dalvik del enésimo hueco JVM. Un long o un double ocupa dos huecos aquí y dos
// registros allá, así que el número de hueco se pasa tal cual. Si el hueco quedó atado a otro
// registro (guardaLocal), ese es su registro ahora.
//
// Los argumentos van al FINAL de la frame, no al principio como en el `.class`: lo exige el formato
// y lo comprueba ART. Medido en el aparato (2026-11-01, §56) con un método de cuatro huecos y dos
// argumentos: nuestro `code_item` decía `registers=10`, `ins=2` y usaba `v6` para `this`, y el
// verificador cortaba con `instance field access on object that has non-reference type Undefined` —
// para ART `v6` era un local sin tipo. Con `d8` el mismo cuerpo sale `registers=3`, `ins=2` y
// `this` en `v1`, o sea los dos últimos registros. Mientras el método no tiene más locales que
// argumentos las dos cuentas coinciden, y por eso el subconjunto medido hasta aquí salía bien.
// esHandler: el pc es el destino de un `catch` de la tabla del `.class`.
func (p *traduce) esHandler(pc int) bool {
	for _, tr := range p.m.Tries {
		if tr.handler == pc {
			return true
		}
	}
	return false
}

// indiceDeStore: el hueco que escribe un `astore`/`astore_N`.
func indiceDeStore(nombre string, in jvmInstr) (int, bool) {
	switch nombre {
	case "astore":
		return int(in.ops[0]), true
	case "astore_0", "astore_1", "astore_2", "astore_3":
		return int(nombre[len(nombre)-1]) - '0', true
	}
	return 0, false
}

func (p *traduce) local(n int) int {
	if r, ok := p.renom[n]; ok {
		return r
	}
	if n < p.m.ArgCount {
		return p.base + p.locales - p.m.ArgCount + n
	}
	return p.base + n - p.m.ArgCount
}

func (p *traduce) palabra(u uint16) { p.w = append(p.w, u) }

// nibble: los formatos 11n, 12x y 35c guardan el registro en CUATRO bits. Pasar de v15 no se
// recorta: se rechaza, porque un v16 mal codificado es otra instrucción. Los 11x y 21c usan un byte
// entero y sí llegan a 255.
func (p *traduce) nibble(r int, in jvmInstr) bool {
	if r < 0 || r > 15 {
		// la pasada de medida trabaja con una conjetura de base: los números de registro que ve son
		// provisionales y el marco definitivo sale más abajo, así que el techo lo pone solo la pasada
		// que escribe las palabras
		if !p.techo {
			return true
		}
		p.falla(in, "register v"+itoa(r)+" does not fit a four-bit format (the /range or /from16 form is not measured here)")
		return false
	}
	return true
}

// octeto: los registros de una 23x van en ocho bits cada uno.
func (p *traduce) octeto(r int, in jvmInstr) bool {
	if r < 0 || r > 0xff {
		if !p.techo {
			return true
		}
		p.falla(in, "register v"+itoa(r)+" does not fit the eight-bit registers of the three-register form")
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

// unidad22v: el opcode y el destino de OCHO bits en la primera palabra; la fuente de DIECISÉIS va en la
// unidad siguiente, que el llamante escribe despues. Medido en `0202 1200` = `move/from16 v2, v18`
// (parrafo 86).
func unidad22v(op byte, destino int) uint16 {
	return uint16(op) | uint16(destino&0xff)<<8
}

// unidad23x: el opcode y el primer registro (OCHO bits) en una unidad, y los otros dos en la siguiente,
// cada uno en su byte. Medido en `4402 0301` (aget v2, v3, v1) y `4b02 0001` (aput v2, v0, v1); los dos
// campos de la segunda unidad son de ocho bits, no de cuatro, así que un v16 no se recorta.
func unidad23x(op byte, a, b, c int) (uint16, uint16) {
	return uint16(op) | uint16(a&0xff)<<8, uint16(b&0xff) | uint16(c&0xff)<<8
}

// tiposNewArray: el atype del JVMS 4.9 para `newarray` y el array que nombra. Los ocho son la tabla
// entera; el subconjunto medido solo escribe `int` desde el fuente, y los demás se aceptan si un
// `.class` los trae porque la traducción es la misma.
var tiposNewArray = map[byte]string{
	4: "[Z", 5: "[C", 6: "[F", 7: "[D", 8: "[B", 9: "[S", 10: "[I", 11: "[J",
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

// accesoEstatico: `getstatic` y `putstatic`. Son 21c —opcode, registro e índice— y no llevan
// receptor: la sonda del parrafo 50 midió `sget v0, field@0` (0x0060 + 0x0000) al leer el campo y
// `const/16 v0, 9` seguido de `sput v0, field@0` (0x0067) al escribirlo. Para una referencia es la
// misma forma con el opcode de objeto, medido sobre `System.out.println` en el parrafo 62.
func (p *traduce) accesoEstatico(in jvmInstr, escribe bool) {
	owner, nombre, desc, _ := p.x.refOf(dexU16(in.ops))
	tipo := byte(0)
	if len(desc) > 0 {
		tipo = desc[0]
	}
	if desc != "I" && desc != "F" && !esObjeto(desc) && !esAncho(desc) {
		p.falla(in, "a static access of type "+string(tipo)+": se midieron el entero, la referencia, el flotante y los anchos; lo demas no")
		return
	}
	campo, ok := p.t.fieldIdx[fieldKey{owner: owner, typeOf: desc, name: nombre}]
	if !ok {
		p.falla(in, "a field that was never interned ("+owner+"."+nombre+" "+desc+")")
		return
	}
	op := byte(opSget)
	if escribe {
		op = opSput
	}
	if esObjeto(desc) {
		if escribe {
			op = opSputObject
		} else {
			op = opSgetObject
		}
	}
	if esAncho(desc) {
		// `sget-wide v2, field@0` es `6102 0000` y `sput-wide v2, field@0` `6802 0000`: la misma forma
		// 21c con el par de registros en vAA (parrafo 74)
		if escribe {
			op = opSPutWide
		} else {
			op = opSGetWide
		}
	}
	// al escribir el registro sale de la pila, y **no** se pide uno nuevo: pedirlo antes de sacarlo
	// dejaba un temporal asignado y perdido por cada `putstatic`, y con dos en un método el marco se
	// quedaba sin temporales —lo destapó el banco al añadir un `println` con concatenación (§62)—
	var r int
	switch {
	case escribe:
		r = p.pop(in)
	case esAncho(desc):
		r = p.tomaPar(in)
	default:
		r = p.toma(in)
	}
	if p.err != nil {
		return
	}
	p.palabra(uint16(op) | uint16(r&0xff)<<8)
	if !p.indice(campo, in, "the field_id of the static access") {
		return
	}
	if escribe {
		if esAncho(desc) {
			p.sueltaPar(r)
		} else {
			p.suelta(r)
		}
		return
	}
	p.push(r)
}

// parcheDEX: una branquia cuyo desplazamiento todavía no se conoce. `desde` es la unidad donde
// empieza la instrucción —el desplazamiento dalvik se cuenta desde ahí, en unidades de código, y no
// desde el operando—, `en` la unidad donde va a escribirse, y `destino` el pc JVM al que se salta.
// `enMitadAlta` marca el `goto` 10t, que lleva los ocho bits dentro de su propia unidad.
type parcheDEX struct {
	desde, en, destino int
	enMitadAlta        bool
}

// salto emite la branquia con el desplazamiento en blanco y la deja apuntada al pc destino. `consume`
// dice cuántos registros baja: dos en la familia de dos operandos, uno en las formas contra cero y
// contra null, ninguno en el `goto`. Las unidades que escribe cada forma son las que midió `d8` sobre
// la sonda del parrafo 52: if-ne v1, v2 es 0x2133 con el desplazamiento en la unidad siguiente;
// if-nez v1 es 0x0139 igual; y el goto de d8 es 0x0228, con el dos en la mitad alta de su propia
// unidad (y 0x0528 cuando el hueco llega a cinco).
func (p *traduce) salto(op byte, destino int, consume int, in jvmInstr) {
	desde := len(p.w)
	switch consume {
	case 2:
		b := p.pop(in)
		a := p.pop(in)
		if !p.nibble(a, in) || !p.nibble(b, in) {
			return
		}
		p.palabra(unidad12x(op, a, b))
		p.palabra(0)
		p.suelta(a)
		p.suelta(b)
	case 1:
		a := p.pop(in)
		if !p.nibble(a, in) {
			return
		}
		p.palabra(unidad11x(op, a))
		p.palabra(0)
		p.suelta(a)
	default:
		p.palabra(uint16(op)) // el goto 10t: la mitad alta queda en blanco hasta el parche
	}
	p.abreSalto(desde, len(p.w)-1, destino, consume == 0, in)
}

func (p *traduce) abreSalto(desde, en, destino int, mitadAlta bool, in jvmInstr) {
	if destino < 0 || destino > len(p.m.Code) {
		p.falla(in, "a branch to pc "+itoa(destino)+", outside the code of the method")
		return
	}
	p.saltos = append(p.saltos, parcheDEX{desde: desde, en: en, destino: destino, enMitadAlta: mitadAlta})
}

// parchaSaltos completa los desplazamientos al final del método, cuando ya se escribió hasta el
// último destino. Un pc que no es el principio de ninguna instrucción no se inventa: se rechaza,
// porque llegar ahí significa que el salto entra en medio de un operando.
func (p *traduce) parchaSaltos() error {
	for _, s := range p.saltos {
		donde, ok := p.pcs[s.destino]
		if !ok {
			if s.destino != len(p.m.Code) {
				return &traduceError{pc: s.destino, nombre: p.m.Name,
					motivo: "a branch that does not land on the start of an instruction"}
			}
			donde = len(p.w) // se salta al hueco que hay despues de la ultima instruccion
		}
		rel := donde - s.desde
		// Las ramas hacia atrás son bucles, y están medidas: `d8` escribe el cuerpo del bucle con un
		// `goto` 10t de desplazamiento negativo —`28f7`, o sea -9 unidades, medido en el §63 sobre un
		// `while` de javac— y la salida con un 22t hacia adelante. El reparto de registros no cambia
		// porque los locales van clavados (`guardaLocal`) y en la cabecera del bucle la pila JVM está
		// vacía, que es lo que exige el verificador para los dos caminos. Lo que no cabe en el formato
		// se sigue rechazando por nombre.
		if s.enMitadAlta {
			if rel < -128 || rel > 127 {
				return &traduceError{nombre: p.m.Name,
					motivo: "a goto of " + itoa(rel) + " code units: the 10t form reaches +-127 and goto/16 is not measured here"}
			}
			p.w[s.en] |= uint16(rel&0xff) << 8
			continue
		}
		if rel < -32768 || rel > 32767 {
			return &traduceError{nombre: p.m.Name, motivo: "a branch offset that does not fit sixteen bits"}
		}
		p.w[s.en] = uint16(rel)
	}
	return nil
}

// cuentaEntera: iadd, isub, imul, idiv, irem. Dalvik los escribe sobre el primer operando
// (`/2addr`), que es lo que salió de la sonda: `add(a,b)` es 0x21b0 = add-int/2addr v1, v2 y
// `sub(a,b)` 0x21b1, con el resultado quedándose en v1, de donde sale el `return v1` (0x010f).
//
// Pero `/2addr` escribe sobre un registro que la pila puede estar usando más abajo, y eso no se vio
// hasta ejecutar `v[i] = i + 1` en ART: el índice y el sumando eran el mismo registro y el `iadd`
// pisaba el índice. Medido en el parrafo 66 con `Vivo`/`Vivo2`, `d8` tiene tres formas:
//
//   - la izquierda muere: `/2addr` sobre ella (lo de siempre);
//   - la izquierda vive y la operación conmuta (suma, producto): `/2addr` sobre la DERECHA
//     (`add-int/2addr v2, v1` para `i + j`, con v1 vivo como índice);
//   - la izquierda vive y no conmuta (resta, división, residuo): la forma de tres registros
//     (`sub-int v2, v1, v2` = `9102 0102`, `div-int` = `9302 0102`, `rem-int` = `9402 0102`).
func (p *traduce) cuentaEntera(in jvmInstr) {
	dos := map[string]byte{"iadd": opSumaInt2, "isub": opRestaInt2, "imul": opMultiplica2,
		"idiv": opDivide2, "irem": opResiduo2}
	tres := map[string]byte{"iadd": 0x90, "isub": 0x91, "imul": 0x92, "idiv": 0x93, "irem": 0x94}
	op, ok := dos[in.name]
	if !ok {
		p.falla(in, "an arithmetic opcode this translator does not know")
		return
	}
	conmuta := in.name == "iadd" || in.name == "imul"
	b := p.pop(in)
	a := p.pop(in)
	vivo := func(r int) bool {
		for _, v := range p.enPila {
			if v == r {
				return true
			}
		}
		return false
	}
	switch {
	case !vivo(a):
		if !p.nibble(a, in) || !p.nibble(b, in) {
			return
		}
		p.palabra(unidad12x(op, a, b))
		p.push(a)
		p.suelta(b)
	case conmuta && !vivo(b):
		if !p.nibble(b, in) || !p.nibble(a, in) {
			return
		}
		p.palabra(unidad12x(op, b, a))
		p.push(b)
		p.suelta(a)
	default:
		r := p.toma(in)
		if !p.octeto(r, in) || !p.octeto(a, in) || !p.octeto(b, in) {
			return
		}
		u1, u2 := unidad23x(tres[in.name], r, a, b)
		p.palabra(u1)
		p.palabra(u2)
		p.push(r)
		p.suelta(a)
		p.suelta(b)
	}
}

// incrementa: `iinc` (0x84) de un local sale `add-int/lit8 vA, vB, #+C` (0xd8), medido en el
// parrafo 67 sobre `Inc.local` (`d800 0001` para `i++` y `d800 00ff` para `i--`). Es la forma 22b:
// el opcode con el destino en el byte alto, y el registro fuente con la constante al lado.
func (p *traduce) incrementa(in jvmInstr) {
	n := int(in.ops[0])
	c := int(int8(in.ops[1]))
	r := p.local(n)
	if !p.octeto(r, in) {
		return
	}
	p.palabra(uint16(0xd8) | uint16(r&0xff)<<8)
	p.palabra(uint16(r&0xff) | uint16(byte(c))<<8)
}

// longitudArreglo: `array-length` (12x). Sale el alto en un temporal y la referencia se libera; `d8`
// a veces reusa el registro de la referencia como destino (`array-length v0, v0`), pero eso es una
// decisión de vivos y el resultado ejecuta igual.
func (p *traduce) longitudArreglo(in jvmInstr) {
	a := p.pop(in)
	r := p.toma(in)
	if !p.nibble(a, in) || !p.nibble(r, in) {
		return
	}
	p.palabra(unidad12x(opLongitudArreglo, r, a))
	p.push(r)
	p.suelta(a)
}

// nuevoArreglo: `newarray` (primitivo, atype) y `anewarray` (de referencia) salen como `new-array`
// 22c con el type_id del arreglo: `[I` para `new int[n]` y `[Ljava/lang/String;` para `new
// String[n]`. Medido en `Arreglos` (§66).
func (p *traduce) nuevoArreglo(in jvmInstr) {
	var desc string
	if in.name == "newarray" {
		d, ok := tiposNewArray[in.ops[0]]
		if !ok {
			p.falla(in, "a newarray of atype "+itoa(int(in.ops[0]))+", which is not one of the eight")
			return
		}
		desc = d
	} else {
		clase := p.x.classDescriptor(dexU16(in.ops))
		if clase == "" {
			p.falla(in, "an anewarray of a class that is not in the pool")
			return
		}
		desc = "[" + clase
	}
	largo := p.pop(in)
	r := p.toma(in)
	if !p.nibble(largo, in) || !p.nibble(r, in) {
		return
	}
	p.palabra(uint16(opNuevoArreglo) | uint16((largo&0xf)<<4|r&0xf)<<8)
	if !p.indice(p.typeIdxDe(desc, in), in, "the type_id of new-array") {
		return
	}
	p.push(r)
	p.suelta(largo)
}

// leeArreglo: `iaload`/`aaload` (23x). La pila JVM trae el arreglo y detrás el índice.
func (p *traduce) leeArreglo(in jvmInstr) {
	op := byte(opAget)
	ancho := false
	switch in.name {
	case "aaload":
		op = opAgetObjeto
	case "laload", "daload":
		// `aget-wide` 0x45: el destino es un PAR y las dos palabras se leen del arreglo (parrafo 84)
		op = opAgetAncho
		ancho = true
	}
	indice := p.pop(in)
	arreglo := p.pop(in)
	r := 0
	if ancho {
		r = p.tomaPar(in)
	} else {
		r = p.toma(in)
	}
	if p.err != nil {
		return
	}
	a, b := unidad23x(op, r, arreglo, indice)
	p.palabra(a)
	p.palabra(b)
	p.push(r)
	p.suelta(arreglo)
	p.suelta(indice)
}

// escribeArreglo: `iastore`/`aastore` (23x). La pila JVM trae el arreglo, el índice y el valor; el
// primer registro de la instrucción es el valor (medido: `aput v2, v0, v1` para `v[i] = x`). Un valor
// ancho se escribe con `aput-wide` 0x4c y se suelta como par.
func (p *traduce) escribeArreglo(in jvmInstr) {
	op := byte(opAput)
	ancho := false
	switch in.name {
	case "aastore":
		op = opAputObjeto
	case "lastore", "dastore":
		op = opAputAncho
		ancho = true
	}
	valor := p.pop(in)
	indice := p.pop(in)
	arreglo := p.pop(in)
	a, b := unidad23x(op, valor, arreglo, indice)
	p.palabra(a)
	p.palabra(b)
	p.suelta(arreglo)
	p.suelta(indice)
	if ancho {
		p.sueltaPar(valor)
	} else {
		p.suelta(valor)
	}
}

// invoca escribe un 35c: primera unidad con el opcode, el recuento de argumentos en los bits 4..7 y
// el quinto registro en los 0..3; segunda unidad con el índice del método; tercera con los
// registros en nibbles, el primero en los bits bajos. Así están las nueve llamadas del fixture
// (§21): invoke-direct {v0}, method@16 sale 1070 0010 0000 y invoke-interface {v2, v0}, method@3
// sale 2072 0003 0002. Con **cero** argumentos también vale 35c, medido el 2026-11-01 sobre
// `invoke-static {}` de un `main` llamado desde javac: d8 escribe `7100 0600 0000`, o sea el recuento
// a cero en el byte alto, el índice del método detras y la tercera unidad en cero.
func (p *traduce) invoca(op byte, regs []int, metodo int, in jvmInstr) {
	n := len(regs)
	if n > 5 {
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
	if n == 0 {
		p.palabra(0)
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
		// donde cae cada instruccion en unidades de codigo: es lo que hace falta para responder un
		// salto, porque el desplazamiento dalvik se mide desde el principio de la branquia
		if p.pcs == nil {
			p.pcs = map[int]int{}
		}
		p.pcs[in.pc] = len(p.w)
		// el handler de un try empieza con el `astore` de la excepcion (o con un `pop` si no se
		// usa): en DEX eso es `move-exception` (medido en el parrafo 71)
		if p.esHandler(in.pc) {
			if n, ok := indiceDeStore(in.name, in); ok {
				r := p.local(n)
				if !p.octeto(r, in) {
					return p.err
				}
				p.palabra(unidad11x(opMueveExcepcion, r))
				continue
			}
			if in.name == "pop" {
				continue
			}
			p.falla(in, "a handler whose first instruction is "+in.name+", and none of those is measured")
			return p.err
		}
		// mira: la siguiente instruccion JVM. Se usa para saber si el resultado de una llamada se
		// va a tirar: d8 no emite move-result-object cuando el pop lo consume, y las palabras del
		// listener del studio lo dejan ver (§21)
		seTira := k+1 < len(lista) && (lista[k+1].name == "pop" || lista[k+1].name == "pop2")
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
		case "pop2":
			// un valor de dos palabras: el `pop2` que javac pone tras el `access$` de un campo ancho
			// (`D$1.run` es `invokestatic access$002:(LD;J)J; pop2`, párrafo 83). d8 no escribe nada.
			r := p.pop(in)
			if p.err != nil {
				return p.err
			}
			p.sueltaPar(r)
		case "dup2":
			// el par duplicado comparte registros y no emite nada: el `dup2` del access$ de un
			// estático ancho (`E.access$002`: `lload_0; dup2; putstatic; lreturn`, párrafo 83)
			if len(p.enPila) == 0 {
				p.falla(in, "dup2 with an empty JVM stack")
				return p.err
			}
			p.push(p.enPila[len(p.enPila)-1])
		case "dup2_x1":
			// pila [objeto, par] -> [par, objeto, par]: las tres entradas comparten registros y no se
			// emite nada, igual que el `dup_x1` del parrafo 72. Medido en `D.access$002`, que d8 dexa
			// como `iput-wide v1, v0` y `return-wide v1`.
			if len(p.enPila) < 2 {
				p.falla(in, "dup2_x1 with fewer than two values on the JVM stack")
				return p.err
			}
			par := p.pop(in)
			obj := p.pop(in)
			p.push(par)
			p.push(obj)
			p.push(par)
		case "dup":
			if len(p.enPila) == 0 {
				p.falla(in, "dup with an empty JVM stack")
				return p.err
			}
			p.push(p.enPila[len(p.enPila)-1])
		case "dup_x1":
			if len(p.enPila) < 2 {
				p.falla(in, "dup_x1 with fewer than two values on the JVM stack")
				return p.err
			}
			// [objeto, valor] -> [valor, objeto, valor]. Las tres entradas comparten registros y no se
			// emite nada: suelta solo recicla cuando ninguna entrada viva lo nombra (lo mismo que `dup`).
			// d8 lo baja igual: el `access$002` de la sonda sale `iput v1, v0` + `return v1` (parrafo 72).
			valor := p.pop(in)
			objeto := p.pop(in)
			p.push(valor)
			p.push(objeto)
			p.push(valor)
		case "return":
			p.palabra(uint16(opRetornaV))
		case "ireturn", "freturn":
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
			if !p.indice(p.typeIdxDe(desc, in), in, "the type_id of new") {
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
		case "getstatic", "putstatic":
			p.accesoEstatico(in, in.name == "putstatic")
		case "iadd", "isub", "imul", "idiv", "irem":
			p.cuentaEntera(in)
		case "ladd", "lsub", "lmul", "ldiv", "lrem",
			"fadd", "fsub", "fmul", "fdiv", "frem",
			"dadd", "dsub", "dmul", "ddiv", "drem":
			p.cuentaAncha(in)
		case "lcmp", "fcmpl", "fcmpg", "dcmpl", "dcmpg":
			p.comparaAncha(in)
		case "lconst_0", "lconst_1":
			v := int64(0)
			if in.name == "lconst_1" {
				v = 1
			}
			p.cargaLargo(in, v)
		case "fconst_0", "fconst_1", "fconst_2", "dconst_0", "dconst_1":
			// `0.0f` cabe en `const/4 v0, #0` y `1.0` en `const-wide/high16 #+0x3FF0`, igual que hace
			// `d8` en `FyD.constanteCorta` y `FyD.deEntero` (parrafo 80)
			if in.name[0] == 'f' {
				p.cargaFlotante(in, float64(int(in.name[len(in.name)-1]-'0')))
			} else {
				p.cargaDoble(in, float64(int(in.name[len(in.name)-1]-'0')))
			}
		case "ldc2_w":
			idx := int(dexU16(in.ops))
			switch p.x.tagOf(idx) {
			case cpLong:
				p.cargaLargo(in, p.x.longOf(idx))
			case cpDouble:
				// `ldc2_w #7 // double 3.14159d` sale en `d8` como `const-wide` con los ocho bytes
				// en little-endian (parrafos 77 y 80)
				p.cargaDoble(in, math.Float64frombits(uint64(p.x.longOf(idx))))
			default:
				p.falla(in, "a ldc2_w whose pool entry is neither a Long nor a Double")
				return p.err
			}
		case "lload", "lload_0", "lload_1", "lload_2", "lload_3",
			"dload", "dload_0", "dload_1", "dload_2", "dload_3":
			n := 0
			if in.name == "lload" || in.name == "dload" {
				n = int(in.ops[0])
			} else {
				n = int(in.name[len(in.name)-1]) - '0'
			}
			p.cargaLocalPar(in, n)
		case "lstore", "lstore_0", "lstore_1", "lstore_2", "lstore_3",
			"dstore", "dstore_0", "dstore_1", "dstore_2", "dstore_3":
			n := 0
			if in.name == "lstore" || in.name == "dstore" {
				n = int(in.ops[0])
			} else {
				n = int(in.name[len(in.name)-1]) - '0'
			}
			p.guardaLocalEn(in, n, true)
		case "i2l", "i2f", "i2d", "l2i", "l2f", "l2d", "f2i", "f2l", "f2d", "d2i", "d2l", "d2f":
			// la rejilla de conversiones, medida en `FyD` con `d8` en el parrafo 80: `int-to-double`
			// 0x83 (`divide`, `deEntero`), `float-to-double` 0x89 (`por`, `aDoble`, `campoEstatico`),
			// `double-to-int` 0x8A (`aEntero`) y `double-to-float` 0x8C (`aFlotante`). Las demas salen
			// del mismo paso —el bloque es la rejilla del `.class` menos cuatro, anclada en `int-to-long`
			// 0x81 y `long-to-int` 0x84 del parrafo 74— y `l2f`, `l2d`, `f2l` y `d2l` no los escribio
			// todavia ninguna sonda.
			op, medida := map[string]byte{
				"i2l": opIntALargo, "i2f": 0x82, "i2d": 0x83,
				"l2i": opLargoAInt, "l2f": 0x85, "l2d": 0x86,
				"f2i": 0x87, "f2l": 0x88, "f2d": 0x89,
				"d2i": 0x8a, "d2l": 0x8b, "d2f": 0x8c,
			}[in.name]
			if !medida {
				p.falla(in, "a numeric conversion outside the measured grid")
				break
			}
			switch in.name {
			case "i2f", "f2i":
				p.conviertePalabra(in, op) // una palabra dentro y una fuera
			case "i2l", "i2d", "f2d", "l2d", "d2l":
				p.convierteEnAncho(in, op) // un registro suelto dentro de un par, o el par entero
			case "l2i", "l2f", "d2i", "d2f":
				p.convierteDeAncho(in, op)
			case "f2l":
				p.convierteEnAncho(in, op)
			}
		case "lreturn", "dreturn":
			r := p.pop(in)
			if p.err != nil {
				return p.err
			}
			if !p.nibble(r, in) {
				return p.err
			}
			p.palabra(unidad11x(opRetornaWide, r))
			p.sueltaPar(r)
		case "arraylength":
			p.longitudArreglo(in)
		case "athrow":
			r := p.pop(in)
			if !p.octeto(r, in) {
				return p.err
			}
			p.palabra(unidad11x(opLanza, r))
			p.enPila = p.enPila[:0]
		case "iinc":
			p.incrementa(in)
		case "newarray", "anewarray":
			p.nuevoArreglo(in)
		case "iaload", "aaload", "laload", "faload", "daload":
			p.leeArreglo(in)
		case "iastore", "aastore", "lastore", "fastore", "dastore":
			p.escribeArreglo(in)
		case "checkcast":
			desc := p.x.classDescriptor(dexU16(in.ops))
			r := p.pop(in)
			p.palabra(uint16(opCheckcastDex) | uint16(r&0xff)<<8)
			if !p.indice(p.typeIdxDe(desc, in), in, "the type_id of check-cast") {
				return p.err
			}
			p.push(r)
		case "instanceof":
			// medido en el parrafo 79 con `Ins2.f` dexada por `d8`: `4128 2` es
			// `instance-of v0, v1, type@2` —opcode 0x20, el hueco entre `check-cast` 0x1f (parrafo 50)
			// y `array-length` 0x21 (parrafo 66), en la forma 22c de siempre: `op|BA` con el destino en
			// el nibble bajo del byte alto—. El destino es un temporal: la referencia que se prueba
			// puede seguir viva despues de la condicion.
			desc := p.x.classDescriptor(dexU16(in.ops))
			objeto := p.pop(in)
			if !p.nibble(objeto, in) {
				return p.err
			}
			r := p.toma(in)
			if !p.nibble(r, in) {
				return p.err
			}
			p.palabra(unidad12x(opInstanceDex, r, objeto))
			if !p.indice(p.typeIdxDe(desc, in), in, "the type_id of instance-of") {
				return p.err
			}
			p.suelta(objeto)
			p.push(r)
		case "areturn":
			r := p.pop(in)
			p.palabra(unidad11x(opRetornaObjeto, r))
		case "ldc_w":
			p.ldc(in)
		case "iconst_2", "iconst_3", "iconst_4", "iconst_5", "iconst_m1", "bipush", "sipush":
			valores := map[string]int{"iconst_m1": -1, "iconst_2": 2, "iconst_3": 3, "iconst_4": 4, "iconst_5": 5}
			v, esConst := valores[in.name]
			if !esConst {
				if in.name == "bipush" {
					v = int(int8(in.ops[0]))
				} else {
					v = int(int16(dexU16(in.ops)))
				}
			}
			r := p.toma(in)
			if !p.entero(r, v, in) {
				return p.err
			}
			p.push(r)
		case "iload", "aload", "fload", "iload_0", "iload_1", "iload_2", "iload_3", "aload_2", "aload_3",
			"fload_0", "fload_1", "fload_2", "fload_3":
			n := 0
			switch in.name {
			case "iload", "aload", "fload":
				n = int(in.ops[0])
			default:
				n = int(in.name[len(in.name)-1]) - '0'
			}
			// la lectura del local: con el registro alto, `move*/from16` lo baja a un temporal
			// (parrafo 86); con v15 o menos se empuja el del local, sin instrucción
			if in.name[0] == 'a' {
				p.cargaLocal(in, n, opMueveObjeto)
			} else {
				p.cargaLocal(in, n, opMueve)
			}
		case "istore", "astore", "fstore", "istore_0", "istore_1", "istore_2", "istore_3", "astore_0", "astore_1", "astore_2", "astore_3",
			"fstore_0", "fstore_1", "fstore_2", "fstore_3":
			n := 0
			switch in.name {
			case "istore", "astore", "fstore":
				n = int(in.ops[0])
			default:
				n = int(in.name[len(in.name)-1]) - '0'
			}
			p.guardaLocal(in, n)
		case "ifeq", "ifne", "iflt", "ifge", "ifgt", "ifle", "ifnull", "ifnonnull":
			p.salto(ifContraCero[in.name], destinoDe(in), 1, in)
		case "if_icmpeq", "if_icmpne", "if_icmplt", "if_icmpge", "if_icmpgt", "if_icmple":
			p.salto(ifEntreDos[in.name], destinoDe(in), 2, in)
		case "goto":
			p.salto(opSalto, destinoDe(in), 0, in)
		case "tableswitch", "lookupswitch":
			p.saltoTabla(in)
		default:
			p.falla(in, "outside the measured subset")
		}
		if p.err != nil {
			return p.err
		}
	}
	if err := p.parchaSaltos(); err != nil {
		return err
	}
	return p.cierraCargas()
}

// cargaSwitch: el payload de un switch, que se escribe al final del método —`d8` lo deja detrás de
// todos los cuerpos— cuando ya se sabe en qué unidad cayó cada destino.
type cargaSwitch struct {
	desde     int      // unidad de la instrucción de switch, desde donde se cuenta el desplazamiento
	datos     []uint16 // ident, tamaño y claves (o first_key); los destinos van a cero hasta el parcheo
	destinos  []int    // pcs JVM de los destinos, en el orden de los pares
	enDestino []int    // unidad de `datos` donde va cada destino
}

// saltoTabla: `tableswitch`/`lookupswitch` del `.class` a `packed-switch`/`sparse-switch`. La clave
// sale de la pila JVM y queda en el registro del switch; el payload se encola y se escribe al final,
// y el defecto se emite como un `goto` detrás del switch porque en DEX el «ninguna clave casa» es el
// pase de lista y en el `.class` es un destino explícito.
func (p *traduce) saltoTabla(in jvmInstr) {
	r := p.pop(in)
	if !p.nibble(r, in) {
		return
	}
	be := func(b []byte) int32 {
		return int32(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]))
	}
	empaquetado := in.op == 0xaa
	c := cargaSwitch{desde: len(p.w)}
	op := byte(opCambioEmpaquetado)
	if !empaquetado {
		op = opCambioDisperso
	}
	p.palabra(uint16(op) | uint16(r&0xff)<<8)
	p.palabra(0) // desplazamiento al payload, que se conoce en cierraCargas
	p.palabra(0)
	defecto := in.pc + int(be(in.ops[0:4]))
	if empaquetado {
		bajo, alto := be(in.ops[4:8]), be(in.ops[8:12])
		n := int(alto-bajo) + 1
		if n <= 0 || 12+4*n > len(in.ops) {
			p.falla(in, "a tableswitch whose key range does not match its operands")
			return
		}
		c.datos = []uint16{identEmpaquetado, uint16(n), uint16(uint32(bajo)), uint16(uint32(bajo) >> 16)}
		for k := 0; k < n; k++ {
			c.destinos = append(c.destinos, in.pc+int(be(in.ops[12+4*k:])))
			c.enDestino = append(c.enDestino, len(c.datos))
			c.datos = append(c.datos, 0, 0)
		}
	} else {
		n := int(be(in.ops[4:8]))
		if n < 0 || 8+8*n > len(in.ops) {
			p.falla(in, "a lookupswitch whose pair count does not match its operands")
			return
		}
		// `sparse-switch-payload` lleva **todas las claves y después todos los destinos**, no pares
		// intercalados: medido en el volcado de `d8` del §64, 26 unidades = ident + tamaño + 6 claves +
		// 6 destinos. Intercalarlos hace que ART lea una clave como destino y corte con
		// `invalid target offset 1000`.
		c.datos = []uint16{identDisperso, uint16(n)}
		destinos := make([]int, 0, n)
		for k := 0; k < n; k++ {
			clave := be(in.ops[8+8*k:])
			c.datos = append(c.datos, uint16(uint32(clave)), uint16(uint32(clave)>>16))
			destinos = append(destinos, in.pc+int(be(in.ops[12+8*k:])))
		}
		for _, pc := range destinos {
			c.destinos = append(c.destinos, pc)
			c.enDestino = append(c.enDestino, len(c.datos))
			c.datos = append(c.datos, 0, 0)
		}
	}
	p.cargas = append(p.cargas, c)
	p.salto(opSalto, defecto, 0, in)
}

// cierraCargas escribe los payloads detrás del último cuerpo, alineados a cuatro bytes como los deja
// `d8`, y parchea con la distancia en unidades el operando de cada switch y cada destino.
func (p *traduce) cierraCargas() error {
	for _, c := range p.cargas {
		for len(p.w)%2 != 0 {
			p.w = append(p.w, 0)
		}
		inicio := len(p.w)
		datos := append([]uint16(nil), c.datos...)
		for k, pc := range c.destinos {
			unidad, ok := p.pcs[pc]
			if !ok {
				return &traduceError{pc: pc, nombre: p.m.Name,
					motivo: "a switch that points at a pc which is not the start of an instruction"}
			}
			rel := int32(unidad - c.desde)
			datos[c.enDestino[k]] = uint16(rel)
			datos[c.enDestino[k]+1] = uint16(uint32(rel) >> 16)
		}
		p.w = append(p.w, datos...)
		rel := int32(inicio - c.desde)
		p.w[c.desde+1] = uint16(rel)
		p.w[c.desde+2] = uint16(uint32(rel) >> 16)
	}
	return nil
}

func (p *traduce) ldc(in jvmInstr) {
	i := int(in.ops[0])
	if len(in.ops) == 2 {
		i = int(dexU16(in.ops))
	}
	r := p.toma(in)
	switch p.x.tagOf(i) {
	case cpString:
		s := p.x.stringValue(i)
		p.palabra(uint16(opConstString) | uint16(r&0xff)<<8)
		if !p.indice(p.t.stringIdx[s], in, "the string_id of ldc") {
			return
		}
	case cpInteger:
		if !p.entero(r, int(p.x.intOf(i)), in) {
			return
		}
	case cpFloat:
		// `ldc #25 // float 1.5f` del parrafo 77: en el dex son los bits de 32, no el numero
		p.palabraFlotante(r, float64(math.Float32frombits(uint32(p.x.intOf(i)))), in)
	default:
		p.falla(in, "an ldc of a pool type that is neither string nor int")
		return
	}
	p.push(r)
}

// entero: la misma elección de formato que d8 tomó sobre la sonda — const/4 para el valor chico
// (medido con el 1, 2, 3, 4 y 6 del <init>), const/16 para lo que cabe en dieciseis bits (medido
// con `const/16 v0, 12`) y const/high16 cuando los bits bajos son cero (medido con la id de
// recurso 0x7f010000 del onCreate). Lo que no cabe en ninguno de los tres se rechaza por nombre.
func (p *traduce) entero(r int, v int, in jvmInstr) bool {
	switch {
	case v >= -8 && v <= 7:
		if !p.nibble(r, in) {
			return false
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
		return false
	}
	return true
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
	case escribe && esAncho(desc):
		op = opIPutWide
	case !escribe && esAncho(desc):
		op = opIGetWide
	case escribe && esObjeto(desc):
		op = opIputObject
	case !escribe && esObjeto(desc):
		op = opIgetObject
	case !escribe && (desc[0] == 'I' || desc[0] == 'F'):
		op = opIget
	case escribe && (desc[0] == 'I' || desc[0] == 'F'):
		// medido en el <init> de la sonda (parrafo 50): seis putfields salen como iput 0x59,
		// `iput v0, v1, field@1`, con el valor en el primer registro y el objeto en el segundo. El
		// `float` comparte forma: vale una palabra, como `sget v2, field@1` que `d8` escribe para el
		// campo `ff` de `FyD.campoEstatico` (parrafo 80)
		op = opIput
	default:
		p.falla(in, "a field access of type "+string(desc[0])+": only int, float, object and wide fields were measured")
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
		// los dos murieron con el almacenamiento: sin soltarlos, un <init> con seis putfields
		// pide seis temporales y revienta contra la pila máxima que midió javap
		if esAncho(desc) {
			p.sueltaPar(valor)
		} else {
			p.suelta(valor)
		}
		p.suelta(objeto)
	} else {
		objeto := p.pop(in)
		var r int
		if esAncho(desc) {
			r = p.tomaPar(in)
		} else {
			r = p.toma(in)
		}
		if p.err != nil {
			return
		}
		if !p.nibble(objeto, in) || !p.nibble(r, in) {
			return
		}
		p.palabra(unidad12x(op, r, objeto))
		p.suelta(objeto)
		p.push(r)
	}
	p.indice(campo, in, "the field_id of the access")
}

func esObjeto(desc string) bool {
	return desc != "" && (desc[0] == 'L' || desc[0] == '[')
}

// metodoPrivado: la clase que se traduce declara ese metodo con ACC_PRIVATE. Es lo que distingue un
// `invokespecial` de un metodo privado (que d8 baja a `invoke-direct`) de una llamada a un constructor o
// a un padre, que la propia instruccion no separa.
func (p *traduce) metodoPrivado(nombre, desc string) bool {
	for _, m := range p.clase.Methods {
		if m.Name == nombre && m.Desc == desc {
			return m.Access&accPrivate != 0
		}
	}
	return false
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
		case owner == p.clase.Name && p.metodoPrivado(nombre, desc):
			// un metodo privado de la propia clase es invoke-direct (0x70), medido en el parrafo 72
			// sobre el access$100 de javac: `7010 0f00 0000` contra `Lp/Fuera;.oculto:()V`
			op = opInvocaDirecto
		default:
			p.falla(in, "an invokespecial that is neither a constructor, a super call nor a private method of this class, not measured here")
			return
		}
	}
	_, ret, params := splitDescriptor(desc)
	// La pila JVM tiene UNA entrada por valor, pero la lista del 35c cuenta PALABRAS: un argumento
	// ancho aporta los dos registros del par (medido: `println:(J)V` sale `6e20 ... 0300` con
	// {v3, v0, v1} y `imprime:(J)V` con {v0, v1}, parrafo 74).
	valores := make([]int, len(params))
	if !estatico {
		valores = append([]int{0}, valores...)
	}
	for i := len(valores) - 1; i >= 0; i-- {
		valores[i] = p.pop(in)
	}
	if p.err != nil {
		return
	}
	ancho := func(i int) bool {
		if estatico {
			return esAncho(params[i])
		}
		return i > 0 && esAncho(params[i-1])
	}
	regs := make([]int, 0, total)
	for i, v := range valores {
		regs = append(regs, v)
		if ancho(i) {
			regs = append(regs, v+1)
		}
	}
	for i, v := range valores {
		if ancho(i) {
			p.sueltaPar(v)
			continue
		}
		p.suelta(v)
	}
	if total > p.outs {
		p.outs = total
	}
	p.invoca(op, regs, metodo, in)
	if p.err != nil {
		return
	}
	switch {
	case ret == "V" || ret == "":
	case seTira:
		// el resultado se tira justo despues: ni move-result ni descarga, que es lo que hizo d8
		// con el MenuItem del listener (§21). Tampoco hace falta saber el tipo devuelto
	case esAncho(ret):
		// `move-result-wide v0` es `0b00` (parrafo 74)
		r := p.tomaPar(in)
		if p.err != nil {
			return
		}
		if !p.nibble(r, in) {
			return
		}
		p.palabra(unidad11x(opMueveResultWide, r))
		p.push(r)
	case esObjeto(ret):
		r := p.toma(in)
		p.palabra(unidad11x(opMueveResultObj, r))
		p.push(r)
	case ret[0] == 'I' || ret[0] == 'B' || ret[0] == 'C' || ret[0] == 'S' || ret[0] == 'Z' || ret[0] == 'F':
		r := p.toma(in)
		if !p.nibble(r, in) {
			return
		}
		p.palabra(unidad11x(opMueveResult, r))
		p.push(r)
	default:
		p.falla(in, "a call returning "+string(ret[0])+": move-result-wide (0x0b) is not measured in this fork")
	}
}

// codeItemDe traduce el cuerpo de un método y devuelve el code_item con las palabras que este
// traductor emite, más lo que el §20 obliga a poner en la cabecera: ins_size son los huecos que
// toman los argumentos, outs_size la llamada más ancha y registers_size temporales más locales.
func (t *dexTables) codeItemDe(clase classFull, m classMethod) (codeItem, error) {
	ci, _, err := t.codeItemConPc(clase, m)
	return ci, err
}

// codeItemConPc es codeItemDe devolviendo ademas la correspondencia pc JVM -> unidad dalvik: el
// debug_info_item del §56 la necesita, porque las lineas del `.class` estan indexadas por pc JVM y
// el dex las nombra por unidad propia.
func (t *dexTables) codeItemConPc(clase classFull, m classMethod) (codeItem, map[int]int, error) {
	if !m.HasCode {
		return codeItem{}, nil, nil
	}
	x := poolIndex{p: clase.p}
	// La conjetura de la pasada de medida es la pila máxima que midió javap más el número de huecos,
	// porque desde el párrafo 50 un temporal puede quedar *atado* a un local (`int v = ...`) y esos
	// registros ya no son reutilizables. Con los valores anchos la conjetura se queda corta a veces:
	// un `double` vivo ocupa DOS registros y la pila JVM los cuenta como dos palabras, pero dos
	// temporales muertos sueltos no forman un par (`tomaPar` pide consecutivos), así que el pico
	// medido puede pasar de la base. La medida no es un rechazo —solo la pasada que escribe las
	// palabras tiene techo—; si esa pasada pide más que el pico (visto con `Ver.compara` después del
	// §81), se sube la base a lo que pidió y se repite **sola ella**: volver a medir rebotaba entre
	// dos picos. Cada vuelta sube al menos un registro y los formatos llegan a 255, así que el bucle
	// termina.
	prueba := &traduce{t: t, x: x, clase: clase, m: m, locales: m.LocalCount, base: m.MaxStack + m.LocalCount + 1}
	if err := prueba.recorre(); err != nil {
		return codeItem{}, nil, err
	}
	base := prueba.pico
	var final *traduce
	for {
		final = &traduce{t: t, x: x, clase: clase, m: m, locales: m.LocalCount, base: base, techo: true}
		err := final.recorre()
		if err == nil {
			break
		}
		if !esPresupuesto(err) {
			return codeItem{}, nil, err
		}
		if final.pico > 0xff {
			return codeItem{}, nil, &traduceError{nombre: m.Name,
				motivo: "the register plan does not fit the eight-bit registers of the three-register form"}
		}
		base = final.pico
	}
	tries, handlers, err := armaTries(final, m)
	if err != nil {
		return codeItem{}, nil, err
	}
	return codeItem{
		registersSize: uint32(final.base + final.locales),
		insSize:       uint32(m.ArgCount),
		outsSize:      uint32(final.outs),
		triesSize:     uint32(len(tries)),
		debugInfoOff:  0,
		insnsSize:     uint32(len(final.w)),
		insns:         final.w,
		tries:         tries,
		handlers:      handlers,
	}, final.pcs, nil
}

// armaTries traduce la exception_table del `.class` a los try_item y la lista de handlers del dex.
// Las entradas que comparten rango se agrupan en un try_item (una por tipo atrapado), el fin del
// `.class` es **exclusivo** y en el dex es start+insn_count, y sin tipo (un `finally`) la lista lleva
// el handler de "cualquiera" con el tamaño en negativo (JVMS 4.7.3 / DEX `encoded_catch_handler`).
func armaTries(p *traduce, m classMethod) ([]tryItem, []byte, error) {
	if len(m.Tries) == 0 {
		return nil, nil, nil
	}
	type grupo struct {
		start, end int
		tipos      []classTry
	}
	var grupos []grupo
	indice := map[[2]int]int{}
	for _, tr := range m.Tries {
		k := [2]int{tr.start, tr.end}
		if i, ok := indice[k]; ok {
			grupos[i].tipos = append(grupos[i].tipos, tr)
			continue
		}
		indice[k] = len(grupos)
		grupos = append(grupos, grupo{start: tr.start, end: tr.end, tipos: []classTry{tr}})
	}
	lista := putUleb(nil, uint32(len(grupos)))
	items := make([]tryItem, 0, len(grupos))
	for _, g := range grupos {
		inicio, ok := p.pcs[g.start]
		if !ok {
			return nil, nil, &traduceError{nombre: m.Name,
				motivo: "a try that starts at a pc which is not an instruction"}
		}
		fin, ok := p.pcs[g.end]
		if !ok {
			return nil, nil, &traduceError{nombre: m.Name,
				motivo: "a try that ends at a pc which is not an instruction"}
		}
		off := len(lista)
		n := 0
		catchAll := -1
		for _, tr := range g.tipos {
			if tr.catchType == "" {
				catchAll = tr.handler
				continue
			}
			n++
		}
		if catchAll >= 0 {
			lista = putSleb(lista, int32(-n))
		} else {
			lista = putSleb(lista, int32(n))
		}
		for _, tr := range g.tipos {
			if tr.catchType == "" {
				continue
			}
			unidad, ok := p.pcs[tr.handler]
			if !ok {
				return nil, nil, &traduceError{nombre: m.Name,
					motivo: "a handler at a pc which is not an instruction"}
			}
			lista = putUleb(lista, uint32(p.t.typeIdx[tr.catchType]))
			lista = putUleb(lista, uint32(unidad))
		}
		if catchAll >= 0 {
			unidad, ok := p.pcs[catchAll]
			if !ok {
				return nil, nil, &traduceError{nombre: m.Name,
					motivo: "a catch-all handler at a pc which is not an instruction"}
			}
			lista = putUleb(lista, uint32(unidad))
		}
		items = append(items, tryItem{start: uint32(inicio), count: uint16(fin - inicio), handlerOff: uint16(off)})
	}
	return items, lista, nil
}
