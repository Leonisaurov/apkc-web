package com.ejemplo.artjar;

import android.app.Activity;
import android.os.Bundle;
import android.support.v4.text.TextUtilsCompat;
import android.util.Log;
import android.widget.TextView;

// La app no define htmlEncode: la única copia de ese método es la del
// android-support-v4.jar, que viaja en classes2.dex. Si ART ejecuta esta actividad y pinta el
// resultado, ejecutó bytecode de nuestro dexer para el jar. `Log.d` deja el resultado en logcat
// para que el guion no dependa de que la UI esté en primer plano.
public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        String salida = TextUtilsCompat.htmlEncode("<b>apkc & cia</b>");
        Log.d("artjar", "htmlEncode(" + "<b>apkc & cia</b>" + ") -> " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
