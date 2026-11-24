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
// instrucción y el dex entero byte a byte salvo el constructor de la anónima, que es la diferencia ya
// medida del §100 (`MethodParameters` de este emisor frente al `Signature: ()V` de javac) y no de esta
// ruta.

import (
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

// TestReceptorHeredadoEnAnonimaContraJavac17: `getExternalFilesDir(null)` sobre el `getApplicationContext()`
// heredado se estima sin emitirlo, la sobrecarga `File(File, String)` del constructor se resuelve y el
// `onClick` sale instrucción a instrucción igual al de javac.
func TestReceptorHeredadoEnAnonimaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesReceptorHeredado(fuenteReceptorHeredado)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el método heredado sin receptor escrito: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteReceptorHeredado)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// la vuelta de `getExternalFilesDir(String)` es `Ljava/io/File;`: es lo que resuelve `File(File, String)`
	nuestraAnonima, suyaAnonima := buscaLaClase(t, nuestras, "Lp/Pantalla$1;", deJavac)
	comparaMetodos(t, nuestraAnonima, suyaAnonima)
	nuestroOnClick := metodoDeClaseEmitida(t, nuestras, "p/Pantalla$1", "onClick", "(Landroid/view/View;)V")
	suOnClick := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla$1;", "onClick", "(Landroid/view/View;)V")
	if err := comparaInstrucciones(nuestroOnClick.Code, suOnClick.Code); err != nil {
		t.Fatalf("Pantalla$1.onClick no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if err := comparaDexSalvoConstructorDeAnonima(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la anónima no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
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

// TestReceptorInexistenteSigueRechazando: el control. Un nombre que no existe en la jerarquía no se
// adivina: con la ruta encendida se rechaza con el mismo mensaje de antes (0 candidatas en esa aridad).
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
