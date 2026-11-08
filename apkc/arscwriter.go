package main

// El escritor del resources.arsc, segunda pieza de la Fase 3-C (la primera es escribePool, en
// poolrecurso.go). Cada numero de este fichero esta copiado de los bytes que produce el aapt2 de
// este telefono, no de un manual; los parrafos 24, 26, 27, 28 y 29 de docs/evidencia.md son la
// medicion y arscwriter_test.go compara la salida contra el fixture fijado.
//
// Lo que se copio medido: la cabecera de la tabla son 12 bytes con packageCount; dentro del paquete
// (cabecera de 288) van primero los dos pools de nombres y despues, por tipo, un RES_TABLE_TYPE_SPEC
// de 16 + 4*entradas seguido de su RES_TABLE_TYPE; entriesStart vale 84 + 4*entradas porque la
// configuracion base mide 64 y empieza en +20; los offsets de entrada son RELATIVOS a entriesStart;
// y el u16 del spec en +10 es el numero de bloques de tipo que siguen para ese id (1 en id y layout,
// 2 en el AppTheme partido por API) — medido sobre tres variantes en el parrafo 29.
//
// Lo que este escritor NO hace, dicho expreso: emite UNA sola configuracion por tipo. aapt2 parte
// los items de un estilo por el API en que nacio cada atributo y emite una configuracion base y una
// -v23 (parrafo 28); reproducir eso exige la tabla atributo-a-API del framework, que aqui no existe
// de donde sacarla sin parsear android.jar. La diferencia queda en el numero de configuraciones, no
// en los ids, las cadenas ni los valores.

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// valorEscribir es un Res_value: ocho bytes — tam 8, reserva 0, el tipo de dato y los datos. Solo se
// escribe el tam que se midio (8) y la reserva que sale a 0 en todos los casos vistos. `texto` no se
// escribe: es el texto del valor cuando el tipo es una cadena (0x03), y lo que necesita el AXML para
// ponerlo en su pool, porque en el binario los datos de una cadena son un indice del pool.
type valorEscribir struct {
	tipo  byte
	datos uint32
	texto string
}

// registroEscribir es uno de los doce bytes de un valor complejo: el id del atributo y su Res_value.
// Medido en el AppTheme del studio (parrafo 27): nombre u32, y detras tam=8, res0=0, tipo, datos.
type registroEscribir struct {
	nombre uint32
	valor  valorEscribir
}

// entradaEscribir es un ResTable_entry. Simple: declara 8 bytes y lleva el Res_value detras.
// Compleja (estilo o attr): declara 16, el id del padre en +8 y EL NUMERO DE REGISTROS en +12, y
// exactamente esa cantidad de registros de doce bytes a continuacion.
type entradaEscribir struct {
	clave     int
	flags     uint16
	valor     valorEscribir
	compleja  bool
	padre     uint32
	registros []registroEscribir
	// falta: la clave no existe en esta configuracion. El offset del entriesStart se escribe
	// 0xffffffff y no hay bytes de entrada (parrafo 94: `values-es` con una clave que la base tiene).
	falta bool
}

// bloqueEscribir es una ResTable_type de una configuracion. `config` es el calificador medido (hoy
// solo el idioma de `values-es`); vacío es la base.
type bloqueEscribir struct {
	config   string
	entradas []entradaEscribir
}

// tipoEscribir es un id de tipo con sus configuraciones. El id es el lugar que ocupa en nombresTipo
// mas uno: aapt2 numera los tipos por orden alfabetico de nombre y los emite en ese orden (medido en
// las tres variantes del parrafo 29: attr, bool, color, id, layout, string, style con ids 1..7).
type tipoEscribir struct {
	id        byte
	specFlags []uint32
	bloques   []bloqueEscribir
}

// tablaEscribir es el resources.arsc completo. Cada pool se escribe en el modo que se pida (medido
// en el fixture: el global y el de claves en UTF-8, el de nombres de tipo en UTF-16) y las banderas
// de cada entrada viajan crudas, con la regla que se midio en el parrafo 29.
type tablaEscribir struct {
	// idPaquete es el NUMERO de paquete, no el prefijo de los ids: en el fixture mide 0x7f y los ids
	// de recurso salen de desplazandolo veinticuatro bits (parrafo 24)
	idPaquete     uint32
	nombrePaquete string
	global        []string
	globalUTF8    bool
	nombresTipo   []string
	nombresUTF8   bool
	claves        []string
	clavesUTF8    bool
	tipos         []tipoEscribir
}

// escribeTablaRecursos arma el resources.arsc. Los unicos rechazos son los que se pueden comprobar
// con lo medido: el nombre de paquete fuera de los 128 caracteres UTF-16, indices de clave fuera de
// la tabla, ids de tipo sin nombre o fuera de su lugar, un spec sin una bandera por entrada, y un
// nombre de registro que no sea un id del framework, que es la unica forma que tiene medida.
func escribeTablaRecursos(t tablaEscribir) ([]byte, error) {
	unidades := utf16.Encode([]rune(t.nombrePaquete))
	if len(unidades) == 0 || len(unidades) > 127 {
		return nil, fmt.Errorf("el nombre de paquete %q no cabe en los 128 caracteres UTF-16 medidos", t.nombrePaquete)
	}
	if len(t.tipos) > len(t.nombresTipo) {
		return nil, fmt.Errorf("%d tipos escritos con %d nombres", len(t.tipos), len(t.nombresTipo))
	}
	for i, ti := range t.tipos {
		if int(ti.id) != i+1 {
			return nil, fmt.Errorf("el tipo #%d declara id %d: aapt2 numera los tipos por su lugar en la tabla de nombres", i, ti.id)
		}
		if len(ti.bloques) == 0 {
			return nil, fmt.Errorf("el tipo %d no tiene ninguna configuracion", ti.id)
		}
		for _, bloque := range ti.bloques {
			if len(ti.specFlags) != len(bloque.entradas) {
				return nil, fmt.Errorf("el spec del tipo %d da %d banderas para %d entradas", ti.id, len(ti.specFlags), len(bloque.entradas))
			}
			for j, e := range bloque.entradas {
				if e.falta {
					continue
				}
				if e.clave < 0 || e.clave >= len(t.claves) {
					return nil, fmt.Errorf("la entrada %d del tipo %d nombra la clave %d y hay %d", j, ti.id, e.clave, len(t.claves))
				}
				if !e.compleja {
					continue
				}
				for k, r := range e.registros {
					if r.nombre&0xff000000 != 0x01000000 {
						return nil, fmt.Errorf("registro %d de la entrada %d del tipo %d: nombre %#08x, y la unica forma medida es un id del framework (0x01……)", k, j, ti.id, r.nombre)
					}
				}
			}
		}
	}

	nombres := escribePool(t.nombresTipo, t.nombresUTF8)
	claves := escribePool(t.claves, t.clavesUTF8)
	global := escribePool(t.global, t.globalUTF8)

	interior := []byte{}
	for _, ti := range t.tipos {
		interior = append(interior, escribeSpec(ti)...)
		for _, bloque := range ti.bloques {
			interior = append(interior, escribeBloqueTipo(ti, bloque)...)
		}
	}

	const cabeceraPaquete = 288
	// El cuerpo del paquete, sin los 8 bytes del encabezado comun: id (4) + nombre (128 unidades
	// UTF-16 = 256) + los cinco u32 de atras = 280, y 8 + 280 son justo los 288 medidos.
	cuerpo := putU32(nil, t.idPaquete)
	for _, u := range unidades {
		cuerpo = putU16(cuerpo, u)
	}
	for len(cuerpo) < 4+256 {
		cuerpo = putU16(cuerpo, 0)
	}
	cuerpo = putU32(cuerpo, cabeceraPaquete) // typeStrings
	cuerpo = putU32(cuerpo, 0)               // lastPublicType
	cuerpo = putU32(cuerpo, uint32(cabeceraPaquete+len(nombres)))
	cuerpo = putU32(cuerpo, 0) // lastPublicKey: vale 0 incluso con <public> declarados (parrafo 29)
	cuerpo = putU32(cuerpo, 0) // typeIdOffset

	paquete := trozoCabecera(trozoPaquete, cabeceraPaquete,
		cabeceraPaquete+len(nombres)+len(claves)+len(interior))
	paquete = append(paquete, cuerpo...)
	paquete = append(paquete, nombres...)
	paquete = append(paquete, claves...)
	paquete = append(paquete, interior...)

	out := trozoCabecera(trozoTabla, 12, 12+len(global)+len(paquete))
	out = putU32(out, 1) // packageCount: el unico que lee y escribe este paso
	out = append(out, global...)
	return append(out, paquete...), nil
}

// trozoCabecera escribe el ResChunk_header comun: tipo, tamano de cabecera y tamano total.
func trozoCabecera(tipo uint16, hs, size int) []byte {
	b := putU16(nil, tipo)
	b = putU16(b, uint16(hs))
	return putU32(b, uint32(size))
}

// escribeSpec: RES_TABLE_TYPE_SPEC, 16 bytes de cabecera mas una bandera u32 por entrada. El u16 de
// +10 cuenta CUANTOS bloques de tipo siguen para este id; aqui siempre uno. Las banderas las pone el
// llamante porque no son visibles: aapt2 escribe 0x00000400 cuando parte un estilo por API y
// 0x40000000 cuando la entrada se declaro con <public> (parrafo 29).
func escribeSpec(ti tipoEscribir) []byte {
	n := 0
	if len(ti.bloques) > 0 {
		n = len(ti.bloques[0].entradas)
	}
	b := trozoCabecera(trozoSpec, 16, 16+4*n)
	b = append(b, ti.id, 0)
	b = putU16(b, uint16(len(ti.bloques))) // numero de configuraciones que siguen
	b = putU32(b, uint32(n))
	for _, f := range ti.specFlags {
		b = putU32(b, f)
	}
	return b
}

// escribeBloqueTipo: RES_TABLE_TYPE con la configuracion base. La configuracion son 64 bytes cuyo
// unico campo no nulo, medido, es el primero: su propio tamano (64). El bloque -v23 del AppTheme
// pone ademas 23 en el u32 de +24, que es la version de API de la configuracion.
func escribeBloqueTipo(ti tipoEscribir, bloque bloqueEscribir) []byte {
	const cabecera = 84 // 20 de campos + 64 de configuracion
	entradas := []byte{}
	offsets := []uint32{}
	for _, e := range bloque.entradas {
		if e.falta {
			offsets = append(offsets, 0xffffffff)
			continue
		}
		offsets = append(offsets, uint32(len(entradas)))
		entradas = apilaEntrada(entradas, e)
	}
	entriesStart := cabecera + 4*len(bloque.entradas)

	b := trozoCabecera(trozoTipo, cabecera, entriesStart+len(entradas))
	b = append(b, ti.id, 0)
	b = putU16(b, 0) // reservado: 0 en todo lo medido
	b = putU32(b, uint32(len(bloque.entradas)))
	b = putU32(b, uint32(entriesStart))
	b = append(b, configDe(bloque.config)...)
	for _, o := range offsets {
		b = putU32(b, o)
	}
	return append(b, entradas...)
}

// configDe: los 64 bytes de ResTable_config que midió el aapt2 del aparato. La base solo declara su
// tamaño (64). `values-es` escribe 'e' y 's' en language[0] y language[1] (offset +8 y +9); el resto
// queda a cero. Medido en el volcado del parrafo 94.
func configDe(lengua string) []byte {
	b := make([]byte, 64)
	binary.LittleEndian.PutUint32(b, 64)
	switch lengua {
	case "":
	case "land":
		// ResTable_config::orientation = ORIENTATION_LAND (2), en +12
		b[12] = 2
	case "hdpi":
		// density = DENSITY_HDPI (240) en +14 y sdkVersion 4 en +24
		b[14], b[15] = 0xf0, 0
		b[24] = 4
	default:
		if len(lengua) != 2 {
			// una región (`es-rMX`) pide country y otra máscara que no está medida
			return b
		}
		b[8], b[9] = lengua[0], lengua[1]
	}
	return b
}

// apilaEntrada escribe una entrada y lo que lleva detras: el Res_value de ocho bytes si es simple,
// o el padre, el recuento y los registros si es compleja.
func apilaEntrada(b []byte, e entradaEscribir) []byte {
	if !e.compleja {
		b = putU16(b, 8)
		b = putU16(b, e.flags)
		b = putU32(b, uint32(e.clave))
		return apilaValor(b, e.valor)
	}
	b = putU16(b, 16)
	b = putU16(b, e.flags)
	b = putU32(b, uint32(e.clave))
	b = putU32(b, e.padre)
	b = putU32(b, uint32(len(e.registros)))
	for _, r := range e.registros {
		b = putU32(b, r.nombre)
		b = apilaValor(b, r.valor)
	}
	return b
}

// apilaValor: los ocho bytes de un Res_value.
func apilaValor(b []byte, v valorEscribir) []byte {
	b = putU16(b, 8) // tam
	b = append(b, 0) // res0: 0 en todo lo medido
	b = append(b, v.tipo)
	return putU32(b, v.datos)
}
