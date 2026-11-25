package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

// Cadena #808 del classes.dex de Termux arm (§354): el string_data_item declara 18 unidades
// UTF-16 (`12` como uleb128) y trae 46 bytes de cuerpo. El lector antiguo producía 20 unidades:
// el sustituto suelto `ed a0 80` se emitía como UTF-8 y Go lo recorría como tres RuneError.
func TestMutf8RealTermux(t *testing.T) {
	cuerpo := []byte{
		0xc0, 0x80, 0x7f, 0xc2, 0xad, 0xd8, 0x80, 0xd8, 0x9c, 0xdb, 0x9d, 0xdc, 0x8f,
		0xe0, 0xa3, 0xa2, 0xe1, 0x9a, 0x80, 0xe1, 0xa0, 0x8e, 0xe2, 0x80, 0x80,
		0xe2, 0x80, 0xa8, 0xe2, 0x81, 0x9f, 0xe2, 0x81, 0xa6, 0xe3, 0x80, 0x80,
		0xed, 0xa0, 0x80, 0xef, 0xbb, 0xbf, 0xef, 0xbf, 0xb9,
	}
	if len(cuerpo) != 46 {
		t.Fatalf("el cuerpo mide %d bytes, quería 46", len(cuerpo))
	}
	s, err := decodeMUTF8(cuerpo)
	if err != nil {
		t.Fatalf("decodeMUTF8: %v", err)
	}
	if got := utf16Len(s); got != 18 {
		t.Errorf("utf16Len = %d, quería 18", got)
	}
	if !strings.ContainsRune(s, runeError()) {
		t.Error("el sustituto suelto no se convirtió en un solo carácter de reemplazo")
	}
	if strings.Count(s, string(runeError())) != 1 {
		t.Errorf("%d caracteres de reemplazo, quería 1", strings.Count(s, string(runeError())))
	}

	// El mismo cuerpo dentro de un dex mínimo: readStringData acepta la cadena #0 sin el error
	// «claims 18 utf-16 units and holds 20».
	raw := make([]byte, 0, 4+1+len(cuerpo)+1)
	raw = append(raw, 0x04, 0x00, 0x00, 0x00) // string_ids[0] apunta al byte 4
	raw = append(raw, 0x12)                   // utf16Count = 18
	raw = append(raw, cuerpo...)
	raw = append(raw, 0x00) // terminador
	d := &dexFile{
		raw:      raw,
		tableOff: map[string][2]uint32{"string_ids": {1, 0}},
	}
	if err := d.readStringData(); err != nil {
		t.Fatalf("readStringData: %v", err)
	}
	if len(d.strings) != 1 || utf16Len(d.strings[0]) != 18 {
		t.Fatalf("leyó %d cadenas con longitudes distintas de 18", len(d.strings))
	}
	if got := binary.LittleEndian.Uint32(raw[:4]); got != 4 {
		t.Fatalf("el offset del string_ids[0] es %d, quería 4", got)
	}
}

func runeError() rune { return 0xfffd }
