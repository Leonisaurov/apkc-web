package main

// La última divergencia de paridad de ApkShare: en `MainActivity.openAppDetailsDialog` nuestro dex de
// la clase salía con cuatro bytes menos que el de javac 17 y `javap -v` lo dejaba en una sola fila de
// la LineNumberTable que nos faltaba, la del `} else {` de la línea 159 sobre el `goto` del pc 155:
//
//	7a8
//	> line 159: 155
//
// La causa no es el `else` ni la lambda: es el cierre del bloque del `then`. javac hace
// `code.statBegin(tree.endpos)` antes de `code.endScopes(limit)` en `visitBlock` (`Gen.visitBlock`,
// JDK 17), y `endScopes` → `endScope(adr)` llama a `curCP()` cuando el local tiene abierto su rango
// vivo; `curCP` vuelca el `statBegin` pendiente (`Code.curCP` → `markStatBegin`). El `addLineNumber`
// cae en el pc donde va a empezar el `goto` de salida, y esa fila sobrevive porque ninguna otra
// sentencia empieza en ese pc. El rango vivo solo se abre al **guardar** el local (`Code.setDefined`),
// y por eso la fila depende de que el bloque declare un local con valor: con una sola sentencia, con
// una llamada —aunque lleve una lambda de argumento—, con `break` (no queda `goto`) o sin `else` no
// hay fila; con un local sin inicializador y sin asignar tampoco. Todo eso está medido abajo contra
// javac 17 `--release 8 -g:lines,source`, con el dex entero byte a byte.
//
// La segunda mitad de la divergencia era el sitio del implementador de la lambda: javac escribe
// `lambda$openAppDetailsDialog$0` **entre** los puentes covariantes y los `access$NNN`, que es el
// orden de sus pasadas (TransTypes crea el puente, LambdaToMethod la lambda y Lower el `access$`). Sin
// eso el `code_item` del dex salía en otro sitio y el dex por clase no coincidía ni con la fila puesta.

import (
	"strings"
	"testing"
)

// fuenteElseGoto: las cinco formas de la fila, más dos controles. `m` es la forma de ApkShare —un local
// cuyo inicializador lleva la lambda dentro de un constructor—; `m2` una sola sentencia; `m3` una
// llamada con la lambda de argumento y sin local; `m4` el `break`, que deja el `then` sin `goto`;
// `m4` el `break` (no queda `goto` de salida) y `m5` sin `else`, que tampoco lo lleva; `m6` un local declarado y nunca guardado, que javac deja sin
// rango vivo; y `m7` el control por el otro lado, un local con inicializador y sin lambda, que también
// anota la fila.
const fuenteElseGoto = `package p;

import java.util.List;
import java.util.function.Consumer;

class H {
    H(Consumer<String> c) {
    }

    void b6(Consumer<String> c) {
    }
}

class E {
    void b(int n) {
    }

    void m(List<String> l) {
        if (l.size() > 1) {
            H h = new H(s -> b(1));
            b(2);
        } else {
            b(3);
        }
        b(4);
    }

    void m2(List<String> l) {
        if (l.size() > 1) {
            b(2);
        } else {
            b(3);
        }
        b(4);
    }

    void m3(List<String> l) {
        if (l.size() > 1) {
            new H(s -> b(1));
        } else {
            b(3);
        }
        b(4);
    }

    void m4(List<String> l) {
        for (int i = 0; i < 3; i++) {
            if (l.size() > 1) {
                int y = i;
                break;
            }
        }
        b(4);
    }

    void m5(List<String> l) {
        if (l.size() > 1) {
            int y = 1;
            b(y);
        }
        b(4);
    }

    void m6(List<String> l) {
        if (l.size() > 1) {
            int y;
            b(2);
        } else {
            b(3);
        }
        b(4);
    }

    void m7(List<String> l) {
        if (l.size() > 1) {
            int y = 1;
            b(y);
        } else {
            b(3);
        }
        b(4);
    }
}
`

// fuenteMainActivityElse: la forma de la línea 155 del zip de ApkShare, con la declaración del
// `ApkAdapter` recortada a su constructor (el del zip toma `(Context, List<Apk>, Consumer<Apk>)`, que
// es lo que decide el tipo de la lambda) y su `else` de dos sentencias. Es el caso que abrió el
// frente; el recuento de las 21 clases del proyecto entero se mide con el `apkc` compilado y el
// `android.jar` (ver el informe), que aquí no cabe.
const fuenteMainActivityElse = `package share.apps.util;

import android.app.Activity;
import android.view.View;

import java.util.List;

public class MainActivity extends Activity {
    public void openAppDetailsDialog(List<Apk> apks, View apks_view, View shareBtn) {
        if (apks.size() > 1) {
            ApkAdapter adapter = new ApkAdapter(this, apks, apk -> shareApp(apk));
            apks_view.setTag(adapter);
            apks_view.setVisibility(0);
        } else {
            apks_view.setVisibility(0);
            shareBtn.setVisibility(0);
        }
        apks_view.setVisibility(1);
    }

    public void shareApp(Apk apk) {
        apk.toString();
    }
}
`

const fuenteApkAdapterElse = `package share.apps.util;

import android.content.Context;

import java.util.List;
import java.util.function.Consumer;

public class ApkAdapter {
    private final Consumer<Apk> fn;

    public ApkAdapter(Context context, List<Apk> apks, Consumer<Apk> fn) {
        this.fn = fn;
    }
}
`

const fuenteApkElse = `package share.apps.util;

public class Apk {
    public final String filePath;

    public Apk(String filePath) {
        this.filePath = filePath;
    }

    @Override
    public String toString() {
        return filePath;
    }
}
`

var fuentesMainActivityElse = []fuenteJava{
	{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteMainActivityElse},
	{nombre: "src/share/apps/util/ApkAdapter.java", contenido: fuenteApkAdapterElse},
	{nombre: "src/share/apps/util/Apk.java", contenido: fuenteApkElse},
}

// filaDeCierre: la fila de la LineNumberTable con esa línea, si está.
func filaDeCierre(m classMethod, linea int) (classLine, bool) {
	for _, l := range m.Lines {
		if l.line == linea {
			return l, true
		}
	}
	return classLine{}, false
}

// TestLaLineaDelElseVaEnElGotoDelThen: las siete formas contra javac 17 `--release 8 -g:lines,source`,
// con el dex entero byte a byte. La fila del cierre del `then` (la del `} else {`) solo sale en `m` —el
// local con la lambda dentro del constructor, que es la forma de la línea 156 del zip— y en `m7`, el
// local con inicializador; y en los dos cae sobre el `goto` de salida. `m2`, `m3`, `m4`, `m5` y `m6` no
// la llevan, que es exactamente lo que javac 17 escribe.
func TestLaLineaDelElseVaEnElGotoDelThen(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/E.java", contenido: fuenteElseGoto}})

	// la línea del `}` del `then` de cada método, contada en el fuente de arriba, y su pc medido.
	casos := []struct {
		metodo string
		cierre int
		pc     int
		anota  bool
	}{
		{"m", 22, 29, true},
		{"m2", 31, 0, false},
		{"m3", 40, 0, false},
		{"m4", 51, 0, false},
		{"m5", 60, 0, false},
		{"m6", 68, 0, false},
		{"m7", 78, 17, true},
	}
	for _, c := range casos {
		m := metodoDeClaseEmitida(t, nuestras, "p/E", c.metodo, "")
		fila, hay := filaDeCierre(m, c.cierre)
		if hay != c.anota {
			t.Errorf("%s: fila con la línea %d = %v, medido %v (tabla: %s)", c.metodo, c.cierre, hay, c.anota, filasDeLinea(m))
			continue
		}
		if !hay {
			continue
		}
		if fila.pc != c.pc {
			t.Errorf("%s: la fila del cierre está en el pc %d y javac la pone en el %d (%s)", c.metodo, fila.pc, c.pc, filasDeLinea(m))
		}
		if fila.pc >= len(m.Code) || m.Code[fila.pc] != opGOTO {
			t.Errorf("%s: la fila de la línea %d no cae sobre un `goto` (%s)", c.metodo, c.cierre, filasDeLinea(m))
		}
	}

	// el oráculo estricto: mismos `max_stack`/`locals`, mismas filas método a método y el dex entero
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))
}

// TestElDexDeMainActivityConElseEsElDeJavac17: la forma de la línea 156 del zip —local con la lambda
// dentro del constructor y `else` de dos sentencias— con el dex byte a byte. Es el caso que dejó
// `MainActivity` como única clase divergente de las 21.
func TestElDexDeMainActivityConElseEsElDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, fuentesMainActivityElse)

	const desc = "(Ljava/util/List;Landroid/view/View;Landroid/view/View;)V"
	metodo := metodoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity", "openAppDetailsDialog", desc)
	lineaDelCierre, pcDelGoto := 0, -1
	for _, l := range metodo.Lines {
		if l.pc < len(metodo.Code) && metodo.Code[l.pc] == opGOTO {
			lineaDelCierre, pcDelGoto = l.line, l.pc
		}
	}
	if lineaDelCierre == 0 {
		t.Fatalf("`openAppDetailsDialog` no lleva la fila del cierre en el `goto` (%s)", filasDeLinea(metodo))
	}
	t.Logf("la fila del cierre del `then` es la %d, sobre el `goto` del pc %d (%s)", lineaDelCierre, pcDelGoto, filasDeLinea(metodo))

	// y el implementador de la lambda sale entre lo declarado y los `access$`, que es el sitio de javac
	var implementador string
	for _, c := range nuestras {
		if c.interno != "share/apps/util/MainActivity" {
			continue
		}
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("MainActivity no se relee: %v", err)
		}
		for _, mm := range cf.Methods {
			if strings.HasPrefix(mm.Name, "lambda$") {
				implementador = mm.Name
			}
		}
	}
	if implementador == "" {
		t.Fatalf("no salió el implementador de la lambda de `openAppDetailsDialog`: %s", volcadoDeClases(nuestras))
	}
	t.Logf("implementador: %s", implementador)

	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))
}

// TestLaFilaDelElseSeVeEnElDiffDeJavap: la fila que faltaba, con la forma del `diff` que dejó medido
// el frente. Con el emisor como está, la tabla de `MainActivity` y la de javac coinciden; con el
// interruptor apagado, el `diff` de las dos tablas es exactamente una línea `> line N: P`, que es la
// forma que tenía el `7a8 > line 159: 155` del zip (el número de línea del `diff` depende del pool; la
// fila, no).
func TestLaFilaDelElseSeVeEnElDiffDeJavap(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	for _, f := range fuentesMainActivityElse {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentesMainActivityElse)
	const desc = "(Ljava/util/List;Landroid/view/View;Landroid/view/View;)V"
	metodoJavac := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/MainActivity;", "openAppDetailsDialog", desc)

	nuestras, err := compilarProyectoJava(fuentesMainActivityElse, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}
	nuestra := metodoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity", "openAppDetailsDialog", desc)
	if diff := diffDeLineas(nuestra, metodoJavac); diff != "" {
		t.Fatalf("la tabla de líneas ya no es la de javac:\n%s", diff)
	}

	anterior := anotaLineaDelElse
	anotaLineaDelElse = false
	t.Cleanup(func() { anotaLineaDelElse = anterior })

	apagadas, err := compilarProyectoJava(fuentesMainActivityElse, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso con el interruptor apagado: %v", err)
	}
	sinFila := metodoDeClaseEmitida(t, apagadas, "share/apps/util/MainActivity", "openAppDetailsDialog", desc)
	diff := diffDeLineas(sinFila, metodoJavac)
	if diff == "" {
		t.Fatal("con el interruptor apagado la fila del cierre sigue en la tabla")
	}
	lineas := strings.Split(strings.TrimSpace(diff), "\n")
	if len(lineas) != 1 || !strings.HasPrefix(lineas[0], "> line ") {
		t.Fatalf("el diff con el interruptor apagado no es una sola fila `> line N: P`:\n%s", diff)
	}
	// la fila del `} else {` de este `MainActivity`: el `goto` de salida del `then`, en el pc medido
	if lineas[0] != "> line 14: 38" {
		t.Fatalf("el diff no es la fila del `} else {` (línea 14, pc 38) del caso:\n%s", diff)
	}
	t.Logf("con el interruptor apagado el diff de la LineNumberTable es:\n%s", diff)
}

// diffDeLineas: el `diff` de las LineNumberTable como lo imprime `javap`: las filas que están en javac
// y no en la nuestra, con el `>` delante. Es la comprobación de la fila que faltaba.
func diffDeLineas(nuestra, suya classMethod) string {
	enLaNuestra := map[classLine]bool{}
	for _, l := range nuestra.Lines {
		enLaNuestra[l] = true
	}
	faltan := []string{}
	for _, l := range suya.Lines {
		if !enLaNuestra[l] {
			faltan = append(faltan, "> line "+itoa(l.line)+": "+itoa(l.pc))
		}
	}
	return strings.Join(faltan, "\n")
}
