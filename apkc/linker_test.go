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

func TestFuentesDelProyectoRecogeValuesYLayouts(t *testing.T) {
	dir := proyecto(t, map[string]string{
		"AndroidManifest.xml":    manifestMinimo,
		"res/values/cadenas.xml": valuesMinimo,
		"res/values/otros.xml":   valuesMinimo,
		"res/layout/main.xml":    `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`,
	})
	valores, layouts, manifiesto, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(valores) != 2 || len(layouts) != 1 {
		t.Fatalf("salieron %d values y %d layouts, se esperaban 2 y 1", len(valores), len(layouts))
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
			"un drawable, que pide copiar ficheros y poner entradas con referencia de fichero",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/drawable/icono.png": "no es png de verdad"},
			"drawable",
		},
		{
			"una carpeta de densidad, que es una segunda configuracion y aqui se emite una sola",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/layout-es/main.xml": `<FrameLayout/>`},
			"configuracion",
		},
		{
			"un recurso que no es xml dentro de layout",
			map[string]string{"AndroidManifest.xml": manifestMinimo, "res/layout/nota.txt": "hola"},
			"no es .xml",
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
		_, _, _, err := fuentesDelProyecto(proyecto(t, c.archivos))
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
