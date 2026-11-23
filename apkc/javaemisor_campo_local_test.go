package main

// §242: el campo de una local como argumento (`getApks(pm, info.applicationInfo)`, `App.java:28`) y un
// arreglo comparado con null (`info.splitSourceDirs == null`, `App.java:34`). El oráculo es el de
// `compilar_test.go`: los mismos fuentes por el emisor y por javac 17 con `--release 8 -g:lines,source`,
// dexados con el mismo dexer, y los dos dex idénticos byte a byte. Los dos fuentes del caso completo son
// los del zip de ApkShare (`src/share/apps/util/App.java` y `Apk.java`), literales; los dos reducidos
// aíslan una rama cada uno para que su fallo nombre la rama.

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// fuenteAppApkShare es `src/share/apps/util/App.java` del zip de ApkShare, copiado literal: el
// `getApks(pm, info.applicationInfo)` de la línea 28 y el `info.splitSourceDirs == null` de la 34.
const fuenteAppApkShare = `package share.apps.util;

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

// fuenteApkApkShare es `src/share/apps/util/Apk.java` del mismo zip, literal: la clase que `App` usa
// como tipo del campo y del `new`, para que el caso completo sea el del usuario.
const fuenteApkApkShare = `package share.apps.util;

import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.graphics.drawable.Drawable;
import android.content.pm.ApplicationInfo;

public class Apk {
    public final String name;
    public final Drawable icon;
    public final String filePath;

    public Apk(PackageManager pm, String filePath) {
        this.filePath = filePath;
        PackageInfo info = pm.getPackageArchiveInfo(filePath, 
            PackageManager.GET_ACTIVITIES |
            PackageManager.GET_META_DATA);
        if (info != null) {
            ApplicationInfo appInfo = info.applicationInfo;
            name = pm.getApplicationLabel(appInfo).toString();
            appInfo.sourceDir = filePath;
            appInfo.publicSourceDir = filePath;
            icon = appInfo.loadIcon(pm);
            return;
        }
        icon = null;
        String name = filePath.substring(filePath.lastIndexOf('/') + 1, filePath.length());
        this.name = name.substring(0, name.lastIndexOf(".apk"));
    }
    
    @Override
    public String toString() {
        return filePath + " - " + name;
    }
}
`

// fuenteCampoDeLocal: solo la rama de `App.java:28`, sin el `if` del 34: el campo de una local como
// argumento de una llamada. Si la estimación del argumento se rompe, este test cae solo.
const fuenteCampoDeLocal = `package com.hola.apkcstudio;

import android.content.pm.ApplicationInfo;
import android.content.pm.PackageInfo;

public class Argumento {
    public final String ruta;

    public Argumento(PackageInfo info) {
        ruta = rutaDe(info.applicationInfo);
    }

    private String rutaDe(ApplicationInfo info) {
        return info.sourceDir;
    }
}
`

// fuenteArregloNulo: solo la rama de `App.java:34`: un `[Ljava/lang/String;` comparado con null. El
// opcode tiene que ser `ifnonnull`/`ifnull` sobre la referencia, no una comparación de enteros.
const fuenteArregloNulo = `package com.hola.apkcstudio;

import android.content.pm.ApplicationInfo;

public class Nulo {
    int m(ApplicationInfo info) {
        if (info.splitSourceDirs == null)
            return 1;
        return 0;
    }
}
`

// clasesDeJavac17: los fuentes por javac 17, con las mismas opciones con las que se midió el resto del
// subconjunto (versión 52 y las líneas).
func clasesDeJavac17(t *testing.T, jar, javac, raiz string, nombres ...string) []classFull {
	t.Helper()
	salida := filepath.Join(raiz, "javac-out")
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida}
	for _, n := range nombres {
		args = append(args, filepath.Join(raiz, n))
	}
	if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	clases, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	return clases
}

// nuestrasClases: los fuentes por el emisor. El dex lo compara `compararDex` (javaemisor_alcance_test.go).
func nuestrasClases(t *testing.T, jar string, fuentes []fuenteJava) []claseEmitida {
	t.Helper()
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo el caso: %v", err)
	}
	return nuestras
}

// TestCampoDeLocalDeApkShareContraJavac17: el `App.java` del zip, entero y literal, por los dos
// caminos. Se compara el dex de **`App` solo**: `Apk.java` trae una diferencia ajena a estas dos ramas
// —no pliega `GET_ACTIVITIES | GET_META_DATA` en `129`— que este frente no toca. El resto de `App`
// entra igual: el constructor, `getApks`, el `for` con dos puntos y `toString`.
func TestCampoDeLocalDeApkShareContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "src/share/apps/util/App.java", fuenteAppApkShare)
	pon(t, raiz, "src/share/apps/util/Apk.java", fuenteApkApkShare)

	nuestras := nuestrasClases(t, jar, []fuenteJava{
		{nombre: "src/share/apps/util/App.java", contenido: fuenteAppApkShare},
		{nombre: "src/share/apps/util/Apk.java", contenido: fuenteApkApkShare},
	})
	suyas := clasesDeJavac17(t, jar, javac, raiz, "src/share/apps/util/App.java", "src/share/apps/util/Apk.java")
	compararDex(t, laClaseNuestra(t, nuestras, "share/apps/util/App"), laClaseDeJavac(t, suyas, "Lshare/apps/util/App;"))
}

// laClaseNuestra y laClaseDeJavac: una clase concreta de las que salieron, para dexar solo esa.
func laClaseNuestra(t *testing.T, clases []claseEmitida, interno string) []claseEmitida {
	t.Helper()
	for _, c := range clases {
		if c.interno == interno {
			return []claseEmitida{c}
		}
	}
	t.Fatalf("el emisor no emitio %s (salieron: %s)", interno, volcadoDeClases(clases))
	return nil
}

func laClaseDeJavac(t *testing.T, clases []classFull, nombre string) []classFull {
	t.Helper()
	for _, c := range clases {
		if c.Name == nombre {
			return []classFull{c}
		}
	}
	t.Fatalf("javac no dejo %s", nombre)
	return nil
}

// TestCampoDeLocalComoArgumentoContraJavac17: la rama del `App.java:28` aislada. El argumento
// `info.applicationInfo` se estima desde el tipo estático del parámetro `info`, sin emitir su receptor.
func TestCampoDeLocalComoArgumentoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Argumento.java", fuenteCampoDeLocal)

	nuestras := nuestrasClases(t, jar, []fuenteJava{{nombre: "Argumento.java", contenido: fuenteCampoDeLocal}})
	compararDex(t, nuestras, clasesDeJavac17(t, jar, javac, raiz, "Argumento.java"))
}

// TestArregloComparadoConNullContraJavac17: la rama del `App.java:34` aislada. javac escribe
// `aload_0; getfield splitSourceDirs:[Ljava/lang/String;; ifnonnull` — el mismo `ifnonnull` de una
// referencia `L`, porque un arreglo también lo es.
func TestArregloComparadoConNullContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Nulo.java", fuenteArregloNulo)

	nuestras := nuestrasClases(t, jar, []fuenteJava{{nombre: "Nulo.java", contenido: fuenteArregloNulo}})
	compararDex(t, nuestras, clasesDeJavac17(t, jar, javac, raiz, "Nulo.java"))
}
