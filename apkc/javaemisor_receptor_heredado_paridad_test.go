//go:build !js

package main

// La paridad con javac 17 de la sobrecarga exterior a la anónima: el `onClick` instrucción a
// instrucción y el dex byte a byte, con el descriptor `usa:(Ljava/lang/String;)V` que elige javac.
// Bajo `GOOS=js` no hay proceso que lanzar, así que el fichero lleva `//go:build !js`; el caso que solo
// mira lo emitido vive en `javaemisor_receptor_heredado_test.go`, sin tag. Esta prueba **no se salta**:
// sin javac 17 falla (ver `dependencias_test.go:5-8`).

import (
	"testing"
)

// TestSobrecargaExteriorContraJavac17: con dos sobrecargas de igual aridad en la clase exterior, el
// estimador resuelve por el tipo de los argumentos —las mismas fases estricta→laxa que la emisión
// (`elegirSobrecarga`)— y el `onClick` sale instrucción a instrucción y en dex byte a byte como el de
// javac. La sobrecarga elegida se comprueba además en el descriptor: javac escribe
// `invokevirtual p/Pantalla.usa:(Ljava/lang/String;)V`, no la de `int`.
func TestSobrecargaExteriorContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	fuentes := fuentesReceptorHeredado(fuenteSobrecargaExterior)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `usa(f(\"x\"))` con dos sobrecargas de igual aridad en la clase exterior: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Pantalla.java", fuenteSobrecargaExterior)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestraAnonima, suyaAnonima := buscaLaClase(t, nuestras, "Lp/Pantalla$1;", deJavac)
	comparaMetodos(t, nuestraAnonima, suyaAnonima)
	nuestroOnClick := metodoDeClaseEmitida(t, nuestras, "p/Pantalla$1", "onClick", "(Landroid/view/View;)V")
	suOnClick := metodoDeClaseLeida(t, deJavac, "Lp/Pantalla$1;", "onClick", "(Landroid/view/View;)V")
	if err := comparaInstrucciones(nuestroOnClick.Code, suOnClick.Code); err != nil {
		t.Fatalf("Pantalla$1.onClick no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("el dex de la sobrecarga exterior no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
}
