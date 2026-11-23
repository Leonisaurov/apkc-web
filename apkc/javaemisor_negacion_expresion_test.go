package main

// La negación unaria de una expresión, del fuente al dex. La sonda cubre lo que antes se rechazaba
// («la negacion de algo que no es un literal entero no esta medida») y lo que ya estaba medido:
//
//   - `-total` (una variable), `-(a + b)` (una cuenta), `-(x)` (un paréntesis) y `-f()` (una llamada)
//     sobre `int`, que javac baja a `iload; ineg` / `invokestatic; ineg` con la negación **detrás**
//     de su operando;
//   - `-x` sobre `long`, `float` y `double`, que son `lneg` 0x75, `fneg` 0x76 y `dneg` 0x77;
//   - `-c` sobre un `char`, que se niega como entero (`iload_0; ineg`) y devuelve `int`;
//   - `-x` sobre un `int` devuelto como `long`, que deja la negación en el sitio de javac con el
//     `i2l` detrás;
//   - `-(1 + 2)`, `-(2L + 3L)` y `-(1.0 + 2.0)`, que javac dobla **en el fuente** (`bipush -3`,
//     `ldc2_w -5l`, `ldc2_w -3.0d`): los literales ya medidos no cambian de comportamiento.
//
// El oráculo es `javac 17 --release 8 -g:lines,source` para el `.class` y, para el dex, el dexer de
// este fork sobre las dos clases: byte a byte. `d8`/`dexdump` confirman las formas dalvik
// (`neg-int` 0x7b, `neg-long` 0x7d, `neg-float` 0x7f y `neg-double` 0x80, todas 12x en el sitio).
// La ejecución en ART es del banco, que corre el mismo programa por los cinco caminos.

import (
	"bytes"
	"encoding/base64"
	"testing"
)

const fuenteNegacionExpresion = `public class NegE {
    static int total(int[] v) {
        int t = 0;
        for (int i = 0; i < v.length; i++) { t += v[i]; }
        return -t;
    }
    static int suma(int a, int b) { return -(a + b); }
    static int variable(int x) { return -x; }
    static long la(long x) { return -x; }
    static float fa(float x) { return -x; }
    static double da(double x) { return -x; }
    static int conLlamada(int x) { return -f(x); }
    static int f(int x) { return x + 1; }
    static int conParentesis(int x) { return -(x); }
    static int anidada(int x, int y) { return -(x + -y); }
    static int desdeChar(char c) { return -c; }
    static long desdeInt(int x) { return -x; }
    static int desdeCharLiteral() { return -'a'; }
    static int constante() { return -(1 + 2); }
    static long constanteLarga() { return -(2L + 3L); }
    static double constanteDoble() { return -(1.0 + 2.0); }
}
`

// negacionExpresionJavac: `javac 17 --release 8 -g:lines,source NegE.java` y `base64 -w0 NegE.class`.
const negacionExpresionJavac = "yv66vgAAADQALwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEABE5lZ0UBAAFmAQAEKEkpSQX/////////+wbACAAAAAAAAAEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAV0b3RhbAEABShbSSlJAQANU3RhY2tNYXBUYWJsZQEABHN1bWEBAAUoSUkpSQEACHZhcmlhYmxlAQACbGEBAAQoSilKAQACZmEBAAQoRilGAQACZGEBAAQoRClEAQAKY29uTGxhbWFkYQEADWNvblBhcmVudGVzaXMBAAdhbmlkYWRhAQAJZGVzZGVDaGFyAQAEKEMpSQEACGRlc2RlSW50AQAEKEkpSgEAEGRlc2RlQ2hhckxpdGVyYWwBAAMoKUkBAAljb25zdGFudGUBAA5jb25zdGFudGVMYXJnYQEAAygpSgEADmNvbnN0YW50ZURvYmxlAQADKClEAQAKU291cmNlRmlsZQEACU5lZ0UuamF2YQAhAAgAAgAAAAAAEQABAAUABgABABEAAAAdAAEAAQAAAAUqtwABsQAAAAEAEgAAAAYAAQAAAAEACAATABQAAQARAAAASQADAAMAAAAZAzwDPRwqvqIADxsqHC5gPIQCAaf/8Rt0rAAAAAIAEgAAAA4AAwAAAAMAAgAEABYABQAVAAAACgAC/QAEAQH6ABEACAAWABcAAQARAAAAHQACAAIAAAAFGhtgdKwAAAABABIAAAAGAAEAAAAHAAgAGAAMAAEAEQAAABsAAQABAAAAAxp0rAAAAAEAEgAAAAYAAQAAAAgACAAZABoAAQARAAAAGwACAAIAAAADHnWtAAAAAQASAAAABgABAAAACQAIABsAHAABABEAAAAbAAEAAQAAAAMidq4AAAABABIAAAAGAAEAAAAKAAgAHQAeAAEAEQAAABsAAgACAAAAAyZ3rwAAAAEAEgAAAAYAAQAAAAsACAAfAAwAAQARAAAAHgABAAEAAAAGGrgAB3SsAAAAAQASAAAABgABAAAADAAIAAsADAABABEAAAAcAAIAAQAAAAQaBGCsAAAAAQASAAAABgABAAAADQAIACAADAABABEAAAAbAAEAAQAAAAMadKwAAAABABIAAAAGAAEAAAAOAAgAIQAXAAEAEQAAAB4AAgACAAAABhobdGB0rAAAAAEAEgAAAAYAAQAAAA8ACAAiACMAAQARAAAAGwABAAEAAAADGnSsAAAAAQASAAAABgABAAAAEAAIACQAJQABABEAAAAcAAIAAQAAAAQadIWtAAAAAQASAAAABgABAAAAEQAIACYAJwABABEAAAAbAAEAAAAAAAMQn6wAAAABABIAAAAGAAEAAAASAAgAKAAnAAEAEQAAABsAAQAAAAAAAxD9rAAAAAEAEgAAAAYAAQAAABMACAApACoAAQARAAAAHAACAAAAAAAEFAANrQAAAAEAEgAAAAYAAQAAABQACAArACwAAQARAAAAHAACAAAAAAAEFAAPrwAAAAEAEgAAAAYAAQAAABUAAQAtAAAAAgAu"

// metodosNegacionExpresion: los quince de la sonda, en el orden del fuente.
var metodosNegacionExpresion = []string{"total", "suma", "variable", "la", "fa", "da",
	"conLlamada", "f", "conParentesis", "anidada", "desdeChar", "desdeInt", "desdeCharLiteral",
	"constante", "constanteLarga", "constanteDoble"}

func dosClasesNegacionExpresion(t *testing.T) (classFull, classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/NegE.java", contenido: fuenteNegacionExpresion}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(negacionExpresionJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return nuestra, suya
}

// TestLaNegacionDeUnaExpresionSeEscribeComoJavac: instruccion a instruccion, con `max_stack`, los
// huecos de local y las filas de linea. Es la prueba que falla con el mensaje del rechazo anterior si
// se quita la ruta nueva del analizador: `-total`, `-(a + b)` y `-f()` no llegaban a emitirse.
func TestLaNegacionDeUnaExpresionSeEscribeComoJavac(t *testing.T) {
	nuestra, suya := dosClasesNegacionExpresion(t)
	for _, nombre := range metodosNegacionExpresion {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no esta en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no esta en la de javac", nombre)
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

// TestLaNegacionDeUnaExpresionSeDexaComoJavac: el dex que escribe este fork sobre nuestras clases es
// byte a byte el de las de javac, con las cuatro negaciones en la forma 12x medida con `d8` y
// `dexdump` (`neg-int` 0x7b, `neg-long` 0x7d, `neg-float` 0x7f y `neg-double` 0x80).
func TestLaNegacionDeUnaExpresionSeDexaComoJavac(t *testing.T) {
	nuestra, suya := dosClasesNegacionExpresion(t)
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		difiere := -1
		for i := range deUno {
			if i < len(deOtro) && deUno[i] != deOtro[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el dex nuestro son %d bytes y el de javac %d; el primer byte distinto esta en el offset %d (%#02x contra %#02x)",
			len(deUno), len(deOtro), difiere, deUno[difiere], deOtro[difiere])
	}
}

// TestLaNegacionDeUnaExpresionDejaSuOpcode: el opcode de la negación aparece en cada método con el 12x
// del dexer, sin depender de la comparación global: así un cambio en otro sitio de la clase no
// esconde que `-x` se compiló como otra cosa. Los anchos salen de `d8`/`dexdump` sobre esta sonda.
func TestLaNegacionDeUnaExpresionDejaSuOpcode(t *testing.T) {
	nuestra, _ := dosClasesNegacionExpresion(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{nuestra}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{
		"total": 0x7b, "suma": 0x7b, "variable": 0x7b, "la": 0x7d,
		"fa": 0x7f, "da": 0x80, "conLlamada": 0x7b, "conParentesis": 0x7b,
		"anidada": 0x7b, "desdeChar": 0x7b, "desdeInt": 0x7b,
	}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no esta en nuestra clase", nombre)
		}
		ci, err := td.codeItemDe(nuestra, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := false
		for _, u := range ci.insns {
			if byte(u) != opcode {
				continue
			}
			if destino, fuente := (u>>8)&0xf, (u>>12)&0xf; destino != fuente {
				t.Errorf("%s: el %#02x no es en el sitio (destino v%d, fuente v%d) en %04x",
					nombre, opcode, destino, fuente, ci.insns)
			}
			visto = true
			break
		}
		if !visto {
			t.Errorf("%s: no aparece el opcode %#02x en %04x", nombre, opcode, ci.insns)
		}
	}
	// las constantes dobladas NO llevan negación: javac las plegó en el fuente y el dex solo carga
	// el número (`const/4` o `const-wide`), así que aquí no debe aparecer ningún `neg-*`.
	for _, nombre := range []string{"desdeCharLiteral", "constante", "constanteLarga", "constanteDoble"} {
		m, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no esta en nuestra clase", nombre)
		}
		ci, err := td.codeItemDe(nuestra, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		for _, u := range ci.insns {
			if op := byte(u); op == 0x7b || op == 0x7d || op == 0x7f || op == 0x80 {
				t.Errorf("%s: lleva una negacion (%#02x) y javac doblo la constante en el fuente: %04x",
					nombre, op, ci.insns)
			}
		}
	}
}
