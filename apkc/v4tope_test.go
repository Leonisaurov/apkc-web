package main

// La v4 era la única ruta que leía ficheros enteros con os.ReadFile: el `.idsig` que va al lado
// del apk y el apk, para recalcular su árbol de verity. Un `.idsig` de gigabytes, que quien ataca
// pone junto a su paquete, agotaba la memoria del worker antes de que hubiera veredicto; un OOM de
// Go no es un pánico y el recover() de verificarApk no lo atrapa. Ahora los dos tamaños se miran
// con Stat y la lectura tiene cota.
//
// El `.idsig` enorme del test es un fichero disperso (os.Truncate): declara 1 GiB sin escribir ni
// uno, y va sin permiso de lectura para que, si el verificador lo abriera, el motivo lo delate.

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// apkMinimo arma un zip válido con una entrada: lo justo para que verificarApk lo pueda listar y
// llegue a mirar la v4. Sin entradas de firma, la v1 y la v2 quedan en mal, que aquí no importa.
func apkMinimo(t *testing.T) string {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	w, err := zw.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("<manifest/>")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(ruta, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// idsigDisperso crea el `.idsig` de `ruta` con `tamano` bytes declarados y sin escribir el
// contenido, y le quita el permiso de lectura: abrirlo es un fallo de permiso, no una lectura.
func idsigDisperso(t *testing.T, ruta string, tamano int64) {
	t.Helper()
	f, err := os.Create(ruta + ".idsig")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(tamano); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ruta+".idsig", 0o000); err != nil {
		t.Fatal(err)
	}
}

// TestVerificarV4NoLeeUnIdsigEnorme: un `.idsig` de 1 GiB al lado de un apk normal. El motivo
// tiene que ser el tope, con el nombre del fichero, y no un fallo de lectura: si el verificador
// llegara a abrirlo, el chmod lo convertiría en «permission denied» antes que en memoria.
func TestVerificarV4NoLeeUnIdsigEnorme(t *testing.T) {
	apk := apkMinimo(t)
	idsigDisperso(t, apk, 1<<30)

	v, err := verificarApk(apk)
	if err != nil {
		t.Fatalf("el paquete no se puede listar: %v", err)
	}
	if !v.v4Presente {
		t.Error("el .idsig no se anunció presente")
	}
	if v.v4 {
		t.Error("la v4 se aprobó con un .idsig que pasa del tope")
	}
	tope := false
	for _, motivo := range v.v4Motivos {
		if strings.Contains(motivo, "permission denied") || strings.Contains(motivo, "no se puede leer") {
			t.Errorf("el .idsig se abrió pese al tope: %q", motivo)
		}
		if strings.Contains(motivo, ".idsig") && strings.Contains(motivo, "tope") &&
			strings.Contains(motivo, "no se lee") {
			tope = true
		}
	}
	if !tope {
		t.Errorf("el .idsig de 1 GiB no se rechazó por el tope: %v", v.v4Motivos)
	}
}

// TestVerificarV4NoLeeUnApkPorEncimaDelTope: el tope del apk se puede bajar sin fabricar un zip
// de un GiB que además sea válido; con el tope en 16 bytes, un apk normal ya no se lee entero.
func TestVerificarV4NoLeeUnApkPorEncimaDelTope(t *testing.T) {
	anterior := topeApkParaV4
	topeApkParaV4 = 16
	defer func() { topeApkParaV4 = anterior }()

	apk := apkMinimo(t)
	// el .idsig tiene que existir: sin él la v4 es una nota y no se mira ningún tamaño
	if err := os.WriteFile(apk+".idsig", []byte{0, 0, 0, 0}, 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := verificarApk(apk)
	if err != nil {
		t.Fatalf("el paquete no se puede listar: %v", err)
	}
	info, err := os.Stat(apk)
	if err != nil {
		t.Fatal(err)
	}
	for _, motivo := range v.v4Motivos {
		if strings.Contains(motivo, "app.apk") && strings.Contains(motivo, "árbol de verity") &&
			strings.Contains(motivo, "no se lee") {
			return
		}
	}
	t.Errorf("el apk de %d bytes no se rechazó con el tope en 16: %v", info.Size(), v.v4Motivos)
}

// TestVerificarV4LeeUnIdsigNormal: el control. Un `.idsig` corriente sigue llegando al lector, que
// se queja del formato; el tope no puede convertir en «no se lee» todo lo que no sea un apk real.
func TestVerificarV4LeeUnIdsigNormal(t *testing.T) {
	apk := apkMinimo(t)
	// versión 2 y nada más: el lector echa en falta el u32 siguiente
	if err := os.WriteFile(apk+".idsig", binary.LittleEndian.AppendUint32(nil, v4Version), 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := verificarApk(apk)
	if err != nil {
		t.Fatalf("el paquete no se puede listar: %v", err)
	}
	if !v.v4Presente {
		t.Error("el .idsig no se anunció presente")
	}
	for _, motivo := range v.v4Motivos {
		if strings.Contains(motivo, "tope") {
			t.Errorf("el .idsig normal se rechazó por el tope: %q", motivo)
		}
		if strings.Contains(motivo, "termina antes de un u32") {
			return
		}
	}
	t.Errorf("el .idsig normal no llegó al lector: %v", v.v4Motivos)
}
