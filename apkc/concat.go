package main

// Desugración de `StringConcatFactory.makeConcatWithConstants`, el `invokedynamic` que `javac`
// escribe desde Java 9 para `a + b + c`. El bootstrap lleva una receta y una lista de constantes:
// `\u0001` consume el siguiente argumento dinámico y `\u0002` la siguiente constante. `d8` con
// `--min-api 21` lo baja a `new-instance StringBuilder`, un `append` por trozo y `toString`
// (medido en el §116). Aquí se hace lo mismo, con el constructor vacío y sin encadenar el
// `move-result-object` de cada `append`: el valor se ignora y el constructor sigue en su registro.
//
// `javac` suele doblar las constantes dentro de la propia receta; usa `\u0002` cuando el texto
// contiene los marcadores (`"a" + "\u0001" + b`), que es el caso que mide el banco.

import (
	"fmt"
	"strings"
)

// concatPieza es un trozo de la receta: texto literal, un argumento dinámico o una constante del
// bootstrap. `arg` y `cons` valen -1 cuando no son de ese tipo.
type concatPieza struct {
	texto string
	arg   int
	cons  int
}

// concatConstante es una constante del bootstrap, ya resuelta. Solo se aceptan los cinco tags que
// la especificación permite; una constante dinámica se rechaza por nombre.
type concatConstante struct {
	tag      byte
	texto    string
	entero   int32
	largo    int64
	flotante float32
	doble    float64
}

// classConcat es un sitio `makeConcatWithConstants` validado: la receta troceada, sus constantes y
// los tipos de los argumentos que llegan por la pila.
type classConcat struct {
	piezas     []concatPieza
	constantes []concatConstante
	params     []string
}

// sitioDeConcat reconoce el bootstrap de concatenación. Devuelve `es` en falso cuando el sitio no
// es suyo, para que `sitioDeLambda` lo intente; si es suyo y no está medido, devuelve el error.
func sitioDeConcat(cf classFull, x poolIndex, indice int) (classConcat, bool, error) {
	boot, nombre, desc := x.invokeDynamicDe(indice)
	if boot < 0 || boot >= len(cf.BootstrapMethods) {
		return classConcat{}, false, nil
	}
	bm := cf.BootstrapMethods[boot]
	_, owner, met, mdesc, _ := x.methodHandleDe(bm.handle)
	if owner != "Ljava/lang/invoke/StringConcatFactory;" {
		return classConcat{}, false, nil
	}
	if met != "makeConcatWithConstants" {
		return classConcat{}, true, fmt.Errorf("el bootstrap del sitio %q es %s.%s.%s, y solo se mide makeConcatWithConstants", nombre, owner, met, mdesc)
	}
	_, ret, params := splitDescriptor(desc)
	if ret != "Ljava/lang/String;" {
		return classConcat{}, true, fmt.Errorf("el sitio de concatenacion %q devuelve %s y no String", nombre, ret)
	}
	if len(bm.args) < 1 || x.tagOf(bm.args[0]) != cpString {
		return classConcat{}, true, fmt.Errorf("el sitio de concatenacion %q no trae la receta como String", nombre)
	}
	constantes, err := constantesDeConcat(x, nombre, bm.args[1:])
	if err != nil {
		return classConcat{}, true, err
	}
	piezas, err := partirReceta(x.stringValue(bm.args[0]), len(params), len(constantes))
	if err != nil {
		return classConcat{}, true, fmt.Errorf("el sitio de concatenacion %q: %w", nombre, err)
	}
	return classConcat{piezas: piezas, constantes: constantes, params: params}, true, nil
}

// constantesDeConcat resuelve los argumentos del bootstrap detrás de la receta. La especificación
// admite String, Integer, Long, Float, Double y ConstantDynamic; aquí se miden los cinco primeros.
func constantesDeConcat(x poolIndex, nombre string, args []int) ([]concatConstante, error) {
	out := make([]concatConstante, 0, len(args))
	for i, a := range args {
		c := concatConstante{tag: x.tagOf(a)}
		switch c.tag {
		case cpString:
			c.texto = x.stringValue(a)
		case cpInteger:
			c.entero = x.intOf(a)
		case cpLong:
			c.largo = x.longOf(a)
		case cpFloat:
			c.flotante = x.floatOf(a)
		case cpDouble:
			c.doble = x.doubleOf(a)
		case cpDynamic:
			return nil, fmt.Errorf("la constante %d del sitio de concatenacion %q es dinamica: no medido", i, nombre)
		default:
			return nil, fmt.Errorf("la constante %d del sitio de concatenacion %q tiene el tag %d: no medido", i, nombre, c.tag)
		}
		out = append(out, c)
	}
	return out, nil
}

// partirReceta trocea la receta por sus dos marcadores y exige que las cuentas cuadren con el
// descriptor y con las constantes del bootstrap.
func partirReceta(receta string, nArgs, nConst int) ([]concatPieza, error) {
	var piezas []concatPieza
	var texto strings.Builder
	cerrar := func() {
		if texto.Len() > 0 {
			piezas = append(piezas, concatPieza{texto: texto.String(), arg: -1, cons: -1})
			texto.Reset()
		}
	}
	arg, cons := 0, 0
	for _, r := range receta {
		switch r {
		case '\u0001':
			cerrar()
			if arg >= nArgs {
				return nil, fmt.Errorf("la receta pide %d argumentos y el descriptor trae %d", arg+1, nArgs)
			}
			piezas = append(piezas, concatPieza{arg: arg, cons: -1})
			arg++
		case '\u0002':
			cerrar()
			if cons >= nConst {
				return nil, fmt.Errorf("la receta pide %d constantes y el bootstrap trae %d", cons+1, nConst)
			}
			piezas = append(piezas, concatPieza{arg: -1, cons: cons})
			cons++
		default:
			texto.WriteRune(r)
		}
	}
	cerrar()
	if arg != nArgs {
		return nil, fmt.Errorf("la receta consume %d argumentos y el descriptor trae %d", arg, nArgs)
	}
	if cons != nConst {
		return nil, fmt.Errorf("la receta consume %d constantes y el bootstrap trae %d", cons, nConst)
	}
	return piezas, nil
}

// appendDeTipo: el `append` que `d8` elige para un argumento dinámico. Los anchos conservan su
// tipo, el `char` y el booleano tienen sobrecarga propia, y `byte`/`short` van por `int`. Cualquier
// otra referencia va por `Object`, que es lo que hace `String.valueOf` con ella (incluidos los
// arreglos, que javac suele convertir antes a String).
func appendDeTipo(tipo string) string {
	switch tipo {
	case "C":
		return "(C)Ljava/lang/StringBuilder;"
	case "Z":
		return "(Z)Ljava/lang/StringBuilder;"
	case "B", "S", "I":
		return "(I)Ljava/lang/StringBuilder;"
	case "J":
		return "(J)Ljava/lang/StringBuilder;"
	case "F":
		return "(F)Ljava/lang/StringBuilder;"
	case "D":
		return "(D)Ljava/lang/StringBuilder;"
	case "Ljava/lang/String;":
		return "(Ljava/lang/String;)Ljava/lang/StringBuilder;"
	default:
		return "(Ljava/lang/Object;)Ljava/lang/StringBuilder;"
	}
}

// appendDeConstante: el `append` de una constante del bootstrap según su tag.
func appendDeConstante(tag byte) string {
	switch tag {
	case cpString:
		return "(Ljava/lang/String;)Ljava/lang/StringBuilder;"
	case cpInteger:
		return "(I)Ljava/lang/StringBuilder;"
	case cpLong:
		return "(J)Ljava/lang/StringBuilder;"
	case cpFloat:
		return "(F)Ljava/lang/StringBuilder;"
	case cpDouble:
		return "(D)Ljava/lang/StringBuilder;"
	}
	return ""
}

// internConcat interna lo que el sitio va a nombrar: el constructor, `toString` y un `append` por
// trozo, más los textos literales como cadenas.
func (t *dexTables) internConcat(c classConcat) {
	t.internType("Ljava/lang/StringBuilder;")
	t.internMethod("Ljava/lang/StringBuilder;", "()V", "<init>")
	t.internMethod("Ljava/lang/StringBuilder;", "()Ljava/lang/String;", "toString")
	for _, p := range c.piezas {
		switch {
		case p.arg >= 0:
			t.internMethod("Ljava/lang/StringBuilder;", appendDeTipo(c.params[p.arg]), "append")
		case p.cons >= 0:
			c := c.constantes[p.cons]
			if c.tag == cpString {
				t.internString(c.texto)
			}
			t.internMethod("Ljava/lang/StringBuilder;", appendDeConstante(c.tag), "append")
		default:
			t.internString(p.texto)
			t.internMethod("Ljava/lang/StringBuilder;", "(Ljava/lang/String;)Ljava/lang/StringBuilder;", "append")
		}
	}
}

// concat: el `invokedynamic` ya validado en concat.go. Los argumentos están en la pila JVM; se
// sacan en orden inverso, se crea el constructor, se hace un `append` por trozo y se cierra con
// `toString`. El resultado queda en un registro nuevo y los argumentos se sueltan al final.
func (p *traduce) concat(in jvmInstr) {
	sitio, ok := p.clase.Concats[in.pc]
	if !ok {
		p.falla(in, "an invokedynamic that the concat expansion did not cover")
		return
	}
	// El registro del constructor se toma **antes** de sacar los argumentos: `toma` solo evita los
	// registros que la pila todavía nombra, y después del `pop` los de los argumentos quedarían
	// libres para reutilizarse. `lambda` hace lo mismo con su objeto.
	b := p.toma(in)
	if p.err != nil {
		return
	}
	valores := make([]int, len(sitio.params))
	for i := len(valores) - 1; i >= 0; i-- {
		valores[i] = p.pop(in)
	}
	if p.err != nil {
		return
	}
	p.palabra(uint16(opNuevaInstancia) | uint16(b&0xff)<<8)
	if !p.indice(p.typeIdxDe("Ljava/lang/StringBuilder;", in), in, "the type_id of the concat builder") {
		return
	}
	ctor, ok := p.metodoInternado("Ljava/lang/StringBuilder;", "()V", "<init>", in)
	if !ok {
		return
	}
	p.invocaConcat(opInvocaDirecto, []int{b}, []bool{false}, []bool{true}, ctor, in)
	for _, pieza := range sitio.piezas {
		switch {
		case pieza.arg >= 0:
			tipo := sitio.params[pieza.arg]
			v := valores[pieza.arg]
			metodo, ok := p.metodoInternado("Ljava/lang/StringBuilder;", appendDeTipo(tipo), "append", in)
			if !ok {
				return
			}
			regs, anchos, refs := []int{b, v}, []bool{false, false}, []bool{true, false}
			if esAncho(tipo) {
				regs = append(regs, v+1)
				anchos = append(anchos, true)
				refs = append(refs, false)
			} else if esObjeto(tipo) {
				refs[1] = true
			}
			p.invocaConcat(opInvocaVirtual, regs, anchos, refs, metodo, in)
		case pieza.cons >= 0:
			if !p.appendConstante(in, b, sitio.constantes[pieza.cons]) {
				return
			}
		default:
			r := p.toma(in)
			if p.err != nil {
				return
			}
			if !p.cadena(r, pieza.texto, in) {
				return
			}
			metodo, ok := p.metodoInternado("Ljava/lang/StringBuilder;", "(Ljava/lang/String;)Ljava/lang/StringBuilder;", "append", in)
			if !ok {
				return
			}
			p.invocaConcat(opInvocaVirtual, []int{b, r}, []bool{false, false}, []bool{true, true}, metodo, in)
			p.suelta(r)
		}
	}
	toString, ok := p.metodoInternado("Ljava/lang/StringBuilder;", "()Ljava/lang/String;", "toString", in)
	if !ok {
		return
	}
	p.invocaConcat(opInvocaVirtual, []int{b}, []bool{false}, []bool{true}, toString, in)
	r := p.toma(in)
	if p.err != nil {
		return
	}
	p.palabra(unidad11x(opMueveResultObj, r))
	for i, v := range valores {
		if esAncho(sitio.params[i]) {
			p.sueltaPar(v)
			continue
		}
		p.suelta(v)
	}
	p.suelta(b)
	p.push(r)
}

// appendConstante carga una constante del bootstrap y la añade. Las anchas dejan el par en la
// pila, se saca y se suelta después de la llamada, como hace `invocaMetodo`.
func (p *traduce) appendConstante(in jvmInstr, b int, c concatConstante) bool {
	desc := appendDeConstante(c.tag)
	metodo, ok := p.metodoInternado("Ljava/lang/StringBuilder;", desc, "append", in)
	if !ok {
		return false
	}
	var (
		regs   []int
		anchos []bool
		refs   []bool
	)
	switch c.tag {
	case cpString:
		r := p.toma(in)
		if p.err != nil {
			return false
		}
		if !p.cadena(r, c.texto, in) {
			return false
		}
		regs, anchos, refs = []int{b, r}, []bool{false, false}, []bool{true, true}
		p.invocaConcat(opInvocaVirtual, regs, anchos, refs, metodo, in)
		p.suelta(r)
		return p.err == nil
	case cpInteger:
		r := p.toma(in)
		if p.err != nil {
			return false
		}
		if !p.entero(r, int(c.entero), in) {
			return false
		}
		regs, anchos, refs = []int{b, r}, []bool{false, false}, []bool{true, false}
		p.invocaConcat(opInvocaVirtual, regs, anchos, refs, metodo, in)
		p.suelta(r)
		return p.err == nil
	case cpLong:
		p.cargaLargo(in, c.largo)
		if p.err != nil {
			return false
		}
		v := p.pop(in)
		regs, anchos, refs = []int{b, v, v + 1}, []bool{false, true, true}, []bool{true, false, false}
		p.invocaConcat(opInvocaVirtual, regs, anchos, refs, metodo, in)
		p.sueltaPar(v)
		return p.err == nil
	case cpFloat:
		p.cargaFlotante(in, float64(c.flotante))
		if p.err != nil {
			return false
		}
		v := p.pop(in)
		regs, anchos, refs = []int{b, v}, []bool{false, false}, []bool{true, false}
		p.invocaConcat(opInvocaVirtual, regs, anchos, refs, metodo, in)
		p.suelta(v)
		return p.err == nil
	case cpDouble:
		p.cargaDoble(in, c.doble)
		if p.err != nil {
			return false
		}
		v := p.pop(in)
		regs, anchos, refs = []int{b, v, v + 1}, []bool{false, true, true}, []bool{true, false, false}
		p.invocaConcat(opInvocaVirtual, regs, anchos, refs, metodo, in)
		p.sueltaPar(v)
		return p.err == nil
	}
	p.falla(in, "a bootstrap constant of a concatenation with tag "+itoa(int(c.tag)))
	return false
}

// cadena: `const-string` 21c de un texto, con el destino ya elegido.
func (p *traduce) cadena(r int, s string, in jvmInstr) bool {
	p.palabra(uint16(opConstString) | uint16(r&0xff)<<8)
	return p.indice(p.t.stringIdx[s], in, "the string_id of a concatenation chunk")
}

// metodoInternado: el method_id de una llamada que `internConcat` ya internó.
func (p *traduce) metodoInternado(owner, desc, nombre string, in jvmInstr) (int, bool) {
	m, ok := p.t.methodIdx[methodKey{owner: owner, proto: protoKeyOf(desc), desc: desc, name: nombre}]
	if !ok {
		p.falla(in, "a method that was never interned ("+owner+"."+nombre+" "+desc+")")
		return 0, false
	}
	return m, true
}

// invocaConcat: como `invoca`, pero contando la llamada para `outs_size`, que es lo que hace
// `invocaMetodo` y no hace el `invoca` pelado.
func (p *traduce) invocaConcat(op byte, regs []int, anchos, refs []bool, metodo int, in jvmInstr) {
	if len(regs) > p.outs {
		p.outs = len(regs)
	}
	p.invoca(op, regs, anchos, refs, metodo, in)
}
