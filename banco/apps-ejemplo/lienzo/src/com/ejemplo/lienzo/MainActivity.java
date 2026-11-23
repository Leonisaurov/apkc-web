package com.ejemplo.lienzo;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;
import android.view.View;

// La pantalla monta la vista propia en código, sin XML. Tocar la superficie avanza el fotograma:
// es la sonda de fidelidad del emulador (Canvas, Paint, invalidate y el ciclo de dibujo).
public class MainActivity extends Activity implements View.OnClickListener {
    private Lienzo lienzo;
    private int toques;

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        Context contexto = this;
        lienzo = new Lienzo(contexto);
        lienzo.setOnClickListener(this);
        setContentView(lienzo);
    }

    @Override
    public void onClick(View vista) {
        toques = toques + 1;
        lienzo.setToques(toques);
        lienzo.avanzar();
    }
}
