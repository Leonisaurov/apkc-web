package main

import (
	"bytes"
	"testing"
)

const switchCadenaFuente = `public class CadenaSwitch {
  static int f(String s) {
    int r = 0;
    switch (s) {
      case "uno": r = 1; break;
      case "dos": r = 2; break;
      default: r = 9;
    }
    return r;
  }
}
`

// TestSwitchDeCadena: `switch (String)` se baja a igualdad con equals más un switch del índice
// del brazo. La ejecución en la JVM se mide en §178; aquí se sujeta la estructura y que el dexer
// acepte el resultado.
func TestSwitchDeCadena(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/CadenaSwitch.java", contenido: switchCadenaFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el switch de cadenas: %v", err)
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
	if bytes.IndexByte(codigo, 0xb6) < 0 { // invokevirtual
		t.Error("al Code le falta la llamada a String.equals")
	}
	if bytes.IndexByte(codigo, 0xaa) < 0 && bytes.IndexByte(codigo, 0xab) < 0 {
		t.Error("al Code le falta el switch de índices")
	}
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("nuestro dexer rechazó el switch: %v", err)
	}
}
