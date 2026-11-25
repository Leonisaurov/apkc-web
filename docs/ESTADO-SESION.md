# ESTADO-SESION — traspaso para una sesión limpia (2026-11-24)

Este fichero no es una medición nueva: es el índice operativo de lo que ya está en disco y de lo que
 quedaba en marcha al cortar la sesión. Las cifras y los comandos de cada línea están en el `§` que se
 cita, en `docs/evidencia.md` y en `docs/evidencia/NN-*.md`; la tabla de fases vigente es `docs/plan.md`
 y el recap histórico, `docs/situacion.md`. Nada de aquí se da por verde por analogía (regla 11).

## 1. Repos, SHAs y estado

Comandos ejecutados sobre `/home/leonisauro/Develop/Go/apkc-web` (padre) y sus submódulos:

```
git rev-parse HEAD                          → 65578d9e9c1e580402a7630114de7d82cbfd1a0a
git status -sb                              → ## web...origin/web
git rev-list --count @{u}..HEAD             → 0
git ls-tree HEAD studio                     → 160000 commit e30d3ad329f60694c83a21a0b0c8edeb0939815e	studio
git ls-tree HEAD apkc-emulador              → 160000 commit 563e991de1fac3c863e5278483c2ff745d57bbb2	apkc-emulador
git -C studio rev-parse HEAD                → e30d3ad329f60694c83a21a0b0c8edeb0939815e
git -C studio status -sb                    → ## emulador-sin-topes...origin/emulador-sin-topes
git -C studio rev-list --count @{u}..HEAD   → 0
git -C studio ls-tree HEAD apkc-emulador    → 160000 commit 563e991de1fac3c863e5278483c2ff745d57bbb2	apkc-emulador
git -C apkc-emulador rev-parse HEAD         → 563e991de1fac3c863e5278483c2ff745d57bbb2
git -C apkc-emulador status -sb             → ## HEAD (no branch)
git -C apkc-emulador rev-list --count @{u}..HEAD → fatal: HEAD does not point to a branch (rc 128)
```

El emulador está en `HEAD` desconectado y sin rama, así que no tiene `@{u}`: su «ahead» es
**incierto** por diseño, no un fallo. Los tres gitlinks cuadran: studio en el padre = `e30d3ad`, y el
pin del emulador es `563e991` tanto en el padre como dentro de `studio` (regla 14 y §356). El push
autorizado está ejecutado: padre y studio tienen `rev-list --count @{u}..HEAD` a cero.

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

## 3. En verificación o no hecho

- CDP y chromium: no ejecutados en el frente de paneles ni en el de fidelidad; `prueba-pestana.js` y
  `prueba-sitio.js` no corrieron en §347/§352; `docs/plan.md:66` y `docs/plan.md:67`.
- `studio/e2e.sh`: **en verificación** en `docs/plan.md:60`; la corrida sobre el árbol final dio
  `e2e: 1 problemas` con `253 comprobaciones bien, 3 mal`, sin `TypeError` en el log del gate (§299)
  (§299). La prueba focal dejó `256 comprobaciones bien, 0 mal`, pero el gate completo no se repitió.
- `tools/entorno.sh gate`, CI y `surge . publish`: sin ejecutar; sitio público de este árbol **no
  publicado**. `empaquetar no es publicar` (regla 11).
- Fidelidad 1:1 del emulador (§334-§339) y separación en su mitad CDP/gate: **en verificación** en
  `docs/plan.md:66-67`. D5 y la fila 8 de D6 siguen `incierto` en `docs/plan.md:57`.
- Ninguna de las tres APK reales se da por funcionando: los layouts siguen siendo los falsos de §354
  y el bloqueo compartido es `ArchTaskExecutor.getInstance usa el opcode 0x1d` (§355).

## 4. Frente en marcha al cortar: pin duplicado del emulador en el padre

Builder `ses_deefc4628ffemxMKqOEaCWnBaW`, worktree
`/home/leonisauro/.local/share/opencode/worktree/c6f089/quitar-pin-emulador-padre`. Estado verificado
en ese worktree: `HEAD 65578d9`, `## HEAD (no branch)` y cambios preparados `M .gitmodules`,
`D apkc-emulador`, `M tools/entorno.sh`, `M tools/materializar-emulador.sh`,
`M tools/preview-automatica.sh`, más `AGENTS.md` sin preparar. `git ls-tree HEAD apkc-emulador` todavía
muestra el gitlink `563e991` porque el borrado aún no está commiteado.

Objetivo: que el único emulador sea `studio/apkc-emulador` y el padre deje de tener su pin espejo.
Criterio de cierre acordado:

1. `git ls-tree HEAD apkc-emulador` → vacío.
2. `git -C studio ls-tree HEAD apkc-emulador` → `563e991`.
3. 34 `cmp` byte a byte con `sh tools/materializar-emulador.sh`.
4. `go test ./...` en verde.
5. Wasm compilado y `node prueba.js` → «prueba: todo en orden».
6. `git push` **solo con autorización expresa** del usuario.

Si este frente consume `§357`, el plan de apps pausadas se reanuda en `§358+`.

## 5. Vista previa local vigente

- URL: `http://127.0.0.1:8801/`; `curl -s -i` da `HTTP/1.1 200 OK` con `Cross-Origin-Opener-Policy:
  same-origin`, `Cross-Origin-Embedder-Policy: require-corp` y `Cache-Control: no-store`.
- `head-construido` = `65578d9e9c1e580402a7630114de7d82cbfd1a0a` = HEAD del padre
  (`~/.cache/apkc-web/preview-automatica/head-construido`).
- SHA del wasm servido, verificado con `curl -s http://127.0.0.1:8801/apkc.wasm | sha256sum`:
  `1d27617ab6e1cdabeaac811be8a321df705d8637676cee7b4bd73ad1db67ca11`, idéntico al de
  `~/.cache/apkc-web/preview/apkc.wasm` y al último «wasm actualizado» del vigilante
  (`~/.cache/apkc-web/preview-automatica/vigilante.log`).
- Procesos vivos: `node servidor.js 8801` y `sh tools/preview-automatica.sh`.
- El vigilante compara solo HEAD (`tools/preview-automatica.sh:22-77`); que se reinicie al cambiar
  `preview-local.sh` o `preview-automatica.sh` es **incierto** con la evidencia recogida: en el guion
  no hay detector de cambios de esos ficheros. Si el orquestador lo reinicia por otro mecanismo,
  documentarlo aquí al retomar. El log muestra reintentos fallidos por «falta el submódulo del
  emulador» y, desde las 04:39 UTC, «wasm actualizado» con el SHA de arriba.

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
