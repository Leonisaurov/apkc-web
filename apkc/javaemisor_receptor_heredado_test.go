package main

// El método heredado sin receptor escrito dentro de una anónima: `MainActivity.java:175-179` de ApkShare,
// donde el `onClick` de un `Runnable`/`OnClickListener` construye
//
//	new File(getApplicationContext().getExternalFilesDir(null), apk.name + ".apk")
//
// Ni `getApplicationContext()` ni `getExternalFilesDir(String)` están en la anónima: los hereda la
// Activity que la encierra (de `Activity`/`Context`, en el `android.jar`) y javac los alcanza por el
// campo `this$0`. `vueltaSinEmitir` sabía resolver la llamada encadenada cuando el receptor se escribía
// (`app.apks.get(0)`, §244 y el frente de `c0b40e9`), pero `tipoPrevisto` solo miraba la clase actual y
// con `getApplicationContext()` devolvía 0 candidatas; el fuente paraba con:
//
//	[build] MainActivity.java: linea 175: linea 176: linea 179: linea 179, columna 67: no se puede
//	estimar el tipo de getExternalFilesDir() sin emitir el receptor de la llamada
//
// Ahora `candidatasSinReceptor` busca el nombre, cuando la clase actual no lo declara, en la cadena de
// clases que la encierran, de dentro afuera, que es el mismo orden con el que la emisión resuelve el
// nombre suelto (parágrafo 72). No se escribe ninguna instrucción: el orden de evaluación de Java queda
// intacto y solo se lee la jerarquía.
//
// El oráculo es el de siempre: javac 17 con `--release 8 -g:lines,source`, el `onClick` instrucción a
// instrucción y el dex entero byte a byte, también el constructor de la anónima: ya no escribe
// `MethodParameters` y javac 17 tampoco (§100, corregido al fijar el oráculo en 17). La prueba de
// `usa(f("x"))` que pregunta a javac vive en `javaemisor_receptor_heredado_paridad_test.go` con
// `//go:build !js`, porque bajo wasm no hay proceso que lanzar; aquí queda la que solo mira lo emitido.

import (
	"bytes"
	"strings"
	"testing"
)

// fuenteReceptorHeredado es el caso de la línea 179 sin el resto de ApkShare. La línea 12 es la que cita
// la falsificación: `new File(getApplicationContext().getExternalFilesDir(null), nombre)`.
const fuenteReceptorHeredado = `package p;

import android.app.Activity;
import android.view.View;

import java.io.File;

public class Pantalla extends Activity {
    public void comparte(View boton, final String nombre) {
        boton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                File destino = new File(getApplicationContext().getExternalFilesDir(null), nombre);
                destino.exists();
            }
        });
    }
}
`

// fuenteReceptorInexistente es el control: un nombre que no existe ni en la anónima, ni en su cadena de
// padres, ni en la Activity que la encierra se sigue rechazando con el mensaje de antes, sin adivinar.
const fuenteReceptorInexistente = `package p;

import android.app.Activity;
import android.view.View;

import java.io.File;

public class Pantalla extends Activity {
    public void comparte(View boton) {
        boton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                File destino = new File(getArchivosInexistentes(), "x");
                destino.exists();
            }
        });
    }
}
`

func fuentesReceptorHeredado(fuente string) []fuenteJava {
	return []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuente}}
}

// TestReceptorHeredadoFalsificacion: apagada la ruta nueva, la sonda vuelve a fallar con el mensaje
// literal que leyó el usuario, nombrando el fuente y la línea de la llamada.
func TestReceptorHeredadoFalsificacion(t *testing.T) {
	const literal = "no se puede estimar el tipo de getExternalFilesDir() sin emitir el receptor de la llamada"
	jar := jarDelSDK(t)
	fuentes := fuentesReceptorHeredado(fuenteReceptorHeredado)

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con la ruta encendida el fuente no compila: %v", err)
	}

	anterior := estimaMetodoDeExterior
	estimaMetodoDeExterior = false
	t.Cleanup(func() { estimaMetodoDeExterior = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario contiene %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "Pantalla.java: ") || !strings.Contains(err.Error(), "linea 12") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la llamada: %q", err)
	}
	t.Logf("sin la ruta: %v", err)
}

// fuenteSobrecargaExterior es la sonda de la sobrecarga de igual aridad en la clase que encierra a la
// anónima: `f(String)` y `f(int)` tienen la misma aridad, la anónima llama `usa(f("x"))` y `usa` vuelve a
// estar sobrecargado. javac 17 resuelve `f("x")` por el tipo del literal (`String`) y después
// `usa(String)`; este emisor estimaba el tipo de una llamada sin receptor contando candidatas por nombre
// y aridad, veía las dos `f` y rechazaba el fuente con «hay 2 candidatas … y el tipo lo decide la
// sobrecarga». La línea 15 es la que cita la falsificación.
const fuenteSobrecargaExterior = `package p;

import android.app.Activity;
import android.view.View;

public class Pantalla extends Activity {
    String f(String s) { return s; }
    int f(int n) { return n; }

    void usa(String s) { }
    void usa(int n) { }

    public void comparte(View boton) {
        boton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                usa(f("x"));
            }
        });
    }
}
`

// TestSobrecargaExteriorFalsificacion: apagado el filtro por tipos, el fuente que javac acepta vuelve a
// rechazarse con el mensaje que veía el usuario, nombrando el fuente, la línea y las dos candidatas.
func TestSobrecargaExteriorFalsificacion(t *testing.T) {
	const literal = "no se puede estimar el tipo de f(): hay 2 candidatas con 1 argumentos y el tipo lo decide la sobrecarga"
	jar := jarDelSDKExigido(t)
	fuentes := fuentesReceptorHeredado(fuenteSobrecargaExterior)

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con el filtro encendido el fuente no compila: %v", err)
	}

	anterior := filtraCandidatasPorTipos
	filtraCandidatasPorTipos = false
	t.Cleanup(func() { filtraCandidatasPorTipos = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con el filtro apagado el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con el filtro apagado el rechazo es %q, y el de antes contiene %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "Pantalla.java: ") || !strings.Contains(err.Error(), "linea 15") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la llamada: %q", err)
	}
	t.Logf("sin el filtro: %v", err)
}

// TestReceptorInexistenteSigueRechazando: el control. Un nombre que no existe en la jerarquía no se
// adivina: con la ruta encendida se rechaza con el mismo mensaje de antes (0 candidatas en esa aridad).
// El caso de una sola candidata (`getExternalFilesDir(null)`) lo fija el test de arriba, que sigue
// comparando byte a byte con javac.
func TestReceptorInexistenteSigueRechazando(t *testing.T) {
	const literal = "no se puede estimar el tipo de getArchivosInexistentes(): hay 0 candidatas con 0 argumentos y el tipo lo decide la sobrecarga"
	jar := jarDelSDK(t)
	fuentes := fuentesReceptorHeredado(fuenteReceptorInexistente)

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("un nombre que no existe en la jerarquía se aceptó: la ruta adivina")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("el rechazo del nombre inexistente es %q, y se esperaba el de siempre %q", err, literal)
	}
	t.Logf("nombre inexistente: %v", err)
}

// TestSobrecargaExteriorResuelvePorElTipo: con dos sobrecargas de igual aridad en la clase exterior, el
// estimador resuelve por el tipo de los argumentos —las mismas fases estricta→laxa que la emisión
// (`elegirSobrecarga`)— sin javac de por medio: `f("x")` es `String`, así que la llamada que queda en el
// `onClick` de la anónima es `usa:(Ljava/lang/String;)V` y no la de `int`. La paridad instrucción a
// instrucción y en dex con javac 17 es `TestSobrecargaExteriorContraJavac17`, en
// `javaemisor_receptor_heredado_paridad_test.go`.
func TestSobrecargaExteriorResuelvePorElTipo(t *testing.T) {
	jar := jarDelSDKExigido(t)
	fuentes := fuentesReceptorHeredado(fuenteSobrecargaExterior)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `usa(f(\"x\"))` con dos sobrecargas de igual aridad en la clase exterior: %v", err)
	}
	anonima := porInterno(nuestras, "p/Pantalla$1")
	if anonima == nil {
		t.Fatalf("no salió la anónima Pantalla$1: %s", nombresDe(nuestras))
	}
	// en el `onClick` de la anónima solo entran `f("x")` y `usa(...)`; la forma `String` del descriptor
	// es la que javac elige por el tipo del literal, y la de `int` no puede aparecer
	if !bytes.Contains(anonima.datos, []byte("(Ljava/lang/String;)V")) {
		t.Errorf("la anónima no llama a las sobrecargas de `String`: %s", volcadoDeClases(nuestras))
	}
	if bytes.Contains(anonima.datos, []byte("(I)V")) {
		t.Errorf("la anónima cayó en las sobrecargas de `int`: %s", volcadoDeClases(nuestras))
	}
}
