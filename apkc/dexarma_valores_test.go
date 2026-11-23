package main

// Los `encoded_value` que el verificador de `dexdump` comprueba y que el jar real destapó tras
// ordenar los `try_item`: la cadena estática va en bytes little-endian (no en uleb) y los elementos
// de una anotación van ordenados por `name_idx`.

import (
	"bytes"
	"testing"
)

// TestDexLaCadenaEstaticaVaEnBytesLittleEndian: un índice de 300 (0x12c) ocupa dos bytes y la
// cabecera dice anchura uno: `37 2c 01`. Falla si se codifica como uleb (`2c 01` con cabecera 0x17
// o `2c 81 02`…), que es lo que dexdump leía como otro índice.
func TestDexLaCadenaEstaticaVaEnBytesLittleEndian(t *testing.T) {
	td := &dexTables{stringIdx: map[string]int{"hola": 300}, strings: make([]string, 301)}
	vf, b, hay := td.valorEstatico(classField{ConstTag: cpString, ConstStr: "hola"})
	if !hay {
		t.Fatal("la cadena estática no se escribió")
	}
	if vf != byte(valueString|1<<5) {
		t.Errorf("cabecera %#02x, esperaba %#02x", vf, byte(valueString|1<<5))
	}
	if len(b) != 2 || b[0] != 0x2c || b[1] != 0x01 {
		t.Errorf("bytes % x, esperaba 2c 01", b)
	}
}

// TestDexLosElementosDeAnotacionVanPorNameIdx: con `a` en el índice 5 y `b` en el 2, la anotación
// tiene que escribir primero `b`. Falla si se respeta el orden del `.class`.
func TestDexLosElementosDeAnotacionVanPorNameIdx(t *testing.T) {
	td := &dexTables{typeIdx: map[string]int{"LA;": 0}, stringIdx: map[string]int{"a": 5, "b": 2}}
	a := &anotacion{tipo: "LA;", elementos: []par{
		{nombre: "a", valor: valorEntero(1)},
		{nombre: "b", valor: valorEntero(2)},
	}}
	quiero := []byte{visibilidadSistema, 0x00, 0x02, 0x02, 0x04, 0x02, 0x05, 0x04, 0x01}
	if got := a.bytes(td); !bytes.Equal(got, quiero) {
		t.Errorf("elementos % x, esperaba % x", got, quiero)
	}
}
