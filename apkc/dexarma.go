package main

// Ensamblado del classes.dex completo: cabecera, las cinco tablas de índices, class_defs, los
// code_item, las type_list, los class_data_item, el área de cadenas y el map_list, con la firma
// sha1 y el checksum adler32 al final (son lo último porque cubren todo lo que queda por encima).
//
// El orden de las areas es el que escribió d8 en el fixture (docs/evidencia.md §20), menos dos que
// este fork no emite y está dicho por qué: debug_info_item (las líneas de depuración, que necesitan
// su propia medición) y el bloque de anotaciones (Inner/Enclosing/MemberClass, que d8 sintetiza
// desde los atributos del .class). Sin ellas el dex es más pequeño y sigue siendo válido: el
// formato dice que un offset cero significa "no está".
//
// Los offsets se calculan en el orden de dependencia, sin iterar: el tamaño del área de código se
// sabe en cuanto se traducen los cuerpos, y con eso la base del área es fija, así que los
// code_off de los class_data_item (que son uleb) se escriben ya en absoluto. Lo único que depende
// del tamaño del class_data es el map_list, y va al final.

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash/adler32"
	"sort"
)

func putU16(b []byte, v uint16) []byte { return append(b, byte(v), byte(v>>8)) }

// tipoArrayCodificado es el kind con el que el map_list declara el area de los
// encoded_array_item, o sea las constantes de los campos estaticos que anuncia
// class_def_item.static_values_off. Vale 0x2005 y se mide en el fichero de `d8`: su mapa cierra
// con `0x2000 size=6 off=2659` y `0x2005 size=4 off=2746`. El area no pide alineacion: los cuatro
// arrays de `d8` empiezan en 2746, 2749, 2755 y 2761. Lo que si comprueba ART es el `map_list` —
// `d8` y nuestro lector no lo miran—, y por eso un kind equivocado aqui solo se nota en el
// aparato: con 0x2007 el verificador responde `Unknown map section type 2007`.
const tipoArrayCodificado dexKind = 0x2005

func putU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// armaDex escribe el fichero entero. Las tablas tienen que estar internadas y ordenadas
// (collectClasses + orderTables) y las clases, leídas de los .class.
func (td *dexTables) armaDex(clases []classFull) ([]byte, error) {
	if len(clases) == 0 {
		return nil, fmt.Errorf("there are no classes to dex")
	}
	if len(td.strings) == 0 || len(td.types) == 0 {
		return nil, fmt.Errorf("the tables were never interned or ordered")
	}
	for _, cf := range clases {
		if err := td.claseCabe(cf); err != nil {
			return nil, err
		}
	}

	// El `class_defs` va **ordenado por type_idx**, que es lo que exige el cargador de ART: busca la
	// clase con una búsqueda binaria sobre ese orden. Un fichero que lo respeta mal instala bien y
	// responde `ClassNotFoundException` al arrancar la actividad —medido en el teléfono sobre el apk
	// que escribió la pestaña (2026-11-01): `pm install` Success, `am start -W` Status ok, y en el
	// logcat `Unable to instantiate activity ComponentInfo{com.myapp/com.myapp.MainActivity}` con
	// `Didn't find class "com.myapp.MainActivity"`, mientras `strings` del dex muestra el nombre—.
	// `d8` no lo nota porque recorre la tabla linealmente, y por eso el oráculo del re-dex no lo
	// atrapaba. Ordenar aquí alinea además las áreas de datos con el orden de d8.
	ordenadas := append([]classFull(nil), clases...)
	sort.SliceStable(ordenadas, func(i, j int) bool {
		return td.typeIdx[ordenadas[i].Name] < td.typeIdx[ordenadas[j].Name]
	})
	clases = ordenadas

	// 1. areas cuyo contenido no depende de ningún offset absoluto
	cadenas, relCadenas := td.areaCadenas()
	listas, relListas, cuantasListas := td.areaListas(clases)
	codigo, relCodigo, cuerpos, err := td.areaCodigo(clases)
	if err != nil {
		return nil, err
	}

	// 2. bases, en el orden de d8
	off := int(dexHeaderSize)
	base := map[dexKind]int{}
	poner := func(kind dexKind, datos []byte, alineado int) {
		off = alignUp(off, alineado)
		base[kind] = off
		off += len(datos)
	}
	poner(0x0001, make([]byte, 4*len(td.strings)), 4)
	poner(0x0002, make([]byte, 4*len(td.types)), 4)
	poner(0x0003, make([]byte, 12*len(td.protos)), 4)
	poner(0x0004, make([]byte, 8*len(td.fields)), 4)
	poner(0x0005, make([]byte, 8*len(td.methods)), 4)
	poner(0x0006, make([]byte, 32*len(clases)), 4)
	poner(0x2001, codigo, 4)
	poner(0x1001, listas, 4)
	poner(0x2002, cadenas, 1)
	// los class_data_item dependen de los code_off absolutos, que ya se pueden saber
	absCodigo := map[[2]string]uint32{}
	for k, r := range relCodigo {
		absCodigo[k] = uint32(base[0x2001]) + r
	}
	datos, relDatos, arrays, relArrays, cuantosArrays := td.areaClassData(clases, absCodigo)
	poner(0x2000, datos, 1)
	// el encoded_array_item con las constantes de los campos estáticos, justo detrás del
	// class_data: es el orden en que lo dejó d8 (class_data 2659..2746, arrays desde 2746, sin
	// separación y sin alinear: el suyo cae en 2746, que no es múltiplo de cuatro)
	poner(tipoArrayCodificado, arrays, 1)

	// 3. ahora sí, los contenidos que nombran offsets absolutos
	stringIDs := []byte{}
	for _, r := range relCadenas {
		stringIDs = putU32(stringIDs, uint32(base[0x2002])+r)
	}
	typeIDs := []byte{}
	for _, t := range td.types {
		typeIDs = putU32(typeIDs, uint32(td.stringIdx[t]))
	}
	protoIDs := []byte{}
	for _, k := range td.protos {
		protoIDs = putU32(protoIDs, uint32(td.stringIdx[k.shorty]))
		protoIDs = putU32(protoIDs, uint32(td.typeIdx[k.ret]))
		if r, ok := relListas["p|"+k.params]; ok && k.params != "" {
			protoIDs = putU32(protoIDs, uint32(base[0x1001])+r)
			continue
		}
		protoIDs = putU32(protoIDs, 0)
	}
	fieldIDs := []byte{}
	for _, k := range td.fields {
		// class_idx, type_idx, name_idx: tres ushort y dos bytes de relleno hasta los ocho que
		// mide el item en el fichero (524 + 4*8 = 556, donde d8 pone method_ids). El relleno se
		// escribe cero para que el vecino no se lea dentro de este.
		fieldIDs = putU16(fieldIDs, uint16(td.typeIdx[k.owner]))
		fieldIDs = putU16(fieldIDs, uint16(td.typeIdx[k.typeOf]))
		fieldIDs = putU16(fieldIDs, uint16(td.stringIdx[k.name]))
		fieldIDs = putU16(fieldIDs, 0)
	}
	methodIDs := []byte{}
	for _, k := range td.methods {
		methodIDs = putU16(methodIDs, uint16(td.typeIdx[k.owner]))
		methodIDs = putU16(methodIDs, uint16(td.protoIdx[k.proto]))
		methodIDs = putU16(methodIDs, uint16(td.stringIdx[k.name]))
		methodIDs = putU16(methodIDs, 0)
	}
	classDefs := []byte{}
	for _, cf := range clases {
		classDefs = putU32(classDefs, uint32(td.typeIdx[cf.Name]))
		classDefs = putU32(classDefs, accesoClaseDex(cf.Access))
		if cf.Super == "" {
			classDefs = putU32(classDefs, 0xffffffff)
		} else {
			classDefs = putU32(classDefs, uint32(td.typeIdx[cf.Super]))
		}
		if r, ok := relListas["i|"+cf.Name]; ok {
			classDefs = putU32(classDefs, uint32(base[0x1001])+r)
		} else {
			classDefs = putU32(classDefs, 0)
		}
		if cf.SourceFile == "" {
			classDefs = putU32(classDefs, 0)
		} else {
			classDefs = putU32(classDefs, uint32(td.stringIdx[cf.SourceFile]))
		}
		classDefs = putU32(classDefs, 0) // annotations_off: este fork no emite anotaciones
		classDefs = putU32(classDefs, uint32(base[0x2000])+relDatos[cf.Name])
		// static_values_off: el encoded_array_item con las constantes de los campos estaticos, o
		// cero si la clase no tiene ninguno (medido: R$id, R$layout y R$style lo llevan; las otras
		// tres clases del fixture, no)
		if r, ok := relArrays[cf.Name]; ok {
			classDefs = putU32(classDefs, uint32(base[tipoArrayCodificado])+r)
		} else {
			classDefs = putU32(classDefs, 0)
		}
	}

	areas := []struct {
		kind  dexKind
		datos []byte
	}{
		{0x0001, stringIDs}, {0x0002, typeIDs}, {0x0003, protoIDs}, {0x0004, fieldIDs},
		{0x0005, methodIDs}, {0x0006, classDefs}, {0x2001, codigo}, {0x1001, listas},
		{0x2002, cadenas}, {0x2000, datos}, {tipoArrayCodificado, arrays},
	}

	// 4. map_list: las entradas, en orden ascendente de offset, como las escribió d8
	entradas := [][3]uint32{{uint32(0x0000), 1, 0}}
	tipos := []dexKind{0x0001, 0x0002, 0x0003, 0x0004, 0x0005, 0x0006, 0x2001, 0x1001, 0x2002, 0x2000, tipoArrayCodificado}
	cuantos := map[dexKind]uint32{
		0x0001: uint32(len(td.strings)), 0x0002: uint32(len(td.types)), 0x0003: uint32(len(td.protos)),
		0x0004: uint32(len(td.fields)), 0x0005: uint32(len(td.methods)), 0x0006: uint32(len(clases)),
		0x2001: uint32(len(cuerpos)), 0x1001: uint32(cuantasListas), 0x2002: uint32(len(td.strings)),
		0x2000: uint32(len(clases)), tipoArrayCodificado: uint32(cuantosArrays),
	}
	mapOff := alignUp(off, 4)
	for _, k := range tipos {
		vacia := true
		for _, a := range areas {
			if a.kind == k && len(a.datos) > 0 {
				vacia = false
			}
		}
		// una area vacia no se declara: d8 tampoco escribe en su mapa las secciones que no existen
		if vacia {
			continue
		}
		entradas = append(entradas, [3]uint32{uint32(k), cuantos[k], uint32(base[k])})
	}
	// el propio map_list se declara a si mismo con el kind 0x1000: es la ultima entrada, y el
	// lector la usa para saber donde empieza el mapa
	entradas = append(entradas, [3]uint32{uint32(0x1000), 1, uint32(mapOff)})
	bufMap := putU32(nil, uint32(len(entradas)))
	for _, e := range entradas {
		bufMap = putU16(bufMap, uint16(e[0]))
		bufMap = putU16(bufMap, 0)
		bufMap = putU32(bufMap, e[1])
		bufMap = putU32(bufMap, e[2])
	}

	// 5. cabecera, areas, mapa
	tamano := uint32(mapOff + len(bufMap))
	b := make([]byte, 0, int(tamano))
	b = append(b, []byte("dex\n035\x00")...)
	b = putU32(b, 0)                   // checksum, al final
	b = append(b, make([]byte, 20)...) // signature, al final
	b = putU32(b, tamano)              // file_size
	b = putU32(b, dexHeaderSize)       // header_size
	b = putU32(b, dexEndianTag)        // endian_tag
	b = putU32(b, 0)                   // link_size
	b = putU32(b, 0)                   // link_off
	b = putU32(b, uint32(mapOff))      // map_off
	for _, k := range tipos[:6] {      // las cinco tablas de índices y class_defs: size, off
		b = putU32(b, cuantos[k])
		b = putU32(b, uint32(base[k]))
	}
	// data_size y data_off: el bloque de datos va del primer code_item al final del class_data
	b = putU32(b, uint32(base[tipoArrayCodificado]+len(arrays)-base[0x2001]))
	b = putU32(b, uint32(base[0x2001]))

	for _, a := range areas {
		for uint32(len(b)) < uint32(base[a.kind]) {
			b = append(b, 0)
		}
		b = append(b, a.datos...)
	}
	for uint32(len(b)) < uint32(mapOff) {
		b = append(b, 0)
	}
	b = append(b, bufMap...)

	// 6. firma y checksum: la signature es el sha1 de todo lo que sigue a ella, y el checksum el
	//    adler32 de todo lo que sigue al campo, medido en el fixture y vigilado por Integridad()
	sig := sha1.Sum(b[32:])
	copy(b[12:32], sig[:])
	binary.LittleEndian.PutUint32(b[8:12], adler32.Checksum(b[12:]))
	if len(b) != int(tamano) {
		return nil, fmt.Errorf("the file came out %d bytes and the header says %d", len(b), tamano)
	}
	return b, nil
}

// áreaCadenas: los string_data_item en el orden de la tabla, con la posición de cada uno.
func (td *dexTables) areaCadenas() ([]byte, []uint32) {
	buf := []byte{}
	rels := []uint32{}
	for _, s := range td.strings {
		rels = append(rels, uint32(len(buf)))
		enc := encodeMUTF8(s)
		buf = putUleb(buf, uint32(utf16Len(s)))
		buf = append(buf, enc...)
		buf = append(buf, 0)
	}
	return buf, rels
}

// áreaListas: el área de type_list, una por cada clase con interfaces y por cada proto con
// parámetros. Las listas idénticas se comparten, como hizo d8 (§19 midió el stride de dos bytes
// por type_item); una lista vacía no se escribe, porque su offset es cero.
func (td *dexTables) areaListas(clases []classFull) ([]byte, map[string]uint32, int) {
	buf := []byte{}
	rels := map[string]uint32{}
	cuantas := 0
	pone := func(clave string, tipos []string) {
		if len(tipos) == 0 {
			return
		}
		if _, visto := rels[clave]; visto {
			return
		}
		for uint32(len(buf))%4 != 0 {
			buf = append(buf, 0)
		}
		rels[clave] = uint32(len(buf))
		cuantas++
		buf = putU32(buf, uint32(len(tipos)))
		for _, t := range tipos {
			i, ok := td.typeIdx[t]
			if !ok {
				continue
			}
			buf = putU16(buf, uint16(i))
		}
	}
	for _, cf := range clases {
		pone("i|"+cf.Name, cf.Interfaces)
	}
	for _, k := range td.protos {
		if k.params == "" {
			continue
		}
		_, _, ps := splitDescriptor("(" + k.params + ")V")
		pone("p|"+k.params, ps)
	}
	for uint32(len(buf))%4 != 0 {
		buf = append(buf, 0)
	}
	return buf, rels, cuantas
}

// áreaCodigo: un code_item por método con cuerpo, con su posición dentro del área y el codeItem
// traducido (el recuento de objetos del map_list sale de ahí).
func (td *dexTables) areaCodigo(clases []classFull) ([]byte, map[[2]string]uint32, []codeItem, error) {
	buf := []byte{}
	rels := map[[2]string]uint32{}
	cuerpos := []codeItem{}
	for _, cf := range clases {
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			ci, err := td.codeItemDe(cf, m)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s%s: %w", cf.Name, m.Name, err)
			}
			for uint32(len(buf))%4 != 0 {
				buf = append(buf, 0)
			}
			clave := [2]string{cf.Name, m.Name}
			rels[clave] = uint32(len(buf))
			buf = putU16(buf, uint16(ci.registersSize))
			buf = putU16(buf, uint16(ci.insSize))
			buf = putU16(buf, uint16(ci.outsSize))
			buf = putU16(buf, uint16(ci.triesSize))
			buf = putU32(buf, ci.debugInfoOff)
			buf = putU32(buf, ci.insnsSize)
			for _, w := range ci.insns {
				buf = putU16(buf, w)
			}
			cuerpos = append(cuerpos, ci)
		}
	}
	for uint32(len(buf))%4 != 0 {
		buf = append(buf, 0)
	}
	return buf, rels, cuerpos, nil
}

// áreaClassData: los class_data_item y, en su propio área, los encoded_array_item con las
// constantes de los campos estáticos. Devuelve también las posiciones relativas de ambos, porque
// class_defs tiene que nombrar los dos.
//
// Los ítems van **pegados**, sin relleno entre ellos. Es lo que hace d8 y lo que exige ART, que
// recorre el área de `class_data_item` de forma contigua a partir del offset del `map_list`: midió
// en el teléfono (Android 15, `dexdump`) que un `class_data_item` de 17 bytes seguido de tres ceros
// de relleno desincronizaba ese recorrido y el verificador cortaba el fichero entero con
// `Bad index for class_data_item field_idx: 8 >= 6` — leía los ceros como los cuatro recuentos de la
// clase siguiente. El fichero de `d8` no lleva relleno: sus ítems miden 17, 24, 12, 12, 12 y 10
// bytes y el último termina justo donde empieza el primer `encoded_array_item` (2746). Tampoco
// alinea los arrays: los suyos empiezan en 2746, 2749, 2755 y 2761.
func (td *dexTables) areaClassData(clases []classFull, absCodigo map[[2]string]uint32) ([]byte, map[string]uint32, []byte, map[string]uint32, int) {
	buf := []byte{}
	rels := map[string]uint32{}
	arrays := []byte{}
	relArrays := map[string]uint32{}
	cuantos := 0
	for _, cf := range clases {
		rels[cf.Name] = uint32(len(buf))
		d, v := td.classDataDe(cf, absCodigo)
		buf = append(buf, d...)
		if len(v) > 0 {
			relArrays[cf.Name] = uint32(len(arrays))
			cuantos++
			arrays = append(arrays, v...)
		}
	}
	return buf, rels, arrays, relArrays, cuantos
}

// classDataDe escribe el class_data_item de una clase y, si tiene campos estáticos con constante,
// su encoded_array_item. El orden dentro de cada grupo es el de la tabla de índices, y por eso los
// deltas nunca son negativos. Los valores NO van pegados al encoded_field: medido sobre el fichero
// de d8, el campo de R$id son los dos bytes `01 19` y la constante vive en el array que anuncia
// class_defs.static_values_off (§22).
func (td *dexTables) classDataDe(cf classFull, absCodigo map[[2]string]uint32) ([]byte, []byte) {
	type par struct {
		idx int
		f   classField
	}
	estaticos, instancias := []par{}, []par{}
	for _, f := range cf.Fields {
		idx, ok := td.fieldIdx[fieldKey{owner: cf.Name, typeOf: f.Desc, name: f.Name}]
		if !ok {
			continue
		}
		p := par{idx, f}
		if f.Access&accStatic != 0 {
			estaticos = append(estaticos, p)
		} else {
			instancias = append(instancias, p)
		}
	}
	directos, virtuales := []int{}, []int{}
	accesoDe := map[int]classMethod{}
	for _, m := range cf.Methods {
		idx, ok := td.methodIdx[methodKey{owner: cf.Name, proto: protoKeyOf(m.Desc), desc: m.Desc, name: m.Name}]
		if !ok {
			continue
		}
		accesoDe[idx] = m
		if m.Name == "<init>" || m.Access&accStatic != 0 {
			directos = append(directos, idx)
			continue
		}
		virtuales = append(virtuales, idx)
	}
	ordena := func(v []par) {
		sort.Slice(v, func(i, j int) bool { return v[i].idx < v[j].idx })
	}
	ordena(estaticos)
	ordena(instancias)
	sort.Ints(directos)
	sort.Ints(virtuales)

	out := make([]byte, 0, 48)
	// los cuatro recuentos, en el orden del formato: estaticos, instancia, directos, virtuales
	out = putUleb(out, uint32(len(estaticos)))
	out = putUleb(out, uint32(len(instancias)))
	out = putUleb(out, uint32(len(directos)))
	out = putUleb(out, uint32(len(virtuales)))
	valores := []byte{}
	if len(estaticos) > 0 {
		// el tamano del array, como uleb128 (ver la nota de §22: con u32 los tres items de R no
		// caben en los seis bytes que d8 les dejo)
		valores = putUleb(valores, uint32(len(estaticos)))
	}
	prev := 0
	for _, p := range estaticos {
		out = putUleb(out, uint32(p.idx-prev))
		prev = p.idx
		out = putUleb(out, accesoDex(p.f.Access))
		if len(estaticos) > 0 {
			vf, bytes := td.valorEstatico(p.f)
			if vf == 0 {
				// El array tiene que traer un elemento por campo estatico, o la cuenta del lector no
				// cuadra. Que el elemento sea del TIPO del campo lo exige ART: medido en el aparato
				// (Android 15, `dexdump`, 2026-11-01) con `Lcom/myapp/MainActivity;` y su
				// `static int total = 5` — sin `ConstantValue` en el `.class` porque no es `final` —,
				// anunciarlo con el byte 0x1e cortaba la verificacion entera con
				// `unexpected static field initial value type: 'L' vs 'I'`. `d8` si escribe la
				// constante (`01 04 05`: un valor de un byte con el tipo del entero, y el 5), porque
				// propaga el `<clinit>`; nosotros todavia no, asi que se anuncia un cero del mismo
				// tipo — el `4` de siempre con los cuatro bytes de siempre, que es la cabecera que
				// `d8` usa para las `R$id` — y el `<clinit>` lo sobrescribe al correr, que es lo que
				// habria hecho de todas formas.
				if p.f.Desc == "I" {
					valores = append(valores, byte(valueInt|3<<5), 0, 0, 0, 0)
					continue
				}
				valores = append(valores, valueNull)
				continue
			}
			valores = append(valores, vf)
			valores = append(valores, bytes...)
		}
	}
	prev = 0
	for _, p := range instancias {
		out = putUleb(out, uint32(p.idx-prev))
		prev = p.idx
		out = putUleb(out, accesoDex(p.f.Access))
	}
	pon := func(lista []int) {
		prev := 0
		for _, idx := range lista {
			out = putUleb(out, uint32(idx-prev))
			prev = idx
			m := accesoDe[idx]
			out = putUleb(out, accesoMetodoDex(m))
			out = putUleb(out, absCodigo[[2]string{cf.Name, m.Name}])
		}
	}
	pon(directos)
	pon(virtuales)
	return out, valores
}

// valorEstatico: el encoded_value de una constante de campo, o tipo cero si no la lleva. El byte
// pone el tipo en los bits 0..4 y el tamaño menos uno en los 5..7; después van los bytes del valor,
// poco a poco y truncados. Es el inverso de lo que lee anchoValor.
func (td *dexTables) valorEstatico(f classField) (byte, []byte) {
	switch f.ConstTag {
	case cpInteger:
		n := uint32(uint32(f.ConstInt))
		return byte(valueInt | 3<<5), []byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}
	case cpLong:
		n := uint64(f.ConstLong)
		b := make([]byte, 0, 8)
		for k := 0; k < 8; k++ {
			b = append(b, byte(n>>(8*k)))
		}
		return byte(valueLong | 7<<5), b
	case cpString:
		if _, ok := td.stringIdx[f.ConstStr]; !ok {
			return 0, nil
		}
		return byte(valueString), putUleb([]byte{}, uint32(td.stringIdx[f.ConstStr]))
	case cpFloat, cpDouble:
		// el .class lo trae, pero classField no conserva el bits de un float: sin campo que
		// leer, no se emite, y eso es un "todavia no" dicho por su nombre
		return 0, nil
	}
	return 0, nil
}

// accesoDex pasa los flags del .class a los del dex. Los once primeros valores coinciden en las dos
// tablas (public, private, protected, static, final, synchronized, volatile, transient, native,
// interface, abstract).
func accesoDex(acceso int) uint32 { return uint32(acceso) & 0x0001ffff }

// accesoClaseDex: ACC_SUPER (0x20) es un artefacto del formato de class file y d8 no lo escribe en
// el class_def: medido en las seis clases del fixture, R$id sale 0x11 en vez del 0x31 de javac y la
// anonima 0x0 en vez del 0x20.
func accesoClaseDex(acceso int) uint32 { return accesoDex(acceso) &^ uint32(accSuper) }

// accConstructor es ACC_CONSTRUCTOR del dex, el bit 16. Medido sobre los nueve constructores del
// fixture: el `public <init>` de R sale 0x10001 y el `<init>` sin modificador de la anónima 0x10000.
const accConstructor = 0x10000

func accesoMetodoDex(m classMethod) uint32 {
	a := accesoDex(m.Access)
	if m.Name == "<init>" {
		a |= accConstructor
	}
	return a
}

// claseCabe: lo que este ensamblador todavía no puede escribir, dicho antes de gastar bytes.
func (td *dexTables) claseCabe(cf classFull) error {
	for _, m := range cf.Methods {
		if m.HasCode && len(m.Tries) > 0 {
			return fmt.Errorf("%s%s has a try block: try_item is not measured here", cf.Name, m.Name)
		}
	}
	return nil
}
