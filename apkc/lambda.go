package main

// Desugración de un `invokedynamic` de lambda sin capturas: el único caso de 5.22 que este fork
// escribe. Lo que `javac --release 8` deja para `x -> x + 1` en un método `static` es un
// `invokedynamic` cuyo bootstrap es `LambdaMetafactory.metafactory` con tres argumentos: el tipo
// SAM, el `MethodHandle` del método que implementa y el tipo instanciado. `d8` lo convierte en una
// clase sintética con constructor `()V` y un método SAM que llama al implementador; aquí se hace lo
// mismo, sin copiar el nombre ni la anotación de `d8` (medido en el §109, `Lam$2` y `lambda$uno$0`).
//
// El resto de `invokedynamic` (capturas, referencias de método de instancia, `altMetafactory` y la
// concatenación de cadenas de JDK 9) se rechaza por nombre: no se escribe un dex a medias.

import (
	"encoding/binary"
	"fmt"
)

// classBootstrap es una fila de BootstrapMethods: el MethodHandle del bootstrap y sus argumentos,
// todos índices del pool.
type classBootstrap struct {
	handle int
	args   []int
}

// classLambda es el sitio desugrado de una clase: qué clase sintética construye y con qué nombre se
// llama a su constructor. La clave es el pc del `invokedynamic` en su método.
type classLambda struct {
	clase string // descriptor, LLam$$Lambda$0;
}

// expandirLambdas sustituye cada `invokedynamic` de lambda sin capturas por una clase sintética y
// anota el sitio en el método dueño. Las clases sintéticas se añaden detrás de las originales.
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
				sitio, impl, err := sitioDeLambda(cf, x, int(binary.BigEndian.Uint16(in.ops)))
				if err != nil {
					return nil, fmt.Errorf("%s%s pc %d: %w", cf.Name, m.Desc, in.pc, err)
				}
				nombre := cf.Name[:len(cf.Name)-1] + "$$Lambda$" + itoa(contador)
				contador++
				extras = append(extras, claseDeLambda(nombre, sitio, impl))
				cf.Lambdas[in.pc] = classLambda{clase: nombre + ";"}
				if impl.acceso&accPrivate != 0 {
					privados[methodKey{owner: impl.owner, proto: protoKeyOf(impl.desc), desc: impl.desc, name: impl.nombre}] = true
				}
			}
		}
		// d8 quita `private` al implementador sintético: la clase de la lambda lo llama con
		// invoke-static desde fuera y el verificador de ART no acepta un privado ajeno (medido en
		// el §109: `lambda$uno$0` sale en el dex con 0x1008, sin private).
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
// `invokedynamic`: el del sitio describe la fábrica (`()LLam$F;`) y el SAM sale del MethodType del
// bootstrap (medido en el §109; confundirlos hacía que el cuerpo del SAM se tradujera con los
// argumentos de la fábrica).
type lambdaSitio struct {
	samNombre string
	samDesc   string
	interfaz  string
}

// implDeLambda: el método que implementa el cuerpo de la lambda.
type implDeLambda struct {
	owner  string
	nombre string
	desc   string
	acceso int
}

// sitioDeLambda valida que el `invokedynamic` sea una lambda sin capturas cuyo implementador sea un
// método estático de la propia clase. Devuelve el nombre/descriptor SAM y el implementador.
func sitioDeLambda(cf classFull, x poolIndex, indice int) (lambdaSitio, implDeLambda, error) {
	boot, nombre, desc := x.invokeDynamicDe(indice)
	if desc == "" || len(desc) < 3 || desc[len(desc)-1] != ';' {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q no tiene un descriptor de fábrica con vuelta de referencia (%q)", nombre, desc)
	}
	_, ret, params := splitDescriptor(desc)
	if len(params) != 0 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("la lambda %q captura %d valores: solo se mide la lambda estática sin capturas", nombre, len(params))
	}
	if boot < 0 || boot >= len(cf.BootstrapMethods) {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q apunta al bootstrap %d, fuera de la tabla", nombre, boot)
	}
	bm := cf.BootstrapMethods[boot]
	_, owner, met, mdesc, _ := x.methodHandleDe(bm.handle)
	if owner != "Ljava/lang/invoke/LambdaMetafactory;" || met != "metafactory" {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el bootstrap del sitio %q es %s.%s.%s, y solo se mide LambdaMetafactory.metafactory", nombre, owner, met, mdesc)
	}
	if len(bm.args) != 3 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el bootstrap del sitio %q lleva %d argumentos (altMetafactory o serialización): no medido", nombre, len(bm.args))
	}
	sam := x.methodTypeDe(bm.args[0])
	inst := x.methodTypeDe(bm.args[2])
	if sam == "" || inst == "" {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("los argumentos 0 y 2 del bootstrap del sitio %q no son MethodType", nombre)
	}
	if sam != inst {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el sitio %q tiene tipo SAM %q y tipo instanciado %q: el SAM generico (con casting) no se mide", nombre, sam, inst)
	}
	implKind, implOwner, implNombre, implDesc, _ := x.methodHandleDe(bm.args[1])
	if implKind != 6 { // REF_invokeStatic
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador del sitio %q usa ref_kind %d: solo se mide REF_invokeStatic", nombre, implKind)
	}
	if implOwner != cf.Name {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador del sitio %q vive en %s; solo se mide el estatico de la propia clase", nombre, implOwner)
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
	if acceso&accStatic == 0 {
		return lambdaSitio{}, implDeLambda{}, fmt.Errorf("el implementador %s del sitio %q no es estatico", implNombre, nombre)
	}
	return lambdaSitio{samNombre: nombre, samDesc: sam, interfaz: ret}, implDeLambda{
		owner: implOwner, nombre: implNombre, desc: implDesc, acceso: acceso,
	}, nil
}

// poolDeLambda: un pool mínimo con Object.<init>()V y el método que implementa la lambda. Se
// construye aquí y no se reutiliza el de la clase dueña porque los índices son de este fichero.
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

// claseDeLambda escribe la clase sintética: `PUBLIC FINAL SYNTHETIC`, super Object, la interfaz SAM,
// constructor `()V` y el método SAM que llama al implementador estático.
func claseDeLambda(descriptor string, sitio lambdaSitio, impl implDeLambda) classFull {
	samNombre, samDesc := sitio.samNombre, sitio.samDesc
	b := &poolDeLambda{p: &pool{
		utf: map[int]string{}, class: map[int]int{}, tag: map[int]byte{}, body: map[int][]byte{},
	}}
	objectInit := b.ref("java/lang/Object", "<init>", "()V")
	implRef := b.ref(impl.owner[1:len(impl.owner)-1], impl.nombre, impl.desc)

	codigoInit := []byte{0x2a, 0xb7, byte(objectInit >> 8), byte(objectInit), 0xb1}
	init := classMethod{
		Access: accPublic | accSynthetic, Name: "<init>", Desc: "()V",
		HasCode: true, MaxStack: 1, LocalCount: 1, ArgCount: 1, Code: codigoInit,
	}

	_, ret, params := splitDescriptor(samDesc)
	codigo := []byte{}
	hueco := 1
	for _, p := range params {
		switch {
		case p == "J":
			codigo = append(codigo, 0x16, byte(hueco))
		case p == "F":
			codigo = append(codigo, 0x17, byte(hueco))
		case p == "D":
			codigo = append(codigo, 0x18, byte(hueco))
		case esObjeto(p):
			codigo = append(codigo, 0x19, byte(hueco))
		default:
			codigo = append(codigo, 0x15, byte(hueco))
		}
		if esAncho(p) {
			hueco += 2
		} else {
			hueco++
		}
	}
	codigo = append(codigo, 0xb8, byte(implRef>>8), byte(implRef))
	switch {
	case ret == "V":
		codigo = append(codigo, 0xb1)
	case ret == "J":
		codigo = append(codigo, 0xad)
	case ret == "F":
		codigo = append(codigo, 0xae)
	case ret == "D":
		codigo = append(codigo, 0xaf)
	case esObjeto(ret):
		codigo = append(codigo, 0xb0)
	default:
		codigo = append(codigo, 0xac)
	}
	sam := classMethod{
		Access: accPublic, Name: samNombre, Desc: samDesc,
		HasCode: true, MaxStack: len(params) + 1, LocalCount: hueco, ArgCount: hueco, Code: codigo,
	}
	interfaz := sitio.interfaz
	return classFull{
		Access:     accPublic | accFinal | accSynthetic,
		Name:       descriptor + ";",
		Super:      "Ljava/lang/Object;",
		Interfaces: []string{interfaz},
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
