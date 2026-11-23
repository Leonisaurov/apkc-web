package main

// Las pruebas del hot swap del preview: el código nuevo entra en la tabla de despacho de una sesión
// VIVA, sin reiniciar la máquina ni reconstruir los objetos.
//
// Cada propiedad del contrato tiene aquí su falsificación:
//   - sin el enganche, el método viejo sigue respondiendo (TestSinElEnganche...);
//   - con él, responde el nuevo y el cuerpo nuevo ve el estado anterior (TestElHotSwap...);
//   - dos referencias al mismo objeto siguen siéndolo (el `Par` de la prueba tiene el mismo `Saludo`
//     en sus dos campos y su `suma()` lo llama dos veces);
//   - el reemplazo es aditivo: un método que no existía se añade y los demás no se tocan, y los
//     estáticos conservan el valor que la sesión ya tenía (TestElHotSwapEsAditivo...).
//
// El dex de las pruebas se genera en caliente con el emisor del fork, como el resto de la Fase 2.

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"hash/adler32"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const fuenteSaludoV1 = `package com.hola;
public class Saludo {
    static int base = 3;
    int n;
    public void sube() { n = n + 1; }
    public int valor() { return n + 100; }
    public int conBase() { return n + base; }
}
`

const fuenteSaludoV2 = `package com.hola;
public class Saludo {
    static int base = 3;
    int n;
    public void sube() { n = n + 1; }
    public int valor() { return n + 200; }
    public int conBase() { return n + base; }
    public int extra() { return 7; }
}
`

const fuentePar = `package com.hola;
public class Par {
    Saludo a;
    Saludo b;
    Par(Saludo a, Saludo b) { this.a = a; this.b = b; }
    int tocaUno() { a.sube(); return b.valor(); }
    int suma() { return a.valor() + b.valor(); }
}
`

// Las fuentes de la pregunta abierta: el cuerpo que se intercambia usa una constante de cadena, y el
// dex de la sesión trae otra clase con sus propias cadenas, así que el índice del `const-string` del
// cuerpo nuevo no significa lo mismo en los dos ficheros.
const fuenteEtiquetaV1 = `package com.hola;
public class Etiqueta {
    public String texto() { return "viejo"; }
}
`

const fuenteRelleno = `package com.hola;
public class Relleno {
    public String uno() { return "alfa"; }
    public String dos() { return "beta"; }
    public String tres() { return "gamma"; }
    public String cuatro() { return "delta"; }
}
`

const fuenteEtiquetaV2 = `package com.hola;
public class Etiqueta {
    public String texto() { return "nuevo"; }
}
`

// sesionDelHotSwap arma una máquina con el dex de la versión 1 y un `Saludo` que ya subió dos veces,
// metido en los dos campos del mismo `Par`: es el estado que el swap no puede perder.
func sesionDelHotSwap(t *testing.T) (*maquinaDalvik, *objetoDalvik, *objetoDalvik, *dexFile) {
	t.Helper()
	d1, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV1},
		fuenteJava{nombre: "Par.java", contenido: fuentePar},
	))
	if err != nil {
		t.Fatalf("el dex de la versión 1 no se lee: %v", err)
	}
	prog, err := cargaPrograma(d1)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	m := nuevaMaquina(prog, nil)

	saludo := &objetoDalvik{clase: "Lcom/hola/Saludo;", campos: map[string]any{}}
	par := &objetoDalvik{clase: "Lcom/hola/Par;", campos: map[string]any{
		"Lcom/hola/Par;.a": saludo,
		"Lcom/hola/Par;.b": saludo,
	}}
	for i := 0; i < 2; i++ {
		llamaProyecto(t, m, "Lcom/hola/Saludo;", "sube", "()V", saludo)
	}
	return m, saludo, par, d1
}

// llamaProyecto ejecuta un método del proyecto sobre un receptor y devuelve lo que rinde.
func llamaProyecto(t *testing.T, m *maquinaDalvik, clase, nombre, proto string, receptor *objetoDalvik) any {
	t.Helper()
	met := m.buscaMetodo(clase, nombre, proto)
	if met == nil {
		t.Fatalf("no se encontró %s.%s%s en la sesión", claseCorta(clase), nombre, proto)
	}
	v, err := m.ejecutaCuerpo(clase, met, []any{receptor}, []string{clase})
	if err != nil {
		t.Fatalf("%s.%s%s falló: %v", claseCorta(clase), nombre, proto, err)
	}
	return v
}

func enteroDe(t *testing.T, v any, quien string) int32 {
	t.Helper()
	n, ok := v.(int32)
	if !ok {
		t.Fatalf("%s devolvió %T (%v) y se esperaba un int", quien, v, v)
	}
	return n
}

// TestElHotSwapConservaIdentidadYEstadoYCambiaElDespacho es el criterio de aceptación: el mismo
// objeto, con el mismo `n`, responde con el cuerpo nuevo tras el swap. La identidad se comprueba por
// los dos caminos: el de la app (mutar por el campo `a` y leerlo por el `b` del `Par`) y el de Go
// (los dos campos son el mismo puntero), y el estado, por el `n` que el cuerpo nuevo ve.
func TestElHotSwapConservaIdentidadYEstadoYCambiaElDespacho(t *testing.T) {
	m, saludo, par, _ := sesionDelHotSwap(t)

	// antes del swap: el cuerpo viejo, sobre n = 2
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 204 {
		t.Fatalf("antes del swap suma() = %d y se esperaba 204 (2·(2+100))", got)
	}

	d2, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2}))
	if err != nil {
		t.Fatalf("el dex de la versión 2 no se lee: %v", err)
	}
	res, err := m.intercambiaMetodo(d2, "Lcom/hola/Saludo;", "valor", "()I")
	if err != nil {
		t.Fatalf("el swap de valor()I falló: %v", err)
	}
	if !res.Existia || res.Antes == nil || res.Despues == nil {
		t.Fatalf("el swap debía sustituir un método existente y dijo %+v", res)
	}
	if res.Antes == res.Despues {
		t.Fatal("el swap dejó la misma implementación: no se enganchó el cuerpo nuevo")
	}

	// después del swap: el cuerpo nuevo ve el n de antes (2) y suma 200
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 404 {
		t.Fatalf("tras el swap suma() = %d y se esperaba 404 (2·(2+200)): el cuerpo nuevo no ve "+
			"el estado o no se despachó", got)
	}

	// la identidad no se tocó: los dos campos del Par siguen siendo el mismo objeto, y es el de antes
	if par.campos["Lcom/hola/Par;.a"] != par.campos["Lcom/hola/Par;.b"] {
		t.Fatal("tras el swap los dos campos del Par ya no son el mismo objeto")
	}
	if par.campos["Lcom/hola/Par;.a"] != saludo {
		t.Fatal("tras el swap el objeto del Par no es el que se creó antes")
	}

	// y por el camino de la app: sube() por el campo `a` y valor() por el `b`, en la misma llamada.
	// Si el swap hubiera recreado el objeto, el `b` no vería el incremento del `a`.
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "tocaUno", "()I", par), "tocaUno"); got != 203 {
		t.Fatalf("tocaUno() = %d y se esperaba 203 (n=3 visto por el otro campo, +200)", got)
	}
	// el método que el swap no nombró sigue con su cuerpo, y el estado acumulado es el de siempre
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 406 {
		t.Fatalf("tras tocaUno() suma() = %d y se esperaba 406: el estado del objeto se perdió o "+
			"un método no nombrado cambió", got)
	}
}

// mismoEvento compara dos eventos del guion campo a campo. El struct lleva el dex del swap como
// []byte y por eso no vale `==`; comparar solo Tipo y Dato dejaría sin verificar el dex, la clase, el
// método y el proto, que es justo lo que un guion mal parseado rompería en silencio.
func mismoEvento(a, b eventoUsuario) bool {
	return a.Tipo == b.Tipo && a.Dato == b.Dato && a.Clase == b.Clase &&
		a.Metodo == b.Metodo && a.Proto == b.Proto && bytes.Equal(a.Dex, b.Dex)
}

// TestElIntercambioSoloSerializaSusCamposPublicos exige que `ejecucion.intercambios` lleve solo lo
// que el consumidor necesita: el `metodoDalvik` de Antes/Despues lleva su code_item y su dex, y sin
// `json:"-"` se serializaba como `{}` y ensuciaba el modelo.
func TestElIntercambioSoloSerializaSusCamposPublicos(t *testing.T) {
	m, _, _, _ := sesionDelHotSwap(t)
	d2, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2}))
	if err != nil {
		t.Fatalf("el dex de la versión 2 no se lee: %v", err)
	}
	r, err := m.intercambiaMetodo(d2, "Lcom/hola/Saludo;", "valor", "()I")
	if err != nil {
		t.Fatalf("el swap falló: %v", err)
	}
	if r.Antes == nil || r.Despues == nil {
		t.Fatalf("la prueba necesita las dos implementaciones y el swap dio %+v", r)
	}

	res := resultadoEjecucion{Intercambios: []resultadoIntercambio{r}}
	datos, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("el resultado no se serializa: %v", err)
	}
	var crudo struct {
		Intercambios []map[string]any `json:"intercambios"`
	}
	if err := json.Unmarshal(datos, &crudo); err != nil {
		t.Fatalf("el JSON no se relee: %v", err)
	}
	if len(crudo.Intercambios) != 1 {
		t.Fatalf("el JSON lleva %d intercambios y se esperaba 1: %s", len(crudo.Intercambios), datos)
	}
	claves := make([]string, 0, len(crudo.Intercambios[0]))
	for k := range crudo.Intercambios[0] {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	if got := strings.Join(claves, ","); got != "clase,existia,metodo,proto" {
		t.Fatalf("el JSON del intercambio lleva %q y solo se esperaban clase,existia,metodo,proto: %s",
			got, datos)
	}
	if strings.Contains(string(datos), `"Antes"`) || strings.Contains(string(datos), `"Despues"`) ||
		strings.Contains(string(datos), `"{}"`) {
		t.Fatalf("el JSON del intercambio se ensució con las implementaciones: %s", datos)
	}
}

// TestElFlagDeHotSwapParteClaseYMetodo comprueba la forma del comando, que es lo único que la
// primitiva no ve: la clase, el método y el proto salen de `clase/metodo(proto)=fichero.dex`, el
// dex se lee al parsear, y las formas sin descriptor o sin retorno se rechazan antes de ejecutar
// nada. (La forma `/metodo` sin proto se acepta; la cubre TestLaFirmaDeIntercambioAceptaLasTresFormas.)
func TestElFlagDeHotSwapParteClaseYMetodo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "saludo.dex")
	datos := dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2})
	if err := os.WriteFile(ruta, datos, 0o644); err != nil {
		t.Fatalf("el dex de la prueba no se escribe: %v", err)
	}

	ev, err := eventoDeIntercambio("Lcom/hola/Saludo;/valor()I=" + ruta)
	if err != nil {
		t.Fatalf("la forma clase/metodo(proto)=fichero no se parsea: %v", err)
	}
	esperado := eventoUsuario{
		Tipo:   ordenIntercambiarTipo,
		Dato:   "Lcom/hola/Saludo;/valor()I=" + ruta,
		Dex:    datos,
		Clase:  "Lcom/hola/Saludo;",
		Metodo: "valor",
		Proto:  "()I",
	}
	if !mismoEvento(ev, esperado) {
		t.Fatalf("el evento salió %+v y se esperaba %+v", ev, esperado)
	}
	// y la comparación no es sorda: cambiar cualquier campo, el dex incluido, la rompe
	for _, caso := range []struct {
		nombre string
		otro   eventoUsuario
	}{
		{"Tipo", eventoUsuario{Tipo: "otro", Dato: esperado.Dato, Dex: datos, Clase: esperado.Clase, Metodo: esperado.Metodo, Proto: esperado.Proto}},
		{"Dato", eventoUsuario{Tipo: esperado.Tipo, Dato: "otro", Dex: datos, Clase: esperado.Clase, Metodo: esperado.Metodo, Proto: esperado.Proto}},
		{"Dex", eventoUsuario{Tipo: esperado.Tipo, Dato: esperado.Dato, Dex: datos[:len(datos)-1], Clase: esperado.Clase, Metodo: esperado.Metodo, Proto: esperado.Proto}},
		{"Clase", eventoUsuario{Tipo: esperado.Tipo, Dato: esperado.Dato, Dex: datos, Clase: "Lcom/hola/Otra;", Metodo: esperado.Metodo, Proto: esperado.Proto}},
		{"Metodo", eventoUsuario{Tipo: esperado.Tipo, Dato: esperado.Dato, Dex: datos, Clase: esperado.Clase, Metodo: "otro", Proto: esperado.Proto}},
		{"Proto", eventoUsuario{Tipo: esperado.Tipo, Dato: esperado.Dato, Dex: datos, Clase: esperado.Clase, Metodo: esperado.Metodo, Proto: "()V"}},
	} {
		if mismoEvento(esperado, caso.otro) {
			t.Errorf("mismoEvento no distingue %s", caso.nombre)
		}
	}

	// sin /metodo, la clase entera
	ev, err = eventoDeIntercambio("Lcom/hola/Saludo;=" + ruta)
	if err != nil {
		t.Fatalf("la forma de clase entera no se parsea: %v", err)
	}
	if ev.Metodo != "" || ev.Proto != "" {
		t.Fatalf("la clase entera salió con método %q proto %q", ev.Metodo, ev.Proto)
	}

	for _, malo := range []string{
		"Lcom/hola/Saludo;/valor()I",         // sin fichero
		"com/hola/Saludo/valor()I=" + ruta,   // sin descriptor
		"Lcom/hola/Saludo;/valor()X=" + ruta, // retorno que no es descriptor
		"Lcom/hola/Saludo;/valor()=" + ruta,  // sin retorno
		"Lcom/hola/Saludo;/valor()I=/no/existe.dex",
	} {
		if _, err := eventoDeIntercambio(malo); err == nil {
			t.Errorf("se aceptó %q", malo)
		}
	}
}

// TestSinElEngancheDeHotSwapElMetodoViejoSigueRespondiendo es la falsificación del enganche: la misma
// sesión, el mismo objeto y el mismo dex nuevo, pero sin llamar al swap, contesta el cuerpo viejo. Es
// lo que se rompe si se quita `cl.metodos[nombre+proto] = met` de `intercambiaMetodo`.
func TestSinElEngancheDeHotSwapElMetodoViejoSigueRespondiendo(t *testing.T) {
	m, _, par, _ := sesionDelHotSwap(t)
	d2, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2}))
	if err != nil {
		t.Fatalf("el dex de la versión 2 no se lee: %v", err)
	}
	// el módulo nuevo se lee y se valida, pero no se engancha
	if _, err := metodoDeDex(d2, "Lcom/hola/Saludo;", "valor", "()I"); err != nil {
		t.Fatalf("el método nuevo no se pudo leer: %v", err)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 204 {
		t.Fatalf("sin enganchar, suma() = %d y se esperaba 204 (el cuerpo viejo)", got)
	}

	// y en la misma sesión, con el enganche, la respuesta cambia
	if _, err := m.intercambiaMetodo(d2, "Lcom/hola/Saludo;", "valor", "()I"); err != nil {
		t.Fatalf("el swap falló: %v", err)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 404 {
		t.Fatalf("con el enganche, suma() = %d y se esperaba 404 (el cuerpo nuevo)", got)
	}
}

// TestElHotSwapEsAditivoYNoTocaOtrosMetodos comprueba las dos mitades del reemplazo aditivo: un
// método que no existía se añade y responde, los que ya estaban conservan su implementación, y el
// estático que la app ya había movido conserva su valor al intercambiar la clase entera.
func TestElHotSwapEsAditivoYNoTocaOtrosMetodos(t *testing.T) {
	m, saludo, par, _ := sesionDelHotSwap(t)
	d2, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2}))
	if err != nil {
		t.Fatalf("el dex de la versión 2 no se lee: %v", err)
	}

	// el estático que la sesión ya tiene movido: el swap no puede devolverlo a su constante del dex
	cl := m.prog.clases["Lcom/hola/Saludo;"]
	cl.estaticos["Lcom/hola/Saludo;.base"] = int32(40)
	valorAntes := cl.metodos["valor()I"]
	subeAntes := cl.metodos["sube()V"]

	res, err := m.intercambiaMetodo(d2, "Lcom/hola/Saludo;", "extra", "()I")
	if err != nil {
		t.Fatalf("el swap aditivo de extra()I falló: %v", err)
	}
	if res.Existia {
		t.Fatalf("extra()I no existía y el swap dijo que la sustituía: %+v", res)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Saludo;", "extra", "()I", saludo), "extra"); got != 7 {
		t.Fatalf("extra() tras el swap = %d y se esperaba 7", got)
	}
	if cl.metodos["valor()I"] != valorAntes || cl.metodos["sube()V"] != subeAntes {
		t.Fatal("el swap aditivo tocó métodos que no nombraba")
	}

	// ahora la clase entera: valor() cambia, los demás siguen y el estático conserva el 40
	hechos, err := m.intercambiaClase(d2, "Lcom/hola/Saludo;")
	if err != nil {
		t.Fatalf("el swap de la clase entera falló: %v", err)
	}
	if len(hechos) != 6 {
		t.Fatalf("la clase v2 declara 6 métodos (con <clinit> y <init>) y el swap hizo %d", len(hechos))
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 404 {
		t.Fatalf("tras el swap de la clase, suma() = %d y se esperaba 404", got)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Saludo;", "conBase", "()I", saludo), "conBase"); got != 42 {
		t.Fatalf("conBase() = %d y se esperaba 42: el estático base no conservó el 40 de la sesión", got)
	}
	if cl.estaticos["Lcom/hola/Saludo;.base"] != int32(40) {
		t.Fatalf("el swap devolvió el estático base a %v", cl.estaticos["Lcom/hola/Saludo;.base"])
	}
}

// indiceDeConstString devuelve el string_idx de la primera constante de cadena del cuerpo de un
// método: `const-string vAA, string@BBBB` es 0x1a y la palabra siguiente es el índice.
func indiceDeConstString(t *testing.T, d *dexFile, clase, nombre, proto string) uint32 {
	t.Helper()
	met, err := metodoDeDex(d, clase, nombre, proto)
	if err != nil {
		t.Fatalf("no se pudo leer %s.%s%s: %v", claseCorta(clase), nombre, proto, err)
	}
	if met.item == nil {
		t.Fatalf("%s.%s%s no tiene cuerpo", claseCorta(clase), nombre, proto)
	}
	ins := met.item.insns
	for i := 0; i+1 < len(ins); i++ {
		if byte(ins[i]) == 0x1a {
			return uint32(ins[i+1])
		}
	}
	t.Fatalf("%s.%s%s no lleva const-string en su cuerpo: %#v", claseCorta(clase), nombre, proto, ins)
	return 0
}

// TestElHotSwapInterpretaElCuerpoNuevoContraSuPropioDex cierra la pregunta abierta: un code_item no es
// autónomo, sus índices apuntan a los pools de SU fichero. El cuerpo nuevo se compila en un dex con
// menos cadenas que el de la sesión, así que el índice de su `const-string` no vale lo mismo en los
// dos; si el intérprete lo resolviera contra `m.prog.dex`, devolvería otra cadena (o fallaría por
// índice fuera del pool) en vez de "nuevo".
func TestElHotSwapInterpretaElCuerpoNuevoContraSuPropioDex(t *testing.T) {
	d1, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "Etiqueta.java", contenido: fuenteEtiquetaV1},
		fuenteJava{nombre: "Relleno.java", contenido: fuenteRelleno},
	))
	if err != nil {
		t.Fatalf("el dex de la versión 1 no se lee: %v", err)
	}
	prog, err := cargaPrograma(d1)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	m := nuevaMaquina(prog, nil)
	etiqueta := &objetoDalvik{clase: "Lcom/hola/Etiqueta;", campos: map[string]any{}}
	texto := func() string {
		t.Helper()
		v := llamaProyecto(t, m, "Lcom/hola/Etiqueta;", "texto", "()Ljava/lang/String;", etiqueta)
		s, ok := v.(string)
		if !ok {
			t.Fatalf("texto() devolvió %T (%v) y se esperaba una cadena", v, v)
		}
		return s
	}
	if got := texto(); got != "viejo" {
		t.Fatalf("antes del swap texto() = %q y se esperaba \"viejo\"", got)
	}

	d2, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Etiqueta.java", contenido: fuenteEtiquetaV2}))
	if err != nil {
		t.Fatalf("el dex de la versión 2 no se lee: %v", err)
	}
	idx := indiceDeConstString(t, d2, "Lcom/hola/Etiqueta;", "texto", "()Ljava/lang/String;")
	if d2.strings[idx] != "nuevo" {
		t.Fatalf("el const-string del cuerpo nuevo apunta a %q y debía ser \"nuevo\"", d2.strings[idx])
	}
	if int(idx) < len(d1.strings) && d1.strings[idx] == "nuevo" {
		t.Fatalf("el índice %d dice \"nuevo\" también en el dex de la sesión: la prueba no discrimina", idx)
	}
	t.Logf("el cuerpo nuevo usa string@%d (\"nuevo\" en su dex); en el dex de la sesión ese índice es %q",
		idx, d1.strings[idx])

	if _, err := m.intercambiaMetodo(d2, "Lcom/hola/Etiqueta;", "texto", "()Ljava/lang/String;"); err != nil {
		t.Fatalf("el swap de texto() falló: %v", err)
	}
	if got := texto(); got != "nuevo" {
		t.Fatalf("tras el swap texto() = %q y se esperaba \"nuevo\": el cuerpo nuevo no se interpretó "+
			"contra su propio dex", got)
	}
}

// offsetDeCodeItem da la posición del code_item de un método en el fichero, que es lo que hay que
// tocar para fabricar un módulo roto sin inventarse bytes.
func offsetDeCodeItem(t *testing.T, d *dexFile, clase, nombre, proto string) uint32 {
	t.Helper()
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("los class_data no se leen: %v", err)
	}
	cd, ok := datos[clase]
	if !ok {
		t.Fatalf("el dex no declara %s", claseCorta(clase))
	}
	for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
		ref := d.methods[m.methodIdx]
		if ref.name == nombre && descDeProto(ref.proto) == proto {
			return m.codeOff
		}
	}
	t.Fatalf("el dex no trae %s.%s%s", claseCorta(clase), nombre, proto)
	return 0
}

// dexConSellos recalcula la firma y el adler32 de un dex parcheado: el lector comprueba las dos, así
// que un byte cambiado sin sellar no llega ni a la prueba.
func dexConSellos(b []byte) []byte {
	out := append([]byte(nil), b...)
	sum := sha1.Sum(out[32:])
	copy(out[12:32], sum[:])
	binary.LittleEndian.PutUint32(out[8:12], adler32.Checksum(out[12:]))
	return out
}

// TestElHotSwapDeClaseEsTodoONada mide la atomicidad: con un módulo cuyo último code_item no cabe,
// `intercambiaClase` falla al leer y NO deja la clase con unos métodos nuevos y otros viejos. Es la
// razón de leer y validar todo antes de tocar la tabla, en vez de deshacer a mano.
func TestElHotSwapDeClaseEsTodoONada(t *testing.T) {
	m, _, par, _ := sesionDelHotSwap(t)
	datos := dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2})
	d2, err := readDex(datos)
	if err != nil {
		t.Fatalf("el dex de la versión 2 no se lee: %v", err)
	}
	// el último método que se leería: si la instalación fuera incremental, ya estaría dentro
	off := offsetDeCodeItem(t, d2, "Lcom/hola/Saludo;", "valor", "()I")
	roto := append([]byte(nil), datos...)
	binary.LittleEndian.PutUint32(roto[off+12:], 0x7fffffff) // insns_size imposible
	dRoto, err := readDex(dexConSellos(roto))
	if err != nil {
		t.Fatalf("el dex parcheado no se lee: %v", err)
	}
	if _, err := metodoDeDex(dRoto, "Lcom/hola/Saludo;", "valor", "()I"); err == nil {
		t.Fatal("el code_item roto se leyó sin error: la prueba no discrimina")
	}

	cl := m.prog.clases["Lcom/hola/Saludo;"]
	antes := map[string]*metodoDalvik{}
	for k, v := range cl.metodos {
		antes[k] = v
	}
	if _, err := m.intercambiaClase(dRoto, "Lcom/hola/Saludo;"); err == nil {
		t.Fatal("el swap aceptó un módulo con un code_item roto")
	}
	if len(cl.metodos) != len(antes) {
		t.Fatalf("la tabla quedó con %d métodos y tenía %d: la instalación no fue todo o nada",
			len(cl.metodos), len(antes))
	}
	for k, v := range antes {
		if cl.metodos[k] != v {
			t.Fatalf("el swap fallido cambió %s: la instalación no fue todo o nada", k)
		}
	}
	if _, hay := cl.metodos["extra()I"]; hay {
		t.Fatal("el swap fallido añadió extra()I: la instalación no fue todo o nada")
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Par;", "suma", "()I", par), "suma"); got != 204 {
		t.Fatalf("tras el swap fallido suma() = %d y se esperaba 204: la sesión cambió", got)
	}
}
