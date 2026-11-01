package main

// El volcado del bloque de anotaciones, escrito dos veces sobre ficheros reales: primero sobre el de
// `d8` para medir el formato (parrafo 58 y 59) y despues sobre el nuestro, que es la prueba de que lo
// que escribimos decodifica igual que lo que escribe el oraculo. No es un lector del formato general:
// cubre lo que estas dos sondas nombran y se queja con nombre si aparece otra cosa.

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// valorLeido: un encoded_value reducido a texto, con los indices resueltos contra las tablas del
// propio fichero. Es la forma con la que se comparan los dos volcados.
func valorLeido(d *dexFile, off int) (string, int) {
	h := d.raw[off]
	tipo := h & 0x1f
	ancho := int(h>>5) + 1
	datos := off + 1
	switch tipo {
	case valueNull:
		return "null", datos
	case valueArray:
		n, p := uleb128En(d.raw, datos)
		out := "arreglo[" + fmt.Sprint(n) + "]{"
		for i := uint32(0); i < n; i++ {
			v, p2 := valorLeido(d, p)
			if i > 0 {
				out += ", "
			}
			out += v
			p = p2
		}
		return out + "}", p
	}
	var n int64
	for i := 0; i < ancho; i++ {
		n |= int64(d.raw[datos+i]) << (8 * uint(i))
	}
	fin := datos + ancho
	switch tipo {
	case valueInt:
		// el signo va en el bit alto del ultimo byte leido, que es como el formato lo escribe
		if d.raw[datos+ancho-1]&0x80 != 0 {
			n -= int64(1) << (8 * uint(ancho))
		}
		return fmt.Sprintf("int %d", n), fin
	case valueBool:
		return fmt.Sprintf("bool %d", n), fin
	case valueString:
		return "string " + fmt.Sprintf("%q", d.strings[n]), fin
	case valueType:
		return "type " + d.types[n], fin
	case valueMethod:
		m := d.methods[n]
		return "method " + m.owner + "." + m.name + descriptorDe(m.proto), fin
	}
	return fmt.Sprintf("tipo %#x sobre %d", tipo, n), fin
}

func descriptorDe(p dexProto) string {
	return "(" + strings.Join(p.params, "") + ")" + p.returnType
}

func uleb128En(raw []byte, off int) (uint32, int) {
	var v uint32
	var shift uint
	for {
		c := raw[off]
		off++
		v |= uint32(c&0x7f) << shift
		if c&0x80 == 0 {
			return v, off
		}
		shift += 7
	}
}

// itemLeido: un annotation_item decodificado, con sus bytes crudos para comparar el area byte a byte.
type itemLeido struct {
	crudo    []byte
	describe string
}

func itemEn(d *dexFile, off int) itemLeido {
	vis := d.raw[off]
	ti, p := uleb128En(d.raw, off+1)
	ne, p := uleb128En(d.raw, p)
	elementos := ""
	for e := 0; e < int(ne); e++ {
		nom, p2 := uleb128En(d.raw, p)
		p = p2
		v, p3 := valorLeido(d, p)
		p = p3
		elementos += fmt.Sprintf(" %s=%s;", d.strings[nom], v)
	}
	return itemLeido{crudo: d.raw[off:p], describe: fmt.Sprintf("vis %#x %s:%s", vis, d.types[ti], elementos)}
}

// volcadoAnotaciones: por cada clase, su set de clase y el de cada metodo anotado, en texto. El orden
// del area de items no entra en la comparacion: lo que se comprueba es el contenido, que los items
// identicos esten compartidos y que los bytes de cada item sean los mismos.
func volcadoAnotaciones(t *testing.T, d *dexFile) ([]string, []string) {
	t.Helper()
	baseCls := int(d.offDe(0x0006))
	if baseCls == 0 {
		t.Fatal("el mapa no declara class_defs")
	}
	lineas := []string{}
	items := []string{}
	set := func(off int) string {
		if off == 0 {
			return "sin set"
		}
		cnt := int(binary.LittleEndian.Uint32(d.raw[off:]))
		out := ""
		for k := 0; k < cnt; k++ {
			io := int(binary.LittleEndian.Uint32(d.raw[off+4+4*k:]))
			it := itemEn(d, io)
			items = append(items, fmt.Sprintf("%x", it.crudo))
			out += " [" + it.describe + "]"
		}
		return out
	}
	for i := range d.classes {
		nombre := d.classes[i].name
		aoff := int(binary.LittleEndian.Uint32(d.raw[baseCls+32*i+20:]))
		if aoff == 0 {
			lineas = append(lineas, nombre+": sin directorio")
			continue
		}
		lineas = append(lineas, nombre+": clase"+set(int(binary.LittleEndian.Uint32(d.raw[aoff:]))))
		nCampos := int(binary.LittleEndian.Uint32(d.raw[aoff+4:]))
		nMetodos := int(binary.LittleEndian.Uint32(d.raw[aoff+8:]))
		nEstaticos := int(binary.LittleEndian.Uint32(d.raw[aoff+12:]))
		if nCampos != 0 || nEstaticos != 0 {
			t.Errorf("%s: esta sonda no anota campos (%d y %d)", nombre, nCampos, nEstaticos)
		}
		// el indice de la lista es un u32 entero, medido: d8 escribe `06 00 00 00` antes del
		// desplazamiento del set, y un uleb dejaria el fichero desfasado dos bytes
		q := aoff + 16
		for j := 0; j < nMetodos; j++ {
			mi := int(binary.LittleEndian.Uint32(d.raw[q:]))
			so := int(binary.LittleEndian.Uint32(d.raw[q+4:]))
			q += 8
			m := d.methods[mi]
			lineas = append(lineas, fmt.Sprintf("%s %s%s:%s", nombre, m.name, descriptorDe(m.proto), set(so)))
		}
	}
	sort.Strings(lineas)
	sort.Strings(items)
	return lineas, items
}

// TestLasAnotacionesArmadasSonLasDeD8: el bloque nuestro decodifica igual que el de `d8` sobre las
// mismas seis clases, y los bytes de cada annotation_item son los mismos. Es la prueba del parrafo 59:
// las tablas de indices ya estaban medidas byte a byte (§22, §23), asi que aqui se ve que los indices
// que los items nombran tampoco se van por su cuenta.
func TestLasAnotacionesArmadasSonLasDeD8(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	b, err := td.armaDex(clases)
	if err != nil {
		t.Fatalf("assembling the dex: %v", err)
	}
	ours, err := readDex(b)
	if err != nil {
		t.Fatalf("the reader refuses what this fork wrote: %v", err)
	}
	theirs, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	mias, itemsMios := volcadoAnotaciones(t, ours)
	suyas, itemsSuyos := volcadoAnotaciones(t, theirs)
	if strings.Join(mias, "\n") != strings.Join(suyas, "\n") {
		t.Errorf("el volcado difiere:\n nuestro:\n%s\n de d8:\n%s", strings.Join(mias, "\n"),
			strings.Join(suyas, "\n"))
	}
	if strings.Join(itemsMios, " ") != strings.Join(itemsSuyos, " ") {
		t.Errorf("los bytes de los annotation_item difieren:\n nuestros: %s\n de d8: %s",
			strings.Join(itemsMios, " "), strings.Join(itemsSuyos, " "))
	}
	if len(distintos(itemsMios)) != 8 {
		t.Errorf("%d annotation_item distintos en el volcado, la cuenta que declara el mapa de d8 es 8",
			len(distintos(itemsMios)))
	}
}

// distintos: las lineas unicas, ordenadas. El area de items comparte los annotation_item identicos
// entre destinos, asi que la cuenta que importa es la de los distintos.
func distintos(lista []string) []string {
	visto := map[string]bool{}
	out := []string{}
	for _, s := range lista {
		if !visto[s] {
			visto[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// TestElLayoutVerificaConElBloqueDeAnotaciones: el verificador del aparato es el unico oraculo que
// mira estas areas, y ya tenemos el andamio del §55. Aqui se le da el fichero nuestro con el bloque
// puesto, y se le da el de d8: los dos tienen que pasar.
func TestElLayoutVerificaConElBloqueDeAnotaciones(t *testing.T) {
	td, clases := tablaDeLasSeis(t)
	b, err := td.armaDex(clases)
	if err != nil {
		t.Fatalf("assembling the dex: %v", err)
	}
	verificaLayout(t, b, "nuestro dex con anotaciones")
	verificaLayout(t, fixtureDex(t), "el dex de d8")
}
