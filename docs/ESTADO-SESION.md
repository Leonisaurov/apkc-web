# ESTADO-SESION — traspaso para una sesión limpia (2026-11-24)

Este fichero no es una medición nueva: es el índice operativo de lo que ya está en disco y de lo que
 quedaba en marcha al cortar la sesión. Las cifras y los comandos de cada línea están en el `§` que se
 cita, en `docs/evidencia.md` y en `docs/evidencia/NN-*.md`; la tabla de fases vigente es `docs/plan.md`
 y el recap histórico, `docs/situacion.md`. Nada de aquí se da por verde por analogía (regla 11).

## 1. Repos, SHAs y estado

Comandos ejecutados sobre `/home/leonisauro/Develop/Go/apkc-web` (padre) y sus submódulos:

```
git rev-parse --short HEAD                  → 3a5f557 (al cortar; desde entonces hay commits locales sin empujar; ver git status -sb)
git status -sb                              → ## web...origin/web
git rev-list --count @{u}..HEAD             → 0 (al cortar; ver git status -sb para el recuento actual)
git ls-tree HEAD studio                     → 160000 commit 4a17f84	studio
git ls-tree HEAD apkc-emulador              → (vacío; el padre no guarda pin espejo, regla 14)
git -C studio rev-parse --short HEAD        → 4a17f84
git -C studio status -sb                    → ## emulador-sin-topes...origin/emulador-sin-topes
git -C studio rev-list --count @{u}..HEAD   → 0
git -C studio ls-tree HEAD apkc-emulador    → 160000 commit 4117a8f6d4cdbab896e82b6f89b29a2cc00d54f6 apkc-emulador
git -C apkc-emulador rev-parse HEAD         → (histórico §356/§357: 563e991…; hoy no aplica, directorio retirado)
git -C apkc-emulador status -sb             → (histórico §356/§357: ## HEAD (no branch))
git -C apkc-emulador rev-list --count @{u}..HEAD → (histórico: fatal … rc 128; hoy no aplica)
git -C studio rev-parse --short HEAD:apkc-emulador → 4117a8f
```

El emulador está en `HEAD` desconectado y sin rama, así que no tiene `@{u}`: su «ahead» es
**incierto** por diseño, no un fallo. Las líneas del bloque que conservan `563e991` son el registro
histórico de §356/§357: en §356 el pin del emulador era `563e991`; tras §358 el gitlink anidado de
`studio` apunta a `4117a8f` (regla 14) y el padre no guarda pin espejo.

Actualización (§358, push ejecutado): el pin vigente es `4117a8f`
(`git -C studio rev-parse --short HEAD:apkc-emulador`), subido en el mismo commit que el emulador.
Los rangos empujados fueron emulador `563e991..4117a8f` → `origin/emulador-modulo`, studio
`e30d3ad..4a17f84` → `origin/emulador-sin-topes` y padre `8ea01c3..3a5f557` → `origin/web`; padre
y studio quedan con `rev-list --count @{u}..HEAD` a cero.

## 2. Hecho y verificado (en su capa: node, Go y sitio empaquetado)

- **Separación del emulador (§350)**: 59 commits importados con `subtree split --ignore-joins`, 34
  ficheros `preview*.go`, wasm byte a byte con el del árbol anterior y el mismo JSON de `preview`
  (`pasos: 22`). `docs/evidencia/23-emulador-modulo.md:1-94`.
- **Materialización gobernada (§351)**: `sh tools/materializar-emulador.sh` da «materializados 34
  ficheros preview*.go»; los 34 `cmp` mudos, seis hooks y la falsación que falla en el byte 121.
  `docs/evidencia/23-emulador-modulo.md:95-146`.
- **Pestaña intacta (§352)**: `go test -count=1 ./...` → `ok github.com/ajinasokan/apkc 260.651s`,
  wasm compilado con `GOOS=js GOARCH=wasm CGO_ENABLED=0` y `node prueba.js` → «prueba: todo en orden».
  `docs/evidencia/23-emulador-modulo.md:147-177`.
- **Cambio de propietario del gitlink (§356)**: la fuente canónica pasa a `studio/apkc-emulador`, el
  padre conserva el pin espejo, un único resolvedor sirve a los nueve guiones de `banco/`, los 34
  `cmp` siguen intactos, la suite Go da `248,739 s`, el wasm servido es `1c2f521b…` y el sitio
  empaquetado no lleva `*.go`, `.gitmodules` ni el directorio del submódulo. El push a `apkc-studio`
  (`e9274dd..a4e983c`) fue autorizado y ejecutado. `docs/evidencia/24-studio-emulador-submodulo.md:3`.
- **Fidelidad F0-F4 (§334-§339)**: medición propia en node y Go, `gofmt -l` sin salida, siete
  `node --check` mudos y `prueba: todo en orden`, con los focales `TestElPreviewPintaDosNotas`,
  `TestElPreviewPintaUnaListaConSuAdaptador` y los de sesión en verde (`rc=0`).
  `docs/evidencia/21-emulador-fiel.md:238-264`. La mitad CDP y Droidloom sigue en verificación
  (`docs/plan.md:66`).
- **Paneles flotantes (§340-§347)**: los seis commits del submódulo (`19d92b9`, `e31b073`, `08e747b`,
  `c0727b6`, `c2215f2`, `b0d16c9`) y la persistencia (`12b98f9`); sobre el árbol final, tres
  `node --check` mudos y `prueba: todo en orden`, código 0. `docs/evidencia/22-panels-flotantes.md:69-98`.
- **CAs oficiales de WISP (§349)**: bundle extraído del `cacert.pem` de curl con `curl -fSL`, 3487 B
  y sha256 `c2b67c490da9b82d68b77f88c924288faa78fcc66077088c530921053ff19a2c`, fijado en
  `studio/cas-wisp.js`. La licencia se confirmó por escrito: MPL 2.0 según la sección «CA certificate
  store license» de `curl.se/docs/caextract.html` y Mozilla Root Store Policy v3.1, sección 1; ambas
  son licencias libres. `docs/evidencia/23-wisp-cas-oficiales.md:214-248`.
- **Hot swap Go+JS (§238, §249, §295)**: identidad y estado conservados al cambiar el método, firma
  automática que deriva `MainActivity$1.onClick` (commit del submódulo `677af8e`), puente
  `apkcWisp/1` en wasm y proxy página-worker; timeline medido `["Pulsos: 0","1","2","3","4","4","9"]`.
  `docs/situacion.md:731-746`.
- **Cronómetro y reloj en sesión (§212, §291-§292, §333, §348)**: reloj lógico sin `Thread.sleep`,
  `--avanzar` y bombeo del cronómetro en la matriz D6 reejecutada sobre `a41db75`; navegación viva
  entre actividades en `6496918` (`fix(sesion): navega entre actividades y publica el recorrido de
  pantallas`). `docs/situacion.md:776-788` y `docs/situacion.md:822-840`.
- **D7 (§348)**: la corrección de `6496918` y su CDP de 247 comprobaciones bien, 0 mal, sobre
  `~/.cache/apkc-web/tmp/cdp-d7.log:262`; el reclamo del «40» sigue sin reproducción y no se cuenta
  como cerrado. `docs/situacion.md:832-840`.
- **Lienzo, selectores y ApkShare (§256-§258, §287, §298, §310)**: el dex de las seis clases de
  ApkShare byte a byte, la numeración de anónimas cerrada, el lienzo con coordenadas y colores reales
  y ApkShare compilando con `--compiler go --linker go`, con y sin `--inc` (21 clases, dex de 16504 B
  idéntico, APK `apksigner verify` con v1/v2/v3 `true`). `docs/situacion.md:759` y §256-§258.
- **D2-D6 (§329-§333)**: coste real de `check`, asignación cualificada, columna del parser, decisión
  de sesión persistente incierta y matriz Java/XML; las filas 1-7 de la matriz medidas en node sobre
  el árbol reconciliado y la 8 `incierto` en CDP. `docs/plan.md:57` y `docs/evidencia/11-emulador-check.md:1439`.
- **Corrección del `jarindice`**: el commit `a46c2fe` repara el fixture que medía 416 frente a 414.
  Verificado ahora con (`cwd=…/apkc`, Go 1.24.0 fijado):
  `PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH GOROOT=/home/leonisauro/sdk/go1.24.0 tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run TestIndiceDelJarSeReutilizaYSeInvalida -v .`
  → `--- PASS: TestIndiceDelJarSeReutilizaYSeInvalida (0.00s)` / `PASS` /
  `ok  	github.com/ajinasokan/apkc	0.007s`.
- **Fase A de dex reales (§355)**: tuplas byte a byte extraídas de los APK — valor `0x17` con índice
  little-endian `0b 7e` (`_COROUTINE`), cola de `SpacerKt` (4 valores para 8 campos) rellena con
  `VALUE_NULL`, y sustituto suelto `ed a0 80` convertido en un solo `U+FFFD` (MUTF-8 18/20)—. Tests
  focales y suite en verde, escritor de dex con el mismo SHA; los tres `preview` pasan de
  `pasos:0` a **292, 286 y 207**. `docs/evidencia/15-apps-reales.md:71-177`.
- **Registro de §358, §365 y §366**: apuntan, no re-miden. §358 cierra el opcode 0x1d/0x1e en los tres
  APK, §365 fija la capa wasm final (0 FAIL/871 PASS/30 SKIP, rc 0) y §366 la línea base pre-D del
  cuarto APK; ninguna de las cuatro apps se da por funcionando y §366 no se fusiona con §361.
  Índice en `docs/evidencia.md:365-367`; fuentes en `docs/evidencia/15-apps-reales.md` y
  `docs/evidencia/14-banco-wisp-lienzo.md`.

## 3. En verificación o no hecho

- CDP y chromium: no ejecutados en el frente de paneles ni en el de fidelidad; `prueba-pestana.js` y
  `prueba-sitio.js` no corrieron en §347/§352; `docs/plan.md:66` y `docs/plan.md:67`.
- `studio/e2e.sh`: **en verificación** en `docs/plan.md:60`; la corrida sobre el árbol final dio
  `e2e: 1 problemas` con `253 comprobaciones bien, 3 mal`, sin `TypeError` en el log del gate
  (§299). La prueba focal dejó `256 comprobaciones bien, 0 mal`, pero el gate completo no se repitió.
- `tools/entorno.sh gate`, CI y `surge . publish`: sin ejecutar; sitio público de este árbol **no
  publicado**. `empaquetar no es publicar` (regla 11).
- Fidelidad 1:1 del emulador (§334-§339) y separación en su mitad CDP/gate: **en verificación** en
  `docs/plan.md:66-67`. D5 y la fila 8 de D6 siguen `incierto` en `docs/plan.md:57`.
- Ninguna de las tres APK reales se da por funcionando: los layouts siguen siendo los falsos de §354,
  re-medidos tras `§358` (3 nodos, `res/A1.xml` / `abc_action_bar_title_item.xml`, 414/355/282 pasos);
  el bloqueo compartido de `§355` (`ArchTaskExecutor.getInstance usa el opcode 0x1d`) quedó resuelto
  en `§358` (`docs/evidencia.md:365`), y los errores actuales son de stubs, pendientes de Fase D.

## 4. Frente en marcha al cortar: §358 cerrado; C (§359) y B (§360) sin implementar

El pin y los rangos empujados están registrados en el apartado 1: el pin vigente de la regla 14 es
`4117a8f`, subido en el mismo commit que el emulador, y los tres rangos (`origin/emulador-modulo`,
`origin/emulador-sin-topes` y `origin/web`) quedaron con `rev-list --count @{u}..HEAD` a cero en su
momento. `§358` está medido, registrado y cerrado: el opcode `0x1d`/`0x1e` dejó de bloquear los tres
APK (`docs/evidencia.md:365`). Después de ese push hay commits documentales posteriores **sin
empujar**; el recuento de `ahead` lo da `git status -sb` en el momento de la lectura (no se fija aquí
una cifra, porque cambia con cada commit), y cualquier nuevo push necesita autorización expresa.

El plan de apps pausadas sigue este orden: C `§359` → B `§360` → D `§361` → E `§362` → F `§363` →
G `§364` (`docs/evidencia.md:391,395`).

### C (§359) — criterios finales

1. Ejecutar y registrar `apkanalyzer manifest print` y `aapt2 dump badging` de F-Droid **antes** de
   usar `org.fdroid.MainActivity`. Hoy eso es hipótesis: `MAIN` + `LEANBACK_LAUNCHER`, alias
   habilitado `IconActivity` → `MainActivity` y alias deshabilitados `IconCalculatorActivity` /
   `CalculatorActivity`.
2. `habilitado` se decide sobre `valor.tipo == tipoBooleano`, nunca sobre `s != "false"`.
3. El orden de elección es `pedido` → `layoutID` → `activity_<Nombre>` → `errSinLayoutXML`, sin
   fallback alfabético.
4. El sentinel `sin-layout-xml:` va en `avisos` y Fase D cuenta por familia, no con `len(avisos)`.

C se abre en worktree propio y se re-falsa después de B.

### B (§360) — criterio final

`ejecucion.actividad == claseCorta(descriptorDeActividad(paquete, nombreActividadC))` (el descriptor
es el de C, no un valor inyectado) **y**, en el test, `met.dex == dx[1]`. Además `Error==""`, `Ciclo`
con `onCreate` y `Capturas` con texto real y sin señuelo. **No se añade campo nuevo al JSON.** Las
colisiones de descriptor son error explícito (0 duplicados medidos en cinco APK). NewPipe y el cuarto
APK no deben registrar regresión.

Ninguna app se da por funcionando; F (Compose) queda para `§363`; `tools/entorno.sh gate`, CDP y
publicación siguen sin ejecutar y `studio/e2e.sh` está en verificación.

## 5. Vista previa local vigente

- Vista previa viva: `curl -s -D - -o /dev/null http://127.0.0.1:8801/` → `HTTP/1.1 200` con
  `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: require-corp` y
  `Cache-Control: no-store`.
- Procesos vivos: `node servidor.js 8801` y `sh tools/preview-automatica.sh`.
- `head-construido` = `git rev-parse HEAD` del padre; se comprueba en
  `~/.cache/apkc-web/preview-automatica/head-construido`. No se fija aquí un SHA ni un recuento de
  `ahead`: ambos cambian con cada commit; para el estado del árbol, `git status -sb`, y para el
  historial de construcciones, `~/.cache/apkc-web/preview-automatica/vigilante.log`.
- El wasm servido es byte a byte idéntico al de `~/.cache/apkc-web/preview/apkc.wasm`, verificado con
  `sha256sum` de la respuesta de `http://127.0.0.1:8801/apkc.wasm` frente al fichero de disco. El
  valor concreto cambia con cada reconstrucción y se lee en `vigilante.log` («wasm actualizado»).
- Lo que sigue **sin medir**: el reinicio automático al cambiar los guiones del vigilante
  (`preview-local.sh` / `preview-automatica.sh`). El guion no tiene detector de cambios de esos
  ficheros, así que esa cuestión concreta no se da por medida.

## 6. Decisiones del usuario que no se pueden perder

- No medir temperatura como bloqueo. Los `§` que citan umbrales `Tctl` (p. ej.
  `docs/evidencia/22-panels-flotantes.md:65,96`) conservan el texto histórico de la medición, pero
  se aclaró al retomar que el motivo vigente para no correr CDP es solo la falta de autorización
  expresa; lo mismo dicen ahora `docs/situacion.md` (§348) y `docs/plan.md:66`.
- No empujar sin autorización; la autorización posterior al push de todo fue ejecutada (padre y studio
  a `rev-list --count @{u}..HEAD` cero). Los futuros pushes vuelven a necesitar autorización.
- Quiere **un solo** `apkc-emulador`, dentro de `studio/` (frente en marcha del apartado 4).
- Prefiere licencias libres: por eso se verificaron MPL 2.0 y Mozilla Root Store Policy v3.1 antes de
  usar el bundle de CAs (§349).
- Quiere todo documentado en disco para empezar en sesión limpia: este fichero y
  `docs/plan-compatibilidad-apps-pausado.md`.

## 7. Comandos canónicos de verificación

```sh
sh tools/materializar-emulador.sh
cd apkc && go test ./...
cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$TMP/apkc-wasm.wasm" .
node shim/navegador-sim.js --montar …/app.apk:/app.apk "$TMP/apkc-wasm.wasm" preview /app.apk
cd studio && node prueba.js
git ls-tree HEAD studio apkc-emulador
git -C studio rev-parse HEAD && git -C studio ls-tree HEAD apkc-emulador
```
