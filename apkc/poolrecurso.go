package main

// El escritor de tablas de cadenas de recursos, la pieza que comparten el AXML y el resources.arsc
// de la Fase 3. Esta medido sobre los tres pools que aapt2 dejo en el fixture del parrafo 26: el
// global (@12, 56 bytes, UTF-8), el de nombres de tipo (@356, 80, UTF-16) y el de claves (@436,
// 68, UTF-8). Esos tres salen byte a byte iguales, que es lo que comprueba el test.
//
// Lo que se copio de los bytes y no de un manual: el orden de los campos de la cabecera, que los
// offsets son RELATIVOS al inicio de los datos (no al inicio del trozo), que el modo UTF-8 escribe
// DOS longitudes (unidades utf-16 y después bytes; el orden se midió con «Hola & adiós ñ» en un
// resources.arsc de aapt2 35.0.1) y el UTF-16 una sola, y que cada cadena termina con un
// cero - dos bytes cero en UTF-16 - y el area se rellena hasta multiple de cuatro.
//
// El modo «UTF-8» de aapt2 no es UTF-8: los caracteres fuera del plano base van como su par
// sustituto, tres bytes por unidad (CESU-8). Con «😀» (U+1F600) el prefijo de unidades vale 2 y el
// de bytes 6, no 4; medido en el resources.arsc de aapt2 35.0.1 con «Hola & adiós ñ 😀», donde el
// primer prefijo es 17 y el segundo 23.

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
			cesu, unidades := codificaCESU8(s)
			datos = apilarLargosUTF8(datos, len(cesu), unidades)
			datos = append(datos, cesu...)
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

// apilarLargosUTF8: las dos longitudes del pool UTF-8, en el orden del formato de Android
// (ResStringPool::string8At): primero la cuenta de unidades utf-16 y después la de bytes. Con texto
// ASCII las dos coinciden y el orden no se ve —que es por lo que el fixture del §26, todo ASCII,
// salía byte a byte con el orden invertido—; con «Hola & adiós ñ» valen 14 y 16 y el cero solo cae
// tras los 16 bytes. Medido sobre el resources.arsc de aapt2 35.0.1 del crítico de la Fase 1.
func apilarLargosUTF8(b []byte, bytes, unidades int) []byte {
	b = pilaUnaLongitud(b, unidades)
	return pilaUnaLongitud(b, bytes)
}

// codificaCESU8 pasa la cadena a unidades utf-16 y las escribe con la forma que usa aapt2 en el
// pool «UTF-8»: uno o dos bytes para el plano base y tres por unidad para el suplementario. Un
// carácter fuera del plano base es un par sustituto, así que ocupa 3+3 bytes —no los 4 del UTF-8
// de Go—; por eso también devuelve la cuenta de unidades, que es el primer prefijo del formato.
func codificaCESU8(s string) ([]byte, int) {
	unidades := utf16.Encode([]rune(s))
	b := make([]byte, 0, len(unidades))
	for _, u := range unidades {
		switch {
		case u < 0x80:
			b = append(b, byte(u))
		case u < 0x800:
			b = append(b, byte(0xc0|u>>6), byte(0x80|u&0x3f))
		default:
			b = append(b, byte(0xe0|u>>12), byte(0x80|u>>6&0x3f), byte(0x80|u&0x3f))
		}
	}
	return b, len(unidades)
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
