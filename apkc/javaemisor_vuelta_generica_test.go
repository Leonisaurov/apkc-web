//go:build !js

package main

import (
	"os"
	"strings"
	"testing"
)

// El frente de la vuelta genérica sin borrado. El `/sdk/android.jar` que sirve la pestaña se arma
// desde fuentes AOSP y no sustituye la variable de tipo por su borrado: `AdapterView.getAdapter`
// sale `()LT;` y compite con el `()Landroid/widget/ListAdapter;` concreto de `ListView`. javac 17
// elige la declaración covariante de la subclase; sin este desempate el emisor rechazaba la línea 55
// de ApkShare con «2 sobrecargas aplicables en la fase estricta a (()Landroid/widget/ListAdapter;
// ()LT;)». Los oráculos de javac van por build tag (`!js`) porque el hospedador de wasm no crea
// procesos; sin javac 17 las pruebas fallan, no se saltan (ver `dependencias_test.go`).

// fuenteVueltaGenerica: la expresión de `MainActivity.java:55` desnuda, sin la lambda de la línea
// 156 ni nada que pertenezca a otro frente. `item` es la llamada encadenada sobre `ListAdapter` y
// `lista` la vuelta directa, para que el descriptor elegido viaje en el `invokevirtual` del dex.
const fuenteVueltaGenerica = `package p;

import android.widget.ListAdapter;
import android.widget.ListView;

class App {
}

class Sonda {
    ListView apps;

    Object item(int positon) {
        return (App) apps.getAdapter().getItem(positon);
    }

    ListAdapter lista() {
        return apps.getAdapter();
    }
}
`

// fuenteVueltaGenericaControl: el mismo nombre sobre un receptor declarado `AdapterView<T>`, donde
// javac sí usa la declaración genérica y escribe un `checkcast` por el borrado. Prueba que el
// desempate no eligió la concreta en todas partes, solo cuando el receptor es la subclase.
const fuenteVueltaGenericaControl = `package p;

import android.widget.AdapterView;
import android.widget.BaseAdapter;

class Control {
    BaseAdapter borrado(AdapterView<BaseAdapter> vista) {
        return vista.getAdapter();
    }
}
`

// jarStubSinBorrado: el jar AOSP servido en la pestaña, el único que trae `()LT;`. Se busca donde lo
// deja `tools/entorno.sh jar` y se puede señalar con `APKC_ANDROID_JAR_STUB`. Sin él la prueba se
// salta con el motivo, porque es un artefacto construido y no vive en el repo.
func jarStubSinBorrado(t *testing.T) string {
	t.Helper()
	for _, r := range []string{
		os.Getenv("APKC_ANDROID_JAR_STUB"),
		homeDir() + "/.cache/apkc-web/androidjar/android-stub-34-full.jar",
	} {
		if r == "" {
			continue
		}
		if _, err := os.Stat(r); err == nil {
			return r
		}
	}
	t.Skip("sin el jar stub sin borrado (android-stub-34-full.jar) bajo ~/.cache/apkc-web/androidjar")
	return ""
}

// TestElDexDeLaVueltaGenericaSinBorradoEsElDeJavac17: compilamos la sonda contra el jar sin borrado
// y javac 17 contra el jar del SDK. Aunque los jars difieran en el descriptor de la genérica, la
// llamada elegida es la misma y el dex de las tres clases coincide byte a byte.
func TestElDexDeLaVueltaGenericaSinBorradoEsElDeJavac17(t *testing.T) {
	jar := jarStubSinBorrado(t)
	jarJavac := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteVueltaGenerica}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la vuelta genérica sin borrado: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteVueltaGenerica)
	deJavac := compilaConJavac(t, javac, jarJavac, raiz, fuentes)

	for _, interno := range []string{"p/App", "p/Sonda"} {
		nuestra := claseEmitidaPorNombre(t, nuestras, interno)
		suya := claseLeidaPorNombre(t, deJavac, "L"+interno+";")
		comparaDexByteAByte(t, dexDeUnaClase(t, nuestra), dexDeUnaClaseLeida(t, suya), volcadoDeClases(nuestras))
	}

	// el oráculo de javac y el nuestro nombran la concreta, no la genérica
	for _, quiere := range []struct{ dueño, nombre, desc string }{
		{"Landroid/widget/ListView;", "getAdapter", "()Landroid/widget/ListAdapter;"},
		{"Landroid/widget/ListAdapter;", "getItem", "(I)Ljava/lang/Object;"},
	} {
		if !internaEnMetodos(t, leeNuestra(t, claseEmitidaPorNombre(t, nuestras, "p/Sonda")),
			quiere.dueño, quiere.nombre, quiere.desc) {
			t.Fatalf("nuestro dex no usa %s.%s:%s", quiere.dueño, quiere.nombre, quiere.desc)
		}
	}
	// asignar la vuelta a su hueco exacto no pide ningún `check-cast`
	lista := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "lista", "()Landroid/widget/ListAdapter;")
	if n := cuentaCheckcast(t, lista.Code); n != 0 {
		t.Fatalf("`lista` lleva %d `checkcast` y javac no escribe ninguno", n)
	}
}

// TestElReceptorAdapterViewSigueUsandoLaGenerica: con un `AdapterView<BaseAdapter>` como receptor no
// hay declaración concreta que gane. javac 17 escribe la genérica del SDK (`()Landroid/widget/Adapter;`)
// y un `checkcast BaseAdapter`; el emisor sobre el jar sin borrado escribe la genérica `()LT;` y el
// mismo `checkcast`. Lo que se compara es la decisión, no el descriptor, que en cada jar es distinto.
func TestElReceptorAdapterViewSigueUsandoLaGenerica(t *testing.T) {
	jar := jarStubSinBorrado(t)
	jarJavac := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Control.java", contenido: fuenteVueltaGenericaControl}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el receptor genérico: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Control.java", fuenteVueltaGenericaControl)
	deJavac := compilaConJavac(t, javac, jarJavac, raiz, fuentes)

	desc := "(Landroid/widget/AdapterView;)Landroid/widget/BaseAdapter;"
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Control;", "borrado", desc)
	if n := cuentaCheckcast(t, suyo.Code); n != 1 {
		t.Fatalf("javac escribe %d `checkcast` en `borrado` y la sonda ya no mide lo que dice", n)
	}
	// Lo que mide este control es la candidata elegida, no el `checkcast`: estrechar `LT;` al tipo
	// instanciado del local es otro frente y con el jar sin borrado todavía no se escribe.
	if !internaEnMetodos(t, leeNuestra(t, claseEmitidaPorNombre(t, nuestras, "p/Control")),
		"Landroid/widget/AdapterView;", "getAdapter", "()LT;") {
		t.Fatalf("nuestro dex no usa la genérica `AdapterView.getAdapter:()LT;`")
	}
	if internaEnMetodos(t, leeNuestra(t, claseEmitidaPorNombre(t, nuestras, "p/Control")),
		"Landroid/widget/ListView;", "getAdapter", "()Landroid/widget/ListAdapter;") {
		t.Fatalf("el desempate se pasó de largo: eligió la concreta sobre un receptor `AdapterView`")
	}
	if !internaEnMetodos(t, claseLeidaPorNombre(t, deJavac, "Lp/Control;"),
		"Landroid/widget/AdapterView;", "getAdapter", "()Landroid/widget/Adapter;") {
		t.Fatalf("javac no usa la genérica borrada de `AdapterView.getAdapter`")
	}
}

// TestFalsificacionDeLaVueltaGenerica: apagado el desempate vuelve el error literal del usuario,
// con el descriptor sin borrar y todo; es la prueba de que lo que arregla la expresión es este paso.
func TestFalsificacionDeLaVueltaGenerica(t *testing.T) {
	jar := jarStubSinBorrado(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteVueltaGenerica}}

	anterior := desempataGenerica
	desempataGenerica = false
	defer func() { desempataGenerica = anterior }()
	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con el desempate apagado se aceptó el `getAdapter` con vuelta sin borrar")
	}
	for _, pista := range []string{
		"android/widget/ListView.getAdapter tiene 2 sobrecargas aplicables en la fase estricta a",
		"(()Landroid/widget/ListAdapter; ()LT;)",
		"ninguna es la mas especifica para este subconjunto",
	} {
		if !strings.Contains(err.Error(), pista) {
			t.Fatalf("el error no trae %q: %v", pista, err)
		}
	}
	t.Logf("error con el desempate apagado: %v", err)
}

// TestElDesempateGenericoSeAbstieneSinPrueba: fail-safe. Sin dueños, con dueños iguales o sin
// parentesco probado, `masEstrictaSobreGenerica` no elige nada y el rechazo sigue en pie; nunca se
// toma la primera candidata por el orden en que llegaron.
func TestElDesempateGenericoSeAbstieneSinPrueba(t *testing.T) {
	jar := jarStubSinBorrado(t)
	busc, err := abrirClasspath([]string{jar})
	if err != nil {
		t.Fatal(err)
	}
	defer busc.cerrar()
	ctx := &contextoJava{busc: busc}

	concreta := candidato{desc: "()Landroid/widget/ListAdapter;"}
	generica := candidato{desc: "()LT;"}
	for _, caso := range []struct {
		nombre string
		cands  []candidato
	}{
		{"sin dueño", []candidato{concreta, generica}},
		{"el mismo dueño", []candidato{
			{desc: concreta.desc, dueño: "android/widget/ListView"},
			{desc: generica.desc, dueño: "android/widget/ListView"},
		}},
		{"sin parentesco", []candidato{
			{desc: concreta.desc, dueño: "java/lang/String"},
			{desc: generica.desc, dueño: "java/lang/Runnable"},
		}},
		{"tres candidatas", []candidato{
			{desc: concreta.desc, dueño: "android/widget/ListView"},
			{desc: generica.desc, dueño: "android/widget/AdapterView"},
			{desc: "()Ljava/lang/Object;", dueño: "java/lang/Object"},
		}},
	} {
		if c, ok := ctx.masEstrictaSobreGenerica(caso.cands); ok {
			t.Errorf("%s: se eligió %q sin prueba suficiente", caso.nombre, c.desc)
		}
	}

	// y con la jerarquía real sí decide, para que el control no pase por abstención constante
	reales := []candidato{
		{desc: concreta.desc, dueño: "android/widget/ListView"},
		{desc: generica.desc, dueño: "android/widget/AdapterView"},
	}
	if c, ok := ctx.masEstrictaSobreGenerica(reales); !ok || c.desc != concreta.desc {
		t.Fatalf("con la jerarquía real no se eligió la concreta: %q %v", c.desc, ok)
	}
}
