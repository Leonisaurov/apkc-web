# AGENTS.md — apkc-web

Este repo guarda **la ambición de compilar una app Android desde el navegador, del lado del
cliente**, separada de la herramienta que funciona hoy. Es documentación + un experimento + una
página. Nada de aquí cambia `../apkc` salvo lo que la Fase 0 autorice expresamente.

## Mapa

| Ruta | Qué es | Cómo se comprueba |
| --- | --- | --- |
| `docs/evidencia.md` | lo medido, cada afirmación con su comando | leerlo |
| `docs/bloqueos.md` | qué existe y qué costaría sustituir `aapt2`, `javac`, `d8` | leerlo |
| `docs/plan.md` | el plan detallado, con fases y criterios de abandono | leerlo |
| `shim/navegador-sim.js` | hospedador con forma de navegador (un `fs` en memoria con la API de Node; monta ficheros reales con `--montar` y devuelve resultados con `--exportar`) | `node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp` |
| `apkc/` | **el fork de la herramienta** (subtree desde `../apkc`, MIT, pin `a909214`): donde se escribe el dexer, el compilador y los recursos en Go | `cd apkc && go test ./...` y el wasm: `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-web-wasm.wasm" .` |
| `studio/` | apkc-studio: genera proyectos apkc, traduce su salida para medir fricción **y hace correr `apkc.wasm` en la pestaña** (OPFS + worker) | `cd studio && APKC=<binario> sh e2e.sh`; la pestaña: `APKC_WASM=… node studio/prueba-pestana.js` |

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

## Entorno (Termux/Android aarch64)

- Rutas: `$PREFIX=/data/data/com.termux/files/usr`, `HOME=/data/data/com.termux/files/home`. Nunca
  `/tmp`, `/usr/bin`, `/etc`. Shebangs absolutos. El shell interactivo del usuario es fish: envolver
  asignaciones de entorno en `sh -c '...'`.
- Comando pesado por `tcr` (`tcr -j 1 -q -- <cmd>`), jamás `-j$(nproc)`. Antes de algo caro:
  `free -m` y `df -h "$PREFIX"`.
^- El SDK está en `~/Android` (build-tools 34.0.0, `platforms/android-34/android.jar`) y el JDK en
  `$PREFIX/lib/jvm/java-21-openjdk`.
- **Dos trampas de wasm ya conocidas**: (a) `go test -exec=$GOROOT/lib/wasm/go_js_wasm_exec` falla
  con `total length of command line and environment variables exceeds limit` si no se le da
  `env -i`; (b) sin `GOOS=js GOARCH=wasm` el lanzador recibe un ELF y se queja de
  `expected magic word 00 61 73 6d`.
- Oráculos que no comparten código con apkc: `apksigner verify --verbose`, `jarsigner -verify`,
  `zipalign -c -p`, `~/Develop/Go/apkc-harness/zipcheck.py`, `javap` (no `dexdump`, que en este
  teléfono no es ejecutable).
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

## Lo que ya se sabe, para no reabrirlo

- El bloque de apkc que no lanza procesos (escritor con alineado, firma v1+v2, lectores de keystore,
  `create`) **corre en hospedador tipo navegador**: 98/98 tests.
- **Y corre en chromium de verdad**: `studio/` tiene hospedador con OPFS (worker de módulo + servidor
  sin dependencias con COOP/COEP) y editor de fuente; `create`, editar entre sesiones y `sign` funcionan
  dentro de la pestaña, verificado por fuera (16 comprobaciones en `docs/evidencia.md` §11 y §15).
- **`apkc sign` existe** (apkc `a909214`) y **firma dentro de wasm**: 1,20 s para un apk de 9504
  bytes, cuyo resultado verifica `apksigner` (v1 y v2), `zipalign -c -p 4`, `jarsigner -verify` y
  `zipcheck.py`. Es la primera pieza de un build real que corre del lado del cliente.
- **`apkc dex` existe y corre dentro de wasm** (§23, rama `web`): `.class` -> `classes.dex` en Go, sin
  `d8` ni proceso. Sobre el proyecto del studio: 2392 bytes (los 2844 de `d8` menos `debug_info`,
  anotaciones y su marca de origen), con `type_ids`/`field_ids`/`method_ids` byte a byte iguales y los
  nueve cuerpos con la misma secuencia de instrucciones, los mismos índices y el mismo número de
  palabras. El oráculo que lo cierra: `d8` lee el dex que escribimos y lo re-dexa con `rc 0`, y el
  fichero que sale del binario `GOOS=js` es **idéntico byte a byte** al del binario nativo.
  Formato medido en §20, §21 y §22: cuatro afirmaciones que estaban mal en el §20 se corrigieron
  comparando nuestro fichero con el de `d8`, no con un manual.
- Al re-firmar un archivo que no hicimos nosotros hay dos reglas que **se leyeron de un oráculo**, no
  se adivinaron: los **directorios** viajan en el paquete y **fuera del manifiesto** (así lo hace
  `java.util.jar`, medido con `jarsigner`), y el firmante anterior se descarta **por sufijo** porque
  este keystore de debug hace que `apksigner` escriba `ANDROIDD.SF`, no `ANDROID.SF`.
- `apkc build` en wasm responde `aapt2 compile never started: pipe: not implemented on js`: el muro
  es la ausencia de proceso, no una ruta mal. **Ningún flag lo arregla.**
- **No se presta una JVM: se emite en Go** (decisión 2026-10-27, sin conexión y sin terceros). El
  objetivo del dexer está medido (§13: 2844 bytes, 217 objetos para el proyecto del studio) y la
  prueba de que la página no habla con ningún CDN está automatizada (§15). El orden ahora es
  **2G -> 3 -> 1G**; ver `docs/plan.md`.
- **La Fase 3 está escrita y cada pieza casa byte a byte con `aapt2`** (§29 a §32): `apkc/arscwriter.go`
  emite la tabla de recursos (idéntica hasta donde aapt2 la parte en dos configuraciones por API),
  `apkc/axmlwriter.go` emite **exactamente** los 784 bytes del layout y los 2000 del manifiesto,
  `apkc/r.go` emite la misma `R.java` que `link --java`, y `apkc/empaquetar.go` ensambla un apk de
  9868 bytes que `apksigner` verifica por v1 y v2, que `zipalign -c -p 4` aprueba y que
  `aapt2 dump badging` entiende. **Faltan dos puentes para que `apkc build` no use `aapt2`**: leer los
  XML de texto y producir esos modelos, y sacar de `android.jar` la tabla de ids del framework.
- Una apk mínima no se puede firmar en el cliente sin `R`: las 4 clases `R*` del recuento las genera
  `aapt2`, así que **2G y 3 van atadas** aunque se midan por separado.
