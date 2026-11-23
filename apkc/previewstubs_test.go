package main

// Pruebas de los stubs que las apps de ejemplo destaparon: ArrayList, BluetoothAdapter, el reloj
// monotónico y la cancelación de tareas de un Handler.
//
// Las cuatro APIs se emulan con estado por instancia y reloj lógico, sin red ni dispositivo: el
// ArrayList guarda sus elementos, el adaptador de Bluetooth se modela encendido y con cero equipos
// vinculados, `SystemClock.elapsedRealtime` lee el reloj de la vista previa y `removeCallbacks`
// retira de la cola las tareas del mismo Runnable. Cada prueba comprueba que la llamada NO cae en
// el aviso de «no está en el dex ni en los stubs» y que dos ejecuciones del mismo guion dan el
// mismo JSON.

import (
	"encoding/json"
	"strings"
	"testing"
)

// sinAvisoDeStub falla si la ejecución avisó de una API que el intérprete no reconoce: es el síntoma
// que tenían estas cuatro antes de tener su stub.
func sinAvisoDeStub(t *testing.T, m *modeloVista) {
	t.Helper()
	for _, a := range m.Ejecucion.Avisos {
		if strings.Contains(a, "no está en el dex ni en los stubs") {
			t.Errorf("la API cayó en el aviso de stub ausente: %s", a)
		}
	}
}

// mismaEjecucion reejecuta el guion y exige el mismo JSON: el reloj lógico y el estado de los stubs
// no pueden depender de nada de fuera del guion.
func mismaEjecucion(t *testing.T, apk []byte, eventos []eventoUsuario) {
	t.Helper()
	primera, err := modeloVistaDeAPKConEventos(apk, "", true, eventos)
	if err != nil {
		t.Fatalf("la primera ejecución falló: %v", err)
	}
	segunda, err := modeloVistaDeAPKConEventos(apk, "", true, eventos)
	if err != nil {
		t.Fatalf("la segunda ejecución falló: %v", err)
	}
	jsonPrimera, err := json.Marshal(primera)
	if err != nil {
		t.Fatalf("el JSON de la primera ejecución: %v", err)
	}
	jsonSegunda, err := json.Marshal(segunda)
	if err != nil {
		t.Fatalf("el JSON de la segunda ejecución: %v", err)
	}
	if string(jsonPrimera) != string(jsonSegunda) {
		t.Errorf("dos ejecuciones del mismo guion dan JSON distinto:\n%s\n%s", jsonPrimera, jsonSegunda)
	}
}

// fuenteLista usa el ArrayList como lo harían `notas`/`tareas`: añade, inserta en un índice, lee
// por índice, cuenta, vacía y pregunta si está vacío, con dos instancias para que el estado no se
// comparta.
const fuenteLista = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    ArrayList lista;
    ArrayList otra;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        preparar();
        String resumen = describir() + vaciar();
        total.setText(resumen);
    }

    private void preparar() {
        lista = new ArrayList();
        lista.add("uno");
        lista.add("dos");
        lista.add("tres");
        lista.add(1, "medio");
        otra = new ArrayList();
        otra.add("x");
    }

    private String describir() {
        String texto = primeros();
        texto = texto + "," + tercero();
        return texto + ",tam" + tamanos();
    }

    private String primeros() {
        return leer(0) + "," + leer(1);
    }

    private String tercero() {
        return leer(3);
    }

    private String leer(int i) {
        return (String) lista.get(i);
    }

    private String tamanos() {
        return "" + lista.size() + "/" + otra.size();
    }

    private String vaciar() {
        lista.clear();
        String texto = "";
        if (lista.isEmpty() == true) {
            texto = ",vacia";
        }
        return texto;
    }
}
`

// TestElArrayListGuardaSuEstadoPorInstancia: el índice manda (el `add(1, …)` queda en medio), el
// tamaño es el de cada lista, `clear` vacía solo la suya y `isEmpty` lo dice.
func TestPreviewElArrayListGuardaSuEstadoPorInstancia(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteLista})
	m, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "uno,medio,tres,tam4/1,vacia" {
		t.Errorf("el ArrayList dejó %q y se esperaba «uno,medio,tres,tam4/1,vacia»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, nil)
}

// fuenteBluetooth recorre el camino de `chatbt`: adaptador presente y encendido, cero equipos
// vinculados, un iterador vacío y el descubrimiento lanzado.
const fuenteBluetooth = `package com.hola.reloj;

import android.app.*;
import android.bluetooth.BluetoothAdapter;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    TextView total;
    BluetoothAdapter adaptador;
    String texto;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        texto = "sin adaptador";
        adaptador = BluetoothAdapter.getDefaultAdapter();
        if (adaptador != null) {
            comprobar();
        }
        total.setText(texto);
    }

    private void comprobar() {
        texto = "apagado";
        if (adaptador.isEnabled() == true) {
            listar();
        }
    }

    private void listar() {
        texto = "listo";
        int cuantos = contar();
        if (hayEquipos() == true) {
            texto = texto + " con equipos";
        }
        texto = texto + " · " + cuantos + " vínculos";
        if (adaptador.startDiscovery() == true) {
            texto = texto + " · buscando";
        }
    }

    private int contar() {
        Set enlazados = adaptador.getBondedDevices();
        return enlazados.size();
    }

    private boolean hayEquipos() {
        Set enlazados = adaptador.getBondedDevices();
        Iterator it = enlazados.iterator();
        return it.hasNext();
    }
}
`

// TestElAdaptadorDeBluetoothModelaSuEstado: `getDefaultAdapter` no es nulo, `isEnabled` dice que sí,
// `getBondedDevices` rinde un conjunto vacío y `startDiscovery` devuelve true sin abrir nada.
func TestPreviewElAdaptadorDeBluetoothModelaSuEstado(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteBluetooth})
	m, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "listo · 0 vínculos · buscando" {
		t.Errorf("el adaptador de Bluetooth dejó %q y se esperaba «listo · 0 vínculos · buscando»; "+
			"avisos %v", got, m.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, nil)
}

// fuenteCancela postea el mismo Runnable dos veces y lo retira con `removeCallbacks`: el primer
// plazo corre y el segundo no.
const fuenteCancela = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity implements Runnable {
    TextView total;
    Handler manejador;
    int pulsos = 0;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        manejador = new Handler();
        int arranque = (int) SystemClock.elapsedRealtime();
        total.setText("t=" + arranque);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                parar();
            }
        });
        programar();
    }

    private void programar() {
        manejador.postDelayed(this, 100L);
        manejador.postDelayed(this, 200L);
    }

    private void parar() {
        manejador.removeCallbacks(this);
    }

    public void run() {
        pulsos = pulsos + 1;
        int ahora = (int) SystemClock.elapsedRealtime();
        total.setText("pulso " + pulsos + " t=" + ahora);
    }
}
`

// TestElRelojLogicoYElRemoveCallbacks: a los 100 ms corre el primer plazo con el reloj lógico en
// 100, el clic retira el de 200 ms y el avance siguiente no encuentra nada. Dos ejecuciones dan el
// mismo JSON.
func TestPreviewElRelojLogicoYElRemoveCallbacks(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteCancela})
	guion := []eventoUsuario{
		{Tipo: "avanzar", Dato: "100"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
		{Tipo: "avanzar", Dato: "500"},
	}
	m, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("el guion falló: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "pulso 1 t=100" {
		t.Errorf("el cronómetro dejó %q y se esperaba «pulso 1 t=100»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	if m.Ejecucion.Tareas != 1 {
		t.Errorf("la cola ejecutó %d tareas y se esperaba 1: el removeCallbacks no retiró la del "+
			"segundo plazo", m.Ejecucion.Tareas)
	}
	if m.Ejecucion.Reloj != 600 {
		t.Errorf("el reloj quedó en %d y se esperaba 600", m.Ejecucion.Reloj)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, guion)
}

// fuenteCancelaTodo postea dos Runnables distintos y los retira todos con
// `removeCallbacksAndMessages(null)`, que es el botón de reiniciar.
const fuenteCancelaTodo = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity {
    TextView total;
    Handler manejador;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        manejador = new Handler();
        total.setText("en cero");
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                manejador.removeCallbacksAndMessages(null);
            }
        });
        manejador.postDelayed(new Uno(total), 100L);
        manejador.postDelayed(new Dos(total), 200L);
    }
}
`

// fuenteUno y fuenteDos son los dos Runnables que el botón debe cancelar.
const fuenteUno = `package com.hola.reloj;

import android.widget.*;

public class Uno implements Runnable {
    TextView total;

    public Uno(TextView t) {
        total = t;
    }

    public void run() {
        total.setText("uno");
    }
}
`

const fuenteDos = `package com.hola.reloj;

import android.widget.*;

public class Dos implements Runnable {
    TextView total;

    public Dos(TextView t) {
        total = t;
    }

    public void run() {
        total.setText("dos");
    }
}
`

// TestRemoveCallbacksAndMessagesVaciaLaCola: cancelado antes de vencer, ninguno de los dos plazos
// deja su texto y la cola ejecuta cero tareas.
func TestPreviewRemoveCallbacksAndMessagesVaciaLaCola(t *testing.T) {
	apk := apkConFuentes(t,
		fuenteJava{nombre: "Casos.java", contenido: fuenteCancelaTodo},
		fuenteJava{nombre: "Uno.java", contenido: fuenteUno},
		fuenteJava{nombre: "Dos.java", contenido: fuenteDos},
	)
	guion := []eventoUsuario{
		{Tipo: "avanzar", Dato: "50"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
		{Tipo: "avanzar", Dato: "500"},
	}
	m, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("el guion falló: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "en cero" {
		t.Errorf("la cola cancelada dejó %q y se esperaba «en cero»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	if m.Ejecucion.Tareas != 0 {
		t.Errorf("la cola ejecutó %d tareas y se esperaba ninguna", m.Ejecucion.Tareas)
	}
	if m.Ejecucion.Reloj != 550 {
		t.Errorf("el reloj quedó en %d y se esperaba 550", m.Ejecucion.Reloj)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, guion)
}

// fuenteBluetoothSinRadio llama a las dos APIs que android-34 deja invocar sobre el adaptador:
// `listenUsingRfcommWithServiceRecord` y `getRemoteDevice`. `createRfcommSocketToServiceRecord` se
// mudó a `BluetoothDevice` en el SDK, así que esa la cubre la llamada directa al stub.
const fuenteBluetoothSinRadio = `package com.hola.reloj;

import android.app.*;
import android.bluetooth.BluetoothAdapter;
import android.bluetooth.BluetoothDevice;
import android.bluetooth.BluetoothServerSocket;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        BluetoothAdapter adaptador = BluetoothAdapter.getDefaultAdapter();
        UUID uuid = null;
        BluetoothServerSocket servidor = adaptador.listenUsingRfcommWithServiceRecord("eco", uuid);
        BluetoothDevice equipo = adaptador.getRemoteDevice("00:11:22:33:44:55");
        String texto = "";
        if (servidor == null) {
            texto = texto + "servidor nulo";
        }
        if (equipo == null) {
            texto = texto + ",equipo nulo";
        }
        total.setText(texto);
    }
}
`

// TestPreviewRechazaBluetoothSinRadio: las tres APIs sin radio se rechazan por su nombre con el
// motivo de la frontera y devuelven el cero de SU tipo de retorno (socket, servidor de sockets y
// dispositivo), no el de un BluetoothSocket para todas. `createRfcommSocketToServiceRecord` se
// cubre con una llamada directa porque android-34 la expone en `BluetoothDevice`, no en el
// adaptador; las otras dos se ejecutan además dentro de una app.
func TestPreviewRechazaBluetoothSinRadio(t *testing.T) {
	m := &maquinaDalvik{vistos: map[string]bool{}}
	adaptador := &objetoDalvik{clase: "Landroid/bluetooth/BluetoothAdapter;",
		campos: map[string]any{}}
	casos := []struct{ nombre, proto, cero string }{
		{"createRfcommSocketToServiceRecord",
			"(Ljava/util/UUID;)Landroid/bluetooth/BluetoothSocket;", "Landroid/bluetooth/BluetoothSocket;"},
		{"listenUsingRfcommWithServiceRecord",
			"(Ljava/lang/String;Ljava/util/UUID;)Landroid/bluetooth/BluetoothServerSocket;",
			"Landroid/bluetooth/BluetoothServerSocket;"},
		{"getRemoteDevice",
			"(Ljava/lang/String;)Landroid/bluetooth/BluetoothDevice;", "Landroid/bluetooth/BluetoothDevice;"},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub([]string{"Landroid/bluetooth/BluetoothAdapter;"},
			c.nombre, c.proto, nil, adaptador)
		if err != nil {
			t.Fatalf("%s devolvió error: %v", c.nombre, err)
		}
		if !ok {
			t.Fatalf("%s no llegó al stub de Bluetooth", c.nombre)
		}
		if v != nil {
			t.Errorf("%s devolvió %v y se esperaba el cero de %s", c.nombre, v, c.cero)
		}
	}
	avisos := strings.Join(m.avisos, "\n")
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna API cayó en el aviso de stub ausente: %s", avisos)
	}
	for _, c := range casos {
		if !strings.Contains(avisos, c.nombre) {
			t.Errorf("falta el aviso por nombre de %s: %v", c.nombre, m.avisos)
		}
		if !strings.Contains(avisos, "se devuelve el cero de "+c.cero) {
			t.Errorf("falta el cero de %s: %v", c.cero, m.avisos)
		}
	}
	if !strings.Contains(avisos, "no hay radio ni red") {
		t.Errorf("el aviso no dice el motivo de la frontera: %v", m.avisos)
	}

	// y las dos que android-34 deja invocar sobre el adaptador, dentro de una app de verdad
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteBluetoothSinRadio})
	vista, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, vista, idTotal); got != "servidor nulo,equipo nulo" {
		t.Errorf("los rechazos dejaron %q y se esperaba «servidor nulo,equipo nulo»; avisos %v",
			got, vista.Ejecucion.Avisos)
	}
	if !strings.Contains(strings.Join(vista.Ejecucion.Avisos, "\n"), "no hay radio ni red") {
		t.Errorf("la app no recibió el motivo de la frontera: %v", vista.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, vista)
	mismaEjecucion(t, apk, nil)
}

// fuenteListaFueraDeRango pide el índice 9 de una lista de dos dentro de un try: la excepción que
// lanza el stub tiene que llegar al catch del preview como cualquier excepción del dex.
const fuenteListaFueraDeRango = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        ArrayList lista = new ArrayList();
        lista.add("uno");
        lista.add("dos");
        String texto = "sin excepcion";
        try {
            Object cual = lista.get(9);
            texto = "no salto";
        } catch (IndexOutOfBoundsException e) {
            texto = "capturada";
        }
        total.setText(texto);
    }
}
`

// TestPreviewCapturaIndexOutOfBoundsDeArrayList: el get(9) de una lista de dos no es un fallo del
// intérprete, es una IndexOutOfBoundsException que el catch del proyecto captura y convierte en
// texto.
func TestPreviewCapturaIndexOutOfBoundsDeArrayList(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteListaFueraDeRango})
	m, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "capturada" {
		t.Errorf("el catch dejó %q y se esperaba «capturada»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, nil)
}
