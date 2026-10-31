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

