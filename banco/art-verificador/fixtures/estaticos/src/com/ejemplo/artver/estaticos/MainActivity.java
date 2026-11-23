package com.ejemplo.artver.estaticos;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

// `static final` de los ocho primitivos más `String`, con los bordes de cada ancho: el encoded_value
// de `static_values_off` tiene que llevar el tipo del campo y el ancho que el formato pide para ese
// valor (Z sin bytes, C sin signo, B un byte siempre, S e I con signo mínimo). ART rechaza el dex
// entero al cargarlo si el tipo no cuadra, y `dexdump` lo enseña pero no lo comprueba.
public class MainActivity extends Activity {
    static final boolean Z = true;
    static final byte B = -128;
    static final char C = '\u00f1';
    static final short S = -32768;
    static final int I = Integer.MIN_VALUE;
    static final long J = Long.MAX_VALUE;
    static final float F = 1.5f;
    static final double D = -2.25;
    static final String T = "hola \u00f1";

    static final boolean ZF = false;
    static final byte BMAX = 127;
    static final char CMAX = '\uffff';
    static final short SMAX = 32767;
    static final int IMAX = Integer.MAX_VALUE;
    static final long JMIN = Long.MIN_VALUE;
    static final float FNAN = Float.NaN;
    static final double DINF = Double.NEGATIVE_INFINITY;
    static final float FZERO = -0.0f;

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        String salida = "Z=" + Z + "/" + ZF
            + " B=" + B + "/" + BMAX
            + " C=" + (int) C + "/" + (int) CMAX
            + " S=" + S + "/" + SMAX
            + " I=" + I + "/" + IMAX
            + " J=" + J + "/" + JMIN
            + " F=" + F + "/" + FNAN + "/" + FZERO
            + " D=" + D + "/" + DINF
            + " T=" + T;
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
