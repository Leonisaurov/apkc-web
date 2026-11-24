# Evidencia — §183-§204

## 183. Trusted Types: los otros sumideros aparecieron al probar el sitio

Medido el 2026-11-21. La primera medición de §177 solo veía los dos sumideros HTML; la prueba del
sitio empaquetado destapó dos más. `new Worker("hospedador.js")` exige `TrustedScriptURL` y fallaba
con `This document requires 'TrustedScriptURL' assignment`. Después, `politica.createURL` no existe
en Chromium y rompía el refresco del árbol. La política de `studio/tt.js` incorporó
`createScriptURL` (acepta el worker del mismo origen y los `blob:`); `enlaceConfiable` comprueba
`typeof politica.createURL === "function"` antes de llamarlo y, si el motor no lo expone, devuelve
la cadena, porque todos los `href` son literales o blobs creados aquí.

```sh
CHROME_BIN=/usr/bin/chromium SITIO_DIR=../sitio \
  ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  APKSIGNER=$ANDROID_HOME/build-tools/35.0.1/apksigner node studio/prueba-sitio.js
GOTMPDIR=$HOME/.cache/apkc-web/gotmp sh tools/entorno.sh gate
```

**Resultado**: `10 comprobaciones bien, 0 mal`, cero peticiones fuera del origen, APK de 9221 bytes
compilada y firmada en el sitio y verificada por apksigner; `gate` rc=0 con `pestaña: 83
comprobaciones`. Lección registrada: el CSP aplicado por el service worker no se manifiesta hasta
que el SW controla la página, así que comprobar solo la primera carga no basta.

## 184. Publicación del sitio integrado

Medido el 2026-11-21. Publicación manual, no automática: `tools/entorno.sh publicar` reconstruye el
wasm desde el árbol integrado, empaqueta `studio/` desde el HEAD confirmado del submódulo, sube a
`https://apkc-studio.surge.sh/` y corre la prueba del sitio sobre el paquete local. El dominio se
comprueba después contra el host real, no contra el paquete.

```sh
GOTMPDIR=$HOME/.cache/apkc-web/gotmp sh tools/entorno.sh publicar
CHROME_BIN=/usr/bin/chromium SITIO_URL=https://apkc-studio.surge.sh/ \
  ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  APKSIGNER=$ANDROID_HOME/build-tools/35.0.1/apksigner node studio/prueba-sitio.js
```

**Resultado**: el paquete local dio `10 comprobaciones bien, 0 mal` con 47 peticiones, 0 fuera del
origen; el host publicado dio también `10 comprobaciones bien, 0 mal`, APK de 9217 bytes firmada y
verificada por apksigner, SW que controla la página y `crossOriginIsolated:true`. Noviembre de
2026: la firma (§181) y ART en CI (§178) siguen siendo fronteras escritas, no piezas terminadas.

## 185. Linaje v3.1 y SourceStamp: de frontera a pieza cerrada

Medido el 2026-11-21. Se corrige lo escrito en §181: ya no es una frontera. `verificar.go` lee los
atributos detrás de los certificados del `signedData`, valida la cadena de proof-of-rotation nodo a
nodo con el algoritmo del padre y las claves de `lineage.go`, exige que el certificado del firmante
sea la hoja del linaje y comprueba los rangos (`rotation-min-sdk`: v3.0 termina en `minSdkAttr-1`,
v3.1 empieza en `minSdkAttr`, y no conviven en el mismo bloque). SourceStamp cubre v2/v3 y también
v1: los bytes firmados del esquema 1 son `pairsOf([{4, SHA-256(MANIFEST.MF)}])`, es decir
`uint32(8+32) | uint32(4) | uint32(32) | SHA-256(de los bytes sin comprimir de MANIFEST.MF)`. La vía
ingeniosa de firmar `MANIFEST.MF` o `.SF` directamente daba `Verification failure`; la causa era el
envoltorio `pairsOf` con el id de digest 4, no el manifiesto.

```sh
apksigner rotate --out lineage --old-signer --ks viejo.p12 --new-signer --ks nuevo.p12
apksigner sign --lineage lineage --rotation-min-sdk-version 33 --ks nuevo.p12 -o rotated.apk in.apk
apksigner verify -v --min-sdk-version 28 rotated.apk
apkc verify rotated.apk
apksigner sign --stamp-signer --ks stamp.p12 -o stamped.apk in.apk
apkc verify stamped.apk
```

**Resultado**: `apksigner` da v3 `true` y v3.1 `true`; `apkc verify` rc 0 con `v3 ok`, `v3.1 ok` y
`SourceStamp: ok`; los cuatro tests nuevos pasan y la suite completa da
`ok github.com/ajinasokan/apkc 49.187s`. Frontera que queda: la rotación con claves EC no se ha
medido con `apksigner` (ya señalada en §121).

## 186. Recursos: uiMode, touchscreen, teclado y navegación

Medido el 2026-11-21 con aapt2 de build-tools 35.0.1. El encargo situaba `uiMode` en +36; la medición
lo corrige a **+29** (spec `0x1400` = `0x1000` de UI_MODE más `0x0400` de versión 8). Touchscreen está
en +13, keyboard en +16 y navigation en +17. Medidos desk=2, car=3, television=4, appliance=5,
watch=6, night=0x20 y car-night=0x23, con sdkVersion 8 implícito; `car-v21` eleva la versión a 21.
El arsc de 5052 bytes sale byte a byte igual que el de aapt2 con la mezcla completa (small, seis
uiMode, car-night, sw600dp, v21, tres teclados, tres navegaciones, tres touchscreen, orientación,
densidad y `values-es` con clave sin base). `claveDeConfig` y `mascaraConfig` incorporan los campos
nuevos sin romper locale/densidad/versión/screenLayout/sw600dp.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar go test -count=1 ./...
```

**Resultado**: los tres tests nuevos (`valores_entrada_uimode_test.go`,
`valores_fuentes_calificadores_test.go`, `valores_sinbase_test.go`) pasan. Un test preexistente
esperaba el motivo sin tilde («configuracion») y se actualizó a «configuración». Frontera: la
fidelidad de `CompoundValue` en estilos y `attr` no se reproduce en el bundle (se dice en §187).

## 187. El .aab nace en Go, también desde el enlazador y el compilador Go

Medido el 2026-11-21. Se cierra la frontera 5.33 (§128). `aab_go.go` convierte el AXML del manifiesto
y de los layouts a `XmlNode` proto, y el `resources.arsc` del enlazador a `ResourceTable` (paquete,
tipos, entradas `0xPPTTEEEE`, configuraciones y `FileReference`), añade `BundleConfig.pb` y el prefijo
`base/`. La medición previa extrajo `Resources.proto` del propio jar de bundletool 1.18.3 y fijó el
contrato con un oráculo preciso: `bundletool validate` rechaza un `resources.pb` vacío y acepta un
`ResourceTable` mínimo con paquete y `FileReference`. La integración CLI cambió `build.go` para no
esperar `resources.pb` cuando el enlazador es Go, para no construir `bundle.zip` con el firmante Go,
y `compilar.go` para dejar de rechazar `--compiler go --aab`.

```sh
java -jar bundletool-all-1.18.3.jar validate --bundle=/tmp/opencode/aab-desde-go.aab
jarsigner -verify /tmp/opencode/aab-desde-go.aab
sh tools/entorno.sh gate
```

**Resultado**: `bundletool validate` rc 0, `jarsigner -verify` → `jar verified`, `apkc verify` →
`bundle=true v1=true`. En el gate: `java: .aab de 5891 bytes, ensamblado y firmado en Go, validado
por bundletool y jarsigner`. Frontera: los valores compuestos conservan su id con `Item.id` vacío y
no se traduce el contenido de `CompoundValue`; no se reproducen `source_pool`, `tool_fingerprint` ni
posiciones de columna, que el oráculo no exige.

## 188. Editor: check en vivo, plantillas, paleta y contraseñas redactadas

Medido el 2026-11-21. `apkc check` (`apkc/check.go`) devuelve una línea JSON con
`{tipo,fichero,lineas,emisor,aviso,errores}` sin escribir `.class`; está excluido de `findSDKs`, así
que corre en wasm sin procesos. `studio/diagnostico-vivo.js` escribe el buffer en un fichero temporal,
llama al hospedador con 350 ms de debounce, limpia el temporal y pinta la lista y el subrayado. Se
añadieron las plantillas `ciclo`, `menu` y `traducido` (compiladas y verificadas en el gate) y la
paleta `Ctrl+Mayús+P` con seis acciones que reutilizan funciones e ids existentes. El hallazgo M1 del
informe de seguridad quedó cerrado: `redactarArgv` sustituye los valores de `--*pass*` por `***` solo
en la copia que se imprime, sin mutar el `argv` que se ejecuta.

```sh
apkc check --android-jar $ANDROID_HOME/platforms/android-34/android.jar /tmp/opencode/check-malo.java
node studio/prueba.js
sh tools/entorno.sh gate
```

**Resultado**: `{"tipo":"check","fichero":"check-malo.java","lineas":…,"errores":[{"linea":5,…}]}`;
`prueba: todo en orden` (24 reglas, 9/9 muestras); gate con `prueba: 85 comprobaciones` (83 antes de
las dos de M1) y las tres plantillas nuevas verificadas por apksigner, zipalign y aapt2. Frontera: el
parser y el emisor paran en el primer error; el JSON no es una lista de recuperación. El diagnóstico
en vivo y la paleta se verificaron con un guion CDP temporal, no con una prueba versionada en el repo.

## 189. Infraestructura: ART y Safari informativos, bench y auditoría

Medido el 2026-11-21. El workflow tiene seis jobs (`rapido`, `pesado`, `webkit`, `safari`, `art`,
`publicar`). `art` arranca un emulador con KVM en `ubuntu-latest` y corre el banco ART; es
informativo porque KVM depende del hardware del runner y aquí no se pudo ejecutar. `safari` corre en
`macos-latest` e imprime la identidad del ejecutable con `tools/safari-info.mjs`: Playwright no
automatiza `Safari.app`, así que sigue siendo WebKit, no Safari real. El job `webkit` preexistente
era un **falso verde**: `npx -y playwright@latest install` deja el paquete en la caché, no en
`node_modules`, y `import("playwright")` fallaba silenciosamente; ahora instala el paquete antes.
`publicar` sigue dependiendo solo de `pesado`. `tools/bench-web.sh` mide con CDP sobre el sitio
empaquetado.

```sh
NODE_PATH=/tmp/opencode/yaml/node_modules node -e "const y=require('js-yaml'),fs=require('fs');console.log(Object.keys(y.load(fs.readFileSync('.github/workflows/gate.yml','utf8')).jobs).join(', '))"
tcr -q -- env ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar sh tools/bench-web.sh "$PWD/sitio"
```

**Resultado**: `jobs: rapido, pesado, webkit, safari, art, publicar`. Bench sobre el árbol integrado:
`aislamiento_ms 406`, `wasm_instantiate_muestras 5`, `wasm_instantiate_total_ms 274`,
`android_jar_opfs_ms 385` para 26361808 bytes, `primer_build_ms 3881` para 9221 bytes, 43 peticiones
de red. `INFORMA-SEGURIDAD.md` recoge M1 (cerrado en §188), M2 (falso verde cerrado) y los hallazgos
B1-B4 abiertos: CSP solo por el SW, nombre de Trusted Types no fijado, `wasm_exec.js` sin fijar y zip
sin cota de descompresión. Frontera: los jobs `art` y `safari` se verificaron por sintaxis YAML y de
guiones, no ejecutándose en sus runners.

## 190. Gate final, dos agentes caídos y lección de orquestación

Medido el 2026-11-21. Se integraron seis worktrees con dueño exclusivo de archivos. Dos agentes
alcanzaron el límite de pasos (Java y firma) y uno devolvió un informe corrupto (recursos): en los
tres casos el trabajo estaba en el worktree y se recuperó, se limpió y se verificó antes de
cherry-pick. En Java solo se conservaron las dos piezas cerradas (`i2b`/`i2s` y `ConstantValue` de
`static final long`); la anónima dentro de anónima emitía `MethodParameters` y `InnerClasses` que no
son los de javac y su test llevaba código de depuración, así que se revirtió y sigue siendo frontera.
No se ejecutaron `addSuppressed`, la anidada privada desde fuera ni la resolución de sobrecargas con
boxing/varargs/ensanchamiento: siguen pendientes y no se afirma lo contrario.

```sh
tcr -q -- env APKC_ANDROID_JAR=… JAVAC=… D8=… go test -count=1 ./...
tcr -q -- sh tools/entorno.sh wasm
tcr -q -- sh tools/entorno.sh gate
python3 ~/.config/opencode/validate_config.py
```

**Resultado**: suite Go `ok … 46.465s` con javac y d8 activos; suite wasm `PASS`; gate rc=0 con
`pestaña: 85 comprobaciones`, cuatro plantillas verificadas y `.aab` ensamblado y validado;
configuración de OpenCode `TODO VÁLIDO` (12 agentes, `git push global = ask`). Lección: el worktree
aislado evitó pisadas entre agentes, pero no evita que un agente se quede sin pasos o devuelva un
informe inútil; el árbol del worktree es la fuente de verdad, no el informe.

## 191. Verificación del host publicado: transporte HTTP/2 truncado, no falso verde

Medido el 2026-11-21. Tras publicar con `tools/entorno.sh publicar`, la prueba contra
`https://apkc-studio.surge.sh/` **no** dio 10/10: el paquete local sí (10/10, 41 peticiones, 0 fuera
del origen), pero el host real dio 8/10. Con chromium por defecto (HTTP/2) la compilación fallaba
primero con `FALLO DEL HOSPEDADOR: Failed to fetch` y, tras corregir la carga, con
`WebAssembly.instantiate(): section (code 10, "Code") extends past end of the module (length 9361416,
remaining bytes 5343765)`: el cuerpo del wasm llega truncado aunque el `fetch` devuelve 200.

`critico` atacó la hipótesis y dio **INCIERTO** en lo fuerte, con dos hallazgos: (1) **CONFIRMADO**
que HTTP/2 trunca el gz — `curl (92) … error -532`, 1093696 bytes descargados frente a 3496258 por
`--http1.1`, y el hash de este último coincide con el local `e72ec244…`; (2) **CONFIRMADO** que
`hospedador.js:151-155` no validaba los bytes y su fallback era inalcanzable si `arrayBuffer()`
lanzaba. Se aplicó solo (2): `try/catch` alrededor de la descompresión, comprobación de la cabecera
`00 61 73 6d` y mensaje explícito si el `.wasm` sin comprimir también llega truncado. La sospecha de
que el SW sirviera una versión vieja quedó **DESCARTADA** por la igualdad de hashes.

```sh
curl -sS -o /dev/null -w "%{http_code} %{size_download}" https://apkc-studio.surge.sh/apkc.wasm.gz
curl --http1.1 -sS -o /dev/null -w "%{http_code} %{size_download}" https://apkc-studio.surge.sh/apkc.wasm.gz
CHROME_BIN=/usr/bin/chromium SITIO_URL=https://apkc-studio.surge.sh/ node studio/prueba-sitio.js
```

**Resultado**: sitio publicado y accesible; verificación local 10/10; verificación del host en este
entorno 8/10 por truncamiento HTTP/2 de un fichero de 3,5 MB, confirmado con `curl`. Frontera
abierta, no disfrazada de verde: no se afirma que el sitio falle para otros navegadores o redes, ni
se afirma que esté 10/10 en el host desde este entorno. Queda pendiente repetir el `fetch` dentro del
worker real con red verificada y medir el SW tras estabilizar la recarga, que son las dos condiciones
que `critico` puso para cerrar la hipótesis.

## 192. ART en la Blackview, no en emulador

Medido el 2026-11-21. Se corrige la frontera de §178 y el informe del subagente de infra, que decía
que no había KVM: `/dev/kvm` existe, pero el usuario pidió usar la tableta que depura por WiFi. El
emulador que se había arrancado se apagó con `adb -s emulator-5554 emu kill` y no se volvió a usar.
`banco/art.sh` fija ahora el serial `192.168.1.53:46041` en **todas** las llamadas a `adb` y comprueba
`ro.product.brand=Blackview` y `ro.product.model=MEGA 3` antes de compilar, para no confundirla con
el Samsung `RFCNB036HWV` conectado por USB. El serial se descubrió con `adb mdns services`
(`adb-tls-connect`); no hay nada en el puerto 5555 de esa subred. El `d8` de build-tools 34.0.0
falla con un `NullPointerException` interno sobre `Ver$1.class`, así que la corrida usa el de 36.0.0.

```sh
adb devices -l
adb -s 192.168.1.53:46041 shell getprop ro.product.model
JAVA_HOME=/home/leonisauro/opt/jdk17 BT=36.0.0 sh banco/art.sh
```

**Resultado**: aparato `MEGA 3`, Android 15, `uid=2000(shell)`; los tres dex —nuestro dex de punta a
punta en Go, el de `d8` sobre javac y el de `d8` sobre nuestra `Ver`— dan `rc=102` con salida idéntica
a la JVM (31 valores) y `dexdump rc=0`. `art: en orden`. Frontera que queda: el job `art` de CI sigue
siendo informativo y usa emulador con KVM, porque un runner no tiene esta tableta; lo que sí está
cerrado es la verificación en ART del árbol actual sobre un aparato real.

## 193. Las cuatro fronteras Java que quedaban, cerradas

Medido el 2026-11-21 con javac 17.0.21 y comparación byte a byte de dex. Se cerraron en cuatro
commits separados, con `go test -count=1 ./...` tras cada uno (el entorno estaba justo de RAM y `tcr`
mató una corrida con `RAM libre del sistema por debajo del umbral`; se usó `GOGC=50 GOMAXPROCS=2`).

1. **`try-with-resources` con `addSuppressed`**: con un recurso se emite el patrón de javac —local
   `primary`, segundo rango protegido para `close()`, `Throwable.addSuppressed` y `athrow— y el dex
   sale byte a byte. Los múltiples recursos siguen con el desugaring funcional de §179.
2. **Anidada privada instanciada desde fuera**: constructor sintético `(LCapa$1;)V`, clase marcador
   con `EnclosingMethod` e `InnerClasses` `0x1008`, llamada con `aconst_null` y soporte de `const/4`
   en el traductor; tres clases byte a byte contra `javac --release 8`.
3. **Sobrecargas con boxing, ensanchamiento y arreglos**: `Integer.valueOf`, `ldc2_w` sobre la
   constante y `int[]` frente a `Object`, byte a byte. **Frontera**: los varargs se siguen rechazando
   en el parser (`un parametro necesita nombre`) y hay un test que lo fija.
4. **Anónima dentro de anónima**: `Capa$1$1` con `this$0:LCapa$1;`, `val$m`, ctor `(LCapa$1;I)V`,
   `Signature: ()V`, `EnclosingMethod` a `Capa$1.run` y las dos filas de `InnerClasses` en el orden de
   javac. `initCaptura` deja de escribir `MethodParameters` cuando la clase creadora es anónima. Las
   tres clases salen byte a byte. La sonda usa `int m = getM()` porque el parser no acepta `final` en
   una declaración local, y se comprobó que el `.class` es idéntico al de la sonda con `final`.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar go test -count=1 ./...
GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh gate
```

**Resultado**: `ok github.com/ajinasokan/apkc 8.497s` sobre el árbol integrado, suite wasm `PASS` y
gate rc=0 con `pestaña: 85 comprobaciones`, cuatro plantillas verificadas y el `.aab` ensamblado en
Go validado. Frontera que queda: varargs en sobrecargas; las otras tres piezas no tienen deuda.

## 194. Deuda abierta al cierre de esta ronda

Medido el 2026-11-21. Se enumera lo que **no** está cerrado, con el comando que lo reproduce, para
no confundir «frontera escrita» con «hecho». Los puntos 1 y 2 se cerraron después con su medición en
§199; este párrafo conserva el estado de la ronda que lo originó.

1. **Varargs en resolución de sobrecargas**. `resuelveSobrecarga` acepta boxing, ensanchamiento y
   arreglos (§193), pero un parámetro `int... xs` se rechaza en el parser con `un parametro necesita
   nombre`. Lo fija `TestLosVarargsSiguenSiendoFrontera`.
2. **`try-with-resources` con varios recursos**. El de un recurso es byte a byte de javac con
   `addSuppressed`; los múltiples siguen con el desugaring funcional de §179 (sin `addSuppressed`).
3. **CI sin ART ni Safari reales**. Los jobs `art` y `safari` son informativos: ART sí se verificó en
   la Blackview (§192), pero un runner no tiene esa tableta; en macOS Playwright ejecuta su WebKit,
   no `Safari.app`. `publicar` depende solo de `pesado`.
4. **Host publicado verificado 10/10 por HTTP/2 en §196 y §198**. HTTP/2 trunca el wasm (§191), pero
   el 8/10 quedó superado por la recuperación por rangos; el sitio sirve aún la build anterior
   (§242 y §252).
5. **Hallazgos del informe de seguridad**: el nombre de la política de Trusted Types quedó fijado en
   `d5efe3d` (§237); `wasm_exec.js` sigue sin fijarse por hash (§198), deuda que queda abierta. B1
   (meta CSP) y B4 (cota de ZIP) sí están cerrados.
6. **Pruebas no versionadas**: el diagnóstico en vivo y la paleta se verificaron con un guion CDP
   temporal; no hay prueba committed para ellos. `pestaña.js` sí tiene la prueba de redacción de
   contraseñas.
7. **Detector de interfaz**: `impeccable detect --json studio/index.html studio/style.css studio/temas.js`
   devuelve **0 warnings** y **2 advisories**: `3.4:1 — #ffffff sobre #2e92d5` (azul calculado para
   un hover, no literal del código) y `repeating-gradient decorative stripes` en `.marca-error`
   (`style.css:1008`, subrayado funcional de error, no decoración). Se decide no nominar publicación
   por un advisory estético; queda escrito.

```sh
grep -n 'TestLosVarargsSiguenSiendoFrontera' apkc/javaemisor_sobrecarga_conversiones_test.go
grep -n 'continue-on-error' .github/workflows/gate.yml
impeccable detect --json studio/index.html studio/style.css studio/temas.js
```

## 195. Diagnóstico en vivo y copias truncadas del wasm

Medido el 2026-11-21. El usuario preguntó por el checker de la pestaña Diagnóstico: `apkc check`
(`apkc/check.go`) devuelve un JSON con errores del parser/emisor sin escribir `.class`, y
`studio/diagnostico-vivo.js` lo llama con 350 ms de debounce, pinta la lista y subraya en
`#diagnostico-marcas`. El mismo usuario reportó que no podía compilar en su navegador con el error
`section (code 10, "Code") extends past end of the module`, que es el truncamiento de §191.

`tools/sitio.sh` escribe ahora `wasm-manifest.json` con el tamaño y el SHA-256 reales del binario,
y `sw.js` lo precachea. El worker comprueba cada copia contra ese manifiesto antes de instanciar:
primero la del SW, después una URL con `?directo=` que fuerza red, después el binario sin comprimir
y por último su variante con cache-busting. Si ninguna sirve, el error es explícito e indica recargar
con `Ctrl+Mayús+R` o borrar los datos del sitio, en vez del mensaje opaco de `WebAssembly`.

```sh
node /tmp/opencode/prueba-vivo-final.js
cat sitio/wasm-manifest.json
sha256sum sitio/apkc.wasm
CHROME_BIN=/usr/bin/chromium SITIO_DIR=../sitio node studio/prueba-sitio.js
```

**Resultado**: `vivo: todo en orden` (9/9: fuente bueno sin errores, error en vivo en el panel, una
marca de subrayado, paleta abierta/filtrada/cerrada). El manifiesto dice `bytes 13320296` y
`sha256 5b64608d…cf09f21`, idéntico al `sha256sum` del fichero. El sitio local da 10/10. En el host
real, desde este entorno donde **todas** las peticiones del binario salen truncadas, la prueba ahora
falla con `no se pudo bajar un apkc.wasm íntegro: recarga con Ctrl+Mayús+R…`, que es la detección
funcionando, no el error de `instantiate`. Frontera: no se pudo verificar aquí que la variante
`?directo=` recupere una copia sana en la red del usuario; eso depende de su navegador.

## 196. Causa del wasm truncado y recuperación

Medido el 2026-11-22. El error del usuario `section (code 10, "Code") extends past end of the module`
no era del proyecto: era el binario llegando a medias. Se separaron tres cosas:

1. **Causa del entorno**: Surge trunca respuestas grandes por HTTP/2 desde esta red. `curl` con
   `--http1.1` baja el gz completo; por HTTP/2 falla con `error -532`. En rangos, 256 KB llegan
   enteros (262144 bytes) y 512 KB se cortan. Cloudflare sirve 2 MB por HTTP/2 sin problema, así
   que no es la red en general ni Chromium.
2. **Lo que empeoró el proceso**: cada publicación cambia la versión del SW; `activate` borra la
   caché anterior y `install` vuelve a hacer `cache.add('./apkc.wasm.gz')` con un `catch` que traga
   el fallo. Con la red ya truncada, la copia sana desapareció y no quedó respaldo. Además el
   fallback con `?directo=` era inútil: `caches.match(peticion, { ignoreSearch: true })` ignoraba la
   query y siempre devolvía la copia cacheada. Se reportó como arreglo sin verificarlo en el host.
3. **Arreglo**: `tools/sitio.sh` escribe `wasm-manifest.json` con tamaño y SHA-256 del wasm y del
   gz; `sw.js` deja pasar `?directo=` sin tocar caché; `hospedador.js` prueba la copia normal, la red
   forzada y, si ambas llegan truncadas, reconstruye el gz con peticiones `Range` de 256 KB y
   comprueba ambos hashes antes de instanciar. Si nada sirve, el error manda a «Reparar caché», un
   botón escondido en Tema → Reparar caché (último recurso) que desregistra el SW, borra las cachés
   y recarga. `tools/verificar-publicacion.sh` compara por HTTP/1.1 el gz publicado con el local y
   falla la publicación si no coinciden.

```sh
sh tools/verificar-publicacion.sh https://apkc-studio.surge.sh/ sitio/apkc.wasm.gz
CHROME_BIN=/usr/bin/chromium SITIO_URL=https://apkc-studio.surge.sh/ node studio/prueba-sitio.js
```

**Resultado**: `publicación verificada: sha256 5e5daa06…f0166f`; el host real da **10 comprobaciones
bien, 0 mal** con chromium por defecto (HTTP/2), 53 peticiones, 0 fuera del origen, APK de 9221 bytes
firmada y verificada. Frontera: el botón de reparación no se ha pulsado en un navegador con la caché
envenenada; se ofrece como último recurso, no como primera línea. No se volvió a publicar después
de la comprobación.

## 197. Carga, respaldo de caché y pruebas versionadas

Medido el 2026-11-22. Se cerraron los cinco puntos que faltaban después del truncamiento:

1. **Progreso del precache**: `sw.js` guarda el shell en secuencial y `index.html` cuenta las entradas
   de la caché cada 200 ms, reflejándolo en el velo mediante `--texto-preparacion`. Con perfil fresco
   se midió `Guardando el navegador (30 ficheros)…` antes del aislamiento. Se descartaron dos vías:
   `client.postMessage` desde `install` y `BroadcastChannel` no entregaban en ese momento; el
   sondeo de la caché sí.
2. **Última copia buena**: `activate` conserva la versión actual y la anterior, en vez de borrar
   todas. Como `caches.match` busca en todas las cachés, una instalación a medias o sin red puede
   servirse de la anterior.
3. **Aviso de recuperación**: cuando el wasm se reconstruye por rangos, el worker envía
   `tipo:"aviso"` y `pestana.js` lo escribe en la consola: «se reconstruyó por trozos: la copia
   guardada estaba incompleta».
4. **Pruebas versionadas**: el diagnóstico en vivo y la paleta viven ahora en `prueba-pestana.js`
   (ya no solo en un script CDP temporal). El gate subió de 85 a **87 comprobaciones**.
5. **Detector**: `.marca-error` pasó de `repeating-linear-gradient` a un fondo plano. El detector
   quedó en **0 warnings, 1 advisory** (el de contraste calculado de §194).

```sh
node /tmp/opencode/diag-caches-local.js
GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh gate
impeccable detect --json studio/index.html studio/style.css studio/temas.js
```

**Resultado**: velo con progreso medido, gate rc=0 con `pestaña: 87 comprobaciones`, detector con
1 advisory, sitio local 10/10. Frontera: no se simuló el escenario de dos versiones de SW en un
mismo perfil para medir el respaldo, ni se disparó a propósito la recuperación por rangos para ver
el aviso en pantalla; ambos están implementados y el gate sigue en verde.

## 198. Estado final para una sesión limpia

Medido el 2026-11-22. Este párrafo es el punto de entrada al reanudar el trabajo: qué está verde,
qué queda y cómo reproducirlo. No sustituye a los párrafos anteriores, los indexa.

```sh
GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh gate
GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh wasm
CHROME_BIN=/usr/bin/chromium SITIO_DIR=../sitio ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar \
  APKSIGNER=$ANDROID_HOME/build-tools/35.0.1/apksigner node studio/prueba-sitio.js
SITIO_URL=https://apkc-studio.surge.sh/ node studio/prueba-sitio.js
sh tools/verificar-publicacion.sh https://apkc-studio.surge.sh/ sitio/apkc.wasm.gz
python3 ~/.config/opencode/validate_config.py
```

**Resultado**: gate rc=0 con `pestaña: 87 comprobaciones`, suite wasm `PASS`, sitio local 10/10, host
real 10/10 por HTTP/2 con 53 peticiones y 0 fuera del origen, `publicación verificada: sha256
db1bf7ca36e39c24bbf00353d358d31d5da6864add486bb8351dfb4b79813aa3`, configuración de OpenCode
válida. `origin/web` y `origin/main` del submódulo están alineados; los worktrees están limpios y
`studio-test.zip` se ignora. Deuda consciente, no cerrada: §194 (varargs y TWR múltiple cerrados
después en §199; jobs
informativos, un advisory del detector (contraste calculado; §197), pruebas no versionadas resueltas en §197, hash de
`wasm_exec.js`), §178 (ART real sí en la Blackview, no en CI) y §181/§185 (firma, sí cerrada).

## 199. Cierre de la deuda Java: varargs y `try-with-resources` múltiple

Medido el 2026-11-22 con javac 17.0.21. Cierra los dos puntos Java de la deuda de §194 (1 y 2).
La comparación es siempre la misma: el fuente se compila con el emisor del fork y con
`javac --release 8 -g:lines,source`, los dos `.class` pasan por `dexDeClases` y el test exige
`bytes.Equal`. El oráculo no comparte código con el emisor.

1. **Varargs en resolución de sobrecargas.** El léxico ya traía `"..."`; el parser ahora lo acepta
   como último parámetro (`javafuente.go`), guarda el tipo como arreglo (`int... xs` → `int[]` →
   `[I`) y marca el método. El `method_info` lleva `ACC_VARARGS` (**0x0080**, medido `0x0088` en un
   `static`: sin el bit el dex no coincide, porque `accesoDex` lo conserva). `buscarMetodo` añade la
   fase 3 de JVMS 15.12.2.4 **después** de las exactas y de las aplicables de aridad fija: los
   primeros `len(ps)-1` argumentos van a los parámetros fijos y cada uno de los restantes debe caber
   en el tipo elemento del último arreglo. Una llamada con el arreglo declarado no empaqueta.
   `argumentosDeLlamada`/`empaquetaVarargs` escriben la cadena de javac (`cargaEntero(n)`,
   `newarray`/`anewarray`, `dup`, índice, valor, `*astore`). Medido byte a byte en los caminos sin
   receptor, con receptor implícito y explícito, y con el tipo cualificando una llamada estática.

2. **Boxing hacia referencia, que la fase 3 necesitaba.** `asignable` solo aceptaba boxing cuando el
   envoltorio era exactamente el parámetro, así que `asignable("I", "Ljava/lang/Object;")` era falso
   y `String.format("%d %s", 1, "x")` moría con «ninguna es la mas especifica». Ahora un primitivo con
   envoltorio conocido se prueba contra la cadena de referencias del envoltorio (JVMS 5.3). `expA`
   emite el `valueOf` y devuelve el descriptor del envoltorio, que es lo que queda en la pila; por eso
   `empaquetaVarargs`, el inicializador `new T[] {…}` y la asignación `a[0] = 7` sobre un `Object[]`
   aceptan un descriptor asignable al elemento, no solo el idéntico. Sin esto el validador veía un
   `int` suelto contra un descriptor `(Ljava/lang/Object;)V`. Cubierto por los tres caminos de
   llamada y por el de asignación a arreglo.

3. **`try-with-resources` con N recursos.** `jtry.recurso` pasa a ser `recursos []jdecl` y
   `recursoAnidado` emite un nivel por recurso, como javac 17: la declaración queda fuera del rango,
   que cubre el nivel interior entero; el manejador guarda la primaria en el hueco que deja su propio
   recurso (javac reutiliza el del interior y, si el cuerpo declaró locals, el del primero de ellos),
   cierra dentro de un segundo rango y añade la del cierre con `Throwable.addSuppressed` antes de
   re-lanzar. Las filas de la tabla salen de dentro hacia fuera. Con el cuerpo vacío javac no abre
   rango ni manejador para el recurso más interno: se emite solo la guarda `if (r != null) r.close()`
   y el `return`. Medido byte a byte con uno, dos y tres recursos, con un local propio en el cuerpo y
   con el cuerpo vacío; `d8 --release` acepta el `.class` de tres recursos con `rc 0`.

4. **Lo que salió al falsar el cierre.** La primera auditoría adversarial encontró tres casos Java
   válidos que fallaban (`String.format`, TWR con local en el cuerpo, TWR con cuerpo vacío); la
   segunda destapó el mismo boxing en el receptor explícito, en la llamada estática cualificada por
   el tipo y en la asignación a un arreglo de referencias. Los cinco están cerrados con su test byte a
   byte: `TestBoxingHaciaObjectoEnVarargs`, `TestReceptorExplicitoConBoxing`,
   `TestTryWithResourcesHuecoDelCuerpo`, `TestTryWithResourcesCuerpoVacio` y el caso
   `estatico_cualificado` de `TestReceptorExplicitoConBoxing`.

```sh
cd apkc && APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar go test -count=1 -run 'TestVarargs|TestBoxingHaciaObjectoEnVarargs|TestReceptorExplicitoConBoxing|TestTryWithResources' -v .
tcr -c 2 -m 2G -r 400M -- env GOGC=50 GOMAXPROCS=2 APKC_ANDROID_JAR=$ANDROID_HOME/platforms/android-34/android.jar go test -count=1 ./...
GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh wasm
GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh gate
```

**Resultado**: los tests de las cuatro piezas pasan; la suite completa da `ok
github.com/ajinasokan/apkc 50.579s`, la suite del hospedador de navegador da `PASS` y el gate da
`e2e: todo en orden` con `pestaña: 87 comprobaciones`, sin cambiar el número de comprobaciones ni
los apks de las plantillas. Los casos atacados por la auditoría que antes fallaban salen ahora byte a
byte: `String.format("%d %s", 1, "x")`, `s.f(7)` sobre `f(Object)`, `S.f(7)`, `a[0] = 7` sobre
`Object[]`, tres recursos, local propio y cuerpo vacío.

**Frontera**: siguen rechazándose por nombre el `try-with-resources` con `catch` o `finally`
propios y con salidas abruptas del cuerpo, el constructor varargs que necesita empaquetado, un `...`
que no es el último parámetro (JLS 8.4.1) y los varargs genéricos. No se midieron varargs privados
accesibles desde una anónima ni el comportamiento en ART de los dex nuevos; la igualdad byte a byte
y `d8` son los oráculos de este párrafo, no el aparato.
## 200. El falso error de `R.layout.main` en el diagnóstico en vivo

Medido el 2026-11-22. El usuario reportó que, escribiendo `setContentView(R.layout.main);` en
`MainActivity.java`, el diagnóstico en vivo de la pestaña decía *el tipo de este argumento no esta
medido*. §199 cierra la deuda Java (varargs y try-with-resources); este párrafo cierra el
diagnóstico en vivo.

**Causa medida, no la de la pista.** `apkc check` compilaba **un** fichero contra el `android.jar`,
y la `R` no viaja en el jar: `resolverSimple("R")` falla, y `tipoEstimado` (`apkc/javaemisor.go`)
tapaba ese fallo con un `linea %d: el tipo de ese argumento no esta medido` genérico. El emisor
nunca estuvo mal: la plantilla completa, con su `R.java`, ya salía byte a byte igual que la de
javac en `TestEtapasDeBuildEnGo`; lo que faltaba era que el diagnóstico viera la misma `R` que ve
la compilación.

Antes y después, con el mismo fuente y el mismo `android.jar`:

```sh
cd apkc && go build -o /tmp/apkc .
# antes: el binario del commit anterior, en un árbol aparte
git archive HEAD~1 apkc | tar -x -C /tmp/antes && ( cd /tmp/antes/apkc && go build -o /tmp/apkc-antes . )
/tmp/apkc-antes check --android-jar "$ANDROID_HOME/platforms/android-34/android.jar" \
  /tmp/proy/src/com/hola/apkcstudio/MainActivity.java
/tmp/apkc check --android-jar "$ANDROID_HOME/platforms/android-34/android.jar" \
  --nombre MainActivity.java /tmp/proy/src/com/hola/apkcstudio/MainActivity.java
```

```
antes:   {"tipo":"check","fichero":"MainActivity.java","lineas":18,"emisor":false,"errores":[
         {"linea":15,"mensaje":"MainActivity.java: linea 11: linea 12: linea 15: linea 15:
          el tipo de ese argumento no esta medido"}]}
después: {"tipo":"check","fichero":"MainActivity.java","lineas":18,"emisor":true,
          "recursos":"R enlazada de res/","errores":[]}
```

Y un id que no existe ya no es un «no medido» sino un error con sitio: si se cambia
`R.layout.main` por `R.layout.noexiste` en ese mismo fuente,

```
{"linea":15,"columna":24,"posicion":"MainActivity.java:15:24",
 "mensaje":"al estimar el tipo de R.layout.noexiste, el campo noexiste no esta en
            com/hola/apkcstudio/R$layout ni en sus padres"}
```

**Lo que cambió, con su porqué.**

- `apkc check` tiene `--proyecto` (por defecto sube desde la carpeta del fichero hasta el
  `AndroidManifest.xml`): enlaza `res/` **en memoria** con el mismo enlazador de `build --linker go`
  y compila esa `R` junto al fuente; si el proyecto ya tiene la `R.java` que dejó un build, usa esa.
  Nunca escribe en el árbol. `--nombre` pone el fichero real en las posiciones (el temporal de la
  pestaña se llama `.apkc-check.java`), y `--sdk-version`/`--codename` dicen la versión del platform
  cuando la ruta del jar no la lleva, que es el caso de la pestaña (`/sdk/android.jar`): sin ellos el
  enlace en la pestaña fallaba con `la ruta del android.jar no dice la version del platform`.
- `pos` (`apkc/javafuente.go`) guarda la **columna** del token, contada en caracteres, y `sitio()`
  la escribe en los mensajes. La cadena de accesos lleva la posición de su cabeza (`R` en
  `R.layout.main`), que es lo que el editor subraya. `tipoEstimado`, `acceso`, `accesoEstatico` y
  `cadenaDeCampos` devuelven la causa real, con su sitio, en vez del mensaje genérico.
- `check` recorta las capas de posición al propagar (`MainActivity.java: linea 11: linea 12: …`) y
  emite por error `linea`, `columna`, `posicion` (`archivo:linea:columna`) y `mensaje`.
- La pestaña (`studio/diagnostico-vivo.js`) pinta `posicion` y subraya el token. Ahí hay dos
  conversiones: la columna de `apkc` está en **caracteres** (runes) y desde 1, y el editor corta en
  **unidades UTF-16** (un astral son dos), así que `indiceUTF16()` la traduce; y el ancho no es
  «columnas por ancho de carácter» —un tabulador llega a su parada de `tab-size: 4`—, así que
  `anchoDe()` mide el prefijo y el token con una sonda de la misma fuente. La marca lleva
  `data-columna` (runes), `data-utf16`, `data-izquierda` (píxeles del prefijo) y `data-ancho`.

**Coste, que es la línea base de la mejora D2.** Tiempos de reloj de `apkc check` en este
escritorio, mismo fuente de 18 líneas:

```
sin proyecto (solo el jar):                  22-23 ms
con --proyecto y la R.java del build delante: 22-25 ms
con --proyecto y res/ enlazado en memoria:    181-193 ms
```

El salto son los ~170 ms de releer y parsear `resources.arsc` del `android.jar` en cada invocación.
Dentro de la pestaña, el mismo `check` con `--proyecto` tarda **1,065 s** (lo imprime `apkc`) y el
diagnóstico enseña el resultado **1,6 s** después de la última pulsación (350 ms de debounce + la
pasada), medido en `prueba-pestana.js` (`--- 4c: roto en 1601 ms, sano en 1602 ms`). Ese coste es
lo que D2 tiene que bajar; aquí solo queda medido.

**Pruebas nuevas.** `apkc/check_test.go`: el fuente del studio con la raíz del proyecto da **cero
errores**; usa la `R.java` del build cuando existe; sin proyecto el error nombra `R.layout.main`,
cae en la línea 15 y en la columna de la `R`, y no repite la posición en el mensaje; un `res/` que el
enlazador no sabe escribir deja el informe con `sin R: …` en vez de un verde falso; no escribe nada;
la raíz sube hasta el manifiesto. `apkc/javaemisor_argumento_estatico_test.go`:
`setContentView(R.layout.main)` y un `R.id` como argumento de un constructor, dexados por este fork
y por **javac 17** (`--release 8 -g:lines,source`), dan el **mismo dex byte a byte**.
`TestCheckColumnaEnCaracteres` fija la convención con un emoji delante del token: la columna del JSON
es la de los caracteres, no la de los bytes ni la de las unidades UTF-16. En la pestaña,
`prueba-pestana.js` comprueba el caso completo: `posicion` con `archivo:línea:columna`, el subrayado
sobre el token (`data-columna`, `data-ancho`) y que el diagnóstico no escribe en la consola ni
cambia `#ultimo-codigo`; y dos casos de geometría que un ancho «columnas por carácter» no resuelve:
un astral antes del token (`/* \U0001F600 */ setContentView(R.layout.noexiste);`, donde el corte
viejo caía dentro del par subrogado y subrayaba un carácter) y dos tabuladores delante, donde la
marca tiene que caer en la parada real (`data-izquierda` medido) y no en `(columna-1) * ancho`. Ojo con el sondeo de esa prueba: el
resumen del diagnóstico es `1 error` y `sin errores`, así que buscar la palabra «error» casaba con
«Abre un fichero .java para ver los errores…» y cerraba la espera antes de tiempo; se espera la
forma del resumen, no la palabra.

**Gate.** Con el árbol de este cambio, con `PREFIX` propio y con puertos propios: en esta máquina
otro `servidor-estatico.js` tenía ocupado el 8795 y otra sesión comparte `$PREFIX/tmp`.

```sh
HOME_APKC_WEB=/home/leonisauro PREFIX=/home/leonisauro/.cache/apkc-web-diag \
CHROME_BIN=/usr/bin/chromium STUDIO_SERVIDOR_PORT=8805 STUDIO_PORT=9375 \
STUDIO_CREAR_PORT=8806 STUDIO_CREAR_CDP=9376 GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh gate
```

```
crear: todo en orden (35 comprobaciones)
pestaña: 94 comprobaciones, apk firmada dentro de chromium de 10285 bytes y verificada
...
e2e: todo en orden
```

Dos intentos anteriores no llegaron a verde por el entorno, no por el código. El primero, con
`FALLO: la pestaña con apkc.wasm` y `el android.jar de /android-jar respondió 404`: un
`studio/servidor-estatico.js` de otra sesión escuchaba en ese puerto, la página hablaba con él y sus
endpoints dinámicos (`/android-jar`, `/keystore-ejemplo`) no existen ahí. El segundo, con
`chromium no subio el debugger` y `apkc check no devolvió JSON`: el gate comparte
`$PREFIX/tmp/apkc-wasm.wasm` con los demás worktrees y esa pasada recogió un binario de otro árbol.
`prueba-pestana.js` ahora comprueba que el puerto HTTP y el de CDP estén libres antes de arrancar y
lo dice con su nombre (`el puerto 8795 ya está ocupado: usa STUDIO_SERVIDOR_PORT`), y el gate se
lanza con `PREFIX` propio para no compartir binarios ni fixtures.

**Fronteras escritas.** (1) Los errores del **parser** llevan línea pero no columna: su posición es
la de la sentencia, no la de un token, y darle columna exige tocar cada sitio que construye una
sentencia sin mover la `LineNumberTable`, que se compara byte a byte. (2) Si el `res/` del proyecto
usa algo que el enlazador de Go no escribe (un drawable XML, por ejemplo), no hay `R` y el informe
lo dice en `recursos`; el error que sale entonces es el de verdad, nunca un verde falso.

## 201. Vista previa del APK: fases 1 a 3

Medido el 2026-11-22. Cierra las fases 1 a 3 de `docs/plan-vista-previa.md` con su comando: el
comando nuevo del fork que lee un apk ya construido y devuelve el modelo de su primera pantalla
(`apkc/preview.go`), el intérprete Dalvik que ejecuta `onCreate` y una interacción
(`apkc/previewinterprete.go`) y la tercera vista de `studio/` que lo pinta en un canvas de solo
lectura (`studio/vista-previa.js`). Las cifras de abajo salen del apk de la plantilla Contador que
genera el propio gate, medido el 2026-11-22 sobre este árbol con Go 1.27.4.

```sh
APK=$TMP/studio-e2e-contador/com.hola.contador/build/app.apk
cd apkc && go build -o "$TMP/apkc-preview" .
"$TMP/apkc-preview" preview --sin-ejecutar "$APK" | wc -c          # 661
"$TMP/apkc-preview" preview "$APK" | wc -c                          # 897
"$TMP/apkc-preview" preview "$APK"                                  # el modelo, una línea de JSON
"$TMP/apkc-preview" preview --sin-ejecutar "$APK" | grep -c ejecucion # 0: la bandera no interpreta
APKC_FIXTURE_MALO=$TMP/apk-switch-roto.apk go test -count=1 \
  -run TestElPreviewRechazaUnSwitchDePayloadRoto .                  # deja el apk con el payload roto
"$TMP/apkc-preview" preview "$TMP/apk-switch-roto.apk"; echo rc=$?  # rc=2 y el error por su nombre
"$TMP/apkc-preview" preview --sin-ejecutar "$TMP/apk-switch-roto.apk" >/dev/null; echo rc=$?      # 0
cd ../shim && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .
node navegador-sim.js --montar "$APK:/app.apk" "$TMP/apkc-wasm.wasm" preview /app.apk
```

1. **Fase 1, el modelo del layout.** `apkc preview --sin-ejecutar` sobre la plantilla Contador
   devuelve **661 bytes** de JSON, `nodos: 3`, un `TextView` con `texto: "Pulsos: 0"` y un `Button`
   con `"Pulsar"`, rc 0. El id del framework que el layout trae como estilo sale sin resolver a
   propósito, con su referencia cruda: `"estilo": "@0x01030042"`. El lector del pool UTF-8 se mide
   contra los **100 bytes** del trozo `RES_STRING_POOL` que escribió aapt2 35.0.1, con `ñ` y 😀
   (el emoji como par sustituto CESU-8, tres bytes por unidad): `go test -run
   TestPreviewLeeElPoolQueEscribeAapt2 -v .` y el resto de `preview_test.go`, en verde en
   `go test -count=1 ./...`.
2. **Fase 2, el intérprete.** `apkc preview` sobre el mismo apk devuelve **897 bytes**: 3 nodos y
   un bloque `ejecucion` con `pasos: 43`, `interacciones: 1` y una captura `TextView.setText` sobre
   `id/total` con `"Pulsos: 1"`, que además queda en el nodo como `textoDinamico`. El layout lo
   elige el id que el intérprete ve pasar por `setContentView`, no el nombre; `--sin-ejecutar`
   devuelve el modelo de la Fase 1 y ni siquiera la clave `ejecucion`. Un dex cuyo payload de
   `packed-switch` no cabe en su propio cuerpo **se rechaza**, no se ejecuta a medias: rc 2 con
   `el classes.dex no es ejecutable: en com.hola.contador.Casos.valor: payload de packed-switch
   truncado: declara 100 brazos y necesita 216 palabras, el cuerpo tiene 20`, sin pánico, y la misma
   apk con `--sin-ejecutar` sigue leyéndose con rc 0.
3. **Fase 3, la vista.** La barra de actividad tiene **tres** vistas (Inicio, Editor y Vista), el
   canvas lleva `role="img"` y no es arrastrable ni editable, y el gate cierra con **98
   comprobaciones** de pestaña y `e2e: todo en orden`. Al compilar, el canvas pinta el modelo
   (`3 nodos` en `#vista-previa-estado`) y no ensucia la consola ni `#ultimo-codigo`; un `preview`
   forzado a fallar deja el panel con su motivo y **no toca** ni la consola ni el último código. El
   texto de la app se pinta con `ctx.fillText` y en el DOM solo se escribe con `textContent`
   (`studio/vista-previa.js:259`, `:169`), así que el HTML de un layout no puede inyectar marcado.
   La recarga offline sirve `vista-previa.js` desde el service worker y repinta el último modelo.
4. **Lo que cuesta el binario.** Sobre compilaciones `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build`
   sin `-s -w` (la forma que documenta el gate), del árbol sin vista previa (`eef01b9`) al que trae
   el lector y el comando (`32851f4`) el wasm crece **+73 884 B** y su gzip `-9` **+21 897 B**; del
   árbol sin intérprete (`c40efc9`) al actual (`4d6dc89`) crece **+365 037 B** y **+77 833 B**. El
   binario final que empaqueta `tools/entorno.sh wasm` (con `-s -w`) mide **13 815 947 B** y
   **3 621 793 B** comprimido.

```sh
git archive eef01b9 apkc | tar -x -C "$TMP/base-preview"
( cd "$TMP/base-preview/apkc" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$TMP/wasm-0.wasm" . )
# ídem con 32851f4 y c40efc9, y el árbol actual: $TMP/wasm-1.wasm, $TMP/wasm-2.wasm, $TMP/wasm-3.wasm
for f in "$TMP"/wasm-0.wasm "$TMP"/wasm-1.wasm "$TMP"/wasm-2.wasm "$TMP"/wasm-3.wasm; do
  stat -c %s "$f"; gzip -9 -c "$f" | wc -c
done
( cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -ldflags='-s -w' -o "$TMP/wasm-final.wasm" . )
stat -c %s "$TMP/wasm-final.wasm"; gzip -9 -c "$TMP/wasm-final.wasm" | wc -c
```

**Resultado**: gate rc=0 con `pestaña: 98 comprobaciones` y `e2e: todo en orden`; el comando del
preview sale rc 0 en el hospedador con forma de navegador (`[apkc] finished in` 37,7-43,1 ms en
cinco ejecuciones) y en nativo; `go test -count=1 ./...` de `apkc/` en verde, que incluye el lector
del pool de aapt2, el intérprete, el rechazo del payload truncado y el modelo del apk. Frontera: el
layout se elige por `--layout` y, sin él, por el id de `setContentView` o por el nombre `main`; el
`setContentView` solo aporta el id, nada más; los ids del framework no se resuelven a su nombre y
salen como `@0x01030042`; no hay `gravity` fino, imágenes, tema del sistema ni fuentes reales, y
los rectángulos son la aproximación declarada (`aproximado: true`), así que este `§` no afirma
render fiel. El intérprete no hace JNI, no carga `.so`, no admite multidex y rechaza las excepciones
por su nombre. El informe previo del builder dio +74 488 B y +18 198 B para la Fase 1: con este
árbol y este toolchain no se reproducen (dan +73 884 B y +21 897 B), y la cifra que vale es la
medida aquí. No se probó a propósito una inyección de HTML en un layout: se sostiene por
construcción (canvas y `textContent`), no por una sonda.

**Sobre el árbol integrado** (Fase 3 y §200 juntas), el gate se volvió a correr con `GOGC=50
GOMAXPROCS=2 sh tools/entorno.sh gate`: `vista previa: 3 nodos, res/layout/main.xml,
LinearLayout[TextView,Button], id/total con textoDinamico "Pulsos: 1"`, `pestaña: 105
comprobaciones` y `e2e: todo en orden`, rc 0. El número sube de 98 porque las comprobaciones del
diagnóstico con la vista previa abierta (§200) se suman a las de la vista; la primera integración
dio 104 bien y 1 mal por una expectativa que buscaba `linea 4` cuando el mensaje ya decía
`vivo.java:4`, y se corrigió aceptando los dos formatos (`studio/prueba-pestana.js`, commit
`66c2a6b` del submódulo). La medición de 98 es la del árbol de la Fase 4 antes de integrar §200;
la del árbol final es esta.

**Ampliaciones posteriores** (mismas órdenes, con `GOGC=50 GOMAXPROCS=2 sh tools/entorno.sh
gate`): la vista dividida del editor (separador, `Previa`, consola de log) y el autocompletado D3
subieron el gate a 123 y 115 comprobaciones en sus ramas; el modo «En vivo» dibuja el layout del
XML del editor sin mostrar su código y sin compilar (`pre === false`, la firma de píxeles cambia al
editar `android:text` con `#ultimo-codigo` intacto) y volvió a subirlo a 127. Sobre el árbol final,
con las dos piezas y la colocación del §201 juntas, el gate da **`pestaña: 137 comprobaciones`** y
`e2e: todo en orden`. El subrayado del diagnóstico no tenía desfase: la marca cae al final de la
línea (`top = padding + (línea-1)*lineHeight + lineHeight - 2`, medido con `lineHeight 19.5 px`);
solo el error de bloque sin cerrar se atribuía al token de fin de fichero y ahora apunta a la última
línea con contenido (`§200`). D3 indexa el jar por rangos: **1 354 339 de 26 361 808 bytes**, 3837
clases en ~50 ms y 0 peticiones; `Tab` entra en el undo tree. Fronteras del modo En vivo:
aproximación declarada (dp≈px, texto a 7 px/carácter), sin `ConstraintLayout`, estilos ni temas;
los eventos siguen solo en «Render (eventos)».


## 202. El producto entero: gate final, editor de layout y recorrido de punta a punta

Medido el 2026-11-22 sobre la rama `web` con HEAD del padre en el commit que trae este `§` y
submódulo `studio/` en `eff7852`. Cierra tres cosas a la vez: el gate rojo que dejó la fusión del
emulador, la integración del editor de layout (`4bc13cf`, del worktree `editor-layout`) y la pasada
de coherencia que el usuario pidió con «no hay una naturalidad en la funcionalidad total». El número
de comprobaciones que vale es el de aquí; los de `§201` y anteriores son los de su árbol.

### El gate final, con su comando

`PREFIX` y `TMP` propios y creados antes, puertos libres (los de los worktrees, no):

```sh
mkdir -p "$HOME/.cache/apkc-web-final/tmp"
tcr -c 2 -m 2G -r 400M -- env HOME_APKC_WEB="$HOME" CHROME_BIN=/usr/bin/chromium \
  GOGC=50 GOMAXPROCS=2 APKC_HARNESS="$HOME/.cache/apkc-web/apkc-harness" \
  PREFIX="$HOME/.cache/apkc-web-final" STUDIO_SERVIDOR_PORT=9001 STUDIO_PORT=9501 \
  STUDIO_NAV_PORT=9002 STUDIO_CREAR_PORT=9003 STUDIO_CREAR_CDP=9502 \
  sh tools/entorno.sh gate
```

Salida literal de las tres líneas que cuentan:

```text
  crear: todo en orden (35 comprobaciones)
  pestaña: 170 comprobaciones, apk firmada dentro de chromium de 10285 bytes y verificada
e2e: todo en orden
```

rc 0. Además de la pestaña: las cuatro plantillas verificadas con `apksigner`/`zipalign`/`aapt2`,
release con `CN=Release de prueba` (v1+v2+v3), `.idsig` de la v4 aprobado por `apksigner` y por
`apkc verify`, `.aab` firmado como JAR v1 sin bloque v2/v3, rotación v3.1 con linaje y v4 de dos
firmantes, EC P-256/P-384, la v1 SHA-1 y el icono PNG literal. `zipcheck.py` corrió porque
`APKC_HARNESS` apunta a un clon existente; sin él `e2e.sh` avisa y omite ese oráculo.

### Por qué el gate estaba rojo, con la línea que lo rompía

La fusión del emulador (`e929a7c`, `6f456c8`) convirtió `pintarRender()` en un bucle
`for (const superficie of SUPERFICIES)` y dejó el `tail` de la versión anterior fuera del bucle:
`const canvas` quedó declarado dentro y se usaba fuera.

```text
ReferenceError: canvas is not defined
    at pintarRender (http://127.0.0.1:9001/previa-panel.js:1013:34)
    at Object.iniciar (http://127.0.0.1:9001/previa-panel.js:1291:5)
    at HTMLDocument.<anonymous> (http://127.0.0.1:9001/pestana.js:2763:18)
```

Lo cazó una sonda CDP (`Runtime.enable` + `Runtime.exceptionThrown`) sobre la página recién cargada,
con el mismo arnés que `studio/prueba-crear.js`. La traza es del árbol **anterior** a `9512567`: en el
árbol de este `§` esa línea ya no existe. La consecuencia no era el panel: la excepción
abortaba `inicializar()` en `pestana.js:2763`, así que no se cableaban árbol, subida, pestañas ni
keystore (todos registrados después) y `documentoActivo()` devolvía `null`:

```text
FALLO: excepcion en la pagina: TypeError: Cannot read properties of null (reading 'hist')
    at <anonymous>:11:42            # studio/prueba-crear.js:508
```

Se reproduce con `APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" node studio/prueba-crear.js` sobre el árbol
anterior a `9512567`. El arreglo: pintado y `canvas.__espacio` dentro del bucle, uno por superficie,
más `PREVIA_PANEL.iniciar()` en `try/catch` para que un fallo del panel no deje sin enganchar el
resto del editor.

### El editor de layout, integrado

`git merge --no-ff tmp-layout-final` sobre el árbol que ya tenía emulador y autocompletado. Cuatro
ficheros con conflicto, resueltos combinando: `previa-panel.js`, `prueba.js`, `AGENTS.md` y
`EDITOR.md`. Dos hallazgos de la fusión, los dos en `previa-panel.js`:

1. **`arrastre` estaba ocupado.** Los tiradores del marco ya usaban `let arrastre`; el modo editar
   traía otro con el mismo nombre, y compartirlo habría movido el nodo al arrastrar el tirador y el
   marco al arrastrar un nodo. El del nodo pasa a `arrastreNodo`.
2. **el contorno se pinta en las dos superficies.** `pintarVivo()` recorre `SUPERFICIES` y dibuja la
   selección con la caja de cada canvas, no solo en el del editor.

Comprobado por las comprobaciones que el gate cuenta en la pestaña, entre ellas: «el lápiz enciende
el modo editar (paleta y estado)», «arrastrar una vista dentro de un FrameLayout escribe su margen y
mueve el dibujo», «Supr borra el nodo seleccionado del XML y del dibujo, y el undo tree lo
devuelve», «clic selecciona con contorno y Esc lo suelta sin tocar el texto» y «cambiar a Emulador
apaga el modo editar y no deja la paleta abierta».

### La pasada de naturalidad, con lo que se midió

Se recorrió el producto en chromium con el arnés CDP de `prueba-pestana.js` (crear → editar Java y
XML → diagnóstico → autocompletado → compilar → vista previa → emulador con un clic → Pantalla →
modo editar y añadir → redimensionar). Las capturas no se versionan (regla 2), así que lo que queda
como evidencia es lo que se puede comprobar con un comando o una comprobación del gate:

1. **El estado se quedaba en «compilando».** `compilarCon` escribía `nota("compilando …")` y nunca
   el resultado. Ahora:

   ```sh
   grep -n 'la compilación falló\|en build/app.apk, firmado' studio/pestana.js
   # 2603:  if (!r || r.codigo !== 0) nota("la compilación falló; el motivo está en la consola");
   # 2604:  else nota("apk " + tipo + " en build/app.apk, firmado; consola con el detalle");
   ```

2. **El aviso de crear pedía lo que ya había hecho.** Decía «abre un fichero del árbol y edítalo»
   cuando el manifiesto ya estaba abierto en el Editor; ahora lo dice (y `prueba-navegador.js` lo
   imprime: `aviso: proyecto creado: manifiesto, layout y MainActivity, abiertos en el Editor`).

3. **Dos nombres para el mismo modo.** La línea de estado del emulador decía «Vista previa
   (aproximada) de …» mientras la pestaña activa se llamaba Emulador; ahora dice «Emulador
   aproximado de …». Y la columna de Pantalla avisa de que habla del emulador.

4. **La compilación de la pestaña se caía con el SDK del escritorio.** `servidor.js` servía el
   `platforms/` más nuevo; con SDK 36, `android.jar` trae la tabla de recursos en forma rala y el
   enlazador en Go no la lee:

   ```sh
   cd "$TMP/studio-e2e-java/com.hola.apkcstudio"
   "$APKC" build --linker go --compiler go --android-jar "$HOME/opt/android-sdk/platforms/android-36/android.jar" \
     --sdk-version 34 --codename 14 --keystore "$HOME/.android/debug.keystore"
   # rc=1
   # [build] la tabla de ids del framework: el tipo 1 viene en tabla rala (flags 0x02): esa forma no
   # esta medida; usa un android.jar de API 34, cuya tabla es densa
   ```

   `servidor.js` ahora prefiere `android-34` (el mismo criterio que `tools/sdk34.sh` en el gate) y el
   error dice qué jar sirve. Medido con el servidor de desarrollo:

   ```sh
   cd studio && APKC_WASM="$APKC_WASM" node servidor.js 9006 &
   node -e 'fetch("http://127.0.0.1:9006/android-jar-info").then(r=>r.text()).then(console.log)'
   # {"disponible":true,"version":34,"bytes":26361808}
   ```

5. **La paleta del modo editar** pone su título en su propia fila; los cinco tipos ya no se mezclan
   con «Añadir» en el mismo renglón.

6. **Vocabulario de las pruebas.** Dos comprobaciones seguían llamando «Render (eventos)» al
   Emulador y «vista previa» a Pantalla; se renombraron (no cambian lo que comprueban).

**Fronteras de este `§`.** (1) Las capturas de pantalla del recorrido no se versionan y no son
citable; lo que sostiene la pasada son las comprobaciones del gate y los comandos de arriba. (2) El
modo editar no reparenta nodos, no conoce `ConstraintLayout`, estilos ni temas, y en un
`LinearLayout` mover es reordenar (no hay posición libre): está en `EDITOR.md`. (3) La vista
**Pantalla** sigue siendo de solo lectura a propósito: el lápiz vive en el panel del editor. (4) No
se probó con un SDK que solo tenga 35/36 instalado: ahí la compilación de la pestaña falla con el
mensaje de tabla rala, que es una frontera del enlazador, no un defecto de la interfaz. (5) De los dos
planes abiertos, el incremental tiene I1 e I2 hechas, I3 cerrada en `d3803bb`, I4 hecha en nativo con
el gate pendiente (en verificación) e I5 medida por debajo del criterio (`docs/plan-incremental-web.md`);
el `android.jar` propio tiene generador validado en local (`tools/androidjar/`, J2 y J3) pero su
workflow no se ha ejecutado en GitHub (J4 y J5): este `§` no afirma que ese job esté verde.
## 203. El emulador enlaza `android:onClick` y separa sus tres secciones

Medido el 2026-11-22 sobre `ee06bf8` (submódulo `studio/`) y `19ec67a` (padre), con Go 1.27.4 y
chromium 156. Cierra dos cosas que el usuario vio en la página: la nota «Esta ejecución no registró
ningún OnClickListener sobre una vista con id» en una app tipo contador y que el registro, la consola
de la app y los controles de pulsar fueran tres bloques apilados.

**El camino que fallaba** era el clic declarado en el layout (`android:onClick="sumar"` y
`public void sumar(View v)` en la actividad): `onCreate` no llama a `setOnClickListener`, así que
`apkc preview` no publicaba ningún oyente y no había nada que pulsar. El camino de la plantilla
**Contador** (`findViewById(...).setOnClickListener(new View.OnClickListener(){…})`) ya funcionaba y
sigue funcionando; esta medición mide los dos por separado y no afirma cuál usó el usuario, porque no
se midió su proyecto.

```sh
export APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar
cd apkc && go build -o "$TMP/apkc-preview" .

# el apk del Clicker declarativo se monta en memoria y la sonda lo deja en disco
APKC_FIXTURE_DECL="$TMP/clicker-declarativo.apk" \
  go test -count=1 -run TestElPreviewEnlazaElOnClickDeclarativoDelLayout -v .
APKC_FIXTURE_DECL=  # sin la variable no escribe nada; la prueba sigue siendo la misma

"$TMP/apkc-preview" preview "$TMP/clicker-declarativo.apk"                       # 810 B, 3 nodos
"$TMP/apkc-preview" preview --pulsar id/pulsar "$TMP/clicker-declarativo.apk"
"$TMP/apkc-preview" preview --pulsar 0x7f010000 --pulsar 0x7f010000 "$TMP/clicker-declarativo.apk"

# el camino de la plantilla Contador, con el apk que deja el gate
APK=$TMP/studio-e2e-contador/com.hola.contador/build/app.apk
"$TMP/apkc-preview" preview "$APK"
"$TMP/apkc-preview" preview --pulsar id/pulsar "$APK"
```

1. **Reposo.** El apk declarativo sale con **810 bytes**, `nodos: 3`, `oyentes:
   [{"id":"id/pulsar","idRef":"0x7f010000"}]` y el `Button` con `oyente: true`. El `TextView` no
   tiene `textoDinamico`: `onCreate` corrió y no se pulsó nada.
2. **Un clic y dos.** `--pulsar id/pulsar` deja `pulsaciones: ["id/pulsar"]`, `interacciones: 1` y
   `textoDinamico: "Pulsos: 1"`; dos `--pulsar 0x7f010000` dejan `interacciones: 2` y
   `"Pulsos: 2"`, porque la lista entera se repite dentro de la misma ejecución.
3. **El camino Contador, para no confundirlos.** El apk del gate publica el mismo oyente
   (`id/pulsar`, `0x7f010000`) y un clic da `"Pulsos: 1"` con la consola
   `Log.d "pulso 1"` (secuencia 1) y `println "contador: pulso 1"` (secuencia 2). Ninguno de los dos
   caminos se apoya en el otro: uno registra el oyente en `onCreate`, el otro lo enlaza `apkc
   preview` al leer el layout, como el inflado del framework.
4. **La prueba se salta sin jar.** `jarDelSDK` busca `APKC_ANDROID_JAR`, después
   `$HOME/Android/platforms/android-34/android.jar` y después `/sdk/android.jar`; en un escritorio
   con el SDK en `~/opt/android-sdk` **sin la variable la prueba se omite** (`t.Skip`), así que el
   comando de arriba la incluye. `go test -count=1 -run
   TestElPreviewEnlazaElOnClickDeclarativoDelLayout -v .` en verde (0,05 s).
5. **Las tres secciones.** El panel del emulador ya no apila pulsar, registro y consola: son
   pestañas (`#previa-seccion-pulsos|registro|consola`) con `role="tab"`, `aria-selected` y
   `aria-controls`; el panel oculto sigue en el DOM (alternar conserva sus líneas: `logLineas: 6`,
   `consolaLineas: 4` antes y después) y la elegida se guarda en `apkc-studio/previa-seccion`. La
   vista **Pantalla** repite el patrón con ids propios (`#vista-seccion-*`, `#vista-pulsos`,
   `#vista-log`, `#vista-consola`). El clic se entrega en **las dos superficies**:
   `#previa-lateral-canvas` y `#vista-previa` (antes el canvas de Pantalla no tenía manejador), con
   la geometría del último pintado (`canvas.__espacio`).
6. **La recarga, medida de verdad.** `prueba-pestana.js` elige Registro en el panel y Consola en
   Pantalla, navega con `Page.navigate`, espera a que arranque la página y comprueba que
   `conectarSecciones()` restaura las dos: `guardadas {panel:"registro", vista:"consola"}`, paneles
   `registro:true/pulsos:false/consola:false` y `consola:true` con su pestaña activa. Después vuelve
   a compilar para repoblar el emulador desde el apk de OPFS (`oyentes: ["id/pulsar"]`).
7. **Gate.** Con el árbol final y la comprobación de recarga dentro,
   `pestaña: 177 comprobaciones` y `e2e: todo en orden` (eran **175** cuando entraron las secciones y
   el Clicker; la recarga suma las dos restantes), sin «sin chromium» ni «sin APKC_WASM». La sonda
   del § es la de un PREFIX nuevo, puertos 8834/9840/8833.

**Fronteras de este `§`.** (1) `android:onClick` se resuelve **solo** en el layout de la actividad
lanzadora: no se siguen `include`, ni `menu`, ni el contexto de un Fragment o un servicio, y la firma
tiene que ser `(Landroid/view/View;)V`. (2) Si el método no existe o la vista no tiene
`android:id`, no se inventa el oyente: queda un aviso en el registro del emulador (probado en la
misma prueba). (3) Las **lambdas Java siguen fuera del subconjunto** del compilador propio: `v ->
{…}` se rechaza al compilar (`">" no es una expresion que este parser reconozca`), medido; el
patrón anónimo, `this`, la clase interna y `android:onClick` sí se ejecutan. (4) No se midió el
proyecto del usuario: se miden los dos patrones por separado y la medición no dice cuál usó. (5) El
sitio publicado (**https://apkc-studio.surge.sh/**) sigue con la build anterior: estos cambios no se
han empaquetado ni publicado (`tools/entorno.sh sitio` + `surge . publish` quedan pendientes). (6)
Entrar en Pantalla repinta el último modelo y un modelo que no viene de un clic reinicia la lista de
pulsos: el primer toque en el teléfono grande arranca en «Pulsos: 1» aunque antes se hubiera pulsado
en el panel.

## 204. Árbol reconciliado: estado real del gate y del `android.jar`

Medido el 2026-11-22 sobre `web` @ `2c6d4a9`, tras integrar uno por uno el Clicker (§203), el
incremental (`--inc`), el plan de dependencias y la corrección documental del workflow del jar.

```sh
mkdir -p $HOME/.cache/apkc-web-clicker-final/tmp
HOME_APKC_WEB=$HOME CHROME_BIN=/usr/bin/chromium GOGC=50 GOMAXPROCS=2 \
  APKC_HARNESS=$HOME/.cache/apkc-web/apkc-harness \
  PREFIX=$HOME/.cache/apkc-web-clicker-final \
  STUDIO_SERVIDOR_PORT=9041 STUDIO_PORT=9541 STUDIO_NAV_PORT=9042 \
  STUDIO_CREAR_PORT=9043 STUDIO_CREAR_CDP=9542 \
  tcr -c 2 -m 2G -r 400M -- sh tools/entorno.sh gate
```

**Resultado**: `crear: todo en orden (35 comprobaciones)`, `pestaña: 184 comprobaciones`,
`e2e: todo en orden`, rc 0. Suite nativa `ok github.com/ajinasokan/apkc 54.149s` y suite wasm
`PASS` (274 ficheros en memoria simulada). `git merge-base --is-ancestor origin/web HEAD` da 0 y
`git rev-list --count HEAD..origin/web` da 0: el rebase no perdió los 9 commits remotos.

**Qué usa la página para compilar**: el `android.jar` de la SDK, **no el generado**.
`curl http://127.0.0.1:8790/android-jar-info` → `{"disponible":true,"version":34,"bytes":26361808}`
y `studio/pestana.js:2052` fija `RUTA_JAR = "/sdk/android.jar"`, que el servidor sirve desde el
SDK local. El jar generado por `tools/androidjar/` (run
<https://github.com/Leonisaurov/apkc-web/actions/runs/45062792689>, artifact
`e2ddfb55…c8827` / `817171a3…41952c`) se ha usado como oráculo de `apkc compile`/`build` y en la
comprobación J5 del autocompletado (171 comprobaciones en local), pero **no está cableado en la
carga normal de la pestaña**. J5 en sitio publicado sigue pendiente; el sitio sigue con la build
anterior.

**Frontera**: este párrafo no afirma CI verde aparte del run real citado, ni render fiel, ni
paridad con ART en dispositivo; el incremental queda con I1/I2 hechas e I3 hecha en parte, I4 e I5
pendientes.

