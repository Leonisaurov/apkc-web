package main

// Un `resources.arsc` real puede poner una entrada compleja (un `style` o un `attr`) que NO es la
// última del bloque de su tipo. El lector tomaba «todo lo que queda del bloque» como registros de
// esa entrada, de modo que un `style` seguido de otra entrada se comía el resto y rechazaba la tabla
// con «declares N records and the block holds M». Lo destaparon apks de terceros (NopeRemote-v2.2 y
// ppcinees); aquí se arma el caso mínimo con los escritores del fork: una entrada compleja y, detrás,
// una simple del mismo tipo.

import "testing"

func TestPreviewRecursosLeeEntradaComplejaSeguidaDeOtra(t *testing.T) {
	tabla := tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "p.q",
		global:        []string{"hola"},
		globalUTF8:    true,
		nombresTipo:   []string{"style"},
		nombresUTF8:   false,
		claves:        []string{"estilo", "texto"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{{
			id:        1,
			specFlags: []uint32{0, 0},
			bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{
					clave:    0,
					flags:    0x0001,
					compleja: true,
					registros: []registroEscribir{{
						nombre: 0x7f0a0000, // un attr del propio paquete, como en un style real
						valor:  valorEscribir{tipo: tipoEnteroDec, datos: 1},
					}},
				},
				// esta es la entrada que el lector viejo se comía como si fuera otro registro
				{clave: 1, valor: valorEscribir{tipo: tipoCadena, datos: 0}},
			}}},
		}},
	}
	datos, err := escribeTablaRecursos(tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	leida, err := leeTabla(datos)
	if err != nil {
		t.Fatalf("la tabla con una compleja seguida de otra no se lee: %v", err)
	}
	bl := leida.bloques[0]
	if len(bl.entradas) != 2 {
		t.Fatalf("el bloque trae %d entradas y se escribieron 2", len(bl.entradas))
	}
	if !bl.entradas[0].compleja || bl.entradas[0].recuentos != 1 ||
		len(bl.entradas[0].crudo) != 12 {
		t.Errorf("la compleja se leyó como %+v", bl.entradas[0])
	}
	if bl.entradas[1].compleja || bl.entradas[1].valor.tipo != tipoCadena {
		t.Errorf("la entrada simple que sigue no se leyó: %+v", bl.entradas[1])
	}
}
