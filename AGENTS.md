# AGENTS.md — apkc-web

Este repo guarda **la ambición de compilar una app Android desde el navegador, del lado del
cliente**, separada de la herramienta que funciona hoy. Es documentación + un experimento + una
página. Nada de aquí cambia `../apkc` salvo lo que la Fase 0 autorice expresamente.

## Mapa

| Ruta | Qué es | Cómo se comprueba |
| --- | --- | --- |
| `docs/evidencia.md` | índice de las 347 mediciones; el texto íntegro está en `docs/evidencia/` | entrar por el índice al `§` |
| `docs/situacion.md` | el recap de lo medido, bullet por bullet (movido desde este fichero el 2026-11-03); el original es el `§` que cada bullet cita | leerlo y comprobar que su `§` existe |
| `docs/bloqueos.md` | qué existe y qué costaría sustituir `aapt2`, `javac`, `d8` | leerlo |
| `docs/plan.md` | el plan detallado, con fases y criterios de abandono | leerlo |
| `docs/plan-mejoras.md` | mejoras posteriores a la publicación, en el orden acordado P3 → P2 → P1 | leerlo antes de seguir con esa cola |
| `docs/plan-incremental-web.md` y `docs/plan-android-jar-propio.md` | lo **decidido y aún abierto**: un incremental propio para el modo web (I1-I5; mide por qué el `--inc` no sirve en la pestaña) y un `android.jar` redistribuible desde fuentes de AOSP (`tools/androidjar/`: J2-J4 hechas con run real; J5 hecha en local, pendiente en el sitio publicado; el jar **no** se usa aún en la página para compilar) | leerlos antes de tocar tiempos de compilación o el empaquetado del jar |
| `apps.md` | informe de ejecutar apps Android desde Termux/chroot: qué falla, por qué y el `am` con `BOOTCLASSPATH` | leerlo y reproducir sus comandos |
| `tools/entorno.sh` | el entorno del repo y del corredor en un solo sitio (`HOME`, `PREFIX`, `ANDROID_HOME`, `PATH`, envoltorio de `tcr` con guardia de 300 MiB y `LD_LIBRARY_PATH` de chromium); tareas `test`, `wasm`, `wisp`, `wisp-externo`, `gate`, `banco`, `jar`, `harness`, `sitio`, `publicar`, `doctor`, `corredor` y `tareas` | `sh tools/correr.sh corredor` y `sh tools/correr.sh tareas` |
| `tools/correr.sh` | único punto de entrada para elegir tarea: `sh tools/correr.sh <tarea>` o `sh tools/correr.sh -- <mando>`; el gate **solo se corre si el usuario lo pide** | `sh tools/correr.sh tareas` (sin correr nada) |
| `tools/gate-local.sh` | alias de compatibilidad con mandos guardados; equivale a `sh tools/correr.sh gate` | `sh tools/gate-local.sh` (cuando se pida el gate) |
| `tools/sdk34.sh` | SDK espejo con solo `build-tools/34.0.0` y `android-34`: `apkc` elige la build-tools más alta y sin esto un escritorio con 36.0.0 pediría android-36 | `sh tools/sdk34.sh "$TMP/apkc-sdk34"` |
| `tools/progreso.py` | genera el JSON del panel de progreso (ítems, estados y el trabajo en curso) desde el inventario y las fronteras de `docs/plan.md` y, si existe, desde la tabla de estado del plan de producto; lo consume el panel de `virtual_screen` | `python3 tools/progreso.py "$PREFIX/tmp/tareas.json"` |
| `tools/sitio.sh` | empaqueta en `sitio/` el sitio publicado (`studio/` versionado + `apkc.wasm` + `wasm_exec.js`), versiona el SW, conserva `CNAME` y comprueba COOP/COEP; no mete jar, keystores ni OPFS | `tools/entorno.sh sitio` |
| `tools/preview-automatica.sh` | vigilante que mantiene `apkc.wasm` sirviendo siempre el `HEAD` de la vista previa | `sh tools/preview-automatica.sh &` (estado en `$HOME/.cache/apkc-web/preview-automatica/`) |
| `tools/progreso.html` | la página del panel: pendientes primero, hechos plegados y una caja «Ahora mismo» con el trabajo en curso, remarcada | `virtual_screen page show --html tools/progreso.html` (el `tareas.json` va en `~/.cache/pantalla-secundaria/www/`) |
| `shim/navegador-sim.js` | hospedador con forma de navegador (un `fs` en memoria con la API de Node; monta ficheros reales con `--montar` y devuelve resultados con `--exportar`) | `node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp` |
| `shim/prueba-salida.js`, `shim/prueba-relink.js` y `shim/prueba-modo.js` | falsaciones del hospedador: código de salida, árbol tras `unlink` y modo de creación | `node shim/prueba-<nombre>.js` (necesitan `go` y `shim/wasm_exec.js` del GOROOT) |
| `apkc/` | **el fork de la herramienta** (subtree desde `../apkc`, MIT, pin `a909214`): donde se escribe el dexer, el compilador y los recursos en Go | `cd apkc && go test ./...` y el wasm: `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-web-wasm.wasm" .` |
| `studio/` | apkc-studio: genera proyectos apkc, los edita y **hace correr `apkc.wasm` en la pestaña** (OPFS + worker); `_headers` es para Netlify/Cloudflare y **`sw.js` pone COOP/COEP en hosts que no los fijan**, con una recarga única | `cd studio && APKC=<binario> sh e2e.sh`; `APKC_WASM=… node studio/prueba-pestana.js`; sobre el host real, `SITIO_URL=… node studio/prueba-sitio.js` |
| `sitio/` | el paquete estático que se sube a **`https://apkc-studio.surge.sh/`**; se construye, no se versiona, y lleva `CNAME` | `tools/entorno.sh sitio && cd sitio && surge . publish` |
| `studio/EDITOR.md` | el editor de la pestaña: estructura, ids, atajos, pruebas y límites | leerlo |
| `studio/AGENTS.md` | lo que un agente necesita al tocar `studio/`: comandos, contratos y trampas de OPFS/wasm | leerlo antes de cambiar la página |
| `banco/` | **el banco de ejecución**: dos fuentes de prueba y un guion que corre el mismo programa por los cinco caminos (JVM, `d8` sobre javac, `d8` sobre lo que emitimos, **nuestro dex en `app_process`** y, desde el §66, **nuestro dex con nuestro `main`**, sin javac ni d8) — sin instalar nada, sin pantalla | `sh banco/banco.sh` (necesita `APKC` compilado, `ANDROID_JAR`, `build-tools` y `rish` o `/usr/sbin/sudo`) |

## Reglas del repo

1. **Un claim, una medición.** Una afirmación sobre coste, compatibilidad o formato que no traiga
   el comando que la reproduce no entra en `docs/evidencia.md`: va a `docs/plan.md` como hipótesis,
   marcada como tal. Es la razón de existir de este repo.
2. **Sin binarios versionados.** `*.wasm` está ignorado: se construye desde el fork `apkc/` con
   `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .`. Tampoco se
   versionan `shim/wasm_exec.js` ni `studio/wasm_exec.js` (son del GOROOT de cada quien); al
   publicar `studio/`, se copia junto a `index.html`.
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
   Todo cambio ya confirmado —código, documentación o configuración— se cierra con su commit: no se
   deja trabajo confirmado sin commitear.
7. **El registro tiene tres sitios y no son intercambiables.** Una medición nueva es un `§` en
   `docs/evidencia/NN-*.md`, con el comando que la reproduce; `docs/evidencia.md` solo enruta y su tabla
   se actualiza con ese `§`. Si cambia lo que el subconjunto acepta o rechaza, actualiza **la fila** del
   inventario de la Fase 5 en `docs/plan.md` (y la tabla Estado del plan si cambia el estado de una fase).
   Este fichero **no admite bullets de hallazgos**: reglas, mapa, entorno, gate y la tabla de áreas — y no
   pasa de ~130 líneas: se lee entero en cada sesión; lo ya escrito está en `docs/situacion.md`, el recap.
8. **Verificación por capas: la punta primero, el gate al integrar.** `tools/entorno.sh` separa
   `test` (suite Go), `wasm` (hospedador de Node), `banco` (los cinco caminos) y `gate` (todo,
   con chromium). Al iterar, ejecuta primero el comando puntual del cambio —p. ej.
   `go test -run TestNombre -v .` en `apkc/`, o `node prueba.js`/`prueba-pestana.js` en `studio/`—
   y después la capa correspondiente (`test`, `wasm`); `gate` se reserva para la integración y para
   cerrar una fase. Pasar el gate completo en cada intento es caro y no añade evidencia: el gate es
   el oráculo final, no el de depuración. Si el cambio toca varias capas, sube de la punta al gate
   por orden, sin repetir las capas ya verdes salvo que el código cambie.
9. **`studio/` es un submódulo de `apkc-studio`.** El editor web tiene su propio repositorio
   (`github.com/Leonisaurov/apkc-studio`): sus cambios se commitean y se empujan **allí** primero, y
   después se actualiza el puntero del submódulo aquí (`git add studio`) en el mismo commit que los
   cambios de este repo que lo necesiten. No edites `studio/` y lo empujes solo en un sitio: el
   checkout de CI usa `submodules: recursive` y el gate se quedaría con el puntero viejo.
10. **Si el usuario pidió abrir el panel de progreso, se actualiza al cerrar cada tarea.** Marcar la fila
   en la tabla del plan (o el `actual.txt` de `$PREFIX/tmp/apkc-progreso/`) y regenerar
   `tareas.json` **no basta**: hay que copiarlo a `~/.cache/pantalla-secundaria/www/` y comprobar que el
   panel lo pinta. La página no se recarga: si el JSON no se copia, sigue enseñando lo anterior durante
   horas y el usuario trabaja con datos falsos.
11. **El estado es evidencia propia, y solo propia.** Antes de cambiar una fila del plan, responder
    estas cuatro preguntas:
    (1) ¿qué comando prueba **esa** fase? (2) ¿lo he ejecutado **después** del último cambio que la
    afecta? (3) ¿la evidencia es suya o de otra fase? (4) ¿el panel dice lo mismo que el plan?
    Si falta una, la fila se queda **pendiente** o **en verificación**; nunca «hecha» por analogía,
    por «debería funcionar» ni porque el cambio sea pequeño. Corolarios que ya fallaron:
    - **CI escrito no es CI verde**, y un workflow en el repo no verifica nada hasta que el job pasa.
    - **Empaquetar no es publicar**: si el sitio no está accesible en un host, la fase sigue pendiente.
    - **El alcance es el cambio**: relanzar el gate por un módulo no revalida ni invalida las demás
      fases; cada una conserva su estado y su prueba.
    - **No se anuncia «0 pendientes» ni «terminado»** hasta que la última fila tenga su prueba en
      verde sobre el árbol final.
 12. **Nada de temporales en tmpfs en escritorio.** `/tmp` es una tmpfs con cuota de usuario: `df`
     puede mostrar gigabytes libres y aun así fallar un `cat` de un kilobyte con `Disk quota exceeded`
     (medido el 2026-11-22, con 2,0 G acumulados en `/tmp/opencode`). Usa siempre
     `$HOME/.cache/apkc-web/…` para binarios, `PREFIX`, `TMP`, `GOTMPDIR`, perfiles de chromium y
     sondas, y fíjalos en disco antes de `tcr … tools/entorno.sh gate`. Un `starship init fish` que
     no define `enable_transience` es síntoma de esa cuota, no un fallo de fish. Si ya hay artefactos
     viejos en `/tmp/opencode`, muévelos a disco en lugar de borrarlos; no los dejes ahí. Al cerrar
     cada frente, y conforme se desocupa cada gate, limpia también los temporales en disco que ya no
     sirven (`PREFIX`, directorios de gate, perfiles de chromium, copias viejas): no los acumules
     hasta el final de la sesión. **Todo lo que esté bajo este directorio
     (`~/Develop/Go/apkc-web/`) o bajo su prefijo de temporales (`$HOME/.cache/apkc-web/…`) puedes
     borrarlo sin pedir confirmación cada vez**; conserva solo la evidencia ya registrada en `docs/`
     y lo que tenga un proceso vivo o sirva a la sesión actual.
 13. **El orquestador administra, no ejecuta.** El diagnóstico y la implementación se delegan: un plan
     del `planificador` pasa por `critico` **antes** de delegarse; cada `builder` que escribe se abre en
     worktree propio con su sesión movida allí, y el auditor no lleva worktree. En el encargo se dice
     «tú eres el subagente, yo soy el orquestador; no delegues, ejecuta y verifica en tu ruta». Los
     gates solo se corren con autorización expresa del usuario; para iterar, pruebas focales. Tras
     integrar y verificar un worktree —su contenido ya está commiteado— el orquestador puede borrarlo
     con `force: true` sin volver a pedir confirmación. El usuario autoriza **hasta 8 subagentes en
     paralelo**: mantén la cola llena con trabajo independiente; el límite lo marca esa autorización,
     no la prudencia de tokens.
14. **El único emulador es el submódulo `studio/apkc-emulador` (§357).** El padre ya no guarda pin
    espejo: el gitlink anidado de `studio` apunta a
    `996ca07ea35735234713faaa4fc9ee8388720d02` y el `HEAD` de `studio` sigue siendo el
    gitlink `HEAD:studio` del padre. Ejecuta la guardia al cerrar cada fase:
    `git -C studio rev-parse HEAD:apkc-emulador` debe dar
    `996ca07ea35735234713faaa4fc9ee8388720d02`; si no, es un fallo, no un aviso.
    Nota (§358): el emulador subió y actualizó su gitlink en el mismo commit, así que el pin pasó de
    `563e991` a `4117a8f`; la guardia sigue siendo `git -C studio rev-parse HEAD:apkc-emulador`.
    Nota (§368, registro histórico fechado): al integrar C → B → D → E, el pin pasó a
    `a2a6c29111783ee09f6e74729c4b303b0dd8242d` en el mismo commit del emulador; en su día fue la
    guardia vigente y la nota de §358 queda como registro histórico fechado.
    Actualización (familia de red, commit `3dbf574`): el pin pasó a `c448442…`; la nota de §368
    queda como registro histórico.
    Actualización (lote integrado, commit `1e1bb03`): el pin pasó a `6a207cd…`; las notas de
    `563e991`, `4117a8f`, `a2a6c29`, `c448442`, `6a207cd` y `ec842d4` quedan como registros
    históricos fechados. Actualización (integración final, commit `688a7ad`): el pin pasó a
    `a83ebcd…`, hoy registro histórico. Actualización (esquema de colores de Termux, commit
    `84a7b90`): el pin pasó a `bf7477e…`, hoy registro histórico. Actualización (lote
    Boolean/gestor, commit `c179a0b`): el pin pasó a `2a42a846a87df44d66178ca9000aabdc764d9a04`,
    vigilado con la guardia de arriba. Actualización (filas del gestor, commit `69e798c`): el pin pasó
    a `0c01986d356c6eeb20a20809e5b96c74468d34ae`, vigilado con la guardia de arriba; `2a42a84…`
    queda como registro histórico fechado.
    Actualización (LayoutParams de Termux, commit `acc7d44`): el pin pasó a
    `c54e55114568b05947170a879f989e550b6766ee`, vigilado con la guardia de arriba; `0c01986…` queda
    como registro histórico fechado.
    Actualización (getLifecycle, fecha 2026-11-26): el pin pasó a `83560681816604d0445ae365c506e453196fff41`, vigilado con la guardia de arriba; `c54e55114568b05947170a879f989e550b6766ee` queda como registro histórico fechado.
    Actualización (saved state, fecha 2026-11-26): el pin pasó a `996ca07ea35735234713faaa4fc9ee8388720d02`, vigilado con la guardia de arriba; `83560681816604d0445ae365c506e453196fff41` queda como registro histórico fechado.
    Subir el emulador exige actualizar ese gitlink en el **mismo commit**. La fuente canónica la
    resuelve `tools/materializar-emulador.sh` (argumento, `APKC_EMULADOR_DIR`,
    `studio/apkc-emulador`) sin cambiar el contrato de 34 ficheros ni su `cmp` byte a byte.

## Entorno (Termux/Android aarch64)

- Rutas: `$PREFIX=/data/data/com.termux/files/usr`, `HOME=/data/data/com.termux/files/home`. Nunca
  `/tmp`, `/usr/bin`, `/etc`. Shebangs absolutos. El shell interactivo del usuario es fish: envolver
  asignaciones de entorno en `sh -c '...'`.
- Comando pesado por `tcr` (`tcr -j 1 -q -- <cmd>`), jamás `-j$(nproc)`. Antes de algo caro:
  `free -m` y `df -h "$PREFIX"`.
- **Go queda fijado en 1.24.0** (§312): `tools/entorno.sh` antepone `$HOME_REAL/sdk/go1.24.0/bin`
  al PATH y `tools/wasm-exec-hash.sh` valida versión y SHA de `wasm_exec.js` antes de copiarlo. No
  usar el Go del sistema (`go1.27.4-X:nodwarf5`) para compilar este repo; no borrar el toolchain fijado.
- **Dependencias de las pruebas de paridad en este escritorio**: `JAVAC17=/home/leonisauro/opt/jdk17/bin/javac`
  (existe; también está `/home/leonisauro/opt/jdk21/bin/javac`) y el jar de stubs sin borrado
  `/home/leonisauro/.cache/apkc-web/androidjar/android-stub-34-full.jar`, seleccionable con
  `APKC_ANDROID_JAR_STUB` (`apkc/javaemisor_vuelta_generica_test.go:59-74`). **No** están en
  `~/.local/share/opencode/.cache/apkc-web/...`: buscarlos ahí produce un falso «incierto por entorno».
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

En GitHub (`.github/workflows/gate.yml`) el job rápido no descarga el SDK y el pesado lo instala y
lo cachea (`android-sdk-34.0.0-v1`). El pesado fija **JDK 17**: el `d8` de build-tools 34.0.0 de
Google rechaza los `.class` de javac 21 con `Unsupported class file major version 65`; el `d8` de
34.9.0 del teléfono sí los lee. El job `publicar` tiene `needs: pesado`, así que un gate rojo no
publica. §152.

En un escritorio sin Termux los guiones usan `#!/bin/sh` y `$XDG_CACHE_HOME/apkc-web` como
temporales; `tools/entorno.sh gate` levanta el SDK espejo de `tools/sdk34.sh` y usa JDK 17. §154.
`/tmp` es tmpfs con cuota de usuario y está prohibida para temporales (regla 12), incluidas las
sondas y los perfiles de chromium: un `disk quota exceeded` en `/tmp/go-build*` se arregla con
`GOTMPDIR=$HOME/.cache/apkc-web/gotmp` y `PREFIX`/`TMP` en disco, no reintentando.
La publicación es **manual**: `tools/entorno.sh sitio` y `cd sitio && surge . publish`, sobre
`apkc-studio.surge.sh`. Surge no respeta `_headers`: el SW de `studio/` pone COOP/COEP y la página
se recarga una vez (§153); verificar siempre con `SITIO_URL=… node studio/prueba-sitio.js` (§155).

## Lo que ya está medido, para no reabrirlo

Cuatro áreas cerradas. Cada `§` es un párrafo de `docs/evidencia.md` con el comando que lo reproduce; el
recap de lo que dice, en `docs/situacion.md`; y lo que el subconjunto de Java acepta o rechaza hoy, con
el `file:line` que rechaza, en el inventario de la Fase 5 de `docs/plan.md`. **Antes de medir algo, mirar
los dos: si ya está, no se reabre ni se escribe aquí.**

| Área | Lo que está cerrado | Dónde está la medición |
| --- | --- | --- |
| el dex (`apkc dex`) | lector y escritor, `debug_info_item`, bloque de anotaciones, el layout que exige ART (cuatro defectos que `d8` no ve), el internado de `type_list`, el marco por encima de v15 con las formas `/from16`, la anotación `Signature` genérica troceada como `d8`, y las lambdas, referencias de método, SAM genéricos, `altMetafactory` y la concatenación dinámica desugrados (`invokedynamic` → `new`/`invoke-direct`, ejecutados en ART); el fichero del binario `GOOS=js` es idéntico byte a byte al nativo y `d8` lo re-dexa con `rc 0` | §20-§23, §55-§60, §86, §101 y §110-§116 |
| el subconjunto Java, del fuente al dex | control de flujo (`if`, bucles, `switch` con la caída al siguiente brazo, `try`/`catch`/`finally` con el valor pendiente, el multi-catch, el `continue` que sale y el `finally` anidado), arreglos —también los de `long`/`float`/`double`—, los anchos `long`/`float`/`double` con la cuenta 23x, el `char` con sus escapes, `private` y sus `access$`, `instanceof` (también `T[]` y la forma negada), concatenación y plegado de constantes (también la promoción del literal), el literal `X.class` y `==` entre referencias, las llamadas de más de cinco palabras con `invoke-*/range`, los constructores propios con `super`/`this`, `throws`/interfaces/abstractos e `invokeinterface`, la anónima en `static` y la captura de locales (`val$x`), `StackMapTable` | §40-§54, §61-§92, §99, §100 y §129-§139 + inventario de la Fase 5 |
| los recursos sin `aapt2` | `resources.arsc`, los tres AXML byte a byte, `R.java`, los ids del framework sacados del `android.jar`, el enlazado desde los ficheros de texto, `build --linker go`, PNG en `drawable`/`mipmap`, `values-xx`, `string-array`/`integer-array`, `plurals`, `fraction`, `attr`/`styleable` (también `format="enum"`/`"flags"` con hijos), `menu`/`xml`, `raw`, `layout-land`/`port`, las densidades `ldpi`-`xxxhdpi`, el `<permission>` con todos sus atributos y `values-es-rMX` | §28-§38, §93-§107, §135, §137 y §138 |
| la pestaña y el aparato | `create`, `sign` y `verify` dentro de wasm; `build --linker go --compiler go` **sin levantar ningún proceso** en chromium y sin una sola petición fuera del origen; el keystore de release de la pestaña (contraseña, alias y certificado, sin `jarsigner`), la firma JAR de un `.aab`, el `.aab` ensamblado en Go sin `bundletool`, la v4 (`.idsig`, árbol de verity), la rotación v3.1 con linaje y v4 de dos firmantes, las claves EC P-256/P-384 y la v1 SHA-1 de `minSdk < 18`; el apk de cierre instalado y arrancado en la pantalla secundaria; un `.dex` suelto ejecutado con `app_process` (`banco/`) | §7-§8, §11, §15, §44-§47, §55, §61-§66 y §117-§136 |


