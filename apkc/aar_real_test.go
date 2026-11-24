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

	// el layout que escribe el SDK para un AAR es `jni/<abi>/`, no `lib/<abi>/`: el fixture usa el
	// real para que la prueba mida lo que de verdad se suelta en `jar/`
	return zipDeAar(t, map[string][]byte{
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
		"jni/arm64-v8a/libfoo.so": so,
	})
}

// zipDeAar escribe el zip de un `.aar` con las entradas en orden estable, para que el mismo
// contenido dé los mismos bytes y la misma huella y un build incremental no vea un cambio donde no
// lo hay. Un `.so` entra almacenado, que es como lo deja el SDK.
func zipDeAar(t *testing.T, entradas map[string][]byte) []byte {
	t.Helper()

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

// aarDeSoloNativos arma un `.aar` que no trae clases ni recursos: solo un `.so` en el layout que se
// le pida (`jni/<abi>/` o `lib/<abi>/`). Un AAR así es legal y el build no debe exigirle
// `classes.jar`.
func aarDeSoloNativos(t *testing.T, layout, abi, fichero string, so []byte) []byte {
	t.Helper()
	return zipDeAar(t, map[string][]byte{
		layout + "/" + abi + "/" + fichero: so,
	})
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

	// un oráculo ajeno lista el zip: `unzip -l` no comparte código con el empaquetador, y el `.so`
	// tiene que estar con la ruta que busca System.loadLibrary
	if unzipBin, err := exec.LookPath("unzip"); err == nil {
		salida, err := exec.Command(unzipBin, "-l", filepath.Join("build", "app.apk")).CombinedOutput()
		if err != nil {
			t.Fatalf("unzip -l no pudo listar el apk: %v\n%s", err, salida)
		}
		if !strings.Contains(string(salida), "lib/arm64-v8a/libfoo.so") {
			t.Fatalf("unzip -l no ve el .so del AAR:\n%s", salida)
		}
		t.Logf("unzip -l: lib/arm64-v8a/libfoo.so presente")
	} else {
		t.Logf("sin unzip: la entrada ya se midió con archive/zip")
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

// planDelPaquete extrae los AAR del proyecto actual y arma el plan del paquete, devolviendo también
// lo que se escribió por `LogW`: ahí sale el aviso de una colisión de nativos.
func planDelPaquete(t *testing.T) ([]bundleEntry, string) {
	t.Helper()
	aares, err := aarsExtraidos("")
	if err != nil {
		t.Fatalf("extraer los AAR: %v", err)
	}
	aaresDelProyecto = aares
	t.Cleanup(func() { aaresDelProyecto = nil })
	var plan []bundleEntry
	avisos := capturarSalida(t, func() {
		plan = packagePlan(false, nil, nil, nativeLibs{})
	})
	return plan, avisos
}

// entradasConNombre devuelve las entradas del plan con ese nombre: el mismo `.so` no puede viajar
// dos veces, y un fallo enumera lo que sí lleva.
func entradasConNombre(plan []bundleEntry, nombre string) []bundleEntry {
	out := []bundleEntry{}
	for _, e := range plan {
		if e.name == nombre {
			out = append(out, e)
		}
	}
	return out
}

// TestAvisoDeColisionDeNativosNombraAlGanadorReal cubre las tres formas de que dos fuentes aporten
// la misma ruta de `System.loadLibrary`: gana el proyecto, gana el primer AAR leído o el mismo AAR
// trae el `.so` por los dos layouts. En los tres casos el paquete lleva una sola entrada y el aviso
// nombra a quien de verdad se quedó, no al proyecto por suposición.
func TestAvisoDeColisionDeNativosNombraAlGanadorReal(t *testing.T) {
	soA := soElfAar()
	soB := soElfAar()
	soB[19] = 0x08

	t.Run("el proyecto gana al AAR", func(t *testing.T) {
		setupProject(t)
		pon(t, "", "lib/arm64-v8a/foo.so", "nativo del proyecto")
		if err := os.MkdirAll("jar", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("jar", "uno.aar"),
			aarDeSoloNativos(t, "jni", "arm64-v8a", "foo.so", soA), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, avisos := planDelPaquete(t)
		got := entradasConNombre(plan, "lib/arm64-v8a/foo.so")
		if len(got) != 1 {
			t.Fatalf("el plan lleva %d entradas lib/arm64-v8a/foo.so y solo cabe una: %+v", len(got), got)
		}
		if got[0].src != filepath.Join("lib", "arm64-v8a", "foo.so") {
			t.Errorf("se quedó el .so de %s y se esperaba el del proyecto", got[0].src)
		}
		for _, quiero := range []string{"ya lo aporta el proyecto", "jar/uno.aar", "jni/arm64-v8a/foo.so"} {
			if !strings.Contains(avisos, quiero) {
				t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
			}
		}
		if strings.Contains(avisos, "ya lo aporta el AAR") {
			t.Errorf("el aviso dice que ganó un AAR cuando ganó el proyecto: %s", avisos)
		}
	})

	t.Run("dos AAR: gana el primero y el aviso lo nombra", func(t *testing.T) {
		setupProject(t)
		if err := os.MkdirAll("jar", 0o755); err != nil {
			t.Fatal(err)
		}
		for nombre, so := range map[string][]byte{
			"a-util.aar": soA,
			"b-otro.aar": soB,
		} {
			if err := os.WriteFile(filepath.Join("jar", nombre),
				aarDeSoloNativos(t, "jni", "arm64-v8a", "foo.so", so), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		plan, avisos := planDelPaquete(t)
		got := entradasConNombre(plan, "lib/arm64-v8a/foo.so")
		if len(got) != 1 {
			t.Fatalf("el plan lleva %d entradas lib/arm64-v8a/foo.so y solo cabe una: %+v", len(got), got)
		}
		datos, err := os.ReadFile(got[0].src)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(datos, soA) {
			t.Error("se quedó el .so del segundo AAR y la prioridad es el orden de lectura")
		}
		for _, quiero := range []string{"ya lo aporta el AAR", "jar/a-util.aar", "jar/b-otro.aar", "jni/arm64-v8a/foo.so", "no lo sustituye"} {
			if !strings.Contains(avisos, quiero) {
				t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
			}
		}
		if strings.Contains(avisos, "ya lo aporta el proyecto") {
			t.Errorf("el aviso culpa al proyecto cuando ganó un AAR: %s", avisos)
		}
	})

	t.Run("el mismo AAR con lib y jni: se queda el primero", func(t *testing.T) {
		setupProject(t)
		if err := os.MkdirAll("jar", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("jar", "uno.aar"), zipDeAar(t, map[string][]byte{
			"lib/arm64-v8a/foo.so": soA,
			"jni/arm64-v8a/foo.so": soB,
		}), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, avisos := planDelPaquete(t)
		got := entradasConNombre(plan, "lib/arm64-v8a/foo.so")
		if len(got) != 1 {
			t.Fatalf("el plan lleva %d entradas lib/arm64-v8a/foo.so y solo cabe una: %+v", len(got), got)
		}
		for _, quiero := range []string{"jar/uno.aar", "dos veces", "lib/arm64-v8a/foo.so", "jni/arm64-v8a/foo.so", "se queda la primera"} {
			if !strings.Contains(avisos, quiero) {
				t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
			}
		}
	})

	t.Run("un AAR con el layout viejo lib sigue aportando su .so", func(t *testing.T) {
		setupProject(t)
		if err := os.MkdirAll("jar", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("jar", "viejo.aar"),
			aarDeSoloNativos(t, "lib", "arm64-v8a", "foo.so", soA), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, avisos := planDelPaquete(t)
		got := entradasConNombre(plan, "lib/arm64-v8a/foo.so")
		if len(got) != 1 {
			t.Fatalf("el plan no lleva el .so del AAR con lib/: %+v", got)
		}
		if strings.Contains(avisos, "ya lo aporta") || strings.Contains(avisos, "dos veces") {
			t.Errorf("un solo AAR no colisiona consigo mismo: %s", avisos)
		}
	})
}

// TestApkConDosAarsLlevaUnSoloSo es la punta del aviso: dos AAR de `jar/` aportan el mismo `.so`,
// y el paquete firmado lleva una sola entrada, con los bytes del que ganó.
func TestApkConDosAarsLlevaUnSoloSo(t *testing.T) {
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
	writeManifest(t, `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.hola.dos">
	<uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
	<application android:label="@string/app_name" android:extractNativeLibs="false"/>
</manifest>`)
	pon(t, raiz, "res/values/app.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Dos AAR</string>
</resources>`)
	pon(t, raiz, "src/com/hola/dos/Main.java", `package com.hola.dos;

public class Main {
    static int uno() { return 1; }
}
`)

	soA := soElfAar()
	soB := soElfAar()
	soB[19] = 0x08
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "jar", "a-util.aar"),
		aarConRecursosYNativos(t, "hola desde el primero", soA), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "jar", "b-otro.aar"),
		aarDeSoloNativos(t, "jni", "arm64-v8a", "libfoo.so", soB), 0o644); err != nil {
		t.Fatal(err)
	}

	// `_build` fija `escribirV4` y `v4SaltHex` con las opciones y no los devuelve: sin guardarlos,
	// esta prueba deja la v4 apagada para las siguientes del paquete
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	_build(buildOpts{
		linker:     linkerGo,
		compiler:   compilerGo,
		signer:     signerGo,
		keyStore:   keystore,
		storePass:  "android",
		keyAlias:   "androiddebugkey",
		jar:        androidJar,
		sdkVersion: 34,
		codename:   "14",
	})

	nombres := entradasDelApk(t)
	cuantas := 0
	for _, nombre := range nombres {
		if nombre == "lib/arm64-v8a/libfoo.so" {
			cuantas++
		}
	}
	if cuantas != 1 {
		t.Fatalf("el apk lleva %d entradas lib/arm64-v8a/libfoo.so y solo cabe una; lleva %v", cuantas, nombres)
	}
	datos := leerEntradaDelApk(t, "lib/arm64-v8a/libfoo.so")
	if !bytes.Equal(datos, soA) {
		t.Fatal("el apk lleva el .so del segundo AAR: la prioridad tiene que ser el orden de lectura")
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
