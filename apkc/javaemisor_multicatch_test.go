package main

import (
	"bytes"
	"testing"
)

// fuenteMulti: la sonda del §132 para `catch (A | B e)`: una fila de la tabla por tipo, un solo
// manejador y la excepcion guardada en el mismo hueco, con y sin `finally`.
const fuenteMulti = `package p;
public class Multi {
    static int t;
    public static void f(int n) {
        try {
            if (n == 0) throw new IllegalArgumentException();
            if (n == 1) throw new IllegalStateException();
            t = 3;
        } catch (IllegalArgumentException | IllegalStateException e) {
            t = 9;
        }
    }
    public static void g(int n) {
        try {
            if (n == 0) throw new IllegalArgumentException();
            t = 3;
        } catch (IllegalArgumentException | IllegalStateException e) {
            t = 9;
        } finally {
            t = t + 1;
        }
    }
}
`

// multiJavac: `javac --release 8 -g:lines,source Multi.java` y `base64 -w0`.
const multiJavac = "yv66vgAAADQAHwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAiamF2YS9sYW5nL0lsbGVnYWxBcmd1bWVudEV4Y2VwdGlvbgoABwADBwALAQAfamF2YS9sYW5nL0lsbGVnYWxTdGF0ZUV4Y2VwdGlvbgoACgADCQAOAA8HABAMABEAEgEAB3AvTXVsdGkBAAF0AQABSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAFmAQAEKEkpVgEADVN0YWNrTWFwVGFibGUHABkBABpqYXZhL2xhbmcvUnVudGltZUV4Y2VwdGlvbgEAAWcHABwBABNqYXZhL2xhbmcvVGhyb3dhYmxlAQAKU291cmNlRmlsZQEACk11bHRpLmphdmEAIQAOAAIAAAABAAgAEQASAAAAAwABAAUABgABABMAAAAdAAEAAQAAAAUqtwABsQAAAAEAFAAAAAYAAQAAAAIACQAVABYAAQATAAAAdgACAAIAAAAnGpoAC7sAB1m3AAm/GgSgAAu7AApZtwAMvwazAA2nAAlMEAmzAA2xAAIAAAAdACAABwAAAB0AIAAKAAIAFAAAAB4ABwAAAAYADAAHABkACAAdAAsAIAAJACEACgAmAAwAFwAAAAkABAwMRgcAGAUACQAaABYAAQATAAAAqgACAAMAAAA4GpoAC7sAB1m3AAm/BrMADbIADQRgswANpwAfTBAJswANsgANBGCzAA2nAA5NsgANBGCzAA0sv7EABAAAABAAGwAHAAAAEAAbAAoAAAAQACwAAAAbACEALAAAAAIAFAAAAC4ACwAAAA8ADAAQABAAFAAYABUAGwARABwAEgAhABQAKQAVACwAFAA1ABUANwAWABcAAAAMAAQMTgcAGFAHABsKAAEAHQAAAAIAHg=="

// TestElCatchDeVariosTipos: el `.class` que produce el emisor y el de javac se dexan igual: una
// entrada de tabla por tipo, un `astore` y un cuerpo, también con el `finally` copiado.
func TestElCatchDeVariosTipos(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Multi", fuenteMulti, multiJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}
