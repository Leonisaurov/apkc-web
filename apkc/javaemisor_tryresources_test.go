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

// dos recursos: se cierran en orden inverso. Se compara otra vez contra la forma manual anidada.
// FRONTERA: con mas de un recurso el emisor sigue con el desugaring funcional a try/finally y no
// escribe `addSuppressed`; el dex no es byte a byte el de javac cuando cuerpo y cierre fallan a la vez.
const tryResourcesDos = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable a, Closeable b) throws Exception {
    try (Closeable x = a; Closeable y = b) { use(x); use(y); }
  }
}
`

const tryResourcesDosManual = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable a, Closeable b) throws Exception {
    Closeable x = a;
    Closeable y = b;
    try {
      try { use(x); use(y); } finally { if (y != null) y.close(); }
    } finally { if (x != null) x.close(); }
  }
}
`

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

func comparaCode(t *testing.T, uno, dos codeItem) {
	t.Helper()
	if uno.registersSize != dos.registersSize || uno.insSize != dos.insSize || uno.outsSize != dos.outsSize {
		t.Errorf("cabecera %d/%d/%d contra %d/%d/%d", uno.registersSize, uno.insSize, uno.outsSize,
			dos.registersSize, dos.insSize, dos.outsSize)
	}
	if uno.triesSize != dos.triesSize {
		t.Errorf("%d try_item contra %d", uno.triesSize, dos.triesSize)
	}
	if len(uno.insns) != len(dos.insns) {
		t.Errorf("%d palabras contra %d", len(uno.insns), len(dos.insns))
		return
	}
	for i := range uno.insns {
		if uno.insns[i] != dos.insns[i] {
			t.Errorf("la palabra %d es %#04x contra %#04x", i, uno.insns[i], dos.insns[i])
			return
		}
	}
}

func TestTryWithResourcesDos(t *testing.T) {
	comparaCode(t, codeDeFuenteUnica(t, tryResourcesDos, "t"), codeDeFuenteUnica(t, tryResourcesDosManual, "t"))
}
