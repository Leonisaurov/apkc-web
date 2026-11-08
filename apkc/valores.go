package main

// Leer los res/values/*.xml en texto y producir la tabla de recursos, que es la primera mitad del
// paso 3-G. Las formas de los valores NO estan supuestas: `string` sale tipo 0x03 con el texto en el
// pool global, `color` 0x1d con el ARGB, `bool` 0x12 con -1 o 0, `item type="id"` 0x12 a cero y con
// banderas 0x0004, y el estilo como valor complejo con sus registros ordenados por id de atributo —
// todo visto en las variantes del parrafo 29 y en los ficheros fijados del parrafo 24.
//
// El orden de emision es el medido: tipos alfabeticos con id = posicion mas uno, entradas de cada
// tipo ordenadas por nombre, claves en el orden de emision y pool global ASCII. Por eso el id de un
// recurso es paquete<<24 | tipo<<16 | indice, sin need de una segunda pasada: en un values.xml no hay
// referencias al propio paquete (las referencias `@+id/…` viven en los layouts, y esas si necesitan
// la segunda pasada de layouts.go).

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// fuenteXML es un fichero del proyecto en texto: su ruta dentro del apk y su contenido.
type fuenteXML struct {
	nombre    string
	contenido []byte
	// config es el calificador de una segunda configuracion, hoy solo el idioma de `values-es`
	// (parrafo 94). Vacío es la configuracion base.
	config string
}

// recursoEntabla es una entrada de la tabla antes de ordenar: su tipo, su clave y el valor ya resuelto
// (o los registros, si es compleja).
type recursoEntabla struct {
	tipo     string
	config   string
	clave    string
	valor    valorEscribir
	texto    string // el texto del valor, que es lo que va al pool global cuando el valor es cadena
	compleja bool
	padre    uint32
	items    []registroEscribir
}

// tablaDeRecursos ordena como ordena aapt2 y arma el modelo: tipos alfabeticos con id = posicion mas
// uno, entradas de cada tipo por nombre, claves en el orden de emision y pool global ASCII con las
// rutas de los layouts y los textos de las cadenas. Las banderas salen de la regla medida en el
// parrafo 29: 0x0004 en el tipo id, 0x0001 en un valor complejo, 0 en el resto.
func tablaDeRecursos(recursos []recursoEntabla, layouts, ficheros []fuenteXML, paquete string) (*tablaEscribir, error) {
	// aapt2 descarta una clave que solo existe en una configuracion secundaria: avisa
	// «removing resource … without required default value» y no entra ni en la tabla ni en el pool
	// (medido en el parrafo 94). La base define el conjunto de claves de cada tipo.
	base := map[string]bool{}
	for _, r := range recursos {
		if r.config == "" {
			base[r.tipo+"\x00"+r.clave] = true
		}
	}
	filtrados := recursos[:0]
	for _, r := range recursos {
		if r.config != "" && !base[r.tipo+"\x00"+r.clave] {
			continue
		}
		filtrados = append(filtrados, r)
	}
	recursos = filtrados

	nombresTipos := map[string]bool{}
	for _, r := range recursos {
		nombresTipos[r.tipo] = true
	}
	for _, f := range layouts {
		nombresTipos[tipoDeRecurso(f.nombre)] = true
	}
	for _, f := range ficheros {
		nombresTipos[tipoDeRecurso(f.nombre)] = true
	}
	tipos := []string{}
	for n := range nombresTipos {
		tipos = append(tipos, n)
	}
	sort.Strings(tipos)

	// El pool global de aapt2 va por configuraciones: primero la base, ordenada por texto, y después
	// cada idioma, también ordenado por texto. Medido en el parrafo 94: `Icono`, `Uno` (base) y
	// `Icono ES` (es); con `Dos`/`Uno` la base sale `Dos`, `Uno`.
	global := []string{}
	for _, f := range layouts {
		global = append(global, f.nombre)
	}
	for _, f := range ficheros {
		global = append(global, f.nombre)
	}
	for _, r := range recursos {
		if r.config == "" && r.valor.tipo == tipoCadena && r.texto != "" {
			global = append(global, r.texto)
		}
		if r.compleja {
			// los `string-array` y los `plurals` llevan sus textos en los registros, no en r.texto
			for _, item := range r.items {
				if item.valor.tipo == tipoCadena && item.valor.texto != "" {
					global = append(global, item.valor.texto)
				}
			}
		}
	}
	sort.Strings(global)
	configsGlobal := map[string][]string{}
	for _, r := range recursos {
		if r.config != "" && r.valor.tipo == tipoCadena && r.texto != "" {
			configsGlobal[r.config] = append(configsGlobal[r.config], r.texto)
		}
	}
	nombresConfig := make([]string, 0, len(configsGlobal))
	for cfg := range configsGlobal {
		nombresConfig = append(nombresConfig, cfg)
	}
	sort.Strings(nombresConfig)
	for _, cfg := range nombresConfig {
		textos := configsGlobal[cfg]
		sort.Strings(textos)
		global = append(global, textos...)
	}
	global = sinRepetidos(global)
	indiceGlobal := map[string]int{}
	for i, s := range global {
		indiceGlobal[s] = i
	}

	claves := []string{}
	indiceClave := map[string]int{}
	escritos := []tipoEscribir{}
	for i, nombreTipo := range tipos {
		entradas := []recursoEntabla{}
		for _, r := range recursos {
			if r.tipo == nombreTipo {
				entradas = append(entradas, r)
			}
		}
		for _, f := range layouts {
			if tipoDeRecurso(f.nombre) != nombreTipo {
				continue
			}
			entradas = append(entradas, recursoEntabla{
				tipo:   nombreTipo,
				config: f.config,
				clave:  claveDeLayout(f.nombre),
				valor:  valorEscribir{tipo: tipoCadena},
				texto:  f.nombre,
			})
		}
		sort.SliceStable(entradas, func(a, b int) bool { return entradas[a].clave < entradas[b].clave })
		// las claves del tipo son la union de las configuraciones; cada bloque lleva una entrada por
		// clave, con `falta` cuando esa configuracion no la declara (medido en `values-es`, parrafo 94)
		conjunto := map[string]bool{}
		for _, r := range entradas {
			conjunto[r.clave] = true
		}
		clavesTipo := make([]string, 0, len(conjunto))
		for k := range conjunto {
			clavesTipo = append(clavesTipo, k)
		}
		sort.Strings(clavesTipo)
		for _, k := range clavesTipo {
			if _, ok := indiceClave[k]; !ok {
				indiceClave[k] = len(claves)
				claves = append(claves, k)
			}
		}
		porConfigYClave := map[string]recursoEntabla{}
		configs := []string{""}
		vistas := map[string]bool{"": true}
		for _, r := range entradas {
			porConfigYClave[r.config+"\x00"+r.clave] = r
			if !vistas[r.config] {
				vistas[r.config] = true
				configs = append(configs, r.config)
			}
		}
		sort.Strings(configs[1:])
		bloques := []bloqueEscribir{}
		for _, cfg := range configs {
			bloque := bloqueEscribir{config: cfg, entradas: make([]entradaEscribir, 0, len(clavesTipo))}
			for _, clave := range clavesTipo {
				r, ok := porConfigYClave[cfg+"\x00"+clave]
				if !ok {
					bloque.entradas = append(bloque.entradas, entradaEscribir{clave: indiceClave[clave], falta: true})
					continue
				}
				registros := append([]registroEscribir(nil), r.items...)
				for i := range registros {
					if registros[i].valor.tipo == tipoCadena {
						indice, ok := indiceGlobal[registros[i].valor.texto]
						if !ok {
							return nil, fmt.Errorf("la cadena de un registro de %s/%s no esta en el pool global", r.tipo, r.clave)
						}
						registros[i].valor.datos = uint32(indice)
					}
				}
				e := entradaEscribir{clave: indiceClave[clave], compleja: r.compleja, padre: r.padre, registros: registros}
				switch {
				case r.compleja:
					e.flags = 0x0001
				case nombreTipo == "id":
					e.flags = 0x0004
					e.valor = r.valor
				case r.valor.tipo == tipoCadena:
					indice, ok := indiceGlobal[r.texto]
					if !ok {
						return nil, fmt.Errorf("la cadena de %s/%s no esta en el pool global", r.tipo, r.clave)
					}
					e.flags = 0
					e.valor = valorEscribir{tipo: tipoCadena, datos: uint32(indice)}
				default:
					e.valor = r.valor
				}
				bloque.entradas = append(bloque.entradas, e)
			}
			if len(bloque.entradas) > 0 {
				bloques = append(bloques, bloque)
			}
		}
		if len(bloques) == 0 {
			continue
		}
		// la bandera del spec por clave dice en qué configuraciones aparece: aapt2 pone 0x0004
		// (locale) en `app_name` porque está en values y en values-es, y 0 en `otro` (parrafo 94)
		specFlags := make([]uint32, len(clavesTipo))
		for j, clave := range clavesTipo {
			for _, cfg := range configs[1:] {
				if _, ok := porConfigYClave[cfg+"\x00"+clave]; ok {
					specFlags[j] |= mascaraConfig(cfg)
				}
			}
		}
		escritos = append(escritos, tipoEscribir{
			id:        byte(i + 1),
			specFlags: specFlags,
			bloques:   bloques,
		})
	}
	return &tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: paquete,
		global:        global,
		globalUTF8:    true, // medido: el pool global del arsc de aapt2 sale UTF-8
		nombresTipo:   tipos,
		nombresUTF8:   false, // y el de nombres de tipo, UTF-16
		claves:        claves,
		clavesUTF8:    true,
		tipos:         escritos,
	}, nil
}

// mascaraConfig: las banderas del spec_map que aapt2 pone para cada configuración medida. `es` es
// locale (0x0004), `land` orientación (0x0080) y `hdpi` densidad (0x0100) más la marca 0x0400 que
// aapt2 añade al tipo drawable con densidad (medido en el parrafo 103: 0x0500).
func mascaraConfig(cfg string) uint32 {
	switch cfg {
	case "land":
		return 0x0080
	case "hdpi":
		return 0x0500
	default:
		return 0x0004
	}
}

// sinRepetidos conserva el orden, que ya viene sorted.
func sinRepetidos(s []string) []string {
	out := s[:0]
	visto := map[string]bool{}
	for _, x := range s {
		if visto[x] {
			continue
		}
		visto[x] = true
		out = append(out, x)
	}
	return out
}

// claveDeLayout saca el nombre del recurso de la ruta: res/layout/main.xml -> main.
func claveDeLayout(ruta string) string {
	base := ruta
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	return base
}

// resourcesDeValores traduce un res/values/*.xml en entradas listas para la tabla. `m` es la tabla del
// framework, que hace falta para los nombres de atributo de los items de un estilo.
func recursosDeValores(f fuenteXML, m *marcoRecursos) ([]recursoEntabla, error) {
	out := []recursoEntabla{}
	d := xml.NewDecoder(strings.NewReader(string(f.contenido)))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		raiz, esInicio := tok.(xml.StartElement)
		if !esInicio || raiz.Name.Local != "resources" {
			continue
		}
		for {
			tok, err := d.Token()
			if err != nil {
				return nil, fmt.Errorf("el <resources> no se cierra: %w", err)
			}
			if fin, esFin := tok.(xml.EndElement); esFin && fin.Name.Local == "resources" {
				break
			}
			el, esInicio := tok.(xml.StartElement)
			if !esInicio {
				continue
			}
			r, err := recursoDeValor(d, el, m)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", el.Name.Local, err)
			}
			out = append(out, r)
		}
	}
}

// recursoDeValor lee un elemento hijo de <resources> y devuelve su entrada.
func recursoDeValor(d *xml.Decoder, el xml.StartElement, m *marcoRecursos) (recursoEntabla, error) {
	nombre := ""
	padre := ""
	tipo := el.Name.Local
	for _, a := range el.Attr {
		if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
			continue
		}
		switch a.Name.Local {
		case "name":
			nombre = a.Value
		case "parent":
			padre = a.Value
		case "type":
			if tipo == "item" {
				tipo = a.Value
			}
		}
	}
	if nombre == "" {
		return recursoEntabla{}, fmt.Errorf("un %s sin nombre", el.Name.Local)
	}
	if tipo == "style" {
		items, err := itemsDeEstilo(d, "style", m)
		if err != nil {
			return recursoEntabla{}, err
		}
		padreId := uint32(0)
		if padre != "" {
			id, err := referenciaDeFramework(padre, m)
			if err != nil {
				return recursoEntabla{}, fmt.Errorf("el padre %q: %w", padre, err)
			}
			padreId = id
		}
		return recursoEntabla{tipo: "style", clave: nombre, compleja: true, padre: padreId, items: items}, nil
	}

	if tipo == "string-array" || tipo == "integer-array" {
		items, err := itemsDeArray(d, el.Name.Local, tipo == "integer-array")
		if err != nil {
			return recursoEntabla{}, err
		}
		return recursoEntabla{tipo: "array", clave: nombre, compleja: true, items: items}, nil
	}
	if tipo == "plurals" {
		items, err := itemsDePlural(d, el.Name.Local)
		if err != nil {
			return recursoEntabla{}, err
		}
		return recursoEntabla{tipo: "plurals", clave: nombre, compleja: true, items: items}, nil
	}
	texto, err := textoDeElemento(d, el.Name.Local)
	if err != nil {
		return recursoEntabla{}, err
	}
	if tipo == "id" {
		// medido en la variante A del parrafo 29: el valor de un id es un booleano a cero
		return recursoEntabla{tipo: "id", clave: nombre, valor: valorEscribir{tipo: tipoBooleano}}, nil
	}
	v, err := valorDeTexto(tipo, nombre, texto, m)
	if err != nil {
		return recursoEntabla{}, err
	}
	// el fuente declara la forma del recurso, y esa forma se exige: un <color> que no casa como color
	// es un error del proyecto, no una cadena. Medidos en la variante A del parrafo 29: string sale
	// 0x03 con el texto en el pool global, color 0x1d con el ARGB, bool 0x12 y entero 0x10.
	switch tipo {
	case "string":
		v = valorEscribir{tipo: tipoCadena}
	case "color":
		if v.tipo != 0x1d {
			return recursoEntabla{}, fmt.Errorf("%s/%s declara color y el texto %q no lo es", tipo, nombre, texto)
		}
	case "bool":
		if v.tipo != tipoBooleano {
			return recursoEntabla{}, fmt.Errorf("%s/%s declara bool y el texto %q no es true ni false", tipo, nombre, texto)
		}
	case "integer":
		if v.tipo != tipoEnteroDec {
			return recursoEntabla{}, fmt.Errorf("%s/%s declara %s y el texto %q no es un numero", tipo, nombre, tipo, texto)
		}
	case "fraction":
		v, err = fraccionDeTexto(texto)
		if err != nil {
			return recursoEntabla{}, fmt.Errorf("%s/%s: %w", tipo, nombre, err)
		}
	default:
		return recursoEntabla{}, fmt.Errorf("el tipo %q de %s/%s no esta medido por este enlazador", tipo, tipo, nombre)
	}
	return recursoEntabla{tipo: tipo, clave: nombre, valor: v, texto: texto}, nil
}

// itemsDeArray: los `<item>` de un `string-array` o `integer-array`. Medido en el parrafo 95: el
// nombre de cada registro es 0x01000000 | (índice + 1), el padre 0 y la bandera 0x0001; el valor es
// una cadena (0x03) o un entero (0x10).
func itemsDeArray(d *xml.Decoder, nombre string, entero bool) ([]registroEscribir, error) {
	out := []registroEscribir{}
	indice := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("el %s no se cierra: %w", nombre, err)
		}
		if fin, esFin := tok.(xml.EndElement); esFin && fin.Name.Local == nombre {
			return out, nil
		}
		item, esInicio := tok.(xml.StartElement)
		if !esInicio || item.Name.Local != "item" {
			continue
		}
		texto, err := textoDeElemento(d, "item")
		if err != nil {
			return nil, err
		}
		v := valorEscribir{tipo: tipoCadena, texto: texto}
		if entero {
			v, err = enteroDeTexto(texto)
			if err != nil {
				return nil, fmt.Errorf("el item %q de %s no es un entero", texto, nombre)
			}
		}
		indice++
		out = append(out, registroEscribir{nombre: 0x01000000 | uint32(indice), valor: v})
	}
}

// itemsDePlural: los `<item quantity="one|other|…">` de un `<plurals>`. Los nombres de los registros
// son los ids de cantidad medidos con aapt2 (parrafo 95): other 0x01000004, zero 0x01000005,
// one 0x01000006, two 0x01000007, few 0x01000008 y many 0x01000009.
func itemsDePlural(d *xml.Decoder, nombre string) ([]registroEscribir, error) {
	cantidades := map[string]uint32{
		"other": 0x01000004, "zero": 0x01000005, "one": 0x01000006,
		"two": 0x01000007, "few": 0x01000008, "many": 0x01000009,
	}
	out := []registroEscribir{}
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("el %s no se cierra: %w", nombre, err)
		}
		if fin, esFin := tok.(xml.EndElement); esFin && fin.Name.Local == nombre {
			// aapt2 conserva el orden del fuente en los registros de un plural (medido: one antes
			// que other, no ordenado por id)
			return out, nil
		}
		item, esInicio := tok.(xml.StartElement)
		if !esInicio || item.Name.Local != "item" {
			continue
		}
		cantidad := ""
		for _, a := range item.Attr {
			if a.Name.Local == "quantity" {
				cantidad = a.Value
			}
		}
		id, ok := cantidades[cantidad]
		if !ok {
			return nil, fmt.Errorf("la cantidad %q de %s no esta medida", cantidad, nombre)
		}
		texto, err := textoDeElemento(d, "item")
		if err != nil {
			return nil, err
		}
		out = append(out, registroEscribir{nombre: id, valor: valorEscribir{tipo: tipoCadena, texto: texto}})
	}
}

// itemsDeEstilo anda los <item name="android:X">valor</item> de un estilo y los deja ordenados por su
// id de atributo, que es como los emite aapt2 (parrafo 27).
func itemsDeEstilo(d *xml.Decoder, nombre string, m *marcoRecursos) ([]registroEscribir, error) {
	out := []registroEscribir{}
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("el %s no se cierra: %w", nombre, err)
		}
		if fin, esFin := tok.(xml.EndElement); esFin && fin.Name.Local == nombre {
			sort.SliceStable(out, func(a, b int) bool { return out[a].nombre < out[b].nombre })
			return out, nil
		}
		item, esInicio := tok.(xml.StartElement)
		if !esInicio || item.Name.Local != "item" {
			continue
		}
		atributo := ""
		for _, a := range item.Attr {
			if a.Name.Local == "name" {
				atributo = a.Value
			}
		}
		sinPrefijo, ok := sinPrefijoAndroid(atributo)
		if !ok {
			return nil, fmt.Errorf("el item %q no es android: y aqui solo se midio ese prefijo", atributo)
		}
		id, err := m.idDeAtributo(sinPrefijo)
		if err != nil {
			return nil, err
		}
		texto, err := textoDeElemento(d, "item")
		if err != nil {
			return nil, err
		}
		v, err := valorDeTexto(sinPrefijo, sinPrefijo, texto, m)
		if err != nil {
			return nil, err
		}
		out = append(out, registroEscribir{nombre: id, valor: v})
	}
}

// sinPrefijoAndroid: los items de un estilo se escriben `android:windowBackground`; cualquier otro
// prefijo no esta medido.
func sinPrefijoAndroid(nombre string) (string, bool) {
	if !strings.HasPrefix(nombre, "android:") {
		return "", false
	}
	return strings.TrimPrefix(nombre, "android:"), true
}

// textoDeElemento consume un elemento hoja y devuelve su texto.
func textoDeElemento(d *xml.Decoder, nombre string) (string, error) {
	sb := strings.Builder{}
	for {
		tok, err := d.Token()
		if err != nil {
			return "", fmt.Errorf("%s sin cierre: %w", nombre, err)
		}
		switch v := tok.(type) {
		case xml.CharData:
			sb.Write([]byte(v))
		case xml.EndElement:
			return sb.String(), nil
		case xml.StartElement:
			return "", fmt.Errorf("%s lleva un hijo %q, y eso no esta medido", nombre, v.Name.Local)
		}
	}
}

// valorDeTexto decide el Res_value de un texto con las reglas medidas: primero los enum o flags del
// atributo del framework (con SU tipo de dato, parrafo 34), luego booleano, color, entero y por ultimo
// cadena. Las referencias `@…` las resuelve referenciaDe, que es otra funcion porque una referencia al
// propio paquete no se puede resolver hasta tener la tabla numerada.
func valorDeTexto(atributo, clave, texto string, m *marcoRecursos) (valorEscribir, error) {
	if strings.HasPrefix(texto, "@android:") {
		id, err := referenciaDeFramework(texto, m)
		if err != nil {
			return valorEscribir{}, err
		}
		return valorEscribir{tipo: tipoReferencia, datos: id}, nil
	}
	if strings.HasPrefix(texto, "@") {
		return valorEscribir{}, fmt.Errorf("la referencia %q de %s/%s apunta al paquete propio: se resuelve con la tabla numerada", texto, atributo, clave)
	}
	for nombre, v := range m.valoresDeAtributo(atributo) {
		if nombre == texto {
			return v, nil
		}
	}
	switch texto {
	case "true":
		return valorEscribir{tipo: tipoBooleano, datos: 0xffffffff}, nil
	case "false":
		return valorEscribir{tipo: tipoBooleano, datos: 0}, nil
	}
	if v, err := colorDeTexto(texto); err == nil {
		return v, nil
	}
	if v, err := enteroDeTexto(texto); err == nil {
		return v, nil
	}
	return valorEscribir{tipo: tipoCadena, datos: 0}, nil
}

// enteroDeTexto: un numero sale entero decimal (0x10) con los negativos tal cuales, que es como el
// manifiesto y los layouts del parrafo 30 escriben versionCode, -1 y -2.
func enteroDeTexto(texto string) (valorEscribir, error) {
	n, err := strconv.ParseInt(texto, 10, 64)
	if err != nil {
		return valorEscribir{}, fmt.Errorf("no es entero")
	}
	return valorEscribir{tipo: tipoEnteroDec, datos: uint32(int32(n))}, nil
}

// fraccionDeTexto: `50%` y `50%p` salen tipo fracción 0x06. Medido en el parrafo 105: 0.25/0.5/0.75
// usan radix 3 (`0x20000030`/`0x40000030`/`0x60000030`), 1.0 es `0x100`, 1.5 es `0xC00020` y el
// bit bajo distingue `%p` (`0x31`). La cuenta se hace en float32, como aapt2: 10% da `0x0CCCCD30`.
func fraccionDeTexto(texto string) (valorEscribir, error) {
	s := strings.TrimSpace(texto)
	porcentaje := false
	if strings.HasSuffix(s, "%p") {
		porcentaje, s = true, strings.TrimSuffix(s, "%p")
	} else if strings.HasSuffix(s, "%") {
		s = strings.TrimSuffix(s, "%")
	} else {
		return valorEscribir{}, fmt.Errorf("%q no es una fracción (falta %%)", texto)
	}
	f, err := strconv.ParseFloat(s, 32)
	if err != nil || f < 0 {
		return valorEscribir{}, fmt.Errorf("%q no es una fracción positiva", texto)
	}
	v := float32(f) / 100
	var datos uint32
	switch {
	case v == 0:
		datos = 0
	case v == 1:
		datos = 0x100
	case v < 1:
		// aapt2 redondea hacia arriba el valor de 24 bits: 10% da 0x0CCCCD30, no 0x0CCCCC30
		datos = uint32(math.Ceil(float64(v*float32(8388608.0))))<<8 | 0x30
	default:
		datos = uint32(math.Ceil(float64(v*float32(8388608.0)))) | 0x20
	}
	if porcentaje {
		datos |= 1
	}
	return valorEscribir{tipo: 0x06, datos: datos}, nil
}

// colorDeTexto: #RGB, #RRGGBB y #AARRGGBB salen como ARGB8 (0x1d), el tipo que llevo statusBarColor
// "#FFFFFF" a ffffffff en el fichero fijado.
func colorDeTexto(texto string) (valorEscribir, error) {
	if !strings.HasPrefix(texto, "#") {
		return valorEscribir{}, fmt.Errorf("no es color")
	}
	dig := texto[1:]
	switch len(dig) {
	case 3:
		dig = string([]byte{dig[0], dig[0], dig[1], dig[1], dig[2], dig[2]})
	case 6:
		dig = "ff" + dig
	case 8:
	default:
		return valorEscribir{}, fmt.Errorf("no es color")
	}
	n, err := strconv.ParseUint(dig, 16, 32)
	if err != nil {
		return valorEscribir{}, fmt.Errorf("no es color")
	}
	return valorEscribir{tipo: 0x1d, datos: uint32(n)}, nil
}

// referenciaDeFramework resuelve `@android:style/TextAppearance.Large` contra la tabla del SDK. Es lo
// que hace falta para el padre de un estilo y para un valor que apunta a un recurso del framework.
func referenciaDeFramework(texto string, m *marcoRecursos) (uint32, error) {
	cuerpo := strings.TrimPrefix(texto, "@")
	i := strings.Index(cuerpo, ":")
	if i < 0 {
		return 0, fmt.Errorf("%q apunta al paquete propio, y eso se resuelve con la tabla numerada", texto)
	}
	prefijo, resto := cuerpo[:i], cuerpo[i+1:]
	if prefijo != "android" {
		return 0, fmt.Errorf("el prefijo %q no esta medido: solo se resuelve android:", prefijo)
	}
	slash := strings.Index(resto, "/")
	if slash < 0 {
		return 0, fmt.Errorf("%q no es de forma @android:tipo/nombre", texto)
	}
	return m.idDe(resto[:slash], resto[slash+1:])
}
