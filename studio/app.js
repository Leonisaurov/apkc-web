// apkc studio — genera el proyecto, manda la salida de apkc a texto legible y guarda
// dónde se atasca quien lo usa. Sin dependencias: lo que hace falta para escribir el zip
// es un CRC32 y una cabecera local por fichero, nada más.

// ---------------------------------------------------------------- plantillas reales
// Copiadas de apkc/template/* del repo: si apkc cambia una plantilla, está página miente,
// y esa es la única forma en que puede quedar desincronizada con la herramienta.
const TEMPLATES = {
  manifest: `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="{{pkgname}}"
    android:versionCode="1"
    android:versionName="0.1">

    <uses-sdk android:minSdkVersion="{{minsdk}}"
          android:targetSdkVersion="{{targetsdk}}"
          android:maxSdkVersion="{{targetsdk}}" />

    <application android:label="{{appname}}"{{extras}}>

{{activity}}    </application>
</manifest>
`,
  activity: `package {{pkgname}};

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
`,
  layoutButton: `    <Button
        android:id="@+id/pulsar"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:text="{{button}}"/>

`,
  styles: `<resources>
    <style name="AppTheme" parent="@android:style/Theme.DeviceDefault.Light">
        <item name="android:windowActionBar">false</item>
        <item name="android:windowNoTitle">true</item>
        <item name="android:statusBarColor">#FFFFFF</item>
        <item name="android:windowLightStatusBar">true</item>
        <item name="android:windowBackground">@android:color/white</item>
    </style>
</resources>`,
  layout: `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:orientation="vertical"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:gravity="center">

    <TextView
        style="@android:style/TextAppearance.Large"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:text="{{hello}}"/>

{{button}}</LinearLayout>
`,
  activityButton: `
    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add("{{hello}}").setOnMenuItemClickListener(new MenuItem.OnMenuItemClickListener() {
            public boolean onMenuItemClick(MenuItem item) {
                Toast.makeText(MainActivity.this, "{{toast}}", Toast.LENGTH_SHORT).show();
                return true;
            }
        });
        return true;
    }
`,
  kotlinActivity: `package {{pkgname}}

import android.app.Activity
import android.os.Bundle

class MainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.main)
    }
}
`,
  gitignore: "build/\n",
};

// ---------------------------------------------------------------- el proyecto
function buildProject(state) {
  const pkg = state.package;
  const dirs = ["src", "lib/arm64-v8a", "lib/armeabi-v7a", "lib/x86", "lib/x86_64",
                "res/layout", "res/values"];
  dirs.push(...pkg.split(".").map((_, i, all) => "src/" + all.slice(0, i + 1).join("/")));
  if (state.kotlin) dirs.push("src/" + pkg.split(".").join("/"));
  if (state.libs) dirs.push("jar");
  if (state.assets) dirs.push("assets");

  // el bloque del botón va antes del último } del fichero, que es el de la clase: anclado al
  // primero se colaba dentro de onCreate y javac paraba el proyecto entero
  const texto = (v, def) => (v === undefined || v === null || String(v).trim() === "" ? def : String(v));
  const boton = { label: texto(state.buttonLabel, "Press me"), toast: texto(state.toast, "Clicked!") };
  const extra = state.button
    ? TEMPLATES.activityButton
        .replace(/\{\{hello\}\}/g, texto(state.hello, "Hello World!"))
        .replace(/\{\{toast\}\}/g, boton.toast)
    : "";
  const activityBody = TEMPLATES.activity.replace(/\{\{pkgname\}\}/g, pkg)
    .replace(/\}\n$/, extra + "}\n");

  const files = {
    "AndroidManifest.xml": TEMPLATES.manifest
      .replace(/\{\{pkgname\}\}/g, pkg)
      .replace(/\{\{appname\}\}/g, state.name)
      .replace(/\{\{minsdk\}\}/g, state.minsdk)
      .replace(/\{\{targetsdk\}\}/g, state.targetsdk)
      .replace(/\{\{extras\}\}/g, state.native ? '\n        android:extractNativeLibs="false"' : "")
      .replace(/\{\{activity\}\}/g, `        <activity
            android:theme="@style/AppTheme"
            android:name=".MainActivity"
            android:label="${state.name}"
            android:exported="true" >

            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER"/>
            </intent-filter>

        </activity>
`),
    "res/layout/main.xml": TEMPLATES.layout
      .replace(/\{\{hello\}\}/g, texto(state.hello, "Hello World!"))
      .replace(/\{\{button\}\}/g, state.button
        ? TEMPLATES.layoutButton.replace(/\{\{button\}\}/g, boton.label) : ""),
    "res/values/styles.xml": TEMPLATES.styles,
    ".gitignore": TEMPLATES.gitignore,
  };
  if (state.kotlin) {
    files["src/" + pkg.split(".").join("/") + "/MainActivity.kt"] =
      TEMPLATES.kotlinActivity.replace(/\{\{pkgname\}\}/g, pkg);
  } else {
    files["src/" + pkg.split(".").join("/") + "/MainActivity.java"] = activityBody;
  }
  // un LEEME dentro de jar/, assets/ o lib/<abi>/ no es inocente: apkc mete lo que hay en
  // assets/ y en lib/ en el apk, y un .txt en lib/arm64-v8a/ viaja como si fuera una .so. La
  // explicación va en la raíz, que es el único sitio que apkc ignora.
  const notas = [];
  if (state.libs) notas.push("jar/       los .jar de dependencias; apkc los dexa y los mete en el apk sin\n"
    + "           que nadie los declare. un .aar no lo lee: saca antes su classes.jar y ponlo aquí.");
  if (state.assets) notas.push("assets/    lo que haya dentro llega al apk tal cual; borrarlo o renombrarlo\n"
    + "           también lo quita del paquete.");
  if (state.native) notas.push("lib/<abi>/  con extractNativeLibs=\"false\" en el manifest, el .so viaja sin comprimir\n"
    + "           y alineado a página; con \"true\" (o sin atributo) el sistema lo descomprime al instalar.");
  if (notas.length) files["LEEME.txt"] = notas.join("\n\n") + "\n";
  return { dirs, files };
}

// ---------------------------------------------------------------- zip (STORE, sin dependencias)
const CRC_TABLE = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  return t;
})();

function crc32(buf) {
  let c = 0xffffffff;
  for (let i = 0; i < buf.length; i++) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

// Solo entradas guardadas (STORE): lo que se guarda es texto pequeño, y así el lector del
// zip no necesita nada para abrirlo. El campo de descriptor de datos se deja sin poner a
// propósito, que es justo el error que zipcheck.py caza en un apk.
function makeZip(entries) {
  const enc = new TextEncoder();
  const parts = [];
  const central = [];
  let offset = 0;

  for (const [name, text] of entries) {
    const nameBytes = enc.encode(name);
    const data = enc.encode(text);
    const crc = crc32(data);

    const local = new DataView(new ArrayBuffer(30));
    local.setUint32(0, 0x04034b50, true);
    local.setUint16(4, 20, true);          // versión needed
    local.setUint16(6, 0x0800, true);      // flags: nombres en UTF-8, sin descriptor
    local.setUint16(8, 0, true);           // metodo: stored
    local.setUint16(10, 0, true);          // hora
    local.setUint16(12, 0x21, true);       // fecha: 1980-01-01, dosTime minimo legal
    local.setUint32(14, crc, true);
    local.setUint32(18, data.length, true);
    local.setUint32(22, data.length, true);
    local.setUint16(26, nameBytes.length, true);
    local.setUint16(28, 0, true);
    parts.push(new Uint8Array(local.buffer), nameBytes, data);

    const cd = new DataView(new ArrayBuffer(46));
    cd.setUint32(0, 0x02014b50, true);
    cd.setUint16(4, 0x031e, true);         // hecho por UNIX, zip 3.0
    cd.setUint16(6, 20, true);
    cd.setUint16(8, 0x0800, true);
    cd.setUint16(10, 0, true);
    cd.setUint16(12, 0, true);
    cd.setUint16(14, 0x21, true);
    cd.setUint32(16, crc, true);
    cd.setUint32(20, data.length, true);
    cd.setUint32(24, data.length, true);
    cd.setUint16(28, nameBytes.length, true);
    // atributos externos: el modo UNIX va en los 16 bits altos, y sin ellos quien
    // descomprime crea los directorios en 000 y el proyecto queda ilegible
    const esDir = name.endsWith("/");
    cd.setUint32(38, (esDir ? 0o40755 : 0o100644) * 0x10000 + (esDir ? 0x10 : 0), true);
    cd.setUint32(42, offset, true);
    central.push(new Uint8Array(cd.buffer), nameBytes);

    offset += 30 + nameBytes.length + data.length;
  }

  const cdStart = offset;
  for (const c of central) { parts.push(c); offset += c.length; }

  const end = new DataView(new ArrayBuffer(22));
  end.setUint32(0, 0x06054b50, true);
  // el número de entradas, no el de trozos escritos: cada entrada son dos partes (record +
  // nombre) y un lector estricto busca después de la última otra firma y se encuentra el EOCD
  end.setUint16(8, entries.length, true);
  end.setUint16(10, entries.length, true);
  end.setUint32(12, offset - cdStart, true);
  end.setUint32(16, cdStart, true);
  parts.push(new Uint8Array(end.buffer));

  const total = parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(total);
  let at = 0;
  for (const p of parts) { out.set(p, at); at += p.length; }
  return out;
}

// ---------------------------------------------------------------- analizador de salida
// Cada regla lleva lit:, la cadena textual que la motivó, y de:, de quien sale esa cadena.
// prueba.js exige que las de `de: "apkc"` existan literalmente en el repo: una regla inventada
// no falla ruidosamente, es peor que no tenerla, porque la página explica un error que nadie
// imprime. Las de otra herramienta cuentan con la salida capturada en muestras.json, y si no
// la hay, la prueba lo dice en vez de callarlo. El id va en ascii porque acaba en los
// contadores del informe.
const RULES = [
  { id: "proyecto-incompleto", re: /stat (src|res|jar|assets|lib)(\/[^:]*)?: no such file or directory|there is no `[^`]+`(, `[^`]+`)* and `[^`]+` here|there is no `[^`]+` here, so this is not an apkc project/,
    de: "apkc", lit: "so this is not an apkc project",
    capa: "proyecto", que: "no hay un proyecto apkc en el directorio donde se ejecutó el comando",
    hacer: "entra en la carpeta del proyecto, o arranca uno con `apkc create .` (o `apkc create <carpeta>`). apkc no busca hacia arriba: trabaja donde está el cwd. Las versiones viejas decían solo `stat src: no such file or directory`. " },
  { id: "sdk-java-home", re: /(ANDROID_HOME|JAVA_HOME) was not found|SDK path unknown/,
    de: "apkc", lit: "SDK path unknown",
    capa: "entorno", que: "apkc no encuentra el SDK de Android o el JDK",
    hacer: "exporta ANDROID_HOME (o JAVA_HOME) en el shell que ejecuta el comando; en Termux el SDK suele estar en ~/Android y el JDK en $PREFIX/lib/jvm. Comprueba con `apkc doctor`, que imprime la ruta que resolvió." },
  { id: "build-tools-parse", re: /error parsing build tools version/,
    de: "apkc", lit: "error parsing build tools version",
    capa: "entorno", que: "una carpeta bajo build-tools no se parece a una versión",
    hacer: "esa carpeta no es del SDK: muévela fuera de build-tools/." },
  { id: "keystore-missing", re: /no debug keystore at/,
    de: "apkc", lit: "no debug keystore at",
    capa: "entorno", que: "no existe el keystore de depuración, y sin él no hay apk firmado",
    hacer: "deja que el SDK lo cree instalando cualquier cosa, o genéralo: keytool -genkeypair -keystore ~/.android/debug.keystore -storepass android -keypass android -alias androiddebugkey -keyalg RSA -dname 'CN=Android Debug,O=Android,C=US'." },
  { id: "herramienta-no-arranca", re: /never started: /,
    de: "apkc", lit: "never started",
    capa: "entorno", que: "la herramienta del SDK no llegó a ejecutarse (no es que fallara ejecutándose)",
    hacer: "es un problema de arranque, no de tu código: el archivo no existe, no tiene permiso de ejecución, o el binario es de otra arquitectura. `apkc doctor` imprime la ruta que resolvió para cada herramienta. En un build js/wasm el motivo es que el navegador no puede hablar con un proceso: ahí ni aapt2 ni javac ni d8 existen." },
  { id: "keystore-file-missing", re: /there is no keystore at|Failed to load signer|java\.io\.FileNotFoundException: .*keystore/,
    de: "apkc", lit: "there is no keystore at",
    capa: "entorno", que: "la ruta pasada con --keystore no apunta a ningún archivo",
    hacer: "compruébala con ls: `apkc doctor` imprime la ruta que está usando y --keystore toma otra. Antes de este cambio la firma en Go la rechazaba con su propio mensaje, pero --signer=jvm dejaba llegar la pila de Java de apksigner; ahora apkc lo corta antes de elegir backend." },
  { id: "aar-not-read", re: /not a PKCS#12 file|not a JKS keystore|is neither a PKCS#12 nor a JKS/,
    de: "apkc", lit: "is neither a PKCS#12 nor a JKS keystore",
    capa: "entorno", que: "el almacén no es un formato que apkc pueda abrir",
    hacer: "conviértelo: keytool -importkeystore -srckeystore viejo -destkeystore nuevo -deststoretype PKCS12. Si no puedes, construye con --signer=jvm." },
  { id: "keystore-password", re: /did not open with the given storepass|password does not open this JKS|tampered with, or the password is wrong/,
    de: "apkc", lit: "did not open with the given storepass",
    capa: "entorno", que: "la contraseña del keystore no abre el almacén",
    hacer: "pasa --storepass con la correcta; en el de depuración es android." },
  { id: "keystore-alias", re: /holds \d+ key\(s\)|and none is /,
    de: "apkc", lit: "and none is",
    capa: "entorno", que: "el alias pedido no está en el keystore",
    hacer: "mira los nombres: keytool -list -keystore <ruta>. Con uno solo y sin nombre, apkc lo usa sin preguntar." },
  { id: "kotlinc", re: /kotlinc is not on PATH/,
    de: "apkc", lit: "kotlinc is not on PATH",
    capa: "entorno", que: "hay fuentes .kt pero no está el compilador de Kotlin",
    hacer: "instala kotlinc o quita los .kt; apkc no baja herramientas por su cuenta." },
  { id: "stdlib", re: /no kotlin-stdlib\*\.jar in jar/,
    de: "apkc", lit: "no kotlin-stdlib",
    capa: "entorno", que: "falta la librería estándar de Kotlin en jar/",
    hacer: "copia kotlin-stdlib-<versión>.jar a jar/: se compila sin ella y luego falta en el arranque." },
  { id: "duplicate-class", re: /is defined multiple times/,
    de: "apkc", lit: "is defined multiple times",
    capa: "proyecto", que: "la misma clase la aportan el proyecto y una dependencia",
    hacer: "quita el jar que la repite o cambia el paquete propio. Es un fallo duro a propósito: dos copias de una clase se resuelven en tiempo de ejecución a elegir una." },
  { id: "d8-drop", re: /was not found, it is required for|desugaring/,
    de: "d8", lit: "it is required for",
    capa: "proyecto", que: "d8 descarta una clase por un tipo que no ve",
    hacer: "normalmente falta el jar que aporta ese tipo; si es una dependencia entre cubos, comprueba que el jar está en jar/." },
  { id: "d8-bucket", re: /the d8 of dependency bucket \d+ failed/,
    de: "apkc", lit: "the d8 of dependency bucket",
    capa: "herramienta", que: "un cubo de dependencias no dejó de dexar",
    hacer: "el log lleva la salida de d8 debajo. Si es memoria, vuelve con APKC_JOBS=1; si es un jar concreto, quítalo y prueba." },
  { id: "bundletool", re: /no bundletool found/,
    de: "apkc", lit: "no bundletool found",
    capa: "entorno", que: "se pidió un .aab y bundletool no está por ningún sitio que apkc mira",
    hacer: "bájate el jar a ~/Android/bundletool o pon bundletool en el PATH." },
  { id: "jarsigner", re: /no jarsigner at/,
    de: "apkc", lit: "no jarsigner at",
    capa: "entorno", que: "sin jarsigner no se puede firmar un bundle",
    hacer: "el jarsigner viene con el JDK; si apkc resolvió otro, revisa JAVA_HOME." },
  { id: "install-signature", re: /INSTALL_FAILED_UPDATE_INCOMPATIBLE/,
    de: "pm", lit: "INSTALL_FAILED_UPDATE_INCOMPATIBLE",
    capa: "dispositivo", que: "la app instalada está firmada con otra clave",
    hacer: "desinstala la versión instalada, o vuelve a la clave con la que se instaló por primera vez." },
  { id: "install-other", re: /INSTALL_FAILED_[A-Z_]+/,
    de: "pm", lit: "INSTALL_FAILED_",
    capa: "dispositivo", que: "el gestor de paquetes rechazó la instalación",
    hacer: "el código dice el motivo; el log de apkc imprime la línea que devolvió pm." },
  { id: "pid", re: /couldn't get the pid|not running/,
    de: "apkc", lit: "couldn't get the pid",
    capa: "dispositivo", que: "la app murió antes de que existiera un proceso del que leer el log",
    hacer: "apkc vuelca el buffer de fallos del teléfono antes de decir nada: mira la excepción FATAL que sale arriba." },
  { id: "native-libs", re: /no android:extractNativeLibs in the manifest/,
    de: "apkc", lit: "no android:extractNativeLibs in the manifest",
    capa: "informativo", que: "aviso: los .so viajan comprimidos porque el manifest no dice nada",
    hacer: "no es un error. Si quieres que el sistema los mapee desde el apk, pon extractNativeLibs=\"false\" y apkc los guarda sin comprimir y alineados a página." },
  { id: "type-changed", re: /The project type changed/,
    de: "apkc", lit: "The project type changed",
    capa: "informativo", que: "cambió el tipo (apk/aab) respecto al último build",
    hacer: "esperado: una ronda completa." },
  { id: "constants-moved", re: /recompiling every source: a constant value moved/,
    de: "apkc", lit: "recompiling every source: a constant value moved",
    capa: "informativo", que: "un valor de R.* o de una constante obligó a recompilar todo",
    hacer: "esperado: javac copia los valores constantes dentro de los .class, así que no se puede invalidar solo a quien lo leía." },
  { id: "state-version", re: /state version \d+ is not \d+/,
    de: "apkc", lit: "state version",
    capa: "informativo", que: "el estado incremental lo escribió otra versión de apkc",
    hacer: "normal tras actualizar: una ronda produce todo otra vez." },
  { id: "not-a-class", re: /not a class file|cannot be read, writing every entry again/,
    de: "apkc", lit: "cannot be read, writing every entry again",
    capa: "herramienta", que: "algo que apkc esperaba leer no es lo que decía ser",
    hacer: "si aparece en el medio de un build correcto, es un fichero corrupto en build/: apkc clean y otra ronda." },
];

function analyze(text) {
  const hits = [];
  for (const rule of RULES) {
    const m = text.match(rule.re);
    if (m) hits.push({ ...rule, linea: m[0] });
  }
  // una línea que no es reconocida también es dato: sin ella la página solo sabría de lo
  // que ya conoce
  const desconocidas = text.split("\n")
    .map((l) => l.replace(/\x1b\[[0-9;]*m/g, "").trim())
    .filter((l) => l && /\[(build|sign|d8|doctor|create|clean|run|watch|watcher|incremental|timing|javac|kotlinc|apkc)\]/i.test(l))
    .filter((l) => !RULES.some((r) => r.re.test(l)))
    .filter((l) => !/^\[timing\]|^\[watch\] round|finished in|Detected a file changed|^\[(build|sign)\] (compiling|bundling|packaging|running|signing|dexing)/i.test(l));
  return { hits, desconocidas: [...new Set(desconocidas)].slice(0, 12) };
}

// ---------------------------------------------------------------- medida de friccion
const STORE_KEY = "apkc-studio/v1";

function loadLog() {
  try { return JSON.parse(localStorage.getItem(STORE_KEY)) || { pasos: [], eventos: [] }; }
  catch (e) { return { pasos: [], eventos: [] }; }
}
// localStorage puede estar lleno o prohibido (modo privado): perder los contadores no puede
// impedir que el diagnóstico pinte lo que ya analizó en memoria
function saveLog(l) {
  try { localStorage.setItem(STORE_KEY, JSON.stringify(l)); }
  catch (e) { /* sin persistencia, la página sigue */ }
}
function registrar(tipo, detalle) {
  const l = loadLog();
  l.eventos.push({ t: new Date().toISOString(), tipo, detalle: detalle || null });
  saveLog(l);
  return l;
}

function informe() {
  const l = loadLog();
  const cuenta = {};
  for (const e of l.eventos) cuenta[e.tipo + (e.detalle ? ": " + e.detalle : "")] =
    (cuenta[e.tipo + (e.detalle ? ": " + e.detalle : "")] || 0) + 1;
  const porCapa = {};
  for (const [k, n] of Object.entries(cuenta)) if (k.startsWith("regla:")) {
    const regla = RULES.find((r) => r.id === k.split(": ").slice(1).join(": "));
    if (regla) porCapa[regla.capa] = (porCapa[regla.capa] || 0) + n;
  }
  let md = "# informe de uso de apkc\n\n";
  md += `- eventos: ${l.eventos.length}\n- generado: ${new Date().toISOString()}\n`;
  if (Object.keys(porCapa).length) {
    md += "\n## dónde se atascó, por capa\n\n";
    for (const [c, n] of Object.entries(porCapa).sort((a, b) => b[1] - a[1]))
      md += `- ${c}: ${n}\n`;
  }
  if (Object.keys(cuenta).length) {
    md += "\n## conteo\n\n";
    for (const [k, n] of Object.entries(cuenta).sort((a, b) => b[1] - a[1]))
      md += `- \`${k}\` x${n}\n`;
  }
  md += "\n## pasos\n\n" + (l.pasos.map((p) => `- ${p}`).join("\n") || "(ninguno anotado)") + "\n";
  md += "\nLos logs pegados en la página se quedan en este navegador: aquí solo hay contadores y las líneas que la página reconoció.\n";
  return md;
}

export { buildProject, makeZip, analyze, crc32, RULES, loadLog, saveLog, registrar, informe };

// Puente para las pruebas de navegador que ya existen y para la consola de desarrollo.
// La página importa este módulo; no hay dependencia de estos globales en el código nuevo.
if (typeof window !== "undefined") {
  Object.assign(window, { buildProject, makeZip, analyze, crc32, RULES, loadLog, saveLog, registrar, informe });
}
