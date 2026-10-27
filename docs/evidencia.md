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
