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

// TestTraduceBranquiasContraLasPalabrasDeD8 fija los numeros que escribio `d8` sobre la sonda del
// parrafo 52 (p/Branquias y p/Saltos, con `--min-api 21`). El cuerpo JVM esta escrito a mano con los
// desplazamientos que midio `javap`, y lo que se compara es la sucesion desmontada sin numeros de
// registro: si el desplazamiento parcheado sale mal, el destino cae en otra instruccion y la fila
// cambia. En `plegable` el nuestro conserva el `goto` que d8 quito por propagacion de copias, con el
// mismo +2 que d8 escribio en su propio goto de `conElse` (0x0228).
func TestTraduceBranquiasContraLasPalabrasDeD8(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	cf := clases[0]
	sondas := []struct {
		nombre  string
		desc    string
		pila    int
		locales int
		args    int
		code    []byte
		quieren string
	}{
		{
			nombre: "igual", desc: "(II)I", pila: 2, locales: 3, args: 3,
			code:    []byte{0x1b, 0x1c, 0xa0, 0x00, 0x05, 0x04, 0xac, 0x03, 0xac},
			quieren: "if-ne v, v, +4 | const/4 v, 1 | return v | const/4 v, 0 | return v",
		},
		{
			nombre: "distinto", desc: "(II)I", pila: 2, locales: 3, args: 3,
			code:    []byte{0x1b, 0x1c, 0x9f, 0x00, 0x05, 0x04, 0xac, 0x03, 0xac},
			quieren: "if-eq v, v, +4 | const/4 v, 1 | return v | const/4 v, 0 | return v",
		},
		{
			nombre: "mayorcero", desc: "(I)I", pila: 1, locales: 2, args: 2,
			code:    []byte{0x1b, 0x9e, 0x00, 0x05, 0x05, 0xac, 0x06, 0xac},
			quieren: "if-lez v, +4 | const/4 v, 2 | return v | const/4 v, 3 | return v",
		},
		{
			nombre: "nulo", desc: "(Ljava/lang/Object;)I", pila: 1, locales: 2, args: 2,
			code:    []byte{0x2b, 0xc7, 0x00, 0x05, 0x07, 0xac, 0x08, 0xac},
			quieren: "if-nez v, +4 | const/4 v, 4 | return v | const/4 v, 5 | return v",
		},
		{
			nombre: "plegable", desc: "(I)I", pila: 1, locales: 2, args: 2,
			code: []byte{0x1b, 0x9a, 0x00, 0x08, 0x04, 0x3c, 0xa7, 0x00, 0x05,
				0x05, 0x3c, 0x1b, 0xac},
			quieren: "if-nez v, +4 | const/4 v, 1 | goto +2 | const/4 v, 2 | return v",
		},
		{
			// tres branquias anidadas que caen al mismo sitio: es `tresNiveles` de la sonda, y
			// aqui si sale palabra por palabra igual que d8 (salvo los numeros de registro)
			nombre: "tresNiveles", desc: "(II)I", pila: 2, locales: 3, args: 3,
			code: []byte{0x1b, 0x1c, 0xa0, 0x00, 0x0d, 0x1b, 0x9e, 0x00, 0x09,
				0x1c, 0x9e, 0x00, 0x05, 0x04, 0xac, 0x05, 0xac},
			quieren: "if-ne v, v, +8 | if-lez v, +6 | if-lez v, +4 | const/4 v, 1 | return v | const/4 v, 2 | return v",
		},
	}
	for _, s := range sondas {
		m := classMethod{HasCode: true, Name: s.nombre, Desc: s.desc,
			MaxStack: s.pila, LocalCount: s.locales, ArgCount: s.args, Code: s.code}
		ci, err := td.codeItemDe(cf, m)
		if err != nil {
			t.Errorf("%s: %v", s.nombre, err)
			continue
		}
		lista, err := desmontaPalabras(ci.insns)
		if err != nil {
			t.Errorf("%s: nuestras palabras no se leen: %v", s.nombre, err)
			continue
		}
		if sale := strings.Join(esqueleto(lista), " | "); sale != s.quieren {
			t.Errorf("%s:\n  nuestro: %s\n  d8:      %s", s.nombre, sale, s.quieren)
		}
	}
}

// TestRechazaUnaRamaHaciaAtras: el cuerpo de un `while` escrito por javac cabe entero en la tabla de
// anchos, y aun asi se rechaza. Lo que no esta medido no es el opcode, es el reparto de registros en
// el destino de una rama que llega por dos caminos; `d8` lo calcula con viveza y aqui no hay ni una
// sonda que lo fije. Se comprueba con el bucle `while (a > 0) { a = 1; }` (iload_1, ifle 9, iconst_1,
// istore_1, goto 0, iload_1, ireturn), que es el que javac escribe para esa linea.
func TestRechazaUnaRamaHaciaAtras(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	bucle := classMethod{HasCode: true, Name: "mientras", Desc: "(I)I",
		MaxStack: 1, LocalCount: 2, ArgCount: 2,
		Code: []byte{0x1b, 0x9e, 0x00, 0x08, 0x04, 0x3c, 0xa7, 0xff, 0xfa, 0x1b, 0xac}}
	if _, err := td.codeItemDe(clases[0], bucle); err == nil {
		t.Error("se acepto una rama hacia atras: un bucle sin medir su punto de fusion")
	} else if !strings.Contains(err.Error(), "loop") {
		t.Errorf("el rechazo no dice por que: %v", err)
	}
}

// TestTraduceLaConcatenacionBajaAStringBuilder: la cadena de StringBuilder que escribe el emisor llega
// al dex con las formas ya medidas (new-instance 0x22, invoke-direct 0x70, const-string 0x1a,
// invoke-virtual 0x6e con su move-result-object y return-object 0x11), y el plegado de constantes llega
// hecho: `1 / 0` conserva el divisor porque javac no lo dobla, y `"hola" + " " + 3` es un solo
// const-string. Se comparan los mnemonicos, no los ids ni los registros: los primeros salen del orden
// en que este internador vio las clases, y eso no es lo que se quiso medir aqui.
func TestTraduceLaConcatenacionBajaAStringBuilder(t *testing.T) {
	cf := claseDelFuente(t, "Cadena.java", fuenteCadena, "com/myapp/Cadena")
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	medidos := map[string]string{
		"todoLiteral": "const-string return-object",
		"plegado":     "const/4 return",
		"entreCero":   "const/4 const/4 div-int/2addr return",
		"conEntero": "new-instance invoke-direct const-string invoke-virtual move-result-object " +
			"invoke-virtual move-result-object invoke-virtual move-result-object return-object",
	}
	for _, m := range cf.Methods {
		quieren, esMedido := medidos[m.Name]
		if !esMedido {
			continue
		}
		ci, err := td.codeItemDe(cf, m)
		if err != nil {
			t.Errorf("%s: %v", m.Name, err)
			continue
		}
		lista, err := desmontaPalabras(ci.insns)
		if err != nil {
			t.Errorf("%s: nuestras palabras no se leen: %v", m.Name, err)
			continue
		}
		nombres := []string{}
		for _, instr := range lista {
			nombres = append(nombres, strings.Fields(instr)[0])
		}
		if sale := strings.Join(nombres, " "); sale != quieren {
			t.Errorf("%s:\n  nuestro: %s\n  medido:  %s", m.Name, sale, quieren)
		}
		delete(medidos, m.Name)
	}
	for nombre := range medidos {
		t.Errorf("%s no salio entre los metodos de la clase (%v)", nombre, nombresMetodos(cf.Methods))
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

// TestLaFramePoneLosArgumentosArriba: la numeración que ART exige (§56). Con mas locales que
// argumentos, los `ins` tienen que ser los ULTIMOS registros de la frame; el aparato rechazaba
// `this` en v6 cuando el item decia registers=10 e ins=2.
func TestLaFramePoneLosArgumentosArriba(t *testing.T) {
	m := classMethod{HasCode: true, Name: "onClick", Desc: "(Landroid/view/View;)V",
		LocalCount: 4, ArgCount: 2, MaxStack: 2}
	p := &traduce{m: m, locales: m.LocalCount, base: 6}
	// this, v son los dos primeros huecos JVM; cero y k son locales del cuerpo
	if got := p.local(0); got != 8 {
		t.Errorf("`this` sale en v%d, y con registers=10 e ins=2 ART lo espera en v8", got)
	}
	if got := p.local(1); got != 9 {
		t.Errorf("el argumento sale en v%d, se esperaba v9", got)
	}
	a, b := p.local(2), p.local(3)
	if a != 6 || b != 7 {
		t.Errorf("los locales del cuerpo salen en v%d y v%d, se esperaban v6 y v7 (por debajo de los argumentos)", a, b)
	}
	// y cuando los huecos son exactamente los argumentos la cuenta es la de siempre: identidad, que
	// es lo que dejaron medido los párrafos 50 a 54
	estrecho := &traduce{m: classMethod{ArgCount: 2}, locales: 2, base: 6}
	if estrecho.local(0) != 6 || estrecho.local(1) != 7 {
		t.Errorf("con locales == argumentos la numeración no puede moverse: salen v%d y v%d", estrecho.local(0), estrecho.local(1))
	}
}
