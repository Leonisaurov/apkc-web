package main

// La llamada encadenada como argumento de otra: `shareApp(app.apks.get(0))`, la línea 163 de
// `src/share/apps/util/MainActivity.java` del zip de ApkShare. El argumento es un `jllamada` cuyo
// receptor (`app.apks`, un `List<Apk>`) todavía no se ha emitido, y `tipoEstimado` se negaba a
// estimarlo; el `check` paraba con:
//
//	[build] MainActivity.java: linea 161: linea 162: linea 163: linea 163, columna 30: no se puede
//	estimar el tipo de get() sin emitir el receptor de la llamada
//
// La vuelta se resuelve desde el tipo estático del receptor —`tipoSinEmitirCompleto`, como el §244 con
// los campos— y el descriptor del método, que trae la aridad y la vuelta borrada; el argumento de tipo
// del `Signature` la estrecha (`List` declara `(I)TE;` y el campo es un `List<Apk>`, así que la vuelta
// es `Apk`, que es con lo que resuelve javac). No se emite ninguna instrucción de más: el orden de
// evaluación de Java queda intacto.
//
// El oráculo es el de siempre: javac 17 con `--release 8 -g:lines,source` por el mismo dexer, el dex
// entero byte a byte, y las instrucciones y los `max_stack`/`locals` método a método. Las dos
// sobrecargas de `comparte` son a propósito: con el borrado `Object` donde va el genérico `Apk` se
// elegiría `comparte(Object)` y el dex dejaría de coincidir con el de javac.

import (
	"strings"
	"testing"
)

// fuenteLlamadaEncadenada es el caso de `MainActivity.java:163` sin la anónima: el receptor es una
// cadena de campos sobre una local, que es lo que ya sabía leer `tipoSinEmitir`.
const fuenteLlamadaEncadenada = `package p;

import java.util.List;

class Apk {
    public String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }
}

class App {
    public final List<Apk> apks;

    public App(List<Apk> apks) {
        this.apks = apks;
    }
}

public class Pantalla {
    public void abreDetalle(App app) {
        comparte(app.apks.get(0));
    }

    public void comparte(Apk apk) {
        apk.ruta.toString();
    }

    public void comparte(Object objeto) {
        objeto.hashCode();
    }
}
`

// fuenteLlamadaEnAnonima es la forma exacta de la línea 163: dentro de la anónima de
// `setOnClickListener`, con `app` capturada del método que la creó (`val$app`).
const fuenteLlamadaEnAnonima = `package p;

import android.view.View;

import java.util.List;

class Apk {
    public String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }
}

class App {
    public final List<Apk> apks;

    public App(List<Apk> apks) {
        this.apks = apks;
    }
}

public class Pantalla {
    public void abreDetalle(App app, View boton) {
        boton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                comparte(app.apks.get(0));
            }
        });
    }

    public void comparte(Apk apk) {
        apk.ruta.toString();
    }

    public void comparte(Object objeto) {
        objeto.hashCode();
    }
}
`

// fuenteLlamadaSinTipo: el control. El receptor es el elemento de un arreglo que es campo y no local,
// así que su tipo no se sabe sin emitirlo: la ruta nueva no lo estima y el rechazo sigue siendo el de
// antes, sin adivinar.
const fuenteLlamadaSinTipo = `package p;

import java.util.List;

class Apk {
    public String ruta;
}

class App {
    public List[] cajas;
}

public class Pantalla {
    public void abreDetalle(App app) {
        comparte(app.cajas[0].get(0));
    }

    public void comparte(Apk apk) {
        apk.ruta.toString();
    }

    public void comparte(Object objeto) {
        objeto.hashCode();
    }
}
`

func fuentesLlamadaEncadenada(fuente string) []fuenteJava {
	return []fuenteJava{{nombre: "src/p/Pantalla.java", contenido: fuente}}
}

// fuenteLlamadaEncadenadaAncha: el argumento de la llamada encadenada es un `int` que llena un `long`
// (`s.f(s.n())`), el caso que la guarda de `javaemisor_ancho_argumento_test.go` rechazaba mientras el
// tipo de `s.n()` no se sabía sin emitir su receptor. Ahora se estima y el `i2l` se escribe delante de
// la llamada, como javac.
const fuenteLlamadaEncadenadaAncha = `public class S {
    int n() {
        return 1;
    }

    void f(long b) {
    }

    void t(S s) {
        s.f(s.n());
    }
}
`

// TestLlamadaEncadenadaComoArgumentoContraJavac17: la vuelta de `get()` se estima sin emitir el
// receptor y el dex entero sale igual al de javac 17, sobrecarga elegida incluida.
func TestLlamadaEncadenadaComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesLlamadaEncadenada(fuenteLlamadaEncadenada)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada encadenada como argumento: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteLlamadaEncadenada)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Pantalla;", deJavac)
	nuestroAbre := metodoDeClaseEmitida(t, nuestras, "p/Pantalla", "abreDetalle", "(Lp/App;)V")
	suAbre := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla;", "abreDetalle", "(Lp/App;)V")
	if err := comparaInstrucciones(nuestroAbre.Code, suAbre.Code); err != nil {
		t.Fatalf("abreDetalle(Lp/App;)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	comparaMetodos(t, nuestra, suya)
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestLlamadaEncadenadaEnAnonimaContraJavac17: la línea 163 tal cual, dentro de la anónima que captura
// `app`. Se comparan las instrucciones de `onClick` y el dex entero con el constructor de la anónima
// normalizado, que es la única diferencia conocida entre javac y este emisor (§100).
func TestLlamadaEncadenadaEnAnonimaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesLlamadaEncadenada(fuenteLlamadaEnAnonima)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada encadenada dentro de la anónima: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteLlamadaEnAnonima)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestroOnClick := metodoDeClaseEmitida(t, nuestras, "p/Pantalla$1", "onClick", "(Landroid/view/View;)V")
	suOnClick := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla$1;", "onClick", "(Landroid/view/View;)V")
	if err := comparaInstrucciones(nuestroOnClick.Code, suOnClick.Code); err != nil {
		t.Fatalf("Pantalla$1.onClick no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if err := comparaDexSalvoConstructorDeAnonima(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la anónima no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestLlamadaEncadenadaFalsificacion: apagada la ruta nueva, la sonda vuelve a fallar con el mensaje
// literal que leyó el usuario, nombrando el fuente y la línea de la llamada.
func TestLlamadaEncadenadaFalsificacion(t *testing.T) {
	const literal = "no se puede estimar el tipo de get() sin emitir el receptor de la llamada"
	jar := jarDelSDK(t)
	fuentes := fuentesLlamadaEncadenada(fuenteLlamadaEncadenada)

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con la ruta encendida el fuente no compila: %v", err)
	}

	anterior := estimaLlamadaEncadenada
	estimaLlamadaEncadenada = false
	t.Cleanup(func() { estimaLlamadaEncadenada = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario contiene %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "Pantalla.java: ") || !strings.Contains(err.Error(), "linea 23") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la llamada: %q", err)
	}
	t.Logf("sin la ruta: %v", err)
}

// TestLlamadaEncadenadaAnchaContraJavac17: la llamada encadenada cuyo argumento además se ensancha
// (`s.f(s.n())`). Mientras el tipo de `s.n()` no se sabía sin emitir el receptor, la sobrecarga se
// resolvía después de bajar el argumento y el `i2l` se perdía: la guarda de
// `javaemisor_ancho_argumento_test.go` lo rechazaba. Con la estimación nueva se escribe el `i2l` donde
// lo escribe javac.
func TestLlamadaEncadenadaAnchaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/S.java", contenido: fuenteLlamadaEncadenadaAncha}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada encadenada ancha: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "S.java", fuenteLlamadaEncadenadaAncha)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestro := metodoDeClaseEmitida(t, nuestras, "S", "t", "(LS;)V")
	suyo := metodoDeClaseLeida(t, deJavac, "LS;", "t", "(LS;)V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("t(LS;)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestLlamadaEncadenadaSinTipoSigueRechazando: el control. Con la ruta encendida, un `get()` sobre un
// receptor cuyo tipo no se sabe sin código (`app.cajas[0]`, un elemento de un arreglo que es campo) se
// rechaza con el mismo mensaje: la ruta no adivina.
func TestLlamadaEncadenadaSinTipoSigueRechazando(t *testing.T) {
	const literal = "no se puede estimar el tipo de get() sin emitir el receptor de la llamada"
	jar := jarDelSDK(t)
	fuentes := fuentesLlamadaEncadenada(fuenteLlamadaSinTipo)

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("un receptor sin tipo se aceptó: la ruta adivina")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("el rechazo del receptor sin tipo es %q, y se esperaba el de siempre %q", err, literal)
	}
	t.Logf("receptor sin tipo: %v", err)
}
