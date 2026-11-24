package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Los tres fuentes de ApkShare que hacen falta para el caso de `AppAdapter.java:21/29/33`, copiados
// verbatim del proyecto real (`~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/src/share/apps/util/`).
// Van tal cual, con sus líneas, porque la falsificación tiene que devolver el error literal
// `linea 33` y `linea 64` que daba el usuario, y el oráculo de javac se mide sobre el mismo texto.
// `R.java` no: el del proyecto trae los ids de todo ApkShare y solo se usan tres, así que la prueba
// trae un `R` mínimo con esos tres.

const fuenteAppShare = `package share.apps.util;

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

const fuenteApkShare = `package share.apps.util;

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

const fuenteAppAdapterShare = `package share.apps.util;

import android.content.Context;

import android.view.LayoutInflater;
import android.view.ViewGroup;
import android.view.View;

import android.widget.ArrayAdapter;
import android.widget.TextView;
import android.widget.ImageView;
// import android.widget.Toast;

import android.content.pm.PackageManager;
// import android.content.pm.PackageInfo;
// import android.content.pm.ApplicationInfo;

import java.util.List;
import java.util.ArrayList;

public class AppAdapter extends ArrayAdapter<App> {
    // private final PackageManager pm;
    private final Context context;

    private List<App> apps;
    private List<App> apps_filter;

    // private String query;
    public AppAdapter(Context context, List<App> apps, PackageManager pm) {
        super(context, R.layout.app_layout, apps);
        this.context = context;
        this.apps = apps;
        this.apps_filter = new ArrayList<>(apps);
        // this.pm = pm;
    }

    @Override
    public View getView(int position, View convertView, ViewGroup parent) {
        if (convertView == null) {
            convertView = LayoutInflater.from(context).inflate(R.layout.app_layout, parent, false);
        }

        App info = apps_filter.get(position);
        if (info == null)
            return convertView;

        TextView txt1 = convertView.findViewById(R.id.app_package);
        txt1.setText(info.name);

        ImageView icon = convertView.findViewById(R.id.app_icon);
        icon.setImageDrawable(info.icon);

        return convertView;
    }

    public void searchQuery(String query) {
        // if (query == null)
        //     query = this.query;
        // else
        //     this.query = query;

        apps_filter = new ArrayList<>();
        for (App app: apps) {
            if(app.toString().toLowerCase().contains(query.toLowerCase())) {
                apps_filter.add(app);
            }
        }
        notifyDataSetChanged();
    }

    @Override
    public App getItem(int position) {
        return apps_filter.get(position);
    }

    @Override
    public long getItemId(int position) {
        return position;
    }

    @Override
    public int getCount() {
        return apps_filter.size();
    }
}
`

// fuenteRDeAppAdapterShare: el `R` que usa `AppAdapter`, con los tres ids y sus valores reales del
// `R.java` del proyecto, para que la constante doblada sea la misma que la del build de ApkShare.
const fuenteRDeAppAdapterShare = `package share.apps.util;

public final class R {
  public static final class id {
    public static final int app_icon=0x7f020003;
    public static final int app_package=0x7f020005;
  }
  public static final class layout {
    public static final int app_layout=0x7f030002;
  }
}
`

// fuenteSondaSubida: el caso mínimo de las dos rutas nuevas, sin nada más del proyecto, para que el
// dex entero se pueda comparar byte a byte: la subida de una referencia al escribir un campo
// (`new ArrayList<>()` en un `List`, por `this` y por un receptor que no es `this`) y el booleano
// suelto en la condición (`if (llama())` y `if (!llama())`).
const fuenteSondaSubida = `package p;

import java.util.ArrayList;
import java.util.List;

class Caja {
    List<String> cosas;
}

class Sonda {
    private List<String> lista;
    private List<String> filtrada;

    Sonda(List<String> lista) {
        this.lista = lista;
        this.filtrada = new ArrayList<>(lista);
    }

    void porReceptor(Caja c, ArrayList<String> nuevas) {
        c.cosas = nuevas;
    }

    boolean hay(String q) {
        return lista.contains(q);
    }

    void busca(String q) {
        if (hay(q)) {
            lista.add(q);
        }
        if (!hay(q)) {
            lista.remove(q);
        }
    }
}
`

// fuenteSondaAccesor: la tercera ruta de la subida, el `access$NN2` que escribe un campo privado del
// exterior desde una anónima. Su dex se compara byte a byte con el de javac 17: el constructor de la
// anónima ya no lleva `MethodParameters` (javac 17 no lo escribe en ninguna), así que el pool no mete
// la cadena `$Ldalvik/annotation/MethodParameters;` que antes rompía la comparación.
const fuenteSondaAccesor = `package p;

import java.util.ArrayList;
import java.util.List;

class Sonda {
    private List<String> oculta;

    void esconde() {
        Runnable r = new Runnable() {
            public void run() {
                oculta = new ArrayList<>();
            }
        };
        r.run();
    }
}
`

// fuentesAppAdapterShare: los cuatro fuentes que la prueba compila juntos, en el orden en que los
// nombra el informe del usuario.
func fuentesAppAdapterShare() []fuenteJava {
	return []fuenteJava{
		{nombre: "src/share/apps/util/AppAdapter.java", contenido: fuenteAppAdapterShare},
		{nombre: "src/share/apps/util/App.java", contenido: fuenteAppShare},
		{nombre: "src/share/apps/util/Apk.java", contenido: fuenteApkShare},
		{nombre: "src/share/apps/util/R.java", contenido: fuenteRDeAppAdapterShare},
	}
}

// dexDeNuestrasClases y dexDeLasDeJavac pasan los `.class` de cada lado por el dexer, que es el
// oráculo byte a byte del repo: el `.class` puede diferir en el orden del pool, el dex no.
func dexDeNuestrasClases(t *testing.T, clases []claseEmitida) []byte {
	t.Helper()
	nuestras := make([]classFull, 0, len(clases))
	for _, c := range clases {
		leida, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase %s no se lee: %v", c.interno, err)
		}
		nuestras = append(nuestras, leida)
	}
	dex, err := dexDeClases(nuestras)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	return dex
}

func dexDeLasDeJavac(t *testing.T, clases []classFull) []byte {
	t.Helper()
	dex, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	return dex
}

// comparaDexByteAByte: el mismo dex byte a byte, con el byte que difiere y el volcado de nuestras
// clases para poder mirar sin volver a compilar.
func comparaDexByteAByte(t *testing.T, nuestro, deJavac []byte, volcado string) {
	t.Helper()
	if len(nuestro) != len(deJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d\n%s", len(nuestro), len(deJavac), volcado)
	}
	for i := range nuestro {
		if nuestro[i] != deJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s", i, nuestro[i], deJavac[i], volcado)
		}
	}
}

// metodoDeClaseEmitida y metodoDeClaseLeida: el método con ese nombre (y ese descriptor, cuando se
// pide) en lo que emitimos nosotros o en lo que emitió javac.
func metodoDeClaseEmitida(t *testing.T, clases []claseEmitida, clase, nombre, desc string) classMethod {
	t.Helper()
	for _, c := range clases {
		if c.interno != clase {
			continue
		}
		leida, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase %s no se lee: %v", clase, err)
		}
		for _, m := range leida.Methods {
			if m.Name == nombre && (desc == "" || m.Desc == desc) {
				return m
			}
		}
	}
	t.Fatalf("no hay %s.%s%s en lo emitido", clase, nombre, desc)
	return classMethod{}
}

func metodoDeClaseLeida(t *testing.T, clases []classFull, clase, nombre, desc string) classMethod {
	t.Helper()
	for _, c := range clases {
		if c.Name != clase {
			continue
		}
		for _, m := range c.Methods {
			if m.Name == nombre && (desc == "" || m.Desc == desc) {
				return m
			}
		}
	}
	t.Fatalf("no hay %s.%s%s en lo de javac", clase, nombre, desc)
	return classMethod{}
}

// TestAppAdapterDeApkShareLlegaAlEmisor: el candado del contrato del `check` sobre los mismos cuatro
// fuentes, con el proyecto en disco para que el `R` lo encuentre donde `apkc build` lo deja. Sin la
// subida de referencia el emisor se paraba en la línea 33 y sin el booleano suelto en la 64; ahora el
// informe del fuente pedido sale con `emisor: true` y sin errores.
func TestAppAdapterDeApkShareLlegaAlEmisor(t *testing.T) {
	raiz := t.TempDir()
	for _, f := range fuentesAppAdapterShare() {
		pon(t, raiz, f.nombre, f.contenido)
	}
	informe := revisarFuenteJavaEn(fuenteAppAdapterShare, "src/share/apps/util/AppAdapter.java",
		"AppAdapter.java", filepath.Join(raiz, "src/share/apps/util/AppAdapter.java"), jarDelSDK(t), raiz, 0, "")
	if len(informe.Errores) != 0 {
		t.Fatalf("el check sigue dando errores: %v", informe.Errores)
	}
	if !informe.Emisor {
		t.Fatalf("el check no llegó al emisor (aviso: %q)", informe.Aviso)
	}
}

// TestFalsificacionDeLaSubidaDeReferencia: con el interruptor apagado, los fuentes reales de ApkShare
// vuelven a devolver el error literal del usuario en la línea 33. Es la ruta nueva y no otra cosa.
func TestFalsificacionDeLaSubidaDeReferencia(t *testing.T) {
	anterior := cabeSubidaDeReferencia
	cabeSubidaDeReferencia = false
	t.Cleanup(func() { cabeSubidaDeReferencia = anterior })

	_, err := compilarProyectoJava(fuentesAppAdapterShare(), jarDelSDK(t))
	if err == nil {
		t.Fatal("el emisor aceptó `apps_filter = new ArrayList<>(apps)` con la ruta apagada")
	}
	t.Logf("error literal del usuario: %v", err)
	const esperado = "linea 33: apps_filter es Ljava/util/List; y se le asigna un Ljava/util/ArrayList;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el rechazo no es el literal del usuario:\nesperado %q\nobtenido %v", esperado, err)
	}
}

// TestFalsificacionDeLaSubidaPorElAccesor: el mismo interruptor, sobre la ruta del `access$NN2`.
func TestFalsificacionDeLaSubidaPorElAccesor(t *testing.T) {
	anterior := cabeSubidaDeReferencia
	cabeSubidaDeReferencia = false
	t.Cleanup(func() { cabeSubidaDeReferencia = anterior })

	_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuenteSondaAccesor}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("el emisor aceptó la subida por el accesor con la ruta apagada")
	}
	t.Logf("error literal del accesor: %v", err)
	const esperado = "oculta es Ljava/util/List; y se le asigna un Ljava/util/ArrayList;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el rechazo no es el esperado:\nesperado %q\nobtenido %v", esperado, err)
	}
}

// TestFalsificacionDelBooleanoSuelto: apagado el interruptor de la condición, los fuentes reales de
// ApkShare vuelven a parar en la línea 64 (`if (app.toString().toLowerCase().contains(...))`) con el
// mensaje de antes. La subida de referencia se queda encendida para que el único motivo sea este.
func TestFalsificacionDelBooleanoSuelto(t *testing.T) {
	anterior := condicionBooleanaSuelta
	condicionBooleanaSuelta = false
	t.Cleanup(func() { condicionBooleanaSuelta = anterior })

	_, err := compilarProyectoJava(fuentesAppAdapterShare(), jarDelSDK(t))
	if err == nil {
		t.Fatal("el emisor aceptó el booleano suelto con la ruta apagada")
	}
	t.Logf("error literal de antes: %v", err)
	const esperado = "linea 64: una condicion que no sea una comparacion no esta medida (sobre un booleano javac usa `ifeq`, y eso no lo leimos)"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el rechazo no es el literal de antes:\nesperado %q\nobtenido %v", esperado, err)
	}
}

// TestLaNegacionDeUnBooleanoSueltoNoMaterializa: `if (!llama())` sale con `ifne` —la carga o la
// llamada y el salto—, sin `iconst`/`ifeq` por medio; y `if (!(a == b))` sigue plegándose en la
// comparación de la tabla, no por la ruta nueva.
func TestLaNegacionDeUnBooleanoSueltoNoMaterializa(t *testing.T) {
	fuente := `package p;

class A {
    boolean listo() {
        return true;
    }

    int f(int a, int b) {
        if (!listo()) {
            return 1;
        }
        if (!(a == b)) {
            return 2;
        }
        return 0;
    }
}
`
	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó la negación: %v", err)
	}
	m := metodoDeClaseEmitida(t, nuestras, "p/A", "f", "(II)I")
	ins, err := walkJvmCode(m.Code)
	if err != nil {
		t.Fatalf("el Code no se desarma: %v", err)
	}
	// 0x03/0x04 son `iconst_0`/`iconst_1`: materializar el booleano es exactamente lo que javac no
	// hace para branquear una condición. Los `iconst` del cuerpo (`return 1`, `return 2`, el `0`) sí
	// están, así que se cuenta solo lo que aparece ANTES del primer salto condicional.
	vistos := 0
	for _, i := range ins {
		if i.op == 0x99 || i.op == 0x9a || i.op == 0x9f || i.op == 0xa0 { // ifeq, ifne, if_icmpeq, if_icmpne
			vistos++
			continue
		}
		if vistos == 0 && (i.op == 0x03 || i.op == 0x04) {
			t.Fatalf("la condición se materializó con un `iconst` antes del primer salto:\n%v", ins)
		}
	}
	if vistos != 2 {
		t.Fatalf("se esperaban dos saltos condicionales (el `ifne` de la negación y el `if_icmpne` de la comparación) y hay %d:\n%v", vistos, ins)
	}
}
