package com.ejemplo.notas;

import android.app.Activity;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.TextView;
import android.widget.ListView;

// Bloc de notas mínimo: el texto se guarda en SharedPreferences con una clave por nota y un
// contador, y la lista se pinta con un ArrayAdapter sobre res/layout/item.xml (una raíz TextView,
// que es lo que el adaptador necesita; no se nombra android.R, que el emisor no resuelve).
public class MainActivity extends Activity implements View.OnClickListener {
    private TextView editor;
    private ArrayAdapter adaptador;
    private SharedPreferences prefs;

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        setContentView(R.layout.principal);
        editor = (TextView) findViewById(R.id.editor);
        ListView lista = (ListView) findViewById(R.id.lista);
        prefs = getSharedPreferences("notas", 0);
        adaptador = new ArrayAdapter(this, R.layout.item);
        lista.setAdapter(adaptador);
        cargar();
        findViewById(R.id.guardar).setOnClickListener(this);
        findViewById(R.id.borrar).setOnClickListener(this);
    }

    private void cargar() {
        int cuantas = prefs.getInt("cuantas", 0);
        for (int i = 0; i < cuantas; i++) {
            String clave = "nota" + i;
            String nota = prefs.getString(clave, "");
            adaptador.add(nota);
        }
    }

    private void guardar() {
        String texto = editor.getText().toString();
        if (texto.length() == 0) {
            return;
        }
        adaptador.add(texto);
        int indice = adaptador.getCount() - 1;
        String clave = "nota" + indice;
        prefs.edit().putString(clave, texto).apply();
        prefs.edit().putInt("cuantas", adaptador.getCount()).apply();
        editor.setText("");
    }

    private void borrar() {
        int cuantas = adaptador.getCount();
        if (cuantas == 0) {
            return;
        }
        int ultima = cuantas - 1;
        String nota = (String) adaptador.getItem(ultima);
        adaptador.remove(nota);
        prefs.edit().putInt("cuantas", adaptador.getCount()).apply();
    }

    @Override
    public void onClick(View vista) {
        if (vista.getId() == R.id.guardar) {
            guardar();
        } else {
            borrar();
        }
    }
}
