package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"sort"
	"strings"
	"testing"
)

// TestCalificadoresResTableConfig cierra los calificadores nuevos contra el resources.arsc que
// produjo aapt2 de build-tools 35.0.1 con --no-auto-version sobre este mismo proyecto: base,
// small, los seis uiMode (desk, car, television, appliance, watch, night) y car-night, sw600dp,
// v21 y car-v21, teclado (nokeys, qwerty, 12key), navegación (nonav, dpad, trackball, wheel),
// orientación, touchscreen (notouch, stylus, finger), densidad y values-es con una clave sin base.
// El arsc de 5052 bytes sale byte a byte. La medición corrigió el encargo: uiMode está en +29,
// no en +36; y el spec de uiMode es 0x1400 (0x1000 de UI_MODE más 0x0400 de la versión 8).
func TestCalificadoresResTableConfig(t *testing.T) {
	lay := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`
	man := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.calif"><application/></manifest>`)}
	layouts := []fuenteXML{}
	for _, p := range [][2]string{
		{"res/layout/main.xml", ""},
		{"res/layout-small-v4/main.xml", "small"},
		{"res/layout-desk-v8/main.xml", "desk"},
		{"res/layout-car-v8/main.xml", "car"},
		{"res/layout-television-v8/main.xml", "television"},
		{"res/layout-appliance-v8/main.xml", "appliance"},
		{"res/layout-watch-v8/main.xml", "watch"},
		{"res/layout-night-v8/main.xml", "night"},
		{"res/layout-car-night-v8/main.xml", "car-night"},
		{"res/layout-sw600dp-v13/main.xml", "sw600dp"},
		{"res/layout-v21/main.xml", "v21"},
		{"res/layout-car-v21/main.xml", "car-v21"},
		{"res/layout-nokeys/main.xml", "nokeys"},
		{"res/layout-qwerty/main.xml", "qwerty"},
		{"res/layout-qwerty-v21/main.xml", "qwerty-v21"},
		{"res/layout-12key/main.xml", "12key"},
		{"res/layout-nonav/main.xml", "nonav"},
		{"res/layout-dpad/main.xml", "dpad"},
		{"res/layout-dpad-v21/main.xml", "dpad-v21"},
		{"res/layout-trackball/main.xml", "trackball"},
		{"res/layout-wheel/main.xml", "wheel"},
		{"res/layout-port/main.xml", "port"},
		{"res/layout-land/main.xml", "land"},
		{"res/layout-notouch/main.xml", "notouch"},
		{"res/layout-notouch-v21/main.xml", "notouch-v21"},
		{"res/layout-stylus/main.xml", "stylus"},
		{"res/layout-finger/main.xml", "finger"},
		{"res/layout-finger-v21/main.xml", "finger-v21"},
		{"res/layout-mdpi-v4/main.xml", "mdpi"},
		{"res/layout-hdpi-v4/main.xml", "hdpi"},
		{"res/layout-es/main.xml", "es"},
	} {
		layouts = append(layouts, fuenteXML{nombre: p[0], contenido: []byte(lay), config: p[1]})
	}
	valores := []fuenteXML{
		{nombre: "res/values/strings.xml", contenido: []byte(`<resources><string name="app_name">Base</string><string name="comun">Comun</string></resources>`)},
		{nombre: "res/values-es/strings.xml", contenido: []byte(`<resources><string name="app_name">ES</string><string name="soloes">SoloES</string></resources>`), config: "es"},
	}
	salida, err := enlazarProyecto(valores, layouts, nil, man, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatal(err)
	}
	nuestro, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	fijado, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(arscCalificadoresEntrada, "\n", ""))
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el arsc nuestro son %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// TestConfigDeCalificadoresNuevos: los bytes sueltos del ResTable_config medidos con aapt2.
// uiMode en +29, touchscreen en +13, keyboard en +16, navigation en +17 y sdkVersion 8 en +24
// para todo uiMode. `car-night` combina 0x03 y 0x20 en +29.
func TestConfigDeCalificadoresNuevos(t *testing.T) {
	casos := []struct {
		cfg         string
		sdk         uint16
		touchscreen byte
		keyboard    byte
		navigation  byte
		uiMode      byte
		orientation byte
	}{
		{"car", 8, 0, 0, 0, 0x03, 0},
		{"desk", 8, 0, 0, 0, 0x02, 0},
		{"night", 8, 0, 0, 0, 0x20, 0},
		{"television", 8, 0, 0, 0, 0x04, 0},
		{"appliance", 8, 0, 0, 0, 0x05, 0},
		{"watch", 8, 0, 0, 0, 0x06, 0},
		{"car-night", 8, 0, 0, 0, 0x23, 0},
		{"notouch", 0, 1, 0, 0, 0, 0},
		{"stylus", 0, 2, 0, 0, 0, 0},
		{"finger", 0, 3, 0, 0, 0, 0},
		{"nokeys", 0, 0, 1, 0, 0, 0},
		{"qwerty", 0, 0, 2, 0, 0, 0},
		{"12key", 0, 0, 3, 0, 0, 0},
		{"nonav", 0, 0, 0, 1, 0, 0},
		{"dpad", 0, 0, 0, 2, 0, 0},
		{"trackball", 0, 0, 0, 3, 0, 0},
		{"wheel", 0, 0, 0, 4, 0, 0},
		{"car-v21", 21, 0, 0, 0, 0x03, 0},
	}
	for _, c := range casos {
		b := configDe(c.cfg)
		if sdk := binary.LittleEndian.Uint16(b[24:]); sdk != c.sdk {
			t.Errorf("%q: sdkVersion %d, quiero %d", c.cfg, sdk, c.sdk)
		}
		if b[13] != c.touchscreen || b[16] != c.keyboard || b[17] != c.navigation ||
			b[29] != c.uiMode || b[12] != c.orientation {
			t.Errorf("%q: campos %x, quiero touch %x keyboard %x nav %x uiMode %x orient %x",
				c.cfg, configDe(c.cfg), c.touchscreen, c.keyboard, c.navigation, c.uiMode, c.orientation)
		}
	}
}

// TestMascaraConfigCalificadoresNuevos: las banderas del spec medidas una a una con aapt2.
func TestMascaraConfigCalificadoresNuevos(t *testing.T) {
	casos := map[string]uint32{
		"car": 0x1400, "desk": 0x1400, "night": 0x1400, "television": 0x1400,
		"appliance": 0x1400, "watch": 0x1400, "car-night": 0x1400,
		"notouch": 0x0008, "stylus": 0x0008, "finger": 0x0008,
		"nokeys": 0x0010, "qwerty": 0x0010, "12key": 0x0010,
		"nonav": 0x0040, "dpad": 0x0040, "trackball": 0x0040, "wheel": 0x0040,
		"port": 0x0080, "land": 0x0080, "mdpi": 0x0500, "hdpi": 0x0500,
		"small": 0x0c00, "sw600dp": 0x2400, "v21": 0x0400, "es": 0x0004,
	}
	for cfg, quiero := range casos {
		if got := mascaraConfig(cfg); got != quiero {
			t.Errorf("%q: bandera %#06x, quiero %#06x", cfg, got, quiero)
		}
	}
}

// TestOrdenCalificadoresNuevos: el orden medido de los bloques, que es el de
// ResTable_config::compare: locale al final, screenType (orientación, touchscreen, densidad),
// input (keyboard, navigation), versión, screenLayout, uiMode y smallestWidth.
func TestOrdenCalificadoresNuevos(t *testing.T) {
	entrada := []string{
		"finger-v21", "es", "v21", "hdpi", "car-v21", "dpad", "port", "night",
		"qwerty", "small", "", "notouch", "car", "sw600dp", "mdpi", "finger", "land",
	}
	quiero := []string{
		"", "small", "car", "night", "sw600dp", "v21", "car-v21", "qwerty",
		"dpad", "port", "land", "notouch", "finger", "finger-v21", "mdpi", "hdpi", "es",
	}
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

// arscCalificadoresEntrada: `aapt2 compile --dir res -o c.zip` + `aapt2 link --no-auto-version` de
// build-tools 35.0.1 sobre el proyecto del test y `unzip -p out.apk resources.arsc | base64 -w0`.
const arscCalificadoresEntrada = `
AgAMALwTAAABAAAAAQAcAFwEAAAiAAAAAAAAAAABAACkAAAAAAAAAAAAAAAHAAAADwAAACsAAABOAAAAcQAAAI8AAACsAAAA
ygAAAOkAAAAEAQAAHQEAAD4BAABbAQAAeQEAAJQBAACyAQAA0QEAAO4BAAAKAgAALAIAAEoCAABlAgAAhgIAAKMCAADCAgAA
3wIAAAEDAAAlAwAARQMAAF8DAAB+AwAAmgMAALADAAAEBEJhc2UABQVDb211bgAZGXJlcy9sYXlvdXQtMTJrZXkvbWFpbi54
bWwAICByZXMvbGF5b3V0LWFwcGxpYW5jZS12OC9tYWluLnhtbAAgIHJlcy9sYXlvdXQtY2FyLW5pZ2h0LXY4L21haW4ueG1s
ABsbcmVzL2xheW91dC1jYXItdjIxL21haW4ueG1sABoacmVzL2xheW91dC1jYXItdjgvbWFpbi54bWwAGxtyZXMvbGF5b3V0
LWRlc2stdjgvbWFpbi54bWwAHBxyZXMvbGF5b3V0LWRwYWQtdjIxL21haW4ueG1sABgYcmVzL2xheW91dC1kcGFkL21haW4u
eG1sABYWcmVzL2xheW91dC1lcy9tYWluLnhtbAAeHnJlcy9sYXlvdXQtZmluZ2VyLXYyMS9tYWluLnhtbAAaGnJlcy9sYXlv
dXQtZmluZ2VyL21haW4ueG1sABsbcmVzL2xheW91dC1oZHBpLXY0L21haW4ueG1sABgYcmVzL2xheW91dC1sYW5kL21haW4u
eG1sABsbcmVzL2xheW91dC1tZHBpLXY0L21haW4ueG1sABwccmVzL2xheW91dC1uaWdodC12OC9tYWluLnhtbAAaGnJlcy9s
YXlvdXQtbm9rZXlzL21haW4ueG1sABkZcmVzL2xheW91dC1ub25hdi9tYWluLnhtbAAfH3Jlcy9sYXlvdXQtbm90b3VjaC12
MjEvbWFpbi54bWwAGxtyZXMvbGF5b3V0LW5vdG91Y2gvbWFpbi54bWwAGBhyZXMvbGF5b3V0LXBvcnQvbWFpbi54bWwAHh5y
ZXMvbGF5b3V0LXF3ZXJ0eS12MjEvbWFpbi54bWwAGhpyZXMvbGF5b3V0LXF3ZXJ0eS9tYWluLnhtbAAcHHJlcy9sYXlvdXQt
c21hbGwtdjQvbWFpbi54bWwAGhpyZXMvbGF5b3V0LXN0eWx1cy9tYWluLnhtbAAfH3Jlcy9sYXlvdXQtc3c2MDBkcC12MTMv
bWFpbi54bWwAISFyZXMvbGF5b3V0LXRlbGV2aXNpb24tdjgvbWFpbi54bWwAHR1yZXMvbGF5b3V0LXRyYWNrYmFsbC9tYWlu
LnhtbAAXF3Jlcy9sYXlvdXQtdjIxL21haW4ueG1sABwccmVzL2xheW91dC13YXRjaC12OC9tYWluLnhtbAAZGXJlcy9sYXlv
dXQtd2hlZWwvbWFpbi54bWwAExNyZXMvbGF5b3V0L21haW4ueG1sAAICRVMAAAAAAAIgAVQPAAB/AAAAYwBvAG0ALgBjAGEA
bABpAGYAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACABAAAAAAAAZAEAAAAAAAAAAAAAAQAcAEQAAAACAAAAAAAAAAAAAAAkAAAA
AAAAAAAAAAAQAAAABgBsAGEAeQBvAHUAdAAAAAYAcwB0AHIAaQBuAGcAAAABABwARAAAAAMAAAAAAAAAAAEAACgAAAAAAAAA
AAAAAAcAAAASAAAABARtYWluAAgIYXBwX25hbWUABQVjb211bgAAAAICEAAUAAAAAQAfAAEAAADcPQAAAQJUAGgAAAABAAAA
AQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAgAAAAAAAAACAAAAyAAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAABAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADGAAAAAECVABoAAAAAQAAAAEAAABYAAAA
QAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAA
AAAAAAgAAAMHAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAMAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwYAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAgAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAAD
GwAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAFAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMDAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAIAAAAAAYAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAx4AAAABAlQA
aAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAACAAAAAAAAAAIAAADEAAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
CAAAAAAjAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMEAAAAAQJUAGgAAAABAAAA
AQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAANAAAAAABYAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAgAAAAAAAAACAAAAxoAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABUAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADHQAAAAECVABoAAAAAQAAAAEAAABYAAAA
QAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFQAAAAADAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAA
AAAAAAgAAAMFAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAxEAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAA
AAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAAD
FwAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAFQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMWAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAAAA
AwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwIAAAABAlQA
aAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAACAAAAAAAAAAIAAADEgAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAAAAAACAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMJAAAAAQJUAGgAAAABAAAA
AQAAAFgAAABAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAVAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAgAAAAAAAAACAAAAwgAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAAAAAAwAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADHAAAAAECVABoAAAAAQAAAAEAAABYAAAA
QAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAA
AAAAAAgAAAMfAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAxUAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAA
AAAAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAAD
DgAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMUAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAAAAAAAAAQAA
AAAAAAAAAAAVAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAxMAAAABAlQA
aAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAACAAAAAAAAAAIAAADGQAAAAECVABoAAAAAQAAAAEAAABYAAAAQAAAAAAAAAAAAAAAAAMAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAMMAAAAAQJUAGgAAAABAAAA
AQAAAFgAAABAAAAAAAAAAAAAAAAAAwAAAAAAAAAAAAAVAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAgAAAAAAAAACAAAAwsAAAABAlQAaAAAAAEAAAABAAAAWAAAAEAAAAAAAAAAAAAAAAAAoAAAAAAAAAAAAAQAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAIAAADDwAAAAECVABoAAAAAQAAAAEAAABYAAAA
QAAAAAAAAAAAAAAAAADwAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAA
AAAAAAgAAAMNAAAAAQJUAGgAAAABAAAAAQAAAFgAAABAAAAAAAAAAGVzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAACAAAAwoAAAACAhAAGAAAAAIAAgACAAAABAAAAAAAAAABAlQA
fAAAAAIAAAACAAAAXAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAEAAAAAgAAAABAAAACAAAAwAAAAAIAAAAAgAAAAgAAAMBAAAAAQJUAGwAAAACAAAAAgAAAFwAAABAAAAA
AAAAAGVzAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAP////8IAAAA
AQAAAAgAAAMhAAAA`
