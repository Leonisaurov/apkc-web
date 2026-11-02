# AGENTS.md — apkc-web

Este repo guarda **la ambición de compilar una app Android desde el navegador, del lado del
cliente**, separada de la herramienta que funciona hoy. Es documentación + un experimento + una
página. Nada de aquí cambia `../apkc` salvo lo que la Fase 0 autorice expresamente.

## Mapa

| Ruta | Qué es | Cómo se comprueba |
| --- | --- | --- |
| `docs/evidencia.md` | lo medido, cada afirmación con su comando | leerlo |
| `docs/bloqueos.md` | qué existe y qué costaría sustituir `aapt2`, `javac`, `d8` | leerlo |
| `docs/plan.md` | el plan detallado, con fases y criterios de abandono | leerlo |
| `shim/navegador-sim.js` | hospedador con forma de navegador (un `fs` en memoria con la API de Node; monta ficheros reales con `--montar` y devuelve resultados con `--exportar`) | `node shim/navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp` |
| `apkc/` | **el fork de la herramienta** (subtree desde `../apkc`, MIT, pin `a909214`): donde se escribe el dexer, el compilador y los recursos en Go | `cd apkc && go test ./...` y el wasm: `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-web-wasm.wasm" .` |
| `studio/` | apkc-studio: genera proyectos apkc, traduce su salida para medir fricción **y hace correr `apkc.wasm` en la pestaña** (OPFS + worker) | `cd studio && APKC=<binario> sh e2e.sh`; la pestaña: `APKC_WASM=… node studio/prueba-pestana.js` |
| `banco/` | **el banco de ejecución**: dos fuentes de prueba y un guion que corre el mismo programa por los cinco caminos (JVM, `d8` sobre javac, `d8` sobre lo que emitimos, **nuestro dex en `app_process`** y, desde el §66, **nuestro dex con nuestro `main`**, sin javac ni d8) — sin instalar nada, sin pantalla | `sh banco/banco.sh` (necesita `APKC` compilado, `ANDROID_JAR`, `build-tools` y `rish`) |

## Reglas del repo

1. **Un claim, una medición.** Una afirmación sobre coste, compatibilidad o formato que no traiga
   el comando que la reproduce no entra en `docs/evidencia.md`: va a `docs/plan.md` como hipótesis,
   marcada como tal. Es la razón de existir de este repo.
2. **Sin binarios versionados.** `*.wasm` está ignorado: se construye desde el fork `apkc/` con
   `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .`. Tampoco se
   versiona `shim/wasm_exec.js` (es del GOROOT de cada quien).
3. **El código de la herramienta vive aquí, en `apkc/`.** Es un fork (MIT, `ajinasokan/apkc`) metido con
   `git subtree add --prefix=apkc ../apkc master --squash` sobre el commit `a909214`, en la rama `web`:
   el dexer, el compilador y los recursos se escriben **aquí**, no allende el muro. Un `git subtree pull`
   desde `../apkc` sigue siendo posible porque hay historial compartido; si algún día se hace un merge
   hacia arriba, se hace desde esta rama. Cada pieza entra con su commit, su test y su gate.
4. **Sin conexión y sin runtimes de terceros** (decisión del usuario, 2026-10-27). Todo lo que
   compila, empaqueta y firma viaja en el propio origen de la página: ni CDN de una JVM wasm, ni
   telemetría, ni fuentes. Se comprueba conduciendo chromium y registrando cada petición
   (`studio/prueba-pestana.js`); **una petición fuera del origen rompe la prueba.** Si aun así se
   evaluara un runtime ajeno, su licencia se lee **antes** de integrarla: si no permite el uso, la
   fase muere y se escribe por qué.
5. **Idioma.** Prosa y comentarios en español con la ortografía completa (tildes, ñ) y **cero
   caracteres de otros alfabetos**; pasar el chequeo antes de commitear. En `../apkc` los comentarios
   son en inglés (estilo del upstream): no arrastrar esa convención aquí, pero tampoco acentuar
   nunca texto que sea literal de una herramienta o contenido de un fichero generado.
6. **Commits** `<type>(<scope>): <summary>` (fix|feat|docs|refactor|chore|ci|test), mensaje auditado.

## Entorno (Termux/Android aarch64)

- Rutas: `$PREFIX=/data/data/com.termux/files/usr`, `HOME=/data/data/com.termux/files/home`. Nunca
  `/tmp`, `/usr/bin`, `/etc`. Shebangs absolutos. El shell interactivo del usuario es fish: envolver
  asignaciones de entorno en `sh -c '...'`.
- Comando pesado por `tcr` (`tcr -j 1 -q -- <cmd>`), jamás `-j$(nproc)`. Antes de algo caro:
  `free -m` y `df -h "$PREFIX"`.
^- El SDK está en `~/Android` (build-tools 34.0.0, `platforms/android-34/android.jar`) y el JDK en
  `$PREFIX/lib/jvm/java-21-openjdk`.
- **Dos trampas de wasm ya conocidas**: (a) `go test -exec=$GOROOT/lib/wasm/go_js_wasm_exec` falla
  con `total length of command line and environment variables exceeds limit` si no se le da
  `env -i`; (b) sin `GOOS=js GOARCH=wasm` el lanzador recibe un ELF y se queja de
  `expected magic word 00 61 73 6d`.
- Oráculos que no comparten código con apkc: `apksigner verify --verbose`, `jarsigner -verify`,
  `zipalign -c -p`, `~/Develop/Go/apkc-harness/zipcheck.py`, `javap`, `java -Xverify:all` (la JVM del
  Termux ejecuta **nuestros** `.class`: destapó el `StackMapTable` que faltaba en el §61 y lo cierra en
  el §65) y
  `dexdump`/`app_process64` **del aparato por `rish`** (no hay `adb`): `/apex/com.android.art/bin/dexdump`
  verifica y desarma, y `app_process64` con `CLASSPATH` ejecuta un `.dex` suelto **sin instalar nada**
  (`sh banco/banco.sh` lo hace por los cuatro caminos y compara el código de salida, §61).
- **Trampas de OPFS/cromium ya conocidas** (detalle completo con síntoma en `docs/evidencia.md` §12):
  `navigator.storage` no existe dentro del worker (la raíz baja desde la página por `postMessage`);
  `crypto`/`performance` son de solo lectura en un worker de módulo; `entries()` rinde
  `[nombre, mango]` y el tipo es `mango.kind`; `createSyncAccessHandle()` es **asíncrona** aunque sus
  métodos sean sincrónicos; y sin COOP/COEP no hay `crossOriginIsolated` ni handle sincrónico.

## Gate antes de dar nada por hecho

```sh
# binario wasm y la suite en hospedador de navegador (todo desde el fork apkc/)
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$PREFIX/tmp/apkc-test.wasm" .'
sh -c 'cd shim && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" . && node navegador-sim.js "$PREFIX/tmp/apkc-test.wasm"'

# la firma, del lado del cliente: un apk real entra y sale verificado por los oráculos
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd shim && node navegador-sim.js \
  --montar "$PREFIX/tmp/sign-probe/app/build/app.apk:/in.apk" \
  --montar "$HOME/.android/debug.keystore:/ks.p12" \
  --exportar "/out.apk:$PREFIX/tmp/sign-probe/wasm-signed.apk" \
  "$PREFIX/tmp/apkc-wasm.wasm" sign --keystore /ks.p12 --out /out.apk /in.apk'

# la página, de punta a punta (genera, compila de verdad, firma, verifica, y la conduce chromium)
sh -c 'cd apkc && go build -o "$PREFIX/tmp/apkc-gate" .'
sh -c 'cd studio && APKC="$PREFIX/tmp/apkc-gate" APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" sh e2e.sh'
```

`studio/e2e.sh` es innegociable antes de decir "la página funciona": su test de node solo veía
verdes seis fallos que solo aparecían al descomprimir y compilar lo generado.

## Lo que ya se sabe, para no reabrirlo

- El bloque de apkc que no lanza procesos (escritor con alineado, firma v1+v2, lectores de keystore,
  `create`) **corre en hospedador tipo navegador**: 98/98 tests.
- **Y corre en chromium de verdad**: `studio/` tiene hospedador con OPFS (worker de módulo + servidor
  sin dependencias con COOP/COEP) y editor de fuente; `create`, editar entre sesiones y `sign` funcionan
  dentro de la pestaña, verificado por fuera (16 comprobaciones en `docs/evidencia.md` §11 y §15).
- **`apkc sign` existe** (apkc `a909214`) y **firma dentro de wasm**: 1,20 s para un apk de 9504
  bytes, cuyo resultado verifica `apksigner` (v1 y v2), `zipalign -c -p 4`, `jarsigner -verify` y
  `zipcheck.py`. Es la primera pieza de un build real que corre del lado del cliente.
- **`apkc dex` existe y corre dentro de wasm** (§23, rama `web`): `.class` -> `classes.dex` en Go, sin
  `d8` ni proceso. Sobre el proyecto del studio: 2680 bytes (2392 hasta el §55, que quitó el relleno
  entre `class_data_item`; 2436 desde el §56, que añade el área de depuración; 2696 desde el §59, que emite
  el bloque de anotaciones; 2680 desde el §60, que interna los `type_list` por contenido y suelta la
  posición de la llave de cierre que el `--release` no escribe). La diferencia con los
  2844 de `d8` son hoy su marca de origen (descartada a propósito, §60), el orden de los ítems del área de
  anotaciones y los offsets absolutos, todo medido, con
  `type_ids`/`field_ids`/`method_ids` byte a byte iguales y los
  nueve cuerpos con la misma secuencia de instrucciones, los mismos índices y el mismo número de
  palabras. El oráculo que lo cierra: `d8` lee el dex que escribimos y lo re-dexa con `rc 0`, y el
  fichero que sale del binario `GOOS=js` es **idéntico byte a byte** al del binario nativo.
  Formato medido en §20, §21 y §22: cuatro afirmaciones que estaban mal en el §20 se corrigieron
  comparando nuestro fichero con el de `d8`, no con un manual.
- Al re-firmar un archivo que no hicimos nosotros hay dos reglas que **se leyeron de un oráculo**, no
  se adivinaron: los **directorios** viajan en el paquete y **fuera del manifiesto** (así lo hace
  `java.util.jar`, medido con `jarsigner`), y el firmante anterior se descarta **por sufijo** porque
  este keystore de debug hace que `apksigner` escriba `ANDROIDD.SF`, no `ANDROID.SF`.
- `apkc build` en wasm responde `aapt2 compile never started: pipe: not implemented on js`: el muro
  es la ausencia de proceso, no una ruta mal. **Ningún flag lo arregla.**
- **No se presta una JVM: se emite en Go** (decisión 2026-10-27, sin conexión y sin terceros). El
  objetivo del dexer está medido (§13: 2844 bytes, 217 objetos para el proyecto del studio) y la
  prueba de que la página no habla con ningún CDN está automatizada (§15). El orden ahora es
  **2G -> 3 -> 1G**; ver `docs/plan.md`.
- **La Fase 3 está cerrada, y enlazando desde texto** (§29 a §37): `apkc/arscwriter.go` emite la tabla de
  recursos (idéntica hasta donde aapt2 la parte en dos configuraciones por API), `apkc/axmlwriter.go`
  emite **exactamente** los 784 bytes del layout y los 2000 del manifiesto, `apkc/r.go` emite la misma
  `R.java` que `link --java`, `apkc/marco.go` saca de `android.jar` los ids del framework (incluidos los
  enum y flags de cada atributo) y `apkc/enlazar.go` + `arbol.go` + `valores.go` **enlazan el proyecto
  desde sus ficheros de texto**, sin `aapt2`. `apkc/empaquetar.go` ensambla un apk de 9868 bytes que
  `apksigner` verifica por v1 y v2, que `zipalign -c -p 4` aprueba y que `aapt2 dump badging` entiende.
- **`apkc build --linker go` enlaza sin `aapt2`** (§38): con esa bandera no se ejecuta `aapt2 compile` ni
  `aapt2 link`; `apkc/linker.go` escribe `build/AndroidManifest.xml`, `build/resources.arsc`,
  `build/res/layout/*.xml` y `src/<pkg>/R.java`, y el empaquetado y la firma de siempre los toma. Sobre el
  proyecto del studio los tres AXML y la `R` salen **idénticos byte a byte** a los de aapt2, el dex
  idéntico, y solo la tabla difiere por la única configuración base declarada en el §28; el apk (10194
  bytes) lo verifica `apksigner` por v1 y v2, lo aprueba `zipalign -c -p 4` y `aapt2 dump badging` lo
  entiende. El portero rechaza con nombre lo no medido: cualquier calificador (`res/layout-es`), recursos
  que no sean values/layout, subcarpetas, no-XML, `<permission>` y una version de platform no medida.
  **Con `--linker go` a secas sí se ejecutan `javac` y `d8`**; lo que ya no se ejecuta es aapt2. Con
  `--compiler go` tampoco esos dos: ver el párrafo de abajo.
- **El objetivo está cubierto dentro de la pestaña** (§45): `studio/index.html` pide el
  `android.jar` (selector de fichero o `/android-jar` del propio servidor) y lo guarda en OPFS como
  `/sdk/android.jar`; el botón `compilar-apk` corre `build --linker go --compiler go` con `--keystore` y deja
  el apk listo para descargar. Medido en chromium real: 26361808 bytes de jar que entran y se releen
  iguales, último código de salida `0`, apk de **9141 bytes** con **los mismos bytes en cada entrada** que el
  build nativo del mismo proyecto, aprobado por `apksigner` (v1 y v2), `zipalign -c -p 4`, `zipcheck.py` y
  `aapt2 dump badging`; 12 peticiones y ninguna fuera del origen. Cuesta dos fallos del hospedador arreglarlo
  (carrera de arranque en `hospedador.js`, directorios intermedios que `fs-fragmento.js` no creaba en
  memoria), y el aviso de «intermitencia de OPFS» del §39 era en realidad el primero.
- **Y también el proyecto que la página genera** (§46): con la clase anónima del botón, el `@+id` y el
  `@style`, chromium escribe los cinco ficheros en OPFS con el propio `buildProject`, saca **seis clases y
  el dex del fixture (2680 bytes desde el §60; 2696 con el bloque de anotaciones del §59; 2436 hasta el §58)**
  y un apk igual entrada por entrada al
  nativo, y `d8` releee ese dex con `rc 0`. Lo pide `studio/e2e.sh` (bloque 4c).
- **El `android.jar` entra sin red por el camino del usuario y dura** (§46, comprobaciones 9 y 9b): se le
  pone en el `<input type="file">` con `DOM.setFileInputFiles` de CDP, la página lo guarda en OPFS con
  **cero peticiones** en ese tramo, y tras recargar la página sigue ahí (26361808 bytes, sin volver a
  pedirlo) y el build vuelve a salir `0`. La prueba de pestaña llega a **32 comprobaciones**.
- **`apkc verify` dictamina el apk dentro de la pestaña** (§47): recalcula el digest de cada entrada,
  digiere el manifiesto sección a sección contra el `.SF`, verifica el PKCS#7 sobre los bytes del `.SF` y
  comprueba el bloque v2 (registro final con el desplazamiento sustituido). `apksigner` coincide en los
  ocho casos medidos, mismo veredicto y mismo código de salida, **incluidos dos que apkc no escribió**:
  un apk firmado por el propio `apksigner` y uno con un par de id desconocido. Dos reglas del formato las
  dijo el oráculo, no un manual: **el bloque de firmas no entra en el digest del contenido** (un byte en
  el par de relleno no lo detecta nadie; uno en el valor del par de la v2 lo detectan los dos) y **los
  pares desconocidos se ignoran y se anotan**, porque herejarlos daría menos aprobados que `apksigner`.
  No valida la cadena de confianza (sin ancla) y rechaza por nombre DSA/EC, digest que no sea SHA-256,
  varios firmantes y las firmas v3/v4.
- **El subconjunto de Java crece midiendo, y ya acepta un fuente propio** (§49 y §50): campos con
  modificadores leídos de `javap -v` (0x0000 a 0x0010), inicializadores bajados al `<init>` con su
  `LineNumberTable`, `<clinit>` al final, locales con `checkcast`, asignación a campo y aritmética entera
  (`add-int/2addr 0xb0` y su familia, medida sobre palabras que escribió `d8`). Un proyecto con
  `private int pulsos = 0`, `Button b = (Button) findViewById(R.id.pulsar)` y `total * 2` compila entero en
  Go: **834 ms y un apk de 9555 bytes** frente a 10,1 s con `javac`+`d8`; `apksigner` (v1+v2), `zipalign`,
  `zipcheck`, `aapt2 dump badging` y `apkc verify` lo aprueban, y `d8` releee nuestro dex con `rc 0`.
- **El `if` entró en el subconjunto medido, no adivinado** (§52): javac emite la **condición invertida**
  (`a == b` sale `if_icmpne`, `a > 0` sale `ifle`, `o == null` sale `ifnonnull`) y el desplazamiento se cuenta
  **desde el opcode**; el `goto` solo aparece cuando el `then` no sale por su cuenta; un método de una línea
  lleva **una** fila en la `LineNumberTable`. En dalvik son tres formas de rama (`10t`, `21t`, `22t`), todas
  medidas contra las palabras de `d8`, y el `22s` de los literales. Dos defectos
  reales salieron de la comparación: un `goto` que no encadenaba por encima de una etiqueta vacía (javac escribe
  `13: goto 27`, no `13: goto 20`) y el formato `22s` mal leído. Quedan rechazados por nombre: la rama **hacia
  atrás** (un bucle, cuyo punto de fusión de registros no está medido), `goto/16`, el booleano suelto como
  condición y el `null` a la izquierda.
- **Concatenar cadenas y plegar constantes, medidos contra javac** (§53): en la versión 52 javac pega con
  `StringBuilder` (`new`/`dup`/`<init>`/una fila de `append`/`toString`, sin usar el constructor
  `StringBuilder(String)`), los literales adjacentes se doblan en **una** constante (`"hola" + " " + 3` es un
  solo `ldc "hola 3"`; si no queda otra cosa no hay `StringBuilder`) y las cuentas de literales se doblan
  (`1 + 2 * 3` es `bipush 7`) **salvo con divisor cero**: `1 / 0` se emite entero, porque la excepción tiene
  que saltar en ejecución. Costó dos huecos reales: `areturn` (0xb0) no estaba en la tabla del caminante
  (el dexer paraba en cuanto un método devolvía una cadena) y el paréntesis de `(x + 1)` se leía como molde.
  `d8` reescribe esa cadena (siete instrucciones y quince palabras donde nosotros escribimos diez y veinte),
  y ese es el motivo principal por el que nuestros cuerpos no coinciden con los suyos palabra por palabra.
  Fuera del subconjunto quedan **arreglos y bucles** (y `switch`/`try`); y sigue en pie una forma de `d8`
  para `a*b` entre dos parámetros (dos unidades, `0x0192` + `0x0201`) que no hemos descifrado — nosotros
  emitimos `mul-int/2addr` y `d8` lee el resultado.
- **La app del contador, y por qué el campo no puede ser `private`** (§54): una anónima sobre
  `View$OnClickListener` que escribe `pulsos = pulsos + 1` en el exterior, decide con `if` y arma el texto
  con concatenación — las tres piezas medidas, juntas por primera vez en un `onClick`. javac **vuelve a
  cargar la captura** (`aload_0; getfield this$0` dos veces) en vez de hacer `dup`, y el estático del
  exterior se lee con `getstatic` sin pasar por ella. Si el campo del exterior es **privado**, javac no
  escribe el campo: escribe `access$000`/`access$002` sintéticos del exterior (el de escritura *devuelve* el
  valor, de ahí el `pop`), y como ni el número ni el orden están medidos, ese caso se rechaza por nombre.
- **El teléfono es el oráculo que faltaba, y encontró cuatro defectos** (§55): con `rish` (el shell de Shizuku,
  sin `adb`) el apk del enlace en Go **se instala y la actividad arranca** —`pm install -r` `Success`,
  `am start -W -S` `Status: ok` / `LaunchState: COLD`, `Displayed com.myapp/.MainActivity: +527ms`, sin `FATAL`,
  y `pm compile -f` `Success` con `[status=verify]`—. Antes de eso ART rechazaba el dex con tres avisos que ni
  `d8` ni nuestros tests veían, porque **nadie recorre las áreas de forma contigua ni mira el `map_list`**:
  (1) el `encoded_array_item` es **0x2005**, no 0x2007, y su área no se alinea a 4 —los arrays de `d8` empiezan
  en 2746, 2749, 2755 y 2761—; (2) los `class_data_item` van **pegados**, sin relleno entre ítems —el relleno
  desincronizaba el caminante y salía `Bad index for class_data_item field_idx: 8 >= 6`—; (3) los `proto_ids`
  se ordenan por **tipo de retorno** primero (`Out-of-order proto_id return types`); y (4) el `encoded_value` de
  un estático tiene que ser **del tipo del campo**: un `static int total = 5` sin `ConstantValue` anunciado con
  `0x1e` cortaba con `unexpected static field initial value type: 'L' vs 'I'`. Prueba permanente:
  `apkc/dexlayout_test.go`, que replica las cuatro comprobaciones y corre también **sobre el fichero de `d8`**.
  El oráculo barato: `/apex/com.android.art/bin/dexdump <dex>` empujado con `rish`, un segundo por fichero.
- **El `debug_info_item` está hecho y se ve en el `logcat`** (§56): el formato se midió con una sonda
  propia y `dexdump -d` del aparato, no con un manual. Cuatro cosas: el item **empieza por la línea
  inicial** —el `header_addr` del manual `d8` lo salta, como el `initial_value` del `encoded_field`—;
  el recuento de argumentos **no cuenta `this`**; los nombres van como bytes `00` cuando el `.class` no
  trae `LocalVariableTable`, y **no** se toman de `MethodParameters` (recompilada la sonda con
  `-parameters` el área sale idéntica y lo único que cambia es la anotación
  `Ldalvik/annotation/MethodParameters;`); y la posición se marca con el especial
  `0x0a + 15·Δpc + (Δlínea + 4)`, Δlínea en [-4, +10]. Emisor en `apkc/dexdebug.go`, área `0x2003` detrás
  de `0x2001`, `debug_info_off` parcheado al conocer la base, e items idénticos compartiendo offset (nueve
  cuerpos del fixture → ocho items). Dos de tres métodos salen **idénticos byte a byte** a `d8`. Prueba en
  el teléfono: una variante que divide por cero imprime
  `java.lang.ArithmeticException: divide by zero at com.myapp.MainActivity$1.onClick(MainActivity.java:18)`
  —la línea exacta—; antes las trazas salían sin número.
- **La frame del dex pone los argumentos al final, y eso solo lo comprobaba ART** (§57): el traductor
  copiaba el orden del `.class` (`slot i → v(base+i)`), que coincide con lo que exige el formato solo
  cuando `locales == ins`. Con dos locales de más, nuestro `code_item` decía `registers=10`, `ins=2` y leía
  `this` en `v6`, y el teléfono cortaba con `VerifyError: … instance field access on object that has
  non-reference type Undefined`. Medida la numeración de `d8` en la misma forma (`registers=3`, `ins=2`,
  `this` en `v1`) y corregida con una permutación en `local(n)` que es **identidad** cuando
  `locales == args` —por eso ninguna de las sucesiones ya medidas contra las palabras de `d8` se mueve—.
  Prueba: `TestLaFramePoneLosArgumentosArriba`. En el aparato: la fuente que antes no verificaba ahora da
  `dexdump rc 0`, instala, corre y imprime `ArithmeticException: divide by zero at
  com.myapp.MainActivity$1.onClick(MainActivity.java:21)`.
- **El 2G.3 está cerrado; lo que separaba nuestro fichero del de `d8` eran las anotaciones, medidas en el §58
  y emitidas en el §59**
  (§58): `annotation_item` (`0x2004`, alineación 1) = visibilidad + `type_idx` + `size` + pares
  `name_idx`/`encoded_value`; el `encoded_value` lleva el tipo en los **cinco bits bajos** y la anchura en
  los **tres altos** (`arg + 1` bytes), con `0x1e` null, `0x17` cadena, `0x18` tipo, `0x1a` método y `0x1c`
  array. El `annotation_set_item` **no va ordenado ascendente** (medido `{2623, 2614}`). El directorio
  (`0x2006`) tiene **cuatro `u32` de cabecera** —`class_annotations_off`, `fields_size`,
  `annotated_methods_size`, `annotated_static_fields_size`— y luego las listas pegadas; los dos `u32` más
  que cita el manual no los escribe `d8`, y `annotations_off` de campo/método apunta directo a un
  `annotation_set_item` sin que exista el kind `0x1002`. `d8` emite `InnerClass`, `EnclosingMethod` o
  `EnclosingClass` en las clases y `MethodParameters`/`Signature` en los métodos, y `MethodParameters` solo
  si el `.class` trae el atributo —lo que javac sí pone en el `<init>` sintético de la anónima; nuestro
  emisor también, y **si al emitir las anotaciones el doble oráculo diverge, eso se resuelve midiendo**,
  no ajustando—. El lector ya guarda esos contenidos (`apkc/classreader.go`: `Enclosing`,
  `Parametros`, `Signature`), y el emisor está en el párrafo siguiente.
- **El bloque de anotaciones se emite, y `dexdump -a` ya no distingue el nuestro del de `d8`** (§59):
  `apkc/dexanota.go` escribe las tres áreas (`0x2004` items, `0x1003` sets, `0x2006` directorios) con sus
  cuentas en el `map_list` —17 entradas, las mismas de `d8`—, los items idénticos compartidos (10
  referencias, 8 items) y `annotations_off` cero en quien no lleva nada, como `d8` con `MainActivity`. Reglas
  que dijo el oráculo: **el índice de las listas del directorio es un `u32` entero** (un uleb desfasa el
  fichero dos bytes), `accessFlags` de `@InnerClass` **viaja sin traducir** (25 = los flags del `.class`, no
  `accesoClaseDex`), `@MemberClasses` lleva los tipos miembro **en el orden de las filas del `.class`**, y
  `@EnclosingClass` es para las miembro **con nombre** (la anónima usa `@EnclosingMethod`). Tres defectos
  salieron del camino: leer `EnclosingMethod` como tres `u2` cuando son **dos** (el segundo es una entrada del
  pool con nombre *y* descriptor; de ahí salían una cadena vacía, un proto `()""` y un `method_id` fantasma
  que rompían la identidad byte a byte de las tablas fijas), el `parameters_count` de `MethodParameters`
  leído como `u2` siendo **`u1`** (el array de flags salía `{0}` en vez de `{32784}`) y la conversión de flags
  de `@InnerClass`. Prueba permanente: `apkc/dexanota_test.go` decodifica las tres áreas y exige volcado
  idéntico y los mismos bytes de item contra el fixture de `d8`. En el aparato, sobre el proyecto del
  contador: `dexdump` **verificando** `rc 0`, `d8 --release` re-dexa `rc 0`, y el bloque `dexdump -a` de 21
  líneas sale **igual con `diff` mudo** contra el dex de `d8` hecho desde las clases de `javac` del mismo
  proyecto; `pm install -r` `Success`, arranque en frío y `[status=verify]` en `dexopt`. Signature (genéricos)
  sigue sin emitirse: la troceadura que hace `d8` no está medida y el subconjunto de Java no tiene parámetros
  de tipo.
- **Las dos diferencias que quedaban, resueltas: el `type_list` sobrante era un bug y la marca de `d8` se
  descarta a propósito** (§60): `areaListas` guardaba cada lista con prefijo de origen (`"i|"`/`"p|"`), así que
  dos listas **iguales** se escribían dos veces; `d8` las interna **por contenido** —en el fixture el item
  `[Landroid/view/MenuItem$OnMenuItemClickListener;]` lo apuntan a la vez el proto
  `(L…OnMenuItemClickListener;)Landroid/view/MenuItem;` y el `class_def` de `MainActivity$1`—, y corregido eso el
  área sale **byte a byte igual** (ocho listas, offsets `+0 +8 +16 +28 +36 +44 +52 +60`) y el fichero baja a 2680
  bytes. Medido también que el área no se remata con relleno. La cadena `~~D8{"backend":"dex",…,"sha-1":"…",
  "version":"9.2.4-dev"}` es la firma con la que `d8` nombra su propia herramienta y su versión sobre un digest
  del fichero que él escribió: **no se copia** (sería mentira de origen y de digest), y ninguna columna de las
  tablas la nombra —es la única cadena de más, 53 contra 52—; `TestLaMarcaDeOrigenDeD8NoSeCopia` lo vigila. El
  tercer punto era la política de líneas: dexando las mismas clases en los dos modos, con `d8 --debug` las nueve
  sucesiones de posiciones son **las nuestras**, y con `d8 --release` falta en tres cuerpos la última, que es la
  pegada a la **última instrucción** (la llave de cierre); `apkc` invoca `d8` con `--release` (`build.go`), así que
  `itemDebug` suelta esa posición cuando hay más de una y el fixture queda con **siete `debug_info_item`, los
  mismos que declara el mapa de `d8`**. Comprobado en el aparato con una sonda que falla **en `onCreate`** (no
  necesita ningún toque): la `LineNumberTable` termina en la fila de la llave del método (la 25, pegada al
  `return-void`) y esa es la que ahora se suelta, y la traza dice
  `MainActivity.java:17` —la de la sentencia— **en los dos caminos de la herramienta**, el de `--compiler go`
  (289 ms) y el de `javac`+`d8` (6,39 s). Suite: **214 PASS + 2 SKIP** nativa, **192 PASS + 24 SKIP** en el
  hospedador; gate de `studio/e2e.sh` en orden (el apk del proyecto generado, 10014 bytes, igual entrada por
  entrada al nativo).
- **El oráculo que faltaba era un ejecutor, no un comparador de bytes** (§61): `app_process64` del aparato
  corre un `.dex` suelto con `CLASSPATH` y un `main`, **sin instalar la app** (`base64 -w0 x.dex | rish -c
  'toybox base64 -d > /data/local/tmp/x.dex'; export CLASSPATH=…; /system/bin/app_process64 /data/local/tmp p.Arranque`).
  `banco/banco.sh` ejecuta el mismo fuente por cuatro caminos y compara el **código de salida**: JVM,
  nuestro dex (1008 bytes), `d8` sobre javac y `d8` sobre nuestra `Ver` (1104 bytes, **idénticos entre sí**
  → el doble oráculo sigue entero); los cuatro dan `rc 9` y `dexdump` verifica `rc 0`. Al ejecutar aparecieron
  tres cosas que ningún `cmp` ni `javap` enseñaba: **`invokestatic` de cero argumentos estaba rechazado por
  error** (medido: `d8` escribe `7100 0600 0000`, la forma 35c vale con recuento cero; arreglado en
  `dexinstr.go` con `TestLlamadaSinArgumentosSaleTresCincoceta`), **nuestro `.class` no verificaba en una
  JVM real** porque al emisor le faltaba `StackMapTable` en los destinos de rama (`d8` lo acepta, `java
  -Xverify:all` no; cerrado en el §65), y dos huecos del traductor que el banco esquiva: `getstatic` de una
  **referencia** (`System.out`, plan 5.24) y el **presupuesto de temporales en la unión de un `if` con
  `else`** (plan 5.25, con la diana de `d8` copiada en el §61: `registers 3, ins 0, outs 0`, 20 palabras y el
  pliegue `total - 4` sobre el registro previo, que sale `add-int/lit8 …, #-2`).
- **Ejecutando de verdad salieron tres fallos más, y el banco ya compara el texto** (§62): el fuente de
  prueba imprime con `System.out.println`, así que los cuatro caminos se comparan por **texto y código de
  salida** (`total 7` / `total 9`, `rc 9` en los tres dex, `dexdump` verificando `rc 0`). Lo que apareció:
  **`sget-object`/`sput-object` medidos** (`6200 0000` y `6901 0100`, misma 21c con el hueco del `wide` en
  medio, plan 5.24) y, al usarlos, que **el internado no recorría los estáticos de otra clase**
  (`System.out` nunca entraba en `field_ids`: `collectCode` sólo seguía `getfield`/`putfield`); **cada
  `putstatic` se llevaba un temporal** porque `accesoEstatico` pedía uno con `toma` y luego usaba el que
  sacaba de la pila —con dos en un método el marco se quedaba vacío, y era la causa real que el plan
  llamaba 5.25, no la unión de brazos—; y **la sobrecarga se elegía por nombre y aridad**, así que
  `println(que)` con un String salía `println:(Z)V`: ahora es exacta por descriptores y, si no, la
  aplicable **más específica** por padres e interfaces del jar (JVMS 15.12.2.5, lo que pide
  `Menu.add("…")`), y sin candidata se rechaza por nombre (plan 5.26). De paso, llamar **sin receptor
  desde un `static`** ya se emite como `invokestatic` (5.27). El dex de `d8` sobre nuestra `Ver` y el de
  javac difieren de forma (javac pliega el primer `append` en el constructor `StringBuilder(String)`):
  el banco lo informa y exige sólo que ejecuten igual. Pruebas: `apkc/javaemisor_sobrecarga_test.go`
  (tres, saltan sin jar).
- **Los bucles entraron midiendo el `goto` de vuelta** (§63): `d8` escribe la vuelta con el `goto` 10t y
  el desplazamiento **negativo en el byte alto** (`28f7`, -9 unidades) y la salida con un 22t; el
  traductor rechazaba `rel < 0` «porque el reparto de registros no está medido», y no hace falta
  análisis de viveza: los locales van clavados (`guardaLocal`) y en la cabecera del bucle la pila JVM
  está vacía. Nuestro cuerpo sale con el mismo `28f7` y una instrucción de más donde `d8` pliega
  `i + 1` a `add-int/lit8`. En el emisor, `while` es un nodo propio y `for` **se desdobla** como
  `init; while (cond) { cuerpo; paso }`: el `.class` de los dos sale con el **mismo `Code`** que el de
  javac y el dex **byte a byte** el de javac (`TestElBucleSeDexaIgualQueElDeJavac`, dos fixtures nuevos),
  `TestLaRamaHaciaAtrasSeEscribeComoD8` fija la vuelta en el traductor y `TestDoYTtrySiguenRechazados`
  la frontera (`do`, `try`; `i++` se nombró aquí y entró en el §67). Ejecutado: el dex
  de javac's `Bucle` traducido por nosotros imprime `10`/`rc 10` en `app_process`, el `.class` nuestro
  también, y el banco lleva un `while` (los cuatro caminos: `total 7|total 9|total 42`, `rc 42`, `dexdump
  rc 0`). Ojo al preparar fuentes para el dexer: **javac 21 compila el `+` de cadenas con
  `invokedynamic`**, que este fork rechaza (5.22); por eso el `Arranque` del banco imprime enteros.
- **El `switch`, del `.class` y del fuente, ya está** (§64): `tableswitch` (`0xaa`) y
  `lookupswitch` (`0xab`) se caminan con el relleno hasta el múltiplo de cuatro **contado desde el
  principio del `Code`**, y salen como `packed-switch` (`0x2b`) / `sparse-switch` (`0x2c`) con el
  payload al final del cuerpo, alineado a cuatro bytes: `0x0100`+tamaño+`first_key`+destinos, y
  `0x0200`+tamaño+**todas las claves y después todos los destinos**. Intercalar clave y destino pasa el
  `dexdump` y lo corta ART en ejecución (`invalid target offset 1000 … target index 1`). En DEX el
  «ninguna clave casa» es el pase de lista, así que el `default` se emite como un `goto` detrás del
  switch en vez de reordenar bloques como hace `d8` —que además **rebaja** las switches pequeñas a
  cadenas de `if-eq` en `--release`, y solo en `--debug` deja ver el `sparse-switch`—. Y ART exige
  `ACC_CONSTRUCTOR` en `<clinit>` con el mismo bit que en `<init>` (`TestElClinitLlevaConstructor`).
  Probado con `Ramas` (densa de ocho y dispersa de tres) en los tres caminos: nuestro dex, la JVM y el
  dex de `d8` sobre nuestras clases dan `100/105/999/3/9`, `rc 240`, `dexdump rc 0`. Y **también se lee
  del fuente**: el analizador acepta `switch` con claves literales enteras y brazos que terminan en
  `break` o `return` (la caída al siguiente se rechaza por nombre) y el emisor escribe
  `tableswitch`/`lookupswitch` con el relleno desde el principio del `Code` y destinos de 32 bits desde
  el opcode; el `.class` sale **con el mismo `Code`** que el de javac (el `StackMapTable` llegó en el
  §65) y el dex **byte a byte** el suyo (1432 bytes,
  `TestElSwitchSeEscribeComoJavac`). La elección entre las dos formas es de este emisor (`tableswitch`
  cuando el rango no pasa del doble de los casos): el criterio de javac 21 se sondeó y no es una
  desigualdad sencilla. El banco lleva bucle y `switch`: `total 7|total 9|total 102`, `rc 102`.
- **El `StackMapTable`, y con él la JVM real** (§65): el único oráculo que quedaba sin satisfacer era
  `java -Xverify:all`, porque `d8` acepta un `.class` sin marcos y ART nunca ve un `.class`. Medido con
  `javap -v`: sin ramas no hay atributo; hay marco en cada destino de salto y en la instrucción que sigue
  a un salto incondicional o a un `return`; el `offset_delta` del primero es su pc; con la cabecera de un
  bucle en el pc 0 javac escribe un marco con delta 0; y **`top` gana en la unión** (la vuelta del bucle
  trae un entero al hueco que la entrada ve sin asignar, y la JVM lo acepta: prohíbe usarlo, no que
  llegue). `apkc/jvmframes.go` es un intérprete abstracto sobre el cuerpo **parcheado** que une estados
  (`top` gana; dos referencias distintas → `Object`) y escribe **`full_frame`** siempre, recortando los
  `top` finales —no copia la compresión `same`/`append` de javac porque el verificador compara contenido,
  no forma—; los tipos de campos y métodos salen del pool ya escrito. Exige además que el alto simulado
  no pase del `max_stack`. Oráculos: `TestLosMarcosSonLosDeJavac` decodifica los dos atributos (el nuestro
  y el del fixture de javac) a un estado por pc y exige que digan lo mismo en `while`, las dos formas del
  `switch` y `Cab.f`/`Cab.g` (cabecera en el pc 0, con y sin local previo), y `TestSinRamasNoHayStackMapTable`
  fija la ausencia; el paso 6 del banco ejecuta nuestra `Ver` con el `Arranque` de javac bajo
  `java -Xverify:all` y ahora dice `rc=102 y salida igual a la JVM  ok`. El dex y los apks no cambian
  (el dexer ignora el atributo).
- **Los arreglos, y un `main(String[])` que corre en ART sin javac ni d8** (§66): el `.class` usa
  `newarray`/`anewarray`/`arraylength`/`iaload`/`iastore`/`aaload`/`aastore` y el dex `new-array`
  (0x23, 22c), `array-length` (0x21, 12x), `aget`/`aget-object` (0x44/0x46) y `aput`/`aput-object`
  (0x4b/0x4d), todos 23x con `op|AA` y `BB|CC<<8`. El analizador lee `T[]` en declaraciones y
  parámetros, `new T[n]`, `v[i]` (lectura y destino) y `v.length`; más de una dimensión se rechaza por
  nombre y el inicializador `new T[] {…}` entró después, midido en el §73. El dex de la sonda
  `Arreglos` (diez métodos) es **byte a
  byte** el de javac. Ejecutar destapó dos defectos: **una tabla vacía escribía su desplazamiento en la
  cabecera** (`field_ids_off` tiene que ser 0; ART: `Offset(228) should be zero when size is zero`) y
  **la cuenta pisaba un operando vivo** (`v[i] = i + 1` daba 3 en ART y 5 en la JVM): `d8` usa `/2addr`
  sobre la izquierda, `/2addr` sobre la **derecha** si conmuta (izquierda viva) y la forma de tres
  registros (0x90 suma … 0x94 residuo) si no. También se midió `move-result` entero (`0a00`).
  `TestLaCuentaNoPisaUnOperandoVivo` fija las palabras. Y el banco tiene un quinto camino: las dos
  fuentes compiladas y dexadas solo por este fork (`main(String[])` incluido) corren en `app_process`
  con el mismo texto y `rc` que la JVM (`total 7|total 9|total 102|10`, `rc 102`).
- **`i++` y `--i`, con `iinc` en el `.class`** (§67): medido `iinc` (índice y constante de un byte con
  signo) para el local, `getstatic; iconst_1; iadd; putstatic` para el estático y `aload_0; dup;
  getfield; …; putfield` para el de instancia; el traductor baja `iinc` a `add-int/lit8 vA, vB, #±1`
  (`d800 0001`, `d800 00ff`) y el `dup` del campo desaparece al traducir, así que el dex de nuestra
  `Inc` es **byte a byte** el de javac. El analizador acepta `++`/`--` como tokens (en prefijo, sufijo
  y paso de `for`); como **valor** se rechaza por nombre, y `+=`/`-=` siguen fuera. El banco usa
  `i++` y un `for` con `j++` en los cinco caminos.
- **`+=`/`-=` y `do-while`** (§68 y §69): javac baja `x += lit` a `iinc` cuando el literal cabe en un
  byte y a la lectura-cuenta-escritura cuando no (`*=`, `/=` y deltas grandes); el analizador reescribe
  el caso del literal al nodo del incremento y compone el resto. El `do-while` es el cuerpo primero y
  la condición **directa** saltando atrás (`if_icmplt 2`), al revés que el `while`; `condicionBranca`
  devuelve la forma que salta al no cumplirse, así que se le pasa la comparación negada. Dos filas de
  la `LineNumberTable` que dijo javac: **no hay fila para el `do`** y la condición lleva la del
  `while`. Los dex de `Mas` y `Doo` salen byte a byte los de javac; el banco lleva `do { k++; }
  while (k < n)` y los cinco caminos dan `total 7|total 9|total 102|10|3`.
- **`break` y `continue`, con su destino medido** (§70): `break` sale al fin del bucle o del switch (el
  switch también apila su fin, para el `break` anidado en un `if` de un brazo); `continue` va a la
  cabecera del `while`, a la **condición** del `do-while` y al **paso** del `for` (medido: `goto 21`
  con el `iinc` del paso). Por eso el `for` tiene nodo propio, `jpara`, con el paso fuera del cuerpo;
  el dex de `Corte` sale byte a byte el de javac. De paso se corrigió que `anonimasDe` no recorría
  `if`/`while`/`switch`: una anónima dentro de un `if` no se compilaba.
- **`try`/`catch` y `throw`, y el defecto de locales que destapó ART** (§71): medido que el rango de la
  exception_table es exclusivo, **excluye un `return`/`goto` final e incluye el `athrow`**, que el
  handler empieza en el `astore` y que `catch_type` lleva el nombre **interno** (no el descriptor). En
  el dex: `try_item` + `encoded_catch_handler_list` (con el relleno de una unidad si el cuerpo es
  impar), `move-exception` 0x0d y `throw` 0x27; `d8` omite el `move-exception` si nadie usa la
  excepción. ART cortó el primer intento con `VerifyError … register v2 has type Undefined`: la fusión
  de `istore` (local = registro del valor) no vale con **dos asignaciones en caminos distintos**, y el
  traductor ahora funde solo la primera escritura de un local que no es argumento desde un temporal y
  suelta `move`/`move-object` (`0110`, `0730`) en el resto. Marcos del handler con la excepción en la
  pila y los locales unidos sobre el rango. El banco lleva `leeSeguro` y los cinco caminos dan
  `total 7|total 9|total 102|10|3|2|4|7|-1`.
- **`private`/`protected` y los `access$NNN` de javac** (§72): los flags de un método son `0x0002` y
  `0x0004`, y un privado de la propia clase se llama con `invokespecial`. Una anónima no toca el
  privado del exterior: javac escribe al final de su `ClassFile` (entre los métodos declarados y el
  `<clinit>`) `access$` + posición del miembro × 100 + sentido (`0` leer o llamar, `2` escribir), con
  posición por orden de **primera creación** compartida entre campos y métodos; la escritura se crea
  antes que la lectura del lado derecho y las anónimas se procesan antes que las anidadas con nombre.
  Cuerpos: `iget`, `aload_0; iload_1; dup_x1; putfield; ireturn` (devuelve el valor, el llamador pone
  el `pop`), `getstatic`, `iload_0; dup; putstatic; ireturn`, `invokespecial`/`invokestatic`; flags
  `0x1008` y una fila de línea con la declaración de la clase. En el dex un privado de instancia es
  método **directo** (si no, `dexdump`: `Direct/virtual method … not in expected list`),
  `invokespecial` es `invoke-direct` y el `dup_x1` no emite nada (las tres entradas comparten registro).
  Destapado ejecutando: el `.class` del accesor escribía `max_locals=0` y la JVM lo rechazaba con
  `Arguments can't fit into locals`. Sonda `Q` (campo y métodos privados tocados desde una anónima):
  JVM `-Xverify:all`, nuestro dex en ART y `d8` sobre nuestras clases dan `3`/`6`; el banco lleva
  `Ver.privados` (devuelve `10`) y los cinco caminos coinciden. Frontera por nombre: una clase anidada
  `private` (constructor sintético y clase marcador) o `protected` con nombre y sin `static` (`this$0`).
- **El inicializador `new T[] {…}`, y una clase llamada `B` que se leía como el primitivo** (§73):
  javac **no** usa `filled-new-array` en el `.class` —es `iconst_N`, `newarray`/`anewarray` y por cada
  elemento `dup`, índice, valor y `iastore`/`aastore`, con el arreglo vivo en la pila—; el emisor
  escribe esa cadena y `d8 --release` la compacta a `filled-new-array` + `move-result-object` al dexar
  (medido sobre la misma sonda). Dos cosas salieron del camino: un **estático de tipo arreglo** se lee
  y escribe con `sget-object`/`sput-object` (`6200`/`6900`), no con `sget`/`sput`, y el nombre interno
  de una clase del paquete por defecto **puede ser una letra de primitivo** —`descriptorOf` devolvía
  `B` tal cual y el dex quedaba con `type_id "B"`, que `dexdump` corta con `Invalid descriptor for
  class_idx`—; por eso `descriptorDeClase` (entradas `Class` del pool) envuelve siempre salvo `L`/`[`.
  Sonda `B` (campo estático con inicializador, suma sobre `new int[] {1,2,3}` y `.length` de un
  `String[]`): JVM `-Xverify:all`, nuestro dex en ART (`dexdump rc 0`) y el dex de `d8` sobre nuestras
  clases dan `6`/`9`/`2`; el banco lleva `Ver.inicial` y los cinco caminos coinciden.
- **El `long` en el `.class`, medido hasta el último hueco** (§74): dos huecos de local y **dos palabras
  de pila** (con `max_stack` contado por elementos la JVM corta con `Operand stack overflow` en un
  `<clinit>` que empieza con `ldc2_w`), la entrada `Long` del pool reserva su segunda posición
  (`agregarDos`), `1L`/`0L` salen como `lconst_1`/`lconst_0` y todo lo demás por `ldc2_w` incluso `2L`,
  y el ensanchado va **en el sitio del operando** (`n + m` largo es `iload_0; i2l; lload_1; ladd`), lo
  que obligó a saber el tipo de los dos lados antes de escribirlos (`tipoPrevisto`). Los marcos ponen
  `ITEM_Long` una sola vez por variable y la segunda mitad no se escribe. Nuestro `L` sale con el mismo
  `Code`, `max_stack` y `max_locals` que el de javac y la JVM lo ejecuta (`306`/`8`); el dex todavía lo
  rechaza por nombre, con las formas de `d8` ya medidas (`const-wide/16` 0x16, `mul-long` 0x9d,
  `add-long/2addr` 0xbb, `sget-wide` 0x61, `sput-wide` 0x68, `return-wide` 0x10, `move-result-wide`
  0x0b y argumentos que valen dos registros). `double` no lo arma ni el léxico (`1.0`); siguen sin medir
  `lcmp`, `dup2`/`pop2`, los arreglos de largos y el `ConstantValue` de un `static final long`.
- **El hecho 2 está cerrado sobre el artefacto del navegador** (§55): instalado `pestanera-contador.apk` (el que
  escribe chromium, 10095 bytes; 10169 desde el §56) → `pm install -r` `Success`, `am start -W -S` `Status: ok` / `COLD` /
  `TotalTime: 316`, y dos `input tap` sobre el `com.myapp:id/pulsar` dejan `content-desc="pulsos 2"`; con cuatro,
  `"muchos 4"`. Es decir: corren las dos ramas del `if`, la suma sobre el campo de instancia capturado, el
  estático del exterior, la concatenación de `StringBuilder`, la llamada virtual y el `Toast` (capas
  `Toast#…` en `SurfaceFlinger`), con `FATAL EXCEPTION` **0 veces**. Desde el §59 el `encoded_value` de las
  cadenas, los tipos, los métodos y los arreglos también está medido y emitido: lo único que sigue sin
  medirse es la troceadura de `Signature`.
- **`apkc build --linker go --compiler go` no levanta ningun proceso** (parrafo 44): recursos en Go, Java en
  Go, dex en Go, empaquetado y firma en Go. Nativo en frio: 255 ms y `build/app.apk` de 9868 bytes, que
  `apksigner` (v1 y v2), `zipalign -c -p 4`, `aapt2 dump badging`, `aapt2 dump resources` y `zipcheck.py`
  aprueban. Dentro del hospedador con forma de navegador sale el mismo apk: **cada entrada con los mismos
  bytes** (manifiesto, `classes.dex`, `resources.arsc`, el layout y hasta `META-INF/MANIFEST.MF`). El apk
  **no es reproducible byte a byte** -dos builds nativos seguidos difieren en 301 bytes porque la hora que
  guarda el zip entra en la firma-, asi que la comprobacion que se escribe es entrada por entrada.
- **`apkc compile`: Java en Go** (rama `web`, parrafos 40 a 43). El analizador (`apkc/javafuente.go`) lee el
  subconjunto que necesita la pagina; el emisor (`apkc/javaemisor.go`) escribe el `.class` (version 52):
  resuelve los nombres contra el `android.jar` leyendo solo las entradas que el fuente nombra, dobla las
  constantes `static final int` como javac, sintetiza el `<init>`, escribe `SourceFile`, `InnerClasses`,
  `EnclosingMethod` y `MethodParameters`, y baja una clase anonima a su campo `this$0` con el constructor que
  la captura. `apkc/compilar.go` lo expone como comando, en la lista de los que **no** sondean el SDK.
  **El oraculo es doble y paso en los dos binarios**: `javap -c -p` sobre nuestras seis clases imprime las
  mismas lineas que sobre las de javac 21 (solo difieren los numeros de pool), y `apkc dex` sobre las unas da
  **el mismo fichero byte a byte** que sobre las otras: 2696 = 2696, `cmp` mudo. Dentro del hospedador con
  forma de navegador, con el jar montado en `/sdk/android.jar`, `compile` tarda 1,16 s y sus seis ficheros son
  idénticos a los del binario nativo (§43). Suite actual: **245 PASS + 2 SKIP** nativa y **196 PASS + 51 SKIP**
  en el hospedador: los tests del traductor (branquias incluidas) pasan dentro; los del emisor se saltan si el
  hospedador no lleva `android.jar` montado. El portero
  rechaza con nombre lo no medido: anónima sobre una clase o dentro de otra o en un método `static`,
  tipos de punto flotante y el `long` en el dex (el `.class` ya se emite, párrafo 74), anotaciones que no sean `@Override`.
- **Dos trampas del hospedador con el jar**: `os.Stat` de un fichero montado responde que no existe mientras
  abrirlo funciona (los tests localizan el jar abriendolo, con `sync.Once` para no releer 26 MB, y
  `fuentesJava` lee el fichero en vez de preguntarle al Stat), y el orden
  de argumentos de `navegador-sim.js` es `--montar`/`--exportar` primero, despues el `.wasm` y despues la
  orden.
- Una apk mínima no se puede firmar en el cliente sin `R`: las 4 clases `R*` del recuento las genera
  `aapt2`, así que **2G y 3 van atadas** aunque se midan por separado.
