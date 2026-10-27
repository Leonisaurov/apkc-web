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
