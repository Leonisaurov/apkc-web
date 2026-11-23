package main

// El menú de opciones del emulador: `--menu` ejecuta `onCreateOptionsMenu` y publica los ítems que
// la app añadió con `menu.add`, y `--menu-item menu/N` entrega el clic a su
// `OnMenuItemClickListener` o, si no lo hay, a `onOptionsItemSelected`. El APK se arma en memoria
// con los escritores del fork y el dex sale del emisor propio, como el resto de la vista previa
// (regla 2: sin binarios versionados).

import (
	"strings"
	"testing"
)

// fuenteMenu es la plantilla «Menú» del estudio más un segundo ítem con id propio atendido desde
// `onOptionsItemSelected` y un tercero invisible, que no debe publicarse ni poder pulsarse.
const fuenteMenu = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
    }

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add("Opciones").setOnMenuItemClickListener(new MenuItem.OnMenuItemClickListener() {
            public boolean onMenuItemClick(MenuItem item) {
                total.setText("Menú: " + item.getTitle());
                Toast.makeText(MainActivity.this, "Opciones pulsadas", Toast.LENGTH_SHORT).show();
                return true;
            }
        });
        menu.add("Oculto").setVisible(false);
        menu.add(0, 7, 2, "Numerada");
        return true;
    }

    @Override
    public boolean onOptionsItemSelected(MenuItem item) {
        if (item.getItemId() == 7) {
            total.setText("Elegida " + item.getTitle());
            return true;
        }
        return false;
    }
}
`

// apkDelMenu arma el apk de la plantilla con su R.java y su tabla.
func apkDelMenu(t *testing.T) []byte {
	t.Helper()
	return apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteMenu})
}

// fuentePlantillaMenu es la plantilla «Menú de opciones» que genera el editor, copiada tal cual de
// `studio/app.js` (TEMPLATES.activityMenu) con el paquete sustituido. No es un fixture: es la fuente
// exacta que el usuario compila desde la página, incluida la anónima de MenuItem.
const fuentePlantillaMenu = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
    }

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add("Opciones").setOnMenuItemClickListener(new MenuItem.OnMenuItemClickListener() {
            public boolean onMenuItemClick(MenuItem item) {
                Toast.makeText(MainActivity.this, "Menu pulsado", Toast.LENGTH_SHORT).show();
                return true;
            }
        });
        return true;
    }
}
`

// TestElPreviewEjecutaLaPlantillaMenuDelEditor reproduce la plantilla real del editor, no un
// fixture: `--menu` publica «Opciones» y `--menu-item menu/0` ejecuta la anónima
// `MenuItem.OnMenuItemClickListener`, cuyo único efecto es un Toast. Es la prueba de que el camino
// que ve el usuario (menú → ítem → Toast en la consola) funciona con la fuente que genera la página.
func TestElPreviewEjecutaLaPlantillaMenuDelEditor(t *testing.T) {
	apk := apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuentePlantillaMenu})

	abierto, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: ordenMenuTipo}})
	if err != nil {
		t.Fatalf("--menu con la plantilla del editor falló: %v", err)
	}
	menu := abierto.Ejecucion.Menu
	if menu == nil || !menu.Abierto || len(menu.Items) != 1 || menu.Items[0].Titulo != "Opciones" {
		t.Fatalf("la plantilla no publica un menú abierto con «Opciones»: %+v", menu)
	}

	pulsado, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
		{Tipo: ordenMenuTipo},
		{Tipo: ordenMenuItemTipo, Dato: "menu/0"},
	})
	if err != nil {
		t.Fatalf("--menu-item menu/0 con la plantilla del editor falló: %v", err)
	}
	if pulsado.Ejecucion.Toast != "Menu pulsado" {
		t.Errorf("el Toast de la anónima es %q y se esperaba «Menu pulsado»",
			pulsado.Ejecucion.Toast)
	}
	linea := false
	for _, l := range pulsado.Ejecucion.Consola {
		if l.Origen == "Toast" && l.Texto == "Menu pulsado" {
			linea = true
		}
	}
	if !linea {
		t.Errorf("la consola no tiene la línea del Toast de la plantilla: %+v", pulsado.Ejecucion.Consola)
	}
	if pulsado.Ejecucion.Menu == nil || pulsado.Ejecucion.Menu.Abierto ||
		pulsado.Ejecucion.Menu.Seleccionado != "menu/0" {
		t.Errorf("el menú no quedó cerrado con el ítem pulsado: %+v", pulsado.Ejecucion.Menu)
	}
	if len(pulsado.Ejecucion.Avisos) != 0 {
		t.Errorf("la plantilla del editor dejó avisos: %+v", pulsado.Ejecucion.Avisos)
	}
}

// TestElPreviewAbreElMenuDeOpciones es el criterio de la carencia: sin `--menu` no hay menú en el
// modelo; con él, `onCreateOptionsMenu` corre y el modelo publica sus ítems visibles con su id
// `menu/N`, saltándose los invisibles sin correr los índices.
func TestElPreviewAbreElMenuDeOpciones(t *testing.T) {
	apk := apkDelMenu(t)
	reposo, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk en reposo no se ejecuta: %v", err)
	}
	if reposo.Ejecucion == nil || reposo.Ejecucion.Menu != nil {
		t.Fatalf("sin --menu el modelo no debe traer menú: %+v", reposo.Ejecucion)
	}

	abierto, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: ordenMenuTipo}})
	if err != nil {
		t.Fatalf("--menu falló: %v", err)
	}
	menu := abierto.Ejecucion.Menu
	if menu == nil {
		t.Fatal("--menu no publicó el menú en ejecucion.menu")
	}
	if !menu.Abierto {
		t.Error("--menu dejó el menú cerrado")
	}
	if len(menu.Items) != 2 {
		t.Fatalf("el menú publica %d ítems visibles y son 2: %+v", len(menu.Items), menu.Items)
	}
	if menu.Items[0].ID != "menu/0" || menu.Items[0].Titulo != "Opciones" {
		t.Errorf("el primer ítem es %+v y se esperaba menu/0 «Opciones»", menu.Items[0])
	}
	// el invisible es el índice 1 del menú, así que el numerado conserva su `menu/2`: el índice de
	// `--menu-item` es la posición real, no la de la lista publicada
	if menu.Items[1].ID != "menu/2" || menu.Items[1].Titulo != "Numerada" ||
		menu.Items[1].ItemID != 7 {
		t.Errorf("el segundo ítem visible es %+v y se esperaba menu/2 «Numerada» con itemId 7",
			menu.Items[1])
	}
}

// TestElPreviewPulsaUnItemDelMenu entrega el clic del primer ítem: corre su
// `OnMenuItemClickListener`, deja su escritura y su Toast en el modelo y cierra el menú.
func TestElPreviewPulsaUnItemDelMenu(t *testing.T) {
	apk := apkDelMenu(t)
	m, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
		{Tipo: ordenMenuTipo},
		{Tipo: ordenMenuItemTipo, Dato: "menu/0"},
	})
	if err != nil {
		t.Fatalf("--menu-item menu/0 falló: %v", err)
	}
	if got := textoDelModelo(t, m); got != "Menú: Opciones" {
		t.Errorf("tras pulsar «Opciones» el texto es %q y se esperaba «Menú: Opciones»", got)
	}
	if m.Ejecucion.Toast != "Opciones pulsadas" {
		t.Errorf("el Toast del ítem es %q", m.Ejecucion.Toast)
	}
	if m.Ejecucion.Menu == nil || m.Ejecucion.Menu.Abierto {
		t.Errorf("elegir un ítem no cerró el menú: %+v", m.Ejecucion.Menu)
	}
	if m.Ejecucion.Menu.Seleccionado != "menu/0" {
		t.Errorf("el modelo no recuerda el ítem pulsado: %q", m.Ejecucion.Menu.Seleccionado)
	}
	if m.Ejecucion.Interacciones != 1 {
		t.Errorf("interacciones = %d y el clic del ítem cuenta una", m.Ejecucion.Interacciones)
	}
}

// TestElPreviewPulsaUnItemConOnOptionsItemSelected cubre el otro camino de la app: un ítem sin
// oyente propio, atendido por `onOptionsItemSelected` con su `getItemId()`.
func TestElPreviewPulsaUnItemConOnOptionsItemSelected(t *testing.T) {
	apk := apkDelMenu(t)
	m, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
		{Tipo: ordenMenuTipo},
		{Tipo: ordenMenuItemTipo, Dato: "menu/2"},
	})
	if err != nil {
		t.Fatalf("--menu-item menu/2 falló: %v", err)
	}
	if got := textoDelModelo(t, m); got != "Elegida Numerada" {
		t.Errorf("tras pulsar «Numerada» el texto es %q y se esperaba «Elegida Numerada»", got)
	}
	if m.Ejecucion.Menu == nil || m.Ejecucion.Menu.Seleccionado != "menu/2" {
		t.Errorf("el modelo no recuerda el ítem numerado: %+v", m.Ejecucion.Menu)
	}
}

// TestElPreviewRechazaUnItemOcultoYFueraDeRango es la frontera del guion: un ítem invisible o un
// índice que no existe no se pueden pulsar, y el comando lo dice en vez de devolver el modelo.
func TestElPreviewRechazaUnItemOcultoYFueraDeRango(t *testing.T) {
	apk := apkDelMenu(t)
	for _, pedido := range []string{"menu/1", "menu/9", "menu/x"} {
		if _, err := modeloVistaDeAPKConEventos(apk, "", true, []eventoUsuario{
			{Tipo: ordenMenuTipo},
			{Tipo: ordenMenuItemTipo, Dato: pedido},
		}); err == nil {
			t.Errorf("--menu-item %q no dio error y debería", pedido)
		}
	}
}

// TestElPreviewExigeOnCreateOptionsMenu es la falsificación del enganche: sin el método en la
// actividad, `--menu` falla con un mensaje que lo nombra, en vez de publicar un menú vacío que se
// leería como «la app no añadió ítems».
func TestElPreviewExigeOnCreateOptionsMenu(t *testing.T) {
	apk := apkDelContador(t, fuentesDelContador(t)...)
	_, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: ordenMenuTipo}})
	if err == nil {
		t.Fatal("--menu sobre una actividad sin onCreateOptionsMenu no dio error")
	}
	if !strings.Contains(err.Error(), "onCreateOptionsMenu") {
		t.Errorf("el error no nombra el método que falta: %v", err)
	}
}
