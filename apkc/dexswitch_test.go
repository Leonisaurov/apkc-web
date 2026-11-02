package main

import (
	"encoding/base64"
	"testing"
)

// La clase con la que se midió el switch (parrafo 64): `p.Ramas` de javac 21, con un `switch` denso de
// ocho casos (0..7, `tableswitch` en el `.class`) y otro disperso (1, 100 y 1000, `lookupswitch`).
const ramasJavac = "yv66vgAAAEEAFgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAB3AvUmFtYXMBAAV0b3RhbAEAAUkBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAFZGVuc28BAAQoSSlWAQANU3RhY2tNYXBUYWJsZQEACGRpc3BlcnNvAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAKUmFtYXMuamF2YQAhAAgAAgAAAAEACAALAAwAAAAEAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAgAJAA8AEAABAA0AAADJAAEAAQAAAHcaqgAAAAAAbwAAAAAAAAAHAAAALwAAADcAAAA/AAAARwAAAE8AAABXAAAAXwAAAGcQZLMAB6cAQRBlswAHpwA5EGazAAenADEQZ7MAB6cAKRBoswAHpwAhEGmzAAenABkQarMAB6cAERBrswAHpwAJEQPnswAHsQAAAAIADgAAAC4ACwAAAAUAMAAGADgABwBAAAgASAAJAFAACgBYAAsAYAAMAGgADQBwAA4AdgAQABEAAAAMAAowBwcHBwcHBwcFAAkAEgAQAAEADQAAAHgAAQABAAAAPxqrAAAAAAA4AAAAAwAAAAEAAAAjAAAAZAAAACoAAAPoAAAAMQSzAAenABYFswAHpwAPBrMAB6cACBAJswAHsQAAAAIADgAAABoABgAAABIAJAATACsAFAAyABUAOQAWAD4AGAARAAAABwAFJAYGBgQACAATAAYAAQANAAAAHQABAAAAAAAFA7MAB7EAAAABAA4AAAAGAAEAAAADAAEAFAAAAAIAFQ=="

func leeClaseDelFixture(t *testing.T, nombre, b64 string) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("%s no es base64: %v", nombre, err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("%s no se lee: %v", nombre, err)
	}
	return cf
}

// TestElSwitchSeDexaComoPackedYSparse: el `tableswitch` del `.class` sale como `packed-switch` y el
// `lookupswitch` como `sparse-switch`, con los payloads que midió el aparato: `0100` + tamaño +
// first_key + destinos, y `0200` + tamaño + **todas** las claves y después todos los destinos
// (intercalarlos hace que ART lea una clave como destino: lo dijo el verificador con nombre).
func TestElSwitchSeDexaComoPackedYSparse(t *testing.T) {
	ramas := leeClaseDelFixture(t, "Ramas", ramasJavac)
	b, err := dexDeClases([]classFull{ramas})
	if err != nil {
		t.Fatalf("el dexer rechazó el switch: %v", err)
	}
	verificaLayout(t, b, "el dex del switch")
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}

	denso, err := codeItemDeMetodo(d, "denso")
	if err != nil {
		t.Fatalf("denso: %v", err)
	}
	if denso.insns[0]&0xff != 0x2b {
		t.Fatalf("denso no empieza con `packed-switch` (0x2b): %#04x", denso.insns[0])
	}
	payload := int(int32(uint32(denso.insns[1]) | uint32(denso.insns[2])<<16))
	if payload <= 0 || payload >= len(denso.insns) {
		t.Fatalf("el payload de denso cae en %d y el cuerpo tiene %d unidades", payload, len(denso.insns))
	}
	if denso.insns[payload] != identEmpaquetado {
		t.Errorf("el payload de denso no lleva el ident 0x0100: %#04x", denso.insns[payload])
	}
	if denso.insns[payload+1] != 8 {
		t.Errorf("el payload de denso anuncia %d claves y son ocho", denso.insns[payload+1])
	}
	if denso.insns[payload+2] != 0 || denso.insns[payload+3] != 0 {
		t.Errorf("la primera clave del payload denso no es cero: %#04x %#04x",
			denso.insns[payload+2], denso.insns[payload+3])
	}

	disperso, err := codeItemDeMetodo(d, "disperso")
	if err != nil {
		t.Fatalf("disperso: %v", err)
	}
	if disperso.insns[0]&0xff != 0x2c {
		t.Fatalf("disperso no empieza con `sparse-switch` (0x2c): %#04x", disperso.insns[0])
	}
	pl := int(int32(uint32(disperso.insns[1]) | uint32(disperso.insns[2])<<16))
	if pl <= 0 || pl >= len(disperso.insns) {
		t.Fatalf("el payload de disperso cae en %d y el cuerpo tiene %d unidades", pl, len(disperso.insns))
	}
	if disperso.insns[pl] != identDisperso {
		t.Errorf("el payload de disperso no lleva el ident 0x0200: %#04x", disperso.insns[pl])
	}
	if disperso.insns[pl+1] != 3 {
		t.Errorf("el payload de disperso anuncia %d claves y son tres", disperso.insns[pl+1])
	}
	// las tres claves, ascendentes y seguidas: 1, 100 y 1000
	claves := []int32{1, 100, 1000}
	for k, quiere := range claves {
		got := int32(uint32(disperso.insns[pl+2+2*k]) | uint32(disperso.insns[pl+3+2*k])<<16)
		if got != quiere {
			t.Errorf("clave #%d del payload: %d, medida %d", k, got, quiere)
		}
	}
	// y los tres destinos, detrás de las claves, apuntando dentro del cuerpo
	for k := 0; k < 3; k++ {
		en := pl + 2 + 2*len(claves) + 2*k
		destino := int(int32(uint32(disperso.insns[en]) | uint32(disperso.insns[en+1])<<16))
		abs := destino
		if abs <= 0 || abs >= len(disperso.insns) {
			t.Errorf("destino #%d del payload cae en %d, fuera del cuerpo (%d unidades)", k, abs, len(disperso.insns))
		}
	}
}

// TestElClinitLlevaConstructor: ART lo dijo con nombre —«<clinit> didn't have expected constructor
// access flag» y «not marked as constructor»— y el bit es el mismo ACC_CONSTRUCTOR de los `<init>`.
func TestElClinitLlevaConstructor(t *testing.T) {
	ramas := leeClaseDelFixture(t, "Ramas", ramasJavac)
	b, err := dexDeClases([]classFull{ramas})
	if err != nil {
		t.Fatalf("dexando: %v", err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("releyendo: %v", err)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("class_data: %v", err)
	}
	encontrado := false
	for _, m := range append(append([]encodedMethod{}, porClase["Lp/Ramas;"].directs...),
		porClase["Lp/Ramas;"].virtuals...) {
		if d.methods[m.methodIdx].name != "<clinit>" {
			continue
		}
		encontrado = true
		if m.accessFlags&0x10000 == 0 {
			t.Errorf("el <clinit> sale con flags %#x y le falta ACC_CONSTRUCTOR (0x10000)", m.accessFlags)
		}
	}
	if !encontrado {
		t.Error("el class_data no anuncia el <clinit>, que javac escribe para `total = 0`")
	}
}
