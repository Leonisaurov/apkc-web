package main

// Los recursos que un AAR puede traer y el enlazador no modelaba: un drawable XML (`<shape>` o
// `<vector>`), un `anim` y un `font`. Se enlazan con la misma regla que los layouts —AXML escrito
// como documento y entrada de la tabla con la ruta—, y un `font` binario (.ttf/.otf) viaja tal cual.
// Además, un value del proyecto y otro del AAR con el mismo tipo, nombre y configuración se resuelven
// por prioridad determinista: gana el proyecto, sin depender del orden en que se leyeron las islas.
//
// La segunda mitad comprueba lo que ve la vista previa: un `android:background="@drawable/x"` cuyo
// XML es un `<shape><solid android:color>` se publica como el color de relleno del nodo; un
// `<layer-list>` con un `<solid>` dentro NO, porque solo la raíz `<shape>` es un fondo único.

import (
	"testing"
)

const islaAmpliada = "a1b2c3d4e5f6"

// proyectoConRecursosDeAAR arma el árbol: values del proyecto y del AAR con un nombre repetido,
// un layout, y las tres carpetas nuevas dentro de la isla.
func proyectoConRecursosDeAAR(t *testing.T) string {
	t.Helper()
	return proyecto(t, map[string]string{
		"AndroidManifest.xml": manifestMinimo,
		"res/values/app.xml": `<resources>
    <string name="app_name">Proyecto</string>
</resources>`,
		"res/layout/main.xml": `<?xml version="1.0" encoding="utf-8"?>
<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="match_parent">
    <TextView android:id="@+id/etiqueta"
        android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:text="@string/app_name" android:background="@drawable/fondo_deps" />
    <TextView android:id="@+id/capas"
        android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:text="@string/solo_deps" android:background="@drawable/capas_deps" />
</FrameLayout>`,
		"res/__deps/" + islaAmpliada + "/values/deps.xml": `<resources>
    <string name="app_name">AAR</string>
    <string name="solo_deps">solo del AAR</string>
</resources>`,
		"res/__deps/" + islaAmpliada + "/drawable/fondo_deps.xml": `<?xml version="1.0" encoding="utf-8"?>
<shape xmlns:android="http://schemas.android.com/apk/res/android" android:shape="rectangle">
    <solid android:color="#112233" />
</shape>`,
		"res/__deps/" + islaAmpliada + "/drawable/capas_deps.xml": `<?xml version="1.0" encoding="utf-8"?>
<layer-list xmlns:android="http://schemas.android.com/apk/res/android">
    <item><shape><solid android:color="#445566" /></shape></item>
</layer-list>`,
		"res/__deps/" + islaAmpliada + "/anim/fade_in.xml": `<?xml version="1.0" encoding="utf-8"?>
<alpha xmlns:android="http://schemas.android.com/apk/res/android"
    android:fromAlpha="0.0" android:toAlpha="1.0" android:duration="300" />`,
		"res/__deps/" + islaAmpliada + "/font/familia.xml": `<?xml version="1.0" encoding="utf-8"?>
<font-family xmlns:android="http://schemas.android.com/apk/res/android">
    <font android:font="@font/deps_fuente" />
</font-family>`,
		"res/__deps/" + islaAmpliada + "/font/deps_fuente.ttf": "TTF falso",
	})
}

// cadenaEnLaTabla busca un texto en el pool global de la tabla escrita.
func cadenaEnLaTabla(tabla tablaEscribir, texto string) bool {
	for _, s := range tabla.global {
		if s == texto {
			return true
		}
	}
	return false
}

// TestPreviewRecursosDeAarDrawableAnimYFont: la puerta y el enlace. Los tres tipos entran, el
// drawable y el anim como AXML, el font XML como AXML y el .ttf como fichero; y los tipos nuevos
// aparecen en la tabla.
func TestPreviewRecursosDeAarDrawableAnimYFont(t *testing.T) {
	jar := jarDelSDK(t)
	m, err := marcoDelAndroidJar(jar)
	if err != nil {
		t.Fatalf("la tabla del framework: %v", err)
	}
	dir := proyectoConRecursosDeAAR(t)
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatalf("las fuentes del proyecto: %v", err)
	}

	enLayouts := map[string]bool{}
	for _, f := range layouts {
		enLayouts[f.nombre] = true
		if f.deDependencia != (f.nombre != "res/layout/main.xml") {
			t.Errorf("%s sale con deDependencia=%v", f.nombre, f.deDependencia)
		}
	}
	for _, quiero := range []string{"res/drawable/fondo_deps.xml", "res/drawable/capas_deps.xml",
		"res/anim/fade_in.xml", "res/font/familia.xml"} {
		if !enLayouts[quiero] {
			t.Errorf("%s no salió como documento AXML; layouts=%v", quiero, enLayouts)
		}
	}
	enFicheros := map[string]bool{}
	for _, f := range ficheros {
		enFicheros[f.nombre] = true
		if !f.deDependencia {
			t.Errorf("%s debería venir de la isla", f.nombre)
		}
	}
	if !enFicheros["res/font/deps_fuente.ttf"] {
		t.Errorf("el .ttf de la isla no salió como fichero; ficheros=%v", enFicheros)
	}

	salida, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, m,
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace: %v", err)
	}
	if doc := salida.layouts["res/drawable/fondo_deps.xml"]; doc.raiz == nil || doc.raiz.nombre != "shape" {
		t.Fatalf("el drawable XML no se escribió como un <shape>: %+v", doc.raiz)
	}
	if doc := salida.layouts["res/drawable/capas_deps.xml"]; doc.raiz == nil || doc.raiz.nombre != "layer-list" {
		t.Fatalf("el layer-list no se escribió como tal: %+v", doc.raiz)
	}
	if doc := salida.layouts["res/anim/fade_in.xml"]; doc.raiz == nil || doc.raiz.nombre != "alpha" {
		t.Fatalf("el anim no se escribió como un <alpha>: %+v", doc.raiz)
	}
	if doc := salida.layouts["res/font/familia.xml"]; doc.raiz == nil || doc.raiz.nombre != "font-family" {
		t.Fatalf("el font XML no se escribió como un <font-family>: %+v", doc.raiz)
	}
	if _, ok := salida.ficheros["res/font/deps_fuente.ttf"]; !ok {
		t.Fatalf("el .ttf no viaja como fichero: %v", salida.ficheros)
	}
	tipos := map[string]bool{}
	for _, nombre := range salida.tabla.nombresTipo {
		tipos[nombre] = true
	}
	for _, quiero := range []string{"anim", "drawable", "font"} {
		if !tipos[quiero] {
			t.Errorf("la tabla no tiene el tipo %q: %v", quiero, salida.tabla.nombresTipo)
		}
	}

	// el value repetido: gana el proyecto y el del AAR no entra ni en el pool
	if !cadenaEnLaTabla(salida.tabla, "Proyecto") {
		t.Error("el valor del proyecto no llegó a la tabla")
	}
	if cadenaEnLaTabla(salida.tabla, "AAR") {
		t.Error("el valor del AAR pisó o convivió con el del proyecto")
	}
	if !cadenaEnLaTabla(salida.tabla, "solo del AAR") {
		t.Error("el value que solo declara el AAR se perdió")
	}
}

// TestPreviewVeElDrawableXmlDeLaDependencia: con el apk armado desde el enlace, el nodo con
// `android:background="@drawable/fondo_deps"` publica el color del `<solid>` del XML del AAR y el
// texto es el del proyecto, no el del AAR. La vista previa no aborta.
func TestPreviewVeElDrawableXmlDeLaDependencia(t *testing.T) {
	jar := jarDelSDK(t)
	m, err := marcoDelAndroidJar(jar)
	if err != nil {
		t.Fatalf("la tabla del framework: %v", err)
	}
	dir := proyectoConRecursosDeAAR(t)
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatalf("las fuentes del proyecto: %v", err)
	}
	salida, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, m,
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace: %v", err)
	}
	manifiestoBinario, err := escribeAXML(salida.manifiesto)
	if err != nil {
		t.Fatal(err)
	}
	arsc, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := escribeAXML(salida.layouts["res/layout/main.xml"])
	if err != nil {
		t.Fatal(err)
	}
	fondo, err := escribeAXML(salida.layouts["res/drawable/fondo_deps.xml"])
	if err != nil {
		t.Fatal(err)
	}
	capas, err := escribeAXML(salida.layouts["res/drawable/capas_deps.xml"])
	if err != nil {
		t.Fatal(err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml":         manifiestoBinario,
		"resources.arsc":              arsc,
		"res/layout/main.xml":         layout,
		"res/drawable/fondo_deps.xml": fondo,
		"res/drawable/capas_deps.xml": capas,
	})
	modelo, err := modeloVistaDeAPKCon(apk, "main", false)
	if err != nil {
		t.Fatalf("la vista previa abortó con el drawable XML del AAR: %v", err)
	}
	if modelo.Raiz == nil || len(modelo.Raiz.Hijos) != 2 {
		t.Fatalf("el modelo no trae los dos TextView: %+v", modelo.Raiz)
	}
	nodo := modelo.Raiz.Hijos[0]
	if nodo.Texto != "Proyecto" {
		t.Errorf("el TextView publica %q y se esperaba el valor del proyecto", nodo.Texto)
	}
	if nodo.Fondo != "#112233" {
		t.Errorf("el fondo del TextView es %q y el <solid> del AAR declara #112233", nodo.Fondo)
	}
	// el <layer-list> no es un <shape> de raíz: aunque tenga un <solid> dentro, no se publica como
	// fondo, porque la primera capa de una lista no es «el color del drawable».
	capasNodo := modelo.Raiz.Hijos[1]
	if capasNodo.Fondo != "" {
		t.Errorf("el <layer-list> publicó el fondo %q; se decidió no interpretarlo", capasNodo.Fondo)
	}
}
