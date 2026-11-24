package main

import (
	"strings"
	"testing"
)

// unMetodoDePrueba arma un método con el code_item dado y sin dex detrás: alcanza para llamar a
// `ejecutaCuerpo` desde una prueba sin compilar fuentes.
func unMetodoDePrueba(nombre string, item *codeItem) *metodoDalvik {
	return &metodoDalvik{nombre: nombre, item: item}
}

// TestLaLlamadaConArgumentosDeMasSeRechazaPorContrato: con `insSize` 1 y tres argumentos el
// intérprete desbordaba el marco de registros y el recover lo disfrazaba de «fallo interno» del
// dex. Es una llamada mal armada por quien invoca: se dice con el método y el conteo, antes del
// recover, y no se marca como dex malformado. El `return-void` de la unidad 0 es un cuerpo válido,
// así que lo único roto es el contrato de la llamada.
func TestLaLlamadaConArgumentosDeMasSeRechazaPorContrato(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)
	met := unMetodoDePrueba("f", &codeItem{registersSize: 1, insSize: 1, insnsSize: 1, insns: []uint16{0x000e}})

	_, err := m.ejecutaCuerpo("Lcom/hola/A;", met,
		[]any{int32(1), int32(2), int32(3)}, []string{"I", "I", "I"})
	if err == nil {
		t.Fatal("la llamada con tres argumentos sobre un solo registro de entrada se ejecutó")
	}
	for _, quiero := range []string{"A.f", "declara 1 registros", "3 argumentos", "no cabe"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("el error de contrato es %q y le falta %q", err, quiero)
		}
	}
	if strings.Contains(err.Error(), "fallo interno") ||
		strings.Contains(err.Error(), "index out of range") ||
		strings.Contains(err.Error(), "dex malformado") {
		t.Errorf("el desborde llegó al recover del dex en vez de validarse antes: %q", err)
	}
	if esMalformado(err) {
		t.Errorf("un error de llamada se marcó como dex malformado: %q", err)
	}

	// el mismo marco con un `long`: una sola palabra de `insSize` tampoco le basta a dos
	_, err = m.ejecutaCuerpo("Lcom/hola/A;", met, []any{int64(1)}, []string{"J"})
	if err == nil {
		t.Fatal("un `long` ocupó dos palabras en un marco de una y no se rechazó")
	}
	if !strings.Contains(err.Error(), "1 argumentos que ocupan 2 palabras") {
		t.Errorf("el error del ancho es %q y se esperaba el conteo en palabras", err)
	}

	// y el control positivo: un argumento que sí cabe se ejecuta sin error
	if _, err := m.ejecutaCuerpo("Lcom/hola/A;", met, []any{int32(7)}, []string{"I"}); err != nil {
		t.Errorf("la llamada que cabe se rechazó: %v", err)
	}
}

// TestUnDexRotoSigueSiendoDexMalformado: la guarda de argumentos no puede tapar el otro camino. Un
// code_item que escribe en un registro fuera del marco (const/16 con AA 0x13 sobre un marco de uno)
// se sigue marcando como dex malformado, con su unidad y su opcode, y sin argumentos de por medio.
func TestUnDexRotoSigueSiendoDexMalformado(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)
	met := unMetodoDePrueba("g", &codeItem{registersSize: 1, insSize: 0, insnsSize: 2,
		insns: []uint16{0x1313, 0}})

	_, err := m.ejecutaCuerpo("Lcom/hola/A;", met, nil, nil)
	if err == nil {
		t.Fatal("el const/16 fuera del marco se ejecutó sin error")
	}
	if !esMalformado(err) {
		t.Errorf("el dex roto no se marcó como malformado: %q", err)
	}
	for _, quiero := range []string{"A.g", "unidad 0", "0x13"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("el error del dex roto es %q y le falta %q", err, quiero)
		}
	}
	if strings.Contains(err.Error(), "declara") && strings.Contains(err.Error(), "registros de entrada") {
		t.Errorf("el dex roto se confundió con un error de llamada: %q", err)
	}
}
