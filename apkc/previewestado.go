package main

// Códec explícito de estado de la sesión del preview.
//
// No se usa `encoding/json` sobre la máquina: un `metodoDalvik` lleva su `code_item` y un puntero al
// dex del que salió (previewhotswap.go:33-37 y previewinterprete.go:385-396), y un `objetoDalvik`
// apunta a otros objetos (oyentes, vistas, menús…). Un marshal directo convertiría esos punteros en
// `{}` y rompería la identidad, que es justo lo que el hot swap tiene que conservar.
//
// En su lugar se construye una instantánea con TABLAS DE IDS ESTABLES:
//   - cada dex recibe un id yViaja por `estadoWasm.Dexes` como bytes (se rehidrata con `readDex`);
//   - cada `*objetoDalvik` y cada `*arregloDalvik` recibe un id, y toda referencia se guarda como ese
//     id dentro de un `nodoValor`; al rehidratar se crean todos los objetos primero y después se
//     rellenan, para que un aliás siga siendo el mismo puntero;
//   - un `metodoDalvik` se guarda como (dex, clase, nombre, proto, …) y su `code_item` se vuelve a
//     leer del dex por id; no se serializan sus índices, que solo tienen sentido dentro de su dex.
//
// La identidad se mide comparando punteros: dos registros que apuntaban al mismo oyente deben seguir
// apuntando al mismo objeto rehidratado, y la vista de un registro debe ser la misma instancia que
// la que el objeto tiene en su campo `oyente`.

import (
	"fmt"
	"sort"
	"strings"
)

// nodoValor es la forma explícita de un valor arbitrario de la máquina. `k` dice qué campo se usa:
// "nulo", "obj" (id de objeto), "arr" (id de arreglo), "i32"/"u32"/"i64", "f32"/"f64", "cad",
// "bool", "lis" (lista de nodos) o "map" (mapa de nodos). Así ningún `any` se deja al marshal: los
// objetos son ids y los escalares tienen un tipo declarado que se reconstruye sin perder el ancho.
type nodoValor struct {
	K string               `json:"k"`
	R int                  `json:"r,omitempty"`
	N int64                `json:"n,omitempty"`
	F float64              `json:"f,omitempty"`
	S string               `json:"s,omitempty"`
	B bool                 `json:"b,omitempty"`
	L []nodoValor          `json:"l,omitempty"`
	M map[string]nodoValor `json:"m,omitempty"`
}

// tablaObjetos es el índice de identidad: dos punteros distintos reciben ids distintos y el mismo
// puntero siempre recibe el mismo id, incluso si aparece en varios sitios del grafo.
type tablaObjetos struct {
	objs     map[*objetoDalvik]int
	arrs     map[*arregloDalvik]int
	objetos  []*objetoDalvik
	arreglos []*arregloDalvik
}

func nuevaTablaObjetos() *tablaObjetos {
	return &tablaObjetos{objs: map[*objetoDalvik]int{}, arrs: map[*arregloDalvik]int{}}
}

// tablaDexes es el índice de los ficheros dex que aparecen en la sesión: el de arranque y los que
// el hot swap haya podido enganchar en la tabla de despacho de cualquier clase viva.
type tablaDexes struct {
	lista []*dexFile
	ids   map[*dexFile]int
}

func nuevaTablaDexes() *tablaDexes { return &tablaDexes{ids: map[*dexFile]int{}} }

func (t *tablaDexes) id(d *dexFile) int {
	if d == nil {
		return -1
	}
	if id, hay := t.ids[d]; hay {
		return id
	}
	id := len(t.lista)
	t.lista = append(t.lista, d)
	t.ids[d] = id
	return id
}

// estadoWasm es la instantánea completa y autocontenida: los dex viajan como bytes, los objetos y
// arreglos como tablas con ids, y la máquina y las clases como estructuras que solo guardan ids.
type estadoWasm struct {
	Dexes    [][]byte               `json:"dexes"`
	Clases   map[string]claseEstado `json:"clases"`
	Objetos  []objetoEstado         `json:"objetos"`
	Arreglos []arregloEstado        `json:"arreglos"`
	Maquina  maquinaEstado          `json:"maquina"`
}

type claseEstado struct {
	Nombre    string                  `json:"nombre"`
	Super     string                  `json:"super"`
	Metodos   map[string]metodoEstado `json:"metodos"`
	Estaticos map[string]nodoValor    `json:"estaticos"`
}

// metodoEstado guarda la identidad de un método y el dex del que salió. `codeOff` no se guarda: se
// recupera del dex por (clase, nombre, proto) al rehidratar, porque el índice de sus instrucciones
// solo vale dentro de su propio fichero.
type metodoEstado struct {
	Dex       int    `json:"dex"`
	Clase     string `json:"clase"`
	Nombre    string `json:"nombre"`
	Proto     string `json:"proto"`
	Retorno   string `json:"retorno"`
	Acceso    uint32 `json:"acceso"`
	ConCuerpo bool   `json:"con_cuerpo"`
}

type arregloEstado struct {
	Tipo  string      `json:"tipo"`
	Datos []nodoValor `json:"datos"`
}

type objetoEstado struct {
	ID             int                  `json:"id"`
	Clase          string               `json:"clase"`
	Campos         map[string]nodoValor `json:"campos"`
	IDRecurso      uint32               `json:"id_recurso"`
	Texto          string               `json:"texto"`
	TextoEscrito   bool                 `json:"texto_escrito"`
	Tag            nodoValor            `json:"tag"`
	Cadena         string               `json:"cadena"`
	Oyente         nodoValor            `json:"oyente"`
	Marcado        bool                 `json:"marcado"`
	MarcadoPuesto  bool                 `json:"marcado_puesto"`
	Vigilantes     []nodoValor          `json:"vigilantes"`
	OyenteMarca    nodoValor            `json:"oyente_marca"`
	Pista          string               `json:"pista"`
	MenuItems      []int                `json:"menu_items"`
	ItemID         int32                `json:"item_id"`
	ItemGrupo      int32                `json:"item_grupo"`
	ItemOrden      int32                `json:"item_orden"`
	ItemTitulo     string               `json:"item_titulo"`
	ItemVisible    bool                 `json:"item_visible"`
	ItemHabilitado bool                 `json:"item_habilitado"`
	ItemMarcado    bool                 `json:"item_marcado"`
	ItemOyente     nodoValor            `json:"item_oyente"`
	IntentClase    string               `json:"intent_clase"`
	Bundle         map[string]nodoValor `json:"bundle"`
	PrefsFichero   string               `json:"prefs_fichero"`
	Editor         map[string]nodoValor `json:"editor"`
	Adaptador      nodoValor            `json:"adaptador"`
	Elementos      []nodoValor          `json:"elementos"`
	LayoutID       uint32               `json:"layout_id"`
	Vistas         map[uint32]int       `json:"vistas"`
	Mensaje        string               `json:"mensaje"`
	Lista          []nodoValor          `json:"lista"`
	Conjunto       []nodoValor          `json:"conjunto"`
	Iterando       []nodoValor          `json:"iterando"`
	Indice         int                  `json:"indice"`
	Bluetooth      bool                 `json:"bluetooth"`
	PaintColor     uint32               `json:"paint_color"`
	PaintPuesto    bool                 `json:"paint_puesto"`
	PaintGrosor    float64              `json:"paint_grosor"`
	AnchoVista     int32                `json:"ancho_vista"`
	AltoVista      int32                `json:"alto_vista"`
}

type oyenteEstado struct {
	Oyente int    `json:"oyente"`
	Vista  int    `json:"vista"`
	Metodo string `json:"metodo"`
}

type oyenteItemEstado struct {
	Oyente int `json:"oyente"`
	Vista  int `json:"vista"`
}

// maquinaEstado es la parte del estado que no vive en los objetos. Los campos que son punteros
// compartidos (`reloj`, `registro`, `almacen`) se guardan por valor: la instantánea es una sesión
// nueva, no una resurrección de la anterior, y lo que importa es el estado lógico que transportan.
// `tabla` y `dibujo` no viajan: los aporta el que rehidrata (son del entorno y del instante, no del
// estado de la app).
type maquinaEstado struct {
	Pasos            int                             `json:"pasos"`
	Avisos           []string                        `json:"avisos"`
	Vistos           map[string]bool                 `json:"vistos"`
	Capturas         []capturaTexto                  `json:"capturas"`
	Consola          []lineaConsola                  `json:"consola"`
	Oyentes          []oyenteEstado                  `json:"oyentes"`
	OyentesItem      []oyenteItemEstado              `json:"oyentes_item"`
	Interaccion      int                             `json:"interaccion"`
	Profundidad      int                             `json:"profundidad"`
	Actividad        int                             `json:"actividad"`
	Vistas           map[uint32]int                  `json:"vistas"`
	TextosDeLayout   map[uint32]string               `json:"textos_layout"`
	TiposVista       map[uint32]string               `json:"tipos_vista"`
	Marcas           map[uint32]bool                 `json:"marcas"`
	Pistas           map[uint32]string               `json:"pistas"`
	MarcasConocidas  map[uint32]bool                 `json:"marcas_conocidas"`
	GruposRadio      map[uint32][]uint32             `json:"grupos_radio"`
	GrupoDe          map[uint32]uint32               `json:"grupo_de"`
	Listas           []int                           `json:"listas"`
	LayoutID         uint32                          `json:"layout_id"`
	Pendiente        string                          `json:"pendiente"`
	Terminada        bool                            `json:"terminada"`
	Arrancada        bool                            `json:"arrancada"`
	Declaradas       map[string]bool                 `json:"declaradas"`
	Ciclo            []string                        `json:"ciclo"`
	Registro         []string                        `json:"registro"`
	Menu             int                             `json:"menu"`
	MenuAbierto      bool                            `json:"menu_abierto"`
	MenuSeleccionado string                          `json:"menu_seleccionado"`
	Intent           int                             `json:"intent"`
	IntentoSaliente  int                             `json:"intento_saliente"`
	BundleInicial    int                             `json:"bundle_inicial"`
	Dialogos         []int                           `json:"dialogos"`
	Toast            string                          `json:"toast"`
	EstadoGuardado   map[string]nodoValor            `json:"estado_guardado"`
	Reloj            int64                           `json:"reloj"`
	Almacen          map[string]map[string]nodoValor `json:"almacen"`
	Tareas           int                             `json:"tareas"`
	Encoladas        int                             `json:"encoladas"`
	Cortada          bool                            `json:"cortada"`
	PasosBase        int                             `json:"pasos_base"`
}

// ---- captura -------------------------------------------------------------------------------

// capturaEstado congela `m` en una instantánea con ids estables. El orden de recorrido es fijo para
// que la misma sesión produzca siempre el mismo estado (los mapas se recorren por clave ordenada).
func capturaEstado(m *maquinaDalvik) *estadoWasm {
	e := &estadoWasm{Clases: map[string]claseEstado{}}
	if m == nil {
		return e
	}
	td := nuevaTablaDexes()
	if m.prog != nil {
		td.id(m.prog.dex)
	}
	to := nuevaTablaObjetos()

	// Descubrir todo el grafo ANTES de serializar: los estáticos pueden apuntar a objetos y su id
	// tiene que existir cuando se capturan las clases, no después.
	registraMaquina(m, to)

	if m.prog != nil {
		for _, nombre := range clavesString(m.prog.clases) {
			cl := m.prog.clases[nombre]
			ce := claseEstado{Nombre: cl.nombre, Super: cl.super,
				Metodos: map[string]metodoEstado{}, Estaticos: map[string]nodoValor{}}
			for k, met := range cl.metodos {
				me := metodoEstado{Clase: cl.nombre}
				if met != nil {
					me.Dex = td.id(met.dex)
					me.Nombre = met.nombre
					me.Proto = met.proto
					me.Retorno = met.retorno
					me.Acceso = met.acceso
					me.ConCuerpo = met.item != nil
				}
				ce.Metodos[k] = me
			}
			for _, k := range clavesString(cl.estaticos) {
				ce.Estaticos[k] = capturaValor(cl.estaticos[k], to)
			}
			e.Clases[nombre] = ce
		}
	}

	for _, obj := range to.objetos {
		e.Objetos = append(e.Objetos, capturaObjeto(obj, to))
	}
	for _, arr := range to.arreglos {
		e.Arreglos = append(e.Arreglos, capturaArreglo(arr, to))
	}
	for _, d := range td.lista {
		e.Dexes = append(e.Dexes, d.raw)
	}
	e.Maquina = capturaMaquina(m, to)
	return e
}

// registraMaquina descubre todos los objetos alcanzables desde la máquina. Registrar antes de
// serializar es lo que permite que una referencia apunte a un id ya asignado, incluso en ciclos.
// Los estáticos del programa también son raíces: un `putstatic` puede guardar ahí una vista o un
// singleton, y sin recorrerlos esa referencia se rehidrataría como el objeto equivocado.
func registraMaquina(m *maquinaDalvik, to *tablaObjetos) {
	if m.prog != nil {
		for _, nombre := range clavesString(m.prog.clases) {
			cl := m.prog.clases[nombre]
			for _, k := range clavesString(cl.estaticos) {
				registraValor(cl.estaticos[k], to)
			}
		}
	}
	registraValor(m.actividad, to)
	for _, reg := range m.oyentes {
		registraValor(reg.vista, to)
		registraValor(reg.oyente, to)
	}
	for _, reg := range m.oyentesItem {
		registraValor(reg.vista, to)
		registraValor(reg.oyente, to)
	}
	for _, k := range clavesUint32(m.vistas) {
		registraValor(m.vistas[k], to)
	}
	for _, v := range m.listas {
		registraValor(v, to)
	}
	for _, v := range m.dialogos {
		registraValor(v, to)
	}
	registraValor(m.menu, to)
	registraValor(m.intent, to)
	registraValor(m.intentoSaliente, to)
	registraValor(m.bundleInicial, to)
	for _, v := range m.estadoGuardado {
		registraValor(v, to)
	}
	if m.almacen != nil {
		for _, f := range m.almacen.ficheros {
			for _, v := range f {
				registraValor(v, to)
			}
		}
	}
}

func registraObjeto(o *objetoDalvik, to *tablaObjetos) {
	if o == nil {
		return
	}
	if _, hay := to.objs[o]; hay {
		return
	}
	to.objs[o] = len(to.objetos)
	to.objetos = append(to.objetos, o)

	registraValor(o.oyente, to)
	registraValor(o.itemOyente, to)
	registraValor(o.oyenteMarca, to)
	registraValor(o.tag, to)
	registraValor(o.adaptador, to)
	for _, v := range o.vigilantesTexto {
		registraValor(v, to)
	}
	for _, v := range o.elementos {
		registraValor(v, to)
	}
	for _, v := range o.lista {
		registraValor(v, to)
	}
	for _, v := range o.conjunto {
		registraValor(v, to)
	}
	for _, v := range o.iterando {
		registraValor(v, to)
	}
	for _, k := range clavesString(o.campos) {
		registraValor(o.campos[k], to)
	}
	for _, k := range clavesString(o.bundle) {
		registraValor(o.bundle[k], to)
	}
	for _, k := range clavesString(o.editor) {
		registraValor(o.editor[k], to)
	}
	for _, item := range o.menuItems {
		registraObjeto(item, to)
	}
	for _, k := range clavesUint32(o.vistas) {
		registraObjeto(o.vistas[k], to)
	}
}

func registraValor(v any, to *tablaObjetos) {
	switch x := v.(type) {
	case *objetoDalvik:
		registraObjeto(x, to)
	case *arregloDalvik:
		if x == nil {
			return
		}
		if _, hay := to.arrs[x]; hay {
			return
		}
		to.arrs[x] = len(to.arreglos)
		to.arreglos = append(to.arreglos, x)
		for _, dato := range x.datos {
			registraValor(dato, to)
		}
	case []any:
		for _, dato := range x {
			registraValor(dato, to)
		}
	case map[string]any:
		for _, dato := range x {
			registraValor(dato, to)
		}
	}
}

func capturaValor(v any, to *tablaObjetos) nodoValor {
	switch x := v.(type) {
	case nil:
		return nodoValor{K: "nulo"}
	case *objetoDalvik:
		if x == nil {
			return nodoValor{K: "nulo"}
		}
		id, hay := to.objs[x]
		if !hay {
			return nodoValor{K: "obj", R: -1}
		}
		return nodoValor{K: "obj", R: id}
	case *arregloDalvik:
		if x == nil {
			return nodoValor{K: "nulo"}
		}
		id, hay := to.arrs[x]
		if !hay {
			return nodoValor{K: "arr", R: -1}
		}
		return nodoValor{K: "arr", R: id}
	case int32:
		return nodoValor{K: "i32", N: int64(x)}
	case int64:
		return nodoValor{K: "i64", N: x}
	case int:
		return nodoValor{K: "i64", N: int64(x)}
	case uint32:
		return nodoValor{K: "u32", N: int64(x)}
	case float32:
		return nodoValor{K: "f32", F: float64(x)}
	case float64:
		return nodoValor{K: "f64", F: x}
	case string:
		return nodoValor{K: "cad", S: x}
	case bool:
		return nodoValor{K: "bool", B: x}
	case *strings.Builder:
		if x == nil {
			return nodoValor{K: "nulo"}
		}
		return nodoValor{K: "cad", S: x.String()}
	case []any:
		n := nodoValor{K: "lis"}
		for _, dato := range x {
			n.L = append(n.L, capturaValor(dato, to))
		}
		return n
	case map[string]any:
		n := nodoValor{K: "map", M: map[string]nodoValor{}}
		for k, dato := range x {
			n.M[k] = capturaValor(dato, to)
		}
		return n
	}
	// Un tipo que el códec no conoce no se finge: queda como nulo y la deuda queda a la vista.
	return nodoValor{K: "nulo"}
}

func capturaArreglo(a *arregloDalvik, to *tablaObjetos) arregloEstado {
	ae := arregloEstado{Tipo: a.tipo}
	for _, dato := range a.datos {
		ae.Datos = append(ae.Datos, capturaValor(dato, to))
	}
	return ae
}

func capturaObjeto(o *objetoDalvik, to *tablaObjetos) objetoEstado {
	oe := objetoEstado{
		ID: idObjeto(o, to), IDRecurso: o.id,
		Clase: o.clase, Texto: o.texto, TextoEscrito: o.textoEscrito,
		Tag: capturaValor(o.tag, to), Oyente: capturaValor(o.oyente, to),
		Marcado: o.marcado, MarcadoPuesto: o.marcadoPuesto,
		OyenteMarca: capturaValor(o.oyenteMarca, to), Pista: o.pista,
		ItemID: o.itemID, ItemGrupo: o.itemGrupo, ItemOrden: o.itemOrden,
		ItemTitulo: o.itemTitulo, ItemVisible: o.itemVisible, ItemHabilitado: o.itemHabilitado,
		ItemMarcado: o.itemMarcado, ItemOyente: capturaValor(o.itemOyente, to),
		IntentClase: o.intentClase, PrefsFichero: o.prefsFichero,
		Adaptador: capturaValor(o.adaptador, to), LayoutID: o.layoutID,
		Mensaje: o.mensajeExcepcion, Indice: o.indice, Bluetooth: o.bluetoothActivo,
		PaintColor: o.paintColor, PaintPuesto: o.paintPuesto, PaintGrosor: o.paintGrosor,
		AnchoVista: o.anchoVista, AltoVista: o.altoVista,
		Campos: map[string]nodoValor{}, Bundle: map[string]nodoValor{},
		Editor: map[string]nodoValor{}, Vistas: map[uint32]int{},
	}
	if o.cadena != nil {
		oe.Cadena = o.cadena.String()
	}
	for _, v := range o.vigilantesTexto {
		oe.Vigilantes = append(oe.Vigilantes, capturaValor(v, to))
	}
	for _, v := range o.elementos {
		oe.Elementos = append(oe.Elementos, capturaValor(v, to))
	}
	for _, v := range o.lista {
		oe.Lista = append(oe.Lista, capturaValor(v, to))
	}
	for _, v := range o.conjunto {
		oe.Conjunto = append(oe.Conjunto, capturaValor(v, to))
	}
	for _, v := range o.iterando {
		oe.Iterando = append(oe.Iterando, capturaValor(v, to))
	}
	for _, k := range clavesString(o.campos) {
		oe.Campos[k] = capturaValor(o.campos[k], to)
	}
	for _, k := range clavesString(o.bundle) {
		oe.Bundle[k] = capturaValor(o.bundle[k], to)
	}
	for _, k := range clavesString(o.editor) {
		oe.Editor[k] = capturaValor(o.editor[k], to)
	}
	for _, item := range o.menuItems {
		oe.MenuItems = append(oe.MenuItems, idObjeto(item, to))
	}
	for _, k := range clavesUint32(o.vistas) {
		oe.Vistas[k] = idObjeto(o.vistas[k], to)
	}
	return oe
}

func capturaMaquina(m *maquinaDalvik, to *tablaObjetos) maquinaEstado {
	me := maquinaEstado{
		Pasos: m.pasos, Avisos: copiaStrings(m.avisos), Vistos: copiaBool(m.vistos),
		Capturas:    append([]capturaTexto(nil), m.capturas...),
		Consola:     append([]lineaConsola(nil), m.consola...),
		Interaccion: m.interaccion, Profundidad: m.profundidad,
		Actividad: idObjeto(m.actividad, to), Vistas: map[uint32]int{},
		TextosDeLayout: copiaTextos(m.textosDeLayout), TiposVista: copiaTextos(m.tiposVista),
		Marcas: copiaMarcas(m.marcas), Pistas: copiaTextos(m.pistas),
		MarcasConocidas: copiaMarcas(m.marcasConocidas),
		GruposRadio:     copiaGrupos(m.gruposRadio), GrupoDe: copiaGrupoDe(m.grupoDe),
		LayoutID: m.layoutID, Pendiente: m.pendiente, Terminada: m.terminada,
		Arrancada: m.arrancada, Declaradas: copiaBool(m.declaradas),
		Ciclo: copiaStrings(m.ciclo), Registro: copiaStrings(derefRegistro(m.registro)),
		Menu: idObjeto(m.menu, to), MenuAbierto: m.menuAbierto,
		MenuSeleccionado: m.menuSeleccionado, Intent: idObjeto(m.intent, to),
		IntentoSaliente: idObjeto(m.intentoSaliente, to),
		BundleInicial:   idObjeto(m.bundleInicial, to), Toast: m.toast,
		EstadoGuardado: map[string]nodoValor{}, Almacen: map[string]map[string]nodoValor{},
		Tareas: m.tareas, Encoladas: m.encoladas, Cortada: m.cortada, PasosBase: m.pasosBase,
	}
	if m.reloj != nil {
		me.Reloj = *m.reloj
	}
	for _, reg := range m.oyentes {
		me.Oyentes = append(me.Oyentes, oyenteEstado{
			Oyente: idAny(reg.oyente, to), Vista: idAny(reg.vista, to), Metodo: reg.metodo,
		})
	}
	for _, reg := range m.oyentesItem {
		me.OyentesItem = append(me.OyentesItem, oyenteItemEstado{
			Oyente: idAny(reg.oyente, to), Vista: idAny(reg.vista, to),
		})
	}
	for _, v := range m.listas {
		me.Listas = append(me.Listas, idObjeto(v, to))
	}
	for _, v := range m.dialogos {
		me.Dialogos = append(me.Dialogos, idObjeto(v, to))
	}
	for _, k := range clavesUint32(m.vistas) {
		me.Vistas[k] = idObjeto(m.vistas[k], to)
	}
	for _, k := range clavesString(m.estadoGuardado) {
		me.EstadoGuardado[k] = capturaValor(m.estadoGuardado[k], to)
	}
	if m.almacen != nil {
		for _, f := range clavesString(m.almacen.ficheros) {
			me.Almacen[f] = map[string]nodoValor{}
			for _, k := range clavesString(m.almacen.ficheros[f]) {
				me.Almacen[f][k] = capturaValor(m.almacen.ficheros[f][k], to)
			}
		}
	}
	return me
}

// ---- rehidratación --------------------------------------------------------------------------

// restauraEstado construye una máquina nueva a partir de la instantánea. Los objetos se crean todos
// antes de rellenarse para que cualquier referencia, incluso un ciclo, resuelva al mismo puntero.
func restauraEstado(e *estadoWasm) (*maquinaDalvik, error) {
	if e == nil {
		return nil, fmt.Errorf("no hay estado que rehidratar")
	}
	dexes := make([]*dexFile, len(e.Dexes))
	for i, raw := range e.Dexes {
		d, err := readDex(raw)
		if err != nil {
			return nil, fmt.Errorf("el dex %d del estado no se lee: %w", i, err)
		}
		dexes[i] = d
	}

	objs := make([]*objetoDalvik, len(e.Objetos))
	for i := range objs {
		objs[i] = &objetoDalvik{}
	}
	arrs := make([]*arregloDalvik, len(e.Arreglos))
	for i := range arrs {
		arrs[i] = &arregloDalvik{}
	}

	prog := &programaDalvik{clases: map[string]*claseDalvik{}}
	if len(dexes) > 0 {
		prog.dex = dexes[0]
	}
	for _, nombre := range clavesString(e.Clases) {
		ce := e.Clases[nombre]
		cl := &claseDalvik{nombre: ce.Nombre, super: ce.Super,
			metodos: map[string]*metodoDalvik{}, estaticos: map[string]any{}}
		for k, me := range ce.Metodos {
			var d *dexFile
			if me.Dex >= 0 && me.Dex < len(dexes) {
				d = dexes[me.Dex]
			}
			met := &metodoDalvik{nombre: me.Nombre, proto: me.Proto,
				retorno: me.Retorno, acceso: me.Acceso, dex: d}
			if me.ConCuerpo && d != nil {
				off := codeOffDeMetodo(d, ce.Nombre, me.Nombre, me.Proto)
				if off != 0 {
					item, err := d.readCodeItem(off)
					if err != nil {
						return nil, fmt.Errorf("%s.%s%s: el code_item del estado no se lee: %w",
							claseCorta(ce.Nombre), me.Nombre, me.Proto, err)
					}
					met.item = &item
				}
			}
			cl.metodos[k] = met
		}
		for _, k := range clavesString(ce.Estaticos) {
			cl.estaticos[k] = decodificaValor(ce.Estaticos[k], objs, arrs)
		}
		prog.clases[nombre] = cl
	}

	for i, oe := range e.Objetos {
		rellenaObjeto(objs[i], oe, objs, arrs)
	}
	for i, ae := range e.Arreglos {
		rellenaArreglo(arrs[i], ae, objs, arrs)
	}

	m := nuevaMaquina(prog, nil)
	rellenaMaquina(m, e.Maquina, objs, arrs)
	return m, nil
}

// codeOffDeMetodo recupera el desplazamiento del code_item dentro de su propio dex. Es la pieza que
// permite no serializar el code_item: sus índices apuntan a los pools del fichero del que salió.
func codeOffDeMetodo(d *dexFile, clase, nombre, proto string) uint32 {
	if d == nil {
		return 0
	}
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		return 0
	}
	for _, em := range append(append([]encodedMethod{}, datos[clase].directs...), datos[clase].virtuals...) {
		if int(em.methodIdx) >= len(d.methods) {
			continue
		}
		ref := d.methods[em.methodIdx]
		if ref.name == nombre && descDeProto(ref.proto) == proto {
			return em.codeOff
		}
	}
	return 0
}

func decodificaValor(n nodoValor, objs []*objetoDalvik, arrs []*arregloDalvik) any {
	switch n.K {
	case "nulo":
		return nil
	case "obj":
		if n.R < 0 || n.R >= len(objs) {
			return nil
		}
		return objs[n.R]
	case "arr":
		if n.R < 0 || n.R >= len(arrs) {
			return nil
		}
		return arrs[n.R]
	case "i32":
		return int32(n.N)
	case "u32":
		return uint32(n.N)
	case "i64":
		return n.N
	case "f32":
		return float32(n.F)
	case "f64":
		return n.F
	case "cad":
		return n.S
	case "bool":
		return n.B
	case "lis":
		out := make([]any, 0, len(n.L))
		for _, hijo := range n.L {
			out = append(out, decodificaValor(hijo, objs, arrs))
		}
		return out
	case "map":
		out := map[string]any{}
		for k, hijo := range n.M {
			out[k] = decodificaValor(hijo, objs, arrs)
		}
		return out
	}
	return nil
}

func decodificaMapa(origen map[string]nodoValor, objs []*objetoDalvik, arrs []*arregloDalvik) map[string]any {
	out := map[string]any{}
	for k, v := range origen {
		out[k] = decodificaValor(v, objs, arrs)
	}
	return out
}

func decodificaLista(origen []nodoValor, objs []*objetoDalvik, arrs []*arregloDalvik) []any {
	var out []any
	for _, v := range origen {
		out = append(out, decodificaValor(v, objs, arrs))
	}
	return out
}

func rellenaArreglo(a *arregloDalvik, ae arregloEstado, objs []*objetoDalvik, arrs []*arregloDalvik) {
	a.tipo = ae.Tipo
	a.datos = decodificaLista(ae.Datos, objs, arrs)
}

func rellenaObjeto(o *objetoDalvik, oe objetoEstado, objs []*objetoDalvik, arrs []*arregloDalvik) {
	o.clase = oe.Clase
	o.campos = decodificaMapa(oe.Campos, objs, arrs)
	o.id = oe.IDRecurso
	o.texto = oe.Texto
	o.textoEscrito = oe.TextoEscrito
	o.tag = decodificaValor(oe.Tag, objs, arrs)
	if oe.Cadena != "" {
		o.cadena = &strings.Builder{}
		o.cadena.WriteString(oe.Cadena)
	}
	o.oyente = decodificaValor(oe.Oyente, objs, arrs)
	o.marcado = oe.Marcado
	o.marcadoPuesto = oe.MarcadoPuesto
	o.vigilantesTexto = decodificaLista(oe.Vigilantes, objs, arrs)
	o.oyenteMarca = decodificaValor(oe.OyenteMarca, objs, arrs)
	o.pista = oe.Pista
	for _, id := range oe.MenuItems {
		if id >= 0 && id < len(objs) {
			o.menuItems = append(o.menuItems, objs[id])
		}
	}
	o.itemID = oe.ItemID
	o.itemGrupo = oe.ItemGrupo
	o.itemOrden = oe.ItemOrden
	o.itemTitulo = oe.ItemTitulo
	o.itemVisible = oe.ItemVisible
	o.itemHabilitado = oe.ItemHabilitado
	o.itemMarcado = oe.ItemMarcado
	o.itemOyente = decodificaValor(oe.ItemOyente, objs, arrs)
	o.intentClase = oe.IntentClase
	o.bundle = decodificaMapa(oe.Bundle, objs, arrs)
	o.prefsFichero = oe.PrefsFichero
	o.editor = decodificaMapa(oe.Editor, objs, arrs)
	o.adaptador = decodificaValor(oe.Adaptador, objs, arrs)
	o.elementos = decodificaLista(oe.Elementos, objs, arrs)
	o.layoutID = oe.LayoutID
	o.vistas = map[uint32]*objetoDalvik{}
	for idVista, id := range oe.Vistas {
		if id >= 0 && id < len(objs) {
			o.vistas[idVista] = objs[id]
		}
	}
	o.mensajeExcepcion = oe.Mensaje
	o.lista = decodificaLista(oe.Lista, objs, arrs)
	o.conjunto = decodificaLista(oe.Conjunto, objs, arrs)
	o.iterando = decodificaLista(oe.Iterando, objs, arrs)
	o.indice = oe.Indice
	o.bluetoothActivo = oe.Bluetooth
	o.paintColor = oe.PaintColor
	o.paintPuesto = oe.PaintPuesto
	o.paintGrosor = oe.PaintGrosor
	o.anchoVista = oe.AnchoVista
	o.altoVista = oe.AltoVista
}

func rellenaMaquina(m *maquinaDalvik, me maquinaEstado, objs []*objetoDalvik, arrs []*arregloDalvik) {
	objeto := func(id int) *objetoDalvik {
		if id < 0 || id >= len(objs) {
			return nil
		}
		return objs[id]
	}
	m.pasos = me.Pasos
	m.avisos = append([]string(nil), me.Avisos...)
	m.vistos = mapaBool(me.Vistos)
	m.capturas = append([]capturaTexto(nil), me.Capturas...)
	m.consola = append([]lineaConsola(nil), me.Consola...)
	for _, reg := range me.Oyentes {
		m.oyentes = append(m.oyentes, oyenteRegistrado{
			oyente: objeto(reg.Oyente), vista: objeto(reg.Vista), metodo: reg.Metodo,
		})
	}
	for _, reg := range me.OyentesItem {
		m.oyentesItem = append(m.oyentesItem, oyenteItemRegistrado{
			oyente: objeto(reg.Oyente), vista: objeto(reg.Vista),
		})
	}
	m.interaccion = me.Interaccion
	m.profundidad = me.Profundidad
	m.actividad = objeto(me.Actividad)
	m.vistas = map[uint32]*objetoDalvik{}
	for idVista, id := range me.Vistas {
		m.vistas[idVista] = objeto(id)
	}
	m.textosDeLayout = mapaTextos(me.TextosDeLayout)
	m.tiposVista = mapaTextos(me.TiposVista)
	m.marcas = mapaMarcas(me.Marcas)
	m.pistas = mapaTextos(me.Pistas)
	m.marcasConocidas = mapaMarcas(me.MarcasConocidas)
	m.gruposRadio = map[uint32][]uint32{}
	for k, v := range me.GruposRadio {
		m.gruposRadio[k] = append([]uint32(nil), v...)
	}
	m.grupoDe = map[uint32]uint32{}
	for k, v := range me.GrupoDe {
		m.grupoDe[k] = v
	}
	for _, id := range me.Listas {
		if v := objeto(id); v != nil {
			m.listas = append(m.listas, v)
		}
	}
	m.layoutID = me.LayoutID
	m.pendiente = me.Pendiente
	m.terminada = me.Terminada
	m.arrancada = me.Arrancada
	m.declaradas = mapaBool(me.Declaradas)
	m.ciclo = append([]string(nil), me.Ciclo...)
	copia := append([]string(nil), me.Registro...)
	m.registro = &copia
	m.menu = objeto(me.Menu)
	m.menuAbierto = me.MenuAbierto
	m.menuSeleccionado = me.MenuSeleccionado
	m.intent = objeto(me.Intent)
	m.intentoSaliente = objeto(me.IntentoSaliente)
	m.bundleInicial = objeto(me.BundleInicial)
	for _, id := range me.Dialogos {
		if v := objeto(id); v != nil {
			m.dialogos = append(m.dialogos, v)
		}
	}
	m.toast = me.Toast
	m.estadoGuardado = decodificaMapa(me.EstadoGuardado, objs, arrs)
	reloj := me.Reloj
	m.reloj = &reloj
	m.almacen = nuevoAlmacenPreferencias()
	for fichero, datos := range me.Almacen {
		m.almacen.ficheros[fichero] = decodificaMapa(datos, objs, arrs)
	}
	m.tareas = me.Tareas
	m.encoladas = me.Encoladas
	m.cortada = me.Cortada
	m.pasosBase = me.PasosBase
}

// ---- utilidades de copia y de ids -----------------------------------------------------------

// idObjeto e idAny son la única puerta a la tabla: una referencia no registrada da -1, nunca el
// cero por defecto de un mapa (que apuntaría silenciosamente al primer objeto de la tabla).
func idObjeto(o *objetoDalvik, to *tablaObjetos) int {
	if o == nil {
		return -1
	}
	if id, hay := to.objs[o]; hay {
		return id
	}
	return -1
}

func idAny(v any, to *tablaObjetos) int {
	if o, ok := v.(*objetoDalvik); ok && o != nil {
		return idObjeto(o, to)
	}
	return -1
}

func derefRegistro(r *[]string) []string {
	if r == nil {
		return nil
	}
	return *r
}

func clavesString[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func clavesUint32[T any](m map[uint32]T) []uint32 {
	out := make([]uint32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func copiaStrings(v []string) []string {
	if v == nil {
		return nil
	}
	return append([]string(nil), v...)
}

func copiaBool(m map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copiaTextos(m map[uint32]string) map[uint32]string {
	out := map[uint32]string{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copiaMarcas(m map[uint32]bool) map[uint32]bool {
	out := map[uint32]bool{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copiaGrupos(m map[uint32][]uint32) map[uint32][]uint32 {
	out := map[uint32][]uint32{}
	for k, v := range m {
		out[k] = append([]uint32(nil), v...)
	}
	return out
}

func copiaGrupoDe(m map[uint32]uint32) map[uint32]uint32 {
	out := map[uint32]uint32{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func mapaBool(m map[string]bool) map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	return copiaBool(m)
}

func mapaTextos(m map[uint32]string) map[uint32]string {
	if m == nil {
		return map[uint32]string{}
	}
	return copiaTextos(m)
}

func mapaMarcas(m map[uint32]bool) map[uint32]bool {
	if m == nil {
		return map[uint32]bool{}
	}
	return copiaMarcas(m)
}
