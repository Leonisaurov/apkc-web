package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Sonda de una clase anidada con nombre SIN static, compilada con javac 17 y
// `javac --release 8 -g:lines,source Inner.java`. El exterior se baja a un campo `this$0` y el
// constructor sintetizado recibe `(LInner;)V`; el `new Hija()` del exterior le pasa `this`.
const innerFuente = `public class Inner {
  class Hija {
    int doble(int n) { return n * 2; }
  }
  Hija make() { return new Hija(); }
  int use() { return make().doble(3); }
}
`

const innerJavac = "yv66vgAAADQAHgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAKSW5uZXIkSGlqYQoABwAKDAAFAAsBAAooTElubmVyOylWCgANAA4HAA8MABAAEQEABUlubmVyAQAEbWFrZQEADigpTElubmVyJEhpamE7CgAHABMMABQAFQEABWRvYmxlAQAEKEkpSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAN1c2UBAAMoKUkBAApTb3VyY2VGaWxlAQAKSW5uZXIuamF2YQEADElubmVyQ2xhc3NlcwEABEhpamEAIQANAAIAAAAAAAMAAQAFAAYAAQAWAAAAHQABAAEAAAAFKrcAAbEAAAABABcAAAAGAAEAAAABAAAAEAARAAEAFgAAACEAAwABAAAACbsAB1kqtwAJsAAAAAEAFwAAAAYAAQAAAAUAAAAYABkAAQAWAAAAIQACAAEAAAAJKrYADAa2ABKsAAAAAQAXAAAABgABAAAABgACABoAAAACABsAHAAAAAoAAQAHAA0AHQAA"

const innerHijaJavac = "yv66vgAAADQAGAkAAgADBwAEDAAFAAYBAApJbm5lciRIaWphAQAGdGhpcyQwAQAHTElubmVyOwoACAAJBwAKDAALAAwBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWAQAKKExJbm5lcjspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAVkb2JsZQEABChJKUkBAApTb3VyY2VGaWxlAQAKSW5uZXIuamF2YQEADElubmVyQ2xhc3NlcwcAFgEABUlubmVyAQAESGlqYQAgAAIACAAAAAEQEAAFAAYAAAACAAAACwANAAEADgAAACIAAgACAAAACiortQABKrcAB7EAAAABAA8AAAAGAAEAAAACAAAAEAARAAEADgAAABwAAgACAAAABBsFaKwAAAABAA8AAAAGAAEAAAADAAIAEgAAAAIAEwAUAAAACgABAAIAFQAXAAA="

const privFuente = `public class Q {
  private class Hija { int val() { return 5; } }
  int use(Hija h) { return h.val(); }
}
`
const privJavac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEABlEkSGlqYQEAA3ZhbAEAAygpSQcADgEAAVEBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQADdXNlAQALKExRJEhpamE7KUkBAApTb3VyY2VGaWxlAQAGUS5qYXZhAQAMSW5uZXJDbGFzc2VzAQAESGlqYQAhAA0AAgAAAAAAAgABAAUABgABAA8AAAAdAAEAAQAAAAUqtwABsQAAAAEAEAAAAAYAAQAAAAEAAAARABIAAQAPAAAAHQABAAIAAAAFK7YAB6wAAAABABAAAAAGAAEAAAADAAIAEwAAAAIAFAAVAAAACgABAAgADQAWAAI="
const privHijaJavac = "yv66vgAAADQAGAkAAgADBwAEDAAFAAYBAAZRJEhpamEBAAZ0aGlzJDABAANMUTsKAAgACQcACgwACwAMAQAQamF2YS9sYW5nL09iamVjdAEABjxpbml0PgEAAygpVgEABihMUTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAN2YWwBAAMoKUkBAApTb3VyY2VGaWxlAQAGUS5qYXZhAQAMSW5uZXJDbGFzc2VzBwAWAQABUQEABEhpamEAIAACAAgAAAABEBAABQAGAAAAAgACAAsADQABAA4AAAAiAAIAAgAAAAoqK7UAASq3AAexAAAAAQAPAAAABgABAAAAAgAAABAAEQABAA4AAAAaAAEAAQAAAAIIrAAAAAEADwAAAAYAAQAAAAIAAgASAAAAAgATABQAAAAKAAEAAgAVABcAAg=="

func TestClaseAnidadaPrivada(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Q.java", contenido: privFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	porNombre := map[string]classFull{}
	for _, s := range salida {
		c, err := readClassFull(s.datos)
		if err != nil {
			t.Fatal(err)
		}
		porNombre[c.Name] = c
	}
	for _, fixture := range []string{privJavac, privHijaJavac} {
		datos, err := base64.StdEncoding.DecodeString(fixture)
		if err != nil {
			t.Fatal(err)
		}
		suya, err := readClassFull(datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestra, ok := porNombre[suya.Name]
		if !ok {
			t.Errorf("no se escribió %s", suya.Name)
			continue
		}
		deUno, err := dexDeClases([]classFull{nuestra})
		if err != nil {
			t.Fatal(err)
		}
		deOtro, err := dexDeClases([]classFull{suya})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(deUno, deOtro) {
			t.Errorf("%s: dex de %d bytes contra %d de javac; primer byte distinto %d",
				suya.Name, len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
		}
	}
}

func TestClaseAnidadaNoEstatica(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Inner.java", contenido: innerFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 2 {
		t.Fatalf("se escribieron %d clases, se esperaban 2: %s", len(salida), nombresDe(salida))
	}
	porNombre := map[string]classFull{}
	for _, s := range salida {
		c, err := readClassFull(s.datos)
		if err != nil {
			t.Fatal(err)
		}
		porNombre[c.Name] = c
	}
	for _, fixture := range []string{innerJavac, innerHijaJavac} {
		datos, err := base64.StdEncoding.DecodeString(fixture)
		if err != nil {
			t.Fatal(err)
		}
		suya, err := readClassFull(datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestra, ok := porNombre[suya.Name]
		if !ok {
			t.Errorf("no se escribió %s", suya.Name)
			continue
		}
		deUno, err := dexDeClases([]classFull{nuestra})
		if err != nil {
			t.Fatal(err)
		}
		deOtro, err := dexDeClases([]classFull{suya})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(deUno, deOtro) {
			t.Errorf("%s: dex de %d bytes contra %d de javac; primer byte distinto %d",
				suya.Name, len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
		}
	}
}
