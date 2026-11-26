# Situación — el recap de lo medido

Movido desde `AGENTS.md` el 2026-11-03, con el texto sin tocar: hasta aquí llegaba el bloque que
`AGENTS.md` ya no admite, porque se leía entero en cada sesión y era la tercera copia de lo medido.

**Qué es.** Un recap, bullet por bullet, de lo que se fue midiendo. El original de cada afirmación es el
párrafo de `docs/evidencia.md` que el bullet cita con su `§`: ahí está el comando que la reproduce.

**Qué no es.** La frontera. Lo que el subconjunto de Java acepta o rechaza hoy, con el `file:line` que
lo rechaza, es la tabla «Inventario» de la Fase 5 de `docs/plan.md`; esa tabla manda sobre este documento.

**Cómo se mantiene.** Aquí no entran bullets nuevos. Una medición se escribe como `§` en
`docs/evidencia.md` y, si cambia algo operativo, actualiza su fila en el inventario del plan. La
excepción es la sección «El estado final, en un recorrido»: no es una medición nueva, es el índice del
producto —qué se hace, en qué orden y con qué comando se comprueba—, y sus cifras son las del `§` que
cita (§202). La lista de bullets de abajo es el histórico y no se reescribe: lo que cambió se anota en
el bullet que lo contaba y se cierra en el `§` correspondiente.

## El estado final, en un recorrido (2026-11-22)

El producto se usa de una sola manera y se comprueba de una sola vez. El gate que lo cierra se lanza
con `PREFIX` propio y puertos libres —nunca un entorno compartido con otro gate—, y estas son sus
líneas reales (medición completa en `docs/evidencia.md` §202):

```sh
PREFIX=$HOME/.cache/apkc-web-clicker-final sh tools/entorno.sh gate
# crear: todo en orden (35 comprobaciones)
# pestaña: 184 comprobaciones, apk firmada dentro de chromium de 10285 bytes y verificada
# e2e: todo en orden
```

Esas cifras son las de `§202` (2026-11-22). El gate posterior sobre el árbol del 2026-11-23 (`§242`)
subió a **36 comprobaciones** en crear y **229** en pestaña; lo que este recorrido describe no cambió,
solo el recuento del gate.

El recorrido, en el orden en que se hace:

1. **Inicio** crea el proyecto en OPFS desde el formulario (nombre, `package` e icono opcional) y
   entra al **Editor** con el `AndroidManifest.xml` abierto; el aviso dice eso mismo, no otra cosa.
2. **Editor**: explorador de OPFS a la izquierda (cerrado al empezar, `build/` marcado como
   generado), pestañas por fichero con carga diferida y undo tree en IndexedDB. Al teclear, el
   panel de **Diagnóstico** pinta lo que dice `apkc check` dentro de la pestaña, y el autocompletado
   ofrece recursos del proyecto y APIs del `android.jar` en `.java`, y etiquetas, atributos y
   valores en `.xml`, cada rama con su contexto y sin cruzarse.
3. **Vista previa** (el panel de la derecha, y a página completa en **Pantalla**): dibuja el
   `res/layout` del editor sin compilar. Con el **lápiz** se enciende el modo editar: paleta de
   cinco tipos, arrastre que escribe márgenes en un marco o reordena en un `LinearLayout`, `Supr`
   para borrar; todo pasa por `EDITOR.cambiar()`, así que entra en el undo tree y el fichero queda
   sucio hasta que se guarda. La vista Pantalla es de solo lectura.
4. **Emulador** (pestaña del mismo panel): al compilar, `apkc preview` devuelve el modelo de la
   pantalla y el panel pinta la app —no una aproximación del XML— con su **registro** (escrituras
   de vista, pasos, avisos) y su **consola** (`println` y `Log.*`, con su secuencia), separadas de
   la consola de los comandos de compilación. Pulsar una vista con `OnClickListener` manda el clic a
   `apkc preview --pulsar` y la pantalla responde («Pulsos: 1»).
5. **Compilar** (el botón Compilar de la barra del fichero abre la pestaña del mismo nombre) escoge la clave (depuración generada en la pestaña o
   keystore de release), compila y firma con `build --linker go --compiler go`, verifica con el
   lector propio y deja `app.apk` enlazado en la barra de descargas; la barra de estado termina
   diciendo el resultado, no se queda en «compilando».
6. **Cerrar y volver**: las pestañas, la carpeta abierta y el ancho del panel sobreviven a la
   recarga; el proyecto vive en OPFS y se exporta e importa como `.zip`.

Lo que ese recorrido **no** hace, con su comando o su `§`: ART y Safari reales en CI (§194), el
host real con truncamiento (§196, §198) y un advisory del detector (contraste calculado; §197)
(`INFORMA-SEGURIDAD.md`). El incremental propio (`docs/plan-incremental-web.md`) ya está activo:
I1 e I2 hechas, I3 cerrada en `d3803bb`, I4 hecha en el camino nativo con el gate pendiente e I5 medida
por debajo del criterio. El `android.jar` propio ya existe y
tiene un run de GitHub Actions en verde (`docs/plan-android-jar-propio.md`, J4 cerrada); **no se
usa para compilar en la página**: la pestaña sigue cargando el `android.jar` de la SDK del usuario
o del servidor (`/android-jar` → `/sdk/android.jar`, 26.361.808 bytes), y el jar generado solo se
ha probado como oráculo y en el autocompletado (J5 local; J5 en sitio pendiente). Fuera del gate
por falta de red: `zipcheck.py` solo si el clon de `apkc-harness` está en `$APKC_HARNESS` —sin él
`e2e.sh` avisa y omite ese oráculo, no lo finge.

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
  No valida la cadena de confianza (sin ancla). EC P-256/P-384, v3/v3.1, v4 y la v1 SHA-1 se
  escriben y verifican después (§121-§126); siguen fuera DSA y los digest que no sean SHA-256.
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
  `13: goto 27`, no `13: goto 20`) y el formato `22s` mal leído. La rama hacia atrás se midió después
  (§63, §69 y §70), así que los bucles ya no están rechazados; siguen fuera `goto/16`, el booleano
  suelto como condición y el `null` a la izquierda.
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
  valor, de ahí el `pop`); el número y el orden se midieron después (§72), así que ese caso ya
  se compila.
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
  `0x0a + 15·dpc + (dlinea + 4)`, dlinea en [-4, +10]. Emisor en `apkc/dexdebug.go`, área `0x2003` detrás
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
  `invokedynamic`**; se desugra en §110-§116, aunque el `Arranque` del banco sigue imprimiendo
  enteros porque así se midió primero.
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
  `break` o `return`; la caída al siguiente se midió después (§129) y el emisor escribe
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
  y paso de `for`); como **valor** se midió en §156 y `+=`/`-=` en §67 y §68. El banco usa
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
  idénticos a los del binario nativo (§43). Suite actual: **336 PASS + 2 SKIP** nativa (342 con subpruebas) y
  **240 PASS + 98 SKIP** en el hospedador (252 con subpruebas): los tests del traductor (branquias incluidas)
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
  del 106, `values-es-rMX` del 107, la firma v3 del 108, las lambdas y referencias del 110 al 115, la
  concatenación dinámica del 116, el keystore de release de la pestaña del 117, la firma JAR del
  `.aab` del 118, la firma v4 del 119, la rotación v3.1 con linaje del 121, las claves EC del 123,
  el `minSdk` del firmante v3 del 124, la v1 SHA-1 del 126, el `.aab` ensamblado en Go del 128,
  la caída del `switch` del 129, el `ConstantValue` ancho y la cuenta 23x del 130, el
  `format="enum"`/`"flags"` del 135, el
  `instanceof` de arreglos del 131, el multi-catch y el `continue` del 132 y las corridas en ART
  del 134, los dos fallos que destapó instalar la app del 136; el `finally` anidado sigue fuera (133)).
- **Dos trampas del hospedador con el jar**: `os.Stat` de un fichero montado responde que no existe mientras
  abrirlo funciona (los tests localizan el jar abriendolo, con `sync.Once` para no releer 26 MB, y
  `fuentesJava` lee el fichero en vez de preguntarle al Stat), y el orden
  de argumentos de `navegador-sim.js` es `--montar`/`--exportar` primero, despues el `.wasm` y despues la
  orden.
- Una apk mínima no se puede firmar en el cliente sin `R`: las 4 clases `R*` del recuento las genera
  `aapt2`, así que **2G y 3 van atadas** aunque se midan por separado.
- **Los tres mundos del editor y el cierre de la cola (§176-§184)**: la página tiene tres pieles
  seleccionables (Taller, Terminal y Azul) con firma propia en el riel y en la pestaña activa; el
  veredicto FIX de la revisión se aplicó y el detector deja 0 warnings. Entraron la instalación PWA,
  el idioma español/inglés, el panel de atajos y Trusted Types con CSP aplicado —los sumideros de
  `Worker` y `URL` solo aparecieron al probar el sitio empaquetado (§183)—. El subconjunto Java sumó
  `switch` de String, `var`, `enum`, `try-with-resources` (sin `addSuppressed`), anidadas con nombre y
  anotaciones con elementos (§179); los recursos sumaron `small`/`large`/`xlarge` y `-vN`, con
  `screenLayout` corregido a +28 (§180); el fuzzing diferencial de 20 casos pasa contra javac y d8
  (§182). El sitio quedó publicado y verificado 10/10 en el host real (§184). Fronteras escritas,
  no cerradas: ART en CI (§178) y linaje/SourceStamp en `verify` (§181).
- **Seis worktrees, una integración y dos agentes caídos (§185-§190)**: el linaje y SourceStamp
  dejaron de ser frontera (§185); los calificadores `uiMode`, touchscreen, teclado y navegación
  corrigen el offset de uiMode a +29 (§186); el `.aab` se ensambla en Go desde el arsc del enlazador,
  sin bundletool (§187); la pestaña gana `check` en vivo, tres plantillas, paleta y contraseñas
  redactadas (§188); el gate de CI añade ART y Safari informativos, bench medible y auditoría de
  seguridad, y cierra el falso verde del job WebKit (§189). De las seis piezas Java pedidas solo se
  conservaron `i2b`/`i2s` y el `ConstantValue` ancho: `addSuppressed`, la anidada privada desde fuera
  y la resolución de sobrecargas ampliada siguen pendientes, y la anónima dentro de anónima se revirtió
  porque su emisión no era la de javac. El gate integrado está en verde con 85 comprobaciones de
  pestaña y las cuatro plantillas verificadas (§190).
- **Cierre de las cuatro fronteras Java y ART en aparato (§192-§193)**: la tableta Blackview MEGA 3
  ejecutó los tres dex con `app_process64` y `dexdump`, con rc y salida idénticos a la JVM, sin
  emulador (§192). En el compilador se cerraron `try-with-resources` con `addSuppressed`, la anidada
  privada instanciada desde fuera, boxing/ensanchamiento/arreglos en sobrecargas y la anónima dentro
  de anónima, todas byte a byte contra javac 17; los varargs quedaron como frontera en §193 y se cerraron en §199. El
  gate integrado sigue en verde con 85 comprobaciones de pestaña.
- **Deuda abierta (§194), con dos puntos cerrados en §199**: varargs en sobrecargas y
  `try-with-resources` con varios recursos y `addSuppressed` ya están medidos y cerrados, igual que
  el diagnóstico/paleta versionado (§197) y el host a 10/10 (§196 y §198). Siguen abiertos los jobs
  `art`/`safari` informativos, un hallazgo del informe de seguridad sin prueba versionada
  (`wasm_exec.js`, §198) y un advisory del detector (contraste calculado; §197). Lo demás no se
  cuenta como cerrado.
- **Punto de entrada, en su día (§198)**: gate en verde con 87 comprobaciones de pestaña, sitio local y
  host real 10/10 por HTTP/2, hash del gz publicado verificado y submódulo alineado. Sirve por sus
  comandos de reproducción, no por sus cifras: el punto de entrada vigente es el recorrido de arriba
  y §202. Sigue mandando §194 (deuda) y el inventario de la Fase 5 de `docs/plan.md`.
- **Vista previa del APK: fases 0-4 cerradas (§201)** (`docs/plan-vista-previa.md`): `apkc preview`
  lee un apk ya construido y devuelve el modelo de su primera pantalla —661 bytes sin ejecutar, 897
  con el intérprete, 3 nodos en la plantilla Contador—, el intérprete corre `onCreate` y una
  interacción (43 pasos, `textoDinamico: "Pulsos: 1"` sobre `id/total`) y rechaza por su nombre un
  dex con el payload de `packed-switch` truncado, y `studio/` pinta el modelo en una tercera vista
  con canvas de solo lectura, vista dividida en el editor, modo «Vista previa» que dibuja el layout
  editado sin enseñar código XML y modo «Emulador» con tres secciones independientes (Pulsar,
  Registro, Consola). El editor de layout permite añadir y arrastrar nodos, editar sus propiedades
  y configurar la raíz; el emulador acepta clics reales tanto del patrón anónimo como de
  `android:onClick` en el XML (§203). Gate en verde con 184 comprobaciones de pestaña sobre el árbol
  reconciliado (105 al cerrar §201, 170 al cerrar §202). El wasm crece +73 884
  B por el lector y +365 037 B por el intérprete. Fronteras: aproximación declarada, sin `gravity`
  fino, imágenes ni tema, ids del framework como `@0x01030042`, e intérprete sin JNI, `.so`,
  multidex ni excepciones. Nada de WineDroid (GPL): es código propio MIT.
- **El intérprete de `preview` se queda sin avisos y el modelo se acerca a la captura del cell
  (§289, §290 y §293)**: dos stubs en `apkc/previewinterprete.go` —`checkSelfPermission` devuelve
  `int32(0)` (`PERMISSION_GRANTED`) e `invalidate` el cero de `V` sin redibujar, porque
  `pintaVistasPropias` (`apkc-emulador/previewdibujo.go:102-127`) corre el `onDraw` una sola vez con una
  instancia nueva— bajan de 1 a 0 los avisos en `chatbt` y `lienzo`, sin mover las cinco capturas
  (`Bluetooth listo - 0 vinculos - 2 mensajes`) ni las 17 operaciones (`fotograma 0`, `toques 0`).
  `--locale es` lee `values-es` y `--locale es-MX` prefiere `values-es-rMX`: `id/saludo` dice `Hola!`
  con flag y `Hello!` sin él, y el contador, sin bloque `es`, cae a la base; solo cambian valores, no
  ids, rutas ni drawables. La captura real a 213 dpi (relleno gris ≈90×40 dp, área blanca 480×776 px)
  destapó que agrandar el botón a 88 dp desplazaba su centro 12 dp: `modeloConMinimos` lo restituye
  (el botón queda en (-12, 28, 88, 40)), con `ALTO_BOTON_DP=40`, `RADIO_BOTON_DP=4`, `clip(marcoRect)`
  para que el quinto botón de `notas` no pinte sobre el editor y el panel como dueño único de
  `#vista-previa`. Frontera: modelo aproximado (sin barra de estado, tema claro), CDP sobre
  `apkc.wasm` local. El selector de idioma de la app en la pestaña quedó cerrado en §326: el control relanza `apkc preview` con `--locale`, el worker transporta ese valor a la sesión en `APKC_SESION_LOCALE` al crear la instancia y la sesión lo conserva en las órdenes sin bandera —la herencia y su falsificación las comprueba el test Go `TestLaSesionHeredaElLocaleDelEntorno`—, y `e2e.sh` falla con `pestaña omitida: falta APKC_WASM` en vez de omitirla.
- **Las diez plantillas compilan y se emulan dentro del gate (§291 y §292)**: `buildProject` de
  `studio/app.js` siembra `hola`, `contador`, `ciclo`, `menu`, `traducido`, `lienzo`, `notas`,
  `chatbt`, `tareas` y `cronometro`, y las diez pasan por `apkc build --linker go --compiler go` y
  `apkc preview` sin `error`, con huellas de `Hello World!` y `Pulsos: 1` a `cuantas:1`,
  `yo: hola`/`eco: hola` y `00:01.00`. Cuatro no salían tal cual —huecos 2 en `chatbt` y `notas`, 3
  en `tareas`, y `lienzo` sin layout para `preview`— y las cinco nuevas no estaban en `e2e.sh`;
  los contratos de emulación los fijaron commits del submódulo (`096aee7` … `b5ff4d9`). Los
  commits `5cfb5b5` y `fb69a51` ampliaron el bucle 0b y añadieron el bloque 0d: el gate siembra,
  compila y emula cada plantilla, `fixture_fresco` recompila si `app.js` es más nuevo que el APK y el
  bloque 0d exige dimensiones positivas, nodos y pasos > 0 y ausencia de `error`. Frontera: `preview`
  es un modelo aproximado, la comprobación es del guion de gate y no de la página en chromium (CDP en
  §293), sin empujar el submódulo ni publicar.
- **Dependencias pasa a vista de primer nivel sobre WISP (§294)**: el commit `3f2b0e9` del submódulo
  saca la sección del panel inferior de Compilar y la deja como `<section id="deps">` hermana de las
  demás, con dos botones `data-vista=deps` y el nombre `deps: "Dependencias"`; la prueba falsifica que
  no quede `data-panel="deps"` ni `mostrarPanel("deps")`, sin ids duplicados y con `sw.js` intacto, y
  en CDP las pestañas salen `["Inicio","Editor","Pantalla","Dependencias"]`. Al resolver,
  `asegurarDecision()` → `resolverPorWisp()` (`studio/deps.js:512-522`) cae al índice local de
  coordenadas si el núcleo WISP no está en la compilación y falla cerrado antes de abrir un socket:
  la falsificación mide `sockets:0, llamadas:0` y el gate de red sigue sin abrir websockets ajenos al
  origen. Frontera: WISP se mide con núcleo simulado en node y con el núcleo incrustado en CDP, nunca
  contra un relay real; sin push del submódulo ni publicación.
- **Hot swap del contador, pendiente (superdado por el bullet siguiente, `677af8e`)**: el `40` no se reproduce; nativo y CDP dan
  `4 -> swap -> 4 -> clic -> 9`. La firma automática sí apunta a la clase exterior, pero derivar
  `MainActivity$1` sigue **pendiente** (plan escrito, sin implementar). No se cuenta como cerrado.
- **Hot swap del contador, sobre §295**: lo que no se reproduce sigue siendo el `40` —es un reporte
  del usuario, sin comando que lo reproduzca y por eso no se registra como medido—; nativo y CDP dan
  `4 → swap → 4 → clic → 9`, y en CDP salen `navegador: todo en orden` y el timeline
  `["Pulsos: 0","1","2","3","4","4","9"]`. La firma automática ya deriva `MainActivity$1.onClick`
  (commit del submódulo `677af8e`), así que lo que el bullet anterior dejaba pendiente quedó
  corregido; el `40` sigue sin contarse como cerrado.
- **Puente `apkcWisp/1` en wasm y proxy página-worker (§295)**: el commit `f841220` pone el global
  `apkcWisp` en el wasm (`apkc/puente_wisp_js.go`, con opt-in y CAs fijadas); el proxy
  `wisp-puente.js` (`d02b2da`) se activa solo con el contrato `apkc-wisp/1`; la validación de la
  respuesta del núcleo (`de92a63`, `wisp.js:294-323`) rechaza cuatro falsificaciones y la forma de
  relay (`2146846`) rechaza tres casos sin dejar de pasar los seis relays del JSON. En CDP el puente
  informa `nucleoDisponible:true` y desaparece el mensaje de que el núcleo no está disponible.
- **Gates del puente (§295)**: `tools/entorno.sh wasm` en verde; la sonda con `apkc-wisp/2` y el relay
  ajeno quedan rechazados con `websockets=0` (log de QA), y el gate de la página da 29/29 `ok` con
  `sockets:0, llamadas:0` sin núcleo (`node prueba-wisp-gates.mjs`). Frontera: gate final, CI y
  publicación quedan fuera; el relay real solo se probó con núcleo simulado, nunca contra un relay
  externo.
- **Arreglo del AXML de los `EditText` (`da9c7de`)**: `apkc/axmlwriter.go` reserva una celda `""` al
  final del pool cuando un atributo `tipoCadena` viene vacío, para que `w.idx("")` apunte a una
  cadena vacía de verdad; antes el `android:text=""` caía en la posición 0 del pool, que en un layout
  es `orientation`, y la siembra del emulador tomaba ese texto prestado, sin mover el orden medido
  del resto del pool. Dos pruebas nuevas (`TestElTextoVacioDelEditText` y
  `TestLosDosEditTextConservanSuIdRef`) reproducen dos `EditText` con idRefs distintos
  (`0x7f010000` y `0x7f010002`) y un `Button` con `@string` «Enviar» de guarda: la siembra deja solo
  el texto del botón, cada idRef publica su texto y dos escrituras llegan a dos capturas distintas.
  Frontera: `e2e.sh` sigue **en verificación** (`docs/plan.md:60`): el gate autorizado sobre el árbol final dio `e2e: 1 problemas` (`FALLO: la pestaña con apkc.wasm`, `253 comprobaciones bien, 3 mal`, sin `TypeError` en el log del gate; §299); dos ejecuciones focales consecutivas del builder del WISP dejaron el 12c en verde con `256 comprobaciones bien, 0 mal`, rc=0, pero el gate completo no se ha repetido. Logs: `~/.cache/apkc-web/tmp/pestana-focal-final.log` y `~/.cache/apkc-web/tmp/pestana-focal-final2.log`.
- §298: puente `wisp-get` de reposo, lienzo con coordenadas y colores reales, pista del `EditText` en `notas` y descripciones de las diez plantillas; suite Go completa pendiente, sin gate final, CI ni publicación.
- §299: el wasm vuelve a compilar (`cff8013` quita el `"net"` huérfano de `apkc/puente_wisp_js.go`; `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build` sin salida, rc 0), la suite Go completa da `ok github.com/ajinasokan/apkc 184.017s` con `tcr -c 3 -m 2G -r 200M` (el guardia por defecto mató el primer intento con 137), el corredor queda centralizado en `tools/entorno.sh` con `tools/correr.sh` (`GUARDIA_MB=300`, `LD_LIBRARY_PATH=…/shimlib`, falsificación WISP rechazada) y 297 `§` se reparten en 14 ficheros con `docs/evidencia.md` como índice. Frontera: el gate autorizado dio `e2e: 1 problemas` (`FALLO: la pestaña con apkc.wasm`, `253 comprobaciones bien, 3 mal`, sin `TypeError` en el log del gate; §299); dos ejecuciones focales consecutivas del builder del WISP dejaron el 12c en verde con `256 comprobaciones bien, 0 mal`, rc=0, pero el gate completo no se ha repetido. Sin CDP final, segundo gate, push, CI ni publicación; `e2e.sh` sigue en verificación (`docs/plan.md:60`).
- §310: ApkShare compila hoy con `--compiler go --linker go`, con y sin `--inc`: 21 clases y dex de `16504 B` idéntico byte a byte en nativo y wasm (SHA-256 `69fdb658…82db9` en los cuatro dex), APK de `305501`/`305500`/`305502 B` y `apksigner verify --verbose` con `Verifies` y v1/v2/v3 `true`; la segunda vuelta con `--inc` tardó `10.935453ms` y dio `the package did not change`. Frontera: la pestaña real con chromium no se re-ejecutó (§285 es transferida), sin suite Go completa ni gate aquí; `TestSondaHeredadaFuente` de §275 sigue sin verificar.
- §312: Go queda fijado en `go1.24.0` (instalado desde el tarball oficial en `$HOME/sdk/go1.24.0`, sin sudo y sin tocar el Go del sistema, que sigue en `go1.27.4-X:nodwarf5`); `wasm_exec.js` es byte-idéntico al del Go local con SHA-256 `0c949f…`, 16992 bytes, y `tools/wasm-exec-hash.sh` valida versión y hash (rc 0; rc 1 con versión ajena o JS modificado). `tools/entorno.sh` antepone el toolchain fijado al PATH (`GOROOT=/home/leonisauro/sdk/go1.24.0`). Frontera: versión fijada, sin segunda pipeline; deudas abiertas y no verdes en `gate.yml:85` (W1 debe sustituir el `cp` directo por el verificador) y `studio/servidor.js` (cuarto sitio que sirve `wasm_exec.js` desde GOROOT); `e2e.sh` sigue en verificación, sin gate, CI ni publicación.
- §317: Apps de ejemplo: firma, alineación y ejecución en Droidloom verificadas. Los cinco APKs
  (`lienzo`, `notas`, `chatbt`, `tareas`, `cronometro`) pasan `apksigner verify --verbose` con
  `Verifies`, v1/v2/v3 `true`, v3.1/v4 y SourceStamp `false`, un firmante, y `zipalign -c -p 4` con
  rc=0 en los diez mandos (salida archivada en
  `~/.cache/apkc-web/apps-ejemplo-medicion/cmd6-salida.txt`); sin recompilar, con build-tools 36.0.0
  y Go 1.24.0 (§312). Droidloom arranca (`status --json` de `running`, `host_pid:178471`), el cell
  sale en `adb devices -l` como `10.177.0.2:5555`, a `1920x1080` y densidad `213`, con las cinco
  apps instaladas y lanzadas con `--user 0`; el workspace 3 queda configurado según
  `droidloom-launch`. Los cinco comandos del QA están en verde: `screencap -p` da PNG
  `1920 x 1080` de `10608 bytes`; `notas` y `tareas` se fuerzan y se arrancan (`pidof` 1707 y 1822),
  `cronometro` arranca (`pidof` 1869); y `dumpsys bluetooth_manager` da `State: OFF`,
  `Bluetooth Service not connected` con `ADAPTER_RC=0` real. Cierre con `droidloomctl stop`
  (`stopped`). Frontera: fidelidad píxel a píxel aparato↔preview sin medir; el adaptador Bluetooth
  está apagado (dato de la medición 5, no fallo); sin gate, chromium ni publicación; `e2e.sh` sigue
  en verificación.
- §313: la vista previa corre como **una sola sesión viva**: `go.argv=["apkc","preview","--sesion",id,apk]`
  y `APKC_SESION_ID` sostienen `apkcSesion.siguiente()` entre órdenes, `setTimeout(...,0)` y un canal
  ceden al event loop, y cada orden publica `apkc:fin-orden` con id y código sin `exit` por orden
  (`os.Exit(2)` solo en el wrapper). Siete tests nuevos con Go 1.24.0 y `-count=1` dan `--- PASS` y
  `ok github.com/ajinasokan/apkc 0.190s`; el timeline medido es
  `["Pulsos: 0","1","2","3","4","4","9"]`, con cinco líneas `onClick` (no nueve), `salir=0` y un
  `fin-orden` por orden. La sonda node da `ordenes: 1,2`, `fin-orden: [1,2]`, `instancias: 1` y
  `timeline: 0,1,2`, y con `--ocupado` falla con `TIMEOUT: sin fin-orden para la orden 2` (rc 1). El
  error pegajoso se corta: `actualizaRes()` limpia `res.Error` (`previewsesion.go:94`) y las dos sendas
  de fallo lo reasignan después (`:205`, `:227`), así que la orden mala publica `error` y la buena
  siguiente no. `TestIndiceDelJarSeReutilizaYSeInvalida` (`jarindice_test.go:151`, 416 frente a 414)
  falla igual en árbol limpio y no es de este cambio. Frontera: sin CDP ni gate; el timeline real en
  navegador y el reclamo del «40» siguen abiertos, no se da por cerrado el `40`.
- §314: medición reproducible del incremental en wasm y nativo por el QA
  `ses_defb50083ffeZC38H50IiYFd2B` sobre `718637c`. Wasm: `--- PASS: TestBuildDosVecesEnWasmMideEtapas (12.66s)`
  y `comparación entrada por entrada: 80 entradas, 0 con bytes distintos []`; cuatro builds con etapas de
  plain frío `aapt2 link 573ms / compile go 1.997s / dex go 814ms / package and sign go 277ms / total 3.782s`,
  plain caliente `108ms / 1.528s / 308ms / 123ms / 2.151s`, con `1866 de 1875 code_item reutilizados` y
  `copied 76 entries`; inc frío `427ms / 1.891s / 641ms / 211ms / 3.297s` con
  `79 resource files changed: relinking`; inc caliente `compile go 1.903s / dex go 356ms / package and sign go 172ms / total 2.591s`,
  con `1866 de 1875 reutilizados`. Sin `apkc devolvio 1`, `FAIL`, `error` ni `panic`. Nativo:
  `r1-plain-frio 993`, `r1-plain-caliente 555`, `r1-inc-frio 808`, `r1-inc-caliente 486`,
  `r1-incremental.json-bytes 35579`; `dex por clase: la vuelta caliente y el dexado completo son identicos`,
  `entradas: 80, con bytes distintos: 0`, `Verifies`, v1/v2/v3 `true` y `zipalign: correcto`, con el árbol
  limpio antes y después. Frontera: el shim no es OPFS; la capa wasm completa sigue con fallos ambientales
  (§279 15/RC=1, §365 0/871/30, `docs/evidencia.md:15-24` 32/38), así que **no** se reclama capa wasm verde,
  solo la prueba focal con jar montado; la fase sigue en verificación hasta el gate final (regla 11); `REPETICIONES=1` deja vacías las medianas de tres repeticiones, que
  es artefacto del guion, no fallo; sin gate final, chromium, red ni commits.
- §318: Fase K re-medida sobre el árbol final (F0): cuatro `--- PASS` (más subtests), `ok
  github.com/ajinasokan/apkc 1.372s`, 0 saltados, con el comando `tcr -c 2 -m 1536M -r 256M -- go
  test -count=1 -run 'TestPreviewRecursosDeAarDrawableAnimYFont|TestPreviewVeElDrawableXmlDeLaDependencia|TestDedupeDeValuesPorClave|TestDedupeDeStyleablesPorClave' -v .`.
  Quedan cerradas, re-medidas, las dos fronteras de §233 y §256/§280: tipos de recurso de §216
  (`drawable` XML, `anim`, `font` y `values`) y claves `values`/`styleable` duplicadas entre AAR.
  Frontera viva: el `R.txt` no se fusiona (`R.java` se regenera del enlace); sin gate, chromium, red
  ni publicación.
- §319: manifiesto del AAR (F1): fusión de hijos de `<manifest>`/`<application>`, prioridad proyecto
  > isla > AAR con `LogW` al colisionar, rechazo explícito de `tools:`, y `R.txt` no fusionado;
  `PASS`, `ok github.com/ajinasokan/apkc 0.764s`, con `apksigner`/`zipalign`/`unzip` presentes y rc 0.
  Frontera honesta: `versionDeExtraccionAar = "e3"` queda **incierto** para I3/I4/I5 —ningún test
  demuestra la invalidación de cachés—; sin gate, chromium, red ni publicación.
- §320: DEPS empaquetado en el sitio (F2): índice con 1 artefacto (205 B, SHA `7dfda8807e76…`),
  `tools/sitio.sh` con 70 ficheros y `VERSION` de `5c81ac704338` a `de190ccf8ee5` con el wasm exacto;
  `prueba-sitio.js` dio `13 comprobaciones bien, 0 mal` (índice 200, artefacto con SHA, cero
  peticiones ajenas) y cuatro falsaciones de traversal (`deps/../../shadow.txt`, `/etc/passwd`,
  barras invertidas y symlink) dieron rc 1 con mensaje explícito. Frontera: DEPS **no publicado**, F3
  pendiente de autorización; `e2e.sh` sigue **en verificación**; sin gate, chromium ni red.
- §348: navegación viva entre actividades en la sesión, ya integrada en el árbol principal en el
  commit `6496918` (`fix(sesion): navega entre actividades y publica el recorrido de pantallas`). El
  test focal con Go 1.24.0 (`cd apkc && env PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH
  GOROOT=/home/leonisauro/sdk/go1.24.0 go test -run TestLaSesionNavegaEntreActividades -count=1 -v .`)
  da `--- PASS: TestLaSesionNavegaEntreActividades (0.02s)` y `ok github.com/ajinasokan/apkc 0.232s`;
  al restaurar `previewsesion.go` a `HEAD` falla con cinco aserciones (la actividad sigue siendo
  `MainActivity`, `pantallas:[MainActivity]`, sin `id/volver`), así que el test sí ejercita el
  arreglo. `apkc/previewsesion.go` gana `pila` y `pantallas` (`:23-28`), `estabilizaSesion()` (`:83`)
  abre `m.pendiente` sin reenviar el guion, `aplicaEvento` (`:164-176`) lo llama tras `entregaEvento`
  y `actualizaRes` (`:188`) publica `res.Pantallas = append([]string(nil), s.pantallas...)`; `finish()`
  con la pila vacía no paniquea. El CDP del builder de D7 (no ejecutado por quien escribe) cerró
  `~/.cache/apkc-web/tmp/cdp-d7.log` con `247 comprobaciones bien, 0 mal` en `:262` y cinco líneas
  `ok … (D7)` en `:127-131`; el total 247 frente a los 258 de corridas anteriores se explica porque
  esta corrida no fijó `KS_RELEASE`, `AAB_EJEMPLO` ni `APKC_WISP_EXTERNO`, y no se oculta. Frontera:
  el único `FAIL` de `go test ./...` es `TestIndiceDelJarSeReutilizaYSeInvalida`
  (`jarindice_test.go:151`, 416 frente a 414), preexistente y reproducido en `HEAD`, no se da por
  verde; `gofmt -l` limpio en los dos ficheros tocados (global lista cuatro preexistentes no
  tocados); CDP sobre el árbol integrado con las variables completas pendiente de autorización
  expresa (el usuario retiró la temperatura como bloqueo, decisión registrada en `ESTADO-SESION.md`);
  sin CDP, chromium, gate ni commits por este `§`.
- §349: las CAs oficiales de `repo1.maven.org` (GTS Root R4 self-signed) y
  `repo.maven.apache.org` (ISRG Root X1, opcionalmente X2) entran como bundle incrustado en
  `studio/cas-wisp.js`: 3494 B, sha256 fijo `c52dbce40b49e83f750f697796b24aee2dd039fe736b75bfb25aa5c18408dade`,
  verificado con `go version go1.24.0 linux/amd64` del toolchain fijado y con `x509.Verify` rc 0 en
  la medición previa (`ses_def6dfe01ffe7a5G21zMMH5eEp`). `wisp.js` lo prepara con la huella fija
  antes de `obtener`, sin `fetch`, sin MIME ni entrada en SW; un `File` local sigue como override y
  el relay nunca aporta CAs. `node --check` limpio y `cd studio && node prueba.js` → «prueba: todo
  en orden», rc 0; la falsación de un byte del bundle se rechaza por huella. Frontera: WISP **sigue
  opt-in y apagado por defecto**; la licencia de las raíces (MPL 2.0 / Mozilla CA Certificate
  Policy) **no verificada localmente**, así que no se publica ni se hace `git push` a `apkc-studio`;
  sin gate, CI, CDP ni red en esta corrida.
- §350: el intérprete sale del padre y vive en el submódulo `apkc-emulador/` sin cambiar una línea
  del compilador ni de la página. El `subtree split` directo falló con
  `could not rev-parse split hash a9092148… from commit ab4fd902…`; con `--ignore-joins` salió
  `aa78f5fbda6dae426eaeb69ad5a24584ea5a17bd`, que se importó sin red y se podó con
  `filter-branch --prune-empty` a **34 ficheros y 59 commits** en la rama `emulador-modulo`
  (el puntero es `563e991 fix(preview): hereda el locale…`). `git rm 'apkc/preview*.go'` y
  `submodule add` con `protocol.file.allow=always` dejaron el gitlink; el puntero de `studio` sigue
  en `8788604081945ed58314d333fa5b06e97eb2b7b3` (`ahead 1`, sin empujar). Actualización (§358, 2026-11-25): el push de estos rangos ya se ejecutó; ver docs/ESTADO-SESION.md y git status -sb para el estado vigente. Compilando los dos
  árboles desde la misma ruta, `cmp` da salida vacía y ambos wasm tienen sha256
  `a00e24effe90aff741a2ae63e5976985ed8dc88880073979bf5b3a707b9c9c43`; el JSON de
  `node shim/navegador-sim.js … preview /app.apk` es idéntico al de antes (`nodos: 3`,
  `pasos: 22`). Frontera: el wasm servido sigue en el checkout principal; la CI con
  `submodules: recursive` fallará hasta que se autorice el push del submódulo y del gitlink;
  deuda registrada, no hecha; sin gate, CDP, red ni publicación.
- §351: el contrato es de fuentes gobernadas, no de import Go (los campos no exportados del
  intérprete lo impiden). `sh tools/materializar-emulador.sh` → «materializados 34 ficheros
  preview*.go» y el bucle `cmp` de los 34 no da salida; seis hooks cubren todos los build
  (`entorno.sh:130-131,135,142,148,170`, `preview-local.sh:22-27`,
  `preview-automatica.sh:32-40`) y nueve guiones de `banco/` compilan con la copia materializada.
  Falsación: un byte cambiado en un clon da `cmp … differ: byte 121, line 4`, `rc=1`; tras
  materializar, el bucle `cmp` da `rc=0`. Frontera: el vigilante en vivo no se corrió en esta
  sesión; su cobertura se demostró leyendo el código y ejecutando el guion, no reconstruyendo el
  servidor.
- §352: la pestaña no cambia. Con los 34 ficheros materializados y Go 1.24.0,
  `go test -run TestPreview -v .` → `ok github.com/ajinasokan/apkc 1.337s` y
  `go test -count=1 ./...` → `ok github.com/ajinasokan/apkc 260.651s`; el wasm se construyó con
  `GOOS=js GOARCH=wasm CGO_ENABLED=0` y el hospedador devolvió el mismo JSON del §350;
  `cd studio && node prueba.js` → «prueba: todo en orden» (`fallos=0`, `prueba.js:6722`), 9/9
  reglas contra salidas reales. Frontera: no se corrieron el gate, `prueba-pestana.js` ni
  `prueba-sitio.js` (CDP), ni `surge`, ni `push`; `studio` sigue `ahead 1` y CI no puede clonar el
  submódulo hasta el push autorizado. Nada se da por verde por analogía (regla 11). Actualización (§358, 2026-11-25): el push de estos rangos ya se ejecutó; ver docs/ESTADO-SESION.md y git status -sb para el estado vigente.
- §354: los tres APK reales de `~/Downloads` (F-Droid, NewPipe y Termux arm; la variante x86_64 solo
  por SHA) arrancan con `pasos:0` y un `ejecucion.error` distinto cada uno — valor estático `0x17` en
  `L_COROUTINE/CoroutineDebuggingKt;`, 4 valores para 8 campos en `SpacerKt` y la cadena #808 que
  declara 18 unidades y decodifica 20—, y pintan el mismo `LinearLayout` de tres nodos con
  `id/action_bar_title`, que **no es** la pantalla de ninguna de las tres apps. Frontera: no se afirma
  que funcionen; la causa byte a byte de la cadena #808 sigue `incierto`, y la identidad de la
  lanzadora de F-Droid es hipótesis. Sostenido por `§354`.
- §355: las tres causas del lector de `apkc/` quedan arregladas con tuplas byte a byte de los APK:
  valor `0x17` little-endian (`_COROUTINE`), cola de `SpacerKt` rellena con `VALUE_NULL` y sustituto
  suelto `ed a0 80` convertido en un solo `U+FFFD`; el test focal y la suite pasan y el escritor de
  dex da el mismo SHA antes y después. Con el binario reconstruido los tres `preview` avanzan de
  `pasos:0` a 292/286/207 y pierden sus errores de `§354`. Frontera: **no verde**: aparece un error
  nuevo compartido, `ArchTaskExecutor.getInstance usa el opcode 0x1d, que el intérprete MVP no
  decodifica`, y los layouts siguen siendo falsos; el bloqueo es del intérprete, no del lector. Sin
  gate, CDP, red ni publicación. Sostenido por `§355`.
- §366: cuarta APK real (`com.martinmimigames.simplefileexplorer_20.apk`, sha256 `fc6c9134…`,
   38.042 B), Java llano con un solo `classes.dex`, sin `.so` y sin opcode `0x1d`, con la actividad
   lanzadora (`MainActivity`) y el layout real `res/R5.xml` de 30 nodos. El `preview` del binario
   `f241c54b…` (`vcs.revision=f59717b2c6c82f154332b7fd1958c8dd8faa8844`, `vcs.modified=false`) da `pasos:162`,
   12 avisos y `ejecucion.error` («se llamó a java.lang.Object.getClass sobre un objeto nulo»);
   `ejecucion.capturas` (`Grant`/`Deny`) son textos de `TextView.setText`, no capturas visuales.
   Frontera: línea base pre-corrección y anterior a Fase D; no se afirma que la app funciona y no se
   fusiona con `§361`. Sin gate, CDP, red ni publicación. Sostenido por `§366`.
 - §367: quinta APK real (`org.quentin_bettoum.librememorygame_9.apk`, sha256 `df049866…`,
    64.773.062 B), Godot 4.5.1 con la lógica en `libgodot_android.so` y dos ABI nativas. El `preview`
    del binario `04456d27…` (`vcs.modified=false`) da `rc=0`, `pasos:333`, `nodos=2`,
    `layout=res/layout/custom_dialog.xml` (`RelativeLayout` + `Button id/dialog_button` con texto
    `' Ok '`) y un `ejecucion.error` con `Thread.getStackTrace sobre un objeto nulo`, con 5 avisos de
    la familia «no está en el dex ni en los stubs» y sin `id/action_bar_title`. Clasificación
    fail-closed: **solo envoltorio, sin lógica Godot**; no se cuenta como app cubierta ni sustituye a
    las cuatro anteriores. Índice en `docs/evidencia.md`; medición en
    `docs/evidencia/15-apps-reales.md`. Sin gate, CDP, red ni publicación. Sostenido por `§367`.
- §359: Fase C (`98cedf1`, original `7fb6857`): `enabled` tipado con `tipoBooleano`, alias con
  `targetActivity`, prioridad `MAIN`+`LAUNCHER`+`DEFAULT`, orden `pedido` → `layoutID` →
  `activity_<Nombre>` y centinela `sin-layout-xml:` sin fallback alfabético. Siete focales en verde y
  suite `460.631s`; F-Droid da `org.fdroid.MainActivity`, `layout=''`, un `FrameLayout` con texto
  «F-Droid» y cero `IconCalculatorActivity`/`res/A1.xml`; el gestor da `res/R5.xml` de 30 nodos.
  Original en `docs/evidencia/15-apps-reales.md`; volcados en `~/.cache/apkc-web/fase-c-evidencia/`
  (709/118 líneas) y JSON en `~/.cache/apkc-web/fase-c/`. Sin CDP ni publicación.
- §360: Fase B (`28c22de`, original `9527958`): `dexDelApk` enumera `classes.dex`,
  `classes2.dex`, … en orden numérico y rechaza duplicados; `ejecutaOnCreateConEventos(dxs, …)`
  comprueba el descriptor del manifiesto **y** `met.dex == dx[1]`, sin campo JSON nuevo. Cuatro
  focales en verde y suite `210.695s`; `dexdump` sitúa las actividades en `classes2.dex` y
  `classes20.dex`. JSON en `~/.cache/apkc-web/fase-b-evidencia/`; `docs/evidencia/15-apps-reales.md`.
  `Error==""` y capturas reales quedan para D y C.
- §361: Fase D (`376def6`, original `6fa1d34`): 12 stubs (`getAction` → `android.intent.action.MAIN`,
  `getClass` → `Class` sintético, `Dialog.setContentView` sin tocar `m.layoutID`, guardián de nulo
  intacto). En el cuarto APK, avisos de la familia 12 → 0, `pasos` 162 → 1131, `error=None`,
  `res/R5.xml` de 30 nodos y SHA `f207c043…` → `28a6e68a…`; suite `438.144s`. Otros tres sin
  regresión (13/8/3 avisos). JSON en `~/.cache/apkc-web/fase-d-evidencia/preview-post.json`;
  `docs/evidencia/15-apps-reales.md`.
- §362: Fase E (`77dc8e5`/`a2a6c29`, originales `1c22cd2`/`4c5f514`, y studio `4d4dbd9`): tema por
  id de atributo (`textColorPrimary`, `colorBackground`, `windowBackground`), sin campos nuevos;
  filas vía `expandeListas` (`preview.go:1019-1071`), test negativo
  `TestElModeloNoInventaColorSinTema`, suite `505.205s` con `tcr -r 256M` y `node prueba.js` →
  «prueba: todo en orden». Comparación visual y publicación pendientes (Fase G).
  `docs/evidencia/15-apps-reales.md`.
- §368: integración de C `98cedf1`, B `28c22de`, D `376def6`, E1 `77dc8e5` y E2 `a2a6c29` en el
  emulador y `4d4dbd9`+`6cf42c1` en studio, con los cinco frentes conservados. Suite integrada
  `go test -count=1 ./...` → `ok … 196.940s`, rc 0, 0 `FAIL`. Cinco JSON en
  `~/.cache/apkc-web/integracion/`: F-Droid `org.fdroid.MainActivity`/`layout=''`/centinela,
  NewPipe `res/v9.xml`/6 nodos, Termux `TermuxActivity`/`activity_termux.xml`/13 nodos, gestor
  `error=None`/0 avisos de familia/`res/R5.xml`/30 nodos y Libre Memory Game mismo error,
  `layout=''`, 1 nodo, 6 avisos y marco «Libre Memory Game». Hecho en capa Go/node, en verificación
  CDP/gate; ninguna APK se da por funcionando. `docs/evidencia/15-apps-reales.md`.
- §369: ROM de un solo usuario con permisos por prefijo (`03b772d`/`49a5726`, en el lote `6a207cd`):
  sobre `/storage/emulated/0`, `id/list` del gestor gana 2 filas solo con
  `--permiso android.permission.READ_EXTERNAL_STORAGE`, no con el clic; SHA `28a6e68a…` →
  `e603739d…` → `88eb9f85…`; `checkSelfPermission` → `-1` y `MANAGE_EXTERNAL_STORAGE` denegado.
  Los hijos llegan de `id/drive_list` como aproximación documentada. Sin CDP ni publicación.
  `docs/evidencia/15-apps-reales.md`.
- §370: lote integrado de cuatro frentes sobre `c448442` (`99a4c3b`, `a7683be`, `157eab5`,
  `9c5ca8d`, `1fbaaa1`, `5b591ba`, `c3f1e55`, `6a207cd`; integración `1e1bb03`): suite
  `go test -count=1 ./...` → `ok … 257.828s`; cinco APK, base → lote, 468→700/12→4 en F-Droid,
  379→508/8→3 en NewPipe, 223→284 en Termux con denegación de la ROM, 1131→1137 con `error=None`
  en el gestor y 333→547 en Libre Memory Game. Ninguna app se da por funcionando.
  `docs/evidencia/15-apps-reales.md`.
- §371: colecciones, cadenas y trazas (`5676be4`/`8ef0b12`/`ec842d4`): 13 focales en verde; avisos
  integrados → deuda, F-Droid 4→4 (`forName` queda), NewPipe 3→3, Libre Memory Game 6→4, Termux
  7→6 y gestor 1→1, con `layout`/`nodos` idénticos en las cinco. `copyOfRange` cotejado con java 17
  (`2 > 1` y `arraycopy: length -2 is negative`). Sin campo JSON nuevo ni CDP.
  `docs/evidencia/15-apps-reales.md`.
- §372: recursos y preferencias de Termux (`af72d22`, `5558e02`, `4bdebeb`, `7162fdc`, `6366291`,
  **ya integrados en el checkout principal con `688a7ad`**, pin `a83ebcd…`): 223 → 64 → 408 →
  503 → 719 → **3798** pasos como registro fechado y **4784** en la corrida final
  (`~/.cache/apkc-web/integracion-final/termux.json`); desaparecen `Set.iterator` de propiedades,
  `containsKey`/`contains`, conjuntos estáticos nulos e `ImmutableBiMap.entrySet`; la frontera
  nueva es `Set.iterator` en `TerminalColorScheme.updateWith`. Actualización (esquema de colores de
  Termux, commit `84a7b90`, pin `bf7477e…`): **6599 pasos** y frontera `setTerminalToolbarView` sobre
  `ViewGroup$LayoutParams.height` nulo, sin `Set.iterator`.
  Tres APK byte-idénticas en el registro fechado y gestor solo con el aviso `preferencias-memoria:`;
  cero peticiones fuera del origen pendiente para Fase G. `docs/evidencia/15-apps-reales.md`.
- §373: relay WISP propio como opción precargada en `#wisp-url`, sin activar el módulo: DNS
  `163.192.151.24` (`getent hosts wisp.dolci-vita.link`), puertos 80/443 abiertos y persistidos en
  la VM pero `curl` → `Connection refused`; studio `ffcc49a`, `64eac8a` (guarda en `relayActual`,
  falsado: sin él `prueba.js` devolvía la URL apagada) y `deda61c`, padre `18a564b`, `d15cf3c` y
  `682c6ec`; aceptación por igualdad exacta con rechazo de variantes canónicas. `#wisp-selector`
  sigue `hidden`, los seis relays de terceros siguen en `RelaysWISP()` y no hay campo nuevo en el
  JSON del emulador. `sh tools/correr.sh wisp` rc 0, `wisp-externo` sin variable rc 2,
  `cd studio && node prueba.js` → «prueba: todo en orden», test
  `TestRelayPropioAceptadoPorComparacionExacta` → PASS y suite `205.713s`. El relay Go de
  `tools/wispd/` estaba aprobado al medir, sin petición externa con el módulo apagado, sin CDP, gate
  externo ni publicación. Actualización del 2026-11-25: el relay sí se desplegó y se verificó en
  remoto (`§374`).
  `docs/evidencia/15-apps-reales.md`.
- §374: primer relay WISP propio desplegado y verificado en remoto (nota fechada del 2026-11-25):
  VM Oracle Ubuntu 22.04.5 en `163.192.151.24`, 80/443 persistidos con iptables, binario `~/wispd`
  con SHA `0c7f334b…` desde `tools/wispd/` (commits `a7ac8d2`, `a836010`, `3cc6ee7`), certificado
  de Let's Encrypt para `wisp.dolci-vita.link` hasta `2027-02-23` con `certbot.timer` activo y
  `wispd.service` `active`/`enabled` (`MemoryMax=640M`, capacidades conservadas,
  `debug.SetMemoryLimit(512<<20)` como único mecanismo de memoria). Siete resultados Wisp v1:
  handshake de 9 bytes, dos orígenes rechazados con `403`, tres CONNECT cerrados (`0x41`, `0x41`,
  `0x48`) y `repo1.maven.org:443` sin `CLOSE` en 6 s; literal IP con `0x41` aceptado como
  comportamiento válido. Módulo WISP apagado y sin integrar en la navegación, con
  `prueba-sitio.js`/CDP pendientes de autorización y sin gate con variable externa ni publicación.
  `docs/evidencia/15-apps-reales.md`.
- §375: lote Boolean/gestor (cherry-picks `f33bfdb`, `7213e25`, `6d84fcf` y `2a42a84`, integrado en
  `c179a0b`, pin de entonces `2a42a84…`): cuatro piezas nuevas en el intérprete con la suite en verde
  (`162.890s`, 11 focales PASS, 34 ficheros byte-idénticos y `diff --check` rc 0). En el gestor, sin
  clic 1137→1330 pasos y 30→33 nodos con `Loading...` y `Access Denied`; las sondas de clic cambian
  de actividad hasta `Download` y muestran `fotos`, pero todavía no `leeme.txt`. F-Droid, NewPipe y
  Libre Memory Game siguen byte-idénticos a `integracion-color/` y Termux sin regresión (6599 pasos).
  Sin CDP, gate ni publicación; el gestor no se da por funcionando. `docs/evidencia/15-apps-reales.md`.
- §376: filas reales del gestor desde la ROM sembrada (cherry-pick `0c01986…` del commit `fd1e5ac`
  sobre `2a42a84`, integrado en `69e798c`, pin de entonces `0c01986d356c6eeb20a20809e5b96c74468d34ae`; pin vigente `c0e60ca39ee746abb5350fe4d6533de683d7467f` (`9a6ec572fc72dcb374e692d094b627b1960c8efc` queda como registro histórico fechado)):
  con `--rom studio/apkc-emulador/rom.json`, `--permiso android.permission.READ_EXTERNAL_STORAGE` y
  tres pulsos (`id/menu_quick_switch`, `id/drive_list:0`, `id/list:4`), 3596 pasos, `error=None`,
  `layout=res/R5.xml` e `id/list` con `fotos` y `leeme.txt`, frente a 1330/33 sin pulsos; suite
  `197.304s`, 12 focales PASS y 34 ficheros byte-idénticos. Las cuatro APK de control siguen
  byte-idénticas a `integracion-gestor-boolean/`. Sin CDP, gate ni publicación; el gestor no se da por
  funcionando y el icono/extensión (`String.lastIndexOf`) queda sin resolver.
  `docs/evidencia/15-apps-reales.md`.
- §377: LayoutParams reales y frontera `bindService` en Termux (cherry-pick
  `c54e55114568b05947170a879f989e550b6766ee` del commit `0c17130` sobre `0c01986`, integrado en
  `acc7d44`, pin de entonces `c54e551…`; pin vigente `9a6ec57…`): sobre el árbol final, 6944 pasos, 25 avisos
  (`avisos-recortados: 40`), `layout=res/layout/activity_termux.xml` y 13 nodos, con
  `com.termux.app.TermuxActivity.onCreate: java.lang.RuntimeException: bindService() failed` en lugar
  del `LayoutParams.height` nulo de `setTerminalToolbarView` (6599 pasos en `§372`); suite `196.481s`,
  17 focales PASS y 34 ficheros byte-idénticos. El aviso de `getLayoutParams` está recortado y no
  aparece entre los 25 visibles, aunque el cambio de error confirma que el stub se ejecutó. Las otras
  cuatro APK siguen byte-idénticas a `integracion-filas/`. Sin CDP, gate ni publicación; Termux no se
  da por funcionando. `docs/evidencia/15-apps-reales.md`.
- §356: la propiedad git de `apkc-emulador` pasa al submódulo de `studio/` (gitlink `563e991`,
  commit `0f6d802`, URL https y rama `emulador-modulo`); el padre conserva su pin espejo y una
  guardia prohíbe que divergan. Un único resolvedor elige la fuente (`$2`, `APKC_EMULADOR_DIR`,
  `studio/apkc-emulador`, `apkc-emulador`), así que los nueve guiones de `banco/` no cambian y el
  `cmp` de 34 ficheros sigue intacto. Suite Go en verde (248,739 s), wasm con el mismo JSON
  (`pasos: 22`, sin `ejecucion.error`) y `node prueba.js` → «prueba: todo en orden». El sitio
  empaquetado no lleva `*.go`, `.gitmodules` ni el directorio del submódulo, y el wasm servido es
  byte a byte el compilado (`1c2f521b…`); los cinco checkouts de CI siguen con
  `submodules: recursive`. Frontera: `studio` queda `ahead 3` sin empujar, la CI remota no verá el
  gitlink anidado hasta que se autorice el push, el vigilante en vivo no se corrió (`incierto`) y
  no hubo gate, CDP, red ni publicación. Sostenido por `§356`. Actualización (2026-11-25): el push
  ya se ejecutó y el vigilante en vivo está medido (`head-construido` = HEAD, wasm servido
  byte-idéntico, COOP/COEP y procesos vivos), con lo que esa frontera histórica de `§356` quedó
  superada; no se reescribe la medición de `§352` ni la de `§356`.
- §357: el usuario acota que solo existe un emulador, el de `studio/apkc-emulador`, y se retira el
   pin espejo del padre (`git rm -f apkc-emulador` y su sección de `.gitmodules`). La regla 14 del
   `AGENTS.md` ya no compara dos gitlinks: vigila solo `git -C studio rev-parse HEAD:apkc-emulador`
   → `563e991`. El resolvedor `materializar-emulador.sh` pierde la caída al pin del padre (quedan
   argumento, `APKC_EMULADOR_DIR` y `studio/apkc-emulador`), con lo que los nueve guiones de
   `banco/` heredan la fuente única sin editarse; `preview-automatica.sh` reintenta en vez de caer
   al padre. 34 `cmp` idénticos, suite Go en verde (264,994 s), wasm con `pasos: 22` y sin
   `ejecucion.error`, y `node prueba.js` → «prueba: todo en orden»; `studio` sigue limpio, sin
   gitlink nuevo. Frontera: sin gate, CDP, red ni publicación; el vigilante en vivo sigue
   `incierto`. Sostenido por `§357`. Actualización (§358): el emulador subió a `4117a8f` y actualizó su
   gitlink en el mismo commit, así que la guardia de la regla 14 espera ahora `4117a8f` en vez de
   `563e991`, sin reescribir lo medido en `§357`.
- **Traspaso para una sesión limpia (2026-11-24)**: `docs/ESTADO-SESION.md` recoge el estado verificado
  de los tres repos (padre `65578d9`, studio `e30d3ad`, emulador `563e991`), lo hecho y verificado en
  su capa, lo que sigue en verificación (CDP, `prueba-pestana.js`, `prueba-sitio.js`, `e2e.sh`, gate y
  publicación), el frente en marcha del pin único del emulador, la vista previa viva en
  `http://127.0.0.1:8801/` (`head-construido` = HEAD, wasm servido `1d27617a…`) y las decisiones del
  usuario para no perderlas. `docs/plan-compatibilidad-apps-pausado.md` traslada el plan de apps reales
  pausado: §354 y §355 ya están medidos, Fase A no se rehace y las Fases B-G se reanudan en `§358+`.
  Ninguno de los dos ficheros añade una medición nueva: apuntan a los `§` citados, que siguen siendo
  la fuente.

### F0 (Compose), cierre

El criterio de abandono de `docs/plan-fase-f-compose.md:31` exige dos intentos sin `compose:Text:` con texto verificado en `resources.arsc`: se cumplieron cuatro frentes (`f0-transfer`, `f0-creada`, `f0-setadd`, `f0-compose`) con cero avisos `compose:Text:`. La causa es doble y está mapeada: `previewcompose.go:106-113` elige la actividad barriendo clases en busca de `onCreate` con `setContent$default` y no consulta el manifiesto (F-Droid declara `org.fdroid.MainActivity` con `MAIN`/`LEANBACK_LAUNCHER` y **tres** alias `LAUNCHER` — `org.fdroid.IconCalculatorActivity`, `org.fdroid.IconActivity` y `org.fdroid.fdroid.panic.CalculatorActivity`, los tres con `targetActivity="org.fdroid.MainActivity"`; medido con `apkanalyzer manifest print` y `aapt2 dump xmltree --file AndroidManifest.xml` sobre `~/Downloads/F-Droid.apk`); y aun alcanzando `Wrapper_androidKt.setContent` (`:183`) y `abreComposable` (`:210`), el cuerpo no llega a `stringResource`/`Text` (`:277-304`), únicos que alimentan `publicaTextoCompose` (`:68-77`). No se integra código Compose en `HEAD`: el enganche y `previewcompose.go` existen sin commitear en el worktree `f0-composer`, y ninguna ejecución posterior ha producido un aviso `compose:Text:` con texto verificado; el criterio de abandono sigue en rojo. Lo aprovechado son `and-int/lit8`/`System.arraycopy` (`e0f2f8a`) y `Set.add` (`4b0af9e`), auditados como correcciones generales; el error `Element with name 'type' is already registered in androidx.navigation.runtime.NavKey` **se midió con la sonda `DEPURACION` en `f0-instrumenta`** (`~/.cache/apkc-web/f0-instrumenta/logs/iter2-salida.txt:13-14`), mientras `java.util.Set.add` seguía «no está en el dex ni en los stubs». **Su desaparición con `Set.add` queda `incierto`: no hay medición con la misma sonda posterior al cambio** (los dos binarios instrumentados conservados siguen sin `Set.add` y siguen dando el error; los JSON posteriores de `f0-reanuda` dan otros errores y no llevan la sonda), pero el criterio F0 sigue en rojo. El riesgo es que forzar el aviso sin texto resuelto sería pantalla inventada; la única vía futura coherente es desenvolver el `Composer` (pertenencia de lambdas, `rememberedValue`/`changedInstance`/`skipToGroupEnd`) con `TestComposeSetContentPublicaTexto` como oráculo. El frente queda documentado como no probado, sin afirmar que la app funciona, con esa vía concreta indicada.
