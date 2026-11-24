package main

// §202: el alcance con el que se resuelven los tipos no es el del llamador. `new Lienzo(this)` desde
// una clase que no importa `android.content.Context` tiene que resolver el parámetro del constructor
// de `Lienzo`, que sí lo importa (o que lo trae de un jar de `jar/`). Antes se resolvía con el paquete
// y los imports del fichero que emitía la llamada y el tipo del otro fichero no se encontraba.
//
// Aquí vive la mitad que no necesita proceso externo —el emisor compila y la clase sale—, que es la
// que corre también bajo `GOOS=js`. La paridad byte a byte con javac 17 vive en
// `javaemisor_alcance_paridad_test.go` con `//go:build !js`, porque bajo wasm no hay proceso que
// lanzar; sus pruebas no se saltan (§5-§8 de `dependencias_test.go`).

import (
	"testing"
)

// fuenteLienzo declara el constructor con `Context`, que solo está importado aquí.
const fuenteLienzo = `package com.hola.apkcstudio;

import android.content.Context;

public class Lienzo {
    private Context contexto;

    public Lienzo(Context contexto) {
        this.contexto = contexto;
    }
}
`

// fuenteUsoLienzo lo instancia desde otra clase y **no** importa `android.content.Context`.
const fuenteUsoLienzo = `package com.hola.apkcstudio;

import android.app.Activity;
import android.os.Bundle;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Lienzo lienzo = new Lienzo(this);
    }
}
`

// TestNewDeOtraClaseResuelveSuConstructor es el caso del usuario: `new Lienzo(this)` no puede acusar
// `el tipo Context no se resuelve` porque el `Context` es del constructor de `Lienzo`, no del llamador.
func TestNewDeOtraClaseResuelveSuConstructor(t *testing.T) {
	jar := jarDelSDKExigido(t)
	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "MainActivity.java", contenido: fuenteUsoLienzo},
		{nombre: "Lienzo.java", contenido: fuenteLienzo},
	}, jar)
	if err != nil {
		t.Fatalf("`new Lienzo(this)` desde otra clase: %v", err)
	}
	if len(nuestras) != 2 {
		t.Fatalf("salieron %d clases, dos", len(nuestras))
	}
}

// buscarClaseEmitida devuelve la clase con ese descriptor releída de nuestros bytes y la de javac.
func buscarClaseEmitida(t *testing.T, nuestras []claseEmitida, descriptor string, deJavac []classFull) (classFull, classFull) {
	t.Helper()
	var nuestra classFull
	encontrada := false
	for _, c := range nuestras {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee: %v", c.interno, err)
		}
		if cf.Name == descriptor {
			nuestra, encontrada = cf, true
		}
	}
	if !encontrada {
		t.Fatalf("no emitimos %s (%s)", descriptor, volcadoDeClases(nuestras))
	}
	for _, cf := range deJavac {
		if cf.Name == descriptor {
			return nuestra, cf
		}
	}
	t.Fatalf("javac no dejó %s", descriptor)
	return classFull{}, classFull{}
}

// compararMetodos: `max_stack`/`locals` y las filas de la LineNumberTable, método a método.
func compararMetodos(t *testing.T, nuestra, suya classFull) {
	t.Helper()
	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el método %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			t.Errorf("%s%s: max_stack/locals %d/%d y javac %d/%d", esperado.Name, esperado.Desc,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount)
		}
		if len(encontrada.Lines) != len(esperado.Lines) {
			t.Errorf("%s%s: %d filas de línea y javac %d", esperado.Name, esperado.Desc,
				len(encontrada.Lines), len(esperado.Lines))
			continue
		}
		for i := range esperado.Lines {
			if encontrada.Lines[i] != esperado.Lines[i] {
				t.Errorf("%s%s: la fila %d es %+v y la de javac %+v", esperado.Name, esperado.Desc,
					i, encontrada.Lines[i], esperado.Lines[i])
			}
		}
	}
}

// compararDex: el oráculo estricto, el dex entero byte a byte.
func compararDex(t *testing.T, nuestras []claseEmitida, deJavac []classFull) error {
	t.Helper()
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
	return nil
}
