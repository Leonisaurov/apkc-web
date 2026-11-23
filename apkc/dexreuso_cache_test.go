package main

// La caché de dex por bucket: que `guarda()` escriba **su** fichero y no la global del proyecto, que
// la vuelta siguiente lo lea y reutilice el `code_item`, y que dos buckets en paralelo no compartan
// temporal. Es el defecto que el crítico confirmó sobre la primera versión del frente: la caché del
// bucket no existía nunca y dexar dependencias pisaba `build/dexcache.bin`.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// proyectoConDependencia deja un proyecto en el directorio actual con `jar/dep.jar` (las clases de
// fuentesDepSaludo) y `build/`: es el árbol en el que corre el dexado.
func proyectoConDependencia(t *testing.T, jarSDK string) string {
	t.Helper()
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	salida, err := compilarProyectoJava(fuentesDepSaludo, jarSDK)
	if err != nil {
		t.Fatalf("el emisor no compiló la dependencia de la prueba: %v", err)
	}
	clases := map[string][]byte{}
	for _, c := range salida {
		clases[c.interno+".class"] = c.datos
	}
	jar := jarDeDependencia(t, raiz, clases)
	t.Chdir(raiz)
	DataTable = Data{}
	t.Cleanup(func() { DataTable = Data{} })
	return jar
}

func mustRead(t *testing.T, ruta string) []byte {
	t.Helper()
	d, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestLaCacheDelBucketSeEscribeEnSuFicheroYReutiliza: (a) `guarda()` escribe en `c.fichero` y en
// ningún otro sitio; (b) el fichero del bucket existe tras dexar y la caché del proyecto no; (c) la
// segunda vuelta lee ese fichero y reutiliza el `code_item` de todos sus métodos —«0 de N
// reutilizados» es exactamente el fallo que dejaba la caché sin escribir—.
func TestLaCacheDelBucketSeEscribeEnSuFicheroYReutiliza(t *testing.T) {
	jarSDK := jarDelSDK(t)
	jar := proyectoConDependencia(t, jarSDK)

	fuente := ficheroCacheDepsGo([]string{jar})
	if fuente == cacheDexFichero {
		t.Fatal("la caché del bucket se llama como la del proyecto")
	}
	clases, err := clasesDeJar(jar)
	if err != nil {
		t.Fatal(err)
	}
	if len(clases) < 3 {
		t.Fatalf("%d clases en el jar, medidas tres", len(clases))
	}

	primera := cargaCacheDexDe(fuente)
	dex, err := dexDeClasesConCache(clases, primera)
	if err != nil {
		t.Fatal(err)
	}
	if primera.resumen().reutilizados != 0 {
		t.Fatalf("una caché vacía reutilizó %d code_item", primera.resumen().reutilizados)
	}
	primera.guarda()
	if !exists(fuente) {
		t.Fatalf("guarda() no escribió %s", fuente)
	}
	if exists(cacheDexFichero) {
		t.Fatalf("guarda() de un bucket escribió la caché del proyecto (%s)", cacheDexFichero)
	}
	if exists(fuente + ".tmp") {
		t.Fatalf("quedó el temporal %s.tmp sin renombrar", fuente)
	}

	// (c) la vuelta siguiente sobre ese mismo fichero reutiliza el code_item de sus métodos
	segunda := cargaCacheDexDe(fuente)
	otro, err := dexDeClasesConCache(clases, segunda)
	if err != nil {
		t.Fatal(err)
	}
	resumen := segunda.resumen()
	if resumen.total == 0 {
		t.Fatal("la segunda vuelta no contó ningún code_item: el dex no se armó")
	}
	if resumen.reutilizados != resumen.total {
		t.Fatalf("%d de %d code_item reutilizados de %s: la caché del bucket no se leyó",
			resumen.reutilizados, resumen.total, fuente)
	}
	// el dex que reutiliza es el mismo byte a byte que el que tradujo todo
	if len(otro) != len(dex) {
		t.Fatalf("la vuelta con caché midió %d bytes y el dex completo %d", len(otro), len(dex))
	}
	for i := range otro {
		if otro[i] != dex[i] {
			t.Fatalf("dex distinto en el byte %d: la caché del bucket no da el mismo fichero", i)
		}
	}
}

// TestDexDepsDosBucketsNoCompartenTemporal: con dos buckets en paralelo cada uno escribe su caché y
// su temporal. Doce jars dan dos buckets (diez por bucket) y APKC_JOBS=2 pone dos trabajadores a la
// vez; después no puede quedar ningún `.tmp`, falta una caché ni haberse tocado la del proyecto.
func TestDexDepsDosBucketsNoCompartenTemporal(t *testing.T) {
	jarSDK := jarDelSDK(t)
	t.Setenv("APKC_JOBS", "2")
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(raiz)
	DataTable = Data{}
	t.Cleanup(func() { DataTable = Data{} })
	// un jar por clase y con bytes distintos: el dedupe por huella no puede juntarlos
	for i := 0; i < 12; i++ {
		fuente := fmt.Sprintf("package dep;\npublic class C%d {\n    public static int v() { return %d; }\n}\n", i, i)
		salida, err := compilarProyectoJava([]fuenteJava{{nombre: fmt.Sprintf("src/dep/C%d.java", i), contenido: fuente}}, jarSDK)
		if err != nil {
			t.Fatalf("el emisor no compiló C%d: %v", i, err)
		}
		clases := map[string][]byte{}
		for _, c := range salida {
			clases[c.interno+".class"] = c.datos
		}
		jarDePrueba(t, filepath.Join("jar", fmt.Sprintf("dep%d.jar", i)), clases)
	}
	// la caché del proyecto, con un contenido reconocible: dexar los buckets no puede tocarla
	if err := os.WriteFile(cacheDexFichero, []byte("proyecto"), 0o644); err != nil {
		t.Fatal(err)
	}

	jars, digests := dependenciasDexeables()
	if len(jars) != 12 {
		t.Fatalf("%d dependencias, medidas doce", len(jars))
	}
	if buckets := depBuckets(len(jars)); buckets != 2 {
		t.Fatalf("%d buckets para doce jars, medidos dos", buckets)
	}
	dexes, ran := dexDeps(21, false, true, jars, digests)
	if !ran || len(dexes) != 2 {
		t.Fatalf("ran=%v y %d dexes, esperados true y dos", ran, len(dexes))
	}
	for _, dex := range dexes {
		if _, err := readDex(mustRead(t, dex)); err != nil {
			t.Fatalf("un dex de bucket no se relee: %v", err)
		}
	}
	caches, err := filepath.Glob(filepath.Join("build", "dexcache", "deps-gocache-*.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(caches) != 2 {
		t.Fatalf("%d cachés de bucket para dos buckets: %v", len(caches), caches)
	}
	temporales, err := filepath.Glob(filepath.Join("build", "dexcache", "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporales) != 0 {
		t.Fatalf("dos buckets a la vez dejaron temporales: %v", temporales)
	}
	if dato := string(mustRead(t, cacheDexFichero)); dato != "proyecto" {
		t.Fatalf("la caché del proyecto quedó como %q", dato)
	}
}
