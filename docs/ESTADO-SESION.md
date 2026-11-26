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

# estado verificado antes del commit documental de §368 (2026-11-25), ejecutado aquí:
git rev-parse --short HEAD                        → 5b9d5e4
git ls-tree HEAD studio                           → 160000 commit 6cf42c1d50409fab5426715d96d7bc383ae6fb8b	studio
git -C studio rev-parse --short HEAD              → 6cf42c1
git -C studio status -sb                          → ## HEAD (no branch)
git -C studio ls-tree HEAD apkc-emulador          → 160000 commit a2a6c29111783ee09f6e74729c4b303b0dd8242d	apkc-emulador
git -C studio rev-parse HEAD:apkc-emulador        → a2a6c29111783ee09f6e74729c4b303b0dd8242d
```

El emulador está en `HEAD` desconectado y sin rama, así que no tiene `@{u}`: su «ahead» es
**incierto** por diseño, no un fallo. Las líneas del bloque que conservan `563e991` son el registro
histórico de §356/§357: en §356 el pin del emulador era `563e991`; tras §358 el gitlink anidado de
`studio` apunta a `4117a8f` (regla 14) y el padre no guarda pin espejo.

Actualización (§358, push ejecutado): el pin de entonces pasó a `4117a8f`
(`git -C studio rev-parse --short HEAD:apkc-emulador`), subido en el mismo commit que el emulador.
Los rangos empujados fueron emulador `563e991..4117a8f` → `origin/emulador-modulo`, studio
`e30d3ad..4a17f84` → `origin/emulador-sin-topes` y padre `8ea01c3..3a5f557` → `origin/web`; padre
y studio quedan con `rev-list --count @{u}..HEAD` a cero.
Actualización (§368, sin push en esa sesión; registro histórico): el pin vigente entonces era
`a2a6c29111783ee09f6e74729c4b303b0dd8242d`, subido en el mismo commit del emulador al integrar
C → B → D → E; los rangos de §358 quedan como registro del push anterior.
Actualización (familia de red, commit `3dbf574`): el pin pasó a `c448442…`; la nota de §368 queda
como registro histórico. Guardia ejecutada aquí: `git -C studio rev-parse HEAD:apkc-emulador` →
`c4484424140b13baef66d5d72f54bc4b2ec886cd` (y `git -C studio ls-tree HEAD apkc-emulador` →
`160000 commit c4484424140b13baef66d5d72f54bc4b2ec886cd apkc-emulador`).
Actualización (lote integrado, commit `1e1bb03` en `web`, `d592102` en `studio`): el pin pasó a
`6a207cdc7204256bb6fc2da2701320dfad603056`, guardia ejecutada aquí con
`git -C studio rev-parse HEAD:apkc-emulador`. La nota de la familia de red queda como registro
histórico. Actualización (integración final, commit `688a7ad`): el pin pasó a
`a83ebcd5654475c00bc2f52c134497e64190b646`, guardia ejecutada aquí con
`git -C studio rev-parse HEAD:apkc-emulador`; hoy es registro histórico. Actualización (esquema de
colores de Termux, commit `84a7b90`): el pin pasó a
`bf7477e7feef577a9ea554120a5b07d824274dde`, con cherry-pick del mismo SHA en
`studio/apkc-emulador`, gitlink de studio `ed39a58f29f89f445b45f4a0181fcbbb813b774f` y guardia
`git -C studio rev-parse HEAD:apkc-emulador` → `bf7477e…`. La nota de la integración final queda como
registro histórico fechado.
Actualización (lote Boolean/gestor, commit `c179a0b`): el pin pasó a
`2a42a846a87df44d66178ca9000aabdc764d9a04`, con los cherry-picks `f33bfdb`, `7213e25`, `6d84fcf` y
`2a42a84` en `studio/apkc-emulador`, el gitlink de studio
`a4d8f434988bf55391d5c647613e709e6199f82c` y la guardia `git -C studio rev-parse HEAD:apkc-emulador`
→ `2a42a84…`. La nota del esquema de colores queda como registro histórico fechado.
Actualización (filas del gestor, commit `69e798c`): el pin pasó a
`0c01986d356c6eeb20a20809e5b96c74468d34ae`, con el cherry-pick del mismo SHA del commit `fd1e5ac`
sobre `2a42a84` en `studio/apkc-emulador`, el gitlink de studio
`a7e5ec847b934678aa3ab096a9d8cac077157555` y la guardia `git -C studio rev-parse HEAD:apkc-emulador`
→ `0c01986…`. La nota del lote Boolean/gestor queda como registro histórico fechado.
Actualización (LayoutParams de Termux, commit `acc7d44`): el pin pasó a
`c54e55114568b05947170a879f989e550b6766ee`, con el cherry-pick del mismo SHA del commit `0c17130`
sobre `0c01986` en `studio/apkc-emulador`, el gitlink de studio
`b54a0b4342356657ecafbdf3d7e19293a332a6dc` y la guardia `git -C studio rev-parse HEAD:apkc-emulador`
→ `c54e551…`. La nota de las filas del gestor queda como registro histórico fechado.

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
- **Quinta APK registrada como caso negativo (§367)**:
  `org.quentin_bettoum.librememorygame_9.apk` (sha256 `df049866…`) queda en el corpus con `pasos:333`,
  `nodos=2`, `layout=res/layout/custom_dialog.xml` y `ejecucion.error` con
  `Thread.getStackTrace sobre un objeto nulo`; etiqueta «solo envoltorio, sin lógica Godot». No se da
  por funcionando ni se cuenta como app cubierta. Índice en `docs/evidencia.md`; medición en
  `docs/evidencia/15-apps-reales.md`.
- **C, B, D y E integrados (§359-§362, §368)**: C `98cedf1` (lanzadora y layout del manifiesto,
  centinela `sin-layout-xml:`), B `28c22de` (`classes2.dex`/`classes20.dex` con descriptor del
  manifiesto), D `376def6` (12 stubs, avisos de familia 12 → 0 en el cuarto APK) y E1 `77dc8e5` /
  E2 `a2a6c29` (tema por id de atributo, en `preview.go:1019-1071`), con `4d4dbd9` y `6cf42c1` en
  `studio`. Los conflictos se resolvieron conservando los cinco frentes. Cada frente tiene su
  medición en `docs/evidencia/15-apps-reales.md`; las cifras de suite son propias de cada frente
  (`460.631s`, `210.695s`, `438.144s` y `505.205s`).
- **Suite integrada en verde (§368)**: re-ejecutada con `-count=1` sobre el HEAD integrado
  `5b9d5e4`, `go test -count=1 ./...` → `ok github.com/ajinasokan/apkc 196.940s`, rc 0, 0 `FAIL`,
  0 `FAIL`. Los cinco JSON de la corrida están en `~/.cache/apkc-web/integracion/`: F-Droid con
  `org.fdroid.MainActivity`/`layout=''`/centinela, NewPipe con `res/v9.xml`/6 nodos, Termux con
  `activity_termux.xml`/13 nodos, el gestor con `error=None`/0 avisos de familia/`res/R5.xml`/30
  nodos y Libre Memory Game con el mismo error de `§367`/`layout=''`/1 nodo/6 avisos. Ninguna de las
  cinco APK se da por funcionando. El preview local sirve el HEAD integrado (wasm `dc29af669c…`);
  el valor concreto se lee en `~/.cache/apkc-web/preview-automatica/vigilante.log`. CDP, gate y
  publicación siguen sin ejecutar.

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
- Ninguna de las cinco APK reales se da por funcionando. Las tres primeras dejaron los layouts
  falsos de `§354` atrás: tras integrar C y E, F-Droid declara `sin-layout-xml:` con un solo
  `FrameLayout` (§359), NewPipe da `res/v9.xml` de 6 nodos y Termux
  `res/layout/activity_termux.xml` de 13 (§368); las cuatro y quinta APK dan `res/R5.xml`/30 nodos
  y `layout=''`/1 nodo respectivamente. C, B, D y E están hechos en capa Go/node (§359-§362), pero
  CDP, gate y publicación siguen sin ejecutar (regla 11).

## 4. Frente en marcha ahora (2026-11-25): modo teléfono en `studio/` y F0 de Compose

Tras la integración de §368, C (`§359`), B (`§360`), D (`§361`) y E (`§362`) están hechos en capa
Go/node y en verificación CDP/gate; la suite integrada está en verde (`196.940s`, rc 0, 0 `FAIL`,
0 `FAIL`). El frente en marcha es el **modo teléfono en `studio/`** y **F0 de Compose** (Fase F,
`§363`): son el trabajo abierto ahora, no un resultado medido, y no se registra aquí ninguna cifra
suya. Fase G (`§364`, fidelidad en navegador y publicación) sigue sin ejecutar.

El pin vigente de la regla 14 es `6e85f44ec9115fe8ca31d69a6f59b6dc106665ac`
(`git -C studio rev-parse HEAD:apkc-emulador` → `6e85f44…`, ejecutado y anotado), vigente tras integrar ApkShare; `b7854d6ad010518676e42f0090914c53eba89fda` queda como registro histórico fechado; `bc4946ad599be3a986b4364b6d45a7c23ed7acd2`, `3627870852d551d954c59039de25c6c4b89048c0`, `26cf94ae33a85da82b05d7fc60e30a0307643ea6`, `c0e60ca39ee746abb5350fe4d6533de683d7467f` y `9a6ec572fc72dcb374e692d094b627b1960c8efc` quedan como registros históricos fechados. Actualización (familia de red, commit `3dbf574`): el pin pasó a `c448442…`; la nota de
§368 queda como registro histórico. Actualización (lote integrado, commit `1e1bb03`): el pin pasó a
`6a207cd…`. Actualización (integración final, commit `688a7ad`): el pin pasó a `a83ebcd…`.
Actualización (esquema de colores de Termux, commit `84a7b90`): el pin pasó a `bf7477e…`.
Actualización (lote Boolean/gestor, commit `c179a0b`): el pin pasó a `2a42a84…`. Actualización (filas
del gestor, commit `69e798c`): el pin pasó a `0c01986…`. Actualización (LayoutParams de Termux,
commit `acc7d44`): el pin pasó a `c54e551…`. Los pins `563e991`, `4117a8f`, `a2a6c29`, `c448442`,
`6a207cd`, `ec842d4`, `a83ebcd`, `bf7477e`, `2a42a84` y `0c01986` quedan como registros históricos
fechados.
Actualización (getLifecycle, fecha 2026-11-26): el pin pasó a `83560681816604d0445ae365c506e453196fff41`, vigilado con la guardia de arriba; `c54e55114568b05947170a879f989e550b6766ee` queda como registro histórico fechado.
Actualización (saved state, fecha 2026-11-26): el pin pasó a `996ca07ea35735234713faaa4fc9ee8388720d02`, vigilado con la guardia de arriba; `83560681816604d0445ae365c506e453196fff41` queda como registro histórico fechado.
Actualización (mLifecycleRegistry, fecha 2026-11-26): el pin pasó a `9a6ec572fc72dcb374e692d094b627b1960c8efc`, vigilado con la guardia de arriba; `996ca07ea35735234713faaa4fc9ee8388720d02` queda como registro histórico fechado. Actualización (FragmentActivity/getSavedStateRegistry, fecha 2026-11-26): el pin pasó a `c0e60ca39ee746abb5350fe4d6533de683d7467f`, vigilado con la guardia de arriba; `9a6ec572fc72dcb374e692d094b627b1960c8efc` queda como registro histórico fechado. Actualización (desreferencia de recursos, fecha 2026-11-26): el pin pasó a `26cf94ae33a85da82b05d7fc60e30a0307643ea6`, vigilado con la guardia de arriba; `c0e60ca39ee746abb5350fe4d6533de683d7467f` queda como registro histórico fechado. Actualización (split de dos argumentos, fecha 2026-11-26): el pin pasó a `3627870852d551d954c59039de25c6c4b89048c0`, vigilado con la guardia de arriba; `26cf94ae33a85da82b05d7fc60e30a0307643ea6` queda como registro histórico fechado.
Actualización (integración final de -trimpath, Services y SplashScreen, fecha 2026-11-26): el pin pasó a `bc4946ad599be3a986b4364b6d45a7c23ed7acd2`, vigilado con la guardia de arriba; `3627870852d551d954c59039de25c6c4b89048c0` queda como registro histórico fechado.
Actualización (servicios sin `os`, fecha 2026-11-26): el pin pasó a `8156934eb63d6e709d8a7f233be78634b981d015`, vigilado con la guardia de arriba; `bc4946ad599be3a986b4364b6d45a7c23ed7acd2` queda como registro histórico fechado.
Actualización (seis familias ausentes, fecha 2026-11-26): el pin pasó a `b7854d6ad010518676e42f0090914c53eba89fda`, hoy registro histórico; `8156934eb63d6e709d8a7f233be78634b981d015` queda como registro histórico fechado.
Actualización (altura de la lista y texto de ApkShare, fecha 2026-11-26): el pin pasó a `6e85f44ec9115fe8ca31d69a6f59b6dc106665ac`, vigilado con la guardia de arriba; `b7854d6ad010518676e42f0090914c53eba89fda` queda como registro histórico fechado.
El gitlink anidado de `studio` es el único sitio donde vive; el padre no guarda pin espejo. Los tres rangos empujados en §358 quedan como registro del
push anterior. En esta sesión **no hay push**: cualquier push necesita autorización expresa, y el
recuento de `ahead` lo da `git status -sb` al leer este fichero (antes del commit documental de
§368 era `[ahead 6]` y sube con cada commit local).

El frente del esquema de colores de Termux está integrado y medido sobre ese pin: suite
`go test -count=1 ./...` → `ok github.com/ajinasokan/apkc 170.448s`, 0 `FAIL`, y focales
`ok … 0.041s`. Termux pasa de 4784 a **6599 pasos**, con `ejecucion.error` en
`setTerminalToolbarView`: se leyó el campo `android.view.ViewGroup$LayoutParams.height` de un objeto
nulo, y desaparece `Set.iterator` en `TerminalColorScheme.updateWith`. Las otras cuatro APK siguen
byte a byte idénticas a `integracion-final/`; el JSON está en
`~/.cache/apkc-web/integracion-color/termux.json` (§372). CDP, gate y publicación siguen fuera.

El orden del plan de apps se mantiene: C → B → D → E hechos, F `§363` en diseño y G `§364` sin
ejecutar (`docs/evidencia.md:401`). Ninguna de las cinco APK se da por funcionando;
`tools/entorno.sh gate`, CDP, publicación y `studio/e2e.sh` siguen sin ejecutar o en verificación.

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
