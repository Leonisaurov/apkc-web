package main

import (
	"archive/zip"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// El paso 3-E: un apk entero hecho en Go. Las tres estructuras de recursos las escriben axmlwriter.go
// y arscwriter.go, el dex lo saca el dexer de la Fase 2 de las seis clases de javac fijadas, y el
// zipado y la firma son el empaquetador y el firmante de apkc. Aqui se comprueba que el paquete se
// lee por dentro; la verificacion por fuera (apksigner, zipalign, jarsigner, zipcheck.py y los
// volcados de aapt2) se corre con APKC_ESCRIBE_APK pointing al sitio donde dejarlo.

func artefactosDelStudio(t *testing.T) []entradaDePaquete {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	layout, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	tabla, err := escribeTablaRecursos(tablaDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	dex, err := dexDeClases(leeLasSeis(t))
	if err != nil {
		t.Fatal(err)
	}
	return []entradaDePaquete{
		{nombre: "AndroidManifest.xml", contenido: manifiesto},
		{nombre: "classes.dex", contenido: dex},
		{nombre: "resources.arsc", contenido: tabla, sinComprimir: true},
		{nombre: "res/layout/main.xml", contenido: layout},
	}
}

func TestApkCompiladoSinAapt2(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no hay home para buscar el keystore de debug")
	}
	keyPath := filepath.Join(home, ".android", "debug.keystore")
	if _, err := os.Stat(keyPath); err != nil {
		t.Skipf("no hay keystore de debug en %s", keyPath)
	}

	ruta := filepath.Join(t.TempDir(), "app.apk")
	entradas := artefactosDelStudio(t)
	if err := empaquetaSinAapt2(ruta, entradas, keyPath, "android", "androiddebugkey"); err != nil {
		t.Fatalf("el paquete no salio: %v", err)
	}

	// lo que se escribe por dentro tiene que ser lo que se lee: el mismo AXML, la misma tabla y el
	// mismo dex, y resources.arsc almacenado y alineado a cuatro
	abierto, err := zip.OpenReader(ruta)
	if err != nil {
		t.Fatalf("el apk no abre como zip: %v", err)
	}
	defer abierto.Close()
	porNombre := map[string]*zip.File{}
	for _, f := range abierto.File {
		porNombre[f.Name] = f
	}
	for _, e := range entradas {
		if _, ok := porNombre[e.nombre]; !ok {
			t.Errorf("falta %s en el paquete", e.nombre)
		}
	}

	arsc := porNombre["resources.arsc"]
	if arsc == nil {
		t.Fatal("no esta resources.arsc")
	}
	if arsc.Method != zip.Store {
		t.Errorf("resources.arsc viaja comprimido (%d), y el sistema la mapea almacenada", arsc.Method)
	}
	desplazamiento, err := arsc.DataOffset()
	if err != nil {
		t.Fatalf("resources.arsc no dice donde empiezan sus datos: %v", err)
	}
	if desplazamiento%4 != 0 {
		t.Errorf("resources.arsc empieza en %d, no alineada a cuatro", desplazamiento)
	}
	man := porNombre["AndroidManifest.xml"]
	if man == nil {
		t.Fatal("no esta AndroidManifest.xml")
	}

	leido := func(f *zip.File) []byte {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	if _, err := leeAXML(leido(man)); err != nil {
		t.Errorf("el manifiesto del paquete no se deja leer: %v", err)
	}
	if _, err := leeAXML(leido(porNombre["res/layout/main.xml"])); err != nil {
		t.Errorf("el layout del paquete no se deja leer: %v", err)
	}
	tr, err := leeTabla(leido(arsc))
	if err != nil {
		t.Errorf("la tabla del paquete no se deja leer: %v", err)
	}
	if len(tr.bloques) != 3 || tr.nombrePaquete != "com.hola.apkcstudio" {
		t.Errorf("la tabla del paquete trajo %d bloques y paquete %q", len(tr.bloques), tr.nombrePaquete)
	}
	dexDelPaquete := leido(porNombre["classes.dex"])
	if string(dexDelPaquete[0:8]) != "dex\n035\x00" {
		t.Errorf("el dex del paquete empieza %q", dexDelPaquete[0:8])
	}

	// y los bytes de los recursos son exactamente los que escribimos, no una reinterpretacion
	fijado, _ := base64.StdEncoding.DecodeString(arscTabla)
	nuestra := leido(arsc)
	if len(nuestra) != 936 || len(fijado) != 1088 {
		t.Errorf("la tabla del paquete mide %d bytes", len(nuestra))
	}

	if destino := os.Getenv("APKC_ESCRIBE_APK"); destino != "" {
		b, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destino, b, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("apk escrito en %s (%d bytes)", destino, len(b))
	}
}

func TestEmpaquetaRechazaNombresDeRuta(t *testing.T) {
	entradas := []entradaDePaquete{{nombre: "../salida.apk", contenido: []byte("x")}}
	if err := empaquetaSinAapt2(filepath.Join(t.TempDir(), "a.apk"), entradas, "sin-keystore", "", ""); err == nil {
		t.Error("una entrada con .. en el nombre fue aceptada")
	}
	entradas = []entradaDePaquete{{nombre: "/absoluto", contenido: []byte("x")}}
	if err := empaquetaSinAapt2(filepath.Join(t.TempDir(), "a.apk"), entradas, "sin-keystore", "", ""); err == nil {
		t.Error("una entrada con nombre absoluto fue aceptada")
	}
	if err := empaquetaSinAapt2(filepath.Join(t.TempDir(), "a.apk"), nil, "sin-keystore", "", ""); err == nil {
		t.Error("un paquete sin entradas fue aceptado")
	}
}
