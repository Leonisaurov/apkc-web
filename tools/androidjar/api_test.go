package main

import (
	"strings"
	"testing"
)

const apiMini = `// Signature format: 2.0
package java.lang {

  public class Object {
    ctor public Object();
  }

  public interface CharSequence {
    method public int length();
  }

  public final class String implements java.lang.CharSequence {
    ctor public String();
    method public int length();
  }
}

package android.demo {

  @UiContext public class Base extends java.lang.Object implements java.lang.Runnable {
    ctor public Base();
    field public static final int NUMBER = 42;
    method public <T extends android.demo.Base> T self(int);
    method public void run();
  }
}
`

func TestLeeAPIMinima(t *testing.T) {
	a, err := leeAPI(strings.NewReader(apiMini))
	if err != nil {
		t.Fatal(err)
	}
	b := a.clases["android.demo.Base"]
	if b == nil {
		t.Fatal("no se leyó android.demo.Base")
	}
	if b.interno != "android/demo/Base" {
		t.Errorf("interno %q", b.interno)
	}
	if b.super != "java/lang/Object" {
		t.Errorf("super %q", b.super)
	}
	if len(b.interfaces) != 1 || b.interfaces[0] != "java/lang/Runnable" {
		t.Errorf("interfaces %v", b.interfaces)
	}
	if len(b.campos) != 1 || b.campos[0].nombre != "NUMBER" || !b.campos[0].tieneVal || b.campos[0].valor.i != 42 {
		t.Errorf("campo %+v", b.campos)
	}
	var self *apiMetodo
	for _, m := range b.metodos {
		if m.nombre == "self" {
			self = m
		}
	}
	if self == nil {
		t.Fatal("no se leyó self")
	}
	if self.desc != "(I)Landroid/demo/Base;" {
		t.Errorf("self descriptor %q; el parámetro T debe borrarse a su cota", self.desc)
	}
	if s := a.clases["java.lang.String"]; s == nil || s.interno != "java/lang/String" {
		t.Errorf("String no quedó como java/lang/String: %+v", s)
	}
}

func TestFormatoDe(t *testing.T) {
	if got := formatoDe("dimension|enum"); got != 0x00010040 {
		t.Errorf("dimension|enum = %#x", got)
	}
	if got := formatoDe("flags"); got != 0x00020000 {
		t.Errorf("flags = %#x", got)
	}
}

func TestArscMinimo(t *testing.T) {
	recursos := []recurso{
		{tipo: "attr", nombre: "gravity", id: 0x010100af},
		{tipo: "id", nombre: "center", id: 0x0102c000},
		{tipo: "style", nombre: "Theme", id: 0x01030000},
	}
	attrs := map[string]*defAtributo{
		"gravity": {formato: 0x00020000, flags: true, items: map[string]int64{"center": 0x11}},
	}
	b, err := escribeARSC(recursos, attrs)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 12 || b[0] != 0x02 || b[1] != 0x00 {
		t.Fatalf("no empieza como RES_TABLE: % x", b[:min(12, len(b))])
	}
	// el paquete declara tres tipos con entradas
	if len(b) < 300 {
		t.Errorf("tabla de %d bytes: no parece tener paquete", len(b))
	}
}
