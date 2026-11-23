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
	"os"
	"path/filepath"
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

// TestElFlagDeHotSwapParteClaseYMetodo comprueba la forma del comando, que es lo único que la
// primitiva no ve: la clase, el método y el proto salen de `clase/metodo(proto)=fichero.dex`, el
// dex se lee al parsear, y las formas ambiguas se rechazan antes de ejecutar nada.
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
	if ev.Tipo != ordenIntercambiarTipo || ev.Clase != "Lcom/hola/Saludo;" ||
		ev.Metodo != "valor" || ev.Proto != "()I" {
		t.Fatalf("el evento salió %+v", ev)
	}
	if len(ev.Dex) != len(datos) {
		t.Fatalf("el evento lleva %d bytes y el dex tiene %d", len(ev.Dex), len(datos))
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
		"Lcom/hola/Saludo;/valor()I",        // sin fichero
		"com/hola/Saludo/valor()I=" + ruta,  // sin descriptor
		"Lcom/hola/Saludo;/valor=" + ruta,   // sin proto
		"Lcom/hola/Saludo;/valor()=" + ruta, // sin retorno
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
