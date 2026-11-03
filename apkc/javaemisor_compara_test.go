package main

import (
	"encoding/base64"
	"testing"
)

// ramJavac: `javac --release 8 -g:lines,source Ram.java` y `base64 -w0` de la sonda del párrafo 81,
// con las ocho relaciones que mezclan las tres familias: `>`, `==` y `<` de largos; `>` , `!=` y `<`
// de dobles; `<=` y `>=` de flotantes.
const ramJavac = "yv66vgAAADQAGwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQADUmFtAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEABmxNYXlvcgEABChKKUkBAA1TdGFja01hcFRhYmxlAQAGbElndWFsAQAFKEpKKUkBAAZsTWVub3IBAAZkTWF5b3IBAAUoREQpSQEACWREaXN0aW50bwEABChEKUkBAAtmTWVub3JJZ3VhbAEABShGRilJAQALZk1heW9ySWd1YWwBAAZkTWVub3IBAApTb3VyY2VGaWxlAQAIUmFtLmphdmEAIQAHAAIAAAAAAAkAAQAFAAYAAQAJAAAAHQABAAEAAAAFKrcAAbEAAAABAAoAAAAGAAEAAAABAAkACwAMAAEACQAAACsABAACAAAACh4JlJ4ABQSsBawAAAACAAoAAAAGAAEAAAACAA0AAAADAAEIAAkADgAPAAEACQAAACsABAAEAAAACh4glJoABQSsBawAAAACAAoAAAAGAAEAAAADAA0AAAADAAEIAAkAEAAPAAEACQAAACsABAAEAAAACh4glJwABQSsBawAAAACAAoAAAAGAAEAAAAEAA0AAAADAAEIAAkAEQASAAEACQAAACsABAAEAAAACiYol54ABQSsBawAAAACAAoAAAAGAAEAAAAFAA0AAAADAAEIAAkAEwAUAAEACQAAACsABAACAAAACiYOl5kABQSsBawAAAACAAoAAAAGAAEAAAAGAA0AAAADAAEIAAkAFQAWAAEACQAAACsAAgACAAAACiIjlp0ABQSsBawAAAACAAoAAAAGAAEAAAAHAA0AAAADAAEIAAkAFwAWAAEACQAAACsAAgACAAAACiIjlZsABQSsBawAAAACAAoAAAAGAAEAAAAIAA0AAAADAAEIAAkAGAASAAEACQAAACsABAAEAAAACiYomJwABQSsBawAAAACAAoAAAAGAAEAAAAJAA0AAAADAAEIAAEAGQAAAAIAGg=="

func claseRam(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(ramJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	return cf
}

// TestLasComparacionesAnchasEnElDex: cada relación baja al bloque contiguo 0x2d..0x31 en formato
// 23x —`fcmpl` 0x2d, `fcmpg` 0x2e, `dcmpl` 0x2f, `dcmpg` 0x30, `lcmp` 0x31— y encima la branquia
// `if-*z` que ya estaba medida. Las parejas de la tabla del párrafo 81, una por método:
//
//	`x > 0L` → lcmp + if-lez · `x == y` → lcmp + if-nez · `x < y` → lcmp + if-gez
//	`x > y` → dcmpl + if-lez · `x != 0.0` → dcmpl + if-eqz · `x < y` → dcmpg + if-gez
//	`x <= y` → fcmpg + if-gtz · `x >= y` → fcmpl + if-ltz
func TestLasComparacionesAnchasEnElDex(t *testing.T) {
	casos := []struct {
		metodo   string
		cmp      byte
		branquia byte
	}{
		{"lMayor", 0x31, 0x3d},
		{"lIgual", 0x31, 0x39},
		{"lMenor", 0x31, 0x3b},
		{"dMayor", 0x2f, 0x3d},
		{"dDistinto", 0x2f, 0x38},
		{"dMenor", 0x30, 0x3b},
		{"fMenorIgual", 0x2e, 0x3c},
		{"fMayorIgual", 0x2d, 0x3a},
	}
	cf := claseRam(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	for _, c := range casos {
		m, ok := buscaMetodo(cf, c.metodo)
		if !ok {
			t.Fatalf("%s no está en la sonda", c.metodo)
		}
		ci, err := td.codeItemDe(cf, m)
		if err != nil {
			t.Fatalf("dexando %s: %v", c.metodo, err)
		}
		encontrado := false
		for i := 0; i+2 < len(ci.insns); i++ {
			if byte(ci.insns[i]) != c.cmp {
				continue
			}
			// el `cmp` es 23x: dos unidades, así que la instrucción siguiente empieza en i+2
			if byte(ci.insns[i+2]) != c.branquia {
				t.Errorf("%s: el `cmp` %#02x no lo consume la branquia %#02x, sino %#02x",
					c.metodo, c.cmp, c.branquia, byte(ci.insns[i+2]))
				continue
			}
			encontrado = true
			break
		}
		if !encontrado {
			t.Errorf("%s: no hay `cmp` %#02x seguido de la branquia %#02x", c.metodo, c.cmp, c.branquia)
		}
	}
}

// TestLasComparacionesAnchasSalenDelFuente: el emisor escribe las siete formas del `.class` —el
// literal cero no se ahorra en las anchas— y su dex coincide con el de las clases de javac. Sin el
// jar del SDK el fuente no se compila y el test se salta (como los demás del emisor).
func TestLasComparacionesAnchasSalenDelFuente(t *testing.T) {
	fuente := `public class Ram {
    public static int lMayor(long x) { if (x > 0L) { return 1; } return 2; }
    public static int lIgual(long x, long y) { if (x == y) { return 1; } return 2; }
    public static int lMenor(long x, long y) { if (x < y) { return 1; } return 2; }
    public static int dMayor(double x, double y) { if (x > y) { return 1; } return 2; }
    public static int dDistinto(double x) { if (x != 0.0) { return 1; } return 2; }
    public static int fMenorIgual(float x, float y) { if (x <= y) { return 1; } return 2; }
    public static int fMayorIgual(float x, float y) { if (x >= y) { return 1; } return 2; }
    public static int dMenor(double x, double y) { if (x < y) { return 1; } return 2; }
}
`
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Ram.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	suya := claseRam(t)
	for _, m := range suya.Methods {
		if !m.HasCode {
			continue
		}
		a, ok := buscaMetodo(nuestra, m.Name)
		if !ok {
			t.Fatalf("%s no está en nuestra clase", m.Name)
		}
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, m); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", m.Name, x, y)
		}
	}
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if string(deUno) != string(deOtro) {
		t.Errorf("el dex nuestro son %d bytes y el de javac %d", len(deUno), len(deOtro))
	}
}
