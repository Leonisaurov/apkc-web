package main

// §201: la cláusula `extends`/`implements` con argumentos de tipo, medida contra javac 17. El
// argumento no cambia el descriptor —es el borrado (type erasure)— pero sí la `Signature` genérica, y
// el dex la lleva como `dalvik.annotation.Signature`: si no la escribiéramos, el dex de javac y el
// nuestro dejarían de coincidir byte a byte.

import (
	"os/exec"
	"path/filepath"
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

// TestClausulaGenericaContraJavac17: nuestro `.class` de `extends android.widget.ArrayAdapter<Apk>`
// tiene que decir lo mismo que el de javac 17 en el `super_class`, en las interfaces, en la `Signature`
// y en los `max_stack`/`locals` y las filas de línea de cada método, y el dex entero tiene que ser el
// mismo byte a byte. El `.class` no se compara entero porque el orden del constant pool difiere (está
// aceptado y escrito en `javaemisor_fuzz_test.go`).
func TestClausulaGenericaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteApkDelAdaptador)
	pon(t, raiz, "ApkAdapter.java", fuenteAdaptadorGenerico)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Apk.java", contenido: fuenteApkDelAdaptador},
		{nombre: "ApkAdapter.java", contenido: fuenteAdaptadorGenerico},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo `extends android.widget.ArrayAdapter<Apk>`: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"), filepath.Join(raiz, "ApkAdapter.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}

	nuestra, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/ApkAdapter;", deJavac)

	// el argumento `Apk` no está en el descriptor: el padre es el crudo, como lo escribe javac
	if nuestra.Super != suya.Super {
		t.Errorf("el super_class es %q y el de javac %q", nuestra.Super, suya.Super)
	}
	if quiero := "Landroid/widget/ArrayAdapter;"; nuestra.Super != quiero {
		t.Errorf("el super_class es %q y el crudo es %q", nuestra.Super, quiero)
	}
	if len(nuestra.Interfaces) != len(suya.Interfaces) {
		t.Errorf("%d interfaces y javac %d", len(nuestra.Interfaces), len(suya.Interfaces))
	}
	// la Signature sí lleva el argumento, y es lo que hace que el dex coincida
	if quiero := "Landroid/widget/ArrayAdapter<Lcom/hola/apkcstudio/Apk;>;"; suya.Signature != quiero {
		t.Fatalf("la Signature de javac es %q y se midió %q", suya.Signature, quiero)
	}
	if nuestra.Signature != suya.Signature {
		t.Errorf("la Signature es %q y la de javac %q", nuestra.Signature, suya.Signature)
	}
	comparaMetodos(t, nuestra, suya)

	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestCreadorGenericoContraJavac17: el `<>` en `implements` deja el `interfaces` con el nombre crudo
// (`android/os/Parcelable$Creator`) y la firma con el argumento. El dex de javac trae además las filas
// del InnerClasses del tipo anidado del jar, así que la comparación estricta se queda en el `.class`:
// el dex entero se mide en el caso del `extends`, que no toca clases anidadas.
func TestCreadorGenericoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Creador.java", fuenteCreadorGenerico)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Creador.java", contenido: fuenteCreadorGenerico},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo `implements Parcelable.Creator<Creador>`: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Creador.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}

	nuestra, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/Creador;", deJavac)
	if quiero := "Landroid/os/Parcelable$Creator;"; len(nuestra.Interfaces) != 1 || nuestra.Interfaces[0] != quiero {
		t.Errorf("las interfaces son %v y la cruda es %q", nuestra.Interfaces, quiero)
	}
	if quiero := "Ljava/lang/Object;Landroid/os/Parcelable$Creator<Lcom/hola/apkcstudio/Creador;>;"; suya.Signature != quiero {
		t.Fatalf("la Signature de javac es %q y se midió %q", suya.Signature, quiero)
	}
	if nuestra.Signature != suya.Signature {
		t.Errorf("la Signature es %q y la de javac %q", nuestra.Signature, suya.Signature)
	}
	comparaMetodos(t, nuestra, suya)
}

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
// se vería una instrucción de más o una fila movida por haber leído distinto la cláusula.
func comparaMetodos(t *testing.T, nuestra, suya classFull) {
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
			t.Errorf("falta el metodo %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			t.Errorf("%s%s: max_stack/locals %d/%d y javac %d/%d", esperado.Name, esperado.Desc,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount)
		}
		if len(encontrada.Lines) != len(esperado.Lines) {
			t.Errorf("%s%s: %d filas de linea y javac %d", esperado.Name, esperado.Desc,
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
