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
	"encoding/json"
	"strings"
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
	// el TextView y el Button se centran los dos: el layout declara gravity="center" y el bloque de
	// hijos queda en el medio de la pantalla (640 - 24 - 48 = 568, entre 2 = 284)
	if texto.Rect.Y != 284 || boton.Rect.Y != texto.Rect.Y+texto.Rect.Alto {
		t.Errorf("con gravity=center se esperaba el TextView en y=284 y el Button en y=%d; están en %d y %d",
			texto.Rect.Y+texto.Rect.Alto, texto.Rect.Y, boton.Rect.Y)
	}
	if m.Raiz.Gravity != "center" {
		t.Errorf("la raíz declara gravity=center y el modelo dice %q", m.Raiz.Gravity)
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

// Cadenas que no son ASCII: en el pool UTF-8 de Android el primer prefijo de una cadena cuenta
// unidades utf-16 y el segundo bytes, y con estos textos dejan de coincidir. «Hola & adiós ñ» mide
// 14 unidades y 16 bytes; el emoji, dos unidades y seis bytes, porque aapt2 lo escribe como su par
// sustituto en CESU-8 (3+3), no como los cuatro bytes del UTF-8 de Go.
const (
	textoAcentos = "Hola & adiós ñ"
	textoEmoji   = "hola \U0001F600"
	textoAmp     = "&amp;"
)

// poolAapt2Emoji son los 100 bytes del trozo RES_STRING_POOL (UTF-8) que aapt2 35.0.1 escribió en
// el resources.arsc de la sonda, copiados tal cual desde el offset 12 del fichero. Sus cadenas, en
// orden, son las de abajo: el «&amp;» que sobrevive al XML, «Hola & adiós ñ 😀» —con 17 unidades
// declaradas y 23 bytes, el emoji como par sustituto—, «ñ» y «😀» solo.
const poolAapt2Emoji = "AQAcAGQAAAAFAAAAAAAAAAABAAAwAAAAAAAAAAAAAAADAAAACwAAACUAAAAqAAAAAAAABQUmYW1wOwARF0hvbGEgJiBhZGnDs3Mgw7Eg7aC97biAAAECw7EAAgbtoL3tuIAAAA=="

// TestPreviewLeeElPoolQueEscribeAapt2: el pool UTF-8 tal cual lo dejó aapt2 35.0.1 en un
// resources.arsc con emoji. No es un ida y vuelta del escritor propio: los bytes están copiados
// del fichero de la herramienta, y son el caso que el lector rechazaba («claims 17 utf-16 units
// and its 23 bytes hold 21») porque la rama de UTF-8 copiaba los bytes sin decodificar el CESU-8.
// Además, el escritor tiene que reproducir ese trozo byte a byte.
func TestPreviewLeeElPoolQueEscribeAapt2(t *testing.T) {
	crudo := mustBase64(t, poolAapt2Emoji, 100)
	p, err := leePool(crudo, 0)
	if err != nil {
		t.Fatalf("el pool de aapt2 con emoji no se deja leer: %v", err)
	}
	esperadas := []string{"", "&amp;", "Hola & adiós ñ \U0001F600", "ñ", "\U0001F600"}
	if len(p.cadenas) != len(esperadas) {
		t.Fatalf("el pool devuelve %d cadenas y tiene %d", len(p.cadenas), len(esperadas))
	}
	for i, esperada := range esperadas {
		if p.cadenas[i] != esperada {
			t.Errorf("cadena #%d leída como %q (% x), escrita %q", i, p.cadenas[i], []rune(p.cadenas[i]), esperada)
		}
	}
	// el emoji vuelve como un rune U+1F600, no como las dos unidades sueltas del par sustituto
	if r := []rune(p.cadenas[4]); len(r) != 1 || r[0] != 0x1F600 {
		t.Errorf("el emoji se lee como %d runes: %U", len(r), r)
	}
	if !p.utf8 {
		t.Error("el trozo de aapt2 está en modo UTF-8 y el lector dice que no")
	}
	// y el escritor del fork vuelve a producir esos mismos 100 bytes
	if nuestro := escribePool(esperadas, true); !bytes.Equal(nuestro, crudo) {
		t.Errorf("el pool escrito mide %d bytes y el de aapt2 %d\nnuestro: % x\nsuyo:   % x",
			len(nuestro), len(crudo), nuestro, crudo)
	}
}

// TestPreviewLeeElPoolUTF8ConAcentosYEmoji ataca la causa aislada del fallo de la Fase 1: leeCadena
// tomaba el primer prefijo como bytes y rompía con cualquier carácter multibyte. El pool se prueba
// suelto y dentro de los dos contenedores que lo usan: un AXML (UTF-8 como un layout y UTF-16 como
// el manifiesto) y un resources.arsc.
func TestPreviewLeeElPoolUTF8ConAcentosYEmoji(t *testing.T) {
	// el pool suelto, con la forma del formato y la terminación en cero en el sitio que toca
	datosPool := escribePool([]string{"id", textoAcentos, textoEmoji, textoAmp}, true)
	p, err := leePool(datosPool, 0)
	if err != nil {
		t.Fatalf("el pool UTF-8 con acentos no se deja leer: %v", err)
	}
	esperadas := []string{"id", textoAcentos, textoEmoji, textoAmp}
	if len(p.cadenas) != len(esperadas) {
		t.Fatalf("el pool devuelve %d cadenas y se escribieron %d", len(p.cadenas), len(esperadas))
	}
	for i, esperada := range esperadas {
		if p.cadenas[i] != esperada {
			t.Errorf("cadena #%d leída como %q, escrita %q", i, p.cadenas[i], esperada)
		}
	}

	// el mismo pool dentro de un AXML UTF-8, el modo del layout
	doc := documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  1,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				{nombre: "TextView", linea: 2, attrs: []atributoEscribir{
					{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: textoAcentos}}},
				{nombre: "TextView", linea: 3, attrs: []atributoEscribir{
					{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: textoEmoji}}},
				{nombre: "TextView", linea: 4, attrs: []atributoEscribir{
					{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: textoAmp}}},
			},
		},
	}
	datosAXML, err := escribeAXML(doc)
	if err != nil {
		t.Fatalf("el AXML con acentos: %v", err)
	}
	axml, err := leeAXML(datosAXML)
	if err != nil {
		t.Fatalf("el AXML con acentos no se deja leer: %v", err)
	}
	ctx := &contextoVista{axml: axml}
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol del AXML con acentos: %v", err)
	}
	if len(arbol.hijos) != 3 {
		t.Fatalf("el layout trae %d hijos y se escribieron 3", len(arbol.hijos))
	}
	for i, esperada := range []string{textoAcentos, textoEmoji, textoAmp} {
		if leida := ctx.textoDeAtributo(arbol.hijos[i], uriDeRecursos, "text"); leida != esperada {
			t.Errorf("el texto #%d se lee como %q y se escribió %q", i, leida, esperada)
		}
	}

	// y el modo UTF-16 del manifiesto, que tiene que seguir intacto: un prefijo y dos bytes por
	// unidad, también con caracteres fuera del plano base
	manifiesto := manifiestoDelStudio()
	manifiesto.raiz.hijos[1].attrs[0].texto = textoAcentos // application/label
	datosManifiesto, err := escribeAXML(manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto UTF-16 con acentos: %v", err)
	}
	axmlManifiesto, err := leeAXML(datosManifiesto)
	if err != nil {
		t.Fatalf("el manifiesto UTF-16 con acentos no se deja leer: %v", err)
	}
	ctxManifiesto := &contextoVista{axml: axmlManifiesto}
	arbolManifiesto, err := ctxManifiesto.arbol()
	if err != nil {
		t.Fatalf("el árbol del manifiesto: %v", err)
	}
	app := arbolManifiesto.hijo("application")
	if app == nil {
		t.Fatal("el manifiesto no trae application")
	}
	if leida := ctxManifiesto.textoDeAtributo(app, uriDeRecursos, "label"); leida != textoAcentos {
		t.Errorf("el label UTF-16 se lee como %q y se escribió %q", leida, textoAcentos)
	}

	// y dentro de un resources.arsc: el global y las claves son los dos pools UTF-8 de la tabla
	tb := tablaDelStudio()
	tb.global = append(tb.global, textoAcentos, textoEmoji)
	tb.claves = append(tb.claves, textoAcentos, textoEmoji)
	datosTabla, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla con acentos: %v", err)
	}
	tr, err := leeTabla(datosTabla)
	if err != nil {
		t.Fatalf("la tabla con acentos no se deja leer: %v", err)
	}
	if n := len(tr.global.cadenas); n != 3 || tr.global.cadenas[1] != textoAcentos || tr.global.cadenas[2] != textoEmoji {
		t.Errorf("el pool global lee %d cadenas: %q", n, tr.global.cadenas)
	}
	if n := len(tr.claves); n != 5 || tr.claves[3] != textoAcentos || tr.claves[4] != textoEmoji {
		t.Errorf("el pool de claves lee %d cadenas: %q", n, tr.claves)
	}
}

// apkDelStudioConTexto es el apk de los escritores con el texto del TextView y el label del
// manifiesto cambiados, y la misma cadena añadida al pool global de la tabla: los tres sitios por
// los que pasa una cadena de la aplicación.
func apkDelStudioConTexto(t *testing.T, texto string) []byte {
	t.Helper()
	manifiesto := manifiestoDelStudio()
	manifiesto.raiz.hijos[1].attrs[0].texto = texto // application/label
	manifiestoBytes, err := escribeAXML(manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout := layoutDelStudio()
	layout.raiz.hijos[0].attrs[3].texto = texto // TextView/android:text
	layoutBytes, err := escribeAXML(layout)
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	tb := tablaDelStudio()
	tb.global = append(tb.global, texto)
	arsc, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiestoBytes,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layoutBytes,
	})
}

// TestPreviewSobreUnAPKConTextoNoASCII: de extremo a extremo, el mismo camino que hace el comando.
// Con el lector que tomaba el primer prefijo como bytes, este apk —que se arma con los escritores
// del fork, no con aapt2— fallaba al leer la tabla con «does not end with a zero byte». No se
// ejecuta el binario: lo que devolvía rc 2 era este error, y el comando solo lo imprime.
func TestPreviewSobreUnAPKConTextoNoASCII(t *testing.T) {
	apk := apkDelStudioConTexto(t, textoAcentos)
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk con «%s» no se deja leer: %v", textoAcentos, err)
	}
	if m.Raiz == nil || len(m.Raiz.Hijos) != 2 {
		t.Fatalf("el modelo no trae los dos hijos del layout: %+v", m.Raiz)
	}
	if leida := m.Raiz.Hijos[0].Texto; leida != textoAcentos {
		t.Errorf("el TextView se lee como %q y se escribió %q", leida, textoAcentos)
	}
	if m.Paquete != "com.hola.apkcstudio" || m.Layout != "res/layout/main.xml" {
		t.Errorf("paquete %q y layout %q", m.Paquete, m.Layout)
	}
	// el JSON que imprime el comando también lleva el texto. `encoding/json` escapa el `&`, así
	// que se busca la parte acentuada, que es la que el lector partía.
	salida, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(salida), "adiós ñ") {
		t.Errorf("el JSON no trae el texto acentuado: %s", salida)
	}
}
