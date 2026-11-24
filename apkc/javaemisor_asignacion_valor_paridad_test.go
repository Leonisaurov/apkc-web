//go:build !js

package main

import "testing"

// La paridad con javac 17 de javaemisor_asignacion_valor_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_asignacion_valor_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestAsignacionComoValorContraJavac17: el bucle de la línea 187, con el `dup` del valor asignado y
// la comparación encima. Si el `dup` falta, el `istore` se lleva el único valor y el `if_icmple` se
// queda sin operando: el verificador lo diría, pero el dex de javac lo dice antes.
func TestAsignacionComoValorContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	for _, f := range fuentesCopiaMinima {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentesCopiaMinima, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la asignación como valor de la línea 187: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentesCopiaMinima)
	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/Copia;", deJavac)

	comparaMetodos(t, nuestra, suya)

	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestAsignacionAnidadaEnCondicionContraJavac17: el `a = b = c` de una condición, contra javac 17
// `--release 8 -g:lines,source`, con el dex entero byte a byte.
func TestAsignacionAnidadaEnCondicionContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Anidada.java", contenido: fuenteAnidada}}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la asignación anidada de la condición: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Anidada;", deJavac)

	comparaMetodos(t, nuestra, suya)

	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestAsignacionAnchaComoValorContraJavac17: el `dup2` del `long` y la comparación con `lcmp`, contra
// javac 17 `--release 8 -g:lines,source`, con el dex entero byte a byte.
func TestAsignacionAnchaComoValorContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Largo.java", contenido: fuenteAncha}}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la asignación ancha como valor: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	nuestra, suya := buscaLaClase(t, nuestras, "Lp/Largo;", deJavac)

	comparaMetodos(t, nuestra, suya)

	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestCopiaDelBloqueDeLaLinea187: el bloque entero del zip, con el `try`/`catch`. Aquí el dex no sale
// byte a byte por una diferencia anterior a este frente —la sección de datos mide 12 bytes más en
// nuestro lado, en el primer ítem de anotaciones—, así que se compara lo que sí es de la línea: las
// instrucciones, una a una, aceptando solo que los índices del pool no coincidan (son índices, no
// opcodes). Un control sin asignación en la condición mide los mismos 12 bytes de diferencia.
func TestCopiaDelBloqueDeLaLinea187(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	for _, f := range fuentesCopiaBloque {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentesCopiaBloque, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el bloque de la línea 187: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentesCopiaBloque)
	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/Copia;", deJavac)

	comparaMetodos(t, nuestra, suya)

	nuestroMetodo, ok := buscaMetodoDesc(nuestra, "copia", "(Ljava/io/File;Ljava/io/File;)V")
	if !ok {
		t.Fatalf("no salió copia(File,File)V: %s", listaDeMetodos(nuestra))
	}
	suMetodo, ok := buscaMetodoDesc(suya, "copia", "(Ljava/io/File;Ljava/io/File;)V")
	if !ok {
		t.Fatal("javac no dejó copia(File,File)V")
	}
	if err := comparaInstrucciones(nuestroMetodo.Code, suMetodo.Code); err != nil {
		t.Fatal(err)
	}
}
