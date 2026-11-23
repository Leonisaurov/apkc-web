package main

// El marco de recursos del framework, ya parseado, entre pasadas.
//
// `check --proyecto` enlaza la R de res/ contra los ids del android.jar, y para eso descomprime y
// recorre resources.arsc (16 MB en el de API 34, §33) en CADA pasada aunque el jar no haya cambiado.
// El índice de I5 (jarindice.go) ya evita abrir el zip con archive/zip, pero no el coste del parseo:
// inflar la tabla y andar sus bloques cuesta ~140 ms medidos en wasm. Aquí el resultado del parseo se
// serializa en build/marco/ con la huella del jar y la versión de la forma en la cabecera; la pasada
// siguiente lo lee y no vuelve a inflar ni a recorrer el resources.arsc.
//
// La caché no es fuente de verdad. Si falta, si la huella no cuadra con el jar, si la marca es de
// otra versión o si el fichero está cortado o tiene basura, leerMarcoCacheado devuelve false y el
// llamador parsea el resources.arsc como siempre. Un fichero inválido no puede servir un marco a
// medias: la cabecera y los límites de cada campo se comprueban antes de devolver nada, y un fallo a
// mitad descarta el fichero entero.
//
// La forma lleva la versión en la marca (`apkc-marco-1`). Añadir o cambiar un campo obliga a subirla:
// una caché de otra marca se ignora, no se reinterpreta.
//
// La huella es la misma que usa el resto del incremental —tamaño del jar y CRC-32 de su directorio
// central—, no un SHA-1: el SHA-1 del directorio cuesta 8 ms por pasada en wasm y el CRC 1 ms, y aquí
// el digest no autentica, solo distingue contenido (la razón completa está en huellaDelJar).

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
)

const (
	// dirMarcoCacheado vive dentro de build/, como build/jarindice: `apkc clean` se lleva el
	// directorio entero y la caché no tiene por qué sobrevivirle.
	dirMarcoCacheado = "build/marco"

	// marcaMarcoCacheado dice de qué fichero se trata y con qué forma.
	marcaMarcoCacheado = "apkc-marco-1\n"
	largoMarcaMarco    = len(marcaMarcoCacheado)

	// cabeceraMarco: marca, sha1 de la ruta del jar, tamaño del jar (8) y CRC-32 de su directorio
	// central (4). Su forma es la misma que la del índice de I5, y es la huella la que decide si el
	// marco cacheado sigue valiendo.
	cabeceraMarco        = largoMarcaMarco + sha1.Size + 8 + 4
	campoRutaMarco       = largoMarcaMarco
	campoTamanoMarco     = campoRutaMarco + sha1.Size
	campoDirectorioMarco = campoTamanoMarco + 8
)

var (
	// directorioDeMarcoCacheado mueve build/marco cuando quien llama no trabaja desde la raíz del
	// proyecto. `check` recibe la raíz en `--proyecto` y no cambia su cwd, así que la fija aquí,
	// igual que hace con build/jarindice.
	directorioDeMarcoCacheado = ""

	// reutilizarMarcoCacheado se apaga en la sonda para medir el camino sin la caché, que es el
	// coste que `check` pagaba en cada pasada. En un build normal siempre está encendido.
	reutilizarMarcoCacheado = true

	// marcosDeCache cuenta los marcos servidos desde el fichero. Por fuera, «salió de
	// build/marco» y «volvió a parsear resources.arsc» se comportan igual; con esto la reutilización
	// se puede afirmar, no suponer.
	marcosDeCache int
)

// ficheroMarcoCacheado es la caché de un jar concreto. La ruta entra por su SHA-1 en el nombre y la
// huella del contenido va en la cabecera: dos jars no se pisan y el mismo jar con otros bytes no se
// sirve.
func ficheroMarcoCacheado(rutaJar string) string {
	digest := sha1.Sum([]byte(rutaJar))
	dir := dirMarcoCacheado
	if directorioDeMarcoCacheado != "" {
		dir = directorioDeMarcoCacheado
	}
	return filepath.Join(dir, hex.EncodeToString(digest[:])+".bin")
}

// leerMarcoCacheado devuelve el marco del jar solo si la caché es de este jar, de esta forma y de
// este contenido. Cualquier duda es false: el llamador parsea el resources.arsc, que es el camino que
// siempre da la respuesta correcta.
func leerMarcoCacheado(rutaJar string, tamano int64, directorio uint32) (*marcoRecursos, bool) {
	if !reutilizarMarcoCacheado {
		return nil, false
	}
	datos, err := os.ReadFile(ficheroMarcoCacheado(rutaJar))
	if err != nil || len(datos) < cabeceraMarco {
		return nil, false
	}
	if string(datos[:largoMarcaMarco]) != marcaMarcoCacheado {
		return nil, false
	}
	ruta := sha1.Sum([]byte(rutaJar))
	if !bytes.Equal(datos[campoRutaMarco:campoRutaMarco+sha1.Size], ruta[:]) {
		return nil, false
	}
	if int64(binary.LittleEndian.Uint64(datos[campoTamanoMarco:])) != tamano ||
		binary.LittleEndian.Uint32(datos[campoDirectorioMarco:]) != directorio {
		return nil, false
	}
	m, ok := decodificaMarco(datos[cabeceraMarco:])
	if !ok {
		return nil, false
	}
	marcosDeCache++
	return m, true
}

// escribirMarcoCacheado deja el marco en build/marco/. Es lo mejor que se puede hacer, no una
// obligación: sin build/ (una prueba fuera de un proyecto) o con el disco lleno se sigue compilando.
// No crea build/: el contrato de `check` es no dejar directorios nuevos en el árbol del proyecto.
func escribirMarcoCacheado(rutaJar string, tamano int64, directorio uint32, m *marcoRecursos) {
	dir := dirMarcoCacheado
	if directorioDeMarcoCacheado != "" {
		dir = directorioDeMarcoCacheado
	}
	if st, err := os.Stat(filepath.Dir(dir)); err != nil || !st.IsDir() {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	datos := serializaMarco(rutaJar, tamano, directorio, m)
	if datos == nil {
		return
	}
	// por un temporal y un rename: un build que muere a mitad no puede dejar una caché corta que la
	// vuelta siguiente lea como buena (la cabecera la cazaría, pero el rename no cuesta nada)
	destino := ficheroMarcoCacheado(rutaJar)
	tmp := destino + ".tmp"
	if err := os.WriteFile(tmp, datos, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, destino); err != nil {
		_ = os.Remove(tmp)
	}
}

// escritorDeMarco arma la forma binaria de la caché. Todo texto va con su longitud en u16 por delante
// y los mapas salen ordenados por clave: dos builds del mismo jar dejan el mismo fichero.
type escritorDeMarco struct {
	b  []byte
	ok bool
}

func (w *escritorDeMarco) texto(s string) {
	if !w.ok {
		return
	}
	if len(s) > 0xffff {
		w.ok = false
		return
	}
	w.b = append(w.b, byte(len(s)), byte(len(s)>>8))
	w.b = append(w.b, s...)
}

func (w *escritorDeMarco) u32(v uint32) {
	if !w.ok {
		return
	}
	var c [4]byte
	binary.LittleEndian.PutUint32(c[:], v)
	w.b = append(w.b, c[:]...)
}

func (w *escritorDeMarco) b1(v byte) {
	if !w.ok {
		return
	}
	w.b = append(w.b, v)
}

// serializaMarco devuelve nil si algún nombre no cabe en u16: en ese caso no se escribe caché y el
// llamador sigue con el parseo, en vez de guardar una forma que no se puede releer.
func serializaMarco(rutaJar string, tamano int64, directorio uint32, m *marcoRecursos) []byte {
	w := &escritorDeMarco{b: make([]byte, 0, 64<<10), ok: true}
	w.b = append(w.b, marcaMarcoCacheado...)
	ruta := sha1.Sum([]byte(rutaJar))
	w.b = append(w.b, ruta[:]...)
	var ocho [8]byte
	binary.LittleEndian.PutUint64(ocho[:], uint64(tamano))
	w.b = append(w.b, ocho[:]...)
	w.u32(directorio)

	w.texto(m.nombre)
	w.u32(uint32(len(m.tipos)))
	for _, t := range m.tipos {
		w.texto(t)
	}

	formatos := clavesOrdenadasDe(m.formatos)
	w.u32(uint32(len(formatos)))
	for _, k := range formatos {
		w.texto(k)
		w.u32(m.formatos[k])
	}

	atributos := make([]string, 0, len(m.valores))
	for k := range m.valores {
		atributos = append(atributos, k)
	}
	sort.Strings(atributos)
	w.u32(uint32(len(atributos)))
	for _, attr := range atributos {
		items := m.valores[attr]
		claves := make([]string, 0, len(items))
		for k := range items {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		w.texto(attr)
		w.u32(uint32(len(claves)))
		for _, k := range claves {
			v := items[k]
			w.texto(k)
			w.b1(v.tipo)
			w.u32(v.datos)
			w.texto(v.texto)
		}
	}

	ids := clavesOrdenadasDe(m.ids)
	w.u32(uint32(len(ids)))
	for _, k := range ids {
		w.texto(k)
		w.u32(m.ids[k])
	}
	if !w.ok {
		return nil
	}
	return w.b
}

// clavesOrdenadasDe devuelve las claves de un mapa de u32 ordenadas: el orden del mapa no es estable
// y el fichero tiene que serlo para poder comparar dos builds.
func clavesOrdenadasDe[V any](m map[string]V) []string {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	return claves
}

// decodificaMarco reconstruye el marco de los bytes que van tras la cabecera. Un cuerpo que no
// cuadra byte a byte con la forma se rechaza entero: no se devuelve un marco a medias.
func decodificaMarco(cuerpo []byte) (*marcoRecursos, bool) {
	// un solo string para todo el cuerpo: cada nombre es un trozo suyo y no se copia al entrar en
	// los mapas, que es la mitad del coste de reconstruirlos
	texto := string(cuerpo)
	p := 0
	leeU16 := func() (uint16, bool) {
		if p+2 > len(cuerpo) {
			return 0, false
		}
		v := binary.LittleEndian.Uint16(cuerpo[p : p+2])
		p += 2
		return v, true
	}
	leeU32 := func() (uint32, bool) {
		if p+4 > len(cuerpo) {
			return 0, false
		}
		v := binary.LittleEndian.Uint32(cuerpo[p : p+4])
		p += 4
		return v, true
	}
	leeByte := func() (byte, bool) {
		if p+1 > len(cuerpo) {
			return 0, false
		}
		v := cuerpo[p]
		p++
		return v, true
	}
	leeTexto := func() (string, bool) {
		n, ok := leeU16()
		if !ok || p+int(n) > len(cuerpo) {
			return "", false
		}
		s := texto[p : p+int(n)]
		p += int(n)
		return s, true
	}
	// una cantidad no puede pedir más registros que bytes quedan: sin esta cota, un número corrupto
	// dispara una reserva enorme antes de que cualquier comprobación de longitud lo note
	cabe := func(cantidad uint32, minimo uint64) bool {
		return uint64(cantidad) <= uint64(len(cuerpo)-p)/minimo
	}

	m := &marcoRecursos{
		ids:      map[string]uint32{},
		porId:    map[uint32]string{},
		valores:  map[string]map[string]valorEscribir{},
		formatos: map[string]uint32{},
	}
	nombre, ok := leeTexto()
	if !ok || nombre == "" {
		return nil, false
	}
	m.nombre = nombre

	cantidad, ok := leeU32()
	if !ok || !cabe(cantidad, 2) {
		return nil, false
	}
	m.tipos = make([]string, int(cantidad))
	for i := range m.tipos {
		if m.tipos[i], ok = leeTexto(); !ok {
			return nil, false
		}
	}

	if cantidad, ok = leeU32(); !ok || !cabe(cantidad, 6) {
		return nil, false
	}
	for i := uint32(0); i < cantidad; i++ {
		k, ok := leeTexto()
		if !ok {
			return nil, false
		}
		v, ok := leeU32()
		if !ok {
			return nil, false
		}
		m.formatos[k] = v
	}

	if cantidad, ok = leeU32(); !ok || !cabe(cantidad, 6) {
		return nil, false
	}
	for i := uint32(0); i < cantidad; i++ {
		attr, ok := leeTexto()
		if !ok {
			return nil, false
		}
		cuenta, ok := leeU32()
		if !ok || !cabe(cuenta, 7) {
			return nil, false
		}
		items := make(map[string]valorEscribir, int(cuenta))
		for j := uint32(0); j < cuenta; j++ {
			k, ok := leeTexto()
			if !ok {
				return nil, false
			}
			tipo, ok := leeByte()
			if !ok {
				return nil, false
			}
			datos, ok := leeU32()
			if !ok {
				return nil, false
			}
			txt, ok := leeTexto()
			if !ok {
				return nil, false
			}
			items[k] = valorEscribir{tipo: tipo, datos: datos, texto: txt}
		}
		m.valores[attr] = items
	}

	if cantidad, ok = leeU32(); !ok || !cabe(cantidad, 6) {
		return nil, false
	}
	for i := uint32(0); i < cantidad; i++ {
		k, ok := leeTexto()
		if !ok {
			return nil, false
		}
		id, ok := leeU32()
		if !ok {
			return nil, false
		}
		// el mapa inverso se reconstruye aquí: es el mismo que armaba leeBloque entrada a entrada
		if _, visto := m.ids[k]; !visto {
			m.ids[k] = id
			m.porId[id] = k
		}
	}
	if p != len(cuerpo) || len(m.ids) == 0 {
		return nil, false
	}
	return m, true
}
