package main

// Un keystore de depuración *típico*, generado dentro del propio apkc: alias
// `androiddebugkey`, contraseña `android`, sujeto `CN=Android Debug` y una clave RSA de
// 2048 bits. La pestaña lo crea una vez en OPFS y lo reutiliza, así que dos compilaciones
// debug de la misma instalación comparten identidad y una apk se puede actualizar encima
// de la otra sin desinstalar.
//
// El formato es JKS, no PKCS#12: el lector de este fork ya lo entiende y escribirlo no
// necesita ni una JVM. El sobre de la clave es el `KeyProtector` de Sun (OID
// 1.3.6.1.4.1.42.2.17.1.1): salt SHA-1, XOR del PKCS#8 y un SHA-1 de comprobación; el
// fichero termina con el digest "Mighty Aphrodite" que usa JKS.

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type algoritmoDeClaveJKS struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type claveProtegidaJKS struct {
	Algorithm algoritmoDeClaveJKS
	Data      []byte
}

func comandoDebugKeystore() {
	set := flag.NewFlagSet("debug-keystore", flag.ExitOnError)
	set.Usage = func() {
		fmt.Print(`usage: apkc debug-keystore --out debug.keystore [--storepass android] [--keyalias androiddebugkey]

Writes the JKS debug keystore a normal Android build uses: RSA 2048, alias
androiddebugkey, store password android and subject CN=Android Debug,O=Android,C=US.
It starts no process and needs no SDK. Refuses to overwrite an existing file unless
--force is passed: replacing a debug key makes the previous install unupdatable.
`)
	}
	out := set.String("out", "", "where to write the JKS debug keystore")
	storepass := set.String("storepass", "android", "store password, and key password, of the generated JKS")
	alias := set.String("keyalias", "androiddebugkey", "alias of the generated private key")
	days := set.Int("validity-days", 10000, "validity of the self-signed debug certificate")
	force := set.Bool("force", false, "overwrite an existing keystore")
	if err := set.Parse(os.Args[2:]); err != nil {
		LogF("debug-keystore", err.Error())
	}
	if *out == "" {
		LogF("debug-keystore", "--out is required")
	}
	if _, err := os.Stat(*out); err == nil && !*force {
		LogF("debug-keystore", *out, "already exists; use --force to replace it")
	}
	if err := escribirDebugKeystore(*out, *storepass, *alias, *days); err != nil {
		LogF("debug-keystore", err.Error())
	}
	LogI("debug-keystore", "wrote", *out, "alias", *alias, "subject CN=Android Debug")
}

type datosKeystore struct {
	ruta, password, alias, commonName, organization, country string
	dias                                                     int
}

func comandoCreateKeystore() {
	set := flag.NewFlagSet("create-keystore", flag.ExitOnError)
	set.Usage = func() {
		fmt.Print(`usage: apkc create-keystore --out release.jks --storepass <pass> --keyalias <alias>
       [--common-name <CN>] [--organization <O>] [--country <C>] [--validity-days 10000] [--force]

Writes a JKS keystore for release signing, with no process and no JVM. The certificate is
self-signed and valid for 10000 days by default. Refuses to overwrite unless --force is passed:
replacing a release key makes previous installs unupdatable.
`)
	}
	out := set.String("out", "", "where to write the JKS keystore")
	storepass := set.String("storepass", "", "store and key password (required)")
	alias := set.String("keyalias", "release", "alias of the generated private key")
	commonName := set.String("common-name", "", "certificate CN; defaults to the alias")
	organization := set.String("organization", "apkc studio", "certificate O")
	country := set.String("country", "ES", "certificate C")
	days := set.Int("validity-days", 10000, "validity of the self-signed certificate")
	force := set.Bool("force", false, "overwrite an existing keystore")
	if err := set.Parse(os.Args[2:]); err != nil {
		LogF("create-keystore", err.Error())
	}
	if *out == "" {
		LogF("create-keystore", "--out is required")
	}
	if *storepass == "" {
		LogF("create-keystore", "--storepass is required")
	}
	if _, err := os.Stat(*out); err == nil && !*force {
		LogF("create-keystore", *out, "already exists; use --force to replace it")
	}
	if err := escribirKeystore(datosKeystore{
		ruta: *out, password: *storepass, alias: *alias, dias: *days,
		commonName: *commonName, organization: *organization, country: *country,
	}); err != nil {
		LogF("create-keystore", err.Error())
	}
	LogI("create-keystore", "wrote", *out, "alias", *alias, "subject CN="+*commonName)
}

func escribirDebugKeystore(ruta, password, alias string, dias int) error {
	return escribirKeystore(datosKeystore{
		ruta: ruta, password: password, alias: alias, dias: dias,
		commonName: "Android Debug", organization: "Android", country: "US",
	})
}

func escribirKeystore(d datosKeystore) error {
	if d.password == "" {
		return fmt.Errorf("la contraseña del keystore no puede estar vacía")
	}
	if d.alias == "" {
		return fmt.Errorf("el alias no puede estar vacío")
	}
	if d.dias <= 0 {
		d.dias = 10000
	}
	if d.commonName == "" {
		d.commonName = d.alias
	}
	clave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("no se pudo generar la clave RSA: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		return fmt.Errorf("no se pudo generar el serial del certificado: %w", err)
	}
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}
	ahora := time.Now()
	sujeto := pkix.Name{CommonName: d.commonName}
	if d.organization != "" {
		sujeto.Organization = []string{d.organization}
	}
	if d.country != "" {
		sujeto.Country = []string{d.country}
	}
	plantilla := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               sujeto,
		NotBefore:             ahora.Add(-time.Hour),
		NotAfter:              ahora.Add(time.Duration(d.dias) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		return fmt.Errorf("no se pudo crear el certificado: %w", err)
	}
	claveDER, err := x509.MarshalPKCS8PrivateKey(clave)
	if err != nil {
		return fmt.Errorf("no se pudo serializar la clave: %w", err)
	}
	sealed, err := sellarClaveJKS(claveDER, d.password)
	if err != nil {
		return err
	}
	blob := jksConClave(d.alias, sealed, certDER, d.password)
	if err := os.MkdirAll(filepath.Dir(d.ruta), 0o755); err != nil {
		return err
	}
	return os.WriteFile(d.ruta, blob, 0o600)
}

func sellarClaveJKS(claveDER []byte, password string) ([]byte, error) {
	pb := jksPasswordBytes(password)
	salt := make([]byte, jksSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("no se pudo generar la sal del keystore: %w", err)
	}
	flujo := make([]byte, len(claveDER))
	anterior := salt
	for off := 0; off < len(flujo); {
		h := sha1.New()
		h.Write(pb)
		h.Write(anterior)
		anterior = h.Sum(nil)
		off += copy(flujo[off:], anterior)
	}
	cuerpo := make([]byte, len(claveDER))
	for i := range claveDER {
		cuerpo[i] = claveDER[i] ^ flujo[i]
	}
	h := sha1.New()
	h.Write(pb)
	h.Write(claveDER)
	datos := make([]byte, 0, len(salt)+len(cuerpo)+jksDigestLen)
	datos = append(datos, salt...)
	datos = append(datos, cuerpo...)
	datos = append(datos, h.Sum(nil)...)
	return asn1.Marshal(claveProtegidaJKS{
		Algorithm: algoritmoDeClaveJKS{
			Algorithm:  asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 42, 2, 17, 1, 1},
			Parameters: asn1.NullRawValue,
		},
		Data: datos,
	})
}

func jksConClave(alias string, sealed, certDER []byte, password string) []byte {
	var b []byte
	b = jksU32(b, jksMagic)
	b = jksU32(b, 2)
	b = jksU32(b, 1) // una sola entrada
	b = jksU32(b, jksPrivateEntry)
	b = jksUTF(b, alias)
	b = jksU64(b, uint64(time.Now().UnixMilli()))
	b = jksU32(b, uint32(len(sealed)))
	b = append(b, sealed...)
	b = jksU32(b, 1) // una cadena de certificados
	b = jksUTF(b, "X.509")
	b = jksU32(b, uint32(len(certDER)))
	b = append(b, certDER...)
	h := sha1.New()
	h.Write(jksPasswordBytes(password))
	h.Write([]byte(jksMightyAphrodite))
	h.Write(b)
	return append(b, h.Sum(nil)...)
}

func jksU32(b []byte, v uint32) []byte {
	var x [4]byte
	binary.BigEndian.PutUint32(x[:], v)
	return append(b, x[:]...)
}

func jksU64(b []byte, v uint64) []byte {
	var x [8]byte
	binary.BigEndian.PutUint64(x[:], v)
	return append(b, x[:]...)
}

func jksUTF(b []byte, s string) []byte {
	var x [2]byte
	binary.BigEndian.PutUint16(x[:], uint16(len(s)))
	b = append(b, x[:]...)
	return append(b, s...)
}
