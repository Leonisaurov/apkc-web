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

// fuenteLlamadaDeLocalGenerico: el receptor de la llamada encadenada es un **parámetro** con
// argumentos de tipo (`List<Apk> m`). El analizador guarda el borrado (`List`) y los argumentos solo
// viajan en `p.generico`, así que sin propagarlos `m.get(0)` devuelve `Object`, se elige `usa(Object)`
// y se pierde el `check-cast` que escribe javac. Es la sonda del crítico.
const fuenteLlamadaDeLocalGenerico = `package p;

import java.util.List;

class Apk {
    public String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }
}

public class Sonda {
    void dame(List<Apk> m) {
        usa(m.get(0));
    }

    void usa(Apk apk) {
        apk.ruta.toString();
    }

    void usa(Object objeto) {
        objeto.hashCode();
    }
}
`

// fuenteLlamadaDeMapaGenerico: el mismo caso con dos parámetros de tipo y una clave que no es la
// vuelta (`Map<String,Apk>`, cuyo `get` declara `(Ljava/lang/Object;)TV;`). El tipo que decide la
// sobrecarga es el segundo, y el control comprueba que la propagación no se queda con el primero.
const fuenteLlamadaDeMapaGenerico = `package p;

import java.util.Map;

class Apk {
    public String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }
}

public class Sonda {
    void dame(Map<String, Apk> m) {
        usa(m.get("uno"));
    }

    void usa(Apk apk) {
        apk.ruta.toString();
    }

    void usa(Object objeto) {
        objeto.hashCode();
    }
}
`

// fuenteLlamadaDeLocalSinArgumentos: el control del fail-safe. Con `var` la local no guarda su tipo
// declarado (el emisor la reserva con el descriptor que deja el valor, `List` y nada más), así que los
// argumentos de `List<Apk>` no se conocen: la ruta no estima y devuelve el rechazo explícito en vez
// del `Object` en silencio.
const fuenteLlamadaDeLocalSinArgumentos = `package p;

import java.util.List;

class Apk {
    public String ruta;
}

public class Sonda {
    void dame(List<Apk> m) {
        var copia = m;
        usa(copia.get(0));
    }

    void usa(Apk apk) {
        apk.ruta.toString();
    }

    void usa(Object objeto) {
        objeto.hashCode();
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

// TestLlamadaEncadenadaDeLocalGenericoContraJavac17: el receptor es un parámetro `List<Apk>` y no un
// campo. El tipo declarado del parámetro llega del AST y `vueltaGenerica` sustituye la `E` de
// `List.get` por `Apk`, así que la sobrecarga elegida es `usa(Apk)` con su `check-cast`, y el dex sale
// byte a byte. Sin la propagación se elige `usa(Object)` y el `check-cast` desaparece: 6 instrucciones
// frente a 7, que es lo que muestran las dos falsificaciones.
func TestLlamadaEncadenadaDeLocalGenericoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteLlamadaDeLocalGenerico}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada encadenada sobre un parámetro genérico: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteLlamadaDeLocalGenerico)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Sonda;", deJavac)
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "dame", "(Ljava/util/List;)V")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Sonda;", "dame", "(Ljava/util/List;)V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("dame(Ljava/util/List;)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if !llevaCheckcast(t, suyo.Code) {
		t.Fatal("javac ya no escribe el check-cast: el oráculo de este test no vale")
	}
	comparaMetodos(t, nuestra, suya)
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestLlamadaEncadenadaDeMapaGenericoContraJavac17: el control de la sustitución por posición. `Map`
// declara `<K,V>` y `get` devuelve `TV;`, así que el tipo que decide la sobrecarga es el **segundo**
// argumento: con el primero (`String`) se elegiría `usa(Object)` y el dex no coincidiría.
func TestLlamadaEncadenadaDeMapaGenericoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteLlamadaDeMapaGenerico}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la llamada encadenada sobre un parámetro `Map`: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteLlamadaDeMapaGenerico)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Sonda;", deJavac)
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "dame", "(Ljava/util/Map;)V")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Sonda;", "dame", "(Ljava/util/Map;)V")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("dame(Ljava/util/Map;)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	comparaMetodos(t, nuestra, suya)
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestLlamadaEncadenadaDeLocalGenericoFalsificacion: con la propagación apagada vuelve el defecto de
// `c0b40e9` —el borrado `Object` en silencio—, así que hay que enseñarlo tal cual: el `dame` de la
// sonda sale con el `check-cast` menos y elige `usa(Object)`. La comprobación es doble: el dex deja de
// coincidir con javac y el opcode `checkcast` (0xc0) desaparece del método.
func TestLlamadaEncadenadaDeLocalGenericoFalsificacion(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteLlamadaDeLocalGenerico}}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteLlamadaDeLocalGenerico)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	suCodigo := metodoDeClaseLeida(t, deJavac, "Lp/Sonda;", "dame", "(Ljava/util/List;)V").Code

	// encendida, el dex es el de javac
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("con la propagación encendida el fuente no compila: %v", err)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatalf("con la propagación encendida el dex no es el de javac: %v", err)
	}

	anterior := propagaGenericoDeLocal
	propagaGenericoDeLocal = false
	t.Cleanup(func() { propagaGenericoDeLocal = anterior })

	nuestras, err = compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("con la propagación apagada el fuente no compila: %v", err)
	}
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "dame", "(Ljava/util/List;)V")
	if llevaCheckcast(t, nuestro.Code) {
		t.Fatalf("con la propagación apagada sigue habiendo `check-cast`; la falsificación no enseña el defecto: %#x", nuestro.Code)
	}
	if llevara := llevaCheckcast(t, suCodigo); !llevara {
		t.Fatal("javac ya no escribe el `check-cast`: el oráculo de la falsificación no vale")
	}
	if err := comparaInstrucciones(nuestro.Code, suCodigo); err == nil {
		t.Fatal("con la propagación apagada las instrucciones siguen siendo las de javac: la falsificación no falsifica")
	}
	nInstr, suyas := cuentaInstrucciones(t, nuestro.Code), cuentaInstrucciones(t, suCodigo)
	if nInstr+1 != suyas {
		t.Fatalf("con la propagación apagada salen %d instrucciones y javac %d: se esperaba una menos (el `check-cast`)", nInstr, suyas)
	}
	t.Logf("sin la propagación, dame(Ljava/util/List;)V: %#x (javac %#x)", nuestro.Code, suCodigo)
}

// TestLlamadaEncadenadaDeLocalSinArgumentosSeRechaza: el fail-safe. Si el tipo declarado del local con
// sus argumentos no se conoce, la ruta **no** devuelve el borrado: se rechaza con el mensaje de
// siempre. Sin esto, `copia.get(0)` elegiría `usa(Object)` y se comería el `check-cast` en silencio.
func TestLlamadaEncadenadaDeLocalSinArgumentosSeRechaza(t *testing.T) {
	const literal = "no se puede estimar el tipo de get() sin emitir el receptor de la llamada"
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteLlamadaDeLocalSinArgumentos}}

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("una local sin tipo declarado conocido se aceptó: la ruta adivina el borrado")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("el rechazo de la local sin argumentos es %q, y se esperaba el de siempre %q", err, literal)
	}
	t.Logf("local sin argumentos declarados: %v", err)
}

// llevaCheckcast y cuentaInstrucciones: las dos vistas del código desarmado que usan las
// falsificaciones, para nombrar el opcode que desaparece sin depender del volcado entero.
func llevaCheckcast(t *testing.T, code []byte) bool {
	t.Helper()
	instr, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("no se desarma el código: %v", err)
	}
	for _, in := range instr {
		if in.op == opCheckcast {
			return true
		}
	}
	return false
}

func cuentaInstrucciones(t *testing.T, code []byte) int {
	t.Helper()
	instr, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("no se desarma el código: %v", err)
	}
	return len(instr)
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
