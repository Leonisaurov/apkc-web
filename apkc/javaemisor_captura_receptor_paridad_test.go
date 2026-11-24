//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_captura_receptor_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_captura_receptor_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestLaCapturaDeLocalEnReceptorSeEscribeComoJavac: `apps.getAdapter()` dentro de la anónima se lee
// como `this.val$apps.getAdapter()`, con el tipo de la local en el campo `val$apps` y en el constructor
// de la anónima, y el cuerpo del `onItemClick` sale instrucción a instrucción igual al de javac.
func TestLaCapturaDeLocalEnReceptorSeEscribeComoJavac(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptor)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la captura de una local en posición de receptor: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCapturaReceptor)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// el campo `val$apps`, del tipo de la local capturada, y las instrucciones que la usan
	nuestraAnonima, suyaAnonima := buscaLaClase(t, nuestras, "Lp/Sonda$1;", deJavac)
	comparaMetodos(t, nuestraAnonima, suyaAnonima)
	nuestroOnItem := metodoDeClaseEmitida(t, nuestras, "p/Sonda$1", "onItemClick", "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V")
	suOnItem := metodoDeClaseLeida(t, deJavac, "Lp/Sonda$1;", "onItemClick", "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V")
	if err := comparaInstrucciones(nuestroOnItem.Code, suOnItem.Code); err != nil {
		t.Fatalf("onItemClick no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// y fuera de la anónima la misma llamada sigue sin captura: es el caso que no toca esta ruta
	if _, err := compilarProyectoJava(fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptorSinAnonima), jar); err != nil {
		t.Fatalf("la misma llamada sin anónima dejó de compilar: %v", err)
	}

	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("%v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestElConstructorDeLaAnonimaNoLlevaMethodParameters: el constructor de la anónima coincide con el de
// javac 17.0.21 en los dos atributos que importan: ninguno lleva `MethodParameters` —javac no lo pone en
// ninguna anónima; el emisor lo escribía por la medición del párrafo 42, hecha con javac 21— y los dos
// llevan `Signature: ()V` cuando el constructor recibe capturas de locales. Fija la aserción que la
// comparación del dex entero ya hace, para que un cambio futuro diga cuál de los dos atributos rompió.
func TestElConstructorDeLaAnonimaNoLlevaMethodParameters(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptor)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCapturaReceptor)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestraAnonima, suyaAnonima := buscaLaClase(t, nuestras, "Lp/Sonda$1;", deJavac)
	nuestro, okNuestro := buscaMetodo(nuestraAnonima, "<init>")
	suyo, okSuyo := buscaMetodo(suyaAnonima, "<init>")
	if !okNuestro || !okSuyo {
		t.Fatalf("sin constructor en la anónima: nuestro %v, javac %v", okNuestro, okSuyo)
	}
	if nuestro.Parameters || nuestro.Signature != "()V" {
		t.Errorf("nuestro constructor no es el de javac 17 (sin MethodParameters y con Signature: ()V): Parameters=%v Signature=%q",
			nuestro.Parameters, nuestro.Signature)
	}
	if suyo.Parameters || suyo.Signature != "()V" {
		t.Errorf("javac 17 ya no deja Signature: ()V en el constructor de la anónima: Parameters=%v Signature=%q", suyo.Parameters, suyo.Signature)
	}
}
