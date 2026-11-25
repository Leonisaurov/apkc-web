package main

// El dibujo de una vista propia dentro de la vista previa. El modelo de la Fase 1 pinta el
// rectángulo de una clase que no es del framework, pero su `onDraw` no se ejecutaba: la pantalla se
// veía vacía y nada lo decía. Aquí se ejecuta el `onDraw` del proyecto con un Canvas que anota las
// operaciones que el intérprete sí sabe medir (color de fondo, línea, rectángulo, círculo y texto).
//
// La frontera se dice con nombre, no se disfraza: los campos que la actividad escribió en la vista
// real (un `findViewById` con un `setToques` desde el `onClick`) se heredan antes de dibujar, pero
// el Canvas y el Paint son nuevos en cada dibujado; y un `<vector>`, una fuente o un `Paint` con
// sombras no se dibujan. Cuando el `onDraw` no se puede ejecutar, `dibujo.frontera` lleva el motivo
// y no se inventa ningún trazo.

import (
	"fmt"
	"strings"
)

// dibujoVista es lo que una vista propia dibujó en el Canvas del modelo. `operaciones` va en el
// orden en que la app las pidió; `frontera` dice, cuando hay algo que el intérprete no puede medir,
// qué quedó fuera y por qué.
type dibujoVista struct {
	Operaciones []operacionDibujo `json:"operaciones,omitempty"`
	Frontera    string            `json:"frontera,omitempty"`
}

// operacionDibujo es una llamada al Canvas que el modelo puede publicar sin inventar geometría. Las
// coordenadas son las que pasó la app, en píxeles del modelo (la pantalla de 160 dpi). `x1`/`y1` es
// el punto de una línea, la esquina del rectángulo o el centro del círculo, y `x2`/`y2` el otro
// extremo; `radio` solo lo lleva el círculo y `texto`/`tamano` el `drawText`.
type operacionDibujo struct {
	Operacion string  `json:"operacion"`
	Color     string  `json:"color,omitempty"`
	Grosor    float64 `json:"grosor,omitempty"`
	X1        float64 `json:"x1,omitempty"`
	Y1        float64 `json:"y1,omitempty"`
	X2        float64 `json:"x2,omitempty"`
	Y2        float64 `json:"y2,omitempty"`
	Radio     float64 `json:"radio,omitempty"`
	Texto     string  `json:"texto,omitempty"`
	Tamano    float64 `json:"tamano,omitempty"`
}

// anotaDibujo guarda una operación del Canvas en el dibujo en curso. Fuera de un `onDraw`
// (`m.dibujo` nil) no hace nada: los stubs de Canvas no tienen efecto sobre el modelo normal.
func (m *maquinaDalvik) anotaDibujo(op operacionDibujo) {
	if m == nil || m.dibujo == nil {
		return
	}
	m.dibujo.Operaciones = append(m.dibujo.Operaciones, op)
}

// enteroDeDibujo lee un color que llega como entero del registro. Un `const` de color del dex puede
// estar como int32 o como int64 después de una extensión; las dos formas valen.
func enteroDeDibujo(v any) (uint32, bool) {
	switch x := v.(type) {
	case int32:
		return uint32(x), true
	case int64:
		return uint32(x), true
	case float32:
		return uint32(x), true
	}
	return 0, false
}

// flotanteDeDibujo lee una coordenada. Un registro no está tipado: el patrón de bits de un `const`
// es un float y el de un `move` puede conservarse como int32; `aFloat32` ya sabe leer las dos.
func flotanteDeDibujo(v any) float64 {
	if f, err := aFloat32(v); err == nil {
		return float64(f)
	}
	if i, err := aInt64(v); err == nil {
		return float64(i)
	}
	return 0
}

// colorDePaint devuelve el ARGB del Paint; un Paint que nadie tocó es negro, como el del framework.
func colorDePaint(p *objetoDalvik) uint32 {
	if p == nil || !p.paintPuesto {
		return 0xff000000
	}
	return p.paintColor
}

// colorDeArgumento resuelve el color del Paint que un `drawX` recibió.
func colorDeArgumento(v any) string {
	if p, ok := v.(*objetoDalvik); ok {
		return formatoColor(colorDePaint(p))
	}
	return formatoColor(0xff000000)
}

// grosorDeArgumento es el `strokeWidth` del Paint que un `drawX` recibió.
func grosorDeArgumento(v any) float64 {
	if p, ok := v.(*objetoDalvik); ok {
		return p.paintGrosor
	}
	return 0
}

// tamanoDeArgumento es el `textSize` del Paint que un `drawText` recibió; un Paint que nadie
// ajustó no publica tamaño, que es lo que el modelo hacía antes de que `setTextSize` existiera.
func tamanoDeArgumento(v any) float64 {
	if p, ok := v.(*objetoDalvik); ok {
		return p.paintTamano
	}
	return 0
}

// pintaVistasPropias busca las vistas del proyecto en el modelo y ejecuta su `onDraw`. `paquete` es
// el del manifiesto: solo una clase de ese paquete se considera propia, para no marcar como frontera
// las vistas de AndroidX que el apk trae compiladas. Un fallo del `onDraw` no aborta el modelo: se
// publica en `dibujo.frontera`.
func pintaVistasPropias(n *nodoVista, res *resultadoEjecucion, paquete string) {
	if n == nil || paquete == "" {
		return
	}
	if strings.HasPrefix(n.Tipo, paquete+".") {
		switch {
		case res == nil || res.maquina == nil:
			motivo := "no se ejecutó onDraw: el apk no trae un classes.dex ejecutable"
			if res != nil && res.Error != "" {
				motivo = "no se ejecutó onDraw: " + res.Error
			}
			n.Dibujo = &dibujoVista{Frontera: motivo}
		default:
			if d := res.maquina.dibujaVista(n.Tipo, n.Rect); d != nil {
				n.Dibujo = d
			}
		}
	}
	for _, h := range n.Hijos {
		pintaVistasPropias(h, res, paquete)
	}
}

// dibujaVista ejecuta el `onDraw(Landroid/graphics/Canvas;)V` de la clase con un Canvas que anota
// las operaciones. `rect` es el rectángulo que el layout ya midió para el nodo: se siembra en la
// instancia nueva para que un `onDraw` que llame a `getWidth`/`getHeight` vea el tamaño real y no
// dibuje todo en el origen. Devuelve nil cuando la clase no es del dex o no declara onDraw: entonces
// no hay nada que decir y el nodo se queda como estaba. El Canvas es un objeto del modelo, no un
// lienzo real: no hay fuentes, ni sombras, ni mapas de bits.
func (m *maquinaDalvik) dibujaVista(nombre string, rect rectVista) *dibujoVista {
	if m == nil || m.prog == nil {
		return nil
	}
	desc := "L" + strings.ReplaceAll(nombre, ".", "/") + ";"
	if _, ok := m.prog.clases[desc]; !ok {
		return &dibujoVista{Frontera: fmt.Sprintf(
			"la clase %s no está en classes.dex: no se ejecuta onDraw", nombre)}
	}
	met := m.buscaMetodo(desc, "onDraw", "(Landroid/graphics/Canvas;)V")
	if met == nil {
		return nil
	}
	dib := &dibujoVista{}
	m.dibujo = dib
	defer func() { m.dibujo = nil }()
	// el dibujo tiene su propio presupuesto de pasos, como una tarea de la cola: un onDraw que se
	// cuelgue se corta con el mismo tope y se publica como frontera, sin dejar el modelo a medias
	base := m.pasosBase
	m.pasosBase = m.pasos
	defer func() { m.pasosBase = base }()

	obj := &objetoDalvik{clase: desc, campos: map[string]any{},
		anchoVista: int32(rect.Ancho), altoVista: int32(rect.Alto)}
	if err := m.construyeVistaPropia(desc, obj); err != nil {
		dib.Frontera = "no se pudo construir la vista " + nombre + ": " + err.Error()
		return dib
	}
	m.heredaEstadoDeVista(desc, obj)
	canvas := &objetoDalvik{clase: "Landroid/graphics/Canvas;", campos: map[string]any{}}
	if _, err := m.ejecutaCuerpo(desc, met, []any{obj, canvas},
		[]string{desc, "Landroid/graphics/Canvas;"}); err != nil {
		dib.Frontera = "onDraw no terminó en " + nombre + ": " + err.Error()
	}
	if len(dib.Operaciones) == 0 && dib.Frontera == "" {
		dib.Frontera = "onDraw no dibujó ninguna operación medida (drawColor, drawLine, drawRect, " +
			"drawCircle o drawText)"
	}
	return dib
}

// heredaEstadoDeVista pasa a la instancia que va a dibujar los campos que la actividad ya escribió
// en la vista real. El `findViewById` del modelo devuelve una vista genérica a la que un
// `check-cast` solo le cambia la clase: su constructor no corrió, así que lo único que hay en sus
// campos es lo que la app puso después (un `setToques` del `onClick`, por ejemplo) y no un estado
// inventado. Se copia DESPUÉS del constructor para que el Paint que este crea siga existiendo y
// los valores que la app acaba de cambiar no se pierdan. Sin esto el `onDraw` se dibujaba siempre
// con el estado inicial y un toque no cambiaba lo que se veía.
func (m *maquinaDalvik) heredaEstadoDeVista(desc string, obj *objetoDalvik) {
	var real *objetoDalvik
	for id, v := range m.vistas {
		if v == nil || v.clase != desc {
			continue
		}
		if real == nil || id < real.id {
			real = v
		}
	}
	if real == nil {
		return
	}
	for clave, valor := range real.campos {
		obj.campos[clave] = valor
	}
}

// construyeVistaPropia ejecuta el constructor de la clase con las formas que un inflado puede usar:
// sin argumentos, con el Context de la actividad y con el AttributeSet nulo. El AttributeSet no se
// simula: una vista que lea `android:...` del XML en su constructor recibirá nil, y eso queda como
// frontera si el constructor falla.
func (m *maquinaDalvik) construyeVistaPropia(desc string, obj *objetoDalvik) error {
	cl := m.prog.clases[desc]
	if cl == nil {
		return nil
	}
	contexto := any(nil)
	if m.actividad != nil {
		contexto = m.actividad
	}
	intentos := []struct {
		proto string
		args  []any
		tipos []string
	}{
		{"()V", []any{obj}, []string{desc}},
		{"(Landroid/content/Context;)V", []any{obj, contexto}, []string{desc, "Landroid/content/Context;"}},
		{"(Landroid/content/Context;Landroid/util/AttributeSet;)V",
			[]any{obj, contexto, nil}, []string{desc, "Landroid/content/Context;", "Landroid/util/AttributeSet;"}},
		{"(Landroid/content/Context;Landroid/util/AttributeSet;I)V",
			[]any{obj, contexto, nil, int32(0)},
			[]string{desc, "Landroid/content/Context;", "Landroid/util/AttributeSet;", "I"}},
	}
	for _, intento := range intentos {
		ctor := cl.metodos["<init>"+intento.proto]
		if ctor == nil {
			continue
		}
		_, err := m.ejecutaCuerpo(desc, ctor, intento.args, intento.tipos)
		return err
	}
	// sin constructor en el dex (una clase sin `<init>` declarado): el objeto vacío sirve para un
	// onDraw que no lea estado de la instancia
	return nil
}
