//go:build !js

package main

// La paridad con javac 17 del §202: los tres casos del alcance de los tipos, byte a byte en el dex
// entero y con las líneas y los `max_stack`/`locals` método a método. Bajo `GOOS=js` no hay proceso
// que lanzar, así que el fichero entero lleva `//go:build !js`; la mitad que solo necesita el emisor
// vive en `javaemisor_alcance_test.go`, sin tag. Estas pruebas **no se saltan**: sin javac 17 fallan
// (ver `dependencias_test.go:5-8`).

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestNewDeOtraClaseContraJavac17: el mismo proyecto que el caso del usuario, byte a byte contra javac
// 17 en el dex entero, y los `max_stack`/`locals` y las líneas método a método.
func TestNewDeOtraClaseContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "MainActivity.java", fuenteUsoLienzo)
	pon(t, raiz, "Lienzo.java", fuenteLienzo)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "MainActivity.java", contenido: fuenteUsoLienzo},
		{nombre: "Lienzo.java", contenido: fuenteLienzo},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "MainActivity.java"), filepath.Join(raiz, "Lienzo.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	for _, nombre := range []string{"Lcom/hola/apkcstudio/MainActivity;", "Lcom/hola/apkcstudio/Lienzo;"} {
		nuestra, suya := buscarClaseEmitida(t, nuestras, nombre, deJavac)
		compararMetodos(t, nuestra, suya)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// fuenteMiTipo es la clase que aporta el jar local, el papel de `android-support-v4.jar`.
const fuenteMiTipo = `package com.hola.dep;

public class MiTipo {
    public int valor() {
        return 7;
    }
}
`

// fuenteLienzoDeJar usa el tipo del jar en el constructor; el import vive solo aquí.
const fuenteLienzoDeJar = `package com.hola.apkcstudio;

import com.hola.dep.MiTipo;

public class Lienzo {
    private MiTipo tipo;

    public Lienzo(MiTipo tipo) {
        this.tipo = tipo;
    }
}
`

// fuenteUsoLienzoDeJar instancia `Lienzo` sin importar `com.hola.dep.MiTipo`, que es lo que el
// llamador no debe necesitar para resolver el constructor: le pasa `null` y el descriptor del
// parámetro se resuelve con el alcance de `Lienzo`.
const fuenteUsoLienzoDeJar = `package com.hola.apkcstudio;

public class Fabrica {
    public Lienzo crear() {
        return new Lienzo(null);
    }
}
`

// crearJarDeClases empaqueta en un `.jar` el árbol de `.class` que dejó javac en `dir`.
func crearJarDeClases(t *testing.T, dir, rutaJar string) {
	t.Helper()
	f, err := os.Create(rutaJar)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	err = filepath.Walk(dir, func(ruta string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(ruta) != ".class" {
			return err
		}
		rel, err := filepath.Rel(dir, ruta)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		if _, err := w.Write(datos); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestNewConTipoDeJar: el tipo del parámetro viene de `jar/`. Se resuelve con el classpath real del
// proyecto (`entradasDelClasspathEn`) y el dex tiene que coincidir byte a byte con javac 17, que recibe
// el mismo jar en su `-classpath`.
func TestNewConTipoDeJar(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "dep/MiTipo.java", fuenteMiTipo)
	pon(t, raiz, "src/Fabrica.java", fuenteUsoLienzoDeJar)
	pon(t, raiz, "src/Lienzo.java", fuenteLienzoDeJar)

	// primero el jar de la dependencia, compilado por javac como lo estaría cualquier apoyo
	depSalida := filepath.Join(raiz, "dep-out")
	cmd := exec.Command(javac, "--release", "8", "-d", depSalida, filepath.Join(raiz, "dep", "MiTipo.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 no compiló la dependencia: %v\n%s", err, out)
	}
	rutaJar := filepath.Join(raiz, "jar", "dependencia.jar")
	if err := os.MkdirAll(filepath.Dir(rutaJar), 0o755); err != nil {
		t.Fatal(err)
	}
	crearJarDeClases(t, depSalida, rutaJar)

	entradas, err := entradasDelClasspathEn(raiz, jar)
	if err != nil {
		t.Fatalf("armando el classpath: %v", err)
	}
	nuestras, err := compilarProyectoJavaConClasspath([]fuenteJava{
		{nombre: "Fabrica.java", contenido: fuenteUsoLienzoDeJar},
		{nombre: "Lienzo.java", contenido: fuenteLienzoDeJar},
	}, rutasDelClasspath(entradas))
	if err != nil {
		t.Fatalf("`new Lienzo(tipo)` con el tipo en jar/: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd = exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar+string(os.PathListSeparator)+rutaJar,
		"-d", salida, filepath.Join(raiz, "src", "Fabrica.java"), filepath.Join(raiz, "src", "Lienzo.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso del jar: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// fuenteBaseConImport y fuenteHijaSinImport cubren la otra mitad del mismo alcance: la cadena de
// padres. `Hija` hereda `ancho()` de `Base`, y `Base` extiende `View`, que solo está importado en el
// fichero de `Base`. El emisor sube a `Base` desde `Hija` y resuelve su padre con el alcance de `Base`.
const fuenteBaseConImport = `package com.hola.apkcstudio;

import android.content.Context;
import android.view.View;

public abstract class Base extends View {
    public Base(Context contexto) {
        super(contexto);
    }

    public int ancho() {
        return 0;
    }
}
`

const fuenteHijaSinImport = `package com.hola.apkcstudio;

public class Hija extends Base {
    public Hija() {
        super(null);
    }

    public int mide() {
        return ancho();
    }
}
`

// TestHerenciaDeOtraClaseSubePorSuPadre: `ancho()` se declara en `Base` y la cadena de padres de `Base`
// nombra `View`, que `Hija` no importa. Resolverla con el alcance de `Hija` era lo que fallaba.
func TestHerenciaDeOtraClaseSubePorSuPadre(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Base.java", fuenteBaseConImport)
	pon(t, raiz, "Hija.java", fuenteHijaSinImport)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Hija.java", contenido: fuenteHijaSinImport},
		{nombre: "Base.java", contenido: fuenteBaseConImport},
	}, jar)
	if err != nil {
		t.Fatalf("`ancho()` heredado de otra clase: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Base.java"), filepath.Join(raiz, "Hija.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
