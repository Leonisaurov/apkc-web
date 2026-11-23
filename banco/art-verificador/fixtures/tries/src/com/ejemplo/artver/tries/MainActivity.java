package com.ejemplo.artver.tries;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

// `try`/`catch`/`finally`: el handler de la tabla de excepciones, la unión de tipos del multi-catch,
// el `finally` que javac duplica alrededor del `return` (el valor pendiente) y el `finally` anidado.
// El oráculo es ART: si el `catch_handler` o el rango de `tries` no cuadra, la verificación se cae.
public class MainActivity extends Activity {

    static int aritmetica(int x) {
        int r = 0;
        try {
            r = 100 / x;
        } catch (ArithmeticException e) {
            r = -1;
        } finally {
            r += 1000;
        }
        return r;
    }

    // el valor devuelto se calcula antes del `finally`, que ya no lo cambia
    static String pendiente(int x) {
        String r = "inicio";
        try {
            if (x == 0) throw new IllegalStateException("cero");
            r = "bien" + x;
            return r;
        } catch (IllegalStateException e) {
            r = "fallo:" + e.getMessage();
            return r;
        } finally {
            r = r + "|finally";
        }
    }

    static String multi(int x) {
        try {
            if (x == 1) throw new IllegalStateException("uno");
            if (x == 2) throw new NumberFormatException("dos");
            return "ok";
        } catch (IllegalStateException | NumberFormatException e) {
            return "multi:" + e.getClass().getSimpleName();
        } finally {
            try {
                if (x == 3) throw new RuntimeException("tres");
            } catch (RuntimeException e) {
                // se traga la del `finally` anidado
            } finally {
                // nada
            }
        }
    }

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        String salida = "aritmetica=" + aritmetica(0) + "," + aritmetica(4)
            + " pendiente=" + pendiente(0) + "," + pendiente(7)
            + " multi=" + multi(1) + "," + multi(2) + "," + multi(3);
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
