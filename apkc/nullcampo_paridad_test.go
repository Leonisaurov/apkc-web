//go:build !js

// La paridad con javac 17 de nullcampo_test.go, que se mueve entero: todas sus pruebas son oráculo de javac,
// que se lanza como proceso, y el hospedador de la capa wasm no crea procesos. Por eso el fichero
// lleva `//go:build !js` y no se salta en silencio bajo `GOOS=js`. No hay SKIP: sin javac 17
// fallan (ver `dependencias_test.go:5-8`).

package main

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAsignacionNulaAUnCampoContraJavac17: `icon = null;` con icon de tipo Drawable. La ruta de campo
// usaba `exp`, que tipa el null como Object; debe usar `expA` con el descriptor del campo, como ya
// hacía la asignación a una variable local. El dex tiene que salir byte a byte el de javac 17.
func TestAsignacionNulaAUnCampoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteNuloEnCampo)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteNuloEnCampo}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `icon = null;`: %v", err)
	}
	nuestra, err := readClassFull(nuestras[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	dexNuestro, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
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
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s", i, dexNuestro[i], dexJavac[i], volcadoDeClases(nuestras))
		}
	}
}

// fuenteNuloEnCampoAnonima: una anónima escribe null en un campo privado del exterior. Es la ruta
// `access$NN2` de `javaemisor.go:5924`, que también pasó de `exp` a `expA` y no tenía prueba.
const fuenteNuloEnCampoAnonima = `package com.hola.apkcstudio;

import android.graphics.drawable.Drawable;

public class Apk {
    private Drawable icon;

    public void limpia() {
        Runnable r = new Runnable() {
            public void run() {
                icon = null;
            }
        };
        r.run();
    }
}
`

// TestAsignacionNulaEnCampoDelExteriorContraJavac17 cubre la ruta del accesor sintético con un null.
func TestAsignacionNulaEnCampoDelExteriorContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteNuloEnCampoAnonima)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteNuloEnCampoAnonima}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la anónima: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(nuestras) != len(deJavac) {
		t.Fatalf("salieron %d clases y javac %d", len(nuestras), len(deJavac))
	}
	nuestasFull := make([]classFull, 0, len(nuestras))
	for _, c := range nuestras {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("clase nuestra no legible: %v", err)
		}
		nuestasFull = append(nuestasFull, cf)
	}
	sort.Slice(nuestasFull, func(i, j int) bool { return nuestasFull[i].Name < nuestasFull[j].Name })
	sort.Slice(deJavac, func(i, j int) bool { return deJavac[i].Name < deJavac[j].Name })
	// El dex entero no es comparable aquí: el constructor sintético de la anónima lleva otro
	// descriptor (diferencia previa documentada en §95), no tiene que ver con este arreglo. Lo que
	// sí se exige es que `run()` escriba el mismo bytecode y que el accesor `access$002` exista.
	metodoDe := func(clases []classFull, nombre, metodo string) classMethod {
		for _, c := range clases {
			if c.Name != nombre {
				continue
			}
			for _, m := range c.Methods {
				if m.Name == metodo {
					return m
				}
			}
		}
		return classMethod{}
	}
	runNuestro := metodoDe(nuestasFull, "Lcom/hola/apkcstudio/Apk$1;", "run")
	runJavac := metodoDe(deJavac, "Lcom/hola/apkcstudio/Apk$1;", "run")
	if !runNuestro.HasCode || !runJavac.HasCode {
		t.Fatalf("run() sin código: nuestro=%v javac=%v", runNuestro.HasCode, runJavac.HasCode)
	}
	// Los índices del constant pool cambian entre emisores; se enmascaran los operandos u2 de
	// `getfield` (0xb4) e `invokestatic` (0xb8) y se comparan los opcodes y la pila.
	sinIndices := func(code []byte) []byte {
		out := append([]byte(nil), code...)
		for i := 0; i+2 < len(out); i++ {
			if out[i] == 0xb4 || out[i] == 0xb8 {
				out[i+1], out[i+2] = 0, 0
				i += 2
			}
		}
		return out
	}
	nuestroSin, javacSin := sinIndices(runNuestro.Code), sinIndices(runJavac.Code)
	if string(nuestroSin) != string(javacSin) {
		t.Fatalf("run() difiere: nuestro %x, javac %x", nuestroSin, javacSin)
	}
	accesor := false
	for _, c := range nuestasFull {
		if c.Name != "Lcom/hola/apkcstudio/Apk;" {
			continue
		}
		for _, m := range c.Methods {
			if strings.HasPrefix(m.Name, "access$00") {
				accesor = true
			}
		}
	}
	if !accesor {
		t.Fatal("la clase exterior no declaró el accesor access$00N para escribir el campo")
	}
}
