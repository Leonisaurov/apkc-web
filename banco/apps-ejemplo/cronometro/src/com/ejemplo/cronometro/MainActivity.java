package com.ejemplo.cronometro;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.os.SystemClock;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.ListView;
import android.widget.TextView;

// Cronómetro con Handler.postDelayed: la propia actividad es el Runnable que se reprograma cada
// 50 ms mientras corre. El tiempo sale de SystemClock.elapsedRealtime() —monotónico, no de pared—,
// así que pausar y reanudar suma sin deriva; el tick solo repinta.
// Además del tiempo se cuentan los pulsos: es lo único que `apkc preview` puede ver del reloj, que
// no finge (no hay SystemClock en sus stubs), y en el aparato sirve para comparar el tick real con
// el lógico de la sonda.
// Las cuentas de tiempo van partidas en métodos pequeños porque cada expresión deja su registro
// reservado para todo el método en el emisor Go.
public class MainActivity extends Activity implements View.OnClickListener, Runnable {
    private static final int MAXIMO = 64;

    private TextView tiempo;
    private TextView estado;
    private ListView lista;
    private ArrayAdapter adaptador;
    private Handler manejador;
    private String[] marcas;
    private int cuantas;
    private long inicio;
    private long acumulado;
    private int corriendo;
    private int pulsos;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        tiempo = (TextView) findViewById(R.id.tiempo);
        estado = (TextView) findViewById(R.id.estado);
        lista = (ListView) findViewById(R.id.vueltas);
        manejador = new Handler();
        marcas = new String[MAXIMO];
        cuantas = 0;
        inicio = 0L;
        acumulado = 0L;
        corriendo = 0;
        pulsos = 0;
        findViewById(R.id.iniciar).setOnClickListener(this);
        findViewById(R.id.pausar).setOnClickListener(this);
        findViewById(R.id.vuelta).setOnClickListener(this);
        findViewById(R.id.reiniciar).setOnClickListener(this);
        pintar();
    }

    private long actual() {
        if (corriendo == 0) {
            return acumulado;
        }
        long ahora = SystemClock.elapsedRealtime();
        long transcurrido = ahora - inicio;
        return acumulado + transcurrido;
    }

    private String dosDigitos(long valor) {
        // `(int)` y concatenación, no `Long.toString(long)`: ese stub no existe en el intérprete de
        // `apkc preview` —devolvía null y la pantalla enseñaba «0null»— y la concatenación con un
        // `long` directo la rechaza el emisor (§88: solo append de entero, cadena y referencia).
        int cifras = (int) valor;
        if (cifras < 10) {
            return "0" + cifras;
        }
        return "" + cifras;
    }

    private String minutos(long total) {
        long valor = total / 60000L;
        return dosDigitos(valor);
    }

    private String segundos(long total) {
        long valor = total % 60000L;
        valor = valor / 1000L;
        return dosDigitos(valor);
    }

    private String centesimas(long total) {
        long valor = total % 60000L;
        valor = valor % 1000L;
        valor = valor / 10L;
        return dosDigitos(valor);
    }

    private String formato(long total) {
        String parteMinutos = minutos(total);
        String parteSegundos = segundos(total);
        String parteCentesimas = centesimas(total);
        return parteMinutos + ":" + parteSegundos + "." + parteCentesimas;
    }

    private void pintar() {
        long total = actual();
        tiempo.setText(formato(total));
        String marcha = "parado";
        if (corriendo == 1) {
            marcha = "corriendo";
        }
        estado.setText(marcha + " · pulsos " + pulsos);
    }

    private void iniciar() {
        if (corriendo == 1) {
            return;
        }
        inicio = SystemClock.elapsedRealtime();
        corriendo = 1;
        manejador.postDelayed(this, 0L);
        pintar();
    }

    private void pausar() {
        if (corriendo == 0) {
            return;
        }
        acumulado = actual();
        corriendo = 0;
        manejador.removeCallbacks(this);
        pintar();
    }

    private void redibujar() {
        String[] copia = new String[cuantas];
        for (int i = 0; i < cuantas; i++) {
            copia[i] = marcas[i];
        }
        adaptador = new ArrayAdapter(this, R.layout.item, copia);
        lista.setAdapter(adaptador);
    }

    // La marca se toma aunque el reloj lógico del emulador valga cero (no finge la hora): mientras
    // el tick haya corrido, el pulso es una marca válida y comparable entre los dos lados.
    private void marcarVuelta() {
        long total = actual();
        if (total == 0L) {
            if (pulsos == 0) {
                return;
            }
        }
        if (cuantas >= MAXIMO) {
            return;
        }
        marcas[cuantas] = formato(total) + " · pulso " + pulsos;
        cuantas = cuantas + 1;
        redibujar();
    }

    private void reiniciar() {
        manejador.removeCallbacks(this);
        corriendo = 0;
        acumulado = 0L;
        inicio = SystemClock.elapsedRealtime();
        pulsos = 0;
        cuantas = 0;
        redibujar();
        pintar();
    }

    @Override
    public void run() {
        if (corriendo == 1) {
            pulsos = pulsos + 1;
            pintar();
            manejador.postDelayed(this, 50L);
        }
    }

    @Override
    public void onClick(View vista) {
        int cual = vista.getId();
        if (cual == R.id.iniciar) {
            iniciar();
        } else if (cual == R.id.pausar) {
            pausar();
        } else if (cual == R.id.vuelta) {
            marcarVuelta();
        } else {
            reiniciar();
        }
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        manejador.removeCallbacks(this);
    }
}
