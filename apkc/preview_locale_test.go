package main

import "testing"

// tablaConIdiomas es el resources.arsc de una sonda con `string/saludo` en tres configuraciones:
// la base, `values-es` y `values-es-rMX`. `string/otro` solo se declara en la base, así que en los
// otros dos bloques su entrada está ausente (el offset 0xffffffff del §94). Se escribe con el
// escritor del fork y se lee con leeTabla, como los recursos del §94 y el §107.
func tablaConIdiomas(t *testing.T) *tablaRecursos {
	t.Helper()
	tb := tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.traducido",
		global:        []string{"Hello!", "Hola!", "Hola MX!", "otro"},
		globalUTF8:    true,
		nombresTipo:   []string{"string"},
		nombresUTF8:   false,
		claves:        []string{"saludo", "otro"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{{
			id:        1,
			specFlags: []uint32{0x0004, 0x0004},
			bloques: []bloqueEscribir{
				{entradas: []entradaEscribir{
					{clave: 0, valor: valorEscribir{tipo: 0x03, datos: 0}},
					{clave: 1, valor: valorEscribir{tipo: 0x03, datos: 3}},
				}},
				{config: "es", entradas: []entradaEscribir{
					{clave: 0, valor: valorEscribir{tipo: 0x03, datos: 1}},
					{clave: 1, falta: true},
				}},
				{config: "es-rMX", entradas: []entradaEscribir{
					{clave: 0, valor: valorEscribir{tipo: 0x03, datos: 2}},
					{clave: 1, falta: true},
				}},
			},
		}},
	}
	datos, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla de la sonda: %v", err)
	}
	tr, err := leeTabla(datos)
	if err != nil {
		t.Fatalf("la tabla escrita no se deja leer: %v", err)
	}
	return tr
}

func textoDeTabla(tr *tablaRecursos, id uint32) (string, bool) {
	v, ok := tr.valorDeId(id)
	if !ok || v.tipo != tipoCadena || int(v.datos) >= len(tr.global.cadenas) {
		return "", false
	}
	return tr.global.cadenas[v.datos], true
}

// TestPreviewLeeValoresDelIdioma: sin locale la tabla se queda en la configuración base, que es el
// contrato que tenia la vista previa. Con `es` el valor sale de `values-es`; con `es-MX` gana el
// país exacto sobre el bloque sin país; y una clave ausente en el idioma vuelve a la base.
func TestPreviewLeeValoresDelIdioma(t *testing.T) {
	const saludo uint32 = 0x7f010000
	const otro uint32 = 0x7f010001

	base := tablaConIdiomas(t)
	if texto, ok := textoDeTabla(base, saludo); !ok || texto != "Hello!" {
		t.Errorf("sin locale el saludo es %q (ok=%v), y la base guarda Hello!", texto, ok)
	}

	es := tablaConIdiomas(t)
	es.locale = "es"
	if texto, ok := textoDeTabla(es, saludo); !ok || texto != "Hola!" {
		t.Errorf("con locale es el saludo es %q (ok=%v), y values-es guarda Hola!", texto, ok)
	}
	// una entrada ausente en el bloque localizado no puede borrar la de la base
	if texto, ok := textoDeTabla(es, otro); !ok || texto != "otro" {
		t.Errorf("la clave ausente en es cae a %q (ok=%v), y la base guarda otro", texto, ok)
	}
	// el idioma no cambia la identidad de los ids: siguen nombrando lo mismo
	if tipo, clave, ok := es.nombreDeId(saludo); !ok || tipo != "string" || clave != "saludo" {
		t.Errorf("el locale cambió la identidad del id: %q/%q (ok=%v)", tipo, clave, ok)
	}

	region := tablaConIdiomas(t)
	region.locale = "es-MX"
	if texto, ok := textoDeTabla(region, saludo); !ok || texto != "Hola MX!" {
		t.Errorf("con locale es-MX el saludo es %q (ok=%v), y gana el país exacto", texto, ok)
	}

	otroIdioma := tablaConIdiomas(t)
	otroIdioma.locale = "fr"
	if texto, ok := textoDeTabla(otroIdioma, saludo); !ok || texto != "Hello!" {
		t.Errorf("un idioma sin bloque cae a %q (ok=%v), y la base guarda Hello!", texto, ok)
	}
}
