package main

import (
	"encoding/base64"
	"testing"
)

// La prueba del arbol en texto (parrafo 36). Los dos fuentes son los del proyecto del studio, fijados
// en base64: el layout (638 bytes) y el manifiesto (806). Lo que se compara son las LINEAS y los
// NOMBRES que aapt2 anoto en los binarios del parrafo 24 y del parrafo 30, leidos con el instrumento
// del parrafo 24: es la unica forma de saber que este lector va a escribir lo mismo.

const layoutFuente = "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPExpbmVhckxheW91dCB4bWxuczphbmRyb2lkPSJodHRwOi8vc2NoZW1hcy5hbmRyb2lkLmNvbS9hcGsvcmVzL2FuZHJvaWQiCiAgICBhbmRyb2lkOm9yaWVudGF0aW9uPSJ2ZXJ0aWNhbCIKICAgIGFuZHJvaWQ6bGF5b3V0X3dpZHRoPSJtYXRjaF9wYXJlbnQiCiAgICBhbmRyb2lkOmxheW91dF9oZWlnaHQ9Im1hdGNoX3BhcmVudCIKICAgIGFuZHJvaWQ6Z3Jhdml0eT0iY2VudGVyIj4KCiAgICA8VGV4dFZpZXcKICAgICAgICBzdHlsZT0iQGFuZHJvaWQ6c3R5bGUvVGV4dEFwcGVhcmFuY2UuTGFyZ2UiCiAgICAgICAgYW5kcm9pZDpsYXlvdXRfd2lkdGg9IndyYXBfY29udGVudCIKICAgICAgICBhbmRyb2lkOmxheW91dF9oZWlnaHQ9IndyYXBfY29udGVudCIKICAgICAgICBhbmRyb2lkOnRleHQ9IkhlbGxvIFdvcmxkISIvPgoKICAgIDxCdXR0b24KICAgICAgICBhbmRyb2lkOmlkPSJAK2lkL3B1bHNhciIKICAgICAgICBhbmRyb2lkOmxheW91dF93aWR0aD0id3JhcF9jb250ZW50IgogICAgICAgIGFuZHJvaWQ6bGF5b3V0X2hlaWdodD0id3JhcF9jb250ZW50IgogICAgICAgIGFuZHJvaWQ6dGV4dD0iUHJlc3MgbWUiLz4KCjwvTGluZWFyTGF5b3V0Pgo="

const manifiestoFuente = "PG1hbmlmZXN0IHhtbG5zOmFuZHJvaWQ9Imh0dHA6Ly9zY2hlbWFzLmFuZHJvaWQuY29tL2Fway9yZXMvYW5kcm9pZCIKICAgIHBhY2thZ2U9ImNvbS5ob2xhLmFwa2NzdHVkaW8iCiAgICBhbmRyb2lkOnZlcnNpb25Db2RlPSIxIgogICAgYW5kcm9pZDp2ZXJzaW9uTmFtZT0iMC4xIj4KCiAgICA8dXNlcy1zZGsgYW5kcm9pZDptaW5TZGtWZXJzaW9uPSIyMSIKICAgICAgICAgIGFuZHJvaWQ6dGFyZ2V0U2RrVmVyc2lvbj0iMzMiCiAgICAgICAgICBhbmRyb2lkOm1heFNka1ZlcnNpb249IjMzIiAvPgoKICAgIDxhcHBsaWNhdGlvbiBhbmRyb2lkOmxhYmVsPSJIb2xhIGFwa2MiCiAgICAgICAgYW5kcm9pZDpleHRyYWN0TmF0aXZlTGlicz0iZmFsc2UiPgoKICAgICAgICA8YWN0aXZpdHkKICAgICAgICAgICAgYW5kcm9pZDp0aGVtZT0iQHN0eWxlL0FwcFRoZW1lIgogICAgICAgICAgICBhbmRyb2lkOm5hbWU9Ii5NYWluQWN0aXZpdHkiCiAgICAgICAgICAgIGFuZHJvaWQ6bGFiZWw9IkhvbGEgYXBrYyIKICAgICAgICAgICAgYW5kcm9pZDpleHBvcnRlZD0idHJ1ZSIgPgoKICAgICAgICAgICAgPGludGVudC1maWx0ZXI+CiAgICAgICAgICAgICAgICA8YWN0aW9uIGFuZHJvaWQ6bmFtZT0iYW5kcm9pZC5pbnRlbnQuYWN0aW9uLk1BSU4iIC8+CiAgICAgICAgICAgICAgICA8Y2F0ZWdvcnkgYW5kcm9pZDpuYW1lPSJhbmRyb2lkLmludGVudC5jYXRlZ29yeS5MQVVOQ0hFUiIvPgogICAgICAgICAgICA8L2ludGVudC1maWx0ZXI+CgogICAgICAgIDwvYWN0aXZpdHk+CiAgICA8L2FwcGxpY2F0aW9uPgo8L21hbmlmZXN0Pgo="

func decodifica(t *testing.T, b64, nombre string, mide int) fuenteXML {
	t.Helper()
	contenido, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	if len(contenido) != mide {
		t.Fatalf("%s mide %d bytes, se esperaban %d", nombre, len(contenido), mide)
	}
	return fuenteXML{nombre: nombre, contenido: contenido}
}

func TestArbolDelLayout(t *testing.T) {
	f := decodifica(t, layoutFuente, "res/layout/main.xml", 638)
	a, err := leerArbolDe(f)
	if err != nil {
		t.Fatal(err)
	}
	if a.raiz.nombre != "LinearLayout" || a.raiz.linea != 2 {
		t.Errorf("la raiz es %q linea %d, medida LinearLayout linea 2", a.raiz.nombre, a.raiz.linea)
	}
	if a.prefijos[uriAndroid] != "android" || a.lineaDe["android"] != 2 {
		t.Errorf("el prefijo android se declaro como %q en la linea %d", a.prefijos[uriAndroid], a.lineaDe["android"])
	}
	if len(a.raiz.attrs) != 4 {
		t.Fatalf("la raiz trae %d atributos, medidos 4", len(a.raiz.attrs))
	}
	// el orden del fuente, no el del binario: aapt2 reordena al escribir, y aqui se lee como esta escrito
	esperados := []struct {
		prefijo, nombre, texto string
	}{
		{"android", "orientation", "vertical"},
		{"android", "layout_width", "match_parent"},
		{"android", "layout_height", "match_parent"},
		{"android", "gravity", "center"},
	}
	for i, quieres := range esperados {
		a := a.raiz.attrs[i]
		if a.prefijo != quieres.prefijo || a.nombre != quieres.nombre || a.texto != quieres.texto {
			t.Errorf("attr #%d es %s:%s=%q, querido %s:%s=%q", i, a.prefijo, a.nombre, a.texto,
				quieres.prefijo, quieres.nombre, quieres.texto)
		}
	}
	if len(a.raiz.hijos) != 2 {
		t.Fatalf("%d hijos de la raiz, medidos 2", len(a.raiz.hijos))
	}
	tv, btn := a.raiz.hijos[0], a.raiz.hijos[1]
	if tv.nombre != "TextView" || tv.linea != 8 || len(tv.attrs) != 4 {
		t.Errorf("TextView en linea %d con %d atributos, medidos 8 y 4", tv.linea, len(tv.attrs))
	}
	// el `style` del fuente esta PRIMERO y no lleva prefijo: asi lo leyo aapt2 y asi lo manda al final
	if style, ok := tv.atribuDe("", "style"); !ok || style != "@android:style/TextAppearance.Large" {
		t.Errorf("el atributo style salio %q (encontrado=%v)", style, ok)
	}
	if btn.nombre != "Button" || btn.linea != 14 {
		t.Errorf("Button en linea %d, medida 14", btn.linea)
	}
	if id, ok := btn.atribuDe("android", "id"); !ok || id != "@+id/pulsar" {
		t.Errorf("el id del Button salio %q", id)
	}
	if e := a.buscar("LinearLayout", "TextView"); e != tv {
		t.Error("buscar no encuentra el TextView por su ruta")
	}
	if a.buscar("LinearLayout", "Inexistente") != nil {
		t.Error("buscar encontro algo que no esta")
	}
}

func TestArbolDelManifiesto(t *testing.T) {
	f := decodifica(t, manifiestoFuente, "AndroidManifest.xml", 806)
	a, err := leerArbolDe(f)
	if err != nil {
		t.Fatal(err)
	}
	if a.raiz.nombre != "manifest" || a.raiz.linea != 1 {
		t.Errorf("raiz %q linea %d", a.raiz.nombre, a.raiz.linea)
	}
	if a.lineaDe["android"] != 1 {
		t.Errorf("el xmlns:android se anoto en la linea %d, medida 1", a.lineaDe["android"])
	}
	if paquete, ok := a.raiz.atribuDe("", "package"); !ok || paquete != "com.hola.apkcstudio" {
		t.Errorf("el paquete del manifiesto sale %q (encontrado=%v)", paquete, ok)
	}
	// la comprobacion que vale: <activity` abre en la linea 13 y cierra el tag en la 17. El
	// decodificador de Go da la 17; aapt2 anota la 13.
	activity := a.buscar("manifest", "application", "activity")
	if activity == nil {
		t.Fatal("no se encontro el activity")
	}
	if activity.linea != 13 {
		t.Errorf("el activity sale en la linea %d, medida 13", activity.linea)
	}
	if len(activity.attrs) != 4 {
		t.Errorf("el activity trae %d atributos, medidos 4", len(activity.attrs))
	}
	if theme, ok := activity.atribuDe("android", "theme"); !ok || theme != "@style/AppTheme" {
		t.Errorf("el theme sale %q", theme)
	}
	for _, par := range []struct {
		ruta  []string
		linea int
	}{
		{[]string{"manifest", "uses-sdk"}, 6},
		{[]string{"manifest", "application"}, 10},
		{[]string{"manifest", "application", "activity", "intent-filter"}, 19},
		{[]string{"manifest", "application", "activity", "intent-filter", "action"}, 20},
		{[]string{"manifest", "application", "activity", "intent-filter", "category"}, 21},
	} {
		n := a.buscar(par.ruta...)
		if n == nil {
			t.Errorf("falta %v", par.ruta)
			continue
		}
		if n.linea != par.linea {
			t.Errorf("%v en la linea %d, medida %d", par.ruta, n.linea, par.linea)
		}
	}
	sdk := a.buscar("manifest", "uses-sdk")
	if v, ok := sdk.atribuDe("android", "minSdkVersion"); !ok || v != "21" {
		t.Errorf("minSdkVersion sale %q", v)
	}
}

// dosNamespacesFuente y su binario son el experimento del parrafo 30: un layout que declara android: y
// tools:, y el fichero de 368 bytes que aapt2 dejo despues de TIRAR los atributos tools: y su
// declaracion. Sirve para comprobar que el lector descarta lo mismo.
const dosNamespacesFuente = "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPEZyYW1lTGF5b3V0IHhtbG5zOmFuZHJvaWQ9Imh0dHA6Ly9zY2hlbWFzLmFuZHJvaWQuY29tL2Fway9yZXMvYW5kcm9pZCIKICAgIHhtbG5zOnRvb2xzPSJodHRwOi8vc2NoZW1hcy5hbmRyb2lkLmNvbS90b29scyIKICAgIGFuZHJvaWQ6bGF5b3V0X3dpZHRoPSJtYXRjaF9wYXJlbnQiCiAgICB0b29sczppZ25vcmU9Ik1pc3NpbmdDb25zdHJhaW50cyI+CiAgICA8VmlldwogICAgICAgIGFuZHJvaWQ6bGF5b3V0X3dpZHRoPSJ3cmFwX2NvbnRlbnQiCiAgICAgICAgdG9vbHM6dmlzaWJpbGl0eT0idmlzaWJsZSIgLz4KPC9GcmFtZUxheW91dD4K"

func TestArbolDescartaLoQueAapt2Descarta(t *testing.T) {
	f := decodifica(t, dosNamespacesFuente, "res/layout/dos.xml", 348)
	a, err := leerArbolDe(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.lineaDe) != 1 {
		nombres := []string{}
		for p := range a.lineaDe {
			nombres = append(nombres, p)
		}
		t.Errorf("quedaron %d declaraciones de namespace (%v), y en el binario solo hay una", len(a.lineaDe), nombres)
	}
	for _, ruta := range [][]string{{"FrameLayout"}, {"FrameLayout", "View"}} {
		n := a.buscar(ruta...)
		if n == nil {
			t.Fatalf("falta %v", ruta)
		}
		for _, attr := range n.attrs {
			if attr.nombre == "ignore" || attr.nombre == "visibility" {
				t.Errorf("%v conserva el atributo tools:%s", ruta, attr.nombre)
			}
		}
		if len(n.attrs) != 1 || n.attrs[0].prefijo != "android" || n.attrs[0].nombre != "layout_width" {
			t.Errorf("%v con %d atributos; en el binario solo queda layout_width", ruta, len(n.attrs))
		}
	}
	if a.raiz.linea != 2 || a.raiz.hijos[0].linea != 6 {
		t.Errorf("lineas %d y %d, medidas 2 y 6 (el View abre en la sexta)", a.raiz.linea, a.raiz.hijos[0].linea)
	}
}

func TestArbolRechaza(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
	}{
		{"sin raiz", ""},
		{"dos raices", "<a/><b/>"},
		{"cierre mal", "<a><b></a></b>"},
		{"sin cerrar", "<a>"},
		{"texto suelto", "<a>hola</a>"},
	}
	for _, c := range casos {
		if _, err := leerArbolDe(fuenteXML{nombre: "x.xml", contenido: []byte(c.texto)}); err == nil {
			t.Errorf("%s: fue aceptado", c.nombre)
		}
	}
	if _, err := leerArbolDe(fuenteXML{nombre: "x.xml", contenido: []byte("<a/><!--comentario--><b/>")}); err == nil {
		t.Error("un comentario seguido de una segunda raiz fue aceptado")
	}
	// la forma buena, de control
	if _, err := leerArbolDe(fuenteXML{nombre: "x.xml", contenido: []byte(`<?xml version="1.0"?><a xmlns:android="http://schemas.android.com/apk/res/android" android:x="1"><b/></a>`)}); err != nil {
		t.Errorf("la forma medida fue rechazada: %v", err)
	}
}
