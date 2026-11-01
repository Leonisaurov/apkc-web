package main

// Las pruebas del §56: el debug_info_item. El oráculo es doble, como en resto del dexer. Por un lado
// las palabras que escribió `d8` sobre una sonda medida en el aparato (`dexdump -d`, Android 15):
//
//	probe/Sonda.java, javac sin `-g`
//	  <init>()V                 0f 00 0e 39 4d 4b 00             lineas 15, 13, 16, 17
//	  cuenta(II)V               06 02 00 00 0e 1e 2d 3c 1f 00    lineas 6, 7, 8, 9, 11
//	  arma(Ljava/lang/String;)V 14 01 00 0e 5a 3c 4b 69 00        lineas 20, 21, 22, 23, 24
//
// Por otro, el decodificador de abajo, que es la sucesión de estados que ART recorre: si lo que
// escribimos se lee con las mismas (pc, línea) que trajo el `.class`, el item es correcto para quien
// lo consume.

import "testing"

// posicionesDebug: decodifica un debug_info_item y devuelve las (unidad dalvik, línea) que anuncia.
func posicionesDebug(t *testing.T, b []byte, off uint32) ([][2]int, int) {
	t.Helper()
	uleb := func(p int) (int, int) {
		var v, sh int
		for {
			d := b[p]
			p++
			v |= int(d&0x7f) << sh
			if d&0x80 == 0 {
				return v, p
			}
			sh += 7
		}
	}
	sleb := func(p int) (int, int) {
		var v, sh int
		c := 0
		for {
			d := b[p]
			p++
			c++
			v |= int(d&0x7f) << sh
			sh += 7
			if d&0x80 == 0 {
				if c < 5 && d&0x40 != 0 {
					v |= -1 << sh
				}
				return v, p
			}
		}
	}
	linea, p := uleb(int(off))
	cuantos, p := uleb(p)
	for i := 0; i < cuantos; i++ {
		_, p = sleb(p) // nombre de argumento: se salta, aquí no se lee ninguno
	}
	pc := 0
	sal := [][2]int{}
	for p < len(b) {
		op := int(b[p])
		p++
		if op >= dbgFirstSpecial {
			adj := op - dbgFirstSpecial
			pc += adj / dbgLineRange
			linea += dbgLineBase + adj%dbgLineRange
			sal = append(sal, [2]int{pc, linea})
			continue
		}
		switch op {
		case dbgEndSequence:
			return sal, p
		case dbgAdvancePC:
			d, q := uleb(p)
			p = q
			pc += d
		case dbgAdvanceLine:
			d, q := sleb(p)
			p = q
			linea += d
		default:
			t.Fatalf("opcode %02x que esta fork no emite (0x03..0x09)", op)
		}
	}
	t.Fatalf("el item no termino con END_SEQUENCE")
	return sal, p
}

func TestItemDebugContraLasPalabrasDeD8(t *testing.T) {
	td := newDexTables()
	// La sucesión de pcs del `.class` es la que midió `javap -l` sobre la sonda, y la correspondencia
	// a unidades dalvik es la que `dexdump -d` del aparato leyó del fichero de `d8`.
	casos := []struct {
		nombre  string
		desc    string
		statico bool
		lineas  [][2]int
		donde   map[int]int
		quiere  []byte
	}{
		{"<init>", "()V", false, [][2]int{{0, 15}, {4, 13}, {10, 16}, {16, 17}},
			map[int]int{0: 0, 4: 3, 10: 7, 16: 11}, []byte{0x0f, 0x00, 0x0e, 0x39, 0x4d, 0x4b, 0x00}},
		// `cuenta` acaba una unidad mas adelante en nuestro cuerpo que en el de d8 (nuestra secuencia
		// de instrucciones difiere en un registro: parrafos 52 y 53), y eso se ve en el ultimo opcode
		{"cuenta", "(II)V", true, [][2]int{{0, 6}, {4, 7}, {8, 8}, {13, 9}, {17, 11}},
			map[int]int{0: 0, 4: 1, 8: 3, 13: 6, 17: 8},
			[]byte{0x06, 0x02, 0x00, 0x00, 0x0e, 0x1e, 0x2d, 0x3c, 0x2e, 0x00}},
		{"arma", "(Ljava/lang/String;)V", false, [][2]int{{0, 20}, {8, 21}, {14, 22}, {20, 23}, {28, 24}},
			map[int]int{0: 0, 8: 5, 14: 8, 20: 12, 28: 18},
			[]byte{0x14, 0x01, 0x00, 0x0e, 0x5a, 0x3c, 0x4b, 0x69, 0x00}},
	}
	for _, c := range casos {
		m := classMethod{Name: c.nombre, Desc: c.desc, HasCode: true, ArgCount: countArgs(c.desc)}
		if c.statico {
			m.Access = accStatic
		} else {
			m.ArgCount++ // `this`, que el item no cuenta
		}
		for _, l := range c.lineas {
			m.Lines = append(m.Lines, classLine{pc: l[0], line: l[1]})
		}
		got := td.itemDebug(m, c.donde)
		quiere := c.quiere
		if len(got) != len(quiere) {
			t.Fatalf("%s: %d bytes (% x), d8 escribio %d (% x)", c.nombre, len(got), got, len(quiere), quiere)
		}
		for i := range quiere {
			if got[i] != quiere[i] {
				t.Errorf("%s: byte %d es %02x, d8 escribio %02x (% x)", c.nombre, i, got[i], quiere[i], got)
			}
		}
		// y releido, tiene que dar las mismas (pc, linea) que anuncian esas palabras
		fuera := append(append([]byte(nil), got...), 0, 0, 0, 0)
		pos, _ := posicionesDebug(t, fuera, 0)
		if len(pos) != len(c.lineas) {
			t.Fatalf("%s: el decodificador conto %d posiciones, el .class tenia %d", c.nombre, len(pos), len(c.lineas))
		}
		for i, p := range pos {
			if p[0] != c.donde[c.lineas[i][0]] || p[1] != c.lineas[i][1] {
				t.Errorf("%s: posicion %d es (0x%04x, linea %d), se esperaba (0x%04x, linea %d)",
					c.nombre, i, p[0], p[1], c.donde[c.lineas[i][0]], c.lineas[i][1])
			}
		}
	}
}

func TestItemDebugFueraDelRangoDelEspecial(t *testing.T) {
	// el especial solo admite Δlínea en [-4, +10] y Δpc hasta 16 unidades: aqui hay un salto de 40
	// líneas, que obliga a ADVANCE_LINE (0x02) y luego el especial (0,0) que marca la posición
	td := newDexTables()
	m := classMethod{Name: "salto", Desc: "()V", HasCode: true, ArgCount: 1,
		Lines: []classLine{{pc: 0, line: 10}, {pc: 4, line: 50}}}
	got := td.itemDebug(m, map[int]int{0: 0, 4: 2})
	if len(got) == 0 {
		t.Fatal("no salio nada")
	}
	pos, _ := posicionesDebug(t, append(append([]byte(nil), got...), 0, 0, 0), 0)
	if len(pos) != 2 || pos[0] != [2]int{0, 10} || pos[1] != [2]int{2, 50} {
		t.Errorf("releido: %v (% x)", pos, got)
	}
	// el orden medido aqui: linea inicial (0x0a), recuento de argumentos (0), la primera posicion con
	// el especial (0,0), y a partir de ahi ADVANCE_LINE porque el salto de 40 lineas no cabe en el
	// especial
	if got[0] != 0x0a || got[2] != 0x0e || got[3] != dbgAdvanceLine {
		t.Errorf("no se emitió ADVANCE_LINE donde el delta se sale del rango: % x", got)
	}
}

func TestElAreaDeDepuracionSeDeduplicaYLaDeclaraElMapa(t *testing.T) {
	b, d := armaElFixture(t)
	area := d.seccion(0x2003)
	if len(area) == 0 {
		t.Fatal("el mapa no declara area de depuracion")
	}
	// nueve cuerpos, ocho items: dos métodos con la misma sucesión de líneas comparten offset, como
	// hace d8 (§20 conto siete items para nueve cuerpos en su fichero `--release`)
	cuantosItems := 0
	for _, e := range d.maps {
		if e.kind == 0x2003 {
			cuantosItems = int(e.size)
		}
	}
	if cuantosItems != 8 {
		t.Errorf("%d items declarados, se esperaban 8 deduplicando los nueve cuerpos", cuantosItems)
	}
	// y cada cuerpo del fichero anuncia una posición dentro del área que declara el mapa
	inicio := int(d.offDe(0x2003))
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}
	vistos := map[int]int{}
	leidas := 0
	for _, c := range d.classes {
		for _, m := range append(append([]encodedMethod{}, porClase[c.name].directs...), porClase[c.name].virtuals...) {
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatal(err)
			}
			dentro := int(ci.debugInfoOff) - inicio
			if dentro < 0 || dentro >= len(area) {
				t.Errorf("%s: debug_info_off %d fuera del area (%d..%d)", c.name, ci.debugInfoOff, inicio, inicio+len(area))
				continue
			}
			vistos[dentro]++
			pos, fin := posicionesDebug(t, b, ci.debugInfoOff)
			if fin > inicio+len(area) {
				t.Errorf("%s: el item termina en %d, pasado el final del area", c.name, fin)
			}
			if len(pos) == 0 {
				t.Errorf("%s: el item no anuncia ninguna posición", c.name)
			}
			leidas += len(pos)
		}
	}
	if len(vistos) != 8 {
		t.Errorf("%d offsets distintos anunciados por los cuerpos, el mapa declara %d", len(vistos), cuantosItems)
	}
	if leidas == 0 {
		t.Error("no se leyó ninguna linea de ningun cuerpo")
	}
}
