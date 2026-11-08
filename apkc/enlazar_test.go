package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"testing"
)

// La prueba de la Fase 3 entera: enlazar el proyecto del studio DESDE SUS FICHEROS DE TEXTO y comparar
// lo que sale contra los tres binarios que dejo aapt2, fijados en base64 (docs/evidencia.md 24, 29, 30,
// 31, 35 y 36). Si esto casa, `aapt2 compile` y `aapt2 link` ya no hacen falta para este proyecto.

// marcoDelEnlace es la parte del framework que usan las tres fuentes. Los numeros no son de aqui: son
// los que TestMarcoDelAndroidJar y TestMarcoValoresDeAtributo comproban contra
// `aapt2 dump resources android.jar` sobre el SDK de este telefono, y se repiten a mano para que la
// prueba corra tambien en el hospedador de navegador.
func marcoDelEnlace() *marcoRecursos {
	ids := map[string]uint32{}
	for k, v := range map[string]uint32{
		"attr/theme": 0x01010000, "attr/label": 0x01010001, "attr/name": 0x01010003,
		"attr/exported": 0x01010010, "attr/windowBackground": 0x01010054, "attr/windowNoTitle": 0x01010056,
		"attr/gravity": 0x010100af, "attr/orientation": 0x010100c4, "attr/id": 0x010100d0,
		"attr/layout_width": 0x010100f4, "attr/layout_height": 0x010100f5, "attr/text": 0x0101014f,
		"attr/minSdkVersion": 0x0101020c, "attr/versionCode": 0x0101021b, "attr/versionName": 0x0101021c,
		"attr/targetSdkVersion": 0x01010270, "attr/maxSdkVersion": 0x01010271,
		"attr/windowActionBar": 0x010102cd, "attr/statusBarColor": 0x01010451,
		"attr/windowLightStatusBar": 0x010104e0, "attr/extractNativeLibs": 0x010104ea,
		"attr/compileSdkVersion": 0x01010572, "attr/compileSdkVersionCodename": 0x01010573,
		"style/TextAppearance.Large": 0x01030042, "style/Theme.DeviceDefault.Light": 0x0103012b,
		"color/white": 0x0106000b,
	} {
		ids[k] = v
	}
	porId := map[uint32]string{}
	for k, v := range ids {
		porId[v] = k
	}
	return &marcoRecursos{
		nombre:  "android",
		ids:     ids,
		porId:   porId,
		tipos:   []string{"attr", "id", "style", "string", "color"},
		valores: marcoDelStudio().valores,
	}
}

func fuentesDelStudio(t *testing.T) (valores, layouts []fuenteXML, manifiesto fuenteXML) {
	t.Helper()
	valores = []fuenteXML{decodifica(t, stylesDelStudio, "res/values/styles.xml", 423)}
	layouts = []fuenteXML{decodifica(t, layoutFuente, "res/layout/main.xml", 638)}
	manifiesto = decodifica(t, manifiestoFuente, "AndroidManifest.xml", 806)
	return
}

func TestEnlazarElProyectoDesdeTexto(t *testing.T) {
	valores, layouts, manifiesto := fuentesDelStudio(t)
	salida, err := enlazarProyecto(valores, layouts, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}

	// el layout: los 784 bytes de aapt2, byte a byte
	fijado, _ := base64.StdEncoding.DecodeString(axmlLayout)
	nuestro, err := escribeAXML(salida.layouts["res/layout/main.xml"])
	if err != nil {
		t.Fatalf("el layout no se escribio: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		difieren := 0
		for i := range fijado {
			if i < len(nuestro) && nuestro[i] != fijado[i] {
				if difieren < 3 {
					t.Errorf("layout byte #%d: %#02x, aapt2 %#02x", i, nuestro[i], fijado[i])
				}
				difieren++
			}
		}
		t.Errorf("layout: %d bytes distintos de %d (escritos %d)", difieren, len(fijado), len(nuestro))
	}

	// el manifiesto: los 2000 bytes de aapt2, con los cuatro atributos que el enlace anyade
	fijado, _ = base64.StdEncoding.DecodeString(axmlManifiesto)
	nuestro, err = escribeAXML(salida.manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto no se escribio: %v", err)
	}
	if !bytes.Equal(nuestro, fijado) {
		difieren := 0
		for i := range fijado {
			if i < len(nuestro) && nuestro[i] != fijado[i] {
				if difieren < 3 {
					t.Errorf("manifiesto byte #%d: %#02x, aapt2 %#02x", i, nuestro[i], fijado[i])
				}
				difieren++
			}
		}
		t.Errorf("manifiesto: %d bytes distintos de %d (escritos %d)", difieren, len(fijado), len(nuestro))
	}

	// la tabla: el mismo control del parrafo 29 — 936 bytes, y los primeros 752 identicos a aapt2
	// salvo los dos campos de tamano que cambian al escribir una configuracion en vez de dos
	fijado, _ = base64.StdEncoding.DecodeString(arscTabla)
	nuestra, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatalf("la tabla no se escribio: %v", err)
	}
	if len(nuestra) != 936 {
		t.Fatalf("la tabla enlazada mide %d bytes, se esperaban 936", len(nuestra))
	}
	esperado := append([]byte{}, fijado...)
	binary.LittleEndian.PutUint32(esperado[4:], uint32(len(nuestra)))
	binary.LittleEndian.PutUint32(esperado[68+4:], uint32(len(nuestra)-68))
	if !bytes.Equal(nuestra[:752], esperado[:752]) {
		for i := range esperado[:752] {
			if nuestra[i] != esperado[i] {
				t.Errorf("tabla byte #%d: %#02x, aapt2 %#02x", i, nuestra[i], esperado[i])
				break
			}
		}
	}

	// y la R, el texto de 493 bytes de aapt2
	if salida.r != rDeAapt2 {
		t.Errorf("la R enlazada desde el texto no casa:\n%s", salida.r)
	}

	// y el mapa de ids que vio el enlace es el que despues leen el AXML y la R
	ids, err := idsDelPaquete(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	for k, quieres := range map[string]uint32{
		"id/pulsar":      0x7f010000,
		"layout/main":    0x7f020000,
		"style/AppTheme": 0x7f030000,
	} {
		if ids[k] != quieres {
			t.Errorf("%s -> %#08x, querido %#08x", k, ids[k], quieres)
		}
	}
}

func TestEnlazarRechaza(t *testing.T) {
	m := marcoDelEnlace()
	valores, layouts, manifiesto := fuentesDelStudio(t)
	opts := opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"}

	casos := []struct {
		nombre     string
		valores    []fuenteXML
		layouts    []fuenteXML
		manifiesto fuenteXML
	}{
		{
			"un manifiesto sin package",
			valores, layouts,
			fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android"><application/></manifest>`)},
		},
		{
			"un atributo que no esta en el framework",
			valores, []fuenteXML{{nombre: "res/layout/x.xml", contenido: []byte(`<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:esoNoExiste="1"/>`)}}, manifiesto,
		},
		{
			"una referencia a un recurso que no esta declarado",
			valores, []fuenteXML{{nombre: "res/layout/x.xml", contenido: []byte(`<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:theme="@style/NoExiste"/>`)}}, manifiesto,
		},
		{
			"una referencia al framework inexistente",
			valores, layouts, fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="p"><application android:theme="@android:style/NoHay"/></manifest>`)},
		},
		{
			"el style de un elemento sin pool donde escribir su texto",
			valores, []fuenteXML{{nombre: "res/layout/x.xml", contenido: []byte(`<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="@desformado"/>`)}}, manifiesto,
		},
	}
	for _, c := range casos {
		if _, err := enlazarProyecto(c.valores, c.layouts, nil, c.manifiesto, m, opts); err == nil {
			t.Errorf("%s: el enlace lo acepto", c.nombre)
		}
	}

	// el camino bueno, de control: que los cinco rechazos no son un error que rechace todo
	if _, err := enlazarProyecto(valores, layouts, nil, manifiesto, m, opts); err != nil {
		t.Errorf("el proyecto valido fue rechazado: %v", err)
	}
}
