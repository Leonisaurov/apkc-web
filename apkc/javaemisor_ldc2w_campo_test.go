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

import "testing"

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
