//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_ldc_pool_largo_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_ldc_pool_largo_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// compilaLaSondaConJavac17: el `.class` de referencia de la sonda, escrito como `PoolLambda.java`
// (la clase es pública y javac exige el nombre del fichero).
func compilaLaSondaConJavac17(t *testing.T, fuente string) classFull {
	t.Helper()
	raiz := t.TempDir()
	pon(t, raiz, "PoolLambda.java", fuente)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac17Exigido(t), "--release", "8", "-g:lines,source", "-classpath", jarDelSDK(t),
		"-d", salida, filepath.Join(raiz, "PoolLambda.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó la sonda: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	for _, c := range deJavac {
		if c.Name == "Lp/PoolLambda;" {
			return c
		}
	}
	t.Fatalf("javac no dejó p/PoolLambda (%d clases)", len(deJavac))
	return classFull{}
}

// TestElPoolDeLaLambdaNoDesplazaElLdcContraJavac17: con el implementador aplazado, la constante que
// cae en el límite sale con la forma corta, como javac, y el dex entero coincide byte a byte.
func TestElPoolDeLaLambdaNoDesplazaElLdcContraJavac17(t *testing.T) {
	const k = 114
	fuente := fuentePoolLambda(k)
	nuestra := compilaLaSonda(t, fuente)
	suya := compilaLaSondaConJavac17(t, fuente)

	// la premisa del oráculo: javac escribe la forma corta para esa constante
	formaJavac, idxJavac := formaDelPrimerLdc(t, suya)
	if formaJavac != "ldc" || idxJavac > 255 {
		t.Fatalf("javac escribe %s con el índice %d en la sonda: la premisa no se cumple", formaJavac, idxJavac)
	}

	forma, idx := formaDelPrimerLdc(t, nuestra)
	if forma != "ldc" {
		t.Errorf("nuestro `limite` carga la constante con %q (índice %d) y javac con `ldc` (%d): el implementador de la lambda desplazó el pool", forma, idx, idxJavac)
	}
	if idx > 255 {
		t.Errorf("el índice %d no cabe en `ldc`: la regla del JVMS 6.5 se está saltando", idx)
	}

	mismoDex(t, nuestra, suya)
}

// TestElInterruptorRestauraLaFormaLarga es la falsificación: con el implementador emitido en el sitio
// —el defecto de antes— las mismas constantes se desplazan por encima de 255 y sale `ldc_w` donde
// javac escribe `ldc`. El `Cleanup` devuelve el interruptor a su sitio aunque el test falle a medias.
func TestElInterruptorRestauraLaFormaLarga(t *testing.T) {
	implementadorEnElSitio = true
	t.Cleanup(func() { implementadorEnElSitio = false })

	fuente := fuentePoolLambda(114)
	nuestra := compilaLaSonda(t, fuente)
	suya := compilaLaSondaConJavac17(t, fuente)
	formaJavac, _ := formaDelPrimerLdc(t, suya)
	forma, idx := formaDelPrimerLdc(t, nuestra)
	if formaJavac != "ldc" {
		t.Fatalf("la premisa de javac no se cumple: %q", formaJavac)
	}
	if forma != "ldc_w" {
		t.Fatalf("con el implementador en el sitio la constante sale con %q (índice %d): la falsificación no reproduce el defecto", forma, idx)
	}
	if idx <= 255 {
		t.Fatalf("`ldc_w` con el índice %d: la falsificación no está midiendo el límite", idx)
	}
}
