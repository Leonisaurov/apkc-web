package main

// Fuzzing diferencial acotado y determinista: veinte fuentes pequeños, generados con una semilla fija a
// partir de las construcciones que el inventario de la Fase 5 ya da por medidas, y tres comparaciones por
// caso:
//
//  1. nuestro emisor escribe el `.class` y este fork lo dexa (`dexDeClases`);
//  2. `javac --release 8` compila el mismo fuente y el mismo dexer dexa sus clases; los dos dex tienen que
//     ser idénticos byte a byte, que es el oráculo estricto que ya usan los tests de una sola construcción;
//  3. `d8` dexa las dos versiones del `.class` por separado y los dos dex de `d8` tienen que coincidir. d8
//     normaliza el orden del pool y la disposición de las secciones, así que esta comparación no exige que
//     nuestro `.class` sea idéntico al de javac, solo equivalente.
//
// Diferencias aceptadas, y por qué no se comparan:
//
//   - el dex de este fork NO se compara byte a byte contra el de `d8`: d8 añade su propia marca de origen,
//     interna las secciones en otro orden y firma con otro layout (§22 y §59). Comparar los dos ficheros
//     sería medir el escritor de d8, no el nuestro; por eso se compara d8(lo nuestro) contra d8(lo de javac).
//   - `--release 8` es obligatorio en javac: desde Java 9 la concatenación de cadenas se baja con
//     `invokedynamic`, y el dexer de este fork la rechaza por nombre hasta el plan 5.22. Con `--release 8`
//     javac escribe StringBuilder, que es la forma que este emisor replica (§53).
//   - se pasa `-g:source,lines` y nunca `-g`: el emisor escribe LineNumberTable y SourceFile, no
//     LocalVariableTable; pedirle a javac la tabla de locales cambiaría el debug_info del dex.
//   - el generador se queda dentro del subconjunto medido: aritmética entera y ancha, `if`/`else`, `while`,
//     `for`, `switch` con caída, `try`/`catch`/`finally`, arreglos, `char`, concatenación, `instanceof`,
//     campos privados y constructores. Nada de `&&`, `||`, `++`, `--`, ternario, lambdas ni clases
//     anónimas: o no están en el inventario o su forma ya tiene su propio test y aquí solo añadiría ruido.
//
// Sin javac el test se salta (no puede construir la referencia); sin d8 se salta solo el tercer paso y se
// avisa. Las dos herramientas se pueden fijar con `JAVAC` y `D8`, y si no se buscan en el `PATH`.

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// casoFuzz es un fuente generado: un solo fichero con su clase pública.
type casoFuzz struct {
	clase  string
	fuente string
}

// TestFuzzDiferencialContraJavacYD8 recorre la semilla fija y compara, caso a caso, los tres caminos.
func TestFuzzDiferencialContraJavacYD8(t *testing.T) {
	jar := jarDelSDK(t)
	javacRuta := herramientaDeFuzz("JAVAC", "javac")
	if javacRuta == "" {
		t.Skip("sin javac en el PATH ni en JAVAC: no hay referencia contra la que comparar")
	}
	d8Ruta := herramientaDeFuzz("D8", "d8")
	if d8Ruta == "" {
		t.Log("sin d8 en el PATH ni en D8: se omite la comparación con d8 y queda la de javac")
	}

	casos := generarCasosFuzz(20, 20261121)
	if len(casos) != 20 {
		t.Fatalf("el generador devolvió %d casos, se piden veinte", len(casos))
	}
	for _, caso := range casos {
		caso := caso
		t.Run(caso.clase, func(t *testing.T) {
			proyecto := t.TempDir()
			nuestrasDir := filepath.Join(proyecto, "nuestras")
			javacDir := filepath.Join(proyecto, "javac")
			origen := filepath.Join(proyecto, "src", "p", caso.clase+".java")
			if err := os.MkdirAll(filepath.Dir(origen), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(origen, []byte(caso.fuente), 0o644); err != nil {
				t.Fatal(err)
			}

			// 1. nuestro emisor y nuestro dexer
			clases, err := compilarProyectoJava([]fuenteJava{
				{nombre: "src/p/" + caso.clase + ".java", contenido: caso.fuente},
			}, jar)
			if err != nil {
				t.Fatalf("el emisor rechazó el fuente generado: %v\n%s", err, caso.fuente)
			}
			if err := escribirClases(nuestrasDir, clases); err != nil {
				t.Fatal(err)
			}
			nuestras, err := leeClases([]string{nuestrasDir})
			if err != nil {
				t.Fatalf("no se relee lo que escribimos: %v", err)
			}
			dexNuestro, err := dexDeClases(nuestras)
			if err != nil {
				t.Fatalf("el dexer rechaza nuestras clases: %v", err)
			}

			// 2. javac como referencia, dexado por el mismo código
			if salida, err := ejecutarFuzz(javacRuta, "--release", "8", "-g:source,lines",
				"-d", javacDir, origen); err != nil {
				t.Fatalf("javac rechazó el fuente generado: %v\n%s\n%s", err, caso.fuente, salida)
			}
			suyas, err := leeClases([]string{javacDir})
			if err != nil {
				t.Fatalf("no se leen las clases de javac: %v", err)
			}
			dexJavac, err := dexDeClases(suyas)
			if err != nil {
				t.Fatalf("el dexer rechaza las clases de javac: %v", err)
			}
			if !bytes.Equal(dexNuestro, dexJavac) {
				t.Errorf("el dex de este fork no es el de javac: %d bytes contra %d, primer byte distinto %d",
					len(dexNuestro), len(dexJavac), primerByteDistinto(dexNuestro, dexJavac))
			}
			if _, err := readDex(dexNuestro); err != nil {
				t.Errorf("el dex que escribimos no se relee: %v", err)
			}

			// 3. d8 como segundo oráculo: normaliza los dos `.class` y los dos dex tienen que coincidir
			if d8Ruta == "" {
				return
			}
			dexD8Nuestro, err := dexarConD8(d8Ruta, proyecto, "d8-nuestro", nuestrasDir)
			if err != nil {
				t.Fatalf("d8 rechaza nuestras clases: %v", err)
			}
			dexD8Javac, err := dexarConD8(d8Ruta, proyecto, "d8-javac", javacDir)
			if err != nil {
				t.Fatalf("d8 rechaza las clases de javac: %v", err)
			}
			if !bytes.Equal(dexD8Nuestro, dexD8Javac) {
				t.Errorf("d8 no ve las dos clases iguales: %d bytes contra %d, primer byte distinto %d",
					len(dexD8Nuestro), len(dexD8Javac), primerByteDistinto(dexD8Nuestro, dexD8Javac))
			}
			if _, err := readDex(dexD8Nuestro); err != nil {
				t.Errorf("el dex de d8 no se relee: %v", err)
			}
		})
	}
}

// herramientaDeFuzz resuelve una herramienta externa: primero la variable de entorno, después el PATH.
func herramientaDeFuzz(variable, nombre string) string {
	if ruta := os.Getenv(variable); ruta != "" {
		if _, err := os.Stat(ruta); err == nil {
			return ruta
		}
		return ""
	}
	ruta, err := exec.LookPath(nombre)
	if err != nil {
		return ""
	}
	return ruta
}

// ejecutarFuzz lanza una herramienta y devuelve su salida combinada; un código distinto de cero es error.
func ejecutarFuzz(ruta string, args ...string) (string, error) {
	cmd := exec.Command(ruta, args...)
	salida, err := cmd.CombinedOutput()
	return string(salida), err
}

// dexarConD8 dexa todos los `.class` de un árbol con d8 y devuelve el `classes.dex` que escribe. Los
// ficheros se pasan ordenados: d8 deriva parte del orden de sus entradas del orden de los argumentos.
func dexarConD8(d8, proyecto, nombre, arbol string) ([]byte, error) {
	var ficheros []string
	err := filepath.Walk(arbol, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".class") {
			ficheros = append(ficheros, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(ficheros)
	if len(ficheros) == 0 {
		return nil, fmt.Errorf("no hay clases en %s", arbol)
	}
	salida := filepath.Join(proyecto, nombre)
	if err := os.MkdirAll(salida, 0o755); err != nil {
		return nil, err
	}
	args := append([]string{"--min-api", "21", "--output", salida}, ficheros...)
	if texto, err := ejecutarFuzz(d8, args...); err != nil {
		return nil, fmt.Errorf("%v\n%s", err, texto)
	}
	return os.ReadFile(filepath.Join(salida, "classes.dex"))
}

// generarCasosFuzz produce `n` fuentes pequeños con una semilla fija. La variación está en las constantes,
// los nombres de campo y el orden de los métodos; las construcciones son siempre las del subconjunto
// medido, para que un fallo signifique una diferencia de forma y no un fuente que el emisor no acepta.
func generarCasosFuzz(n int, semilla int64) []casoFuzz {
	r := rand.New(rand.NewSource(semilla))
	plantillas := []func(r *rand.Rand, clase, campo string, k, k2 int) string{
		plantillaCampos,
		plantillaWhile,
		plantillaFor,
		plantillaSwitch,
		plantillaTry,
		plantillaArreglos,
		plantillaAnchos,
		plantillaChar,
		plantillaCadena,
		plantillaInstanceof,
		plantillaConstructor,
		plantillaSobrecarga,
	}
	casos := make([]casoFuzz, 0, n)
	for i := 0; i < n; i++ {
		clase := fmt.Sprintf("Caso%02d", i)
		campo := []string{"n", "total", "valor", "cuenta"}[r.Intn(4)]
		k := 1 + r.Intn(9)
		k2 := 1 + r.Intn(20)
		fuente := plantillas[i%len(plantillas)](r, clase, campo, k, k2)
		casos = append(casos, casoFuzz{clase: clase, fuente: fuente})
	}
	return casos
}

// Las plantillas. Cada una es Java válido dentro del subconjunto y usa `clase`, `campo`, `k` y `k2` para
// que dos casos de la misma plantilla no sean el mismo texto.

func plantillaCampos(r *rand.Rand, clase, campo string, k, k2 int) string {
	op := []string{"+", "-", "*"}[r.Intn(3)]
	return fmt.Sprintf(`package p;
public class %s {
    int %s = %d;
    static int sTotal = %d;
    public int f(int x) {
        if (x > %d) { sTotal = sTotal + x; return x %s %s; }
        return x * %d;
    }
    public int g(int x, int y) { return x %% %d + y / 2; }
}
`, clase, campo, k, k2, k, op, campo, k2, 3+r.Intn(5))
}

func plantillaWhile(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    static int total = 0;
    public static int corre(int n) {
        int i = %d;
        while (i < n) {
            total = total + i;
            i = i + 1;
        }
        return total;
    }
    public int resta(int n) {
        int i = n;
        while (i > %d) { i = i - 1; }
        return i;
    }
}
`, clase, k%3, k)
}

func plantillaFor(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    int %s = 0;
    public int suma(int hasta) {
        for (int i = 0; i < hasta; i = i + 1) { %s = %s + i; }
        return %s;
    }
    public int acota(int hasta) {
        for (int i = %d; i < hasta; i = i + 2) { %s = %s + 1; }
        return %s;
    }
}
`, clase, campo, campo, campo, campo, k, campo, campo, campo)
}

func plantillaSwitch(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    int %s = 0;
    public int elige(int k) {
        switch (k) {
        case 0: %s = %d; break;
        case 1: %s = %d;
        case 2: %s = %s + %d; break;
        default: %s = %d;
        }
        return %s;
    }
}
`, clase, campo, campo, k, campo, k2, campo, campo, k2, campo, k, campo)
}

func plantillaTry(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    int %s = 0;
    public int f(int n) {
        try {
            if (n == 0) { return %s; }
            %s = n + %d;
        } catch (RuntimeException e) {
            %s = %d;
        } finally {
            %s = %s + %d;
        }
        return %s;
    }
    public int g(int n) {
        try {
            if (n < 0) { return 0 - n; }
            return n + %d;
        } catch (RuntimeException e) {
            return %d;
        }
    }
}
`, clase, campo, campo, campo, k, campo, k2, campo, campo, k2, campo, k2, k)
}

func plantillaArreglos(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    public int f(int n) {
        int[] a = new int[3];
        a[0] = n;
        a[1] = n + %d;
        a[2] = a[0] + a[1];
        return a[2] + a.length;
    }
    public long[] g(long x) {
        long[] b = new long[2];
        b[0] = x;
        b[1] = x + %d;
        return b;
    }
    public int dos(int[][] m) { return m[0][1] + m.length; }
}
`, clase, k, k2)
}

func plantillaAnchos(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    long a = %dL;
    static double d = 1.5;
    public long f(long x) { return x + a; }
    public double g(double x) { d = d * 2.0; return x + d; }
    public float h(float x) { return x - %d.5f; }
}
`, clase, k2, k)
}

func plantillaChar(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    int c = 97;
    public int f(char x) { if (x == 'z') { return %d; } return 0; }
    public int g() { return c; }
    public int h() { return 'A' + %d; }
}
`, clase, k, k2%8)
}

func plantillaCadena(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    int %s = %d;
    public String f(int x) { return "a" + x + "%d"; }
    public String g(int x) { return "v" + (x + %d); }
    public String h() { return "hola" + " " + %d; }
    public String campo() { return "c" + %s; }
}
`, clase, campo, k, k2, k, k2, campo)
}

func plantillaInstanceof(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    public int f(Object o) {
        if (o instanceof String) { return %d; }
        return 0;
    }
    public int g(Object o) {
        if (o instanceof int[]) { return %d; }
        return -1;
    }
    public int h(Object o) {
        if (!(o instanceof String)) { return %d; }
        return 1;
    }
}
`, clase, k, k2, k2)
}

func plantillaConstructor(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    int %s;
    %s() { this(%d); }
    %s(int v) { %s = v + %d; }
    public int f() { return %s; }
    public int g(int x) { return %s + x; }
}
`, clase, campo, clase, k, clase, campo, k2, campo, campo)
}

func plantillaSobrecarga(r *rand.Rand, clase, campo string, k, k2 int) string {
	return fmt.Sprintf(`package p;
public class %s {
    public int f(int x) { return x + %d; }
    public int f(int x, int y) { return x + y + %d; }
    public String f(String s) { return "s" + s; }
    public int f(char c) { return c; }
}
`, clase, k, k2)
}
