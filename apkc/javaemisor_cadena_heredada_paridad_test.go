//go:build !js

// La paridad con javac 17 de javaemisor_cadena_heredada_test.go, que se mueve entero: todas sus pruebas son oráculo de javac,
// que se lanza como proceso, y el hospedador de la capa wasm no crea procesos. Por eso el fichero
// lleva `//go:build !js` y no se salta en silencio bajo `GOOS=js`. No hay SKIP: sin javac 17
// fallan (ver `dependencias_test.go:5-8`).

package main

// Un `static final String` con `ConstantValue` heredado de un padre del classpath es también una
// constante de compilación cuando se nombra suelto, sin receptor: `class Hijo extends Base` con
// `Base.V` en un jar emite `ldc` de la cadena, no un `getstatic`. Antes de este frente, la rama de
// plegado de `campoDeLaClaseActual` emitía el entero por defecto con el descriptor de referencia y
// dejaba bytecode inválido: `Hijo.java: linea N: ... pc 1 (areturn): se esperaba una referencia y hay
// entero`, que es el defecto que la crítica encontró. El oráculo es el de siempre: javac 17 con
// `--release 8 -g:lines,source` por el mismo dexer, el dex entero byte a byte, y el verificador de la
// JVM.
//
// Los dos controles van en el mismo fuente: `Base.N` es un `static final int` heredado y se sigue
// plegando como entero (`bipush 7`), y `Base.M` es un `static String` **sin** `ConstantValue`, así que
// sigue siendo un `getstatic`.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fuenteBaseHeredada = `package p;
public class Base {
    public static final String V = "hola";
    public static final int N = 7;
    public static String M = "mutable";
}
`

const fuenteHijoHeredado = `package p;
public class Hijo extends Base {
    public String dame() {
        return V;
    }

    public int num() {
        return N;
    }

    public String mutable() {
        return M;
    }
}
`

// baseHeredadaEnJar: compila `Base` con el javac del oráculo y la deja en un jar del classpath, que es
// la forma en que el campo llega por la rama de `buscarCampo` que lee `ConstantValue` del jar.
func baseHeredadaEnJar(t *testing.T, javac string) string {
	t.Helper()
	raiz := t.TempDir()
	pon(t, raiz, "base/p/Base.java", fuenteBaseHeredada)
	salida := filepath.Join(raiz, "baseclasses")
	if out, err := exec.Command(javac, "--release", "8", "-d", salida, filepath.Join(raiz, "base", "p", "Base.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó Base: %v\n%s", err, out)
	}
	jarBin := filepath.Join(filepath.Dir(javac), "jar")
	baseJar := filepath.Join(raiz, "base.jar")
	if out, err := exec.Command(jarBin, "cf", baseJar, "-C", salida, ".").CombinedOutput(); err != nil {
		t.Fatalf("`jar` rechazó Base: %v\n%s", err, out)
	}
	return baseJar
}

// cuentaOp: cuántas veces aparece un opcode en el cuerpo desarmado de un método.
func cuentaOp(t *testing.T, metodo classMethod, op byte) int {
	t.Helper()
	instr, err := walkJvmCode(metodo.Code)
	if err != nil {
		t.Fatalf("no se desarma el método %s: %v", metodo.Name, err)
	}
	n := 0
	for _, in := range instr {
		if in.op == op {
			n++
		}
	}
	return n
}

// TestCadenaEstaticaHeredadaContraJavac17: `V` suelto y heredado del classpath sale como `ldc` de la
// cadena y el dex de `Hijo` entero coincide con el de javac. `N` sigue plegándose como entero y `M`,
// sin `ConstantValue`, sigue leyéndose con `getstatic`.
func TestCadenaEstaticaHeredadaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	baseJar := baseHeredadaEnJar(t, javac)
	classpath := strings.Join([]string{jar, baseJar}, string(os.PathListSeparator))
	fuentes := []fuenteJava{{nombre: "src/p/Hijo.java", contenido: fuenteHijoHeredado}}

	nuestras, err := compilarProyectoJavaConClasspath(fuentes, []string{jar, baseJar})
	if err != nil {
		t.Fatalf("el emisor rechazó la constante `String` heredada: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "p/Hijo" {
		t.Fatalf("salieron %d clases (%s), y javac deja una", len(nuestras), nombresDe(nuestras))
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Hijo.java", fuenteHijoHeredado)
	deJavac := compilaConJavac(t, javac, classpath, raiz, fuentes)

	// `dame()`: un solo `ldc`, el de la cadena, y ni un `getstatic`. Si el campo viajara como acceso,
	// el opcode sería 0xb2 y el valor no estaría en el pool como String; si viajara como entero, el
	// análisis de marcos ya habría parado la emisión.
	nuestro := metodoDeClaseEmitida(t, nuestras, "p/Hijo", "dame", "()Ljava/lang/String;")
	suyo := metodoDeClaseLeida(t, deJavac, "Lp/Hijo;", "dame", "()Ljava/lang/String;")
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
		t.Fatalf("dame()Ljava/lang/String; no es el de javac: %v", err)
	}
	if ldc, getstatic := cuentaOp(t, nuestro, opLdc), cuentaOp(t, nuestro, opGetstatic); ldc != 1 || getstatic != 0 {
		t.Errorf("dame() tiene %d `ldc` y %d `getstatic`; se esperaba 1 y 0: `V` se dobla", ldc, getstatic)
	}
	nuestroPool := poolDeEmitida(t, nuestras, "p/Hijo")
	instr, err := walkJvmCode(nuestro.Code)
	if err != nil {
		t.Fatalf("no se desarma nuestro dame(): %v", err)
	}
	for _, in := range instr {
		if in.op != opLdc {
			continue
		}
		if got := nuestroPool.constantOf(int(in.ops[0])); got != "shola" {
			t.Errorf("el `ldc` de dame() carga %q y se esperaba la cadena `hola`", got)
		}
	}

	// control 1: `N` es un `static final int` heredado y se sigue plegando como entero
	nuestroNum := metodoDeClaseEmitida(t, nuestras, "p/Hijo", "num", "()I")
	suyoNum := metodoDeClaseLeida(t, deJavac, "Lp/Hijo;", "num", "()I")
	if err := comparaInstrucciones(nuestroNum.Code, suyoNum.Code); err != nil {
		t.Fatalf("num()I no es el de javac: %v", err)
	}
	if getstatic := cuentaOp(t, nuestroNum, opGetstatic); getstatic != 0 {
		t.Errorf("num() tiene %d `getstatic`: `N` es constante y se pliega como entero", getstatic)
	}

	// control 2: `M` no trae `ConstantValue` y sigue siendo un `getstatic`
	nuestroMutable := metodoDeClaseEmitida(t, nuestras, "p/Hijo", "mutable", "()Ljava/lang/String;")
	suyoMutable := metodoDeClaseLeida(t, deJavac, "Lp/Hijo;", "mutable", "()Ljava/lang/String;")
	if err := comparaInstrucciones(nuestroMutable.Code, suyoMutable.Code); err != nil {
		t.Fatalf("mutable()Ljava/lang/String; no es el de javac: %v", err)
	}
	if ldc, getstatic := cuentaOp(t, nuestroMutable, opLdc), cuentaOp(t, nuestroMutable, opGetstatic); ldc != 0 || getstatic != 1 {
		t.Errorf("mutable() tiene %d `ldc` y %d `getstatic`; `M` no tiene ConstantValue y se lee con getstatic", ldc, getstatic)
	}

	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de Hijo no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// el verificador de la JVM: el bytecode plegado tiene que cargar y verificar
	verificaHeredadaContraLaJVM(t, javac, nuestras, baseJar)
}

// verificaHeredadaContraLaJVM escribe nuestro `p/Hijo` y lo carga con `-Xverify:all` desde una sonda
// que invoca cada método declarado sobre una instancia sin constructor (los `Stub!` no llegan a
// tocarse: `Base` viene del jar de javac). Se salta si no hay `java` junto al `javac` del oráculo.
func verificaHeredadaContraLaJVM(t *testing.T, javac string, nuestras []claseEmitida, baseJar string) {
	t.Helper()
	java := filepath.Join(filepath.Dir(javac), "java")
	if _, err := os.Stat(java); err != nil {
		t.Skipf("sin `java` junto a %s: %v", javac, err)
	}
	const sonda = `import java.lang.reflect.Field;
import java.lang.reflect.Method;

public class VerificarHeredada {
    public static void main(String[] args) throws Exception {
        Class<?> unsafeC = Class.forName("sun.misc.Unsafe");
        Field f = unsafeC.getDeclaredField("theUnsafe");
        f.setAccessible(true);
        Object unsafe = f.get(null);
        Method alloc = unsafeC.getMethod("allocateInstance", Class.class);
        Class<?> c = Class.forName(args[0], false, VerificarHeredada.class.getClassLoader());
        Object o = alloc.invoke(unsafe, c);
        for (Method m : c.getDeclaredMethods()) {
            m.invoke(o);
        }
        System.out.println("verificado: " + args[0]);
    }
}
`
	raiz := t.TempDir()
	pon(t, raiz, "VerificarHeredada.java", sonda)
	if out, err := exec.Command(javac, "-d", raiz, filepath.Join(raiz, "VerificarHeredada.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac no compiló la sonda del verificador: %v\n%s", err, out)
	}
	destino := filepath.Join(raiz, "p", "Hijo.class")
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destino, claseEmitidaPorNombre(t, nuestras, "p/Hijo"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(java, "-Xverify:all", "-cp", strings.Join([]string{raiz, baseJar}, string(os.PathListSeparator)), "VerificarHeredada", "p.Hijo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("el verificador rechazó nuestro `Hijo`: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "verificado: p.Hijo") {
		t.Fatalf("la sonda no verificó la clase: %s", out)
	}
}

// TestCadenaEstaticaHeredadaFalsificacion: apagado el interruptor, el emisor revive el defecto entero
// —un `iconst` con el descriptor de referencia— y el análisis de marcos lo para con el error literal
// que leyó la crítica. Con el interruptor encendido el fuente compila.
func TestCadenaEstaticaHeredadaFalsificacion(t *testing.T) {
	const literal = "pc 1 (areturn): se esperaba una referencia y hay entero"
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	baseJar := baseHeredadaEnJar(t, javac)
	fuentes := []fuenteJava{{nombre: "src/p/Hijo.java", contenido: fuenteHijoHeredado}}

	if _, err := compilarProyectoJavaConClasspath(fuentes, []string{jar, baseJar}); err != nil {
		t.Fatalf("con el interruptor encendido el fuente no compila: %v", err)
	}

	anterior := pliegaCadenaHeredada
	pliegaCadenaHeredada = false
	t.Cleanup(func() { pliegaCadenaHeredada = anterior })

	_, err := compilarProyectoJavaConClasspath(fuentes, []string{jar, baseJar})
	if err == nil {
		t.Fatal("con el interruptor apagado el fuente fue aceptado: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con el interruptor apagado el rechazo es %q, y contiene %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "Hijo.java: ") {
		t.Fatalf("el rechazo no nombra el fuente: %q", err)
	}
	t.Logf("sin el interruptor: %v", err)
}
