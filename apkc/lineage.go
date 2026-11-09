package main

// El linaje de rotación de claves (SigningCertificateLineage) y su escritura.
//
// Un linaje es una cadena de certificados donde cada nodo lo firma el anterior; el
// atributo `proof-of-rotation` de un firmante v3 lleva el linaje **interno** tal cual
// (sin la magia exterior). El formato se midió en el §120: magia 0x3eff39d1, versión 1,
// longitud del interno; dentro, versión 1 y cada nodo con su longitud. Un nodo es
// `signedData` (certificado con longitud y el algoritmo del padre) + `flags` +
// `sigAlgorithm` + `signature`, y el `signedData` va envuelto en su longitud.
//
// Las capacidades por defecto de los dos nodos son 0x17, las que escribe `apksigner`
// cuando no se piden capacidades.

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
	linajeMagia          = uint32(0x3eff39d1)
	linajeVersion        = uint32(1)
	linajeCapacidades    = uint32(0x17)
	linajeSigAlgRSA      = uint32(0x0103)
	proofOfRotationAttr  = uint32(0x3ba06f8c)
	rotationMinSdkAttr   = uint32(0x559f8b02)
	rotacionMinSdkPorDef = 33
)

// signedDataLinaje devuelve el bloque firmado de un nodo: el certificado con su
// longitud y el algoritmo con el que lo firmó el padre, todo con la longitud delante.
func signedDataLinaje(cert *x509.Certificate, parentAlg uint32) []byte {
	inner := ponBytes(nil, cert.Raw)
	inner = ponU32(inner, parentAlg)
	return ponBytes(nil, inner)
}

// nodoLinaje arma un nodo con su longitud de lista. `firma` se calcula sobre el
// **contenido** del signedData (sin la longitud que lo envuelve), que es lo que relee
// `apksigner`.
func nodoLinaje(cert *x509.Certificate, parentAlg, sigAlg uint32, firma []byte) []byte {
	nodo := signedDataLinaje(cert, parentAlg)
	nodo = ponU32(nodo, linajeCapacidades)
	nodo = ponU32(nodo, sigAlg)
	nodo = ponBytes(nodo, firma)
	return ponBytes(nil, nodo)
}

// crearLinaje enlaza el certificado viejo con el nuevo: el nodo del nuevo lleva la
// firma de la clave vieja sobre su signedData.
func crearLinaje(oldCert, newCert *x509.Certificate, oldKey *rsa.PrivateKey) ([]byte, error) {
	// lo que se firma es el contenido del signedData del nodo nuevo
	innerNuevo := ponBytes(nil, newCert.Raw)
	innerNuevo = ponU32(innerNuevo, linajeSigAlgRSA)
	resumen := sha256.Sum256(innerNuevo)
	firma, err := rsa.SignPKCS1v15(rand.Reader, oldKey, crypto.SHA256, resumen[:])
	if err != nil {
		return nil, fmt.Errorf("la clave vieja no firma el linaje: %w", err)
	}

	variable := ponU32(nil, linajeVersion)
	variable = append(variable, nodoLinaje(oldCert, 0, linajeSigAlgRSA, nil)...)
	variable = append(variable, nodoLinaje(newCert, linajeSigAlgRSA, 0, firma)...)

	out := ponU32(nil, linajeMagia)
	out = ponU32(out, linajeVersion)
	out = ponBytes(out, variable)
	return out, nil
}

// innerLinaje devuelve el linaje interno (sin la magia exterior) que va como valor del
// atributo proof-of-rotation. El fichero que escribió `apkc rotate` o `apksigner rotate`
// es magia, versión, longitud e interno.
func innerLinaje(blob []byte) ([]byte, error) {
	if len(blob) < 12 {
		return nil, fmt.Errorf("el linaje mide %d bytes y no llega a la cabecera", len(blob))
	}
	if binary.LittleEndian.Uint32(blob[:4]) != linajeMagia {
		return nil, fmt.Errorf("el linaje no empieza por la magia 0x%08x", linajeMagia)
	}
	if binary.LittleEndian.Uint32(blob[4:8]) != linajeVersion {
		return nil, fmt.Errorf("el linaje es de versión %d y solo se mide la %d",
			binary.LittleEndian.Uint32(blob[4:8]), linajeVersion)
	}
	n := int(binary.LittleEndian.Uint32(blob[8:12]))
	if 12+n != len(blob) {
		return nil, fmt.Errorf("el linaje declara %d bytes internos y el fichero trae %d", n, len(blob)-12)
	}
	return blob[12:], nil
}

// atributosRotacion arma la lista de atributos del firmante v3 con el proof-of-rotation:
// una lista con longitud, un par con su longitud, el id y el linaje interno. El par mide
// el id y el valor, y la lista mide el par entero (medido en el §120).
func atributosRotacion(inner []byte) []byte {
	par := ponU32(nil, uint32(4+len(inner)))
	par = ponU32(par, proofOfRotationAttr)
	par = append(par, inner...)
	out := ponU32(nil, uint32(len(par)))
	return append(out, par...)
}

// atributosRotacionMin: el atributo que dice desde qué SDK empieza a usarse la clave
// nueva; va en el firmante v3.0 (la vieja) y en el v3.1.
func atributosRotacionMin(minSdk int) []byte {
	par := ponU32(nil, 8)
	par = ponU32(par, rotationMinSdkAttr)
	par = ponU32(par, uint32(minSdk))
	out := ponU32(nil, uint32(len(par)))
	return append(out, par...)
}

// atributosRotacionCompleta: los dos atributos juntos, en el orden que escribe apksigner:
// proof-of-rotation y después rotation-min-sdk-version.
func atributosRotacionCompleta(inner []byte, minSdk int) []byte {
	p1 := ponU32(nil, uint32(4+len(inner)))
	p1 = ponU32(p1, proofOfRotationAttr)
	p1 = append(p1, inner...)
	p2 := ponU32(nil, 8)
	p2 = ponU32(p2, rotationMinSdkAttr)
	p2 = ponU32(p2, uint32(minSdk))
	out := ponU32(nil, uint32(len(p1)+len(p2)))
	out = append(out, p1...)
	return append(out, p2...)
}
