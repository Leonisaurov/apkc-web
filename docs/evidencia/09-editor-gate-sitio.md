# Evidencia — §140-§182, §320

## 140. Abrir la pestaña en Brave Nightly desde Termux: el `BOOTCLASSPATH` que `am` no trae

Medido el 2026-11-09. La página se sirve en `127.0.0.1:8787` y el navegador que ya estaba
inicializado en el aparato es Brave Nightly (`com.brave.browser_nightly`). Brave a secas
(`com.brave.browser`) estaba en el primer arranque (`WelcomeOnboardingActivity`) y con una
petición de rol de navegador predeterminado encima: lanzarlo por el componente de URL abría esa
pantalla, no la página.

**Comando**

```sh
# 1. el servidor de la pestaña, con la página nueva
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8787/index.html

# 2. el entorno de Android que app_process no hereda de Termux: se copia de zygote64
zygote=$(/usr/sbin/sudo sh -c 'pidof zygote64 2>/dev/null || pidof zygote 2>/dev/null' | tr -d '\r')
eval "$(/usr/sbin/sudo sh -c "tr '\0' '\n' < /proc/$zygote/environ" |
  grep -E '^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH)=' | sed 's/^/export /')"

# 3. el intento explícito a Brave Nightly. Se va a ChromeTabbedActivity, no al
#    IntentDispatcher: el dispatcher es el que puede caer en el primer arranque.
am start --user 0 \
  -n com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity \
  -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'
```

**Resultado**: salida 0 y

```text
Starting: Intent { act=android.intent.action.VIEW dat=http://127.0.0.1:8787/...
  cmp=com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity }
```

Si la tarea de Nightly ya existía, la segunda vez responde
`Warning: Activity not started, its current task has been brought to the front`, pero el intento
`VIEW` llega igual y la pestaña abre la URL. La comprobación que vale es la pantalla, no esa
advertencia:

```sh
/usr/sbin/sudo dumpsys activity activities | grep -E 'topResumedActivity|mResumedActivity'
/usr/sbin/sudo screencap -p /data/local/tmp/brave.png
```

Con Nightly delante, el task de `com.brave.browser_nightly` tiene `ChromeTabbedActivity` y la
captura enseña la barra de direcciones en `127.0.0.1:8787/#pestana` con el explorador y el
editor. El lanzamiento corre con el uid de Termux; `sudo` solo se usa para leer el `environ` de
zygote, que Termux no puede leer.

**Los dos fallos anteriores, para no repetirlos**

1. `am start ...` a secas aborta con salida 134 y `logcat` dice
   `Check failed: !location.empty() BOOTCLASSPATH and DEX2OATBOOTCLASSPATH must not be empty`.
   El `am` de Termux ejecuta `/system/bin/app_process` sin el entorno de Android; el `BOOTCLASSPATH`
   de zygote64 es exactamente lo que falta (el mismo preludio que usa `banco/banco.sh` para
   `app_process64`).
2. `cmd activity start --user 0 -p com.brave.browser ...` falla con
   `SecurityException: Permission Denial: package=com.android.shell does not belong to uid=10237`
   porque `cmd` se identifica como `com.android.shell` y no como Termux. Sin `-p`, o con `sudo`,
   `cmd activity start` responde `cmd: Failure calling service activity: Failed transaction
   (2147483646)` en este aparato. La ruta que funciona es el `am` de Termux con el entorno copiado.

## 141. El scroll del editor: tres capas con scroll propio, no un `transform`

Medido el 2026-11-09 en chromium headless con la comprobación `3b` de
`studio/prueba-pestana.js`. La primera versión del editor movía `#resalto` y `#numeros` con
`transform: translate(0, -scrollTop)`. El `<pre>` lleva `overflow: hidden` y su caja mide el alto
visible: lo que queda fuera de esa caja no se pinta, y al trasladar la caja hacia arriba el
editor se veía vacío en cuanto se pasaba de la primera pantalla. Las tres capas tienen que ser
contenedores con scroll propio y copiarse `scrollTop` (y `scrollLeft` el resaltado).

**Comando**

```sh
tools/entorno.sh -- sh -c 'APKC_WASM=$TMP/apkc-wasm.wasm \
  APK_EJEMPLO=$TMP/studio-e2e-java/com.hola.apkcstudio/build/app.apk \
  KS_EJEMPLO=$HOME/.android/debug.keystore KS_RELEASE=$TMP/studio-release.p12 \
  AAB_EJEMPLO=$TMP/studio-ejemplo.aab \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar \
  node studio/prueba-pestana.js'
```

**Resultado**: `70 comprobaciones bien, 0 mal`. La comprobación `3b` pone 121 líneas (una de
ellas de 400 caracteres), baja el textarea 500 px y exige que `#codigo`, `#resalto` y `#numeros`
tengan el mismo `scrollTop` (`[500,500,500]`) y que el resaltado comparta además el
`scrollLeft` (`[80,80]`). La `3c` comprueba que Tab sin selección inserta en el cursor, que
Shift+Tab desindenta, y que `document.execCommand("undo")` revierte la edición: las ediciones
programáticas pasan por `execCommand("insertText")`, no por `setRangeText`.

## 142. El `android.jar` sale del SDK local o del paquete oficial de Google; el repo no lo versiona

Medido el 2026-11-09. El `android.jar` del API 34 pesa **26 361 808 bytes** y su licencia de SDK
no permite redistribuirlo como si fuera del repo, así que no se versiona. Hay dos caminos, y
ninguno mete el jar en el repositorio:

- **Servidor local (desarrollo):** `servidor.js` lo busca en `ANDROID_JAR`, `ANDROID_HOME`,
  `ANDROID_SDK_ROOT` y `~/Android` (de la plataforma más nueva a la más vieja), lo sirve en
  `/android-jar` y publica versión y tamaño en `/android-jar-info`. La página lo pide a **su propio
  origen** al compilar si no lo tiene en OPFS y lo guarda allí.
- **Página estática (producción):** no hay endpoint. El panel Compilar pide aceptar los términos
  del SDK y ofrece el enlace oficial al paquete `platform-34-ext7_r03.zip` de `dl.google.com`
  (~63 MB); el editor abre el `.zip`, saca `android-34/android.jar` y lo guarda en OPFS. También
  acepta el `.jar` suelto. El enlace no sale sin marcar la casilla de términos.

Si no hay SDK local, `tools/entorno.sh jar` instala `platforms;android-34` con el `sdkmanager`
oficial.

**Comando**

```sh
tools/entorno.sh -- sh -c 'cd studio && APKC_WASM=$TMP/apkc-wasm.wasm \
  node servidor.js 8788 >$TMP/jar-servidor.log 2>&1 & pid=$!; sleep 2; \
  curl -sS http://127.0.0.1:8788/android-jar-info; echo; \
  curl -sS -o $TMP/jar-servido.bin -w "jar: %{http_code} %{size_download} bytes\n" \
    http://127.0.0.1:8788/android-jar; \
  cmp $TMP/jar-servido.bin $HOME/Android/platforms/android-34/android.jar && echo idéntico; \
  kill $pid'
```

**Resultado**: `{"disponible":true,"version":34,"bytes":26361808}` y
`jar: 200 26361808 bytes`, `idéntico`. En `prueba-pestana.js`, la comprobación
`sin jar en OPFS, compilar lo trae solo del SDK local` borra `/sdk/android.jar`, llama a
`compilarDelTodo()` y exige que el jar vuelva a OPFS y que el apk salga con código 0; la
comprobación `el .zip oficial se descomprime y deja el android.jar` construye el zip con
`makeZip`, lo entrega por el selector y relee `/sdk/android.jar`; y
`la descarga oficial exige aceptar los términos y apunta a Google` comprueba el `aria-disabled`
del enlace. **70 comprobaciones bien, 0 mal**.

El paquete oficial real también se abrió una vez: `platform-34-ext7_r03.zip` mide
**63 180 081 bytes** (SHA-1 `1f2e9478d6a7601425ceaa553311dc43191f103d`, el que publica
`repository2-3.xml`), contiene `android-34/android.jar`, y `jarDeSeleccion` devolvió
**26 361 808 bytes idénticos byte a byte** al `android.jar` del SDK instalado
(SHA-256 `6cea1df3efb77103ac3e2beb9bf4718964b0e0869ab16d39d29d5cbae1c147ad`).

```sh
curl -fL -o /tmp/platform-34-ext7_r03.zip \
  https://dl.google.com/android/repository/platform-34-ext7_r03.zip
node -e 'const fs=require("fs"),crypto=require("crypto"),p=require("./studio/pestana.js");
p.jarDeSeleccion(new Uint8Array(fs.readFileSync("/tmp/platform-34-ext7_r03.zip")))
 .then(j=>console.log(j.length, crypto.createHash("sha256").update(j).digest("hex")))'
```

## 143. Compilar dos veces seguidas ya no deja huérfano `build/res`

Medido el 2026-11-09 sobre el mismo chromium y el mismo comando. La segunda compilación del
proyecto, sin borrar `build/`, fallaba con
`[build] open build/res/layout/main.xml: No such file or directory`. El `fs` del shim aceptaba
`unlink` sobre un directorio y borraba el nodo de `build/res` **sin sus hijos**; el `os.RemoveAll`
de Go daba por hecho el borrado y el segundo enlace dejaba el layout en un directorio que ya no
existía. Ahora `unlink` de un directorio responde `EISDIR` y la limpieza de OPFS de `unlink` y
`rmdir` es recursiva, de modo que memoria y disco no se separan.

**Resultado**: con la corrección, la comprobación de la §142 (que compila, borra el jar, vuelve a
traerlo y compila otra vez sin limpiar `build/`) termina con **70 comprobaciones bien, 0 mal**, y
el segundo `apkc build --linker go --compiler go` sale 0 con `build/res` presente
(`/proyecto/build/res:dir` en el árbol).

## 144. Compilar debug sin keystore: la clave típica se genera dentro del propio apkc

Medido el 2026-11-09. `apkc debug-keystore` escribe un JKS con alias `androiddebugkey`,
contraseña `android`, `CN=Android Debug,O=Android,C=US`, RSA 2048 y 10 000 días de validez: el
mismo perfil que el keystore de depuración de Android. No arranca procesos ni necesita SDK. El
sobre de la clave es el `KeyProtector` de JKS (OID `1.3.6.1.4.1.42.2.17.1.1`, sal SHA-1, XOR del
PKCS#8 y digest final de JKS), y el fichero lo leen tanto el lector de este fork como
`keytool` y `apksigner`.

La pestaña tiene **Compilar debug** y **Compilar release** separados. Debug, si no existe
`/sdk/debug.keystore`, llama a `apkc debug-keystore`, guarda el fichero en OPFS y firma con él;
release sigue pidiendo el keystore. El fichero se conserva entre recargas para que una apk debug
pueda actualizarse encima de la anterior.

**Comando**

```sh
tools/entorno.sh -- sh -c 'cd apkc && tcr -j 1 -q -- go build -o $TMP/apkc-debug .'
tools/entorno.sh -- sh -c 'cd $TMP && rm -rf debugks && mkdir debugks && cd debugks && \
  $TMP/apkc-debug debug-keystore --out debug.keystore && \
  $TMP/apkc-debug keystore --keystore debug.keystore --storepass android'
```

**Resultado**: `keystore-json` con `"alias":"androiddebugkey"`,
`"subject":"CN=Android Debug,O=Android,C=US"` y huella SHA-256; `keytool -list` abre el JKS y
`apksigner sign --ks debug.keystore --ks-pass pass:android --ks-key-alias androiddebugkey`
produce un apk que `apksigner verify` acepta con v1, v2 y v3 `true`, firmante
`CN=Android Debug`. En chromium, la comprobación
`compilar debug crea la clave típica y firma sin keystore de release` borra
`/sdk/keystore.p12` y `/sdk/debug.keystore`, deja vacía la contraseña de release, llama a
`compilarDebug()` y exige `/sdk/debug.keystore`, un apk de más de 5000 bytes, código 0 y
`certificado Android Debug` en la salida: **70 comprobaciones bien, 0 mal**.

## 145. La consola pinta ANSI y las descargas se ven sin abrir Compilar

Medido el 2026-11-09. `apkc` colorea su salida con secuencias ANSI; un `<pre>` las enseñaba
crudas (`\u001b[34m...`). `consolaHtml()` las traduce a spans (`consola-fg-4`, `consola-fuerte`,
…), descarta el resto de secuencias CSI y escapa el HTML. La prueba de node comprueba el
traductor sin DOM; la de chromium comprueba que después de compilar la consola tiene clases de
color y su `textContent` no contiene `\u001b`. La paleta sale de
`~/.termux/colors.properties` (Catppuccin Mocha): fondo `#1e1e2e`, texto `#cdd6f4` y sus 16
colores; `LS_COLORS` está vacío, así que no había una lista de colores de `ls` que copiar.

Además, los enlaces de descarga estaban dentro de Compilar, y `correr()` deja activa la pestaña
Salida: después de compilar no se veían. Ahora `#panel-descargas` vive encima de las vistas y
muestra `app.apk`, su `.idsig` y la apk firmada con nombre y tamaño; `#descargar-fichero`
descarga el buffer abierto desde la fila de pestañas, y cada fichero del explorador tiene su
`↓` (`.arbol-descargar`) para descargarlo sin abrirlo.

**Comando**

```sh
tools/entorno.sh -- node studio/prueba.js
tools/entorno.sh -- sh -c 'APKC_WASM=$TMP/apkc-wasm.wasm \
  APK_EJEMPLO=$TMP/studio-e2e-java/com.hola.apkcstudio/build/app.apk \
  KS_EJEMPLO=$HOME/.android/debug.keystore KS_RELEASE=$TMP/studio-release.p12 \
  AAB_EJEMPLO=$TMP/studio-ejemplo.aab \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar \
  node studio/prueba-pestana.js'
```

**Resultado**: `prueba: todo en orden` y `70 comprobaciones bien, 0 mal`. Las comprobaciones
`la descarga del apk se ve en la barra del panel` (`"app.apk (9252 bytes)"`),
`el clic en el enlace descarga de verdad el app.apk` (9252 bytes en el directorio de descargas
del navegador), `el explorador ofrece descargar cada fichero` y
`la consola pinta los colores ANSI y no enseña los códigos`
(`{"colores":true,"escape":false}`) son las que cubren este cambio.

## 146. Menú de mantener pulsado y velo de carga para OPFS

Medido el 2026-11-09. La fila del explorador abre `#menu-archivo` con el botón derecho o con una
pulsación larga (550 ms, se cancela si el dedo se mueve más de 8 px): **Abrir**, **Descargar**,
**Renombrar** y **Borrar**. Renombrar usa el mensaje `renombrar` del hospedador, que llama a
`fs.rename` sin copiar bytes; borrar usa `limpiar` y deja `Sin fichero` si el fichero abierto
era el borrado. Las operaciones lentas que no son compilar (leer un apk para el blob, extraer
`android.jar` de un `.zip`, abrir un fichero grande o bajar el jar del SDK) pasan por
`conCarga()`, que enseña `#cargando` solo si tardan más de 120 ms y lo retira en `finally`.

**Comando**: el mismo `prueba-pestana.js` de la §145.

**Resultado**: además de las comprobaciones anteriores,
`renombrar y borrar ficheros funciona en OPFS` (`{"tipo":"renombrado","texto":"hola","viejo":false,"borrado":false}`),
`el menú del fichero ofrece abrir, descargar, renombrar y borrar` y
`la pantalla de carga aparece en las operaciones lentas`
(`{"visible":true,"texto":"probando…","oculto":true}`) pasan: **70 comprobaciones bien, 0 mal**.

## 147. La clave de release se crea en la pestaña y la barra se puede dejar en iconos

Medido el 2026-11-10. Tres cambios de producto que no tocan el motor de formatos.

**1. `apkc create-keystore`.** El escritor JKS de `debug-keystore` toma un sujeto parametrizado
(alias, contraseña, CN, O y C) y un comando nuevo lo expone. No arranca procesos ni necesita SDK:
entra en la lista de comandos que `main.go` no manda a `findSDKs`, igual que `debug-keystore`.
Se niega a sobrescribir un fichero existente salvo `--force`, porque reemplazar la clave de
release deja las instalaciones anteriores sin poder actualizarse.

```sh
tools/entorno.sh -- sh -c 'cd apkc && go build -o $TMP/apkc-doc .'
tools/entorno.sh -- sh -c 'rm -rf $TMP/doc-ks && mkdir -p $TMP/doc-ks && cd $TMP/doc-ks && \
  $TMP/apkc-doc create-keystore --out release.jks --storepass doc-pass-123 \
    --keyalias apkc-doc --common-name Documentación --organization "apkc studio" --country ES && \
  $TMP/apkc-doc keystore --keystore release.jks --storepass doc-pass-123'
```

**Resultado**: `keystore-json` con
`"subject":"CN=Documentación,O=apkc studio,C=ES"`, `"notAfter":"2054-03-28"` y la huella
SHA-256; `keytool -list` abre el JKS y declara `PrivateKeyEntry` con esa misma huella. En el
hospedador de navegador, el mismo comando sobre el binario `GOOS=js` escribe el fichero sin
proceso:

```sh
tools/entorno.sh -- sh -c 'cd shim && node navegador-sim.js \
  --exportar "/release.jks:$TMP/doc-ks/wasm-release.jks" "$TMP/apkc-wasm.wasm" \
  create-keystore --out /release.jks --storepass wasm-pass-123 --keyalias wasm-doc \
    --common-name "Wasm Doc" --organization "apkc studio" --country ES'
```

**Resultado**: `[shim] exportado /release.jks … (2165 bytes)`, un JKS que el lector de este fork
vuelve a abrir con la contraseña y el alias elegidos.

**2. El diálogo de la pestaña.** `#crear-clave-release`, en **Firma release**, abre
`#dialogo-clave`: alias, CN, organización, país, contraseña y confirmación, más un aviso
obligatorio de que perder la clave impide actualizar la app. Al generar, la página corre
`create-keystore`, guarda `/sdk/release.keystore` (la ruta que antes era `/sdk/keystore.p12`),
descarga `<alias>.jks`, rellena la contraseña y el alias del formulario de compilación y pasa al
paso **Clave creada**, que no deja cerrar hasta marcar que se guardó una copia fuera del
navegador. La contraseña no se persiste. Si el fichero ya existe, `decirEstadoClave()` deja
`#detalle-release` plegado con `.jar-listo`, muestra `#clave-badge` y el botón pasa a
**Cambiar clave de release**, el mismo patrón que la sección del jar (comprobación
`con el jar cargado la sección se marca lista y no llama la atención`).

**Comando**: `prueba-pestana.js` con `APKC_WASM`, `APK_EJEMPLO`, `KS_EJEMPLO` y `ANDROID_JAR`
(los de `studio/README.md`) y, para las comprobaciones de release, `KS_RELEASE` y `AAB_EJEMPLO`.

**Resultado**: `se crea una clave de release y se ofrece para descargar` deja
`{"creada":true,"bytes":2190,"descarga":"clave-navegador.jks","blob":true,"seccionLista":true,"badge":true,"cerrarDeshabilitado":true}`,
`la clave creada deja la sección lista y exige confirmar que se guardó` exige los dos pasos, y
`la clave generada se puede inspeccionar con apkc` abre el fichero con `apkc keystore` y lee el
alias `clave-navegador`: **77 comprobaciones bien, 0 mal**.

**3. La barra.** El botón `#alternar-panel` pasa a llamarse **Compilar** y abre el panel en esa
pestaña; la pestaña de salida pasa de **Salida** a **Consola** con el icono de terminal. El botón
del build clásico se retira: en wasm no puede funcionar y ya estaba medido (§4), así que la
prueba baja de 78 a 77 comprobaciones. **Tema → Solo iconos en la barra del editor** pone
`data-botones="iconos"` en `<html>`, oculta `.texto-boton` y guarda la preferencia en
`localStorage`; `title` y `aria-label` se quedan para que el botón siga teniendo nombre accesible.

**Comando**: `tools/entorno.sh -- node studio/prueba-navegador.js`.

**Resultado**: la comprobación nueva exige `{"botones":"iconos","oculto":"none"}` al marcar la
casilla y `navegador: todo en orden`; al desmarcarla, el texto vuelve. El resto del gate no
cambia: `e2e: todo en orden`.


## 148. Propiedad de los datos, ajustes y una consola que se puede seguir

Medido el 2026-11-10. Cierra las fases A1, A2, A3, A4, B1, D1-D4, H1 y H2 del plan de producto, y
deja escrito por qué B2 no se puede hacer en la pestaña.

**1. Exportar e importar el proyecto.** `studio/proyecto.js` escribe el zip con el mismo `makeZip`
de `app.js` (que ahora acepta `Uint8Array`) y lo vuelve a leer del directorio central: admite STORE
y DEFLATE (`DecompressionStream("deflate-raw")`), comprueba CRC y tamaño, y rechaza rutas
absolutas, `..`, `\`, entradas cifradas y ZIP64. Al importar, `/proyecto` se reemplaza solo después
de la confirmación.

**Comando**: `APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm node studio/prueba-crear.js` (el mismo de la
§145, con `HOME`/`PREFIX` del repo).

**Resultado**: `exportar el proyecto produce un zip con manifiesto y binarios` con
`{"entradas":21,"png":9,"antes":{"nota":false,"png":false}}`; el PNG de prueba sale byte a byte
(`137,80,78,71,0,255,10,20,30`) y al importar el zip vuelve `{"nota":"hola desde el
dispositivo\n","png":[...],"aviso":"proyecto importado: 8 ficheros"}`. `prueba.js` cubre además un
zip con una entrada DEFLATE construida con `CompressionStream`, y que `../fuera.txt`, `/abs.txt` y
`a\b.txt` se rechazan. **33 comprobaciones bien, 0 mal** en la prueba de creación.

**2. Ajustes en el manifiesto.** `conAjustes()` escribe `versionCode`, `versionName`,
`minSdkVersion` y `targetSdkVersion` en el XML, añade `<uses-sdk>` si falta y quita
`maxSdkVersion`; la plantilla nueva ya no lo escribe. `prueba.js` comprueba los cuatro valores, la
inserción del bloque, que no se duplica y que se rechazan `targetSdk < minSdk` y un `versionName`
con `<`. En chromium, `leerAjustes()`/`guardarAjustes()` dejan
`{"antes":{"min":"21","target":"34","code":"1","nombre":"0.1"},"leidos":{"versionCode":"7","versionName":"1.0","minSdk":"24","targetSdk":"35"}}`.

**3. El aviso de almacenamiento.** `#aviso-almacenamiento`, en Inicio, dice que el proyecto vive
en OPFS de ese origen y remite a **Archivo → Exportar proyecto (.zip)**. `prueba-navegador.js`
exige que el texto nombre OPFS y la exportación; `navegador: todo en orden`.

**4. La consola.** Go escribe en el fd 1 del fs, no en `process.stdout`: `hospedador.js` envuelve
`consola.escribir` y manda `{tipo:"salida"}`. `correr()` pinta cada trozo, usa la última línea como
fase en `#ocupado` y **Detener** (`hospedador.cancelar`) termina el worker y lo recrea en el
siguiente comando. `enlazarSalidas()` convierte `ruta:línea` en un enlace que abre el fichero y
selecciona la línea.

**Resultado**: `la salida de apkc llega por trozos antes de terminar` con `{"trozos":3,...}`;
`un enlace ruta:línea de la consola abre el fichero en esa línea` deja
`{"activo":"src/com/myapp/MainActivity.java","linea":3,"seleccion":"import android.app.*;"}`;
`Detener termina el worker y el siguiente comando arranca otro` con
`{"primero":true,"codigo":0}`; y `la barra de trabajo enseña la fase que imprime apkc` con
`{"texto":"apkc: [apkc] finished in 38.595072ms","oculto":true}`. **81 comprobaciones bien, 0 mal**.

**5. El árbol.** `#filtro-arbol` conserva la cadena de padres y `Ctrl+P` lo enfoca; `build/` lleva
la clase `generado`, el color atenuado y la etiqueta. Medido:
`{"clase":"dir generado","texto":"buildgenerado"}` y el filtro por `MainActivity` deja
`["/proyecto/src","/proyecto/src/com","/proyecto/src/creada","/proyecto/src/com/creada/app","/proyecto/src/com/creada/app/MainActivity.java"]`.

**6. La clave de depuración también se descarga.** Al generarla con `apkc debug-keystore`, el botón
`#descargar-debug-keystore` aparece y baja el JKS: `{"alias":"androiddebugkey","bytes":2173,"visible":true}`.

**7. B2 no se puede hacer en la pestaña.** `--compiler go` rechaza los bundles con su propio
texto:

```sh
cd "$PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio" && \
  "$PREFIX/tmp/apkc-gate" build --linker go --compiler go --aab \
    --android-jar "$HOME/Android/platforms/android-34/android.jar" \
    --sdk-version 34 --codename 14 --keystore "$HOME/.android/debug.keystore"
# [build] --compiler go no escribe bundles: d8 es quien produce el .aar de las dependencias
```

Firmar un `.aab` que ya existe sí funciona (§128 y la comprobación `la pestaña firma un .aab con el
keystore de release`, 2321 bytes); ensamblarlo no. La casilla del bundle se retiró de la interfaz
para no ofrecer un botón que no puede funcionar, igual que se hizo con el build clásico.

**8. CI.** `studio/e2e.sh` ya no depende de `tcr`: usa `TCR` si está, o `tcr` del PATH, o ejecuta
directo. El navegador sale de `CHROME_BIN` y, si falta, se busca entre `chromium-browser`,
`chromium`, `google-chrome` y `google-chrome-stable`; los tres `prueba-*.js` leen `CHROME_BIN`.
`.github/workflows/gate.yml` ejecuta el job rápido en cada push y el pesado (SDK 34, build-tools
34.0.0, Chrome) con `e2e.sh`, y sube los logs si falla. **No se ha ejecutado todavía en GitHub**:
eso queda como la frontera de esta fase, no como una medición.

## 149. Plantillas, buscar/reemplazar, aviso de targetSdk y el gate que ya se paga solo

Medido el 2026-11-10. Cierra H3, J (la parte de buscar/reemplazar e ir a línea), B3 y deja E/12.5
en «hecho en parte» con la frontera escrita.

**1. Plantilla Contador.** `#plantilla` elige `hola` o `contador`; la segunda sustituye el layout
(`@+id/total`, `@+id/pulsar`) y la actividad por una con campo `TextView total`, listener anónimo
sobre un campo del exterior y `setText("Pulsos: " + pulsos)`. `prueba.js` comprueba el fuente y el
layout, `prueba-crear.js` la crea desde el formulario y `e2e.sh` la compila de verdad:

```sh
tools/entorno.sh -- sh -c 'cd studio && APKC=$TMP/apkc-gate APKC_WASM=$TMP/apkc-wasm.wasm \
  ANDROID_HOME=$HOME/Android CHROME_BIN=chromium-browser sh e2e.sh'
```

**Resultado**: `plantilla Contador: apk de 10086 bytes, firmada y alineada`, y el `e2e` entero
`e2e: todo en orden`. En chromium, `crear: todo en orden (35 comprobaciones)`.

**2. Buscar y reemplazar.** `#buscar-barra` es del editor: `Ctrl+F`, `Ctrl+H`, `Ctrl+G`, `Enter`,
`Shift+Enter` y `Esc`. Reemplazar pasa por `EDITOR.cambiar()`, así que entra en el undo tree. La
prueba destapó un defecto real: `execCommand("insertText")` actúa sobre el elemento con foco y la
barra se lo quitaba al textarea, de modo que el reemplazo decía «3 reemplazadas» sin cambiar nada;
`cambiar()` ahora enfoca el área antes. **Resultado**:
`{"primero":true,"seleccion":"uno","texto":"cero dos cero\ndos cero\n","aviso":"3 reemplazadas"}`
y `Ctrl+G salta a la línea pedida — {"linea":5}`. **83 comprobaciones bien, 0 mal**.

**3. Aviso de targetSdk.** Guardar un `targetSdk` menor que 34 deja
`guardado en AndroidManifest.xml · targetSdk 30 está por detrás de la política actual de Google
Play` en `#estado-ajustes`, y el manifiesto lleva `android:targetSdkVersion="30"`. El selector de
API sigue ofreciendo 27-35 con 34 por defecto.

**4. El gate ya se paga solo.** `e2e.sh` generaba el proyecto Java de la pestaña solo si ya existía
de una sesión anterior: en un runner limpio esa sección se omitía en silencio. Ahora `sembrar_proyecto`
usa el propio `buildProject` y `compilar_fixture` lo compila con `--linker go --compiler go`; lo
mismo con el Contador. La prueba imprime las dos compilaciones en el log del gate.

**5. Publicación.** `.github/workflows/gate.yml` añade un job `publicar` con `needs: pesado` y
`if: startsWith(github.ref, 'refs/tags/')`: si el gate falla, no hay release; si pasa, el sitio
(copiado de `studio/` con `apkc.wasm` y `wasm_exec.js`) viaja como `apkc-studio.zip` en la release
de GitHub. El host definitivo sigue sin decidirse; esto no lo elige.

## 150. Tamaño del editor, iconos por tipo y `consola.js` fuera del orquestador

Medido el 2026-11-10. Cierra la parte de J que faltaba del editor y avanza G.

`#fuente-editor` escribe `--ed-fuente` (12, 13, 14 o 16 px) y lo persiste en
`apkc-studio/fuente-editor`; las tres capas del editor leen esa variable. `prueba-navegador.js`
elige 16 px y exige `getComputedStyle(#codigo).fontSize === "16px"`; el resto de la suite sigue en
`navegador: todo en orden`. Las filas del árbol llevan `tipo-java`, `tipo-kt`, `tipo-xml` y
`tipo-png` para colorear el icono; `prueba-crear.js` exige `tipo-java` en `MainActivity.java`
dentro de la misma comprobación del filtro.

La consola (ANSI, enlaces `ruta:línea`, buffer y velo de carga) salió de `pestana.js` a
`consola.js`: 106 líneas con su propio `export`, y `pestana.js` las importa. El orquestador sigue
grande; la extracción se hizo sin tocar comportamiento y el gate entero pasó:

```sh
tools/entorno.sh -- sh -c 'cd studio && APKC=$TMP/apkc-gate APKC_WASM=$TMP/apkc-wasm.wasm \
  ANDROID_HOME=$HOME/Android CHROME_BIN=chromium-browser sh e2e.sh'
```

**Resultado**: `crear: todo en orden (35 comprobaciones)`, `pestaña: 83 comprobaciones` y
`e2e: todo en orden`, sin ninguna línea `FALLA`.

## 151. G cerrada: el gate entero en verde y la segunda pestaña sin carreras

Medido el 2026-11-11. Cierra G (desmontar `pestana.js`) por su criterio de verificación: los
módulos separados —`consola.js`, `editor.js`, `busqueda.js`, `ajustes.js`, `historial.js`,
`fusion.js`, `temas.js`, `offline.js` y `proyecto.js`— quedan en el árbol y el gate completo pasa
dos veces seguidas. El orquestador sigue siendo `pestana.js`; lo que cambia en este párrafo es que
la verificación que faltaba ya no es una promesa.

```sh
tools/entorno.sh gate
```

**Resultado de las dos pasadas** (`e2e-h.log` y `e2e-j.log`):

```text
crear: todo en orden (35 comprobaciones)
pestaña: 83 comprobaciones, apk firmada dentro de chromium de 10324 bytes y verificada
e2e: todo en orden
```

Antes de esas dos pasadas el mismo gate falló dos veces con la pestaña omitida a medias: la
segunda ventana de `prueba-crear.js` abría pero no leía `res/nota.txt`. Eran dos defectos reales,
no ruido del runner:

1. **La prueba no esperaba de verdad.** `abrirSegundaPagina()` hacía sesenta intentos y, si la
   pestaña no llegaba a cargar los módulos, seguía igual; las comprobaciones fallaban con textos
   vacíos. Ahora espera hasta 30 s, cierra y reintenta una vez, y si no carga falla con el log de
   Chrome en el mensaje. Además la comprobación devuelve `{abierta, texto}` en vez de solo el
   texto, para distinguir «no abrió» de «abrió vacío».
2. **La lectura de OPFS no toleraba el acceso exclusivo de otra pestaña.**
   `createSyncAccessHandle()` es exclusivo por fichero; si el worker de la primera pestaña lo tenía
   abierto en ese instante, `extraer()` lanzaba y la pestaña nueva contestaba «no existe». Ahora,
   si el acceso exclusivo falla, se lee con `getFile()` —que no lo pide y da una copia
   consistente—, y `asegurarAcceso()` rellena la memoria por la misma vía. El arreglo está en
   `studio/fs-fragmento.js`; la espera y el reintento, en `studio/prueba-crear.js`.

Los dos cambios están commiteados en `apkc-studio` (`fa66b98` y `94c8905`) y el puntero del
submódulo se actualiza en el mismo commit que este párrafo.

## 152. E cerrada: el gate pesado pasa en Ubuntu y el release espera al gate

Medido el 2026-11-11. Cierra E (CI portable). El mismo `studio/e2e.sh` que corre en Termux pasó
en un runner Linux limpio, con el SDK de Google, build-tools 34.0.0 y Chrome de escritorio; el
job rápido no descarga el SDK, el pesado lo instala una vez y lo cachea, y el job de release
depende del pesado (`needs: pesado`), así que un gate rojo no publica.

```sh
gh run view 42965693455 --log | grep -E 'crear: todo|pestaña: |e2e: todo'
gh run view 42966004867 --log | grep -E 'Cache restored|crear: todo|pestaña: |e2e: todo'
```

**Resultado del run `42965693455`** (`pesado: success`):

```text
crear: todo en orden (35 comprobaciones)
pestaña: 83 comprobaciones, apk firmada dentro de chromium de 10262 bytes y verificada
e2e: todo en orden
```

**Resultado del run `42966004867`**, ya con la caché del SDK (`pesado: success`):

```text
Cache restored from key: android-sdk-34.0.0-v1
crear: todo en orden (35 comprobaciones)
pestaña: 83 comprobaciones, apk firmada dentro de chromium de 10262 bytes y verificada
e2e: todo en orden
```

El primer intento del CI, con JDK 21, falló con un muro real que no aparece en Termux:
`d8` de build-tools 34.0.0 aborta con `Unsupported class file major version 65` porque javac 21
escribe la versión 65 y ese `d8` solo lee hasta la 61. El `d8` de las build-tools 34.9.0 del
teléfono sí la lee; el CI usa las de Google, así que el workflow fija **JDK 17** para el camino
nativo (`java-version: "17"`). El camino de la pestaña no cambia: `--compiler go` no usa javac.

**Frontera del runner, dicha y no disimulada**: en Ubuntu no hay `zipcheck.py`, ni `bundletool`,
ni `kotlin-stdlib`; `e2e.sh` los omite con aviso (`sin zipcheck.py … se omite ese oráculo`,
`sin bundletool …`, `kotlin: sin kotlin-stdlib … se omite`) y el gate sigue verde por las demás
comprobaciones. Esos tres oráculos sí corren en Termux, donde están instalados; el CI no los
sustituye. El artefacto `gate-logs` del run fallido `42754896276` se subió igualmente, que es lo
que pide E4 para diagnosticar un rojo.

El job `publicar` quedó **skipped** en los dos runs porque no había tag; su condición es
`needs: pesado` y `if: startsWith(github.ref, 'refs/tags/')`, de modo que la release no puede
adelantar al gate.

## 153. Surge no fija cabeceras: las pone el service worker y la página se recarga una vez

Medido el 2026-11-21. La documentación de Surge (`surge.sh/docs/platform/` y su guía de migración
desde Netlify) no ofrece un fichero de cabeceras: los ficheros de configuración son `CNAME`,
`.surgeignore`, `200.html`, `404.html`, `ROUTER`, `AUTH` y `CORS`, y «Custom headers» se manda a
*Lucid Caching*. `CORS` solo escribe `Access-Control-Allow-Origin`. Por tanto el `_headers` del
paquete es inerte allí, y sin COOP/COEP `studio/hospedador.js` lanza `la página no es
crossOriginIsolated`: no hay `createSyncAccessHandle` y el `fs` que exige Go no arranca.

```sh
curl -sI https://apkc-studio.surge.sh/ | grep -i cross-origin     # no devuelve nada
grep -n 'Cross-Origin' studio/sw.js                                # las pone el SW, no el host
```

La solución vive en el propio origen y no añade dependencia: `studio/sw.js` envuelve **todas** las
respuestas del mismo origen (navegación, módulos y las que salen de `Cache`) con
`Cross-Origin-Opener-Policy: same-origin` y `Cross-Origin-Embedder-Policy: require-corp`;
`index.html`, antes de los módulos, registra ese SW con `updateViaCache: "none"` y, si no hay
aislamiento, espera `navigator.serviceWorker.ready` y recarga una sola vez (`sessionStorage`
impide el bucle). La versión del SW se inyecta en el empaquetado desde el hash del sitio.

```sh
SITIO_DIR=sitio SIN_CABECERAS=1 PREFIX=$HOME/.cache/apkc-web CHROME_BIN=/usr/bin/chromium \
  APKC_WASM=$PWD/sitio/apkc.wasm APK_EJEMPLO=<fixture> KS_EJEMPLO=<keystore> \
  node studio/prueba-pestana.js
SITIO_DIR=sitio PREFIX=$HOME/.cache/apkc-web CHROME_BIN=/usr/bin/chromium \
  ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  APKSIGNER=$HOME/.cache/apkc-web/tmp/apkc-sdk34/build-tools/34.0.0/apksigner \
  node studio/prueba-sitio.js
```

**Resultado**: `84 comprobaciones bien, 0 mal` con un servidor que no envía COOP/COEP pero sí
tiene las rutas de prueba, y `8 comprobaciones bien, 0 mal` contra el paquete empaquetado servido
por `studio/servidor-estatico.js`, sin cabeceras y sin endpoints de desarrollo. En las dos el
estado es `{"aislada":true,"controlado":true,"recarga":"1"}` y la apk resultante (9217 bytes)
verifica v1, v2 y v3 con `apksigner`. La recarga es automática y no hay peticiones fuera del
origen. Nota de CLI: `surge .` con esta versión (0.52.4) responde `surge: nothing to do - try
'surge . publish'`; con `CNAME` en el directorio basta `surge . publish`.

## 154. El gate completo en un escritorio Linux, sin Termux

Medido el 2026-11-21 en Arch x86_64. Los guiones tenían shebang y `PREFIX`/`TMP` de Termux:
ahora `#!/bin/sh`, y si `/data/data/com.termux/files/usr` no existe se usa
`$XDG_CACHE_HOME/apkc-web`. `apkc` toma la build-tools **más alta** (§ AGENTS) y de ahí deriva la
platform: con 36.0.0 instalada pedía android-36, así que `tools/sdk34.sh` expone un SDK espejo
con solo `build-tools/34.0.0` y `platforms/android-34` mediante enlaces, y el gate lo usa. El
camino nativo usa **JDK 17** (`~/opt/jdk17`), por el mismo motivo del §152.

```sh
sh tools/entorno.sh doctor
sh tools/entorno.sh test
sh tools/entorno.sh wasm
sh tools/entorno.sh gate
```

**Resultado**: `doctor` sin `FALTA` (solo avisa de que `zipcheck.py` es opcional); `test rc=0`
(`ok github.com/ajinasokan/apkc 1.987s`); `wasm rc=0` con `PASS`; `gate rc=0` con
`crear: todo en orden (35 comprobaciones)`, `pestaña: 83 comprobaciones` y `e2e: todo en orden`.
El oráculo `zipcheck.py` se omite con aviso, igual que en el CI del §152; los demás (`apksigner`,
`zipalign`, `aapt2 dump`, `d8` y el navegador) corren.

## 155. Publicado en apkc-studio.surge.sh y verificado desde el host

Medido el 2026-11-21. El directorio `sitio/` lo genera `tools/entorno.sh sitio` (27 ficheros,
3,55 MB de zip, versión `97d25afb595e`) y se sube desde dentro con `surge . publish`.

```sh
cd sitio && surge . publish
curl -sI https://apkc-studio.surge.sh/ | grep -iE 'HTTP/|content-type'
curl -sI https://apkc-studio.surge.sh/apkc.wasm | grep -i content-type
curl -s https://apkc-studio.surge.sh/apkc.wasm | sha256sum
sha256sum sitio/apkc.wasm
surge apkc-studio.surge.sh debug status
SITIO_URL=https://apkc-studio.surge.sh/ PREFIX=$HOME/.cache/apkc-web CHROME_BIN=/usr/bin/chromium \
  ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  APKSIGNER=$HOME/.cache/apkc-web/tmp/apkc-sdk34/build-tools/34.0.0/apksigner \
  node studio/prueba-sitio.js
```

**Resultado**: `Success! - Published to apkc-studio.surge.sh`; `HTTP/2 200` y
`content-type: text/html; charset=UTF-8` en la raíz, `application/wasm` en el binario; los dos
`sha256` coinciden (`4533c3e5b073d57a878bcdf88fd4d343d3c39cc3e5920a37709ee7c3db443125`); el
dominio dice `live`; `/android-jar-info` devuelve 404, así que no hay endpoints de desarrollo en
el host. La prueba en el sitio real da **`9 comprobaciones bien, 0 mal`**: recarga única, SW
controlador, `crossOriginIsolated`, jar de 26361808 bytes entrado por el selector de fichero,
compilación y firma enteras en la pestaña, apk de 9225 bytes verificada por `apksigner` con
v1/v2/v3, y **36 peticiones, 0 fuera del origen**. Es el hecho que cierra la publicación: el sitio
no solo está empaquetado, está accesible y compila en un navegador ajeno a la máquina que lo
construyó.

## 156. `++`/`--` como valor: prefijo y postfijo, local y campo

Medido el 2026-11-21. Cierra la frontera que quedaba en la fila 5.1. La sonda `IncValor` tiene once
métodos: `++i`, `i++`, `--i`, `i--`, un `++i + i++`, y las cuatro formas sobre un campo de instancia
y otro estático. Oráculo: `javac 17 -g` y `javap -c -p` (§151 del plan de mejoras).

```text
prefijoLocal:    0: iinc 0,1 | 3: iload_0
postfijoLocal:   0: iload_0 | 1: iinc 0,1
prefijoCampo:    aload_0; dup; getfield; iconst_1; iadd; dup_x1; putfield
postfijoCampo:   aload_0; dup; getfield; dup_x1; iconst_1; iadd; putfield
prefijoEstatico: getstatic; iconst_1; iadd; dup; putstatic
postfijoEstatico: getstatic; dup; iconst_1; iadd; putstatic
```

En el campo de instancia **`dup_x1` va en los dos sentidos**: después de la cuenta en el prefijo y
antes en el postfijo. La primera versión puso un `dup` en el prefijo y el verificador lo dijo:
`pc 8 (putfield): se esperaba una referencia y hay entero`. `apkc/jvmframes.go` ya modelaba el
`dup_x1` del §72, así que el error sirvió de oráculo además del `javap`.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run TestElIncrementoComoValorSeDexaIgualQueJavac -count=1 -v .
sh tools/entorno.sh gate
# ejecución en la JVM delante del verificador, contra el javac del mismo fuente:
cd apkc && go run . compile --android-jar $ANDROID_HOME/platforms/android-34/android.jar \
  -o /tmp/opencode/proj/classes2 /tmp/opencode/proj/src /tmp/opencode/proj/jvm
java -Xverify:all -cp /tmp/opencode/proj/classes2 Main
```

**Resultado**: el test pasa y el **dex de nuestra clase es byte a byte el de javac** (las longitudes de
`Code` coinciden; los dos bytes del índice de `fieldref` difieren por el orden del pool y el dex los
resuelve igual). `java -Xverify:all` termina con rc 0 y su salida es idéntica a la de las clases de
javac en los once métodos. El gate sigue en verde (`e2e: todo en orden`). Frontera escrita: solo
entero de esta clase; un campo ajeno (`obj.campo++` con base explícita), un ancho (`long`/`double`) y
un delta mayor que un byte se rechazan por nombre. **ART queda pendiente de dispositivo**: en este
escritorio no hay `rish` ni `sudo` con `app_process64`, así que no se afirma lo que no se ha corrido
allí.

## 157. Asignaciones de desplazamiento y bits: `<<=`, `>>=`, `>>>=`, `&=`, `|=`, `^=`

Medido el 2026-11-21. Cierra la otra frontera de la fila 5.1. La sonda `Asig` las ejerce sobre un
local, un campo de instancia y un campo estático, con literal y con variable a la derecha. El orden de
precedencia de Java se rellenó con `|`, `^`, `&`, los desplazamientos y los comparadores, sin cambiar
el orden relativo de lo ya medido.

```text
javap 17:  a <<= 2 -> iload; iconst_2; ishl; istore
           campo &= b -> aload_0; dup; getfield; iload_1; iand; putfield
           s <<= 2 -> getstatic; iconst_2; ishl; putstatic
dexdump de d8 sobre la misma clase: and-int/2addr 0xb5, or-int/2addr 0xb6, xor-int/2addr 0xb7,
shl-int/2addr 0xb8, shr-int/2addr 0xb9, ushr-int/2addr 0xba; con literal, and/or/xor-int/lit8 0xdd/0xde/0xdf
y shl/shr/ushr-int/lit8 0xe0/0xe1/0xe2; el desplazamiento no conmutativo con el izquierdo vivo sale
en 23x (shl-int 0x98).
```

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run TestLasAsignacionesDeBitsSeDexanIgualQueJavac -count=1 -v .
sh tools/entorno.sh gate
```

**Resultado**: el test pasa con el **dex de nuestras clases byte a byte el de javac** y longitudes de
`Code` iguales método por método; `java -Xverify:all` sobre las clases que emite este fork da la misma
salida que javac (`9, 6, 3, 2, 0`) con rc 0; el gate sigue en verde. `jvmframes.go` y `jvmopcode.go`
necesitaron los seis opcodes nuevos: sin ellos el verificador propio decía `opcode outside the table
this fork measured`. Frontera: solo `int`; el ancho (`long`/`double`) y un desplazamiento con una
expresión a la derecha que no sea literal ni variable siguen rechazados por nombre. ART pendiente de
dispositivo, igual que en §156.

## 158. `protectionLevel` compuesto en `<permission>`

Medido el 2026-11-21. Cierra la frontera del §138. La sonda cambia `dangerous` por
`signature|privileged` en el mismo manifiesto y se enlaza con `aapt2 link --no-auto-version` y con
`--linker go` sobre el mismo `android.jar` de API 34.

```sh
BT=$ANDROID_HOME/build-tools/34.0.0
$BT/aapt2 dump xmltree /tmp/opencode/perm-comp/out4.apk --file AndroidManifest.xml | grep protectionLevel
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run 'TestPermisoCon' -count=1 -v .
```

**Resultado**: aapt2 escribe
`protectionLevel(0x01010009)=0x00000012` — la OR de `signature` (0x2) y `privileged` (0x10) — y
nuestro AXML sale **byte a byte el suyo** (1364 bytes). `TestPermisoConProtectionLevelCompuesto` y
el anterior `TestPermisoConSusAtributos` pasan, y el gate sigue en verde. El valor compuesto se
resuelve en `valorDeAtributo` con la OR de los ítems del framework y el tipo `0x11` que ya exigía el
formato `flags`; un nombre que el framework no conozca se rechaza con su línea. Frontera: el separador
es `|` sin expresiones más complejas, y solo se mide en atributos del manifiesto.

## 159. `nodpi`, `tvdpi` y `sw600dp`: los tres calificadores que faltaban

Medido el 2026-11-21. Cierra la frontera de la fila 5.17. Sonda: un PNG base, `drawable-nodpi-v4`,
`drawable-tvdpi-v4`, un layout base y `layout-sw600dp-v13`, enlazados con `aapt2 link
--no-auto-version` y con `--linker go`.

```text
aapt2 dump resources:  drawable/i -> () (tvdpi-v4) (nodpi-v4)
                       layout/main -> () (sw600dp-v13)
ResTable_config (64 bytes, volcado con el instrumento del §24):
  nodpi:    density +14 = 0xffff, sdkVersion +24 = 4
  tvdpi:    density +14 = 0x00d5 (213), sdkVersion +24 = 4
  sw600dp:  smallestScreenWidthDp +30 = 0x0258 (600), sdkVersion +24 = 13
spec: drawable 0x0500, layout 0x2400
```

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run 'TestNodpiTvdpiYSw600dp|TestDensidadesYOrientacionPort' -count=1 -v .
sh tools/entorno.sh gate
```

**Resultado**: los dos tests pasan y el `resources.arsc` de este fork sale **byte a byte** el de
aapt2 (1168 bytes); el gate sigue en verde. Detalle medido de aapt2: las build-tools 34.0.0 escriben
0 en el u16 de +10 del `RES_TABLE_TYPE_SPEC`, mientras que 35.0.1 y 36.0.0 escriben el número de
configuraciones (3 y 2 aquí), que es lo que ya hacía este escritor; el fixture se generó con 35.0.1
para comparar la misma convención. `quitaVersion()` separa el `-v4`/`-v13` de la ruta del
calificador, que sí va al pool de cadenas. Frontera: un tipo con solo configuraciones y **sin entrada
base** sigue sin enlazarse (el validador de ids de tipo espera que el primer tipo escrito tenga id 1);
la sonda y el proyecto de la página incluyen la base.

## 160. Los oráculos que faltaban: `zipcheck.py` y `bundletool`

Medido el 2026-11-21. `apkc-harness` ya está publicado, así que el gate deja de omitir el lector de
zip ajeno a apkc. `tools/entorno.sh harness` lo clona en `$PREFIX/apkc-harness` (sin versionarlo),
`doctor` lo nombra y `gate` lo usa. `e2e.sh` encuentra el `bundletool` de cualquier SDK por
`ANDROID_HOME`/`BUNDLETOOL_JAR`, no solo por `$HOME/Android` (§154).

```sh
sh tools/entorno.sh harness
sh tools/entorno.sh doctor | grep zipcheck
sh tools/entorno.sh gate
```

**Resultado**: `ok /home/leonisauro/.cache/apkc-web/apkc-harness/zipcheck.py`; el gate ya no imprime
«sin zipcheck.py» y todas las apks de la pestaña dicen `verificado por apksigner (v1+v2), zipalign,
zipcheck y aapt2`. El `.aab` dice `validado por bundletool y jarsigner`. El workflow de GitHub
clonea `apkc-harness` y descarga `bundletool-all-1.18.3.jar` antes del gate, con `APKC_HARNESS` y
`BUNDLETOOL_JAR` en el entorno del paso; la verificación del run queda para §163.

## 161. Firefox: el service worker también aísla fuera de Chromium

Medido el 2026-11-21. `prueba-navegador.js` no sirve para Firefox (sus flags son de CDP), así que
`studio/prueba-firefox.js` conduce Firefox por WebDriver con `geckodriver` contra el servidor
estático sin cabeceras. El `android.jar` se sirve en una ruta **solo de desarrollo** del mismo
origen para no meter 26 MB en base64 por el protocolo de WebDriver; el sitio publicado no la tiene.

```sh
GECKODRIVER=/ruta/geckodriver FIREFOX_BIN=/usr/bin/firefox SITIO_DIR=sitio \
  ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar node studio/prueba-firefox.js
```

**Resultado**: `4 comprobaciones bien, 0 mal`. Firefox se recarga sola, `navigator.serviceWorker.controller`
está puesto, `crossOriginIsolated === true`, el worker escribe y lee OPFS, el jar de 26361808 bytes
entra en OPFS y la compilación entera firma una apk de 9221 bytes con `v1: ok` y `v2: ok` en el
propio veredicto. Safari no está disponible en este equipo: queda como frontera escrita, no como
afirmación.

## 162. Sin destello en la primera carga

Medido el 2026-11-21. `index.html` añade `apkc-aislando` a `<html>` antes de registrar el SW y lo
quita en cuanto `crossOriginIsolated` es verdadero (o cuando el aviso de fallo tiene que verse);
`style.css` muestra un velo mientras tanto y oculta el cuerpo.

```sh
grep -n 'apkc-aislando' sitio/index.html sitio/style.css
sh tools/entorno.sh gate
```

**Resultado**: el velo está en los cuatro puntos de ida y vuelta del arranque y las pruebas siguen
en verde: `pestaña: 83 comprobaciones`, `navegador: todo en orden`, `e2e: todo en orden` y
`prueba-sitio.js` 9/9 sobre el host publicado. La recarga automática sigue ocurriendo; solo deja de
verse como un parpadeo.

## 163. Un solo comando para publicar y verificar

Medido el 2026-11-21. `tools/entorno.sh publicar` construye el wasm y `sitio/`, lee el dominio de
`.surge-domain`, ejecuta `surge . publish` desde `sitio/` y corre `prueba-sitio.js` contra la URL
resultante con los mismos oráculos del gate. El hash del service worker ya no se calcula solo sobre
el wasm, el SW y `index.html`: entra **cada fichero del paquete**, así que un cambio en cualquier
módulo cambia la versión y un visitante con caché no se queda con código viejo.

```sh
sh tools/entorno.sh publicar
grep -o 'sw.js?v=[0-9a-f]*' sitio/index.html | head -1
curl -sI https://apkc-studio.surge.sh/ | head -1
```

**Resultado**: `Success! - Published to apkc-studio.surge.sh`, `9 comprobaciones bien, 0 mal` contra
el host real y apk de 9221 bytes verificada por `apksigner`. La versión del SW pasó de
`97d25afb595e` (hash de tres ficheros) a `efffbf3972ee` (hash de todo el paquete). El sitio sigue
`HTTP/2 200`.

## 164. `apkc verify` comprueba v3 y v3.1

Medido el 2026-11-21. Cierra la asimetría que quedaba en la firma: `verify` nombraba los pares v3 y
v3.1 pero no los comprobaba. El firmante de v3 cambia el de v2 en dos bytes medidos: `minSdk` y
`maxSdk` van crudos fuera de `signedData`, y dentro de `signedData` va una lista de atributos
adicional detrás de los certificados (en la rotación, `proof-of-rotation`). La firma cubre los
bytes íntegros de `signedData`, no solo sus dos primeros elementos.

```sh
cd apkc && go test -run 'TestVerificarV3' -count=1 -v .
/tmp/opencode/apkc-gate verify <apk-con-v3>
/tmp/opencode/apkc-gate verify <apk-con-rotacion-v3.1>
$ANDROID_HOME/build-tools/34.0.0/apksigner verify --verbose --min-sdk-version 21 <apk-con-v3>
```

**Resultado**: los dos tests pasan: una apk firmada de verdad da `v1 ok`, `v2 ok`, `v3 ok` con un
firmante, y un byte cambiado en `classes.dex` da `v3: mal — el digest v3 guardado no es el de este
contenido`; `apksigner` dice `DOES NOT VERIFY` con el mismo `CHUNKED_SHA256 digest mismatch`. En una
apk con rotación, `apkc verify` dice `v3 ok` y `v3.1 ok` y `apksigner` aprueba v1/v2/v3/v3.1.
Frontera escrita: los atributos de rotación se leen pero no se validan contra el linaje, y la
cadena de confianza sigue sin ancla, igual que en v1/v2.

## 165. Sal en la firma v4 y el estado de los firmantes alternativos

Medido el 2026-11-21. `apksigner` de build-tools 34.0.0 no ofrece una opción de sal al firmar, pero
su propio `VerityTreeBuilder` la acepta al verificar: la sal se prefija **cruda a cada bloque de
cada nivel** del árbol, y va también en `HashingInfo` y en la carga firmada de cada `SigningInfo`.
`--v4-salt <hex>` (en `build` y en `sign`) la activa en este fork; sin ella el `.idsig` es el de
siempre. Los firmantes alternativos ya estaban en §121: con rotación v3.1 el `.idsig` lleva el
`SigningInfoBlock` con id `0x1b93ad61` y el gate lo verifica con `apksigner`.

```sh
cd apkc && go test -run TestV4 -count=1 -v .
/tmp/opencode/apkc-gate sign --v4-salt 0011223344 --out out.apk in.apk
$ANDROID_HOME/build-tools/34.0.0/apksigner verify --verbose --min-sdk-version 21 \
  --v4-signature-file out.apk.idsig out.apk
```

**Resultado**: los cuatro tests de v4 pasan, incluido `TestV4ConSal` (escritor y lector acuerdan
con sal no vacía). Con la sal del ejemplo, la raíz y el árbol que escribe este fork son **byte a byte**
los que producen las clases de `apksigner.jar`: `6ab21b9b…a0cce` y 4096 bytes de árbol, y la firma
verifica con el código de `V4Signature.getSignedData` del propio `apksigner`. **Frontera medida**:
el CLI `apksigner verify` de las build-tools 34.0.0, 35.0.1 y 36.0.0 reporta `Verified using v4
scheme: false` para ese mismo fichero (no así para el de sal vacía, que aprueba), así que no se
afirma que un dispositivo acepte la sal: el oráculo alto no la confirma aunque sus clases bajas sí.
`apkc verify` la acepta con los mismos oráculos internos que la v4 sin sal.

## 166. Más oráculos en CI: `dexdump` sí, ART en dispositivo no

Medido el 2026-11-21. El gate ya pasaba el dex del navegador por `d8`; ahora `e2e.sh` pasa el mismo
`classes.dex` por **`dexdump` de build-tools antes de que `d8` lo reescriba**, y falla si no lo
desarma. Es un oráculo de ART que corre en el host, así que funciona también en el runner de GitHub.
La comprobación del cotejo con la v3 de `apksigner` se actualizó: antes exigía el aviso de «no se
verificó aquí» (§164 la cerró) y ahora exige `v3: ok`.

```sh
sh tools/entorno.sh gate
$ANDROID_HOME/build-tools/34.0.0/dexdump -d $PREFIX/tmp/studio-e46-dex/classes.dex | head
```

**Resultado**: `gate rc=0` y `e2e: todo en orden` con `dexdump` activo. **Frontera**: en el runner de
GitHub no hay teléfono con `rish`/Shizuku ni `app_process64`, así que la ejecución real en ART
(`banco/banco.sh`, §61) sigue siendo una prueba local del aparato; no se puede añadir al CI sin un
dispositivo o un emulador con el `BOOTCLASSPATH` de `zygote64`, y eso no se afirma aquí.

## 167. `try` con `catch` que cruza una salida

Medido el 2026-11-21. Cierra la última frontera del control de flujo que quedaba en la fila 5.9.
La sonda `SalidaCatch` tiene tres formas: `try/catch/finally` con un `return` en una rama y caída en
la otra, la misma sobre un campo de instancia, y `try/catch` sin `finally` con dos `return`.

```text
javap 17 de f():  Exception table
  2  8  27  Class java/lang/RuntimeException
  15 19 27  Class java/lang/RuntimeException
  2  8  38  any
  15 19 38  any
  27 30 38  any
javap 17 de h():  0 7 12 RuntimeException ; 8 11 12 RuntimeException
```

El emisor ya repartía las filas `any` por trozo con `finally`; ahora reparte también las de cada
tipo de `catch`, y cuando no hay `finally` calcula los trozos con un caminante que excluye los
`return`/`goto` (pero **no** el `athrow`, que javac deja dentro del rango desde §71). La copia del
`finally` delante de la salida sigue sin protegerse.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run TestElTryConCatchQueCruzaSalida -count=1 -v .
sh tools/entorno.sh gate
```

**Resultado**: el test pasa con `Code`, **filas de excepción** y dex byte a byte los de javac; el
test que exigía rechazar este caso (`TestElFinalmenteRechazaLoNoMedido`) se actualizó. La ejecución
con `java -Xverify:all` sobre nuestras clases da la misma salida que javac en los once valores de
`-1..2` para `f`, `g` y `h`, con rc 0, y el gate sigue en verde. ART pendiente de dispositivo, como
en §156.

## 168. `arraycopy` y arreglos de dos dimensiones

Medido el 2026-11-21. `System.arraycopy(a, 0, b, 0, 3)` ya se resolvía contra el `android.jar` y su
`Code` difiere del de javac solo en los índices del pool; se sujeta aquí para que la frontera de la
fila 5.11 deje de nombrarlo. Lo nuevo es `new int[2][3]` y `m[i][j]`: javac escribe
`multianewarray`, que este emisor no conoce, así que se baja como d8:

```text
getstatic Integer.TYPE; newarray int con {2,3}; invokestatic Array.newInstance:(Class;[I)Object;
checkcast [[I; aaload; iaload
```

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run TestArregloDeDosDimensiones -count=1 -v .
java -Xverify:all -cp <clases> Main     # 7, igual que javac
d8 --debug ... Dos.class | dexdump -d  # filled-new-array, Array.newInstance, checkcast [[I
```

**Resultado**: el test pasa con los cinco opcodes esperados en el `Code` y el dex de este fork;
`java -Xverify:all` da el mismo valor que javac; `d8` relee la clase y produce las mismas
instrucciones Dalvik que con el `.class` de javac (`filled-new-array`, `invoke-static
Array.newInstance`, `checkcast [[I`, `aget-object`, `aget`). ART pendiente de dispositivo.

## 169. `goto_w` y `goto/16`

Medido el 2026-11-21. Una sonda generada con 10000 asignaciones fuerza un método de más de 48000
bytes de código, donde el `goto` de 2 bytes ya no alcanza. javac escribe
`3: ifgt 11; 6: goto_w 48016` — invierte la condición para que la branquia corta salte solo el
`goto_w` y el salto larguísimo lo haga la forma ancha.

El emisor ahora ensancha un `goto` a `0xc8` (4 bytes de desplazamiento) y, si el que se sale de rango
es un condicional, lo invierte e inserta el `goto_w` detrás. Insertar bytes obliga a desplazar
etiquetas, parches, filas de línea y la tabla de excepción, todo en el mismo punto. El verificador
(`jvmframes.go`) y el dexer (`dexinstr.go`) conocen `goto_w` y lo bajan a `goto/16` (0x29), que es lo
que d8 escribe (`2900 e32e`).

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run TestSaltoAncho -count=1 -v .
java -Xverify:all -cp <clases> Main     # 12000 y 5
```

**Resultado**: el test pasa, la clase escrita por este fork tiene `ifgt 11; goto_w ...`,
`java -Xverify:all` da los mismos valores que javac con rc 0, `d8` la desarma con `goto/16`, el dexer
propio la acepta y el gate sigue en verde. Frontera: los saltos mayores que 32767 **unidades Dalvik**
seguirían pidiendo `goto/32`, que no se ha medido.

## 170. Clase anidada estática y anotaciones más allá de `@Override`

Medido el 2026-11-21. Dos fronteras del analizador:

- `static class Anidada` dentro de `Exterior`: el nombre simple `Anidada` resuelve a
  `Exterior$Anidada`, y el par de clases sale con el dex byte a byte el de javac (incluido el
  `InnerClasses` con `static`). `java -Xverify:all` sobre nuestras clases imprime lo mismo que javac.
- `@Deprecated` sobre un método: javac escribe el atributo `Deprecated` (longitud 0) y un
  `RuntimeVisibleAnnotations` con una entrada sin pares; el dex de nuestras clases y de javac
  coincide. `@SuppressWarnings` no deja huella y se tolera igual que `@Override`.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run 'TestClaseAnidadaEstatica|TestAnotacionDeprecated' -count=1 -v .
sh tools/entorno.sh gate
```

**Resultado**: los dos tests pasan y el gate sigue en verde. Frontera escrita: una anidada con
nombre **`private`/`protected`** o **sin `static`** (pide `this$0`), y una anotación con elementos
o con retención distinta de las medidas, siguen rechazadas por nombre con su línea.

## 171. `layout-v1`, la copia automática de aapt2

Medido el 2026-11-21. `aapt2 link` **sin** `--no-auto-version` duplica cada layout base en la
configuración `v1`: `res/layout/main.xml` aparece también como `res/layout-v1/main.xml`. El
`ResTable_config` de esa copia tiene `sdkVersion = 1` en +24 y el spec lleva la bandera `0x0400`,
que es la de «clave con base» ya vista en densidades y `sw600dp` (§159).

`opcionesDeEnlace.autoVersion` reproduce ese modo; por defecto sigue desactivado para no romper los
fixtures del repo, que se generaron con `--no-auto-version`.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  go test -run TestAutoVersionLayoutV1 -count=1 -v .
```

**Resultado**: el test pasa con el `resources.arsc` byte a byte el de aapt2. El fixture se generó con
build-tools 35.0.1 porque la 34.0.0 escribe 0 en el u16 de +10 del `RES_TABLE_TYPE_SPEC` mientras
que 35 y 36 escriben el número de configuraciones, que es lo que hace este escritor (misma diferencia
medida en §159). Frontera: los calificadores cubiertos siguen siendo `values-xx`, `values-es-rMX`,
`land`, `port`, `sw600dp`, `nodpi`, `tvdpi`, las densidades `ldpi`-`xxxhdpi` y ahora `v1`; cualquier
otro (`small`, `large`, `xlarge`, `-vN` arbitrario) se rechaza por nombre.

## 172. CSP con hashes, PWA instalable y el `wasm-unsafe-eval`

Medido el 2026-11-21. El sitio ya tiene `manifest.webmanifest` con `id: "/"`, `start_url: "/"`,
`display: "standalone"` e iconos PNG de 192 y 512; `index.html` lo enlaza y el SW los precachea.
Surge no deja fijar cabeceras, así que la CSP la añade el service worker como el resto.

`tools/sitio.sh` calcula los hashes SHA-256 de los dos `<script>` inline de `index.html` y genera:

```text
default-src 'self'; script-src 'self' 'wasm-unsafe-eval' 'sha256-…' 'sha256-…';
worker-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self';
font-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'
```

```sh
sh tools/entorno.sh sitio
SITIO_DIR=sitio node studio/prueba-sitio.js        # Chromium
GECKODRIVER=… SITIO_DIR=sitio node studio/prueba-firefox.js
sh tools/entorno.sh gate
```

**Resultado**: `10 comprobaciones bien, 0 mal` en Chromium (el manifiesto y los dos iconos dan 200),
`4/4` en Firefox y gate en verde. El primer intento sin `'wasm-unsafe-eval'` rompió la compilación:
`WebAssembly.instantiate(): Compiling or instantiating ...` era el CSP bloqueando el wasm, no un
fallo del compilador; con ese token (que no autoriza `eval` de JavaScript) todo pasa. Las guías de
`modern-web-guidance` para manifest, iconos y seguridad se consultaron antes de escribir esta pieza.
Frontera: `require-trusted-types-for 'script'` no se activa porque el editor escribe en `<textarea>`
y en un `<pre>` con `textContent`, pero no se ha auditado con esa directiva.

## 173. Tamaño del wasm: qué reduce y qué no

Medido el 2026-11-21. Tres mediciones sobre el mismo fuente, con `GOOS=js GOARCH=wasm`:

```sh
cd apkc
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o /tmp/opencode/apkc-wasm.wasm .
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -ldflags='-s -w' -o /tmp/opencode/apkc-small.wasm .
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /tmp/opencode/apkc-small2.wasm .
gzip -9 -c /tmp/opencode/apkc-small.wasm | wc -c
```

**Resultado**: 12 875 248 bytes el binario normal, 12 779 866 con `-s -w` (solo un 0,7 %) y
12 770 117 con `-trimpath` además. En cambio `gzip -9` lo deja en **3 372 669 bytes**, un 73,6 %
menos. Surge no comprime `.wasm` (`content-length: 12907487` y sin `content-encoding`), así que
`tools/sitio.sh` publica `apkc.wasm.gz` y `hospedador.js` lo descomprime con `DecompressionStream`
si la API existe, con respaldo al `.wasm` sin comprimir. `tools/entorno.sh` usa `-s -w` en sus tres
caminos de build.

Chromium vuelve a dar `10 comprobaciones bien, 0 mal` con la ruta comprimida. Firefox había dado
`4/4` antes de este cambio; la repetición posterior se quedó en `session not created: Failed to set
preferences` de geckodriver en este equipo, así que **no se afirma** que la ruta comprimida esté
verificada en Firefox: queda como frontera del entorno de pruebas, no del código.

## 174. Accesibilidad y teclado: enlace de salto y foco visible

Medido el 2026-11-21. Auditoría del DOM y del CSS con la guía de accesibilidad de
`modern-web-guidance`: la página ya tenía `lang="es"`, `<title>`, `nav` con `aria-label`, botones
nativos y SVG decorativos con `aria-hidden`, pero suppressions de foco y ninguna vía de saltar el
encabezado.

Cambios: un `.skip-link` antes del `header` que apunta a `<main id="area-principal" tabindex="-1">`,
visible solo al recibir `:focus-visible`; y sustitución de los `outline: none` de `input`, `select`,
`textarea` y `#codigo` por anillos en `:focus-visible` con `outline-offset`, sin tocar el ring nativo
de los `button` que ya existía.

```sh
sh tools/entorno.sh gate
grep -E 'Saltar|focus-visible' studio/index.html studio/style.css
```

**Resultado**: `navegador: todo en orden` (la prueba de DOM comprueba `lang`, el enlace y el
`tabIndex=-1` del destino), `e2e: todo en orden` y `prueba-sitio.js` 10/10. La guía
`accessibility` se consultó antes de escribir estos cambios. Frontera: la auditoría no incluyó un
lector de pantalla real ni el patrón completo de teclado del árbol `role="tree"`, cuyas filas son
`<button>` nativos.

## 175. Informe agregado de fricción, sin red

Medido el 2026-11-21. El panel ya guardaba contadores en `localStorage` y ofrecía un informe
descargable, pero solo contaba las reglas reconocidas. Ahora cada línea que `analyze()` no sabe
explicar se cuenta como `friccion: <línea>` (sin pegar el log completo) y el informe añade una
sección «líneas sin explicar» con las diez más frecuentes. Se descarga como `blob:` desde el enlace
que ya existía: ninguna petición sale del navegador.

```sh
sh tools/entorno.sh gate
grep -n 'friccion' studio/diagnostico.js studio/app.js
```

**Resultado**: `navegador: todo en orden` y `e2e: todo en orden`. La prueba de DOM pega una línea
conocida y otra desconocida (`[build] mensaje nuevo sin explicar xyz`), descarga el informe desde el
`blob:` y comprueba que la segunda aparece en el agregado. Durante esta prueba se corrigió una
sonda rota por una secuencia `\\n` dentro de la plantilla del evaluador, que daba
`SyntaxError: Invalid or unexpected token` antes de llegar al aserto. Frontera: los contadores son
locales a cada navegador; agregarlos entre visitantes exigiría un servidor, que está fuera de la
ambición.

## 176. Tres mundos visuales para el editor, sin clonar VS Code

Medido el 2026-11-21. El usuario pidió las tres propuestas (Taller artesanal, Terminal moderno y
Azul refinado), no dos. La verdad del producto se escribió en `studio/PRODUCT.md` a partir de la
evidencia del repo, sin entrevista de producto: se dice aquí expresamente porque el usuario pidió
«adelante con todo» y esa sustitución debe quedar visible. El contrato de `Operate` está en
`studio/.impeccable/surfaces/index-html.md`: los temas cambian tipo, paleta, densidad y **una** firma;
no cambian navegación, ids ni controles.

Los tres mundos están en `studio/style.css` (`data-tema`) y se eligen en Ajustes. Cada uno aporta
dos marcas (riel y pestaña activa), no solo colores. `studio/DESIGN.md` y
`studio/.impeccable/design.json` fijan tokens y assets. Se corrigió el veredicto **FIX** de la
revisión final: se eliminó el kicker sobre el H1, se sustituyeron siete colores prestados del VS Code
por tokens de tema, se completaron las seis marcas, se cambiaron los glifos Unicode por `clip-path`
y SVG, y el foco de botón pasó de 1px a 2px.

```sh
impeccable detect --json studio/index.html studio/style.css studio/temas.js
node /tmp/opencode/capture-temas.js "$PWD/sitio" "$PWD/studio/.impeccable/review"
```

**Resultado**: el detector deja **0 warnings** y **1 advisory** (`3,4:1 — #ffffff sobre #2e92d5`,
azul calculado para el hover de un botón, no un literal del código). Las seis capturas viven en
`studio/.impeccable/review/`; la firma se ve en las parejas `*-editor.png`. Frontera: no se probó un
lector de pantalla real; el contraste se audita con el detector y con las capturas, no con un
espectrófotómetro.

## 177. Instalación PWA, idioma, atajos y Trusted Types

Medido el 2026-11-21. `#instalar-app` solo aparece si el navegador expone `navigator.install()` y
no hace ninguna petición: el manifest ya está enlazado en el documento. `studio/idioma.js` traduce
el marco y el Inicio entre español e inglés con `data-i18n` y guarda la elección en `localStorage`;
`studio/main.js` lo importa como módulo. El panel de atajos del Inicio se amplió con cabecera y
textos traducidos. El gate de WebKit corre en GitHub Actions con Playwright (job informativo: es el
motor de Safari en Linux, no Safari real) y `studio/prueba-webkit.js` comprueba aislamiento, carga y
cero peticiones fuera del origen.

Los cinco sumideros HTML se auditaron. Tres (`select.innerHTML = ""`) pasaron a `replaceChildren()`;
los otros dos (consola y resalto) pasaron por una política de Trusted Types en `studio/tt.js` que
rechaza `<script` y atributos `on*`. `tools/sitio.sh` inyecta `require-trusted-types-for 'script'` en
el CSP del service worker. Durante la verificación apareció un fallo real: el global es
`trustedTypes` en minúsculas; con `TrustedTypes` la política no se creaba y el CSP bloqueaba la
consola con `This document requires 'TrustedHTML' assignment`. Corregido y vuelto a medir.

```sh
node /tmp/opencode/prueba-idioma.js "$PWD/sitio"   # es→en, sin red
node /tmp/opencode/prueba-tt.js "$PWD/sitio"        # SW controla, CSP activo, cero excepciones
grep -n "require-trusted-types-for" tools/sitio.sh
```

**Resultado**: `idioma: todo en orden` (`lang`, H1, botón y selector en los dos idiomas) y
`trusted types: todo en orden` (`aislado:true, controlado:true, tt:true, cspActiva:true`, sin
excepciones). Frontera: Safari no se ejecuta en los runners; el job de WebKit es una aproximación
informativa y no bloquea el gate. No se añadió una cabecera CSP report-only paralela porque el SW ya
inyecta la política única: dos políticas duplicarían los informes sin añadir cobertura.

## 178. ART en CI: frontera justificada

Medido el 2026-11-21. La verificación en ART se hace hoy en el aparato, por `banco/banco.sh`, con
`dexdump` y `app_process64` del sistema (seis referencias en el guion); no hay `adb` y el banco usa
`rish` o `/usr/sbin/sudo`. El workflow de GitHub corre en `ubuntu-latest` y no instala emulador, AVD
ni imágenes del sistema: un runner Linux no puede ejecutar los binarios ART de un aparato Android y
la matriz de GitHub no ofrece macOS con emulador Android en este repo.

```sh
grep -n -iE 'emulator|avd|app_process|dexdump|art' .github/workflows/gate.yml
grep -c 'app_process64\|dexdump' banco/banco.sh
```

**Resultado**: el workflow no contiene emulador ni ART; el banco local sí (6 referencias). Frontera:
la cobertura de ART queda ligada al aparato y no a CI. Sustituirla exigiría un runner con KVM y una
imagen de Android arrancada, con coste de arranque y de mantenimiento fuera de la ambición del
proyecto; se registra aquí como deuda medida, no como fase cerrada.

## 179. Subconjunto Java: switch de String, var, enum, TWR, anidadas y anotaciones

Medido el 2026-11-21. Se cerraron seis piezas del inventario de la Fase 5, con javac 17 y el dexer
propio como oráculos. `switch (String)` se baja a `String.equals` más un `tableswitch` del índice del
brazo; la ejecución en la JVM con `-Xverify:all` dio `1, 2, 9`, igual que javac. `var` infiere el
descriptor del inicializador y el dex resultante es byte a byte el de `javac --release 11`
(`--release 8` no conoce `var`). `enum` simple emite ACC_ENUM, `java/lang/Enum`, `$VALUES`,
`values()`, `valueOf(String)` y el `<clinit>`, con dex byte a byte el de javac. El
`try-with-resources` se desugara a `try/finally` con `close()` en orden inverso: es equivalente
funcional, **sin `addSuppressed`**, así que no es el dex de javac 17 y esa diferencia queda escrita.
Las anidadas con nombre sin `static` y `private` emiten `this$0`, `initInner()` y ACC_PRIVATE
(byte a byte contra javac, salvo instanciar la privada desde fuera, que se rechaza con mensaje). Las
anotaciones con elementos escriben `RuntimeVisibleAnnotations` en clase, campo y método con
contenido semántico igual al de javac.

Dos tests preexistentes asumían que las anotaciones seguían rechazándose; se actualizaron a un valor
no simple (`@MiAnotacion(x = 1 + 2)`), que el parser rechaza con «elemento simple medido».

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar go test -count=1 ./...
sh tools/entorno.sh wasm
```

**Resultado**: `ok github.com/ajinasokan/apkc 40.055s` y el suite wasm en el hospedador de navegador
en verde, sin `expected magic word`. Frontera: TWR sin `addSuppressed` y la anidada privada
instanciada desde fuera siguen siendo diferencias funcionales documentadas, no paridad con javac.

## 180. Recursos: small/large/xlarge y -vN

Medido el 2026-11-21 con `aapt2` de build-tools 35.0.1. La premisa del encargo situaba
`screenLayout` en el offset +12 del `ResTable_config`; la medición lo corrige: está en **+28**
(`small`=1, `large`=3, `xlarge`=4) y la versión implícita que aapt2 eleva es 4. `-vN` se escribe en
+24. El orden de aapt2 es locale → `screenType` → entrada/screenSize → versión → `screenLayout` →
uiMode → smallestWidth; `claveDeConfig` se reescribió con esa clave, y el comparador de
`tablaDeRecursos` la usa. `mascaraConfig` usa `0x0c00` para tamaño, `0x0400` para `-vN` y `0x2400`
para `sw`. `fuentesDelProyecto` reconoce `layout-small/large/xlarge`, `values-v21`,
`layout-small-v21` y añade `-v4` a la ruta de salida cuando el tamaño no trae versión explícita.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar go test -count=1 ./...
```

**Resultado**: los tests nuevos (`valores_pantalla_version_test.go`,
`valores_pantalla_linker_test.go`) pasan con fixtures byte a byte de aapt2 y la suite completa da
`ok ... 40.055s`. Frontera: la clave de ordenación se midió con las combinaciones del fixture; un
calificador nuevo fuera de ese conjunto necesita su propia medición antes de entrar en el inventario.

## 181. Linaje y SourceStamp: frontera medida

Medido el 2026-11-21. `apkc verify` hoy lee los atributos del bloque v3.1 pero los descarta
(`verificar.go:730`); el formato del proof-of-rotation ya está escrito en `lineage.go` y lo valida
`lineage_test.go`. Con `apksigner rotate` + `apksigner sign --lineage … --rotation-min-sdk-version 33`
se obtuvo un APK que `apksigner verify --verbose --min-sdk-version 28` aprueba, con el atributo
`0x3ba06f8c` en v3.1 y solo `0x559f8b02` en v3.0. SourceStamp usa el par `0x6dff800d` y la entrada
ZIP `stamp-cert-sha256` (SHA-256 del DER del certificado de sello); `apksigner verify -v` lo marca
`true`. La firma del sello cubre pares `(id de algoritmo de digest, digest)` de los esquemas v2/v3;
el digest del esquema v1 no se identificó y `openssl dgst -verify` rechazó `MANIFEST.MF` y `.SF` como
candidatos.

```sh
cd /tmp/opencode/sst && apksigner verify -v stamped.apk | grep -i sourcestamp
grep -n 'proofOfRotationAttr\|rotationMinSdkAttr' apkc/lineage.go
```

**Resultado**: formato medido, **implementación no hecha**. Frontera: verificar el linaje exigiría
enganchar la cadena de firmas en `verificarUnBloqueV3`; SourceStamp podría cubrir v2/v3 pero no v1
sin fijar antes qué bytes se firman. Se deja como deuda medida, no como pieza cerrada.

## 182. Gate final sobre el árbol integrado

Medido el 2026-11-21, tras integrar las seis piezas Java, los calificadores de recursos, el fuzzing
y los cambios de la pestaña.

```sh
GOTMPDIR=$HOME/.cache/apkc-web/gotmp sh tools/entorno.sh gate
GOTMPDIR=$HOME/.cache/apkc-web/gotmp sh tools/entorno.sh wasm
cd apkc && D8=$ANDROID_HOME/build-tools/35.0.1/d8 go test -run TestFuzzDiferencialContraJavacYD8 -count=1 .
```

**Resultado**: `gate` rc=0 con `pestaña: 83 comprobaciones, apk firmada dentro de chromium de 10285
bytes y verificada`, build de la pestaña verificado por apksigner, zipalign, zipcheck y aapt2;
`wasm` rc=0 con `PASS`; el fuzzing diferencial de 20 casos contra javac `--release 8` y d8,
`ok ... 39.378s`. No se anuncia «0 pendientes»: la firma y ART siguen siendo fronteras escritas en
§178 y §181.

## 320. DEPS empaquetado en el sitio, todavía sin publicar

Medido el 2026-11-24 (F2). El paquete de dependencias generado fuera del repo se copia al árbol del
sitio antes de calcular `VERSION`, de modo que un cambio en el índice o en los artefactos cambie la
huella del service worker.

```sh
node studio/deps-index-local.mjs
DEPS_DIR=… sh tools/sitio.sh
cd studio && SITIO_DIR=../sitio node prueba-sitio.js
```

**Resultado**: el índice se generó con 1 artefacto (205 B, SHA `7dfda8807e76…`); `tools/sitio.sh`
empaquetó 70 ficheros y `VERSION` cambió de `5c81ac704338` a `de190ccf8ee5` (con el wasm exacto);
`prueba-sitio.js` dio `13 comprobaciones bien, 0 mal`, incluidas índice 200, artefacto con SHA y
cero peticiones ajenas. Cuatro falsaciones de traversal (`deps/../../shadow.txt`, `/etc/passwd`,
barras invertidas y symlink) dan rc 1 con mensaje explícito.

**Frontera**: nada publicado; F3 queda pendiente de autorización; `e2e.sh` sigue en verificación;
sin gate, chromium ni red.

