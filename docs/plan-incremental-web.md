# Plan de incremental para el modo web (`build --linker go --compiler go`)

**Estado: decidido, sin implementar.** La decisión de la primera fase está tomada y medida; las
fases I1 a I5 están pendientes. Este fichero es un plan con mediciones: cada cifra lleva el comando
que la reproduce (regla 1 del `AGENTS.md`). Lo que no se pudo medir está en «Lo que no se midió» y
no se usa para decidir.

Fecha: 2026-11-22. Origen: comprobar si `--inc` ayuda a la pestaña y, si no, diseñar un incremento
propio. Las mediciones se hicieron en el worktree `incremental-web` de la rama `web`, sobre el fork
`apkc/` en el commit de partida `9118630`.

## Estado actual

- El fork tiene el incremental: `apkc/build.go:113` registra `--inc` y `apkc/build.go:207` lo usa
  para decidir qué etapas corren; el estado vive en `build/incremental.json` (`apkc/utils.go:126`) y
  las decisiones por fichero en `apkc/utils.go:196` (`hasChanged`) y `apkc/utils.go:346` (`getFiles`).
- La pestaña **no lo pasa**: `studio/pestana.js:2592` llama a
  `build --linker go --compiler go --android-jar … --sdk-version … --codename … --keystore …
  --v4=…`, sin `--inc`. (Línea leída del checkout principal,
  `~/Develop/Go/apkc-web/studio/pestana.js`, porque el submódulo `studio/` no está inicializado en
  este worktree.)
- El compilador en Go **no tiene incremental**: `apkc/compilar.go:150-155` lo dice («No hay
  incremental aquí: el emisor resuelve los nombres entre ficheros, así que un fuente que cambia
  obliga a compilarlos todos, y eso es lo que hace»), `apkc/build.go:296-299` llama a
  `compilarFuentesEnGo()` sin parámetro de incremental, y `apkc/build.go:340-352` mete
  `compSource` (siempre verdadero con fuentes) en `appStale`, así que `dexarClasesEnGo()` corre
  todas las vueltas.
- Con `--linker go` no hay flats que compilar (`apkc/build.go:543-550` devuelve `false` sin hacer
  nada), pero el **enlace** sí se salta cuando los recursos no cambian (`apkc/build.go:258`), y el
  paquete reutiliza entradas del apk anterior (`apkc/build.go:420-437`).

De ahí la pregunta que se mide: si la compilación y el dexado son siempre completos, ¿qué ahorra
`--inc` en este camino?

## Montaje de la medición

Proyecto sintético generado por `banco/gen-proyecto-grande.py` (nuevo en este plan):

- 231 fuentes `.java`: 200 clases `ModNNN` con campos, bucles, `switch`, `try`/`catch`/`finally`,
  arreglos y concatenación; 30 actividades que leen la `R`; una `MainActivity`. El enlazador añade
  `R.java`, así que el build ve 232 fuentes y emite 244 clases.
- 79 ficheros de recurso: `values` (72 `string`, `string-array`, `integer-array`, `plurals`, 24
  `color`, 25 `fraction`, 9 `style`), `values-es` (20 `string`), 41 `layout` (31 en `layout`, 10 en
  `layout-land`), 20 PNG válidos en `drawable`, 6 `menu`, 4 `xml` y 3 `raw`.
- Manifest con `minSdkVersion 21` y una actividad launcher.

Cada repetición hace: `apkc clean`, build en frío, edición de **un solo** fuente
(`Mod037.java`: `fBase + N;` pasa a `fBase + N+100;`) y build en caliente. Tres repeticiones. El
guion compila el binario (`go build` en `apkc/`) si no le das `APKC`, y exporta `GOGC=50` y
`GOMAXPROCS=2`; `tcr` envuelve la tanda entera, no cada build.

| caso | comando |
| --- | --- |
| nativo, camino Go | `tcr -c 2 -m 2G -r 400M -- sh banco/incremental.sh` |
| nativo, referencia javac/d8 | `tcr -c 2 -m 2G -r 400M -- env COMPILER=javac sh banco/incremental.sh` |
| wasm/shim | `tcr -c 2 -m 2G -r 400M -- bash /tmp/opencode/inc-web/correr-wasm.sh` (no reproducible desde el repo, ver fronteras) |

El guion exporta `GOGC=50` y `GOMAXPROCS=2`; la tanda de javac no los necesita (javac y d8 son
procesos aparte), pero los hereda sin efecto.

## Medición 1: camino Go de la pestaña, nativo

`tcr -c 2 -m 2G -r 400M -- sh banco/incremental.sh` en este portátil (Ryzen 3 7335U, 8 hilos),
3 repeticiones, milisegundos:

| vuelta | r1 | r2 | r3 | mediana |
| --- | --- | --- | --- | --- |
| plain, frío | 597 | 472 | 453 | 472 |
| plain, caliente | 539 | 470 | 510 | **510** |
| `--inc`, frío | 639 | 462 | 468 | 468 |
| `--inc`, caliente | 318 | 347 | 327 | **327** |

`build/incremental.json` = 11 913 B con 232 fuentes.

Una primera pasada con el guion de trabajo (`/tmp/opencode/inc-web/medir.sh`, mismo montaje y mismo
`GOGC`/`GOMAXPROCS`) dio medianas de **554 ms** plain caliente y **346 ms** con `--inc`. Las dos
tandas coinciden en la dirección y en el orden de magnitud (183-208 ms de ahorro); la diferencia es
la carga del portátil en cada momento.

**Lectura por etapas** (r3 de la tanda reproducible, `log-r3-*.txt`; solo se imprimen las etapas de
más de 100 ms):

| etapa | plain caliente | `--inc` caliente |
| --- | --- | --- |
| enlace de recursos (Go) | 134 ms | no corre |
| compilar Go (232 fuentes) | 216 ms | 190 ms |
| dexar (244 clases) | 112 ms | 113 ms |
| empaquetar y firmar | reescribe 80 entradas | «copied 76 entries from the previous package» |
| total | 505 ms | 323 ms |

Coste fijo del `android.jar`: un proyecto mínimo (1 fuente + `R.java`, mismo binario) tarda
219/205/211 ms en total y **no imprime la etapa `compile go`**, que solo aparece por encima de
100 ms (`apkc/log.go:16`). Es decir, abrir e indexar el zip de 26 MB cuesta menos de 100 ms nativo;
los 216 ms de 232 fuentes son coste por fichero (≈0,9 ms cada uno).

## Medición 2: referencia javac + d8

`tcr -c 2 -m 2G -r 400M -- env COMPILER=javac sh banco/incremental.sh`, mismo proyecto y mismo
esquema:

| vuelta | r1 | r2 | r3 | mediana |
| --- | --- | --- | --- | --- |
| plain, frío | 11 126 | 9 366 | 11 258 | 11 126 |
| plain, caliente | 10 923 | 11 205 | 11 009 | **11 009** |
| `--inc`, frío | 11 094 | 11 396 | 10 212 | 11 094 |
| `--inc`, caliente | 8 131 | 7 573 | 6 963 | **7 573** |

`build/incremental.json` = 51 676 B en este camino (lleva el índice de dependencias de javac).

Log de `r3-inc-caliente`: «javac recompiles 1 java file», javac 878 ms, d8 6,036 s, sin enlace,
«copied 76 entries». Log de `r3-plain-caliente`: javac 3,919 s sobre 232 fuentes y d8 6,864 s.

Esto separa las dos cosas: **el incremental de javac sí funciona** (1 fichero en vez de 232, 0,9 s
en vez de 3,9 s) y **d8 no es incremental** (6,0-6,9 s sobre todas las clases en los dos casos). Por
eso el ahorro de `--inc` en el camino javac es de ~3,4 s sobre 11 s: casi todo lo que queda es d8.

## Medición 3: dentro del hospedador de navegador (shim)

`shim/navegador-sim.js` con un test desechable que llama a `_build` dos veces por modo en el mismo
proceso (el FS simulado no sobrevive a un proceso de node, así que la segunda pasada tiene que vivir
dentro de la misma ejecución). El árbol de build no se monta: la simulación empieza vacía. Milisegundos:

| vuelta | r1 | r2 | r3 | mediana |
| --- | --- | --- | --- | --- |
| plain, frío | 4 507 | 1 862 | 1 901 | — (r1 incluye calentamiento del runtime) |
| plain, caliente | 3 042 | 1 854 | 1 872 | **1 872** |
| `--inc`, frío | 2 554 | 1 431 | 1 538 | — (con estado previo no es frío de verdad) |
| `--inc`, caliente | 1 435 | 1 450 | 1 467 | **1 450** |

Por etapas, en wasm: plain caliente = enlace 438-457 ms + compilar 787-808 ms + dexar 421-439 ms +
empaquetar/firmar 132-145 ms; `--inc` caliente = compilar 792-834 ms + dexar 433-486 ms +
empaquetar/firmar 137-145 ms, **sin enlace**.

Dos diferencias con el nativo, ambas del mismo origen:

- El enlace cuesta ~3× más en wasm (440 ms frente a 134 ms), así que saltarlo vale más: 422 ms de
  ahorro, el 23 % de la pasada caliente.
- La reutilización de entradas del paquete no se nota (137 ms frente a 140 ms): el zip vive en
  memoria y lo que cuesta es la firma, no copiar bytes.

El coste fijo del `android.jar` en wasm no se midió con el proyecto mínimo: el test quedó escrito y
compilado, pero no se ejecutó. Queda como pregunta abierta de I5, no como cifra.

## Comparación de los apks

`apk-r3-plain.apk` y `apk-r3-inc.apk` (mismo estado de fuentes, misma clave):

- 80 entradas, en el mismo orden, **0 entradas con bytes distintos** (`sha256` de cada una).
- Mismo tamaño: 101 779 B.
- Los dos verifican con `apksigner verify --verbose` (v1, v2 y v3 en `true`) y pasan
  `zipalign -c -p 4`.
- El fichero **no** es idéntico: difieren en el byte 791 (0x317), la marca de tiempo DOS de la
  cabecera local del zip (`…53 76…` frente a `…53 77…`), y por eso difiere también la firma. Es
  metadato del contenedor, no contenido.

El comando lo repite `banco/incremental.sh`: compara entrada por entrada y sale con error si alguna
difiere.

## Dónde está el cuello, y qué se puede incrementar de verdad

Con `--inc` ya activo, el reparto de una pasada caliente en wasm es: **compilar ≈55 %, dexar ≈30 %,
empaquetar/firmar ≈9 %**; el enlace es lo que se ahorra. En nativo: compilar 190 ms, dexar 113 ms,
paquete ~30 ms. Por tanto:

- **Hipótesis H1, caché de AST y resolución por fuente.** `compilarProyectoJava`
  (`apkc/javaemisor.go:1581`) parsea los 232 ficheros y registra todas las clases antes de emitir;
  un fuente que no cambió paga el parseo y la emisión. Medición propuesta: tres builds calientes con
  la misma edición y `--inc` (una pasada sin edición incluida) sobre `banco/incremental.sh`;
  aceptación: `compile go` baja ≥30 % en wasm con el dex idéntico; abandono: <20 % o dex distinto.
- **Hipótesis H2, dexado por clase.** `dexDeClases` (`apkc/dex.go:82`) reconstruye todas las tablas
  desde cero. Medición propuesta: `cmp build/classes.dex` entre el build completo y el incremental
  tras tocar un fuente; aceptación: byte a byte igual y `dex go` ≤50 % del actual; abandono: no
  idéntico.
- **Hipótesis H3, índice del `android.jar` entre pasadas.** `abrirBuscador`
  (`apkc/javaemisor.go:346`) itera las entradas del zip en cada build. Medición propuesta: proyecto
  mínimo en el shim, con y sin caché, 3 repeticiones; aceptación: ≥100 ms menos en wasm; abandono:
  <50 ms (el nativo ya está por debajo de 100 ms).
- **Descartado por la medición:** firmar solo entradas cambiadas. En wasm el paquete no baja de
  ~135 ms con o sin reutilización (medición 3), así que no hay nada que ganar ahí.

## Decisión

1. **Activar `--inc` en la pestaña ya** (fase I1). Ahorra 183 ms de 510 nativo (36 %) y 422 ms de
   1872 wasm (23 %) en la pasada caliente de este caso, y el apk es igual entrada por entrada. No
   hay razón para no hacerlo, y es un cambio de una bandera.
2. **Abrir fases propias para el compilador** (I3-I5): el resto del tiempo está en compilar y dexar,
   que `--inc` no toca porque el compilador Go no tiene incremental.
3. **No se abre ninguna fase más para lo ya medido**: no se toca el enlazador, ni aapt2, ni
   la firma.

## Fases

Cada fase se cierra con su comando en verde sobre el árbol final; ninguna se marca hecha por
analogía.

- **I1, pasar `--inc` desde la pestaña.** Cambio en `studio/pestana.js` (repositorio
  `apkc-studio`, regla 8): añadir `--inc` a la lista de `build`. Aceptación:
  `cd studio && APKC=… APKC_WASM=… sh e2e.sh` en verde y una segunda compilación en la misma
  pestaña con el apk igual entrada por entrada (`node studio/prueba-pestana.js` ya descomprime y
  compila). Reproducción del ahorro: `sh banco/incremental.sh`. Estado: pendiente.
- **I2, medir en chromium y OPFS, no en el shim.** Comando: `APKC_WASM=… node studio/prueba-pestana.js`
  con dos compilaciones seguidas y `SITIO_URL=… node studio/prueba-sitio.js` si se publica.
  Aceptación: la segunda pasada con `--inc` baja ≥15 % respecto de la primera sin `--inc` y el apk
  coincide entrada por entrada. Abandono: si baja <15 %, se anota y no se sigue por esta vía.
- **I3, caché de AST y resolución por fuente (H1).** Comando: `banco/incremental.sh` ampliado con
  una pasada sin edición y tres ediciones distintas. Aceptación: `compile go` ≥30 % más rápido en
  wasm, dex idéntico entrada por entrada. Abandono: <20 % o dex distinto.
- **I4, dexado por clase (H2).** Comando: `cmp build/classes.dex` entre el build completo y el
  incremental tras editar un fuente. Aceptación: idéntico byte a byte y `dex go` ≤50 %. Abandono:
  no idéntico (el layout de ART no se negocia).
- **I5, índice del `android.jar` entre pasadas (H3).** Comando: el test del shim sobre el proyecto
  mínimo, con y sin caché. Aceptación: ≥100 ms menos en wasm. Abandono: <50 ms.

## Fronteras (lo que este plan no promete)

- **No promete chromium ni OPFS reales.** La medición 3 es el shim en memoria de node; la I2 es la
  que decide sobre el navegador de verdad.
- **No promete compilar solo el fichero cambiado.** El emisor resuelve los nombres entre ficheros
  (`apkc/javaemisor.go:1581`); cualquier caché tiene que respetar esa dependencia.
- **No toca aapt2, javac, d8, Kotlin ni las dependencias `jar/`.** El camino medido es
  `--linker go --compiler go` sin jars.
- **No promete un apk incremental de instalación.** La v4 (`.idsig`) ya existe y es otra cosa.
- **No promete reducir la firma.** Medido: en wasm no baja con la reutilización de entradas.
- **Solo es reproducible desde el repo la parte nativa.** `banco/incremental.sh` y
  `banco/gen-proyecto-grande.py` son autocontenidos (generan su SDK mínimo de datos, su keystore y su
  proyecto, y no versionan binarios). La medición 3 depende de `/tmp/opencode/inc-web/`: el test que
  llama a `_build` dos veces, el guion de montaje y la copia parcheada del shim. Llevarlo al repo es
  la tarea aparte del hallazgo de arriba.

## Hallazgo del harness: el shim no borra directorios como POSIX

`shim/navegador-sim.js` implementaba `unlink` sobre un directorio borrando el nodo y dejando sus
hijos huérfanos en el mapa; el segundo `RemoveAll("build/res")` dentro del mismo proceso rompía el
build con `open build/res/layout/…: No such file or directory`. Es un defecto del hospedador
simulado, no de `apkc`. Para poder medir la segunda pasada se parcheó **solo la copia de trabajo**
(`/tmp/opencode/inc-web/shim/navegador-sim.js`): `unlink` de un directorio devuelve `EISDIR` y
`rmdir` comprueba cualquier descendiente, no solo los hijos directos.

- Reproducción: `tcr -c 2 -m 2G -r 400M -- bash /tmp/opencode/inc-web/correr-wasm.sh`, cambiando
  antes la copia de `/tmp/opencode/inc-web/shim/navegador-sim.js` por el `shim/navegador-sim.js` del
  repo (sin el parche); el segundo build del mismo árbol falla en el enlace de recursos.
- Propuesta (tarea separada, no incluida aquí): llevar ese parche al repo con una prueba en `shim/`
  que haga dos `RemoveAll` sobre el mismo árbol y compare. Mientras no se haga, la medición 3 no es
  reproducible con el shim versionado y así se declara abajo.

## Registro previsto en `docs/evidencia.md`

Cuando I1 esté implementada, el § nuevo (numeración a asignar por el registro) dirá, en este orden:

1. Que `--inc` existe (`apkc/build.go:113`) y la pestaña no lo pasaba (`studio/pestana.js:2592`).
2. Que el compilador Go no es incremental (`apkc/compilar.go:150-155`) y que aun así `--inc` ahorra
   porque salta el enlace Go y reutiliza entradas del paquete.
3. Las tablas de las mediciones 1, 2 y 3 con sus comandos (`sh banco/incremental.sh`,
   `COMPILER=javac sh banco/incremental.sh` y el comando del shim con su frontera de
   reproducibilidad).
4. La comparación de apks: 80 entradas, 0 distintas, 101 779 B, `apksigner` y `zipalign` en verde,
   y la diferencia en el byte 791 (marca de tiempo DOS) como metadato.
5. La decisión de I1 y el estado de I2-I5.

No entra en evidencia nada de la medición 3 sin la nota de que corrió en el shim y de que hizo falta
parchearlo; el § debe decirlo en la primera línea del apartado.

## Lo que no se midió

- Chromium ni OPFS reales: solo el shim en memoria.
- El coste fijo del `android.jar` en wasm (test escrito y compilado, sin ejecutar).
- Los apks de las repeticiones 1 y 2: solo se compararon los de la 3.
- Una medición con dependencias `jar/`, Kotlin o `--compiler javac` dentro del wasm.
