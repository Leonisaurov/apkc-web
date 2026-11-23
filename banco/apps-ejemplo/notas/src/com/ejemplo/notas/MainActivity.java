package com.ejemplo.notas;

import android.app.Activity;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.ListView;
import android.widget.TextView;

// Bloc de notas: crear, editar y borrar con SharedPreferences. Las notas viven en un arreglo fijo y
// un contador, y se guardan en una clave por nota (nota0, nota1…) más «cuantas»; al borrar se
// reescriben todas las claves para que no queden huecos.
// El adaptador se vuelve a construir con una copia del arreglo en cada cambio: es la forma que
// `apkc preview` pinta (el ArrayAdapter del framework solo enseña los elementos que recibe en su
// constructor) y en el aparato es lo mismo que hace notifyDataSetChanged, sin caché que invalidar.
// La selección se mueve con dos botones y no con clics en la lista, para que la secuencia de la
// sonda sea la misma en el emulador y en el aparato.
public class MainActivity extends Activity implements View.OnClickListener {
    private static final int MAXIMO = 64;

    private TextView editor;
    private TextView seleccion;
    private ListView lista;
    private ArrayAdapter adaptador;
    private SharedPreferences prefs;
    private String[] notas;
    private int cuantas;
    private int elegida;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        editor = (TextView) findViewById(R.id.editor);
        seleccion = (TextView) findViewById(R.id.seleccion);
        lista = (ListView) findViewById(R.id.lista);
        prefs = getSharedPreferences("notas", 0);
        notas = new String[MAXIMO];
        cuantas = 0;
        elegida = -1;
        cargar();
        redibujar();
        findViewById(R.id.anterior).setOnClickListener(this);
        findViewById(R.id.siguiente).setOnClickListener(this);
        findViewById(R.id.nueva).setOnClickListener(this);
        findViewById(R.id.guardar).setOnClickListener(this);
        findViewById(R.id.borrar).setOnClickListener(this);
        pintarSeleccion();
    }

    private void cargar() {
        int guardadas = prefs.getInt("cuantas", 0);
        for (int i = 0; i < guardadas; i++) {
            if (cuantas >= MAXIMO) {
                return;
            }
            String clave = "nota" + i;
            String nota = prefs.getString(clave, "");
            notas[cuantas] = nota;
            cuantas = cuantas + 1;
        }
    }

    private void redibujar() {
        String[] copia = new String[cuantas];
        for (int i = 0; i < cuantas; i++) {
            copia[i] = notas[i];
        }
        adaptador = new ArrayAdapter(this, R.layout.item, copia);
        lista.setAdapter(adaptador);
    }

    private void persistir() {
        for (int i = 0; i < cuantas; i++) {
            String clave = "nota" + i;
            prefs.edit().putString(clave, notas[i]).apply();
        }
        prefs.edit().putInt("cuantas", cuantas).apply();
    }

    private void pintarSeleccion() {
        if (elegida < 0) {
            seleccion.setText("Nota nueva · hay " + cuantas);
            return;
        }
        String texto = "Nota " + (elegida + 1) + " de " + cuantas;
        seleccion.setText(texto);
    }

    private void mostrarElegida() {
        if (elegida < 0) {
            editor.setText("");
            return;
        }
        editor.setText(notas[elegida]);
    }

    private void mover(int paso) {
        if (cuantas == 0) {
            return;
        }
        int destino = elegida + paso;
        if (destino < 0) {
            destino = cuantas - 1;
        }
        if (destino >= cuantas) {
            destino = -1;
        }
        elegida = destino;
        mostrarElegida();
        pintarSeleccion();
    }

    private void nueva() {
        elegida = -1;
        editor.setText("");
        pintarSeleccion();
    }

    // `getEditableText()` y no `getText()`: el segundo tiene dos sobrecargas en `EditText` y el
    // emisor no elige la más específica de este subconjunto (limitación medida).
    private void guardar() {
        String texto = editor.getEditableText().toString();
        if (texto.length() == 0) {
            return;
        }
        if (elegida < 0) {
            if (cuantas >= MAXIMO) {
                return;
            }
            notas[cuantas] = texto;
            cuantas = cuantas + 1;
            elegida = cuantas - 1;
        } else {
            notas[elegida] = texto;
        }
        persistir();
        redibujar();
        pintarSeleccion();
    }

    private void borrar() {
        if (cuantas == 0) {
            return;
        }
        int cual = elegida;
        if (cual < 0) {
            cual = cuantas - 1;
        }
        for (int i = cual; i < cuantas - 1; i++) {
            notas[i] = notas[i + 1];
        }
        cuantas = cuantas - 1;
        notas[cuantas] = null;
        persistir();
        // la clave que quedó detrás del contador se borra: no basta con no leerla, porque un
        // `cuantas` restaurado desde otra sesión la resucitaría
        String sobrante = "nota" + cuantas;
        prefs.edit().remove(sobrante).apply();
        redibujar();
        elegida = cual;
        if (elegida >= cuantas) {
            elegida = cuantas - 1;
        }
        mostrarElegida();
        pintarSeleccion();
    }

    @Override
    public void onClick(View vista) {
        int cual = vista.getId();
        if (cual == R.id.anterior) {
            mover(-1);
        } else if (cual == R.id.siguiente) {
            mover(1);
        } else if (cual == R.id.nueva) {
            nueva();
        } else if (cual == R.id.guardar) {
            guardar();
        } else {
            borrar();
        }
    }
}
