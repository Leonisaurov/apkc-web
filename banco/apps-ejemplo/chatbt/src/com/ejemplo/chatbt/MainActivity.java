package com.ejemplo.chatbt;

import android.app.Activity;
import android.bluetooth.BluetoothAdapter;
import android.bluetooth.BluetoothDevice;
import android.content.pm.PackageManager;
import android.os.Bundle;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.ListView;
import android.widget.TextView;

import java.util.Iterator;
import java.util.Set;

// Chat Bluetooth: la lista de mensajes, el envío local y el estado del adaptador. La conexión real
// por BluetoothSocket queda como frontera: aquí no se abre ningún socket ni se toca la red; el
// mensaje entra en la lista y un eco local confirma el recorrido del texto.
// En Android 12+ el listado y el descubrimiento necesitan BLUETOOTH_CONNECT/SCAN en tiempo de
// ejecución, así que la app los pide al arrancar y, si no están, lo dice en vez de caerse.
// El adaptador de la lista se reconstruye con una copia del arreglo en cada cambio: es la forma que
// `apkc preview` pinta y en el aparato equivale a notifyDataSetChanged sin caché que invalidar.
public class MainActivity extends Activity implements View.OnClickListener {
    private static final int MAXIMO = 128;
    private static final int SIN_ADAPTADOR = 0;
    private static final int APAGADO = 1;
    private static final int LISTO = 2;

    private TextView estado;
    private TextView entrada;
    private ListView lista;
    private ArrayAdapter adaptador;
    private BluetoothAdapter adaptadorBt;
    private String[] mensajes;
    private int cuantos;
    private int marcha;
    private int vinculos;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        estado = (TextView) findViewById(R.id.estado);
        entrada = (TextView) findViewById(R.id.entrada);
        lista = (ListView) findViewById(R.id.mensajes);
        mensajes = new String[MAXIMO];
        cuantos = 0;
        marcha = SIN_ADAPTADOR;
        vinculos = 0;
        redibujar();
        findViewById(R.id.enviar).setOnClickListener(this);
        findViewById(R.id.buscar).setOnClickListener(this);
        findViewById(R.id.limpiar).setOnClickListener(this);
        pedirPermisos();
    }

    private void redibujar() {
        String[] copia = new String[cuantos];
        for (int i = 0; i < cuantos; i++) {
            copia[i] = mensajes[i];
        }
        adaptador = new ArrayAdapter(this, R.layout.item, copia);
        lista.setAdapter(adaptador);
        pintarEstado();
    }

    // El estado del adaptador es texto y no solo un color: es lo que comparan los oráculos del
    // emulador y del aparato, que no ven nada más de una conexión que no se abre.
    private void pintarEstado() {
        String texto = "Bluetooth listo";
        if (marcha == SIN_ADAPTADOR) {
            texto = "Este aparato no tiene Bluetooth";
        } else if (marcha == APAGADO) {
            texto = "Bluetooth apagado";
        }
        estado.setText(texto + " · " + vinculos + " vínculos · " + cuantos + " mensajes");
    }

    private void pedirPermisos() {
        int permiso = checkSelfPermission("android.permission.BLUETOOTH_CONNECT");
        if (permiso == PackageManager.PERMISSION_GRANTED) {
            preparar();
            return;
        }
        String[] cuales = new String[] { "android.permission.BLUETOOTH_CONNECT", "android.permission.BLUETOOTH_SCAN" };
        requestPermissions(cuales, 7);
    }

    @Override
    public void onRequestPermissionsResult(int codigo, String[] permisos, int[] resultados) {
        super.onRequestPermissionsResult(codigo, permisos, resultados);
        if (resultados.length > 0) {
            if (resultados[0] == PackageManager.PERMISSION_GRANTED) {
                preparar();
                return;
            }
        }
        pintarEstado();
    }

    private void preparar() {
        adaptadorBt = BluetoothAdapter.getDefaultAdapter();
        if (adaptadorBt == null) {
            marcha = SIN_ADAPTADOR;
            pintarEstado();
            return;
        }
        boolean encendido = adaptadorBt.isEnabled();
        if (encendido == false) {
            marcha = APAGADO;
            pintarEstado();
            return;
        }
        marcha = LISTO;
        vinculos = 0;
        Set enlazados = adaptadorBt.getBondedDevices();
        if (enlazados != null) {
            vinculos = enlazados.size();
        }
        pintarEstado();
    }

    // Un vínculo por línea, con nombre y dirección; el nombre puede faltar y entonces se dice.
    private void listarVinculos() {
        if (marcha != LISTO) {
            preparar();
            return;
        }
        Set enlazados = adaptadorBt.getBondedDevices();
        if (enlazados == null) {
            pintarEstado();
            return;
        }
        Iterator it = enlazados.iterator();
        boolean hay = it.hasNext();
        while (hay == true) {
            BluetoothDevice equipo = (BluetoothDevice) it.next();
            String nombre = equipo.getName();
            String linea = nombre + " " + equipo.getAddress();
            anotar(linea);
            hay = it.hasNext();
        }
        pintarEstado();
    }

    // El descubrimiento se lanza, pero sus resultados no se recogen: registrar un
    // BroadcastReceiver exigiría una clase anónima que extiende una clase (no medida) o asignar una
    // subclase a un campo del supertipo (el emisor lo rechaza por nombre). Frontera documentada.
    private void descubrir() {
        if (marcha != LISTO) {
            return;
        }
        adaptadorBt.startDiscovery();
        estado.setText(R.string.buscando);
    }

    private void anotar(String linea) {
        if (cuantos >= MAXIMO) {
            return;
        }
        mensajes[cuantos] = linea;
        cuantos = cuantos + 1;
    }

    // La concatenación se hace en un local y no en el argumento: el emisor no estima el tipo de una
    // expresión usada como argumento sin emitirla antes (limitación medida con `Evento.getX()`).
    private void anotarDe(String prefijo, String texto) {
        String linea = prefijo + texto;
        anotar(linea);
    }

    // Envío local: el texto entra como «yo» y el eco prueba que el mensaje pasó por la lista. No hay
    // socket, así que el eco lo escribe la propia app y se dice en la línea.
    private void enviar() {
        String texto = entrada.getEditableText().toString();
        if (texto.length() == 0) {
            return;
        }
        anotarDe("yo: ", texto);
        String eco = texto + " (sin socket)";
        anotarDe("eco: ", eco);
        entrada.setText("");
        redibujar();
    }

    private void limpiar() {
        cuantos = 0;
        redibujar();
    }

    @Override
    public void onClick(View vista) {
        int cual = vista.getId();
        if (cual == R.id.enviar) {
            enviar();
        } else if (cual == R.id.limpiar) {
            limpiar();
        } else {
            listarVinculos();
            descubrir();
        }
    }
}
