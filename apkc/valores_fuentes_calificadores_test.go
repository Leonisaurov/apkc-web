package main

import "testing"

// TestFuentesDelProyectoCalificadoresNuevos: la puerta del enlazador acepta las carpetas nuevas y
// calcula la ruta de salida igual que aapt2 de build-tools 35.0.1. Medido con `aapt2 dump resources`:
// uiMode añade `-v8` a la ruta (car-night-v8), sw añade `-v13`, densidad y tamaño `-v4`, y
// touchscreen/keyboard/navigation no añaden nada. Un `-vN` explícito manda y no se duplica.
func TestFuentesDelProyectoCalificadoresNuevos(t *testing.T) {
	lay := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`
	archivos := map[string]string{
		"AndroidManifest.xml": manifestMinimo,
		"res/layout/main.xml": lay,
	}
	quiero := map[string]string{
		"res/layout-car/main.xml":           "car",
		"res/layout-desk/main.xml":          "desk",
		"res/layout-night/main.xml":         "night",
		"res/layout-television/main.xml":    "television",
		"res/layout-appliance/main.xml":     "appliance",
		"res/layout-watch/main.xml":         "watch",
		"res/layout-car-night/main.xml":     "car-night",
		"res/layout-car-v21/main.xml":       "car-v21",
		"res/layout-notouch/main.xml":       "notouch",
		"res/layout-finger/main.xml":        "finger",
		"res/layout-stylus/main.xml":        "stylus",
		"res/layout-nokeys/main.xml":        "nokeys",
		"res/layout-qwerty/main.xml":        "qwerty",
		"res/layout-12key/main.xml":         "12key",
		"res/layout-nonav/main.xml":         "nonav",
		"res/layout-dpad/main.xml":          "dpad",
		"res/layout-trackball/main.xml":     "trackball",
		"res/layout-wheel/main.xml":         "wheel",
		"res/layout-sw600dp/main.xml":       "sw600dp",
		"res/values-desk/cadenas.xml":       "desk",
		"res/values-qwerty-v21/cadenas.xml": "qwerty-v21",
	}
	salidas := map[string]string{
		"res/layout-car/main.xml":        "res/layout-car-v8/main.xml",
		"res/layout-desk/main.xml":       "res/layout-desk-v8/main.xml",
		"res/layout-night/main.xml":      "res/layout-night-v8/main.xml",
		"res/layout-television/main.xml": "res/layout-television-v8/main.xml",
		"res/layout-appliance/main.xml":  "res/layout-appliance-v8/main.xml",
		"res/layout-watch/main.xml":      "res/layout-watch-v8/main.xml",
		"res/layout-car-night/main.xml":  "res/layout-car-night-v8/main.xml",
		"res/layout-car-v21/main.xml":    "res/layout-car-v21/main.xml",
		"res/layout-notouch/main.xml":    "res/layout-notouch/main.xml",
		"res/layout-finger/main.xml":     "res/layout-finger/main.xml",
		"res/layout-stylus/main.xml":     "res/layout-stylus/main.xml",
		"res/layout-nokeys/main.xml":     "res/layout-nokeys/main.xml",
		"res/layout-qwerty/main.xml":     "res/layout-qwerty/main.xml",
		"res/layout-12key/main.xml":      "res/layout-12key/main.xml",
		"res/layout-nonav/main.xml":      "res/layout-nonav/main.xml",
		"res/layout-dpad/main.xml":       "res/layout-dpad/main.xml",
		"res/layout-trackball/main.xml":  "res/layout-trackball/main.xml",
		"res/layout-wheel/main.xml":      "res/layout-wheel/main.xml",
		"res/layout-sw600dp/main.xml":    "res/layout-sw600dp-v13/main.xml",
	}
	for entrada, config := range quiero {
		archivos[entrada] = lay
		_ = config
	}
	archivos["res/values-desk/cadenas.xml"] = valuesMinimo
	archivos["res/values-qwerty-v21/cadenas.xml"] = valuesMinimo
	dir := proyecto(t, archivos)
	valores, layouts, _, _, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatalf("el proyecto no se leyo: %v", err)
	}
	vistos := map[string]string{}
	for _, f := range layouts {
		vistos[f.nombre] = f.config
	}
	for entrada, nombreSalida := range salidas {
		config, ok := vistos[nombreSalida]
		if !ok {
			t.Errorf("no salio %q (entrada %q); salio %q", nombreSalida, entrada, vistos)
			continue
		}
		if config != quiero[entrada] {
			t.Errorf("%q: config %q, quiero %q", nombreSalida, config, quiero[entrada])
		}
	}
	for _, f := range valores {
		if f.nombre == "res/values-desk/cadenas.xml" && f.config != "desk" {
			t.Errorf("values-desk: config %q, quiero desk", f.config)
		}
		if f.nombre == "res/values-qwerty-v21/cadenas.xml" && f.config != "qwerty-v21" {
			t.Errorf("values-qwerty-v21: config %q, quiero qwerty-v21", f.config)
		}
	}
	if len(layouts) != len(salidas)+1 {
		t.Errorf("salieron %d layouts, esperaba %d", len(layouts), len(salidas)+1)
	}
}
