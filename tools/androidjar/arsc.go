package main

// Escritor de un resources.arsc mínimo con el paquete `android`. Copia el
// layout que `apkc/arscwriter.go` midió contra aapt2: cabecera de tabla,
// paquete de 288, pools de tipo y claves, un RES_TABLE_TYPE_SPEC y un
// RES_TABLE_TYPE por tipo. Lo que no copia son las configuraciones múltiples ni
// el pool global de valores: `apkc/marco.go` no los lee.

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
)

const (
	trozoPool    = uint16(0x0001)
	trozoTabla   = uint16(0x0002)
	trozoPaquete = uint16(0x0200)
	trozoTipo    = uint16(0x0201)
	trozoSpec    = uint16(0x0202)
)

// Los 23 nombres del pool del framework, en el orden que da el id de tipo
// (posición + 1). Los ids 1-17 son los que usan las plantillas; el hueco 0x16 y
// los dos últimos mantienen la longitud que espera TestMarco.
var tiposDelFramework = []string{
	"attr", "id", "style", "string", "dimen", "color", "array", "drawable",
	"layout", "anim", "animator", "interpolator", "mipmap", "integer",
	"transition", "raw", "bool", "^attr-private", "fraction", "menu", "plurals",
	"?", "xml",
}

type recurso struct {
	tipo   string
	nombre string
	id     uint32
}

type defAtributo struct {
	formato uint32
	flags   bool
	items   map[string]int64
}

type valorARSC struct {
	tipo  byte
	datos uint32
}

type registroARSC struct {
	nombre uint32
	valor  valorARSC
}

type entradaARSC struct {
	clave     int
	compleja  bool
	registros []registroARSC
}

type bloqueARSC struct {
	entradas []entradaARSC
}

type tipoARSC struct {
	id     byte
	flags  []uint32
	bloque bloqueARSC
}

type tablaARSC struct {
	nombrePaquete string
	nombresTipo   []string
	claves        []string
	tipos         []tipoARSC
}

func escribeARSC(r []recurso, attrs map[string]*defAtributo) ([]byte, error) {
	// claves: una por entrada, en orden estable
	claves := []string{}
	indiceClave := map[string]int{}
	for _, e := range r {
		if _, ok := indiceClave[e.nombre]; ok {
			continue
		}
		indiceClave[e.nombre] = len(claves)
		claves = append(claves, e.nombre)
	}
	porTipo := map[string][]recurso{}
	for _, e := range r {
		porTipo[e.tipo] = append(porTipo[e.tipo], e)
	}
	tabla := tablaARSC{nombrePaquete: "android", nombresTipo: tiposDelFramework, claves: claves}
	for id, nombre := range tiposDelFramework {
		es := porTipo[nombre]
		if len(es) == 0 {
			continue
		}
		sort.Slice(es, func(i, j int) bool { return es[i].id&0xffff < es[j].id&0xffff })
		max := int(es[len(es)-1].id & 0xffff)
		bloque := bloqueARSC{entradas: make([]entradaARSC, max+1)}
		for i := range bloque.entradas {
			bloque.entradas[i] = entradaARSC{clave: -1}
		}
		flags := make([]uint32, max+1)
		for _, e := range es {
			k := int(e.id & 0xffff)
			entrada := entradaARSC{clave: indiceClave[e.nombre]}
			if nombre == "attr" {
				if d := attrs[e.nombre]; d != nil && (d.formato != 0 || len(d.items) > 0) {
					entrada.compleja = true
					entrada.registros = append(entrada.registros, registroARSC{
						nombre: 0x01000000,
						valor:  valorARSC{tipo: 0x10, datos: d.formato},
					})
					nombres := make([]string, 0, len(d.items))
					for n := range d.items {
						nombres = append(nombres, n)
					}
					sort.Strings(nombres)
					for _, n := range nombres {
						idItem, ok := idDeTipo(r, "id", n)
						if !ok {
							continue
						}
						tipo := byte(0x10)
						if d.flags {
							tipo = 0x11
						}
						entrada.registros = append(entrada.registros, registroARSC{
							nombre: idItem,
							valor:  valorARSC{tipo: tipo, datos: uint32(d.items[n])},
						})
					}
				}
			}
			bloque.entradas[k] = entrada
			flags[k] = 0x40000000
		}
		tabla.tipos = append(tabla.tipos, tipoARSC{id: byte(id + 1), flags: flags, bloque: bloque})
	}
	return escribeTablaARSC(tabla)
}

func idDeTipo(r []recurso, tipo, nombre string) (uint32, bool) {
	for _, e := range r {
		if e.tipo == tipo && e.nombre == nombre {
			return e.id, true
		}
	}
	return 0, false
}

func escribeTablaARSC(t tablaARSC) ([]byte, error) {
	unidades := utf16.Encode([]rune(t.nombrePaquete))
	if len(unidades) == 0 || len(unidades) > 127 {
		return nil, fmt.Errorf("nombre de paquete fuera de 128 unidades UTF-16")
	}
	nombres := escribePool(t.nombresTipo, false)
	claves := escribePool(t.claves, false)
	global := escribePool(nil, false)
	interior := []byte{}
	for _, ti := range t.tipos {
		interior = append(interior, escribeSpec(ti)...)
		interior = append(interior, escribeBloque(ti)...)
	}
	const cabecera = 288
	cuerpo := putU32(nil, 1)
	for _, u := range unidades {
		cuerpo = putU16(cuerpo, u)
	}
	for len(cuerpo) < 4+256 {
		cuerpo = putU16(cuerpo, 0)
	}
	cuerpo = putU32(cuerpo, cabecera)
	cuerpo = putU32(cuerpo, 0)
	cuerpo = putU32(cuerpo, uint32(cabecera+len(nombres)))
	cuerpo = putU32(cuerpo, 0)
	cuerpo = putU32(cuerpo, 0)
	paquete := trozo(trozoPaquete, cabecera, cabecera+len(nombres)+len(claves)+len(interior))
	paquete = append(paquete, cuerpo...)
	paquete = append(paquete, nombres...)
	paquete = append(paquete, claves...)
	paquete = append(paquete, interior...)
	out := trozo(trozoTabla, 12, 12+len(global)+len(paquete))
	out = putU32(out, 1)
	out = append(out, global...)
	return append(out, paquete...), nil
}

func trozo(tipo uint16, hs, size int) []byte {
	b := putU16(nil, tipo)
	b = putU16(b, uint16(hs))
	return putU32(b, uint32(size))
}

func escribeSpec(ti tipoARSC) []byte {
	n := len(ti.bloque.entradas)
	b := trozo(trozoSpec, 16, 16+4*n)
	b = append(b, ti.id, 0)
	b = putU16(b, 1)
	b = putU32(b, uint32(n))
	for _, f := range ti.flags {
		b = putU32(b, f)
	}
	return b
}

func escribeBloque(ti tipoARSC) []byte {
	const cabecera = 84
	entradas := []byte{}
	offsets := []uint32{}
	for _, e := range ti.bloque.entradas {
		if e.clave < 0 {
			offsets = append(offsets, 0xffffffff)
			continue
		}
		offsets = append(offsets, uint32(len(entradas)))
		entradas = apilaEntrada(entradas, e)
	}
	entradasStart := cabecera + 4*len(ti.bloque.entradas)
	b := trozo(trozoTipo, cabecera, entradasStart+len(entradas))
	b = append(b, ti.id, 0)
	b = putU16(b, 0)
	b = putU32(b, uint32(len(ti.bloque.entradas)))
	b = putU32(b, uint32(entradasStart))
	config := make([]byte, 64)
	binary.LittleEndian.PutUint32(config, 64)
	b = append(b, config...)
	for _, o := range offsets {
		b = putU32(b, o)
	}
	return append(b, entradas...)
}

func apilaEntrada(b []byte, e entradaARSC) []byte {
	if !e.compleja {
		b = putU16(b, 8)
		b = putU16(b, 0)
		b = putU32(b, uint32(e.clave))
		return append(b, 8, 0, 0, 0, 0, 0, 0, 0)
	}
	b = putU16(b, 16)
	b = putU16(b, 0)
	b = putU32(b, uint32(e.clave))
	b = putU32(b, 0)
	b = putU32(b, uint32(len(e.registros)))
	for _, r := range e.registros {
		b = putU32(b, r.nombre)
		b = putU16(b, 8)
		b = append(b, 0, r.valor.tipo)
		b = putU32(b, r.valor.datos)
	}
	return b
}

func escribePool(cadenas []string, utf8Mode bool) []byte {
	const cabecera = 28
	datos := []byte{}
	offsets := []uint32{}
	for _, s := range cadenas {
		offsets = append(offsets, uint32(len(datos)))
		if !utf8Mode {
			u := utf16.Encode([]rune(s))
			datos = putU16(datos, uint16(len(u)))
			for _, r := range u {
				datos = putU16(datos, r)
			}
			datos = putU16(datos, 0)
			continue
		}
		b := []byte(s)
		datos = append(datos, byte(len(b)), byte(len(b)>>8))
		// El modo UTF-8 de aapt2 escribe dos longitudes: unidades UTF-16 y
		// bytes. Para ASCII son iguales; para el resto, se calcula.
		unidades := len(utf16.Encode([]rune(s)))
		datos[len(datos)-2] = byte(unidades)
		datos[len(datos)-1] = byte(unidades >> 8)
		datos = append(datos, b...)
		datos = append(datos, 0)
	}
	for len(datos)%4 != 0 {
		datos = append(datos, 0)
	}
	out := putU16(nil, trozoPool)
	out = putU16(out, cabecera)
	out = putU32(out, uint32(cabecera+4*len(cadenas)+len(datos)))
	out = putU32(out, uint32(len(cadenas)))
	out = putU32(out, 0)
	flags := uint32(0)
	if utf8Mode {
		flags = 0x100
	}
	out = putU32(out, flags)
	out = putU32(out, uint32(cabecera+4*len(cadenas)))
	out = putU32(out, 0)
	for _, o := range offsets {
		out = putU32(out, o)
	}
	return append(out, datos...)
}

func putU16(b []byte, v uint16) []byte { return append(b, byte(v), byte(v>>8)) }
func putU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// formatoDe convierte el `format="..."` de attrs.xml en la máscara de
// ResTable_map que lee `marco.esFlags`. Los bits son los de ResTable_map.
func formatoDe(s string) uint32 {
	var m uint32
	for _, p := range strings.Split(s, "|") {
		switch strings.TrimSpace(p) {
		case "reference":
			m |= 0x00000001
		case "string":
			m |= 0x00000002
		case "integer":
			m |= 0x00000004
		case "boolean":
			m |= 0x00000008
		case "color":
			m |= 0x00000010
		case "float":
			m |= 0x00000020
		case "dimension":
			m |= 0x00000040
		case "fraction":
			m |= 0x00000080
		case "enum":
			m |= 0x00010000
		case "flags":
			m |= 0x00020000
		}
	}
	return m
}
