package main

// La prueba de aceptación de E2 del plan de emulación: un ListView con un BaseAdapter del proyecto.
// El APK se arma en memoria con los escritores del fork y el dex sale del emisor del fork, como el
// resto de las pruebas de la vista previa (regla 2: sin binarios versionados).
//
// El adaptador infla un layout de fila y escribe en él el texto de cada posición; el modelo tiene
// que enseñar tres hijos con su texto y, al pulsar `id/lista:1`, entregar el onItemClick de la
// posición 1. La forma `id/lista:fila` es la que esta fase añade a `--pulsar`: la lista va antes de
// los dos puntos y la posición después.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tablaDeListas: los tres ids de la pantalla y los dos layouts, con su ruta en el pool global.
func tablaDeListas() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.listas",
		global:        []string{"res/layout/main.xml", "res/layout/fila.xml"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout"},
		nombresUTF8:   false,
		claves:        []string{"lista", "nombre", "pie", "fila", "main"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{
				id:        1,
				specFlags: []uint32{0, 0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 1, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 2, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
				}}},
			},
			{
				id:        2,
				specFlags: []uint32{0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 3, valor: valorEscribir{tipo: 0x03, datos: 1}},
					{clave: 4, valor: valorEscribir{tipo: 0x03, datos: 0}},
				}}},
			},
		},
	}
}

// layoutDeLaLista es la pantalla: un LinearLayout vertical con el ListView y un TextView que el
// onItemClick reescribe con la fila elegida.
func layoutDeLaLista() documentoAXML {
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
					nombre: "ListView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
					},
				},
				{
					nombre: "TextView",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010002},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Sin elegir"},
					},
				},
			},
		},
	}
}

// layoutDeLaFila es el layout que el getView infla para cada elemento: un TextView con id.
func layoutDeLaFila() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "TextView",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010001},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
				{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "fila"},
			},
		},
	}
}

// manifiestoDeListas declara una sola actividad, la lanzadora.
func manifiestoDeListas() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: false,
		raiz: &elementoEscribir{
			nombre: "manifest",
			linea:  1,
			attrs: []atributoEscribir{
				{nombre: "package", tipo: tipoCadena, texto: "com.hola.listas"},
				{ns: "android", nombre: "versionName", id: 0x0101021c, tipo: tipoCadena, texto: "0.1"},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "application",
					linea:  9,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Listas"},
					},
					hijos: []*elementoEscribir{
						{
							nombre: "activity",
							linea:  11,
							attrs: []atributoEscribir{
								{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
									texto: ".MainActivity"},
							},
							hijos: []*elementoEscribir{
								{
									nombre: "intent-filter",
									linea:  1,
									hijos: []*elementoEscribir{
										{
											nombre: "action",
											linea:  2,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003,
													tipo: tipoCadena, texto: "android.intent.action.MAIN"},
											},
										},
										{
											nombre: "category",
											linea:  3,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003,
													tipo: tipoCadena, texto: "android.intent.category.LAUNCHER"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// fuenteMainListas arma la lista con el adaptador y el oyente de fila. El `notifyDataSetChanged`
// va a propósito: es una de las llamadas de framework que la fase promete aceptar. `prueba` llama
// a getItem y getItemId con una condición imposible: el comando no deja avisos solo si las dos
// llegaron al método del proyecto, y la pantalla no cambia.
const fuenteMainListas = `package com.hola.listas;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        ListView lista = (ListView) findViewById(R.id.lista);
        Adaptador adaptador = new Adaptador(this);
        lista.setAdapter(adaptador);
        adaptador.notifyDataSetChanged();
        prueba(adaptador);
        lista.setOnItemClickListener(new AdapterView.OnItemClickListener() {
            public void onItemClick(AdapterView padre, View vista, int posicion, long id) {
                TextView pie = (TextView) findViewById(R.id.pie);
                pie.setText("Elegido " + posicion);
            }
        });
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }

    void prueba(Adaptador adaptador) {
        int cuantas = adaptador.getCount();
        long ultimo = adaptador.getItemId(cuantas - 1);
        Object primero = adaptador.getItem(0);
        if (cuantas == 4) {
            if (ultimo == 9) {
                if (primero == null) {
                    TextView pie = (TextView) findViewById(R.id.pie);
                    pie.setText("no deberia pasar");
                }
            }
        }
    }
}
`

// fuenteAdaptador es un BaseAdapter del proyecto: getCount, getItem, getItemId y getView son suyos.
// getView infla el layout de fila y escribe el texto, que es lo que el modelo tiene que medir.
const fuenteAdaptador = `package com.hola.listas;

import android.app.*;
import android.view.*;
import android.widget.*;

public class Adaptador extends BaseAdapter {
    private Activity duenio;

    public Adaptador(Activity contexto) {
        duenio = contexto;
    }

    public int getCount() {
        return 3;
    }

    public Object getItem(int posicion) {
        if (posicion == 0) {
            return "Alfa";
        }
        if (posicion == 1) {
            return "Beta";
        }
        return "Gama";
    }

    public long getItemId(int posicion) {
        return posicion;
    }

    public View getView(int posicion, View convertida, ViewGroup padre) {
        View fila = duenio.getLayoutInflater().inflate(R.layout.fila, padre, false);
        TextView nombre = (TextView) fila.findViewById(R.id.nombre);
        nombre.setText("Fila " + posicion + ": " + getItem(posicion));
        return fila;
    }
}
`

// apkDeListas arma el APK de la lista: manifiesto, tabla, los dos layouts y el dex de las dos
// clases del proyecto con su R.java generado desde la tabla.
func apkDeListas(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDeListas())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	main, err := escribeAXML(layoutDeLaLista())
	if err != nil {
		t.Fatalf("el layout main: %v", err)
	}
	fila, err := escribeAXML(layoutDeLaFila())
	if err != nil {
		t.Fatalf("el layout fila: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDeListas())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDeListas())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": main,
		"res/layout/fila.xml": fila,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: r},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteMainListas},
			fuenteJava{nombre: "Adaptador.java", contenido: fuenteAdaptador}),
	})
}

// textoDeLaFila es lo que el modelo enseña de una fila: el texto que el getView escribió encima del
// que declara el layout.
func textoDeLaFila(n *nodoVista) string {
	if n.TextoDinamico != "" {
		return n.TextoDinamico
	}
	return n.Texto
}

// TestElPreviewPintaUnaListaConSuAdaptador es el criterio de aceptación de E2: el ListView tiene un
// hijo por fila con su texto, cada fila mide lo que su layout declara, y `--pulsar id/lista:1`
// ejecuta el onItemClick con la posición 1, que reescribe el pie de la pantalla.
func TestElPreviewPintaUnaListaConSuAdaptador(t *testing.T) {
	apk := apkDeListas(t)

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk de la lista no se lee: %v", err)
	}
	if reposo.Ejecucion == nil || reposo.Ejecucion.Error != "" {
		t.Fatalf("la ejecución en reposo falló: %+v", reposo.Ejecucion)
	}
	if reposo.Ejecucion.Actividad != "com.hola.listas.MainActivity" {
		t.Errorf("la actividad es %q", reposo.Ejecucion.Actividad)
	}
	if avisos := strings.Join(reposo.Ejecucion.Avisos, "\n"); avisos != "" {
		t.Errorf("la lista dejó avisos: %v", reposo.Ejecucion.Avisos)
	}
	if reposo.Raiz == nil || len(reposo.Raiz.Hijos) != 2 {
		t.Fatalf("la pantalla no tiene sus dos hijos: %+v", reposo.Raiz)
	}
	lista := reposo.Raiz.Hijos[0]
	if lista.Tipo != "ListView" {
		t.Fatalf("el primer hijo es un %s y se esperaba el ListView", lista.Tipo)
	}
	if len(lista.Hijos) != 3 {
		t.Fatalf("la lista tiene %d hijos y se esperaban 3: %+v", len(lista.Hijos), lista.Hijos)
	}
	esperados := []string{"Fila 0: Alfa", "Fila 1: Beta", "Fila 2: Gama"}
	for i, fila := range lista.Hijos {
		if fila.Tipo != "TextView" {
			t.Errorf("la fila %d es un %s y se esperaba el TextView del layout de fila", i, fila.Tipo)
		}
		if texto := textoDeLaFila(fila); texto != esperados[i] {
			t.Errorf("la fila %d dice %q y se esperaba %q", i, texto, esperados[i])
		}
		if fila.Rect.Alto <= 0 || fila.Rect.Ancho <= 0 {
			t.Errorf("la fila %d no se midió: %+v", i, fila.Rect)
		}
	}
	if reposo.Raiz.Hijos[1].TextoDinamico != "" {
		t.Errorf("sin clics el pie ya dice %q", reposo.Raiz.Hijos[1].TextoDinamico)
	}

	conClic, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/lista:1"})
	if err != nil {
		t.Fatalf("el clic en la fila 1 falló: %v", err)
	}
	if conClic.Ejecucion == nil || conClic.Ejecucion.Error != "" {
		t.Fatalf("la ejecución con el clic falló: %+v", conClic.Ejecucion)
	}
	if len(conClic.Ejecucion.Pulsaciones) != 1 || conClic.Ejecucion.Pulsaciones[0] != "id/lista:1" {
		t.Errorf("la pulsación entregada no es la pedida: %v", conClic.Ejecucion.Pulsaciones)
	}
	if conClic.Ejecucion.Interacciones != 1 {
		t.Errorf("el clic no contó una interacción: %d", conClic.Ejecucion.Interacciones)
	}
	pie := conClic.Raiz.Hijos[1]
	if texto := textoDeLaFila(pie); texto != "Elegido 1" {
		t.Errorf("tras pulsar la fila 1 el pie dice %q y se esperaba «Elegido 1»", texto)
	}
	if len(conClic.Raiz.Hijos[0].Hijos) != 3 {
		t.Errorf("tras el clic la lista tiene %d filas", len(conClic.Raiz.Hijos[0].Hijos))
	}
}

// fuenteMainNotas es el esqueleto del bloc de notas de la plantilla: el adaptador se reconstruye
// con una copia del arreglo en cada cambio, y el primer `redibujar` recibe un String[0]. Ese arreglo
// vacío es el que dejaba `elementos` nulo y hacía fallar al getCount del framework (§335).
const fuenteMainNotas = `package com.hola.listas;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity implements View.OnClickListener {
    private String[] notas;
    private int cuantas;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        notas = new String[8];
        cuantas = 0;
        redibujar();
        findViewById(R.id.pie).setOnClickListener(this);
    }

    public void onClick(View vista) {
        notas[cuantas] = "Nota " + cuantas;
        cuantas = cuantas + 1;
        redibujar();
    }

    private void redibujar() {
        String[] copia = new String[cuantas];
        for (int i = 0; i < cuantas; i++) {
            copia[i] = notas[i];
        }
        ListView lista = (ListView) findViewById(R.id.lista);
        lista.setAdapter(new ArrayAdapter(this, R.layout.fila, copia));
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
}
`

// apkDeNotasDevuelve el apk en memoria, sin binarios versionados, y el dex suelto para el hot swap.
func apkDeNotas(t *testing.T) ([]byte, []byte) {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDeListas())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	main, err := escribeAXML(layoutDeLaLista())
	if err != nil {
		t.Fatalf("el layout main: %v", err)
	}
	fila, err := escribeAXML(layoutDeLaFila())
	if err != nil {
		t.Fatalf("el layout fila: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDeListas())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDeListas())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	dex := dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: r},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteMainNotas})
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": main,
		"res/layout/fila.xml": fila,
		"classes.dex":         dex,
	}), dex
}

// filasDeNotas lee el modelo publicado por una orden y devuelve los textos de las filas de la lista
// y los avisos de la ejecución, para no repetir el parseo en cada aseveración.
func filasDeNotas(t *testing.T, linea string) ([]string, []string) {
	t.Helper()
	var modelo struct {
		Raiz struct {
			Hijos []struct {
				Tipo  string `json:"tipo"`
				Hijos []struct {
					Texto string `json:"texto"`
				} `json:"hijos"`
			} `json:"hijos"`
		} `json:"raiz"`
		Ejecucion struct {
			Avisos []string `json:"avisos"`
		} `json:"ejecucion"`
	}
	if err := json.Unmarshal([]byte(linea), &modelo); err != nil {
		t.Fatalf("el modelo no se lee: %v", err)
	}
	var filas []string
	for _, n := range modelo.Raiz.Hijos {
		if n.Tipo == "ListView" {
			for _, h := range n.Hijos {
				filas = append(filas, h.Texto)
			}
		}
	}
	return filas, modelo.Ejecucion.Avisos
}

// TestElPreviewPintaDosNotasEs la falsación de §335: la primera pantalla de un bloc de notas vacío
// no deja avisos, dos altas pintan dos filas, y la sesión viva conserva las dos filas después de un
// hot swap sin replay. Antes de la corrección el String[0] inicial dejaba `elementos` nulo y el
// getCount del framework fallaba con «no tiene getCount()I».
func TestElPreviewPintaDosNotas(t *testing.T) {
	apk, dex := apkDeNotas(t)

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk de notas no se lee: %v", err)
	}
	if avisos := strings.Join(reposo.Ejecucion.Avisos, "\n"); avisos != "" {
		t.Errorf("la lista vacía dejó avisos: %v", reposo.Ejecucion.Avisos)
	}
	if lista := reposo.Raiz.Hijos[0]; lista.Tipo != "ListView" || len(lista.Hijos) != 0 {
		t.Fatalf("la pantalla inicial no es una lista vacía: %+v", reposo.Raiz.Hijos[0])
	}

	conNotas, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pie", "id/pie"})
	if err != nil {
		t.Fatalf("las dos notas fallaron: %v", err)
	}
	if avisos := strings.Join(conNotas.Ejecucion.Avisos, "\n"); avisos != "" {
		t.Errorf("las dos notas dejaron avisos: %v", conNotas.Ejecucion.Avisos)
	}
	esperadas := []string{"Nota 0", "Nota 1"}
	lista := conNotas.Raiz.Hijos[0]
	if len(lista.Hijos) != len(esperadas) {
		t.Fatalf("la lista tiene %d filas y se esperaban 2", len(lista.Hijos))
	}
	for i, fila := range lista.Hijos {
		if texto := textoDeLaFila(fila); texto != esperadas[i] {
			t.Errorf("la fila %d dice %q y se esperaba %q", i, texto, esperadas[i])
		}
	}

	// la misma secuencia, pero entregada de una orden en una sesión viva, sin replay del guion
	s, err := nuevaSesionPreview(apk, "", "")
	if err != nil {
		t.Fatalf("la sesión de notas no arrancó: %v", err)
	}
	linea := ""
	for _, argv := range [][]string{
		{"--pulsar", "id/pie"},
		{"--pulsar", "id/pie"},
	} {
		if codigo := s.ejecutaOrden("notas", argv, func(l string) { linea = l }); codigo != 0 {
			t.Fatalf("la orden %v devolvió %d", argv, codigo)
		}
	}
	filas, avisos := filasDeNotas(t, linea)
	if len(avisos) != 0 {
		t.Errorf("la sesión dejó avisos: %v", avisos)
	}
	if strings.Join(filas, ",") != strings.Join(esperadas, ",") {
		t.Errorf("la sesión pinta %v y se esperaba %v", filas, esperadas)
	}

	// el hot swap con el MISMO dex no cambia la máquina: las dos filas siguen vivas
	ruta := filepath.Join(t.TempDir(), "mismo.dex")
	if err := os.WriteFile(ruta, dex, 0o644); err != nil {
		t.Fatalf("el dex del hot swap no se escribió: %v", err)
	}
	linea = ""
	codigo := s.ejecutaOrden("notas", []string{
		"--intercambiar", "Lcom/hola/listas/MainActivity;/onClick(Landroid/view/View;)V=" + ruta,
	}, func(l string) { linea = l })
	if codigo != 0 {
		t.Fatalf("el hot swap devolvió %d", codigo)
	}
	filas, avisos = filasDeNotas(t, linea)
	if len(avisos) != 0 {
		t.Errorf("el hot swap dejó avisos: %v", avisos)
	}
	if strings.Join(filas, ",") != strings.Join(esperadas, ",") {
		t.Errorf("tras el hot swap la sesión pinta %v y se esperaba %v", filas, esperadas)
	}
}
