package main

import (
	"bytes"
	"testing"
)

const dosDimFuente = `public class Dos {
  static int f() {
    int[][] m = new int[2][3];
    m[0][1] = 7;
    return m[0][1] + m[1][2];
  }
}
`

// TestArregloDeDosDimensiones: `new int[2][3]` se baja como d8 (Array.newInstance con un int[] de
// dimensiones y check-cast) y `m[i][j]` encadena aaload e iaload. La ejecución en la JVM y el re-dex
// de d8 se miden en §168; aquí se sujeta la estructura del Code y que nuestro dexer lo acepte.
func TestArregloDeDosDimensiones(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Dos.java", contenido: dosDimFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	clase, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	var codigo []byte
	for _, m := range clase.Methods {
		if m.Name == "f" {
			codigo = m.Code
		}
	}
	if len(codigo) == 0 {
		t.Fatal("no se escribió el método f")
	}
	for nombre, op := range map[string]byte{
		"getstatic": 0xb2, "invokestatic": 0xb8, "checkcast": 0xc0, "aaload": 0x32, "iaload": 0x2e,
	} {
		if bytes.IndexByte(codigo, op) < 0 {
			t.Errorf("al Code le falta el opcode %s (%#02x)", nombre, op)
		}
	}
	dex, err := dexDeClases([]classFull{clase})
	if err != nil {
		t.Fatalf("nuestro dexer no leyó la clase: %v", err)
	}
	if len(dex) == 0 {
		t.Fatal("el dex salió vacío")
	}
}
