package main

// The encoders the dex writer is built from: LEB128, its signed form, and modified UTF-8.
//
// They live apart from the section writers because each one has a rule that is not obvious from
// the surrounding format, and a rule that is not obvious is a rule worth a test of its own:
// LEB128 is little-endian by groups of seven bits with the high bit meaning "more", ULEB128 has
// no sign extension, and the string form of a dex is not UTF-8 — it is the modified one, which
// spends two bytes on U+0000 so a terminator can never appear inside a string, and writes a
// character outside the basic plane as the two three-byte surrogates of CESU-8 instead of the
// four-byte form UTF-8 uses.
//
// The last one is why the ordering of string_ids cannot be done with Go's string comparison: the
// format orders by utf-16 code units, and an astral character is two of them.

import (
	"errors"
	"fmt"
	"sort"
	"unicode/utf16"
	"unicode/utf8"
)

// putUleb appends v as ULEB128. A dex writes lengths, counts and indices this way, so the number
// of bytes it costs is part of the offsets of everything after it.
func putUleb(b []byte, v uint32) []byte {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			c |= 0x80
			b = append(b, c)
			continue
		}
		return append(b, c)
	}
}

// putSleb appends v as SLEB128, which is the signed form: the sign is carried by the last byte's
// remaining bit, so -1 is 0x7f and not 0xff 0x01.
func putSleb(b []byte, v int32) []byte {
	for {
		c := byte(v & 0x7f)
		// arithmetic shift: the sign has to keep propagating until what is left of v is all
		// zeroes for a positive number or all ones for a negative one
		v >>= 7
		signNegative := v < 0
		done := (v == 0 && c&0x40 == 0) || (v == -1 && c&0x40 != 0)
		if done {
			return append(b, c)
		}
		_ = signNegative
		b = append(b, c|0x80)
	}
}

// putByteAlign pads to a multiple of size with zeroes, the way the format does: an alignment of
// four for the index tables, of eight for a double-width item.
func putByteAlign(b []byte, size uint32) []byte {
	if size < 2 {
		return b
	}
	for uint32(len(b))%size != 0 {
		b = append(b, 0)
	}
	return b
}

// alignUp is where the next item has to start. It answers in absolute terms because the offsets a
// dex declares are absolute: an offset *inside a section buffer* aligned here is wrong twice.
func alignUp(at int, size int) int {
	if size < 2 {
		return at
	}
	if r := at % size; r != 0 {
		return at + size - r
	}
	return at
}

// encodeMUTF8 writes a string the way a dex holds it. The two departures from UTF-8 are spelled
// out rather than left to a table: U+0000 becomes C0 80, and anything above FFFF becomes a pair of
// three-byte sequences, one per surrogate.
func encodeMUTF8(s string) []byte {
	out := []byte{}
	for _, r := range s {
		switch {
		case r == 0:
			out = append(out, 0xC0, 0x80)
		case r < 0x80:
			out = append(out, byte(r))
		case r < 0x800:
			out = append(out, byte(0xC0|r>>6), byte(0x80|r&0x3f))
		case r <= 0xFFFF:
			out = append(out, byte(0xE0|r>>12), byte(0x80|r>>6&0x3f), byte(0x80|r&0x3f))
		default:
			hi, lo := utf16.EncodeRune(r)
			out = append(out,
				byte(0xE0|hi>>12), byte(0x80|hi>>6&0x3f), byte(0x80|hi&0x3f),
				byte(0xE0|lo>>12), byte(0x80|lo>>6&0x3f), byte(0x80|lo&0x3f))
		}
	}
	return out
}

// mutf8OrderLess reports whether a sorts before b in the order the string_ids table requires: by
// utf-16 code unit, not by rune and not by byte. Two strings can be equal in Go and differ here,
// which is why the comparison is written on the encoded form and not on the string.
func mutf8OrderLess(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// writeStrings lays a set of strings out as the string_data_item area of a dex: in code-unit
// order, each one preceded by its utf-16 unit count and terminated by a zero byte, with no
// padding between them (the format gives the items no alignment, which is what makes the
// concatenation comparable).
func writeStrings(items []string) []byte {
	orden := append([]string{}, items...)
	sortByMUTF8(orden)
	out := []byte{}
	for _, s := range orden {
		enc := encodeMUTF8(s)
		out = putUleb(out, uint32(utf16Len(s)))
		out = append(out, enc...)
		out = append(out, 0)
	}
	return out
}

// sortByMUTF8 puts a slice of strings in the order the string_ids table requires, in place.
func sortByMUTF8(s []string) {
	sort.Slice(s, func(i, j int) bool { return mutf8OrderLess(s[i], s[j]) })
}

// checkMUTF8 decodes enough to answer what the format asks of a writer: how many utf-16 units a
// byte sequence holds, and whether what was written reads back as the string it came from.
func checkMUTF8(s string) (int, error) {
	enc := encodeMUTF8(s)
	back, err := decodeMUTF8(enc)
	if err != nil {
		return 0, fmt.Errorf("what we encoded does not decode: %w", err)
	}
	if back != s {
		return 0, fmt.Errorf("round trip changed the string: %q in, %q out", s, back)
	}
	for i, r := range s {
		if r == utf8.RuneError && i == 0 {
			return 0, errors.New("input is not valid utf-8")
		}
	}
	return utf16Len(s), nil
}
