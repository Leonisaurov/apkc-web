package main

// El ensanchamiento de un argumento en la llamada (JLS 5.1.2), medido con javac 17
// `--release 8 -g:lines,source` y el dex byte a byte.
//
// El defecto: con `h.f(n)` donde `n` es un `int` y `f` declara `long`, javac escribe `iload; i2l` y
// deja DOS palabras en la pila, que es lo que el descriptor `(J)V` descuenta. El emisor bajaba el
// argumento con su propio tipo (una palabra) y descontaba después el descriptor ya resuelto (dos), así
// que el método cerraba con `quedan -1 valores en la pila`, que es el error con el que ApkShare se
// paraba en `MainActivity.java:105`: `handler.postDelayed(runnable, AutoResearchDelay)`, con
// `AutoResearchDelay` un `int` contra el `(Ljava/lang/Runnable;J)Z` de `android.os.Handler`. Con dos
// argumentos así el saldo era `-2`.
//
// La corrección tiene dos caras, las dos en `javaemisor.go`:
//
//	`necesitaConversionEn` (antes `necesitaBoxingEn`) manda resolver la sobrecarga ANTES de bajar los
//	    argumentos también cuando el parámetro pide un ensanchamiento que escribe opcode, así que
//	    `expA` mete el `i2l`/`i2f`/`i2d` en el sitio de javac. Los literales siguen plegándose sobre la
//	    constante (`f(97)` es `ldc2_w 97l`, no `bipush 97; i2l`), que es lo que mide el §85.
//	`expA` sube `byte`/`short`/`char`/`boolean` a `int` sin opcode y desde ahí convierte: `f(long)` con
//	    un `byte b` es `iload; i2l`, no `i2b` de por medio (JVMS 2.11.1).
//
// Y para el camino que NO puede resolver antes —el tipo del argumento no se puede estimar sin
// emitirlo, p. ej. `s.f(s.n())`—, `revisaAnchosDeArgumentos` convierte el descuadre silencioso en un
// error con su línea. Ahí la conversión iría detrás de un valor ya emitido y en medio de los demás
// argumentos, y eso pide reemitir la lista; queda como deuda (ver el informe del frente).

import (
	"strings"
	"testing"
)

// Sondas de una línea, como las del resto del emisor, para que la LineNumberTable coincida con la de
// javac y el dex se pueda comparar entero.
const (
	anchoLargoFuente       = `public class S { void f(long b){} void t(S s, int n){ s.f(n); } }`
	anchoDosLargosFuente   = `public class S { void g(long a, long b){} void t(S s, int n, int m){ s.g(n, m); } }`
	anchoFlotanteFuente    = `public class S { void h(float x){} void t(S s, int n){ s.h(n); } }`
	anchoDobleFuente       = `public class S { void i(double x){} void t(S s, int n){ s.i(n); } }`
	anchoMezclaFuente      = `public class S { void j(int a, long b){} void t(S s, int n){ s.j(n, n); } }`
	anchoEstaticoFuente    = `public class S { static void k(long b){} static void t(int n){ k(n); } }`
	anchoCualificadoFuente = `public class S { static void k(long b){} static void t(int n){ S.k(n); } }`
	anchoByteFuente        = `public class S { void f(long b){} void t(S s, byte b){ s.f(b); } }`
	anchoShortFuente       = `public class S { void f(double b){} void t(S s, short b){ s.f(b); } }`
	anchoCharFuente        = `public class S { void f(float b){} void t(S s, char c){ s.f(c); } }`
	anchoControlFuente     = `public class S { void f(long b){} void t(S s, long n){ s.f(n); } }`
	anchoLiteralFuente     = `public class S { void f(long b){} void t(S s){ s.f(97); } }`
	anchoCaracterFuente    = `public class S { void f(long b){} void t(S s){ s.f('a'); } }`
	anchoCeroFuente        = `public class S { void f(double b){} void t(S s){ s.f(0); } }`
	anchoThisFuente        = `public class S { S(){} S(long L){ this(); } S(int n){ this((long) n); } }`
)

// TestElEnsanchamientoDelArgumentoLlevaSuConversion: los quince casos salen byte a byte como los de
// javac 17. Las dos primeras son el defecto (una y dos palabras de menos); las de `float` y `double`
// conservan el saldo pero les faltaba el `i2f`/`i2d`; las de `byte`, `short` y `char` son la subida a
// `int` sin opcode y la conversión detrás; las tres últimas son el control —`long` a `long` no escribe
// nada, y el literal se pliega sobre la constante (`ldc2_w 97l`, `fconst_0`, `dconst_1`)— y el
// `this(...)` de un constructor, que pasa por la misma cuenta.
func TestElEnsanchamientoDelArgumentoLlevaSuConversion(t *testing.T) {
	casos := []struct{ nombre, fuente string }{
		{"int_a_long", anchoLargoFuente},
		{"dos_int_a_dos_long", anchoDosLargosFuente},
		{"int_a_float", anchoFlotanteFuente},
		{"int_a_double", anchoDobleFuente},
		{"int_y_long_mezclados", anchoMezclaFuente},
		{"static_desde_la_propia_clase", anchoEstaticoFuente},
		{"static_cualificado_por_el_tipo", anchoCualificadoFuente},
		{"byte_a_long", anchoByteFuente},
		{"short_a_double", anchoShortFuente},
		{"char_a_float", anchoCharFuente},
		{"control_long_a_long", anchoControlFuente},
		{"control_literal_entero", anchoLiteralFuente},
		{"control_literal_caracter", anchoCaracterFuente},
		{"control_literal_cero", anchoCeroFuente},
		{"this_con_long", anchoThisFuente},
	}
	for _, c := range casos {
		_, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/S.java", contenido: c.fuente}})
		comparaDexByteAByte(t, nuestro, suyo, c.nombre)
	}
}

// fuenteRetardoShare reproduce el bloque de `~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/src/
// share/apps/util/MainActivity.java:84-107` —verbatim, con `handler.postDelayed(runnable,
// AutoResearchDelay)` en la 105— dentro de una clase con los miembros que ese bloque usa: `handler` y
// `runnable` son los campos del `MainActivity` real, `AutoResearchDelay` el `int` de la clase, y
// `adapter` y `searchQuery` los stubs mínimos para no arrastrar `AppAdapter` ni `R`.
const fuenteRetardoShare = `package share.apps.util;

import android.os.Handler;
import android.text.Editable;
import android.text.TextWatcher;
import android.widget.EditText;

public class Retardo {
    Handler handler = new Handler();
    Runnable runnable;
    int AutoResearchDelay = 600;
    AppAdapter adapter;
    EditText searchQuery;

    void observa() {
        searchQuery.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {

            }

            @Override
            public void onTextChanged(CharSequence s, int start, int before, int count) {
                if (runnable != null) {
                    handler.removeCallbacks(runnable);
                }
            }

            @Override
            public void afterTextChanged(Editable s) {
                runnable = new Runnable() {
                    @Override
                    public void run() {
                        adapter.searchQuery(s.toString());
                    }
                };
                handler.postDelayed(runnable, AutoResearchDelay);
            }
        });
    }
}
`

const fuenteAppAdapterStub = `package share.apps.util;
public class AppAdapter {
    public void searchQuery(String q) {}
}
`

// TestElPostDelayedDeApkShareLlevaSuI2L: el bloque real compila —el `-1` que paraba ApkShare ya no
// está— y el `afterTextChanged` de la anónima sale instrucción a instrucción como el de javac 17,
// con su `max_stack` y sus huecos, y con el `i2l` de la 105 justo delante del `invokevirtual` de
// `android.os.Handler.postDelayed:(Ljava/lang/Runnable;J)Z`. El dex entero no se compara: javac no
// escribe el `MethodParameters` de los constructores de la anónima que sí emite este emisor, y esa
// diferencia es de otro frente.
func TestElPostDelayedDeApkShareLlevaSuI2L(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{
		{nombre: "src/share/apps/util/Retardo.java", contenido: fuenteRetardoShare},
		{nombre: "src/share/apps/util/AppAdapter.java", contenido: fuenteAppAdapterStub},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el bloque real de ApkShare no compila: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	const (
		clase = "share/apps/util/Retardo$1"
		desc  = "(Landroid/text/Editable;)V"
	)
	nuestra := metodoDeClaseEmitida(t, nuestras, clase, "afterTextChanged", desc)
	suya := metodoDeClaseLeida(t, deJavac, "L"+clase+";", "afterTextChanged", desc)
	if err := comparaInstrucciones(nuestra.Code, suya.Code); err != nil {
		t.Fatalf("afterTextChanged: %v", err)
	}
	if nuestra.MaxStack != suya.MaxStack || nuestra.LocalCount != suya.LocalCount {
		t.Errorf("afterTextChanged: max_stack/locals %d/%d y javac %d/%d",
			nuestra.MaxStack, nuestra.LocalCount, suya.MaxStack, suya.LocalCount)
	}
	instrucciones, err := walkJvmCode(nuestra.Code)
	if err != nil {
		t.Fatalf("nuestro `afterTextChanged` no se desarma: %v", err)
	}
	for i, in := range instrucciones {
		if in.name != "invokevirtual" {
			continue
		}
		if i == 0 || instrucciones[i-1].name != "i2l" {
			t.Fatalf("el `invokevirtual` de la pc %d no lleva el `i2l` detrás: %v", in.pc, instrucciones)
		}
		t.Logf("`postDelayed` en la pc %d, con el `i2l` de la pc %d delante", in.pc, instrucciones[i-1].pc)
		return
	}
	t.Fatalf("no hay ningún `invokevirtual` en `afterTextChanged`: %v", instrucciones)
}

// TestFalsificacionDelEnsanchamientoEnElArgumento: con el interruptor apagado vuelve el defecto
// literal —el argumento se baja antes de resolver y el descriptor ancho descuenta una palabra de
// más—, tanto en la sonda mínima como en el bloque de ApkShare. Es el error que el frente anterior
// midió en `MainActivity.java:105`.
func TestFalsificacionDelEnsanchamientoEnElArgumento(t *testing.T) {
	jar := jarDelSDKExigido(t)
	anterior := ajustaAnchosDeArgumento
	ajustaAnchosDeArgumento = false
	t.Cleanup(func() { ajustaAnchosDeArgumento = anterior })

	_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: anchoLargoFuente}}, jar)
	if err == nil {
		t.Fatal("sin el ajuste, `s.f(n)` con `f(long)` y `n` int compila: el interruptor no apaga la ruta")
	}
	if !strings.Contains(err.Error(), "quedan -1 valores en la pila al cerrar el metodo") {
		t.Errorf("el error de la sonda no es el defecto medido: %v", err)
	}
	t.Logf("sonda mínima sin el ajuste: %v", err)

	_, err = compilarProyectoJava([]fuenteJava{
		{nombre: "src/share/apps/util/Retardo.java", contenido: fuenteRetardoShare},
		{nombre: "src/share/apps/util/AppAdapter.java", contenido: fuenteAppAdapterStub},
	}, jar)
	if err == nil {
		t.Fatal("sin el ajuste, el bloque real de ApkShare compila: el interruptor no apaga la ruta")
	}
	if !strings.Contains(err.Error(), "quedan -1 valores en la pila al cerrar el metodo") {
		t.Errorf("el error del bloque real no es el defecto medido: %v", err)
	}
	t.Logf("bloque de ApkShare sin el ajuste: %v", err)
}

// TestElArgumentoAnchoQueNoSePuedeEstimarSeRechaza: cuando el tipo del argumento no se puede estimar
// sin emitirlo, la sobrecarga se resuelve después de bajar los argumentos y ya no hay dónde meter la
// conversión. Se rechaza con la línea y las dos cuentas; antes de esta guarda el método cerraba con
// `-1` y el `.class` salía con la pila descuadrada.
//
// La ruta `vueltaSinEmitir` estima ahora `s.n()` sin emitir el receptor (`S.n` tiene una sola candidata
// y devuelve `I`), y `vueltaEstaticaCualificada` estima `S.n()` leyendo el jar, así que la guarda se
// mide con **las dos** rutas apagadas: sigue cubriendo el caso de un argumento cuyo tipo no se sabe. Con
// la ruta encadenada encendida el mismo fuente se emite como javac, y eso lo fija
// `TestLlamadaEncadenadaAnchaContraJavac17` (`javaemisor_llamada_argumento_test.go`); con la estática
// encendida lo fija `TestEstaticaCualificadaAnchaContraJavac17`
// (`javaemisor_estatica_cualificada_test.go`).
func TestElArgumentoAnchoQueNoSePuedeEstimarSeRechaza(t *testing.T) {
	anterior := estimaLlamadaEncadenada
	estimaLlamadaEncadenada = false
	t.Cleanup(func() { estimaLlamadaEncadenada = anterior })

	anteriorEstatica := estimaEstaticaCualificada
	estimaEstaticaCualificada = false
	t.Cleanup(func() { estimaEstaticaCualificada = anteriorEstatica })

	fuentes := map[string]string{
		"receptor_explicito": `public class S { int n(){ return 1; } void f(long b){} void t(S s){ s.f(s.n()); } }`,
		"cualificado":        `public class S { static int n(){ return 1; } static void f(long b){} static void t(S s){ s.f(S.n()); } }`,
	}
	for nombre, fuente := range fuentes {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: fuente}}, jarDelSDKExigido(t))
		if err == nil {
			t.Errorf("%s: se aceptó un argumento ancho que no se puede estimar", nombre)
			continue
		}
		if !strings.Contains(err.Error(), "(J)V pide 2 palabras y los argumentos dejan 1") {
			t.Errorf("%s: el rechazo no explica la conversión: %v", nombre, err)
		}
	}
}
