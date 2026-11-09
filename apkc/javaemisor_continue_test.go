package main

import (
	"bytes"
	"testing"
)

// fuenteCont: la sonda del §132 para el `continue` que sale de un `try` con `finally`: la copia del
// bloque va delante del `goto` a la cabeza del bucle y el tramo protegido se corta donde acaba el
// salto, como el `break`.
const fuenteCont = `package p;
public class Cont {
    static int t;
    public static void f() {
        for (int i = 0; i < 3; i++) {
            try {
                if (i == 1) continue;
                t = t + i;
            } finally {
                t = t + 10;
            }
        }
    }
}
`

// contJavac: `javac --release 8 -g:lines,source Cont.java` y `base64 -w0`.
const contJavac = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABnAvQ29udAEAAXQBAAFJAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAWYBAA1TdGFja01hcFRhYmxlBwASAQATamF2YS9sYW5nL1Rocm93YWJsZQEAClNvdXJjZUZpbGUBAAlDb250LmphdmEAIQAIAAIAAAABAAgACwAMAAAAAgABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAIACQAPAAYAAQANAAAApAACAAIAAAA/AzsaBqIAOhoEoAAPsgAHEApgswAHpwAjsgAHGmCzAAeyAAcQCmCzAAenAA9MsgAHEApgswAHK7+EAAGn/8exAAIABwAMACwAAAAYACAALAAAAAIADgAAAC4ACwAAAAUABwAHAAwACgAVAAcAGAAIACAACgApAAsALAAKADYACwA4AAUAPgANABAAAAAPAAX8AAIBFVMHABEL+gAFAAEAEwAAAAIAFA=="

// TestElContinueQueSaleDeUnTryConFinally: el dex del fuente sale byte a byte el de javac; la copia
// del `finally` va antes del `goto` que vuelve a la cabeza del bucle.
func TestElContinueQueSaleDeUnTryConFinally(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Cont", fuenteCont, contJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}
