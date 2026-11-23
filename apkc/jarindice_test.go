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
// para que el lector propio pase por los dos métodos de compresión.
func jarDePrueba(t *testing.T, ruta string, entradas map[string][]byte) {
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

// rompeElFinDelDirectorio destruye la firma del fin del directorio central sin tocar el tamaño: es la
// forma de preguntar si un `abrirBuscador` leyó el zip o se creyó la caché.
func rompeElFinDelDirectorio(t *testing.T, ruta string) {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.LastIndex(datos, firmaFinDirectorio)
	if i < 0 {
		t.Fatal("el jar de la prueba no tiene fin de directorio")
	}
	copy(datos[i:i+4], []byte("\x00\x00\x00\x00"))
	if err := os.WriteFile(ruta, datos, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIndiceDelJarSeReutilizaYSeInvalida cierra la mitad de I5 que no es el cronómetro: la vuelta sin
// cambios lee el índice de build/jarindice.bin, y un jar con otros bytes —u otra fecha— lo tira.
func TestIndiceDelJarSeReutilizaYSeInvalida(t *testing.T) {
	raiz := t.TempDir()
	t.Chdir(raiz)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(raiz, "android.jar")
	jarDePrueba(t, jar, map[string][]byte{
		"p/A.class":            {1, 2, 3, 4, 5},
		"p/B.class":            {6, 7, 8},
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\n"),
	})

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
	if _, err := os.Stat(ficheroIndiceDelJar); err != nil {
		t.Fatalf("la primera vuelta no dejó el índice en %s: %v", ficheroIndiceDelJar, err)
	}

	// otro jar con otra entrada: el tamaño cambia, así que la caché no vale y hay que reindexar
	jarDePrueba(t, jar, map[string][]byte{
		"p/A.class": {1, 2, 3, 4, 5},
		"p/C.class": {9},
	})
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("el segundo jar no abre: %v", err)
	}
	if b.hayEnElJar("p/B") {
		t.Fatal("el índice sigue teniendo una clase que ya no está en el jar")
	}
	if !b.hayEnElJar("p/C") {
		t.Fatal("el índice no tiene la clase que se añadió al jar")
	}
	b.cerrar()
	indice, ok := leerIndiceDelJar(jar)
	if !ok {
		t.Fatal("el índice reindexado no se puede volver a leer")
	}
	if _, hay := indice["p/C"]; !hay {
		t.Fatalf("el índice en disco no se reescribió con el jar nuevo: %v", indice)
	}
	if _, hay := indice["p/B"]; hay {
		t.Fatal("el índice en disco conserva la clase que se borró del jar")
	}

	// con la misma huella (mismo tamaño y misma fecha) la vuelta siguiente no necesita el zip: se
	// rompe su fin de directorio y, aun así, el índice abre y lee las clases. La fecha se fija a un
	// segundo exacto porque el sistema de ficheros simulado del hospedador guarda milisegundos y un
	// `Chtimes` con la fecha que él mismo dio no siempre la devuelve igual.
	fija := time.Unix(1700000000, 0)
	if err := os.Chtimes(jar, fija, fija); err != nil {
		t.Fatal(err)
	}
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("el jar con la fecha fija no abre: %v", err)
	}
	b.cerrar()
	rompeElFinDelDirectorio(t, jar)
	if err := os.Chtimes(jar, fija, fija); err != nil {
		t.Fatal(err)
	}
	b, err = abrirBuscador(jar)
	if err != nil {
		t.Fatalf("con la caché buena el jar no debería necesitar su fin de directorio: %v", err)
	}
	if !b.hayEnElJar("p/A") || !b.hayEnElJar("p/C") {
		t.Fatalf("el índice cacheado no tiene las clases del jar: %v", b.indice)
	}
	if datos := leeEntrada(t, b, "p/A"); !bytes.Equal(datos, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("la entrada cacheada no son sus bytes: %v", datos)
	}
	b.cerrar()

	// y la fecha es parte de la huella: el mismo fichero con otra fecha no reutiliza nada, así que
	// con el zip roto abrir tiene que fallar
	otra := fija.Add(time.Second)
	if err := os.Chtimes(jar, otra, otra); err != nil {
		t.Fatal(err)
	}
	if _, err := abrirBuscador(jar); err == nil {
		t.Fatal("con otra fecha y el zip roto, el índice viejo no puede servir")
	}
}

// TestDirectorioCentralDelJarContraArchiveZip comprueba el lector propio contra archive/zip sobre el
// android.jar de verdad: las mismas entradas, el mismo método, tamaño y CRC, y las mismas clases.
func TestDirectorioCentralDelJarContraArchiveZip(t *testing.T) {
	ruta := jarDelSDK(t)
	propio, err := directorioCentralDelJar(ruta)
	if err != nil {
		t.Fatalf("el lector propio no recorre el jar: %v", err)
	}
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	esperadas := map[string]*zip.File{}
	for _, f := range zr.File {
		interno, ok := recortaClase(f.Name)
		if !ok {
			continue
		}
		if _, ya := esperadas[interno]; !ya {
			esperadas[interno] = f
		}
	}
	if len(propio) != len(esperadas) {
		t.Fatalf("el lector propio ve %d clases y archive/zip %d", len(propio), len(esperadas))
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
		if err := os.Remove(ficheroIndiceDelJar); err != nil && !os.IsNotExist(err) {
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
