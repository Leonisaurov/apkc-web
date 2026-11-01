package main

// El verificador de layout, escrito en Go a partir de lo que el de ART rechazó en el aparato.
//
// Ni `d8` ni nuestro lector miran el map_list ni recorren las áreas de forma contigua: un dex con
// los ítems mal pegados o con los protos fuera de orden se releía bien aquí y salía del binario con
// `rc 0`, y el teléfono lo rechazaba despacio (`ClassNotFoundException` y, detrás, el aviso del
// verificador). Medido el 2026-11-01 con `/apex/com.android.art/bin/dexdump` en Android 15; el aviso
// que dio origen a cada bloque está escrito ahí.

import (
	"encoding/binary"
	"fmt"
	"testing"
)

type art struct{ b []byte }

func (a *art) u32(off int) uint32 { return binary.LittleEndian.Uint32(a.b[off:]) }
func (a *art) u16(off int) uint32 { return uint32(binary.LittleEndian.Uint16(a.b[off:])) }

func (a *art) uleb(off int) (uint32, int) {
	var v uint32
	var sh uint
	for {
		d := a.b[off]
		off++
		v |= uint32(d&0x7f) << sh
		if d&0x80 == 0 {
			return v, off
		}
		sh += 7
	}
}

func (a *art) cadena(sidx uint32) string {
	if int(sidx)*4+64 > len(a.b) {
		return "?"
	}
	st := int(a.u32(int(a.u32(60)) + int(sidx)*4))
	if st <= 0 || st >= len(a.b) {
		return "?"
	}
	lg, desde := a.uleb(st)
	if desde+int(lg) > len(a.b) {
		return "?"
	}
	return string(a.b[desde : desde+int(lg)])
}

func (a *art) descriptor(tidx uint32) string {
	if int(tidx)*4+72 > len(a.b) {
		return "?"
	}
	return a.cadena(a.u32(int(a.u32(68)) + int(tidx)*4))
}

type entradaMapa struct {
	kind, cuantos, off uint32
}

func (a *art) mapa() []entradaMapa {
	mo := int(a.u32(52))
	n, _ := a.uleb(mo)
	sal := make([]entradaMapa, 0, n)
	for i := uint32(0); i < n; i++ {
		p := mo + 4 + int(i)*12
		sal = append(sal, entradaMapa{a.u32(p) & 0xffff, a.u32(p + 4), a.u32(p + 8)})
	}
	return sal
}

// kinds que el formato define: hasta 0x2006. ART responde `Unknown map section type 2007` con
// cualquiera que no esté aquí (medido: nuestro encoded_array estaba declarado como 0x2007).
var kindsConocidos = map[uint32]bool{
	0x0000: true, 0x0001: true, 0x0002: true, 0x0003: true, 0x0004: true, 0x0005: true,
	0x0006: true, 0x0007: true, 0x0008: true,
	0x1000: true, 0x1001: true, 0x1002: true, 0x1003: true,
	0x2000: true, 0x2001: true, 0x2002: true, 0x2003: true, 0x2004: true, 0x2005: true,
	0x2006: true,
}

// verificaLayout: las cuatro comprobaciones que el verificador del aparato hace sobre el fichero y
// que aquí no hacía nadie. `deDonde` es el nombre con el que se señalan los fallos.
func verificaLayout(t *testing.T, b []byte, deDonde string) {
	t.Helper()
	a := &art{b: b}
	fallas := []string{}

	entradas := a.mapa()
	// 1. el map_list: kinds del formato y offsets ascendentes
	for i, e := range entradas {
		if !kindsConocidos[e.kind] {
			fallas = append(fallas, fmt.Sprintf("kind desconocido 0x%04x en la entrada %d del mapa", e.kind, i))
		}
		if i > 0 && e.off < entradas[i-1].off {
			fallas = append(fallas, fmt.Sprintf("el mapa no crece: 0x%04x en %d después de %d", e.kind, e.off, entradas[i-1].off))
		}
	}

	// 2. proto_ids con el tipo de retorno no decreciente: `Out-of-order proto_id return types`,
	// medido en el aparato con los protos ordenados por shorty, que dejaba el `V` de void en medio.
	cuantos, desde := int(a.u32(72)), int(a.u32(76))
	anterior := uint32(0)
	for i := 0; i < cuantos; i++ {
		ret := a.u32(desde + i*12 + 4)
		if i > 0 && ret < anterior {
			fallas = append(fallas, fmt.Sprintf("el proto %d devuelve %s, después de %s",
				i, a.descriptor(ret), a.descriptor(anterior)))
		}
		anterior = ret
	}

	// 3. el área de class_data, recorrida de forma contigua desde el offset que dice el mapa: así la
	// recorre ART, y por eso el relleno entre ítems desincronizaba la lectura entera
	// (`Bad index for class_data_item field_idx: 8 >= 6`, medido en el aparato). El encoded_field no
	// lleva el byte de valor: los ítems de d8 terminan justo donde empieza el siguiente.
	cuantosCampos, cuantosMetodos := a.u32(80), a.u32(88)
	for i, e := range entradas {
		if e.kind != 0x2000 {
			continue
		}
		fuera := len(b)
		for _, o := range entradas[i+1:] {
			if int(o.off) < fuera {
				fuera = int(o.off)
			}
		}
		at := int(e.off)
		for n := uint32(0); n < e.cuantos; n++ {
			var est, ins, dir, vir uint32
			est, at = a.uleb(at)
			ins, at = a.uleb(at)
			dir, at = a.uleb(at)
			vir, at = a.uleb(at)
			for _, lista := range [][2]uint32{{est, cuantosCampos}, {ins, cuantosCampos}} {
				prev := uint32(0)
				for k := uint32(0); k < lista[0]; k++ {
					var d uint32
					d, at = a.uleb(at)
					prev += d
					_, at = a.uleb(at)
					if prev >= lista[1] {
						fallas = append(fallas, fmt.Sprintf("class_data #%d: field_idx %d >= %d", n, prev, lista[1]))
					}
					if k > 0 && d == 0 {
						fallas = append(fallas, fmt.Sprintf("class_data #%d: dos campos con el mismo índice", n))
					}
				}
			}
			for _, lista := range [][2]uint32{{dir, cuantosMetodos}, {vir, cuantosMetodos}} {
				prev := uint32(0)
				for k := uint32(0); k < lista[0]; k++ {
					var d, co uint32
					d, at = a.uleb(at)
					prev += d
					_, at = a.uleb(at)
					co, at = a.uleb(at)
					if prev >= lista[1] {
						fallas = append(fallas, fmt.Sprintf("class_data #%d: method_idx %d >= %d", n, prev, lista[1]))
					}
					if co != 0 && (int(co) >= len(b) || co%4 != 0) {
						fallas = append(fallas, fmt.Sprintf("class_data #%d: code_off %d fuera del fichero o desalineado", n, co))
					}
					if k > 0 && d == 0 {
						fallas = append(fallas, fmt.Sprintf("class_data #%d: dos métodos con el mismo índice", n))
					}
				}
			}
			if at > fuera {
				fallas = append(fallas, fmt.Sprintf("el class_data #%d desborda su área: termina en %d, el área acaba en %d", n, at, fuera))
			}
		}
	}

	// 4. el encoded_value de cada campo estático tiene que ser del tipo del campo:
	// `unexpected static field initial value type: 'L' vs 'I'`, medido en el aparato sobre
	// `static int total = 5`, que en el .class no lleva ConstantValue y anunciábamos con 0x1e.
	nClases, fila := int(a.u32(96)), int(a.u32(100))
	for i := 0; i < nClases; i++ {
		cdo, sv := int(a.u32(fila+i*32+24)), int(a.u32(fila+i*32+28))
		if cdo == 0 || sv == 0 {
			continue
		}
		est, at := a.uleb(cdo)
		_, at = a.uleb(at) // ins
		_, at = a.uleb(at) // dir
		_, at = a.uleb(at) // vir
		nArray, desdeArray := a.uleb(sv)
		at = desdeArray
		prev := uint32(0)
		leídos := uint32(0)
		for k := uint32(0); k < est; k++ {
			var d uint32
			d, at = a.uleb(at)
			prev += d
			_, at = a.uleb(at)
			if leídos >= nArray {
				fallas = append(fallas, fmt.Sprintf("clase %d: el array de estáticos se acabó con %d campos por delante",
					i, est-leídos))
				break
			}
			vt, siguiente := a.uleb(at)
			ancho := anchoValor(byte(vt))
			if ancho < 0 {
				fallas = append(fallas, fmt.Sprintf("clase %d: encoded_value de tipo %d que nadie midió", i, vt&0x1f))
				break
			}
			at = siguiente + ancho
			leídos++
			desc := "?"
			if int(prev) < int(a.u32(80)) {
				desc = a.descriptor(a.u16(int(a.u32(84)) + int(prev)*8 + 2))
			}
			if len(desc) > 0 && desc[0] == 'I' && int(vt&0x1f) != valueInt {
				fallas = append(fallas, fmt.Sprintf("%s: un campo int se anuncia con el tipo %d (%02x), y ART quiere %d",
					a.descriptor(a.u32(fila+i*32)), vt&0x1f, byte(vt), valueInt))
			}
		}
	}

	for _, f := range fallas {
		t.Errorf("%s: %s", deDonde, f)
	}
}

func TestElLayoutDelDexArmadoPasaElVerificadorDeART(t *testing.T) {
	b, _ := armaElFixture(t)
	verificaLayout(t, b, "nuestro dex del fixture")
}

func TestElLayoutDelDexDeD8PasaElVerificadorDeART(t *testing.T) {
	// el oráculo manda: si la prueba no pasa con el fichero que escribió d8, la prueba está mal
	verificaLayout(t, fixtureDex(t), "el dex de d8")
}

func TestElLayoutDelContadorPasaElVerificadorDeART(t *testing.T) {
	// la clase que ART rechazaba: `static int total = 5`, sin ConstantValue en el .class
	c := claseDelFuente(t, "Contador.java", fuenteContador, "com/myapp/Contador")
	td := newDexTables()
	if err := td.collectClasses([]classFull{c}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	b, err := td.armaDex([]classFull{c})
	if err != nil {
		t.Fatalf("armado: %v", err)
	}
	verificaLayout(t, b, "Contador con un estático sin constante")
}
