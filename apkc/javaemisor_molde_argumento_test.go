package main

// El molde `(T) x` como argumento de una llamada: `tipoEstimado` tiene que devolver el descriptor del
// molde y no el de su receptor, que puede ser una llamada encadenada que no se sabe sin emitirla. Es
// el bloqueo que el usuario encontró al compilar ApkShare con `--compiler go`:
//
//	MainActivity.java: linea 52: linea 53: linea 55: linea 55, columna 38: el tipo de una expresion
//	main.jcast no se puede estimar sin emitirla
//
// La línea 55 es `openAppDetailsDialog((App) apps.getAdapter().getItem(positon));`. El oráculo es el de
// siempre: nuestro `.class` y el de javac 17 `--release 8 -g:lines,source` por el dexer, byte a byte,
// y las instrucciones y los `max_stack`/`locals` método a método. La sonda es el mismo caso sin la
// anónima —el receptor es un campo de la propia clase— porque hoy la captura de una local en posición
// de receptor (`apps.getAdapter()`) es la frontera siguiente y no la resuelve esta ruta.

import (
	"strings"
	"testing"
)

// fuenteSondaMolde: el caso mínimo con la forma exacta de la línea 55. `abreDetalle` está sobrecargado
// a propósito: si la estimación del argumento devolviera el tipo del receptor (`Object`, lo que
// devuelve `getItem`) en vez del molde (`App`), la sobrecarga elegida sería otra y el dex no
// coincidiría con el de javac.
const fuenteSondaMolde = `package p;

class App {
    public int id;
}

class Adaptador {
    Object getItem(int posicion) {
        return null;
    }
}

class Sonda {
    private Adaptador adaptador;

    Sonda(Adaptador adaptador) {
        this.adaptador = adaptador;
    }

    public void abreDetalle(App app) {
        app.id = 1;
    }

    public void abreDetalle(Object objeto) {
        objeto.hashCode();
    }

    public void crea(int positon) {
        abreDetalle((App) adaptador.getItem(positon));
    }
}
`

func fuentesSondaMolde() []fuenteJava {
	return []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteSondaMolde}}
}

// TestMoldeDeArgumentoFalsificacion: con la ruta apagada, la sonda vuelve a fallar con el mensaje que
// el usuario leyó. Se comprueba el texto literal del emisor (`%T` de un `jcast` es `main.jcast`,
// porque el paquete de la herramienta es `main`) y que el prefijo del fuente y la línea son los de la
// llamada, no los de la declaración.
func TestMoldeDeArgumentoFalsificacion(t *testing.T) {
	const literal = "el tipo de una expresion main.jcast no se puede estimar sin emitirla"
	jar := jarDelSDK(t)
	fuentes := fuentesSondaMolde()

	// encendida, la sonda se acepta y llega hasta el final
	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con la ruta del molde encendida la sonda no se aceptó: %v", err)
	}

	anterior := estimaMoldeSinEmitir
	estimaMoldeSinEmitir = false
	t.Cleanup(func() { estimaMoldeSinEmitir = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta del molde apagada la sonda fue aceptada: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con la ruta apagada el rechazo es %q, y no lleva el mensaje del usuario %q", err, literal)
	}
	// `abreDetalle((App) …)` está en la línea 29 del fuente de la sonda: el prefijo del fuente y la
	// línea de la llamada son los que el usuario ve en MainActivity.java:55
	if !strings.HasPrefix(err.Error(), "Sonda.java: ") || !strings.Contains(err.Error(), "linea 29") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la llamada: %q", err)
	}
	t.Logf("sin la ruta: %v", err)
}
