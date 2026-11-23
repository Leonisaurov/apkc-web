package main

// Tests de `apkc check`, el diagnóstico en vivo de la pestaña. El caso que los abrió (§200) es el que
// el usuario veía como falso error: `setContentView(R.layout.main)`, donde la `R` no viaja en el
// android.jar y solo existe si el proyecto la trae generada o se enlaza de `res/`.

import (
	"fmt"
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

// fuenteApkCualificada es el caso de D4: un campo de la propia clase escrito con el receptor `this`,
// que es la forma que trae el `Apk.java` del usuario. La asignacion cae en la linea 7.
const fuenteApkCualificada = `package com.hola.apkcstudio;

public class Apk {
    private String filePath;

    public Apk(String filePath) {
        this.filePath = filePath;
    }

    public String ruta() {
        return filePath;
    }
}
`

// TestCheckAsignacionCualificadaThis: `this.filePath = filePath` es un campo de la propia clase, no
// dos nombres sin medir. El emisor tiene que resolver el receptor `this` y el campo contra la clase y
// dejar el fuente en cero errores, igual que `filePath = filePath`.
func TestCheckAsignacionCualificadaThis(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteApkCualificada, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("`this.filePath = filePath` da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

// TestCheckAsignacionCualificadaNombraElCampo es el control negativo: si el campo no existe, el error
// tiene que nombrar el campo que falta (`noExiste`) y no volver al «mas de un nombre en el destino».
func TestCheckAsignacionCualificadaNombraElCampo(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	roto := strings.Replace(fuenteApkCualificada, "this.filePath = filePath;", "this.noExiste = filePath;", 1)
	if roto == fuenteApkCualificada {
		t.Fatal("no se sustituyo la asignacion cualificada")
	}
	r := revisarFuenteJava(roto, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if !strings.Contains(e.Mensaje, "noExiste") {
		t.Errorf("el mensaje no nombra el campo que falta: %q", e.Mensaje)
	}
	if strings.Contains(e.Mensaje, "mas de un nombre en el destino") {
		t.Errorf("el mensaje sigue siendo el generico de antes: %q", e.Mensaje)
	}
	if e.Linea != 7 || e.Posicion != "Apk.java:7" {
		t.Errorf("el error cae en %q y la asignacion esta en la linea 7", e.Posicion)
	}
}

// fuenteApkReceptor es el caso del usuario: `appInfo.sourceDir = filePath`, con el receptor en un
// parámetro de tipo `ApplicationInfo`. La asignación cae en la línea 7.
const fuenteApkReceptor = `package com.hola.apkcstudio;

import android.content.pm.ApplicationInfo;

public class Apk {
    public void lee(ApplicationInfo appInfo, String filePath) {
        appInfo.sourceDir = filePath;
    }
}
`

// TestCheckAsignacionPorReceptor: `appInfo.sourceDir = filePath` tiene el receptor en un parámetro con
// tipo conocido, así que el emisor busca `sourceDir` en `ApplicationInfo` y deja el fuente en cero
// errores. Era el caso que caía en «una asignacion con mas de un nombre en el destino».
func TestCheckAsignacionPorReceptor(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteApkReceptor, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("`appInfo.sourceDir = filePath` da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

// TestCheckAsignacionPorReceptorNombraElCampo es el control negativo del receptor con tipo: si el campo
// no existe en su clase, el error tiene que nombrar `noExiste` y no volver al mensaje genérico.
func TestCheckAsignacionPorReceptorNombraElCampo(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	roto := strings.Replace(fuenteApkReceptor, "appInfo.sourceDir = filePath;", "appInfo.noExiste = filePath;", 1)
	if roto == fuenteApkReceptor {
		t.Fatal("no se sustituyo la asignacion por receptor")
	}
	r := revisarFuenteJava(roto, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if !strings.Contains(e.Mensaje, "noExiste") {
		t.Errorf("el mensaje no nombra el campo que falta: %q", e.Mensaje)
	}
	if strings.Contains(e.Mensaje, "mas de un nombre en el destino") {
		t.Errorf("el mensaje sigue siendo el generico de antes: %q", e.Mensaje)
	}
	if e.Linea != 7 || e.Posicion != "Apk.java:7" {
		t.Errorf("el error cae en %q y la asignacion esta en la linea 7", e.Posicion)
	}
}

// TestCheckAsignacionReceptorSinTipo: un receptor que no es local, ni parámetro, ni campo, ni el nombre
// de un tipo no tiene tipo conocido, y el error lo dice nombrando el receptor en vez de adivinar.
func TestCheckAsignacionReceptorSinTipo(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	roto := strings.Replace(fuenteApkReceptor, "public void lee(ApplicationInfo appInfo, String filePath) {",
		"public void lee(String filePath) {", 1)
	if roto == fuenteApkReceptor {
		t.Fatal("no se quito el parametro del receptor")
	}
	r := revisarFuenteJava(roto, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if !strings.Contains(e.Mensaje, "no se conoce el tipo del receptor appInfo") {
		t.Errorf("el mensaje no dice que el receptor no tiene tipo conocido: %q", e.Mensaje)
	}
	if strings.Contains(e.Mensaje, "mas de un nombre en el destino") {
		t.Errorf("el mensaje sigue siendo el generico de antes: %q", e.Mensaje)
	}
	if e.Linea != 7 {
		t.Errorf("el error cae en la linea %d y la asignacion esta en la 7", e.Linea)
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

// TestCheckColumnaEnCaracteres fija la convención del JSON: la columna va en **caracteres** (runes) y
// desde 1, no en bytes ni en unidades UTF-16. Un emoji antes del token cuenta uno; el editor traduce
// esa columna a unidades UTF-16 para cortar y a píxeles para subrayar (studio/diagnostico-vivo.js).
func TestCheckColumnaEnCaracteres(t *testing.T) {
	raiz, fuente := proyectoDeCheck(t)
	linea := "        /* \U0001F600 */ setContentView(R.layout.noexiste);"
	roto := strings.Replace(fuente, "        setContentView(R.layout.main);", linea, 1)
	if roto == fuente {
		t.Fatal("no se sustituyó la línea del setContentView")
	}
	r := revisarFuenteJava(roto, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if e.Linea != 15 {
		t.Fatalf("el error cae en la línea %d y el setContentView está en la 15", e.Linea)
	}
	antes := linea[:strings.Index(linea, "R.layout.noexiste")]
	if quiero := len([]rune(antes)) + 1; e.Columna != quiero {
		t.Errorf("la columna es %d y la R está en el carácter %d de %q", e.Columna, quiero, linea)
	}
	if e.Columna == len(antes)+1 || e.Columna == len([]byte(antes)) {
		t.Errorf("la columna %d parece contada en bytes o en unidades UTF-16, no en caracteres", e.Columna)
	}
	if quiero := fmt.Sprintf("MainActivity.java:15:%d", e.Columna); e.Posicion != quiero {
		t.Errorf("la posición es %q, medida %q", e.Posicion, quiero)
	}
}

// fuenteNuloEnCampoPrimitivo: null no cabe en un campo int. El emisor debe rechazarlo con el
// descriptor esperado (I) frente al que tipa el literal (Ljava/lang/Object;), no aceptarlo de paso.
const fuenteNuloEnCampoPrimitivo = `package com.hola.apkcstudio;

public class Apk {
    private int n;

    public Apk() {
        n = null;
    }
}
`

func TestCheckAsignacionNulaAUnCampoPrimitivo(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteNuloEnCampoPrimitivo, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("`n = null` dio %d errores: %+v", len(r.Errores), r.Errores)
	}
	mensaje := r.Errores[0].Mensaje
	if !strings.Contains(mensaje, "n es I y se le asigna un") {
		t.Fatalf("el error no nombra el tipo del campo: %q", mensaje)
	}
}

// TestCheckBloqueSinCerrarApuntaALaUltimaLinea: una llave sin cerrar se atribuye al fin de fichero, y
// ese token tiene que estar en la **última línea con contenido**. Antes se contaba la línea que hay
// tras el salto final, y el usuario veía la marca una línea por debajo del `}` que sí está escrito.
// fuenteNuloEnCampo: el error del usuario, `icon = null;` donde `icon` es un campo de tipo Drawable.
// El literal null no tiene tipo propio: en un hueco de referencia debe tomar el del campo, no Object.
const fuenteNuloEnCampo = `package com.hola.apkcstudio;

import android.graphics.drawable.Drawable;

public class Apk {
    private Drawable icon;

    public Apk() {
        icon = null;
    }
}
`

// TestCheckAsignacionNulaAUnCampo fija que asignar null a un campo de referencia no deje error.
func TestCheckAsignacionNulaAUnCampo(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteNuloEnCampo, "Apk.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("`icon = null;` da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

func TestCheckBloqueSinCerrarApuntaALaUltimaLinea(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	// cinco líneas de contenido y un salto final: el `}` de x() está en la 5 y la clase queda abierta
	fuente := `package com.hola.apkcstudio;

public class MainActivity extends Activity {
    void x() {
    }
`
	r := revisarFuenteJava(fuente, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if e.Linea != 5 {
		t.Errorf("el error se pone en la línea %d y la última con contenido es la 5 (%q)", e.Linea, e.Posicion)
	}
	// el error es del parser: lleva la línea y no la columna (frontera escrita en el plan), pero la
	// posición no puede apuntar a la línea vacía de después del salto
	if e.Posicion != "MainActivity.java:5" {
		t.Errorf("la posición es %q, medida \"MainActivity.java:5\"", e.Posicion)
	}
	if e.Mensaje == "" || strings.Contains(e.Mensaje, "linea 5") {
		t.Errorf("el mensaje no es una explicación limpia: %q", e.Mensaje)
	}
	// el fuente sin salto final tiene que seguir apuntando a su última línea
	sinSalto := strings.TrimSuffix(fuente, "\n")
	r = revisarFuenteJava(sinSalto, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 || r.Errores[0].Linea != 5 {
		t.Fatalf("sin salto final el error no cae en la línea 5: %+v", r.Errores)
	}
}
