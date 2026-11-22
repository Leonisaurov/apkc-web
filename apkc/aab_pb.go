package main

// Escritor mínimo del wire format de protobuf que usan los dos mensajes del .aab:
// com.android.aapt.Resources.ResourceTable (resources.pb) y
// com.android.aapt.Resources.XmlNode (manifest/AndroidManifest.xml y los XML de res/).
// Los números de campo y los tipos están tomados de Resources.proto y Configuration.proto
// tal como vienen en bundletool 1.18.3 (medido en §127: el .aab de referencia se valida con rc 0).
//
// No se intenta reproducir byte a byte lo que escribe aapt2: el oráculo es bundletool validate,
// que exige que cada fichero bajo res/ esté referenciado desde la tabla y que el manifiesto sea
// un XmlNode con su elemento raíz. Los campos que no aportan información al enlazador Go
// (source_pool, tool_fingerprint, posiciones columna) se omiten o se dejan en cero.

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// --- wire format ---

func pbVarint(n uint64) []byte {
	var b [10]byte
	i := 0
	for n >= 0x80 {
		b[i] = byte(n) | 0x80
		n >>= 7
		i++
	}
	b[i] = byte(n)
	return append([]byte(nil), b[:i+1]...)
}

func pbClave(campo int, tipo byte) []byte {
	return pbVarint(uint64(campo)<<3 | uint64(tipo))
}

func pbU32(campo int, v uint32) []byte {
	return append(pbClave(campo, 0), pbVarint(uint64(v))...)
}

func pbBool(campo int, v bool) []byte {
	if v {
		return pbU32(campo, 1)
	}
	return pbU32(campo, 0)
}

func pbBytes(campo int, v []byte) []byte {
	out := pbClave(campo, 2)
	out = pbVarintAppend(out, uint64(len(v)))
	return append(out, v...)
}

func pbVarintAppend(dst []byte, n uint64) []byte {
	return append(dst, pbVarint(n)...)
}

func pbCadena(campo int, s string) []byte {
	return pbBytes(campo, []byte(s))
}

func pbMensaje(campo int, piezas ...[]byte) []byte {
	n := 0
	for _, p := range piezas {
		n += len(p)
	}
	cuerpo := make([]byte, 0, n)
	for _, p := range piezas {
		cuerpo = append(cuerpo, p...)
	}
	return pbBytes(campo, cuerpo)
}

// int32 en protobuf se codifica como varint de 64 bits en complemento a dos cuando es negativo.
func pbEntero(campo int, v int32) []byte {
	return pbU32(campo, uint32(v))
}

func pbFlotante(campo int, v float32) []byte {
	out := pbClave(campo, 5)
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	return append(out, b[:]...)
}

func pbUnir(piezas ...[]byte) []byte {
	n := 0
	for _, p := range piezas {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range piezas {
		out = append(out, p...)
	}
	return out
}

// --- ResourceTable ---

// idDeRecurso compone el id de 32 bits con la forma 0xPPTTEEEE que usa el R.java del enlazador.
func idDeRecurso(paquete, tipo uint32, entrada int) uint32 {
	return paquete<<24 | tipo<<16 | uint32(entrada)
}

// itemDeValor convierte un Res_value (tipo y datos, más su texto cuando es cadena) en el Item
// del proto. Los tipos numericos son los medidos en el AXML/AARSC del proyecto del studio:
// 0x01 referencia, 0x03 cadena, 0x10 entero decimal, 0x11 hex, 0x12 booleano, 0x1c color,
// 0x05 dimension, 0x04 float, 0x06 fraccion y 0x00 vacío.
func itemDeValor(tipo byte, datos uint32, texto string, nombres map[uint32]string) []byte {
	switch tipo {
	case 0x01: // TYPE_REFERENCE
		ref := pbU32(2, datos)
		if nombre, ok := nombres[datos]; ok {
			ref = pbUnir(ref, pbCadena(3, nombre))
		}
		return pbMensaje(1, ref) // Item.ref
	case 0x02: // TYPE_ATTRIBUTE (?)
		ref := pbUnir(pbU32(1, 1), pbU32(2, datos)) // Reference.type = ATTRIBUTE
		if nombre, ok := nombres[datos]; ok {
			ref = pbUnir(ref, pbCadena(3, nombre))
		}
		return pbMensaje(1, ref)
	case 0x03: // TYPE_STRING
		return pbMensaje(2, pbCadena(1, texto)) // Item.str
	case 0x04: // TYPE_FLOAT
		return pbMensaje(7, pbFlotante(3, math.Float32frombits(datos))) // Item.prim.float_value
	case 0x05: // TYPE_DIMENSION
		return pbMensaje(7, pbU32(13, datos)) // Primitive.dimension_value
	case 0x06: // TYPE_FRACTION
		return pbMensaje(7, pbU32(14, datos)) // Primitive.fraction_value
	case 0x10: // TYPE_INT_DEC
		return pbMensaje(7, pbEntero(6, int32(datos))) // Primitive.int_decimal_value
	case 0x11: // TYPE_INT_HEX
		return pbMensaje(7, pbU32(7, datos)) // Primitive.int_hexadecimal_value
	case 0x12: // TYPE_INT_BOOLEAN
		return pbMensaje(7, pbBool(8, datos != 0)) // Primitive.boolean_value
	case 0x1c: // TYPE_INT_COLOR_ARGB8
		return pbMensaje(7, pbU32(9, datos))
	case 0x1d: // TYPE_INT_COLOR_RGB8
		return pbMensaje(7, pbU32(10, datos))
	case 0x1e: // TYPE_INT_COLOR_ARGB4
		return pbMensaje(7, pbU32(11, datos))
	case 0x1f: // TYPE_INT_COLOR_RGB4
		return pbMensaje(7, pbU32(12, datos))
	case 0x00: // TYPE_NULL
		return pbMensaje(7, pbMensaje(1)) // Primitive.null_value
	default:
		// Un tipo no medido no se inventa: se deja como entero hexadecimal, que es
		// una representación válida del mismo dato y no cambia el id del recurso.
		return pbMensaje(7, pbU32(7, datos))
	}
}

// textoDeValor recupera la cadena que aapt2 pone en XmlAttribute.value cuando el rawValue del
// AXML es -1 (tipos numericos). El manifiesto del .aab la lleva aunque el compiled_item ya
// tenga el dato tipado, y bundletool la lee para el package, la version y el moduleType.
func textoDeValor(tipo byte, datos uint32, texto string, nombres map[uint32]string) string {
	if tipo == 0x03 {
		return texto
	}
	switch tipo {
	case 0x01, 0x02:
		if nombre, ok := nombres[datos]; ok {
			if tipo == 0x02 {
				return "?" + nombre
			}
			return "@" + nombre
		}
		return fmt.Sprintf("@0x%08x", datos)
	case 0x10:
		return strconv.FormatInt(int64(int32(datos)), 10)
	case 0x11:
		return "0x" + strconv.FormatUint(uint64(datos), 16)
	case 0x12:
		if datos != 0 {
			return "true"
		}
		return "false"
	case 0x04:
		return strconv.FormatFloat(float64(math.Float32frombits(datos)), 'g', -1, 32)
	default:
		if texto != "" {
			return texto
		}
		return strconv.FormatUint(uint64(datos), 10)
	}
}

// fuentePB es el SourcePosition de un nodo o atributo. La columna no está en el AXML que escribe
// este fork, así que solo se pasa la línea, medida y comprobada en axmlwriter_test.go.
func fuentePB(linea int) []byte {
	if linea <= 0 {
		return nil
	}
	return pbMensaje(3, pbU32(1, uint32(linea)))
}

// campoFuentePB construye directamente un SourcePosition para el campo que se pida.
func campoFuentePB(campo, linea int) []byte {
	if linea <= 0 {
		return nil
	}
	return pbMensaje(campo, pbU32(1, uint32(linea)))
}

// configPB convierte los 64 bytes de un ResTable_config en el Configuration del proto de
// aapt2. Solo se traducen los ejes que este enlazador escribe (§137, §159): idioma,
// orientación, densidad, sdk, tamaño de pantalla y smallestScreenWidthDp.
func configPB(config []byte) []byte {
	if len(config) < 32 || config[0] == 0 {
		return nil
	}
	var piezas [][]byte
	lang := strings.TrimRight(string(config[8:10]), "\x00")
	pais := strings.TrimRight(string(config[10:12]), "\x00")
	if len(lang) == 2 {
		locale := lang
		if len(pais) == 2 {
			locale += "-" + pais
		}
		piezas = append(piezas, pbCadena(3, locale))
	}
	if o := config[12]; o == 1 || o == 2 {
		piezas = append(piezas, pbU32(15, uint32(o)))
	}
	if d := binary.LittleEndian.Uint16(config[14:]); d != 0 {
		piezas = append(piezas, pbU32(18, uint32(d)))
	}
	if v := binary.LittleEndian.Uint16(config[24:]); v != 0 {
		piezas = append(piezas, pbU32(24, uint32(v)))
	}
	if sl := binary.LittleEndian.Uint32(config[28:]); sl&0x0f != 0 {
		piezas = append(piezas, pbU32(10, sl&0x0f))
	}
	if sw := binary.LittleEndian.Uint16(config[30:]); sw != 0 {
		piezas = append(piezas, pbU32(9, uint32(sw)))
	}
	if len(piezas) == 0 {
		return nil
	}
	return pbUnir(piezas...)
}

// fileReferencePB es el Item.file que apunta a un fichero dentro del bundle. El tipo 3 es
// PROTO_XML: los XML de res/ que produce el enlazador Go se convierten a proto antes de entrar.
func fileReferencePB(ruta string) []byte {
	tipo := uint32(2) // BINARY_XML, para .xml sin convertir y ficheros opacos
	switch {
	case strings.HasSuffix(ruta, ".png"), strings.HasSuffix(ruta, ".webp"):
		tipo = 1 // PNG
	case strings.HasSuffix(ruta, ".xml"):
		tipo = 3 // PROTO_XML
	default:
		tipo = 0 // UNKNOWN: raw, sonido y demás ficheros opacos
	}
	return pbMensaje(5, pbUnir(pbCadena(1, ruta), pbU32(2, tipo))) // Item.file
}

// configValuePB envuelve un Item en Value/ConfigValue. El campo 6 es Value.item.
func configValuePB(config, item []byte) []byte {
	value := pbMensaje(4, item)
	cv := []byte{}
	if len(config) > 0 {
		cv = pbMensaje(1, config)
	}
	return pbUnir(cv, pbMensaje(2, value))
}
