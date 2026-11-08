package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"os"
	"testing"
)

// La prueba de el escritor del resources.arsc (Fase 3-C, parrafo 29). No es un muestreo: se compara
// byte a byte contra el fichero que dejo `aapt2 link` en el fixture (§24, 1088 bytes) hasta el punto
// donde aapt2 abre la tabla en dos configuraciones por el API del atributo, que es la unica
// diferencia que este escritor declara (parrafo 28). Despues se lee lo nuestro con el lector de la
// 3-B, que es lo que va a usar el paso 3.4 para sacar la R.

// tablaDelStudio es el resources.arsc del proyecto del studio como lo describe nuestro modelo: los
// datos salen del fixture leido con el instrumento del parrafo 24 y las banderas de la regla medida
// sobre tres variantes en el parrafo 29 (id -> 0x0004, estilo -> 0x0001, lo demas -> 0).
func tablaDelStudio() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.apkcstudio",
		global:        []string{"res/layout/main.xml"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout", "style"},
		nombresUTF8:   false, // medido: este pool sale UTF-16 y los otros dos UTF-8
		claves:        []string{"pulsar", "main", "AppTheme"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{
				id:        1,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
				}}},
			},
			{
				id:        2,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 1, valor: valorEscribir{tipo: 0x03, datos: 0}},
				}}},
			},
			{
				id:        3,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{
						clave:    2,
						flags:    0x0001,
						compleja: true,
						padre:    0x0103012b,
						// los registros van ordenados por id de atributo, que es como los emite
						// aapt2 (medido en el bloque base y en el -v23 del fixture)
						registros: []registroEscribir{
							{nombre: 0x01010054, valor: valorEscribir{tipo: 0x01, datos: 0x0106000b}},
							{nombre: 0x01010056, valor: valorEscribir{tipo: 0x12, datos: 0xffffffff}},
							{nombre: 0x010102cd, valor: valorEscribir{tipo: 0x12, datos: 0}},
							{nombre: 0x01010451, valor: valorEscribir{tipo: 0x1d, datos: 0xffffffff}},
							{nombre: 0x010104e0, valor: valorEscribir{tipo: 0x12, datos: 0xffffffff}},
						},
					},
				}}},
			},
		},
	}
}

func TestEscribeTablaContraFixture(t *testing.T) {
	fijado, err := base64.StdEncoding.DecodeString(arscTabla)
	if err != nil {
		t.Fatal(err)
	}
	if len(fijado) != 1088 {
		t.Fatalf("el fixture mide %d bytes, no 1088", len(fijado))
	}
	nuestro, err := escribeTablaRecursos(tablaDelStudio())
	if err != nil {
		t.Fatal(err)
	}

	// hasta el spec del estilo (offset 752, medido en el parrafo 24) aapt2 y nosotros escribimos lo
	// mismo: cabecera de tabla, pool global, paquete con sus dos pools, y los tipos id y layout. La
	// unica diferencia de esos 752 bytes son los DOS campos de tamano que cambian al escribir una
	// configuracion en vez de dos: el de la tabla (en +4) y el del paquete (en +72). Se comprueba que
	// el fixture anuncia 1088 y 1020, y se compara con esos dos numeros reemplazados por los nuestros.
	const coincideHasta = 752
	const paqueteEn = 68
	if u32re(fijado, 4) != 1088 || u32re(fijado, paqueteEn+4) != 1088-paqueteEn {
		t.Fatalf("los campos de tamano del fixture no son los medidos: tabla %d, paquete %d",
			u32re(fijado, 4), u32re(fijado, paqueteEn+4))
	}
	esperado := append([]byte{}, fijado...)
	binary.LittleEndian.PutUint32(esperado[4:], uint32(len(nuestro)))
	binary.LittleEndian.PutUint32(esperado[paqueteEn+4:], uint32(len(nuestro)-paqueteEn))
	if !bytes.Equal(nuestro[:coincideHasta], esperado[:coincideHasta]) {
		for i := range nuestro[:coincideHasta] {
			if nuestro[i] != esperado[i] {
				t.Fatalf("el byte #%d es %#02x y el de aapt2 es %#02x", i, nuestro[i], esperado[i])
			}
		}
		t.Fatalf("los primeros %d bytes difieren en alguna parte no cubierta por la comparacion", coincideHasta)
	}

	// y despues de ahi la diferencia es la anunciada: aapt2 emite dos bloques (4 registros el base,
	// 5 en la -v23) y nosotros uno solo con los cinco. 1088 - 152 (el bloque base del estilo) = 936.
	if len(nuestro) != 936 {
		t.Errorf("la tabla mide %d bytes; con una configuracion y cinco registros se esperaban 936", len(nuestro))
	}
	if nuestro[752+10] != 1 || fijado[752+10] != 2 {
		t.Errorf("el spec del estilo dice %d configuraciones escritas y %d en el fixture", nuestro[752+10], fijado[752+10])
	}

	tr, err := leeTabla(nuestro)
	if err != nil {
		t.Fatalf("nuestra propia tabla no se deja leer: %v", err)
	}
	if tr.idPaquete != 0x7f || tr.nombrePaquete != "com.hola.apkcstudio" {
		t.Errorf("paquete leido como %q id=%#08x", tr.nombrePaquete, tr.idPaquete)
	}
	if len(tr.global.cadenas) != 1 || tr.global.cadenas[0] != "res/layout/main.xml" {
		t.Errorf("el pool global salio %v", tr.global.cadenas)
	}
	if len(tr.bloques) != 3 || tr.especificaciones != 3 {
		t.Fatalf("se leyeron %d bloques y %d specs, se esperaban 3 y 3", len(tr.bloques), tr.especificaciones)
	}
	ids := []uint32{}
	for _, bl := range tr.bloques {
		if len(bl.entradas) != 1 {
			t.Fatalf("el tipo %s trae %d entradas", bl.nombre, len(bl.entradas))
		}
		en := bl.entradas[0]
		// el id de recurso es numero de paquete desplazado 24, el id de tipo en los bits 16..23 y el
		// indice de la entrada abajo (parrafo 24: pulsar = 0x7f010000)
		ids = append(ids, tr.idPaquete<<24|uint32(bl.id)<<16)
		if en.clave < 0 || int(en.clave) >= len(tr.claves) {
			t.Fatalf("entrada fuera de la tabla de claves")
		}
		switch bl.nombre {
		case "id":
			if en.compleja || en.valor.tipo != 0x12 || en.flags != 0x0004 {
				t.Errorf("id/%s leida como tam=%d flags=%#x tipo=%#02x", tr.clave(en), en.tam, en.flags, en.valor.tipo)
			}
		case "layout":
			if en.compleja || en.valor.tipo != 0x03 || en.valor.datos != 0 {
				t.Errorf("layout/%s leido como tipo=%#02x datos=%#08x", tr.clave(en), en.valor.tipo, en.valor.datos)
			}
		case "style":
			if !en.compleja {
				t.Fatalf("el estilo no se reconocio como complejo (tam=%d flags=%#x)", en.tam, en.flags)
			}
			if en.padre != 0x0103012b || en.recuentos != 5 || len(en.crudo) != 60 {
				t.Errorf("estilo: padre=%#08x recuento=%d registros=%d", en.padre, en.recuentos, len(en.crudo)/12)
			}
		}
	}
	if ids[0] != 0x7f010000 || ids[1] != 0x7f020000 || ids[2] != 0x7f030000 {
		t.Errorf("los ids de recurso salieron %#08x %#08x %#08x", ids[0], ids[1], ids[2])
	}

	// el instrumento de la medicion exterior: con APKC_ESCRIBE_ARSBC se guarda lo escrito para pasarle
	// por fuera el `aapt2 dump resources` del SDK, que es el oraculo de esta fase. Sin la variable,
	// nada: en wasm no hay filesystem.
	if r := os.Getenv("APKC_ESCRIBE_ARSBC"); r != "" {
		if err := os.WriteFile(r, nuestro, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEscribeTablaRechaza(t *testing.T) {
	base := tablaDelStudio()

	malo := tablaDelStudio()
	malo.tipos[0].bloques[0].entradas[0].clave = len(malo.claves)
	if _, err := escribeTablaRecursos(malo); err == nil {
		t.Error("una entrada que nombra una clave inexistente fue aceptada")
	}

	malo = tablaDelStudio()
	malo.tipos[2].id = 7
	if _, err := escribeTablaRecursos(malo); err == nil {
		t.Error("un tipo con un id que no es su lugar en la tabla de nombres fue aceptado")
	}

	malo = tablaDelStudio()
	malo.tipos[1].specFlags = nil
	if _, err := escribeTablaRecursos(malo); err == nil {
		t.Error("un spec sin una bandera por entrada fue aceptado")
	}

	malo = tablaDelStudio()
	malo.tipos[2].bloques[0].entradas[0].registros[0].nombre = 0x7f010000
	if _, err := escribeTablaRecursos(malo); err == nil {
		t.Error("un registro con un nombre que no es un id del framework fue aceptado")
	}

	malo = tablaDelStudio()
	malo.nombrePaquete = ""
	if _, err := escribeTablaRecursos(malo); err == nil {
		t.Error("un paquete sin nombre fue aceptado")
	}

	malo = tablaDelStudio()
	malo.nombrePaquete = string(bytes.Repeat([]byte("a"), 128))
	if _, err := escribeTablaRecursos(malo); err == nil {
		t.Error("un nombre de paquete de 128 caracteres fue aceptado")
	}

	// la forma buena, intacta: sirve de control de que los cinco rechazos de arriba no son un error
	// que rechace cualquier cosa
	if _, err := escribeTablaRecursos(base); err != nil {
		t.Errorf("la tabla valida fue rechazada: %v", err)
	}
}
