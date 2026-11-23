package main

// Las colas de seguridad del empaquetado: el temporal del paquete no puede tener un nombre
// predecible, tiene que borrarse cuando el paquete no llega a escribirse, y la contraseña del
// keystore no puede viajar en la línea de órdenes de apksigner ni en la del hijo del vigilante.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTemporalDelPaqueteNoTieneNombrePredecible: dos aperturas seguidas no comparten nombre y
// ninguna es el `ruta + ".tmp"` que un atacante podría plantar antes de la escritura.
func TestTemporalDelPaqueteNoTieneNombrePredecible(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "app.apk")

	primero, rutaPrimero, err := abrirTemporalDePaquete(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(rutaPrimero)
	defer primero.Close()
	segundo, rutaSegundo, err := abrirTemporalDePaquete(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(rutaSegundo)
	defer segundo.Close()

	if rutaPrimero == rutaSegundo {
		t.Errorf("dos temporales compartieron el nombre %s", rutaPrimero)
	}
	for _, r := range []string{rutaPrimero, rutaSegundo} {
		if r == ruta+".tmp" {
			t.Errorf("el temporal conserva el nombre predecible %s", r)
		}
		if filepath.Dir(r) != dir {
			t.Errorf("el temporal %s no está junto al destino: el rename no sería atómico", r)
		}
	}
}

// TestEmpaquetarNoSigueUnEnlaceEnElTemporal: con el nombre predecible, un enlace simbólico plantado
// en `ruta + ".tmp"` hacía que el paquete se escribiera sobre la víctima. Con CreateTemp el enlace
// se queda donde está y la salida es un fichero nuevo.
func TestEmpaquetarNoSigueUnEnlaceEnElTemporal(t *testing.T) {
	keyPath := debugKeystore(t)
	dir := t.TempDir()
	ruta := filepath.Join(dir, "app.apk")
	victima := filepath.Join(dir, "victima.txt")
	const original = "no me toques"
	if err := os.WriteFile(victima, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victima, ruta+".tmp"); err != nil {
		t.Skipf("no se puede plantar el enlace: %v", err)
	}

	entradas := []entradaDePaquete{{nombre: "AndroidManifest.xml", contenido: []byte("manifiesto")}}
	if err := empaquetaSinAapt2(ruta, entradas, keyPath, "android", "androiddebugkey"); err != nil {
		t.Fatalf("el paquete no salió: %v", err)
	}

	datos, err := os.ReadFile(victima)
	if err != nil {
		t.Fatal(err)
	}
	if string(datos) != original {
		t.Errorf("el enlace del temporal se siguió y la víctima cambió a %q", datos)
	}
	info, err := os.Lstat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("la salida quedó como enlace simbólico en vez de fichero propio")
	}
}

// TestEmpaquetarBorraElTemporalSiFalla: un keystore que no está aborta después de crear el
// temporal; no puede quedar ni el temporal nuevo ni el viejo `ruta + ".tmp"`.
func TestEmpaquetarBorraElTemporalSiFalla(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "app.apk")
	entradas := []entradaDePaquete{{nombre: "AndroidManifest.xml", contenido: []byte("manifiesto")}}
	err := empaquetaSinAapt2(ruta, entradas, filepath.Join(dir, "no-existe.p12"), "android", "androiddebugkey")
	if err == nil {
		t.Fatal("un keystore que no está firmó el paquete")
	}
	if _, err := os.Stat(ruta + ".tmp"); !os.IsNotExist(err) {
		t.Error("quedó el temporal predecible después del fallo")
	}
	sobrantes, err := filepath.Glob(filepath.Join(dir, ".apkc-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sobrantes) != 0 {
		t.Errorf("quedaron temporales después del fallo: %v", sobrantes)
	}
}

// TestSignNoSigueUnEnlaceEnElTemporal: el mismo enlace plantado en la firma de un paquete que ya
// existe no puede desviar los bytes; y la salida tiene que ser un fichero, no el enlace renombrado.
func TestSignNoSigueUnEnlaceEnElTemporal(t *testing.T) {
	in := fixturePackage(t)
	dir := filepath.Dir(in)
	out := filepath.Join(dir, "signed.apk")
	victima := filepath.Join(dir, "victima.txt")
	const original = "no me toques"
	if err := os.WriteFile(victima, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victima, out+".tmp"); err != nil {
		t.Skipf("no se puede plantar el enlace: %v", err)
	}
	key, cert := signingPair(t)

	if _, err := resignWithKey(in, out, key, cert); err != nil {
		t.Fatalf("la firma no salió: %v", err)
	}
	datos, err := os.ReadFile(victima)
	if err != nil {
		t.Fatal(err)
	}
	if string(datos) != original {
		t.Errorf("el enlace del temporal se siguió y la victima cambió a %q", datos)
	}
	info, err := os.Lstat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("la salida quedó como enlace simbólico en vez de fichero propio")
	}
}

// TestSignBorraElTemporalSiFalla: si el rename final falla porque el destino es un directorio, el
// temporal no puede quedarse en disco.
func TestSignBorraElTemporalSiFalla(t *testing.T) {
	in := fixturePackage(t)
	out := t.TempDir()
	key, cert := signingPair(t)

	if _, err := resignWithKey(in, out, key, cert); err == nil {
		t.Fatal("se firmó sobre un destino que es un directorio")
	}
	sobrantes, err := filepath.Glob(filepath.Join(filepath.Dir(out), ".apkc-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sobrantes) != 0 {
		t.Errorf("quedaron temporales después del fallo: %v", sobrantes)
	}
}

// TestResuelveLaPasswordDeUnFichero: la forma file: que usa apksigner se lee y se recorta el salto
// de línea del fichero; un fichero que no está es un error, no una contraseña vacía.
func TestResuelveLaPasswordDeUnFichero(t *testing.T) {
	if got, err := resolverPassword("literal"); err != nil || got != "literal" {
		t.Errorf("la contraseña literal no se devolvió tal cual: %q, %v", got, err)
	}

	dir := t.TempDir()
	fichero := filepath.Join(dir, "pass.txt")
	if err := os.WriteFile(fichero, []byte("secreta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := resolverPassword("file:" + fichero); err != nil || got != "secreta" {
		t.Errorf("file: no leyó la contraseña: %q, %v", got, err)
	}
	if _, err := resolverPassword("file:" + filepath.Join(dir, "no-existe")); err == nil {
		t.Error("un fichero de contraseña que no está se aceptó")
	}
}

// TestArgumentosDeFirmaNoLlevanLaPassword: apksigner recibe --ks-pass file:<ruta>, el fichero es de
// solo el dueño y ningún argumento contiene la contraseña.
func TestArgumentosDeFirmaNoLlevanLaPassword(t *testing.T) {
	const password = "una-contraseña-larga"
	args, borrar, err := argumentosDeFirma("debug.p12", password, "androiddebugkey")
	if err != nil {
		t.Fatal(err)
	}

	var ruta string
	for _, a := range args {
		if strings.Contains(a, password) {
			t.Errorf("la contraseña viajó en el argumento %q", a)
		}
		if strings.HasPrefix(a, "pass:") {
			t.Errorf("apksigner sigue recibiendo la contraseña con pass: en %q", a)
		}
		if strings.HasPrefix(a, "file:") {
			ruta = strings.TrimPrefix(a, "file:")
		}
	}
	if ruta == "" {
		t.Fatal("apksigner no recibió --ks-pass file:")
	}
	info, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if permiso := info.Mode().Perm(); permiso&0o077 != 0 {
		t.Errorf("el fichero de la contraseña es legible por otros: %o", permiso)
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if string(datos) != password {
		t.Errorf("el fichero de la contraseña guardó %q", datos)
	}

	borrar()
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Error("el fichero de la contraseña sobrevivió al borrado")
	}
}

// TestSigningKeyAceptaLaPasswordEnFichero: el hijo del vigilante recibe --storepass file:<ruta>, y
// el lector del keystore tiene que abrirlo igual que con la contraseña literal.
func TestSigningKeyAceptaLaPasswordEnFichero(t *testing.T) {
	path := debugKeystore(t)
	fichero := filepath.Join(t.TempDir(), "pass.txt")
	if err := os.WriteFile(fichero, []byte("android\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := signingKey(path, "file:"+fichero, "androiddebugkey"); err != nil {
		t.Fatalf("el keystore no se abrió con file:: %v", err)
	}
}
