package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"
)

// La prueba del escritor de AXML (Fase 3-D, parrafo 30). Dos ficheros de aapt2 fijados en base64: el
// AndroidManifest.xml (2000 bytes, pool UTF-16) y el layout (784, pool UTF-8). Lo que produce
// axmlwriter.go se compara byte a byte, y despues se lee con el lector de la 3-B, que es el que va a
// usar el empaquetado.

const uriAndroid = "http://schemas.android.com/apk/res/android"

// layoutDelStudio es el res/layout/main.xml del proyecto, con los ids de atributo que puso aapt2 en
// su XML_RESOURCE_MAP (0x010100af gravity, 0x010100c4 orientation, 0x010100d0 id, 0x010100f4
// layout_width, 0x010100f5 layout_height, 0x0101014f text). Los atributos van en el orden del fuente,
// no en el del binario: el escritor los reordena como aapt2.
func layoutDelStudio() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: 0x010100c4, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "gravity", id: 0x010100af, tipo: tipoEnteroHex, datos: 0x11},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{nombre: "style", tipo: tipoReferencia, datos: 0x01030042},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Hello World!"},
					},
				},
				{
					nombre: "Button",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Press me"},
					},
				},
			},
		},
	}
}

// manifiestoDelStudio es el AndroidManifest.xml binario. Cuatro de sus atributos no estan en el
// fuente: compileSdkVersion, compileSdkVersionCodename, platformBuildVersionCode y
// platformBuildVersionName, que los pone aapt2 en el enlace (34, "14", 34 y 14 entero — el ultimo no
// es cadena, medido con raw -1 y tipo 0x10).
func manifiestoDelStudio() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: false,
		raiz: &elementoEscribir{
			nombre: "manifest",
			linea:  1,
			attrs: []atributoEscribir{
				{nombre: "package", tipo: tipoCadena, texto: "com.hola.apkcstudio"},
				{ns: "android", nombre: "versionCode", id: 0x0101021b, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "versionName", id: 0x0101021c, tipo: tipoCadena, texto: "0.1"},
				{ns: "android", nombre: "compileSdkVersion", id: 0x01010572, tipo: tipoEnteroDec, datos: 34},
				{ns: "android", nombre: "compileSdkVersionCodename", id: 0x01010573, tipo: tipoCadena, texto: "14"},
				{nombre: "platformBuildVersionCode", tipo: tipoEnteroDec, datos: 34},
				{nombre: "platformBuildVersionName", tipo: tipoEnteroDec, datos: 14},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "uses-sdk",
					linea:  6,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "minSdkVersion", id: 0x0101020c, tipo: tipoEnteroDec, datos: 21},
						{ns: "android", nombre: "targetSdkVersion", id: 0x01010270, tipo: tipoEnteroDec, datos: 33},
						{ns: "android", nombre: "maxSdkVersion", id: 0x01010271, tipo: tipoEnteroDec, datos: 33},
					},
				},
				{
					nombre: "application",
					linea:  10,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Hola apkc"},
						{ns: "android", nombre: "extractNativeLibs", id: 0x010104ea, tipo: tipoBooleano, datos: 0},
					},
					hijos: []*elementoEscribir{
						{
							nombre: "activity",
							linea:  13,
							attrs: []atributoEscribir{
								{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: ".MainActivity"},
								{ns: "android", nombre: "theme", id: 0x01010000, tipo: tipoReferencia, datos: 0x7f030000},
								{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Hola apkc"},
								{ns: "android", nombre: "exported", id: 0x01010010, tipo: tipoBooleano, datos: 0xffffffff},
							},
							hijos: []*elementoEscribir{
								{
									nombre: "intent-filter",
									linea:  19,
									hijos: []*elementoEscribir{
										{
											nombre: "action",
											linea:  20,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "android.intent.action.MAIN"},
											},
										},
										{
											nombre: "category",
											linea:  21,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "android.intent.category.LAUNCHER"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestEscribeAXMLContraFixture(t *testing.T) {
	casos := []struct {
		nombre  string
		fixture string
		mide    int
		doc     documentoAXML
	}{
		{"layout", axmlLayout, 784, layoutDelStudio()},
		{"manifiesto", axmlManifiesto, 2000, manifiestoDelStudio()},
	}
	for _, c := range casos {
		fijado, err := base64.StdEncoding.DecodeString(c.fixture)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if len(fijado) != c.mide {
			t.Fatalf("%s: el fixture mide %d bytes, no %d", c.nombre, len(fijado), c.mide)
		}
		nuestro, err := escribeAXML(c.doc)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if len(nuestro) != len(fijado) {
			t.Errorf("%s: escribimos %d bytes y aapt2 %d", c.nombre, len(nuestro), len(fijado))
		}
		if !bytes.Equal(nuestro, fijado) {
			diferentes := 0
			for i := range fijado {
				if i < len(nuestro) && nuestro[i] != fijado[i] {
					if diferentes < 4 {
						t.Errorf("%s: byte #%d es %#02x y aapt2 escribe %#02x", c.nombre, i, nuestro[i], fijado[i])
					}
					diferentes++
				}
			}
			t.Errorf("%s: %d bytes distintos de %d", c.nombre, diferentes, len(fijado))
		}

		// y el lector de la 3-B tiene que reconocer lo nuestro: mismo numero de trozos de nodo, la
		// misma pool y los mismos nombres en el mismo orden
		a, err := leeAXML(nuestro)
		if err != nil {
			t.Fatalf("%s: no se deja leer: %v", c.nombre, err)
		}
		b, err := leeAXML(fijado)
		if err != nil {
			t.Fatalf("%s: el fixture no se deja leer: %v", c.nombre, err)
		}
		if len(a.nodos) != len(b.nodos) {
			t.Errorf("%s: %d trozos de nodo escritos, %d en aapt2", c.nombre, len(a.nodos), len(b.nodos))
		}
		if len(a.tabla.cadenas) != len(b.tabla.cadenas) {
			t.Errorf("%s: pool de %d cadenas escritas contra %d de aapt2",
				c.nombre, len(a.tabla.cadenas), len(b.tabla.cadenas))
		}
		for i := range b.nodos {
			if i >= len(a.nodos) {
				break
			}
			if a.nodos[i].tipo != b.nodos[i].tipo || a.nodos[i].nombre != b.nodos[i].nombre ||
				a.nodos[i].ns != b.nodos[i].ns || a.nodos[i].linea != b.nodos[i].linea {
				t.Errorf("%s: nodo #%d %q linea %d, aapt2 %q linea %d",
					c.nombre, i, a.nombreDe(a.nodos[i].nombre), a.nodos[i].linea,
					b.nombreDe(b.nodos[i].nombre), b.nodos[i].linea)
			}
			if len(a.nodos[i].attrs) != len(b.nodos[i].attrs) {
				t.Errorf("%s: nodo #%d con %d atributos escritos y %d en aapt2",
					c.nombre, i, len(a.nodos[i].attrs), len(b.nodos[i].attrs))
			}
		}
		if len(a.idsMapa) != len(b.idsMapa) {
			t.Errorf("%s: mapa de %d ids escrito contra %d de aapt2", c.nombre, len(a.idsMapa), len(b.idsMapa))
		}

		// el instrumento de la medicion exterior: con APKC_ESCRIBE_AXML se guarda lo escrito, con el
		// nombre que lleva el fichero dentro del apk, para cotejarlo con `aapt2 dump xmltree`
		if dir := os.Getenv("APKC_ESCRIBE_AXML"); dir != "" {
			nombre := "AndroidManifest.xml"
			if c.nombre == "layout" {
				nombre = "main.xml"
			}
			if err := os.WriteFile(dir+"/"+nombre, nuestro, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestEscribeAXMLRechaza(t *testing.T) {
	base := layoutDelStudio()

	malo := layoutDelStudio()
	malo.raiz.hijos[1].attrs[0].id = 0 // android:id sin id resuelto
	if _, err := escribeAXML(malo); err == nil {
		t.Error("un atributo de namespace sin id fue aceptado")
	}

	malo = layoutDelStudio()
	malo.raiz.hijos[0].attrs[0].id = 0x01010000 // style, que no tiene namespace, con id
	if _, err := escribeAXML(malo); err == nil {
		t.Error("un atributo sin namespace con id de recurso fue aceptado")
	}

	malo = layoutDelStudio()
	malo.raiz.attrs[0].ns = "tools" // un prefijo que nadie declaro
	if _, err := escribeAXML(malo); err == nil {
		t.Error("un prefijo no declarado fue aceptado")
	}

	malo = manifiestoDelStudio()
	// dos ids distintos para el MISMO nombre: android:name vale 0x01010003 y aqui el de la activity
	// pasa a llevar el de android:label, que es otro atributo
	malo.raiz.hijos[1].hijos[0].attrs[2].id = 0x01010003
	if _, err := escribeAXML(malo); err == nil {
		t.Error("dos ids distintos para el mismo nombre de atributo fueron aceptados")
	}

	malo = layoutDelStudio()
	malo.raiz = nil
	if _, err := escribeAXML(malo); err == nil {
		t.Error("un documento sin raiz fue aceptado")
	}

	malo = layoutDelStudio()
	malo.ns[0].uri = ""
	if _, err := escribeAXML(malo); err == nil {
		t.Error("una declaracion de namespace sin URI fue aceptada")
	}

	if _, err := escribeAXML(base); err != nil {
		t.Errorf("el layout valido fue rechazado: %v", err)
	}
}
