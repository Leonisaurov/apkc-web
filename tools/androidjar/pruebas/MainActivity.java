package com.myapp;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total.setText("Pulsos: " + pulsos);
            }
        });
        Toast.makeText(this, "Ciclo: onCreate", Toast.LENGTH_SHORT).show();
        setTitle("Ciclo: onStart");
    }

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add("Opciones").setOnMenuItemClickListener(new MenuItem.OnMenuItemClickListener() {
            public boolean onMenuItemClick(MenuItem item) {
                Toast.makeText(MainActivity.this, "Menu pulsado", Toast.LENGTH_SHORT).show();
                return true;
            }
        });
        return true;
    }
}
