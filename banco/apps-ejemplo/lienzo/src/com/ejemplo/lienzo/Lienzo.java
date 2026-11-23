package com.ejemplo.lienzo;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.view.View;

// Vista propia: todo el dibujo pasa por Canvas y Paint dentro de onDraw, sin recursos de imagen.
// Es la sonda de fidelidad del emulador: círculos, líneas y texto en coordenadas flotantes.
// Cada figura va en su propio método: el emisor reparte registros por método y una onDraw con
// demasiados locales a la vez se queda sin sitio para la forma corta de las instrucciones.
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

    // Una línea por método: cada expresión deja su registro reservado para todo el método y seis
    // líneas en uno solo pasan de los quince que caben en la forma corta de `aload`/`fload`.
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

    @Override
    protected void onDraw(Canvas lienzo) {
        super.onDraw(lienzo);
        actualizarGeometria();
        pintar(lienzo);
    }
}
