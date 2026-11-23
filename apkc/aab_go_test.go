package main

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// preparaEnlaceGo corre el enlazador Go sobre el proyecto del studio y deja en build/ los
// cuatro productos que el ensamblador consume: manifiesto AXML, resources.arsc, el layout
// AXML y un classes.dex con la cabecera que bundletool acepta (medido: un dex de relleno
// con "dex\n035\0" pasa validate).
func preparaEnlaceGo(t *testing.T) string {
	t.Helper()
	valores, layouts, manifiesto := fuentesDelStudio(t)
	salida, err := enlazarProyecto(valores, layouts, nil, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace Go no salió: %v", err)
	}
	dir := t.TempDir()
	build := filepath.Join(dir, "build")
	if err := os.MkdirAll(filepath.Join(build, "res", "layout"), 0o755); err != nil {
		t.Fatal(err)
	}
	man, err := escribeAXML(salida.manifiesto)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(build, "AndroidManifest.xml"), man, 0o644); err != nil {
		t.Fatal(err)
	}
	arsc, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(build, "resources.arsc"), arsc, 0o644); err != nil {
		t.Fatal(err)
	}
	for ruta, doc := range salida.layouts {
		axml, err := escribeAXML(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(build, filepath.FromSlash(ruta)), axml, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for ruta, datos := range salida.ficheros {
		destino := filepath.Join(build, filepath.FromSlash(ruta))
		os.MkdirAll(filepath.Dir(destino), 0o755)
		if err := os.WriteFile(destino, datos, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dex := append([]byte("dex\n035\x00"), make([]byte, 112)...)
	if err := os.WriteFile(filepath.Join(build, "classes.dex"), dex, 0o644); err != nil {
		t.Fatal(err)
	}
	return build
}

// TestElAABSeConstruyeDesdeElEnlazadorGo: la frontera 5.33 cerrada. El manifiesto AXML y
// el resources.arsc que produce el linker Go se convierten a los dos protobuf del bundle,
// se prefijan con base/ y bundletool validate lo acepta.
func TestElAABSeConstruyeDesdeElEnlazadorGo(t *testing.T) {
	build := preparaEnlaceGo(t)
	aab := filepath.Join(filepath.Dir(build), "app.aab")
	if err := ensamblarAABDesdeGo(build, dexEntries(build, nil), aab); err != nil {
		t.Fatalf("el .aab no se ensambló desde el linker Go: %v", err)
	}
	zr, err := zip.OpenReader(aab)
	if err != nil {
		t.Fatal(err)
	}
	tenidos := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		r.Close()
		tenidos[f.Name] = buf.Bytes()
	}
	zr.Close()
	for _, nombre := range []string{
		"BundleConfig.pb", "base/manifest/AndroidManifest.xml",
		"base/dex/classes.dex", "base/resources.pb", "base/res/layout/main.xml",
	} {
		datos, ok := tenidos[nombre]
		if !ok {
			t.Errorf("falta %s en el .aab", nombre)
			continue
		}
		if len(datos) == 0 {
			t.Errorf("%s está vacío", nombre)
		}
	}
	// El manifiesto proto debe empezar con el campo 1 (XmlNode.element), y la tabla
	// con el campo 2 (ResourceTable.package): medido sobre el bundle de referencia.
	if got := tenidos["base/manifest/AndroidManifest.xml"]; len(got) > 0 && got[0] != 0x0a {
		t.Errorf("el manifiesto proto no empieza por XmlNode.element: %x", got[:4])
	}
	if got := tenidos["base/resources.pb"]; len(got) > 0 && got[0] != 0x12 {
		t.Errorf("resources.pb no empieza por ResourceTable.package: %x", got[:4])
	}
	if !bytes.Equal(tenidos["BundleConfig.pb"], bundleConfigPB()) {
		t.Errorf("BundleConfig.pb = %x", tenidos["BundleConfig.pb"])
	}

	if volcado := os.Getenv("AAB_DUMP"); volcado != "" {
		datos, err := os.ReadFile(aab)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(volcado, datos, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	validaBundle(t, aab)
}

// TestElAABDelEnlazadorGoSeFirmaYVerifica: el bundle recién ensamblado entra por la misma
// firma JAR v1 que el camino de aapt2, y los tres oráculos lo reconocen (§128).
func TestElAABDelEnlazadorGoSeFirmaYVerifica(t *testing.T) {
	build := preparaEnlaceGo(t)
	dir := filepath.Dir(build)
	aab := filepath.Join(dir, "app.aab")
	if err := ensamblarAABDesdeGo(build, dexEntries(build, nil), aab); err != nil {
		t.Fatal(err)
	}
	key, cert := signingPair(t)
	firmante, err := firmanteDe(key, cert)
	if err != nil {
		t.Fatal(err)
	}
	firmado := filepath.Join(dir, "firmado.aab")
	if _, err := resignBundleCon(aab, firmado, firmante); err != nil {
		t.Fatalf("el .aab del linker Go no se firmó: %v", err)
	}
	v, err := verificarApk(firmado)
	if err != nil {
		t.Fatal(err)
	}
	if !v.bundle || !v.v1 {
		t.Errorf("el verificador propio no reconoce el bundle: bundle=%v v1=%v (%v)", v.bundle, v.v1, v.v1Motivos)
	}
	verificaConJarsigner(t, firmado)
	validaBundle(t, firmado)
}

// TestElAABNoRepiteElDexDeLosBuckets es la falsificación del defecto que midió el banco
// (aab-multidex con PROBAR_DEPENDENCIAS=1): el ensamblador recorría build/ entero y nombraba
// cada dex con filepath.Base, así que el classes.dex que un bucket de dependencias deja bajo
// build/dexcache/… caía sobre el de la app, el .aab listaba base/dex/classes.dex dos veces y
// la firma lo rechazaba (sign.go). La prueba reproduce esa lista sobre el mismo árbol y exige
// el error de duplicados; después, con la lista de dexEntries que usa el plan del apk, el mismo
// árbol produce classes.dex + classes2.dex y firma y verifica.
func TestElAABNoRepiteElDexDeLosBuckets(t *testing.T) {
	build := preparaEnlaceGo(t)
	// un bucket de dependencias real deja su dex bajo build/dexcache/deps-go-…/classes.dex
	bucket := filepath.Join(build, "dexcache", "deps-go-0123456789abcdef")
	if err := os.MkdirAll(bucket, 0o755); err != nil {
		t.Fatal(err)
	}
	dexDep := append([]byte("dex\n035\x00"), make([]byte, 112)...)
	dexDep[8] = 0x01 // distinto del dex de la app: si se cuela, se nota
	if err := os.WriteFile(filepath.Join(bucket, "classes.dex"), dexDep, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(build)

	// la lista vieja: todo .dex bajo build/, nombrado por su base. Se reproduce aquí, sobre
	// el árbol real del build, para que la falsificación sea del defecto y no del camino nuevo.
	rel, err := ficherosDeArbol(build)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rel)
	vieja := []bundleEntry{}
	for _, nombre := range rel {
		if filepath.Ext(nombre) != ".dex" {
			continue
		}
		vieja = append(vieja, bundleEntry{src: filepath.Join(build, nombre), name: filepath.Base(nombre)})
	}
	nombres := map[string]int{}
	for _, e := range vieja {
		nombres[e.name]++
	}
	if nombres["classes.dex"] != 2 {
		t.Fatalf("el árbol no reproduce el defecto: %v dex por nombre", nombres)
	}

	aabViejo := filepath.Join(dir, "viejo.aab")
	if err := ensamblarAABDesdeGo(build, vieja, aabViejo); err != nil {
		t.Fatal(err)
	}
	key, cert := signingPair(t)
	firmante, err := firmanteDe(key, cert)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resignBundleCon(aabViejo, filepath.Join(dir, "viejo-firmado.aab"), firmante)
	if err == nil {
		t.Fatal("el .aab con base/dex/classes.dex duplicado se firmó: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), "base/dex/classes.dex twice") {
		t.Fatalf("la firma falló por otra razón: %v", err)
	}
	t.Logf("con la lista vieja: %v", err)

	// la lista del plan: el dex de la app y el del bucket, numerados como en el apk
	nuevo := filepath.Join(dir, "nuevo.aab")
	if err := ensamblarAABDesdeGo(build, dexEntries(build, []string{filepath.Join(bucket, "classes.dex")}), nuevo); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(nuevo)
	if err != nil {
		t.Fatal(err)
	}
	tenidos := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		r.Close()
		tenidos[f.Name] = buf.Bytes()
	}
	zr.Close()
	for _, nombre := range []string{"base/dex/classes.dex", "base/dex/classes2.dex"} {
		if _, ok := tenidos[nombre]; !ok {
			t.Errorf("falta %s en el .aab", nombre)
		}
	}
	if !bytes.Equal(tenidos["base/dex/classes2.dex"], dexDep) {
		t.Errorf("base/dex/classes2.dex no lleva el dex del bucket")
	}
	firmado := filepath.Join(dir, "nuevo-firmado.aab")
	if _, err := resignBundleCon(nuevo, firmado, firmante); err != nil {
		t.Fatalf("el .aab con el dex de la app y el del bucket no se firmó: %v", err)
	}
	v, err := verificarApk(firmado)
	if err != nil {
		t.Fatal(err)
	}
	if !v.bundle || !v.v1 {
		t.Errorf("el verificador propio no reconoce el bundle: bundle=%v v1=%v (%v)", v.bundle, v.v1, v.v1Motivos)
	}
	validaBundle(t, firmado)
}

func verificaConJarsigner(t *testing.T, ruta string) {
	t.Helper()
	if _, err := exec.LookPath("jarsigner"); err != nil {
		t.Skipf("jarsigner no está en el PATH: %v", err)
	}
	out, err := exec.Command("jarsigner", "-verify", ruta).CombinedOutput()
	if err != nil {
		t.Fatalf("jarsigner -verify falló: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("jar verified")) {
		t.Fatalf("jarsigner no dijo 'jar verified': %s", out)
	}
}

// validaBundle pasa el oráculo externo cuando el jar está disponible; si no lo está, la
// prueba se salta con una razón explícita en vez de fingir que validate pasó.
func validaBundle(t *testing.T, ruta string) {
	t.Helper()
	jar := os.Getenv("BUNDLETOOL_JAR")
	if jar == "" {
		jar = "/home/leonisauro/opt/android-sdk/bundletool/bundletool-all-1.18.3.jar"
	}
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("bundletool no está en %s: %v", jar, err)
	}
	cmd := exec.Command("java", "-jar", jar, "validate", "--bundle="+ruta)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bundletool validate falló: %v\n%s", err, out)
	}
}
