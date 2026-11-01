package main

// El debug_info_item: las líneas del `.class` llevadas al dex, ordenadas por unidad dalvik.
//
// Medido con `dexdump -d` del aparato (Android 15, por `rish`) sobre una sonda propia (§56): tres
// métodos de un fichero con líneas conocidas, compilado por javac **sin `-g`** —que es la forma en
// que escribe este fork: solo `LineNumberTable`—. Lo que ART leyó, byte a byte contra lo que escribió
// `d8`:
//
//	0d 00 0e 39 4d 4b 00                 -> <init>: 0x0000 line=13, 0x0003 line=11,
//	                                        0x0007 line=14, 0x000b line=15
//	06 02 00 00 0e 1e 2d 00              -> suma:  0x0000 line=6, 0x0002 line=7, 0x0004 line=8
//	12 01 00 0e 4b 3c 2e 00              -> largo: 0x0000 line=18, 0x0004 line=19,
//	                                        0x0007 line=20, 0x0009 line=22
//
// De ahí salen las cuatro reglas de este fichero. (1) El item empieza por la **línea inicial**, no por
// el `header_addr` que cita la documentación del formato: `d8` lo salta y el parser de ART también.
// (2) Después va el recuento de argumentos **sin contar `this`** (`<init>()V` sale 0, `largo(String)`
// sale 1) y un byte por nombre: `d8` escribe **cero** cuando el `.class` no trae nombres, y no los
// toma de `MethodParameters` —medido: recompilada la sonda con `-parameters`, el área de debug sale
// **idéntica byte a byte** y lo único que cambia es que aparece una anotación
// `Ldalvik/annotation/MethodParameters;`—, así que los nombres con localizadores (`-g`) son un caso
// que aquí no se emite y no se finge. (3) Las posiciones van con el opcode especial
// `0x0a + 15·Δpc + (Δlínea + 4)`, cuyo rango es Δlínea en [-4, +10]; fuera de él se emite
// `ADVANCE_LINE` (`0x02`, sleb128) y `ADVANCE_PC` (`0x01`, uleb128) y **detrás** un especial (0,0),
// que es lo que marca la posición —los `ADVANCE` no marcan ninguna—. (4) Cierra `END_SEQUENCE`
// (`0x00`), y no hace falta `SET_FILE`: el nombre del fichero viene de
// `class_def_item.source_file_idx`.
//
// Los items **idénticos comparten offset**, como hace `d8`: en el fixture del estudio nueve métodos
// usan siete items (§20).

import "sort"

const (
	dbgEndSequence  = 0x00
	dbgAdvancePC    = 0x01
	dbgAdvanceLine  = 0x02
	dbgFirstSpecial = 0x0a
	dbgLineBase     = -4
	dbgLineRange    = 15
)

// itemDebug: los bytes del debug_info_item de un método, o nil si el `.class` no trajo líneas o si
// ninguna de sus pcs aterriza en el cuerpo traducido. `pcs` es la correspondencia JVM pc -> unidad
// dalvik que el traductor armó para resolver los saltos.
func (td *dexTables) itemDebug(m classMethod, pcs map[int]int) []byte {
	if len(m.Lines) == 0 || len(pcs) == 0 {
		return nil
	}
	type posicion struct{ pc, linea int }
	puestas := make([]posicion, 0, len(m.Lines))
	for _, l := range m.Lines {
		donde, ok := pcs[l.pc]
		if !ok {
			// una linea del .class en un pc que no es el arranque de ninguna instrucción nuestra:
			// se descarta, no se inventa una unidad
			continue
		}
		puestas = append(puestas, posicion{donde, l.line})
	}
	if len(puestas) == 0 {
		return nil
	}
	sort.SliceStable(puestas, func(i, j int) bool {
		if puestas[i].pc != puestas[j].pc {
			return puestas[i].pc < puestas[j].pc
		}
		return puestas[i].linea < puestas[j].linea
	})
	// dos filas del .class en la misma unidad dalvik (pasa cuando el cuerpo se comprimió): se queda
	// la primera, que es la que ART contaría desde el inicio de esa instrucción
	sinRepetidas := puestas[:1]
	for _, p := range puestas[1:] {
		if p.pc != sinRepetidas[len(sinRepetidas)-1].pc {
			sinRepetidas = append(sinRepetidas, p)
		}
	}
	puestas = sinRepetidas

	cuantos := m.ArgCount
	if m.Access&accStatic == 0 {
		cuantos-- // `this` no es un argumento a los efectos del item
	}
	out := putUleb(nil, uint32(puestas[0].linea))
	out = putUleb(out, uint32(cuantos))
	for i := 0; i < cuantos; i++ {
		out = append(out, 0)
	}
	pc, linea := 0, puestas[0].linea
	for _, p := range puestas {
		dp, dl := p.pc-pc, p.linea-linea
		if dl >= dbgLineBase && dl <= dbgLineBase+dbgLineRange-1 &&
			dp >= 0 && dp*dbgLineRange+(dl-dbgLineBase) <= 0xff-dbgFirstSpecial {
			out = append(out, byte(dbgFirstSpecial+dp*dbgLineRange+(dl-dbgLineBase)))
		} else {
			if dl != 0 {
				out = append(out, dbgAdvanceLine)
				out = putSleb(out, int32(dl))
			}
			if dp != 0 {
				out = append(out, dbgAdvancePC)
				out = putUleb(out, uint32(dp))
			}
			// el especial (0,0): la posicion se marca con un delta nulo
			out = append(out, dbgFirstSpecial-dbgLineBase)
		}
		pc, linea = p.pc, p.linea
	}
	return append(out, dbgEndSequence)
}

// areaDebug: los items de todos los métodos con cuerpo, deduplicados, con su posición relativa y
// cuantos distintos hay (el `map_list` declara ese recuento, no el de métodos).
func (td *dexTables) areaDebug(clases []classFull, pcs map[[2]string]map[int]int) ([]byte, map[[2]string]uint32, int) {
	buf := []byte{}
	rels := map[[2]string]uint32{}
	vistos := map[string]uint32{}
	cuantos := 0
	for _, cf := range clases {
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			item := td.itemDebug(m, pcs[[2]string{cf.Name, m.Name}])
			if len(item) == 0 {
				continue
			}
			clave := [2]string{cf.Name, m.Name}
			if r, ok := vistos[string(item)]; ok {
				rels[clave] = r
				continue
			}
			vistos[string(item)] = uint32(len(buf))
			rels[clave] = uint32(len(buf))
			cuantos++
			buf = append(buf, item...)
		}
	}
	return buf, rels, cuantos
}
