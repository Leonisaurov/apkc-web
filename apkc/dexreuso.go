package main

// I4, dexado por clase: reutilizar el trabajo caro de las clases que no cambiaron.
//
// El `classes.dex` es un ensamblado con offsets absolutos —cambiar una instrucción de una clase
// mueve todo lo que va detrás—, así que no se puede parchear el fichero de la vuelta anterior. Lo
// que sí se puede reutilizar es lo que se calcula **por método** y no nombra ningún offset: el
// `code_item` con sus palabras dalvik y el `debug_info_item` con sus líneas. Eso es además donde se
// va el tiempo de la etapa: con `-cpuprofile` sobre las 244 clases del proyecto grande de
// `banco/gen-proyecto-grande.py`, `areaCodigo` acumula 200 de las 480 muestras de CPU (el 41 %) y
// `collectCode` 60 más.
//
// La condición de validez es una sola y no se negocia: **las tablas ordenadas tienen que ser las
// mismas**. Cada operando de una instrucción traducida es un índice de `string_ids`, `type_ids`,
// `proto_ids`, `field_ids` o `method_ids`, así que basta con que entre o salga una cadena para que
// los índices de todas las demás clases se muevan. Por eso la caché guarda el resumen de las cinco
// tablas ya ordenadas (`huellaTablas`) y, si no coincide con el de esta pasada, se tira entera: esa
// vuelta cuesta lo mismo que un dexado completo, y sale idéntica porque el camino es el mismo.
//
// Lo que se guarda, por método, se indexa con el SHA-1 del fichero `.class` de su clase y su
// nombre y descriptor. El fichero entero es la clave, no el método: el traductor también mira la
// superclase, los métodos privados de la propia clase y los sitios de `invokedynamic` que la clase
// declara, y todo eso cabe en el mismo resumen sin tener que enumerarlo. Las clases sintéticas de
// las lambdas no tienen fichero detrás y quedan fuera de la caché por construcción.
//
// La caché vive en `build/`, al lado del dex, y por tanto `apkc clean` la borra con él. No entra en
// el apk: el empaquetado nombra sus rutas una a una.

import (
	"crypto/sha1"
	"encoding/binary"
	"os"
)

const (
	cacheDexFichero = "build/dexcache.bin"
	// la marca lleva versión: un cambio en lo que se guarda tiene que dejar ilegible lo anterior,
	// porque una entrada vieja leída con otro formato no es una entrada, es basura
	cacheDexMarca = "apkc-dex-reuso-1\n"
)

// cacheDex es la caché de una pasada: lo leído de la vuelta anterior, lo que esta pasada vuelve a
// calcular y los contadores que el informe necesita. `nuevoCodigo` y `nuevoDepuracion` son lo único
// que se escribe al final, de modo que la caché no crece con las clases que ya no están.
type cacheDex struct {
	huella     [20]byte
	codigo     map[[20]byte][]byte
	depuracion map[[20]byte][]byte

	nuevoCodigo     map[[20]byte][]byte
	nuevoDepuracion map[[20]byte][]byte

	reutilizados int
	total        int
}

func nuevoCacheDex() *cacheDex {
	return &cacheDex{
		codigo:          map[[20]byte][]byte{},
		depuracion:      map[[20]byte][]byte{},
		nuevoCodigo:     map[[20]byte][]byte{},
		nuevoDepuracion: map[[20]byte][]byte{},
	}
}

// cargaCacheDex lee la caché de la vuelta anterior. Una caché que no se puede leer no es un motivo
// para no dexar: se devuelve una vacía, la pasada cuesta lo de siempre y la siguiente la repone.
func cargaCacheDex() *cacheDex {
	c := nuevoCacheDex()
	b, err := os.ReadFile(cacheDexFichero)
	if err != nil {
		return c
	}
	if len(b) < len(cacheDexMarca)+20 || string(b[:len(cacheDexMarca)]) != cacheDexMarca {
		LogW("build", cacheDexFichero, "no es una caché de esta versión: se dexan las clases enteras")
		return c
	}
	p := len(cacheDexMarca)
	copy(c.huella[:], b[p:p+20])
	p += 20
	leer := func(destino map[[20]byte][]byte) bool {
		if p+4 > len(b) {
			return false
		}
		cuantas := int(binary.LittleEndian.Uint32(b[p:]))
		p += 4
		for i := 0; i < cuantas; i++ {
			if p+24 > len(b) {
				return false
			}
			var clave [20]byte
			copy(clave[:], b[p:p+20])
			p += 20
			n := int(binary.LittleEndian.Uint32(b[p:]))
			p += 4
			if n < 0 || p+n > len(b) {
				return false
			}
			destino[clave] = b[p : p+n : p+n]
			p += n
		}
		return true
	}
	if !leer(c.codigo) || !leer(c.depuracion) {
		LogW("build", cacheDexFichero, "está truncada: se dexan las clases enteras")
		return nuevoCacheDex()
	}
	return c
}

// activa cierra la caché contra las tablas de esta pasada: si el resumen de las cinco tablas
// ordenadas no es el mismo, ningún índice de lo guardado sirve y se tira todo.
func (c *cacheDex) activa(t *dexTables) {
	huella := huellaTablas(t)
	if huella == c.huella {
		return
	}
	if len(c.codigo) > 0 {
		LogI("build", "las tablas del dex cambiaron: se dexan las clases enteras")
	}
	c.codigo = map[[20]byte][]byte{}
	c.depuracion = map[[20]byte][]byte{}
	c.huella = huella
}

// reusaCodigo devuelve el `code_item` guardado de un método, con `debug_info_off` a cero (lo
// parchea el ensamblado, que es el único que sabe dónde cae el área de depuración). Solo cuenta
// como reutilizado si también está guardada su depuración: el área de depuración se arma a partir
// de la correspondencia pc JVM -> unidad dalvik que sale de traducir el cuerpo, y con el cuerpo
// reutilizado esa correspondencia no existe. Las dos entradas se escriben juntas, así que la única
// forma de tener una sin la otra es una caché de otra versión, y esa no se lee.
func (c *cacheDex) reusaCodigo(clave [20]byte) ([]byte, bool) {
	codigo, ok := c.codigo[clave]
	if !ok {
		return nil, false
	}
	depuracion, ok := c.depuracion[clave]
	if !ok {
		return nil, false
	}
	c.nuevoCodigo[clave] = codigo
	c.nuevoDepuracion[clave] = depuracion
	c.reutilizados++
	return codigo, true
}

// guardaCodigo apunta el `code_item` que esta pasada tuvo que traducir.
func (c *cacheDex) guardaCodigo(clave [20]byte, blob []byte) {
	c.nuevoCodigo[clave] = blob
}

// reusaDepuracion devuelve el `debug_info_item` guardado de un método. Un item vacío también se
// guarda: distingue «este método no trae líneas» de «no lo he mirado», y sin eso la vuelta
// siguiente volvería a armarlo para saberlo.
func (c *cacheDex) reusaDepuracion(clave [20]byte) ([]byte, bool) {
	b, ok := c.depuracion[clave]
	if ok {
		c.nuevoDepuracion[clave] = b
	}
	return b, ok
}

func (c *cacheDex) guardaDepuracion(clave [20]byte, blob []byte) {
	c.nuevoDepuracion[clave] = blob
}

// guarda escribe la caché de esta pasada: solo lo que esta pasada tocó. Un fallo al escribir no
// puede tumbar el build —la caché es un atajo, no un producto— y se queda en un aviso.
func (c *cacheDex) guarda() {
	b := make([]byte, 0, len(cacheDexMarca)+20+8+len(c.nuevoCodigo)*64)
	b = append(b, cacheDexMarca...)
	b = append(b, c.huella[:]...)
	escribe := func(mapa map[[20]byte][]byte) {
		var n [4]byte
		binary.LittleEndian.PutUint32(n[:], uint32(len(mapa)))
		b = append(b, n[:]...)
		for clave, blob := range mapa {
			b = append(b, clave[:]...)
			binary.LittleEndian.PutUint32(n[:], uint32(len(blob)))
			b = append(b, n[:]...)
			b = append(b, blob...)
		}
	}
	escribe(c.nuevoCodigo)
	escribe(c.nuevoDepuracion)
	if err := os.WriteFile(cacheDexFichero, b, 0644); err != nil {
		LogW("build", "no se pudo escribir", cacheDexFichero, ":", err.Error())
	}
}

// claveMetodo resume en un solo SHA-1 lo que un `code_item` depende de su clase: el fichero
// `.class` entero y el nombre y descriptor del método. Devuelve false para una clase sin fichero
// detrás (las sintéticas que añade la expansión de lambdas), que no se cachea.
func claveMetodo(cf classFull, m classMethod) ([20]byte, bool) {
	var cero [20]byte
	if cf.huella == cero {
		return cero, false
	}
	h := sha1.New()
	h.Write(cf.huella[:])
	h.Write([]byte(m.Name))
	h.Write([]byte{0})
	h.Write([]byte(m.Desc))
	var clave [20]byte
	copy(clave[:], h.Sum(nil))
	return clave, true
}

// huellaTablas resume las cinco tablas ya ordenadas: es lo que decide si un índice guardado sigue
// nombrando lo mismo. Se escribe con la longitud delante de cada cadena para que dos tablas
// distintas no puedan dar el mismo flujo de bytes.
func huellaTablas(t *dexTables) [20]byte {
	h := sha1.New()
	var n [4]byte
	cadena := func(s string) {
		binary.LittleEndian.PutUint32(n[:], uint32(len(s)))
		h.Write(n[:])
		h.Write([]byte(s))
	}
	seccion := func(etiqueta byte, cuantas int) {
		h.Write([]byte{etiqueta})
		binary.LittleEndian.PutUint32(n[:], uint32(cuantas))
		h.Write(n[:])
	}
	seccion('s', len(t.strings))
	for _, s := range t.strings {
		cadena(s)
	}
	seccion('t', len(t.types))
	for _, s := range t.types {
		cadena(s)
	}
	seccion('p', len(t.protos))
	for _, k := range t.protos {
		cadena(k.shorty)
		cadena(k.ret)
		cadena(k.params)
	}
	seccion('f', len(t.fields))
	for _, k := range t.fields {
		cadena(k.owner)
		cadena(k.typeOf)
		cadena(k.name)
	}
	seccion('m', len(t.methods))
	for _, k := range t.methods {
		cadena(k.owner)
		cadena(k.name)
		cadena(k.proto.shorty)
		cadena(k.proto.ret)
		cadena(k.proto.params)
	}
	var huella [20]byte
	copy(huella[:], h.Sum(nil))
	return huella
}

// resumenCacheDex es lo que el informe del build dice de la caché, y lo que la prueba mira para
// saber que la vuelta reutilizó de verdad en vez de acertar por casualidad.
type resumenCacheDex struct {
	reutilizados int
	total        int
}

func (c *cacheDex) resumen() resumenCacheDex {
	if c == nil {
		return resumenCacheDex{}
	}
	return resumenCacheDex{reutilizados: c.reutilizados, total: c.total}
}
