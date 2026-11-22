package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Frontera 6: el `ConstantValue` de un `static final long`. La sonda javac 17 del parrafo 130 es la
// misma `constantesJavac` que se usa en `javaemisor_tres_registros_test.go`: el campo `L` lleva la
// entrada `Long` (tag 5) y el `<clinit>` no escribe `putstatic`. Se comprueba aqui de forma aislada,
// sobre el atributo del `.class` y sobre el dex, para que no quede cubierto solo «de paso» por la
// prueba de la cuenta ancha.
func TestConstantValueDeStaticFinalLong(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Constantes.java", contenido: fuenteConstantes}}, jarDelSDK(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(salida) != 1 {
		t.Fatalf("%d clases, una", len(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(constantesJavac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		quien string
		cf    classFull
	}{{ "nuestra", nuestra }, { "javac", suya }} {
		campo := -1
		for i, f := range c.cf.Fields {
			if f.Name == "L" && f.Desc == "J" {
				campo = i
			}
		}
		if campo < 0 {
			t.Fatalf("%s: no está el campo L:J", c.quien)
		}
		f := c.cf.Fields[campo]
		if !f.HasConst || f.ConstTag != 5 || f.ConstLong != 6 {
			t.Errorf("%s: ConstantValue de L es tag %d, valor %d, presente %t; se esperaba 5, 6, true",
				c.quien, f.ConstTag, f.ConstLong, f.HasConst)
		}
		for _, m := range c.cf.Methods {
			if m.Name == "<clinit>" {
				if bytes.Contains(m.Code, []byte{0xb3}) {
					t.Errorf("%s: el <clinit> escribe putstatic", c.quien)
				}
			}
		}
	}
	nuestro, suyoDex := dexDeFuente(t, "Constantes", fuenteConstantes, constantesJavac)
	if !bytes.Equal(nuestro, suyoDex) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyoDex), primerByteDistinto(nuestro, suyoDex))
	}
}
