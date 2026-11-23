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
