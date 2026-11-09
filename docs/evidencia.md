# Evidencia

Todo lo de aquí se midió en este teléfono (Termux/Android aarch64, Chromium y node 26 locales).
Las mediciones del 2026-10-26 se hicieron contra `../apkc`; desde 2026-10-27 el código de la
herramienta es un fork en `apkc/` de este repo (rama `web`), así que los `cd ../apkc` que aparecen
abajo son el recordatorio de cómo se obtuvo cada número, no la ruta de hoy.
Cada bloque lleva el comando que lo produce. Fecha de todas las mediciones: **2026-10-26**.

## 1. El código puro de apkc corre en un hospedador con forma de navegador

**Comando**

```sh
cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$PREFIX/tmp/apkc-test.wasm" .
cd shim && node navegador-sim.js "$PREFIX/tmp/apkc-test.wasm" -test.v
```

**Resultado**: `PASS`, salida 0, **88 tests**. Entre ellos los que importan para esta ambición, que
son exactamente los que no lanzan ningún programa: `TestStoredEntryKeepsItsBytesAndCrc`,
`TestHeadersOfAWrittenEntryAgreeWithItsDirectoryRecord`, `TestBundleEntriesWithoutSigningStayLegal`,
`TestPKCS7SignatureVerifiesFromItsOwnBytes`, `TestSignatureFileCoversTheNamesTooLongForOneLine`,
`TestPaddingAfterLandsDataOnTheBoundary`, `TestBagAliasReadsWhatEachWriterSpells`, los de JKS y
`TestAndListPutsTheLastPieceBehindAnd`.

`navegador-sim.js` **no** le da a Go el `fs` de Node: le da un `fs` en memoria con la misma forma.
Ese es el punto: lo que funciona aquí funcionaría en un navegador con un hospedador equivalente
(OPFS o IndexedDB detrás de la misma API).

## 2. `apkc create` entero, del lado del cliente

```sh
node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp
```

17 nodos escritos en memoria (manifest, `src/com/myapp/MainActivity.java`, `res/layout/main.xml`,
`res/values/styles.xml`, `.gitignore`, `lib/<abi>`), salida 0, **256 ms**. Las plantillas viajan
dentro del binario por `//go:embed`, así que no piden red ni disco.

## 3. Lo que se rompió al montar el hospedador, y por qué no era de apkc

Cuatro huecos del shim, cada uno con el síntoma que dio. Sirven de checklist para quien escriba el
hospedador de navegador de verdad:

| Qué faltaba | Síntoma | Quién lo provocaba |
| --- | --- | --- |
| `fs.writeSync` | `TypeError: fs.writeSync is not a function` dentro de `runtime.printpanics` | el **runtime de Go** escribe su salida con la API síncrona; la de syscalls es asíncrona y son cosas distintas (`wasm_exec.js:255`) |
| Campos `dev`, `ino`, `nlink`, `rdev`, `blksize`, `blocks` en el objeto `stat` | `panic: syscall/js: call of Value.Int on undefined` dentro de `os.MkdirAll`, desde `create.go:37` | `setStat` (`src/syscall/fs_js.go:183`) copia los doce campos de Node; con uno menos panica el runtime y parece un fallo de apkc |
| `/tmp` en el árbol | `TempDir: stat /tmp: No such file or directory` en tres tests de firma | `testing.T.TempDir()` |
| `HOME` en `go.env` | `[doctor] $HOME is not defined` y salida 1 | `findSDKs` lee el entorno |

**API mínima que Go pide al hospedador** — las 24 funciones, sacadas de los `fsCall(...)` de
`src/syscall/fs_js.go`, no inventadas: `chmod chown close fchmod fchown fstat fsync ftruncate
lchown link lstat mkdir open read readdir readlink rename rmdir stat symlink truncate unlink write`
— más `fs.constants` (`O_WRONLY O_RDWR O_CREAT O_TRUNC O_APPEND O_EXCL O_DIRECTORY`),
`path.resolve`, y `process.{cwd,chdir,getuid,getgid,geteuid,getegid,getgroups,umask}`.

## 4. El límite duro: `build` no puede existir del lado del cliente hoy

```sh
node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" build     # en un proyecto real
# en node, con el SDK a la vista:
GOOS=js GOARCH=wasm $PREFIX/tmp/apkc-wasm.wasm build              # mismo resultado
```

Imprime `[build] compiling resources` y muere. La causa es que **`aapt2` es un proceso externo** y
`js/wasm` no tiene `fork/exec` (por eso también `watch` se autoclora: `proc_stub.go` lleva el texto
`this build of apkc starts no external program, so there is nothing to rebuild on a change`).

El fallo además **no decía nada**: el patrón `LogF("build", string(out))` descartaba `err` en seis
sitios (`build.go` en `compileRes`, `bundleRes`, el de `kotlinc`, el de `javac` y el de `d8`;
`apk.go` en `alignAPK` y `signAPK`; `aab.go` en `runOrAbort`), y como el programa nunca arrancó,
`out` estaba vacío: la línea que veía el usuario era `[build]  `.

**Arreglado** en apkc `e7bfd01` (2026-10-26). El mismo comando, con el mismo binario wasm, ahora
responde el motivo que solo estaba en `err`:

```
[build] compiling resources
[build] aapt2 compile never started: pipe: not implemented on js
```

`pipe: not implemented on js` es la respuesta exacta de `os/exec` bajo `js/wasm`: no es que falte
`aapt2`, es que **el hospedador no puede crear la tubería con la que se habla de un proceso**. Ese
es el muro real de esta ambición, y ahora lo dice el propio apkc.

## 5. Fricción del entorno (Termux), para no perder tiempo dos veces

- `go test -exec=$GOROOT/lib/wasm/go_js_wasm_exec` falla con
  `total length of command line and environment variables exceeds limit` **antes de correr un solo
  test**: el lanzador pasa el entorno entero al arrancar node. Hay que envolverlo en `env -i`. No es
  una regresión de apkc.
- Sin `GOOS=js GOARCH=wasm`, `go test -exec=...wasm_exec` compila un ELF y el shim se queja de
  `expected magic word 00 61 73 6d, found 7f 45 4c 46`.

## 6. Cuánto cuesta hoy cada etapa (para saber qué merecería la pena portar)

Medido sobre el proyecto de prueba que genera la página, con `tcr -j 1` y el backend Go
(`--signer=auto`, que en este proyecto elige Go):

| Etapa | Coste | ¿Proceso externo? |
| --- | --- | --- |
| `aapt2 compile` + `aapt2 link` | 373-377 ms | sí |
| `javac` (2 ficheros) | 3.9-4.0 s | sí |
| `d8 app` | ~5.0 s | sí |
| empaquetar + firmar (v1+v2) | dentro de apkc | **no** |
| ronda idle bajo `--inc`, sin cambios | 2.9-5.6 ms | no |
| total de un build trivial | 9.4-9.6 s | |
| variante Kotlin | +26.4 s de `kotlinc` (y ruido de `jansi` por stderr) | sí |

La lectura honesta: **la firma ya es Go**, así que no hay nada que portar ahí; el peso del build
está en `javac` y `d8`, que son precisamente las dos piezas que no se pueden sustituir en Go sin
escribir un compilador.

## 7. `apkc sign`: la firma como comando suelto, con los tres oráculos (apkc `a909214`)

Es el paso que convierte "la firma es Go" en "la firma se puede pedir desde una pestaña". Dentro de
`build` no existía como operación: había que pasar antes por `aapt2`, `javac` y `d8`.

```sh
sh -c 'cd ../apkc && go build -o "$PREFIX/tmp/apkc-sign" .'
# un apk firmado por el apksigner de verdad, no por apkc:
sh -c 'cd ../apkc && go build -o "$PREFIX/tmp/apkc-gate" .'
mkdir -p "$PREFIX/tmp/sign-probe" && cd "$PREFIX/tmp/sign-probe" && $PREFIX/tmp/apkc-gate create app
cd app && tcr -j 1 -q -- $PREFIX/tmp/apkc-gate build --signer=jvm
$PREFIX/tmp/apkc-sign sign --out ../apkc-signed.apk build/app.apk
```

**Resultado** (apk de 9504 bytes, 4 entradas de contenido). Comandos y respuesta de cada oráculo:

| Oráculo | Comando | Respuesta |
| --- | --- | --- |
| `apksigner` | `build-tools/34.0.0/apksigner verify --verbose --min-sdk-version 21 apkc-signed.apk` | `Verifies`, `v1 scheme: true`, `v2 scheme: true`, `Number of signers: 1`, rc 0 |
| `zipalign` | `build-tools/34.0.0/zipalign -c -p 4 apkc-signed.apk` | rc 0 (y `-c -v 4`: `Verification successful`) |
| `jarsigner` | `$PREFIX/lib/jvm/java-21-openjdk/bin/jarsigner -verify apkc-signed.apk` | `jar verified.` rc 0 |
| andamiaje estricto | `python3 ~/Develop/Go/apkc-harness/zipcheck.py apkc-signed.apk` | `7 entries, every record agrees with itself` |

Las advertencias de `jarsigner` ("self-signed", "no timestamp", "Manifest is missing when reading
via JarInputStream") **son idénticas en el apk firmado por `apksigner`**: se compararon las dos
salidas y no hay ninguna diferencia entre lo que dice el oráculo sobre el trabajo de apkc y sobre el
de la herramienta oficial. Eso es lo que hace servir `jarsigner` como oráculo y no como juez.

**Control negativo**, alterar un byte del contenido almacenado de `resources.arsc` (empieza en 1772,
lo dijo `zipalign -c -v`):

```sh
python3 - apkc-signed.apk tampered2.apk <<'EOF'
import sys
datos = bytearray(open(sys.argv[1], "rb").read()); datos[1772 + 5] ^= 0xFF
open(sys.argv[2], "wb").write(datos)
EOF
apksigner verify --min-sdk-version 21 tampered2.apk   # DOES NOT VERIFY: CHUNKED_SHA256 digest mismatch
jarsigner -verify tampered2.apk                       # SecurityException: SHA-256 digest error for resources.arsc, rc 1
```

Los dos esquemas detectan el byte: v2 por el digest troceado del fichero entero, v1 por el digest de
la entrada. `apksigner` corta al primer fallo y no llega a nombrar v1, así que la prueba de v1 se
hizo con el JDK.

**Coste**: 30 ms nativo para re-firmar el apk de 9504 bytes.

## 8. La firma dentro de wasm, en el hospedador con forma de navegador

El hospedador no podía ni recibir un fichero real ni devolver el resultado; ahora puede
(`--montar real:ruta`, `--exportar ruta:real`, y el código de salida de apkc es el del proceso).

```sh
sh -c 'cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
cd shim
node navegador-sim.js \
  --montar "$PREFIX/tmp/sign-probe/app/build/app.apk:/in.apk" \
  --montar "$HOME/.android/debug.keystore:/ks.p12" \
  --exportar "/out.apk:$PREFIX/tmp/sign-probe/wasm-signed.apk" \
  "$PREFIX/tmp/apkc-wasm.wasm" sign --keystore /ks.p12 --out /out.apk /in.apk
apksigner verify --verbose --min-sdk-version 21 "$PREFIX/tmp/sign-probe/wasm-signed.apk"
```

**Resultado**: `exportado /out.apk (9504 bytes)`, rc 0, **1,20 s dentro de wasm contra 30 ms
nativo**; `apksigner` responde `Verifies` con v1 y v2 `true`, `zipalign -c -p 4` sale 0 y
`zipcheck.py` no encuentra violaciones. **Un apk firmado enteramente dentro de un módulo wasm, sin
server, sin SDK y sin JVM, verificado por la herramienta oficial.** La diferencia de coste está en
la derivación de clave del keystore PKCS#12 (PBKDF2 en Go puro), no en el zip: el keystore se lee
una vez por firma.

Los 305 bytes en que difieren el apk firmado en wasm y el firmado nativo (`cmp -l`) son las fechas
DOS y los campos de cabecera de las tres entradas de `META-INF`, que se escriben con la hora de la
firma; las entradas de contenido son las mismas bytes.

La suite completa, en el mismo hospedador: **98 tests** (`--- PASS` 98, `--- FAIL` 0, rc 0).

## 9. Dos reglas de formato que se leyeron de un oráculo en vez de adivinarse

**Los directorios no van al manifiesto v1.** Un apk re-firmado a partir de un archivo que lista sus
directorios (algo que un plan de build no hace nunca, pero un `unzip` de cualquier sitio sí) podría
escribir secciones para `res/`. Se preguntó al JDK:

```sh
$PREFIX/lib/jvm/java-21-openjdk/bin/jar cf app.jar AndroidManifest.xml res/   # con res/drawable/ dentro
$PREFIX/lib/jvm/java-21-openjdk/bin/jarsigner -keystore ks.jks app.jar k
unzip -l app.jar | grep '/$'          # res/ y res/drawable/ siguen en el archivo
unzip -p app.jar META-INF/MANIFEST.MF # y no hay ninguna sección para ellos
```

`java.util.jar` los deja fuera. apkc hace lo mismo, con su test.

**El nombre del firmante no es fijo.** `apkc` escribe `META-INF/ANDROID.SF`; el `apksigner` de
build-tools 34 con el keystore de debug de este teléfono escribe **`ANDROIDD.SF`** (se ve en
`unzip -l` del apk del punto 7). Por eso al re-firmar se descartan las entradas de la firma anterior
**por sufijo** (`.SF .RSA .DSA .EC .MF`) y no por el nombre que apkc usa: si no, dos firmantes en un
mismo paquete y el verificador lo cuenta como corrupción en vez de como firma vieja.

## 10. Un fallo del hospedador que parecía de apkc

Sustituir `go.exit` por uno mismo (para conocer el código de salida) corta el `_exitPromise` que
`wasm_exec.js` usa para resolver `go.run()`: el `then` que exporta los ficheros nunca corre y, unos
segundos después, el temporizador de scheduling del propio `wasm_exec` salta con
`Error: Go program has already exited`. Con el `setTimeout` fijo de la versión anterior del shim el
síntoma era invisible. La corrección es resolver `go._resolveExitPromise()` desde el `exit`
sustituido; el texto de arriba (`--montar`/`--exportar` funcionando con rc 0 y resumen) es la prueba.

## 11. El hospedador de navegador DE VERDAD: chromium, OPFS y worker (no node)

`shim/navegador-sim.js` prueba la API; lo que faltaba era el navegador. `studio/` tiene ahora un
hospedador que corre en Chromium con los ficheros detrás de OPFS:

| Pieza | Qué es |
| --- | --- |
| `studio/servidor.js` | servidor http sin dependencias: da COOP/COEP (sin eso `crossOriginIsolated` es falso y el access handle sincrónico no existe), sirve `/apkc.wasm` desde la variable de entorno y `/wasm_exec.js` desde el GOROOT |
| `studio/fs-fragmento.js` | las 24 funciones de `fs_js.go` con la memoria por delante y OPFS detrás (escritura espejada, lectura perezosa) |
| `studio/hospedador.js` | `DedicatedWorker` de tipo módulo: recibe el mango de OPFS desde la página, da el `fs`/`path`/`process` a Go y ejecuta **un comando cada vez** |
| `studio/pestana.js` | editor de fuente (textarea + capa resaltada, sin dependencias) y los botones que hablan con el worker |
| `studio/prueba-pestana.js` | conduce chromium por CDP y **verifica por fuera** la apk que salió de la pestaña |

```sh
sh -c 'cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" \
  APK_EJEMPLO="$PREFIX/tmp/sign-probe/app/build/app.apk" \
  KS_EJEMPLO="$HOME/.android/debug.keystore" node prueba-pestana.js'
```

**Resultado: 16 comprobaciones, 0 fallos**, todas dentro de chromium headless (15 en la
corrida del 2026-10-26; la decimosexta es la de conexión, añadida el 2026-10-27 al exigirse "sin
recurso a terceros"):

1. `crossOriginIsolated` es `true` (lo mide la propia página).
2. `apkc create /proyecto` en wasm sale 0 y escribe `AndroidManifest.xml`,
   `src/com/myapp/MainActivity.java` y `res/layout/main.xml` — 5 ficheros en el árbol del worker.
3. El manifest que se lee de OPFS contiene `package="com.myapp"`: lo escribió Go, no lo simula la
   página.
4. Lo editado en el editor **llega a OPFS byte a byte** (el test compara el texto exacto).
5. El resaltador pinta palabra clave y comentario; los números de línea siguen al contenido.
6. **Tras `Page.navigate` (recargar de verdad) el fuente editado sigue ahí e idéntico.** Eso es OPFS
   frente a `localStorage`, y la razón de montar el worker.
7. `apkc sign` dentro de chromium: sale 0 en ~1 s y devuelve 9504 bytes con magic `PK`.
8. `apkc build` en wasm dice el muro con las palabras exactas que ya se conocían:
   `[build] aapt2 compile never started: pipe: not implemented on js`.

Y la apk que salió de la pestaña se verificó fuera, con los oráculos de siempre:

```sh
apksigner verify --verbose --min-sdk-version 21 "$PREFIX/tmp/pestana-apk/pestanera.apk"
# Verifies / v1 true / v2 true, rc 0
zipalign -c -p 4 "$PREFIX/tmp/pestana-apk/pestanera.apk"     # rc 0
jarsigner -verify "$PREFIX/tmp/pestana-apk/pestanera.apk"    # jar verified. rc 0
python3 ~/Develop/Go/apkc-harness/zipcheck.py "$PREFIX/tmp/pestana-apk/pestanera.apk"
# 7 entries, every record agrees with itself
```

**Hoy, en una pestaña y sin subir nada a ningún sitio: crear el proyecto, editar el fuente,
conservarlo entre sesiones y firmar una apk que `apksigner` acepta.** Lo que sigue sin poder hacer
en el cliente es `build`: mientras `aapt2`/`javac`/`d8` sean procesos, no hay dex ni recursos.

## 12. Cuatro cosas del navegador que se rompieron antes de funcionar

Cada una con su síntoma, para quien repita el camino:

| Qué | Síntoma en chromium | Quién lo causaba |
| --- | --- | --- |
| `navigator.storage` | `este navegador no expone OPFS`, dentro del worker | OPFS vive en el hilo principal; en el worker `navigator.storage` es **undefined**. La raíz se baja en la página y se pasa por `postMessage` (structured clone) |
| `globalThis.crypto = crypto` | `Cannot set property crypto of #<WorkerGlobalScope> which has only a getter` | en un worker de módulo `performance` y `crypto` ya son globales de solo lectura; hay que **no** tocarlos (el shim de node sí necesita ponerlos) |
| `FileSystemDirectoryHandle.entries()` | tras recargar, `/proyecto` era un **fichero de 0 bytes** y `apkc build` decía `mkdir build: Not a directory` | rinde `[nombre, mango]`, **no** `[nombre, "directory"]`: el tipo es `mango.kind`. Comparar la segunda posición contra una cadena convierte cada directorio en fichero |
| `createSyncAccessHandle()` | los ficheros se leían vacíos y nada quedaba espejado | es **asíncrona** (lo sincrónico son sus métodos: `read`, `write`, `getSize`, `flush`). Guardar la promesa sin `await` dejaba el acceso inservible y el `try/catch` se lo comía en silencio |

Se suman dos fallos del propio hospedador, ya corregidos: `sistema` se asignaba **antes** de importar
`wasm_exec.js`, así que un arranque a medias quedaba tapado y el aviso real aparecía veinte mensajes
después como `Go is not defined`; y un fallo de la cola de tareas se respondía como si fueran bytes,
lo que se convertía en un `TypeError` de `TextDecoder` sin nombre. Ahora el estado se asigna al final
del `iniciar()`, el motivo de un import fallido se guarda (lo cuenta el `sondeo`), y la cola responde
`{tipo:"error"}`.

**Tamaño de lo que hay que bajar a la pestaña** (medida parcial de la fase 4.4): `apkc.wasm`
9.458.093 bytes, más el `wasm_exec.js` del GOROOT. Sin `android.jar`, porque sin `javac` no hay nada
que compilar todavía.

## 13. Cuánto hay que emitir: el `classes.dex` de un proyecto real, despiezado

Decisión del 2026-10-27: **la compilación tiene que ocurrir entera en el navegador, sin conexión y
sin runtimes de terceros**. Caída la idea de prestar una JVM, lo único que queda es que **Go emita**
lo que hoy emiten `javac` y `d8`, así que la primera medición útil es *de qué tamaño es el objetivo*.

```sh
# el proyecto del studio, compilado con las herramientas reales (javac 4,3 s + d8 5,6 s)
sh -c 'APKC="$PREFIX/tmp/apkc-gate" sh studio/e2e.sh'
cd "$(dirname "$(find "$PREFIX/tmp/studio-e2e-java" -name classes.dex | head -1)")"
unzip -o -q build/app.apk && python3 - <<'EOF'
import struct
d = open("classes.dex", "rb").read()
print("magic", d[0:8], "| file_size", struct.unpack_from("<I", d, 32)[0])
for i, n in enumerate(["string_ids","type_ids","proto_ids","field_ids","method_ids","class_defs","data"]):
    print("   %-12s size=%-5d off=%d" % ((n,) + struct.unpack_from("<2I", d, 56 + i*8)))
map_off = struct.unpack_from("<I", d, 52)[0]
n, = struct.unpack_from("<I", d, map_off); o = map_off + 4
for i in range(n):
    t, unused, size, off = struct.unpack_from("<HHII", d, o); o += 12   # entradas de 12 bytes
    print("   mapa tipo=%#-8x size=%-4d offset=%d" % (t, size, off))
EOF
```

**Resultado sobre el proyecto completo del studio** (el de botón, `Toast` y menú): 6 ficheros
`.class` (3333 bytes) entran, **2844 bytes de `classes.dex`** salen, con 17 tipos de sección en el
`map_list` y estos recuentos de cabecera:

| Sección | Objetos | | Sección | Objetos |
| --- | --- | --- | --- | --- |
| `string_ids` | 53 | | `code_item` | 9 |
| `type_ids` | 23 | | `class_data_item` | 7 |
| `proto_ids` | 9 | | `string_data_item` | 53 |
| `field_ids` | 4 | | `type_list` / `type_item` | 8 / 8 |
| `method_ids` | 17 | | `debug_info_item` | 6 |
| `class_defs` | 6 | | anotaciones (`0x2005`, set, colección) | 3 / 5 / 6 |

**217 objetos en total**, fuera cabecera y mapa. La lectura honesta, que contradice una línea de este
mismo plan escrita el 2026-10-26 ("escribir un dexer en Go es otro proyecto de otra escala"): para el
subconjunto que necesita una app mínima **el objetivo es finito y medible**; lo que no es un
subconjunto es el desugaring de lambdas y de `invokedynamic` sobre dependencias ajenas, y eso sigue
siendo la frontera. Las 6 clases de entrada son `MainActivity`, `MainActivity$1` (la clase anónima
del botón) y `R`, `R$id`, `R$layout`, `R$style` — estas cuatro últimas las genera `aapt2`, así que el
dexer de Go no existe sin la Fase 3 de recursos.

Y una advertencia sobre el oráculo, medida aquí: el `map_list` del dex **no** tiene entradas de 16
bytes sino de 12 (`ushort tipo, ushort unused, uint size, uint offset`). Un parser que se equivoque
aquí lee 3486512 secciones y concluye cualquier cosa; a mí me pasó antes de escribir esto.

## 14. El subconjunto que hay que emitir, enumerado con `javap` (Fase 2G.1 y 1G.1)

La pregunta que decide si 2G/1G son semanas o un pozo sin fondo no se responde opinando: se responde
leyendo los `.class` que el proyecto del `studio/` ya produce con las herramientas reales.

```sh
C=$(find "$PREFIX/tmp/studio-e2e-java" -type d -name classes | head -1); cd "$C"
J=$PREFIX/lib/jvm/java-21-openjdk/bin
CLASES=$(find . -name '*.class' | sed 's|^\./||; s|\.class$||; s|/|.|g' | tr '\n' ' ')
$J/javap -v -c -p -classpath . $CLASES > "$PREFIX/tmp/javap-v.txt"
grep -oE '^ +#[0-9]+ = [A-Za-z]+' "$PREFIX/tmp/javap-v.txt" | awk -F'= ' '{print $2}' | sort | uniq -c | sort -rn
awk '/Code:/{d=1} d && /^ +[0-9]+: [a-z]/ {print $2}' "$PREFIX/tmp/javap-v.txt" | sort | uniq -c | sort -rn
```

**Resultado sobre las 6 clases del proyecto completo** (Activity, la interna del botón, `R` y sus
tres anidadas):

| Qué | Cuánto |
| --- | --- |
| clases | 6 (mayor version del `.class`: **65**, la que escribe este JDK por defecto; ver la corrección en §17) |
| tipos referenciados | 12 (`android/app/Activity`, `android/view/Menu`, `android/view/MenuItem`, `android/view/MenuItem$OnMenuItemClickListener`, `android/widget/Toast`, `java/lang/Object` y las 6 propias) |
| clases con `Code` | 9 métodos |
| instrucciones de bytecode en total | **47** |
| operaciones JVM distintas usadas | **16**: `aload_0` (11), `invokespecial` (8), `return` (7), `ldc` (3), `aload_1` (3), `invokevirtual` (2), `invokeinterface` (2), `iconst_1` (2), `ireturn` (2), `new`, `dup`, `pop`, `getfield`, `putfield`, `invokestatic`, `iconst_0` |
| kinds de constant pool usadas | **8**: `Utf8` (119), `Class` (28), `NameAndType` (15), `Methodref` (11), `Integer` (4), `String` (2), `InterfaceMethodref` (2), `Fieldref` (1) |
| atributos de miembro | `Code` (9); de clase: `MethodParameters` (1) |
| descriptores de método | `()V` ×5, `(Landroid/os/Bundle;)V`, `(Landroid/view/Menu;)Z`, `(Landroid/view/MenuItem;)Z`, `(Lcom/hola/apkcstudio/MainActivity;)V` |

**Lo que NO aparece y por tanto no está en el subconjunto** (medido por ausencia en el mismo `javap`):
`Long`/`Double`/`Float` en el pool, `Type`/`Handle`/`Dynamic`/`InvokeDynamic`, `invokedynamic` en
código, arrays, `MonitorEnter/Exit`, `jsr/ret`. Es decir: **ni lambdas ni `invokedynamic`**, que son
justo lo que `d8` desugara y lo que este plan declara fuera de alcance (Fase 2G.5). Con esa
restricción escrita, el trabajo de 2G es emitir 8 kinds de pool, 16 operaciones y 12 tipos: finito.

Complemento del §13: esas 6 clases de entrada (47 instrucciones) producen 217 objetos de dex, o sea
**~4,6 objetos de dex por instrucción de fuente**. Ese es el factor por el que el emisor de `.dex` se
puede validar antes que el compilador Java: la entrada se puede tomar de `javac` nativo y comparar
byte a byte contra `d8`, sin tocar el dispositivo.

Y un aviso de método, porque me mordió dos veces escribiendo esto: parsear un `.class` a mano es fácil
de equivocar. Mi primer parser saltó `interfaces_count` (y leyó descriptores imposibles, `desc=4112`),
y el `map_list` del dex tiene entradas de **12** bytes, no de 16. Cuando existe un oráculo oficial
aquí es `javap`, se usa el oráculo.

## 15. La pestaña no habla con ningún sitio que no sea su propio origen

El requisito de "sin conexión" necesita ser comprobable, no una intención. `Network.enable` por CDP
registra cada petición que hace la página durante toda la prueba (crear, editar, recargar, firmar):

```sh
sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" \
  APK_EJEMPLO=$(find "$PREFIX/tmp/studio-e2e-java" -name app.apk -path "*/build/*" | head -1) \
  KS_EJEMPLO="$HOME/.android/debug.keystore" node prueba-pestana.js'
```

**Resultado: `16 comprobaciones bien, 0 mal`, y `10 peticiones, todas locales`** (`http://127.0.0.1`
de `servidor.js`, más `data:`/`blob:` del propio origen). Ninguna a un CDN, ninguna a un dominio de
runtime de terceros, ninguna de telemetría. Con esto el criterio de aceptación de la Fase 4 queda
escrito como prueba: **una petición fuera del origen rompe `prueba-pestana.js`**.

## 16. El lector de `classes.dex`, con el dex de `d8` como fixture (Fase 2G, primera pieza)

Escrito en el fork (`apkc/dexreader.go`, rama `web`). Se empieza por leer y no por escribir porque
la comparación byte a byte contra `d8` solo responde "distinto"; hace falta quien responda
"**en qué sección**". comando de la prueba:

```sh
sh -c 'cd apkc && go test -run "Dex|MUTF8" -v .'
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$PREFIX/tmp/apkc-test.wasm" .
         cd ../shim && node navegador-sim.js "$PREFIX/tmp/apkc-test.wasm" -test.v'
```

**Resultado**: 7 tests nuevos en verde; la suite nativa pasa y **la wasm en hospedador de navegador
sube de 98 a 105** (`--- PASS` 105, `--- FAIL` 0, rc 0). Lo que se comprueba:

- El fixture es el `classes.dex` **real** de `d8 9.2.4-dev` sobre el proyecto del studio: 2844 bytes,
  pineado en base64 (el repo no versiona binarios). Su tamaño, su versión `035` y los recuentos
  (53 / 23 / 9 / 4 / 17 / 6 y 17 secciones) son exactamente los del §13: la medición queda convertida
  en test, así que un cambio de formato salta aquí y no en un teléfono.
- **Integridad, no solo parseo**: el lector verifica el `adler32` y el `sha-1` que la propia cabecera
  declara. Un byte cambiado dentro de la sección `data` hace que `Integrity()` nombre el checksum que
  rompió — y si el fichero llega a parser, la prueba lo acepta como negativa igualmente: lo que no
  puede pasar es que se acepte.
- **MUTF-8** tiene su propio test: `U+0000` escrito `C0 80` (el motivo de la forma: un terminador nunca
  aparece dentro de una cadena) y el par de sustitutos CESU-8 recompuesto a un carácter de plano 1,
  porque el orden del `string_ids` se define por **unidades UTF-16** y un emisor que compare por
  runes ordena distinto.
- La marca `~~D8{...}` del pool se lee: si el lector se parara antes del final de las cadenas, esa
  aserción es la que lo avisa.

Tres cosas que salieron mal antes de salir bien, escritas porque cuestan menos repetidas:
`fmt.Sprintf("%#04x", k)` sobre un tipo con `String()` es una llamada **recursiva** (lo detecta
`go vet`, no el compilador); no se pueden añadir métodos a `uint16`, hace falta un tipo nombrado; y
mezclar `uint32` con el `int` que devuelve un buscador de bytes es un `panic` fuera de rango con
pinta de fichero malformed.

Lo que el lector **todavía no lee**: `code_item` (instrucciones), `class_data_item` más allá de los
cuatro recuentos, y las anotaciones. Se escriben cuando haga falta para comparar, no antes.

## 17. La tabla de opcodes JVM, medida contra `javap` en vez de recordada

El caminante de `apkc/jvmopcode.go` necesita la anchura de cada operación. Escribirla de memoria
sobre JVMS es el tipo de claim que este repo no admite: una anchura mal no se nota en un resultado
raro, se nota en que el caminante entra dentro de la instrucción siguiente y **el método parece
parsear**. Así que se midió: para cada instrucción que `javap -v -c` lista, la anchura es
`(pc siguiente - pc - 1)`, y el byte del cuerpo de código en ese `pc` es el opcode de ese nombre.

```sh
# 1. la lista del oráculo, sobre las seis clases que javac dejó en build/classes
C=$(find "$PREFIX/tmp/studio-e2e-java" -type d -name classes | head -1); cd "$C"
J=$PREFIX/lib/jvm/java-21-openjdk/bin
CLASES=$(find . -name '*.class' | sed 's|^\./||; s|\.class$||; s|/|.|g' | tr '\n' ' ')
$J/javap -v -c -p -classpath . $CLASES > "$PREFIX/tmp/javap-v.txt"

# 2. la derivacion (los cuerpos de Code salen de los .class con un parser que SI resuelve los
#    nombres de atributo contra el pool; el primer intento los adivinaba por forma y salia
#    un "metodo" con max_stack=256, que es un StackMapTable disfrazado)
CLASES="$C" JP="$PREFIX/tmp/javap-v.txt" python3 "$PREFIX/tmp/mideopcodes.py"
```

**Resultado: 16 opcodes medidos, ninguno con conflicto** (un mismo byte nunca apareció con dos
anchuras distintas en los nueve cuerpos):

```
0x03 iconst_0/0     0x2a aload_0/0      0xb1 return/0          0xb8 invokestatic/2
0x04 iconst_1/0     0x2b aload_1/0      0xb4 getfield/2        0xb9 invokeinterface/4
0x12 ldc/1           0x57 pop/0         0xb5 putfield/2        0xbb new/2
0x59 dup/0           0xac ireturn/0     0xb6 invokevirtual/2   0xb7 invokespecial/2
```

y el recuento por método, que es el que el test `TestReadClassFullAnswersWhatJavapCounted` exige:

| método | stack | locals | bytes de Code | instrucciones |
| --- | --- | --- | --- | --- |
| `MainActivity$1.<init>` | 2 | 2 | 10 | 6 |
| `MainActivity$1.onMenuItemClick` | 3 | 2 | 15 | 8 |
| `MainActivity.<init>` | 1 | 1 | 5 | 3 |
| `MainActivity.onCreate` | 2 | 2 | 12 | 7 |
| `MainActivity.onCreateOptionsMenu` | 4 | 2 | 24 | 11 |
| `R$id/R$layout/R$style/R .<init>` | 1 | 1 | 5 | 3 cada una |

Total **47 instrucciones en 9 métodos con Code**, la misma cifra del §14 contada ahora por el
caminante de Go: `go test -run "ReadClassFull|WalkJvm|ArgCount|DescriptorOf"` en verde.

**Fuera de la tabla, a propósito**: `tableswitch`, `lookupswitch` y `wide` no tienen anchura fija y
el subconjunto no los usa; el proyecto del studio no lleva `switch`. Nada en la tabla es un cálculo
sin medición, y el caminante se niega por el byte que no conoce (`0x6a`, `baload`, en el test) en
lugar de suponer.

**Un claim que se corrigió midiendo**: el §14 decía que estas clases son *major version 52* (Java 8).
Es **65**, lo dice `javap -v` ("major version: 65") y lo confirman los bytes del fichero
(`cafebabe 0000 0041`); el 52 venía de un script descartable que leía mal la columna. El test del
lector lleva ahora el número del oráculo.

## 18. Otro fallo del hospedador, este lo destapó el timing

La suite wasm del fork subió a **117 tests** y pasó entera, pero `node navegador-sim.js` terminaba
con **rc 1**: al salir el programa quedaban `setTimeout` pendientes del *scheduling* de
`wasm_exec.js`, y al disparar llamaban a `_resume()` sobre un programa ya muerto —
`Error: Go program has already exited`. Como stdout va por tubería (buffer asincrónico) y stderr no,
el log mostraba el `PASS` completo y el fallo parecía posterior: el verde estaba impreso y el
proceso moría igual. La corrección es descartar `go._scheduledTimeouts` en el `exit` sustituido, tanto
en el shim como en `studio/hospedador.js`; con eso `rc=0` otra vez.

Lección escrita para no repetirla: **el código de salida del hospedador es parte de la prueba**. Un
hospedador que imprime verde y sale 1 no es un verde con ruido, es un fallo de infraestructura que
cualquier script de CI lee como fallo — con razón.

## 19. Las tablas de índices que `d8` deriva de las mismas clases: tres reglas que se aprendieron midiendo

`apkc/dextabla.go` interna tipos, protos, campos y métodos a partir de los seis `.class`, y el test
los compara **enteros** con las tablas del `classes.dex` de `d8` (leyéndolas con el lector del §16).
Al primer correr fallaron cuatro cosas; las cuatro eran reglas que este código no sabía, y las tres
primeras se escribieron mal aquí, no en el formato:

| Qué | Cómo se comportaba `d8` | Qué se hizo |
| --- | --- | --- |
| `type_list` de un proto | el lector avanzaba **4 bytes por parámetro** y devolvía descriptores que no existían (`makeText` salió con `(Context;I;Activity;)` en vez de `(Context;CharSequence;I)`) | el `type_item` es un `ushort`: stride **2**, medido contra las cadenas reales |
| `method_ids` | `MainActivity` ordena `<init>`, `onCreate`, `onCreateOptionsMenu`, `setContentView` — o sea **clase, nombre, proto**, no clase, proto, nombre como cita la documentación del formato | orden corregido, con la observación escrita en el comentario |
| `field_ids` | igual: clase, nombre, tipo | ya estaba bien |
| tipos que faltaban | `d8` tiene cinco `Ldalvik/annotation/…` que ningún `.class` menciona: las anotaciones que **sintetiza** | se internan a partir de lo que el class file declara: un `InnerClasses` con `inner == this` y `name != ""` pide `InnerClass`; un miembro con nombre pide `EnclosingClass`; la anonima con `name == ""` pide `EnclosingMethod`; los `MemberClasses` salen de las entradas con `outer == this`; y `MethodParameters` aparece donde el class file lleva el atributo |

Y un expectation mía que era falsa: el string `"Z"` de la tabla de `d8` **no** es un shorty, es el
descriptor del `boolean` que devuelven los dos listeners. Nadie lo iba a saber mirando mi tabla de
shorties, y el test lo dijo.

Comprobación, con las tablas comparadas completas (no un muestreo):

```sh
sh -c 'cd apkc && go test -run "DexTables|SplitDescriptor|ReadClassFull|ReadDex|WriteStrings" -v .'
```

**Resultado**: en verde nativo; la suite wasm en el hospedador de navegador sube a **122 tests**
(`--- PASS` 122, `--- FAIL` 0, rc 0). Después de esto, las cinco tablas de índices de lo que emite
este fork son ** iguales en contenido y en orden** a las que `d8` escribió sobre las mismas clases, y
la sección de cadenas ya lo era desde el §14-B (2G-B).

Lo que queda para que el dex sea un fichero: `class_data_item`, `code_item` (traducción de las 16
operaciones a registros dalvik), `debug_info_item`, y el ensamblado con `map_list`, cabecera y los
dos checksums (2G-E y 2G-F). Lo que **no** entra: `invokedynamic`, lambdas y desugaring de
dependencias (§14 y plan 2G.5).

## 20. La cabecera del `code_item`, palabra por palabra, casada con `javap` y con el `map_list`

Aquí es donde la precaución de no fiarse de la memoria pagó dos veces. Primero, porque la lectura
canónica que yo recordaba (doce bytes antes de las instrucciones) **no cuadra con el fichero que
escribió `d8 9.2.4-dev`**: seguirla da un `tries_size` de 1170 o un `code_item` que pide 452998451
palabras. Después, porque cuando escribí "lo que este fork no sabe" todavía no había casado las
cuatro palabras bajas con nada, y el `map_list` que usaba de referencia tenía dos etiquetas
cruzadas (`0x2000`/`0x2003`), con lo que cualquier conclusión sobre *dónde vive cada área* salía con
sentido pero falsa. Corregido el mapa, las seis palabras se nombran solas.

Los bytes del fixture (`R$<init>;` en el offset 1064): el opcode `0x70` (`invoke-direct`) está en
**+16**, la palabra `0x000e` (`return-void`) en **+22**, y el `u16` en **+12** vale **4**, que es
justo las palabras de `invoke-direct {v0}, Object.<init>` (3) y `return-void` (1). La comprobación
global que cierra el asunto: con ese desplazamiento, los nueve `code_item` **balacean** el área de
código sin solaparse, cada uno ocupa `16 + 2·unidades` y el hueco hasta el siguiente es de a lo sumo
3 bytes de alineado; el último termina antes de donde el mapa dice que empieza `debug_info_item`.

El mapa del fixture, ordenado por offset (es lo que permite decir si un número es un offset de debug
o una cantidad de palabras):

| sección | offset | tamaño |
| --- | --- | --- |
| `header` | 0 | 1 |
| `string_ids` | 112 | 53 |
| `type_ids` | 324 | 23 |
| `proto_ids` | 416 | 9 |
| `field_ids` | 524 | 4 |
| `method_ids` | 556 | 17 |
| `class_defs` | 692 | 6 |
| `code_item` | **884** | 9 |
| `debug_info_item` | **1160** | 7 (el área acaba en 1192, donde empieza `type_list`) |
| `type_list` | 1192 | 8 |
| `string_data_item` | 1258 | 53 |
| `annotation_item` | 2311 | 8 |
| `class_data_item` | **2384** | 6 |
| `encoded_array_item` | 2465 | 3 |
| `class_annotation_set_item` | 2484 | 6 |
| `annotations_directory_item` | 2548 | 5 |
| `map` | 2636 | 1 |

Y las palabras de cada cabecera, con al lado lo que `javap -v` midió sobre los mismos `.class`
(§17):

| método | off | +0 reg | +2 ins | +4 outs | +6 tries | +8 debug | +12 palabras | args | locals | max_stack | bytes Code |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `MainActivity$1.onMenuItemClick` | 884 | 4 | 2 | 3 | 0 | 1160 | 14 | 2 | 2 | 3 | 15 |
| `MainActivity$1.<init>` | 928 | 2 | 2 | 1 | 0 | 1165 | 6 | 2 | 2 | 2 | 10 |
| `MainActivity.onCreateOptionsMenu` | 956 | 3 | 2 | 2 | 0 | 1165 | 16 | 2 | 2 | 4 | 24 |
| `MainActivity.<init>` | 1004 | 1 | 1 | 1 | 0 | 1170 | 4 | 1 | 1 | 1 | 5 |
| `MainActivity.onCreate` | 1028 | 2 | 2 | 2 | 0 | 1174 | 9 | 2 | 2 | 2 | 12 |
| `R$id.<init>` | 1064 | 1 | 1 | 1 | 0 | 1170 | 4 | 1 | 1 | 1 | 5 |
| `R$layout.<init>` | 1088 | 1 | 1 | 1 | 0 | 1180 | 4 | 1 | 1 | 1 | 5 |
| `R$style.<init>` | 1112 | 1 | 1 | 1 | 0 | 1184 | 4 | 1 | 1 | 1 | 5 |
| `R.<init>` | 1136 | 1 | 1 | 1 | 0 | 1188 | 4 | 1 | 1 | 1 | 5 |

Lo que se puede afirmar de la tabla, y está asertado en el test:

- **+2 es `ins_size`**: coincide con `args_size` de `javap` en **nueve de nueve**. Es la única
  coincidencia exacta entre las cuatro palabras bajas, y por eso fue la primera en tener nombre.
- **+0 es `registers_size`**: nunca por debajo de los `locals` de `javap` (empata salvo donde el
  método necesita temporales: 4 frente a 2 en `onMenuItemClick`, 3 frente a 2 en
  `onCreateOptionsMenu`), y nunca por debajo de `ins_size` ni de `outs_size`.
- **+4 es `outs_size`**, la llamada más ancha: en las nueve no pasa de `max_stack`, que es justo lo que
  exige el hecho de que la pila de la JVM tuvo que sostener esos mismos argumentos antes del
  `invoke`. Nótese que **no** es `max_stack`: `onCreateOptionsMenu` tiene pila 4 y `outs_size` 2.
- **+6 es `tries_size`**: cero en las nueve, y `javap` no muestra `exception table` en ninguna. El
  `u16` en **+14** también es cero, pero no por tries: es la mitad alta del `u32` de `insns_size`.
- **+8 es `debug_info_off`**: los nueve valores caen dentro del área `debug_info_item` que declara
  el mapa (1160..1192). Esto es lo que antes se leía mal — con `0x2003` etiquetado como
  `class_data_item` parecía un valor "dentro del área de class_data, así que no puede ser un offset
  de debug". Un mapa mal etiquetado hace decir cualquier cosa sobre dónde vive cada estructura.
- **+12 es `insns_size`** como `u32`: `2·palabras` no baja de los bytes de `Code` en las nueve (una
  instrucción JVM
  se vuelve al menos una palabra dalvik y las hay de dos), y `16 + 2·palabras` es el item entero.

Total de palabras: **65** para las 47 instrucciones JVM del §14, dentro del área 884..1160 (276
bytes) que el mapa asigna a 9 `code_item`.

**Otra cosa que solo se ve comparando**: los nueve métodos usan **siete** `debug_info_item`
distintos — `d8` comparte el de `MainActivity.<init>` con el de `R$id.<init>` (1170) y el de
`MainActivity$1.<init>` con `onCreateOptionsMenu` (1165). El escritor que emita un debug_info por
método no será byte a byte igual que `d8` aunque cada item sea correcto; esto es lo que hay que
deduplicar en 2G-F, no un error de codificación.

Comprobación:

```sh
sh -c 'cd apkc && go test -run "ReadClassData|CodeItems" -v .'
```

**Resultado**: verde. `TestReadClassDataSplitsTheSixClassesTheWayJavacLeftThem` casa los recuentos
del `class_data_item` con `javap` — `{1,2,0,0}` en `MainActivity`, `{1,1,0,1}` en la anónima (su
`this$0`), `{1,0,1,0}` en cada `R*`, `{1,0,0,0}` en `R`, 9 métodos con cuerpo y 3 constantes
estáticas — y `TestCodeItemsAddUpToTheFile` exige la tabla de arriba más el solape, el hueco de
alineado, el `0x70` inicial y las relaciones con `javap` y con el área de debug. La suite nativa
sube a **131 tests** (`--- PASS` 131, `--- FAIL` 0) y la misma binaria en el hospedador de navegador
da **127 PASS más 4 SKIP, rc 0** (los cuatro que faltan son `TestSigningKey*`, que piden el
keystore de debug del `HOME` real, y `TestKotlinShortcomings*`, que necesita ejecutar un fichero
0755: en `GOOS=js` no hay proceso, así que el test se salta y lo dice).

**Dos cosas que se rompieron por no leer antes con cuidado**, escritas para que no se repitan con
otro fichero: **la primera es de esta sección y se corrigió en el §22** — ahí afirmé que el tipo de
valor de un campo estático empieza en el bit 16 de `access_flags`; era un artefacto de mi propio
escritor, y `d8` guarda las constantes en `static_values`, no dentro del `encoded_field`. Las otras
dos sí se sostienen: un campo solo puede traer valor si es **estático**, y el desplazamiento dentro
de `type_list` es de 2 bytes por `type_item`, no de 4.

**Lo que sigue sin medir**: las tablas de `try_item`/`encoded_catch_handler` que van detrás del
código, porque ninguno de los nueve métodos las tiene. `tamanoEnBytes` lo dice en el comentario y
`tries_size` distinto de cero se rechazaría, no se adivinaría; hará falta un fixture con `try` antes
de escribir esas dos estructuras.

También queda anotado el modo de trabajo: a media fase 2G estuve parcheando estos ficheros con
scripts de `python` en vez de la herramienta de edición, y salieron duplicados (`anchoValor` dos
veces), tipos borrados de un tajo y comentarios rotos. Con lectura previa y `Edit` se ve lo que se
toca; con `sed` no.

## 21. Las palabras dalvik, una por una: qué formato tiene cada instrucción y con qué criterio lo eligió `d8`

El paso entre leer un `code_item` y escribirlo es saber **qué palabra significa qué**. Volqué las
palabras de los nueve cuerpos junto a las instrucciones JVM de los mismos `.class` (un test
descartable que imprimía `walkJvmCode` al lado de `readCodeItem`, más `javap -c` de comprobación), y
de ahí salieron quince formatos. Cada fila de la tabla es una palabra que está en el fichero de
`d8`, no una que yo recordara:

| palabra de `d8` | qué dice ser | de dónde se saca |
| --- | --- | --- |
| `0x000e` | `return-void` (10x) | el final de los cuatro constructores de `R` |
| `0x030f` | `return v3` (11x: registro en la mitad alta) | el `iconst_1; ireturn` de `onMenuItemClick` |
| `0x020c` | `move-result-object v2` (11x) | el `Menu.add(...)` que sigue usando el resultado |
| `0x0112` / `0x1312` | `const/4 v1, 0` / `const/4 v3, 1` (11n: valor con signo en el nibble alto, registro en el bajo) | los dos `iconst_*` |
| `0x0115` + `0x7f02` | `const/high16 v1` (21c: registro abajo, mitad alta del valor arriba) | `R.layout.main` = `0x7f020000`, y **no** es un `string@` |
| `0x001a` + `0x0002` | `const-string v0, string@2` (21c) | el `"Clicked!"` |
| `0x0022` + `0x0008` | `new-instance v0, type@8` (21c) | el `new MainActivity$1(...)` |
| `0x2354` + `0x0000` | `iget-object v3, v2, field@0` (22c: destino en el nibble bajo, fuente en el alto, índice en la segunda unidad) | leer el `this$0` |
| `0x015b` + `0x0000` | `iput-object v1, v0, field@0` (22c) | escribir el `this$0` en el constructor |
| `0x1070 0x0010 0x0000` | `invoke-direct {v0}, method@16` (35c: `op`, luego recuento en los bits 4..7 y G en los 0..3, luego el **índice**, luego los registros en nibbles) | `Object.<init>` |
| `0x206f 0x0001 0x0010` | `invoke-super {v0, v1}, method@1` | el `super.onCreate(bundle)` |
| `0x2072 0x0003 0x0002` | `invoke-interface {v2, v0}, method@3` | `MenuItem.setOnMenuItemClickListener` |
| `0x3071 0x0004 0x0103` | `invoke-static {v3, v0, v1}, method@4` | `Toast.makeText` |
| `0x206e 0x000b 0x0010` | `invoke-virtual {v0, v1}, method@11` | `setContentView` |

Tres reglas de elección que tampoco estaban en ninguna parte y salieron comparando:

- **`invokespecial` no es siempre `invoke-direct`**: los `<init>` salen como `0x70`, pero el
  `invokespecial` de `super.onCreate` sale como **`invoke-super` (`0x6f`)**. El reparto depende del
  destino, no del opcode JVM.
- **Un `pop` justo después de una llamada se traga el `move-result-object`**: `d8` emite
  `invoke-interface` y sigue con `const/4`, sin palabra de resultado, porque el `MenuItem` devuelto
  no se usa. Emitir el `move-result` sería una palabra de más (y así lo detectó el test: 17 palabras
  contra las 16 de `d8`).
- **El entero se elige por su forma**: `0` y `1` van en `const/4`; `0x7f020000` va en
  `const/high16` (la mitad baja es cero) y no en `const/16`.

Con eso escrito, `apkc/dexinstr.go` traduce y `apkc/dexcode.go` desmonta. La comparación, método por
método y palabra por palabra:

```sh
sh -c 'cd apkc && go test -run "Traduce|Cabecera|Rechaza" -v .'
```

**Resultado**: los nueve cuerpos emiten **la misma secuencia de instrucciones, sobre los mismos
índices y con el mismo número de palabras** que `d8` (14, 6, 16, 4, 9, 4, 4, 4, 4). **Seis de nueve
salen idénticos hasta los números de registro** — los cuatro `R.<init>`, el de `MainActivity` y el de
la anónima con su `this$0`. Los tres restantes difieren solo en cómo se numeran los registros y
piden **a lo sumo un registro más** (`onMenuItemClick` 5 contra 4, `onCreateOptionsMenu` 4 contra 3,
`onCreate` 3 contra 2): `d8` reutiliza el registro de un argumento muerto, y este traductor no hace
análisis de vida. Es una diferencia de tamaño de marco, no de contenido: `registers_size` es un `u2`
de la cabecera y no cuesta ni un byte ni una palabra.

Y un detalle de la numeración que el test fija: los parámetros **no** empiezan en `v0`, sino en
`registers_size - ins_size` (§20). Con un marco de 4 y `ins_size` 2, el `this` es `v2`. Escribir
`{v0, v1}` ahí funciona solo por casualidad, y esa casualidad es la que hace que un verifier rechace
la clase.

**Lo que el traductor rechaza por nombre, sin emitirlo** (cada uno con su test en
`TestRechazaLoQueNoEstaMedido` o su mensaje en el código): cualquier opcode fuera de las dieciséis
medidas de §17 (lo dice con el byte, no con un misterio), `move-result` de un primitivo (devolver un
`int` desde una llamada: hace falta un fixture que lo escriba), un `invokespecial` que no sea
constructor ni llamada al super (método privado: sin medir), una llamada de más de cinco argumentos
(`invoke/range`), y cualquier registro `v16` o mayor en un formato de cuatro bits (las formas
`/from16` y `/range`). El subconjunto cubre la plantilla que genera la página; lo otro se mide con un
caso real antes de emitirlo.

Suite nativa después de esto: **135 tests** (`--- PASS` 135, `--- FAIL` 0); la misma binaria en
`GOOS=js` dentro del hospedador de navegador: **131 PASS más 4 SKIP, rc 0**.

## 22. El `classes.dex` completo, escrito en Go: cuatro cosas que se arreglaron leyendo los bytes, y un oráculo que lo acepta

`apkc/dexarma.go` ensambla el fichero entero (cabecera, las cinco tablas, `class_defs`, los
`code_item`, las `type_list`, los `class_data_item`, los `encoded_array_item` de las constantes, el
área de cadenas y el `map_list`, con `sha1` y `adler32` al final). Los offsets se calculan en orden
de dependencia, sin iterar. Sobre las seis clases del proyecto del studio salen **2392 bytes contra
los 2844 de `d8`**, y la diferencia tiene nombre: no emitimos `debug_info_item` (1160..1192, 7
objetos) ni el bloque de anotaciones (`annotation_item` 2311, `class_annotation_set_item` 2484,
`annotations_directory_item` 2548, 21 objetos en total), y tampoco la marca de origen con la que
`d8` se firma: la cadena `~~D8{"backend":"dex",…}` es suya, y copiarla sería mentir sobre quién
compiló. Con esas tres ausencias explicadas, las tablas que sí emitimos salen **byte a byte como las
de `d8`** (`type_ids`, `field_ids`, `method_ids` idénticos; `proto_ids` idénticos salvo el
`parameters_off`, que es un offset absoluto; y cada `string_data_item` idéntico, recortado por su
terminador cero y no "hasta la siguiente entrada del mapa", que era un error: detrás del última
cadena viene otra sección).

Cuatro afirmaciones que tenía escritas en el §20 eran falsas, y las destapó comparar lo que **nosotros
escribíamos** contra lo que **`d8` escribió**, no contra un manual:

- **La constante de un campo estático no viaja dentro del `encoded_field`.** Los bytes crudos de
  `d8` para `R$id` son `01 19` (índice por diferencia y `access_flags`) y nada más: el valor está en
  el `encoded_array_item` que anuncia `class_defs.static_values_off`, en 2465, 2471 y 2477. La
  lectura que plegaba el byte del valor en los bits 16 de `access_flags` era un artefacto de mi
  propio escritor: hacía leer `access_flags = 0x640019` donde `d8` tiene `0x19`. El §20 dijo lo
  contrario durante una revisión entera; queda corregido aqui, no borrado.
- **El tamaño de un `encoded_array_item` no es un `uint` de cuatro bytes.** Los tres items de `R`
  están a seis bytes de distancia; con `uint` cada uno ocuparía nueve (4 de tamaño + 4 de `VALUE_INT`
  + 1 del tipo). Con `uint`, el primero diría que tiene 25601 valores. Es `uleb128`.
- **`field_id_item` y `method_id_item` miden ocho bytes, no seis.** Comprobado con las offsets del
  mapa: `524 + 4·8 = 556`, que es exactamente dónde `d8` pone `method_ids`, y `556 + 17·8 = 692`,
  dónde están los `class_defs`. Los tres campos son `ushort` (`class_idx`, `type_idx`/`proto_idx`,
  `name_idx`) y quedan **dos bytes de relleno**. El lector los leía como `u32` y pasaba con el
  fichero de `d8` porque su relleno es cero; con el nuestro devolvía índices absurdos (`index out of
  range [51] with length 23`).
- **`ACC_SUPER` no llega al `class_def` y `<init>` sí lleva `ACC_CONSTRUCTOR`.** `javac` escribe
  `0x31` en `R$id` y `d8` deja `0x11`; la anónima sale `0x20` en el `.class` y `0x0` en el dex. Y los
  nueve constructores llevan el bit 16: `public <init>` = `0x10001`, el `<init>` sin modificador de
  la anónima = `0x10000`.

También se corrigió el internado de cadenas: un atributo `InnerClasses` menciona también las clases
que se *usan* (`MainActivity$1` lista `android/view/MenuItem$OnMenuItemClickListener` con su nombre
simple), y yo internaba ese nombre. `d8` solo internan los nombres de las entradas que hablan de la
clase propia, así que el nombre del interfaz sobraba y, en cambio, faltaba `"value"`, el miembro de
`@MemberClasses`. Con la condición puesta, las 52 cadenas que emitimos son exactamente las 53 de `d8`
sin su marca.

**El oráculo.** `dexdump` no es ejecutable en este teléfono, así que se usó el propio `d8` como
lector: un `.dex` también entra en `d8`, y si la estructura estuviera mal, R8 lo diría.

```sh
# primero se vuelca el dex que escribe este fork (una prueba temporal con APKC_VOLCAR, ya borrada)
java -cp ~/Android/build-tools/34.0.0/lib/d8.jar com.android.tools.r8.D8 \
  --min-api 21 --release --output out in/classes.dex
```

**Resultado**: `rc 0`, y `out/classes.dex` son 2076 bytes que este mismo lector vuelve a leer enteros.
Lo que sobrevivió a la ida y vuelta por R8 es lo que importa:

| qué | nuestro dex | re-dex de `d8` |
| --- | --- | --- |
| clases | 6 | 6 |
| métodos con cuerpo | 9 | 9 |
| `field_ids` / `method_ids` / `protos` | 4 / 17 / 9 | 4 / 17 / 9 |
| `pulsar`, `main`, `AppTheme` | 2130771968, 2130837504, 2130903040 | los mismos tres valores, leídos de **nuestro** `static_values` |
| cadenas / tipos | 52 / 23 | 40 / 18 |

La última fila es el detalle interesante: `d8` **poda** al re-dexar. Los cinco tipos
`Ldalvik/annotation/…` y sus cadenas viajan en nuestro fichero porque los internamos para el caso de
las anotaciones, pero como no emitimos las anotaciones que los usan, R8 los considera inalcanzables y
los quita. Es la prueba de que el internado del §19 es más amplio que lo que hace falta cuando no hay
anotaciones, y de que un dex puede ser correcto y más pequeño que lo que su productor escribió.

Comprobación automatizada (nada de esto depende de tener el JDK):

```sh
sh -c 'cd apkc && go test -run "DexArmado|CadenasDelDex|TablasFijas|ClassDefs|CodeItemsEscritos|ClassDataEscrito" -v .'
```

**Resultado**: seis pruebas en verde. La suite nativa sube a **140 tests** (`--- FAIL` 0) y la misma
binaria compilada con `GOOS=js` dentro del hospedador de navegador da **136 PASS más 4 SKIP, rc 0**.

**Lo que falta para que esto sea un apk instalable**, por orden: el `debug_info_item` (si se quieren
trazas legibles), el bloque de anotaciones (si se quiere que `InnerClasses` sobreviva), y la Fase 3
(`AXML` del `AndroidManifest.xml` y de los layouts, `resources.arsc` y la `R`), que es la que pone el
`classes.dex` dentro de un archivo que el sistema instala. Ninguna de esas tres cambia lo medido aquí.

## 23. `apkc dex`: la etapa, como comando, corriendo dentro de wasm

El ensamblado de §22 no servía de nada mientras solo lo llamaran las pruebas. Ahora es una orden:

```
apkc dex [-o classes.dex] carpeta-con-class [mas-carpetas...]
```

Recorre el árbol, lee los `.class` con el lector del §17, interna, ordena y ensambla. Ignora los
`META-INF` y el `module-info` (un jar de dependencia no es código nuestro). Rechaza por nombre todo lo
que salga del subconjunto medido, y rechaza **antes** de escribir: `claseCabe` mira los `try` y el
traductor mira los opcodes, así que un proyecto que necesite más formato se queda sin dex y con un
mensaje, no con un dex a medias.

Está cableado en el despachador de `main.go`, en la ayuda, y probado de dos maneras:

- `TestDexEnDiscoLeeLosArbolesYArmaElFichero` levanta el árbol del studio en un `t.TempDir()`, mete
  además un `META-INF/module-info.class` (que no debe contar) y comprueba que lo que sale se lee,
  pasa `Integrity()` y son los 2392 bytes del ensamblado medido.
- `TestElBinarioDexa` compila el paquete y ejecuta el binario de verdad: sin el caso `dex` en el
  `switch` de `main`, el fichero no aparecería. En `GOOS=js` se salta esta, porque ahí no hay
  proceso; la misma llamada la ejercitan el hospedador del navegador y la prueba de arriba.

**Una decisión de interfaz que salió de medir**: primero se escribió `-o -` (el dex por stdout, para
que la página lo recogiera de la salida del comando). Al probarlo con el binario, stdout devolvía
**2426 bytes en vez de 2392**: el log de progreso y el `finished in …` de `main` viven en stdout, y
34 bytes de charla se habrían colado dentro del dex. Se quitó el canal de stdout: la salida es un
fichero, que es justo lo que el hospedador OPFS de la pestaña sabe leer y extraer.

**Y la prueba que decide la fase** — la etapa entera corriendo en el binario wasm, en el hospedador
con forma de navegador, con los seis `.class` montados en la memoria simulada:

```sh
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd shim && node navegador-sim.js \
  --montar "<seis .class del studio>:/in/com/hola/apkcstudio/<nombre>.class" (uno por clase) \
  --exportar "/out/classes.dex:$PREFIX/tmp/dexwasm/nuestro-wasm.dex" \
  "$PREFIX/tmp/apkc-wasm.wasm" dex -o /out/classes.dex /in'
```

**Resultado**: `rc 0`, **2392 bytes** en `/out/classes.dex`, y `cmp` con el dex que produce el binario
nativo sobre las mismas clases: **idénticos, byte a byte** (`sha1 920c0f3e…`). Y `d8` vuelve a
aceptarlo como entrada (`rc 0`), la misma comprobación del §22 pero sobre el fichero que salió de
wasm. Con esto, la etapa de dex **no necesita el equipo**: corre donde corre la página.

Suite nativa: **142 tests** (`--- FAIL` 0). La misma binaria en `GOOS=js` dentro del hospedador de
navegador: **137 PASS más 5 SKIP, rc 0**. Los cinco que se saltan son siempre por lo mismo: los tres
`TestSigningKey*` piden el keystore de debug del `HOME` real (en la memoria simulada no está),
`TestKotlinShortcomings*` necesita ejecutar un fichero 0755, y `TestElBinarioDexa` compila y corre el
binario: en `GOOS=js` no hay proceso, y lo que esa prueba comprueba lo comprueban arriba el hospedador
de navegador y la prueba de disco.

## 24. Fase 3 medida antes de escribir un byte: qué produce `aapt2` sobre el proyecto del studio

Objetivo de la fase, en bytes y con el mando que los produce (el proyecto lo genera la propia
página; la variable `STUDIO_OUT` es relativa a `$PREFIX/tmp`, que es donde el test escribe):

```sh
sh -c 'P=$PREFIX/tmp/fase3; rm -rf "$P"; mkdir -p "$P"; cd "$P" && \
  STUDIO_KOTLIN=0 STUDIO_OUT=f3-proyecto.zip node '"$PWD"'/studio/prueba.js >/dev/null && \
  unzip -q $PREFIX/tmp/f3-proyecto.zip -d "$P" && \
  cd "$P"/com.hola.apkcstudio && $PREFIX/tmp/apkc-gate build'
unzip -l build/app.apk
```

| qué | bytes | dónde |
| --- | --- | --- |
| `AndroidManifest.xml` binario | 2000 | en el apk y en `build/`, **idénticos** (`cmp`) |
| `resources.arsc` | 1088 | idem |
| `res/layout/main.xml` binario | 784 | el layout del apk también es AXML, no texto |
| `classes.dex` | 2844 | lo que ya escribe este fork en 2392 (§22) |
| `build/flats/*.flat` | 1076 + 536 | paso intermedio de `aapt2 compile`; **no** viaja en el apk |
| firma v1 (`META-INF/*`) | 500 + 1221 + 376 | la escribe el escritor Go de este fork (§7) |

O sea: **la Fase 3 tiene que emitir 3872 bytes de recursos binarios** (2000 + 1088 + 784) y nada más,
porque el `.dex` ya sale y el zip+firma ya sale. El apk entero del studio son 10348 bytes.

**El instrumento** se quedó en el repo: `apkc/recursos_dump_test.go` anda el fichero trozo a trozo
desde los offsets que el propio fichero declara y se salta sin `APKC_ARBOLES`. Todo lo que sigue es
lo que imprimió sobre los ficheros de la tabla de arriba.

### El AXML (los tres ficheros binarios comparten forma)

Encabezado común a todos los trozos, medido con `xxd -l 32`: `type` en 0..1, `headerSize` en 2..3,
`size` en 4..7. Con eso el resto se lee solo:

| trozo | tipo | cabecera | contenido medido |
| --- | --- | --- | --- |
| `XML` | 0x0003 | 8 | solo tamano total (2000); los hijos empiezan en +8 |
| `STRING_POOL` | 0x0001 | 28 | `cadenas`(4) `estilos`(4) `flags`(4) `stringsStart`(4) `stylesStart`(4), y tras la cabecera un `u32` por cadena **relativo a `stringsStart`** |
| `XML_RESOURCE_MAP` | 0x0180 | 8 | una lista de `u32` con los ids de atributo (`0x01010000`, `0x01010001`, …): el del manifiesto son 12 ids en 56 bytes |
| `XML_START_NS` / `END_NS` | 0x0100 / 0x0101 | 16 | nodo de 16 (`chunk`+`linea`+`comentario`) y luego `ns`(4) + `nombre`(4): `ns=18` ("android"), `nombre=24` (la URI) |
| `XML_START_TAG` | 0x0102 | 16 | nodo de 16 + `attrExt` de **20**: `ns`(4) `nombre`(4) `attrStart`(2) `attrSize`(2) `attrCount`(2) `idIndex`(2) `classIndex`(2) `styleIndex`(2); los atributos empiezan en `+16+attrStart` |
| `XML_END_TAG` | 0x0103 | 16 | nodo de 16 + `ns`(4) + `nombre`(4) = 24 exactos |

Cada atributo son 20 bytes: `ns`(4) `nombre`(4) `rawValue`(4) `size`(2) `res0`(1) `dataType`(1)
`data`(4). Y el `dataType` es el del `Res_value` del arsc: `0x03` = cadena (los datos son un índice
de la pool), `0x10` = entero decimal, `0x01` = referencia. Comprobado contra el manifiesto de texto:
el `<manifest>` sale con 7 atributos y 176 bytes (`36 + 7·20`), y los dos primeros son
`android:versionCode=1` (`tipo 0x10, datos 1`) y `android:versionName="0.1"` (`raw=13`, `datos=13`,
que en la pool es la cadena `"0.1"`. La pool del manifiesto son **31 cadenas, `flags=0` o sea
UTF-16**, sin ordenar (en orden de primer uso), y el fichero termina clavado: el último trozo
(`END_NS` en 1976, 24 bytes) da exactamente 2000.

### El `resources.arsc`

```
@0     TABLE       hs=12  size=1088  packageCount=1
@12    STRING_POOL hs=28  size=56    1 cadena, flags=0x100 (UTF-8): "res/layout/main.xml"
@68    PACKAGE     hs=288 size=1020  id=127  nombre="com.hola.apkcstudio"
                          typeStrings=288  lastPublicType=0  keyStrings=368  lastPublicKey=0  typeIdOffset=0
  @356   STRING_POOL 3 cadenas UTF-16: id, layout, style
  @436   STRING_POOL 3 cadenas UTF-8:  pulsar, main, AppTheme
  @504/@628/@752  TYPE_SPEC hs=16 size=20 (una por tipo)
  @524  TYPE id=1 hs=84 size=104  1 entrada  clave=0 "pulsar"  valor tipo 0x12 datos 0
  @648  TYPE id=2 hs=84 size=104  1 entrada  clave=1 "main"    valor tipo 0x03 datos 0 ("res/layout/main.xml")
  @772  TYPE id=3 hs=84 size=152  1 entrada  clave=2 "AppTheme" valor complejo (padre 0x01000008)
  @924  TYPE id=3 hs=84 size=164  segunda configuracion del mismo tipo
```

(El `0x12` de la primera fila es el byte `dataType` del `Res_value` de un recurso de tipo `id`, con
`datos=0`: un `id` no lleva valor. Los otros dos que aparecen aqui son `0x03` = cadena y, en el
estilo, un valor complejo; el `dataType` que usa el **manifiesto** para sus enteros es `0x10`
(decimal), medido arriba. Qué significa cada número del framework no se afirma aqui: se mide al
escribir el emisor, con `aapt2 dump resources` como oráculo.)

Tres cosas que se aprendieron leyendo, no recordando: los offsets de entrada de un `TYPE` son
**relativos a `entriesStart`** (sumarlos al principio del trozo devuelve la cabecera del propio
tipo); `entriesStart` vale 88 con `headerSize` 84 porque entre medias están los `u32` de los offsets,
uno por entrada; y el `ResTable_config` que lleva cada `TYPE` ocupa **64 bytes** en esta versión de
`aapt2` (de ahí el `hs=84`). El fichero también termina clavado: el último `TYPE` (924 + 164) es
1088.

**Y el oráculo existe en este teléfono**, aunque `dexdump` no: `aapt2 dump xmltree` sí es ejecutable
y su salida casa con lo leído de los bytes:

```sh
~/Android/build-tools/34.0.0/aapt2 dump xmltree --file AndroidManifest.xml build/app.apk
```

```
N: android=http://schemas.android.com/apk/res/android (line=1)
  E: manifest (line=1)
    A: http://schemas.android.com/apk/res/android:versionCode(0x0101021b)=1
    A: http://schemas.android.com/apk/res/android:versionName(0x0101021c)="0.1" (Raw: "0.1")
    A: http://schemas.android.com/apk/res/android:compileSdkVersion(0x01010572)=34
    A: http://schemas.android.com/apk/res/android:minSdkVersion(0x0101020c)=21
```

Los ids entre paréntesis son los del `XML_RESOURCE_MAP` (§24) y los números de línea, los del campo
`lineNumber` de cada trozo: las dos lecturas —la del árbol y la de los bytes— coinciden. Eso cambia
el plan de la fase: la comprobación de 3.1 a 3.3 puede ser byte a byte contra `aapt2 dump`, sin
esperar a un dispositivo.

**Lo que esto decide para la Fase 3**: hay que emitir dos estructuras (AXML y tabla de recursos) con
sus pools de cadenas en los dos modos (UTF-16 en el manifiesto, UTF-8 en la tabla y en las claves),
los ids de framework que lleva el `XML_RESOURCE_MAP` (12 en el manifiesto, 6 en el layout) y el
valor complejo de un estilo, que es lo único que todavía no está despiezado del todo. **Queda fuera
de esta medición** el `.flat` (no viaja en el apk) y cualquier recurso que no sea `id`/`layout`/
`style` del subconjunto del studio: dibujos, cadenas de texto y `drawable` no aparecen en el
recuento y no se emiten todavía.

Suite nativa con el instrumento del §24 dentro del repo: **142 PASS / 0 FAIL** (no suma tests: la
prueba se salta sin `APKC_ARBOLES`). La misma binaria en `GOOS=js` dentro del hospedador de
navegador: **137 PASS más 6 SKIP, rc 0**, y el sexto salto es el propio volcado, que pide los
ficheros del SDK que en wasm no existen.

## 25. Las clases del disco real producen el mismo dex que las fijadas en base64

El dexer estaba probado contra los `.class` del proyecto fijados en el test (§17). Quedaba por ver
si lo mismo salía de un `apkc build` de verdad, con el `javac` y el `aapt2` de este teléfono, y no de
una copia en base64. Se hizo con la medición del §24 (proyecto generado por la página, `build/`
borrado antes de compilar para que no quedara nada viejo):

```sh
# el arbol limpio: seis .class y dos .flat, ni uno mas
rm -rf build && apkc build
find build/classes -name '*.class' | wc -l        # -> 6
ls build/flats | wc -l                            # -> 2

apkc dex -o /tmp/nuestro-real.dex build/classes
java -cp ~/Android/build-tools/34.0.0/lib/d8.jar com.android.tools.r8.D8 \
  --min-api 21 --release --output out /tmp/nuestro-real.dex    # -> rc 0
cmp /tmp/nuestro-real.dex /tmp/nuestro-wasm.dex                 # -> identicales
```

**Resultado**: `2392 bytes`, **exactamente los mismos bytes** que el dex que sale de las clases
fijadas y de las que corrieron dentro de wasm (`sha1 920c0f3ec828e835d7dc8f848be84b7ba25d7dd4`), y
`d8` lo acepta como entrada con `rc 0`. Con un `build/` limpio no queda ni un `.class` de mas: la
`R` que escribe `aapt2` son las cuatro clases ya contadas (`R`, `R$id`, `R$layout`, `R$style`), asi
que el recuento del §14 y las pruebas fijadas en base64 describen el proyecto real, no una version
suya.

Esto cierra una duda que el método de fixtures dejaba abierta: que el pinado en base64 se hubiera
quedado atrás respecto a lo que producen hoy el `javac` y el `aapt2` del teléfono. No: son los
mismos bytes, y por eso la comparación contra `d8` del §21 y el byte a byte del §22 valen para el
proyecto que genera la página, no solo para el fixture.

## 26. Lector Go del AXML y de la tabla de recursos, con los ficheros de `aapt2` de fixture

`apkc/recursos.go` lee las dos estructuras. Los ficheros contra los que se prueba son los de este
teléfono, fijados en base64 en `apkc/recursosfixture_test.go` (2000 y 1088 bytes, `assert` de la
longitud en el test). Es el mismo método del §16 con el dex: antes de escribir nada que el sistema
tenga que aceptar, hay que poder leer lo que acepta hoy y decir en qué se diferencia.

Lo que reprodujo el lector a la primera, tal como se midió en el §24: las 31 cadenas UTF-16 del
manifiesto, los 12 ids del `XML_RESOURCE_MAP` (el primero `0x01010000`), los **16** trozos de nodo
(dos de espacio de nombres y catorce de etiqueta), y los siete atributos de `<manifest>` **por su
nombre** — `versionCode`, `versionName`, `compileSdkVersion`, `compileSdkVersionCodename`,
`package`, `platformBuildVersionCode`, `platformBuildVersionName`, el mismo orden que imprime
`aapt2 dump xmltree`, con `versionCode` en entero (`tipo 0x10, datos 1`) y `versionName` como
referencia de cadena (`0x03`, que es `"0.1"`). En la tabla: pool global de una cadena en UTF-8
(`res/layout/main.xml`), paquete 127 con su nombre, tres tipos y tres claves, cuatro bloques, la
entrada `pulsar` con su clave, `main` apuntando a la pool global y el `ResTable_config` de 64 bytes.

Tres cosas que antes no estaban medidas y salieron leyendo:

- **La forma del valor complejo de un estilo.** La entrada declara 16 bytes: tamaño, banderas, clave,
  y **dentro de esa cabecera** el id del padre (`0x0103012b`, un id de estilo del framework) y un
  recuento. Detrás van registros de doce bytes: nombre `u32` + `Res_value` de ocho (`size=8`,
  reserva, `dataType`, `datos`). Los cuatro del primer bloque son `0x01010054`, `0x01010056`,
  `0x010102cd`, `0x01010451`, todos ids de atributo.
- **El campo de recuento no se entiende todavía, y se dice.** Las dos configuraciones de `AppTheme`
  declaran `4` registros y traen **4 y 5**: el bloque de la segunda termina con un registro más del
  que anuncia su cabecera. El lector guarda los registros crudos, comprueba su forma (que cada
  nombre empiece en `0x01…` y que cada valor mida 8) y no interpreta el conteo. Medirlo es lo
  primero de 3-C, porque para **escribir** un estilo hay que saber qué número va ahí.
- **Las banderas de `ResTable_entry` viajan crudas.** `pulsar` mide 8 con `flags 0x04`, `main` 8 con
  `0x00`, `AppTheme` 16 con `0x01`. Qué bit significa "pública" y cuál "compleja" no se afirma aquí:
  el lector usa `tam==16 && flags&1` porque es lo que separa el estilo de las simples en este
  fichero, y lo demás se medirá al emitir.

Y cinco rechazos probados, cada uno con un byte cambiado sobre el fixture: el `file_size` del XML,
un offset de cadena fuera de su pool, un nombre de etiqueta fuera de la tabla, un `packageCount`
distinto de uno, una cabecera de paquete con `headerSize` inventado, una clave fuera de la tabla y
un valor complejo mal alineado. Ninguno se acepta; ninguno panica.

```sh
sh -c 'cd apkc && go test -run "Lector" -v .'
```

**Resultado**: tres pruebas en verde. La suite nativa sube a **145 tests** (`--- FAIL` 0) y la
misma binaria en `GOOS=js` dentro del hospedador de navegador da **140 PASS más 6 SKIP, rc 0**.

## 27. El valor complejo de un estilo, medido con variantes; y la discrepancia que abrí y se cerró

Para dejar de razonar sobre el estilo se varió `res/values/styles.xml` sobre el proyecto del studio y
se reconstruyó con el `aapt2` de este teléfono (cada vez `rm -rf build`; el manifest no se toca,
porque pide `@style/AppTheme` y si se renombra, `aapt2 link` falla con
`resource style/AppTheme not found` — que es además una comprobación gratis de que el que manda es
el manifest):

| variante | ítems declarados | lo que salió en la tabla |
| --- | --- | --- |
| `AppTheme` con 2 ítems | 2 | `recuento=2`, dos registros de doce bytes |
| `AppTheme` con 9 ítems | 9 | `recuento=9`, nueve registros |
| tres estilos (5, 1 y 3 ítems) | 9 | tres entradas en el bloque `style`, con recuentos 5, 1 y 3 |
| la original (5 ítems en un estilo) | 5 | recuentos **4 y 5** en los dos bloques de configuración |

Con eso la forma queda medida, y **rectifica el §26**: la cabecera de una entrada compleja son 16
bytes — `tam` (que vale 16), `flags` (`0x0001`), `clave`, **el id del padre** en `+8` y **el número
de registros** en `+12`; después vienen exactamente `recuento` registros de doce bytes (nombre `u32`
+ `Res_value` de ocho: `tam=8`, reserva, `dataType`, `datos`). La lectura que en el §26 hacía decir
"cuatro en ambas configuraciones" tomaba el campo **una palabra más arriba**, donde en realidad está
la mitad baja del nombre del primer registro (`0x0054`), que es lo que yo había llamado `size=84` y
no era un tamaño: el `Res_value` de un registro empieza en el byte 4 del registro, no en el 0.

**Y la discrepancia que dejé escrita se cerró el mismo día, con causa y arreglo**: en dos volcados
del `resources.arsc` de `build/` —sobre un proyecto cuyo sha256 es **idéntico al del fixture** (1088
bytes, `4b2b7ec08c98da92…`; el manifiesto también: `31f398254cb8a31c…`)— el instrumento imprimió
primero `4/4` y `5/5` y después `1/1`. Mismos bytes, dos lecturas. El culpable era el propio
instrumento: su bucle de registros recorría **hasta el final del bloque** en vez de hasta la
entrada siguiente, así que con varias entradas por bloque contaba las ajenas (por eso la variante de
tres estilos llegó a imprimir "11 registros" para una entrada que declaraba 5) y con un `grep
| sort -u` encima, las dos líneas idénticas se colapsaban en una. Puesta la cota —recorrer hasta el
offset de la entrada siguiente— el mismo fichero da `recuento=4 registros=4` y `recuento=5
registros=5`, que es lo que el lector de `recursos.go` venía exigiendo desde que lleva
`registros == recuento` y falla si no. Lección para repetir: **cuando dos lecturas de los mismos
bytes discrepan, el sospechoso es el que lee**, y un `sort -u` en el filtro es capaz de esconder una
entrada de más.

Lo que la Fase 3 puede usar como objetivo, medido y estable: el `AndroidManifest.xml` (2000) y el
`resources.arsc` (1088) que construye el SDK de este teléfono son byte a byte los del fixture, así
que el oráculo es fijo.

```sh
sh -c 'cd apkc && go test -run "Lector" -v .'   # y el volcado, con APKC_ARBOLES sobre build/
```

**Resultado**: las tres pruebas del lector siguen en verde y el recuento de la suite no cambia con el
arreglo del instrumento, porque el recorrido acotado afecta solo al volcado: nativa **145 PASS / 0
FAIL**, `GOOS=js` en el hospedador de navegador **140 PASS + 6 SKIP, rc 0**.

Con esto la Fase 3 ya tiene con que comparar: cuando el emisor escriba su `AndroidManifest.xml`, la
comprobación es `leeAXML(lo nuestro)` contra `leeAXML(lo de aapt2)` y, por fuera, `aapt2 dump
xmltree` sobre los dos.

## 28. El escritor de tablas de cadenas de recursos, byte a byte contra `aapt2`, y lo que separa
## nuestra tabla de la suya

`apkc/poolrecurso.go` escribe un `RES_STRING_POOL`. La prueba no es un muestreo: se toman los tres
pools del `resources.arsc` fijado (el global en `@12`, UTF-8, 56 bytes; el de nombres de tipo en
`@356`, UTF-16, 80; el de claves en `@436`, UTF-8, 68), se leen con el lector de la 3-B para sacar
las cadenas, y lo que produce el escritor se compara **byte a byte** con los bytes originales. Los
tres salen idénticos.

Lo que hubo que copiar de los bytes, porque el manual no lo deja claro: los offsets del array son
**relativos al inicio de los datos** (no al del trozo), el modo UTF-8 lleva **dos** longitudes
(bytes y unidades utf-16) y el UTF-16 **una**, cada cadena termina con un cero (dos bytes en
UTF-16) y el área de datos se rellena hasta múltiplo de cuatro. Con una cadena de 200 caracteres y
otra del plano suplementario la ida y vuelta por el lector cierra el resto.

Y medida, para que nadie lo descubra escribiendo, la razón por la que la tabla de recursos **no**
puede salir byte a byte con una sola configuración: los dos bloques `id=3` del `AppTheme` del
proyecto difieren en **un byte** de su `ResTable_config` — el `+28` vale `0x00` en el primero y
`0x17` (23) en el segundo — y el bloque con `23` es el que trae el registro extra
(`0x010104e0`, un atributo que nació en API 23). Es decir: `aapt2` **parte los ítems de un estilo
por el nivel de API en que se introdujo cada atributo**, y emite una configuración base y una
`-v23`. Reproducir eso exige la tabla atributo a API del framework, que aquí no hay de dónde sacarla
sin `android.jar` parseado. Así que el plan para 3-C es emisor de **una sola configuración base**
con todos los ítems (válida, instalable, y comprobable con `aapt2 dump resources`), y diferencia
documentada en el número de configuraciones — no en los ids, ni en las cadenas, ni en los valores.

Comprobación:

```sh
sh -c 'cd apkc && go test -run "Pool" -v .'
```

**Resultado**: dos pruebas en verde. Nativa **147 PASS / 0 FAIL**; la misma binaria en `GOOS=js`
en el hospedador de navegador **142 PASS + 6 SKIP, rc 0**.

## 29. El cuerpo del `resources.arsc`, escrito en Go: byte a byte con `aapt2` salvo donde aapt2 abre
## la tabla en dos configuraciones

Primero hubo que cerrar una incógnita que el lector de la 3-B dejaba cruda: **qué significan las
banderas de una entrada**. Para no adivinarla se reconstruyó el objetivo con el `aapt2` de este
teléfono —los mismos bytes del fixture, `sha256 4b2b7ec0…`, comprobado— y se varió el `res/values`,
volcando después cada `ResTable_typeSpec` y cada `ResTable_entry` con el instrumento del §24 (que
hasta hoy no imprimía las banderas del spec; ya las imprime):

```sh
A=$HOME/Android/build-tools/34.0.0/aapt2; J=$HOME/Android/platforms/android-34/android.jar
P=$PREFIX/tmp/flagprobe; rm -rf $P; mkdir -p $P/res/values $P/res/layout
cp $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/{AndroidManifest.xml} $P/
cp -r $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/res/layout $P/res/
cp $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/res/values/styles.xml $P/res/values/
cd $P && $A compile --dir res -o flats.zip
$A link -o out.apk -I $J --manifest AndroidManifest.xml --auto-add-overlay -R flats.zip
unzip -p out.apk resources.arsc | sha256sum          # 4b2b7ec08c98da92… el fixture, idéntico
# variante A: + <item type=id>, <string>, <bool>, <color> y un segundo estilo sin atributo de API 23
# variante B: el AppTheme sin windowLightStatusBar (el atributo que nació en API 23)
# variante C: los mismos recursos con <public name=… type=…/> y un <declare-styleable>
env APKC_ARBOLES="$P/xA/resources.arsc $PREFIX/tmp/flagprobe2/x/resources.arsc" \
  go test -C apkc -run TestArbolDeTroz -v
```

Y la regla, medida sobre las tres variantes (no inferida del `header.h`):

| qué | `ResTable_entry::flags` | `ResTable_typeSpec::entryFlags[i]` |
| --- | --- | --- |
| entrada de tipo `id` (declarada en `values` o con `@+id/` en un layout) | `0x0004` | `0x00000000` |
| entrada de tipo `style` o `attr` (valor complejo) | `0x0001` | `0x00000000` |
| entrada declarada con `<public>` | `0x0002` | `0x40000000` |
| resto (`layout`, `string`, `bool`, `color`) | `0x0000` | `0x00000000` |
| un estilo **partido por API** (el `AppTheme` con un atributo de API 23) | `0x0001` | `0x00000400` |

Tres cosas más salieron de esas variantes, todas antes supuestas y ahora vistas:

- El `u16` del spec en `+10` **es el número de bloques `RES_TABLE_TYPE` que siguen para ese id**: vale
  `1` en `id`, `layout`, `bool`, `color`, `string` y `attr`, y `2` en `style` cuando el estilo se
  parte. En la variante B (mismo estilo, sin el atributo de API 23) vuelve a `1` y la tabla mide 924
  en vez de 1088: **exactamente un bloque de 164 bytes menos**.
- Los tipos van **ordenados alfabéticamente por nombre** y el `id` de tipo es su posición más uno
  (`attr, bool, color, id, layout, string, style` → `01..07`); las claves del pool siguen el mismo
  orden (por tipo y dentro del tipo por nombre), y los registros de un estilo van ordenados por su
  `id` de atributo.
- `lastPublicType` y `lastPublicKey` del paquete valen **0 incluso declarando `<public>`**; el
  `typeIdOffset` también. La cabecera del paquete son 288 bytes: 8 de trozo, 4 de **número** de
  paquete (mide `0x7f`, no `0x7f000000` — los ids de recurso salen de desplazarlo 24), 256 de nombre
  UTF-16 y cinco `u32` detrás.

Con eso `apkc/arscwriter.go` escribe la tabla entera: cabecera, pool global, paquete con sus dos
pools de nombres y, por tipo, un `RES_TABLE_TYPE_SPEC` de 16 + 4·entradas seguido de su
`RES_TABLE_TYPE` de cabecera 84 (20 de campos + la configuración base de 64, cuyo único campo no nulo
es su propio tamaño), con `entriesStart = 84 + 4·entradas` y los offsets relativos a él. Una
configuración, como decidió el §28.

La comparación es byte a byte, con las dos únicas diferencias anunciadas antes de medir:

- Los primeros **752 bytes** —cabecera de tabla, pool global, paquete, los dos pools de nombres, y
  los tipos `id` y `layout` enteros— son idénticos a los del fixture **salvo dos campos**: el
  `file_size` de la tabla (en `+4`) y el del paquete (en `+72`), que cambian porque nosotros
  escribimos una configuración en vez de dos. El test lo que hace es comprobar que el fixture anuncia
  1088 y 1020 en esos dos sitios, sustituirlos por los nuestros, y comparar.
- Después de `@752` el `AppTheme` sale distinto por diseño: `aapt2` emite base (4 registros) + `-v23`
  (5) y nosotros un bloque con los **5** registros en la configuración base. Nuestra tabla mide 936
  bytes = 1088 − 152 (el bloque base del estilo que no escribimos).

Y el oráculo de verdad, que no es nuestro lector: metida nuestra tabla en un zip con forma de apk,
`aapt2 dump resources` la lee y imprime **el mismo listado** que imprime para la tabla de `aapt2`, con
`0x7f010000 id/pulsar`, `0x7f020000 layout/main`, `0x7f030000 style/AppTheme` y el estilo con
`size=5 parent=0x0103012b` y sus cinco atributos (el de la fijación lista 4 en la base y 5 en la
`-v23`). El aviso `no such path exists` que imprime nuestra variante es del zip de prueba —no metió
`res/layout/main.xml`—, no de la tabla.

```sh
A=$HOME/Android/build-tools/34.0.0/aapt2
env APKC_ESCRIBE_ARSBC=$PREFIX/tmp/flagprobe/nuestro.arsc \
  go test -C apkc -run TestEscribeTablaContraFixture .
mkdir -p $PREFIX/tmp/flagprobe/ours && cp $PREFIX/tmp/flagprobe/nuestro.arsc $PREFIX/tmp/flagprobe/ours/resources.arsc
cp $PREFIX/tmp/flagprobe/x/AndroidManifest.xml $PREFIX/tmp/flagprobe/ours/
(cd $PREFIX/tmp/flagprobe/ours && zip -X -0 -q ../nuestro.apk resources.arsc AndroidManifest.xml)
$A dump resources $PREFIX/tmp/flagprobe/nuestro.apk   # y lo mismo sobre out.apk, para cotejar
```

Comprobación del escritor:

```sh
go test -C apkc -run "EscribeTabla" -v .
```

**Resultado**: dos pruebas en verde (la de byte a byte con el fixture y la de los seis rechazos:
clave inexistente, `id` de tipo que no es su lugar, spec sin bandera por entrada, registro con nombre
fuera del framework, nombre de paquete vacío y nombre de paquete de 128 caracteres). Contando solo
pruebas de primer nivel la suite nativa está en **147 PASS / 0 FAIL / 1 SKIP** y con subpruebas en
**149 PASS / 1 SKIP**; la misma binaria en `GOOS=js` dentro del hospedador de navegador da **142 PASS
+ 6 SKIP, rc 0** (144 y 6 contando subpruebas). Queda dicho el modo de contar, porque los párrafos
anteriores usaban el otro y en más de un sitio se leen dos unidades de diferencia.

## 30. El AXML del layout y el del manifiesto, escritos en Go: byte a byte con `aapt2` en los dos

`apkc/axmlwriter.go` escribe los binarios XML. La comparación es la misma que en el §29 pero aquí **no
hay diferencia que declarar**: los 784 bytes del `res/layout/main.xml` y los 2000 del
`AndroidManifest.xml` que produjo el SDK de este teléfono salen **idénticos byte a byte**. Los dos
ficheros quedan fijados en base64 (`apkc/recursosfixture_test.go`, con sus longitudes asertadas) y la
prueba recorre los dos casos.

Lo que hubo que medir, porque el manual no lo dice o miente:

- **El orden del pool**: primero los nombres de atributo **que tienen id de recurso, ordenados por ese
  id** —que es exactamente el orden del `XML_RESOURCE_MAP`— y después el resto de las cadenas
  ordenadas por valor ASCII, con las mayúsculas antes que las minúsculas. En el layout: 6 + 8; en el
  manifiesto: 12 + 19. El orden del fuente no tiene nada que ver: en el fuente el `TextView` declara
  `style` el primero y en el binario sale el último.
- **Los atributos de un elemento salen reordenados**: los de namespace por su id y luego los sin
  namespace por nombre. Medido en los cuatro del `LinearLayout` y en los siete de `<manifest>`.
- **El campo `ns` de un atributo y de un elemento es la posición del URI, no la del prefijo.**
  Medido con un layout de dos namespaces (`tools:` incluido): los trozos de namespace llevan
  prefijo (3) y URI (4), y el atributo apunta a **4**.
- **`idIndex`, `classIndex` y `styleIndex` son índices 1-based** dentro de la lista ya ordenada de
  atributos del propio elemento, y 0 si no hay. Medido: `android:id` el primero del `Button` da 1, y
  `style` el último del `TextView` da 4.
- **El valor crudo solo existe para las cadenas**, y entonces crudo y datos apuntan a la MISMA cadena
  del pool (`text="Hello World!"` → raw 7 y datos 7). En los tipos numérico, de referencia y de enum o
  flags el crudo vale −1. Es el campo que me faltó en el primer intento: salían 2 bytes distintos en
  el layout y 8 en el manifiesto, y los ocho eran siempre los cuatro bytes de `datos` de un valor de
  tipo `0x03`.
- **Los tipos de valor que usan los dos ficheros**, copiados crudos: `0x01` referencia, `0x03` cadena,
  `0x10` entero decimal (con `0xffffffff` para −1 y `0xfffffffe` para −2, que son `match_parent` y
  `wrap_content`), `0x11` entero hexadecimal (`gravity="center"` → 0x11), `0x12` booleano (`true` →
  0xffffffff, `false` → 0).
- **`aapt2` tira los atributos de los namespaces que no conoce.** Probado con `tools:ignore` y
  `tools:visibility` sobre un `FrameLayout`: ni los atributos ni la declaración `xmlns:tools` llegan
  al binario (el fichero de tres namespaces declarados tiene un solo `START_NAMESPACE` y cinco
  cadenas). Así que la decisión de qué se descarta es del que enlaza, no del escritor.
- **Cuatro atributos del manifiesto no están en el fuente**: `compileSdkVersion` (34),
  `compileSdkVersionCodename` ("14"), `platformBuildVersionCode` (34) y `platformBuildVersionName`,
  que sale como **entero** 14 con crudo −1, no como cadena.

Comprobación:

```sh
go test -C apkc -run "EscribeAXML" -v .
# y por fuera, con el oráculo que no comparte código con nosotros:
env APKC_ESCRIBE_AXML=$PREFIX/tmp/axml-out go test -C apkc -run EscribeAXMLContraFixture .
mkdir -p $PREFIX/tmp/axml-out/paquete/res/layout && cd $PREFIX/tmp/axml-out &&
  cp AndroidManifest.xml paquete/ && cp main.xml paquete/res/layout/ &&
  cp $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/build/resources.arsc paquete/
(cd paquete && zip -X -0 -q ../nuestro.apk AndroidManifest.xml resources.arsc res/layout/main.xml)
A=$HOME/Android/build-tools/34.0.0/aapt2
for f in AndroidManifest.xml res/layout/main.xml; do
  $A dump xmltree --file $f nuestro.apk > n.txt; $A dump xmltree --file $f .../build/app.apk > a.txt
  diff n.txt a.txt
done
```

**Resultado**: dos pruebas en verde (la byte a byte sobre los dos ficheros, con su lectura de vuelta
por el lector de la 3-B, y la de los seis rechazos: atributo de namespace sin id, atributo sin
namespace con id, prefijo no declarado, dos ids para el mismo nombre, documento sin raíz y declaración
de namespace sin URI). El `aapt2 dump xmltree` responde rc 0 sobre nuestro paquete y su volcado es
idéntico al del apk de aapt2 en los dos ficheros. Suite nativa: **149 PASS / 0 FAIL / 1 SKIP** de
primer nivel (151 con subpruebas); la misma binaria en `GOOS=js` en el hospedador de navegador **144
PASS + 6 SKIP, rc 0** (146 con subpruebas).

## 31. La `R` sale de nuestra propia tabla, y el texto es el mismo que escribe `aapt2 link --java`

Paso 3.4: `apkc/r.go` genera `R.java` a partir del modelo que alimenta a `escribeTablaRecursos`, así
que los números de la `R` y los del `resources.arsc` **son la misma cuenta**, no dos programas
contándose el uno al otro.

Medido sobre el proyecto del studio y sobre la variante con siete tipos (`link --java genD`):

- El id es `número de paquete << 24 | id de tipo << 16 | INDICE de la entrada dentro del tipo`. Con
  dos entradas por tipo: `id_bare=0x7f040000` y `pulsar=0x7f040001`; `AppTheme=0x7f070000` y
  `StyBare=0x7f070001`. No es un desplazamiento en bytes: es la posición.
- Las clases anidadas van en el orden de los tipos (alfabético por nombre de tipo, que es el orden en
  que aapt2 los numera) y las entradas en el orden en que se emiten, también alfabético por nombre de
  recurso: con `normal`, `con-guion.y.punto` y `str_simple` los ids 0x7f060000, 0x7f060001 y
  0x7f060002 caen en ese orden.
- El nombre se desfigura a identificador de Java: **`con-guion.y.punto` sale `con_guion_y_punto`** —
  fuera de letras, dígitos, `_` y `$`, todo carácter pasa a `_`.
- El texto, con `cat -A`: cabecera de siete líneas de comentario, línea en blanco, `package X;`,
  línea en blanco, `public final class R {`, **dos espacios** por nivel, `= ` sin espacios alrededor
  (`int pulsar=0x7f010000;`), y el fichero **termina en la llave de cierre sin salto de línea**: 493
  bytes exactos para el proyecto del studio.
- Confirmación cruzada del §29: `aapt2 dump resources` etiqueta **PUBLIC** a las entradas declaradas
  con `<public>` — las que llevaban `0x0002` en `ResTable_entry` — y NO etiqueta a los estilos, que
  llevaban `0x0001`. Así que el bit público es el `0x0002` y el `0x0001` de un valor complejo es otra
  cosa; el escritor no lo interpreta, lo copia.

Comprobación:

```sh
A=$HOME/Android/build-tools/34.0.0/aapt2; J=$HOME/Android/platforms/android-34/android.jar
$A link -o out.apk -I $J --manifest AndroidManifest.xml --auto-add-overlay -R flats.zip \
   --java genjava                       # y comparar genjava/…/R.java con lo que escribe r.go
go test -C apkc -run "TestRLaMismaQueAapt2|TestRDesfiguraNombresYOrdena|TestRRechaza" -v .
```

**Resultado**: tres pruebas en verde — la R nuestra es idéntica byte a byte a los 493 bytes de aapt2,
la de la variante de siete tipos casa con las líneas medidas, y rechaza clave inexistente, tipo sin
nombre, entradas fuera de orden y dos nombres que chocan al desfigurarlos. Suite nativa **152 PASS /
0 FAIL / 1 SKIP** de primer nivel (154 con subpruebas); la binaria `GOOS=js` en el hospedador de
navegador, **rc 0** con 147 PASS + 6 SKIP de primer nivel (149 con subpruebas).

Con esto la Fase 3 tiene escritas **las tres estructuras que produce `aapt2`** —tabla de recursos,
AXML del manifiesto y AXML del layout— y la `R`, todas contra el oráculo: falta 3-E, que es
empaquetarlas en un apk y verificarlo por fuera.

## 32. Un apk hecho enteramente en Go: firma v1 y v2, alineado, y `aapt2 dump badging` lo entiende

Paso 3-E: `apkc/empaquetar.go` reúne los artefactos que ya escriben este fork —el AXML del manifiesto,
el del layout, la tabla de recursos y el `classes.dex` que saca el dexer de la Fase 2 de las seis
clases de javac fijadas— y los mete en el apk con el **mismo** empaquetador y el **mismo** firmante que
usa `apkc build --signer go` y `apkc sign`. No pasa `zipalign` ni pasa `apksigner`: la alineación se
decide entrada a entrada (almacenada a cuatro, nativa a página) y la firma se calcula sobre lo que va
saliendo.

El resultado son **9868 bytes con siete entradas** y un `classes.dex` de 2392, que es el tamaño que el
dexer viene dando desde el §22. Verificación con los oráculos que no comparten código con nosotros:

```sh
go test -C apkc -count=1 -run "TestApkCompiladoSinAapt2|TestEmpaquetaRechazaNombresDeRuta" -v .
# deja el paquete si se le dice dónde escribirlo:
env APKC_ESCRIBE_APK=$PREFIX/tmp/flagprobe/go-built.apk \
  go test -C apkc -count=1 -run TestApkCompiladoSinAapt2 .
BT=$HOME/Android/build-tools/34.0.0; A=$PREFIX/tmp/flagprobe/go-built.apk
$BT/apksigner verify --verbose $A
$BT/zipalign -c -p 4 $A
$PREFIX/lib/jvm/java-21-openjdk/bin/jarsigner -verify $A
python3 ~/Develop/Go/apkc-harness/zipcheck.py $A
$BT/aapt2 dump resources $A
$BT/aapt2 dump xmltree --file AndroidManifest.xml $A
$BT/aapt2 dump xmltree --file res/layout/main.xml $A
$BT/aapt2 dump badging $A
```

**Lo que devolvieron**:

- `apksigner verify --verbose`: `Verifies`, **v1 true y v2 true** (v3 false, que es lo que firma este
  firmante), un firmante con el certificado `CN=Android Debug`.
- `zipalign -c -p 4`: rc 0, sin quejas. `zipcheck.py`: «7 entries, every record agrees with itself;
  0 stored .so at a page boundary, **1 other stored entries at 4**» — la entrada almacenada es
  `resources.arsc`, que es la que el sistema mapea.
- `jarsigner -verify`: `jar verified.`
- `aapt2 dump resources` lee nuestra tabla dentro del paquete (`0x7f010000 id/pulsar`,
  `0x7f020000 layout/main`, el estilo con sus cinco ítems) y `aapt2 dump xmltree` lee los dos AXML.
- `aapt2 dump badging` — que es el que dereferencia el manifiesto contra la tabla — responde
  `package: name='com.hola.apkcstudio' versionCode='1' versionName='0.1'`, `minSdkVersion:'21'`,
  `targetSdkVersion:'33'`, `application: label='Hola apkc'` y
  `launchable-activity: name='com.hola.apkcstudio.MainActivity'`. Si el `@style/AppTheme` del
  manifiesto no apuntara a un id que existe en nuestra tabla, aquí se quejaría.

Por dentro, la prueba también comprueba lo que escribe el zip: `resources.arsc` viaja
`zip.Store` y su dato empieza en múltiplo de cuatro, los dos AXML y la tabla se releen con nuestros
lectores, y el dex conserva la cabecera `dex\n035`.

**Comando de la suite**: nativa **154 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del hospedador de
navegador **150 PASS + 7 SKIP, rc 0** —el apk se salta el caso firmado porque el hospedador no tiene
`~/.android/debug.keystore`, igual que las otras cuatro pruebas que lo necesitan.

Lo que **todavía no** está en Go, dicho para que no se lea como hecho: este apk se ensambla desde los
**modelos** que escribieron las pruebas (los recursos del proyecto del studio ya traducidos a
estructuras). Falta el paso que lee el `res/values/*.xml`, el layout y el manifiesto **en texto** y
produce esos modelos, y la tabla de ids del framework —que hoy la da `android.jar`— para que un
layout cualquiera use atributos que no estén entre los doce del estudio. Con esos dos puentes,
`apkc build` deja de necesitar `aapt2`. Sigue quedando `javac` (Fase 1G) y la página.

## 33. La tabla de ids del framework, leída de `android.jar` con nuestro propio lector

El puente que faltaba para los atributos: `apkc/marco.go` anda el `resources.arsc` del SDK y saca el
mapa `nombre → id` del paquete `android`, que es lo que `aapt2` presta hoy al resolver
`android:layout_width` o `@style/TextAppearance.Large`.

Lo medido sobre el `android.jar` de este teléfono (API 34, `resources.arsc` de 16.390.624 bytes):

- La tabla declara **cuatro paquetes y los cuatro llevan id 1 y se llaman `android`**: el primero mide
  5.355.916 bytes, trae **23 tipos** y **12.877 claves**; los otros tres (4.656, 7.604 y 13.276)
  declaran 255, 223 y 221 tipos. **Los ids que imprime `aapt2 dump resources` son los del primero**;
  leer los siguientes pisaba el mapa con índices de otro espacio, y eso fue exactamente lo que dio la
  primera versión de este lector: 221 tipos y nombres como `?1`. Se corta en el primer paquete con
  id 1.
- El **pool global no se materializa**: 11.009.160 bytes con 152.615 cadenas y **1.307 estilos**
  (`styleCount` distinto de cero, algo que los pools de una app nunca traen). Se salta por su tamaño
  declarado; los dos pools de nombres del paquete sí se leen, y `leePool` devuelve ahora también el
  `styleCount` para poder rechazar en vez de suponer.
- Un tipo del framework **no es un bloque**: son muchos, uno por configuración, y solo uno cubre el
  espacio de entradas que anuncia su `RES_TABLE_TYPE_SPEC` (el de `string` tiene un bloque de 2492
  entradas y luego bloques de 1, 2, 3, 4, 9… con `flags 0x01`). Los índices de un bloque parcial **no
  son el id**; el lector toma el primer bloque cuya cuenta iguale a la del spec.
- Dentro de un bloque denso la entrada ausente se marca con **offset `0xFFFFFFFF`**, no con un bit de
  cabecera: medido en la entrada #616 del tipo `attr`, con `flags 0x00` en el bloque. El índice sigue
  siendo el lugar en el array, así que se salta la entrada y no la numeración.
- El `u16` del spec en `+10` —el que en una app vale el número de configuraciones— vale **0** en el
  framework, y las banderas de sus entradas `attr` valen `0x0003`.

Comprobación contra el oráculo que no comparte código:

```sh
J=$HOME/Android/platforms/android-34/android.jar
$HOME/Android/build-tools/34.0.0/aapt2 dump resources $J > marco.txt   # 240.637 lineas
grep -oE "resource 0x[0-9a-f]{8} (attr|color|style|dimen)/[^ ]+" marco.txt | awk '{print $2, $3}' | sort -u
go test -C apkc -count=1 -run TestMarco -v .
```

**Resultado**: los **28 ids** copiados del volcado de aapt2 coinciden con lo que lee `marco.go` —los
veintidós atributos que usan los ficheros del studio (`attr/theme 0x01010000` …
`attr/compileSdkVersionCodename 0x01010573`), `color/white 0x0106000b`, `color/black 0x0106000c`,
`style/TextAppearance.Large 0x01030042`, `style/Theme.DeviceDefault.Light 0x0103012b` y
`dimen/app_icon_size 0x01050000`—, el paquete da 23 tipos con `attr`, `id`, `style` como tres
primeros (**el orden del framework no es alfabético: es el de `public.xml`**), el tipo `attr` aporta
más de 1500 nombres y un nombre inventado se rechaza. Los rechazos probados: tabla sin paquete id 1
(es nuestra propia tabla de app), primer trozo que no es tabla, `packageCount` mentiroso y fichero
vacío.

Arreglado de paso un **panic real** que encontró el test nuevo: `leeTabla`, `leeAXML` y `leeMarco`
comprobaban el largo en la condición y **dentro del mensaje de error volvían a llamar a `u16re` sobre
el fichero corto**, así que un XML o una tabla de pocos bytes producía `index out of range` en lugar
de un rechazo. Los tres comprueban el tamaño antes de leer, y
`TestLectoresRechazanFicheroCorto` pasa longitudes de 0, 1, 7, 8, 11, 12 y 27 bytes por los cuatro
lectores. Importa porque en la página el fichero lo sube el usuario.

Suite: nativa **157 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del hospedador de navegador **152
PASS + 8 SKIP, rc 0** (el marco se salta allí, porque el hospedador no tiene `android.jar`).

## 34. Cómo define el framework un atributo: los `enum` y `flags` se guardan como recursos de tipo `id`

Para resolver `wrap_content` o `center` sin `aapt2` hace falta saber qué valores admite cada atributo, y
eso está dentro de la misma tabla del jar. Medido sobre `attr/layout_width` (entrada 0x0f4 del tipo
`attr`) y `attr/gravity` (entrada 0x0af):

- El valor de un `attr` es una **entrada compleja de 16 bytes** con `flags 0x0003`, `padre 0x00000000`
  y un recuento, seguida de exactamente `recuento` registros de doce bytes: la misma forma del §27 para
  los estilos.
- **El primer registro no es un valor**: su nombre es `0x01000000` y sus datos son la máscara de formato
  del atributo (`layout_width` lleva `0x00010040`). Se descarta por esa forma, no por posición.
- Los registros de `enum` y `flags` llevan el **nombre referido como recurso de tipo `id`**, no como
  cadena: `wrap_content` = `0x010205af`, `fill_parent` = `0x010202d7`, `center` = `0x01020240`,
  `center_horizontal` = `0x01020243`. `aapt2 dump resources` lo confirma:
  `0x01020240 id/center`, `0x010205af id/wrap_content`.
- Los datos del registro son el valor que hay que escribir, **con el tipo que el framework eligió**:
  `wrap_content` → `tipo 0x10` (entero decimal) `datos 0xfffffffe`; `center` → `tipo 0x11` (entero
  hexadecimal) `datos 0x11`. Es decir: el tipo del `Res_value` no se decide en el enlazador, se copia
  del atributo.

`marco.go` lo expone en dos pasadas, porque el tipo `attr` (id 1) se recorre **antes** que el tipo `id`
(id 2) y su mapa inverso aún está vacío: los registros se acumulan como `itemDeAtributo` y
`resuelveItems()` los traduce al terminar el paquete. La prueba compara cuatro valores concretos contra
el volcado de `aapt2 dump resources` (`layout_width/wrap_content` y `fill_parent`,
`gravity/center`, `center_horizontal` y `center_vertical`) y el `nombreDeId` de `0x0103012b`.

**Resultado**: tres pruebas de marco en verde. Nativa **158 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js`
dentro del hospedador de navegador **152 PASS + 9 SKIP, rc 0**.

## 35. Del `res/values/styles.xml` en texto a la misma tabla y a la misma `R`

Primera mitad del paso 3-G: `apkc/valores.go` lee un `res/values/*.xml` en texto con el decodificador
de Go y produce el modelo `tablaEscribir`. El fuente queda fijado en base64 (423 bytes,
`sha256 a490b9438728f1af…`) —el mismo `styles.xml` con el que aapt2 dejo los ficheros del §24— y la
prueba no muestrea: compara el modelo contra el que escribió a mano `arscwriter_test.go`.

Lo que se comprueba, en tres niveles:

- El modelo sale **igual campo a campo** (`reflect.DeepEqual` en tipos, claves, pool global, y por cada
  entrada su clave, sus banderas, su valor, su padre y sus cinco registros). Las claves salen en el
  orden medido —`pulsar`, `main`, `AppTheme`—, los tipos alfabéticos con id por posición, los registros
  del estilo ordenados por id de atributo (`0x01010054`, `0x01010056`, `0x010102cd`, `0x01010451`,
  `0x010104e0`) y el padre `@android:style/Theme.DeviceDefault.Light` resuelto contra la tabla del jar a
  `0x0103012b`, que es el número que está en los bytes de aapt2.
- Escrito el modelo, **los bytes siguen siendo los del fixture**: 936 bytes y los primeros 752 idénticos
  a los de aapt2 con los dos campos de tamaño sustituidos, la misma comprobación del §29.
- La `R` que sale del mismo modelo **es el texto de aapt2**, el de 493 bytes del §31.

Las formas de los valores no se inventan: `string` es `0x03` con el texto en el pool global, `color`
`0x1d` con el ARGB, `bool` `0x12` con −1 o 0, `integer` `0x10`, `item type="id"` un booleano a cero con
banderas `0x0004` en la entrada, y todo eso se vio en las variantes del §29. Un `<color>` cuyo texto no
es color, un `<bool>` que no dice `true`/`false`, un `<integer>` que no es número, un tipo de recurso no
medido (`dimen`, un `type=` raro), un item de estilo que no sea `android:`, un padre inexistente en el
framework, un `<string>` con un hijo, o una referencia al paquete propio intentando resolverse antes de
numerar la tabla: **los siete se rechazan con nombre y razón**, y eso es la segunda prueba del fichero.

```sh
go test -C apkc -count=1 -run "TestValores" -v .
```

**Resultado**: dos pruebas en verde. Nativa **160 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del
hospedador de navegador **154 PASS + 9 SKIP, rc 0** —esta parte corre allí también porque el marco que
necesita el estilo está reconstruido a mano en la prueba con los ids y valores que §33 y §34 copiaran de
`aapt2 dump resources`, y el hospedador no tiene `android.jar`.

**Lo que queda de 3-G**: la otra mitad, que es la que exige las dos pasadas —leer layouts y manifiesto a
árbol, descubrir los `@+id/…` (que añaden entradas a la tabla), numerar, y entonces escribir los AXML con
las referencias resueltas y los cuatro atributos que el manifiesto gana. Las restricciones están escritas
en `docs/plan.md`.

## 36. El árbol en texto: las líneas que anota aapt2 no son las que da el decodificador de Go

`apkc/arbol.go` convierte el layout y el manifiesto en texto en un árbol con nombres, **líneas**,
prefijos y orden de hijos. Es la pieza que faltaba para la segunda mitad de 3-G, y trae una medición
sobre la propia librería de Go, que es la que decide su forma:

- Sondeado con `encoding/xml` (el programa está en `$PREFIX/tmp/probeXML/sondeo.go`, 24 líneas): en una
  declaración `xmlns:android="…"` el decodificador responde `Space="xmlns"` y `Local="android"`, o sea
  **el prefijo**; en un atributo `android:orientation` responde `Space="http://schemas.android.com/apk/res/android"`,
  o sea **el URI ya resuelto**. El binario quiere el prefijo en el nodo y el URI en el campo `ns`, así
  que se guarda el mapa en las dos direcciones.
- Y la que no se veía venir: **`InputPos()` cae detrás del `>` de la etiqueta de apertura**. En el
  manifiesto del studio `<activity` abre en la línea 13, sus cuatro atributos ocupan hasta la 17, y el
  decodificador informa 17. `aapt2` anota **13** (medido sobre el binario con el instrumento del §24, y
  vuelto a medir en el §30). La línea se busca entonces en el texto con un cursor que solo avanza,
  buscando el `<nombre` siguiente. La prueba `TestArbolDelManifiesto` comprueba las siete líneas del
  binario: manifiesto 1, uses-sdk 6, application 10, **activity 13**, intent-filter 19, action 20,
  category 21.
- El orden de los atributos en el árbol es el del **fuente**, no el del binario: en el `TextView` el
  `style` sin prefijo es el primero leído y aapt2 lo escribe el último. Eso es materia del escritor de
  AXML (que ya ordena como aapt2, §30), y se comprueba aquí para que el lector no lo pierda.

Rechazos probados sobre el propio lector: documento sin raíz, dos raíces, cierre mal anidado, elemento
sin cerrar, texto suelto dentro de un elemento, y un comentario entre dos raíces. La forma buena
(declaración `<?xml …?>`, `xmlns:android` y un hijo) se acepta como control de que los cinco rechazos no
son un error que rechace todo.

```sh
go test -C apkc -count=1 -run "TestArbolDel|TestArbolRechaza" -v .
```

**Resultado**: tres pruebas en verde. Nativa **163 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del
hospedador de navegador **157 PASS + 9 SKIP, rc 0**.

## 37. Enlazado: de los tres ficheros de texto a los tres binarios de aapt2, byte a byte

`apkc/enlazar.go` junta las dos mitades del paso 3-G —`arbol.go` (layout y manifiesto en texto) y
`valores.go` (los `res/values/*.xml`)—, descubre los `@+id/…` que el layout declara, numera la tabla con
ellos y entonces materializa los modelos. Con eso el proyecto del studio se enlaza **sin `aapt2`** y lo
que sale se compara contra los binarios fijados:

| salida | contra lo que escribió aapt2 | resultado |
| --- | --- | --- |
| `res/layout/main.xml` (AXML) | los 784 bytes del fixture | **idéntico byte a byte** |
| `AndroidManifest.xml` (AXML) | los 2000 bytes del fixture | **idéntico byte a byte** |
| `resources.arsc` | los 1088 bytes del fixture | 936 escritos; los **primeros 752 idénticos** salvo los dos campos de tamaño, y la diferencia es la declarada en el §28 (una configuración en vez de dos) |
| `R.java` | los 493 bytes de `link --java` | **idéntico byte a byte** |

Ninguna de las cuatro comparaciones es contra un modelo propio: se compara contra **los ficheros que dejó
la herramienta que este fork quiere sustituir**, fijados en base64.

Dos fallos que encontraron estas pruebas y que valen como medición:

- **`match_parent` no estaba en el marco de la prueba** y caía al último caso de `valorDeAtributo`, la
  cadena: el layout salía con 15 cadenas en vez de 14, 20 bytes de más y **472 bytes distintos**. El §34
  ya había medido que `layout_width` trae cuatro registros y que dos de ellos valen `0xffffffff`
  (`fill_parent` en `0x010202d7` y otro en `0x010203a0`, que es `match_parent`); puesto el item, el layout
  sale exacto. `TestMarcoValoresDeAtributo` exige ahora `match_parent` contra el jar, para que el marco
  escrito a mano no pueda volver a quedarse corto.
- **`platformBuildVersionName` vale 14, no 34.** El primer intento lo copiaba de `compileSdkVersion` y
  daba un solo byte distinto en todo el manifiesto (el `#1368`: `0x22` contra `0x0e`). Los cuatro campos
  que añade el enlace salen de dos números distintos: `compileSdkVersion` y `platformBuildVersionCode` son
  la API (34) y `compileSdkVersionCodename` y `platformBuildVersionName` son la **versión de Android**
  ("14" como texto y 14 como entero, con crudo −1). El enlace deriva el entero del codename y rechaza un
  codename no numérico, en vez de inventarse la forma.

Rechazos probados en el enlazador: manifiesto sin `package`, un atributo `android:` que no esté en la
tabla del framework, una referencia `@style/NoExiste` del paquete propio, una referencia
`@android:style/…` inexistente y una referencia mal formada.

```sh
go test -C apkc -count=1 -run "TestEnlazar|TestValores|TestArbol|TestMarco" -v .
sh -c 'cd studio && APKC=$PREFIX/tmp/apkc-gate APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm sh e2e.sh'
```

**Resultado**: once pruebas en verde en ese conjunto. Nativa **166 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js`
dentro del hospedador de navegador **160 PASS + 9 SKIP, rc 0** —el enlace corre allí también, con el marco
reconstruido a mano sobre los números que §33 y §34 copiaran del jar—. Y el portero del repo, `sh e2e.sh`
con el binario de este fork, sigue diciendo **`e2e: todo en orden`**, con sus 16 comprobaciones de la
pestaña conducida por chromium.

Con esto **la Fase 3 queda cerrada de punta a punta**: texto del proyecto → tabla, dos AXML y `R`, y de ahí
el apk firmado del §32. Lo que queda del objetivo declarado (compilar en la pestaña sin conexión) son dos
piezas grandes: la Fase 1G, que es escribir el `javac` en Go, y el cableado de la página.

## 38. `apkc build --linker go`: un apk real sin ejecutar aapt2, y sus productos comparados

`apkc/linker.go` engancha el enlace de Go en la pipeline de siempre. `--linker` toma `aapt2` (el
default, que es lo que apkc hizo toda su vida) o `go`; con `go`, `compileRes` no ejecuta nada —el `.flat`
solo alimentaba a `aapt2 link`, y eso ya se dijo en el §24— y `bundleRes` escribe los cuatro productos que
el plan del paquete (`packagePlan`) ya sabe tomar: `build/AndroidManifest.xml`, `build/resources.arsc`
(almacenado, porque el sistema lo mapea), `build/res/layout/*.xml` y `src/<paquete>/R.java`. El empaquetado
y la firma no se tocan.

Corrido sobre el proyecto del studio en una copia limpia:

```sh
apkc build --linker go
# [build] linking resources with the Go linker
# [timing] aapt2 link 226ms  <- el enlace, con el nombre del tramo de siempre
# [build] compiling java files / javac 3.77s
# [build] bundling classes / d8 app 4.813s
# [build] packaging and signing inside apkc
```

Contra los productos del proyecto enlazado con aapt2:

| producto | aapt2 | go | resultado |
| --- | --- | --- | --- |
| `build/AndroidManifest.xml` | 2000 | 2000 | **idéntico byte a byte** |
| `build/res/layout/main.xml` | 784 | 784 | **idéntico byte a byte** |
| `src/…/R.java` | 493 | 493 | **idéntico byte a byte** |
| `build/classes.dex` | 2392 | 2392 | **idéntico** (lo produce el dexer del §22, no cambió) |
| `build/resources.arsc` | 1088 | 936 | distinto **por lo declarado en el §28**: una configuración base con los cinco ítems en vez de base + `-v23` |

Y el apk resultante (10194 bytes) pasa los oráculos de siempre:

```sh
apksigner verify --verbose build/app.apk   # Verifies, v1 true, v2 true
zipalign -c -p 4 build/app.apk             # rc 0
python3 ~/Develop/Go/apkc-harness/zipcheck.py build/app.apk
# 7 entries, every record agrees with itself; 1 other stored entries at 4
aapt2 dump badging build/app.apk           # package, versiones, label, launchable-activity
```

**El portero, y un agujero que tapó el test.** `fuentesDelProyecto` rechaza con nombre lo que este enlace
no sabe escribir: `res/drawable` y compañía (piden copiar ficheros y poner entradas con referencia de
fichero), **cualquier calificador** —`res/layout-es`, `drawable-v21`, `values-land`— porque es una segunda
configuración y aquí se emite una sola (§28), subcarpetas dentro de `values` o `layout`, recursos que no
sean `.xml`, un manifiesto con `<permission>` (aapt2 escribe además un `Manifest.java` que aquí no se
produce), un proyecto sin manifiesto, y una `android.jar` cuya versión no esté medida (`opcionesDelProyecto`
saca 34 y "14" de la ruta del platform; API 21 se rechaza en vez de adivinar el codename). El agujero era
precisamente el calificador: la primera versión partía el nombre por el guion y **dejaba pasar
`res/layout-es`**, que habría producido un apk al que le faltaran los recursos de ese idioma sin decir
nada. `TestFuentesDelProyectoRechazaLoQueNoSabeEnlazar` exige ahora los seis rechazos y que cada mensaje
nombre el motivo.

**Resultado**: suite nativa **169 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del hospedador de
navegador **163 PASS + 9 SKIP, rc 0**. `javac` y `d8` siguen ejecutándose en este camino: lo que ya no se
ejecuta es `aapt2`.

## 39. `apkc link` dentro del hospedador con forma de navegador: los recursos, compilados del lado del cliente

Tercer comando del fork que no levanta proceso (después de `dex` y de `sign`): `apkc link` hace **solo el
enlace de recursos**, con `--android-jar` apuntando al jar del SDK y `-o` al directorio del proyecto. Se
midió corriéndolo en `shim/navegador-sim.js` —el hospedador con forma de navegador, un `fs` en memoria,
sin red y sin proceso—, con el `android.jar` montado como un fichero más:

```sh
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o $PREFIX/tmp/apkc-link.wasm .'
sh -c 'cd shim && node navegador-sim.js \
  --montar $P/AndroidManifest.xml:/proyecto/AndroidManifest.xml \
  --montar $P/res/values/styles.xml:/proyecto/res/values/styles.xml \
  --montar $P/res/layout/main.xml:/proyecto/res/layout/main.xml \
  --montar $HOME/Android/platforms/android-34/android.jar:/sdk/android.jar \
  --exportar /proyecto/build/AndroidManifest.xml:$O/AndroidManifest.xml \
  --exportar /proyecto/build/resources.arsc:$O/resources.arsc \
  --exportar /proyecto/build/res/layout/main.xml:$O/main.xml \
  --exportar /proyecto/src/com/hola/apkcstudio/R.java:$O/R.java \
  $PREFIX/tmp/apkc-link.wasm link --android-jar /sdk/android.jar --sdk-version 34 -o /proyecto'
```

**Resultado**: `3,22 s` (leer el jar de 26 MB y andar su tabla de 16,39 MB, todo en el hospedador) y los
cuatro productos exportados: **`AndroidManifest.xml` idéntico byte a byte al de aapt2, `main.xml` idéntico,
`R.java` idéntica**, y `resources.arsc` de 936 bytes frente a los 1088 de aapt2 por la única configuración
base declarada en el §28.

Tres cosas que esta corrida obligó a arreglar, y que en el teléfono no habrían aparecido:

- **`main()` sondaba el SDK antes de despachar cualquier orden** (`findSDKs`), y en el hospedador eso
  terminaba el comando con `[doctor] open /android/build-tools: No such file or directory` antes de
  empezar. `link` y `dex`, que son los dos que no levantan proceso, quedan fuera de ese sondeo:
  `default: findSDKs()`.
- **La versión del platform se leía de la ruta del jar** (`platforms/android-34/android.jar`). En la
  pestaña el usuario monta el jar donde quiere y la ruta no informa nada, así que `--sdk-version` y
  `--codename` existen; la ruta sigue siendo el defecto cómodo para `apkc build`. Un `--codename` sin
  `--sdk-version`, o una versión cuyo nombre no esté medido (API 21, por ejemplo), se rechazan con nombre.
- **El orden de argumentos del hospedador**: `navegador-sim.js <wasm> <orden> …`, la orden primero. Llamado
  con la ruta delante, `apkc` interpretaba `/proyecto` como el nombre del comando y no pasaba de ahí.

El control de que los rechazos del nuevo portero no son un error que rechace todo sigue en
`linker_test.go`, y la suite nativa y la del hospedador (las 163 + 9 del §38) no cambiaron con el arreglo.

**Y el portero del studio, con una nota sobre su intermitencia.** `sh e2e.sh` con el binario nuevo dijo
**`e2e: todo en orden`** (16 comprobaciones de la pestaña, apk de 10348 bytes firmado dentro de chromium y
verificado por fuera). En una corrida anterior, con el mismo binario, habia fallado tres: la que
sobrevivía al filtro imprimía

```
--- nodos tras recargar: ["/proyecto:dir","/proyecto/AndroidManifest.xml:0b","/proyecto/res/layout:dir","/proyecto/src:dir"]
```

o sea que al recargar la pestaña el árbol persistido en OPFS estaba **ahi pero con ficheros de cero
bytes**. Eso cambia el diagnóstico que estaba escrito (`create` falla a veces): el fallo no esta en lo que
genera el proyecto, esta en el **persistido** —se consulta el árbol antes de que el contenido este
escrito—, y por eso es intermitente y por eso la comprobacion falla al recargar y no al crear. Queda
anotado con su salida, no como sospecha; reproduce una de cada varias corridas y pasa en la siguiente.

## 40. El `.class` que hay que escribir: la plantilla de `apkc create`, medida con `javap -v -p`

Objetivo de la Fase 1G, medido antes de escribir el emisor. El fuente es la plantilla tal cual la genera
`apkc create` (`MainActivity.java`, 17 líneas) y la `R.java` que escribe **nuestro** enlazador de la
Fase 3, con `com.hola.j1g` como paquete:

```sh
$PREFIX/tmp/apkc-gate create $PREFIX/tmp/j1g/proyecto           # plantilla, con las respuestas al prompt
cd $PREFIX/tmp/j1g/proyecto && $PREFIX/tmp/apkc-gate link \
  --android-jar $HOME/Android/platforms/android-34/android.jar --sdk-version 34 --codename 14
$PREFIX/lib/jvm/java-21-openjdk/bin/javac -nowarn -d $PREFIX/tmp/j1g/classes \
  -classpath $HOME/Android/platforms/android-34/android.jar \
  src/com/hola/j1g/R.java src/com/hola/j1g/MainActivity.java     # rc 0
$PREFIX/lib/jvm/java-21-openjdk/bin/javap -J-Xmx256m -v -p -classpath $PREFIX/tmp/j1g/classes \
  com.hola.j1g.MainActivity com.hola.j1g.R 'com.hola.j1g.R$layout'
```

`javac` del JDK 21 del teléfono deja **cuatro** clases: `MainActivity.class` de 445 bytes, `R.class` de
324, `R$layout.class` de 323 y `R$style.class` de 325. Todas con `major version: 65` y
`flags: (0x0021) ACC_PUBLIC, ACC_SUPER` (la `R` añade `ACC_FINAL`, o sea `0x0031`).

El `onCreate` completo, que es el cuerpo que hay que emitir:

```
  public void onCreate(android.os.Bundle);
    descriptor: (Landroid/os/Bundle;)V
    flags: (0x0001) ACC_PUBLIC
    Code:
      stack=2, locals=2, args_size=2
         0: aload_0
         1: aload_1
         2: invokespecial #7                  // Method android/app/Activity.onCreate:(Landroid/os/Bundle;)V
         5: aload_0
         6: ldc           #13                 // int 2130771968
         8: invokevirtual #14                 // Method setContentView:(I)V
        11: return
      LineNumberTable:
        line 14: 0
        line 15: 5
        line 16: 11
```

y el `<init>` que **javac sintetiza** (en el fuente no hay constructor escrito), con su tabla de líneas
apuntando a la línea de la declaración de clase:

```
  public com.hola.j1g.MainActivity();
    Code:
      stack=1, locals=1, args_size=1
         0: aload_0
         1: invokespecial #1                  // Method android/app/Activity."<init>":()V
         4: return
      LineNumberTable:
        line 11: 0
```

Cuatro cosas se leyeron aquí y no en un manual:

- **`setContentView` se resuelve contra la clase actual.** `#14 = Methodref #15.#16` y `#15 = Class
  com/hola/j1g/MainActivity`, aunque el método se declara en `android.app.Activity` y `javap` lo imprime
  sin prefijo de clase. Del camino de superclases se toma la **descripción** (`(I)V`), no el dueño de la
  referencia. `super.onCreate` es lo contrario: `#7 = Methodref #2.#8` con `#2 = Class
  android/app/Activity`, porque ahí el receptor está escrito.
- **`R.layout.main` no es un `getstatic`.** Sale `#13 = Integer 2130771968` (`0x7f010000`) y un `ldc` de
  un byte de índice: javac dobla en el llamador la constante de compilación. En la `R$layout` ese mismo
  valor viaja como atributo del campo —`ConstantValue: int 2130771968`, `flags: (0x0019) ACC_PUBLIC,
  ACC_STATIC, ACC_FINAL`— y **no** deja método de inicialización estática: la clase tiene un solo método,
  el `<init>` sintetizado contra `java/lang/Object`.
- **`@Override` no deja huella.** `attributes: 2` en `MainActivity` (SourceFile e InnerClasses) y ningún
  atributo de anotaciones en los métodos: la anotación se puede descartar sin más. La `R` tiene tres
  atributos porque además lleva `NestMembers` (`com/hola/j1g/R$style`, `com/hola/j1g/R$layout`), y cada
  anidada tiene `NestHost`; eso es de Java 11 y **no viaja en la versión 52** que emite este fork.
- **Las líneas de la `LineNumberTable` son las del fuente**, contadas como las cuenta javac: la 11 es
  `public class MainActivity extends Activity {`, la 14 `super.onCreate(...)`, la 15 `setContentView(...)`
  y la 16 el `}` que cierra el método. El pool de `MainActivity` trae `#11 = Class com/hola/j1g/R$layout`
  y `#27 = Utf8 layout` solo por el `InnerClasses`, no por el código.

Con esto el emisor escribe la **versión 52** (Java 8): los 65 de javac 21 no aportan nada que el dexer o
`d8` necesiten, y 52 es la que este fork ya lee y dexa. El oráculo de la fase es el `javap -c -p` textual
de arriba, más abajo, y el dex que sale de ahí.

## 41. El `.class` escrito en Go: `javap` lo lee igual que el de javac, y el dex sale byte a byte idéntico

`apkc/javaemisor.go` emite las cuatro clases del parrafo 40. Dos oráculos; el segundo es el que cierra la
prueba.

```sh
# vuelco del emisor (prueba auxiliar, borrada despues de medir) y javap sobre lo que escribe
sh -c 'cd apkc && go test -count=1 -timeout 120s -run TestVuelcaParaJavap .'
J=$PREFIX/lib/jvm/java-21-openjdk/bin/javap
for k in MainActivity Rlayout R; do
  $J -J-Xmx256m -c -p -classpath $PREFIX/tmp/j1g/ours    "com.hola.j1g.$k" > a.txt
  $J -J-Xmx256m -c -p -classpath $PREFIX/tmp/j1g/classes "com.hola.j1g.$k" > b.txt
  diff a.txt b.txt
done   # Rlayout es R$layout, con el nombre entre comillas
```

**Primer oráculo**: `javap -c -p` sobre nuestras clases imprime **las mismas líneas de desensamblaje** que
sobre las de javac en las cuatro clases. Las únicas diferencias del `diff` son espacios de alineación de la
columna de comentarios, porque el índice de pool que javac escribe en dos dígitos aqui es de uno.

**Segundo oráculo**, dentro de la suite (`TestEmisorDexaLoQueDexaJavac`): las cuatro clases de javac viajan
como base64 en el test, y el test dexa las nuestras y las de javac con `dexDeClases`. **El dex sale idéntico
byte a byte** (1284 = 1284, cero bytes distintos), incluidas la firma SHA-1 y la `checksum` del encabezado,
que son lo primero que se rompe cuando el contenido difiere. No queda nada que comprobar del significado de
nuestros `.class`: el mismo lector, el mismo dexer y el mismo resultado que produce `javac` en el teléfono.

Tamaños: los nuestros son 467, 298, 304 y 306 bytes frente a los 445, 324, 323 y 325 de javac. La diferencia
es la versión 52 contra 65 (el `NestMembers`/`NestHost` de 8 bytes por clase y las entradas de pool que
javac interna para escribirlos) y nada más: el `Code` es el mismo. *Estas cuatro cifras y el 1284 se midieron
otra vez con `apkc compile` después del §42, que añadió las reglas de mención del `InnerClasses`; el oráculo
del párrafo anterior es el test, que sigue pasando.*

Tres defectos que solo aparecieron comparando con el de fuera, no con un manual:

- **El índice del pool estaba desplazado un puesto.** `agregar` devolvía `len(entradas) + 1` después de
  añadir, o sea que la primera entrada se llamaba 2 a sí misma: cada `CONSTANT_Class` apuntaba a su propio
  índice en vez de a su `Utf8`. Nuestro lector lo pasaba; **javap lo paró con un mensaje exacto**:
  `unexpected entry at #3 -- expected tag 7, found 1`. Con el índice correcto el conteo sigue siendo
  `última posición + 1`.
- **El `<init>` sintetizado va escrito primero.** javac pone el constructor antes que los demás métodos
  (medido: `methods: 2`, el primero `<init>`); emitirlo al revés cambiaba el orden de los `class_data` y
  saltaba el test de igualdad.
- **Doblar una constante no borra la referencia al tipo.** `R.layout.main` sale `ldc int` sin `getstatic`, y
  por eso el emisor no escribía ninguna `Class` de `R$layout`: el `InnerClasses` de MainActivity salía
  vacío. javac sí deja `#11 = Class com/hola/j1g/R$layout` en el pool **solo por el atributo** (medido en el
  §40), así que la mención se registra al resolver la cadena de nombres, aunque el código no la use.

Y dos reglas de formato que conviene tener escritas porque el código las hace a mano: la mágica es
`CA FE BA BE` (no `CAFED00D`, que es como la nombra el estándar), y `ldc` lleva el índice en **un byte** —
a partir de 256 entradas hay que pasar a `ldc_w`, y mientras eso no esté medido el emisor lo nombra
(`el pool de ... pasa de 255 entradas y ldc_w no esta medido`) en vez de truncar el operando.

La suite nativa pasa con **179 tests** (tres nuevos: la plantilla contra `javap`, el dex contra javac y el
portero de rechazos). El portero rechaza con nombre lo que este emisor no escribe: clase anónima (falta el
`this$0`), `if`, `long`/`double`/`float`, una anotación que no sea `@Override` y un tipo que no aparezca en
el fuente, en su paquete, en los imports ni en el `android.jar`.

## 42. La clase anónima: `this$0`, `EnclosingMethod`, `invokeinterface` y el dex idéntico de las seis clases

Lo que pedía la muestra del `studio/` (`menu.add("…").setOnMenuItemClickListener(new MenuItem.OnMenuItem-
ClickListener() { … })` con `Toast.makeText(MainActivity.this, "Clicked!", Toast.LENGTH_SHORT).show()`) se
midió antes de escribirlo, sobre las seis clases que `javac` dejó en `build/classes` del proyecto generado
por la página:

```sh
$J=$(echo $PREFIX)/lib/jvm/java-21-openjdk/bin/javap
S=$(echo $PREFIX)/tmp/studio-e2e-java/com.hola.apkcstudio
$J -J-Xmx256m -v -p -classpath $S/build/classes 'com.hola.apkcstudio.MainActivity$1'
$J -J-Xmx256m -c -p -classpath $S/build/classes com.hola.apkcstudio.MainActivity
```

**La anonima, `javap -v -p`**: `flags: (0x0020) ACC_SUPER` (ni pública ni final), `super_class`
`java/lang/Object` **aunque el fuente dice `new MenuItem.OnMenuItemClickListener()`** — una anonima sobre una
interface extiende Object e *implementa* la interface: `interfaces: 1, fields: 1, methods: 2, attributes: 4`.
El campo es `final com.hola.apkcstudio.MainActivity this$0;` con `descriptor: Lcom/hola/apkcstudio/MainActivity;`
y `flags: (0x1010) ACC_FINAL, ACC_SYNTHETIC`. El constructor es `MainActivity$1(MainActivity)` con
`flags: (0x0000)` y:

```
   0: aload_0
   1: aload_1
   2: putfield      #1   // Field this$0:Lcom/hola/apkcstudio/MainActivity;
   5: aload_0
   6: invokespecial #7   // Method java/lang/Object."<init>":()V
   9: return
  LineNumberTable:
    line 20: 0
  MethodParameters:
    Name        Flags
    <no name>   final mandated
```

o sea: **primero se guarda la captura y después se llama al padre**, la tabla de líneas apunta a la línea del
`new` (la 20), y el parámetro sintético lleva `MethodParameters` con `ACC_FINAL|ACC_MANDATED` (0x8010) y sin
nombre. El método implementado, `onMenuItemClick(MenuItem)`, sale `stack=3, locals=2` con
`aload_0; getfield this$0; ldc "Clicked!"; iconst_0; invokestatic Toast.makeText; invokevirtual Toast.show;
iconst_1; ireturn` en las líneas 22 y 23 — `MainActivity.this` es un `getfield`, `Toast.LENGTH_SHORT` es un
`iconst_0` (constante doblada del jar) y `show()` es `invokevirtual` porque el receptor es una clase.

**El `InnerClasses` tiene tres filas en el exterior y dos en la anonima**, y ahí estaba la parte que un
manual no dice. En `MainActivity`: `layout=R$layout of R`, despues `#28;` (la anonima, **sin exterior y sin
nombre**) y despues `OnMenuItemClickListener=class android/view/MenuItem$OnMenuItemClickListener of
android/view/MenuItem` — en **orden de primer uso**, no alfabetico ni de declaración. Y esa tercera fila
existe aunque la interface no aparezca en ningún opcode: solo está escrita dentro del descriptor de
`setOnMenuItemClickListener`. En la R, las tres anidadas van listas `style, layout, id`, o sea **al revés de
como se declaran**. Los flags de una anidada del framework salen `0x0609` en la fila (`public static`)
mientras su propio ClassFile dice `0x0601`: javac añade `ACC_STATIC` a la fila.

**Resultado con el emisor encima**: `javap -c -p` sobre las seis clases nuestras imprime lo mismo que sobre
las seis de javac, con la única diferencia del **número de pool** (`invokespecial #10` frente a `#1`), que no
es un hecho semántico. Y el oráculo duro, dentro de la suite (`TestEmisorMuestraDexaLoQueDexaJavac`): el dex
de nuestras seis clases es **idéntico byte a byte** al dex de las seis de javac, 2392 = 2392, cero
diferencias. La suite nativa pasa con **182 tests**.

Tres defectos que solo salieron comparando, y cada uno con su síntoma:

- **El descriptor de un constructor sin vuelta.** `new` armaba `(Lcom/hola/apkcstudio/MainActivity;)` y
  olvidaba la `V`. El lector no se quejaba; el dex salía con un `method_id`
  `MainActivity$1.<init>(…)I` y una cadena `L` de más, y el fichero crecía 32 bytes. El formato pide
  `(...)V` siempre.
- **`fields_count` sin contar el campo de captura.** Se escribía el campo pero el conteo salía del AST, que
  no lo tiene. `javap`: `unexpected end of file while reading MainActivity$1`; nuestro lector:
  `class … ends inside needs 720914 bytes at 727 of 881`. El conteo va de lo *emitido*, no de lo declarado.
- **El orden de las `class_defs`.** El dexer conserva el orden de entrada, así que la lista emitida se ordena
  por `interno + ".class"` — el mismo orden de rutas que usa `leeClases` — y por eso `MainActivity$1.class`
  (el `$` es 0x24) va **antes** que `MainActivity.class` (el punto es 0x2e) y la `R` después de sus anidadas.
  Con el orden crudo por nombre el dex medía 4 bytes más y no coincidía ni uno.

Quedan rechazadas con nombre, porque no están medidas: la anonima sobre una clase (`javac` la pone como
`super`, no como interface), una anonima dentro de otra, una anonima creada en un método `static` (no hay
`this` que capturar) o en un inicializador de campo (no hay `EnclosingMethod` que anotar), el `if`, los tipos
de dos huecos y las anotaciones que no sean `@Override`.

## 43. `apkc compile`: el `.class` sin `javac`, medido en el binario y dentro del hospedador con forma de navegador

El emisor de los párrafos 41 y 42 era función de librería; la página necesita un comando. `apkc/compilar.go`
añade `apkc compile --android-jar android.jar [-o build/classes] [carpeta-con-java ...]`, que lee los `.java`
del árbol, resuelve los nombres contra el jar y escribe una clase por fichero con el nombre internal — el
mismo sitio y el mismo nombre de donde los lee `apkc dex`. `compile` entra en la lista de comandos que **no**
sondean el SDK (`case "dex", "link", "compile":`), porque dentro de la pestaña no hay SDK que encontrar.

**Medición nativa**, sobre el proyecto real que genera la página (`studio-e2e-java/com.hola.apkcstudio`):

```sh
$G compile --android-jar $HOME/Android/platforms/android-34/android.jar -o $T/compilado $S/src
#   [compile] 2 fuentes -> 6 clases en .../compilado   finished in 33.588615ms
$G dex -o $T/dex-go.dex   $T/compilado            # 6 classes -> 2392 bytes
$G dex -o $T/dex-javac.dex $S/build/classes       # 6 classes -> 2392 bytes
cmp $T/dex-go.dex $T/dex-javac.dex                # identical
```

Los seis ficheros miden 1034, 881, 312, 318, 320 y 362 bytes frente a los 1036, 900, 331, 337, 339 y 390 de
`javac`; `javap -c -p` sobre ellos imprime **las mismas líneas**, y las únicas diferencias del `diff` en las
seis clases son los **números de pool** (`invokespecial #10` frente a `#1`), que no son un hecho semántico. Y
el dex que sale de los unos es **idéntico byte a byte** al que sale de los otros: 2392 = 2392, `cmp` mudo.

**Medición dentro del hospedador** (`shim/navegador-sim.js`, un `fs` en memoria, sin red y sin proceso), con
el jar montado como un fichero más:

```sh
sh -c 'cd shim && node navegador-sim.js \
  --montar $HOME/Android/platforms/android-34/android.jar:/sdk/android.jar \
  --montar $S/src/com/hola/apkcstudio/MainActivity.java:/proyecto/src/com/hola/apkcstudio/MainActivity.java \
  --montar $S/src/com/hola/apkcstudio/R.java:/proyecto/src/com/hola/apkcstudio/R.java \
  --exportar /proyecto/build/classes/com/hola/apkcstudio/MainActivity.class:$T/wasm-classes/.../MainActivity.class \
  ... (seis exportaciones) ... \
  $PREFIX/tmp/apkc-wasm.wasm compile --android-jar /sdk/android.jar -o /proyecto/build/classes /proyecto/src'
```

**Resultado**: `[compile] 2 fuentes -> 6 clases`, **1,16 s** (leer los 26361808 bytes del jar por el `fs`
simulado es lo que tarda), y las seis clases exportadas son **idénticas byte a byte a las que produce el
binario nativo**; dexadas desde fuera, dan los mismos 2392 bytes que las clases de `javac`. O sea: el paso
Java completo corre dentro de la forma de navegador, con el mismo resultado.

La suite, en ese hospedador: **176 PASS, 0 FAIL, 9 SKIP**, y los **doce** tests de Java (`TestLexico*`,
`TestParsear*`, `TestEmisor*`, `TestCompile*`) **pasan dentro** en vez de saltarse, con el jar montado en
`/sdk/android.jar`; el más caro, `TestCompileEscribeLasClasesDelProyecto`, tarda 2,88 s allí dentro. Nativa:
**184 tests**. Tres tests nuevos por el comando: el camino de disco entero (`fuentesJava` →
`compilarProyectoJava` → `escribirClases` → `leeClases` → `dexDeClases` igual byte a byte que javac), el
rechazo que no escribe nada, y el recorrido que ignora los `.kt` y los `.txt` y nombra una raíz que no existe.

**Otra trampa del hospedador, corregida en el propio comando**: `fuentesJava` averiguaba si la raíz era
fichero o árbol con `os.Stat`, y en el `fs` simulado eso falla para un fichero montado; ahora intenta el
`ReadFile` cuando el nombre acaba en `.java` y si no, camina el árbol, que es lo que funcionaba en el §39.

**Una trampa del hospedador, anotada porque costó un salto de prueba**: dentro del `fs` simulado
`os.Stat("/sdk/android.jar")` responde que no existe, mientras que abrirlo funciona (así lo usó el §39). El
test localizaba el jar con `Stat`, así que se saltaba aunque el jar estuviera montado; ahora lo **abre**
(`abrirBuscador`) y guarda el resultado en un `sync.Once`, que además evita releer los 26 MB en cada prueba.

## 44. `apkc build --linker go --compiler go`: un apk completo sin levantar ningún proceso, también en el hospedador con forma de navegador

Con `compile` y `dex` ya medidos, el cableado dentro de `build` son dos ramas y tres banderas. `--compiler
go` sustituye las dos etapas de código (`compileJava` por `compilarFuentesEnGo`, `bundleJava` por
`dexarClasesEnGo`) y salta `closeApiGap`, que es un remiendo de javac: el emisor lee todos los fuentes del
proyecto juntos, así que no tiene punto ciego. Como dentro de la pestaña no hay SDK que sondear, `build`
acepta ahora `--android-jar`, `--sdk-version` y `--codename` (las mismas tres razones del §39) y `main()`
omite el sondeo cuando la línea de comando pide `--linker go` **y** `--compiler go` (`sinProcesos`).

**El portero** (`revisarCompilerGo`, aislado del `LogF` para poder probarlo) rechaza con nombre lo que el
camino todo-en-Go no mide: fuentes `.kt` en `src`, jars en `jar/` (esas los dexa `d8`) y `--aab`. Cuatro
subpruebas, incluida la de un proyecto que sí está medido y no debe rechazar nada.

**Medición nativa**, en frío (`rm -rf build`) sobre el proyecto que genera la página:

```sh
cd $S && apkc build --linker go --compiler go \
  --android-jar $HOME/Android/platforms/android-34/android.jar --sdk-version 34 --codename 14 \
  --keystore $HOME/.android/debug.keystore
#  [build] linking resources with the Go linker   [aapt2 link] 242ms
#  [build] 2 fuentes de Java en Go -> 6 clases
#  [build] 6 clases -> 2392 bytes en build/classes.dex
#  [build] packaging and signing inside apkc      [timing] total 255ms
```

Sale `build/app.apk` de **9868 bytes**, y los oráculos de siempre lo aprueban: `apksigner verify --verbose`
→ v1 `true`, v2 `true`; `zipalign -c -p 4` → `rc 0`; `aapt2 dump badging` → `package name=com.hola.apkcstudio
versionCode='1' platformBuildVersionName='14' platformBuildVersionCode='34' compileSdkVersion='34'` y
`minSdkVersion:'21'`; `aapt2 dump resources` → `type id id=01 entryCount=1, resource 0x7f010000 id/pulsar`;
`zipcheck.py` → 7 entradas, cada registro de acuerdo consigo mismo. Frente a los 10194 bytes del build con
`javac`+`d8`, la diferencia es la que ya midió el §23: el dex de `d8` trae `debug_info` y anotaciones (2844
frente a nuestros 2392).

**Medición dentro del hospedador**, una sola orden y sin proceso (el proyecto se monta en la raíz porque
`build` trabaja en el directorio de proceso, y el jar y el keystore como ficheros más):

```sh
sh -c 'cd shim && node navegador-sim.js \
  --montar $S/AndroidManifest.xml:/AndroidManifest.xml \
  --montar $S/res/values/styles.xml:/res/values/styles.xml \
  --montar $S/res/layout/main.xml:/res/layout/main.xml \
  --montar $S/src/com/hola/apkcstudio/MainActivity.java:/src/com/hola/apkcstudio/MainActivity.java \
  --montar $HOME/Android/platforms/android-34/android.jar:/android.jar \
  --montar $HOME/.android/debug.keystore:/keystore.p12 \
  --exportar /build/app.apk:$O/app.apk \
  $PREFIX/tmp/apkc-wasm.wasm build --linker go --compiler go --android-jar /android.jar \
    --sdk-version 34 --codename 14 --keystore /keystore.p12'
```

**Resultado**: `app.apk` de **9868 bytes**, las mismas siete entradas que el apk nativo y **los mismos bytes
en cada una** (comprobado extrayendo los dos y `cmp` entrada a entrada: `AndroidManifest.xml`,
`META-INF/ANDROID.RSA`, `META-INF/ANDROID.SF`, `META-INF/MANIFEST.MF`, `classes.dex`,
`res/layout/main.xml`, `resources.arsc`), verificado por `apksigner` (v1 y v2) y `zipalign` también el
fichero del hospedador.

**Y una nota sobre qué NO claimar aquí**: el apk **no es reproducible byte a byte**, ni siquiera dos veces
seguidas en nativo — dos `build` separados por dos segundos difieren en 301 bytes, porque la fecha y hora
que el zip guarda de cada fichero entra en el cálculo de la firma. Por eso la comparación que se escribió
fue entrada por entrada; el `cmp` directo de los dos apks marca el byte 11, que es justo el campo de hora.
Si alguien quiere reproducibilidad hará falta fijar esos tiempos, y eso no está medido todavía.

Suite nativa: **190 tests**, cero fallos. Lo que esta sección todavía no medía era chromium: faltaba que
la página llamara a esa orden en vez de a `sign`. Ese cableado está hecho y medido en el §45.

## 45. La página compila dentro de chromium: el apk entero, y byte a byte igual que el build nativo

El cableado son tres ficheros de la página y nada más de apkc. En `studio/index.html` el bloque 2 gana un
`<details>` con el selector del jar (`#android-jar`), el del keystore (`#keystore-compilar`), la versión de
SDK y su nombre en clave (`#sdk-version`, `#codename`), un aviso de estado (`#estado-jar`) y dos botones:
`#usar-android-jar` guarda el jar en OPFS y `#compilar-apk` llama a `compilarDelTodo()`
(`studio/pestana.js`), que pone el cwd en `/proyecto`, corre `build --linker go --compiler go --android-jar
/sdk/android.jar --sdk-version … --codename … --keystore /sdk/keystore.p12` y, si sale 0, deja el apk en el
ancla `#descargar-apk-compilada`. `studio/servidor.js` sirve `/android-jar` desde la variable `ANDROID_JAR`:
el jar entra por el propio origen de la página, no por un CDN (regla 4).

**Aparecieron dos fallos, y los dos eran del hospedador de la pestaña, no de apkc.**

1. **Carrera de arranque.** Desde el segundo comando en adelante, `hospedador.ejecutar` devolvía `codigo 1`
   con la consola vacía. Se reprodujo con dos órdenes seguidas (`version`, `build`) en vez de con una: la
   página llamaba a `iniciar()` dos veces a la vez, cada una montaba su `fs` y el segundo `postMessage`
   pisaba el primero. `studio/hospedador.js` guarda ahora la promesa de arranque en `arranque` (un solo
   `hacerIniciar()` en vuelo; si falla, se borra para reintentar). Esto reinterpreta la nota de
   «intermitencia de OPFS» del §39: no era intermitencia, era esta carrera.
2. **Montar en un subdirectorio.** `/sdk/android.jar` escrito con `escribir` se leía perfecto desde JS y Go
   contestaba `open /sdk/android.jar: No such file or directory`; el mismo jar en la raíz (`/android.jar`)
   abría. `caminoAbierto` del fragmento de fs exige que **cada tramo** del camino exista como nodo de
   memoria, y `montar` solo creaba los directorios intermedios en OPFS. `studio/fs-fragmento.js` crea ahora
   también los nodos de directorio intermedios.

**Medición dentro de chromium** (conducida por CDP, con el binario `GOOS=js` y el jar real):

```sh
sh -c 'cd studio && APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm \
  APK_EJEMPLO=$PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/build/app.apk \
  KS_EJEMPLO=$HOME/.android/debug.keystore \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar node prueba-pestana.js'
```

- el jar entra por `/android-jar` (origen propio): **26361808 bytes** escritos en OPFS, y releídos por el fs
  de la pestaña dan **los mismos 26361808**, con cabecera `PK\3\4`;
- `compilarDelTodo()` deja `ultimo-codigo` en **`0`**, `finished in 5.08793472s` (`[timing] total 5.022s`,
  de los que `961ms` son empaquetar y firmar) y el apk en `/proyecto/build/app.apk`: **9141 bytes**;
- las **21** comprobaciones de la prueba de pestaña pasan, cero fallos, y las **12 peticiones** registradas
  son todas del propio origen: cero peticiones ajenas. (La prueba creció a **32 comprobaciones** y **18
  peticiones** con las cuatro secciones del §46, y sigue sin salir de su origen.)

**Los oráculos de fuera, sobre el fichero que salió de la pestaña** (`studio/e2e.sh` lo pide solo, bloque 4b):

```sh
$BUILD_TOOLS/apksigner verify --verbose --min-sdk-version 21 $PREFIX/tmp/pestana-apk/pestanera-completa.apk
#  Verified using v1 scheme (JAR signing): true
#  Verified using v2 scheme (APK Signature Scheme v2): true
$BUILD_TOOLS/zipalign -c -p 4 …pestanera-completa.apk           # rc 0
python3 ~/Develop/Go/apkc-harness/zipcheck.py …pestanera-completa.apk
#  7 entries, every record agrees with itself; 1 other stored entry at 4
$BUILD_TOOLS/aapt2 dump badging …pestanera-completa.apk
#  package: name='com.myapp' versionCode='1' versionName='0.1' platformBuildVersionName='14'
#  platformBuildVersionCode='34' compileSdkVersion='34' … minSdkVersion:'21' targetSdkVersion:'33'
```

**Y la comparación que cierra el objetivo**: el mismo proyecto, compilado con el binario nativo
(`apkc create proyecto`, el mismo `src/com/myapp/Editado.java` que escribe la prueba, y el mismo
`build --linker go --compiler go`) da **exactamente 9141 bytes** y **los mismos bytes en cada entrada**.
`unzip -p … | sha256sum` de las cinco entradas que importan, pestaña frente a nativo:

| entrada | sha256 (16 primeros), pestaña = nativo | bytes |
| --- | --- | --- |
| `AndroidManifest.xml` | `ed604df837035a84` | 1908 |
| `resources.arsc` | `b38acea1833e0226` | 788 |
| `res/layout/main.xml` | `0cc057bb56f82857` | 604 |
| `classes.dex` | `907c811250803c58` | 1396 |
| `META-INF/MANIFEST.MF` | `96485e1eb5fc0a7f` | 376 |

Es decir: **el apk que produce el navegador sin levantar un solo proceso no se parece al nativo, es igual**,
entrada por entrada; solo difiere el tiempo del zip (lo del §44, que sigue sin claimar reproducibilidad). El
`classes.dex` es de 1396 bytes y no los 2392 del proyecto del studio porque aquí se compilan dos fuentes
distintos: la plantilla de `apkc create` (`com.myapp`, sin clase anónima) más el `Editado` de la prueba.

Con esto el objetivo del repo está cubierto del lado del cliente: `create`, editar, compilar recursos,
compilar Java, dexar, empaquetar y firmar, todo dentro de la pestaña y sin conexión. Lo que sigue siendo
frontera, y no falla silenciosamente: el emisor de `.class` rechaza por nombre lo que no mide (§41, §42) y
el enlazador rechaza los calificadores de carpeta y los recursos que no sean `values`/`layout` (§38).

## 46. También el proyecto que la página *genera*: la clase anónima, el `@+id` y el `style`, dentro de chromium

El §45 compiló la plantilla de `apkc create`, que es el camino fácil: dos fuentes, `MainActivity` sin clase
anónima y una `R` con dos tipos. Lo que la página **escribe por su cuenta** cuando se activa el botón es el
proyecto del §42: `onCreateOptionsMenu` con `new MenuItem.OnMenuItemClickListener() { ... }`,
`Toast.makeText(MainActivity.this, …)`, `@+id/pulsar` en el layout y `@style/AppTheme` en el manifiesto. Ese
es el que estaba medido con `javap` y con `d8` **en node y en el hospedador con forma de navegador**, pero no
en chromium. Se cableó como comprobación 8 de `studio/prueba-pestana.js`: la propia página llama a
`buildProject(estado)` (el mismo generador del zip), escribe los cinco ficheros en OPFS bajo `/estudio` y
corre el build sin procesos.

```sh
sh -c 'cd studio && APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm \
  APK_EJEMPLO=$PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/build/app.apk \
  KS_EJEMPLO=$HOME/.android/debug.keystore \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar node prueba-pestana.js'
```

**Dentro de chromium**: cinco ficheros escritos, `codigo 0`, y en la consola de la pestaña las dos líneas que
se esperaban — `[build] 2 fuentes de Java en Go -> 6 clases` y `[build] 6 clases -> 2392 bytes en
build/classes.dex`, **los mismos 2392 bytes que midieron el §42 y el §44** — con `[timing] package and sign
go 788ms`, `total 4.066s` y `finished in 4.067974912s` (en la corrida del gate fueron `546ms` y `3.875s`:
el tiempo cambia, los bytes no). El apk sale de **9832 bytes**. La prueba de pestaña queda en **32
comprobaciones** (llegó a **37** con el verificador del §47), cero fallos, con **18 peticiones** y ninguna
fuera de su origen.

**Y el jar entra sin red, por el camino del usuario (comprobación 9)**: se borra `/sdk/android.jar` (la
página pasa a decir «sin android.jar en esta pestaña todavía»), se pone el fichero real en el
`<input type="file">` con `DOM.setFileInputFiles` de CDP, se pulsa `#usar-android-jar` y el aviso queda en
`hay un android.jar de 26361808 bytes en esta pestaña (/sdk/android.jar)` con **cero peticiones** en ese
tramo; después `compilarDelTodo()` vuelve a dar `codigo 0` y el mismo apk de 9141 bytes del §45. Hasta aquí
el «ni siquiera sin conexión» solo estaba probado por el `fetch` a `/android-jar` del servidor de pruebas;
así lo está por el selector de fichero, que es lo que hace alguien con el avión puesto.

**Y se elige una vez, no una por sesión (comprobación 9b)**: recargada la página con `Page.navigate`, el
aviso sigue diciendo `hay un android.jar de 26361808 bytes en esta pestaña (/sdk/android.jar)`, `hospedador`
lee los 26361808 bytes de OPFS y en todo el tramo de recarga no aparece ni una petición a `/android-jar`
(6 peticiones, todas de assets de la propia página); el build vuelve a salir `codigo 0` con los mismos 9141
bytes del §45.

**Frente al nativo, mismo proyecto mismo camino** (`buildProject` escrito a disco con node y
`apkc build --linker go --compiler go`, 733 ms): también **9832 bytes**, y `unzip -p | sha256sum` entrada por
entrada:

| entrada | sha256 (16 primeros), pestaña = nativo | bytes |
| --- | --- | --- |
| `AndroidManifest.xml` | `8d54e26aa55a9833` | 1932 |
| `classes.dex` | `26da19adb481c95b` | 2392 |
| `resources.arsc` | `f1a468123da12d9e` | 936 |
| `res/layout/main.xml` | `c2a0f09d5610287d` | 784 |
| `META-INF/ANDROID.SF` | `305475898b29d5de` | 500 |
| `META-INF/MANIFEST.MF` | `a469730c65d2a806` | 376 |

Los oráculos de fuera lo aprueban: `apksigner verify --verbose --min-sdk-version 21` → v1 `true` y v2 `true`
(1 firmante); `zipalign -c -p 4` → `rc 0`; `zipcheck.py` → 7 entradas coherentes; `aapt2 dump badging` →
`package: name='com.hola.apkcstudio' versionCode='1' versionName='0.1' platformBuildVersionName='14'
platformBuildVersionCode='34'` con `minSdkVersion:'21'` y `targetSdkVersion:'33'`.

**Y un oráculo que no pasa por nuestro código**: el `classes.dex` que escribió el navegador lo vuelve a leer
`d8` (R8, la herramienta de la cadena oficial) con `rc 0`:

```sh
unzip -qo $PREFIX/tmp/pestana-apk/pestanera-estudio.apk classes.dex -d $PREFIX/tmp/e46-d8
$BUILD_TOOLS/d8 --release --lib $HOME/Android/platforms/android-34/android.jar --min-api 21 \
  --output $PREFIX/tmp/e46-d8 $PREFIX/tmp/e46-d8/classes.dex
#  rc 0, 2076 bytes (en `--release` recorta lo que nadie usa, por eso baja de 2392)
```

Con esto `studio/e2e.sh` pide tres cosas más (bloque 4c): que `d8` lea el dex del navegador, que las seis
entradas comparadas coincidan con el nativo y que el apk del proyecto generado pase `apksigner`, `zipalign`,
`zipcheck` y `aapt2`.

**Señales de que el sistema tendría con qué arrancar, sin teléfono de por medio**: `aapt2 dump badging` del
apk de la pestaña declara el componente lanzable — `launchable-activity: name='com.hola.apkcstudio.MainActivity'
label='Hola apkc'` — y en el `classes.dex` que escribió el navegador están las dos clases que ese componente
necesita, `Lcom/hola/apkcstudio/MainActivity;` y su anónima `Lcom/hola/apkcstudio/MainActivity$1;`, más el
interfaz del framework al que se enlaza la llamada (`Landroid/view/MenuItem$OnMenuItemClickListener;`), que
es justamente `invokeinterface` del §42. Comprobado con `unzip -p … classes.dex | strings`.

**Lo que sigue sin medirse es el hecho 2 del objetivo**: instalar y ver arrancar la
actividad, que necesita un teléfono y no depende de ningún código.

## 47. `apkc verify`: la pestaña dictamina su propio apk, cotejado con `apksigner` caso por caso

Cierra el punto **0.2.2** del plan y es lo único que le faltaba a la página para no tener que pedirle a
nadie que le diga si el apk que ella escribió está bien. `apkc/verificar.go` vuelve a leer el paquete desde
el `fs` y comprueba por su cuenta las dos firmas que esta herramienta escribe: recalcula el digest de cada
entrada contra lo que dice `META-INF/MANIFEST.MF`, digiere el manifiesto sección a sección contra el `.SF`,
verifica el PKCS#7 sobre los bytes exactos del `.SF`, y localiza el bloque de firmas antes del directorio
central para recalcular el digest de las tres secciones — con el campo de desplazamiento del registro final
sustituido por el desplazamiento del bloque, que es la trampa del formato que ya describía el §44 — y
comprobar la RSA sobre los datos firmados con la clave del certificado emparejado.

Lo que no está medido se rechaza con nombre: `DSA`/`EC`, un digest que no sea SHA-256, varios firmantes, o
una clave pública que no sea la del certificado. **Y la cadena de confianza no se valida**: un certificado
de debug es auto-firmado y no hay ancla, así que se informa de él (asunto, serie, caducidad) sin afirmar que
sea digno de confianza, igual que hace `apksigner` cuando dice «Verifies, but not trusted».

**Dos cosas del formato las decidió el oráculo, no yo** — y las dos me hacían marcar mal un apk ajeno:

- el **bloque de firmas no entra en el digest del contenido**: el digest cubre las entradas, el directorio
  central y el registro final con el campo sustituido. Medido: un byte cambiado dentro del par de
  *rellenado* (los ceros que alinean a página) deja `apksigner` en `true/true` y `apkc verify` en `ok/ok`; un
  byte cambiado dentro del **valor del par de la v2** da `DOES NOT VERIFY` en `apksigner` y «la firma RSA
  sobre los datos-firmados no verifica» aquí. La diferencia entre los dos puntos es exactamente esa.
- **un par con id desconocido se ignora y se dice**: añadido un par `0x11223344` al bloque, `apksigner`
  sigue diciendo `v1 true, v2 true` y `apkc verify` dice `ok/ok` más la nota `par desconocido en el bloque de
  firmas: 0x11223344`. Herejarlo como fallo habría hecho que esta orden aprobara menos que el oráculo.

**Cotejo con `apksigner`** — el mismo fichero, los dos verificadores, el mismo código de salida. Las tres
primeras filas y la última las pide `studio/e2e.sh` (bloque 5); las demás se midieron a mano con el mismo
binario:

| caso | `apksigner verify --verbose` | `apkc verify` | salida |
| --- | --- | --- | --- |
| apk compilado y firmado en chromium (§46) | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` | `0` y `0` |
| apk firmado por `apksigner` (v1+v2, sin v3) | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` | `0` y `0` |
| apk firmado por `apksigner` con v1+v2+v3 | v1 `true`, v2 `true`, v3 `true` | v1 `ok`, v2 `ok` + nota «la firma v3 no se verificó aquí» | `0` y `0` |
| byte cambiado en el valor del par de la v2 | `DOES NOT VERIFY` | v1 `ok` (las entradas no se tocaron), v2 mal: «la firma RSA sobre los datos-firmados no verifica» | `1` y `1` |
| byte cambiado en `resources.arsc`, que viaja almacenado | `DOES NOT VERIFY` | v1 mal: «no se puede leer resources.arsc: zip: checksum error», v2 mal: «el digest guardado no es el de este contenido» | `1` y `1` |
| byte cambiado en el par de *rellenado* (los ceros que alinean a página) | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` | `0` y `0` |
| par desconocido `0x11223344` añadido al bloque | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` + nota «par desconocido en el bloque de firmas: 0x11223344» | `0` y `0` |
| paquete sin ninguna firma (fuera los tres `META-INF`) | `DOES NOT VERIFY` · «Missing META-INF/MANIFEST.MF» | v1 mal: «faltan piezas de la firma v1 dentro del paquete», v2 mal: «no hay bloque de firmas antes del directorio central» | `1` y `1` |

Nótese la segunda fila: **el verificador lee una firma que no escribió él**, que es la única prueba de que
no está confirmando sus propios supuestos. Y nótese la de `resources.arsc`: ahí la detención no es nuestra,
es el CRC-32 que el zip lleva por entrada, así que un byte torcido lo detectan tres comprobaciones
independientes —el CRC del archivo, el digest del manifiesto y el digest del bloque.

**Coste, medido en los tres hospedadores**: nativo `finished in 1.064384ms` y `2.301ms` sobre el apk de 9832
bytes del §46; dentro del hospedador con forma de navegador (`navegador-sim.js`, mismo `apkc.wasm`)
`finished in 402.866176ms`; dentro de chromium `finished in 51.294976ms` y `57.554944ms`. Es un orden de
magnitud más rápido que el `build` completo: verificar no compila nada.

**En la página** el botón `#verificar-apk` corre `verify build/app.apk` y el dictamen se lee en
`#estado-verificacion`; `compilarDelTodo()` lo llama solo, al terminar un build bueno, para que nadie tenga
que pulsar dos veces. Las dos comprobaciones nuevas de `studio/prueba-pestana.js` (10 y 10b) valen por la
prueba de fuego: dentro de chromium, sobre el apk que la propia pestaña compiló, el veredicto es `v1 ok` y
`v2 ok`; y sobre una copia con **un byte cambiado dentro de `resources.arsc`** — escrito en OPFS como
`build/mal.apk`, con el byte localizado desde el header local de la entrada — el comando sale con `codigo 1`
y dice `v1 mal` y `v2 mal`. La prueba de pestaña queda en **37 comprobaciones**, cero fallos, y las
peticiones siguen siendo todas de su origen.

**Comandos**:

```sh
# nativo y con el oráculo al lado, sobre el apk que salió de chromium
$PREFIX/tmp/apkc-gate verify $PREFIX/tmp/pestana-apk/pestanera-estudio.apk
$BUILD_TOOLS/apksigner verify --verbose --min-sdk-version 21 $PREFIX/tmp/pestana-apk/pestanera-estudio.apk
# dentro del hospedador con forma de navegador
sh -c 'cd shim && node navegador-sim.js --montar $PREFIX/tmp/pestana-apk/pestanera-estudio.apk:/en.apk \
  $PREFIX/tmp/apkc-wasm.wasm verify /en.apk'
# la prueba de chromium, que además del veredicto bueno comprueba el caso alterado
sh -c 'cd studio && APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm APK_EJEMPLO=… KS_EJEMPLO=… \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar node prueba-pestana.js'
```

**Suite**: cinco tests nuevos en `apkc/verificar_test.go` (bueno, byte cambiado en una entrada almacenada,
sin firma dicho por esquemas, valor del par alterado, par desconocido ignorado y anotado). Nativa: **189
PASS, 1 SKIP, 0 fallos** en 15,4 s. Dentro del hospedador con forma de navegador, con el `android.jar`
montado en `/sdk/android.jar`: **181 PASS, 9 SKIP, 0 fallos**, y los cinco tests del verificador pasan
todos ahí (`TestVerificarApruebaLoQueEstaBienFirmado` en 0,16 s, el más rápido en 0,04 s), lo que quiere
decir que el dictamen corre en `js/wasm` igual que en nativo. La comparación contra `apksigner` vive en
`studio/e2e.sh` (bloque 5) para que un falso «ok» propio no pueda colarse en el gate: `bueno: apksigner si,
apkc verify si · con-v3: apksigner si, apkc verify si · bloque-alterado: apksigner no, apkc verify no`.

Con `verify` dentro del binario, la cadena completa —crear, compilar, dexar, empaquetar, firmar y
**dictaminar**— corre en la pestaña. Lo que sigue sin medirse es el hecho 2 del objetivo: instalar ese apk y
ver la actividad arrancar.


## 48. Lo más cerca de `pm install` que hay sin teléfono: la clase que el manifiesto promete está en el dex del navegador

Un apk puede firmar bien y aun así no arrancar si el componente que declara su manifiesto no existe dentro
del `classes.dex`. Eso es comprobable sin dispositivo, y `studio/e2e.sh` lo pide desde ahora, en los dos
apks que salen de chromium:

```sh
lanzable=$(sed -n "s/^launchable-activity: name='\([^']*\)'.*\$/\1/p" $TMP/studio-pestana-badging.log)
interno=$(printf '%s' "$lanzable" | tr '.' '/')           # com.myapp.MainActivity -> com/myapp/MainActivity
unzip -p $TMP/pestana-apk/pestanera-completa.apk classes.dex | strings | grep -q "L$interno;"
```

**Medido**: para el apk de la plantilla, `aapt2 dump badging` declara `launchable-activity: name='com.myapp.MainActivity'`
y esa clase está en el dex que escribió el navegador (el gate lo avisa: `el apk de la pestaña declara
com.myapp.MainActivity y esa clase está dentro de su dex`). Para el proyecto que la página genera (§46), el
gate exige tres nombres dentro del mismo `classes.dex` de 2392 bytes:

- `Lcom/hola/apkcstudio/MainActivity;` — la actividad,
- `Lcom/hola/apkcstudio/MainActivity$1;` — la clase anónima que emite el fuente, con su `this$0` del §42,
- `Landroid/view/MenuItem$OnMenuItemClickListener;` — la interfaz del framework contra la que va el
  `invokeinterface` del botón, es decir el enlace que ART tendría que resolver al cargarla.

Todo junto, la corrida del gate queda así (`sh e2e.sh`, `rc 0`): variante java 10348 bytes y kotlin 678823
bytes verificadas por `apksigner`/`zipcheck`; la pestaña con **37 comprobaciones**; el apk compilado dentro
de chromium de 9141 bytes aprobado por `apksigner` (v1+v2), `zipalign -c -p 4`, `zipcheck.py` y `aapt2 dump
badging`; el del proyecto generado de 9832 bytes **igual entrada por entrada al nativo** y su dex releído
por `d8` con `rc 0`; y el cotejo del verificador propio contra `apksigner` — `bueno: si/si`,
`con-v3: si/si`, `bloque-alterado: no/no`.

**Otra cosa que sí se puede comprobar por fuera, sin teléfono**: `aapt2 dump xmltree --file AndroidManifest.xml`
sobre el apk que escribió la pestaña imprime, dentro del `<activity>` de la línea 12,
`android:exported(0x01010010)=true` junto al `name=".MainActivity"`, y más abajo los dos
`android:name` del filtro (`android.intent.action.MAIN` y `android.intent.category.LAUNCHER`), con
`targetSdkVersion=33` en el `<uses-sdk>`. El atributo `exported` es el que la plataforma empezó a exigir en
las actividades con filtro de intención a partir de API 31; no lo medimos aquí instalando nada, pero su
ausencia habría sido un motivo conocido de rechazo en el `pm install`, y está. Que lo escriba nuestro
`axmlwriter` saliendo del texto del `studio/app.js` es mérito del enlazador del §37, que copia el atributo
como los demás.

**El límite de esta comprobación, dicho sin adornos**: `strings` sobre el dex prueba que el *tipo* existe y
que la *referencia* a la interfaz del framework está escrita. No prueba que el cargador de ART la resuelva
contra el `framework.jar` del aparato ni que `onCreate` sobreviva a la inflación del layout. Eso último es
exactamente el hecho 2 del objetivo y sigue necesitando un teléfono.
## 49. Escribir de verdad: campos, `static {}`, variables locales y aritmética, medidos contra `javap -v -c -l`

Hasta aquí la pestaña compila lo que **ella genera**. «Escribir o pegar el fuente de una app mínima»
pide algo más: un campo propio, una variable local con su cast, una condición. Esta sección mide qué
emite javac para eso y qué faltaba para emitirlo igual.

**El objetivo, leído de javac 21 con `--release 8`** (la versión 52 que escribe este emisor):

```sh
javac -nowarn --release 8 -cp $HOME/Android/platforms/android-34/android.jar -d cj com/myapp/R.java com/myapp/Campos.java
javap -J-Xmx256m -p -v -l -cp cj com.myapp.Campos
```

Con seis campos declarados así:

```java
private int oculto = 3;  protected int visible = 1;  int paquete = 2;
public int publico7 = 4;  static int shared = 5;  final int fijo = 6;
```

los flags son `0x0002`, `0x0004`, `0x0000`, `0x0001`, `0x0008` y `0x0010`, y **solo `fijo` lleva
`ConstantValue: int 6`** — un `final` de instancia con literal se dobla *y* se baja al `<init>`, que es
la parte que nadie adivina bien. El `<init>` medido:

```
 0: aload_0    1: invokespecial Activity.<init>:()V    4: aload_0  5: iconst_3  6: putfield oculto:I
 9: aload_0   10: iconst_1   11: putfield visible:I   14: … paquete  19: … publico7
24: aload_0   25: bipush 6   27: putfield fijo:I      30: return
LineNumberTable: line 8:0, 9:4, 10:9, 11:14, 12:19, 14:24     stack=2, locals=1
```

una fila de línea **por campo**, apuntando al `aload_0` de cada uno, y el `static int shared = 5` fuera
de ahí: va a un `<clinit>` (`flags 0x0008`, `()V`, `stack=1, locals=0`, una fila `line 13: 0`) que javac
escribe **después de todos los métodos declarados**. En el cuerpo, `View v = findViewById(...)` es
`invokevirtual; astore_2`; `Button b = (Button) v` añade `checkcast #43` antes del `astore_3`; `oculto =
oculto + fijo` es `aload_0; aload_0; getfield; bipush 6; iadd; putfield` — la constante `fijo` llega
**doblada**, no leída; y `shared = shared * 2` no lleva receptor: `getstatic; iconst_2; imul; putstatic`.
`contar(int)` sale `stack=2, locals=3` y su cuerpo es `iload_1; iconst_1; isub; istore_2; iload_2;
iconst_3; irem; istore_2; iload_2; ireturn`, idéntico en las dos compilaciones; `a*b`, `a/b`, `a%b` dan
`imul` `0x68`, `idiv` `0x6c`, `irem` `0x70`.

**Tres defectos reales salió a buscar esta medición**, y los tres estaban en código que ya pasaba sus
pruebas:

1. **`astore` estaba mapeado a la fila de `double`.** `guarda()` devolvía `0x47` para un objeto, que es
   `dstore_0`; el JVMS pone `astore_0` en `0x4b`. Nadie lo había visto porque **el subconjunto medido
   nunca guardaba una variable local de tipo referencia**: la plantilla y la `R` no tienen locales. Con
   `0x47` nuestro `View v = …` salía como `dstore_2`, que además cuenta la ranura de a dos.
2. **La cadena de padres del jar pedía un nombre internal y recibía un descriptor.**
   `classreader.go` guarda `super_class` como `Landroid/widget/TextView;` y `enLaCadena` pasaba ese valor
   tal cual a `claseDelJar`, que indexa por `android/widget/TextView`. Resultado: `b.setText(...)`
   respondía `android/widget/Button: Landroid/widget/TextView; no esta en el android.jar`. Se veía solo
   cuando el método **no** está declarado en la clase del receptor, y aquí sí lo estaba todo: el
   `setContentView` de la plantilla vive en `Activity`, un salto de cadena. Añadido
   `internoDeDescriptor` en los dos recorridos (métodos y campos).
3. **Un `final int` propio no se doblaba.** `buscarCampo` marcaba constante solo si era
   `estatico && final`, y javac dobla también el `final` de instancia con literal (medido: `bipush 6`,
   no `getfield fijo`).

**Estado de la comprobación**: con estos tres arreglos, `javap -p -c -l` sobre nuestra clase y sobre la de
javac es **idéntica línea por línea** (112 líneas, `diff` mudo), y `TestEmisorCamposContraJavap` fija los
flags de los seis campos, el orden `<init> / onCreate / contar / <clinit>`, las alturas de pila y ranuras
`(2,1) (3,4) (2,3) (1,0)` y las seis filas de la `LineNumberTable`. Suite nativa: verde.

**Y lo que todavía no vale, dicho sin adornos**: `apkc dex` traduce dieciséis opcodes medidos
(`jvmMeasured`) y se niega con nombre ante el resto — este fuente le trae `bipush`, `iconst_3`, `iadd`,
`imul`, `irem`, `astore_n`, `getstatic`/`putstatic` y `checkcast`. Mientras el traductor no crezca con su
propia medición contra `d8`, **el avance del emisor no llega al apk**: `build --compiler go` sobre un
proyecto con campos falla en el dexer, no en el compilador. La pestaña sigue compilando hoy lo que genera
la página, que es lo que los §44 a §48 demuestran. El siguiente incremento es ese: la tabla del dexer y su
bajada a instrucciones DEX, con el oráculo de siempre — `d8` sobre los `.class` de javac frente a los
nuestros, byte a byte.

## 50. El traductor DEX aprende las cuentas y los campos: un proyecto con variables propias ya llega al apk

El §49 cerraba diciendo que su avance no llegaba al apk porque `apkc dex` se negaba ante los opcodes nuevos.
Este párrafo los mide y los ingresa; al terminar, un proyecto con campos propios compila de principio a fin
sin `javac` ni `d8`.

**Cómo se midió**, dos lados del mismo puente:

- el **ancho** de cada opcode JVM es la diferencia de desplazamientos que imprime `javap -c` sobre las clases
  de javac (el método del §17, repetido sobre la sonda nueva: `bipush` ocupa dos bytes porque el siguiente
  `pc` está a +2, `astore_2` uno, `getstatic` tres);
- la **forma dalvik** sale de las palabras que `d8` escribió sobre esas mismas clases, leídas con nuestro
  propio lector (`apkc/dexvolcado_test.go`, con `APKC_DEX_FIXTURE`; sin la variable se omite: es un anteojo,
  no una comprobación).

| lo que hace el fuente | palabra de `d8` | lectura |
| --- | --- | --- |
| `a + b` (dos registros) | `0x21b0` | `add-int/2addr v1, v2`: escribe sobre el primer operando |
| `a - b` | `0x21b1` | `sub-int/2addr`, misma familia contigua (add `0xb0`, sub `0xb1`, mul `0xb2`, div `0xb3`, rem `0xb4`) |
| `a << b` | `0x21b8` | otro miembro de la misma tabla, verificado más lejos |
| `x = x + 7` | `0x01d8` + `0x0701` | `add-int/lit8 v1, v1, 7`: en la segunda unidad el nibble bajo es el registro fuente y el byte alto el literal |
| `a * 2`, `a / 4`, `a % 3` | `0x01da`, `0x01db`, `0x01dc` | la familia `/lit8` repite el orden add/mul/div/rem, y eso es lo que confirma la de `/2addr` |
| `hasta - 1` | `0x01d8` + `0xff01` | `d8` lo reescribe a `+ (-1)`: mismo opcode, literal `0xff` |
| `a + 300` | `0x01d0` + `0x012c` | `add-int/lit16`, literal de dieciseis bits en la segunda unidad |
| leer `shared` | `0x0060` + índice | `sget v0, field@N` (21c), **sin receptor** |
| escribir `shared` | `const/16 v0, 9` + `0x0067` + índice | `sput v0, field@N` |
| `private int oculto = 3` en el `<init>` | `0x1059` + índice | `iput v0, v1, field@N`: valor primero, objeto después |
| `(Button) v` | `0x011f` + `type@1` | `check-cast v1, type@N` |
| devolver un objeto | `0x0111` | `return-object v1` |

**Observación abierta, escrita tal cual**: para `a * b` con dos parámetros `d8` escribió `0x0192` +
`0x0201`, una forma de dos unidades que todavía no desciframos (`/2addr` habría sido `0x2192`). Nosotros
emitimos `mul-int/2addr` `0xb2`, la posición que le toca en la familia verificada en los otros cuatro
miembros; la prueba de que eso es legal no es nuestra opinión sino el hecho de que **`d8` lee el dex que
escribimos con `rc 0`** y `apksigner`, `zipalign` y `aapt2` lo aprueban abajo.

**Tres defectos del propio traductor salieron al pasar fuentes nuevas**, y los tres estaban en código con
sus pruebas en verde:

1. **fuga de temporales en los `put`**: `accesoCampo` sacaba valor y objeto de la pila simulada y nunca los
   devolvía a la bolsa, así que un `<init>` con seis campos inicializados pedía seis temporales y reventaba
   contra la pila máxima que midió javap (`iconst_2: it needs more temporaries than the max stack`).
2. **`getstatic` vaciaba la pila**: la ruta de lectura llamaba a `pop` antes de mirar si era lectura, así
   que el primer `getstatic` de un método sin nada en la pila respondía `the JVM stack is empty`.
3. **la base de la primera pasada se quedó corta**: al atar un temporal a un local (`View v = findViewById(…)`)
   ese registro deja de ser reutilizable, y con `base = max_stack` el cuarto temporal chocaba contra el
   borde. La primera pasada usa ahora `max_stack + locales + 1`; la segunda sigue partiendo de lo que gastó
   la primera, así que **las palabras del proyecto del studio no cambian** y su comparación contra `d8`
   (§21, §23) sigue en verde.

**End-to-end**, un proyecto que la página no genera hoy —`private int pulsos = 0`, `static int total = 5`,
`Button boton = (Button) findViewById(R.id.pulsar)`, `pulsos = pulsos + 1`, `total = total * 2`,
`boton.setText("pulsar")`— con `apkc build --linker go --compiler go`:

```
[build] 2 fuentes de Java en Go -> 5 clases
[build] 5 clases -> 1748 bytes en build/classes.dex
[timing] total 834ms        → build/app.apk de 9555 bytes
```

y los oráculos, todos de fuera: `apksigner verify --verbose` → v1 `true` y v2 `true`; `zipalign -c -p 4` →
`rc 0`; `zipcheck.py` → 7 entradas coherentes; `aapt2 dump badging` → `package: name='com.myapp'` y
`launchable-activity: name='com.myapp.MainActivity'`; `d8 --release` sobre el `classes.dex` que escribimos
→ `rc 0`; `apkc verify` sobre el apk → v1 `ok` y v2 `ok`. El mismo proyecto con la cadena de siempre
(`apkc build`, o sea `javac` + `d8`) tarda **10,095 s** y da 9995 bytes con un dex de 2136: los 388 bytes
de diferencia en el dex son `debug_info` y anotaciones, el hueco ya escrito del §23, no una diferencia de
semántica.

**Y la comprobación que cierra el círculo del §41**: sobre la muestra de campos, `apkc dex` de las clases de
javac y `apkc dex` de las nuestras dan **el mismo fichero byte a byte, 1960 bytes** (`cmp` mudo). Suite
nativa: **190 PASS, 2 SKIP, 0 fallos**; en el hospedador con forma de navegador, **182 PASS, 10 SKIP, 0
fallos** (el salto de uno en cada lado es `TestEmisorCamposContraJavap`, que pasa también en `js/wasm`, y
el anteojo del volcado, que se omite sin su variable).

**Lo que sigue rechazado por nombre**, para que nadie crea que esto es un compilador de Java completo: el
`if` (los anchos de las branquias y sus formatos dalvik todavía no están medidos), la concatenación de
cadenas (que en la versión 52 es una cadena de `StringBuilder.append`), arreglos, bucles, `long`/`float`,
anotaciones que no sean `@Override` y cualquier miembro cuyo `private`/`protected` no hayamos leído en un
`javap`.

## 51. El fuente se escribe en la página: campos, cadena de llamadas y cuentas, compilados y verificados en chromium

Lo que prometía el objetivo con las palabras «escribir o pegar el fuente» no era recompilar la plantilla,
sino que alguien cambiara el `.java` desde el editor y siguiera teniendo apk. Con los dos párrafos anteriores
eso ya se puede comprobar dentro del navegador, y `studio/prueba-pestana.js` lo hace (comprobación 11, que
sube la prueba a **39**): se sobrescribe `/proyecto/src/com/myapp/MainActivity.java` con un fuente que la
página no genera,

```java
public class MainActivity extends Activity {
    private int pulsos = 0;
    static int total = 5;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        View raiz = getWindow().getDecorView();
        pulsos = pulsos + 1;
        total = total * 2;
        raiz.setVisibility(0);
        Toast.makeText(this, "cuenta", Toast.LENGTH_SHORT).show();
    }
}
```

se pulsa el mismo camino del botón (`compilarDelTodo()`, o sea `build --linker go --compiler go` con el jar
de OPFS) y sale **un apk de 9459 bytes con `ultimo-codigo` 0**; la pestaña lo verifica sola (`apkc verify` →
v1 `ok`, 4 entradas, y v2 `ok`, 7 entradas) y ninguna de las **18** peticiones sale de su origen.

**Lo mismo, medido fuera de la pestaña sobre ese mismo fichero** (`studio/e2e.sh`, bloque 4d): `apksigner
verify --verbose --min-sdk-version 21` → v1 `true` y v2 `true`; `zipalign -c -p 4` → `rc 0`; `zipcheck.py` →
las entradas coherentes; `aapt2 dump badging` → `launchable-activity: name='com.myapp.MainActivity'`, y en
el `classes.dex` que escribió el navegador están la clase, los dos nombres de campo (`pulsos`, `total`) y
el método del framework al que enlaza la cadena de llamadas (`setVisibility`).

**Y la igualdad con la cadena de siempre, otra vez byte a byte**: el mismo fuente, compilado con `javac
--release 8` y con nuestro emisor, da `javap -p -c -l` **idéntico línea por línea**; `apkc dex` sobre las
clases de javac y sobre las nuestras produce **el mismo fichero de 1656 bytes** (`cmp` mudo), y `d8` lee ese
dex con `rc 0`. En nativo, `apkc compile` tardó 42 ms sobre esos dos fuentes.

Es el cierre del §44 al §50 en una frase: **la página compila un fuente que alguien escribió, y el
resultado es el mismo que daría `javac` + `d8` hasta donde este subconjunto llega**. Lo del teléfono sigue
siendo el hecho 2 del objetivo: instalar y ver la ventana.

## 52. El `if`, medido dos veces: la inversión de javac en el `.class` y las tres formas de rama en el dex

El `if` era la pieza que el §50 dejaba fuera: un fuente con condicionales no se podía escribir en la página.
Se midió en dos tramos, con dos oráculos que no comparten código con este fork —`javap` sobre las clases de
javac 21 y las palabras de `d8`— y con una sonda de doce métodos, en `$PREFIX/tmp/e52/p`:

```java
public int igual(int a, int b) { if (a == b) { return 1; } return 0; }
public int mayorcero(int a) { if (a > 0) { return 2; } return 3; }
public int nulo(Object o) { if (o == null) { return 4; } return 5; }
public int elseCorto(int a) { if (a == 0) { return 6; } else { return 7; } }
public int conElse(int a) { if (a == 0) { shared = 1; } else { shared = 2; } return shared; }
public int plegable(int a) { if (a == 0) { a = 1; } else { a = 2; } return a; }
public int anidadaConElse(int a, int b) { if (a == b) { if (a > 0) { shared = 1; } else { shared = 2; } } else { shared = 3; } return shared; }
public int tresNiveles(int a, int b) { if (a == b) { if (a > 0) { if (b > 0) { return 1; } } } return 2; }
```

**Primera medición — el `.class`.** `javac --release 8` y `apkc compile --android-jar android-34/android.jar`
sobre el mismo fuente, y `javap -p -c -l` a los dos lados: **idéntico línea por línea**, 186 líneas para
`Branquias` (doce métodos con `if`, tres de ellos anidados) y 81 para `Saltos`. Lo que se leyó:

- **javac emite la condición invertida**, y la tabla salió de esas 186 líneas, no del manual:
  `==`→`if_icmpne`, `!=`→`if_icmpeq`, `<`→`if_icmpge`, `>=`→`if_icmplt`, `>`→`if_icmple`, `<=`→`if_icmpgt`;
  contra cero, con una sola palabra en la pila simulada: `==0`→`ifne`, `!=0`→`ifeq`, `<0`→`ifge`,
  `>=0`→`iflt`, `>0`→`ifle`, `<=0`→`ifgt`; contra null: `==null`→`ifnonnull`, `!=null`→`ifnull`.
  `0 == a` no se dobla: javac escribe el literal primero (`iconst_0; iload_1; if_icmpne`).
- **el `goto` solo aparece si el `then` no sale por su cuenta**: `if (a == b) { return 1; } return 0;` no
  lleva ninguno, y `conElse` lleva `8: goto 15`. Un `if` sin `else` cuyo cuerpo sí cae al final tampoco lo
  lleva: el `else` vacío y el final son la misma posición.
- **el desplazamiento se cuenta desde el opcode**, no desde el operando: `2: if_icmpne 7` tiene por operando
  `00 05`. Todas las branquias miden tres bytes, así que el ancho de la tabla de caminantes (`jvmMeasured`)
  es 2 operandos para las quince formas nuevas.
- **la `LineNumberTable` de un método escrito en una línea tiene una sola fila** (`line 12: 0` en
  `anidadaConElse`), aunque el cuerpo tenga cuatro saltos.

**Segunda medición — las palabras de `d8`.** Tres formas de rama nuevas (`22t`, `21t` y `10t`), leídas de sus
ficheros
(`--release --min-api 21 --lib android.jar`) sobre las clases de javac, y verificados contra los nuestros:

| qué | d8 | el nuestro |
| --- | --- | --- |
| `if (a == b)` → `if-ne v1, v2, +4` | `2133 0004` | `4333 0004` |
| `if (a > 0)` → `if-lez v1, +4` | `013d 0004` | `033d 0004` |
| `if (o == null)` → `if-nez v1, +4` | `0139 0004` | `0339 0004` |
| `if (o != null)` → `if-eqz v1, +5` | `0138 0005` | `0338 0005` |
| `goto` → `+2` | `0228` | `0228` |

El `22t` (dos registros) reparte los registros en nibbles igual que el `22c` y lleva el desplazamiento con
signo en la segunda unidad; el `21t` (contra cero y contra null) usa un byte entero para el registro y el
segundo para el desplazamiento; el `10t` es una sola unidad con ocho bits con signo en la mitad alta. El
desplazamiento va **en unidades de código y desde la propia instrucción**, igual que en el JVMS pero con la
unidad de dos bytes. El null es cero para dalvik, así que `ifnull`/`ifnonnull` bajan a `if-eqz`/`if-nez`.

**Dos defectos reales salieron a buscar, y los dos estaban en código que ya pasaba sus tests.**

1. El emisor escribía `13: goto 20` donde javac escribe `13: goto 27`. En la posición 20 no había ninguna
   instrucción antes de otro `goto`: era una etiqueta vacia. javac encadena el salto por encima, y el
   emisor ahora hace lo mismo (`encadenaGotos`, pasada final antes de cerrar el atributo `Code`). Con
   esto el `javap` de los tres métodos anidados sale idéntico; sin esto se habría colado un dex que no es
   el que escribiría javac, que es justo lo que el §49 prometía no volver a hacer.
2. `desmontaPalabras` —el lector con el que se comparan nuestras palabras y las de `d8`— no conocía ni las
   branquias ni `sget`, `sput`, `iput`, `check-cast`, `return-object` (nueve opcodes que el §50 ya escribía)
   ni la familia `/lit8` con su `add-int/lit16` (seis que escribía `d8` y había que leer para comparar). Ahora
   conoce veintinueve mas, repartidos en cuatro casos nuevos de formato (`10t`, `21t`, `22t` y el `22s`).
   Y el `22s` **no** es un `22c`: la palabra de d8 `00d8 0100` es
   `add-int/lit8 v0, v0, 1` —registro de ocho bits en el byte alto de la primera unidad, literal con signo
   en el byte alto de la segunda y `vBB` en el nibble bajo—. La primera versión lo leía como el `22c` e
   imprimía `add-int/lit8 v, v, 256`, un número que ningún literal de ocho bits puede tener: esa absurdez
   fue la señal.

**Cotejo con `d8` sobre los veintiún cuerpos del par** (las dos sondas, instrucciones desmontadas sin
números de registro): **quince salen iguales instrucción por instrucción y con el mismo desplazamiento**,
incluido `tresNiveles` (`if-ne +8 | if-lez +6 | if-lez +4 | const/4 1 | return | const/4 2 | return`, palabra
por palabra la de d8 salvo el registro). Los seis restantes difieren por tres optimizaciones de d8, cada
una con su nombre:

- los dos `<clinit>`: d8 deja `return-void` (pliega la constante estática `shared = 5`); nosotros escribimos
  el `const/4` + `sput` que escribió javac;
- `conElse`, `anidadaConElse`, `enElElse`: d8 fusiona las colas idénticas (un solo `sput` al final, con
  saltos que a veces van hacia atrás); nosotros conservamos la forma de javac, con un `sput` por rama;
- `plegable`: d8 propaga las copias (`istore`+`iload` se convierten en `return`); nosotros no.

Ninguna de las tres es una diferencia de semántica que nos toque decidir: el dex que escribimos **lo relee
`d8` y lo re-dexa con `rc 0`** en los dos casos, y `apkc dex` da **el mismo fichero byte a byte** sobre las
clases de javac que sobre las nuestras (1796 bytes la sonda entera, 1684 el `MainActivity` de la página), que
es la prueba de que el traductor no distingue quién escribió el `.class`.

**Lo que se rechaza por nombre**, porque no lo midió ninguna sonda: una condición que no sea comparación (el
booleano suelto, donde javac usa `ifeq` sobre el valor), el `null` a la izquierda, dos referencias comparadas
entre sí, los tipos de dos huecos en una comparación, una rama **hacia atrás** (un bucle: el reparto de
registros en el punto de fusión no está medido; `d8` lo calcula con análisis de viveza), y un `goto` que pase
de 127 unidades (`goto/16`, 0x29, sin medir).

**Y en la página**: `studio/prueba-pestana.js` sobrescribe el fuente con dos branquias
(`if (pulsos > 2) { total = total * 2; } else { total = total + 1; }` y `if (raiz != null) { raiz.setVisibility(0); }`),
compila entero en chromium con `build --linker go --compiler go` y saca **un apk de 9477 bytes** que la
pestaña verifica sola (v1 `ok`, v2 `ok`) sin hacer ninguna petición fuera de su origen. Fuera de la pestaña,
`studio/e2e.sh` (bloque 4d) le pasa los mismos oráculos de siempre y **el nuevo**: `d8` relee el `classes.dex`
que escribió el navegador con las branquias dentro, `rc 0`.

Tests permanentes: `TestEmisorBranquiasContraJavap` (siete sucesiones con sus destinos, dos de ellas
anidadas), `TestTraduceBranquiasContraLasPalabrasDeD8` (seis cuerpos, con los desplazamientos que escribió
d8) y `TestRechazaUnaRamaHaciaAtras` (el `while` que no se acepta). Suite: **199 PASS + 2 SKIP** nativa y
**181 PASS + 20 SKIP** en el hospedador con forma de navegador.

## 53. Concatenar cadenas y plegar constantes: la versión 52 pega con `StringBuilder`, y `1 / 0` no se dobla

El fuente que alguien escribe en la página necesita decir «llevas 3», no solo decir «cuenta». Con `--release 8`
sobre javac 21, la concatenación **no** es `invokedynamic` (eso llega en la 55): es una cadena de
`StringBuilder`, y el andamiaje que falta en el subconjunto eran tres cosas —la cadena de `append`, el plegado
de constantes y el paréntesis como agrupación—. Sonda en `$PREFIX/tmp/e53/p` (cuatro clases) y comando:

```sh
javac --release 8 -d cj p/*.java                    # el oráculo
apkc compile --android-jar android-34/android.jar -o ours p
for c in Cadena Cero Comp Pliegue; do
  javap -p -c -l cj/p/$c.class   | sed 's/#[0-9]*/#n/g; s/  */ /g' > a.txt
  javap -p -c -l ours/p/$c.class | sed 's/#[0-9]*/#n/g; s/  */ /g' > b.txt
  cmp a.txt b.txt || diff a.txt b.txt
done
```

`javap` sale **idéntico línea por línea** en las cuatro clases: 134 líneas en `Cadena`, 35 en `Cero`, 206 en
`Comp` (la sonda de las branquias, que sigue igual) y 54 en `Pliegue`. Cuatro reglas, todas leídas de esas
líneas:

1. **la cadena de `append` empieza por el primer operando**: `a + "-" + b` es `new StringBuilder; dup;
   invokespecial <init>; aload_1; append(String); ldc "-"; append(String); aload_2; append(String); toString`.
   javac **no** usa el constructor `StringBuilder(String)`, así que el primer `ldc`/`aload` se appendea.
2. **los literales adjacentes se doblan en una sola constante**, y si después del doble no queda más que
   cadena, no hay `StringBuilder`: `"hola" + " " + 3` es `0: ldc "hola 3"; 2: areturn`. El entero se escribe
   en decimal, igual que lo escribiría `String.valueOf`.
3. **la agrupación es la del fuente**: en `1 + 2 + x + "s"` el subárbol `(1+2)+x` no es concatenación, sale
   como cuenta entera (`iconst_3; iload_1; iadd`) y se appendea con `append:(I)`. Por eso el analizador
   necesita el paréntesis: `"s" + (x + 1)` da `ldc "s"; append(String); iload_1; iconst_1; iadd; append(I)`.
4. **el tipo del operando eligió la sobrecarga**, medida la de entero, la de cadena y la de referencia
   (`"v=" + o` sale `append:(Ljava/lang/Object;)`). Lo no medido se rechaza: `append(Z)`, `append(C)`,
   `append(J)` y `append(D)` no los leyimos.

**El plegado de constantes enteras** tiene su propia sonda (`Pliegue`) y un límite que no es obvio:
`1 + 2` sale `iconst_3`, `1 + 2 * 3` sale `bipush 7` (el 7 va en `bipush` porque `iconst` llega al 5),
`7 - 10` sale `bipush -3`, `9 / 2` sale `iconst_4` y `9 % 4` sale `iconst_1`. **Pero `1 / 0` no se dobla**:
javac escribe `iconst_1; iconst_0; idiv`, que es la `ArithmeticException` en tiempo de ejecución y no en la
compilación. Nuestro plegador respeta esa excepción: con divisor cero devuelve «no plegable» y la cuenta se
emite entera, medido en `Cero` (35 líneas iguales).

**Dos piezas que faltaban salieron a la primera de dex**: `areturn` (`0xb0`) estaba en el traductor pero **no**
en la tabla del caminante, así que `apkc dex` se paraba con `byte 0xb0: opcode outside the table this fork
measured` en cuanto el fuente devolvía una cadena; y el analizador leía el `(` de `(x + 1)` como el principio
de un molde. El ancho de `areturn` es el que marca `javap`: `0: ldc …; 2: areturn` con la longitud del código
en 3, o sea un byte y ningún operando.

**Cotejo con `d8` sobre los treinta y nueve cuerpos** (las cuatro clases, desmontadas sin números de
registro): **veinticinco salen iguales instrucción por instrucción, con los mismos ids y los mismos
desplazamientos** —entre ellos `todoLiteral`, que es `const-string; return-object` en los dos ficheros— y
veintisiete si se ignoran los ids. Los doce que se separan tienen cuatro motivos, cada uno con su nombre:

- **`d8` reescribe la cadena de `StringBuilder`** (siete cuerpos, los siete que concatenan algo que no es
  solo literales): usa `invoke-direct <init>:(Ljava/lang/String;)` con la primera constante, y se salta los
  `move-result-object` porque sabe que `append` devuelve `this` — en `conEntero` quedan siete instrucciones y
  quince palabras (`new-instance | const-string | invoke-direct | invoke-virtual | invoke-virtual |
  move-result-object | return-object`) frente a nuestras diez instrucciones y veinte palabras;
- **pliega el `<clinit>`** de la constante estática (`static int total = 5` queda en `return-void`, un cuerpo);
- **mete el operando constante en la forma `/lit8`** (`add-int/lit8 v, v, 3` donde nosotros escribimos
  `const/4 3` y `add-int/2addr`, y `div-int/lit8 v, v, 0` en `1 / 0`), tres cuerpos;
- **compara contra cero con la forma `/z`** (`if-nez v` en `if (0 == a)`, donde javac carga el 0 en una
  palabra y `d8` lo reconoce como el cero), un cuerpo. En esta sonda no aparece la fusión de colas del §52:
  los `if` de `Comp` terminan cada rama en su propio `return`.

Nuestro fichero, con todo, **lo relee `d8` y lo re-dexa con `rc 0`**, y `apkc dex` da **el mismo fichero byte a
byte** sobre las clases de javac que sobre las nuestras: **3240 bytes**, `cmp` mudo. La prueba de que el
traductor no distingue quién escribió el `.class` sigue siendo la misma que en el §50.

**Lo que se rechaza por nombre**: concatenar con una llamada cuyo tipo no se puede estimar sin emitirla
(devolvería el mensaje «la concatenación está medida sobre literales, campos, variables y `this`»), un
`append` de tipo no medido, y todo lo de siempre: arreglos, bucles, `switch`.

Tests permanentes: `TestEmisorCadenaContraJavap` (ocho cuerpos con su sucesión de mnemonicos, sus
desplazamientos y sus destinos) y `TestTraduceLaConcatenacionBajaAStringBuilder` (la cadena baja al dex con
las formas medidas, y el plegado llega ya plegado: `1 / 0` conserva el `div-int/2addr` con su cero). Suite
después de este párrafo: **201 PASS + 2 SKIP** nativa y **181 PASS + 22 SKIP** en el hospedador con forma de
navegador (los dos tests nuevos piden el `android.jar` real a través de `jarDelSDK`, así que en el hospedador
sin montarlo se saltan: esos son los dos SKIP que faltaban).

**Y el proyecto entero, con la concatenación dentro**: la pestaña escribe `Toast.makeText(this, "cuenta " +
pulsos, Toast.LENGTH_SHORT).show()` en el fuente propio de su comprobación 11 y saca **un apk de 9598 bytes**
que verifica sola (v1 `ok`, v2 `ok`) sin salir de su origen; el bloque 4d del gate le pasa los oráculos de
siempre, **exige `cuenta` y `StringBuilder` en el dex** que escribió el navegador y `d8` lo relee con `rc 0`.
Nativo, sobre el proyecto generado por la página con ese mismo fuente (`$PREFIX/tmp/e53/medir-pagina.sh`):
**431 ms y un apk de 10096 bytes** —`apkc build --linker go --compiler go`, sin un solo proceso—, que
`apksigner` aprueba por v1 y v2, `zipalign -c -p 4` aprueba, `zipcheck.py` aprueba, `aapt2 dump badging`
lee como `launchable-activity: name='com.hola.apkcstudio.MainActivity'`, `apkc verify` dictamina `ok` y
`d8` relee su dex con `rc 0` (re-dexado: 2424 bytes frente a los 2736 que escribimos, otra vez por la
reescritura de la cadena).

## 54. La app del contador: anónima, campo del exterior, `if` y concatenación en el mismo cuerpo

El objetivo dice «un APK utilizable», y utilizable quiere decir que algo cambie cuando se pulsa. El fuente
que hace eso cabe en lo medido salvo una pieza: **escribir un campo del exterior desde dentro de la
anónima**. Sonda en `$PREFIX/tmp/e54/p`, con `javac --release 8 -cp android.jar` y `apkc compile` sobre el
mismo fichero:

```java
public class Contador extends Activity {
    int pulsos = 0;
    static int total = 5;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total = total + pulsos;
                if (pulsos > 3) { v.setContentDescription("muchos " + pulsos); }
                else { v.setContentDescription("pulsos " + pulsos); }
            }
        });
    }
}
```

**`javap -p -c -l` sale idéntico línea por línea** en las cinco clases: `Contador` (49 líneas),
`Contador$1` (69), `R` (10), `R$layout` (12) y `R$id` (12). Tres reglas nuevas, todas leídas de esas 69
líneas del `onClick`:

- **el campo del exterior se alcanza por la captura, dos veces**: `pulsos = pulsos + 1` es
  `aload_0; getfield this$0:Lp/Contador;` (el receptor del `putfield`), otra `aload_0; getfield this$0` con
  su `getfield p/Contador.pulsos:I` (la lectura del valor), `iconst_1; iadd`, `putfield p/Contador.pulsos:I`.
  javac **no** duplica el receptor con `dup`: vuelve a cargarlo. `d8` sí comparte la lectura —de ahí que su
  cuerpo tenga un `iget-object` menos que el nuestro—, que es una optimización suya, no una diferencia de
  semántica.
- **el estático del exterior no pasa por la captura**: `total = total + pulsos` es
  `getstatic p/Contador.total:I`, la captura para leer `pulsos`, `iadd` y `putstatic` — las formas del §50,
  ahora desde la anónima.
- **el `if` del §52 y la concatenación del §53 viven dentro del `onClick`**: `if_icmple 73`, dos cadenas de
  `StringBuilder` cerradas con `setContentDescription:(Ljava/lang/CharSequence;)V` (se les pasa un `String`,
  que es lo que ya midió `setText` en el §49) y el `goto 102` del `then` que no sale por su cuenta.

**El campo tiene que ser del paquete, y el `private` se rechaza por nombre.** Con `private int pulsos`, javac
no escribe el `getfield` contra el campo: escribe dos **métodos sintéticos del exterior**, leídos en la primera
corrida de esta sonda —

```
 8: invokestatic  // Method p/Contador.access$000:(Lp/Contador;)I
13: invokestatic  // Method p/Contador.access$002:(Lp/Contador;I)I
16: pop
```

lectura `access$000`, escritura `access$002` **que devuelve el valor** (de ahí el `pop`). Ni el número
(`000`, `002`) ni el lugar que ocupan en el `methods` del exterior están medidos, así que el emisor los
rechaza con ese texto en vez de inventarlos: es el único caso del contador que queda fuera, y se dice con su
motivo.

**Cotejo con `d8`**: sobre los ocho cuerpos de la sonda, **cinco salen iguales** instrucción por instrucción y
con los mismos ids y desplazamientos; los tres que se separan son los tres motivos de siempre —el `<clinit>`
plegado, la cadena de `StringBuilder` reescrita (con su `add-int/lit8` y su `invoke-direct <init>(String)`) y
el `this$0` compartido—. `apkc dex` da **el mismo fichero de 2304 bytes** sobre las clases de javac que sobre
las nuestras, y `d8` relee el nuestro con `rc 0`.

Test permanente: `TestEmisorAnonimaConCampoDelExterior`, con las dos sucesiones completas medidas (`enruta` y
el `onClick`).

**Falta una nota de proyecto**: el layout que escribe `apkc create` no tiene ningún `@+id`, así que un fuente
que haga `R.id.pulsar` se rechaza con «el campo id no esta en com/myapp/R ni en sus padres» —y está bien que
pase, porque el recurso no existe—. Al anadirle el botón (`android:id="@+id/pulsar"`), el enlazador de Go emite
su clase `id` (`public static final class id { public static final int pulsar=0x7f010000; }`) y el fuente
compila entero. Eso es lo que hace la comprobación 12 de `studio/prueba-pestana.js`: escribe el layout y el
`MainActivity` del contador en el proyecto que creó la propia pestaña, y compila dentro de chromium —**41
comprobaciones, 0 fallos, un apk de 10098 bytes** con v1 `ok` y v2 `ok`, y las **18** peticiones todas de su
origen—.

Fuera del navegador, el mismo proyecto (`apkc create` + los dos ficheros escritos a mano,
`$PREFIX/tmp/e54/medir-proyecto.sh` y el `repro` de la sonda): **499 ms y un apk de 10048 bytes** con
`--linker go --compiler go`, `apksigner` por v1 y v2, `zipalign -c -p 4`, `zipcheck.py`, `aapt2 dump badging`
que lee `launchable-activity: name='com.myapp.MainActivity'`, y su `classes.dex` (2724 bytes) **releído por
`d8` con `rc 0`** (re-dexado a 2408). El bloque 4e del gate comprueba además que en el dex del navegador están
`Lcom/myapp/MainActivity$1;`, `Landroid/view/View$OnClickListener;`, `setContentDescription`, `pulsos` y la
cadena que arma el concatenedor.

## 55. El teléfono dice la última palabra: tres defectos de `map_list` y de áreas que ni `d8` ni nuestros tests veían

El segundo hecho del plan («el apk se instala y la actividad arranca») se probó en este aparato con el shell de
Shizuku, sin `adb`: `rish` (en el PATH, `uid=2000(shell)`, Android 15). Los ficheros se empujan y se ordena con
`rish -c`, y el oráculo que cerró el caso es el verificador de ART, que vive en el propio aparato:

```sh
rish -c 'cat > /data/local/tmp/ours.dex' < build/classes.dex
rish -c '/apex/com.android.art/bin/dexdump /data/local/tmp/ours.dex >/dev/null 2>&1; echo rc=$?'
```

Cuesta un segundo por fichero y **no requiere instalar nada**. Con eso delante, el dex que escribía la pestaña
sacó tres defectos que ningún oráculo previo veía.

**Cómo se instaló y qué respondió.** `pm install -r /data/local/tmp/ir.apk` → `Success`;
`am start -W -S -n com.myapp/.MainActivity` → `Status: ok`, `LaunchState: COLD`, `TotalTime: 527`; el proceso
quedaba vivo (`pidof -s com.myapp`) y `logcat` no traía `FATAL`. Pero la primera vez el arranque moría así:

```
java.lang.ClassNotFoundException: Didn't find class "com.myapp.MainActivity" on path: DexPathList[...base.apk...]
Caused by: ... Failure to verify dex file '.../base.apk': Bad index for class_data_item field_idx: 8 >= 6
```

Es decir: `am start` respondía `Status: ok` **y la actividad no existía** — el aviso de ART estaba enterrado en
el `logcat`, no en la salida de `am`. Nada de esto lo veía `d8`: releía nuestro fichero con `rc 0` porque no
mira el `map_list` ni recorre las áreas de forma contigua.

**Defecto 1 — el `map_list` declaraba el área de los valores estáticos con un kind inventado.** Medido en el
fichero de `d8`, el mapa cierra con `0x2000 size=6 off=2659` y `0x2005 size=4 off=2746`: el
`encoded_array_item` es **0x2005**, y 0x2007 no existe. Con 0x2007 escrito por nosotros, ART respondía
`Offset(2537) should be aligned by 4 for maplist item of type 8199` (el 8199 es 0x2007 en decimal) y después,
corregido el kind, `Unknown map section type 2007`. Los arrays de `d8` empiezan en 2746, 2749, 2755 y 2761:
**sin alineación de 4** — el `class_data` termina justo donde empieza el primer array.

**Defecto 2 — relleno entre `class_data_item`, y ART recorre el área contigua.** Quedaba el aviso original
(`field_idx: 8 >= 6`) y la bisección lo acorraló: se generaron 12 variantes del dex (anulando el
`class_data_off` y el `static_values_off` de cada una de las seis filas de `class_defs`, con el `checksum`
Adler32 recalculado) y 6 más con las filas anuladas acumulativamente. **Las 18 siguieron dando el mismo
mensaje**, así que el puntero de la fila no importaba: lo que importa es que el verificador camina por el área
a partir del offset del mapa, item tras item. Y ahí estaba la diferencia:

| | item 0 | 1 | 2 | 3 | 4 | 5 |
| --- | --- | --- | --- | --- | --- | --- |
| `d8` (2659..) | 17 | 24 | 12 | 12 | 12 | 10 (termina en 2746) |
| el nuestro (2447..) | 20 | 24 | 12 | 12 | 12 | 13 |

Los nuestros llevaban tres ceros de relleno delante de cada item (un intento de alinear a 4 dentro del área), y
el caminante los leía como los cuatro recuentos de la clase siguiente: de ahí un `field_idx` 8 contra una tabla
de 6. Corregido en `areaClassData`: los items van pegados y el área se alinea a 1, como la de `d8`. El
`encoded_field` **no lleva byte de valor** —en el fichero de `d8` el item de `Lcom/myapp/MainActivity;` mide 24
bytes y termina justo donde empieza el de `R$id`, con dos uleb por campo—, y esa es la lectura que ART acepta.

**Defecto 3 — orden de `proto_ids`.** Tras pegar los items, ART pasó al siguiente aviso:
`Out-of-order proto_id return types`. Los doce protos de `d8` van con el tipo de retorno ascendente
(`Landroid/view/View;`, `Landroid/widget/Toast;`, `Ljava/lang/String;`, `Ljava/lang/StringBuilder;` y los ocho
`V`); el nuestro los ordenaba por `shorty` primero, y el `V` de `void` caía en medio. Corregido el orden en
`orderTables` (retorno, luego `shorty`, luego parámetros).

**Defecto 4 — el `encoded_value` de un estático sin `ConstantValue`.** El último aviso fue
`unexpected static field initial value type: 'L' vs 'I'`, sobre `static int total = 5;`: al no ser `final`,
javac y nuestro emisor no escriben `ConstantValue` en el `.class`, y nosotros anunciábamos el elemento del array
con el byte `0x1e`, que ART lee como un tipo de objeto. `d8` si lleva el valor (`01 04 05`: tamaño 1, cabecera
`04`, un byte con el 5) porque propaga el `<clinit>`; nosotros todavía no, así que se anuncia **un cero del tipo
del campo** —la cabecera `64` (entero de cuatro bytes) que `d8` usa para las constantes de `R`, con ceros—, y el
`<clinit>` lo sobrescribe al correr, que es lo que habría hecho de todas formas. Los tipos que no sean `I`
siguen con el comportamiento viejo: **no están medidos en el aparato**.

**Verificación final.** Con los cuatro arreglos: `dexdump` sobre el dex del enlace en Go responde **`rc 0` y 6
clases**; `pm install -r` → `Success`; `am start -W -S` → `Status: ok`, `LaunchState: COLD`, `TotalTime: 527`, y
`logcat` trae `Displayed com.myapp/.MainActivity for user 0: +527ms` sin ninguna línea `FATAL`; `pm compile -f
com.myapp` → `Success` y `dumpsys package dexopt` muestra `[status=verify] [reason=cmdline]` con su
`base.odex` (`artd ... kOatUpToDate with filter 'verify'`). El dex del fixture pasa de 2392 a **2384 bytes** (los
ocho bytes de relleno que se iban metiendo), y la suite nativa entera queda en verde. El gate, rehecho después de
los cuatro arreglos: **41 comprobaciones, 0 fallos**, y el apk del contador que escribe chromium pasa de 10098
(§54) a **10095 bytes**.

**Y el `onClick` corre en la pantalla.** Con el apk que escribió **chromium** (`$PREFIX/tmp/pestana-apk/
pestanera-contador.apk`, 10095 bytes; `classes.dex` con sha256 que empieza `a79c1a0e36821918`):
`pm install -r` → `Success`, `am start -W -S -n com.myapp/.MainActivity` → `Status: ok`,
`LaunchState: COLD`, `TotalTime: 316`. Dos `input tap 1280 837` (el centro del `com.myapp:id/pulsar`, que
`uiautomator` reporta en `bounds="[1192,789][1368,885]"`) dejan el botón con `content-desc="pulsos 2"`, y con
cuatro, `content-desc="muchos 4"` — o sea las **dos ramas del `if`**, la resta sobre el campo de instancia
capturado (`pulsos = pulsos + 1`), el estático del exterior (`total = total + pulsos`), la concatenación con
`StringBuilder` y la llamada virtual `setContentDescription`—. El `Toast` también se ve desde fuera: `logcat`
registra las capas `Toast#3760`/`Toast#3761` en `SurfaceFlinger`, y `FATAL EXCEPTION` aparece **0 veces**.

Con esto el segundo hecho del plan está cerrado sobre el artefacto del navegador, y la suite queda en
**205 PASS + 2 SKIP** nativa y **183 PASS + 24 SKIP** en el hospedador con forma de navegador.

**Queda por medir** (no bloquea el hecho 2): el `content-desc` de un `static` de tipo objeto sin constante, y en
general los `encoded_value` que no sean enteros —ahí sigue en pie el comportamiento viejo, sin oráculo—.

**Prueba permanente:** `apkc/dexlayout_test.go` — `verificaLayout` replica en Go las cuatro comprobaciones del
verificador de ART (kinds del mapa y offsets ascendentes, retorno de los protos no decreciente, recorrido
contiguo del área de `class_data` con los índices dentro de rango y el `code_off` alineado, y tipo del
`encoded_value` contra el descriptor del campo). Corre sobre tres ficheros: nuestro dex del fixture, **el dex
que escribió `d8`** (si la prueba no pasa con el oráculo, la prueba está mal) y la clase del contador con su
`static int total = 5`.

## 56. El `debug_info_item`: lo que faltaba del 2G.3, medido con `dexdump -d` y una traza en el teléfono

El formato se midió con una sonda propia, no con un manual. La sonda (`$PREFIX/tmp/dbg/probe/Sonda.java`)
son tres métodos con las líneas separadas a propósito: `cuenta(II)V` con las sentencias en las líneas 6, 7,
8, 9 y 11; `Sonda()` en la 15, la 13 (el campo), la 16 y la 17; `arma(String)` en la 20, 21, 22, 23 y 24.

```sh
"$J/javac" -d . probe/Sonda.java                      # sin -g: la forma en que escribe este fork
d8 --min-api 21 --lib "$ANDROID_JAR" --output . probe/Sonda.class
rish -c 'cat > /data/local/tmp/d8sonda.dex' < classes.dex
rish -c '/apex/com.android.art/bin/dexdump -d /data/local/tmp/d8sonda.dex'   # positions: 0x0000 line=20 …
```

Con las palabras crudas delante (`0f 00 0e 39 4d 4b 00`, `06 02 00 00 0e 1e 2d 3c 1f 00`,
`14 01 00 0e 5a 3c 4b 69 00`) salen cuatro cosas del formato, todas medidas:

- **el item empieza por la línea inicial**. El `header_addr` que cita la documentación del formato **no
  se escribe**: `d8` lo salta y el parser de ART también. Es la segunda vez en este proyecto que el manual
  describe un campo que quien escribe el fichero omite (la primera fue el `initial_value` del
  `encoded_field`, §22 y §55).
- **el recuento de argumentos no cuenta `this`**: `<init>()V` sale `00`, `arma(String)` sale `01`,
  `cuenta(II)V` sale `02`.
- **los nombres de los argumentos son un byte `00` cada uno** cuando el `.class` no trae
  `LocalVariableTable`, y `d8` **no los toma de `MethodParameters`**: recompilada la sonda con
  `-parameters`, el área de debug sale **idéntica byte a byte** y lo único que cambia en el fichero es que
  aparece la anotación `Ldalvik/annotation/MethodParameters;`. Con `-g` sí cambian (ahí van los
  `DBG_START_LOCAL`), y ese caso no se emite aquí.
- **la posición se marca con el opcode especial** `0x0a + 15·Δpc + (Δlínea + 4)`, cuyo rango es Δlínea en
  [-4, +10]; cerrar es `0x00`, y no hace falta `SET_FILE` porque el nombre del fichero ya está en
  `class_def_item.source_file_idx`.

Emisor en `apkc/dexdebug.go`, cableado en `armaDex`: área nueva `0x2003` justo detrás del código
(alineación 1) y el `debug_info_off` de cada `code_item` **parcheado** después de conocer la base, porque
es una referencia hacia adelante. Los items **idénticos comparten offset**: nueve cuerpos del fixture dan
**ocho items**, y el `map_list` declara ocho.

**Contra las palabras de `d8`.** Dexada la misma `.class` de la sonda por los dos caminos:

| método | `d8` | nosotros |
| --- | --- | --- |
| `<init>()V` | `0f 00 0e 39 4d 4b 00` | idéntico |
| `arma(Ljava/lang/String;)V` | `14 01 00 0e 5a 3c 4b 69 00` | idéntico |
| `cuenta(II)V` | `06 02 00 00 0e 1e 2d 3c 1f 00` | `… 2e 00`: el último salto es de dos unidades porque nuestro cuerpo tiene una más —la diferencia de secuencia de instrucciones de los párrafos 52 y 53, no del formato— |

**En el aparato.** Con el proyecto del contador rehecho: el dex pasa de 2716 a **2784 bytes**, `dexdump`
sigue diciendo `rc 0`, y `-d` lista **18 posiciones**: en `MainActivity$1.onClick` lee
`0x0000 line=18 … 0x0070 line=23`, que son las líneas reales del fuente. Y la prueba que le importa al
usuario: instalada una variante que divide por cero (`total = total / total`, con `total` en 0, línea 18
del fuente), al pulsar el botón el `logcat` del teléfono imprime

```
E/AndroidRuntime(29111): java.lang.ArithmeticException: divide by zero
E/AndroidRuntime(29111): 	at com.myapp.MainActivity$1.onClick(MainActivity.java:18)
```

Antes de este párrafo las trazas salían sin número. Ahora el apk que se compila en la pestaña lleva sus
propias líneas hasta el `logcat`, escritas por un emisor de Go.

**Efecto en las cuentas.** Fixture: 2384 → **2436 bytes**, y 13 → 14 entradas en el `map_list`. Suite
nativa: **208 PASS + 2 SKIP**, con tres pruebas nuevas en `apkc/dexdebug_test.go` (las palabras de `d8`,
el caso en que el delta se sale del especial y el área deduplicada releída con un decodificador propio).
En el hospedador con forma de navegador: **186 PASS + 24 SKIP**. Gate entero: `rc 0`, **41
comprobaciones**, y los apks que escribe chromium crecen con su área de depuración —el del contador pasa de
10095 (§55) a **10169 bytes**, el compilado de la página generada de 9834 a **9888**, y el del fuente
escrito a mano de 9589 a **9638**—.

**Defecto aparte, encontrado de paso y todavía SIN ARREGLAR.** La primera variante de la sonda de crash
metía dos locales nuevos dentro del `onClick` (`int cero = 0; int k = pulsos / cero;`), y ART rechazó la
clase: `VerifyError: … onClick failed to verify: [0x0] instance field access on object that has
non-reference type Undefined`. Medido en su `code_item`: `registers=10`, `ins=2`, y el cuerpo usa `v6`
para `this` — el traductor numera los locales del `.class` en orden (`slot i → v(base+i)`), así que los
argumentos caen en `v6`/`v7` cuando el formato exige que los `ins` registros **sean los últimos** de la
frame (`v8`/`v9`). Mientras `locales == ins` las dos cuentas coinciden, y por eso el subconjunto medido
hasta hoy (el `onClick` del contador: dos locales y dos argumentos) salía bien. En cuanto un método tiene
más locales que argumentos el dex sale **inverificable**. Hay que remapear la asignación para que los
argumentos ocupen la parte alta de la frame, medido contra la numeración de `d8`, y añadir una prueba con
un método de tres locales y dos argumentos. **Arreglado en el §57.**

## 57. La frame: los argumentos van al final, y eso solo lo sabía ART

El defecto del último párrafo del §56, cerrado con medición propia. Lo que decía el aparato al instalar
una app cuyo `onClick` declaraba dos locales más que argumentos:

```
E/AndroidRuntime: java.lang.VerifyError: Verifier rejected class com.myapp.MainActivity$1:
    void com.myapp.MainActivity$1.onClick(android.view.View) failed to verify:
    [0x0] instance field access on object that has non-reference type Undefined
```

Medido en nuestro `code_item`: `registers=10`, `ins=2`, y el cuerpo leía `this` en **`v6`**. La numeración
de `d8` para la misma forma (sonda `probe/Sonda.java` con `largo(String)`: cuatro huecos JVM —`this`,
`entrada`, `n`, `m`— y dos argumentos) sale `registers=3`, `ins=2`, y `dexdump -d` muestra

```
0000: invoke-virtual {v2}, Ljava/lang/String;.length:()I   <- entrada = v2, el ultimo registro
0006: const/4 v0, #int 3                                    <- n, m y los temporales, por debajo
000d: iput-object v0, v1, Lprobe/Sonda;.nombre              <- this = v1 = registers - ins
```

O sea: **los `ins_size` registros últimos de la frame son los argumentos**, y los demás locales y los
temporales van por debajo. Nuestro traductor copiaba el orden del `.class` (hueco JVM `i` → `v(base+i)`),
que coincide solo cuando no hay locales extra; con dos locales de más los argumentos caían en `v6`/`v7`
mientras ART miraba `v8`/`v9`, y `v6` no tenía tipo.

El arreglo está en `apkc/dexinstr.go`, una permutación en `local(n)`: el hueco `n < args` pasa a
`v(base + locales - args + n)` y los demás a `v(base + n - args)`. Con `locales == args` la fórmula es la
identidad, así que **no mueve ninguna de las sucesiones ya medidas**: las comparaciones con las palabras de
`d8` de `dexinstr_test.go` siguen pasando sin retocar una sola palabra, y el fixture sigue en 2436 bytes.

Prueba permanente: `TestLaFramePoneLosArgumentosArriba`, que fija la cuenta (`this` en `v8`, el argumento
en `v9`, los dos locales del cuerpo en `v6` y `v7` con `registers=10` e `ins=2`) y comprueba que con
`locales == argumentos` la numeración no se mueve.

Y la prueba en el aparato, con la misma fuente que antes no pasaba el verificador —`int cero = 0;` e
`int k = pulsos / cero;` dentro del `onClick`, la división en la línea 21 del fuente—:

```sh
rish -c '/apex/com.android.art/bin/dexdump /data/local/tmp/ours.dex >/dev/null 2>&1; echo rc=$?'  # rc=0
rish -c 'pm install -r /data/local/tmp/rompe.apk'                                                  # Success
rish -c 'am start -W -S -n com.myapp/.MainActivity; input tap 1280 837'                             # Status: ok
# logcat:
E/AndroidRuntime( 6432): java.lang.ArithmeticException: divide by zero
E/AndroidRuntime( 6432): 	at com.myapp.MainActivity$1.onClick(MainActivity.java:21)
```

El `VerifyError` desapareció, el método **corrió** y la traza trae la línea exacta de la división: frame y
depuración vistas desde fuera en la misma corrida. Lo que esto abre es el caso que el subconjunto todavía
no cubría —cualquier método con variables locales además de los argumentos—, que hasta ahora producía un
apk que el teléfono rechazaba en silencio hasta el primer toque.

**Cuentas después de este arreglo.** Suite nativa **209 PASS + 2 SKIP**; en el hospedador con forma de
navegador **187 PASS + 24 SKIP**; gate entero `rc 0` con **41 comprobaciones**. El fixture sigue en 2436
bytes y el apk del contador de la pestaña en 10169 (ahí `locales == ins`, así que nada se mueve); el apk
del fuente escrito a mano pasa de 9638 a **9636** —dos bytes menos, porque en `onCreate` hay un local
además de los argumentos y la numeración nueva cambia algún formato de registro; no lo he desglosado—.

## 58. El bloque de anotaciones, medido byte a byte (y por qué es la diferencia del 2G.4, no el 2G.3)

Es lo único que separa nuestro `classes.dex` del de `d8` desde el §56, aparte de la cadena con la que
`d8` se firma a sí misma. Está **medido**, y en ese momento aún **sin emitir**: `annotations_off` salía cero en nuestras clases. La
medición va aquí para que no se vuelva a descubrir a ciegas. Dos sondas —`$PREFIX/tmp/e57/an`, una anónima
dentro de un método, y `$PREFIX/tmp/e58/an`, con `@Deprecated` en clase, campo, campo estático y método—,
ambas compiladas con el `javac` del teléfono y el `d8` del SDK y leídas con
`/apex/com.android.art/bin/dexdump -a` en el aparato.

**`annotation_item`** (kind `0x2004`, alineación 1: d8 lo pone en 741 y en 1396, impar): es un
`encoded_annotation` = `ubyte` de visibilidad (`0x02` = SYSTEM en las `Ldalvik/annotation/*`), `type_idx`
uleb128, `size` uleb128 y por cada elemento un `name_idx` uleb128 seguido de un `encoded_value`.

**`encoded_value`** con la numeración que ya usa este fork (`valueInt = 0x04`, `valueString = 0x17`,
`valueType = 0x18`, `valueMethod = 0x1a`, `valueArray = 0x1c`, `valueNull = 0x1e`; el tipo van en los
**cinco bits bajos** y el código de anchura en los **tres altos**, anchura = `arg + 1` bytes):

| bytes | qué son | medido en |
| --- | --- | --- |
| `1e` | null, sin datos | `names={null, null}` |
| `44 10 80 00` | entero en 3 bytes = 32784 | `accessFlags={32784, 4112}` |
| `24 10 10` | entero en 2 bytes = 4112 | la segunda fila del array |
| `1c 02 …` | array de 2 elementos | el array de `MethodParameters` |
| `17 00` | cadena = índice 0 (`"()V"`) | `Signature; value={"()V"}` |
| `1a 0a` | método = índice 10 (`largo`) | `EnclosingMethod; value=largo` |

**`annotation_set_item`** (`0x1003`, alineación 4) = `u32` con el número de elementos y una lista de
`u32` con los desplazamientos de cada `annotation_item`. **Ojo, medida, contra lo que se cita
normalmente**: `d8` los escribe **sin ordenar** —el set de `Lcom/myapp/R$id;` es `{2623, 2614}`, decreciente—,
así que el `Integridad`/comparador byte a byte no puede exigir orden ascendente.

**`annotations_directory_item`** (`0x2006`, alineación 4). Con campos anotados (`$PREFIX/tmp/e58/an`,
directorio en 776) la rejilla de `u32` sale:

```
+ 0: 768   class_annotations_off      (un set_item con la anotacion @Deprecated)
+ 4:   2   fields_size
+ 8:   1   annotated_methods_size
+12:   0   annotated_static_fields_size
+16:   0   field_annotation[0].field_idx        = 0 (nombre)
+20: 768   field_annotation[0].annotations_off  -- y el campo 1 apunta al MISMO set (24: 1, 28: 768)
```

Es decir: **cuatro `u32` de cabecera y luego las listas**, con los `u32` de campos estáticos/instancia y
parámetros que cita la documentación del formato *no escritos* por `d8` cuando van a cero —y `annotations_off`
de un campo o un método apunta directo a un `annotation_set_item` (misma forma que un `annotation_set_ref_list`:
`u32` cuenta + `u32[]`), sin que el mapa declare nunca el kind `0x1002`—. Medido también que la cabecera de
seis `u32` que cita el manual **no es lo que hay**: leyéndola así, en `probe.Sonda$1` (directorio en 1496,
con un solo método anotado) saldría `annotated_instance_fields_size = 7`, que es imposible en una clase con
cero campos.

**Qué elige `d8` emitir** (leído con `dexdump -a`, no inferido): en la clase `InnerClass` con
`accessFlags` y `name` (nulo para una anónima), y `EnclosingMethod` (valor = índice de método) o
`EnclosingClass` (valor = tipo) según la clase venga de un método o de otra clase; en el constructor de la
anónima `MethodParameters` con dos arrays (`accessFlags` con los flags del `.class`, `names` nulos cuando el
nombre no está declarado) y `Signature`. `MethodParameters` **solo aparece si el `.class` trae el atributo**: la sonda compilada sin `-parameters` no
lo lleva en los métodos escritos a mano y sí en el `<init>` sintético de la anónima, que es lo que javac
hace por defecto. Nuestro emisor también lo escribe en ese constructor, así que el doble oráculo
(`apkc dex` dando el mismo fichero sobre las clases de javac y sobre las de este fork) **no debería
romperse**, pero eso es una comprobación que queda pendiente de hacer junto con el emisor: si al emitir las
anotaciones las dos salidas divergen, la diferencia será un atributo que un lado pone y el otro no, y hay
que resolverlo por medición y no por ajuste.

Por eso esto pertenece al **2G.4**, no al 2G.3: el 2G.4 acepta la diferencia documentada *si el apk se
instala y arranca*, y esa condición se cumplió en el §55 y volvería a comprobarse igual al emitirlas.
**Y se emitió**: el párrafo siguiente escribe el bloque y comprueba contra ese mismo oráculo.

## 59. El bloque de anotaciones **se emite**, y `dexdump -a` ya no distingue el nuestro del de `d8`

Con lo medido en el §58 escrito en `apkc/dexanota.go`, el fixture del studio pasa de **2436 a 2696 bytes** y
el `map_list` de 14 a **17 entradas** —las mismas que declara `d8` sobre las mismas seis clases—, con las
tres áreas nuevas en el mismo orden y con las mismas cuentas que él: `annotation_item` **8** (el suyo en 2311),
`class_annotation_set_item` **6** (en 2484) y `annotations_directory_item` **5** (en 2548). El proyecto del
contador pasa de 2724 bytes (§54) a **3108** y su apk a 10292. Lo que sigue son tres defectos que salieron
del camino, no adornos: cada uno se vio porque las tablas dejaron de coincidir byte a byte o porque `dexdump`
lo contó distinto.

**Defecto 1 — el atributo `EnclosingMethod` son dos `u2`, no tres.** `apkc/classreader.go` leía
`class_index`, `name_index`, `descriptor_index`; el JVMS 4.7.7 y `javap -v` sobre la anónima del fixture
(`EnclosingMethod: #35.#37`, y `#37 = NameAndType #38:#39 // onCreateOptionsMenu:(Landroid/view/Menu;)Z`)
dicen `class_index` y **una sola entrada del pool** que nombra al método. Con la lectura vieja salían
`nombre=""` y `desc=""`, y el internado se llevaba por delante una cadena vacía, un proto `()""` y un
`method_id` fantasma: `type_ids`, `field_ids` y `method_ids` dejaron de ser idénticos a los de `d8` (53
cadenas contra 52, 10 protos contra 9, 18 métodos contra 17). Ahora `metodoDelPool` resuelve las dos formas
(NameAndType directo, o `CONSTANT_Method`/`Methodref` con el índice de clase delante) y **devuelve vacío si
el resultado no empieza por `(`**: lo no medido se queda sin emitir en vez de inventarse un índice. Resultado:
52/23/9/4/17 otra vez, y el test de las tablas fijas vuelve a callar.

**Defecto 2 — `parameters_count` es un `u1`.** Leído como `u2`, el cuerpo `01 00 00 80 10` de la anónima
(un parámetro, nombre ausente, flags `0x8010` = final|mandated) se desfasaba: el conteo salía 256, el índice
de nombre se comía los `00 80` y el de los flags se caía del atributo, así que el array emitido decía
`accessFlags={0}` donde `d8` escribe `{32784}`. Con el `u1` la fila sale `{nombre: access:32784}` y el
`dexdump -a` del aparato coincide.

**Defecto 3 — `accessFlags` de `@InnerClass` viaja **tal cual lo dejó javac en `InnerClasses`**.** Medido:
`25` (0x19, public+static+final) para `R$id`, `R$layout` y `R$style`, y `0` para la anónima. Este fork lo
pasaba por `accesoClaseDex`, que en el dex junta public y static en el mismo bit y habría escrito otro
número. Los flags del `.class` no se traducen aquí.

**Lo que manda el oráculo, no el manual.** El índice de las listas del directorio es un **`u32` entero**: `d8`
escribe `06 00 00 00 b4 09 00 00` (método 6, set en 2484) y con un uleb el fichero queda dos bytes desfasado.
Las reglas de qué va dónde, leídas de su fichero: `@EnclosingMethod` (valor = `method_id` del método que
envuelve) para la anónima; `@EnclosingClass` (valor = tipo del exterior) para una clase miembro **con nombre**;
`@InnerClass` para toda clase que tenga su propia fila en `InnerClasses`, con `name` nulo si es anónima;
`@MemberClasses` en el exterior con un array de los tipos miembro **en el orden de las filas del `.class`**
(en `R`: style, layout, id, que es como javac las escribe); `@MethodParameters` solo en los métodos cuyo
`.class` trae el atributo, con los dos arrays paralelos. `MainActivity` no lleva directorio: su
`annotations_off` es cero, como el nuestro. Los `annotation_item` **se comparten entre destinos**: el
`@EnclosingClass` que en `d8` está en 2348 (en el nuestro en 2192) lo apuntan `R$id`, `R$layout` y `R$style`;
por eso diez referencias dan ocho ítems, y ocho declara el mapa. Lo que **no** coincide es el orden de los ítems
dentro del área (`d8` pone primero el `MethodParameters` de la anónima; nosotros, los de clase) —los bytes de
cada ítem son los mismos, y así lo comprueba el test.

**No se emite `Signature`**, por el mismo motivo del §58: `d8` copia el atributo troceado a su manera en un
array de cadenas (`"<T::" "Ljava/lang/CharSequence;" ">" "Ljava/lang/Object;"`) y esa troceadura no está
medida; además el subconjunto de Java de `apkc compile` no tiene parámetros de tipo.

**Lo que queda distinto sobre las mismas seis clases, contado hoy** (mapeo offset a offset de los dos
`map_list` y volcado de las dos áreas con el lector del §16): las 52 cadenas (falta la `~~D8{…}` de `d8`, §22),
**8 `debug_info_item` contra los 7 que declara `d8`** —dos de nuestros items llevan una última posición de
línea que su equivalente de `d8` no lleva: el nuestro de `onMenuItemClick` es `16 01 00 0e c3 00` y el suyo
`16 01 00 0e 00`; el nuestro de `onCreateOptionsMenu` es `14 01 00 0e e6 00` y el suyo `14 01 00 0e 00`. Esos
bytes de más impiden que dos cuerpos compartan el item, y por eso él se queda en siete y nosotros en ocho;
la regla por la que `d8` descarta esa posición final **no está medida**, y se queda escrita como diferencia—,
**9 `type_list` contra sus 8**, y el orden de los ítems dentro del área de anotaciones. Todo lo demás —las
cinco tablas de índices, `class_defs`, los tres directorios, los seis sets y los ocho ítems byte a byte—
coincide.

**Comprobaciones (aparato Android 15 por `rish`, y tests permanentes).** `apkc/dexanota_test.go` decodifica
las tres áreas con el lector del §16 y exige que el volcado en texto del nuestro sea **igual línea por línea**
al del fixture de `d8`, y que el multiconjunto de bytes de los `annotation_item` sea el mismo (8 distintos).
Sobre el proyecto del contador, construido con `--linker go --compiler go` en 521 ms: `dexdump` **sin `-j`**
(verificación encendida) sobre nuestro dex → `rc 0`; `d8 --release` lo re-dexa → `rc 0` (3108 → 3092);
y el bloque `dexdump -a` —21 líneas— sale **idéntico con `diff` mudo** contra (i) el re-dex de `d8` sobre
nuestro fichero y (ii) el dex que `d8` saca de las clases de `javac` del mismo proyecto
(`$PREFIX/tmp/e54/proyecto-javac.dex`, 3188 bytes, 19,28 s de build con JVM contra 521 ms nuestros). En
`pm install -r` → `Success`; `am start -W -S` → `Status: ok`, `LaunchState: COLD`, 1171 ms, `pidof` 5200 vivo,
y `dumpsys package dexopt` lo marca `[status=verify]`. Como `onCreate` construye el oyente anónimo, esa clase
anotada se instancia al arrancar: el cargador de ART lee su `annotations_off` sin quejarse. `apksigner` (v1 y
v2), `zipalign -c -p 4`, `zipcheck.py`, `aapt2 dump badging` y `apkc verify` siguen aprobando el apk. Suite:
**211 PASS + 2 SKIP** nativa, **189 PASS + 24 SKIP** en el hospedador con forma de navegador. El gate de
`studio/e2e.sh`, entero y con el cambio hecho: **todo en orden**, con la pestaña en **41 comprobaciones** y
sus apks de chromium re-medidos —el del proyecto generado **10030 bytes** (igual entrada por entrada al
nativo, `d8` releyendo su dex), el del fuente escrito a mano **9729**, el de la app del contador **10305** y
el de la pestaña que compila **9255**—. El tiempo del camino sin procesos del contador sigue en **521 ms**.

*(Los números de este párrafo son de antes del §60: después de interna los `type_list` por contenido y de
adoptar la política de posiciones del `--release`, el fixture mide 2680 bytes, siete `debug_info_item` y ocho
`type_list`, y el dex del contador 3092.)*

## 60. Las dos diferencias que quedaban: la marca de `d8` se descarta a propósito y el `type_list` sobrante era un bug

Cierra lo que el §59 dejó escrito como diferencia, con las tres cosas que faltaban medidas.

**1. El `type_list` sobrante era un defecto nuestro, y estaba en la clave del internado.** `areaListas`
guardaba cada lista con prefijo de origen (`"i|"+clase` para las interfaces, `"p|"+parametros` para los
protos), así que dos listas con **el mismo contenido** se escribían dos veces. Medido sobre el fixture,
recorriendo el área (cada item mide `4 + 2·size` bytes y el siguiente se alinea a 4) y anotando quién apunta
a cada una: en el fichero de `d8` el item de `+0`, `[Landroid/view/MenuItem$OnMenuItemClickListener;]`,
lo apuntan **el proto `(Landroid/view/MenuItem$OnMenuItemClickListener;)Landroid/view/MenuItem;` y el
`class_def_item` de `MainActivity$1`**; nosotros teníamos dos items iguales (`+0` y `+8`), nueve listas
contra sus ocho. La regla es entonces: **se interna por contenido, sin distinguir el uso**. Corregido,
nuestra área sale byte a byte idéntica a la de `d8` (64 bytes, los mismos offsets `+0 +8 +16 +28 +36 +44
+52 +60`) y el fichero baja de 2696 a **2688**. De paso, medida también la terminación del área: `d8` no la
rellena hasta un múltiplo de cuatro (su área mide 66 hasta el `string_data_item` que sigue), así que se
quitó el relleno final. Prueba permanente: `TestElAreaDeListasEsLaDeD8`, que compara la sección byte a
byte, exige 8 listas en los dos mapas y comprueba que el proto y el `class_def` apuntan al mismo offset.

**2. La marca `~~D8{...}` se descarta, y ahora está medida la razón.** Su contenido, leído del fixture
(byte a byte, con su `string_data_item`):

```
~~D8{"backend":"dex","compilation-mode":"release","has-checksums":false,"min-api":21,
     "sha-1":"084a89126ae0596c599143d57a65464646af2831 3","version":"9.2.4-dev"}
```

Es la firma con la que `d8` nombra su propio trabajo: dice otra herramienta, su versión y **un resumen
SHA-1 del fichero que escribió**. Copiarla sería doble mentira —el origen y un digest que no corresponde a
nuestros bytes—, y no es que nadie la necesite: medida también la soledad de esa cadena, ninguna columna de
las tablas la nombra (ni `type_ids`, ni los `shorty_idx` de `proto_ids`, ni los `name_idx` de campos y
métodos, ni los `source_file_idx`; los elementos de las anotaciones se inspeccionaron en el §59 y tampoco).
Es la única cadena de más en su tabla: **53 contra nuestras 52**. Queda escrita la decisión y su prueba en
`TestLaMarcaDeOrigenDeD8NoSeCopia`, que además falla si el fixture dejara de traerla (para que la
comprobación no se quede dormida). Emitir una marca propia (`~~apkc{...}`) sería honesto pero añadir a
propósito una diferencia nueva donde hoy tenemos una medida: no se hace.

**3. El `debug_info_item` de más era la política del modo, y adoptamos la del `--release`.** Volviendo a
dexar las mismas seis clases y la sonda del §56 en los dos modos —`$PREFIX/tmp/an/r` (2844 bytes, `--release`,
exactamente el tamaño del fixture: el fixture está hecho así) y `$PREFIX/tmp/an/d` (2852, `--debug`)—: con
`--debug` las **nueve** sucesiones de posiciones del oráculo son idénticas a las nuestras, y con
`--release` falta en tres cuerpos la última, que es la que javac pega a la **última instrucción**
(`onMenuItemClick` pierde la línea 23, `onCreateOptionsMenu` la 26, `onCreate` la 16; los cuatro `<init>`
de una sola línea conservan la suya, porque es además la primera). La regla medida: **se suelta la posición
cuya unidad dalvik es el arranque del `return` final, salvo si es la única posición del cuerpo**. Como
`apkc` invoca a `d8` con `--release` (`build.go`: `{"--lib", androidJar, "--release"}`), el camino en Go
imita ese modo: `itemDebug` recibe las palabras del cuerpo y `areaCodigo` las devuelve por clase y método.
Efecto en el fixture: **siete items, los mismos que declara el mapa de `d8`**, y el fichero en **2680
bytes**; las sucesiones de *números de línea* por cuerpo coinciden con las de `d8 --release` en los nueve
(`TestLasLineasDeDepuracionSonLasDeD8Release`) y seis de los siete items salen byte a byte iguales —el de
`onCreate` difiere en la unidad de una posición porque nuestro cuerpo la pone en `+3` y el de `d8` en `+5`,
diferencia de asignación ya aceptada en el 2G.4—.

**La política no se copió a ciegas: se comprobó en el aparato con una traza.** Sonda nueva
(`$PREFIX/tmp/e60/traza`), con la división por cero **dentro de `onCreate`** para no necesitar ningún
toque: línea 16 `int cero = 0;`, línea 17 `int k = 72 / cero;`. Construida dos veces con el mismo binario,
una por cada camino —`--compiler go` (289 ms, dex 2720 bytes) y `javac`+`d8` (6,39 s, dex 2848)—, las dos
instaladas y arrancadas (`pm install -r` `Success`, `am start -W -S` `Complete`): las dos trazas dicen
`E/AndroidRuntime: java.lang.ArithmeticException: divide by zero at
com.myapp.MainActivity.onCreate(MainActivity.java:17)` —**la línea de la sentencia, no la de la llave**—.
La `LineNumberTable` de ese `onCreate`, leída con `javap -l` sobre el `.class` que escribe este fork, es
`14:0 15:5 16:11 17:13 18:18 19:26 25:43`: la última fila (25) es la llave del método, pegada al
`return-void`, y es justo la que la política del `--release` deja fuera; la 17, la de la división, sigue
publicada. Es decir: soltar la cola no cuesta precisión en las trazas, y los dos caminos de la herramienta
coinciden.
Sobre el contador completo, `dexdump` verificando da `rc 0` en nuestro dex de 3092 bytes y `d8 --release` lo
re-dexa a 3088 con `rc 0`; comparando con `dexdump -d` contra el dex de `d8` sobre las clases de `javac`, la última
línea anunciada por cada cuerpo es la misma en los dos (el `onClick` de `d8` publica siete posiciones y el
nuestro cinco porque el suyo vuelve a pasar por 20 y 21 al reconstruir el `if`, mientras la
`LineNumberTable` que escribimos va una fila por sentencia: medida en `javap -l` sobre los dos `.class`,
ambos con seis filas y la última en la línea 23).

Suite tras los tres cambios: **214 PASS + 2 SKIP** nativa y **192 PASS + 24 SKIP** en el hospedador.

## 61. Ejecutar en vez de instalar: `app_process` corre el dex en el terminal y destapó tres cosas

Sugerencia del usuario el 2026-11-01: para no pensar en la app mientras se adquieren instrucciones,
**ejecutar** lo que este fork compila. Se puede sin instalar nada: `app_process64` del aparato corre un
`.dex` suelto con un `main`, por `rish`, en un segundo. El comando, medido:

```sh
base64 -w0 algundex.dex | rish -c 'toybox base64 -d > /data/local/tmp/algo.dex'
rish -c 'export CLASSPATH=/data/local/tmp/algo.dex; /system/bin/app_process64 /data/local/tmp p.Arranque; echo rc=$?'
```

El banco quedó versionado en `banco/` (`sh banco/banco.sh`): dos fuentes (`p.Ver`, emitida por
`apkc compile`; `p.Arranque`, de javac, porque `main` necesita un tipo de array que el analizador aún
rechaza), y cinco salidas comparadas por el **código de salida del proceso** (`System.exit(total)`). En
el fuente, `total = 5`, dos llamadas a `cuenta()` que suman de dos en dos con un `if (total > 10)` que
no llega a activarse: la respuesta es **9**, y la dan los cuatro caminos —

| camino | resultado |
| --- | --- |
| JVM sobre las clases de javac | `rc 9` |
| **nuestro dex** (`apkc compile` + `apkc dex`, 1008 bytes) en `app_process` | `rc 9`, y `dexdump` verificando `rc 0` |
| `d8` sobre javac (1104 bytes) en `app_process` | `rc 9`, `dexdump rc 0` |
| `d8` sobre **nuestra `Ver`.class** en `app_process` | `rc 9`, `dexdump rc 0`, y su dex sale **byte a byte idéntico** al de javac (los mismos 1104 bytes: `cmp` mudo) |

Cuatro cosas salieron de aquí, y ninguna las daban los oráculos de bytes.

**1. `invokestatic` de cero argumentos estaba rechazado por error.** El banco no dexaba: `a call of 0
arguments: 35c goes up to five and invoke/range is not measured here`. Medido en el fichero de `d8`, la
forma `35c` **sí** existe con recuento cero: `7100 0600 0000` (opcode `0x71`, nibble alto del segundo
byte = 0, índice del método, y la tercera unidad en cero). Corregido el guardia (`n < 1` sobraba) y
fijado con `TestLlamadaSinArgumentosSaleTresCincoceta`, que además comprueba el `sget`, el
`invoke-static {v0}` de `System.exit` y las 12 palabras del cuerpo.

**2. El `.class` que emite este fork no verifica en una JVM real.** `java -Xverify:all` sobre nuestra
`Ver` dice `VerifyError: Expecting a stackmap frame at branch target 24` en `if_icmple`, y el
`javap -v` lo confirma: **cero** atributos `StackMapTable` en nuestro `.class`, dos en el de javac. El
`javap -c -p` del oráculo doble compara las *instrucciones*, no la tabla, y `d8` traga la ausencia
(`rc 0` dexando nuestra `Ver`), así que el producto no se rompió nunca: lo que falta es
**interoperabilidad con una JVM** —el `.class` de este fork solo lo consume su propio dexer—. Va al
plan como **5.23**, y el banco lo señala en cada corrida. *(Cerrado en el §65: la JVM ya verifica y
ejecuta nuestro `.class`.)*

**3. `getstatic`/`putstatic` de una referencia no está medido.** `System.out.println` no pasó de
`apkc dex`: `a static access of type L: solo se midió el entero` (`dexinstr.go:265`). El banco lo
evita escribiendo el resultado por `System.exit`. Va como **5.24** (mide `sget-object`/`sput-object`
y el `Ljava/io/PrintStream;` del pool).

**4. El `if` con `else` pide más temporales de los que cabe.** Con el `else` del fuente de arriba,
`apkc dex` rechazó: `at pc 30, iconst_4: it needs more temporaries than the max stack javap measured`
—la pila no se descarga en el punto de unión de los dos brazos, y el presupuesto de temporales
(`base = max_stack + locales + 1`, párrafo 50) se queda corto. La diana, medida con `dexdump -d` sobre
`d8 --release` del mismo fuente: `registers 3, ins 0, outs 0`, **20 palabras**, cada sentencia fusionada
(`sget v0` + `add-int/lit8 v1, v0, #2` + `sput v1`), la comparación invertida **conservando el
registro del sumando** (`const/16 v2, #int 10` + `if-le v1, v2, +0007`), el brazo `then` dentro del `if`
seguido de su `return-void`, y el `else` colocado **después** de ese retorno. Ojo con el plegado, que
parece una errata y no lo es: `total = total - 4` sale `add-int/lit8 v0, v0, #int -2`, porque `v0` es
el `total` *antes* de sumar los dos —`d8` calcula sobre el registro, no sobre la sentencia—. Va como
**5.25** y es la antesala directa de los bucles (5.10).

Anotación de paso, para que la comparación de registros no sorprenda: nuestro `main` declara **3**
registros donde el de `d8` declara **1** (reutiliza el hueco del `String[]` que nadie lee); es la
diferencia de numeración ya aceptada en el 2G.4, medida otra vez sobre un caso que ahora **corre**.

Suite tras el arreglo del punto 1: **216 PASS + 2 SKIP** nativa (los dos nuevos, del banco:
`TestLlamadaSinArgumentosSaleTresCincoceta` y `TestNuestrasClasesEmitidasSeDexanYLeyen`) y **194 PASS + 24
SKIP** en el hospedador con forma de navegador, con las cuentas de tamaño del dex intactas (2680 bytes:
ninguna sucesión del fixture lleva una llamada sin argumentos) y el gate de `studio/e2e.sh` otra vez en
orden con los mismos apks de la pestaña (10014, 9725 y 10285 bytes).

## 62. Con el banco ejecutando de verdad: estáticos de referencia, sobrecargas y un temporal que se perdía

Seguir el camino del §61 —ejecutar en vez de instalar— destapó cuatro cosas más, y tres eran fallos
reales del emisor y del traductor. El banco (`banco/banco.sh`) ya no compara solo el código de salida:
el fuente de prueba imprime con `System.out.println`, así que compara **el texto y el código de salida**
del mismo programa por los cuatro caminos.

**1. `sget-object` y `sput-object`, medidos.** Sonda `Lanza` con `System.out.println(que)` y
`mensaje = que` (un String estático). En el fichero de `d8`: `sget-object v0, field@0000` → **`6200
0000`** y `sput-object v1, field@0001` → **`6901 0100`**, o sea la misma forma 21c con el hueco de dos
opcodes entre el entero y el de objeto (`sget` 0x60, `wide` 0x61, `object` 0x62; `sput` 0x67, 0x68, 0x69).
`accesoEstatico` acepta ahora los dos prefijos medidos (`I` y `L`) y sigue rechazando el resto por nombre.
Con eso, nuestro dex de `Lanza.di` sale **palabra a palabra igual** al de `d8` (`6200 0000 | 6e20 0000
1000 | 6901 0100 | 0e00`), y en el aparato imprime `uno`/`dos`.

**2. El internado no recorría `getstatic`/`putstatic`.** Con el objeto mío, `apkc dex` cortó con
`a field that was never interned (Ljava/lang/System;.out Ljava/io/PrintStream;)`: `collectCode` seguía
`getfield`/`putfield` e ignoraba los estáticos, así que un campo de **otra** clase nunca entraba en
`field_ids`. Los propios funcionaban por la pasada de `collectClasses`, que interna todos los campos de
la clase. Corregido en `dextabla.go` (mismo `internField`, cuatro nombres en el `case`).

**3. Cada `putstatic` se llevaba un temporal.** Con el `println` del banco, el traductor cortó con
`at pc 34, ldc: it needs more temporaries than the max stack javap measured`. Traza de `toma`/`suelta`
con el `println` de Go a la vista: `accesoEstatico` pedía un registro con `toma` **también al
escribir** y luego lo tiraba (`r = p.pop(in)`), así que cada `putstatic` dejaba un temporal asignado y
perdido; con dos en un método el marco se quedaba sin registros. Corregido: al escribir el registro sale
de la pila y no se pide ninguno. Es el hueco que el plan llamaba 5.25 («temporales en la unión de un
`if`»): la unión no tenía nada que ver.

**4. La sobrecarga se elegía por nombre y aridad.** Ejecutando la sonda: nuestro dex no imprimía nada y
`dexdump` enseñó que la llamada iba a `PrintStream.println:(Z)V` —la primera sobrecarga declarada—.
`javap -v` sobre nuestro `.class` lo confirmó: `Methodref java/io/PrintStream.println:(Z)V` donde javac
escribe `(Ljava/lang/String;)V`. `buscarMetodo` recorría por nombre y número de argumentos; ahora:
1) candidatas por nombre y aridad, deducidas de la jerarquía; 2) con una sola, esa (lo de siempre);
3) con varias, la que coincide **exactamente** con los descriptores de los argumentos; 4) si ninguna es
exacta, la **aplicable más específica** por la cadena de padres y las interfaces del jar (JVMS
15.12.2.5), que es lo que pide `Menu.add("…")` del fixture —`add(CharSequence)` y `add(int)`, y el
String solo cabe en la primera—; 5) si no hay una sola, **rechazo por nombre** con las candidatas y los
tipos a la vista. El `boxing`, los varargs, los arrays y el ensanchamiento de primitivos no están
medidos y caen en el rechazo. Los tipos de los argumentos salen del propio emisor (`e.exp` devuelve el
descriptor al bajar el valor; para las llamadas sin receptor dentro de un `static`, donde hay que
resolver antes de bajar nada, se estiman con `tipoEstimado`).

**Y una cosa que estaba rechazada y no hacía falta: llamar sin receptor desde un `static`.** Los tests
de arriba tropezaron con `llamar di sin receptor desde un metodo estatico no esta medido`; javac escribe
`invokestatic` contra la clase actual, que es la forma 35c ya medida, y ahora se emite así (y se sigue
rechazando si el método no es estático).

**Pruebas permanentes**: `apkc/javaemisor_sobrecarga_test.go` comprueba contra el `android.jar` que
`println(String)` e `println(int)` son las que se internan y que `(Z)V` no aparece, que entre
`h(CharSequence)` y `h(Object)` gana la primera con un String, y que sin candidata aplicable el mensaje
nombra las sobrecargas y los tipos. El banco, de punta a punta: nuestro dex imprime `total 7`/`total 9`
y sale con `rc 9`, **igual que la JVM y que los dos dex de `d8`**, y los tres pasan `dexdump
verificando` con `rc 0`. El dex de `d8` sobre nuestra `Ver` y sobre la de javac miden lo mismo (1404
bytes) pero **no son idénticos**: javac pliega `new StringBuilder(); append("…")` en el constructor
`StringBuilder(String)` —36 palabras y 4 registros— y nosotros escribimos las dos filas del §53 —37
palabras y 3 registros—; el banco lo informa en vez de fallar, porque la forma del código es una
diferencia aceptada y lo que se exige es que ejecute igual.

**Lo que costó del banco mismo**, para no repetirlo: `rish` duplica al terminal lo que imprime la orden
remota cuando la salida es un terminal, así que las capturas van por fichero en el aparato y se bajan en
`base64` (una tubería directa devolvía solo la última línea y el banco comparaba contra nada). Y una
trampa de proceso: el primer gate de este párrafo salió con **un problema** —«el proyecto generado no
compila en nativo»— porque se lanzó con un `apkc-gate` **viejo**, de antes de la resolución nueva;
reconstruidos los dos binarios, el mismo gate queda en orden. Los apks de la pestaña cambian de tamaño
con el código emitido: de 9725 y 10285 bytes a **9720** y **10280**.

Suite: **219 PASS + 2 SKIP** nativa (tres nuevas: dos de sobrecarga y la del rechazo sin candidata) y
**194 PASS + 27 SKIP** en el hospedador, donde las tres nuevas se saltan si no hay `android.jar`
montado; el gate de `studio/e2e.sh`, en orden.

## 63. Los bucles: el `goto` de vuelta medido, y el `.class` del `while` idéntico al de javac

Lo que el plan tenía como el ítem grande de control de flujo (5.10). Medido primero, como siempre:
fuente `while (i < n) { total = total + i; i = i + 1; }` con `static int total`, compilado por javac 21
y dexado por `d8 --release` (fichero de 1380 bytes, `dexdump -d` en el aparato):

```
registers 3, ins 1, outs 0, 12 palabras
0000: const/4 v0, #int 0            ← el local `i`, que `d8` deja en v0
0001: if-ge v0, v2, +000a           ← la condición invertida; v2 es el argumento `n`
0003: sget v1, total ; 0005: add-int/2addr v1, v0 ; 0006: sput v1, total
0008: add-int/lit8 v0, v0, #int 1   ← el paso, plegado a literal
000a: goto 0001                     ← **`28f7`: opcode 0x28 y desplazamiento -9 en el byte alto**
000b: return-void
```

De ahí salen tres cosas. **La rama hacia atrás usa el `goto` 10t con el desplazamiento en el byte alto y
con signo** (el `28f7` de arriba), y la salida un 22t con el desplazamiento en la unidad siguiente; el
traductor ya tenía las dos formas, solo que rechazaba `rel < 0` «porque el reparto de registros en el
destino no está medido». **No hace falta ningún análisis de viveza**: los locales van clavados a su
registro (`guardaLocal`) y en la cabecera del bucle la pila JVM está vacía —lo exige el verificador para
los dos caminos—, así que la vuelta encuentra los mismos registros que dejó la primera pasada. Y **el
cuerpo nuestro es el mismo que el de `d8`** salvo un plegado: nosotros escribimos `const/4 v1, #1` +
`add-int/2addr v0, v1` donde él escribe `add-int/lit8 v0, v0, #1` (la diferencia de forma del §21 y el
§53, que no se toca aquí). El `goto` sale **`28f7` idéntico**.

**El emisor ya escribe el bucle.** `while (cond) cuerpo` se analiza a un nodo propio y se emite con la
forma de javac: `cabecera: <condición invertida> if_… fin ; cuerpo ; goto cabecera ; fin:`. `for
(init; cond; paso)` **se desdobla en el analizador** como `init; while (cond) { cuerpo; paso }`, que es
exactamente lo que escribe javac: el `.class` de nuestro `for` tiene **el mismo `Code` byte a byte** que
el suyo (23 bytes, `iconst_0 1:istore_1 2:iload_1 3:iload_0 4:if_icmpge 22 … 19:goto 2 22:return`), y el
`while` también. El `goto` de la vuelta se resuelve con una función nueva, `resuelveAtras`, porque
`cierra` solo parchea al cerrar y la cabecera se cerró antes del cuerpo; el desplazamiento se comprueba
en 16 bits con signo y lo que no cabe se rechaza por nombre (`goto_w` no está medido).

**Oráculos.** El doble, con la dosis de siempre: `TestElBucleSeDexaIgualQueElDeJavac` compila los dos
fuentes con este fork y exige que el dex sea **byte a byte** el de las clases de javac para el mismo
fuente (dos fixtures nuevos en base64), y además comprueba la forma: `if-ge` (0x35) con +10, el `goto`
`0x28` con **-9**, y `return-void` al final. `TestLaRamaHaciaAtrasSeEscribeComoD8` fija la vuelta en el
traductor (cuerpo sintético, `goto` -3 palabras a la cabecera). `TestDoYTtrySiguenRechazados` deja
escrita la frontera: `do` y `try` siguen saliendo por su nombre, y `i++` también (se nombró en
el portero en vez de dejar el mensaje genérico). *(`switch` entró después, en el §64.)*

**Ejecutado.** El dex de javac's `Bucle` traducido por nosotros imprime `10` y sale con `rc 10` en
`app_process`; el `.class` del `while` escrito por nosotros, dexado por nosotros, también (`10`, `rc 10`),
y el del `for` igual. Y el banco (`banco/banco.sh`) lleva ahora un bucle: `Ver.suma(3)` con su `while`,
de modo que los cuatro caminos imprimen `total 7`, `total 9` y `total 42` y salen con `rc 42` — la JVM,
nuestro dex de punta a punta, `d8` sobre javac y `d8` sobre nuestra `Ver`, los tres dex con `dexdump
verificando rc 0`.

**Frontera, con motivo**: no hay `do-while`, ni `break`/`continue`, ni bucles con valores vivos
distintos de un local, ni `goto/16`; *(y `i++`, que se nombraba aquí, entró en el §67)*; y el `+` de
cadenas en un fuente compilado por **javac 21** sale
como `invokedynamic` y el dexer lo rechaza (plan 5.22), que es la razón de que el `Arranque` del banco
imprima enteros y no cadenas.

Suite: **221 PASS + 2 SKIP** nativa y **194 PASS + 29 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.

## 64. El `switch`: `packed-switch` y `sparse-switch` medidos, y el `<clinit>` que ART exigía

El dexer ya traduce `switch`, con las dos formas y sus payloads medidos en el aparato. Lo que se midió
primero, con javac 21 y `d8`:

- **El `.class`**: `tableswitch` (opcode `0xaa`) con relleno hasta el siguiente múltiplo de cuatro
  **contado desde el principio del `Code`** —la tabla de la sonda está en el pc 1 y sus operandos empiezan
  en el 4, o sea `00 00 00` de relleno— y `lookupswitch` (`0xab`) con la misma regla. Los dos llevan
  `default` primero y, después, `low`/`high` y los destinos, o el número de pares y los pares
  (clave, destino). Cada destino es un desplazamiento **desde el opcode**, con signo.
- **El dex de `d8`**: la tabla densa sale como `packed-switch v0, +0x30` (opcode `0x2b`) y el payload
  —`ident 0x0100`, tamaño, `first_key`, destinos— **al final del cuerpo**, alineado a cuatro bytes y
  detrás de todos los bloques; 20 unidades para ocho claves. La dispersa pequeña **no** sale como
  `sparse-switch`: en `--release` `d8` la **rebaja a una cadena de `if-eq`** (con los cuerpos
  reordenados y `return` en vez de `goto`), y hay que ir a `--debug` para ver `sparse-switch v0, +0x20`
  (`0x2c`) con `ident 0x0200`, tamaño y 26 unidades para seis claves.

**Lo que el formato no dice y el volcado sí: en el payload disperso van primero TODAS las claves y
después TODOS los destinos**, no pares intercalados (26 = 1 + 1 + 6 + 6). Intercalarlos pasó el
`dexdump` —que verifica— y lo cazó ART en ejecución: `VerifyError: Verifier rejected class: invalid
target offset 1000 (end 0x24) in switch payload`, o sea leyó una clave como destino. Y **en DEX, si
ninguna clave casa se sigue por la instrucción siguiente**, así que el `default` del `.class` (un destino
explícito) se escribe como un `goto` de una unidad detrás del switch en vez de reordenar bloques como
hace `d8`.

**Y un defecto que solo ART veía**: nuestro `<clinit>` salía con `access 0x0008 (STATIC)` y el
verificador avisó —«`<clinit>` didn't have expected constructor access flag», «not marked as
constructor»— antes de cortar la clase. Lleva `ACC_CONSTRUCTOR` como los `<init>` (0x10000), y corregido
el fichero corre. Va con `TestElClinitLlevaConstructor`.

**Ejecutado, tres caminos iguales**: `Ramas` (densa de ocho casos y dispersa de tres) llamada desde un
`Arranque` que imprime cada `total`. Nuestro dex en `app_process` da `100`, `105`, `999`, `3`, `9` y
`rc 240`; la JVM con las mismas clases, lo mismo; y el dex de `d8` sobre nuestras clases, lo mismo. Los
tres pasan `dexdump` verificando con `rc 0`. Pruebas permanentes: `TestElSwitchSeDexaComoPackedYSparse`
(las dos formas, los dos idents, las tres claves ascendentes y los destinos detrás de ellas, más el
layout del fichero) y la del `<clinit>`.

**La otra mitad: el `switch` del fuente.** El analizador lee `switch (selector) { case k: … break;
default: … }` con claves literales enteras y sin claves repetidas, y **cada brazo tiene que terminar en
`break` o en `return`** (la caída al siguiente se rechaza por nombre; el último brazo puede acabar sin
`break`). El emisor escribe `tableswitch` o `lookupswitch` con la forma medida: el relleno hasta el
múltiplo de cuatro **contado desde el principio del `Code`** y cada destino un entero de 32 bits con
signo desde el opcode. La elección de forma es de este emisor —`tableswitch` cuando el rango no pasa del
doble de los casos—: el criterio de javac 21 se sondeó y **no es una desigualdad sencilla** (con dos
casos y rango 4 elige `lookupswitch`, con cuatro casos y rango 10 elige `tableswitch`, con seis y rango
15 también `tableswitch`), así que se copia el formato, no la heurística. El resultado, con la switch
densa de ocho casos y la dispersa de tres: el `.class` emitido tiene **el mismo `Code` que el de javac**
(mismos `tableswitch` en el pc 1, mismos destinos 48…112, mismo `default`) y el dex de nuestras clases es
**byte a byte** el de las clases de javac (1432 bytes, `cmp` mudo): `TestElSwitchSeEscribeComoJavac`.
Ejecutado, los tres caminos coinciden: `100`, `105`, `999`, `2`, `9`, `rc 250` en ART con nuestro dex,
en la JVM y en el dex de `d8` sobre nuestras clases, con `dexdump rc 0`. Y el banco lleva ahora un
`switch` además del bucle: los cuatro caminos imprimen `total 7|total 9|total 102` y salen con `rc 102`.

**Frontera**: `do-while`, `break`/`continue` fuera de un `switch`, claves negativas o que no sean
literales, la caída de un caso al siguiente y un `default` que no esté al final (el emisor lo coloca el
último) se rechazan por nombre.

Efecto colateral medido en el gate: el bit nuevo del `<clinit>` cambia los apks de la pestaña de **9720 a
9727** y de **10280 a 10287** bytes (el atributo de acceso del método pasa de un byte a tres en el
`class_data`), y siguen saliendo **igual entrada por entrada** al nativo.

Suite: **225 PASS + 2 SKIP** nativa y **196 PASS + 31 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.





## 65. El `StackMapTable`: la JVM real ya verifica nuestro `.class`, y los marcos coinciden con los de javac

El ítem 5.23 del plan. Hasta aquí todo el camino de ejecución pasaba por `d8` o por ART, y **ninguno de
los dos mira el atributo**: `d8` acepta un `.class` sin marcos y ART nunca ve un `.class`. La JVM sí, y
en el §61 lo dijo con un `Expecting a stackmap frame at branch target` sobre nuestro `.class` en
`java -Xverify:all`. Es el único oráculo que quedaba sin satisfacer.

**Qué escribe javac, medido con `javap -v`** sobre una sonda con siete métodos (`Sonda.java`, compilada
con `javac -g`):

- Un cuerpo recto (`int b = a + 1; return b;`) **no lleva el atributo**.
- Un `if` sin `else` lleva **un** marco, el del destino (pc 12): `frame_type 252 /* append */`,
  `offset_delta 12`, `locals [int]` — el local que asigna el `then` pasa a estar asignado en la unión.
- Un `if/else` lleva dos: `same` en el pc 9 (la rama `else`, que no toca el local) y `append` en el 12
  (`offset_delta 2`), porque por los dos caminos el local quedó asignado.
- El bucle lleva dos: `append` en la cabecera (`offset_delta 4`, los dos locales) y `same` en la salida
  (`offset_delta 14` = 19 − 4 − 1).
- El `switch` lleva uno por destino (`same` en 28, 34 y 40) y uno tras los `goto` (en 43, `append`).
- Con referencias, el marco del cruce dice `class java/lang/String`, no un comodín.

O sea: **marco en cada destino de salto y en la instrucción que sigue a un salto incondicional o a un
`return`**, que es "cada comienzo de bloque básico salvo el primero". Y el `offset_delta` del primer
marco es el pc, no el pc menos algo.

**La forma que rompe la intuición, medida aparte** (`Cab.java`): un bucle cuya cabecera está en el pc 0.
javac escribe un marco **en el pc 0** (`offset_delta 0`, `same`) aunque ese pc ya tenga el marco
implícito, y el local declarado dentro del cuerpo (`int x = 1;`) aparece como `top`: la vuelta del bucle
llega con un entero al hueco que la entrada ve sin asignar. `java -Xverify:all` sobre esa clase no dice
nada (y el programa ni siquiera tiene `main`: la JVM verifica antes de buscarlo), así que **`top` gana en
la unión** y un hueco `top` en el marco no prohíbe que llegue un valor: prohíbe usarlo.

**Lo que hace este emisor** (`apkc/jvmframes.go`). Un intérprete abstracto sobre el cuerpo **ya
parcheado** (las posiciones dependen de los anchos definitivos de los saltos): camina las instrucciones
con la misma tabla del dexer, calcula el estado de cada comienzo de bloque con una cola de trabajo y une
los estados que llegan al mismo pc (`top` gana; dos referencias distintas se unen en `Object`; un entero
y una referencia no se unen y se rechaza por nombre). Los tipos de campos y métodos salen del **pool que
el emisor acaba de escribir**, no de una segunda copia. Cada marco se escribe como `full_frame` —el JVMS
lo permite en cualquier posición— y los `top` del final se recortan; no se copia la compresión de javac
(`same`/`append`) porque lo que el verificador compara es el contenido, no la forma, y una sola forma es
menos superficie para equivocarse. Sin ramas no hay atributo, como en javac. El analizador comprueba
además que el alto de pila que calcula no pase del `max_stack` declarado: si pasara, el `.class` no
verificaría.

**Oráculos.** (1) El barato: `TestLosMarcosSonLosDeJavac` decodifica los dos atributos —el nuestro y el
del fixture de javac— a un estado por pc y exige que **digan lo mismo**: mismos pcs, mismos locales y
misma pila, en el `while`, en las dos formas del `switch`, y en `Cab.f` y `Cab.g` (la cabecera en el pc 0
con y sin local previo). `TestSinRamasNoHayStackMapTable` fija la ausencia en el constructor recto.
(2) El de verdad: `banco/banco.sh` paso 6 ejecuta nuestra `Ver` con el `Arranque` de javac bajo
`java -Xverify:all` y compara **texto y código de salida** con la JVM. Antes decía "VerifyError por
StackMapTable faltante"; ahora dice `jvm con nuestra clase emitida (Xverify:all): rc=102 y salida igual a
la JVM  ok`. Ese `Ver` lleva el `if`, el `while` y el `switch` del banco, o sea marcos con uniones de
enteros y una referencia (`System.out`).

Dos defectos propios salieron escribiéndolo, no del oráculo: el intérprete dimensionaba los locales con
los argumentos del descriptor y rechazaba cualquier local declarado después (`se escribe el hueco 1 y el
marco tiene 1`) —se rellena con `top` hasta el `max_locals`—, y los casos no ramificados del `switch`
caían al `return` de error que sigue al `switch` en vez de continuar el flujo.

No cambia ni un byte de lo que ya estaba medido: el dexer ignora el atributo, así que el dex y los apks
salen iguales. Suite: **227 PASS + 2 SKIP** nativa y **196 PASS + 33 SKIP** en el hospedador (los dos
tests nuevos necesitan el `android.jar` y allí se saltan); gate de `studio/e2e.sh` en orden, con los
apks de siempre (10014, 9727 y 10287 bytes).

**Frontera**: `uninitializedThis`/`uninitialized` no se emiten en ningún marco (los `new` van pegados a
su `<init>` y ningún comienzo de bloque cae en medio, y los constructores llaman a `super` antes de
cualquier rama); un `long`/`double` en un descriptor lo rechaza el propio analizador por nombre (5.8); y
arrays siguen fuera (5.11).

## 66. Arreglos: el `main(String[])` compilado y dexado solo por nosotros corre en ART

El ítem 5.11, el que faltaba para una lista de verdad y para `main(String[])`. Se midió como siempre:
una sonda `Arreglos` con alta (`new int[n]`, `new String[n]`), `.length`, lectura y escritura de un
elemento, y dos métodos con bucle; compilada con `javac -g`, dexada por `d8 --release` y desarmada con
`dexdump -d` del aparato.

**La forma del `.class`**, de `javap -c`: `iconst_3; newarray int`, `iload_0; anewarray #7 // class
java/lang/String`, `aload_0; arraylength`, `aload_0; iload_1; iaload`, `aload_0; iload_1; iload_2;
iastore`, y lo mismo con `aaload`/`aastore` para las referencias. **La forma del dex**, del volcado:

```
array-length v0, v3     2130      12x: destino en el nibble bajo, fuente en el alto
new-array v0, v0, [I    2300 0500 22c: registros en el byte alto, type_id en la unidad siguiente
aget v2, v3, v1         4402 0301 23x: op|AA, después BB|CC<<8
aput v2, v0, v1         4b02 0001 23x, con el valor primero
aget-object · aput-object   0x46 · 0x4d
```

con el tipo `[I` (o `[Ljava/lang/String;`) en `type_ids`. En el `.class` el `anewarray` nombra la clase
del **elemento** (`java/lang/String`), no la del arreglo.

**El emisor ya escribe arreglos.** El analizador acepta `T[]` en declaraciones, parámetros y vueltas
(el tipo se guarda como el texto del fuente con sus corchetes), `new T[n]`, `v[i]` —de lectura y como
destino de una asignación— y `v.length` (`arraylength`). `descriptorDe` escribe `[` delante del
descriptor del elemento, así que `int[]` es `[I`, `String[]` es `[Ljava/lang/String;` y `int[][]` sería
`[[I`. El inicializador `new T[] {…}` y más de una dimensión se rechazan por nombre (piden
`filled-new-array`, que todavía no se ha medido). Trampa que costó un defecto: `pareceDeclaracion`
miraba «un `[` detrás del nombre» y confundía `v[i] = x` con una declaración; ahora solo son
declaración los corchetes **vacíos** seguidos de un nombre.

**El traductor.** `newarray` (primitivo por atype), `anewarray` (clase del elemento), `arraylength`,
`iaload`/`aaload`, `iastore`/`aastore`, con las formas de arriba, y `new-array`/`anewarray` internan su
tipo de arreglo en `collectCode`. El oráculo doble: nuestro dex de nuestra `Arreglos` es **byte a byte**
el de las clases de javac del mismo fuente (diez métodos, `cmp` mudo), o sea que el `Code` emitido es el
de javac y el traductor los baja igual.

**Ejecutar encontró dos defectos que ningún `cmp` veía, y uno era grave.** El primero, del dexer, al
verificar con `dexdump`: una tabla vacía escribía su desplazamiento en la cabecera y ART corta con
`Offset(228) should be zero when size is zero for field-ids`; una clase sin campos jamás lo había
tocado. El segundo lo destapó ART al correr `v[i] = i + 1` y sacar `3` donde la JVM saca `5`: el `iadd`
se escribía como `/2addr` sobre el registro de la **izquierda**, que era `i`… y `i` también estaba en
la pila como índice, así que el índice quedaba pisado y el `aput` escribía `v[i] = i`. Medido en `d8`
con `Vivo`/`Vivo2`, la cuenta entera tiene **tres formas** según quién muera:

```
v[i] = i + 1   d800 0201   add-int/lit8 v0, v2, #1     (22b, resultado en un registro nuevo)
v[i] = i + j   b012        add-int/2addr v2, v1        (la izquierda vive; conmuta, va a la derecha)
v[i] = i / j   9302 0102   div-int v2, v1, v2          (no conmuta: tres registros, 23x)
v[i] = i - j   9102 0102   sub-int v2, v1, v2
v[i] = i % j   9402 0102   rem-int v2, v1, v2
```

y la familia 23x es `0x90` suma, `0x91` resta, `0x92` producto, `0x93` división, `0x94` residuo.
`cuentaEntera` ahora mira la pila simulada: si la izquierda no vive, `/2addr` sobre ella (lo de
siempre); si vive y la operación conmuta y la derecha no vive, `/2addr` sobre la derecha; y si no,
la forma de tres registros en un temporal. `TestLaCuentaNoPisaUnOperandoVivo` fija las palabras de los
dos casos (`1210 b020 004b 0201 000e` y `0091 0302 004b 0201 000e`).

**`move-result` de un entero (0x0a), medido de paso.** El camino "todo nuestro" del banco llamó a un
método que devuelve `I` y el traductor solo tenía `move-result-object`: medido en `Mr.h`, `d8` escribe
`0a00` —la 11x gemela—, y ahora se emite para los cinco enteros; `long`/`double` siguen rechazados
por nombre.

**El hecho que se buscaba: un `main(String[])` que compila y dexa este fork, y corre en el aparato.**
Fuente con `int[] v = new int[4]`, dos bucles, `v.length`, `v[i] = i + 1`, una copia a otro arreglo y
cuatro `System.out.println`. `apkc compile` (34 ms) y `apkc dex` (1076 bytes) y en `app_process`:
`4`, `5`, `4`, `1`, `rc 0`, `dexdump` verificando; y la JVM con `-Xverify:all` sobre las mismas clases
dice lo mismo. Antes de arreglar el alias, ART imprimía `4`, `3`, `4`, `0`: el fallo no da
`VerifyError`, da un número. El banco tiene ahora un quinto camino con esa forma —las dos fuentes
compiladas por nosotros, sin javac ni d8—: `nuestro dex con nuestro main (sin javac ni d8): rc=102 y
salida igual a la JVM  ok`, con la salida `total 7|total 9|total 102|10` de los cinco caminos.

**Los marcos, con arreglos.** El tipo de verificación de un arreglo es `ITEM_Object` con una `Class`
cuyo nombre es el **descriptor** (`class "[I"`, medido en `Arreglos.copia`), y así se escribe; la unión
de dos arreglos distintos da `Object`, que es lo que el verificador acepta. El decodificador del test
tuvo que dejar de usar `internalName` (que convierte `[I` en el tipo null) y leer el nombre del pool
tal cual. `TestLosMarcosDeUnArregloSonLosDeJavac` compara `suma` y `copia` expandidos contra javac.

Suite: **231 PASS + 2 SKIP** nativa y **196 PASS + 37 SKIP** en el hospedador (los cuatro tests nuevos
piden el `android.jar`); gate de `studio/e2e.sh` en orden con los apks de siempre (10014, 9727 y 10287
bytes). El banco: cinco caminos y `jvm con nuestra clase emitida (Xverify:all)` en orden.

**Frontera**: el inicializador `new T[] {…}`, más de una dimensión, los arreglos de `char`/`boolean`
desde el fuente (el traductor sí acepta el atype; el emisor del fuente solo escribe `int`), pasar un
arreglo a un parámetro `Object` (`asignable` no conoce la jerarquía de arreglos; se rechaza por
nombre) y `arraycopy` no están medidos.

## 67. `i++` y `--i`: `iinc` en el `.class`, `add-int/lit8` en el dex

El ítem 5.1, el que se nombraba en el portero desde el §63. Medido con `Inc`, un fuente con `i++` e
`i--` dentro de un bucle, un `++` sobre un campo estático y otro sobre uno de instancia:

```
javac                                   d8 (dexdump -d)
i++    iinc 1, 1                        d800 0001   add-int/lit8 v0, v0, #1
i--    iinc 1, -1                       d800 00ff   add-int/lit8 v0, v0, #-1
total++  getstatic; iconst_1; iadd;     sget; add-int/lit8; sput
         putstatic
propio++ aload_0; dup; getfield;        iget v0, v1; add-int/lit8 v0, v0, #1;
         iconst_1; iadd; putfield       iput v0, v1     (el `dup` desaparece)
```

`iinc` (0x84) lleva el índice del local y una constante de un byte con signo; en DEX es la 22b
`add-int/lit8 vA, vB, #+C` (0xd8), con el destino en el byte alto de la primera unidad y el fuente con
la constante al lado. La forma 22b se conocía del plegado `i + 1` del §63; ahora está medida también
para el decremento.

**El emisor.** El analizador acepta `++` y `--` como tokens propios (antes se leían dos `+`), en
prefijo y en sufijo, y como paso de un `for`. Como sentencia: si el destino es un local se escribe
`iinc` —la forma de javac, no un `iload`/`iconst`/`iadd`/`istore`, que daría el mismo resultado con un
`.class` distinto—; si es un campo se arma `destino = destino ± 1`, que es `getstatic; iconst_1;
iadd/isub; putstatic` o el `aload_0; dup; getfield; …; putfield` de javac. **Como valor** (`x = i++`)
no está medido y se rechaza por nombre: javac tiene que conservar el valor viejo y eso es otra forma.
`+=` y `-=` siguen rechazados.

**Oráculo doble**: el dex de nuestra `Inc` es **byte a byte** el de las clases de javac (sin contar el
`dup`, que en el traductor es una copia de la pila y no una instrucción: la lectura-cuenta-escritura
del campo de instancia sale igual). `TestElIncrementoSeDexaIgualQueElDeJavac` lo fija.

**Ejecutado**: el banco usa ahora `i++` en `Ver.suma` y un `for (int j = 0; …; j++)` en
`Ver.sumaArreglo`, y los cinco caminos siguen dando `total 7|total 9|total 102|10` con `rc 102` y
`dexdump` verificando —incluido el camino "todo nuestro", con las dos fuentes compiladas y dexadas por
este fork—; la JVM con `-Xverify:all` sobre nuestras clases también.

Suite: **232 PASS + 2 SKIP** nativa y **196 PASS + 38 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden con los apks de siempre (10014, 9727 y 10287 bytes).

**Frontera**: `++`/`--` como valor, `+=`/`-=`, y `++` sobre algo que no sea una variable.

## 68. `+=` y `-=`: `iinc` si el literal cabe, lectura-cuenta-escritura si no

La mitad que faltaba del 5.1. Medido con `Mas`, que suma y resta sobre un campo y sobre un local:

```
total += 2     getstatic; iconst_2; iadd; putstatic
i += 2         iinc 0, 2            ← javac pliega el literal en el propio iinc
i -= 3         iinc 0, -3
i += 7         iinc 1, 7            (sobre un local ya cargado)
i *= 3         iload_0; iconst_3; imul; istore_0   ← el producto no se pliega
```

O sea: javac usa `iinc` cuando el operador es `+` o `-` **y** el literal cabe en un byte; para `*=`,
`/=` y para un delta que no cabe, compone la lectura-cuenta-escritura. El analizador reescribe
`x += lit` como el mismo nodo de incremento del §67 (que ya sabe que en un local es `iinc` y en un
campo `destino = destino ± lit`), y deja `x op= expr` como `x = x op expr` cuando el valor no es un
literal entero. `>>=`, `<<=`, `&=` y los demás siguen fuera: el tokenizador ni los arma.

Oráculo doble: el dex de nuestra `Mas` es **byte a byte** el de javac
(`TestElMasIgualSeDexaComoJavac`). El banco imprime además `Ver.alMenosUna(3)` con su `do-while`, así
que los cinco caminos dan `total 7|total 9|total 102|10|3` y `rc 102`.

## 69. El `do-while`: el cuerpo primero y la condición directa

Medido con `Doo`. javac escribe el cuerpo y **después** la condición directa saltando hacia atrás a la
primera instrucción del cuerpo:

```
0: iconst_0 ; 1: istore_1          int i = 0
2: getstatic total ; 5: iload_1 ; 6: iadd ; 7: putstatic
10: iinc 1, 1
13: iload_1 ; 14: iload_0 ; 15: if_icmplt 2   ← `i < n`: salta ATRÁS si se cumple
18: return
```

Es el espejo del `while`, que invierte la condición para salir. `condicionBranca` escribe la forma que
salta cuando la condición **no** se cumple, así que el emisor le pasa la comparación negada
(`i < n` → `i >= n`, cuyo salto es `if_icmplt`). El `StackMapTable` gana un marco en la cabecera: en
`Doo.hasta` es `append` en el pc 2, el primer renglón del cuerpo.

Dos detalles de la `LineNumberTable`, medidos de javac: **no hay fila para el `do`** —la cabecera
lleva la línea del primer renglón del cuerpo— y la condición lleva la línea del `while`
(`line 8: 13`). Con eso, el dex de nuestro `Doo` es **byte a byte** el de javac
(`TestElDoWhileSeDexaComoJavac`); con el `do` anotado de más, la tabla de líneas se apartaba.

Frontera: `do` con una condición que no sea una comparación (`while (true)`) se rechaza por nombre.

Suite tras los dos: **234 PASS + 2 SKIP** nativa y **196 PASS + 40 SKIP** en el hospedador; gate de
`studio/e2e.sh` en orden con los apks de siempre.

## 70. `break` y `continue`: el destino de cada uno, medido

Medido con `Corte`, que corta un `while` con `break` y salta con `continue` en un `for` y en un
`do-while`:

```
while:      break    → goto 24   (después del bucle)
for:        continue → goto 21   (¡el `iinc` del PASO, no la condición!)
do-while:   continue → goto 19   (la condición)
```

Es lo que no se podía componer con el desdoblamiento del `for` en `init; while (cond) { cuerpo; paso }`
del §63: ahí el `continue` habría saltado a la condición, y javac lo manda al paso. Por eso el
analizador tiene ahora un nodo propio, `jpara{cuerpo, cond, paso}`, cuyo `paso` vive **fuera** del
cuerpo: el emisor lo escribe detrás (el `.class` sale igual que con el desdoblamiento, y el dex de
`Para` sigue byte a byte el de javac) y apila el destino de `continue` en la etiqueta que abre justo
antes del paso.

**El emisor.** Dos pilas de contextos: `quiebres` (el fin de cada bucle y de cada `switch`, que es a
donde va `break`) y `continuar` (la cabecera del `while`, la condición del `do-while` y el paso del
`for`). `break` dentro de un brazo de `switch` —también anidado en un `if`— sale del switch porque el
switch apila su fin. Un `continue` hacia una etiqueta ya cerrada (la cabecera del `while`) se parchea
en el acto con `resuelveAtras`; el resto son saltos hacia adelante que `cierra` completa. Fuera de
sitio, los dos se rechazan por nombre.

**Un defecto de camino encontrado al tocarlo**: `anonimasDe`, que decide qué clases anónimas se
emiten, no recorría `if`, `while` ni `switch` —solo bloques, declaraciones y asignaciones—, así que un
`new OnClickListener() { … }` dentro de un `if` no se compilaba. Ahora recorre las cuatro formas.

Oráculo doble: el dex de nuestra `Corte` es **byte a byte** el de javac
(`TestElCorteSeDexaComoJavac`), y `TestRompeYContinuaFueraDeSitio` fija los dos rechazos. El banco
lleva `primerIgual(5)` (un `while` con `break`, 2) y `sinDos(4)` (un `for` con `continue`, 4): los
cinco caminos dan `total 7|total 9|total 102|10|3|2|4` con `rc 102`.

Suite: **236 PASS + 2 SKIP** nativa y **196 PASS + 42 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.

## 71. `try/catch` y `throw`: la tabla de excepciones, y el defecto de locales que destapó ART

El ítem 5.9, el último grande de control de flujo. Medido con `T`, que tiene un `try` que devuelve
desde dentro, un `throw` y un `try` con catch que se ejecuta.

**El `.class`**, de `javap -c -v`:

```
lee:     0 aload_0 ; 1 iload_1 ; 2 iaload ; 3 ireturn
         4 astore_2 ; 5 iconst_m1 ; 6 ireturn
         Exception table: from 0 to 3, target 4, Class java/lang/ArrayIndexOutOfBoundsException
atrapa:  ... 8 goto 14 ; 11 astore_2 ; 12 iconst_2 ; 13 istore_1 ; 14 iload_1 ; 15 ireturn
         Exception table: from 2 to 8, target 11
lanza:   ... 13 athrow ; 14 return
```

Cuatro reglas que dijo el oráculo, no el manual: el `to` de la tabla es **exclusivo**; el handler
empieza con el `astore` de la excepción (el único valor que el verificador deja en la pila); el rango
**excluye un `return`/`goto` final** —no pueden lanzar— pero **incluye el `athrow`** (`T.f` mide
`0..10` con el `athrow` en 9, y `g` mide `0..1` con el `ireturn` en 1); y `catch_type` es una `Class`
con el nombre **interno**, no el descriptor (con `Ljava/lang/X;` javap imprimía
`Class "Ljava/lang/X;"`).

**El dex**, de `dexdump -d`: `try_item` (start, insn_count, handler_off) detrás de las palabras, con un
relleno de una unidad si el cuerpo tiene un número impar, y la `encoded_catch_handler_list` detrás
(`uleb` de tamaño, `sleb` de cuántos tipos —negativo si hay catch-all—, pares tipo/destino). `throw`
es `0x27` (`2701`) y el handler que usa la excepción empieza con `move-exception` (`0x0d`); `d8` lo
omite cuando nadie la usa (en `atrapa` su `astore` desaparece y el handler arranca en el `const/4`).

**El defecto que encontró ART, y era de fondo.** El primer intento corrió y ART cortó con
`VerifyError: int T.atrapa(int): [0x8] register v2 has type Undefined`. La causa no era el try: era el
modelo de locales del traductor. `istore` **fundía** el local con el registro del valor (sin
instrucción), y con dos asignaciones en caminos distintos —`r = 1` en el try y `r = 2` en el catch—
la segunda fusión reescribía el mapa y una de las dos rutas quedaba con un registro sin definir.
Medido en `d8` con `Mv2` y `Mv3`: cuando el valor viene de un local o de un argumento vivo, `d8`
**mueve** (`move v0, v1` = `0110`, `move-object v0, v3` = `0730`). El traductor ahora funde solo la
**primera** escritura de un local que no es argumento y cuando el valor viene de un temporal; el resto
sueltan `move`/`move-object` al registro del local. Los nueve cuerpos del fixture no cambian (sus
locales se escriben una vez o en el mismo registro).

**Los marcos**: el handler es un comienzo de bloque con la excepción encima de la pila y los locales
unidos sobre todo el rango protegido (`same_locals_1_stack_item` en `lee`, `full_frame` en `atrapa`).
El analizador añade esa arista por cada instrucción protegida.

**Ejecutado**: nuestro dex de `T` en `app_process` da `9`, `-1`, `2`, `1` con `rc 0` —lo mismo que la
JVM—, `dexdump` verifica `rc 0`; la JVM con `-Xverify:all` sobre nuestras clases también. El banco
lleva `Ver.leeSeguro` y los cinco caminos dan `total 7|total 9|total 102|10|3|2|4|7|-1` con `rc 102`.
`TestElTrySeDexaComoJavac` compara por método (registros, `tries` y palabras) y no el fichero entero:
la `LineNumberTable` de un `try` sigue la política de este emisor, y javac anota además el `goto` de
salida con la línea de la llave del catch.

De paso entró el **menos de un literal entero** (`return -1;` era `iconst_m1`): el analizador dobla el
signo y el resto de la negación sigue fuera.

Suite: **237 PASS + 2 SKIP** nativa y **196 PASS + 43 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.

**Frontera**: `finally`, más de un `catch` por `try`, `catch` de un tipo que no sea clase y la
negación de algo que no sea un literal entero se rechazan por nombre.

## 72. `private`/`protected` y los `access$NNN` de javac, por los dos dexers

**Los flags, con `javap -v`** sobre `Dentro` (`private void oculto()`, `protected int medio()`): un
método privado es `0x0002` y uno protegido `0x0004`, los mismos bits que §49 midió en un campo; no hay
nada más en el `method_info`. Dentro de la propia clase, `oculto()` sale como
`aload_0; invokespecial #13 // Method oculto:()V` —javac llama a un privado con `invokespecial`, no con
`invokevirtual`— y `medio()` como `invokevirtual`.

**La anónima no toca el privado: javac le fabrica métodos.** Sobre `Fuera` (un `private int secreto`,
un `private void oculto()` y una anónima que hace `secreto = secreto + 2; oculto();`), `javap -p` de la
clase exterior añade tres métodos **al final**, después de los declarados y antes del `<clinit>`:
`static int access$002(p.Fuera, int)`, `static int access$000(p.Fuera)` y `static void
access$100(p.Fuera)`, los tres con flags `0x1008` (`ACC_STATIC|ACC_SYNTHETIC`). El nombre es
`access$` + **posición del miembro** × 100 + **sentido** (`0` leer o llamar, `2` escribir), con tres
dígitos: `access$000`/`access$002` son el primer miembro, `access$100` el segundo. La posición es una
sola secuencia para campos y métodos y se asigna por **orden de primera creación**, no de declaración:
con `b` tocado antes que `a`, `b` es el 0 y `a` el 1. Una escritura crea su accesor **antes** de la
lectura del lado derecho (`secreto = secreto + 2` deja `002` antes de `000`), y las anónimas se
procesan antes que las clases anidadas con nombre (con `Interior` declarada antes del método que crea
la anónima, los dos accesores de la anónima salen primero).

**Los cuerpos, todos de `javap -c -p`** y con una sola fila de `LineNumberTable` en la línea de la
**declaración de la clase** (no la del miembro):

| miembro | access$ | cuerpo |
| --- | --- | --- |
| campo de instancia, leer | `000` | `aload_0; getfield; ireturn` |
| campo de instancia, escribir | `002` | `aload_0; iload_1; dup_x1; putfield; ireturn` (devuelve el valor) |
| campo `String`, escribir | `202` | `aload_0; aload_1; dup_x1; putfield; areturn` |
| campo estático, leer | `300` | `getstatic; ireturn` |
| campo estático, escribir | `302` | `iload_0; dup; putstatic; ireturn` |
| método de instancia | `400` | `aload_0; invokespecial; return`/`ireturn` |
| método estático | `500` | `invokestatic; return` |

**Los sitios de llamada, de la anónima:** leer es `aload_0; getfield this$0; invokestatic
access$000:(LExterior;)I`; escribir carga la captura, deja el valor y acaba en
`invokestatic access$002; pop`; un método privado es `aload_0; getfield this$0; invokestatic
access$100` (sin `invokespecial` desde la anónima). Un método **no** privado del exterior se llama con
la captura y `invokevirtual`, y uno estático con `invokestatic` sin receptor.

**En el dex**, de `dexdump -d` sobre `d8`:
- `access$000` es `iget v0, v0` + `return v0` (d8 reutiliza el registro del argumento; nuestro
  asignador usa uno más, `iget v0, v1`, porque el argumento no se declara muerto).
- `access$002` es `5901 0400` (`iput v1, v0`) + `0f01` (`return v1`): el `dup_x1` no deja ninguna
  instrucción; las tres entradas de la pila comparten el registro del valor y `suelta` no lo recicla
  mientras otra entrada viva lo nombre.
- `access$100` es `7010 0f00 0000` (`invoke-direct {v0}`) —un privado de la propia clase es
  `invoke-direct`— y el estático `access$500` es `invoke-static`; el campo estático es `sget`
  (`6000 0800`).

Y el reparto del `class_data` cambió: **un método privado de instancia es un método directo** (junto a
constructores y estáticos). Sin eso `dexdump` corta el fichero con `Direct/virtual method
11(LQ;.lee) not in expected list 0`.

**Dos defectos que solo destapó ejecutar.** (1) `java -Xverify:all` rechazó el primer `.class` con
`ClassFormatError: Arguments can't fit into locals in class file Q`: el accesor escribía
`max_locals=0` aunque su descriptor pidiera argumentos; ahora `max_locals` es el número de huecos del
descriptor (el accesor siempre es `static`, aunque el miembro no lo sea). (2) `dexdump` rechazó el dex
por la lista directa del párrafo anterior.

**Ejecutado.** Sonda `Q`: un `private int n`, un `private static int total`, `sube()`, `suma(int)`,
`lee()` privados y una anónima que hace `n = n + 2; sube(); suma(3); total = total + lee();`.
- Nuestro `.class` en la JVM (`java -Xverify:all`): `3` y `6`, `rc 0`.
- Nuestro dex (compilador + dexer en Go): `dexdump rc 0`, `app_process` imprime `3` y `6`, `rc 0`.
- `d8 --release` sobre **nuestras** clases: `rc 0`, su dex también imprime `3` y `6` con `rc 0`.
- El banco lleva `Ver.privados` (campo de instancia, campo estático y tres métodos privados tocados
  desde una anónima; devuelve `10`) y los cinco caminos dan la misma salida, ahora
  `total 7|total 9|total 102|10|3|2|4|7|-1|10`, y el sexto —la `Ver` nuestra con el `Arranque` de
  javac bajo `-Xverify:all`— `rc=102 y salida igual a la JVM`. `d8` y los caminos de la JVM reciben
  también `Ver$1.class`: la anónima es un fichero aparte y sin él `d8` pide un *nest mate* y la JVM
  muere en `privados`.

**Pruebas permanentes**: `apkc/javaemisor_privada_test.go` compara campo a campo, método a método
(nombres, descriptores, flags y orden) y, para cada cuerpo, la sucesión de instrucciones **con el
miembro resuelto por nombre** contra `javac --release 8 -g` —comparar el `Code` crudo no vale porque
los índices del pool se ordenan distinto—; incluye las dos formas de la frontera.
`apkc/dexprivada_test.go` fija que privados y accesores están en la lista directa y las formas
`iget`/`iput`+`return`/`invoke-direct`/`invoke-static`/`sget`.

**Frontera**: una clase anidada `private` (javac le añade un constructor sintético `(Exterior$1)` y
una clase marcador `Exterior$1`) y una `protected` con nombre y sin `static` (pide la captura
`this$0`) se rechazan por nombre; `protected` en campos y métodos sí entra.

## 73. El inicializador `new T[] {…}`, y una clase llamada `B` que se leía como el primitivo

**La forma de javac**, de `javap -c -p` sobre `A` (`return new int[] {1, 2, 3};`, un `String[]`, un
local y un argumento de llamada): no hay `filled-new-array`; es el largo, el `newarray`/`anewarray` y
por cada elemento `dup`, indice, valor y `iastore`/`aastore`, con el arreglo vivo en la pila:

```
0: iconst_3
1: newarray       int
3: dup
4: iconst_0
5: iconst_1
6: iastore
7: dup
8: iconst_1
9: iconst_2
10: iastore
...
15: areturn
```

El analizador admite `new T[] {a, b, …}` (y `{}` y la coma final); el emisor escribe esa cadena y
rechaza por nombre `new T[]` sin inicializador, más de una dimensión y un elemento de otro tipo.
`d8 --release` sí lo compacta a `filled-new-array {v0, v1}, [I` + `move-result-object` (medido sobre
la sonda `B`), pero el `.class` que emitimos es el de javac y `d8` lo acepta.

**Un estático de tipo arreglo es `sget-object`/`sput-object`**, no `sget`/`sput`: medido en el dex de
`d8` sobre `static int[] v` (`6200 0000` para leer y `6900 0000` para escribir). El traductor solo
aceptaba `I` y `L…`; ahora acepta cualquier descriptor de referencia (`[` incluido).

**El defecto que destapó la sonda llamada `B`**: `descriptorOf` tenía un caso para los primitivos de
una letra y devolvía `B` tal cual; una clase del paquete por defecto llamada `B` (o `I`, `S`…) quedaba
en el dex con el `type_id` `"B"` y `dexdump` cortaba con `Invalid descriptor for class_idx: 'B'`. La
entrada `Class` del pool es siempre un **nombre interno**, nunca un descriptor: `descriptorDeClase`
envuelve en `L…;` salvo que ya empiece por `L` o `[`, y lo usan `refOf` y `classDescriptor`.
`TestUnaClaseLlamadaBNoEsElPrimitivo` lo fija.

**Ejecutado** con la sonda `B` (`static int[] v = new int[] {4, 5}`, `suma(new int[] {1, 2, 3})`,
`nombres().length`):
- JVM con `-Xverify:all`: `6`, `9`, `2`, `rc 0`.
- Nuestro dex (compilador + dexer en Go): `dexdump rc 0`, `app_process` `6`/`9`/`2`, `rc 0`.
- `d8 --release` sobre nuestras clases: `rc 0`, su dex `dexdump rc 0` y `6`/`9`/`2` en ART.

**Pruebas**: `apkc/javaemisor_inicializador_test.go` compara método a método (nombres, flags y la
sucesión de instrucciones con el miembro resuelto por nombre) contra `javac --release 8 -g`, y fija
los tres rechazos; `apkc/classreader_nombre_test.go` fija el nombre de clase de una letra.

**Suite y gates con esto**: **243 PASS + 2 SKIP** nativa (eran 240; dos pruebas del inicializador y
una del nombre de clase), **196 PASS + 49 SKIP** en el hospedador (las tres nuevas se saltan sin
`android.jar` montado), `studio/e2e.sh` en orden y el banco con los cinco caminos y el sexto de
interoperabilidad: salida `total 7|total 9|total 102|10|3|2|4|7|-1|10|6`, `rc 102`, `dexdump rc 0` en
los cuatro dex. El `dexdump` del banco se lee ahora en la **misma** llamada a `rish` que lo ejecuta:
con dos llamadas seguidas el cliente leía el fichero de estado antes de que el aparato lo terminara
de escribir, y el veredicto salía con el `rc` vacío.

## 74. El `long`: dos huecos, dos palabras, la constante `Long` del pool y el `const-wide` del dex

**El pool, con `javap -v` sobre `L`**: una entrada `Long` (tag 5) **ocupa dos posiciones** —`#7 = Long 2l`
y la siguiente es `#9`; el #8 no existe—, así que el escritor reserva un hueco (`agregarDos`) y
`serializar` lo cuenta sin escribir bytes.

**Los huecos de local, con `javap -v -c` sobre `F.conRama(long a, int n)` con un `long x` dentro**:
`locals=5` con `a` en 0-1, `n` en 2 y `x` en 3-4. El `StackMapTable` de javac es un `append` con
`locals = [ long ]`: **una** entrada para las dos mitades. El marco `full_frame` de este emisor hace lo
mismo: la segunda mitad se marca `verifSegundo` en el modelo y no se escribe.

**Las palabras de pila, con `javap -v -c` sobre `L.cuenta(long,long)`**: `stack=6` (dos largos dentro y
uno fuera, dos palabras cada uno). Destapado ejecutando: con `max_stack` contado por *elementos* la JVM
corta `VerifyError: Operand stack overflow … Exceeded max stack size` en el `<clinit>` que empieza con
`ldc2_w`; el conteo es por **palabras** y javac escribe 2 para un solo largo.

**Los opcodes, leídos de `javap -c` sobre `L` y `W` y confirmados al ejecutar**: `lconst_0` 0x09,
`lconst_1` 0x0a, `ldc2_w` 0x14 (dos bytes de índice), `lload` 0x16 / `lload_0..3` 0x1e..0x21, `lstore`
0x37 / `lstore_0..3` 0x3f..0x42, `ladd` 0x61, `lsub` 0x65, `lmul` 0x69, `ldiv` 0x6d, `lrem` 0x71,
`i2l` 0x85, `l2i` 0x88, `lreturn` 0xad. `1L` y `0L` salen como `lconst_1`/`lconst_0` y cualquier otro
por `ldc2_w`, incluso `2L`.

**El ensanchado, con `javap -c` sobre `W`**: `long x = 0` es `lconst_0` (el literal se escribe ancho),
`return n;` con `n` entero es `iload_0; i2l`, y `n + m` con `m` largo es **`iload_0; i2l; lload_1; ladd`**
—el entero se ensancha en su propio sitio, antes de cargar el otro lado—, así que el emisor necesita el
tipo de los dos operandos *antes* de escribirlos: por eso aparece `tipoPrevisto`.

**Lado del dex**, medido con `dexdump -d` sobre `d8 --release` con la misma sonda `L`: `const-wide/16`
(`1600 0200`, dos unidades), `mul-long` 0x9d en forma 23x, `add-long/2addr` 0xbb, `sget-wide` 0x61,
`sput-wide` 0x68, `return-wide` 0x10, `move-result-wide` 0x0b, y en la frame los argumentos largos valen
**dos registros**: `cuenta:(JJ)J` sale con `registers=6, ins=4` y `imprime:(J)V` con
`registers=3, ins=2, outs=3`, con `invoke-static {v0, v1}` para un solo argumento largo. También midió
`d8` que reasigna el par muerto (`mul-long v4, v4, v0` escribe encima del segundo argumento) y que
pliega `1L + 300L` en una sola constante.

**Añadido con dos sondas más** (`V`: conversiones, campo de instancia ancho, constantes de 17 y 5
dígitos; `M`: las cuatro cuentas y una copia): `int-to-long v0, v2` es **0x81** y `long-to-int v1, v0`
**0x84** (12x); `iget-wide` **0x53** y `iput-wide` **0x5a** (22c, el par destino en el nibble bajo);
`const-wide/16` **0x16** (`1600 0100`, dos unidades), `const-wide/32` **0x17** (`1700 7011 0100`, tres)
y `const-wide` entero **0x18** con los ocho bytes en little-endian en cuatro unidades
(`1800 cb04 fb71 1f01` para `1234567890123L`); la familia 23x larga va de **0x9b** (suma) a **0x9f**
(residuo) —`mul-long v4, v4, v0` medido como `9d04 0400`— y la `/2addr` de **0xbb** (suma) a **0xbf**
(residuo), con `div-long/2addr v0, v2` = `be20` y `add-long/2addr v0, v4` = `bb40`. `return-wide` es
**0x10** y `move-result-wide` **0x0b**. La frame cuenta los argumentos **en palabras**: `cuenta:(JJ)J`
sale `registers=6, ins=4`, `imprime:(J)V` `registers=3, ins=2`, `escribeCampo:(LV;J)V` `ins=3`, y una
llamada con un largo es `invoke-static {v0, v1}` (dos registros para un argumento). `d8` además
reutiliza el par del operando muerto (`add-long/2addr v0, v4` encima de una constante) y no emite nada
para `long b = a; return b;` (`return-wide v0` a secas).

**Escrito y ejecutado**. El traductor ya emite esas formas (`tomaPar` da dos registros consecutivos, y
la cuenta ancha usa el mismo reparto de vida que la entera del §66). Con la sonda `L`:

- Nuestro `.class` en la JVM con `-Xverify:all`: `306` y `8`, `rc 0`.
- **Nuestro dex sobre nuestras clases es byte a byte el mismo que nuestro dex sobre las clases de
  `javac`** (1104 bytes, `cmp` mudo) —con el fixture compilado `--release 8 -g:lines,source`; con
  `-g:none` faltaba el `SourceFile` y con `-g` sobraba el `LocalVariableTable`, y las dos cosas cambian
  el fichero—.
- `d8 --release` sobre nuestras clases: `rc 0`, y su dex también da `306` y `8` en ART.
- `dexdump` del nuestro: `rc 0`, y `imprime:(J)V` sale con las mismas seis palabras que `d8`
  (`6200 0100`, `6e30 0600 1002`, `0e00`, `registers 3 / ins 2 / outs 3`). Donde elegimos otra forma
  válida está dicho en la prueba: para `a + b * 2L` nosotros hacemos `mul-long/2addr v4, v0` (`bd04`)
  encima del par muerto y `d8` escribe el `mul-long` de tres registros; y no plegamos `1L + 300L`.

Tres defectos salieron al ejecutar, ninguno visible en los `cmp` anteriores: (1) un `static long` sin
`ConstantValue` se anunciaba en el `encoded_array_item` con `value_null` y ART cortaba con
`unexpected static field initial value type: 'L' vs 'J'` —es la regla 4 del §55 con un campo ancho, y
ahora va con `valueLong` y ocho bytes—; (2) el `parameters_count` del `debug_info_item` se sacaba de
`ArgCount`, que cuenta **palabras**, y con dos `long` declaraba cuatro parámetros: ART avisaba
`invalid stream - problem with parameter iterator … for method long L.cuenta(long, long)`; el conteo es
el de parámetros del descriptor (y `this` ya no estaba dentro, así que se quita la resta que hacía el
§56); (3) una clase llamada `L` —que empieza por la `L` de un descriptor— salía al dex como
`type_id "L"` (`Invalid type descriptor: 'L'`), la misma familia de `B` del §73, y se cierra exigiendo
el `;` de cierre en `descriptorDeClase`.

**Pruebas**: `apkc/javaemisor_largo_test.go` compara método a método contra `javac --release 8`, con
`max_stack` y `max_locals` incluidos, y fija los rechazos de `double`, del arreglo de largos y de la
comparación de largos; `apkc/dexlargo_test.go` exige el dex idéntico entre nuestras clases y las de
javac y las palabras de `d8` en `imprime`; `apkc/classreader_nombre_test.go` cubre ahora `B` y `L`. La
prueba de la frontera del caminante cambió de byte (`0x61` ya está medido; queda `0xd2`).

Suite: **246 PASS + 2 SKIP** nativa y **196 PASS + 52 SKIP** en el hospedador. El banco lleva
`saldo`, `cuentaLarga(2L, 3L)` y `acumula()`, y los cinco caminos dan
`total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306` con `rc 102`.

**Frontera**: `float` y `double` (el léxico ni siquiera arma `1.0`), arreglos de largos (`laload` /
`lastore` sin medir), `lcmp` y las ramas sobre largos, `dup2`/`pop2` y el `access$` de un campo ancho
(pide `dup2_x1` y `pop2`). Un `static final long` con `ConstantValue` tampoco: `classField` guarda el
`Long` pero `buscarCampo` sigue doblando solo enteros.

## 75. El `finally` y los dos `catch` en el mismo `try`: la copia en cada camino y el `any` que re-lanza

**Cuatro sondas** (`Fin`: `finally` solo, `return` en el `try` y varios `catch` con `finally`; `Fin2`: el valor
pendiente, un `finally` que lanza y un `break` dentro del `try`; `Fin3`: un `finally` vacío, un `try` vacío y
el `throw` como última sentencia; `Fin4`: un `void` que termina en el `athrow` del `finally` y un `try`
anidado dentro de un `try` con `finally`), leídas con `javac --release 8 -g:lines,source` y `javap -c -p -l` y
`javap -v`.

**No hay subrutina**: el bloque se **copia** al pie de cada camino que puede caer —el normal y el de cada
`catch`— y el último manejador es el `any`, que guarda la excepción en un local, copia el bloque y hace
`aload` + `athrow`. En `Fin.simple` (`try { a = 1; } finally { a = 2; }`): `0 iconst_1; 1 putstatic; 4 iconst_2;
5 putstatic; 8 goto 18; 11 astore_0; 12 iconst_2; 13 putstatic; 16 aload_0; 17 athrow; 18 getstatic` con
`0..4 → 11 any`. En `Fin.multi` la fila del `any` del cuerpo va **después** de los tipos y **antes** de las de
los cuerpos de los `catch`: `0..7 → 15 AE`, `0..7 → 28 RE`, `0..7 → 42 any`, `15..20 → 42 any`,
`28..34 → 42 any`.

**Los huecos de local**: cada `catch` declara su excepción **en el mismo hueco** (los dos brazos de `multi`
escriben `astore_1`), y el temporal del `any` va en el **más alto que haya visto el método** (`astore_2` en
`multi`; `astore_3` en `Fini.anidado`, donde el `try` interior ya usó el 2). Con `return` pendiente el valor se
guarda antes y la excepción sube un hueco más: `Fin2.retornoConCatch` es valor `1`, excepción del `catch` `1`,
valor del `catch` `2`, `any` `3`.

**Las filas de la `LineNumberTable`**: la copia del camino normal lleva la línea de la **primera sentencia del
`finally`** y el `goto` (o la re-lanza) que la sigue, la de la **llave que lo cierra** —`multi`: `33 en 7` y
`34 en 12`; `simple`: `9 en 4` y `10 en 8`—; la entrada del manejador `any` se anota con la **misma línea de la
sentencia** del `finally`, no con la del `catch` ni la del `try` (`multi`: `33 en 42`, `aload` en `34 en 48`;
`Fin.lanza`: `43 en 8` y `44 en 14`). Sin `finally`, el `goto` del `try/catch` lleva la línea de la **llave que
cierra el último `catch`** —`Fini.dosCatch`: `44 en 7` y `44 en 13`—, regla que el párrafo 71 no había escrito.

**Las salidas que cambian el remate**: un `finally` que lanza (`Fin2.lanzaEnFinally`) no deja `goto` después de
la copia, y en un método `void` javac tampoco escribe el `return` implícito —`Fin4.vacioConThrowFinal` termina
en el `athrow` del manejador, en el pc 20—. Un `finally` **vacío** se suelta entero (`Fin3.vacioFinalmente`:
cuerpo del `try` y nada más, sin copia, sin `goto` y sin fila en la tabla) y un `try` **vacío** con `catch`
tampoco escribe la tabla (`Fin3.sinCuerpo` es `iload_0; ireturn`).

**Dos rangos que van al mismo manejador con el mismo tipo se mezclan**: `Fin3.soloLanzaConCatch` protege
`0..11 → 19 any` (cuerpo y `catch` de una) y `Fini.anidado` escribe `2..13 → 20 any` aunque en el medio haya un
`goto` (pc 7) y el manejador interior (10..13).

**La trampa que queda fuera**: cuando la **última sentencia del cuerpo es un `throw`**, javac escribe un rango
que se pasa a la primera instrucción de su propio manejador —`Fin.lanza`: `0..9 → 8 any` con el `astore` en 8—
y mezcla cuerpo con `catch`. Este emisor lo rechaza por nombre, y también rechaza el `return`/`break`/`continue`
que **sale** de un `try` con `finally` (el valor pendiente: `Fin2.retorno` es `istore_0` antes de la copia e
`iload_0` después; `Fin2.ruptura` guarda el destino del `break` en otro local) y un `catch` con dos tipos
separados por `|`.

**Los marcos del verificador**: cada entrada de manejador lleva **el tipo declarado en el `catch`** sobre la
pila y el `any` lleva `Throwable` —`javap -v` de `Fin.multi`: `same_locals_1_stack_item` con
`ArithmeticException`, `RuntimeException` y `Throwable`, y `same` en la salida (pc 50)—. `apkc/jvmframes.go` ya
unía esos estados desde el párrafo 71; no hizo falta cambiarlo.

**En el dex** no hizo falta escribir nada nuevo: `armaTries` (párrafo 71) ya agrupa por rango las entradas del
mismo `try`, ya escribe el `catch_all` con el tamaño en negativo y ya acepta varios `try_item` por método. Lo
que aporta el fuente son los cuerpos de los `catch` protegidos y los huecos de local del temporal, que suben
`registers`. Nuestro dex de `Fini` (siete métodos, dos con tres manejadores) es **byte a byte** el de javac:
**1452 = 1452 bytes, `cmp` mudo**.

**Ejecutado**: el banco lleva `Ver.finalmente(5)` (camino normal, 22) y `Ver.finalmente(0)` (atrapa la división
por cero, 7); los cinco caminos y la JVM con **nuestra clase emitida** bajo `-Xverify:all` dan
`total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306|22|7` con `rc 102`, y `dexdump` del aparato verifica los
cuatro dex con `rc 0`.

**Pruebas**: `apkc/javaemisor_finally_test.go` — compara método a método contra `javac --release 8
-g:lines,source` las instrucciones, la `exception_table`, la `LineNumberTable`, `max_stack` y los huecos de
local; exige el dex idéntico byte a byte; y fija los cuatro rechazos (valor pendiente del `return`, valor
pendiente del `break`, `throw` último con `finally`, `catch` con `|`). Se fue
`TestElTryDelFuenteRechazaLoNoMedido`, que reclamaba como no medidos los dos casos que este párrafo midió.

Suite: **248 PASS + 2 SKIP** nativa y **196 PASS + 54 SKIP** en el hospedador.

**Frontera**: el valor pendiente que javac guarda cuando un `return`, un `break` o un `continue` **sale** de un
`try` con `finally` (medido en `Fin2`, no escrito); el `throw` como última sentencia del cuerpo de un `try` con
`finally` (el rango se pasa al propio manejador); `catch (A | B e)`; y `try`-with-resources.

## 76. El valor pendiente: un `return` o un `break` que cruza el `finally`

Dos sondas más (`Fin5`: un `return` en una rama del cuerpo y un `break` dentro de un bucle con `finally`;
`Fin6`: el `return` de un `void`, el mismo en una rama, un `finally` anidado y un `return` en la rama `else`),
con `javac --release 8 -g:lines,source` y `javap -c -p -l`.

**El cuerpo deja de ser un rango único**: cada salida abrupta **cierra un trozo** y el siguiente trozo empieza
**después** de la copia, del recargado y del `return` (o del `goto`). Medido en `Fin5.rama`
(`try { if (n > 0) { return 1; } n = n + 1; } finally { a = 2; }`):
`0 iload_0; 1 ifle 12; 4 iconst_1; 5 istore_1; 6 iconst_2; 7 putstatic; 10 iload_1; 11 ireturn;
12 iload_0; 13 iconst_1; 14 iadd; 15 istore_0; 16 iconst_2; 17 putstatic; 20 goto 30` con la tabla
`0..6 → 23 any` y `12..16 → 23 any`. El primer rango termina en 6 —justo después del `istore` que guarda el
valor— y el segundo **empieza en 12**, no en 0: la copia y el `return` del camino primero no están protegidos
(si estuvieran, el `finally` correría dos veces). Lo mismo con el `break` (`Fin5.breakConFinally`:
`6..11 → 29 any` y `18..22 → 29 any`, con el `goto 43` del `break` en 15) y con el `void`
(`Fin6.ramaVoid`: `0..4 → 22 any` y `10..14 → 22 any`).

**El hueco del valor**: `reserva` sin nombre, en el hueco que toca (`Fin5.rama`: `istore_1` con `n` en el 0;
`Fin2.retornoConCatch`: valor del `try` en 1, excepción del `catch` en 1, valor del `catch` en 2 y el temporal
del `any` en 3 —el más alto que vio el método, regla del párrafo 75—). Un `return;` de `void` no deja valor y
no usa hueco: la copia va primero y el `return` después (`Fin6.ramaVoid`: `4 bipush 6; 6 putstatic; 9 return`).

**Las filas de línea**: el `goto` de un `break` que cruza el `finally` lleva **la línea del `break`** (medido
en `Fin5.breakConFinally`: `21 en 15`, y en `Fin2.ruptura`: `54 en 9`), mientras que el `goto` del camino que
cae lleva la de la **llave del `finally`** (párrafo 75). El `return` pendiente marca dos veces su propia
línea: una en la evaluación/`istore` (pc 4 de `rama`) y otra en el `iload` (pc 10). En el `void` no hay fila
para el `return` hasta su instrucción —`Fin6.ramaVoid`: `115 en 4` (la copia) y `111 en 9` (el `return`), sin
fila del `return;` en 4—, así que la copia se escribe **antes** de marcar la línea de la sentencia.

**Un cuerpo que no deja nada protegido escribe en javac un manejador muerto**: `Fin6.soloVoid`
(`try { return; } finally { a = 5; }`) sale con la copia en 0, el `return` en 4, el `astore_0` en 5 y la
**tabla sin filas** —el `walkJvmCode` de este emisor llama la atención sobre el pc inalcanzable, así que el
caso se rechaza por nombre—. Con `catch` y `finally`, javac reparte los tipos por trozos cuando el cuerpo sale
por dos (otro rechazo con nombre).

**Anidado, medido y sin escribir**: `Fin6.anidado` (un `return` dentro de un `try` con `finally` dentro de otro
`try` con `finally`) copia los dos bloques seguidos —`6 bipush 7; 8 putstatic; 11 bipush 8; 13 putstatic;
16 iload_1; 17 ireturn`— y la tabla queda con `0..6 → 26 any`, `0..11 → 42 any` y `18..34 → 42 any`, es decir,
**rangos que se solapan**. En el dex los `try_item` no pueden solaparse, así que hay que repartirlos; este
emisor rechaza la forma por nombre.

**En el dex** no hizo falta tocar nada otra vez: los rangos se agrupan por `(inicio, fin)` y cada uno lleva su
lista de manejadores. El dex de la sonda ampliada (**doce métodos**, con `retorno`, `retornoConCatch`, `rama`,
`ramaVoid` y `breakConFinally`) sale **byte a byte** el de javac: **2052 = 2052 bytes**.

**Ejecutado**: el banco añade `Ver.salidaPendiente(5)` (el `return` pendiente, 10) y `Ver.salidaPendiente(0)`
(la caída, 101); los cinco caminos y la JVM con **nuestra clase** bajo `-Xverify:all` dan
`total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306|22|7|10|101` con `rc 102`, y `dexdump` del aparato verifica
los cuatro dex con `rc 0`.

**Pruebas**: `apkc/javaemisor_finally_test.go` compara ahora doce métodos —instrucciones, `exception_table`,
`LineNumberTable`, `max_stack` y huecos de local—, exige el dex idéntico, y fija siete rechazos: el
`continue` que sale, un `return`/`break` que cruza otro `try` anidado, uno dentro del propio `finally`, el
`throw` último del cuerpo y del `catch`, el cuerpo que sale por dos trozos con `catch`, el cuerpo sin nada
protegido y el `catch (A | B e)`.

Suite: **248 PASS + 2 SKIP** nativa y **196 PASS + 54 SKIP** en el hospedador.

**Frontera**: los dos `finally` anidados con una salida pendiente (los rangos se solapan y el dex exige
repartirlos), el `continue` que sale de un `try` con `finally`, el `catch (A | B e)`, `try`-with-resources, y
lo que seguía del 75: `float`/`double`, las ramas sobre `long` (`lcmp`, `dup2`/`pop2`), los arreglos de largos y
el `ConstantValue` de un `static final long`.

## 77. El `float` y el `double` en el `.class`: un hueco contra dos, y los tags 4 y 6 del pool

**El léxico era la frontera real**, no el emisor: el lector de números paraba en los dígitos y las letras
`a`–`f`, así que `2.5` salía como `2`, `.`, `5` («tras el punto se esperaba un nombre y hay "5"»). Se le
añade el punto decimal —punto seguido de dígito, y después lo que ya admitía el bucle (dígitos, letras y el
sufijo `f`/`d`)—. El exponente con signo (`1e-3`) sigue sin armarse, porque el `+`/`-` no entra en el token, y
el hexadecimal con `p` se rechaza con su nombre.

**Los *tags*, medidos en los bytes de `FyD.class` y no en la memoria**: se busca la secuencia del `2.5`
(`40 04 00 00 00 00 00 00`) y el byte anterior es **0x06**; la del `1.5f` (`3F C0 00 00`) va precedida de
**0x04**. O sea `Float` = tag 4 con cuatro bytes y **una** posición, `Double` = tag 6 con ocho y **dos**, como
la `Long` tag 5 del párrafo 74. La tabla del JVMS confirmada a mano: `Integer` es 3 (el que ya usábamos).

**Un hueco contra dos, con `javap -c`**: `resta(float a, float b)` es `fload_0; fload_1` —dos flotantes en
huecos consecutivos— mientras `suma(double a, double b)` es `dload_0; dload_2`, y `por(float a, double b)`
pone `a` en 0 y `b` en 1-2. El `esAncho` del párrafo 74 contaba el `F` como ancho: estaba mal y no se
ejercitaba porque el guardia lo rechazaba; **ancho es `J` y `D`**, el `float` vale un hueco y una palabra.

**Las cuentas y la promoción**, leídas de `FyD`: `fadd` 0x62, `fsub` 0x66, `fmul` 0x6a, `fdiv` 0x6e,
`frem` 0x72 y `dadd` 0x63, `dsub` 0x67, `dmul` 0x6b, `ddiv` 0x6f, `drem` 0x73 —cada familia va justo después
de la entera y la larga, que ya estaban medidas (0x60/0x61, 0x64/0x65, …)—. La conversión va **en el sitio del
operando**: `por` es `fload_0; f2d; dload_1; dmul`, `divide(double a, int b)` es `dload_0; iload_2; i2d; ddiv`
y `campoEstatico` (`dd + ff`) es `getstatic dd; getstatic ff; f2d; dadd`. La promoción de Java es
**doble > flotante > largo > entero**, así que `float + long` da flotante: por eso `tipoPrevisto` devuelve el
tipo promocionado en vez de mirar solamente el `J`.

**Las constantes cortas**: `0.0f` es `fconst_0` (0x0b) y `1.0` es `dconst_1` (0x0f), medido en `deEntero`
(`x + 1.0` sale `dconst_1`); lo demás va por pool —`3.14159` con `ldc2_w`, `1.5f` con `ldc #25`—. El menos se
pliega en la constante: `negativo()` es `ldc2_w #21 // double -2.5d`.

**Retornos, cargas y guardas**: `freturn` 0xae, `dreturn` 0xaf; `fload` 0x17 con `fload_0..3` 0x22..0x25,
`dload` 0x18 con `dload_0..3` 0x26..0x29, `fstore` 0x38 con `fstore_0..3` 0x43..0x46 y `dstore` 0x39 con
`dstore_0..3` 0x47..0x4a. La rejilla de conversiones es `i2l` 0x85, `i2f` 0x86, `i2d` 0x87, `l2i` 0x88,
`l2f` 0x89, `l2d` 0x8a, `f2i` 0x8b, `f2l` 0x8c, `f2d` 0x8d, `d2i` 0x8e, `d2l` 0x8f, `d2f` 0x90 —anclada en los
dos valores medidos del párrafo 74 y confirmada instrucción a instrucción contra `FyD`—. Un cast numérico
**no** es `checkcast`: `(int) d` sale `d2i` y `(float) d` sale `d2f`.

**Los marcos del verificador**: `ITEM_Float` es 2 y `ITEM_Double` 3 (el `ITEM_Long` 4 del párrafo 74 sigue
valiendo), y el doble entra en el marco con **una** entrada para los dos huecos, igual que el largo. La
pila simulada cuenta palabras: el `double` suma dos y el `float` una (`altoPilas`).

**Ejecutado**: un conductor compilado con javac (`Driver`, que suma los trece métodos e imprime) corre
contra **nuestra** `.class` con `java -Xverify:all` y da el mismo texto que contra la de javac: `36.39159` y
`1.5|2.5`, `rc 0` en los dos caminos. Nuestro `FyD.class` son 1181 bytes.

**Frontera por nombre, en aquel momento**: el traductor se paraba en `dload_0` (`outside the measured subset`)
—las formas de dalvik para el flotante y el doble entraron midiendo el fichero de `d8` en el párrafo 80—;
`faload`/`fastore` (un arreglo de `F` o de `D` sale con «un arreglo de F no está medido»); las comparaciones
sobre flotantes o dobles (`>` entre `D` y `D`); los casts a `byte`, `short` y `char`
(`i2b`/`i2c`/`i2s`, sin medir) y el exponente con signo.

**Pruebas**: `apkc/javaemisor_flotante_test.go` — tres pruebas: compara los trece métodos uno a uno contra
`javac --release 8 -g:lines,source` (instrucciones, `max_stack`, huecos de local y filas de línea), exige el
dex idéntico entre nuestras clases y las de javac, y fija tres rechazos. Cayeron dos rechazos que ya no son
ciertos —`double d = 1.0` en la prueba del largo y el del léxico sobre `1.0`— y la prueba del caminante
cambió de byte (0x6a ya es `fmul`; ahora es 0x94, `lcmp`).

Suite: **251 PASS + 2 SKIP** nativa y **196 PASS + 57 SKIP** en el hospedador (las tres pruebas nuevas se
saltan si el hospedador no lleva `android.jar` montado).

**El entorno se rompió con el reset de fábrica del aparato, no el repo.** La sesión pasó a otro usuario de
Android (`u0_a237`, antes `u0_a379`) y los enlaces de `~/Android/build-tools/34.0.0/`
(`aapt`, `aapt2`, `apksigner`, `d8`, `zipalign`) quedaron con el contexto SELinux del usuario anterior:
`ls -Z` del objetivo sano muestra `u:object_r:app_data_file:s0:c237,…`, y sobre el enlace viejo ni
`readlink`, ni `stat`, ni `unlink` —todo `Permission denied`—. Con eso el gate daba `aapt2: permission denied`
y el primer `e2e` de este párrafo salió con dos problemas.

**Reparación medida**: los binarios existen como paquetes de Termux en `$PREFIX/bin/` (`pacman -Q`:
`aapt2 16.0.0.4-4`, `d8 37.0.0-0`) y corren desde ahí —`aapt2 version` → `2.20-android-16.0.0_r4`,
`d8 --version` → `D8 9.2.4-dev`—. Se creó `~/Android/build-tools/34.9.0/` con enlaces **nuevos** a esos
binarios (los enlaces nuevos nacen con el contexto actual y sí funcionan) y `banco/banco.sh` ahora elige la
carpeta de build-tools más alta igual que `studio/e2e.sh`, en vez de una ruta fija. Primer intento con
`35.0.0` y el build falló pidiendo `platforms/android-35/android.jar`: **apkc deriva la platform del número
de build-tools**, y en este SDK solo hay `android-34`; renombrada a `34.9.0`, `apkc doctor` vuelve a resolver
`platforms/android-34/android.jar` ✓. Con eso el gate vuelve a estar **en orden** (el apk del contador,
10287 bytes, verificado por `apksigner`, `zipalign`, `zipcheck.py`, `aapt2 dump` y releído por `d8`).

**Lo que falta en esta sesión**: `rish` desapareció con el reset (`command -v rish` vacío), así que
`banco/banco.sh` no puede correr sus caminos de aparato (`dexdump` y `app_process64` del teléfono). La
evidencia de ejecución del punto flotante, mientras, es la de la JVM real: `-Xverify:all` sobre nuestra clase,
que ya está arriba.

## 79. El `instanceof`, y el oráculo de `d8` sin aparato: `instance-of` es 0x20 en forma 22c

**El `.class`, con `javap -c -p -l` sobre `Ins`**: `static int esCadena(Object x) { if (x instanceof String) { return 1; } return 0; }` es
`0 aload_0; 1 instanceof #7 // class java/lang/String; 4: ifeq 9; 7 iconst_1; 8 ireturn; 9 iconst_0; 10 ireturn`.
Tres cosas: el `instanceof` consume la referencia y **deja un entero** (el alto de pila no cambia), la
condición del `if` es el `ifeq` **sin invertir** —al revés que las seis comparaciones del párrafo 52, que
salen invertidas—, y el indice es de **dos bytes** (`instanceof` = 0xc1, junto al `checkcast` 0xc0 y el
`multianewarray` 0xc2, este ultimo fuera del subconjunto). La filas de línea del `if` son las de la
sentencia (`line 5: 0`, `line 6: 7`, `line 8: 9`) —no hay fila para el `ifeq`—, y el marco del destino es
un `same_locals_1_stack_item`… aqui `same`, porque en el `else` la pila ya está vacía: `javap -v` muestra
`stack=1, locals=2` y un unico marco `same` en el pc 9.

**El dex, medido sobre el fichero de `d8` y no sobre un manual** (sin `rish` hace falta un oráculo local, y
`d8` lo da: escribe el dex, y las **palabras** las leo con nuestro lector `readDex`, que no comparte código
con el traductor porque no lo interpreta —solo corta `code_item`—). `d8 --debug` sobre `Ins.class` deja en
`esCadena` las palabras `32 4 | 56 4 | 4114 15 | 18 15`, y en `esSecuencia` `32 2 | 56 4 | 8210 15 |
12306 15`. Decodificadas con la tabla ya medida en el repo (`check-cast` 0x1f, `array-length` 0x21,
`new-array` 0x23, `if-eqz` 0x38, `const/4` 0x12, `return` 0x0f):

- `32` = **0x20**, el hueco que queda entre `check-cast` y `array-length`, con la forma 22c: el primer
  registro en el nibble bajo del byte alto y el segundo en el alto (`0x0020` → `v0, v0`), y el `type@CCCC`
  en la unidad siguiente (`4` = `Ljava/lang/String;`, `2` = `Ljava/lang/CharSequence;` ✓ los indices
  coinciden con los del `instanceof` del `.class`).
- `4114` = `0x1012` = `const/4 v0, #1` ✓ (el literal en el nibble alto: `0x2012` → `#2` en `esSecuencia` y
  `0x3012` → `#3` ✓ las dos constantes del fuente), y `15` = `0x000f` = `return v0` ✓.

Para fijar el orden de los nibbles hace falta un caso con **destino y origen distintos**, y lo di
`Ins2.f(Object a, Object b)` con `if (b instanceof CharSequence)`: `d8` escribe `4128 2` =
`0x1020` = `instance-of v0, v1, type@2` ✓ —destino v0 (nibble bajo del byte alto), origen v1 (nibble
alto)—. Por eso el traductor pide un **temporal** para el destino en vez de escribir sobre el de la
referencia: la referencia probada puede seguir viva despues de la condicion (`if (b instanceof X) { usa(b); }`).

**Añadido al emisor y al traductor**: el analizador lee `instanceof` con la precedencia de las relacionales
(nivel 4, no asociativo); el emisor escribe `0xc1` con el indice de la clase en el pool (la misma `usar` +
`pool.clase` del `checkcast`, así que la clase probada entra en `InnerClasses`-like como tipo usado);
`condicionBranca` devuelve `opIFEQ` con una palabra consumida; la tabla del caminante lleva
`0xc1: {"instanceof", 2}`; el interprete de marcos saca la referencia y empuja un entero; y el traductor emite
`unidad12x(0x20, temporal, origen)` + el indice de tipo.

**Comparados con javac, los cinco métodos**: `esCadena`, `esSecuencia`, `conArreglo` (el `instanceof` dentro
del bucle sobre `vs[i]`, que es donde el temporal importa), `baja` (el `checkcast` de siempre) y `anidado`.
Las instrucciones, `max_stack`, los huecos de local y las filas de línea salen **iguales**, y el dex que
escribe este fork sobre nuestras clases es **byte a byte** el que escribe sobre las de javac.

**Ejecutado en la JVM real**: un conductor compilado con javac (`InsDriver`, con `new Object[] { "hola", 3, "adios" }`)
corre contra nuestra clase con `-Xverify:all` y da el mismo texto que contra la de javac:
`1|0|2|3|2|cadena|5`, `rc 0` en los dos. **Lo que no se pudo comprobar en esta sesión es el `instance-of` en
ART**, porque `rish` no está: falta el `dexdump` del aparato y el `app_process` del banco.

**Rechazado por nombre, con la forma medida**: la condición negada (`if (!(x instanceof T))`, que en javac
usa el `ifne` y no la leímos), `x instanceof String[]` (el tipo de arreglo en el pool) y `instanceof` sobre
un primitivo (que en Java ni compila, pero el mensaje sale de nuestro portero).

**Pruebas**: `apkc/javaemisor_instof_test.go` — tres pruebas (comparación con javac método a método, dex
idéntico, y los tres rechazos). La prueba del caminante que fijaba `0x6a` como byte no medido ya no podia
usarlo (`fmul`, párrafo 77) y pasó a `0x94`, `lcmp`.

Suite: **254 PASS + 2 SKIP** nativa, **196 PASS + 60 SKIP** en el hospedador y `studio/e2e.sh` **en orden**
(el apk del proyecto generado, verificado por `apksigner`, `zipalign`, `zipcheck.py`, `aapt2 dump` y releído
por `d8`).

**Frontera**: el `instanceof` en ART hasta que vuelva `rish`; `x instanceof T[]`; la forma negada; y lo que
seguía: las ramas sobre `long` (`lcmp`, `dup2`/`pop2`) y los arreglos de largos o flotantes. El punto flotante
en el dex entró después, en el párrafo 80.

## 81. Las comparaciones anchas: `lcmp`, `fcmpl`/`fcmpg`, `dcmpl`/`dcmpg`, medidas las cinco

Dos sondas (`Ram`: `x > 0L`, `x == y` de largos, `x > y` y `x != 0.0` de dobles, `x <= y` de flotantes;
`Ram2`: `x < y` de dobles, `x == y` de dobles, `x >= y` y `x < y` de flotantes, `x < y` de largos), leídas con
`javac --release 8 -g:lines,source`, `javap -c` y —para el dex— `d8 --debug` sobre la clase de javac con las
palabras cortadas por `readDex`, igual que en el párrafo 80.

**En el `.class` la inversión del párrafo 52 se mantiene tal cual**, con el resultado de la comparación en la
pila como un entero:

| fuente | bytecode |
| --- | --- |
| `if (x > 0L)` | `lload_0; lconst_0; lcmp; ifle` |
| `if (x == y)` (largos) | `lload_0; lload_2; lcmp; ifne` |
| `if (x < y)` (largos) | `lload_0; lload_2; lcmp; ifge` |
| `if (x > y)` (dobles) | `dload_0; dload_2; dcmpl; ifle` |
| `if (x != 0.0)` | `dload_0; dconst_0; dcmpl; ifeq` |
| `if (x <= y)` (flotantes) | `fload_0; fload_1; fcmpg; ifgt` |
| `if (x >= y)` (flotantes) | `fload_0; fload_1; fcmpl; iflt` |
| `if (x < y)` (dobles) | `dload_0; dload_2; dcmpg; ifge` |

O sea: **`…l` con las relaciones que javac escribe al derecho (`>`, `>=`, `!=`, `==`) y `…g` con las que
escribe invertidas (`<`, `<=`)** —es la regla de NaN del JVMS 2.9, no una elección de javac: `dcmpl` deja -1
con un NaN y `dcmpg` deja 1, y cada una se combina con la branquia que ya estaba medida—. El `lcmp` no tiene
hermano porque un largo no puede ser NaN.

**En el dex**, las cinco formas y el formato 23x (`op|AA`, `BB|CC<<8`), con la branquia `if-*z` encima del
resultado —d8 no emite `if-eq` contra cero, sino `if-ltz`/`if-gez`/`if-gtz`/`if-lez`, que ya estaban medidos
(0x38..0x3d)—:

| método | palabras | lectura |
| --- | --- | --- |
| `fMayorIgual` (`fcmpl`) | `45 256 \| 58 4` | `0x2D cmp…` → **`fcmpl` = 0x2d**, `if-gez v0, +4` (0x3a) |
| `fMenor` (`fcmpg`) | `46 256 \| 59 4` | **`fcmpg` = 0x2e**, `if-ltz v0, +4` (0x3b) |
| `dIgual` / `doubleMayor` (`dcmpl`) | `47 512 \| 57 4` y `47 512 \| 61 4` | **`dcmpl` = 0x2f**, `if-nez` (0x39) y `if-lez` (0x3d) |
| `dMenor` (`dcmpg`) | `48 512 \| 59 4` | **`dcmpg` = 0x30**, `if-ltz` |
| `iguales` / `mayorQue` / `lMenor` (`lcmp`) | `49 512 \| 57 4`, `… 561 2 \| 573 4 …`, `49 512 \| 59 5` | **`lcmp` = 0x31**, con `if-nez`, `if-lez` y `if-ltz` |

Cada número sale de dos o tres casos independientes y coincide ✓; el bloque es contiguo 0x2d..0x31 y
remata en `if-eq` 0x32, que estaba medido en el párrafo 52. El operando del `cmp-long`/`cmpg-double` es la
**pareja** de cada lado (`512` = `0x0200` → `v0` y `v2`) y el destino es un registro suelto, con lo que un
método `double(double,double)` con `registers=4, ins=4` escribe el resultado encima de un operando muerto,
como ya hacía con las cuentas `/2addr` (párrafo 66).

**Ya escrito** (2026-11-03): el emisor acepta la comparación sobre `J`/`F`/`D` —promociona los dos lados
como en las cuentas, emite `lcmp`/`fcmpl`/`fcmpg`/`dcmpl`/`dcmpg` y encima la branquia contra cero de la
tabla de arriba, sin ahorrarse el literal cero— y el traductor conoce los cinco opcodes. La sonda `Ram` (ocho
métodos) sale con el mismo `Code` que la de javac y su dex **byte a byte** el suyo (1192 bytes); `dexdump -d`
lee `cmpl-double` 0x2f con `if-lez`/`if-eqz`, `cmpg-double` 0x30 con `if-gez`, `cmpl-float` 0x2d con
`if-ltz`, `cmpg-float` 0x2e con `if-gtz` y `cmp-long` 0x31 con `if-nez`/`if-lez`/`if-gez`. Ejecutado en el
aparato por el camino «todo nuestro» (`apkc compile` + `apkc dex`, sin javac ni d8), un driver con las ocho
relaciones imprime `1 2 1 2 …` igual que la JVM, con `dexdump` verificando `rc 0`; el banco lleva
`Ver.compara(2)` (devuelve 6) y los cinco caminos coinciden. `javaemisor_compara_test.go` fija el par
`cmp`+branquia de cada método y la identidad del `.class` y del dex contra javac.

Lo que sigue fuera, con la forma vista: `dup2`/`pop2` (el `access$` de un campo ancho, párrafo 72) y los
arreglos de largos y flotantes.

## 80. Las formas dalvik del `float` y el `double`, medidas sobre el fichero de `d8` —y ya implementadas

Sin `rish` no hay `dexdump` en el aparato, pero el oráculo sigue siendo ajeno: **`d8 --debug` escribe el
dex y las palabras las corta `readDex`**, que no comparte nada con el traductor (no interpreta opcode).
Sonda: `FyD` del párrafo 77, `d8 --debug --min-api 28 --lib platforms/android-34/android.jar` sobre la
clase de `javac` (1728 bytes), y las listas de palabras por método:

| método | palabras (`d8`) | lectura |
| --- | --- | --- |
| `suma(double a, double b)` | `0x20CB 0x0010` | `add-double/2addr v0, v2` + `return-wide v0` |
| `resta(float a, float b)` | `0x10C7 0x000F` | `sub-float/2addr v0, v1` + `return v0` |
| `por(float a, double b)` | `0x2089 0x30CD 0x0010` | `float-to-double v0, v2`, `mul-double/2addr v0, v3`, `return-wide v0` |
| `divide(double a, int b)` | `0x4083 0x02CE 0x0210` | `int-to-double v0, v4`, `div-double/2addr v2, v0`, `return-wide v2` |
| `constante()` (3.14159) | `0x0018 0x866E 0xEFFB 0x21F9 0x4009 0x0010` | `const-wide v0, #+0x400921F9EFFB8669` (los ocho bytes en little-endian, cinco unidades) + `return-wide` |
| `constanteCorta()` (0.0f) | `0x0012 0x000F` | `const/4 v0, #0` + `return v0` —un `0.0f` es un entero de una palabra— |
| `deEntero(int n)` | `0x4083 0x0219 0x3FF0 0x20CB 0x0010` | `int-to-double`, **`const-wide/high16 v2, #+0x3FF0`** (= 1.0, dos unidades), `add-double/2addr v0, v2`, `return-wide v0` |
| `aEntero(double d)` | `0x008A 0x000F` | `double-to-int v0, v0` + `return v0` |
| `aFlotante(double d)` | `0x008C 0x000F` | `double-to-float v0, v0` + `return v0` |
| `aDoble(float f)` | `0x2089 0x0010` | `float-to-double v0, v2` + `return-wide v0` |
| `campoEstatico()` (`dd + ff`) | `0x0061 0x0000 0x0260 0x0001 0x2289 0x20CB 0x0010` | `sget-wide v0, field@0` (el `double`), **`sget v2, field@1`** (el `float` va por la forma entera), `float-to-double v2, v2`, `add-double/2addr v0, v2`, `return-wide v0` |
| `negativo()` (-2.5) | `0x0019 0xC004 0x0010` | `const-wide/high16 v0, #+0xC004` + `return-wide v0` |

**Lo que fija la tabla**, anclado en lo ya medido (`const-wide` 0x18, `const-wide/16` 0x16, `sget-wide` 0x61,
`return-wide` 0x10, `move`/`move-wide` 0x01/0x04, `add-long/2addr` 0xbb y su bloque 0xbb..0xbf, `add-int`
0x90..0x94, `move-result` 0x0a):

- **Conversiones**: el bloque es la rejilla del `.class` menos cuatro —`int-to-long` 0x81 y `long-to-int` 0x84
  (párrafo 74) encajan con `int-to-double` 0x83 (`divide`, `deEntero`), `float-to-double` 0x89 (`por`,
  `aDoble`, `campoEstatico`), `double-to-int` 0x8A (`aEntero`) y `double-to-float` 0x8C (`aFlotante`)—. Los
  seis restantes de ese bloque (`int-to-float` 0x82, `long-to-float` 0x85, `long-to-double` 0x86,
  `float-to-int` 0x87, `float-to-long` 0x88, `double-to-long` 0x8B) salen del mismo paso y quedan por ver.
- **Cuentas `/2addr`**: el bloque de `float` es 0xC6..0xCA (`sub-float/2addr` 0xC7 por `resta`) y el de
  `double` 0xCB..0xCF (`add` 0xCB, `mul` 0xCD, `div` 0xCE por `suma`, `por`, `divide`) —después del de
  `long` 0xBB..0xBF, cinco posiciones cada uno porque no hay bit a bit sobre flotantes. Las formas 23x
  (`add-float` 0xA6.. y `add-double` 0xAB..) se infieren del mismo salto y **no** están vistas aún.
- **Constantes**: `0.0f` se escribe como `const/4 v0, #0` (una palabra), `1.0` y `-2.5` caben en
  `const-wide/high16` (**0x19**, dos unidades: `op|AA` + los dieciseis bits altos) y `3.14159` va por
  `const-wide` 0x18 en cinco unidades. Nuestro traductor no tiene por qué imitar la elección: basta con que
  emita una forma válida; la igualdad que exigimos es entre **nuestro** dex sobre nuestras clases y **nuestro**
  dex sobre las de javac (párrafo 74).
- **Locales y campos**: el `float` usa las formas de una palabra (`sget`/`sput`, `move`, `return`,
  `aget`/`aput`) y el `double` las anchas ya medidas (`sget-wide` 0x61, `move-wide` 0x04, `return-wide` 0x10,
  `iget-wide` 0x53 / `iput-wide` 0x5a). Un local `float` o `double` cargado desde el sitio que le da la
  frame no emite nada, como con el `long` ✓ (en `suma` y `resta` no hay ninguna instrucción de carga).

**Implementado sobre esta medición** (y volviendo a cambiar `TestElFlotanteSeDexaComoJavac` por la
comparación de bytes): el traductor acepta `fload`/`dload` (con sus breves), `fstore`/`dstore`, las cinco
cuentas de cada tipo en forma `/2addr`, las conversiones `i2f`/`i2d`/`f2d`/`d2i`/`d2f`/`f2i` (más las que
conservan palabra), `fconst_*`/`dconst_*`, el `ldc` de una entrada `Float` y el `ldc2_w` de una `Double`,
el `freturn`/`dreturn` y el acceso a campos de `float` por la forma de una palabra (`sget`/`sput`/`iget`/`iput`,
como `d8`) y a los de `double` por la ancha ya medida. **El dex de `FyD` (trece métodos) sale byte a byte el
mismo escrito sobre nuestras clases que sobre las de javac**, y la frontera que queda es concreta: la forma de
**tres registros** de una cuenta ancha de `float`/`double` (ninguna sonda la escribió; se rechaza con el nombre
del opcode), `faload`/`fastore` y `aget-wide`/`aput-wide` (arreglos de flotantes), `l2f`/`l2d`/`f2l`/`d2l`
(inferidas del mismo paso, sin sonda). La comprobación en ART llegó en el párrafo 82, cuando el aparato se
alcanzó con `sudo`.

## 82. El banco en ART sin `rish`: `float`, `double` e `instanceof` ejecutados, y los tres defectos que salieron

Al volver el root al aparato, el banco se alcanza con `sudo` en vez de `rish` (Shizuku desapareció con el
reset; `banco/banco.sh` elige uno u otro con `enAparato`). Con eso, la sonda del banco —que ya llevaba
`finalmente`, `salidaPendiente` y el `long`— ganó lo que faltaba del §79 y del §80 y se pudo **ejecutar**:

```java
static double doble = 2.5;
static float flotante = 1.5f;
public static int flotantes(int n) {
    double d = n * doble + flotante;
    long l = (long) d;
    Object o = "cadena";
    if (o instanceof String) { l = l + 1; }
    if (o instanceof CharSequence) { l = l + 1; }
    return (int) l;
}
public static double dobleCon(int n) { return n * 1.5; }
```

```sh
sh -c 'APKC="$PREFIX/tmp/apkc-banco" sh banco/banco.sh'
# jvm (javac): rc=102, salida: total 7|…|306|22|7|10|101|8|3.0
#   nuestro dex (Go de punta a punta): rc=102 y salida igual a la JVM  ok
#   dex de d8 sobre javac: rc=102 y salida igual a la JVM  ok
#   dex de d8 sobre nuestra Ver: rc=102 y salida igual a la JVM  ok
#   nuestro dex con nuestro main (sin javac ni d8): rc=102 y salida igual a la JVM  ok
# jvm con nuestra clase emitida (Xverify:all): rc=102 y salida igual a la JVM  ok
# banco: en orden
```

Los cuatro dex dan `dexdump` verificando `rc 0`, y el camino «todo nuestro» —las dos fuentes compiladas y
dexadas por este fork— imprime `…|101|8|3.0`: el `8` es `flotantes(2)` (el `double`, el `float`, el `long`,
las dos `instanceof` y las dos sumas) y el `3.0` es `dobleCon(2)`, o sea un `return-wide` leído por
`println(D)V`. Los cinco caminos coinciden.

**Tres defectos, y los tres solo los vio el aparato** (ni `d8`, ni la identidad byte a byte, ni la JVM con
nuestro `.class`):

1. **`unexpected static field initial value type: 'L' vs 'D'`.** El `encoded_value` de un estático tiene que
   ser del tipo del campo. El emisor ya lo hacía para `I` (párrafo 55) y `J` (74), pero un `double`/`float`
   sin `ConstantValue` caía en `valueNull`, y ART cortaba la clase entera antes de ejecutar nada. Se emite el
   cero de la anchura del tipo —`valueDouble | 7<<5` (0x11, ocho bytes) y `valueFloat | 3<<5` (0x10, cuatro)—,
   que el `<clinit>` sobrescribe al correr. De paso, `valorEstatico` ya emite la constante de un
   `static final double`/`float` (el `.class` la trae y el lector guardaba los bits en `ConstLong`/`ConstInt`):
   `dexdump` de nuestra sonda `Fd` lee `FD = 2.5` y `FF = 1.5`. `TestLosEstaticosAnchosLlevanSuTipo` lo fija
   leyendo el `encoded_array_item` con el lector propio.

2. **`instance-of on unexpected class Integer`.** `collectCode` no seguía `instanceof` (ni `checkcast`): la
   clase del `instanceof` solo entraba en `type_ids` si el fichero la volvía a nombrar en otra parte (un
   campo, una llamada, un descriptor). Con `o instanceof CharSequence` en una clase que no la nombra, el
   mapa devolvía el cero de la clave ausente y la instrucción salía `instance-of v5, v4, I` —ART la leía
   como «probar contra `int`» y rechazaba el método—. El test destapó que el fallo era *silencioso*: ahora
   `typeIdxDe` rechaza con nombre («the type_id … was never interned») en las cuatro instrucciones que
   llevan tipo (`new`, `new-array`, `check-cast`, `instance-of`), y el colector interna los dos operandos.
   `TestElTipoDelInstanceOfSeInterna` comprueba que los dos `instance-of` de la sonda apuntan a
   `Ljava/lang/String;` y `Ljava/lang/CharSequence;`; sin el arreglo el test falla con el rechazo nuevo.

3. **`it needs more wide temporaries than the max stack javap measured`, y después `register v16 does not fit
   a four-bit format`.** El marco de `main` se quedaba sin pares: un `double` vivo ocupa DOS registros, la
   pila de javac lo cuenta como dos palabras, pero dos temporales muertos sueltos no forman un par, y el
   reparto pedía siempre dos registros nuevos. Tres cambios, todos medidos por el efecto en el aparato:
   - `tomaPar` busca el **par consecutivo más bajo** de la bolsa libre, no solo las dos últimas entradas
     (con la bolsa `[0 2 3 4 5 6 7]` el par `(2,3)` existía y no se veía);
   - la pasada de medida **no es un rechazo**: `techo` solo lo pone la pasada que escribe las palabras, y sus
     números de registro son provisionales (una base conjetural empujaba `iload_0` a v16 y el propio
     instrumento de medida se caía);
   - la base se busca por **punto fijo**: se repite la medida subiendo la conjetura hasta que el pico cabe
     debajo, porque el plan de la pasada final depende de la base. `main` del banco pedía 11 temporales con
     una conjetura de 9; `Ver.flotantes` acabó con `registers=14` (base 8 y seis huecos) frente a los 6 de
     `d8` —el marco puede ser más grande, lo que no puede es pasar de v15 en un formato de cuatro bits—.
     `TestLaFrameAnchaCabe` exige `registers ≤ 16` en los dos métodos de la sonda.

**Y dos huecos del caminante** que solo aparecen con locales por encima del 3, como el `long` de esta sonda:
`lload` (0x16) y `lstore` (0x37) con el índice de un byte. `javap -c` sobre la sonda los enseña
(`lstore 4`, `lload 4`); sin ellos el caminante paraba en el primer `lstore 4`.
`TestElCaminanteConoceLosAnchosIndexados` fija las dos formas.

Comprobado además: `go test ./...` de punta a punta en el hospedador wasm (PASS) y `studio/e2e.sh` en orden
(41 comprobaciones en la pestaña; el proyecto generado y la app del contador siguen saliendo verificados por
`apksigner`, `zipalign`, `zipcheck`, `aapt2` y releídos por `d8`).

## 83. Los campos anchos privados: `dup2_x1`, `dup2`, `pop2`, y el orden de los `proto_ids` que ART exige

Una clase con `private long largo` y `private double doble` tocados desde una anónima
(`largo = largo + 1; doble = doble + 1.5;`) hace que javac escriba los accesores sintéticos del §72 con
las formas anchas. Medido con `javac --release 8 -g:lines,source` y `javap -c -p`:

```java
static long access$002(D, long);      // escribir
    Code:  0: aload_0  1: lload_1  2: dup2_x1  3: putfield #6  6: lreturn
static long access$000(D);            // leer
    Code:  0: aload_0  1: getfield #6  4: lreturn
static long access$002(long);         // el estático, en `E`
    Code:  0: lload_0  1: dup2  2: putstatic #1  5: lreturn
```

y el llamador anónimo tira el valor que devuelve la escritura con **`pop2`** (0x58):
`invokestatic D.access$002:(LD;J)J; pop2`. En el `.class` las tres formas nuevas son `dup2_x1` (0x5d),
`dup2` (0x5c) y `pop2` (0x58); `getfield`/`putfield` no cambian de opcode, el ancho lo dice el descriptor.

En el dex, `d8 --debug` sobre las mismas clases (`dexdump -d` del aparato) **no escribe nada** para el
`dup2_x1` ni para el `dup2` —las entradas comparten registros, como el `dup_x1` del §72— y el `pop2`
desaparece con la llamada, igual que el `pop` absorbido del §21. El accesor queda en `iput-wide` (0x5a) y
`return-wide` (0x10), y en `run` hay **dos** `move-result-wide` para cuatro `invoke-static`: las dos
lecturas guardan su resultado y las dos escrituras lo tiran.

**Cuatro defectos salieron ejecutándolo**, y ninguno lo veía `javap`:

1. `max_locals` del accesor contaba **argumentos** y no **huecos**: `(LD;J)J` pide tres y el accesor salía
   con `locals=2`, con lo que el traductor ponía `aload_0` en v-1 y el dex se rechazaba con
   `register v does not fit a four-bit format`. `javap -v` del nuestro y del de javac lo enseñan.
2. El número del `access$` se contaba sobre todo el proyecto: con `D` y `E` juntas, la segunda recibía
   `access$200`/`access$202` donde javac pone `access$000`/`access$002`. La posición es **por clase
   exterior**.
3. La contabilidad de pila del accesor no descontaba las dos palabras del `return` ancho (`el access$002
   deja 1 valores en la pila al cerrar`, que el emisor mismo detecta).
4. **El orden de los `proto_ids`**: ART cortaba el fichero con `Out-of-order proto_id arguments`. La regla
   medida es la de `d8` —por tipo de retorno y después por la **lista de parámetros** comparada por sus
   `type_idx`—, no por el shorty: `(LD;J)J` va antes que `(LF;)J` aunque sus shortys sean «JLJ» y «JL», que
   ordenarían al revés. Nuestro fichero ponía `(LF;)J` en medio (`5 (LD;)J`, `6 (LF;)J`, `7 (LD;J)J`) y el
   de `d8` dice `5 (LD;)J`, `6 (LD;J)J`, `7 (LF;)J`.

Con las cuatro correcciones, las cinco clases de la sonda salen con el mismo `javap -c` que las de javac
(solo cambian los números de pool) y su dex, escrito por este fork, verifica (`dexdump`, `rc 0`) y se
ejecuta en el aparato por el camino «todo nuestro» (`apkc compile` + `apkc dex`, sin javac ni d8):

```sh
$PREFIX/tmp/apkc-dup compile --android-jar $HOME/Android/platforms/android-34/android.jar -o $M/nuestro \
    $M/D.java $M/DM.java $M/E.java $M/F.java $M/G.java
$PREFIX/tmp/apkc-dup dex -o $M/todo.dex $M/nuestro/*.class
# /data/local/tmp: dexdump rc 0;  app_process64 p.DM  →  6 4.0   (la JVM dice lo mismo)
```

El banco lleva el caso en `Ver.anchoPrivado` (campo `private long ancho = 5L`, anónima que suma 2, devuelve
7) y los cinco caminos dan `…|6|7` con `rc 102`, incluida la JVM con nuestra clase emitida bajo
`-Xverify:all`. Tests: `javaemisor_privada_ancha_test.go` (el accesor con `iput-wide` y los dos
`move-result-wide` de cuatro llamadas; y el `.class` del emisor contra javac) y `dexprotos_test.go` (el
orden de los protos con el contraejemplo del shorty).

Lo que sigue fuera de este bloque: los arreglos de largos y de flotantes (`laload`/`lastore`,
`faload`/`fastore`, `aget-wide`/`aput-wide`) y el `ConstantValue` de un `static final long`.

## 84. Los arreglos de `long`, `float` y `double`, y el defecto de pares que solo ve ART

Una sonda con seis métodos (`long leeLargo(long[], int)`, `void escribeLargo(long[], int, long)`, y lo
mismo con `float` y `double`) medida con `javac --release 8 -g:lines,source`, `javap -c` y `d8 --debug`
desarmado con `dexdump -d` del aparato:

| fuente | `.class` | dex |
| --- | --- | --- |
| `v[i]` de un `long[]` | `laload` (0x2f) | `aget-wide v0, v2, v3` (**0x45**, 23x) |
| `v[i]` de un `float[]` | `faload` (0x30) | `aget v0, v0, v1` (**0x44**) |
| `v[i]` de un `double[]` | `daload` (0x31) | `aget-wide` 0x45 |
| `v[i] = x` de un `long[]` | `lastore` (0x50) | `aput-wide v2, v0, v1` (**0x4c**) |
| `v[i] = x` de un `float[]` | `fastore` (0x51) | `aput` (**0x4b**) |
| `v[i] = x` de un `double[]` | `dastore` (0x52) | `aput-wide` 0x4c |

El `float` va por las formas de una palabra y el `long`/`double` por las anchas, como en los campos y los
locales. En el `.class` las tres parejas de `*aload`/`*astore` son opcodes propios y el ancho lo dice el
descriptor del arreglo.

**Implementado y ejecutado**: el caminante conoce las seis formas, los marcos consumen y empujan lo que
toca (dos palabras en `laload`/`daload`, una en `faload`), el traductor baja a `aget-wide`/`aput-wide` con
la pareja como destino/valor (y suelta el par), y el emisor ya no rechaza `new long[n]`, `v[i]`, `v[i] = x`
ni `new T[] {…}` de esos tipos: escribe `newarray` con el atype de la tabla, `laload`/`lastore`, etc., y
contabiliza la pila con dos palabras. El camino «todo nuestro» (`apkc compile` + `apkc dex`, sin javac ni
d8) ejecuta en ART un driver con las seis operaciones y da `7 9 3 7 5 9`, lo mismo que la JVM, con
`dexdump` verificando `rc 0`.

**El defecto que destapó ART, y que ningún comparador de bytes veía**:

```
java.lang.VerifyError: ... [0x5F] wide register v7 has type Double (Low Half)/Double (Low Half)
```

El cuerpo era `move-result-wide v7` (el par es `v7,v8`), `int-to-double v8, v0` (el par es `v8,v9`) y
`mul-double/2addr v7, v8`: los dos pares se **solapaban en v8**. La causa estaba en el reparto: la pila
guarda solo el registro BAJO de un par, así que `suelta` liberaba la mitad alta de un par que seguía vivo
(`suelta(r+1)` no la veía en la pila), y `tomaPar` podía crear un par nuevo sobre esa mitad. Se corrigió
con tres piezas: `altos` (la mitad alta de un par vivo no se libera), `sueltaPar` que no devuelve nada
mientras el par siga en la pila, y `ocupado`, que hace que `toma`/`tomaPar` **salten** los registros
vivos al repartir un par nuevo (de dos en dos, para no partir pares).

**Lo que quedó pendiente entonces y se midió después**: el `Code` de la sonda del driver no era idéntico al
de javac porque javac pliega `fa * 2` a la constante `fconst_2` (2.0f) y este emisor escribía
`iconst_2; i2f` —cerrado en el §85—, y faltaban el test permanente de la sonda y el caso en el banco, que
al meterlo destapó dos defectos más (el `baja(3)` del inicializador ancho, §85, y el marco por encima de
v15, §86).

## 85. El literal se promociona **sobre la constante**, antes de elegir su forma

Sonda `Plega` (veinte métodos) medida con `javac --release 8 -g:lines,source Plega.java`, `javap -c -p
Plega.class` y el fixture en base64 dentro de `apkc/javaemisor_plega_test.go`. Para reproducir:

```sh
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/plega" &&
  javac --release 8 -g:lines,source Plega.java && javap -c -p Plega.class && base64 -w0 Plega.class'
sh -c 'cd apkc && go test -count=1 -run Plega .'
```

| fuente | `.class` que escribe javac | qué fija |
| --- | --- | --- |
| `f * 2` | `fload_0; fconst_2; fmul` | el 2 llega siendo flotante: no hay `i2f` |
| `f + 0.0f` / `f + 0` | `fconst_0` | da igual cómo esté escrito el cero |
| `f + 3` / `f * 100` | `ldc` con entrada `Float` (tag 4, **una** posición) | 3.0f y 100.0f no son forma corta |
| `f * -2` | `ldc` con `-2.0f` | el signo entra en la constante: no es `fconst_2; fneg` |
| `-0.0` / `-0.0f` / `0.0 * -1.0` | `ldc2_w` / `ldc` con el cero con signo | `dconst_0` es el cero **positivo**; el signo se mira |
| `d * 2` / `d + 2.0` | `ldc2_w` con `2.0d` | javac **nunca** usa `dconst_2` (0x10 existe, no lo escribe) |
| `d + 1` / `l + 1` / `l + 0` | `dconst_1` / `lconst_1` / `lconst_0` | la forma corta la decide el valor ya promocionado |
| `l + 2` / `l * 3` | `ldc2_w` | no existe `lconst_2` |
| `i + 100` / `i + 3000` / `i + -1` | `bipush` / `sipush` / `iconst_m1` | la regla del §53 sigue entera |
| `2.5f * 2` | una sola `ldc 5.0f`, sin `fmul` | dos literales se pliegan, cada operando entra ya promocionado |
| `1.5f + 2L` / `1.0 + 2.5f` | `ldc 3.5f` / `ldc2_w 3.5d` | la promoción binaria antes de la cuenta |
| `1.0f / 3.0f` | `ldc 0.33333334f` | la cuenta de `float` se hace en precisión de `float32`, no de `float64` |
| `2.5f / 0` / `1.0 / 0` / `1.0 % 0` | `ldc Infinity` / `ldc2_w Infinity` / `ldc2_w NaN` | el cero flotante de divisor **sí** se pliega: es el resultado de IEEE, no una excepción |
| `1 / 0` y `1L / 0` | `iconst_1; iconst_0; idiv` y `lconst_1; lconst_0; ldiv` | la entera y la larga no: la excepción tiene que saltar en ejecución (§53) |
| `2147483647 + 1` | `ldc -2147483648` | el desborde envuelve |
| `d + i` (`i` variable) | `iload_0; i2d` | una variable sí se convierte **en su sitio** (§77), no se pliega |
| `if (f > 2)` | `fconst_2; fcmpl; ifle` | la comparación promociona igual (§81) |
| `float g = 2` / `return 3` de un `double` | `fconst_2` / `ldc2_w 3.0d` | la regla no es solo de las cuentas: rige en asignación, retorno y campo |

Implementado en `apkc/javaemisor.go` con `plegarConstante` (reemplaza a `plegarEntero` del §53, que solo
sabía de enteros), `cargaConstante` (las formas cortas de la tabla) y la promoción dentro de `expA`, que es
por donde ya pasaban la cuenta, la comparación, el retorno, la declaración y la asignación. La lectura de
los literales `j`/`f`/`d` del fuente pasa por el mismo `cargaConstante`, así que `-0.0f` deja de salir con
`fconst_0`.

Oráculos: `TestElLiteralPromocionadoSeEscribeComoJavac` (los veinte cuerpos instrucción a instrucción,
con `max_stack`, huecos de local y filas de línea), `TestElLiteralPromocionadoSeDexaComoJavac` (el dex sale
**byte a byte** igual sobre nuestras clases que sobre las de javac: **1872 bytes**) y
`TestLaAsignacionYElRetornoPromocionanSuLiteral`.

**El defecto que destapó meter el caso en el banco**: `new double[] {1.5, 2.0}`. El inicializador del §73
bajaba **tres** palabras de pila por elemento (`e.baja(3)`: arreglo, índice y valor), correcto para
`int[]` y `String[]`, y un valor ancho son **cuatro**. El emisor respondía `quedan 2 valores en la pila al
cerrar el metodo` y el `.class` no salía. Con `ArreglosAnchos` (diez métodos: las seis operaciones del §84
más `new long[n]`, `new double[] {…}`, `.length` de un `float[]` y una suma con bucle) el `.class` sale con
el mismo `Code`, la misma pila, los mismos huecos y las mismas filas que el de javac, y el dex es **byte a
byte** el de javac: **1496 bytes**, `TestLosArreglosAnchosSeEscribenComoJavac` y
`TestLosArreglosAnchosSeDexanIgualQueLosDeJavac`.

## 86. Un marco por encima de v15: las formas `/from16` que escribe `d8`, medidas sin aparato

El caso nuevo del banco (`Ver.arreglosAnchos`) reventó el camino «todo nuestro» con

```
[dex] Lp/Ver;arreglosAnchos: at pc 0, iload_0: register v22 does not fit a four-bit format
      (the /range or /from16 form is not measured here)
```

Los argumentos van al FINAL de la frame (§57): en cuanto el pico de temporales sube, un hueco se escribe en
v16 o más y las formas 11n, 12x, 21t, 22c y 22s no lo alcanzan. Fallaba igual sobre las clases de `javac`,
así que el defecto era del traductor, no del emisor.

Medición. Sin `rish` (el reset dejó solo `sudo`), el oráculo es `d8 --debug` y el `dexdump -d` del aparato:

```sh
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/dieciseis" &&
  javac --release 8 -g:lines,source D16.java && d8 --debug --min-api 28 --output d8_16 D16.class &&
  base64 -w0 d8_16/classes.dex | sudo sh -c "toybox base64 -d > /data/local/tmp/d16.dex;
  /apex/com.android.art/bin/dexdump -d /data/local/tmp/d16.dex"'
```

Lo que hace `d8` con un registro alto, palabra escrita por palabra:

| situación | palabra | decodificada | qué significa |
| --- | --- | --- | --- |
| leer un argumento alto para una cuenta de cuatro bits | `0202 1200` | `move/from16 v2, v18` | forma 22v: **destino de ocho bits** en la primera palabra, **fuente de dieciséis** en la segunda; el valor baja y de ahí `int-to-double v2, v2` |
| lo mismo con un `long`/`double` | `0501 1300` | `move-wide/from16 v1, v19` | la pareja de destino tiene que caber entera en cuatro bits (techo v14) |
| lo mismo con una referencia | `0801 1300` | `move-object/from16 v1, v19` | y de ahí `iput-object v1, v0, field` |
| una cuenta con un operando alto | `900a 0a10` / `ab02 0210` | `add-int v10, v10, v16` / `add-double v2, v2, v16` | cambia la forma de dos registros por la de **tres**, que lleva ocho bits por campo |
| una llamada con los argumentos altos | `7714 0200 0100` / `7401 0500 1200` | `invoke-static/range {v1..v20}` / `invoke-virtual/range {v18}` | la `/range` toma un registro inicial de 16 bits |
| reasignar un argumento alto | `d804 1001` | `add-int/lit8 v4, v16, #1` | **nunca escribe sobre el alto**: renombra el local a v4 (el `/lit8` sí lee un fuente de ocho bits) |

Corregido en `apkc/dexinstr.go`: `bajaAlto` ata el hueco a un temporal bajo con la 22v medida y
`guardaLocalEn` copia sobre ese bajo. La copia se emite **una vez por hueco**, no por lectura: atar el hueco
es el mismo mecanismo con el que `guardaLocal` ata la primera escritura, así que el registro alto no se
vuelve a nombrar y no hace falta ningún análisis de viveza. Copiar en cada lectura (la primera versión) le
costaba a `Ver.compara` un temporal más por uso y su marco se salía también de v15 —el test
`TestElMarcoSeReintentaCuandoLaPasadaFinalPideMas` lo cazó—. De paso se arregló `unidad23x`, que recortaba
a nibbles el segundo y el tercer registro: los dos campos de esa palabra son de **ocho** bits, como
demuestra el `add-int v10, v10, v16` medido arriba.

Frontera, rechazada por nombre: las formas `/range` de las llamadas y cualquier `/from16` de una cuenta
(`const/16`, `add-int/lit16`) siguen sin emitirse —plan 5.4—; con el `move` abajo basta para lo que hoy
compila el subconjunto, y escribir SOBRE un registro alto no lo hace `d8`, así que no está medido y no se
hace.

Prueba permanente: `apkc/dex16_test.go`, dos fuentes cuyo cuerpo entero es leer un argumento alto y
volverlo, para que el cuerpo salga predecible palabra por palabra —`[move/from16 v0, v17][return v0]` y
`[move-wide/from16 v0, v18][return-wide v0]`—, y que el destino de la copia quede por debajo del techo.
Ejecutada, la prueba es el banco con el caso nuevo (`Ver.arreglosAnchos(2)`, que usa el alta ancha, el
inicializador `new double[] {1.5, 2.0}`, el `u[0] * 2` del §85 y una comparación larga):

```
jvm (javac): rc=102, salida: total 7|…|6|7|10
  nuestro dex (Go de punta a punta): rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  dex de d8 sobre javac:             rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  dex de d8 sobre nuestra Ver:       rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  nuestro dex con nuestro main:      rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  jvm con nuestra clase emitida (Xverify:all): rc=102 y salida igual a la JVM  ok
banco: en orden
```

## 87. El `char`: viaja como entero, se estrecha con `i2c` y tiene opcodes propios en los dos mundos

Sonda `Letras` (veintiún métodos y el `<clinit>`), medida con `javac --release 8 -g:lines,source`,
`javap -c -p` y `javap -v` para el `ConstantValue`; el lado del dex, con `d8 --debug` sobre las mismas
clases y el `dexdump -d` del aparato por `sudo` (tras el reset no queda `rish`):

```sh
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/char/sonda" &&
  javac --release 8 -g:lines,source Letras.java && javap -c -p Letras.class && base64 -w0 Letras.class'
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/char" &&
  javac --release 8 -g:lines,source Letras.java && d8 --debug --min-api 28 --output d8out Letras.class &&
  base64 -w0 d8out/classes.dex | sudo sh -c "toybox base64 -d > /data/local/tmp/letras.dex;
  /apex/com.android.art/bin/dexdump -d /data/local/tmp/letras.dex"'
sh -c 'cd apkc && go test -count=1 -run "Char|Letras" .'
sh -c 'cd /data/data/com.termux/files/home/Develop/Go/apkc-web &&
  APKC=/data/data/com.termux/files/usr/tmp/apkc-gate sh banco/banco.sh'
```

En el `.class` el `char` **es un `int` en la pila**; lo único propio son el descriptor `C` y cuatro
opcodes:

| fuente | `.class` | qué fija |
| --- | --- | --- |
| `'\n'`, `'\''`, `'\\'`, `'\t'` | `bipush 10 / 39 / 92 / 9` | la forma corta del entero, por valor (§53) |
| `'á'` (UTF-8 suelto) y `'ÿ'` | `sipush 225`, `sipush 255` | el punto de código, no el byte |
| `'\101'` y `'\u0008'` | `bipush 65`, `bipush 8` | el octal y el escape unicode |
| `c + 1` con `char c` | `iload_0; iconst_1; iadd` | **ninguna** conversión: ensanchar `char` a `int` no escribe nada |
| `(char) (a - b)` | `isub; i2c` | estrechar es `i2c` = **0x92** (el 0x91 es `i2b` y el 0x93 `i2s`) |
| `char c = 65;` y `v[0] = 65` | `bipush 65; istore_0` / `bipush 65; castore` | la constante entera entra en el hueco `char` sin conversión |
| `v[i]` / `v[i] = c` | `caload` **0x34** / `castore` **0x55** | las formas de arreglo propias del `char` |
| `new char[n]` | `iload_0; newarray 5` | el atype 5, que ya estaba en la tabla del §84 |
| `switch (c) { case 'a': … }` | `lookupswitch`/`tableswitch` con 97 | la clave `char` es el punto de código |
| `static final char CONSTE = 'Z'` | descriptor `C`, flags `0x0018`, `ConstantValue: int 90` y **sin** `putstatic` | la misma dobladura del `static final int` |
| `fijo = s; return fijo` con `static char fijo` | `putstatic`/`getstatic` con descriptor `C`, y en el `<clinit>` `bipush 65; putstatic` | el campo estático `char` existe |

En el dex las formas son **tipadas**, no la genérica del `int`:

| `.class` | palabra de `d8` | decodificada |
| --- | --- | --- |
| `caload` | `4900 0001` | `aget-char v0, v0, v1` —**0x49**, no el `aget` 0x44 del `int`— |
| `castore` | `5002 0001` | `aput-char v2, v0, v1` —**0x50** |
| `i2c` | `8e00` | `int-to-char v0, v0` —**0x8e**, en la rejilla de conversiones del §80 |
| `getstatic` de `fijo:C` | `6500 0100` | `sget-char v0, LLetras;.fijo:C` —**0x65** |
| `putstatic` de `fijo:C` | `6c00 0100` | `sput-char v0, …` —**0x6c** |
| `newarray 5` | `2300 0700` | `new-array v0, v0, [C` (la forma del §66, con su `type_id`) |

Medido de paso, y **no** implementado: `d8` reescribe el inicializador `new char[] {'a','b'}` como
`fill-array-data` (`2600 0500 0000`) con un payload `0003 0200 0200 0000 6100 6200`, cuyo
`element_width` es **2** —frente al 4 del `float` y al 8 del `double`, leídos en el volcado del §84—, en
lugar de la cadena `dup`/`castore`. Es la misma diferencia de reescritura ya aceptada del §53 y del 2G.4:
el oráculo manda sobre las estructuras, no sobre el cuerpo optimizado, y este fork emite la cadena de javac.

**La elección `tableswitch`/`lookupswitch` queda como diferencia escrita** (era del §64 y se apoya ahora
con datos): sobre una rejilla medida de claves equidistantes, javac escribe tabla en `(3 claves, rango 4)`,
`(4, 8)`, `(5, 10)`, `(6, 18)`, `(6, 19)` y `(8, 24)`, y búsqueda en `(2, 1)`, `(3, 6)`, `(4, 12)`,
`(5, 15)`, `(6, 24)` y `(8, 32)`; este emisor se queda con la tabla cuando `rango + 1 <= 2 · claves`, que
es más conservador —con cuatro claves y rango 8 javac escribe tabla y el emisor, búsqueda—. Las dos formas
son legales; la sonda del test usa claves consecutivas, donde los dos coinciden, y la comparación del `Code`
no depende de ese criterio.

Implementado: el léxico lee el literal con sus escapes (`leerCharJava` y `escapeJava` en
`apkc/javafuente.go`), el emisor emite el `char` como entero con descriptor `C`, estrecha con `i2c`,
conoce `caload`/`castore` y dobla el `ConstantValue`; el caminante y los marcos conocen los tres opcodes
nuevos; y el traductor baja a `aget-char`, `aput-char`, `int-to-char`, `sget-char` y `sput-char`. El
portero sigue rechazando por nombre `i2b`/`i2s` (el `byte` y el `short` no los escribió sonda alguna).

Oráculos: `apkc/javaemisor_char_test.go` —`TestElCharSeEscribeComoJavac` (veintidós cuerpos con pila,
huecos y filas de línea), `TestElCharSeDexaIgualQueLosDeJavac` (el dex sale **byte a byte** el mismo sobre
nuestras clases que sobre las de javac: **2148 bytes**), `TestElCharConstantValueEsEntero` (tag `Integer`,
valor 90) y `TestLasFormasDexDelChar`—, y la ejecución: `Ver.letras(char[], int)` en el banco
(`new char[] {'a','b','c'}`, `'A' + n`, el `(char)`, la comparación y `v[0] = c`) da **32** por los cinco
caminos, con `dexdump` verificando `rc 0` en los cuatro dex.

## 88. El entorno del shell y el banco por `sudo` sin `rish`

Medido el 2026-11-08 tras el reset de fábrica. El shell arranca con `HOME=/home/leonisauro`, que no
tiene `Develop/` ni `Android/`; el repo y el SDK viven bajo `/data/data/com.termux/files/home`. Sin
corregirlo, `studio/e2e.sh` no encuentra `../apkc` ni `~/Develop/Go/apkc-harness/zipcheck.py` y falla
con `ENOENT` aunque el código esté bien:

```sh
sh -c 'HOME=/data/data/com.termux/files/home PREFIX=/data/data/com.termux/files/usr \
  ANDROID_HOME=/data/data/com.termux/files/home/Android \
  APKC=/data/data/com.termux/files/usr/tmp/apkc-gate \
  APKC_WASM=/data/data/com.termux/files/usr/tmp/apkc-wasm.wasm \
  sh studio/e2e.sh'
```

`tools/entorno.sh` fija esas cuatro variables y ofrece `test`, `wasm`, `gate`, `banco` y `doctor`.

Sin `rish`, el banco corre por `sudo`, pero `app_process64` no hereda el entorno de Android y aborta con
`Check failed: !location.empty() BOOTCLASSPATH and DEX2OATBOOTCLASSPATH must not be empty` (rc 134). La
copia que funciona es la de `zygote64`:

```sh
sudo sh -c 'export CLASSPATH=/data/local/tmp/bnuestro.dex; \
  eval "$(tr "\0" "\n" < /proc/$(pidof zygote64)/environ | \
  grep -E "^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH|SYSTEMSERVERCLASSPATH|STANDALONE_SYSTEMSERVER_JARS)=" | \
  sed "s/^/export /")"; /system/bin/app_process64 /data/local/tmp p.Arranque'
```

Da `rc=102` y la misma salida que la JVM; `banco/banco.sh` lo hace solo cuando `RISH=sudo`. El `sudo`
que vale es **`/usr/sbin/sudo`** (el de Termux, `$PREFIX/bin/sudo`, no encuentra un `su` que soporte
`-c`).

## 89. El literal `X.class`: `ldc` de CONSTANT_Class, `getstatic` del primitivo y `if_acmp*`

Sonda `Clase` (diez métodos y el `<clinit>`), medida con `javac --release 8 -g:lines,source`,
`javap -c -p -v`, `d8 --debug`, el `dexdump -d` del aparato y la ejecución por el banco:

```sh
cd /data/data/com.termux/files/usr/tmp/clase2
javac --release 8 -g:lines,source sonda/Clase.java
javap -c -p -v sonda/Clase.class
d8 --debug --min-api 28 --output d8 sonda/Clase.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/Clase.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
d8 --debug --min-api 28 --output d8desde nuestra/Clase.class
cmp d8/classes.dex d8desde/classes.dex
/apex/com.android.art/bin/dexdump -d nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

En el `.class`:

| fuente | forma | qué fija |
| --- | --- | --- |
| `String.class`, `Object.class` | `ldc #7 // class java/lang/String` | `ldc` de una entrada `CONSTANT_Class` (tag 7) |
| `String[].class` | `ldc #9 // class "[Ljava/lang/String;"` | el nombre de la clase es el descriptor del arreglo |
| `int[].class` | `ldc #20 // class "[I"` | también para el arreglo de un primitivo |
| `int.class`, `void.class` | `getstatic java/lang/Integer.TYPE:Ljava/lang/Class;` / `Void.TYPE` | **no** es `ldc`: es el campo `TYPE` del envoltorio |
| `o.getClass() == String.class` | `invokevirtual Object.getClass; ldc String.class; if_acmpne` | `if_acmpne` **0xa6**; `if_acmpeq` **0xa5** es el `!=` |
| `String.class.getName()` | `ldc; invokevirtual Class.getName:()Ljava/lang/String;` | el tipo que sale del literal es `Ljava/lang/Class;` |

En el dex, medido con `dexdump -d` sobre nuestro fichero:

| `.class` | palabra | decodificada |
| --- | --- | --- |
| `ldc` de `String.class` | `1c00 0400` | `const-class v0, Ljava/lang/String;` —**0x1c**, formato 21c |
| `ldc` de `String[].class` | `1c00 0900` | `const-class v0, [Ljava/lang/String;` |
| `ldc` de `int[].class` | `1c00 0800` | `const-class v0, [I` |
| `getstatic Integer.TYPE` | `6200 0100` | `sget-object v0, Ljava/lang/Integer;.TYPE:Ljava/lang/Class;` —**0x62** |
| `if_acmpne` | `3310 0400` | `if-ne v0, v1, +0004` —**0x33** de dos registros; `if_acmpeq` baja a `if-eq` **0x32** |

Los dos dex de este fork —sobre nuestras clases y sobre las de javac— son **1412 bytes** y `cmp` mudo;
`d8 --debug` sobre cualquiera de las dos da **1512 bytes** y los dos ficheros de `d8` son idénticos, con
`rc 0`. La diferencia de tamaño con el nuestro es el bloque de anotaciones y la forma del `debug_info`
ya aceptada en el 2G.4 y el §53, no el cuerpo.

La medición destapó un defecto del lector de clases: `constantPool` solo guardaba `tag[i]` en la rama
`else`, así que una `CONSTANT_Class` (y una `Utf8`) quedaban con etiqueta **0** y el traductor no podía
distinguir el `ldc` de una clase. Corregido en `apkc/classfile.go`; a partir de ahí el `ldc` de clase
llega a `const-class`.

Implementado: el parser reconoce `X.class`, `primitivo.class` y `T[].class` (`literalDeClase` en
`apkc/javafuente.go`); el emisor escribe `ldc` de `CONSTANT_Class` o el `getstatic` del envoltorio y
resuelve `==`/`!=` entre referencias con `if_acmpeq`/`if_acmpne` (`literalDeClase` y
`comparacionDeReferencias` en `apkc/javaemisor.go`); el caminante y los marcos conocen los dos opcodes;
y el traductor baja el `ldc` de clase a `const-class` 0x1c y las dos comparaciones a `if-eq`/`if-ne`
(`apkc/dexinstr.go`).

Oráculos: `apkc/javaemisor_clase_test.go` —`TestLaClaseSeEscribeComoJavac` (los diez cuerpos con pila,
huecos y filas de línea; el índice del pool se normaliza porque javac interna `String` al leer el campo
y este emisor al escribir el `<clinit>`, y el oráculo de bytes es el dex), `TestLaClaseSeDexaIgualQueLosDeJavac`
(el dex byte a byte idéntico: **1412 bytes**) y `TestLasFormasDexDeLaClase` (los opcodes 0x1c y 0x62)—, y
la ejecución: `Ver.clases()` en el banco (el `ldc`, el `getstatic` del primitivo, el arreglo y las dos
comparaciones) devuelve **2** por los cinco caminos, con `dexdump` verificando `rc 0` en los cuatro dex.

## 90. Las llamadas de más de cinco palabras: `invoke-*/range` (3rc)

Sonda `Seis`: un estático con seis enteros, otro con dos `long` (ocho palabras) y un virtual con
receptor y seis enteros (siete palabras). Medida con `javac --release 8 -g:lines,source`, `d8 --debug`,
`dexdump -d` y la ejecución por el banco:

```sh
cd /data/data/com.termux/files/usr/tmp/seis
javac --release 8 -g:lines,source sonda/Seis.java
d8 --debug --min-api 28 --output d8 sonda/Seis.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/Seis.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
d8 --debug --min-api 28 --output d8desde nuestra/Seis.class
cmp d8/classes.dex d8desde/classes.dex
/apex/com.android.art/bin/dexdump -d nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

Las palabras de `d8`, leídas con `dexdump -d`:

| llamada | palabras | decodificada |
| --- | --- | --- |
| estática de seis enteros | `7706 0600 0000` | `invoke-static/range {v0…v5}`: opcode **0x77**, `AA=6` palabras, método `@6`, primer registro `0` |
| estática con dos `long` | `7708 0100 0000` | `AA=8`: los dos pares cuentan dos palabras cada uno |
| virtual de receptor + seis | `7407 0200 0000` | `invoke-virtual/range` **0x74**, `AA=7`; el receptor es el **primer** registro del bloque |

El formato 3rc es: primera unidad `op | AA<<8` (AA = palabras), segunda el índice del método, tercera
el primer registro del bloque contiguo. `d8` usa el bloque que ya tiene; este fork, cuando los
argumentos no son contiguos, pide un bloque nuevo por encima de todo lo vivo (`tomaBloque`), copia
cada valor en orden con `move`/`move-object`/`move-wide` (12x, o 22v `/from16` si el destino pasa de
v15) y emite el 3rc con el receptor el primero. En `llamaInst` eso da `{v6…v12}` en vez del
`{v0…v6}` de `d8`; las dos formas son legales y ART ejecuta la nuestra.

La primera versión puso el receptor al final del bloque y ART la rechazó:
`int p.Ver.seisInstLlama() failed to verify: [0x6] tried to get class from non-reference register v0
(type=BooleanConstant)`. El verificador lee el primer registro como receptor; el orden del bloque es
parte de la instrucción, no un detalle del asignador.

Los dos dex de este fork —sobre nuestras clases y sobre las de javac— son **1032 bytes** y `cmp` mudo;
`d8 --debug` da **1132 bytes** sobre cualquiera de las dos y los dos ficheros de `d8` son idénticos,
con `rc 0`. La diferencia de tamaño es de nuevo el bloque de anotaciones/debug ya aceptado.

Implementado: `invoca` elige 35c o 3rc según las palabras o si un registro no cabe en un nibble;
`invocaRango` escribe el 3rc; `tomaBloque` reserva el bloque y `mueveRegistro` copia con 12x o 22v
(`apkc/dexinstr.go`). El banco añade `seis`, `seisLlama`, `seisInst`, `seisInstLlama`, `seisAncho` y
`seisAnchoLlama`; `java p.Arranque` y los cinco caminos dan `21|21|21`, con `dexdump` verificando
`rc 0` en los cuatro dex.

## 91. Constructores escritos a mano: `super`/`this`, los inicializadores de campo y dos `code_item`

Sonda `Base`/`Ctor` (un `super(x)` explícito, un `this(2)` y un campo con inicializador) y
`Base2`/`Simple` (el `super()` implícito). Medida con `javac --release 8 -g:lines,source`,
`javap -c -p -v`, `d8 --debug`, `dexdump` y el banco:

```sh
cd /data/data/com.termux/files/usr/tmp/ctor
javac --release 8 -g:lines,source sonda/Base.java sonda/Ctor.java
javap -c -p -v sonda/Ctor.class
d8 --debug --min-api 28 --output d8 sonda/Base.class sonda/Ctor.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/Base.java sonda/Ctor.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
/apex/com.android.art/bin/dexdump -f nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

En el `.class`:

| constructor | forma | qué fija |
| --- | --- | --- |
| `Ctor()` con `this(2)` | `aload_0; iconst_2; invokespecial <init>(I)V; return` | delegar **no** baja los inicializadores de campo |
| `Ctor(int)` con `super(x)` | `aload_0; iload_1; invokespecial Base.<init>(I)V; aload_0; iconst_1; putfield a; …` | el `super` va primero y después `a = 1`, en la línea de la declaración del campo |
| `Simple()` sin llamada | `aload_0; invokespecial Base2.<init>(); aload_0; iconst_1; putfield a; …` | el `super()` implícito lleva la línea de la **firma** del constructor |
| descriptor | `()V` / `(I)V`; flags `0x0000` | un constructor vuelve `V`, no el nombre de la clase |

Líneas medidas: `Ctor()` → `line 5: 0`, `line 6: 5`; `Ctor(int)` → `line 8: 0` (super), `line 2: 5`
(inicializador), `line 9: 10`, `line 10: 17`; `Simple()` → `line 3: 0` (super implícito), `line 2: 4`,
`line 4: 9`, `line 5: 14`.

En el dex: `invokespecial` baja a `invoke-direct` 0x70 y cada inicializador es un `iput` con el
`putfield` del `.class`. Nuestro dex sobre nuestras clases y sobre las de javac es **860 bytes** y `cmp`
mudo; `d8 --debug` da **968 bytes** sobre cualquiera de las dos y los dos ficheros de `d8` son
idénticos. `dexdump -f` verifica con `rc 0`.

**El defecto que destapó el banco**: `areaCodigo` y `areaDebug` guardaban sus mapas por
`[clase, nombre]`, sin el descriptor. Con dos `<init>` distintos, los dos métodos apuntaban al mismo
`code_item` (los dos salían con `code=0x9c8`), y `dexdump` cortaba con
`Non-zero padding 3 before section of type 8195 at offset 0x11c8` y ART con
`No pending exception expected: java.lang.ClassNotFoundException: p.Arranque`. El doble oráculo no lo
veía porque las clases de javac colisionaban igual; lo destapó ejecutar el dex en el aparato. La clave
pasa a `[clase, nombre, descriptor]` (`apkc/dexarma.go` y `apkc/dexdebug.go`) y hay una prueba que
exige un `code_item` por método con cuerpo (`TestLosConstructoresSobrecargadosTienenCodeItemPropio`).

Implementado: el parser lee `super(...)` y `this(...)` (`parrafo 91`), `metodoDeClassFile` deja de
rechazar `<init>` y escribe la delegación, el `super()` implícito y los inicializadores en el orden de
javac; `metodo` deja el retorno vacío en un constructor y ya no consume dos veces el nombre. El banco
añade a `Ver` los dos constructores y `sumaCtor()`: `new Ver().sumaCtor()` da **9** por los cinco
caminos, porque `Ver()` delega en `Ver(3)` (`campoCtor = 5`, `otroCtor = 4`).

## 92. `throws`, interfaces propias, métodos abstractos y `invokeinterface`

Sonda `I`/`Impl`/`Uso`/`A`: una interface con una constante y dos métodos sin cuerpo, una clase que la
implementa, una llamada por la interface y un método con uno y con dos `throws`; una clase abstracta
con un método abstracto. Medida con `javac --release 8 -g:lines,source`, `d8 --debug`, `dexdump` y el
banco:

```sh
cd /data/data/com.termux/files/usr/tmp/iface
javac --release 8 -g:lines,source sonda/I.java sonda/Impl.java sonda/Uso.java sonda/A.java
javap -c -p -v sonda/I.class sonda/Impl.class sonda/Uso.class sonda/A.class
d8 --debug --min-api 28 --output d8 sonda/I.class sonda/Impl.class sonda/Uso.class sonda/A.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/I.java sonda/Impl.java sonda/Uso.java sonda/A.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
/apex/com.android.art/bin/dexdump -d nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

En el `.class`:

| pieza | forma | qué fija |
| --- | --- | --- |
| interface `I` | flags **0x0601** (`PUBLIC INTERFACE ABSTRACT`), `super java/lang/Object`, sin `<init>` | una interface no lleva constructor ni `ACC_SUPER` |
| campo `I.N` | flags **0x0019** y `ConstantValue: int 3` | en una interface el campo es `public static final` aunque el fuente no lo diga |
| `I.doble`, `I.nada` | flags **0x0401**, **sin** `Code` | los métodos de una interface son `public abstract` |
| `Impl` | `interfaces: 1`, flags **0x0021** | la clase declara la interface y conserva `ACC_SUPER` |
| `A` y `A.x` | clase **0x0421**, método **0x0401** sin `Code` | una clase abstracta conserva `ACC_SUPER` y añade `ACC_ABSTRACT` |
| `Uso.falla` / `falla2` | `Code` y después `Exceptions` con 1 / 2 clases | el atributo cuenta y nombra `java/io/IOException` y `java/lang/IllegalStateException` |
| `Uso.usa` | `invokeinterface #7, 2  // InterfaceMethod I.doble:(I)I` | la llamada se decide por el tipo de la interface, no por la clase del receptor |

En el dex: `invoke-interface` **0x72** en 35c, medido como `7220 0200 0100` (`{v1, v0}`,
`method@2`); los métodos abstractos y la interface no llevan `code_item`; el `Exceptions` no viaja al
dex (es una comprobación de compilación). Nuestro dex sobre nuestras clases y sobre las de javac es
**1172 bytes** y `cmp` mudo; `d8 --debug` da **1504 bytes** sobre cualquiera de las dos y los dos
ficheros de `d8` son idénticos. `dexdump -d` verifica con `rc 0`.

Tres defectos del camino, destapados por la sonda: (a) `throws` no estaba en `palabrasJava`, así que
el tokenizador lo leía como identificador y el cuerpo del método fallaba con «se esperaba `{` y hay
`throws`»; (b) un cuerpo vacío `{ }` se confundía con un método abstracto y se rechazaba; (c)
`accesosDe` no devolvía `ACC_INTERFACE` para una interface del propio fuente, así que `Uso.usa` salía
como `invokevirtual`. Corregidos los tres.

Implementado: el parser lee `throws` (lista en `jmetodo.lanza`), `interface` y su `extends` múltiple,
y marca `abstracto` en el `;` o en la interface (`apkc/javafuente.go`); el emisor escribe los flags, el
`Exceptions`, los campos implícitos de la interface y el `invokeinterface`
(`apkc/javaemisor.go`). Oráculos: `apkc/javaemisor_iface_test.go` (flags, `Code`, `Thrown`,
`interfaces` y dex byte a byte) y el banco, donde `Ver implements Operable` y
`Ver.usa(new Ver())` devuelve **6** y `Ver.conThrows()` **5** por los cinco caminos; `Arranque.main`
declara `throws IOException` y también pasa por nuestro compilador.

## 93. Recursos binarios: `res/drawable` y `mipmap` con PNG, `roundIcon`

El enlace acepta un PNG (o cualquier binario) en `res/drawable` y `res/mipmap`: el fichero viaja
literal al apk y su entrada de la tabla es un `Res_value` de tipo cadena con la ruta. Medido con
`aapt2 dump resources` y el empaquetado de la página:

```sh
cd /data/data/com.termux/files/usr/tmp/icono/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o app.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources app.apk
unzip -l app.apk
unzip -p app.apk res/drawable/icono.png | sha256sum
sha256sum res/drawable/icono.png
# y el mismo proyecto por nuestro enlace, sin aapt2 ni zipalign:
apkc build --linker go --compiler go --signer go --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
aapt2 dump resources build/app.apk
```

`aapt2 dump resources` del paquete que escribe este fork:

```text
type drawable id=01 entryCount=1
  resource 0x7f010000 drawable/icono
    () (file) res/drawable/icono.png type=PNG
type mipmap id=02 entryCount=1
  resource 0x7f020000 mipmap/ic_launcher
    () (file) res/mipmap/ic_launcher.png type=PNG
type layout id=03 entryCount=1
...
```

El PNG del zip y el del proyecto tienen el mismo `sha256sum`; `roundIcon="@mipmap/ic_launcher"` es un
atributo del manifiesto y no necesita nada especial. El `resources.arsc` de este fork mide 928 bytes y
el de `aapt2` 1060 en el mismo proyecto: la diferencia es la segunda configuración `layout-v1` que
`aapt2` añade a los layouts y este enlace sigue emitiendo una sola (diferencia ya escrita en el §28).
La entrada `drawable` y la `mipmap` son las mismas en los dos.

`studio/e2e.sh` lo comprueba de punta a punta: crea un proyecto con `res/drawable/icono.png`,
`res/mipmap/ic_launcher.png`, `values-es` y `roundIcon`, compila con `--linker go --compiler go`, y
exige que `aapt2 dump resources` declare las dos entradas y el PNG viaje literal; `apksigner` y
`zipalign -c -p 4` aprueban el apk (`icono: apk de 10185 bytes`).

## 94. `values-es`: language, `spec_map`, offset ausente y orden del pool

Dos sondas de dos idiomas, comparadas **byte a byte** con el `resources.arsc` de `aapt2`:

```sh
cd /data/data/com.termux/files/usr/tmp/idioma2/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
# idioma3: la misma comparación con una clave que solo existe en values-es
```

Las dos salen **idénticas**: 728 bytes en `idioma2` (base `app_name`, `otro`; es `app_name`) y 716 en
`idioma3` (base `s1`, `s2`; es `s1`, `s3`). Lo que fija el fichero:

| pieza | valor |
| --- | --- |
| `ResTable_config` de `es` | tamaño 64 y `language[0]='e'`, `language[1]='s'` en los offsets +8 y +9 |
| bloque base | tamaño 64 y el resto a cero |
| `spec_map` | **0x0004** (locale) en la clave que tiene `es`; 0 en la que no |
| clave ausente en `es` | offset **0xffffffff** en el bloque `es`, con `entryCount` igual al de la base |
| clave solo en `es` | `aapt2` la **elimina** (`removing resource … without required default value`) y no entra ni en la tabla ni en el pool |
| orden del pool global | la base ordenada por texto y después cada idioma ordenado por texto (`Icono`, `Uno`, `Icono ES`; `Dos`, `Uno`, `Uno ES`) |

Implementado: `fuentesDelProyecto` acepta `values-xx` de dos letras y guarda el calificador en
`fuenteXML.config`; `tablaDeRecursos` agrupa por tipo y configuración, calcula el `spec_map` 0x0004,
escribe el offset ausente y descarta las claves sin base; `escribeBloqueTipo`/`configDe` escriben la
configuración y el escritor admite varios bloques por tipo (`apkc/linker.go`, `apkc/valores.go`,
`apkc/arscwriter.go`). Prueba: `apkc/valores_es_test.go` compara el arsc de 728 bytes con el fixture
de `aapt2`; la segunda sonda de 716 bytes se midió igual y se comprueba en `studio/e2e.sh`.

**Frontera escrita**: `values-xx` es el único calificador implementado. `layout-land`,
`drawable-hdpi` y `values-<región>` piden las máscaras de orientación/densidad/país, que no están
medidas; se rechazan por nombre. El criterio de cierre «dos idiomas en `res/values`» sí queda cubierto.

## 95. La app del criterio de cierre, compilada y firmada sin `aapt2` ni `javac`

El criterio de la Fase 5 se juntó en un proyecto `cierre`: un `MainActivity` con **bucle sobre un
arreglo**, **`switch`**, **`try`/`catch`**, un icono en `res/drawable` y dos idiomas en
`res/values`/`res/values-es`. Compilado y firmado por el camino todo-Go de la página:

```sh
cd /data/data/com.termux/files/usr/tmp/cierre/proj
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" \
  --keystore "$HOME/.android/debug.keystore"
apksigner verify --verbose --min-sdk-version 21 build/app.apk
zipalign -c -p 4 build/app.apk
aapt2 dump resources build/app.apk
```

El apk mide **9861 bytes**, `apksigner` dice v1 y v2 correctos, `zipalign -c -p 4` sale 0 y `aapt2
dump resources` declara `drawable/icono (file) res/drawable/icono.png type=PNG` y la cadena
`string/app_name` con `()` «Cierre» y `(es)` «Cierre ES». El dex de 2012 bytes trae el bucle, el
`switch` y el `try`/`catch` del fuente.

Lo que **no** está hecho: instalarlo y arrancarlo en el teléfono. Eso exige `rish` o `pm install` por
`sudo`, y es una acción sobre el aparato que se pide al usuario antes de hacerla (memoria del repo:
`feedback-telefono-por-rish`). Hasta entonces la Fase 5 se queda **abierta**: el hecho 2 del objetivo
—instala y arranca en un teléfono real— ya se midió en el §55 con el contador, pero no con esta app de
cierre.

## 96. `string-array`, `integer-array` y `plurals` en un `values`

El enlace de recursos acepta los valores compuestos de un `values`: `string-array`, `integer-array` y
`plurals`, además de `bool`, `color` e `integer` que ya estaban. La tabla de la sonda sale **byte a
byte** la de `aapt2` (1152 bytes):

```sh
cd /data/data/com.termux/files/usr/tmp/tipos/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
aapt2 dump resources build/app.apk
```

`aapt2 dump resources`:

```text
type array id=01 entryCount=2
  resource 0x7f010000 array/nombres
    () (array) size=2
      ["uno", "dos"]
  resource 0x7f010001 array/numeros
    () (array) size=2
      [1, 2]
type plurals id=05 entryCount=1
  resource 0x7f050000 plurals/cuenta
    () (plurals) size=2
      one="uno"
      other="varios"
```

Lo que fija el binario (medido con el lector del §24 y `aapt2 dump`):

| pieza | valor |
| --- | --- |
| entrada de array | compleja, `padre=0`, flags `0x0001`, un registro por elemento |
| nombre del registro | `0x01000000 \| (índice + 1)` (0x01000001, 0x01000002, …) |
| valor de `string-array` | tipo cadena 0x03 con el índice del pool global |
| valor de `integer-array` | tipo entero decimal 0x10 con el número |
| plural | compleja, registros con los ids de cantidad de Android: other 0x01000004, zero 0x01000005, one 0x01000006, two 0x01000007, few 0x01000008, many 0x01000009 |
| orden de los plurales | **el del fuente** (one antes que other en la sonda), no ordenado por id |

Implementado en `apkc/valores.go`: `itemsDeArray`/`itemsDePlural` leen los hijos, y
`tablaDeRecursos` añade los textos de los registros al pool global y los resuelve al escribir.
Prueba: `apkc/valores_tipos_test.go` (fixture de 1152 bytes de `aapt2`).

**Frontera**: `fraction`, `attr` y `styleable` siguen fuera; se rechazan por nombre. `bool`, `color` e
`integer` ya estaban desde el §29.

## 97. `menu`, `xml` y `raw`: AXML de raíz libre y fichero literal

`res/menu` y `res/xml` se enlazan como los layouts (son AXML), y `res/raw` como un fichero binario.
Medido con `aapt2 dump resources` y el empaquetado de la página:

```sh
cd /data/data/com.termux/files/usr/tmp/otros/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources aapt2.apk
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
aapt2 dump resources build/app.apk
unzip -l build/app.apk
```

`aapt2 dump resources` del apk de este fork:

```text
type id id=01 entryCount=1
  resource 0x7f010000 id/accion
type menu id=02 entryCount=1
  resource 0x7f020000 menu/principal
    () (file) res/menu/principal.xml type=XML
type raw id=03 entryCount=1
  resource 0x7f030000 raw/dato
    () (file) res/raw/dato.txt
type string id=04 entryCount=1
  resource 0x7f040000 string/app_name
    () "Otros"
type xml id=05 entryCount=1
  resource 0x7f050000 xml/prefs
    () (file) res/xml/prefs.xml type=XML
```

El `menu` y el `xml` viajan como AXML (`res/menu/principal.xml`, `res/xml/prefs.xml`) y el `raw` viaja
con sus bytes (`res/raw/dato.txt`). `aapt2` añade además `res/menu-v1/principal.xml` por la misma
configuración por API que ya se acepta en el §28; este enlace emite una sola. La tabla y el `R.java`
llevan los tipos `menu`, `raw` y `xml` como cualquier otro recurso.

Implementado en `apkc/linker.go` (se aceptan las carpetas, `menu`/`xml` van a la lista de AXML y
`raw` a los ficheros) y `apkc/valores.go`/`apkc/enlazar.go` (el tipo sale de la ruta). Prueba:
`apkc/linker_otros_test.go` exige los tres tipos, el AXML de los dos XML y los bytes del `raw`.

**Frontera**: el `<permission>` del manifiesto sigue rechazado, porque `aapt2` escribe además un
`Manifest.java` que este enlace no genera.

## 98. La versión de platform: API → versión de Android en la cabecera AXML

La tabla que usa `nombreDeVersionDeAndroid` para `compileSdkVersionCodename` y
`platformBuildVersionName` se midió con `aapt2 link --compile-sdk-version-code`/
`--compile-sdk-version-name` y el volcado de la cabecera:

```sh
cd /data/data/com.termux/files/usr/tmp/tipos/proj
for par in '35 15' '33 13' '32 12L' '31 12' '28 9' '27 8.1'; do
  set -- $par
  aapt2 link -o v$1.apk --manifest AndroidManifest.xml \
    --compile-sdk-version-code "$1" --compile-sdk-version-name "$2" \
    -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
  aapt2 dump xmltree v$1.apk --file AndroidManifest.xml | grep -E 'platformBuildVersion(Name|Code)'
done
```

La tabla implementada es `27→8.1`, `28→9`, `29→10`, `30→11`, `31→12`, `32→12L`, `33→13`, `34→14`
y `35→15`. La 34/14 se había medido en el §37; las demás se comprobaron con el comando de arriba.
Cuando el nombre no es numérico (`8.1`, `12L`), `platformBuildVersionName` se escribe como **cadena**,
que es lo que hace aapt2; con un nombre numérico sigue siendo entero. Cualquier API fuera de la tabla
se rechaza por nombre y se puede pasar con `--codename`. Prueba: `TestVersionesDeAndroidMedidas`.

## 99. Una anónima dentro de un método `static` sin captura

La mitad de 5.14 que no necesita capturar locales: una anónima creada en un método `static` no lleva
`this$0`. Medido con `javac --release 8 -g:lines,source`, `javap -c -p -v` y el doble oráculo:

```sh
cd /data/data/com.termux/files/usr/tmp/cap/javac
javac --release 8 -g:lines,source Captura.java
javap -c -p -v Captura.class 'Captura$1.class'
# y la sonda por el emisor/dexer del fork:
cd /data/data/com.termux/files/home/Develop/Go/apkc-web
tools/entorno.sh -- sh -c 'cd apkc && go test -count=1 -run "AnonimaEnStatic" .'
```

`Captura$1` de javac tiene constructor **`()V`** (`aload_0; invokespecial Object.<init>; return`), **sin**
campo `this$0` y con `EnclosingMethod` apuntando a `Captura.uno`. Nuestra clase y su dex salen byte a
byte los de javac; los tests comparan el `Code` de `<init>`/`run`, las filas de línea y el dex.

## 100. La captura de locales effectively-final: `val$x`, el ctor y `MethodParameters`

La segunda mitad de 5.14: una anónima captura un parámetro/local del método que la crea. Se midieron
los dos casos, la captura en un método `static` y la que además lleva el `this$0` del exterior:

```sh
cd /data/data/com.termux/files/usr/tmp/cap2 && javac --release 8 -g:lines,source Captura.java
javap -c -p -v 'Captura$1.class'
cd /data/data/com.termux/files/usr/tmp/cap3/javac && javac --release 8 -g:lines,source Captura3.java
javap -c -p -v 'Captura3$1.class'
cd /data/data/com.termux/files/home/Develop/Go/apkc-web
tools/entorno.sh -- sh -c 'cd apkc && go test -count=1 -run "Captura" .'
```

| caso | constructor | campo y `MethodParameters` |
| --- | --- | --- |
| `Captura.dos(int x)` (static) | **`(I)V`**; `aload_0; iload_1; putfield val$x; aload_0; invokespecial Object.<init>; return` | `val$x` final synthetic (0x1010); **sin** `MethodParameters` |
| `Captura3.r(int x)` (instancia) | **`(LCaptura3;I)V`**; asigna `this$0` y `val$x` antes del `super` | campos en orden `val$x` y después `this$0`; `MethodParameters` con dos filas: el exterior `final mandated` (0x8010) y `val$x` `final synthetic` (0x1010) |

La referencia `x` dentro de `run` sale `aload_0; getfield val$x` (y `x + y` añade el `getfield` del
campo del exterior). Nuestras clases y sus dex salen byte a byte los de javac en los dos casos; las
pruebas comparan campos, `Code`, filas de línea y dex.

Implementado: el parser registra las locales del método (`jmetodo.locales`), al entrar en la anónima
resuelve un nombre contra ellas y lo convierte en `this.val$nombre`, añade el campo `val$…` y guarda
la captura; el emisor pone los campos en el orden de javac, el constructor con sus parámetros y
`MethodParameters`, y `new` pasa las capturas antes de los argumentos (`apkc/javafuente.go`,
`apkc/javaemisor.go`). Frontera: una anónima dentro de otra sigue rechazada por nombre.

## 101. `Signature` genérica en el dex: el troceado de `d8`

El dexer copia el atributo `Signature` del `.class` a la anotación de sistema
`Ldalvik/annotation/Signature;`, con la firma troceada en un arreglo de cadenas. Medido en una clase
con firma de clase, de campo y de método:

```sh
cd /data/data/com.termux/files/usr/tmp/gen
javac --release 8 -g:lines,source Gen.java
d8 --debug --min-api 28 --output d8 Gen.class
apex/com.android.art/bin/dexdump -a d8/classes.dex
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex .
/apex/com.android.art/bin/dexdump -a nuestro.dex
```

`dexdump -a` del fichero de este fork, igual que el de `d8`:

```text
Annotations on class
  VISIBILITY_SYSTEM Ldalvik/annotation/Signature; value={ "<T::" "Ljava/lang/CharSequence;" ">" "Ljava/lang/Object;" }
Annotations on field #0 'xs'
  VISIBILITY_SYSTEM Ldalvik/annotation/Signature; value={ "Ljava/util/List<" "Ljava/lang/String;" ">;" }
Annotations on method #1 'id'
  VISIBILITY_SYSTEM Ldalvik/annotation/Signature; value={ "<R:" "Ljava/lang/Object;" ">(TR;)TR;" }
```

La regla de troceado, deducida de los tres casos y de
`()Ljava/util/Map<Ljava/lang/String;Ljava/lang/Integer;>;`:
cada descriptor de clase `L…;` es un trozo; si el descriptor va seguido de `<`, el trozo termina en
`<`; todo lo que queda entre descriptores (`<T::`, `>`, `()`, `>;`) es un trozo. El arreglo se añade
como `value` de la anotación de clase, campo o método.

Implementado: el lector guarda `Signature` de clase, campo y método (`apkc/classreader.go`);
`firmaEnTrozos`/`anotacionDeFirma` y las listas de campos anotados en `apkc/dexanota.go`; el
directorio de anotaciones escribe `fields_size` y los `field_idx` además de clases y métodos.
Prueba: `apkc/dexanota_firma_test.go` (los cuatro troceados y las tres reglas).

## 102. El `<permission>` del manifiesto y su `Manifest.java`

`aapt2` no rechaza un manifiesto con `<permission>`: además del AXML escribe
`<paquete>/Manifest.java` con las constantes. Medido con `aapt2 link --java`:

```sh
cd /data/data/com.termux/files/usr/tmp/perm/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o app.apk --manifest AndroidManifest.xml --java ../java \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
cat ../java/com/perm/Manifest.java
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
cat src/com/perm/Manifest.java
```

El fichero que escribe este fork es el de aapt2, byte a byte salvo la ausencia de salto final (nosotros
también lo omitimos): cabecera `AUTO-GENERATED`, `package com.perm;`, `public final class Manifest`,
`public static final class permission` y una constante por permiso en el orden del manifiesto:

```text
public static final String CAMARA="com.perm.CAMARA";
public static final String RED="com.perm.RED";
```

Implementado: `manifestJavaDe` recorre los `<permission android:name=…>` del árbol del manifiesto y
`enlazarConGo` escribe `src/<paquete>/Manifest.java` junto a la `R.java`; el cambio de contenido entra
en la detección de recompilación. Prueba: `TestManifestJavaDePermissions`. Queda fuera cualquier
atributo de `<permission>` distinto de `android:name`.

## 103. `layout-land` y `drawable-hdpi`: orientación y densidad

Las dos configuraciones que faltaban de 5.17. Se comparó contra `aapt2 link --no-auto-version`, que
es la forma que coincide con la configuración única de este enlace (el `aapt2` por defecto añade
además una copia `layout-v1` de cada layout, ya aceptada en el §28):

```sh
cd /data/data/com.termux/files/usr/tmp/cal/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link --no-auto-version -o noauto.apk --manifest AndroidManifest.xml \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources noauto.apk
unzip -p noauto.apk resources.arsc > noauto.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp noauto.arsc nuestro.arsc
```

Los dos arsc son **1204 bytes idénticos**. Lo que fija:

| configuración | `ResTable_config` | spec |
| --- | --- | --- |
| `layout-land` | orientation `2` en +12 | `0x0080` |
| `drawable-hdpi` | density `240` en +14 y sdkVersion `4` en +24 | `0x0500` |
| `values-es` | language 'e','s' en +8/+9 | `0x0004` |

La ruta de la densidad lleva el sufijo que usa `aapt2`: `res/drawable-hdpi-v4/icono.png`; la de
orientación, `res/layout-land/main.xml`. `aapt2 dump resources` del apk de este fork declara las dos
entradas. Implementado en `apkc/linker.go` (acepta las carpetas y reescribe la ruta de densidad),
`apkc/arscwriter.go` (los bytes de configuración) y `apkc/valores.go` (`mascaraConfig`). Prueba:
`apkc/valores_cal_test.go` (fixture de 1204 bytes).

## 104. Estado de 5.21 (`v3`) y 5.22 (`invokedynamic`)

**5.21 queda en parte.** `apkc verify` no verifica una firma v3, pero la tolera y lo dice: el cotejo
de `studio/e2e.sh` firma el apk base con `--v3-signing-enabled true` y exige que las dos herramientas
lo acepten y que `apkc verify` anote `no se verificó aquí`.

```sh
sh -c 'cd studio && APKC=... APKC_WASM=... sh e2e.sh' | grep con-v3
# con-v3: apksigner si, apkc verify si
grep -n "idParV3\|no se verificó aquí" apkc/verificar.go
```

Lo que falta es **escribir** v3 (y v4): el bloque `APK Signature Scheme v3` y su prueba de rotación
no están implementados. La pestaña sigue firmando con el keystore de depuración; un keystore de
release propio y `--min-sdk` del proyecto tampoco están en el camino de la página.

**5.22 no se implementa aquí.** El lector de `.class` rechaza `invokedynamic` con nombre, y el
compilador de fuente no tiene lambdas ni referencias de método; implementar la desugración de `d8`
(externalizar `lambda$…`, `method_handle`/`method_type` y la tabla de bootstrap) es la pieza XL del
plan y no se ha medido. El rechazo es deliberado:

```sh
grep -n "invokedynamic" apkc/classreader.go
# pool entry %d is an invokedynamic: lambdas and dynamic concatenation are not dexed here
```

Por tanto la Fase 5 no se declara cerrada: quedan `attr`/`styleable` (5.18), las regiones de 5.17,
la firma v3 escrita (5.21) y `invokedynamic` (5.22).

## 105. `fraction`: radix, 1.0 y el bit de `%p`

El `<fraction>` de un `values` sale tipo **0x06** con un entero de 32 bits. Medido con nueve valores
(0, 10, 25, 50, 75, 100, 150, 50p y 100p) y comparado byte a byte con aapt2:

```sh
cd /data/data/com.termux/files/usr/tmp/frac/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
```

El arsc sale **796 bytes idéntico**. La codificación medida:

| valor | datos |
| --- | --- |
| `0%` | `0x00000000` |
| `10%` | `0x0CCCCD30` (radix 3; el valor de 24 bits se redondea hacia arriba) |
| `25%`, `50%`, `75%` | `0x20000030`, `0x40000030`, `0x60000030` |
| `100%` | `0x00000100` (radix 0) |
| `150%` | `0x00C00020` (radix 2) |
| `50%p`, `100%p` | el mismo valor con el bit bajo a 1 (`0x…31` / `0x101`) |

Implementado: `fraccionDeTexto` en `apkc/valores.go` calcula en `float32` (10% solo coincide así),
redondea el valor de 24 bits con `math.Ceil` y conserva el bit de `%p`. Prueba:
`apkc/valores_fraccion_test.go` (fixture de 796 bytes).

## 106. `attr` y `declare-styleable`

`<attr format="…">` produce una entrada compleja del tipo `attr`; `<declare-styleable>` no produce
entrada en `resources.arsc`, solo la clase `styleable` de `R.java`. Medido con
`aapt2 dump resources` y comparado byte a byte:

```sh
cd /data/data/com.termux/files/usr/tmp/attrs/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml --java ../java \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources aapt2.apk
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
```

El arsc sale **596 bytes idéntico**. La entrada de un attr es compleja (`padre=0`, flags `0x0001`,
un registro) con nombre `0x01000000` (`attr/format`) y tipo entero 0x10; los bits del formato medidos
son:

| format | bits |
| --- | --- |
| `string`, `reference`, `integer`, `boolean` | `0x01`, `0x02`, `0x04`, `0x08` |
| `color`, `float`, `dimension`, `fraction` | `0x10`, `0x20`, `0x40`, `0x80` |

El `R.java` de este fork lleva la clase `styleable` con el arreglo de ids y los offsets; el arreglo se
escribe `new int[] {…}` (y sin `final`) porque el emisor de este fork todavía no da `ConstantValue` a
un arreglo, pero Java lo acepta y `R.styleable.Vista`/`Vista_modo` quedan utilizables. Para el arreglo
de ids hizo falta el `const` 0x14 de 31 bits en el dexer (las ids `0x7f010000` no caben en
`const/high16`).

Implementado: `attrDeElemento`/`styleableDeElemento`/`formatoDeAttr` en `apkc/valores.go`, la clase
`styleable` en `apkc/r.go` y `const` 0x14 en `apkc/dexinstr.go`. Prueba:
`apkc/valores_attrs_test.go` (arsc de 596 bytes y el texto de `R.java`). Frontera: `format="enum"` y
`format="flags"` con sus hijos no están medidos.

## 107. `values-es-rMX`: país dentro de la configuración

Una región añade `country` a la configuración del idioma. Medido con `aapt2` y comparado byte a byte:

```sh
cd /data/data/com.termux/files/usr/tmp/region/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
```

El arsc de **680 bytes** es idéntico: `language[0]='e'` y `language[1]='s'` en +8/+9, `country[0]='M'` y
`country[1]='X'` en +10/+11; la máscara del spec sigue siendo `0x0004` (locale). Implementado en
`fuentesDelProyecto` (reconoce `values-xx-rYY`) y `configDe` (`apkc/arscwriter.go`). Prueba:
`apkc/valores_region_test.go`.

## 108. APK Signature Scheme v3, escrito y verificado por `apksigner`

El paquete Go escribe ahora el par v3 (`0xf05368c0`) junto al v2. El signer v3 se midió sobre un apk
que firmó `apksigner --v3-signing-enabled true` y después se reprodujo:

```sh
cd /data/data/com.termux/files/usr/tmp/region/proj
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" \
  --keystore "$HOME/.android/debug.keystore"
apksigner verify --verbose --min-sdk-version 21 build/app.apk
```

Salida:

```text
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
```

La forma medida del signer v3:

| campo | valor |
| --- | --- |
| `signed data` | `digests` y `certificates`, **cada uno con su longitud**, y después `minSdk` 24, `maxSdk` 0x7fffffff y la lista de atributos vacía (u32 0) |
| signer | `signed data` con longitud, minSdk/maxSdk crudos, las firmas con longitud y la clave pública con longitud |
| bloque | un par v2 y otro v3 dentro del mismo APK Signing Block |

El defecto del primer intento fue no envolver `digests`/`certs` en su longitud (faltaban 8 bytes) y
poner la lista de atributos **fuera** del signed data: `apksigner` respondía `Invalid SDK Version
parameter(s)` y `No signatures`. Corregidos los dos, verifica v3. Prueba:
`TestElBloqueV3LlevaSuParYSuSigner` (pares y longitudes del signer); `studio/e2e.sh` exige
`v3 scheme … true` en el apk del proyecto y en el del icono.

## 109. `invokedynamic`: el coste medido de la desugración y por qué queda fuera

Sonda `Lam` con una lambda estática, una referencia de método y una lambda que captura `this`:

```sh
cd /data/data/com.termux/files/usr/tmp/lam/sonda
javac --release 8 -g:lines,source Lam.java
cd .. && d8 --debug --min-api 28 --output d8 sonda/Lam.class sonda/Lam\$*.class
/apex/com.android.art/bin/dexdump -d d8/classes.dex
/apex/com.android.art/bin/dexdump -a d8/classes.dex
```

`d8` no deja el `invokedynamic`: **genera tres clases** y dos métodos sintéticos. Lo medido:

| pieza | qué escribe `d8` |
| --- | --- |
| `Lam$0` | clase `PUBLIC FINAL SYNTHETIC`, campo `f$0:LLam;`, constructor `(LLam;)V`, `f(I)I` que llama a `lambda$captura$1$Lam` |
| `Lam$1` | clase para `Lam::doble`, constructor `()V` y `f` que llama a `doble` |
| `Lam$2` | clase para `x -> x + 1`, `f` que llama a `lambda$uno$0` |
| `lambda$uno$0` / `lambda$captura$1$Lam` | métodos sintéticos en `Lam`, el segundo con el sufijo de la captura |
| anotación | `Lcom/android/tools/r8/annotations/LambdaMethod;` con `holder`, `method` y `proto` |
| bootstrap | `LambdaMetafactory.metafactory` con `samMethodType`, el `MethodHandle` de implementación y `instantiatedMethodType`, leídos del atributo `BootstrapMethods` |

Para emitirlo desde este dexer haría falta leer `CONSTANT_InvokeDynamic` (tag 18), el atributo
`BootstrapMethods`, el `MethodHandle` y los `MethodType`; sintetizar una clase por sitio con campos,
constructor, método SAM y anotación; inyectar los `lambda$…` en la clase dueña; y sustituir la
instrucción por `new`/`dup`/capturas/`invokespecial`. Es la pieza XL del plan (5.22), no un ítem de
una sesión; el lector de `.class` la sigue rechazando por nombre:

```text
pool entry %d is an invokedynamic: lambdas and dynamic concatenation are not dexed here
```

No se declara hecha. La Fase 5 queda abierta por esta pieza (y por el keystore de release/v4 de
5.21).


## 110. `invokedynamic` de una lambda estática sin capturas, desugrado y ejecutado

El único caso de 5.22 que se implementa: `x -> x + 1` dentro de un método `static`. `javac
--release 8` deja el sitio con `LambdaMetafactory.metafactory`, el `MethodType` SAM, el
`MethodHandle` del implementador y el `MethodType` instanciado; este fork lee el sitio y lo
convierte en una clase sintética con constructor y método SAM.

```sh
cd /data/data/com.termux/files/usr/tmp/lamok
javac --release 8 -g:lines,source -d clases src/Lam.java
apkc dex -o lamok.dex clases
dexdump -d lamok.dex
sh banco/lambda.sh
```

`dexdump -d` de nuestro fichero:

```text
Class descriptor  : 'LLam$$Lambda$0;'
  name          : '<init>'
000220: 7010 0800 0000  invoke-direct {v0}, Ljava/lang/Object;.<init>:()V
  name          : 'f'
000238: 7110 0400 0200  invoke-static {v2}, LLam;.lambda$uno$0:(I)I
...
  name          : 'uno'
00026c: 2200 0100           new-instance v0, LLam$$Lambda$0;
000270: 7010 0000 0000      invoke-direct {v0}, LLam$$Lambda$0;.<init>:()V
```

`banco/lambda.sh` compila con `javac`, dexa con este fork y ejecuta el `.dex` en el aparato con
`app_process64`; la salida es la misma que la JVM y `dexdump` lo verifica:

```text
  jvm (javac): rc=0, salida: 42
  aparato: rc=0, salida: 42
  dexdump verificando: rc 0
lambda: la lambda estatica sin capturas se desugro y corre igual que la JVM
```

Lo que se mide y lo que se rechaza:

| pieza | qué hace este fork |
| --- | --- |
| `CONSTANT_InvokeDynamic` (tag 18), `BootstrapMethods`, `MethodHandle` (15) y `MethodType` (16) | se leen; el `invokedynamic` deja de rechazarse |
| bootstrap `LambdaMetafactory.metafactory` con 3 argumentos | se acepta si el SAM y el tipo instanciado coinciden (nada de SAM genérico) |
| implementador | `REF_invokeStatic` de la propia clase; a un `private static synthetic` se le quita `private`, como hace `d8` |
| clase sintética | `PUBLIC FINAL SYNTHETIC`, la interfaz SAM, `<init>()V` y el método SAM que llama al implementador |
| sitio en el llamador | `new-instance` + `invoke-direct {v}, <init>()V`; el valor queda en el mismo registro |
| capturas (`(I)`, `(LLam;)`) y `this::m` | se desugran con campos de captura en el §111 |
| referencias sin receptor (`Tipo::m`) | se desugran en el §112 |
| SAM genérico y borrado a una cota | se desugran con el descriptor borrado y `checkcast` en los §113 y §114 |
| `altMetafactory` serializable y con marcadores | se desugra con las interfaces extra en el §115 |
| concatenación dinámica (`makeConcatWithConstants`) | se desugra a `StringBuilder` en el §116 |
| puentes de `altMetafactory` y constantes dinámicas | se rechazan por nombre |

Implementado en `apkc/lambda.go` (expansión, validación y clase sintética), `apkc/classreader.go`
(`BootstrapMethods` y los tags del pool), `apkc/dextabla.go` (internado del sitio),
`apkc/dexinstr.go` (el `new`/`invoke-direct`) y `apkc/jvmopcode.go` (el ancho 0xba). Pruebas:
`apkc/lambda_test.go` (fixture de `javac`, estructura de la clase y del dex) y `banco/lambda.sh`
(ejecución en ART; desde el §111 cubre también las capturas y `this::m`). El compilador de fuente de
este fork sigue sin aceptar lambdas: esto mide el dexer sobre `.class` de `javac`, no la sintaxis.

## 111. Lambdas con capturas: un campo por valor y el receptor de `this::m`

La lambda que usa una local o `this` llega con esos valores como parámetros del descriptor del sitio
(`(I)LLamLoc$F;`, `(LLamThis;)LLamThis$F;`). La clase sintética los guarda en campos, el constructor
los recibe y el método SAM los carga antes de los argumentos, igual que hace `d8` con sus `f$0`
(§109). Se midieron cuatro casos con `javac --release 8` → `apkc dex` → ART:

```sh
sh banco/lambda.sh
```

Salida:

```text
  Lam: jvm rc=0 «42» · aparato rc=0 «42»
  LamLoc: jvm rc=0 «42» · aparato rc=0 «42»
  LamThis: jvm rc=0 «42» · aparato rc=0 «42»
  LamRef: jvm rc=0 «42» · aparato rc=0 «42»
  dexdump verificando: rc 0
lambda: las cuatro lambdas se desugraron y corren igual que la JVM
```

Lo que fija cada caso:

| caso | sitio | sintética | implementador |
| --- | --- | --- | --- |
| `x -> x + 1` (static) | `()LLam$F;` | `()V`, sin campos | `REF_invokeStatic lambda$uno$0(I)I` |
| captura un local `y` | `(I)LLamLoc$F;` | `(I)V`, campo `I` | `REF_invokeStatic lambda$con$0(II)I` (captura primero) |
| captura `this` | `(LLamThis;)LLamThis$F;` | `(LLamThis;)V`, campo `LLamThis;` | `REF_invokeSpecial lambda$captura$0(I)I`; se le quita `private` y el SAM usa `invoke-virtual` |
| `this::suma` | `(LLamRef;)LLamRef$F;` | `(LLamRef;)V`, campo `LLamRef;` | `REF_invokeVirtual LamRef.suma(I)I` |

El orden es el del JVM: el receptor de una referencia de instancia es la **primera** captura, las
demás capturas van en los campos y después los argumentos del SAM. En el llamador, el `invokedynamic`
se cambia por `new-instance` y `invoke-direct {objeto, capturas…}, <init>`, con las capturas que ya
estaban en la pila.

Pruebas: `apkc/lambda_test.go` (los cuatro casos) y `banco/lambda.sh`. La referencia sin receptor se
mide en el §112.

## 112. Referencia a un método de instancia sin receptor (`Tipo::m`)

`LamUnb::suma` no captura nada: el sitio es `()LLamUnb$F;` y el **primer parámetro del SAM** es el
receptor (`(LLamUnb;I)I`). La clase sintética no lleva campos; su método SAM carga todos los
argumentos y llama con `invoke-virtual`, usando el primero como objeto. Medido y ejecutado:

```sh
sh banco/lambda.sh
```

```text
  LamUnb: jvm rc=0 «42» · aparato rc=0 «42»
  dexdump verificando: rc 0
lambda: las cinco lambdas se desugraron y corren igual que la JVM
```

La validación distingue los dos caminos del receptor: si hay capturas, la primera tiene que ser el
dueño (`this::m`); si no hay capturas, el primer parámetro del SAM tiene que ser el dueño
(`Tipo::m`), y los parámetros del implementador son los del SAM menos ese receptor. Prueba:
`TestLaReferenciaSinReceptor` en `apkc/lambda_test.go` y el caso `LamUnb` de `banco/lambda.sh`.

Sigue fuera: nada de lo que `javac` emite para una lambda. El SAM genérico se mide en el §113,
`altMetafactory` en el §115 y la concatenación dinámica de JDK 9 en el §116.

## 113. SAM genérico: descriptor borrado y `checkcast` al tipo instanciado

`Function<String,Integer> f = s -> s.length()` tiene dos tipos en el bootstrap: el **borrado**
`(Ljava/lang/Object;)Ljava/lang/Object;`, que es el que implementa la interfaz, y el **instanciado**
`(Ljava/lang/String;)Ljava/lang/Integer;`, que es el del implementador. La clase sintética declara
el SAM borrado y, antes de llamar, moldea el argumento:

```sh
sh banco/lambda.sh
```

```text
  LamGen: jvm rc=0 «4» · aparato rc=0 «4»
  dexdump verificando: rc 0
lambda: las seis lambdas se desugraron y corren igual que la JVM
```

`dexdump -d` del SAM de la sintética:

```text
  name          : 'apply'
000230: 1f02 0600    check-cast v2, Ljava/lang/String;
000234: 7110 0300 0200 invoke-static {v2}, LLamGen;.lambda$main$0:(Ljava/lang/String;)Ljava/lang/Integer;
```

La validación admite que el parámetro borrado sea `Object` y el instanciado una referencia; el
implementador se comprueba contra el tipo instanciado. En el cuerpo, cada parámetro que difiere se
carga con su tipo del SAM y se moldea con `checkcast` (0xc0) antes de la llamada; la vuelta usa el
tipo del SAM (`Object` acepta el `Integer` del implementador, que ya viene encajado por `javac`).
Prueba: `TestElSamGenericoUsaElDescriptorBorrado` y el caso `LamGen` de `banco/lambda.sh`.

Frontera: el borrado a un tipo que no sea `Object` se mide en el §114; `altMetafactory`
(serializables y marcadores) en el §115 y la concatenación dinámica de JDK 9 en el §116.

## 114. Borrado a una cota (`T extends Number`) y el orden del `annotation_set`

Una interfaz genérica con cota borra a esa cota: `F<T extends Number>` deja el SAM
`(Ljava/lang/Number;)Ljava/lang/String;` y el tipo instanciado `(Ljava/lang/Integer;)…`; la clase
sintética implementa el SAM con `Number` y su cuerpo moldea a `Integer` antes de llamar.

```sh
sh banco/lambda.sh
```

```text
  LamBound: jvm rc=0 «42» · aparato rc=0 «42»
  dexdump verificando: rc 0
lambda: las siete lambdas se desugraron y corren igual que la JVM
```

La validación ya no exige que el borrado sea `Object`: acepta cualquier par de tipos de referencia
en el parámetro y moldea con `checkcast`; los primitivos tienen que coincidir. El constructor y los
campos no cambian respecto del §113.

**Defecto que destapó esta sonda**: el `annotation_set_item` tiene que ir **ordenado por `type_idx`**
y este fork escribía las anotaciones en el orden de las reglas. Una clase con `Signature` y
`EnclosingClass`/`InnerClass` (la interfaz genérica del caso) salía con el conjunto desordenado y
`dexdump` la rechazaba con `Out-of-order entry types: 4 then 2`. Ahora `armaAnotaciones` ordena cada
conjunto por `type_idx` antes de escribirlo (`apkc/dexanota.go`). Sin eso, el dex ni siquiera se
verificaba.

Prueba: `TestElSamConCotaUsaElDescriptorBorrado` (descriptor `Number`, `checkcast` y dex) y el caso
`LamBound` de `banco/lambda.sh`, que pasa por `dexdump` y por `app_process64`.

## 115. `altMetafactory`: lambda serializable y marcadores de una intersección

Cuando el tipo destino no es una interfaz sola, `javac` cambia el bootstrap a
`LambdaMetafactory.altMetafactory`. Los tres primeros argumentos son los de `metafactory` (SAM,
implementador y tipo instanciado) y detrás vienen los `flags`, la cuenta de marcadores con sus
clases y la cuenta de puentes con sus `MethodType`. Los bits medidos son `1` (serializable), `2`
(marcadores) y `4` (puentes).

```sh
P=/data/data/com.termux/files/usr
W=$P/tmp/lamser
cat >"$W/src/LamSer.java" <<'J'
public class LamSer {
    interface R extends Runnable, java.io.Serializable {}
    static R make() { return () -> System.out.println("hola"); }
    public static void main(String[] a) { make().run(); }
}
J
"$P/lib/jvm/java-21-openjdk/bin/javac" --release 8 -g:lines,source -d "$W/clases" "$W/src/LamSer.java"
"$HOME/Android/build-tools/34.9.0/d8" --release --min-api 21 --output "$W/d8" \
  "$W/clases/LamSer.class" "$W/clases/LamSer\$R.class"
/apex/com.android.art/bin/dexdump "$W/d8/classes.dex"
```

`javap -v` del sitio de `LamSer` deja `()V`, el `MethodHandle` del implementador, `()V`, `5` y `0`:
`5` es serializable **y** puentes, y la cuenta de puentes es cero. `d8` no escribe ni `writeReplace`
ni `$deserializeLambda$`: borra el `$deserializeLambda$` que dejó `javac` y añade
`java/io/Serializable` a las interfaces de la clase sintética. El orden que escribe es el del sitio,
luego los marcadores y `Serializable` al final:

```text
  Class descriptor  : 'LLamSer$0;'
  Interfaces        -
    #0              : 'LLamSer$R;'
    #1              : 'Ljava/io/Serializable;'
```

Con una intersección de dos marcadores, `(R & Runnable & Serializable & Cloneable)`, `javac` deja
`flags 7` y los marcadores `R` y `Cloneable`; `d8` los lista en ese orden detrás del tipo del sitio:

```text
    #0              : 'Ljava/lang/Runnable;'
    #1              : 'LLamSer3$R;'
    #2              : 'Ljava/lang/Cloneable;'
    #3              : 'Ljava/io/Serializable;'
```

Este fork hace lo mismo: lee los flags y los marcadores, añade esas interfaces a la clase sintética,
quita `$deserializeLambda$` del dueño y deja el dex sin `writeReplace`, como `d8`. Un `altMetafactory`
con puentes distintos de cero se rechaza por nombre, porque no está medido. La lambda corre en ART
junto a las siete anteriores:

```sh
APKC=$P/tmp/apkc-gate sh banco/lambda.sh
```

```text
  LamSer: jvm rc=0 «hola» · aparato rc=0 «hola»
  LamSerCap: jvm rc=0 «hola7» · aparato rc=0 «hola7»
  LamMarcas: jvm rc=0 «x» · aparato rc=0 «x»
  dexdump verificando: rc 0
lambda: las diez lambdas se desugraron y corren igual que la JVM
```

`LamSerCap` captura un `int` y demuestra que el flag serializable no cambia el campo ni el
constructor; `LamMarcas` es la intersección con dos marcadores. Pruebas:
`TestLaLambdaSerializableAceptaAltMetafactory`, `TestLaLambdaSerializableConCapturas` y
`TestLosMarcadoresDeLaInterseccion` en `apkc/lambda_test.go`, más los tres casos de
`banco/lambda.sh` que pasan por `dexdump` y `app_process64`.

Frontera: la serialización de ida y vuelta no se mide; se mide que el dex se construya y ejecute
como el de `d8`. Los puentes de `altMetafactory` siguen fuera y se rechazan por nombre; la
concatenación dinámica de JDK 9 se mide en el §116.

## 116. `makeConcatWithConstants`: la concatenación dinámica de JDK 9

Desde Java 9, `javac` no escribe `StringBuilder` para `a + b + c`: deja un `invokedynamic` cuyo
bootstrap es `java/lang/invoke/StringConcatFactory.makeConcatWithConstants`. El primer argumento del
bootstrap es una **receta**: cada `\u0001` consume el siguiente argumento dinámico y cada `\u0002` la
siguiente constante de la lista que viene detrás. Con `--min-api 21`, `d8` lo baja otra vez a
`new-instance StringBuilder`, un `append` por trozo y `toString`.

```sh
P=/data/data/com.termux/files/usr
W=$P/tmp/lamser
cat >"$W/src/ConcatB.java" <<'J'
public class ConcatB {
    public static void main(String[] a) {
        int n = a.length;
        String r = "a" + "\u0001" + "b" + "\u0002" + "c" + n;
        System.out.println(r.length() + ":" + r.equals("a\u0001b\u0002c" + n));
    }
}
J
"$P/lib/jvm/java-21-openjdk/bin/javac" --release 11 -g:lines,source -d "$W/clases" "$W/src/ConcatB.java"
"$P/lib/jvm/java-21-openjdk/bin/javap" -v -cp "$W/clases" ConcatB
"$HOME/Android/build-tools/34.9.0/d8" --release --min-api 21 --output "$W/d8catb" "$W/clases/ConcatB.class"
/apex/com.android.art/bin/dexdump -d "$W/d8catb/classes.dex"
```

`javap -v` deja la receta `\u0002\u0001` y una sola constante, `a\u0001b\u0002c`. `\u0002` aparece
justo cuando el texto contiene los marcadores; sin ellos, `javac` dobla el literal dentro de la
propia receta y la lista de constantes queda vacía. `d8` escribe:

```text
0001: new-instance v0, Ljava/lang/StringBuilder;
0003: const-string v1, "a\001b\002c"
0005: invoke-direct {v0, v1}, Ljava/lang/StringBuilder;.<init>:(Ljava/lang/String;)V
0008: invoke-virtual {v0, v5}, Ljava/lang/StringBuilder;.append:(I)Ljava/lang/StringBuilder;
000c: invoke-virtual {v0}, Ljava/lang/StringBuilder;.toString:()Ljava/lang/String;
```

El constructor lleva el primer texto cuando la receta empieza por literal o por constante; si empieza
por `\u0001`, `d8` usa el constructor vacío. Los `append` que elige son los del tipo del argumento:
`(I)`, `(J)`, `(F)`, `(D)`, `(C)`, `(Z)`, `(Ljava/lang/String;)` y, para una referencia cualquiera,
`(Ljava/lang/Object;)`. `javac` convierte antes las referencias y los arreglos con
`String.valueOf`, así que en la práctica el descriptor solo trae `String` y primitivos.

Este fork trocea la receta igual y baja cada sitio a `new-instance` + `<init>()V` + un `append` por
trozo + `toString`, con el constructor vacío siempre y sin encadenar el `move-result-object` de cada
`append`: el comportamiento es el mismo que el de `d8`, aunque la forma no sea idéntica. Está en
`apkc/concat.go`; el sitio se valida en `expandirLambdas` (`apkc/lambda.go`) y se guarda en
`classFull.Concats`, el internador lo sigue en `dextabla.collectCode` y el traductor lo despacha en
`dexinstr.go`. Se rechazan por nombre la receta que no cuadra con el descriptor o con las
constantes, las constantes dinámicas del bootstrap y cualquier otro bootstrap de
`StringConcatFactory`.

La ejecución en ART, junto a las diez lambdas del §115:

```sh
APKC=$P/tmp/apkc-gate sh banco/lambda.sh
```

```text
  Concat: jvm rc=0 «n=0!» · aparato rc=0 «n=0!»
  ConcatB: jvm rc=0 «6:true» · aparato rc=0 «6:true»
  ConcatW: jvm rc=0 «ztruecxj2f1.5d2.5i0ssoobj» · aparato rc=0 «ztruecxj2f1.5d2.5i0ssoobj»
  dexdump verificando: rc 0
lambda: las trece lambdas y concatenaciones se desugraron y corren igual que la JVM
```

`Concat` es la forma básica; `ConcatB` es la receta con `\u0002`; `ConcatW` conserva un argumento
dinámico de cada tipo y comprueba que el dex nombra los siete `append`. Pruebas:
`TestLaConcatenacionConConstantesDelBootstrap` y `TestLaConcatenacionEligeElAppendDeCadaTipo` en
`apkc/concat_test.go`, más los tres casos de `banco/lambda.sh` que pasan por `dexdump` y
`app_process64`.

Frontera: los puentes de `altMetafactory` con cuenta distinta de cero y las constantes dinámicas de
la concatenación siguen fuera y se rechazan por nombre; `javac` no produce ninguna de las dos.

## 117. Un keystore de release en la pestaña: contraseña, alias y certificado

La página ya tenía un selector de fichero, pero solo pasaba `--keystore`: los valores por defecto
(`--storepass android`, `--keyalias androiddebugkey`) solo abren un keystore de depuración. Un
keystore de release normal tiene otra contraseña y otro alias, y fallaba en uno de los dos sitios:

```sh
P=/data/data/com.termux/files/usr
KEYTOOL=$P/lib/jvm/java-21-openjdk/bin/keytool
"$KEYTOOL" -genkeypair -keystore "$P/tmp/studio-release.p12" -storetype PKCS12 \
  -storepass saludo -keypass saludo -alias lanzable -keyalg RSA -keysize 2048 \
  -validity 3650 -dname "CN=Release de prueba, O=apkc-web, C=ES"
apkc sign --keystore "$P/tmp/studio-release.p12" --out /tmp/x.apk app.apk
# [sign] ... (PKCS#12): the keystore did not open with the given storepass: ... (wrong storepass?)
apkc sign --keystore "$P/tmp/studio-release.p12" --storepass saludo --out /tmp/x.apk app.apk
# [sign] ... holds 1 key(s), the nearest name being "lanzable", and none is androiddebugkey: check --keyalias
```

Ahora `apkc keystore` lista las identidades del fichero con los mismos lectores de PKCS#12 y JKS
del firmante, sin proceso ni JVM, y la página la usa para enseñar el certificado antes de firmar:

```sh
apkc keystore --keystore "$P/tmp/studio-release.p12" --storepass saludo
# keystore-json: {"keys":1,"entries":[{"alias":"lanzable","subject":"CN=Release de prueba,O=apkc-web,C=ES","sha256":"BD:03:7F:CB:4F:D8:79:38:2F:6A:EF:8D:75:0C:62:C2:B6:ED:DA:1B:05:AF:9C:49:78:C4:A9:90:C5:6D:84:F6","notAfter":"2036-11-06"}]}
```

La huella es la misma que imprime `apksigner`, y la salida es **una sola línea con prefijo** para
que la página la parsee sin limpiar los códigos de color de las demás. Una contraseña mala se nombra
como contraseña (`did not open with the given storepass`), no como un fichero corrupto; un alias que
no está se nombra con el más parecido.

La página pide la contraseña en cada firma (no la guarda), elige el alias entre los que declara el
fichero —y lo exige si hay más de uno—, enseña sujeto, huella y validez, y pasa
`--storepass`/`--keyalias` tanto a `apkc sign` como a `build --linker go --compiler go`. «Olvidar»
borra el fichero de OPFS. La prueba de la pestaña genera el keystore de release con `keytool`, lo
mete por la página, firma y apksigner lee el certificado elegido:

```sh
sh -c 'cd studio && APKC_WASM=$P/tmp/apkc-wasm.wasm ... KS_RELEASE=$P/tmp/studio-release.p12 node prueba-pestana.js'
apksigner verify --verbose --print-certs --min-sdk-version 21 "$P/tmp/pestana-apk/pestanera-release.apk"
```

```text
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
V3.0 Signer: certificate DN: CN=Release de prueba, O=apkc-web, C=ES
V3.0 Signer: certificate SHA-256 digest: bd037fcb4fd879382f6aef8d750c62c2b6edda1b05af9c4978c4a990c56d84f6
```

`studio/e2e.sh` lo exige y lo resume:

```text
pestaña release: apk de 10352 bytes firmado con CN=Release de prueba y verificado por apksigner (v1+v2+v3)
```

Pruebas: `TestInspectarKeystoreListaAliasSujetoYHuella`, `TestInspectarKeystoreNombraLaContrasenaMal`
y `TestInspectarKeystoreLeeJKSYListaSuAlias` en `apkc/keystorelist_test.go`; el caso nuevo de
`studio/prueba-pestana.js` conduce la página (lista, certificado, firma) y `studio/e2e.sh` corre
`apksigner` sobre el apk que sale. Frontera: v4, la rotación v3.1, las claves EC, el `.aab` y el
`minSdk` del proyecto en el firmante v3 quedan como piezas propias (§5.29-§5.35 del plan).

## 118. Un `.aab` en la pestaña: firma JAR v1, sin bloque de apk

Un bundle no es un apk: es un zip con `BundleConfig.pb` y sus módulos, se firma como un JAR y no
lleva bloque de firmas v2/v3 ni pasa por `zipalign`. `apkc sign` lo detecta por la extensión y escribe
solo la v1; el manifiesto va **el primero** porque `JarInputStream` lo busca ahí y `jarsigner` avisa
si no está (`Manifest is missing when reading via JarInputStream`).

```sh
P=/data/data/com.termux/files/usr
W=$P/tmp/aabtest
mkdir -p "$W/base/dex" "$W/base/manifest"
printf 'BundleConfig' > "$W/BundleConfig.pb"
printf 'manifest' > "$W/base/manifest/AndroidManifest.xml"
printf 'dex' > "$W/base/dex/classes.dex"
(cd "$W" && python3 -m zipfile -c in.aab BundleConfig.pb base/dex/classes.dex base/manifest/AndroidManifest.xml)
apkc sign --keystore "$P/tmp/studio-release.p12" --storepass saludo --keyalias lanzable \
  --out "$W/out.aab" "$W/in.aab"
jarsigner -verify "$W/out.aab"
apkc verify "$W/out.aab"
```

```text
[sign] out.aab carries a v1 JAR signature of its own, 3 entries taken over as their bytes already were
jar verified.
[verify] v1 (JAR signing): ok — 3 entradas con su digest y la firma PKCS#7 verificada
[verify] el paquete es un .aab: la firma es JAR v1 y no lleva bloque v2/v3
```

`jarsigner` sigue avisando de que el certificado es auto-firmado y de que no hay timestamp, como con
cualquier clave de prueba; lo que ya no dice es que el manifiesto falte. `apkc verify` reconoce el
`.aab` por la extensión, verifica la v1 con su propio lector y **no** le pide una v2 que el formato no
tiene; `apksigner` no es oráculo aquí porque espera un apk.

La pestaña usa la misma sección de firma: si la ruta de entrada termina en `.aab`, el destino es
`firmado.aab`, el MIME de descarga es `application/octet-stream` y, tras firmar, la propia página
ejecuta `apkc verify` y enseña el veredicto. `studio/e2e.sh` crea un `.aab` mínimo con `python3 -m
zipfile`, lo firma desde la pestaña con el keystore de release y exige:

```text
pestaña .aab: 2340 bytes, v1 JAR verificada por jarsigner y apkc verify, sin bloque v2/v3
```

Pruebas: `TestUnBundleSeFirmaComoJarYManifiestoPrimero` y
`TestSignDistingueElBundlePorLaExtension` en `apkc/bundle_sign_test.go`, más el caso `.aab` de
`studio/prueba-pestana.js`. Construir el bundle (la tabla de recursos en protobuf, `BundleConfig.pb`
y los módulos) es 5.33; aquí solo se firma el que ya existe.

## 119. Firma v4 (`.idsig`): árbol de verity y la firma que va al lado del apk

La v4 no va dentro del apk: es un fichero `.idsig` al lado, el que usa `adb install
--incremental` en Android 11+. Se midió sobre uno que escribe `apksigner
--v4-signing-enabled true` y se reproduce con estos campos, todos little-endian y los
variables con su longitud en u32:

```text
u32 version = 2
bytes hashingInfo:      u32 hashAlgorithm = 1 (SHA-256), u8 log2BlockSize = 12,
                        bytes salt (vacía), bytes rawRootHash (32)
bytes signingInfos:     bytes apkDigest (32), bytes certificate (DER), bytes additionalData,
                        bytes publicKey (SubjectPublicKeyInfo DER), u32 sigAlg = 0x0103,
                        bytes signature
bytes verityTree:       el árbol de fs-verity
```

El `apkDigest` **no** se recalcula sobre el fichero final: es el mismo digest que va dentro
del bloque v3, y por eso el firmante pasa el que ya calculó para v2/v3. La firma es
RSASSA-PKCS1-v1_5/SHA-256 sobre esta carga, que también incluye el tamaño del apk y el
árbol:

```text
u32 tamaño; u64 fileSize; u32 hashAlgorithm; u8 log2BlockSize;
bytes salt; bytes rawRootHash; bytes apkDigest; bytes certificate; bytes additionalData
```

El árbol es el de fs-verity: hojas de 4096 bytes (la última rellena con ceros), cada nivel
partido en bloques de 4096 y relleno hasta un múltiplo de 4096, y el nivel superior el
primero en el fichero. El nivel superior es el primero cuyos hashes caben en 4096 bytes; su
SHA-256 es `rawRootHash`. Para el apk de 12756 bytes de la sonda son cuatro hojas, el nivel
0 (128 bytes de hashes) relleno a 4096, y `rawRootHash = 50081f11…a17c`, el mismo que el
`.idsig` de `apksigner`.

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
W=$P/tmp/v4probe
apkc sign --v4 --keystore "$P/tmp/studio-release.p12" --storepass saludo \
  --keyalias lanzable --out "$W/ours.apk" "$W/in.apk"
"$BT/apksigner" verify --v4-signature-file "$W/ours.apk.idsig" "$W/ours.apk"
apkc verify "$W/ours.apk"
```

```text
[sign] ours.apk carries a v1+v2+v3 signature of its own, 4 entries taken over as their bytes already were
[verify] v1 (JAR signing): ok — 4 entradas con su digest y la firma PKCS#7 verificada
[verify] v2 (APK Signature Scheme v2): ok — 7 entradas cubiertas por el digest del bloque
[verify] v4 (APK Signature Scheme v4): ok — árbol de verity y firma del .idsig
```

`apksigner verify --v4-signature-file` sale 0 con el `.idsig` que escribe este fork, tanto
sobre el apk mínimo como sobre uno de 678823 bytes (varios niveles de árbol), y también sale
0 al revés: `apkc verify` aprueba el `.idsig` que escribió `apksigner`. La página tiene una
casilla `escribir el .idsig` encendida por defecto, pasa `--v4=true` a `sign`/`build`, ofrece
la descarga del sidecar y su `apkc verify` lo comprueba; la prueba de la pestaña exporta el
`.idsig` y `studio/e2e.sh` lo coteja con `apksigner`:

```text
pestaña v4: .idsig de 5562 bytes, apksigner y apkc verify lo aprueban
```

Pruebas: `TestV4FirmaYVerifica`, `TestV4RechazaUnArbolCambiado` y
`TestV4DesactivadoNoEscribeIdsig` en `apkc/v4sign_test.go`, más el caso v4 de
`studio/prueba-pestana.js`. Frontera: una sal no vacía y los firmantes alternativos del
`.idsig` se rechazan por nombre; un `.aab` no lleva `.idsig` porque no tiene bloque v2/v3.

## 120. Rotación v3.1: el linaje y el bloque que escribe `apksigner`

Medido para 5.31, que sigue abierto: el comando que crea el linaje, el bloque v3.1 y el
atributo de prueba de rotación. No hay implementación todavía; esto es el formato que hay
que reproducir.

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
K=$P/lib/jvm/java-21-openjdk/bin/keytool
W=$P/tmp/v31probe
"$K" -genkeypair -keystore old.p12 -storetype PKCS12 -storepass viejito -keypass viejito \
  -alias viejo -keyalg RSA -keysize 2048 -validity 3650 -dname "CN=Viejo, O=apkc, C=ES"
"$K" -genkeypair -keystore new.p12 -storetype PKCS12 -storepass nuevito -keypass nuevito \
  -alias nuevo -keyalg RSA -keysize 2048 -validity 3650 -dname "CN=Nuevo, O=apkc, C=ES"
"$BT/apksigner" rotate --out lineage.bin \
  --old-signer --ks old.p12 --ks-key-alias viejo --ks-pass pass:viejito \
  --new-signer --ks new.p12 --ks-key-alias nuevo --ks-pass pass:nuevito
"$BT/apksigner" sign --ks old.p12 --ks-pass pass:viejito \
  --next-signer --ks new.p12 --ks-pass pass:nuevito \
  --lineage lineage.bin --rotation-min-sdk-version 33 \
  --v1-signing-enabled false --v2-signing-enabled false \
  --out rotated33.apk in.apk
"$BT/apksigner" verify --verbose --min-sdk-version 28 rotated33.apk
```

```text
Verifies
Verified using v3 scheme (APK Signature Scheme v3): true
Verified using v3.1 scheme (APK Signature Scheme v3.1): true
Number of signers: 1
```

El bloque de firmas lleva tres pares:

```text
0x1b93ad61  3285   <- v3.1, el identificador medido (no 0xf05368c1)
0xf05368c0  1437   <- v3.0, para las plataformas que no leen v3.1
0x42726577  3402   <- relleno
```

El fichero `lineage.bin` (1863 bytes) es un envoltorio con magia `0x3eff39d1`, versión 1 y
la longitud del linaje interno. El linaje interno (1851 bytes) empieza con versión 1 y una
lista de nodos; cada nodo es `signedData` con longitud, `flags`, `sigAlgorithm` y `signature`
con longitud, y su `signedData` es el certificado con longitud más los `flags`. El nuevo
nodo lo firma la clave vieja. El **valor** del atributo de prueba de rotación es el linaje
interno tal cual:

| constante | valor |
| --- | --- |
| id del bloque v3.1 | `0x1b93ad61` |
| atributo `proof-of-rotation` | `0x3ba06f8c` |
| atributo `rotation-min-sdk-version` | `0x559f8b02` |
| atributo `rotation-on-dev-release` | `0xc2a6b3ba` |
| versión de rotación por defecto | 33 (Android 13) |

Con `--rotation-min-sdk-version 28` el mismo linaje va a un bloque **v3.0** (un firmante, el
nuevo, con el atributo) y `v3.1` sale `false`; con 33 salen los dos bloques y firmantes con
rangos de SDK. Las constantes del verificador ya usan `0x1b93ad61` para v3.1; escribirlo
sigue siendo 5.31.
