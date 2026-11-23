package main

// El ensamblador del .aab que no necesita aapt2 ni bundletool. Toma lo que deja el enlazador
// Go en build/ — AndroidManifest.xml y los layouts en AXML, resources.arsc, classes.dex y
// el arbol res/ — y escribe los dos mensajes protobuf que aapt2 --proto-format ponía en el
// bundle.zip: manifest/AndroidManifest.xml (XmlNode) y resources.pb (ResourceTable).
//
// Medido antes de escribir esta pieza (ver §128 y el experimento del §127): un .aab de
// referencia validado por bundletool 1.18.3 con rc 0, y un ResourceTable mínimo con solo
// el paquete y una entrada FileReference que también da rc 0. Lo que bundletool exige y
// este escritor cumple es: BundleConfig.pb, el prefijo base/, un XmlNode con elemento en
// base/manifest/AndroidManifest.xml y una referencia desde resources.pb por cada fichero
// bajo base/res/. No se fingen campos que el enlazador Go no produce (source_pool,
// tool_fingerprint, compiled_item de cada atributo del fuente): se omiten y el bundle
// sigue siendo válido para el oráculo.

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// elemProto es un elemento del XmlNode mientras se apila el recorrido de los nodos AXML.
type elemProto struct {
	ns     []declaracionNS
	uri    string
	nombre string
	linea  int
	attrs  [][]byte
	hijos  [][]byte
}

func (e *elemProto) construir() []byte {
	var piezas [][]byte
	for _, n := range e.ns {
		dec := pbUnir(pbCadena(1, n.prefijo), pbCadena(2, n.uri), campoFuentePB(3, n.linea))
		piezas = append(piezas, pbMensaje(1, dec)) // XmlElement.namespace_declaration
	}
	if e.uri != "" {
		piezas = append(piezas, pbCadena(2, e.uri))
	}
	piezas = append(piezas, pbCadena(3, e.nombre))
	for _, a := range e.attrs {
		piezas = append(piezas, pbMensaje(4, a)) // XmlElement.attribute
	}
	for _, h := range e.hijos {
		piezas = append(piezas, pbMensaje(5, h)) // XmlElement.child
	}
	return pbUnir(piezas...)
}

// protoXmlDeAXML convierte un fichero AXML del enlazador Go en el XmlNode que lleva el .aab.
// El AXML ya trae el mapa de ids de atributo y los valores tipados: solo hay que reordenarlos
// en el mensaje de proto. `nombres` resuelve las referencias @tipo/entrada para el texto legible.
func protoXmlDeAXML(datos []byte, nombres map[uint32]string) ([]byte, error) {
	a, err := leeAXML(datos)
	if err != nil {
		return nil, fmt.Errorf("el AXML no se pudo leer: %w", err)
	}
	if len(a.nodos) == 0 {
		return nil, fmt.Errorf("el AXML no tiene nodos")
	}
	var raiz []byte
	pila := []*elemProto{}
	nsPendientes := []declaracionNS{}
	for _, n := range a.nodos {
		switch n.tipo {
		case trozoInicioNS:
			nsPendientes = append(nsPendientes, declaracionNS{
				prefijo: a.nombreDe(n.ns), uri: a.nombreDe(n.nombre), linea: n.linea,
			})
		case trozoFinNS:
			// El proto solo guarda la declaración en el elemento que la abre.
		case trozoInicioEtiqueta:
			e := &elemProto{ns: nsPendientes, nombre: a.nombreDe(n.nombre), linea: n.linea}
			nsPendientes = nil
			if n.ns >= 0 {
				e.uri = a.nombreDe(n.ns)
			}
			for _, at := range n.attrs {
				uri := ""
				if at.ns >= 0 {
					uri = a.nombreDe(at.ns)
				}
				nombre := a.nombreDe(at.nombre)
				texto := ""
				if at.raw >= 0 {
					texto = a.nombreDe(at.raw)
				}
				var id uint32
				if int(at.nombre) >= 0 && int(at.nombre) < len(a.idsMapa) {
					id = a.idsMapa[at.nombre]
				}
				piezas := [][]byte{}
				if uri != "" {
					piezas = append(piezas, pbCadena(1, uri))
				}
				piezas = append(piezas, pbCadena(2, nombre),
					pbCadena(3, textoDeValor(at.valor.tipo, at.valor.datos, texto, nombres)),
					campoFuentePB(4, n.linea))
				if id != 0 {
					piezas = append(piezas, pbU32(5, id))
				}
				// aapt2 no escribe compiled_item para los atributos de cadena: el value ya
				// basta. Para el resto, el Res_value del AXML da el Item completo.
				if at.valor.tipo != 0x03 {
					piezas = append(piezas, pbMensaje(6, itemDeValor(at.valor.tipo, at.valor.datos, texto, nombres)))
				}
				e.attrs = append(e.attrs, pbUnir(piezas...))
			}
			if len(pila) > 0 {
				pila[len(pila)-1].hijos = append(pila[len(pila)-1].hijos, e.construir())
			} else {
				raiz = e.construir()
			}
			pila = append(pila, e)
		case trozoFinEtik:
			if len(pila) > 0 {
				pila = pila[:len(pila)-1]
			}
		}
	}
	if len(pila) != 0 {
		return nil, fmt.Errorf("%d elementos quedaron sin cerrar", len(pila))
	}
	if len(raiz) == 0 {
		return nil, fmt.Errorf("el AXML no tiene elemento raíz")
	}
	return pbUnir(pbMensaje(1, raiz), fuentePB(1)), nil
}

// tablaPBDeARSC convive el resources.arsc del enlazador Go en el ResourceTable del .aab.
// Las claves se numeran como en R.java: el id de entrada es su posición en el bloque base,
// ordenado por nombre, y el id de recurso es 0xPPTTEEEE. `ficheros` son las rutas relativas
// del arbol res/ (por ejemplo "layout/main.xml"), que deben aparecer referenciadas.
func tablaPBDeARSC(datos []byte, ficheros []string) ([]byte, error) {
	tr, err := leeTabla(datos)
	if err != nil {
		return nil, fmt.Errorf("el resources.arsc no se pudo leer: %w", err)
	}
	if len(tr.nombresTipo) == 0 || len(tr.bloques) == 0 {
		return nil, fmt.Errorf("la tabla no declara tipos ni entradas")
	}

	tipos := map[byte]*tipoProto{}
	referenciados := map[string]bool{}
	for _, bl := range tr.bloques {
		t := tipos[bl.id]
		if t == nil {
			if int(bl.id) < 1 || int(bl.id) > len(tr.nombresTipo) {
				return nil, fmt.Errorf("el tipo %d no tiene nombre", bl.id)
			}
			t = &tipoProto{id: bl.id, nombre: tr.nombresTipo[int(bl.id)-1], entradas: map[int]*entradaProto{}}
			tipos[bl.id] = t
		}
		config := configPB(bl.config)
		for j, en := range bl.entradas {
			if en.clave < 0 || int(en.clave) >= len(tr.claves) {
				return nil, fmt.Errorf("la entrada %d del tipo %s nombra la clave %d", j, bl.nombre, en.clave)
			}
			nombre := tr.claves[en.clave]
			e := t.entradas[j]
			if e == nil {
				e = &entradaProto{id: j, nombre: nombre}
				t.entradas[j] = e
			}
			item := []byte{}
			switch {
			case en.compleja:
				// Un estilo o un attr: el id debe existir aunque el valor compuesto no se
				// haya traducido; un Item.id reserva el número sin inventar un contenido.
				item = pbMensaje(6) // Item.id
			case en.valor.tipo == 0x03:
				texto := ""
				if int(en.valor.datos) < len(tr.global.cadenas) {
					texto = tr.global.cadenas[en.valor.datos]
				}
				if strings.HasPrefix(texto, "res/") {
					item = fileReferencePB(texto)
					referenciados[texto] = true
				} else {
					item = itemDeValor(en.valor.tipo, en.valor.datos, texto, nil)
				}
			default:
				item = itemDeValor(en.valor.tipo, en.valor.datos, "", nil)
			}
			e.valores = append(e.valores, configValuePB(config, item))
		}
	}

	// Las referencias @id y @style del AXML se resuelven con los nombres ya asignados.
	nombres := map[uint32]string{}
	for _, t := range tipos {
		for j, e := range t.entradas {
			if e != nil {
				nombres[idDeRecurso(tr.idPaquete, uint32(t.id), j)] = t.nombre + "/" + e.nombre
			}
		}
	}

	// bundletool rechaza un bundle con ficheros bajo res/ que no estén en la tabla. El
	// enlazador Go escribe las configuraciones alternativas como bloques distintos; si
	// alguna ruta no aparece en la tabla (porque solo existe en otra configuración), se
	// reserva una entrada aparte en un tipo añadido. No cambia los ids de R.java.
	faltan := []string{}
	for _, r := range ficheros {
		if !referenciados["res/"+r] {
			faltan = append(faltan, r)
		}
	}
	sort.Strings(faltan)
	if len(faltan) > 0 {
		idExtra := byte(0)
		for id := range tipos {
			if id > idExtra {
				idExtra = id
			}
		}
		if idExtra >= 255 {
			return nil, fmt.Errorf("no queda id de tipo para %d ficheros sin referenciar", len(faltan))
		}
		idExtra++
		t := &tipoProto{id: idExtra, nombre: "aabfile", entradas: map[int]*entradaProto{}}
		for j, r := range faltan {
			ruta := "res/" + r
			t.entradas[j] = &entradaProto{id: j, nombre: fmt.Sprintf("file%d", j),
				valores: [][]byte{configValuePB(nil, fileReferencePB(ruta))}}
		}
		tipos[idExtra] = t
	}

	ids := make([]int, 0, len(tipos))
	for id := range tipos {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	piezas := [][]byte{}
	for _, id := range ids {
		t := tipos[byte(id)]
		piezas = append(piezas, t.construir())
	}
	paquete := pbUnir(pbMensaje(1, pbU32(1, tr.idPaquete)),
		pbCadena(2, tr.nombrePaquete))
	for _, p := range piezas {
		paquete = append(paquete, pbMensaje(3, p)...)
	}
	return pbUnir(pbMensaje(2, paquete)), nil
}

type tipoProto struct {
	id       byte
	nombre   string
	entradas map[int]*entradaProto
}

func (t *tipoProto) construir() []byte {
	piezas := [][]byte{pbMensaje(1, pbU32(1, uint32(t.id))), pbCadena(2, t.nombre)}
	ids := make([]int, 0, len(t.entradas))
	for id := range t.entradas {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		e := t.entradas[id]
		piezas = append(piezas, pbMensaje(3, e.construir()))
	}
	return pbUnir(piezas...)
}

type entradaProto struct {
	id      int
	nombre  string
	valores [][]byte
}

func (e *entradaProto) construir() []byte {
	piezas := [][]byte{pbMensaje(1, pbU32(1, uint32(e.id))), pbCadena(2, e.nombre)}
	for _, v := range e.valores {
		piezas = append(piezas, pbMensaje(6, v)) // Entry.config_value
	}
	return pbUnir(piezas...)
}

// ficherosDeArbol lista las rutas relativas bajo un directorio, con '/'.
func ficherosDeArbol(raiz string) ([]string, error) {
	lista := []string{}
	err := filepath.Walk(raiz, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(raiz, path)
		if err != nil {
			return err
		}
		lista = append(lista, filepath.ToSlash(rel))
		return nil
	})
	return lista, err
}

// ensamblarAABDesdeGo es la pieza que cierra la frontera 5.33: convierte la salida del
// enlazador Go en un .aab con la misma forma que el que ensamblaba aapt2 + bundletool.
// `dirBuild` es el directorio build/ del proyecto y `dexes` la lista que arma dexEntries, la
// misma que empaqueta el apk: el ensamblador no recorre build/ por su cuenta, porque los
// `classes.dex` que dejan los buckets de dependencias bajo build/dexcache/… colisionaban con
// el de la app al nombrarlos con filepath.Base. La firma JAR v1 se aplica después con signAAB,
// igual que en el camino de aapt2.
func ensamblarAABDesdeGo(dirBuild string, dexes []bundleEntry, salida string) error {
	axmlManifiesto, err := os.ReadFile(filepath.Join(dirBuild, "AndroidManifest.xml"))
	if err != nil {
		return fmt.Errorf("el manifiesto AXML: %w", err)
	}
	arsc, err := os.ReadFile(filepath.Join(dirBuild, "resources.arsc"))
	if err != nil {
		return fmt.Errorf("el resources.arsc: %w", err)
	}
	rutaRes := filepath.Join(dirBuild, "res")
	relRes := []string{}
	if _, err := os.Stat(rutaRes); err == nil {
		relRes, err = ficherosDeArbol(rutaRes)
		if err != nil {
			return fmt.Errorf("el arbol res/: %w", err)
		}
	}

	// Primero la tabla, para resolver los nombres de las referencias del manifiesto.
	// Se vuelve a leer el arsc dentro de tablaPBDeARSC; dos lecturas de un fichero de
	// pocos kilobytes son más baratas que mantener dos vistas distintas del mismo iterator.
	tabla, err := tablaPBDeARSC(arsc, relRes)
	if err != nil {
		return fmt.Errorf("resources.pb: %w", err)
	}
	nombres := nombresDeArsc(arsc)
	manifiesto, err := protoXmlDeAXML(axmlManifiesto, nombres)
	if err != nil {
		return fmt.Errorf("el manifiesto proto: %w", err)
	}

	out, tmp, err := abrirTemporalDePaquete(salida)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	escribe := func(nombre string, datos []byte) error {
		w, err := zw.Create(nombre)
		if err != nil {
			return err
		}
		_, err = w.Write(datos)
		return err
	}
	ok := false
	defer func() {
		if !ok {
			out.Close()
			os.Remove(tmp)
		}
	}()
	if err := escribe("BundleConfig.pb", bundleConfigPB()); err != nil {
		return err
	}
	if err := escribe("base/manifest/AndroidManifest.xml", manifiesto); err != nil {
		return err
	}
	for _, de := range dexes {
		datos, err := os.ReadFile(de.src)
		if err != nil {
			return err
		}
		if err := escribe("base/dex/"+de.name, datos); err != nil {
			return err
		}
	}
	if err := escribe("base/resources.pb", tabla); err != nil {
		return err
	}
	for _, rel := range relRes {
		datos, err := os.ReadFile(filepath.Join(rutaRes, rel))
		if err != nil {
			return err
		}
		if strings.HasSuffix(rel, ".xml") {
			datos, err = protoXmlDeAXML(datos, nombres)
			if err != nil {
				return fmt.Errorf("res/%s: %w", rel, err)
			}
		}
		if err := escribe("base/res/"+rel, datos); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, salida); err != nil {
		return err
	}
	ok = true
	return nil
}

// nombresDeArsc da el mapa id -> "tipo/entrada" para resolver las referencias del AXML.
func nombresDeArsc(datos []byte) map[uint32]string {
	m := map[uint32]string{}
	tr, err := leeTabla(datos)
	if err != nil {
		return m
	}
	vistos := map[byte]map[string]int{}
	for _, bl := range tr.bloques {
		porNombre := vistos[bl.id]
		if porNombre == nil {
			porNombre = map[string]int{}
			vistos[bl.id] = porNombre
		}
		for j, en := range bl.entradas {
			nombre := tr.claves[en.clave]
			if _, ok := porNombre[nombre]; !ok {
				porNombre[nombre] = j
			}
		}
	}
	for id, porNombre := range vistos {
		tipo := ""
		if int(id) > 0 && int(id) <= len(tr.nombresTipo) {
			tipo = tr.nombresTipo[int(id)-1]
		}
		for nombre, j := range porNombre {
			m[idDeRecurso(tr.idPaquete, uint32(id), j)] = tipo + "/" + nombre
		}
	}
	return m
}
