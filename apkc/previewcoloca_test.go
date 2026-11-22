package main

// La colocación real del layout: lo que el modelo tenía que dejar de estimar a ojo. Dos oráculos y
// dos clases de prueba:
//   - los atributos (gravity, padding, márgenes, medidas) se comparan con lo que imprime
//     `aapt2 dump xmltree` sobre un apk de verdad, que es donde se midieron los valores complejos de
//     una dimensión (1.5dp = 0x00c00021, 16px = 0x00001000, 16dp = 0x00001001);
//   - los rectángulos se comparan con la regla de LinearLayout/FrameLayout aplicada a mano sobre los
//     atributos que el fuente declara. No hay aparato en esta máquina, así que la geometría no tiene
//     oráculo del framework: lo que se fija aquí es la cuenta documentada (medir y después colocar,
//     como `measureVertical`/`layoutVertical`), no una medida del dispositivo.
//
// Los apk de las pruebas se arman en memoria con los escritores del fork (regla 2: sin binarios
// versionados) y los dos caminos del enlace se prueban por separado: la dimensión como
// TYPE_DIMENSION (lo que escribe aapt2) y como cadena `"16dp"` (lo que deja el enlazador en Go).

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ids de atributo del framework, los mismos que imprime `aapt2 dump xmltree` entre paréntesis.
const (
	idAttrGravity        = 0x010100af
	idAttrLayoutGravity  = 0x010100b3
	idAttrOrientation    = 0x010100c4
	idAttrId             = 0x010100d0
	idAttrPadding        = 0x010100d5
	idAttrLayoutWidth    = 0x010100f4
	idAttrLayoutHeight   = 0x010100f5
	idAttrLayoutMargin   = 0x010100f6
	idAttrLayoutMarginAb = 0x010100fa
	idAttrWeightSum      = 0x01010128
	idAttrLayoutWeight   = 0x01010181
	idAttrText           = 0x0101014f
)

// atributoDim arma un atributo con una dimensión en la forma que elija el caso: el entero complejo
// de aapt2 o la cadena con sufijo del enlazador en Go.
func atributoDim(nombre string, id uint32, tipo byte, datos uint32, texto string) atributoEscribir {
	return atributoEscribir{ns: "android", nombre: nombre, id: id, tipo: tipo, datos: datos, texto: texto}
}

// layoutHola es el res/layout/main.xml de la plantilla Hola del studio: LinearLayout vertical,
// match_parent, gravity="center" y un TextView wrap_content con el texto literal.
func layoutHola() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: idAttrOrientation, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "gravity", id: idAttrGravity, tipo: tipoEnteroHex, datos: 0x11},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Hello World!"},
					},
				},
			},
		},
	}
}

// layoutContador es el res/layout/main.xml de la plantilla Contador del studio: el TextView del total
// y el Button, con el mismo gravity="center".
func layoutContadorDelStudio() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: idAttrOrientation, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "gravity", id: idAttrGravity, tipo: tipoEnteroHex, datos: 0x11},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: idAttrId, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Pulsos: 0"},
					},
				},
				{
					nombre: "Button",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: idAttrId, tipo: tipoReferencia, datos: 0x7f010001},
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Pulsar"},
					},
				},
			},
		},
	}
}

// apkConLayout arma el apk del studio con el layout dado y su tabla.
func apkConLayout(t *testing.T, layout documentoAXML) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layoutBytes, err := escribeAXML(layout)
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
		"res/layout/main.xml": layoutBytes,
	})
}

// TestPreviewColocaLaPlantillaHola: el modelo de la plantilla Hola con la colocación real. El
// TextView mide 12 caracteres por 8 dp más el relleno del texto (112) y 24 de alto, y gravity="center"
// lo deja en el medio de 360x640: x = (360-112)/2 = 124, y = (640-24)/2 = 308.
func TestPreviewColocaLaPlantillaHola(t *testing.T) {
	m, err := modeloVistaDeAPK(apkConLayout(t, layoutHola()), "")
	if err != nil {
		t.Fatalf("el apk de la plantilla Hola no se deja leer: %v", err)
	}
	if m.Raiz == nil || m.Raiz.Gravity != "center" {
		t.Fatalf("la raíz del layout Hola es %+v y declara gravity=center", m.Raiz)
	}
	if r := m.Raiz.Rect; r != (rectVista{X: 0, Y: 0, Ancho: 360, Alto: 640}) {
		t.Errorf("la raíz ocupa %+v", r)
	}
	if len(m.Raiz.Hijos) != 1 {
		t.Fatalf("el layout Hola trae %d hijos y tiene uno", len(m.Raiz.Hijos))
	}
	texto := m.Raiz.Hijos[0]
	if texto.Tipo != "TextView" || texto.Texto != "Hello World!" {
		t.Errorf("el hijo es %q con texto %q", texto.Tipo, texto.Texto)
	}
	if r := texto.Rect; r != (rectVista{X: 124, Y: 308, Ancho: 112, Alto: 24}) {
		t.Errorf("el TextView de Hola se coloca en %+v: se esperaba {124 308 112 24}", r)
	}
}

// TestPreviewColocaLaPlantillaContador: el TextView del total y el Button, los dos centrados. El
// bloque mide 24+48=72, así que empieza en (640-72)/2 = 284.
func TestPreviewColocaLaPlantillaContador(t *testing.T) {
	m, err := modeloVistaDeAPK(apkConLayout(t, layoutContadorDelStudio()), "")
	if err != nil {
		t.Fatalf("el apk de la plantilla Contador no se deja leer: %v", err)
	}
	if len(m.Raiz.Hijos) != 2 {
		t.Fatalf("el Contador trae %d hijos", len(m.Raiz.Hijos))
	}
	total, boton := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	// «Pulsos: 0» son 9 caracteres: 9*8+16 = 88
	if r := total.Rect; r != (rectVista{X: 136, Y: 284, Ancho: 88, Alto: 24}) {
		t.Errorf("el TextView del total se coloca en %+v: se esperaba {136 284 88 24}", r)
	}
	// «Pulsar» son 6 caracteres: 6*8+16 = 64, y el Button mide 48 de alto
	if r := boton.Rect; r != (rectVista{X: 148, Y: 308, Ancho: 64, Alto: 48}) {
		t.Errorf("el Button se coloca en %+v: se esperaba {148 308 64 48}", r)
	}
}

// TestPreviewColocaConPaddingMargenYPeso: el layout del que salieron las medidas de aapt2, con
// padding de 16 dp, un margen inferior de 8 dp, un ancho fijo de 200 dp, layout_gravity y peso. La
// cuenta a mano: el interior es 328x608; el TextView con peso se queda con 608-24-8 = 576 de alto y
// el bloque llena el interior, así que el primero va en y=16 y el segundo en 48.
func TestPreviewColocaConPaddingMargenYPeso(t *testing.T) {
	// el mismo contenido escrito con las dos formas de dimensión del enlace
	casos := []struct {
		nombre string
		tipo   byte
		// las tres dimensiones del caso: padding 16dp, margen 8dp y ancho 200dp
		datos [3]uint32
		texto [3]string
	}{
		{nombre: "TYPE_DIMENSION de aapt2", tipo: tipoDimension, datos: [3]uint32{0x00001001, 0x00000801, 0x0000c801}},
		{nombre: "cadena del enlazador en Go", tipo: tipoCadena, texto: [3]string{"16dp", "8dp", "200dp"}},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			layout := documentoAXML{
				ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
				utf8: true,
				raiz: &elementoEscribir{
					nombre: "LinearLayout",
					linea:  2,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "orientation", id: idAttrOrientation, tipo: tipoEnteroDec, datos: 1},
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xffffffff},
						{ns: "android", nombre: "gravity", id: idAttrGravity, tipo: tipoEnteroHex, datos: 0x11},
						atributoDim("padding", idAttrPadding, caso.tipo, caso.datos[0], caso.texto[0]),
					},
					hijos: []*elementoEscribir{
						{
							nombre: "TextView",
							linea:  8,
							attrs: []atributoEscribir{
								{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
								{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
								atributoDim("layout_marginBottom", idAttrLayoutMarginAb, caso.tipo, caso.datos[1], caso.texto[1]),
								{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Hola"},
							},
						},
						{
							nombre: "TextView",
							linea:  14,
							attrs: []atributoEscribir{
								{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: caso.tipo, datos: caso.datos[2], texto: caso.texto[2]},
								{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
								{ns: "android", nombre: "layout_weight", id: idAttrLayoutWeight, tipo: tipoFlotante, datos: 0x3f800000},
								{ns: "android", nombre: "layout_gravity", id: idAttrLayoutGravity, tipo: tipoEnteroHex, datos: 0x01},
								{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Mundo"},
							},
						},
					},
				},
			}
			m, err := modeloVistaDeAPK(apkConLayout(t, layout), "")
			if err != nil {
				t.Fatalf("el apk del layout con relleno no se deja leer: %v", err)
			}
			raiz := m.Raiz
			if raiz.Padding == nil || *raiz.Padding != (ladosVista{16, 16, 16, 16}) {
				t.Errorf("el padding de la raíz es %+v", raiz.Padding)
			}
			primero, segundo := raiz.Hijos[0], raiz.Hijos[1]
			if primero.Margen == nil || *primero.Margen != (ladosVista{0, 0, 0, 8}) {
				t.Errorf("el margen del primer hijo es %+v", primero.Margen)
			}
			if primero.Rect != (rectVista{X: 156, Y: 16, Ancho: 48, Alto: 24}) {
				t.Errorf("el primer TextView se coloca en %+v: se esperaba {156 16 48 24}", primero.Rect)
			}
			if segundo.Ancho != "200dp" || segundo.Peso != 1 {
				t.Errorf("el segundo declara %q de ancho y peso %v", segundo.Ancho, segundo.Peso)
			}
			if segundo.Rect != (rectVista{X: 80, Y: 48, Ancho: 200, Alto: 576}) {
				t.Errorf("el segundo TextView se coloca en %+v: se esperaba {80 48 200 576}", segundo.Rect)
			}
		})
	}
}

// TestPreviewColocaFrameLayoutPorGravity: un FrameLayout con dos hijos, uno centrado y otro
// abajo a la derecha, con las medidas que aapt2 escribió: 1.5dp y 12.5dp (radix 2), 12sp y 10pt.
func TestPreviewColocaFrameLayoutPorGravity(t *testing.T) {
	layout := documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "FrameLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xffffffff},
				atributoDim("padding", idAttrPadding, tipoDimension, 0x00000801, ""),
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  7,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_gravity", id: idAttrLayoutGravity, tipo: tipoEnteroHex, datos: 0x11},
						atributoDim("layout_width", idAttrLayoutWidth, tipoDimension, 0x00c00021, ""),
						atributoDim("layout_height", idAttrLayoutHeight, tipoDimension, 0x06400021, ""),
						atributoDim("layout_margin", idAttrLayoutMargin, tipoDimension, 0x00000401, ""),
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Centro"},
					},
				},
				{
					nombre: "TextView",
					linea:  16,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_gravity", id: idAttrLayoutGravity, tipo: tipoEnteroHex, datos: 0x55},
						atributoDim("layout_width", idAttrLayoutWidth, tipoDimension, 0x00000c02, ""),
						atributoDim("layout_height", idAttrLayoutHeight, tipoDimension, 0x00000a03, ""),
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Unidades"},
					},
				},
			},
		},
	}
	m, err := modeloVistaDeAPK(apkConLayout(t, layout), "")
	if err != nil {
		t.Fatalf("el apk del FrameLayout no se deja leer: %v", err)
	}
	centro, esquina := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	// 1.5dp se redondea a 2 y 12.5dp a 13; el interior es 344x624, que con el margen de 4 da
	// x = 8 + (344-2)/2 = 179 e y = 8 + (624-13)/2 = 313
	if centro.Rect != (rectVista{X: 179, Y: 313, Ancho: 2, Alto: 13}) {
		t.Errorf("el hijo centrado se coloca en %+v: se esperaba {179 313 2 13}", centro.Rect)
	}
	// 12sp son 12 dp y 10pt son 22 dp (10*160/72 redondeado)
	if esquina.Rect != (rectVista{X: 340, Y: 610, Ancho: 12, Alto: 22}) {
		t.Errorf("el hijo de la esquina se coloca en %+v: se esperaba {340 610 12 22}", esquina.Rect)
	}
	if centro.Ancho != "2dp" || centro.Alto != "13dp" {
		t.Errorf("las medidas del hijo centrado se leen como %q x %q", centro.Ancho, centro.Alto)
	}
}

// TestPreviewDecodificaDimensionesDeAapt2: los enteros complejos que aapt2 escribió de verdad en el
// xmltree, con la unidad y el radix que llevan. Se midieron con `aapt2 dump xmltree` y el volcado del
// lector sobre el mismo apk: `16dp` 0x00001001, `8dp` 0x00000801, `200dp` 0x0000c801, `16px`
// 0x00001000, `1.5dp` 0x00c00021, `12.5dp` 0x06400021, `12sp` 0x00000c02, `10pt` 0x00000a03 y
// `1in` 0x00000104.
func TestPreviewDecodificaDimensionesDeAapt2(t *testing.T) {
	casos := []struct {
		datos uint32
		dp    int
		que   string
	}{
		{0x00001001, 16, "16dp"},
		{0x00000801, 8, "8dp"},
		{0x0000c801, 200, "200dp"},
		{0x00001000, 16, "16px a 160 dpi"},
		{0x00c00021, 2, "1.5dp redondeado"},
		{0x06400021, 13, "12.5dp redondeado"},
		{0x00000c02, 12, "12sp con fontScale 1"},
		{0x00000a03, 22, "10pt"},
		{0x00000104, 160, "1in"},
		{0x00000505, 31, "5mm"},
	}
	for _, c := range casos {
		dp, ok := dpDeComplejo(c.datos)
		if !ok || dp != c.dp {
			t.Errorf("%s (%#010x): %d dp (ok=%v), se esperaba %d", c.que, c.datos, dp, ok, c.dp)
		}
	}
	// y las mismas medidas escritas como cadena por el enlazador en Go dan el mismo número
	cadenas := []struct {
		texto string
		dp    int
	}{
		{"16dp", 16}, {"16dip", 16}, {"8dp", 8}, {"200dp", 200},
		{"16px", 16}, {"12sp", 12}, {"10pt", 22}, {"1in", 160}, {"5mm", 31},
	}
	for _, c := range cadenas {
		dp, ok := dpDeTexto(c.texto)
		if !ok || dp != c.dp {
			t.Errorf("%q: %d dp (ok=%v), se esperaba %d", c.texto, dp, ok, c.dp)
		}
	}
	if _, ok := dpDeTexto("16"); ok {
		t.Error("una cadena sin unidad no es una dimensión")
	}
}

// TestPreviewResuelveElTextoDeUnaCadena: el contrato del que avisa la queja del usuario. Un
// `android:text="@string/saludo"` tiene que salir como el texto de la tabla, no como `@0x…` ni como
// `string/saludo`. La tabla lleva la cadena en la configuración base y otra en `es`: la que se lee es
// la base, que es la que cargaría un teléfono en español neutro sin calificador.
func TestPreviewResuelveElTextoDeUnaCadena(t *testing.T) {
	tb := tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.apkcstudio",
		global:        []string{"res/layout/main.xml", "¡Hola, mundo!"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout", "string"},
		nombresUTF8:   true,
		claves:        []string{"pulsar", "main", "saludo"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{id: 1, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: tipoBooleano, datos: 0}},
			}}}},
			{id: 2, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 1, valor: valorEscribir{tipo: tipoCadena, datos: 0}},
			}}}},
			{id: 3, specFlags: []uint32{0}, bloques: []bloqueEscribir{
				{entradas: []entradaEscribir{{clave: 2, valor: valorEscribir{tipo: tipoCadena, datos: 1}}}},
				{config: "es", entradas: []entradaEscribir{{clave: 2, valor: valorEscribir{tipo: tipoCadena, datos: 1}}}},
			}},
		},
	}
	arsc, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla con la cadena: %v", err)
	}
	layout := layoutHola()
	// el TextView de Hola, con el texto como referencia a string/saludo (0x7f030000)
	layout.raiz.hijos[0].attrs[2] = atributoEscribir{
		ns: "android", nombre: "text", id: idAttrText, tipo: tipoReferencia, datos: 0x7f030000,
	}
	layoutBytes, err := escribeAXML(layout)
	if err != nil {
		t.Fatalf("el layout con la referencia: %v", err)
	}
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layoutBytes,
	})
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk con @string no se deja leer: %v", err)
	}
	texto := m.Raiz.Hijos[0]
	if texto.Texto != "¡Hola, mundo!" {
		t.Errorf("android:text=@string/saludo se lee como %q y la tabla dice «¡Hola, mundo!»", texto.Texto)
	}
}

// TestPreviewDiceLoMismoQueAapt2: el oráculo externo sobre los bytes de aapt2. Se monta el apk del
// fixture (manifiesto, tabla y layout tal como los escribió el aapt2 del SDK) y se pide su xmltree:
// los atributos que el modelo consume —gravity, layout_width, layout_height y text— tienen que ser
// los que el modelo dice, y el rectángulo el que sale de esos mismos atributos.
func TestPreviewDiceLoMismoQueAapt2(t *testing.T) {
	aapt2 := aapt2DeLaMaquina(t)
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": mustBase64(t, axmlManifiesto, 2000),
		"resources.arsc":      mustBase64(t, arscTabla, 1088),
		"res/layout/main.xml": mustBase64(t, axmlLayout, 784),
	})
	ruta := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(ruta, apk, 0o644); err != nil {
		t.Fatalf("el apk temporal: %v", err)
	}
	salida, err := exec.Command(aapt2, "dump", "xmltree", ruta, "--file", "res/layout/main.xml").Output()
	if err != nil {
		t.Fatalf("aapt2 dump xmltree: %v", err)
	}
	volcado := string(salida)

	for _, esperado := range []string{
		":gravity(0x010100af)=0x00000011",
		":layout_width(0x010100f4)=-1",
		":layout_height(0x010100f5)=-1",
		`text(0x0101014f)="Hello World!" (Raw: "Hello World!")`,
	} {
		if !strings.Contains(volcado, esperado) {
			t.Errorf("el xmltree de aapt2 no trae %q:\n%s", esperado, volcado)
		}
	}

	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk de aapt2 no se deja leer: %v", err)
	}
	if m.Raiz.Gravity != "center" {
		t.Errorf("aapt2 dice gravity=0x11 y el modelo %q", m.Raiz.Gravity)
	}
	if m.Raiz.Hijos[0].Texto != "Hello World!" {
		t.Errorf("aapt2 dice text=\"Hello World!\" y el modelo %q", m.Raiz.Hijos[0].Texto)
	}
	if r := m.Raiz.Hijos[0].Rect; r != (rectVista{X: 124, Y: 284, Ancho: 112, Alto: 24}) {
		t.Errorf("con gravity=0x11 el hijo se coloca en %+v: se esperaba {124 284 112 24}", r)
	}
}

// layoutDePesos arma un LinearLayout vertical match_parent con los atributos de raíz que se pidan y
// los hijos dados: la sonda de las reglas de layout_weight de measureVertical.
func layoutDePesos(attrsRaiz []atributoEscribir, hijos ...*elementoEscribir) documentoAXML {
	base := []atributoEscribir{
		{ns: "android", nombre: "orientation", id: idAttrOrientation, tipo: tipoEnteroDec, datos: 1},
		{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
		{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xffffffff},
	}
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs:  append(base, attrsRaiz...),
			hijos:  hijos,
		},
	}
}

// hijoDeTexto arma un TextView de ancho match_parent con el alto que se le diga (0x1 es `0dp` en el
// entero complejo de aapt2) y el peso. El peso va como flotante, que es como lo escribe aapt2
// (`layout_weight(0x01010181)=1`).
func hijoDeTexto(alto uint32, peso int, texto string) *elementoEscribir {
	return &elementoEscribir{
		nombre: "TextView",
		linea:  8,
		attrs: []atributoEscribir{
			{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
			atributoDim("layout_height", idAttrLayoutHeight, tipoDimension, alto, ""),
			{ns: "android", nombre: "layout_weight", id: idAttrLayoutWeight, tipo: tipoFlotante, datos: math.Float32bits(float32(peso))},
			{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: texto},
		},
	}
}

// TestPreviewReparteElPesoDeUnHijoDeCero: el patrón «0dp + layout_weight», el más común. La raíz de
// 640 con un hijo de 24 dp y otro de 0dp con peso 1 deja al segundo 640-24 = 616 de alto, no 0: el
// tamaño declarado no puede ganar al EXACTLY que reparte el peso (measureVertical mide al hijo del
// peso con makeMeasureSpec(share, EXACTLY)).
func TestPreviewReparteElPesoDeUnHijoDeCero(t *testing.T) {
	m, err := modeloVistaDeAPK(apkConLayout(t, layoutDePesos(nil,
		hijoDeTexto(0x00001801, 0, "AAA"), // 24dp sin peso
		hijoDeTexto(0x00000001, 1, "BBB"), // 0dp con peso 1
	)), "")
	if err != nil {
		t.Fatalf("el apk del peso 0dp no se deja leer: %v", err)
	}
	primero, segundo := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if primero.Rect != (rectVista{X: 0, Y: 0, Ancho: 360, Alto: 24}) {
		t.Errorf("el primer hijo se coloca en %+v: se esperaba {0 0 360 24}", primero.Rect)
	}
	if segundo.Rect != (rectVista{X: 0, Y: 24, Ancho: 360, Alto: 616}) {
		t.Errorf("el hijo de 0dp con peso 1 ocupa %+v: se esperaba {0 24 360 616}", segundo.Rect)
	}
}

// TestPreviewReparteElExcesoNegativo: dos hijos que no caben (400+300 en 640) con peso 1 cada uno.
// El bucle de measureVertical admite remainingExcess negativo y reparte el recorte: 400-30 = 370 y
// 300-30 = 270, que suman 640. El modelo no puede dejarlos en 700.
func TestPreviewReparteElExcesoNegativo(t *testing.T) {
	m, err := modeloVistaDeAPK(apkConLayout(t, layoutDePesos(nil,
		hijoDeTexto(0x00019001, 1, "A"), // 400dp
		hijoDeTexto(0x00012c01, 1, "B"), // 300dp
	)), "")
	if err != nil {
		t.Fatalf("el apk del exceso negativo no se deja leer: %v", err)
	}
	primero, segundo := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if primero.Rect != (rectVista{X: 0, Y: 0, Ancho: 360, Alto: 370}) {
		t.Errorf("el primer hijo se coloca en %+v: se esperaba {0 0 360 370}", primero.Rect)
	}
	if segundo.Rect != (rectVista{X: 0, Y: 370, Ancho: 360, Alto: 270}) {
		t.Errorf("el segundo hijo se coloca en %+v: se esperaba {0 370 360 270}", segundo.Rect)
	}
	if fondo := segundo.Rect.Y + segundo.Rect.Alto; fondo > dpPantallaAlto {
		t.Errorf("los hijos llegan a %d y la pantalla tiene %d: el recorte no se repartió", fondo, dpPantallaAlto)
	}
}

// TestPreviewRespetaWeightSum: android:weightSum manda como denominador. Con weightSum=2 y un solo
// hijo de peso 1, el hijo ocupa la mitad del hueco: 640/2 = 320, no los 640 que daría la suma de pesos.
func TestPreviewRespetaWeightSum(t *testing.T) {
	m, err := modeloVistaDeAPK(apkConLayout(t, layoutDePesos(
		[]atributoEscribir{{ns: "android", nombre: "weightSum", id: idAttrWeightSum, tipo: tipoEnteroDec, datos: 2}},
		hijoDeTexto(0x00000001, 1, "Medio"),
	)), "")
	if err != nil {
		t.Fatalf("el apk con weightSum no se deja leer: %v", err)
	}
	if r := m.Raiz.Hijos[0].Rect; r != (rectVista{X: 0, Y: 0, Ancho: 360, Alto: 320}) {
		t.Errorf("con weightSum=2 el hijo ocupa %+v: se esperaba {0 0 360 320}", r)
	}
}

// TestPreviewReparteElPesoEnSecuencia: el reparto es secuencial, como el bucle de measureVertical:
// cada hijo descuenta su parte del exceso y su peso del denominador. Con pesos 1 y 2 sobre 640, el
// primero se lleva 640/3 = 213 y el segundo los 427 que quedan; un reparto en paralelo daría 426.
func TestPreviewReparteElPesoEnSecuencia(t *testing.T) {
	m, err := modeloVistaDeAPK(apkConLayout(t, layoutDePesos(nil,
		hijoDeTexto(0x00000001, 1, "Uno"),
		hijoDeTexto(0x00000001, 2, "Dos"),
	)), "")
	if err != nil {
		t.Fatalf("el apk del reparto secuencial no se deja leer: %v", err)
	}
	primero, segundo := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if primero.Rect != (rectVista{X: 0, Y: 0, Ancho: 360, Alto: 213}) {
		t.Errorf("el primero se coloca en %+v: se esperaba {0 0 360 213}", primero.Rect)
	}
	if segundo.Rect != (rectVista{X: 0, Y: 213, Ancho: 360, Alto: 427}) {
		t.Errorf("el segundo se coloca en %+v: se esperaba {0 213 360 427}", segundo.Rect)
	}
}

// aapt2DeLaMaquina busca el aapt2 del SDK (o el del PATH) para las pruebas que lo usan como oráculo.
// Sin él la prueba se salta: el resto de la suite no depende de un SDK.
func aapt2DeLaMaquina(t *testing.T) string {
	t.Helper()
	if ruta, err := exec.LookPath("aapt2"); err == nil {
		return ruta
	}
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		t.Skip("sin aapt2 en el PATH ni ANDROID_HOME: no hay oráculo externo")
	}
	raices, err := filepath.Glob(filepath.Join(sdk, "build-tools", "*", "aapt2"))
	if err != nil || len(raices) == 0 {
		t.Skipf("no hay aapt2 en %s/build-tools", sdk)
	}
	sort.Strings(raices)
	return raices[len(raices)-1]
}
