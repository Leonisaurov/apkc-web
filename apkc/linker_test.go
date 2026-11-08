package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// La prueba del enganche: lo que lee fuentesDelProyecto del arbol de un proyecto, y lo que rechaza. El
// enlace en si esta probado en enlazar_test.go contra los ficheros de aapt2; aqui se comprueba la puerta,
// que es lo que impide que `--linker go` escriba un apk callandose para un proyecto que no sabe enlazar.

func proyecto(t *testing.T, archivos map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for ruta, contenido := range archivos {
		completa := filepath.Join(dir, filepath.FromSlash(ruta))
		if err := os.MkdirAll(filepath.Dir(completa), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(completa, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const manifestMinimo = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="p.q"><application/></manifest>`
const valuesMinimo = `<resources><string name="s">hola</string></resources>`

func TestVersionesDeAndroidMedidas(t *testing.T) {
	for api, quiero := range map[int]string{28: "9", 29: "10", 30: "11", 31: "12", 33: "13", 34: "14", 35: "15"} {
		got, ok := nombreDeVersionDeAndroid(api)
		if !ok || got != quiero {
			t.Errorf("API %d: %q/%v, se esperaba %q", api, got, ok, quiero)
		}
	}
	for _, api := range []int{27, 32, 36} {
		if _, ok := nombreDeVersionDeAndroid(api); ok {
			t.Errorf("API %d no deberia estar en la tabla (nombre no numerico o no medido)", api)
		}
	}
}

func TestFuentesDelProyectoRecogeValuesYLayouts(t *testing.T) {
	dir := proyecto(t, map[string]string{
		"AndroidManifest.xml":    manifestMinimo,
		"res/values/cadenas.xml": valuesMinimo,
		"res/values/otros.xml":   valuesMinimo,
		"res/layout/main.xml":    `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`,
	})
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(valores) != 2 || len(layouts) != 1 {
		t.Fatalf("salieron %d values y %d layouts, se esperaban 2 y 1", len(valores), len(layouts))
	}
	if len(ficheros) != 0 {
		t.Fatalf("salieron %d ficheros binarios en un proyecto sin drawables", len(ficheros))
	}
	if valores[0].nombre != "res/values/cadenas.xml" || valores[1].nombre != "res/values/otros.xml" {
		t.Errorf("los valores salen %q y %q", valores[0].nombre, valores[1].nombre)
	}
	if layouts[0].nombre != "res/layout/main.xml" {
		t.Errorf("el layout sale %q", layouts[0].nombre)
	}
	if manifiesto.nombre != "AndroidManifest.xml" || !strings.Contains(string(manifiesto.contenido), "package=\"p.q\"") {
		t.Error("el manifiesto no llego como se leyo")
	}
}

func TestFuentesDelProyectoRechazaLoQueNoSabeEnlazar(t *testing.T) {
	casos := []struct {
		nombre        string
		archivos      map[string]string
		quierenPiensa string
	}{
		{
			"un drawable XML (vector/shape), que no es un binario que se copie",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/drawable/vector.xml": `<shape/>`},
			"drawable XML",
		},
		{
			"una carpeta de densidad, que es una segunda configuracion y aqui se emite una sola",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/layout-es/main.xml": `<FrameLayout/>`},
			"configuracion",
		},
		{
			"un recurso que no es xml dentro de layout",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/layout/nota.txt": "hola"},
			"layout solo admite .xml",
		},
		{
			"un manifiesto con permission, que aapt2 acompana de un Manifest.java",
			map[string]string{
				"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="p"><permission android:name="x"/><application/></manifest>`,
				"res/values/a.xml":    valuesMinimo,
			},
			"Manifest.java",
		},
		{
			"un proyecto sin manifiesto",
			map[string]string{"res/values/a.xml": valuesMinimo},
			"AndroidManifest.xml",
		},
		{
			"una subcarpeta dentro de values",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/values/sub/a.xml": valuesMinimo},
			"subcarpeta",
		},
	}
	for _, c := range casos {
		_, _, _, _, err := fuentesDelProyecto(proyecto(t, c.archivos))
		if err == nil {
			t.Errorf("%s: fue aceptado", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.quierenPiensa) {
			t.Errorf("%s: el rechazo no nombra el motivo (%q dice %q)", c.nombre, c.quierenPiensa, err)
		}
	}
}

func TestOpcionesDelProyecto(t *testing.T) {
	opts, err := opcionesDelProyecto("/x/platforms/android-34/android.jar", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if opts.compileSdkVersion != 34 || opts.compileSdkCodename != "14" {
		t.Errorf("las opciones salen %+v, medidas 34 y \"14\"", opts)
	}
	// una version no medida se rechaza con nombre, no se adivina el codename
	if _, err := opcionesDelProyecto("/x/platforms/android-21/android.jar", 0, ""); err == nil {
		t.Error("API 21 fue aceptado sin haber medido su nombre de version")
	}
	// y la ruta puede ser cualquiera si se dice la version: en la pagina el jar se monta donde quiere
	// el usuario, y ahi la ruta no informa nada
	pedidas, err := opcionesDelProyecto("/montado/apk-sdk.jar", 34, "")
	if err != nil || pedidas.compileSdkVersion != 34 || pedidas.compileSdkCodename != "14" {
		t.Errorf("--sdk-version sin ruta informativa: %+v, %v", pedidas, err)
	}
	if _, err := opcionesDelProyecto("/x/no-android.jar", 0, ""); err == nil {
		t.Error("una ruta que no dice la version y sin bandera fue aceptada")
	}
	codigos, err := opcionesDelProyecto("/x/no-android.jar", 21, "12")
	if err != nil || codigos.compileSdkVersion != 21 || codigos.compileSdkCodename != "12" {
		t.Errorf("--codename con su version: %+v, %v", codigos, err)
	}
	if _, err := opcionesDelProyecto("/x/no-android.jar", 0, "12"); err == nil {
		t.Error("un codename sin version fue aceptado")
	}
}
