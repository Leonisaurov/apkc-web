package main

// La mitad de recursos del frente de dependencias: un AAR importado deja `res/__deps/<isla>/…` y
// `lib/<abi>/*.so` en el proyecto (es la forma que produce `apkc-deps`, ver `docs/plan-deps-red.md`),
// y `build --linker go --compiler go` tiene que enlazar esos recursos con el enlazador de Go y meter
// los nativos en el apk, sin aapt2 y sin levantar un proceso. Aquí se arma un `.aar` sintético con
// las tres partes (`classes.jar`, `res/`, `lib/`) y se mide el paquete resultante por dentro.

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// islaDePrueba es el nombre de carpeta que el importador deriva de la coordenada
// (`sha1(coord)[:12]`); aquí es fijo para poder nombrar los ficheros de values que llevan su sufijo.
const islaDePrueba = "9f1c2a7b3d4e"

// soFalso es una cabecera ELF de 32 bits para x86 (`EM_386`), sin secciones ni símbolos: el
// empaquetador no interpreta un `.so`, pero el fichero tiene la forma que Android espera encontrar y
// no es texto con extensión de librería.
func soFalso() []byte {
	b := make([]byte, 52)
	copy(b, "\x7fELF")
	b[4] = 1  // ELFCLASS32
	b[5] = 1  // ELFDATA2LSB
	b[6] = 1  // EV_CURRENT
	b[16] = 3 // e_type = ET_DYN
	b[18] = 3 // e_machine = EM_386
	b[20] = 1 // e_version = EV_CURRENT
	return b
}

// aarDePrueba arma el `.aar` mínimo: un `classes.jar` (vacío salvo su manifiesto), un `values` con
// una cadena y un color, un `layout` que referencia la cadena y un `.so` bajo `lib/x86/`.
func aarDePrueba(t *testing.T) []byte {
	t.Helper()

	clases := &bytes.Buffer{}
	cj := zip.NewWriter(clases)
	w, err := cj.Create("META-INF/MANIFEST.MF")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("Manifest-Version: 1.0\n")); err != nil {
		t.Fatal(err)
	}
	if err := cj.Close(); err != nil {
		t.Fatal(err)
	}

	entradas := map[string][]byte{
		"classes.jar": clases.Bytes(),
		"res/values/deps_strings-" + islaDePrueba + ".xml": []byte(`<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="deps_saludo_9f1c">Hola desde el AAR 4711</string>
    <color name="deps_color_9f1c">#ff3355</color>
</resources>`),
		"res/layout/deps_fila.xml": []byte(`<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="wrap_content">
    <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:text="@string/deps_saludo_9f1c" />
</LinearLayout>`),
		"lib/x86/libejemplo.so": soFalso(),
	}

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for nombre, datos := range entradas {
		w, err := zw.Create(nombre)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(datos); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// importaAAR reparte el artefacto en el proyecto como lo hace el importador: el jar a `jar/`, los
// recursos a la isla `res/__deps/<isla>/` y los nativos a `lib/<abi>/`, conservando la ruta relativa
// de estos dos últimos.
func importaAAR(t *testing.T, raiz string, aar []byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(aar), int64(len(aar)))
	if err != nil {
		t.Fatalf("el aar no abre como zip: %v", err)
	}
	for _, f := range zr.File {
		var destino string
		switch {
		case f.Name == "classes.jar":
			destino = "jar/com.ejemplo.deps-1.0.jar"
		case strings.HasPrefix(f.Name, "res/"):
			destino = "res/" + carpetaDeps + "/" + islaDePrueba + "/" + strings.TrimPrefix(f.Name, "res/")
		case strings.HasPrefix(f.Name, "lib/"):
			destino = f.Name
		default:
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		datos, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		pon(t, raiz, destino, string(datos))
	}
}

// TestFuentesDelProyectoLeeLaIslaDeDependencias mide la puerta: los ficheros de un AAR entran con la
// ruta de su tipo (la que deja `aapt2 link` en `build/res`), no con la de la isla, y el tipo se lee
// del nivel correcto de `res/__deps/<isla>/<tipo>`.
func TestFuentesDelProyectoLeeLaIslaDeDependencias(t *testing.T) {
	dir := proyecto(t, map[string]string{
		"AndroidManifest.xml": manifestMinimo,
		"res/values/app.xml":  valuesMinimo,
		"res/__deps/" + islaDePrueba + "/values/deps_strings-" + islaDePrueba + ".xml": `<resources><string name="deps_saludo_9f1c">hola</string></resources>`,
		"res/__deps/" + islaDePrueba + "/layout/deps_fila.xml":                         `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`,
		"res/__deps/" + islaDePrueba + "/drawable/deps_fondo.png":                      "PNG falso",
	})
	valores, layouts, ficheros, _, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(valores) != 2 {
		t.Fatalf("salieron %d values, se esperaban el del proyecto y el de la isla", len(valores))
	}
	// res/__deps va antes que res/values por orden alfabético: se busca por nombre, no por índice
	nombreIsla := "res/values/deps_strings-" + islaDePrueba + ".xml"
	encontrado := false
	for _, v := range valores {
		if v.nombre == nombreIsla {
			encontrado = true
		}
	}
	if !encontrado {
		t.Errorf("el values de la isla no sale como %q: %v", nombreIsla, valores)
	}
	if len(layouts) != 1 || layouts[0].nombre != "res/layout/deps_fila.xml" {
		t.Fatalf("el layout de la isla sale %v", layouts)
	}
	if len(ficheros) != 1 || ficheros[0].nombre != "res/drawable/deps_fondo.png" {
		t.Fatalf("el drawable de la isla sale %v", ficheros)
	}
	// el tipo se lee del nivel de la isla, que es lo que numera la tabla
	if tipo := tipoDeRecurso(layouts[0].nombre); tipo != "layout" {
		t.Errorf("tipoDeRecurso(%q) = %q", layouts[0].nombre, tipo)
	}
	if tipo := tipoDeRecurso("res/" + carpetaDeps + "/" + islaDePrueba + "/drawable/deps_fondo.png"); tipo != "drawable" {
		t.Errorf("el tipo del drawable de la isla sale %q", tipo)
	}

	// y lo que no es una carpeta de una dependencia se rechaza por su nombre
	mala := proyecto(t, map[string]string{
		"AndroidManifest.xml":   manifestMinimo,
		"res/__deps/suelto.xml": `<resources/>`,
	})
	if _, _, _, _, err := fuentesDelProyecto(mala); err == nil || !strings.Contains(err.Error(), "no es la carpeta de una dependencia") {
		t.Errorf("un fichero suelto en res/__deps no se rechazó: %v", err)
	}
}

// TestRecursosDeDependenciaEnElApk es la punta: el build real con los dos backends de Go enlaza el
// `values` y el `layout` del AAR en `resources.arsc`/`build/res` y el apk lleva el `.so` en la ruta
// que Android carga (`lib/x86/libejemplo.so`). Después mide el incremental por contenido: sin tocar
// la isla el apk no se rehace, y al editar su cadena o cambiar el `.so` sí.
func TestRecursosDeDependenciaEnElApk(t *testing.T) {
	androidJar = jarDelSDK(t)
	setupProject(t)
	raiz, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	keystore := filepath.Join(raiz, "debug.keystore")
	if err := escribirDebugKeystore(keystore, "android", "androiddebugkey", 10000); err != nil {
		t.Fatalf("escribirDebugKeystore: %v", err)
	}

	writeManifest(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.myapp">
	<uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name"/>
</manifest>`)
	pon(t, raiz, "res/values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Recursos</string>
</resources>`)
	pon(t, raiz, "src/com/myapp/Hola.java", "package com.myapp;\n\npublic class Hola {\n    static int n() { return 1; }\n}\n")

	importaAAR(t, raiz, aarDePrueba(t))
	// la mitad de clases de `jar/` la consume el compilador Go en otro frente (hoy
	// compilar.go:143 la rechaza); aquí se retira para medir recursos y nativos sin mezclar frentes
	if err := os.Remove(filepath.Join(raiz, "jar", "com.ejemplo.deps-1.0.jar")); err != nil {
		t.Fatalf("la importación no dejó el jar de la dependencia: %v", err)
	}

	opciones := buildOpts{
		incremental: true,
		linker:      linkerGo,
		compiler:    compilerGo,
		signer:      signerGo,
		keyStore:    keystore,
		storePass:   "android",
		keyAlias:    "androiddebugkey",
		jar:         androidJar,
		sdkVersion:  34,
		codename:    "14",
		v4:          true,
	}
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(opciones)
	if !contieneEnArsc(t, "Hola desde el AAR 4711") {
		t.Fatal("la cadena del AAR no llegó a resources.arsc: la isla no se enlazó")
	}
	if !contieneEnArsc(t, "deps_saludo_9f1c") {
		t.Fatal("el nombre del recurso del AAR no está en la tabla")
	}
	layout, err := os.ReadFile(filepath.Join("build", "res", "layout", "deps_fila.xml"))
	if err != nil {
		t.Fatalf("el layout del AAR no se escribió donde el paquete lo busca: %v", err)
	}
	if _, err := leeAXML(layout); err != nil {
		t.Fatalf("el layout del AAR no es un AXML que se lea: %v", err)
	}
	enElApk(t, "res/layout/deps_fila.xml", nil)
	enElApk(t, "lib/x86/libejemplo.so", soFalso())
	verificada(t)
	frio := huellaApk(t)

	// una vuelta sin tocar la isla no rehace el paquete
	_build(opciones)
	if huellaApk(t) != frio {
		t.Fatal("una vuelta sin cambios en la isla rehízo el apk")
	}

	// editar un value de la dependencia sí: la tabla tiene que traer el texto nuevo
	pon(t, raiz, "res/__deps/"+islaDePrueba+"/values/deps_strings-"+islaDePrueba+".xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="deps_saludo_9f1c">Hola desde el AAR 8822</string>
    <color name="deps_color_9f1c">#ff3355</color>
</resources>`)
	_build(opciones)
	if !contieneEnArsc(t, "Hola desde el AAR 8822") {
		t.Fatal("con --inc, editar un value de la dependencia no llegó a resources.arsc")
	}
	if huellaApk(t) == frio {
		t.Fatal("con --inc, editar un value de la dependencia dejó el mismo apk")
	}
	verificada(t)
	editado := huellaApk(t)

	// y cambiar solo el nativo también: el roster de lib/ es lo que decide el paquete
	otroSo := soFalso()
	otroSo[19] = 0x08 // un byte distinto en e_machine: el fichero es otro
	pon(t, raiz, "lib/x86/libejemplo.so", string(otroSo))
	_build(opciones)
	if huellaApk(t) == editado {
		t.Fatal("con --inc, cambiar un .so de la dependencia dejó el mismo apk")
	}
	enElApk(t, "lib/x86/libejemplo.so", otroSo)
	verificada(t)
}

// enElApk exige que la entrada exista en el apk firmado y, si se da el contenido, que sea el mismo.
func enElApk(t *testing.T, nombre string, contenido []byte) {
	t.Helper()
	abierto, err := zip.OpenReader(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("el apk no abre como zip: %v", err)
	}
	defer abierto.Close()
	for _, f := range abierto.File {
		if f.Name != nombre {
			continue
		}
		if contenido == nil {
			return
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		datos, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(datos, contenido) {
			t.Fatalf("%s viaja con %d bytes, se esperaban los %d del fichero del proyecto", nombre, len(datos), len(contenido))
		}
		return
	}
	t.Fatalf("el apk no lleva la entrada %s", nombre)
}

// TestRecursosDeFicheroRepetidos mide el choque que abre la isla: la ruta de un layout no lleva la
// isla, así que la app y un AAR con el mismo `layout/main.xml` pelearían por la misma entrada.
func TestRecursosDeFicheroRepetidos(t *testing.T) {
	propio := fuenteXML{nombre: "res/layout/main.xml"}
	deLaIsla := fuenteXML{nombre: "res/layout/main.xml"}
	if err := recursosDeFicheroRepetidos([]fuenteXML{propio}, []fuenteXML{deLaIsla}); err == nil ||
		!strings.Contains(err.Error(), "res/layout/main.xml") {
		t.Errorf("dos layouts con la misma ruta no se rechazaron: %v", err)
	}
	// dos configuraciones distintas del mismo nombre sí conviven
	if err := recursosDeFicheroRepetidos([]fuenteXML{
		{nombre: "res/layout/main.xml"},
		{nombre: "res/layout-land/main.xml", config: "land"},
	}, nil); err != nil {
		t.Errorf("dos configuraciones del mismo layout se rechazaron: %v", err)
	}
	// y un drawable y un layout con el mismo nombre son tipos distintos
	if err := recursosDeFicheroRepetidos([]fuenteXML{{nombre: "res/layout/main.xml"}}, []fuenteXML{{nombre: "res/drawable/main.png"}}); err != nil {
		t.Errorf("un layout y un drawable con el mismo nombre se rechazaron: %v", err)
	}
}
