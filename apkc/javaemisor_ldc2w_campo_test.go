package main

// El literal `long` y la constante entera que llena un hueco `long`, medidos con javac 17.0.21
// `--release 8 -g:lines,source` (la versión que fija la CI, §152) y el dex byte a byte.
//
// La regla medida, sonda `P` de este fichero:
//
//	`0L` y `0`      → lconst_0 (0x09)
//	`1L` y `1`      → lconst_1 (0x0a)
//	cualquier otro  → ldc2_w (0x14) con la entrada Long de ocho bytes: también `5L`, `127L`,
//	                  `-1L` y `32767L`, que en `int` cabrían en `iconst`/`bipush`/`sipush`
//
// El mismo patrón vale para un `static final int` doblado en un hueco `long`: `long x = INT500` es
// `ldc2_w 500l`, nunca `sipush 500; i2l`. Es lo que hace javac con `AutoResearchDelay` en
// `MainActivity$3` de ApkShare (`MainActivity.java:105`, `handler.postDelayed(runnable,
// AutoResearchDelay)`), y lo que este emisor no hacía: doblaba el campo a `int` y dejaba la conversión
// detrás. La diferencia se veía con `javap -c` sobre las dos clases y costaba 4 B de dex: la brecha
// total con el oráculo queda en 12 B (16516 frente a 16504), no en los 8 B estimados al medirla.

import (
	"bytes"
	"strings"
	"testing"
)

// fuenteTablaLdc2w es la sonda de la regla: los trece valores largos de la tabla, los mismos doce como
// literales `int` en un hueco `long`, y cinco constantes `static final int` con su cuenta. La
// comparación es el dex entero contra el de javac 17.
const fuenteTablaLdc2w = `package p;
public class Tabla {
    static final int INT0 = 0;
    static final int INT1 = 1;
    static final int INT500 = 500;
    static final int INT32768 = 32768;
    static final int INTNEG = -32769;

    static long l0() { return 0L; }
    static long l1() { return 1L; }
    static long l5() { return 5L; }
    static long l127() { return 127L; }
    static long l128() { return 128L; }
    static long l500() { return 500L; }
    static long l32767() { return 32767L; }
    static long l32768() { return 32768L; }
    static long lm1() { return -1L; }
    static long lm32768() { return -32768L; }
    static long lm32769() { return -32769L; }
    static long lmax() { return 2147483647L; }
    static long lgrande() { return 5000000000L; }

    static long i0() { return 0; }
    static long i1() { return 1; }
    static long i5() { return 5; }
    static long i127() { return 127; }
    static long i128() { return 128; }
    static long i500() { return 500; }
    static long i32767() { return 32767; }
    static long i32768() { return 32768; }
    static long im1() { return -1; }
    static long im32768() { return -32768; }
    static long im32769() { return -32769; }
    static long imax() { return 2147483647; }

    static long c0() { return INT0; }
    static long c1() { return INT1; }
    static long c500() { return INT500; }
    static long c32768() { return INT32768; }
    static long cneg() { return INTNEG; }
    static long csuma() { return INT500 + INT0; }
}
`

// TestLaTablaDelLargoEsLaDeJavac17: el dex entero de la sonda byte a byte y, además, la forma de cada
// instrucción leída en nuestro `.class`, porque el dex no distingue `ldc2_w` de una constante cargada
// por otra vía de tres bytes. `lconst_0`/`lconst_1` solo para 0 y 1; `ldc2_w` para el resto; y ningún
// `i2l` detrás de un `sipush` (la forma que se corrige).
func TestLaTablaDelLargoEsLaDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Tabla.java", contenido: fuenteTablaLdc2w}})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	clase := claseEmitidaPorNombre(t, nuestras, "p/Tabla")
	for _, metodos := range [][2]string{
		{"l0", "lconst_0"}, {"l1", "lconst_1"}, {"l5", "ldc2_w"},
		{"l127", "ldc2_w"}, {"l128", "ldc2_w"}, {"l500", "ldc2_w"},
		{"l32767", "ldc2_w"}, {"l32768", "ldc2_w"}, {"lm1", "ldc2_w"},
		{"lm32768", "ldc2_w"}, {"lm32769", "ldc2_w"}, {"lmax", "ldc2_w"},
		{"lgrande", "ldc2_w"},
		{"i0", "lconst_0"}, {"i1", "lconst_1"}, {"i5", "ldc2_w"},
		{"i127", "ldc2_w"}, {"i128", "ldc2_w"}, {"i500", "ldc2_w"},
		{"i32767", "ldc2_w"}, {"i32768", "ldc2_w"}, {"im1", "ldc2_w"},
		{"im32768", "ldc2_w"}, {"im32769", "ldc2_w"}, {"imax", "ldc2_w"},
		{"c0", "lconst_0"}, {"c1", "lconst_1"}, {"c500", "ldc2_w"},
		{"c32768", "ldc2_w"}, {"cneg", "ldc2_w"}, {"csuma", "ldc2_w"},
	} {
		codigo := codigoDeMetodoEmitido(t, clase, metodos[0], "()J")
		instrucciones, err := walkJvmCode(codigo.Code)
		if err != nil {
			t.Fatalf("%s no se desarma: %v", metodos[0], err)
		}
		carga, siguiente := instrucciones[0], instrucciones[1]
		if carga.name != metodos[1] {
			t.Errorf("%s carga con %s y javac con %s: %v", metodos[0], carga.name, metodos[1], instrucciones)
		}
		if siguiente.name != "lreturn" {
			t.Errorf("%s: detrás de la carga va %s y javac escribe lreturn: %v", metodos[0], siguiente.name, instrucciones)
		}
		for _, in := range instrucciones {
			if in.name == "i2l" {
				t.Errorf("%s escribe i2l y javac dobla la constante en el hueco ancho: %v", metodos[0], instrucciones)
			}
		}
	}
}

// fuenteRetardoConstante es el control mínimo del defecto: el `static final int` de ApkShare en la
// llamada que pide un `long`, y al lado el literal `500` que ya funcionaba. `handler.postDelayed` es
// `(Ljava/lang/Runnable;J)Z` en el `android.jar` del SDK.
const fuenteRetardoConstante = `package p;
import android.os.Handler;
public class Retardo {
    private static final int AutoResearchDelay = 500;
    void porCampo(Handler h, Runnable r) { h.postDelayed(r, AutoResearchDelay); }
    void porLiteral(Handler h, Runnable r) { h.postDelayed(r, 500); }
}
`

// TestElRetardoConstanteLlevaLdc2w: la clase entera dexa como la de javac 17 y la llamada del campo
// lleva su `ldc2_w 500l` delante del `invokevirtual`, sin `sipush; i2l` y sin dejar el `pop` del
// `boolean` de vuelta. Es el `MainActivity$3` de ApkShare reducido a lo que se mide.
func TestElRetardoConstanteLlevaLdc2w(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Retardo.java", contenido: fuenteRetardoConstante}})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	clase := claseEmitidaPorNombre(t, nuestras, "p/Retardo")
	for _, nombre := range []string{"porCampo", "porLiteral"} {
		instrucciones, err := walkJvmCode(codigoDeMetodoEmitido(t, clase, nombre, "(Landroid/os/Handler;Ljava/lang/Runnable;)V").Code)
		if err != nil {
			t.Fatalf("%s no se desarma: %v", nombre, err)
		}
		visto := false
		for i, in := range instrucciones {
			if in.name == "i2l" {
				t.Fatalf("%s escribe i2l donde javac dobla la constante: %v", nombre, instrucciones)
			}
			if in.name != "invokevirtual" {
				continue
			}
			if i == 0 || instrucciones[i-1].name != "ldc2_w" {
				t.Fatalf("%s: el postDelayed de la pc %d no lleva ldc2_w delante: %v", nombre, in.pc, instrucciones)
			}
			visto = true
		}
		if !visto {
			t.Fatalf("%s no llama a postDelayed: %v", nombre, instrucciones)
		}
	}
}

// TestFalsificacionDelPlegadoDelCampoSuelto: con el interruptor apagado vuelve `sipush 500; i2l` y el
// dex deja de ser el de javac; con él encendido, la divergencia desaparece. Sin esta comprobación el
// arreglo podría estar tapado por otra ruta que también doblara el campo.
func TestFalsificacionDelPlegadoDelCampoSuelto(t *testing.T) {
	_, conPlegado, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Retardo.java", contenido: fuenteRetardoConstante}})
	if !bytes.Equal(conPlegado, suyo) {
		t.Fatalf("con el plegado encendido el dex ya no es el de javac: %d frente a %d bytes", len(conPlegado), len(suyo))
	}

	anterior := pliegaCampoSuelto
	pliegaCampoSuelto = false
	t.Cleanup(func() { pliegaCampoSuelto = anterior })

	sinPlegado, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Retardo.java", contenido: fuenteRetardoConstante}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("con el plegado apagado el emisor rechazó la sonda: %v", err)
	}
	nuestraSin, err := dexDeClases([]classFull{leidaDeEmitida(t, claseEmitidaPorNombre(t, sinPlegado, "p/Retardo"))})
	if err != nil {
		t.Fatalf("dexando la sonda sin plegado: %v", err)
	}
	if bytes.Equal(nuestraSin, conPlegado) {
		t.Fatalf("con el plegado apagado el dex no cambia: la falsificación no falsifica")
	}
	instrucciones, err := walkJvmCode(codigoDeMetodoEmitido(t, claseEmitidaPorNombre(t, sinPlegado, "p/Retardo"), "porCampo", "(Landroid/os/Handler;Ljava/lang/Runnable;)V").Code)
	if err != nil {
		t.Fatalf("la sonda sin plegado no se desarma: %v", err)
	}
	var nombres []string
	for _, in := range instrucciones {
		nombres = append(nombres, in.name)
	}
	if !strings.Contains(strings.Join(nombres, " "), "sipush i2l") {
		t.Fatalf("con el plegado apagado no aparece `sipush i2l`: %v", instrucciones)
	}
	// y el control de arriba: el dex de las dos versiones medidas difiere, que es la divergencia que
	// el arreglo elimina
	if len(nuestraSin) == len(suyo) && bytes.Equal(nuestraSin, suyo) {
		t.Fatalf("la sonda sin plegado dexa igual que javac: no hay divergencia que medir")
	}
}

// leidaDeEmitida: la `.class` emitida vuelta a leer para dexarla sola.
func leidaDeEmitida(t *testing.T, datos []byte) classFull {
	t.Helper()
	c, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("la clase emitida no se lee: %v", err)
	}
	return c
}

// codigoDeMetodoEmitido: el `Code` del método de una `.class` emitida, para desarmarlo.
func codigoDeMetodoEmitido(t *testing.T, datos []byte, nombre, desc string) classMethod {
	t.Helper()
	c := leidaDeEmitida(t, datos)
	for _, m := range c.Methods {
		if m.Name == nombre && m.Desc == desc {
			return m
		}
	}
	t.Fatalf("no está %s%s en la clase emitida", nombre, desc)
	return classMethod{}
}
