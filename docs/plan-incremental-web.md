# Plan de incremental para el modo web (`build --linker go --compiler go`)

**Estado: I1 e I2 hechas; I3 cerrada en `d3803bb` tras el APROBAR del crítico; I4 hecha en el
camino nativo (dex idéntico y `dex go` 100 → 45 ms), con el gate pendiente; I5 medida (ahorra
80-92 ms por build en wasm, por debajo del criterio de ≥100 ms) y no cerrada.** La huella del jar es
ruta + tamaño + CRC-32 del directorio central, nunca la fecha. `--inc` está activo
en la pestaña y el ahorro se midió en chromium/OPFS; el cuello que queda está en compilar y dexar.
Este fichero es un plan con mediciones: cada cifra lleva el comando que la reproduce (regla 1 del
`AGENTS.md`). Lo que no se pudo medir está en «Lo que no se midió» y no se usa para decidir.

Fecha: 2026-11-22. Origen: comprobar si `--inc` ayuda a la pestaña y, si no, diseñar un incremento
propio. Las mediciones se hicieron en el worktree `incremental-web` de la rama `web`, sobre el fork
`apkc/` en el commit de partida `9118630`.

## Estado de las fases

| fase | estado | comando que lo prueba | cifra medida |
| --- | --- | --- | --- |
| I1, pasar `--inc` desde la pestaña | **hecha** | `sh tools/entorno.sh gate` (con `PREFIX` propio y los puertos del gate libres) | la página compila con `--inc`; el apk es igual entrada por entrada al de un build completo (7 entradas, 0 distintas en el proyecto de la prueba; 80 y 0 en `banco/incremental.sh`); un recurso nuevo o editado reenlaza y llega a `resources.arsc` (hallazgo del crítico, arreglado) |
| I2, medir en chromium y OPFS | **hecha** | el mismo gate; las cifras salen del test `12c` de `studio/prueba-pestana.js` | con `--inc` solo: 1831 ms en frío → 1034 ms en caliente (−44 %); con I3: 1458 → 152 ms (−90 %; otras tandas 1539 → 166, 1357 → 201 y 1877 → 138). El criterio era ≥15 % |
| I3, reutilizar las clases emitidas por hash | **cerrada en `d3803bb` (APROBAR del crítico)** | gate + `go test -count=1 ./...` con `APKC_ANDROID_JAR=…` | `compile go` 390 ms → 0 y `dex go` 106 → 0 en wasm (criterio: ≥30 %), dex idéntico; en nativo el atajo se paga en parte por el estado más grande (véase I3) |
| I3, caché de AST por fuente dentro del proceso | **descartada por estructura** | `studio/hospedador.js:240` | cada comando de la página arranca una instancia nueva de Go: ninguna caché en memoria cruza dos builds |
| I4, dexado por clase | **hecha en nativo, gate pendiente** | `sh banco/incremental.sh` (bloque «dex por clase») y `go test -count=1 -run TestDexadoPorClaseReutilizaYNoCambiaUnByte .` | `cmp` mudo entre la vuelta caliente y el dexado completo del mismo árbol; `dex go` 100 → 45 ms con 244 clases y 1875 `code_item` (1866 reutilizados tras editar un fuente); el dex no cambia ni un byte |
| I5, índice del `android.jar` entre pasadas | **medida, por debajo del criterio** | `TestIndiceDelJarMideElAhorro` en el shim con el jar montado, tres procesos nuevos | 80-92 ms menos por build (criterio ≥100 ms, abandono <50 ms): el zip cuesta 88-102 ms la primera pasada y 51-71 en régimen, y con la caché puesta son 4,4-10,2 ms. La huella es ruta + tamaño + CRC-32 del directorio central, no la fecha |


## Estado anterior (lo que el plan encontró)

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

El coste fijo del `android.jar` en wasm se midió después, con el proyecto mínimo y en el hospedador
de Node: 90-100 ms la primera pasada de un proceso y 57-75 ms en régimen, que con la caché de I5 se
quedan en 4,0-7,5 ms. El detalle y la tabla están en «I5: el índice del `android.jar` entre pasadas».

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

## I1: `--inc` en la pestaña (hecho)

- `studio/pestana.js` arma el argv de `compilarCon()` con `--inc` de serie; `compilarCon(k, tipo,
  opciones)` acepta `{incremental: false}` **solo** para que la prueba pueda pedir el build completo
  del mismo árbol y comparar. No hay otra forma de compilar sin `--inc` desde la página.
- `studio/prueba-pestana.js` (sección `12c`) hace, en la misma pestaña y sobre el proyecto de la
  plantilla `hola` (7 entradas de apk, clave de depuración):
  1. `apkc clean` + build completo sin `--inc` → foto del apk (SHA-256 de cada entrada, leído por
     `leerZip`, que valida el CRC de todas);
  2. `apkc clean` + build con `--inc` en frío → foto; tiene que coincidir con la anterior;
  3. build con `--inc` otra vez, sin tocar nada → foto; las clases no se recomponen y el apk sigue
     igual;
  4. edición de un fuente (una clase nueva al final del `MainActivity.java`) + build con `--inc` → el
     dex cambia y el enlace de recursos **no** corre;
  5. `apkc clean` + build completo del árbol editado → foto; tiene que coincidir con la incremental.
- Seis comprobaciones nuevas, todas en verde en el gate:

```sh
tcr -c 2 -m 2G -r 400M -- env HOME_APKC_WEB=/home/leonisauro CHROME_BIN=/usr/bin/chromium \
  GOGC=50 GOMAXPROCS=2 PREFIX="$PREFIX" sh tools/entorno.sh gate
# ok la pestaña compila con --inc y la prueba sigue pudiendo pedir el build completo
# ok el apk con --inc es igual entrada por entrada al de un build completo — {"entradas":7,"distintas":[]}
# ok la segunda compilación en la misma pestaña no vuelve a enlazar recursos
# ok la segunda pasada no es más lenta que la primera en la misma pestaña
# ok con un fuente editado el build con --inc no reenlaza y el dex cambia
# ok el apk incremental tras editar un fuente es igual entrada por entrada al completo
```

El detalle de la cuarta línea es la medición de I2 (abajo). El argv que la prueba ve en la consola de
la página es literal: `$ apkc build --inc --linker go --compiler go --android-jar /sdk/android.jar
--sdk-version 34 --codename 14 --keystore /sdk/debug.keystore --storepass *** --keyalias
androiddebugkey --v4=true`.

## I2: medido en chromium y OPFS (hecho)

Mismo gate, misma pestaña, mismas opciones de `apkc build`; los milisegundos son el `[timing] total`
que imprime apkc en cada build (no el reloj de la página, que además suma verificar, descargar y la
vista previa). Proyecto de la prueba, 7 entradas, `--v4=true`. Las dos columnas son la misma prueba en
dos árboles: el gate del 13:22 solo con I1, y el gate del 13:38 con I1, I2 e I3.

| vuelta | solo I1 | con I3 |
| --- | --- | --- |
| con `--inc`, en frío (el estado no existe, hace todo) | 1 831 | 1 539 |
| con `--inc`, caliente (nada cambió) | 1 034 | **166** |
| ahorro de la caliente frente a la fría | **44 %** | **89 %** |
| reloj de la página, fría → caliente | 2 497 → 1 716 ms (31 %) | 1 939 → 753 ms (61 %) |
| `compile go`, fría → caliente | no se registró | 390 → 0 |
| `dex go`, fría → caliente | no se registró | 106 → 0 |

Una segunda tanda de la misma prueba, ya con I3 y en solitario, dio 1 357 → 201 ms (85 %) y
`compile go` 300 → 0; el gate final, sobre el árbol ya commiteado, dio 1 458 → 152 ms (90 %),
`compile go` 291 → 0 y una pasada completa de 1 601 ms. Una cuarta, ya con el arreglo de recursos y
`PREFIX` nuevo, dio 1 877 → 138 ms (93 %), `compile go` 431 → 0 y una completa de 1 732 ms. La
dirección y el orden de magnitud se repiten en las cuatro.

La pasada **completa sin `--inc`** de la misma secuencia midió 4 204 ms (`compile go` 1 283,
`dex go` 420) en la tanda del 13:38 y 1 601 ms (`compile go` 341) en la final: en esta máquina la
cifra absoluta de esa pasada varía entre tandas (había otro gate en paralelo en la primera), así que el
criterio no se apoya en ella. Se apoya en dos pasadas seguidas en la misma pestaña, que es lo que pide
y lo que no depende de la carga del portátil.

El criterio de abandono de I2 («si baja <15 %, se anota y no se sigue») no se activa: baja 44 % con
solo I1 y 89 % con I3. La lectura por etapas confirma el mecanismo, y no el cronómetro: la pasada fría
imprime `[build] linking resources with the Go linker` y `[timing] package and sign go`; la caliente no
imprime ninguna de las dos, imprime `ningun fuente cambio: se reutilizan las clases de build/classes`,
y el apk resultante es byte a byte el anterior.

**Frontera de esta medición**: el proyecto de la pestaña es pequeño (7 entradas). Un proyecto con más
fuentes gana más en el enlace (es el 23 % medido en wasm sobre 232 fuentes) pero también paga más el
compilador, que I3 solo se salta cuando no cambió ningún fuente. Las cifras de 232 fuentes siguen
siendo las de la medición 1 (nativo) y la 3 (shim), no las de chromium.

## I3: reutilizar las clases emitidas por hash (cerrada, APROBAR del crítico)

**Integración y cierre.** `d3803bb` (`feat(apkc): reutiliza build/classes cuando ningun fuente
cambio`) es cherry-pick de `3b7f4e3` del worktree `inc-i3`. Los tests enfocados pasan
(`TestCompilarEnGoReutilizaLasClasesSinFuentesCambiados`,
`TestIncrementalReenlazaCuandoUnRecursoEsNuevoOEditado`, `TestAuxRoster.*` y las regresiones de
reloj y de topes): `ok … 0.553s`; el paquete completo en el worktree, `ok … 57.097s`. El builder
midió en nativo un proyecto de 92 fuentes: 315 ms en frío y 15 ms sin cambios, con
`ningun fuente cambio: se reutilizan las clases de build/classes` y
`nothing to put in the package this round`, los mismos SHA-256 de APK y dex (`1e2c5df7…` y
`6a2e683d…`) y `incremental.json` de 24 700 B. El crítico
`ses_df9baefaeffemtyU4YC0gXywOR` dio **APROBAR** sobre `d3803bb`, sin hallazgos `CONFIRMADO`, y la
fase queda **cerrada**. Queda una pregunta abierta no bloqueante: el classpath no invalida el
reuse; I5 la resolvió con la huella del jar en `clasesAlDia()` (véase «I5»). La medición del builder
queda en este plan; no se abre `§` en `docs/evidencia.md`.

Lo que **sí** se puede hacer, y está hecho:

- `apkc/compilar.go`: `compilarFuentesEnGo(incremental bool)`. Con `--inc` llama primero a
  `clasesAlDia()`; si responde que sí, imprime `ningun fuente cambio: …`, devuelve `(nil, false)` y la
  etapa `compile go` no corre.
- `clasesAlDia()` exige las dos mitades: (a) cada `.java` de `src/` conserva el digest que registró la
  última compilación —los fuentes del camino Go no se registraban en ningún sitio, así que la primera
  vuelta tras este cambio compila y **a partir de ahí** hay contra qué comparar— y (b) el digest del
  roster de `build/classes` coincide con el registrado (`DataTable.ClassRoster`,
  `apkc/utils.go:130`). La segunda mitad es la que distingue «no cambió nada» de «un build murió a
  medias» o «alguien borró una clase a mano».
- `classRosterSHA()` resume `build/classes` en un SHA-1: ruta relativa y SHA-1 de cada fichero, en el
  orden léxico de `filepath.Walk`.
- Como `compSource` queda en `false`, `appStale` también, y `dexarClasesEnGo()` no corre: el
  `classes.dex` del disco se queda como estaba y el paquete no se rehace
  (`apkc/build.go:456`, `mustPackageAgain`). El apk resultante es byte a byte el anterior, y por eso
  las fotos de la prueba coinciden.

Por qué la letra literal de I3 (una caché de AST por fuente **dentro del proceso de wasm**) no puede
ser: `studio/hospedador.js:240` crea una instancia nueva de Go para cada comando —un binario de Go en
wasm no se puede volver a correr sobre la misma instancia después de salir—, así que no hay proceso que
sobreviva entre dos builds de la pestaña. Cualquier caché tiene que vivir en disco (OPFS), y por eso lo
implementado es el roster por contenido.

Coste medido del estado más grande, con `sh banco/incremental.sh` (`PREFIX` propio, `GOGC=50`,
`GOMAXPROCS=2`, 232 fuentes):

- `build/incremental.json` pasa de **11 913 B a 29 954 B**: ahora lleva el digest de las 232 fuentes y
  el roster de las 244 clases.
- Pasada caliente tras editar un fuente: `--inc` **370 ms** frente a **512 ms** sin `--inc` (medianas
  de tres repeticiones, tanda `2026-11-22T13:2x`, `APKC=/…/apkc-banco`). Antes de I3 las mismas
  medianas eran 327 y 510: el atajo de la vuelta sin cambios se paga con ~40 ms de hashing y de estado
  en la vuelta con cambios. Una segunda tanda en la misma máquina, con más carga, dio 463 y 675 ms: la
  dirección se mantiene y la cifra absoluta no.
- El apk sigue saliendo igual: 80 entradas, **0 con bytes distintos**, `apksigner` v1/v2/v3 en `true` y
  `zipalign -c -p 4` correcto.

Lo que **no** se hizo de I3: emitir solo los fuentes cambiados. El emisor resuelve los nombres entre
ficheros y pliega constantes de otras clases, así que reutilizar la emisión de un fuente exige un
índice de API por clase como el `ClassAPI` que ya usa el camino de javac (`apkc/utils.go`), y el
criterio de I3 prohíbe que el dex cambie ni un byte. Queda como la parte abierta de la fase, con este
mismo banco como comando.

## I5: el índice del `android.jar` entre pasadas (medido, por debajo del criterio)

`abrirBuscador` (`apkc/javaemisor.go:349`) recorría el zip del `android.jar` en cada build: 13 968
entradas de directorio central, 5 720 de ellas clases. Ahora el directorio se lee en una pasada propia
(`apkc/jarindice.go`) y se deja en `build/jarindice.bin` con la huella del jar en la cabecera; la
vuelta siguiente lee ese fichero de 230 KB y no parsea el zip. Si la huella no cuadra, si el fichero
está truncado o si el zip es zip64 se vuelve a recorrer el directorio, y si el lector propio no puede
se cae al `archive/zip` de siempre, que es también el oráculo de la prueba
(`TestDirectorioCentralDelJarContraArchiveZip`: mismas 5 720 entradas, método, tamaño y CRC, y una
muestra de 24 clases byte a byte).

**La huella es ruta + tamaño + CRC-32 del directorio central, nunca la fecha.** El crítico de I5
encontró que la fecha no es un dato del contenido: el hospedador de la pestaña devolvía una sintética
que cambiaba en cada recarga, así que con ella se reindexaba y `clasesAlDia()` recompilaba el proyecto
entero por nada. El directorio central es exactamente lo que el índice guarda —nombres, offsets,
tamaños, CRC y método de cada entrada—, y su CRC cambia cuando cambia algo de lo que el índice
depende. Es un CRC-32 y no un SHA-1 porque el SHA-1 del directorio cuesta 8 ms por build en wasm y el
CRC 1 ms (medido); aquí el digest no autentica, distingue contenido, y una colisión no puede servir
bytes equivocados porque cada entrada se comprueba contra su CRC al leerla.

La caché se invalida con el jar, no solo con los fuentes. Además de la cabecera de
`build/jarindice.bin`, la huella entra en `build/incremental.json` (`Data.AndroidJar`) y
`clasesAlDia()` la exige, así que cambiar el classpath tira también las clases reutilizadas de I3 —
la pregunta abierta de su crítico. El límite que queda, dicho explícito: un jar **reescrito con los
mismos bytes** no invalida nada, porque no hay nada que recompilar; y la huella no es criptográfica,
así que una colisión de CRC-32 entre dos jars distintos se leería como «el mismo jar» (una entre cuatro mil millones por cambio, y el dex del proyecto se comprueba por bytes en las pruebas de I3).

Comando, en el hospedador de Node con el jar montado en `/sdk/android.jar`. Tres procesos: cada
comando de la pestaña arranca una instancia nueva de Go, así que la primera pasada de un proceso es la
que paga un build.

```sh
cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$TMP/jar.wasm" .
node shim/navegador-sim.js --montar "$ANDROID_JAR:/sdk/android.jar" "$TMP/jar.wasm" \
  -test.run TestIndiceDelJarMideElAhorro -test.v
```

Milisegundos, tres procesos, en el orden en que los imprime la prueba:

| vuelta | `archive/zip` (como antes) | lector propio, caché vacía | caché puesta |
| --- | --- | --- | --- |
| 1.ª de un proceso nuevo | 102,1 / 88,3 / 92,2 | 25,6 / 21,0 / 23,5 | 10,2 / 8,3 / 10,0 |
| mediana de tres vueltas | 70,7 / 51,4 / 55,2 | 15,7 / 12,9 / 13,2 | 6,0 / 4,4 / 4,5 |

El proyecto mínimo (la plantilla de `apkc create`, `compilarProyectoJava` entero) pasa de 66,4-81,5 ms
a 25,3-34,3 ms en la primera vuelta de cada proceso.

**Lo que da y lo que no.** El ahorro por build es de **80-92 ms** (mediana 82) en la primera pasada de
un proceso nuevo, y de **47-65 ms** (mediana 51) en régimen. El criterio de aceptación era ≥100 ms y
**no se alcanza**: el techo es el propio zip, que cuesta 88-102 ms la primera vez y 51-71 en régimen, y
la caché ya se queda en 4-10 ms de eso. El umbral de abandono (<50 ms) queda justo en el borde en
régimen y no se toca en la primera pasada, que es la que paga un build de la pestaña. El criterio se
fijó antes de medir el coste (la medición 1 lo dejaba abierto), y la medición dice que el techo del
ahorro es el coste del zip. La vía siguiente es medirla en la pestaña, donde el jar vive en OPFS y el
zip lee 1,29 MB de disco contra los 230 KB de la caché: allí el ahorro debería ser mayor que en el
shim, donde el fichero está en memoria. Ese es el comando de I2 (`studio/prueba-pestana.js`).

Lo que las pruebas cierran además del cronómetro:

- `TestIndiceDelJarSeReutilizaYSeInvalida`: la vuelta con el mismo jar **no parsea** (lo dice el
  contador `parseosDelDirectorio`, porque por fuera caché y parseo se comportan igual); moverle la
  fecha al jar no reindexa; un jar con **el mismo tamaño y otra entrada** sí reindexa, y el índice de
  disco es el del jar nuevo.
- `TestClasesAlDiaSigueAlContenidoDelJar`: con el estado que deja una compilación, `clasesAlDia()` es
  cierto; cambiarle la fecha al jar lo deja cierto —la fecha no es contenido— y cambiarle los bytes
  con el mismo tamaño lo pone en falso, que es el caso que el crítico echaba en falta.

## Hallazgo del crítico: un recurso nuevo no reenlazaba (arreglado)

La sonda del crítico: proyecto mínimo, `build --inc --linker go --compiler go --v4=false`, una
primera pasada, y después **añadir** `<string name="critico_marcador">CRITICO_NUEVA_CADENA_7391</string>`
a `res/values/strings.xml` (y, aparte, **editar** el valor de una cadena que ya existía). Antes del
arreglo:

| paso | sha256 del apk (16) | `grep -ac` de la cadena en el apk |
| --- | --- | --- |
| primera pasada con `--inc` | `ddf790ac5a9ab250` | — |
| tras **añadir** la cadena, `--inc` | `ddf790ac5a9ab250` (el mismo) | **0** |
| tras **editar** una cadena, `--inc` | `ddf790ac5a9ab250` (el mismo) | **0** |
| control: build completo del mismo árbol | `5d4dbaf36a49f355` | **1** |

Causa, en tres piezas que encajan mal:

- `compileRes` devolvía **siempre `false`** con `--linker go` (`apkc/build.go:543`, «no hay flats que
  compilar»), y ese valor es el papel de «los recursos cambiaron» en la condición que llama a
  `bundleRes` (`apkc/build.go:258`).
- `pruneDeleted` (`apkc/utils.go:252`) solo mira BORRADOS: quita el registro de un fichero que ya no
  está y marca `resStale`. Un alta o una edición no pasaban por ahí.
- El manifiesto sí se detectaba (`manifestChanged`), y los `jar/`, `lib/` y `assets/` también (por
  roster). Los recursos eran el único hueco, y el apk anterior se servía entero: el incremental
  prometía lo que no cumplía.

Arreglo:

- `apkc/build.go:543` `compileRes`: con `--linker go` compara `res/**` por contenido
  (`getFiles("res", "", true)`) y devuelve `true` si hay algún fichero nuevo o editado, que es lo que
  obliga a `bundleRes`. Un borrado lo sigue marcando `pruneDeleted`.
- `apkc/build.go:601` `bundleRes`: tras `enlazarConGo`, con `--inc` registra los digests de todo
  `res/`, que es contra lo que compara la vuelta siguiente (igual que `aapt2 compile` registra los
  flats que consumió).

La misma sonda después del arreglo:

| paso | sha256 del apk (16) | `grep -ac` |
| --- | --- | --- |
| primera pasada con `--inc` | `332e9a99691bfd3d` | — |
| tras **añadir** la cadena, `--inc` | `2a904e74f5208b4e` (otro) | **1** |
| tras **editar** una cadena, `--inc` | `84d3e0e261f5bc78` (otro) | **1** |
| control: build completo del mismo árbol | `84d3e0e261f5bc78` (**el mismo** que la incremental) | **1** |

El comando de la sonda está abajo, en «Reproducción del hallazgo»; el apk de la pasada incremental
coincide byte a byte con el del build completo, que es la propiedad que se buscaba.

Tests que lo cierran:

- `apkc/incremental_recursos_test.go`, `TestIncrementalReenlazaCuandoUnRecursoEsNuevoOEditado`:
  `_build` con las mismas banderas que la pestaña, alta de una cadena (tiene que estar en
  `build/resources.arsc` y el apk tiene que cambiar), edición de un valor existente (lo mismo), y una
  vuelta sin tocar nada (el apk no se rehace), con `verificarApk` en verde en cada paso. Con el
  arreglo revertido el test falla con «con --inc, una cadena nueva no llegó a resources.arsc: no se
  reenlazó».
- `studio/prueba-pestana.js` (sección `12c`, paso `f`): lo mismo dentro de chromium, con la cadena
  buscada en los bytes de `/proyecto/build/resources.arsc` y las entradas del apk comparadas con la
  pasada anterior. `pestaña: 177 comprobaciones` (la cifra de esta medición, §201-§202; el árbol de
  hoy da 192).

Reproducción del hallazgo (sonda de shell, fuera del repo, con el binario de turno):

```sh
sh sonda-recursos.sh <apkc> /tmp/sonda   # proyecto mínimo + alta + edición + control completo
# imprime el sha256 corto del apk y grep -ac de la cadena en cada paso
```

Coste del arreglo: cada vuelta con `--inc` recorre y hashea `res/` una vez (79 ficheros en el banco
de 232 fuentes, la mayoría pequeños). No cambia el número de etapas ni el dex.

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

- **I1, pasar `--inc` desde la pestaña. HECHA.** `--inc` en la lista de `build` de
  `studio/pestana.js`; gate verde (177 comprobaciones **al cerrar esta fase**; el árbol de
  hoy da 192) y apk igual entrada por entrada.
  Prueba: sección `12c` de `studio/prueba-pestana.js`. Reproducción: `sh banco/incremental.sh`.
- **I2, medir en chromium y OPFS. HECHA.** Resultado: 1831 → 1034 ms con `--inc` solo (−44 %),
  1357 → 201 ms con I3 (−85 %), apk igual entrada por entrada. Criterio ≥15 % superado.
- **I3, reutilizar las clases emitidas. CERRADA (APROBAR del crítico).** `d3803bb`; la vuelta sin
  fuentes cambiados no compila (`compile go` 300 → 0 ms, dex idéntico). No cabe caché en memoria
  porque cada comando arranca una instancia nueva de Go (`studio/hospedador.js:240`); lo que queda
  es emitir solo fuentes cambiados. El crítico `ses_df9baefaeffemtyU4YC0gXywOR` dio **APROBAR** sin
  hallazgos `CONFIRMADO`; la pregunta abierta del classpath la resolvió I5 (la huella del jar entra en
  `clasesAlDia()`).
- **I4, dexado por clase (H2). HECHA EN NATIVO, GATE PENDIENTE.** `cmp build/classes.dex` entre el
  dexado completo y el de la vuelta que reutiliza: **idéntico** (§214 de `docs/evidencia.md`). El
  ensamblado tiene offsets absolutos, así que no se parchea el fichero anterior: se guarda por método
  lo que no nombra ningún offset —el `code_item` y el `debug_info_item`— en `build/dexcache.bin`, y
  solo vale mientras las cinco tablas ordenadas sean las mismas (`huellaTablas`); si entra una cadena
  o un método, la caché se tira y la vuelta cuesta lo de siempre. La clave es el SHA-1 del `.class`
  entero, de modo que cualquier cambio de la clase re-traduce todos sus métodos. Medido: `dex go`
  100 → 45 ms, 1875 `code_item`, 1866 reutilizados al editar un fuente. Prueba:
  `TestDexadoPorClaseReutilizaYNoCambiaUnByte` (cuatro vueltas: frío, edición con tablas iguales,
  cuerpo que **crece** —las áreas se desplazan y los offsets se recalculan— y clase nueva que mueve
  las tablas) y `TestCacheDexCorruptaCaeAlDexadoCompleto` (caché truncada, de otra versión, con una
  longitud imposible o con un byte cambiado: cae al dexado completo con aviso y el dex sale
  idéntico; §215). Lo que aún se paga en esta etapa —`leeClases` 11 ms,
  `collectClasses` 13 ms y el ensamblado de las áreas globales— no es I5: I5 es el índice del
  `android.jar` entre pasadas, que ataca a la compilación, no al dex.
- **I5, índice del `android.jar` entre pasadas (H3). MEDIDA, POR DEBAJO DEL CRITERIO.** El
  directorio central se lee una pasada y se deja en `build/jarindice.bin`; la vuelta siguiente lee ese
  fichero y no parsea el zip. Ahorro: 80-92 ms por build en wasm (mediana 82) y 47-65 ms en régimen;
  aceptación ≥100 ms, abandono <50 ms, así que se queda en medida y no en cerrada. La huella del jar
  (ruta, tamaño y CRC-32 del directorio central —la fecha no entra: en OPFS era sintética) invalida la
  caché **y** las clases reutilizadas de I3, que era la pregunta abierta de su crítico. Detalle en
  «I5: el índice del `android.jar` entre pasadas».

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
- El ahorro de I5 dentro de la pestaña: en el shim el jar está en memoria, y en OPFS el zip lee
  1,29 MB de disco contra los 230 KB de la caché, así que la cifra de allí debería ser mayor.
- Los apks de las repeticiones 1 y 2: solo se compararon los de la 3.
- Una medición con dependencias `jar/`, Kotlin o `--compiler javac` dentro del wasm.
