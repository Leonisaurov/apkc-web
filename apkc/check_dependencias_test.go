package main

// Las dependencias del classpath y `check`: el diagnóstico armaba el classpath con el primero de
// `jar/` por orden léxico cuando dos dependencias definían la misma clase, mientras `build` aborta
// en `dependenciasDexeables` con «la clase X está definida más de una vez». Con `check` en verde y el
// build en rojo, la pestaña enseñaba que el proyecto compilaba y el APK no salía. La guarda es la
// misma función (`claseDuplicadaEntreJars`) y ahora se llama desde los dos sitios.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteDependenciaDup es la clase que las dos dependencias definen, emitida con el propio emisor
// para que el classpath la resuelva de verdad si la guarda no está.
const fuenteDependenciaDup = `package dup;

public class Dup {
    public static int valor() {
        return 7;
    }
}
`

// fuenteDependenciaDupSinValor es la otra copia de la misma clase, sin el método: si el diagnóstico
// resolviera contra `b.jar` en vez de contra `a.jar` el fuente pedido no compilaría, así que el
// verde de la falsificación dice además que el orden léxico sigue siendo el que elige.
const fuenteDependenciaDupSinValor = `package dup;

public class Dup {
}
`

// fuenteUsaDup es el fuente pedido: nombra la clase ambigua y no se compila sin el classpath.
const fuenteUsaDup = `package com.hola.apkcstudio;

public class Uso {
    public int usa() {
        return dup.Dup.valor();
    }
}
`

// TestCheckAbortaConDosJarsQueDefinenLaMismaClase es el hallazgo: `check` tiene que rechazar la
// misma colisión que `build`, nombrando la clase y los dos artefactos, en vez de resolver al
// primero por orden léxico. El control es el mismo proyecto con un solo jar: ahí no hay colisión y
// el fuente compila, así que el error no puede venir de una dependencia ilegible.
func TestCheckAbortaConDosJarsQueDefinenLaMismaClase(t *testing.T) {
	jar := jarDelSDK(t)
	conValor := clasesDeFuente(t, jar, "src/dup/Dup.java", fuenteDependenciaDup)
	sinValor := clasesDeFuente(t, jar, "src/dup/Dup.java", fuenteDependenciaDupSinValor)
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "a.jar"), conValor)

	// con una sola dependencia el diagnóstico sigue: la guarda no estorba al caso normal
	r := revisarFuenteJava(fuenteUsaDup, "Uso.java", jar, raiz)
	if len(r.Errores) != 0 || !r.Emisor {
		t.Fatalf("con una sola dependencia el fuente no compiló: %+v", r)
	}

	jarDePrueba(t, filepath.Join(raiz, "jar", "b.jar"), sinValor)

	// la guarda del classpath nombra la clase y los dos artefactos, como la del build
	if _, err := entradasDelClasspathEn(raiz, jar); err == nil {
		t.Error("el classpath aceptó dos jars que definen dup.Dup")
	} else {
		for _, quiero := range []string{"dup.Dup", "a.jar", "b.jar"} {
			if !strings.Contains(err.Error(), quiero) {
				t.Errorf("la colisión no nombra %q: %v", quiero, err)
			}
		}
	}

	// y `check` la reporta como error del diagnóstico, sin darse por comprobado
	r = revisarFuenteJava(fuenteUsaDup, "Uso.java", jar, raiz)
	t.Logf("check con la colisión: emisor=%v errores=%+v", r.Emisor, r.Errores)
	if len(r.Errores) != 1 {
		t.Errorf("%d errores con la colisión, medido uno: %+v", len(r.Errores), r.Errores)
	}
	if r.Emisor {
		t.Error("se dio por comprobado un fuente cuyo classpath tiene una clase duplicada")
	}
	for _, quiero := range []string{"dup.Dup", "a.jar", "b.jar"} {
		if len(r.Errores) == 1 && !strings.Contains(r.Errores[0].Mensaje, quiero) {
			t.Errorf("el error del check no nombra %q: %q", quiero, r.Errores[0].Mensaje)
		}
	}
}

// TestCheckAbortaConAarYJarQueDefinenLaMismaClase cubre la otra entrada del classpath: el
// `classes.jar` extraído de un AAR es una dependencia dexable más, así que un AAR y un jar que
// definan la misma clase tienen que recibir la misma guarda. El AAR se monta aquí con un
// `classes.jar` real, sin descargar nada.
func TestCheckAbortaConAarYJarQueDefinenLaMismaClase(t *testing.T) {
	jar := jarDelSDK(t)
	clases := clasesDeFuente(t, jar, "src/dup/Dup.java", fuenteDependenciaDup)
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "a.jar"), clases)

	// el AAR de la prueba: un zip con su Manifest y el classes.jar que define dup.Dup
	classesJar := filepath.Join(raiz, "classes.jar")
	jarDePrueba(t, classesJar, clases)
	bytesClasses, err := os.ReadFile(classesJar)
	if err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "lib.aar"), map[string][]byte{
		"AndroidManifest.xml": []byte("<manifest package=\"dup\"/>\n"),
		"classes.jar":         bytesClasses,
	})

	// `classesJarDelAar` escribe su extracción en `build/aar/` del directorio de trabajo
	t.Chdir(raiz)
	_, err = entradasDelClasspathEn(raiz, jar)
	if err == nil {
		t.Fatal("el classpath aceptó un AAR y un jar que definen dup.Dup")
	}
	// el AAR se nombra por su `classes.jar` extraído, que es lo que se abre y lo que nombra el
	// build: la extracción lleva la huella del AAR, así que el diagnóstico sigue siendo rastreable
	for _, quiero := range []string{"dup.Dup", "a.jar", filepath.Join("build", "aar")} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("la colisión no nombra %q: %v", quiero, err)
		}
	}
}
