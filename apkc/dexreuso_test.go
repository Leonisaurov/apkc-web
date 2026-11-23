package main

// Prueba del dexado por clase (I4, dexreuso.go). El criterio del plan es una sola frase: el
// `classes.dex` de una vuelta que reutilizó clases tiene que ser el mismo fichero, byte a byte, que
// el de un dexado completo del mismo árbol. Aquí se comprueba en las dos direcciones que importan:
// una edición que no toca las tablas (se reutiliza casi todo) y una clase nueva que sí las mueve
// (no se puede reutilizar nada y la caché se tira), y en las dos el oráculo es volver a dexar desde
// cero sin caché y comparar.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const fuenteReusoUno = `package com.hola.reuso;

public class Uno {
    static int doble(int x) {
        return x * 2;
    }

    static int suma(int a, int b) {
        int s = a + b;
        return s;
    }
}
`

const fuenteReusoDos = `package com.hola.reuso;

public class Dos {
    static int triple(int x) {
        return x * 3;
    }

    static int cuenta(int n) {
        int s = 0;
        int i = 0;
        while (i < n) {
            s = s + i;
            i = i + 1;
        }
        return s;
    }
}
`

// la misma clase con otro cuerpo y los mismos nombres: no entra ni sale ninguna cadena, ningún
// método y ningún campo, así que las cinco tablas del dex quedan iguales y lo guardado sigue valiendo
const fuenteReusoDosEditada = `package com.hola.reuso;

public class Dos {
    static int triple(int x) {
        return x * 7;
    }

    static int cuenta(int n) {
        int s = 0;
        int i = 0;
        while (i < n) {
            s = s + i;
            i = i + 1;
        }
        return s;
    }
}
`

const fuenteReusoTres = `package com.hola.reuso;

public class Tres {
    static String nombre() {
        return "tres";
    }

    static int resto(int x) {
        return x - 10;
    }
}
`

const fuenteReusoCuatro = `package com.hola.reuso;

public class Cuatro {
    static int resta(int a, int b) {
        return a - b;
    }
}
`

func TestDexadoPorClaseReutilizaYNoCambiaUnByte(t *testing.T) {
	androidJar = jarDelSDK(t)
	raiz := t.TempDir()
	pon(t, raiz, "src/com/hola/reuso/Uno.java", fuenteReusoUno)
	pon(t, raiz, "src/com/hola/reuso/Dos.java", fuenteReusoDos)
	pon(t, raiz, "src/com/hola/reuso/Tres.java", fuenteReusoTres)
	t.Chdir(raiz)
	ultimoDexReuso = resumenCacheDex{}

	leerDex := func() []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join("build", "classes.dex"))
		if err != nil {
			t.Fatalf("no hay build/classes.dex: %v", err)
		}
		return b
	}

	// 1. la primera vuelta no tiene de dónde reutilizar y deja la caché escrita
	compilarFuentesEnGo(false)
	dexarClasesEnGo()
	if ultimoDexReuso.reutilizados != 0 {
		t.Fatalf("sin caché previa se reutilizaron %d code_item", ultimoDexReuso.reutilizados)
	}
	if ultimoDexReuso.total == 0 {
		t.Fatal("la etapa no contó ningún método con cuerpo")
	}
	if _, err := os.Stat(filepath.Join("build", "dexcache.bin")); err != nil {
		t.Fatalf("la vuelta no dejó la caché: %v", err)
	}
	primero := leerDex()

	// 2. un cuerpo cambia y las tablas no: se reutiliza el resto y el fichero no puede moverse
	pon(t, raiz, "src/com/hola/reuso/Dos.java", fuenteReusoDosEditada)
	compilarFuentesEnGo(false)
	dexarClasesEnGo()
	if ultimoDexReuso.reutilizados == 0 {
		t.Fatal("la vuelta no reutilizó ningún code_item: la caché no está sirviendo")
	}
	if ultimoDexReuso.reutilizados == ultimoDexReuso.total {
		t.Fatalf("reutilizó los %d code_item: el método editado no se volvió a traducir",
			ultimoDexReuso.total)
	}
	caliente := leerDex()
	if bytes.Equal(primero, caliente) {
		t.Fatal("la edición no llegó al dex")
	}

	// el oráculo: el mismo árbol dexado desde cero, sin caché
	if err := os.Remove(filepath.Join("build", "dexcache.bin")); err != nil {
		t.Fatal(err)
	}
	dexarClasesEnGo()
	if ultimoDexReuso.reutilizados != 0 {
		t.Fatal("sin caché en disco no se puede reutilizar nada")
	}
	completo := leerDex()
	if !bytes.Equal(caliente, completo) {
		t.Fatalf("el dex con caché mide %d bytes y el completo %d, y difieren",
			len(caliente), len(completo))
	}

	// 3. una clase nueva mueve las tablas: lo guardado deja de nombrar lo mismo y se tira entero,
	//    y el dex sigue siendo idéntico al de un dexado completo de ese árbol
	pon(t, raiz, "src/com/hola/reuso/Cuatro.java", fuenteReusoCuatro)
	compilarFuentesEnGo(false)
	dexarClasesEnGo()
	if ultimoDexReuso.reutilizados != 0 {
		t.Fatalf("con las tablas cambiadas se reutilizaron %d code_item", ultimoDexReuso.reutilizados)
	}
	conCuatro := leerDex()
	if bytes.Equal(completo, conCuatro) {
		t.Fatal("la clase nueva no llegó al dex")
	}
	if err := os.Remove(filepath.Join("build", "dexcache.bin")); err != nil {
		t.Fatal(err)
	}
	dexarClasesEnGo()
	if !bytes.Equal(conCuatro, leerDex()) {
		t.Fatal("el dex de la vuelta con la caché tirada difiere del completo")
	}
}
