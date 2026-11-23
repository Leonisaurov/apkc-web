package main

// La prueba de aceptación de E7 del plan de emulación: un `AlertDialog` propio aparece como capa
// del modelo con su mensaje y sus botones, pulsar un botón ejecuta su `OnClickListener` y cierra el
// diálogo, y un `Toast` deja su línea en la consola y el último aviso efímero. El APK se arma en
// memoria con los escritores del fork y el dex sale del emisor del fork, como el resto de las
// pruebas de la vista previa (regla 2: sin binarios versionados).

import (
	"strings"
	"testing"
)

// fuenteDialogo es una actividad con un botón que abre un `AlertDialog` de dos botones y muestra un
// Toast. El botón positivo escribe el texto y llama a `dismiss()`; el negativo solo escribe, para
// que la prueba distinga el cierre por `dismiss()` del que hace el framework al pulsar.
const fuenteDialogo = `package com.hola.contador;

import android.app.*;
import android.content.*;
import android.os.*;
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
                abreDialogo();
            }
        });
    }

    void abreDialogo() {
        this.configura(
            new AlertDialog.Builder(this).setTitle("Confirmar").setMessage("Seguro?"),
            new DialogInterface.OnClickListener() {
                public void onClick(DialogInterface d, int which) {
                    total.setText("Aceptado");
                    d.dismiss();
                }
            },
            new DialogInterface.OnClickListener() {
                public void onClick(DialogInterface d, int which) {
                    total.setText("Cancelado");
                }
            });
        Toast.makeText(this, "dialogo abierto", Toast.LENGTH_SHORT).show();
    }

    void configura(AlertDialog.Builder b, DialogInterface.OnClickListener ok,
            DialogInterface.OnClickListener no) {
        b.setPositiveButton("Aceptar", ok);
        b.setNegativeButton("Cancelar", no);
        b.show();
    }
}
`

// textoDelModelo es el texto dinámico del primer hijo del layout, que es el TextView del Contador.
func textoDelModelo(t *testing.T, m *modeloVista) string {
	t.Helper()
	if m.Raiz == nil || len(m.Raiz.Hijos) == 0 {
		t.Fatalf("el modelo no trae la pantalla: %+v", m)
	}
	return m.Raiz.Hijos[0].TextoDinamico
}

// TestElPreviewMuestraUnDialogoPropio es el criterio de aceptación de E7: pulsar el botón de la
// actividad abre un diálogo con su título, su mensaje y sus dos botones; pulsar el botón positivo
// corre su `OnClickListener`, cierra el diálogo y deja el texto en la pantalla; el Toast que la app
// muestra queda en la consola. El botón negativo cierra el diálogo sin que la app llame a
// `dismiss()`, que es el cierre del framework.
func TestElPreviewMuestraUnDialogoPropio(t *testing.T) {
	apk := apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteDialogo})

	soloAbre, err := modeloVistaDeAPKConOrdenes(apk, "", true,
		[]ordenEjecucion{{Tipo: ordenPulsarTipo, Valor: "id/pulsar"}})
	if err != nil {
		t.Fatalf("el clic que abre el diálogo falló: %v", err)
	}
	e := soloAbre.Ejecucion
	if e == nil || e.Error != "" {
		t.Fatalf("la ejecución falló: %+v", e)
	}
	if len(e.Dialogos) != 1 {
		t.Fatalf("la ejecución publica %d diálogos y se esperaba uno: %+v", len(e.Dialogos), e.Dialogos)
	}
	d := e.Dialogos[0]
	if d.Titulo != "Confirmar" {
		t.Errorf("el título del diálogo es %q y se esperaba «Confirmar»", d.Titulo)
	}
	if d.Mensaje != "Seguro?" {
		t.Errorf("el mensaje del diálogo es %q y se esperaba «Seguro?»", d.Mensaje)
	}
	if len(d.Botones) != 2 {
		t.Fatalf("el diálogo publica %d botones y se esperaban dos: %+v", len(d.Botones), d.Botones)
	}
	if d.Botones[0].Tipo != "positivo" || d.Botones[0].Texto != "Aceptar" {
		t.Errorf("el primer botón es %+v y se esperaba el positivo «Aceptar»", d.Botones[0])
	}
	if d.Botones[1].Tipo != "negativo" || d.Botones[1].Texto != "Cancelar" {
		t.Errorf("el segundo botón es %+v y se esperaba el negativo «Cancelar»", d.Botones[1])
	}
	// el diálogo es también la capa que pinta el modelo, con el mismo contenido
	if soloAbre.Dialogo == nil {
		t.Error("el modelo no publica el diálogo como capa")
	} else if soloAbre.Dialogo.Titulo != "Confirmar" || soloAbre.Dialogo.Mensaje != "Seguro?" ||
		len(soloAbre.Dialogo.Botones) != 2 {
		t.Errorf("la capa del modelo no coincide con la ejecución: %+v", soloAbre.Dialogo)
	}
	// el Toast: línea de consola con su origen y el aviso efímero del modelo
	if e.Toast != "dialogo abierto" {
		t.Errorf("el aviso efímero es %q y se esperaba «dialogo abierto»", e.Toast)
	}
	consola := ""
	for _, l := range e.Consola {
		if l.Origen == "Toast" && l.Texto == "dialogo abierto" {
			consola = l.Texto
		}
	}
	if consola == "" {
		t.Errorf("el Toast no dejó su línea en la consola: %+v", e.Consola)
	}

	// pulsar el botón positivo: su listener escribe el texto, llama a dismiss() y el diálogo se
	// cierra. Dos clics entregados (el de la actividad y el del diálogo).
	acepta, err := modeloVistaDeAPKConOrdenes(apk, "", true, []ordenEjecucion{
		{Tipo: ordenPulsarTipo, Valor: "id/pulsar"},
		{Tipo: ordenPulsarTipo, Valor: "dialogo/positivo"},
	})
	if err != nil {
		t.Fatalf("pulsar el botón positivo del diálogo falló: %v", err)
	}
	if got := textoDelModelo(t, acepta); got != "Aceptado" {
		t.Errorf("tras pulsar Aceptar el texto es %q y se esperaba «Aceptado»; "+
			"capturas=%+v avisos=%v", got, acepta.Ejecucion.Capturas, acepta.Ejecucion.Avisos)
	}
	if acepta.Dialogo != nil || len(acepta.Ejecucion.Dialogos) != 0 {
		t.Errorf("tras pulsar Aceptar el diálogo sigue abierto: %+v", acepta.Ejecucion.Dialogos)
	}
	if acepta.Ejecucion.Interacciones != 2 {
		t.Errorf("se entregaron %d interacciones y se esperaban 2", acepta.Ejecucion.Interacciones)
	}
	for _, aviso := range acepta.Ejecucion.Avisos {
		if strings.Contains(aviso, "no está en el dex ni en los stubs") {
			t.Errorf("el diálogo llamó a algo fuera del subconjunto: %s", aviso)
		}
	}

	// pulsar el botón negativo: su listener escribe el texto y el framework cierra el diálogo sin
	// que la app llame a dismiss()
	cancela, err := modeloVistaDeAPKConOrdenes(apk, "", true, []ordenEjecucion{
		{Tipo: ordenPulsarTipo, Valor: "id/pulsar"},
		{Tipo: ordenPulsarTipo, Valor: "dialogo/negativo"},
	})
	if err != nil {
		t.Fatalf("pulsar el botón negativo del diálogo falló: %v", err)
	}
	if got := textoDelModelo(t, cancela); got != "Cancelado" {
		t.Errorf("tras pulsar Cancelar el texto es %q y se esperaba «Cancelado»", got)
	}
	if cancela.Dialogo != nil || len(cancela.Ejecucion.Dialogos) != 0 {
		t.Errorf("tras pulsar Cancelar el diálogo sigue abierto: %+v", cancela.Ejecucion.Dialogos)
	}
}
