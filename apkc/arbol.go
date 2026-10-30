package main

// El arbol en texto de un layout o del manifiesto, primera mitad de la otra mitad del paso 3-G. Lee el
// XML con el decodificador de Go y deja lo que hace falta para escribir el AXML: el nombre de cada
// elemento, su linea, sus atributos con el prefijo con el que fueron escritos y el orden de los hijos.
//
// Dos cosas medida sobre Go y sobre los ficheros de aapt2 mandan la forma de este fichero:
//   - el decodificador da en `Name.Space` el URI resuelto, y en las declaraciones `xmlns:` el prefijo
//     (sondeo: attr Space="http://schemas.android.com/apk/res/android" Local="orientation", y
//     Space="xmlns" Local="android"). El binario quiere el PREFIJO para mirar el URI despues, asi que
//     se guarda el mapa al reves.
//   - `InputPos()` cae detras del `>` de la etiqueta de apertura, que en el manifiesto esta tres lineas
//     mas abajo de donde empieza la etiqueta (`<activity` abre en la 13, sus atributos ocupan hasta la
//     17 y aapt2 anota 13). La linea se busca entonces en el texto con un cursor que solo avanza.

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// attrFuente es un atributo tal como esta escrito en el texto, sin resolver.
type attrFuente struct {
	// prefijo es el que escribio el fuente: "android" en android:text, "" en style.
	prefijo string
	nombre  string
	texto   string
}

// nodoFuente es un elemento del arbol.
type nodoFuente struct {
	prefijo string
	nombre  string
	linea   int
	attrs   []attrFuente
	hijos   []*nodoFuente
}

// arbolFuente es lo que saca leerArbolDe: la raiz y el mapa de prefijos a URI, con la linea donde cada
// prefijo se declaro (que es la linea que llevan los trozos de namespace del binario).
type arbolFuente struct {
	raiz *nodoFuente
	// prefijos: URI -> prefijo, en el orden en que se declararon
	prefijos map[string]string
	// declaracion: prefijo -> URI, con su linea
	lineaDe map[string]int
}

// leerArbolDe convierte una fuente en texto en un arbol. Un texto suelto dentro de un elemento se
// rechaza: en los dos ficheros medidos el contenido de un layout y del manifiesto son solo elementos.
func leerArbolDe(f fuenteXML) (*arbolFuente, error) {
	texto := string(f.contenido)
	salida := &arbolFuente{prefijos: map[string]string{}, lineaDe: map[string]int{}}
	d := xml.NewDecoder(strings.NewReader(texto))
	cursor := 0
	lineaDeApertura := func(nombre string) int {
		i := strings.Index(texto[cursor:], "<"+nombre)
		if i < 0 {
			return 1
		}
		cursor += i
		return 1 + strings.Count(texto[:cursor], "\n")
	}

	pila := []*nodoFuente{}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.nombre, err)
		}
		switch v := tok.(type) {
		case xml.ProcInst:
			continue // la declaracion <?xml version…?> no deja huella en el binario
		case xml.Comment:
			continue // y los comentarios tampoco
		case xml.StartElement:
			nodo := &nodoFuente{nombre: v.Name.Local, linea: lineaDeApertura(v.Name.Local)}
			for _, a := range v.Attr {
				if a.Name.Space == "xmlns" {
					// xmlns:android="…" : el prefijo es el nombre del atributo
					if _, visto := salida.lineaDe[a.Name.Local]; !visto {
						salida.prefijos[a.Value] = a.Name.Local
						salida.lineaDe[a.Name.Local] = nodo.linea
					}
					continue
				}
				if a.Name.Local == "xmlns" {
					if _, visto := salida.lineaDe[""]; !visto {
						salida.prefijos[a.Value] = ""
						salida.lineaDe[""] = nodo.linea
					}
					continue
				}
				nodo.attrs = append(nodo.attrs, attrFuente{
					prefijo: salida.prefijos[a.Name.Space],
					nombre:  a.Name.Local,
					texto:   a.Value,
				})
			}
			if v.Name.Space != "" {
				nodo.prefijo = salida.prefijos[v.Name.Space]
			}
			if len(pila) == 0 {
				if salida.raiz != nil {
					return nil, fmt.Errorf("%s: dos raices en el documento", f.nombre)
				}
				salida.raiz = nodo
			} else {
				pila[len(pila)-1].hijos = append(pila[len(pila)-1].hijos, nodo)
			}
			pila = append(pila, nodo)
		case xml.EndElement:
			if len(pila) == 0 {
				return nil, fmt.Errorf("%s: cierre de %s sin apertura", f.nombre, v.Name.Local)
			}
			if pila[len(pila)-1].nombre != v.Name.Local {
				return nil, fmt.Errorf("%s: se cierra %s dentro de %s", f.nombre, v.Name.Local, pila[len(pila)-1].nombre)
			}
			pila = pila[:len(pila)-1]
		case xml.CharData:
			if len(pila) > 0 && strings.TrimSpace(string(v)) != "" {
				return nil, fmt.Errorf("%s: texto suelto dentro de %s, y eso no esta medido", f.nombre, pila[len(pila)-1].nombre)
			}
		}
	}
	if salida.raiz == nil {
		return nil, fmt.Errorf("%s no tiene raiz", f.nombre)
	}
	if len(pila) != 0 {
		return nil, fmt.Errorf("%s: %s queda sin cerrar", f.nombre, pila[len(pila)-1].nombre)
	}
	return salida, nil
}

// buscar baja por una ruta de nombres que EMPIEZA en la raiz: buscar("manifest", "application",
// "activity"). Devuelve el primer nodo que casa, o nil.
func (a *arbolFuente) buscar(ruta ...string) *nodoFuente {
	if a.raiz == nil || len(ruta) == 0 || a.raiz.nombre != ruta[0] {
		return nil
	}
	n := a.raiz
	for _, nombre := range ruta[1:] {
		var siguiente *nodoFuente
		for _, h := range n.hijos {
			if h.nombre == nombre {
				siguiente = h
				break
			}
		}
		if siguiente == nil {
			return nil
		}
		n = siguiente
	}
	return n
}

// atribuDe: el texto del atributo con ese prefijo y nombre, mas si estaba.
func (n *nodoFuente) atribuDe(prefijo, nombre string) (string, bool) {
	for _, a := range n.attrs {
		if a.prefijo == prefijo && a.nombre == nombre {
			return a.texto, true
		}
	}
	return "", false
}
