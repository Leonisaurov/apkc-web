package main

// La concatenación como argumento de un constructor: la línea 179 de
// `src/share/apps/util/MainActivity.java` del zip de ApkShare,
//
//	File toApkFile = new File(getApplicationContext().getExternalFilesDir(null), apk.name + ".apk");
//
// El segundo argumento de `File(File,String)` es un `joperador` de `+`, y `tipoEstimado` no tenía el
// caso: la sobrecarga no se podía resolver y el `check` paraba con
//
//	[build] MainActivity.java: linea 175: linea 176: linea 179: linea 179, columna 103: el tipo de una
//	expresion main.joperador no se puede estimar sin emitirla
//
// La columna 103 es el `+`. En Java el `+` es concatenación en cuanto uno de sus operandos es
// `String` (JLS 15.18.1) y el resultado es siempre `String`, así que se estima sin emitir nada; la
// emisión sigue yendo a `exp` → `concatenar`, que es la ruta ya medida (StringBuilder) y no cambia el
// orden de evaluación. El oráculo es el de siempre: javac 17 con `--release 8 -g:lines,source` por el
// mismo dexer, las instrucciones de `run()V` y el dex entero. La única diferencia conocida es el
// atributo del constructor de la anónima (§100), que se normalizaba; ya no, desde d123135.

import (
	"strings"
	"testing"
)

// fuenteConcatEnFile: las líneas 171-179 reales, con la anónima que captura `apk`, y un `Apk` mínimo
// con los dos campos que la línea usa. Los dos `exists()` del final son para que los dos `File` no
// queden sin usar: la asignación de la 178 no se puede quitar sin dejar de ser la línea 179 del fuente.
const fuenteConcatEnFile = `package p;

import android.app.Activity;

import java.io.File;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

class Apk {
    public final String name;
    public final String filePath;

    Apk(String name, String filePath) {
        this.name = name;
        this.filePath = filePath;
    }
}

public class MainActivity extends Activity {
    public void shareApp(Apk apk) {
        ExecutorService package_fetch = Executors.newSingleThreadExecutor();
        package_fetch.execute(new Runnable() {
            @Override
            public void run() {
                File apkFile = new File(apk.filePath);
                File toApkFile = new File(getApplicationContext().getExternalFilesDir(null), apk.name + ".apk");
                apkFile.exists();
                toApkFile.exists();
            }
        });
    }
}
`

// fuenteConcatStringEntero: el control que sí es concatenación. `int + String` es el orden que javac
// pliega igual (el entero se appendea con `append(I)`), y el resultado sigue siendo `String`.
const fuenteConcatStringEntero = `package p;

import android.app.Activity;

import java.io.File;

public class MainActivity extends Activity {
    public File conEntero(int i) {
        return new File(getApplicationContext().getExternalFilesDir(null), i + ".apk");
    }
}
`

// fuenteCuentaEnFile: el control negativo. `1 + 2` es una cuenta entera y `File(File,int)` no existe,
// así que javac rechaza el fuente; el emisor tampoco puede inventarse `String`: tiene que seguir
// devolviendo el rechazo explícito de esta ruta.
const fuenteCuentaEnFile = `package p;

import android.app.Activity;

import java.io.File;

public class MainActivity extends Activity {
    public File conCuenta() {
        return new File(getApplicationContext().getExternalFilesDir(null), 1 + 2);
    }
}
`

func fuentesConcatEnFile(fuente string) []fuenteJava {
	return []fuenteJava{{nombre: "src/p/MainActivity.java", contenido: fuente}}
}

// TestConcatComoArgumentoFalsificacion: apagado el caso `joperador`, la sonda de la línea 179 vuelve
// a parar con el mensaje literal que leyó el usuario, nombrando el fuente y la línea.
func TestConcatComoArgumentoFalsificacion(t *testing.T) {
	const literal = "el tipo de una expresion main.joperador no se puede estimar sin emitirla"
	jar := jarDelSDK(t)
	fuentes := fuentesConcatEnFile(fuenteConcatEnFile)

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con el caso encendido el fuente no compila: %v", err)
	}

	anterior := estimaConcatEnArgumento
	estimaConcatEnArgumento = false
	t.Cleanup(func() { estimaConcatEnArgumento = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con el caso apagado el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con el caso apagado el rechazo es %q, y el del usuario contiene %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "MainActivity.java: ") || !strings.Contains(err.Error(), "linea 26") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la concatenación: %q", err)
	}
	t.Logf("sin el caso joperador: %v", err)
}

// invocacionesDeFileInit: los descriptores con los que un método llama a `java/io/File.<init>`, en orden
// de pc, leídos del pool de su propia clase (el índice del pool no se compara entre javac y nosotros,
// pero su contenido sí dice qué sobrecarga se eligió).
func invocacionesDeFileInit(t *testing.T, p *pool, code []byte) []string {
	t.Helper()
	instr, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("no se desarma el código: %v", err)
	}
	var out []string
	for _, in := range instr {
		if in.op != opInvokespec || len(in.ops) < 2 {
			continue
		}
		idx := int(in.ops[0])<<8 | int(in.ops[1])
		body := p.body[idx]
		if len(body) < 4 {
			t.Fatalf("Methodref sin operandos en %d", idx)
		}
		if p.utf[p.class[int(body[0])<<8|int(body[1])]] != "java/io/File" {
			continue
		}
		nt := p.body[int(body[2])<<8|int(body[3])]
		if len(nt) < 4 || p.utf[int(nt[0])<<8|int(nt[1])] != "<init>" {
			continue
		}
		out = append(out, p.utf[int(nt[2])<<8|int(nt[3])])
	}
	return out
}

// poolDeEmitida y poolDeLeida: el pool de cada lado, que es donde vive el Methodref de `File.<init>`.
func poolDeEmitida(t *testing.T, clases []claseEmitida, clase string) *pool {
	t.Helper()
	for _, c := range clases {
		if c.interno != clase {
			continue
		}
		leida, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase %s no se lee: %v", clase, err)
		}
		return leida.p
	}
	t.Fatalf("no hay %s en lo emitido", clase)
	return nil
}

func poolDeLeida(t *testing.T, clases []classFull, clase string) *pool {
	t.Helper()
	for _, c := range clases {
		if c.Name == clase {
			return c.p
		}
	}
	t.Fatalf("no hay %s en lo de javac", clase)
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
