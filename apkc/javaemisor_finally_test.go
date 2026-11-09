package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// fuenteFinalmente es la sonda del parrafo 75: un `try/finally` con y sin locales, dos `catch` en el
// mismo `try`, los dos `catch` sin `finally`, un `finally` que lanza y un `finally` vacio.
const fuenteFinalmente = `public class Fini {
    static int a = 0;

    static int simple() {
        try {
            a = 1;
        } finally {
            a = 2;
        }
        return a;
    }

    static int local(int n) {
        int r = 0;
        try {
            r = n + 1;
        } finally {
            r = r + 10;
        }
        return r;
    }

    static int multi(int n) {
        try {
            a = 100 / n;
        } catch (ArithmeticException e) {
            a = 5;
        } catch (RuntimeException e) {
            a = 6;
        } finally {
            a = 7;
        }
        return a;
    }

    static int dosCatch(int n) {
        int r = 0;
        try {
            r = 100 / n;
        } catch (ArithmeticException e) {
            r = 5;
        } catch (RuntimeException e) {
            r = 6;
        }
        return r;
    }

    static void lanzaEnFinally() {
        try {
            a = 1;
        } finally {
            throw new ArithmeticException();
        }
    }

    static int vacioFinalmente(int n) {
        try {
            n = n + 1;
        } finally {
        }
        return n;
    }

    static int anidado(int n) {
        int r = 0;
        try {
            try {
                r = 100 / n;
            } catch (ArithmeticException e) {
                r = 2;
            }
        } finally {
            r = r + 1;
        }
        return r;
    }

    static int retorno() {
        try {
            return 1;
        } finally {
            a = 2;
        }
    }

    static int retornoConCatch(int n) {
        try {
            return 100 / n;
        } catch (ArithmeticException e) {
            return 2;
        } finally {
            a = 3;
        }
    }

    static int rama(int n) {
        try {
            if (n > 0) {
                return 1;
            }
            n = n + 1;
        } finally {
            a = 2;
        }
        return n;
    }

    static void ramaVoid(int n) {
        try {
            if (n > 0) {
                return;
            }
            a = 1;
        } finally {
            a = 6;
        }
    }

    static int breakConFinally(int n) {
        int r = 0;
        while (n > 0) {
            try {
                if (n > 2) {
                    break;
                }
                r = r + 1;
            } finally {
                a = 3;
            }
            n = n - 1;
        }
        return r;
    }
}
`

// finalmenteJavac: `javac --release 8 -g:lines,source Fini.java` y `base64 -w0 Fini.class`.
const finalmenteJavac = "yv66vgAAADQAKQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABEZpbmkBAAFhAQABSQcADgEAHWphdmEvbGFuZy9Bcml0aG1ldGljRXhjZXB0aW9uBwAQAQAaamF2YS9sYW5nL1J1bnRpbWVFeGNlcHRpb24KAA0AAwEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAZzaW1wbGUBAAMoKUkBAA1TdGFja01hcFRhYmxlBwAYAQATamF2YS9sYW5nL1Rocm93YWJsZQEABWxvY2FsAQAEKEkpSQEABW11bHRpAQAIZG9zQ2F0Y2gBAA5sYW56YUVuRmluYWxseQEAD3ZhY2lvRmluYWxtZW50ZQEAB2FuaWRhZG8BAAdyZXRvcm5vAQAPcmV0b3Jub0NvbkNhdGNoAQAEcmFtYQEACHJhbWFWb2lkAQAEKEkpVgEAD2JyZWFrQ29uRmluYWxseQEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEACUZpbmkuamF2YQAhAAgAAgAAAAEACAALAAwAAAAOAAEABQAGAAEAEgAAAB0AAQABAAAABSq3AAGxAAAAAQATAAAABgABAAAAAQAIABQAFQABABIAAABXAAEAAQAAABYEswAHBbMAB6cACksFswAHKr+yAAesAAEAAAAEAAsAAAACABMAAAAaAAYAAAAGAAQACAAIAAkACwAIABAACQASAAoAFgAAAAcAAksHABcGAAgAGQAaAAEAEgAAAGUAAgADAAAAGAM8GgRgPBsQCmA8pwALTRsQCmA8LL8brAABAAIABgAOAAAAAgATAAAAHgAHAAAADgACABAABgASAAsAEwAOABIAFAATABYAFAAWAAAADwAC/wAOAAIBAQABBwAXBwAIABsAGgABABIAAAC/AAIAAwAAADYQZBpsswAHEAezAAenACZMCLMABxAHswAHpwAZTBAGswAHEAezAAenAAtNEAezAAcsv7IAB6wABQAAAAcADwANAAAABwAcAA8AAAAHACoAAAAPABQAKgAAABwAIgAqAAAAAgATAAAAOgAOAAAAGQAHAB8ADAAgAA8AGgAQABsAFAAfABkAIAAcABwAHQAdACIAHwAnACAAKgAfADAAIAAyACEAFgAAAA8ABE8HAA1MBwAPTQcAFwcACAAcABoAAQASAAAAdwACAAMAAAAWAzwQZBpsPKcADU0IPKcAB00QBjwbrAACAAIABwAKAA0AAgAHABAADwACABMAAAAmAAkAAAAlAAIAJwAHACwACgAoAAsAKQANACwAEAAqABEAKwAUAC0AFgAAABMAA/8ACgACAQEAAQcADUUHAA8DAAgAHQAGAAEAEgAAAEUAAgABAAAAFQSzAAe7AA1ZtwARv0u7AA1ZtwARvwABAAAABAAMAAAAAgATAAAACgACAAAAMgAEADQAFgAAAAYAAUwHABcACAAeABoAAQASAAAAIgACAAEAAAAGGgRgOxqsAAAAAQATAAAACgACAAAAOgAEAD0ACAAfABoAAQASAAAAgwACAAQAAAAdAzwQZBpsPKcABk0FPBsEYDynAApOGwRgPC2/G6wAAgACAAcACgANAAIADQAUAAAAAgATAAAAKgAKAAAAQQACAEQABwBHAAoARQALAEYADQBJABEASgAUAEkAGQBKABsASwAWAAAAFAAE/wAKAAIBAQABBwANAkYHABcGAAgAIAAVAAEAEgAAAEsAAQACAAAADwQ7BbMABxqsTAWzAAcrvwABAAAAAgAIAAAAAgATAAAAFgAFAAAAUAACAFIABgBQAAgAUgANAFMAFgAAAAYAAUgHABcACAAhABoAAQASAAAAewACAAQAAAAbEGQabDwGswAHG6xMBT0GswAHHKxOBrMABy2/AAMAAAAFAAsADQAAAAUAFAAAAAsADgAUAAAAAgATAAAAJgAJAAAAWAAFAFwACQBYAAsAWQAMAFoADgBcABIAWgAUAFwAGQBdABYAAAAKAAJLBwANSAcAFwAIACIAGgABABIAAAB6AAIAAwAAACAangALBDwFswAHG6waBGA7BbMAB6cACk0FswAHLL8arAACAAAABgAXAAAADAAQABcAAAACABMAAAAqAAoAAABiAAQAYwAGAGcACgBjAAwAZQAQAGcAFABoABcAZwAcAGgAHgBpABYAAAAIAAMMSgcAFwYACAAjACQAAQASAAAAdQABAAIAAAAfGp4ACRAGswAHsQSzAAcQBrMAB6cAC0wQBrMAByu/sQACAAAABAAWAAAACgAOABYAAAACABMAAAAmAAkAAABuAAQAcwAJAG8ACgBxAA4AcwATAHQAFgBzABwAdAAeAHUAFgAAAAgAAwpLBwAXBwAIACUAGgABABIAAACUAAIAAwAAAC0DPBqeACgaBaQACgazAAenABwbBGA8BrMAB6cACk0GswAHLL8aBGQ7p//aG6wAAgAGAAsAHQAAABIAFgAdAAAAAgATAAAAMgAMAAAAeAACAHkABgB7AAsAgAAPAHwAEgB+ABYAgAAaAIEAHQCAACIAgQAkAIIAKwCEABYAAAANAAX8AAIBD0oHABcGBgAIACYABgABABIAAAAdAAEAAAAAAAUDswAHsQAAAAEAEwAAAAYAAQAAAAIAAQAnAAAAAgAo"

// tablaDeExcepciones: las filas de la exception_table como texto, con el `any` nombrado.
func tablaDeExcepciones(m classMethod) string {
	fila := []string{}
	for _, tr := range m.Tries {
		tipo := tr.catchType
		if tipo == "" {
			tipo = "any"
		}
		fila = append(fila, itoa(tr.start)+".."+itoa(tr.end)+" -> "+itoa(tr.handler)+" "+tipo)
	}
	return strings.Join(fila, " | ")
}

// buscaMetodo: el metodo de un nombre, sin importar el descriptor.
func buscaMetodo(cf classFull, nombre string) (classMethod, bool) {
	for _, m := range cf.Methods {
		if m.Name == nombre {
			return m, true
		}
	}
	return classMethod{}, false
}

// metodosFinalmente: los métodos de la sonda, en el orden del fuente.
var metodosFinalmente = []string{"simple", "local", "multi", "dosCatch", "lanzaEnFinally", "vacioFinalmente", "anidado",
	"retorno", "retornoConCatch", "rama", "ramaVoid", "breakConFinally"}

// filasDeLinea: la LineNumberTable como texto, en el orden en que se escribe.
func filasDeLinea(m classMethod) string {
	fila := []string{}
	for _, l := range m.Lines {
		fila = append(fila, itoa(l.line)+" en "+itoa(l.pc))
	}
	return strings.Join(fila, ", ")
}

// TestElFinalmenteSeEscribeComoJavac: cada metodo de la sonda sale con las mismas instrucciones y la
// misma exception_table que escribe javac, incluidos `max_stack` y los huecos de local —que es donde
// se ve la reutilizacion del mismo numero para la excepcion de cada `catch` y el hueco mas alto para
// la del `finally` (parrafo 75).
func TestElFinalmenteSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Fini.java", contenido: fuenteFinalmente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(finalmenteJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	for _, nombre := range metodosFinalmente {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s: no esta en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s: no esta en la de javac", nombre)
		}
		if a.MaxStack != b.MaxStack || a.LocalCount != b.LocalCount {
			t.Errorf("%s: pila %d y locales %d (con los argumentos), y los de javac %d y %d",
				nombre, a.MaxStack, a.LocalCount, b.MaxStack, b.LocalCount)
		}
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if x, y := tablaDeExcepciones(a), tablaDeExcepciones(b); x != y {
			t.Errorf("%s: la tabla nuestra es %q y la de javac %q", nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s: las filas nuestras son %q y las de javac %q", nombre, x, y)
		}
	}
}

// TestElFinalmenteSeDexaComoJavac: los cuerpos del `try/finally` y de los dos `catch` salen palabra por
// palabra los mismos, con sus `try_item` y la lista de handlers (el `catch_all` con el tamano negativo).
func TestElFinalmenteSeDexaComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Fini.java", contenido: fuenteFinalmente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(finalmenteJavac)
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
		difiere := -1
		for i := range deUno {
			if deUno[i] != deOtro[i] {
				difiere = i
				break
			}
		}
		t.Errorf("el dex nuestro son %d bytes y el de javac %d; el primer byte distinto esta en el offset %d (%#02x contra %#02x)",
			len(deUno), len(deOtro), difiere, deUno[difiere], deOtro[difiere])
		uno, err := readDex(deUno)
		if err != nil {
			t.Fatalf("nuestro dex no se relee: %v", err)
		}
		otro, err := readDex(deOtro)
		if err != nil {
			t.Fatalf("el dex de javac no se relee: %v", err)
		}
		for _, nombre := range metodosFinalmente {
			a, err := codeItemDeMetodo(uno, nombre)
			if err != nil {
				t.Errorf("%s: %v", nombre, err)
				continue
			}
			b, err := codeItemDeMetodo(otro, nombre)
			if err != nil {
				t.Errorf("%s: %v", nombre, err)
				continue
			}
			if a.registersSize != b.registersSize || a.insSize != b.insSize || a.outsSize != b.outsSize ||
				a.triesSize != b.triesSize || len(a.insns) != len(b.insns) {
				t.Errorf("%s: cabecera y tries %d/%d/%d/%d con %d palabras, y los de javac %d/%d/%d/%d con %d",
					nombre, a.registersSize, a.insSize, a.outsSize, a.triesSize, len(a.insns),
					b.registersSize, b.insSize, b.outsSize, b.triesSize, len(b.insns))
				continue
			}
			for i := range a.insns {
				if a.insns[i] != b.insns[i] {
					t.Errorf("%s: la palabra %d es %#04x y la de javac %#04x", nombre, i, a.insns[i], b.insns[i])
					break
				}
			}
		}
	}
}

// TestElFinalmenteRechazaLoNoMedido: las formas de javac que este emisor todavia no escribe salen por
// su nombre, no por un mensaje generico.
func TestElFinalmenteRechazaLoNoMedido(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"return dentro del finally", "package p;\npublic class A { public static int f(int n) { try { f(n); } finally { if (n > 0) { return 1; } } return 2; } }\n", "dentro del propio `finally`"},
		{"throw ultimo del cuerpo", "package p;\npublic class A { public static void f() { try { throw new RuntimeException(); } finally { f(); } } }\n", "ultima sentencia"},
		{"throw ultimo del catch", "package p;\npublic class A { public static void f(int n) { try { f(n); } catch (RuntimeException e) { throw new RuntimeException(); } finally { f(n); } } }\n", "ultima sentencia de un `catch`"},
		{"cuerpo que sale por dos trozos con catch", "package p;\npublic class A { public static int f(int n) { try { if (n > 0) { return 1; } f(n); } catch (RuntimeException e) { f(n); } finally { f(n); } return 2; } }\n", "dos trozos"},
		{"cuerpo sin nada protegido", "package p;\npublic class A { public static void f(int n) { try { return; } finally { f(n); } } }\n", "nada protegido"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto lo que no esta medido", c.nombre)
			continue
		}
		if !bytes.Contains([]byte(err.Error()), []byte(c.pista)) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
