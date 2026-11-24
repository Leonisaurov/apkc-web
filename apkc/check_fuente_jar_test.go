package main

// Una clase que el proyecto redefina y que además viaje en un jar del classpath. La resolución del
// emisor prefiere la copia propia (`ctx.hay`), así que el diagnóstico daba por bueno un fuente que
// llamaba a un miembro que solo existía en la copia local: `emisor:true` con cero errores, mientras
// el build dexa la copia propia y el runtime resuelve contra el jar. La guarda contrasta los nombres
// registrados con el índice de cada artefacto del classpath antes de emitir nada.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteClaseDelJar es la clase que trae la dependencia, emitida con el propio emisor.
const fuenteClaseDelJar = `package dup;

public class Dup {
    public static int enJar() {
        return 1;
    }
}
`

// fuenteClaseRedefinida es la copia local de la misma clase, con un método que el jar no tiene.
const fuenteClaseRedefinida = `package dup;

public class Dup {
    public static int valorAudit() {
        return 2;
    }
}
`

// fuenteUsaValorAudit llama al método que solo existe en la copia local: es el verde falso.
const fuenteUsaValorAudit = `package com.hola.apkcstudio;

public class Uso {
    public int usa() {
        return dup.Dup.valorAudit();
    }
}
`

// TestCheckAbortaConUnFuenteQueRedefineUnaClaseDelJar cubre el cruce fuente↔jar con una dependencia
// montada aquí: sin la copia local, el fuente pedido no compila (el método no está en el jar); con
// ella, el proyecto redefiniría la clase y el check tiene que decirlo en vez de dar el pedido por
// comprobado.
func TestCheckAbortaConUnFuenteQueRedefineUnaClaseDelJar(t *testing.T) {
	jar := jarDelSDK(t)
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarDePrueba(t, filepath.Join(raiz, "jar", "a.jar"), clasesDeFuente(t, jar, "src/dup/Dup.java", fuenteClaseDelJar))

	// el control: sin la copia local, el método del jar no existe y el diagnóstico ya lo decía
	r := revisarFuenteJava(fuenteUsaValorAudit, "Uso.java", jar, raiz)
	if len(r.Errores) != 1 || r.Emisor {
		t.Fatalf("sin la copia local el fuente no debía compilar: %+v", r)
	}

	// con la copia local, el emisor resolvía contra ella y salía en verde: ahora es un fallo de
	// registro que nombra la clase y el jar
	pon(t, raiz, "src/dup/Dup.java", fuenteClaseRedefinida)
	r = revisarFuenteJava(fuenteUsaValorAudit, "Uso.java", jar, raiz)
	if r.Emisor {
		t.Fatalf("se dio por comprobado un proyecto que redefine dup.Dup del jar: emisor=%v aviso=%q errores=%+v", r.Emisor, r.Aviso, r.Errores)
	}
	if len(r.Errores) != 0 {
		t.Fatalf("el fallo del proyecto se coló como error del pedido: %+v", r.Errores)
	}
	for _, quiero := range []string{"dup.Dup", "a.jar", "src/dup/Dup.java"} {
		if !strings.Contains(r.Aviso, quiero) {
			t.Errorf("el aviso no nombra %q: %q", quiero, r.Aviso)
		}
	}
}

// jarRealDeApkShare localiza el jar real que usó el crítico. No se versiona —son 648 KB de un
// binario ajeno— y no se descarga: se toma del entorno o de la ruta en la que ya está. Sin él la
// prueba se salta, y el cruce fuente↔jar sigue cubierto por la de arriba con una dependencia hecha
// aquí.
func jarRealDeApkShare(t *testing.T) string {
	t.Helper()
	candidatos := []string{os.Getenv("APKC_JAR_REAL")}
	if home, err := os.UserHomeDir(); err == nil {
		candidatos = append(candidatos, filepath.Join(home, "Downloads", "ApkShare-main", "jar", "android-support-v4.jar"))
	}
	for _, ruta := range candidatos {
		if ruta == "" {
			continue
		}
		if _, err := os.Stat(ruta); err == nil {
			return ruta
		}
	}
	t.Skipf("sin el jar real de ApkShare: exporta APKC_JAR_REAL o deja el fichero en ~/Downloads/ApkShare-main/jar/android-support-v4.jar (probadas: %s)",
		strings.Join(candidatos, ", "))
	return ""
}

// TestCheckAbortaConUnFuenteQueRedefineUnaClaseDelJarReal es el fixture del crítico, con el jar de
// verdad: `android-support-v4.jar` trae `android.support.v4.text.TextUtilsCompat`, el proyecto lo
// vuelve a declarar y el fuente pedido llama a un método que solo tiene la copia local. Antes de la
// guarda el informe salía `emisor:true` con cero errores.
func TestCheckAbortaConUnFuenteQueRedefineUnaClaseDelJarReal(t *testing.T) {
	jarReal := jarRealDeApkShare(t)
	jar := jarDelSDK(t)
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	datos, err := os.ReadFile(jarReal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "jar", "android-support-v4.jar"), datos, 0o644); err != nil {
		t.Fatal(err)
	}
	pon(t, raiz, "src/android/support/v4/text/TextUtilsCompat.java", `package android.support.v4.text;

public class TextUtilsCompat {
    public static String valorAudit() {
        return "solo en la copia local";
    }
}
`)
	pedido := `package com.hola.apkcstudio;

public class Uso {
    public String usa() {
        return android.support.v4.text.TextUtilsCompat.valorAudit();
    }
}
`
	r := revisarFuenteJava(pedido, "Uso.java", jar, raiz)
	if r.Emisor {
		t.Fatalf("se dio por comprobado el fixture del crítico: emisor=%v aviso=%q errores=%+v", r.Emisor, r.Aviso, r.Errores)
	}
	for _, quiero := range []string{"android.support.v4.text.TextUtilsCompat", "android-support-v4.jar"} {
		if !strings.Contains(r.Aviso, quiero) {
			t.Errorf("el aviso no nombra %q: %q", quiero, r.Aviso)
		}
	}
}
