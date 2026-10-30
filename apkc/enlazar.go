package main

// Unir las dos mitades del paso 3-G: el arbol en texto (arbol.go) y los recursos de los values
// (valores.go) producen la tabla, y de la tabla numerada salen los dos AXML y la R. Es lo que hace
// `aapt2 link`, escrito en Go.
//
// El orden del trabajo no es capricho: un layout declara ids con `@+id/pulsar`, esos ids entran en la
// tabla, y el id de un recurso es su INDICE dentro del tipo (parrafo 29). O sea que las referencias no
// se pueden escribir hasta que la tabla esta ordenada y numerada. De ahi las dos pasadas.

import (
	"fmt"
	"strings"
)

// opcionesDeEnlace son las que aapt2 recibe por bandera y que aqui se pasan a mano: la version con la
// que se compila, que el manifiesto gana cuatro atributos de estas, y su nombre en texto.
type opcionesDeEnlace struct {
	compileSdkVersion  int
	compileSdkCodename string
}

// idsNuevosDeArbol son los `@+id/nombre` que un arbol declara. Se apuntan antes de numerar.
func idsNuevosDeArbol(a *arbolFuente) []recursoEntabla {
	out := []recursoEntabla{}
	visto := map[string]bool{}
	var recorrer func(*nodoFuente)
	recorrer = func(n *nodoFuente) {
		for _, attr := range n.attrs {
			if !strings.HasPrefix(attr.texto, "@+id/") {
				continue
			}
			nombre := strings.TrimPrefix(attr.texto, "@+id/")
			if visto[nombre] {
				continue
			}
			visto[nombre] = true
			// medido: el valor de una entrada de tipo id es un booleano a cero (parrafo 29, variante A)
			out = append(out, recursoEntabla{tipo: "id", clave: nombre, valor: valorEscribir{tipo: tipoBooleano}})
		}
		for _, h := range n.hijos {
			recorrer(h)
		}
	}
	if a != nil && a.raiz != nil {
		recorrer(a.raiz)
	}
	return out
}

// idsDelPaquete invierte la tabla ya ordenada: "style/AppTheme" -> 0x7f030000. Es el mapa con el que se
// resuelven las referencias del propio paquete en la segunda pasada.
func idsDelPaquete(t tablaEscribir) (map[string]uint32, error) {
	out := map[string]uint32{}
	for _, ti := range t.tipos {
		if int(ti.id) > len(t.nombresTipo) {
			return nil, fmt.Errorf("el tipo %d no tiene nombre", ti.id)
		}
		nombreTipo := t.nombresTipo[int(ti.id)-1]
		for j, e := range ti.entradas {
			if e.clave >= len(t.claves) {
				return nil, fmt.Errorf("la entrada %d del tipo %s nombra la clave %d y hay %d", j, nombreTipo, e.clave, len(t.claves))
			}
			out[nombreTipo+"/"+t.claves[e.clave]] = t.idPaquete<<24 | uint32(ti.id)<<16 | uint32(j)
		}
	}
	return out, nil
}

// resuelveReferenciaDePaquete traduce `@style/AppTheme` o `@id/pulsar` contra la tabla numerada.
func resuelveReferenciaDePaquete(texto string, ids map[string]uint32) (uint32, error) {
	cuerpo := strings.TrimPrefix(texto, "@")
	cuerpo = strings.TrimPrefix(cuerpo, "+") // un `@+id/x` ya quedo declarado en la primera pasada
	i := strings.Index(cuerpo, "/")
	if i < 0 {
		return 0, fmt.Errorf("%q no es de forma @tipo/nombre", texto)
	}
	tipo, nombre := cuerpo[:i], cuerpo[i+1:]
	id, ok := ids[tipo+"/"+nombre]
	if !ok {
		return 0, fmt.Errorf("%q no esta entre los recursos del paquete", texto)
	}
	return id, nil
}

// valorDeAtributoDa el Res_value de un atributo de un arbol. `m` resuelve los enum y flags y las
// referencias al framework; `ids` resuelve las referencias al paquete propio, que no pueden resolverse
// antes de numerar la tabla.
func valorDeAtributo(attr attrFuente, m *marcoRecursos, ids map[string]uint32) (valorEscribir, error) {
	if strings.HasPrefix(attr.texto, "@android:") {
		id, err := referenciaDeFramework(attr.texto, m)
		if err != nil {
			return valorEscribir{}, err
		}
		return valorEscribir{tipo: tipoReferencia, datos: id}, nil
	}
	if strings.HasPrefix(attr.texto, "@") {
		id, err := resuelveReferenciaDePaquete(attr.texto, ids)
		if err != nil {
			return valorEscribir{}, err
		}
		return valorEscribir{tipo: tipoReferencia, datos: id}, nil
	}
	// el nombre del atributo es la clave de los enum: `layout_width` -> wrap_content, `gravity` -> center
	for nombre, v := range m.valoresDeAtributo(attr.nombre) {
		if nombre == attr.texto {
			return v, nil
		}
	}
	switch attr.texto {
	case "true":
		return valorEscribir{tipo: tipoBooleano, datos: 0xffffffff}, nil
	case "false":
		return valorEscribir{tipo: tipoBooleano, datos: 0}, nil
	}
	if v, err := colorDeTexto(attr.texto); err == nil {
		return v, nil
	}
	if v, err := enteroDeTexto(attr.texto); err == nil {
		return v, nil
	}
	return valorEscribir{tipo: tipoCadena, texto: attr.texto}, nil
}

// documentoDeArbol materializa el AXML de un arbol. `utf8` es el modo del pool, que se midio distinto
// segun la herramienta que escribe el fichero: UTF-8 en los layouts (los compila `aapt2 compile`) y
// UTF-16 en el manifiesto (lo enlaza `aapt2 link`).
func documentoDeArbol(a *arbolFuente, ruta string, m *marcoRecursos, ids map[string]uint32, utf8 bool, extras []atributoEscribir) (*documentoAXML, error) {
	prefijoRecursos, declarado := a.prefijos[uriDeRecursos]
	ns := []declaracionNS{}
	if declarado {
		ns = append(ns, declaracionNS{prefijo: prefijoRecursos, uri: uriDeRecursos, linea: a.lineaDe[prefijoRecursos]})
	}
	raiz, err := nodoAXMLDe(a.raiz, m, ids, prefijoRecursos, extras)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ruta, err)
	}
	return &documentoAXML{ns: ns, raiz: raiz, utf8: utf8}, nil
}

// nodoAXMLDe convierte un nodo del arbol en su elemento Escribir, con el id de cada atributo resuelto
// contra la tabla del framework.
func nodoAXMLDe(n *nodoFuente, m *marcoRecursos, ids map[string]uint32, prefijoRecursos string, extras []atributoEscribir) (*elementoEscribir, error) {
	out := &elementoEscribir{nombre: n.nombre, linea: n.linea}
	if n.prefijo != "" {
		if n.prefijo != prefijoRecursos {
			return nil, fmt.Errorf("el elemento %q usa un prefijo no medido (%q)", n.nombre, n.prefijo)
		}
		out.ns = n.prefijo
	}
	for _, attr := range n.attrs {
		escrito := atributoEscribir{ns: attr.prefijo, nombre: attr.nombre}
		if attr.prefijo != "" {
			id, err := m.idDeAtributo(attr.nombre)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n.nombre, err)
			}
			escrito.id = id
		}
		v, err := valorDeAtributo(attr, m, ids)
		if err != nil {
			return nil, fmt.Errorf("%s: atributo %s: %w", n.nombre, attr.nombre, err)
		}
		escrito.tipo, escrito.texto, escrito.datos = v.tipo, v.texto, v.datos
		out.attrs = append(out.attrs, escrito)
	}
	out.attrs = append(out.attrs, extras...)
	for _, h := range n.hijos {
		hijo, err := nodoAXMLDe(h, m, ids, prefijoRecursos, nil)
		if err != nil {
			return nil, err
		}
		out.hijos = append(out.hijos, hijo)
	}
	return out, nil
}

// atributosQueElEnlaceAnyade: los cuatro que aapt2 escribe en <manifest> sin que esten en el fuente
// (parrafo 30, medidos sobre el fichero fijado). compileSdkVersion y platformBuildVersionCode salen
// enteros, compileSdkVersionCodename cadena, y platformBuildVersionName ENTERO aunque el texto sea "14".
func atributosQueElEnlaceAnade(opts opcionesDeEnlace, m *marcoRecursos) ([]atributoEscribir, error) {
	idDe := func(nombre string) (uint32, error) {
		if nombre == "platformBuildVersionCode" || nombre == "platformBuildVersionName" {
			return 0, nil // sin namespace: no van al mapa de recursos
		}
		return m.idDeAtributo(nombre)
	}
	// medido en el fichero fijado: platformBuildVersionCode es el numero de API (34) y
	// platformBuildVersionName es el NUMERO DE VERSION de Android escrito como entero (14), con crudo
	// -1. El nombre sale del codename, que en la fijacion vale "14".
	nombrePlataforma, err := enteroDeTexto(opts.compileSdkCodename)
	if err != nil {
		return nil, fmt.Errorf("platformBuildVersionName sale del codename, y %q no es un numero: la forma con un codename no numerico no esta medida", opts.compileSdkCodename)
	}
	out := []atributoEscribir{}
	for _, par := range []struct {
		ns, nombre, texto string
		tipo              byte
		datos             uint32
	}{
		{"android", "compileSdkVersion", "", tipoEnteroDec, uint32(opts.compileSdkVersion)},
		{"android", "compileSdkVersionCodename", opts.compileSdkCodename, tipoCadena, 0},
		{"", "platformBuildVersionCode", "", tipoEnteroDec, uint32(opts.compileSdkVersion)},
		{"", "platformBuildVersionName", "", tipoEnteroDec, nombrePlataforma.datos},
	} {
		id, err := idDe(par.nombre)
		if err != nil {
			return nil, err
		}
		v := valorEscribir{tipo: par.tipo, datos: par.datos}
		if par.tipo == tipoCadena {
			v = valorEscribir{tipo: tipoCadena, texto: par.texto}
		}
		out = append(out, atributoEscribir{ns: par.ns, nombre: par.nombre, id: id, tipo: v.tipo, texto: v.texto, datos: v.datos})
	}
	return out, nil
}

// proyectoEnlazado es lo que sale de enlazarProyecto: la tabla, los dos AXML y la R.
type proyectoEnlazado struct {
	tabla      tablaEscribir
	manifiesto documentoAXML
	layouts    map[string]documentoAXML
	r          string
}

// enlazarProyecto hace las dos pasadas. `valores` son los res/values/*.xml, `layouts` los
// res/layout/*.xml y `manifiesto` el AndroidManifest.xml, todos en texto.
func enlazarProyecto(valores, layouts []fuenteXML, manifiesto fuenteXML, m *marcoRecursos, opts opcionesDeEnlace) (*proyectoEnlazado, error) {
	recursos := []recursoEntabla{}
	for _, f := range valores {
		nuevos, err := recursosDeValores(f, m)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.nombre, err)
		}
		recursos = append(recursos, nuevos...)
	}
	// primera pasada por los arboles: descubrir los ids que declaran
	arbolManifesto, err := leerArbolDe(manifiesto)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", manifiesto.nombre, err)
	}
	recursos = append(recursos, idsNuevosDeArbol(arbolManifesto)...)
	arboles := map[string]*arbolFuente{}
	for _, f := range layouts {
		a, err := leerArbolDe(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.nombre, err)
		}
		arboles[f.nombre] = a
		recursos = append(recursos, idsNuevosDeArbol(a)...)
	}

	paquete, err := nombreDelPaquete(arbolManifesto)
	if err != nil {
		return nil, err
	}
	tabla, err := tablaDeRecursos(recursos, layouts, paquete)
	if err != nil {
		return nil, err
	}
	ids, err := idsDelPaquete(*tabla)
	if err != nil {
		return nil, err
	}

	salida := &proyectoEnlazado{tabla: *tabla, layouts: map[string]documentoAXML{}}
	extras, err := atributosQueElEnlaceAnade(opts, m)
	if err != nil {
		return nil, err
	}
	man, err := documentoDeArbol(arbolManifesto, manifiesto.nombre, m, ids, false, extras)
	if err != nil {
		return nil, err
	}
	salida.manifiesto = *man
	for _, f := range layouts {
		doc, err := documentoDeArbol(arboles[f.nombre], f.nombre, m, ids, true, nil)
		if err != nil {
			return nil, err
		}
		salida.layouts[f.nombre] = *doc
	}
	r, err := rDeTabla(*tabla)
	if err != nil {
		return nil, err
	}
	salida.r = r
	return salida, nil
}

// nombreDelPaquete saca el atributo `package` del root del manifiesto, que es de donde aapt2 toma el
// nombre del paquete (medido: vale 0x7f en los ids, y el nombre sale del fuente).
func nombreDelPaquete(a *arbolFuente) (string, error) {
	paquete, ok := a.raiz.atribuDe("", "package")
	if !ok || paquete == "" {
		return "", fmt.Errorf("el manifiesto no declara package")
	}
	return paquete, nil
}
