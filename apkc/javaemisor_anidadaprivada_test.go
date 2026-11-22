package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Fuente de la sonda: una anidada private estatica instanciada desde la clase que la contiene.
// javac 17 con `--release 8 -g:lines,source` escribe un constructor sintetico `(LCapa$1;)V` en la
// anidada, la clase marcador `Capa$1` y la llamada pasa `null` (medido con `javap -c -v`).
const capaPrivadaFuente = `public class Capa {
  private static class Hija { int val() { return 5; } }
  static int f() { return new Hija().val(); }
}
`

const capaJavac = "yv66vgAAADQAGwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAJQ2FwYSRIaWphCgAHAAoMAAUACwEACyhMQ2FwYSQxOylWCgAHAA0MAA4ADwEAA3ZhbAEAAygpSQcAEQEABENhcGEBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABZgEAClNvdXJjZUZpbGUBAAlDYXBhLmphdmEBAAxJbm5lckNsYXNzZXMBAARIaWphBwAaAQAGQ2FwYSQxACEAEAACAAAAAAACAAEABQAGAAEAEgAAAB0AAQABAAAABSq3AAGxAAAAAQATAAAABgABAAAAAQAIABQADwABABIAAAAkAAMAAAAAAAy7AAdZAbcACbYADKwAAAABABMAAAAGAAEAAAADAAIAFQAAAAIAFgAXAAAAEgACAAcAEAAYAAoAGQAAAAAQCA=="
const capaHijaJavac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBAAlDYXBhJEhpamEBAAY8aW5pdD4BAAMoKVYKAAgAAwcACQEAEGphdmEvbGFuZy9PYmplY3QBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQADdmFsAQADKClJAQALKExDYXBhJDE7KVYBAApTb3VyY2VGaWxlAQAJQ2FwYS5qYXZhAQAMSW5uZXJDbGFzc2VzBwATAQAEQ2FwYQEABEhpamEHABYBAAZDYXBhJDEAIAACAAgAAAAAAAMAAgAFAAYAAQAKAAAAHQABAAEAAAAFKrcAB7EAAAABAAsAAAAGAAEAAAACAAAADAANAAEACgAAABoAAQABAAAAAgisAAAAAQALAAAABgABAAAAAhAAAAUADgABAAoAAAAdAAEAAgAAAAUqtwABsQAAAAEACwAAAAYAAQAAAAIAAgAPAAAAAgAQABEAAAASAAIAAgASABQACgAVAAAAABAI"
const capaMarcadorJavac = "yv66vgAAADQACwcAAgEABkNhcGEkMQcABAEAEGphdmEvbGFuZy9PYmplY3QBAApTb3VyY2VGaWxlAQAJQ2FwYS5qYXZhAQAPRW5jbG9zaW5nTWV0aG9kBwAJAQAEQ2FwYQEADElubmVyQ2xhc3NlcxAgAAEAAwAAAAAAAAADAAUAAAACAAYABwAAAAQACAAAAAoAAAAKAAEAAQAAAAAQCA=="

// TestAnidadaPrivadaDesdeFuera: las tres clases (exterior, anidada y marcador) salen con el mismo
// dex que las de javac 17: la llamada usa el constructor sintetico con `null`, la anidada lleva el
// constructor ACC_SYNTHETIC y el marcador es una clase vacia con EnclosingMethod.
func TestAnidadaPrivadaDesdeFuera(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Capa.java", contenido: capaPrivadaFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 3 {
		t.Fatalf("%d clases, se esperaban 3: %s", len(salida), nombresDe(salida))
	}
	nuestras := map[string]classFull{}
	for _, s := range salida {
		c, err := readClassFull(s.datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestras[c.Name] = c
	}
	for _, b64 := range []string{capaJavac, capaHijaJavac, capaMarcadorJavac} {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		suya, err := readClassFull(datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestra, ok := nuestras[suya.Name]
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
