package main

// Lectura de lo que escribe `aapt2`: el AXML de los ficheros binarios XML y la tabla de recursos
// `resources.arsc`. Nace de la medicion del parrafo 24 de docs/evidencia.md, donde cada campo que se
// lee aqui esta sacado de los offsets que los propios ficheros declaran; nada de lo que hay debajo
// de un encabezado se interpreta por recuerdo. La forma del valor complejo de un estilo esta medida
// sobre variantes del proyecto (parrafos 26 y 27); lo que sigue sin interpretarse es el
// `ResTable_config` (64 bytes guardados crudos) y el significado de los bits de `flags`.
//
// El lector existe para que el escritor de la Fase 3 tenga con que compararse byte a byte: si lo que
// emite este fork no se lee con esto, o se lee con otras cuentas, la diferencia esta senalada antes
// de tocar un telefono. Lo que aun no esta medido - el `ResTable_config` (64 bytes) y el valor
// complejo de un estilo - viaja como bytes, no como significados.

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

func u16re(b []byte, i int) uint16 { return binary.LittleEndian.Uint16(b[i:]) }
func u32re(b []byte, i int) uint32 { return binary.LittleEndian.Uint32(b[i:]) }

const (
	trozoPool           = uint16(0x0001)
	trozoTabla          = uint16(0x0002)
	trozoXML            = uint16(0x0003)
	trozoInicioNS       = uint16(0x0100)
	trozoFinNS          = uint16(0x0101)
	trozoInicioEtiqueta = uint16(0x0102)
	trozoFinEtik        = uint16(0x0103)
	trozoMapaRec        = uint16(0x0180)
	trozoPaquete        = uint16(0x0200)
	trozoTipo           = uint16(0x0201)
	trozoSpec           = uint16(0x0202)
)

// pool es la tabla de cadenas, en los dos modos que se midieron: UTF-16 (flags 0) en el manifiesto,
// UTF-8 (flags 0x100) en la tabla de recursos y en las claves.
type tablaCadenas struct {
	cadenas []string
	utf8    bool
	tamaño  int
	// estilos es el styleCount declarado. Los pools que se leen aqui son todos de 0; un pool con
	// spans de estilo (el global del android.jar trae 1.307) tiene otra area de offsets y lee
	// cadenas de otra manera, asi que se comprueba en vez de suponer.
	estilos uint32
}

// leePool lee la tabla desde su offset y responde tambien cuanto ocupa. Un offset de cadena que se
// salga del area es un error: es la unica forma de que un fichero mal alineado no pase por bueno.
func leePool(b []byte, off int) (tablaCadenas, error) {
	if off+28 > len(b) {
		return tablaCadenas{}, fmt.Errorf("the string pool at %d does not fit %d bytes", off, len(b))
	}
	if u16re(b, off) != trozoPool {
		return tablaCadenas{}, fmt.Errorf("at %d there is a chunk %#04x, not a string pool", off, u16re(b, off))
	}
	hs := int(u16re(b, off+2))
	size := int(u32re(b, off+4))
	if size <= 0 || off+size > len(b) || hs < 28 {
		return tablaCadenas{}, fmt.Errorf("string pool at %d claims header %d and size %d in a %d byte file", off, hs, size, len(b))
	}
	n := int(u32re(b, off+8))
	flags := u32re(b, off+16)
	inicio := off + int(u32re(b, off+20))
	if inicio > off+size {
		return tablaCadenas{}, fmt.Errorf("the string pool at %d puts its data at %d, past its own end", off, inicio)
	}
	p := tablaCadenas{utf8: flags&0x100 != 0, tamaño: size, estilos: u32re(b, off+12)}
	for i := 0; i < n; i++ {
		o := off + hs + i*4
		if o+4 > off+size {
			return p, fmt.Errorf("string #%d has no offset inside the pool (it ends at %d)", i, off+size)
		}
		desde := inicio + int(u32re(b, o))
		s, hasta, err := leeCadena(b, desde, off+size, p.utf8)
		if err != nil {
			return p, fmt.Errorf("string #%d: %w", i, err)
		}
		p.cadenas = append(p.cadenas, s)
		_ = hasta
	}
	return p, nil
}

// leeCadena: la forma del formato, con sus dos prefijos de longitud. En UTF-8 van primero la
// cuenta de unidades utf-16 y después la de bytes; en UTF-16 solo la cuenta de caracteres. El
// terminador es un cero, y por eso el modo UTF-8 no puede contener un 00 suelto.
//
// El orden es el de ResStringPool::string8At de Android (u16len primero, byte length después) y
// está medido sobre un resources.arsc de aapt2 35.0.1 con «Hola & adiós ñ»: los dos prefijos valen
// 14 y 16, y el offset de la cadena más 16 —no 14— es el cero que cierra. Leer el primero como
// bytes partía la cadena y decía «does not end with a zero byte».
//
// Los bytes de ese modo son CESU-8, no UTF-8: aapt2 escribe cada unidad de un carácter fuera del
// plano base como tres bytes, así que «😀» son 2 unidades y 6 bytes (el par sustituto), no 4. Por
// eso se decodifican a unidades utf-16 antes de validar la cuenta y de armar la cadena de Go.
func leeCadena(b []byte, desde, hasta int, esUTF8 bool) (string, int, error) {
	if desde >= hasta || desde >= len(b) {
		return "", desde, fmt.Errorf("string offset %d is outside [%d,%d)", desde, 0, hasta)
	}
	if esUTF8 {
		p := desde
		unidades, p, err := prefijo(b, p, hasta) // la cuenta utf-16 que declara el formato
		if err != nil {
			return "", p, err
		}
		n, p, err := prefijo(b, p, hasta) // los bytes CESU-8, que son los que se copian
		if err != nil {
			return "", p, err
		}
		if p+n > hasta {
			return "", p, fmt.Errorf("a %d byte string at %d runs past the pool", n, desde)
		}
		if p+n >= hasta {
			return "", p, fmt.Errorf("the string at %d has no terminator", desde)
		}
		if b[p+n] != 0 {
			return "", p, fmt.Errorf("the string at %d does not end with a zero byte", desde)
		}
		decodificadas, err := decodificaCESU8(b[p:p+n], p)
		if err != nil {
			return "", p, err
		}
		if len(decodificadas) != unidades {
			return "", p, fmt.Errorf("the string at %d claims %d utf-16 units and its %d bytes hold %d", desde, unidades, n, len(decodificadas))
		}
		return string(utf16.Decode(decodificadas)), p + n + 1, nil
	}
	n, p, err := prefijoU16(b, desde, hasta)
	if err != nil {
		return "", p, err
	}
	if p+2*n+2 > hasta {
		return "", p, fmt.Errorf("%d utf-16 units at %d run past the pool", n, desde)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteRune(rune(u16re(b, p+2*i)))
	}
	if u16re(b, p+2*n) != 0 {
		return "", p, fmt.Errorf("the utf-16 string at %d has no zero terminator", desde)
	}
	return sb.String(), p + 2*n + 2, nil
}

// decodificaCESU8 pasa los bytes del pool a unidades utf-16. Acepta la forma de aapt2 —cada
// unidad, también las de un par sustituto, en uno a tres bytes— y además el UTF-8 de cuatro bytes,
// que no escribe aapt2 pero sí otros emisores. Un byte que no encabeza ninguna secuencia válida es
// un error del fichero, no texto. Los offsets del mensaje son los del fichero (base).
func decodificaCESU8(b []byte, base int) ([]uint16, error) {
	unidades := make([]uint16, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c < 0x80:
			unidades = append(unidades, uint16(c))
			i++
		case c&0xe0 == 0xc0:
			if i+1 >= len(b) || b[i+1]&0xc0 != 0x80 {
				return nil, fmt.Errorf("a two-byte sequence at %d is cut short", base+i)
			}
			unidades = append(unidades, uint16(c&0x1f)<<6|uint16(b[i+1]&0x3f))
			i += 2
		case c&0xf0 == 0xe0:
			if i+2 >= len(b) || b[i+1]&0xc0 != 0x80 || b[i+2]&0xc0 != 0x80 {
				return nil, fmt.Errorf("a three-byte sequence at %d is cut short", base+i)
			}
			unidades = append(unidades,
				uint16(c&0x0f)<<12|uint16(b[i+1]&0x3f)<<6|uint16(b[i+2]&0x3f))
			i += 3
		case c&0xf8 == 0xf0:
			if i+3 >= len(b) || b[i+1]&0xc0 != 0x80 || b[i+2]&0xc0 != 0x80 || b[i+3]&0xc0 != 0x80 {
				return nil, fmt.Errorf("a four-byte sequence at %d is cut short", base+i)
			}
			r := rune(c&0x07)<<18 | rune(b[i+1]&0x3f)<<12 | rune(b[i+2]&0x3f)<<6 | rune(b[i+3]&0x3f)
			unidades = append(unidades, utf16.Encode([]rune{r})...)
			i += 4
		default:
			return nil, fmt.Errorf("the byte %#02x at %d does not start a UTF-8 sequence", c, base+i)
		}
	}
	return unidades, nil
}

func prefijo(b []byte, at, hasta int) (int, int, error) {
	if at >= hasta {
		return 0, at, fmt.Errorf("no length byte at %d", at)
	}
	n := int(b[at])
	at++
	if n&0x80 != 0 {
		if at >= hasta {
			return 0, at, fmt.Errorf("a two-byte length at %d is cut short", at-1)
		}
		n = (n&0x7f)<<8 | int(b[at])
		at++
	}
	return n, at, nil
}

func prefijoU16(b []byte, at, hasta int) (int, int, error) {
	if at+2 > hasta {
		return 0, at, fmt.Errorf("no length unit at %d", at)
	}
	n := int(u16re(b, at))
	at += 2
	if n&0x8000 != 0 {
		if at+2 > hasta {
			return 0, at, fmt.Errorf("a two-unit length at %d is cut short", at-2)
		}
		n = (n&0x7fff)<<16 | int(u16re(b, at))
		at += 2
	}
	return n, at, nil
}

// valor es un Res_value: tamano, reserva, tipo de dato y datos. Los tipos que se midieron en el
// manifiesto son 0x03 (cadena: los datos son un indice de la pool) y 0x10 (entero decimal).
type valor struct {
	tam   int
	res0  byte
	tipo  byte
	datos uint32
}

type atributo struct {
	ns     int32
	nombre int32
	raw    int32
	valor  valor
}

// nodo es un trozo de XML con nombre: inicio o fin de espacio de nombres, inicio o fin de etiqueta.
// Los cuatro comparten el nodo de 16 bytes (cabecera + linea + comentario) y despues ns y nombre.
type nodoAXML struct {
	tipo   uint16
	linea  int
	ns     int32
	nombre int32
	attrs  []atributo
}

// axml es el fichero binario XML: la pool, el mapa de ids del framework y los nodos en orden.
type axml struct {
	tabla   tablaCadenas
	idsMapa []uint32
	nodos   []nodoAXML
	tamaño  int
}

func leeAXML(b []byte) (*axml, error) {
	if len(b) < 8 {
		return nil, fmt.Errorf("a binary XML needs at least 8 bytes, this has %d", len(b))
	}
	if u16re(b, 0) != trozoXML {
		return nil, fmt.Errorf("not a binary XML: first chunk is %#04x", u16re(b, 0))
	}
	size := int(u32re(b, 4))
	if size != len(b) {
		return nil, fmt.Errorf("the XML header declares %d bytes and the file has %d", size, len(b))
	}
	a := &axml{tamaño: size}
	off := 8
	for off < size {
		tipo := u16re(b, off)
		hs := int(u16re(b, off+2))
		s := int(u32re(b, off+4))
		if s <= 0 || off+s > size {
			return a, fmt.Errorf("chunk %#04x at %d claims %d bytes and the file ends at %d", tipo, off, s, size)
		}
		switch tipo {
		case trozoPool:
			p, err := leePool(b, off)
			if err != nil {
				return a, err
			}
			a.tabla = p
		case trozoMapaRec:
			for o := off + hs; o+4 <= off+s; o += 4 {
				a.idsMapa = append(a.idsMapa, u32re(b, o))
			}
		case trozoInicioNS, trozoFinNS, trozoInicioEtiqueta, trozoFinEtik:
			n, err := a.leeNodo(b, off, hs, s)
			if err != nil {
				return a, err
			}
			a.nodos = append(a.nodos, n)
		default:
			return a, fmt.Errorf("chunk %#04x at %d is not one of the six measured in aapt2's AXML", tipo, off)
		}
		off += s
	}
	if off != size {
		return a, fmt.Errorf("the chunks stop at %d and the header says %d", off, size)
	}
	return a, nil
}

func (a *axml) leeNodo(b []byte, off, hs, s int) (nodoAXML, error) {
	if hs != 16 || off+24 > s+off {
		return nodoAXML{}, fmt.Errorf("XML node at %d has header size %d, the measured ones are 16", off, hs)
	}
	n := nodoAXML{tipo: u16re(b, off), linea: int(u32re(b, off+8)),
		ns: int32(u32re(b, off+16)), nombre: int32(u32re(b, off+20))}
	if n.nombre >= 0 && int(n.nombre) >= len(a.tabla.cadenas) {
		return n, fmt.Errorf("the node at %d names string %d and the pool has %d", off, n.nombre, len(a.tabla.cadenas))
	}
	if n.tipo != trozoInicioEtiqueta {
		if s != 24 {
			return n, fmt.Errorf("a node without attributes at %d occupies %d bytes, measured 24", off, s)
		}
		return n, nil
	}
	attrStart := int(u16re(b, off+24))
	attrSize := int(u16re(b, off+26))
	nAttr := int(u16re(b, off+28))
	if attrSize != 20 {
		return n, fmt.Errorf("attributes of %d bytes at %d: the measured size is 20", attrSize, off)
	}
	if off+16+attrStart+nAttr*20 != off+s {
		return n, fmt.Errorf("node at %d: header 16 + attrExt %d + %d attrs x 20 leaves %d bytes unaccounted",
			off, attrStart, nAttr, s-(16+attrStart+nAttr*20))
	}
	for i := 0; i < nAttr; i++ {
		o := off + 16 + attrStart + i*20
		attrs := atributo{
			ns:     int32(u32re(b, o)),
			nombre: int32(u32re(b, o+4)),
			raw:    int32(u32re(b, o+8)),
			valor: valor{tam: int(u16re(b, o+12)), res0: b[o+14], tipo: b[o+15],
				datos: u32re(b, o+16)},
		}
		n.attrs = append(n.attrs, attrs)
	}
	return n, nil
}

// nombreDe da la cadena del pool con ese indice, o un nombre legible para los mensajes y para el
// diff con `aapt2 dump`, que imprime nombres y no numeros.
func (a *axml) nombreDe(i int32) string {
	if i < 0 || int(i) >= len(a.tabla.cadenas) {
		return fmt.Sprintf("ninguna(%d)", i)
	}
	return a.tabla.cadenas[i]
}

// entrada es un ResTable_entry: tamano, flags (publica / compleja), la clave y su valor. Un valor
// complejo (los estilos) se guarda crudo: su interpretacion no esta medida todavia.
type entradaRecurso struct {
	tam       int
	flags     uint16
	compleja  bool
	clave     int32
	padre     uint32
	recuentos uint32
	valor     valor
	crudo     []byte
	// ausente es el offset 0xffffffff: la clave existe en el tipo pero esta configuracion no la
	// declara (medido en `values-es`, parrafo 94). La entrada se guarda en su sitio para que el
	// indice siga siendo el numero de entrada del id.
	ausente bool
}

// bloqueTipo es un ResTable_type: el id, su nombre, la configuracion (guardada como 64 bytes sin
// interpretar, medidos en parrafo 24) y las entradas, con las offsets relativas a entriesStart.
type bloqueTipo struct {
	id       byte
	nombre   string
	config   []byte
	entradas []entradaRecurso
	tamaño   int
}

// tablaRecursos es el resources.arsc leido: pool global, paquete, sus dos tablas de nombres y los
// bloques de tipo en el orden en que aparecen en el fichero.
type tablaRecursos struct {
	global           tablaCadenas
	idPaquete        uint32
	nombrePaquete    string
	nombresTipo      []string
	claves           []string
	bloques          []bloqueTipo
	especificaciones int
	tamaño           int
}

func leeTabla(b []byte) (*tablaRecursos, error) {
	// el tamano se comprueba antes de leer: el mensaje leia u16re sobre un fichero corto, y eso es un
	// panic en lugar de un rechazo
	if len(b) < 12 {
		return nil, fmt.Errorf("a resource table needs at least 12 bytes, this has %d", len(b))
	}
	if u16re(b, 0) != trozoTabla {
		return nil, fmt.Errorf("not a resource table: first chunk is %#04x", u16re(b, 0))
	}
	size := int(u32re(b, 4))
	if size != len(b) {
		return nil, fmt.Errorf("the table declares %d bytes and the file has %d", size, len(b))
	}
	if n := int(u32re(b, 8)); n != 1 {
		return nil, fmt.Errorf("packageCount is %d; only the single-package form aapt2 writes here is read", n)
	}
	tr := &tablaRecursos{tamaño: size}
	off := 12
	p, err := leePool(b, off)
	if err != nil {
		return tr, err
	}
	tr.global = p
	off += p.tamaño
	for off+8 <= size {
		tipo := u16re(b, off)
		hs := int(u16re(b, off+2))
		s := int(u32re(b, off+4))
		if s <= 0 || off+s > size {
			return tr, fmt.Errorf("chunk %#04x at %d claims %d bytes, the file ends at %d", tipo, off, s, size)
		}
		switch tipo {
		case trozoPaquete:
			if err := tr.leePaquete(b, off, hs, s); err != nil {
				return tr, err
			}
			// el paquete es un contenedor: dentro llevan sus dos tablas de nombres, las
			// especificaciones de tipo y los bloques de entradas. No recorrerlos dejaba la tabla
			// leida con cero tipos, que es peor que un error: parece un fichero vacio valido.
			if err := tr.leeInterior(b, off+hs, off+s); err != nil {
				return tr, err
			}
		default:
			return tr, fmt.Errorf("unexpected chunk %#04x at top level, at %d", tipo, off)
		}
		off += s
	}
	if off != size {
		return tr, fmt.Errorf("the chunks stop at %d and the table says %d", off, size)
	}
	return tr, nil
}

// leeInterior anda los trozos que hay dentro del paquete.
func (tr *tablaRecursos) leeInterior(b []byte, desde, hasta int) error {
	off := desde
	for off+8 <= hasta {
		tipo := u16re(b, off)
		hs := int(u16re(b, off+2))
		s := int(u32re(b, off+4))
		if s <= 0 || off+s > hasta {
			return fmt.Errorf("chunk %#04x at %d claims %d bytes and the package ends at %d", tipo, off, s, hasta)
		}
		switch tipo {
		case trozoPool: // los dos pools de nombres ya los leyo leePaquete por su offset
		case trozoSpec:
			tr.especificaciones++
		case trozoTipo:
			bl, err := tr.leeBloque(b, off, hs, s)
			if err != nil {
				return err
			}
			tr.bloques = append(tr.bloques, bl)
		default:
			return fmt.Errorf("chunk %#04x at %d is not one of the four measured inside a package", tipo, off)
		}
		off += s
	}
	if off != hasta {
		return fmt.Errorf("the package's inner chunks stop at %d and its header says %d", off, hasta)
	}
	return nil
}

func (tr *tablaRecursos) leePaquete(b []byte, off, hs, s int) error {
	if hs != 288 {
		return fmt.Errorf("package header is %d bytes, the measured one is 288", hs)
	}
	tr.idPaquete = u32re(b, off+8)
	var sb strings.Builder
	for i := 0; i < 128; i++ {
		c := u16re(b, off+12+2*i)
		if c == 0 {
			break
		}
		sb.WriteRune(rune(c))
	}
	tr.nombrePaquete = sb.String()
	pT, err := leePool(b, off+int(u32re(b, off+268)))
	if err != nil {
		return fmt.Errorf("type names: %w", err)
	}
	pK, err := leePool(b, off+int(u32re(b, off+276)))
	if err != nil {
		return fmt.Errorf("entry keys: %w", err)
	}
	tr.nombresTipo, tr.claves = pT.cadenas, pK.cadenas
	return nil
}

func (tr *tablaRecursos) leeBloque(b []byte, off, hs, s int) (bloqueTipo, error) {
	n := int(u32re(b, off+12))
	entriesStart := int(u32re(b, off+16))
	bl := bloqueTipo{id: b[off+8], tamaño: s}
	if int(bl.id)-1 >= len(tr.nombresTipo) {
		return bl, fmt.Errorf("type id %d has no name among %d", bl.id, len(tr.nombresTipo))
	}
	bl.nombre = tr.nombresTipo[int(bl.id)-1]
	if hs < 20 {
		return bl, fmt.Errorf("type header of %d bytes cannot hold the measured fields", hs)
	}
	bl.config = b[off+20 : off+hs]
	for i := 0; i < n; i++ {
		o := off + hs + i*4
		if o+4 > off+entriesStart {
			return bl, fmt.Errorf("entry offset #%d at %d is past the offsets table", i, o)
		}
		desplazamiento := u32re(b, o)
		if desplazamiento == 0xffffffff {
			// la clave no existe en esta configuracion; el lector guarda el hueco para que el
			// indice de la entrada siga siendo el que compone el id de recurso
			bl.entradas = append(bl.entradas, entradaRecurso{ausente: true})
			continue
		}
		e := off + entriesStart + int(desplazamiento)
		if e+8 > off+s {
			return bl, fmt.Errorf("entry #%d at %d runs past its type block", i, e)
		}
		en := entradaRecurso{
			tam:   int(u16re(b, e)),
			flags: u16re(b, e+2),
			clave: int32(u32re(b, e+4)),
		}
		// Medido en el fichero de aapt2: una entrada con valor simple declara 8 bytes y los flags
		// 0x00 o 0x04; la del estilo declara 16 y flags 0x01. Que ese bit signifique "compleja" o
		// "publica" no esta confirmado: se guarda la bandera cruda y la lectura de esos bits se
		// medira al escribir (parrafos 26 y 27), no se supone del header de aapt2.
		en.compleja = en.tam == 16 && en.flags&0x0001 != 0
		if int(en.clave) >= len(tr.claves) {
			return bl, fmt.Errorf("entry #%d names key %d and there are %d keys", i, en.clave, len(tr.claves))
		}
		v := e + en.tam
		if en.compleja {
			// Forma medida sobre el estilo del studio (parrafo 26, corregido hoy): la entrada
			// compleja declara 16 bytes - tam, banderas, clave, el id del padre y EL RECUENTO DE
			// REGISTROS - y detras vienen exactamente `recuentos` registros de doce bytes (nombre
			// u32 + Res_value de ocho: tam 8, reserva, tipo, datos). Las dos configuraciones de
			// AppTheme dicen 4 y 5 y traen 4 y 5. La lectura que hacia decir "cuatro en ambas"
			// tomaba el campo una palabra mas arriba, donde en realidad esta el padre.
			if en.tam != 16 || e+16 > off+s {
				return bl, fmt.Errorf("entry #%d: a complex entry declares %d bytes, the measured header is 16", i, en.tam)
			}
			en.padre = u32re(b, e+8)
			en.recuentos = u32re(b, e+12)
			en.crudo = b[e+16 : off+s]
			if len(en.crudo)%12 != 0 {
				return bl, fmt.Errorf("entry #%d: %d bytes of records is not a whole number of twelve", i, len(en.crudo))
			}
			if uint32(len(en.crudo)/12) != en.recuentos {
				return bl, fmt.Errorf("entry #%d declares %d records and the block holds %d", i, en.recuentos, len(en.crudo)/12)
			}
			for k := 0; k < len(en.crudo); k += 12 {
				if nombre := u32re(en.crudo, k); nombre&0xff000000 != 0x01000000 {
					return bl, fmt.Errorf("entry #%d record at +%d names %#08x, which is not a resource id", i, k, nombre)
				}
				if u16re(en.crudo, k+4) != 8 {
					return bl, fmt.Errorf("entry #%d record at +%d: a value of %d bytes, measured 8", i, k, u16re(en.crudo, k+4))
				}
			}
			bl.entradas = append(bl.entradas, en)
			continue
		}
		if en.tam != 8 || v+8 > off+s {
			return bl, fmt.Errorf("entry #%d declares %d bytes: a plain value follows an 8-byte header", i, en.tam)
		}
		en.valor = valor{tam: int(u16re(b, v)), res0: b[v+2], tipo: b[v+3], datos: u32re(b, v+4)}
		bl.entradas = append(bl.entradas, en)
	}
	return bl, nil
}

// clave de la entrada, para los mensajes y para la R.
func (tr *tablaRecursos) clave(e entradaRecurso) string {
	if int(e.clave) >= len(tr.claves) {
		return fmt.Sprintf("ninguna(%d)", e.clave)
	}
	return tr.claves[int(e.clave)]
}

// bloqueDeTipo busca el bloque del tipo dado; el de la configuración base manda, y si no hay base
// se queda con el primero que aparezca. Es la consulta que necesita la vista previa: un layout se
// resuelve contra lo que un teléfono corriente cargaría.
func (tr *tablaRecursos) bloqueDeTipo(tipo uint32) *bloqueTipo {
	var primero *bloqueTipo
	for i := range tr.bloques {
		bl := &tr.bloques[i]
		if uint32(bl.id) != tipo {
			continue
		}
		if esConfiguracionBase(bl.config) {
			return bl
		}
		if primero == nil {
			primero = bl
		}
	}
	return primero
}

// entradaDeId resuelve el id de recurso (0xppTTHHHH) a su entrada en la configuración base: el
// paquete tiene que ser el de esta tabla, el tipo el que ocupa el byte de en medio y la entrada el
// índice bajo. Es la forma que un layout escribe en `android:id` para `@+id/total`.
func (tr *tablaRecursos) entradaDeId(id uint32) (bloqueTipo, entradaRecurso, bool) {
	if id>>24 != tr.idPaquete&0xff {
		return bloqueTipo{}, entradaRecurso{}, false
	}
	bl := tr.bloqueDeTipo((id >> 16) & 0xff)
	if bl == nil {
		return bloqueTipo{}, entradaRecurso{}, false
	}
	indice := int(id & 0xffff)
	if indice >= len(bl.entradas) {
		return bloqueTipo{}, entradaRecurso{}, false
	}
	return *bl, bl.entradas[indice], true
}

// nombreDeId devuelve el tipo y la clave del recurso, que es como se nombra una referencia en el
// modelo (`id/total`, `string/hola`).
func (tr *tablaRecursos) nombreDeId(id uint32) (tipo, clave string, ok bool) {
	bl, e, ok := tr.entradaDeId(id)
	if !ok || e.ausente {
		return "", "", false
	}
	return bl.nombre, tr.clave(e), true
}

// valorDeId da el Res_value de una entrada simple. Un valor complejo (un estilo) no tiene uno y se
// rechaza: su nombre lo da nombreDeId.
func (tr *tablaRecursos) valorDeId(id uint32) (valor, bool) {
	_, e, ok := tr.entradaDeId(id)
	if !ok || e.ausente || e.compleja {
		return valor{}, false
	}
	return e.valor, true
}
