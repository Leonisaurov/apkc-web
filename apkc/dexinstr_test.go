package main

import (
	"regexp"
	"strings"
	"testing"
)

// tablaDeLasSeis construye y ordena las tablas de índices de los seis .class, que es lo que el
// traductor necesita para resolver un operando del pool a un id del dex.
func tablaDeLasSeis(t *testing.T) (*dexTables, []classFull) {
	t.Helper()
	clases := leeLasSeis(t)
	td := newDexTables()
	if err := td.collectClasses(clases); err != nil {
		t.Fatalf("interning: %v", err)
	}
	td.orderTables()
	return td, clases
}

var regExp = regexp.MustCompile(`v\d+`)

// esqueleto quita los números de registro: lo que se compara con d8 son las instrucciones, los ids
// que nombran y el orden. Los registros los reparte un asignador que aquí no es el de d8, y esa
// diferencia está aceptada y escrita (docs/plan.md 2G.4); la de una instrucción que falta no.
func esqueleto(lista []string) []string {
	out := make([]string, 0, len(lista))
	for _, s := range lista {
		out = append(out, regExp.ReplaceAllString(s, "v"))
	}
	return out
}

// TestTraduceEmiteLasMismasInstruccionesQueD8 es la comparación fina que el plan 2G.3 pide: por
// cada uno de los nueve cuerpos, las palabras que emitimos aquí desmontan en las mismas
// instrucciones, sobre los mismos índices y en el mismo orden que las que escribió d8.
func TestTraduceEmiteLasMismasInstruccionesQueD8(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}
	deD8 := map[[2]string]codeItem{}
	for _, c := range d.classes {
		cd := porClase[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatal(err)
			}
			deD8[[2]string{c.name, d.methods[m.methodIdx].name}] = ci
		}
	}

	nueve := 0
	exactos := 0
	for _, cf := range clases {
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			nueve++
			ci, err := td.codeItemDe(cf, m)
			if err != nil {
				t.Errorf("%s%s: %v", cf.Name, m.Name, err)
				continue
			}
			nuestro, err := desmontaPalabras(ci.insns)
			if err != nil {
				t.Errorf("%s%s: our own words do not read back: %v", cf.Name, m.Name, err)
				continue
			}
			suyo := deD8[[2]string{cf.Name, m.Name}]
			suyoLista, err := desmontaPalabras(suyo.insns)
			if err != nil {
				t.Errorf("%s%s: d8's words do not read: %v", cf.Name, m.Name, err)
				continue
			}
			a, b := esqueleto(nuestro), esqueleto(suyoLista)
			if strings.Join(a, " | ") != strings.Join(b, " | ") {
				t.Errorf("%s%s emits a different instruction sequence\n  ours: %s\n  d8:   %s",
					cf.Name, m.Name, strings.Join(a, " | "), strings.Join(b, " | "))
			}
			if len(ci.insns) != len(suyo.insns) {
				t.Errorf("%s%s: %d words, d8 wrote %d", cf.Name, m.Name, len(ci.insns), len(suyo.insns))
			}
			if strings.Join(nuestro, " | ") == strings.Join(suyoLista, " | ") {
				exactos++
			}
			// el unico sitio donde este traductor se separa de d8 es el numero de registro, y
			// eso se nota aqui: cinco de los nueve piden a lo sumo un registro mas que d8, que
			// reutiliza los de argumentos muertos. No es un dex distinto: es un marco mas ancho
			if extra := int(ci.registersSize) - int(suyo.registersSize); extra < 0 || extra > 1 {
				t.Errorf("%s%s: registers_size %d against d8's %d, a gap wider than one register",
					cf.Name, m.Name, ci.registersSize, suyo.registersSize)
			}
		}
	}
	if nueve != 9 {
		t.Errorf("%d methods with Code in the six classes, javap counted 9", nueve)
	}
	// seis de nueve salen palabra por palabra igual que d8, incluidos los numeros de registro:
	// los cuatro constructores de R, el de MainActivity y el de la anonima con su this$0. Los
	// otros tres difieren solo en como se numeran los registros (§21)
	if exactos != 6 {
		t.Errorf("%d of 9 methods come out word-for-word equal to d8, the measurement says 6", exactos)
	}
}

// TestLaCabeceraSaleCoherenteConLoMedido: ins_size tiene que ser el args_size de javap y
// registers_size no puede quedar por debajo de sus locales ni de sus propias palabras.
func TestLaCabeceraSaleCoherenteConLoMedido(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	for _, cf := range clases {
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			ci, err := td.codeItemDe(cf, m)
			if err != nil {
				t.Fatalf("%s%s: %v", cf.Name, m.Name, err)
			}
			if int(ci.insSize) != m.ArgCount {
				t.Errorf("%s%s: ins_size %d, javap args_size %d", cf.Name, m.Name, ci.insSize, m.ArgCount)
			}
			if int(ci.registersSize) < m.LocalCount {
				t.Errorf("%s%s: %d registers for %d javap locals", cf.Name, m.Name, ci.registersSize, m.LocalCount)
			}
			if ci.registersSize < ci.insSize || ci.registersSize < ci.outsSize {
				t.Errorf("%s%s: registers_size %d does not cover ins %d / outs %d",
					cf.Name, m.Name, ci.registersSize, ci.insSize, ci.outsSize)
			}
			if int(ci.outsSize) > m.MaxStack {
				t.Errorf("%s%s: outs_size %d above javap max_stack %d", cf.Name, m.Name, ci.outsSize, m.MaxStack)
			}
			if ci.triesSize != 0 || ci.debugInfoOff != 0 {
				t.Errorf("%s%s: the fork emits no tries and no debug yet", cf.Name, m.Name)
			}
			if ci.tamanoEnBytes() != 16+2*len(ci.insns) {
				t.Errorf("%s%s: the size of the item does not follow its words", cf.Name, m.Name)
			}
		}
	}
}

// TestRechazaLoQueNoEstaMedido: un opcode fuera de la tabla, y un registro que no cabe en cuatro
// bits, se dicen por su nombre. Que salga un dex silenciosamente mal es lo único peor que parar.
func TestRechazaLoQueNoEstaMedido(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	cf := clases[1] // MainActivity

	// ladd (0x61) no esta en la tabla: el caminante lo dice por su byte. (Iadd estuvo aquí hasta el
	// parrafo 50, que lo midio y lo ingreso en la tabla con sus cuatro hermanos.)
	sinMedir := classMethod{HasCode: true, Name: "suma", Desc: "()I", LocalCount: 1, ArgCount: 1, MaxStack: 2,
		Code: []byte{0x03, 0x61}}
	if _, err := td.codeItemDe(cf, sinMedir); err == nil {
		t.Error("an opcode outside the measured table was accepted")
	} else if !strings.Contains(err.Error(), "0x61") {
		t.Errorf("the refusal does not name the byte: %v", err)
	}

	// un local que cae en v16 con el registro de cuatro bits de un 11n/12x/35c: se rechaza, no se
	// recorta (aload_0 + ireturn, las dos medidas)
	conBaseAncha := classMethod{HasCode: true, Name: "devuelve", Desc: "()I", LocalCount: 1, ArgCount: 1, MaxStack: 1,
		Code: []byte{0x2a, 0xac}}
	p := &traduce{t: td, x: poolIndex{p: cf.p}, clase: cf, m: conBaseAncha, locales: 1, base: 16}
	if err := p.recorre(); err == nil {
		t.Error("a register above v15 in a four-bit format was accepted")
	} else if !strings.Contains(err.Error(), "four-bit") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}
