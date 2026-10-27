package main

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf16"
)

// TestPutUlebUsesTheNumbersTheFormatDocumenta pins the encoders to the examples that appear in
// the format itself, not to what this file thinks they should be.
func TestPutUlebAndSlebAnswerLikeTheFormat(t *testing.T) {
	casos := []struct {
		v    uint32
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{63, []byte{0x3f}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{16384, []byte{0x80, 0x80, 0x01}},
		{32768, []byte{0x80, 0x80, 0x02}},
		{624485, []byte{0xE5, 0x8E, 0x26}}, // the example the spec uses
	}
	for _, c := range casos {
		if got := putUleb(nil, c.v); !bytes.Equal(got, c.want) {
			t.Errorf("uleb %d: % x, want % x", c.v, got, c.want)
		}
	}

	signed := []struct {
		v    int32
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{-1, []byte{0x7f}},
		{-2, []byte{0x7e}},
		{63, []byte{0x3f}},
		{64, []byte{0xc0, 0x00}},
		{-64, []byte{0x40}},
		{-65, []byte{0xBF, 0x7F}},
		{-128, []byte{0x80, 0x7f}},
		{128, []byte{0x80, 0x01}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0x78}},
	}
	for _, c := range signed {
		if got := putSleb(nil, c.v); !bytes.Equal(got, c.want) {
			t.Errorf("sleb %d: % x, want % x", c.v, got, c.want)
		}
	}
}

// TestSlebReadsBackAsUlebIsNotThePoint, but it does have to read back: the reader's uleb is the
// only decoder here, and a signed value that survives it is a value that will survive a verifier.
func TestSlebRoundTripsThroughASignedReader(t *testing.T) {
	for _, v := range []int32{0, 1, -1, 63, -64, 127, -128, 8191, -8192, 1 << 30, -(1 << 30)} {
		b := putSleb(nil, v)
		got, ok := readSleb(b)
		if !ok {
			t.Errorf("sleb %d could not be read back from % x", v, b)
			continue
		}
		if got != v {
			t.Errorf("sleb round trip: %d became %d", v, got)
		}
	}
}

func TestEncodeMUTF8IsNotUTF8InItsTwoSpecialCases(t *testing.T) {
	casos := []struct {
		nombre  string
		dado    string
		querido []byte
	}{
		{"ascii", "Lcom/x/Y;", []byte("Lcom/x/Y;")},
		// U+0000 costs two bytes and is never one: the terminator stays a terminator
		{"nul", "a\x00b", []byte{0x61, 0xC0, 0x80, 0x62}},
		{"dos bytes", "é", []byte{0xC3, 0xA9}},
		{"tres bytes", "€", []byte{0xE2, 0x82, 0xAC}},
		// outside the plane: two three-byte surrogates, not one four-byte sequence
		{"astral", "\U0001F600", []byte{0xED, 0xA0, 0xBD, 0xED, 0xB8, 0x80}},
	}
	for _, c := range casos {
		got := encodeMUTF8(c.dado)
		if !bytes.Equal(got, c.querido) {
			t.Errorf("%s: encoded % x, want % x", c.nombre, got, c.querido)
		}
		n, err := checkMUTF8(c.dado)
		if err != nil {
			t.Errorf("%s: %v", c.nombre, err)
			continue
		}
		if want := len(utf16.Encode([]rune(c.dado))); n != want {
			t.Errorf("%s: claimed %d utf-16 units, the string has %d", c.nombre, n, want)
		}
	}
}

// TestMUTF8OrderIsNotByteOrder is the reason the comparison is written: a pair of surrogates sorts
// by code unit, so a string with an astral character early on can land somewhere its Go ordering
// would never put it.
func TestMUTF8OrderIsNotByteOrder(t *testing.T) {
	// aqui si discrepan: un caracter del plano 1 se codifica como sustitutos D800-DFFF, que van
	// DELANTE de los U+E000..U+FFFF del plano multilingual, mientras que a bytes de UTF-8 el
	// orden de puntos de codigo pone U+E000 primero. Comparar strings de Go seria, por tanto,
	// un orden distinto del que exige el formato.
	astral := "\U0001F600"
	bmpAlto := "\uE000"
	if mutf8OrderLess(astral, bmpAlto) != true {
		t.Errorf("an astral character must sort before U+E000 by code unit, since its first surrogate is D83D")
	}
	if astral < bmpAlto {
		t.Error("Go's own byte order was expected to disagree here; if it stops disagreeing, this test is no longer proving anything")
	}
	if mutf8OrderLess("I", "LL") != true {
		t.Error(`"I" must sort before "LL": that is the order the fixture shows (dex string_ids #4 then #5)`)
	}
	if mutf8OrderLess("<init>", "AppTheme") != true {
		t.Error(`"<init>" sorts before "AppTheme" by code unit: '<' is 0x3c, 'A' is 0x41, and the fixture puts them in exactly that order`)
	}

	cadenas := []string{"z", "a", "~~D8", "<init>", "I", "LL", "LLLI", "AppTheme"}
	ordenado := append([]string{}, cadenas...)
	sortByMUTF8(ordenado)
	for i := 1; i < len(ordenado); i++ {
		if mutf8OrderLess(ordenado[i], ordenado[i-1]) {
			t.Errorf("sorted output is out of order at %d: %q before %q", i, ordenado[i], ordenado[i-1])
		}
	}
	if got := strings.Join(ordenado, "|"); got != "<init>|AppTheme|I|LL|LLLI|a|z|~~D8" {
		t.Errorf("sorted order came out %v, and the code-unit order asks for the acentos minusculas (0x61..) after las mayusculas", ordenado)
	}
}

func TestAlignUpOnlyMovesWhenItHasTo(t *testing.T) {
	for _, c := range []struct {
		at, size, want int
	}{
		{0, 4, 0}, {1, 4, 4}, {4, 4, 4}, {5, 4, 8}, {7, 8, 8}, {8, 8, 8}, {9, 8, 16},
		{1258, 2, 1258}, {1259, 2, 1260},
	} {
		if got := alignUp(c.at, c.size); got != c.want {
			t.Errorf("alignUp(%d, %d) = %d, want %d", c.at, c.size, got, c.want)
		}
	}
	b := putByteAlign([]byte{1, 2, 3}, 4)
	if len(b) != 4 || b[3] != 0 {
		t.Errorf("padding wrote % x: three bytes plus one zero, and the zero is the whole point", b)
	}
}

// readSleb is the signed counterpart of the reader's uleb, kept here for the round trip above: the
// writer must not be tested only by what it emits.
func readSleb(b []byte) (int32, bool) {
	var result int32
	var shift uint
	for i, c := range b {
		result |= int32(c&0x7f) << shift
		shift += 7
		if c&0x80 == 0 {
			if shift < 32 && c&0x40 != 0 {
				result |= -1 << shift // sign extend
			}
			return result, true
		}
		_ = i
	}
	return 0, false
}

// TestWriteStringsReproducesD8sStringArea byte for byte: the 53 strings of the fixture, fed in
// reverse so the sorting has to do the work, must come back as the bytes d8 wrote. This is the
// first place where the writer is measured against the tool it replaces rather than against
// itself, and it checks two things at once: the code-unit order and the uleb128 counts.
func TestWriteStringsReproducesD8sStringArea(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}

	alreves := append([]string{}, d.strings...)
	for i, j := 0, len(alreves)-1; i < j; i, j = i+1, j-1 {
		alreves[i], alreves[j] = alreves[j], alreves[i]
	}

	producido := writeStrings(alreves)

	esperado := []byte{}
	for i := range d.strings {
		item := d.stringItemBytes(i)
		if len(item) == 0 {
			t.Fatalf("string #%d has no bytes to compare against", i)
		}
		esperado = append(esperado, item...)
	}

	if !bytes.Equal(producido, esperado) {
		t.Errorf("the string area differs from d8's: %d bytes written against %d", len(producido), len(esperado))
		// donde empieza a diferir, porque un offset de mas o de menos lo desplaza todo
		for i := 0; i < len(producido) && i < len(esperado); i++ {
			if producido[i] != esperado[i] {
				fino := i
				for fino > 0 && producido[fino-1] != esperado[fino-1] {
					fino--
				}
				t.Errorf("first difference at byte %d: % x against % x", fino,
					producido[fino:intMin(fino+16, len(producido))], esperado[fino:intMin(fino+16, len(esperado))])
				break
			}
		}
	}

	// la concatenacion solo equivale al area si los items vienen detras unos de otros: si d8
	// hubiera dejado padding entre cadenas, comparar bytes sueltos no diria nada. Cada offset
	// tiene que ser el final del item anterior, y eso es lo que se comprueba aqui.
	base := dexLittleEndian.Uint32(d.raw[d.tableOff["string_ids"][1]:])
	anterior := base
	for i := range d.strings {
		off := dexLittleEndian.Uint32(d.raw[d.tableOff["string_ids"][1]+uint32(i)*4:])
		if off != anterior {
			t.Errorf("string #%d starts at %d and the previous item ended at %d: the area is not contiguous", i, off, anterior)
			break
		}
		anterior = off + uint32(len(d.stringItemBytes(i)))
	}
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
