package main

// La prueba de la fase E6 del plan de emulación: hilos y temporizadores controlados.
//
// No hay hilos ni reloj de pared. La app postea tareas con `Handler.postDelayed` (y `View.post`,
// `runOnUiThread`, `AsyncTask.execute`) y el intérprete las encola con su instante en milisegundos;
// solo `--avanzar N` mueve el reloj lógico y las ejecuta cuando su instante llega. Dos ejecuciones
// del mismo guion tienen que dar el mismo JSON, y eso es lo que comprueba la prueba.
//
// El dex y el R.java se generan en caliente con el emisor del fork, como en el resto de la fase.

import (
	"encoding/json"
	"flag"
	"io"
	"math"
	"strings"
	"testing"
)

// Los `idRef` de las vistas que la prueba consulta, con el nombre de la clave delante para que se
// lea qué vista es: la tabla ordena las claves y aapt2 numera en ese orden.
const (
	idOrden = "0x7f010000"
	idTotal = "0x7f010002"
	idUI    = "0x7f010003"
)

// tablaDelReloj es la tabla del apk de las pruebas: cuatro ids y el layout, con la ruta en el pool
// global, que es lo que resuelve un id de `R.layout`.
func tablaDelReloj() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.reloj",
		global:        []string{"res/layout/main.xml"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout"},
		nombresUTF8:   false,
		claves:        []string{"orden", "pulsar", "total", "ui", "main"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{
				id:        1,
				specFlags: []uint32{0, 0, 0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 1, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 2, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 3, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
				}}},
			},
			{
				id:        2,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 4, valor: valorEscribir{tipo: 0x03, datos: 0}},
				}}},
			},
		},
	}
}

// layoutDelReloj es el layout del apk: el contador, la vista que escribe `runOnUiThread`, la que
// escriben las fases del AsyncTask y el botón. `texto` vacío no escribe `android:text`: el inflado
// deja la vista sin texto propio y lo que se ve es solo lo que la app escriba.
func layoutDelReloj() documentoAXML {
	texto := func(nombre, contenido string) *elementoEscribir {
		attrs := []atributoEscribir{
			{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia,
				datos: idDeClaveDelReloj(nombre)},
			{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
			{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
		}
		if contenido != "" {
			attrs = append(attrs, atributoEscribir{ns: "android", nombre: "text", id: 0x0101014f,
				tipo: tipoCadena, texto: contenido})
		}
		return &elementoEscribir{nombre: "TextView", linea: 3, attrs: attrs}
	}
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: 0x010100c4, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				texto("total", "Tic: 0"),
				texto("ui", ""),
				texto("orden", ""),
				{
					nombre: "Button",
					linea:  20,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia,
							datos: idDeClaveDelReloj("pulsar")},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Pulsar"},
					},
				},
			},
		},
	}
}

// idDeClaveDelReloj lleva el nombre de un id de la tabla del reloj a su id de recurso: es la misma
// cuenta que hace aapt2 (0x7f, tipo 1 y la posición de la clave entre las ordenadas por nombre).
func idDeClaveDelReloj(clave string) uint32 {
	switch clave {
	case "orden":
		return 0x7f010000
	case "pulsar":
		return 0x7f010001
	case "total":
		return 0x7f010002
	case "ui":
		return 0x7f010003
	}
	return 0
}

// fuenteReloj es la actividad de la prueba: en `onCreate` arma la cadena de un tic por segundo con
// `postDelayed`, escribe desde `runOnUiThread` y lanza un AsyncTask. El botón reescribe la vista de
// `runOnUiThread`, que es lo que permite comprobar que el orden de clics y avances se respeta.
const fuenteReloj = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity {
    int tics = 0;
    TextView total;
    TextView ui;
    TextView orden;
    Handler reloj;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        ui = (TextView) findViewById(R.id.ui);
        orden = (TextView) findViewById(R.id.orden);
        reloj = new Handler();
        programa();
        runOnUiThread(new Runnable() {
            public void run() {
                ui.setText("ya");
            }
        });
        new Tarea(orden).execute("x");
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                ui.setText("clic");
            }
        });
    }

    private void programa() {
        reloj.postDelayed(new Runnable() {
            public void run() {
                tics = tics + 1;
                total.setText("Tic: " + tics);
                programa();
            }
        }, 1000L);
    }
}
`

// fuenteTarea es el AsyncTask de la prueba, con las tres fases escribiendo en la misma vista para
// que el orden quede en el modelo.
const fuenteTarea = `package com.hola.reloj;

import android.os.*;
import android.widget.*;

public class Tarea extends AsyncTask {
    TextView salida;

    public Tarea(TextView s) {
        salida = s;
    }

    protected void onPreExecute() {
        salida.append("pre;");
    }

    protected Object doInBackground(Object[] params) {
        salida.append("curro;");
        return "listo";
    }

    protected void onPostExecute(Object resultado) {
        salida.append("post(" + resultado + ");");
    }
}
`

// fuenteQuieta es una actividad que no postea nada: sirve para medir el reloj solo, sin la cola de
// tareas del tic.
const fuenteQuieta = `package com.hola.reloj;

import android.app.*;
import android.os.*;

public class Casos extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
    }
}
`

// fuenteRetraso es la sonda del retraso negativo: la actividad postea el padre con -1000 ms y el
// padre, cuando corre, postea la hija con +1100. Con el retraso acotado a cero, el padre corre en el
// instante 0 y la hija queda en 1100; sin acotarlo, el padre correría en -1000 y la hija se
// adelantaría a 100.
const fuenteRetraso = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.widget.*;

public class Casos extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        Handler reloj = new Handler();
        reloj.postDelayed(new Padre(total, reloj), -1000L);
    }
}
`

// fuentePadre es el Runnable que se postea con retraso negativo y reprograma a su hija.
const fuentePadre = `package com.hola.reloj;

import android.os.*;
import android.widget.*;

public class Padre implements Runnable {
    TextView total;
    Handler reloj;

    public Padre(TextView t, Handler r) {
        total = t;
        reloj = r;
    }

    public void run() {
        total.setText("1");
        reloj.postDelayed(new Hija(total), 1100L);
    }
}
`

// fuenteHija es la tarea que el padre deja a 1100 ms: si el reloj retrocediera, correría antes.
const fuenteHija = `package com.hola.reloj;

import android.widget.*;

public class Hija implements Runnable {
    TextView total;

    public Hija(TextView t) {
        total = t;
    }

    public void run() {
        total.setText("2");
    }
}
`

// fuenteLejana es la actividad de la sonda del retraso desbordado: el botón postea una tarea con el
// retraso máximo, que solo cabe si el reloj está en cero.
const fuenteLejana = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity {
    TextView total;
    Handler reloj;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        reloj = new Handler();
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                postLejana();
            }
        });
    }

    private void postLejana() {
        reloj.postDelayed(new Lejana(total), 9223372036854775807L);
    }
}
`

// fuenteTareaLejana es la tarea del retraso máximo: si el reloj envolviera, correría de inmediato y
// dejaría su marca en la pantalla.
const fuenteTareaLejana = `package com.hola.reloj;

import android.widget.*;

public class Lejana implements Runnable {
    TextView total;

    public Lejana(TextView t) {
        total = t;
    }

    public void run() {
        total.setText("M");
    }
}
`

// fuenteTics es el tic de un segundo sin nada más: el AsyncTask del arranque de fuenteReloj contaría
// como tarea y dejaría el avance largo justo en el borde de la salvaguarda.
const fuenteTics = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.widget.*;

public class Casos extends Activity {
    int tics = 0;
    TextView total;
    Handler reloj;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        reloj = new Handler();
        programa();
    }

    private void programa() {
        reloj.postDelayed(new Runnable() {
            public void run() {
                tics = tics + 1;
                total.setText("Tic: " + tics);
                programa();
            }
        }, 1000L);
    }
}
`

// apkDelReloj arma en memoria el apk de la prueba: manifiesto, tabla, layout y el dex de las dos
// fuentes.
func apkDelReloj(t *testing.T) []byte {
	t.Helper()
	return apkConFuentes(t,
		fuenteJava{nombre: "Casos.java", contenido: fuenteReloj},
		fuenteJava{nombre: "Tarea.java", contenido: fuenteTarea},
	)
}

// apkConFuentes es el armazón común: el mismo manifiesto, la misma tabla y el mismo layout, con el
// dex de las fuentes que pida cada prueba. El R.java sale siempre de la tabla, para que las fuentes
// usen los ids reales.
func apkConFuentes(t *testing.T, fuentes ...fuenteJava) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelReloj())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelReloj())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDelReloj())
	if err != nil {
		t.Fatalf("el R.java no sale de la tabla: %v", err)
	}
	todas := append([]fuenteJava{{nombre: "R.java", contenido: r}}, fuentes...)
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, todas...),
	})
}

// textoDeNodo busca el `textoDinamico` del nodo con ese idRef en el modelo.
func textoDeNodo(t *testing.T, m *modeloVista, idRef string) string {
	t.Helper()
	var busca func(n *nodoVista) (string, bool)
	busca = func(n *nodoVista) (string, bool) {
		if n == nil {
			return "", false
		}
		if n.IDRef == idRef {
			return n.TextoDinamico, true
		}
		for _, h := range n.Hijos {
			if v, ok := busca(h); ok {
				return v, true
			}
		}
		return "", false
	}
	if v, ok := busca(m.Raiz); ok {
		return v
	}
	t.Fatalf("el modelo no trae el nodo %s: %+v", idRef, m.Raiz)
	return ""
}

// TestElPreviewAvanzaElRelojLogico es el criterio de aceptación de E6: el contador que se
// reprograma cada 1000 ms vale 3 tras `--avanzar 3000`; `runOnUiThread` corre en el acto y las tres
// fases del AsyncTask dejan su huella en orden; y el mismo guion, dos veces seguidas, da el mismo
// JSON.
func TestElPreviewAvanzaElRelojLogico(t *testing.T) {
	apk := apkDelReloj(t)

	// sin avanzar el reloj no corre ninguna tarea diferida: la pantalla es la de `onCreate`
	reposo, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk en reposo no se lee: %v", err)
	}
	if got := textoDeNodo(t, reposo, idTotal); got != "" {
		t.Errorf("sin --avanzar el contador ya vale %q", got)
	}
	if reposo.Ejecucion.Reloj != 0 || reposo.Ejecucion.Tareas != 1 {
		t.Errorf("en reposo el reloj es %d y las tareas %d; se esperaba 0 y la del AsyncTask del "+
			"arranque (el tic espera a los 1000 ms y runOnUiThread no pasa por la cola)",
			reposo.Ejecucion.Reloj, reposo.Ejecucion.Tareas)
	}
	// runOnUiThread corre en el acto, como en el aparato cuando se llama desde el hilo de la interfaz
	if got := textoDeNodo(t, reposo, idUI); got != "ya" {
		t.Errorf("runOnUiThread dejó %q y se esperaba «ya»", got)
	}
	// el AsyncTask corre en el mismo instante cero, con sus tres fases en orden
	if got := textoDeNodo(t, reposo, idOrden); got != "pre;curro;post(listo);" {
		t.Errorf("el AsyncTask dejó %q y se esperaba «pre;curro;post(listo);»; avisos %v",
			got, reposo.Ejecucion.Avisos)
	}

	// el criterio: un tic por segundo, tres segundos, tres tics
	guion := []eventoUsuario{{Tipo: "avanzar", Dato: "3000"}}
	tres, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("--avanzar 3000 falló: %v", err)
	}
	if got := textoDeNodo(t, tres, idTotal); got != "Tic: 3" {
		t.Errorf("tras --avanzar 3000 el contador vale %q y se esperaba «Tic: 3»", got)
	}
	if tres.Ejecucion.Reloj != 3000 {
		t.Errorf("el reloj final es %d y se esperaba 3000", tres.Ejecucion.Reloj)
	}
	// tres tics más el AsyncTask del arranque
	if tres.Ejecucion.Tareas != 4 {
		t.Errorf("la cola ejecutó %d tareas y se esperaban 4", tres.Ejecucion.Tareas)
	}

	// un segundo y medio: solo el tic de 1000, que se reprograma a 2000 y ya no llega
	uno, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{{Tipo: "avanzar", Dato: "1500"}})
	if err != nil {
		t.Fatalf("--avanzar 1500 falló: %v", err)
	}
	if got := textoDeNodo(t, uno, idTotal); got != "Tic: 1" {
		t.Errorf("tras --avanzar 1500 el contador vale %q y se esperaba «Tic: 1»", got)
	}

	// dos avances seguidos suman, como si se hubiera pedido uno solo
	sumados, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
		{Tipo: "avanzar", Dato: "1000"},
		{Tipo: "avanzar", Dato: "2000"},
	})
	if err != nil {
		t.Fatalf("los dos avances fallaron: %v", err)
	}
	if got := textoDeNodo(t, sumados, idTotal); got != "Tic: 3" {
		t.Errorf("tras --avanzar 1000 --avanzar 2000 el contador vale %q", got)
	}

	// el guion se entrega en el orden pedido: avanzar y después pulsar deja «clic», al revés deja
	// «clic» también, así que se mide el reloj y las pulsaciones, que es lo que distingue el orden
	mixto := []eventoUsuario{
		{Tipo: "avanzar", Dato: "2000"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
		{Tipo: "avanzar", Dato: "1000"},
	}
	despues, err := modeloVistaDeAPKConEventos(apk, "", true, mixto)
	if err != nil {
		t.Fatalf("el guion mixto falló: %v", err)
	}
	if got := textoDeNodo(t, despues, idTotal); got != "Tic: 3" {
		t.Errorf("el guion mixto deja el contador en %q y se esperaba «Tic: 3»", got)
	}
	if got := textoDeNodo(t, despues, idUI); got != "clic" {
		t.Errorf("el clic del guion mixto dejó %q", got)
	}
	if len(despues.Ejecucion.Pulsaciones) != 1 || despues.Ejecucion.Pulsaciones[0] != "id/pulsar" {
		t.Errorf("las pulsaciones del guion mixto son %v", despues.Ejecucion.Pulsaciones)
	}

	// determinismo: el mismo guion, dos ejecuciones seguidas, el mismo JSON
	primera, err := modeloVistaDeAPKConEventos(apk, "", true, mixto)
	if err != nil {
		t.Fatalf("la primera ejecución del guion falló: %v", err)
	}
	segunda, err := modeloVistaDeAPKConEventos(apk, "", true, mixto)
	if err != nil {
		t.Fatalf("la segunda ejecución del guion falló: %v", err)
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
	// y el guion mixto deja el mismo contador que avanzar el total de una vez
	if got := textoDeNodo(t, primera, idTotal); got != "Tic: 3" {
		t.Errorf("la ejecución repetida deja el contador en %q", got)
	}
}

// TestElGuionDelPreviewConservaElOrden comprueba la parte del comando: `--avanzar` y `--pulsar`
// comparten una sola lista de eventos, así que el orden de la línea de comandos llega intacto al
// intérprete, y un avance que no es un entero no negativo se rechaza al parsear, no a mitad de la
// ejecución.
func TestElGuionDelPreviewConservaElOrden(t *testing.T) {
	set := flag.NewFlagSet("preview", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	guion := []eventoUsuario{}
	set.Var(guionDeEntrada{tipo: "pulsar", eventos: &guion}, "pulsar", "")
	set.Var(guionDeEntrada{tipo: "avanzar", eventos: &guion}, "avanzar", "")
	err := set.Parse([]string{"--avanzar", "1000", "--pulsar", "id/pulsar", "--avanzar", "2000"})
	if err != nil {
		t.Fatalf("el guion no se parsea: %v", err)
	}
	esperado := []eventoUsuario{
		{Tipo: "avanzar", Dato: "1000"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
		{Tipo: "avanzar", Dato: "2000"},
	}
	if len(guion) != len(esperado) {
		t.Fatalf("el guion tiene %d pasos y se esperaban %d: %+v", len(guion), len(esperado), guion)
	}
	for i := range esperado {
		// campo a campo: el struct del guion lleva el dex de un hot swap como []byte y no es
		// comparable con `==`, pero comparar solo Tipo/Dato dejaría sin verificar los demás campos
		if !mismoEvento(guion[i], esperado[i]) {
			t.Errorf("el paso %d es %+v y se esperaba %+v", i, guion[i], esperado[i])
		}
	}
	if err := set.Parse([]string{"--avanzar", "-1"}); err == nil {
		t.Error("--avanzar -1 se aceptó y el reloj lógico no puede ir hacia atrás")
	}
	if err := set.Parse([]string{"--avanzar", "medio"}); err == nil {
		t.Error("--avanzar medio se aceptó y no son milisegundos")
	}
}

// TestElRelojNoDesbordaEnDifiere es la sonda del retraso que no cabe: con el reloj en 100 ms, un
// `postDelayed(MaxInt64)` no puede envolver a un instante pasado. La tarea queda en el último
// instante representable, se avisa, y ni corre ni deja su marca en la pantalla.
func TestElRelojNoDesbordaEnDifiere(t *testing.T) {
	apk := apkConFuentes(t,
		fuenteJava{nombre: "Casos.java", contenido: fuenteLejana},
		fuenteJava{nombre: "Lejana.java", contenido: fuenteTareaLejana},
	)
	m, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
		{Tipo: "avanzar", Dato: "100"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
	})
	if err != nil {
		t.Fatalf("el guion falló: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got == "M" {
		t.Errorf("la tarea del retraso desbordado corrió en el pasado y dejó %q", got)
	}
	if m.Ejecucion.Reloj != 100 {
		t.Errorf("el reloj es %d y se esperaba 100: la tarea no puede estar antes del arranque",
			m.Ejecucion.Reloj)
	}
	if m.Ejecucion.Tareas != 0 {
		t.Errorf("la cola ejecutó %d tareas y se esperaba ninguna", m.Ejecucion.Tareas)
	}
	if !strings.Contains(strings.Join(m.Ejecucion.Avisos, " "), "no cabe") {
		t.Errorf("falta el aviso del retraso que no cabe: %v", m.Ejecucion.Avisos)
	}
}

// TestElRelojNoMienteAlAgotarPasos comprueba que el presupuesto de pasos es por tarea: un avance de
// 100000000 ms con un tic por segundo ejecuta los 100000 tics, el reloj llega al destino y no hay
// aviso. Con el presupuesto global, la cola se cortaba en 5880 tics y el reloj publicado mentía.
func TestElRelojNoMienteAlAgotarPasos(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteTics})
	m, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: "avanzar", Dato: "100000000"}})
	if err != nil {
		t.Fatalf("--avanzar 100000000 falló: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "Tic: 100000" {
		t.Errorf("el contador vale %q y se esperaba «Tic: 100000»: el reloj no puede publicar un "+
			"destino con tics sin ejecutar", got)
	}
	if m.Ejecucion.Reloj != 100000000 {
		t.Errorf("el reloj es %d y se esperaba 100000000", m.Ejecucion.Reloj)
	}
	if a := avisoDeTruncamiento(m.Ejecucion); a != "" {
		t.Errorf("el avance se truncó: %s", a)
	}
}

// avisoDeTruncamiento devuelve el aviso de la salvaguarda de la cola, si la ejecución lo trae.
func avisoDeTruncamiento(r *resultadoEjecucion) string {
	for _, a := range r.Avisos {
		if strings.Contains(a, "se detuvo") {
			return a
		}
	}
	return ""
}

// TestElRelojNoDesborda fija la salida del desbordamiento: un avance que no cabe en el reloj no se
// satura en silencio ni envuelve a negativo; el comando devuelve error, como cualquier otra orden
// que no se puede entregar.
func TestElRelojNoDesborda(t *testing.T) {
	// el borde exacto, sobre una máquina sin programa: MaxInt64-1 más uno cabe, y un milisegundo
	// más no
	m := &maquinaDalvik{}
	reloj := int64(math.MaxInt64 - 1)
	m.reloj = &reloj
	if err := m.avanza(1); err != nil {
		t.Fatalf("MaxInt64-1 más uno cabe y devolvió %v", err)
	}
	if reloj != math.MaxInt64 {
		t.Fatalf("el reloj quedó en %d y se esperaba %d", reloj, int64(math.MaxInt64))
	}
	if err := m.avanza(1); err == nil {
		t.Fatalf("MaxInt64 más uno no desbordó: el reloj quedó en %d", reloj)
	}

	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteQuieta})
	tope := "9223372036854775807"
	uno, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{{Tipo: "avanzar", Dato: tope}})
	if err != nil {
		t.Fatalf("un solo --avanzar %s falló: %v", tope, err)
	}
	if uno.Ejecucion.Reloj != math.MaxInt64 {
		t.Errorf("el reloj es %d y se esperaba %d", uno.Ejecucion.Reloj, int64(math.MaxInt64))
	}
	// dos avances válidos por separado: el segundo no cabe y el comando se rechaza, sin publicar un
	// reloj envuelto
	_, err = modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
		{Tipo: "avanzar", Dato: tope},
		{Tipo: "avanzar", Dato: "1"},
	})
	if err == nil {
		t.Fatal("MaxInt64 más uno se aceptó y el reloj habría envuelto a negativo")
	}
	if !strings.Contains(err.Error(), "desborda") {
		t.Errorf("el error no dice que desborda: %v", err)
	}
}

// TestElRelojAvanzaSinTruncar comprueba que la salvaguarda de la cola no corta un avance legítimo:
// 300 y 3000 tics de un segundo corren enteros, el reloj llega al destino y no hay aviso.
func TestElRelojAvanzaSinTruncar(t *testing.T) {
	apk := apkDelReloj(t)
	casos := []struct {
		avance string
		tics   string
		reloj  int64
	}{
		{"300000", "Tic: 300", 300000},
		{"3000000", "Tic: 3000", 3000000},
	}
	for _, c := range casos {
		m, err := modeloVistaDeAPKConEventos(apk, "", true,
			[]eventoUsuario{{Tipo: "avanzar", Dato: c.avance}})
		if err != nil {
			t.Fatalf("--avanzar %s falló: %v", c.avance, err)
		}
		if got := textoDeNodo(t, m, idTotal); got != c.tics {
			t.Errorf("tras --avanzar %s el contador vale %q y se esperaba %q", c.avance, got, c.tics)
		}
		if m.Ejecucion.Reloj != c.reloj {
			t.Errorf("tras --avanzar %s el reloj es %d y se esperaba %d", c.avance,
				m.Ejecucion.Reloj, c.reloj)
		}
		if a := avisoDeTruncamiento(m.Ejecucion); a != "" {
			t.Errorf("--avanzar %s se truncó: %s", c.avance, a)
		}
	}
}

// TestElRetrasoNegativoNoRetrocedeElReloj es la sonda del retraso negativo: `postDelayed(…, -1000)`
// se trata como cero, así que el padre corre en el instante 0, la hija queda en 1100 y el reloj
// nunca baja de cero. Sin acotar, el padre correría en -1000 y la hija se adelantaría a 100.
func TestElRetrasoNegativoNoRetrocedeElReloj(t *testing.T) {
	apk := apkConFuentes(t,
		fuenteJava{nombre: "Casos.java", contenido: fuenteRetraso},
		fuenteJava{nombre: "Padre.java", contenido: fuentePadre},
		fuenteJava{nombre: "Hija.java", contenido: fuenteHija},
	)

	medio, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: "avanzar", Dato: "500"}})
	if err != nil {
		t.Fatalf("--avanzar 500 falló: %v", err)
	}
	if medio.Ejecucion.Reloj < 0 {
		t.Errorf("el reloj quedó en %d: el retraso negativo lo hizo retroceder", medio.Ejecucion.Reloj)
	}
	if medio.Ejecucion.Reloj != 500 {
		t.Errorf("el reloj es %d y se esperaba 500", medio.Ejecucion.Reloj)
	}
	if got := textoDeNodo(t, medio, idTotal); got != "1" {
		t.Errorf("a los 500 ms el contador vale %q: la hija de 1100 ms no debía correr", got)
	}

	completo, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: "avanzar", Dato: "2000"}})
	if err != nil {
		t.Fatalf("--avanzar 2000 falló: %v", err)
	}
	if completo.Ejecucion.Reloj != 2000 {
		t.Errorf("el reloj es %d y se esperaba 2000", completo.Ejecucion.Reloj)
	}
	if got := textoDeNodo(t, completo, idTotal); got != "2" {
		t.Errorf("a los 2000 ms el contador vale %q y se esperaba «2»", got)
	}
}
