package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteMas es la sonda del parrafo 68: `+=` y `-=` sobre un campo y sobre un local, `*=` y un `+=`
// con un literal que no cabe en un byte (`iinc` solo llega a -128..127).
const fuenteMas = `public class Mas {
    static int total = 0;
    static void campo() { total += 2; }
    static void local() { int i = 0; i += 2; }
    static void localMenos() { int i = 0; i -= 3; }
    static void localMas() { int i = 5; i *= 3; }
    static int lee(int a) { int i = a; i += 7; return i; }
}
`

// masJavac: `javac -g Mas.java` y `base64 -w0 Mas.class`.
const masJavac = "yv66vgAAAEEAHQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAA01hcwEABXRvdGFsAQABSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQAFTE1hczsBAAVjYW1wbwEABWxvY2FsAQABaQEACmxvY2FsTWVub3MBAAhsb2NhbE1hcwEAA2xlZQEABChJKUkBAAFhAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAITWFzLmphdmEAIQAIAAIAAAABAAgACwAMAAAABwABAAUABgABAA0AAAAvAAEAAQAAAAUqtwABsQAAAAIADgAAAAYAAQAAAAEADwAAAAwAAQAAAAUAEAARAAAACAASAAYAAQANAAAAIQACAAAAAAAJsgAHBWCzAAexAAAAAQAOAAAABgABAAAAAwAIABMABgABAA0AAAAwAAEAAQAAAAYDO4QAArEAAAACAA4AAAAGAAEAAAAEAA8AAAAMAAEAAgAEABQADAAAAAgAFQAGAAEADQAAADAAAQABAAAABgM7hAD9sQAAAAIADgAAAAYAAQAAAAUADwAAAAwAAQACAAQAFAAMAAAACAAWAAYAAQANAAAAMQACAAEAAAAHCDsaBmg7sQAAAAIADgAAAAYAAQAAAAYADwAAAAwAAQACAAUAFAAMAAAACAAXABgAAQANAAAAOwABAAIAAAAHGjyEAQcbrAAAAAIADgAAAAYAAQAAAAcADwAAABYAAgAAAAcAGQAMAAAAAgAFABQADAABAAgAGgAGAAEADQAAAB0AAQAAAAAABQOzAAexAAAAAQAOAAAABgABAAAAAgABABsAAAACABw="

// TestElMasIgualSeDexaComoJavac: `x += n` es `iinc` en un local cuando el literal cabe en un byte,
// y `x = x + n` (la lectura-cuenta-escritura de javac) cuando no; `x *= n` no se pliega. Si la
// composición no es la de javac, el dex deja de ser el suyo byte a byte.
func TestElMasIgualSeDexaComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Mas.java", contenido: fuenteMas}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(masJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
	}
}
