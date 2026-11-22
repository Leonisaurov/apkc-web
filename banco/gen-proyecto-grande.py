#!/usr/bin/env python3
# Genera un proyecto apkc sintetico y representativo de una app real para medir el build:
# 200 clases con campos, bucles, switch, try/catch y arreglos; 30 actividades que leen la R;
# values (strings, colores, fraction, arrays, plurals), values-es, layout, layout-land,
# drawable con PNG validos, menu, xml y raw. Todo dentro del subconjunto Java que mide el
# emisor del fork (ni ldc_w, ni this.campo, ni .length de un arreglo: su error se ve en el
# acto y no compila a medias).
#
#   uso: python3 banco/gen-proyecto-grande.py <directorio-nuevo> [numero-de-clases]
#
# Lo usa banco/incremental.sh; el numero de ficheros no es una promesa de nada, es el tamano
# del caso medido. No escribe en el repo ni versiona binarios: el PNG se inventa en memoria.
import os, sys, zlib, struct

raiz = sys.argv[1]
nmods = int(sys.argv[2]) if len(sys.argv) > 2 else 200
if os.path.isdir(raiz) and os.listdir(raiz):
    sys.exit("el directorio %s no esta vacio: este generador no borra nada" % raiz)
if os.path.exists(raiz) and not os.path.isdir(raiz):
    sys.exit("%s existe y no es un directorio" % raiz)

pkg = "com.example.big"
srcdir = os.path.join(raiz, "src", *pkg.split("."))
for d in [srcdir, os.path.join(raiz,"res","values"), os.path.join(raiz,"res","values-es"),
          os.path.join(raiz,"res","layout"), os.path.join(raiz,"res","layout-land"),
          os.path.join(raiz,"res","drawable"), os.path.join(raiz,"res","menu"),
          os.path.join(raiz,"res","xml"), os.path.join(raiz,"res","raw")]:
    os.makedirs(d, exist_ok=True)

def w(ruta, texto):
    with open(ruta, "w", encoding="utf-8") as f:
        f.write(texto)

# --- PNG mínimo válido -------------------------------------------------------------
def png(ancho, alto, cr, cg, cb):
    def trozo(tipo, datos):
        c = struct.pack(">I", len(datos)) + tipo + datos
        return c + struct.pack(">I", zlib.crc32(tipo + datos) & 0xFFFFFFFF)
    filas = b""
    for _ in range(alto):
        fila = b"\x00"
        for _ in range(ancho):
            fila += bytes([cr, cg, cb, 255])
        filas += fila
    cabecera = struct.pack(">IIBBBBB", ancho, alto, 8, 6, 0, 0, 0)
    return (b"\x89PNG\r\n\x1a\n" + trozo(b"IHDR", cabecera) +
            trozo(b"IDAT", zlib.compress(filas, 6)) + trozo(b"IEND", b""))

# --- Java -----------------------------------------------------------------------------
manifiesto = f'''<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="{pkg}"
    android:versionCode="1"
    android:versionName="1.0">

    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <uses-permission android:name="android.permission.INTERNET" />

    <application android:label="@string/app_name" android:icon="@drawable/icono0">
        <activity android:name=".MainActivity" android:label="@string/app_name" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
'''
w(os.path.join(raiz, "AndroidManifest.xml"), manifiesto)

modulo = '''package {pkg};

public class Mod{i:03d} {{
    private int fBase;
    private long fAcum;
    private String fNombre;
    private double fPeso;
    private int[] fTabla;

    public Mod{i:03d}(int base, String nombre) {{
        fBase = base;
        fNombre = nombre;
        fAcum = base * 3L;
        fPeso = base / 2.0;
        fTabla = new int[12];
        for (int k = 0; k < 12; k++) {{
            fTabla[k] = k * base + {i};
        }}
    }}

    public int sumar(int limite) {{
        int total = 0;
        for (int k = 0; k < limite; k++) {{
            if (k % 3 == 0) {{
                total += k * 2;
            }} else {{
                total += k;
            }}
        }}
        return total + fBase;
    }}

    public int elegir(int clave) {{
        switch (clave) {{
            case 0:
                return fBase;
            case 1:
                return fBase + {i};
            case 2:
                return fTabla[2];
            default:
                return -1;
        }}
    }}

    public long acumular(int vueltas) {{
        long t = fAcum;
        int k = 0;
        while (k < vueltas) {{
            long paso = (long) (k * fBase);
            t = t + paso;
            k++;
        }}
        return t;
    }}

    public int[] tabla() {{
        int[] copia = new int[12];
        for (int k = 0; k < 12; k++) {{
            copia[k] = fTabla[k] + {i};
        }}
        return copia;
    }}

    public String describir() {{
        return fNombre + ":{i}:" + fBase;
    }}

    public int protegido(int x) {{
        try {{
            if (x < 0) {{
                throw new IllegalArgumentException("negativo");
            }}
            int a = sumar(x);
            int resto = x % 3;
            int b = elegir(resto);
            return a + b;
        }} catch (IllegalArgumentException e) {{
            return -1;
        }} finally {{
            fPeso = fPeso + 0.5;
        }}
    }}

    public boolean comparar(Object otro) {{
        if (otro instanceof Mod{i:03d}) {{
            return true;
        }}
        return false;
    }}

    public int largo() {{
        return fNombre.length() + 12;
    }}
}}
'''
for i in range(nmods):
    w(os.path.join(srcdir, f"Mod{i:03d}.java"), modulo.format(pkg=pkg, i=i))

# actividades de pantalla: usan R y los módulos
pantalla = '''package {pkg};

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;

public class Pantalla{n:02d} extends Activity {{
    @Override
    public void onCreate(Bundle savedInstanceState) {{
        super.onCreate(savedInstanceState);
        setContentView(R.layout.pantalla{n:02d});
        TextView vista = (TextView) findViewById(R.id.titulo);
        Mod{a:03d} m = new Mod{a:03d}({n}, getString(R.string.titulo{n:02d}));
        vista.setText(m.describir());
    }}
}}
'''
for n in range(30):
    w(os.path.join(srcdir, f"Pantalla{n:02d}.java"), pantalla.format(pkg=pkg, n=n, a=(n*7) % nmods))

principal = '''package {pkg};

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;

public class MainActivity extends Activity {{
    @Override
    public void onCreate(Bundle savedInstanceState) {{
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        TextView vista = (TextView) findViewById(R.id.titulo);
        Mod000 m = new Mod000(7, getString(R.string.app_name));
        int total = m.sumar(10);
        vista.setText(getString(R.string.resumen) + ":" + total);
    }}
}}
'''.replace("{pkg}", pkg).replace("{{", "{").replace("}}", "}")
w(os.path.join(srcdir, "MainActivity.java"), principal)

# --- resources -----------------------------------------------------------------------
lineas = ['<?xml version="1.0" encoding="utf-8"?>', "<resources>"]
lineas.append('  <string name="app_name">Proyecto Grande</string>')
lineas.append('  <string name="resumen">Resumen</string>')
for n in range(30):
    lineas.append(f'  <string name="titulo{n:02d}">Titulo numero {n}</string>')
for n in range(40):
    lineas.append(f'  <string name="texto{n:02d}">Texto de relleno numero {n} con acentos: acción, niño, cañón</string>')
lineas.append('  <string-array name="dias">')
for d in ["lunes","martes","miercoles","jueves","viernes","sabado","domingo"]:
    lineas.append(f'    <item>{d}</item>')
lineas.append('  </string-array>')
lineas.append('  <integer-array name="numeros">')
for n in range(10):
    lineas.append(f'    <item>{n}</item>')
lineas.append('  </integer-array>')
lineas.append('  <plurals name="filas">')
lineas.append('    <item quantity="one">%d fila</item>')
lineas.append('    <item quantity="other">%d filas</item>')
lineas.append('  </plurals>')
lineas.append('</resources>')
w(os.path.join(raiz,"res","values","strings.xml"), "\n".join(lineas)+"\n")

colores = ['<?xml version="1.0" encoding="utf-8"?>', "<resources>"]
for n in range(24):
    colores.append(f'  <color name="color{n:02d}">#{n:02x}{(n*3)%256:02x}{(n*7)%256:02x}</color>')
colores.append('</resources>')
w(os.path.join(raiz,"res","values","colors.xml"), "\n".join(colores)+"\n")

dim = ['<?xml version="1.0" encoding="utf-8"?>', "<resources>"]
for n in range(24):
    dim.append('  <fraction name="f%d">%d%%</fraction>' % (n, n*2))
dim.append('  <fraction name="mitad">50%</fraction>')
dim.append('</resources>')
w(os.path.join(raiz,"res","values","dimens.xml"), "\n".join(dim)+"\n")

estilos = ['<?xml version="1.0" encoding="utf-8"?>', "<resources>"]
estilos.append('  <style name="AppTheme" parent="@android:style/Theme.DeviceDefault.Light">')
estilos.append('    <item name="android:windowActionBar">false</item>')
estilos.append('    <item name="android:windowNoTitle">true</item>')
estilos.append('  </style>')
for n in range(8):
    estilos.append(f'  <style name="Estilo{n:02d}" parent="@android:style/TextAppearance">')
    estilos.append(f'    <item name="android:textColor">#{(n*17)%256:02x}{(n*29)%256:02x}{(n*41)%256:02x}</item>')
    estilos.append('  </style>')
estilos.append('</resources>')
w(os.path.join(raiz,"res","values","styles.xml"), "\n".join(estilos)+"\n")

es = ['<?xml version="1.0" encoding="utf-8"?>', "<resources>"]
for n in range(20):
    es.append(f'  <string name="titulo{n:02d}">Título número {n}</string>')
es.append('</resources>')
w(os.path.join(raiz,"res","values-es","strings.xml"), "\n".join(es)+"\n")

def layout(nombre, textos, idioma):
    return f'''<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:orientation="vertical"
    android:layout_width="match_parent"
    android:layout_height="match_parent">

    <TextView
        android:id="@+id/titulo"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:text="@string/{textos}"/>

    <ImageView
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:src="@drawable/icono{n % 20}"/>

    <TextView
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:text="@string/app_name"/>
</LinearLayout>
'''

w(os.path.join(raiz,"res","layout","activity_main.xml"), layout("main","app_name",0))
for n in range(30):
    w(os.path.join(raiz,"res","layout",f"pantalla{n:02d}.xml"), layout(f"pantalla{n:02d}", f"titulo{n:02d}", n))
for n in range(10):
    w(os.path.join(raiz,"res","layout-land",f"pantalla{n:02d}.xml"), layout(f"pantalla{n:02d}", f"texto{n:02d}", n))

for n in range(20):
    with open(os.path.join(raiz,"res","drawable",f"icono{n}.png"), "wb") as f:
        f.write(png(16+n, 16+n, (n*11)%256, (n*23)%256, (n*37)%256))

for n in range(6):
    menu = f'''<?xml version="1.0" encoding="utf-8"?>
<menu xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:id="@+id/opcion{n}" android:title="@string/texto{n:02d}" />
</menu>
'''
    w(os.path.join(raiz,"res","menu",f"menu{n}.xml"), menu)

for n in range(4):
    w(os.path.join(raiz,"res","xml",f"prefs{n}.xml"),
      '<?xml version="1.0" encoding="utf-8"?>\n<PreferenceScreen xmlns:android="http://schemas.android.com/apk/res/android" />\n')

for n in range(3):
    w(os.path.join(raiz,"res","raw",f"datos{n}.txt"), "linea de datos %d\n" % n)

print("java:", len([f for f in os.listdir(srcdir) if f.endswith('.java')]))
print("res:", sum(len(fs) for _,_,fs in os.walk(os.path.join(raiz,"res"))))
