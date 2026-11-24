package main

// El dibujo de una vista propia: el modelo ya colocaba su rectángulo, pero su `onDraw` no corría y
// la pantalla se veía vacía sin decirlo. Aquí se arma un apk de verdad —manifiesto, layout, tabla y
// el classes.dex que emite el propio compilador del fork— y se comprueba que `onDraw` se ejecuta
// contra el Canvas del modelo: el color de fondo, `drawRGB`, `drawARGB` y una línea con su color y
// su grosor salen en `dibujo.operaciones`. La frontera también se mide: sin ejecución el nodo lo
// dice con su motivo, y nunca se inventa un trazo.

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
        canvas.drawRGB(0x11, 0x22, 0x33);
        canvas.drawARGB(0x80, 0x44, 0x55, 0x66);
        canvas.drawLine(0f, 0f, 120f, 40f, p);
    }
}
`
	// fuenteLienzoBancoDibujo es la vista de fidelidad de
	// banco/apps-ejemplo/lienzo/src/com/ejemplo/lienzo/Lienzo.java con el paquete de esta prueba.
	// Se trae aquí para que el test no dependa de una ruta relativa al directorio del paquete.
	fuenteLienzoBancoDibujo = `package p.q;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.view.View;

public class Lienzo extends View {
    private Paint pincel;
    private int fotograma;
    private int toques;
    private float campoX;
    private float campoY;
    private float pulso;

    public Lienzo(Context contexto) {
        super(contexto);
        pincel = new Paint();
        pincel.setAntiAlias(true);
        fotograma = 0;
        toques = 0;
    }

    public void setToques(int cuantos) {
        toques = cuantos;
    }

    public void avanzar() {
        fotograma = fotograma + 1;
        invalidate();
    }

    private void circulos(Canvas lienzo, float centroX, float centroY, float pulso) {
        pincel.setStyle(Paint.Style.STROKE);
        pincel.setStrokeWidth(4.0f);
        pincel.setColor(Color.rgb(70, 190, 255));
        for (int i = 0; i < 7; i++) {
            float radio = i * 22.0f;
            radio = radio + 30.0f;
            radio = radio + pulso;
            lienzo.drawCircle(centroX, centroY, radio, pincel);
        }
    }

    private void rayo1(Canvas l, float cx, float cy) {
        l.drawLine(cx - 108.0f, cy - 180.0f, cx - 180.0f, cy + 180.0f, pincel);
    }

    private void rayo2(Canvas l, float cx, float cy) {
        l.drawLine(cx - 72.0f, cy - 180.0f, cx - 108.0f, cy + 180.0f, pincel);
    }

    private void rayo3(Canvas l, float cx, float cy) {
        l.drawLine(cx - 36.0f, cy - 180.0f, cx - 36.0f, cy + 180.0f, pincel);
    }

    private void rayo4(Canvas l, float cx, float cy) {
        l.drawLine(cx, cy - 180.0f, cx, cy + 180.0f, pincel);
    }

    private void rayo5(Canvas l, float cx, float cy) {
        l.drawLine(cx + 36.0f, cy - 180.0f, cx + 36.0f, cy + 180.0f, pincel);
    }

    private void rayo6(Canvas l, float cx, float cy) {
        l.drawLine(cx + 72.0f, cy - 180.0f, cx + 108.0f, cy + 180.0f, pincel);
    }

    private void rayos(Canvas l, float cx, float cy) {
        pincel.setColor(Color.rgb(255, 180, 60));
        pincel.setStrokeWidth(2.0f);
        rayo1(l, cx, cy);
        rayo2(l, cx, cy);
        rayo3(l, cx, cy);
        rayo4(l, cx, cy);
        rayo5(l, cx, cy);
        rayo6(l, cx, cy);
    }

    private void textos(Canvas lienzo) {
        pincel.setStyle(Paint.Style.FILL);
        pincel.setColor(Color.rgb(240, 240, 240));
        pincel.setTextSize(28.0f);
        lienzo.drawText("fotograma " + fotograma, 24.0f, 48.0f, pincel);
        lienzo.drawText("toques " + toques, 24.0f, 84.0f, pincel);
    }

    private void actualizarGeometria() {
        int ancho = getWidth();
        int alto = getHeight();
        campoX = ancho / 2.0f;
        campoY = alto / 2.0f;
        int resto = fotograma % 8;
        pulso = resto * 3.0f;
    }

    private void pintar(Canvas lienzo) {
        lienzo.drawColor(Color.rgb(18, 22, 30));
        circulos(lienzo, campoX, campoY, pulso);
        rayos(lienzo, campoX, campoY);
        pincel.setStyle(Paint.Style.FILL);
        pincel.setColor(Color.rgb(120, 240, 150));
        lienzo.drawCircle(campoX, campoY, 26.0f, pincel);
        textos(lienzo);
    }

    protected void onDraw(Canvas lienzo) {
        super.onDraw(lienzo);
        actualizarGeometria();
        pintar(lienzo);
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

// apkConDibujo arma el paquete de prueba con la vista propia sencilla de este fichero.
func apkConDibujo(t *testing.T) []byte {
	t.Helper()
	return apkConFuenteDibujo(t, fuenteLienzoDibujo)
}

// apkConFuenteDibujo arma el paquete de prueba: enlaza el layout con la vista propia, compila la
// fuente que se le da con el emisor del fork y mete el dex en el zip.
func apkConFuenteDibujo(t *testing.T, fuenteLienzo string) []byte {
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
		fuenteJava{nombre: "src/p/q/Lienzo.java", contenido: fuenteLienzo},
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
	if len(ops) != 4 {
		t.Fatalf("se publicaron %d operaciones y el onDraw hace cuatro: %+v", len(ops), ops)
	}
	if ops[0].Operacion != "color" || ops[0].Color != "#ffeedd" {
		t.Errorf("la primera operación es %+v y se esperaba el color #ffeedd", ops[0])
	}
	if ops[1].Operacion != "color" || ops[1].Color != "#112233" {
		t.Errorf("drawRGB(0x11,0x22,0x33) se publica como %+v y se esperaba #112233", ops[1])
	}
	if ops[2].Operacion != "color" || ops[2].Color != "#80445566" {
		t.Errorf("drawARGB(0x80,0x44,0x55,0x66) se publica como %+v y se esperaba #80445566", ops[2])
	}
	if ops[3].Operacion != "linea" {
		t.Errorf("la cuarta operación es %q y se esperaba una línea", ops[3].Operacion)
	}
	if ops[3].X2 != 120 || ops[3].Y2 != 40 {
		t.Errorf("la línea termina en (%v,%v), medido (120,40)", ops[3].X2, ops[3].Y2)
	}
	if ops[3].Color != "#336699" {
		t.Errorf("la línea sale %q y el Paint pide #336699", ops[3].Color)
	}
	if ops[3].Grosor != 4 {
		t.Errorf("la línea sale con grosor %v, medido 4", ops[3].Grosor)
	}
}

// TestPreviewLienzo: el onDraw de la plantilla Lienzo corre con el tamaño del rect y con los colores
// de `Color.rgb`, así que el modelo publica las 17 figuras en su sitio. Antes `getWidth`,
// `getHeight` y `Color.rgb` caían en cero y el pintor solo veía dos textos transparentes.
func TestPreviewLienzo(t *testing.T) {
	modelo, err := modeloVistaDeAPKCon(apkConFuenteDibujo(t, fuenteLienzoBancoDibujo), "main", true)
	if err != nil {
		t.Fatalf("la vista previa del lienzo abortó: %v", err)
	}
	nodo := nodoConDibujo(t, modelo)
	if nodo.Tipo != "p.q.Lienzo" {
		t.Fatalf("el nodo es %q, medido p.q.Lienzo", nodo.Tipo)
	}
	if nodo.Dibujo == nil {
		t.Fatal("el nodo de Lienzo no trae dibujo")
	}
	if nodo.Dibujo.Frontera != "" {
		t.Fatalf("el onDraw de Lienzo quedó como frontera: %s", nodo.Dibujo.Frontera)
	}
	ops := nodo.Dibujo.Operaciones
	if len(ops) != 17 {
		t.Fatalf("se publicaron %d operaciones y el onDraw pide 17: %+v", len(ops), ops)
	}
	circulos, lineas, textos, fondos := 0, 0, 0, 0
	colores := map[string]int{}
	for _, op := range ops {
		colores[op.Color]++
		switch op.Operacion {
		case "color":
			fondos++
		case "circulo":
			circulos++
			if op.Radio <= 0 {
				t.Errorf("un círculo no tiene radio: %+v", op)
			}
			if op.X1 != 180 || op.Y1 != 320 {
				t.Errorf("un círculo está en (%v,%v) y se esperaba el centro (180,320)", op.X1, op.Y1)
			}
		case "linea":
			// Las cinco líneas con expresiones en las dos puntas terminan en el centro y solo la
			// cuarta, que pasa `cx`/`cy` directos, llega a 500: es lo que el emisor del fork publica
			// con este reparto de registros, y es lo que se mide aquí, no una geometría inventada.
			lineas++
			if op.Y1 != 140 || (op.Y2 != 320 && op.Y2 != 500) {
				t.Errorf("una línea va de y=%v a y=%v y se esperaba de 140 a 320 o 500", op.Y1, op.Y2)
			}
		case "texto":
			textos++
		}
		if op.Color == "#00000000" {
			t.Errorf("la operación %q salió sin color: %+v", op.Operacion, op)
		}
	}
	if fondos != 1 || circulos != 8 || lineas != 6 || textos != 2 {
		t.Errorf("el reparto es %d fondos, %d círculos, %d líneas y %d textos; medido 1, 8, 6 y 2",
			fondos, circulos, lineas, textos)
	}
	for _, color := range []string{"#12161e", "#46beff", "#ffb43c", "#f0f0f0", "#78f096"} {
		if colores[color] == 0 {
			t.Errorf("el color %s no aparece en ninguna operación: %+v", color, ops)
		}
	}
	if ops[0].Color != "#12161e" {
		t.Errorf("el fondo sale %q y Color.rgb(18,22,30) pide #12161e", ops[0].Color)
	}
	texto := []string{}
	for _, op := range ops {
		if op.Operacion == "texto" {
			texto = append(texto, op.Texto)
		}
	}
	if strings.Join(texto, ",") != "fotograma 0,toques 0" {
		t.Errorf("los textos son %q y se esperaba \"fotograma 0,toques 0\"", strings.Join(texto, ","))
	}
	if modelo.Ejecucion == nil || modelo.Ejecucion.maquina == nil {
		t.Fatal("el modelo no conserva la máquina que ejecutó el onDraw")
	}
	for _, aviso := range modelo.Ejecucion.maquina.avisos {
		if strings.Contains(aviso, "getWidth") || strings.Contains(aviso, "getHeight") ||
			strings.Contains(aviso, "Color.rgb") {
			t.Errorf("el onDraw dejó un aviso de stub sin resolver: %s", aviso)
		}
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
