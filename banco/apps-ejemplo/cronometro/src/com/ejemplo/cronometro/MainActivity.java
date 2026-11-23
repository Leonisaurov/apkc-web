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
// 50 ms mientras corre. Las cuentas de tiempo van partidas en métodos pequeños porque cada
// expresión deja su registro reservado para todo el método en el emisor Go.
public class MainActivity extends Activity implements View.OnClickListener, Runnable {
    private TextView tiempo;
    private ArrayAdapter vueltas;
    private Handler manejador;
    private long inicio;
    private long acumulado;
    private int corriendo;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        tiempo = (TextView) findViewById(R.id.tiempo);
        ListView lista = (ListView) findViewById(R.id.vueltas);
        vueltas = new ArrayAdapter(this, R.layout.item);
        lista.setAdapter(vueltas);
        manejador = new Handler();
        inicio = 0L;
        acumulado = 0L;
        corriendo = 0;
        findViewById(R.id.iniciar).setOnClickListener(this);
        findViewById(R.id.parar).setOnClickListener(this);
        findViewById(R.id.vuelta).setOnClickListener(this);
    }

    private long actual() {
        long ahora = SystemClock.elapsedRealtime();
        long transcurrido = ahora - inicio;
        return acumulado + transcurrido;
    }

    private String dosDigitos(long valor) {
        String cifras = Long.toString(valor);
        if (valor < 10L) {
            return "0" + cifras;
        }
        return cifras;
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
        String texto = parteMinutos + ":" + parteSegundos + "." + parteCentesimas;
        return texto;
    }

    private void pintar() {
        long total = actual();
        tiempo.setText(formato(total));
    }

    private void iniciar() {
        if (corriendo == 1) {
            return;
        }
        inicio = SystemClock.elapsedRealtime();
        corriendo = 1;
        manejador.postDelayed(this, 0L);
    }

    private void parar() {
        if (corriendo == 0) {
            return;
        }
        acumulado = actual();
        corriendo = 0;
        pintar();
    }

    private void marcarVuelta() {
        long total = actual();
        vueltas.add(formato(total));
    }

    @Override
    public void run() {
        if (corriendo == 1) {
            pintar();
            manejador.postDelayed(this, 50L);
        }
    }

    @Override
    public void onClick(View vista) {
        int cual = vista.getId();
        if (cual == R.id.iniciar) {
            iniciar();
        } else if (cual == R.id.parar) {
            parar();
        } else {
            marcarVuelta();
        }
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        manejador.removeCallbacks(this);
    }
}
