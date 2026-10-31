# apkc studio

Una página para crear proyectos de [apkc](../apkc) en el navegador y medir dónde se atasca
quien los usa. Sin dependencias: abre `index.html` con `file://` y funciona.

Qué hace, en el orden en que duele:

1. **Genera el proyecto** (manifest, `MainActivity.java` o `.kt`, layout, styles, carpetas) y lo
   baja como zip. Las plantillas son copia literal de `apkc/template/*`; `prueba.js` compara los
   atributos que apkc escribe con los del repo, porque una tilde dentro de `android:versionCode`
   rompe el XML y `aapt2` lo rechaza en el teléfono.
2. **Dice qué comandos correr**, en el orden en que un desconocido los necesita.
3. **Traduce la salida real de apkc**: 24 reglas, cada una con `lit:` (la cadena que la motivó) y
   `de:` (quién la imprime). Las de apkc se verifican contra el fuente en la propia prueba; una
   regla inventada explicaría un error que nadie imprime, que es peor que no tenerla.
4. **Lista las líneas que no conoce** y lleva contadores de dónde se rompió cada intento. Eso es
   la medida de fricción; el informe en markdown es para traerlo a quien mantiene la herramienta.
   Los logs pegados no salen del navegador: se quedan en `localStorage`.

## Probar

```sh
node prueba.js                 # lo que genera: ascii, plantillas contra el repo, EOCD, reglas
node prueba-navegador.js       # chromium headless por CDP: DOM, clicks, descarga, localStorage
APKC=/ruta/apkc sh e2e.sh      # todo lo anterior + compilar y verificar los dos proyectos
```

## La pestaña: apkc.wasm corriendo aquí dentro

`index.html` tiene una sección que **sí ejecuta apkc en el navegador**, con los ficheros detrás de
OPFS y el worker como hospedador. No hay backend: `servidor.js` solo sirve estáticos con las
cabeceras COOP/COEP que OPFS exige (por `file://` no arranca ni el worker de módulo).

```sh
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" node servidor.js'   # http://127.0.0.1:8787/
# y la prueba que lo conduce todo, con una apk, un keystore y el android.jar del API 34 de ejemplo:
sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" \
  APK_EJEMPLO="$PREFIX/tmp/sign-probe/app/build/app.apk" \
  KS_EJEMPLO="$HOME/.android/debug.keystore" \
  ANDROID_JAR="$HOME/Android/platforms/android-34/android.jar" node prueba-pestana.js'
# sin ANDROID_JAR la prueba lo avisa y no comprueba el build completo (queda en crear/editar/recargar/firmar)
```

Qué hace de verdad, y qué no:

- **hace**: `apkc create`, editar el fuente (con resaltado propio, sin dependencias), conservarlo
  entre sesiones en OPFS, `apkc sign` — una apk firmada v1+v2 que `apksigner`, `zipalign -c -p`,
  `jarsigner` y `zipcheck.py` aceptan por fuera — y **compilar el proyecto entero** con
  `build --linker go --compiler go`: enlazar los recursos, sacar el `.class`, dexar y firmar, todo dentro
  de la pestaña y sin levantar un proceso. El keystore y el `android.jar` se eligen del disco y no salen de
  la pestaña; el jar se guarda en OPFS una vez y sobrevive a recargar. Medido en `docs/evidencia.md` §45 y
  §46: el apk que sale es **igual entrada por entrada** al del mismo comando en el binario nativo, y `d8`
  releee el dex que escribió el navegador.
- **hace también**: compilar un fuente que la página **no** generó. Campos con sus modificadores, variables
  locales con cast, cuentas enteras, el `if` con su `else` y la concatenación de cadenas
  (`docs/evidencia.md` §49 a §53), medidos contra `javap -c -l` de javac 21 línea por línea y contra las
  palabras de `d8`. El editor guarda, compila y descarga ese apk.
- **no hace**: el `build` de siempre. En wasm `js` no puede crear tuberías, así que `aapt2`/`javac`/`d8` no
  arrancan; el botón «intentar compilar con aapt2/javac/d8» lo demuestra y apkc lo dice con su texto:
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
| `servidor.js` | estáticos + COOP/COEP, sirve `/apkc.wasm` (de `APKC_WASM`), `/wasm_exec.js` (del `GOROOT`) y `/android-jar` (de `ANDROID_JAR`), solo para la prueba; ningún binario versionado |
| `fs-fragmento.js` | el `fs` con forma de Node que exige `fs_js.go`, con la memoria delante y OPFS detrás |
| `hospedador.js` | worker de módulo: da `fs`/`path`/`process` a Go y ejecuta un comando por vez |
| `pestana.js` | editor y botones de la sección; `resaltar()` es puro y se prueba en node |
| `prueba-pestana.js` | chromium por CDP: crea, edita, recarga, firma, compila el proyecto entero con el jar del disco, dictamina el apk con `apkc verify` —también el alterado y el de un fuente escrito a mano— y comprueba 39 cosas |

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
