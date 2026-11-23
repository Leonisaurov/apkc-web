package main

// Reconciliación del reparto de registros en los puntos de unión.
//
// El traductor es un barrido lineal del cuerpo JVM: un solo estado (`enPila`, `renom`) que avanza
// instrucción a instrucción. Eso basta mientras cada bloque tenga un solo camino de entrada, pero en
// un punto de unión los caminos llegan con los mismos VALORES en registros distintos y el barrido se
// queda con el reparto del camino que cae. Medido con `javac 17 -g` y `d8` sobre
// `static int ternario(boolean c, int a, int b) { int r = c ? a : b; return r; }`:
//
//	0: iload_0 / 1: ifeq 8 / 4: iload_1 / 5: goto 9 / 8: iload_2 / 9: istore_3 / 10: iload_3 / 11: ireturn
//
// `d8` escribe `if-eqz v1,0004; move v0,v2; goto 0005; move v0,v3; return v0`: un registro canónico
// y un `move` por predecesor. Antes de esta pasada el barrido lineal llegaba al pc 9 con la pila
// `[v1, v2]` —los dos caminos apilados uno encima del otro— y `istore_3` se quedaba con `v2`, así que
// las DOS ramas devolvían `b`. El dex pasaba `dexdump` y ART lo ejecutaba: el error era el valor.
//
// Lo que hace esta pasada: en cada `comienzo` (destino de salto o la instrucción que sigue a una que
// no cae) fija un reparto canónico para la pila; el camino que cae ya lo trae (se queda como está) y
// los saltos que llegan con otro reparto pasan por un bloque de fuera de línea que emite los
// `move`/`move-object`/`move-wide` que faltan y salta al cuerpo. Los bloques van al final del método,
// como los payloads de `switch`, para que el camino que cae no los ejecute ni se muevan las unidades
// ya escritas.
//
// Los LOCALES no necesitan esto: su registro es estado global (`renom`) y una escritura en una rama
// se ve desde la otra, que es lo que ya hacía `guardaLocal` (§71).

// estadoPila es el reparto de registros de la pila JVM en un punto: una entrada por VALOR, con su
// registro, su tipo de arreglo y su ancho.
type estadoPila []valorPila

// puntoDeUnion es un salto que llega a un pc, con la pila que lleva al saltar. Se guarda el hueco
// donde hay que anotar el bloque de reconciliación: el índice en `p.saltos` para un `goto`/`if-*`, o
// el par (carga, hueco) para un destino de `switch`, que se parchea en el payload.
type puntoDeUnion struct {
	pila    estadoPila
	destino int
	origen  int // pc de la instrucción de salto, para nombrar una frontera
	salto   int // índice en p.saltos, o -1
	carga   int // índice en p.cargas, o -1
	hueco   int // índice del destino dentro de c.destinos
}

// fronteraUnion: un camino que llega a un comienzo con OTRA altura de pila que la del bloque. El
// reparto de registros no puede cambiar una altura —no hay registro que inventar ni que tirar sin una
// instrucción—, así que esa llegada se deja como estaba en vez de abortar el método: la altura la fija
// el cuerpo JVM que se traduce, no este plan. Medido el 2026-11-23 con `$record$hashCode` del
// desugrado de records, cuyo `goto` de vuelta apunta al `istore_3` de la cabecera (pc 8) en vez de al
// `iload_3` (pc 9): el camino que cae llega con un valor y el borde de vuelta con ninguno.
type fronteraUnion struct {
	pc       int // pc de la instrucción de salto
	destino  int // pc del comienzo
	cabecera int // altura con la que el bloque quedó escrito
	borde    int // altura con la que llega este camino
}

// String nombra la frontera: es el mensaje propio de estas llegadas, distinto del de un reparto que
// no se puede escribir (ese sí aborta, porque un `move` mal puesto cambia el valor).
func (f fronteraUnion) String() string {
	return "the path at pc " + itoa(f.pc) + " reaches pc " + itoa(f.destino) + " with " + itoa(f.borde) +
		" values and that block starts with " + itoa(f.cabecera) + ": the register plan cannot change " +
		"a stack height, so that arrival keeps the plan it had"
}

// anotaFrontera apunta una llegada con otra altura sin repetir la que ya estuviera anotada. La forma
// larga de un condicional (`saltoCondicionalLargo`) pasa por `reconciliaAntesDeSaltar` dos veces
// —una desde `escribeBranquia` y otra desde `salto32`— y, como con una altura distinta esa función no
// hace `fijaPila`, la segunda veía la misma llegada: el par (pc, destino) se anotaba dos veces. Dos
// fronteras idénticas no describen dos caminos, sino el mismo; se compara la estructura entera para no
// descartar una llegada del mismo pc a otro bloque con otra altura.
func (p *traduce) anotaFrontera(f fronteraUnion) {
	for _, ya := range p.fronteras {
		if ya == f {
			return
		}
	}
	p.fronteras = append(p.fronteras, f)
}

// padRecon es un bloque de fuera de línea que iguala el reparto de un camino con el del cuerpo.
type padRecon struct {
	pila   estadoPila // reparto con el que llega el camino
	hasta  estadoPila // reparto canónico del cuerpo
	origen int        // pc del comienzo, para nombrar un fallo
	desde  int        // unidad donde empieza el bloque, se fija al escribirlo
	cuerpo int        // unidad del cuerpo al que salta
}

// movPila es un valor que cambia de registro: de `de` a `a`.
type movPila struct {
	de valorPila
	a  valorPila
}

// pilaActual copia el reparto de la pila virtual.
func (p *traduce) pilaActual() estadoPila {
	e := make(estadoPila, len(p.enPila))
	for i, r := range p.enPila {
		v := valorPila{r: r, move: opMueve}
		if i < len(p.pilaTipo) {
			v.tipo = p.pilaTipo[i]
		}
		if i < len(p.pilaAncho) {
			v.ancho = p.pilaAncho[i]
		}
		if i < len(p.pilaMove) {
			v.move = p.pilaMove[i]
		}
		e[i] = v
	}
	return e
}

// igualPila: los dos repartos ponen cada valor en el mismo registro. El tipo y el ancho no se miran
// porque describen el mismo valor en los dos lados.
func igualPila(a, b estadoPila) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].r != b[i].r || a[i].ancho != b[i].ancho {
			return false
		}
	}
	return true
}

// enEstado: el reparto lleva ese registro.
func enEstado(e estadoPila, r int) bool {
	for _, v := range e {
		if v.r == r {
			return true
		}
	}
	return false
}

// fijaPila deja la pila virtual en `e`. Los temporales que el reparto viejo llevaba y el nuevo no se
// devuelven a la bolsa: si no, el marco crecería con registros que ya nadie usa.
func (p *traduce) fijaPila(e estadoPila) {
	vieja := p.pilaActual()
	p.enPila = p.enPila[:0]
	p.pilaTipo = p.pilaTipo[:0]
	p.pilaAncho = p.pilaAncho[:0]
	p.pilaMove = p.pilaMove[:0]
	for _, v := range vieja {
		if enEstado(e, v.r) {
			continue
		}
		if v.ancho {
			p.sueltaPar(v.r)
		} else {
			p.suelta(v.r)
		}
	}
	for _, v := range e {
		p.pushValor(v.r, v.tipo, v.ancho)
		p.ponMoveCima(v.move)
	}
}

// solapa: los dos valores comparten alguna palabra de registro.
func solapa(a, b valorPila) bool {
	fa, fb := 1, 1
	if a.ancho {
		fa = 2
	}
	if b.ancho {
		fb = 2
	}
	for x := 0; x < fa; x++ {
		for y := 0; y < fb; y++ {
			if a.r+x == b.r+y {
				return true
			}
		}
	}
	return false
}

// pisaFuente: escribir el destino de `pend[i]` pisaría el origen de otro valor que aún no se ha
// movido. Mientras eso pase, ese movimiento tiene que esperar.
func pisaFuente(pend []movPila, i int) bool {
	for j := range pend {
		if j == i {
			continue
		}
		if solapa(pend[i].a, pend[j].de) {
			return true
		}
	}
	return false
}

// registroDeTrabajo busca un registro que no lleve ni un origen ni un destino pendientes, para romper
// un ciclo de movimientos (dos valores que se intercambian los registros). Por debajo de `base` solo
// hay temporales, y el bloque de reconciliación está fuera de línea: nadie más los tiene vivos.
func (p *traduce) registroDeTrabajo(pend []movPila, ancho bool) (int, bool) {
	for r := 0; r+1 < p.base || (!ancho && r < p.base); r++ {
		if p.fijos[r] || p.altos[r] {
			continue
		}
		libre := true
		for _, m := range pend {
			if solapa(valorPila{r: r, ancho: ancho}, m.de) || solapa(valorPila{r: r, ancho: ancho}, m.a) {
				libre = false
				break
			}
		}
		if libre {
			return r, true
		}
	}
	return 0, false
}

// escribeMovimiento emite la copia de un valor. La forma de cuatro bits es la de siempre; si el
// destino o el origen no caben en cuatro bits se usa la 22v (`move/from16`), que es la que midió el
// §86 para bajar un registro alto.
func (p *traduce) escribeMovimiento(m movPila, in jvmInstr) bool {
	op := byte(opMueve)
	switch {
	case m.de.ancho:
		op = opMueveWide
	case m.de.move == opMueveObjeto:
		op = opMueveObjeto
	}
	techo := 15
	if m.de.ancho {
		techo = 14
	}
	if m.a.r <= techo && m.de.r <= techo {
		p.palabra(unidad12x(op, m.a.r, m.de.r))
		return true
	}
	if m.a.r > 0xff || m.de.r > 0xffff {
		p.falla(in, "a join needs a move out of the eight-bit registers")
		return false
	}
	p.palabra(unidad22v(mueveFrom16(op), m.a.r))
	p.palabra(uint16(m.de.r))
	return true
}

// escribeReconciliacion emite los movimientos que llevan la pila de `de` a los registros de `a`, en
// un orden que no pise ningún origen pendiente, y rompe un ciclo con un registro de trabajo.
func (p *traduce) escribeReconciliacion(de, a estadoPila, in jvmInstr) bool {
	pend := []movPila{}
	for i := range de {
		if de[i].r == a[i].r {
			continue
		}
		pend = append(pend, movPila{de: de[i], a: a[i]})
	}
	for len(pend) > 0 {
		hecho := -1
		for i := range pend {
			if !pisaFuente(pend, i) {
				hecho = i
				break
			}
		}
		if hecho < 0 {
			libre, ok := p.registroDeTrabajo(pend, pend[0].de.ancho)
			if !ok {
				p.falla(in, "the stack registers of a join form a cycle and no scratch register is free")
				return false
			}
			op := byte(opMueve)
			if pend[0].de.ancho {
				op = opMueveWide
			}
			p.palabra(unidad12x(op, libre, pend[0].de.r))
			pend[0].de.r = libre
			continue
		}
		if !p.escribeMovimiento(pend[hecho], in) {
			return false
		}
		pend = append(pend[:hecho], pend[hecho+1:]...)
	}
	return true
}

// reconcilia fija el reparto canónico de la pila en un comienzo y deja apuntados los bloques de los
// caminos que no coinciden. Devuelve false con `p.err` puesto si dos caminos llegan con pilas de
// distinta altura, que no es un reparto malo sino un emisor malo.
func (p *traduce) reconcilia(in jvmInstr) bool {
	actual := p.pilaActual()
	sitios := p.union[in.pc]
	var canon estadoPila
	switch {
	case p.caidas[in.pc]:
		// el camino que cae trae el reparto bueno: es el estado que el barrido tiene ahora mismo
		canon = actual
	case len(sitios) > 0:
		// ningún camino cae aquí, así que el estado lineal es el de un bloque que no cae: se tira y
		// manda el primero de los saltos que ya se emitieron
		canon = p.sitios[sitios[0]].pila
		p.fijaPila(canon)
	default:
		// una cabecera de bucle cuyo borde llega después: se deja el reparto que trae el barrido y se
		// apunta, para que el borde de vuelta se iguale a él al emitirse
		p.pilaEn[in.pc] = actual
		p.pcs[in.pc] = len(p.w)
		return true
	}
	creados := []int{}
	for _, idx := range sitios {
		s := &p.sitios[idx]
		if len(s.pila) != len(canon) {
			// otra altura: el plan de registros no la puede igualar, así que este camino se queda
			// con el que traía (fronteraUnion) en vez de tumbar el método
			p.anotaFrontera(fronteraUnion{pc: s.origen, destino: in.pc,
				cabecera: len(canon), borde: len(s.pila)})
			continue
		}
		if igualPila(s.pila, canon) {
			continue
		}
		p.pads = append(p.pads, padRecon{pila: s.pila, hasta: canon, origen: in.pc, cuerpo: -1})
		creados = append(creados, len(p.pads)-1)
		p.fijaPad(idx, len(p.pads)-1)
	}
	p.pcs[in.pc] = len(p.w)
	for _, i := range creados {
		p.pads[i].cuerpo = p.pcs[in.pc]
	}
	p.pilaEn[in.pc] = canon
	return true
}

// fijaPad apunta un punto de unión al bloque que lo reconcilia.
func (p *traduce) fijaPad(sitio, pad int) {
	s := &p.sitios[sitio]
	if s.salto >= 0 {
		p.saltos[s.salto].pad = pad
		return
	}
	p.cargas[s.carga].pads[s.hueco] = pad
}

// reconciliaAntesDeSaltar iguala el reparto de la pila con el del destino cuando ese destino ya se
// emitió (un borde de vuelta, o sea un bucle). Se llama ANTES de escribir la branquia para que los
// `move` caigan en el camino del salto y el desplazamiento siga midiéndose desde la branquia.
func (p *traduce) reconciliaAntesDeSaltar(destino int, in jvmInstr) bool {
	canon, ok := p.pilaEn[destino]
	if !ok {
		return true
	}
	actual := p.pilaActual()
	if igualPila(actual, canon) {
		return true
	}
	if len(actual) != len(canon) {
		// el borde de vuelta llega con otra altura: se deja el reparto que traía, sin abortar (ver
		// fronteraUnion). Cambiar la altura aquí exigiría inventar o tirar un valor, y eso cambia el
		// programa.
		p.anotaFrontera(fronteraUnion{pc: in.pc, destino: destino,
			cabecera: len(canon), borde: len(actual)})
		return true
	}
	if !p.escribeReconciliacion(actual, canon, in) {
		return false
	}
	p.fijaPila(canon)
	return true
}

// escribePads escribe los bloques de reconciliación detrás del cuerpo, antes de los payloads de los
// `switch`: los `move` que igualan el reparto y un `goto/32` al cuerpo. El `goto/32` aguanta cualquier
// distancia —el cuerpo está detrás—, así que un bloque nunca se queda corto.
func (p *traduce) escribePads() error {
	for i := range p.pads {
		pd := &p.pads[i]
		pd.desde = len(p.w)
		in := jvmInstr{pc: pd.origen, name: "join"}
		if !p.escribeReconciliacion(pd.pila, pd.hasta, in) {
			return p.err
		}
		desde := len(p.w)
		p.palabra(uint16(opGoto32))
		p.palabra(0)
		p.palabra(0)
		rel := pd.cuerpo - desde
		p.w[desde+1] = uint16(uint32(rel))
		p.w[desde+2] = uint16(uint32(rel) >> 16)
	}
	return nil
}

// caeDespues: la instrucción deja que el control siga por la siguiente. Se mira por nombre y no por
// opcode porque los `return` de los seis tipos y el `athrow` tienen opcodes sueltos.
func caeDespues(nombre string) bool {
	switch nombre {
	case "goto", "goto_w", "tableswitch", "lookupswitch",
		"return", "ireturn", "lreturn", "freturn", "dreturn", "areturn", "athrow":
		return false
	}
	return true
}

// comienzosDe: los pc donde empieza un bloque, con la misma regla que el StackMapTable (§65): cada
// destino de salto y la instrucción que sigue a una que no cae. Es donde el verificador une los
// caminos y, por tanto, donde el reparto de registros tiene que estar de acuerdo.
func comienzosDe(lista []jvmInstr) (map[int]bool, error) {
	c := map[int]bool{}
	for k, in := range lista {
		switch {
		case esCondicional(in.op):
			c[salto16(in)] = true
		case in.op == 0xa7:
			c[salto16(in)] = true
		case in.op == 0xc8:
			c[salto32(in)] = true
		case in.op == 0xaa || in.op == 0xab:
			ds, err := destinosDeSwitch(in)
			if err != nil {
				return nil, err
			}
			for _, d := range ds {
				c[d] = true
			}
		}
		if !caeDespues(in.name) && k+1 < len(lista) {
			c[lista[k+1].pc] = true
		}
	}
	return c, nil
}
