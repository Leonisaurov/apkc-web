package com.ejemplo.artver.ifaces;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

// `invokeinterface` de verdad: la llamada se resuelve en la tabla del objeto, no en la de la clase
// que la escribe. Un arreglo de la interfaz, dos implementaciones directas, una clase abstracta que
// implementa la interfaz y un `instanceof` sobre la interfaz. Si el `method_ids` de la llamada o el
// `invokeinterface` con su conteo de palabras no cuadran, la verificación falla en ART.
public class MainActivity extends Activity {

    interface Operacion {
        int aplica(int x);
    }

    interface Cadena {
        String une(String a, String b);
    }

    static class Dobla implements Operacion {
        public int aplica(int x) {
            return x * 2;
        }
    }

    static class Niega implements Operacion {
        public int aplica(int x) {
            return -x;
        }
    }

    abstract static class Base implements Operacion {
        public int aplica(int x) {
            return aplica2(x) + 1;
        }

        abstract int aplica2(int x);
    }

    static class Suma extends Base {
        int aplica2(int x) {
            return x + 10;
        }
    }

    static class Junta implements Cadena {
        public String une(String a, String b) {
            return a + "-" + b;
        }
    }

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        Operacion[] ops = new Operacion[] {new Dobla(), new Niega(), new Suma()};
        String salida = "";
        for (int i = 0; i < ops.length; i++) {
            if (i > 0) {
                salida = salida + ",";
            }
            salida = salida + ops[i].aplica(5);
        }
        Cadena cadena = new Junta();
        Object objeto = cadena;
        boolean esCadena = objeto instanceof Cadena;
        salida += " cadena=" + cadena.une("a", "b") + " instanceof=" + esCadena;
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
