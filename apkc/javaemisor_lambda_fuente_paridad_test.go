//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// La paridad con javac 17 de javaemisor_lambda_fuente_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_lambda_fuente_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestLambdaDeUnParametroContraJavac17: el sitio de la línea 156, con el implementador y el
// `invokedynamic` que escribe javac.
func TestLambdaDeUnParametroContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	t.Logf("dex de la clase de la lambda: %d B (javac %d B)",
		longitudDelDex(t, clasesLeidas(t, nuestras)), longitudDelDex(t, deJavac))
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestDosSobrecargasNoCompartenImplementadorDeLambda: el nombre del implementador es único en la
// clase, no dentro del método. Sin eso, dos sobrecargas del mismo nombre emiten dos
// `lambda$f$0(Ljava/lang/Object;)V` y el classfile es inválido (el crítico lo destapó).
func TestDosSobrecargasNoCompartenImplementadorDeLambda(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "X.java", fuenteSobrecargasLambda)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "X.java", contenido: fuenteSobrecargasLambda}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó las dos lambdas: %v", err)
	}
	emitida := porInterno(nuestras, "p/X")
	if emitida == nil {
		t.Fatalf("no salió p/X (%s)", nombresDe(nuestras))
	}
	nuestra := leer(t, emitida)

	// ni un nombre+descriptor repetido: es la condición que un ClassFile no puede incumplir
	vistos := map[string]int{}
	for _, m := range nuestra.Methods {
		vistos[m.Name+m.Desc]++
	}
	for firma, n := range vistos {
		if n > 1 {
			t.Errorf("metodo %s %d veces", firma, n)
		}
	}
	implantadores := []string{}
	for _, m := range nuestra.Methods {
		if strings.HasPrefix(m.Name, "lambda$") {
			implantadores = append(implantadores, m.Name)
		}
	}
	if len(implantadores) != 2 {
		t.Fatalf("implementadores de lambda: %v (se esperaban 2)", implantadores)
	}
	for _, nombre := range []string{"lambda$f$0", "lambda$f$1"} {
		if _, ok := buscaMetodo(nuestra, nombre); !ok {
			t.Errorf("falta %s (%v)", nombre, implantadores)
		}
	}

	// y son los de javac: mismo juego de metodos y el dex entero
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-d", salida, filepath.Join(raiz, "X.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	deJavacX := porNombre(t, deJavac, "Lp/X;")
	comparaMetodos(t, nuestra, deJavacX)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
