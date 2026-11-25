package main

// Las pruebas del índice del android.jar entre pasadas (I5, jarindice.go): que el directorio central
// propio dice lo mismo que archive/zip, que la caché se reutiliza cuando el jar no cambió y se tira
// cuando cambió, y cuánto se ahorra el build con ella.

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// jarDePrueba escribe un zip con las entradas dadas: las `.class` con deflate y el resto guardadas,
// para que el lector propio pase por los dos métodos de compresión. Con metodoFijo se fuerza un método
// para todas las entradas: zip.Store hace que el tamaño del jar dependa solo de la longitud de los
// datos y no de su contenido, que es lo que necesita la prueba que cambia bytes sin cambiar tamaño.
func jarDePrueba(t *testing.T, ruta string, entradas map[string][]byte, metodoFijo ...uint16) {
	t.Helper()
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	nombres := make([]string, 0, len(entradas))
	for nombre := range entradas {
		nombres = append(nombres, nombre)
	}
	sort.Strings(nombres)
	for _, nombre := range nombres {
		metodo := zip.Store
		if strings.HasSuffix(nombre, ".class") {
			metodo = zip.Deflate
		}
		if len(metodoFijo) > 0 {
			metodo = metodoFijo[0]
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: nombre, Method: metodo})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entradas[nombre]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// leeEntrada saca los bytes de una clase por el offset del índice, que es el camino de la caché.
func leeEntrada(t *testing.T, b *buscadorDeClases, interno string) []byte {
	t.Helper()
	e, hay := b.indice[interno]
	if !hay {
		t.Fatalf("%s no está en el índice", interno)
	}
	datos, err := datosDeLaEntrada(b.archivo, e)
	if err != nil {
		t.Fatalf("la entrada %s no se pudo leer por su offset: %v", interno, err)
	}
	return datos
}

// TestIndiceDelJarSeReutilizaYSeInvalida cierra la mitad de I5 que no es el cronómetro: la vuelta sin
// cambios lee el índice de build/jarindice.bin sin volver a parsear el directorio central, otra fecha
// con los mismos bytes no la tira, y un jar con otros bytes sí. El contador del parser es lo único que
// distingue «salió de la caché» de «volvió a parsear»: por fuera se comportan igual.
func TestIndiceDelJarSeReutilizaYSeInvalida(t *testing.T) {
	raiz := t.TempDir()
	t.Chdir(raiz)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(raiz, "android.jar")
	// Store: el tamaño tiene que depender solo de la longitud de los datos, para que la reescritura
	// de abajo cambie el contenido sin cambiar el tamaño y la huella tenga que mirar el CRC.
	jarDePrueba(t, jar, map[string][]byte{
		"p/A.class":            {1, 2, 3, 4, 5},
		"p/B.class":            {6, 7, 8},
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\n"),
	}, zip.Store)

	b, err := abrirBuscador(jar)
	if err != nil {
		t.Fatalf("el jar de la prueba no abre: %v", err)
	}
	if !b.hayEnElJar("p/A") || !b.hayEnElJar("p/B") {
		t.Fatalf("el índice se dejó clases del jar: %v", b.indice)
	}
	if b.hayEnElJar("p/Z") || b.hayEnElJar("META-INF/MANIFEST") {
		t.Fatal("el índice tiene entradas que no son clases del jar")
	}
	if datos := leeEntrada(t, b, "p/A"); !bytes.Equal(datos, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("la entrada leída por offset no son sus bytes: %v", datos)
	}
	b.cerrar()
	if _, err := os.Stat(ficheroIndiceDelJar(jar)); err != nil {
		t.Fatalf("la primera vuelta no dejó el índice en %s: %v", ficheroIndiceDelJar(jar), err)
	}

	// la vuelta con el mismo jar no parsea nada: la sirve el fichero, y las clases salen igual
	antes := parseosDelDirectorio
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("la segunda vuelta no abre: %v", err)
	}
	if parseosDelDirectorio != antes {
		t.Fatal("con el mismo jar la vuelta siguiente volvió a parsear el directorio central")
	}
	if !b.hayEnElJar("p/A") || !b.hayEnElJar("p/B") {
		t.Fatalf("el índice cacheado no tiene las clases del jar: %v", b.indice)
	}
	if datos := leeEntrada(t, b, "p/A"); !bytes.Equal(datos, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("la entrada cacheada no son sus bytes: %v", datos)
	}
	b.cerrar()

	// la fecha del jar no es contenido: moverla no puede reindexar ni, menos aún, recompilar
	fija := time.Unix(1700000000, 0)
	if err := os.Chtimes(jar, fija, fija); err != nil {
		t.Fatal(err)
	}
	antes = parseosDelDirectorio
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("el jar con otra fecha no abre: %v", err)
	}
	if parseosDelDirectorio != antes {
		t.Fatal("otra fecha con los mismos bytes no puede tirar el índice")
	}
	b.cerrar()

	// otros bytes con el mismo tamaño sí: se reescribe una entrada con la misma longitud y la caché
	// no puede valer, porque su CRC vive en el directorio central
	antesDe, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, jar, map[string][]byte{
		"p/A.class":            {9, 9, 9, 9, 9},
		"p/B.class":            {6, 7, 8},
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\n"),
	}, zip.Store)
	despuesDe, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	if antesDe.Size() != despuesDe.Size() {
		t.Fatalf("la prueba necesita el mismo tamaño para probar el contenido: %d y %d",
			antesDe.Size(), despuesDe.Size())
	}
	antes = parseosDelDirectorio
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("el jar con otros bytes no abre: %v", err)
	}
	if parseosDelDirectorio == antes {
		t.Fatal("un jar con los mismos bytes en tamaño y otra entrada no puede reutilizar el índice")
	}
	if datos := leeEntrada(t, b, "p/A"); !bytes.Equal(datos, []byte{9, 9, 9, 9, 9}) {
		t.Fatalf("la entrada reindexada no son los bytes nuevos: %v", datos)
	}
	b.cerrar()

	// y el índice en disco es el del jar nuevo
	tamano, digest, err := huellaDelJar(jar)
	if err != nil {
		t.Fatal(err)
	}
	indice, ok := leerIndiceDelJar(jar, tamano, digest)
	if !ok {
		t.Fatal("el índice reindexado no se puede volver a leer")
	}
	if len(indice) != 2 {
		t.Fatalf("el índice en disco no es el del jar nuevo: %v", indice)
	}

	// otro jar con otra entrada: se ve en el índice y se reindexa
	jarDePrueba(t, jar, map[string][]byte{
		"p/A.class": {1, 2, 3, 4, 5},
		"p/C.class": {9},
	})
	antes = parseosDelDirectorio
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("el tercer jar no abre: %v", err)
	}
	if parseosDelDirectorio == antes {
		t.Fatal("un jar con otras entradas tiene que reindexarse")
	}
	if b.hayEnElJar("p/B") {
		t.Fatal("el índice sigue teniendo una clase que ya no está en el jar")
	}
	if !b.hayEnElJar("p/C") {
		t.Fatal("el índice no tiene la clase que se añadió al jar")
	}
	b.cerrar()
}

// TestJarDePruebaMismoTamanoConDistintoContenido falsifica el fixture de la prueba anterior: fuerza
// zip.Store, así que un jar con entradas de la misma longitud y bytes distintos tiene que medir
// exactamente lo mismo. Si alguien vuelve a dejar que el deflate decida el tamaño, esta prueba falla
// antes de que la de arriba pierda su sentido (o vuelva a fallar por los 2 bytes de la compresión).
func TestJarDePruebaMismoTamanoConDistintoContenido(t *testing.T) {
	raiz := t.TempDir()
	original := filepath.Join(raiz, "original.jar")
	reescrito := filepath.Join(raiz, "reescrito.jar")
	jarDePrueba(t, original, map[string][]byte{
		"p/A.class":            {1, 2, 3, 4, 5},
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\n"),
	}, zip.Store)
	jarDePrueba(t, reescrito, map[string][]byte{
		"p/A.class":            {9, 9, 9, 9, 9},
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\n"),
	}, zip.Store)

	antes, err := os.Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	despues, err := os.Stat(reescrito)
	if err != nil {
		t.Fatal(err)
	}
	if antes.Size() != despues.Size() {
		t.Fatalf("el fixture volvió a generar tamaños distintos: %d y %d", antes.Size(), despues.Size())
	}
	// y el contenido sí tiene que cambiar: si no, la huella no tendría nada que invalidar
	_, crcOriginal, err := huellaDelJar(original)
	if err != nil {
		t.Fatal(err)
	}
	_, crcReescrito, err := huellaDelJar(reescrito)
	if err != nil {
		t.Fatal(err)
	}
	if crcOriginal == crcReescrito {
		t.Fatal("el fixture cambió el tamaño del jar pero no su contenido")
	}
}

// TestDirectorioCentralDelJarContraArchiveZip comprueba el lector propio contra archive/zip sobre el
// android.jar de verdad: las mismas entradas, el mismo método, tamaño y CRC, y las mismas clases.
//
// El contrato del índice es «todas las clases más la única entrada que no es una clase, la tabla de
// recursos del framework», porque `check` la lee para enlazar el res/ del proyecto sin abrir el zip.
// Por eso la comparación es del conjunto completo y no solo de las clases: filtrar ambos lados a
// `.class` dejaba caer el marco sin que se notara (5721 contra 5720), y añadir cualquier otra entrada
// al índice debe romper esta prueba a propósito, que es un cambio de contrato.
func TestDirectorioCentralDelJarContraArchiveZip(t *testing.T) {
	ruta := jarDelSDK(t)
	propio, err := directorioCentralDelJar(ruta)
	if err != nil {
		t.Fatalf("el lector propio no recorre el jar: %v", err)
	}
	// El marco se afirma aparte: si se cae del índice por cualquier motivo, esta prueba falla por
	// esta vía y no solo por el conteo, que es lo que pasó cuando `check` dejó de verlo.
	if _, hay := propio[entradaMarcoDelJar]; !hay {
		t.Fatalf("el índice no guarda %q: `check` volvería a recorrer el jar por el marco", entradaMarcoDelJar)
	}
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	esperadas := map[string]*zip.File{}
	for _, f := range zr.File {
		interno, ok := recortaClase(f.Name)
		if !ok && f.Name == entradaMarcoDelJar {
			interno, ok = f.Name, true
		}
		if !ok {
			continue
		}
		if _, ya := esperadas[interno]; !ya {
			esperadas[interno] = f
		}
	}
	if len(propio) != len(esperadas) {
		t.Fatalf("el lector propio ve %d entradas y archive/zip %d", len(propio), len(esperadas))
	}
	nombres := make([]string, 0, len(esperadas))
	for interno := range esperadas {
		nombres = append(nombres, interno)
	}
	sort.Strings(nombres)
	for _, interno := range nombres {
		f := esperadas[interno]
		e, hay := propio[interno]
		if !hay {
			t.Fatalf("%s está en el zip y no en el índice", interno)
		}
		if e.metodo != f.Method || int64(e.comprimido) != int64(f.CompressedSize64) || e.crc != f.CRC32 {
			t.Fatalf("%s: el índice dice método %d, %d bytes y CRC %#x; el zip %d, %d y %#x",
				interno, e.metodo, e.comprimido, e.crc, f.Method, f.CompressedSize64, f.CRC32)
		}
	}
	// y una muestra de las clases, leída por el offset del índice, es byte a byte la del zip
	archivo, err := os.Open(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer archivo.Close()
	for i, interno := range nombres {
		if i == 24 {
			break
		}
		datos, err := datosDeLaEntrada(archivo, propio[interno])
		if err != nil {
			t.Fatalf("%s: %v", interno, err)
		}
		rc, err := esperadas[interno].Open()
		if err != nil {
			t.Fatal(err)
		}
		esperado, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(datos, esperado) {
			t.Fatalf("%s: el lector propio devuelve %d bytes y archive/zip %d, y no son los mismos",
				interno, len(datos), len(esperado))
		}
	}
}

// rutaDelJar busca el android.jar sin abrirlo: la medición tiene que poder medir el primer
// `abrirBuscador` del proceso, que es lo que paga un build de la pestaña, y `jarDelSDK` ya abriría
// uno antes y calentaría el runtime.
func rutaDelJar(t *testing.T) string {
	t.Helper()
	candidatos := []string{os.Getenv("APKC_ANDROID_JAR"), homeDir() + "/Android/platforms/android-34/android.jar", "/sdk/android.jar"}
	for _, r := range candidatos {
		if r == "" {
			continue
		}
		if _, err := os.Stat(r); err == nil {
			return r
		}
	}
	t.Skipf("sin android.jar que medir (%v): el emisor resuelve los nombres contra el SDK", candidatos)
	return ""
}

// TestIndiceDelJarMideElAhorro es la medición de I5: el camino de archive/zip que se pagaba en cada
// build, el lector propio con la caché vacía y con la caché puesta, y el proyecto mínimo compilado
// en los dos casos. Tres vueltas; la primera de cada camino es la que paga el calentamiento del
// runtime, así que el registro lleva las tres y la mediana.
func TestIndiceDelJarMideElAhorro(t *testing.T) {
	ruta := rutaDelJar(t)
	raiz := t.TempDir()
	t.Chdir(raiz)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	borra := func() {
		t.Helper()
		if err := os.Remove(ficheroIndiceDelJar(ruta)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	mide := func(abrir func() (*buscadorDeClases, error)) time.Duration {
		t.Helper()
		comienzo := time.Now()
		b, err := abrir()
		d := time.Since(comienzo)
		if err != nil {
			t.Fatalf("abrir el jar: %v", err)
		}
		b.cerrar()
		return d
	}
	proyecto := []fuenteJava{
		{nombre: "src/com/hola/apkcstudio/MainActivity.java", contenido: fuenteMainActivityStudio},
		{nombre: "src/com/hola/apkcstudio/R.java", contenido: fuenteRStudio},
	}
	mideProyecto := func() (time.Duration, []claseEmitida) {
		t.Helper()
		comienzo := time.Now()
		clases, err := compilarProyectoJava(proyecto, ruta)
		d := time.Since(comienzo)
		if err != nil {
			t.Fatalf("el proyecto mínimo no compila: %v", err)
		}
		return d, clases
	}
	var comoHoy, enFrio, enCaliente, proyectoFrio, proyectoCaliente []time.Duration
	var frias, calientes []claseEmitida
	for i := 0; i < 3; i++ {
		borra()
		comoHoy = append(comoHoy, mide(func() (*buscadorDeClases, error) { return abrirBuscadorConZip(ruta) }))
		borra()
		enFrio = append(enFrio, mide(func() (*buscadorDeClases, error) { return abrirBuscador(ruta) }))
		enCaliente = append(enCaliente, mide(func() (*buscadorDeClases, error) { return abrirBuscador(ruta) }))
		borra()
		d, clases := mideProyecto()
		proyectoFrio = append(proyectoFrio, d)
		if i == 0 {
			frias = clases
		}
		d, clases = mideProyecto()
		proyectoCaliente = append(proyectoCaliente, d)
		if i == 0 {
			calientes = clases
		}
		t.Logf("vuelta %d: archive/zip %.1f ms; lector propio con la caché vacía %.1f ms; con la caché puesta %.1f ms; proyecto mínimo %.1f y %.1f ms",
			i+1, milisegundos(comoHoy[i]), milisegundos(enFrio[i]), milisegundos(enCaliente[i]),
			milisegundos(proyectoFrio[i]), milisegundos(proyectoCaliente[i]))
	}
	t.Logf("medianas: archive/zip %.1f ms; lector propio con la caché vacía %.1f ms; con la caché puesta %.1f ms; proyecto mínimo %.1f y %.1f ms",
		milisegundos(mediana(comoHoy)), milisegundos(mediana(enFrio)), milisegundos(mediana(enCaliente)),
		milisegundos(mediana(proyectoFrio)), milisegundos(mediana(proyectoCaliente)))
	if mediana(enCaliente) >= mediana(comoHoy) {
		t.Errorf("la caché no ahorra nada: %.1f ms frente a %.1f ms",
			milisegundos(mediana(enCaliente)), milisegundos(mediana(comoHoy)))
	}

	// el proyecto mínimo compilado dos veces: con la caché vacía y con la caché puesta, las clases
	// emitidas son las mismas byte a byte, que es lo que el índice no puede cambiar
	if len(frias) != len(calientes) {
		t.Fatalf("la caché cambió el número de clases: %d frente a %d", len(frias), len(calientes))
	}
	for i := range frias {
		if frias[i].interno != calientes[i].interno || !bytes.Equal(frias[i].datos, calientes[i].datos) {
			t.Fatalf("la caché cambió la clase %s", frias[i].interno)
		}
	}
}

func mediana(v []time.Duration) time.Duration {
	copia := append([]time.Duration(nil), v...)
	sort.Slice(copia, func(i, j int) bool { return copia[i] < copia[j] })
	return copia[len(copia)/2]
}

func milisegundos(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// TestClasesAlDiaSigueAlContenidoDelJar cierra el defecto del crítico de I5: cambiar el classpath
// tiene que invalidar las clases reutilizadas de I3 aunque el jar tenga el mismo tamaño, y moverle la
// fecha no puede invalidarlas, porque la fecha no es contenido y en OPFS la del stat es sintética.
func TestClasesAlDiaSigueAlContenidoDelJar(t *testing.T) {
	raiz := t.TempDir()
	t.Chdir(raiz)
	jar := filepath.Join(raiz, "android.jar")
	jarDePrueba(t, jar, map[string][]byte{"p/X.class": {1, 2, 3, 4}})
	anterior := androidJar
	androidJar = jar
	DataTable = Data{}
	t.Cleanup(func() {
		androidJar = anterior
		DataTable = Data{}
	})
	// el estado que deja una compilación, sin compilar: un fuente registrado y una clase en
	// build/classes. Lo que se prueba es la decisión, no el emisor.
	pon(t, raiz, "src/p/A.java", "package p;\npublic class A {}\n")
	pon(t, raiz, "build/classes/p/A.class", "\xca\xfe\xba\xbe")
	updateRecords([]string{"src/p/A.java"})
	DataTable.ClassRoster = classRosterSHA()
	DataTable.AndroidJar = huellaDelJarTexto(jar)
	saveTable()
	huella := DataTable.AndroidJar
	if huella == "" {
		t.Fatal("el jar de la prueba no deja huella")
	}
	if !clasesAlDia(huella) {
		t.Fatal("con el estado que deja una compilación, las clases tienen que estar al día")
	}

	// la fecha no entra: el mismo jar con otra fecha no puede costar una recompilación entera
	otra := time.Unix(1700000000, 0)
	if err := os.Chtimes(jar, otra, otra); err != nil {
		t.Fatal(err)
	}
	if huellaDelJarTexto(jar) != huella {
		t.Fatal("mover la fecha del jar no puede cambiar su huella")
	}
	if !clasesAlDia(huellaDelJarTexto(jar)) {
		t.Fatal("un jar con los mismos bytes y otra fecha no puede invalidar las clases de I3")
	}

	// otros bytes con el mismo tamaño sí, que es el caso que faltaba: el CRC de la entrada vive en
	// el directorio central, así que la huella cambia aunque el fichero mida lo mismo
	antes, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, jar, map[string][]byte{"p/X.class": {9, 9, 9, 9}})
	despues, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	if antes.Size() != despues.Size() {
		t.Fatalf("la prueba necesita el mismo tamaño para probar el contenido: %d y %d",
			antes.Size(), despues.Size())
	}
	nueva := huellaDelJarTexto(jar)
	if nueva == huella {
		t.Fatal("otra entrada con el mismo tamaño tiene que cambiar la huella del jar")
	}
	if clasesAlDia(nueva) {
		t.Fatal("un jar con otros bytes tiene que invalidar las clases reutilizadas de I3")
	}
}

// y que el nombre que se imprime en los fallos del emisor sigue nombrando el jar, no el índice
func TestIndiceDelJarNombraLaEntradaQueFalta(t *testing.T) {
	raiz := t.TempDir()
	t.Chdir(raiz)
	jar := filepath.Join(raiz, "android.jar")
	jarDePrueba(t, jar, map[string][]byte{"p/A.class": {1}})
	b, err := abrirBuscador(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer b.cerrar()
	if _, err := b.claseDelJar("p/NoExiste"); err == nil {
		t.Fatal("una clase que no está en el jar no puede resolverse")
	} else if !strings.Contains(err.Error(), "p/NoExiste") {
		t.Errorf("el error no nombra la clase: %v", err)
	}
	if _, err := b.claseDelJar("p/A"); err == nil {
		t.Fatal("los bytes de la prueba no son una clase y el lector tiene que rechazarlos")
	} else if !strings.Contains(err.Error(), "p/A") {
		t.Errorf("el error no nombra la entrada: %v", err)
	}
}
