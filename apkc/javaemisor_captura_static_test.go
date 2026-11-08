package main

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"sort"
	"testing"
)

// Sonda de 5.14 sin captura: una anónima creada en un método `static`, con constructor `()V` y sin
// `this$0`. La captura de locales vive en javaemisor_captura_test.go.
const fuenteCapturaStatic = `public class Captura {
    static Runnable uno(int x) {
        return new Runnable() {
            public void run() {
            }
        };
    }
}
`

const capturaStaticJavac = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAJQ2FwdHVyYSQxCgAHAAMHAAsBAAdDYXB0dXJhAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAA3VubwEAFyhJKUxqYXZhL2xhbmcvUnVubmFibGU7AQAKU291cmNlRmlsZQEADENhcHR1cmEuamF2YQEADElubmVyQ2xhc3NlcwAhAAoAAgAAAAAAAgABAAUABgABAAwAAAAdAAEAAQAAAAUqtwABsQAAAAEADQAAAAYAAQAAAAEACAAOAA8AAQAMAAAAIAACAAEAAAAIuwAHWbcACbAAAAABAA0AAAAGAAEAAAADAAIAEAAAAAIAEQASAAAACgABAAcAAAAAAAA="
const capturaStatic1Javac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAJQ2FwdHVyYSQxBwAKAQASamF2YS9sYW5nL1J1bm5hYmxlAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAA3J1bgEAClNvdXJjZUZpbGUBAAxDYXB0dXJhLmphdmEBAA9FbmNsb3NpbmdNZXRob2QHABIBAAdDYXB0dXJhDAAUABUBAAN1bm8BABcoSSlMamF2YS9sYW5nL1J1bm5hYmxlOwEADElubmVyQ2xhc3NlcwAgAAcAAgABAAkAAAACAAAABQAGAAEACwAAAB0AAQABAAAABSq3AAGxAAAAAQAMAAAABgABAAAAAwABAA0ABgABAAsAAAAZAAAAAQAAAAGxAAAAAQAMAAAABgABAAAABQADAA4AAAACAA8AEAAAAAQAEQATABYAAAAKAAEABwAAAAAAAA=="

func TestLaAnonimaEnStaticSinCapturaSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Captura.java", contenido: fuenteCapturaStatic}}, jarDelSDK(t))
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
	for _, b64 := range []string{capturaStaticJavac, capturaStatic1Javac} {
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
		if a.Name != b.Name || len(a.Fields) != len(b.Fields) {
			t.Errorf("%s: campos %d, javac %d", a.Name, len(a.Fields), len(b.Fields))
		}
		for _, nombre := range []string{"<init>", "run"} {
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
	deUno, err := dexDeClases(nuestras)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases(suyas)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex nuestro son %d bytes y el de javac %d", len(deUno), len(deOtro))
	}
}
