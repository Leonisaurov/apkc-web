package main

// La prueba de aceptación de E3 del plan de emulación: el estado de la app sobrevive a una
// recreación (una rotación) y a la preferencia local. El APK se arma en memoria con los escritores
// del fork y el dex sale del emisor del fork, como el resto de las pruebas de la vista previa
// (regla 2: sin binarios versionados).

import (
	"encoding/json"
	"strings"
	"testing"
)

// fuenteContadorConEstado es un contador que guarda su valor en `onSaveInstanceState`, lo restaura
// en `onCreate` y escribe el mismo valor en un SharedPreferences propio. Imprime lo que lee de la
// preferencia para que la prueba pueda distinguir «la escribió» de «la leyó al recrear».
const fuenteContadorConEstado = `package com.hola.contador;

import android.app.*;
import android.content.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        if (savedInstanceState != null) {
            pulsos = savedInstanceState.getInt("pulsos");
        }
        SharedPreferences prefs = getSharedPreferences("cuenta", 0);
        int guardado = prefs.getInt("pulsos", 0);
        System.out.println("pref: " + guardado);
        total.setText("Pulsos: " + pulsos);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total.setText("Pulsos: " + pulsos);
                getSharedPreferences("cuenta", 0).edit().putInt("pulsos", pulsos).apply();
            }
        });
    }

    @Override
    public void onSaveInstanceState(Bundle outState) {
        super.onSaveInstanceState(outState);
        outState.putInt("pulsos", pulsos);
    }
}
`

// enteroGuardado lleva a int64 un valor del Bundle o de las preferencias, ya sea el int32 que guarda
// el intérprete o el float64 que deja el JSON.
func enteroGuardado(t *testing.T, v any) int64 {
	t.Helper()
	switch x := v.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	}
	t.Fatalf("el valor guardado es %T y se esperaba un entero", v)
	return 0
}

// TestElPreviewConservaElEstadoAlRecrear es el criterio de aceptación de E3: tras dos clics y una
// recreación el texto sigue en «Pulsos: 2», la preferencia escrita antes de recrear se lee después,
// y el Bundle y el SharedPreferences salen publicados en el JSON para reproducir la ejecución. La
// segunda parte comprueba que la recreación no reejecuta el `onCreate` de una actividad cubierta.
func TestElPreviewConservaElEstadoAlRecrear(t *testing.T) {
	apk := apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteContadorConEstado})

	ordenes := []ordenEjecucion{
		{Tipo: ordenPulsarTipo, Valor: "id/pulsar"},
		{Tipo: ordenPulsarTipo, Valor: "id/pulsar"},
		{Tipo: ordenRecrearTipo},
	}
	modelo, err := modeloVistaDeAPKConOrdenes(apk, "", true, ordenes)
	if err != nil {
		t.Fatalf("el apk con estado no se ejecuta: %v", err)
	}
	e := modelo.Ejecucion
	if e == nil || e.Error != "" {
		t.Fatalf("la ejecución falló: %+v", e)
	}
	for _, aviso := range e.Avisos {
		if strings.Contains(aviso, "no está en el dex ni en los stubs") {
			t.Errorf("la recreación llamó a algo fuera del subconjunto: %s", aviso)
		}
	}

	// el texto de la pantalla es el del estado restaurado, no el «Pulsos: 0» del layout
	if modelo.Raiz == nil || len(modelo.Raiz.Hijos) == 0 {
		t.Fatalf("el modelo no trae la pantalla: %+v", modelo)
	}
	if got := modelo.Raiz.Hijos[0].TextoDinamico; got != "Pulsos: 2" {
		t.Errorf("tras dos clics y una recreación el texto es %q y se esperaba «Pulsos: 2»; "+
			"capturas=%+v avisos=%v", got, e.Capturas, e.Avisos)
	}

	// el Bundle que recibió la instancia recreada y el SharedPreferences quedan publicados
	if got := enteroGuardado(t, e.Bundle["pulsos"]); got != 2 {
		t.Errorf("el Bundle guardado tiene pulsos=%d y se esperaba 2: %+v", got, e.Bundle)
	}
	if got := enteroGuardado(t, e.Preferencias["cuenta"]["pulsos"]); got != 2 {
		t.Errorf("la preferencia cuenta/pulsos es %d y se esperaba 2: %+v", got, e.Preferencias)
	}
	consola := []string{}
	for _, l := range e.Consola {
		consola = append(consola, l.Texto)
	}
	unida := strings.Join(consola, "\n")
	if !strings.Contains(unida, "pref: 0") || !strings.Contains(unida, "pref: 2") {
		t.Errorf("la app no leyó la preferencia antes y después de recrear: %v", consola)
	}

	// la secuencia de la recreación: el estado se guarda entre la instancia vieja y la nueva. Esta
	// actividad no declara onStart/onResume, así que el ciclo solo anota lo que existe en el dex.
	esperado := []string{
		"com.hola.contador.MainActivity.onCreate",
		"com.hola.contador.MainActivity.onSaveInstanceState",
		"com.hola.contador.MainActivity.onCreate",
	}
	if strings.Join(e.Ciclo, "\n") != strings.Join(esperado, "\n") {
		t.Errorf("el ciclo de la recreación no es el de una rotación:\n%v", e.Ciclo)
	}
	if len(e.Pantallas) != 2 || e.Pantallas[0] != "com.hola.contador.MainActivity" ||
		e.Pantallas[1] != "com.hola.contador.MainActivity" {
		t.Errorf("el recorrido de la recreación es %v", e.Pantallas)
	}

	// la publicación es reproducible: la misma ejecución deja el mismo JSON
	otra, err := modeloVistaDeAPKConOrdenes(apk, "", true, ordenes)
	if err != nil {
		t.Fatalf("la segunda ejecución falló: %v", err)
	}
	primeraJSON, err := json.Marshal(modelo)
	if err != nil {
		t.Fatalf("el modelo no se serializa: %v", err)
	}
	segundaJSON, err := json.Marshal(otra)
	if err != nil {
		t.Fatalf("la segunda ejecución no se serializa: %v", err)
	}
	if string(primeraJSON) != string(segundaJSON) {
		t.Errorf("dos ejecuciones iguales no dan el mismo JSON:\n%s\n%s", primeraJSON, segundaJSON)
	}

	// la máquina recreada está viva: un clic después de recrear sigue desde 2
	conOtroClic := append(append([]ordenEjecucion{}, ordenes...),
		ordenEjecucion{Tipo: ordenPulsarTipo, Valor: "id/pulsar"})
	tercero, err := modeloVistaDeAPKConOrdenes(apk, "", true, conOtroClic)
	if err != nil {
		t.Fatalf("el clic posterior a la recreación falló: %v", err)
	}
	if got := tercero.Raiz.Hijos[0].TextoDinamico; got != "Pulsos: 3" {
		t.Errorf("tras recrear y pulsar el texto es %q y se esperaba «Pulsos: 3»", got)
	}

	// la recreación no vuelve a crear las actividades cubiertas: solo la que está delante
	navega := apkDelNavega(t)
	dos, err := modeloVistaDeAPKConOrdenes(navega, "", true, []ordenEjecucion{
		{Tipo: ordenPulsarTipo, Valor: "id/abrir"},
		{Tipo: ordenRecrearTipo},
	})
	if err != nil {
		t.Fatalf("la recreación de la segunda pantalla falló: %v", err)
	}
	if dos.Ejecucion == nil {
		t.Fatal("la recreación de la segunda pantalla no dejó ejecución")
	}
	if dos.Layout != "res/layout/segunda.xml" || dos.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("tras recrear la pantalla es %q / %q", dos.Layout, dos.Actividad)
	}
	cuenta := func(paso string) int {
		n := 0
		for _, p := range dos.Ejecucion.Ciclo {
			if p == paso {
				n++
			}
		}
		return n
	}
	if n := cuenta("com.hola.navega.MainActivity.onCreate"); n != 1 {
		t.Errorf("la actividad cubierta repitió su onCreate %d veces", n)
	}
	if n := cuenta("com.hola.navega.SegundaActivity.onCreate"); n != 2 {
		t.Errorf("la actividad recreada hizo onCreate %d veces y se esperaban 2", n)
	}
}
