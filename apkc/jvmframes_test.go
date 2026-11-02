package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// El StackMapTable, medido contra javac con dos formas que comprimen distinto (parrafo 65): javac
// escribe `same`/`append`/`full` segun lo que cambie y este emisor escribe siempre `full_frame`, asi
// que la comparacion no es de bytes sino de lo que cada atributo DICE: en que pc hay marco y con que
// locales y pila. Lo que se exige es que, expandidos los dos, coincidan.

// fuenteCab es la forma que obliga a mirar el pc 0: el bucle empieza en la primera instruccion, y en
// `f` hay un local declarado dentro del cuerpo, asi que la vuelta llega con un entero al hueco que la
// entrada ve sin asignar. javac escribe ahi `same` y la JVM lo verifica: `top` gana en la union.
const fuenteCab = `public class Cab {
    static int f(int n) {
        while (n > 0) {
            int x = 1;
            n = n - x;
        }
        return n;
    }
    static int g(int n) {
        int t = 0;
        while (n > 0) {
            int x = 2;
            t = t + x;
            n = n - 1;
        }
        return t;
    }
}
`

// cabJavac: las clases de javac 21 para fuenteCab, sacadas con `javac -g` (el mismo binario que los
// demas fixtures) y `base64 -w0 Cab.class`.
const cabJavac = "yv66vgAAAEEAGAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQADQ2FiAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAEkxvY2FsVmFyaWFibGVUYWJsZQEABHRoaXMBAAVMQ2FiOwEAAWYBAAQoSSlJAQABeAEAAUkBAAFuAQANU3RhY2tNYXBUYWJsZQEAAWcBAAF0AQAKU291cmNlRmlsZQEACENhYi5qYXZhACEABwACAAAAAAADAAEABQAGAAEACQAAAC8AAQABAAAABSq3AAGxAAAAAgAKAAAABgABAAAAAQALAAAADAABAAAABQAMAA0AAAAIAA4ADwABAAkAAABdAAIAAgAAAA8angAMBDwaG2Q7p//2GqwAAAADAAoAAAAWAAUAAAADAAQABAAGAAUACgAGAA0ABwALAAAAFgACAAYABAAQABEAAQAAAA8AEgARAAAAEwAAAAQAAgAMAAgAFAAPAAEACQAAAHgAAgADAAAAFQM8Gp4AEAU9GxxgPBoEZDun//IbrAAAAAMACgAAAB4ABwAAAAoAAgALAAYADAAIAA0ADAAOABAADwATABAACwAAACAAAwAIAAgAEAARAAIAAAAVABIAEQAAAAIAEwAVABEAAQATAAAABwAC/AACARAAAQAWAAAAAgAX"

// marcosMetodo es lo que el test compara: los marcos ya expandidos y el alto de locales.
type marcosMetodo struct {
	maxLoc   int
	marcos   map[int]estadoFrame
	hayTabla bool
}

// marcosDeClase saca el StackMapTable del metodo pedido. Recorre el fichero con el mismo `cursor` y
// el mismo `pool` que usa el dexer, para no tener un segundo lector que se desincronice.
func marcosDeClase(datos []byte, metodo string) (marcosMetodo, error) {
	c := &cursor{b: datos}
	if c.u4() != classMagic {
		return marcosMetodo{}, fmt.Errorf("no es una clase")
	}
	c.u2()
	c.u2()
	p, err := c.constantPool()
	if err != nil {
		return marcosMetodo{}, err
	}
	c.u2()
	propia := p.classOf(c.u2())
	c.u2()
	c.u2s()
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		c.u2()
		c.u2()
		c.u2()
		leerAtributos(c, p)
	}
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		acc := c.u2()
		nombre := p.text(c.u2())
		desc := p.text(c.u2())
		attrs := leerAtributos(c, p)
		if nombre != metodo {
			continue
		}
		codigo, ok := attrs["Code"]
		if !ok {
			return marcosMetodo{}, fmt.Errorf("%s no lleva Code", metodo)
		}
		cc := &cursor{b: codigo}
		cc.u2()
		maxLoc := cc.u2()
		cc.bytes(int(cc.u4()))
		for j, m := 0, cc.u2(); j < m && !cc.failed(); j++ {
			cc.bytes(8)
		}
		internos := leerAtributos(cc, p)
		if cc.failed() {
			return marcosMetodo{}, fmt.Errorf("%s: %s", metodo, cc.reason())
		}
		tabla, hay := internos["StackMapTable"]
		out := marcosMetodo{maxLoc: maxLoc, hayTabla: hay}
		if !hay {
			return out, nil
		}
		out.marcos, err = decodificaMarcos(tabla, p, desc, acc&accStatic != 0, propia, maxLoc)
		return out, err
	}
	return marcosMetodo{}, fmt.Errorf("el metodo %s no esta en la clase", metodo)
}

func leerAtributos(c *cursor, p *pool) map[string][]byte {
	out := map[string][]byte{}
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		nombre := p.text(c.u2())
		out[nombre] = c.bytes(int(c.u4()))
	}
	return out
}

// decodificaMarcos expande las seis formas de frame del JVMS a un estado completo por pc.
func decodificaMarcos(cuerpo []byte, p *pool, desc string, estatico bool, propia string, maxLoc int) (map[int]estadoFrame, error) {
	c := &cursor{b: cuerpo}
	args, err := tiposDeArgs(desc)
	if err != nil {
		return nil, err
	}
	locales := []tipoVerif{}
	if !estatico {
		locales = append(locales, verifRef(propia))
	}
	locales = append(locales, args...)
	out := map[int]estadoFrame{}
	previo := -1
	n := c.u2()
	for i := 0; i < n && !c.failed(); i++ {
		ft := c.u1()
		delta := 0
		var pila []tipoVerif
		switch {
		case ft <= 63:
			delta = ft
		case ft <= 127:
			delta = ft - 64
			t, err := leeTipoDeMarco(c, p)
			if err != nil {
				return nil, err
			}
			pila = []tipoVerif{t}
		case ft == 247:
			delta = c.u2()
			t, err := leeTipoDeMarco(c, p)
			if err != nil {
				return nil, err
			}
			pila = []tipoVerif{t}
		case ft >= 248 && ft <= 250:
			delta = c.u2()
			corte := 251 - ft
			if corte > len(locales) {
				return nil, fmt.Errorf("un `chop` de %d locales con %d", corte, len(locales))
			}
			locales = locales[:len(locales)-corte]
		case ft == 251:
			delta = c.u2()
		case ft >= 252 && ft <= 254:
			delta = c.u2()
			for k := 0; k < ft-251; k++ {
				t, err := leeTipoDeMarco(c, p)
				if err != nil {
					return nil, err
				}
				locales = append(locales, t)
			}
		case ft == 255:
			delta = c.u2()
			nl := c.u2()
			locales = make([]tipoVerif, nl)
			for k := range locales {
				t, err := leeTipoDeMarco(c, p)
				if err != nil {
					return nil, err
				}
				locales[k] = t
			}
			ns := c.u2()
			for k := 0; k < ns; k++ {
				t, err := leeTipoDeMarco(c, p)
				if err != nil {
					return nil, err
				}
				pila = append(pila, t)
			}
		default:
			return nil, fmt.Errorf("el frame_type %d no esta medido", ft)
		}
		pc := delta
		if previo >= 0 {
			pc = previo + delta + 1
		}
		previo = pc
		out[pc] = estadoFrame{
			locales: rellenaTop(locales, maxLoc),
			pila:    pila,
		}
	}
	if c.failed() {
		return nil, fmt.Errorf("la tabla se corta: %s", c.reason())
	}
	return out, nil
}

func leeTipoDeMarco(c *cursor, p *pool) (tipoVerif, error) {
	switch tag := c.u1(); tag {
	case 0:
		return verifTop, nil
	case 1:
		return verifInt, nil
	case 5:
		return verifNulo, nil
	case 7:
		return verifRef(p.classOf(c.u2())), nil
	case 6:
		return tipoVerif{etiqueta: 'u', clase: "this"}, nil
	case 8:
		return tipoVerif{etiqueta: 'u', clase: fmt.Sprintf("%d", c.u2())}, nil
	default:
		return tipoVerif{}, fmt.Errorf("el verification_type_info %d no esta medido", tag)
	}
}

func rellenaTop(locales []tipoVerif, n int) []tipoVerif {
	out := append([]tipoVerif(nil), locales...)
	for len(out) < n {
		out = append(out, verifTop)
	}
	return out
}

func describeMarco(e estadoFrame) string {
	ts := func(v []tipoVerif) string {
		s := make([]string, len(v))
		for i, t := range v {
			s[i] = t.String()
		}
		return "[" + strings.Join(s, ", ") + "]"
	}
	return "locales " + ts(e.locales) + ", pila " + ts(e.pila)
}

func igualesTipos(a, b []tipoVerif) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestLosMarcosSonLosDeJavac: los marcos de este emisor, expandidos, dicen lo mismo que los de javac
// en las mismas posiciones. Cubre el if/else (un marco por destino y otro tras el goto), el bucle (la
// vuelta), las dos formas del switch y la cabecera en el pc 0.
func TestLosMarcosSonLosDeJavac(t *testing.T) {
	casos := []struct {
		nombre, fuente, clase, metodo, fixture string
	}{
		{"while", fuenteBucle, "Bucle", "corre", bucleJavac},
		{"switch denso", fuenteSwitch, "Prueba", "denso", pruebaJavac},
		{"switch disperso", fuenteSwitch, "Prueba", "disperso", pruebaJavac},
		{"cabecera en el pc 0", fuenteCab, "Cab", "f", cabJavac},
		{"cabecera en el pc 0 con local previo", fuenteCab, "Cab", "g", cabJavac},
	}
	for _, c := range casos {
		salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/" + c.clase + ".java", contenido: c.fuente}}, jarDelSDK(t))
		if err != nil {
			t.Fatalf("%s: el emisor rechazo el fuente: %v", c.nombre, err)
		}
		nuestra, err := marcosDeClase(salida[0].datos, c.metodo)
		if err != nil {
			t.Fatalf("%s: nuestros marcos no se leen: %v", c.nombre, err)
		}
		datos, err := base64.StdEncoding.DecodeString(c.fixture)
		if err != nil {
			t.Fatalf("%s: el fixture no es base64: %v", c.nombre, err)
		}
		suya, err := marcosDeClase(datos, c.metodo)
		if err != nil {
			t.Fatalf("%s: los marcos de javac no se leen: %v", c.nombre, err)
		}
		if suya.hayTabla != nuestra.hayTabla {
			t.Errorf("%s: javac lleva tabla=%v y nosotros %v", c.nombre, suya.hayTabla, nuestra.hayTabla)
			continue
		}
		if len(nuestra.marcos) != len(suya.marcos) {
			t.Errorf("%s: %d marcos nuestros y %d de javac", c.nombre, len(nuestra.marcos), len(suya.marcos))
		}
		for pc, ea := range suya.marcos {
			eb, ok := nuestra.marcos[pc]
			if !ok {
				t.Errorf("%s: falta el marco del pc %d que javac escribe (%s)", c.nombre, pc, describeMarco(ea))
				continue
			}
			if !igualesTipos(ea.locales, eb.locales) || !igualesTipos(ea.pila, eb.pila) {
				t.Errorf("%s: en el pc %d javac dice %s y nosotros %s", c.nombre, pc, describeMarco(ea), describeMarco(eb))
			}
		}
	}
}

// TestSinRamasNoHayStackMapTable: la ausencia tambien es una forma medida. El constructor sintetico
// del fixture del bucle es recto y javac no le pone el atributo; nosotros tampoco.
func TestSinRamasNoHayStackMapTable(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Bucle.java", contenido: fuenteBucle}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := marcosDeClase(salida[0].datos, "<init>")
	if err != nil {
		t.Fatalf("nuestro constructor no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(bucleJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := marcosDeClase(datos, "<init>")
	if err != nil {
		t.Fatalf("el constructor de javac no se lee: %v", err)
	}
	if suya.hayTabla {
		t.Fatalf("se esperaba que javac no pusiera tabla en el constructor recto")
	}
	if nuestra.hayTabla {
		t.Errorf("un cuerpo recto lleva StackMapTable y javac no lo lleva")
	}
}
