package main

// §228: la referencia cualificada en una expresión no es el primer segmento. `com.x.dos.Helper.doble(x)`
// se resolvía como el tipo `com` y el emisor se paraba con «el tipo com no se resuelve». El tipo es el
// prefijo más largo de la cadena que resuelve a una clase, y lo que queda detrás son campos o el nombre
// del método. El oráculo es javac 17 `--release 8 -g:lines,source` y el dex byte a byte, con dos clases
// del mismo nombre simple en paquetes distintos para que una resolución perezosa al primero se vea; esa
// mitad vive en `javaemisor_cualificado_paridad_test.go` con `//go:build !js`, porque bajo wasm no hay
// proceso que lanzar. Aquí queda la resolución del tipo, que no necesita javac.

import (
	"bytes"
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
