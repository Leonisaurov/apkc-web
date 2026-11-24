package main

// Dos fuentes del proyecto que declaran la misma clase. El emisor registra todas las clases antes de
// emitir, y el mapa de clases propias se quedaba con la última: al emitir el fuente pedido sus
// nombres se resolvían contra la clase del otro fichero. En el caso suave —las dos clases válidas—
// el check salía en verde con `emisor:true`; en el otro, un fallo de emisión del segundo fichero
// también marcaba el pedido como comprobado, por el `etapa == "emisor"` de `check.go`. Ni javac ni
// el build aceptan la doble definición.

import (
	"strings"
	"testing"
)

// fuentePedidoSimple es el pedido sin herencia: su emisión no depende de resolver su propio nombre,
// así que la clase duplicada del otro fichero no lo hace fallar por sí sola.
const fuentePedidoSimple = `package com.hola.apkcstudio;

public class MainActivity {
    public int suma(int a, int b) {
        return a + b;
    }
}
`

// TestCheckRechazaDosFuentesConLaMismaClase fija el caso suave: las dos definiciones son válidas y
// el proyecto no compila igual, porque javac rechaza la clase repetida y el build escribiría dos
// veces el mismo nombre. El pedido no tiene errores propios, pero no se da por comprobado.
func TestCheckRechazaDosFuentesConLaMismaClase(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/MainActivity.java", fuentePedidoSimple)
	pon(t, raiz, "src/com/hola/apkcstudio/Copia.java", fuentePedidoSimple)

	r := revisarFuenteJava(fuentePedidoSimple, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el fallo del proyecto se coló como error del pedido: %+v", r.Errores)
	}
	if r.Emisor {
		t.Fatal("se dio por comprobado un proyecto que declara dos veces MainActivity")
	}
	for _, quiero := range []string{"com.hola.apkcstudio.MainActivity", "MainActivity.java", "Copia.java"} {
		if !strings.Contains(r.Aviso, quiero) {
			t.Errorf("el aviso no nombra %q: %q", quiero, r.Aviso)
		}
	}
}

// TestCheckLaClaseDuplicadaNoSeMarcaComprobadaAunqueElOtroFicheroFalle cubre la variante que el
// crítico marcó como incierta: la segunda definición de la clase falla al emitirse. Antes, ese fallo
// de emisión ponía `emisor:true` —«el pedido se emitió antes»— aunque el proyecto tuviera la clase
// repetida.
func TestCheckLaClaseDuplicadaNoSeMarcaComprobadaAunqueElOtroFicheroFalle(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/MainActivity.java", fuentePedidoSimple)
	pon(t, raiz, "src/com/hola/apkcstudio/Copia.java", `package com.hola.apkcstudio;

public class MainActivity {
    public static int copia() {
        return NoExiste.x();
    }
}
`)

	r := revisarFuenteJava(fuentePedidoSimple, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el fallo de emisión de Copia.java se coló en el pedido: %+v", r.Errores)
	}
	if r.Emisor {
		t.Fatalf("se dio por comprobado un proyecto con dos MainActivity: aviso %q", r.Aviso)
	}
	if !strings.Contains(r.Aviso, "MainActivity") || !strings.Contains(r.Aviso, "Copia.java") {
		t.Fatalf("el aviso no nombra la clase y el fichero que la repite: %q", r.Aviso)
	}
}
