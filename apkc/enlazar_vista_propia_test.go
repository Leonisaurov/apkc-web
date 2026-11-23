package main

// La vista propia dentro del layout: el elemento es una clase totalmente cualificada, no un widget
// del framework, y el enlazador tiene que copiar sus `android:layout_width`/`android:layout_height`
// con el id del framework aunque no conozca el tipo. El fixture declara `xmlns:android` detrás de
// esos atributos a propósito: en XML el orden de los atributos no significa nada, y leerlos en el
// orden escrito dejaba los dos sin namespace, el AXML salía sin id y el framework abortaba el
// inflado con «You must supply a layout_width attribute» aunque el valor estuviera en el fichero
// (medido en el aparato con el layout de `lienzo`).
//
// La prueba exige las dos mitades: el binario con los ids del framework y el modelo de la vista
// previa publicando el nodo con su id y sus medidas, sin abortar la pantalla. La frontera que no se
// mide aquí está dicha en el comentario del modelo: la clase propia se dibuja como su rectángulo y
// su `onDraw` no se ejecuta.

import (
	"os"
	"testing"
)

const vistaPropiaFixture = "testdata/vista-propia/main.xml"

// attrDeElemento busca el atributo por nombre en un elemento del AXML enlazado.
func attrDeElemento(e *elementoEscribir, nombre string) (atributoEscribir, bool) {
	if e == nil {
		return atributoEscribir{}, false
	}
	for _, a := range e.attrs {
		if a.nombre == nombre {
			return a, true
		}
	}
	return atributoEscribir{}, false
}

// TestEnlazarVistaPropiaEnElLayout enlaza el fixture desde el texto, comprueba que los atributos de
// la clase propia y de la raíz llevan el id del framework, relee el AXML escrito y arma con él el
// modelo de la vista previa.
func TestEnlazarVistaPropiaEnElLayout(t *testing.T) {
	contenido, err := os.ReadFile(vistaPropiaFixture)
	if err != nil {
		t.Fatalf("el fixture: %v", err)
	}
	valores, _, manifiesto := fuentesDelStudio(t)
	salida, err := enlazarProyecto(valores,
		[]fuenteXML{{nombre: "res/layout/main.xml", contenido: contenido}},
		nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salió: %v", err)
	}

	doc := salida.layouts["res/layout/main.xml"]
	// La raíz declara `xmlns:android` detrás de sus dos atributos: aun así tienen que llevar el id
	// del framework y el valor -1 de match_parent, que es lo que el inflado lee.
	for _, medida := range []struct {
		nombre string
		id     uint32
	}{{"layout_width", 0x010100f4}, {"layout_height", 0x010100f5}} {
		a, ok := attrDeElemento(doc.raiz, medida.nombre)
		if !ok {
			t.Fatalf("la raíz no lleva %s", medida.nombre)
		}
		if a.ns != "android" || a.id != medida.id {
			t.Errorf("la raíz lleva %s como ns=%q id=%#08x, medido android/%#08x",
				medida.nombre, a.ns, a.id, medida.id)
		}
		if a.tipo != tipoEnteroDec || a.datos != 0xffffffff {
			t.Errorf("la raíz lleva %s como tipo %#02x datos %#x, medido entero -1",
				medida.nombre, a.tipo, a.datos)
		}
	}

	// Y el hijo es la clase propia, que el enlazador no conoce como widget.
	if len(doc.raiz.hijos) != 1 {
		t.Fatalf("la raíz trae %d hijos y el fixture tiene 1", len(doc.raiz.hijos))
	}
	propia := doc.raiz.hijos[0]
	if propia.nombre != "com.ejemplo.lienzo.Lienzo" {
		t.Fatalf("el hijo es %q, medido com.ejemplo.lienzo.Lienzo", propia.nombre)
	}
	for _, medida := range []struct {
		nombre string
		id     uint32
	}{{"layout_width", 0x010100f4}, {"layout_height", 0x010100f5}} {
		a, ok := attrDeElemento(propia, medida.nombre)
		if !ok {
			t.Fatalf("la vista propia no lleva %s", medida.nombre)
		}
		if a.ns != "android" || a.id != medida.id {
			t.Errorf("la vista propia lleva %s como ns=%q id=%#08x, medido android/%#08x",
				medida.nombre, a.ns, a.id, medida.id)
		}
		if a.tipo != tipoEnteroDec || a.datos != 0xffffffff {
			t.Errorf("la vista propia lleva %s como tipo %#02x datos %#x, medido entero -1",
				medida.nombre, a.tipo, a.datos)
		}
	}
	if id, ok := attrDeElemento(propia, "id"); !ok || id.ns != "android" {
		t.Errorf("la vista propia no lleva android:id con namespace: %+v", id)
	}

	// El AXML escrito se relee: los dos atributos siguen con su namespace y su valor de framework,
	// que es lo que el intérprete del aparato mira para no abortar el inflado.
	datos, err := escribeAXML(doc)
	if err != nil {
		t.Fatalf("el layout no se escribió: %v", err)
	}
	axml, err := leeAXML(datos)
	if err != nil {
		t.Fatalf("el AXML de la vista propia no se deja leer: %v", err)
	}
	ctx := &contextoVista{axml: axml}
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol del AXML: %v", err)
	}
	leida := arbol.hijo("com.ejemplo.lienzo.Lienzo")
	if leida == nil {
		t.Fatal("el AXML releído no trae el elemento de la clase propia")
	}
	for _, nombre := range []string{"layout_width", "layout_height"} {
		v, ok := leida.atributo(uriDeRecursos, nombre)
		if !ok {
			t.Fatalf("el AXML releído no trae android:%s en la vista propia", nombre)
		}
		if v.tipo != tipoEnteroDec || v.datos != 0xffffffff {
			t.Errorf("android:%s se relee con tipo %#02x datos %#x, medido entero -1",
				nombre, v.tipo, v.datos)
		}
	}

	// El modelo de la vista previa: la pantalla no aborta y el nodo viaja con su id y sus medidas.
	manifiestoBytes, err := escribeAXML(salida.manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	arsc, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiestoBytes,
		"resources.arsc":      arsc,
		"res/layout/main.xml": datos,
	})
	modelo, err := modeloVistaDeAPKCon(apk, "main", false)
	if err != nil {
		t.Fatalf("el preview abortó con el layout de la vista propia: %v", err)
	}
	if modelo.Nodos != 2 {
		t.Errorf("el modelo publica %d nodos y el layout tiene 2", modelo.Nodos)
	}
	if modelo.Raiz == nil || modelo.Raiz.Tipo != "FrameLayout" {
		t.Fatalf("la raíz del modelo es %+v", modelo.Raiz)
	}
	if r := modelo.Raiz.Rect; r.Ancho != dpPantallaAncho || r.Alto != dpPantallaAlto {
		t.Errorf("la raíz ocupa %dx%d y la pantalla es %dx%d", r.Ancho, r.Alto, dpPantallaAncho, dpPantallaAlto)
	}
	if len(modelo.Raiz.Hijos) != 1 {
		t.Fatalf("el modelo publica %d hijos y el layout tiene 1", len(modelo.Raiz.Hijos))
	}
	nodo := modelo.Raiz.Hijos[0]
	if nodo.Tipo != "com.ejemplo.lienzo.Lienzo" {
		t.Errorf("el nodo es %q, medido com.ejemplo.lienzo.Lienzo", nodo.Tipo)
	}
	if nodo.ID != "id/lienzo" {
		t.Errorf("el nodo lleva id %q, medido id/lienzo", nodo.ID)
	}
	if nodo.Ancho != "match_parent" || nodo.Alto != "match_parent" {
		t.Errorf("el nodo se publica como ancho %q alto %q, medido match_parent/match_parent",
			nodo.Ancho, nodo.Alto)
	}
	if r := nodo.Rect; r.X != 0 || r.Y != 0 || r.Ancho != dpPantallaAncho || r.Alto != dpPantallaAlto {
		t.Errorf("el nodo de la vista propia se coloca en %+v y se esperaba 0,0 %dx%d",
			r, dpPantallaAncho, dpPantallaAlto)
	}
}
