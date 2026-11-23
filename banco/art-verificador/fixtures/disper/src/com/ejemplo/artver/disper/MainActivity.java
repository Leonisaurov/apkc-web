package com.ejemplo.artver.disper;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

// `switch` con casos dispersos (javac emite `lookupswitch`, que el dexer traduce a `sparse-switch`),
// con casos contiguos (`tableswitch` → `packed-switch`), con `char` fuera de ASCII y con clave de
// cadena. El payload y los destinos tienen que caer donde ART los espera o la verificación falla.
public class MainActivity extends Activity {

    static String disperso(int v) {
        switch (v) {
            case -1000000: return "A";
            case -3: return "B";
            case -1: return "C";
            case 0: return "D";
            case 7: return "E";
            case 40: return "F";
            case 999: return "G";
            case 1000000: return "H";
            default: return "?";
        }
    }

    static String contiguo(int v) {
        switch (v) {
            case 1: return "uno";
            case 2: return "dos";
            case 3: return "tres";
            case 4: return "cuatro";
            default: return "otro";
        }
    }

    static String letra(char c) {
        switch (c) {
            case 'a': return "a";
            case '\u00f1': return "enie";
            case 'z': return "zeta";
            default: return "?";
        }
    }

    static String texto(String s) {
        switch (s) {
            case "alfa": return "1";
            case "beta": return "2";
            case "omega": return "3";
            default: return "0";
        }
    }

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        int[] casos = {-1000000, -3, -1, 0, 7, 40, 999, 1000000, 5};
        String salida = "";
        for (int i = 0; i < casos.length; i++) {
            if (i > 0) {
                salida = salida + ",";
            }
            salida = salida + disperso(casos[i]);
        }
        salida += " contiguo=" + contiguo(1) + contiguo(2) + contiguo(3) + contiguo(4) + contiguo(9)
            + " letra=" + letra('a') + letra('\u00f1') + letra('z') + letra('m')
            + " texto=" + texto("alfa") + texto("beta") + texto("omega") + texto("gamma");
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
