//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_llamada_argumento_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_llamada_argumento_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestLlamadaEncadenadaComoArgumentoContraJavac17: la vuelta de `get()` se estima sin emitir el
// receptor y el dex entero sale igual al de javac 17, sobrecarga elegida incluida.
func TestLlamadaEncadenadaComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
// `app`. Se comparan las instrucciones de `onClick` y el dex entero, sin normalizar desde d123135
// (antes se normalizaba el constructor de la anónima, §100).
func TestLlamadaEncadenadaEnAnonimaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la anónima no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestLlamadaEncadenadaAnchaContraJavac17: la llamada encadenada cuyo argumento además se ensancha
// (`s.f(s.n())`). Mientras el tipo de `s.n()` no se sabía sin emitir el receptor, la sobrecarga se
// resolvía después de bajar el argumento y el `i2l` se perdía: la guarda de
// `javaemisor_ancho_argumento_test.go` lo rechazaba. Con la estimación nueva se escribe el `i2l` donde
// lo escribe javac.
func TestLlamadaEncadenadaAnchaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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
