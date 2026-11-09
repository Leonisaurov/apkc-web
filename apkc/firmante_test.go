package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"path/filepath"
	"testing"
	"time"
)

func parEC(t *testing.T, nombre string, curva elliptic.Curve, serial int64) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(curva, rand.Reader)
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

// TestElFirmanteEligeElAlgoritmo: RSA y P-256 van con 0x0103/0x0201 y SHA-256; P-384 con
// 0x0202 y SHA-512 (§122).
func TestElFirmanteEligeElAlgoritmo(t *testing.T) {
	rsaKey, rsaCert := signingPair(t)
	key256, cert256 := parEC(t, "P256", elliptic.P256(), 0x3001)
	key384, cert384 := parEC(t, "P384", elliptic.P384(), 0x3002)
	casos := []struct {
		nombre string
		signer crypto.Signer
		cert   *x509.Certificate
		alg    uint32
		bytes  int
	}{
		{"rsa", rsaKey, rsaCert, algRSASSAWithSHA256, 32},
		{"p256", key256, cert256, algECDSAWithSHA256, 32},
		{"p384", key384, cert384, algECDSAWithSHA512, 64},
	}
	for _, c := range casos {
		f, err := firmanteDe(c.signer, c.cert)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if f.algID != c.alg {
			t.Errorf("%s: alg 0x%04x, se esperaba 0x%04x", c.nombre, f.algID, c.alg)
		}
		if f.contentHash.Size() != c.bytes {
			t.Errorf("%s: digest de %d bytes, se esperaban %d", c.nombre, f.contentHash.Size(), c.bytes)
		}
	}
}

// TestECFirmaYVerifica: una firma completa con P-256 y con P-384 se relee con el
// verificador propio: v1 (PKCS#7 ECDSA), v2 (digest y firma ECDSA) y v4 (dos/tres).
func TestECFirmaYVerifica(t *testing.T) {
	for _, c := range []struct {
		nombre string
		curva  elliptic.Curve
	}{
		{"p256", elliptic.P256()},
		{"p384", elliptic.P384()},
	} {
		key, cert := parEC(t, c.nombre, c.curva, 0x4001)
		f, err := firmanteDe(key, cert)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		salida := filepath.Join(t.TempDir(), c.nombre+".apk")
		if _, err := resignWithFirmante(fixturePackage(t), salida, f); err != nil {
			t.Fatalf("%s no firma: %v", c.nombre, err)
		}
		v, err := verificarApk(salida)
		if err != nil {
			t.Fatalf("%s no se verifica: %v", c.nombre, err)
		}
		if !v.v1 || !v.v2 || !v.v4Presente || !v.v4 {
			t.Errorf("%s: v1=%v v2=%v v4=%v (%v / %v)", c.nombre,
				v.v1, v.v2, v.v4, v.v1Motivos, v.v4Motivos)
		}
	}
}
