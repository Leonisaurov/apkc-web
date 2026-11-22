package main

// Escritor de ClassFile reducido. Copia la forma del pool y de los atributos que
// usa `apkc/javaemisor.go` (JVMS 4.1-4.7), pero no emite Code en los metodos:
// un stub de compilacion solo tiene que presentar nombres, descriptores y
// constantes. Los constructores llevan un Code minimo porque el compilador de
// `apkc` resuelve el `<init>` del padre al escribir el constructor implicito.

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

type poolBuilder struct {
	entradas [][]byte
	indices  map[string]int
}

func nuevoPool() *poolBuilder { return &poolBuilder{indices: map[string]int{}} }

func (p *poolBuilder) agregar(clave string, e []byte) int {
	if i, ok := p.indices[clave]; ok {
		return i
	}
	p.entradas = append(p.entradas, e)
	i := len(p.entradas)
	p.indices[clave] = i
	return i
}

func (p *poolBuilder) utf8(s string) int {
	b := []byte{1, byte(len(s) >> 8), byte(len(s))}
	return p.agregar("u\x00"+s, append(b, []byte(s)...))
}

func (p *poolBuilder) entero(v int32) int {
	b := []byte{3, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[1:], uint32(v))
	return p.agregar(fmt.Sprintf("i\x00%d", v), b)
}

func (p *poolBuilder) largo(v int64) int {
	b := []byte{5, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint64(b[1:], uint64(v))
	return p.agregarDos(fmt.Sprintf("j\x00%d", v), b)
}

func (p *poolBuilder) flotante(v float64) int {
	b := []byte{4, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[1:], math.Float32bits(float32(v)))
	return p.agregar(fmt.Sprintf("f\x00%v", v), b)
}

func (p *poolBuilder) doble(v float64) int {
	b := []byte{6, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(v))
	return p.agregarDos(fmt.Sprintf("d\x00%v", v), b)
}

func (p *poolBuilder) agregarDos(clave string, e []byte) int {
	if i, ok := p.indices[clave]; ok {
		return i
	}
	p.entradas = append(p.entradas, e, nil)
	i := len(p.entradas) - 1
	p.indices[clave] = i
	return i
}

func (p *poolBuilder) cadena(texto string) int {
	u := p.utf8(texto)
	b := []byte{8, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(u))
	return p.agregar("s\x00"+texto, b)
}

func (p *poolBuilder) clase(nombre string) int {
	u := p.utf8(nombre)
	b := []byte{7, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(u))
	return p.agregar("c\x00"+nombre, b)
}

func (p *poolBuilder) nombreYTipo(nombre, desc string) int {
	ni := p.utf8(nombre)
	di := p.utf8(desc)
	b := []byte{12, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(ni))
	binary.BigEndian.PutUint16(b[3:], uint16(di))
	return p.agregar("nt\x00"+nombre+"|"+desc, b)
}

func (p *poolBuilder) metodoRef(clase, nombre, desc string) int {
	ci := p.clase(clase)
	nt := p.nombreYTipo(nombre, desc)
	b := []byte{10, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(ci))
	binary.BigEndian.PutUint16(b[3:], uint16(nt))
	return p.agregar("m\x00"+clase+"|"+nombre+"|"+desc, b)
}

func (p *poolBuilder) serializar() []byte {
	total := len(p.entradas) + 1
	out := u2be(uint16(total))
	for _, e := range p.entradas {
		out = append(out, e...)
	}
	return out
}

func u2be(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }
func u4be(v uint32) []byte { return append(u2be(uint16(v>>16)), u2be(uint16(v))...) }

func attr(pool *poolBuilder, nombre string, cuerpo []byte) []byte {
	return append(append(u2be(uint16(pool.utf8(nombre))), u4be(uint32(len(cuerpo)))...), cuerpo...)
}

func claseDeStub(c *apiClase, todas map[string]*apiClase) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("clase vacia")
	}
	p := nuevoPool()
	this := p.clase(c.interno)
	super := 0
	sup := c.super
	if sup == "" && c.interno != "java/lang/Object" {
		// JVMS 4.1: hasta las interfaces llevan java/lang/Object como super.
		sup = "java/lang/Object"
	}
	if sup != "" {
		super = p.clase(sup)
	}
	ifaces := []int{}
	for _, in := range c.interfaces {
		ifaces = append(ifaces, p.clase(in))
	}

	campos := []byte{}
	nCampos := 0
	for _, f := range ordenaCampos(c.campos) {
		acc := int(f.access)
		if acc == 0 {
			acc = 0x0001
		}
		if c.esEnum && f.tieneVal == false && f.desc == "L"+c.interno+";" {
			acc |= 0x4000 // ACC_ENUM
		}
		campos = append(campos, u2be(uint16(acc))...)
		campos = append(campos, u2be(uint16(p.utf8(f.nombre)))...)
		campos = append(campos, u2be(uint16(p.utf8(f.desc)))...)
		if f.tieneVal {
			idx, ok := indiceConstante(p, f.valor, f.desc)
			if ok {
				campos = append(campos, u2be(1)...)
				campos = append(campos, attr(p, "ConstantValue", u2be(uint16(idx)))...)
			} else {
				campos = append(campos, u2be(0)...)
			}
		} else {
			campos = append(campos, u2be(0)...)
		}
		nCampos++
	}

	// constructores: los declarados y, si no hay uno sin parametros, uno
	// sintetico. `apkc` resuelve el `<init>()V` del padre al compilar la clase
	// del usuario; sin el, el constructor implicito no encuentra a quien llamar.
	metodos := []*apiMetodo{}
	tieneVacio := false
	for _, m := range c.metodos {
		if m.nombre == "<init>" && m.desc == "()V" {
			tieneVacio = true
		}
	}
	if !c.esInterfaz && !tieneVacio {
		metodos = append(metodos, &apiMetodo{nombre: "<init>", desc: "()V", access: 0x0001})
	}
	metodos = append(metodos, c.metodos...)
	vistos := map[string]bool{}
	cuerpos := []byte{}
	nMetodos := 0
	for _, m := range metodos {
		k := m.nombre + m.desc
		if vistos[k] {
			continue
		}
		vistos[k] = true
		acc := int(m.access)
		if acc == 0 {
			acc = 0x0001
		}
		if m.nombre == "<init>" {
			// un constructor no puede ser native; lleva Code minimo.
			acc &^= 0x0100
			cuerpos = append(cuerpos, metodoConInit(p, c, m, acc)...)
			nMetodos++
			continue
		}
		if c.esInterfaz {
			acc |= 0x0400
		} else if acc&0x0400 == 0 {
			acc |= 0x0100 // ACC_NATIVE: sin Code, pero valido
		}
		cuerpos = append(cuerpos, u2be(uint16(acc))...)
		cuerpos = append(cuerpos, u2be(uint16(p.utf8(m.nombre)))...)
		cuerpos = append(cuerpos, u2be(uint16(p.utf8(m.desc)))...)
		cuerpos = append(cuerpos, u2be(0)...)
		nMetodos++
	}

	out := []byte{0xca, 0xfe, 0xba, 0xbe}
	out = append(out, u2be(0)...)
	out = append(out, u2be(52)...)
	out = append(out, p.serializar()...)
	out = append(out, u2be(c.access)...)
	out = append(out, u2be(uint16(this))...)
	out = append(out, u2be(uint16(super))...)
	out = append(out, u2be(uint16(len(ifaces)))...)
	for _, i := range ifaces {
		out = append(out, u2be(uint16(i))...)
	}
	out = append(out, u2be(uint16(nCampos))...)
	out = append(out, campos...)
	out = append(out, u2be(uint16(nMetodos))...)
	out = append(out, cuerpos...)
	out = append(out, u2be(0)...) // atributos de clase
	return out, nil
}

func metodoConInit(p *poolBuilder, c *apiClase, m *apiMetodo, acc int) []byte {
	super := c.super
	if super == "" {
		super = "java/lang/Object"
	}
	ref := p.metodoRef(super, "<init>", "()V")
	// aload_0; invokespecial super.<init>()V; return
	code := []byte{0x2a, 0xb7, byte(ref >> 8), byte(ref), 0xb1}
	cuerpo := u2be(2) // max_stack
	cuerpo = append(cuerpo, u2be(uint16(maxLocales(m.desc)))...)
	cuerpo = append(cuerpo, u4be(uint32(len(code)))...)
	cuerpo = append(cuerpo, code...)
	cuerpo = append(cuerpo, u2be(0)...) // exception_table
	cuerpo = append(cuerpo, u2be(0)...) // atributos de Code
	b := u2be(uint16(acc))
	b = append(b, u2be(uint16(p.utf8(m.nombre)))...)
	b = append(b, u2be(uint16(p.utf8(m.desc)))...)
	b = append(b, u2be(1)...)
	return append(b, attr(p, "Code", cuerpo)...)
}

func maxLocales(desc string) int {
	n := 1
	dentro := false
	for i := 0; i < len(desc); i++ {
		switch desc[i] {
		case '(':
			dentro = true
		case ')':
			dentro = false
		case 'J', 'D':
			if dentro {
				n += 2
			}
		default:
			if dentro {
				n++
			}
		}
	}
	// el bucle cuenta tambien la letra `L` y el `;`; se corrige contando
	// parametros de verdad: es mas simple volver a recorrer el descriptor.
	return 1 + cuentaSlots(desc)
}

func cuentaSlots(desc string) int {
	abre := -1
	for i := 0; i < len(desc); i++ {
		if desc[i] == '(' {
			abre = i
			break
		}
	}
	if abre < 0 {
		return 0
	}
	n := 0
	for i := abre + 1; i < len(desc) && desc[i] != ')'; {
		switch desc[i] {
		case '[':
			for i < len(desc) && desc[i] == '[' {
				i++
			}
			if i < len(desc) && desc[i] == 'L' {
				for i < len(desc) && desc[i] != ';' {
					i++
				}
			}
			i++
			n++
		case 'L':
			for i < len(desc) && desc[i] != ';' {
				i++
			}
			i++
			n++
		case 'J', 'D':
			i++
			n += 2
		default:
			i++
			n++
		}
	}
	return n
}

func indiceConstante(p *poolBuilder, v apiValor, desc string) (int, bool) {
	switch v.tag {
	case 3:
		return p.entero(v.i), true
	case 4:
		return p.flotante(v.f), true
	case 5:
		return p.largo(v.j), true
	case 6:
		return p.doble(v.f), true
	case 8:
		return p.cadena(v.texto), true
	}
	return 0, false
}

func ordenaCampos(campos []*apiCampo) []*apiCampo {
	out := append([]*apiCampo{}, campos...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].access&0x0010 != 0 && out[j].access&0x0010 == 0 {
			return true
		}
		if out[i].access&0x0010 == 0 && out[j].access&0x0010 != 0 {
			return false
		}
		return out[i].nombre < out[j].nombre
	})
	return out
}
