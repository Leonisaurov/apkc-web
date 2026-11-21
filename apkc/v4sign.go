package main

// Firma v4 de un apk: el fichero `.idsig` que permite la instalación incremental.
//
// No va dentro del apk. Es un fichero aparte con tres cosas: el árbol de Merkle de
// fs-verity del apk (bloques de 4096 y SHA-256), los `SigningInfo` que firman el digest
// del bloque v2/v3 junto con el tamaño y el árbol, y el propio árbol serializado. El
// formato se midió sobre un `.idsig` de `apksigner --v4-signing-enabled true` y se
// reproduce aquí; el oráculo es `apksigner verify --v4-signature-file`.
//
// Con rotación v3.1 el `.idsig` lleva **dos** firmantes: el principal (clave vieja) y un
// `SigningInfoBlock` con el id del bloque v3.1 (0x1b93ad61) y la clave nueva. Es lo que
// `apksigner` llama V4.1 y exige cuando el apk trae v3.1.
//
// El `apkDigest` del SigningInfo es **el mismo digest que va dentro del bloque v3**, no
// uno recalculado sobre el fichero final: por eso `finishSignedPackage` pasa el que ya
// calculó para v2/v3.

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
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
func arbolVerity(apk, sal []byte) (raiz, arbol []byte, err error) {
	if len(apk) == 0 {
		return nil, nil, fmt.Errorf("un apk vacío no tiene árbol de verity")
	}
	// fs-verity prefija la sal a CADA bloque de CADA nivel; con sal vacía esto es sha256 del bloque,
	// que es lo medido para el `.idsig` por defecto de apksigner.
	hashBloque := func(bloque []byte) []byte {
		h := sha256.New()
		if len(sal) > 0 {
			h.Write(sal)
		}
		h.Write(bloque)
		return h.Sum(nil)
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
			crudo = append(crudo, hashBloque(bloque[:])...)
		}
		// cada nivel se guarda relleno hasta un múltiplo de 4096; el superior es el
		// primero que cabe en un bloque
		relleno := ((len(crudo) + v4BlockSize - 1) / v4BlockSize) * v4BlockSize
		nivel := make([]byte, relleno)
		copy(nivel, crudo)
		niveles = append([][]byte{nivel}, niveles...)
		if len(crudo) <= v4BlockSize {
			raiz = hashBloque(nivel)
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

// hashingInfoV4: hashAlgorithm, log2BlockSize, sal (vacía) y raíz.
func hashingInfoV4(raiz, sal []byte) []byte {
	out := ponU32(nil, v4HashSHA256)
	out = append(out, byte(v4Log2BlockSize))
	out = ponBytes(out, sal)
	return ponBytes(out, raiz)
}

// signingInfoV4 arma y firma un SigningInfo para un firmante. La carga firmada incluye
// el tamaño del apk, el algoritmo del árbol (SHA-256), la raíz y el digest del bloque
// v2/v3, además del certificado. La firma usa el hash del firmante (SHA-256 o SHA-512).
func signingInfoV4(apk, apkDigest []byte, raiz, sal []byte, f *firmante) ([]byte, error) {
	pub, err := x509.MarshalPKIXPublicKey(f.cert.PublicKey)
	if err != nil {
		return nil, err
	}
	if len(apkDigest) != f.contentHash.Size() {
		return nil, fmt.Errorf("el digest del apk mide %d bytes y se esperaban %d", len(apkDigest), f.contentHash.Size())
	}

	signed := ponU32(nil, 0) // se rellena cuando ya se sabe el tamaño
	signed = ponU64(signed, uint64(len(apk)))
	signed = ponU32(signed, v4HashSHA256)
	signed = append(signed, byte(v4Log2BlockSize))
	signed = ponBytes(signed, sal)
	signed = ponBytes(signed, raiz)
	signed = ponBytes(signed, apkDigest)
	signed = ponBytes(signed, f.cert.Raw)
	signed = ponBytes(signed, nil) // additionalData
	binary.LittleEndian.PutUint32(signed[:4], uint32(len(signed)))

	firma, err := f.firma(signed)
	if err != nil {
		return nil, err
	}

	// SigningInfo: apkDigest, certificado, additionalData, clave pública, algoritmo y firma.
	signingInfo := ponBytes(nil, apkDigest)
	signingInfo = ponBytes(signingInfo, f.cert.Raw)
	signingInfo = ponBytes(signingInfo, nil)
	signingInfo = ponBytes(signingInfo, pub)
	signingInfo = ponU32(signingInfo, f.algID)
	return ponBytes(signingInfo, firma), nil
}

// v4Bytes: versión, hashingInfo y signingInfos, cada uno con su longitud; y al final el
// árbol de verity, también con la suya.
func v4Bytes(hashing, signingInfos, arbol []byte) []byte {
	out := ponU32(nil, v4Version)
	out = ponBytes(out, hashing)
	out = ponBytes(out, signingInfos)
	return ponBytes(out, arbol)
}

// v4Signature: el `.idsig` de un apk sin rotación, con un solo firmante.
func salV4() ([]byte, error) {
	sal, err := hex.DecodeString(v4SaltHex)
	if err != nil {
		return nil, fmt.Errorf("--v4-salt no es hexadecimal: %w", err)
	}
	return sal, nil
}

func v4Signature(apk, apkDigest []byte, f *firmante) ([]byte, error) {
	sal, err := salV4()
	if err != nil {
		return nil, err
	}
	raiz, arbol, err := arbolVerity(apk, sal)
	if err != nil {
		return nil, err
	}
	si, err := signingInfoV4(apk, apkDigest, raiz, sal, f)
	if err != nil {
		return nil, err
	}
	return v4Bytes(hashingInfoV4(raiz, sal), si, arbol), nil
}

// v4SignatureRotada: el `.idsig` de una rotación v3.1. Lleva el firmante viejo como
// principal y uno nuevo en un SigningInfoBlock con el id del bloque v3.1, que es lo que
// `apksigner` llama V4.1.
func v4SignatureRotada(apk, apkDigest []byte, oldF *firmante,
	newF *firmante) ([]byte, error) {
	sal, err := salV4()
	if err != nil {
		return nil, err
	}
	raiz, arbol, err := arbolVerity(apk, sal)
	if err != nil {
		return nil, err
	}
	viejo, err := signingInfoV4(apk, apkDigest, raiz, sal, oldF)
	if err != nil {
		return nil, err
	}
	nuevo, err := signingInfoV4(apk, apkDigest, raiz, sal, newF)
	if err != nil {
		return nil, err
	}
	signingInfos := append([]byte{}, viejo...)
	signingInfos = ponU32(signingInfos, apkSigningBlockIDv3_1)
	signingInfos = ponBytes(signingInfos, nuevo)
	return v4Bytes(hashingInfoV4(raiz, sal), signingInfos, arbol), nil
}
