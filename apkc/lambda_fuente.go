package main

// La lambda del fuente: `apk -> shareApp(apk)` tal como la escribe el usuario. El analizador la deja
// como `jlambda` y aquí se resuelve el tipo destino —el parametro formal de la llamada que la recibe,
// que es quien declara la interface funcional (JLS 15.27.3)—, se escribe el implementador
// `lambda$metodo$N` con `ACC_PRIVATE|ACC_SYNTHETIC` y se emite el `invokedynamic` con el bootstrap
// `LambdaMetafactory.metafactory` que `lambda.go` ya desugra a `new`/`invoke-direct`.
//
// Medido con javac 17 `--release 8 -g:lines,source` sobre `MainActivity.java:156` del zip de ApkShare:
//
//	private void lambda$openAppDetailsDialog$0(share/apps/util/Apk);
//	  Code: aload_0; aload_1; invokevirtual shareApp:(Lshare/apps/util/Apk;)V; return
//	  LineNumberTable: line 156: 0
//
// y el sitio con tres capturas de bootstrap: `(Ljava/lang/Object;)V`, el `MethodHandle`
// `REF_invokeSpecial` al implementador y `(Lshare/apps/util/Apk;)V` como tipo instanciado. El
// implementador es de instancia cuando la lambda usa `this`, que es lo que hace javac en vez de pasar
// el receptor como primer parametro.
//
// Fronteras de este subconjunto: un parametro sin tipo, un cuerpo de expresion, y la captura de
// `this`. Capturar una local del metodo que la escribe, la lambda dentro de otra lambda y el cuerpo de
// bloque se rechazan por nombre, cada uno con su motivo.

import (
	"fmt"
	"strings"
)

// tipoLambda es el descriptor con el que la resolucion de un `new` marca el hueco de una lambda: no es
// un tipo real (ninguna clase se llama así), solo dice «aquí va una interface funcional».
const tipoLambda = "<lambda>"

// esMetodoDeObject: `equals`, `hashCode` y `toString` declarados abstractos no cuentan para el metodo
// abstracto funcional (JLS 9.8): una interface que los repita sigue siendo funcional.
func esMetodoDeObject(nombre, desc string) bool {
	switch nombre {
	case "equals":
		return desc == "(Ljava/lang/Object;)Z"
	case "hashCode":
		return desc == "()I"
	case "toString":
		return desc == "()Ljava/lang/String;"
	}
	return false
}

// samSitio: la interface funcional de un parametro formal y su metodo abstracto único.
type samSitio struct {
	interfaz string // interno de la interface, `java/util/function/Consumer`
	nombre   string // el metodo abstracto, `accept`
	borrado  string // su descriptor borrado, `(Ljava/lang/Object;)V`
	inst     string // el descriptor con los argumentos de tipo del sitio, `(Lshare/apps/util/Apk;)V`
}

// buscarCtorConLambdas elige el constructor cuando algun argumento es una lambda. La diferencia con
// `buscarMetodo` es que el hueco de la lambda no se compara por descriptor —no lo tiene—: se comprueba
// que el parametro formal sea una interface funcional. Si queda mas de una candidata, la llamada es
// ambigua y se rechaza con sus descriptores en vez de elegir por orden.
func (ctx *contextoJava) buscarCtorConLambdas(interno string, tipos []string) (candidato, error) {
	cands, err := ctx.candidatosDe(interno, "<init>", len(tipos), false)
	if err != nil {
		return candidato{}, err
	}
	if len(cands) == 0 {
		return candidato{}, fmt.Errorf("el metodo %s.<init> con %d argumentos no esta en %s ni en sus padres",
			interno, len(tipos), interno)
	}
	aplicables := []candidato{}
	for _, c := range cands {
		_, _, ps := splitDescriptor(c.desc)
		if len(ps) != len(tipos) {
			continue
		}
		cabe := true
		for i := range ps {
			if tipos[i] == tipoLambda {
				if _, err := ctx.samDeParametro(c, i); err != nil {
					cabe = false
					break
				}
				continue
			}
			if !ctx.asignable(tipos[i], ps[i]) {
				cabe = false
				break
			}
		}
		if cabe {
			aplicables = append(aplicables, c)
		}
	}
	if len(aplicables) == 1 {
		return aplicables[0], nil
	}
	posibles := make([]string, 0, len(cands))
	for _, c := range cands {
		posibles = append(posibles, c.desc)
	}
	if len(aplicables) == 0 {
		return candidato{}, fmt.Errorf("%s.<init> tiene %d candidatas de %d argumentos (%s) y ninguna recibe una interface funcional donde va la lambda",
			interno, len(cands), len(tipos), strings.Join(posibles, " "))
	}
	return candidato{}, fmt.Errorf("%s.<init> tiene %d candidatas que reciben la lambda (%s): la llamada es ambigua",
		interno, len(aplicables), strings.Join(posibles, " "))
}

// samDeParametro: la interface funcional del parametro formal `pos` del candidato, con los argumentos
// de tipo que la declaracion del fuente le da. `Consumer<Apk>` aporta el `Apk` con el que el borrado
// `(Ljava/lang/Object;)V` pasa a `(Lshare/apps/util/Apk;)V`, que es el tipo instanciado del sitio.
func (ctx *contextoJava) samDeParametro(c candidato, pos int) (samSitio, error) {
	_, _, ps := splitDescriptor(c.desc)
	if pos < 0 || pos >= len(ps) {
		return samSitio{}, fmt.Errorf("el parametro %d no esta en %s", pos, c.desc)
	}
	if c.decl == nil {
		// una clase del classpath no trae la declaracion del fuente: solo se pueden rellenar los
		// argumentos de tipo si el parametro formal ya es el tipo instanciado (una interface no
		// generica, `Runnable`)
		return ctx.samDe(ps[pos], nil)
	}
	i := pos - (len(ps) - len(c.decl.params))
	if i < 0 || i >= len(c.decl.params) {
		return samSitio{}, fmt.Errorf("el parametro %d de %s no viene de la declaracion del fuente", pos, c.desc)
	}
	args := []string{}
	if g := c.decl.params[i].generico; g != nil {
		for _, a := range g.args {
			d, err := ctx.descriptorDe(a.crudo())
			if err != nil {
				return samSitio{}, fmt.Errorf("linea %d: el argumento de tipo de %s no se resuelve: %w",
					c.decl.params[i].donde.linea, c.decl.params[i].tipo, err)
			}
			args = append(args, d)
		}
	}
	return ctx.samDe(ps[pos], args)
}

// samDe: la interface funcional `borrado` y su metodo abstracto unico, ya instanciado con `args` (un
// descriptor por parametro de tipo de la interface, en su orden). La interface puede ser del proyecto
// o del classpath: en los dos casos se mira igual, y sin argumentos de tipo solo vale si el descriptor
// instanciado es el borrado.
func (ctx *contextoJava) samDe(borrado string, args []string) (samSitio, error) {
	if len(borrado) < 3 || borrado[0] != 'L' || borrado[len(borrado)-1] != ';' {
		return samSitio{}, fmt.Errorf("%s no es una referencia y por tanto no es una interface funcional", borrado)
	}
	interno := borrado[1 : len(borrado)-1]
	type abstracto struct{ nombre, desc, firma string }
	metodos := []abstracto{}
	firmaClase := ""
	if propia, ok := ctx.clasePropia(interno); ok {
		if !propia.esInterface {
			return samSitio{}, fmt.Errorf("%s no es una interface y no puede ser el tipo de una lambda", interno)
		}
		for _, m := range propia.metodos {
			if !m.abstracto || m.esEstatico {
				continue
			}
			d, err := ctx.descDeMetodoEn(propia, m)
			if err != nil {
				return samSitio{}, fmt.Errorf("linea %d: %w", m.linea, err)
			}
			if esMetodoDeObject(m.nombre, d) {
				continue
			}
			firma, err := ctx.firmaDeMetodo(m)
			if err != nil {
				firma = ""
			}
			metodos = append(metodos, abstracto{nombre: m.nombre, desc: d, firma: firma})
		}
		if f, err := ctx.firmaDeClase(propia); err == nil {
			firmaClase = f
		}
	} else {
		cf, err := ctx.busc.claseDelJar(interno)
		if err != nil {
			return samSitio{}, fmt.Errorf("%s no esta en el classpath: %w", interno, err)
		}
		if cf.Access&accInterface == 0 {
			return samSitio{}, fmt.Errorf("%s no es una interface y no puede ser el tipo de una lambda", interno)
		}
		for _, m := range cf.Methods {
			if m.Access&accAbstract == 0 || m.Access&accStatic != 0 {
				continue
			}
			if esMetodoDeObject(m.Name, m.Desc) {
				continue
			}
			metodos = append(metodos, abstracto{nombre: m.Name, desc: m.Desc, firma: m.Signature})
		}
		firmaClase = cf.Signature
	}
	if len(metodos) != 1 {
		nombres := make([]string, 0, len(metodos))
		for _, m := range metodos {
			nombres = append(nombres, m.nombre+m.desc)
		}
		return samSitio{}, fmt.Errorf("%s declara %d metodos abstractos (%s) y no es una interface funcional",
			interno, len(metodos), strings.Join(nombres, " "))
	}
	m := metodos[0]
	inst := m.desc
	parametros := parametrosDeFirma(firmaClase)
	if len(parametros) > 0 || len(args) > 0 {
		if len(parametros) != len(args) {
			return samSitio{}, fmt.Errorf("%s tiene %d parametros de tipo y el parametro formal del sitio da %d argumentos",
				interno, len(parametros), len(args))
		}
		if m.firma != "" {
			d, err := descriptorDeFirma(m.firma, parametros, args)
			if err != nil {
				return samSitio{}, fmt.Errorf("%s.%s: %w", interno, m.nombre, err)
			}
			inst = d
		}
	}
	return samSitio{interfaz: interno, nombre: m.nombre, borrado: m.desc, inst: inst}, nil
}

// parametrosDeFirma: los nombres de los parametros de tipo formales al principio de una Signature
// (`<T:Ljava/lang/Object;U::Ljava/io/Serializable;>…` da `T` y `U`). Sin `<…>` no hay parametros.
func parametrosDeFirma(sig string) []string {
	if len(sig) == 0 || sig[0] != '<' {
		return nil
	}
	out := []string{}
	i := 1
	for i < len(sig) && sig[i] != '>' {
		j := strings.IndexByte(sig[i:], ':')
		if j < 0 {
			return out
		}
		out = append(out, sig[i:i+j])
		i += j
		// las cotas: la de clase puede faltar (`T::Linterfaz;`) y detras puede venir otra
		for i < len(sig) && sig[i] == ':' {
			i++
			if i < len(sig) && sig[i] != ':' {
				i = finDeTipoDeFirma(sig, i)
			}
		}
	}
	return out
}

// finDeTipoDeFirma: el indice que sigue al tipo de una Signature que empieza en `i`.
func finDeTipoDeFirma(sig string, i int) int {
	for i < len(sig) {
		switch sig[i] {
		case 'L':
			prof := 0
			for i < len(sig) {
				switch sig[i] {
				case '<':
					prof++
				case '>':
					prof--
				case ';':
					if prof == 0 {
						return i + 1
					}
				}
				i++
			}
			return i
		case 'T':
			if j := strings.IndexByte(sig[i:], ';'); j >= 0 {
				return i + j + 1
			}
			return len(sig)
		case '[':
			i++
		default:
			return i + 1
		}
	}
	return i
}

// descriptorDeFirma: pasa una Signature de metodo (`(TT;)V`) al descriptor con los argumentos de tipo
// del sitio (`(Lshare/apps/util/Apk;)V`). Los tipos con parametros (`Ljava/util/List<TT;>;`) se borran
// a su crudo, que es lo que hace la JVM al describir el tipo instanciado. Lo que no se puede sustituir
// —una variable de tipo sin argumento— se rechaza con su nombre.
func descriptorDeFirma(firma string, parametros, args []string) (string, error) {
	// los parametros de tipo del propio metodo (`<T:…>(TT;)V`) no los usa este subconjunto
	i := 0
	if i < len(firma) && firma[i] == '<' {
		prof := 0
		for i < len(firma) {
			if firma[i] == '<' {
				prof++
			} else if firma[i] == '>' {
				prof--
				if prof == 0 {
					i++
					break
				}
			}
			i++
		}
	}
	if i >= len(firma) || firma[i] != '(' {
		return "", fmt.Errorf("la Signature %q no empieza por una lista de parametros", firma)
	}
	i++
	var sb strings.Builder
	sb.WriteByte('(')
	for i < len(firma) && firma[i] != ')' {
		switch firma[i] {
		case 'L':
			// el crudo: se copia hasta la `;` del nivel de fuera, tirando los argumentos
			prof := 0
			for i < len(firma) {
				c := firma[i]
				if c == '<' {
					prof++
				} else if c == '>' {
					prof--
				} else if c == ';' && prof == 0 {
					sb.WriteByte(';')
					i++
					break
				}
				if !(prof > 0 || c == '<' || c == '>') {
					sb.WriteByte(c)
				}
				i++
			}
		case 'T':
			j := strings.IndexByte(firma[i:], ';')
			if j < 0 {
				return "", fmt.Errorf("la Signature %q corta una variable de tipo", firma)
			}
			nombre := firma[i+1 : i+j]
			k := -1
			for n, p := range parametros {
				if p == nombre {
					k = n
					break
				}
			}
			if k < 0 || k >= len(args) {
				return "", fmt.Errorf("la variable de tipo %s no tiene argumento en el sitio (parametros %v, argumentos %v)",
					nombre, parametros, args)
			}
			sb.WriteString(args[k])
			i += j + 1
		case '*', '+', '-':
			// un comodin solo aparece dentro de `<…>`, que el caso `L` ya tira
			return "", fmt.Errorf("un comodin suelto en la Signature %q no esta medido", firma)
		default:
			sb.WriteByte(firma[i])
			i++
		}
	}
	if i >= len(firma) {
		return "", fmt.Errorf("la Signature %q no cierra los parametros", firma)
	}
	sb.WriteByte(')')
	i++
	resto := firma[i:]
	if resto == "V" || len(resto) == 1 {
		sb.WriteString(resto)
		return sb.String(), nil
	}
	d, err := descriptorDeFirmaTipo(resto, parametros, args)
	if err != nil {
		return "", err
	}
	sb.WriteString(d)
	return sb.String(), nil
}

// descriptorDeFirmaTipo: el descriptor de un tipo de vuelta escrito como Signature. Se reutiliza el
// camino de los parametros con una lista de uno y se quita el envoltorio.
func descriptorDeFirmaTipo(sig string, parametros, args []string) (string, error) {
	d, err := descriptorDeFirma("("+sig+")V", parametros, args)
	if err != nil {
		return "", err
	}
	return d[1 : len(d)-2], nil
}

// lambdaEnParametro: escribe la lambda en el hueco `pos` del descriptor del candidato. El tipo
// instanciado sale del parametro formal de la fuente (medido en el frente de ApkShare), no de la
// expresion.
func (e *emisor) lambdaEnParametro(l jlambda, c candidato, pos int) (string, error) {
	sam, err := e.ctx.samDeParametro(c, pos)
	if err != nil {
		return "", fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	return e.lamda(l, sam)
}

// lamda: el sitio `invokedynamic` de una lambda y su implementador. Devuelve el descriptor que queda en
// la pila, que es la interface funcional borrada (lo que ve el `invokespecial` del constructor).
func (e *emisor) lamda(l jlambda, sam samSitio) (string, error) {
	target := "L" + sam.interfaz + ";"
	_, retInst, paramsInst := splitDescriptor(sam.inst)
	if len(paramsInst) != 1 {
		return "", fmt.Errorf("linea %d: la lambda escribe un parametro y %s.%s toma %d",
			l.donde.linea, sam.interfaz, sam.nombre, len(paramsInst))
	}
	if nombre, ok := e.localCapturada(l); ok {
		return "", fmt.Errorf("linea %d: la lambda captura la local %s y solo se mide la captura de this", l.donde.linea, nombre)
	}
	// el receptor: javac deja `this` como primer valor capturado y el implementador como metodo de
	// instancia con REF_invokeSpecial (medido en `javap -v` de MainActivity.java:156)
	capturas := []string{}
	refKind := byte(6) // REF_invokeStatic
	if !e.estatico {
		capturas = append(capturas, "L"+e.interno+";")
		refKind = 7 // REF_invokeSpecial
	}
	implDesc := "(" + strings.Join(paramsInst, "") + ")" + retInst
	base := e.metodo
	if base == "<init>" {
		base = "new"
	} else if base == "<clinit>" {
		base = "static"
	}
	implNombre := "lambda$" + base + "$" + itoa(e.lambdaN)
	e.lambdaN++
	// El implementador no se escribe aquí: se encola y se emite al cerrar la clase, que es donde
	// javac lo genera. Así sus constantes —el Methodref del método que llama el cuerpo, los
	// MethodType y los MethodHandle del bootstrap— entran al final del pool y no desplazan las que
	// ya escribió el resto de la clase. Sin esto, un `ldc` de una constante posterior al sitio de
	// la lambda sube de 255 y sale `ldc_w` donde javac escribe `ldc` (`MainActivity` de ApkShare).
	e.lambdaPendientes = append(e.lambdaPendientes, implementadorPendiente{
		l: l, nombre: implNombre, desc: implDesc, conReceptor: len(capturas) > 0,
		param: paramsInst[0], vuelta: retInst, metodo: e.metodo,
	})
	if implementadorEnElSitio {
		p := e.lambdaPendientes[len(e.lambdaPendientes)-1]
		e.lambdaPendientes = e.lambdaPendientes[:len(e.lambdaPendientes)-1]
		if err := e.escribeImplementador(p); err != nil {
			return "", err
		}
	}

	// el sitio: descriptor de fabrica con las capturas y la vuelta de la interface borrada. Los
	// operandos del bootstrap (MethodHandle y MethodType) también se aplazan: javac los escribe en
	// el atributo BootstrapMethods, con el pool ya cerrado, y por eso salen detrás de todo.
	indyDesc := "(" + strings.Join(capturas, "") + ")" + target
	interno := e.interno
	clave := implNombre + "|" + implDesc + "|" + sam.borrado + "|" + sam.inst + "|" + itoa(int(refKind))
	calcular := func(p *poolBuilder) (int, []int) {
		mtSam := p.metodoTipo(sam.borrado)
		mtInst := p.metodoTipo(sam.inst)
		mhImpl := p.metodoManejador(refKind, p.metodo(interno, implNombre, implDesc))
		mhBoot := p.metodoManejador(6, p.metodo("java/lang/invoke/LambdaMetafactory", "metafactory", descriptorMetafactory))
		return mhBoot, []int{mtSam, mhImpl, mtInst}
	}
	var boot int
	if implementadorEnElSitio {
		handle, args := calcular(e.pool)
		boot = e.registraBootstrap(handle, args)
		e.usar("java/lang/invoke/MethodHandles$Lookup")
	} else {
		boot = e.registraBootstrapDiferido(clave, calcular)
	}
	idx := e.pool.invocaDinamico(boot, sam.nombre, indyDesc)
	// los valores capturados se bajan al pico antes del sitio: javac escribe `aload_0` y detrás el
	// `invokedynamic`, que los consume como argumentos de su descriptor de fabrica. Hoy el único valor
	// capturado es `this` (una local se rechaza arriba), así que el receptor es siempre el del hueco 0.
	for range capturas {
		e.cargaDe(0, "L")
	}
	e.emite(0xba, byte(idx>>8), byte(idx), 0, 0)
	e.baja(len(capturas))
	e.sube()
	if e.truncado {
		return "", fmt.Errorf("linea %d: el pool de %s pasa de 255 entradas y el sitio de la lambda no cabe en un byte", l.donde.linea, e.interno)
	}
	return target, nil
}

// descriptorMetafactory: el descriptor con el que javac llama al bootstrap de una lambda.
const descriptorMetafactory = "(Ljava/lang/invoke/MethodHandles$Lookup;Ljava/lang/String;" +
	"Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodHandle;" +
	"Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/CallSite;"

// implementadorEnElSitio es el interruptor de la falsificación del aplazamiento. Apagado (el binario
// que se publica), el implementador de una lambda y las constantes de su bootstrap se emiten al cerrar
// la clase, que es donde javac los escribe. Encendido, se emiten en el sitio y el pool crece antes de
// tiempo: es el defecto que hacía salir `ldc_w` donde javac escribe `ldc` —el `MainActivity` de
// ApkShare—, porque una constante posterior al sitio se desplaza por encima de 255. Lo usa
// `javaemisor_ldc_pool_largo_test.go`.
var implementadorEnElSitio = false

// implementadorPendiente es una lambda cuyo `method_info` falta: se emite al cerrar la clase, con el
// pool ya cargado con las constantes de todos los métodos. `metodo` es el nombre del método que
// contiene el sitio, que es lo que nombra al implementador (`lambda$<metodo>$N`).
type implementadorPendiente struct {
	l            jlambda
	nombre, desc string
	conReceptor  bool
	param        string
	vuelta       string
	metodo       string
}

// escribeImplementador: el metodo `lambda$metodo$N` con el cuerpo de la lambda. Se emite en un emisor
// aparte que comparte el pool y las clases vistas, y se llama al cerrar la clase. La LineNumberTable
// lleva la linea del cuerpo: la de la expresion, que en `MainActivity.java:156` es la de la
// declaracion entera.
func (e *emisor) escribeImplementador(p implementadorPendiente) error {
	l, nombre, desc, conReceptor, param, vuelta := p.l, p.nombre, p.desc, p.conReceptor, p.param, p.vuelta
	sub := &emisor{ctx: e.ctx, pool: e.pool, c: e.c, interno: e.interno, vistas: e.vistas}
	sub.preparaMetodo(desc, !conReceptor)
	sub.metodo = p.metodo
	if conReceptor {
		// el `this` capturado es el hueco 0 del implementador de instancia, que no viaja en el
		// descriptor: lo que cuenta es que el metodo sea de instancia
		sub.reserva("", "L"+e.interno+";")
	}
	sub.reserva(l.param, param)
	sub.marcaLinea(l.donde.linea)
	d, err := sub.expA(l.cuerpo, vuelta)
	if err != nil {
		return fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	if d != vuelta {
		return fmt.Errorf("linea %d: el cuerpo de la lambda deja %s y su metodo funcional devuelve %s",
			l.donde.linea, d, vuelta)
	}
	if sub.prof != 0 {
		return fmt.Errorf("linea %d: quedan %d valores en la pila al cerrar el implementador de la lambda", l.donde.linea, sub.prof)
	}
	sub.emite(retorno(vuelta[0]))
	codigo, err := sub.codeAttr()
	if err != nil {
		return fmt.Errorf("linea %d: %w", l.donde.linea, err)
	}
	metodo := sub.methodInfo(nombre, desc, accPrivate|accSynthetic, attrInfo(e.pool, "Code", codigo), 1)
	e.lambdaMetodos = append(e.lambdaMetodos, metodo)
	e.menciona = append(e.menciona, sub.menciona...)
	return nil
}

// registraBootstrap: añade (o reutiliza) una fila de BootstrapMethods y devuelve su indice.
func (e *emisor) registraBootstrap(handle int, args []int) int {
	if e.bootIdx == nil {
		e.bootIdx = map[string]int{}
	}
	clave := itoa(handle)
	for _, a := range args {
		clave += "|" + itoa(a)
	}
	if i, ok := e.bootIdx[clave]; ok {
		return i
	}
	i := len(e.bootstraps)
	e.bootstraps = append(e.bootstraps, classBootstrap{handle: handle, args: args})
	e.bootIdx[clave] = i
	return i
}

// registraBootstrapDiferido: la misma fila, pero con los índices del pool sin calcular. Se reserva la
// posición ahora —el `invokedynamic` la necesita en su operando— y `calcular` la rellena cuando el
// código de la clase está cerrado. La clave es simbólica (nombre, descriptores y ref_kind), porque
// los índices todavía no existen para deduplicar por ellos.
func (e *emisor) registraBootstrapDiferido(clave string, calcular func(*poolBuilder) (int, []int)) int {
	if e.bootIdx == nil {
		e.bootIdx = map[string]int{}
	}
	if i, ok := e.bootIdx[clave]; ok {
		return i
	}
	i := len(e.bootstraps)
	e.bootstraps = append(e.bootstraps, classBootstrap{pendiente: calcular})
	e.bootIdx[clave] = i
	return i
}

// tablaDeBootstraps: el cuerpo del atributo BootstrapMethods (JVMS 4.7.23): el conteo y, por fila, el
// MethodHandle y la lista de argumentos, todos indices del pool.
func tablaDeBootstraps(tabla []classBootstrap) []byte {
	out := u2be(uint16(len(tabla)))
	for _, b := range tabla {
		out = append(out, u2be(uint16(b.handle))...)
		out = append(out, u2be(uint16(len(b.args)))...)
		for _, a := range b.args {
			out = append(out, u2be(uint16(a))...)
		}
	}
	return out
}

// localCapturada: si el cuerpo de la lambda nombra una local o un parametro del metodo que la escribe.
// Solo se mide la captura de `this`; capturar una local pediría un valor capturado por nombre en el
// constructor de la clase sintetica y no está medido, así que se rechaza con la local en el mensaje.
func (e *emisor) localCapturada(l jlambda) (string, bool) {
	var cual string
	encontrada := false
	var mirar func(jexpr)
	mirar = func(x jexpr) {
		if encontrada {
			return
		}
		switch v := x.(type) {
		case jacceso:
			if v.base != nil {
				mirar(v.base)
				return
			}
			if v.nombre == l.param {
				return
			}
			if _, ok := e.locales[v.nombre]; ok {
				cual, encontrada = v.nombre, true
			}
		case jllamada:
			if v.base != nil {
				mirar(v.base)
			}
			for _, a := range v.args {
				mirar(a)
			}
		case jnuevo:
			for _, a := range v.args {
				mirar(a)
			}
		case jnuevoArreglo:
			if v.largo != nil {
				mirar(v.largo)
			}
			for _, a := range v.elementos {
				mirar(a)
			}
		case jindice:
			mirar(v.base)
			mirar(v.indice)
		case joperador:
			mirar(v.izq)
			mirar(v.der)
		case jcast:
			mirar(v.dentro)
		case jinstanceof:
			mirar(v.dentro)
		case jnegacion:
			mirar(v.dentro)
		case jnegativo:
			mirar(v.dentro)
		case jincr:
			mirar(v.destino)
		case jasignExp:
			// `x = y` como valor: los dos lados se miran igual que en una sentencia
			mirar(v.destino)
			mirar(v.valor)
		case jlambda:
			// una lambda dentro de otra: su parametro tapa al de fuera y el cuerpo se mira igual
			for _, a := range []jexpr{v.cuerpo} {
				mirar(a)
			}
		}
	}
	mirar(l.cuerpo)
	return cual, encontrada
}
