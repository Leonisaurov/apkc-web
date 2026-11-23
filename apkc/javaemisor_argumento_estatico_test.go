package main

// §200: el acceso estatico encadenado como argumento (`setContentView(R.layout.main)`) byte a byte
// contra javac 17. El oraculo es el de `compilar_test.go`: las dos compilaciones se dexan con el mismo
// dexer y los dos dex tienen que ser identicos, byte a byte. Aqui javac corre en caliente porque el
// caso es nuevo y no habia fixture; si no hay un javac 17 en la maquina, el test se salta y lo dice.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteArgumentoEstatico es el caso del usuario, con las dos formas que produce un acceso de varios
// segmentos: una constante de `R.layout` como argumento de una llamada heredada y una de `R.id` como
// argumento de un constructor.
const fuenteArgumentoEstatico = `package com.hola.apkcstudio;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView t = new TextView(this);
        t.setId(R.id.pulsar);
    }
}
`

// buscarJavac17 devuelve el javac 17 de la máquina, o vacío si no hay. Es el compilador de referencia
// de este caso: el que se midio para el subconjunto.
func buscarJavac17() string {
	for _, c := range []string{os.Getenv("JAVAC17"), homeDir() + "/opt/jdk17/bin/javac", "javac"} {
		if c == "" {
			continue
		}
		ruta, err := exec.LookPath(c)
		if err != nil {
			continue
		}
		salida, err := exec.Command(ruta, "-version").CombinedOutput()
		if err != nil || !strings.Contains(string(salida), "javac 17") {
			continue
		}
		return ruta
	}
	return ""
}

func javac17(t *testing.T) string {
	t.Helper()
	if ruta := buscarJavac17(); ruta != "" {
		return ruta
	}
	t.Skip("sin javac 17: no hay con que comparar el acceso estatico encadenado")
	return ""
}

// TestAccesoEstaticoEncadenadoContraJavac17: nuestro dex de MainActivity y R, compilados por nosotros,
// y el de los mismos fuentes compilados por javac 17 con `--release 8`, tienen que coincidir byte a
// byte. Si el tipo de `R.layout.main` se estimara mal, la constante no se doblaria igual y la
// secuencia de instrucciones dejaria de coincidir.
func TestAccesoEstaticoEncadenadoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "MainActivity.java", fuenteArgumentoEstatico)
	pon(t, raiz, "R.java", fuenteRStudio)

	// lo nuestro: el emisor desde los mismos dos fuentes
	fuentes := []fuenteJava{
		{nombre: "MainActivity.java", contenido: fuenteArgumentoEstatico},
		{nombre: "R.java", contenido: fuenteRStudio},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo el caso: %v", err)
	}
	clases := make([]classFull, 0, len(nuestras))
	for _, c := range nuestras {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee: %v", c.interno, err)
		}
		clases = append(clases, cf)
	}
	dexNuestro, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}

	// lo de javac 17, con las mismas opciones con las que se midio el resto del subconjunto: version 52
	// y las lineas, que es lo unico que el emisor escribe en el ClassFile
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "MainActivity.java"), filepath.Join(raiz, "R.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	dexJavac, err := dexDeClases(deJavac)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}

	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s\n%s",
				i, dexNuestro[i], dexJavac[i], volcadoDeClases(nuestras), nombresDe(nuestras))
		}
	}
}

// volcadoDeClases: la lista de clases que emitimos, para que un fallo diga por donde iba.
func volcadoDeClases(cs []claseEmitida) string {
	var sb strings.Builder
	for _, c := range cs {
		sb.WriteString("  " + c.interno + "\n")
	}
	return sb.String()
}
