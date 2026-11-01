package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// pruebaDexEnDisco: la orden `apkc dex` empieza en carpetas con .class y termina en un fichero, así
// que se prueba con el arbol de verdad del studio montado en un directorio temporal. El oraculo es
// el mismo que el §22: lo que sale lo lee este fork, y lo que sale del lector casa con lo que midio
// javap. La ejecucion real del binario se comprueba aparte, abajo, con el camino de `-o -`.
func TestDexEnDiscoLeeLosArbolesYArmaElFichero(t *testing.T) {
	nombres := []string{
		"com/hola/apkcstudio/MainActivity$1.class",
		"com/hola/apkcstudio/MainActivity.class",
		"com/hola/apkcstudio/R$id.class",
		"com/hola/apkcstudio/R$layout.class",
		"com/hola/apkcstudio/R$style.class",
		"com/hola/apkcstudio/R.class",
	}
	datos := []string{clase1Fixture, clase2Fixture, clase3Fixture, clase4Fixture, clase5Fixture, clase6Fixture}

	carpeta := t.TempDir()
	raiz := filepath.Join(carpeta, "build", "classes")
	for i, n := range nombres {
		b, err := base64.StdEncoding.DecodeString(datos[i])
		if err != nil {
			t.Fatalf("fixture %d: %v", i, err)
		}
		ruta := filepath.Join(raiz, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(ruta), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	// un META-INF de una dependencia y un module-info: no son codigo nuestro y la orden los ignora
	falso := filepath.Join(raiz, "META-INF", "module-info.class")
	if err := os.MkdirAll(filepath.Dir(falso), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(falso, datosYNoClase(), 0644); err != nil {
		t.Fatal(err)
	}

	clases, err := leeClases([]string{raiz})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if len(clases) != 6 {
		t.Fatalf("%d classes read, the tree has six and two entries that are not ours", len(clases))
	}
	b, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("dexing: %v", err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("the reader refuses what the command wrote: %v", err)
	}
	if err := d.Integrity(); err != nil {
		t.Errorf("signature or checksum of the written file: %v", err)
	}
	if len(d.classes) != 6 {
		t.Errorf("%d class_defs", len(d.classes))
	}
	// 2436 con las ocho areas del §56 (2384 sin el area de depuracion, 2392 con el relleno que quito
	// el §55), **2696** con el bloque de anotaciones del §59 (8 annotation_item, 6 sets, 5 directorios y
	// sus tres entradas nuevas en el mapa) y **2680** desde el §60, que interna los type_list por
	// contenido —ocho listas en vez de nueve, las mismas que declara d8— y suelta la posicion de depuracion
	// pegada a la ultima instruccion, que es la politica del `--release` que `apkc` invoca
	if len(b) != 2680 {
		t.Errorf("the command wrote %d bytes and the measured assembly is 2680", len(b))
	}
	// el archivo por defecto del comando (`build/outputs/classes.dex`) no se crea aqui dentro: si
	// existiera en esta carpeta, la orden habria escrito donde no la llamamos
	if _, err := os.Stat(filepath.Join(carpeta, "classes.dex")); err == nil {
		t.Errorf("the command created a file nobody asked for")
	}
}

func datosYNoClase() []byte { return []byte{0xca, 0xfe, 0xba, 0xbe} }

// TestElBinarioDexa: la orden tiene que estar conectada al despachador de main, y eso solo se ve
// ejecutando el binario. Se compila el paquete y se le da el arbol del studio; si `dex` no estuviera
// en el switch, el fichero no existiria. En GOOS=js no hay proceso, asi que ahi se salta: la misma
// llamada la ejercitan las pruebas de arriba y el hospedador del navegador.
func TestElBinarioDexa(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("no subprocess in wasm: the same code is driven by TestDexEnDisco and by the browser host")
	}
	bin := filepath.Join(t.TempDir(), "apkc-dex-test")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}

	nombres := []string{
		"com/hola/apkcstudio/MainActivity$1.class",
		"com/hola/apkcstudio/MainActivity.class",
		"com/hola/apkcstudio/R$id.class",
		"com/hola/apkcstudio/R$layout.class",
		"com/hola/apkcstudio/R$style.class",
		"com/hola/apkcstudio/R.class",
	}
	datos := []string{clase1Fixture, clase2Fixture, clase3Fixture, clase4Fixture, clase5Fixture, clase6Fixture}
	raiz := filepath.Join(t.TempDir(), "classes")
	for i, n := range nombres {
		b, err := base64.StdEncoding.DecodeString(datos[i])
		if err != nil {
			t.Fatal(err)
		}
		ruta := filepath.Join(raiz, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(ruta), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, b, 0644); err != nil {
			t.Fatal(err)
		}
	}

	carpeta := t.TempDir()
	dex := filepath.Join(carpeta, "classes.dex")
	salida, err := exec.Command(bin, "dex", "-o", dex, raiz).CombinedOutput()
	if err != nil {
		t.Fatalf("apkc dex: %v\n%s", err, salida)
	}
	b, err := os.ReadFile(dex)
	if err != nil {
		t.Fatalf("the command left no file: %v\n%s", err, salida)
	}
	// 2680 desde el §60 (2696 con el bloque de anotaciones del §59, 2436 con el área de depuración del
	// §56, 2384 sin ella, 2392 con el relleno que quitó el §55)
	if len(b) != 2680 {
		t.Errorf("the binary wrote %d bytes, the assembly measured is 2680", len(b))
	}
	if _, err := readDex(b); err != nil {
		t.Errorf("the binary's own output does not read back: %v", err)
	}

	// y el aviso de que un switch escrito despues de las carpetas no se mira: la misma trampa que
	// `sign` ya rechaza, aqui con las rutas
	salida, err = exec.Command(bin, "dex", raiz, "-o", dex).CombinedOutput()
	if err == nil {
		t.Errorf("`apkc dex %s -o %s` was accepted: the switch after the folder would be read as a folder", raiz, dex)
	} else if !strings.Contains(string(salida), "the switches go before the folders") {
		t.Errorf("the refusal does not say why: %s", salida)
	}
}
