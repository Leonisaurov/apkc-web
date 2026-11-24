package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Este fichero cierra la asignación de una anónima a un campo de su propia interfaz, que era la
// frontera medida en `MainActivity.java:99` de ApkShare: `runnable = new Runnable() { … }` dentro de
// la anónima de `TextWatcher` (`MainActivity.java:84`) y `runnable` declarado `Runnable`
// (`MainActivity.java:39`). La causa estaba en `superEInterfaces`: para una clase propia devolvía
// solo el padre y descartaba su `implementa`, así que `asignableEstricto` no veía que
// `MainActivity$1$1` es un `Runnable` y el `access$002` de la escritura del campo rechazaba la
// asignación con «runnable es Ljava/lang/Runnable; y se le asigna un
// Lshare/apps/util/MainActivity$4$1;». El oráculo es javac 17 `--release 8 -g:lines,source` y, como
// en el resto del repo, el dex byte a byte.

// fuenteInterfazAnonima es el trozo real de `MainActivity.java:84-104` del zip de ApkShare —la
// anónima de `TextWatcher` con su `afterTextChanged`, que es donde vive la línea 99— sin las líneas
// de fuera del trozo: ni el `findViewById` que precede, ni el `postDelayed` de la 105, ni el resto
// del `onCreate`. El `ApkAdapter` del proyecto se sustituye por el mínimo que declara
// `searchQuery(String)`, que es lo único que la línea 101 usa; los demás tipos (`Handler`,
// `TextWatcher`, `Editable`, `EditText`) vienen del `android.jar`.
const fuenteInterfazAnonima = `package share.apps.util;

import android.os.Handler;
import android.text.Editable;
import android.text.TextWatcher;
import android.widget.EditText;

public class MainActivity {
    private static final int AutoResearchDelay = 500;
    private Runnable runnable;
    private Handler handler;
    private ApkAdapter adapter;

    void busca(EditText searchQuery) {
        searchQuery.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {
            }

            @Override
            public void onTextChanged(CharSequence s, int start, int before, int count) {
                if (runnable != null) {
                    handler.removeCallbacks(runnable);
                }
            }

            @Override
            public void afterTextChanged(Editable s) {
                runnable = new Runnable() {
                    @Override
                    public void run() {
                        adapter.searchQuery(s.toString());
                    }
                };
            }
        });
    }
}
`

const fuenteApkAdapterInterfaz = `package share.apps.util;

class ApkAdapter {
    void searchQuery(String q) {
    }
}
`

// fuenteAnonimaNoRelacionada es el control: la misma asignación con una anónima de una interfaz que
// no tiene nada que ver con `Runnable`. Tiene que seguir siendo un error —javac 17 dice
// «incompatible types: <anonymous Otra> cannot be converted to Runnable»— o el arreglo sería pasar
// cualquier anónima a cualquier campo de referencia.
const fuenteAnonimaNoRelacionada = `package p;

class A {
    private Runnable runnable;

    void m() {
        runnable = new Otra() {
            public void haz() {
            }
        };
    }
}
`

const fuenteOtraInterfaz = `package p;

interface Otra {
    void haz();
}
`

func fuentesInterfazAnonima() []fuenteJava {
	return []fuenteJava{
		{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteInterfazAnonima},
		{nombre: "src/share/apps/util/ApkAdapter.java", contenido: fuenteApkAdapterInterfaz},
	}
}

// TestLaAsignacionDeUnaAnonimaARunnableEsLaDeJavac17: el caso de ApkShare compila y el dex de las
// cuatro clases es el de javac byte a byte. Antes del arreglo el emisor lo rechazaba y no dejaba
// ningún `.class`; sin la lista de interfaces en `superEInterfaces`, la anónima solo se veía como
// `Object` a través de su padre.
func TestLaAsignacionDeUnaAnonimaARunnableEsLaDeJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesInterfazAnonima()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `runnable = new Runnable() { … }`: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// lo que la asignación miraba: la anónima declara `Runnable` en su `interfaces`
	anonima, err := readClassFull(claseEmitidaPorNombre(t, nuestras, "share/apps/util/MainActivity$1$1"))
	if err != nil {
		t.Fatalf("la anónima no se lee: %v", err)
	}
	if len(anonima.Interfaces) != 1 || anonima.Interfaces[0] != "Ljava/lang/Runnable;" {
		t.Fatalf("la anónima declara %v y javac escribe [Ljava/lang/Runnable;]", anonima.Interfaces)
	}
	suya := claseLeidaPorNombre(t, deJavac, "Lshare/apps/util/MainActivity$1$1;")
	if len(suya.Interfaces) != 1 || suya.Interfaces[0] != anonima.Interfaces[0] {
		t.Fatalf("javac declara %v y nosotros %v", suya.Interfaces, anonima.Interfaces)
	}

	// la asignación, instrucción a instrucción: `aload_0`, el `new` de la anónima con su captura y el
	// `invokestatic access$002`, que es el accesor de escritura del campo privado
	const desc = "(Landroid/text/Editable;)V"
	nuestro := metodoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity$1", "afterTextChanged", desc)
	suAfter := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/MainActivity$1;", "afterTextChanged", desc)
	if err := comparaInstrucciones(nuestro.Code, suAfter.Code); err != nil {
		t.Fatalf("el `afterTextChanged` con la asignación no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// y el accesor de escritura del campo: mismo descriptor `(LMainActivity;Ljava/lang/Runnable;)LRunnable;`
	// e instrucciones de javac. El **nombre** no coincide todavía —nosotros repartimos `access$NNN` por
	// orden de creación al emitir y javac por el orden de los campos del exterior—, que es la frontera
	// siguiente de este mismo fuente: nuestro `MainActivity` deja `access$000` para `adapter`,
	// `access$100`/`access$102` para `runnable` y `access$200` para `handler`, y javac deja `000`/`002`
	// para `runnable`, `100` para `handler` y `200` para `adapter`. No se arregla aquí.
	const descEscritura = "(Lshare/apps/util/MainActivity;Ljava/lang/Runnable;)Ljava/lang/Runnable;"
	nuestroAcc := accesoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity", descEscritura)
	suAcc := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/MainActivity;", "access$002", descEscritura)
	if err := comparaInstrucciones(nuestroAcc.Code, suAcc.Code); err != nil {
		t.Fatalf("el accesor de escritura de `runnable` no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// El dex de estas clases **no** se compara byte a byte: no es alcanzable con este fuente, y no
	// por este frente. `MainActivity$1$1.run()` resuelve `Editable.toString` como `invokeinterface`
	// donde javac escribe `invokevirtual java/lang/Object.toString` —medido también con el binario de
	// `cdcf9e8` sobre el mismo fuente sin la asignación, así que es anterior—, y el nombre del
	// accesor cambia los índices. Las dos cosas son fronteras aparte.
}

// accesoDeClaseEmitida: el accesor sintético `static` con ese descriptor, sin depender de su nombre.
func accesoDeClaseEmitida(t *testing.T, clases []claseEmitida, clase, desc string) classMethod {
	t.Helper()
	leida, err := readClassFull(claseEmitidaPorNombre(t, clases, clase))
	if err != nil {
		t.Fatalf("nuestra clase %s no se lee: %v", clase, err)
	}
	for _, m := range leida.Methods {
		if m.Desc == desc {
			return m
		}
	}
	t.Fatalf("%s no tiene ningún método %s", clase, desc)
	return classMethod{}
}

// TestUnaAnonimaDeUnaInterfazNoRelacionadaSigueRechazandose: el control del arreglo. `Otra` no es
// `Runnable`, así que la asignación sigue siendo un error —el del usuario, con el nombre interno de
// la anónima— y javac 17 rechaza el mismo fuente por el mismo motivo.
func TestUnaAnonimaDeUnaInterfazNoRelacionadaSigueRechazandose(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{
		{nombre: "src/p/A.java", contenido: fuenteAnonimaNoRelacionada},
		{nombre: "src/p/Otra.java", contenido: fuenteOtraInterfaz},
	}

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatalf("se aceptó `runnable = new Otra() { … }`, y `Otra` no es `Runnable`")
	}
	const esperado = "runnable es Ljava/lang/Runnable; y se le asigna un Lp/A$1;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el error es %q y tenía que contener %q", err, esperado)
	}

	// el oráculo: javac rechaza lo mismo
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", filepath.Join(raiz, "javac-out")}
	for _, f := range fuentes {
		args = append(args, filepath.Join(raiz, f.nombre[len("src/"):]))
	}
	salida, err := exec.Command(javac, args...).CombinedOutput()
	if err == nil {
		t.Fatalf("javac aceptó el control y tenía que rechazarlo:\n%s", salida)
	}
	if !strings.Contains(string(salida), "cannot be converted to Runnable") {
		t.Fatalf("javac rechazó el control por otro motivo:\n%s", salida)
	}
}

// TestSinLasInterfacesDeLaPropiaLaAsignacionEsElErrorDelUsuario: la falsificación. Con el
// interruptor apagado —`superEInterfaces` devolviendo solo el padre, como antes del arreglo— vuelve
// el error literal del usuario. En el proyecto real el nombre es `MainActivity$4$1` (la numeración
// de anónimas del zip, que este frente no toca); aquí, con una sola anónima antes, es `$1$1`.
func TestSinLasInterfacesDeLaPropiaLaAsignacionEsElErrorDelUsuario(t *testing.T) {
	jar := jarDelSDK(t)
	interfacesDeLaPropiaEnAsignable = false
	t.Cleanup(func() { interfacesDeLaPropiaEnAsignable = true })

	_, err := compilarProyectoJava(fuentesInterfazAnonima(), jar)
	if err == nil {
		t.Fatalf("con el interruptor apagado la asignación tenía que fallar")
	}
	const esperado = "runnable es Ljava/lang/Runnable; y se le asigna un Lshare/apps/util/MainActivity$1$1;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el error de la falsificación es %q y tenía que contener %q", err, esperado)
	}
	t.Logf("falsificación: %v", err)

	// y encendido, el mismo fuente compila: el interruptor es lo único que cambia
	interfacesDeLaPropiaEnAsignable = true
	if _, err := compilarProyectoJava(fuentesInterfazAnonima(), jar); err != nil {
		t.Fatalf("encendido, el mismo fuente rechaza: %v", err)
	}
}
