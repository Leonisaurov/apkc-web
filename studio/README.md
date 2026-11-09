# apkc studio

Una página para crear proyectos de [apkc](../apkc) en el navegador y medir dónde se atasca
quien los usa. Sin dependencias ni build: se sirve por `http(s)` con `servidor.js` o un host
estático con COOP/COEP. Los ES modules no cargan por `file://`; allí solo se muestra un aviso.

Qué hace, en el orden en que duele:

1. **Genera el proyecto** (manifest, `MainActivity.java` o `.kt`, layout, styles, carpetas) y lo
   escribe en OPFS. Las plantillas son copia literal de `apkc/template/*`; `prueba.js` compara
   los atributos que apkc escribe con los del repo, porque una tilde dentro de
   `android:versionCode` rompe el XML y `aapt2` lo rechaza en el teléfono.
2. **Traduce la salida real de apkc** en el panel de diagnóstico: 24 reglas, cada una con `lit:`
   (la cadena que la motivó) y `de:` (quién la imprime). Las de apkc se verifican contra el
   fuente en la propia prueba; una regla inventada explicaría un error que nadie imprime, que es
   peor que no tenerla.
3. **Lista las líneas que no conoce** y lleva contadores de dónde se rompió cada intento. Eso es
   la medida de fricción; el informe en markdown es para traerlo a quien mantiene la herramienta.
   Los logs pegados no salen del navegador: los contadores se quedan en `localStorage`.
4. **Compila, firma y verifica** en la pestaña con `apkc.wasm`; el resto de este README lo
   detalla.

## Probar

```sh
node prueba.js                 # lo que genera: ascii, plantillas contra el repo, EOCD, reglas
node prueba-navegador.js       # chromium headless por CDP: DOM, clicks, descarga, localStorage
APKC=/ruta/apkc sh e2e.sh      # todo lo anterior + compilar y verificar los dos proyectos
```

`package.json` repite esos comandos con `npm test`, `npm run test:navegador` y `npm run e2e`.
No hay dependencias: el `package.json` existe para que un editor o una CI encuentren los nombres
de las tareas, no para instalar nada.

## La pestaña: apkc.wasm corriendo aquí dentro

`index.html` tiene una sección que **sí ejecuta apkc en el navegador**, con los ficheros detrás de
OPFS y el worker como hospedador. No hay backend: `servidor.js` solo sirve estáticos con las
cabeceras COOP/COEP que OPFS exige (por `file://` no cargan los módulos).

```sh
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" node servidor.js'   # http://127.0.0.1:8787/
# y la prueba que lo conduce todo, con una apk, un keystore y el jar del SDK para el selector
# de fichero. El servidor ya detecta el SDK local sin ANDROID_JAR; la prueba lo pide para poder
# ejercitar también el camino de elegir el jar desde el disco.
sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" \
  APK_EJEMPLO="$PREFIX/tmp/sign-probe/app/build/app.apk" \
  KS_EJEMPLO="$HOME/.android/debug.keystore" \
  ANDROID_JAR="$HOME/Android/platforms/android-34/android.jar" node prueba-pestana.js'
```

Qué hace de verdad, y qué no:

- **hace**: `apkc create`, editar el fuente (con resaltado propio, sin dependencias), conservarlo
  entre sesiones en OPFS, `apkc sign` — una apk firmada v1+v2 que `apksigner`, `zipalign -c -p`,
  `jarsigner` y `zipcheck.py` aceptan por fuera — y **compilar el proyecto entero** con
  `build --linker go --compiler go`: enlazar los recursos, sacar el `.class`, dexar y firmar, todo dentro
  de la pestaña y sin levantar un proceso. «Compilar debug» no pide keystore: la primera vez
  genera `/sdk/debug.keystore` con `apkc debug-keystore` (alias `androiddebugkey`, contraseña
  `android`, `CN=Android Debug`) y reutiliza esa identidad; «Compilar release» sigue usando el
  keystore elegido. El keystore y el `android.jar` no salen de la pestaña:
  con `servidor.js`, el jar se toma del SDK local y se sirve en `/android-jar`; en una página
  estática, la pestaña Compilar de la consola pide aceptar los términos del SDK y enlaza al paquete oficial de
  Google, y el editor extrae `android-34/android.jar` del `.zip` (o acepta el `.jar` suelto). Esa
  descarga es una navegación que pulsa la persona, no una petición de la página: la pestaña sigue
  sin pedir nada fuera de su origen. En
  ambos casos se guarda una vez en OPFS y sobrevive a recargar. `tools/entorno.sh jar` instala
  `platforms;android-34` con el `sdkmanager` oficial si falta. La salida de la consola traduce los
  colores ANSI de `apkc` (nada de `\u001b[34m` crudo) y la barra de descargas deja a la vista el
  `app.apk`, su `.idsig` y la apk firmada; el fichero abierto se descarga desde el menú **Archivo**
  y cada fila del explorador tiene menú de mantener pulsado (abrir, descargar, renombrar, borrar;
  en una carpeta, además, nuevo fichero y subir archivos o una carpeta entera).
  La consola usa la paleta de `~/.termux/colors.properties` y un velo de carga para las
  operaciones lentas de OPFS. Medido en `docs/evidencia.md` §45 y
  §46: el apk que sale es **igual entrada por entrada** al del mismo comando en el binario nativo, y `d8`
  releee el dex que escribió el navegador.
- **hace también**: compilar un fuente que la página **no** generó. Campos con sus modificadores, variables
  locales con cast, cuentas enteras, el `if` con su `else` y la concatenación de cadenas
  (`docs/evidencia.md` §49 a §53), medidos contra `javap -c -l` de javac 21 línea por línea y contra las
  palabras de `d8`. Y la **app del contador** (§54): un botón con `@+id`, un `OnClickListener` anónimo que
  escribe el campo de la Activity, decide con `if` y arma el texto que muestra — compilada, firmada y
  dictaminada dentro de la pestaña. El editor guarda, compila y descarga ese apk.
- **no hace**: el `build` de siempre. En wasm `js` no puede crear tuberías, así que `aapt2`/`javac`/`d8` no
  arrancan; el botón «Probar el build clásico» lo demuestra y apkc lo dice con su texto:
  `[build] aapt2 compile never started: pipe: not implemented on js`. Y dentro del `--compiler go` el
  fuente tiene frontera: sin bucles, sin arreglos, sin `switch` ni `try` — lo que no está medido se rechaza
  con su línea, no se compila a medias.
- **dictamina**: `apkc verify build/app.apk` vuelve a leer el apk desde OPFS y comprueba las dos firmas con
  lectores propios —digest de cada entrada, `.SF` sección a sección, PKCS#7 y bloque v2—, sin `apksigner` y
  sin conexión. El botón `verificar el apk` lo corre y el veredicto se lee en la página. Cotejado con
  `apksigner` en ocho casos, mismo veredicto y misma salida (`docs/evidencia.md` §47); la cadena de
  confianza no se valida porque no hay ancla, y lo no medido (DSA, SHA-1, v3, v4, varios firmantes) se
  rechaza por nombre.

| Fichero | Qué es |
| --- | --- |
| `servidor.js` | estáticos + COOP/COEP, sirve `/apkc.wasm` (de `APKC_WASM`), `/wasm_exec.js` (del `GOROOT`), `/android-jar` y `/android-jar-info` (del `ANDROID_JAR` o del SDK local que detecta solo); ningún binario versionado |
| `main.js` | entrada ESM del sitio; importa el editor, las vistas y el diagnóstico |
| `app.js` | modelo puro del proyecto: plantillas, `buildProject`, `makeZip`, reglas y análisis de salida; sin DOM, con `export` para el navegador y node |
| `vistas.js` | qué vista se ve, reflejo del formulario y creación del proyecto en OPFS |
| `diagnostico.js` | el panel de diagnóstico de Inicio: pinta las reglas de `app.js` y ofrece el informe |
| `fs-fragmento.js` | el `fs` con forma de Node que exige `fs_js.go`, con la memoria delante y OPFS detrás |
| `hospedador.js` | worker de módulo: da `fs`/`path`/`process` a Go y ejecuta un comando por vez |
| `pestana.js` | editor, pestañas, árbol y botones de la sección; `resaltar()` es puro y se prueba en node |
| `historial.js` | undo tree por documento: deltas, checkpoints, poda e IndexedDB por ventana; sus funciones puras se prueban en node |
| `temas.js` | temas de interfaz y de terminal; solo cambia `data-tema` y variables CSS, sin dependencias |
| `offline.js` | registra el service worker cuando la página se sirve por http(s) |
| `sw.js` | service worker: cachea el shell y `apkc.wasm`/`wasm_exec.js`; no cachea jar, keystores ni OPFS |
| `EDITOR.md` | el detalle del editor: estructura, ids, atajos, pruebas y límites |
| `AGENTS.md` | lo que un agente necesita saber antes de tocar esta carpeta: comandos, contratos y trampas |
| `_headers` | COOP/COEP para publicar en un host estático que las respete |
| `package.json` | nombres de las tareas (`npm test`, `npm run e2e`); sin dependencias |
| `prueba-pestana.js` | chromium por CDP: crea, edita, recarga, firma, compila el proyecto entero con el jar del disco, dictamina el apk con `apkc verify` —también el alterado, el de un fuente escrito a mano y el de la app del contador— y comprueba más de 50 cosas |

## Varias ventanas y sin red

Dos pestañas del mismo origen comparten `/proyecto` en OPFS. Cada ventana tiene sus pestañas de
editor (sessionStorage), su undo tree (IndexedDB por ventana) y su worker. Al guardar, la ventana
toma un lock (`navigator.locks`), escribe una revisión y publica `guardado` por `BroadcastChannel`.
Las demás refrescan su espejo de OPFS y recargan el fichero si está limpio; si está sucio, avisan
y permiten **sobrescribir** al guardar (con confirmación si la revisión cambió). Las mutaciones de
fichero (crear, subir, renombrar, mover, borrar) se publican igual y refrescan el árbol.

`sw.js` cachea el shell y, si el host los sirve, `apkc.wasm` y `wasm_exec.js`. No cachea el `android.jar`
ni los keystores, y el proyecto no pasa por HTTP. Una recarga sin red conserva COOP/COEP porque las
respuestas cacheadas mantienen sus cabeceras. La primera visita debe ser online para instalar el
worker y llenar la caché.

## Publicar

La carpeta `studio/` es el sitio: no hay build. Para que OPFS y el worker funcionen, el host
tiene que mandar `Cross-Origin-Opener-Policy: same-origin` y
`Cross-Origin-Embedder-Policy: require-corp`; `_headers` ya las declara para Netlify y Cloudflare
Pages. **GitHub Pages no permite fijar cabeceras**, así que allí Inicio y el diagnóstico
funcionan pero el Editor no arranca.

Además:

- sirve `studio/` como **raíz del origen**: el worker pide `/apkc.wasm` y `/wasm_exec.js` con
  ruta absoluta. Si publicas bajo `/studio/`, cambia esas dos rutas en `hospedador.js`.
- copia `wasm_exec.js` desde `$(go env GOROOT)/lib/wasm/` junto a `index.html` y deja el
  `apkc.wasm` construido en `/apkc.wasm`; ninguno de los dos se versiona.
- el `android.jar` y los keystores no viajan con la página: salen del SDK local (mismo origen) o
  del selector de fichero, y el paquete oficial de Google es una navegación del usuario tras
  aceptar los términos.

## Abrir la pestaña en Brave Nightly

Con `servidor.js` escuchando (por ejemplo en `127.0.0.1:8787`), Brave Nightly se abre desde
Termux copiando primero el `BOOTCLASSPATH` de `zygote64` y lanzando el componente de la pestaña:

```sh
zygote=$(/usr/sbin/sudo sh -c 'pidof zygote64 2>/dev/null || pidof zygote 2>/dev/null' | tr -d '\r')
eval "$(/usr/sbin/sudo sh -c "tr '\0' '\n' < /proc/$zygote/environ" |
  grep -E '^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH)=' | sed 's/^/export /')"
am start --user 0 \
  -n com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity \
  -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'
```

El porqué de cada pieza y los dos fallos que no funcionan (`am` sin `BOOTCLASSPATH` y `cmd
activity`) están medidos en `docs/evidencia.md` §140.

`e2e.sh` incluye la pestaña cuando `APKC_WASM` apunta a un binario wasm; si no, lo avisa en vez de
darla por probada.

`e2e.sh` es el que importa: descomprime el zip de verdad, corre `apkc build` sobre él y exige que
`apksigner verify` (v1 y v2) y `zipcheck.py` del harness acepten el apk. Con `node prueba.js`
solo no basta — encontró seis fallos que el test propio veía verdes: contador del EOCD duplicado,
directorios en modo `000`, `versiónCode` con tilde, un bloque de Java metido dentro de `onCreate`,
`STUDIO_KOTLIN=0` que era verdad en JS (la variante "java" compilaba Kotlin) y un `LEEME.txt` en
`lib/arm64-v8a/` que acababa dentro del apk como si fuera una `.so`.

La variante kotlin necesita `kotlinc` y un `kotlin-stdlib*.jar` por `~/Android`; si no hay, `e2e.sh`
la omite y lo dice.

## Riesgo conocido

`apkc/template/*` puede cambiar y la página no se entera sola: `prueba.js` compara contra el repo
que esté en `APKC_REPO` (por defecto `~/Develop/Go/apkc`), así que la desincronización salta al
correr la prueba, no al usar la página.

## La piel

La página tiene un marco de editor ligero, sin dependencias: barra de título, barra de
actividad, explorador del proyecto en la barra lateral, pestañas y barra de estado. En Inicio
solo se piden nombre, package y, si quieres, un icono PNG; al crear, el proyecto se escribe en
OPFS y el manifiesto se abre en el editor, que ocupa el alto completo. El editor tiene números
de línea, resaltado propio, Tab/Shift+Tab para sangrar, Enter que conserva la sangría y una
consola inferior con Salida, Historial, Compilar y Firmar (cerrada hasta que hace falta;
``Ctrl + ` `` la abre). Cada fichero abierto es una **pestaña**; cambiar de pestaña no pierde
cambios y al recargar vuelven las pestañas, la activa y las carpetas abiertas del árbol. El
contenido se lee de OPFS solo para la pestaña que se ve: las demás se cargan al pulsarlas. Cada
documento tiene un **undo tree** con ramas: deshacer y escribir otra cosa no borra la rama
anterior; el historial se guarda en IndexedDB por ventana y documento, con deltas y checkpoints.
La barra del fichero deja **Guardar** a la vista y agrupa en **Archivo** lo demás (nuevo,
subir, descargar, renombrar, borrar, historial). La API de compilación se elige en Opciones
avanzadas (34 por defecto). El tema de interfaz (oscuro, Catppuccin, Rosé Pine, Tokyo Night,
Gruvbox) y la paleta de la terminal se eligen en **Tema**, en la barra de estado. El árbol arranca
**cerrado** y la cabecera del explorador
tiene botones para contraerlo o desplegarlo entero. El botón de subir escribe en la carpeta del
fichero abierto; el menú de una carpeta permite subir ahí o una carpeta entera, conservando sus
subdirectorios. Para mover, se arrastra una fila a otra carpeta con ratón o lápiz, o se usa
**Mover a…** en el menú de mantener pulsado (la vía fiable en táctil). Todo va directo a OPFS:
ningún byte sale del navegador.
`pestana.js` lleva el editor, pestañas, árbol y apkc.wasm; `historial.js` el undo tree; `temas.js`
aplica la piel; `vistas.js` siembra el árbol, cambia de vista (también por hash,
`index.html#pestana`) y refleja rótulos; `diagnostico.js` explica la salida pegada. Las pruebas de navegador, de creación y de la pestaña usan los mismos ids de siempre. El
detalle de la estructura, los ids, los atajos y los límites del editor está en
`studio/EDITOR.md`.
