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

## 1. `javac` — resuelto por otros, con coste

- **Qué hace apkc con él**: fuente -> `.class`, con `-sourcepath src -implicit:class` y
  `android.jar` en el classpath. Ya lo hace en el sitio correcto (la closure de dependientes), desde
  el trabajo incremental.
- **Qué existe**: una demo oficial de GraalVM con **`javac` compilado a wasm con native-image** que
  compila fuente Java a bytecode dentro de la página, y **CheerpJ 3.x**, JVM completa en wasm con
  runtime OpenJDK.
- **Qué costaría**: bajar y enlazar uno de los dos, y darle un `fs` compartido con el `apkc.wasm`.
  Es integración, no escritura de compilador. CheerpJ es licencia comercial (free tier para
  no-comercial): **hay que leer la licencia antes de meterlo en nada que se publique**.

## 2. `d8` — la incógnita que manda

- **Qué hace**: `.class` -> `.dex`, y desugara según `--min-api`. apkc lo llama sobre el proyecto y
  sobre los cubos de dependencias, con estado propio en `build/dexcache`.
- **Qué existe**: **nada portable**. `d8` es un jar de Android (fuente en `build/soong` y en R8). En
  Go solo hay **analizadores** de dex (`dutchcoders/godex`), no escritores. Escribir un dexer en Go
  es escribir un compilador: formato de dex + asignación de registros + desugaring de lambdas,
  default methods y `invokedynamic`.
- **Lo único razonable**: correr el jar sobre la JVM wasm del punto 1. **No está probado que d8
  corra ahí** (usa `java.nio.file`, concurrencia y reflexiones sobre el classpath; las JVM wasm no
  soportan todo). Este es el experimento que decide si la ambición vive o muere, y vale más que
  cualquier otra línea de este repo.

## 3. `aapt2` — el que no tiene atajo

- **Qué hace apkc con él**: `compile` de `res/**` a `.flat`, y `link` que produce
  `resources.arsc`, el `AndroidManifest.xml` **binario** (AXML) y `R.java`. Sin esas cuatro salidas
  no hay apk instalable: las referencias `R.layout.main` se resuelven contra `resources.arsc`, y el
  manifest lo lee el paquete de Android, no un XML parser.
- **Qué existe**: **nada fuera del árbol de Android**. `aapt2` es C++ (`frameworks/base/tools/aapt2`)
  y ni apktool lo reimplementa: invoca al binario. No hay biblioteca mantenida que escriba AXML +
  `resources.arsc`.
- **Rutas**: (a) compilar `aapt2` a wasm con Cheerp/Emscripten — arrastra `libbase`, `libutils`,
  `androidfw` y un build de Android completo, probablemente más trabajo que (b); (b) **escribir en Go
  el subconjunto que apkc necesita**. Es la pieza cara y la que queda para siempre: formato AXML,
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

## Lo que la ambición no puede solucionar aunque porte todo

- **Memoria y tamaño**: `android.jar` del API 34 pesa decenas de MB, la JVM wasm otros tantos, y los
  demos reales de apkc son de 142 MB (un apk de `MnnChat`). Un build del proyecto Firebase (60 jars)
  en una pestaña es otro planeta respecto al teléfono.
- **El primer arranque**: el usuario de la página esperaría segundos, no la bajada de un runtime.
- **El `fs`**: hay que dar el `fs` de forma de Node (24 funciones) o reescribir la capa de ficheros
  de apkc. La primera opción está demostrada aquí; la segunda toca todo el repo.
