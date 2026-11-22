package main

// La prueba de aceptación de E1 del plan de emulación: ciclo de vida y navegación entre dos
// actividades propias. El APK se arma en memoria con los escritores del fork y el dex sale del
// emisor del fork, como el resto de las pruebas de la vista previa (regla 2: sin binarios
// versionados).

import (
	"strings"
	"testing"
)

// tablaDelNavega: dos layouts y los cuatro ids que usan las dos actividades.
func tablaDelNavega() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.navega",
		global:        []string{"res/layout/main.xml", "res/layout/segunda.xml"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout"},
		nombresUTF8:   false,
		claves:        []string{"abrir", "texto", "total", "volver", "main", "segunda"},
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
				specFlags: []uint32{0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 4, valor: valorEscribir{tipo: 0x03, datos: 0}},
					{clave: 5, valor: valorEscribir{tipo: 0x03, datos: 1}},
				}}},
			},
		},
	}
}

// layoutDeUnaPantalla arma un LinearLayout vertical con un TextView y un Button, que es todo lo que
// necesitan las dos pantallas de la prueba.
func layoutDeUnaPantalla(idTexto uint32, texto string, idBoton uint32, etiqueta string) documentoAXML {
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
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: idTexto},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: texto},
					},
				},
				{
					nombre: "Button",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: idBoton},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: etiqueta},
					},
				},
			},
		},
	}
}

// manifiestoDelNavega declara las dos actividades: la primera con el filtro LAUNCHER y la segunda
// solo con su nombre, que es lo que `startActivity` exige para poder abrirla.
func manifiestoDelNavega() documentoAXML {
	actividad := func(nombre string, lanzadora bool) *elementoEscribir {
		hijos := []*elementoEscribir{}
		if lanzadora {
			hijos = append(hijos, &elementoEscribir{
				nombre: "intent-filter",
				linea:  1,
				hijos: []*elementoEscribir{
					{
						nombre: "action",
						linea:  2,
						attrs: []atributoEscribir{
							{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
								texto: "android.intent.action.MAIN"},
						},
					},
					{
						nombre: "category",
						linea:  3,
						attrs: []atributoEscribir{
							{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
								texto: "android.intent.category.LAUNCHER"},
						},
					},
				},
			})
		}
		return &elementoEscribir{
			nombre: "activity",
			linea:  11,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: nombre},
			},
			hijos: hijos,
		}
	}
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: false,
		raiz: &elementoEscribir{
			nombre: "manifest",
			linea:  1,
			attrs: []atributoEscribir{
				{nombre: "package", tipo: tipoCadena, texto: "com.hola.navega"},
				{ns: "android", nombre: "versionName", id: 0x0101021c, tipo: tipoCadena, texto: "0.1"},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "application",
					linea:  9,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Navega"},
					},
					hijos: []*elementoEscribir{
						actividad(".MainActivity", true),
						actividad(".SegundaActivity", false),
					},
				},
			},
		},
	}
}

const fuenteMainNavega = `package com.hola.navega;

import android.app.*;
import android.content.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.abrir).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                abrir();
            }
        });
    }

    void abrir() {
        pulsos = pulsos + 1;
        total.setText("Pulsos: " + pulsos);
        startActivity(new Intent(this, SegundaActivity.class));
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
    public void onRestart() { }
}
`

const fuenteSegundaNavega = `package com.hola.navega;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class SegundaActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.segunda);
        TextView texto = (TextView) findViewById(R.id.texto);
        texto.setText("Pantalla 2");
        findViewById(R.id.volver).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                finish();
            }
        });
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
}
`

// apkDelNavega arma el APK de dos pantallas: manifiesto con las dos actividades, tabla, los dos
// layouts y el dex de las dos clases con su R.java generado desde la tabla.
func apkDelNavega(t *testing.T) []byte {
	t.Helper()
	return apkDelNavegaConManifiesto(t, manifiestoDelNavega())
}

// apkDelNavegaConManifiesto es el mismo APK con otro manifiesto: lo usa la prueba de la frontera
// para dejar la segunda actividad fuera de la declaración sin cambiar el dex.
func apkDelNavegaConManifiesto(t *testing.T, manifiestoDoc documentoAXML) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDoc)
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	main, err := escribeAXML(layoutDeUnaPantalla(0x7f010002, "Pulsos: 0", 0x7f010000, "Abrir"))
	if err != nil {
		t.Fatalf("el layout main: %v", err)
	}
	segunda, err := escribeAXML(layoutDeUnaPantalla(0x7f010001, "Segunda", 0x7f010003, "Volver"))
	if err != nil {
		t.Fatalf("el layout segunda: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelNavega())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDelNavega())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml":    manifiesto,
		"resources.arsc":         arsc,
		"res/layout/main.xml":    main,
		"res/layout/segunda.xml": segunda,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: r},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteMainNavega},
			fuenteJava{nombre: "SegundaActivity.java", contenido: fuenteSegundaNavega}),
	})
}

func tienePaso(pasos []string, buscado string) bool {
	for _, p := range pasos {
		if p == buscado {
			return true
		}
	}
	return false
}

// TestElPreviewNavegaEntreActividadesPropias es el criterio de aceptación de E1: sin clics se ve la
// lanzadora; un clic que llama a `startActivity` deja delante la segunda pantalla con su layout y su
// actividad; y `finish()` vuelve a la primera conservando lo que ya había cambiado (el texto sigue
// en «Pulsos: 1», no vuelve al «Pulsos: 0» del layout), que es lo que distingue conservar la
// máquina de repetir su `onCreate`.
func TestElPreviewNavegaEntreActividadesPropias(t *testing.T) {
	apk := apkDelNavega(t)

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk de dos pantallas no se lee: %v", err)
	}
	if reposo.Ejecucion == nil || reposo.Ejecucion.Error != "" {
		t.Fatalf("la ejecución en reposo falló: %+v", reposo.Ejecucion)
	}
	if reposo.Layout != "res/layout/main.xml" {
		t.Errorf("sin clics el layout es %q y se esperaba el main", reposo.Layout)
	}
	if len(reposo.Ejecucion.Pantallas) != 1 {
		t.Errorf("sin clics el recorrido es %v", reposo.Ejecucion.Pantallas)
	}
	for _, esperado := range []string{"com.hola.navega.MainActivity.onCreate",
		"com.hola.navega.MainActivity.onStart", "com.hola.navega.MainActivity.onResume"} {
		if !tienePaso(reposo.Ejecucion.Ciclo, esperado) {
			t.Errorf("el ciclo no pasó por %s: %v", esperado, reposo.Ejecucion.Ciclo)
		}
	}

	una, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir"})
	if err != nil {
		t.Fatalf("el clic que navega falló: %v", err)
	}
	if una.Layout != "res/layout/segunda.xml" {
		t.Fatalf("tras navegar el layout es %q y se esperaba el de la segunda pantalla; capturas=%+v avisos=%v",
			una.Layout, una.Ejecucion.Capturas, una.Ejecucion.Avisos)
	}
	if una.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("tras navegar la actividad es %q", una.Actividad)
	}
	if una.Ejecucion == nil || una.Ejecucion.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("la ejecución no dice que corre la segunda actividad: %+v", una.Ejecucion)
	}
	if len(una.Ejecucion.Pantallas) != 2 ||
		una.Ejecucion.Pantallas[1] != "com.hola.navega.SegundaActivity" {
		t.Errorf("el recorrido tras navegar es %v", una.Ejecucion.Pantallas)
	}
	if una.Raiz == nil || len(una.Raiz.Hijos) != 2 ||
		una.Raiz.Hijos[0].TextoDinamico != "Pantalla 2" {
		t.Errorf("la segunda pantalla no pinta su texto: %+v", una.Raiz)
	}
	for _, esperado := range []string{
		"com.hola.navega.MainActivity.onPause", "com.hola.navega.MainActivity.onStop",
		"com.hola.navega.SegundaActivity.onCreate", "com.hola.navega.SegundaActivity.onStart",
		"com.hola.navega.SegundaActivity.onResume",
	} {
		if !tienePaso(una.Ejecucion.Ciclo, esperado) {
			t.Errorf("el ciclo tras navegar no pasó por %s: %v", esperado, una.Ejecucion.Ciclo)
		}
	}

	vuelta, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir", "id/volver"})
	if err != nil {
		t.Fatalf("el clic que vuelve falló: %v", err)
	}
	if vuelta.Layout != "res/layout/main.xml" {
		t.Fatalf("tras finish() el layout es %q y se esperaba el main", vuelta.Layout)
	}
	if vuelta.Actividad != "com.hola.navega.MainActivity" {
		t.Errorf("tras finish() la actividad es %q", vuelta.Actividad)
	}
	if len(vuelta.Ejecucion.Pantallas) != 3 ||
		vuelta.Ejecucion.Pantallas[2] != "com.hola.navega.MainActivity" {
		t.Errorf("el recorrido tras volver es %v", vuelta.Ejecucion.Pantallas)
	}
	if vuelta.Raiz == nil || len(vuelta.Raiz.Hijos) != 2 {
		t.Fatalf("la pantalla de vuelta no tiene sus dos hijos: %+v", vuelta.Raiz)
	}
	texto := vuelta.Raiz.Hijos[0].TextoDinamico
	if texto != "Pulsos: 1" {
		t.Errorf("al volver el texto es %q y se esperaba «Pulsos: 1»: el estado no se conservó", texto)
	}
	for _, esperado := range []string{
		"com.hola.navega.SegundaActivity.onPause", "com.hola.navega.SegundaActivity.onStop",
		"com.hola.navega.SegundaActivity.onDestroy",
		"com.hola.navega.MainActivity.onRestart", "com.hola.navega.MainActivity.onResume",
	} {
		if !tienePaso(vuelta.Ejecucion.Ciclo, esperado) {
			t.Errorf("el ciclo al volver no pasó por %s: %v", esperado, vuelta.Ejecucion.Ciclo)
		}
	}
	// el ciclo compartido va en orden real, no agrupado por pantalla: la pausa de la primera va
	// antes del onCreate de la segunda, y el reinicio de la primera va después del destroy
	orden := []string{
		"com.hola.navega.MainActivity.onCreate", "com.hola.navega.MainActivity.onStart",
		"com.hola.navega.MainActivity.onResume",
		"com.hola.navega.MainActivity.onPause", "com.hola.navega.MainActivity.onStop",
		"com.hola.navega.SegundaActivity.onCreate", "com.hola.navega.SegundaActivity.onStart",
		"com.hola.navega.SegundaActivity.onResume",
		"com.hola.navega.SegundaActivity.onPause", "com.hola.navega.SegundaActivity.onStop",
		"com.hola.navega.SegundaActivity.onDestroy",
		"com.hola.navega.MainActivity.onRestart", "com.hola.navega.MainActivity.onStart",
		"com.hola.navega.MainActivity.onResume",
	}
	if strings.Join(vuelta.Ejecucion.Ciclo, "\n") != strings.Join(orden, "\n") {
		t.Errorf("el ciclo no va en orden real:\n%v\ny se esperaba\n%v", vuelta.Ejecucion.Ciclo, orden)
	}
	if strings.Contains(strings.Join(vuelta.Ejecucion.Avisos, "\n"), "no está declarada") {
		t.Errorf("la navegación entre actividades declaradas dejó un aviso: %v", vuelta.Ejecucion.Avisos)
	}
}

// manifiestoSoloConMain deja la segunda actividad fuera del manifiesto sin tocar el dex: es la
// frontera que E1 promete rechazar por nombre.
func manifiestoSoloConMain() documentoAXML {
	m := manifiestoDelNavega()
	m.raiz.hijos[0].hijos = m.raiz.hijos[0].hijos[:1]
	return m
}

// TestElPreviewNoAbreUnaActividadNoDeclarada comprueba que `startActivity` a una clase que existe
// en el dex pero no está declarada en el manifiesto deja el aviso y no cambia de pantalla: es la
// misma regla del aparato, que no puede arrancar una actividad que el manifiesto no declara.
func TestElPreviewNoAbreUnaActividadNoDeclarada(t *testing.T) {
	apk := apkDelNavegaConManifiesto(t, manifiestoSoloConMain())
	m, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir"})
	if err != nil {
		t.Fatalf("el clic falló: %v", err)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("se cambió de pantalla a %q con la actividad sin declarar", m.Layout)
	}
	if len(m.Ejecucion.Pantallas) != 1 {
		t.Errorf("el recorrido abrió una actividad no declarada: %v", m.Ejecucion.Pantallas)
	}
	avisos := strings.Join(m.Ejecucion.Avisos, "\n")
	if !strings.Contains(avisos, "no está declarada en el manifiesto") {
		t.Errorf("falta el aviso de actividad no declarada: %v", m.Ejecucion.Avisos)
	}
}
