package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

const exteriorFuente = `public class Exterior {
  static class Anidada {
    static int doble(int n) { return n * 2; }
  }
  static int f(int n) { return Anidada.doble(n); }
}
`

const exteriorJavac = "yv66vgAAAD0AHAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAEEV4dGVyaW9yJEFuaWRhZGEBAAVkb2JsZQEABChJKUkHAA4BAAhFeHRlcmlvcgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQAKTEV4dGVyaW9yOwEAAWYBAAFuAQABSQEAClNvdXJjZUZpbGUBAA1FeHRlcmlvci5qYXZhAQALTmVzdE1lbWJlcnMBAAxJbm5lckNsYXNzZXMBAAdBbmlkYWRhACEADQACAAAAAAACAAEABQAGAAEADwAAAC8AAQABAAAABSq3AAGxAAAAAgAQAAAABgABAAAAAQARAAAADAABAAAABQASABMAAAAIABQADAABAA8AAAAvAAEAAQAAAAUauAAHrAAAAAIAEAAAAAYAAQAAAAUAEQAAAAwAAQAAAAUAFQAWAAAAAwAXAAAAAgAYABkAAAAEAAEACAAaAAAACgABAAgADQAbAAg="
const anidadaJavac = "yv66vgAAAD0AGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAQRXh0ZXJpb3IkQW5pZGFkYQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQASTEV4dGVyaW9yJEFuaWRhZGE7AQAFZG9ibGUBAAQoSSlJAQABbgEAAUkBAApTb3VyY2VGaWxlAQANRXh0ZXJpb3IuamF2YQEACE5lc3RIb3N0BwAWAQAIRXh0ZXJpb3IBAAxJbm5lckNsYXNzZXMBAAdBbmlkYWRhACAABwACAAAAAAACAAAABQAGAAEACQAAAC8AAQABAAAABSq3AAGxAAAAAgAKAAAABgABAAAAAgALAAAADAABAAAABQAMAA0AAAAIAA4ADwABAAkAAAAuAAIAAQAAAAQaBWisAAAAAgAKAAAABgABAAAAAwALAAAADAABAAAABAAQABEAAAADABIAAAACABMAFAAAAAIAFQAXAAAACgABAAcAFQAYAAg="

// TestClaseAnidadaEstatica: el nombre simple `Anidada` resuelve a `Exterior$Anidada` y el par de
// clases sale con el mismo dex que el de javac (§170).
func TestClaseAnidadaEstatica(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Exterior.java", contenido: exteriorFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 2 {
		t.Fatalf("se escribieron %d clases, se esperaban 2", len(salida))
	}
	porNombre := map[string]classFull{}
	for _, s := range salida {
		c, err := readClassFull(s.datos)
		if err != nil {
			t.Fatal(err)
		}
		porNombre[c.Name] = c
	}
	for _, datos64 := range []string{exteriorJavac, anidadaJavac} {
		datos, err := base64.StdEncoding.DecodeString(datos64)
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
