package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Sonda javac 17 (`javac --release 8 -g:lines,source Estrecha.java`): el estrechamiento a `byte` es
// `i2b` 0x91 y a `short` `i2s` 0x93, siempre sobre una palabra `int` (antes, si hace falta, la
// conversion ancha del parrafo 77). Cierra la frontera que el parrafo 87 dejaba para estos dos
// opcodes.
const estrechaFuente = `package p;
public class Estrecha {
    static byte a(int n) { return (byte) n; }
    static short b(int n) { return (short) n; }
    static byte c(long n) { return (byte) n; }
    static short d(double n) { return (short) n; }
    static int e(byte x, short y) { return x + y; }
    static byte f(byte x) { byte r = (byte) (x + 1); return r; }
}
`

const estrechaJavac = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAKcC9Fc3RyZWNoYQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAFhAQAEKEkpQgEAAWIBAAQoSSlTAQABYwEABChKKUIBAAFkAQAEKEQpUwEAAWUBAAUoQlMpSQEAAWYBAAQoQilCAQAKU291cmNlRmlsZQEADUVzdHJlY2hhLmphdmEAIQAHAAIAAAAAAAcAAQAFAAYAAQAJAAAAHQABAAEAAAAFKrcAAbEAAAABAAoAAAAGAAEAAAACAAgACwAMAAEACQAAABsAAQABAAAAAxqRrAAAAAEACgAAAAYAAQAAAAMACAANAA4AAQAJAAAAGwABAAEAAAADGpOsAAAAAQAKAAAABgABAAAABAAIAA8AEAABAAkAAAAcAAIAAgAAAAQeiJGsAAAAAQAKAAAABgABAAAABQAIABEAEgABAAkAAAAcAAIAAgAAAAQmjpOsAAAAAQAKAAAABgABAAAABgAIABMAFAABAAkAAAAcAAIAAgAAAAQaG2CsAAAAAQAKAAAABgABAAAABwAIABUAFgABAAkAAAAfAAIAAgAAAAcaBGCRPBusAAAAAQAKAAAABgABAAAACAABABcAAAACABg="

func TestI2BYI2S(t *testing.T) {
	nuestro, suyo := dexDeFuente(t, "Estrecha", estrechaFuente, estrechaJavac)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	datos, err := base64.StdEncoding.DecodeString(estrechaJavac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
	}
	dex, err := readDex(nuestro)
	if err != nil {
		t.Fatal(err)
	}
	// en el dex son `int-to-byte` 0x8d y `int-to-short` 0x8f, con el opcode en el byte bajo
	for metodo, opcode := range map[string]byte{"a": 0x8d, "b": 0x8f, "c": 0x8d, "d": 0x8f, "f": 0x8d} {
		ci, err := codeItemDeMetodo(dex, metodo)
		if err != nil {
			t.Fatal(err)
		}
		encontrado := false
		for _, w := range ci.insns {
			if byte(w) == opcode {
				encontrado = true
			}
		}
		if !encontrado {
			t.Errorf("%s: no lleva i2b/i2s 0x%02x en el dex", metodo, opcode)
		}
		enSonda := false
		for _, m := range suya.Methods {
			if m.Name == metodo {
				enSonda = true
			}
		}
		if !enSonda {
			t.Errorf("la sonda de javac no trae el metodo %s", metodo)
		}
	}
}
