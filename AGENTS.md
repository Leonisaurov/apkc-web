# AGENTS.md — apkc-web

Este repo guarda **la ambición de compilar una app Android desde el navegador, del lado del
cliente**, separada de la herramienta que funciona hoy. Es documentación + un experimento + una
página. Nada de aquí cambia `../apkc` salvo lo que la Fase 0 autorice expresamente.

## Mapa

| Ruta | Qué es | Cómo se comprueba |
| --- | --- | --- |
| `docs/evidencia.md` | lo medido, cada afirmación con su comando | leerlo |
| `docs/situacion.md` | el recap de lo medido, bullet por bullet (movido desde este fichero el 2026-11-03); el original es el `§` que cada bullet cita | leerlo y comprobar que su `§` existe |
| `docs/bloqueos.md` | qué existe y qué costaría sustituir `aapt2`, `javac`, `d8` | leerlo |
| `docs/plan.md` | el plan detallado, con fases y criterios de abandono | leerlo |
| `tools/entorno.sh` | el entorno del repo en un solo sitio (`HOME`, `PREFIX`, `ANDROID_HOME`, `PATH`) y las tareas `test`, `wasm`, `gate`, `banco` y `doctor` | `tools/entorno.sh doctor` |
| `shim/navegador-sim.js` | hospedador con forma de navegador (un `fs` en memoria con la API de Node; monta ficheros reales con `--montar` y devuelve resultados con `--exportar`) | `node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp` |
| `apkc/` | **el fork de la herramienta** (subtree desde `../apkc`, MIT, pin `a909214`): donde se escribe el dexer, el compilador y los recursos en Go | `cd apkc && go test ./...` y el wasm: `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-web-wasm.wasm" .` |
| `studio/` | apkc-studio: genera proyectos apkc, traduce su salida para medir fricción **y hace correr `apkc.wasm` en la pestaña** (OPFS + worker) | `cd studio && APKC=<binario> sh e2e.sh`; la pestaña: `APKC_WASM=… node studio/prueba-pestana.js` |
| `banco/` | **el banco de ejecución**: dos fuentes de prueba y un guion que corre el mismo programa por los cinco caminos (JVM, `d8` sobre javac, `d8` sobre lo que emitimos, **nuestro dex en `app_process`** y, desde el §66, **nuestro dex con nuestro `main`**, sin javac ni d8) — sin instalar nada, sin pantalla | `sh banco/banco.sh` (necesita `APKC` compilado, `ANDROID_JAR`, `build-tools` y `rish`) |

## Reglas del repo

1. **Un claim, una medición.** Una afirmación sobre coste, compatibilidad o formato que no traiga
   el comando que la reproduce no entra en `docs/evidencia.md`: va a `docs/plan.md` como hipótesis,
   marcada como tal. Es la razón de existir de este repo.
2. **Sin binarios versionados.** `*.wasm` está ignorado: se construye desde el fork `apkc/` con
   `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .`. Tampoco se
   versiona `shim/wasm_exec.js` (es del GOROOT de cada quien).
3. **El código de la herramienta vive aquí, en `apkc/`.** Es un fork (MIT, `ajinasokan/apkc`) metido con
   `git subtree add --prefix=apkc ../apkc master --squash` sobre el commit `a909214`, en la rama `web`:
   el dexer, el compilador y los recursos se escriben **aquí**, no allende el muro. Un `git subtree pull`
   desde `../apkc` sigue siendo posible porque hay historial compartido; si algún día se hace un merge
   hacia arriba, se hace desde esta rama. Cada pieza entra con su commit, su test y su gate.
4. **Sin conexión y sin runtimes de terceros** (decisión del usuario, 2026-10-27). Todo lo que
   compila, empaqueta y firma viaja en el propio origen de la página: ni CDN de una JVM wasm, ni
   telemetría, ni fuentes. Se comprueba conduciendo chromium y registrando cada petición
   (`studio/prueba-pestana.js`); **una petición fuera del origen rompe la prueba.** Si aun así se
   evaluara un runtime ajeno, su licencia se lee **antes** de integrarla: si no permite el uso, la
   fase muere y se escribe por qué.
5. **Idioma.** Prosa y comentarios en español con la ortografía completa (tildes, ñ) y **cero
   caracteres de otros alfabetos**; pasar el chequeo antes de commitear. En `../apkc` los comentarios
   son en inglés (estilo del upstream): no arrastrar esa convención aquí, pero tampoco acentuar
   nunca texto que sea literal de una herramienta o contenido de un fichero generado.
6. **Commits** `<type>(<scope>): <summary>` (fix|feat|docs|refactor|chore|ci|test), mensaje auditado.
7. **El registro tiene tres sitios y no son intercambiables.** Una medición nueva es un `§` en
   `docs/evidencia.md`, con el comando que la reproduce. Si cambia lo que el subconjunto acepta o rechaza,
   se actualiza **la fila** del inventario de la Fase 5 en `docs/plan.md` (y la tabla Estado del plan si
   cambia el estado de una fase). Este fichero **no admite bullets de hallazgos**: reglas, mapa, entorno,
   gate y la tabla de áreas — y no pasa de ~130 líneas, porque se lee entero en cada sesión. Lo ya escrito
   no se copia aquí: `docs/situacion.md` es el recap de las mediciones.

## Entorno (Termux/Android aarch64)

- Rutas: `$PREFIX=/data/data/com.termux/files/usr`, `HOME=/data/data/com.termux/files/home`. Nunca
  `/tmp`, `/usr/bin`, `/etc`. Shebangs absolutos. El shell interactivo del usuario es fish: envolver
  asignaciones de entorno en `sh -c '...'`.
- Comando pesado por `tcr` (`tcr -j 1 -q -- <cmd>`), jamás `-j$(nproc)`. Antes de algo caro:
  `free -m` y `df -h "$PREFIX"`.
- **El shell puede arrancar con `HOME=/home/leonisauro`**, que no tiene `Develop/` ni `Android/`; el repo
  y el SDK viven bajo `/data/data/com.termux/files/home`. Los guiones que usan `$HOME/Develop` o `../apkc`
  fallan con ENOENT. `tools/entorno.sh` fija `HOME`, `PREFIX`, `ANDROID_HOME` y `PATH`, y evita repetir
  asignaciones en cada comando.
- El SDK está en `~/Android` (`platforms/android-34/android.jar`) y el JDK en
  `$PREFIX/lib/jvm/java-21-openjdk`. Las build-tools accesibles son `~/Android/build-tools/34.9.0/`, unos
  enlaces **nuevos** a los binarios del paquete (`$PREFIX/bin/{aapt,aapt2,apksigner,d8,zipalign}`): los
  enlaces viejos de `34.0.0` quedaron con el contexto SELinux del usuario anterior tras un reset de fábrica
  y son inaccesibles (ni `readlink` ni `unlink`). **El número de la carpeta no puede superar al de la
  platform**, porque apkc deriva de ahí la platform (`35.0.0` pedía `android-35`, que no existe).
  `studio/e2e.sh` y `banco/banco.sh` eligen la carpeta más alta con `sort -V`. Tras ese reset `rish`
  desapareció del PATH; el banco usa entonces **`/usr/sbin/sudo`** (el del sistema; el `sudo` de Termux no
  encuentra un `su` que soporte `-c`) y copia el `BOOTCLASSPATH` de `zygote64`, porque sin él
  `app_process64` aborta con `BOOTCLASSPATH and DEX2OATBOOTCLASSPATH must not be empty`.
- **Dos trampas de wasm ya conocidas**: (a) `go test -exec=$GOROOT/lib/wasm/go_js_wasm_exec` falla
  con `total length of command line and environment variables exceeds limit` si no se le da
  `env -i`; (b) sin `GOOS=js GOARCH=wasm` el lanzador recibe un ELF y se queja de
  `expected magic word 00 61 73 6d`.
- Oráculos que no comparten código con apkc: `apksigner verify --verbose`, `jarsigner -verify`,
  `zipalign -c -p`, `~/Develop/Go/apkc-harness/zipcheck.py`, `javap`, `java -Xverify:all` (la JVM del
  Termux ejecuta **nuestros** `.class`: destapó el `StackMapTable` que faltaba en el §61 y lo cierra en
  el §65) y
  `dexdump`/`app_process64` **del aparato por `rish` o `sudo`** (no hay `adb`): `/apex/com.android.art/bin/dexdump`
  verifica y desarma, y `app_process64` con `CLASSPATH` ejecuta un `.dex` suelto **sin instalar nada**
  (`sh banco/banco.sh` lo hace por los cinco caminos y compara el código de salida, §61).
- **Trampas de OPFS/cromium ya conocidas** (detalle completo con síntoma en `docs/evidencia.md` §12):
  `navigator.storage` no existe dentro del worker (la raíz baja desde la página por `postMessage`);
  `crypto`/`performance` son de solo lectura en un worker de módulo; `entries()` rinde
  `[nombre, mango]` y el tipo es `mango.kind`; `createSyncAccessHandle()` es **asíncrona** aunque sus
  métodos sean sincrónicos; y sin COOP/COEP no hay `crossOriginIsolated` ni handle sincrónico.

## Gate antes de dar nada por hecho

```sh
# binario wasm y la suite en hospedador de navegador (todo desde el fork apkc/)
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$PREFIX/tmp/apkc-test.wasm" .'
sh -c 'cd shim && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" . && node navegador-sim.js "$PREFIX/tmp/apkc-test.wasm"'

# la firma, del lado del cliente: un apk real entra y sale verificado por los oráculos
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd shim && node navegador-sim.js \
  --montar "$PREFIX/tmp/sign-probe/app/build/app.apk:/in.apk" \
  --montar "$HOME/.android/debug.keystore:/ks.p12" \
  --exportar "/out.apk:$PREFIX/tmp/sign-probe/wasm-signed.apk" \
  "$PREFIX/tmp/apkc-wasm.wasm" sign --keystore /ks.p12 --out /out.apk /in.apk'

# la página, de punta a punta (genera, compila de verdad, firma, verifica, y la conduce chromium)
sh -c 'cd apkc && go build -o "$PREFIX/tmp/apkc-gate" .'
sh -c 'cd studio && APKC="$PREFIX/tmp/apkc-gate" APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" sh e2e.sh'
```

`studio/e2e.sh` es innegociable antes de decir "la página funciona": su test de node solo veía
verdes seis fallos que solo aparecían al descomprimir y compilar lo generado.
Los bloques de arriba se lanzan con el entorno ya puesto por `tools/entorno.sh test|wasm|gate|banco`;
sin el wrapper, `HOME` y `PREFIX` tienen que apuntar a `/data/data/com.termux/files/home` y
`/data/data/com.termux/files/usr`.

## Lo que ya está medido, para no reabrirlo

Cuatro áreas cerradas. Cada `§` es un párrafo de `docs/evidencia.md` con el comando que lo reproduce; el
recap de lo que dice, en `docs/situacion.md`; y lo que el subconjunto de Java acepta o rechaza hoy, con
el `file:line` que rechaza, en el inventario de la Fase 5 de `docs/plan.md`. **Antes de medir algo, mirar
los dos: si ya está, no se reabre ni se escribe aquí.**

| Área | Lo que está cerrado | Dónde está la medición |
| --- | --- | --- |
| el dex (`apkc dex`) | lector y escritor, `debug_info_item`, bloque de anotaciones, el layout que exige ART (cuatro defectos que `d8` no ve), el internado de `type_list`, el marco por encima de v15 con las formas `/from16`, la anotación `Signature` genérica troceada como `d8`, y las lambdas, referencias de método y SAM genéricos desugrados (`invokedynamic` → `new`/`invoke-direct`, ejecutados en ART); el fichero del binario `GOOS=js` es idéntico byte a byte al nativo y `d8` lo re-dexa con `rc 0` | §20-§23, §55-§60, §86, §101 y §110-§114 |
| el subconjunto Java, del fuente al dex | control de flujo (`if`, bucles, `switch`, `try`/`catch`/`finally` y el valor pendiente), arreglos —también los de `long`/`float`/`double`—, los anchos `long`/`float`/`double`, el `char` con sus escapes, `private` y sus `access$`, `instanceof`, concatenación y plegado de constantes (también la promoción del literal), el literal `X.class` y `==` entre referencias, las llamadas de más de cinco palabras con `invoke-*/range`, los constructores propios con `super`/`this`, `throws`/interfaces/abstractos e `invokeinterface`, la anónima en `static` y la captura de locales (`val$x`), `StackMapTable` | §40-§54, §61-§92, §99 y §100 + inventario de la Fase 5 |
| los recursos sin `aapt2` | `resources.arsc`, los tres AXML byte a byte, `R.java`, los ids del framework sacados del `android.jar`, el enlazado desde los ficheros de texto, `build --linker go`, PNG en `drawable`/`mipmap`, `values-xx`, `string-array`/`integer-array`, `plurals`, `fraction`, `attr`/`styleable`, `menu`/`xml`, `raw`, `layout-land`, `drawable-hdpi` y `values-es-rMX` | §28-§38, §93-§107 |
| la pestaña y el aparato | `create`, `sign` y `verify` dentro de wasm; `build --linker go --compiler go` **sin levantar ningún proceso** en chromium y sin una sola petición fuera del origen; el apk instalado y arrancado en el teléfono; un `.dex` suelto ejecutado con `app_process` (`banco/`) | §7-§8, §11, §15, §44-§47, §55, §61-§66 |


