package main

import (
	"bytes"
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

// TestElDexDeLaVueltaCovarianteEsElDeJavac17: la sonda dexa como la de javac 17 `--release 8
// -g:lines,source`, byte a byte. El dex es el oráculo que sí ve el descriptor elegido: la firma del
// `invokevirtual` lleva el retorno, así que si el desempate escogiera el `Adapter` de `AdapterView` el
// `method_id` sería otro y el dex dejaría de coincidir. Y como el receptor de `getItem` es `ListAdapter`,
// que no declara `getItem` —lo hereda de `Adapter`—, la coincidencia también prueba el recorrido de
// superinterfaces de `candidatosDelJar`.
func TestElDexDeLaVueltaCovarianteEsElDeJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteSondaVuelta}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda de la vuelta covariante: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaVuelta)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra := claseEmitidaPorNombre(t, nuestras, "p/Sonda")
	suya := claseLeidaPorNombre(t, deJavac, "Lp/Sonda;")
	comparaDexByteAByte(t, dexDeUnaClase(t, nuestra), dexDeUnaClaseLeida(t, suya), volcadoDeClases(nuestras))

	// las tres llamadas, leídas del dex de javac, para dejar escrito qué elige cada una
	for _, quiere := range []struct {
		dueño, nombre, desc string
	}{
		{"Landroid/widget/ListView;", "getAdapter", "()Landroid/widget/ListAdapter;"},
		{"Landroid/widget/ListAdapter;", "getItem", "(I)Ljava/lang/Object;"},
		{"Landroid/widget/AdapterView;", "getAdapter", "()Landroid/widget/Adapter;"},
	} {
		if !internaEnMetodos(t, suya, quiere.dueño, quiere.nombre, quiere.desc) {
			t.Fatalf("javac no usa %s.%s:%s y el caso ya no mide lo que dice medir",
				quiere.dueño, quiere.nombre, quiere.desc)
		}
		if !internaEnMetodos(t, leeNuestra(t, nuestra), quiere.dueño, quiere.nombre, quiere.desc) {
			t.Errorf("nuestro dex no usa %s.%s:%s", quiere.dueño, quiere.nombre, quiere.desc)
		}
	}

	// el `check-cast` que exige el tipo borrado del otro: `AdapterView.getAdapter` devuelve `Adapter` y
	// el hueco pide `BaseAdapter`, así que javac lo escribe y nosotros también
	borrado := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "borrado", "(Landroid/widget/AdapterView;)Landroid/widget/BaseAdapter;")
	if n := cuentaCheckcast(t, borrado.Code); n != 1 {
		t.Fatalf("`borrado` lleva %d `checkcast` y javac escribe 1", n)
	}
	// y `lista`, que es el hueco exacto del retorno elegido, no lleva ninguno
	lista := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "lista", "()Landroid/widget/ListAdapter;")
	if n := cuentaCheckcast(t, lista.Code); n != 0 {
		t.Fatalf("`lista` lleva %d `checkcast` y javac no escribe ninguno", n)
	}
}

// TestElDexDeLaExpresionDeApkShareEsElDeJavac17: la fuente de `MainActivity.java:48-57` entera. Las
// tres clases —la de fuera, `App` y la anónima— dexan byte a byte. La anónima se comparaba solo
// instrucción a instrucción por dos divergencias de su constructor que ya no existen: el
// `MethodParameters` que este emisor escribía y javac 17 no pone en ninguna anónima, y la `Signature`
// `()V`, que ahora escribe igual. El descriptor elegido se sigue comprobando además en los bytes del
// propio `.class`, porque ahí se ve que es `ListAdapter` y no el `Adapter` que borra la genérica.
func TestElDexDeLaExpresionDeApkShareEsElDeJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteGetAdapterShare}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la expresión de ApkShare: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "share/apps/util/MainActivity.java", fuenteGetAdapterShare)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	for _, interno := range []string{"share/apps/util/App", "share/apps/util/MainActivity"} {
		nuestra := claseEmitidaPorNombre(t, nuestras, interno)
		suya := claseLeidaPorNombre(t, deJavac, "L"+interno+";")
		comparaDexByteAByte(t, dexDeUnaClase(t, nuestra), dexDeUnaClaseLeida(t, suya), volcadoDeClases(nuestras))
	}
	// y el dex de las tres clases juntas, la anónima incluida, sin normalizar el constructor
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la expresión de ApkShare no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	anónima := claseEmitidaPorNombre(t, nuestras, "share/apps/util/MainActivity$1")
	// el descriptor elegido viaja en el pool: es `ListAdapter`, no el `Adapter` que borra la genérica
	if !bytes.Contains(anónima, []byte("()Landroid/widget/ListAdapter;")) {
		t.Fatalf("la anónima no referencia `getAdapter:()Landroid/widget/ListAdapter;`")
	}
	if bytes.Contains(anónima, []byte("()Landroid/widget/Adapter;")) {
		t.Fatalf("la anónima referencia el `getAdapter:()Landroid/widget/Adapter;` de `AdapterView`")
	}
	// y las instrucciones de la llamada son las de javac
	const descOnItemClick = "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V"
	if err := comparaInstrucciones(
		metodoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity$1", "onItemClick", descOnItemClick).Code,
		metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/MainActivity$1;", "onItemClick", descOnItemClick).Code,
	); err != nil {
		t.Fatalf("las instrucciones de `onItemClick` no son las de javac: %v", err)
	}
}

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

// TestElDesempatePorVueltaEnUnaJerarquiaPropia: `trae()` tiene dos declaraciones visibles desde `Hija`
// —la suya, que devuelve `String`, y la heredada, que devuelve `Object`— y gana la estrecha. Antes del
// desempate esto era ambiguo y `usa()` no compilaba.
func TestElDesempatePorVueltaEnUnaJerarquiaPropia(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Hija.java", contenido: fuenteGetAdapterDueñoPropio}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la jerarquía con la vuelta covariante: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Hija.java", fuenteGetAdapterDueñoPropio)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	for _, interno := range []string{"p/Base", "p/Hija"} {
		nuestra := claseEmitidaPorNombre(t, nuestras, interno)
		suya := claseLeidaPorNombre(t, deJavac, "L"+interno+";")
		comparaDexByteAByte(t, dexDeUnaClase(t, nuestra), dexDeUnaClaseLeida(t, suya), volcadoDeClases(nuestras))
	}
}

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
