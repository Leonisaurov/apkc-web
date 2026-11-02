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
| `banco/` | **el banco de ejecución**: dos fuentes de prueba y un guion que corre el mismo programa por los cuatro caminos (JVM, `d8` sobre javac, `d8` sobre lo que emitimos, y **nuestro dex en `app_process`**) — sin instalar nada, sin pantalla | `sh banco/banco.sh` (necesita `APKC` compilado, `ANDROID_JAR`, `build-tools` y `rish`) |

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
  Termux ejecuta **nuestros** `.class`: es el oráculo que destapó el `StackMapTable` que falta, §61) y
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
  `dexinstr.go` con `TestLlamadaSinArgumentosSaleTresCincoceta`), **nuestro `.class` no verifica en una JVM
  real** porque al emisor le falta `StackMapTable` en los destinos de rama (`d8` lo acepta, `java
  -Xverify:all` no: plan 5.23), y dos huecos del traductor que el banco esquiva: `getstatic` de una
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
  idénticos a los del binario nativo (§43). Suite actual: **219 PASS + 2 SKIP** nativa y **194 PASS + 27 SKIP**
  en el hospedador: los tests del traductor (branquias incluidas) pasan dentro; los del emisor se saltan si el
  hospedador no lleva `android.jar` montado. El portero
  rechaza con nombre lo no medido: anónima sobre una clase o dentro de otra o en un método `static`,
  tipos de dos huecos, anotaciones que no sean `@Override`.
- **Dos trampas del hospedador con el jar**: `os.Stat` de un fichero montado responde que no existe mientras
  abrirlo funciona (los tests localizan el jar abriendolo, con `sync.Once` para no releer 26 MB, y
  `fuentesJava` lee el fichero en vez de preguntarle al Stat), y el orden
  de argumentos de `navegador-sim.js` es `--montar`/`--exportar` primero, despues el `.wasm` y despues la
  orden.
- Una apk mínima no se puede firmar en el cliente sin `R`: las 4 clases `R*` del recuento las genera
  `aapt2`, así que **2G y 3 van atadas** aunque se midan por separado.
