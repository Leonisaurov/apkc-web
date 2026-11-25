package com.hola.navega;

import android.app.*;
import android.content.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.abrir).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                abrir();
            }
        });
    }

    void abrir() {
        pulsos = pulsos + 1;
        total.setText("Pulsos: " + pulsos);
        startActivity(new Intent(this, SegundaActivity.class));
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
    public void onRestart() { }
}
