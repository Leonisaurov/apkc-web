package main

// Verificación de la firma v4: se vuelve a leer el `.idsig` que está al lado del apk,
// se recalcula el árbol de verity del fichero y se comprueba la firma de cada
// SigningInfo. Un apk con rotación v3.1 lleva dos (el viejo como principal y el nuevo en
// un SigningInfoBlock con id 0x1b93ad61); sin rotación, uno. El formato es el que escribe
// `v4sign.go`, medido contra `apksigner`; lo que no se ha medido (sal no vacía, bloques
// de otros ids) se rechaza con su nombre.

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
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

type signerV4 struct {
	apkDigest []byte
	certDER   []byte
	extra     []byte
	pubDER    []byte
	sigAlg    uint32
	firma     []byte
}

func leerSignerV4(l *lectorV4) (signerV4, error) {
	var s signerV4
	var err error
	if s.apkDigest, err = l.bytes(); err != nil {
		return s, err
	}
	if s.certDER, err = l.bytes(); err != nil {
		return s, err
	}
	if s.extra, err = l.bytes(); err != nil {
		return s, err
	}
	if s.pubDER, err = l.bytes(); err != nil {
		return s, err
	}
	if s.sigAlg, err = l.u32(); err != nil {
		return s, err
	}
	if s.firma, err = l.bytes(); err != nil {
		return s, err
	}
	return s, nil
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
	principal, err := leerSignerV4(s)
	if err != nil {
		v.maloV4("SigningInfo: " + err.Error())
		return
	}
	firmantes := []signerV4{principal}
	for s.o < len(signing) {
		blockID, err := s.u32()
		if err != nil {
			v.maloV4("SigningInfoBlock: " + err.Error())
			return
		}
		if blockID != apkSigningBlockIDv3_1 {
			v.maloV4(fmt.Sprintf("SigningInfoBlock con id 0x%08x: solo se mide el de v3.1", blockID))
			return
		}
		bloque, err := s.bytes()
		if err != nil {
			v.maloV4("SigningInfoBlock: " + err.Error())
			return
		}
		bl := &lectorV4{b: bloque}
		extra, err := leerSignerV4(bl)
		if err != nil || bl.o != len(bloque) {
			v.maloV4("SigningInfoBlock: el firmante alternativo no se puede leer")
			return
		}
		firmantes = append(firmantes, extra)
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

	for i, s := range firmantes {
		quien := fmt.Sprintf("firmante %d", i+1)
		if len(v.digestV2) == 0 {
			v.maloV4("no hay digest v2 con el que comparar el del .idsig")
			return
		}
		if !bytes.Equal(s.apkDigest, v.digestV2) {
			v.maloV4(quien + ": el digest del .idsig no es el del bloque v2/v3 del apk")
			return
		}
		cert, err := x509.ParseCertificate(s.certDER)
		if err != nil {
			v.maloV4(quien + ": el certificado del .idsig no se puede leer: " + err.Error())
			return
		}
		pub, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
		if err != nil || !bytes.Equal(pub, s.pubDER) {
			v.maloV4(quien + ": la clave pública del .idsig no es la del certificado")
			return
		}
		alg, h, ok := hashDeClave(cert.PublicKey)
		if !ok {
			v.maloV4(quien + ": el certificado del .idsig no lleva una clave RSA o EC medida")
			return
		}
		if s.sigAlg != alg {
			v.maloV4(fmt.Sprintf("%s: algoritmo de firma 0x%04x; la clave pide 0x%04x", quien, s.sigAlg, alg))
			return
		}
		// la misma carga que arma v4sign.go, con el tamaño del fichero final
		firmado := ponU32(nil, 0)
		firmado = ponU64(firmado, uint64(len(apk)))
		firmado = ponU32(firmado, algoritmo)
		firmado = append(firmado, log2)
		firmado = ponBytes(firmado, sal)
		firmado = ponBytes(firmado, raizDicha)
		firmado = ponBytes(firmado, s.apkDigest)
		firmado = ponBytes(firmado, s.certDER)
		firmado = ponBytes(firmado, s.extra)
		binary.LittleEndian.PutUint32(firmado[:4], uint32(len(firmado)))
		hh := h.New()
		hh.Write(firmado)
		resumen := hh.Sum(nil)
		switch k := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			if err := rsa.VerifyPKCS1v15(k, h, resumen, s.firma); err != nil {
				v.maloV4(quien + ": la firma RSA del .idsig no verifica con su certificado")
				return
			}
		case *ecdsa.PublicKey:
			if !ecdsa.VerifyASN1(k, resumen, s.firma) {
				v.maloV4(quien + ": la firma ECDSA del .idsig no verifica con su certificado")
				return
			}
		}
	}
	v.v4 = len(v.v4Motivos) == 0
}
