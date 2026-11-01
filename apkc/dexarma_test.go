package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

// armaElFixture: las tablas y el dex completo de las seis clases del studio, con las cuentas que
// d8 usó como orden de internado (orderTables) y no con otro.
func armaElFixture(t *testing.T) ([]byte, *dexFile) {
	t.Helper()
	td, clases := tablaDeLasSeis(t)
	b, err := td.armaDex(clases)
	if err != nil {
		t.Fatalf("assembling the dex: %v", err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("the reader refuses what this fork wrote: %v", err)
	}
	return b, d
}

// TestElDexArmadoSeLeeConsigoMismo es la prueba de ida y vuelta: lo que se escribe lo lee el lector
// del §16, con las mismas cuentas de objeto que el fichero de d8 y con la firma y el checksum
// calculados sobre lo que quedó escrito.
func TestElDexArmadoSeLeeConsigoMismo(t *testing.T) {
	b, d := armaElFixture(t)
	if d.version != "035" {
		t.Errorf("version %q", d.version)
	}
	if len(b) < 1000 {
		t.Errorf("only %d bytes came out", len(b))
	}
	t.Logf("nuestro dex: %d bytes; el de d8: %d", len(b), len(fixtureDex(t)))
	if err := d.Integrity(); err != nil {
		t.Errorf("the signature or the checksum of what we wrote does not verify: %v", err)
	}
	cuentas := map[dexKind]int{0x0001: 52, 0x0002: 23, 0x0003: 9, 0x0004: 4, 0x0005: 17, 0x0006: 6,
		0x2003: 7, 0x2004: 8, 0x1003: 6, 0x2006: 5, 0x1001: 8}
	for k, q := range cuentas {
		got := 0
		for _, e := range d.maps {
			if e.kind == k {
				got = int(e.size)
			}
		}
		// 52 cadenas y no las 53 de d8: la que falta es su propia marca de origen, el string
		// ~~D8{"backend":"dex",...} que la herramienta se escribe a si misma. Copiarla seria
		// mentir sobre quien compilo (docs/evidencia.md §22)
		//
		// Las tres cuentas de anotaciones (8 items, 6 sets, 5 directorios) son **las mismas que
		// declara el mapa del fichero de d8**, medida entrada por entrada en el parrafo 59: los
		// items identicos se comparten entre destinos, como alli.
		if got != q {
			t.Errorf("section %s has %d items, expected %d", k, got, q)
		}
	}
	// el map_list declara las quince áreas que escribimos (las doce del §56 más las tres de
	// anotaciones del §59), la cabecera y el propio mapa: diecisiete entradas, las mismas que d8
	if len(d.maps) != 17 {
		t.Errorf("%d map_list entries, expected 17 (header, fifteen areas, the map itself)", len(d.maps))
	}
	n := 0
	for _, e := range d.maps {
		if e.kind == 0x2001 {
			n = int(e.size)
		}
	}
	if n != 9 {
		t.Errorf("the map says %d code items, javap counted 9 methods with code", n)
	}
}

// TestLasCadenasDelDexArmadoSonLasDeD8: cada string_data_item byte a byte, recortado con los
// offsets que declara string_ids en cada fichero. Es el claim del §14-B (2G-B) ya dentro de un
// fichero completo.
func TestLasCadenasDelDexArmadoSonLasDeD8(t *testing.T) {
	_, d := armaElFixture(t)
	d8, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	sinMarca := []string{}
	for _, s := range d8.strings {
		if strings.HasPrefix(s, "~~") {
			continue // la marca de origen de d8: no la copiamos (§22)
		}
		sinMarca = append(sinMarca, s)
	}
	if len(d.strings) != len(sinMarca) {
		t.Fatalf("%d strings against %d", len(d.strings), len(sinMarca))
	}
	// item a item: recortar "el area" entre dos entradas del mapa no vale, porque d8 deja detras
	// cosas que no son cadenas
	for i := range d.strings {
		if d.strings[i] != sinMarca[i] {
			t.Errorf("string #%d: %q, d8 has %q", i, d.strings[i], sinMarca[i])
			break
		}
		a, e := itemCadena(d, i), itemCadenaEn(d8, i, sinMarca)
		if !bytes.Equal(a, e) {
			t.Errorf("string_data #%d (%q): %s, d8 has %s", i, d.strings[i],
				hex.EncodeToString(a), hex.EncodeToString(e))
			break
		}
	}
}

// itemCadenaEn es itemCadena sobre el fichero de d8, buscando el indice real de la cadena: su
// tabla tiene la marca de origen, y los offsets de string_ids siguen el orden de la tabla entera.
func itemCadenaEn(d *dexFile, i int, sinMarca []string) []byte {
	real := 0
	for j, s := range d.strings {
		if s == sinMarca[i] {
			real = j
			break
		}
	}
	return itemCadena(d, real)
}

// itemCadena corta UN string_data_item: desde el offset que declara string_ids hasta su terminador
// cero incluido. El cero del final se puede buscar byte a byte porque el MUTF-8 del formato nunca
// escribe un 00 suelto (U+0000 sale C0 80), que es justamente la razon de ser del modificador.
// Cortar "hasta el siguiente offset" no vale para la ultima cadena del area: detras viene otra seccion.
func itemCadena(d *dexFile, i int) []byte {
	base := d.offDe(0x0001)
	inicio := uint32(binary.LittleEndian.Uint32(d.raw[base+4*uint32(i):]))
	// el item empieza con el uleb128 del numero de unidades utf-16: se salta esa cabecera antes de
	// buscar el terminador, porque un valor de tamano como 0x20 no es un cero pero un uleb de dos
	// bytes podria empezar por un byte que no es cero; el escaneo real empieza detras
	k := int(inicio)
	for shift := uint(0); ; shift += 7 {
		c := d.raw[k]
		k++
		if c&0x80 == 0 {
			break
		}
		_ = shift
	}
	for ; k < len(d.raw); k++ {
		if d.raw[k] == 0 {
			return d.raw[inicio : uint32(k)+1]
		}
	}
	return d.raw[inicio:]
}

// TestLasTablasFijasSonLasDeD8: type_ids, field_ids y method_ids byte a byte (no nombran offsets,
// solo indices), y proto_ids con los parameters_off rebasados a la base de cada fichero.
func TestLasTablasFijasSonLasDeD8(t *testing.T) {
	_, d := armaElFixture(t)
	d8, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []dexKind{0x0002, 0x0004, 0x0005} {
		ours, theirs := d.seccion(k), d8.seccion(k)
		if !bytes.Equal(ours, theirs) {
			t.Errorf("section %s is %d bytes and does not match d8's", k, len(ours))
		}
	}
	ours, theirs := d.seccion(0x0003), d8.seccion(0x0003)
	if len(ours) != len(theirs) {
		t.Fatalf("proto_ids: %d bytes against %d", len(ours), len(theirs))
	}
	for i := 0; i+12 <= len(ours); i += 12 {
		for j := 0; j < 8; j += 4 {
			a, e := binary.LittleEndian.Uint32(ours[i+j:]), binary.LittleEndian.Uint32(theirs[i+j:])
			if a != e {
				t.Errorf("proto_ids #%d field at +%d: we wrote %d, d8 has %d", i/12, j, a, e)
			}
		}
		a := binary.LittleEndian.Uint32(ours[i+8:])
		e := binary.LittleEndian.Uint32(theirs[i+8:])
		switch {
		case a == 0 && e == 0:
		case a == 0 || e == 0:
			t.Errorf("proto_ids #%d: parameters_off %d against d8's %d, one of them is zero", i/12, a, e)
		default:
			// se compara solo que ambos caigan dentro de su area de type_list: el desplazamiento
			// concreto puede diferir porque aqui las listas identicas se comparten, y eso es un
			// dato de tamano, no un error
			if a < d.offDe(0x1001) || int(a) >= len(d.raw) {
				t.Errorf("proto_ids #%d: parameters_off %d is not inside our type_list area", i/12, a)
			}
			if e < d8.offDe(0x1001) || e >= d8.offDe(0x2002) {
				t.Errorf("proto_ids #%d: d8's parameters_off %d is not inside its type_list area", i/12, e)
			}
		}
	}
}

// TestLosClassDefsComparanAccesoYOrigenConLosDeD8: type_idx, access_flags, super, source_file y
// el orden de la tabla. annotations_off y static_values_off tambien se emiten ya (§55 y §59) y se
// comprueban por su lado, en el volcado de anotaciones y en las cuentas del class_data.
func TestLosClassDefsComparanAccesoYOrigenConLosDeD8(t *testing.T) {
	_, d := armaElFixture(t)
	d8, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.classes) != len(d8.classes) {
		t.Fatalf("%d class_defs against %d", len(d.classes), len(d8.classes))
	}
	for i := range d.classes {
		a, e := d.classes[i], d8.classes[i]
		if a.name != e.name {
			t.Errorf("class_def #%d: %s against d8's %s", i, a.name, e.name)
			continue
		}
		if a.super != e.super {
			t.Errorf("%s: super %s, d8 has %s", a.name, a.super, e.super)
		}
		if a.access != e.access {
			t.Errorf("%s: access_flags %#x, d8 wrote %#x", a.name, a.access, e.access)
		}
	}
}

// TestLosCodeItemsEscritosSeLeenComoFueronEscritos: cada cuerpo del fichero nuestro tiene que
// desmontar en las mismas palabras que el traductor emiti6, y la cabecera tiene que casar con lo
// que javap midio (ins_size) y con su propia cuenta de palabras.
func TestLosCodeItemsEscritosSeLeenComoFueronEscritos(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	b, d := armaElFixture(t)
	inicioDep := d.offDe(0x2003)
	leidos := map[[2]string]codeItem{}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.classes {
		cd := porClase[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			nombre := d.methods[m.methodIdx].name
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatalf("%s %s: %v", c.name, nombre, err)
			}
			leidos[[2]string{c.name, nombre}] = ci
			if ci.triesSize != 0 {
				t.Errorf("%s %s: the fork emits no tries", c.name, nombre)
			}
			// desde el §56 cada cuerpo anuncia su debug_info_item: el offset tiene que caer dentro
			// del area que declara el mapa, que es como la recorre ART
			if ci.debugInfoOff < inicioDep || int(ci.debugInfoOff) >= len(b) {
				t.Errorf("%s %s: debug_info_off %d is not inside the debug area (starts at %d, file is %d)",
					c.name, nombre, ci.debugInfoOff, inicioDep, len(b))
			}
			// el area de codigo no puede pasar de donde el mapa dice que empieza lo de despues
			if m.codeOff+uint32(ci.tamanoEnBytes()) > uint32(len(b)) {
				t.Errorf("%s %s: the item runs past the end of the file", c.name, nombre)
			}
		}
	}
	if len(leidos) != 9 {
		t.Fatalf("%d code items read back, javap counted 9", len(leidos))
	}
	for _, cf := range clases {
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			ci, err := td.codeItemDe(cf, m)
			if err != nil {
				t.Fatal(err)
			}
			k := [2]string{cf.Name, m.Name}
			got, ok := leidos[k]
			if !ok {
				t.Errorf("%s%s never reached the file", cf.Name, m.Name)
				continue
			}
			if len(got.insns) != len(ci.insns) {
				t.Errorf("%s%s: %d words read back, %d written", cf.Name, m.Name, len(got.insns), len(ci.insns))
				continue
			}
			for i := range ci.insns {
				if got.insns[i] != ci.insns[i] {
					t.Errorf("%s%s word %d: %#04x read back, %#04x written", cf.Name, m.Name, i, got.insns[i], ci.insns[i])
					break
				}
			}
			if got.insSize != ci.insSize || got.outsSize != ci.outsSize || got.registersSize != ci.registersSize {
				t.Errorf("%s%s: header read back as %d/%d/%d, written %d/%d/%d", cf.Name, m.Name,
					got.registersSize, got.insSize, got.outsSize, ci.registersSize, ci.insSize, ci.outsSize)
			}
		}
	}
}

// TestElClassDataEscritoDiceLosMismosMiembros: los indices de campo y metodo, sus access flags y
// las constantes de R, comparados con lo que el lector encuentra en el fichero de d8. Es la cuenta
// que el sistema usa para saber que pertenece a que clase.
func TestElClassDataEscritoDiceLosMismosMiembros(t *testing.T) {
	_, d := armaElFixture(t)
	d8, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	nuestros, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("reading our own class_data: %v", err)
	}
	aLosDeEllos, err := d8.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}
	for clase, cd := range nuestros {
		e := aLosDeEllos[clase]
		if len(cd.directs) != len(e.directs) || len(cd.virtuals) != len(e.virtuals) {
			t.Errorf("%s: %d direct / %d virtual, d8 has %d / %d", clase,
				len(cd.directs), len(cd.virtuals), len(e.directs), len(e.virtuals))
			continue
		}
		for i, m := range cd.directs {
			if m.methodIdx != e.directs[i].methodIdx || m.accessFlags != e.directs[i].accessFlags {
				t.Errorf("%s direct #%d: idx %d flags %#x, d8 has %d %#x", clase, i,
					m.methodIdx, m.accessFlags, e.directs[i].methodIdx, e.directs[i].accessFlags)
			}
			// el code_off tiene que apuntar a un code_item que existe y que cuadra con el mapa
			if m.codeOff == 0 || int(m.codeOff)+16 > len(d.raw) {
				t.Errorf("%s direct #%d: code_off %d does not point inside the file", clase, i, m.codeOff)
			}
		}
		for i, m := range cd.virtuals {
			if m.methodIdx != e.virtuals[i].methodIdx || m.accessFlags != e.virtuals[i].accessFlags {
				t.Errorf("%s virtual #%d: idx %d flags %#x, d8 has %d %#x", clase, i,
					m.methodIdx, m.accessFlags, e.virtuals[i].methodIdx, e.virtuals[i].accessFlags)
			}
		}
		if len(cd.staticFields) != len(e.staticFields) {
			t.Errorf("%s: %d static fields, d8 has %d", clase, len(cd.staticFields), len(e.staticFields))
			continue
		}
		for i, f := range cd.staticFields {
			g := e.staticFields[i]
			if f.fieldIdx != g.fieldIdx || f.accessFlags != g.accessFlags {
				t.Errorf("%s static field #%d: idx %d flags %#x, d8 has %d %#x",
					clase, i, f.fieldIdx, f.accessFlags, g.fieldIdx, g.accessFlags)
			}
			if f.valueType != g.valueType || f.value != g.value {
				t.Errorf("%s static field #%d: value type %#x value %d, d8 has %#x %d",
					clase, i, f.valueType, f.value, g.valueType, g.value)
			}
		}
	}
}

// TestElAreaDeListasEsLaDeD8: el area de type_list byte a byte con la de `d8`, y con la cuenta que él
// declara. Es la regla que estaba mal escrita aqui: las listas se internan **por contenido**, sin
// distinguir los parametros de un proto de las interfaces de una clase, asi que en el fixture la lista
// `[Landroid/view/MenuItem$OnMenuItemClickListener;]` se escribe UNA vez y la apuntan las dos cosas
// (medido en el parrafo 60; con el prefijo de origen salian nueve listas contra sus ocho).
func TestElAreaDeListasEsLaDeD8(t *testing.T) {
	_, d := armaElFixture(t)
	d8, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	mias, suyas := d.seccion(0x1001), d8.seccion(0x1001)
	if len(mias) != len(suyas) || !bytes.Equal(mias, suyas) {
		t.Errorf("el area de type_list mide %d bytes y la de d8 %d", len(mias), len(suyas))
	}
	cuantas := func(f *dexFile) int {
		for _, e := range f.maps {
			if e.kind == 0x1001 {
				return int(e.size)
			}
		}
		return -1
	}
	if cuantas(d) != 8 || cuantas(d) != cuantas(d8) {
		t.Errorf("%d listas declaradas nosotros y %d d8; la cuenta medida es 8", cuantas(d), cuantas(d8))
	}
	// y la comparticion se ve en los apuntadores: el proto y el class_def van al MISMO offset absoluto.
	// Se compara el valor absoluto, no la posicion relativa, porque la lista compartida cae en el
	// primer byte del area y su offset relativo es cero
	listaDe := func(f *dexFile, off int) uint32 { return dexLittleEndian.Uint32(f.raw[off:]) }
	protoOff, claseOff := uint32(0), uint32(0)
	for i := range d.protos {
		if strings.Contains(descriptorDe(d.protos[i]), "OnMenuItemClickListener") {
			protoOff = listaDe(d, int(d.offDe(0x0003))+12*i+8)
		}
	}
	for i := range d.classes {
		if d.classes[i].name == "Lcom/hola/apkcstudio/MainActivity$1;" {
			claseOff = listaDe(d, int(d.offDe(0x0006))+32*i+12)
		}
	}
	if protoOff == 0 || protoOff != claseOff || protoOff != d.offDe(0x1001) {
		t.Errorf("la lista compartida no esta compartida: proto en %d, interfaces en %d, area en %d",
			protoOff, claseOff, d.offDe(0x1001))
	}
}

// TestLaMarcaDeOrigenDeD8NoSeCopia: la unica cadena que falta en nuestra tabla es la firma con la que
// `d8` se nombra a si mismo, y se descarta **a proposito** (medida en el parrafo 60): su contenido es
//
//	~~D8{"backend":"dex","compilation-mode":"release","has-checksums":false,"min-api":21,
//	    "sha-1":"<los 40 hex del fichero>","version":"9.2.4-dev"}
//
// o sea que nombran a otra herramienta, con su versión y con un resumen del fichero que ella escribió.
// Copiarla sería firmar una mentira y además un resumen que no corresponde a nuestros bytes. Lo que se
// comprueba aquí es que no la copiemos, que sea efectivamente la única cadena de más y que ninguna
// estructura del fichero la nombre: ni un tipo, ni un shorty, ni un nombre de campo o método, ni un
// `source_file_idx` (los elementos de las anotaciones se midieron en el §59 y tampoco la nombran).
func TestLaMarcaDeOrigenDeD8NoSeCopia(t *testing.T) {
	_, d := armaElFixture(t)
	d8, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	marca := -1
	for i, s := range d8.strings {
		if strings.HasPrefix(s, "~~") {
			if marca >= 0 {
				t.Fatalf("hay dos marcas: #%d y #%d", marca, i)
			}
			marca = i
		}
	}
	if marca < 0 {
		t.Fatal("el fixture de d8 ya no trae la marca de origen: hay que reescribir esta prueba")
	}
	if !strings.Contains(d8.strings[marca], `"compilation-mode":"release"`) {
		t.Errorf("la marca medida no dice release: %s", d8.strings[marca])
	}
	for _, s := range d.strings {
		if strings.HasPrefix(s, "~~") {
			t.Errorf("nuestra tabla copia una marca de otro compilador: %q", s)
		}
	}
	if len(d8.strings) != len(d.strings)+1 {
		t.Errorf("%d cadenas las nuestras contra %d de d8: la marca debería ser la única diferencia",
			len(d.strings), len(d8.strings))
	}
	// ninguna columna de las tablas de índices apunta a ella
	nombrados := map[uint32]bool{}
	u32 := func(kind dexKind, i uint32, en int) uint32 {
		base := d8.offDe(kind)
		if base == 0 {
			t.Fatalf("el mapa no declara %s", kind)
		}
		stride := dexItemStride(kind)
		if stride == 0 {
			t.Fatalf("%s no es una tabla de tamano fijo", kind)
		}
		return dexLittleEndian.Uint32(d8.raw[base+i*uint32(stride)+uint32(en):])
	}
	for i := uint32(0); i < uint32(len(d8.types)); i++ {
		nombrados[u32(0x0002, i, 0)] = true
	}
	for i := uint32(0); i < uint32(len(d8.protos)); i++ {
		nombrados[u32(0x0003, i, 0)] = true
	}
	for i := uint32(0); i < uint32(len(d8.fields)); i++ {
		nombrados[u32(0x0004, i, 4)] = true
	}
	for i := uint32(0); i < uint32(len(d8.methods)); i++ {
		nombrados[u32(0x0005, i, 4)] = true
	}
	for i := uint32(0); i < uint32(len(d8.classes)); i++ {
		nombrados[u32(0x0006, i, 16)] = true
	}
	if nombrados[uint32(marca)] {
		t.Errorf("la marca #%d sí es nombrada por alguna tabla: no es una cadena huérfana", marca)
	}
	// y su string_data_item existe, con su uleb del tamaño en unidades utf-16
	if item := d8.stringItemBytes(marca); len(item) < 2 || item[len(item)-1] != 0 {
		t.Errorf("el item de la marca no tiene pinta de string_data_item: % x", item)
	}
}
