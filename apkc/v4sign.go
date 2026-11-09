package main

// Firma v4 de un apk: el fichero `.idsig` que permite la instalación incremental.
//
// No va dentro del apk. Es un fichero aparte con tres cosas: el árbol de Merkle de
// fs-verity del apk (bloques de 4096 y SHA-256), el `SigningInfo` que firma el digest
// del bloque v2/v3 junto con el tamaño y el árbol, y el propio árbol serializado. El
// formato se midió sobre un `.idsig` de `apksigner --v4-signing-enabled true` y se
// reproduce aquí; el oráculo es `apksigner verify --v4-signature-file`.
//
// El `apkDigest` del SigningInfo es **el mismo digest que va dentro del bloque v3**, no
// uno recalculado sobre el fichero final: por eso `finishSignedPackage` pasa el que ya
// calculó para v2/v3.

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
)

const (
	v4Version       = 2
	v4HashSHA256    = 1
	v4Log2BlockSize = 12
	v4BlockSize     = 1 << v4Log2BlockSize
	v4SigAlgRSA     = 0x0103 // RSASSA-PKCS1-v1_5 con SHA-256, el id de apksig
)

// arbolVerity devuelve la raíz y el árbol serializado de fs-verity: hojas de 4096 bytes
// (la última rellena con ceros), cada nivel partido en bloques de 4096 y relleno hasta un
// múltiplo de 4096, y el nivel superior el primero en el fichero. El nivel superior es el
// primero cuyos hashes caben en 4096 bytes; su SHA-256 es la raíz.
func arbolVerity(apk []byte) (raiz, arbol []byte, err error) {
	if len(apk) == 0 {
		return nil, nil, fmt.Errorf("un apk vacío no tiene árbol de verity")
	}
	niveles := [][]byte{}
	datos := apk
	for {
		n := (len(datos) + v4BlockSize - 1) / v4BlockSize
		crudo := make([]byte, 0, n*sha256.Size)
		for i := 0; i < len(datos); i += v4BlockSize {
			fin := i + v4BlockSize
			if fin > len(datos) {
				fin = len(datos)
			}
			var bloque [v4BlockSize]byte
			copy(bloque[:], datos[i:fin])
			suma := sha256.Sum256(bloque[:])
			crudo = append(crudo, suma[:]...)
		}
		// cada nivel se guarda relleno hasta un múltiplo de 4096; el superior es el
		// primero que cabe en un bloque
		relleno := ((len(crudo) + v4BlockSize - 1) / v4BlockSize) * v4BlockSize
		nivel := make([]byte, relleno)
		copy(nivel, crudo)
		niveles = append([][]byte{nivel}, niveles...)
		if len(crudo) <= v4BlockSize {
			suma := sha256.Sum256(nivel)
			raiz = suma[:]
			break
		}
		datos = nivel
	}
	for _, nivel := range niveles {
		arbol = append(arbol, nivel...)
	}
	return raiz, arbol, nil
}

func ponU32(dst []byte, v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return append(dst, b[:]...)
}

func ponU64(dst []byte, v uint64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return append(dst, b[:]...)
}

// ponBytes: el formato del `.idsig` prefija cada campo variable con su longitud en u32
// little-endian.
func ponBytes(dst, b []byte) []byte {
	dst = ponU32(dst, uint32(len(b)))
	return append(dst, b...)
}

// v4Signature arma el contenido del `.idsig` para un apk ya firmado. `apkDigest` es el
// digest del bloque v3 (o v2) que el paquete lleva dentro.
func v4Signature(apk, apkDigest []byte, key *rsa.PrivateKey, cert *x509.Certificate) ([]byte, error) {
	raiz, arbol, err := arbolVerity(apk)
	if err != nil {
		return nil, err
	}
	pub, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, err
	}
	if len(apkDigest) != sha256.Size {
		return nil, fmt.Errorf("el digest del apk mide %d bytes y se esperaban %d", len(apkDigest), sha256.Size)
	}

	// HashingInfo: hashAlgorithm, log2BlockSize, sal (vacía) y raíz.
	hashing := ponU32(nil, v4HashSHA256)
	hashing = append(hashing, byte(v4Log2BlockSize))
	hashing = ponBytes(hashing, nil)
	hashing = ponBytes(hashing, raiz)

	// SigningInfo firmado: tamaño, tamaño del fichero, algoritmo y los campos con
	// longitud. El tamaño incluye el u32 del propio tamaño y el u64 del fichero.
	signed := ponU32(nil, 0) // se rellena cuando ya se sabe el tamaño
	signed = ponU64(signed, uint64(len(apk)))
	signed = ponU32(signed, v4HashSHA256)
	signed = append(signed, byte(v4Log2BlockSize))
	signed = ponBytes(signed, nil) // sal
	signed = ponBytes(signed, raiz)
	signed = ponBytes(signed, apkDigest)
	signed = ponBytes(signed, cert.Raw)
	signed = ponBytes(signed, nil) // additionalData
	binary.LittleEndian.PutUint32(signed[:4], uint32(len(signed)))

	resumen := sha256.Sum256(signed)
	firma, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, resumen[:])
	if err != nil {
		return nil, err
	}

	// SigningInfo: apkDigest, certificado, additionalData, clave pública, algoritmo y
	// firma. El bloque de firmantes alternativos (SigningInfoBlock) no se usa: un solo
	// firmante, como en el apk que se firma.
	signingInfo := ponBytes(nil, apkDigest)
	signingInfo = ponBytes(signingInfo, cert.Raw)
	signingInfo = ponBytes(signingInfo, nil)
	signingInfo = ponBytes(signingInfo, pub)
	signingInfo = ponU32(signingInfo, v4SigAlgRSA)
	signingInfo = ponBytes(signingInfo, firma)

	// V4Signature: versión, hashingInfo y signingInfos, cada uno con su longitud; y al
	// final el árbol de verity, también con la suya.
	out := ponU32(nil, v4Version)
	out = ponBytes(out, hashing)
	out = ponBytes(out, signingInfo)
	out = ponBytes(out, arbol)
	return out, nil
}
