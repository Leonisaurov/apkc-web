package com.ejemplo.tareas;

import android.app.Activity;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.ListView;
import android.widget.TextView;

import java.util.ArrayList;

// Lista de tareas con categoría y estado persistente. Tres listas paralelas (texto, categoría y
// hecha) se guardan en SharedPreferences con un contador y se pintan con un ArrayAdapter sobre
// res/layout/item.xml. "Completar la primera" marca la primera pendiente y vuelve a pintar.
public class MainActivity extends Activity implements View.OnClickListener {
    private TextView nueva;
    private TextView etiqueta;
    private ArrayAdapter adaptador;
    private SharedPreferences prefs;
    private ArrayList textos;
    private ArrayList categorias;
    private ArrayList hechas;
    private String categoria;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        nueva = (TextView) findViewById(R.id.nueva);
        etiqueta = (TextView) findViewById(R.id.categoria);
        ListView lista = (ListView) findViewById(R.id.lista);
        prefs = getSharedPreferences("tareas", 0);
        textos = new ArrayList();
        categorias = new ArrayList();
        hechas = new ArrayList();
        categoria = "Casa";
        adaptador = new ArrayAdapter(this, R.layout.item);
        lista.setAdapter(adaptador);
        findViewById(R.id.casa).setOnClickListener(this);
        findViewById(R.id.trabajo).setOnClickListener(this);
        findViewById(R.id.estudio).setOnClickListener(this);
        findViewById(R.id.agregar).setOnClickListener(this);
        findViewById(R.id.completar).setOnClickListener(this);
        cargar();
        pintarCategoria();
    }

    private void pintarCategoria() {
        etiqueta.setText(getString(R.string.categoria) + categoria);
    }

    private void cargar() {
        int cuantas = prefs.getInt("cuantas", 0);
        for (int i = 0; i < cuantas; i++) {
            String clave = "texto" + i;
            String texto = prefs.getString(clave, "");
            textos.add(texto);
            clave = "categoria" + i;
            texto = prefs.getString(clave, "Casa");
            categorias.add(texto);
            clave = "hecha" + i;
            texto = prefs.getString(clave, "no");
            hechas.add(texto);
        }
        redibujar();
    }

    private String componer(int i) {
        String texto = (String) textos.get(i);
        String cat = (String) categorias.get(i);
        String hecha = (String) hechas.get(i);
        String linea = "[ ] ";
        boolean lista = hecha.equals("si");
        if (lista == true) {
            linea = "[x] ";
        }
        linea = linea + texto;
        linea = linea + " (";
        linea = linea + cat;
        linea = linea + ")";
        return linea;
    }

    private void redibujar() {
        adaptador.clear();
        int cuantas = textos.size();
        for (int i = 0; i < cuantas; i++) {
            adaptador.add(componer(i));
        }
    }

    private void agregar() {
        String texto = nueva.getText().toString();
        if (texto.length() == 0) {
            return;
        }
        int indice = textos.size();
        textos.add(texto);
        categorias.add(categoria);
        hechas.add("no");
        guardar(indice, texto, categoria, "no");
        nueva.setText("");
        redibujar();
    }

    private void guardar(int indice, String texto, String cat, String hecha) {
        String clave = "texto" + indice;
        prefs.edit().putString(clave, texto).apply();
        clave = "categoria" + indice;
        prefs.edit().putString(clave, cat).apply();
        clave = "hecha" + indice;
        prefs.edit().putString(clave, hecha).apply();
        prefs.edit().putInt("cuantas", textos.size()).apply();
    }

    private int primeraPendiente() {
        int cuantas = hechas.size();
        for (int i = 0; i < cuantas; i++) {
            String hecha = (String) hechas.get(i);
            boolean pendiente = hecha.equals("no");
            if (pendiente == true) {
                return i;
            }
        }
        return -1;
    }

    private void completar() {
        int elegida = primeraPendiente();
        if (elegida < 0) {
            return;
        }
        hechas.set(elegida, "si");
        String texto = (String) textos.get(elegida);
        String cat = (String) categorias.get(elegida);
        guardar(elegida, texto, cat, "si");
        redibujar();
    }

    private void elegirCategoria(String cual) {
        categoria = cual;
        pintarCategoria();
    }

    @Override
    public void onClick(View vista) {
        int cual = vista.getId();
        if (cual == R.id.casa) {
            elegirCategoria("Casa");
        } else if (cual == R.id.trabajo) {
            elegirCategoria("Trabajo");
        } else if (cual == R.id.estudio) {
            elegirCategoria("Estudio");
        } else if (cual == R.id.agregar) {
            agregar();
        } else {
            completar();
        }
    }
}
