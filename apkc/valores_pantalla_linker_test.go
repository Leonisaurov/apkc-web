package main

import "testing"

// TestFuentesDelProyectoPantallaYVersion: la puerta del enlazador reconoce las carpetas con
// `small`/`large`/`xlarge` y con `-vN`, y añade el `-v4` que aapt2 pone en la ruta de un tamaño de
// pantalla sin versión explícita.
func TestFuentesDelProyectoPantallaYVersion(t *testing.T) {
	layout := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`
	dir := proyecto(t, map[string]string{
		"AndroidManifest.xml":           manifestMinimo,
		"res/layout/main.xml":           layout,
		"res/layout-small/main.xml":     layout,
		"res/layout-large/main.xml":     layout,
		"res/layout-v21/main.xml":       layout,
		"res/layout-small-v21/main.xml": layout,
		"res/values-v33/cadenas.xml":    valuesMinimo,
	})
	_, layouts, _, _, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatalf("el proyecto no se leyo: %v", err)
	}
	quiero := map[string]string{
		"res/layout/main.xml":           "",
		"res/layout-small-v4/main.xml":  "small",
		"res/layout-large-v4/main.xml":  "large",
		"res/layout-v21/main.xml":       "v21",
		"res/layout-small-v21/main.xml": "small-v21",
	}
	if len(layouts) != len(quiero) {
		t.Fatalf("salieron %d layouts, se esperaban %d", len(layouts), len(quiero))
	}
	vistos := map[string]bool{}
	for _, f := range layouts {
		config, ok := quiero[f.nombre]
		if !ok {
			t.Errorf("ruta inesperada %q", f.nombre)
			continue
		}
		if f.config != config {
			t.Errorf("%q: config %q, quiero %q", f.nombre, f.config, config)
		}
		vistos[f.nombre] = true
	}
	for nombre := range quiero {
		if !vistos[nombre] {
			t.Errorf("no salio %q", nombre)
		}
	}
}

// TestCalificadorExtraDeCarpetaRechazaLoQueNoSabe: un sufijo que no sea tamaño de pantalla ni
// versión sigue rechazándose, como antes.
func TestCalificadorExtraDeCarpetaRechazaLoQueNoSabe(t *testing.T) {
	for _, nombre := range []string{"layout-nosep", "layout-small-nosep", "anim-raro", "layout", ""} {
		if _, _, _, ok := calificadorExtraDeCarpeta(nombre); ok {
			t.Errorf("%q no deberia aceptarse", nombre)
		}
	}
}
