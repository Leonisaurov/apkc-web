# Situación — el recap de lo medido

Movido desde `AGENTS.md` el 2026-11-03, con el texto sin tocar: hasta aquí llegaba el bloque que
`AGENTS.md` ya no admite, porque se leía entero en cada sesión y era la tercera copia de lo medido.

**Qué es.** Un recap, bullet por bullet, de lo que se fue midiendo. El original de cada afirmación es el
párrafo de `docs/evidencia.md` que el bullet cita con su `§`: ahí está el comando que la reproduce.

**Qué no es.** La frontera. Lo que el subconjunto de Java acepta o rechaza hoy, con el `file:line` que
lo rechaza, es la tabla «Inventario» de la Fase 5 de `docs/plan.md`; esa tabla manda sobre este documento.

**Cómo se mantiene.** Aquí no entran bullets nuevos. Una medición se escribe como `§` en
`docs/evidencia.md` y, si cambia algo operativo, actualiza su fila en el inventario del plan.

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
  entiende. El enlace acepta además PNG en `res/drawable`/`res/mipmap` (§93: el fichero viaja literal y
  `aapt2 dump resources` declara la entrada) y `values-xx` de dos letras (§94: dos sondas byte a byte
  contra aapt2, con `spec_map` 0x0004 y el offset ausente). El portero rechaza con nombre lo no medido:
  calificadores de layout/densidad/región (`res/layout-land`, `drawable-hdpi`, `values-es-rMX`),
  recursos que no sean values/layout/menu/xml/drawable/mipmap/raw, subcarpetas, no-XML en los XML,
  `<permission>` y una versión de platform no medida.
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
  `Code`, `max_stack` y `max_locals` que el de javac y la JVM lo ejecuta (`306`/`8`). **Y el dex también**:
  `const-wide/16` 0x16, `const-wide/32` 0x17 y `const-wide` 0x18, `lload`/`lstore` como `move-wide` 0x04 con
  **par de registros**, la familia 23x `0x9b..0x9f` y `/2addr` `0xbb..0xbf` con la misma regla de viveza del
  entero, `int-to-long` 0x81 / `long-to-int` 0x84, `iget-wide` 0x53 / `iput-wide` 0x5a / `sget-wide` 0x61 /
  `sput-wide` 0x68, `return-wide` 0x10 con `move-result-wide` 0x0b, y los argumentos largos que valen dos
  registros (`(JJ)J` sale `registers=6, ins=4`). Nuestro dex sobre nuestras clases es **byte a byte** el
  mismo que sobre las de javac (1104 bytes, `cmp` mudo) y ART lo ejecuta: `306` y `8`. Tres defectos salieron
  al ejecutar: el `encoded_value` de un `static long` debe anunciar `valueLong` con 8 bytes, el
  `parameters_count` del `debug_info_item` va por **parámetros declarados** y no por palabras, y una clase
  llamada `L` chocaba con el `L` de un descriptor. `double` no lo arma ni el léxico (`1.0`); el `lcmp` y las
  comparaciones de NaN entraron en el §81 y `dup2`/`dup2_x1`/`pop2` en el §83; siguen sin medir los arreglos de
  largos y el `ConstantValue` de un `static final long`.
- **El `finally` y los dos `catch`, con la copia en cada camino** (§75): javac no escribe subrutina —copia el
  bloque al pie de cada camino que puede caer (el normal y el de cada `catch`) y añade un último manejador
  `any` que guarda la excepción en un local, copia el bloque y hace `aload`+`athrow`—. Las filas de la tabla
  salen en ese orden (tipos del cuerpo, `any` del cuerpo, `any` de cada cuerpo del `catch`), y dos rangos que
  van al mismo manejador con el mismo tipo **se mezclan** (`Fin3.soloLanzaConCatch` protege `0..11`). Cada
  `catch` declara su excepción **en el mismo hueco** y el temporal del `any` va en el **más alto que vio el
  método** (`astore_1` los dos brazos de `multi`, `astore_3` el `any` de un `try` anidado). Las filas de
  línea: la copia lleva la línea de la primera sentencia del `finally` y el `goto`/re-lanza que la sigue, la de
  su llave; la entrada del `any` se anota con la línea de la sentencia, no con la del `catch`. Sin `finally`,
  el `goto` del `try/catch` lleva la línea de la llave que cierra el último `catch` (regla que faltaba del
  §71). Un `finally` que lanza no deja `goto`, y en un `void` javac tampoco escribe el `return` implícito; un
  `finally` vacío se suelta entero. En el dex no hizo falta escribir nada: `armaTries` ya agrupaba por rango
  y ya ponía el `catch_all` con el tamaño negativo. El dex de la sonda (siete métodos) sale **byte a byte** el
  de javac (1452) y el banco lleva `finalmente(5)` y `finalmente(0)`: los cinco caminos y la JVM con nuestra
  clase dan `…|306|22|7` con `rc 102`. Frontera por nombre: el valor pendiente de un `return`/`break`/`continue`
  que sale de un `try` con `finally` (medido en `Fin2`, escrito en el §76), el `throw` como última sentencia del cuerpo con
  `finally` (el rango de javac se pasa a su propio manejador) y `catch (A | B e)`.
- **El `float` y el `double` en el `.class`** (§77): la frontera era el **léxico** —partía `2.5` en `2`, `.` y
  `5`—, no el emisor. Los *tags* del pool se midieron en los bytes de la sonda (`FyD.class`): `Float` es **tag 4**
  con cuatro bytes y **una** posición; `Double` **tag 6** con ocho y **dos** (la `Long` del §74 es 5). Un
  `float` ocupa un hueco y una palabra (`resta(float a, float b)` es `fload_0; fload_1`), el `double` dos de cada
  (`suma` es `dload_0; dload_2`): el `esAncho` del §74 contaba `F` como ancho y estaba mal. Cuentas
  `fadd` 0x62..`frem` 0x72 y `dadd` 0x63..`drem` 0x73; la conversión va **en el sitio del operando** (`por` es
  `fload_0; f2d; dload_1; dmul`, `divide(double,int)` es `dload_0; iload_2; i2d; ddiv`) con la promoción
  doble > flotante > largo > entero, así que `tipoPrevisto` devuelve el tipo promocionado. `0.0f` es
  `fconst_0`, `1.0` es `dconst_1`, lo demás por pool (`ldc` / `ldc2_w`) y `-2.5` se pliega en la constante;
  `freturn` 0xae, `dreturn` 0xaf; la rejilla de conversiones es `i2f` 0x86, `i2d` 0x87, `l2f` 0x89, `l2d` 0x8a,
  `f2i` 0x8b, `f2l` 0x8c, `f2d` 0x8d, `d2i` 0x8e, `d2l` 0x8f, `d2f` 0x90 y un cast numérico **no** es
  `checkcast`. Marcos: `ITEM_Float` 2, `ITEM_Double` 3, una entrada por los dos huecos. Los trece métodos de la
  sonda salen con las mismas instrucciones, `max_stack`, huecos y filas de línea que `javac --release 8`, y
  **nuestra clase corre en la JVM con `-Xverify:all`** dando el mismo texto que la de javac (`36.39159` y
  `1.5|2.5`, `rc 0`). Frontera por nombre entonces: el traductor se paraba en `dload_0` (las formas de dalvik
  entraron midiendo `d8` en el §80),
  `faload`/`fastore`, las comparaciones sobre flotantes o dobles, `i2b`/`i2c`/`i2s` y el exponente con
  signo. La prueba del caminante cambió de byte (0x6a ya es `fmul`; es 0x94, `lcmp`).
- **El punto flotante en el dex, medido con `d8` como oráculo sin aparato** (§80): sin `rish` no hay
  `dexdump`, pero `d8 --debug` escribe el dex y `readDex` corta las palabras sin interpretarlas. Fijado:
  **`instance-of` 0x20** (en el §79), conversiones = la rejilla del `.class` menos cuatro (`int-to-double`
  0x83, `float-to-double` 0x89, `double-to-int` 0x8A, `double-to-float` 0x8C, ancladas en `int-to-long` 0x81
  y `long-to-int` 0x84 del §74), `/2addr` de `float` 0xC6..0xCA y de `double` 0xCB..0xCF (cinco posiciones
  cada bloque, porque no hay bit a bit sobre flotantes), `0.0f` es `const/4 v0,#0`, `1.0` y `-2.5` caben en
  **`const-wide/high16` 0x19** (dos unidades) y `3.14159` va en `const-wide` 0x18. El `float` usa las formas
  de una palabra (`sget`/`iget`/`move`/`return`) y el `double` las anchas del §74. Con eso el dex de `FyD`
  (trece métodos) sale **byte a byte** igual escrito sobre nuestras clases que sobre las de javac. Quedan
  rechazadas con nombre: la forma de **tres registros** de una cuenta ancha de flotante (ninguna sonda la
  escribió), `faload`/`fastore`, `aget-wide`/`aput-wide`, `l2f`/`l2d`/`f2l`/`d2l` (inferidas del mismo paso,
  sin sonda). La comprobación en ART llegó en el §82 (con `sudo` en vez de `rish`): el banco ejecuta `flotantes`
  y `dobleCon` por los cinco caminos.
- **El banco en ART después del reset, y los tres defectos que solo ve el aparato** (§82): el aparato se alcanza
  con `sudo` (`banco.sh` elige `rish` o `sudo` con `enAparato`) y la sonda del banco ya lleva `float`, `double`,
  dos `instanceof` y un `return-wide` leído por `println(D)V`; los cinco caminos dan `…|101|8|3.0` con `rc 102`.
  Los tres defectos, invisibles para `d8` y para la identidad byte a byte: (1) el `encoded_value` de un estático
  `double`/`float` sin `ConstantValue` salía `valueNull` y ART cortaba con `unexpected static field initial value
  type: 'L' vs 'D'` — ahora lleva el cero del tipo (`valueDouble|7<<5`, `valueFloat|3<<5`) y la constante real
  cuando el `.class` trae `ConstantValue`; (2) `collectCode` no seguía `instanceof` ni `checkcast`, así que la
  clase de un `instanceof CharSequence` que no se nombraba en otro sitio faltaba de `type_ids` y la instrucción
  salía `instance-of v5, v4, I` (`ART: instance-of on unexpected class Integer`) — ahora se internan y `typeIdxDe`
  rechaza con nombre cualquier tipo ausente en las cuatro instrucciones que lo llevan; (3) el reparto de pares
  anchos agotaba el marco (`it needs more wide temporaries…` y luego `register v16 does not fit a four-bit
  format`) — `tomaPar` busca el par consecutivo **más bajo** de la bolsa, la pasada de medida no tiene techo y la
  base se busca por **punto fijo**. De paso entraron `lload` 0x16 y `lstore` 0x37 (forma indexada, la que javac
  escribe con el local > 3). Tests: `javaemisor_ancho_test.go` (cuatro) y `TestRechazaLoQueNoEstaMedido` con
  `techo: true`.
- **Las comparaciones anchas, del fuente y del dex** (§81): javac mantiene la inversión del §52 y emite el
  `cmp` con el sufijo que pide la relación —`…l` para `>`, `>=`, `==`, `!=` y `…g` para `<`, `<=`, la regla
  de NaN del JVMS 2.9— dejando un entero que consume la branquia de siempre; el literal cero **no** se
  ahorra en las anchas (`if (x > 0L)` es `lload_0; lconst_0; lcmp; ifle`). En el dex son el bloque 0x2d..0x31
  en 23x (`fcmpl` 0x2d, `fcmpg` 0x2e, `dcmpl` 0x2f, `dcmpg` 0x30, `lcmp` 0x31) con la `if-*z` encima. El
  emisor promociona los dos lados con `expA` y el traductor pide un temporal para el resultado (las dos
  parejas pueden seguir vivas por un `dup`). La sonda `Ram` (ocho métodos) sale con el mismo `Code` que
  javac y su dex byte a byte el suyo (1192 bytes); ejecutado en ART por el camino «todo nuestro» da
  `1 2 1 2 …` como la JVM, y el banco lleva `Ver.compara(2)` (6). `javaemisor_compara_test.go` (dos) fija el
  par `cmp`+branquia de cada método.
- **El reparto de temporales se reintenta** (§81): la base se mide con una conjetura holgada y se escribe con
  la base = pico; si esa pasada pide más (visto con `Ver.compara`), la conjetura sube por encima del pico
  pedido y se repite el par entero. `esPresupuesto` distingue el fallo de marco (se reintenta) de un opcode o
  un tipo sin medir (se devuelve), y la conjetura crece en cada vuelta, así que el bucle termina.
- **Los campos anchos privados y su `access$`** (§83): javac baja `largo = largo + 1` en una anónima a
  `access$000` (leer: `getfield` + `lreturn`) y `access$002` (escribir: `aload_0; lload_1; dup2_x1; putfield;
  lreturn`), y el llamador tira el valor con `pop2`; el estático ancho usa `dup2`. En el dex, `dup2_x1` y el
  `dup2` **no emiten nada** (las entradas comparten registros) y el `pop2` se absorbe con la llamada, igual que
  el `pop` del §21; el `iput-wide` 0x5a ya estaba. Cuatro defectos salieron al ejecutarlo: `max_locals` del
  accesor contaba argumentos y no **huecos** (`(LD;J)J` pide 3 y quedaba 2, con `aload_0` en v-1), el número
  del `access$` se contaba en todo el proyecto y no por exterior (`D`+`E` daban `access$200` en la segunda), el
  `prof` del accesor no descontaba las dos palabras del `return` ancho, y la ordenación de `proto_ids` usaba el
  shorty: ART pide la **lista de parámetros por `type_idx`** y cortaba con `Out-of-order proto_id arguments`
  (`(LD;J)J` va antes que `(LF;)J` aunque sus shortys sean «JLJ» y «JL»). Ejecutado por los cinco caminos del
  banco (`anchoPrivado` = 7) y por la JVM con nuestra clase (`-Xverify:all`). Tests:
  `javaemisor_privada_ancha_test.go` (dos) y `dexprotos_test.go`.
- **Los arreglos de `long`, `float` y `double`** (§84): en el `.class` son `laload` 0x2f / `faload` 0x30 /
  `daload` 0x31 y `lastore` 0x50 / `fastore` 0x51 / `dastore` 0x52; en el dex, `aget-wide` **0x45** y
  `aput-wide` **0x4c** para los anchos y `aget`/`aput` para el `float`, con el par como destino/valor.
  Implementados caminante, marcos, traductor y emisor (`new T[n]`, `v[i]`, `v[i] = x`, `new T[] {…}`), y el
  camino «todo nuestro» ejecuta en ART `7 9 3 7 5 9`, como la JVM. **Defecto que solo vio ART**: la pila
  guarda solo el registro bajo de un par, así que la mitad alta se liberaba con el par vivo y `tomaPar`
  creaba pares solapados (`wide register v7 has type Double (Low Half)`); se cerró con `altos` (mitad alta
  protegida), `sueltaPar` que espera a que el par muera y `ocupado` en `toma`/`tomaPar`. Las tres cosas que
  quedaron pendientes entonces están cerradas: el plegado del literal promocionado y el `baja(3)` del
  inicializador ancho en el §85, el marco por encima de v15 y las formas `/from16` en el §86, con el test de
  la sonda (`javaemisor_arreglos_anchos_test.go`, `.class` y dex de 1496 bytes byte a byte) y el caso
  `arreglosAnchos(2)` en el banco.
- **El `instanceof` y el oráculo de `d8` sin aparato** (§79): en el `.class` es `aload_0; instanceof #7; 4: ifeq 9`
  —el índice ocupa **dos bytes** (0xc1) y la condición va **sin invertir**, al revés que las seis comparaciones
  del §52—, consume la referencia y deja un entero (el alto de pila no cambia, el marco del `else` es `same`).
  En el dex, **`instance-of` es 0x20 en formato 22c**: medido sobre el fichero que escribe `d8 --debug` y
  decodificando sus palabras con el lector `readDex` (el oráculo no comparte código con el traductor: solo
  corta el `code_item`), porque sin `rish` no hay `dexdump`. `Ins2.f(Object a, Object b)` con
  `if (b instanceof CharSequence)` sale `4128 2` = `instance-of v0, v1, type@2` ✓, lo que fija el orden de los
  nibbles y obliga a pedir un **temporal** para el destino (la referencia probada puede seguir viva). Los cinco
  métodos de `Ins` salen iguales que javac (instrucciones, `max_stack`, huecos, filas de línea), el dex que
  escribimos sobre nuestras clases es **byte a byte** el de sobre las de javac, y la JVM con `-Xverify:all`
  ejecuta nuestra clase con el mismo texto: `1|0|2|3|2|cadena|5`. La comprobación en ART llegó en el §82 (con
  el banco por `sudo`): `instance-of` corre y da el resultado esperado, y de paso destapó que la clase del
  `instanceof` no se internaba; `x instanceof T[]` y la forma negada siguen sin medirse.
- **El valor pendiente: un `return` o un `break` que cruza el `finally`** (§76): el cuerpo deja de ser un rango
  único —cada salida abrupta **cierra un trozo** cuyo fin es el pc justo detrás del `istore` que guarda el valor, y
  el trozo siguiente empieza **después** de la copia, del `iload` y del `return` (o del `goto` del `break`): medido
  en `Fin5.rama` (`0..6 → 23 any` y `12..16 → 23 any`), `Fin5.breakConFinally` (`6..11` y `18..22`) y
  `Fin6.ramaVoid` (`0..4` y `10..14`), donde la copia y el `return` intermedios quedan **sin proteger** (si no, el
  `finally` correría dos veces). El valor espera en un hueco sin nombre; un `return;` de `void` no usa hueco y
  escribe la copia antes de marcar la línea de la sentencia. El `goto` de un `break` que cruza lleva la línea del
  `break`, no la de la llave. Rechazados por nombre, con la forma medida: el `continue` que sale, un
  `return`/`break` que cruza **otro `try` anidado** (`Fin6.anidado` solapa rangos `0..6` y `0..11`, y en el dex los
  `try_item` no pueden solaparse), uno dentro del propio `finally`, el cuerpo que no deja nada protegido (javac
  escribe el manejador **muerto** y la tabla sin filas: `Fin6.soloVoid`) y el cuerpo que sale por dos trozos con
  `catch`. En el dex no se tocó nada: `armaTries` agrupa por `(inicio, fin)` y cada grupo lleva su lista. El dex de
  la sonda ampliada (doce métodos) sale **byte a byte** el de javac (2052) y el banco lleva `salidaPendiente(5)` y
  `salidaPendiente(0)`: los cinco caminos y la JVM con nuestra clase dan `…|22|7|10|101` con `rc 102`.
- **El hecho 2 está cerrado sobre el artefacto del navegador** (§55): instalado `pestanera-contador.apk` (el que
  escribe chromium, 10095 bytes; 10169 desde el §56) → `pm install -r` `Success`, `am start -W -S` `Status: ok` / `COLD` /
  `TotalTime: 316`, y dos `input tap` sobre el `com.myapp:id/pulsar` dejan `content-desc="pulsos 2"`; con cuatro,
  `"muchos 4"`. Es decir: corren las dos ramas del `if`, la suma sobre el campo de instancia capturado, el
  estático del exterior, la concatenación de `StringBuilder`, la llamada virtual y el `Toast` (capas
  `Toast#…` en `SurfaceFlinger`), con `FATAL EXCEPTION` **0 veces**. Desde el §59 el `encoded_value` de las
  cadenas, los tipos, los métodos y los arreglos también está medido y emitido: lo único que sigue sin
  medirse es la troceadura de `Signature`.
- **`apkc build --linker go --compiler go` no levanta ningún proceso** (párrafo 44): recursos en Go, Java en
  Go, dex en Go, empaquetado y firma en Go. Nativo en frío: 255 ms y `build/app.apk` de 9868 bytes, que
  `apksigner` (v1 y v2), `zipalign -c -p 4`, `aapt2 dump badging`, `aapt2 dump resources` y `zipcheck.py`
  aprueban. Dentro del hospedador con forma de navegador sale el mismo apk: **cada entrada con los mismos
  bytes** (manifiesto, `classes.dex`, `resources.arsc`, el layout y hasta `META-INF/MANIFEST.MF`). El apk
  **no es reproducible byte a byte** -dos builds nativos seguidos difieren en 301 bytes porque la hora que
  guarda el zip entra en la firma-, así que la comprobación que se escribe es entrada por entrada.
- **`apkc compile`: Java en Go** (rama `web`, párrafos 40 a 43). El analizador (`apkc/javafuente.go`) lee el
  subconjunto que necesita la página; el emisor (`apkc/javaemisor.go`) escribe el `.class` (versión 52):
  resuelve los nombres contra el `android.jar` leyendo solo las entradas que el fuente nombra, dobla las
  constantes `static final int` como javac, sintetiza el `<init>`, escribe `SourceFile`, `InnerClasses`,
  `EnclosingMethod` y `MethodParameters`, y baja una clase anonima a su campo `this$0` con el constructor que
  la captura. `apkc/compilar.go` lo expone como comando, en la lista de los que **no** sondean el SDK.
  **El oraculo es doble y paso en los dos binarios**: `javap -c -p` sobre nuestras seis clases imprime las
  mismas lineas que sobre las de javac 21 (solo difieren los numeros de pool), y `apkc dex` sobre las unas da
  **el mismo fichero byte a byte** que sobre las otras: 2696 = 2696, `cmp` mudo. Dentro del hospedador con
  forma de navegador, con el jar montado en `/sdk/android.jar`, `compile` tarda 1,16 s y sus seis ficheros son
  idénticos a los del binario nativo (§43). Suite actual: **307 PASS + 2 SKIP** nativa (313 con subpruebas) y
  **221 PASS + 88 SKIP** en el hospedador (227 con subpruebas): los tests del traductor (branquias incluidas)
  pasan dentro; los del emisor se saltan si el hospedador no lleva `android.jar` montado. El portero
  rechaza con nombre lo no medido: anónima sobre una clase o dentro de otra o en un método `static`,
  una condición que sea un booleano suelto y anotaciones que no sean `@Override` (el `long`
  entero —`.class` y dex— está desde el párrafo 74; el `float` y el `double` en el `.class` (77), en el dex
  (80), en las comparaciones (81) y en los arreglos (84); el plegado del literal promocionado y los
  inicializadores anchos son del 85, el marco por encima de v15 con las formas `/from16` del 86, el
  `char` —léxico, `caload`/`castore`, `i2c`, `aget-char`/`aput-char` y `sget-char`/`sput-char`— del 87,
  el literal `X.class` con `const-class` 0x1c y la igualdad de referencias con `if_acmp*` del 89, las
  llamadas de más de cinco palabras con `invoke-*/range` del 90, los constructores propios con
  `super`/`this`, sus inicializadores y los dos `code_item` de una sobrecarga del 91, `throws`,
  interfaces propias, abstractos e `invokeinterface` del 92, el PNG de `drawable`/`mipmap` del 93,
  `values-xx` del 94, `string-array`/`integer-array`/`plurals` del 96, `menu`/`xml`/`raw` del 97, la
  anónima en `static` y la captura de locales del 99 y 100, `Signature` del 101, el `Manifest.java` de
  `<permission>` del 102, `layout-land`/`drawable-hdpi` del 103, `fraction` del 105, `attr`/`styleable`
  del 106, `values-es-rMX` del 107, la firma v3 del 108 y las lambdas y referencias del 110 al 115).
- **Dos trampas del hospedador con el jar**: `os.Stat` de un fichero montado responde que no existe mientras
  abrirlo funciona (los tests localizan el jar abriendolo, con `sync.Once` para no releer 26 MB, y
  `fuentesJava` lee el fichero en vez de preguntarle al Stat), y el orden
  de argumentos de `navegador-sim.js` es `--montar`/`--exportar` primero, despues el `.wasm` y despues la
  orden.
- Una apk mínima no se puede firmar en el cliente sin `R`: las 4 clases `R*` del recuento las genera
  `aapt2`, así que **2G y 3 van atadas** aunque se midan por separado.
