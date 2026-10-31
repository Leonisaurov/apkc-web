# Bloqueos

El build de apkc lanza cuatro programas. Para que una app se compile en el navegador, cada uno tiene
que dejar de ser un proceso. Esto es lo que hay y lo que costaría, por orden de incertidumbre.

## Dónde está la costura exacta

`os/exec` se usa en seis ficheros: `aab.go`, `apk.go`, `build.go`, `device.go`, `doctor.go` y los
`proc_*.go`. Ese es el listado real de lo que hay que convertir en interfaz, y también lo que acota
el trabajo: **no es "reescribir apkc"**, es dar una segunda implementación a esas llamadas.

La interfaz hace falta aunque el navegador consiguiera las herramientas: `js/wasm` no tiene
`fork/exec`, así que "ejecutar un programa" no existe como operación. En el teléfono hoy
`watch` se autoclora por eso mismo (`proc_stub.go`).

## 1. `javac` — hay que escribirlo

- **Qué hace apkc con él**: fuente -> `.class`, con `-sourcepath src -implicit:class` y
  `android.jar` en el classpath. Ya lo hace en el sitio correcto (la closure de dependientes), desde
  el trabajo incremental.
- **Qué existe fuera del árbol**: JVMs compiladas a wasm de proveedores que las sirven desde su propio
  CDN. **Fuera por decisión del 2026-10-27**: ese uso gratuito exige red y dependencia de un tercero,
  así que no puede cumplir "sin conexión". No hay ruta de prestación: se emite en Go.
- **Qué queda**: emitir el bytecode desde Go, con `javap` como oráculo, **para el subconjunto que
  necesita el proyecto del `studio/`** y con la frontera escrita donde el subconjunto se acaba. La
  parte que no es traducción es la comprobación de tipos contra `android.jar`.

## 2. `d8` — la que decide, y ya tiene objetivo medido

- **Qué hace**: `.class` -> `.dex`, y desugara según `--min-api`. apkc lo llama sobre el proyecto y
  sobre los cubos de dependencias, con estado propio en `build/dexcache`.
- **Qué existe**: **nada portable, y ya no se busca**. En Go solo hay **analizadores** de dex
  (`dutchcoders/godex`), no escritores.
- **El tamaño real del problema, medido** (`docs/evidencia.md` §13): el proyecto del `studio/` entra
  como 6 `.class` (3333 bytes) y sale como **2844 bytes de dex con 217 objetos** en 17 tipos de
  sección. Eso es lo que hay que emitir para una app mínima: un artifact finito y comparable byte a
  byte contra `d8`.
- **Donde sí es otro proyecto**: el desugaring de lambdas, `invokedynamic`, default methods y todo lo
  que aporten dependencias ajenas. **Fuera del plan**, con el motivo escrito; no se promete.

## 3. `aapt2` — el que no tiene atajo

- **Qué hace apkc con él**: `compile` de `res/**` a `.flat`, y `link` que produce
  `resources.arsc`, el `AndroidManifest.xml` **binario** (AXML) y `R.java`. Sin esas cuatro salidas
  no hay apk instalable: las referencias `R.layout.main` se resuelven contra `resources.arsc`, y el
  manifest lo lee el paquete de Android, no un XML parser.
- **Qué existe**: **nada fuera del árbol de Android**. `aapt2` es C++ (`frameworks/base/tools/aapt2`)
  y ni apktool lo reimplementa: invoca al binario. No hay biblioteca mantenida que escriba AXML +
  `resources.arsc`.
- **Rutas**: (a) compilar `aapt2` a wasm con una cadena C->wasm autoalojable (Emscripten) — cumple lo
  de sin conexión, pero arrastra `libbase`, `libutils`, `androidfw` y un build de Android completo;
  probablemente más trabajo que (b); (b) **escribir en Go el subconjunto que apkc necesita**. Es la pieza cara y la que queda para siempre: formato AXML,
  tabla de recursos con sus config flags, tipos de referencia, y `R.java`. La ventaja es que apkc ya
  tiene medio camino hecho: lee y escribe formatos de bajo nivel propio (`classfile.go`,
  `zipwriter.go`, `apksig.go`) y tiene oráculo en el teléfono.

## 4. `kotlinc`

Ninguna expectativa de que corra en el navegador (es un compilador mayor, sobre JVM, y hoy cuesta
**26.4 s** incluso nativo). Si alguna vez existe la versión navegador, el proyecto en Kotlin queda
fuera: **el lenguaje del lado cliente sería Java**, o lo que se invente después.

## 5. Lo que ya no bloquea

`zipalign` y `apksigner` **dejaron de ser necesarios**: el escritor propio alinea por construcción y
la firma v1+v2 se escribe en Go (`zipwriter.go`, `v1sign.go`, `apksig.go`, `gopackage.go`), con los
lectores de keystore propios (`pkcs12.go`, `jks.go`). Medido aquí: ese bloque completo pasa 88 tests
en un hospedador sin Node. Quedan en el backend JVM tres cosas concretas —un `.aab`, un keystore
JCEKS, y un `minSdkVersion` < 18— que no son del navegador.

**Actualizado al cerrar las fases (§40 a §46)**, porque este documento se escribió antes del trabajo: de
los cuatro programas que apkc lanza, **tres tienen sustituto en Go y corren dentro de chromium** para el
subconjunto medido — `javac` lo sustituye `apkc/javaemisor.go` con `apkc/compilar.go` (§40 a §43), `d8` lo
sustituye el dexer de la Fase 2G (§20 a §23, y `d8` releee ese dex con `rc 0` en §46) y `aapt2` lo sustituye
`build --linker go` (§28 a §38). El cuarto, `zipalign`, ya no hacía falta por el párrafo de arriba. El coste
que se argumenta en los puntos 1 a 4 sigue escrito tal cual se midió: es la razón por la que se hizo en este
orden, no una predicción pendiente.

**Lo que de verdad sigue bloqueando el navegador**: `kotlinc` (26,4 s incluso nativo, y es compilador
ajeno), y el `build` sin banderas, que es el que llama a las herramientas externas. Y el subconjunto de Java
que no está medido: el emisor lo rechaza por nombre (§41, §42) en vez de improvisar.

## Lo que la ambición no puede solucionar aunque porte todo

- **Memoria y tamaño**: `android.jar` del API 34 pesa decenas de MB y los demos reales son de 142 MB
  (un apk de `MnnChat`). Con la decisión de no prestar runtime, lo que baja a la pestaña es
  `apkc.wasm` (9,46 MB medidos) más lo que se resuelva de SDK; un build del proyecto Firebase (60
  jars) en una pestaña sigue siendo otro planeta respecto al teléfono.
- **El primer arranque**: sin descarga de terceros, el coste es compilar e instanciar el wasm. Medido:
  `create` ~1,3 s y `sign` ~1 s dentro de chromium, incluyendo la bajada local del binario.
- **El `fs`**: hay que dar el `fs` de forma de Node (24 funciones) o reescribir la capa de ficheros
  de apkc. La primera opción está demostrada dos veces: en el shim de node y en chromium con OPFS
  (`docs/evidencia.md` §11).
- **Lo que la decisión NO quita**: sigue sin haber quien escriba el `.dex`, el AXML y
  `resources.arsc` por nosotros. Al negarnos el runtime prestado, el trabajo de las fases 2G, 1G y 3
  pasa a ser entero nuestro; es más lento y es el único camino que cumple "sin conexión". *(Escrito antes
  de hacerlas: hoy ese trabajo está hecho y medido —§20 a §23, §28 a §38 y §40 a §46—, y lo que queda de
  este punto es el tamaño: el `android.jar` de 26 MB y los ~9,5 MB de `apkc.wasm` siguen viajando a la
  pestaña.)*
