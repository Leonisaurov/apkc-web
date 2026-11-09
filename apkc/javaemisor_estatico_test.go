package main

import (
	"bytes"
	"testing"
)

// fuenteLlamada: una clase con metodos `static` llamados sin receptor desde un metodo de instancia.
// javac escribe `invokestatic`; emitir `invokevirtual` compila pero ART corta con
// `IncompatibleClassChangeError` (medido al instalar la app de cierre, §136).
const fuenteLlamada = `package p;
public class Llamada {
    static int doble(int x) {
        return x + x;
    }
    static int seis(int a, int b, int c, int d, int e, int f) {
        return a + b + c + d + e + f;
    }
    public int f() {
        return doble(3) + seis(1, 2, 3, 4, 5, 6);
    }
    public int g(int z) {
        return doble(z);
    }
}
`

// llamadaJavac: `javac --release 8 -g:lines,source Llamada.java` y `base64 -w0`.
const llamadaJavac = "yv66vgAAADQAGAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEACXAvTGxhbWFkYQEABWRvYmxlAQAEKEkpSQoACAAODAAPABABAARzZWlzAQAJKElJSUlJSSlJAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAWYBAAMoKUkBAAFnAQAKU291cmNlRmlsZQEADExsYW1hZGEuamF2YQAhAAgAAgAAAAAABQABAAUABgABABEAAAAdAAEAAQAAAAUqtwABsQAAAAEAEgAAAAYAAQAAAAIACAALAAwAAQARAAAAHAACAAEAAAAEGhpgrAAAAAEAEgAAAAYAAQAAAAQACAAPABAAAQARAAAAJgACAAYAAAAOGhtgHGAdYBUEYBUFYKwAAAABABIAAAAGAAEAAAAHAAEAEwAUAAEAEQAAACgABwABAAAAEAa4AAcEBQYHCBAGuAANYKwAAAABABIAAAAGAAEAAAAKAAEAFQAMAAEAEQAAAB0AAQACAAAABRu4AAesAAAAAQASAAAABgABAAAADQABABYAAAACABc="

// TestLosEstaticosSeLlamanSinReceptor: el dex lleva `invoke-static` en los dos metodos y sale byte a
// byte el de javac.
func TestLosEstaticosSeLlamanSinReceptor(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Llamada", fuenteLlamada, llamadaJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}
