package main

// El `.aar` que se puede soltar en `jar/` sin pasar por el importador: su `classes.jar` ya se dexaba,
// pero su `res/` y su `lib/<abi>/` no llegaban al paquete, así que un layout del artefacto no
// existía para `R` y un `.so` no viajaba. Aquí se arma un AAR completo con `archive/zip` y un ELF de
// cabecera mínima (sin binarios versionados ni descargas), se construye con los dos backends de Go y
// se mide el paquete por dentro y por los oráculos del SDK: `unzip` lógico sobre el zip, la tabla, la
// `R` que compila el proyecto, `apksigner verify` y `zipalign -c -p 4`.
//
// Después se mide que el AAR participa del incremental por sus bytes: una vuelta sin tocarlo no
// rehace el paquete, y cambiar su cadena o su `.so` sí, sin que quede el recurso ni el nativo viejos.

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// soElfAar es una cabecera ELF de 64 bits para aarch64 (`EM_AARCH64`), sin secciones ni símbolos:
// el empaquetador no interpreta un `.so`, pero el fichero tiene la forma que Android espera cargar.
func soElfAar() []byte {
	b := make([]byte, 64)
	copy(b, "\x7fELF")
	b[4] = 2    // ELFCLASS64
	b[5] = 1    // ELFDATA2LSB
	b[6] = 1    // EV_CURRENT
	b[16] = 3   // e_type = ET_DYN
	b[18] = 183 // e_machine = EM_AARCH64
	b[20] = 1   // e_version = EV_CURRENT
	b[52] = 64  // e_ehsize
	b[54] = 56  // e_phentsize
	return b
}

// aarConRecursosYNativos arma el `.aar` entero: manifiesto, `R.txt`, un `classes.jar` con una clase
// real (el fixture del lector, no un zip vacío) y las tres partes que este frente conecta.
func aarConRecursosYNativos(t *testing.T, saludo string, so []byte) []byte {
	t.Helper()

	clase := classFixture{
		name:   "com/hola/util/Util",
		super:  "java/lang/Object",
		access: 0x0021,
		methods: []memberFixture{{
			name:       "valor",
			descriptor: "()I",
			access:     0x0009,
			code:       []byte{0x03, 0xac},
		}},
	}
	clases := &bytes.Buffer{}
	cj := zip.NewWriter(clases)
	for _, e := range []struct {
		nombre string
		datos  []byte
	}{
		{"META-INF/MANIFEST.MF", []byte("Manifest-Version: 1.0\n")},
		{"com/hola/util/Util.class", clase.bytes()},
	} {
		w, err := cj.Create(e.nombre)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.datos); err != nil {
			t.Fatal(err)
		}
	}
	if err := cj.Close(); err != nil {
		t.Fatal(err)
	}

	entradas := map[string][]byte{
		"AndroidManifest.xml": []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.hola.util"/>`),
		"R.txt":               []byte("int string aar_saludo 0x0\nint layout aar_panel 0x0\n"),
		"classes.jar":         clases.Bytes(),
		"res/values/aar_strings.xml": []byte(`<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="aar_saludo">` + saludo + `</string>
</resources>`),
		"res/layout/aar_panel.xml": []byte(`<?xml version="1.0" encoding="utf-8"?>
<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="wrap_content">
    <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:text="@string/aar_saludo" />
</FrameLayout>`),
		"lib/arm64-v8a/libfoo.so": so,
	}

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	nombres := make([]string, 0, len(entradas))
	for nombre := range entradas {
		nombres = append(nombres, nombre)
	}
	// orden estable: el mismo AAR da el mismo fichero y la misma huella
	for i := 0; i < len(nombres); i++ {
		for j := i + 1; j < len(nombres); j++ {
			if nombres[j] < nombres[i] {
				nombres[i], nombres[j] = nombres[j], nombres[i]
			}
		}
	}
	for _, nombre := range nombres {
		metodo := zip.Deflate
		if strings.HasSuffix(nombre, ".so") {
			metodo = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: nombre, Method: metodo})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entradas[nombre]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// entradaApk es lo que se mide de una entrada del paquete sin dejar el zip abierto: su método, el
// desplazamiento de sus datos y sus bytes.
type entradaApk struct {
	metodo uint16
	offset int64
	datos  []byte
}

// entradaDelApk devuelve la entrada del apk firmado y si está; quien la mide nombra lo que falta.
func entradaDelApk(t *testing.T, nombre string) (entradaApk, bool) {
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
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		datos, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		offset, err := f.DataOffset()
		if err != nil {
			t.Fatal(err)
		}
		return entradaApk{metodo: f.Method, offset: offset, datos: datos}, true
	}
	return entradaApk{}, false
}

// entradasDelApk devuelve los nombres de las entradas del apk, para que un fallo diga qué sí lleva.
func entradasDelApk(t *testing.T) []string {
	t.Helper()
	abierto, err := zip.OpenReader(filepath.Join("build", "app.apk"))
	if err != nil {
		t.Fatalf("el apk no abre como zip: %v", err)
	}
	defer abierto.Close()
	nombres := make([]string, 0, len(abierto.File))
	for _, f := range abierto.File {
		nombres = append(nombres, f.Name)
	}
	return nombres
}

// TestAarEnJarAportaRecursosYNativos es la punta del frente: el AAR está solo en `jar/` (como lo deja
// cualquiera que copie el artefacto), y aun así el proyecto compila `R.layout.aar_panel`, la tabla
// lleva su cadena, el layout viaja compilado y el `.so` entra almacenado y alineado a página.
func TestAarEnJarAportaRecursosYNativos(t *testing.T) {
	apksigner := herramientaDelSDK(t, "apksigner")
	zipalign := herramientaDelSDK(t, "zipalign")
	if apksigner == "" || zipalign == "" {
		t.Skip("sin apksigner y zipalign en las build-tools: los oráculos del SDK no se inventan")
	}

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

	writeManifest(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.hola.aarprobe">
	<uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name" android:extractNativeLibs="false"/>
</manifest>`)
	pon(t, raiz, "res/values/app.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Aar real</string>
</resources>`)
	// el fuente que reproduce el fallo medido: el campo tiene que existir en la R que escribe el
	// enlace, o el compilador de Go rechaza el proyecto
	pon(t, raiz, "src/com/hola/aarprobe/Usa.java", `package com.hola.aarprobe;

public class Usa {
    static int panel() { return R.layout.aar_panel; }
    static int saludo() { return R.string.aar_saludo; }
}
`)

	so := soElfAar()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "jar", "libutil-1.0.aar"),
		aarConRecursosYNativos(t, "Hola desde el AAR real", so), 0o644); err != nil {
		t.Fatal(err)
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

	// los recursos del AAR, enlazados y en el paquete
	if !contieneEnArsc(t, "Hola desde el AAR real") {
		t.Fatal("la cadena del AAR no llegó a resources.arsc: su res/ no se enlazó")
	}
	if !contieneEnArsc(t, "aar_panel") {
		t.Fatal("el nombre del layout del AAR no está en la tabla")
	}
	for _, quiero := range []string{"aar_panel", "aar_saludo"} {
		if !strings.Contains(leerRJava(t), quiero) {
			t.Errorf("R.java no declara %q: el proyecto no puede compilar contra el AAR", quiero)
		}
	}
	layout, err := os.ReadFile(filepath.Join("build", "res", "layout", "aar_panel.xml"))
	if err != nil {
		t.Fatalf("el layout del AAR no se escribió donde el paquete lo busca: %v", err)
	}
	if _, err := leeAXML(layout); err != nil {
		t.Fatalf("el layout del AAR no es un AXML legible: %v", err)
	}

	// `unzip -l` del apk: el recurso compilado y el nativo con su ruta canónica
	if _, hay := entradaDelApk(t, "res/layout/aar_panel.xml"); !hay {
		t.Fatalf("el apk no lleva el layout del AAR; lleva %v", entradasDelApk(t))
	}
	soEnApk, hay := entradaDelApk(t, "lib/arm64-v8a/libfoo.so")
	if !hay {
		t.Fatalf("el apk no lleva el .so del AAR; lleva %v", entradasDelApk(t))
	}
	if !bytes.Equal(soEnApk.datos, so) {
		t.Fatalf("el .so viaja con %d bytes, se esperaban los %d del AAR", len(soEnApk.datos), len(so))
	}
	if soEnApk.metodo != zip.Store {
		t.Errorf("el .so viaja comprimido (método %d) y android:extractNativeLibs=false pide almacenarlo", soEnApk.metodo)
	}
	if resto := soEnApk.offset % 4096; resto != 0 {
		t.Errorf("el .so empieza en %d, que no es múltiplo de página (resto %d)", soEnApk.offset, resto)
	}

	// los oráculos del SDK, por fuera de nuestro código
	salida, err := exec.Command(apksigner, "verify", "--verbose", filepath.Join("build", "app.apk")).CombinedOutput()
	if err != nil {
		t.Fatalf("apksigner verify falló: %v\n%s", err, salida)
	}
	t.Logf("apksigner verify --verbose: rc 0, %d líneas", len(strings.Split(strings.TrimSpace(string(salida)), "\n")))
	for _, linea := range strings.Split(strings.TrimSpace(string(salida)), "\n") {
		if strings.Contains(linea, "Verified using") {
			t.Logf("  %s", strings.TrimSpace(linea))
		}
	}
	if salida, err := exec.Command(zipalign, "-c", "-p", "4", filepath.Join("build", "app.apk")).CombinedOutput(); err != nil {
		t.Fatalf("zipalign no da el apk por alineado: %v\n%s", err, salida)
	} else {
		t.Logf("zipalign -c -p 4: rc 0")
	}
	t.Logf("entradas del apk: %v", entradasDelApk(t))
	frio := huellaApk(t)

	// una vuelta sin tocar el AAR no rehace el paquete
	_build(opciones)
	if huellaApk(t) != frio {
		t.Fatal("una vuelta sin cambios en el AAR rehízo el apk")
	}

	// cambiar la cadena y el .so del AAR sí: la tabla trae el texto nuevo y el nativo nuevo
	otro := soElfAar()
	otro[19] = 0x08
	if err := os.WriteFile(filepath.Join(raiz, "jar", "libutil-1.0.aar"),
		aarConRecursosYNativos(t, "Hola desde el AAR cambiado", otro), 0o644); err != nil {
		t.Fatal(err)
	}
	_build(opciones)
	if huellaApk(t) == frio {
		t.Fatal("cambiar el AAR dejó el mismo apk")
	}
	if !contieneEnArsc(t, "Hola desde el AAR cambiado") {
		t.Fatal("la cadena nueva del AAR no llegó a la tabla: el enlace no vio el cambio")
	}
	datos := leerEntradaDelApk(t, "lib/arm64-v8a/libfoo.so")
	if !bytes.Equal(datos, otro) {
		t.Fatalf("el apk conserva el .so anterior; viaja % x", datos[:8])
	}
	verificada(t)
	if salida, err := exec.Command(zipalign, "-c", "-p", "4", filepath.Join("build", "app.apk")).CombinedOutput(); err != nil {
		t.Fatalf("tras cambiar el AAR, zipalign no da el apk por alineado: %v\n%s", err, salida)
	}
}

// leerRJava devuelve la R que el enlace escribió en el árbol del proyecto.
func leerRJava(t *testing.T) string {
	t.Helper()
	datos, err := os.ReadFile(filepath.Join("src", "com", "hola", "aarprobe", "R.java"))
	if err != nil {
		t.Fatalf("el enlace no escribió la R del proyecto: %v", err)
	}
	return string(datos)
}

// leerEntradaDelApk lee los bytes de una entrada del apk, con la ruta que se mide.
func leerEntradaDelApk(t *testing.T, nombre string) []byte {
	t.Helper()
	e, hay := entradaDelApk(t, nombre)
	if !hay {
		t.Fatalf("el apk no lleva la entrada %s; lleva %v", nombre, entradasDelApk(t))
	}
	return e.datos
}
