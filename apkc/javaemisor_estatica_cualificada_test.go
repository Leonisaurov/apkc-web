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
	"os/exec"
	"path/filepath"
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

// TestEstaticaCualificadaComoArgumentoContraJavac17: la línea 212 con receptor implícito. El dex entero
// sale igual al de javac 17, con un solo `invokestatic` (la llamada a `createChooser`) y un solo
// `invokevirtual` (el `startActivity`).
func TestEstaticaCualificadaComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuenteEstaticaCualificada}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada estática calificada por el tipo: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteEstaticaCualificada)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Pantalla;", deJavac)
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Pantalla", "comparte", "(Landroid/content/Intent;)V")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla;", "comparte", "(Landroid/content/Intent;)V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("comparte(Landroid/content/Intent;)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if n := cuentaOpcode(t, suyo.Code, opInvokestat); n != 1 {
		t.Fatalf("javac 17 ya no escribe un solo `invokestatic` en la sonda (%d): el oráculo no vale", n)
	}
	if n := cuentaOpcode(t, nuestro.Code, opInvokestat); n != 1 {
		t.Fatalf("`Intent.createChooser` no sale con `invokestatic`: hay %d `invokestatic` en %#x", n, nuestro.Code)
	}
	if n := cuentaOpcode(t, nuestro.Code, opInvokevirt); n != 1 {
		t.Fatalf("se esperaba un solo `invokevirtual` (el `startActivity`) y hay %d en %#x", n, nuestro.Code)
	}
	comparaMetodos(t, nuestra, suya)
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestEstaticaCualificadaAnchaContraJavac17: la llamada estática calificada por el tipo como argumento
// que se ensancha (`s.f(S.n())` con `f(long)`). La vuelta se estima `I` desde el jar y el `i2l` se
// escribe delante del `invokevirtual`, como javac; es el variante «cualificado» de la guarda de
// `javaemisor_ancho_argumento_test.go`, que se mide con esta ruta apagada.
func TestEstaticaCualificadaAnchaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/S.java", contenido: fuenteEstaticaCualificadaAncha}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada estática calificada ancha: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "S.java", fuenteEstaticaCualificadaAncha)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestro := metodoDeClaseEmitida(t, nuestras, "S", "t", "()V")
	suyo := metodoDeClaseLeida(t, deJavac, "LS;", "t", "()V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("t()V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	instrucciones, err := walkJvmCode(nuestro.Code)
	if err != nil {
		t.Fatalf("nuestro `t` no se desarma: %v", err)
	}
	// el `i2l` del argumento ancho va justo delante del `invokestatic` de `f`, como en javac
	if !llevaI2lAntesDeInvocacion(t, nuestro.Code, opInvokestat) {
		t.Fatalf("ningún `invokestatic` lleva el `i2l` delante: %v", instrucciones)
	}
	if !llevaI2lAntesDeInvocacion(t, suyo.Code, opInvokestat) {
		t.Fatalf("javac ya no escribe el `i2l` delante del `invokestatic`: el oráculo no vale")
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

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

// TestEstaticaCualificadaEnAnonimaContraJavac17: la línea 212 tal cual, dentro de la anónima que captura
// el `Intent` y resuelve `startActivity` en la Activity exterior. Se comparan las instrucciones de `run`
// y el dex entero con el constructor de la anónima normalizado, que es la única diferencia conocida entre
// javac y este emisor (§100).
func TestEstaticaCualificadaEnAnonimaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuenteEstaticaCualificadaEnAnonima}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada estática calificada dentro de la anónima: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteEstaticaCualificadaEnAnonima)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Pantalla$1", "run", "()V")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla$1;", "run", "()V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("Pantalla$1.run no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	// en la anónima, `startActivity` va por el `this$0` (invokevirtual) y `createChooser` por la clase
	if n := cuentaOpcode(t, nuestro.Code, opInvokestat); n != 1 {
		t.Fatalf("`Intent.createChooser` en la anónima no sale con un solo `invokestatic`: %d en %#x", n, nuestro.Code)
	}
	if err := comparaDexSalvoConstructorDeAnonima(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la anónima no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
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

// TestEstaticaCualificadaTipoIncorrectoSeRechaza: el control negativo. Un argumento que no encaja en la
// firma de `createChooser` no se acepta: la estimación resuelve la candidata de verdad y devuelve su
// motivo, en vez de estimar el tipo por el nombre del método. javac 17 lo rechaza igual.
func TestEstaticaCualificadaTipoIncorrectoSeRechaza(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuenteEstaticaCualificadaMala}}

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("un `Intent.createChooser(Intent, int)` se aceptó: la estimación adivina en vez de resolver")
	}
	// el motivo puede ser el de la candidata o el de la anchura del argumento en la emisión: lo que
	// importa es que la llamada resolvió y no se aceptó por el nombre del método
	t.Logf("con el argumento de tipo incorrecto: %v", err)

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteEstaticaCualificadaMala)
	salida := filepath.Join(raiz, "javac-out")
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "p/Pantalla.java")}
	if out, err := exec.Command(javac, args...).CombinedOutput(); err == nil {
		t.Fatalf("javac 17 aceptó el caso que se quiere rechazar: el control no vale\n%s", out)
	}
}

// TestInstanciaCalificadaPorVariableSigueSiendoInvokeVirtual: el control de la instancia. Un receptor
// que es una variable no pasa por la ruta nueva: las instrucciones siguen siendo las de javac, con el
// `invokevirtual` de `StringBuilder.append` y los `invokeinterface` de `List`.
func TestInstanciaCalificadaPorVariableSigueSiendoInvokeVirtual(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuenteInstanciaCalificada}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada sobre una variable: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteInstanciaCalificada)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Pantalla;", deJavac)
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Pantalla", "usa", "(Ljava/util/List;Ljava/lang/StringBuilder;)V")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla;", "usa", "(Ljava/util/List;Ljava/lang/StringBuilder;)V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("usa(Ljava/util/List;Ljava/lang/StringBuilder;)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if n := cuentaOpcode(t, nuestro.Code, opInvokevirt); n != 1 {
		t.Fatalf("`StringBuilder.append` no sale con un solo `invokevirtual`: %d en %#x", n, nuestro.Code)
	}
	if n := cuentaOpcode(t, nuestro.Code, opInvointf); n != 2 {
		t.Fatalf("`List.size` y `List.add` no salen con `invokeinterface`: %d en %#x", n, nuestro.Code)
	}
	if n := cuentaOpcode(t, nuestro.Code, opInvokestat); n != 0 {
		t.Fatalf("una llamada sobre una variable salió con `invokestatic`: %d en %#x", n, nuestro.Code)
	}
	comparaMetodos(t, nuestra, suya)
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
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
