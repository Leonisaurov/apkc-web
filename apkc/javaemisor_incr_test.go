package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteInc es la sonda del parrafo 67: `i++` e `i--` en un bucle, el `++` de un campo estatico y el
// de un campo de instancia.
const fuenteInc = `public class Inc {
    static int total = 0;
    int propio = 0;
    static void local(int n) { int i = 0; while (i < n) { i++; } }
    static void localMenos(int n) { int i = 0; while (i < n) { i--; } }
    static void campo() { total++; }
    void instancia() { propio++; }
}
`

// incJavac: `javac -g Inc.java` (el javac 21 de siempre) y `base64 -w0 Inc.class`.
const incJavac = "yv66vgAAAEEAIAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAA0luYwEABnByb3BpbwEAAUkJAAgADgwADwAMAQAFdG90YWwBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQASTG9jYWxWYXJpYWJsZVRhYmxlAQAEdGhpcwEABUxJbmM7AQAFbG9jYWwBAAQoSSlWAQABbgEAAWkBAA1TdGFja01hcFRhYmxlAQAKbG9jYWxNZW5vcwEABWNhbXBvAQAJaW5zdGFuY2lhAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAISW5jLmphdmEAIQAIAAIAAAACAAgADwAMAAAAAAALAAwAAAAGAAEABQAGAAEAEAAAADgAAgABAAAACiq3AAEqA7UAB7EAAAACABEAAAAKAAIAAAABAAQAAwASAAAADAABAAAACgATABQAAAAIABUAFgABABAAAABPAAIAAgAAAA4DPBsaogAJhAEBp//4sQAAAAMAEQAAAAYAAQAAAAQAEgAAABYAAgAAAA4AFwAMAAAAAgAMABgADAABABkAAAAHAAL8AAIBCgAIABoAFgABABAAAABPAAIAAgAAAA4DPBsaogAJhAH/p//4sQAAAAMAEQAAAAYAAQAAAAUAEgAAABYAAgAAAA4AFwAMAAAAAgAMABgADAABABkAAAAHAAL8AAIBCgAIABsABgABABAAAAAhAAIAAAAAAAmyAA0EYLMADbEAAAABABEAAAAGAAEAAAAGAAAAHAAGAAEAEAAAADUAAwABAAAACypZtAAHBGC1AAexAAAAAgARAAAABgABAAAABwASAAAADAABAAAACwATABQAAAAIAB0ABgABABAAAAAdAAEAAAAAAAUDswANsQAAAAEAEQAAAAYAAQAAAAIAAQAeAAAAAgAf"

// TestElIncrementoSeDexaIgualQueElDeJavac: el oraculo doble sobre `++`/`--`. En un local javac
// escribe `iinc` y en un campo la lectura-cuenta-escritura; el traductor baja `iinc` a
// `add-int/lit8 vA, vB, #±1` (medido: `d800 0001` y `d800 00ff`), así que el dex de nuestras clases
// tiene que salir byte a byte el de las de javac.
func TestElIncrementoSeDexaIgualQueElDeJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Inc.java", contenido: fuenteInc}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(incJavac)
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
