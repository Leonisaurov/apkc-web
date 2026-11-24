package main

// §241: los comodines en los argumentos de tipo, `?`, `? extends T` y `? super T`. El que abrió este
// frente fue el `AdapterView<?>` de la línea 54 de `MainActivity.java` del zip de ApkShare:
//
//	src/share/apps/util/MainActivity.java: linea 54: el caracter '?' no es de este subconjunto
//
// El comodín no toca el descriptor —el borrado lo tira: `AdapterView<?>` es `Landroid/widget/AdapterView;`—
// y solo viaja en la `Signature` genérica, con los signos de la JVMS 4.7.9.1: `*` para `?`, `+` delante
// de la cota de `? extends T` y `-` delante de la de `? super T`. El oráculo es javac 17
// `--release 8 -g:lines,source`: la `Signature` método a método y el dex entero byte a byte.

import (
	"strings"
	"testing"
)

// fuenteComodines es la línea 54 real —`AdapterView<?>`— más las dos cotas que ese fuente no trae pero
// la misma gramática sí, en un campo, en un parámetro y en el retorno.
const fuenteComodines = `package com.hola.apkcstudio;

import android.widget.AdapterView;
import android.view.View;
import java.util.List;

public class Pantalla {
    private List<?> cosas;

    public void onItemClick(AdapterView<?> adapter, View view, int positon, long id) {
    }

    public void acota(List<? extends Number> numeros, List<? super String> destino) {
    }

    public List<?> todas() {
        return cosas;
    }

    public List<? extends CharSequence> textos() {
        return null;
    }
}
`

// fuenteListenerComodin es la escucha anónima de `MainActivity.java:52-54`: el comodín vive en el
// método de la anónima, así que su `.class` es `Pantalla$1` y la `Signature` del método es la que
// tiene que coincidir con la de javac.
const fuenteListenerComodin = `package com.hola.apkcstudio;

import android.widget.AdapterView;
import android.view.View;

public class Pantalla extends android.app.Activity {
    public void preparar(AdapterView<?> lista) {
        lista.setOnItemClickListener(new AdapterView.OnItemClickListener() {
            @Override
            public void onItemClick(AdapterView<?> adapter, View view, int positon, long id) {
            }
        });
    }
}
`

// firmaDeMetodoDe devuelve el descriptor y la `Signature` del método con ese nombre en la clase.
func firmaDeMetodoDe(t *testing.T, clase classFull, nombre string) (string, string) {
	t.Helper()
	for _, m := range clase.Methods {
		if m.Name == nombre {
			return m.Desc, m.Signature
		}
	}
	t.Fatalf("%s no tiene el método %s", clase.Name, nombre)
	return "", ""
}

// firmaDeCampoDe devuelve el descriptor y la `Signature` del campo con ese nombre.
func firmaDeCampoDe(t *testing.T, clase classFull, nombre string) (string, string) {
	t.Helper()
	for _, f := range clase.Fields {
		if f.Name == nombre {
			return f.Desc, f.Signature
		}
	}
	t.Fatalf("%s no tiene el campo %s", clase.Name, nombre)
	return "", ""
}

// fuenteMainActivityDelUsuario son las líneas 1-54 de `src/share/apps/util/MainActivity.java` del zip
// de ApkShare, verbatim, con las llaves que cierran el método, la anónima, `onCreate` y la clase. La
// línea 54 es la que el usuario leyó en el rechazo; el resto está porque es lo que el parser tiene que
// atravesar para llegar a ella.
const fuenteMainActivityDelUsuario = `package share.apps.util;

import android.app.*;
import android.os.*;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.*;
import android.widget.*;
import android.graphics.*;
import android.graphics.drawable.ColorDrawable;
import android.util.*;
import android.net.*;

import android.content.pm.PackageManager;
import android.content.Intent;
import android.content.pm.PackageInfo;

import android.widget.Toast;
import android.view.View;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.ArrayList;
import java.util.List;
import java.util.Arrays;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import android.support.v4.content.FileProvider;

public class MainActivity extends Activity {
    private static final int AutoResearchDelay = 500;

    private Handler handler = new Handler(Looper.getMainLooper());
    private Runnable runnable;

    private AlertDialog loading_dialog;
    private boolean loading_dialog_open;
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);

        ListView apps = (ListView) findViewById(R.id.apps);

        PackageManager pm = getPackageManager();
        List<App> apps_list = new ArrayList<>();
        apps.setOnItemClickListener(new AdapterView.OnItemClickListener() {
            @Override
            public void onItemClick(AdapterView<?> adapter, View view, int positon, long id) {
            }
        });
    }
}
`

// TestComodinesDeTipoFalsificacion: la falsificación de la ruta nueva. Con el interruptor apagado, el
// fuente del usuario tiene que volver a fallar con el mensaje que leyó, «el caracter '?' no es de
// este subconjunto», en la misma línea y con el prefijo del fichero que pone `compilarProyectoJava`.
func TestComodinesDeTipoFalsificacion(t *testing.T) {
	const literal = "src/share/apps/util/MainActivity.java: linea 54: el caracter '?' no es de este subconjunto"
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/share/apps/util/MainActivity.java", contenido: fuenteMainActivityDelUsuario}}

	// encendida, la línea 54 se acepta; el fuente del zip se para después en la línea 38 —el
	// `new Handler(Looper.getMainLooper())`, otra frontera—, y eso es lo que se espera aquí
	if _, err := compilarProyectoJava(fuentes, jar); err != nil && strings.Contains(err.Error(), "linea 54:") {
		t.Fatalf("con los comodines encendidos la línea 54 no se aceptó: %v", err)
	} else if err != nil {
		t.Logf("con los comodines encendidos, la frontera siguiente: %v", err)
	}

	anterior := leeComodinesDeTipo
	leeComodinesDeTipo = false
	t.Cleanup(func() { leeComodinesDeTipo = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if err.Error() != literal {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario %q", err, literal)
	}
	if !strings.Contains(err.Error(), "linea 54") {
		t.Fatalf("el rechazo no nombra la línea 54: %v", err)
	}
	t.Logf("sin la ruta: %v", err)
}
