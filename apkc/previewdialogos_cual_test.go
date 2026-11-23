package main

// Cierra la condición que dejó abierta el crítico de E7: la prueba de aceptación comprobaba que el
// oyente del diálogo se ejecutaba, pero no leía el `which` que recibe. Aquí cada botón tiene su
// propio `OnClickListener`, y lo único que hace es volcar el valor recibido en el TextView y en la
// consola (`Log.i`), así que el JSON del modelo enseña el número exacto que entregó el intérprete.
// Si el emulador mandara +1/+2/+3, o cambiara los papeles, o no existiera el botón neutro, las
// aserciones de abajo fallan.

import (
	"strings"
	"testing"
)

// fuenteDialogoConWhich es una actividad con un `AlertDialog` de tres botones. Cada oyente escribe
// su papel y el `which` que recibe, sin traducirlo ni compararlo: el valor viaja tal cual al texto
// de la vista y a la consola, y es la prueba la que decide si es el correcto.
const fuenteDialogoConWhich = `package com.hola.contador;

import android.app.*;
import android.content.*;
import android.os.*;
import android.util.Log;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                abre();
            }
        });
    }

    void abre() {
        this.configura(
            new AlertDialog.Builder(this).setTitle("Elige").setMessage("Que hago?"),
            new DialogInterface.OnClickListener() {
                public void onClick(DialogInterface d, int which) {
                    total.setText("positivo=" + which);
                    Log.i("which", "positivo=" + which);
                }
            },
            new DialogInterface.OnClickListener() {
                public void onClick(DialogInterface d, int which) {
                    total.setText("negativo=" + which);
                    Log.i("which", "negativo=" + which);
                }
            },
            new DialogInterface.OnClickListener() {
                public void onClick(DialogInterface d, int which) {
                    total.setText("neutro=" + which);
                    Log.i("which", "neutro=" + which);
                }
            });
    }

    void configura(AlertDialog.Builder b, DialogInterface.OnClickListener si,
            DialogInterface.OnClickListener no, DialogInterface.OnClickListener quizas) {
        b.setPositiveButton("Si", si);
        b.setNegativeButton("No", no);
        b.setNeutralButton("Quizas", quizas);
        b.show();
    }
}
`

// TestElPreviewEntregaElWhichDeCadaBotonDelDialogo pulsa los tres botones de un diálogo con tres
// oyentes y exige, en cada caso, el `which` de `DialogInterface` del framework (-1 positivo, -2
// negativo, -3 neutro), que el oyente del papel correcto sea el que corrió, que el diálogo quede
// cerrado y que la línea del oyente esté en la consola.
func TestElPreviewEntregaElWhichDeCadaBotonDelDialogo(t *testing.T) {
	apk := apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteDialogoConWhich})

	// el diálogo tiene que publicar los tres botones: sin el neutro no hay nada que pulsar
	abre, err := modeloVistaDeAPKConOrdenes(apk, "", true,
		[]ordenEjecucion{{Tipo: ordenPulsarTipo, Valor: "id/pulsar"}})
	if err != nil {
		t.Fatalf("el clic que abre el diálogo falló: %v", err)
	}
	if abre.Ejecucion == nil || len(abre.Ejecucion.Dialogos) != 1 {
		t.Fatalf("la ejecución no publica el diálogo: %+v", abre.Ejecucion)
	}
	if botones := abre.Ejecucion.Dialogos[0].Botones; len(botones) != 3 {
		t.Fatalf("el diálogo publica %d botones y se esperaban tres: %+v", len(botones), botones)
	}
	for i, tipo := range []string{"positivo", "negativo", "neutro"} {
		if got := abre.Ejecucion.Dialogos[0].Botones[i].Tipo; got != tipo {
			t.Errorf("el botón %d es %q y se esperaba %q", i, got, tipo)
		}
	}

	casos := []struct {
		tipo     string
		which    int
		esperado string
	}{
		{"positivo", -1, "positivo=-1"},
		{"negativo", -2, "negativo=-2"},
		{"neutro", -3, "neutro=-3"},
	}
	for _, c := range casos {
		m, err := modeloVistaDeAPKConOrdenes(apk, "", true, []ordenEjecucion{
			{Tipo: ordenPulsarTipo, Valor: "id/pulsar"},
			{Tipo: ordenPulsarTipo, Valor: "dialogo/" + c.tipo},
		})
		if err != nil {
			t.Fatalf("pulsar dialogo/%s falló: %v", c.tipo, err)
		}
		e := m.Ejecucion
		if e == nil || e.Error != "" {
			t.Fatalf("la ejecución de dialogo/%s falló: %+v", c.tipo, e)
		}
		// el valor que el oyente leyó, tal cual, en el TextView
		if got := textoDelModelo(t, m); got != c.esperado {
			t.Errorf("dialogo/%s entregó un which que el oyente escribió como %q y se esperaba %q "+
				"(el framework usa %d)", c.tipo, got, c.esperado, c.which)
		}
		// y la misma línea en la consola, con su origen
		linea := ""
		for _, l := range e.Consola {
			if l.Origen == "Log.i" && strings.HasPrefix(l.Texto, c.tipo+"=") {
				linea = l.Texto
			}
		}
		if linea != c.esperado {
			t.Errorf("dialogo/%s no dejó %q en la consola con origen Log.i: %+v",
				c.tipo, c.esperado, e.Consola)
		}
		// deja constancia del valor medido: en `-v` se lee qué recibió cada botón
		t.Logf("dialogo/%s: el oyente leyó which=%d y lo escribió como %q", c.tipo, c.which, linea)
		// el clic cierra el diálogo, como el framework
		if len(e.Dialogos) != 0 || m.Dialogo != nil {
			t.Errorf("tras dialogo/%s el diálogo sigue abierto: %+v", c.tipo, e.Dialogos)
		}
		if e.Interacciones != 2 {
			t.Errorf("tras dialogo/%s se entregaron %d interacciones y se esperaban 2",
				c.tipo, e.Interacciones)
		}
		for _, aviso := range e.Avisos {
			if strings.Contains(aviso, "no está en el dex ni en los stubs") {
				t.Errorf("dialogo/%s llamó a algo fuera del subconjunto: %s", c.tipo, aviso)
			}
		}
	}
}
