package main

import (
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
