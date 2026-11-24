package main

// Un `static final String` con `ConstantValue` del classpath es una constante de compilación: javac la
// sustituye por su literal, no deja ni `getstatic` ni el Fieldref, y el valor tiene que viajar como
// cadena. El caso es `MainActivity.java:198` de ApkShare, `new Intent(Intent.ACTION_SEND)`, que paraba
// con «al estimar el tipo de Intent.ACTION_SEND, la constante android/content/Intent.ACTION_SEND no es
// un entero: doblarla no esta medida». El oráculo es el de siempre: javac 17 con
// `--release 8 -g:lines,source` por el mismo dexer, y el dex entero byte a byte.
//
// El control va en el mismo fuente: `android.os.Build.MODEL` es `static final String` **sin**
// `ConstantValue`, así que sigue siendo un `getstatic`, y `Intent.FLAG_GRANT_READ_URI_PERMISSION` es un
// `static final int` que se sigue plegando como antes (`iconst_1`).

import (
	"strings"
	"testing"
)

const fuenteCadenaEstatica = `package p;

import android.content.Intent;
import android.os.Build;

public class Envio {
    public Intent comparte() {
        Intent i = new Intent(Intent.ACTION_SEND);
        i.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        return i;
    }

    public String modelo() {
        return Build.MODEL;
    }
}
`

// TestCadenaEstaticaDelAndroidJarContraJavac17: `Intent.ACTION_SEND` sale como `ldc` de la cadena y el
// dex entero coincide con el de javac. `Build.MODEL`, sin `ConstantValue`, sigue emitiendo `getstatic`.
func TestCadenaEstaticaDelAndroidJarContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
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

// TestCadenaEstaticaFalsificacion: apagado el interruptor, el fuente vuelve a parar con el error
// literal que leyó el usuario, nombrando el fuente y la línea del `new Intent(Intent.ACTION_SEND)`.
func TestCadenaEstaticaFalsificacion(t *testing.T) {
	const literal = "la constante android/content/Intent.ACTION_SEND no es un entero: doblarla no esta medida"
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/p/Envio.java", contenido: fuenteCadenaEstatica}}

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con el interruptor encendido el fuente no compila: %v", err)
	}

	anterior := pliegaCadenaEstatica
	pliegaCadenaEstatica = false
	t.Cleanup(func() { pliegaCadenaEstatica = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con el interruptor apagado el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con el interruptor apagado el rechazo es %q, y el del usuario contiene %q", err, literal)
	}
	if !strings.Contains(err.Error(), "al estimar el tipo de Intent.ACTION_SEND") {
		t.Fatalf("el rechazo no pasa por la estimación del argumento de `Intent(String)`: %q", err)
	}
	if !strings.HasPrefix(err.Error(), "Envio.java: ") || !strings.Contains(err.Error(), "linea 8") {
		t.Fatalf("el rechazo no nombra el fuente y la línea del `new Intent`: %q", err)
	}
	t.Logf("sin el interruptor: %v", err)
}
