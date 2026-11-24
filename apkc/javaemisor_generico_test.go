package main

// §201: la cláusula `extends`/`implements` con argumentos de tipo, medida contra javac 17. El
// argumento no cambia el descriptor —es el borrado (type erasure)— pero sí la `Signature` genérica, y
// el dex la lleva como `dalvik.annotation.Signature`: si no la escribiéramos, el dex de javac y el
// nuestro dejarían de coincidir byte a byte.

import (
	"fmt"
	"strings"
	"testing"
)

// fuenteApkDelAdaptador es la clase que viaja como argumento de tipo. Va en su propio fuente, como en
// el proyecto del usuario.
const fuenteApkDelAdaptador = `package com.hola.apkcstudio;

public class Apk {
    private String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }

    public String getRuta() {
        return ruta;
    }
}
`

// fuenteAdaptadorGenerico es el fuente que abrió este caso, con el tipo cualificado y el constructor
// heredado que el usuario tenía: `super(contexto, 0)` resuelve contra `ArrayAdapter(Context, int)`.
const fuenteAdaptadorGenerico = `package com.hola.apkcstudio;

public class ApkAdapter extends android.widget.ArrayAdapter<Apk> {
    public ApkAdapter() {
        super(null, 0);
    }
}
`

// fuenteCreadorGenerico: el `<>` en `implements`, sobre la interface genérica del android.jar.
const fuenteCreadorGenerico = `package com.hola.apkcstudio;

import android.os.Parcelable;

public abstract class Creador implements Parcelable.Creator<Creador> {
}
`

// buscaLaClase devuelve la clase de ese descriptor en las dos compilaciones, releyendo la nuestra de
// los bytes que emitió.
func buscaLaClase(t *testing.T, nuestras []claseEmitida, descriptor string, deJavac []classFull) (classFull, classFull) {
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
	t.Fatalf("javac no dejo %s", descriptor)
	return classFull{}, classFull{}
}

// comparaMetodos: los `max_stack`/`locals` y las filas de la LineNumberTable, método a método. Es donde
// se vería una instrucción de más o una fila movida por haber leído distinto la cláusula. Y también
// avisa de un método que nosotros escribimos y javac no: un implementador de lambda de más, un
// `access$` inventado o un sintético que no toca, que solo iterando los de javac se quedaban callados.
func comparaMetodos(t *testing.T, nuestra, suya classFull) {
	t.Helper()
	for _, problema := range problemasDeMetodos(nuestra, suya) {
		t.Error(problema)
	}
}

// problemasDeMetodos: la comparación de `comparaMetodos` como lista de textos, para poder falsificarla
// sin un `*testing.T` de verdad (ver TestComparaMetodosVeUnMetodoDeMas).
func problemasDeMetodos(nuestra, suya classFull) []string {
	problemas := []string{}
	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			problemas = append(problemas, fmt.Sprintf("falta el metodo %s%s", esperado.Name, esperado.Desc))
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			problemas = append(problemas, fmt.Sprintf("%s%s: max_stack/locals %d/%d y javac %d/%d", esperado.Name, esperado.Desc,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount))
		}
		if len(encontrada.Lines) != len(esperado.Lines) {
			problemas = append(problemas, fmt.Sprintf("%s%s: %d filas de linea y javac %d", esperado.Name, esperado.Desc,
				len(encontrada.Lines), len(esperado.Lines)))
			continue
		}
		for i := range esperado.Lines {
			if encontrada.Lines[i] != esperado.Lines[i] {
				problemas = append(problemas, fmt.Sprintf("%s%s: la fila %d es %+v y la de javac %+v", esperado.Name, esperado.Desc,
					i, encontrada.Lines[i], esperado.Lines[i]))
			}
		}
	}
	// la vuelta: lo que está en el nuestro y no en el de javac
	for _, propia := range nuestra.Methods {
		enJavac := false
		for _, esperado := range suya.Methods {
			if propia.Name == esperado.Name && propia.Desc == esperado.Desc {
				enJavac = true
				break
			}
		}
		if !enJavac {
			problemas = append(problemas, fmt.Sprintf("sobra el metodo %s%s, que javac no escribe", propia.Name, propia.Desc))
		}
	}
	return problemas
}

// TestComparaMetodosVeUnMetodoDeMas es la falsificación de la vuelta de arriba: quitarle un método al
// de javac tiene que salir como «sobra», no quedarse en silencio. Es el fallo que dejó pasar dos
// implementadores de lambda con el mismo nombre y descriptor.
func TestComparaMetodosVeUnMetodoDeMas(t *testing.T) {
	jar := jarDelSDK(t)
	nuestras, err := compilarProyectoJava(fuentesLambda, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la lambda: %v", err)
	}
	nuestra := leer(t, porInterno(nuestras, "share/apps/util/MainActivity"))
	// el de javac, con un método menos: el implementador que sí escribimos los dos
	suya := nuestra
	suya.Methods = nil
	for _, m := range nuestra.Methods {
		if m.Name == "lambda$openAppDetailsDialog$0" {
			continue
		}
		suya.Methods = append(suya.Methods, m)
	}
	problemas := problemasDeMetodos(nuestra, suya)
	falta := false
	for _, p := range problemas {
		if strings.Contains(p, "sobra el metodo lambda$openAppDetailsDialog$0") {
			falta = true
		}
	}
	if !falta {
		t.Fatalf("sin la vuelta, el método de más se queda callado: %v", problemas)
	}
	// y con la clase de javac entera no hay ningún problema
	suya.Methods = nuestra.Methods
	if p := problemasDeMetodos(nuestra, suya); len(p) != 0 {
		t.Fatalf("la comparación inventa problemas con la misma clase: %v", p)
	}
}

// comparaDex: el oráculo estricto de siempre, el dex entero byte a byte.
func comparaDex(t *testing.T, nuestras []claseEmitida, deJavac []classFull) error {
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
