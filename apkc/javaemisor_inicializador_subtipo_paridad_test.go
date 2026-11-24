//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_inicializador_subtipo_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_inicializador_subtipo_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestElInicializadorDeCampoAceptaUnSubtipoContraJavac17: el campo `List<App>` con `new ArrayList<>()`
// compila, y la clase entera —el `<init>` con su LineNumberTable y el `putfield` al descriptor del
// campo— dexa igual que la de javac 17. Sin la subida, el emisor ni siquiera llega aquí: rechaza el
// fuente con el error literal del informe.
func TestElInicializadorDeCampoAceptaUnSubtipoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, fuentesInicializadorSubtipo[0].nombre[len("src/"):], fuenteInicializadorSubtipo)

	nuestras, err := compilarProyectoJava(fuentesInicializadorSubtipo, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el inicializador con un subtipo: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentesInicializadorSubtipo)
	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Pantalla;", deJavac)

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestElInicializadorDeCampoExactoSigueAceptandose: el control que ya pasaba. El descriptor del valor
// es el del campo, así que la ruta no cambia; se compara igual contra javac para que la puerta nueva
// no haya movido un byte del `<init>`.
func TestElInicializadorDeCampoExactoSigueAceptandose(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Exacta.java", contenido: fuenteInicializadorExacto}}
	raiz := t.TempDir()
	pon(t, raiz, fuentes[0].nombre[len("src/"):], fuenteInicializadorExacto)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el inicializador del tipo exacto: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Exacta;", deJavac)

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestElInicializadorDeCampoAceptaElLiteralNulo: la tercera pieza de la política. El hueco es `List`,
// lo que hay en la pila es `aconst_null` y javac no escribe nada entre los dos; el dex de la clase
// tiene que coincidir byte a byte, con el `putfield` al descriptor del campo.
func TestElInicializadorDeCampoAceptaElLiteralNulo(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Nula.java", contenido: fuenteInicializadorNulo}}
	raiz := t.TempDir()
	pon(t, raiz, fuentes[0].nombre[len("src/"):], fuenteInicializadorNulo)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el inicializador con `null`: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Nula;", deJavac)

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
