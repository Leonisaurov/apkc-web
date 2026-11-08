package main

import (
	"encoding/base64"
	"regexp"
	"sort"
	"testing"
)

// Sonda de 5.14 con captura: un parámetro effectively-final `x` de un método static viaja al campo
// `val$x` y al constructor `(I)V` de la anónima. Medido en el párrafo 100.
const fuenteCapturaDos = `public class Captura {
    static Runnable dos(int x) {
        return new Runnable() {
            public void run() {
                System.out.println(x);
            }
        };
    }
}
`

const capturaDosJavac = "yv66vgAAADQAKAkAAgADBwAEDAAFAAYBAAlDYXB0dXJhJDEBAAV2YWwkeAEAAUkKAAgACQcACgwACwAMAQAQamF2YS9sYW5nL09iamVjdAEABjxpbml0PgEAAygpVgkADgAPBwAQDAARABIBABBqYXZhL2xhbmcvU3lzdGVtAQADb3V0AQAVTGphdmEvaW8vUHJpbnRTdHJlYW07CgAUABUHABYMABcAGAEAE2phdmEvaW8vUHJpbnRTdHJlYW0BAAdwcmludGxuAQAEKEkpVgcAGgEAEmphdmEvbGFuZy9SdW5uYWJsZQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAlTaWduYXR1cmUBAANydW4BAApTb3VyY2VGaWxlAQAMQ2FwdHVyYS5qYXZhAQAPRW5jbG9zaW5nTWV0aG9kBwAjAQAHQ2FwdHVyYQwAJQAmAQADZG9zAQAXKEkpTGphdmEvbGFuZy9SdW5uYWJsZTsBAAxJbm5lckNsYXNzZXMAIAACAAgAAQAZAAEQEAAFAAYAAAACAAAACwAYAAIAGwAAACIAAgACAAAACiobtQABKrcAB7EAAAABABwAAAAGAAEAAAADAB0AAAACAAwAAQAeAAwAAQAbAAAAJwACAAEAAAALsgANKrQAAbYAE7EAAAABABwAAAAKAAIAAAAFAAoABgADAB8AAAACACAAIQAAAAQAIgAkACcAAAAKAAEAAgAAAAAAAA=="
const capturaDosOuterJavac = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAJQ2FwdHVyYSQxCgAHAAoMAAUACwEABChJKVYHAA0BAAdDYXB0dXJhAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAA2RvcwEAFyhJKUxqYXZhL2xhbmcvUnVubmFibGU7AQAKU291cmNlRmlsZQEADENhcHR1cmEuamF2YQEADElubmVyQ2xhc3NlcwAhAAwAAgAAAAAAAgABAAUABgABAA4AAAAdAAEAAQAAAAUqtwABsQAAAAEADwAAAAYAAQAAAAEACAAQABEAAQAOAAAAIQADAAEAAAAJuwAHWRq3AAmwAAAAAQAPAAAABgABAAAAAwACABIAAAACABMAFAAAAAoAAQAHAAAAAAAA"

const fuenteCapturaTres = `public class Captura3 {
    int y;
    Runnable r(int x) {
        return new Runnable() {
            public void run() {
                System.out.println(x + y);
            }
        };
    }
}
`

const captura3Javac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAKQ2FwdHVyYTMkMQoABwAKDAAFAAsBAA4oTENhcHR1cmEzO0kpVgcADQEACENhcHR1cmEzAQABeQEAAUkBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABcgEAFyhJKUxqYXZhL2xhbmcvUnVubmFibGU7AQAKU291cmNlRmlsZQEADUNhcHR1cmEzLmphdmEBAAxJbm5lckNsYXNzZXMAIQAMAAIAAAABAAAADgAPAAAAAgABAAUABgABABAAAAAdAAEAAQAAAAUqtwABsQAAAAEAEQAAAAYAAQAAAAEAAAASABMAAQAQAAAAIgAEAAIAAAAKuwAHWSobtwAJsAAAAAEAEQAAAAYAAQAAAAQAAgAUAAAAAgAVABYAAAAKAAEABwAAAAAAAA=="
const captura31Javac = "yv66vgAAADQAMQkAAgADBwAEDAAFAAYBAApDYXB0dXJhMyQxAQAGdGhpcyQwAQAKTENhcHR1cmEzOwkAAgAIDAAJAAoBAAV2YWwkeAEAAUkKAAwADQcADgwADwAQAQAQamF2YS9sYW5nL09iamVjdAEABjxpbml0PgEAAygpVgkAEgATBwAUDAAVABYBABBqYXZhL2xhbmcvU3lzdGVtAQADb3V0AQAVTGphdmEvaW8vUHJpbnRTdHJlYW07CQAYABkHABoMABsACgEACENhcHR1cmEzAQABeQoAHQAeBwAfDAAgACEBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChJKVYHACMBABJqYXZhL2xhbmcvUnVubmFibGUBAA4oTENhcHR1cmEzO0kpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABBNZXRob2RQYXJhbWV0ZXJzAQAJU2lnbmF0dXJlAQADcnVuAQAKU291cmNlRmlsZQEADUNhcHR1cmEzLmphdmEBAA9FbmNsb3NpbmdNZXRob2QMAC4ALwEAAXIBABcoSSlMamF2YS9sYW5nL1J1bm5hYmxlOwEADElubmVyQ2xhc3NlcwAgAAIADAABACIAAhAQAAkACgAAEBAABQAGAAAAAgAAAA8AJAADACUAAAAnAAIAAwAAAA8qK7UAASoctQAHKrcAC7EAAAABACYAAAAGAAEAAAAEACcAAAAJAgAAgBAAABAQACgAAAACABAAAQApABAAAQAlAAAALwADAAEAAAATsgARKrQAByq0AAG0ABdgtgAcsQAAAAEAJgAAAAoAAgAAAAYAEgAHAAMAKgAAAAIAKwAsAAAABAAYAC0AMAAAAAoAAQACAAAAAAAA"

func TestLaCapturaConThisSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Captura3.java", contenido: fuenteCapturaTres}}, jarDelSDK(t))
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
	for _, b64 := range []string{captura3Javac, captura31Javac} {
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
	sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }
	for i := range nuestras {
		a, b := nuestras[i], suyas[i]
		if len(a.Fields) != len(b.Fields) {
			t.Fatalf("%s: %d campos, javac %d", a.Name, len(a.Fields), len(b.Fields))
		}
		for j := range a.Fields {
			if a.Fields[j].Name != b.Fields[j].Name || a.Fields[j].Access != b.Fields[j].Access {
				t.Errorf("%s campo #%d: %s/%#x, javac %s/%#x", a.Name, j,
					a.Fields[j].Name, a.Fields[j].Access, b.Fields[j].Name, b.Fields[j].Access)
			}
		}
		for _, nombre := range []string{"<init>", "run", "r"} {
			am, ok := buscaMetodo(a, nombre)
			if !ok {
				continue
			}
			bm, _ := buscaMetodo(b, nombre)
			if x, y := sinPool(formaDeMetodo(t, a, am)), sinPool(formaDeMetodo(t, b, bm)); x != y {
				t.Errorf("%s.%s:\nnuestro  %s\njavac    %s", a.Name, nombre, x, y)
			}
			if x, y := filasDeLinea(am), filasDeLinea(bm); x != y {
				t.Errorf("%s.%s: filas %q contra %q", a.Name, nombre, x, y)
			}
		}
	}
	if _, err := dexDeClases(nuestras); err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
}

func TestLaCapturaDeLocalSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Captura.java", contenido: fuenteCapturaDos}}, jarDelSDK(t))
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
	for _, b64 := range []string{capturaDosOuterJavac, capturaDosJavac} {
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
	sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }
	for i := range nuestras {
		a, b := nuestras[i], suyas[i]
		for _, nombre := range []string{"<init>", "run", "dos"} {
			am, ok := buscaMetodo(a, nombre)
			if !ok {
				continue
			}
			bm, _ := buscaMetodo(b, nombre)
			if x, y := sinPool(formaDeMetodo(t, a, am)), sinPool(formaDeMetodo(t, b, bm)); x != y {
				t.Errorf("%s.%s:\nnuestro  %s\njavac    %s", a.Name, nombre, x, y)
			}
			if x, y := filasDeLinea(am), filasDeLinea(bm); x != y {
				t.Errorf("%s.%s: filas %q contra %q", a.Name, nombre, x, y)
			}
		}
	}
	if _, err := dexDeClases(nuestras); err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
}
