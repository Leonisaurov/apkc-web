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
| `shim/navegador-sim.js` | hospedador con forma de navegador (un `fs` en memoria con la API de Node) | `node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp` |
| `studio/` | apkc-studio: genera proyectos apkc y traduce su salida para medir fricción | `cd studio && APKC=<binario> sh e2e.sh` |

## Reglas del repo

1. **Un claim, una medición.** Una afirmación sobre coste, compatibilidad o formato que no traiga
   el comando que la reproduce no entra en `docs/evidencia.md`: va a `docs/plan.md` como hipótesis,
   marcada como tal. Es la razón de existir de este repo.
2. **Sin binarios versionados.** `*.wasm` está ignorado: se construye desde `../apkc` con
   `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .`. Tampoco se
   versiona `shim/wasm_exec.js` (es del GOROOT de cada quien).
3. **No se toca `../apkc` desde aquí.** La única excepción es la Fase 0 de `docs/plan.md`, y cada
   cambio allende el muro entra con su propio commit, su test y su gate.
4. **Licencias antes que código.** CheerpX/CheerpJ y cualquier runtime de terceros se leen **antes**
   de integrarlos: CheerpJ es comercial con tier gratuito no-comercial. Si la licencia no permite el
   uso, la fase muere y se escribe por qué.
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

## Gate antes de dar nada por hecho

```sh
# binario wasm y la suite en hospedador de navegador
sh -c 'cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$PREFIX/tmp/apkc-test.wasm" .'
sh -c 'cd shim && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" . && node navegador-sim.js "$PREFIX/tmp/apkc-test.wasm"'

# la página, de punta a punta (genera, compila de verdad, firma, verifica, y la conduce chromium)
sh -c 'cd ../apkc && go build -o "$PREFIX/tmp/apkc-gate" .'
sh -c 'cd studio && APKC="$PREFIX/tmp/apkc-gate" sh e2e.sh'
```

`studio/e2e.sh` es innegociable antes de decir "la página funciona": su test de node solo veía
verdes seis fallos que solo aparecían al descomprimir y compilar lo generado.

## Lo que ya se sabe, para no reabrirlo

- El bloque de apkc que no lanza procesos (escritor con alineado, firma v1+v2, lectores de keystore,
  `create`) **corre en hospedador tipo navegador**: 88/88 tests.
- `apkc build` en wasm responde `aapt2 compile never started: pipe: not implemented on js`: el muro
  es la ausencia de proceso, no una ruta mal. **Ningún flag lo arregla.**
- `d8` sobre una JVM wasm **no está probado**; es el experimento que decide la ambición y va antes
  de escribir cualquier compilador. Ver el orden en `docs/plan.md`.
