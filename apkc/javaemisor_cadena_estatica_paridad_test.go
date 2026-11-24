//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_cadena_estatica_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_cadena_estatica_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestCadenaEstaticaDelAndroidJarContraJavac17: `Intent.ACTION_SEND` sale como `ldc` de la cadena y el
// dex entero coincide con el de javac. `Build.MODEL`, sin `ConstantValue`, sigue emitiendo `getstatic`.
func TestCadenaEstaticaDelAndroidJarContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Envio.java", contenido: fuenteCadenaEstatica}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el `static final String` del android.jar: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "p/Envio" {
		t.Fatalf("salieron %d clases (%s), y javac deja una", len(nuestras), nombresDe(nuestras))
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Envio.java", fuenteCadenaEstatica)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// `comparte()`: un solo `ldc`, el de la cadena, y ni un `getstatic` en el cuerpo. Si el campo
	// viajara como acceso, el opcode sería 0xb2 y el valor no estaría en el pool como String.
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Envio", "comparte", "()Landroid/content/Intent;")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Envio;", "comparte", "()Landroid/content/Intent;")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("comparte()Landroid/content/Intent; no es el de javac: %v", err)
	}
	nuestroPool := poolDeEmitida(t, nuestras, "p/Envio")
	instr, err := walkJvmCode(nuestro.Code)
	if err != nil {
		t.Fatalf("no se desarma nuestro comparte(): %v", err)
	}
	ldc, getstatic := 0, 0
	for _, in := range instr {
		switch in.op {
		case opLdc:
			ldc++
			if len(in.ops) != 1 {
				t.Fatalf("`ldc` con %d operandos en comparte()", len(in.ops))
			}
			if got := nuestroPool.constantOf(int(in.ops[0])); got != "sandroid.intent.action.SEND" {
				t.Errorf("el `ldc` de comparte() carga %q y se esperaba la cadena `android.intent.action.SEND`", got)
			}
		case opGetstatic:
			getstatic++
		}
	}
	if ldc != 1 || getstatic != 0 {
		t.Errorf("comparte() tiene %d `ldc` y %d `getstatic`; se esperaba 1 y 0: `Intent.ACTION_SEND` se dobla", ldc, getstatic)
	}

	// `modelo()`: `Build.MODEL` no trae `ConstantValue` y sigue siendo un `getstatic`.
	nuestroModelo := metodoDeClaseEmitida(t, nuestras, "p/Envio", "modelo", "()Ljava/lang/String;")
	suyoModelo := metodoDeClaseLeida(t, deJavac, "Lp/Envio;", "modelo", "()Ljava/lang/String;")
	if err := comparaInstrucciones(nuestroModelo.Code, suyoModelo.Code); err != nil {
		t.Fatalf("modelo()Ljava/lang/String; no es el de javac: %v", err)
	}
	instrModelo, err := walkJvmCode(nuestroModelo.Code)
	if err != nil {
		t.Fatalf("no se desarma nuestro modelo(): %v", err)
	}
	ldcModelo, getstaticModelo := 0, 0
	for _, in := range instrModelo {
		switch in.op {
		case opLdc:
			ldcModelo++
		case opGetstatic:
			getstaticModelo++
		}
	}
	if ldcModelo != 0 || getstaticModelo != 1 {
		t.Errorf("modelo() tiene %d `ldc` y %d `getstatic`; `Build.MODEL` no tiene ConstantValue y se lee con getstatic", ldcModelo, getstaticModelo)
	}

	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de Envio no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
}
