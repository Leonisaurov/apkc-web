package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"sort"
	"testing"
)

// TestPantallaYVersion: cierra la frontera de los calificadores `small`/`large`/`xlarge` y de un
// `-vN` arbitrario. aapt2 de build-tools 35.0.1 escribe el tamaño de pantalla en el byte bajo de
// screenLayout (+28 del ResTable_config): small=1, large=3, xlarge=4, siempre con sdkVersion 4 en
// +24; `v21` y `v33` escriben 21 y 33 en +24; `small-v21` combina ambos; `sw600dp` sigue en +30 con
// sdkVersion 13. El orden de los bloques es el de ResTable_config::compare: versión antes que
// screenLayout, y screenType (port/land) después de todos ellos. La tabla tiene que salir byte a
// byte la suya.
func TestPantallaYVersion(t *testing.T) {
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.pantalla"><application/></manifest>`)}
	layout := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`
	layouts := []fuenteXML{
		{nombre: "res/layout/main.xml", contenido: []byte(layout)},
		{nombre: "res/layout-small-v4/main.xml", contenido: []byte(layout), config: "small"},
		{nombre: "res/layout-large-v4/main.xml", contenido: []byte(layout), config: "large"},
		{nombre: "res/layout-xlarge-v4/main.xml", contenido: []byte(layout), config: "xlarge"},
		{nombre: "res/layout-sw600dp-v13/main.xml", contenido: []byte(layout), config: "sw600dp"},
		{nombre: "res/layout-v21/main.xml", contenido: []byte(layout), config: "v21"},
		{nombre: "res/layout-small-v21/main.xml", contenido: []byte(layout), config: "small-v21"},
		{nombre: "res/layout-v33/main.xml", contenido: []byte(layout), config: "v33"},
		{nombre: "res/layout-port/main.xml", contenido: []byte(layout), config: "port"},
		{nombre: "res/layout-land/main.xml", contenido: []byte(layout), config: "land"},
	}
	valores := []fuenteXML{
		{nombre: "res/values/strings.xml", contenido: []byte(`<resources><string name="app_name">Base</string></resources>`)},
		{nombre: "res/values-es/strings.xml", contenido: []byte(`<resources><string name="app_name">ES</string></resources>`), config: "es"},
	}
	salida, err := enlazarProyecto(valores, layouts, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribe: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString(arscPantallaVersion)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// arscPantallaVersion: `aapt2 compile --dir res` + `aapt2 link --no-auto-version` de build-tools
// 35.0.1 sobre el fuente del test (base, small, large, xlarge, sw600dp-v13, v21, small-v21, v33,
// port, land y values/values-es) y `unzip -p out.apk resources.arsc | base64 -w0`.
const arscPantallaVersion = "AgAMACgIAAABAAAAAQAcAHgBAAAMAAAAAAAAAAABAABMAAAAAAAAAAAAAAAHAAAAIgAAAEEAAABcAAAAfAAAAJsAAAC9AAAA1wAAAPEAAAARAQAAJwEAAAQEQmFzZQAYGHJlcy9sYXlvdXQtbGFuZC9tYWluLnhtbAAcHHJlcy9sYXlvdXQtbGFyZ2UtdjQvbWFpbi54bWwAGBhyZXMvbGF5b3V0LXBvcnQvbWFpbi54bWwAHR1yZXMvbGF5b3V0LXNtYWxsLXYyMS9tYWluLnhtbAAcHHJlcy9sYXlvdXQtc21hbGwtdjQvbWFpbi54bWwAHx9yZXMvbGF5b3V0LXN3NjAwZHAtdjEzL21haW4ueG1sABcXcmVzL2xheW91dC12MjEvbWFpbi54bWwAFxdyZXMvbGF5b3V0LXYzMy9tYWluLnhtbAAdHXJlcy9sYXlvdXQteGxhcmdlLXY0L21haW4ueG1sABMTcmVzL2xheW91dC9tYWluLnhtbAACAkVTAAACIAGkBgAAfwAAAGMAbwBtAC4AcABhAG4AdABhAGwAbABhAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAQAAAAAAAGQBAAAAAAAAAAAAAAEAHABEAAAAAgAAAAAAAAAAAAAAJAAAAAAAAAAAAAAAEAAAAAYAbABhAHkAbwB1AHQAAAAGAHMAdAByAGkAbgBnAAAAAQAcADgAAAACAAAAAAAAAAABAAAkAAAAAAAAAAAAAAAHAAAABARtYWluAAgIYXBwX25hbWUAAAACAhAAFAAAAAEACgABAAAAgCwAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMKAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwUAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAADAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADAgAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMJAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAANAAAAAABYAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwYAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABUAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADBwAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMEAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAhAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwgAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADAwAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMBAAAAAgIQABQAAAACAAIAAQAAAAQAAAABAlQAaAAAAAIAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAEAAAAIAAADAAAAAAECVABoAAAAAgAAAAEAAABYAAAAQAAAAAAAAABlcwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAQAAAAgAAAMLAAAA"

// TestClaveDeConfigOrdenAapt2: el orden medido de los bloques, con independencia del fixture.
func TestClaveDeConfigOrdenAapt2(t *testing.T) {
	entrada := []string{"land", "v33", "es", "small-v21", "port", "v21", "xlarge", "sw600dp", "", "large", "small"}
	quiero := []string{"", "small", "large", "xlarge", "sw600dp", "v21", "small-v21", "v33", "port", "land", "es"}
	sort.SliceStable(entrada, func(i, j int) bool {
		gi, ni, ti := claveDeConfig(entrada[i])
		gj, nj, tj := claveDeConfig(entrada[j])
		if gi != gj {
			return gi < gj
		}
		if ni != nj {
			return ni < nj
		}
		return ti < tj
	})
	if len(entrada) != len(quiero) {
		t.Fatalf("longitudes distintas: %d contra %d", len(entrada), len(quiero))
	}
	for i := range quiero {
		if entrada[i] != quiero[i] {
			t.Errorf("en %d quiero %q y salio %q", i, quiero[i], entrada[i])
		}
	}
}

// TestConfigDePantallaYVersion: los bytes medidos del ResTable_config, sueltos.
func TestConfigDePantallaYVersion(t *testing.T) {
	casos := []struct {
		cfg        string
		sdk        uint16
		screenSize uint8
		sw         uint16
	}{
		{"small", 4, 1, 0},
		{"large", 4, 3, 0},
		{"xlarge", 4, 4, 0},
		{"v21", 21, 0, 0},
		{"v33", 33, 0, 0},
		{"small-v21", 21, 1, 0},
		{"sw600dp", 13, 0, 600},
		{"", 0, 0, 0},
	}
	for _, c := range casos {
		b := configDe(c.cfg)
		if sdk := binary.LittleEndian.Uint16(b[24:]); sdk != c.sdk {
			t.Errorf("%q: sdkVersion %d, quiero %d", c.cfg, sdk, c.sdk)
		}
		if b[28] != c.screenSize {
			t.Errorf("%q: screenLayout %d, quiero %d", c.cfg, b[28], c.screenSize)
		}
		if sw := binary.LittleEndian.Uint16(b[30:]); sw != c.sw {
			t.Errorf("%q: smallestScreenWidthDp %d, quiero %d", c.cfg, sw, c.sw)
		}
	}
}
