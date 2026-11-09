package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteNuevo: `new A(b)` con un argumento de un subtipo. javac firma el constructor con el
// descriptor de la DECLARACION `(Lp/Nuevo$A;)V`; firmarlo con el tipo del argumento compila y ART
// corta con `NoSuchMethodError` al arrancar la app de cierre (§136).
const fuenteNuevo = ""
const fuenteA = `package p;
public class A {
    public int v;
    public A(int x) {
        v = x;
    }
    public A(A otra) {
        v = otra.v + 1;
    }
}
`
const fuenteB = `package p;
public class B extends A {
    public B() {
        super(1);
    }
}
`
const fuenteNuevoF = `package p;
public class Nuevo {
    public static int f() {
        B b = new B();
        A a = new A(b);
        return a.v;
    }
}
`

// nuevoJavac son los tres `.class` de `javac --release 8 -g:lines,source Nuevo.java`, en el orden
// Nuevo$A, Nuevo$B y Nuevo.
var nuevoJavac = []string{
	"yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAA3AvQQEAAXYBAAFJAQAEKEkpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAgoTHAvQTspVgEAClNvdXJjZUZpbGUBAAZBLmphdmEAIQAIAAIAAAABAAEACwAMAAAAAgABAAUADQABAA4AAAAqAAIAAgAAAAoqtwABKhu1AAexAAAAAQAPAAAADgADAAAABAAEAAUACQAGAAEABQAQAAEADgAAAC8AAwACAAAADyq3AAEqK7QABwRgtQAHsQAAAAEADwAAAA4AAwAAAAcABAAIAA4ACQABABEAAAACABI=",
	"yv66vgAAADQADgoAAgADBwAEDAAFAAYBAANwL0EBAAY8aW5pdD4BAAQoSSlWBwAIAQADcC9CAQADKClWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNvdXJjZUZpbGUBAAZCLmphdmEAIQAHAAIAAAAAAAEAAQAFAAkAAQAKAAAAIgACAAEAAAAGKgS3AAGxAAAAAQALAAAACgACAAAABAAFAAUAAQAMAAAAAgAN",
	"yv66vgAAADQAGwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQADcC9CCgAHAAMHAAsBAANwL0EKAAoADQwABQAOAQAIKExwL0E7KVYJAAoAEAwAEQASAQABdgEAAUkHABQBAAdwL051ZXZvAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAWYBAAMoKUkBAApTb3VyY2VGaWxlAQAKTnVldm8uamF2YQAhABMAAgAAAAAAAgABAAUABgABABUAAAAdAAEAAQAAAAUqtwABsQAAAAEAFgAAAAYAAQAAAAIACQAXABgAAQAVAAAANgADAAIAAAAWuwAHWbcACUu7AApZKrcADEwrtAAPrAAAAAEAFgAAAA4AAwAAAAQACAAFABEABgABABkAAAACABo=",
}

// TestElConstructorSeFirmaConSuDescriptor: con un argumento de un subtipo, el `invokespecial` lleva
// `(Lp/Nuevo$A;)V`, no el tipo del argumento; el dex de las tres clases sale byte a byte el de javac.
func TestElConstructorSeFirmaConSuDescriptor(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/p/A.java", contenido: fuenteA},
		{nombre: "src/p/B.java", contenido: fuenteB},
		{nombre: "src/p/Nuevo.java", contenido: fuenteNuevoF},
	}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 3 {
		t.Fatalf("%d clases, tres (Nuevo, Nuevo$A y Nuevo$B)", len(salida))
	}
	nuestras := make([]classFull, 0, 3)
	for _, s := range salida {
		c, err := readClassFull(s.datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestras = append(nuestras, c)
	}
	suyas := make([]classFull, 0, 3)
	for _, b64 := range nuevoJavac {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		c, err := readClassFull(datos)
		if err != nil {
			t.Fatal(err)
		}
		suyas = append(suyas, c)
	}
	nuestro, err := dexDeClases(nuestras)
	if err != nil {
		t.Fatal(err)
	}
	suyo, err := dexDeClases(suyas)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}
