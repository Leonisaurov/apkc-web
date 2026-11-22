package main

// Las pruebas de la Fase 1 de la vista previa. Hay dos clases, y las dos hacen falta:
//   - simetría: lo que escriben axmlwriter.go y arscwriter.go se lee con el lector y se resuelve
//     igual que lo escrito (mismos nombres, mismas cadenas, mismos ids);
//   - los bytes reales de aapt2 que ya fijó la Fase 3 en base64 (axmlManifiesto, axmlLayout y
//     arscTabla), que son el otro extremo: el lector tiene que servir para el fichero de una
//     herramienta que no es este fork.
//
// El apk se arma en memoria con archive/zip: no se versiona ningún binario (regla 2), y así la
// prueba del comando completo no depende de un sdk ni de un proceso.

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"testing"
)

// zipDeEntradas arma un zip en memoria con las entradas dadas, que es lo que espera un lector de
// apk. Los nombres son los del paquete: AndroidManifest.xml, resources.arsc y res/layout/*.xml.
func zipDeEntradas(t *testing.T, entradas map[string][]byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for nombre, datos := range entradas {
		w, err := zw.Create(nombre)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if _, err := w.Write(datos); err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// apkDeLosEscritores es el apk completo salido del fork: el manifiesto y el layout de
// axmlwriter_test.go y la tabla de arscwriter_test.go, que son los mismos bytes que aapt2 deja en
// el proyecto del studio. Nada de aquí lo escribió aapt2: es la simetría de punta a punta.
func apkDeLosEscritores(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelStudio())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
	})
}

// TestPreviewLeeElAXMLQueEscribeElFork: el árbol del layout, con sus atributos y el texto que el
// escritor dejó en el pool del propio fichero. Sin tabla, un id se queda en su número.
func TestPreviewLeeElAXMLQueEscribeElFork(t *testing.T) {
	datos, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	a, err := leeAXML(datos)
	if err != nil {
		t.Fatalf("el AXML del fork no se deja leer: %v", err)
	}
	if !a.tabla.utf8 {
		t.Error("el layout se escribe en UTF-8 y el lector dice UTF-16")
	}
	ctx := &contextoVista{axml: a}
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol: %v", err)
	}
	if arbol.nombre != "LinearLayout" {
		t.Fatalf("la raíz se lee como %q", arbol.nombre)
	}
	if len(arbol.hijos) != 2 || arbol.hijos[0].nombre != "TextView" || arbol.hijos[1].nombre != "Button" {
		t.Fatalf("los hijos se leen como %d nodos, y se esperaban TextView y Button", len(arbol.hijos))
	}
	if texto := ctx.textoDeAtributo(arbol.hijos[0], uriDeRecursos, "text"); texto != "Hello World!" {
		t.Errorf("el texto del TextView se lee como %q", texto)
	}
	if texto := ctx.textoDeAtributo(arbol.hijos[1], uriDeRecursos, "text"); texto != "Press me" {
		t.Errorf("el texto del Button se lee como %q", texto)
	}
	// el ancho va como entero con signo: -2 es wrap_content y -1 match_parent
	ancho, ok := arbol.hijos[0].atributo(uriDeRecursos, "layout_width")
	if !ok || valorDeMedida(ancho) != "wrap_content" {
		t.Errorf("layout_width del TextView: %+v -> %q", ancho, valorDeMedida(ancho))
	}
	alto, ok := arbol.atributo(uriDeRecursos, "layout_height")
	if !ok || valorDeMedida(alto) != "match_parent" {
		t.Errorf("layout_height de la raíz: %+v -> %q", alto, valorDeMedida(alto))
	}
	// sin tabla, la referencia del id no tiene nombre y se queda su número
	id, ok := arbol.hijos[1].atributo(uriDeRecursos, "id")
	if !ok || ctx.texto(id) != "@0x7f010000" {
		t.Errorf("el id sin tabla se lee como %q, y el fichero guarda la referencia 0x7f010000", ctx.texto(id))
	}
	// la orientación la escribió el enlace como el enum 1, que es vertical
	orientacion, _ := arbol.atributo(uriDeRecursos, "orientation")
	if int32(orientacion.datos) != 1 {
		t.Errorf("orientation se lee como %d, el fichero guarda 1 (vertical)", int32(orientacion.datos))
	}
}

// TestPreviewResuelveLaTablaQueEscribeElFork: los ids de un layout, contra la tabla del propio
// escritor. Incluye la segunda configuración con una clave ausente, que hasta ahora el lector no
// sabía leer: el offset 0xffffffff se guarda como hueco en su sitio y no revienta.
func TestPreviewResuelveLaTablaQueEscribeElFork(t *testing.T) {
	tb := tablaDelStudio()
	// una `values-es` que no declara `main`: es la forma medida en el §94, con el offset -1
	tb.tipos[1].bloques = append(tb.tipos[1].bloques, bloqueEscribir{
		config:   "es",
		entradas: []entradaEscribir{{clave: 1, falta: true}},
	})
	datos, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	tr, err := leeTabla(datos)
	if err != nil {
		t.Fatalf("la tabla del fork no se deja leer: %v", err)
	}

	tipo, clave, ok := tr.nombreDeId(0x7f010000)
	if !ok || tipo != "id" || clave != "pulsar" {
		t.Errorf("0x7f010000 resuelve a %q/%q (ok=%v), y la tabla lo llama id/pulsar", tipo, clave, ok)
	}
	v, ok := tr.valorDeId(0x7f020000)
	if !ok || v.tipo != tipoCadena || int(v.datos) >= len(tr.global.cadenas) ||
		tr.global.cadenas[v.datos] != "res/layout/main.xml" {
		t.Errorf("0x7f020000 no apunta a la ruta del layout: %+v (ok=%v)", v, ok)
	}
	if _, ok := tr.valorDeId(0x7f030000); ok {
		t.Error("un estilo es un valor complejo y no tiene Res_value simple")
	}
	if _, _, ok := tr.nombreDeId(0x7f040000); ok {
		t.Error("un tipo que la tabla no tiene no puede resolver")
	}
	if tipo, clave, ok := tr.nombreDeId(0x01030042); ok {
		t.Errorf("un id del framework se resolvió como %q/%q sin estar en la tabla", tipo, clave)
	}

	ausentes := 0
	for _, bl := range tr.bloques {
		for _, e := range bl.entradas {
			if e.ausente {
				ausentes++
			}
		}
	}
	if ausentes != 1 {
		t.Errorf("la configuración `es` deja %d entradas ausentes y se esperaba una", ausentes)
	}
}

// TestPreviewSobreUnAPKDeLosEscritores: el comando completo sobre el apk que arma el propio fork.
// Es la prueba que dice que el modelo no depende de aapt2: manifiesto, tabla y layout salen de los
// escritores, y el lector los vuelve a juntar.
func TestPreviewSobreUnAPKDeLosEscritores(t *testing.T) {
	m, err := modeloVistaDeAPK(apkDeLosEscritores(t), "")
	if err != nil {
		t.Fatalf("el apk del fork no se deja leer: %v", err)
	}
	if m.Paquete != "com.hola.apkcstudio" {
		t.Errorf("el paquete se lee como %q", m.Paquete)
	}
	if m.Version != "0.1" {
		t.Errorf("la versión se lee como %q y el manifiesto dice 0.1", m.Version)
	}
	if m.Actividad != ".MainActivity" {
		t.Errorf("la actividad lanzadora se lee como %q", m.Actividad)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("el layout se lee como %q", m.Layout)
	}
	if !m.Aproximado || m.Pantalla.Ancho != dpPantallaAncho || m.Pantalla.Alto != dpPantallaAlto {
		t.Errorf("la pantalla del modelo es %+v (aproximado=%v)", m.Pantalla, m.Aproximado)
	}
	if m.Nodos != 3 || m.Raiz == nil || m.Raiz.Tipo != "LinearLayout" {
		t.Fatalf("el modelo trae %d nodos con raíz %+v", m.Nodos, m.Raiz)
	}
	if r := m.Raiz.Rect; r.X != 0 || r.Y != 0 || r.Ancho != dpPantallaAncho || r.Alto != dpPantallaAlto {
		t.Errorf("la raíz ocupa %+v y el layout pide match_parentxmatch_parent", r)
	}
	if len(m.Raiz.Hijos) != 2 {
		t.Fatalf("la raíz trae %d hijos", len(m.Raiz.Hijos))
	}
	texto, boton := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if texto.Tipo != "TextView" || texto.Texto != "Hello World!" {
		t.Errorf("el primer hijo es %q con texto %q", texto.Tipo, texto.Texto)
	}
	if boton.Tipo != "Button" || boton.Texto != "Press me" {
		t.Errorf("el segundo hijo es %q con texto %q", boton.Tipo, boton.Texto)
	}
	if boton.ID != "id/pulsar" || boton.IDRef != "0x7f010000" {
		t.Errorf("el id del Button se lee como %q (%q), y la tabla lo llama id/pulsar", boton.ID, boton.IDRef)
	}
	if texto.Ancho != "wrap_content" || texto.Alto != "wrap_content" {
		t.Errorf("el TextView declara %s x %s", texto.Ancho, texto.Alto)
	}
	// el TextView va arriba y el Button debajo: la orientación vertical del layout manda
	if texto.Rect.Y != 0 || boton.Rect.Y != texto.Rect.Alto {
		t.Errorf("en vertical se esperaba el Button en y=%d y está en %d", texto.Rect.Alto, boton.Rect.Y)
	}
}

// TestPreviewSobreLosBytesDeAapt2: los tres ficheros reales de la Fase 3, los que escribió el aapt2
// del teléfono sobre el proyecto del studio, metidos en un apk en memoria. Es el otro extremo de la
// simetría y la prueba de que el formato leído no es solo el que escribimos nosotros.
func TestPreviewSobreLosBytesDeAapt2(t *testing.T) {
	manifiesto := mustBase64(t, axmlManifiesto, 2000)
	layout := mustBase64(t, axmlLayout, 784)
	arsc := mustBase64(t, arscTabla, 1088)
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
	})
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk de aapt2 no se deja leer: %v", err)
	}
	if m.Paquete != "com.hola.apkcstudio" || m.Version != "0.1" || m.Actividad != ".MainActivity" {
		t.Errorf("el manifiesto de aapt2 se lee como %q %q %q", m.Paquete, m.Version, m.Actividad)
	}
	if m.Layout != "res/layout/main.xml" || m.Nodos != 3 || m.Raiz == nil {
		t.Fatalf("layout %q con %d nodos y raíz %+v", m.Layout, m.Nodos, m.Raiz)
	}
	if m.Raiz.Hijos[0].Texto != "Hello World!" || m.Raiz.Hijos[1].Texto != "Press me" {
		t.Errorf("los textos se leen como %q y %q", m.Raiz.Hijos[0].Texto, m.Raiz.Hijos[1].Texto)
	}
	if m.Raiz.Hijos[1].ID != "id/pulsar" {
		t.Errorf("el id del Button se lee como %q", m.Raiz.Hijos[1].ID)
	}
	if m.Raiz.Hijos[0].Estilo != "@0x01030042" {
		t.Errorf("el style del TextView es un id del framework y se lee como %q", m.Raiz.Hijos[0].Estilo)
	}
}

// TestPreviewRechazaLoQueNoEs: los errores van dichos por su nombre, que en la pestaña es lo único
// que el usuario llega a ver cuando el paquete está a medias.
func TestPreviewRechazaLoQueNoEs(t *testing.T) {
	if _, err := modeloVistaDeAPK([]byte("no soy un zip"), ""); err == nil {
		t.Error("se aceptó un fichero que no es un zip")
	}
	sinManifiesto := zipDeEntradas(t, map[string][]byte{"res/layout/main.xml": {0x03, 0x00, 0x08, 0x00}})
	if _, err := modeloVistaDeAPK(sinManifiesto, ""); err == nil {
		t.Error("se aceptó un apk sin AndroidManifest.xml")
	}
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	sinLayout := zipDeEntradas(t, map[string][]byte{"AndroidManifest.xml": manifiesto})
	if _, err := modeloVistaDeAPK(sinLayout, ""); err == nil {
		t.Error("se aceptó un apk sin ningún layout")
	}
	if _, err := modeloVistaDeAPK(apkDeLosEscritores(t), "otro"); err == nil {
		t.Error("se aceptó pedir un layout que la tabla no tiene")
	}
	// y el layout pedido por nombre sí se encuentra
	if m, err := modeloVistaDeAPK(apkDeLosEscritores(t), "main"); err != nil || m.Layout != "res/layout/main.xml" {
		t.Errorf("pedir el layout main: %v, %q", err, m.Layout)
	}
	// un paquete sin tabla: el layout se busca en el zip y las referencias se quedan en su número
	layout, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	sinTabla := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"res/layout/main.xml": layout,
	})
	m, err := modeloVistaDeAPK(sinTabla, "")
	if err != nil {
		t.Fatalf("un apk sin resources.arsc tiene que leerse igual: %v", err)
	}
	if m.Layout != "res/layout/main.xml" || m.Raiz.Hijos[1].ID != "@0x7f010000" {
		t.Errorf("sin tabla: layout %q e id %q", m.Layout, m.Raiz.Hijos[1].ID)
	}
}

func mustBase64(t *testing.T, s string, mide int) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if len(b) != mide {
		t.Fatalf("el fixture mide %d bytes y se esperaban %d", len(b), mide)
	}
	return b
}
