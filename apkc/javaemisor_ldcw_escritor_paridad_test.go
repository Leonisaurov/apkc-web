//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_ldcw_escritor_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_ldcw_escritor_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// compilaConJavac17: el `.class` de referencia de la sonda. Se compara después con el nuestro.
func compilaConJavac17(t *testing.T, raiz, fuente string) classFull {
	t.Helper()
	javac := javac17Exigido(t)
	jar := jarDelSDK(t)
	pon(t, raiz, "PoolGrande.java", fuente)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "PoolGrande.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó la sonda: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejó %d clases, una", len(deJavac))
	}
	return deJavac[0]
}

// TestElLdcWEscribeLaFormaLargaYCoincideConJavac17: el pool de la sonda pasa de 255 y nuestro `.class`
// tiene que resolverlo como javac: forma corta donde cabe y larga donde no, con el mismo dex byte a
// byte y las mismas pilas declaradas. Antes del arreglo esta compilación se rechazaba con el error del
// usuario y no salía ningún `.class`.
func TestElLdcWEscribeLaFormaLargaYCoincideConJavac17(t *testing.T) {
	fuente := fuentePoolGrande(200, 80)
	nuestra := nuestraClase(t, fuente)
	suya := compilaConJavac17(t, t.TempDir(), fuente)

	// premisa: la sonda de javac de verdad pasa de 255 y usa `ldc_w` para lo que no cabe.
	_, largasJavac := formasDelLdc(t, suya)
	if len(largasJavac) == 0 {
		t.Fatalf("el `.class` de javac no trae `ldc_w`: la sonda no pasó de 255 entradas")
	}
	for _, idx := range largasJavac {
		if idx <= 255 {
			t.Errorf("javac escribe `ldc_w` con el índice %d: la premisa de la sonda no se cumple", idx)
		}
	}

	cortas, largas := formasDelLdc(t, nuestra)
	if len(largas) == 0 {
		t.Fatalf("nuestro `.class` no trae `ldc_w` con el pool por encima de 255 (%d `ldc` cortos)", len(cortas))
	}
	for _, idx := range cortas {
		if idx > 255 {
			t.Errorf("`ldc` con el índice %d: no cabe en un byte y JVMS 6.5 pide `ldc_w`", idx)
		}
	}
	for _, idx := range largas {
		if idx <= 255 {
			t.Errorf("`ldc_w` con el índice %d: cabe en un byte y javac usa `ldc`", idx)
		}
		if idx > 0xffff {
			t.Errorf("`ldc_w` con el índice %d: no cabe en dos bytes", idx)
		}
	}

	// las pilas declaradas, método a método: la forma larga no cambia ni una palabra de la pila.
	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el método %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if !esperado.HasCode {
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			t.Errorf("%s: max_stack %d y locals %d, javac declara %d y %d", esperado.Name,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount)
		}
	}

	mismoDex(t, nuestra, suya)
}

// TestElPoolCortoSeQuedaEnLdc es el control: con menos de 256 entradas no hay ni una `ldc_w` y el
// `.class` sigue siendo el mismo de antes. Si el emisor escribiera la forma larga siempre, el dex
// seguiría coincidiendo y solo esta prueba lo vería.
func TestElPoolCortoSeQuedaEnLdc(t *testing.T) {
	fuente := fuentePoolGrande(3, 2)
	nuestra := nuestraClase(t, fuente)
	suya := compilaConJavac17(t, t.TempDir(), fuente)

	cortas, largas := formasDelLdc(t, nuestra)
	if len(cortas) == 0 {
		t.Fatalf("la sonda no cargó ni una constante corta: no mide nada")
	}
	if len(largas) != 0 {
		t.Errorf("con un pool corto salen %d `ldc_w` (%v)", len(largas), largas)
	}
	mismoDex(t, nuestra, suya)
}
