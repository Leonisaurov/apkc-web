package com.ejemplo.artver.zchar;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

// El caso ya cerrado del dexer: `static final boolean` y `static final char` viajan en
// `static_values_off` y su encoded_value tiene que ser del tipo del campo (Z y C), no `int`.
// ART rechazaba el dex entero con «unexpected static field initial value type: 'I' vs 'Z'» y
// `dexdump` no lo veía. Se incluyen también los estáticos SIN `ConstantValue` (no son `final`),
// cuyo cero escribe la rama del `<clinit>`.
public class MainActivity extends Activity {
    static final boolean VIVO = true;
    static final boolean MUERTO = false;
    static final char LETRA = 'C';
    static final char TILDE = '\u00f1';
    static final char TOPE = '\uffff';

    // sin `ConstantValue`: el `.class` no lleva el valor y el `<clinit>` lo asigna al correr
    static boolean interruptor = true;
    static int contador = 5;
    static char marca = 'K';

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        String salida = "Z=" + VIVO + "/" + MUERTO
            + " C=" + LETRA + TILDE + "(" + (int) TOPE + ")"
            + " clinit=" + interruptor + "/" + contador + "/" + marca;
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
