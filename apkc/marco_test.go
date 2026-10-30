package main

import (
	"os"
	"path/filepath"
	"testing"
)

// La prueba del lector de la tabla del framework (parrafo 33). El oraculo es `aapt2 dump resources
// android.jar`, que imprime 240.637 lineas y de el se copiaron los ids de abajo; son los MISMos
// numeros que aparecen en los ficheros fijados del §24 y del §30, asi que la comprobación cierra el
// circulo: lo que aapt2 puso en el AXML es lo que nosotros leemos del jar.

// idsDelFramework copiados del volcado de aapt2 sobre el android.jar de este telefono (API 34).
var idsDelFramework = map[string]uint32{
	"attr/theme":                      0x01010000,
	"attr/label":                      0x01010001,
	"attr/name":                       0x01010003,
	"attr/exported":                   0x01010010,
	"attr/windowBackground":           0x01010054,
	"attr/windowNoTitle":              0x01010056,
	"attr/gravity":                    0x010100af,
	"attr/orientation":                0x010100c4,
	"attr/id":                         0x010100d0,
	"attr/layout_width":               0x010100f4,
	"attr/layout_height":              0x010100f5,
	"attr/text":                       0x0101014f,
	"attr/minSdkVersion":              0x0101020c,
	"attr/versionCode":                0x0101021b,
	"attr/versionName":                0x0101021c,
	"attr/targetSdkVersion":           0x01010270,
	"attr/maxSdkVersion":              0x01010271,
	"attr/windowActionBar":            0x010102cd,
	"attr/statusBarColor":             0x01010451,
	"attr/windowLightStatusBar":       0x010104e0,
	"attr/extractNativeLibs":          0x010104ea,
	"attr/compileSdkVersion":          0x01010572,
	"attr/compileSdkVersionCodename":  0x01010573,
	"color/white":                     0x0106000b,
	"color/black":                     0x0106000c,
	"style/TextAppearance.Large":      0x01030042,
	"style/Theme.DeviceDefault.Light": 0x0103012b,
	"dimen/app_icon_size":             0x01050000,
}

func jarDelFramework(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no hay home para buscar el SDK")
	}
	ruta := filepath.Join(home, "Android", "platforms", "android-34", "android.jar")
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("no hay android.jar en %s", ruta)
	}
	return ruta
}

func TestMarcoDelAndroidJar(t *testing.T) {
	m, err := marcoDelAndroidJar(jarDelFramework(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.nombre != "android" {
		t.Errorf("el paquete id 1 se llama %q", m.nombre)
	}
	if len(m.tipos) != 23 {
		t.Errorf("el framework trae %d tipos, medidos 23", len(m.tipos))
	}
	// los tipos del framework NO van alfabeticos: el orden es el de public.xml (attr, id, style,
	// string, dimen…), y el id de tipo es la posicion mas uno — por eso se comprueba aqui
	if m.tipos[0] != "attr" || m.tipos[1] != "id" || m.tipos[2] != "style" {
		t.Errorf("los tres primeros tipos del framework son %q, %q, %q", m.tipos[0], m.tipos[1], m.tipos[2])
	}
	for k, quieres := range idsDelFramework {
		got, ok := m.ids[k]
		if !ok {
			t.Errorf("falta %s, que aapt2 lista en %#08x", k, quieres)
			continue
		}
		if got != quieres {
			t.Errorf("%s: leido %#08x, aapt2 dice %#08x", k, got, quieres)
		}
	}

	// y los ids que usaron los escritores salen del marco, no de la copia a mano: el atributo del
	// layout, el del manifiesto y el recurso al que apuntaba el estilo del fixture
	id, err := m.idDeAtributo("layout_width")
	if err != nil || id != 0x010100f4 {
		t.Errorf("layout_width: %#08x, %v", id, err)
	}
	id, err = m.idDe("style", "Theme.DeviceDefault.Light")
	if err != nil || id != 0x0103012b {
		t.Errorf("el padre del AppTheme: %#08x, %v", id, err)
	}
	if _, err := m.idDe("attr", "noExisteNingunoDeEstos"); err == nil {
		t.Error("un atributo inexistente fue aceptado")
	}
	if len(m.nombresDe("attr")) < 1500 {
		t.Errorf("el tipo attr dio %d nombres, aapt2 lista 1545", len(m.nombresDe("attr")))
	}
}

func TestMarcoRechaza(t *testing.T) {
	// nuestra propia tabla de app: packageCount 1 y paquete 0x7f, o sea que no hay paquete android
	nuestra, err := escribeTablaRecursos(tablaDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leeMarco(nuestra); err == nil {
		t.Error("una tabla sin el paquete del framework fue aceptada")
	}

	malo := append([]byte{}, nuestra...)
	malo[0], malo[1] = 0x03, 0x00 // un trozo XML en lugar de la tabla
	if _, err := leeMarco(malo); err == nil {
		t.Error("un fichero que no es tabla fue aceptado")
	}

	malo = append([]byte{}, nuestra...)
	malo[8] = 4 // packageCount 4, con un solo paquete dentro
	if _, err := leeMarco(malo); err == nil {
		t.Error("un conteo de paquetes que no cuadra fue aceptado")
	}

	// una tabla de doce bytes sin pool detras tampoco pasa
	if _, err := leeMarco([]byte{}); err == nil {
		t.Error("un fichero vacio fue aceptado como tabla")
	}
}
