package main

// Verificación de la firma v4: se vuelve a leer el `.idsig` que está al lado del apk,
// se recalcula el árbol de verity del fichero y se comprueba la firma del SigningInfo.
// El formato es el que escribe `v4sign.go`, medido contra `apksigner`; lo que no se ha
// medido (sal no vacía, varios firmantes) se rechaza con su nombre.

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"os"
)

type lectorV4 struct {
	b []byte
	o int
}

func (l *lectorV4) u8() (byte, error) {
	if l.o+1 > len(l.b) {
		return 0, fmt.Errorf("el .idsig termina antes de un byte")
	}
	v := l.b[l.o]
	l.o++
	return v, nil
}

func (l *lectorV4) u32() (uint32, error) {
	if l.o+4 > len(l.b) {
		return 0, fmt.Errorf("el .idsig termina antes de un u32")
	}
	v := binary.LittleEndian.Uint32(l.b[l.o:])
	l.o += 4
	return v, nil
}

func (l *lectorV4) bytes() ([]byte, error) {
	n, err := l.u32()
	if err != nil {
		return nil, err
	}
	if l.o+int(n) > len(l.b) {
		return nil, fmt.Errorf("el .idsig dice %d bytes y solo quedan %d", n, len(l.b)-l.o)
	}
	v := l.b[l.o : l.o+int(n)]
	l.o += int(n)
	return v, nil
}

func (v *verificacion) maloV4(motivo string) {
	v.v4Motivos = append(v.v4Motivos, motivo)
}

// verificarV4 mira el `.idsig` que acompaña al apk. Su ausencia es una nota, no un
// fallo: hay apks antiguos sin v4. Si está, tiene que cuadrar con el fichero entero.
func (v *verificacion) verificarV4(ruta string) {
	idsig, err := os.ReadFile(ruta + ".idsig")
	if err != nil {
		if os.IsNotExist(err) {
			v.notas = append(v.notas, "no hay .idsig al lado del apk: la firma v4 no se verifica")
			return
		}
		v.maloV4("el .idsig no se puede leer: " + err.Error())
		v.v4Presente = true
		return
	}
	v.v4Presente = true
	apk, err := os.ReadFile(ruta)
	if err != nil {
		v.maloV4("el apk no se puede releer para el árbol de verity: " + err.Error())
		return
	}

	c := &lectorV4{b: idsig}
	version, err := c.u32()
	if err != nil {
		v.maloV4(err.Error())
		return
	}
	if version != v4Version {
		v.maloV4(fmt.Sprintf("versión %d del .idsig; solo se mide la %d", version, v4Version))
		return
	}
	hashing, err := c.bytes()
	if err != nil {
		v.maloV4(err.Error())
		return
	}
	signing, err := c.bytes()
	if err != nil {
		v.maloV4(err.Error())
		return
	}
	arbolDicho, err := c.bytes()
	if err != nil {
		v.maloV4(err.Error())
		return
	}
	if c.o != len(idsig) {
		v.maloV4("quedan bytes después del árbol de verity")
		return
	}

	h := &lectorV4{b: hashing}
	algoritmo, err := h.u32()
	if err != nil {
		v.maloV4("HashingInfo: " + err.Error())
		return
	}
	log2, err := h.u8()
	if err != nil {
		v.maloV4("HashingInfo: " + err.Error())
		return
	}
	sal, err := h.bytes()
	if err != nil {
		v.maloV4("HashingInfo: " + err.Error())
		return
	}
	raizDicha, err := h.bytes()
	if err != nil {
		v.maloV4("HashingInfo: " + err.Error())
		return
	}
	if h.o != len(hashing) || algoritmo != v4HashSHA256 || int(log2) != v4Log2BlockSize || len(sal) != 0 {
		v.maloV4("HashingInfo no es la forma medida (SHA-256, 4096, sal vacía)")
		return
	}

	s := &lectorV4{b: signing}
	apkDigest, err := s.bytes()
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	certDER, err := s.bytes()
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	extra, err := s.bytes()
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	pubDER, err := s.bytes()
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	sigAlg, err := s.u32()
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	firma, err := s.bytes()
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	if s.o != len(signing) {
		v.maloV4("el .idsig trae firmantes alternativos, que no están medidos")
		return
	}
	if sigAlg != v4SigAlgRSA {
		v.maloV4(fmt.Sprintf("algoritmo de firma 0x%04x; solo se mide 0x%04x", sigAlg, v4SigAlgRSA))
		return
	}

	// el árbol que el fichero lleva tiene que ser el del apk, byte a byte
	raiz, arbol, err := arbolVerity(apk)
	if err != nil {
		v.maloV4(err.Error())
		return
	}
	if !bytes.Equal(raiz, raizDicha) {
		v.maloV4("la raíz del árbol del .idsig no es la del apk")
		return
	}
	if !bytes.Equal(arbol, arbolDicho) {
		v.maloV4("el árbol del .idsig no es el del apk")
		return
	}

	// el digest del SigningInfo es el del bloque v2/v3 del apk
	if len(v.digestV2) == 0 {
		v.maloV4("no hay digest v2 con el que comparar el del .idsig")
		return
	}
	if !bytes.Equal(apkDigest, v.digestV2) {
		v.maloV4("el digest del .idsig no es el del bloque v2/v3 del apk")
		return
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		v.maloV4("el certificado del .idsig no se puede leer: " + err.Error())
		return
	}
	pub, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil || !bytes.Equal(pub, pubDER) {
		v.maloV4("la clave pública del .idsig no es la del certificado")
		return
	}
	clave, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		v.maloV4("el certificado del .idsig no lleva una clave RSA")
		return
	}

	// y la firma: la misma carga que arma v4sign.go, con el tamaño del fichero final
	firmado := ponU32(nil, 0)
	firmado = ponU64(firmado, uint64(len(apk)))
	firmado = ponU32(firmado, algoritmo)
	firmado = append(firmado, log2)
	firmado = ponBytes(firmado, sal)
	firmado = ponBytes(firmado, raizDicha)
	firmado = ponBytes(firmado, apkDigest)
	firmado = ponBytes(firmado, certDER)
	firmado = ponBytes(firmado, extra)
	binary.LittleEndian.PutUint32(firmado[:4], uint32(len(firmado)))
	resumen := sha256.Sum256(firmado)
	if err := rsa.VerifyPKCS1v15(clave, crypto.SHA256, resumen[:], firma); err != nil {
		v.maloV4("la firma RSA del .idsig no verifica con su certificado")
		return
	}
	v.v4 = len(v.v4Motivos) == 0
}
