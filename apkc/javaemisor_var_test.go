package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

const varFuente = `public class Var {
  static int f(int n) { var x = n + 1; var y = x * 2; return y; }
}
`

// varJavac: `javac --release 11 -g Var.java` con javac 17 (var no existe en --release 8).
const varJavac = "yv66vgAAADcAFgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQADVmFyAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAEkxvY2FsVmFyaWFibGVUYWJsZQEABHRoaXMBAAVMVmFyOwEAAWYBAAQoSSlJAQABbgEAAUkBAAF4AQABeQEAClNvdXJjZUZpbGUBAAhWYXIuamF2YQAhAAcAAgAAAAAAAgABAAUABgABAAkAAAAvAAEAAQAAAAUqtwABsQAAAAIACgAAAAYAAQAAAAEACwAAAAwAAQAAAAUADAANAAAACAAOAA8AAQAJAAAASAACAAMAAAAKGgRgPBsFaD0crAAAAAIACgAAAAYAAQAAAAIACwAAACAAAwAAAAoAEAARAAAABAAGABIAEQABAAgAAgATABEAAgABABQAAAACABU="

func TestInferenciaVar(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Var.java", contenido: varFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó var: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(varJavac)
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
