package main

import (
	"bytes"
	"testing"
)

// fuenteTipo: la sonda del §131 para `instanceof` sobre un arreglo (`String[]` -> `[Ljava/lang/String;`
// y `int[]` -> `[I`) y para la forma negada (`if (!(o instanceof T))`, que javac branquea con `ifne`
// después del mismo `instanceof`).
const fuenteTipo = `package p;
public class Tipo {
    public static int f(Object o) {
        if (o instanceof String[]) return 1;
        return 0;
    }
    public static int g(Object o) {
        if (!(o instanceof String)) return 1;
        return 0;
    }
    public static int h(Object o) {
        if (!(o instanceof int[])) return 7;
        return 8;
    }
}
`

// tipoJavac: `javac --release 8 -g:lines,source Tipo.java` y `base64 -w0`.
const tipoJavac = "yv66vgAAADQAGAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQATW0xqYXZhL2xhbmcvU3RyaW5nOwcACgEAEGphdmEvbGFuZy9TdHJpbmcHAAwBAAJbSQcADgEABnAvVGlwbwEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAFmAQAVKExqYXZhL2xhbmcvT2JqZWN0OylJAQANU3RhY2tNYXBUYWJsZQEAAWcBAAFoAQAKU291cmNlRmlsZQEACVRpcG8uamF2YQAhAA0AAgAAAAAABAABAAUABgABAA8AAAAdAAEAAQAAAAUqtwABsQAAAAEAEAAAAAYAAQAAAAIACQARABIAAQAPAAAAMAABAAEAAAALKsEAB5kABQSsA6wAAAACABAAAAAKAAIAAAAEAAkABQATAAAAAwABCQAJABQAEgABAA8AAAAwAAEAAQAAAAsqwQAJmgAFBKwDrAAAAAIAEAAAAAoAAgAAAAgACQAJABMAAAADAAEJAAkAFQASAAEADwAAADIAAQABAAAADSrBAAuaAAYQB6wQCKwAAAACABAAAAAKAAIAAAAMAAoADQATAAAAAwABCgABABYAAAACABc="

// TestInstanceOfDeUnArregloYSuFormaNegada: el emisor acepta `instanceof T[]` (el descriptor del
// arreglo va al pool como tipo) y la negación de un `instanceof`, y el dex sale byte a byte el de javac.
func TestInstanceOfDeUnArregloYSuFormaNegada(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Tipo", fuenteTipo, tipoJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	for _, descriptor := range []string{"[Ljava/lang/String;", "[I"} {
		if !bytes.Contains(nuestro, []byte(descriptor)) {
			t.Errorf("el dex no nombra el tipo %s", descriptor)
		}
	}
}
