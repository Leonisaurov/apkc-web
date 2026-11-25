# PLAN-apps-reales — plan pausado (traslado al corpus)

**Plan pausado el 2026-11-24 para ejecutar la separación del emulador; retomar en Fase B.**

Este fichero es copia literal de `…/c6f089/plan-apps-reales/PLAN-apps-reales.md` (449 líneas), el plan
aprobado que pausamos. La separación ya está ejecutada (§350-§352 y §356), así que la frontera del
apartado 5 del plan original está resuelta: **Fase A no se rehace** (está medida en §355) y las Fases
B-G se ejecutan sobre el módulo separado.

**Numeración al reanudar.** La numeración `§350`-`§357` que propone el plan original **no es la del
corpus**: al registrar las mediciones se usaron `§354` (medición base, cuatro SHA y los tres errores
de dex) y `§355` (Fase A, ya hecha), y `§350`-`§352` y `§356` están ocupados por la separación. El
frente del pin único del emulador consume `§357`, así que **al reanudar se usan `§358` opcode (en curso), `§359` Fase C, `§360` Fase B, `§361` Fase D, `§362` Fase E, `§363` Fase F y `§364` Fase G**, en ese orden; `§366` queda reservado a la línea base del cuarto APK. `§353` no se reutiliza para Fase C: sigue siendo reserva vacía de F5 en `docs/evidencia.md:389`.
No se rehace Fase A: sus tuplas byte a byte y su frontera (`pasos
292/286/207`, bloqueo `ArchTaskExecutor … opcode 0x1d`) están en `docs/evidencia/15-apps-reales.md:71-177`.

**Estado del traslado.** §354 y §355 están medidos y publicados en el índice
(`docs/evidencia.md:360-361`); las Fases B-G siguen siendo **criterios, no resultados** (regla 11).
Este fichero no añade medición nueva y no toca `docs/plan.md` ni los planes de los worktrees.

---

# PLAN-apps-reales — llevar F-Droid, Termux y NewPipe al emulador web

## 0. Encuadre y criterio de «bien respondida»

**Pregunta en una frase**: ¿qué hay que cambiar en `apkc preview` y en la pestaña para que los tres
APK reales que tiene el usuario en `~/Downloads` muestren su pantalla de inicio verdadera en vez de
una pantalla falsa?

**Alcance**: solo comandos locales (lectura, `unzip -l`, `aapt2`, `apkanalyzer`, `dexdump`,
`apksigner`, `apkc preview` nativo). Sin CDP, sin chromium, sin gate, sin red externa y sin escribir
en el repo. Este fichero es el único artefacto que se escribe.

**Criterio falsable de éxito**, por app, sobre el APK sin recompilar ni parchear:

1. `apkc preview <apk>` devuelve `rc=0`, `ejecucion.error` ausente, `ejecucion.pasos > 0`,
   `actividad` = la activity lanzadora real declarada en el manifiesto, `layout` = el layout real de
   esa activity (o, si la pantalla es Compose, un aviso explícito `compose-sin-modelo` en vez de un
   layout inventado), y **0 avisos** de la familia «llamada que no se sabe resolver».
2. La pestaña pinta ese modelo sin `FALLO` y el texto de la pantalla coincide con el del modelo.
3. Ninguna petición fuera del origen (regla 4 del repo).

**Hoy ninguno de los tres cumple**: los tres dan `pasos:0`, un `ejecucion.error`, y el mismo layout
falso `res/A1.xml` con 3 nodos (es `abc_action_bar_title_item`, un ítem de la barra de acciones de
AppCompat). Esa es la línea base que el plan tiene que mover.

---

## 1. §350 — Medición base de los tres APK (HECHO VERIFICADO)

Todos los comandos se ejecutaron en esta sesión, sin red y sin chromium.

### 1.1 Localización e identidad

```
find /home/leonisauro/Download /home/leonisauro/Descargas /home/leonisauro/Downloads -maxdepth 3 -type f \( -iname '*fdroid*.apk' -o -iname '*termux*.apk' -o -iname '*newpipe*.apk' -o -iname '*.apk' \) 2>/dev/null | head -40
```
Salida (relevante):
```
/home/leonisauro/Downloads/F-Droid.apk
/home/leonisauro/Downloads/org.schabi.newpipe_1017.apk
/home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
/home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_x86_64.apk
```
Hay **dos** Termux (armeabi-v7a y x86_64); se mide el de arm por ser el que el cell puede ejecutar
y el de x86_64 queda como variante registrada, no como app distinta. El resto (`app.apk`,
`ApkShare.apk`, `Contador.apk`, `app (1..3).apk`) no es ninguna de las tres pedidas.

```
sha256sum "/home/leonisauro/Downloads/F-Droid.apk" "/home/leonisauro/Downloads/org.schabi.newpipe_1017.apk" "/home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk" "/home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_x86_64.apk"
```
```
dac55a0d17b1edf7e7286b48cfbd70c7c52fbb32a00af062c68830a437c47959  F-Droid.apk
576228432fd261ab3d34b30ea9d5a23b3d53022e18c98f5b5e2e18b885988f05  org.schabi.newpipe_1017.apk
89416397b70f9ff67a0044e8abe6ef82487cd48fcf543e2d23e02688cc541cf0  termux-app_..._armeabi-v7a.apk
3550e61f4d9eb49b712fd1bd9519dc37085a4d8eb597c57a340f0a64859b7144  termux-app_..._x86_64.apk
```
Tamaños: F-Droid 12 774 751 B; NewPipe 16 980 865 B; Termux arm 32 250 911 B; Termux x86_64
35 189 933 B.

### 1.2 Manifiesto (`aapt2 dump badging` / `apkanalyzer manifest print`)

| app | package / versión | minSdk | targetSdk | compileSdk | native-code |
| --- | --- | --- | --- | --- | --- |
| F-Droid | `org.fdroid.fdroid` 2000053 / 2.0.3 | 24 | **37** | 37 | arm64-v8a, armeabi-v7a, x86, x86_64 |
| NewPipe | `org.schabi.newpipe` 1017 / 0.29.3 | 23 | 35 | 37 | arm64-v8a, armeabi-v7a, x86, x86_64 |
| Termux (ambas) | `com.termux` 1002 / 0.118.3 | 24 | 28 | 30 | armeabi-v7a / x86_64 |

Comando: `aapt2 dump badging <apk> 2>&1 | grep -iE '^package:|SdkVersion|launchable|native-code|uses-library|uses-feature|other-|main'`.
Salidas literales clave:
```
package: name='org.fdroid.fdroid' versionCode='2000053' ... compileSdkVersion='37'
minSdkVersion:'24'  targetSdkVersion:'37'
leanback-launchable-activity: name='org.fdroid.MainActivity'
package: name='org.schabi.newpipe' versionCode='1017' ... compileSdkVersion='37'
minSdkVersion:'23'  targetSdkVersion:'35'
launchable-activity: name='org.schabi.newpipe.MainActivity'
package: name='com.termux' versionCode='1002' ... compileSdkVersion='30'
minSdkVersion:'24'  targetSdkVersion:'28'
launchable-activity: name='com.termux.app.TermuxActivity'
```
Las tres declaran `uses-library-not-required:'androidx.window.extensions'` y
`'androidx.window.sidecar'`. F-Droid pide 34 permisos (INTERNET, BLUETOOTH_*,
MANAGE_EXTERNAL_STORAGE, REQUEST_INSTALL_PACKAGES, QUERY_ALL_PACKAGES, CAMERA…); NewPipe y Termux
piden INTERNET/ACCESS_NETWORK_STATE y Termux añade SYSTEM_ALERT_WINDOW, READ_LOGS, DUMP,
WRITE_SECURE_SETTINGS, PACKAGE_USAGE_STATS, FOREGROUND_SERVICE.

Componentes (`apkanalyzer manifest print <apk>`): F-Droid 38 activity/alias + 12 service + 10
receiver + 5 provider (SwapService, LocalRepoService, AppInstallService, WorkManager
SystemJobService, ACRA…); NewPipe 14 activity + 9 service + 6 receiver + 3 provider
(PlayerService, DownloadManagerService, FeedLoadService, `RouterActivity$FetcherService`,
`ReCaptchaActivity`, `net.newpipe.app.ComposeActivity`, ACRA…); Termux 6 activity + 2 service +
2 receiver + 2 provider (`TermuxService`, `RunCommandService`, `TermuxDocumentsProvider`,
`TermuxOpenReceiver`).

Firma (`apksigner verify --verbose --print-certs <apk>`): F-Droid v1=false, v2=true, v3=true; NewPipe
v1/v2/v3=true; Termux v1=false, v2=true, v3=false. No es bloqueante para el emulador, pero dice que
un re-firmaje en la pestaña tiene que escribir v2 (y v3 cuando aplique).

### 1.3 Contenido del ZIP (`unzip -l`)

Comando: bucle con `unzip -l <apk> | grep -cE '^ *[0-9]+ '`, `grep -oE 'classes[0-9]*\.dex'`,
`grep -oE 'lib/[a-zA-Z0-9_-]+/[^ ]+\.so'`, `grep -c 'resources.arsc'`, `grep -cE ' res/'`,
`grep -cE ' assets/'`.

| app | entradas | `classes*.dex` | `.so` | arsc | `res/` | `assets/` |
| --- | --- | --- | --- | --- | --- | --- |
| F-Droid | 974 | **3** (`classes.dex`, 2, 3) | `libandroidx.graphics.path.so` ×4 ABI | 1 | 703 | 96 |
| NewPipe | 1326 | **3** (ídem) | `libandroidx.graphics.path.so`, `libsqliteJni.so` ×4 ABI | 1 | 1021 | 138 |
| Termux arm | 758 | **29** (de `classes.dex` a `classes29.dex`) | `libtermux.so`, `libtermux-bootstrap.so` | 1 | 637 | 0 |

**Multidex real en los tres.** Con `dexdump` (estricto, `grep 'Class descriptor.*L<clase>;'`) se
localizó la clase candidata a lanzadora (cuál es la lanzadora real queda como hipótesis a confirmar
en Fase C, §359):
```
F-Droid  : org/fdroid/MainActivity      -> classes2.dex   -> 1 definición
NewPipe  : org/schabi/newpipe/MainActivity -> classes.dex -> 1 definición
Termux   : com/termux/app/TermuxActivity   -> classes20.dex -> 1 definición
```
`apkc preview` lee **solo** `classes.dex` (`apkc/preview.go:746`), así que F-Droid y Termux no
pueden ni empezar.

Los tres tienen recursos ofuscados por aapt2 (F-Droid y NewPipe): los ficheros viven como
`res/f_.xml`, `res/v9.xml`, `res/A1.xml`… y el nombre lógico solo está en `resources.arsc`:
```
aapt2 dump resources F-Droid.apk | grep -A1 'layout/main_tab_nearby'
    resource 0x7f0b0039 layout/main_tab_nearby
      () (file) res/f_.xml type=XML
aapt2 dump resources org.schabi.newpipe_1017.apk | grep -A4 'layout/activity_main'
    resource 0x7f0d001e layout/activity_main
      () (file) res/v9.xml type=XML
    resource 0x7f0d001f layout/activity_player_queue_control
      () (file) res/tC.xml type=XML
      (land) (file) res/El.xml type=XML
```
Termux conserva rutas legibles (`res/layout/activity_termux.xml`); tiene 137 layouts, 119 vectores,
58 `anim`/`animator`, 19 `xml` y 2 `raw`. F-Droid **no tiene** `layout/main` ni `activity_main`: su
interfaz es Compose. El `layout/main_tab_nearby` medido (`res/f_.xml`) no es la pantalla de esa
actividad, y la identidad de la lanzadora sigue como hipótesis a confirmar en Fase C (§359).
NewPipe sí tiene `layout/activity_main` (`res/v9.xml`); el `(land) res/El.xml` pertenece a
`layout/activity_player_queue_control`, no a `activity_main`.

### 1.4 Temas, bibliotecas y APIs (`strings` sobre los dex y el arsc)

```
for d in $(unzip -Z1 <apk> | grep -E '^classes[0-9]*\.dex$'); do unzip -p <apk> "$d" | strings -n 6; done | grep -oE '<marcadores>' | sort | uniq -c | sort -rn
```
| marcador | F-Droid | NewPipe | Termux |
| --- | --- | --- | --- |
| `kotlin` | 5642 | 1067 | 7553 |
| `androidx.compose.material3.*` | 39 `ComposableSingletons` + decenas | `ComposeActivity` en el manifiesto | — |
| `Fragment` | 305 | 752 | 874 |
| `RecyclerView` | 14 | 92 | 177 |
| `ViewPager` / `ViewPager2` | — | 21 | 73 / 57 |
| `TabLayout` | — | 24 | 129 |
| `okhttp3` | 308 | 189 | — |
| `Room` | 144 | 62 (`androidx.room` y `androidx.room3`) | 2 |
| `WorkManager` | 52 | 33 | — |
| `ExoPlayer` / `com.google.android.exoplayer2` | — | 92 / 27 | — |
| `WebView` / `WebChromeClient` | — | 41 / 2 | 5 / — |
| `Coil` / `Glide` | Glide 6 | Coil 2 | — |
| `TerminalSession` | — | — | 69 |
| `java.nio.channels.SeekableByteChannel` | — | 1 | 1 |
| `BroadcastReceiver` / `ContentProvider` | 30 / 10 | 17 / 9 | 17 / 10 |

`strings` sobre `resources.arsc` confirma dependencia fuerte de temas:
F-Droid `m3_` ×938, `Material3` ×150, `AppCompat` ×87, `MaterialComponents` ×76;
NewPipe `exo_` ×232, `abc_` ×222, `AppCompat` ×98, `Theme.YouTube`/`Theme.SoundCloud`/
`Theme.PeerTube`/`Theme.Bandcamp` (temas propios); Termux `abc_` ×516, `AppCompat` ×133,
`MaterialComponents` ×103. Ninguno usa `font` como recurso propio en arsc (el marcador `font`
viene de nombres de atributo).

Cadenas de red: F-Droid trae SQL con `JOIN RepositoryPreferences`, `SELECT repoId, packageName…`
(índice de repositorio en SQLite/Room); NewPipe trae `HttpDataSource$InvalidResponseCodeException`
y `SocketTimeoutException`; Termux trae `ExecutionCommand`, `mTermuxTerminalSessionClient` y
`libtermux-bootstrap.so`.

### 1.5 Comportamiento actual de `apkc preview` (HECHO VERIFICADO)

Binario local fresco: `/home/leonisauro/.cache/apkc-web/emulador-fiel/apkc` (2026-11-24 20:06).
Comandos y salidas (recortadas a lo esencial, `rc=0` en los tres):

```
$ apkc preview /home/leonisauro/Downloads/F-Droid.apk
{"paquete":"org.fdroid.fdroid","version":"2.0.3","actividad":"org.fdroid.IconCalculatorActivity",
 "layout":"res/A1.xml","nodos":3,
 "ejecucion":{"metodo":"onCreate","pasos":0,
 "error":"L_COROUTINE/CoroutineDebuggingKt;: static values: value #0 of type 0x17 is not read by this fork yet"}}
```
```
$ apkc preview org.schabi.newpipe_1017.apk
{"paquete":"org.schabi.newpipe","version":"0.29.3","actividad":"org.schabi.newpipe.MainActivity",
 "layout":"res/A1.xml","nodos":3,
 "ejecucion":{"metodo":"onCreate","pasos":0,
 "error":"Landroidx/compose/foundation/layout/SpacerKt;: 4 static values for 8 static fields"}}
```
```
$ apkc preview termux-app_v0.118.3+github-debug_armeabi-v7a.apk
{"paquete":"com.termux","version":"0.118.3","actividad":"com.termux.app.TermuxActivity",
 "layout":"res/layout/abc_action_bar_title_item.xml","nodos":3,
 "ejecucion":{"pasos":0,"error":"el classes.dex no se deja leer: string #808 claims 18 utf-16 units and holds 20"}}
```
En los tres, el árbol pintado es el mismo `LinearLayout` con `id/action_bar_title` e
`id/action_bar_subtitle`, estilo `style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem`.
**Es una pantalla falsa, no una aproximación de la app.**

### 1.6 Registro obligatorio de `§354` antes de registrar `§355`

`§350` está medido arriba. **No rehacer Fase A (§355, ya hecha; ver docs/evidencia/15-apps-reales.md:71-177); el frente siguiente es §358 opcode**. El registro en el corpus quedó en este orden:

1. Crear el cuerpo `## 350. Apps reales en ~/Downloads: estado actual` en
   `docs/evidencia/15-apps-reales.md` (o el fichero libre que se elija). Incluir, con los comandos
   literales que los producen:
   - los cuatro `sha256sum` de §1.1 (`dac55a0d…`, `57622843…`, `89416397…`, `3550e61f…`);
   - el error literal de F-Droid: `L_COROUTINE/CoroutineDebuggingKt;: static values: value #0 of
     type 0x17 is not read by this fork yet` (`pasos:0`, `layout:res/A1.xml`);
   - el error literal de NewPipe: `Landroidx/compose/foundation/layout/SpacerKt;: 4 static
     values for 8 static fields` (`pasos:0`, `layout:res/A1.xml`);
   - el error de Termux: **`incierto` si no se reproduce** con el comando y su salida literal en
     el cuerpo (`el classes.dex no se deja leer: string #808 claims 18 utf-16 units and holds
     20`). La medición local de este subagente no sustituye al registro: si no se re-ejecuta y se
     pega la salida, se escribe como hipótesis, no como hecho (regla 11).
2. Añadir la fila `§350 | Apps reales en ~/Downloads: estado actual` a la tabla de
   `docs/evidencia.md`, junto a la de `§349`, y ajustar el preámbulo a `§1`–`§350`.
3. Añadir un bullet en `docs/situacion.md` con el recap: los tres APK arrancan con `pasos:0` y el
   layout falso `res/A1.xml`, sostenido por ese `§`.
4. Solo después de los tres pasos anteriores, registrar `§355` (Fase A; ya medido). El orquestador ajusta `AGENTS.md`
   al integrar; este plan no lo toca.

Nota: en `§350` la lanzadora de F-Droid aparece solo como `leanback-launchable-activity`; su
identidad se confirma en Fase C, no se da por medida aquí.

---

## 2. Causas raíz compartidas (una solución, varios apps)

| id | Causa raíz (con `archivo:línea`) | Apps | Solución reutilizable |
| --- | --- | --- | --- |
| R1 | `anchoValor` no conoce `VALUE_STRING` 0x17 ni TYPE/FIELD/METHOD/ENUM/ARRAY/ANNOTATION (`apkc/dexcode.go:47-58`, constantes en `:23-42`). `dexdump` muestra `ARTIFICIAL_FRAME_PACKAGE_NAME = "_COROUTINE"` en `L_COROUTINE/CoroutineDebuggingKt;` → static String real. | F-Droid (y cualquier dex con constantes String) | Ampliar `anchoValor` + `valoresEstaticos` para leer un `uleb128` de índice y saltar recursivamente arrays/anotaciones. |
| R2 | `methodsDeCadaClase` exige `len(valores)==len(staticFields)` (`apkc/dexcode.go:354-364`). `dexdump` de `SpacerKt` muestra 8 campos y solo 4 valores (`Horizontal=15`, el resto sin `value`). | NewPipe (y R8 en general) | Rellenar con `VALUE_NULL` las entradas ausentes al final; nunca rechazar por corto. |
| R3 | Validación MUTF-8 estricta `utf16Len(s)!=utf16Count` (`apkc/dexreader.go:280-282`). `dexdump -f` abre sin queja el dex de Termux (DEX 037, 65 866 strings, 6 525 class_defs). | Termux | Aislar la cadena #808 y corregir `decodeMUTF8`/`utf16Len` (o relajar el chequeo a «no truncado», medido contra dexdump). |
| R4 | `modeloVistaDeAPKConEventos` abre solo `classes.dex` (`apkc/preview.go:746`). | F-Droid, Termux | Cargar todos los `classes*.dex` en orden y resolver descriptors de clase a través del conjunto; una sola tabla de clases para el intérprete. |
| R5 | `actividadLanzadora` toma el primer MAIN+LAUNCHER, aunque sea `activity-alias` con `enabled=false` (`apkc/preview.go:1245-1263`, `esLanzadora:1290-1313`); F-Droid picking `IconCalculatorActivity` (alias con `targetActivity="org.fdroid.MainActivity"`, `enabled=false`). | F-Droid | Resolver `targetActivity`, descartar aliases deshabilitados, priorizar `<activity>` sobre `<activity-alias>` y DEFAULT category. |
| R6 | `rutaDelLayout`: sin `setContentView` y sin layout llamado `main`, devuelve el primero por orden alfabético (`apkc/preview.go:1235-1240`) → `abc_action_bar_title_item`. | los tres | Nunca inventar pantalla: pedir id a `setContentView`, mapear `MainActivity`→`activity_main`, `TermuxActivity`→`activity_termux`; si no hay XML (Compose), publicar aviso `compose-sin-modelo` y un marco raíz, no un layout ajeno. |
| R7 | `intentaStub` atiende ~90 métodos por sufijo (`apkc/previewinterprete.go:4014-5303`); no hay `RecyclerView`, `Fragment`, `ViewPager`, `TabLayout`, `WebView`, `Service`, `ContentProvider`, red/TLS, ExoPlayer ni Compose. | los tres | Ampliar stubs por familia con effects observables y avisos contabilizados; la misma capa sirve a las tres apps. |
| R8 | Límites del intérprete: `pasosMaximos=200000`, `profundidadMaximaDalvik=48`, `avisosMaximos=24`, `filasMaximas=50`, `pantallasMaximas=1024` (`apkc/previewinterprete.go:47-63`). | los tres | No subirlos «a ver si ahora»: medir primero cuál se alcanza con cada app y ampliar solo el que la medición diga, con aviso dedicado. |
| R9 | Sin JNI ni `.so` (`apkc/previewinterprete.go:26`). `libtermux.so`, `libtermux-bootstrap.so`, `libsqliteJni.so`, `libandroidx.graphics.path.so`. | los tres | Stub de `System.loadLibrary` que registre la biblioteca y devuelva un objeto inerte; **no simular** terminal ni SQLite. Clasificación final: parcial. |
| R10 | Resources ofuscados + temas AppCompat/Material3 (`aapt2 dump resources`, §1.3/§1.4). | F-Droid, NewPipe | La tabla ya guarda la ruta del pool (`rutaDeLayoutPorId`, `apkc/preview.go:1134-1145`); verificar que `leeTabla` resuelve `res/v9.xml` y que los estilos del tema se aplican al texto/fondo. |
| R11 | `plugin` de pestaña: `vista-xml.js:254-258` dibuja `ViewPager`, `WebView`, `Toolbar`, `Switch`… como cajas, pero `RecyclerView`/`ListView` no están en la lista y no hay contenido de WebView ni pestañas reales. | NewPipe, Termux | Extender el modelo con nodos `lista`/`pestañas`/`webview` y que el panel pinte filas del adaptador y etiquetas de pestañas; sin falsificar contenido web. |

---

## 3. Tabla por app: requisitos → rechazo → clasificación

| app | Requisitos medidos | Dónde se rechaza / no se modela hoy | Clasificación |
| --- | --- | --- | --- |
| **F-Droid** (`dac55a0d…`) | activity `org.fdroid.MainActivity` en `classes2.dex`; Compose Material3 + `navigation3`; Room con índice de repos; WorkManager; okhttp; Glide; 3 dex; 1 `.so`; targetSdk 37; temas `m3_`/Material3; 12 services, 10 receivers, 5 providers; MAIN+LAUNCHER ×3 y alias deshabilitado | `apkc/dexcode.go:394` (String 0x17), `apkc/preview.go:746` (solo classes.dex), `:1245-1263` (alias), `:1235-1240` (layout alfabético), `intentaStub` sin Compose/Room/WorkManager | **bloqueante** (ni arranca); la pantalla Compose es además **parcial** por diseño (§6) |
| **NewPipe** (`57622843…`) | activity `org.schabi.newpipe.MainActivity` en `classes.dex`; `layout/activity_main` ofuscado `res/v9.xml`; ExoPlayer/Media3, WebView+WebChromeClient, RecyclerView 92, ViewPager 21, TabLayout 24, Fragment 752, Room, WorkManager, Coil, `SeekableByteChannel`; temas propios (`Theme.YouTube` etc.) y `exo_` ×232; 3 dex; 2 `.so` | `apkc/dexcode.go:362` (4 valores/8 campos), `apkc/preview.go:1235-1240` (layout falso), `intentaStub` sin RecyclerView/WebView/ExoPlayer/Fragment/TabLayout, `vista-xml.js:254-258` sin lista con filas | **bloqueante** en el lector; **parcial** en la pantalla incluso tras arreglar el lector |
| **Termux arm** (`89416397…`) | activity `com.termux.app.TermuxActivity` en `classes20.dex`; 29 dex; `libtermux.so` + `libtermux-bootstrap.so`; 137 layouts, 119 vectores, 58 anim; `TerminalSession` ×69, `ExecutionCommand`, RecyclerView 177, ViewPager2 57, TabLayout 129, Fragment 874; `TermuxService` foreground, `RunCommandService`, `TermuxDocumentsProvider` | `apkc/dexreader.go:281` (string #808), `apkc/preview.go:746` (classes20.dex), `intentaStub` sin TerminalSession/Service/provider/fragmentos, `previewinterprete.go:26` sin `.so` | **bloqueante**; la semántica de terminal queda **parcial** por diseño |
| Termux x86_64 (`3550e61f…`) | Misma versión 0.118.3, misma estructura ZIP (758 entradas, 29 dex, 637 `res/`), distinto ABI | Iguales tres causas; no medido con `apkc preview` por separado | **incierto** en detalle, se da por cubierto por la variante arm tras medir |

---

## 4. Fases de implementación, priorizadas por «desbloquea más apps»

Convención: cada fase lleva su `§` propuesto, ficheros, comando focal, criterio falsable y commit.
El índice está en `§349` (`docs/evidencia.md:357`) y `§350`-`§357` ya están ocupados por la separación
y el pin del emulador (`docs/evidencia.md:388`); la asignación vigente está en `docs/evidencia.md:389,393`:
Fase A `§355` (hecha), `§358` opcode, C `§359`, B `§360`, D `§361`, E `§362`, F `§363` y G `§364`, con `§366` reservado.
Regla 11: **nada se marca «hecho» sin medición propia**;
los `§` de abajo son criterios, no resultados.

### Fase A — §355: lector de dex tolerante con dex de R8/javac real (desbloquea 3/3)

- **Por qué va primero**: es el primer error de los tres y es compartido. Sin esto no hay ni
  `onCreate`.
- **Ficheros**: `apkc/dexcode.go:47-58` (`anchoValor`), `:379-404` (`valoresEstaticos`),
  `:354-368` (`methodsDeCadaClase`); `apkc/dexreader.go:261-285` (`readStringData`),
  `:522-562` (`decodeMUTF8`), `:579-586` (`utf16Len`). Nuevos tests
  `apkc/dexvalores_reales_test.go` y `apkc/dexmutf8_real_test.go` con las tuplas byte a byte
  extraídas de los tres APK (F-Droid `L_COROUTINE/CoroutineDebuggingKt;`, NewPipe `SpacerKt`,
  Termux string #808).
- **Comandos focales**:
  `go test -run 'ValoresEstaticosReales|Mutf8Real' -v .` en `apkc/`, y luego
  `apkc preview <cada apk>` con el binario reconstruido.
- **Criterio falsable**: desaparecen los tres `ejecucion.error` de §1.5; `dexdump -a` sobre los
  mismos dex sigue mostrando los mismos valores; la suite Go queda en verde y el escritor de dex no
  cambia ni un byte (test de paridad existente).
- **Commit**: `fix(preview): lee valores estáticos y MUTF-8 de dex reales`.
- **Dependencia de la separación**: **no**; toca el lector compartido, así que **esta fase se
  ejecuta antes de separar el módulo**, con cuidado de no tocar el escritor. Si el plan de
  separación moviera `dexcode.go`/`dexreader.go` a `apkc-emulador/` antes de que esta fase corra,
  las citas `file:line` de arriba quedan inválidas: re-anclarlas con `grep -n` sobre la ruta nueva
  (`grep -nE 'func (\([^)]*\) )?(anchoValor|valoresEstaticos|methodsDeCadaClase)' apkc-emulador/*.go`
  y `grep -nE 'func (\([^)]*\) )?(readStringData|decodeMUTF8|utf16Len)' apkc-emulador/*.go`) y
  ejecutar la fase allí, nunca sobre la ruta vieja. El receptor en el patrón es obligatorio: el
  crítico midió que sin él solo aparece `dexcode.go:47` y se pierden los dos métodos con receptor.
  Salida esperada sobre la ruta actual (la misma tras mudar los ficheros sin renombrarlos):
  ```
  apkc/dexcode.go:47:func anchoValor(vf byte) int {
  apkc/dexcode.go:347:func (d *dexFile) methodsDeCadaClase() (map[string]classData, error) {
  apkc/dexcode.go:379:func (d *dexFile) valoresEstaticos(off uint32) ([]encodedField, error) {
  ```
  ```
  apkc/dexreader.go:261:func (d *dexFile) readStringData() error {
  apkc/dexreader.go:522:func decodeMUTF8(b []byte) (string, error) {
  apkc/dexreader.go:579:func utf16Len(s string) int {
  ```

### Fase B — §360: multidex en el modelo (desbloquea 2/3 para empezar)

- **Ficheros**: `apkc/preview.go:694-790` (bucle `entradaDeZip(zr,"classes.dex")`),
  nueva función `dexDelApk` que devuelve `[]*dexFile` y una tabla descriptor→dex;
  `ejecutaOnCreateConEventos` y `preparaVistasDelLayout` deben recibir el conjunto.
- **Comando focal**: `apkc preview <F-Droid>` y `<Termux>`; `dexdump` ya fijó en §1.3 que las
  activities están en `classes2.dex` y `classes20.dex`.
- **Criterio falsable**: `ejecucion.metodo != ""` y `pasos > 0` en F-Droid y Termux; el descriptor
  de la activity lanzadora se encuentra en el dex correcto; NewPipe no regresa.
- **Commit**: `feat(preview): ejecuta la actividad de cualquier classesN.dex`.
- **Dependencia de la separación**: **este frente se ejecuta después de separar el módulo**
  (vive en `preview.go`, dentro del paquete que se va a extraer).

### Fase C — §359: lanzadora y layout correctos, sin pantallas inventadas

- **Ficheros**: `apkc/preview.go:1245-1313` (`actividadLanzadora`, `esLanzadora`),
  `:1124-1241` (`rutaDelLayoutDeLaActividad`, `rutaDelLayout`).
- **Reglas a implementar**: (1) un `activity-alias` con `enabled="false"` no es lanzador;
  (2) si es alias, seguir `targetActivity`; (3) priorizar `<activity>` con MAIN+LAUNCHER+DEFAULT;
  (4) el layout sale de `setContentView` (id ya lo lee el intérprete) o del nombre de la activity;
  (5) si no hay layout XML para la actividad, **no** caer al alfabético: publicar
  `sin-layout-xml` en el modelo y un marco raíz con la etiqueta de la app.
- **Criterio falsable (lanzadora aún como hipótesis)**: no se da por probado que
  `org.fdroid.MainActivity` sea la lanzadora real hasta ejecutar completos
  `apkanalyzer manifest print /home/leonisauro/Downloads/F-Droid.apk` y
  `aapt2 dump badging /home/leonisauro/Downloads/F-Droid.apk`, y confirmar con su salida literal que
  `<activity android:name="org.fdroid.MainActivity">` lleva `MAIN`+`LAUNCHER`, y que
  `org.fdroid.IconCalculatorActivity` es un `activity-alias` con `enabled="false"` cuyo
  `targetActivity` apunta a esa activity. Hoy solo está medido
  `leanback-launchable-activity: name='org.fdroid.MainActivity'`: la identidad de la lanzadora es
  **hipótesis a confirmar con comando y salida esperada**, no dato. Tras confirmarla: el modelo dice
  esa activity y no `IconCalculatorActivity`; NewPipe usa `res/v9.xml` (`layout/activity_main`);
  Termux usa `res/layout/activity_termux.xml`; en los tres desaparece `id/action_bar_title`.
- **Commit**: `fix(preview): elige lanzadora y layout reales sin inventar pantalla`.
- **Dependencia de la separación**: **después de separar el módulo**.

### Fase D — §361: stubs de framework por familia (desbloquea las pantallas reales)

- **Ficheros**: `apkc/previewinterprete.go:4014-5303` (`intentaStub`) y nuevos
  `apkc/previewstubs_<familia>.go`; tests en `apkc/previewstubsframework_test.go` y
  `apkc/previewstubs_test.go`.
- **Familias, en orden de rentabilidad**: (1) `RecyclerView.Adapter`/`onCreateViewHolder`/
  `onBindViewHolder` → filas reales en el modelo (sirve a los 3); (2) `Fragment` + `ViewPager2` +
  `TabLayout` → pestañas y contenido (NewPipe, Termux); (3) ciclo de vida `Service`/
  `startService`/`bindService` y `ContentProvider`/`BroadcastReceiver` (los 3);
  (4) `WebView`+`WebChromeClient` como placeholder con aviso, sin pedir red (NewPipe, Termux);
  (5) red/TLS: `HttpURLConnection`/okhttp devuelven una respuesta vacía **registrada** en la
  consola, nunca una petición real (los 3); (6) `ExoPlayer`/Media3 y `SeekableByteChannel`
  como objeto inerte (NewPipe); (7) `System.loadLibrary` inerte (§4 R9).
- **Criterio falsable**: en la primera pantalla de cada app, el número de avisos de la familia
  «llamada que no se sabe resolver» es **0** (`apkc preview … | python3 -c …` contando
  `ejecucion.avisos`), y las filas/pestañas del modelo son las que el adaptador/devuelve.
- **Commit**: `feat(preview): stubs de listas, pestañas y servicios para apps reales`.
- **Dependencia de la separación**: **después de separar el módulo**; es la fase que más se beneficia
  de tener el intérprete suelto.

### Fase E — §362: temas, recursos ofuscados y pintado en la pestaña

- **Ficheros**: `apkc/preview.go` (resolución de estilos del tema), `apkc/previewdibujo.go:102-127`,
  `studio/vista-xml.js:254-258`, `studio/previa-panel.js`, `studio/vistas.js`. Cambios en `studio/`
  siguen la regla 9: commit local; `git push` a `apkc-studio` solo con autorización expresa del
  usuario; sin ella, queda sin empujar y se reporta como pendiente. El push previo autorizado
  (`e9274dd..a4e983c`) no autoriza los futuros; después se actualiza el puntero del submódulo.
- **Criterio falsable**: el modelo trae texto/fondo del tema (`m3_`/`abc_`), las filas del
  adaptador se pintan y la pestaña enseña la misma pantalla que `apkc preview` (sin `FALLO`).
- **Commit**: `feat(previa): pinta listas, pestañas y tema de apps reales` (submódulo) +
  `chore(studio): actualiza puntero`.
- **Dependencia de la separación**: **después de separar el módulo**.

### Fase F — §363: F-Droid y la pantalla Compose (decisión de alcance)

- `F-Droid` no tiene layout XML para `MainActivity`: su UI es `androidx.compose.material3.*` con
  `navigation3`. Emular Compose de verdad (composition, recomposición, `remember`) es un frente
  mucho mayor que el intérprete Dalvik actual.
- **Propuesta acotada**: detectar Compose por `org.jetbrains.compose.resources.AndroidContextProvider`
  en el manifiesto + ausencia de `setContentView` con layout, y publicar un modelo explícito
  `compose-sin-modelo` con el marco raíz y la etiqueta de la app. **No se da por «cubierta»**: se
  clasifica como **parcial** y se pide decisión del usuario antes de invertir en un runtime Compose.
- **Criterio falsable**: `apkc preview F-Droid.apk` ya no pinta `res/A1.xml` y el aviso aparece
  documentado; la pestaña no muestra una pantalla que parezca real sin serlo.
- **Dependencia de la separación**: **después de separar el módulo**.

### Fase G — §364: fidelity en navegador (solo con autorización)

- CDP/Droidloom comparando la captura real con la del emulador para las pantallas que ya tengan
  modelo (NewPipe `activity_main`, Termux `activity_termux`).
- **Puerta dura**: autorización expresa del usuario; sin gate completo sin autorización; sin
  `surge`, sin `git push`, sin publicación. Hoy **no ejecutada**.
- **Criterio falsable**: captura CDP y captura de Droidloom con el mismo conjunto de nodos y texto,
  con la diferencia de geometría ya documentada en §293/§334-339.
- **Dependencia de la separación**: **después de separar el módulo** y de que Fases A-E estén en
  verde.

---

## 5. Lo que se delega al plan de separación del emulador

Otro agente está diseñando la separación del módulo de emulación. Este plan no la redefine; solo
declara la frontera:

- **Antes de separar**: únicamente Fase A (`§355`, ya hecha). Está en el lector compartido
  (`dexcode.go`/`dexreader.go`) y su prueba de no-regresión es que el escritor de dex quede byte a
  byte igual.
- **Después de separar el módulo**: Fases B, C, D, E, F y G. Todas viven en `preview*.go`, en los
  stubs del intérprete o en `studio/`; hacerlas antes obligaría a rehacerlas en el módulo nuevo.
- El plan de separación debe conservar el contrato de `apkc preview` (JSON de un solo renglón,
  `--pulsar`/`--escribir`/`--marcar`/`--recrear`/`--avanzar`/`--locale`) y la sesión viva
  (`previewsesion.go`, §313), porque las pruebas de las fases B-F se apoyan en él.
- Si la separación cambia la ruta de importación o mueve `dexcode.go`/`dexreader.go` a
  `apkc-emulador/`, `§355` se re-ancla antes de seguir con `grep -n` sobre la ruta nueva (ver Fase A);
  no se duplica el trabajo en dos árboles.

---

## 6. Reglas, límites e incertidumbre

**Reglas que rigen la ejecución**: una medición, un `§` con comando literal (regla 1); `§350` ya
está medido aquí, `§358+` son planes con criterio, no hechos (regla 11); `docs/evidencia.md` solo
enruta y el texto va a `docs/evidencia/NN-*.md`; si cambia el subconjunto aceptado, se actualiza la
fila del inventario de Fase 5 de `docs/plan.md`; commits `<type>(<scope>): <summary>`; prosa en
español con tildes y sin caracteres de otros alfabetos salvo literales; sin binarios versionados;
sin peticiones fuera del origen. Este subagente **no escribe en el corpus**: la delegación de
registro la hace el orquestador.

**Incertidumbre abierta (no inventar conclusiones)**:

1. Causa exacta del rechazo MUTF-8 de Termux: el archivo declara 18 unidades UTF-16 y
   `decodeMUTF8` produce 20; `dexdump` lo acepta, así que la sospecha es un falso rechazo, pero la
   causa (secuencia CESU-8, carácter suplementario o validación demasiado estricta) **no está
   aislada**. Siguiente paso: extraer los bytes de la cadena #808 y escribir un test con esa tupla.
2. El padded de static values (R2) se infiere de `dexdump` (8 campos, 4 valores); falta confirmar a
   nivel de bytes que el `encoded_array_item` declara tamaño 4.
3. El alcance de Compose (Fase F) es una decisión de producto: sin decisión, F-Droid queda parcial
   con aviso explícito, nunca fingido.
4. `targetSdk 37` de F-Droid está por encima de la tabla API→versión medida hasta 35 (§5.20 del
   plan). No se ha medido si `leeTabla` rechaza configuraciones conSDK 37; es una pregunta abierta
   para Fase E.
5. No hay medición CDP en este informe (prohibida por el encargo), así que la fidelidad en la
   pestaña se hereda de §293 y §334-339 como hipótesis, no como hecho nuevo.
6. La variante Termux x86_64 no se ejecutó con `apkc preview`; su sha y su estructura están
   registradas, su comportamiento es `incierto`.

**Siguiente paso concreto**: no rehacer Fase A (§355, ya hecha; ver docs/evidencia/15-apps-reales.md:71-177); el frente siguiente es §358 opcode. Con las tuplas byte a byte de los tres dex,
medir con `go test` focal y `apkc preview`, y solo entonces abrir Fase B sobre el módulo ya
separado. Si el módulo aún no está separado, esperar a la separación para B en vez de duplicar
código en dos árboles.
