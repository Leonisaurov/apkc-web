package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Enum simple compilado con javac 17 y `javac --release 8 -g:lines,source Color.java`: el andamiaje
// completo (java/lang/Enum, constantes, $VALUES, values, valueOf, constructor, $values y <clinit>) viaja
// al dex, así que la prueba compara el dex byte a byte con el de javac.
const enumFuente = `public enum Color { RED, GREEN, BLUE }
`

const enumJavac = "yv66vgAAADQAMgcAAgEABUNvbG9yCQABAAQMAAUABgEAA1JFRAEAB0xDb2xvcjsJAAEACAwACQAGAQAFR1JFRU4JAAEACwwADAAGAQAEQkxVRQkAAQAODAAPABABAAckVkFMVUVTAQAIW0xDb2xvcjsKABIAEwcAEAwAFAAVAQAFY2xvbmUBABQoKUxqYXZhL2xhbmcvT2JqZWN0OwoAFwAYBwAZDAAaABsBAA5qYXZhL2xhbmcvRW51bQEAB3ZhbHVlT2YBADUoTGphdmEvbGFuZy9DbGFzcztMamF2YS9sYW5nL1N0cmluZzspTGphdmEvbGFuZy9FbnVtOwoAFwAdDAAeAB8BAAY8aW5pdD4BABYoTGphdmEvbGFuZy9TdHJpbmc7SSlWCAAFCgABAB0IAAkIAAwKAAEAJQwAJgAnAQAHJHZhbHVlcwEACigpW0xDb2xvcjsBAAZ2YWx1ZXMBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAbKExqYXZhL2xhbmcvU3RyaW5nOylMQ29sb3I7AQAJU2lnbmF0dXJlAQADKClWAQAIPGNsaW5pdD4BABlMamF2YS9sYW5nL0VudW08TENvbG9yOz47AQAKU291cmNlRmlsZQEACkNvbG9yLmphdmFAMQABABcAAAAEQBkABQAGAABAGQAJAAYAAEAZAAwABgAAEBoADwAQAAAABQAJACgAJwABACkAAAAiAAEAAAAAAAqyAA22ABHAABKwAAAAAQAqAAAABgABAAAAAQAJABoAKwABACkAAAAiAAIAAQAAAAoSASq4ABbAAAGwAAAAAQAqAAAABgABAAAAAQACAB4AHwACACkAAAAfAAMAAwAAAAcqKxy3AByxAAAAAQAqAAAABgABAAAAAQAsAAAAAgAtEAoAJgAnAAEAKQAAAC8ABAAAAAAAFwa9AAFZA7IAA1NZBLIAB1NZBbIAClOwAAAAAQAqAAAABgABAAAAAQAIAC4ALQABACkAAABGAAQAAAAAAC67AAFZEiADtwAhswADuwABWRIiBLcAIbMAB7sAAVkSIwW3ACGzAAq4ACSzAA2xAAAAAQAqAAAABgABAAAAAQACACwAAAACAC8AMAAAAAIAMQ=="

func TestEnumSimple(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Color.java", contenido: enumFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el enum: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("se escribieron %d clases: %s", len(salida), nombresDe(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(enumJavac)
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
