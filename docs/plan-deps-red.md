# Plan: dependencias Maven y red en la pestaña

Estado: **autorizado y priorizado por el usuario** (2026-11-22). La vía es importar
artefactos desde disco u OPFS (Fase 1 y Fase K); la regla 4 de `AGENTS.md` sigue intacta y
cualquier descarga en runtime está prohibida. La Fase J está cerrada y la Fase K, aprobada en
nativo. Todo lo que no trae comando ejecutado va marcado HIPÓTESIS.

## Resumen ejecutivo

**Viabilidad de traer `apkc-deps` a la pestaña: sí sin red, no con red y regla 4
intacta.** Hay tres caminos reales y solo dos son compatibles con la regla
vigente:

1. **Importar**: el usuario resuelve con `apkc-deps` en su escritorio (Python,
   ya funciona y tiene `--offline`) y la pestaña ingiere `jar/`, `lib/<abi>/` y
   `res/__deps/` desde disco u OPFS. Cero peticiones, regla 4 intacta, coste
   acotado a un importador.
2. **Vendorizar** los artefactos en el propio origen (como ya se hace con el
   `android.jar`): cero peticiones en runtime, pero el sitio crece y hay que
   revisar la licencia de cada AAR que se redistribuya.
3. **Descargar en vivo** de Maven Central o Google Maven: **no funciona hoy**,
   porque ninguno de los dos manda `Access-Control-Allow-Origin`; y arreglarlo
   exige un proxy ajeno al origen, que es exactamente lo que la regla 4 prohíbe.

La creencia de que «un worker evita CORS» es falsa. El proyecto que inspiró la
pregunta no lo hace así: túnela TCP por WebSocket contra un proxy Wisp.

## 1. Qué es «umkernel» (y por qué no evita CORS)

HECHO VERIFICADO. No existe ningún proyecto llamado «umkernel» con la conducta
descrita: la búsqueda local (cero coincidencias en `Develop/`) y la web solo
devuelven la opción `umkernel=on` de QEMU para Malta/MIPS, que no tiene relación.
El proyecto real es **`pithings/unkernel`**, «A real Linux kernel, compiled to
WebAssembly», y el usuario casi con seguridad se refería a él.

- README: <https://github.com/pithings/unkernel> (1,1k estrellas, creado el
  2026-11-12). Licencias: **GPL-2.0 y MIT**; el SDK de npm es MIT según su
  README, el kernel es GPL-2.0.
- Cita literal del README: «Browsers can't open raw TCP sockets, so the page
  reaches the internet through a Wisp proxy. The default one accepts only known
  origins, so if you host the page somewhere else, point `?wisp=` at your own
  proxy.»
- API del SDK: `wispBackend(DEFAULT_WISP_SERVERS)` frente a `directBackend()`
  (sockets del host, solo Node). O sea: en el navegador **siempre** hay un
  servidor intermedio; no hay modo cliente puro.
- Wisp es un protocolo de **proxy de sockets TCP/UDP sobre un WebSocket**:
  <https://github.com/MercuryWorkshop/wisp-protocol>, licencia **CC-BY-4.0**
  (medido por la API de GitHub).

Conclusión técnica: unkernel no burla CORS. El invitado tiene su propia pila
TCP/IP en JS y cada conexión sale por un WebSocket hacia un relay que sí abre el
socket TCP. El navegador no aplica CORS al handshake de WebSocket, pero el
servidor existe, es de alguien y ve todo el tráfico. Es un proxy con otro
nombre: la misma clase de pieza que la regla 4 excluye.

HIPÓTESIS (no medida): el motivo de usar WebSocket y no `fetch` no es CORS sino
que el invitado necesita TCP crudo, no HTTP; para bajar un `.aar` suelto bastaría
un proxy HTTP normal, más simple y con el mismo coste de regla.

## 2. ¿Es técnicamente posible en este sitio?

HECHO VERIFICADO, medido con `curl` contra los repos reales, con cabecera
`Origin` del sitio publicado:

```sh
curl -sI -H "Origin: https://apkc-studio.surge.sh" \
  https://repo1.maven.org/maven2/com/google/code/gson/gson/2.10.1/gson-2.10.1.pom
# HTTP/2 200, content-type: text/xml, SIN access-control-allow-origin
curl -sI -H "Origin: https://apkc-studio.surge.sh" \
  https://dl.google.com/dl/android/maven2/androidx/core/core/1.9.0/core-1.9.0.pom
# HTTP/2 200, content-type: application/octet-stream, SIN access-control-allow-origin
curl -sI https://raw.githubusercontent.com/google/gson/main/README.md
# HTTP/2 200, access-control-allow-origin: *
```

Probados también `repo.maven.apache.org` (mismo resultado que `repo1`) y
`maven-central.storage-download.googleapis.com` (sin CORS). `maven.google.com`
responde 301 a `dl.google.com`, que tampoco manda la cabecera. Por tanto un
`fetch()` de la página a Maven Central falla en el navegador; no es una limitación
de wasm ni de Go, es CORS.

Qué puede y qué no puede cada pieza que ya existe:

- **`studio/sw.js`** intercepta solo peticiones del mismo origen y lo dice en un
  comentario (`sw.js:34-36`): «Las peticiones de otro origen se descartan antes
  (no se pueden reescribir respuestas opacas)». Un Service Worker no es un proxy
  CORS: no puede leer el cuerpo de una respuesta opaca (`mode:'no-cors'` →
  `type: "opaque"`, `status: 0`, cuerpo ilegible), así que no puede entregar bytes
  a wasm. Y si reemite la petición en modo `cors`, sigue sin cabeceras y falla
  igual: la comprobación la hace el navegador, no el SW.
- **`studio/hospedador.js`** es un DedicatedWorker de módulo con OPFS porque los
  access handles sincrónicos solo existen ahí; un Worker hereda las reglas de
  origen de su página. Mover el `fetch` al worker no cambia ni una cabecera.
- **`tools/sitio.sh`** empaqueta estáticos: mete `studio/`, `apkc.wasm` y
  `wasm_exec.js`, y no puede añadir un backend (el host es Surge, sin funciones).
- **`studio/prueba-pestana.js:3303`** comprueba que ninguna petición salga del
  origen (`ajenas.length === 0`). No es una convención: es un test que hoy pasa.

Cuadro de opciones (coste = esfuerzo de ingeniería, no bytes):

| Opción | Licencia / dependencia | Coste | Qué regla rompe |
| --- | --- | --- | --- |
| A. Importar `jar/`, `lib/`, `res/` desde disco a OPFS | ninguna nueva; usa el `apkc-deps` actual (Python) en el escritorio | medio: selector de directorio, validación, cuota | ninguna (cero red) |
| B. Vendorizar AAR/JAR en el origen, con índice y `deps.lock.json` | la de cada artefacto; AndroidX/Material son Apache-2.0, no todas lo son | medio-alto: peso del sitio y auditoría de licencias | ninguna si se sirve del propio origen; sí la 2 si se versionan binarios |
| C. Proxy CORS propio (Cloudflare Worker, `corsproxy.io`, etc.) | servicio de terceros + su política | bajo en código, alto en dependencia | **regla 4** (red en runtime y tercero en el camino) |
| D. Relay Wisp/WebSocket como unkernel | spec CC-BY-4.0; servidor propio o ajeno | alto: proxy + cliente TCP | **regla 4**, y añade un servidor que ve las descargas |
| E. `fetch` directo a Maven | — | nulo | técnicamente imposible: sin CORS (medido) |
| F. Extensión de navegador o servidor en `localhost` | — | alto para el usuario | «sin instalar nada» y regla 4 |

## 3. Qué es `apkc-deps` y qué costaría traerlo

HECHO VERIFICADO, leído en `/home/leonisauro/Develop/Python/apkc-deps/`:

- Es **un solo fichero Python 3 de 132.630 bytes, solo stdlib** (`AGENTS.md`:
  «sin `pip`, sin Maven, sin Gradle»), shebang absoluto de Termux. No es Go y no
  tiene versión wasm: para la pestaña habría que reescribirlo o portarlo.
- Resuelve el grafo Maven con orden de repos `google` → `central` → `m2local`,
  colapsa rangos, aplica BOM, verifica sha1 y deja `deps.lock.json` determinista;
  instala en `jar/`, `lib/<abi>/` y `res/__deps/` (formato que `apkc` entiende).
  Tiene `--offline` sobre `~/.m2` y `$HOME/.cache/apkc-deps`.
- Necesita red para: `maven-metadata.xml` de rangos, POM de cada nodo, AAR/JAR de
  cada artefacto y los sidecar de checksum. HIPÓTESIS: ~2-3 peticiones por
  artefacto; una app con AndroidX + Material + Kotlin ronda las 25-60 peticiones.
- El fork `apkc/` **no tiene nada de Maven** (grep sin resultados fuera de
  tests): hoy la resolución vive solo en el script Python.

Tamaños reales medidos (`content-length` por `curl -sI`), para dimensionar la
opción B:

| Artefacto | Bytes |
| --- | --- |
| `androidx.core:core:1.9.0` (.aar) | 1.115.684 |
| `androidx.appcompat:appcompat:1.7.0` (.aar) | 1.148.962 |
| `com.google.android.material:material:1.12.0` (.aar) | 2.323.473 |
| `androidx.core:core-ktx:1.13.1` (.aar) | 171.581 |
| `org.jetbrains.kotlin:kotlin-stdlib:2.0.21` (.jar) | 1.747.660 |
| `com.google.code.gson:gson:2.10.1` (.jar) | 283.367 |

Es decir: una app pequeña con esas seis piezas son **~6,8 MB** de artefactos,
más recursos y `.so`; comparable al `android.jar` completo (26,4 MB) que hoy se
pide una vez y se guarda en OPFS. HIPÓTESIS: la cuota de OPFS lo absorbe, pero no
está medido en el aparato del usuario.

Lo que se puede hacer **sin red** y no exige tocar la regla 4:

- Reutilizar el `apkc-deps` de escritorio y añadir un **importador** en la
  pestaña: elegir un directorio `.m2`/proyecto con File System Access, o soltar
  un `.zip`, y escribir `jar/`, `lib/<abi>/`, `res/__deps/` en OPFS. Es la
  continuación natural de lo que ya se hace con el `android.jar` (§154,
  `plan-android-jar-propio.md`).
- Vendorizar un catálogo curado (p. ej. Gson + AndroidX + Material) servido del
  propio origen, con sus licencias al lado y `deps.lock.json` versionado.

Lo que **exige** cambiar la regla 4: cualquier descarga en tiempo de ejecución,
sea `fetch` (imposible sin CORS), sea a través de un proxy CORS, sea un relay
Wisp. No hay una cuarta vía: el navegador no permite leer bytes de otro origen
sin cooperación del servidor.

## 4. Arquitectura aprobada: importar sin red

Fase 0 — decisión. Resuelta el 2026-11-22: el usuario autorizó y priorizó la importación sin
red (Fase 1; la Fase J ya está cerrada). La red en runtime sigue fuera.

Fase 1 — importador sin red. Entrada: directorio o `.zip` con la salida de `apkc-deps`. Salida:
`jar/`, `lib/<abi>/`, `res/__deps/` en OPFS. Medir con
`APKC_WASM=… node studio/prueba-pestana.js` que sigue habiendo **cero**
peticiones fuera del origen, y con `navigator.storage.estimate()` la cuota real
antes y después.

Fase 2 — catálogo vendorizado (opcional). Índice JSON + blobs en el origen,
`deps.lock.json` con sha256 y licencia por artefacto. Criterio de abandono: si el
peso del sitio supera lo que el host acepta o alguna licencia no permite
redistribuir, se limita a las piezas Apache-2.0.

Fase 3 — resolución en Go dentro de `apkc/` (HIPÓTESIS, solo con red
autorizada). Puerto del resolvedor a Go con `net/http`; en wasm usa `fetch` y
por tanto **hereda CORS**, así que solo funcionaría contra orígenes que manden
`Access-Control-Allow-Origin` (medido: GitHub raw sí; Maven Central y Google
Maven no). Antes de escribir nada habría que medir qué repos lo mandan y si hay
espejo con CORS.

Fase 4 — proxy (solo si la regla 4 se cambia explícitamente). Fuera del alcance
de este repo mientras la regla siga en pie.

Fase J — `jar/` en el compilador Go (**cerrada**). El usuario quiere que
`apkc compile/build --compiler go` consuma `jar/*.jar` y `classes.jar` de los
`.aar`, sin red y sin `apkc-deps` en runtime. Criterio de aceptación: un proyecto
con un jar en `jar/` compila con el compilador Go y las clases del jar resuelven
en el dex; se mide con un test enfocado y se verifica en la pestaña con cero
peticiones fuera del origen. Frontera: no se resuelven versiones ni se descargan
artefactos; el catálogo y la red siguen fuera. Subestados:

- **Clases aprobadas en nativo** (`5bc1e8c`, condiciones cerradas en `8d6ee67`;
  crítico `ses_df91c8423ffeRBUEJpqWc5sSv5`). 7/7 tests, dex idéntico a javac 17,
  `classes.jar` de AAR, índice por jar e invalidación de I3. Su frontera de dex
  con `d8` quedó cerrada por el subestado siguiente.
- **Importador aprobado** (`be7b417`, `studio/` en `66c35ab`; crítico
  `ses_df91c58e7ffeuwOqrjEwncRX3Z`): sin `fetch`, Maven ni proxy; rutas OPFS
  `jar/`, `lib/<abi>/`, `res/__deps/<hash>/…` y `deps.importado.json`; traversal
  `../` rechazado. Notas no bloqueantes: TOCTOU del `confirm`, atomicidad ante
  cuota y hash de isla FNV frente al `sha1(coord)[:12]` de `apkc-deps`.
- **Fixture aprobado** (`8d99e52` y `aee59d4`; crítico
  `ses_df911247affeIZ83mX0fq3TTsR`): camino nativo de 1136/728 bytes y camino Go
  con `rc 3` y `OBJETIVO PENDIENTE`.
- **Dex en Go aprobado** (`3bc6c52`, corregido en `e84a64f`; crítico
  `ses_df90847d1ffeaWfOT8dxl9zhB3`): el dexer propio escribe el `classes.dex` de
  las dependencias sin `d8`, con caché por bucket en
  `build/dexcache/deps-gocache-*.bin`, sin pisar `build/dexcache.bin` ni dejar
  `.tmp`. 4 tests, `rc 0` del fixture y preview `sha256 90fc0011…`. Evidencia:
  §220 de `docs/evidencia.md`.
- **No medido / trabajo aparte**: el dexer propio escribe un `classes.dex` por
  bucket y no parte al superar 65 536 referencias de método; un `invokedynamic`
  no medido falla nombrando jar, entrada y sitio.

Fase K — recursos `res/__deps/` y `lib/<abi>/` (**aprobada en nativo**). `976b02e`
(`feat(apkc): enlaza recursos y lib/ de dependencias`) lee la isla `res/__deps/`, valida una carpeta
por artefacto y tipo y escribe la salida como `res/<tipo>/x.xml` sin la isla; rechaza con nombre de
fichero un recurso de fichero duplicado entre app y AAR, como el «duplicate value» de `aapt2`.
`packagePlan` ya metía `lib/**`. El crítico `ses_df915d191ffetJ53F3oGQiKaI1` dio **APROBAR** sin
hallazgos confirmados: atacó la ruta de salida, el duplicado, la entrada `lib/x86/libejemplo.so`, el
rebuild incremental al cambiar el `.so` y la regresión de I3/I4/I5. Fronteras declaradas, no
defectos: tipos de recurso no medidos (`drawable` XML/vectorial, `anim`, `font` y algunos `values`),
dos AAR con la misma clave `values` (solo se detectan ficheros duplicados) y el `AndroidManifest.xml`
y `R.txt` del AAR fuera. Evidencia: §216 de `docs/evidencia.md`.

Criterios de abandono comunes: si el importador cubre el caso de uso real (traer
una librería que el usuario ya tiene), las fases 3 y 4 mueren; si la cuota de
OPFS no aguanta ~7 MB por proyecto, se vuelve a vendorizar por proyecto y no por
catálogo.

## 5. Decisión del usuario (2026-11-22)

El usuario **autorizó y priorizó** el frente de dependencias por la vía sin red: importar los
artefactos que ya tiene desde disco u OPFS y que el compilador Go los consuma. Eso resuelve la
Fase 0 y deja la Fase 1 como trabajo activo; la Fase J está cerrada y la Fase K, aprobada en
nativo.

La pregunta de red en runtime **no se reabre**: la regla 4 sigue vigente y la respuesta es no.
Proxy y Wisp siguen descartados.

## 6. Registro de estado

- Fase J **cerrada**: clases aprobadas en nativo (`5bc1e8c`/`8d6ee67`), importador aprobado
  (`be7b417`/`66c35ab`), fixture aprobado (`8d99e52`/`aee59d4`) y dex en Go aprobado
  (`3bc6c52`/`e84a64f`).
- **J-bis, reabierta el 2026-11-23 por evidencia real**: con `jar/android-support-v4.jar`, la pestaña
  dexea el bucket y falla en `ActionBarDrawerToggle$SlideDrawable` con `byte 0x76: opcode outside the
  table this fork measured`. `0x76` es `invoke-direct/range`, no medido por el traductor. El cierre con
  fixtures era válido para lo medido, pero no para ese jar real; queda en curso con un agente
  específico (`dex-opcode76`) que añadirá la familia `/range` que falte y su prueba. Hasta que pase,
  la Fase J no se da por terminada frente a dependencias reales.
- No medido / trabajo aparte: un `classes.dex` por bucket, sin partir a las 65 536 referencias de
  método; `invokedynamic` no medido.
- Fase K **aprobada en nativo** (`976b02e`, crítico `ses_df915d191ffetJ53F3oGQiKaI1`), con sus
  fronteras de tipos de recurso, `values` duplicados entre AAR y manifiesto/`R.txt` fuera.
- Opción A (importar desde disco u OPFS) es la vía.
- Vendorizar un catálogo curado queda detrás.
- Proxy y Wisp siguen descartados.
- Sigue prohibido cualquier acceso de red en runtime; la carga inicial y la compilación sin
  dependencias nuevas conservan el gate de cero peticiones fuera del origen.
