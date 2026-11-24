package main

// La llamada estática calificada por el nombre del tipo como argumento de otra llamada:
// `startActivity(Intent.createChooser(sharedIntent, "Compartir con"))`, la línea 212 de
// `src/share/apps/util/MainActivity.java` del zip de ApkShare. El argumento es un `jllamada` cuyo
// receptor (`Intent`) no es una local ni un campo: es el nombre de una clase del `android.jar`, y
// `tipoEstimado` lo trataba como una instancia que había que emitir, así que el `build` paraba con:
//
//	[build] MainActivity.java: linea 175: linea 176: linea 212: linea 212, columna 31: no se puede
//	estimar el tipo de createChooser() sin emitir el receptor de la llamada
//
// La vuelta sale del descriptor del método leído del jar (`vueltaEstaticaCualificada`), con la misma
// resolución que usa la emisión para la forma «una llamada estática sobre un nombre de tipo»: el prefijo
// más largo que resuelve como tipo tiene que ser el receptor entero y la candidata tiene que ser
// estática. No se escribe ninguna instrucción de más y el orden de evaluación de Java queda intacto; la
// emisión sigue bajando los argumentos sin receptor y escribiendo `invokestatic` contra la clase, que es
// lo que escribe javac (nunca `invokevirtual`, que en ART revienta con `IncompatibleClassChangeError`).
//
// El oráculo es el de siempre: javac 17 con `--release 8 -g:lines,source` por el mismo dexer, el dex
// entero byte a byte y las instrucciones método a método. La sonda directa y la de la anónima son la
// misma línea por los dos caminos de resolución (receptor implícito y método del exterior que captura la
// anónima); el control de instancia comprueba que un receptor que es variable no cambia de opcode.

import (
	"strings"
	"testing"
)

// fuenteEstaticaCualificada es la línea 212 sin la anónima: `startActivity(Intent)` resuelve su
// sobrecarga con el tipo de `Intent.createChooser(...)`, que se estima sin emitir el receptor.
const fuenteEstaticaCualificada = `package p;

import android.content.Intent;

public class Pantalla extends android.app.Activity {
    public void comparte(Intent compartido) {
        startActivity(Intent.createChooser(compartido, "Compartir con"));
    }
}
`

// fuenteEstaticaCualificadaEnAnonima es la forma exacta de la línea 212: dentro de la anónima de un
// `Runnable`, con el `Intent` capturado del método que la creó (`val$compartido`) y `startActivity`
// resuelto en la clase exterior, que es el camino por el que llegó el rechazo del usuario.
const fuenteEstaticaCualificadaEnAnonima = `package p;

import android.content.Intent;

public class Pantalla extends android.app.Activity {
    public void comparte(final Intent compartido) {
        enHilo(new Runnable() {
            public void run() {
                startActivity(Intent.createChooser(compartido, "Compartir con"));
            }
        });
    }

    void enHilo(Runnable hilo) {
        hilo.run();
    }
}
`

// fuenteEstaticaCualificadaMala: el control del tipo incorrecto. El segundo parámetro de
// `createChooser` es `CharSequence` y aquí va un `int`: javac lo rechaza y el emisor tiene que
// rechazarlo también, con el motivo de la candidata y no aceptándolo por estimar cualquier cosa.
const fuenteEstaticaCualificadaMala = `package p;

import android.content.Intent;

public class Pantalla extends android.app.Activity {
    public void comparte(Intent compartido) {
        startActivity(Intent.createChooser(compartido, 7));
    }
}
`

// fuenteInstanciaCalificada: el control de la instancia. Un receptor que es una variable sigue su camino
// de siempre: `invokevirtual` para una clase (`StringBuilder.append`) e `invokeinterface` para una
// interfaz (`List.add`/`List.size`). La ruta nueva no toca ninguno de los dos.
const fuenteInstanciaCalificada = `package p;

import java.util.List;

public class Pantalla {
    public void usa(List<String> lista, StringBuilder texto) {
        texto.append(lista.size());
        lista.add("uno");
    }
}
`

// fuenteEstaticaCualificadaAncha: la llamada estática calificada por el tipo como argumento que se
// ensancha (`f(S.n())` con `f(long)`). Es el variante «cualificado» de la guarda de
// `javaemisor_ancho_argumento_test.go`: con la vuelta estimada desde el jar, el `i2l` se escribe donde
// lo escribe javac en vez de resolverse la sobrecarga después de bajar el argumento.
const fuenteEstaticaCualificadaAncha = `public class S {
    static int n() {
        return 1;
    }

    static void f(long b) {
    }

    static void t() {
        f(S.n());
    }
}
`

// llevaI2lAntesDeInvocacion: si alguna instrucción del opcode dado lleva un `i2l` inmediatamente delante,
// que es como javac ensancha el argumento antes de la llamada.
func llevaI2lAntesDeInvocacion(t *testing.T, code []byte, op byte) bool {
	t.Helper()
	instrucciones, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("no se desarma el código: %v", err)
	}
	for i, in := range instrucciones {
		if in.op == op && i > 0 && instrucciones[i-1].name == "i2l" {
			return true
		}
	}
	return false
}

// TestEstaticaCualificadaFalsificacion: apagada la ruta nueva, la sonda vuelve a fallar con el mensaje
// literal que leyó el usuario, nombrando el fuente y la línea de la llamada.
func TestEstaticaCualificadaFalsificacion(t *testing.T) {
	const literal = "no se puede estimar el tipo de createChooser() sin emitir el receptor de la llamada"
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuenteEstaticaCualificada}}

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con la ruta encendida el fuente no compila: %v", err)
	}

	anterior := estimaEstaticaCualificada
	estimaEstaticaCualificada = false
	t.Cleanup(func() { estimaEstaticaCualificada = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario contiene %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "Pantalla.java: ") || !strings.Contains(err.Error(), "linea 7") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la llamada: %q", err)
	}
	t.Logf("sin la ruta: %v", err)
}

// cuentaOpcode: cuántas instrucciones de un opcode lleva un `Code`, desarmado con `walkJvmCode`. Es la
// vista que usan los controles para nombrar la forma de la invocación sin depender del índice del pool.
func cuentaOpcode(t *testing.T, code []byte, op byte) int {
	t.Helper()
	instr, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("no se desarma el código: %v", err)
	}
	n := 0
	for _, in := range instr {
		if in.op == op {
			n++
		}
	}
	return n
}
