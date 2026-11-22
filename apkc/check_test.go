package main

// Tests de `apkc check`, el diagnóstico en vivo de la pestaña. El caso que los abrió (§200) es el que
// el usuario veía como falso error: `setContentView(R.layout.main)`, donde la `R` no viaja en el
// android.jar y solo existe si el proyecto la trae generada o se enlaza de `res/`.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// proyectoDeCheck escribe en un directorio temporal el proyecto mínimo del studio: el manifiesto, un
// layout con un id y la actividad que llama a `setContentView(R.layout.main)` en la línea 15.
func proyectoDeCheck(t *testing.T) (raiz, fuente string) {
	t.Helper()
	raiz = t.TempDir()
	pon(t, raiz, "AndroidManifest.xml", `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.hola.apkcstudio"
    android:versionCode="1"
    android:versionName="1.0">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
    <application android:label="Hola">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN"/>
                <category android:name="android.intent.category.LAUNCHER"/>
            </intent-filter>
        </activity>
    </application>
</manifest>
`)
	pon(t, raiz, "res/layout/main.xml", `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:orientation="vertical"
    android:layout_width="match_parent"
    android:layout_height="match_parent">
    <TextView android:id="@+id/total" android:layout_width="wrap_content"
        android:layout_height="wrap_content" android:text="Hola"/>
</LinearLayout>
`)
	// la linea 15 es la del `setContentView`: los tests de posicion cuentan con ella
	fuente = `package com.hola.apkcstudio;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;
import android.graphics.*;
import android.util.*;
import android.net.*;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
    }
}
`
	pon(t, raiz, "src/com/hola/apkcstudio/MainActivity.java", fuente)
	return raiz, fuente
}

// TestCheckEnlazaLaRDelProyecto es el caso del usuario: el mismo fuente, con la raiz del proyecto
// delante, no puede dar ni un error. Sin esto, `check` solo tenía el android.jar delante y acusaba
// `R.layout.main` de no estar medido.
func TestCheckEnlazaLaRDelProyecto(t *testing.T) {
	raiz, fuente := proyectoDeCheck(t)
	r := revisarFuenteJava(fuente, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el fuente del studio da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
	if !strings.Contains(r.Recursos, "res/") && !strings.Contains(r.Recursos, "R.java") {
		t.Fatalf("el informe no dice de donde salio la R: %q", r.Recursos)
	}
}

// TestCheckUsaLaRQueDejoUnBuild: si el proyecto ya tiene su R.java escrita, se usa esa y no se vuelve
// a enlazar. Es la misma clase que el build compilo, con sus ids.
func TestCheckUsaLaRQueDejoUnBuild(t *testing.T) {
	raiz, fuente := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/R.java", fuenteRStudio)
	r := revisarFuenteJava(fuente, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("con la R en disco el fuente da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if r.Recursos != "R.java del proyecto" {
		t.Fatalf("el informe no dice que uso la R del proyecto: %q", r.Recursos)
	}
}

// TestCheckSinProyectoNombraLaR comprueba el otro lado: sin proyecto no hay R, y el error tiene que
// decir **que** no se resuelve y **donde**, no un «el tipo de ese argumento no esta medido».
func TestCheckSinProyectoNombraLaR(t *testing.T) {
	_, fuente := proyectoDeCheck(t)
	r := revisarFuenteJava(fuente, "MainActivity.java", jarDelSDK(t), "")
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if e.Linea != 15 {
		t.Errorf("el error cae en la linea %d y el setContentView esta en la 15", e.Linea)
	}
	// la columna tiene que caer sobre la `R`, que es donde empieza el token que el editor subraya
	linea := strings.Split(fuente, "\n")[e.Linea-1]
	if quiero := strings.Index(linea, "R.layout.main") + 1; e.Columna != quiero {
		t.Errorf("la columna es %d y la R esta en la %d de %q", e.Columna, quiero, linea)
	}
	if quiero := "MainActivity.java:15:" + itoa(e.Columna); e.Posicion != quiero {
		t.Errorf("la posicion es %q, medida %q", e.Posicion, quiero)
	}
	if !strings.Contains(e.Mensaje, "R.layout.main") || !strings.Contains(e.Mensaje, "no se resuelve") {
		t.Errorf("el mensaje no nombra lo que no se resuelve: %q", e.Mensaje)
	}
	if strings.Contains(e.Mensaje, "no esta medido") {
		t.Errorf("el mensaje sigue siendo el generico: %q", e.Mensaje)
	}
	if strings.Contains(e.Mensaje, "linea 15") {
		t.Errorf("el mensaje repite la posicion, que ya va en `posicion`: %q", e.Mensaje)
	}
}

// TestCheckRecursosSinMedirNoFalsifica: un `res/` que el enlazador de Go no sabe escribir (un drawable
// XML) no puede dejar el informe en verde por su cuenta ni romper el check: se dice por que no hay R
// y el error del fuente sigue siendo el de la R que falta.
func TestCheckRecursosSinMedirNoFalsifica(t *testing.T) {
	raiz, fuente := proyectoDeCheck(t)
	pon(t, raiz, "res/drawable/forma.xml", `<shape xmlns:android="http://schemas.android.com/apk/res/android"/>`)
	r := revisarFuenteJava(fuente, "MainActivity.java", jarDelSDK(t), raiz)
	if !strings.Contains(r.Recursos, "sin R") {
		t.Fatalf("el informe no dice por que no hay R: %q", r.Recursos)
	}
	if r.Emisor {
		t.Fatal("se declaro comprobado un fuente cuyo unico error es la R que falta")
	}
	if len(r.Errores) != 1 || !strings.Contains(r.Errores[0].Mensaje, "no se resuelve") {
		t.Fatalf("el error no es el de la R: %+v", r.Errores)
	}
}

// TestRaizDelProyectoSubeHastaElManifiesto: `check` sin bandera busca el proyecto desde la carpeta del
// fichero, que es lo que hace el comando con el temporal de la pestaña.
func TestRaizDelProyectoSubeHastaElManifiesto(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	if got := raizDelProyecto(filepath.Join(raiz, "src", "com", "hola", "apkcstudio")); got != raiz {
		t.Fatalf("la raiz es %q, medida %q", got, raiz)
	}
	if got := raizDelProyecto(t.TempDir()); got != "" {
		t.Fatalf("un directorio sin manifiesto devuelve %q y deberia ser vacio", got)
	}
}

// TestCheckNoEscribeNada: el diagnostico no puede ensuciar el arbol del proyecto; el enlace de la R se
// hace en memoria. Se mira que no aparezca ni build/ ni R.java donde no estaba.
func TestCheckNoEscribeNada(t *testing.T) {
	raiz, fuente := proyectoDeCheck(t)
	if r := revisarFuenteJava(fuente, "MainActivity.java", jarDelSDK(t), raiz); !r.Emisor {
		t.Fatalf("el check no compilo: %+v", r)
	}
	for _, ruta := range []string{"build", "src/com/hola/apkcstudio/R.java"} {
		if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash(ruta))); err == nil {
			t.Errorf("el check escribio %s", ruta)
		}
	}
}
