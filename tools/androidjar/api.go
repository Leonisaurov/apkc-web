package main

// Lector del formato de firma 2.0 de AOSP (core/api/current.txt). No interpreta
// toda la API: solo lo que hace falta para escribir un stub que `apkc compile`
// pueda leer. Las lineas que no se entienden se cuentan y se avisan; no se
// inventa una firma.

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type api struct {
	clases map[string]*apiClase
	orden  []string
}

type apiClase struct {
	nombre      string
	interno     string
	access      uint16
	super       string
	interfaces  []string
	campos      []*apiCampo
	metodos     []*apiMetodo
	esInterfaz  bool
	esEnum      bool
	esAnotacion bool
}

type apiCampo struct {
	nombre   string
	desc     string
	access   uint16
	tieneVal bool
	valor    apiValor
}

type apiMetodo struct {
	nombre string
	desc   string
	access uint16
}

type apiValor struct {
	tag   byte // 3 int, 4 float, 5 long, 6 double, 8 cadena
	i     int32
	j     int64
	f     float64
	texto string
}

var (
	reAnotacion  = regexp.MustCompile(`@[A-Za-z0-9_.$]+(?:\([^)]*\))?`)
	reCabecera   = regexp.MustCompile(`^(?:(?:public|protected|private|static|final|abstract|sealed|non-sealed|strictfp)\s+)*(class|interface|enum|__ANOTACION__)\s+([A-Za-z0-9_.$]+)(.*)\{$`)
	reExtends    = regexp.MustCompile(`\bextends\s+(.+?)(?:\s+implements\s+(.+))?$`)
	reImplements = regexp.MustCompile(`\bimplements\s+(.+)$`)
	reThrows     = regexp.MustCompile(`\bthrows\s+(.+)$`)
)

func leeAPI(r io.Reader) (*api, error) {
	a := &api{clases: map[string]*apiClase{}}
	if err := leeAPIEn(r, a); err != nil {
		return nil, err
	}
	return a, nil
}

func leeAPIEn(r io.Reader, a *api) error {
	type marco struct {
		nombre string
		indent int
		clase  *apiClase
	}
	var pila []marco
	paquete := ""
	indent := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	linea := 0
	var pendiente string
	var inicio int
	for sc.Scan() {
		linea++
		cruda := sc.Text()
		sin := strings.TrimSpace(cruda)
		if sin == "" || strings.HasPrefix(sin, "//") {
			continue
		}
		sin = cortaComentario(sin)
		if pendiente != "" {
			pendiente += " " + sin
		} else {
			pendiente = sin
			inicio = len(cruda) - len(strings.TrimLeft(cruda, " "))
		}
		// una declaracion puede terminar en una linea posterior; se acumula
		if !strings.HasSuffix(strings.TrimSpace(pendiente), "{") &&
			!strings.HasSuffix(strings.TrimSpace(pendiente), "}") &&
			!strings.HasSuffix(strings.TrimSpace(pendiente), ";") {
			continue
		}
		texto := strings.TrimSpace(pendiente)
		indent = inicio
		pendiente = ""
		if texto == "}" {
			if len(pila) > 0 && pila[len(pila)-1].indent == indent {
				pila = pila[:len(pila)-1]
			}
			if indent == 0 {
				paquete = ""
			}
			continue
		}
		if strings.HasPrefix(texto, "package ") && strings.HasSuffix(texto, "{") {
			paquete = strings.TrimSuffix(strings.TrimPrefix(texto, "package "), "{")
			paquete = strings.TrimSpace(paquete)
			pila = nil
			continue
		}
		if c := interpretaCabecera(texto, paquete, indent); c != nil {
			if _, ya := a.clases[c.nombre]; ya {
				return fmt.Errorf("linea %d: clase repetida %s", linea, c.nombre)
			}
			c.interno = internoDe(c.nombre, a.clases)
			a.clases[c.nombre] = c
			a.orden = append(a.orden, c.nombre)
			pila = append(pila, marco{nombre: c.nombre, indent: indent, clase: c})
			continue
		}
		if len(pila) == 0 {
			continue
		}
		c := pila[len(pila)-1]
		if indent != c.indent+2 {
			continue
		}
		if err := interpretaMiembro(c.clase, texto, a.clases); err != nil {
			return fmt.Errorf("linea %d, %s: %w", linea, c.nombre, err)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	// El nombre interno necesita el mapa completo, que no existe hasta haber leido
	// todas las cabeceras. Se corrige en una segunda pasada.
	for _, c := range a.clases {
		c.interno = internoDe(c.nombre, a.clases)
		c.super = internoDe(c.super, a.clases)
		for i, in := range c.interfaces {
			c.interfaces[i] = internoDe(in, a.clases)
		}
	}
	return nil
}

func limpiarAnotaciones(s string) string {
	s = strings.ReplaceAll(s, "@interface", "__ANOTACION__")
	s = reAnotacion.ReplaceAllString(s, " ")
	return strings.ReplaceAll(s, "__ANOTACION__", "@interface")
}

func interpretaCabecera(texto, paquete string, indent int) *apiClase {
	t := strings.TrimSpace(limpiarAnotaciones(texto))
	t = strings.ReplaceAll(t, "@interface", "__ANOTACION__")
	m := reCabecera.FindStringSubmatch(t)
	if m == nil {
		return nil
	}
	nombre := m[2]
	if !strings.Contains(nombre, ".") && paquete != "" {
		nombre = paquete + "." + nombre
	} else if paquete != "" && !strings.HasPrefix(nombre, paquete+".") {
		nombre = paquete + "." + nombre
	}
	c := &apiClase{nombre: nombre}
	prefijo := strings.TrimSpace(t[:strings.Index(t, m[1])])
	for _, mod := range strings.Fields(prefijo) {
		if mod == "abstract" {
			c.access |= 0x0400
		}
		if mod == "final" {
			c.access |= 0x0010
		}
	}
	switch m[1] {
	case "interface":
		c.esInterfaz = true
		c.access = 0x0601 // public interface abstract
	case "__ANOTACION__":
		c.esInterfaz = true
		c.esAnotacion = true
		c.access = 0x2601 // public interface abstract annotation
	case "enum":
		c.esEnum = true
		c.access = 0x4021 // public final super enum
	default:
		c.access = 0x0021 // public super
	}
	resto := strings.TrimSpace(m[3])
	resto = sinGenericos(resto)
	if strings.Contains(resto, "extends") {
		em := reExtends.FindStringSubmatch(resto)
		if em != nil {
			ext := strings.TrimSpace(em[1])
			partes := camposDeLista(ext)
			if c.esInterfaz {
				for _, p := range partes {
					c.interfaces = append(c.interfaces, p)
				}
			} else if len(partes) > 0 {
				c.super = partes[0]
			}
			if em[2] != "" {
				for _, p := range camposDeLista(em[2]) {
					c.interfaces = append(c.interfaces, p)
				}
			}
		}
	} else if im := reImplements.FindStringSubmatch(resto); im != nil {
		for _, p := range camposDeLista(im[1]) {
			c.interfaces = append(c.interfaces, p)
		}
	}
	return c
}

func interpretaMiembro(c *apiClase, texto string, clases map[string]*apiClase) error {
	t := limpiarAnotaciones(texto)
	t = strings.TrimSpace(t)
	switch {
	case strings.HasPrefix(t, "field "):
		return interpretaCampo(c, strings.TrimPrefix(t, "field "), clases)
	case strings.HasPrefix(t, "method "):
		return interpretaMetodo(c, strings.TrimPrefix(t, "method "), clases)
	case strings.HasPrefix(t, "ctor "):
		return interpretaConstructor(c, strings.TrimPrefix(t, "ctor "), clases)
	}
	return nil
}

func interpretaCampo(c *apiClase, cuerpo string, clases map[string]*apiClase) error {
	cuerpo = cortaComentario(cuerpo)
	cuerpo = strings.TrimSuffix(strings.TrimSpace(cuerpo), ";")
	izq, der, tiene := parteIgual(cuerpo)
	mods, resto := separaMods(izq)
	nombre, tipo := ultimaPalabra(resto)
	if nombre == "" || tipo == "" {
		return fmt.Errorf("campo sin nombre o sin tipo: %q", cuerpo)
	}
	desc, err := descriptorDe(tipo, clases)
	if err != nil {
		return err
	}
	f := &apiCampo{nombre: nombre, desc: desc, access: mods}
	if tiene {
		if v, ok := valorDe(der); ok {
			f.tieneVal = true
			f.valor = v
		}
	}
	c.campos = append(c.campos, f)
	return nil
}

func interpretaMetodo(c *apiClase, cuerpo string, clases map[string]*apiClase) error {
	cuerpo = cortaComentario(cuerpo)
	cuerpo = strings.TrimSuffix(strings.TrimSpace(cuerpo), ";")
	mods, resto := separaMods(cuerpo)
	vars := map[string]string{}
	resto, vars = quitaGenericosDeMetodoConVars(resto)
	abre := strings.Index(resto, "(")
	cierra := cierreParentesis(resto, abre)
	if abre < 0 || cierra < 0 {
		return fmt.Errorf("metodo sin parentesis: %q", cuerpo)
	}
	cab := strings.TrimSpace(resto[:abre])
	nombre, devuelve := ultimaPalabra(cab)
	if nombre == "" || devuelve == "" {
		return fmt.Errorf("metodo sin nombre o sin retorno: %q", cuerpo)
	}
	desc, err := descriptorDeCon(devuelve, clases, vars)
	if err != nil {
		return err
	}
	desc = "(" + descriptorDeListaCon(resto[abre+1:cierra], clases, vars) + ")" + desc
	m := &apiMetodo{nombre: nombre, desc: desc, access: mods}
	c.metodos = append(c.metodos, m)
	return nil
}

func interpretaConstructor(c *apiClase, cuerpo string, clases map[string]*apiClase) error {
	cuerpo = cortaComentario(cuerpo)
	cuerpo = strings.TrimSuffix(strings.TrimSpace(cuerpo), ";")
	mods, resto := separaMods(cuerpo)
	abre := strings.Index(resto, "(")
	cierra := cierreParentesis(resto, abre)
	if abre < 0 || cierra < 0 {
		return fmt.Errorf("constructor sin parentesis: %q", cuerpo)
	}
	desc := "(" + descriptorDeLista(resto[abre+1:cierra], clases) + ")V"
	c.metodos = append(c.metodos, &apiMetodo{nombre: "<init>", desc: desc, access: mods})
	return nil
}

func separaMods(s string) (uint16, string) {
	var access uint16
	partes := strings.Fields(s)
	i := 0
	for ; i < len(partes); i++ {
		switch partes[i] {
		case "public":
			access |= 0x0001
		case "protected":
			access |= 0x0004
		case "private":
			access |= 0x0002
		case "static":
			access |= 0x0008
		case "final":
			access |= 0x0010
		case "abstract":
			access |= 0x0400
		case "native":
			access |= 0x0100
		case "synchronized":
			access |= 0x0020
		case "transient":
			access |= 0x0080
		case "volatile":
			access |= 0x0040
		case "strictfp":
			access |= 0x0800
		case "default":
			// un metodo default de interface no se emite como abstract; el
			// stub no ejecuta, pero `apkc` solo lee el descriptor.
			access &^= 0x0400
		default:
			return access, strings.Join(partes[i:], " ")
		}
	}
	return access, ""
}

func descriptorDeLista(lista string, clases map[string]*apiClase) string {
	return descriptorDeListaCon(lista, clases, nil)
}

func descriptorDeListaCon(lista string, clases map[string]*apiClase, vars map[string]string) string {
	var b strings.Builder
	for _, p := range parteLista(lista) {
		p = limpiarAnotaciones(p)
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "...") {
			p = strings.TrimSuffix(p, "...") + "[]"
		}
		d, err := descriptorDeCon(p, clases, vars)
		if err == nil {
			b.WriteString(d)
		}
	}
	return b.String()
}

func descriptorDe(tipo string, clases map[string]*apiClase) (string, error) {
	return descriptorDeCon(tipo, clases, nil)
}

func descriptorDeCon(tipo string, clases map[string]*apiClase, vars map[string]string) (string, error) {
	tipo = limpiarAnotaciones(tipo)
	tipo = strings.TrimSpace(tipo)
	tipo = sinGenericos(tipo)
	tipo = strings.ReplaceAll(tipo, " ", "")
	if tipo == "" {
		return "", fmt.Errorf("tipo vacio")
	}
	dim := 0
	for strings.HasSuffix(tipo, "[]") {
		tipo = strings.TrimSuffix(tipo, "[]")
		dim++
	}
	var base string
	switch tipo {
	case "void":
		base = "V"
	case "boolean":
		base = "Z"
	case "byte":
		base = "B"
	case "char":
		base = "C"
	case "short":
		base = "S"
	case "int":
		base = "I"
	case "long":
		base = "J"
	case "float":
		base = "F"
	case "double":
		base = "D"
	default:
		if vars != nil {
			if bound, ok := vars[tipo]; ok {
				return descriptorDeCon(bound, clases, nil)
			}
		}
		base = "L" + internoDe(tipo, clases) + ";"
	}
	return strings.Repeat("[", dim) + base, nil
}

func internoDe(nombre string, clases map[string]*apiClase) string {
	if nombre == "" {
		return ""
	}
	if _, ok := clases[nombre]; !ok && !strings.Contains(nombre, ".") {
		if _, ok := clases["java.lang."+nombre]; ok {
			nombre = "java.lang." + nombre
		}
	}
	// una clase anidada es `Outer.Inner`; se busca el prefijo mas largo que sea
	// clase para poner `$` en la frontera.
	partes := strings.Split(nombre, ".")
	for i := len(partes) - 1; i > 0; i-- {
		p := strings.Join(partes[:i], ".")
		if _, ok := clases[p]; ok {
			resto := strings.Join(partes[i:], "$")
			return strings.ReplaceAll(p, ".", "/") + "$" + resto
		}
	}
	return strings.ReplaceAll(nombre, ".", "/")
}

func sinGenericos(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		switch r {
		case '<':
			n++
		case '>':
			if n > 0 {
				n--
			}
		default:
			if n == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func quitaGenericosDeMetodo(s string) string {
	s, _ = quitaGenericosDeMetodoConVars(s)
	return s
}

func quitaGenericosDeMetodoConVars(s string) (string, map[string]string) {
	s = strings.TrimSpace(s)
	vars := map[string]string{}
	if !strings.HasPrefix(s, "<") {
		return s, vars
	}
	fin := cierraAngulo(s, 0)
	if fin < 0 {
		return s, vars
	}
	dentro := s[1:fin]
	for _, p := range parteLista(dentro) {
		campos := strings.Fields(strings.TrimSpace(p))
		if len(campos) == 0 {
			continue
		}
		nombre := campos[0]
		cota := "java.lang.Object"
		if len(campos) >= 3 && campos[1] == "extends" {
			cota = strings.Join(campos[2:], " ")
		}
		vars[nombre] = cota
	}
	return strings.TrimSpace(s[fin+1:]), vars
}

func cierraAngulo(s string, abre int) int {
	n := 0
	for i := abre; i < len(s); i++ {
		switch s[i] {
		case '<':
			n++
		case '>':
			n--
			if n == 0 {
				return i
			}
		}
	}
	return -1
}

func cierreParentesis(s string, abre int) int {
	if abre < 0 {
		return -1
	}
	n := 0
	for i := abre; i < len(s); i++ {
		switch s[i] {
		case '(':
			n++
		case ')':
			n--
			if n == 0 {
				return i
			}
		}
	}
	return -1
}

func ultimaPalabra(s string) (string, string) {
	s = strings.TrimSpace(s)
	i := strings.LastIndex(s, " ")
	if i < 0 {
		return s, ""
	}
	return strings.TrimSpace(s[i+1:]), strings.TrimSpace(s[:i])
}

func parteLista(s string) []string {
	var out []string
	n := 0
	desde := 0
	for i, r := range s {
		switch r {
		case '<', '(':
			n++
		case '>', ')':
			if n > 0 {
				n--
			}
		case ',':
			if n == 0 {
				out = append(out, s[desde:i])
				desde = i + 1
			}
		}
	}
	out = append(out, s[desde:])
	return out
}

func camposDeLista(s string) []string {
	var out []string
	for _, p := range strings.Fields(strings.ReplaceAll(s, ",", " ")) {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func cortaComentario(s string) string {
	if i := strings.Index(s, "//"); i >= 0 {
		return s[:i]
	}
	return s
}

func parteIgual(s string) (string, string, bool) {
	n := 0
	for i, r := range s {
		switch r {
		case '<':
			n++
		case '>':
			if n > 0 {
				n--
			}
		case '"':
			// no se esperan comillas a la izquierda del `=`
		}
		if r == '=' && n == 0 {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

func valorDe(s string) (apiValor, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ";"))
	if s == "" || s == "null" {
		return apiValor{}, false
	}
	if strings.HasPrefix(s, "\"") {
		if v, err := strconv.Unquote(s); err == nil {
			return apiValor{tag: 8, texto: v}, true
		}
		return apiValor{}, false
	}
	if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") && len(s) >= 3 {
		return apiValor{tag: 3, i: int32(s[1])}, true
	}
	switch s {
	case "true":
		return apiValor{tag: 3, i: 1}, true
	case "false":
		return apiValor{tag: 3, i: 0}, true
	}
	if strings.HasSuffix(s, "f") || strings.HasSuffix(s, "F") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(s, "f"), "F"), 64); err == nil {
			return apiValor{tag: 4, f: v}, true
		}
	}
	if strings.HasSuffix(s, "d") || strings.HasSuffix(s, "D") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(s, "d"), "D"), 64); err == nil {
			return apiValor{tag: 6, f: v}, true
		}
	}
	if strings.HasSuffix(s, "l") || strings.HasSuffix(s, "L") {
		if v, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimSuffix(s, "l"), "L"), 0, 64); err == nil {
			return apiValor{tag: 5, j: v}, true
		}
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "-0x") {
		if v, err := strconv.ParseInt(s, 0, 32); err == nil {
			return apiValor{tag: 3, i: int32(v)}, true
		}
	}
	if v, err := strconv.ParseInt(s, 10, 32); err == nil {
		return apiValor{tag: 3, i: int32(v)}, true
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return apiValor{tag: 6, f: v}, true
	}
	return apiValor{}, false
}
