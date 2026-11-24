package main

// §225: el `for` con dos puntos, `for (T v : coleccion)`, medido contra javac 17 `--release 8`. Es el
// bloqueo que el usuario encontró al compilar ApkShare con `--compiler go`:
//
//	src/share/apps/util/App.java: linea 37: se esperaba ";" y hay ":"
//
// javac lo baja de dos maneras, y las dos están aquí:
//
//   - sobre un arreglo (`String[]`, `int[]`), copia el arreglo y su largo a locales sintéticos y
//     cuenta con un índice: `astore arr$; aload arr$; arraylength; istore len$; iconst_0; istore i$`,
//     `iload i$; iload len$; if_icmpge fin`, `aload arr$; iload i$; *aload; *store v` y `iinc i$, 1`.
//     El `iinc` lleva la línea del `for`; el `goto` de vuelta la hereda.
//   - sobre un `Iterable`, `iterator()` una vez y `hasNext()`/`next()` por vuelta, con `checkcast`
//     del elemento (sin él cuando es `Object`). El `goto` de vuelta lleva la línea del cierre del
//     cuerpo, no la del `for`, y `continue` salta a la condición porque javac pliega el paso vacío.
//
// El oráculo es el de siempre: nuestro dex y el de javac 17 byte a byte, y además el `max_stack`/
// `locals` y las filas de la LineNumberTable método a método.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteApkDeApkShare es la clase que viaja como argumento de `List<Apk>` y la que `App.getApks`
// construye dentro del `for`, reducida a lo que el `for` necesita: el constructor `(PackageManager,
// String)` del zip. La forma completa de `Apk.java` (con `getPackageArchiveInfo` y la unión de flags)
// no aporta nada al oráculo del `for` y traería sus propias fronteras.
const fuenteApkDeApkShare = `package share.apps.util;

import android.content.pm.PackageManager;

public class Apk {
    public final String filePath;

    public Apk(PackageManager pm, String filePath) {
        this.filePath = filePath;
    }
}
`

// fuenteAppPorCada es `App.getApks` de `App.java`, con la línea 37 **verbatim**
// (`for (String path: info.splitSourceDirs) {`) y la misma forma alrededor: el `new ArrayList<>()`,
// el `if` sin llaves con su `return`, el `dirs.add(new Apk(pm, path))` dentro del `for` y el campo
// `List<Apk> apks` que el constructor llena. Se reducen solo las dos cosas que no tocan el `for`:
// `getApks(pm, info)` recibe el parámetro entero y el `sourceDir` pasa por una local, porque usar un
// `local.campo` como argumento de una llamada cae en la estimación de tipos, que es **otra** frontera
// (§225, medida en la falsificación: con el `for` cerrado, `App.java` completo se detiene en la
// línea 28, `getApks(pm, info.applicationInfo)`).
const fuenteAppPorCada = `package share.apps.util;

import android.content.pm.ApplicationInfo;
import android.content.pm.PackageManager;

import java.util.List;
import java.util.ArrayList;

public class App {
    public final List<Apk> apks;

    public App(PackageManager pm, ApplicationInfo info) {
        apks = getApks(pm, info);
    }

    private List<Apk> getApks(PackageManager pm, ApplicationInfo info) {
        List<Apk> dirs = new ArrayList<>();
        String dir = info.sourceDir;
        dirs.add(new Apk(pm, dir));
        if (pm == null)
            return dirs;

        for (String path: info.splitSourceDirs) {
            dirs.add(new Apk(pm, path));
        }

        return dirs;
    }
}
`

// fuenteCada es la sonda de las formas del `for` con dos puntos que no salen en App.java: el arreglo
// de primitivos, el `Iterable` por interface con `continue`, el `Iterable` por clase (que resuelve
// `iterator()` a `invokevirtual`) con `break`, y el elemento `Object`, que no lleva `checkcast`.
const fuenteCada = `package p;

import java.util.List;
import java.util.ArrayList;

public class Cada {
    static int total;

    public static int suma(String[] a) {
        int n = 0;
        for (String s : a) {
            n = n + s.length();
        }
        return n;
    }

    public static int sumaInts(int[] a) {
        int n = 0;
        for (int v : a) {
            n = n + v;
        }
        return n;
    }

    public static int cuenta(List<String> l) {
        int n = 0;
        for (String s : l) {
            if (s == null) continue;
            n = n + s.length();
        }
        return n;
    }

    public static int corta(ArrayList<Apk> l) {
        int n = 0;
        for (Apk a : l) {
            if (a.n > 3) break;
            n = n + a.n;
        }
        return n;
    }

    public static int objetos(List<Object> l) {
        int n = 0;
        for (Object o : l) {
            n = n + o.hashCode();
        }
        return n;
    }
}

class Apk { int n; }
`

// compilaLosDos compila `fuentes` con el emisor y con javac 17 `--release 8 -g:lines,source` y deja
// las dos listas de clases. Es el arranque común de las dos pruebas de oráculo.
func compilaLosDos(t *testing.T, jar, javac string, fuentes []fuenteJava) ([]claseEmitida, []classFull) {
	t.Helper()
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el `for` con dos puntos: %v", err)
	}
	raiz := t.TempDir()
	rutas := make([]string, 0, len(fuentes))
	for _, f := range fuentes {
		pon(t, raiz, f.nombre, f.contenido)
		rutas = append(rutas, filepath.Join(raiz, f.nombre))
	}
	salida := filepath.Join(raiz, "javac-out")
	args := append([]string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida}, rutas...)
	if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	return nuestras, deJavac
}

// lineaDeLaSonda: la línea del fuente de la sonda que lleva `marca`, contada desde 1. Evita clavar un
// número que se mueve al tocar los imports de la sonda.
func lineaDeLaSonda(t *testing.T, fuente, marca string) int {
	t.Helper()
	for i, l := range strings.Split(fuente, "\n") {
		if strings.Contains(l, marca) {
			return i + 1
		}
	}
	t.Fatalf("la sonda no tiene la marca %q", marca)
	return 0
}

// mapaDeLineas: la LineNumberTable de un método, por línea. Se usa para dejar escrito que el `for`
// con dos puntos cae en la línea del fuente que el usuario vio en el error.
func mapaDeLineas(t *testing.T, c classFull, metodo string) map[int]int {
	t.Helper()
	for _, m := range c.Methods {
		if m.Name != metodo {
			continue
		}
		out := map[int]int{}
		for _, fila := range m.Lines {
			out[fila.line] = fila.pc
		}
		return out
	}
	t.Fatalf("javac no dejó el método %s en %s", metodo, c.Name)
	return nil
}

// TestPorCadaFalsificacion: la falsificación de la ruta nueva. Con el interruptor apagado, el fuente
// literal de `App.java` tiene que volver a fallar con el mensaje que el usuario leyó,
// «src/share/apps/util/App.java: linea 37: se esperaba ";" y hay ":"»: el mismo texto, con el prefijo
// del fichero que pone `compilarProyectoJava`.
func TestPorCadaFalsificacion(t *testing.T) {
	const literal = `src/share/apps/util/App.java: linea 37: se esperaba ";" y hay ":"`
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{
		{nombre: "src/share/apps/util/Apk.java", contenido: fuenteApkDeApkShare},
		{nombre: "src/share/apps/util/App.java", contenido: fuenteAppDelUsuario},
	}

	// encendida, la línea 37 se acepta: el parser ya no se detiene ahí. Lo que venga después es otra
	// frontera del mismo fichero (hoy, la línea 28 al estimar el tipo de `info.applicationInfo`), y se
	// deja escrito en el registro para no confundirla con esta.
	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		if contieneLinea37(err) {
			t.Fatalf("con la ruta del `for` con dos puntos encendida la línea 37 no se aceptó: %v", err)
		}
		t.Logf("con la ruta encendida, la siguiente frontera de App.java: %v", err)
	}

	anterior := admitePorCada
	admitePorCada = false
	t.Cleanup(func() { admitePorCada = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta del `for` con dos puntos apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if err.Error() != literal {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario %q", err, literal)
	}
	t.Logf("sin la ruta: %v", err)
}

// contieneLinea37: si el error habla de la línea 37, que es la que mide esta prueba. Cualquier otro
// rechazo (otra frontera del mismo fuente) no invalida que la línea 37 se acepte.
func contieneLinea37(err error) bool {
	return err != nil && strings.Contains(err.Error(), "linea 37:")
}
