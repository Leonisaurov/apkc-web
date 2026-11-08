package main

import (
	"testing"
)

// TestMenuXmlYRawSeEnlazan: menu y xml son AXML (como layout) y raw es un fichero literal. Medido
// con `aapt2 dump resources` en el parrafo 97: la tabla declara menu/principal (file XML),
// xml/prefs (file XML) y raw/dato (file), y el raw viaja con sus bytes.
func TestMenuXmlYRawSeEnlazan(t *testing.T) {
	dir := proyecto(t, map[string]string{
		"AndroidManifest.xml":    manifestMinimo,
		"res/values/strings.xml": valuesMinimo,
		"res/menu/principal.xml": `<menu xmlns:android="http://schemas.android.com/apk/res/android"><item android:text="Hola"/></menu>`,
		"res/xml/prefs.xml":      `<PreferenceScreen xmlns:android="http://schemas.android.com/apk/res/android"/>`,
		"res/raw/dato.txt":       "datos crudos\n",
		"res/layout/main.xml":    `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`,
	})
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatal(err)
	}
	// layouts lleva layout, menu y xml; ficheros lleva el raw
	if len(layouts) != 3 {
		t.Fatalf("salieron %d XML, se esperaban 3 (layout, menu, xml)", len(layouts))
	}
	if len(ficheros) != 1 || ficheros[0].nombre != "res/raw/dato.txt" {
		t.Fatalf("los ficheros salen %+v", ficheros)
	}
	salida, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	tipos := map[string]bool{}
	for _, nombre := range salida.tabla.nombresTipo {
		tipos[nombre] = true
	}
	for _, nombre := range []string{"layout", "menu", "xml", "raw", "string"} {
		if !tipos[nombre] {
			t.Errorf("la tabla no declara el tipo %s: %v", nombre, salida.tabla.nombresTipo)
		}
	}
	for _, ruta := range []string{"res/menu/principal.xml", "res/xml/prefs.xml"} {
		if _, ok := salida.layouts[ruta]; !ok {
			t.Errorf("no se escribio el AXML %s", ruta)
		}
	}
	if string(salida.ficheros["res/raw/dato.txt"]) != "datos crudos\n" {
		t.Errorf("el raw no llega entero: %q", salida.ficheros["res/raw/dato.txt"])
	}
}
