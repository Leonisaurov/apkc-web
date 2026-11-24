package main

// §253: el tipo cualificado en una declaración local. El que abrió este frente fue la línea 218 de
// `MainActivity.java` del zip de ApkShare:
//
//	src/share/apps/util/MainActivity.java: linea 218: se esperaba ";" y hay "builder"
//
// El tipo `AlertDialog.Builder` —dos nombres unidos por un punto, con el último como clase anidada de
// la primera— ya lo leía `nombrePunto`, y el emisor ya lo resolvía con `tipoAInterno`; lo que faltaba
// era que `pareceDeclaracion` viera los puntos: se paraba en el primero y la sentencia caía en la ruta
// de la expresión, donde `AlertDialog.Builder` es un acceso y el nombre de la variable sobra. El
// contexto es lo que separa las dos cosas: en Java `a.b c;` no es ninguna expresión, y un acceso a
// campo es receptor de una llamada o de una asignación, así que no llega a esta rama.
//
// El oráculo es javac 17 `--release 8 -g:lines,source`: los métodos y el dex entero byte a byte —esa
// mitad vive en `javaemisor_tipo_cualificado_local_paridad_test.go` con `//go:build !js`, porque bajo
// wasm no hay proceso que lanzar—, con
// las otras formas del tipo cualificado que `pareceDeclaracion` no gobierna —campo, parámetro,
// `new` e `instanceof`— en el mismo fuente, para que una resolución que se quedara en el primer
// segmento se vea.

import (
	"bytes"
	"strings"
	"testing"
)

// fuenteTipoCualificadoLocal trae la línea 218 del zip —verbatim, con el `import android.app.*;` del
// fichero real para que `AlertDialog` llegue por el import con estrella— y, en el mismo fuente, el
// tipo cualificado como campo, como parámetro de un método y como argumento de `instanceof`.
// `MainActivity extends Activity` es lo que hace que `this` sea un `Context` y el `new` sea Java.
const fuenteTipoCualificadoLocal = `package share.apps.util;

import android.app.*;
import android.os.*;

public class MainActivity extends Activity {
    private AlertDialog.Builder enEspera;

    public void show_loading_dialog() {
        AlertDialog.Builder builder = new AlertDialog.Builder(this);
        builder.setCancelable(false);
        enEspera = builder;
        AlertDialog dialogo = builder.create();
        dialogo.show();
    }

    public void reusa(android.app.AlertDialog.Builder otro) {
        otro.setCancelable(true);
    }

    public boolean esVista(Object v) {
        return v instanceof android.view.View;
    }

    public int cuenta(java.util.List<String> xs) {
        java.util.List<String> nombres = xs;
        return nombres.size();
    }
}
`

// fuenteTipoCualificadoLinea218 pone la declaración de la línea 218 del zip en la línea 218: las de
// relleno son comentarios dentro del método, que no dejan huella en la LineNumberTable.
func fuenteTipoCualificadoLinea218() string {
	lineas := []string{
		"package share.apps.util;",
		"",
		"import android.app.*;",
		"",
		"public class MainActivity extends Activity {",
		"    public void show_loading_dialog() {",
	}
	for len(lineas) < 217 {
		lineas = append(lineas, "        // relleno hasta la linea 218")
	}
	lineas = append(lineas,
		"        AlertDialog.Builder builder = new AlertDialog.Builder(this);",
		"    }",
		"}",
	)
	return strings.Join(lineas, "\n") + "\n"
}

// TestTipoCualificadoLocalApuntaAlTipoAnidado: la declaración local con el tipo cualificado emite la
// variable como el tipo anidado, `Landroid/app/AlertDialog$Builder;`, y no como el primer segmento. La
// paridad con javac 17 —métodos, LineNumberTable y dex entero— es
// `TestTipoCualificadoLocalContraJavac17`, en `javaemisor_tipo_cualificado_local_paridad_test.go`.
func TestTipoCualificadoLocalApuntaAlTipoAnidado(t *testing.T) {
	jar := jarDelSDKExigido(t)
	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteTipoCualificadoLocal},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el tipo cualificado de una declaración local: %v", err)
	}
	emitida := porInterno(nuestras, "share/apps/util/MainActivity")
	if emitida == nil {
		t.Fatalf("no salió MainActivity (%s)", nombresDe(nuestras))
	}
	// la variable es del tipo anidado, con el `$` con el que javac lo nombra: no el primer segmento
	if !bytes.Contains(emitida.datos, []byte("Landroid/app/AlertDialog$Builder;")) {
		t.Errorf("la declaración no apunta a android/app/AlertDialog$Builder: %s", volcadoDeClases(nuestras))
	}
}

// TestTipoCualificadoLocalFalsificacion: con el interruptor apagado, la línea 218 del usuario tiene
// que volver a fallar con su mensaje literal, en la misma línea y con el prefijo del fichero que pone
// `compilarProyectoJava`. Encendido, el fuente entero compila: no queda ninguna frontera detrás.
func TestTipoCualificadoLocalFalsificacion(t *testing.T) {
	const literal = "src/share/apps/util/MainActivity.java: linea 218: se esperaba \";\" y hay \"builder\""
	jar := jarDelSDKExigido(t)
	fuente := fuenteTipoCualificadoLinea218()
	if lineas := strings.Split(fuente, "\n"); !strings.Contains(lineas[217], "AlertDialog.Builder builder") {
		t.Fatalf("la línea 218 del fuente de prueba es %q y no la del zip", lineas[217])
	}
	fuentes := []fuenteJava{{nombre: "src/share/apps/util/MainActivity.java", contenido: fuente}}

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con el tipo cualificado encendido el fuente no compila: %v", err)
	}

	anterior := leeTipoCualificadoDeclaracion
	leeTipoCualificadoDeclaracion = false
	t.Cleanup(func() { leeTipoCualificadoDeclaracion = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if err.Error() != literal {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario %q", err, literal)
	}
	t.Logf("sin la ruta: %v", err)
}
