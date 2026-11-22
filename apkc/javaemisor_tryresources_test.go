package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// tryResourcesFuente: la sonda de un recurso. El oráculo es javac 17 (`javac -g:lines,source`
// de /home/leonisauro/opt/jdk17), no la equivalencia con un try/finally escrito a mano: javac
// guarda la excepcion del cuerpo en un local, cierra el recurso dentro de un segundo rango
// protegido y, si `close()` tambien falla, le añade la segunda con `Throwable.addSuppressed`.
const tryResourcesFuente = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable c) throws Exception {
    try (Closeable r = c) { use(r); }
  }
}
`

// tryResourcesJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
const tryResourcesJavac17 = "yv66vgAAAD0AIQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAA1JlcwEAA3VzZQEAFihMamF2YS9pby9DbG9zZWFibGU7KVYLAA4ADwcAEAwAEQAGAQARamF2YS9pby9DbG9zZWFibGUBAAVjbG9zZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUKABIAFQwAFgAXAQANYWRkU3VwcHJlc3NlZAEAGChMamF2YS9sYW5nL1Rocm93YWJsZTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQANU3RhY2tNYXBUYWJsZQEACkV4Y2VwdGlvbnMHAB4BABNqYXZhL2xhbmcvRXhjZXB0aW9uAQAKU291cmNlRmlsZQEACFJlcy5qYXZhACEACAACAAAAAAADAAEABQAGAAEAGAAAAB0AAQABAAAABSq3AAGxAAAAAQAZAAAABgABAAAAAgAIAAsADAABABgAAAAZAAAAAQAAAAGxAAAAAQAZAAAABgABAAAAAwAIABoADAACABgAAACFAAIABAAAACoqTCu4AAcrxgAiK7kADQEApwAZTSvGABIruQANAQCnAAlOLC22ABQsv7EAAgACAAYAEwASABgAHgAhABIAAgAZAAAACgACAAAABQApAAYAGwAAACkABP8AEwACBwAOBwAOAAEHABL/AA0AAwcADgcADgcAEgABBwASBfkAAQAcAAAABAABAB0AAQAfAAAAAgAg"

// dexDeFuenteUnica compila el fuente con el emisor, dex el `.class` que sale y dex tambien el
// `.class` que javac dejó como oráculo, para compararlos byte a byte.
func dexDeFuenteUnica(t *testing.T, fuente, javac string) (nuestro, suyo []byte) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("salieron %d clases: %s", len(salida), nombresDe(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(javac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
	}
	nuestro, err = dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	suyo, err = dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	return nuestro, suyo
}

// TestTryWithResourcesAddSuppressed: el dex del emisor es byte a byte el de javac 17, con dos
// try_item (el cuerpo y el cierre del camino excepcional) y una llamada a Throwable.addSuppressed.
func TestTryWithResourcesAddSuppressed(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesFuente, tryResourcesJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	td := newDexTables()
	cf, err := readClassFull(func() []byte {
		salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: tryResourcesFuente}}, jarDelSDK(t))
		if err != nil {
			t.Fatal(err)
		}
		return salida[0].datos
	}())
	if err != nil {
		t.Fatal(err)
	}
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatal(err)
	}
	td.orderTables()
	encontro := false
	for _, m := range td.methods {
		if m.owner == "Ljava/lang/Throwable;" && m.name == "addSuppressed" && m.desc == "(Ljava/lang/Throwable;)V" {
			encontro = true
		}
	}
	if !encontro {
		t.Error("no se internó Throwable.addSuppressed")
	}
}

// dos recursos: cada uno abre un nivel anidado, se cierran en orden inverso y cada manejador añade
// la excepción del cierre con `addSuppressed`. El oráculo es javac 17, igual que en el caso de uno
// solo: el dex del emisor tiene que salir byte a byte el suyo.
const tryResourcesDos = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable a, Closeable b) throws Exception {
    try (Closeable x = a; Closeable y = b) { use(x); use(y); }
  }
}
`

// tryResourcesDosJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
// Medido con `javap -c -p -v`: las declaraciones en 0-3, el cuerpo protegido [4,12) -> 25, el cierre
// de `y` en [31,37) -> 40, el rango exterior [2,52) -> 65 y su cierre en [70,76) -> 79; el manejador
// exterior hace `astore_3`, el hueco que dejó `y`, y `locals=6`. La tabla de excepciones va de dentro
// hacia fuera.
const tryResourcesDosJavac17 = "yv66vgAAAD0AIgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAA1JlcwEAA3VzZQEAFihMamF2YS9pby9DbG9zZWFibGU7KVYLAA4ADwcAEAwAEQAGAQARamF2YS9pby9DbG9zZWFibGUBAAVjbG9zZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUKABIAFQwAFgAXAQANYWRkU3VwcHJlc3NlZAEAGChMamF2YS9sYW5nL1Rocm93YWJsZTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQApKExqYXZhL2lvL0Nsb3NlYWJsZTtMamF2YS9pby9DbG9zZWFibGU7KVYBAA1TdGFja01hcFRhYmxlAQAKRXhjZXB0aW9ucwcAHwEAE2phdmEvbGFuZy9FeGNlcHRpb24BAApTb3VyY2VGaWxlAQAIUmVzLmphdmEAIQAIAAIAAAAAAAMAAQAFAAYAAQAYAAAAHQABAAEAAAAFKrcAAbEAAAABABkAAAAGAAEAAAACAAgACwAMAAEAGAAAABkAAAABAAAAAbEAAAABABkAAAAGAAEAAAADAAgAGgAbAAIAGAAAAO8AAgAGAAAAWipNK04suAAHLbgABy3GACctuQANAQCnAB46BC3GABUtuQANAQCnAAw6BRkEGQW2ABQZBL8sxgAkLLkADQEApwAbTizGABQsuQANAQCnAAs6BC0ZBLYAFC2/sQAEAAQADAAZABIAHwAlACgAEgACADQAQQASAEYATABPABIAAgAZAAAACgACAAAABQBZAAYAHAAAAFMACP8AGQAEBwAOBwAOBwAOBwAOAAEHABL/AA4ABQcADgcADgcADgcADgcAEgABBwASCPkAAkwHABL/AA0ABAcADgcADgcADgcAEgABBwASB/kAAQAdAAAABAABAB4AAQAgAAAAAgAh"

func codeDeFuenteUnica(t *testing.T, fuente, metodo string) codeItem {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("salieron %d clases: %s", len(salida), nombresDe(salida))
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	d, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatal(err)
	}
	dex, err := readDex(d)
	if err != nil {
		t.Fatal(err)
	}
	item, err := codeItemDeMetodo(dex, metodo)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

// TestTryWithResourcesDos: con dos recursos, el dex del emisor es byte a byte el de javac 17. El
// `.class` sale con cuatro filas de excepción —el cuerpo y el cierre de `y`, y el cuerpo y el cierre
// de `x`— de dentro hacia fuera.
func TestTryWithResourcesDos(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesDos, tryResourcesDosJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	if item := codeDeFuenteUnica(t, tryResourcesDos, "t"); item.triesSize != 6 {
		// el `.class` lleva 4 filas (dos por recurso); al traducir, el rango exterior solapa con los
		// interiores y el dexer lo parte: [2,4), [4,12), [12,31), [31,37), [37,52) y [70,76)
		t.Errorf("%d try_item, medidos 6 (4 filas del .class partidas por el solape)", item.triesSize)
	}
}
