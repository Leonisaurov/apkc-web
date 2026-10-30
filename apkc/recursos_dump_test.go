package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Instrumento de medición del §24: la estructura de los ficheros binarios que escribe aapt2 (AXML y
// resources.arsc), leida trozo a trozo desde los offsets que el propio fichero declara. Se ejecuta
// con la variable de entorno APKC_ARBOLES apuntando a las rutas (separadas por espacios) y sin ella
// se salta, porque necesita el AndroidManifest.xml y el resources.arsc que produce `apkc build` con
// el SDK del equipo: lo que emita este fork en la Fase 3 se compara contra esos mismos bytes.
//
// Lo unico que se da por sabido es el encabezado comun (type 0..1, headerSize 2..3, size 4..7), que
// sale de los primeros bytes vistos con xxd. Todo lo demas esta deducido de los ficheros de aapt2,
// como queda escrito en docs/evidencia.md §24.

// los helpers de lectura que usan solo este instrumento de medicion
func u16en(b []byte, i int) uint16 { return binary.LittleEndian.Uint16(b[i:]) }
func u32en(b []byte, i int) uint32 { return binary.LittleEndian.Uint32(b[i:]) }

func nombreTrozo(t uint16) string {
	switch t {
	case 0x0001:
		return "STRING_POOL"
	case 0x0002:
		return "TABLE"
	case 0x0003:
		return "XML"
	case 0x0100:
		return "XML_START_NS"
	case 0x0101:
		return "XML_END_NS"
	case 0x0102:
		return "XML_START_TAG"
	case 0x0103:
		return "XML_END_TAG"
	case 0x0104:
		return "XML_TEXT"
	case 0x0180:
		return "XML_RESOURCE_MAP"
	case 0x0200:
		return "TABLE_PACKAGE"
	case 0x0201:
		return "TABLE_TYPE"
	case 0x0202:
		return "TABLE_TYPE_SPEC"
	case 0x0203:
		return "TABLE_LIBRARY"
	}
	return fmt.Sprintf("TROZO_%#04x", t)
}

func TestArbolDeTroz(t *testing.T) {
	rutas := os.Getenv("APKC_ARBOLES")
	if rutas == "" {
		t.Skip("sin APKC_ARBOLES")
	}
	for _, ruta := range strings.Fields(rutas) {
		b, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("== %s (%d bytes, trozo raiz %s hs=%d size=%d)",
			ruta, len(b), nombreTrozo(u16en(b, 0)), u16en(b, 2), u32en(b, 4))
		recorre(b, int(u16en(b, 2)), len(b), 1, t)
	}
}

// recorre anda los trozos hermanos entre `desde` y `hasta`. Un trozo que declare un tamano que se
// sale del fichero se dice y se corta el recorrido: seguir andando por ahi daria un arbol inventado.
func recorre(b []byte, desde, hasta, nivel int, t *testing.T) {
	pad := strings.Repeat("  ", nivel)
	off := desde
	for off+8 <= hasta {
		tipo := u16en(b, off)
		hs := int(u16en(b, off+2))
		size := int(u32en(b, off+4))
		if size <= 0 || off+size > hasta {
			t.Logf("%s@%-5d %s hs=%d size=%d SE SALE (%d) — se corta el recorrido", pad, off, nombreTrozo(tipo), hs, size, hasta)
			return
		}
		t.Logf("%s@%-5d %-18s hs=%-4d size=%-5d %s", pad, off, nombreTrozo(tipo), hs, size,
			cabecera(b, off, tipo, hs))
		switch tipo {
		case 0x0001:
			cadenas(b, off, pad, t)
		case 0x0180:
			idsAtributo(b, off+hs, off+size, pad, t)
		case 0x0102:
			atributos(b, off, hs, size, pad, t)
		case 0x0200:
			paquete(b, off, hs, size, pad, t)
			recorre(b, off+hs, off+size, nivel+1, t)
		case 0x0201:
			volcarBloqueTipo(b, off, hs, size, pad, t)
		}
		off += size
	}
	if off != hasta {
		t.Logf("%s@%-5d sobran %d bytes tras el ultimo trozo", pad, off, hasta-off)
	}
}

// cabecera imprime los campos fijos que cada tipo de trozo tiene despues del encabezado comun.
func cabecera(b []byte, off int, tipo uint16, hs int) string {
	switch tipo {
	case 0x0001:
		flags := u32en(b, off+16)
		return fmt.Sprintf("cadenas=%d estilos=%d flags=%#x stringsStart=%d stylesStart=%d (ordenadas=%v, utf8=%v)",
			u32en(b, off+8), u32en(b, off+12), flags, u32en(b, off+20), u32en(b, off+24),
			flags&1 != 0, flags&0x100 != 0)
	case 0x0002:
		return fmt.Sprintf("packageCount=%d", u32en(b, off+8))
	case 0x0100, 0x0101:
		return fmt.Sprintf("line=%d comentario=%d ns=%d nombre=%d",
			u32en(b, off+8), u32en(b, off+12), u32en(b, off+16), u32en(b, off+20))
	case 0x0102, 0x0103:
		ext := off + 16 // el nodo son 8 de cabecera + 4 de linea + 4 de comentario
		return fmt.Sprintf("line=%d comentario=%d ns=%d nombre=%d attrStart=%d attrSize=%d attrNum=%d id/cl/st=%d/%d/%d",
			u32en(b, off+8), u32en(b, off+12), u32en(b, ext+0), u32en(b, ext+4),
			u16en(b, ext+8), u16en(b, ext+10), u16en(b, ext+12),
			u16en(b, ext+14), u16en(b, ext+16), u16en(b, ext+18))
	case 0x0180:
		return fmt.Sprintf("(%d ids de atributo)", (int(u32en(b, off+4))-hs)/4)
	case 0x0200:
		nom := ""
		for i := 0; i < 128; i++ {
			c := u16en(b, off+12+i*2)
			if c == 0 {
				break
			}
			nom += string(rune(c))
		}
		return fmt.Sprintf("id=%d nombre=%q typeStrings=%d lastPublicType=%d keyStrings=%d lastPublicKey=%d typeIdOffset=%d",
			u32en(b, off+8), nom, u32en(b, off+268), u32en(b, off+272), u32en(b, off+276),
			u32en(b, off+280), u32en(b, off+284))
	case 0x0201:
		return fmt.Sprintf("id=%d flags=%#x entradas=%d entriesStart=%d configSize=%d",
			b[off+8], b[off+9], u32en(b, off+12), u32en(b, off+16), u32en(b, off+20))
	}
	return ""
}

// cadenas: la tabla, decodificando cada cadena con su prefijo de longitud. El modo (utf-8 o
// utf-16) lo dice el flag 0x100; se imprime tambien el crudo de las tres primeras por si el flag
// miente.
func cadenas(b []byte, off int, pad string, t *testing.T) {
	hs := int(u16en(b, off+2))
	size := int(u32en(b, off+4))
	n := int(u32en(b, off+8))
	flags := u32en(b, off+16)
	// los offsets de las cadenas son RELATIVOS al principio de los datos (stringsStart), no al
	// principio del trozo: sumarlos a `off` da basura que parece legible
	inicio := off + int(u32en(b, off+20))
	for i := 0; i < n; i++ {
		o := off + hs + i*4
		desde := inicio + int(u32en(b, o))
		s := leeCadenaCruda(b, desde, off+size, flags&0x100 != 0)
		t.Logf("%s   [%2d] @%-4d %q", pad, i, desde, s)
	}
}

func leeCadenaCruda(b []byte, desde, hasta int, esUTF8 bool) string {
	if desde >= hasta {
		return ""
	}
	if esUTF8 {
		// dos longitudes: la de bytes y la de unidades utf-16; cada una es un byte o dos con el
		// alto puesto
		p := desde
		n1 := int(b[p])
		p++
		if n1&0x80 != 0 {
			n1 = (n1&0x7f)<<8 | int(b[p])
			p++
		}
		n2 := int(b[p])
		p++
		if n2&0x80 != 0 {
			n2 = (n2&0x7f)<<8 | int(b[p])
			p++
		}
		fin := p + n1
		if fin > hasta {
			fin = hasta
		}
		return string(b[p:fin])
	}
	n := int(u16en(b, desde))
	p := desde + 2
	if n&0x8000 != 0 {
		n = ((n & 0x7fff) << 16) | int(u16en(b, p))
		p += 2
	}
	var sb strings.Builder
	for i := 0; i < n && p+1 < hasta; i++ {
		sb.WriteRune(rune(u16en(b, p)))
		p += 2
	}
	return sb.String()
}

func idsAtributo(b []byte, desde, hasta int, pad string, t *testing.T) {
	for o := desde; o+4 <= hasta; o += 4 {
		t.Logf("%s   id %#08x", pad, u32en(b, o))
	}
}

// atributos: el trozo XML_START_TAG lleva, tras su cabecera de 16, un ResXMLTree_attrExt con el
// nombre y la cuenta de atributos; cada atributo son 20 bytes (nombre, valor crudo, y un ResValue
// de ocho: tamano, reserva, tipo de dato y datos).
func atributos(b []byte, off, hs, size int, pad string, t *testing.T) {
	ext := off + 16
	// un END_TAG no tiene attrExt: su tamano es justo 16 + 8
	if off+size <= ext+20 {
		return
	}
	nAttr := int(u16en(b, ext+12))
	a := ext + int(u16en(b, ext+8))
	for i := 0; i < nAttr; i++ {
		t.Logf("%s   attr #%d ns=%d nombre=%d raw=%d tam=%d res0=%d tipo=%#02x datos=%#08x", pad, i,
			u32en(b, a), u32en(b, a+4), int32(u32en(b, a+8)), u16en(b, a+12), b[a+14], b[a+15], u32en(b, a+16))
		a += 20
	}
}

// paquete: se listan los pools de nombres de tipo y de clave que anuncia el propio paquete. El
// tamano de las tablas se deduce del salto entre los dos offsets, medido: typeStrings=288 y
// keyStrings=368 dejan 80 bytes para el primer pool, que son su cabecera de 28 mas 3 cadenas.
func paquete(b []byte, off, hs, size int, pad string, t *testing.T) {
	for _, par := range [][2]int{{268, 0}, {276, 1}} {
		o := off + int(u32en(b, off+par[0]))
		t.Logf("%s   pool declarado en +%d:", pad, par[0])
		cadenas(b, o, pad+"     ", t)
	}
}

// volcarBloqueTipo: las entradas de un tipo, con su clave y su Res_value. Los offsets son RELATIVOS a
// entriesStart, no al principio del trozo: sumarlos a `off` da la cabecera del propio tipo.
func volcarBloqueTipo(b []byte, off, hs, size int, pad string, t *testing.T) {
	n := int(u32en(b, off+12))
	entriesStart := int(u32en(b, off+16))
	t.Logf("%s   offsets de entrada (entriesStart=%d, array en +%d):", pad, entriesStart, hs)
	for i := 0; i < n; i++ {
		o := off + hs + i*4
		e := off + entriesStart + int(u32en(b, o))
		if e+16 > off+size || e+16 > len(b) {
			t.Logf("%s     #%d @%d fuera (%d)", pad, i, e, off+size)
			continue
		}
		clave := u32en(b, e+4)
		v := e + int(u16en(b, e))
		t.Logf("%s   #%d @%-4d tam=%d flags=%#04x clave=%d value(tam=%d res0=%d tipo=%#02x datos=%#08x)",
			pad, i, e, u16en(b, e), u16en(b, e+2), clave, u16en(b, v), b[v+2], b[v+3], u32en(b, v+4))
		// config del bloque: los primeros 12 bytes del ResTable_config (tamano, tipo, y el identificador
		// de locale/densidad/API que ocupa 8)
		t.Logf("%s     config: % x", pad, b[off+20:off+32])
		if u16en(b, e) == 16 && u16en(b, e+2)&1 != 0 {
			padre, cuenta := u32en(b, e+8), u32en(b, e+12)
			// se recorre HASTA la siguiente entrada (o el final del bloque) y no hasta el final del
			// bloque: con varias entradas en un tipo, el recorrido viejo contaba registros ajenos y
			// imprimia "11 registros" para una entrada que declaraba 5 (parrafo 27)
			fin := off + size
			if n > 1 && i+1 < n {
				fin = off + entriesStart + int(u32en(b, off+hs+(i+1)*4))
			}
			registros := []string{}
			for k := e + 16; k+12 <= fin; k += 12 {
				registros = append(registros, fmt.Sprintf("%#08x/t%#02x/%#08x",
					u32en(b, k), b[k+7], u32en(b, k+8)))
			}
			t.Logf("%s     complejo: padre=%#08x recuento=%d registros=%d %v",
				pad, padre, cuenta, len(registros), registros)
		}
	}
}
