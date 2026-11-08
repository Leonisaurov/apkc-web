package main

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"testing"
)

// fuenteClase es la sonda del párrafo 89: el literal de clase `X.class`. Lo que fija cada método:
//
//	deCadena / deObjeto / deArreglo   `String.class`, `Object.class`, `String[].class` → `ldc` de CONSTANT_Class
//	dePrimitivo / deVoid              `int.class`, `void.class` → `getstatic Integer.TYPE` / `Void.TYPE`
//	deArregloPrimitivo                `int[].class` → `ldc` con el descriptor `[I`
//	esCadena                          `o.getClass() == String.class` → `invokevirtual; ldc; if_acmpne`
//	nombre                            `String.class.getName()` → `ldc; invokevirtual Class.getName`
//	local                             `Class c = int.class` → `getstatic; astore_0`
//	guardada                          `static Class guardada = String.class` → `ldc` en el `<clinit>`
const fuenteClase = `public class Clase {
    static Class guardada = String.class;
    static Class deCadena() { return String.class; }
    static Class deObjeto() { return Object.class; }
    static Class deArreglo() { return String[].class; }
    static Class dePrimitivo() { return int.class; }
    static Class deVoid() { return void.class; }
    static Class deArregloPrimitivo() { return int[].class; }
    static boolean esCadena(Object o) { return o.getClass() == String.class; }
    static String nombre() { return String.class.getName(); }
    static Class local() { Class c = int.class; return c; }
}
`

// claseJavac: `javac --release 8 -g:lines,source Clase.java` y `base64 -w0 Clase.class`.
const claseJavac = "yv66vgAAADQANQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAQamF2YS9sYW5nL1N0cmluZwcACgEAE1tMamF2YS9sYW5nL1N0cmluZzsJAAwADQcADgwADwAQAQARamF2YS9sYW5nL0ludGVnZXIBAARUWVBFAQARTGphdmEvbGFuZy9DbGFzczsJABIADQcAEwEADmphdmEvbGFuZy9Wb2lkBwAVAQACW0kKAAIAFwwAGAAZAQAIZ2V0Q2xhc3MBABMoKUxqYXZhL2xhbmcvQ2xhc3M7CgAbABwHAB0MAB4AHwEAD2phdmEvbGFuZy9DbGFzcwEAB2dldE5hbWUBABQoKUxqYXZhL2xhbmcvU3RyaW5nOwkAIQAiBwAjDAAkABABAAVDbGFzZQEACGd1YXJkYWRhAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEACGRlQ2FkZW5hAQAIZGVPYmpldG8BAAlkZUFycmVnbG8BAAtkZVByaW1pdGl2bwEABmRlVm9pZAEAEmRlQXJyZWdsb1ByaW1pdGl2bwEACGVzQ2FkZW5hAQAVKExqYXZhL2xhbmcvT2JqZWN0OylaAQANU3RhY2tNYXBUYWJsZQEABm5vbWJyZQEABWxvY2FsAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAKQ2xhc2UuamF2YQAhACEAAgAAAAEACAAkABAAAAALAAEABQAGAAEAJQAAAB0AAQABAAAABSq3AAGxAAAAAQAmAAAABgABAAAAAQAIACcAGQABACUAAAAbAAEAAAAAAAMSB7AAAAABACYAAAAGAAEAAAADAAgAKAAZAAEAJQAAABsAAQAAAAAAAxICsAAAAAEAJgAAAAYAAQAAAAQACAApABkAAQAlAAAAGwABAAAAAAADEgmwAAAAAQAmAAAABgABAAAABQAIACoAGQABACUAAAAcAAEAAAAAAASyAAuwAAAAAQAmAAAABgABAAAABgAIACsAGQABACUAAAAcAAEAAAAAAASyABGwAAAAAQAmAAAABgABAAAABwAIACwAGQABACUAAAAbAAEAAAAAAAMSFLAAAAABACYAAAAGAAEAAAAIAAgALQAuAAEAJQAAADIAAgABAAAADyq2ABYSB6YABwSnAAQDrAAAAAIAJgAAAAYAAQAAAAkALwAAAAUAAg1AAQAIADAAHwABACUAAAAeAAEAAAAAAAYSB7YAGrAAAAABACYAAAAGAAEAAAAKAAgAMQAZAAEAJQAAAB4AAQABAAAABrIAC0sqsAAAAAEAJgAAAAYAAQAAAAsACAAyAAYAAQAlAAAAHgABAAAAAAAGEgezACCxAAAAAQAmAAAABgABAAAAAgABADMAAAACADQ="

var metodosClase = []string{"deCadena", "deObjeto", "deArreglo", "dePrimitivo", "deVoid",
	"deArregloPrimitivo", "esCadena", "nombre", "local", "<clinit>"}

func dosClasesClase(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Clase.java", contenido: fuenteClase}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(claseJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestLaClaseSeEscribeComoJavac: el `Code` de los diez cuerpos, con `max_stack`, los huecos de local y
// las filas de línea. El literal de objeto es `ldc` de CONSTANT_Class; el de un primitivo no es `ldc`
// sino el `getstatic` del `TYPE` de su envoltorio (parrafo 89).
func TestLaClaseSeEscribeComoJavac(t *testing.T) {
	nuestra, suya := dosClasesClase(t)
	for _, nombre := range metodosClase {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no está en la de javac", nombre)
		}
		if a.MaxStack != b.MaxStack || a.LocalCount != b.LocalCount {
			t.Errorf("%s: pila %d y locales %d, y los de javac %d y %d",
				nombre, a.MaxStack, a.LocalCount, b.MaxStack, b.LocalCount)
		}
		// El índice del pool no es parte de la forma: javac interna `String` como clase al leer el
		// campo `guardada`, y este emisor al emitir el `<clinit>`, así que apuntan a entradas
		// distintas con el mismo contenido. El oráculo de bytes de verdad es el dex, que ordena sus
		// tablas por contenido y sale idéntico (TestLaClaseSeDexaIgualQueLosDeJavac).
		sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }
		if x, y := sinPool(formaDeMetodo(t, nuestra, a)), sinPool(formaDeMetodo(t, suya, b)); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s: las filas nuestras son %q y las de javac %q", nombre, x, y)
		}
	}
}

// TestLaClaseSeDexaIgualQueLosDeJavac: el oráculo doble. El `ldc` de CONSTANT_Class baja a
// `const-class` 0x1c (formato 21c) y el `getstatic` del primitivo a `sget-object` 0x62; el dex de la
// sonda sale byte a byte el mismo sobre nuestras clases que sobre las de javac (parrafo 89).
func TestLaClaseSeDexaIgualQueLosDeJavac(t *testing.T) {
	nuestra, suya := dosClasesClase(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 89: %d bytes", len(deUno))
	if !bytes.Equal(deUno, deOtro) {
		difiere := -1
		for i := range deUno {
			if i < len(deOtro) && deUno[i] != deOtro[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el dex nuestro son %d bytes y el de javac %d; el primer byte distinto está en el offset %d (%#02x contra %#02x)",
			len(deUno), len(deOtro), difiere, deUno[difiere], deOtro[difiere])
	}
}

// TestLasFormasDexDeLaClase: `const-class` 0x1c en los literales de objeto y `sget-object` 0x62 en los
// de primitivo, dentro del cuerpo que escribe este fork.
func TestLasFormasDexDeLaClase(t *testing.T) {
	nuestra, _ := dosClasesClase(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{nuestra}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{"deCadena": 0x1c, "deArreglo": 0x1c, "dePrimitivo": 0x62, "deVoid": 0x62}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no está en nuestra clase", nombre)
		}
		ci, err := td.codeItemDe(nuestra, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := 0
		for _, u := range ci.insns {
			if byte(u) == opcode {
				visto++
			}
		}
		if visto == 0 {
			t.Errorf("%s: no aparece el opcode %#02x en %v", nombre, opcode, ci.insns)
		}
	}
}
