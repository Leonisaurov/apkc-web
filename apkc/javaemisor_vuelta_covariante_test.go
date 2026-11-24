package main

import (
	"strings"
	"testing"
)

// Este fichero cierra el desempate de sobrecargas por el tipo de vuelta. El caso es la línea 55 de
// `MainActivity.java` de ApkShare, `apps.getAdapter().getItem(positon)` con `apps` un `ListView`: javac
// ve dos declaraciones con la misma lista de parámetros vacía —la de `AbsListView`, que devuelve
// `ListAdapter`, y la genérica de `AdapterView`, que borra a `Adapter`— y elige la primera por el
// retorno covariante (JVMS 4.6.5). Sin ese desempate el emisor rechazaba la llamada con «tiene 2
// sobrecargas aplicables … y ninguna es la mas especifica», que es el error que se midió con el binario
// del árbol anterior.

// fuenteGetAdapterShare: las líneas 48-57 de `MainActivity.java` de ApkShare, con el aparato mínimo que
// hace falta para que la expresión quede dentro de la anónima y con la captura de la local (`val$apps`),
// que es como viaja en el fuente de verdad. Lo demás —`findViewById`, la escucha, el `App`— es el
// contexto que javac necesita para tipar la misma llamada.
const fuenteGetAdapterShare = `package share.apps.util;

import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.widget.AdapterView;
import android.widget.ListView;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        ListView apps = (ListView) findViewById(0);
        apps.setOnItemClickListener(new AdapterView.OnItemClickListener() {
            @Override
            public void onItemClick(AdapterView<?> adapter, View view, int positon, long id) {
                detalle((App) apps.getAdapter().getItem(positon));
            }
        });
    }

    void detalle(App a) {
    }
}

class App {
}
`

// fuenteSondaVuelta: el caso desnudo del desempate y el del `check-cast` que exige el tipo borrado del
// otro, en una clase sin anónima para que el dex entero se pueda comparar byte a byte. `item` y `lista`
// llaman al `getAdapter` covariante (`ListAdapter`), y `borrado` llama al genérico de `AdapterView`, que
// borra a `Adapter`, con un hueco más estrecho: javac escribe `checkcast BaseAdapter` detrás.
const fuenteSondaVuelta = `package p;

import android.widget.AdapterView;
import android.widget.BaseAdapter;
import android.widget.ListAdapter;
import android.widget.ListView;

class Sonda {
    ListView apps;

    Object item(int i) {
        return apps.getAdapter().getItem(i);
    }

    ListAdapter lista() {
        return apps.getAdapter();
    }

    BaseAdapter borrado(AdapterView<BaseAdapter> vista) {
        return vista.getAdapter();
    }
}
`

// leeNuestra: la `.class` emitida, leída para poder mirarla con las mismas herramientas que la de javac.
func leeNuestra(t *testing.T, datos []byte) classFull {
	t.Helper()
	c, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	return c
}

// internaEnMetodos: si el dex de esa clase nombra ese método con ese dueño, nombre y descriptor. Es la
// misma vista que usan los tests de sobrecarga: `collectClasses` sigue las referencias del cuerpo, así que
// el `method_id` de una llamada aparece aunque su dueño no viaje en la clase.
func internaEnMetodos(t *testing.T, clase classFull, dueño, nombre, desc string) bool {
	t.Helper()
	tabla := newDexTables()
	if err := tabla.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando %s: %v", clase.Name, err)
	}
	tabla.orderTables()
	for _, m := range tabla.methods {
		if m.owner == dueño && m.name == nombre && m.desc == desc {
			return true
		}
	}
	return false
}

// fuenteGetAdapterDueñoPropio: el mismo desempate sobre una jerarquía de los fuentes, para que no quede
// pegado al `android.jar`. `Hija` reescribe `trae()` con una vuelta más estrecha y `usa()` la llama.
const fuenteGetAdapterDueñoPropio = `package p;

class Base {
    Object trae() {
        return null;
    }
}

class Hija extends Base {
    @Override
    String trae() {
        return "x";
    }

    String usa() {
        return trae();
    }
}
`

// TestLosRetornosNoEmparentadosSiguenSiendoAmbiguos: el control. Dos máximas por parámetros cuyas
// vueltas no son subtipo la una de la otra no se desempatan: no hay «la más específica» y la llamada
// sigue siendo ambigua. Se comprueba sobre la resolución misma, porque en Java no se puede escribir esa
// jerarquía —javac rechaza el redefinido con un retorno incompatible— y por eso no hay fuente que la
// reproduzca. Los tipos son del `android.jar` de verdad, que es el classpath que usa el emisor.
func TestLosRetornosNoEmparentadosSiguenSiendoAmbiguos(t *testing.T) {
	jar := jarDelSDK(t)
	busc, err := abrirClasspath([]string{jar})
	if err != nil {
		t.Fatalf("abriendo el classpath: %v", err)
	}
	defer busc.cerrar()
	ctx := &contextoJava{busc: busc}

	// `String` y `Runnable` no están emparentados: ninguna vuelta cabe en la otra
	sueltas := []candidato{{desc: "()Ljava/lang/String;"}, {desc: "()Ljava/lang/Runnable;"}}
	if unica, ok := ctx.masEspecificaDe(sueltas); ok {
		t.Fatalf("se desempató por vuelta sin relación y salió %s", unica.desc)
	}

	// y el desempate sí está midiendo el parentesco: `String` cabe en `Object`
	emparentadas := []candidato{{desc: "()Ljava/lang/Object;"}, {desc: "()Ljava/lang/String;"}}
	unica, ok := ctx.masEspecificaDe(emparentadas)
	if !ok || unica.desc != "()Ljava/lang/String;" {
		t.Fatalf("`String` no ganó a `Object`: %v %v", unica.desc, ok)
	}
}

// TestFalsificacionDelDesempatePorVuelta: apagado el desempate vuelve el error literal que se midió con
// el binario del árbol anterior. Es la prueba de que lo que arregla la expresión de ApkShare es este paso
// y no otra cosa del árbol.
func TestFalsificacionDelDesempatePorVuelta(t *testing.T) {
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteGetAdapterShare}}

	anterior := desempataPorVuelta
	desempataPorVuelta = false
	_, err := compilarProyectoJava(fuentes, jar)
	desempataPorVuelta = anterior
	if err == nil {
		t.Fatal("con el desempate apagado se aceptó el `getAdapter` ambiguo")
	}
	// el mensaje del usuario, con la línea del fuente delante
	for _, pista := range []string{
		"linea 15",
		"android/widget/ListView.getAdapter tiene 2 sobrecargas aplicables en la fase estricta a",
		"(()Landroid/widget/ListAdapter; ()Landroid/widget/Adapter;)",
		"ninguna es la mas especifica para este subconjunto",
	} {
		if !strings.Contains(err.Error(), pista) {
			t.Fatalf("el error no trae %q: %v", pista, err)
		}
	}
	t.Logf("error con el desempate apagado: %v", err)
}
