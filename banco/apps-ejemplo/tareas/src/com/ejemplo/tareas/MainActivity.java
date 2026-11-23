package com.ejemplo.tareas;

import android.app.Activity;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.ListView;
import android.widget.TextView;

// Lista de tareas con categoría, estado y filtro. Tres arreglos paralelos (texto, categoría y hecha)
// más un contador, persistidos en SharedPreferences con una clave por campo y tarea.
// El adaptador se reconstruye con la copia ya filtrada cada vez que algo cambia: es la forma que
// `apkc preview` pinta (el ArrayAdapter del framework enseña los elementos que recibe en el
// constructor) y en el aparato equivale a notifyDataSetChanged sin caché que invalidar.
public class MainActivity extends Activity implements View.OnClickListener {
    private static final int MAXIMO = 64;

    private TextView nueva;
    private TextView etiqueta;
    private ListView lista;
    private ArrayAdapter adaptador;
    private SharedPreferences prefs;
    private String[] textos;
    private String[] categorias;
    private String[] hechas;
    private int cuantas;
    private int filtro;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        nueva = (TextView) findViewById(R.id.nueva);
        etiqueta = (TextView) findViewById(R.id.categoria);
        lista = (ListView) findViewById(R.id.lista);
        prefs = getSharedPreferences("tareas", 0);
        textos = new String[MAXIMO];
        categorias = new String[MAXIMO];
        hechas = new String[MAXIMO];
        cuantas = 0;
        filtro = 0;
        cargar();
        findViewById(R.id.casa).setOnClickListener(this);
        findViewById(R.id.trabajo).setOnClickListener(this);
        findViewById(R.id.estudio).setOnClickListener(this);
        findViewById(R.id.agregar).setOnClickListener(this);
        findViewById(R.id.completar).setOnClickListener(this);
        findViewById(R.id.filtrar).setOnClickListener(this);
        redibujar();
    }

    private void cargar() {
        int guardadas = prefs.getInt("cuantas", 0);
        for (int i = 0; i < guardadas; i++) {
            if (cuantas >= MAXIMO) {
                return;
            }
            String clave = "texto" + i;
            textos[cuantas] = prefs.getString(clave, "");
            clave = "categoria" + i;
            categorias[cuantas] = prefs.getString(clave, "Casa");
            clave = "hecha" + i;
            hechas[cuantas] = prefs.getString(clave, "no");
            cuantas = cuantas + 1;
        }
    }

    private void guardar(int indice) {
        String clave = "texto" + indice;
        prefs.edit().putString(clave, textos[indice]).apply();
        clave = "categoria" + indice;
        prefs.edit().putString(clave, categorias[indice]).apply();
        clave = "hecha" + indice;
        prefs.edit().putString(clave, hechas[indice]).apply();
        prefs.edit().putInt("cuantas", cuantas).apply();
    }

    private String nombreFiltro() {
        if (filtro == 1) {
            return "Casa";
        }
        if (filtro == 2) {
            return "Trabajo";
        }
        if (filtro == 3) {
            return "Estudio";
        }
        return "Todas";
    }

    private boolean visible(int i) {
        if (filtro == 0) {
            return true;
        }
        return categorias[i].equals(nombreFiltro());
    }

    private String componer(int i) {
        String marca = "[ ] ";
        boolean lista = hechas[i].equals("si");
        if (lista == true) {
            marca = "[x] ";
        }
        return marca + textos[i] + " (" + categorias[i] + ")";
    }

    private int contarVisibles() {
        int total = 0;
        for (int i = 0; i < cuantas; i++) {
            boolean dentro = visible(i);
            if (dentro == true) {
                total = total + 1;
            }
        }
        return total;
    }

    private int contarPendientes() {
        int total = 0;
        for (int i = 0; i < cuantas; i++) {
            boolean dentro = visible(i);
            if (dentro == true) {
                boolean pendiente = hechas[i].equals("no");
                if (pendiente == true) {
                    total = total + 1;
                }
            }
        }
        return total;
    }

    private String[] copiaVisible() {
        int n = contarVisibles();
        String[] copia = new String[n];
        int siguiente = 0;
        for (int i = 0; i < cuantas; i++) {
            boolean dentro = visible(i);
            if (dentro == true) {
                copia[siguiente] = componer(i);
                siguiente = siguiente + 1;
            }
        }
        return copia;
    }

    private void pintarLista() {
        String[] copia = copiaVisible();
        adaptador = new ArrayAdapter(this, R.layout.item, copia);
        lista.setAdapter(adaptador);
    }

    private void redibujar() {
        pintarLista();
        int visibles = contarVisibles();
        int pendientes = contarPendientes();
        String texto = "Filtro: " + nombreFiltro() + " · " + visibles + " · pendientes " + pendientes;
        etiqueta.setText(texto);
    }

    // El texto de la tarea nueva y su categoría salen de dos campos: el de al lado del teclado y el
    // que se elige con los tres botones. Al agregar se queda seleccionada la misma categoría.
    private void agregar() {
        String texto = nueva.getEditableText().toString();
        if (texto.length() == 0) {
            return;
        }
        if (cuantas >= MAXIMO) {
            return;
        }
        textos[cuantas] = texto;
        categorias[cuantas] = nombreFiltro();
        boolean sinFiltro = categorias[cuantas].equals("Todas");
        if (sinFiltro == true) {
            categorias[cuantas] = "Casa";
        }
        hechas[cuantas] = "no";
        guardar(cuantas);
        cuantas = cuantas + 1;
        prefs.edit().putInt("cuantas", cuantas).apply();
        nueva.setText("");
        redibujar();
    }

    // Marca la primera pendiente que el filtro deja ver, y no la primera de la lista completa: con
    // el filtro en Trabajo, completar la primera de Casa sería un efecto invisible.
    private void completar() {
        for (int i = 0; i < cuantas; i++) {
            boolean dentro = visible(i);
            if (dentro == true) {
                boolean pendiente = hechas[i].equals("no");
                if (pendiente == true) {
                    hechas[i] = "si";
                    guardar(i);
                    redibujar();
                    return;
                }
            }
        }
    }

    private void elegirCategoria(int cual) {
        filtro = cual;
        redibujar();
    }

    @Override
    public void onClick(View vista) {
        int cual = vista.getId();
        if (cual == R.id.casa) {
            elegirCategoria(1);
        } else if (cual == R.id.trabajo) {
            elegirCategoria(2);
        } else if (cual == R.id.estudio) {
            elegirCategoria(3);
        } else if (cual == R.id.agregar) {
            agregar();
        } else if (cual == R.id.completar) {
            completar();
        } else {
            int siguiente = filtro + 1;
            if (siguiente > 3) {
                siguiente = 0;
            }
            elegirCategoria(siguiente);
        }
    }
}
