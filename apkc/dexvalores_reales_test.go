package main

import "testing"

// Las tuplas de estos tests salieron de los APK reales medidos en §354 y se copiaron byte a byte.
// El guion de extracción vive fuera del repo, en ~/.cache/apkc-web/apps-reales-fase-a/extraer.py.

// F-Droid, classes.dex: L_COROUTINE/CoroutineDebuggingKt; tiene un solo campo estático y su
// encoded_array_item es `01 37 0b 7e`: tamaño 1, VALUE_STRING con dos bytes de índice. Los bytes
// little-endian `0b 7e` dan 32267, que es la cadena "_COROUTINE" del string_ids de ese dex.
func TestValoresEstaticosRealesFDroid(t *testing.T) {
	raw := []byte{0x01, 0x37, 0x0b, 0x7e}
	if got := anchoValor(raw[1]); got != 2 {
		t.Fatalf("anchoValor(0x37) = %d, quería 2", got)
	}
	valores, err := (&dexFile{raw: raw}).valoresEstaticos(0)
	if err != nil {
		t.Fatalf("valoresEstaticos: %v", err)
	}
	if len(valores) != 1 {
		t.Fatalf("%d valores, quería 1", len(valores))
	}
	if valores[0].valueType != valueString || valores[0].value != 32267 {
		t.Fatalf("valor #0 = (tipo %#x, valor %d), quería (0x17, 32267)",
			valores[0].valueType, valores[0].value)
	}
}

// NewPipe, classes.dex: Landroidx/compose/foundation/layout/SpacerKt; declara 8 campos
// estáticos y su encoded_array_item es `04 1e 1e 1e 04 0f`: 4 valores (tres null y un int 15).
func TestValoresEstaticosRealesNewPipe(t *testing.T) {
	raw := []byte{0x04, 0x1e, 0x1e, 0x1e, 0x04, 0x0f}
	valores, err := (&dexFile{raw: raw}).valoresEstaticos(0)
	if err != nil {
		t.Fatalf("valoresEstaticos: %v", err)
	}
	if len(valores) != 4 {
		t.Fatalf("%d valores, quería 4", len(valores))
	}
	quiero := []struct {
		tipo  byte
		valor uint64
	}{{valueNull, 0}, {valueNull, 0}, {valueNull, 0}, {valueInt, 15}}
	for i, q := range quiero {
		if valores[i].valueType != q.tipo || valores[i].value != q.valor {
			t.Errorf("valor #%d = (tipo %#x, valor %d), quería (%#x, %d)",
				i, valores[i].valueType, valores[i].value, q.tipo, q.valor)
		}
	}
	completos, err := completaValoresEstaticos(valores, 8)
	if err != nil {
		t.Fatalf("completaValoresEstaticos: %v", err)
	}
	if len(completos) != 8 {
		t.Fatalf("%d campos tras completar, quería 8", len(completos))
	}
	for i := 4; i < 8; i++ {
		if completos[i].valueType != valueNull {
			t.Errorf("campo #%d = %#x, quería VALUE_NULL", i, completos[i].valueType)
		}
	}
	if _, err := completaValoresEstaticos(valores, 3); err == nil {
		t.Error("sobrar valores no se rechazó")
	}
}

// Un array anidado se recorre de golpe: sin saltar sus dos valores, el lector confundiría el
// VALUE_STRING que viene detrás con el contenido del array.
func TestValoresEstaticosRealesArrayAnidado(t *testing.T) {
	raw := []byte{0x01, 0x1c, 0x02, 0x04, 0x07, 0x17, 0x05}
	if anchoValor(raw[1]) != anchoVariable {
		t.Fatal("VALUE_ARRAY no se marcó como de anchura variable")
	}
	valores, err := (&dexFile{raw: raw}).valoresEstaticos(0)
	if err != nil {
		t.Fatalf("valoresEstaticos: %v", err)
	}
	if len(valores) != 1 || valores[0].valueType != valueArray {
		t.Fatalf("leyó %d valores, quería un VALUE_ARRAY", len(valores))
	}
}
