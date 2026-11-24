package main

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// El hot swap del preview: sustituir la implementación de un método en una sesión VIVA.
//
// La decisión de diseño es del usuario: la sesión conserva la identidad de sus objetos y el estado
// que ya tenían, y el código nuevo entra de forma aditiva sobre la tabla de despacho. No se reinicia
// la máquina ni se reconstruyen las instancias: `objetoDalvik` no guarda métodos, solo su clase y sus
// campos, así que cambiar el `metodoDalvik` que cuelga de `claseDalvik.metodos` basta para que la
// siguiente llamada ejecute el cuerpo nuevo y las referencias anteriores sigan siendo el mismo objeto.
//
// Atomicidad: la única operación que puede fallar es LEER el dex del módulo nuevo (class_data, el
// code_item de cada método), así que `intercambiaClase` lee y valida todos sus métodos antes de tocar
// la tabla y después instala, que ya no falla. El resultado es todo o nada sin necesitar deshacer:
// un módulo a medias no deja la clase con unos métodos nuevos y otros viejos.
//
// Frontera declarada, que la primitiva NO cruza: la forma de los campos. Los campos de una instancia
// son un mapa por `dueño.nombre` y la clase viva no guarda su tabla, así que un cuerpo nuevo que lea
// un campo que la app nunca declaró recibe su cero, y no se detecta aquí: es una diferencia con el
// aparato que el modelo tiene que decir, no fingir. Tampoco se reemplazan las clases del framework.

// resultadoIntercambio dice qué hizo un swap sobre la tabla de despacho de la sesión.
type resultadoIntercambio struct {
	Clase   string `json:"clase"`
	Metodo  string `json:"metodo"`
	Proto   string `json:"proto"`
	Existia bool   `json:"existia"`
	// Antes es la implementación que se sustituyó, nil cuando el método se añadió. Se devuelve para
	// poder deshacer el swap en una prueba o en la pestaña, pero no viaja al JSON: un `metodoDalvik`
	// lleva su code_item y su dex, y serializarlo ensucia `ejecucion.intercambios` con `{}`.
	Antes *metodoDalvik `json:"-"`
	// Despues es la que quedó instalada, con el code_item del dex nuevo; tampoco viaja al JSON.
	Despues *metodoDalvik `json:"-"`
}

// metodoDeClase busca en `datos` (los class_data_item de `d`, ya leídos) el método `nombre`+`proto`
// de `clase` y lo devuelve listo para ejecutar, con su code_item leído y el dex del que salió
// anotado. No toca la sesión: es la lectura del módulo nuevo, separada del enganche, para que un dex
// que no trae el método falle antes de cambiar nada.
//
// Con `proto` vacío resuelve por nombre, que es la forma `clase/metodo` del flag: solo vale si la
// clase declara un único método con ese nombre. Si hay sobrecargas se rechaza pidiendo el proto
// completo, porque elegir una al azar cambiaría un método distinto del que el usuario cree.
func metodoDeClase(d *dexFile, datos map[string]classData, clase, nombre, proto string) (*metodoDalvik, error) {
	cd, ok := datos[clase]
	if !ok {
		return nil, fmt.Errorf("el dex del swap no declara la clase %s", claseCorta(clase))
	}
	metodos := append(append([]encodedMethod{}, cd.directs...), cd.virtuals...)
	elegidos := make([]int, 0, 1)
	for i, m := range metodos {
		if int(m.methodIdx) >= len(d.methods) {
			return nil, fmt.Errorf("%s: el método %d no está en method_ids", claseCorta(clase), m.methodIdx)
		}
		ref := d.methods[m.methodIdx]
		if ref.name != nombre {
			continue
		}
		if proto != "" && descDeProto(ref.proto) != proto {
			continue
		}
		elegidos = append(elegidos, i)
	}
	if len(elegidos) == 0 {
		return nil, fmt.Errorf("el dex del swap no trae %s.%s%s", claseCorta(clase), nombre, proto)
	}
	if len(elegidos) > 1 {
		vistos := make([]string, 0, len(elegidos))
		for _, i := range elegidos {
			vistos = append(vistos, descDeProto(d.methods[metodos[i].methodIdx].proto))
		}
		return nil, fmt.Errorf("el dex del swap trae %s.%s sobrecargado (%s): pide el proto completo, "+
			"como %s.%s%s", claseCorta(clase), nombre, strings.Join(vistos, ", "),
			claseCorta(clase), nombre, vistos[0])
	}
	m := metodos[elegidos[0]]
	ref := d.methods[m.methodIdx]
	met := &metodoDalvik{
		nombre:  ref.name,
		proto:   descDeProto(ref.proto),
		retorno: ref.proto.returnType,
		acceso:  m.accessFlags,
		dex:     d,
	}
	if m.codeOff != 0 {
		item, err := d.readCodeItem(m.codeOff)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", claseCorta(clase), nombre, err)
		}
		met.item = &item
	}
	return met, nil
}

// metodoDeDex es `metodoDeClase` leyendo primero los class_data_item del fichero.
func metodoDeDex(d *dexFile, clase, nombre, proto string) (*metodoDalvik, error) {
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	return metodoDeClase(d, datos, clase, nombre, proto)
}

// claseParaSwap devuelve la clase viva y, si la sesión no la tenía, la crea vacía con su super para
// que el despacho heredado siga subiendo por la jerarquía del dex nuevo. Es la puerta aditiva.
func (m *maquinaDalvik) claseParaSwap(nuevo *dexFile, clase string) *claseDalvik {
	if cl, ok := m.prog.clases[clase]; ok {
		return cl
	}
	super := ""
	for _, c := range nuevo.classes {
		if c.name == clase {
			super = c.super
			break
		}
	}
	cl := &claseDalvik{
		nombre:    clase,
		super:     super,
		metodos:   map[string]*metodoDalvik{},
		estaticos: map[string]any{},
	}
	m.prog.clases[clase] = cl
	return cl
}

// instalaMetodo engancha `met` en la tabla de despacho de la clase y devuelve qué hizo. No puede
// fallar: la lectura y la validación ya pasaron en `metodoDeDex`.
func instalaMetodo(cl *claseDalvik, met *metodoDalvik) resultadoIntercambio {
	antes := cl.metodos[met.nombre+met.proto]
	cl.metodos[met.nombre+met.proto] = met
	return resultadoIntercambio{
		Clase:   cl.nombre,
		Metodo:  met.nombre,
		Proto:   met.proto,
		Existia: antes != nil,
		Antes:   antes,
		Despues: met,
	}
}

// siembraEstaticos añade a la clase viva los estáticos del módulo que la sesión no conocía. Los que
// ya estaban conservan su valor: la identidad del estado es parte del contrato, y un swap no puede
// devolver a su constante un contador que la app ya movió.
func siembraEstaticos(cl *claseDalvik, datos map[string]classData, nuevo *dexFile, clase string) {
	for _, f := range datos[clase].staticFields {
		if int(f.fieldIdx) >= len(nuevo.fields) {
			continue
		}
		clave := claveCampo(nuevo.fields[f.fieldIdx])
		if _, hay := cl.estaticos[clave]; !hay {
			cl.estaticos[clave] = valorCodificado(f.valueType, f.value)
		}
	}
}

// intercambiaMetodo instala en la sesión viva el `nombre`+`proto` que declara `nuevo`, sin recrear
// instancias ni tocar sus campos. Si el método ya estaba en la clase viva se sustituye y el
// resultado lo dice; si no estaba se añade. Los demás métodos de la clase y las demás clases quedan
// como estaban, y los estáticos conservan su valor actual.
func (m *maquinaDalvik) intercambiaMetodo(nuevo *dexFile, clase, nombre, proto string) (resultadoIntercambio, error) {
	if m == nil || m.prog == nil {
		return resultadoIntercambio{}, fmt.Errorf("el swap necesita una sesión con programa cargado")
	}
	if nuevo == nil {
		return resultadoIntercambio{}, fmt.Errorf("el swap necesita el dex del código nuevo")
	}
	datos, err := nuevo.methodsDeCadaClase()
	if err != nil {
		return resultadoIntercambio{}, err
	}
	met, err := metodoDeClase(nuevo, datos, clase, nombre, proto)
	if err != nil {
		return resultadoIntercambio{}, err
	}
	cl := m.claseParaSwap(nuevo, clase)
	r := instalaMetodo(cl, met)
	siembraEstaticos(cl, datos, nuevo, clase)
	return r, nil
}

// intercambiaClase instala todos los métodos que `nuevo` declara para `clase`: es la forma natural de
// la pestaña, que recompila un fichero y quiere que la clase entera responda con lo nuevo.
//
// Es todo o nada. Primero se leen y se validan los code_item de todos los métodos del módulo —el
// único paso que puede fallar: un class_data roto, un method_idx fuera de method_ids, un code_item
// que no cabe—; solo si todos salen se toca la tabla, y entonces ya no hay error posible. Si la
// lectura falla, la sesión queda exactamente como estaba y no hay nada que deshacer.
// ---- delta de dexes: alta / cambio / baja ---------------------------------------------------

// firmaMetodo identifica un método por su classe y su nombre+proto, y lleva el hash de sus
// instrucciones. Dos firmas con la misma clave y distinto hash son un cuerpo cambiado; una clave
// que solo está en un lado es un alta o una baja. El hash va sobre las palabras del code_item, no
// sobre los bytes del fichero: así un reordenamiento de los pools no cuenta como cambio de código.
type firmaMetodo struct {
	Clase  string
	Nombre string
	Proto  string
	Hash   uint64
}

func (f firmaMetodo) clave() string { return f.Clase + "\x00" + f.Nombre + f.Proto }

// cambioDex es una operación del delta. `Vista` es el id de la vista ancla de un `baja`: la limpieza
// de oyentes se hace por ese id (que es estable) y no por el ordinal `$N` del anónimo de javac, que
// cambia al borrar una clase intermedia. Cero significa «todas las vistas del oyente» y es lo que
// sale del delta cuando el que lo aplica no conoce el layout.
type cambioDex struct {
	Tipo   string `json:"tipo"`
	Clase  string `json:"clase"`
	Nombre string `json:"metodo"`
	Proto  string `json:"proto"`
	Vista  uint32 `json:"vista,omitempty"`
}

// firmasDeDex lee todos los métodos con cuerpo de `d` y devuelve su firma, en el orden de las
// class_defs del fichero para que el delta sea reproducible.
func firmasDeDex(d *dexFile) ([]firmaMetodo, error) {
	if d == nil {
		return nil, fmt.Errorf("no hay dex del que calcular firmas")
	}
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	firmas := make([]firmaMetodo, 0)
	for _, clase := range d.classes {
		cd := datos[clase.name]
		for _, em := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			if int(em.methodIdx) >= len(d.methods) {
				return nil, fmt.Errorf("%s: el método %d no está en method_ids",
					claseCorta(clase.name), em.methodIdx)
			}
			ref := d.methods[em.methodIdx]
			f := firmaMetodo{Clase: clase.name, Nombre: ref.name, Proto: descDeProto(ref.proto)}
			if em.codeOff != 0 {
				item, err := d.readCodeItem(em.codeOff)
				if err != nil {
					return nil, fmt.Errorf("%s.%s%s: %w", claseCorta(clase.name), ref.name, f.Proto, err)
				}
				h := fnv.New64a()
				var palabra [2]byte
				for _, ins := range item.insns {
					palabra[0] = byte(ins)
					palabra[1] = byte(ins >> 8)
					_, _ = h.Write(palabra[:])
				}
				f.Hash = h.Sum64()
			}
			firmas = append(firmas, f)
		}
	}
	return firmas, nil
}

// deltaDeFirmas compara dos listas de firmas. Es el corazón del delta y se prueba con firmas armadas
// a mano, sin depender de cómo renumbere javac las clases anónimas al borrar una intermedia.
func deltaDeFirmas(viejas, nuevas []firmaMetodo) []cambioDex {
	porClave := func(fs []firmaMetodo) map[string]firmaMetodo {
		m := make(map[string]firmaMetodo, len(fs))
		for _, f := range fs {
			m[f.clave()] = f
		}
		return m
	}
	a := porClave(viejas)
	b := porClave(nuevas)
	cambios := make([]cambioDex, 0)
	for _, f := range viejas {
		n, hay := b[f.clave()]
		switch {
		case !hay:
			cambios = append(cambios, cambioDex{Tipo: "baja", Clase: f.Clase, Nombre: f.Nombre, Proto: f.Proto})
		case n.Hash != f.Hash:
			cambios = append(cambios, cambioDex{Tipo: "cambio", Clase: f.Clase, Nombre: f.Nombre, Proto: f.Proto})
		}
	}
	for _, f := range nuevas {
		if _, hay := a[f.clave()]; !hay {
			cambios = append(cambios, cambioDex{Tipo: "alta", Clase: f.Clase, Nombre: f.Nombre, Proto: f.Proto})
		}
	}
	return cambios
}

// deltaDeDexes calcula los cambios entre el dex de la sesión y el recién compilado.
func deltaDeDexes(viejo, nuevo *dexFile) ([]cambioDex, error) {
	v, err := firmasDeDex(viejo)
	if err != nil {
		return nil, fmt.Errorf("el dex de la sesión no se puede firmar: %w", err)
	}
	n, err := firmasDeDex(nuevo)
	if err != nil {
		return nil, fmt.Errorf("el dex nuevo no se puede firmar: %w", err)
	}
	return deltaDeFirmas(v, n), nil
}

// esDeClase y esDeVista son el anclaje de un oyente. La vista se compara por su id de recurso, no
// por el nombre de la clase anónima ni por su posición en la lista: ni el `$N` de javac ni un índice
// sobreviven a borrar un anónimo intermedio.
func esDeClase(oyente any, clase string) bool {
	o, ok := oyente.(*objetoDalvik)
	return ok && o != nil && o.clase == clase
}

func esDeVista(vista any, idVista uint32) bool {
	v, ok := vista.(*objetoDalvik)
	return ok && v != nil && v.id == idVista
}

// retiraMetodo saca un método de la tabla de despacho de una clase viva. No toca las instancias: su
// único efecto es que la siguiente llamada a ese nombre+proto ya no lo encuentre en la clase.
func (m *maquinaDalvik) retiraMetodo(clase, nombre, proto string) {
	if m == nil || m.prog == nil {
		return
	}
	if cl, ok := m.prog.clases[clase]; ok {
		delete(cl.metodos, nombre+proto)
	}
}

// retiraOyentesDeClase limpia las TRES listas donde un `setOnClickListener`/`setOnItemClickListener`
// deja su rastro: el registro de la máquina (`m.oyentes`), el campo `oyente` de la vista que se
// registró (que es lo que `pulsa` vuelve a leer al sustituir un listener) y el registro de listas
// (`m.oyentesItem`).
//
// Contrato de anclaje: CON idVista distinto de cero el ancla es SOLO el id de la vista; el nombre de
// clase no se consulta, porque el ordinal `$N` del anónimo de javac cambia al borrar una clase
// intermedia. SIN idVista se cae a la clase, que es lo único que se conoce entonces. Así la baja
// retira al oyente de la vista correcta aunque la sesión viva y el dex nuevo llamen distinto al
// anónimo. Solo se borra el campo `oyente` de la vista cuando apunta exactamente al oyente que se
// va, para no pisar un listener programático posterior que la app haya puesto en esa misma vista.
func (m *maquinaDalvik) retiraOyentesDeClase(clase string, idVista uint32) {
	if m == nil {
		return
	}
	quita := func(oyente, vista any) bool {
		if idVista != 0 {
			return esDeVista(vista, idVista)
		}
		return esDeClase(oyente, clase)
	}
	vivos := m.oyentes[:0]
	for _, reg := range m.oyentes {
		if quita(reg.oyente, reg.vista) {
			if v, ok := reg.vista.(*objetoDalvik); ok && v.oyente == reg.oyente {
				v.oyente = nil
			}
			continue
		}
		vivos = append(vivos, reg)
	}
	m.oyentes = vivos

	vivosItem := m.oyentesItem[:0]
	for _, reg := range m.oyentesItem {
		if quita(reg.oyente, reg.vista) {
			continue
		}
		vivosItem = append(vivosItem, reg)
	}
	m.oyentesItem = vivosItem
}

// claseDelOyenteEnVista resuelve la clase VIVA del oyente registrado sobre una vista. Bajo una
// renumeración real de javac el `$N` del dex nuevo ya no es el de la sesión: el id de la vista es lo
// estable y permite retirar el método de la clase que la sesión tiene de verdad.
func claseDelOyenteEnVista(m *maquinaDalvik, idVista uint32, porDefecto string) string {
	for _, reg := range m.oyentes {
		if esDeVista(reg.vista, idVista) {
			if o, ok := reg.oyente.(*objetoDalvik); ok && o != nil && o.clase != "" {
				return o.clase
			}
		}
	}
	return porDefecto
}

// aplicaBaja retira un método y limpia sus oyentes. Con id de vista, la clase se resuelve desde el
// oyente vivo; sin él se usa la que trae el delta.
func (m *maquinaDalvik) aplicaBaja(c cambioDex) {
	clase := c.Clase
	if c.Vista != 0 {
		clase = claseDelOyenteEnVista(m, c.Vista, c.Clase)
	}
	m.retiraMetodo(clase, c.Nombre, c.Proto)
	m.retiraOyentesDeClase(clase, c.Vista)
}

// aplicaDelta es TODO O NADA. Primero valida los tipos y LEE todos los altas y cambios del dex nuevo
// (el único paso que puede fallar); solo si todos salen se toca la tabla de despacho, y las bajas se
// aplican AL FINAL. Así un alta o un cambio que no se puede leer no deja la sesión con el método viejo
// y sus oyentes ya retirados. No hay recarga silenciosa: cualquier error se devuelve sin mutar.
func (m *maquinaDalvik) aplicaDelta(cambios []cambioDex, nuevo *dexFile) ([]resultadoIntercambio, error) {
	if m == nil || m.prog == nil {
		return nil, fmt.Errorf("el delta necesita una sesión con programa cargado")
	}
	altas := make([]cambioDex, 0, len(cambios))
	bajas := make([]cambioDex, 0, len(cambios))
	for _, c := range cambios {
		switch c.Tipo {
		case "alta", "cambio":
			altas = append(altas, c)
		case "baja":
			bajas = append(bajas, c)
		default:
			return nil, fmt.Errorf("el delta trae un tipo de cambio desconocido: %q", c.Tipo)
		}
	}
	if len(altas) == 0 {
		for _, c := range bajas {
			m.aplicaBaja(c)
		}
		return nil, nil
	}
	if nuevo == nil {
		return nil, fmt.Errorf("el delta necesita el dex del código nuevo")
	}

	// 1) leer y validar todo antes de tocar la sesión
	datos, err := nuevo.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	type listo struct {
		cambio cambioDex
		met    *metodoDalvik
	}
	validados := make([]listo, 0, len(altas))
	for _, c := range altas {
		met, err := metodoDeClase(nuevo, datos, c.Clase, c.Nombre, c.Proto)
		if err != nil {
			return nil, fmt.Errorf("%s %s.%s%s: %w", c.Tipo, claseCorta(c.Clase),
				c.Nombre, c.Proto, err)
		}
		validados = append(validados, listo{cambio: c, met: met})
	}

	// 2) instalar: desde aquí no hay error posible
	hechos := make([]resultadoIntercambio, 0, len(validados))
	sembradas := map[string]bool{}
	for _, v := range validados {
		cl := m.claseParaSwap(nuevo, v.cambio.Clase)
		hechos = append(hechos, instalaMetodo(cl, v.met))
		if !sembradas[v.cambio.Clase] {
			siembraEstaticos(cl, datos, nuevo, v.cambio.Clase)
			sembradas[v.cambio.Clase] = true
		}
	}

	// 3) las bajas, al final, con la clase resuelta desde la vista viva
	for _, c := range bajas {
		m.aplicaBaja(c)
	}
	return hechos, nil
}

func (m *maquinaDalvik) intercambiaClase(nuevo *dexFile, clase string) ([]resultadoIntercambio, error) {
	if m == nil || m.prog == nil {
		return nil, fmt.Errorf("el swap necesita una sesión con programa cargado")
	}
	if nuevo == nil {
		return nil, fmt.Errorf("el swap necesita el dex del código nuevo")
	}
	datos, err := nuevo.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	cd, ok := datos[clase]
	if !ok {
		return nil, fmt.Errorf("el dex del swap no declara la clase %s", claseCorta(clase))
	}
	metodos := append(append([]encodedMethod{}, cd.directs...), cd.virtuals...)

	// 1) leer y validar todo antes de tocar la sesión
	listos := make([]*metodoDalvik, 0, len(metodos))
	for _, em := range metodos {
		if int(em.methodIdx) >= len(nuevo.methods) {
			return nil, fmt.Errorf("%s: el método %d no está en method_ids", claseCorta(clase), em.methodIdx)
		}
		ref := nuevo.methods[em.methodIdx]
		met, err := metodoDeClase(nuevo, datos, clase, ref.name, descDeProto(ref.proto))
		if err != nil {
			return nil, err
		}
		listos = append(listos, met)
	}

	// 2) instalar: desde aquí no hay fallo que pueda dejar la clase a medias
	cl := m.claseParaSwap(nuevo, clase)
	hechos := make([]resultadoIntercambio, 0, len(listos))
	for _, met := range listos {
		hechos = append(hechos, instalaMetodo(cl, met))
	}
	siembraEstaticos(cl, datos, nuevo, clase)
	return hechos, nil
}
