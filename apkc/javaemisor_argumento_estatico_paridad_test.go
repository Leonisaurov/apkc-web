//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_argumento_estatico_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_argumento_estatico_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestAccesoEstaticoEncadenadoContraJavac17: nuestro dex de MainActivity y R, compilados por nosotros,
// y el de los mismos fuentes compilados por javac 17 con `--release 8`, tienen que coincidir byte a
// byte. Si el tipo de `R.layout.main` se estimara mal, la constante no se doblaria igual y la
// secuencia de instrucciones dejaria de coincidir.
func TestAccesoEstaticoEncadenadoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
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

// TestAsignacionCualificadaContraJavac17: `this.filePath = filePath` tiene que salir del emisor como el
// `.class` de javac 17 y dar el mismo dex. El receptor `this` se emite con el `aload_0` que javac pone
// delante del valor, y la linea de la sentencia sigue marcandose al empezar el `jasign` —antes del
// receptor—, que es donde javac la pone: la `LineNumberTable` no se mueve.
//
// El `.class` no se compara byte a byte porque javac ordena el pool por insercion de simbolos y este
// emisor por declaracion; esa diferencia esta aceptada y escrita en `javaemisor_fuzz_test.go`. Lo que si
// se compara, ademas del dex entero (el oraculo estricto de siempre), es el `Code` y la `LineNumberTable`
// metodo a metodo: ahi se veria cualquier instruccion de mas o una fila movida.
func TestAsignacionCualificadaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteAsignacionCualificada)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteAsignacionCualificada}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo `this.filePath = filePath`: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "com/hola/apkcstudio/Apk" {
		t.Fatalf("salieron %d clases (%s), y javac deja una", len(nuestras), nombresDe(nuestras))
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
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejo %d clases, una", len(deJavac))
	}
	suya := deJavac[0]

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
		// el Code no se compara byte a byte porque los dos bytes de cada indice del pool cambian con el
		// orden del pool; las instrucciones y sus referencias las normaliza el dex, que se compara entero
		// mas abajo. La LineNumberTable no depende del pool, asi que aqui si va fila a fila.
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

// TestAsignacionPorReceptorContraJavac17: `appInfo.sourceDir = filePath` —un receptor que es un
// parámetro, no `this`— tiene que salir del emisor como el `.class` de javac 17 y dar el mismo dex byte
// a byte. Se comparan además el `Code` y la `LineNumberTable` método a método: si el receptor se
// marcara con su propia fila o si la pila contara una palabra de más, se vería ahí.
func TestAsignacionPorReceptorContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteAsignacionReceptor)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteAsignacionReceptor}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo `appInfo.sourceDir = filePath`: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "com/hola/apkcstudio/Apk" {
		t.Fatalf("salieron %d clases (%s), y javac deja una", len(nuestras), nombresDe(nuestras))
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
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejo %d clases, una", len(deJavac))
	}
	suya := deJavac[0]

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
