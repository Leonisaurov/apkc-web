package main

// §228: la referencia cualificada en una expresión no es el primer segmento. `com.x.dos.Helper.doble(x)`
// se resolvía como el tipo `com` y el emisor se paraba con «el tipo com no se resuelve». El tipo es el
// prefijo más largo de la cadena que resuelve a una clase, y lo que queda detrás son campos o el nombre
// del método. El oráculo es javac 17 `--release 8 -g:lines,source` y el dex byte a byte, con dos clases
// del mismo nombre simple en paquetes distintos para que una resolución perezosa al primero se vea.

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// fuenteHelperCualificadoUno es el homónimo que NO se llama: si la referencia cualificada se quedara en
// el primer nombre simple, este `doble` (x * 2) es el que saldría.
const fuenteHelperCualificadoUno = `package com.x.uno;

public class Helper {
    public static int doble(int x) {
        return x * 2;
    }
}
`

// fuenteHelperCualificadoDos es el que nombra la referencia entera: `com.x.dos.Helper`.
const fuenteHelperCualificadoDos = `package com.x.dos;

public class Helper {
    public static int VALOR;

    public static int doble(int x) {
        return x + 100;
    }
}
`

// fuenteMainCualificado usa el tipo por su nombre completo en una llamada y en la lectura de un campo
// estático: los dos caminos que resolvían `segs[0]` y se paraban en `com`.
const fuenteMainCualificado = `package com.x.app;

public class Main {
    public static int usa(int x) {
        return com.x.dos.Helper.doble(x);
    }

    public static int lee() {
        return com.x.dos.Helper.VALOR;
    }
}
`

// TestEmisorReferenciaCualificada resuelve el tipo completo y no el primer segmento.
func TestEmisorReferenciaCualificada(t *testing.T) {
	jar := jarDelSDKExigido(t)
	fuentes := []fuenteJava{
		{nombre: "src/com/x/app/Main.java", contenido: fuenteMainCualificado},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperCualificadoDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperCualificadoUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("`com.x.dos.Helper.doble(x)` desde otro paquete: %v", err)
	}
	main := porInterno(nuestras, "com/x/app/Main")
	if main == nil {
		t.Fatalf("no salió Main: %s", nombresDe(nuestras))
	}
	if !bytes.Contains(main.datos, []byte("com/x/dos/Helper")) {
		t.Errorf("la referencia cualificada no apunta a com/x/dos/Helper: %s", volcadoDeClases(nuestras))
	}
	if bytes.Contains(main.datos, []byte("com/x/uno/Helper")) {
		t.Errorf("la referencia cualificada cayó en el homónimo com/x/uno/Helper: %s", volcadoDeClases(nuestras))
	}
}

// TestEmisorReferenciaCualificadaContraJavac17 fija la forma: los métodos y el dex entero, byte a byte.
func TestEmisorReferenciaCualificadaContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{
		{nombre: "src/com/x/app/Main.java", contenido: fuenteMainCualificado},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperCualificadoDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperCualificadoUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "src/com/x/app/Main.java", fuenteMainCualificado)
	pon(t, raiz, "src/com/x/dos/Helper.java", fuenteHelperCualificadoDos)
	pon(t, raiz, "src/com/x/uno/Helper.java", fuenteHelperCualificadoUno)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "src/com/x/app/Main.java"),
		filepath.Join(raiz, "src/com/x/dos/Helper.java"),
		filepath.Join(raiz, "src/com/x/uno/Helper.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	for _, nombre := range []string{"Lcom/x/app/Main;", "Lcom/x/dos/Helper;", "Lcom/x/uno/Helper;"} {
		nuestra, suya := buscarClaseEmitida(t, nuestras, nombre, deJavac)
		compararMetodos(t, nuestra, suya)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
