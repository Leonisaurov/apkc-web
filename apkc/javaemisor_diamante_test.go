package main

// §224: el `new` con argumentos de tipo, `new ArrayList<>()` y `new ArrayList<Apk>()`, medido contra
// javac 17. Es el bloqueo que el usuario encontró al compilar ApkShare con `--compiler go`:
//
//	src/share/apps/util/App.java: linea 32: un `new ArrayList` sin constructor no esta medido
//
// El diamante no cambia el descriptor —el `.class` de javac no escribe `Signature` para un `new`, el
// argumento es del tipo de la variable— y el constructor es el del borrado, `()V` o
// `(Ljava/util/Collection;)V`. El oráculo es el de siempre: nuestro dex y el de javac 17 byte a byte,
// y además el `max_stack`/`locals` y las filas de línea método a método.

import (
	"strings"
	"testing"
)

// fuenteAppDelUsuario es `App.java` del zip de ApkShare, verbatim (48 líneas, `linea 32` incluida).
// Solo la usa la falsificación: con la ruta del diamante apagada el parser tiene que devolver el
// mensaje literal que el usuario vio, en la misma línea. No se compila entera porque el `for` con dos
// puntos de la línea 37 es otra frontera, la siguiente que apareció al volver a compilar el proyecto.
const fuenteAppDelUsuario = `package share.apps.util;

import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.graphics.drawable.Drawable;
import android.content.pm.ApplicationInfo;

import java.util.List;
import java.util.ArrayList;

public class App {
    public final String name;
    public final String appPackage;
    public final Drawable icon;
    public final List<Apk> apks;

    // public App(String name, String appPackage, Drawable icon) {
    //     this.name = name;
    //     this.appPackage = appPackage;
    //     this.icon = icon;
    // }

    public App(PackageInfo info, PackageManager pm) {
        appPackage = info.packageName;
        ApplicationInfo appInfo = info.applicationInfo;
        name = pm.getApplicationLabel(appInfo).toString();
        icon = info.applicationInfo.loadIcon(pm);
        apks = getApks(pm, info.applicationInfo);
    }

    private List<Apk> getApks(PackageManager pm, ApplicationInfo info) {
        List<Apk> dirs = new ArrayList<>();
        dirs.add(new Apk(pm, info.sourceDir));
        if (info.splitSourceDirs == null)
            return dirs;

        for (String path: info.splitSourceDirs) {
            dirs.add(new Apk(pm, path));
        }

        return dirs;
    }
    
    @Override
    public String toString() {
        return name + " - " + appPackage;
    }
}
`

// fuenteApkDelDiamante es la clase que viaja como argumento de tipo, en su propio fuente como en el
// proyecto del usuario.
const fuenteApkDelDiamante = `package com.hola.apkcstudio;

public class Apk {
    private String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }

    public String getRuta() {
        return ruta;
    }
}
`

// fuenteAppDiamante es el método `getApks` de App.java con las tres formas del `new` que aparecen en
// el zip: el diamante vacío de App.java:32 y MainActivity.java:51, el diamante con argumento de
// AppAdapter.java:33 —que resuelve al constructor `ArrayList(Collection<? extends E>)`— y la forma
// explícita `new ArrayList<Apk>()`, que no aparece en el zip pero entra en la misma ruta. El `for`
// con dos puntos de la línea 37 se sustituye por un índice: es la frontera siguiente y no la mide
// este párrafo.
const fuenteAppDiamante = `package com.hola.apkcstudio;

import java.util.List;
import java.util.ArrayList;

public class App {
    private List<Apk> getApks(String[] rutas) {
        List<Apk> dirs = new ArrayList<>();
        dirs.add(new Apk(rutas[0]));
        List<Apk> copia = new ArrayList<>(dirs);
        List<Apk> explicita = new ArrayList<Apk>();
        explicita.add(new Apk(rutas[1]));
        return copia;
    }
}
`

// TestDiamanteDelNuevoFalsificacion: la falsificación de la ruta nueva. Con el interruptor apagado,
// el fuente literal de `App.java` tiene que volver a fallar con el mensaje que el usuario leyó,
// «src/share/apps/util/App.java: linea 32: un `new ArrayList` sin constructor no esta medido»: el
// mismo texto, con el prefijo del fichero que pone `compilarProyectoJava`.
func TestDiamanteDelNuevoFalsificacion(t *testing.T) {
	const literal = "src/share/apps/util/App.java: linea 32: un `new ArrayList` sin constructor no esta medido"
	jar := jarDelSDK(t)
	fuentes := []fuenteJava{{nombre: "src/share/apps/util/App.java", contenido: fuenteAppDelUsuario}}

	// encendida, la línea 32 se acepta y el parser sigue hasta la frontera del `for` con dos puntos
	if _, err := compilarProyectoJava(fuentes, jar); err != nil && strings.Contains(err.Error(), "linea 32:") {
		t.Fatalf("con la ruta del diamante encendida la línea 32 no se aceptó: %v", err)
	}

	anterior := leeArgumentosDeTipoDelNuevo
	leeArgumentosDeTipoDelNuevo = false
	t.Cleanup(func() { leeArgumentosDeTipoDelNuevo = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta del diamante apagada el fuente fue aceptado: la falsificación no falsifica")
	}
	if err.Error() != literal {
		t.Fatalf("con la ruta apagada el rechazo es %q, y el del usuario %q", err, literal)
	}
	t.Logf("sin la ruta: %v", err)
}
