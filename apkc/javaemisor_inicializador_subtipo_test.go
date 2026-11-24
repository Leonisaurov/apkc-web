package main

// El inicializador de campo con un subtipo. El que abrió este frente fue el error literal del informe
// sobre `private List<App> lista = new ArrayList<>();`:
//
//	linea 10: el campo lista es Ljava/util/List; y su inicializador deja Ljava/util/ArrayList;
//
// La ruta del inicializador comparaba el descriptor del valor con el del campo y exigía identidad,
// mientras la escritura de campo (`x.lista = new ArrayList<>()`) ya mediaba la subida de referencias
// con `cabeEn`. La política es la de JLS 5.2 —identidad, subida de referencias y el literal `null`—,
// y javac no escribe ninguna conversión: medido con javac 17 `--release 8 -g:lines,source`, el
// constructor de `private List<App> lista = new ArrayList<>();` queda
// `aload_0; new ArrayList; dup; invokespecial; putfield lista:Ljava/util/List;` sin `checkcast`.
// El oráculo es ese mismo javac: el dex de la clase, byte a byte.

import (
	"strings"
	"testing"
)

// fuenteInicializadorSubtipo: el hallazgo, reducido a lo mínimo que lo dispara, con la línea del campo
// en la 10 para que el error literal de la falsificación sea el del informe.
const fuenteInicializadorSubtipo = `package p;

import java.util.ArrayList;
import java.util.List;

class App {
}

class Pantalla {
    private List<App> lista = new ArrayList<>();

    List<App> expone() {
        return lista;
    }
}
`

// fuenteInicializadorExacto: el control del tipo exacto, que ya compilaba y tiene que seguir igual: el
// descriptor del valor es el del campo y `cabeEn` ni entra en la subida.
const fuenteInicializadorExacto = `package p;

import java.util.ArrayList;

class Exacta {
    private ArrayList<String> cosas = new ArrayList<String>();

    ArrayList<String> todas() {
        return cosas;
    }
}
`

// fuenteInicializadorIncompatible: el control del inicializador que no cabe. `String` no es `List`, y
// javac 17 lo rechaza con `incompatible types: String cannot be converted to List<String>`; el emisor
// tiene que seguir dándolo por error, no aceptarlo por la puerta nueva.
const fuenteInicializadorIncompatible = `package p;

import java.util.List;

class Mala {
    private List<String> lista = "texto";
}
`

// fuenteInicializadorNulo: la tercera pieza de la política, el literal `null`. `exp` lo devuelve como
// `Object` porque en la pila es un `aconst_null` sin tipo propio; el hueco del campo es `List` y javac
// no escribe nada entre los dos (`aconst_null; putfield`), así que el inicializador tiene que aceptarlo
// sin compararlo por descriptor.
const fuenteInicializadorNulo = `package p;

import java.util.List;

class Nula {
    private List<String> lista = null;

    List<String> todas() {
        return lista;
    }
}
`

var fuentesInicializadorSubtipo = []fuenteJava{
	{nombre: "src/p/Pantalla.java", contenido: fuenteInicializadorSubtipo},
}

// TestElInicializadorDeCampoIncompatibleSigueRechazandose: el otro control. `String` no cabe en
// `List`, javac lo rechaza, y el emisor tiene que seguir rechazándolo con el mismo mensaje. Si la
// puerta nueva aceptara cualquier referencia, este test se pondría verde sin querer.
func TestElInicializadorDeCampoIncompatibleSigueRechazandose(t *testing.T) {
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/p/Mala.java", contenido: fuenteInicializadorIncompatible}}

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatalf("el emisor aceptó un `String` como inicializador de un `List`")
	}
	esperado := "el campo lista es Ljava/util/List; y su inicializador deja Ljava/lang/String;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el rechazo no es el esperado\n  quiero: %s\n  tengo:  %v", esperado, err)
	}
}

// TestFalsificacionDeLaSubidaEnElInicializador: apagado `cabeSubidaDeReferencia` —el mismo interruptor
// que falsifica la escritura de campo— la ruta vuelve a exigir el descriptor exacto y devuelve el
// error literal del informe. Encendido, compila. Es la prueba de que la coincidencia de arriba es de
// la subida y no de otra cosa.
func TestFalsificacionDeLaSubidaEnElInicializador(t *testing.T) {
	jar := jarDelSDK(t)

	anterior := cabeSubidaDeReferencia
	cabeSubidaDeReferencia = false
	defer func() { cabeSubidaDeReferencia = anterior }()
	_, err := compilarProyectoJava(fuentesInicializadorSubtipo, jar)
	cabeSubidaDeReferencia = anterior

	if err == nil {
		t.Fatalf("con la subida apagada el emisor aceptó el inicializador: la ruta no la usa")
	}
	t.Logf("con la subida apagada el emisor dice: %v", err)
	esperado := "linea 10: el campo lista es Ljava/util/List; y su inicializador deja Ljava/util/ArrayList;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el error con la ruta apagada no es el del informe\n  quiero: %s\n  tengo:  %s", esperado, err.Error())
	}
	if _, err := compilarProyectoJava(fuentesInicializadorSubtipo, jar); err != nil {
		t.Fatalf("encendida la subida el emisor sigue rechazando: %v", err)
	}
}
