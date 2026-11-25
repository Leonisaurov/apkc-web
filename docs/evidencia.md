# Evidencia

Todo lo de aquí se midió en este teléfono (Termux/Android aarch64, Chromium y node 26 locales).
Las mediciones del 2026-10-26 se hicieron contra `../apkc`; desde 2026-10-27 el código de la
herramienta es un fork en `apkc/` de este repo (rama `web`), así que los `cd ../apkc` que aparecen
abajo son el recordatorio de cómo se obtuvo cada número, no la ruta de hoy.
Cada bloque lleva el comando que lo produce. Fecha de todas las mediciones: **2026-10-26**.

**Numeración de este registro.** Los encabezados van del `§1` al `§372` (342 entradas en el índice), con los bloques `§334-339` y `§346-347` incorporados fuera de orden; la secuencia tiene un
hueco y un desorden históricos: **no existe un `§78`** y, en la zona del 77 al 81, el `§80` se escribió
después del `§81`; junto a los 212-215, el `§213` se escribió después del `§215`. Las cabeceras se
dejan como se publicaron para no romper las citas cruzadas; el número del encabezado es la etiqueta de
la entrada, no su orden en el fichero.

**Estado de la capa `wasm` (2026-11-23).**
**Bloque histórico:** el estado medido de este árbol está en `§365`; lo que sigue se conserva tal como se publicó.
Las cifras `PASS` y `rc 0` de los primeros `§` —`§1`
(88 tests, `:47`), `§8` (`:217`, 98) y el recuento de `:444-445` (105)— **están desfasadas**: son el
registro de cuando se midieron, no el estado de este árbol. Se dejan como se publicaron. Lo medido
hoy por el QA de la capa `wasm`, con `TMPDIR=… GOTMPDIR=… tcr -- sh tools/entorno.sh wasm`, es que
la compilación a wasm pasa y **la ejecución en el hospedador simulado falla**: su salida, resumida,
da 32 tests y 38 líneas `--- FAIL`, sin `rc 0`. El shim no propaga el código de salida —hubo que
matar `node` con SIGTERM— y otro frente lo corrige; no se da por cerrado. Las causas son
ambientales y preexistentes al último cambio: 25 por «sin android.jar que abrir: exporta
APKC_ANDROID_JAR» (`apkc/dependencias_test.go:32`, el guard de `8d6ee67`; el fs simulado no tiene
`/android`), 5 por `pipe: not implemented on js`, 2 por salida de hijo vacía, 1 por
`testdata/vista-propia/main.xml` sin montar, 1 por permisos `644` y 1 por
`apkc/wisp_endurecimiento_test.go:283` (bajo `GOOS=js`, `AbrirWebSocketWISP` sí funciona y el test
espera que falle). Ese desglose del informe del QA suma 35 entradas y no cuadra con las 38 líneas
`--- FAIL` anunciadas ni con los 32 tests: discrepancia **sin resolver**, no se da por cuadrada
hasta repetir la capa con la salida completa guardada. Los 8 tests nuevos de `45ec24a`, `445f7fb` y
`f064e5f` **no aparecen entre los fallos**: pasaron en el hospedador. Se reproduce con

```sh
sed -n '47p;217p;444,445p' docs/evidencia.md
grep -n 'APKC_ANDROID_JAR' apkc/dependencias_test.go
sed -n '280,286p' apkc/wisp_endurecimiento_test.go
git log --oneline -1 8d6ee67
TMPDIR=… GOTMPDIR=… tcr -- sh tools/entorno.sh wasm
```

## Índice de mediciones

| § | Medición | Fichero |
| --- | --- | --- |
| §1 | El código puro de apkc corre en un hospedador con forma de navegador | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §2 | `apkc create` entero, del lado del cliente | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §3 | Lo que se rompió al montar el hospedador, y por qué no era de apkc | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §4 | El límite duro: `build` no puede existir del lado del cliente hoy | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §5 | Fricción del entorno (Termux), para no perder tiempo dos veces | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §6 | Cuánto cuesta hoy cada etapa (para saber qué merecería la pena portar) | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §7 | `apkc sign`: la firma como comando suelto, con los tres oráculos (apkc `a909214`) | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §8 | La firma dentro de wasm, en el hospedador con forma de navegador | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §9 | Dos reglas de formato que se leyeron de un oráculo en vez de adivinarse | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §10 | Un fallo del hospedador que parecía de apkc | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §11 | El hospedador de navegador DE VERDAD: chromium, OPFS y worker (no node) | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §12 | Cuatro cosas del navegador que se rompieron antes de funcionar | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §13 | Cuánto hay que emitir: el `classes.dex` de un proyecto real, despiezado | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §14 | El subconjunto que hay que emitir, enumerado con `javap` (Fase 2G.1 y 1G.1) | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §15 | La pestaña no habla con ningún sitio que no sea su propio origen | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §16 | El lector de `classes.dex`, con el dex de `d8` como fixture (Fase 2G, primera pieza) | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §17 | La tabla de opcodes JVM, medida contra `javap` en vez de recordada | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §18 | Otro fallo del hospedador, este lo destapó el timing | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §19 | Las tablas de índices que `d8` deriva de las mismas clases: tres reglas que se aprendieron midiendo | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §20 | La cabecera del `code_item`, palabra por palabra, casada con `javap` y con el `map_list` | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §21 | Las palabras dalvik, una por una: qué formato tiene cada instrucción y con qué criterio lo eligió `d8` | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §22 | El `classes.dex` completo, escrito en Go: cuatro cosas que se arreglaron leyendo los bytes, y un oráculo que lo acepta | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §23 | `apkc dex`: la etapa, como comando, corriendo dentro de wasm | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §24 | Fase 3 medida antes de escribir un byte: qué produce `aapt2` sobre el proyecto del studio | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §25 | Las clases del disco real producen el mismo dex que las fijadas en base64 | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §26 | Lector Go del AXML y de la tabla de recursos, con los ficheros de `aapt2` de fixture | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §27 | El valor complejo de un estilo, medido con variantes; y la discrepancia que abrí y se cerró | [02-dexer-y-formato.md](evidencia/02-dexer-y-formato.md) |
| §28 | El escritor de tablas de cadenas de recursos, byte a byte contra `aapt2`, y lo que separa | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §29 | El cuerpo del `resources.arsc`, escrito en Go: byte a byte con `aapt2` salvo donde aapt2 abre | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §30 | El AXML del layout y el del manifiesto, escritos en Go: byte a byte con `aapt2` en los dos | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §31 | La `R` sale de nuestra propia tabla, y el texto es el mismo que escribe `aapt2 link --java` | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §32 | Un apk hecho enteramente en Go: firma v1 y v2, alineado, y `aapt2 dump badging` lo entiende | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §33 | La tabla de ids del framework, leída de `android.jar` con nuestro propio lector | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §34 | Cómo define el framework un atributo: los `enum` y `flags` se guardan como recursos de tipo `id` | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §35 | Del `res/values/styles.xml` en texto a la misma tabla y a la misma `R` | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §36 | El árbol en texto: las líneas que anota aapt2 no son las que da el decodificador de Go | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §37 | Enlazado: de los tres ficheros de texto a los tres binarios de aapt2, byte a byte | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §38 | `apkc build --linker go`: un apk real sin ejecutar aapt2, y sus productos comparados | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §39 | `apkc link` dentro del hospedador con forma de navegador: los recursos, compilados del lado del cliente | [03-recursos-y-build-go.md](evidencia/03-recursos-y-build-go.md) |
| §40 | El `.class` que hay que escribir: la plantilla de `apkc create`, medida con `javap -v -p` | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §41 | El `.class` escrito en Go: `javap` lo lee igual que el de javac, y el dex sale byte a byte idéntico | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §42 | La clase anónima: `this$0`, `EnclosingMethod`, `invokeinterface` y el dex idéntico de las seis clases | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §43 | `apkc compile`: el `.class` sin `javac`, medido en el binario y dentro del hospedador con forma de navegador | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §44 | `apkc build --linker go --compiler go`: un apk completo sin levantar ningún proceso, también en el hospedador con forma de navegador | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §45 | La página compila dentro de chromium: el apk entero, y byte a byte igual que el build nativo | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §46 | También el proyecto que la página *genera*: la clase anónima, el `@+id` y el `style`, dentro de chromium | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §47 | `apkc verify`: la pestaña dictamina su propio apk, cotejado con `apksigner` caso por caso | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §48 | Lo más cerca de `pm install` que hay sin teléfono: la clase que el manifiesto promete está en el dex del navegador | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §49 | Escribir de verdad: campos, `static {}`, variables locales y aritmética, medidos contra `javap -v -c -l` | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §50 | El traductor DEX aprende las cuentas y los campos: un proyecto con variables propias ya llega al apk | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §51 | El fuente se escribe en la página: campos, cadena de llamadas y cuentas, compilados y verificados en chromium | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §52 | El `if`, medido dos veces: la inversión de javac en el `.class` y las tres formas de rama en el dex | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §53 | Concatenar cadenas y plegar constantes: la versión 52 pega con `StringBuilder`, y `1 / 0` no se dobla | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §54 | La app del contador: anónima, campo del exterior, `if` y concatenación en el mismo cuerpo | [04-clase-java-y-pagina.md](evidencia/04-clase-java-y-pagina.md) |
| §55 | El teléfono dice la última palabra: tres defectos de `map_list` y de áreas que ni `d8` ni nuestros tests veían | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §56 | El `debug_info_item`: lo que faltaba del 2G.3, medido con `dexdump -d` y una traza en el teléfono | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §57 | La frame: los argumentos van al final, y eso solo lo sabía ART | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §58 | El bloque de anotaciones, medido byte a byte (y por qué es la diferencia del 2G.4, no el 2G.3) | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §59 | El bloque de anotaciones **se emite**, y `dexdump -a` ya no distingue el nuestro del de `d8` | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §60 | Las dos diferencias que quedaban: la marca de `d8` se descarta a propósito y el `type_list` sobrante era un bug | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §61 | Ejecutar en vez de instalar: `app_process` corre el dex en el terminal y destapó tres cosas | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §62 | Con el banco ejecutando de verdad: estáticos de referencia, sobrecargas y un temporal que se perdía | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §63 | Los bucles: el `goto` de vuelta medido, y el `.class` del `while` idéntico al de javac | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §64 | El `switch`: `packed-switch` y `sparse-switch` medidos, y el `<clinit>` que ART exigía | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §65 | El `StackMapTable`: la JVM real ya verifica nuestro `.class`, y los marcos coinciden con los de javac | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §66 | Arreglos: el `main(String[])` compilado y dexado solo por nosotros corre en ART | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §67 | `i++` y `--i`: `iinc` en el `.class`, `add-int/lit8` en el dex | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §68 | `+=` y `-=`: `iinc` si el literal cabe, lectura-cuenta-escritura si no | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §69 | El `do-while`: el cuerpo primero y la condición directa | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §70 | `break` y `continue`: el destino de cada uno, medido | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §71 | `try/catch` y `throw`: la tabla de excepciones, y el defecto de locales que destapó ART | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §72 | `private`/`protected` y los `access$NNN` de javac, por los dos dexers | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §73 | El inicializador `new T[] {…}`, y una clase llamada `B` que se leía como el primitivo | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §74 | El `long`: dos huecos, dos palabras, la constante `Long` del pool y el `const-wide` del dex | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §75 | El `finally` y los dos `catch` en el mismo `try`: la copia en cada camino y el `any` que re-lanza | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §76 | El valor pendiente: un `return` o un `break` que cruza el `finally` | [05-dex-art-banco.md](evidencia/05-dex-art-banco.md) |
| §77 | El `float` y el `double` en el `.class`: un hueco contra dos, y los tags 4 y 6 del pool | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §79 | El `instanceof`, y el oráculo de `d8` sin aparato: `instance-of` es 0x20 en forma 22c | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §81 | Las comparaciones anchas: `lcmp`, `fcmpl`/`fcmpg`, `dcmpl`/`dcmpg`, medidas las cinco | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §80 | Las formas dalvik del `float` y el `double`, medidas sobre el fichero de `d8` —y ya implementadas | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §82 | El banco en ART sin `rish`: `float`, `double` e `instanceof` ejecutados, y los tres defectos que salieron | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §83 | Los campos anchos privados: `dup2_x1`, `dup2`, `pop2`, y el orden de los `proto_ids` que ART exige | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §84 | Los arreglos de `long`, `float` y `double`, y el defecto de pares que solo ve ART | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §85 | El literal se promociona **sobre la constante**, antes de elegir su forma | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §86 | Un marco por encima de v15: las formas `/from16` que escribe `d8`, medidas sin aparato | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §87 | El `char`: viaja como entero, se estrecha con `i2c` y tiene opcodes propios en los dos mundos | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §88 | El entorno del shell y el banco por `sudo` sin `rish` | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §89 | El literal `X.class`: `ldc` de CONSTANT_Class, `getstatic` del primitivo y `if_acmp*` | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §90 | Las llamadas de más de cinco palabras: `invoke-*/range` (3rc) | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §91 | Constructores escritos a mano: `super`/`this`, los inicializadores de campo y dos `code_item` | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §92 | `throws`, interfaces propias, métodos abstractos y `invokeinterface` | [06-subconjunto-ancho.md](evidencia/06-subconjunto-ancho.md) |
| §93 | Recursos binarios: `res/drawable` y `mipmap` con PNG, `roundIcon` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §94 | `values-es`: language, `spec_map`, offset ausente y orden del pool | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §95 | La app del criterio de cierre, compilada y firmada sin `aapt2` ni `javac` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §96 | `string-array`, `integer-array` y `plurals` en un `values` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §97 | `menu`, `xml` y `raw`: AXML de raíz libre y fichero literal | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §98 | La versión de platform: API → versión de Android en la cabecera AXML | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §99 | Una anónima dentro de un método `static` sin captura | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §100 | La captura de locales effectively-final: `val$x`, el ctor y `MethodParameters` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §101 | `Signature` genérica en el dex: el troceado de `d8` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §102 | El `<permission>` del manifiesto y su `Manifest.java` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §103 | `layout-land` y `drawable-hdpi`: orientación y densidad | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §104 | Estado de 5.21 (`v3`) y 5.22 (`invokedynamic`) | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §105 | `fraction`: radix, 1.0 y el bit de `%p` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §106 | `attr` y `declare-styleable` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §107 | `values-es-rMX`: país dentro de la configuración | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §108 | APK Signature Scheme v3, escrito y verificado por `apksigner` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §109 | `invokedynamic`: el coste medido de la desugración y por qué queda fuera | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §110 | `invokedynamic` de una lambda estática sin capturas, desugrado y ejecutado | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §111 | Lambdas con capturas: un campo por valor y el receptor de `this::m` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §112 | Referencia a un método de instancia sin receptor (`Tipo::m`) | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §113 | SAM genérico: descriptor borrado y `checkcast` al tipo instanciado | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §114 | Borrado a una cota (`T extends Number`) y el orden del `annotation_set` | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §115 | `altMetafactory`: lambda serializable y marcadores de una intersección | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §116 | `makeConcatWithConstants`: la concatenación dinámica de JDK 9 | [07-recursos-lambdas.md](evidencia/07-recursos-lambdas.md) |
| §117 | Un keystore de release en la pestaña: contraseña, alias y certificado | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §118 | Un `.aab` en la pestaña: firma JAR v1, sin bloque de apk | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §119 | Firma v4 (`.idsig`): árbol de verity y la firma que va al lado del apk | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §120 | Rotación v3.1: el linaje y el bloque que escribe `apksigner` | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §121 | Rotación v3.1 escrita en Go: linaje, v3.0, v3.1 y v4 de dos firmantes | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §122 | EC en el firmante: los algoritmos medidos (5.32, aún abierto) | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §123 | EC en el firmante: RSA, P-256 y P-384 en el mismo camino | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §124 | El `minSdk` del proyecto no cambia el firmante v3 de un apk sin rotación | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §125 | La v1 con SHA-1 para `minSdk < 18`: el formato medido (5.34, aún abierto) | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §126 | La v1 con SHA-1, escrita y verificada | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §127 | El `.aab`: qué es lo que `bundletool` añade al `bundle.zip` (5.33, en marcha) | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §128 | El `.aab` ensamblado y firmado en Go, sin `bundletool` ni `jarsigner` | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §129 | La caída de un brazo del `switch` al siguiente | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §130 | `ConstantValue` ancho y la cuenta de `float`/`double` en tres registros | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §131 | `instanceof T[]` y la forma negada | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §132 | `catch (A \| B e)` y el `continue` que sale de un `try` con `finally` | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §133 | Los dos `finally` anidados con salida pendiente: sigue fuera, y por qué | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §134 | Las piezas nuevas ejecutadas en ART | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §135 | `format="enum"` y `format="flags"` con hijos | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §136 | Instalar la app de cierre destapó dos fallos del emisor | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §137 | Más densidades y la orientación `port` | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §138 | Los atributos de `<permission>`: `flags` se escribe en hexadecimal | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §139 | El `finally` anidado con salida pendiente: copias y manejadores como javac | [08-firma-subconjunto.md](evidencia/08-firma-subconjunto.md) |
| §140 | Abrir la pestaña en Brave Nightly desde Termux: el `BOOTCLASSPATH` que `am` no trae | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §141 | El scroll del editor: tres capas con scroll propio, no un `transform` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §142 | El `android.jar` sale del SDK local o del paquete oficial de Google; el repo no lo versiona | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §143 | Compilar dos veces seguidas ya no deja huérfano `build/res` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §144 | Compilar debug sin keystore: la clave típica se genera dentro del propio apkc | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §145 | La consola pinta ANSI y las descargas se ven sin abrir Compilar | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §146 | Menú de mantener pulsado y velo de carga para OPFS | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §147 | La clave de release se crea en la pestaña y la barra se puede dejar en iconos | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §148 | Propiedad de los datos, ajustes y una consola que se puede seguir | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §149 | Plantillas, buscar/reemplazar, aviso de targetSdk y el gate que ya se paga solo | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §150 | Tamaño del editor, iconos por tipo y `consola.js` fuera del orquestador | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §151 | G cerrada: el gate entero en verde y la segunda pestaña sin carreras | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §152 | E cerrada: el gate pesado pasa en Ubuntu y el release espera al gate | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §153 | Surge no fija cabeceras: las pone el service worker y la página se recarga una vez | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §154 | El gate completo en un escritorio Linux, sin Termux | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §155 | Publicado en apkc-studio.surge.sh y verificado desde el host | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §156 | `++`/`--` como valor: prefijo y postfijo, local y campo | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §157 | Asignaciones de desplazamiento y bits: `<<=`, `>>=`, `>>>=`, `&=`, `\|=`, `^=` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §158 | `protectionLevel` compuesto en `<permission>` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §159 | `nodpi`, `tvdpi` y `sw600dp`: los tres calificadores que faltaban | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §160 | Los oráculos que faltaban: `zipcheck.py` y `bundletool` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §161 | Firefox: el service worker también aísla fuera de Chromium | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §162 | Sin destello en la primera carga | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §163 | Un solo comando para publicar y verificar | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §164 | `apkc verify` comprueba v3 y v3.1 | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §165 | Sal en la firma v4 y el estado de los firmantes alternativos | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §166 | Más oráculos en CI: `dexdump` sí, ART en dispositivo no | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §167 | `try` con `catch` que cruza una salida | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §168 | `arraycopy` y arreglos de dos dimensiones | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §169 | `goto_w` y `goto/16` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §170 | Clase anidada estática y anotaciones más allá de `@Override` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §171 | `layout-v1`, la copia automática de aapt2 | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §172 | CSP con hashes, PWA instalable y el `wasm-unsafe-eval` | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §173 | Tamaño del wasm: qué reduce y qué no | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §174 | Accesibilidad y teclado: enlace de salto y foco visible | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §175 | Informe agregado de fricción, sin red | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §176 | Tres mundos visuales para el editor, sin clonar VS Code | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §177 | Instalación PWA, idioma, atajos y Trusted Types | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §178 | ART en CI: frontera justificada | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §179 | Subconjunto Java: switch de String, var, enum, TWR, anidadas y anotaciones | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §180 | Recursos: small/large/xlarge y -vN | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §181 | Linaje y SourceStamp: frontera medida | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §182 | Gate final sobre el árbol integrado | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §183 | Trusted Types: los otros sumideros aparecieron al probar el sitio | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §184 | Publicación del sitio integrado | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §185 | Linaje v3.1 y SourceStamp: de frontera a pieza cerrada | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §186 | Recursos: uiMode, touchscreen, teclado y navegación | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §187 | El .aab nace en Go, también desde el enlazador y el compilador Go | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §188 | Editor: check en vivo, plantillas, paleta y contraseñas redactadas | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §189 | Infraestructura: ART y Safari informativos, bench y auditoría | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §190 | Gate final, dos agentes caídos y lección de orquestación | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §191 | Verificación del host publicado: transporte HTTP/2 truncado, no falso verde | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §192 | ART en la Blackview, no en emulador | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §193 | Las cuatro fronteras Java que quedaban, cerradas | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §194 | Deuda abierta al cierre de esta ronda | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §195 | Diagnóstico en vivo y copias truncadas del wasm | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §196 | Causa del wasm truncado y recuperación | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §197 | Carga, respaldo de caché y pruebas versionadas | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §198 | Estado final para una sesión limpia | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §199 | Cierre de la deuda Java: varargs y `try-with-resources` múltiple | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §200 | El falso error de `R.layout.main` en el diagnóstico en vivo | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §201 | Vista previa del APK: fases 1 a 3 | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §202 | El producto entero: gate final, editor de layout y recorrido de punta a punta | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §203 | El emulador enlaza `android:onClick` y separa sus tres secciones | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §204 | Árbol reconciliado: estado real del gate y del `android.jar` | [10-fronteras-deuda.md](evidencia/10-fronteras-deuda.md) |
| §205 | E1 del plan de emulación: ciclo de vida y navegación entre actividades propias | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §206 | E1.1: correcciones de la auditoría del emulador, con su crítica | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §207 | E3: estado, recreación y persistencia local, con su crítica | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §208 | E4: fidelidad visual del modelo, con su crítica | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §209 | E2: listas con adaptador del proyecto, su merge y su crítica | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §210 | E5: entrada de usuario, cerrada con su medición | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §211 | E7: diálogos propios y el `which` de sus botones, con su crítica | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §212 | E6: reloj lógico, cerrado con la aprobación de la ronda 3 | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §214 | I4: dexado por clase, sin cambiar un byte del `classes.dex` | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §215 | I4: los dos `INCIERTO` del crítico, cerrados | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §213 | El intérprete de la vista previa ejecuta la tabla de excepciones del dex | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §216 | Dependencias: recursos `res/__deps/` y `lib/<abi>/` en el enlazador | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §217 | Dependencias: importador local sin red | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §218 | Dependencias: clases de `jar/` en el compilador Go | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §219 | Dependencias: fixture nativo y su rechazo clasificado | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §220 | Dependencias: dex en Go, sin d8, y caché por bucket | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §221 | Apps de ejemplo: cinco builds reproducibles y la comparación no archivada | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §222 | El colector de la tabla sigue el operando de `ldc_w` | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §223 | Reparto de un bucket grande en varios dex | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §224 | Clase duplicada entre dependencias | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §225 | El frente `jar/` cierra: inventario a cero y el APK del jar real verificado | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §226 | Promoción a `goto/32` de las ramas largas | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §227 | `check` compila todos los fuentes del proyecto | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §228 | El dex de la app también se reparte | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §229 | El `.aab` multi-dex, derivado por bundletool | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §230 | El gate final, en verde | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §231 | Genéricos en `extends` y en el campo: los dos bugs del usuario | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §232 | El marco de recursos se cachea entre pasadas de `check` | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §233 | Recursos de AAR, dibujo de vista propia y un APK arrastrado | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §234 | `check` con varios fuentes: colisiones y hermanos inválidos | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §235 | `goto_w` en el emisor de `.class`, y el rechazo nombrado de `invokedynamic` y `condy` | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §236 | La verificación de ART, ampliada a ocho casos | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §237 | Inyección de shell y «zip bombs»: los dos arreglos de seguridad | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §238 | Hot swap en el intérprete: identidad, estado y despacho | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §239 | Los records: `invokedynamic` desugrado a métodos directos | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §240 | Los puntos de unión: el reparto de registros reconciliado | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §317 | Apps de ejemplo: firma, alineación y ejecución en Droidloom verificadas | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §241 | ApkShare desde el zip: diamante, `for` con dos puntos y comodines | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §242 | El gate final sobre el árbol final, en verde | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §243 | Las dos fronteras que quedan: `condy` y API < 21 | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §244 | Los campos de locales y el null de un arreglo: las dos falsificaciones | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §245 | El condicional promovido no duplica su frontera de unión | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §246 | El `final` de una declaración local | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §247 | La firma de `--intercambiar` con tipos de referencia en el proto | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §248 | El swap por nombre sin proto, y el proto obligatorio si hay sobrecargas | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §249 | La interfaz del intercambio de métodos en el emulador | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §250 | Las condiciones cerradas del main-dex pre-21 | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §251 | La lambda de un parámetro en la fuente | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §252 | La asignación como expresión, y la declaración sin inicializador | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §253 | El cierre de la ronda de auditorías: firmas, cotas, `check`, pestaña y compilador | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §254 | WISP: la spec, el TLS que no llega en JS y los relays que responden | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §255 | ART verifica y ejecuta el `check-cast` de `AppAdapter`, y destapa el dexer ancho | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §256 | El dex de las 6 clases de ApkShare, byte a byte, y el dedupe de los `styleables` | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §257 | El avance de ApkShare hasta la anónima anidada, y WISP entre el contrato y el núcleo | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §258 | La numeración de anónimas cerrada, la interfaz implícita y el endurecimiento de WISP | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §259 | La allowlist de WISP y el cierre del vigilante de la preview | [12-wisp-emisor.md](evidencia/12-wisp-emisor.md) |
| §260 | Los cierres del emisor, el AAR con `jni/` y los dos gates de WISP | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §261 | El receptor heredado y los cierres finales de WISP | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §262 | El genérico del local, la sobrecarga por tipo y el gate de red con workers | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §263 | La concatenación como argumento de `File` (`f064e5f`) | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §264 | El aviso «ni PKCS#12 ni JKS» salta también si el keystore no existe | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §265 | Las constantes `String` del classpath se doblan como su literal (`98dd864`) | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §266 | El keystore ausente ya no se anuncia como formato ilegible (`0e65ac1`) | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §267 | Sin sondeo de SDK, el keystore por defecto queda vacío y el build aborta antes de firmar | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §268 | La estática calificada por el nombre de tipo se estima sin emitir el receptor | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §269 | `ldc_w` cierra la compilación completa de ApkShare | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §270 | El gate de red suelta los workers con la red ya encendida | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §271 | El hospedador de wasm propaga el código de salida sin cortar la salida | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §272 | La capa wasm monta el jar y el fixture, y captura sin tubería (`e3c7259`) | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §273 | El oráculo javac 17 no escribe `MethodParameters` en las anónimas | [13-wasm-cierre.md](evidencia/13-wasm-cierre.md) |
| §274 | El banco no es verificable en este escritorio; su mitad local sí | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §275 | La constante `String` heredada se dobla con su literal, no como entero (`c56c51f`) | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §276 | La constante suelta en un hueco largo se dobla como javac (`6e42499`) | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §277 | El shim respeta el modo de creación de `open` (`e907433`) | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §278 | El fs simulado deja de dejar huérfanos y las pruebas ajenas a la pestaña salen por tag | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §279 | La capa wasm llega al final sin aborto y deja 15 fallos ambientales | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §280 | Un método de `Object` sobre interfaz, el hueco del `try` y la captura del dedupe | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §281 | Los dos gates WISP pasan con el núcleo inerte y sin contactar relays | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §282 | El implementador de la lambda se emite al cerrar la clase, no en el sitio | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §283 | Los oráculos que exigen javac 17 salen del binario de la pestaña por build tag | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §284 | La capa wasm deja de saltarse oráculos y queda sin fallos | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §285 | La página permite activar o desactivar la compilación incremental | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §286 | La última fila de `LineNumberTable` y el `getAdapter` del stub | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §287 | Emulador con medidas reales, hot swap sin selector y compilación en un clic | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §288 | Consola acoplable y teclado en los `EditText` del emulador | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §289 | Los dos avisos del intérprete de `preview`: permiso concedido e `invalidate` sin redibujado | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §290 | `apkc preview --locale`: idioma en los valores, sin cambiar la identidad de los recursos | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §291 | Las diez plantillas compilan y se emulan con `apkc preview` | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §292 | `e2e.sh` compila y emula las diez plantillas en un solo gate | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §293 | Fidelidad del emulador web frente a la captura real de Droidloom | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §294 | La sección de Dependencias pasa a primer nivel sobre WISP | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §295 | Hot swap, puente WISP y gates | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §296 | Arreglo del AXML, latch del puente, destino canónico y validación | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §297 | Content-Length menor que el cuerpo en el puente WISP | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §298 | Cierre del puente de reposo, lienzo, pista y descripciones | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §299 | El wasm vuelve a compilar, el corredor se centraliza y el gate falla sin `TypeError` en su log | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §310 | ApkShare compila con `--compiler go --linker go` y con `--inc` en nativo y en el hospedador wasm | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §312 | Go 1.24.0 fijado y wasm_exec.js verificado por hash | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §313 | Sesión viva de preview: una instancia, sin replay y sin error pegajoso | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §314 | Medición reproducible del incremental en wasm y nativo | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §326 | Selector de idioma de la app en la pestaña | [01-hospedador-y-pestaña.md](evidencia/01-hospedador-y-pestaña.md) |
| §334 | F0: el diff del §311 nunca integrado aplica limpio sobre `035744f` | [21-emulador-fiel.md](evidencia/21-emulador-fiel.md) |
| §335 | F1: la lista de notas enseña N filas en la sesión viva, sin el aviso del `getCount` | [21-emulador-fiel.md](evidencia/21-emulador-fiel.md) |
| §336 | F2: cinco botones dentro de 360 dp y no-intersección separada con huecos de 64 dp | [21-emulador-fiel.md](evidencia/21-emulador-fiel.md) |
| §337 | F3: el `clip` sigue y la capa del EditText se recorta al canvas | [21-emulador-fiel.md](evidencia/21-emulador-fiel.md) |
| §338 | F4: el EditText del emulador toma el tema del aparato, no el del editor | [21-emulador-fiel.md](evidencia/21-emulador-fiel.md) |
| §339 | Cierre del frente de fidelidad: pruebas focales, falsaciones y frontera CDP | [21-emulador-fiel.md](evidencia/21-emulador-fiel.md) |
| §348 | Navegación viva entre actividades en la sesión | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §349 | CAs oficiales del propio origen para WISP, bundle de 3487 B extraído del `cacert.pem` de curl con huella fija; se usan solo al activar WISP, MPL 2.0 / Mozilla Root Store Policy v3.1 confirmadas por escrito sobre ese fichero, publicación autorizada | [23-wisp-cas-oficiales.md](evidencia/23-wisp-cas-oficiales.md) |
| §350 | Emulador separado en el submódulo `apkc-emulador/`: 59 commits importados, 34 ficheros y el mismo JSON de `preview` (`pasos: 22`) | [23-emulador-modulo.md](evidencia/23-emulador-modulo.md) |
| §351 | Copia gobernada de los 34 `preview*.go` con `cmp` byte a byte en los seis puntos de build y detección de deriva (falla en el byte 121) | [23-emulador-modulo.md](evidencia/23-emulador-modulo.md) |
| §352 | Pestaña intacta: `go test ./...` en verde y `node prueba.js` → «prueba: todo en orden» sobre el wasm separado, sin CDP | [23-emulador-modulo.md](evidencia/23-emulador-modulo.md) |
| §356 | El gitlink de `apkc-emulador` pasa a `studio/`, con el pin espejo del padre: fuente única, 34 `cmp`, wasm y sitio sin fuentes Go, sin push | [24-studio-emulador-submodulo.md](evidencia/24-studio-emulador-submodulo.md) |
| §357 | Se retira el pin espejo del padre: `studio/apkc-emulador` es el único emulador, 34 `cmp`, suite Go, wasm (`pasos: 22`) y pestaña en orden | [25-un-solo-emulador.md](evidencia/25-un-solo-emulador.md) |
| §354 | Apps reales en `~/Downloads`: los tres arrancan con `pasos:0` y una pantalla falsa; SHA de los cuatro APK | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §355 | Fase A: el lector acepta los dex reales sin tocar el escritor; los tres avanzan, pero aparece el opcode 0x1d del intérprete | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §358 | El intérprete decodifica `monitor-enter`/`monitor-exit` (`0x1d`/`0x1e`): oráculo ART, paridad del escritor, corrida pareada de avisos y los tres APK avanzan sin `0x1d` | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §366 | Cuarta APK real (`com.martinmimigames.simplefileexplorer_20.apk`): línea base previa a Fase D, `pasos:162`, 12 avisos y `ejecucion.error`; `layout` real `res/R5.xml` de 30 nodos | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §367 | Quinta APK real (`org.quentin_bettoum.librememorygame_9.apk`): caso negativo fail-closed, `pasos:333`, `nodos=2`, `layout=res/layout/custom_dialog.xml`, 5 avisos; solo envoltorio, sin lógica Godot | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §359 | Fase C: lanzadora y layout leídos del manifiesto (`enabled` tipado, alias con `targetActivity`, centinela `sin-layout-xml:` sin fallback alfabético), sin pantalla inventada | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §360 | Fase B: `dexDelApk` descubre los `classes*.dex` en orden numérico y el `onCreate` se ejecuta en `classes2.dex`, con el descriptor derivado del manifiesto | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §361 | Fase D: 12 stubs bajan a 0 los avisos de la familia en el cuarto APK (`pasos` 162 → 1131), sin regresión en las otras tres | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §362 | Fase E: el tema se resuelve por id de atributo, sin campos nuevos en el JSON; suite `505.205s` y `prueba: todo en orden` | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §368 | Integración de C, B, D y E: suite `go test -count=1 ./...` en verde (`196.940s`, 960 tests) y cinco JSON en `~/.cache/apkc-web/integracion/` | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §365 | La capa `wasm` se re-ejecuta sobre el árbol final: 0 `FAIL`, 871 `PASS`, 30 `SKIP` (901 `=== RUN`), rc 0 | [14-banco-wisp-lienzo.md](evidencia/14-banco-wisp-lienzo.md) |
| §318 | Fase K re-medida sobre el árbol final: recursos de AAR y dedupe de claves | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §319 | Manifiesto del AAR: fusión de hijos, prioridad proyecto > isla > AAR y rechazo de `tools:` | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §320 | DEPS empaquetado en el sitio: índice validado, versión del SW y cero peticiones ajenas, sin publicar | [09-editor-gate-sitio.md](evidencia/09-editor-gate-sitio.md) |
| §329 | Coste real de `check`: medianas en nativo y en la pestaña por CDP | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §330 | Asignación cualificada: receptor `this` y receptor con tipo | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §331 | La columna del parser apunta al token, no a una línea inventada | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §332 | Overhead de arranque: decisión incierta, sin sesión persistente | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §333 | Matriz de cobertura Java/XML con estado medido | [11-emulador-check.md](evidencia/11-emulador-check.md) |
| §346 | Persistencia versionada de paneles, migración de claves antiguas y z-order al conectar | [22-panels-flotantes.md](evidencia/22-panels-flotantes.md) |
| §347 | Paneles flotantes para todos los módulos: geometría y persistencia en node; CDP, Droidloom, gate y publicación en verificación | [22-panels-flotantes.md](evidencia/22-panels-flotantes.md) |
| §369 | ROM con permisos: `id/list` del gestor gana 2 filas con `/storage/emulated/0` solo con `READ` concedido; `checkSelfPermission` → `-1` y `MANAGE_EXTERNAL_STORAGE` denegado | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §370 | Lote integrado de cuatro frentes sobre `c448442`: suite `go test -count=1 ./...` → `ok … 257.828s` y cinco APK con `pasos` 700/508/284/1137/547, sin CDP ni gate | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §371 | Colecciones, cadenas y trazas: 13 focales en verde y avisos integrados → deuda (F-Droid 4→4 con `forName`, Libre Memory Game 6→4, Termux 7→6), sin campo JSON nuevo | [15-apps-reales.md](evidencia/15-apps-reales.md) |
| §372 | Recursos y preferencias de Termux en worktrees aún no integrados: 223 → 64 → 408 → 503 → 719 → 3798 pasos, tres APK byte-idénticas y frontera `java.io.File.isFile` sobre nulo | [15-apps-reales.md](evidencia/15-apps-reales.md) |

**Excepciones.** Las dos únicas excepciones de la comprobación cruzada de `§` son `§0.x` y `§78`: los
`§0.x` son secciones internas de `docs/plan-android-jar-propio.md:94,97,757,775,810` y no se numeran
aquí; `§78` no existe y su hueco está explicado en el preámbulo. La tabla sigue el orden del fichero
original, donde `79`, `81`, `80` y `214`, `213` quedaron desordenados por decisión histórica; las filas
añadidas después (`§313`, `§314`, `§317`, `§348`, `§349`, `§350`, `§351`, `§352`, `§354`, `§355`, `§356`, `§357`, `§365`, `§358`, `§366`) se sitúan en el bloque de su fichero, no en orden numérico.

**Reservados en sesión.** Estas etiquetas están tomadas por trabajos en marcha y no se reutilizan:
`§311` (fidelidad), `§315-316` (incremental), `§321-325` (infra),
`§327-328` (J5), `§329-333` (diagnóstico) y `§340-347` (paneles). `§334-339` ya no está
reservado: sus mediciones están escritas en este índice. `§348` ya no está reservado: su medición
está escrita en este índice. `§349` ya no está reservado: su medición está escrita en este índice.
`§350`, `§351`, `§352`, `§354`, `§355`, `§356`, `§357` y `§358` ya no están libres: sus mediciones están escritas en este índice.
`§353` sigue reservado a F5 como **reserva vacía, no como medición**: F5 está cerrado en `d22fe3a`/`36a69c0` con crítico APROBAR condicionado al gate (`docs/auditorias.md:30-33`) y no tiene `§` propio (`docs/evidencia/21-emulador-fiel.md` solo `§334-§339`). La etiqueta se libera solo al escribir esa medición con su comando; mientras tanto no se reutiliza, en particular Fase C de `docs/plan-compatibilidad-apps-pausado.md` pasa a `§359`. `§359` y siguientes ya no quedan libres: están asignadas como se detalla en la línea siguiente; `§358` ya está medido y registrado en este índice. `§313` ya no está reservado: su medición está escrita en este índice.
`§326` ya no está reservado: su medición está escrita en este índice.
`§314` ya no está reservado: su medición está escrita en este índice.
`§346` y `§347` ya no están libres: sus mediciones están escritas en este índice.
Actualización de la numeración del plan de apps reales reanudado: `§358` medido y registrado (opcode `0x1d`/`0x1e`), `§359` Fase C, `§360` Fase B, `§361` Fase D, `§362` Fase E, `§363` Fase F y `§364` Fase G, en ese orden; `§365` ya está medido en este índice. **`§366` ya está medido**: línea base del cuarto APK (`com.martinmimigames.simplefileexplorer_20.apk`), anterior a Fase D, con su fila en el índice y el recuento en 332. Sus 12 avisos y su `ejecucion.error` son pre-corrección y no se fusionan con `§361`. **`§367` ya está medido**: quinta APK (`org.quentin_bettoum.librememorygame_9.apk`), caso negativo fail-closed con la etiqueta «solo envoltorio, sin lógica Godot», con su fila en el índice y el recuento en 333; no sustituye a `§366`, no se cuenta como app cubierta y no se fusiona con `§361`. **`§359` (Fase C), `§360` (Fase B), `§361` (Fase D), `§362` (Fase E) y `§368` (integración de los cuatro frentes) ya están medidos**: cada uno con su fila en el índice y el recuento en 338; C, B, D y E quedan hechos en capa Go/node y en verificación CDP/gate, sin que ninguna APK se dé por funcionando. **`§369` (ROM con permisos), `§370` (lote integrado sobre `c448442`), `§371` (colecciones, cadenas y trazas) y `§372` (recursos y preferencias de Termux, aún en worktrees) ya están medidos**: cada uno con su fila en el índice y el recuento en 342; las cuatro mediciones siguen en `docs/evidencia/15-apps-reales.md`, con CDP, gate y publicación fuera, y sin que ninguna APK se dé por funcionando.

**Nota sobre el comando del preámbulo.** `sed -n '47p;217p;444,445p' docs/evidencia.md` (:33) cita
números de línea del fichero antiguo de 14.501 líneas: hoy el equivalente es buscar `§1` (88 tests) y
`§8` (98 tests) en `docs/evidencia/01-hospedador-y-pestaña.md`, y `§16` (105 tests) en
`docs/evidencia/02-dexer-y-formato.md`. Las tres mediciones siguen en esos `§`, sin cambiar una palabra.
