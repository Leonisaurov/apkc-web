package main

// Fase 2 de la vista previa: un intérprete Dalvik mínimo en Go, sobre el lector de dex que este fork
// ya tenía (dexreader.go, dexcode.go) y sin ningún proceso ni runtime ajeno. Ejecuta el `onCreate`
// de la actividad lanzadora y recoge el texto que la app escribe en pantalla.
//
// Lo que hace, dicho con precisión para que la frontera no se lea como un fallo:
//   - decodifica las instrucciones sobre las palabras del code_item que ya leía dexcode.go, con una
//     máquina de registros de 32 bits (un `long` o un `double` ocupa dos, como en Dalvik);
//   - resuelve llamadas a las clases del propio dex (constructores incluidos) y stubs de framework
//     para lo que la app toca: `Activity.onCreate`, `setContentView`, `findViewById`,
//     `View.setOnClickListener`, `TextView.setText`, `StringBuilder`, `Log` y `println`;
//   - tras `onCreate` pulsa una vez cada listener registrado. Es lo único que hace falta para que la
//     plantilla Contador escriba: su `setText` vive en el `onClick`, no en el `onCreate`.
//
// Lo que NO hace: JNI, bibliotecas `.so`, multidex, excepciones (`try`/`catch` se rechazan por
// nombre), `fill-array-data` y cualquier API de framework fuera de los stubs. Una llamada que no se
// sabe resolver no se inventa: se devuelve el cero de su tipo y se anota un aviso en el JSON, que es
// lo único que impide que una aproximación se confunda con una ejecución completa.
//
// Es código propio (MIT). NekoDroid se leyó como referencia de comportamiento, no de código, y
// WineDroid (GPL) no se ha traducido ni copiado.

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	pasosMaximos            = 200000
	capturasMaximas         = 64
	avisosMaximos           = 24
	profundidadMaximaDalvik = 48
	interaccionesMax        = 4
)

// capturaTexto es una escritura que el intérprete pudo ver: qué stub la recibió, sobre qué view (si
// es una view con id de recurso) y con qué texto.
type capturaTexto struct {
	Origen string `json:"origen"`
	ID     string `json:"id,omitempty"`
	IDRef  string `json:"idRef,omitempty"`
	Texto  string `json:"texto"`
}

// resultadoEjecucion viaja dentro del modelo de la vista. `error` no tumba el comando: un layout sin
// intérprete sigue siendo útil, y el consumidor tiene que poder distinguir «no hay clases.dex» de
// «el onCreate falló».
type resultadoEjecucion struct {
	Actividad     string         `json:"actividad,omitempty"`
	Metodo        string         `json:"metodo,omitempty"`
	Pasos         int            `json:"pasos"`
	Interacciones int            `json:"interacciones,omitempty"`
	Capturas      []capturaTexto `json:"capturas,omitempty"`
	Avisos        []string       `json:"avisos,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// marcaAncha ocupa el segundo registro de un valor de 64 bits, para que un `move-wide` copie los dos
// y una lectura del registro alto no devuelva un valor que parece un entero normal.
type marcaAncha struct{}

// objetoDalvik es una instancia: su clase, sus campos (con la clave dueño.nombre, para que un campo
// heredado no se confunda con otro), el id de recurso si vino de `findViewById`, el texto que ya se
// le puso y el listener que se registró sobre una view.
type objetoDalvik struct {
	clase  string
	campos map[string]any
	id     uint32
	texto  string
	cadena *strings.Builder
	oyente any
}

// arregloDalvik es un arreglo: el descriptor de su tipo y sus huecos. Los de tipos primitivos
// guardan el valor ya extendido (un `byte` como int32 con signo).
type arregloDalvik struct {
	tipo  string
	datos []any
}

// metodoDalvik es un método del dex listo para ejecutarse. `item` es nil cuando el método no tiene
// cuerpo (abstracto o nativo), y entonces solo puede atenderse como stub.
type metodoDalvik struct {
	nombre  string
	proto   string
	retorno string
	acceso  uint32
	item    *codeItem
}

type claseDalvik struct {
	nombre    string
	super     string
	metodos   map[string]*metodoDalvik
	estaticos map[string]any
}

// programaDalvik es el dex cargado como programa: clases con sus métodos y sus estáticos.
type programaDalvik struct {
	dex    *dexFile
	clases map[string]*claseDalvik
}

// cargaPrograma convierte el dex leído por dexreader.go en un programa ejecutable. Los estáticos
// toman su constante del encoded_array_item que ya lee dexcode.go: las de R son lo que hace que
// `setContentView(R.layout.main)` sea un `const` con el id dentro.
func cargaPrograma(d *dexFile) (*programaDalvik, error) {
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		return nil, err
	}
	p := &programaDalvik{dex: d, clases: make(map[string]*claseDalvik, len(d.classes))}
	for _, c := range d.classes {
		cl := &claseDalvik{
			nombre:    c.name,
			super:     c.super,
			metodos:   map[string]*metodoDalvik{},
			estaticos: map[string]any{},
		}
		cd := datos[c.name]
		metodos := append(append([]encodedMethod{}, cd.directs...), cd.virtuals...)
		for _, m := range metodos {
			if int(m.methodIdx) >= len(d.methods) {
				return nil, fmt.Errorf("%s: el método %d no está en method_ids", c.name, m.methodIdx)
			}
			ref := d.methods[m.methodIdx]
			met := &metodoDalvik{
				nombre:  ref.name,
				proto:   descDeProto(ref.proto),
				retorno: ref.proto.returnType,
				acceso:  m.accessFlags,
			}
			if m.codeOff != 0 {
				item, err := d.readCodeItem(m.codeOff)
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %w", c.name, ref.name, err)
				}
				met.item = &item
			}
			cl.metodos[ref.name+met.proto] = met
		}
		for _, f := range cd.staticFields {
			if int(f.fieldIdx) >= len(d.fields) {
				continue
			}
			cl.estaticos[claveCampo(d.fields[f.fieldIdx])] = valorCodificado(f.valueType, f.value)
		}
		p.clases[c.name] = cl
	}
	return p, nil
}

// descDeProto arma el descriptor de un proto: es la firma con la que se busca un método.
func descDeProto(p dexProto) string {
	return "(" + strings.Join(p.params, "") + ")" + p.returnType
}

// claveCampo nombra un campo por dueño y nombre. El tipo no entra: un mismo nombre en dos clases de
// la misma jerarquía sí tiene que distinguirse, y eso lo da el dueño.
func claveCampo(f dexField) string { return f.owner + "." + f.name }

// valorCodificado traduce el encoded_value de un estático a un valor de la máquina.
func valorCodificado(vf byte, v uint64) any {
	switch vf & 0x1f {
	case valueInt:
		return int32(v)
	case valueLong:
		return int64(v)
	case valueFloat:
		return math.Float32frombits(uint32(v))
	case valueDouble:
		return math.Float64frombits(v)
	case valueShort:
		return int32(int16(v))
	case valueChar:
		return int32(uint16(v))
	case valueByte:
		return int32(int8(v))
	case valueBool:
		return v != 0
	}
	return nil
}

// valorCero es el cero del tipo de un campo o de un retorno: así una lectura de algo que nadie
// escribió no revienta y tampoco devuelve un valor que parezca medido.
func valorCero(tipo string) any {
	if tipo == "" {
		return nil
	}
	switch tipo[0] {
	case 'Z', 'B', 'S', 'C', 'I':
		return int32(0)
	case 'J':
		return int64(0)
	case 'F':
		return float32(0)
	case 'D':
		return float64(0)
	}
	return nil
}

// anchoDe son las palabras que ocupa un valor de ese tipo: dos para `long` y `double`.
func anchoDe(tipo string) int {
	if tipo != "" && (tipo[0] == 'J' || tipo[0] == 'D') {
		return 2
	}
	return 1
}

// maquinaDalvik es el estado de una ejecución.
type maquinaDalvik struct {
	prog        *programaDalvik
	tabla       *tablaRecursos
	pasos       int
	capturas    []capturaTexto
	avisos      []string
	vistos      map[string]bool
	layoutID    uint32
	oyentes     []oyenteRegistrado
	interaccion int
	profundidad int
}

type oyenteRegistrado struct {
	oyente any
	vista  any
}

func nuevaMaquina(p *programaDalvik, tabla *tablaRecursos) *maquinaDalvik {
	return &maquinaDalvik{prog: p, tabla: tabla, vistos: map[string]bool{}}
}

func (m *maquinaDalvik) avisa(formato string, args ...any) {
	texto := fmt.Sprintf(formato, args...)
	if m.vistos[texto] || len(m.avisos) >= avisosMaximos {
		return
	}
	m.vistos[texto] = true
	m.avisos = append(m.avisos, texto)
}

// captura anota una escritura de texto. El id se resuelve contra la tabla para que el modelo diga
// `id/total` y no un número; sin tabla se queda su número, que es lo que hay.
func (m *maquinaDalvik) captura(origen string, obj *objetoDalvik, texto string) {
	c := capturaTexto{Origen: origen, Texto: texto}
	if obj != nil {
		obj.texto = texto
		if obj.id != 0 {
			c.IDRef = fmt.Sprintf("%#08x", obj.id)
			if m.tabla != nil {
				if tipo, clave, ok := m.tabla.nombreDeId(obj.id); ok {
					c.ID = tipo + "/" + clave
				}
			}
		}
	}
	if len(m.capturas) >= capturasMaximas {
		return
	}
	m.capturas = append(m.capturas, c)
}

// textoDe es cómo un stub imprime un valor: lo que haría `String.valueOf`.
func textoDe(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case *objetoDalvik:
		if x.texto != "" {
			return x.texto
		}
		return claseCorta(x.clase)
	case *arregloDalvik:
		return x.tipo
	}
	return fmt.Sprint(v)
}

// claseCorta convierte un descriptor en el nombre con el que se lee un aviso.
func claseCorta(descriptor string) string {
	s := strings.TrimSuffix(strings.TrimPrefix(descriptor, "L"), ";")
	return strings.ReplaceAll(s, "/", ".")
}

// ejecutaOnCreate carga el programa y corre la actividad. Devuelve siempre un resultado; el error de
// la ejecución viaja dentro, no fuera, porque el modelo de la vista se puede construir igual.
func ejecutaOnCreate(d *dexFile, tabla *tablaRecursos, actividad string) (*resultadoEjecucion, uint32) {
	res := &resultadoEjecucion{Metodo: "onCreate"}
	prog, err := cargaPrograma(d)
	if err != nil {
		res.Error = err.Error()
		return res, 0
	}
	m := nuevaMaquina(prog, tabla)
	clase, conMain := eligeClaseLanzadora(prog, actividad)
	if clase == "" {
		res.Error = "el dex no trae ninguna clase con main ni onCreate"
		return res, 0
	}
	res.Actividad = claseCorta(clase)

	cl := prog.clases[clase]
	obj := &objetoDalvik{clase: clase, campos: map[string]any{}}
	if ctor := cl.metodos["<init>()V"]; ctor != nil {
		if _, err := m.ejecutaCuerpo(clase, ctor, []any{obj}, []string{clase}); err != nil {
			res.Error = err.Error()
			res.Pasos = m.pasos
			return res, m.layoutID
		}
	}

	if conMain {
		res.Metodo = "main"
		main := cl.metodos["main([Ljava/lang/String;)V"]
		vacio := &arregloDalvik{tipo: "[Ljava/lang/String;", datos: []any{}}
		if _, err := m.ejecutaCuerpo(clase, main, []any{vacio}, []string{"[Ljava/lang/String;"}); err != nil {
			res.Error = err.Error()
		}
	} else {
		onCreate := cl.metodos["onCreate(Landroid/os/Bundle;)V"]
		if onCreate == nil {
			res.Error = fmt.Sprintf("%s no tiene onCreate(Landroid/os/Bundle;)V", claseCorta(clase))
		} else if _, err := m.ejecutaCuerpo(clase, onCreate, []any{obj, nil},
			[]string{clase, "Landroid/os/Bundle;"}); err != nil {
			res.Error = err.Error()
		}
	}

	// una pulsación por listener: el texto de la plantilla Contador se escribe en el onClick, y sin
	// esta interacción el intérprete no tendría nada que capturar. Es una simulación declarada.
	if res.Error == "" {
		m.pulsaOyentes()
	}
	res.Pasos = m.pasos
	res.Interacciones = m.interaccion
	res.Capturas = m.capturas
	res.Avisos = m.avisos
	return res, m.layoutID
}

// pulsaOyentes invoca `onClick` una vez por listener registrado, con la view sobre la que se
// registró. El tope existe para que un listener que registre otro no encadene sin fin.
func (m *maquinaDalvik) pulsaOyentes() {
	for _, reg := range m.oyentes {
		if m.interaccion >= interaccionesMax {
			m.avisa("se dejaron listeners sin pulsar por el tope de %d", interaccionesMax)
			return
		}
		obj, ok := reg.oyente.(*objetoDalvik)
		if !ok {
			continue
		}
		met := m.buscaMetodo(obj.clase, "onClick", "(Landroid/view/View;)V")
		if met == nil {
			m.avisa("%s se registró como listener y no tiene onClick", claseCorta(obj.clase))
			continue
		}
		m.interaccion++
		if _, err := m.ejecutaCuerpo(obj.clase, met, []any{obj, reg.vista},
			[]string{obj.clase, "Landroid/view/View;"}); err != nil {
			m.avisa("el onClick de %s falló: %v", claseCorta(obj.clase), err)
		}
	}
}

// eligeClaseLanzadora busca a quién ejecutar: primero la actividad del manifiesto si tiene un punto
// de entrada, después cualquier clase sin `$` con `main` y luego con `onCreate`. Ignorar las
// anónimas es lo que evita elegir `MainActivity$1`, que también es una clase con código.
func eligeClaseLanzadora(p *programaDalvik, preferida string) (string, bool) {
	tipoDe := func(nombre string) byte {
		cl, ok := p.clases[nombre]
		if !ok {
			return 0
		}
		if cl.metodos["main([Ljava/lang/String;)V"] != nil {
			return 'm'
		}
		if cl.metodos["onCreate(Landroid/os/Bundle;)V"] != nil {
			return 'o'
		}
		return 0
	}
	if t := tipoDe(preferida); t != 0 {
		return preferida, t == 'm'
	}
	nombres := make([]string, 0, len(p.clases))
	for n := range p.clases {
		if strings.Contains(n, "$") {
			continue
		}
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	for _, n := range nombres {
		if p.clases[n].metodos["main([Ljava/lang/String;)V"] != nil {
			return n, true
		}
	}
	for _, n := range nombres {
		if p.clases[n].metodos["onCreate(Landroid/os/Bundle;)V"] != nil {
			return n, false
		}
	}
	return "", false
}

// buscaMetodo sube por la jerarquía de clases del dex hasta encontrar nombre+firma.
func (m *maquinaDalvik) buscaMetodo(clase, nombre, proto string) *metodoDalvik {
	for i := 0; i < 32; i++ {
		cl, ok := m.prog.clases[clase]
		if !ok {
			return nil
		}
		if met := cl.metodos[nombre+proto]; met != nil {
			return met
		}
		if cl.super == "" || cl.super == clase {
			return nil
		}
		clase = cl.super
	}
	return nil
}

// ejecutaCuerpo interpreta el code_item de un método con los argumentos dados. `tipos` dice el tipo
// de cada argumento, que es lo que decide cuántas palabras ocupa en los registros.
func (m *maquinaDalvik) ejecutaCuerpo(clase string, met *metodoDalvik, args []any, tipos []string) (res any, err error) {
	if met == nil {
		return nil, fmt.Errorf("%s: método sin cuerpo que ejecutar", claseCorta(clase))
	}
	if met.item == nil {
		return m.stubSinCuerpo(clase, met, args)
	}
	it := met.item
	if it.triesSize > 0 {
		return nil, fmt.Errorf("%s.%s lleva tabla de excepciones y el intérprete MVP no la ejecuta",
			claseCorta(clase), met.nombre)
	}
	if m.profundidad >= profundidadMaximaDalvik {
		return nil, fmt.Errorf("llamadas anidadas por encima de %d en %s.%s",
			profundidadMaximaDalvik, claseCorta(clase), met.nombre)
	}
	m.profundidad++
	defer func() { m.profundidad-- }()
	// un dex mal formado puede nombrar un registro fuera del marco; se convierte en error dicho, no
	// en un panic que se lleve por delante el modelo de la vista
	defer func() {
		if r := recover(); r != nil {
			res = nil
			err = fmt.Errorf("%s.%s: instrucción con un registro fuera del marco (%v)",
				claseCorta(clase), met.nombre, r)
		}
	}()

	regs := make([]any, it.registersSize)
	pos := int(it.registersSize) - int(it.insSize)
	for i, a := range args {
		tipo := ""
		if i < len(tipos) {
			tipo = tipos[i]
		}
		regs[pos] = a
		if anchoDe(tipo) == 2 && pos+1 < len(regs) {
			regs[pos+1] = marcaAncha{}
		}
		pos += anchoDe(tipo)
	}

	ins := it.insns
	pc := 0
	var resultado any
	var hayResultado bool
	poner := func(i int, v any) { regs[i] = v }
	ponerAncho := func(i int, v any) {
		regs[i] = v
		if i+1 < len(regs) {
			regs[i+1] = marcaAncha{}
		}
	}

	for {
		if pc < 0 || pc >= len(ins) {
			return nil, fmt.Errorf("%s.%s se salió del código en la unidad %d",
				claseCorta(clase), met.nombre, pc)
		}
		m.pasos++
		if m.pasos > pasosMaximos {
			return nil, fmt.Errorf("el intérprete se detuvo a los %d pasos (posible bucle sin salida)",
				pasosMaximos)
		}
		u := ins[pc]
		op := byte(u)
		aa := int(u >> 8)
		segunda := func() (uint16, error) {
			if pc+1 >= len(ins) {
				return 0, fmt.Errorf("%s.%s: el opcode %#02x pide una palabra y el código termina",
					claseCorta(clase), met.nombre, op)
			}
			return ins[pc+1], nil
		}
		tercera := func() (uint16, error) {
			if pc+2 >= len(ins) {
				return 0, fmt.Errorf("%s.%s: el opcode %#02x pide tres palabras y el código termina",
					claseCorta(clase), met.nombre, op)
			}
			return ins[pc+2], nil
		}

		switch op {
		case 0x00: // nop
			pc++

		// --- movimientos -------------------------------------------------------------------
		case 0x01, 0x04, 0x07: // move / move-wide / move-object (12x)
			dst, src := int(u>>8)&0xf, int(u>>12)
			if op == 0x01 || op == 0x07 {
				poner(dst, regs[src])
			} else {
				ponerAncho(dst, regs[src])
			}
			pc++
		case 0x02, 0x05, 0x08: // move/from16, move-wide/from16, move-object/from16 (22x)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if op == 0x02 || op == 0x08 {
				poner(aa, regs[int(w)])
			} else {
				ponerAncho(aa, regs[int(w)])
			}
			pc += 2
		case 0x03, 0x06, 0x09: // move/16, move-wide/16, move-object/16 (32x)
			if pc+2 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un move/16")
			}
			src := int(ins[pc+2])
			if op == 0x03 || op == 0x09 {
				poner(aa, regs[src])
			} else {
				ponerAncho(aa, regs[src])
			}
			pc += 3
		case 0x0a: // move-result
			if !hayResultado {
				m.avisa("move-result sin llamada previa en %s.%s", claseCorta(clase), met.nombre)
			}
			poner(aa, resultado)
			pc++
		case 0x0b: // move-result-wide
			ponerAncho(aa, resultado)
			pc++
		case 0x0c: // move-result-object
			poner(aa, resultado)
			pc++
		case 0x0d: // move-exception
			return nil, fmt.Errorf("%s.%s usa move-exception y el MVP no ejecuta excepciones",
				claseCorta(clase), met.nombre)

		// --- retornos ----------------------------------------------------------------------
		case 0x0e:
			return nil, nil
		case 0x0f, 0x10, 0x11: // return, return-wide, return-object
			return regs[aa], nil

		// --- constantes --------------------------------------------------------------------
		case 0x12: // const/4: el registro es el nibble bajo de AA y el literal el alto, con signo
			v := aa >> 4
			if v >= 8 {
				v -= 16
			}
			poner(aa&0xf, int32(v))
			pc++
		case 0x13: // const/16
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			poner(aa, int32(int16(w)))
			pc += 2
		case 0x14: // const (31i)
			if pc+2 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un const")
			}
			poner(aa, int32(uint32(ins[pc+1])|uint32(ins[pc+2])<<16))
			pc += 3
		case 0x15: // const/high16
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			poner(aa, int32(uint32(w)<<16))
			pc += 2
		case 0x16: // const-wide/16
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			ponerAncho(aa, int64(int16(w)))
			pc += 2
		case 0x17: // const-wide/32
			if pc+2 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un const-wide/32")
			}
			ponerAncho(aa, int64(int32(uint32(ins[pc+1])|uint32(ins[pc+2])<<16)))
			pc += 3
		case 0x18: // const-wide (51l)
			if pc+4 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un const-wide")
			}
			v := uint64(ins[pc+1]) | uint64(ins[pc+2])<<16 | uint64(ins[pc+3])<<32 | uint64(ins[pc+4])<<48
			ponerAncho(aa, int64(v))
			pc += 5
		case 0x19: // const-wide/high16
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			ponerAncho(aa, int64(int16(w))<<48)
			pc += 2
		case 0x1a: // const-string
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if int(w) >= len(m.prog.dex.strings) {
				return nil, fmt.Errorf("const-string con la cadena %d fuera del pool", w)
			}
			poner(aa, m.prog.dex.strings[w])
			pc += 2
		case 0x1b: // const-string/jumbo
			if pc+2 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un const-string/jumbo")
			}
			idx := uint32(ins[pc+1]) | uint32(ins[pc+2])<<16
			if int(idx) >= len(m.prog.dex.strings) {
				return nil, fmt.Errorf("const-string/jumbo con la cadena %d fuera del pool", idx)
			}
			poner(aa, m.prog.dex.strings[idx])
			pc += 3
		case 0x1c: // const-class
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if int(w) >= len(m.prog.dex.types) {
				return nil, fmt.Errorf("const-class con el tipo %d fuera de type_ids", w)
			}
			poner(aa, &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
				"Ljava/lang/Class;.$tipo": m.prog.dex.types[w],
			}})
			pc += 2

		// --- objetos -----------------------------------------------------------------------
		case 0x1f: // check-cast
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if obj, ok := regs[aa].(*objetoDalvik); ok && int(w) < len(m.prog.dex.types) {
				obj.clase = m.prog.dex.types[w]
			}
			pc += 2
		case 0x20: // instance-of (22c)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			es := false
			if obj, ok := regs[int(u>>12)].(*objetoDalvik); ok && int(w) < len(m.prog.dex.types) {
				es = m.esDe(obj, m.prog.dex.types[w])
			}
			poner(aa&0xf, boolAInt(es))
			pc += 2
		case 0x21: // array-length
			arr, ok := regs[int(u>>12)].(*arregloDalvik)
			if !ok {
				return nil, fmt.Errorf("array-length sobre un registro que no es arreglo")
			}
			poner(int(u>>8)&0xf, int32(len(arr.datos)))
			pc++
		case 0x22: // new-instance
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if int(w) >= len(m.prog.dex.types) {
				return nil, fmt.Errorf("new-instance con el tipo %d fuera de type_ids", w)
			}
			poner(aa, &objetoDalvik{clase: m.prog.dex.types[w], campos: map[string]any{}})
			pc += 2
		case 0x23: // new-array (22c)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			n, err := aInt32(regs[int(u>>12)])
			if err != nil {
				return nil, fmt.Errorf("new-array: %w", err)
			}
			if n < 0 || n > 1<<20 {
				return nil, fmt.Errorf("new-array con longitud %d", n)
			}
			tipo := ""
			if int(w) < len(m.prog.dex.types) {
				tipo = m.prog.dex.types[w]
			}
			poner(int(u>>8)&0xf, &arregloDalvik{tipo: tipo, datos: make([]any, n)})
			pc += 2
		case 0x24: // filled-new-array (35c)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			t, err := tercera()
			if err != nil {
				return nil, err
			}
			tipo := ""
			if int(w) < len(m.prog.dex.types) {
				tipo = m.prog.dex.types[w]
			}
			regsArg := registros35c(u, t)
			datos := make([]any, 0, len(regsArg))
			for _, r := range regsArg {
				datos = append(datos, regs[r])
			}
			resultado = &arregloDalvik{tipo: tipo, datos: datos}
			hayResultado = true
			pc += 3
		case 0x25: // filled-new-array/range (3rc)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			t, err := tercera()
			if err != nil {
				return nil, err
			}
			tipo := ""
			if int(w) < len(m.prog.dex.types) {
				tipo = m.prog.dex.types[w]
			}
			primero, cuenta := int(t), int(u>>8)
			datos := make([]any, 0, cuenta)
			for i := 0; i < cuenta; i++ {
				datos = append(datos, regs[primero+i])
			}
			resultado = &arregloDalvik{tipo: tipo, datos: datos}
			hayResultado = true
			pc += 3
		case 0x26: // fill-array-data
			return nil, fmt.Errorf("%s.%s usa fill-array-data y el intérprete MVP no lo ejecuta",
				claseCorta(clase), met.nombre)
		case 0x27: // throw
			return nil, fmt.Errorf("%s.%s lanza una excepción y el MVP no las ejecuta",
				claseCorta(clase), met.nombre)

		// --- saltos ------------------------------------------------------------------------
		case 0x28: // goto (10t)
			pc += int(int8(aa))
		case 0x29: // goto/16 (20t)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			pc += int(int16(w))
		case 0x2a: // goto/32 (30t)
			if pc+2 >= len(ins) {
				return nil, fmt.Errorf("código cortado en un goto/32")
			}
			pc += int(int32(uint32(ins[pc+1]) | uint32(ins[pc+2])<<16))
		case 0x2b, 0x2c: // packed-switch / sparse-switch
			salto, err := m.saltoDeSwitch(ins, pc, op == 0x2b, regs[aa])
			if err != nil {
				return nil, err
			}
			pc += salto
		case 0x2d, 0x2e, 0x2f, 0x30, 0x31: // cmp*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			a, b := regs[int(w)&0xff], regs[int(w)>>8]
			cmp, err := m.compara(op, a, b)
			if err != nil {
				return nil, err
			}
			poner(aa, int32(cmp))
			pc += 2
		case 0x32, 0x33, 0x34, 0x35, 0x36, 0x37: // if-* (22t)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			cumple, err := m.comparaIf(op, regs[int(u>>8)&0xf], regs[int(u>>12)])
			if err != nil {
				return nil, err
			}
			if cumple {
				pc += int(int16(w))
			} else {
				pc += 2
			}
		case 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d: // if-*z (21t)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			cumple, err := m.comparaIf(op-6, regs[aa], int32(0))
			if err != nil {
				return nil, err
			}
			if cumple {
				pc += int(int16(w))
			} else {
				pc += 2
			}

		// --- arreglos ----------------------------------------------------------------------
		case 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a: // aget*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			arr, ok := regs[int(w)&0xff].(*arregloDalvik)
			if !ok {
				return nil, fmt.Errorf("aget sobre un registro que no es arreglo")
			}
			idx, err := aInt32(regs[int(w)>>8])
			if err != nil {
				return nil, err
			}
			if int(idx) < 0 || int(idx) >= len(arr.datos) {
				return nil, fmt.Errorf("aget con el índice %d fuera del arreglo de %d", idx, len(arr.datos))
			}
			v := arr.datos[idx]
			if op == 0x45 {
				ponerAncho(aa, v)
			} else {
				poner(aa, v)
			}
			pc += 2
		case 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51: // aput*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			arr, ok := regs[int(w)&0xff].(*arregloDalvik)
			if !ok {
				return nil, fmt.Errorf("aput sobre un registro que no es arreglo")
			}
			idx, err := aInt32(regs[int(w)>>8])
			if err != nil {
				return nil, err
			}
			if int(idx) < 0 || int(idx) >= len(arr.datos) {
				return nil, fmt.Errorf("aput con el índice %d fuera del arreglo de %d", idx, len(arr.datos))
			}
			arr.datos[idx] = regs[aa]
			pc += 2

		// --- campos ------------------------------------------------------------------------
		case 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58: // iget*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			v, err := m.leeCampo(uint32(w), regs[int(u>>12)], false)
			if err != nil {
				return nil, err
			}
			if op == 0x53 {
				ponerAncho(int(u>>8)&0xf, v)
			} else {
				poner(int(u>>8)&0xf, v)
			}
			pc += 2
		case 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f: // iput*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if err := m.escribeCampo(uint32(w), regs[int(u>>12)], regs[int(u>>8)&0xf], false); err != nil {
				return nil, err
			}
			pc += 2
		case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66: // sget*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			v, err := m.leeCampo(uint32(w), nil, true)
			if err != nil {
				return nil, err
			}
			if op == 0x61 {
				ponerAncho(aa, v)
			} else {
				poner(aa, v)
			}
			pc += 2
		case 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d: // sput*
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			if err := m.escribeCampo(uint32(w), nil, regs[aa], true); err != nil {
				return nil, err
			}
			pc += 2

		// --- llamadas ----------------------------------------------------------------------
		case 0x6e, 0x6f, 0x70, 0x71, 0x72: // invoke-* (35c)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			t, err := tercera()
			if err != nil {
				return nil, err
			}
			v, err := m.invoca(op, uint32(w), registros35c(u, t), regs, clase)
			if err != nil {
				return nil, err
			}
			resultado, hayResultado = v, true
			pc += 3
		case 0x74, 0x75, 0x76, 0x77, 0x78: // invoke-*/range (3rc)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			t, err := tercera()
			if err != nil {
				return nil, err
			}
			primero, cuenta := int(t), int(u>>8)
			regsArg := make([]int, cuenta)
			for i := 0; i < cuenta; i++ {
				regsArg[i] = primero + i
			}
			v, err := m.invoca(op-6, uint32(w), regsArg, regs, clase)
			if err != nil {
				return nil, err
			}
			resultado, hayResultado = v, true
			pc += 3

		// --- unarios y conversiones --------------------------------------------------------
		case 0x7b, 0x7c, 0x7d, 0x7e, 0x7f, 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86,
			0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f:
			dst, src := int(u>>8)&0xf, int(u>>12)
			v, ancho, err := unaria(op, regs[src])
			if err != nil {
				return nil, err
			}
			if ancho == 2 {
				ponerAncho(dst, v)
			} else {
				poner(dst, v)
			}
			pc++

		// --- binarias ----------------------------------------------------------------------
		case 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a,
			0x9b, 0x9c, 0x9d, 0x9e, 0x9f, 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5,
			0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf:
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			v, ancho, err := binaria(op, regs[int(w)&0xff], regs[int(w)>>8])
			if err != nil {
				return nil, err
			}
			if ancho == 2 {
				ponerAncho(aa, v)
			} else {
				poner(aa, v)
			}
			pc += 2
		case 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba,
			0xbb, 0xbc, 0xbd, 0xbe, 0xbf, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5,
			0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf: // */2addr (12x)
			dst, src := int(u>>8)&0xf, int(u>>12)
			v, ancho, err := binaria(op-0x20, regs[dst], regs[src])
			if err != nil {
				return nil, err
			}
			if ancho == 2 {
				ponerAncho(dst, v)
			} else {
				poner(dst, v)
			}
			pc++
		case 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7: // lit16 (22s)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			lit := int32(int16(w))
			v, err := enteraLit(op-0xd0, regs[aa], lit)
			if err != nil {
				return nil, err
			}
			poner(aa, v)
			pc += 2
		case 0xd8, 0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0, 0xe1, 0xe2: // lit8 (22b)
			w, err := segunda()
			if err != nil {
				return nil, err
			}
			lit := int32(int8(w >> 8))
			v, err := enteraLit(op-0xd8, regs[int(w)&0xf], lit)
			if err != nil {
				return nil, err
			}
			poner(aa, v)
			pc += 2
		default:
			return nil, fmt.Errorf("%s.%s usa el opcode %#02x, que el intérprete MVP no decodifica",
				claseCorta(clase), met.nombre, op)
		}
	}
}

// registros35c reconstruye la lista de registros del formato 35c: el contador en la mitad alta de la
// primera palabra, los cuatro primeros repartidos en nibbles de la tercera y el quinto en la
// segunda. Nuestro emisor usa 3rc para más de cinco palabras, así que aquí nunca hay más de cinco.
func registros35c(u, tercera uint16) []int {
	n := int(u>>12) & 0xf
	g := int(u>>8) & 0xf
	out := make([]int, 0, 5)
	for k := 0; k < n && k < 4; k++ {
		out = append(out, int(tercera>>(4*k))&0xf)
	}
	if n == 5 {
		out = append(out, g)
	}
	return out
}

// saltoDeSwitch lee el payload de un switch y devuelve el desplazamiento que toca desde la
// instrucción del switch, o 3 si el valor no cae en ningún brazo (el tamaño de la propia
// instrucción: la ejecución sigue detrás).
func (m *maquinaDalvik) saltoDeSwitch(ins []uint16, pc int, compacto bool, valor any) (int, error) {
	if pc+1 >= len(ins) {
		return 0, fmt.Errorf("código cortado en un switch")
	}
	base := pc + int(int16(ins[pc+1]))
	if base < 0 || base+1 >= len(ins) {
		return 0, fmt.Errorf("el payload del switch cae fuera del código")
	}
	ident := ins[base]
	if compacto && ident != 0x0100 {
		return 0, fmt.Errorf("un packed-switch apunta a un payload %#04x", ident)
	}
	if !compacto && ident != 0x0200 {
		return 0, fmt.Errorf("un sparse-switch apunta a un payload %#04x", ident)
	}
	clave, err := aInt32(valor)
	if err != nil {
		return 0, fmt.Errorf("switch: %w", err)
	}
	n := int(ins[base+1])
	if base+2+n > len(ins) {
		return 0, fmt.Errorf("el payload del switch declara %d brazos y no caben", n)
	}
	if compacto {
		primera := int32(uint32(ins[base+2]) | uint32(ins[base+3])<<16)
		indice := int(clave - primera)
		if indice >= 0 && indice < n {
			dst := base + 4 + 2*indice
			if dst < len(ins) {
				// los destinos del payload son relativos a la INSTRUCCIÓN del switch, no al payload
				return int(int32(uint32(ins[dst]) | uint32(ins[dst+1])<<16)), nil
			}
		}
		return 3, nil
	}
	for k := 0; k < n; k++ {
		kpos := base + 2 + 2*k
		if kpos+1 >= len(ins) {
			break
		}
		claveK := int32(uint32(ins[kpos]) | uint32(ins[kpos+1])<<16)
		if claveK == clave {
			dpos := base + 2 + 2*n + 2*k
			if dpos+1 < len(ins) {
				return int(int32(uint32(ins[dpos]) | uint32(ins[dpos+1])<<16)), nil
			}
		}
	}
	return 3, nil
}

// esDe responde a `instance-of` subiendo por la jerarquía del dex.
func (m *maquinaDalvik) esDe(obj *objetoDalvik, tipo string) bool {
	clase := obj.clase
	for i := 0; i < 32; i++ {
		if clase == tipo {
			return true
		}
		cl, ok := m.prog.clases[clase]
		if !ok {
			break
		}
		if cl.super == "" || cl.super == clase {
			break
		}
		clase = cl.super
	}
	return false
}

// --- campos ---------------------------------------------------------------------------------

func (m *maquinaDalvik) leeCampo(idx uint32, receptor any, estatico bool) (any, error) {
	d := m.prog.dex
	if int(idx) >= len(d.fields) {
		return nil, fmt.Errorf("campo %d fuera de field_ids", idx)
	}
	f := d.fields[idx]
	if estatico {
		return m.leeEstatico(f)
	}
	if receptor == nil {
		return nil, fmt.Errorf("se leyó el campo %s.%s de un objeto nulo", claseCorta(f.owner), f.name)
	}
	obj, ok := receptor.(*objetoDalvik)
	if !ok {
		return valorCero(f.typeOf), nil
	}
	if v, ok := obj.campos[claveCampo(f)]; ok {
		return v, nil
	}
	return valorCero(f.typeOf), nil
}

func (m *maquinaDalvik) leeEstatico(f dexField) (any, error) {
	// `System.out` y `System.err` son el destino de println; se representan como el flujo que son
	if f.owner == "Ljava/lang/System;" && (f.name == "out" || f.name == "err") {
		return &objetoDalvik{clase: "Ljava/io/PrintStream;", campos: map[string]any{}}, nil
	}
	if cl, ok := m.prog.clases[f.owner]; ok {
		if v, ok := cl.estaticos[claveCampo(f)]; ok {
			return v, nil
		}
	}
	return valorCero(f.typeOf), nil
}

func (m *maquinaDalvik) escribeCampo(idx uint32, receptor any, v any, estatico bool) error {
	d := m.prog.dex
	if int(idx) >= len(d.fields) {
		return fmt.Errorf("campo %d fuera de field_ids", idx)
	}
	f := d.fields[idx]
	if estatico {
		cl, ok := m.prog.clases[f.owner]
		if !ok {
			m.avisa("se escribió el estático %s.%s sin cargar su clase", claseCorta(f.owner), f.name)
			return nil
		}
		cl.estaticos[claveCampo(f)] = v
		return nil
	}
	if receptor == nil {
		return fmt.Errorf("se escribió el campo %s.%s en un objeto nulo", claseCorta(f.owner), f.name)
	}
	obj, ok := receptor.(*objetoDalvik)
	if !ok {
		return fmt.Errorf("se escribió el campo %s sobre un registro que no es objeto", f.name)
	}
	obj.campos[claveCampo(f)] = v
	return nil
}

// --- llamadas -------------------------------------------------------------------------------

// invoca resuelve una instrucción invoke: primero las clases del propio dex, y si el método no está
// ahí, los stubs de framework. `op` es el opcode del invoke (0x6e..0x72) y `palabras` los registros
// que llevan los argumentos, contados en palabras.
func (m *maquinaDalvik) invoca(op byte, idx uint32, palabras []int, regs []any, claseActual string) (any, error) {
	d := m.prog.dex
	if int(idx) >= len(d.methods) {
		return nil, fmt.Errorf("invoke con el método %d fuera de method_ids", idx)
	}
	ref := d.methods[idx]
	proto := descDeProto(ref.proto)

	esEstatico := op == 0x71
	var receptor any
	base := 0
	if !esEstatico {
		if len(palabras) == 0 {
			return nil, fmt.Errorf("invoke-virtual de %s.%s sin registros", claseCorta(ref.owner), ref.name)
		}
		receptor = regs[palabras[0]]
		if receptor == nil {
			return nil, fmt.Errorf("se llamó a %s.%s sobre un objeto nulo", claseCorta(ref.owner), ref.name)
		}
		base = 1
	}
	vals := make([]any, 0, len(ref.proto.params))
	tipos := make([]string, 0, len(ref.proto.params))
	pos := base
	for _, p := range ref.proto.params {
		if pos >= len(palabras) {
			return nil, fmt.Errorf("invoke de %s.%s: faltan registros para sus argumentos",
				claseCorta(ref.owner), ref.name)
		}
		vals = append(vals, regs[palabras[pos]])
		tipos = append(tipos, p)
		pos += anchoDe(p)
	}

	// 1) las clases del propio dex. La resolución no es la misma para las cinco formas: un
	// `invoke-direct` y un `invoke-super` van a la clase que DECLARA el método (el constructor de
	// Activity, no el de MainActivity, aunque el receptor sea la actividad); solo virtual e
	// interface despachan por la clase del receptor.
	en := func(clase string, met *metodoDalvik, args []any, argTipos []string) (any, error) {
		v, err := m.ejecutaCuerpo(clase, met, args, argTipos)
		if err != nil {
			return nil, fmt.Errorf("en %s.%s: %w", claseCorta(clase), met.nombre, err)
		}
		return v, nil
	}
	if esEstatico {
		if met := m.buscaMetodo(ref.owner, ref.name, proto); met != nil {
			return en(ref.owner, met, vals, tipos)
		}
	} else if op == 0x6f || op == 0x70 {
		if met := m.buscaMetodo(ref.owner, ref.name, proto); met != nil {
			args := append([]any{receptor}, vals...)
			argTipos := append([]string{ref.owner}, tipos...)
			return en(ref.owner, met, args, argTipos)
		}
	} else if obj, ok := receptor.(*objetoDalvik); ok {
		if met := m.buscaMetodo(obj.clase, ref.name, proto); met != nil {
			args := append([]any{obj}, vals...)
			argTipos := append([]string{obj.clase}, tipos...)
			return en(obj.clase, met, args, argTipos)
		}
	}

	// 2) los stubs de framework. La clase que declara el método es la del ref, pero un
	// `findViewById` invocado sobre MainActivity se declara en Activity: se prueban las dos.
	candidatos := []string{ref.owner}
	if obj, ok := receptor.(*objetoDalvik); ok {
		candidatos = append([]string{obj.clase}, candidatos...)
	}
	if v, ok, err := m.intentaStub(candidatos, ref.name, proto, vals, receptor); ok || err != nil {
		return v, err
	}

	// 3) sin resolver: cero del tipo de retorno y aviso, que es lo que evita confundir una
	// aproximación con una ejecución
	m.avisa("%s.%s%s no está en el dex ni en los stubs: se devuelve el cero de %s",
		claseCorta(ref.owner), ref.name, proto, ref.proto.returnType)
	return valorCero(ref.proto.returnType), nil
}

// stubSinCuerpo atiende los métodos que el dex declara sin código: los `printInt`, `print` y
// `println` que una app usa como salida capturan su argumento; el resto se avisa.
func (m *maquinaDalvik) stubSinCuerpo(clase string, met *metodoDalvik, args []any) (any, error) {
	switch strings.ToLower(met.nombre) {
	case "printint", "print", "println", "log":
		if len(args) > 0 {
			m.captura(met.nombre, nil, textoDe(args[len(args)-1]))
		}
		return valorCero(met.retorno), nil
	}
	m.avisa("%s.%s no tiene cuerpo y no es un stub conocido", claseCorta(clase), met.nombre)
	return valorCero(met.retorno), nil
}

// intentaStub resuelve las llamadas de framework que la app toca. `candidatos` son las clases por
// las que se busca (la del objeto, la que declara el método); los sufijos bastan porque lo que
// distingue un `setText` es el nombre y su firma, no la jerarquía entera del framework.
func (m *maquinaDalvik) intentaStub(candidatos []string, nombre, proto string, vals []any, receptor any) (any, bool, error) {
	es := func(sufijos ...string) bool {
		for _, c := range candidatos {
			for _, s := range sufijos {
				if strings.HasSuffix(c, s) {
					return true
				}
			}
		}
		return false
	}
	obj, _ := receptor.(*objetoDalvik)

	switch nombre {
	case "<init>":
		// el constructor de una clase del framework (Activity, Object, la vista que sea): no hay
		// cuerpo que ejecutar y no hay nada que capturar
		return nil, true, nil
	case "onCreate", "onStart", "onResume", "onPause", "onStop", "onDestroy", "onRestart",
		"onSaveInstanceState", "onRestoreInstanceState", "onPostCreate", "onPostResume":
		if es("Landroid/app/Activity;", "Landroid/app/ListActivity;", "Landroid/app/ActivityGroup;") {
			return nil, true, nil
		}
	case "setContentView":
		if len(vals) == 1 {
			if id, err := aInt32(vals[0]); err == nil {
				m.layoutID = uint32(id)
				return nil, true, nil
			}
			if o, ok := vals[0].(*objetoDalvik); ok && o.id != 0 {
				m.layoutID = o.id
				return nil, true, nil
			}
		}
	case "findViewById":
		if len(vals) == 1 {
			id, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			return &objetoDalvik{clase: "Landroid/view/View;", campos: map[string]any{},
				id: uint32(id)}, true, nil
		}
	case "setOnClickListener":
		if len(vals) == 1 {
			if o := obj; o != nil {
				o.oyente = vals[0]
				m.oyentes = append(m.oyentes, oyenteRegistrado{oyente: vals[0], vista: o})
			}
			return nil, true, nil
		}
	case "setText":
		if len(vals) == 1 && es("Landroid/widget/TextView;", "Landroid/widget/Button;",
			"Landroid/widget/EditText;", "Landroid/widget/CheckBox;", "Landroid/widget/Switch;") {
			texto := textoDe(vals[0])
			// `setText(R.string.x)` llega como su número: se resuelve contra la tabla, que es lo
			// que el layout hace con `@string/x`. Un literal cualquiera no cae aquí porque la
			// tabla solo resuelve ids de SU paquete.
			if id, ok := vals[0].(int32); ok && m.tabla != nil {
				if v, ok := m.tabla.valorDeId(uint32(id)); ok && v.tipo == tipoCadena &&
					int(v.datos) < len(m.tabla.global.cadenas) {
					texto = m.tabla.global.cadenas[v.datos]
				}
			}
			m.captura("TextView.setText", obj, texto)
			return nil, true, nil
		}
	case "setTitle":
		if len(vals) == 1 && es("Landroid/app/Activity;") {
			m.captura("Activity.setTitle", obj, textoDe(vals[0]))
			return nil, true, nil
		}
	case "toString":
		if len(vals) == 0 && es("Ljava/lang/StringBuilder;") && obj != nil && obj.cadena != nil {
			return obj.cadena.String(), true, nil
		}
		if len(vals) == 0 && obj != nil {
			return textoDe(obj), true, nil
		}
	case "append":
		if len(vals) == 1 && obj != nil {
			if obj.cadena == nil {
				obj.cadena = &strings.Builder{}
			}
			obj.cadena.WriteString(textoDe(vals[0]))
			return obj, true, nil
		}
	case "length":
		if len(vals) == 0 && es("Ljava/lang/StringBuilder;") && obj != nil && obj.cadena != nil {
			return int32(obj.cadena.Len()), true, nil
		}
	case "d", "i", "w", "e", "v", "wtf":
		if es("Landroid/util/Log;") {
			texto := ""
			if len(vals) >= 2 {
				texto = textoDe(vals[1])
			}
			m.captura("Log."+nombre, nil, texto)
			return int32(0), true, nil
		}
	case "println", "print":
		if es("Ljava/io/PrintStream;", "Ljava/io/OutputStream;") {
			if len(vals) >= 1 {
				m.captura("println", nil, textoDe(vals[0]))
			}
			return nil, true, nil
		}
	case "valueOf":
		if len(vals) == 1 && es("Ljava/lang/String;") {
			return textoDe(vals[0]), true, nil
		}
	case "parseInt":
		if len(vals) >= 1 && es("Ljava/lang/Integer;", "Ljava/lang/Long;") {
			n, err := strconv.ParseInt(strings.TrimSpace(textoDe(vals[0])), 10, 32)
			if err != nil {
				return nil, true, fmt.Errorf("parseInt de %q: %w", textoDe(vals[0]), err)
			}
			if proto == "(Ljava/lang/String;)J" {
				return n, true, nil
			}
			return int32(n), true, nil
		}
	case "charAt":
		if len(vals) == 1 && es("Ljava/lang/String;") {
			i, err := aInt32(vals[0])
			if err != nil {
				return nil, true, err
			}
			s, ok := receptor.(string)
			if !ok || int(i) < 0 || int(i) >= len(s) {
				return nil, true, fmt.Errorf("charAt con el índice %d fuera de la cadena", i)
			}
			return int32(s[i]), true, nil
		}
	}
	return nil, false, nil
}

// --- operaciones ----------------------------------------------------------------------------

func aInt32(v any) (int32, error) {
	switch x := v.(type) {
	case int32:
		return x, nil
	case int64:
		return int32(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un entero")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un entero", v)
}

func aInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int32:
		return int64(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un long")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un long", v)
}

func aFloat32(v any) (float32, error) {
	switch x := v.(type) {
	case float32:
		return x, nil
	case int32:
		// un registro no está tipado: el patrón de bits de un `const` puede ser un float
		return math.Float32frombits(uint32(x)), nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un float")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un float", v)
}

func aFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case int64:
		return math.Float64frombits(uint64(x)), nil
	case nil:
		return 0, fmt.Errorf("el registro está vacío y se esperaba un double")
	}
	return 0, fmt.Errorf("el registro guarda %T y se esperaba un double", v)
}

func boolAInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// esReferencia: lo que se compara por identidad en Dalvik.
func esReferencia(v any) bool {
	switch v.(type) {
	case nil, *objetoDalvik, *arregloDalvik, string:
		return true
	}
	return false
}

// igualesDalvik es el `==` de Dalvik: identidad para objetos y arreglos, contenido para cadenas.
func igualesDalvik(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch x := a.(type) {
	case *objetoDalvik:
		y, ok := b.(*objetoDalvik)
		return ok && x == y
	case *arregloDalvik:
		y, ok := b.(*arregloDalvik)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	}
	return false
}

func (m *maquinaDalvik) comparaIf(op byte, a, b any) (bool, error) {
	if op == 0x32 || op == 0x33 { // if-eq / if-ne
		eq := false
		if esReferencia(a) || esReferencia(b) {
			eq = igualesDalvik(a, b)
		} else {
			av, err := aInt64(a)
			if err != nil {
				return false, err
			}
			bv, err := aInt64(b)
			if err != nil {
				return false, err
			}
			eq = av == bv
		}
		if op == 0x32 {
			return eq, nil
		}
		return !eq, nil
	}
	av, err := aInt64(a)
	if err != nil {
		return false, err
	}
	bv, err := aInt64(b)
	if err != nil {
		return false, err
	}
	switch op {
	case 0x34: // if-lt
		return av < bv, nil
	case 0x35: // if-ge
		return av >= bv, nil
	case 0x36: // if-gt
		return av > bv, nil
	case 0x37: // if-le
		return av <= bv, nil
	}
	return false, fmt.Errorf("comparación %#02x desconocida", op)
}

func (m *maquinaDalvik) compara(op byte, a, b any) (int, error) {
	switch op {
	case 0x2d, 0x2e: // cmpl-float / cmpg-float
		av, err := aFloat32(a)
		if err != nil {
			return 0, err
		}
		bv, err := aFloat32(b)
		if err != nil {
			return 0, err
		}
		if math.IsNaN(float64(av)) || math.IsNaN(float64(bv)) {
			if op == 0x2d {
				return -1, nil
			}
			return 1, nil
		}
		return cmpFloat(float64(av), float64(bv)), nil
	case 0x2f, 0x30: // cmpl-double / cmpg-double
		av, err := aFloat64(a)
		if err != nil {
			return 0, err
		}
		bv, err := aFloat64(b)
		if err != nil {
			return 0, err
		}
		if math.IsNaN(av) || math.IsNaN(bv) {
			if op == 0x2f {
				return -1, nil
			}
			return 1, nil
		}
		return cmpFloat(av, bv), nil
	case 0x31: // cmp-long
		av, err := aInt64(a)
		if err != nil {
			return 0, err
		}
		bv, err := aInt64(b)
		if err != nil {
			return 0, err
		}
		switch {
		case av < bv:
			return -1, nil
		case av > bv:
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("comparación %#02x desconocida", op)
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// unaria ejecuta las conversiones y los negados/notados de 12x.
func unaria(op byte, v any) (any, int, error) {
	switch op {
	case 0x7b: // neg-int
		x, err := aInt32(v)
		return -x, 1, err
	case 0x7c: // not-int
		x, err := aInt32(v)
		return ^x, 1, err
	case 0x7d: // neg-long
		x, err := aInt64(v)
		return -x, 2, err
	case 0x7e: // not-long
		x, err := aInt64(v)
		return ^x, 2, err
	case 0x7f: // neg-float
		x, err := aFloat32(v)
		return -x, 1, err
	case 0x80: // neg-double
		x, err := aFloat64(v)
		return -x, 2, err
	case 0x81: // int-to-long
		x, err := aInt32(v)
		return int64(x), 2, err
	case 0x82: // int-to-float
		x, err := aInt32(v)
		return float32(x), 1, err
	case 0x83: // int-to-double
		x, err := aInt32(v)
		return float64(x), 2, err
	case 0x84: // long-to-int
		x, err := aInt64(v)
		return int32(x), 1, err
	case 0x85: // long-to-float
		x, err := aInt64(v)
		return float32(x), 1, err
	case 0x86: // long-to-double
		x, err := aInt64(v)
		return float64(x), 2, err
	case 0x87: // float-to-int
		x, err := aFloat32(v)
		return int32(x), 1, err
	case 0x88: // float-to-long
		x, err := aFloat32(v)
		return int64(x), 2, err
	case 0x89: // float-to-double
		x, err := aFloat32(v)
		return float64(x), 2, err
	case 0x8a: // double-to-int
		x, err := aFloat64(v)
		return int32(x), 1, err
	case 0x8b: // double-to-long
		x, err := aFloat64(v)
		return int64(x), 2, err
	case 0x8c: // double-to-float
		x, err := aFloat64(v)
		return float32(x), 1, err
	case 0x8d: // int-to-byte
		x, err := aInt32(v)
		return int32(int8(x)), 1, err
	case 0x8e: // int-to-char
		x, err := aInt32(v)
		return int32(uint16(x)), 1, err
	case 0x8f: // int-to-short
		x, err := aInt32(v)
		return int32(int16(x)), 1, err
	}
	return nil, 1, fmt.Errorf("unario %#02x desconocido", op)
}

// binaria ejecuta las operaciones de 23x. El desplazamiento del int va enmascarado a cinco bits y el
// del long a seis, como manda Dalvik, no como deja el hardware.
func binaria(op byte, a, b any) (any, int, error) {
	switch {
	case op >= 0x90 && op <= 0x9a: // int
		x, err := aInt32(a)
		if err != nil {
			return nil, 1, err
		}
		y, err := aInt32(b)
		if err != nil {
			return nil, 1, err
		}
		v, err := entera(op-0x90, x, y)
		return v, 1, err
	case op >= 0x9b && op <= 0xa5: // long
		x, err := aInt64(a)
		if err != nil {
			return nil, 2, err
		}
		y, err := aInt64(b)
		if err != nil {
			return nil, 2, err
		}
		v, err := larga(op-0x9b, x, y)
		return v, 2, err
	case op >= 0xa6 && op <= 0xaa: // float
		x, err := aFloat32(a)
		if err != nil {
			return nil, 1, err
		}
		y, err := aFloat32(b)
		if err != nil {
			return nil, 1, err
		}
		v, err := flotante(op-0xa6, x, y)
		return v, 1, err
	case op >= 0xab && op <= 0xaf: // double
		x, err := aFloat64(a)
		if err != nil {
			return nil, 2, err
		}
		y, err := aFloat64(b)
		if err != nil {
			return nil, 2, err
		}
		v, err := doble(op-0xab, x, y)
		return v, 2, err
	}
	return nil, 1, fmt.Errorf("binaria %#02x desconocida", op)
}

// Las operaciones se nombran por su índice dentro de la familia: 0 suma, 1 resta, 2 producto, 3
// división, 4 resto, 5 and, 6 or, 7 xor, 8 desplazamiento a la izquierda, 9 a la derecha con signo
// y 10 a la derecha sin signo.
func entera(k byte, a, b int32) (int32, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		if b == 0 {
			return 0, fmt.Errorf("división entera por cero")
		}
		return a / b, nil
	case 4:
		if b == 0 {
			return 0, fmt.Errorf("resto entero por cero")
		}
		return a % b, nil
	case 5:
		return a & b, nil
	case 6:
		return a | b, nil
	case 7:
		return a ^ b, nil
	case 8:
		return a << (uint32(b) & 31), nil
	case 9:
		return a >> (uint32(b) & 31), nil
	case 10:
		return int32(uint32(a) >> (uint32(b) & 31)), nil
	}
	return 0, fmt.Errorf("operación entera %d desconocida", k)
}

func larga(k byte, a, b int64) (int64, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		if b == 0 {
			return 0, fmt.Errorf("división de long por cero")
		}
		return a / b, nil
	case 4:
		if b == 0 {
			return 0, fmt.Errorf("resto de long por cero")
		}
		return a % b, nil
	case 5:
		return a & b, nil
	case 6:
		return a | b, nil
	case 7:
		return a ^ b, nil
	case 8:
		return a << (uint64(b) & 63), nil
	case 9:
		return a >> (uint64(b) & 63), nil
	case 10:
		return int64(uint64(a) >> (uint64(b) & 63)), nil
	}
	return 0, fmt.Errorf("operación de long %d desconocida", k)
}

func flotante(k byte, a, b float32) (float32, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		return a / b, nil
	case 4:
		return float32(math.Mod(float64(a), float64(b))), nil
	}
	return 0, nil
}

func doble(k byte, a, b float64) (float64, error) {
	switch k {
	case 0:
		return a + b, nil
	case 1:
		return a - b, nil
	case 2:
		return a * b, nil
	case 3:
		return a / b, nil
	case 4:
		return math.Mod(a, b), nil
	}
	return 0, nil
}

// enteraLit es el `lit16`/`lit8`: los mismos índices que `entera`, con la resta invertida
// (`rsub-int` es el literal menos el registro) y los desplazamientos al final.
func enteraLit(k byte, valor any, lit int32) (int32, error) {
	a, err := aInt32(valor)
	if err != nil {
		return 0, err
	}
	switch k {
	case 0:
		return a + lit, nil
	case 1:
		return lit - a, nil
	case 2:
		return a * lit, nil
	case 3:
		if lit == 0 {
			return 0, fmt.Errorf("división entera por cero")
		}
		return a / lit, nil
	case 4:
		if lit == 0 {
			return 0, fmt.Errorf("resto entero por cero")
		}
		return a % lit, nil
	case 5:
		return a & lit, nil
	case 6:
		return a | lit, nil
	case 7:
		return a ^ lit, nil
	case 8:
		return a << (uint32(lit) & 31), nil
	case 9:
		return a >> (uint32(lit) & 31), nil
	case 10:
		return int32(uint32(a) >> (uint32(lit) & 31)), nil
	}
	return 0, fmt.Errorf("operación con literal %d desconocida", k)
}
