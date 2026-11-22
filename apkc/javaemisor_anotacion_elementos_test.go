package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
)

// La sonda de anotaciones con elementos simples, compilada con javac 17 y
// `javac --release 8 -g:lines,source Mark.java Annotated.java` (el tipo Mark es @interface en el
// fixture; aquí se declara como interface para que el emisor resuelva el descriptor LMark;).
// El dex no distingue las anotaciones de fuente —el dexer las sintetiza de los hechos estructurales—,
// así que el oráculo es el atributo RuntimeVisibleAnnotations del propio .class, comparado por su
// contenido semántico (nombre y pares nombre/valor), no por índices del pool.
const anotElementosJavac = "yv66vgAAADQAHAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAJQW5ub3RhdGVkAQAFZmllbGQBAAFJAQAZUnVudGltZVZpc2libGVBbm5vdGF0aW9ucwEABkxNYXJrOwEAA251bQMAAAABAQAEbmFtZQEAAWYBAAJvbgMAAAAAAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEABChJKUkDAAAAAgEAAW0BAApTb3VyY2VGaWxlAQAOQW5ub3RhdGVkLmphdmEDAAAAKgEABGhvbGEAIQAHAAIAAAABAAAACQAKAAEACwAAABUAAQAMAAMADUkADgAPcwAQABFaABIAAgABAAUABgABABMAAAAdAAEAAQAAAAUqtwABsQAAAAEAFAAAAAYAAQAAAAIAAAAQABUAAgATAAAAGgABAAIAAAACG6wAAAABABQAAAAGAAEAAAAGAAsAAAAVAAEADAADAA1JABYAD3MAFwARWgAOAAIAGAAAAAIAGQALAAAAFQABAAwAAwANSQAaAA9zABsAEVoADg=="

const anotElementosFuente = `@Mark(num = 42, name = "hola", on = true)
public class Annotated {
  @Mark(num = 1, name = "f", on = false)
  int field;
  @Mark(num = 2, name = "m", on = true)
  int f(int n) { return n; }
}
interface Mark {}
`

// decValorAnot decodifica un element_value (JVMS 4.7.16.1) del cuerpo de RuntimeVisibleAnnotations,
// resolviendo los índices del pool contra el texto que guardan. Es datos de test, no del emisor.
func decValorAnot(p *pool, r *cursor) string {
	tag := r.u1()
	switch tag {
	case 'I', 'Z':
		i := int(r.u2())
		v := int32(binary.BigEndian.Uint32(p.body[i]))
		return string(rune(tag)) + ":" + strconv.Itoa(int(v))
	case 's':
		return "s:" + p.utf[int(r.u2())]
	case 'c':
		return "c:" + p.utf[int(r.u2())]
	case 'e':
		t := p.utf[int(r.u2())]
		n := p.utf[int(r.u2())]
		return "e:" + t + "." + n
	case '[':
		c := int(r.u2())
		xs := []string{}
		for i := 0; i < c; i++ {
			xs = append(xs, decValorAnot(p, r))
		}
		return "[" + strings.Join(xs, ",") + "]"
	}
	return "?" + string(rune(tag))
}

func decAnotaciones(p *pool, body []byte) string {
	r := &cursor{b: body}
	n := int(r.u2())
	out := []string{}
	for i := 0; i < n && !r.failed(); i++ {
		tipo := p.utf[int(r.u2())]
		pares := int(r.u2())
		elems := []string{}
		for j := 0; j < pares && !r.failed(); j++ {
			nombre := p.utf[int(r.u2())]
			elems = append(elems, nombre+"="+decValorAnot(p, r))
		}
		out = append(out, tipo+"{"+strings.Join(elems, ",")+"}")
	}
	return strings.Join(out, ";")
}

// anotacionesDeBusca los atributos RuntimeVisibleAnnotations de campos, metodos y clase y devuelve su
// contenido normalizado. Vuelve a recorrer el ClassFile porque classFull solo conserva el de la clase.
func anotacionesDeBusca(t *testing.T, data []byte) (clase, campo, metodo string, err error) {
	r := &cursor{b: data}
	r.u4()
	r.u2()
	r.u2()
	p, err := r.constantPool()
	if err != nil {
		return "", "", "", err
	}
	r.u2() // access_flags
	r.u2() // this_class
	r.u2() // super_class
	for _, n := range r.u2s() {
		_ = n
	}
	for nivel := 0; nivel < 2; nivel++ { // campos, metodos
		num := int(r.u2())
		for i := 0; i < num; i++ {
			r.u2() // access_flags
			r.u2() // name_index
			r.u2() // descriptor_index
			for _, a := range r.attributeTable(p) {
				if a.name != "RuntimeVisibleAnnotations" {
					continue
				}
				s := decAnotaciones(p, a.body)
				if nivel == 0 {
					campo = s
				} else {
					metodo = s
				}
			}
		}
	}
	// los atributos de la clase son UNA tabla, no una por miembro
	for _, a := range r.attributeTable(p) {
		if a.name == "RuntimeVisibleAnnotations" {
			clase = decAnotaciones(p, a.body)
		}
	}
	return clase, campo, metodo, nil
}

func TestAnotacionesConElementosSimples(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Annotated.java", contenido: anotElementosFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó la anotación: %v", err)
	}
	var mia []byte
	for _, c := range salida {
		if bytes.Contains(c.datos, []byte("Annotated")) {
			mia = c.datos
			break
		}
	}
	if mia == nil {
		// el nombre también está en el pool de la interface; buscamos por el internal exacto
		for _, c := range salida {
			if c.interno == "Annotated" {
				mia = c.datos
			}
		}
	}
	if mia == nil {
		t.Fatalf("no salió Annotated: %s", nombresDe(salida))
	}
	suya, err := base64.StdEncoding.DecodeString(anotElementosJavac)
	if err != nil {
		t.Fatal(err)
	}
	claseMia, campoMio, metodoMio, err := anotacionesDeBusca(t, mia)
	if err != nil {
		t.Fatalf("nuestro classfile: %v", err)
	}
	claseSuya, campoSuya, metodoSuya, err := anotacionesDeBusca(t, suya)
	if err != nil {
		t.Fatalf("el classfile de javac: %v", err)
	}
	for _, par := range []struct {
		donde string
		mia   string
		suya  string
	}{
		{"clase", claseMia, claseSuya},
		{"campo", campoMio, campoSuya},
		{"metodo", metodoMio, metodoSuya},
	} {
		if par.mia == "" {
			t.Errorf("%s: no escribimos RuntimeVisibleAnnotations", par.donde)
			continue
		}
		if par.mia != par.suya {
			t.Errorf("%s: nuestro %q contra javac %q", par.donde, par.mia, par.suya)
		} else {
			t.Logf("%s: %s", par.donde, par.mia)
		}
	}
}
