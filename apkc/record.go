package main

// Desugración del `invokedynamic` de un record (Java 16 o posterior). `javac --release 17` compila
// el `toString`, el `hashCode` y el `equals` de un record a un `invokedynamic` por método cuyo
// bootstrap es `java/lang/runtime/ObjectMethods.bootstrap`. Sus argumentos estáticos, medidos con
// `javap -v` sobre `record Punto(int x, int y)`, son el `Class` del record, la lista de componentes
// separada por `;` (`"x;y"`) y un `REF_getField` por componente; el atributo `Record` no hace falta
// para nada de esto, así que no se lee.
//
// `d8` lo desugra con dos clases `$$ExternalSyntheticRecord` que arman el texto con
// `Class.getSimpleName()` y `String.split` y calculan el hash con `Class.hashCode()`. Aquí se toma
// la variante directa: el texto se concatena con los nombres literales del bootstrap y el
// `hashCode` y el `equals` van por `Arrays.hashCode` y `Arrays.equals` sobre el arreglo boxeado de
// componentes. Los cuatro auxiliares viven en la clase dueña, como los `$record$…` de `d8`.
//
// La superclase es la otra mitad del desugrado: un record extiende `java/lang/Record`, que en
// Android solo existe desde la API 34. `d8` reescribe el `super` a `com/android/tools/r8/RecordTag`
// (una clase abstracta sintética con un constructor protegido) y aquí se hace lo mismo, para no
// atar los records a la API 34.

import (
	"fmt"
	"strings"
)

const (
	// recordTagNombre es el nombre interno de la superclase sintética que `d8` pone a un record
	// desugrado por debajo de la API 34 (medido con `dexdump` sobre `--min-api 26`).
	recordTagNombre = "com/android/tools/r8/RecordTag"
	recordTagDesc   = "Lcom/android/tools/r8/RecordTag;"

	recordToString = "$record$toString"
	recordHashCode = "$record$hashCode"
	recordEquals   = "$record$equals"
	recordCampos   = "$record$getFieldsAsObjects"
)

// recordComponente es un componente del record: el nombre que trae el bootstrap y el tipo del
// campo, que es el que decide el `append` del texto y la caja del `hashCode`.
type recordComponente struct {
	nombre string
	desc   string
}

// recordSitio es el bootstrap ya validado: qué método del record es, sus componentes, el nombre
// simple que va en el texto y el descriptor del sitio.
type recordSitio struct {
	metodo      string
	componentes []recordComponente
	simple      string
	desc        string
}

// recordPendiente: un sitio de record que la expansión ya reconoció y que se anota en la clase
// cuando la pasada por sus métodos ha terminado, para no tocar `Methods` mientras se recorre.
type recordPendiente struct {
	clave sitioKey
	sitio recordSitio
}

// classRecord es lo que el traductor necesita del sitio: el auxiliar al que se llama y el
// descriptor del sitio, del que salen los argumentos que hay en la pila.
type classRecord struct {
	ayuda     string
	ayudaDesc string
	desc      string
}

// sitioDeRecord reconoce el bootstrap de `ObjectMethods`. Devuelve `es` en falso cuando el sitio no
// es suyo, para que `sitioDeLambda` lo intente; si es suyo y no está medido, el error lo nombra.
func sitioDeRecord(cf classFull, x poolIndex, indice int) (recordSitio, bool, error) {
	boot, nombre, desc := x.invokeDynamicDe(indice)
	if boot < 0 || boot >= len(cf.BootstrapMethods) {
		return recordSitio{}, false, nil
	}
	bm := cf.BootstrapMethods[boot]
	_, owner, met, _, _ := x.methodHandleDe(bm.handle)
	if owner != "Ljava/lang/runtime/ObjectMethods;" || met != "bootstrap" {
		return recordSitio{}, false, nil
	}
	// a partir de aquí el sitio es de un record: o se mide, o se rechaza nombrando la forma
	if nombre != "toString" && nombre != "hashCode" && nombre != "equals" {
		return recordSitio{}, true, fmt.Errorf("el bootstrap ObjectMethods.bootstrap del sitio %q no es toString, hashCode ni equals: no medido", nombre)
	}
	if cf.Super != "Ljava/lang/Record;" {
		return recordSitio{}, true, fmt.Errorf("el sitio %q usa ObjectMethods.bootstrap y %s no extiende java/lang/Record", nombre, cf.Name)
	}
	if len(bm.args) < 2 || x.tagOf(bm.args[0]) != cpClass || x.tagOf(bm.args[1]) != cpString {
		return recordSitio{}, true, fmt.Errorf("el sitio %q no trae el Class del record y la lista de componentes de ObjectMethods", nombre)
	}
	if clase := x.classDescriptor(bm.args[0]); clase != cf.Name {
		return recordSitio{}, true, fmt.Errorf("el sitio %q nombra el record %s y vive en %s", nombre, clase, cf.Name)
	}
	nombres := x.stringValue(bm.args[1])
	var partes []string
	if nombres != "" {
		partes = strings.Split(nombres, ";")
	}
	if len(bm.args) != 2+len(partes) {
		return recordSitio{}, true, fmt.Errorf("el sitio %q nombra %d componentes y trae %d handles", nombre, len(partes), len(bm.args)-2)
	}
	componentes := make([]recordComponente, 0, len(partes))
	for i, parte := range partes {
		kind, hOwner, hNombre, hDesc, _ := x.methodHandleDe(bm.args[2+i])
		if kind != 1 || hOwner != cf.Name || hNombre != parte {
			return recordSitio{}, true, fmt.Errorf("el handle %d del sitio %q no es el REF_getField de %q", i, nombre, parte)
		}
		descCampo := ""
		for _, f := range cf.Fields {
			if f.Name == parte {
				descCampo = f.Desc
				break
			}
		}
		if descCampo == "" {
			return recordSitio{}, true, fmt.Errorf("el componente %q del sitio %q no es un campo de %s", parte, nombre, cf.Name)
		}
		if hDesc != descCampo {
			return recordSitio{}, true, fmt.Errorf("el handle del componente %q del sitio %q lleva %s y el campo es %s", parte, nombre, hDesc, descCampo)
		}
		componentes = append(componentes, recordComponente{nombre: parte, desc: descCampo})
	}
	_, ret, params := splitDescriptor(desc)
	if len(params) == 0 || params[0] != cf.Name {
		return recordSitio{}, true, fmt.Errorf("el sitio %q no recibe el record como primer argumento (%s)", nombre, desc)
	}
	switch nombre {
	case "toString":
		if len(params) != 1 || ret != "Ljava/lang/String;" {
			return recordSitio{}, true, fmt.Errorf("el sitio toString del record %s tiene el descriptor %s y se espera uno de vuelta String", cf.Name, desc)
		}
	case "hashCode":
		if len(params) != 1 || ret != "I" {
			return recordSitio{}, true, fmt.Errorf("el sitio hashCode del record %s tiene el descriptor %s y se espera uno de vuelta int", cf.Name, desc)
		}
	case "equals":
		if len(params) != 2 || params[1] != "Ljava/lang/Object;" || ret != "Z" {
			return recordSitio{}, true, fmt.Errorf("el sitio equals del record %s tiene el descriptor %s y se espera (L;Ljava/lang/Object;)Z", cf.Name, desc)
		}
	}
	return recordSitio{metodo: nombre, componentes: componentes, simple: nombreSimple(cf), desc: desc}, true, nil
}

// nombreSimple es el que `d8` escribe en el texto del record, que sale de `Class.getSimpleName()`:
// el nombre interno sin el paquete y, si la clase es anidada, sin la clase exterior. El
// InnerClasses es el que dice si es anidada; el nombre interno de una clase de primer nivel puede
// llevar `$` sin serlo, y ahí `getSimpleName` no lo quita.
func nombreSimple(cf classFull) string {
	interno := cf.Name[1 : len(cf.Name)-1]
	if i := strings.LastIndexByte(interno, '/'); i >= 0 {
		interno = interno[i+1:]
	}
	for _, in := range cf.Inners {
		if in.inner == cf.Name && in.name != "" {
			return in.name
		}
	}
	return interno
}

// ayudaDeRecord: el auxiliar de la clase dueña que resuelve el sitio, y su descriptor.
func ayudaDeRecord(metodo string) (string, string) {
	switch metodo {
	case "toString":
		return recordToString, "()Ljava/lang/String;"
	case "hashCode":
		return recordHashCode, "()I"
	}
	return recordEquals, "(Ljava/lang/Object;)Z"
}

// poolDeClase es un escritor de entradas NUEVAS sobre el pool de una clase ya leída. El pool del
// `.class` es un mapa por índice y los anchos dejan el hueco siguiente sin etiqueta, así que el
// primero libre es el mayor índice más uno, no `len(tag)`.
func poolDeClase(cf *classFull) *poolDeLambda {
	max := 0
	for i := range cf.p.tag {
		if i > max {
			max = i
		}
	}
	return &poolDeLambda{p: cf.p, next: max}
}

// preparaRecord deja la clase dueña con lo que el sitio necesita: el `super` pasa a `RecordTag` y
// se añaden los cuatro auxiliares privados. La clase ya no puede llamarse `java/lang/Record` en
// ningún `invokespecial`, así que el constructor se reescribe en su propio `Code`.
func preparaRecord(cf *classFull, rec recordSitio) error {
	if cf.p == nil {
		return fmt.Errorf("la clase del record no trae su pool")
	}
	for _, nombre := range []string{recordToString, recordHashCode, recordEquals, recordCampos} {
		for _, m := range cf.Methods {
			if m.Name == nombre {
				return fmt.Errorf("la clase ya declara %s: el desugrado del record no puede añadirlo", nombre)
			}
		}
	}
	b := poolDeClase(cf)
	if err := reescribeSuperDeRecord(cf, b); err != nil {
		return err
	}
	cf.Methods = append(cf.Methods,
		metodoDeRecord(recordCampos, "()[Ljava/lang/Object;", 1, 2, 6, cuerpoCampos(*cf, rec, b)),
		metodoDeRecord(recordToString, "()Ljava/lang/String;", 1, 2, 6, cuerpoToString(*cf, rec, b)),
		metodoDeRecord(recordHashCode, "()I", 1, 5, 4, cuerpoHashCode(*cf, b)),
		metodoDeRecord(recordEquals, "(Ljava/lang/Object;)Z", 2, 2, 4, cuerpoEquals(*cf, b)),
	)
	cf.Super = recordTagDesc
	return nil
}

// metodoDeRecord: un auxiliar privado y sintético, con el `Code` ya escrito y sin líneas ni
// parámetros: es lo que `d8` escribe para los suyos.
func metodoDeRecord(nombre, desc string, args, locales, pila int, codigo []byte) classMethod {
	return classMethod{
		Access: accPrivate | accSynthetic, Name: nombre, Desc: desc,
		HasCode: true, MaxStack: pila, LocalCount: locales, ArgCount: args, Code: codigo,
	}
}

// reescribeSuperDeRecord cambia el `invokespecial java/lang/Record.<init>` del constructor por el
// de `RecordTag`, que es el `super` nuevo. El operando mide lo mismo (dos bytes), así que el
// `Code` no se mueve y los saltos y el `StackMapTable` del `.class` siguen valiendo.
func reescribeSuperDeRecord(cf *classFull, b *poolDeLambda) error {
	nuevo := b.ref(recordTagNombre, "<init>", "()V")
	x := poolIndex{p: cf.p}
	parcheado := false
	for mi := range cf.Methods {
		m := &cf.Methods[mi]
		if m.Name != "<init>" || !m.HasCode {
			continue
		}
		lista, err := walkJvmCode(m.Code)
		if err != nil {
			return fmt.Errorf("%s<init>: %w", cf.Name, err)
		}
		for _, in := range lista {
			if in.name != "invokespecial" || len(in.ops) < 2 {
				continue
			}
			owner, nombre, desc, _ := x.refOf(dexU16(in.ops))
			if owner != "Ljava/lang/Record;" || nombre != "<init>" || desc != "()V" {
				continue
			}
			in.ops[0] = byte(nuevo >> 8)
			in.ops[1] = byte(nuevo)
			parcheado = true
		}
	}
	if !parcheado {
		return fmt.Errorf("el constructor de %s no llama a java/lang/Record.<init> y no se puede reescribir el super", cf.Name)
	}
	return nil
}

// claseRecordTag es la superclase sintética que `d8` añade con `--min-api` por debajo de 34:
// PUBLIC ABSTRACT SYNTHETIC, super Object y un `<init>()V` protegido (medido con `dexdump`).
func claseRecordTag() classFull {
	b := &poolDeLambda{p: &pool{
		utf: map[int]string{}, class: map[int]int{}, tag: map[int]byte{}, body: map[int][]byte{},
	}}
	init := b.ref("java/lang/Object", "<init>", "()V")
	return classFull{
		Access: accPublic | accAbstract | accSynthetic,
		Name:   recordTagDesc,
		Super:  "Ljava/lang/Object;",
		Methods: []classMethod{{
			Access: accProtected | accSynthetic, Name: "<init>", Desc: "()V",
			HasCode: true, MaxStack: 1, LocalCount: 1, ArgCount: 1,
			Code: []byte{0x2a, 0xb7, byte(init >> 8), byte(init), 0xb1},
		}},
		p: b.p,
	}
}

// cajaDeTipo: la clase envoltorio de un primitivo y su `valueOf`, que es lo que `d8` escribe en
// `$record$getFieldsAsObjects` (medido con `dexdump` sobre `PuntoMix`).
type cajaDeTipo struct {
	owner string
	desc  string
}

func cajaDe(desc string) (cajaDeTipo, bool) {
	switch desc {
	case "Z":
		return cajaDeTipo{"java/lang/Boolean", "(Z)Ljava/lang/Boolean;"}, true
	case "B":
		return cajaDeTipo{"java/lang/Byte", "(B)Ljava/lang/Byte;"}, true
	case "C":
		return cajaDeTipo{"java/lang/Character", "(C)Ljava/lang/Character;"}, true
	case "S":
		return cajaDeTipo{"java/lang/Short", "(S)Ljava/lang/Short;"}, true
	case "I":
		return cajaDeTipo{"java/lang/Integer", "(I)Ljava/lang/Integer;"}, true
	case "J":
		return cajaDeTipo{"java/lang/Long", "(J)Ljava/lang/Long;"}, true
	case "F":
		return cajaDeTipo{"java/lang/Float", "(F)Ljava/lang/Float;"}, true
	case "D":
		return cajaDeTipo{"java/lang/Double", "(D)Ljava/lang/Double;"}, true
	}
	return cajaDeTipo{}, false
}

// emisorJVM escribe el `Code` de los auxiliares con las instrucciones del JVMS que hacen falta.
// Los índices del pool se piden a `b`, que los añade a la clase dueña.
type emisorJVM struct {
	b    *poolDeLambda
	code []byte
}

func (e *emisorJVM) u1(v byte) { e.code = append(e.code, v) }

func (e *emisorJVM) u2(v int) { e.code = append(e.code, byte(v>>8), byte(v)) }

// orden: un opcode con un índice de pool de dos bytes.
func (e *emisorJVM) orden(op byte, idx int) { e.u1(op); e.u2(idx) }

// entero: `sipush` (0x11), que sirve para cualquier valor de 16 bits y evita el signo de `bipush`.
func (e *emisorJVM) entero(v int) { e.u1(0x11); e.u2(v) }

func (e *emisorJVM) getfield(owner, nombre, desc string) {
	e.orden(0xb4, e.b.campo(owner, nombre, desc))
}

func (e *emisorJVM) invoca(op byte, owner, nombre, desc string) {
	e.orden(op, e.b.ref(owner, nombre, desc))
}

// ponU2 escribe un operando de dos bytes ya emitido: se usa para cerrar los saltos cuando ya se
// sabe dónde cae su destino.
func (e *emisorJVM) ponU2(pos, v int) {
	e.code[pos] = byte(v >> 8)
	e.code[pos+1] = byte(v)
}

// cuerpoCampos: `$record$getFieldsAsObjects()[Ljava/lang/Object;`. Crea el arreglo y va guardando
// cada componente en su hueco, boxeado si es primitivo. `d8` usa `filled-new-array`; aquí se llega
// al mismo `[Ljava/lang/Object;` con `new-array` y un `aput-object` por componente, que es lo que
// el traductor ya sabe bajar.
func cuerpoCampos(cf classFull, rec recordSitio, b *poolDeLambda) []byte {
	e := &emisorJVM{b: b}
	owner := cf.Name[1 : len(cf.Name)-1]
	e.entero(len(rec.componentes))
	e.orden(0xbd, b.claseDeDescriptor("Ljava/lang/Object;")) // anewarray
	e.u1(0x4c)                                               // astore_1
	for i, c := range rec.componentes {
		e.u1(0x2b) // aload_1
		e.entero(i)
		e.u1(0x2a) // aload_0
		e.getfield(owner, c.nombre, c.desc)
		if caja, ok := cajaDe(c.desc); ok {
			e.invoca(0xb8, caja.owner, "valueOf", caja.desc)
		}
		e.u1(0x53) // aastore
	}
	e.u1(0x2b) // aload_1
	e.u1(0xb0) // areturn
	return e.code
}

// cuerpoToString: `$record$toString()Ljava/lang/String;`, el texto del record con los nombres del
// bootstrap: `Punto[x=1, y=2]`. Sin `split` ni `getSimpleName`: el nombre simple ya está aquí.
func cuerpoToString(cf classFull, rec recordSitio, b *poolDeLambda) []byte {
	e := &emisorJVM{b: b}
	owner := cf.Name[1 : len(cf.Name)-1]
	e.orden(0xbb, b.clase("java/lang/StringBuilder")) // new
	e.u1(0x59)                                        // dup
	e.invoca(0xb7, "java/lang/StringBuilder", "<init>", "()V")
	e.u1(0x4c) // astore_1
	anade := func(texto string) {
		e.u1(0x2b)                     // aload_1
		e.orden(0x13, b.cadena(texto)) // ldc_w
		e.invoca(0xb6, "java/lang/StringBuilder", "append", "(Ljava/lang/String;)Ljava/lang/StringBuilder;")
		e.u1(0x57) // pop
	}
	anade(rec.simple + "[")
	for i, c := range rec.componentes {
		if i > 0 {
			anade(", ")
		}
		anade(c.nombre + "=")
		e.u1(0x2b) // aload_1
		e.u1(0x2a) // aload_0
		e.getfield(owner, c.nombre, c.desc)
		e.invoca(0xb6, "java/lang/StringBuilder", "append", appendDeTipo(c.desc))
		e.u1(0x57) // pop
	}
	anade("]")
	e.u1(0x2b) // aload_1
	e.invoca(0xb6, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
	e.u1(0xb0) // areturn
	return e.code
}

// cuerpoHashCode: `$record$hashCode()I`, con la fórmula del `ObjectMethods.bootstrap` de la JVM:
// `hash = 31 * hash + hashCode(componente)`, empezando en cero y con el nulo valiendo cero. Medido
// con `java` sobre `record Dos(int a, int b)`: `Dos(1, 2)` da 33, que es `31*1 + 2`.
//
// `d8` NO da esto: sintetiza `31 * Arrays.hashCode(componentes) + getClass().hashCode()`, que sobre
// el mismo `Dos(1, 2)` da `31*994 + …`, porque `Arrays.hashCode` empieza en uno. Se copia la JVM y
// no `d8` porque el oráculo del repo es la JVM (`banco/`), y el contrato de un record es el de la
// JVM. El bucle escribe el acumulador en un local y no deja ningún valor en la pila al saltar: así
// los dos caminos del `if` se juntan con la pila vacía, que es lo que el traductor sabe seguir.
func cuerpoHashCode(cf classFull, b *poolDeLambda) []byte {
	e := &emisorJVM{b: b}
	owner := cf.Name[1 : len(cf.Name)-1]
	e.u1(0x2a) // aload_0
	e.invoca(0xb7, owner, recordCampos, "()[Ljava/lang/Object;")
	e.u1(0x4c) // astore_1
	e.u1(0x03) // iconst_0
	e.u1(0x3d) // istore_2: hash = 0
	e.u1(0x03) // iconst_0
	e.u1(0x3e) // istore_3: i = 0
	bucle := len(e.code)
	e.u1(0x1d) // iload_3
	e.u1(0x2b) // aload_1
	e.u1(0xbe) // arraylength
	salir := len(e.code)
	e.u1(0xa2) // if_icmpge
	e.u2(0)
	e.u1(0x2b) // aload_1
	e.u1(0x1d) // iload_3
	e.u1(0x32) // aaload
	e.u1(0x3a) // astore
	e.u1(0x04)
	e.u1(0x19) // aload
	e.u1(0x04)
	nulo := len(e.code)
	e.u1(0xc7) // ifnonnull
	e.u2(0)
	e.u1(0x1c) // iload_2
	e.entero(31)
	e.u1(0x68) // imul
	e.u1(0x3d) // istore_2: hash = 31 * hash + 0
	trasNulo := len(e.code)
	e.u1(0xa7) // goto
	e.u2(0)
	noNulo := len(e.code)
	e.u1(0x1c) // iload_2
	e.entero(31)
	e.u1(0x68) // imul
	e.u1(0x19) // aload
	e.u1(0x04)
	e.invoca(0xb6, "java/lang/Object", "hashCode", "()I")
	e.u1(0x60) // iadd
	e.u1(0x3d) // istore_2
	siguiente := len(e.code)
	e.u1(0x84) // iinc
	e.u1(0x03)
	e.u1(0x01)
	e.u1(0xa7) // goto bucle
	e.u2(bucle - len(e.code))
	fin := len(e.code)
	e.u1(0x1c) // iload_2
	e.u1(0xac) // ireturn
	e.ponU2(salir+1, fin-salir)
	e.ponU2(nulo+1, noNulo-nulo)
	e.ponU2(trasNulo+1, siguiente-trasNulo)
	return e.code
}

// cuerpoEquals: `$record$equals(Ljava/lang/Object;)Z`. Un record es final, así que la clase del
// otro se compara con `getClass`, como `d8`, y antes se descarta el nulo: `d8` no lo hace y su
// `$record$equals` revienta con `equals(null)`, que aquí se devuelve como falso.
func cuerpoEquals(cf classFull, b *poolDeLambda) []byte {
	e := &emisorJVM{b: b}
	owner := cf.Name[1 : len(cf.Name)-1]
	e.u1(0x2b) // aload_1
	nulo := len(e.code)
	e.u1(0xc6) // ifnull
	e.u2(0)
	e.u1(0x2a) // aload_0
	e.invoca(0xb6, "java/lang/Object", "getClass", "()Ljava/lang/Class;")
	e.u1(0x2b) // aload_1
	e.invoca(0xb6, "java/lang/Object", "getClass", "()Ljava/lang/Class;")
	clase := len(e.code)
	e.u1(0xa6) // if_acmpne
	e.u2(0)
	e.u1(0x2a) // aload_0
	e.invoca(0xb7, owner, recordCampos, "()[Ljava/lang/Object;")
	e.u1(0x2b)                                  // aload_1
	e.orden(0xc0, b.claseDeDescriptor(cf.Name)) // checkcast
	e.invoca(0xb7, owner, recordCampos, "()[Ljava/lang/Object;")
	e.invoca(0xb8, "java/util/Arrays", "equals", "([Ljava/lang/Object;[Ljava/lang/Object;)Z")
	e.u1(0xac) // ireturn
	falso := len(e.code)
	e.u1(0x03) // iconst_0
	e.u1(0xac) // ireturn
	e.ponU2(nulo+1, falso-nulo)
	e.ponU2(clase+1, falso-clase)
	return e.code
}

// record: el `invokedynamic` de un record ya desugrado en `expandirLambdas`. Los argumentos del
// sitio (el receptor y, en `equals`, el otro objeto) están en la pila JVM; se sacan en orden
// inverso, se llama al auxiliar privado de la clase con `invoke-direct` y se deja el resultado en
// un registro nuevo. Es la misma forma que `lambda` y `concat`.
func (p *traduce) record(in jvmInstr) {
	sitio, ok := p.clase.Records[claveSitio(p.m, in.pc)]
	if !ok {
		p.falla(in, "an invokedynamic that the record expansion did not cover")
		return
	}
	metodo, ok := p.t.methodIdx[methodKey{owner: p.clase.Name, proto: protoKeyOf(sitio.ayudaDesc), desc: sitio.ayudaDesc, name: sitio.ayuda}]
	if !ok {
		p.falla(in, "the record helper "+sitio.ayuda+sitio.ayudaDesc+" was never interned")
		return
	}
	_, ret, params := splitDescriptor(sitio.desc)
	// El registro del resultado se toma ANTES de sacar los argumentos: `toma` solo evita los
	// registros que la pila todavía nombra, y después del `pop` los suyos quedarían libres.
	r := p.toma(in)
	if p.err != nil {
		return
	}
	valores := make([]int, len(params))
	for i := len(valores) - 1; i >= 0; i-- {
		valores[i] = p.pop(in)
	}
	if p.err != nil {
		return
	}
	regs := make([]int, 0, len(valores))
	anchos := make([]bool, 0, len(valores))
	refs := make([]bool, 0, len(valores))
	for i, t := range params {
		regs = append(regs, valores[i])
		anchos = append(anchos, esAncho(t))
		refs = append(refs, esObjeto(t))
		if esAncho(t) {
			regs = append(regs, valores[i]+1)
			anchos = append(anchos, true)
			refs = append(refs, false)
		}
	}
	p.invocaConcat(opInvocaDirecto, regs, anchos, refs, metodo, in)
	if p.err != nil {
		return
	}
	switch {
	case esAncho(ret):
		p.palabra(unidad11x(opMueveResultWide, r))
		p.pushAncho(r)
	case esObjeto(ret):
		p.palabra(unidad11x(opMueveResultObj, r))
		p.push(r)
		p.ponMoveCima(opMueveObjeto)
	default:
		p.palabra(unidad11x(opMueveResult, r))
		p.push(r)
	}
	for i, v := range valores {
		if esAncho(params[i]) {
			p.sueltaPar(v)
			continue
		}
		p.suelta(v)
	}
}
