//go:build !js

package main

// La lambda del fuente contra el android.jar de stubs que sirve la página. Ese jar no escribe el
// atributo `Signature` y el escritor que lo generó dejaba los `default` de la API como abstractos:
// `java/util/function/Consumer` sale con `accept(LT;)V` y `andThen(...)Consumer`, los dos con
// `ACC_ABSTRACT`. `samDe` los leía como dos métodos funcionales y descartaba la única sobrecarga
// del constructor que recibe la lambda. El oráculo sigue siendo javac 17
// `--release 8 -g:lines,source`: `--release` toma `java.util.function.Consumer` del JDK, no del jar,
// así que el dex de referencia es el mismo tanto con el stub como con el android.jar del SDK.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteCuatroCandidatas: la forma mínima del bloqueo de la página. `ApkAdapter` hereda de
// `ArrayAdapter` los tres constructores `(Context,int,int)`, `(Context,int,Object[])` y
// `(Context,int,List)`, y declara el suyo `(Context,List<Apk>,Consumer<Apk>)`. Son cuatro
// candidatas de tres argumentos y solo la última puede recibir la lambda.
const fuenteCuatroCandidatas = `package p;

import java.util.List;
import java.util.function.Consumer;

class Context {}

class ArrayAdapter {
    ArrayAdapter(Context c, int resource, int id) {}
    ArrayAdapter(Context c, int resource, Object[] objects) {}
    ArrayAdapter(Context c, int resource, List objects) {}
}

class Apk {}

class ApkAdapter extends ArrayAdapter {
    ApkAdapter(Context c, List<Apk> apks, Consumer<Apk> fn) {
        super(c, 0, apks);
    }
}

class X {
    void f(List<Apk> apks) {
        new ApkAdapter(new Context(), apks, apk -> g(apk));
    }

    void g(Apk apk) {}
}
`

// fuenteDosAbstractos: una interfaz del fuente con dos métodos abstractos de verdad. La
// exclusión de `default` del stub no puede hacerla funcional.
const fuenteDosAbstractos = `package p;

interface Dos {
    void uno(Object o);
    void dos(Object o);
}

class H {
    H(Dos d) {}
}

class X {
    void f() {
        new H(x -> g(x));
    }

    void g(Object o) {}
}
`

// stubJarDelCache: el android.jar de stubs publicado, con el defecto todavía presente. Se puede
// apuntar a otra copia con `APKC_STUB_JAR`. No se salta: sin el jar no se puede medir nada.
func stubJarDelCache(t *testing.T) string {
	t.Helper()
	ruta := os.Getenv("APKC_STUB_JAR")
	if ruta == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("sin HOME para buscar el jar de stubs: %v", err)
		}
		ruta = filepath.Join(home, ".cache", "apkc-web", "androidjar", "android-stub-34-full.jar")
	}
	b, err := abrirBuscador(ruta)
	if err != nil {
		t.Fatalf("sin android.jar de stubs en %s: %v", ruta, err)
	}
	b.cerrar()
	return ruta
}

// compruebaElDefectoDelStub: la medida que justifica el test. Si el jar ya trae `Signature` o el
// `andThen` default bien marcado, esta prueba no está midiendo lo que dice su nombre.
func compruebaElDefectoDelStub(t *testing.T, ruta string) {
	t.Helper()
	cp, err := abrirClasspath([]string{ruta})
	if err != nil {
		t.Fatalf("abriendo el classpath del stub: %v", err)
	}
	defer cp.cerrar()
	cf, err := cp.claseDelJar("java/util/function/Consumer")
	if err != nil {
		t.Fatalf("leyendo Consumer del stub: %v", err)
	}
	if cf.Signature != "" {
		t.Fatalf("el stub ya trae Signature de clase (%q): el defecto desaparecio", cf.Signature)
	}
	for _, m := range cf.Methods {
		if m.Name == "accept" {
			if m.Desc != "(LT;)V" || m.Signature != "" {
				t.Fatalf("accept del stub: descriptor %q, Signature %q", m.Desc, m.Signature)
			}
		}
		if m.Name == "andThen" {
			if m.Access&accAbstract == 0 || m.HasCode {
				t.Fatalf("andThen del stub ya no es abstracto sin Code: flags %#x, Code %v", m.Access, m.HasCode)
			}
		}
	}
}

// javacConFuentes: el oráculo. `--release 8` resuelve `java.util.function.Consumer` en el JDK,
// mientras que nuestro emisor lo lee del jar que se le pase.
func javacConFuentes(t *testing.T, javac, jar string, fuentes []fuenteJava, raiz string) []classFull {
	t.Helper()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	salida := filepath.Join(raiz, "javac-out")
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida}
	for _, f := range fuentes {
		args = append(args, filepath.Join(raiz, f.nombre[len("src/"):]))
	}
	if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	clases, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	return clases
}

// TestSAMDelStubSinSignatureCoincideConJavac17: el bloqueo de la página, con el jar de stubs de un
// lado y javac del otro. El dex entero byte a byte exige el mismo SAM borrado e instanciado.
func TestSAMDelStubSinSignatureCoincideConJavac17(t *testing.T) {
	stub := stubJarDelCache(t)
	compruebaElDefectoDelStub(t, stub)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	deJavac := javacConFuentes(t, javac, stub, fuentesLambda, raiz)

	for _, caso := range []struct {
		nombre string
		jar    string
	}{
		{"stub", stub},
		{"sdk", jarDelSDK(t)},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			nuestras, err := compilarProyectoJava(fuentesLambda, caso.jar)
			if err != nil {
				t.Fatalf("el emisor rechazo la lambda con %s: %v", caso.nombre, err)
			}
			emitida := porInterno(nuestras, "share/apps/util/MainActivity")
			if emitida == nil {
				t.Fatalf("no salio MainActivity (%s)", nombresDe(nuestras))
			}
			nuestra := leer(t, emitida)
			m, ok := buscaMetodo(nuestra, "lambda$openAppDetailsDialog$0")
			if !ok || m.Desc != "(Lshare/apps/util/Apk;)V" {
				t.Fatalf("el implementador no toma el Apk del parametro formal: %+v", m)
			}
			if len(nuestra.BootstrapMethods) != 1 {
				t.Fatalf("%d filas de BootstrapMethods, se esperaba una", len(nuestra.BootstrapMethods))
			}
			suyo := porNombre(t, deJavac, "Lshare/apps/util/MainActivity;")
			comparaMetodos(t, nuestra, suyo)
			if err := comparaDex(t, nuestras, deJavac); err != nil {
				t.Fatal(err)
			}
			t.Logf("dex byte a byte con javac 17: %d B", longitudDelDex(t, clasesLeidas(t, nuestras)))
		})
	}
}

// TestFalsificacionDelStubRestauraElBloqueo: con el interruptor encendido el lector se queda con
// el defecto publicado y vuelve a salir el error literal de las cuatro candidatas.
func TestFalsificacionDelStubRestauraElBloqueo(t *testing.T) {
	stub := stubJarDelCache(t)
	compruebaElDefectoDelStub(t, stub)
	fuentes := []fuenteJava{{nombre: "X.java", contenido: fuenteCuatroCandidatas}}

	if _, err := compilarProyectoJava(fuentes, stub); err != nil {
		t.Fatalf("con la correccion puesta, la llamada con cuatro candidatas fallo: %v", err)
	}

	tratarDefaultsDeStubComoAbstractos = true
	defer func() { tratarDefaultsDeStubComoAbstractos = false }()
	_, err := compilarProyectoJava(fuentes, stub)
	if err == nil {
		t.Fatal("la falsificacion acepto la lambda: el interruptor no restauro el bloqueo")
	}
	quiero := "tiene 4 candidatas de 3 argumentos"
	if !strings.Contains(err.Error(), quiero) {
		t.Fatalf("el error no cuenta las cuatro candidatas: %v", err)
	}
	quiero = "ninguna recibe una interface funcional donde va la lambda"
	if !strings.Contains(err.Error(), quiero) {
		t.Fatalf("el error no es el literal de la pagina: %v", err)
	}
	quiero = "(Lp/Context;Ljava/util/List;Ljava/util/function/Consumer;)V"
	if !strings.Contains(err.Error(), quiero) {
		t.Fatalf("el error no nombra la candidata de Consumer: %v", err)
	}
	t.Logf("error falsificado: %v", err)
}

// TestInterfazConDosAbstractosSeSigueRechazando: la corrección no convierte en funcional cualquier
// interfaz con un método que cuadre; dos métodos abstractos de verdad se rechazan por su nombre.
func TestInterfazConDosAbstractosSeSigueRechazando(t *testing.T) {
	stub := stubJarDelCache(t)
	fuentes := []fuenteJava{{nombre: "X.java", contenido: fuenteDosAbstractos}}
	_, err := compilarProyectoJava(fuentes, stub)
	if err == nil {
		t.Fatal("el emisor acepto una interface con dos metodos abstractos")
	}
	if !strings.Contains(err.Error(), "ninguna recibe una interface funcional donde va la lambda") {
		t.Fatalf("el rechazo no es el de la sobrecarga sin candidata: %v", err)
	}
	t.Logf("rechazo: %v", err)
}

// TestVariablesDeDescriptorStub: la convención del jar (`LT;`, `LR;`) no se confunde con un tipo
// real (`Ljava/lang/Object;`, `LApk;`). Sin esto el descriptor del SAM queda sin instanciar.
func TestVariablesDeDescriptorStub(t *testing.T) {
	casos := []struct {
		desc   string
		quiero []string
	}{
		{"(LT;)V", []string{"T"}},
		{"(LT;LU;)LR;", []string{"T", "U", "R"}},
		{"(Ljava/lang/Object;)V", nil},
		{"(LApk;)V", nil},
		{"(LT;Ljava/lang/Object;)V", []string{"T"}},
	}
	for _, c := range casos {
		got := variablesDeDescriptor(c.desc)
		if strings.Join(got, ",") != strings.Join(c.quiero, ",") {
			t.Errorf("variablesDeDescriptor(%q) = %v, queria %v", c.desc, got, c.quiero)
		}
	}
}
