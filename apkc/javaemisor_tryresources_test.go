package main

import "testing"

// try-with-resources se baja en el parser a un try/finally funcional: `if (r != null) r.close();`.
// El oráculo es la equivalencia con el fuente escrito a mano: si el desugaring es correcto, el dex de
// las dos formas tiene que ser el mismo. No se compara con el javac 17 de `try (…)`, que además del
// cierre escribe `Throwable.addSuppressed` para no perder la excepción del cuerpo cuando `close()`
// también falla; aquí esa rama NO está implementada (frontera documentada en este fichero).
const tryResourcesFuente = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable c) throws Exception {
    try (Closeable r = c) { use(r); }
  }
}
`

const tryResourcesManual = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable c) throws Exception {
    Closeable r = c;
    try { use(r); } finally { if (r != null) r.close(); }
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

func TestTryWithResources(t *testing.T) {
	comparaCode(t, codeDeFuenteUnica(t, tryResourcesFuente, "t"), codeDeFuenteUnica(t, tryResourcesManual, "t"))
}

// dos recursos: se cierran en orden inverso. Se compara otra vez contra la forma manual anidada.
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

func TestTryWithResourcesDos(t *testing.T) {
	comparaCode(t, codeDeFuenteUnica(t, tryResourcesDos, "t"), codeDeFuenteUnica(t, tryResourcesDosManual, "t"))
}
