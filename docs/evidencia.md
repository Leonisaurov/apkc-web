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
