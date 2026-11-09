package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// parNuevo genera una clave y un certificado de prueba con el nombre pedido.
func parNuevo(t *testing.T, nombre string, serial int64) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: nombre, Organization: []string{"apkc"}, Country: []string{"ES"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return key, cert
}

// leerNodoLinaje devuelve el contenido del signedData y los campos del nodo.
func leerNodoLinaje(t *testing.T, b []byte) (sd []byte, flags, sigAlg uint32, firma []byte, resto []byte) {
	t.Helper()
	leer := func(p *int) []byte {
		n := int(binary.LittleEndian.Uint32(b[*p:]))
		v := b[*p+4 : *p+4+n]
		*p += 4 + n
		return v
	}
	p := 0
	nodo := leer(&p)
	resto = b[p:]
	q := 0
	sdN := int(binary.LittleEndian.Uint32(nodo[q:]))
	sd = nodo[q+4 : q+4+sdN]
	q += 4 + sdN
	flags = binary.LittleEndian.Uint32(nodo[q:])
	sigAlg = binary.LittleEndian.Uint32(nodo[q+4:])
	q += 8
	fn := int(binary.LittleEndian.Uint32(nodo[q:]))
	firma = nodo[q+4 : q+4+fn]
	return
}

// TestElLinajeEnlazaYFirmaElNodoNuevo: el fichero que escribe crearLinaje tiene la
// magia y las versiones del §120, y la firma del nodo nuevo verifica con la clave vieja
// sobre el contenido del signedData (sin su longitud).
func TestElLinajeEnlazaYFirmaElNodoNuevo(t *testing.T) {
	vieja, certVieja := parNuevo(t, "Vieja", 0x1001)
	_, certNueva := parNuevo(t, "Nueva", 0x1002)

	blob, err := crearLinaje(certVieja, certNueva, vieja)
	if err != nil {
		t.Fatalf("el linaje no se crea: %v", err)
	}
	if got := binary.LittleEndian.Uint32(blob[:4]); got != linajeMagia {
		t.Fatalf("magia 0x%08x, se esperaba 0x%08x", got, linajeMagia)
	}
	if got := binary.LittleEndian.Uint32(blob[4:8]); got != linajeVersion {
		t.Fatalf("versión exterior %d", got)
	}
	if got, want := int(binary.LittleEndian.Uint32(blob[8:12])), len(blob)-12; got != want {
		t.Fatalf("longitud interna %d, fichero con %d", got, want)
	}

	inner, err := innerLinaje(blob)
	if err != nil {
		t.Fatalf("innerLinaje: %v", err)
	}
	if got := binary.LittleEndian.Uint32(inner[:4]); got != linajeVersion {
		t.Fatalf("versión interna %d", got)
	}
	sd0, _, _, firma0, resto := leerNodoLinaje(t, inner[4:])
	if len(firma0) != 0 {
		t.Error("el primer nodo del linaje no lleva firma")
	}
	if len(sd0) == 0 {
		t.Error("el primer nodo no trae signedData")
	}
	sd1, _, sigAlg1, firma1, _ := leerNodoLinaje(t, resto)
	if sigAlg1 != 0 {
		t.Errorf("el nodo hoja tiene sigAlg 0x%x y se esperaba 0", sigAlg1)
	}
	if len(firma1) != 256 {
		t.Fatalf("la firma del nodo nuevo mide %d bytes", len(firma1))
	}
	// el signedData es certificado con longitud + algoritmo del padre; el contenido se firma
	q := 0
	n := int(binary.LittleEndian.Uint32(sd1[q:]))
	certDicho := sd1[q+4 : q+4+n]
	q += 4 + n
	if !bytes.Equal(certDicho, certNueva.Raw) {
		t.Error("el nodo nuevo no lleva el certificado nuevo")
	}
	if alg := binary.LittleEndian.Uint32(sd1[q:]); alg != linajeSigAlgRSA {
		t.Errorf("algoritmo del padre 0x%x", alg)
	}
	// el contenido firmado es el signedData sin la longitud exterior
	contenido := sd1
	resumen := sha256.Sum256(contenido)
	if err := rsa.VerifyPKCS1v15(&vieja.PublicKey, crypto.SHA256, resumen[:], firma1); err != nil {
		t.Errorf("la firma del nodo nuevo no verifica con la clave vieja: %v", err)
	}
}

// TestLaRotacionV31EscribeV2V31V3: con minSdk 33 el bloque lleva v2, v3.1 y v3; la v1/v2
// las firma la clave vieja y el .idsig lleva dos firmantes (V4.1).
func TestLaRotacionV31EscribeV2V31V3(t *testing.T) {
	vieja, certVieja := parNuevo(t, "Vieja", 0x2001)
	nueva, certNueva := parNuevo(t, "Nueva", 0x2002)
	blob, err := crearLinaje(certVieja, certNueva, vieja)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := innerLinaje(blob)
	if err != nil {
		t.Fatal(err)
	}
	anterior := rotacionActiva
	rotacionActiva = &rotacion{nuevaLlave: nueva, nuevoCert: certNueva, linaje: inner, minSdk: 33}
	defer func() { rotacionActiva = anterior }()

	salida := filepath.Join(t.TempDir(), "rotado.apk")
	if _, err := resignWithKey(fixturePackage(t), salida, vieja, certVieja); err != nil {
		t.Fatalf("la rotación no firma: %v", err)
	}

	datos, err := os.ReadFile(salida)
	if err != nil {
		t.Fatal(err)
	}
	// los pares del bloque: v2, v3.1 y v3, en el orden de apksigner
	pos := bytes.LastIndex(datos, []byte("APK Sig Block 42"))
	if pos < 0 {
		t.Fatal("no hay bloque de firmas")
	}
	inicio := pos + 16 - int(binary.LittleEndian.Uint64(datos[pos-8:])) - 8
	o := inicio + 8
	var ids []uint32
	for o < pos-8 {
		n := int(binary.LittleEndian.Uint64(datos[o:]))
		id := binary.LittleEndian.Uint32(datos[o+8:])
		ids = append(ids, id)
		o += 8 + n
	}
	quiero := []uint32{apkSigningBlockIDv2, apkSigningBlockIDv3_1, apkSigningBlockIDv3}
	if len(ids) < 3 || ids[0] != quiero[0] || ids[1] != quiero[1] || ids[2] != quiero[2] {
		t.Fatalf("pares del bloque %x, se esperaban %x", ids, quiero)
	}

	v, err := verificarApk(salida)
	if err != nil {
		t.Fatal(err)
	}
	if !v.v1 || !v.v2 || !v.v4Presente || !v.v4 {
		t.Errorf("veredicto: v1=%v v2=%v v4=%v (%v)", v.v1, v.v2, v.v4, v.v4Motivos)
	}
}
