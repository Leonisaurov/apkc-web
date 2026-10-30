package main

// La tabla de recursos del framework, leída de android.jar. Es lo que hoy presta aapt2 cuando resuelve
// `android:layout_width` o `@style/TextAppearance.Large`, y sin ello este fork no puede enlazar un
// layout que no esté entre los doce atributos del proyecto del studio.
//
// Medido sobre el android.jar de este teléfono (API 34, docs/evidencia.md 33): la tabla declara
// cuatro paquetes, el pool global mide 11.009.160 bytes con 152.615 cadenas y 1.307 estilos —se salta
// por su tamaño declarado, porque materializarlas costaría 11 MB de entradas que nadie va a
// preguntar—, y el paquete `android` (id 1) trae cabecera de 288 con 23 nombres de tipo y 12.877
// claves. Sus bloques RES_TABLE_TYPE van DENSOS: el de `attr` mide flags 0x00, 1672 entradas y
// entriesStart 6772 = 84 + 4·1672, con offsets u32 relativas a entriesStart, igual que en el
// resources.arsc de una app.
//
// El id de recurso es paquete<<24 | tipo<<16 | INDICE de la entrada, que es también la fórmula con la
// que aapt2 imprime su volcado: 0x01010000 attr/theme, 0x0101014f attr/text, 0x0103012b
// style/Theme.DeviceDefault.Light.

import (
	"archive/zip"
	"fmt"
	"io"
	"sort"
	"strings"
)

// paqueteDelFramework: el id del paquete `android`, que es el unico que se lee de la tabla del jar.
const paqueteDelFramework = uint32(1)

// claveSinEntrada: en una tabla densa un indice puede no tener recurso; aapt2 escribe entonces la
// clave a -1. Medido en el bloque de attr del android.jar de API 34: el offset que no apunta a ninguna
// entrada vale 0xFFFFFFFF (la entrada 616 del tipo attr lo lleva), y esa es la senal que usa el
// framework para decir "este indice no existe en esta configuracion".
const (
	claveSinEntrada = uint32(0xffffffff)
	entradaAusente  = uint32(0xffffffff)
)

// marcoRecursos es el mapa nombre -> id del paquete android de un resources.arsc de framework.
type marcoRecursos struct {
	nombre string
	// ids indexa por "tipo/nombre", como lo imprime `aapt2 dump resources`: "attr/text", "style/…".
	ids map[string]uint32
	// tipos son los nombres de tipo en su orden, que es el que da el id de tipo (posicion + 1).
	tipos []string
}

// idDe da el id de recurso de un nombre dentro de un tipo.
func (m *marcoRecursos) idDe(tipo, nombre string) (uint32, error) {
	id, ok := m.ids[tipo+"/"+nombre]
	if !ok {
		return 0, fmt.Errorf("el framework no tiene %s/%s: aapt2 lo resolveria y aqui no esta", tipo, nombre)
	}
	return id, nil
}

// idDeAtributo resuelve el nombre de un atributo sin prefijo (`layout_width`), que es lo que hace
// falta para el XML_RESOURCE_MAP del AXML y para los `name="android:…" de un estilo.
func (m *marcoRecursos) idDeAtributo(nombre string) (uint32, error) {
	return m.idDe("attr", nombre)
}

// nombresDe devuelve las claves conocidas de un tipo, ordenadas, para los mensajes y para el vuelco
// de comprobacion contra el oraculo.
func (m *marcoRecursos) nombresDe(tipo string) []string {
	prefijo := tipo + "/"
	out := []string{}
	for k := range m.ids {
		if strings.HasPrefix(k, prefijo) {
			out = append(out, strings.TrimPrefix(k, prefijo))
		}
	}
	sort.Strings(out)
	return out
}

// leeMarco anda un resources.arsc de framework y saca el mapa del paquete android. No materializa el
// pool global: en el jar de API 34 son 152.615 cadenas en 11 MB, y lo unico que se necesita son los
// nombres de tipo y las claves del propio paquete.
func leeMarco(arsc []byte) (*marcoRecursos, error) {
	// el tamano se comprueba ANTES de leer: el mensaje de error leia u16re sobre el fichero corto y
	// eso es un panic, no un rechazo
	if len(arsc) < 12 {
		return nil, fmt.Errorf("una tabla de recursos mide al menos 12 bytes y aqui hay %d", len(arsc))
	}
	if u16re(arsc, 0) != trozoTabla {
		return nil, fmt.Errorf("no es una tabla de recursos: el primer trozo es %#04x", u16re(arsc, 0))
	}
	if int(u32re(arsc, 4)) > len(arsc) {
		return nil, fmt.Errorf("la tabla declara %d bytes y el fichero tiene %d", u32re(arsc, 4), len(arsc))
	}
	m := &marcoRecursos{ids: map[string]uint32{}}
	off := 12
	if off+8 > len(arsc) || u16re(arsc, off) != trozoPool {
		return nil, fmt.Errorf("tras la cabecera se esperaba el pool global, en %d", off)
	}
	off += int(u32re(arsc, off+4))

	encontrado := false
	for off+8 <= len(arsc) {
		tipo := u16re(arsc, off)
		hs := int(u16re(arsc, off+2))
		size := int(u32re(arsc, off+4))
		if size <= 0 || off+size > len(arsc) {
			return m, fmt.Errorf("el trozo %#04x en %d dice medir %d y el fichero termina en %d", tipo, off, size, len(arsc))
		}
		if tipo != trozoPaquete {
			// tras los paquetes pueden venir bibliotecas de recursos (0x0203) y otros; aqui se
			// corta el recorrido porque solo se busca el paquete android
			break
		}
		if hs < 288 {
			return m, fmt.Errorf("cabecera de paquete de %d bytes, medida en 288", hs)
		}
		if u32re(arsc, off+8) == paqueteDelFramework {
			if err := m.leePaquete(arsc, off, hs, size); err != nil {
				return m, err
			}
			// medido: el android.jar de API 34 trae CUATRO paquetes con id 1 y los cuatro llamados
			// `android` —el primero, de 5.355.916 bytes con 23 tipos y 12.877 claves, y despues tres
			// de configuracion de 255, 223 y 221 tipos—. Los ids que imprime `aapt2 dump resources`
			// son los del primero, asi que se corta aqui en vez de pisarlos con los siguientes.
			encontrado = true
			break
		}
		off += size
	}
	if !encontrado {
		return m, fmt.Errorf("ningun paquete del fichero lleva id %d", paqueteDelFramework)
	}
	if len(m.ids) == 0 {
		return m, fmt.Errorf("el paquete android no dejo ninguna entrada leida")
	}
	return m, nil
}

// leePaquete lee los dos pools de nombres del paquete y despues sus bloques de tipo. Las claves se
// toman por INDICE de entrada dentro del bloque, que es la mitad baja del id de recurso.
func (m *marcoRecursos) leePaquete(arsc []byte, off, hs, size int) error {
	if err := m.nombreDelPaquete(arsc, off); err != nil {
		return err
	}
	tipos, err := leePool(arsc, off+int(u32re(arsc, off+268)))
	if err != nil {
		return fmt.Errorf("nombres de tipo: %w", err)
	}
	claves, err := leePool(arsc, off+int(u32re(arsc, off+276)))
	if err != nil {
		return fmt.Errorf("claves: %w", err)
	}
	if claves.estilos != 0 || tipos.estilos != 0 {
		return fmt.Errorf("los pools de nombres del framework traen %d y %d estilos, y aqui solo se leen pools sin ellos",
			tipos.estilos, claves.estilos)
	}
	m.tipos = tipos.cadenas

	// Medido en el android.jar de API 34: un tipo no ocupa un bloque, ocupa muchos —uno por
	// configuracion— y solo uno de ellos cubre todo el espacio de ids, que es el que trae tantas
	// entradas como anuncia su RES_TABLE_TYPE_SPEC (el de `string` tiene uno de 2492 entradas y
	// despues bloques de 1, 2, 3, 4, 9… para las otras configuraciones, esos con flags 0x01). Los
	// indices de un bloque parcial no son el id de la entrada, asi que se ignora: del spec se guarda
	// cuantas entradas declaraba y del tipo se lee el primero cuya cuenta cuadre con esa cifra.
	cuentaDelSpec := map[byte]uint32{}
	leido := map[byte]bool{}

	p := off + hs
	for p+8 <= off+size {
		t := u16re(arsc, p)
		phs := int(u16re(arsc, p+2))
		psize := int(u32re(arsc, p+4))
		if psize <= 0 || p+psize > off+size {
			return fmt.Errorf("el trozo %#04x del paquete en %d dice medir %d", t, p, psize)
		}
		switch t {
		case trozoSpec:
			cuentaDelSpec[arsc[p+8]] = u32re(arsc, p+12)
		case trozoTipo:
			idTipo := arsc[p+8]
			if leido[idTipo] || u32re(arsc, p+12) != cuentaDelSpec[idTipo] {
				break
			}
			leido[idTipo] = true
			if err := m.leeBloque(arsc, p, phs, psize, claves.cadenas); err != nil {
				return err
			}
		}
		p += psize
	}
	if len(leido) == 0 {
		return fmt.Errorf("ningún bloque de tipo del paquete android cubrió el espacio de entradas de su spec")
	}
	return nil
}

// nombreDelPaquete saca el nombre UTF-16 de 128 unidades que va despues del id.
func (m *marcoRecursos) nombreDelPaquete(arsc []byte, off int) error {
	sb := strings.Builder{}
	for i := 0; i < 128; i++ {
		c := u16re(arsc, off+12+2*i)
		if c == 0 {
			break
		}
		if c < 0x800 {
			sb.WriteRune(rune(c))
			continue
		}
		return fmt.Errorf("el nombre del paquete lleva un caracter de escape UTF-16, que aqui no se resuelve")
	}
	m.nombre = sb.String()
	return nil
}

// leeBloque anda un RES_TABLE_TYPE denso y apunta cada entrada con nombre. El bit de tabla rala
// (flags 0x02) esta medido en el android.jar de este telefono como 0 en el bloque de attr: si algun
// dia aparece, se dice en vez de adivinar.
func (m *marcoRecursos) leeBloque(arsc []byte, off, hs, size int, claves []string) error {
	idTipo := arsc[off+8]
	flags := arsc[off+9]
	cuenta := int(u32re(arsc, off+12))
	entriesStart := int(u32re(arsc, off+16))
	if flags&0x02 != 0 {
		return fmt.Errorf("el tipo %d viene en tabla rala (flags %#02x): la forma no esta medida en este telefono", idTipo, flags)
	}
	if idTipo == 0 || int(idTipo) > len(m.tipos) {
		return fmt.Errorf("el bloque de tipo %d no tiene nombre entre %d", idTipo, len(m.tipos))
	}
	if hs+4*cuenta > entriesStart || entriesStart+8 > off+size {
		return fmt.Errorf("el bloque del tipo %d: cabecera %d, %d offsets y entriesStart %d no caben en %d bytes",
			idTipo, hs, cuenta, entriesStart, size)
	}
	nombre := m.tipos[int(idTipo)-1]
	for i := 0; i < cuenta; i++ {
		o := int(u32re(arsc, off+hs+4*i))
		if uint32(o) == entradaAusente {
			// medido en el bloque de attr del framework: el offset de la entrada 616 vale
			// 0xFFFFFFFF, que dice que ese indice no tiene recurso en esta configuracion. El id
			// sigue siendo el indice, asi que se salta la entrada y no la numeracion
			continue
		}
		e := off + entriesStart + o
		if e+8 > off+size {
			return fmt.Errorf("la entrada %d del tipo %s cae fuera del bloque", i, nombre)
		}
		clave := u32re(arsc, e+4)
		if clave == claveSinEntrada {
			continue
		}
		if int(clave) >= len(claves) {
			return fmt.Errorf("la entrada %d del tipo %s nombra la clave %d y hay %d", i, nombre, clave, len(claves))
		}
		k := nombre + "/" + claves[int(clave)]
		if _, visto := m.ids[k]; !visto {
			m.ids[k] = paqueteDelFramework<<24 | uint32(idTipo)<<16 | uint32(i)
		}
	}
	return nil
}

// marcoDeJar saca el resources.arsc de un android.jar y lo lee. El jar es un zip; la entrada puede
// venir comprimida, asi que se descomprime a memoria (16 MB en el de API 34, medido).
func marcoDeJar(abrir func(nombre string) (io.ReadCloser, error), nombre string) (*marcoRecursos, error) {
	rc, err := abrir(nombre)
	if err != nil {
		return nil, fmt.Errorf("no se puede abrir %s: %w", nombre, err)
	}
	defer rc.Close()
	arsc, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return leeMarco(arsc)
}

// marcoDelAndroidJar es la puerta normal: abre el jar con archive/zip y busca resources.arsc dentro.
func marcoDelAndroidJar(ruta string) (*marcoRecursos, error) {
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		return nil, fmt.Errorf("el android.jar %s no abre como zip: %w", ruta, err)
	}
	defer zr.Close()
	abrir := func(nombre string) (io.ReadCloser, error) {
		for _, f := range zr.File {
			if f.Name == nombre {
				return f.Open()
			}
		}
		return nil, fmt.Errorf("no hay %s dentro del jar", nombre)
	}
	return marcoDeJar(abrir, "resources.arsc")
}
