package main

// Sonda de D2: el coste real de `apkc check` en nativo, con las tres mismas formas que mide la
// sonda de la pestaña (`studio/sonda-check.js`). No es una cifra transferida de §232 ni de I5: se
// mide aquí con reloj de pared sobre el árbol final y se registra en §329.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCheckTiempo mide la mediana de tres escenarios: el fuente suelto contra el android.jar, el
// proyecto recién creado sin R.java (se enlaza res/ en memoria) y el proyecto con la R.java que
// dejó un build (camino rápido que ve el usuario que ya compiló). La primera vuelta de cada escenario
// calienta el índice del jar y el marco de recursos; no entra en la mediana.
func TestCheckTiempo(t *testing.T) {
	jar := jarDelSDK(t)
	const repeticiones = 5

	medir := func(nombre string, caso func() (string, string)) time.Duration {
		t.Helper()
		var tiempos []time.Duration
		for i := 0; i < repeticiones+1; i++ {
			raiz, contenido := caso()
			comienzo := time.Now()
			r := revisarFuenteJava(contenido, "MainActivity.java", jar, raiz)
			d := time.Since(comienzo)
			if r.Tipo != "check" {
				t.Fatalf("%s: el check no devolvió un informe: %+v", nombre, r)
			}
			if i == 0 {
				continue // calienta cachés
			}
			tiempos = append(tiempos, d)
		}
		m := mediana(tiempos)
		t.Logf("medida %s %.1f", nombre, milisegundos(m))
		return m
	}

	// (1) sin --proyecto: solo el fuente contra el android.jar. Da un error (la R no viaja en el
	// jar), pero lo que se mide es el trabajo del parser y del emisor sin enlazar recursos.
	_, suelto := proyectoDeCheck(t)
	sinProyecto := medir("check_sin_proyecto_ms", func() (string, string) { return "", suelto })

	// (2) sin R.java: cada vuelta es un proyecto recién creado, sin build/, así que no hay índice
	// ni marco cacheados y `check` enlaza res/ y lee resources.arsc del android.jar.
	sinR := medir("check_sin_R_enlazando_res_ms", func() (string, string) {
		raiz, fuente := proyectoDeCheck(t)
		return raiz, fuente
	})

	// (3) con la R.java escrita y build/ presente, como después de un build de la pestaña: el
	// camino rápido usa la R del proyecto y el marco cacheado en build/marco.
	raizConR, fuenteConR := proyectoDeCheck(t)
	pon(t, raizConR, "src/com/hola/apkcstudio/R.java", fuenteRStudio)
	if err := os.MkdirAll(filepath.Join(raizConR, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	conR := medir("check_con_proyecto_y_R_ms", func() (string, string) { return raizConR, fuenteConR })
	if milisegundos(conR) >= 30 {
		t.Errorf("la mediana con proyecto y R es %.1f ms y el objetivo de D5 es <30 ms en nativo", milisegundos(conR))
	}
	// no se exige que el camino con R gane al fuente suelto: con 5 repeticiones y el escritorio
	// bajo carga, diez milisegundos de ruido pueden invertirlos; lo falsable aquí es el objetivo
	// de 30 ms, no una comparación entre dos medianas tan cercanas.
	t.Logf("resumen: sin proyecto %.1f ms; sin R enlazando res %.1f ms; con proyecto y R %.1f ms",
		milisegundos(sinProyecto), milisegundos(sinR), milisegundos(conR))
}
