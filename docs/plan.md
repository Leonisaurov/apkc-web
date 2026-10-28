# Plan — compilar una app Android en el navegador con apkc

## Objetivo y definición de éxito

**Éxito**: abrir una página en el navegador, escribir o pegar el fuente de una app mínima
(`Activity` + layout), y obtener un `app.apk` **instalable y verificado**, sin server, sin SDK, sin
JDK instalados en la máquina del usuario **y sin conexión**: todo lo que compila y empaqueta viaja en
el propio origen de la página. Comprobable: ninguna petición fuera del origen (`evidencia.md` §15),
ningún runtime de terceros. Se demuestra con tres hechos, todos comprobables con
oráculos ajenos a apkc:

1. `apksigner verify --verbose` (en otra máquina) dice v1 y v2 correctos.
2. El apk se instala en un teléfono real y la actividad arranca.
3. El `classes.dex` producido en el navegador es **idéntico byte a byte** al que produce `d8`
   nativo sobre los mismos `.class` (o diferido con motivo escrito).

**No es éxito**: empaquetar y firmar algo que ya se compiló en otro sitio. Eso es lo único que hoy
funciona del lado cliente y está documentado, pero no es el objetivo.

## Reglas de orden (por qué este orden y no otro)

- **El orden lo marca ahora el oráculo más barato, no la dependencia técnica.** 2G se puede validar
  byte a byte contra `d8` sin teléfono y sin licencia; 3 se valida con `aapt2 dump` y con el
  dispositivo; 1G con `javap`. Por eso 2G va primero.
- **No se abre 1G hasta que 2G emita un dex que el sistema lea.** De nada sirve un compilador Java
  cuyo output no se puede convertir en nada instalable.
- **Nada de este repo entra en `../apkc`** salvo la Fase 0, que es trabajo de la herramienta en sí.
- **Ninguna fase se declara "imposible" sin el texto del error y el comando que lo produjo.** El
  muro de esta ambición se encontró así: `aapt2 compile never started: pipe: not implemented on js`.

## Estado

| Fase | Estado | Evidencia |
| --- | --- | --- |
| 0.1 mensajes de herramienta | **hecha** (apkc `e7bfd01`) | `[build] aapt2 compile never started: pipe: not implemented on js` |
| 0.2.1 `apkc sign` | **hecha** (apkc `a909214`) | `docs/evidencia.md` §7 y §8: `apksigner`, `zipalign -c -p`, `jarsigner` y `zipcheck.py` sobre el apk re-firmado, y el mismo apk firmado **dentro de wasm** en 1,20 s |
| 0.2.2 `apkc verify` | pendiente | — |
| 1 y 2 (runtime JVM prestado) | **cerradas por decisión del usuario el 2026-10-27**: sin conexión y sin terceros. `d8` sobre una JVM wasm **nunca se probó**, y ya no se probará así | `docs/evidencia.md` §15: la pestaña no hace ni una petición fuera de su origen |
| 2G | **empezada**: lector de dex hecho (`18c9773`), escritor pendiente. Objetivo medido: 2844 bytes, 217 objetos | `docs/evidencia.md` §13 y §16 |
| 3 recursos sin `aapt2` | necesaria: sin `R` no hay las 6 clases que dexar | — |
| 1G | Java -> `.class` en Go, subconjunto del `studio/`, con `javap` de oráculo | — |
| 4.1 etapas como interfaz | pendiente | — |
| 4.2 hospedador de navegador | **hecha**: OPFS + worker + chromium, conducido y verificado por fuera (`docs/evidencia.md` §11, 16 comprobaciones) | `studio/prueba-pestana.js` |
| 4.3 la página | **en marcha**: editor de fuente sobre OPFS, `create` y `sign` dentro de la pestaña, **sin conexión y sin CDN**; `build` sigue muriendo en el muro conocido | `docs/evidencia.md` §11 y §15 |
| 4.4 empaque y tamaño | medida parcial: `apkc.wasm` 9,46 MB; el `android.jar` (25 MB) aún no hace falta porque no hay compilador | — |

---

## Fase 0 — apkc correcto (esto es de `../apkc`, pero lo manda este plan)

### 0.1 · nombrar la herramienta que falló — **hecha**

Seis sitios logueaban `string(out)` y descartaban `err`. Ahora pasan por `toolFailure(nombre, out,
err)`: si la salida está vacía, la línea dice qué herramienta no arrancó y por qué. Test propio en
`utils_test.go`.

### 0.2 · `apkc sign` y `apkc verify` sobre un apk existente

**Por qué importa**: la firma de apkc ya es Go y ya corre en hospedador de navegador, pero vive
dentro de `build`. Sin comando suelto no hay nada ejecutable del lado cliente.

- **0.2.1 `apkc sign <apk> --keystore ... --storepass ... [--out ...]`**
  - Qué: reempaqueta el apk dado con el escritor propio (`zipwriter.go`) y le escribe v1+v2
    (`v1sign.go`, `apksig.go`). Sin `aapt2`, sin `javac`, sin `d8`.
  - Aceptación: sobre `build/app.apk` generado por un build normal, el apk re-firmado verifica v1 y
    v2 con `apksigner verify --verbose`, y `zipcheck.py` no encuentra violaciones. Con `--keystore`
    inexistente, la negativa es la de `requireKeystore` (`e7bfd01`), no una pila de Java.
  - Oráculos: `apksigner verify`, `jarsigner -verify`, `zipcheck.py`.
  - Riesgo concreto a controlar: el plegado de 72 bytes de `java.util.jar` — el fallo que dejó 16 de
    82 entradas sin firmar vive aquí. Test de cobertura nombre a nombre contra `unzip -l`, como en el
    harness.
  - Esfuerzo: una sesión. Es mover llamadas existentes a un comando, no escribir formato.

**Hecha (apkc `a909214`), y tres cosas salieron distintas de como estaban escritas aquí:**

- El riesgo concreto del plegado de 72 bytes no reapareció: el re-firmado conserva las entradas
  originales y el manifiesto se comprueba nombre a nombre contra lo que se lee del archivo (test
  propio), que es la misma defensa que el build.
- Hay una regla nueva que el plan no preveía: **los directorios viajan en el paquete y fuera del
  manifiesto**, leída de `jarsigner` (§9 de evidencia). Un apk de otra procedencia siempre los
  lista; un plan de build nunca.
- Y otra: **el nombre del firmante no es `ANDROID`**, este keystore de debug hace escribir
  `ANDROIDD.SF` a `apksigner` (§9). Descartar la firma anterior por el nombre que apkc usa dejaría
  dos firmantes en un paquete.
- Criterio de aceptación cumplido, con un número que no estaba en el plan: **1,20 s firmando dentro
  de wasm** frente a 30 ms nativo, en el hospedador de forma-navegador (§8).

### 0.2.2 `apkc verify <apk>`
  - Qué: lee el APK Signing Block, comprueba los digests de las entradas y la firma v2 contra el
    certificado; imprime quién firmó y hasta cuándo es válido.
  - Aceptación: los tres apks de demostración dan el mismo resultado que `apksigner verify
    --verbose`; un apk alterado en un byte de un asset da negativo.
  - Esfuerzo: 2-3 sesiones (el parser del bloque ya existe para escribir; falta la ruta de lectura y
    la comprobación de firmas).

---

## Fase 1 y 2, reescritas el 2026-10-27

**Decisión que cambia el plan** (indicación del usuario, registrada abajo): la compilación tiene que
ocurrir **entera en el navegador, sin conexión y sin runtimes de terceros**. La ruta anterior —
prestar una JVM compilada a wasm para correr dentro de ella el `javac` y el `d8` reales— quedó
cerrada por eso, no por su resultado: nunca
se probó `d8` sobre una JVM wasm y ya no va a probarse así. Lo que queda es **que Go emita** lo que
hoy emiten esas dos herramientas, y por eso el orden del plan se reescribe desde el riesgo real.

Consecuencia de método: cada etapa nueva entra en `../apkc` con su propio commit, su test y su gate
(regla 3 del repo), y **nada se declara capaz de un subconjunto que no haya emitido**. Se escribe el
subconjunto del `studio/`, no "Java".

---

## Fase 2G — `classes.dex` en Go (la que decide todo ahora)

**Qué prueba**: que Go escriba un `classes.dex` a partir de los `.class` del proyecto del `studio/`,
con `d8` nativo como oráculo, y que ese dex quepa en un apk que se instala y arranca.

Objetivo medido (`docs/evidencia.md` §13): 6 `.class` (3333 bytes) -> **2844 bytes de dex**, 17 tipos
de sección, **217 objetos** (53 cadenas, 23 tipos, 9 protos, 4 campos, 17 métodos, 6 class defs,
9 code items, 7 class data, 6 debug info, 21 de anotaciones).

- **2G.1** Lector del `.class` y del `.dex`. **Hecha la mitad del dex** (apkc-web `18c9773`):
  `apkc/dexreader.go` lee cabecera, `map_list`, las cinco tablas de índices y `class_defs`, con el
  `classes.dex` real de `d8` de fixture y sus cuentas como aserciones (`docs/evidencia.md` §16; 98 a 105
  tests). **Hecha también la lectura de `code_item`** y del `class_data_item` (§20): los nueve cuerpos y
  los recuentos por clase casan con `javap`.
- **2G.2** Escritor de secciones: `string_ids`/`type_ids`/`proto_ids`/`field_ids`/`method_ids`, orden
  canónico (los índices salen de `d8`, no de un capricho nuestro) y `map_list`.
  **Hecho y medido**: internadas desde los `.class`, las tablas son iguales en contenido y en orden a
  las de `d8` (§19), y el fichero completo lo ensambla `apkc/dexarma.go` con su `map_list`, su firma
  `sha1` y su `adler32` (§22).
- **2G.4** Comparación byte a byte con `d8` sobre **los mismos `.class`**. **Medida sección por
  sección** (§22): `type_ids`, `field_ids` y `method_ids` salen byte a byte iguales; `proto_ids`
  difiere solo en los offsets absolutos; cada `string_data_item` es idéntico. El fichero entero son
  2392 bytes contra los 2844 de `d8`, y la diferencia está nombrada: no emitimos `debug_info_item`,
  ni el bloque de anotaciones, ni la cadena con la que `d8` se firma a sí misma. En los cuerpos la
  secuencia de instrucciones, los índices y el número de palabras coinciden en los nueve métodos;
  seis de nueve salen idénticos hasta los números de registro (§21). **El oráculo que cierra el
  paso**: `d8` lee el dex que escribimos y lo re-dexa sin un solo error (`rc 0`), y sobreviven las
  seis clases, los nueve cuerpos y las tres constantes de `R` leídas de nuestro `static_values`.
  Falta la prueba de dispositivo, que necesita la Fase 3 (un dex suelto no se instala).
- **2G.3** `class_data_item`, `code_item` y `debug_info_item`: la codificación de instrucciones en sí.
  **Leídos y casados con `javap`** (§20): la cabecera del `code_item` son dieciséis bytes y las
  instrucciones empiezan en +16, con `ins_size` igual al `args_size` de `javap` en los nueve métodos,
  `registers_size` siempre por encima de sus `locals`, `outs_size` siempre por debajo de su
  `max_stack`, `tries_size` cero (ninguno tiene tabla de excepciones) y el `u32` de +8 cayendo dentro
  del área `debug_info_item` que declara el `map_list`. **Hecha también la traducción de las dieciséis
  operaciones JVM a sus quince formatos dalvik** (§21): los nueve cuerpos emiten la misma secuencia de
  instrucciones, sobre los mismos índices y con el mismo número de palabras que `d8`, y seis de nueve
  salen idénticos hasta los números de registro. Lo que queda para escribir aquí es el
  **`debug_info_item` compartido**: nueve métodos usan siete, porque `d8` deduplica los cuerpos con
  las mismas líneas; emitir uno por método dará un dex más grande sin estar mal.
  **Sin medir a propósito**: `try_item` y `encoded_catch_handler`, porque ninguno de los nueve métodos
  del fixture los tiene; hará falta un fichero con `try` antes de escribirlos. Lo mismo con
  `move-result` de un primitivo, los métodos privados llamados con `invokespecial` y cualquier forma
  `/range` o `/from16`: el traductor los rechaza por nombre en vez de adivinar la palabra.
- **2G.4** Comparación byte a byte con `d8` sobre **los mismos `.class`**. Si difiere, `xxd` y escribir
  a qué sección corresponde; se acepta diferencia documentada **solo si el apk se instala y arranca**.
- **2G.5** La frontera declarada y probada por separado: lambdas, `invokedynamic`, default methods,
  y cualquier cosa que dependencias ajenas aporten. **Fuera del subconjunto**, con el motivo escrito.
  **Hecha y probada** (§21 y §23): el lector de `.class` rechaza `invokedynamic`, `dynamic` y los
  method handles por su nombre (`refuseOutsideSubset`), el caminante JVM se niega en el byte que no
  conoce y el traductor rechaza `move-result` de un primitivo, el `invokespecial` de un método
  privado, las llamadas de más de cinco argumentos y cualquier registro `v16` en un formato de cuatro
  bits. `apkc dex` falla **antes** de escribir, nunca a medias. **Sin prueba en dispositivo**: eso
  llega con la Fase 3, porque un dex suelto no se instala.

- **2G.6** La etapa como comando y en wasm (**hecha**, §23): `apkc dex [-o classes.dex] carpeta…`
  corre en el binario `GOOS=js` dentro del hospedador de navegador y su salida es **byte a byte
  idéntica** a la del binario nativo sobre las mismas clases; `d8` la acepta como entrada (`rc 0`).
  Con esto, el dex no necesita el equipo.

**Mata la fase**: si tras emitir `string_ids`+`type_ids`+`class_defs` el dex resultante no lo lee el
sistema (o `d8` no lo acepta como entrada en una ronda de ida y vuelta), no tiene sentido seguir: un
dex escrito a medias no es un compilador.

**Coste**: semanas, pero acotadas por el recuento medido. Es la primera que se abre ahora.

---

## Fase 1G — Java -> `.class` en Go, para el subconjunto del `studio/`

**Qué prueba**: que el `MainActivity.java` que genera la página se convierta en `.class` en la pestaña,
sin `javac` y sin conexión. Oráculo: `javap -c -p` sobre los `.class` que produce el `javac` nativo del
JDK 21 del teléfono, método por método e instrucción por instrucción.

- **1G.1** Qué Java necesita de verdad el proyecto del `studio/`: el árbol de sintaxis que cubre
  (declaraciones, `@Override`, `new View.OnClickListener() { ... }`, llamadas, cadenas, `R.id.*`).
  Se lista **leyendo la plantilla**, no adivinando, y se escribe la lista antes del código.
- **1G.2** Comprobación de tipos mínima contra el `android.jar` del API 34 — aquí está la parte que no
  es un traductor: hace falta resolver símbolos contra las firmas de 25 MB de SDK. Medir si conviene
  recortar el `android.jar` (y qué se pierde).
- **1G.3** Emisión del bytecode: `method_info`, `Code`, tabla de excepciones, atributos que `d8` mira.
- **1G.4** El `android.jar` entero viaja a la pestaña o se recorta: decisión con su número (fase 4.4).

**Mata la fase**: el momento en que el subconjunto deje de ser un subconjunto honesto: si para
aceptar el fuente hay que implementar la semántica completa de Java (sobrecarga, inferencia,
jerarquías), esto deja de ser una fase y pasa a ser otro proyecto. Eso se escribe, no se disimula.

**Coste**: semanas. Va **después** de 2G y de 3 porque 2G tiene oráculo byte a byte sin dispositivo.

---

## Fase 3 — recursos sin `aapt2` (el trabajo gordo y el que queda para siempre)

**Qué prueba**: escribir en Go las cuatro salidas que apkc pide hoy a `aapt2`, para un proyecto con
`<TextView>` y `<Button>`, y que el apk resultante se instale y muestre la pantalla.

**Objetivo medido antes de escribir nada** (`docs/evidencia.md` §24, e instrumento de medición en
`apkc/recursos_dump_test.go`): sobre el proyecto que genera la página, `aapt2` deja en el apk
**2000 bytes de AXML de manifiesto, 1088 de `resources.arsc` y 784 del layout binario**: 3872 bytes,
que es lo que hay que emitir. El dex ya sale de este fork (§22) y el zip con firma también (§7).
El `.flat` intermedio (1076 + 536) **no viaja en el apk**, así que 3.5 se resuelve en "no hay que
emularlo" y se cierra.

**3-A HECHA** (§24): la estructura está leda trozo a trozo desde los offsets que el propio fichero
declara — encabezado comun de trozo, pool de cadenas en sus dos modos (UTF-16 en el manifiesto,
UTF-8 en la tabla), `XML_RESOURCE_MAP` con los ids del framework (12 en el manifiesto, 6 en el
layout), nodo de 16 con su `attrExt` de 20 y atributos de 20, y en la tabla: `TABLE` + pool global +
`PACKAGE` de cabecera 288 con dos pools de nombres, tres `TYPE_SPEC` y cuatro `TYPE`, con los offsets
de entrada **relativos a `entriesStart`** y un `ResTable_config` de **64 bytes**. Los dos ficheros
terminan clavados al tamano declarado, asi que no queda trozo sin explicar.

Por formato, cada uno con su oraculo y su test:

- **3.1 AXML del layout** — binario de `res/layout/*.xml` (cabeceras, pool de strings, nodos,
  atributos con resource ids). Oráculo: `aapt2 dump xmltree` byte a byte contra nuestra salida sobre
  el layout del `studio/`.
- **3.2 AXML del `AndroidManifest.xml`** — lo anterior más el mapa de recursos del paquete.
- **3.3 `resources.arsc`** — tabla de recursos: paquetes, tipos, entradas, config flags, y el pool de
  strings con sus variantes. Oráculo: `aapt2 dump resources`. Es el que decide si el apk es
  instalable, así que su prueba final es el teléfono.
- **3.4 `R.java`** — apkc ya lo genera hoy a partir de lo que enlaza `aapt2`; hay que alimentarlo del
  mismo mapa de ids que escribamos en 3.3, con los mismos números.
- **3.5 `aapt2 compile` -> `.flat`** — **resuelto sin escribir código** (§24): los `.flat` no viajan
  en el apk; su único consumidor es `aapt2 link`, y si este fork hace el link, sobran.

**Criterio de abandono**: si tras 3.1 y 3.3 el apk no se instala y la diferencia no se localiza en
dos intentos, se para y se documenta. El formato no tiene spec pública: se trabaja contra las
fuentes de `frameworks/base/tools/aapt2` y contra el oráculo, y eso tiene un techo.

**El oráculo sí corre en este teléfono** (medido hoy): `aapt2 dump xmltree --file AndroidManifest.xml
build/app.apk` responde y casa con lo que se leyó de los bytes en §24 (`versionCode(0x0101021b)=1`,
`versionName(0x0101021c)="0.1"`, `minSdkVersion(0x0101020c)=21`). Lo que no es ejecutable aquí es
`dexdump` (sigue valiendo `d8` como lector del dex, §22). Así que la comprobación de 3.1 a 3.3 puede
ser **byte a byte contra la salida de `aapt2 dump`**, sin esperar a un dispositivo.

**Coste**: semanas. No se abre antes de que la Fase 2 viva.

---

## Fase 4 — la costura y la página

Solo si 1+2+3 vivieron.

- **4.1 Interfaz de etapas en apkc**: `os/exec` se usa en seis ficheros (`aab.go`, `apk.go`,
  `build.go`, `device.go`, `doctor.go`, `proc_*.go`). Cada etapa (recursos, compile, dex, empaquetado)
  pasa a una interfaz con dos implementaciones: **proceso** (la de hoy, nativa) y **módulo** (wasm).
  Sin esto no hay navegador posible: `js/wasm` no tiene `fork/exec`.
- **4.2 Hospedador real de navegador**: `shim/navegador-sim.js` ya demuestra la API que exige Go
  (24 funciones de `fs`, `fs.constants`, `path.resolve`, `process.*`, `fs.writeSync` para el runtime,
  los doce campos del `stat`). Falta respaldarlo en OPFS en vez de memoria, y probarlo en Chromium
  (no en node).
- **4.3 La página**: el `studio/` actual genera el proyecto y traduce salidas. Con 4.1 y 4.2 detrás,
  el mismo formulario añadiría "compilar aquí" y devolvería el apk.
- **4.4 Empaquetado y tamaño**: qué se baja a la pestaña y cuánto (android.jar, el runtime, el wasm de
  d8). Publicar el número en `docs/evidencia.md`; si el peso hace el flujo absurdo, eso es un
  resultado, no un detalle.

---

## Riesgos transversales

- **Memoria**: los demos reales pesan 142 MB (assets) y 60 jars de dependencias. El plan solo
  promete el proyecto mínimo del `studio/`; decir más sin medirlo sería mentir.
- **Licencias**: la ruta de un runtime de terceros está cerrada por decisión, así que no hay nada
  que licenciar. Lo que queda: `d8` y `aapt2` son Apache-2.0 del árbol de Android y un derivado en Go
  mantiene la atribución; `android.jar` es de la SDK, cuya licencia no permite redistribuirlo como si
  fuera propio — hay que medir qué se lleva la pestaña y cómo.
- **Mantenimiento eterno**: la Fase 3 es format reverse-engineered. Cada versión nueva del formato
  puede romper lo que escribamos, y no hay upstream que nos avise.
- **Falsa verdes**: la lección del `studio/` (seis fallos que su test de node veía correctos) se
  aplica igual aquí: ninguna fase se cierra sin oráculo externo y, cuando cabe, sin dispositivo.

## Decisiones registradas

- **Sin conexión y sin runtimes de terceros** (decisión del usuario, 2026-10-27, que cierra las
  fases 1 y 2 tal como estaban escritas: prestar una JVM de terceros para correr `javac` y `d8`
  dentro). La compilación tiene que ocurrir entera en el navegador. Lo que eso implica escrito: `javac` y `d8` no se prestan, **se emiten en Go**; y la
  sentencia anterior de este plan ("escribir un dexer en Go es otro proyecto de otra escala") queda
  matizada por una medición — el objetivo del `studio/` son 217 objetos (§13) — pero no refutada para
  el caso general, que sigue sin estar en el plan.
- La página de fricción **vive dentro de este repo** (`studio/`), no en `~/Develop/Web`: todo lo
  relacionado con web va aquí, por indicación del usuario de 2026-10-26.
- El orden 1 -> 2 -> 3 es por riesgo, no por dependencia técnica: la Fase 2 puede matar la ambición en
  dos días, y la Fase 3 cuesta semanas.
- `apkc` no se modifican sus mensajes ni sus comandos desde aquí: se pide, se aprueba, y se commitea
  en `../apkc` con su propio gate.
