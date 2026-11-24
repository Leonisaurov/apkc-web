//go:build !js

package main

import (
	"strings"
	"testing"
)

// La paridad con javac 17 de javaemisor_appadapter_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_appadapter_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestAppAdapterDeApkShareCompilaContraJavac17: los cuatro fuentes reales de ApkShare que rodean a
// `AppAdapter.java` se compilan sin un solo error —el `check` deja de decir «no se resuelve» y su
// `emisor` deja de ser falso— y el constructor de `AppAdapter`, que es donde estaban la línea 30
// (`super(context, R.layout.app_layout, apps)`) y la 33 (`apps_filter = new ArrayList<>(apps)`), sale
// instrucción a instrucción como el de javac 17 `--release 8`.
//
// El puente `getItem:(I)Ljava/lang/Object;` que javac sintetiza para el retorno covariante sobre
// `ArrayAdapter<App>` y el `check-cast` tras el `List.get` borrado (`AppAdapter.java:73`) ya se
// emiten (§251), y el dex de las seis clases de ApkShare es byte a byte el de javac 17
// (`javaemisor_ambito_local_test.go`).
func TestAppAdapterDeApkShareCompilaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesAppAdapterShare()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes de ApkShare: %v", err)
	}

	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	const desc = "(Landroid/content/Context;Ljava/util/List;Landroid/content/pm/PackageManager;)V"
	nuestroInit := metodoDeClaseEmitida(t, nuestras, "share/apps/util/AppAdapter", "<init>", desc)
	suInit := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/AppAdapter;", "<init>", desc)
	if err := comparaInstrucciones(nuestroInit.Code, suInit.Code); err != nil {
		t.Fatalf("el constructor de AppAdapter no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	// el descriptor del constructor es el que dice que `super(context, int, List<T>)` se resolvió
	// contra el `ArrayAdapter<T>` genérico y no contra otra sobrecarga: javac escribe el mismo.
	if !strings.Contains(desc, "Ljava/util/List;") {
		t.Fatalf("el descriptor del constructor no lleva el `List` borrado: %s", desc)
	}
}

// TestLaSubidaDeUnaReferenciaAlEscribirUnCampoDexaComoJavac17: la ruta nueva de `cabeEn` medida sobre
// un dex entero byte a byte. Cubre `this.filtrada = new ArrayList<>(lista)` (la forma de
// `AppAdapter.java:33`), `c.cosas = nuevas` (receptor distinto de `this`) y las dos condiciones
// booleanas de `condicionBranca`.
func TestLaSubidaDeUnaReferenciaAlEscribirUnCampoDexaComoJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteSondaSubida}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la subida: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaSubida)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	comparaDexByteAByte(t, dexDeNuestrasClases(t, nuestras), dexDeLasDeJavac(t, deJavac), volcadoDeClases(nuestras))
}

// TestElAccesorDeUnaAnonimaAceptaLaSubida: el tercer sitio de `cabeEn`, la escritura por `access$NN2`
// desde una anónima. Sin la ruta nueva el emisor paraba en ella; con ella, el dex de la sonda es el de
// javac 17 byte a byte, el de la anónima incluida.
func TestElAccesorDeUnaAnonimaAceptaLaSubida(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteSondaAccesor}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la subida por el accesor: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaAccesor)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	comparaDexByteAByte(t, dexDeNuestrasClases(t, nuestras), dexDeLasDeJavac(t, deJavac), volcadoDeClases(nuestras))
}
