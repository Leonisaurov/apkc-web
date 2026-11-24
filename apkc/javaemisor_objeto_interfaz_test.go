package main

import (
	"bytes"
	"testing"
)

// Este fichero cierra la cuarta causa de paridad de ApkShare: `MainActivity$3$1` y
// `MainActivity$4` llaman a `toString()` sobre un receptor de tipo `android.text.Editable`, que es
// una interfaz, y nosotros lo emitíamos como `invokeinterface android/text/Editable.toString`
// (5 bytes) donde javac 17 `--release 8 -g:lines,source` escribe `invokevirtual
// java/lang/Object.toString` (3 bytes).
//
// La regla de javac, medida con `javap -c` sobre la sonda de abajo:
//
//	receptor estático          declaración resuelta                opcode
//	CharSequence               CharSequence (la declara él mismo)  invokeinterface CharSequence.toString
//	CharSequence               java/lang/Object (hashCode/equals)  invokevirtual java/lang/Object.hashCode
//	Editable (hereda de
//	CharSequence)              java/lang/Object                    invokevirtual java/lang/Object.toString
//	SinToString (propia)       java/lang/Object                    invokevirtual java/lang/Object.toString
//	ConToString (redeclara)    ConToString                         invokeinterface ConToString.toString
//
// No basta con que la interfaz herede la declaración: `Editable extends CharSequence` y
// `CharSequence` declara `toString`, y javac aun así va a `Object`. Solo se queda en la interfaz
// cuando el **tipo estático del receptor** declara el método él mismo (`ConToString`, `CharSequence`).

// fuenteObjetoEnInterfaz es la sonda de la tabla: los tres métodos de `Object` sobre una interfaz
// del SDK que sí los declara (`CharSequence`), una que hereda `toString` (`Editable`), una propia que
// no lo declara, una propia que lo redeclara y el caso trivial de `Object`.
const fuenteObjetoEnInterfaz = `package p;

import android.text.Editable;

interface SinToString {
    String propio();
}

interface ConToString extends SinToString {
    @Override
    String toString();
}

class Objetos {
    String porCharSequence(CharSequence s) {
        return s.toString();
    }

    int porCharSequenceHash(CharSequence s) {
        return s.hashCode();
    }

    boolean porCharSequenceEquals(CharSequence s) {
        return s.equals(s);
    }

    String porEditable(Editable e) {
        return e.toString();
    }

    String porSinToString(SinToString s) {
        return s.toString();
    }

    String porConToString(ConToString c) {
        return c.toString();
    }

    String porObjeto(Object o) {
        return o.toString();
    }
}
`

// fuenteCapturaEditable es el mínimo de `MainActivity$3$1` y `MainActivity$4` del zip: el primero
// llama a `toString()` sobre la `Editable` que captura en `val$s`; el segundo, sobre lo que devuelve
// `EditText.getText()`, que también es `Editable`.
const fuenteCapturaEditable = `package p;

import android.text.Editable;
import android.widget.EditText;

class Actividad {
    private Runnable tarea;

    void busca(EditText searchQuery) {
        final Editable s = searchQuery.getText();
        tarea = new Runnable() {
            public void run() {
                s.toString();
            }
        };
        searchQuery.getText().toString();
    }
}
`

// TestElToStringSobreUnaInterfazEsElDeJavac17: el dex de la sonda, byte a byte contra javac 17.
func TestElToStringSobreUnaInterfazEsElDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{
		{nombre: "src/p/Objetos.java", contenido: fuenteObjetoEnInterfaz},
	})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	// y la forma de cada llamada en lo emitido: `invokeinterface` solo donde el tipo estático del
	// receptor declara el método él mismo
	objetos := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porCharSequence", "(Ljava/lang/CharSequence;)Ljava/lang/String;")
	if n := cuentaOpcode(t, objetos.Code, opInvointf); n != 1 {
		t.Fatalf("`porCharSequence` lleva %d `invokeinterface` y javac escribe uno (CharSequence.toString)", n)
	}
	hash := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porCharSequenceHash", "(Ljava/lang/CharSequence;)I")
	if n := cuentaOpcode(t, hash.Code, opInvointf); n != 0 {
		t.Fatalf("`porCharSequenceHash` lleva %d `invokeinterface` y javac va a `Object.hashCode`", n)
	}
	igual := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porCharSequenceEquals", "(Ljava/lang/CharSequence;)Z")
	if n := cuentaOpcode(t, igual.Code, opInvointf); n != 0 {
		t.Fatalf("`porCharSequenceEquals` lleva %d `invokeinterface` y javac va a `Object.equals`", n)
	}
	editable := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porEditable", "(Landroid/text/Editable;)Ljava/lang/String;")
	if n := cuentaOpcode(t, editable.Code, opInvointf); n != 0 {
		t.Fatalf("`porEditable` lleva %d `invokeinterface`: `Editable` hereda `toString` y javac va a `Object`", n)
	}
	sin := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porSinToString", "(Lp/SinToString;)Ljava/lang/String;")
	if n := cuentaOpcode(t, sin.Code, opInvointf); n != 0 {
		t.Fatalf("`porSinToString` lleva %d `invokeinterface`: la interfaz no declara `toString` y javac va a `Object`", n)
	}
	// el control: la que sí lo redeclara sigue yendo a la interfaz
	con := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porConToString", "(Lp/ConToString;)Ljava/lang/String;")
	if n := cuentaOpcode(t, con.Code, opInvointf); n != 1 {
		t.Fatalf("`porConToString` lleva %d `invokeinterface` y javac escribe uno (ConToString.toString)", n)
	}
}

// TestElToStringDeUnaEditableCapturadaEsElDeJavac17: el caso real de `MainActivity$3$1` (la `Editable`
// capturada en la anónima) y el de `MainActivity$4` (`getText().toString()`), byte a byte contra javac.
func TestElToStringDeUnaEditableCapturadaEsElDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{
		{nombre: "src/p/Actividad.java", contenido: fuenteCapturaEditable},
	})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	anonima := metodoDeClaseEmitida(t, nuestras, "p/Actividad$1", "run", "()V")
	if n := cuentaOpcode(t, anonima.Code, opInvointf); n != 0 {
		t.Fatalf("la anónima lleva %d `invokeinterface` y javac escribe `invokevirtual Object.toString`", n)
	}
	busca := metodoDeClaseEmitida(t, nuestras, "p/Actividad", "busca", "(Landroid/widget/EditText;)V")
	if n := cuentaOpcode(t, busca.Code, opInvointf); n != 0 {
		t.Fatalf("`busca` lleva %d `invokeinterface` y javac escribe `invokevirtual Object.toString`", n)
	}
	t.Logf("la anónima con la captura y `getText().toString()` byte a byte los de javac: %d bytes de dex", len(nuestro))
}

// TestFalsificacionDelToStringSobreInterfaz: con el interruptor apagado vuelve la forma vieja
// (`invokeinterface p/SinToString.toString` y `invokeinterface android/text/Editable.toString`) y el
// dex deja de ser el de javac. El control que sí redeclara no cambia: su llamada nunca fue a `Object`.
func TestFalsificacionDelToStringSobreInterfaz(t *testing.T) {
	anterior := objetoEnInterfazVirtual
	objetoEnInterfazVirtual = false
	t.Cleanup(func() { objetoEnInterfazVirtual = anterior })

	fuentes := []fuenteJava{{nombre: "src/p/Objetos.java", contenido: fuenteObjetoEnInterfaz}}
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, fuentes)
	if bytes.Equal(nuestro, suyo) {
		t.Fatalf("con el interruptor apagado el dex de la sonda sigue siendo el de javac")
	}

	editable := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porEditable", "(Landroid/text/Editable;)Ljava/lang/String;")
	if n := cuentaOpcode(t, editable.Code, opInvointf); n != 1 {
		t.Fatalf("con el interruptor apagado `porEditable` lleva %d `invokeinterface` y se esperaba la forma vieja (1)", n)
	}
	sin := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porSinToString", "(Lp/SinToString;)Ljava/lang/String;")
	if n := cuentaOpcode(t, sin.Code, opInvointf); n != 1 {
		t.Fatalf("con el interruptor apagado `porSinToString` lleva %d `invokeinterface` y se esperaba la forma vieja (1)", n)
	}
	con := metodoDeClaseEmitida(t, nuestras, "p/Objetos", "porConToString", "(Lp/ConToString;)Ljava/lang/String;")
	if n := cuentaOpcode(t, con.Code, opInvointf); n != 1 {
		t.Fatalf("el control `porConToString` lleva %d `invokeinterface` con el interruptor apagado y javac escribe uno", n)
	}
	primero := -1
	for i := 0; i < len(nuestro) && i < len(suyo); i++ {
		if nuestro[i] != suyo[i] {
			primero = i
			break
		}
	}
	t.Logf("con la forma vieja: nuestro dex %d bytes, javac %d, primer byte distinto en %d", len(nuestro), len(suyo), primero)
}
