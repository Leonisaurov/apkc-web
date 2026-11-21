package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

const anotFuente = `public class Anot {
  @Deprecated
  static int f(int n) { return n + 1; }
}
`

const anotJavac = "yv66vgAAAD0AFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAEQW5vdAEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQAGTEFub3Q7AQABZgEABChJKUkBAAFuAQABSQEACkRlcHJlY2F0ZWQBABlSdW50aW1lVmlzaWJsZUFubm90YXRpb25zAQAWTGphdmEvbGFuZy9EZXByZWNhdGVkOwEAClNvdXJjZUZpbGUBAAlBbm90LmphdmEAIQAHAAIAAAAAAAIAAQAFAAYAAQAJAAAALwABAAEAAAAFKrcAAbEAAAACAAoAAAAGAAEAAAABAAsAAAAMAAEAAAAFAAwADQAAAAgADgAPAAMACQAAAC4AAgABAAAABBoEYKwAAAACAAoAAAAGAAEAAAADAAsAAAAMAAEAAAAEABAAEQAAABIAAAAAABMAAAAGAAEAFAAAAAEAFQAAAAIAFg=="

// TestAnotacionDeprecated: @Deprecated deja un atributo Deprecated y un RuntimeVisibleAnnotations
// con una sola entrada sin pares; el dex de las clases de este fork y de javac coincide (§170).
func TestAnotacionDeprecated(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Anot.java", contenido: anotFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó la anotación: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(anotJavac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
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
		t.Errorf("dex de %d bytes contra %d de javac; primer byte distinto %d",
			len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
	}
}
