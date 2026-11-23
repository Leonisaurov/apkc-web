package main

// La lambda del fuente. El que abrió este frente fue la línea 156 de `MainActivity.java` del zip
// de ApkShare:
//
//	src/share/apps/util/MainActivity.java: linea 156: ">" no es una expresion que este parser reconozca
//
// La línea es `ApkAdapter adapter = new ApkAdapter(this, apks, apk -> shareApp(apk));`: el `->` no era
// un símbolo del léxico y la lambda no existía como expresión. El tipo funcional no viaja en la
// expresión —lo pone el parámetro formal, `Consumer<Apk>` (JLS 15.27.3)—, así que la resolución del
// constructor mira la declaración del fuente antes de bajar ningún argumento, escribe el implementador
// `lambda$openAppDetailsDialog$0` con `ACC_PRIVATE|ACC_SYNTHETIC` y emite el `invokedynamic` con el
// bootstrap `LambdaMetafactory.metafactory` que `lambda.go` ya desugra.
//
// El oráculo es javac 17 `--release 8 -g:lines,source`: los métodos (descriptor, flags, `max_stack`,
// `locals` y la LineNumberTable) y el dex entero byte a byte. El dex coincide porque los dos lados
// pasan por la misma expansión de lambdas (`expandirLambdas` nombra la sintética `$$Lambda$0` en los
// dos), así que lo que se compara de verdad es el sitio y el implementador, no el nombre que d8 elige.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteMainActivityLambda trae la línea 156 del zip verbatim, con la declaración de `ApkAdapter`
// recortada a su constructor (la del zip declara `(Context, List<Apk>, Consumer<Apk>)`, que es lo que
// decide el tipo de la lambda) y un `Apk` con el campo que usa `shareApp`.
const fuenteMainActivityLambda = `package share.apps.util;

import android.app.Activity;

import java.util.List;

public class MainActivity extends Activity {
    public void openAppDetailsDialog(List<Apk> apks) {
        ApkAdapter adapter = new ApkAdapter(this, apks, apk -> shareApp(apk));
    }

    public void shareApp(Apk apk) {
        apk.toString();
    }
}
`

// fuenteApkAdapterLambda: el constructor del zip, con `java.util.function.Consumer<Apk>` de verdad (el
// del `android.jar`), sin el `ArrayAdapter` del que hereda para no arrastrar la vista.
const fuenteApkAdapterLambda = `package share.apps.util;

import android.content.Context;

import java.util.List;
import java.util.function.Consumer;

public class ApkAdapter {
    private final List<Apk> apks;
    private final Consumer<Apk> fn;

    private final Context context;
    public ApkAdapter(Context context, List<Apk> apks, Consumer<Apk> fn) {
        this.context = context;
        this.apks = apks;
        this.fn = fn;
    }
}
`

const fuenteApkLambda = `package share.apps.util;

public class Apk {
    public final String filePath;

    public Apk(String filePath) {
        this.filePath = filePath;
    }

    @Override
    public String toString() {
        return filePath;
    }
}
`

var fuentesLambda = []fuenteJava{
	{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteMainActivityLambda},
	{nombre: "src/share/apps/util/ApkAdapter.java", contenido: fuenteApkAdapterLambda},
	{nombre: "src/share/apps/util/Apk.java", contenido: fuenteApkLambda},
}

// TestLambdaDeUnParametroContraJavac17: el sitio de la línea 156, con el implementador y el
// `invokedynamic` que escribe javac.
func TestLambdaDeUnParametroContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	for _, f := range fuentesLambda {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentesLambda, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la lambda de la línea 156: %v", err)
	}
	emitida := porInterno(nuestras, "share/apps/util/MainActivity")
	if emitida == nil {
		t.Fatalf("no salió MainActivity (%s)", nombresDe(nuestras))
	}

	salida := filepath.Join(raiz, "javac-out")
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida}
	for _, f := range fuentesLambda {
		args = append(args, filepath.Join(raiz, f.nombre[len("src/"):]))
	}
	if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/MainActivity;", deJavac)

	comparaMetodos(t, nuestra, suya)

	// el implementador es el que escribe javac: privado, sintético y con el descriptor del SAM
	m, ok := buscaMetodo(nuestra, "lambda$openAppDetailsDialog$0")
	if !ok {
		t.Fatalf("falta lambda$openAppDetailsDialog$0: %s", nombresDeMetodos(nuestra))
	}
	if m.Desc != "(Lshare/apps/util/Apk;)V" {
		t.Errorf("el descriptor del implementador es %s", m.Desc)
	}
	if m.Access != accPrivate|accSynthetic {
		t.Errorf("los flags del implementador son 0x%04x y javac escribe 0x1002", m.Access)
	}
	if len(nuestra.BootstrapMethods) != 1 {
		t.Fatalf("%d filas de BootstrapMethods, se esperaba una", len(nuestra.BootstrapMethods))
	}
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestLaLambdaDelFuenteSeDesugraYEjecuta: el `.class` que escribimos pasa por la expansión de lambdas
// del dexer (la misma que usan las clases de javac) y deja la sintética con su campo y su método SAM.
// Es lo que separa «el sitio se emitió» de «el sitio se entiende».
func TestLaLambdaDelFuenteSeDesugra(t *testing.T) {
	jar := jarDelSDK(t)
	nuestras, err := compilarProyectoJava(fuentesLambda, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la lambda: %v", err)
	}
	clases := make([]classFull, 0, len(nuestras))
	for _, c := range nuestras {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee con nuestro lector: %v", c.interno, err)
		}
		clases = append(clases, cf)
	}
	expandidas, err := expandirLambdas(clases)
	if err != nil {
		t.Fatalf("la expansión rechazó nuestro sitio: %v", err)
	}
	var sintetica classFull
	encontrada := false
	for _, c := range expandidas {
		if c.Name == "Lshare/apps/util/MainActivity$$Lambda$0;" {
			sintetica, encontrada = c, true
		}
	}
	if !encontrada {
		t.Fatalf("no se sintetizó la clase de la lambda: %v", nombresDeLambda(expandidas))
	}
	if len(sintetica.Fields) != 1 || sintetica.Fields[0].Desc != "Lshare/apps/util/MainActivity;" {
		t.Fatalf("campos de la sintética: %+v", sintetica.Fields)
	}
	sam, ok := buscaMetodo(sintetica, "accept")
	if !ok || sam.Desc != "(Ljava/lang/Object;)V" {
		t.Fatalf("el método SAM de la sintética: %+v", sam)
	}
	if _, err := dexDeClases(clases); err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
}

// TestLaLambdaConBloqueSeRechaza: el cuerpo de bloque no está medido y se dice con su nombre; sin esta
// guarda, `x -> { … }` se leería como una lambda cuyo cuerpo es un bloque y el error saldría lejos.
func TestLaLambdaConBloqueSeRechaza(t *testing.T) {
	fuente := `package p;

import java.util.function.Consumer;

class A {
    void f() {
        Consumer<A> c = x -> { f(); };
    }
}
`
	_, err := compilarProyectoJava([]fuenteJava{{nombre: "A.java", contenido: fuente}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("el emisor aceptó una lambda con cuerpo de bloque")
	}
	if !strings.Contains(err.Error(), "cuerpo de bloque") {
		t.Fatalf("el rechazo no nombra el bloque: %v", err)
	}
}

// nombresDeMetodos: la lista de métodos de una clase, para los fallos.
func nombresDeMetodos(cf classFull) string {
	out := make([]string, 0, len(cf.Methods))
	for _, m := range cf.Methods {
		out = append(out, m.Name+m.Desc)
	}
	return strings.Join(out, " ")
}
