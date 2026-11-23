package main

// Prueba del dexado por clase (I4, dexreuso.go). El criterio del plan es una sola frase: el
// `classes.dex` de una vuelta que reutilizó clases tiene que ser el mismo fichero, byte a byte, que
// el de un dexado completo del mismo árbol. Aquí se comprueba en las dos direcciones que importan:
// una edición que no toca las tablas (se reutiliza casi todo) y una clase nueva que sí las mueve
// (no se puede reutilizar nada y la caché se tira), y en las dos el oráculo es volver a dexar desde
// cero sin caché y comparar.

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
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

// el mismo `Dos` de la vuelta anterior con el cuerpo de `cuenta` más largo: ni una cadena, ni un
// método, ni un campo nuevos —las tablas no se mueven—, pero el `code_item` crece y con él el área
// de código, así que todo lo que va detrás (la depuración, el class_data, los arrays y sus offsets)
// tiene que recalcularse. Es la vuelta que ejercita el desplazamiento de áreas que la edición de un
// solo literal no toca.
const fuenteReusoDosCrece = `package com.hola.reuso;

public class Dos {
    static int triple(int x) {
        return x * 7;
    }

    static int cuenta(int n) {
        int s = 0;
        int i = 0;
        while (i < n) {
            s = s + i;
            s = s + 1;
            s = s + 2;
            s = s + 3;
            i = i + 1;
        }
        return s;
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
		return leerDexDeLaEtapa(t)
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

	// 3. un cuerpo que CRECE, sin mover las tablas: el área de código se alarga, la depuración y el
	//    class_data se desplazan y los offsets de las áreas restantes tienen que recalcularse. La
	//    clase tocada se re-traduce entera y las demás se reutilizan; el oráculo vuelve a ser el
	//    dexado desde cero del mismo árbol.
	pon(t, raiz, "src/com/hola/reuso/Dos.java", fuenteReusoDosCrece)
	compilarFuentesEnGo(false)
	dexarClasesEnGo()
	if ultimoDexReuso.reutilizados == 0 {
		t.Fatal("con el cuerpo más largo no se reutilizó ningún code_item")
	}
	if ultimoDexReuso.reutilizados == ultimoDexReuso.total {
		t.Fatalf("reutilizó los %d code_item: el cuerpo alargado no se volvió a traducir",
			ultimoDexReuso.total)
	}
	crecido := leerDex()
	if len(crecido) <= len(completo) {
		t.Fatalf("el dex no creció al alargar el cuerpo: %d bytes frente a %d",
			len(crecido), len(completo))
	}
	if err := os.Remove(filepath.Join("build", "dexcache.bin")); err != nil {
		t.Fatal(err)
	}
	dexarClasesEnGo()
	if !bytes.Equal(crecido, leerDex()) {
		t.Fatal("el dex del cuerpo alargado con caché difiere del dexado completo")
	}

	// 4. una clase nueva mueve las tablas: lo guardado deja de nombrar lo mismo y se tira entero,
	//    y el dex sigue siendo idéntico al de un dexado completo de ese árbol
	pon(t, raiz, "src/com/hola/reuso/Cuatro.java", fuenteReusoCuatro)
	compilarFuentesEnGo(false)
	dexarClasesEnGo()
	if ultimoDexReuso.reutilizados != 0 {
		t.Fatalf("con las tablas cambiadas se reutilizaron %d code_item", ultimoDexReuso.reutilizados)
	}
	conCuatro := leerDex()
	if bytes.Equal(crecido, conCuatro) {
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

// leerDexDeLaEtapa devuelve el `build/classes.dex` que dejó la última etapa.
func leerDexDeLaEtapa(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("build", "classes.dex"))
	if err != nil {
		t.Fatalf("no hay build/classes.dex: %v", err)
	}
	return b
}

// conResumen rehace una caché a partir de su cuerpo: marca y SHA-1 delante. Sirve para fabricar
// ficheros con el resumen correcto y el contenido imposible, que es la única forma de llegar al
// parser con longitudes del fichero sin que el resumen los descarte antes.
func conResumen(cuerpo []byte) []byte {
	resumen := sha1.Sum(cuerpo)
	b := append([]byte(nil), cacheDexMarca...)
	b = append(b, resumen[:]...)
	return append(b, cuerpo...)
}

// TestCacheDexCorruptaCaeAlDexadoCompleto cierra el segundo `INCIERTO` del crítico de I4: una caché
// ilegible —truncada, con una longitud imposible o con un byte cambiado en los blobs— no puede
// tumbar el build ni colarse en el dex. Se cae al dexado completo, con aviso, y el fichero sale
// idéntico al de referencia.
func TestCacheDexCorruptaCaeAlDexadoCompleto(t *testing.T) {
	androidJar = jarDelSDK(t)
	raiz := t.TempDir()
	pon(t, raiz, "src/com/hola/reuso/Uno.java", fuenteReusoUno)
	pon(t, raiz, "src/com/hola/reuso/Dos.java", fuenteReusoDos)
	pon(t, raiz, "src/com/hola/reuso/Tres.java", fuenteReusoTres)
	t.Chdir(raiz)
	ultimoDexReuso = resumenCacheDex{}

	compilarFuentesEnGo(false)
	dexarClasesEnGo()
	referencia := leerDexDeLaEtapa(t)
	buena, err := os.ReadFile(cacheDexFichero)
	if err != nil {
		t.Fatalf("la etapa no dejó la caché: %v", err)
	}
	cabecera := len(cacheDexMarca) + 20
	if len(buena) <= cabecera+20 {
		t.Fatalf("la caché mide %d bytes: no hay cuerpo que corromper", len(buena))
	}

	// el cuerpo con la longitud del primer blob puesta a 0xFFFFFFFF: la huella ocupa 20 bytes, el
	// recuento de la sección de código otros 4 y la primera entrada empieza con sus 20 de clave
	imposible := append([]byte(nil), buena[cabecera:]...)
	binary.LittleEndian.PutUint32(imposible[44:], 0xffffffff)
	if c := nuevoCacheDex(); c.leeCuerpo(imposible) {
		t.Fatal("el parser aceptó un blob de cuatro gigabytes: falta el guard del desbordamiento")
	}

	cambiado := append([]byte(nil), buena...)
	cambiado[len(cambiado)-1] ^= 0xff

	casos := []struct {
		nombre string
		bytes  []byte
	}{
		{"truncada a la mitad", buena[:len(buena)/2]},
		{"sin cuerpo", buena[:cabecera]},
		{"marca de otra versión", append([]byte("apkc-dex-reuso-1\n"), buena[len(cacheDexMarca):]...)},
		{"longitud de cuatro gigabytes con el resumen bueno", conResumen(imposible)},
		{"un byte cambiado en un blob", cambiado},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if err := os.WriteFile(cacheDexFichero, caso.bytes, 0644); err != nil {
				t.Fatal(err)
			}
			dexarClasesEnGo()
			if ultimoDexReuso.reutilizados != 0 {
				t.Fatalf("se reutilizaron %d code_item de una caché ilegible",
					ultimoDexReuso.reutilizados)
			}
			if !bytes.Equal(leerDexDeLaEtapa(t), referencia) {
				t.Fatal("el dex de la vuelta con la caché corrupta difiere del de referencia")
			}
			// y la vuelta deja una caché sana para la siguiente
			if _, err := os.Stat(cacheDexFichero); err != nil {
				t.Fatalf("la vuelta no repuso la caché: %v", err)
			}
		})
	}
}
