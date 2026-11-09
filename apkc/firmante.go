package main

// Un firmante es la clave con su certificado y lo que el formato del apk necesita saber de
// ella: el id del algoritmo para v2/v3/v4, el hash con el que se firma el signed data y el
// hash del digest del contenido. RSA usa 0x0103 y SHA-256; ECDSA P-256 usa 0x0201 y
// SHA-256; P-384/P-521 usan 0x0202 y SHA-512 (medido en el §122).
//
// La v1 no usa estos ids: su PKCS#7 va con SHA-256 y, para EC, con el OID
// ecdsa-with-SHA256, que es lo que verifica cualquier lector de JAR.

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	_ "crypto/sha512" // registra SHA-512 para crypto.Hash.New
	"crypto/x509"
	"errors"
	"fmt"
)

const (
	algRSASSAWithSHA256 = uint32(0x0103)
	algECDSAWithSHA256  = uint32(0x0201)
	algECDSAWithSHA512  = uint32(0x0202)
)

type firmante struct {
	signer      crypto.Signer
	cert        *x509.Certificate
	algID       uint32
	hash        crypto.Hash // hash del signed data (y del PKCS#7 de la v1 si es SHA-256)
	contentHash crypto.Hash // hash del digest del contenido que va en v2/v3/v4
}

// firmanteDe envuelve la clave que devolvió el keystore. Rechaza por nombre una clave que
// no sea RSA de 2048 o una curva EC medida.
func firmanteDe(signer crypto.Signer, cert *x509.Certificate) (*firmante, error) {
	switch k := signer.(type) {
	case *rsa.PrivateKey:
		if k == nil || k.N == nil || k.N.BitLen() < 2048 {
			return nil, errors.New("the key of the keystore is below 2048 bits, and an apk signature needs at least that")
		}
		return &firmante{signer: signer, cert: cert, algID: algRSASSAWithSHA256, hash: crypto.SHA256, contentHash: crypto.SHA256}, nil
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P256():
			return &firmante{signer: signer, cert: cert, algID: algECDSAWithSHA256, hash: crypto.SHA256, contentHash: crypto.SHA256}, nil
		case elliptic.P384(), elliptic.P521():
			return &firmante{signer: signer, cert: cert, algID: algECDSAWithSHA512, hash: crypto.SHA512, contentHash: crypto.SHA512}, nil
		default:
			return nil, fmt.Errorf("the Go signer measures P-256 and P-384; this keystore holds %v", k.Curve.Params().Name)
		}
	default:
		return nil, fmt.Errorf("the Go signer writes RSA or ECDSA signatures; this keystore holds a %T", signer)
	}
}

// firma firma los bytes con el hash del esquema v2/v3/v4. Para RSA es PKCS#1 v1.5; para
// ECDSA es la firma DER.
func (f *firmante) firma(datos []byte) ([]byte, error) {
	h := f.hash.New()
	h.Write(datos)
	return f.signer.Sign(rand.Reader, h.Sum(nil), f.hash)
}

// firmaV1 firma el .SF para el PKCS#7 de la v1. El hash normal es SHA-256; para los
// paquetes con minSdk < 18 es SHA-1.
func (f *firmante) firmaV1(datos []byte) ([]byte, error) {
	return f.firmaV1Con(datos, crypto.SHA256)
}

func (f *firmante) firmaV1Con(datos []byte, hash crypto.Hash) ([]byte, error) {
	h := hash.New()
	h.Write(datos)
	return f.signer.Sign(rand.Reader, h.Sum(nil), hash)
}
