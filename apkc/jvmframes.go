package main

// El StackMapTable (JVMS 4.7.4), medido con `javap -v` sobre dos sondas (parrafo 65) y con la JVM
// como oraculo.
//
// Lo que javac escribe, leido de sus ficheros y no de un manual:
//
//   - Un cuerpo recto no lleva el atributo. Un `if` sin `else` lleva UN marco, el del destino.
//   - Hay marco en cada destino de salto y en la instruccion que sigue a un salto incondicional o a
//     un `return`; o sea, en cada comienzo de bloque basico salvo el primero, que es el implicito.
//   - Un destino puede ser el pc 0 (un bucle con la cabecera al principio): javac escribe entonces
//     un marco con `offset_delta 0`, y el hueco del local que solo existe dentro del cuerpo sale
//     `top`. La JVM lo acepta: comprobado con `java -Xverify:all` sobre `Cab`.
//   - `top` gana en la union: entrar con un local sin asignar y volver con el asignado deja `top`.
//
// Este emisor escribe sus marcos como `full_frame`, que el JVMS permite en cualquier posicion; no
// intenta reproducir la compresion de javac (`same`/`append`) porque el oraculo que importa es que
// la JVM verifique, y una forma sola es menos superficie para equivocarse. Lo que si reproduce es
// QUE posiciones llevan marco y QUE tipos tienen en cada una, que es lo que el verificador compara.

import (
	"fmt"
	"sort"
	"strings"
)

// tipoVerif es un verification_type_info: entero, largo, referencia (con su nombre interno; vacio es el
// tipo null), la SEGUNDA MITAD de un valor ancho o top (un hueco sin asignar en alguna rama).
type tipoVerif struct {
	etiqueta byte // 'i' entero, 'j' largo, 's' segunda mitad, 'l' referencia, 't' top
	clase    string
}

var (
	verifTop   = tipoVerif{etiqueta: 't'}
	verifInt   = tipoVerif{etiqueta: 'i'}
	verifNulo  = tipoVerif{etiqueta: 'l'}
	verifLargo = tipoVerif{etiqueta: 'j'}
	// el `float` vale una palabra y el `double` dos, medido con `javap -c -v` sobre `FyD` (parrafo 77):
	// `por(float a, double b)` tiene `a` en el hueco 0 y `b` en el 1-2.
	verifFlotante = tipoVerif{etiqueta: 'f'}
	verifDoble    = tipoVerif{etiqueta: 'd'}
	verifSegundo  = tipoVerif{etiqueta: 's'}
)

func verifRef(clase string) tipoVerif { return tipoVerif{etiqueta: 'l', clase: clase} }

func (t tipoVerif) String() string {
	switch t.etiqueta {
	case 'i':
		return "entero"
	case 'j':
		return "largo"
	case 'f':
		return "flotante"
	case 'd':
		return "doble"
	case 's':
		return "segunda mitad de un ancho"
	case 'l':
		if t.clase == "" {
			return "null"
		}
		return "referencia a " + t.clase
	}
	return "top"
}

// ----- leer el pool que este emisor acaba de escribir -----

// lectorPool resuelve los indices que el codigo ya lleva escritos. Sale del propio poolBuilder, cuyas
// entradas guardan sus bytes tal cual: no hay una segunda copia de los tipos que pueda desincronizarse.
type lectorPool struct{ entradas [][]byte }

func (p *poolBuilder) lector() *lectorPool { return &lectorPool{entradas: p.entradas} }

func (l *lectorPool) entrada(i int) ([]byte, error) {
	if i <= 0 || i > len(l.entradas) {
		return nil, fmt.Errorf("el indice %d no esta en el pool", i)
	}
	return l.entradas[i-1], nil
}

func (l *lectorPool) utf8(i int) (string, error) {
	e, err := l.entrada(i)
	if err != nil {
		return "", err
	}
	if len(e) < 3 || e[0] != 1 {
		return "", fmt.Errorf("la entrada %d del pool no es un Utf8", i)
	}
	n := int(e[1])<<8 | int(e[2])
	if len(e) < 3+n {
		return "", fmt.Errorf("el Utf8 %d se corta", i)
	}
	return string(e[3 : 3+n]), nil
}

func u2De(b []byte, i int) int { return int(b[i])<<8 | int(b[i+1]) }

func (l *lectorPool) claseDe(i int) (string, error) {
	e, err := l.entrada(i)
	if err != nil {
		return "", err
	}
	if len(e) < 3 || e[0] != 7 {
		return "", fmt.Errorf("la entrada %d del pool no es una Class", i)
	}
	return l.utf8(u2De(e, 1))
}

// refDe devuelve la clase y el descriptor de un campo o metodo: Fieldref (9), Methodref (10) e
// InterfaceMethodref (11) comparten forma, clase + NameAndType, y el descriptor esta en el segundo.
func (l *lectorPool) refDe(i int) (string, string, error) {
	e, err := l.entrada(i)
	if err != nil {
		return "", "", err
	}
	if len(e) < 5 || (e[0] != 9 && e[0] != 10 && e[0] != 11) {
		return "", "", fmt.Errorf("la entrada %d del pool no es una referencia", i)
	}
	clase, err := l.claseDe(u2De(e, 1))
	if err != nil {
		return "", "", err
	}
	nt, err := l.entrada(u2De(e, 3))
	if err != nil {
		return "", "", err
	}
	if len(nt) < 5 || nt[0] != 12 {
		return "", "", fmt.Errorf("la entrada %d del pool no es un NameAndType", u2De(e, 3))
	}
	desc, err := l.utf8(u2De(nt, 3))
	if err != nil {
		return "", "", err
	}
	return clase, desc, nil
}

// tipoDeConstante: lo que deja un `ldc`. En el subconjunto medido solo hay enteros y cadenas.
func (l *lectorPool) tipoDeConstante(i int) (tipoVerif, error) {
	e, err := l.entrada(i)
	if err != nil {
		return tipoVerif{}, err
	}
	if len(e) == 0 {
		return tipoVerif{}, fmt.Errorf("la entrada %d del pool esta vacia", i)
	}
	switch e[0] {
	case 3: // Integer
		return verifInt, nil
	case 8: // String
		return verifRef("java/lang/String"), nil
	case 7: // Class
		return verifRef("java/lang/Class"), nil
	case 5: // Long (`ldc2_w`, parrafo 74)
		return verifLargo, nil
	case 4: // Float (`ldc`, parrafo 77)
		return verifFlotante, nil
	case 6: // Double (`ldc2_w`, parrafo 77)
		return verifDoble, nil
	}
	return tipoVerif{}, fmt.Errorf("un `ldc` de la etiqueta %d del pool no esta medido", e[0])
}

// ----- descriptores -----

func tipoDeDesc(desc string) (tipoVerif, error) {
	if desc == "" {
		return tipoVerif{}, fmt.Errorf("un descriptor vacio")
	}
	switch desc[0] {
	case 'I', 'Z', 'B', 'C', 'S':
		return verifInt, nil
	case 'L':
		if len(desc) < 3 || desc[len(desc)-1] != ';' {
			return tipoVerif{}, fmt.Errorf("el descriptor %q no cierra", desc)
		}
		return verifRef(desc[1 : len(desc)-1]), nil
	case '[':
		// un arreglo es una referencia cuyo tipo de verificacion se nombra con su propio descriptor:
		// medido en `Arreglos.copia` (parrafo 66), donde javac escribe `locals = [ class "[I", int ]`.
		return verifRef(desc), nil
	case 'J':
		// el largo ocupa DOS huecos: medido en `javap -v` de la sonda `F`, donde el append de javac
		// escribe `locals = [ long ]` para un local que va del hueco 3 al 4 (parrafo 74)
		return verifLargo, nil
	case 'F':
		return verifFlotante, nil
	case 'D':
		// el doble, igual que el largo, se escribe con **una** entrada del marco para dos huecos
		// (parrafo 77, medido en `javap -v -c` de `FyD.local`)
		return verifDoble, nil
	}
	return tipoVerif{}, fmt.Errorf("el tipo %q de un descriptor no esta medido", desc)
}

// tiposDeArgs parte el trozo entre parentesis de un descriptor de metodo.
func tiposDeArgs(desc string) ([]tipoVerif, error) {
	if desc == "" || desc[0] != '(' {
		return nil, fmt.Errorf("el descriptor %q no empieza por parentesis", desc)
	}
	var out []tipoVerif
	i := 1
	for i < len(desc) && desc[i] != ')' {
		ini := i
		for i < len(desc) && desc[i] == '[' {
			i++
		}
		if i >= len(desc) {
			return nil, fmt.Errorf("el descriptor %q se corta", desc)
		}
		if desc[i] == 'L' {
			fin := strings.IndexByte(desc[i:], ';')
			if fin < 0 {
				return nil, fmt.Errorf("el descriptor %q tiene una L sin cerrar", desc)
			}
			i += fin + 1
		} else {
			i++
		}
		t, err := tipoDeDesc(desc[ini:i])
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if i >= len(desc) {
		return nil, fmt.Errorf("el descriptor %q no cierra el parentesis", desc)
	}
	return out, nil
}

// ----- el estado de una posicion y su union -----

type estadoFrame struct {
	locales []tipoVerif
	pila    []tipoVerif
}

func copiaEstado(e estadoFrame) estadoFrame {
	return estadoFrame{
		locales: append([]tipoVerif(nil), e.locales...),
		pila:    append([]tipoVerif(nil), e.pila...),
	}
}

// unirTipos: dos ramas que llegan al mismo punto. `top` gana porque es el hueco que alguna rama no
// asigno (medido en `Cab`: la vuelta del bucle trae un entero al hueco que en la entrada es top, y
// javac escribe top). Dos referencias distintas se unen en Object, que es el padre comun que el
// verificador acepta sin mirar la jerarquia.
func unirTipos(a, b tipoVerif) (tipoVerif, error) {
	if a == b {
		return a, nil
	}
	if a.etiqueta == 't' || b.etiqueta == 't' {
		return verifTop, nil
	}
	if a.etiqueta == 'l' && b.etiqueta == 'l' {
		if a.clase == "" {
			return b, nil
		}
		if b.clase == "" {
			return a, nil
		}
		return verifRef("java/lang/Object"), nil
	}
	return tipoVerif{}, fmt.Errorf("se unen %s y %s, y eso no lo produce el subconjunto medido", a, b)
}

// fusiona mete `nuevo` en el estado de `pc` y dice si cambio. Un stack distinto de alto en el mismo
// punto es un defecto del emisor, no del fuente, y se nombra.
func fusiona(estados map[int]estadoFrame, pc int, nuevo estadoFrame) (bool, error) {
	viejo, hay := estados[pc]
	if !hay {
		estados[pc] = copiaEstado(nuevo)
		return true, nil
	}
	if len(viejo.pila) != len(nuevo.pila) {
		return false, fmt.Errorf("dos caminos llegan con la pila de %d y %d valores", len(viejo.pila), len(nuevo.pila))
	}
	loc := append([]tipoVerif(nil), viejo.locales...)
	pila := append([]tipoVerif(nil), viejo.pila...)
	cambio := false
	for i := range pila {
		t, err := unirTipos(viejo.pila[i], nuevo.pila[i])
		if err != nil {
			return false, err
		}
		if t != pila[i] {
			pila[i] = t
			cambio = true
		}
	}
	for i := range loc {
		var otra tipoVerif = verifTop
		if i < len(nuevo.locales) {
			otra = nuevo.locales[i]
		}
		t, err := unirTipos(viejo.locales[i], otra)
		if err != nil {
			return false, err
		}
		if t != loc[i] {
			loc[i] = t
			cambio = true
		}
	}
	if cambio {
		estados[pc] = estadoFrame{locales: loc, pila: pila}
	}
	return cambio, nil
}

// ----- el interprete abstracto -----

type analisis struct {
	lector *lectorPool
	est    estadoFrame
}

func (a *analisis) empuja(t tipoVerif) { a.est.pila = append(a.est.pila, t) }

// altoPilas: la profundidad en PALABRAS, que es la unidad de `max_stack` para el verificador: un
// `long` vale dos. Medido al ejecutar la sonda `L`: con `max_stack=1` y un `ldc2_w` la JVM corta con
// `Operand stack overflow … Exceeded max stack size`, y javac escribe `stack=2` (parrafo 74).
func altoPilas(pila []tipoVerif) int {
	n := 0
	for _, t := range pila {
		if t.etiqueta == 'j' || t.etiqueta == 'd' {
			n += 2
			continue
		}
		n++
	}
	return n
}

func (a *analisis) saca() (tipoVerif, error) {
	if len(a.est.pila) == 0 {
		return tipoVerif{}, fmt.Errorf("la pila esta vacia")
	}
	t := a.est.pila[len(a.est.pila)-1]
	a.est.pila = a.est.pila[:len(a.est.pila)-1]
	return t, nil
}

func (a *analisis) sacaEntero() error {
	t, err := a.saca()
	if err != nil {
		return err
	}
	if t.etiqueta != 'i' {
		return fmt.Errorf("se esperaba un entero y hay %s", t)
	}
	return nil
}

// sacaLargo: lo que deja un `ladd`, un `ldiv` o un `lreturn` en la pila (parrafo 74).
func (a *analisis) sacaLargo() error {
	t, err := a.saca()
	if err != nil {
		return err
	}
	if t.etiqueta != 'j' {
		return fmt.Errorf("se esperaba un largo y hay %s", t)
	}
	return nil
}

func (a *analisis) sacaRef() (tipoVerif, error) {
	t, err := a.saca()
	if err != nil {
		return tipoVerif{}, err
	}
	if t.etiqueta != 'l' {
		return tipoVerif{}, fmt.Errorf("se esperaba una referencia y hay %s", t)
	}
	return t, nil
}

func (a *analisis) cargaLocal(idx int, que byte) error {
	if idx < 0 || idx >= len(a.est.locales) {
		return fmt.Errorf("se lee el hueco %d y el marco tiene %d", idx, len(a.est.locales))
	}
	t := a.est.locales[idx]
	if t.etiqueta == 't' {
		return fmt.Errorf("se lee el hueco %d sin asignar en algun camino", idx)
	}
	if t.etiqueta != que {
		return fmt.Errorf("el hueco %d tiene %s y se carga como %s", idx, t, nombreTipoVerif(que))
	}
	a.empuja(t)
	return nil
}

func (a *analisis) guardaLocal(idx int, que byte) error {
	if idx < 0 || idx >= len(a.est.locales) {
		return fmt.Errorf("se escribe el hueco %d y el marco tiene %d", idx, len(a.est.locales))
	}
	t, err := a.saca()
	if err != nil {
		return err
	}
	if t.etiqueta != que {
		return fmt.Errorf("se guarda %s en el hueco %d y se esperaba %s", t, idx, nombreTipoVerif(que))
	}
	a.est.locales[idx] = t
	if t.etiqueta == 'j' || t.etiqueta == 'd' {
		// el largo y el doble ocupan tambien el hueco siguiente, que el verificador no nombra: medido
		// en la sonda `F`, donde `long x` vive en los huecos 3 y 4 (parrafo 74), y en `FyD.por`, donde
		// el `double b` vive en el 1 y el 2 (parrafo 77)
		if idx+1 >= len(a.est.locales) {
			return fmt.Errorf("el largo del hueco %d se sale del marco de %d huecos", idx, len(a.est.locales))
		}
		a.est.locales[idx+1] = verifSegundo
	}
	return nil
}

// sacaDe: lo que exige una cuenta, una conversion o un retorno del tipo que se nombre ('f' flotante o
// 'd' doble, parrafo 77).
func (a *analisis) sacaDe(que byte) error {
	t, err := a.saca()
	if err != nil {
		return err
	}
	if t.etiqueta != que {
		return fmt.Errorf("se esperaba %s y hay %s", nombreTipoVerif(que), t)
	}
	return nil
}

// nombreTipoVerif: como se dice la etiqueta en los mensajes.
func nombreTipoVerif(que byte) string {
	switch que {
	case 'i':
		return "un entero"
	case 'j':
		return "un largo"
	case 'f':
		return "un flotante"
	case 'd':
		return "un doble"
	case 'l':
		return "una referencia"
	}
	return "top"
}

func salto16(in jvmInstr) int { return in.pc + int(int16(uint16(in.ops[0])<<8|uint16(in.ops[1]))) }

func leer4(b []byte, i int) int32 {
	return int32(uint32(b[i])<<24 | uint32(b[i+1])<<16 | uint32(b[i+2])<<8 | uint32(b[i+3]))
}

// destinosDeSwitch: el `default` es el primer u4 y despues van los destinos; en el `tableswitch`
// uno por clave del rango, y en el `lookupswitch` el segundo de cada par (clave, destino).
func destinosDeSwitch(in jvmInstr) ([]int, error) {
	ops := in.ops
	if in.op == 0xaa {
		if len(ops) < 12 {
			return nil, fmt.Errorf("el tableswitch se corta")
		}
		def := leer4(ops, 0)
		bajo := leer4(ops, 4)
		alto := leer4(ops, 8)
		n := int(alto-bajo) + 1
		if n < 0 || len(ops) < 12+4*n {
			return nil, fmt.Errorf("el tableswitch declara %d destinos y no los lleva", n)
		}
		out := []int{in.pc + int(def)}
		for i := 0; i < n; i++ {
			out = append(out, in.pc+int(leer4(ops, 12+4*i)))
		}
		return out, nil
	}
	if len(ops) < 8 {
		return nil, fmt.Errorf("el lookupswitch se corta")
	}
	n := int(leer4(ops, 4))
	if n < 0 || len(ops) < 8+8*n {
		return nil, fmt.Errorf("el lookupswitch declara %d pares y no los lleva", n)
	}
	out := []int{in.pc + int(leer4(ops, 0))}
	for i := 0; i < n; i++ {
		out = append(out, in.pc+int(leer4(ops, 12+8*i)))
	}
	return out, nil
}

func esCondicional(op byte) bool {
	return (op >= 0x99 && op <= 0xa4) || op == 0xc6 || op == 0xc7
}

func tieneCaida(op byte) bool {
	return !(op == 0xa7 || op == 0xaa || op == 0xab || op == 0xac || op == 0xb0 || op == 0xb1)
}

// aplica simula una instruccion sobre el estado. Devuelve los destinos a los que hay que propagar y
// si el control sigue por la instruccion siguiente.
func (a *analisis) aplica(in jvmInstr) ([]int, bool, error) {
	op := in.op
	switch {
	case op == 0x01:
		a.empuja(verifNulo)
	case op >= 0x02 && op <= 0x08:
		a.empuja(verifInt)
	case op == 0x10 || op == 0x11:
		a.empuja(verifInt)
	case op == 0x12 || op == 0x13:
		idx := int(in.ops[0])
		if op == 0x13 {
			idx = u2De(in.ops, 0)
		}
		t, err := a.lector.tipoDeConstante(idx)
		if err != nil {
			return nil, false, err
		}
		a.empuja(t)
	case op == 0x15:
		if err := a.cargaLocal(int(in.ops[0]), 'i'); err != nil {
			return nil, false, err
		}
	case op >= 0x1a && op <= 0x1d:
		if err := a.cargaLocal(int(op-0x1a), 'i'); err != nil {
			return nil, false, err
		}
	case op == 0x19:
		if err := a.cargaLocal(int(in.ops[0]), 'l'); err != nil {
			return nil, false, err
		}
	case op >= 0x2a && op <= 0x2d:
		if err := a.cargaLocal(int(op-0x2a), 'l'); err != nil {
			return nil, false, err
		}
	case op == 0x36:
		if err := a.guardaLocal(int(in.ops[0]), 'i'); err != nil {
			return nil, false, err
		}
	case op >= 0x3b && op <= 0x3e:
		if err := a.guardaLocal(int(op-0x3b), 'i'); err != nil {
			return nil, false, err
		}
	case op == 0x3a:
		if err := a.guardaLocal(int(in.ops[0]), 'l'); err != nil {
			return nil, false, err
		}
	case op >= 0x4b && op <= 0x4e:
		if err := a.guardaLocal(int(op-0x4b), 'l'); err != nil {
			return nil, false, err
		}
	case op == 0x84: // iinc: no toca la pila, solo el local
		if len(in.ops) < 2 {
			return nil, false, fmt.Errorf("el iinc se corta")
		}
		n := int(in.ops[0])
		if n >= len(a.est.locales) || a.est.locales[n].etiqueta != 'i' {
			return nil, false, fmt.Errorf("el iinc sobre el hueco %d, que no es un entero asignado", n)
		}
	case op == 0x57:
		if _, err := a.saca(); err != nil {
			return nil, false, err
		}
	case op == 0x59:
		if len(a.est.pila) < 1 {
			return nil, false, fmt.Errorf("el `dup` con la pila vacia")
		}
		a.empuja(a.est.pila[len(a.est.pila)-1])
	case op == 0x09 || op == 0x0a: // lconst_0, lconst_1 (medidos en `W.cero` y `F.conComparacion`)
		a.empuja(verifLargo)
	case op == 0x0b || op == 0x0c || op == 0x0d: // fconst_0..2 (`FyD.constanteCorta`, parrafo 77)
		a.empuja(verifFlotante)
	case op == 0x0e || op == 0x0f: // dconst_0, dconst_1 (`FyD.deEntero`: el `1.0` es dconst_1)
		a.empuja(verifDoble)
	case op == 0x14: // ldc2_w: la constante larga o doble del pool
		t, err := a.lector.tipoDeConstante(u2De(in.ops, 0))
		if err != nil {
			return nil, false, err
		}
		if t.etiqueta != 'j' && t.etiqueta != 'd' {
			return nil, false, fmt.Errorf("el ldc2_w del indice %d no trae un largo ni un doble", u2De(in.ops, 0))
		}
		a.empuja(t)
	case op == 0x16: // lload
		if err := a.cargaLocal(int(in.ops[0]), 'j'); err != nil {
			return nil, false, err
		}
	case op >= 0x1e && op <= 0x21: // lload_0 .. lload_3
		if err := a.cargaLocal(int(op-0x1e), 'j'); err != nil {
			return nil, false, err
		}
	case op == 0x37: // lstore
		if err := a.guardaLocal(int(in.ops[0]), 'j'); err != nil {
			return nil, false, err
		}
	case op >= 0x3f && op <= 0x42: // lstore_0 .. lstore_3
		if err := a.guardaLocal(int(op-0x3f), 'j'); err != nil {
			return nil, false, err
		}
	case op == 0x61 || op == 0x65 || op == 0x69 || op == 0x6d || op == 0x71: // ladd, lsub, lmul, ldiv, lrem
		if err := a.sacaLargo(); err != nil {
			return nil, false, err
		}
		if err := a.sacaLargo(); err != nil {
			return nil, false, err
		}
		a.empuja(verifLargo)
	case op == 0x85: // i2l
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		a.empuja(verifLargo)
	case op == 0x88: // l2i
		if err := a.sacaLargo(); err != nil {
			return nil, false, err
		}
		a.empuja(verifInt)
	// ----- el `float` y el `double`, leidos de `javap -c` sobre `FyD` (parrafo 77) -----
	case op == 0x17: // fload
		if err := a.cargaLocal(int(in.ops[0]), 'f'); err != nil {
			return nil, false, err
		}
	case op == 0x18: // dload
		if err := a.cargaLocal(int(in.ops[0]), 'd'); err != nil {
			return nil, false, err
		}
	case op >= 0x22 && op <= 0x25: // fload_0 .. fload_3
		if err := a.cargaLocal(int(op-0x22), 'f'); err != nil {
			return nil, false, err
		}
	case op >= 0x26 && op <= 0x29: // dload_0 .. dload_3
		if err := a.cargaLocal(int(op-0x26), 'd'); err != nil {
			return nil, false, err
		}
	case op == 0x38: // fstore
		if err := a.guardaLocal(int(in.ops[0]), 'f'); err != nil {
			return nil, false, err
		}
	case op == 0x39: // dstore
		if err := a.guardaLocal(int(in.ops[0]), 'd'); err != nil {
			return nil, false, err
		}
	case op >= 0x43 && op <= 0x46: // fstore_0 .. fstore_3
		if err := a.guardaLocal(int(op-0x43), 'f'); err != nil {
			return nil, false, err
		}
	case op >= 0x47 && op <= 0x4a: // dstore_0 .. dstore_3
		if err := a.guardaLocal(int(op-0x47), 'd'); err != nil {
			return nil, false, err
		}
	case op == 0x62 || op == 0x66 || op == 0x6a || op == 0x6e || op == 0x72: // fadd, fsub, fmul, fdiv, frem
		if err := a.sacaDe('f'); err != nil {
			return nil, false, err
		}
		if err := a.sacaDe('f'); err != nil {
			return nil, false, err
		}
		a.empuja(verifFlotante)
	case op == 0x63 || op == 0x67 || op == 0x6b || op == 0x6f || op == 0x73: // dadd, dsub, dmul, ddiv, drem
		if err := a.sacaDe('d'); err != nil {
			return nil, false, err
		}
		if err := a.sacaDe('d'); err != nil {
			return nil, false, err
		}
		a.empuja(verifDoble)
	case op == 0x86: // i2f
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		a.empuja(verifFlotante)
	case op == 0x87: // i2d (`FyD.deEntero`)
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		a.empuja(verifDoble)
	case op == 0x89 || op == 0x8a: // l2f, l2d
		if err := a.sacaLargo(); err != nil {
			return nil, false, err
		}
		if op == 0x89 {
			a.empuja(verifFlotante)
		} else {
			a.empuja(verifDoble)
		}
	case op == 0x8b: // f2i
		if err := a.sacaDe('f'); err != nil {
			return nil, false, err
		}
		a.empuja(verifInt)
	case op == 0x8c: // f2l
		if err := a.sacaDe('f'); err != nil {
			return nil, false, err
		}
		a.empuja(verifLargo)
	case op == 0x8d: // f2d (`FyD.por` y `FyD.aDoble`)
		if err := a.sacaDe('f'); err != nil {
			return nil, false, err
		}
		a.empuja(verifDoble)
	case op == 0x8e: // d2i (`FyD.aEntero`)
		if err := a.sacaDe('d'); err != nil {
			return nil, false, err
		}
		a.empuja(verifInt)
	case op == 0x8f: // d2l
		if err := a.sacaDe('d'); err != nil {
			return nil, false, err
		}
		a.empuja(verifLargo)
	case op == 0x90: // d2f (`FyD.aFlotante`)
		if err := a.sacaDe('d'); err != nil {
			return nil, false, err
		}
		a.empuja(verifFlotante)
	case op == 0xae: // freturn
		if err := a.sacaDe('f'); err != nil {
			return nil, false, err
		}
	case op == 0xaf: // dreturn
		if err := a.sacaDe('d'); err != nil {
			return nil, false, err
		}
	case op == 0xad: // lreturn
		if err := a.sacaLargo(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	case op == 0x5a: // dup_x1: [objeto, valor] -> [valor, objeto, valor], medido en el access$002 (parrafo 72)
		if len(a.est.pila) < 2 {
			return nil, false, fmt.Errorf("el `dup_x1` con menos de dos valores")
		}
		valor := a.est.pila[len(a.est.pila)-1]
		objeto := a.est.pila[len(a.est.pila)-2]
		a.est.pila = append(a.est.pila[:len(a.est.pila)-2], valor, objeto, valor)
	case op == 0x60 || op == 0x64 || op == 0x68 || op == 0x6c || op == 0x70:
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		a.empuja(verifInt)
	case op == 0xac:
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	case op == 0x2e: // iaload
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
		a.empuja(verifInt)
	case op == 0x32: // aaload
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		arreglo, err := a.sacaRef()
		if err != nil {
			return nil, false, err
		}
		elem := verifRef("java/lang/Object")
		if len(arreglo.clase) > 1 && arreglo.clase[0] == '[' {
			elem = verifRef(arreglo.clase[1:])
		}
		a.empuja(elem)
	case op == 0x4f: // iastore
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
	case op == 0x53: // aastore
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
	case op == 0xbe: // arraylength
		arreglo, err := a.sacaRef()
		if err != nil {
			return nil, false, err
		}
		if len(arreglo.clase) == 0 || arreglo.clase[0] != '[' {
			return nil, false, fmt.Errorf("el `arraylength` sobre %s, que no es un arreglo", arreglo)
		}
		a.empuja(verifInt)
	case op == 0xbc: // newarray
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		desc, ok := tiposNewArray[in.ops[0]]
		if !ok {
			return nil, false, fmt.Errorf("el atype %d de newarray no esta medido", in.ops[0])
		}
		a.empuja(verifRef(desc))
	case op == 0xbd: // anewarray
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		clase, err := a.lector.claseDe(u2De(in.ops, 0))
		if err != nil {
			return nil, false, err
		}
		a.empuja(verifRef("[" + clase))
	case op == 0xb0:
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	case op == 0xb1:
		return nil, false, nil
	case op == 0xbf: // athrow: se come la referencia y el bloque no sigue
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	case op >= 0xb2 && op <= 0xb5:
		idx := u2De(in.ops, 0)
		_, desc, err := a.lector.refDe(idx)
		if err != nil {
			return nil, false, err
		}
		t, err := tipoDeDesc(desc)
		if err != nil {
			return nil, false, err
		}
		switch op {
		case 0xb2: // getstatic
			a.empuja(t)
		case 0xb3: // putstatic
			if _, err := a.saca(); err != nil {
				return nil, false, err
			}
		case 0xb4: // getfield
			if _, err := a.sacaRef(); err != nil {
				return nil, false, err
			}
			a.empuja(t)
		case 0xb5: // putfield
			if _, err := a.saca(); err != nil {
				return nil, false, err
			}
			if _, err := a.sacaRef(); err != nil {
				return nil, false, err
			}
		}
	case op >= 0xb6 && op <= 0xb9:
		idx := u2De(in.ops, 0)
		_, desc, err := a.lector.refDe(idx)
		if err != nil {
			return nil, false, err
		}
		args, err := tiposDeArgs(desc)
		if err != nil {
			return nil, false, err
		}
		for i := len(args) - 1; i >= 0; i-- {
			t, err := a.saca()
			if err != nil {
				return nil, false, err
			}
			if t.etiqueta != args[i].etiqueta {
				return nil, false, fmt.Errorf("el argumento %d de %s llega %s y el descriptor pide %s", i, desc, t, args[i])
			}
		}
		if op != 0xb8 { // invokestatic no lleva receptor
			if _, err := a.sacaRef(); err != nil {
				return nil, false, err
			}
		}
		if v := vueltaDe(desc); v != "" && v != "V" {
			t, err := tipoDeDesc(v)
			if err != nil {
				return nil, false, err
			}
			a.empuja(t)
		}
	case op == 0xbb:
		clase, err := a.lector.claseDe(u2De(in.ops, 0))
		if err != nil {
			return nil, false, err
		}
		a.empuja(verifRef(clase))
	case op == 0xc1: // instanceof: consume la referencia y deja un entero (`Ins.esCadena`, parrafo 79)
		if _, err := a.lector.claseDe(u2De(in.ops, 0)); err != nil {
			return nil, false, err
		}
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
		a.empuja(verifInt)
	case op == 0xc0:
		clase, err := a.lector.claseDe(u2De(in.ops, 0))
		if err != nil {
			return nil, false, err
		}
		if _, err := a.sacaRef(); err != nil {
			return nil, false, err
		}
		a.empuja(verifRef(clase))
	case esCondicional(op):
		if op == 0xc6 || op == 0xc7 {
			if _, err := a.sacaRef(); err != nil {
				return nil, false, err
			}
		} else if op >= 0x9f && op <= 0xa4 {
			if err := a.sacaEntero(); err != nil {
				return nil, false, err
			}
			if err := a.sacaEntero(); err != nil {
				return nil, false, err
			}
		} else {
			if err := a.sacaEntero(); err != nil {
				return nil, false, err
			}
		}
		return []int{salto16(in)}, true, nil
	case op == 0xa7:
		return []int{salto16(in)}, false, nil
	case op == 0xaa || op == 0xab:
		if err := a.sacaEntero(); err != nil {
			return nil, false, err
		}
		ds, err := destinosDeSwitch(in)
		if err != nil {
			return nil, false, err
		}
		return ds, false, nil
	default:
		return nil, false, fmt.Errorf("la instruccion %s no esta en el analisis de marcos", in.name)
	}
	return nil, true, nil
}

// ----- calcular y escribir el atributo -----

// framesDeMetodo devuelve el cuerpo del StackMapTable (sin el nombre ni el largo) y el alto maximo
// que alcanzo la pila simulada. Si no hay ningun marco, el atributo no va.
func framesDeMetodo(code []byte, desc string, estatico bool, propio string, pool *poolBuilder, maxLoc int, tries []entradaTry) ([]byte, int, error) {
	lector := pool.lector()
	instrs, err := walkJvmCode(code)
	if err != nil {
		return nil, 0, err
	}
	porPC := map[int]jvmInstr{}
	siguiente := map[int]int{}
	for i, in := range instrs {
		porPC[in.pc] = in
		if i+1 < len(instrs) {
			siguiente[in.pc] = instrs[i+1].pc
		} else {
			siguiente[in.pc] = len(code)
		}
	}

	comienzos := map[int]bool{}
	for _, in := range instrs {
		if esCondicional(in.op) || in.op == 0xa7 {
			comienzos[salto16(in)] = true
		}
		if in.op == 0xaa || in.op == 0xab {
			ds, err := destinosDeSwitch(in)
			if err != nil {
				return nil, 0, err
			}
			for _, d := range ds {
				comienzos[d] = true
			}
		}
		if !tieneCaida(in.op) {
			if s := siguiente[in.pc]; s < len(code) {
				comienzos[s] = true
			}
		}
	}
	for pc := range comienzos {
		if _, ok := porPC[pc]; !ok && pc != len(code) {
			return nil, 0, fmt.Errorf("un salto apunta al pc %d, que no es una instruccion", pc)
		}
		if pc == len(code) {
			return nil, 0, fmt.Errorf("un salto apunta al final del cuerpo")
		}
	}
	// los handlers de un try son comienzos de bloque: el verificador les da una pila con la excepcion
	for _, tr := range tries {
		comienzos[tr.handler] = true
	}

	args, err := tiposDeArgs(desc)
	if err != nil {
		return nil, 0, err
	}
	inicial := estadoFrame{locales: []tipoVerif{}}
	if !estatico {
		inicial.locales = append(inicial.locales, verifRef(propio))
	}
	for _, t := range args {
		inicial.locales = append(inicial.locales, t)
		if t.etiqueta == 'j' || t.etiqueta == 'd' {
			// un argumento largo o doble ocupa el hueco y el siguiente (medido en `F.conRama`: `a` en
			// 0-1 y `n` en 2, parrafo 74; y en `FyD.por`: `a` en 0 y `b` en 1-2, parrafo 77)
			inicial.locales = append(inicial.locales, verifSegundo)
		}
	}
	// los huecos declarados despues de los argumentos existen desde el principio como top: el
	// verificador los da por no asignados hasta que una instruccion los escribe
	for len(inicial.locales) < maxLoc {
		inicial.locales = append(inicial.locales, verifTop)
	}

	estados := map[int]estadoFrame{0: inicial}
	cola := []int{0}
	enCola := map[int]bool{0: true}
	alto := 0
	for len(cola) > 0 {
		pc := cola[0]
		cola = cola[1:]
		enCola[pc] = false
		a := &analisis{lector: lector, est: copiaEstado(estados[pc])}
		cur := pc
		for {
			in, ok := porPC[cur]
			if !ok {
				return nil, 0, fmt.Errorf("el flujo llega al pc %d, que no es una instruccion", cur)
			}
			// cada instrucción protegida puede lanzar: el handler recibe sus locales con la
			// excepción encima de la pila (JVMS 4.10.1.6, medido en el §71 sobre `T`)
			for _, tr := range tries {
				if cur < tr.inicio || cur >= tr.fin {
					continue
				}
				tipo := verifRef("java/lang/Throwable")
				if tr.tipo != "" {
					t, err := tipoDeDesc(tr.tipo)
					if err != nil {
						return nil, 0, err
					}
					tipo = t
				}
				entrada := estadoFrame{locales: append([]tipoVerif(nil), a.est.locales...), pila: []tipoVerif{tipo}}
				cambio, err := fusiona(estados, tr.handler, entrada)
				if err != nil {
					return nil, 0, fmt.Errorf("pc %d: %w", tr.handler, err)
				}
				if cambio && !enCola[tr.handler] {
					cola = append(cola, tr.handler)
					enCola[tr.handler] = true
				}
			}
			sucesores, sigue, err := a.aplica(in)
			if err != nil {
				return nil, 0, fmt.Errorf("pc %d (%s): %w", cur, in.name, err)
			}
			if altoPilas(a.est.pila) > alto {
				alto = altoPilas(a.est.pila)
			}
			for _, s := range sucesores {
				cambio, err := fusiona(estados, s, a.est)
				if err != nil {
					return nil, 0, fmt.Errorf("pc %d: %w", s, err)
				}
				if cambio && !enCola[s] {
					cola = append(cola, s)
					enCola[s] = true
				}
			}
			if !sigue {
				break
			}
			cur = siguiente[cur]
			if cur >= len(code) {
				break
			}
			if comienzos[cur] {
				cambio, err := fusiona(estados, cur, a.est)
				if err != nil {
					return nil, 0, fmt.Errorf("pc %d: %w", cur, err)
				}
				if cambio && !enCola[cur] {
					cola = append(cola, cur)
					enCola[cur] = true
				}
				break
			}
		}
	}

	orden := []int{}
	for pc := range comienzos {
		orden = append(orden, pc)
	}
	sort.Ints(orden)
	for _, pc := range orden {
		if _, ok := estados[pc]; !ok {
			return nil, 0, fmt.Errorf("el pc %d no se alcanza desde el principio (codigo muerto)", pc)
		}
	}
	if len(orden) == 0 {
		return nil, alto, nil
	}
	return codificaFrames(estados, orden, pool), alto, nil
}

// codificaFrames escribe `full_frame` para cada comienzo, con los locales que llegan (recortando los
// `top` del final, que el verificador da por supuestos) y la pila entera. Necesita el pool para
// internar la clase de cada referencia que nombre.
func codificaFrames(estados map[int]estadoFrame, orden []int, pool *poolBuilder) []byte {
	cuerpo := u2be(uint16(len(orden)))
	previo := -1
	for _, pc := range orden {
		est := estados[pc]
		delta := pc - previo - 1
		if previo < 0 {
			delta = pc
		}
		cuerpo = append(cuerpo, 255)
		cuerpo = append(cuerpo, u2be(uint16(delta))...)
		var lista []tipoVerif
		for _, t := range est.locales {
			if t.etiqueta == 's' {
				// la segunda mitad de un ancho NO es una variable: javac escribe un solo `long` en la
				// lista de locales y el hueco extra cuenta para max_locals (medido en `F.conRama`,
				// donde el append dice `locals = [ long ]` para el local de los huecos 3 y 4)
				continue
			}
			lista = append(lista, t)
		}
		n := len(lista)
		for n > 0 && lista[n-1].etiqueta == 't' {
			n--
		}
		cuerpo = append(cuerpo, u2be(uint16(n))...)
		for _, t := range lista[:n] {
			cuerpo = append(cuerpo, codificaTipo(t, pool)...)
		}
		cuerpo = append(cuerpo, u2be(uint16(len(est.pila)))...)
		for _, t := range est.pila {
			cuerpo = append(cuerpo, codificaTipo(t, pool)...)
		}
		previo = pc
	}
	return cuerpo
}

// codificaTipo escribe un verification_type_info: ITEM_Top 0, ITEM_Integer 1, ITEM_Null 5 e
// ITEM_Object 7 con el indice de una Class del pool (JVMS 4.7.4).
func codificaTipo(t tipoVerif, pool *poolBuilder) []byte {
	switch t.etiqueta {
	case 'i':
		return []byte{1}
	case 'j':
		// ITEM_Long 4; la segunda mitad del hueco no se escribe (parrafo 74)
		return []byte{4}
	case 'f':
		return []byte{2} // ITEM_Float 2
	case 'd':
		return []byte{3} // ITEM_Double 3, una entrada para los dos huecos
	case 'l':
		if t.clase == "" {
			return []byte{5}
		}
		return append([]byte{7}, u2be(uint16(pool.clase(t.clase)))...)
	}
	return []byte{0}
}
