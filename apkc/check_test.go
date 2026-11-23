package main

// Tests de `apkc check`, el diagnóstico en vivo de la pestaña. El caso que los abrió (§200) es el que
// el usuario veía como falso error: `setContentView(R.layout.main)`, donde la `R` no viaja en el
// android.jar y solo existe si el proyecto la trae generada o se enlaza de `res/`.

import (
	"archive/zip"
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

// TestCheckRecursosSinMedirNoFalsifica: un `res/` que el enlazador de Go no sabe escribir (un color
// state list) no puede dejar el informe en verde por su cuenta ni romper el check: se dice por qué
// no hay R y el error del fuente sigue siendo el de la R que falta. Este caso usaba un drawable XML,
// pero desde el frente de recursos de AAR ese tipo ya se enlaza: el que sigue sin medirse es `color`.
func TestCheckRecursosSinMedirNoFalsifica(t *testing.T) {
	raiz, fuente := proyectoDeCheck(t)
	pon(t, raiz, "res/color/estado.xml", `<selector xmlns:android="http://schemas.android.com/apk/res/android"/>`)
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

// TestCheckAprovechaElIndiceDelJar fija la parte de I5 que `check` usa: cuando el proyecto tiene
// build/ —el estado tras un `apkc build`— la tabla del framework sale del índice que dejó el build
// en build/jarindice y su parseo se reutiliza de build/marco, sin que archive/zip recorra el jar ni
// se inflen los 16 MB de resources.arsc otra vez. Los contadores son lo único que distingue los dos
// caminos. Y sin build/ el check no lo crea: el diagnóstico no escribe en el árbol del proyecto, que
// es el contrato de esta fase.
func TestCheckAprovechaElIndiceDelJar(t *testing.T) {
	jar := jarDelSDK(t)
	raiz, fuente := proyectoDeCheck(t)

	// proyecto recién creado: no hay build/, así que se enlaza res/ y no se deja nada escrito
	r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz)
	if len(r.Errores) != 0 || !r.Emisor {
		t.Fatalf("sin build/ el check no compiló: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(raiz, "build")); err == nil {
		t.Fatal("el check creó build/: no puede dejar directorios en el proyecto")
	}

	// con build/ (lo que deja un build), la primera pasada escribe el índice y el marco del
	// framework; la segunda sirve el marco de la caché y no vuelve a extraer ni a parsear nada
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	extracciones := extraccionesDelIndice
	if r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz); len(r.Errores) != 0 {
		t.Fatalf("con build/ el check da errores: %+v", r.Errores)
	}
	if extraccionesDelIndice == extracciones {
		t.Fatal("la primera pasada con build/ no sacó resources.arsc del índice: lo abrió con archive/zip")
	}
	marcos, parseos := marcosDeCache, parseosDelDirectorio
	if r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz); len(r.Errores) != 0 || !r.Emisor {
		t.Fatalf("la segunda pasada con el índice no compiló: %+v", r)
	}
	if marcosDeCache == marcos {
		t.Fatal("la segunda pasada volvió a parsear resources.arsc teniendo el marco cacheado")
	}
	if parseosDelDirectorio != parseos {
		t.Fatal("la segunda pasada volvió a parsear el directorio central teniendo el índice")
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

// fuenteClausulaGenerica es el caso del usuario: `extends ArrayAdapter<Apk>`. El argumento `Apk` no
// cambia el descriptor de la superclase —es el borrado—, pero antes el analizador se paraba en el `<`
// con «se esperaba "{"». La clase `Apk` va en el mismo fichero para que se resuelva sin más fuentes.
const fuenteClausulaGenerica = `package com.hola.apkcstudio;

import android.content.Context;
import android.view.View;
import android.view.ViewGroup;
import android.widget.ArrayAdapter;

public class ApkAdapter extends ArrayAdapter<Apk> {
    public ApkAdapter(Context contexto) {
        super(contexto, 0);
    }

    @Override
    public View getView(int posicion, View reusada, ViewGroup padre) {
        return reusada;
    }
}

class Apk {
    private String ruta;

    Apk(String ruta) {
        this.ruta = ruta;
    }
}
`

// TestCheckClausulaGenericaEnExtends: `extends ArrayAdapter<Apk>` no puede dar ni un error, y el emisor
// tiene que correr: el `<` de los argumentos de tipo ya no es el final de la cláusula.
func TestCheckClausulaGenericaEnExtends(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteClausulaGenerica, "ApkAdapter.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("`extends ArrayAdapter<Apk>` da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

// fuenteClausulaGenericaCualificada: el mismo caso con el tipo cualificado, sin import. El nombre que
// va al `super_class` es el crudo (`android/widget/ArrayAdapter`) y los argumentos no lo tocan.
const fuenteClausulaGenericaCualificada = `package com.hola.apkcstudio;

public class ApkAdapter extends android.widget.ArrayAdapter<Apk> {
    public ApkAdapter() {
        super(null, 0);
    }
}

class Apk {
}
`

func TestCheckClausulaGenericaCualificada(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteClausulaGenericaCualificada, "ApkAdapter.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("`extends android.widget.ArrayAdapter<Apk>` da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

// fuenteClausulaGenericaEnImplements: el mismo `<>` en una interface. `Parcelable.Creator<T>` es la
// interface genérica del android.jar; una clase abstracta no tiene que implementar sus métodos.
const fuenteClausulaGenericaEnImplements = `package com.hola.apkcstudio;

import android.os.Parcelable;

public abstract class Creador implements Parcelable.Creator<Creador> {
}
`

func TestCheckClausulaGenericaEnImplements(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteClausulaGenericaEnImplements, "Creador.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("`implements Parcelable.Creator<Creador>` da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

// TestCheckClausulaGenericaNombraElTipoBase: el control negativo. Aceptar el `<>` no puede tapar que
// la clase base no existe: el error tiene que nombrar `NoExiste`, no el `<`.
const fuenteClausulaGenericaSinBase = `package com.hola.apkcstudio;

public class Roto extends NoExiste<Apk> {
}

class Apk {
}
`

func TestCheckClausulaGenericaNombraElTipoBase(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteClausulaGenericaSinBase, "Roto.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	mensaje := r.Errores[0].Mensaje
	if !strings.Contains(mensaje, "NoExiste") {
		t.Errorf("el error no nombra el tipo base que falta: %q", mensaje)
	}
	if strings.Contains(mensaje, "se esperaba") {
		t.Errorf("el error sigue siendo el del parser en el `<`: %q", mensaje)
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

// fuenteCampoGenerico es el caso del usuario: el campo con `final` y `List<Apk>` era «se esperaba un
// nombre de miembro y hay "<"». Añade el anidado, el cualificado, el arreglo y un parámetro y una
// local genéricos, que van por el mismo lector del tipo. `Apk` va en el mismo fichero para que el
// argumento se resuelva sin más fuentes.
const fuenteCampoGenerico = `package com.hola.apkcstudio;

import java.util.List;
import java.util.Map;

public class ApkAdapter {
    private final List<Apk> apks;
    private Map<String, List<Apk>> porNombre;
    private android.widget.ArrayAdapter<Apk> adaptador;
    private List<Apk>[] tandas;

    public ApkAdapter() {
        this.apks = null;
    }

    public void guardar(List<Apk> copia) {
        List<Apk> otra = copia;
        porNombre = null;
    }
}

class Apk {
    private String ruta;

    Apk(String ruta) {
        this.ruta = ruta;
    }
}
`

// TestCheckCampoGenerico: `private final List<Apk> apks;` no puede dar ni un error y el emisor tiene
// que correr. El `<` del tipo ya no es el final de la declaración, y `final` se aceptaba ya.
func TestCheckCampoGenerico(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteCampoGenerico, "ApkAdapter.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el campo genérico da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrio: aviso %q, recursos %q", r.Aviso, r.Recursos)
	}
}

// fuenteHelper es una clase del proyecto que vive en otro fichero. El defecto que abrió este frente:
// `check` compilaba solo el fuente pedido y `Helper` no existía para el emisor.
const fuenteHelper = `package com.hola.apkcstudio;

public class Helper {
    public static int doble(int x) {
        return x + x;
    }
}
`

// fuenteMainUsaHelper usa una clase que no está en su propio fichero.
const fuenteMainUsaHelper = `package com.hola.apkcstudio;

public class MainActivity {
    public int usa() {
        return Helper.doble(2);
    }
}
`

// TestCheckCompilaLosDemasFuentesDelProyecto: con `--proyecto` (aquí, la raíz) `check` lee todos los
// `.java` del proyecto y los compila juntos, así que `Helper.doble` resuelve aunque viva en otro
// fichero. Deja además una copia rota del fichero pedido en disco: el fuente que se diagnostica es el
// que se pasa, no el guardado, y la ruta/nombre del pedido lo saca del conjunto.
func TestCheckCompilaLosDemasFuentesDelProyecto(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/Helper.java", fuenteHelper)
	pon(t, raiz, "src/com/hola/apkcstudio/MainActivity.java", "public class MainActivity {\n")
	r := revisarFuenteJava(fuenteMainUsaHelper, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("con Helper en otro fichero el fuente da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrió: aviso %q", r.Aviso)
	}
	if r.Aviso != "" {
		t.Fatalf("aviso inesperado: %q", r.Aviso)
	}
}

// TestCheckSigueDiagnosticandoElFuentePedido: reunir los fuentes no puede tapar el error del pedido.
// `Helper` existe, pero `noExiste` no está en él: el error tiene que ser del pedido y nombrar el
// miembro que falta.
func TestCheckSigueDiagnosticandoElFuentePedido(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/Helper.java", fuenteHelper)
	roto := strings.Replace(fuenteMainUsaHelper, "Helper.doble(2)", "Helper.noExiste(2)", 1)
	if roto == fuenteMainUsaHelper {
		t.Fatal("no se sustituyó la llamada al Helper")
	}
	r := revisarFuenteJava(roto, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	e := r.Errores[0]
	if !strings.Contains(e.Mensaje, "noExiste") {
		t.Errorf("el error no nombra el miembro que falta: %q", e.Mensaje)
	}
	if e.Posicion == "" || !strings.HasPrefix(e.Posicion, "MainActivity.java:") {
		t.Errorf("la posición no es del fuente pedido: %q", e.Posicion)
	}
}

// TestCheckNoAtribuyeElErrorDeOtroFuente fija el contrato de los fallos ajenos: si el primer error
// que encuentra el emisor es de otro `.java` del proyecto, no entra en `errores` del pedido; se dice
// en `aviso` con el fichero que falla y el informe no se da por comprobado.
func TestCheckNoAtribuyeElErrorDeOtroFuente(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/Helper.java", "public class Helper {\n")
	r := revisarFuenteJava(fuenteMainUsaHelper, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el error de Helper.java se coló en la lista del pedido: %+v", r.Errores)
	}
	if r.Emisor {
		t.Fatal("se dio por comprobado un fuente cuyo Helper.java no compila")
	}
	if !strings.Contains(r.Aviso, "Helper.java") {
		t.Fatalf("el aviso no nombra el fichero que falla: %q", r.Aviso)
	}
}

// fuenteHelperRoto parsea bien pero no emite: su cuerpo nombra un tipo que no existe. El pedido se
// emite antes (va primero), así que su diagnóstico sí se completa aunque Helper falle al emitirse.
const fuenteHelperRoto = `package com.hola.apkcstudio;

public class Helper {
    public static int doble(int x) {
        return NoExiste.doble(x);
    }
}
`

// TestCheckElFalloDeEmisionDeOtroFuenteNoTapaElPedido: el pedido se emite primero; si después falla
// otro fichero al emitirse, el pedido ya está comprobado (`emisor:true`, cero errores) y el aviso
// nombra al que falla.
func TestCheckElFalloDeEmisionDeOtroFuenteNoTapaElPedido(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/Helper.java", fuenteHelperRoto)
	r := revisarFuenteJava(fuenteMainUsaHelper, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el error de emisión de Helper.java se coló en el pedido: %+v", r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el pedido se emitió antes del fallo y no se marcó: aviso %q", r.Aviso)
	}
	if !strings.Contains(r.Aviso, "Helper.java") || !strings.Contains(r.Aviso, "NoExiste") {
		t.Fatalf("el aviso no nombra el fichero y la causa: %q", r.Aviso)
	}
}

// fuenteCampoGenericoSinImport deja `List` sin importar: el control negativo. El error tiene que
// nombrar el tipo base que falta, no el `<` ni el nombre del campo.
const fuenteCampoGenericoSinImport = `package com.hola.apkcstudio;

public class Roto {
    private List<Apk> apks;
}

class Apk {
}
`

func TestCheckCampoGenericoNombraElTipoBase(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	r := revisarFuenteJava(fuenteCampoGenericoSinImport, "Roto.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 1 {
		t.Fatalf("%d errores, medido uno: %+v", len(r.Errores), r.Errores)
	}
	mensaje := r.Errores[0].Mensaje
	if !strings.Contains(mensaje, "List") {
		t.Errorf("el error no nombra el tipo base que falta: %q", mensaje)
	}
	if strings.Contains(mensaje, "se esperaba un nombre de miembro") {
		t.Errorf("el error sigue siendo el del parser en el `<`: %q", mensaje)
	}
}

// fuenteMainSolo es un pedido que no depende de ninguna otra clase del proyecto: sirve para medir lo
// que aportan los hermanos sin que el propio pedido tape el resultado con un error suyo.
const fuenteMainSolo = `package com.hola.apkcstudio;

public class MainActivity {
    public int uno() {
        return 1;
    }
}
`

// fuenteHelperUno y fuenteHelperDos son dos clases con el mismo nombre simple en paquetes distintos.
// El pedido (`com.hola.apkcstudio.uno.Helper`) referencia a la otra por su nombre cualificado en la
// firma, que es legal sin `import` y la única forma de nombrarla: dentro de su propio fuente el nombre
// simple `Helper` ya es su clase.
const fuenteHelperUno = `package com.hola.apkcstudio.uno;

public class Helper {
    public static int suma(com.hola.apkcstudio.dos.Helper otro) {
        return otro.hashCode() + 1;
    }
}
`

const fuenteHelperDos = `package com.hola.apkcstudio.dos;

public class Helper {
    public static int doble(int x) {
        return x + x;
    }
}
`

// TestCheckDosHelperEnPaquetesDistintos cierra el límite que dejó `a4bf224`: el fuente pedido se
// saltaba del conjunto por su **nombre simple**, y con eso se llevaba por delante a cualquier otro
// `.java` que se llamase igual, aunque viviera en otro paquete y el pedido lo referenciara. Aquí el
// pedido es `com.hola.apkcstudio.uno.Helper` y el proyecto tiene además `com.hola.apkcstudio.dos.Helper`:
// el del otro paquete tiene que entrar en el conjunto y la copia en disco del pedido, no.
func TestCheckDosHelperEnPaquetesDistintos(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/uno/Helper.java", fuenteHelperUno)
	pon(t, raiz, "src/com/hola/apkcstudio/dos/Helper.java", fuenteHelperDos)
	r := revisarFuenteJava(fuenteHelperUno, "Helper.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("con Helper en dos paquetes el pedido da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrió: aviso %q", r.Aviso)
	}
	if r.Aviso != "" {
		t.Fatalf("aviso inesperado: %q", r.Aviso)
	}
	// el conjunto de hermanos es el que decide: el Helper del otro paquete entra y la copia en disco
	// del pedido se salta por su nombre cualificado, no solo por el nombre simple
	otros := otrosFuentesDelProyecto(raiz, "", "Helper.java", fuenteHelperUno)
	queridos := map[string]bool{
		"src/com/hola/apkcstudio/dos/Helper.java":   true,
		"src/com/hola/apkcstudio/MainActivity.java": true,
	}
	for _, f := range otros {
		if !queridos[f.nombre] {
			t.Fatalf("hermano inesperado %q en %+v", f.nombre, nombresDeFuentes(otros))
		}
		delete(queridos, f.nombre)
	}
	if len(queridos) != 0 {
		t.Fatalf("faltan hermanos: %+v (medidos %+v)", queridos, nombresDeFuentes(otros))
	}
}

// nombresDeFuentes deja legible un conjunto de fuentes en un fallo de test.
func nombresDeFuentes(fuentes []fuenteJava) []string {
	salida := make([]string, 0, len(fuentes))
	for _, f := range fuentes {
		salida = append(salida, f.nombre)
	}
	return salida
}

// TestCheckHermanoQueNoParseaSeAvisa cubre el otro límite declarado: un fuente hermano que ni siquiera
// parsea no puede convertirse en una fila de error del fichero pedido. El informe dice el fichero y la
// causa en `aviso`, no da el pedido por comprobado (`emisor:false`) y deja `errores` vacío.
func TestCheckHermanoQueNoParseaSeAvisa(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	pon(t, raiz, "src/com/hola/apkcstudio/Roto.java", "public class Roto {\n")
	r := revisarFuenteJava(fuenteMainSolo, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el fallo de Roto.java se coló en la lista del pedido: %+v", r.Errores)
	}
	if r.Emisor {
		t.Fatal("se dio por comprobado un fuente cuyo hermano Roto.java no parsea")
	}
	if !strings.Contains(r.Aviso, "Roto.java") {
		t.Fatalf("el aviso no nombra el fichero que no parsea: %q", r.Aviso)
	}
	// la causa del parser tiene que viajar con el aviso: sin ella el aviso no sirve para arreglarlo
	if !strings.Contains(r.Aviso, "se esperaba") && !strings.Contains(r.Aviso, "sin cerrar") {
		t.Fatalf("el aviso no dice la causa del parser: %q", r.Aviso)
	}
	if strings.HasPrefix(r.Aviso, "MainActivity.java") {
		t.Fatalf("el aviso atribuye el fallo al fuente pedido: %q", r.Aviso)
	}
}

// fuenteTipoDelJar es la clase de una dependencia local: `jar/` la aporta al classpath y un hermano la usa.
const fuenteTipoDelJar = `package com.hola.apkcstudio.dep;

public class MiTipo {
    public static int valor() {
        return 7;
    }
}
`

// fuenteUsaDep es el hermano que referencia el tipo del jar; el pedido no lo toca.
const fuenteUsaDep = `package com.hola.apkcstudio;

import com.hola.apkcstudio.dep.MiTipo;

public class UsaDep {
    public int v() {
        return MiTipo.valor();
    }
}
`

// jarConClasesEmitidas escribe en `jar/<nombre>` las clases que emite apkc para esos fuentes. Emitirlas
// aquí evita depender de javac y mide lo único que importa: que `check` abra el jar del proyecto.
func jarConClasesEmitidas(t *testing.T, raiz, nombre, rutaJar string, fuentes []fuenteJava) string {
	t.Helper()
	clases, err := compilarProyectoJava(fuentes, rutaJar)
	if err != nil {
		t.Fatalf("preparando el jar de la prueba: %v", err)
	}
	ruta := filepath.Join(raiz, "jar", nombre)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, c := range clases {
		w, err := zw.Create(c.interno + ".class")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(c.datos); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// TestCheckTipoDeJarLocalEnUnHermano cierra el tercer caso: un tipo que vive en `jar/` y que solo
// referencia otro fuente del proyecto tiene que resolver, porque `check` arma el mismo classpath que
// `build`. Antes el classpath se armaba con la raíz vacía (el cwd) y el jar del proyecto no entraba.
func TestCheckTipoDeJarLocalEnUnHermano(t *testing.T) {
	raiz, _ := proyectoDeCheck(t)
	jarConClasesEmitidas(t, raiz, "dependencia.jar", jarDelSDK(t), []fuenteJava{{nombre: "MiTipo.java", contenido: fuenteTipoDelJar}})
	pon(t, raiz, "src/com/hola/apkcstudio/UsaDep.java", fuenteUsaDep)
	r := revisarFuenteJava(fuenteMainSolo, "MainActivity.java", jarDelSDK(t), raiz)
	if len(r.Errores) != 0 {
		t.Fatalf("el tipo de jar/ en un hermano da %d errores: %+v", len(r.Errores), r.Errores)
	}
	if !r.Emisor {
		t.Fatalf("el emisor no corrió: aviso %q", r.Aviso)
	}
	if r.Aviso != "" {
		t.Fatalf("aviso inesperado: %q", r.Aviso)
	}
}
