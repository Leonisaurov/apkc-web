package main

import (
	"archive/zip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Estas pruebas sujetan la verificación del proof-of-rotation que se enganchó en
// verificarUnBloqueV3. El oráculo es `apksigner rotate` + `apksigner sign --lineage`,
// aprobado por `apksigner verify -v`; si la herramienta no está, se salta.
// La cadena nodo a nodo y la pertenencia de la hoja se prueban además sin oráculo,
// para poder romperlas a propósito sin que apksigner se niegue a firmar.

type parOraculo struct {
	clave   *rsa.PrivateKey
	cert    *x509.Certificate
	keyDER  []byte
	certPEM []byte
}

func parParaOraculo(t *testing.T, nombre string, serial int64) parOraculo {
	t.Helper()
	clave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: nombre, Organization: []string{"apkc"}, Country: []string{"ES"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(clave)
	if err != nil {
		t.Fatal(err)
	}
	return parOraculo{
		clave:   clave,
		cert:    cert,
		keyDER:  keyDER,
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}
}

func escribeParOraculo(t *testing.T, dir, prefijo string, p parOraculo) (string, string) {
	t.Helper()
	keyPath := filepath.Join(dir, prefijo+".pk8")
	certPath := filepath.Join(dir, prefijo+".pem")
	if err := os.WriteFile(keyPath, p.keyDER, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, p.certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	return keyPath, certPath
}

// apkParaApksigner es un zip con una entrada llamada AndroidManifest.xml. No hace falta
// que sea un AXML de verdad: con --min-sdk-version apksigner firma y verifica igual.
func apkParaApksigner(t *testing.T, dir string) string {
	t.Helper()
	ruta := filepath.Join(dir, "unsigned.apk")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	entrada, err := w.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entrada.Write([]byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.rot"/>`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func correApksigner(t *testing.T, args ...string) string {
	t.Helper()
	salida, err := exec.Command("apksigner", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("apksigner %v falló: %v\n%s", args, err, salida)
	}
	return string(salida)
}

func oraculoDisponible(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("apksigner"); err != nil {
		t.Skip("no hay apksigner en el PATH: no se puede contrastar la rotación")
	}
}

// TestVerificarRotacionV31ConOraculoApksigner: un apk rotado por apksigner con
// --rotation-min-sdk-version 33 debe aprobarse en v3 y v3.1 sin motivos de linaje.
func TestVerificarRotacionV31ConOraculoApksigner(t *testing.T) {
	oraculoDisponible(t)
	dir := t.TempDir()
	vieja := parParaOraculo(t, "Vieja", 0x3001)
	nueva := parParaOraculo(t, "Nueva", 0x3002)
	oldKey, oldCert := escribeParOraculo(t, dir, "old", vieja)
	newKey, newCert := escribeParOraculo(t, dir, "new", nueva)

	base := apkParaApksigner(t, dir)
	oldAPK := filepath.Join(dir, "old.apk")
	correApksigner(t, "sign", "--key", oldKey, "--cert", oldCert,
		"--min-sdk-version", "28", "--out", oldAPK, base)

	linaje := filepath.Join(dir, "lineage.bin")
	correApksigner(t, "rotate", "--out", linaje,
		"--old-signer", "--key", oldKey, "--cert", oldCert,
		"--new-signer", "--key", newKey, "--cert", newCert)

	rotado := filepath.Join(dir, "rotated.apk")
	correApksigner(t, "sign", "--key", oldKey, "--cert", oldCert,
		"--next-signer", "--key", newKey, "--cert", newCert,
		"--lineage", linaje, "--rotation-min-sdk-version", "33",
		"--min-sdk-version", "28", "--out", rotado, oldAPK)

	aprobado := correApksigner(t, "verify", "-v", "--min-sdk-version", "28", rotado)
	if !strings.Contains(aprobado, "v3.1 scheme (APK Signature Scheme v3.1): true") {
		t.Fatalf("el oráculo no aprobó v3.1:\n%s", aprobado)
	}

	v, err := verificarApk(rotado)
	if err != nil {
		t.Fatal(err)
	}
	if !v.v3 || !v.v31 || len(v.v3Motivos) > 0 || len(v.v31Motivos) > 0 {
		t.Errorf("rotación v3.1: v3=%v %v, v3.1=%v %v", v.v3, v.v3Motivos, v.v31, v.v31Motivos)
	}
	if len(v.rotacionBloques) != 2 {
		t.Fatalf("se leyeron %d bloques de rotación, se esperaban 2", len(v.rotacionBloques))
	}
	var conPrueba, conMin int
	for _, b := range v.rotacionBloques {
		if b.tienePrueba {
			conPrueba++
			if !b.es31 {
				t.Error("la prueba del linaje apareció en el bloque v3.0")
			}
		}
		if b.tieneMinSdkAttr {
			conMin++
			if b.minSdkAttr != 33 {
				t.Errorf("rotation-min-sdk-version leído %d, esperado 33", b.minSdkAttr)
			}
		}
	}
	if conPrueba != 1 || conMin != 1 {
		t.Errorf("prueba del linaje en %d bloques y rotation-min-sdk en %d; se esperaba uno de cada",
			conPrueba, conMin)
	}
}

// TestLeerLinajeV3FirmaRota: la cadena escrita por crearLinaje se lee y verifica, y deja
// de verificar en cuanto se cambia un byte de la firma del nodo nuevo.
func TestLeerLinajeV3FirmaRota(t *testing.T) {
	vieja, certVieja := parNuevo(t, "Vieja", 0x4001)
	_, certNueva := parNuevo(t, "Nueva", 0x4002)
	viejaF, err := firmanteDe(vieja, certVieja)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := crearLinaje(certVieja, certNueva, viejaF)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := innerLinaje(blob)
	if err != nil {
		t.Fatal(err)
	}
	nodos, err := leerLinajeV3(inner)
	if err != nil {
		t.Fatalf("leerLinajeV3: %v", err)
	}
	if len(nodos) != 2 {
		t.Fatalf("el linaje trae %d nodos, se esperaban 2", len(nodos))
	}
	if err := verificarCadenaLinaje(nodos); err != nil {
		t.Errorf("la cadena del linaje no verifica: %v", err)
	}
	if !laHojaEsElFirmante(nodos, certNueva.Raw) {
		t.Error("la hoja del linaje no es el certificado nuevo")
	}
	if laHojaEsElFirmante(nodos, certVieja.Raw) {
		t.Error("el certificado viejo se tomó por hoja del linaje")
	}

	// romper la firma del nodo nuevo sin tocar su longitud
	ultimo := len(blob) - 1
	roto := append([]byte(nil), blob...)
	roto[ultimo] ^= 0x01
	innerRoto, err := innerLinaje(roto)
	if err != nil {
		t.Fatal(err)
	}
	nodosRotos, err := leerLinajeV3(innerRoto)
	if err != nil {
		t.Fatal(err)
	}
	if err := verificarCadenaLinaje(nodosRotos); err == nil {
		t.Error("la cadena verificó con la firma del nodo nuevo rota")
	}
}

// TestCoherenciaRotationMinSdk: el v3.0 tiene que terminar justo antes del SDK en el
// que empieza el v3.1, y la prueba del linaje no puede convivir con el atributo de
// rotation-min-sdk-version en el mismo bloque.
func TestCoherenciaRotationMinSdk(t *testing.T) {
	casos := []struct {
		nombre   string
		bloques  []rotacionBloque
		aceptaV3 bool
	}{
		{
			nombre: "coherente",
			bloques: []rotacionBloque{
				{minSdk: 24, maxSdk: 32, tieneMinSdkAttr: true, minSdkAttr: 33},
				{es31: true, minSdk: 33, maxSdk: 0x7fffffff, tienePrueba: true},
			},
			aceptaV3: true,
		},
		{
			nombre: "el rango del v3.0 no llega al rotation-min-sdk",
			bloques: []rotacionBloque{
				{minSdk: 24, maxSdk: 30, tieneMinSdkAttr: true, minSdkAttr: 33},
				{es31: true, minSdk: 33, maxSdk: 0x7fffffff, tienePrueba: true},
			},
		},
		{
			nombre: "prueba y rotation-min-sdk en el mismo bloque",
			bloques: []rotacionBloque{
				{es31: true, minSdk: 33, maxSdk: 0x7fffffff, tienePrueba: true,
					tieneMinSdkAttr: true, minSdkAttr: 33},
			},
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			v := &verificacion{v3: true, v31: true, rotacionBloques: caso.bloques}
			v.verificarCoherenciaRotacion()
			if caso.aceptaV3 {
				if !v.v3 || !v.v31 || len(v.v3Motivos) > 0 || len(v.v31Motivos) > 0 {
					t.Errorf("caso coherente rechazado: v3=%v %v, v3.1=%v %v",
						v.v3, v.v3Motivos, v.v31, v.v31Motivos)
				}
			} else if v.v3 && v.v31 {
				t.Error("el caso incoherente se aprobó sin motivos")
			}
		})
	}
}
