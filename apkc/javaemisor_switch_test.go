package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// fuenteSwitch es el fuente que produjo el fixture de javac: una switch densa de ocho casos y otra
// dispersa de tres, cada brazo terminando en `break` y con su `default`.
const fuenteSwitch = `package p;
public class Prueba {
    static int total = 0;
    public static void denso(int n) {
        switch (n) {
            case 0: total = 100; break;
            case 1: total = 101; break;
            case 2: total = 102; break;
            case 3: total = 103; break;
            case 4: total = 104; break;
            case 5: total = 105; break;
            case 6: total = 106; break;
            case 7: total = 107; break;
            default: total = 999;
        }
    }
    public static void disperso(int n) {
        switch (n) {
            case 1: total = 1; break;
            case 100: total = 2; break;
            case 1000: total = 3; break;
            default: total = 9;
        }
    }
}
`

const pruebaJavac = "yv66vgAAAEEAFgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEACHAvUHJ1ZWJhAQAFdG90YWwBAAFJAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEABWRlbnNvAQAEKEkpVgEADVN0YWNrTWFwVGFibGUBAAhkaXNwZXJzbwEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEAC1BydWViYS5qYXZhACEACAACAAAAAQAIAAsADAAAAAQAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAACAAkADwAQAAEADQAAAMkAAQABAAAAdxqqAAAAAABvAAAAAAAAAAcAAAAvAAAANwAAAD8AAABHAAAATwAAAFcAAABfAAAAZxBkswAHpwBBEGWzAAenADkQZrMAB6cAMRBnswAHpwApEGizAAenACEQabMAB6cAGRBqswAHpwAREGuzAAenAAkRA+ezAAexAAAAAgAOAAAALgALAAAABQAwAAYAOAAHAEAACABIAAkAUAAKAFgACwBgAAwAaAANAHAADgB2ABAAEQAAAAwACjAHBwcHBwcHBwUACQASABAAAQANAAAAeAABAAEAAAA/GqsAAAAAADgAAAADAAAAAQAAACMAAABkAAAAKgAAA+gAAAAxBLMAB6cAFgWzAAenAA8GswAHpwAIEAmzAAexAAAAAgAOAAAAGgAGAAAAEgAkABMAKwAUADIAFQA5ABYAPgAYABEAAAAHAAUkBgYGBAAIABMABgABAA0AAAAdAAEAAAAAAAUDswAHsQAAAAEADgAAAAYAAQAAAAMAAQAUAAAAAgAV"

// TestElSwitchSeEscribeComoJavac: el `switch` del fuente, compilado por este fork, da el mismo
// `.class` (mismo `tableswitch` con el relleno desde el principio del `Code`, mismos destinos) y el
// mismo dex byte a byte que las clases de javac 21 para el mismo fuente. Es el oráculo doble aplicado
// al `switch`; el de la ejecución está en el §65 (ART y JVM dan lo mismo).
func TestElSwitchSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Prueba.java", contenido: fuenteSwitch}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente del switch: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("%d clases, una", len(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(pruebaJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
	}
	// y las dos formas: la densa acaba en `packed-switch` (0x2b) y la dispersa en `sparse-switch`
	// (0x2c). El selector no ocupa una instrucción propia: el traductor lo funde en el registro que
	// lee el switch, que es lo que hace con la condición del `if` (parrafo 52)
	d, err := readDex(deUno)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	conOpcode := func(nombre string, opcode byte) {
		ci, err := codeItemDeMetodo(d, nombre)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		for _, w := range ci.insns {
			if byte(w) == opcode {
				return
			}
		}
		t.Errorf("%s no lleva la forma %#02x en el cuerpo (%04x)", nombre, opcode, ci.insns)
	}
	conOpcode("denso", 0x2b)
	conOpcode("disperso", 0x2c)
}

// TestElSwitchDelFuenteRechazaLoNoMedido: la caída de un caso al siguiente, una clave que no es
// literal entero y una clave repetida se rechazan por nombre; `do` y `try` siguen fuera.
func TestElSwitchDelFuenteRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"caida", "package p;\npublic class A { static int t; public static void f(int n) { switch (n) { case 0: t = 1; case 1: t = 2; break; } } }\n", "caida"},
		{"clave", "package p;\npublic class A { static int t; static final int K = 3; public static void f(int n) { switch (n) { case K: t = 1; break; } } }\n", "literal entero"},
		{"repetida", "package p;\npublic class A { static int t; public static void f(int n) { switch (n) { case 1: t = 1; break; case 1: t = 2; break; } } }\n", "dos veces"},
		{"do", "package p;\npublic class A { static int t; public static void f(int n) { do { n = n + 1; } while (n < 3); } }\n", "do"},
		{"try", "package p;\npublic class A { static int t; public static void f() { try { f(); } catch (RuntimeException e) { f(); } } }\n", "try"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se aceptó algo que no está medido", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.pista) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
