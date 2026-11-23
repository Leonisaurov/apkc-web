package main

// El índice del android.jar, entre pasadas.
//
// Abrir el jar con archive/zip y recorrer su directorio central cuesta 90-100 ms en wasm la primera
// pasada de un proceso (medido con `TestIndiceDelJarMideElAhorro` en el hospedador de Node), y se
// pagaba en cada build aunque el proyecto no cambiara: es el coste fijo del jar. Aquí el directorio se
// lee en una pasada propia, sin un *zip.File por entrada, y se deja en build/jarindice.bin con su
// huella en la cabecera; la vuelta siguiente lee ese fichero y no parsea el zip.
//
// La huella es el tamaño del jar y el CRC-32 de su directorio central, **no su fecha**. La fecha no es
// un dato del contenido: el hospedador de la pestaña devolvía una fecha sintética que cambiaba en cada
// recarga aunque el jar fuera el mismo, y con ella se reindexaba y `clasesAlDia()` recomponía el
// proyecto entero por nada. El directorio central es exactamente lo que el índice guarda —nombres,
// offsets, tamaños, CRC y método de cada entrada—, así que su digest cambia cuando cambia algo de lo
// que el índice depende, y solo entonces.
//
// La caché es una comodidad, nunca una fuente de verdad: si falta, si no cuadra con la huella o si
// está truncada se vuelve a recorrer el directorio central, y si eso tampoco sale (zip64, un prefijo
// delante del zip) se cae al archive/zip de siempre.

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sort"
)

const (
	// ficheroIndiceDelJar vive dentro de build/ porque `apkc clean` se lleva el directorio entero y
	// porque la pestaña ya guarda ahí el estado del incremental: la caché no sobrevive a un clean, y
	// no tiene por qué.
	ficheroIndiceDelJar = "build/jarindice.bin"

	// marcaIndiceDelJar dice de qué fichero se trata y con qué forma. El 2 es la forma que lleva el
	// digest del directorio central en vez de la fecha del jar: una caché de la 1 se ignora entera.
	marcaIndiceDelJar   = "apkc-jarindice-2\n"
	largoMarcaIndiceJar = 17

	// cabeceraIndiceDelJar: marca (17), sha1 de la ruta del jar (20), tamaño del jar (8), CRC-32 del
	// directorio central (4), número de entradas (4) y bytes del bloque de nombres (4).
	cabeceraIndiceDelJar = 64
	campoMarca           = 0
	campoRuta            = 17
	campoTamano          = 37
	campoDirectorio      = 45
	campoCuenta          = 49
	campoNombres         = 53

	// registroIndiceDelJar: cabecera local (8), comprimido (4), CRC (4), método (2), largo del
	// nombre (2) e inicio del nombre en su bloque (4).
	registroIndiceDelJar = 24

	// colaMaximaDelJar es lo que puede ocupar un fin de directorio central con el comentario más
	// largo que admite el formato: más atrás no hay otro.
	colaMaximaDelJar = 22 + 0xffff
)

var (
	firmaFinDirectorio     = []byte("PK\x05\x06")
	firmaEntradaDirectorio = []byte("PK\x01\x02")
	firmaCabeceraLocal     = []byte("PK\x03\x04")

	// parseosDelDirectorio cuenta cuántas veces se parseó el directorio central en este proceso. Lo
	// lee una prueba: por fuera, «salió de build/jarindice.bin» y «volvió a recorrer el zip» se
	// comportan igual, y sin esto la reutilización de la caché no se puede afirmar, solo suponer.
	parseosDelDirectorio int
)

// entradaJar dice dónde está una clase dentro del jar y cómo leerla sin archive/zip.
type entradaJar struct {
	cabecera   int64  // el offset de la cabecera local, tal y como lo da el directorio central
	comprimido uint32 // bytes comprimidos de la entrada
	crc        uint32 // CRC-32 del contenido sin comprimir, para no devolver bytes rotos en silencio
	metodo     uint16 // 0 guardado, 8 deflate
	f          *zip.File
}

// huellaDelJar devuelve el tamaño del jar y el CRC-32 de su directorio central: lo que decide si la
// caché y las clases reutilizadas siguen valiendo. Un jar con los mismos bytes no la cambia aunque su
// fecha sí, y uno con otras entradas —o con la misma entrada y otros bytes— la cambia aunque su fecha
// y su tamaño sean los mismos, porque el CRC de cada entrada vive en el directorio central.
//
// Es un CRC y no un SHA-1 porque el SHA-1 del directorio cuesta 8 ms por build en wasm y el CRC 1 ms
// (medido en el hospedador de Node), y aquí el digest no autentica nada: distingue contenido. Una
// colisión de CRC-32 no puede servir bytes equivocados, porque cada entrada se comprueba contra su
// CRC al leerla (datosDeLaEntrada); a lo sumo da una respuesta equivocada a «¿está esta clase en el
// jar?», que falla en voz alta.
func huellaDelJar(rutaJar string) (int64, uint32, error) {
	directorio, tamano, _, err := leeDirectorioCentral(rutaJar)
	if err != nil {
		return 0, 0, err
	}
	return tamano, crc32.ChecksumIEEE(directorio), nil
}

// huellaDelJarTexto resume el jar en una cadena, para el estado del incremental: cambiar el classpath
// tiene que invalidar las clases reutilizadas de I3, no solo la caché del índice. Un jar reescrito con
// los mismos bytes no la cambia —no hay nada que recompilar— y uno con otros bytes sí, sin mirar la
// fecha, que no es contenido.
func huellaDelJarTexto(rutaJar string) string {
	tamano, directorio, err := huellaDelJar(rutaJar)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s|%d|%08x", rutaJar, tamano, directorio)
}

// leerIndiceDelJar devuelve el índice de build/jarindice.bin cuando su cabecera dice que es el de este
// jar. Un fichero de otra forma, de otro jar o a medias no se lee: se devuelve false y el llamador
// parsea el directorio central, que ya tiene en la mano.
func leerIndiceDelJar(rutaJar string, tamano int64, directorio uint32) (map[string]entradaJar, bool) {
	datos, err := os.ReadFile(ficheroIndiceDelJar)
	if err != nil || len(datos) < cabeceraIndiceDelJar {
		return nil, false
	}
	if string(datos[campoMarca:campoMarca+largoMarcaIndiceJar]) != marcaIndiceDelJar {
		return nil, false
	}
	ruta := sha1.Sum([]byte(rutaJar))
	if !bytes.Equal(datos[campoRuta:campoRuta+sha1.Size], ruta[:]) {
		return nil, false
	}
	if int64(binary.LittleEndian.Uint64(datos[campoTamano:])) != tamano ||
		binary.LittleEndian.Uint32(datos[campoDirectorio:]) != directorio {
		return nil, false
	}
	cuenta := int(binary.LittleEndian.Uint32(datos[campoCuenta:]))
	largoNombres := int(binary.LittleEndian.Uint32(datos[campoNombres:]))
	// sin esta cota, un fichero con basura en el número de entradas desborda `base` antes de que
	// ninguna comprobación de longitud lo note
	if cuenta <= 0 || cuenta > len(datos)/registroIndiceDelJar {
		return nil, false
	}
	base := cabeceraIndiceDelJar + cuenta*registroIndiceDelJar
	if base+largoNombres != len(datos) {
		return nil, false
	}
	// un solo string para todo el fichero: cada nombre es un trozo suyo y no se copia al entrar en
	// el mapa, que es la mitad del coste de reconstruir el índice
	texto := string(datos)
	indice := make(map[string]entradaJar, cuenta)
	for i := 0; i < cuenta; i++ {
		r := datos[cabeceraIndiceDelJar+i*registroIndiceDelJar:]
		largo := int(binary.LittleEndian.Uint16(r[18:20]))
		ini := int(binary.LittleEndian.Uint32(r[20:24]))
		if base+ini+largo > len(datos) {
			return nil, false
		}
		indice[texto[base+ini:base+ini+largo]] = entradaJar{
			cabecera:   int64(binary.LittleEndian.Uint64(r[0:8])),
			comprimido: binary.LittleEndian.Uint32(r[8:12]),
			crc:        binary.LittleEndian.Uint32(r[12:16]),
			metodo:     binary.LittleEndian.Uint16(r[16:18]),
		}
	}
	return indice, true
}

// escribirIndiceDelJar deja el índice en disco. Es lo mejor que se puede hacer, no una obligación: sin
// build/ (una prueba que no está en un proyecto) o con el disco lleno se sigue compilando.
func escribirIndiceDelJar(rutaJar string, tamano int64, directorio uint32, indice map[string]entradaJar) {
	if st, err := os.Stat("build"); err != nil || !st.IsDir() {
		return
	}
	nombres := make([]string, 0, len(indice))
	largoNombres := 0
	for nombre := range indice {
		nombres = append(nombres, nombre)
		largoNombres += len(nombre)
	}
	// el orden es el léxico y no el del mapa: dos builds del mismo jar dejan el mismo fichero
	sort.Strings(nombres)
	datos := make([]byte, cabeceraIndiceDelJar+len(nombres)*registroIndiceDelJar+largoNombres)
	copy(datos[campoMarca:], marcaIndiceDelJar)
	ruta := sha1.Sum([]byte(rutaJar))
	copy(datos[campoRuta:], ruta[:])
	binary.LittleEndian.PutUint64(datos[campoTamano:], uint64(tamano))
	binary.LittleEndian.PutUint32(datos[campoDirectorio:], directorio)
	binary.LittleEndian.PutUint32(datos[campoCuenta:], uint32(len(nombres)))
	binary.LittleEndian.PutUint32(datos[campoNombres:], uint32(largoNombres))
	base := cabeceraIndiceDelJar + len(nombres)*registroIndiceDelJar
	enNombres := 0
	for i, nombre := range nombres {
		e := indice[nombre]
		r := datos[cabeceraIndiceDelJar+i*registroIndiceDelJar:]
		binary.LittleEndian.PutUint64(r[0:8], uint64(e.cabecera))
		binary.LittleEndian.PutUint32(r[8:12], e.comprimido)
		binary.LittleEndian.PutUint32(r[12:16], e.crc)
		binary.LittleEndian.PutUint16(r[16:18], e.metodo)
		binary.LittleEndian.PutUint16(r[18:20], uint16(len(nombre)))
		binary.LittleEndian.PutUint32(r[20:24], uint32(enNombres))
		copy(datos[base+enNombres:], nombre)
		enNombres += len(nombre)
	}
	// por un temporal y un rename, como el estado del incremental: un build que muere a mitad no
	// puede dejar un índice corto que la vuelta siguiente lea como bueno (la cabecera lo caza)
	tmp := ficheroIndiceDelJar + ".tmp"
	if err := os.WriteFile(tmp, datos, 0644); err != nil {
		return
	}
	if err := os.Rename(tmp, ficheroIndiceDelJar); err != nil {
		_ = os.Remove(tmp)
	}
}

// leeDirectorioCentral devuelve los bytes del directorio central del zip, el tamaño del fichero y el
// número de entradas que declara su fin de directorio. Es lo que se lee antes de decidir nada: su
// digest es la huella del jar y sus entradas son el índice.
func leeDirectorioCentral(rutaJar string) ([]byte, int64, int, error) {
	f, err := os.Open(rutaJar)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, 0, 0, err
	}
	tamano := st.Size()
	if tamano < 22 {
		return nil, 0, 0, fmt.Errorf("el android.jar %s no llega a un zip", rutaJar)
	}
	cola := int64(colaMaximaDelJar)
	if cola > tamano {
		cola = tamano
	}
	buf := make([]byte, cola)
	if _, err := f.ReadAt(buf, tamano-cola); err != nil {
		return nil, 0, 0, err
	}
	// el fin del directorio se busca de atrás adelante y se acepta el primero cuyo comentario
	// termine exactamente con el fichero: uno que solo aparezca dentro de un comentario no vale
	var eocd []byte
	for hasta := len(buf); hasta >= 22; {
		j := bytes.LastIndex(buf[:hasta], firmaFinDirectorio)
		if j < 0 {
			break
		}
		if j+22 <= len(buf) {
			largo := int(binary.LittleEndian.Uint16(buf[j+20 : j+22]))
			if j+22+largo == len(buf) {
				eocd = buf[j:]
				break
			}
		}
		hasta = j
	}
	if eocd == nil {
		return nil, 0, 0, fmt.Errorf("el android.jar %s no tiene fin de directorio central", rutaJar)
	}
	cuenta := int(binary.LittleEndian.Uint16(eocd[10:12]))
	largoCD := binary.LittleEndian.Uint32(eocd[12:16])
	offCD := binary.LittleEndian.Uint32(eocd[16:20])
	// los centinelas del zip64 piden los campos de 64 bits que viven en su propio registro: este
	// lector no los sigue, y el archive/zip de respaldo sí
	if cuenta == 0xffff || largoCD == 0xffffffff || offCD == 0xffffffff {
		return nil, 0, 0, fmt.Errorf("el android.jar %s tiene un directorio central zip64", rutaJar)
	}
	if int64(offCD)+int64(largoCD) > tamano {
		return nil, 0, 0, fmt.Errorf("el directorio central del android.jar %s se sale del fichero", rutaJar)
	}
	cd := make([]byte, largoCD)
	if _, err := f.ReadAt(cd, int64(offCD)); err != nil {
		return nil, 0, 0, err
	}
	return cd, tamano, cuenta, nil
}

// parseaDirectorioCentral se queda con las entradas `.class` de los bytes del directorio central: su
// cabecera local, su tamaño comprimido, su CRC y su método. Es lo mismo que hace archive/zip, sin un
// *zip.File ni un campo extra por entrada, y por eso cuesta la mitad.
func parseaDirectorioCentral(cd []byte, cuenta int) (map[string]entradaJar, error) {
	parseosDelDirectorio++
	indice := make(map[string]entradaJar, cuenta)
	for p := 0; p < len(cd); {
		if p+46 > len(cd) || !bytes.Equal(cd[p:p+4], firmaEntradaDirectorio) {
			return nil, fmt.Errorf("directorio central roto en el byte %d", p)
		}
		metodo := binary.LittleEndian.Uint16(cd[p+10 : p+12])
		crc := binary.LittleEndian.Uint32(cd[p+16 : p+20])
		comprimido := binary.LittleEndian.Uint32(cd[p+20 : p+24])
		largoNombre := int(binary.LittleEndian.Uint16(cd[p+28 : p+30]))
		largoExtra := int(binary.LittleEndian.Uint16(cd[p+30 : p+32]))
		largoComentario := int(binary.LittleEndian.Uint16(cd[p+32 : p+34]))
		offset := binary.LittleEndian.Uint32(cd[p+42 : p+46])
		if p+46+largoNombre > len(cd) {
			return nil, fmt.Errorf("directorio central cortado en el byte %d", p)
		}
		nombre := string(cd[p+46 : p+46+largoNombre])
		if interno, ok := recortaClase(nombre); ok {
			if _, ya := indice[interno]; !ya {
				indice[interno] = entradaJar{cabecera: int64(offset), comprimido: comprimido, crc: crc, metodo: metodo}
			}
		}
		p += 46 + largoNombre + largoExtra + largoComentario
	}
	return indice, nil
}

// directorioCentralDelJar es la puerta de una sola llamada: lee el directorio central y lo parsea.
func directorioCentralDelJar(rutaJar string) (map[string]entradaJar, error) {
	cd, _, cuenta, err := leeDirectorioCentral(rutaJar)
	if err != nil {
		return nil, err
	}
	return parseaDirectorioCentral(cd, cuenta)
}

// recortaClase devuelve el nombre interno de una entrada `.class` del jar.
func recortaClase(nombre string) (string, bool) {
	const sufijo = ".class"
	if len(nombre) <= len(sufijo) || nombre[len(nombre)-len(sufijo):] != sufijo {
		return "", false
	}
	return nombre[:len(nombre)-len(sufijo)], true
}

// datosDeLaEntrada lee y descomprime una entrada con el offset del índice, sin archive/zip. Pasa por
// la cabecera local porque el directorio central da el offset de esa cabecera, no el de los datos.
func datosDeLaEntrada(archivo *os.File, e entradaJar) ([]byte, error) {
	var cabecera [30]byte
	if _, err := archivo.ReadAt(cabecera[:], e.cabecera); err != nil {
		return nil, err
	}
	if !bytes.Equal(cabecera[0:4], firmaCabeceraLocal) {
		return nil, fmt.Errorf("la cabecera local no está en el offset %d que dio el índice", e.cabecera)
	}
	largoNombre := int64(binary.LittleEndian.Uint16(cabecera[26:28]))
	largoExtra := int64(binary.LittleEndian.Uint16(cabecera[28:30]))
	crudo := make([]byte, e.comprimido)
	if _, err := archivo.ReadAt(crudo, e.cabecera+30+largoNombre+largoExtra); err != nil {
		return nil, err
	}
	var datos []byte
	switch e.metodo {
	case zip.Store:
		datos = crudo
	case zip.Deflate:
		zr := flate.NewReader(bytes.NewReader(crudo))
		var err error
		datos, err = io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("método de compresión %d, y el jar solo usa guardado o deflate", e.metodo)
	}
	if crc32.ChecksumIEEE(datos) != e.crc {
		return nil, fmt.Errorf("la entrada no cuadra con su CRC: el jar cambió y el índice no se enteró")
	}
	return datos, nil
}
