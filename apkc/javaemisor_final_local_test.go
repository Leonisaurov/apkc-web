package main

// §242: el `final` en una declaración local. El que abrió este frente fue la línea 137 de
// `MainActivity.java` del zip de ApkShare:
//
//	src/share/apps/util/MainActivity.java: linea 137: "final" no es una expresion que este parser reconozca
//
// `pareceDeclaracion` y el emisor ya sabían qué hacer con el modificador; lo que faltaba era que el
// guardia de `sentencia` lo aceptara como inicio de tipo y que `tipoConArgumentos` lo consumiera. El
// oráculo es javac 17 `--release 8 -g:lines,source`: la tabla de líneas, `max_stack`/`locals` y el dex
// entero byte a byte. `final` no deja huella en el `.class` sin `-g:vars`: no hay LocalVariableTable
// ni LocalVariableTypeTable, así que el descriptor y la `Signature` son los del tipo a secas.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteFinalLocal trae la línea 137 del zip —verbatim— más el `final` de un primitivo, el de un
// genérico con el diamante y el de un parámetro, que son las formas que el guardia tiene que dejar
// pasar. El inicializador del primitivo es `base + 1` y no un literal: un `final int fijo = 6` es una
// variable constante (JLS 4.12.4) y javac la pliega en sus usos —medido con `javap -c -l`: `bipush 6;
// ireturn`, sin la declaración—, un plegado que es otra frontera y que aquí no se mide.
const fuenteFinalLocal = `package com.hola.apkcstudio;

import java.util.ArrayList;
import java.util.List;

public class Finales {
    int suma(int base) {
        final int fijo = base + 1;
        return fijo;
    }

    List<Apk> lista() {
        final List<Apk> apks = new ArrayList<>();
        return apks;
    }

    int doble(final int n) {
        return n * 2;
    }

    void abre() {
        final Dialog dialog = new Dialog(this);
        dialog.show();
    }
}

class Apk {
}

class Dialog {
    Dialog(Object contexto) {
    }

    void show() {
    }
}
`

// TestFinalEnDeclaracionLocalContraJavac17: el `.class` de las tres declaraciones locales con `final`
// tiene que decir lo mismo que el de javac 17 en `max_stack`/`locals`, en la LineNumberTable y en el
// dex entero. Y ni el suyo ni el nuestro pueden llevar tabla de variables locales: con
// `-g:lines,source` javac no la escribe, así que el `final` no viaja al fichero.
func TestFinalEnDeclaracionLocalContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Finales.java", fuenteFinalLocal)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Finales.java", contenido: fuenteFinalLocal},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el `final` de una declaración local: %v", err)
	}
	emitida := porInterno(nuestras, "com/hola/apkcstudio/Finales")
	if emitida == nil {
		t.Fatalf("no salió Finales (%s)", nombresDe(nuestras))
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Finales.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	nuestra, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/Finales;", deJavac)

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}

	// el `final` no deja tabla de locales: javac la omitiría con `-g:vars`, y aquí se comprueba que
	// tampoco la inventa nadie en la ruta nueva
	deJavacCrudo, err := os.ReadFile(filepath.Join(salida, "com/hola/apkcstudio/Finales.class"))
	if err != nil {
		t.Fatalf("leyendo el .class de javac: %v", err)
	}
	for _, aguja := range []string{"LocalVariableTable", "LocalVariableTypeTable"} {
		if bytes.Contains(deJavacCrudo, []byte(aguja)) {
			t.Fatalf("el .class de javac lleva %s: el fuente no es el medido", aguja)
		}
		if bytes.Contains(emitida.datos, []byte(aguja)) {
			t.Errorf("nuestro .class lleva %s y el `final` de una local no lo escribe", aguja)
		}
	}
}

// fuenteMainActivityLinea137 arma un fuente con la línea 137 del zip, verbatim, en la línea 137: las
// de relleno son comentarios dentro del método. Con el `Dialog` de la misma unidad el caso compila
// entero cuando la ruta está encendida, así que la falsificación solo puede fallar por el `final`.
func fuenteMainActivityLinea137() string {
	lineas := []string{
		"package share.apps.util;",
		"",
		"public class MainActivity {",
		"    void abrir() {",
	}
	for len(lineas) < 136 {
		lineas = append(lineas, "        // relleno hasta la linea 137")
	}
	lineas = append(lineas,
		"        final Dialog dialog = new Dialog(this);",
		"    }",
		"}",
		"",
		"class Dialog {",
		"    Dialog(Object contexto) {",
		"    }",
		"}",
	)
	return strings.Join(lineas, "\n") + "\n"
}

// TestFinalLocalFalsificacion: con el interruptor apagado, la línea 137 del usuario tiene que volver a
// fallar con su mensaje literal, en la misma línea y con el prefijo del fichero que pone
// `compilarProyectoJava`. Encendido, el fuente entero compila: no queda ninguna frontera detrás.
func TestFinalLocalFalsificacion(t *testing.T) {
	const literal = "src/share/apps/util/MainActivity.java: linea 137: \"final\" no es una expresion que este parser reconozca"
	jar := jarDelSDK(t)
	fuente := fuenteMainActivityLinea137()
	if lineas := strings.Split(fuente, "\n"); !strings.Contains(lineas[136], "final Dialog dialog") {
		t.Fatalf("la línea 137 del fuente de prueba es %q y no la del zip", lineas[136])
	}
	fuentes := []fuenteJava{{nombre: "src/share/apps/util/MainActivity.java", contenido: fuente}}

	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con el `final` encendido el fuente no compila: %v", err)
	}

	anterior := leeFinalEnDeclaracion
	leeFinalEnDeclaracion = false
	t.Cleanup(func() { leeFinalEnDeclaracion = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if err.Error() != literal {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario %q", err, literal)
	}
	t.Logf("sin la ruta: %v", err)
}
