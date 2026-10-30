package main

// El escritor de AXML, la pieza 3-D. Escribe los binarios de AndroidManifest.xml y de los layouts
// con la forma medida sobre los ficheros que deja el aapt2 de este telefono (parrafos 24, 26 y 30 de
// docs/evidencia.md) y se compara byte a byte contra ellos en axmlwriter_test.go.
//
// Lo copiado de los bytes, no del manual:
//   - el orden del pool: primero los NOMBRES DE ATRIBUTO QUE TIENEN id DE RECURSO, ordenados por ese
//     id (que es el mismo orden del XML_RESOURCE_MAP), y despues el resto de las cadenas ordenadas
//     por valor ASCII. Medido en los dos ficheros del fixture: 6 + 8 en el layout, 12 + 19 en el
//     manifiesto.
//   - los atributos de un elemento salen ordenados: los de namespace por su id y despues los sin
//     namespace por su nombre. En el fuente el TextView declara style primero y en el binario sale
//     ultimo, o sea que aapt2 reordena; no es el orden de entrada.
//   - el campo ns de un ATRIBUTO y de un ELEMENTO es el indice del URI, no el del prefijo (medido:
//     vale 4 en el layout de dos namespaces, donde 3 es "android" y 4 es el URI), mientras que los
//     trozos de namespace llevan primero el prefijo y despues el URI.
//   - idIndex, classIndex y styleIndex son indices 1-based dentro de la lista de atributos del
//     propio elemento, y 0 cuando no hay. Medido: el Button con android:id de primero da idIndex 1,
//     y el TextView con style de ultimo da styleIndex 4.
//   - el valor crudo (rawValue) solo se escribe cuando el valor tipado es una cadena (tipo 0x03), y
//     entonces apunta a la MISMA cadena que los datos; en los tipos numericos, de referencia y de
//     enum o flags vale -1. Medido en los dos ficheros.
//   - un END_TAG repite la linea de su START_TAG y mide 24; un START_TAG mide 16 + 20 + 20*attrs,
//     con un attrExt de 20 y cada atributo de 20.
//   - aapt2 TIRA los atributos de los namespaces que no conoce: probado con tools:ignore, y ni el
//     atributo ni su declaracion llegan al binario (parrafo 30). Aqui el que llama decide que se
//     tira; este escritor rechaza lo que no sabe escribir.

import (
	"fmt"
	"sort"
)

// tipos de Res_value medidos en los dos ficheros del fixture
const (
	tipoReferencia byte = 0x01
	tipoCadena     byte = 0x03
	tipoEnteroDec  byte = 0x10
	tipoEnteroHex  byte = 0x11
	tipoBooleano   byte = 0x12
)

// atributoEscribir es un atributo de un elemento. ns es el PREFIJO declarado ("" = sin namespace),
// id es el id de recurso del atributo — 0 para los que no lo tienen, que no entran al mapa — y texto
// es el valor como cadena, que es lo que se escribe como crudo y como datos cuando tipo es 0x03.
type atributoEscribir struct {
	ns     string
	nombre string
	id     uint32
	tipo   byte
	texto  string
	datos  uint32
}

// elementoEscribir es un nodo de inicio con sus atributos y sus hijos. linea es la linea del fuente,
// la que escriben tanto el START_TAG como su END_TAG.
type elementoEscribir struct {
	ns     string
	nombre string
	linea  int
	attrs  []atributoEscribir
	hijos  []*elementoEscribir
}

// declaracionNS es un xmlns:prefijo="uri" con la linea donde se declaro, que es la linea de su
// START_NAMESPACE y de su END_NAMESPACE.
type declaracionNS struct {
	prefijo string
	uri     string
	linea   int
}

// documentoAXML es el fichero binario completo. utf8 elige el modo del pool: medido, el layout sale
// en UTF-8 y el AndroidManifest.xml en UTF-16, porque los escribe herramientas distintas.
type documentoAXML struct {
	ns   []declaracionNS
	raiz *elementoEscribir
	utf8 bool
}

// sinNamespace: el -1 que llevan los campos de nombre sin namespace, medido en los dos ficheros.
const sinNamespace = uint32(0xffffffff)

// escritoAXML resuelve los indices ya montado el pool. Lleva el mapa de cadena a posicion y el de
// prefijo a URI, porque en el binario el campo ns de un nodo o un atributo es la posicion del URI.
type escritoAXML struct {
	posicion map[string]int
	uri      map[string]string
}

func (w escritoAXML) idx(s string) uint32 { return uint32(w.posicion[s]) }

func (w escritoAXML) nsDe(prefijo string) uint32 {
	if prefijo == "" {
		return sinNamespace
	}
	return w.idx(w.uri[prefijo])
}

// escribeAXML arma el fichero entero. Rechaza lo que no esta medido: un atributo de namespace sin id
// resuelto, un atributo sin namespace con id, un prefijo usado sin declaracion y un nombre de
// atributo que aparece con dos ids distintos.
func escribeAXML(doc documentoAXML) ([]byte, error) {
	if doc.raiz == nil {
		return nil, fmt.Errorf("el documento no tiene raiz")
	}
	if doc.raiz.nombre == "" {
		return nil, fmt.Errorf("un elemento sin nombre")
	}
	uris := map[string]string{}
	for _, d := range doc.ns {
		if d.prefijo == "" || d.uri == "" {
			return nil, fmt.Errorf("declaracion de namespace con prefijo o uri vacios")
		}
		uris[d.prefijo] = d.uri
	}

	conId := map[string]uint32{} // nombre de atributo que va al mapa
	demas := map[string]bool{}   // el resto de las cadenas
	meter := func(s string) {
		if s != "" {
			demas[s] = true
		}
	}
	var recorrer func(*elementoEscribir) error
	recorrer = func(e *elementoEscribir) error {
		if e.nombre == "" {
			return fmt.Errorf("un elemento hijo sin nombre")
		}
		meter(e.nombre)
		if e.ns != "" {
			uri, ok := uris[e.ns]
			if !ok {
				return fmt.Errorf("el elemento %q usa el prefijo %q sin declararlo", e.nombre, e.ns)
			}
			meter(e.ns)
			meter(uri)
		}
		for i := range e.attrs {
			a := &e.attrs[i]
			if a.nombre == "" {
				return fmt.Errorf("el elemento %q tiene un atributo sin nombre", e.nombre)
			}
			if a.id != 0 && a.ns == "" {
				return fmt.Errorf("el atributo %q de %q lleva id %#08x sin namespace: lo unico medido es un id en un atributo declarado con prefijo", a.nombre, e.nombre, a.id)
			}
			if a.id == 0 && a.ns != "" {
				return fmt.Errorf("el atributo %s:%q de %q no tiene id resuelto: la unica forma medida es la de un atributo de namespace con su id", a.ns, a.nombre, e.nombre)
			}
			if a.ns != "" {
				uri, ok := uris[a.ns]
				if !ok {
					return fmt.Errorf("el atributo %s:%q usa un prefijo no declarado", a.ns, a.nombre)
				}
				meter(a.ns)
				meter(uri)
			}
			if previo, ok := conId[a.nombre]; ok && a.id != 0 && previo != a.id {
				return fmt.Errorf("el nombre de atributo %q lleva dos ids: %#08x y %#08x", a.nombre, previo, a.id)
			}
			if a.id != 0 {
				conId[a.nombre] = a.id
			} else {
				meter(a.nombre)
			}
			if a.tipo == tipoCadena {
				meter(a.texto)
			}
		}
		for _, h := range e.hijos {
			if err := recorrer(h); err != nil {
				return err
			}
		}
		return nil
	}
	if err := recorrer(doc.raiz); err != nil {
		return nil, err
	}

	// grupo 1: los nombres con id, en el orden del mapa (por id). grupo 2: todo lo demas, ordenado
	// por valor ASCII, con las mayusculas antes que las minusculas como sale en los dos ficheros.
	nombres := make([]string, 0, len(conId))
	for n := range conId {
		nombres = append(nombres, n)
	}
	sort.Slice(nombres, func(i, j int) bool { return conId[nombres[i]] < conId[nombres[j]] })
	resto := make([]string, 0, len(demas))
	for s := range demas {
		resto = append(resto, s)
	}
	sort.Strings(resto)
	cadenas := append(append([]string{}, nombres...), resto...)
	posicion := map[string]int{}
	for i, s := range cadenas {
		if _, ok := posicion[s]; !ok {
			posicion[s] = i
		}
	}
	w := escritoAXML{posicion: posicion, uri: uris}

	pool := escribePool(cadenas, doc.utf8)

	mapa := trozoCabecera(trozoMapaRec, 8, 8+4*len(nombres))
	for _, n := range nombres {
		mapa = putU32(mapa, conId[n])
	}

	nodos := []byte{}
	for _, d := range doc.ns {
		nodos = append(nodos, trozoNS(trozoInicioNS, w.idx(d.prefijo), w.idx(d.uri), d.linea)...)
	}
	var escribir func(*elementoEscribir)
	escribir = func(e *elementoEscribir) {
		attrs := ordenarAtributos(e.attrs)
		id, clase, estilo := indicesEspeciales(attrs)
		nodos = append(nodos, inicioTag(w, e, attrs, id, clase, estilo)...)
		for _, h := range e.hijos {
			escribir(h)
		}
		nodos = append(nodos, finTag(w.nsDe(e.ns), w.idx(e.nombre), e.linea)...)
	}
	escribir(doc.raiz)
	// los namespaces se cierran en el orden inverso al de declaracion; medido con uno solo, y es lo
	// que hace anidarse bien con los trozos de nodo que van en medio
	for i := len(doc.ns) - 1; i >= 0; i-- {
		d := doc.ns[i]
		nodos = append(nodos, trozoNS(trozoFinNS, w.idx(d.prefijo), w.idx(d.uri), d.linea)...)
	}

	out := trozoCabecera(trozoXML, 8, 8+len(pool)+len(mapa)+len(nodos))
	out = append(out, pool...)
	out = append(out, mapa...)
	return append(out, nodos...), nil
}

// ordenarAtributos reproduce el orden de salida de aapt2: los atributos con namespace por su id de
// recurso y despues los sin namespace por su nombre. Estable, porque dos atributos de un mismo
// elemento no comparten nombre.
func ordenarAtributos(attrs []atributoEscribir) []atributoEscribir {
	out := append([]atributoEscribir{}, attrs...)
	clave := func(a atributoEscribir) string {
		if a.ns == "" {
			return "\x01" + a.nombre
		}
		return fmt.Sprintf("\x00%08x\x00%s", a.id, a.nombre)
	}
	sort.SliceStable(out, func(i, j int) bool { return clave(out[i]) < clave(out[j]) })
	return out
}

// indicesEspeciales: idIndex, classIndex y styleIndex, 1-based dentro de la lista ya ordenada, y 0
// cuando no hay. Medidos android:id (de primero en el Button: 1) y style sin namespace (de ultimo en
// el TextView: 4); para class se aplica la misma regla, dicha como analogia y no como medicion.
func indicesEspeciales(attrs []atributoEscribir) (id, clase, estilo uint16) {
	for i, a := range attrs {
		pos := uint16(i + 1)
		switch {
		case a.ns == "android" && a.nombre == "id":
			id = pos
		case a.ns == "" && a.nombre == "class":
			clase = pos
		case a.ns == "" && a.nombre == "style":
			estilo = pos
		}
	}
	return id, clase, estilo
}

// trozoNS: START_NAMESPACE o END_NAMESPACE, 24 bytes con el prefijo y el URI. Miden lo mismo y se
// distinguen solo por el tipo.
func trozoNS(tipo uint16, prefijo, uri uint32, linea int) []byte {
	b := trozoCabecera(tipo, 16, 24)
	b = putU32(b, uint32(linea))
	b = putU32(b, sinNamespace) // comentario: -1 en todo lo medido
	b = putU32(b, prefijo)
	return putU32(b, uri)
}

// finTag: un END_TAG mide 24 y repite la linea de su START_TAG.
func finTag(ns, nombre uint32, linea int) []byte {
	b := trozoCabecera(trozoFinEtik, 16, 24)
	b = putU32(b, uint32(linea))
	b = putU32(b, sinNamespace)
	b = putU32(b, ns)
	return putU32(b, nombre)
}

// inicioTag: cabecera de 16, attrExt de 20 y un atributo de 20 por cada uno.
func inicioTag(w escritoAXML, e *elementoEscribir, attrs []atributoEscribir, id, clase, estilo uint16) []byte {
	b := trozoCabecera(trozoInicioEtiqueta, 16, 16+20+20*len(attrs))
	b = putU32(b, uint32(e.linea))
	b = putU32(b, sinNamespace) // comentario
	b = putU32(b, w.nsDe(e.ns))
	b = putU32(b, w.idx(e.nombre))
	b = putU16(b, 20) // attrStart: lo que mide el attrExt
	b = putU16(b, 20) // attrSize: lo que mide un atributo
	b = putU16(b, uint16(len(attrs)))
	b = putU16(b, id)
	b = putU16(b, clase)
	b = putU16(b, estilo)
	for _, a := range attrs {
		b = putU32(b, w.nsDe(a.ns))
		b = putU32(b, w.idx(a.nombre))
		datos := a.datos
		if a.tipo == tipoCadena {
			// crudo y tipado apuntan a la MISMA cadena del pool, medido en los dos ficheros
			datos = w.idx(a.texto)
			b = putU32(b, datos)
		} else {
			b = putU32(b, sinNamespace)
		}
		b = apilaValor(b, valorEscribir{tipo: a.tipo, datos: datos})
	}
	return b
}
