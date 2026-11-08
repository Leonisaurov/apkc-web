package main

// Desugración de un `invokedynamic` de lambda. `javac --release 8` deja un sitio cuyo bootstrap es
// `LambdaMetafactory.metafactory` con tres argumentos: el tipo SAM, el `MethodHandle` del método que
// implementa y el tipo instanciado. `d8` lo convierte en una clase sintética con un campo por valor
// capturado, un constructor que los guarda y un método SAM que llama al implementador; aquí se hace
// lo mismo, sin copiar el nombre ni la anotación de `d8` (medido en los §109 y §111).
//
// Se cubren las lambdas estáticas con o sin capturas, las que capturan `this`, las referencias a
// métodos de instancia con receptor fijo (`this::m`), el SAM genérico con su descriptor borrado y
// `altMetafactory` en su forma medida: el flag serializable, los marcadores de una intersección y
// cero puentes. Lo que sigue fuera se rechaza por nombre: los puentes de `altMetafactory` y la
// concatenación de cadenas de JDK 9, que no son el mismo bootstrap.

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// classBootstrap es una fila de BootstrapMethods: el MethodHandle del bootstrap y sus argumentos,
// todos índices del pool.
type classBootstrap struct {
	handle int
	args   []int
}

// classLambda es el sitio desugrado: qué clase sintética construye el llamador, con qué constructor
// y cuántos valores captura (en orden). La clave es el pc del `invokedynamic` en su método.
type classLambda struct {
	clase    string   // descriptor, LLam$$Lambda$0;
	ctorDesc string   // descriptor del constructor, (I)V o (LLam;)V
	capturas []string // descriptores de los valores capturados, en el orden del sitio
}

// expandirLambdas sustituye cada `invokedynamic` de lambda por una clase sintética y anota el sitio
// en el método dueño. Las clases sintéticas se añaden detrás de las originales.
func expandirLambdas(clases []classFull) ([]classFull, error) {
	salida := make([]classFull, 0, len(clases))
	extras := []classFull{}
	for _, cf := range clases {
		if cf.p == nil {
			salida = append(salida, cf)
			continue
		}
		x := poolIndex{p: cf.p}
		if cf.Lambdas == nil {
			cf.Lambdas = map[int]classLambda{}
		}
		// `javac` deja `$deserializeLambda$` para que la JVM reconstruya una lambda serializable.
		// `d8` la quita y la clase sintética implementa Serializable a secas (medido en el §115);
		// aquí se quita igual, porque además su `invokedynamic` repetiría el sitio.
		sinDeserializar := cf.Methods[:0]
		for _, m := range cf.Methods {
			if m.Name == "$deserializeLambda$" && m.Access&accSynthetic != 0 {
				continue
			}
			sinDeserializar = append(sinDeserializar, m)
		}
		cf.Methods = sinDeserializar
		privados := map[methodKey]bool{}
		contador := 0
		for mi := range cf.Methods {
			m := &cf.Methods[mi]
			if !m.HasCode {
				continue
			}
			lista, err := walkJvmCode(m.Code)
			if err != nil {
				return nil, fmt.Errorf("%s%s: %w", cf.Name, m.Desc, err)
			}
			for _, in := range lista {
				if in.name != "invokedynamic" {
					continue
				}
				// La concatenación de JDK 9 es otro bootstrap: si el sitio es suyo, se anota en
				// `Concats` y no se crea ninguna clase sintética.
				if concat, es, err := sitioDeConcat(cf, x, int(binary.BigEndian.Uint16(in.ops))); err != nil {
					return nil, fmt.Errorf("%s%s pc %d: %w", cf.Name, m.Desc, in.pc, err)
				} else if es {
					if cf.Concats == nil {
						cf.Concats = map[int]classConcat{}
					}
					cf.Concats[in.pc] = concat
					continue
				}
				sitio, impl, err := sitioDeLambda(cf, x, int(binary.BigEndian.Uint16(in.ops)))
				if err != nil {
					return nil, fmt.Errorf("%s%s pc %d: %w", cf.Name, m.Desc, in.pc, err)
				}
				nombre := cf.Name[:len(cf.Name)-1] + "$$Lambda$" + itoa(contador)
				contador++
				extras = append(extras, claseDeLambda(nombre, sitio, impl))
				cf.Lambdas[in.pc] = classLambda{
					clase:    nombre + ";",
					ctorDesc: descriptorVacio(sitio.capturas),
					capturas: sitio.capturas,
				}
				if impl.acceso&accPrivate != 0 {
					privados[methodKey{owner: impl.owner, proto: protoKeyOf(impl.desc), desc: impl.desc, name: impl.nombre}] = true
				}
			}
		}
		// d8 quita `private` al implementador sintético: la clase de la lambda lo llama desde fuera
		// y el verificador de ART no acepta un privado ajeno (medido en el §109: `lambda$uno$0`
		// sale en el dex con 0x1008, sin private).
		if len(privados) > 0 {
			for mi := range cf.Methods {
				m := &cf.Methods[mi]
				if m.Access&accSynthetic == 0 {
					continue
				}
				if privados[methodKey{owner: cf.Name, proto: protoKeyOf(m.Desc), desc: m.Desc, name: m.Name}] {
					m.Access &^= accPrivate
				}
			}
		}
		salida = append(salida, cf)
	}
	return append(salida, extras...), nil
}

// lambdaSitio: lo que el bootstrap dice del sitio, ya resuelto. El descriptor SAM **no** es el del
// `invokedynamic`: el del sitio describe la fábrica (`(I)LLam$F;`) y el SAM sale del MethodType del
// bootstrap; sus parámetros son los valores capturados.
type lambdaSitio struct {
	samNombre    string
	samDesc      string // el descriptor ERASED que implementa la interfaz
	samInst      string // el tipo instanciado; sus parámetros son los del implementador
	interfaz     string
	marcadores   []string // las interfaces extra de `altMetafactory`, en su orden
	serializable bool
	capturas     []string
}

// implDeLambda: el método que implementa el cuerpo de la lambda.
type implDeLambda struct {
	owner  string
	nombre string
	desc   string
	acceso int
	kind   byte // 6 REF_invokeStatic, 5 REF_invokeVirtual, 7 REF_invokeSpecial
}

// descriptorVacio: el descriptor del constructor de la clase sintética, con los tipos capturados.
func descriptorVacio(capturas []string) string {
	return "(" + strings.Join(capturas, "") + ")V"
}

// sitioDeLambda valida el sitio y devuelve su nombre/descriptor SAM, los tipos capturados y el
// implementador. Se miden el estático (con o sin capturas) y el de instancia con receptor fijo.
func sitioDeLambda(cf classFull, x poolIndex, indice int) (lambdaSitio, implDeLambda, error) {
	boot, nombre, desc := x.invokeDynamicDe(indice)
	if desc == "" || len(desc) < 3 || desc[len(desc)-1] != ';' {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q no tiene un descriptor de fábrica con vuelta de referencia (%q)", nombre, desc)
	}
	_, ret, capturas := splitDescriptor(desc)
	if boot < 0 || boot >= len(cf.BootstrapMethods) {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q apunta al bootstrap %d, fuera de la tabla", nombre, boot)
	}
	bm := cf.BootstrapMethods[boot]
	_, owner, met, mdesc, _ := x.methodHandleDe(bm.handle)
	if owner != "Ljava/lang/invoke/LambdaMetafactory;" || (met != "metafactory" && met != "altMetafactory") {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el bootstrap del sitio %q es %s.%s.%s, y solo se miden LambdaMetafactory.metafactory y altMetafactory", nombre, owner, met, mdesc)
	}
	var (
		marcadores   []string
		serializable bool
		err          error
	)
	if met == "altMetafactory" {
		marcadores, serializable, err = argumentosAltMeta(x, nombre, bm.args)
		if err != nil {
			return lambdaSitio{}, implDeLambda{}, err
		}
	} else if len(bm.args) != 3 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el bootstrap del sitio %q lleva %d argumentos y no es altMetafactory", nombre, len(bm.args))
	}
	sam := x.methodTypeDe(bm.args[0])
	inst := x.methodTypeDe(bm.args[2])
	if sam == "" || inst == "" {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("los argumentos 0 y 2 del bootstrap del sitio %q no son MethodType", nombre)
	}
	_, retSam, paramsSam := splitDescriptor(sam)
	_, retInst, paramsInst := splitDescriptor(inst)
	if len(paramsSam) != len(paramsInst) {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q tiene %d parametros SAM y %d instanciados", nombre, len(paramsSam), len(paramsInst))
	}
	for i := range paramsSam {
		if paramsSam[i] == paramsInst[i] {
			continue
		}
		// el SAM puede borrar a una cota (`Number`) y el tipo instanciado ser más concreto
		// (`Integer`); el cuerpo moldea con checkcast. Los primitivos tienen que coincidir.
		if !esObjeto(paramsSam[i]) || !esObjeto(paramsInst[i]) {
			return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el SAM generico del sitio %q pasa de %s a %s: solo se mide el borrado entre referencias", nombre, paramsSam[i], paramsInst[i])
		}
	}
	if retSam != retInst && !(esObjeto(retSam) && esObjeto(retInst)) {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el SAM generico del sitio %q devuelve %s y el tipo instanciado %s", nombre, retSam, retInst)
	}
	implKind, implOwner, implNombre, implDesc, _ := x.methodHandleDe(bm.args[1])
	if implKind != 6 && implKind != 5 && implKind != 7 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador del sitio %q usa ref_kind %d: solo se miden el estatico, el virtual y el especial", nombre, implKind)
	}
	if implOwner != cf.Name {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador del sitio %q vive en %s; solo se mide el de la propia clase", nombre, implOwner)
	}
	_, retImpl, paramsImpl := splitDescriptor(implDesc)
	receptor := implKind == 5 || implKind == 7
	// `this::m` captura el receptor; `Tipo::m` lo deja como primer parámetro del SAM.
	receptorEnSam := false
	resto := capturas
	if receptor {
		switch {
		case len(capturas) > 0 && capturas[0] == implOwner:
			resto = capturas[1:]
		case len(capturas) == 0 && len(paramsSam) > 0 && paramsSam[0] == implOwner:
			receptorEnSam = true
			resto = nil
		default:
			return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador de instancia del sitio %q no tiene receptor: ni en las capturas %v ni en el primer parametro del SAM %s", nombre, capturas, sam)
		}
	}
	if retImpl != retInst {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q devuelve %s en el tipo instanciado y %s en el implementador", nombre, retInst, retImpl)
	}
	esperado := append([]string{}, resto...)
	if receptorEnSam {
		esperado = append(esperado, paramsInst[1:]...)
	} else {
		esperado = append(esperado, paramsInst...)
	}
	if strings.Join(esperado, "") != strings.Join(paramsImpl, "") {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("los valores capturados %v del sitio %q y el SAM instanciado %s no son los parametros del implementador %s", capturas, nombre, inst, implDesc)
	}
	acceso := -1
	for _, m := range cf.Methods {
		if m.Name == implNombre && m.Desc == implDesc {
			acceso = m.Access
			break
		}
	}
	if acceso < 0 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador %s%s del sitio %q no esta en la clase", implNombre, implDesc, nombre)
	}
	if receptor && acceso&accStatic != 0 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador %s del sitio %q es de instancia y esta marcado static", implNombre, nombre)
	}
	if !receptor && acceso&accStatic == 0 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador %s del sitio %q no es estatico", implNombre, nombre)
	}
	return lambdaSitio{
		samNombre: nombre, samDesc: sam, samInst: inst, interfaz: ret,
		marcadores: marcadores, serializable: serializable, capturas: capturas,
	}, implDeLambda{
		owner: implOwner, nombre: implNombre, desc: implDesc, acceso: acceso, kind: implKind,
	}, nil
}

// argumentosAltMeta lee lo que `altMetafactory` añade a los tres argumentos de `metafactory`:
// `flags`, los marcadores de una intersección (`(Runnable & Serializable & Cloneable)`) y los
// puentes. Se miden el flag serializable y los marcadores; los puentes, que `javac` deja en cero,
// se rechazan por nombre si alguna vez vienen. El orden que exige el dex no es el de los flags: la
// interfaz del sitio primero, luego los marcadores en su orden y `Serializable` al final (medido en
// el §115 contra `d8`).
func argumentosAltMeta(x poolIndex, nombre string, args []int) ([]string, bool, error) {
	const (
		flagSerializable = 1
		flagMarcadores   = 2
		flagPuentes      = 4
	)
	if len(args) < 4 || x.tagOf(args[3]) != cpInteger {
		return nil, false, fmt.Errorf("el sitio %q no trae los flags de altMetafactory", nombre)
	}
	flags := x.intOf(args[3])
	if flags&^(flagSerializable|flagMarcadores|flagPuentes) != 0 {
		return nil, false, fmt.Errorf("el sitio %q pide flags 0x%x en altMetafactory: no medido", nombre, flags)
	}
	i := 4
	var marcadores []string
	if flags&flagMarcadores != 0 {
		if i >= len(args) || x.tagOf(args[i]) != cpInteger {
			return nil, false, fmt.Errorf("el sitio %q no trae el numero de marcadores de altMetafactory", nombre)
		}
		n := int(x.intOf(args[i]))
		i++
		for j := 0; j < n; j++ {
			if i >= len(args) {
				return nil, false, fmt.Errorf("el sitio %q declara %d marcadores y solo trae %d", nombre, n, j)
			}
			d := x.classDescriptor(args[i])
			if d == "" {
				return nil, false, fmt.Errorf("el marcador %d del sitio %q no es una clase", j, nombre)
			}
			marcadores = append(marcadores, d)
			i++
		}
	}
	if flags&flagPuentes != 0 {
		if i >= len(args) || x.tagOf(args[i]) != cpInteger {
			return nil, false, fmt.Errorf("el sitio %q no trae el numero de puentes de altMetafactory", nombre)
		}
		n := int(x.intOf(args[i]))
		i++
		if n != 0 {
			return nil, false, fmt.Errorf("el sitio %q pide %d puentes en altMetafactory: no medido", nombre, n)
		}
	}
	if i != len(args) {
		return nil, false, fmt.Errorf("el sitio %q deja %d argumentos sin leer en altMetafactory", nombre, len(args)-i)
	}
	return marcadores, flags&flagSerializable != 0, nil
}

// poolDeLambda: el pool mínimo de la clase sintética. Se construye aquí y no se reutiliza el de la
// clase dueña porque los índices son de este fichero.
type poolDeLambda struct {
	p    *pool
	next int
}

func (b *poolDeLambda) add(tag byte, cuerpo []byte) int {
	b.next++
	b.p.tag[b.next] = tag
	if tag != cpUtf8 && tag != cpClass {
		b.p.body[b.next] = cuerpo
	}
	return b.next
}

func (b *poolDeLambda) utf8(s string) int {
	b.next++
	b.p.tag[b.next] = cpUtf8
	b.p.utf[b.next] = s
	return b.next
}

func (b *poolDeLambda) clase(internal string) int {
	i := b.add(cpClass, nil)
	b.p.class[i] = b.utf8(internal)
	return i
}

func (b *poolDeLambda) nat(nombre, desc string) int {
	return b.add(cpNameAndType, append(u2be(uint16(b.utf8(nombre))), u2be(uint16(b.utf8(desc)))...))
}

func (b *poolDeLambda) ref(owner, nombre, desc string) int {
	return b.add(cpMethodref, append(u2be(uint16(b.clase(owner))), u2be(uint16(b.nat(nombre, desc)))...))
}

func (b *poolDeLambda) campo(owner, nombre, desc string) int {
	return b.add(cpFieldref, append(u2be(uint16(b.clase(owner))), u2be(uint16(b.nat(nombre, desc)))...))
}

// claseDeDescriptor: la entrada CONSTANT_Class de un descriptor. Un arreglo se nombra con su propio
// descriptor (`[Ljava/lang/String;`); una clase, con el nombre interno.
func (b *poolDeLambda) claseDeDescriptor(desc string) int {
	if strings.HasPrefix(desc, "[") {
		return b.clase(desc)
	}
	return b.clase(desc[1 : len(desc)-1])
}

// cargaDeSlot: el `iload`/`lload`/… de un parámetro. El hueco lo cuenta el llamador, que es quien
// sabe cuánto ocupa cada tipo.
func cargaDeSlot(desc string, hueco int) []byte {
	switch {
	case desc == "J":
		return []byte{0x16, byte(hueco)}
	case desc == "F":
		return []byte{0x17, byte(hueco)}
	case desc == "D":
		return []byte{0x18, byte(hueco)}
	case esObjeto(desc):
		return []byte{0x19, byte(hueco)}
	default:
		return []byte{0x15, byte(hueco)}
	}
}

func retornoJVM(ret string) byte {
	switch {
	case ret == "V":
		return 0xb1
	case ret == "J":
		return 0xad
	case ret == "F":
		return 0xae
	case ret == "D":
		return 0xaf
	case esObjeto(ret):
		return 0xb0
	default:
		return 0xac
	}
}

func huecosDe(tipos []string) int {
	n := 0
	for _, t := range tipos {
		if esAncho(t) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// claseDeLambda escribe la clase sintética: `PUBLIC FINAL SYNTHETIC`, super Object, la interfaz SAM,
// un campo por captura, un constructor que las guarda y el método SAM que las carga y llama al
// implementador.
func claseDeLambda(descriptor string, sitio lambdaSitio, impl implDeLambda) classFull {
	samNombre, samDesc := sitio.samNombre, sitio.samDesc
	interno := descriptor[1:] // sin la L: el nombre interno que pide el pool
	b := &poolDeLambda{p: &pool{
		utf: map[int]string{}, class: map[int]int{}, tag: map[int]byte{}, body: map[int][]byte{},
	}}
	objectInit := b.ref("java/lang/Object", "<init>", "()V")
	implRef := b.ref(impl.owner[1:len(impl.owner)-1], impl.nombre, impl.desc)
	campos := make([]classField, 0, len(sitio.capturas))
	fieldRefs := make([]int, len(sitio.capturas))
	for i, cap := range sitio.capturas {
		nombre := "arg$" + itoa(i+1)
		campos = append(campos, classField{Access: accPrivate | accFinal | accSynthetic, Name: nombre, Desc: cap})
		fieldRefs[i] = b.campo(interno, nombre, cap)
	}

	// constructor: super() y después cada captura en su campo (mismo orden que el sitio)
	codigoInit := []byte{0x2a, 0xb7, byte(objectInit >> 8), byte(objectInit)}
	hueco := 1
	for i, cap := range sitio.capturas {
		codigoInit = append(codigoInit, 0x2a)
		codigoInit = append(codigoInit, cargaDeSlot(cap, hueco)...)
		codigoInit = append(codigoInit, 0xb5, byte(fieldRefs[i]>>8), byte(fieldRefs[i]))
		if esAncho(cap) {
			hueco += 2
		} else {
			hueco++
		}
	}
	codigoInit = append(codigoInit, 0xb1)
	initDesc := descriptorVacio(sitio.capturas)
	initArg := 1 + huecosDe(sitio.capturas)
	init := classMethod{
		Access: accPublic | accSynthetic, Name: "<init>", Desc: initDesc,
		HasCode: true, MaxStack: 2, LocalCount: initArg, ArgCount: initArg, Code: codigoInit,
	}

	_, ret, params := splitDescriptor(samDesc)
	_, _, paramsInst := splitDescriptor(sitio.samInst)
	codigo := []byte{}
	// el receptor de `this::m` es la primera captura; el de `Tipo::m` es el primer argumento del SAM
	receptor := impl.kind == 5 || impl.kind == 7
	receptorEnSam := receptor && len(sitio.capturas) == 0
	desde := 0
	if receptor && !receptorEnSam {
		codigo = append(codigo, 0x2a, 0xb4, byte(fieldRefs[0]>>8), byte(fieldRefs[0]))
		desde = 1
	}
	// las demás capturas se cargan de sus campos, antes de los argumentos del SAM
	for i := desde; i < len(sitio.capturas); i++ {
		codigo = append(codigo, 0x2a, 0xb4, byte(fieldRefs[i]>>8), byte(fieldRefs[i]))
	}
	hueco = 1
	for i, p := range params {
		codigo = append(codigo, cargaDeSlot(p, hueco)...)
		// el SAM borrado recibe Object: antes de llamar al implementador se moldea al tipo instanciado
		if i < len(paramsInst) && p != paramsInst[i] {
			idx := b.claseDeDescriptor(paramsInst[i])
			codigo = append(codigo, 0xc0, byte(idx>>8), byte(idx))
		}
		if esAncho(p) {
			hueco += 2
		} else {
			hueco++
		}
	}
	opInvoca := byte(0xb8) // invokestatic
	if receptor {
		opInvoca = 0xb6 // invokevirtual, ya sin private (como d8)
	}
	codigo = append(codigo, opInvoca, byte(implRef>>8), byte(implRef))
	codigo = append(codigo, retornoJVM(ret))
	samArg := hueco
	maxPila := 1 + huecosDe(sitio.capturas)
	if receptor {
		maxPila = huecosDe(sitio.capturas)
	}
	maxPila += huecosDe(params)
	sam := classMethod{
		Access: accPublic, Name: samNombre, Desc: samDesc,
		HasCode: true, MaxStack: maxPila, LocalCount: samArg, ArgCount: samArg, Code: codigo,
	}
	// d8 lista la interfaz del sitio, luego los marcadores de la intersección y `Serializable` al
	// final, sin quitar los repetidos que ya hereda la interfaz del sitio (§115).
	interfaces := append([]string{sitio.interfaz}, sitio.marcadores...)
	if sitio.serializable {
		interfaces = append(interfaces, "Ljava/io/Serializable;")
	}
	return classFull{
		Access:     accPublic | accFinal | accSynthetic,
		Name:       descriptor + ";",
		Super:      "Ljava/lang/Object;",
		Interfaces: interfaces,
		Fields:     campos,
		Methods:    []classMethod{init, sam},
		SourceFile: "",
		p:          b.p,
	}
}

// invokeDynamicDe: el índice de bootstrap y el NameAndType de un CONSTANT_InvokeDynamic (tag 18).
func (x poolIndex) invokeDynamicDe(i int) (boot int, nombre, desc string) {
	if x.p.tag[i] != cpInvokeDynamic {
		return -1, "", ""
	}
	b := x.p.body[i]
	if len(b) < 4 {
		return -1, "", ""
	}
	boot = int(b[0])<<8 | int(b[1])
	nombre, desc = x.nameAndType(int(b[2])<<8 | int(b[3]))
	return boot, nombre, desc
}

// methodHandleDe: ref_kind y destino de un CONSTANT_MethodHandle (tag 15).
func (x poolIndex) methodHandleDe(i int) (kind byte, owner, nombre, desc string, iface bool) {
	if x.p.tag[i] != cpMethodHandle {
		return 0, "", "", "", false
	}
	b := x.p.body[i]
	if len(b) < 3 {
		return 0, "", "", "", false
	}
	owner, nombre, desc, iface = x.refOf(int(b[1])<<8 | int(b[2]))
	return b[0], owner, nombre, desc, iface
}

// methodTypeDe: el descriptor de un CONSTANT_MethodType (tag 16).
func (x poolIndex) methodTypeDe(i int) string {
	if x.p.tag[i] != cpMethodType {
		return ""
	}
	b := x.p.body[i]
	if len(b) < 2 {
		return ""
	}
	return x.p.utf[int(b[0])<<8|int(b[1])]
}
