package main

// El rechazo de `boolean` como argumento que ensancha (JLS 5.1.2), medido con javac 17
// `--release 8`.
//
// El defecto que corrige este frente: `conversionPrimitiva` y la rama de anchos de `expA` metían `Z`
// en el mismo cajón que `B`/`C`/`S` (`strings.IndexByte("BCSZ", …)`), así que `f(true)` contra
// `f(long)`/`f(float)`/`f(double)` se resolvía antes de bajar el argumento y `expA` escribía
// `iload; i2l` (`i2f`, `i2d`) sobre un `boolean`. javac 17 no compila ese fuente y el padre de
// `af73a8a` lo rechazaba con `quedan -1 valores en la pila…`; el frente lo aceptaba en silencio. El
// rechazo de ahora sale con línea y nombra el tipo y el parámetro (o la sobrecarga, si hay varias),
// nunca con el descuadre de pila.
//
// `byte`, `short`, `char` e `int` **sí** ensanchan: son el control y siguen saliendo byte a byte
// como javac 17. La falsificación vuelve a meter `Z` con `booleanEnsanchaEnArgumentos` y muestra que
// el programa inválido se acepta otra vez.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// booleanSondas: un solo candidato (donde `buscarMetodo` devuelve la única sin comprobar
// aplicabilidad) hacia los tres anchos, la forma estática —que ni siquiera pasa por
// `revisaAnchosDeArgumentos` y por eso el rechazo tiene que salir de `expA`— y el caso con tres
// sobrecargas anchas (donde falla la resolución estricta y la laxa).
var booleanSondas = []struct{ nombre, fuente string }{
	{"receptor_long", `public class S { void f(long b){} void t(S s){ s.f(true); } }`},
	{"receptor_float", `public class S { void f(float b){} void t(S s){ s.f(true); } }`},
	{"receptor_double", `public class S { void f(double b){} void t(S s){ s.f(true); } }`},
	{"estatico_long", `public class S { static void f(long b){} static void t(){ f(true); } }`},
	{"tres_sobrecargas_anchas", `public class S { void f(long a){} void f(float a){} void f(double a){} void t(S s){ s.f(true); } }`},
}

// TestElBooleanNoEnsanchaEnElArgumento: javac 17 rechaza las cinco sondas y el emisor también, con
// un error que nombra el tipo (`boolean`, o `Z` en la lista de tipos llamados), lleva la línea y no
// es el descuadre `quedan -1 valores…`.
func TestElBooleanNoEnsanchaEnElArgumento(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	for _, c := range booleanSondas {
		t.Run(c.nombre, func(t *testing.T) {
			raiz := t.TempDir()
			pon(t, raiz, "S.java", c.fuente)
			salida, errJavac := exec.Command(javac, "--release", "8", "-d", raiz, filepath.Join(raiz, "S.java")).CombinedOutput()
			if errJavac == nil {
				t.Fatalf("javac 17 aceptó %s: el oráculo no es el esperado", c.nombre)
			}
			if !strings.Contains(string(salida), "cannot be converted") && !strings.Contains(string(salida), "incompatible types") {
				t.Logf("javac rechazó con: %s", salida)
			}

			_, errNuestro := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: c.fuente}}, jar)
			if errNuestro == nil {
				t.Fatalf("%s: el emisor aceptó `boolean` → ancho (regresión de af73a8a)", c.nombre)
			}
			if !strings.Contains(errNuestro.Error(), "linea ") {
				t.Errorf("%s: el rechazo no lleva línea: %v", c.nombre, errNuestro)
			}
			if !strings.Contains(errNuestro.Error(), "boolean") && !strings.Contains(errNuestro.Error(), "(Z)") {
				t.Errorf("%s: el rechazo no nombra el tipo del argumento: %v", c.nombre, errNuestro)
			}
			if strings.Contains(errNuestro.Error(), "quedan -1 valores") {
				t.Errorf("%s: el rechazo es el descuadre de pila, no un error claro: %v", c.nombre, errNuestro)
			}
			t.Logf("%s: %v", c.nombre, errNuestro)
		})
	}
}

// controlEnsanchaFuente: los cuatro tipos de una palabra que SÍ ensanchan. `int`, `byte`, `short` y
// `char` contra `long` y contra `double` en la misma clase, para que el control cubra las dos ramas
// de anchos de `expA`.
const controlEnsanchaFuente = `public class S {
    void f(long b) {}
    void g(double d) {}
    void t(S s, int n, byte by, short sh, char c) {
        s.f(n); s.f(by); s.f(sh); s.f(c);
        s.g(n); s.g(by); s.g(sh); s.g(c);
    }
}`

// TestElControlDeLosAnchosQueSiEnsanchan: `int`/`byte`/`short`/`char` siguen compilando y el dex sale
// byte a byte como el de javac 17. Es el control de que quitar `Z` no se llevó por delante los anchos
// de verdad.
func TestElControlDeLosAnchosQueSiEnsanchan(t *testing.T) {
	_, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/S.java", contenido: controlEnsanchaFuente}})
	comparaDexByteAByte(t, nuestro, suyo, "control_anchos")
}

// TestFalsificacionDelBooleanQueEnsancha: con `booleanEnsanchaEnArgumentos` encendido, `Z` vuelve a
// tratarse como `int` y `s.f(true)` / `f(true)` se aceptan otra vez —el defecto literal de af73a8a—,
// lo que prueba que el rechazo de arriba depende de la corrección y no de otra cosa.
func TestFalsificacionDelBooleanQueEnsancha(t *testing.T) {
	jar := jarDelSDKExigido(t)
	anterior := booleanEnsanchaEnArgumentos
	booleanEnsanchaEnArgumentos = true
	t.Cleanup(func() { booleanEnsanchaEnArgumentos = anterior })

	// el primer y el cuarto caso son los que un solo candidato deja pasar con el interruptor encendido
	for _, fuente := range []string{booleanSondas[0].fuente, booleanSondas[3].fuente} {
		if _, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: fuente}}, jar); err != nil {
			t.Fatalf("con `Z` de vuelta como `int` el programa debería aceptarse y no: %v", err)
		}
	}
}
