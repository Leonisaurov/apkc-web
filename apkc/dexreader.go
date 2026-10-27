package main

// Reading a dex file, which is what makes writing one checkable.
//
// The writer phase of this fork (the plan calls it 2G) has to produce a classes.dex that the
// platform accepts. Comparing our output against d8's byte for byte is only half the check: when
// it differs, the answer has to be *which section* differs, and that needs a reader. So the
// reader comes first, and it is also the thing that lets the counts in docs/evidencia.md (§13)
// become assertions in a test instead of a paragraph.
//
// Only what the writer needs is decoded here: the header, the map, the five index tables and the
// class definitions. Code and annotations stay as offsets — reading them is a separate step, and
// pretending to understand them before emitting anything would be a guess with a struct around it.

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/adler32"
	"sort"
)

const (
	dexHeaderSize    = 0x70
	dexEndianTag     = uint32(0x12345678)
	dexMapEntrySize  = 12 // ushort kind, ushort unused, uint size, uint offset — NOT 16
	dexStringIdsOff  = 56 // the seven pairs start here: size, offset
	dexFieldIDsCount = 4
	dexMethodIDSize  = 8
	dexFieldIDSize   = 8
	dexTypeSize      = 4
	dexProtoSize     = 12
	dexClassDefSize  = 32
	dexMagicPrefix   = "dex\n"
	dexChecksumAt    = 8  // adler32 over everything after it
	dexSignatureAt   = 12 // sha-1 over everything after it
	dexFileSizeAt    = 32
	dexHeaderSizeAt  = 36
	dexEndianAt      = 40
	dexMapOffAt      = 52
)

// dexLittleEndian is the only form the format allows a dex to be written in, and the endian tag
// in the header is checked against it rather than assumed.
var dexLittleEndian = binary.LittleEndian

// dexMapKind names the section types the map_list reports, so an error can say "call_site_ids"
// instead of "0x7". Values are from the format, not chosen: the 0x20xx band is the unnumbered
// data items and 0x1001 the type lists.
// dexKind is the section type a map_list entry carries. Named, because Go does not let a method
// live on uint16 and because "0x2003" in an error message tells nobody what "class_data_item" says.
type dexKind uint16

var dexMapKind = map[dexKind]string{
	0x0000: "header", 0x0001: "string_ids", 0x0002: "type_ids", 0x0003: "proto_ids",
	0x0004: "field_ids", 0x0005: "method_ids", 0x0006: "class_defs", 0x0007: "call_site_ids",
	0x0008: "method_handle_ids", 0x1000: "map", 0x1001: "type_list", 0x1002: "annot_set_list",
	0x1003: "class_annotation_set_item",
	// la banda 0x20xx: estos valores los tuve mal una revisión entera (0x2000 es class_data y no
	// debug_info, 0x2003 es debug_info y no class_data), y un mapa mal etiquetado hace que
	// cualquier conclusion sobre donde esta cada cosa salga con sentido pero falsa
	0x2000: "class_data_item", 0x2001: "code_item", 0x2002: "string_data_item",
	0x2003: "debug_info_item", 0x2004: "annotation_item", 0x2005: "encoded_array_item",
	0x2006: "annotations_directory_item",
}

func (k dexKind) String() string {
	if n, ok := dexMapKind[k]; ok {
		return n
	}
	return fmt.Sprintf("unknown section %#04x", uint16(k))
}

type dexMapEntry struct {
	kind   dexKind
	size   uint32
	offset uint32
}

type dexProto struct {
	shorty     string
	returnType string
	params     []string
}

type dexField struct {
	owner  string
	typeOf string
	name   string
}

type dexMethod struct {
	owner string
	proto dexProto
	name  string
}

type dexClass struct {
	name     string
	super    string
	access   uint32
	directs  int
	virtuals int

	// where this class's own data starts, so the reader of class_data_item can be pointed at
	// it without the caller recomputing the offset of the entry
	dataOff uint32

	// the encoded_array_item with the constants of the static fields. Measured on d8's file:
	// the R constants live HERE (2465, 2471, 2477), not inline in the encoded_field
	staticValuesOff uint32
}

// dexFile is a parsed classes.dex: the index tables resolved to strings, which is the form a
// comparison needs. The raw bytes stay reachable because a byte-level difference has to be
// located in a section, and that means the offsets of the original.
type dexFile struct {
	raw     []byte
	version string
	strings []string
	types   []string
	protos  []dexProto
	fields  []dexField
	methods []dexMethod
	classes []dexClass
	maps    []dexMapEntry

	// where each table sits in the file, for reporting a difference by section
	tableOff map[string][2]uint32
}

func readDex(b []byte) (*dexFile, error) {
	if len(b) < dexHeaderSize {
		return nil, fmt.Errorf("a dex file starts with a %X byte header, and this is %d bytes", dexHeaderSize, len(b))
	}
	if string(b[0:4]) != dexMagicPrefix {
		return nil, fmt.Errorf("magic %q: not a classes.dex", b[0:4])
	}
	size := dexLittleEndian.Uint32(b[dexFileSizeAt:])
	if int(size) != len(b) {
		return nil, fmt.Errorf("the header claims %d bytes and the file holds %d", size, len(b))
	}
	if h := dexLittleEndian.Uint32(b[dexHeaderSizeAt:]); h != dexHeaderSize {
		return nil, fmt.Errorf("header_size is %#x, expected %#x", h, dexHeaderSize)
	}
	if e := dexLittleEndian.Uint32(b[dexEndianAt:]); e != dexEndianTag {
		return nil, fmt.Errorf("endian_tag %#x: this fork writes and reads little endian only", e)
	}

	d := &dexFile{raw: b, version: string(b[4:7]), tableOff: map[string][2]uint32{}}
	for i, name := range []string{"string_ids", "type_ids", "proto_ids", "field_ids", "method_ids", "class_defs", "data"} {
		n := dexLittleEndian.Uint32(b[dexStringIdsOff+i*8:])
		o := dexLittleEndian.Uint32(b[dexStringIdsOff+i*8+4:])
		d.tableOff[name] = [2]uint32{n, o}
	}

	if err := d.readInts(); err != nil {
		return nil, err
	}
	if err := d.readStringData(); err != nil {
		return nil, err
	}
	if err := d.resolveTypes(); err != nil {
		return nil, err
	}
	if err := d.resolveMembers(); err != nil {
		return nil, err
	}
	if err := d.resolveClasses(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *dexFile) readInts() error {
	mapOff := dexLittleEndian.Uint32(d.raw[dexMapOffAt:])
	if int(mapOff)+4 > len(d.raw) {
		return fmt.Errorf("map_list offset %d is past the end of the file", mapOff)
	}
	n := dexLittleEndian.Uint32(d.raw[mapOff:])
	if uint64(len(d.raw)) < uint64(mapOff)+4+uint64(n)*dexMapEntrySize {
		return fmt.Errorf("the map_list claims %d entries and the file has no room for them", n)
	}
	o := mapOff + 4
	for i := uint32(0); i < n; i++ {
		kind := dexKind(binary.LittleEndian.Uint16(d.raw[o:]))
		size := dexLittleEndian.Uint32(d.raw[o+4:])
		off := dexLittleEndian.Uint32(d.raw[o+8:])
		if uint64(off)+uint64(size)*uint64(dexItemStride(kind)) > uint64(len(d.raw)) && kind != 0x0000 && kind != 0x1000 {
			return fmt.Errorf("section %s at %d with %d items does not fit the file", kind, off, size)
		}
		d.maps = append(d.maps, dexMapEntry{kind: kind, size: size, offset: off})
		o += dexMapEntrySize
	}
	// el map_list se le tal como viene; ordenarlo por offset es lo que permite decir donde TERMINA
	// un area, y por tanto recortar sus bytes para compararlos con los de otro fichero. Sin esa
	// cuenta, una seccion "hasta el final" se solapa con la siguiente y cualquier diff miente.
	sort.Slice(d.maps, func(i, j int) bool { return d.maps[i].offset < d.maps[j].offset })
	return nil
}

// offDe responde dónde empieza una sección según el map_list, y cero si el mapa no la declara.
func (d *dexFile) offDe(kind dexKind) uint32 {
	for _, e := range d.maps {
		if e.kind == kind {
			return e.offset
		}
	}
	return 0
}

// seccion corta los bytes de un área del fichero: desde el offset que declara el mapa hasta el de
// la siguiente área. Para las secciones de tamaño fijo el propio mapa dice cuántas hay, así que la
// cuenta es exacta sin depender del vecino.
func (d *dexFile) seccion(kind dexKind) []byte {
	off := d.offDe(kind)
	if off == 0 {
		return nil
	}
	if u := dexItemStride(kind); u > 0 {
		for _, e := range d.maps {
			if e.kind == kind {
				return d.raw[off : off+uint32(e.size)*u]
			}
		}
	}
	for i, e := range d.maps {
		if e.kind != kind {
			continue
		}
		fin := uint32(len(d.raw))
		if i+1 < len(d.maps) {
			fin = d.maps[i+1].offset
		}
		return d.raw[off:fin]
	}
	return nil
}

// dexItemStride is how many bytes one item of a section occupies in the file, for the sanity
// check above. Sections whose stride is not known here (the data items, which are variable
// length) answer 0 and are not checked by stride: their offset still has to be inside the file.
func dexItemStride(kind dexKind) uint32 {
	switch kind {
	case 0x0001, 0x0002: // string_ids y type_ids: un u32 cada uno
		return 4
	case 0x0003:
		return dexProtoSize
	case 0x0004:
		return dexFieldIDSize
	case 0x0005:
		return dexMethodIDSize
	case 0x0006:
		return dexClassDefSize
	default:
		return 0
	}
}

func (d *dexFile) readStringData() error {
	n, off := d.tableOff["string_ids"][0], d.tableOff["string_ids"][1]
	d.strings = make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		at := dexLittleEndian.Uint32(d.raw[off+i*4:])
		utf16Count, body, err := d.uleb(at)
		if err != nil {
			return fmt.Errorf("string #%d: %w", i, err)
		}
		inicio := int(body) // byteIndex answers an int, and mixing the two here is a silent
		// out-of-range panic on a file that only claims to be a dex
		end := byteIndex(d.raw[inicio:], 0)
		if end < 0 {
			return fmt.Errorf("string #%d is not terminated", i)
		}
		s, err := decodeMUTF8(d.raw[inicio : inicio+end])
		if err != nil {
			return fmt.Errorf("string #%d: %w", i, err)
		}
		if uint32(utf16Len(s)) != utf16Count {
			return fmt.Errorf("string #%d claims %d utf-16 units and holds %d", i, utf16Count, utf16Len(s))
		}
		d.strings = append(d.strings, s)
	}
	return nil
}

// stringItemBytes is the whole string_data_item as the file holds it: the uleb128 count, the
// modified utf-8 and the terminator. The writer compares against these bytes, which is the only
// way to know that a count we chose is the count the format asked for.
func (d *dexFile) stringItemBytes(i int) []byte {
	if i < 0 || i >= len(d.strings) {
		return nil
	}
	off := dexLittleEndian.Uint32(d.raw[d.tableOff["string_ids"][1]+uint32(i)*4:])
	_, body, err := d.uleb(off)
	if err != nil {
		return nil
	}
	inicio := int(body)
	end := byteIndex(d.raw[inicio:], 0)
	if end < 0 {
		return nil
	}
	return d.raw[int(off) : inicio+end+1]
}

func (d *dexFile) uleb(at uint32) (uint32, uint32, error) {
	var value uint32
	var shift uint
	o := at
	for {
		if int(o) >= len(d.raw) {
			return 0, 0, errors.New("uleb128 runs past the end of the file")
		}
		b := d.raw[o]
		o++
		value |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, o, nil
		}
		shift += 7
		if shift > 28 {
			return 0, 0, errors.New("uleb128 longer than a uint32")
		}
	}
}

func (d *dexFile) at(section string, i uint32) (uint32, error) {
	pair, ok := d.tableOff[section]
	if !ok {
		return 0, fmt.Errorf("no %s table", section)
	}
	if i >= pair[0] {
		return 0, fmt.Errorf("%s index %d out of range (table holds %d)", section, i, pair[0])
	}
	return dexLittleEndian.Uint32(d.raw[pair[1]+i*4:]), nil
}

func (d *dexFile) stringAt(i uint32) (string, error) {
	if int(i) >= len(d.strings) {
		return "", fmt.Errorf("string index %d out of range (table holds %d)", i, len(d.strings))
	}
	return d.strings[i], nil
}

func (d *dexFile) resolveTypes() error {
	n, off := d.tableOff["type_ids"][0], d.tableOff["type_ids"][1]
	d.types = make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		idx := dexLittleEndian.Uint32(d.raw[off+i*dexTypeSize:])
		s, err := d.stringAt(idx)
		if err != nil {
			return fmt.Errorf("type_ids #%d: %w", i, err)
		}
		d.types = append(d.types, s)
	}

	pn, poff := d.tableOff["proto_ids"][0], d.tableOff["proto_ids"][1]
	d.protos = make([]dexProto, 0, pn)
	for i := uint32(0); i < pn; i++ {
		base := poff + i*dexProtoSize
		shortyIdx := dexLittleEndian.Uint32(d.raw[base:])
		retIdx := dexLittleEndian.Uint32(d.raw[base+4:])
		paramsOff := dexLittleEndian.Uint32(d.raw[base+8:])
		shorty, err := d.stringAt(shortyIdx)
		if err != nil {
			return fmt.Errorf("proto_ids #%d: %w", i, err)
		}
		p := dexProto{shorty: shorty, returnType: d.types[retIdx]}
		if paramsOff != 0 {
			// type_item es un ushort por parametro, packed: avanzar de a 4 leia el
			// doble de lejos y devolvia descriptores que no existian
			count := dexLittleEndian.Uint32(d.raw[paramsOff:])
			for k := uint32(0); k < count; k++ {
				tidx := binary.LittleEndian.Uint16(d.raw[paramsOff+4+k*2:])
				p.params = append(p.params, d.types[tidx])
			}
		}
		d.protos = append(d.protos, p)
	}
	return nil
}

func (d *dexFile) resolveMembers() error {
	fn, foff := d.tableOff["field_ids"][0], d.tableOff["field_ids"][1]
	d.fields = make([]dexField, 0, fn)
	for i := uint32(0); i < fn; i++ {
		base := foff + i*dexFieldIDSize
		cls := binary.LittleEndian.Uint16(d.raw[base:])
		typ := binary.LittleEndian.Uint16(d.raw[base+2:])
		// name_idx es un ushort: los dos bytes que siguen son relleno, y el item mide 8 no 6. Leer
		// el u32 pasaba con d8 porque su relleno es cero; con nuestro propio fichero, donde el
		// relleno lo escribe quien sea, el u32 se llevaba los bytes de delante y el indice salia
		// de rango. Medido: 524 + 4*8 = 556, que es exactamente donde esta method_ids.
		name := uint32(binary.LittleEndian.Uint16(d.raw[base+4:]))
		s, err := d.stringAt(name)
		if err != nil {
			return fmt.Errorf("field_ids #%d: %w", i, err)
		}
		d.fields = append(d.fields, dexField{owner: d.types[cls], typeOf: d.types[typ], name: s})
	}

	mn, moff := d.tableOff["method_ids"][0], d.tableOff["method_ids"][1]
	d.methods = make([]dexMethod, 0, mn)
	for i := uint32(0); i < mn; i++ {
		base := moff + i*dexMethodIDSize
		cls := binary.LittleEndian.Uint16(d.raw[base:])
		proto := binary.LittleEndian.Uint16(d.raw[base+2:])
		name := uint32(binary.LittleEndian.Uint16(d.raw[base+4:])) // igual que field_ids: u16 y dos de relleno
		s, err := d.stringAt(name)
		if err != nil {
			return fmt.Errorf("method_ids #%d: %w", i, err)
		}
		if int(proto) >= len(d.protos) {
			return fmt.Errorf("method_ids #%d points at proto %d and there are %d", i, proto, len(d.protos))
		}
		d.methods = append(d.methods, dexMethod{owner: d.types[cls], proto: d.protos[proto], name: s})
	}
	return nil
}

func (d *dexFile) resolveClasses() error {
	n, off := d.tableOff["class_defs"][0], d.tableOff["class_defs"][1]
	d.classes = make([]dexClass, 0, n)
	for i := uint32(0); i < n; i++ {
		base := off + i*dexClassDefSize
		cidx := dexLittleEndian.Uint32(d.raw[base:])
		access := dexLittleEndian.Uint32(d.raw[base+4:])
		super := dexLittleEndian.Uint32(d.raw[base+8:])
		classDataOff := dexLittleEndian.Uint32(d.raw[base+24:])
		staticValuesOff := dexLittleEndian.Uint32(d.raw[base+28:])
		if int(cidx) >= len(d.types) || int(super) >= len(d.types) {
			return fmt.Errorf("class_defs #%d names type %d or %d out of range", i, cidx, super)
		}
		c := dexClass{name: d.types[cidx], super: d.types[super], access: access, dataOff: classDataOff,
			staticValuesOff: staticValuesOff}
		if classDataOff != 0 {
			// class_data_item: sizes first as uleb128 (static fields, instance fields,
			// direct methods, virtual methods); only the two method counts are needed here
			_, o, err := d.uleb(classDataOff)
			if err != nil {
				return fmt.Errorf("class_defs #%d: %w", i, err)
			}
			_, o, err = d.uleb(o)
			if err != nil {
				return fmt.Errorf("class_defs #%d: %w", i, err)
			}
			directs, o, err := d.uleb(o)
			if err != nil {
				return fmt.Errorf("class_defs #%d: %w", i, err)
			}
			virtuals, _, err := d.uleb(o)
			if err != nil {
				return fmt.Errorf("class_defs #%d: %w", i, err)
			}
			c.directs, c.virtuals = int(directs), int(virtuals)
		}
		d.classes = append(d.classes, c)
	}
	return nil
}

// stringsOf lists the pool entries starting with a prefix: how a test asks "did d8 put its
// version marker in here" without hard-coding an index.
func (d *dexFile) stringsOf(prefix string) []string {
	out := []string{}
	for _, s := range d.strings {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			out = append(out, s)
		}
	}
	return out
}

// Integrity checks the two claims the header makes about its own bytes. A dex whose adler32 or
// sha-1 does not match is a file no installer will accept, and a writer that produces one needs
// to be told by a test rather than by a phone.
func (d *dexFile) Integrity() error {
	claim := dexLittleEndian.Uint32(d.raw[dexChecksumAt:])
	if got := adler32.Checksum(d.raw[dexChecksumAt+4:]); got != claim {
		return fmt.Errorf("adler32 of the file is %08x and the header claims %08x", got, claim)
	}
	want := d.raw[dexSignatureAt : dexSignatureAt+20]
	sum := sha1.Sum(d.raw[dexSignatureAt+20:])
	if !bytesEqual(want, sum[:]) {
		return fmt.Errorf("sha-1 signature of the file is %x and the header claims %x", sum[:], want)
	}
	return nil
}

// decodeMUTF8 turns the string bytes of a dex into Go's UTF-8. The form is not UTF-8: U+0000 is
// written as the two bytes C0 80 (so no terminator can appear inside a string), and a character
// outside the plane is written as the two three-byte surrogates of CESU-8, which are recombined
// here because a dex sorted by utf-16 units and a Go comparison by rune have to agree.
func decodeMUTF8(b []byte) (string, error) {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c&0x80 == 0:
			out = append(out, c)
			i++
		case c&0xe0 == 0xc0:
			if i+1 >= len(b) {
				return "", errors.New("truncated two byte form")
			}
			v := rune(c&0x1f)<<6 | rune(b[i+1]&0x3f)
			// U+0000 exists only as C0 80 — that is the whole trick of the form, and the reason
			// a terminator can never appear inside a string. Everything else in 1..7FF is an
			// ordinary two byte character, so it must be accepted, not treated as overlong.
			if v == 0 && (c != 0xc0 || b[i+1] != 0x80) {
				return "", fmt.Errorf("two byte form at %d is an overlong encoding of U+0000", i)
			}
			out = appendRune(out, v)
			i += 2
		case c&0xf0 == 0xe0:
			if i+2 >= len(b) {
				return "", errors.New("truncated three byte form")
			}
			r := rune(c&0x0f)<<12 | rune(b[i+1]&0x3f)<<6 | rune(b[i+2]&0x3f)
			i += 3
			if r >= 0xd800 && r <= 0xdbff && i+2 < len(b) && b[i]&0xf0 == 0xe0 {
				low := rune(b[i]&0x0f)<<12 | rune(b[i+1]&0x3f)<<6 | rune(b[i+2]&0x3f)
				if low >= 0xdc00 && low <= 0xdfff {
					r = 0x10000 + (r-0xd800)*0x400 + (low - 0xdc00)
					i += 3
				}
			}
			out = appendRune(out, r)
		default:
			return "", fmt.Errorf("byte %#x at %d is not a start of a modified utf-8 sequence", c, i)
		}
	}
	return string(out), nil
}

func appendRune(b []byte, r rune) []byte {
	switch {
	case r < 0x80:
		return append(b, byte(r))
	case r < 0x800:
		return append(b, byte(0xc0|r>>6), byte(0x80|r&0x3f))
	case r < 0x10000:
		return append(b, byte(0xe0|r>>12), byte(0x80|r>>6&0x3f), byte(0x80|r&0x3f))
	default:
		return append(b, byte(0xf0|r>>18), byte(0x80|r>>12&0x3f), byte(0x80|r>>6&0x3f), byte(0x80|r&0x3f))
	}
}

// utf16Len counts what the string_data_item claims: code units, so a supplementary character is
// two and U+0000 is one.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xffff {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func byteIndex(b []byte, v byte) int {
	for i, x := range b {
		if x == v {
			return i
		}
	}
	return -1
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
