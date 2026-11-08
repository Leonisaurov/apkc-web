# Plan — compilar una app Android en el navegador con apkc

## Objetivo y definición de éxito

**Éxito**: abrir una página en el navegador, escribir o pegar el fuente de una app mínima
(`Activity` + layout), y obtener un `app.apk` **instalable y verificado**, sin server, sin SDK, sin
JDK instalados en la máquina del usuario **y sin conexión**: todo lo que compila y empaqueta viaja en
el propio origen de la página. Comprobable: ninguna petición fuera del origen (`evidencia.md` §15),
ningún runtime de terceros. Se demuestra con tres hechos, todos comprobables con
oráculos ajenos a apkc:

1. `apksigner verify --verbose` (en otra máquina) dice v1 y v2 correctos.
2. El apk se instala en un teléfono real y la actividad arranca. *(HECHO el 2026-11-01 con `rish`, el shell de
   Shizuku, en Android 15, sobre el apk que escribe chromium: instala, arranca, y dos y cuatro pulsos sobre el
   botón dejan `content-desc="pulsos 2"` y `"muchos 4"` sin un solo `FATAL` —`evidencia.md` §55—.)*
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
| 0.2.2 `apkc verify` | **hecha** (rama `web`): lee el paquete desde el `fs` y comprueba las dos firmas con lectores propios, sin `apksigner` ni `jarsigner` | `docs/evidencia.md` §47: ocho casos cotejados con `apksigner`, mismo veredicto y mismo código de salida, incluidos dos casos que apkc no escribió |
| 1 y 2 (runtime JVM prestado) | **cerradas por decisión del usuario el 2026-10-27**: sin conexión y sin terceros. `d8` sobre una JVM wasm **nunca se probó**, y ya no se probará así | `docs/evidencia.md` §15: la pestaña no hace ni una petición fuera de su origen |
| 2G | **hecha** (rama `web`): lector y escritor de dex en Go, **con `debug_info_item` desde el §56**. El objetivo medido eran 2844 bytes de `d8`; `apkc dex` da **el mismo fichero byte a byte** sobre las `.class` de javac que sobre las que escribe este fork, y lo que separa nuestro fichero del de `d8` ya no es el debug sino el bloque de anotaciones y su marca de origen. `d8` lee el nuestro y lo re-dexa con `rc 0`, y ART del aparato lo verifica y lee sus líneas | `docs/evidencia.md` §13, §20 a §23, §41, §42, §56 |
| 3 recursos sin `aapt2` | **hecha**: `arscwriter.go`, `axmlwriter.go`, `r.go`, `marco.go`, `enlazar.go` + `arbol.go` + `valores.go` enlazan desde los ficheros de texto; `build --linker go` produce un apk que `apksigner`, `zipalign`, `zipcheck.py` y `aapt2 dump` aprueban | `docs/evidencia.md` §28 a §38 |
| 1G Java -> `.class` en Go | **hecha para el subconjunto del `studio/` y algo más**: `javafuente.go`, `javaemisor.go`, `compilar.go`; oráculo doble (`javap -c -p` y el dex idéntico) y rechazo por nombre de lo no medido. El subconjunto llega a campos con modificadores, variables locales, `checkcast`, aritmética entera y el `if`/`else`, la concatenación con `StringBuilder` y la anónima que toca un campo del exterior | `docs/evidencia.md` §40 a §43, §49 a §52 y §53-§54 |
| 4.1 etapas como interfaz | **logrado por otro camino**: no hay interfaz con dos implementaciones, hay banderas (`--linker go`, `--compiler go`) que eligen el módulo de Go en vez del proceso, y `sinProcesos()` se salta el sondeo del SDK. El `os/exec` de siempre sigue ahí para el camino nativo, que es el que funciona con SDK completo | `docs/evidencia.md` §38, §44, §45 |
| 4.2 hospedador de navegador | **hecha**: OPFS + worker + chromium, conducido y verificado por fuera (`docs/evidencia.md` §11, 16 comprobaciones) | `studio/prueba-pestana.js` |
| 4.3 la página | **hecha**: el editor crea, **compila** (`build --linker go --compiler go`, sin procesos), dexa, firma y descarga; sin conexión y sin CDN | `docs/evidencia.md` §44 y §45 |
| 4.4 empaque y tamaño | medida: `apkc.wasm` 9,46 MB y el `android.jar` entra por el propio origen (**26361808 bytes**, una vez, guardado en OPFS) | `docs/evidencia.md` §45 |
| 5 cerrar el subconjunto (Java y recursos) | **abierta el 2026-11-01**: la página compila apps sencillas; faltan `throws`, interfaces, el `MainActivity.class` y las llamadas de seis argumentos, y en recursos los iconos, los calificadores y los tipos que faltan en `values`. **Hechos 5.24 a 5.27**, el `invokestatic` sin argumentos, **5.10 (`while`, `for`, `do-while`, `break`/`continue`, §63, §69 y §70)**, **5.12 (`switch`, §64)**, **5.23 (`StackMapTable`, §65)**, **5.11 (arreglos, `main(String[])`, `new T[] {…}` y los arreglos anchos, §66, §73, §84 y §85)**, **5.1 (`++`/`--` y `+=`/`-=`, §67 y §68)**, **5.9 (`try`/`catch`, `throw`, los dos `catch` y el `finally`, §71 y §75)**, **5.5 (`private`/`protected` y `access$…`, §72)**, **5.8 entero menos dos piezas (§74, §77, §80, §81, §82, §83, §84 y §85)**, **5.4 (`/from16` y `invoke-*/range`, §86 y §90)**, **5.2 (`char`, sus escapes, `caload`/`castore`, `i2c`, `aget-char`/`aput-char` y `sget-char`/`sput-char`, §87)**, **5.3 (`X.class`, `const-class` 0x1c y `==` entre referencias con `if_acmp*`, §89)**, **5.7 (constructores propios, `super`/`this` y los dos `code_item` de una sobrecarga, §91)** y **5.6 (`throws`, interfaces propias, abstractos e `invokeinterface`, §92)**, **5.16 (PNG en `drawable`/`mipmap` y `roundIcon`, §93)** y **5.17 (`values-xx`, `layout-land` y `drawable-hdpi`, §94 y §103)**; están cerrados el control de flujo, los números, el `char`, los literales de clase, las llamadas de seis palabras, los constructores, `throws`/interfaces, el icono y los dos idiomas, 5.18 en su parte de arrays/plurals (§96), 5.19 en su parte de `menu`/`xml`/`raw` (§97), 5.20 para API 28-35 (§98), 5.14 completa (§99 y §100), 5.15 (`Signature`, §101), 5.19 completa (§97 y §102) y 5.17 completa (§94 y §103); lo que sigue son `attr`/`styleable`, las regiones y las APIs sin nombre numérico. **5.21 queda en parte** (v3 se tolera y se anota, no se escribe) y **5.22 se declara fuera de alcance** (rechazado por nombre; desugración XL no medida), §104. La **app de cierre** (bucle, `switch`, `try`/`catch`, icono y dos idiomas) ya compila y firma con el camino todo-Go (§95, 9861 bytes, `apksigner` y `zipalign` correctos); **falta instalarla y arrancarla** en el teléfono, que es acción sobre el aparato y se pide al usuario | la sección «Fase 5» de este plan |
| el apk en un teléfono real (hecho 2 del objetivo) | **HECHO, y sobre el apk que escribe chromium** —sin `adb`, con `rish` (el shell de Shizuku) en Android 15—: `pm install -r` `Success`; `am start -W -S` `Status: ok`, `LaunchState: COLD`, `TotalTime: 316`; dos `input tap` sobre `com.myapp:id/pulsar` dejan `content-desc="pulsos 2"` y cuatro `"muchos 4"`; `Toast` en `SurfaceFlinger` y `FATAL EXCEPTION` 0 veces; `pm compile -f` `Success` con `[status=verify]`. El camino hasta ahí destapó cuatro defectos de layout que `d8` no veía (§55) | — |

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
  `sha1` y su `adler32` (§22). Con el bloque de anotaciones emitido (§59) el fichero entero eran 2696 bytes;
  con los `type_list` internados por contenido (§60) son **2680 contra los 2844 de `d8`**, con las mismas
  17 entradas de `map_list` y las mismas cuentas de área.
- **2G.4** Comparación byte a byte con `d8` sobre **los mismos `.class`**. **Medida sección por
  sección** (§22): `type_ids`, `field_ids` y `method_ids` salen byte a byte iguales; `proto_ids`
  difiere solo en los offsets absolutos; cada `string_data_item` es idéntico. **Desde el §59 también el
  bloque de anotaciones: los ocho `annotation_item` son los mismos bytes y el volcado `dexdump -a` es
  idéntico línea por línea**, y **desde el §60 también el área de `type_list`: byte a byte igual, ocho
  listas con los mismos offsets**. Lo que falta para que el fichero entero sea igual son tres cosas, todas
  medidas y con la decisión escrita: (1) **la cadena con la que `d8` se firma a sí mismo**
  (`~~D8{…,"sha-1":"…","version":"9.2.4-dev"}`, §60) —se **descarta a propósito**: nombra otra herramienta
  y trae un digest del fichero que esa escribió; `TestLaMarcaDeOrigenDeD8NoSeCopia` vigilando está—;
  (2) **el orden de los ítems dentro del área de anotaciones** (mismos bytes, otra sucesión);
  (3) **la unidad dalvik de algunas posiciones de línea**, que es el reflejo de la diferencia de
  numeración de registros ya aceptada abajo. Las demás cuentas ya cuadran: **siete `debug_info_item` como
  los suyos** (§60 adoptó la política de posiciones del `--release`, que es el modo con el que `apkc`
  invoca a `d8`, y comprobó en el aparato que la traza de una excepción sigue diciendo la línea de la
  sentencia), 8 `type_list`, 52 cadenas, 23 tipos, 9 protos, 4 campos, 17 métodos, 6 `class_defs`.
  El fichero entero era 2384 bytes hasta el §56 y 2436 hasta el §58 (2392 con el relleno que
  quitó el §55). En los cuerpos la
  secuencia de instrucciones, los índices y el número de palabras coinciden en los nueve métodos;
  seis de nueve salen idénticos hasta los números de registro (§21). **El oráculo que cierra el
  paso**: `d8` lee el dex que escribimos y lo re-dexa sin un solo error (`rc 0`), y sobreviven las
  seis clases, los nueve cuerpos y las tres constantes de `R` leídas de nuestro `static_values`. La prueba de
  dispositivo llegó con la Fase 3 y está escrita en el §55: ART, en el propio aparato, lee nuestro dex con
  `dexdump rc 0` (después de corregir los cuatro defectos de layout que ese párrafo destapó) y el apk del
  navegador instala, arranca y ejecuta el `onClick`. Con las anotaciones puestas, la misma prueba vuelve a
  pasar: `dexdump` verificando, `pm install` en `Success`, actividad arrancada en frío y `dexdump -a`
  idéntico al de `d8` sobre el mismo proyecto (§59); y con la política de líneas del `--release`, otra vez
  y en los dos caminos de la herramienta (`§60`).
- **2G.3** `class_data_item`, `code_item` y `debug_info_item`: la codificación de instrucciones en sí.
  **Leídos y casados con `javap`** (§20): la cabecera del `code_item` son dieciséis bytes y las
  instrucciones empiezan en +16, con `ins_size` igual al `args_size` de `javap` en los nueve métodos,
  `registers_size` siempre por encima de sus `locals`, `outs_size` siempre por debajo de su
  `max_stack`, `tries_size` cero (ninguno tiene tabla de excepciones) y el `u32` de +8 cayendo dentro
  del área `debug_info_item` que declara el `map_list`. **Hecha también la traducción de las dieciséis
  operaciones JVM a sus quince formatos dalvik** (§21): los nueve cuerpos emiten la misma secuencia de
  instrucciones, sobre los mismos índices y con el mismo número de palabras que `d8`, y seis de nueve
  salen idénticos hasta los números de registro. **El `debug_info_item` está hecho y medido** (§56):
  formato casado contra las palabras de `d8` con una sonda propia y con `dexdump -d` del aparato, emisor
  en `apkc/dexdebug.go`, área `0x2003` en el mapa, items idénticos compartiendo offset (los nueve cuerpos
  del fixture dan siete items, los mismos que `d8`: ocho hasta el §59, que contaba la posición de la llave
  de cierre que el `--release` no escribe, corregido en el §60) y trazas con número de línea en el `logcat`
  del teléfono.
  **Hecho también el defecto de numeración encontrado de paso (§57)**: el traductor numeraba los locales
  como en el `.class`, así que un método con **más locales que argumentos** producía un dex que ART
  rechazaba con `instance field access on object that has non-reference type Undefined`. Corregido con una
  permutación en `local(n)` que es identidad cuando `locales == ins`.
  Con las tres estructuras del título —`class_data_item`, `code_item` y `debug_info_item`— **el punto está
  cerrado**, y **también está cerrado el bloque de anotaciones**, que se emite desde el §59 (`apkc/dexanota.go`)
  con sus tres áreas declaradas en el mapa y su volcado `dexdump -a` igual al de `d8`.
  **Sin medir a propósito**: `try_item` y `encoded_catch_handler`, porque ninguno de los nueve métodos
  del fixture los tiene; hará falta un fichero con `try` antes de escribirlos. Lo mismo con
  `move-result` de un primitivo, los métodos privados llamados con `invokespecial` y cualquier forma
  `/range` o `/from16`: el traductor los rechaza por nombre en vez de adivinar la palabra.
- **2G.4** Comparación byte a byte con `d8` sobre **los mismos `.class`**. Si difiere, `xxd` y escribir
  a qué sección corresponde; se acepta diferencia documentada **solo si el apk se instala y arranca**.
  **Diferencia escrita y condición cumplida**: tras el §60 la lista se queda en tres ítems, todos medidos y
  con su decisión escrita —la cadena `~~D8{…}` con la que `d8` se firma a sí mismo, **descartada a propósito
  porque nombra otra herramienta y trae un digest del fichero que esa escribió** (§60, con prueba
  vigilante), el orden de los ítems dentro del área de anotaciones y los offsets absolutos (`parameters_off`
  de `proto_ids` y alguna unidad de posición de línea), que son reflejo de la numeración de registros ya
  aceptada—. Ya **no son diferencia**: el bloque de anotaciones (sus ocho `annotation_item` son los mismos
  bytes, §59), el área de `type_list` (byte a byte igual, ocho listas, §60) y la cuenta de
  `debug_info_item` (siete como los suyos, porque adoptamos la política de posiciones del `--release` con la
  que `apkc` invoca a `d8`, §60). El apk con lo que queda se instala y arranca desde el §55, con las líneas
  de depuración leyendo en el `logcat` desde el §56 y **la traza de una excepción diciendo la misma línea en
  los dos caminos de la herramienta desde el §60**.
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

- **1G.1 HECHA** — la lista se leyó de la plantilla y de la muestra del `studio/`, no se adivinó: paquete,
  imports con estrella, `class` con `extends`, `@Override` (que no deja huella en el `.class`, medido en el
  §40), métodos con parámetros, `super.<metodo>()`, llamadas sin receptor, llamadas sobre un tipo
  (`Toast.makeText`) y sobre un valor (`menu.add`), `new Tipo(args)` y `new Tipo(args) { ... }`, literales de
  cadena y entero (hexo incluido), `this`, `Tipo.this`, acceso encadenado (`R.layout.main`), `return`. El
  léxico y el analizador están en `apkc/javafuente.go` con cinco pruebas; lo que no está medido se rechaza
  con nombre del fuente y su línea.
- **1G.2 HECHA para el subconjunto** — la resolución de tipos no recorta el `android.jar`: se lee el índice
  central una vez (`abrirBuscador`) y cada entrada `.class` que hace falta se descomprime y se pasa por el
  lector de la Fase 2, con caché (`buscadorDeClases`). Sobre la plantilla del §40 el emisor abre **cuatro**
  entradas del jar (`Activity`, `Bundle`, y las dos `R` propias vienen del otro fuente). El tamaño del jar
  sigue siendo el problema de 1G.4, no de ésta.
- **1G.3 HECHA para la plantilla** — `apkc/javaemisor.go` escribe el pool, los `field_info` con
  `ConstantValue`, los `method_info` con `Code` y `LineNumberTable`, el `SourceFile` y el `InnerClasses`, y
  sintetiza el `<init>`. Oráculo doble y medido en el §41: `javap -c -p` sobre lo que escribimos imprime las
  mismas líneas que sobre lo que escribe javac, y **el dex de nuestras clases es byte a byte idéntico al dex
  de las clases de javac** (mismo lector, mismo dexer, 1284 = 1284, cero diferencias).
- **1G.3b HECHA — la clase anónima**, con su medición previa (parrafo 42): campo `this$0` con
  `ACC_FINAL|ACC_SYNTHETIC`, constructor `(LExterior;)V` con `MethodParameters` (`final mandated`, 0x8010),
  atributo `EnclosingMethod`, la fila del `InnerClasses` sin exterior ni nombre, y las menciones que javac
  apunta desde los descriptores. Oráculo: `javap -c -p` igual en las seis clases (solo cambian los números de
  pool) y `apkc dex` dando **2392 bytes idénticos** desde nuestras clases y desde las de javac.
- **1G.3c HECHA — el comando.** `apkc compile --android-jar android.jar [-o build/classes] [src...]`
  (parrafo 43), en la lista de comandos que no sondean el SDK. Medido en el binario nativo y dentro del
  hospedador con forma de navegador: 1,16 s con el jar de 26 MB montado, y las seis clases exportadas son
  idénticas byte a byte a las nativas.
- **1G.3d HECHA — enhebrado en `apkc build`.** `--compiler go` sustituye las dos etapas de código
  (`compileJava` y `bundleJava`) y salta `closeApiGap`, que es un remiendo de javac: aqui todos los fuentes
  se leen juntos. `build` acepta `--android-jar`, `--sdk-version` y `--codename` por la razon del §39, y
  `main()` no sondea el SDK cuando la linea pide `--linker go --compiler go`. Medido nativo (255 ms, apk de
  9868 bytes aprobado por `apksigner`, `zipalign`, `aapt2 dump badging`/`resources` y `zipcheck.py`) y dentro
  del hospedador: el mismo apk, **entrada por entrada con los mismos bytes**. Parrafo 44, con la nota de que
  el apk no es reproducible byte a byte por los tiempos del zip.
- **1G.4 HECHA — el `android.jar` entra en la pestaña por su propio origen.** La página lo pide una vez
  (`/android-jar` desde `servidor.js`, o el selector de fichero `#android-jar`) y lo guarda en OPFS como
  `/sdk/android.jar`; `compilarDelTodo()` se lo pasa a `build --compiler go`. Medido dentro de chromium:
  26361808 bytes escritos, releídos iguales, y el apk que sale es **de los mismos 9141 bytes y con los
  mismos bytes en cada entrada** que el build nativo del mismo proyecto. Párrafo 45, con los dos fallos del
  hospedador que aparecieron al cablear (carrera de arranque en `hospedador.js` y directorios intermedios
  que `fs-fragmento.js` no creaba en memoria).

- **1G.5 HECHA — el subconjunto admite un fuente que la página no genera** (§49 y §50): campos con los
  modificadores leídos de `javap -v`, sus inicializadores en el `<init>` (y los estáticos en un `<clinit>`
  que va al final), variables locales con `checkcast`, asignación a campo y aritmética entera. Medido dos
  veces: `javap -p -c -l` idéntico línea por línea contra javac 21, y `apkc dex` dando el mismo fichero byte
  a byte sobre nuestras clases que sobre las de javac (1960 bytes). End-to-end en Go: 834 ms y un apk de 9555
  bytes que `apksigner`, `zipalign`, `zipcheck`, `aapt2` y `apkc verify` aprueban, y cuyo dex relee `d8` con
  `rc 0`.
- **1G.6 HECHA — el `if` y su `else`** (§52): los anchos de las branquias del JVMS, las tres formas
  dalvik de rama (`10t`, `21t`, `22t`) más el `22s` que el anteojo leía como `22c`, la inversión de la
  condición que hace javac y el `goto` que solo aparece cuando el cuerpo no sale por su cuenta — y el
  encadenado por encima de una etiqueta vacía, que es lo que destapó el `if` anidado. Medido dos veces:
  `javap -p -c -l` idéntico línea por línea contra javac 21 (186 y 81 líneas en las dos sondas, 78 en el
  `MainActivity` de la página), y `apkc dex` dando el mismo fichero byte a byte sobre nuestras clases que
  sobre las de javac (1796 y 1684 bytes). Contra `d8`: quince de los veintiún cuerpos iguales instrucción
  por instrucción y con el mismo desplazamiento; los seis restantes, por optimizaciones suyas (plegado del
  `<clinit>`, fusión de colas idénticas, propagación de copias).
- **1G.7 HECHA — la concatenación de cadenas y el plegado de constantes** (§53): en la versión 52 javac
  pega cadenas con una cadena de `StringBuilder.append`, los literales adjacentes se doblan en una sola
  constante (`"hola" + " " + 3` es un `ldc "hola 3"`, y si no queda otra cosa no hay `StringBuilder`), y las
  cuentas de literales se doblan — salvo `1 / 0`, que javac deja `idiv` para que la excepción salte en
  ejecución. Añadido el paréntesis como agrupación en el analizador (con rechazo del molde ambiguo) y
  `areturn` (0xb0) a la tabla del caminante, que faltaba. Cuatro sondas con `javap` idéntico (134, 35, 206
  y 54 líneas), `apkc dex` igual byte a byte sobre las dos procedencias (3240 bytes) y `d8` releyendo el
  nuestro con `rc 0`; veinticinco de treinta y nueve cuerpos salen iguales a `d8` incluso con los ids, y los
  doce restantes difieren por reescritura de la cadena de `StringBuilder`, plegado de `<clinit>`, `/lit8`
  contra operando constante, `if-nez` en vez de comparar contra cero, y tres fusiones de cola.
- **1G.8 PENDIENTE, con su motivo**: arreglos y bucles (la rama hacia atrás rechaza hoy el bucle porque el
  punto de fusión de registros no está medido), `switch`, `try`/`catch`, las cadenas que se concatenan con
  una llamada cuyo tipo no se puede estimar sin emitirla, y **los campos privados del exterior leídos desde
  una anónima**: javac los resuelve con dos métodos sintéticos del exterior (`access$000` para leer,
  `access$002` para escribir, que además devuelve el valor) y ni el número ni el orden están medidos (§54).
  Hasta entonces el portero los rechaza por nombre; no se abre ninguna de estas puertas "de paso".
- **1G.9 HECHA — la app que hace algo al pulsar** (§54): anónima sobre `View$OnClickListener`, escritura del
  campo del exterior por la captura, `if` y concatenación dentro del mismo `onClick`. Cinco clases con
  `javap` idéntico (49 y 69 líneas las dos del contador), `apkc dex` igual byte a byte sobre las dos
  procedencias (2304 bytes), cinco de los ocho cuerpos iguales a `d8` instrucción por instrucción y el resto
  por sus tres optimizaciones conocidas. Compilada también dentro de la pestaña, con su apk verificado y su
  dex releído por `d8` (bloque 4e del gate).

**Cerrada la Fase 1G y el cableado**: `create`, editar, compilar recursos, compilar Java, dexar, empaquetar,
firmar y verificar ocurren dentro de la pestaña, sin conexión y sin un solo proceso. El emisor y el enlazador
siguen rechazando por nombre lo que no está medido (§41, §42, §38, §49): esa es la frontera honesta, no un
fallo del camino.

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

**3-A HECHA** (§24 y §25): la estructura está medida trozo a trozo desde los offsets que el propio fichero
declara — encabezado comun de trozo, pool de cadenas en sus dos modos (UTF-16 en el manifiesto,
UTF-8 en la tabla), `XML_RESOURCE_MAP` con los ids del framework (12 en el manifiesto, 6 en el
layout), nodo de 16 con su `attrExt` de 20 y atributos de 20, y en la tabla: `TABLE` + pool global +
`PACKAGE` de cabecera 288 con dos pools de nombres, tres `TYPE_SPEC` y cuatro `TYPE`, con los offsets
de entrada **relativos a `entriesStart`** y un `ResTable_config` de **64 bytes**. Los dos ficheros
terminan clavados al tamano declarado, asi que no queda trozo sin explicar.

**3-B HECHA** (§26): `apkc/recursos.go` lee las dos estructuras. Los ficheros de `aapt2` están
fijados en base64 en `apkc/recursosfixture_test.go` (2000 y 1088 bytes, con la longitud asertada) y
el lector reproduce las cuentas del §24: 31 cadenas UTF-16 y los 12 ids del manifiesto, sus
**16** trozos de nodo y los siete atributos de `<manifest>` **por su nombre**, en el orden que
imprime `aapt2 dump xmltree`. Salió además la forma del valor complejo de un estilo, **corregida en
el parrafo 27**: cabecera de 16 bytes con el id del padre (`0x0103012b`) y el **número de registros**
dentro, y detrás exactamente esos registros de doce bytes (nombre `u32` + `Res_value` de ocho).
Medido con variantes sobre el proyecto de la página: 2 ítems dan recuento 2, 9 dan 9, y el proyecto
original 4 y 5 en sus dos configuraciones. La lectura del parrafo 26 ("cuatro en ambas") estaba
torcida una palabra y el lector exige ahora `registros == recuento` para que no vuelva a pasar. La
unica discrepancia que quedó (mismos bytes, dos lecturas del volcado) tenía causa en el instrumento —
recorría hasta el final del bloque en vez de hasta la entrada siguiente— y está cerrada en el
párrafo 27. Siete rechazos probados cambiando un byte del fixture. Y algo que hace falta saber para comparar: el
`resources.arsc` (1088) y el `AndroidManifest.xml` (2000) que construye el SDK de este teléfono son
**byte a byte iguales** a los del fixture (`4b2b7ec0…` y `31f39825…`), así que el oráculo es fijo.

Por formato, cada uno con su oraculo y su test:

- **3.1 AXML del layout** — binario de `res/layout/*.xml` (cabeceras, pool de strings, nodos,
  atributos con resource ids). Oráculo: `aapt2 dump xmltree` byte a byte contra nuestra salida sobre
  el layout del `studio/`.
  **HECHO** (párrafo 30): `apkc/axmlwriter.go` produce los **784 bytes** del layout del estudio
  **idénticos byte a byte**, leídos de vuelta por el lector de la 3-B y con el volcado de `aapt2`
  idéntico por fuera.
- **3.2 AXML del `AndroidManifest.xml`** — lo anterior más el mapa de recursos del paquete.
  **HECHO** (párrafo 30): los **2000 bytes** del manifiesto también salen idénticos. Queda medido y
  dicho en ese párrafo: el orden del pool (atributos con id por id, resto por ASCII), que el campo
  `ns` de nodo y atributo apunta al **URI**, los `idIndex`/`classIndex`/`styleIndex` como 1-based, que
  el valor crudo solo existe para las cadenas, y que `aapt2` **tira los atributos de los namespaces
  que no conoce** (`tools:` no llega al binario) — lo que se descarta lo decide el que enlaza.
- **3.3 `resources.arsc`** — tabla de recursos: paquetes, tipos, entradas, config flags, y el pool de
  strings con sus variantes. Oráculo: `aapt2 dump resources`. Es el que decide si el apk es
  instalable, así que su prueba final es el teléfono.
  **Decidido con medición** (parrafo 28): el pool de cadenas ya sale byte a byte; la tabla entera no
  puede salir byte a byte con una configuración porque `aapt2` parte los ítems de un estilo por el
  nivel de API del atributo (los dos bloques `id=3` difieren en un byte del `ResTable_config`: `+28`
  vale `0x00` y `0x17`, y el segundo trae el registro del atributo que nació en API 23). Se emite
  **una única configuración base** con todos los ítems; la diferencia queda en el número de
  configuraciones, no en ids, cadenas ni valores, y se comprueba con `aapt2 dump resources`.
  **3-C HECHA para la tabla** (párrafo 29): `apkc/arscwriter.go` escribe la tabla entera —cabecera,
  pool global, `PACKAGE` de 288 con sus dos pools, y por tipo un `TYPE_SPEC` de 16 + 4·entradas con su
  `TYPE` de cabecera 84— y los **primeros 752 bytes del `resources.arsc` son idénticos a los de
  `aapt2`** salvo los dos campos de tamaño que cambian al escribir una configuración en vez de dos.
  Después de ese punto el estilo sale con los cinco registros en la base, y `aapt2 dump resources`
  sobre nuestra tabla imprime el mismo listado que sobre la suya (`0x7f010000`, `0x7f020000`,
  `0x7f030000`). Con las variantes del párrafo 29 quedó medida la regla de las banderas (`id` →
  `0x0004`, estilo/attr → `0x0001`, `<public>` → `0x0002` y `0x40000000` en el spec, el `u16` del spec
  en `+10` cuenta configuraciones) y el orden: tipos alfabéticos con `id` = posición + 1, claves por
  tipo y dentro del tipo por nombre, registros de estilo por `id` de atributo.
- **3.4 `R.java`** — apkc ya lo genera hoy a partir de lo que enlaza `aapt2`; hay que alimentarlo del
  mismo mapa de ids que escribamos en 3.3, con los mismos números.
  **HECHO** (párrafo 31): `apkc/r.go` saca la `R` del mismo `tablaEscribir` que alimenta al escritor de
  la tabla, y el texto sale **idéntico byte a byte** a los 493 bytes que escribe `aapt2 link --java`
  sobre el proyecto del studio. Medido además: el id usa el **índice de la entrada** dentro del tipo
  (no un desplazamiento), los tipos van en orden alfabético y las entradas también, y todo carácter
  fuera de `[A-Za-z0-9_$]` pasa a `_` en el identificador.
- **3.5 `aapt2 compile` -> `.flat`** — **resuelto sin escribir código** (§24): los `.flat` no viajan
  en el apk; su único consumidor es `aapt2 link`, y si este fork hace el link, sobran.
- **3-E ensamblar y firmar** — **HECHO** (párrafo 32): `apkc/empaquetar.go` mete el AXML del
  manifiesto, el del layout, la tabla de recursos y el dex del §22 en un apk con el empaquetador y el
  firmante de apkc, sin `zipalign` ni `apksigner` por en medio. Son 9868 bytes con siete entradas que
  `apksigner verify` acepta por v1 **y** por v2, `zipalign -c -p 4` aprueba, `jarsigner -verify`
  verifica, `zipcheck.py` no encuentra ningún registro que se contradiga, y `aapt2 dump badging`
  dereferencia el manifiesto contra nuestra tabla y devuelve la activity lanzable.
  **Lo que falta para que `apkc build` prescinda de `aapt2`** está dicho en el mismo párrafo: leer el
  `res/values/*.xml`, el layout y el manifiesto **en texto** para producir los modelos, y sacar de
  `android.jar` la tabla de ids del framework (el lector de la 3-B hoy no anda paquetes múltiples).
- **3-F ids del framework desde `android.jar`** — **HECHO** (párrafo 33): `apkc/marco.go` lee el
  `resources.arsc` del SDK y da `nombre → id` del paquete `android`; los 28 ids copiados del volcado
  de `aapt2 dump resources` coinciden. Quedó medido que el jar trae **cuatro paquetes con id 1** (se
  corta en el primero), que el pool global trae 1.307 `styleCount` y no se materializa, que un tipo es
  **muchos bloques** y solo cuenta el que iguala la cuenta de su spec, y que la entrada ausente se
  marca con offset `0xFFFFFFFF`. De paso se arregló un panic de los tres lectores con ficheros cortos
  (`leeTabla`, `leeAXML`, `leeMarco` leían `u16re` dentro del mensaje de error).
  **Falta el último puente**: leer los XML **en texto** (`res/values/*.xml`, layout y manifiesto) y
  producir los modelos con este marco, que es lo que la página va a necesitar.
- **3-G del texto al modelo** — **MITAD HECHA** (párrafo 35): `apkc/valores.go` lee un
  `res/values/*.xml` en texto y produce la `tablaEscribir`; el modelo sale **igual campo a campo** que el
  escrito a mano, los bytes que escribe `arscwriter.go` siguen siendo los 752 idénticos del fixture y la
  `R` sigue siendo el texto de aapt2. Rechaza las siete formas no medidas (color que no es color, bool
  que no es booleano, tipo de recurso desconocido, item que no es `android:`, padre inexistente,
  `<string>` con hijos y referencia propia resuelta antes de numerar).
  **Y el árbol ya esta hecho** (párrafo 36): `apkc/arbol.go` lee layout y manifiesto en texto con nombres,
  **líneas**, prefijos y orden de hijos; comprueba las siete líneas del binario del manifiesto (la del
  `activity` es la 13, y el decodificador de Go informa la 17 porque `InputPos()` cae detrás del `>`),
  guarda los prefijos en las dos direcciones porque Go da el URI en los atributos y el prefijo en las
  declaraciones, y rechaza seis formas rotas.
  **Falta el ultimo paso**: unir las dos mitades —descubrir los `@+id/…` del árbol, numerar la tabla con
  ellos, y escribir los dos AXML y la R a partir de lo materializado.
  **HECHO, y con esto la Fase 3 se cierra** (párrafo 37): `apkc/enlazar.go` hace las dos pasadas y el
  proyecto del studio enlazado **desde sus tres ficheros de texto** reproduce byte a byte los 784 bytes
  del layout y los 2000 del manifiesto, la tabla con los 752 iniciales idénticos (la única diferencia es
  el número de configuraciones declarado en el §28) y los 493 bytes de la `R`. Dos mediciones salieron de
  las pruebas: `match_parent` es un item enum de `layout_width` con valor −1 (sin él el layout salía con
  472 bytes distintos), y `platformBuildVersionName` es la **versión de Android** como entero (14), no la
  API (34).
  **Restricciones de la otra mitad**, razonadas antes de escribir código (un par de intentos salieron
  ficheros que no compilaban; se deja aqui lo que cuesta):
  - **Hace falta dos pasadas sí o sí**: un layout declara ids con `@+id/pulsar`, esos ids entran en la
    tabla, y la tabla se numera por posición; por tanto las referencias (`@style/AppTheme`,
    `@android:style/TextAppearance.Large`) solo se pueden escribir cuando la tabla ya está ordenada.
    El orden de emision ya está medido (§29): tipos alfabéticos, claves por tipo y dentro del tipo por
    nombre, entradas por nombre, registros de estilo por id de atributo.
  - **La línea del nodo no la da el decodificador de Go**: `InputPos()` cae detrás del `>` cuando los
    atributos ocupan varias líneas (en el manifiesto `<activity` abre en la 13 y cierra en la 17, y
    aapt2 anota 13). Hay que localizar el `<nombre` siguiente a un cursor que solo avanza.
  - Los `xmlns:` de un namespace que no es `…/apk/res/android` **no dejan huella** en el binario
    (medido en el §30 con `tools:`), así que el lector los descarta al construir el modelo.
  - El manifiesto **gana cuatro atributos** que no están en el fuente (§30), y el valor de
    `platformBuildVersionName` sale como **entero**, no como cadena.
  - El tipo de cada valor **no se deduce del texto, se pide al atributo del framework** (`wrap_content`
    es entero -2, `center` es hexadecimal 0x11) — §34. Lo que no esté en los enum/flags del attr cae
    por orden: referencia `@…`, booleano, color `#…`, entero, y si no cadena con su crudo.

- **3-H enganche en `apkc build`** — **HECHO** (párrafo 38): `--linker go` sustituye a
  `aapt2 compile` + `aapt2 link` dentro de la pipeline de siempre; escribe los cuatro productos que
  `packagePlan` ya sabe tomar y deja el empaquetado y la firma intactos. Sobre el proyecto del studio
  salen **idénticos byte a byte** el manifiesto (2000), el layout (784) y la `R` (493), idéntico el dex, y
  el apk de 10194 bytes lo verifica `apksigner` por v1 y v2, lo aprueba `zipalign -c -p 4` y
  `aapt2 dump badging` lo entiende. El portero rechaza con nombre lo no medido (cualquier calificador
  como `res/layout-es`, drawables, subcarpetas, no-XML, `<permission>`, una API no medida).

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

## Fase 5 — cerrar el subconjunto: de «lo que ya está medido» a «una app que se pueda escribir»

**Para qué sirve esta fase.** Lo que hay hoy compila en la pestaña apps del estilo del contador: campos,
`if`/`else`, cuentas enteras, cadenas con `StringBuilder` y una anónima de `View$OnClickListener`. Eso no es
un lenguaje: es el subconjunto que fuimos midiendo. La Fase 5 cierra lo que falta **con el mismo método**,
item por item, y el criterio de éxito no es «el parser lo acepta» sino: *la página compila y firma sin
procesos ni red una app con bucle sobre un arreglo, un `switch`, un `try/catch`, dos idiomas en `res/values`
y un icono en `res/drawable`, y esa app se instala y arranca en un teléfono real.*

### La plantilla, igual para todos los ítems (no hay excepciones)

1. **Sonda**: un fuente mínimo que use la construcción, escrito a mano en `$PREFIX/tmp`.
2. **Oráculos**: `javap -l -v -c` sobre el `.class` de javac 21; `d8 --release` sobre ese `.class`;
   `/apex/com.android.art/bin/dexdump` (con verificación) y `dexdump -d`/`-a` en el aparato por `rish`;
   `apksigner`, `zipalign -c -p 4`, `zipcheck.py` y `aapt2 dump` para el paquete; y **la ejecución**:
   `banco/banco.sh` corre el mismo fuente por los caminos (JVM, nuestro dex, `d8` sobre javac, `d8` sobre
   lo que emitimos y, desde el §66, nuestro dex con nuestro `main`) con `app_process64` en el aparato,
   sin instalar nada. Ese es el
   oráculo que destapó los tres primeros huecos del §61, ninguno visible comparando bytes.
3. **Emisor/traductor** solo con lo que dijo la medición; lo que la sonda no cubre se queda **rechazado por
   nombre** con un mensaje que diga qué falta.
4. **Prueba permanente** que compare contra las palabras/bytes del oráculo (el patrón de
   `dexinstr_test.go`, `dexdebug_test.go`, `dexanota_test.go` y `dexlayout_test.go`), y una prueba de
   layout ART que recorra el fichero entero.
5. **Gate**: un bloque nuevo en `studio/e2e.sh` (o en `prueba-pestana.js` si toca la pestaña) y, cuando el
   caso lo pide, **el teléfono**: instalar, arrancar y ver la traza o el `content-desc`.
6. **Registro**, en este orden y cada texto en su sitio: el párrafo nuevo en `docs/evidencia.md` con el
   comando que reproduce cada afirmación, y su commit; **la fila** del inventario de arriba si cambia lo
   que el subconjunto acepta o rechaza (y la tabla Estado si cambia el estado de una fase); `AGENTS.md`
   **solo** si cambia una regla, un muro del entorno o el gate — nunca un bullet de hallazgos (regla 7 de
   `AGENTS.md`; el recap de las mediciones es `docs/situacion.md`, que tampoco recibe bullets nuevos).

Orden recomendado abajo. Los **costes son hipótesis** (XS/S/M/L/XL), no mediciones: lo que manda este repo es
que la *tarea de medición* va siempre antes que la estimación de trabajo.

### Inventario, con el sitio exacto que rechaza hoy

| # | Qué falta | Dónde se rechaza ahora | Qué hay que medir antes | Coste |
| --- | --- | --- | --- | --- |
| 5.1 | `+=`, `-=`, `*=`, `/=`, `++`, `--` | **HECHO** (§67 y §68): `++`/`--` con `iinc` en el local y lectura-cuenta-escritura en el campo; `x += lit` es `iinc` si el literal cabe en un byte y `x op= expr` se compone como `x = x op expr` (`*=`, `/=` y deltas grandes). Los dex de `Inc` y `Mas` salen byte a byte los de javac. Frontera: `++`/`--` como valor (se rechaza por nombre) y `>>=`, `<<=`, `&=` (ni el tokenizador los arma) | — | hecho |
| 5.2 | literal `char` y escapes restantes | `javafuente.go:569` (más de un caracter), `:620` (escape no medido), `javaemisor.go:1978` | **HECHO** (§87): el `char` viaja como `int` en la pila del `.class` (`c + 1` es `iload_0; iconst_1; iadd`, sin conversión; `int n = c` no escribe nada) y estrechar es `i2c` **0x92** (el 0x91 es `i2b` y el 0x93 `i2s`, que siguen sin sonda). Formas propias: `caload` 0x34 y `castore` 0x55, que el traductor baja a `aget-char` **0x49** y `aput-char` **0x50**; el campo estático `C` usa `sget-char` **0x65** y `sput-char` **0x6c**, y la conversión es `int-to-char` **0x8e**. El léxico lee el caracter, los escapes `\n \t \r \b \f \' \" \\`, el octal (`'\101'` = 65), el unicode (`'\u0008'`, `'\uFFFF'` = `ldc` con 65535) y el UTF-8 suelto (`'á'` = `sipush 225`). `static final char` se dobla con `ConstantValue: int 90` y sin `putstatic`, igual que el `int`; la clave `case` de un `switch` es el punto de código. `new char[] {…}` sale con la cadena de javac (`dup`/`castore`), no con el `fill-array-data` de `d8` (§53). La elección `tableswitch`/`lookupswitch` queda escrita como diferencia: el criterio de este emisor es más conservador que el de javac, medido en la rejilla del §87 | — | hecho |
| 5.3 | literales de clase `X.class` | **HECHO** (§89): medidos `ldc` de `CONSTANT_Class` (tag 7) para `String.class`, `Object.class` y los arreglos (`[Ljava/lang/String;`, `[I`), y `getstatic Wrapper.TYPE:Ljava/lang/Class;` para `int.class`/`void.class` (no es `ldc`). En el dex es `const-class` **0x1c** (21c) y `sget-object` **0x62**. De paso entró la igualdad de referencias, que la sonda necesitaba como valor y como rama: `if_acmpne` **0xa6** / `if_acmpeq` **0xa5**, que bajan a `if-ne` **0x33** / `if-eq` **0x32**. El lector de clases no guardaba la etiqueta de `CONSTANT_Class` (fix en `classfile.go`). Nuestro dex sobre nuestras clases y sobre las de javac: **1412 bytes**, `cmp` mudo; `d8` da 1512 sobre las dos y las dos son idénticas. `Ver.clases()` da 2 por los cinco caminos del banco | §89 | hecho |
| 5.4 | llamadas de más de cinco argumentos y las formas `/range` | **HECHO** (§86 y §90): medidas sobre el fichero de `d8 --debug` y **emitidas** `move/from16` 0x02, `move-wide/from16` 0x05 y `move-object/from16` 0x08 —formato 22v—; `bajaAlto` ata el hueco alto a un temporal bajo una sola vez y `unidad23x` deja de recortar a nibbles sus dos últimos campos. El §90 cierra `invoke-*/range`: 3rc con `AA` = palabras, método y primer registro; `tomaBloque` reserva un bloque contiguo y `mueveRegistro` copia con 12x/22v, con el receptor primero (la primera versión lo puso al final y ART la rechazó con `tried to get class from non-reference register v0`). Las tres formas de la sonda (`7706`, `7708`, `7407`) dexa y corre el banco; nuestro dex es 1032 = 1032 y `d8` 1132 = 1132. Queda fuera el `/from16` de una cuenta (`const/16`, `add-int/lit16`), que el subconjunto no necesita. Prueba: `apkc/dex16_test.go` y `apkc/javaemisor_seis_test.go` | — | hecho |
| 5.5 | `private`/`protected` y los accesos sintéticos `access$…` | **HECHO** (§72): medidos los flags `0x0002`/`0x0004` y que un privado de la propia clase se llama con `invokespecial`. La anónima toca los privados del exterior por los `access$NNN` que javac escribe al final del `ClassFile` (flags `0x1008`): nombre = `access$` + posición del miembro × 100 + sentido (`0` leer/llamar, `2` escribir), posición por orden de primera creación y compartida por campos y métodos; cuerpos medidos para campo de instancia/estático y método de instancia/estático (el de escritura devuelve el valor y el llamador pone el `pop`). En el dex: privado de instancia = método **directo**, `invokespecial` = `invoke-direct`, y el `dup_x1` del accesor de escritura no emite nada. Frontera: una clase anidada `private` (constructor sintético y clase marcador) o `protected` con nombre y sin `static` (`this$0`) se rechazan por nombre | — | hecho |
| 5.6 | `throws`, el atributo `Exceptions`, métodos abstractos e interfaces propias | **HECHO** (§92): la interface sale con flags **0x0601** (sin `<init>` ni `ACC_SUPER`), sus campos con **0x0019** y `ConstantValue`, y sus métodos con **0x0401** y sin `Code`; la clase que la implementa lleva `interfaces: 1` y el `invokeinterface` se emite por el tipo de la interface (`Uso.usa`). `throws A, B` escribe `Exceptions` con uno y dos índices, después del `Code`; una clase abstracta conserva `ACC_SUPER` y suma `ACC_ABSTRACT`. Tres defectos del camino corregidos: `throws` no era palabra clave, un cuerpo `{ }` se confundía con abstracto y `accesosDe` no marcaba `ACC_INTERFACE` para una interface propia. Nuestro dex 1172 = 1172; `d8` 1504 = 1504; `Ver.usa(new Ver())` = 6 y `conThrows()` = 5 por los cinco caminos | §92 | hecho |
| 5.7 | constructores escritos a mano | **HECHO** (§91): medidos `this(...)` y `super(...)` (el parser no leía ninguno), el `super()` implícito con la línea de la firma, la delegación `this` que **no** baja inicializadores y el `super` explícito que sí los baja después. Un constructor vuelve `V` en el descriptor. El banco destapó que `areaCodigo`/`areaDebug` guardaban los mapas por `[clase, nombre]` sin descriptor: dos `<init>` compartían `code_item` y ART cortaba el dex (`Non-zero padding 3 before section of type 8195`); la clave lleva ahora el descriptor y `TestLosConstructoresSobrecargadosTienenCodeItemPropio` lo exige. Nuestro dex 860 = 860; `d8` 968 = 968; `new Ver().sumaCtor()` da 9 por los cinco caminos | §91 | hecho |
| 5.8 | `long`, `float`, `double` (dos huecos) | **HECHO `long`** (§74): en el `.class`, los dos huecos de local, las dos palabras de pila, la entrada `Long` del pool que reserva posición, `lconst_0/1`, `ldc2_w` y el `i2l` en el sitio del operando; en el dex, `const-wide/16|32|64`, `iget-wide`/`iput-wide`/`sget-wide`/`sput-wide`, `int-to-long`/`long-to-int`, `return-wide`, `move-result-wide` y los argumentos que valen dos registros. Nuestro dex sobre nuestras clases es **byte a byte** el que sale de las de javac, ART lo corre (`306`/`8`) y `d8` acepta las nuestras. **HECHO `float`/`double` en el `.class`** (§77): el léxico arma `2.5`/`1.5f`, el `Float` es tag 4 con un hueco y el `Double` tag 6 con dos, las cuentas `fadd`..`drem`, la conversión en el sitio del operando con la promoción doble > flotante > largo > entero, `fconst_*`/`dconst_*`, `freturn`/`dreturn`, la rejilla de casts numéricos y los marcos `ITEM_Float` 2 / `ITEM_Double` 3; los trece métodos salen como los de javac y **nuestra clase corre en la JVM con `-Xverify:all`**. **HECHO además**: la comprobación en ART del punto flotante (§82, con `sudo` porque el reset dejó sin `rish`), las cinco comparaciones anchas con `lcmp`/`fcmpl`/`fcmpg`/`dcmpl`/`dcmpg` y sus ramas (§81), `dup2`/`pop2` con los campos anchos privados (§83), `faload`/`fastore`/`daload`/`dastore` con `aget`/`aget-wide`/`aput`/`aput-wide` en los arreglos de `long`, `float` y `double` (§84 y §85) y el **plegado del literal promocionado** (§85: `f * 2` es `fconst_2`, `d + 1` es `dconst_1`, `l + 2` es `ldc2_w` porque no existe `lconst_2`, `d * 2` es `ldc2_w` porque javac nunca usa `dconst_2`, `f * -2` es `ldc -2.0f` con el signo dentro de la constante, `2.5f * 2` es una sola `ldc 5.0f`, el cero flotante de divisor se pliega a `Infinity`/`NaN` mientras `1 / 0` y `1L / 0` siguen dejando la excepción en ejecución, y la cuenta de `float` va en precisión de `float32`). **Falta** la forma de tres registros de una cuenta ancha de flotante —medida en `d8` como `ab02 0210` = `add-double v2, v2, v16` (§86) pero todavía no emitida por el traductor— y el `ConstantValue` de un `static final long` | el `parameters_count` del `debug_info_item` son parámetros declarados, no palabras (§74) | M restante |
| 5.9 | `try`/`catch`/`finally` | **HECHO** (§71, §75 y §76): medidos el rango exclusivo y que excluye un `return`/`goto` final pero incluye el `athrow`, el handler que empieza en el `astore` (y `catch_type` con el nombre interno), y en el dex el `try_item` + `encoded_catch_handler_list` con `move-exception` (0x0d) y `throw` (0x27). §75 añade el `finally` —copia del bloque al pie de cada camino, manejador `any` que guarda la excepción y la re-lanza, los huecos de local (el mismo para cada `catch`, el más alto para el `any`) y las filas de línea de la copia y de la llave— y los dos `catch` en el mismo `try`. §76 añade el **valor pendiente**: cada `return`/`break` que cruza el bloque cierra un trozo del rango y el siguiente empieza después del `return`/`goto`, con el valor en un hueco sin nombre. El dex de la sonda (doce métodos) sale byte a byte el de javac (2052) y el banco lo ejecuta. **Falta** el `continue` que sale, los dos `finally` anidados con salida pendiente (rangos solapados) y `catch (A \| B e)` | — | hecho |
| 5.10 | **bucles** `for`, `while`, `do` | **HECHO** (§63, §69 y §70): medido el `goto` 10t con desplazamiento negativo (`28f7`) y la forma de javac; el traductor ya acepta la rama hacia atrás, el analizador escribe `while`, `do-while` (condición directa) y `for` con nodo propio (`jpara`, para que `continue` salte al paso), todo con el **`Code` idéntico al de javac**; `break` y `continue` van a pilas de contextos (el fin del bucle o del switch; cabecera, condición o paso). Los dex de `Bucle`, `Doo` y `Corte` salen byte a byte los de javac y el banco los ejecuta en ART. Solo queda fuera `goto/16` | — | hecho |
| 5.11 | **arreglos** (`new int[n]`, `s[i]`, `length`, de referencia y multidimensional) | **HECHO** (§66): medidos `new-array` (0x23, 22c), `array-length` (0x21, 12x), `aget`/`aget-object` (0x44/0x46, 23x) y `aput`/`aput-object` (0x4b/0x4d); el analizador lee `T[]`, `new T[n]`, `v[i]` (lectura y destino) y `.length`; el dex de nuestra `Arreglos` es byte a byte el de javac. Ejecutar en ART destapó dos defectos: la cabecera con desplazamiento en una tabla vacía (`field_ids_off` tiene que ser 0) y la cuenta que pisaba un operando vivo (`v[i] = i + 1` daba 3), corregida con las tres formas que usa `d8` (`/2addr` izquierda, `/2addr` derecha conmutativa, 23x). Un `main(String[])` compilado y dexado solo por este fork corre en ART con la misma salida que la JVM. Frontera: más de una dimensión y `arraycopy`. El **inicializador `new T[] {…}`** entró después (§73): el analizador lo lee y el emisor escribe la cadena de javac (largo, `newarray`, y por elemento `dup`/indice/valor/`iastore`), con los rechazos de `new T[]` sin inicializador, dos dimensiones y elemento de otro tipo. Los **elementos e inicializadores anchos** (`long[]`, `float[]`, `double[]`) llegaron con el §84 y el §85: en el `.class` son `laload`/`faload`/`daload` y `lastore`/`fastore`/`dastore`, el inicializador ancho baja **cuatro** palabras por elemento (arreglo, índice y las dos del valor) y no tres, y la sonda de diez métodos sale con el dex **byte a byte** el de javac (1496 bytes) | — | hecho |
| 5.12 | `switch` de entero | **HECHO** (§64): el dexer camina `tableswitch`/`lookupswitch` (relleno hasta el múltiplo de cuatro desde el principio del `Code`) y escribe `packed-switch`/`sparse-switch` con el payload al final (identes `0x0100`/`0x0200`; en el disperso **todas las claves y luego todos los destinos**), y el analizador lee `switch` del fuente (brazos que terminan en `break`/`return`; la caída al siguiente se rechaza por nombre) con el `.class` **idéntico al de javac** salvo el `StackMapTable` que se añadió en el §65, y el dex **byte a byte** el suyo. Ojo: ART exige `ACC_CONSTRUCTOR` en `<clinit>`, medido por su propio aviso | — | hecho |
| 5.13 | `instanceof` y cast entre referencias | **HECHO en el `.class` y en el dex nuestro** (§79): `aload; instanceof #k; ifeq` con la condición **sin invertir**, `0xc1` con indice de dos bytes, el marco `same` en el `else`; en el dex `instance-of` **0x20** en 22c, medido sobre el fichero de `d8` (`Ins2.f` da `4128 2` = `instance-of v0, v1, type@2`) con un temporal para el destino. Los cinco métodos de `Ins` salen iguales que javac, el dex es idéntico entre nuestras clases y las suyas, y la JVM con `-Xverify:all` ejecuta la nuestra dando `1|0|2|3|2|cadena|5`. **Falta** la comprobación en ART (sin `rish` no hay `dexdump` ni banco), `x instanceof T[]` y la forma negada | §79 | hecho |
| 5.14 | anónima dentro de método `static` y captura de variables locales effectively-final | **HECHO** (§99 y §100): la anónima en `static` sin captura sale con constructor `()V`, sin `this$0`; la captura de un parámetro/local effectively-final baja a `val$x`, con constructor `(I)V` en static (sin `MethodParameters`) y `(LCaptura3;I)V` en instancia (dos filas: exterior 0x8010 y `val$x` 0x1010), y asigna antes del `super`. Campos, `Code`, filas de línea y dex salen byte a byte los de javac en los dos casos. Frontera: anónima dentro de otra anónima sigue rechazada | §99 y §100 | hecho |
| 5.15 | `Signature` (genéricos) en el dex | **HECHO** (§101): se leen `Signature` de clase, campo y método; `d8` lo copia a `Ldalvik/annotation/Signature;` con la firma troceada (cada `L…;`/`L…<` es un trozo y el resto `<T::`, `>`, `()`, `>;` va aparte). El directorio de anotaciones escribe también `fields_size` y los `field_idx`. `dexdump -a` del fichero de este fork y del de `d8` coinciden en clase, campo y método; `apkc/dexanota_firma_test.go` fija los cuatro troceados | §101 | hecho |
| 5.16 | **recursos**: `res/drawable*` con PNG, `mipmap` y `roundIcon` | **HECHO** (§93): el PNG se copia literal al apk y su entrada es un `Res_value` de tipo cadena con la ruta; `aapt2 dump resources` del apk que escribe este fork declara `drawable/icono (file) res/drawable/icono.png type=PNG` y `mipmap/ic_launcher`; el PNG del zip tiene el mismo `sha256sum` que el del proyecto. `roundIcon` es un atributo del manifiesto y no añade nada. El arsc mide 928 bytes frente a 1060 de `aapt2` por la segunda configuración `layout-v1` de los layouts, ya aceptada en el §28. `studio/e2e.sh` lo comprueba con `apksigner` y `zipalign` | §93 | hecho |
| 5.17 | **recursos**: calificadores (`values-es`, `layout-land`, `drawable-hdpi`) y `config` por API | **HECHO** (§94 y §103): `values-xx` de dos letras con dos sondas byte a byte contra `aapt2` (728 y 716 bytes), `layout-land` (orientation 2 en +12, spec 0x0080) y `drawable-hdpi` (density 240 en +14, sdkVersion 4, spec 0x0500, ruta `-v4`): el arsc de las tres configuraciones sale **1204 bytes idéntico** al de `aapt2 link --no-auto-version`. Medidos language, `spec_map`, offset 0xffffffff, claves sin base y orden del pool. Frontera: regiones (`es-rMX`), otras densidades/orientaciones y la copia `layout-v1` que el aapt2 por defecto añade | §94 y §103 | hecho |
| 5.18 | **recursos**: tipos que faltan en `values` (`color`, `integer-array`, `string-array`, `plurals`, `bool`, `fraction`, `attr`/`styleable`) | **HECHO en parte (§96 y §105)**: `string-array` e `integer-array` son entradas complejas con `padre=0`, flags 0x0001 y registros de nombre `0x01000000\|(índice+1)`; `plurals` usa los ids de cantidad de Android en el orden del fuente; `fraction` es tipo 0x06 con radix 3 para <1, `0x100` para 1.0, radix 2 para >1 y el bit de `%p` (796 bytes idénticos). `bool`, `color` e `integer` ya estaban. Faltan `attr` y `styleable`, que se rechazan por nombre | §96 y §105 | parcial |
| 5.19 | **recursos**: `menu`/`xml` (AXML con raíz que no es `layout`), `raw`, y el `<permission>` del manifiesto | **HECHO** (§97 y §102): `res/menu` y `res/xml` se enlazan como AXML con su raíz y `res/raw` viaja literal; la tabla declara `menu/principal`, `xml/prefs` y `raw/dato`. El `<permission android:name=…>` se recoge del manifiesto y `enlazarConGo` escribe `src/<paquete>/Manifest.java` byte a byte como aapt2 (cabecera, clase y constantes en el orden del manifiesto). Frontera: atributos de `<permission>` distintos de `name` | §97 y §102 | hecho |
| 5.20 | versión de platform no medida y `--codename`/`--sdk-version` | **HECHO para API 28-35 (§98)**: la tabla API→versión de Android es 28→9, 29→10, 30→11, 31→12, 33→13, 34→14 y 35→15; con ella se escriben `android:compileSdkVersionCodename` y `platformBuildVersionName` (entero). Las APIs sin nombre numérico (27→8.1, 32→12L) y las no medidas se siguen rechazando por nombre; `--codename` sigue siendo la salida para ellas | §98 | hecho |
| 5.21 | keystore de release propio en la pestaña y banderas de firma (`v3`, `--min-sdk` del proyecto) | la pestaña pasa `--keystore` con el de depuración | **HECHO en parte (§104)**: `apkc verify` tolera y anota una v3 ajena (`studio/e2e.sh` hace `con-v3: apksigner si, apkc verify si`), pero no la verifica ni la escribe; la lectura de PKCS#12/JKS ya estaba en `apkc sign`, y la pestaña sigue con el keystore de depuración. Falta el bloque v3/v4 y un keystore de release en la página | §104 | parcial |
| 5.23 | `StackMapTable` en el `.class` que emitimos | **HECHO** (§65): medido con `javap -v` dónde pone javac cada marco (sin ramas no hay; en cada destino de salto y tras cada salto incondicional o `return`; `offset_delta` del primero = su pc) y que `top` gana en la unión y que un destino puede ser el pc 0. `apkc/jvmframes.go` calcula los estados con un intérprete abstracto sobre el cuerpo parcheado y escribe `full_frame` (forma legal sin la compresión de javac); el analizador exige que el alto simulado quepa en `max_stack`. Oráculos: `TestLosMarcosSonLosDeJavac` (nuestros marcos expandidos dicen lo mismo que los de javac en `while`, `switch` y la cabecera en el pc 0) y el banco, que ahora ejecuta nuestra `Ver` con `java -Xverify:all` y la compara por texto y rc | — | hecho |
| 5.24 | `getstatic`/`putstatic` de una **referencia** (`System.out`) | **HECHO** (§62): medido `sget-object` `6200` y `sput-object` `6901` en el fichero de `d8`; `accesoEstatico` acepta `I` y `L` y el resto se rechaza por nombre. De paso, el internado no recorría los estáticos de otra clase y un campo ajeno nunca entraba en `field_ids` (corregido en `dextabla.go`) | — | hecho |
| 5.25 | temporales en el punto de unión de un `if` con `else` | **HECHO, y la causa no era la unión** (§62): cada `putstatic` pedía un temporal con `toma` y luego lo tiraba al sacar el registro de la pila, así que el marco se quedaba sin temporales; corregido. La unión de brazos no necesitó nada. Queda por medir el presupuesto cuando un bucle tenga valores vivos en la cabecera (5.10) | — | hecho |
| 5.26 | Resolución de sobrecargas (`println(String)` salía `println(Z)`) | **HECHO** (§62): candidatas por nombre y aridad de la jerarquía, exacta por descriptores, si no la aplicable más específica por cadena de padres e interfaces del jar (JVMS 15.12.2.5), y si no rechazo por nombre. `boxing`, varargs, arrays y ensanchamiento siguen fuera, por nombre | — | hecho |
| 5.27 | Llamar a un método **sin receptor** desde un `static` | **HECHO** (§62): javac escribe `invokestatic` contra la clase actual, que es la forma 35c ya medida; se rechaza si el método no es estático | — | hecho |
| 5.22 | `invokedynamic` (lambdas y references de método) | **FUERA, rechazado por nombre (§104)**: el lector de `.class` rechaza `invokedynamic` en `refuseOutsideSubset` y el compilador de fuente no tiene lambdas ni referencias de método. La desugración completa (`lambda$…`, capturas, `method_type`/`method_handle` y bootstrap) es la pieza XL del plan y no se ha medido; se queda como frontera escrita, no como hecho | §104 | fuera de alcance |

### Dependencias y orden

- **Primer bloque (semana de cosas pequeñas, desbloquea escribir Java normal):** 5.1, 5.2, 5.3, 5.4, 5.13,
  5.20. No dependen de nada nuevo y cada uno se cierra con su sonda.
- **Segundo bloque (dos huecos antes de lo que los usa):** 5.8 **antes** de 5.9/5.10/5.11, porque los
  locales de dos huecos cambian la numeración de la frame (§57) y cualquier bucle que acumule un `long` la
  necesita.
- **Tercer bloque (control de flujo):** 5.10 (bucles) y 5.12 (`switch`) juntos —los dos son *ramas* y
  comparten el mecanismo de fusión y el `goto/16`—, después 5.9 (`try/catch`), que además toca el
  `code_item`.
- **Cuarto bloque (datos):** 5.11 (arreglos), **hecho sin 5.8** (§66): los tipos `[` entraron con el
  subconjunto que el fuente escribe (`int[]`, `String[]`) y la frame los nombra con su descriptor.
- **Quinto bloque (miembros y tipos):** 5.5, 5.6, 5.7, 5.14, 5.15.
- **Sexto bloque (recursos que hacen que la app parezca una app):** 5.16, 5.17, 5.18, 5.19.
- **Séptimo (opcional, el más caro):** 5.21, 5.22.
- **5.23 y 5.24 tocaron antes**: eran de la familia «el fuente real lo necesita» (`System.out` y el
  `.class` verificable), no de control de flujo; 5.25 es el preludio obligado de 5.10. Las dos están
  cerradas (§62 y §65).
- **Lo primero de todo fue el ejecutor** (§61): antes de tocar un ítem, `banco/banco.sh` corre el fuente
  por los cuatro caminos. En una tarde destapó cinco cosas que ningún oráculo de bytes veía (5.23 a
  5.27 y el `invokestatic` sin argumentos).
- **Lo que sigue**: `attr`/`styleable` de 5.18, las regiones de 5.17, las APIs sin nombre numérico de
  5.20 (27, 32 y las no medidas) y, en la parte opcional, escribir v3 (5.21) y la desugración de
  `invokedynamic` (5.22), hoy fuera de alcance y rechazada por nombre.
  Después, el `continue` que sale de un `try` con `finally` y los dos `finally` anidados con salida
  pendiente (5.9, §76). El punto flotante del dex, las ramas anchas, el `char`, los literales de clase,
  `/range`, los constructores, `throws`/interfaces, el icono y `values-es` ya están cerrados
  (§80-§94).

### Hitos con prueba en el teléfono

- **M1** (fin del primer bloque): el contador reescrito con `+=` y `++`, un literal `char`, un
  `MainActivity.class` y un método de seis argumentos; instalado, arrancado y con la traza de una excepción
  en la línea exacta. Todavía **sin bucles**: es el hito que deja el Java de todos los días salvo el control
  de flujo.
- **M2** (con 5.8): la misma app acumulando en `long` y mostrando un `double` formateado, con la traza de
  una excepción en la línea exacta (sonda del §60 ampliada).
- **M3** (con 5.10/5.12/5.11): **una lista de verdad**: arreglo de cadenas, bucle que lo recorre, `switch`
  por posición y `try/catch` alrededor; instalada, con dos `input tap` y el `content-desc` comprobado.
- **M4** (con 5.16/5.17): la app con icono en `res/drawable` y textos en `values-es`, instalada y con
  `aapt2 dump badging`/`resources` aprobando el apk que salió del navegador.

### Fuera de esta fase, con el motivo escrito

- **Kotlin** (el `apk` de kotlin que ve el gate sale del camino con JVM; en Go no hay frontend).
- **`multianewarray`, `synchronized`/`monitorenter`/`monitorexit`, `assert`, y reflexión** — ninguna sonda;
  siguen rechazados por nombre.
- **Más de 65 536 métodos (varios `classes.dex`)** y **.aab con `split` por densidad/idioma**: son fases
  propias, no ítems de ésta.
- **Interoperabilidad con Gradle**: no está en la ambición.
- **`d8` como referencia para lo optimizado**: no imitaremos las reescrituras de `d8` (fusiones de cadena,
  dead code). El oráculo manda sobre las *palabras y las estructuras*, no sobre el cuerpo optimizado, que ya
  está aceptado como diferencia en el 2G.4 y medido en el §53.

### Mata la fase (o la parte que toque)

- **5.10**: si la medición del punto de fusión sale que hace falta un **análisis de viveza completo** (un
  segundo asignador de registros con fusión por `StackMapTable`) para que un bucle verifique en ART, 5.10 deja
  de ser un ítem y es una fase aparte: se escribe el coste medido y se decide antes de seguir.
- **5.9/5.11**: si un caso razonable de `try`/arreglo exige estructuras que el formato define con
  variantes no observables en nuestras sondas (p. ej. `on-demand` de la tabla de handlers por versión de
  `dex`), se queda como diferencia escrita y el portero lo rechaza por nombre, como hasta ahora.
- **5.17**: si una segunda configuración con `spec_map` no sale byte a byte igual que `aapt2` en dos
  sondas distintas, se para y se escribe: los recursos con calificadores son mantenimiento eterno (§
  «Riesgos transversales»).

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
