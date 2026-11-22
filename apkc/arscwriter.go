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
	"strconv"
	"strings"
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
	// nombreId, si no está vacío, es el nombre de un recurso `id` cuyo número no se conoce todavía:
	// los hijos `enum`/`flags` de un atributo se guardan con el id de su propio recurso, que depende
	// del orden de la tabla (§135)
	nombreId string
	valor    valorEscribir
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
	// styleables son los <declare-styleable>; no llevan entrada en resources.arsc, solo la clase
	// styleable de R.java (parrafo 106).
	styleables []styleableEscribir
}

type styleableEscribir struct {
	nombre string
	attrs  []string
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
	anterior := 0
	for _, ti := range t.tipos {
		// aapt2 reserva un id por tipo declarado, incluso si todas sus entradas se descartan por
		// falta de valor base: deja un hueco con un nombre de relleno `?N` (medido con un bool y
		// un integer que solo existen en values-es; string quedo con id 3)
		if int(ti.id) < 1 || int(ti.id) > len(t.nombresTipo) {
			return nil, fmt.Errorf("el tipo con id %d esta fuera de la tabla de %d nombres", ti.id, len(t.nombresTipo))
		}
		if int(ti.id) <= anterior {
			return nil, fmt.Errorf("el tipo con id %d no va despues del %d", ti.id, anterior)
		}
		anterior = int(ti.id)
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
					// las dos formas medidas: un atributo del framework (0x01……) en un estilo, y el id
					// de un recurso `id` del propio paquete en el mapa de un `enum`/`flags` (§135)
					if r.nombre&0xff000000 != 0x01000000 && r.nombre&0xff000000 != 0x7f000000 {
						return nil, fmt.Errorf("registro %d de la entrada %d del tipo %d: nombre %#08x, y solo se midieron ids del framework (0x01……) y del paquete (0x7f……)", k, j, ti.id, r.nombre)
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

// densidadDe: el número de una carpeta `drawable-…` medida, o 0 si el calificador no es una
// densidad. Medido con aapt2 en el §137: ldpi 120, mdpi 160, hdpi 240, xhdpi 320, xxhdpi 480 y
// xxxhdpi 640.
func densidadDe(calificador string) uint16 {
	switch calificador {
	case "ldpi":
		return 120
	case "mdpi":
		return 160
	case "hdpi":
		return 240
	case "xhdpi":
		return 320
	case "xxhdpi":
		return 480
	case "xxxhdpi":
		return 640
	case "tvdpi":
		return 213
	case "nodpi":
		return 0xffff // DENSITY_ANY, medido en §159
	}
	return 0
}

// quitaVersion: el `-v4`/`-v13` que aapt2 añade al nombre de carpeta no es parte del calificador
// que se guarda en la tabla, pero sí del nombre de la ruta que va al pool de cadenas.
func quitaVersion(c string) string {
	if i := strings.LastIndex(c, "-v"); i > 0 {
		if _, err := strconv.Atoi(c[i+2:]); err == nil {
			return c[:i]
		}
	}
	return c
}

// anchoMinimoDe: `sw600dp` -> 600, el smallestScreenWidthDp que aapt2 escribe en +30 del
// ResTable_config (medido en §159).
func anchoMinimoDe(c string) uint16 {
	c = quitaVersion(c)
	if !strings.HasPrefix(c, "sw") || !strings.HasSuffix(c, "dp") {
		return 0
	}
	n, err := strconv.Atoi(c[2 : len(c)-2])
	if err != nil || n <= 0 || n > 65535 {
		return 0
	}
	return uint16(n)
}

// versionDe: el `-vN` de un calificador, que aapt2 guarda como sdkVersion en +24 del
// ResTable_config. Medido con aapt2 de build-tools 35.0.1: `v21` -> 21, `v33` -> 33, y `small-v21`
// -> 21. En un int de 16 bits cabe hasta v65535.
func versionDe(c string) uint16 {
	for _, parte := range strings.Split(c, "-") {
		if len(parte) < 2 || parte[0] != 'v' {
			continue
		}
		n, err := strconv.Atoi(parte[1:])
		if err == nil && n >= 0 && n <= 65535 {
			return uint16(n)
		}
	}
	return 0
}

// tamanoPantallaDe: `small`/`large`/`xlarge`, los valores del MASK_SCREENSIZE que aapt2 escribe en
// el byte bajo de screenLayout (+28 del ResTable_config). Medido con build-tools 35.0.1:
// small=1, large=3, xlarge=4; todos con sdkVersion 4 en +24.
func tamanoPantallaDe(c string) uint8 {
	for _, parte := range strings.Split(c, "-") {
		switch parte {
		case "small":
			return 1
		case "large":
			return 3
		case "xlarge":
			return 4
		}
	}
	return 0
}

// camposConfig: los campos del ResTable_config que este enlazador escribe, en el orden en que
// ResTable_config::compare los compara (locale, screenType, input, screenSize, version,
// screenLayout, screenLayout2, colorMode, uiMode, smallestScreenWidthDp, screenSizeDp). Los que no
// están medidos quedan a cero. `locale` es el u32 que resulta de los cuatro bytes language/country
// en little endian; `screenType` empaqueta orientation | touchscreen<<8 | density<<16 y `input`
// empaqueta keyboard | navigation<<8 | inputFlags<<16.
type camposConfig struct {
	locale        uint32
	screenType    uint32
	input         uint32
	screenSize    uint32
	version       uint32
	screenLayout  uint32
	uiMode        uint32
	smallestWidth uint32
	screenSizeDp  uint32
}

// uiModeDe: los valores del byte +29 medidos con aapt2 de build-tools 35.0.1: desk=2, car=3,
// television=4, appliance=5, watch=6 y el bit de night 0x20. Se combinan con OR, así car-night da
// 0x23, que es lo que escribe aapt2. La versión implícita de cualquier uiMode es 8 (FROYO).
func uiModeDe(c string) uint8 {
	var v uint8
	for _, p := range strings.Split(c, "-") {
		switch p {
		case "desk":
			v |= 0x02
		case "car":
			v |= 0x03
		case "television":
			v |= 0x04
		case "appliance":
			v |= 0x05
		case "watch":
			v |= 0x06
		case "night":
			v |= 0x20
		}
	}
	return v
}

// pantallaTactilDe: touchscreen en +13 del ResTable_config, medido con build-tools 35.0.1:
// notouch=1, stylus=2 y finger=3.
func pantallaTactilDe(c string) uint8 {
	for _, p := range strings.Split(c, "-") {
		switch p {
		case "notouch":
			return 1
		case "stylus":
			return 2
		case "finger":
			return 3
		}
	}
	return 0
}

// tecladoDe: keyboard en +16, medido: nokeys=1, qwerty=2 y 12key=3.
func tecladoDe(c string) uint8 {
	for _, p := range strings.Split(c, "-") {
		switch p {
		case "nokeys":
			return 1
		case "qwerty":
			return 2
		case "12key":
			return 3
		}
	}
	return 0
}

// navegacionDe: navigation en +17, medido: nonav=1, dpad=2, trackball=3 y wheel=4.
func navegacionDe(c string) uint8 {
	for _, p := range strings.Split(c, "-") {
		switch p {
		case "nonav":
			return 1
		case "dpad":
			return 2
		case "trackball":
			return 3
		case "wheel":
			return 4
		}
	}
	return 0
}

func esLengua(s string) bool {
	if len(s) != 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

func soloDigitos(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// camposDeConfig descompone un calificador compuesto (`small-v21`, `sw600dp-v13`, `es-rMX`, …) en
// los campos que escribe configDe. Las versiones implícitas que aapt2 aplica por compatibilidad
// (densidad y tamaño de pantalla desde la API 4, smallestScreenWidthDp desde la 13) se añaden cuando
// no hay un `-vN` explícito.
func camposDeConfig(c string) camposConfig {
	var f camposConfig
	partes := strings.Split(c, "-")
	for i, p := range partes {
		switch {
		case p == "port":
			f.screenType |= 1
		case p == "land":
			f.screenType |= 2
		case densidadDe(p) != 0:
			f.screenType |= uint32(densidadDe(p)) << 16
		case pantallaTactilDe(p) != 0:
			f.screenType |= uint32(pantallaTactilDe(p)) << 8
		case tecladoDe(p) != 0:
			f.input |= uint32(tecladoDe(p))
		case navegacionDe(p) != 0:
			f.input |= uint32(navegacionDe(p)) << 8
		case uiModeDe(p) != 0:
			f.uiMode = uint32(uiModeDe(c))
		case strings.HasPrefix(p, "sw") && anchoMinimoDe(p) != 0:
			f.smallestWidth = uint32(anchoMinimoDe(p))
		case tamanoPantallaDe(p) != 0:
			f.screenLayout = uint32(tamanoPantallaDe(p))
		case strings.HasPrefix(p, "v") && soloDigitos(p[1:]):
			n, err := strconv.Atoi(p[1:])
			if err == nil && n <= 65535 {
				f.version = uint32(n)
			}
		case esLengua(p):
			// locale es el u32 little endian de los cuatro bytes language[2] country[2], que es
			// lo que compara ResTable_config::compare (medido con es y es-rMX)
			f.locale = uint32(p[0]) | uint32(p[1])<<8
			if i+1 < len(partes) && len(partes[i+1]) == 3 &&
				(partes[i+1][0] == 'r' || partes[i+1][0] == 'R') {
				f.locale |= uint32(partes[i+1][1])<<16 | uint32(partes[i+1][2])<<24
			}
		}
	}
	if f.version == 0 {
		switch {
		case f.smallestWidth != 0:
			// sw/w/h dp nacieron en la API 13 (§159)
			f.version = 13
		case f.uiMode != 0:
			// medido con build-tools 35.0.1: car, desk, night, television, appliance y watch
			// escriben sdkVersion 8 en +24 y la bandera 0x1400 en el spec
			f.version = 8
		case f.screenLayout != 0, f.screenType&0xffff0000 != 0:
			// el tamaño de pantalla y la densidad nacieron en la API 4 (§137)
			f.version = 4
		}
	}
	return f
}

// configDe: los 64 bytes de ResTable_config que midió el aapt2 del aparato. La base solo declara su
// tamaño (64). `values-es` escribe 'e' y 's' en language[0] y language[1] (offset +8 y +9); el resto
// queda a cero. Medido en el volcado del parrafo 94 y ampliado con build-tools 35.0.1 para
// screenLayout (+28) y `-vN` (sdkVersion en +24).
func configDe(calificador string) []byte {
	b := make([]byte, 64)
	binary.LittleEndian.PutUint32(b, 64)
	partes := strings.Split(calificador, "-")
	for i, p := range partes {
		if esLengua(p) {
			b[8], b[9] = p[0], p[1]
			if i+1 < len(partes) && len(partes[i+1]) == 3 &&
				(partes[i+1][0] == 'r' || partes[i+1][0] == 'R') {
				b[10], b[11] = partes[i+1][1], partes[i+1][2]
			}
			break
		}
	}
	for _, p := range partes {
		switch p {
		case "port":
			// ResTable_config::orientation = ORIENTATION_PORT (1), en +12 (§137)
			b[12] = 1
		case "land":
			// ResTable_config::orientation = ORIENTATION_LAND (2), en +12
			b[12] = 2
		}
	}
	if d := anchoMinimoDe(calificador); d != 0 {
		// smallestScreenWidthDp en +30 (§159)
		binary.LittleEndian.PutUint16(b[30:], d)
	}
	if d := densidadDe(calificador); d != 0 {
		// density en +14 (§137)
		binary.LittleEndian.PutUint16(b[14:], d)
	}
	if t := tamanoPantallaDe(calificador); t != 0 {
		// screenLayout en +28: small=1, large=3, xlarge=4
		binary.LittleEndian.PutUint32(b[28:], uint32(t))
	}
	// touchscreen en +13, keyboard en +16 y navigation en +17: medidos con build-tools 35.0.1
	if t := pantallaTactilDe(calificador); t != 0 {
		b[13] = t
	}
	if t := tecladoDe(calificador); t != 0 {
		b[16] = t
	}
	if t := navegacionDe(calificador); t != 0 {
		b[17] = t
	}
	if m := uiModeDe(calificador); m != 0 {
		// uiMode en +29 (no en +36): medido byte a byte con aapt2 35.0.1
		b[29] = m
	}
	binary.LittleEndian.PutUint16(b[24:], uint16(camposDeConfig(calificador).version))
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
