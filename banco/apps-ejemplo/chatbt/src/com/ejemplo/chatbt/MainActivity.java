package com.ejemplo.chatbt;

import android.app.Activity;
import android.bluetooth.BluetoothAdapter;
import android.bluetooth.BluetoothDevice;
import android.content.Context;
import android.content.pm.PackageManager;
import android.os.Bundle;
import android.view.View;
import android.widget.ArrayAdapter;
import android.widget.ListView;
import android.widget.TextView;

import java.util.Iterator;
import java.util.Set;

// Chat Bluetooth: la UI de mensajes, el estado del adaptador y el descubrimiento de equipos.
// La conexión real por BluetoothSocket queda como frontera: aquí no se abre ningún socket.
// En Android 12+ el listado y el descubrimiento necesitan BLUETOOTH_CONNECT/SCAN en tiempo de
// ejecución, así que la app los pide al arrancar y, si no están, lo dice en vez de caerse.
public class MainActivity extends Activity implements View.OnClickListener {
    private TextView estado;
    private TextView entrada;
    private ArrayAdapter mensajes;
    private BluetoothAdapter adaptadorBt;

    @Override
    public void onCreate(Bundle guardado) {
        super.onCreate(guardado);
        setContentView(R.layout.principal);
        estado = (TextView) findViewById(R.id.estado);
        entrada = (TextView) findViewById(R.id.entrada);
        ListView lista = (ListView) findViewById(R.id.mensajes);
        mensajes = new ArrayAdapter(this, R.layout.item);
        lista.setAdapter(mensajes);
        findViewById(R.id.enviar).setOnClickListener(this);
        findViewById(R.id.buscar).setOnClickListener(this);
        pedirPermisos();
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
        estado.setText(R.string.sin_permiso);
    }

    private void preparar() {
        adaptadorBt = BluetoothAdapter.getDefaultAdapter();
        if (adaptadorBt == null) {
            estado.setText(R.string.sin_bt);
            return;
        }
        if (adaptadorBt.isEnabled() == false) {
            estado.setText(R.string.apagado);
            return;
        }
        estado.setText(R.string.listo);
    }

    private void listarVinculos() {
        Set vinculos = adaptadorBt.getBondedDevices();
        Iterator it = vinculos.iterator();
        boolean hay = it.hasNext();
        while (hay == true) {
            BluetoothDevice equipo = (BluetoothDevice) it.next();
            String nombre = equipo.getName();
            String linea = nombre + " " + equipo.getAddress();
            mensajes.add(linea);
            hay = it.hasNext();
        }
    }

    // El descubrimiento se lanza, pero sus resultados no se recogen: registrar un
    // BroadcastReceiver exigiría una clase anónima que extiende una clase (no medida) o asignar una
    // subclase a un campo del supertipo (el emisor lo rechaza por nombre). Frontera documentada.
    private void descubrir() {
        adaptadorBt.startDiscovery();
        estado.setText(R.string.buscando);
    }

    private void buscar() {
        if (adaptadorBt == null) {
            preparar();
            return;
        }
        listarVinculos();
        descubrir();
    }

    private void enviar() {
        String texto = entrada.getText().toString();
        if (texto.length() == 0) {
            return;
        }
        mensajes.add("yo: " + texto);
        entrada.setText("");
        if (adaptadorBt == null) {
            estado.setText(R.string.sin_conexion);
        }
    }

    @Override
    public void onClick(View vista) {
        if (vista.getId() == R.id.enviar) {
            enviar();
        } else {
            buscar();
        }
    }
}
