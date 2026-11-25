package com.hola.navega;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class SegundaActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.segunda);
        TextView texto = (TextView) findViewById(R.id.texto);
        texto.setText("Pantalla 2");
        findViewById(R.id.volver).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                finish();
            }
        });
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
}
