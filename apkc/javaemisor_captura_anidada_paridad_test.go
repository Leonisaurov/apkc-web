//go:build !js

package main

import (
	"strings"
	"testing"
)

// La paridad con javac 17 de javaemisor_captura_anidada_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_captura_anidada_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestLaCapturaDeLocalAtraviesaLaAnonimaAnidada: `adapter` dentro de la anonima interior se lee por
// `this$1` y el `val$adapter` de la exterior, con el tipo real de la local; `esconde()` sube los dos
// saltos hasta la Activity; y todo el dex coincide con el de javac 17 salvo el constructor de la
// anonima, que ya tenia sus dos divergencias fijadas.
func TestLaCapturaDeLocalAtraviesaLaAnonimaAnidada(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesSondaCapturaAnidada(fuenteSondaCapturaAnidada)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo la captura a traves de la anonima anidada: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCapturaAnidada)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// el nombre de la anonima interior es el de javac: Sonda$1$1
	if !strings.Contains(nombresDe(nuestras), "p/Sonda$1$1") {
		t.Fatalf("no se emitio p/Sonda$1$1: %s", nombresDe(nuestras))
	}

	nuestraInterior, suyaInterior := buscaLaClase(t, nuestras, "Lp/Sonda$1$1;", deJavac)
	nuestraExterior, suyaExterior := buscaLaClase(t, nuestras, "Lp/Sonda$1;", deJavac)

	// el campo sintetico de la interior es `this$1` —no `this$0`— y del tipo de la exterior; y no
	// lleva un `val$adapter` propio, que es lo que distingue la cadena de javac de una captura directa
	if f := campoDe(nuestraInterior, "this$1"); f.Desc != "Lp/Sonda$1;" {
		t.Errorf("la anonima interior no guarda su exterior en this$1:Lp/Sonda$1;: %+v (campos %v)",
			f, nuestraInterior.Fields)
	}
	if f := campoDe(nuestraInterior, "this$0"); f.Name != "" {
		t.Errorf("la anonima interior se quedo con this$0: %+v", f)
	}
	if f := campoDe(nuestraInterior, "val$adapter"); f.Name != "" {
		t.Errorf("la anonima interior capturo la local por su cuenta: %+v", f)
	}

	// y la exterior si la tiene, con el tipo real de la local
	if f := campoDe(nuestraExterior, "val$adapter"); f.Desc != "Lp/Adaptador;" {
		t.Errorf("la anonima exterior no tiene val$adapter:Lp/Adaptador;: %+v (campos %v)", f, nuestraExterior.Fields)
	}

	// el cuerpo que usa la local, instruccion a instruccion contra javac
	nuestroRun := metodoDeClaseEmitida(t, nuestras, "p/Sonda$1$1", "run", "()V")
	suRun := metodoDeClaseLeida(t, deJavac, "Lp/Sonda$1$1;", "run", "()V")
	if err := comparaInstrucciones(nuestroRun.Code, suRun.Code); err != nil {
		t.Fatalf("el run() de la anonima interior no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// los metodos y las LineNumberTable de las dos anonimas y de la clase
	comparaMetodos(t, nuestraInterior, suyaInterior)
	comparaMetodos(t, nuestraExterior, suyaExterior)

	// el dex entero, con el atributo del constructor de la anonima fuera
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("%v\n%s", err, volcadoDeClases(nuestras))
	}
}
