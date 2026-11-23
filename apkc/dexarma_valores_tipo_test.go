package main

// Los `encoded_value` de un campo estático entero llevan el TIPO del descriptor del campo, no
// siempre `VALUE_INT`. ART comprueba `class_defs.static_values_off` al cargar el fichero y rechaza
// el dex entero con «unexpected static field initial value type: 'I' vs 'Z'» (medido en Droidloom
// con `banco/art-jar-real/`), y `dexdump` no lo comprueba: por eso la prueba vive aquí.

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"
)

// TestElValorEstaticoEnteroTomaElTipoDelCampo fija las formas medidas con `d8`
// (build-tools 35.0.1, `--release --min-api 21`) sobre `Tipos.class`, leídas del
// `static_values_off` del fichero que emite:
//
//	Z  0x1f falso y 0x3f verdadero, sin bytes (el valor va en el campo del tamaño)
//	C  0x03 con el ancho mínimo sin signo (0x23 con dos bytes)
//	B  0x00 con un byte siempre
//	S  0x02 con el ancho mínimo con signo (0x22 con dos bytes)
//	I  0x04 con el ancho mínimo con signo (0x64 con cuatro bytes)
func TestElValorEstaticoEnteroTomaElTipoDelCampo(t *testing.T) {
	casos := []struct {
		desc  string
		valor int32
		vf    byte
		bytes []byte
	}{
		{"Z", 0, valueBool, nil},
		{"Z", 1, valueBool | 1<<5, nil},
		{"C", 0, valueChar, []byte{0x00}},
		{"C", 57, valueChar, []byte{0x39}},
		{"C", 65535, valueChar | 1<<5, []byte{0xff, 0xff}},
		{"B", 0, valueByte, []byte{0x00}},
		{"B", -1, valueByte, []byte{0xff}},
		{"B", -128, valueByte, []byte{0x80}},
		{"S", -1, valueShort, []byte{0xff}},
		{"S", 32767, valueShort | 1<<5, []byte{0xff, 0x7f}},
		{"S", -32768, valueShort | 1<<5, []byte{0x00, 0x80}},
		{"I", 0, valueInt, []byte{0x00}},
		{"I", 1, valueInt, []byte{0x01}},
		{"I", -1, valueInt, []byte{0xff}},
		{"I", 0x7fffffff, valueInt | 3<<5, []byte{0xff, 0xff, 0xff, 0x7f}},
		{"I", -0x80000000, valueInt | 3<<5, []byte{0x00, 0x00, 0x00, 0x80}},
	}
	for _, c := range casos {
		vf, b := valorEnteroEstatico(c.desc, c.valor)
		if vf != c.vf || !bytes.Equal(b, c.bytes) {
			t.Errorf("campo %s con %d: %02x % x, esperaba %02x % x",
				c.desc, c.valor, vf, b, c.vf, c.bytes)
		}
	}
}

// tiposJavac: un `static final` de cada tipo entero, con los extremos de cada ancho.
// `javac --release 8 -g:lines,source Tipos.java` y `base64 -w0 Tipos.class`. Las formas que `d8`
// escribe para este mismo `.class` están en la tabla de `valorEnteroEstatico` y se exigen abajo tal
// cual: `1f`/`3f`, `03 39`, `23 ff ff`, `00 80`, `02 ff`, `22 ff 7f`, `04 00`, `64 00 00 00 80`.
const tiposJavac = "yv66vgAAADQAJwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAFVGlwb3MBAAJaVAEAAVoBAA1Db25zdGFudFZhbHVlAwAAAAEBAAJaRgMAAAAAAQACQzkBAAFDAwAAADkBAARDTUFYAwAA//8BAARCTUlOAQABQgP///+AAQAEQk5FRwP/////AQAEU01JTgEAAVMD//+AAAEABFNNQVgDAAB//wEAAkkwAQABSQEABElMT1cDgAAAAAEABElORUcBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAKU291cmNlRmlsZQEAClRpcG9zLmphdmEAIQAHAAIAAAALABgACQAKAAEACwAAAAIADAAYAA0ACgABAAsAAAACAA4AGAAPABAAAQALAAAAAgARABgAEgAQAAEACwAAAAIAEwAYABQAFQABAAsAAAACABYAGAAXABUAAQALAAAAAgAYABgAGQAaAAEACwAAAAIAGwAYABwAGgABAAsAAAACAB0AGAAeAB8AAQALAAAAAgAOABgAIAAfAAEACwAAAAIAIQAYACIAHwABAAsAAAACABgAAQABAAUABgABACMAAAAdAAEAAQAAAAUqtwABsQAAAAEAJAAAAAYAAQAAAAEAAQAlAAAAAgAm"

// valoresCrudos: los bytes de cada encoded_value de un encoded_array_item, cabecera incluida, tal
// y como quedaron en el fichero. Es lo que `valoresEstaticos` no devuelve (se queda con el tipo
// sin el campo del tamaño), y aquí hace falta para exigir `3f` frente a `1f`.
func valoresCrudos(d *dexFile, off uint32) ([][]byte, error) {
	n, at, err := d.uleb(off)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, n)
	for i := uint32(0); i < n; i++ {
		if int(at) >= len(d.raw) {
			return nil, fmt.Errorf("valor #%d se sale del fichero", i)
		}
		ini := at
		ancho := anchoValor(d.raw[at])
		if ancho < 0 {
			return nil, fmt.Errorf("valor #%d con el tipo %#02x, que nadie midió", i, d.raw[at])
		}
		at += 1 + uint32(ancho)
		if int(at) > len(d.raw) {
			return nil, fmt.Errorf("valor #%d se sale del fichero", i)
		}
		out = append(out, d.raw[ini:at])
	}
	return out, nil
}

// TestElDexEscritoLlevaElTipoDeCadaCampoEstatico dexa el `.class` de javac y cruza cada
// `encoded_value` del `static_values_off` con el descriptor de su campo. Falla si un `boolean` o un
// `char` se anuncia como `int`: es el defecto que ART rechaza y `dexdump` no ve.
func TestElDexEscritoLlevaElTipoDeCadaCampoEstatico(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(tiposJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	clase, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	dex, err := dexDeClases([]classFull{clase})
	if err != nil {
		t.Fatalf("dexando: %v", err)
	}
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	var objetivo *dexClass
	for i := range d.classes {
		if d.classes[i].name == "LTipos;" {
			objetivo = &d.classes[i]
		}
	}
	if objetivo == nil {
		t.Fatal("la clase no está en el dex")
	}
	cd, err := d.readClassData(objetivo.dataOff)
	if err != nil {
		t.Fatalf("leyendo el class_data: %v", err)
	}
	crudos, err := valoresCrudos(d, objetivo.staticValuesOff)
	if err != nil {
		t.Fatalf("leyendo static_values_off: %v", err)
	}
	if len(crudos) != len(cd.staticFields) {
		t.Fatalf("%d valores para %d campos estáticos", len(crudos), len(cd.staticFields))
	}
	quiero := map[string][]byte{
		"ZT":   {0x3f},
		"ZF":   {0x1f},
		"C9":   {0x03, 0x39},
		"CMAX": {0x23, 0xff, 0xff},
		"BMIN": {0x00, 0x80},
		"BNEG": {0x00, 0xff},
		"SMIN": {0x22, 0x00, 0x80},
		"SMAX": {0x22, 0xff, 0x7f},
		"I0":   {0x04, 0x00},
		"ILOW": {0x64, 0x00, 0x00, 0x00, 0x80},
		"INEG": {0x04, 0xff},
	}
	if len(quiero) != len(cd.staticFields) {
		t.Fatalf("la tabla de formas cubre %d campos y el fixture tiene %d", len(quiero), len(cd.staticFields))
	}
	for i, f := range cd.staticFields {
		campo := d.fields[f.fieldIdx]
		e, ok := quiero[campo.name]
		if !ok {
			t.Fatalf("%s: campo sin forma medida", campo.name)
		}
		if !bytes.Equal(crudos[i], e) {
			t.Errorf("%s (%s): % x, esperaba % x", campo.name, campo.typeOf, crudos[i], e)
		}
	}
}
