package main

import (
	"bytes"
	"testing"
)

// fuenteDobleFinally: un `return` que cruza dos `finally` anidados. javac copia los dos bloques de
// dentro a fuera y el rango exterior incluye la copia del interior, así que la tabla solapa (§139).
const fuenteDobleFinally = `package p;
public class D {
  static int t;
  public static int f(int n) {
    try {
      try {
        if (n == 0) return 1;
        t = 2;
      } finally {
        t = t + 10;
      }
    } finally {
      t = t + 100;
    }
    return 0;
  }
  public static int g() {
    int r = 0;
    for (int i = 0; i < 4; i++) {
      try {
        try {
          if (i == 1) continue;
          if (i == 2) break;
          r = r + i;
        } finally {
          r = r + 10;
        }
      } finally {
        r = r + 100;
      }
    }
    return r;
  }
}
`

// dobleFinallyJavac: `javac --release 8 -g:lines,source D.java` y `base64 -w0`.
const dobleFinallyJavac = "yv66vgAAADQAGAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAA3AvRAEAAXQBAAFJAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAWYBAAQoSSlJAQANU3RhY2tNYXBUYWJsZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUBAAFnAQADKClJAQAKU291cmNlRmlsZQEABkQuamF2YQAhAAgAAgAAAAEACAALAAwAAAADAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAgAJAA8AEAABAA0AAADPAAIABAAAAFAamgAZBDyyAAcQCmCzAAeyAAcQZGCzAAcbrAWzAAeyAAcQCmCzAAenAA9NsgAHEApgswAHLL+yAAcQZGCzAAenAA9OsgAHEGRgswAHLb8DrAAEAAAABgAqAAAAGgAeACoAAAAAAA8AQgAAABoANgBCAAAAAgAOAAAAOgAOAAAABwAGAAoADwANABgABwAaAAgAHgAKACcACwAqAAoANAALADYADQA/AA4AQgANAEwADgBOAA8AEQAAAA0ABRpPBwASC0sHABILAAkAFAAVAAEADQAAAQ0AAgAEAAAAWQM7AzwbB6IAURsEoAAQGhAKYDsaEGRgO6cAORsFoAAQGhAKYDsaEGRgO6cALRobYDsaEApgO6cAC00aEApgOyy/GhBkYDunAAtOGhBkYDstv4QBAaf/sBqsAAYACQAOADkAAAAbACAAOQAAAC0AMQA5AAAACQATAEkAAAAbACUASQAAAC0AQQBJAAAAAgAOAAAAVgAVAAAAEgACABMACQAWAA4AGgATAB0AGAAWABsAFwAgABoAJQAdACoAFwAtABgAMQAaADYAGwA5ABoAPwAbAEEAHQBGAB4ASQAdAE8AHgBRABMAVwAgABEAAAAWAAj9AAQBARYRSwcAEgdHBwASB/oABQABABYAAAACABc="

// TestLaSalidaQueCruzaDosFinally: el dex sale byte a byte el de javac aunque la tabla del `.class`
// solape rangos; cubre `return`, `continue` y `break` que cruzan los dos bloques.
func TestLaSalidaQueCruzaDosFinally(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "D", fuenteDobleFinally, dobleFinallyJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}
