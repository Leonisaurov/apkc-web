# Evidencia

Todo lo de aquí se midió en este teléfono (Termux/Android aarch64, Chromium y node 26 locales).
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
| clases | 6 (mayor version del `.class`: 52, Java 8) |
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
