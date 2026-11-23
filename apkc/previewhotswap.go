package main

import "fmt"

// El hot swap del preview: sustituir la implementación de un método en una sesión VIVA.
//
// La decisión de diseño es del usuario: la sesión conserva la identidad de sus objetos y el estado
// que ya tenían, y el código nuevo entra de forma aditiva sobre la tabla de despacho. No se reinicia
// la máquina ni se reconstruyen las instancias: `objetoDalvik` no guarda métodos, solo su clase y sus
// campos, así que cambiar el `metodoDalvik` que cuelga de `claseDalvik.metodos` basta para que la
// siguiente llamada ejecute el cuerpo nuevo y las referencias anteriores sigan siendo el mismo objeto.
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
	// poder deshacer el swap en una prueba o en la pestaña.
	Antes *metodoDalvik
	// Despues es la que quedó instalada, con el code_item del dex nuevo.
	Despues *metodoDalvik
}

// metodoDeDex busca en `d` el método `nombre`+`proto` de `clase` y lo devuelve listo para ejecutar,
// con su code_item leído y el dex del que salió anotado. No toca la sesión: es la lectura del módulo
// nuevo, separada del enganche, para que un dex que no trae el método falle antes de cambiar nada.
func metodoDeDex(d *dexFile, clase, nombre, proto string) (*metodoDalvik, error) {
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	cd, ok := datos[clase]
	if !ok {
		return nil, fmt.Errorf("el dex del swap no declara la clase %s", claseCorta(clase))
	}
	metodos := append(append([]encodedMethod{}, cd.directs...), cd.virtuals...)
	for _, m := range metodos {
		if int(m.methodIdx) >= len(d.methods) {
			return nil, fmt.Errorf("%s: el método %d no está en method_ids", claseCorta(clase), m.methodIdx)
		}
		ref := d.methods[m.methodIdx]
		if ref.name != nombre || descDeProto(ref.proto) != proto {
			continue
		}
		met := &metodoDalvik{
			nombre:  ref.name,
			proto:   proto,
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
	return nil, fmt.Errorf("el dex del swap no trae %s.%s%s", claseCorta(clase), nombre, proto)
}

// intercambiaMetodo instala en la sesión viva el `nombre`+`proto` que declara `nuevo`, sin recrear
// instancias ni tocar sus campos. Si el método ya estaba en la clase viva se sustituye y el
// resultado lo dice; si no estaba se añade. Los demás métodos de la clase y las demás clases quedan
// como estaban. Los estáticos de la clase conservan su valor actual: solo se siembran los que la
// sesión no conocía, para que un swap no borre el estado que la app ya tenía.
func (m *maquinaDalvik) intercambiaMetodo(nuevo *dexFile, clase, nombre, proto string) (resultadoIntercambio, error) {
	if m == nil || m.prog == nil {
		return resultadoIntercambio{}, fmt.Errorf("el swap necesita una sesión con programa cargado")
	}
	if nuevo == nil {
		return resultadoIntercambio{}, fmt.Errorf("el swap necesita el dex del código nuevo")
	}
	met, err := metodoDeDex(nuevo, clase, nombre, proto)
	if err != nil {
		return resultadoIntercambio{}, err
	}

	cl, ok := m.prog.clases[clase]
	if !ok {
		// una clase que la sesión no tenía: entra entera por la puerta aditiva, con su super para
		// que el despacho heredado siga subiendo por la jerarquía del dex nuevo
		super := ""
		for _, c := range nuevo.classes {
			if c.name == clase {
				super = c.super
				break
			}
		}
		cl = &claseDalvik{
			nombre:    clase,
			super:     super,
			metodos:   map[string]*metodoDalvik{},
			estaticos: map[string]any{},
		}
		m.prog.clases[clase] = cl
	}

	antes := cl.metodos[nombre+proto]
	cl.metodos[nombre+proto] = met

	// los estáticos nuevos, solo los que faltan: la identidad del estado es parte del contrato
	datos, err := nuevo.methodsDeCadaClase()
	if err == nil {
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

	return resultadoIntercambio{
		Clase:   clase,
		Metodo:  nombre,
		Proto:   proto,
		Existia: antes != nil,
		Antes:   antes,
		Despues: met,
	}, nil
}

// intercambiaClase instala todos los métodos que `nuevo` declara para `clase`: es la forma natural de
// la pestaña, que recompila un fichero y quiere que la clase entera responda con lo nuevo. Cada
// método se engancha con `intercambiaMetodo`, así que la identidad y los estáticos se conservan
// método a método y un fallo a mitad no deja la tabla a medias: se lee y se engancha de uno en uno.
func (m *maquinaDalvik) intercambiaClase(nuevo *dexFile, clase string) ([]resultadoIntercambio, error) {
	datos, err := nuevo.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	cd, ok := datos[clase]
	if !ok {
		return nil, fmt.Errorf("el dex del swap no declara la clase %s", claseCorta(clase))
	}
	metodos := append(append([]encodedMethod{}, cd.directs...), cd.virtuals...)
	hechos := make([]resultadoIntercambio, 0, len(metodos))
	for _, em := range metodos {
		if int(em.methodIdx) >= len(nuevo.methods) {
			return hechos, fmt.Errorf("%s: el método %d no está en method_ids", claseCorta(clase), em.methodIdx)
		}
		ref := nuevo.methods[em.methodIdx]
		r, err := m.intercambiaMetodo(nuevo, clase, ref.name, descDeProto(ref.proto))
		if err != nil {
			return hechos, err
		}
		hechos = append(hechos, r)
	}
	return hechos, nil
}
