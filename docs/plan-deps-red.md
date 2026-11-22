# Plan: dependencias Maven y red en la pestaña

Estado: **investigación, sin implementar y sin autorizar** (2026-11-22). Nada de
este documento cambia la regla 4 de `AGENTS.md`; la decisión es del usuario y
está al final, en «Pregunta abierta». Todo lo que no trae comando ejecutado va
marcado HIPÓTESIS.

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

## 4. Arquitectura propuesta (solo si se autoriza)

Fase 0 — decisión. Sin la respuesta del usuario no se escribe código.

Fase 1 — importador sin red (no requiere autorización de red, pero sí de
producto). Entrada: directorio o `.zip` con la salida de `apkc-deps`. Salida:
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

Criterios de abandono comunes: si el importador cubre el caso de uso real (traer
una librería que el usuario ya tiene), las fases 3 y 4 mueren; si la cuota de
OPFS no aguanta ~7 MB por proyecto, se vuelve a vendorizar por proyecto y no por
catálogo.

## 5. Pregunta abierta

Para autorizar o rechazar el cambio de la regla 4 hace falta que el usuario
responda, literalmente:

> ¿Se permite que la pestaña haga peticiones de red en tiempo de ejecución? Si
> sí: ¿a qué orígenes exactos (solo `repo1.maven.org` y `dl.google.com`, o
> cualquiera), con qué propósito (solo descargar AAR/JAR/POM, o también
> resolver rangos y metadatos), y se mantiene como gate innegociable que la carga
> inicial y la compilación sin dependencias nuevas hagan **cero** peticiones
> fuera del origen (aunque la descarga explícita de una dependencia sí las haga)?

Sin esa respuesta, el trabajo sigue por la Fase 1, que no toca la regla.
