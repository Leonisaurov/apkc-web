# Plan — compilar una app Android en el navegador con apkc

## Objetivo y definición de éxito

**Éxito**: abrir una página en el navegador, escribir o pegar el fuente de una app mínima
(`Activity` + layout), y obtener un `app.apk` **instalable y verificado**, sin server, sin SDK y sin
JDK instalados en la máquina del usuario. Se demuestra con tres hechos, todos comprobables con
oráculos ajenos a apkc:

1. `apksigner verify --verbose` (en otra máquina) dice v1 y v2 correctos.
2. El apk se instala en un teléfono real y la actividad arranca.
3. El `classes.dex` producido en el navegador es **idéntico byte a byte** al que produce `d8`
   nativo sobre los mismos `.class` (o diferido con motivo escrito).

**No es éxito**: empaquetar y firmar algo que ya se compiló en otro sitio. Eso es lo único que hoy
funciona del lado cliente y está documentado, pero no es el objetivo.

## Reglas de orden (por qué este orden y no otro)

- **Nada de la Fase 3 (recursos en Go) se empieza antes de que la Fase 2 diga que `d8` corre en
  wasm.** Un compilador de recursos sin dexer no produce nada instalable: son semanas de trabajo
  que dependen de un resultado que cabe en un experimento de días.
- **Nada de este repo entra en `../apkc`** salvo la Fase 0, que es trabajo de la herramienta en sí.
- **Ninguna fase se declara "imposible" sin el texto del error y el comando que lo produjo.** El
  muro de esta ambición se encontró así: `aapt2 compile never started: pipe: not implemented on js`.

## Estado

| Fase | Estado | Evidencia |
| --- | --- | --- |
| 0.1 mensajes de herramienta | **hecha** (apkc `e7bfd01`) | `[build] aapt2 compile never started: pipe: not implemented on js` |
| 0.2.1 `apkc sign` | **hecha** (apkc `a909214`) | `docs/evidencia.md` §7 y §8: `apksigner`, `zipalign -c -p`, `jarsigner` y `zipcheck.py` sobre el apk re-firmado, y el mismo apk firmado **dentro de wasm** en 1,20 s |
| 0.2.2 `apkc verify` | pendiente | — |
| 1 `javac` en wasm | pendiente (riesgo bajo, es integración) | — |
| 2 `d8` sobre JVM wasm | **pendiente y decisiva** | — |
| 3 recursos sin `aapt2` | bloqueada hasta que 2 viva | — |
| 4.1 etapas como interfaz | pendiente | — |
| 4.2 hospedador de navegador | **a medias**: el de node ya monta y exporta ficheros reales (§8); falta OPFS y Chromium | — |
| 4.3 la página | bloqueada hasta 1+2+3 para *compilar*; lo que sí puede hacer hoy es crear el proyecto y firmar | — |
| 4.4 empaque y tamaño | medida parcial: `apkc.wasm` 9,4 MB | — |

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

## Fase 1 — `javac` dentro del navegador

**Qué prueba**: que un `MainActivity.java` de un proyecto apkc se convierta en `.class` en la
pestaña, sin nada instalado.

- **1.1** Bajar la demo de GraalVM (`javac` con native-image a wasm) y CheerpJ 3.x. Registrar
  tamaño descargado, tiempo del primer compile y del segundo.
- **1.2** Leer la licencia **antes** de cualquier integración. Si el uso previsto no está permitido,
  la fase se cierra aquí con el motivo escrito.
- **1.3** Compilar el proyecto del `studio/` con el `javac` wasm y comparar con el nativo:
  `javap -c -p -classpath build/classes ...` sobre los mismos `.class`, operación por operación.
  Diferencia tolerada: ninguna que afecte al bytecode; se documenta cada una.
- **1.4** Pasar el `android.jar` del API 34 como classpath al runtime wasm (25 MB: medir qué tal le
  sienta a la pestaña, y si hace falta recortarlo).

**Mata la fase**: licencia, o un tiempo que haga el flujo inusable (umbral honesto: > 60 s para un
proyecto de un fichero en el teléfono).

**Coste**: días.

---

## Fase 2 — `d8` sobre la JVM wasm (la que decide todo)

**Qué prueba**: que el `d8` real de `build-tools/34.0.0` corra sobre el runtime de la Fase 1 y
produzca un `classes.dex` **idéntico byte a byte** al del `d8` nativo.

**Protocolo del experimento** (escrito para que otro lo repita sin preguntarme nada):

1. Con los `.class` de la Fase 1.3 —los mismos, no una recompilación—, correr
   `d8 --min-api 21 --output <dir> <los .class>` nativo. Guardar `classes.dex` y su sha256.
2. Correr el mismo jar bajo el runtime wasm, con el mismo `--min-api` y las mismas entradas.
3. Comparar sha256. Si difiere, `xxd` las primeras 16 bytes de diferencia y escribir a qué sección
   del dex corresponde (`map_list`, `string_ids`, orden de `class_defs`...).
4. Medir: tiempo, memoria máxima de la pestaña, y si `d8` necesita algo que el runtime no
   implementa (`java.nio.file`, concurrencia, `invokedynamic` en su propio código).
5. Repetir con un jar de dependencia real (un `androidx` pequeño) para ver cómo se comporta el
   desugaring sobre entradas ajenas al proyecto.

**Resultados posibles y qué hace cada uno**: idéntico -> Fase 3 tiene sentido y se abre. Diferente
pero instalable (el apk resultante se instala y arranca) -> se acepta con la diferencia documentada y
la prueba de dispositivo manda. No corre -> **la ambición muere aquí** y este repo pasa a archivo con
el intento escrito; escribir un dexer en Go es otro proyecto de otra escala y hay que decirlo así.

**Coste**: días, y es lo primero que se hace después de la Fase 1.

---

## Fase 3 — recursos sin `aapt2` (el trabajo gordo y el que queda para siempre)

**Qué prueba**: escribir en Go las cuatro salidas que apkc pide hoy a `aapt2`, para un proyecto con
`<TextView>` y `<Button>`, y que el apk resultante se instale y muestre la pantalla.

Por formato, cada uno con su oráculo y su test:

- **3.1 AXML del layout** — binario de `res/layout/*.xml` (cabeceras, pool de strings, nodos,
  atributos con resource ids). Oráculo: `aapt2 dump xmltree` byte a byte contra nuestra salida sobre
  el layout del `studio/`.
- **3.2 AXML del `AndroidManifest.xml`** — lo anterior más el mapa de recursos del paquete.
- **3.3 `resources.arsc`** — tabla de recursos: paquetes, tipos, entradas, config flags, y el pool de
  strings con sus variantes. Oráculo: `aapt2 dump resources`. Es el que decide si el apk es
  instalable, así que su prueba final es el teléfono.
- **3.4 `R.java`** — apkc ya lo genera hoy a partir de lo que enlaza `aapt2`; hay que alimentarlo del
  mismo mapa de ids que escribamos en 3.3, con los mismos números.
- **3.5 `aapt2 compile` -> `.flat`** — posiblemente se pueda **saltar**: es un formato intermedio cuyo
  único consumidor es `aapt2 link`. Si hacemos el link, los `.flat` sobran, y con ello se quita una
  dependencia de formato. Confirmar antes de escribir nada.

**Criterio de abandono**: si tras 3.1 y 3.3 el apk no se instala y la diferencia no se localiza en
dos intentos, se para y se documenta. El formato no tiene spec pública: se trabaja contra las
fuentes de `frameworks/base/tools/aapt2` y contra el oráculo, y eso tiene un techo.

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
- **Licencias**: CheerpX/CheerpJ es comercial. `d8` y `aapt2` son Apache-2.0 del árbol de Android,
  pero un derivado en Go mantiene la atribución.
- **Mantenimiento eterno**: la Fase 3 es format reverse-engineered. Cada versión nueva del formato
  puede romper lo que escribamos, y no hay upstream que nos avise.
- **Falsa verdes**: la lección del `studio/` (seis fallos que su test de node veía correctos) se
  aplica igual aquí: ninguna fase se cierra sin oráculo externo y, cuando cabe, sin dispositivo.

## Decisiones registradas

- La página de fricción **vive dentro de este repo** (`studio/`), no en `~/Develop/Web`: todo lo
  relacionado con web va aquí, por indicación del usuario de 2026-10-26.
- El orden 1 -> 2 -> 3 es por riesgo, no por dependencia técnica: la Fase 2 puede matar la ambición en
  dos días, y la Fase 3 cuesta semanas.
- `apkc` no se modifican sus mensajes ni sus comandos desde aquí: se pide, se aprueba, y se commitea
  en `../apkc` con su propio gate.
