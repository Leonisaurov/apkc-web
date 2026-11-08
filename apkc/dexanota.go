package main

// El bloque de anotaciones que `d8` escribe y del que este fork se quedaba fuera. Las reglas de
// codificación están medidas en el parrafo 58 sobre dos sondas propias, leidas en el aparato con
// `/apex/com.android.art/bin/dexdump -a`; la emisión está medida en el parrafo 59 contra el fichero de
// `d8` sobre las seis clases del studio, y es donde salieron los tres defectos del lector de `.class`
// (los dos `u2` de `EnclosingMethod`, el `u1` de `parameters_count` y los flags sin traducir de
// `InnerClass`):
//
//	annotation_item      = ubyte visibilidad | uleb type_idx | uleb numero_de_elementos
//	                       [ uleb name_idx | encoded_value ]...
//	encoded_value        = cabecera uleb, con el tipo en los cinco bits bajos y el codigo de anchura
//	                       en los tres altos (anchura = codigo + 1 bytes). El arreglo (0x1c) lleva uleb
//	                       de conteo y sus elementos detras; el nulo (0x1e) no lleva datos.
//	annotation_set_item  = u32 conteo | u32[] de desplazamientos a cada annotation_item
//	annotations_directory_item = u32 class_annotations_off | u32 campos | u32 metodos |
//	                       u32 campos_estaticos | y despues las listas {u32 indice, u32 desplazamiento}
//
// Que emite `d8`, medido: en la clase `InnerClass` (elementos `accessFlags` y `name`, con `name` nulo
// para una anónima) y `EnclosingMethod` (`value` con un indice de metodo) o `EnclosingClass` (`value`
// con un indice de tipo) segun si la clase esta dentro de un metodo o dentro de otra clase; en los
// metodos, `MethodParameters` (`accessFlags` y `names`, dos arreglos paralelos) cuando el `.class` trae
// el atributo, con los flags de javac copiados tal cual (medidos: 0x1010 y 0x8010).
//
// **No se emite `Signature`**: `d8` copia con ella el atributo `Signature` del `.class`, pero troceada
// en un array de cuerdas a su manera (`"<T::" "Ljava/lang/CharSequence;" ">" "Ljava/lang/Object;"`,
// medido en una sonda generica) y esa regla de troceado no esta medida. Ademas el subconjunto de Java
// que `apkc compile` acepta no tiene parametros de tipo. Queda escrita como diferencia en el parrafo 58.

import (
	"encoding/binary"
	"fmt"
	"sort"
)

const (
	anotacionInnerClass      = "Ldalvik/annotation/InnerClass;"
	anotacionEnclosingMethod = "Ldalvik/annotation/EnclosingMethod;"
	anotacionEnclosingClass  = "Ldalvik/annotation/EnclosingClass;"
	anotacionMemberClasses   = "Ldalvik/annotation/MemberClasses;"
	anotacionMethodParams    = "Ldalvik/annotation/MethodParameters;"
	anotacionSignature       = "Ldalvik/annotation/Signature;"

	// Las visibilidades del formato son 0x00 de construccion, 0x01 de runtime y 0x02 de sistema; las cinco
	// anotaciones de Ldalvik/annotation/* que se emiten salen 0x02 en las dos sondas medidas.
	visibilidadSistema = 0x02

	// Nombres de los elementos, tal como aparecen en el pool de d8.
	elementoAccessFlags = "accessFlags"
	elementoName        = "name"
	elementoNames       = "names"
	elementoValue       = "value"
)

// valor: uno de los encoded_value que este subconjunto necesita. Los indices se resuelven al codificar,
// contra las tablas ya ordenadas; por eso aqui viajan los nombres, no los numeros.
type valorAnotacion struct {
	clase  byte // valueNull, valueInt, valueString, valueType, valueMethod o valueArray
	entero int64
	cadena string // texto de cuerda o descriptor de tipo, segun la clase
	duenio string // las tres piezas de una referencia a metodo
	desc   string
	nombre string
	lista  []valorAnotacion
}

func valorNulo() valorAnotacion           { return valorAnotacion{clase: valueNull} }
func valorEntero(n int64) valorAnotacion  { return valorAnotacion{clase: valueInt, entero: n} }
func valorCadena(s string) valorAnotacion { return valorAnotacion{clase: valueString, cadena: s} }
func valorTipo(d string) valorAnotacion   { return valorAnotacion{clase: valueType, cadena: d} }
func valorArreglo(v ...valorAnotacion) valorAnotacion {
	return valorAnotacion{clase: valueArray, lista: v}
}

// valorMetodo: el valor que lleva EnclosingMethod. d8 escribe un indice de method_ids, medido con
// dexdump -a sobre la sonda: `value=largo` y, en crudo, `1a 0a` (tipo metodo, un byte, indice 10).
func valorMetodo(duenio, desc, nombre string) valorAnotacion {
	return valorAnotacion{clase: valueMethod, duenio: duenio, desc: desc, nombre: nombre}
}

// clave: identificacion del valor sin resolver indices, para deduplicar items.
func (v valorAnotacion) clave() string {
	switch v.clase {
	case valueNull:
		return "null"
	case valueInt:
		return fmt.Sprintf("%d", v.entero)
	case valueString:
		return "cadena:" + v.cadena
	case valueType:
		return "tipo:" + v.cadena
	case valueMethod:
		return "metodo:" + v.duenio + "." + v.nombre + v.desc
	}
	s := "arreglo["
	for i, e := range v.lista {
		if i > 0 {
			s += ","
		}
		s += e.clave()
	}
	return s + "]"
}

// anotacion: un annotation_item. `bytes` se llama con las tablas ordenadas; `clave` deduplica, que d8
// comparte items entre destinos (medido: los dos campos anotados de la sonda apuntan al mismo set).
type anotacion struct {
	tipo      string
	elementos []par
}

type par struct {
	nombre string
	valor  valorAnotacion
}

func (a *anotacion) clave() string {
	s := a.tipo
	for _, p := range a.elementos {
		s += "|" + p.nombre + "=" + p.valor.clave()
	}
	return s
}

func (a *anotacion) bytes(td *dexTables) []byte {
	out := []byte{visibilidadSistema}
	out = putUleb(out, uint32(td.typeIdx[a.tipo]))
	out = putUleb(out, uint32(len(a.elementos)))
	for _, p := range a.elementos {
		out = putUleb(out, uint32(td.stringIdx[p.nombre]))
		out = codificaValor(td, out, p.valor)
	}
	return out
}

// codificaValor: cabecera y datos, con la anchura minima que representa el valor: lo que hace d8
// (medido: 32784 sale con codigo 2 y 4112 con codigo 1).
func codificaValor(td *dexTables, out []byte, v valorAnotacion) []byte {
	switch v.clase {
	case valueNull:
		return append(out, valueNull)
	case valueInt:
		return codificaBytes(out, valueInt, anchuraEntero(v.entero), v.entero)
	case valueString:
		return codificaBytes(out, valueString, anchuraIndice(uint32(td.stringIdx[v.cadena])),
			int64(td.stringIdx[v.cadena]))
	case valueType:
		return codificaBytes(out, valueType, anchuraIndice(uint32(td.typeIdx[v.cadena])),
			int64(td.typeIdx[v.cadena]))
	case valueMethod:
		k := methodKey{owner: v.duenio, proto: protoKeyOf(v.desc), desc: v.desc, name: v.nombre}
		i := int64(td.methodIdx[k])
		return codificaBytes(out, valueMethod, anchuraIndice(uint32(i)), i)
	case valueArray:
		out = append(out, valueArray)
		out = putUleb(out, uint32(len(v.lista)))
		for _, e := range v.lista {
			out = codificaValor(td, out, e)
		}
		return out
	}
	return out
}

// codificaBytes: la cabecera (tipo | codigo de anchura << 5) y `ancho + 1` bytes del numero en
// little-endian.
func codificaBytes(out []byte, tipo byte, codigo uint32, n int64) []byte {
	out = append(out, tipo|byte(codigo<<5))
	for i := uint32(0); i <= codigo; i++ {
		out = append(out, byte(n>>(8*i)))
	}
	return out
}

// anchuraEntero: el codigo de anchura (0 = un byte) del entero mas pequeno que representa al valor con
// su signo.
func anchuraEntero(n int64) uint32 {
	for _, c := range []struct {
		codigo uint32
		limite int64
	}{{0, 0x7f}, {1, 0x7fff}, {2, 0x7fffff}, {3, 0x7fffffff}, {7, 0x7fffffffffffffff}} {
		if n >= -c.limite-1 && n <= c.limite {
			return c.codigo
		}
	}
	return 7
}

// anchuraIndice: lo mismo para un indice de tabla, que nunca es negativo.
func anchuraIndice(n uint32) uint32 {
	for _, c := range []struct {
		codigo uint32
		limite uint32
	}{{0, 0xff}, {1, 0xffff}, {2, 0xffffff}, {3, 0xffffffff}} {
		if n <= c.limite {
			return c.codigo
		}
	}
	return 3
}

// --- las reglas, el internado y las tres areas -------------------------------------------------

// anotacionesDe: las que lleva una clase. Se calcula antes de ordenar las tablas, porque lo que nombran
// hay que internarlo.
type anotacionesDe struct {
	clase   []anotacion
	campos  []anotacionesCampo
	metodos []anotacionesMiembro
}

// anotacionesCampo: las de un campo; el field_id se resuelve al escribir el directorio.
type anotacionesCampo struct {
	owner  string
	desc   string
	nombre string
	items  []anotacion
}

// anotacionesMiembro: las de un metodo. El indice de method_ids se resuelve al escribir las areas, que
// es cuando las tablas ya estan ordenadas.
type anotacionesMiembro struct {
	owner  string
	desc   string
	nombre string
	items  []anotacion
}

// firmaEnTrozos: la troceadura de una firma genérica que hace d8 en el arreglo de cadenas de
// `Signature`. Cada descriptor de clase `L…;` (o `L…<` con argumentos) es un trozo; lo que queda
// entre descriptores (`<T::`, `>`, `()`, `>;`) va en su propio trozo. Medido en el parrafo 101:
// `Ljava/util/Map<Ljava/lang/String;Ljava/lang/Integer;>;` sale
// ["()" "Ljava/util/Map<" "Ljava/lang/String;" "Ljava/lang/Integer;" ">;"].
func firmaEnTrozos(firma string) []string {
	out := []string{}
	inicio := 0
	for i := 0; i < len(firma); {
		if firma[i] != 'L' {
			i++
			continue
		}
		if i > inicio {
			out = append(out, firma[inicio:i])
		}
		j := i
		for j < len(firma) && firma[j] != ';' && firma[j] != '<' {
			j++
		}
		if j < len(firma) {
			j++ // incluye `;` o `<`
		}
		out = append(out, firma[i:j])
		inicio = j
		i = j
	}
	if inicio < len(firma) {
		out = append(out, firma[inicio:])
	}
	return out
}

func anotacionDeFirma(firma string) anotacion {
	partes := firmaEnTrozos(firma)
	valores := make([]valorAnotacion, 0, len(partes))
	for _, p := range partes {
		valores = append(valores, valorCadena(p))
	}
	return anotacion{tipo: anotacionSignature, elementos: []par{
		{elementoValue, valorArreglo(valores...)},
	}}
}

func reglasDe(cf classFull) anotacionesDe {
	sal := anotacionesDe{}
	if cf.Signature != "" {
		sal.clase = append(sal.clase, anotacionDeFirma(cf.Signature))
	}
	for _, f := range cf.Fields {
		if f.Signature != "" {
			sal.campos = append(sal.campos, anotacionesCampo{
				owner: cf.Name, desc: f.Desc, nombre: f.Name,
				items: []anotacion{anotacionDeFirma(f.Signature)},
			})
		}
	}
	var propia *classInner
	miembros := []classInner{}
	for i := range cf.Inners {
		in := cf.Inners[i]
		if in.inner == cf.Name {
			propia = &cf.Inners[i]
		}
		// solo las que tienen nombre y dueho declarado son miembro: la anonima del studio sale
		// outer="" name="" y d8 no la apunta desde MemberClasses
		if in.outer == cf.Name && in.name != "" {
			miembros = append(miembros, in)
		}
	}
	// Medido en la anonima: el set es {EnclosingMethod, InnerClass}, o sea primero donde esta la clase
	// y despues como se llama.
	if cf.EnclosingMethod && cf.Enclosing.nombre != "" {
		sal.clase = append(sal.clase, anotacion{tipo: anotacionEnclosingMethod, elementos: []par{
			{elementoValue, valorMetodo(cf.Enclosing.clase, cf.Enclosing.desc, cf.Enclosing.nombre)},
		}})
	} else if propia != nil && propia.outer != "" && propia.name != "" {
		// @EnclosingClass la pide una clase miembro CON nombre (R$id, R$layout, R$style), no la
		// anonima: asi esta en el fichero de d8 del fixture, medido en el parrafo 59
		sal.clase = append(sal.clase, anotacion{tipo: anotacionEnclosingClass, elementos: []par{
			{elementoValue, valorTipo(propia.outer)},
		}})
	}
	if propia != nil {
		nom := valorNulo()
		if propia.name != "" {
			nom = valorCadena(propia.name)
		}
		// accessFlags va **tal cual lo dejo javac en InnerClasses** (medido: 0x19 para R$id, 0x0 para
		// la anonima), no convertido a los flags del dex, que colisionan entre public y static
		sal.clase = append(sal.clase, anotacion{tipo: anotacionInnerClass, elementos: []par{
			{elementoAccessFlags, valorEntero(int64(propia.access))},
			{elementoName, nom},
		}})
	}
	if len(miembros) > 0 {
		valores := make([]valorAnotacion, 0, len(miembros))
		for _, in := range miembros {
			valores = append(valores, valorTipo(in.inner))
		}
		// el orden del arreglo es el orden de las filas del InnerClasses del .class (medido: style,
		// layout, id en R, que es como javac las escribe y como d8 las copia)
		sal.clase = append(sal.clase, anotacion{tipo: anotacionMemberClasses, elementos: []par{
			{elementoValue, valorArreglo(valores...)},
		}})
	}
	for _, m := range cf.Methods {
		items := []anotacion{}
		if m.Parameters && len(m.Parametros) > 0 {
			flags := make([]valorAnotacion, 0, len(m.Parametros))
			nombres := make([]valorAnotacion, 0, len(m.Parametros))
			for _, p := range m.Parametros {
				flags = append(flags, valorEntero(int64(p.access)))
				if p.nombre == "" {
					nombres = append(nombres, valorNulo())
					continue
				}
				nombres = append(nombres, valorCadena(p.nombre))
			}
			items = append(items, anotacion{tipo: anotacionMethodParams, elementos: []par{
				{elementoAccessFlags, valorArreglo(flags...)},
				{elementoNames, valorArreglo(nombres...)},
			}})
		}
		if m.Signature != "" {
			items = append(items, anotacionDeFirma(m.Signature))
		}
		if len(items) > 0 {
			sal.metodos = append(sal.metodos, anotacionesMiembro{owner: cf.Name, desc: m.Desc, nombre: m.Name, items: items})
		}
	}
	return sal
}

// reglasDeTodas: por clase anotada, omitiendo las que no llevan nada (su annotations_off sale cero,
// como en los ficheros de d8).
func reglasDeTodas(clases []classFull) map[string]anotacionesDe {
	sal := map[string]anotacionesDe{}
	for _, cf := range clases {
		if a := reglasDe(cf); len(a.clase) > 0 || len(a.campos) > 0 || len(a.metodos) > 0 {
			sal[cf.Name] = a
		}
	}
	return sal
}

// internAnotaciones: anade a las tablas lo que las anotaciones nombran. Va entre collectClasses y
// orderTables, que es cuando todavia se pueden anadir entradas.
func (t *dexTables) internAnotaciones(de map[string]anotacionesDe) {
	for _, nombre := range []string{elementoAccessFlags, elementoName, elementoNames, elementoValue} {
		t.internString(nombre)
	}
	for _, a := range de {
		listas := [][]anotacion{a.clase}
		for _, f := range a.campos {
			listas = append(listas, f.items)
		}
		for _, m := range a.metodos {
			listas = append(listas, m.items)
		}
		for _, l := range listas {
			for _, it := range l {
				t.internType(it.tipo)
				for _, p := range it.elementos {
					t.internString(p.nombre)
					internaValor(t, p.valor)
				}
			}
		}
	}
}

func internaValor(t *dexTables, v valorAnotacion) {
	switch v.clase {
	case valueString:
		if v.cadena != "" {
			t.internString(v.cadena)
		}
	case valueType:
		t.internType(v.cadena)
	case valueMethod:
		t.internType(v.duenio)
		t.internMethod(v.duenio, v.desc, v.nombre)
	}
	for _, e := range v.lista {
		internaValor(t, e)
	}
}

// parcheAnot: un u32 que se escribe en cuanto se saben las bases: `area + en` recibe `destino + rel`.
type parcheAnot struct {
	area    dexKind
	en      int
	destino dexKind
	rel     uint32
}

// planAnotaciones: las tres areas, los parches y el offset relativo del directorio de cada clase.
type planAnotaciones struct {
	items    []byte
	sets     []byte
	dirs     []byte
	parches  []parcheAnot
	offClase map[string]uint32
	nItems   int
	nSets    int
	nDirs    int
}

func (p *planAnotaciones) vacia() bool { return p == nil || p.nDirs == 0 }

// armaAnotaciones: el plan de las areas. Los items identicos se comparten, como hace d8.
func (td *dexTables) armaAnotaciones(clases []classFull, de map[string]anotacionesDe) *planAnotaciones {
	p := &planAnotaciones{offClase: map[string]uint32{}}
	relItems := map[string]uint32{}
	claveItems := func(l []anotacion) []uint32 {
		// el annotation_set_item va ordenado por type_idx (lo exige el verificador de ART: sin
		// esto, una clase con Signature y EnclosingClass sale «Out-of-order entry types»)
		ordenadas := append([]anotacion(nil), l...)
		sort.SliceStable(ordenadas, func(i, j int) bool {
			return td.typeIdx[ordenadas[i].tipo] < td.typeIdx[ordenadas[j].tipo]
		})
		sal := make([]uint32, 0, len(ordenadas))
		for i := range ordenadas {
			k := ordenadas[i].clave()
			if r, ok := relItems[k]; ok {
				sal = append(sal, r)
				continue
			}
			rel := uint32(len(p.items))
			p.items = append(p.items, ordenadas[i].bytes(td)...)
			relItems[k] = rel
			p.nItems++
			sal = append(sal, rel)
		}
		return sal
	}
	setDe := func(rels []uint32) uint32 {
		rel := uint32(len(p.sets))
		p.sets = putU32(p.sets, uint32(len(rels)))
		for _, r := range rels {
			p.parches = append(p.parches, parcheAnot{area: 0x1003, en: len(p.sets), destino: 0x2004, rel: r})
			p.sets = putU32(p.sets, 0)
		}
		p.nSets++
		return rel
	}
	for _, cf := range clases {
		a, hay := de[cf.Name]
		if !hay {
			continue
		}
		dir := uint32(len(p.dirs))
		p.nDirs++
		relClase := uint32(0)
		hayClase := len(a.clase) > 0
		if hayClase {
			relClase = setDe(claveItems(a.clase))
		}
		p.dirs = putU32(p.dirs, 0) // class_annotations_off
		if hayClase {
			p.parches = append(p.parches, parcheAnot{area: 0x2006, en: len(p.dirs) - 4, destino: 0x1003, rel: relClase})
		}
		p.dirs = putU32(p.dirs, uint32(len(a.campos)))  // fields_size
		p.dirs = putU32(p.dirs, uint32(len(a.metodos))) // annotated_methods_size
		p.dirs = putU32(p.dirs, 0)                      // annotated_static_fields_size
		for _, f := range a.campos {
			rel := setDe(claveItems(f.items))
			k := fieldKey{owner: f.owner, typeOf: f.desc, name: f.nombre}
			p.dirs = putU32(p.dirs, uint32(td.fieldIdx[k]))
			p.dirs = putU32(p.dirs, 0)
			p.parches = append(p.parches, parcheAnot{area: 0x2006, en: len(p.dirs) - 4, destino: 0x1003, rel: rel})
		}
		for _, m := range a.metodos {
			rel := setDe(claveItems(m.items))
			k := methodKey{owner: m.owner, proto: protoKeyOf(m.desc), desc: m.desc, name: m.nombre}
			p.dirs = putU32(p.dirs, uint32(td.methodIdx[k]))
			p.dirs = putU32(p.dirs, 0)
			p.parches = append(p.parches, parcheAnot{area: 0x2006, en: len(p.dirs) - 4, destino: 0x1003, rel: rel})
		}
		p.offClase[cf.Name] = dir
	}
	return p
}

// aplica: escribe los u32 absolutos que faltaban. Se llama con todas las bases asignadas.
func (p *planAnotaciones) aplica(base map[dexKind]int) {
	for _, q := range p.parches {
		var area []byte
		switch q.area {
		case 0x1003:
			area = p.sets
		case 0x2006:
			area = p.dirs
		default:
			area = p.items
		}
		binary.LittleEndian.PutUint32(area[q.en:], uint32(base[q.destino])+q.rel)
	}
}
