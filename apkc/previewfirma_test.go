package main

// La firma de `--intercambiar` se parte con la gramática del descriptor DEX, no buscando el último
// `;`. Un proto con tipos de referencia (`(Lcom/hola/Saludo;)I`) lleva `;` propios después del que
// cerró la clase, y cortar por el último partía la firma dentro del parámetro: la UI solo podía usar
// la forma de clase entera cuando el método tenía referencias.
//
//   - las tres formas (clase, clase/metodo sin proto, clase/metodo(proto)retorno) dan el evento con
//     Clase, Metodo y Proto correctos;
//   - la forma con referencia en el proto, pasada por el camino real (parsear y luego intercambiar),
//     sustituye el método vivo y su cuerpo nuevo se ejecuta;
//   - la forma sin proto resuelve por nombre cuando la clase tiene un solo método con ese nombre, y
//     se rechaza pidiendo el proto completo cuando el nombre está sobrecargado;
//   - falsificación: con el `LastIndex(";")` de antes, la forma con referencia se rechaza al parsear.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fuenteSaludoRefV1 = `package com.hola;
public class Saludo {
    int n;
    public void sube() { n = n + 1; }
    public int valor() { return n + 100; }
    public int suma(Saludo otro) { return n + otro.valor() + 1000; }
}
`

const fuenteSaludoRefV2 = `package com.hola;
public class Saludo {
    int n;
    public void sube() { n = n + 1; }
    public int valor() { return n + 100; }
    public int suma(Saludo otro) { return n + otro.valor() + 2000; }
}
`

func TestLaFirmaDeIntercambioAceptaLasTresFormas(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "saludo.dex")
	datos := dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoRefV2})
	if err := os.WriteFile(ruta, datos, 0o644); err != nil {
		t.Fatalf("el dex de la prueba no se escribe: %v", err)
	}

	casos := []struct {
		forma  string
		clase  string
		metodo string
		proto  string
	}{
		// solo la clase: el evento pide la clase entera
		{"Lcom/hola/Saludo;", "Lcom/hola/Saludo;", "", ""},
		// clase y método sin proto: el método se pide por nombre
		{"Lcom/hola/Saludo;/suma", "Lcom/hola/Saludo;", "suma", ""},
		// clase, método y proto con un tipo de referencia dentro
		{"Lcom/hola/Saludo;/suma(Lcom/hola/Saludo;)I", "Lcom/hola/Saludo;", "suma", "(Lcom/hola/Saludo;)I"},
	}
	for _, c := range casos {
		ev, err := eventoDeIntercambio(c.forma + "=" + ruta)
		if err != nil {
			t.Errorf("la forma %q no se parsea: %v", c.forma, err)
			continue
		}
		if ev.Clase != c.clase || ev.Metodo != c.metodo || ev.Proto != c.proto {
			t.Errorf("la forma %q salió con clase %q método %q proto %q; se esperaba %q %q %q",
				c.forma, ev.Clase, ev.Metodo, ev.Proto, c.clase, c.metodo, c.proto)
		}
		if ev.Tipo != ordenIntercambiarTipo || ev.Dato != c.forma+"="+ruta || !bytes.Equal(ev.Dex, datos) {
			t.Errorf("la forma %q no dejó el evento completo: %+v", c.forma, ev)
		}
	}

	// el proto con arreglo tampoco confunde el troceo, y el `[` se queda dentro del proto
	if _, metodo, proto, err := parteFirmaDeIntercambio("Lcom/hola/Saludo;/pinta([I[[Lcom/hola/Saludo;)V"); err != nil {
		t.Errorf("el proto con arreglos no se parte: %v", err)
	} else if metodo != "pinta" || proto != "([I[[Lcom/hola/Saludo;)V" {
		t.Errorf("el proto con arreglos salió método %q proto %q", metodo, proto)
	}

	for _, malo := range []string{
		"com/hola/Saludo;",                      // sin descriptor
		"Lcom/hola/Saludo",                      // descriptor sin `;`
		"Lcom/hola/Saludo;/",                    // `/` sin método
		"Lcom/hola/Saludo;/(Com/hola/Saludo;)V", // proto sin nombre de método
		"Lcom/hola/Saludo;/suma()",              // proto sin retorno
		"Lcom/hola/Saludo;/suma()X",             // retorno que no es descriptor
		"Lcom/hola/Saludo/suma()I",              // barra fuera del descriptor
	} {
		if _, _, _, err := parteFirmaDeIntercambio(malo); err == nil {
			t.Errorf("se aceptó %q", malo)
		}
	}
}

// TestLaFirmaConReferenciaEnElProtoIntercambiaElMetodoVivo encadena el parseo real del flag con el
// swap real del intérprete: el proto con `;` dentro del parámetro llega entero a `intercambiaMetodo`
// y el cuerpo nuevo responde sobre el objeto vivo, con su `n` de antes.
func TestLaFirmaConReferenciaEnElProtoIntercambiaElMetodoVivo(t *testing.T) {
	d1, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoRefV1},
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
	for i := 0; i < 2; i++ {
		llamaProyecto(t, m, "Lcom/hola/Saludo;", "sube", "()V", saludo)
	}

	// antes del swap: n=2, valor()=102 y suma(saludo)=2+102+1000
	sumaViva := func() int32 {
		t.Helper()
		met := m.buscaMetodo("Lcom/hola/Saludo;", "suma", "(Lcom/hola/Saludo;)I")
		if met == nil {
			t.Fatal("no se encontró Saludo.suma(Lcom/hola/Saludo;)I en la sesión")
		}
		v, err := m.ejecutaCuerpo("Lcom/hola/Saludo;", met,
			[]any{saludo, saludo}, []string{"Lcom/hola/Saludo;", "Lcom/hola/Saludo;"})
		if err != nil {
			t.Fatalf("suma(Lcom/hola/Saludo;)I falló: %v", err)
		}
		n, ok := v.(int32)
		if !ok {
			t.Fatalf("suma devolvió %T (%v) y se esperaba un int", v, v)
		}
		return n
	}
	if got := sumaViva(); got != 1104 {
		t.Fatalf("antes del swap suma() = %d y se esperaba 1104 (2 + 102 + 1000)", got)
	}

	ruta := filepath.Join(t.TempDir(), "saludo2.dex")
	datos2 := dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoRefV2})
	if err := os.WriteFile(ruta, datos2, 0o644); err != nil {
		t.Fatalf("el dex de la versión 2 no se escribe: %v", err)
	}
	ev, err := eventoDeIntercambio("Lcom/hola/Saludo;/suma(Lcom/hola/Saludo;)I=" + ruta)
	if err != nil {
		t.Fatalf("la firma con referencia en el proto no se parsea: %v", err)
	}
	if ev.Clase != "Lcom/hola/Saludo;" || ev.Metodo != "suma" || ev.Proto != "(Lcom/hola/Saludo;)I" {
		t.Fatalf("el evento salió clase %q método %q proto %q", ev.Clase, ev.Metodo, ev.Proto)
	}

	nuevo, err := readDex(ev.Dex)
	if err != nil {
		t.Fatalf("el dex del evento no se lee: %v", err)
	}
	res, err := m.intercambiaMetodo(nuevo, ev.Clase, ev.Metodo, ev.Proto)
	if err != nil {
		t.Fatalf("el swap de %s%s falló: %v", ev.Metodo, ev.Proto, err)
	}
	if !res.Existia || res.Antes == nil || res.Despues == nil {
		t.Fatalf("el swap debía sustituir el método existente y dijo %+v", res)
	}

	// después: el cuerpo nuevo ve el mismo n (2) y suma 2000 en vez de 1000
	if got := sumaViva(); got != 2104 {
		t.Fatalf("tras el swap suma() = %d y se esperaba 2104 (2 + 102 + 2000): el proto con "+
			"referencia no llegó al swap o no se despachó el cuerpo nuevo", got)
	}
}

// Las dos versiones de la clase tienen un solo método: sin proto, el nombre basta para resolverlo.
const fuenteValorUnicoV1 = `package com.hola;
public class Saludo {
    int n;
    public void sube() { n = n + 1; }
    public int valor() { return n + 100; }
}
`

const fuenteValorUnicoV2 = `package com.hola;
public class Saludo {
    int n;
    public void sube() { n = n + 1; }
    public int valor() { return n + 200; }
}
`

// TestLaFirmaSinProtoResuelvePorNombre intercambia con `Lcom/hola/Saludo;/valor`, sin proto: el dex
// del swap declara un único `valor`, así que la resolución por nombre lo elige, el swap se aplica y
// el cuerpo nuevo responde sobre el objeto vivo con su estado.
func TestLaFirmaSinProtoResuelvePorNombre(t *testing.T) {
	d1, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "Saludo.java", contenido: fuenteValorUnicoV1},
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
	for i := 0; i < 2; i++ {
		llamaProyecto(t, m, "Lcom/hola/Saludo;", "sube", "()V", saludo)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Saludo;", "valor", "()I", saludo), "valor"); got != 102 {
		t.Fatalf("antes del swap valor() = %d y se esperaba 102 (2 + 100)", got)
	}

	ruta := filepath.Join(t.TempDir(), "saludo2.dex")
	datos2 := dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteValorUnicoV2})
	if err := os.WriteFile(ruta, datos2, 0o644); err != nil {
		t.Fatalf("el dex de la versión 2 no se escribe: %v", err)
	}
	ev, err := eventoDeIntercambio("Lcom/hola/Saludo;/valor=" + ruta)
	if err != nil {
		t.Fatalf("la firma sin proto no se parsea: %v", err)
	}
	if ev.Metodo != "valor" || ev.Proto != "" {
		t.Fatalf("la firma sin proto salió método %q proto %q", ev.Metodo, ev.Proto)
	}

	nuevo, err := readDex(ev.Dex)
	if err != nil {
		t.Fatalf("el dex del evento no se lee: %v", err)
	}
	res, err := m.intercambiaMetodo(nuevo, ev.Clase, ev.Metodo, ev.Proto)
	if err != nil {
		t.Fatalf("el swap por nombre de valor falló: %v", err)
	}
	if !res.Existia || res.Antes == nil || res.Despues == nil {
		t.Fatalf("el swap debía sustituir el método existente y dijo %+v", res)
	}
	// el resultado dice el proto que resolvió el nombre, no el vacío del flag
	if res.Proto != "()I" || res.Metodo != "valor" {
		t.Fatalf("el swap por nombre resolvió %s%s y se esperaba valor()I", res.Metodo, res.Proto)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Saludo;", "valor", "()I", saludo), "valor"); got != 202 {
		t.Fatalf("tras el swap valor() = %d y se esperaba 202 (2 + 200): la resolución por "+
			"nombre no enganchó el cuerpo nuevo", got)
	}
}

// Dos sobrecargas del mismo nombre: sin proto la elección sería una adivinanza.
const fuenteValorDobleV1 = `package com.hola;
public class Saludo {
    int n;
    public int valor() { return n + 100; }
    public int valor(int x) { return n + x + 100; }
}
`

const fuenteValorDobleV2 = `package com.hola;
public class Saludo {
    int n;
    public int valor() { return n + 200; }
    public int valor(int x) { return n + x + 200; }
}
`

// TestLaFirmaSinProtoConSobrecargasPideElProto exige el rechazo cuando el nombre está sobrecargado:
// el mensaje dice cuáles son los protos y pide el completo. Con el proto completo el mismo dex sí se
// intercambia, para que el rechazo no tape un fallo de lectura.
func TestLaFirmaSinProtoConSobrecargasPideElProto(t *testing.T) {
	d1, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "Saludo.java", contenido: fuenteValorDobleV1},
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
	llamaProyecto(t, m, "Lcom/hola/Saludo;", "valor", "()I", saludo)

	ruta := filepath.Join(t.TempDir(), "saludo2.dex")
	datos2 := dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteValorDobleV2})
	if err := os.WriteFile(ruta, datos2, 0o644); err != nil {
		t.Fatalf("el dex de la versión 2 no se escribe: %v", err)
	}

	// sin proto: rechazo que nombra las dos sobrecargas y pide el proto completo
	ev, err := eventoDeIntercambio("Lcom/hola/Saludo;/valor=" + ruta)
	if err != nil {
		t.Fatalf("la firma sin proto no se parsea: %v", err)
	}
	nuevo, err := readDex(ev.Dex)
	if err != nil {
		t.Fatalf("el dex del evento no se lee: %v", err)
	}
	_, err = m.intercambiaMetodo(nuevo, ev.Clase, ev.Metodo, ev.Proto)
	if err == nil {
		t.Fatal("el swap sin proto aceptó un nombre sobrecargado")
	}
	for _, quiere := range []string{"sobrecargado", "()I", "(I)I", "proto completo"} {
		if !strings.Contains(err.Error(), quiere) {
			t.Fatalf("el rechazo no dice %q: %v", quiere, err)
		}
	}
	// y la sesión no cambió: valor() sigue respondiendo el cuerpo viejo
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Saludo;", "valor", "()I", saludo), "valor"); got != 100 {
		t.Fatalf("tras el rechazo valor() = %d y se esperaba 100: la sesión se tocó", got)
	}

	// con el proto completo, el mismo dex sí entra
	ev, err = eventoDeIntercambio("Lcom/hola/Saludo;/valor()I=" + ruta)
	if err != nil {
		t.Fatalf("la firma con proto completo no se parsea: %v", err)
	}
	res, err := m.intercambiaMetodo(nuevo, ev.Clase, ev.Metodo, ev.Proto)
	if err != nil {
		t.Fatalf("el swap con proto completo falló: %v", err)
	}
	if !res.Existia {
		t.Fatalf("el swap con proto completo no sustituyó valor()I: %+v", res)
	}
	if got := enteroDe(t, llamaProyecto(t, m, "Lcom/hola/Saludo;", "valor", "()I", saludo), "valor"); got != 200 {
		t.Fatalf("tras el swap valor() = %d y se esperaba 200", got)
	}
}
