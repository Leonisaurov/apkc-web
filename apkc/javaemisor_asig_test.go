package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// asigFuente es la sonda del párrafo 157: +=/-= ya estaban medidos; ahora <<=, >>=, >>>=, &=, |= y ^=
// sobre un local, un campo de instancia y un campo estático, con literal y con variable a la derecha.
const asigFuente = `public class Asig {
  int campo;
  static int s;
  static int shifts(int a, int b) { a <<= 2; a >>= 1; a >>>= 1; return a + b; }
  static int bits(int a, int b) { a &= b; a |= 3; a ^= 1; return a; }
  int campoShifts(int b) { campo <<= 2; campo >>= 1; return campo + b; }
  int campoBits(int b) { campo &= b; campo |= 3; campo ^= 1; return campo; }
  static void estatico(int b) { s <<= 2; s &= b; }
}
`

// asigJavac: `javac -g Asig.java && base64 -w0 Asig.class` con el javac 17 del gate.
const asigJavac = "yv66vgAAAD0AIQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABEFzaWcBAAVjYW1wbwEAAUkJAAgADgwADwAMAQABcwEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQAGTEFzaWc7AQAGc2hpZnRzAQAFKElJKUkBAAFhAQABYgEABGJpdHMBAAtjYW1wb1NoaWZ0cwEABChJKUkBAAljYW1wb0JpdHMBAAhlc3RhdGljbwEABChJKVYBAApTb3VyY2VGaWxlAQAJQXNpZy5qYXZhACEACAACAAAAAgAAAAsADAAAAAgADwAMAAAABgABAAUABgABABAAAAAvAAEAAQAAAAUqtwABsQAAAAIAEQAAAAYAAQAAAAEAEgAAAAwAAQAAAAUAEwAUAAAACAAVABYAAQAQAAAARAACAAIAAAAQGgV4OxoEejsaBHw7GhtgrAAAAAIAEQAAAAYAAQAAAAQAEgAAABYAAgAAABAAFwAMAAAAAAAQABgADAABAAgAGQAWAAEAEAAAAEIAAgACAAAADhobfjsaBoA7GgSCOxqsAAAAAgARAAAABgABAAAABQASAAAAFgACAAAADgAXAAwAAAAAAA4AGAAMAAEAAAAaABsAAQAQAAAATwADAAIAAAAbKlm0AAcFeLUABypZtAAHBHq1AAcqtAAHG2CsAAAAAgARAAAABgABAAAABgASAAAAFgACAAAAGwATABQAAAAAABsAGAAMAAEAAAAcABsAAQAQAAAAVwADAAIAAAAjKlm0AAcbfrUABypZtAAHBoC1AAcqWbQABwSCtQAHKrQAB6wAAAACABEAAAAGAAEAAAAHABIAAAAWAAIAAAAjABMAFAAAAAAAIwAYAAwAAQAIAB0AHgABABAAAAA7AAIAAQAAABGyAA0FeLMADbIADRp+swANsQAAAAIAEQAAAAYAAQAAAAgAEgAAAAwAAQAAABEAGAAMAAAAAQAfAAAAAgAg"

// TestLasAsignacionesDeBitsSeDexanIgualQueJavac: oráculo doble. La longitud del Code por método y,
// sobre todo, el dex completo byte a byte de nuestras clases y de las de javac.
func TestLasAsignacionesDeBitsSeDexanIgualQueJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Asig.java", contenido: asigFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(asigJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el método %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if len(encontrada.Code) != len(esperado.Code) {
			t.Errorf("%s%s: nuestro Code mide %d bytes y el de javac %d",
				esperado.Name, esperado.Desc, len(encontrada.Code), len(esperado.Code))
		}
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
