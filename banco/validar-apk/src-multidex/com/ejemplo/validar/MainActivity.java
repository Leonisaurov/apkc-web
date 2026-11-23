package com.ejemplo.validar;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import dep.D20;
import saludo.Saludo;

// La app de la pasada con varios dex: además del jar del fixture, llama a una clase que solo existe
// en uno de los jars sintéticos. Con 22 jars el dexado de dependencias parte en tres buckets, así
// que el APK sale con classes.dex (la app) y classes2.dex…classes4.dex: el nombre y el orden con
// los que el runtime abre cada fichero.
public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        TextView texto = new TextView(this);
        texto.setText(Saludo.saludo("apkc") + " d20=" + D20.valor());
        setContentView(texto);
    }
}
