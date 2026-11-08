package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuenteLetras es la sonda del párrafo 87: el `char` del fuente al dex. Lo que fija cada método:
//
//	salto/comilla/barra/tabla  `'\n'`, `'\''`, `'\\'`, `'\t'`  → bipush con el punto de código
//	acento/alto                `'á'` (UTF-8 suelto) y `'ÿ'`    → sipush 225 y sipush 255
//	masUno                     `c + 1` de un `char`             → iload_0; iconst_1; iadd, SIN conversion
//	resta                      `(char) (a - b)`                 → isub; i2c (0x92)
//	compara                    `if (c > '0')`                   → iload_0; bipush 48; if_icmp*
//	lee/escribe                `v[i]` y `v[i] = c`               → caload 0x34 y castore 0x55
//	alta                       `new char[n]`                     → newarray con el atype 5
//	inicial                    `new char[] {'a','b'}`             → la cadena del §73 con castore
//	ponCero                    `v[0] = 65`                        → el literal entero entra sin conversión
//	deEntero / aEntero          `(char) n` y `int n = c`          → i2c para estrechar, NADA para ensanchar
//	recorre                    bucle con `char r = 0`             → la misma forma del §63 con un char
//	cambia                     `c = (char) (c + 1)`               → iadd; i2c; istore_0
//	campo                      `fijo = s; return fijo`            → sput-char 0x6c y sget-char 0x65 en el dex
//	delSwitch                  `switch (c) { case 'a': … }`       → lookupswitch/tableswitch con el punto de código
//
// La elección entre `tableswitch` y `lookupswitch` NO se compara aquí: es la diferencia ya aceptada del
// §64 (el emisor se queda con la tabla cuando el rango no pasa del doble de los casos, y `d8`/`javac`
// eligen tabla en más casos todavía —medido en el párrafo 87: cuatro claves con rango 8 salen tabla en
// javac y búsqueda en este emisor—). Las dos formas son correctas; la sonda usa claves consecutivas,
// donde los dos coinciden.
const fuenteLetras = `public class Letras {
    static char fijo = 'A';
    static final char CONSTE = 'Z';
    static char salto() { return '\n'; }
    static char comilla() { return '\''; }
    static char barra() { return '\\'; }
    static char tabla() { return '\t'; }
    static char acento() { return 'á'; }
    static char alto() { return 'ÿ'; }
    static int masUno(char c) { return c + 1; }
    static char resta(char a, char b) { return (char) (a - b); }
    static int compara(char c) { if (c > '0') { return 1; } return 0; }
    static char lee(char[] v, int i) { return v[i]; }
    static void escribe(char[] v, int i, char c) { v[i] = c; }
    static char[] alta(int n) { return new char[n]; }
    static char[] inicial() { return new char[] {'a', 'b'}; }
    static int largo(char[] v) { return v.length; }
    static char deEntero(int n) { return (char) n; }
    static int aEntero(char c) { return c; }
    static char ponCero(char[] v) { v[0] = 65; return v[0]; }
    static char recorre(char[] v) {
        char r = 0;
        int i = 0;
        while (i < v.length) {
            if (v[i] > r) { r = v[i]; }
            i++;
        }
        return r;
    }
    static char cambia(char c) { c = (char) (c + 1); return c; }
    static char campo(char s) { fijo = s; return fijo; }
    static char delSwitch(char c) { switch (c) { case 'a': return 'x'; case 'b': return 'y'; case 'c': return 'z'; default: return 'q'; } }
}
`

// letrasJavac: `javac --release 8 -g:lines,source Letras.java` y `base64 -w0 Letras.class`.
const letrasJavac = "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"

var metodosLetras = []string{"salto", "comilla", "barra", "tabla", "acento", "alto", "masUno", "resta",
	"compara", "lee", "escribe", "alta", "inicial", "largo", "deEntero", "aEntero", "ponCero", "recorre",
	"cambia", "campo", "delSwitch", "<clinit>"}

func dosClasesLetras(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Letras.java", contenido: fuenteLetras}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(letrasJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestElCharSeEscribeComoJavac: el `Code` de los veintiún métodos, con `max_stack`, los huecos de local
// y las filas de línea. Lo que fija es que el `char` viaja como entero en la pila (`masUno` no escribe
// ninguna conversión), que estrechar cuesta `i2c` 0x92 y que las formas de arreglo son `caload` 0x34 y
// `castore` 0x55.
func TestElCharSeEscribeComoJavac(t *testing.T) {
	nuestra, suya := dosClasesLetras(t)
	for _, nombre := range metodosLetras {
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
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s: las filas nuestras son %q y las de javac %q", nombre, x, y)
		}
	}
}

// TestElCharConstantValueEsEntero: `static final char CONSTE = 'Z'` sale del javac con una entrada
// `ConstantValue` de tipo `Integer` y valor 90, y sin `putstatic` en el `<clinit>` (parrafo 87).
func TestElCharConstantValueEsEntero(t *testing.T) {
	nuestra, suya := dosClasesLetras(t)
	campoDe := func(cf classFull, nombre string) (classField, bool) {
		for _, f := range cf.Fields {
			if f.Name == nombre {
				return f, true
			}
		}
		return classField{}, false
	}
	a, ok := campoDe(nuestra, "CONSTE")
	if !ok {
		t.Fatal("CONSTE no está en nuestra clase")
	}
	b, ok := campoDe(suya, "CONSTE")
	if !ok {
		t.Fatal("CONSTE no está en la de javac")
	}
	if !a.HasConst || !b.HasConst {
		t.Fatalf("nuestra constante vale %v y la de javac %v", a.HasConst, b.HasConst)
	}
	if a.ConstTag != cpInteger || a.ConstInt != b.ConstInt || a.ConstInt != 90 {
		t.Errorf("el ConstantValue nuestro es tag %d valor %d, y el de javac tag %d valor %d",
			a.ConstTag, a.ConstInt, b.ConstTag, b.ConstInt)
	}
}

// TestElCharSeDexaIgualQueLosDeJavac: el oráculo doble. `caload` baja a `aget-char` 0x49, `castore` a
// `aput-char` 0x50, `i2c` a `int-to-char` 0x8e y el campo estático a `sget-char` 0x65 / `sput-char`
// 0x6c, todo medido sobre el fichero que escribe `d8 --debug` (parrafo 87).
func TestElCharSeDexaIgualQueLosDeJavac(t *testing.T) {
	nuestra, suya := dosClasesLetras(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 87: %d bytes, idéntico escrito sobre nuestras clases que sobre las de javac", len(deUno))
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

// TestLasFormasDexDelChar: los opcodes medidos a `d8 --debug` y leidos con el `dexdump -d` del aparato
// (parrafo 87) tienen que aparecer en el cuerpo que escribe este fork: `aget-char` 0x49 en `lee`,
// `aput-char` 0x50 en `escribe`, `int-to-char` 0x8e en `deEntero` y `sget-char` 0x65 en `campo`.
func TestLasFormasDexDelChar(t *testing.T) {
	nuestra, _ := dosClasesLetras(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{nuestra}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{"lee": 0x49, "escribe": 0x50, "deEntero": 0x8e, "campo": 0x65}
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
