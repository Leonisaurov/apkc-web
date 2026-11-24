//go:build !js

package main

// La paridad con javac 17 del bloque real de ApkShare (§ del frente de `af73a8a`): el `afterTextChanged`
// de la anónima instrucción a instrucción, su `max_stack` y el dex byte a byte de `Retardo$1`. Bajo
// `GOOS=js` no hay proceso que lanzar, así que el fichero lleva `//go:build !js`; el caso que solo mira
// lo emitido vive en `javaemisor_ancho_argumento_test.go`, sin tag. Esta prueba **no se salta**: sin
// javac 17 falla (ver `dependencias_test.go:5-8`).

import (
	"testing"
)

// TestElPostDelayedDeApkShareLlevaSuI2L: el bloque real compila —el `-1` que paraba ApkShare ya no
// está— y el `afterTextChanged` de la anónima sale instrucción a instrucción como el de javac 17,
// con su `max_stack` y sus huecos, y con el `i2l` de la 105 justo delante del `invokevirtual` de
// `android.os.Handler.postDelayed:(Ljava/lang/Runnable;J)Z`. El constructor de la anónima ya no lleva
// `MethodParameters`, así que `Retardo$1` dexa byte a byte; el dex entero sigue sin compararse porque
// `Retardo$1$1` llama a `Editable.toString()` y ahí nuestro emisor elige `invoke-static` donde javac 17
// escribe `invoke-virtual` (diferencia de la resolución del método, no del constructor ni de esta ruta).
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
	visto := false
	for i, in := range instrucciones {
		if in.name != "invokevirtual" {
			continue
		}
		if i == 0 || instrucciones[i-1].name != "i2l" {
			t.Fatalf("el `invokevirtual` de la pc %d no lleva el `i2l` detrás: %v", in.pc, instrucciones)
		}
		t.Logf("`postDelayed` en la pc %d, con el `i2l` de la pc %d delante", in.pc, instrucciones[i-1].pc)
		visto = true
		break
	}
	if !visto {
		t.Fatalf("no hay ningún `invokevirtual` en `afterTextChanged`: %v", instrucciones)
	}
	// y la anónima —la que lleva el `postDelayed`— dexa byte a byte el de javac 17: su constructor ya
	// no arrastra el `MethodParameters` que se dejaba fuera antes
	nuestraAnonima := dexDeUnaClase(t, claseEmitidaPorNombre(t, nuestras, "share/apps/util/Retardo$1"))
	suyaAnonima := dexDeUnaClaseLeida(t, claseLeidaPorNombre(t, deJavac, "Lshare/apps/util/Retardo$1;"))
	comparaDexByteAByte(t, nuestraAnonima, suyaAnonima, volcadoDeClases(nuestras))
}

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
