package main

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"sort"
	"testing"
)

// Sonda del párrafo 92: `throws`, una interface propia con una constante, un field y dos métodos
// abstractos, una clase que la implementa y una llamada `invokeinterface`.
const fuenteInterfaz = `public interface I {
    int N = 3;
    int doble(int x);
    void nada();
}
`

const fuenteImpl = `public class Impl implements I {
    public int doble(int x) {
        return x * 2;
    }
    public void nada() {
    }
}
`

const fuenteUso = `import java.io.IOException;

public class Uso {
    static int usa(I i) {
        return i.doble(3);
    }
    static void falla() throws IOException {
    }
    static void falla2() throws IOException, IllegalStateException {
    }
}
`

const fuenteAbstracta = `public abstract class A {
    public abstract int x();
}
`

const interfazJavac = "yv66vgAAADQADgcAAgEAAUkHAAQBABBqYXZhL2xhbmcvT2JqZWN0AQABTgEADUNvbnN0YW50VmFsdWUDAAAAAwEABWRvYmxlAQAEKEkpSQEABG5hZGEBAAMoKVYBAApTb3VyY2VGaWxlAQAGSS5qYXZhBgEAAQADAAAAAQAZAAUAAgABAAYAAAACAAcAAgQBAAgACQAABAEACgALAAAAAQAMAAAAAgAN"
const implJavac = "yv66vgAAADQAEgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAESW1wbAcACgEAAUkBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAFZG9ibGUBAAQoSSlJAQAEbmFkYQEAClNvdXJjZUZpbGUBAAlJbXBsLmphdmEAIQAHAAIAAQAJAAAAAwABAAUABgABAAsAAAAdAAEAAQAAAAUqtwABsQAAAAEADAAAAAYAAQAAAAEAAQANAA4AAQALAAAAHAACAAIAAAAEGwVorAAAAAEADAAAAAYAAQAAAAMAAQAPAAYAAQALAAAAGQAAAAEAAAABsQAAAAEADAAAAAYAAQAAAAYAAQAQAAAAAgAR"
const usoJavac = "yv66vgAAADQAHAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCwAIAAkHAAoMAAsADAEAAUkBAAVkb2JsZQEABChJKUkHAA4BAANVc28BAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQADdXNhAQAGKExJOylJAQAFZmFsbGEBAApFeGNlcHRpb25zBwAWAQATamF2YS9pby9JT0V4Y2VwdGlvbgEABmZhbGxhMgcAGQEAH2phdmEvbGFuZy9JbGxlZ2FsU3RhdGVFeGNlcHRpb24BAApTb3VyY2VGaWxlAQAIVXNvLmphdmEAIQANAAIAAAAAAAQAAQAFAAYAAQAPAAAAHQABAAEAAAAFKrcAAbEAAAABABAAAAAGAAEAAAADAAgAEQASAAEADwAAACAAAgABAAAACCoGuQAHAgCsAAAAAQAQAAAABgABAAAABQAIABMABgACAA8AAAAZAAAAAAAAAAGxAAAAAQAQAAAABgABAAAACAAUAAAABAABABUACAAXAAYAAgAPAAAAGQAAAAAAAAABsQAAAAEAEAAAAAYAAQAAAAoAFAAAAAYAAgAVABgAAQAaAAAAAgAb"
const abstractaJavac = "yv66vgAAADQADwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQABQQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF4AQADKClJAQAKU291cmNlRmlsZQEABkEuamF2YQQhAAcAAgAAAAAAAgABAAUABgABAAkAAAAdAAEAAQAAAAUqtwABsQAAAAEACgAAAAYAAQAAAAEEAQALAAwAAAABAA0AAAACAA4="

func dosArbolesInterfaz(t *testing.T) ([]classFull, []classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/I.java", contenido: fuenteInterfaz},
		{nombre: "src/Impl.java", contenido: fuenteImpl},
		{nombre: "src/Uso.java", contenido: fuenteUso},
		{nombre: "src/A.java", contenido: fuenteAbstracta},
	}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestras := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase %s no se lee: %v", c.interno, err)
		}
		nuestras = append(nuestras, cf)
	}
	var suyas []classFull
	for _, b64 := range []string{abstractaJavac, interfazJavac, implJavac, usoJavac} {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("el fixture no es base64: %v", err)
		}
		cf, err := readClassFull(datos)
		if err != nil {
			t.Fatalf("el fixture de javac no se lee: %v", err)
		}
		suyas = append(suyas, cf)
	}
	sort.Slice(nuestras, func(i, j int) bool { return nuestras[i].Name < nuestras[j].Name })
	sort.Slice(suyas, func(i, j int) bool { return suyas[i].Name < suyas[j].Name })
	return nuestras, suyas
}

func clasePorNombre(clases []classFull, nombre string) (classFull, bool) {
	for _, c := range clases {
		if c.Name == nombre {
			return c, true
		}
	}
	return classFull{}, false
}

// TestLaInterfaceYElThrowsSeEscribenComoJavac: flags de la interface (0x0601), campo N con
// ConstantValue, métodos abstractos sin Code, `interfaces` de Impl y el atributo Exceptions de Uso.
func TestLaInterfaceYElThrowsSeEscribenComoJavac(t *testing.T) {
	nuestras, suyas := dosArbolesInterfaz(t)
	sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }

	ni, _ := clasePorNombre(nuestras, "LI;")
	su, _ := clasePorNombre(suyas, "LI;")
	if ni.Access != su.Access || ni.Access != 0x0601 {
		t.Errorf("flags de la interface: nuestro %#04x, javac %#04x", ni.Access, su.Access)
	}
	if len(ni.Methods) != 2 || len(ni.Fields) != 1 {
		t.Fatalf("interface: %d metodos y %d campos", len(ni.Methods), len(ni.Fields))
	}
	for _, nombre := range []string{"doble", "nada"} {
		a, ok := buscaMetodo(ni, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra interface", nombre)
		}
		b, ok := buscaMetodo(su, nombre)
		if !ok {
			t.Fatalf("%s: no está en la de javac", nombre)
		}
		if a.Access != b.Access || a.Access != 0x0401 || a.HasCode {
			t.Errorf("%s: flags %#04x, javac %#04x, Code %v", nombre, a.Access, b.Access, a.HasCode)
		}
	}

	nimpl, _ := clasePorNombre(nuestras, "LImpl;")
	simp, _ := clasePorNombre(suyas, "LImpl;")
	if len(nimpl.Interfaces) != 1 || nimpl.Interfaces[0] != "LI;" {
		t.Errorf("Impl no declara la interface: %v", nimpl.Interfaces)
	}
	for _, nombre := range []string{"doble", "nada"} {
		a, _ := buscaMetodo(nimpl, nombre)
		b, _ := buscaMetodo(simp, nombre)
		if x, y := sinPool(formaDeMetodo(t, nimpl, a)), sinPool(formaDeMetodo(t, simp, b)); x != y {
			t.Errorf("Impl.%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("Impl.%s: filas %q contra %q", nombre, x, y)
		}
	}

	na, _ := clasePorNombre(nuestras, "LA;")
	sa, _ := clasePorNombre(suyas, "LA;")
	if na.Access != sa.Access || na.Access != 0x0421 {
		t.Errorf("flags de la clase abstracta: nuestro %#04x, javac %#04x", na.Access, sa.Access)
	}
	if a, ok := buscaMetodo(na, "x"); !ok || a.Access != 0x0401 || a.HasCode {
		t.Errorf("A.x: flags %#04x, Code %v", a.Access, a.HasCode)
	}

	nu, _ := clasePorNombre(nuestras, "LUso;")
	su2, _ := clasePorNombre(suyas, "LUso;")
	ua, _ := buscaMetodo(nu, "usa")
	ub, _ := buscaMetodo(su2, "usa")
	if x, y := sinPool(formaDeMetodo(t, nu, ua)), sinPool(formaDeMetodo(t, su2, ub)); x != y {
		t.Errorf("Uso.usa:\nnuestro  %s\njavac    %s", x, y)
	}
	for _, nombre := range []string{"falla", "falla2"} {
		a, _ := buscaMetodo(nu, nombre)
		b, _ := buscaMetodo(su2, nombre)
		if x, y := sinPool(formaDeMetodo(t, nu, a)), sinPool(formaDeMetodo(t, su2, b)); x != y {
			t.Errorf("Uso.%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if len(a.Thrown) != len(b.Thrown) {
			t.Errorf("Uso.%s: %d excepciones, javac %d", nombre, len(a.Thrown), len(b.Thrown))
			continue
		}
		for i := range a.Thrown {
			if a.Thrown[i] != b.Thrown[i] {
				t.Errorf("Uso.%s: excepción %d %s, javac %s", nombre, i, a.Thrown[i], b.Thrown[i])
			}
		}
	}
}

// TestLaInterfaceYElThrowsSeDexanIgual: el dex de las cuatro clases, byte a byte.
func TestLaInterfaceYElThrowsSeDexanIgual(t *testing.T) {
	nuestras, suyas := dosArbolesInterfaz(t)
	deUno, err := dexDeClases(nuestras)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases(suyas)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 92: %d bytes", len(deUno))
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex nuestro son %d bytes y el de javac %d", len(deUno), len(deOtro))
	}
}
