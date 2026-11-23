package com.ejemplo.aabmultidex;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import dep.D0;
import dep.D20;
import saludo.Saludo;

// La app del caso multi-dex del .aab: llama a una clase del primer jar sintético y a otra del
// último, que caen en buckets distintos. Con veintidós jars el dexado de dependencias parte en
// tres, así que el bundle sale con base/dex/classes.dex (la app) y classes2.dex…classes4.dex.
public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        TextView texto = new TextView(this);
        texto.setText(Saludo.saludo("apkc") + " d0=" + D0.valor() + " d20=" + D20.valor());
        setContentView(texto);
    }
}
