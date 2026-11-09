package main

// Verificar un apk con lectores propios: el punto 0.2.2 del plan, y lo único que le falta a la
// pestaña para poder decir algo sobre el apk que ella misma acaba de escribir.
//
// Los dos esquemas que esta herramienta escribe (v1 en `v1sign.go`, v2 en `apksig.go`) se comprueban
// aquí volviendo a leer el paquete desde el disco: el manifiesto se re-analiza, el digest de cada
// entrada se recalcula sobre sus bytes descomprimidos, el `.SF` se digiere otra vez sección a
// sección, y el bloque de firmas se busca antes del directorio central como lo describe el formato.
// Con el escritor solo se comparten las constantes y la función de digest; no el estado del build,
// porque un verificador que lee lo que el escritor recuerda no verifica nada.
//
// Lo que no está medido se rechaza con nombre, como en el resto del fork: se sabe leer una firma v1
// con SHA-256, una v2 con RSASSA-PKCS1-v1_5/SHA-256 de un único firmante y el `.idsig` de la v4.
// DSA, SHA-1, v3, varios firmantes o un certificado que no empareja con su clave pública se dicen y
// no se aprueban. Tampoco se valida la cadena de confianza: el certificado de debug es auto-firmado
// y no hay ancla que lo respalde, así que se informa de él (asunto, serie, caducidad) sin afirmar
// que sea digno de confianza; `apksigner` hace exactamente lo mismo y lo llama "not trusted".

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	idParV2      = uint32(0x7109871a)
	idParV3      = uint32(0xf05368c0)
	idParV3_1    = uint32(0xf05368c1)
	idParV4      = uint32(0x42726576)
	idParRelleno = uint32(0x42726577)

	// el único par de algoritmo que esta herramienta escribe y sabe verificar
	algRSASSAConSHA256 = uint32(sigAlgRSASSAWithSHA256)

	// el registro final: 22 bytes fijos y el desplazamiento del directorio central en [16:20]
	tamanoRegistroFinal = 22
	sitioDesplazamiento = 16
)

// verificacion es el resultado de mirar un paquete: un veredicto por esquema con sus motivos, el
// número de firmantes y lo que se puede decir del certificado sin prometer confianza.
type verificacion struct {
	entradas       int
	entradasFirmas int
	v1             bool
	v2             bool
	v4             bool
	bundle         bool
	v4Presente     bool
	v1Motivos      []string
	v2Motivos      []string
	v4Motivos      []string
	notas          []string
	firmantes      int
	certificado    string
	digestV2       []byte
}

func verificar() {
	set := flag.NewFlagSet("verify", flag.ExitOnError)
	set.Parse(os.Args[2:])

	paquetes := set.Args()
	if len(paquetes) == 0 {
		LogF("verify", "apkc verify necesita el paquete: apkc verify app.apk")
	}
	if len(paquetes) > 1 {
		LogF("verify", "apkc verify mira un paquete; han sobrado nombres después de", paquetes[0], paquetes[1:])
	}

	v, err := verificarApk(paquetes[0])
	if err != nil {
		LogF("verify", err.Error())
	}
	// los dos veredictos se dicen siempre: salir al primer fallo ocultaría el estado del otro
	// esquema, y "este paquete tiene v2 y no tiene v1" es justo la frase que alguien necesita oír.
	// Un .aab solo tiene v1: pedirle v2 sería reprocharle el formato que le corresponde.
	if v.v1 {
		LogI("verify", "v1 (JAR signing): ok —", v.entradasFirmas, "entradas con su digest y la firma PKCS#7 verificada")
	} else {
		LogE("verify", "v1 (JAR signing): mal —", strings.Join(v.v1Motivos, " — "))
	}
	if !v.bundle {
		if v.v2 {
			LogI("verify", "v2 (APK Signature Scheme v2): ok —", v.entradas, "entradas cubiertas por el digest del bloque")
		} else {
			LogE("verify", "v2 (APK Signature Scheme v2): mal —", strings.Join(v.v2Motivos, " — "))
		}
		if v.v4Presente {
			if v.v4 {
				LogI("verify", "v4 (APK Signature Scheme v4): ok — árbol de verity y firma del .idsig")
			} else {
				LogE("verify", "v4 (APK Signature Scheme v4): mal —", strings.Join(v.v4Motivos, " — "))
			}
		}
	}
	if v.certificado != "" {
		LogI("verify", "certificado", v.certificado, "· firmantes", v.firmantes,
			"· la cadena de confianza no se valida: no hay ancla")
	}
	for _, nota := range v.notas {
		LogW("verify", nota)
	}
	if !v.v1 || (!v.bundle && !v.v2) || (v.v4Presente && !v.v4) {
		os.Exit(1)
	}
}

// verificarApk abre el paquete como lo abriría cualquier lector: archive/zip para listar y leer las
// entradas, y el descriptor del fichero para los tramos crudos que necesita el digest de la v2.
func verificarApk(ruta string) (*verificacion, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("el paquete no se puede listar: %w", err)
	}

	v := &verificacion{entradas: len(zr.File)}
	// un .aab es un zip firmado como JAR: no tiene bloque v2/v3, y pedirle uno sería
	// reprocharle el formato que le corresponde.
	if strings.HasSuffix(strings.ToLower(ruta), ".aab") {
		v.bundle = true
		v.verificarV1(zr)
		v.notas = append(v.notas, "el paquete es un .aab: la firma es JAR v1 y no lleva bloque v2/v3")
		return v, nil
	}
	v.verificarV2(f, info.Size())
	v.verificarV1(zr)
	v.verificarV4(ruta)
	return v, nil
}

// --- v1: manifiesto, fichero de firma y bloque PKCS#7 ---------------------------------------------

func (v *verificacion) verificarV1(zr *zip.Reader) {
	var mani, sf, bloque *zip.File
	for _, file := range zr.File {
		name := file.Name
		switch {
		case name == v1ManifestEntry:
			mani = file
		case strings.HasPrefix(name, "META-INF/") && strings.HasSuffix(name, ".SF"):
			sf = file
		case strings.HasPrefix(name, "META-INF/") && strings.HasSuffix(name, ".RSA"):
			bloque = file
		case strings.HasPrefix(name, "META-INF/") && (strings.HasSuffix(name, ".DSA") || strings.HasSuffix(name, ".EC")):
			v.maloV1(name + " no es un bloque RSA: aquí solo se verifica RSASSA-PKCS1-v1_5")
		}
	}
	if mani == nil || sf == nil || bloque == nil {
		v.maloV1("faltan piezas de la firma v1 dentro del paquete")
		return
	}

	manifesto, err := contenidoDe(mani)
	if err != nil {
		v.maloV1("no se puede leer MANIFEST.MF: " + err.Error())
		return
	}
	bytesSF, err := contenidoDe(sf)
	if err != nil {
		v.maloV1("no se puede leer " + sf.Name + ": " + err.Error())
		return
	}
	bloquePKCS7, err := contenidoDe(bloque)
	if err != nil {
		v.maloV1("no se puede leer " + bloque.Name + ": " + err.Error())
		return
	}

	secciones := seccionesDeManifiesto(manifesto)
	if len(secciones) == 0 {
		v.maloV1("el manifiesto no trae ninguna sección de entrada")
		return
	}

	// lo que el manifiesto dice de cada entrada tiene que ser el digest de lo que el paquete guarda
	// bajo ese nombre, y ninguna entrada del paquete puede quedarse fuera del manifiesto
	listadas := map[string]bool{}
	for _, seccion := range secciones {
		nombre := atributoDe(seccion, attrName)
		if nombre == "" {
			v.maloV1("hay una sección del manifiesto sin nombre")
			continue
		}
		listadas[nombre] = true
		dicho := atributoDe(seccion, attrDigest)
		if dicho == "" {
			v.maloV1("la sección de " + nombre + " no trae " + attrDigest)
			continue
		}
		file, err := buscarEntrada(zr, nombre)
		if err != nil {
			v.maloV1(nombre + " está en el manifiesto pero no en el paquete")
			continue
		}
		contenido, err := contenidoDe(file)
		if err != nil {
			v.maloV1("no se puede leer " + nombre + ": " + err.Error())
			continue
		}
		if obtenido := base64SHA256(contenido); obtenido != dicho {
			v.maloV1("el " + attrDigest + " de " + nombre + " no es el de sus bytes")
			continue
		}
		v.entradasFirmas++
	}
	for _, file := range zr.File {
		nombre := file.Name
		if strings.HasPrefix(nombre, "META-INF/") || strings.HasSuffix(nombre, "/") || listadas[nombre] {
			continue
		}
		v.maloV1(nombre + " viaja en el paquete fuera del manifiesto: no está firmada")
	}

	// el .SF digiere el manifiesto entero y cada una de sus secciones, con los bytes tal como están
	dichoMain := atributoDe(primerSeccion(bytesSF), attrDigestMain)
	if dichoMain == "" {
		v.maloV1("el .SF no trae " + attrDigestMain)
	} else if obtenido := base64SHA256(manifesto); obtenido != dichoMain {
		v.maloV1(attrDigestMain + " del .SF no es el digest del manifiesto")
	}
	for _, seccion := range secciones {
		nombre := atributoDe(seccion, attrName)
		if nombre == "" {
			continue
		}
		seccionSF := seccionDel(nombre, bytesSF)
		if seccionSF == "" {
			v.maloV1("el .SF no tiene sección para " + nombre)
			continue
		}
		if dicho := atributoDe(seccionSF, attrDigest); dicho != base64SHA256([]byte(seccion)) {
			v.maloV1("el .SF digiere " + nombre + " distinto de lo que el manifiesto guarda")
		}
	}

	// y la firma del .SF: PKCS#7 detached, con los bytes del .SF como contenido firmado
	cert, firma, err := leerPKCS7(bloquePKCS7)
	if err != nil {
		v.maloV1("el bloque PKCS#7 no se puede leer: " + err.Error())
		return
	}
	clave, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		v.maloV1("el certificado del bloque no lleva clave RSA")
		return
	}
	resumen := sha256.Sum256(bytesSF)
	if err := rsa.VerifyPKCS1v15(clave, crypto.SHA256, resumen[:], firma); err != nil {
		v.maloV1("la firma RSA del .SF no verifica con su propio certificado")
		return
	}

	v.registrarCertificado(cert)
	v.v1 = len(v.v1Motivos) == 0
}

func (v *verificacion) maloV1(motivo string) {
	v.v1Motivos = append(v.v1Motivos, motivo)
}

func buscarEntrada(zr *zip.Reader, nombre string) (*zip.File, error) {
	for _, file := range zr.File {
		if file.Name == nombre {
			return file, nil
		}
	}
	return nil, errors.New("no existe")
}

func contenidoDe(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// atributoDe lee un atributo de una sección que tiene varios. `attrValue` vale para lo que el
// escritor produce (el nombre abre la sección y lo que sigue son continuación de ese mismo valor),
// pero un manifiesto tiene `Name:` en la primera línea y el digest en la segunda, y un `.SF` puede
// traer además `X-Android-APK-Signed` y dos digests distintos: aquí se recorre la sección línea a
// línea y se unen las continuaciones, que empiezan con un espacio.
func atributoDe(seccion, atributo string) string {
	lineas := strings.Split(seccion, "\r\n")
	for i, linea := range lineas {
		prefijo := atributo + ": "
		if !strings.HasPrefix(linea, prefijo) {
			continue
		}
		valor := strings.TrimPrefix(linea, prefijo)
		for _, continuacion := range lineas[i+1:] {
			if !strings.HasPrefix(continuacion, " ") {
				break
			}
			valor += strings.TrimPrefix(continuacion, " ")
		}
		return valor
	}
	return ""
}

// seccionesDeManifiesto parte el manifiesto como lo parte el escritor: el separador "\r\n\r\n" se
// queda al final de cada sección, porque esa sección con su línea en blanco es lo que se digiere.
func seccionesDeManifiesto(manifesto []byte) []string {
	partes := strings.SplitAfter(string(manifesto), "\r\n\r\n")
	salida := []string{}
	for _, s := range partes[1:] {
		if strings.HasPrefix(s, attrName+":") {
			salida = append(salida, s)
		}
	}
	return salida
}

func primerSeccion(b []byte) string {
	partes := strings.SplitAfter(string(b), "\r\n\r\n")
	if len(partes) == 0 {
		return ""
	}
	return partes[0]
}

func seccionDel(nombre string, bytesSF []byte) string {
	for _, s := range seccionesDeManifiesto(bytesSF) {
		if atributoDe(s, attrName) == nombre {
			return s
		}
	}
	return ""
}

// leerPKCS7 saca el primer certificado y la firma del único signerInfo, de la estructura que escribe
// `pkcs7Signature`: ContentInfo/SignedData con los certificados en un [0] IMPLICIT y sin atributos
// autenticados, porque eso es lo que openssl y apksig verifican de una firma JAR.
func leerPKCS7(b []byte) (*x509.Certificate, []byte, error) {
	var envoltura pkcs7ContentInfo
	if _, err := asn1.Unmarshal(b, &envoltura); err != nil {
		return nil, nil, err
	}
	if !envoltura.ContentType.Equal(oidPKCS7SignedData) {
		return nil, nil, fmt.Errorf("el bloque no es pkcs7-signedData sino %s", envoltura.ContentType.String())
	}
	var dentro pkcs7SignedData
	if _, err := asn1.Unmarshal(envoltura.Content.Bytes, &dentro); err != nil {
		return nil, nil, fmt.Errorf("el SignedData no se puede leer: %w", err)
	}
	if len(dentro.SignerInfos) != 1 {
		return nil, nil, fmt.Errorf("el bloque trae %d signerInfo y aquí se verifica con uno", len(dentro.SignerInfos))
	}
	signer := dentro.SignerInfos[0]
	if !signer.DigestAlgorithm.Algorithm.Equal(oidSHA256) {
		return nil, nil, fmt.Errorf("el digest firmado es %s, no SHA-256", signer.DigestAlgorithm.Algorithm.String())
	}
	if !signer.DigestEncryptionAlgorithm.Algorithm.Equal(oidRSAEncryption) {
		return nil, nil, fmt.Errorf("la firma no es RSA sino %s", signer.DigestEncryptionAlgorithm.Algorithm.String())
	}
	if len(dentro.Certificates.Bytes) == 0 {
		return nil, nil, errors.New("el bloque no trae certificados")
	}
	var resto asn1.RawValue
	if _, err := asn1.Unmarshal(dentro.Certificates.Bytes, &resto); err != nil {
		return nil, nil, fmt.Errorf("el certificado del bloque no se puede leer: %w", err)
	}
	cert, err := x509.ParseCertificate(resto.FullBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("el DER del certificado no es un certificado: %w", err)
	}
	return cert, signer.EncryptedDigest, nil
}

// --- v2: el bloque antes del directorio central -----------------------------------------------------

type parDeBloque struct {
	id    uint32
	valor []byte
}

func (v *verificacion) verificarV2(f *os.File, tamano int64) {
	eocd, err := leerRegistroFinal(f, tamano)
	if err != nil {
		v.maloV2(err.Error())
		return
	}
	bloque, err := leerBloqueDeFirmas(f, eocd)
	if err != nil {
		v.maloV2(err.Error())
		return
	}
	if bloque == nil {
		v.maloV2("no hay bloque de firmas antes del directorio central: el paquete no tiene v2")
		return
	}

	var blobV2 []byte
	for _, par := range bloque.pares {
		switch par.id {
		case idParV2:
			blobV2 = par.valor
		case idParV3, idParV3_1:
			v.notar(fmt.Sprintf("el paquete trae también una firma v3 (par 0x%08x): no se verificó aquí", par.id))
		case idParV4:
			v.notar("el paquete trae también una firma v4: no se verificó aquí")
		case idParRelleno:
		default:
			v.notar(fmt.Sprintf("par desconocido en el bloque de firmas: 0x%08x", par.id))
		}
	}
	if blobV2 == nil {
		if len(v.v2Motivos) == 0 {
			v.maloV2("el bloque de firmas no trae el par de la v2 (0x7109871a)")
		}
		return
	}

	// el valor del par es una secuencia de versiones, cada versión una secuencia de firmantes y cada
	// firmante una secuencia de tres elementos: datos-firmados, firma y clave pública. Son tres
	// envoltorios, no dos: con uno el fichero parsea como firmante del tamaño equivocado y
	// `apksigner` lo descarta como "malformed signature record".
	versiones, err := leerSecuencia(blobV2)
	if err != nil || len(versiones) == 0 {
		v.maloV2("el valor del par v2 no trae ninguna versión firmada")
		return
	}
	signers, err := leerSecuencia(versiones[0])
	if err != nil {
		v.maloV2("la secuencia de firmantes no se puede leer: " + err.Error())
		return
	}
	v.firmantes = len(signers)
	if len(signers) != 1 {
		v.maloV2(fmt.Sprintf("hay %d firmantes y aquí se verifica con uno", len(signers)))
		return
	}
	elementos, err := leerSecuencia(signers[0])
	if err != nil || len(elementos) != 3 {
		v.maloV2("el firmante no trae datos-firmados, firmas y clave pública")
		return
	}
	datosFirmados, firmas, claveDicha := elementos[0], elementos[1], elementos[2]

	// los datos-firmados son cuatro elementos: la secuencia de (algoritmo, digest), la de
	// certificados, los atributos adicionales y la firma v1 embebida, esta última vacía
	centro, err := leerSecuencia(datosFirmados)
	if err != nil || len(centro) != 4 {
		v.maloV2("los datos-firmados no traen los cuatro elementos del formato")
		return
	}
	digests, err := leerPares(centro[0])
	if err != nil {
		v.maloV2("los digests del firmante no se pueden leer: " + err.Error())
		return
	}
	certs, err := leerSecuencia(centro[1])
	if err != nil || len(certs) == 0 {
		v.maloV2("el firmante no trae certificado")
		return
	}
	cert, err := x509.ParseCertificate(certs[0])
	if err != nil {
		v.maloV2("el certificado del firmante no se parsea: " + err.Error())
		return
	}
	clave, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		v.maloV2("el certificado del firmante no lleva clave RSA")
		return
	}
	if der, err := x509.MarshalPKIXPublicKey(cert.PublicKey); err != nil || !bytes.Equal(der, claveDicha) {
		v.maloV2("la clave pública del firmante no es la del certificado")
		return
	}
	paresFirmas, err := leerPares(firmas)
	if err != nil {
		v.maloV2("las firmas del firmante no se pueden leer: " + err.Error())
		return
	}

	// el digest cubre los tres tramos del paquete, con el campo de desplazamiento del registro final
	// sustituido por el desplazamiento del bloque: sin esa sustitución la firma no podría cubrir el
	// registro que depende de ella
	directorio := make([]byte, eocd.cdTamaño)
	if _, err := f.ReadAt(directorio, eocd.cdOffset); err != nil {
		v.maloV2("el directorio central no se lee: " + err.Error())
		return
	}
	registroFinal := make([]byte, tamano-eocd.pos)
	if _, err := f.ReadAt(registroFinal, eocd.pos); err != nil {
		v.maloV2("el registro final no se lee: " + err.Error())
		return
	}
	binary.LittleEndian.PutUint32(registroFinal[sitioDesplazamiento:], uint32(bloque.inicio))

	obtenido, err := chunkedSHA256(fileSection(f, bloque.inicio), bytesSection(directorio), bytesSection(registroFinal))
	if err != nil {
		v.maloV2("el digest del contenido no se puede calcular: " + err.Error())
		return
	}
	v.digestV2 = obtenido

	medido := 0
	for _, par := range digests {
		if par.id != algRSASSAConSHA256 {
			v.maloV2(fmt.Sprintf("algoritmo de digest 0x%04x no medido aquí", par.id))
			continue
		}
		medido++
		if !bytes.Equal(par.valor, obtenido) {
			v.maloV2("el digest guardado no es el de este contenido: el paquete cambió después de firmarse")
		}
	}
	for _, par := range paresFirmas {
		if par.id != algRSASSAConSHA256 {
			v.maloV2(fmt.Sprintf("algoritmo de firma 0x%04x no medido aquí", par.id))
			continue
		}
		medido++
		resumen := sha256.Sum256(datosFirmados)
		if err := rsa.VerifyPKCS1v15(clave, crypto.SHA256, resumen[:], par.valor); err != nil {
			v.maloV2("la firma RSA sobre los datos-firmados no verifica")
		}
	}
	if medido == 0 {
		v.maloV2("el firmante no trae ningún par con el algoritmo que esta orden sabe leer")
		return
	}

	v.registrarCertificado(cert)
	v.v2 = len(v.v2Motivos) == 0
}

func (v *verificacion) maloV2(motivo string) {
	v.v2Motivos = append(v.v2Motivos, motivo)
}

// notar es lo contrario de un motivo: algo del paquete que esta orden no comprueba pero que quien
// lea el resultado tiene que saber. Un par v3 no invalida la v1 ni la v2, porque el propio formato
// manda ignorar los pares que no se conocen, así que decirlo como fallo sería un falso negativo.
func (v *verificacion) notar(nota string) {
	v.notas = append(v.notas, nota)
}

func (v *verificacion) registrarCertificado(cert *x509.Certificate) {
	if v.certificado != "" {
		return
	}
	v.certificado = fmt.Sprintf("%s, serie %s, válido hasta %s", cert.Subject.CommonName,
		cert.SerialNumber.String(), cert.NotAfter.Format("2006-01-02"))
}

// registroFinal es el EOCD leído del disco, con su posición para poder recalcular el digest.
type registroFinal struct {
	pos      int64
	cdOffset int64
	cdTamaño uint32
	entradas uint32
}

func leerRegistroFinal(f *os.File, tamano int64) (*registroFinal, error) {
	ventana := tamano
	if ventana > 66000 {
		ventana = 66000
	}
	buf := make([]byte, ventana)
	if _, err := f.ReadAt(buf, tamano-ventana); err != nil {
		return nil, err
	}
	firma := []byte{'P', 'K', 0x05, 0x06}
	for i := len(buf) - tamanoRegistroFinal; i >= 0; i-- {
		if !bytes.Equal(buf[i:i+4], firma) {
			continue
		}
		commento := int(binary.LittleEndian.Uint16(buf[i+20:]))
		if i+tamanoRegistroFinal+commento != len(buf) {
			continue
		}
		return &registroFinal{
			pos:      tamano - ventana + int64(i),
			cdOffset: int64(binary.LittleEndian.Uint32(buf[i+16:])),
			cdTamaño: binary.LittleEndian.Uint32(buf[i+12:]),
			entradas: uint32(binary.LittleEndian.Uint16(buf[i+10:])),
		}, nil
	}
	return nil, errors.New("no hay registro final de directorio central: esto no es un zip")
}

type bloqueDeFirmas struct {
	inicio int64
	largo  int64
	pares  []parDeBloque
}

// leerBloqueDeFirmas busca el contenedor por la magia que lo cierra. Un paquete sin bloque no es un
// error: es un paquete sin v2, y hay que poder decirlo.
func leerBloqueDeFirmas(f *os.File, eocd *registroFinal) (*bloqueDeFirmas, error) {
	if eocd.cdOffset < 24 {
		return nil, nil
	}
	trasero := make([]byte, 24)
	if _, err := f.ReadAt(trasero, eocd.cdOffset-24); err != nil {
		return nil, err
	}
	if string(trasero[8:]) != apkSigningBlockMagic {
		return nil, nil
	}
	// cada campo de tamaño dice cuántos bytes hay desde él hasta la magia, sin contarse a sí mismo
	largo := binary.LittleEndian.Uint64(trasero[:8])
	inicio := eocd.cdOffset - int64(largo) - 8
	if inicio < 0 {
		return nil, errors.New("el bloque de firmas apunta antes del principio del fichero")
	}
	primero := make([]byte, 8)
	if _, err := f.ReadAt(primero, inicio); err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint64(primero) != largo {
		return nil, errors.New("los dos campos de tamaño del bloque de firmas no coinciden")
	}

	cuerpo := make([]byte, largo-24)
	if _, err := f.ReadAt(cuerpo, inicio+8); err != nil {
		return nil, err
	}
	bloque := &bloqueDeFirmas{inicio: inicio, largo: int64(largo) + 8}
	// cada par del bloque lleva delante un uint64 con el tamaño de lo que viene después de él mismo:
	// el id (4) y su valor. El registro entero mide por tanto 8+t, y leer el prefijo como uint32 es
	// tropezarse con el id del par siguiente.
	for len(cuerpo) >= 12 {
		t := int(binary.LittleEndian.Uint64(cuerpo))
		if t < 4 || 8+t > len(cuerpo) {
			return nil, errors.New("un par del bloque anuncia " + fmt.Sprint(t) + " bytes y no caben")
		}
		id := binary.LittleEndian.Uint32(cuerpo[8:])
		valor := t - 4
		bloque.pares = append(bloque.pares, parDeBloque{id: id, valor: cuerpo[12 : 12+valor]})
		cuerpo = cuerpo[8+t:]
	}
	return bloque, nil
}

// leerSecuencia parte una secuencia de elementos con prefijo uint32, que es cómo `seqOf` escribe.
func leerSecuencia(b []byte) ([][]byte, error) {
	out := [][]byte{}
	for len(b) >= 4 {
		n := int(binary.LittleEndian.Uint32(b))
		if n+4 > len(b) {
			return nil, fmt.Errorf("un elemento pide %d bytes y quedan %d", n, len(b)-4)
		}
		out = append(out, b[4:4+n])
		b = b[4+n:]
	}
	if len(b) != 0 {
		return nil, fmt.Errorf("sobran %d bytes al final de la secuencia", len(b))
	}
	return out, nil
}

// leerPares parte una secuencia de elementos como la que escribe `pairsOf`: prefijo uint32 con el
// tamaño de lo que sigue (id de 4, tamaño del valor de 4 y el valor), o sea 8 + valor.
func leerPares(b []byte) ([]parDeBloque, error) {
	out := []parDeBloque{}
	for len(b) >= 12 {
		t := int(binary.LittleEndian.Uint32(b))
		if t < 8 || t+4 > len(b) {
			return nil, fmt.Errorf("un par pide %d bytes y quedan %d", t, len(b)-4)
		}
		id := binary.LittleEndian.Uint32(b[4:])
		valor := int(binary.LittleEndian.Uint32(b[8:]))
		if valor+8 != t {
			return nil, errors.New("el valor de un par no cabe en su propio tamaño")
		}
		out = append(out, parDeBloque{id: id, valor: b[12 : 12+valor]})
		b = b[t+4:]
	}
	if len(b) != 0 {
		return nil, fmt.Errorf("sobran %d bytes al final de los pares", len(b))
	}
	return out, nil
}
