package main

// El dibujo de una vista propia: el modelo ya colocaba su rectángulo, pero su `onDraw` no corría y
// la pantalla se veía vacía sin decirlo. Aquí se arma un apk de verdad —manifiesto, layout, tabla y
// el classes.dex que emite el propio compilador del fork— y se comprueba que `onDraw` se ejecuta
// contra el Canvas del modelo: el color de fondo y una línea con su color y su grosor salen en
// `dibujo.operaciones`. La frontera también se mide: sin ejecución el nodo lo dice con su motivo, y
// nunca se inventa un trazo.

import (
	"strings"
	"testing"
)

const (
	fuenteMainDibujo = `package p.q;

import android.app.Activity;
import android.os.Bundle;

public class MainActivity extends Activity {
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
    }
}
`
	fuenteLienzoDibujo = `package p.q;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.view.View;

public class Lienzo extends View {
    public Lienzo(Context contexto) {
        super(contexto);
    }

    protected void onDraw(Canvas canvas) {
        Paint p = new Paint();
        p.setColor(0xFF336699);
        p.setStrokeWidth(4f);
        canvas.drawColor(0xFFFFEEDD);
        canvas.drawLine(0f, 0f, 120f, 40f, p);
    }
}
`
	layoutDibujo = `<?xml version="1.0" encoding="utf-8"?>
<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="match_parent">
    <p.q.Lienzo android:id="@+id/lienzo"
        android:layout_width="match_parent" android:layout_height="match_parent" />
</FrameLayout>
`
	manifiestoDibujo = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="p.q">
    <application>
        <activity android:name=".MainActivity" />
    </application>
</manifest>`
)

// apkConDibujo arma el paquete de prueba: enlaza el layout con la vista propia, compila el fuente
// con el emisor del fork y mete el dex en el zip.
func apkConDibujo(t *testing.T) []byte {
	t.Helper()
	m, err := marcoDelAndroidJar(jarDelSDK(t))
	if err != nil {
		t.Fatalf("la tabla del framework: %v", err)
	}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(manifiestoDibujo)}
	layouts := []fuenteXML{{nombre: "res/layout/main.xml", contenido: []byte(layoutDibujo)}}
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
		fuenteJava{nombre: "src/p/q/MainActivity.java", contenido: fuenteMainDibujo},
		fuenteJava{nombre: "src/p/q/Lienzo.java", contenido: fuenteLienzoDibujo},
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
	layout, err := escribeAXML(salida.layouts["res/layout/main.xml"])
	if err != nil {
		t.Fatal(err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiestoBinario,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dex,
	})
}

// nodoConDibujo devuelve el nodo de la vista propia del modelo.
func nodoConDibujo(t *testing.T, modelo *modeloVista) *nodoVista {
	t.Helper()
	if modelo.Raiz == nil || len(modelo.Raiz.Hijos) != 1 {
		t.Fatalf("el modelo no trae la vista propia: %+v", modelo.Raiz)
	}
	return modelo.Raiz.Hijos[0]
}

// TestPreviewDibujaElOnDrawDeLaVistaPropia: el onDraw corre y publica el color de fondo y la línea
// con el color y el grosor del Paint. Sin esto la vista propia solo era su rectángulo.
func TestPreviewDibujaElOnDrawDeLaVistaPropia(t *testing.T) {
	modelo, err := modeloVistaDeAPKCon(apkConDibujo(t), "main", true)
	if err != nil {
		t.Fatalf("la vista previa abortó con una vista propia que dibuja: %v", err)
	}
	nodo := nodoConDibujo(t, modelo)
	if nodo.Tipo != "p.q.Lienzo" {
		t.Fatalf("el nodo es %q, medido p.q.Lienzo", nodo.Tipo)
	}
	if nodo.Dibujo == nil {
		t.Fatal("el nodo de la vista propia no trae dibujo")
	}
	if nodo.Dibujo.Frontera != "" {
		t.Fatalf("el onDraw quedó como frontera: %s", nodo.Dibujo.Frontera)
	}
	ops := nodo.Dibujo.Operaciones
	if len(ops) != 2 {
		t.Fatalf("se publicaron %d operaciones y el onDraw hace dos: %+v", len(ops), ops)
	}
	if ops[0].Operacion != "color" || ops[0].Color != "#ffeedd" {
		t.Errorf("la primera operación es %+v y se esperaba el color #ffeedd", ops[0])
	}
	if ops[1].Operacion != "linea" {
		t.Errorf("la segunda operación es %q y se esperaba una línea", ops[1].Operacion)
	}
	if ops[1].X2 != 120 || ops[1].Y2 != 40 {
		t.Errorf("la línea termina en (%v,%v), medido (120,40)", ops[1].X2, ops[1].Y2)
	}
	if ops[1].Color != "#336699" {
		t.Errorf("la línea sale %q y el Paint pide #336699", ops[1].Color)
	}
	if ops[1].Grosor != 4 {
		t.Errorf("la línea sale con grosor %v, medido 4", ops[1].Grosor)
	}
}

// TestPreviewMarcaLaFronteraDelOnDrawSinEjecucion: sin ejecutar el dex no hay máquina que corra el
// onDraw. El nodo no finge el dibujo: publica la frontera con el motivo, y el modelo sigue saliendo.
func TestPreviewMarcaLaFronteraDelOnDrawSinEjecucion(t *testing.T) {
	modelo, err := modeloVistaDeAPKCon(apkConDibujo(t), "main", false)
	if err != nil {
		t.Fatalf("la vista previa abortó con --sin-ejecutar: %v", err)
	}
	nodo := nodoConDibujo(t, modelo)
	if nodo.Dibujo == nil || nodo.Dibujo.Frontera == "" {
		t.Fatalf("sin ejecución el nodo no declara la frontera: %+v", nodo.Dibujo)
	}
	if len(nodo.Dibujo.Operaciones) != 0 {
		t.Errorf("sin ejecución el modelo publicó operaciones inventadas: %+v", nodo.Dibujo.Operaciones)
	}
	if !strings.Contains(nodo.Dibujo.Frontera, "no se ejecutó onDraw") {
		t.Errorf("la frontera no dice qué pasó: %q", nodo.Dibujo.Frontera)
	}
}
