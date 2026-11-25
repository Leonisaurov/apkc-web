package main

// Regresión del ejemplo Lienzo. La plantilla pone la vista propia como raíz del layout, la actividad
// implementa View.OnClickListener y el onClick incrementa `toques`, se lo pasa a la vista y llama a
// `avanzar`. Antes de la corrección el modelo dibujaba «toques 0» tras pulsar, dejaba dos avisos de
// stub en la máquina (View.onDraw y Paint.setTextSize) y publicaba el texto sin tamaño. Aquí se
// comprueban las tres cosas a la vez, en la pasada única y en la sesión viva.

import (
	"encoding/json"
	"strings"
	"testing"
)

const fuenteMainLienzoToques = `package p.q;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.view.View;

public class MainActivity extends Activity implements View.OnClickListener {
    private Lienzo lienzo;
    private int toques;

    protected void onCreate(Bundle estado) {
        super.onCreate(estado);
        setContentView(R.layout.lienzo);
        lienzo = (Lienzo) findViewById(R.id.lienzo);
        lienzo.setOnClickListener(this);
    }

    public void onClick(View vista) {
        toques = toques + 1;
        lienzo.setToques(toques);
        lienzo.avanzar();
        Log.d("LIENZO", "toques " + toques);
    }
}
`

// layoutLienzoRaiz es el de la plantilla: la vista propia es la raíz y lleva id/lienzo.
const layoutLienzoRaiz = `<?xml version="1.0" encoding="utf-8"?>
<p.q.Lienzo xmlns:android="http://schemas.android.com/apk/res/android"
    android:id="@+id/lienzo"
    android:layout_width="match_parent"
    android:layout_height="match_parent" />
`

// apkConLienzoDeToques arma la APK de la plantilla con el compilador del fork, sin javac ni d8.
func apkConLienzoDeToques(t *testing.T) []byte {
	t.Helper()
	m, err := marcoDelAndroidJar(jarDelSDK(t))
	if err != nil {
		t.Fatalf("la tabla del framework: %v", err)
	}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(manifiestoDibujo)}
	layouts := []fuenteXML{{nombre: "res/layout/lienzo.xml", contenido: []byte(layoutLienzoRaiz)}}
	salida, err := enlazarProyecto(nil, layouts, nil, manifiesto, m,
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace con la vista propia: %v", err)
	}
	r, err := rDeTabla(salida.tabla)
	if err != nil {
		t.Fatalf("la R: %v", err)
	}
	dex := dexDeFuentes(t,
		fuenteJava{nombre: "src/p/q/MainActivity.java", contenido: fuenteMainLienzoToques},
		fuenteJava{nombre: "src/p/q/Lienzo.java", contenido: fuenteLienzoBancoDibujo},
		fuenteJava{nombre: "src/p/q/R.java", contenido: r},
	)
	manifiestoBinario, err := escribeAXML(salida.manifiesto)
	if err != nil {
		t.Fatal(err)
	}
	arsc, err := escribeTablaRecursos(salida.tabla)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := escribeAXML(salida.layouts["res/layout/lienzo.xml"])
	if err != nil {
		t.Fatal(err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml":   manifiestoBinario,
		"resources.arsc":        arsc,
		"res/layout/lienzo.xml": layout,
		"classes.dex":           dex,
	})
}

// textosYDibujo lee del JSON publicado los textos del onDraw y su tamaño.
func textosYDibujo(t *testing.T, linea []byte) ([]string, []float64) {
	t.Helper()
	var modelo struct {
		Raiz struct {
			Dibujo struct {
				Operaciones []struct {
					Operacion string  `json:"operacion"`
					Texto     string  `json:"texto"`
					Tamano    float64 `json:"tamano"`
				} `json:"operaciones"`
			} `json:"dibujo"`
		} `json:"raiz"`
	}
	if err := json.Unmarshal(linea, &modelo); err != nil {
		t.Fatalf("el modelo publicado no se lee: %v", err)
	}
	textos := []string{}
	tamanos := []float64{}
	for _, op := range modelo.Raiz.Dibujo.Operaciones {
		if op.Operacion == "texto" {
			textos = append(textos, op.Texto)
			tamanos = append(tamanos, op.Tamano)
		}
	}
	return textos, tamanos
}

// compruebaSinAvisosNiTextoViejo es la aserción compartida por las dos pruebas: el toque llegó al
// campo, los dos stubs están resueltos y el texto se dibuja con lo que `setTextSize` pidió.
func compruebaSinAvisosNiTextoViejo(t *testing.T, avisos []string, capturas []capturaTexto,
	textos []string, tamanos []float64) {
	t.Helper()
	for _, aviso := range avisos {
		if strings.Contains(aviso, "onDraw") || strings.Contains(aviso, "setTextSize") {
			t.Errorf("el intérprete dejó el aviso %q: los dos stub todavía no están resueltos", aviso)
		}
	}
	conToques := false
	for _, c := range capturas {
		if c.Texto == "toques 1" {
			conToques = true
		}
	}
	if !conToques {
		t.Errorf("el onClick no puso el campo toques a 1: capturas=%+v", capturas)
	}
	quiero := []string{"fotograma 1", "toques 1"}
	if strings.Join(textos, ",") != strings.Join(quiero, ",") {
		t.Errorf("el onDraw dibujó %q y se esperaba %q: el estado del toque no llegó al dibujo",
			strings.Join(textos, ","), strings.Join(quiero, ","))
	}
	for _, n := range tamanos {
		if n != 28 {
			t.Errorf("el texto se publicó con tamaño %v y setTextSize pidió 28: %v", n, tamanos)
		}
	}
}

// TestPreviewLienzoEntregaElToqueSinAvisos: en la pasada única, pulsar id/lienzo ejecuta el onClick,
// el campo sube a 1 y el onDraw lo pinta con el tamaño que pide el Paint. Sin los dos stubs y sin la
// herencia del estado esta prueba falla en tres sitios a la vez.
func TestPreviewLienzoEntregaElToqueSinAvisos(t *testing.T) {
	apk := apkConLienzoDeToques(t)
	modelo, err := modeloVistaDeAPKConEventos(apk, "lienzo", true,
		[]eventoUsuario{{Tipo: "pulsar", Dato: "id/lienzo"}})
	if err != nil {
		t.Fatalf("la vista previa del lienzo falló al pulsar: %v", err)
	}
	if got := strings.Join(modelo.Ejecucion.Pulsaciones, ","); got != "id/lienzo" {
		t.Fatalf("las pulsaciones son %q y se esperaba id/lienzo", got)
	}
	if modelo.Raiz == nil || modelo.Raiz.Dibujo == nil {
		t.Fatal("la vista propia no publicó dibujo")
	}
	textos := []string{}
	tamanos := []float64{}
	for _, op := range modelo.Raiz.Dibujo.Operaciones {
		if op.Operacion == "texto" {
			textos = append(textos, op.Texto)
			tamanos = append(tamanos, op.Tamano)
		}
	}
	avisos := append([]string(nil), modelo.Ejecucion.Avisos...)
	if modelo.Ejecucion.maquina != nil {
		avisos = append(avisos, modelo.Ejecucion.maquina.avisos...)
	}
	compruebaSinAvisosNiTextoViejo(t, avisos, modelo.Ejecucion.Capturas, textos, tamanos)
}

// TestSesionLienzoConservaLosToques: la misma APK en la sesión viva. Dos pulsaciones seguidas dan
// «toques 1» y «toques 2»: la máquina sigue viva, no se repite la primera orden y los eventos
// posteriores siguen llegando.
func TestSesionLienzoConservaLosToques(t *testing.T) {
	apk := apkConLienzoDeToques(t)
	s, err := nuevaSesionPreview(apk, "lienzo", "")
	if err != nil {
		t.Fatalf("la sesión del lienzo no arrancó: %v", err)
	}
	for n, quiero := range []string{"toques 1", "toques 2"} {
		linea, codigo := entrega(t, s, "pulso-"+string(rune('1'+n)),
			[]string{"--pulsar", "id/lienzo"})
		if codigo != 0 {
			t.Fatalf("la pulsación %d devolvió código %d: %s", n+1, codigo, linea)
		}
		textos, tamanos := textosYDibujo(t, []byte(linea))
		encontrado := false
		for _, texto := range textos {
			if texto == quiero {
				encontrado = true
			}
		}
		if !encontrado {
			t.Errorf("la pulsación %d dibujó %q y se esperaba %q", n+1, strings.Join(textos, ","), quiero)
		}
		for _, tam := range tamanos {
			if tam != 28 {
				t.Errorf("el texto de la pulsación %d salió con tamaño %v, medido 28", n+1, tam)
			}
		}
	}
	if s.m == nil || s.m.actividad == nil {
		t.Fatal("la sesión perdió su máquina tras los toques")
	}
	avisos := append([]string(nil), s.res.Avisos...)
	avisos = append(avisos, s.m.avisos...)
	for _, aviso := range avisos {
		if strings.Contains(aviso, "onDraw") || strings.Contains(aviso, "setTextSize") {
			t.Errorf("la sesión dejó el aviso %q", aviso)
		}
	}
}
