//go:build !js

package main

import (
	"bytes"
	"testing"
)

// La paridad con javac 17 de javaemisor_vuelta_covariante_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_vuelta_covariante_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestElDexDeLaVueltaCovarianteEsElDeJavac17: la sonda dexa como la de javac 17 `--release 8
// -g:lines,source`, byte a byte. El dex es el oráculo que sí ve el descriptor elegido: la firma del
// `invokevirtual` lleva el retorno, así que si el desempate escogiera el `Adapter` de `AdapterView` el
// `method_id` sería otro y el dex dejaría de coincidir. Y como el receptor de `getItem` es `ListAdapter`,
// que no declara `getItem` —lo hereda de `Adapter`—, la coincidencia también prueba el recorrido de
// superinterfaces de `candidatosDelJar`.
func TestElDexDeLaVueltaCovarianteEsElDeJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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

// TestElDesempatePorVueltaEnUnaJerarquiaPropia: `trae()` tiene dos declaraciones visibles desde `Hija`
// —la suya, que devuelve `String`, y la heredada, que devuelve `Object`— y gana la estrecha. Antes del
// desempate esto era ambiguo y `usa()` no compilaba.
func TestElDesempatePorVueltaEnUnaJerarquiaPropia(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
