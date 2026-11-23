package com.ejemplo.artver.arreglos;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

// Un arreglo de cada tipo primitivo más `String` y dos dimensiones, con los bordes de cada ancho.
// Cubre `new-array`, `aget`/`aput` de todos los tipos, `array-length`, `filled-new-array` y el
// `checkcast` al pasar por `Object`. Cada tipo se mide en su propio método: así el reparto de
// registros no mezcla valores anchos con estrechos y el caso mide el dex, no al asignador.
public class MainActivity extends Activity {

    static int suma(int[] a) {
        int s = 0;
        for (int i = 0; i < a.length; i++) {
            s += a[i];
        }
        return s;
    }

    static String booleanos() {
        boolean[] a = new boolean[] {true, false, true};
        return a.length + ":" + a[0] + ":" + a[1];
    }

    static String bytes() {
        byte[] a = new byte[] {-128, 0, 127};
        return a[0] + "," + a[2];
    }

    static String caracteres() {
        char[] a = new char[] {'a', '\u00f1', '\uffff'};
        return (int) a[0] + "," + (int) a[1] + "," + (int) a[2];
    }

    static String cortos() {
        short[] a = new short[] {-32768, 0, 32767};
        return a[0] + "," + a[2];
    }

    static String enteros() {
        int[] a = new int[] {1, -2, 3};
        return String.valueOf(suma(a));
    }

    static String largos() {
        long[] a = new long[] {Long.MIN_VALUE, 0L, Long.MAX_VALUE};
        return a.length + ":" + a[0] + ":" + a[2];
    }

    static String flotantes() {
        float[] a = new float[] {1.5f, -0.0f};
        return a.length + ":" + Float.floatToIntBits(a[0]) + ":" + Float.floatToIntBits(a[1]);
    }

    static String dobles() {
        double[] a = new double[] {2.25, -1e300};
        return a.length + ":" + Double.doubleToLongBits(a[0]) + ":" + Double.doubleToLongBits(a[1]);
    }

    static String textos() {
        String[] a = new String[] {"uno", "dos"};
        return a[0] + a[1];
    }

    static String matriz() {
        int[][] a = new int[][] {{1, 2}, {3, 4}};
        return String.valueOf(suma(a[0]) + suma(a[1]));
    }

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        String salida = "z=" + booleanos()
            + " b=" + bytes()
            + " c=" + caracteres()
            + " s=" + cortos()
            + " i=" + enteros()
            + " j=" + largos()
            + " f=" + flotantes()
            + " d=" + dobles()
            + " t=" + textos()
            + " m=" + matriz();
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
