package com.ejemplo.deps;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import saludo.Saludo;

// App mínima que usa una clase que no está en src/ sino en jar/saludo.jar. Es el caso que el
// compilador Go tiene que resolver leyendo el jar, igual que javac con -classpath.
public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        TextView vista = new TextView(this);
        vista.setText(Saludo.saludo());
        setContentView(vista);
    }
}
