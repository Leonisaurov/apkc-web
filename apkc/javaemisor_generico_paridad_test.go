//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_generico_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_generico_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestClausulaGenericaContraJavac17: nuestro `.class` de `extends android.widget.ArrayAdapter<Apk>`
// tiene que decir lo mismo que el de javac 17 en el `super_class`, en las interfaces, en la `Signature`
// y en los `max_stack`/`locals` y las filas de línea de cada método, y el dex entero tiene que ser el
// mismo byte a byte. El `.class` no se compara entero porque el orden del constant pool difiere (está
// aceptado y escrito en `javaemisor_fuzz_test.go`).
func TestClausulaGenericaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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
	javac := javac17Exigido(t)
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
