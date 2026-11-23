package com.ejemplo.validar;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import saludo.Saludo;

// La app de la pasada de un solo jar: llama a una clase de jar/saludo.jar, que el compilador Go
// resuelve leyendo el jar y el dexer Go traduce a classes2.dex. Lo que se valida después es que el
// APK firmado lleve los dos dex con los nombres que el runtime espera.
public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        TextView texto = new TextView(this);
        texto.setText(Saludo.saludo("apkc"));
        setContentView(texto);
    }
}
