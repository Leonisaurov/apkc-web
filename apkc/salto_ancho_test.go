package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestSaltoAncho: un método de más de 32 KB de código obliga a javac a escribir `goto_w` y al
// dexer a bajarlo a `goto/16`. La sonda se genera aquí para no versionar un fuente de 50 KB.
func TestSaltoAncho(t *testing.T) {
	cuerpo := strings.Repeat("      x = x + 1;\n", 10000)
	fuente := fmt.Sprintf(`public class SaltoAncho {
  static int f(int n) {
    int x = 0;
    if (n > 0) {
%s    } else {
      x = 5;
    }
    return x;
  }
}
`, cuerpo)
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/SaltoAncho.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el método grande: %v", err)
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
	if len(codigo) <= 32767 {
		t.Fatalf("el método mide %d bytes: la sonda no alcanzó a forzar goto_w", len(codigo))
	}
	if bytes.IndexByte(codigo, 0xc8) < 0 {
		t.Fatal("al Code le falta el opcode goto_w (0xc8)")
	}
	dex, err := dexDeClases([]classFull{clase})
	if err != nil {
		t.Fatalf("nuestro dexer no bajó el goto_w: %v", err)
	}
	if len(dex) == 0 {
		t.Fatal("el dex salió vacío")
	}
}
