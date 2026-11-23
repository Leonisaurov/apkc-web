package main

// Las cuatro rutas que todavía escribían su salida en `ruta + ".tmp"` dejan ahora el temporal
// con `abrirTemporalDePaquete`. En un directorio compartido, el nombre predecible permitía
// plantar un enlace simbólico antes de la escritura y que los bytes del paquete acabaran donde
// apuntara el enlace. Cada prueba planta el enlace en la ruta exacta de su escritor y exige que
// la víctima quede intacta y que la salida sea un fichero propio.

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

const contenidoVictima = "no me toques"

func plantaEnlaceEnElTemporal(t *testing.T, destino, victima string) {
	t.Helper()
	if err := os.WriteFile(victima, []byte(contenidoVictima), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victima, destino+".tmp"); err != nil {
		t.Skipf("no se puede plantar el enlace en %s: %v", destino+".tmp", err)
	}
}

func exigeVictimaIntacta(t *testing.T, victima string) {
	t.Helper()
	datos, err := os.ReadFile(victima)
	if err != nil {
		t.Fatal(err)
	}
	if string(datos) != contenidoVictima {
		t.Errorf("el enlace del temporal se siguió y la víctima cambió a %q", datos)
	}
}

func exigeFicheroPropio(t *testing.T, ruta string) {
	t.Helper()
	info, err := os.Lstat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("%s quedó como enlace simbólico en vez de fichero propio", ruta)
	}
}

func TestEnsamblarAABNoSigueEnlaceEnElTemporal(t *testing.T) {
	dir := t.TempDir()
	entrada := filepath.Join(dir, "bundle.zip")
	f, err := os.Create(entrada)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("manifest/AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("manifiesto")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	salida := filepath.Join(dir, "app.aab")
	victima := filepath.Join(dir, "victima.txt")
	plantaEnlaceEnElTemporal(t, salida, victima)

	if err := ensamblarAAB(entrada, salida); err != nil {
		t.Fatalf("el .aab no se ensambló: %v", err)
	}
	exigeVictimaIntacta(t, victima)
	exigeFicheroPropio(t, salida)
}

func TestEnsamblarAABDesdeGoNoSigueEnlaceEnElTemporal(t *testing.T) {
	build := preparaEnlaceGo(t)
	aab := filepath.Join(filepath.Dir(build), "app.aab")
	victima := filepath.Join(filepath.Dir(build), "victima.txt")
	plantaEnlaceEnElTemporal(t, aab, victima)

	if err := ensamblarAABDesdeGo(build, dexEntries(build, nil), aab); err != nil {
		t.Fatalf("el .aab no se ensambló desde el enlazador Go: %v", err)
	}
	exigeVictimaIntacta(t, victima)
	exigeFicheroPropio(t, aab)
}

func TestPackageWithGoNoSigueEnlaceEnElTemporal(t *testing.T) {
	DataTable = Data{}
	t.Cleanup(func() { DataTable = Data{} })
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("build", "AndroidManifest.xml"), []byte("manifiesto"), 0o644); err != nil {
		t.Fatal(err)
	}
	salida := filepath.Join("build", "app.apk")
	victima := filepath.Join(dir, "victima.txt")
	plantaEnlaceEnElTemporal(t, salida, victima)

	plan := []bundleEntry{{
		src:      filepath.Join("build", "AndroidManifest.xml"),
		name:     "AndroidManifest.xml",
		compress: true,
	}}
	if _, err := packageWithGo(plan, nil, debugKeystore(t), "android", "androiddebugkey"); err != nil {
		t.Fatalf("el paquete Go no salió: %v", err)
	}
	exigeVictimaIntacta(t, victima)
	exigeFicheroPropio(t, salida)
}

func TestBuildBundleNoSigueEnlaceEnElTemporal(t *testing.T) {
	DataTable = Data{}
	t.Cleanup(func() { DataTable = Data{} })
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("build", "AndroidManifest.xml"), []byte("manifiesto"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("build", "resources.arsc"), []byte("tabla"), 0o644); err != nil {
		t.Fatal(err)
	}
	salida := filepath.Join("build", "bundle.zip")
	victima := filepath.Join(dir, "victima.txt")
	plantaEnlaceEnElTemporal(t, salida, victima)

	buildBundle(false, nil, nil, nativeLibs{})

	exigeVictimaIntacta(t, victima)
	exigeFicheroPropio(t, salida)
}
