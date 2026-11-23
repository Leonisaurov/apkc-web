package main

import (
	"fmt"
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
