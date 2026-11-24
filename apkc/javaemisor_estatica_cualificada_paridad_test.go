//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_estatica_cualificada_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_estatica_cualificada_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestEstaticaCualificadaComoArgumentoContraJavac17: la línea 212 con receptor implícito. El dex entero
// sale igual al de javac 17, con un solo `invokestatic` (la llamada a `createChooser`) y un solo
// `invokevirtual` (el `startActivity`).
func TestEstaticaCualificadaComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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

// TestEstaticaCualificadaEnAnonimaContraJavac17: la línea 212 tal cual, dentro de la anónima que captura
// el `Intent` y resuelve `startActivity` en la Activity exterior. Se comparan las instrucciones de `run`
// y el dex entero, sin normalizar desde d123135 (antes se normalizaba el constructor de la anónima, §100).
func TestEstaticaCualificadaEnAnonimaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la anónima no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestEstaticaCualificadaTipoIncorrectoSeRechaza: el control negativo. Un argumento que no encaja en la
// firma de `createChooser` no se acepta: la estimación resuelve la candidata de verdad y devuelve su
// motivo, en vez de estimar el tipo por el nombre del método. javac 17 lo rechaza igual.
func TestEstaticaCualificadaTipoIncorrectoSeRechaza(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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
