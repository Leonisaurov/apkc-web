package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Este fichero cierra las dos fronteras que quedaban en `AppAdapter.java` de ApkShare y que el test
// del constructor dejó escritas: el `check-cast` detrás de la llamada borrada y el puente del retorno
// covariante. Las dos se miden contra javac 17 `--release 8 -g:lines,source` y contra su dex, que es
// el oráculo del repo; y la primera se falsifica además con el verificador de la JVM, que es el que
// destapa el defecto de verdad (`Type 'java/lang/Object' … is not assignable to 'share/apps/util/App'`).

// claseEmitidaPorNombre: la `.class` de ese nombre interno en lo que emitimos nosotros.
func claseEmitidaPorNombre(t *testing.T, clases []claseEmitida, interno string) []byte {
	t.Helper()
	for _, c := range clases {
		if c.interno == interno {
			return c.datos
		}
	}
	t.Fatalf("no se emitió %s", interno)
	return nil
}

// claseLeidaPorNombre: la `.class` de ese nombre en descriptor (`Lshare/apps/util/AppAdapter;`) en lo
// que emitió javac.
func claseLeidaPorNombre(t *testing.T, clases []classFull, nombre string) classFull {
	t.Helper()
	for _, c := range clases {
		if c.Name == nombre {
			return c
		}
	}
	t.Fatalf("javac no dejó %s", nombre)
	return classFull{}
}

// dexDeUnaClase: el dex de esa única clase. Sirve para comparar byte a byte una clase suelta cuando
// el proyecto entero trae otras fronteras abiertas: el `Apk.java` de ApkShare pliega
// `GET_ACTIVITIES | GET_META_DATA` a 129 y nuestro emisor todavía escribe el `or-int` (otro frente),
// y eso no tiene nada que ver con `AppAdapter`.
func dexDeUnaClase(t *testing.T, datos []byte) []byte {
	t.Helper()
	leida, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	return dexDeUnaClaseLeida(t, leida)
}

// dexDeUnaClaseLeida: lo mismo para una clase que ya viene leída de javac.
func dexDeUnaClaseLeida(t *testing.T, c classFull) []byte {
	t.Helper()
	dex, err := dexDeClases([]classFull{c})
	if err != nil {
		t.Fatalf("dexando la clase: %v", err)
	}
	return dex
}

// TestElDexDeAppAdapterEsElDeJavac17: la clase entera —los cinco métodos, el puente y los cuatro
// `check-cast`— dexa como la de javac 17, byte a byte. Sin el `check-cast` de la línea 73 el dex no
// coincide (y el verificador rechaza el método); sin el puente falta un `getItem` entero.
func TestElDexDeAppAdapterEsElDeJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesAppAdapterShare()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes de ApkShare: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra := claseEmitidaPorNombre(t, nuestras, "share/apps/util/AppAdapter")
	suya := claseLeidaPorNombre(t, deJavac, "Lshare/apps/util/AppAdapter;")
	comparaDexByteAByte(t, dexDeUnaClase(t, nuestra), dexDeUnaClaseLeida(t, suya), volcadoDeClases(nuestras))

	// el conteo que el builder anterior midió a mano: `grep -c getItem` daba 2 en javac y 1 en lo
	// nuestro. Ahora son los dos, y el puente lleva los flags de javac (0x1041).
	leida, err := readClassFull(nuestra)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	getItems := []classMethod{}
	for _, m := range leida.Methods {
		if m.Name == "getItem" {
			getItems = append(getItems, m)
		}
	}
	if len(getItems) != 2 {
		t.Fatalf("se esperaban dos `getItem` (el estrecho y su puente) y hay %d", len(getItems))
	}
	puente := metodoDeClaseEmitida(t, nuestras, "share/apps/util/AppAdapter", "getItem", "(I)Ljava/lang/Object;")
	if puente.Access != accPublic|accBridge|accSynthetic {
		t.Fatalf("el puente sale con flags %#x y javac escribe %#x", puente.Access, accPublic|accBridge|accSynthetic)
	}
	if err := comparaInstrucciones(puente.Code, metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/AppAdapter;", "getItem", "(I)Ljava/lang/Object;").Code); err != nil {
		t.Fatalf("el cuerpo del puente no es el de javac: %v", err)
	}
}

// fuenteSondaCast: el caso mínimo del `check-cast`, con las dos direcciones en el mismo método. `get`
// borra a `Object`, así que el hueco tipado necesita el `check-cast` y el hueco `Object` no. Es la
// falsificación de la regla: si se emitiera siempre, `Object o = …` saldría con una instrucción de
// más, y si no se emitiera nunca, `App a = …` quedaría con `Object` donde la firma declara `App`.
const fuenteSondaCast = `package p;

import java.util.ArrayList;
import java.util.List;

class App {
    String nombre = "x";
}

class Sonda {
    private List<App> lista;

    Sonda(List<App> lista) {
        this.lista = lista;
    }

    App primero() {
        return lista.get(0);
    }

    Object crudo() {
        Object o = lista.get(0);
        return o;
    }

    String nombre() {
        App a = lista.get(1);
        return a.nombre;
    }

    void guarda(List<App> otras) {
        for (App cada : lista) {
            otras.add(cada);
        }
    }
}
`

// TestElCheckcastSoloEnElHuecoTipado: el dex de la sonda byte a byte contra javac 17 y, además, la
// lectura de las instrucciones para dejar escrito qué hace cada dirección.
func TestElCheckcastSoloEnElHuecoTipado(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteSondaCast}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCast)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	comparaDexByteAByte(t, dexDeUnaClase(t, claseEmitidaPorNombre(t, nuestras, "p/Sonda")),
		dexDeUnaClaseLeida(t, claseLeidaPorNombre(t, deJavac, "Lp/Sonda;")), volcadoDeClases(nuestras))

	// `primero` devuelve `App` desde un `get` borrado: `checkcast` y `areturn`
	primero := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "primero", "()Lp/App;")
	if n := cuentaCheckcast(t, primero.Code); n != 1 {
		t.Fatalf("`return lista.get(0)` con la vuelta `App` lleva %d `checkcast` y javac escribe 1", n)
	}
	// `crudo` devuelve `Object`: el hueco ES `Object` y no hay conversión que escribir
	crudo := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "crudo", "()Ljava/lang/Object;")
	if n := cuentaCheckcast(t, crudo.Code); n != 0 {
		t.Fatalf("`Object o = lista.get(0)` lleva %d `checkcast` y javac no escribe ninguno", n)
	}
	// `nombre` declara la local `App`: cast detrás del `invokeinterface` y después el `getfield`
	nombre := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "nombre", "()Ljava/lang/String;")
	if n := cuentaCheckcast(t, nombre.Code); n != 1 {
		t.Fatalf("`App a = lista.get(1)` lleva %d `checkcast` y javac escribe 1", n)
	}
}

// cuentaCheckcast: cuántos `checkcast` (0xc0) hay en el Code.
func cuentaCheckcast(t *testing.T, code []byte) int {
	t.Helper()
	ins, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el Code no se desarma: %v", err)
	}
	n := 0
	for _, i := range ins {
		if i.op == opCheckcast {
			n++
		}
	}
	return n
}

// fuenteSondaPuente: el retorno covariante sobre una clase del propio fuente. `D.m()` devuelve `String`
// y `B.m()` `Object`, así que javac sintetiza `m:()Ljava/lang/Object;` en `D`.
const fuenteSondaPuente = `package p;

class B {
    Object m() {
        return null;
    }
}

class D extends B {
    @Override
    public String m() {
        return "a";
    }
}
`

// TestElPuenteCovarianteDeUnaClasePropia: el mismo puente sobre una jerarquía de los fuentes, para
// que no quede pegado a la clase del `android.jar`. `D` dexa como la de javac, y en particular tiene
// los dos `m`.
func TestElPuenteCovarianteDeUnaClasePropia(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/D.java", contenido: fuenteSondaPuente}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/D.java", fuenteSondaPuente)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	comparaDexByteAByte(t, dexDeUnaClase(t, claseEmitidaPorNombre(t, nuestras, "p/D")),
		dexDeUnaClaseLeida(t, claseLeidaPorNombre(t, deJavac, "Lp/D;")), volcadoDeClases(nuestras))

	leida, err := readClassFull(claseEmitidaPorNombre(t, nuestras, "p/D"))
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	n := 0
	for _, m := range leida.Methods {
		if m.Name == "m" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("`D` sale con %d métodos `m` y javac escribe 2 (el estrecho y el puente)", n)
	}
}

// TestFalsificacionDelCheckcast: apagada la ruta, el dex de `AppAdapter` deja de ser el de javac y la
// comprobación dice exactamente qué instrucción sobra. Es la prueba de que la coincidencia de arriba
// es del `check-cast` y no de otra cosa.
func TestFalsificacionDelCheckcast(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesAppAdapterShare()

	anterior := insertaCheckcast
	insertaCheckcast = false
	nuestras, err := compilarProyectoJava(fuentes, jar)
	insertaCheckcast = anterior
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes con la ruta apagada: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestro := metodoDeClaseEmitida(t, nuestras, "share/apps/util/AppAdapter", "getItem", "(I)Lshare/apps/util/App;")
	suyo := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/AppAdapter;", "getItem", "(I)Lshare/apps/util/App;")
	// lo que sobra es exactamente un `checkcast`: nuestras instrucciones son las de javac sin él
	nuestrasIns, err := walkJvmCode(nuestro.Code)
	if err != nil {
		t.Fatalf("nuestro Code no se desarma: %v", err)
	}
	suyasIns, err := walkJvmCode(suyo.Code)
	if err != nil {
		t.Fatalf("el Code de javac no se desarma: %v", err)
	}
	if n := cuentaCheckcast(t, suyo.Code); n != 1 {
		t.Fatalf("javac no escribe un `checkcast` en `getItem` sino %d", n)
	}
	if n := cuentaCheckcast(t, nuestro.Code); n != 0 {
		t.Fatalf("con la ruta apagada el método lleva %d `checkcast`", n)
	}
	filtradas := []jvmInstr{}
	for _, i := range suyasIns {
		if i.op == opCheckcast {
			continue
		}
		filtradas = append(filtradas, i)
	}
	if len(filtradas) != len(nuestrasIns) {
		t.Fatalf("sin el `checkcast`, javac dejaría %d instrucciones y nosotros escribimos %d:\n%v",
			len(filtradas), len(nuestrasIns), nuestrasIns)
	}
	for k := range filtradas {
		if filtradas[k].op != nuestrasIns[k].op {
			t.Fatalf("en %d (pc %d) la opcode es %d y javac escribe %d sin el cast",
				k, nuestrasIns[k].pc, nuestrasIns[k].op, filtradas[k].op)
		}
	}
	t.Logf("el `getItem` sin `check-cast` tiene %d instrucciones y el de javac %d: la que falta es el `checkcast` de `App`",
		len(nuestrasIns), len(suyasIns))
}

// TestFalsificacionDelPuente: apagada la síntesis del puente, `AppAdapter` vuelve a tener un solo
// `getItem` —el defecto del informe— y el dex de la clase deja de coincidir con el de javac.
func TestFalsificacionDelPuente(t *testing.T) {
	jar := jarDelSDK(t)
	fuentes := fuentesAppAdapterShare()

	anterior := sintetizaPuentesCovariantes
	sintetizaPuentesCovariantes = false
	nuestras, err := compilarProyectoJava(fuentes, jar)
	sintetizaPuentesCovariantes = anterior
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes con la ruta apagada: %v", err)
	}
	leida, err := readClassFull(claseEmitidaPorNombre(t, nuestras, "share/apps/util/AppAdapter"))
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	n := 0
	for _, m := range leida.Methods {
		if m.Name == "getItem" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("sin el puente `AppAdapter` sale con %d `getItem` y se esperaba 1 (el defecto original)", n)
	}
}

// TestElDexDeAppAdapterPasaElVerificadorDeLaJVM: el oráculo que destapa el defecto de verdad. Con la
// ruta del `check-cast` encendida, nuestras clases se cargan y todos sus métodos se verifican; con la
// ruta apagada, el verificador rechaza `getView` con `Type 'java/lang/Object' … is not assignable to
// 'share/apps/util/App'`, que es el motivo por el que el dex no llegaría al aparato. Se salta si no
// hay `java` junto al `javac` del oráculo.
func TestElDexDeAppAdapterPasaElVerificadorDeLaJVM(t *testing.T) {
	javac := javac17(t)
	java := filepath.Join(filepath.Dir(javac), "java")
	if _, err := os.Stat(java); err != nil {
		t.Skipf("sin `java` junto a %s: %v", javac, err)
	}
	jar := jarDelSDK(t)

	// la sonda que carga las clases sin llamar a su constructor (los `Stub!` del `android.jar`) y
	// verifica cada método declarado al invocarlo
	const sonda = `import java.lang.reflect.Field;
import java.lang.reflect.Method;

public class VerificarSonda {
    public static void main(String[] args) throws Exception {
        Class<?> unsafeC = Class.forName("sun.misc.Unsafe");
        Field f = unsafeC.getDeclaredField("theUnsafe");
        f.setAccessible(true);
        Object unsafe = f.get(null);
        Method alloc = unsafeC.getMethod("allocateInstance", Class.class);
        for (String arg : args) {
            Class<?> c = Class.forName(arg, false, VerificarSonda.class.getClassLoader());
            Object o = alloc.invoke(unsafe, c);
            for (Method m : c.getDeclaredMethods()) {
                Class<?>[] ps = m.getParameterTypes();
                Object[] as = new Object[ps.length];
                boolean sirve = true;
                for (int i = 0; i < ps.length; i++) {
                    if (ps[i] == int.class) as[i] = 0;
                    else if (ps[i] == long.class) as[i] = 0L;
                    else if (ps[i] == boolean.class) as[i] = false;
                    else if (ps[i].isPrimitive()) sirve = false;
                    else as[i] = null;
                }
                if (!sirve) continue;
                try {
                    m.invoke(o, as);
                } catch (java.lang.reflect.InvocationTargetException esperado) {
                    // un NPE o un índice fuera de rango es el cuerpo del método corriendo: la
                    // verificación ya pasó. Lo que no vale es un VerifyError.
                }
            }
            System.out.println("verificado: " + arg);
        }
    }
}
`
	raiz := t.TempDir()
	pon(t, raiz, "VerificarSonda.java", sonda)
	if out, err := exec.Command(javac, "-d", raiz, filepath.Join(raiz, "VerificarSonda.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac no compiló la sonda del verificador: %v\n%s", err, out)
	}

	// las clases de las dos variantes, en su directorio
	escribe := func(dir string, clases []claseEmitida, interno string) string {
		destino := filepath.Join(dir, filepath.FromSlash(interno)+".class")
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destino, claseEmitidaPorNombre(t, clases, interno), 0o644); err != nil {
			t.Fatal(err)
		}
		return destino
	}
	fuentes := fuentesAppAdapterShare()
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes: %v", err)
	}
	con := t.TempDir()
	escribe(con, nuestras, "share/apps/util/AppAdapter")

	anterior := insertaCheckcast
	insertaCheckcast = false
	sin, err := compilarProyectoJava(fuentes, jar)
	insertaCheckcast = anterior
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes con la ruta apagada: %v", err)
	}
	sinDir := t.TempDir()
	escribe(sinDir, sin, "share/apps/util/AppAdapter")

	// las dos variantes necesitan `App` (la de fuera del hueco del `checkcast` es la que el
	// verificador tiene que rechazar, y `App` es el tipo del campo que lee `getView`)
	escribe(con, nuestras, "share/apps/util/App")
	escribe(sinDir, sin, "share/apps/util/App")

	verifica := func(dir string) (string, error) {
		cmd := exec.Command(java, "-Xverify:all", "-cp", strings.Join([]string{raiz, dir, jar}, string(os.PathListSeparator)), "VerificarSonda", "share.apps.util.AppAdapter")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := verifica(con)
	if err != nil {
		t.Fatalf("el verificador rechazó nuestro `AppAdapter` con la ruta encendida: %v\n%s", err, out)
	}
	if !strings.Contains(out, "verificado: share.apps.util.AppAdapter") {
		t.Fatalf("la sonda no verificó la clase: %s", out)
	}
	out, err = verifica(sinDir)
	if err == nil {
		t.Fatalf("el verificador aceptó el `AppAdapter` sin `check-cast`: la falsificación no prueba nada\n%s", out)
	}
	if !strings.Contains(out, "VerifyError") || !strings.Contains(out, "not assignable to 'share/apps/util/App'") {
		t.Fatalf("el rechazo no es el del hueco tipado:\n%s", out)
	}
	t.Logf("verificador sin check-cast: %s", primeraLineaDeVerifyError(out))
}

// primeraLineaDeVerifyError: la línea del `VerifyError` y su motivo, para el log del test.
func primeraLineaDeVerifyError(salida string) string {
	lineas := strings.Split(salida, "\n")
	for i, l := range lineas {
		if strings.Contains(l, "VerifyError") && i+1 < len(lineas) {
			return strings.TrimSpace(l) + " | " + strings.TrimSpace(lineas[i+1])
		}
	}
	return strings.TrimSpace(salida)
}
