//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_molde_argumento_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_molde_argumento_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestMoldeDeArgumentoContraJavac17: el argumento con molde se estima y se emite como javac 17. La
// estimación tiene que coincidir con el `check-cast` que escribe `exp`, así que el dex de la sonda
// entera —sobrecarga elegida incluida— sale byte a byte igual al de javac.
func TestMoldeDeArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesSondaMolde()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el molde como argumento: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaMolde)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Sonda;", deJavac)
	nuestroCrea := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "crea", "(I)V")
	suCrea := metodoDeClaseLeida(t, deJavac, "Lp/Sonda;", "crea", "(I)V")
	if err := comparaInstrucciones(nuestroCrea.Code, suCrea.Code); err != nil {
		t.Fatalf("crea(I)V no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}
	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
