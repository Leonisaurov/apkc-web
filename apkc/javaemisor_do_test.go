package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteDoo es la sonda del parrafo 69: un `do-while` con el cuerpo escribiendo un estático y un
// local, y otro que se ejecuta al menos una vez sin inicializar nada fuera.
const fuenteDoo = `public class Doo {
    static int total = 0;
    static void hasta(int n) {
        int i = 0;
        do {
            total = total + i;
            i++;
        } while (i < n);
    }
    static int alMenosUna(int n) {
        do {
            n = n - 1;
        } while (n > 0);
        return n;
    }
}
`

// dooJavac: `javac -g Doo.java` y `base64 -w0 Doo.class`.
const dooJavac = "yv66vgAAAEEAHAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAA0RvbwEABXRvdGFsAQABSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQAFTERvbzsBAAVoYXN0YQEABChJKVYBAAFuAQABaQEADVN0YWNrTWFwVGFibGUBAAphbE1lbm9zVW5hAQAEKEkpSQEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEACERvby5qYXZhACEACAACAAAAAQAIAAsADAAAAAQAAQAFAAYAAQANAAAALwABAAEAAAAFKrcAAbEAAAACAA4AAAAGAAEAAAABAA8AAAAMAAEAAAAFABAAEQAAAAgAEgATAAEADQAAAGMAAgACAAAAEwM8sgAHG2CzAAeEAQEbGqH/87EAAAADAA4AAAAWAAUAAAAEAAIABgAKAAcADQAIABIACQAPAAAAFgACAAAAEwAUAAwAAAACABEAFQAMAAEAFgAAAAYAAfwAAgEACAAXABgAAQANAAAARQACAAEAAAAKGgRkOxqd//sarAAAAAMADgAAAA4AAwAAAAwABAANAAgADgAPAAAADAABAAAACgAUAAwAAAAWAAAAAwABAAAIABkABgABAA0AAAAdAAEAAAAAAAUDswAHsQAAAAEADgAAAAYAAQAAAAIAAQAaAAAAAgAb"

// TestElDoWhileSeDexaComoJavac: el `do-while` escribe el cuerpo primero y después la condición
// **directa** saltando atrás a la cabecera (`if_icmplt 2`), al revés que el `while`; si la forma se
// aparta, el dex deja de ser el de javac byte a byte.
func TestElDoWhileSeDexaComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Doo.java", contenido: fuenteDoo}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(dooJavac)
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
