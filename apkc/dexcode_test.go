package main

import (
	"sort"
	"strings"
	"testing"
)

// TestReadClassDataSplitsTheSixClassesTheWayJavacLeftThem: los numeros de esta tabla son los que
// produce javac sobre el proyecto del studio (constructores directos, metodos de instancia
// virtuales, los tres ids estaticos de R y el this$0 sintetico de la clase anonima). El oraculo es
// javap -v: la forma de cada clase esta ahi, y el class_data_item tiene que contar lo mismo.
func TestReadClassDataSplitsTheSixClassesTheWayJavacLeftThem(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("reading the class data: %v", err)
	}
	if len(porClase) != 6 {
		t.Fatalf("%d clases con data, se esperan 6", len(porClase))
	}

	esperado := map[string][4]int{ // directos, virtuales, campos estaticos, campos de instancia
		"Lcom/hola/apkcstudio/MainActivity;":   {1, 2, 0, 0},
		"Lcom/hola/apkcstudio/MainActivity$1;": {1, 1, 0, 1},
		"Lcom/hola/apkcstudio/R;":              {1, 0, 0, 0},
		"Lcom/hola/apkcstudio/R$id;":           {1, 0, 1, 0},
		"Lcom/hola/apkcstudio/R$layout;":       {1, 0, 1, 0},
		"Lcom/hola/apkcstudio/R$style;":        {1, 0, 1, 0},
	}
	directos, virtuales, estaticos := 0, 0, 0
	for clase, q := range esperado {
		cd, ok := porClase[clase]
		if !ok {
			t.Errorf("%s has no class_data_item", clase)
			continue
		}
		if got := [4]int{len(cd.directs), len(cd.virtuals), len(cd.staticFields), len(cd.instanceFields)}; got != q {
			t.Errorf("%s: direct/virtual/static/instance = %v, javap says %v", clase, got, q)
		}
		directos += len(cd.directs)
		virtuales += len(cd.virtuals)
		estaticos += len(cd.staticFields)
	}
	if directos+virtuales != 9 {
		t.Errorf("%d directos y %d virtuales: javap cuenta 9 metodos con Code", directos, virtuales)
	}
	if estaticos != 3 {
		t.Errorf("%d campos estaticos, y las tres constantes de R son pulsar, main y AppTheme", estaticos)
	}
}

// TestCodeItemsAddUpToTheFile es la comprobacion de lo medido: los nueve code_item del fixture,
// ordenados por su offset, tienen que dejar espacio exacto para su cabecera de dieciseis bytes y
// sus palabras, sin solaparse y terminando antes de donde el map_list dice que empieza el area de
// debug_info. Y la primera palabra de cada constructor tiene que ser 0x70, que es invoke-direct: la
// prueba directa de que ahi empiezan las instrucciones. Las cuatro palabras bajas se comparan con lo
// que javap midio sobre los mismos .class (§17), no con lo que asegura un header sin leer el fichero.
func TestCodeItemsAddUpToTheFile(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}

	type item struct {
		off    uint32
		clase  string
		nombre string
		ci     codeItem
	}
	lista := []item{}
	for _, c := range d.classes {
		cd := porClase[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			nombre := "?"
			if int(m.methodIdx) < len(d.methods) {
				nombre = d.methods[m.methodIdx].name
			}
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatalf("%s %s: %v", c.name, nombre, err)
			}
			lista = append(lista, item{off: m.codeOff, clase: c.name, nombre: nombre, ci: ci})
		}
	}
	if len(lista) != 9 {
		t.Fatalf("%d code_item, javap cuenta 9 metodos con cuerpo", len(lista))
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].off < lista[j].off })

	// el area de codigo termina donde empieza la siguiente seccion del mapa: los nueve code_item
	// tienen que balazarla sin pasarse. Con los tipos corregidos del map_list, 0x2003 es
	// debug_info_item (1160) y 0x2000 class_data_item; antes tenia esos dos valores cruzados, y
	// un mapa mal etiquetado hace decir cualquier cosa de donde vive cada estructura
	finDeZona := func(kind dexKind) uint32 {
		for _, e := range d.maps {
			if e.kind == kind {
				return e.offset
			}
		}
		return 0
	}
	trasElCodigo := finDeZona(0x2003)
	if trasElCodigo == 0 {
		t.Fatal("the map_list does not name the section that follows the code")
	}

	// el area de debug_info tal como la declara el mapa: desde su offset hasta el de la siguiente
	// seccion. Es la que tiene que contener el u32 de +8 de cada code_item, y la unica forma de
	// saber que ese campo ES un offset de debug es tener un area donde caiga, no fiarse del formato
	var inicioDebug, finDebug uint32
	ordenadas := append([]dexMapEntry{}, d.maps...)
	sort.Slice(ordenadas, func(i, j int) bool { return ordenadas[i].offset < ordenadas[j].offset })
	for i, e := range ordenadas {
		if e.kind == 0x2003 {
			inicioDebug = e.offset
			finDebug = uint32(len(d.raw))
			if i+1 < len(ordenadas) {
				finDebug = ordenadas[i+1].offset
			}
		}
	}
	if inicioDebug == 0 {
		t.Fatal("the map_list has no debug_info_item area")
	}
	if n := finDeZona(0x2001); n != 884 {
		t.Errorf("the code_item area starts at %d, the fixture's first code_off is 884", n)
	}

	// lo que javap midio sobre estos mismos .class: args_size, locales, max_stack y bytes de Code
	medido := map[[2]string][4]int{
		{"Lcom/hola/apkcstudio/MainActivity$1;", "<init>"}:            {2, 2, 2, 10},
		{"Lcom/hola/apkcstudio/MainActivity$1;", "onMenuItemClick"}:   {2, 2, 3, 15},
		{"Lcom/hola/apkcstudio/MainActivity;", "<init>"}:              {1, 1, 1, 5},
		{"Lcom/hola/apkcstudio/MainActivity;", "onCreate"}:            {2, 2, 2, 12},
		{"Lcom/hola/apkcstudio/MainActivity;", "onCreateOptionsMenu"}: {2, 2, 4, 24},
		{"Lcom/hola/apkcstudio/R$id;", "<init>"}:                      {1, 1, 1, 5},
		{"Lcom/hola/apkcstudio/R$layout;", "<init>"}:                  {1, 1, 1, 5},
		{"Lcom/hola/apkcstudio/R$style;", "<init>"}:                   {1, 1, 1, 5},
		{"Lcom/hola/apkcstudio/R;", "<init>"}:                         {1, 1, 1, 5},
	}
	debugDistintos := map[uint32]int{}

	totalPalabras := 0
	for i, it := range lista {
		fin := uint64(it.off) + uint64(it.ci.tamanoEnBytes())
		siguiente := uint64(trasElCodigo)
		if i+1 < len(lista) {
			siguiente = uint64(lista[i+1].off)
		}
		if fin > siguiente {
			t.Errorf("%s %s at %d: %d bytes of header+code reach %d, and the next item starts at %d",
				it.clase, it.nombre, it.off, it.ci.tamanoEnBytes(), fin, siguiente)
			continue
		}
		if it.ci.insnsSize == 0 {
			t.Errorf("%s %s has no code words", it.clase, it.nombre)
		}
		// y el hueco hasta el siguiente item tiene que ser de relleno, no de datos: si las
		// palabras empezaran en otro lado, el tamano no cuadraria con el vecino
		if hueco := siguiente - fin; hueco > 3 {
			t.Errorf("%s %s at %d: %d bytes to the next item, more than alignment padding",
				it.clase, it.nombre, it.off, hueco)
		}
		q, ok := medido[[2]string{it.clase, it.nombre}]
		if !ok {
			t.Errorf("%s %s is not in the javap table", it.clase, it.nombre)
			continue
		}
		args, locales, pila, bytesCode := q[0], q[1], q[2], q[3]
		// +2 == args_size: la unica coincidencia exacta entre las cuatro palabras bajas
		if int(it.ci.insSize) != args {
			t.Errorf("%s %s: ins_size %d, javap args_size %d", it.clase, it.nombre, it.ci.insSize, args)
		}
		// +0: nunca por debajo de lo que javap cuenta como locales, ni de sus propias palabras
		if int(it.ci.registersSize) < locales {
			t.Errorf("%s %s: %d registers for %d javap locals", it.clase, it.nombre, it.ci.registersSize, locales)
		}
		if it.ci.registersSize < it.ci.insSize || it.ci.registersSize < it.ci.outsSize {
			t.Errorf("%s %s: registers_size %d does not cover ins %d / outs %d",
				it.clase, it.nombre, it.ci.registersSize, it.ci.insSize, it.ci.outsSize)
		}
		// +4: la llamada mas ancha, y la pila de la JVM tiene que haber sostenido esos mismos
		// argumentos antes del invoke, asi que nunca puede superar max_stack
		if int(it.ci.outsSize) > pila {
			t.Errorf("%s %s: outs_size %d, javap max_stack %d", it.clase, it.nombre, it.ci.outsSize, pila)
		}
		if it.ci.triesSize != 0 {
			t.Errorf("%s %s: tries_size %d, y javap no ve exception table en ninguno", it.clase, it.nombre, it.ci.triesSize)
		}
		// +8: dentro del area debug_info_item del mapa, que es lo que lo declara offset de debug
		if it.ci.debugInfoOff < inicioDebug || uint64(it.ci.debugInfoOff) >= uint64(finDebug) {
			t.Errorf("%s %s: debug_info_off %d is outside the map's debug area [%d,%d)",
				it.clase, it.nombre, it.ci.debugInfoOff, inicioDebug, finDebug)
		}
		debugDistintos[it.ci.debugInfoOff]++
		// +12: cada instruccion JVM se vuelve al menos una palabra, y las hay de dos, asi que
		// 2·palabras no puede quedar por debajo de los bytes de Code del .class
		if 2*int(it.ci.insnsSize) < bytesCode {
			t.Errorf("%s %s: %d code words (%d bytes) for a %d byte Code attribute",
				it.clase, it.nombre, it.ci.insnsSize, 2*it.ci.insnsSize, bytesCode)
		}
		if it.clase == "Lcom/hola/apkcstudio/R$id;" || it.clase == "Lcom/hola/apkcstudio/R;" {
			// los cuatro constructores sin nada que inicializar empiezan por invoke-direct, y
			// eso es lo que fijo donde estan las palabras (medida, no convencion)
			if op := byte(it.ci.insns[0] & 0xff); op != 0x70 {
				t.Errorf("%s: the constructor starts with opcode %#02x, not invoke-direct (0x70)", it.clase, op)
			}
		}
		totalPalabras += int(it.ci.insnsSize)
	}
	// nueve items con area de debug, siete offsets distintos: d8 comparte el debug_info de los
	// constructores identicos (los cuatro R y el de MainActivity repiten offset). El escritor tiene
	// que poder emitir lo mismo o el dex saldra mas grande que el de d8 sin estar mal
	if len(debugDistintos) != 7 {
		t.Errorf("%d distinct debug_info offsets for 9 methods, the fixture has 7", len(debugDistintos))
	}
	// 47 instrucciones JVM (docs/evidencia.md §14) y cada una de ellas en una o mas palabras: el
	// total de palabras tiene que ser al menos ese numero
	if totalPalabras < 47 {
		t.Errorf("%d palabras de codigo en total para 47 instrucciones JVM", totalPalabras)
	}
}

func TestReadClassDataRefusesDataThatRunsOffTheFile(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	// una clase que apunta al ultimo byte: el cuarto uleb no cabe y eso es un error, no un 0
	d.classes[0].dataOff = uint32(len(d.raw) - 1)
	if _, err := d.methodsDeCadaClase(); err == nil {
		t.Error("a class_data_item at the last byte of the file was accepted")
	} else if !strings.Contains(err.Error(), "MainActivity") {
		t.Errorf("the refusal does not say which class: %v", err)
	}
	if _, err := d.readCodeItem(uint32(len(d.raw) - 2)); err == nil {
		t.Error("a code_item that does not fit was accepted")
	}
	if ci, err := d.readCodeItem(0); err != nil || len(ci.insns) != 0 {
		t.Errorf("no code (offset 0) should read as empty, got %v / %v", ci, err)
	}
}
