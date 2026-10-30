package main

// El escritor de tablas de cadenas de recursos, la pieza que comparten el AXML y el resources.arsc
// de la Fase 3. Esta medido sobre los tres pools que aapt2 dejo en el fixture del parrafo 26: el
// global (@12, 56 bytes, UTF-8), el de nombres de tipo (@356, 80, UTF-16) y el de claves (@436,
// 68, UTF-8). Esos tres salen byte a byte iguales, que es lo que comprueba el test.
//
// Lo que se copio de los bytes y no de un manual: el orden de los campos de la cabecera, que los
// offsets son RELATIVOS al inicio de los datos (no al inicio del trozo), que el modo UTF-8 escribe
// DOS longitudes (bytes y unidades utf-16) y el UTF-16 una sola, y que cada cadena termina con un
// cero - dos bytes cero en UTF-16 - y el area se rellena hasta multiple de cuatro.

import (
	"unicode/utf16"
)

// escribePool laying out una tabla de cadenas como trozo RES_STRING_POOL. Con utf8 en verdadero usa
// el modo de un byte (o dos, si la longitud pasa de 127) por longitud; con falso escribe UTF-16 con
// longitudes de una unidad.
func escribePool(cadenas []string, utf8 bool) []byte {
	const cabecera = 28
	// putU16 y putU32 son los del escritor de dex: mismas unidades, misma extremidad
	put16, put32 := putU16, putU32
	datos := []byte{}
	offsets := []uint32{}
	for _, s := range cadenas {
		offsets = append(offsets, uint32(len(datos)))
		if utf8 {
			datos = apilarLargosUTF8(datos, len(s), utf16Len(s))
			datos = append(datos, s...)
			datos = append(datos, 0)
			continue
		}
		// UTF-16: una unidad de longitud (dos si pasan de 32767, con el alto puesto), los
		// caracteres y un cero de terminacion. Las unidades salen de utf16.Encode, que es lo que
		// escribe aapt2: un caracter fuera del plano base son dos unidades, no una.
		unidades := utf16.Encode([]rune(s))
		n := len(unidades)
		if n <= 0x7fff {
			datos = put16(datos, uint16(n))
		} else {
			datos = put16(datos, uint16(0x8000|n>>16))
			datos = put16(datos, uint16(n))
		}
		for _, u := range unidades {
			datos = put16(datos, u)
		}
		datos = put16(datos, 0)
	}
	for len(datos)%4 != 0 {
		datos = append(datos, 0)
	}
	out := put16(nil, trozoPool)
	out = put16(out, cabecera)
	out = put32(out, uint32(cabecera+4*len(cadenas)+len(datos)))
	out = put32(out, uint32(len(cadenas)))
	out = put32(out, 0) // styleCount: los pools de recursos de este proyecto no llevan estilos
	flags := uint32(0)
	if utf8 {
		flags = 0x100
	}
	out = put32(out, flags)
	out = put32(out, uint32(cabecera+4*len(cadenas)))
	out = put32(out, 0) // stylesStart
	for _, o := range offsets {
		out = put32(out, o)
	}
	return append(out, datos...)
}

// apilarLargosUTF8: dos longitudes de una o dos unidades, con el bit alto avisando de que sigue
// otra. Medido con cadenas cortas; la forma larga es la misma regla leida al reves por el lector.
func apilarLargosUTF8(b []byte, bytes, unidades int) []byte {
	b = pilaUnaLongitud(b, bytes)
	return pilaUnaLongitud(b, unidades)
}

func pilaUnaLongitud(b []byte, n int) []byte {
	if n > 127 {
		b = append(b, byte(0x80|n>>8), byte(n))
		return b
	}
	return append(b, byte(n))
}

// leePoolCrudo devuelve los bytes exactos del trozo que empieza en off, para comparar la salida de
// escribePool contra el fichero de aapt2.
func leePoolCrudo(b []byte, off int) []byte {
	p, err := leePool(b, off)
	if err != nil || p.tamaño <= 0 {
		return nil
	}
	return b[off : off+p.tamaño]
}
