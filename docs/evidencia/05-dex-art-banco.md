# Evidencia — §55-§76

## 55. El teléfono dice la última palabra: tres defectos de `map_list` y de áreas que ni `d8` ni nuestros tests veían

El segundo hecho del plan («el apk se instala y la actividad arranca») se probó en este aparato con el shell de
Shizuku, sin `adb`: `rish` (en el PATH, `uid=2000(shell)`, Android 15). Los ficheros se empujan y se ordena con
`rish -c`, y el oráculo que cerró el caso es el verificador de ART, que vive en el propio aparato:

```sh
rish -c 'cat > /data/local/tmp/ours.dex' < build/classes.dex
rish -c '/apex/com.android.art/bin/dexdump /data/local/tmp/ours.dex >/dev/null 2>&1; echo rc=$?'
```

Cuesta un segundo por fichero y **no requiere instalar nada**. Con eso delante, el dex que escribía la pestaña
sacó tres defectos que ningún oráculo previo veía.

**Cómo se instaló y qué respondió.** `pm install -r /data/local/tmp/ir.apk` → `Success`;
`am start -W -S -n com.myapp/.MainActivity` → `Status: ok`, `LaunchState: COLD`, `TotalTime: 527`; el proceso
quedaba vivo (`pidof -s com.myapp`) y `logcat` no traía `FATAL`. Pero la primera vez el arranque moría así:

```
java.lang.ClassNotFoundException: Didn't find class "com.myapp.MainActivity" on path: DexPathList[...base.apk...]
Caused by: ... Failure to verify dex file '.../base.apk': Bad index for class_data_item field_idx: 8 >= 6
```

Es decir: `am start` respondía `Status: ok` **y la actividad no existía** — el aviso de ART estaba enterrado en
el `logcat`, no en la salida de `am`. Nada de esto lo veía `d8`: releía nuestro fichero con `rc 0` porque no
mira el `map_list` ni recorre las áreas de forma contigua.

**Defecto 1 — el `map_list` declaraba el área de los valores estáticos con un kind inventado.** Medido en el
fichero de `d8`, el mapa cierra con `0x2000 size=6 off=2659` y `0x2005 size=4 off=2746`: el
`encoded_array_item` es **0x2005**, y 0x2007 no existe. Con 0x2007 escrito por nosotros, ART respondía
`Offset(2537) should be aligned by 4 for maplist item of type 8199` (el 8199 es 0x2007 en decimal) y después,
corregido el kind, `Unknown map section type 2007`. Los arrays de `d8` empiezan en 2746, 2749, 2755 y 2761:
**sin alineación de 4** — el `class_data` termina justo donde empieza el primer array.

**Defecto 2 — relleno entre `class_data_item`, y ART recorre el área contigua.** Quedaba el aviso original
(`field_idx: 8 >= 6`) y la bisección lo acorraló: se generaron 12 variantes del dex (anulando el
`class_data_off` y el `static_values_off` de cada una de las seis filas de `class_defs`, con el `checksum`
Adler32 recalculado) y 6 más con las filas anuladas acumulativamente. **Las 18 siguieron dando el mismo
mensaje**, así que el puntero de la fila no importaba: lo que importa es que el verificador camina por el área
a partir del offset del mapa, item tras item. Y ahí estaba la diferencia:

| | item 0 | 1 | 2 | 3 | 4 | 5 |
| --- | --- | --- | --- | --- | --- | --- |
| `d8` (2659..) | 17 | 24 | 12 | 12 | 12 | 10 (termina en 2746) |
| el nuestro (2447..) | 20 | 24 | 12 | 12 | 12 | 13 |

Los nuestros llevaban tres ceros de relleno delante de cada item (un intento de alinear a 4 dentro del área), y
el caminante los leía como los cuatro recuentos de la clase siguiente: de ahí un `field_idx` 8 contra una tabla
de 6. Corregido en `areaClassData`: los items van pegados y el área se alinea a 1, como la de `d8`. El
`encoded_field` **no lleva byte de valor** —en el fichero de `d8` el item de `Lcom/myapp/MainActivity;` mide 24
bytes y termina justo donde empieza el de `R$id`, con dos uleb por campo—, y esa es la lectura que ART acepta.

**Defecto 3 — orden de `proto_ids`.** Tras pegar los items, ART pasó al siguiente aviso:
`Out-of-order proto_id return types`. Los doce protos de `d8` van con el tipo de retorno ascendente
(`Landroid/view/View;`, `Landroid/widget/Toast;`, `Ljava/lang/String;`, `Ljava/lang/StringBuilder;` y los ocho
`V`); el nuestro los ordenaba por `shorty` primero, y el `V` de `void` caía en medio. Corregido el orden en
`orderTables` (retorno, luego `shorty`, luego parámetros).

**Defecto 4 — el `encoded_value` de un estático sin `ConstantValue`.** El último aviso fue
`unexpected static field initial value type: 'L' vs 'I'`, sobre `static int total = 5;`: al no ser `final`,
javac y nuestro emisor no escriben `ConstantValue` en el `.class`, y nosotros anunciábamos el elemento del array
con el byte `0x1e`, que ART lee como un tipo de objeto. `d8` si lleva el valor (`01 04 05`: tamaño 1, cabecera
`04`, un byte con el 5) porque propaga el `<clinit>`; nosotros todavía no, así que se anuncia **un cero del tipo
del campo** —la cabecera `64` (entero de cuatro bytes) que `d8` usa para las constantes de `R`, con ceros—, y el
`<clinit>` lo sobrescribe al correr, que es lo que habría hecho de todas formas. Los tipos que no sean `I`
siguen con el comportamiento viejo: **no están medidos en el aparato**.

**Verificación final.** Con los cuatro arreglos: `dexdump` sobre el dex del enlace en Go responde **`rc 0` y 6
clases**; `pm install -r` → `Success`; `am start -W -S` → `Status: ok`, `LaunchState: COLD`, `TotalTime: 527`, y
`logcat` trae `Displayed com.myapp/.MainActivity for user 0: +527ms` sin ninguna línea `FATAL`; `pm compile -f
com.myapp` → `Success` y `dumpsys package dexopt` muestra `[status=verify] [reason=cmdline]` con su
`base.odex` (`artd ... kOatUpToDate with filter 'verify'`). El dex del fixture pasa de 2392 a **2384 bytes** (los
ocho bytes de relleno que se iban metiendo), y la suite nativa entera queda en verde. El gate, rehecho después de
los cuatro arreglos: **41 comprobaciones, 0 fallos**, y el apk del contador que escribe chromium pasa de 10098
(§54) a **10095 bytes**.

**Y el `onClick` corre en la pantalla.** Con el apk que escribió **chromium** (`$PREFIX/tmp/pestana-apk/
pestanera-contador.apk`, 10095 bytes; `classes.dex` con sha256 que empieza `a79c1a0e36821918`):
`pm install -r` → `Success`, `am start -W -S -n com.myapp/.MainActivity` → `Status: ok`,
`LaunchState: COLD`, `TotalTime: 316`. Dos `input tap 1280 837` (el centro del `com.myapp:id/pulsar`, que
`uiautomator` reporta en `bounds="[1192,789][1368,885]"`) dejan el botón con `content-desc="pulsos 2"`, y con
cuatro, `content-desc="muchos 4"` — o sea las **dos ramas del `if`**, la resta sobre el campo de instancia
capturado (`pulsos = pulsos + 1`), el estático del exterior (`total = total + pulsos`), la concatenación con
`StringBuilder` y la llamada virtual `setContentDescription`—. El `Toast` también se ve desde fuera: `logcat`
registra las capas `Toast#3760`/`Toast#3761` en `SurfaceFlinger`, y `FATAL EXCEPTION` aparece **0 veces**.

Con esto el segundo hecho del plan está cerrado sobre el artefacto del navegador, y la suite queda en
**205 PASS + 2 SKIP** nativa y **183 PASS + 24 SKIP** en el hospedador con forma de navegador.

**Queda por medir** (no bloquea el hecho 2): el `content-desc` de un `static` de tipo objeto sin constante, y en
general los `encoded_value` que no sean enteros —ahí sigue en pie el comportamiento viejo, sin oráculo—.

**Prueba permanente:** `apkc/dexlayout_test.go` — `verificaLayout` replica en Go las cuatro comprobaciones del
verificador de ART (kinds del mapa y offsets ascendentes, retorno de los protos no decreciente, recorrido
contiguo del área de `class_data` con los índices dentro de rango y el `code_off` alineado, y tipo del
`encoded_value` contra el descriptor del campo). Corre sobre tres ficheros: nuestro dex del fixture, **el dex
que escribió `d8`** (si la prueba no pasa con el oráculo, la prueba está mal) y la clase del contador con su
`static int total = 5`.

## 56. El `debug_info_item`: lo que faltaba del 2G.3, medido con `dexdump -d` y una traza en el teléfono

El formato se midió con una sonda propia, no con un manual. La sonda (`$PREFIX/tmp/dbg/probe/Sonda.java`)
son tres métodos con las líneas separadas a propósito: `cuenta(II)V` con las sentencias en las líneas 6, 7,
8, 9 y 11; `Sonda()` en la 15, la 13 (el campo), la 16 y la 17; `arma(String)` en la 20, 21, 22, 23 y 24.

```sh
"$J/javac" -d . probe/Sonda.java                      # sin -g: la forma en que escribe este fork
d8 --min-api 21 --lib "$ANDROID_JAR" --output . probe/Sonda.class
rish -c 'cat > /data/local/tmp/d8sonda.dex' < classes.dex
rish -c '/apex/com.android.art/bin/dexdump -d /data/local/tmp/d8sonda.dex'   # positions: 0x0000 line=20 …
```

Con las palabras crudas delante (`0f 00 0e 39 4d 4b 00`, `06 02 00 00 0e 1e 2d 3c 1f 00`,
`14 01 00 0e 5a 3c 4b 69 00`) salen cuatro cosas del formato, todas medidas:

- **el item empieza por la línea inicial**. El `header_addr` que cita la documentación del formato **no
  se escribe**: `d8` lo salta y el parser de ART también. Es la segunda vez en este proyecto que el manual
  describe un campo que quien escribe el fichero omite (la primera fue el `initial_value` del
  `encoded_field`, §22 y §55).
- **el recuento de argumentos no cuenta `this`**: `<init>()V` sale `00`, `arma(String)` sale `01`,
  `cuenta(II)V` sale `02`.
- **los nombres de los argumentos son un byte `00` cada uno** cuando el `.class` no trae
  `LocalVariableTable`, y `d8` **no los toma de `MethodParameters`**: recompilada la sonda con
  `-parameters`, el área de debug sale **idéntica byte a byte** y lo único que cambia en el fichero es que
  aparece la anotación `Ldalvik/annotation/MethodParameters;`. Con `-g` sí cambian (ahí van los
  `DBG_START_LOCAL`), y ese caso no se emite aquí.
- **la posición se marca con el opcode especial** `0x0a + 15·delta_pc + (delta_linea + 4)`, cuyo rango es delta_linea en
  [-4, +10]; cerrar es `0x00`, y no hace falta `SET_FILE` porque el nombre del fichero ya está en
  `class_def_item.source_file_idx`.

Emisor en `apkc/dexdebug.go`, cableado en `armaDex`: área nueva `0x2003` justo detrás del código
(alineación 1) y el `debug_info_off` de cada `code_item` **parcheado** después de conocer la base, porque
es una referencia hacia adelante. Los items **idénticos comparten offset**: nueve cuerpos del fixture dan
**ocho items**, y el `map_list` declara ocho.

**Contra las palabras de `d8`.** Dexada la misma `.class` de la sonda por los dos caminos:

| método | `d8` | nosotros |
| --- | --- | --- |
| `<init>()V` | `0f 00 0e 39 4d 4b 00` | idéntico |
| `arma(Ljava/lang/String;)V` | `14 01 00 0e 5a 3c 4b 69 00` | idéntico |
| `cuenta(II)V` | `06 02 00 00 0e 1e 2d 3c 1f 00` | `… 2e 00`: el último salto es de dos unidades porque nuestro cuerpo tiene una más —la diferencia de secuencia de instrucciones de los párrafos 52 y 53, no del formato— |

**En el aparato.** Con el proyecto del contador rehecho: el dex pasa de 2716 a **2784 bytes**, `dexdump`
sigue diciendo `rc 0`, y `-d` lista **18 posiciones**: en `MainActivity$1.onClick` lee
`0x0000 line=18 … 0x0070 line=23`, que son las líneas reales del fuente. Y la prueba que le importa al
usuario: instalada una variante que divide por cero (`total = total / total`, con `total` en 0, línea 18
del fuente), al pulsar el botón el `logcat` del teléfono imprime

```
E/AndroidRuntime(29111): java.lang.ArithmeticException: divide by zero
E/AndroidRuntime(29111): 	at com.myapp.MainActivity$1.onClick(MainActivity.java:18)
```

Antes de este párrafo las trazas salían sin número. Ahora el apk que se compila en la pestaña lleva sus
propias líneas hasta el `logcat`, escritas por un emisor de Go.

**Efecto en las cuentas.** Fixture: 2384 → **2436 bytes**, y 13 → 14 entradas en el `map_list`. Suite
nativa: **208 PASS + 2 SKIP**, con tres pruebas nuevas en `apkc/dexdebug_test.go` (las palabras de `d8`,
el caso en que el delta se sale del especial y el área deduplicada releída con un decodificador propio).
En el hospedador con forma de navegador: **186 PASS + 24 SKIP**. Gate entero: `rc 0`, **41
comprobaciones**, y los apks que escribe chromium crecen con su área de depuración —el del contador pasa de
10095 (§55) a **10169 bytes**, el compilado de la página generada de 9834 a **9888**, y el del fuente
escrito a mano de 9589 a **9638**—.

**Defecto aparte, encontrado de paso y todavía SIN ARREGLAR.** La primera variante de la sonda de crash
metía dos locales nuevos dentro del `onClick` (`int cero = 0; int k = pulsos / cero;`), y ART rechazó la
clase: `VerifyError: … onClick failed to verify: [0x0] instance field access on object that has
non-reference type Undefined`. Medido en su `code_item`: `registers=10`, `ins=2`, y el cuerpo usa `v6`
para `this` — el traductor numera los locales del `.class` en orden (`slot i → v(base+i)`), así que los
argumentos caen en `v6`/`v7` cuando el formato exige que los `ins` registros **sean los últimos** de la
frame (`v8`/`v9`). Mientras `locales == ins` las dos cuentas coinciden, y por eso el subconjunto medido
hasta hoy (el `onClick` del contador: dos locales y dos argumentos) salía bien. En cuanto un método tiene
más locales que argumentos el dex sale **inverificable**. Hay que remapear la asignación para que los
argumentos ocupen la parte alta de la frame, medido contra la numeración de `d8`, y añadir una prueba con
un método de tres locales y dos argumentos. **Arreglado en el §57.**

## 57. La frame: los argumentos van al final, y eso solo lo sabía ART

El defecto del último párrafo del §56, cerrado con medición propia. Lo que decía el aparato al instalar
una app cuyo `onClick` declaraba dos locales más que argumentos:

```
E/AndroidRuntime: java.lang.VerifyError: Verifier rejected class com.myapp.MainActivity$1:
    void com.myapp.MainActivity$1.onClick(android.view.View) failed to verify:
    [0x0] instance field access on object that has non-reference type Undefined
```

Medido en nuestro `code_item`: `registers=10`, `ins=2`, y el cuerpo leía `this` en **`v6`**. La numeración
de `d8` para la misma forma (sonda `probe/Sonda.java` con `largo(String)`: cuatro huecos JVM —`this`,
`entrada`, `n`, `m`— y dos argumentos) sale `registers=3`, `ins=2`, y `dexdump -d` muestra

```
0000: invoke-virtual {v2}, Ljava/lang/String;.length:()I   <- entrada = v2, el ultimo registro
0006: const/4 v0, #int 3                                    <- n, m y los temporales, por debajo
000d: iput-object v0, v1, Lprobe/Sonda;.nombre              <- this = v1 = registers - ins
```

O sea: **los `ins_size` registros últimos de la frame son los argumentos**, y los demás locales y los
temporales van por debajo. Nuestro traductor copiaba el orden del `.class` (hueco JVM `i` → `v(base+i)`),
que coincide solo cuando no hay locales extra; con dos locales de más los argumentos caían en `v6`/`v7`
mientras ART miraba `v8`/`v9`, y `v6` no tenía tipo.

El arreglo está en `apkc/dexinstr.go`, una permutación en `local(n)`: el hueco `n < args` pasa a
`v(base + locales - args + n)` y los demás a `v(base + n - args)`. Con `locales == args` la fórmula es la
identidad, así que **no mueve ninguna de las sucesiones ya medidas**: las comparaciones con las palabras de
`d8` de `dexinstr_test.go` siguen pasando sin retocar una sola palabra, y el fixture sigue en 2436 bytes.

Prueba permanente: `TestLaFramePoneLosArgumentosArriba`, que fija la cuenta (`this` en `v8`, el argumento
en `v9`, los dos locales del cuerpo en `v6` y `v7` con `registers=10` e `ins=2`) y comprueba que con
`locales == argumentos` la numeración no se mueve.

Y la prueba en el aparato, con la misma fuente que antes no pasaba el verificador —`int cero = 0;` e
`int k = pulsos / cero;` dentro del `onClick`, la división en la línea 21 del fuente—:

```sh
rish -c '/apex/com.android.art/bin/dexdump /data/local/tmp/ours.dex >/dev/null 2>&1; echo rc=$?'  # rc=0
rish -c 'pm install -r /data/local/tmp/rompe.apk'                                                  # Success
rish -c 'am start -W -S -n com.myapp/.MainActivity; input tap 1280 837'                             # Status: ok
# logcat:
E/AndroidRuntime( 6432): java.lang.ArithmeticException: divide by zero
E/AndroidRuntime( 6432): 	at com.myapp.MainActivity$1.onClick(MainActivity.java:21)
```

El `VerifyError` desapareció, el método **corrió** y la traza trae la línea exacta de la división: frame y
depuración vistas desde fuera en la misma corrida. Lo que esto abre es el caso que el subconjunto todavía
no cubría —cualquier método con variables locales además de los argumentos—, que hasta ahora producía un
apk que el teléfono rechazaba en silencio hasta el primer toque.

**Cuentas después de este arreglo.** Suite nativa **209 PASS + 2 SKIP**; en el hospedador con forma de
navegador **187 PASS + 24 SKIP**; gate entero `rc 0` con **41 comprobaciones**. El fixture sigue en 2436
bytes y el apk del contador de la pestaña en 10169 (ahí `locales == ins`, así que nada se mueve); el apk
del fuente escrito a mano pasa de 9638 a **9636** —dos bytes menos, porque en `onCreate` hay un local
además de los argumentos y la numeración nueva cambia algún formato de registro; no lo he desglosado—.

## 58. El bloque de anotaciones, medido byte a byte (y por qué es la diferencia del 2G.4, no el 2G.3)

Es lo único que separa nuestro `classes.dex` del de `d8` desde el §56, aparte de la cadena con la que
`d8` se firma a sí misma. Está **medido**, y en ese momento aún **sin emitir**: `annotations_off` salía cero en nuestras clases. La
medición va aquí para que no se vuelva a descubrir a ciegas. Dos sondas —`$PREFIX/tmp/e57/an`, una anónima
dentro de un método, y `$PREFIX/tmp/e58/an`, con `@Deprecated` en clase, campo, campo estático y método—,
ambas compiladas con el `javac` del teléfono y el `d8` del SDK y leídas con
`/apex/com.android.art/bin/dexdump -a` en el aparato.

**`annotation_item`** (kind `0x2004`, alineación 1: d8 lo pone en 741 y en 1396, impar): es un
`encoded_annotation` = `ubyte` de visibilidad (`0x02` = SYSTEM en las `Ldalvik/annotation/*`), `type_idx`
uleb128, `size` uleb128 y por cada elemento un `name_idx` uleb128 seguido de un `encoded_value`.

**`encoded_value`** con la numeración que ya usa este fork (`valueInt = 0x04`, `valueString = 0x17`,
`valueType = 0x18`, `valueMethod = 0x1a`, `valueArray = 0x1c`, `valueNull = 0x1e`; el tipo van en los
**cinco bits bajos** y el código de anchura en los **tres altos**, anchura = `arg + 1` bytes):

| bytes | qué son | medido en |
| --- | --- | --- |
| `1e` | null, sin datos | `names={null, null}` |
| `44 10 80 00` | entero en 3 bytes = 32784 | `accessFlags={32784, 4112}` |
| `24 10 10` | entero en 2 bytes = 4112 | la segunda fila del array |
| `1c 02 …` | array de 2 elementos | el array de `MethodParameters` |
| `17 00` | cadena = índice 0 (`"()V"`) | `Signature; value={"()V"}` |
| `1a 0a` | método = índice 10 (`largo`) | `EnclosingMethod; value=largo` |

**`annotation_set_item`** (`0x1003`, alineación 4) = `u32` con el número de elementos y una lista de
`u32` con los desplazamientos de cada `annotation_item`. **Ojo, medida, contra lo que se cita
normalmente**: `d8` los escribe **sin ordenar** —el set de `Lcom/myapp/R$id;` es `{2623, 2614}`, decreciente—,
así que el `Integridad`/comparador byte a byte no puede exigir orden ascendente.

**`annotations_directory_item`** (`0x2006`, alineación 4). Con campos anotados (`$PREFIX/tmp/e58/an`,
directorio en 776) la rejilla de `u32` sale:

```
+ 0: 768   class_annotations_off      (un set_item con la anotacion @Deprecated)
+ 4:   2   fields_size
+ 8:   1   annotated_methods_size
+12:   0   annotated_static_fields_size
+16:   0   field_annotation[0].field_idx        = 0 (nombre)
+20: 768   field_annotation[0].annotations_off  -- y el campo 1 apunta al MISMO set (24: 1, 28: 768)
```

Es decir: **cuatro `u32` de cabecera y luego las listas**, con los `u32` de campos estáticos/instancia y
parámetros que cita la documentación del formato *no escritos* por `d8` cuando van a cero —y `annotations_off`
de un campo o un método apunta directo a un `annotation_set_item` (misma forma que un `annotation_set_ref_list`:
`u32` cuenta + `u32[]`), sin que el mapa declare nunca el kind `0x1002`—. Medido también que la cabecera de
seis `u32` que cita el manual **no es lo que hay**: leyéndola así, en `probe.Sonda$1` (directorio en 1496,
con un solo método anotado) saldría `annotated_instance_fields_size = 7`, que es imposible en una clase con
cero campos.

**Qué elige `d8` emitir** (leído con `dexdump -a`, no inferido): en la clase `InnerClass` con
`accessFlags` y `name` (nulo para una anónima), y `EnclosingMethod` (valor = índice de método) o
`EnclosingClass` (valor = tipo) según la clase venga de un método o de otra clase; en el constructor de la
anónima `MethodParameters` con dos arrays (`accessFlags` con los flags del `.class`, `names` nulos cuando el
nombre no está declarado) y `Signature`. `MethodParameters` **solo aparece si el `.class` trae el atributo**: la sonda compilada sin `-parameters` no
lo lleva en los métodos escritos a mano y sí en el `<init>` sintético de la anónima, que es lo que javac
hace por defecto. Nuestro emisor también lo escribe en ese constructor, así que el doble oráculo
(`apkc dex` dando el mismo fichero sobre las clases de javac y sobre las de este fork) **no debería
romperse**, pero eso es una comprobación que queda pendiente de hacer junto con el emisor: si al emitir las
anotaciones las dos salidas divergen, la diferencia será un atributo que un lado pone y el otro no, y hay
que resolverlo por medición y no por ajuste.

Por eso esto pertenece al **2G.4**, no al 2G.3: el 2G.4 acepta la diferencia documentada *si el apk se
instala y arranca*, y esa condición se cumplió en el §55 y volvería a comprobarse igual al emitirlas.
**Y se emitió**: el párrafo siguiente escribe el bloque y comprueba contra ese mismo oráculo.

## 59. El bloque de anotaciones **se emite**, y `dexdump -a` ya no distingue el nuestro del de `d8`

Con lo medido en el §58 escrito en `apkc/dexanota.go`, el fixture del studio pasa de **2436 a 2696 bytes** y
el `map_list` de 14 a **17 entradas** —las mismas que declara `d8` sobre las mismas seis clases—, con las
tres áreas nuevas en el mismo orden y con las mismas cuentas que él: `annotation_item` **8** (el suyo en 2311),
`class_annotation_set_item` **6** (en 2484) y `annotations_directory_item` **5** (en 2548). El proyecto del
contador pasa de 2724 bytes (§54) a **3108** y su apk a 10292. Lo que sigue son tres defectos que salieron
del camino, no adornos: cada uno se vio porque las tablas dejaron de coincidir byte a byte o porque `dexdump`
lo contó distinto.

**Defecto 1 — el atributo `EnclosingMethod` son dos `u2`, no tres.** `apkc/classreader.go` leía
`class_index`, `name_index`, `descriptor_index`; el JVMS 4.7.7 y `javap -v` sobre la anónima del fixture
(`EnclosingMethod: #35.#37`, y `#37 = NameAndType #38:#39 // onCreateOptionsMenu:(Landroid/view/Menu;)Z`)
dicen `class_index` y **una sola entrada del pool** que nombra al método. Con la lectura vieja salían
`nombre=""` y `desc=""`, y el internado se llevaba por delante una cadena vacía, un proto `()""` y un
`method_id` fantasma: `type_ids`, `field_ids` y `method_ids` dejaron de ser idénticos a los de `d8` (53
cadenas contra 52, 10 protos contra 9, 18 métodos contra 17). Ahora `metodoDelPool` resuelve las dos formas
(NameAndType directo, o `CONSTANT_Method`/`Methodref` con el índice de clase delante) y **devuelve vacío si
el resultado no empieza por `(`**: lo no medido se queda sin emitir en vez de inventarse un índice. Resultado:
52/23/9/4/17 otra vez, y el test de las tablas fijas vuelve a callar.

**Defecto 2 — `parameters_count` es un `u1`.** Leído como `u2`, el cuerpo `01 00 00 80 10` de la anónima
(un parámetro, nombre ausente, flags `0x8010` = final|mandated) se desfasaba: el conteo salía 256, el índice
de nombre se comía los `00 80` y el de los flags se caía del atributo, así que el array emitido decía
`accessFlags={0}` donde `d8` escribe `{32784}`. Con el `u1` la fila sale `{nombre: access:32784}` y el
`dexdump -a` del aparato coincide.

**Defecto 3 — `accessFlags` de `@InnerClass` viaja **tal cual lo dejó javac en `InnerClasses`**.** Medido:
`25` (0x19, public+static+final) para `R$id`, `R$layout` y `R$style`, y `0` para la anónima. Este fork lo
pasaba por `accesoClaseDex`, que en el dex junta public y static en el mismo bit y habría escrito otro
número. Los flags del `.class` no se traducen aquí.

**Lo que manda el oráculo, no el manual.** El índice de las listas del directorio es un **`u32` entero**: `d8`
escribe `06 00 00 00 b4 09 00 00` (método 6, set en 2484) y con un uleb el fichero queda dos bytes desfasado.
Las reglas de qué va dónde, leídas de su fichero: `@EnclosingMethod` (valor = `method_id` del método que
envuelve) para la anónima; `@EnclosingClass` (valor = tipo del exterior) para una clase miembro **con nombre**;
`@InnerClass` para toda clase que tenga su propia fila en `InnerClasses`, con `name` nulo si es anónima;
`@MemberClasses` en el exterior con un array de los tipos miembro **en el orden de las filas del `.class`**
(en `R`: style, layout, id, que es como javac las escribe); `@MethodParameters` solo en los métodos cuyo
`.class` trae el atributo, con los dos arrays paralelos. `MainActivity` no lleva directorio: su
`annotations_off` es cero, como el nuestro. Los `annotation_item` **se comparten entre destinos**: el
`@EnclosingClass` que en `d8` está en 2348 (en el nuestro en 2192) lo apuntan `R$id`, `R$layout` y `R$style`;
por eso diez referencias dan ocho ítems, y ocho declara el mapa. Lo que **no** coincide es el orden de los ítems
dentro del área (`d8` pone primero el `MethodParameters` de la anónima; nosotros, los de clase) —los bytes de
cada ítem son los mismos, y así lo comprueba el test.

**No se emite `Signature`**, por el mismo motivo del §58: `d8` copia el atributo troceado a su manera en un
array de cadenas (`"<T::" "Ljava/lang/CharSequence;" ">" "Ljava/lang/Object;"`) y esa troceadura no está
medida; además el subconjunto de Java de `apkc compile` no tiene parámetros de tipo.

**Lo que queda distinto sobre las mismas seis clases, contado hoy** (mapeo offset a offset de los dos
`map_list` y volcado de las dos áreas con el lector del §16): las 52 cadenas (falta la `~~D8{…}` de `d8`, §22),
**8 `debug_info_item` contra los 7 que declara `d8`** —dos de nuestros items llevan una última posición de
línea que su equivalente de `d8` no lleva: el nuestro de `onMenuItemClick` es `16 01 00 0e c3 00` y el suyo
`16 01 00 0e 00`; el nuestro de `onCreateOptionsMenu` es `14 01 00 0e e6 00` y el suyo `14 01 00 0e 00`. Esos
bytes de más impiden que dos cuerpos compartan el item, y por eso él se queda en siete y nosotros en ocho;
la regla por la que `d8` descarta esa posición final **no está medida**, y se queda escrita como diferencia—,
**9 `type_list` contra sus 8**, y el orden de los ítems dentro del área de anotaciones. Todo lo demás —las
cinco tablas de índices, `class_defs`, los tres directorios, los seis sets y los ocho ítems byte a byte—
coincide.

**Comprobaciones (aparato Android 15 por `rish`, y tests permanentes).** `apkc/dexanota_test.go` decodifica
las tres áreas con el lector del §16 y exige que el volcado en texto del nuestro sea **igual línea por línea**
al del fixture de `d8`, y que el multiconjunto de bytes de los `annotation_item` sea el mismo (8 distintos).
Sobre el proyecto del contador, construido con `--linker go --compiler go` en 521 ms: `dexdump` **sin `-j`**
(verificación encendida) sobre nuestro dex → `rc 0`; `d8 --release` lo re-dexa → `rc 0` (3108 → 3092);
y el bloque `dexdump -a` —21 líneas— sale **idéntico con `diff` mudo** contra (i) el re-dex de `d8` sobre
nuestro fichero y (ii) el dex que `d8` saca de las clases de `javac` del mismo proyecto
(`$PREFIX/tmp/e54/proyecto-javac.dex`, 3188 bytes, 19,28 s de build con JVM contra 521 ms nuestros). En
`pm install -r` → `Success`; `am start -W -S` → `Status: ok`, `LaunchState: COLD`, 1171 ms, `pidof` 5200 vivo,
y `dumpsys package dexopt` lo marca `[status=verify]`. Como `onCreate` construye el oyente anónimo, esa clase
anotada se instancia al arrancar: el cargador de ART lee su `annotations_off` sin quejarse. `apksigner` (v1 y
v2), `zipalign -c -p 4`, `zipcheck.py`, `aapt2 dump badging` y `apkc verify` siguen aprobando el apk. Suite:
**211 PASS + 2 SKIP** nativa, **189 PASS + 24 SKIP** en el hospedador con forma de navegador. El gate de
`studio/e2e.sh`, entero y con el cambio hecho: **todo en orden**, con la pestaña en **41 comprobaciones** y
sus apks de chromium re-medidos —el del proyecto generado **10030 bytes** (igual entrada por entrada al
nativo, `d8` releyendo su dex), el del fuente escrito a mano **9729**, el de la app del contador **10305** y
el de la pestaña que compila **9255**—. El tiempo del camino sin procesos del contador sigue en **521 ms**.

*(Los números de este párrafo son de antes del §60: después de interna los `type_list` por contenido y de
adoptar la política de posiciones del `--release`, el fixture mide 2680 bytes, siete `debug_info_item` y ocho
`type_list`, y el dex del contador 3092.)*

## 60. Las dos diferencias que quedaban: la marca de `d8` se descarta a propósito y el `type_list` sobrante era un bug

Cierra lo que el §59 dejó escrito como diferencia, con las tres cosas que faltaban medidas.

**1. El `type_list` sobrante era un defecto nuestro, y estaba en la clave del internado.** `areaListas`
guardaba cada lista con prefijo de origen (`"i|"+clase` para las interfaces, `"p|"+parametros` para los
protos), así que dos listas con **el mismo contenido** se escribían dos veces. Medido sobre el fixture,
recorriendo el área (cada item mide `4 + 2·size` bytes y el siguiente se alinea a 4) y anotando quién apunta
a cada una: en el fichero de `d8` el item de `+0`, `[Landroid/view/MenuItem$OnMenuItemClickListener;]`,
lo apuntan **el proto `(Landroid/view/MenuItem$OnMenuItemClickListener;)Landroid/view/MenuItem;` y el
`class_def_item` de `MainActivity$1`**; nosotros teníamos dos items iguales (`+0` y `+8`), nueve listas
contra sus ocho. La regla es entonces: **se interna por contenido, sin distinguir el uso**. Corregido,
nuestra área sale byte a byte idéntica a la de `d8` (64 bytes, los mismos offsets `+0 +8 +16 +28 +36 +44
+52 +60`) y el fichero baja de 2696 a **2688**. De paso, medida también la terminación del área: `d8` no la
rellena hasta un múltiplo de cuatro (su área mide 66 hasta el `string_data_item` que sigue), así que se
quitó el relleno final. Prueba permanente: `TestElAreaDeListasEsLaDeD8`, que compara la sección byte a
byte, exige 8 listas en los dos mapas y comprueba que el proto y el `class_def` apuntan al mismo offset.

**2. La marca `~~D8{...}` se descarta, y ahora está medida la razón.** Su contenido, leído del fixture
(byte a byte, con su `string_data_item`):

```
~~D8{"backend":"dex","compilation-mode":"release","has-checksums":false,"min-api":21,
     "sha-1":"084a89126ae0596c599143d57a65464646af2831 3","version":"9.2.4-dev"}
```

Es la firma con la que `d8` nombra su propio trabajo: dice otra herramienta, su versión y **un resumen
SHA-1 del fichero que escribió**. Copiarla sería doble mentira —el origen y un digest que no corresponde a
nuestros bytes—, y no es que nadie la necesite: medida también la soledad de esa cadena, ninguna columna de
las tablas la nombra (ni `type_ids`, ni los `shorty_idx` de `proto_ids`, ni los `name_idx` de campos y
métodos, ni los `source_file_idx`; los elementos de las anotaciones se inspeccionaron en el §59 y tampoco).
Es la única cadena de más en su tabla: **53 contra nuestras 52**. Queda escrita la decisión y su prueba en
`TestLaMarcaDeOrigenDeD8NoSeCopia`, que además falla si el fixture dejara de traerla (para que la
comprobación no se quede dormida). Emitir una marca propia (`~~apkc{...}`) sería honesto pero añadir a
propósito una diferencia nueva donde hoy tenemos una medida: no se hace.

**3. El `debug_info_item` de más era la política del modo, y adoptamos la del `--release`.** Volviendo a
dexar las mismas seis clases y la sonda del §56 en los dos modos —`$PREFIX/tmp/an/r` (2844 bytes, `--release`,
exactamente el tamaño del fixture: el fixture está hecho así) y `$PREFIX/tmp/an/d` (2852, `--debug`)—: con
`--debug` las **nueve** sucesiones de posiciones del oráculo son idénticas a las nuestras, y con
`--release` falta en tres cuerpos la última, que es la que javac pega a la **última instrucción**
(`onMenuItemClick` pierde la línea 23, `onCreateOptionsMenu` la 26, `onCreate` la 16; los cuatro `<init>`
de una sola línea conservan la suya, porque es además la primera). La regla medida: **se suelta la posición
cuya unidad dalvik es el arranque del `return` final, salvo si es la única posición del cuerpo**. Como
`apkc` invoca a `d8` con `--release` (`build.go`: `{"--lib", androidJar, "--release"}`), el camino en Go
imita ese modo: `itemDebug` recibe las palabras del cuerpo y `areaCodigo` las devuelve por clase y método.
Efecto en el fixture: **siete items, los mismos que declara el mapa de `d8`**, y el fichero en **2680
bytes**; las sucesiones de *números de línea* por cuerpo coinciden con las de `d8 --release` en los nueve
(`TestLasLineasDeDepuracionSonLasDeD8Release`) y seis de los siete items salen byte a byte iguales —el de
`onCreate` difiere en la unidad de una posición porque nuestro cuerpo la pone en `+3` y el de `d8` en `+5`,
diferencia de asignación ya aceptada en el 2G.4—.

**La política no se copió a ciegas: se comprobó en el aparato con una traza.** Sonda nueva
(`$PREFIX/tmp/e60/traza`), con la división por cero **dentro de `onCreate`** para no necesitar ningún
toque: línea 16 `int cero = 0;`, línea 17 `int k = 72 / cero;`. Construida dos veces con el mismo binario,
una por cada camino —`--compiler go` (289 ms, dex 2720 bytes) y `javac`+`d8` (6,39 s, dex 2848)—, las dos
instaladas y arrancadas (`pm install -r` `Success`, `am start -W -S` `Complete`): las dos trazas dicen
`E/AndroidRuntime: java.lang.ArithmeticException: divide by zero at
com.myapp.MainActivity.onCreate(MainActivity.java:17)` —**la línea de la sentencia, no la de la llave**—.
La `LineNumberTable` de ese `onCreate`, leída con `javap -l` sobre el `.class` que escribe este fork, es
`14:0 15:5 16:11 17:13 18:18 19:26 25:43`: la última fila (25) es la llave del método, pegada al
`return-void`, y es justo la que la política del `--release` deja fuera; la 17, la de la división, sigue
publicada. Es decir: soltar la cola no cuesta precisión en las trazas, y los dos caminos de la herramienta
coinciden.
Sobre el contador completo, `dexdump` verificando da `rc 0` en nuestro dex de 3092 bytes y `d8 --release` lo
re-dexa a 3088 con `rc 0`; comparando con `dexdump -d` contra el dex de `d8` sobre las clases de `javac`, la última
línea anunciada por cada cuerpo es la misma en los dos (el `onClick` de `d8` publica siete posiciones y el
nuestro cinco porque el suyo vuelve a pasar por 20 y 21 al reconstruir el `if`, mientras la
`LineNumberTable` que escribimos va una fila por sentencia: medida en `javap -l` sobre los dos `.class`,
ambos con seis filas y la última en la línea 23).

Suite tras los tres cambios: **214 PASS + 2 SKIP** nativa y **192 PASS + 24 SKIP** en el hospedador.

## 61. Ejecutar en vez de instalar: `app_process` corre el dex en el terminal y destapó tres cosas

Sugerencia del usuario el 2026-11-01: para no pensar en la app mientras se adquieren instrucciones,
**ejecutar** lo que este fork compila. Se puede sin instalar nada: `app_process64` del aparato corre un
`.dex` suelto con un `main`, por `rish`, en un segundo. El comando, medido:

```sh
base64 -w0 algundex.dex | rish -c 'toybox base64 -d > /data/local/tmp/algo.dex'
rish -c 'export CLASSPATH=/data/local/tmp/algo.dex; /system/bin/app_process64 /data/local/tmp p.Arranque; echo rc=$?'
```

El banco quedó versionado en `banco/` (`sh banco/banco.sh`): dos fuentes (`p.Ver`, emitida por
`apkc compile`; `p.Arranque`, de javac, porque `main` necesita un tipo de array que el analizador aún
rechaza), y cinco salidas comparadas por el **código de salida del proceso** (`System.exit(total)`). En
el fuente, `total = 5`, dos llamadas a `cuenta()` que suman de dos en dos con un `if (total > 10)` que
no llega a activarse: la respuesta es **9**, y la dan los cuatro caminos —

| camino | resultado |
| --- | --- |
| JVM sobre las clases de javac | `rc 9` |
| **nuestro dex** (`apkc compile` + `apkc dex`, 1008 bytes) en `app_process` | `rc 9`, y `dexdump` verificando `rc 0` |
| `d8` sobre javac (1104 bytes) en `app_process` | `rc 9`, `dexdump rc 0` |
| `d8` sobre **nuestra `Ver`.class** en `app_process` | `rc 9`, `dexdump rc 0`, y su dex sale **byte a byte idéntico** al de javac (los mismos 1104 bytes: `cmp` mudo) |

Cuatro cosas salieron de aquí, y ninguna las daban los oráculos de bytes.

**1. `invokestatic` de cero argumentos estaba rechazado por error.** El banco no dexaba: `a call of 0
arguments: 35c goes up to five and invoke/range is not measured here`. Medido en el fichero de `d8`, la
forma `35c` **sí** existe con recuento cero: `7100 0600 0000` (opcode `0x71`, nibble alto del segundo
byte = 0, índice del método, y la tercera unidad en cero). Corregido el guardia (`n < 1` sobraba) y
fijado con `TestLlamadaSinArgumentosSaleTresCincoceta`, que además comprueba el `sget`, el
`invoke-static {v0}` de `System.exit` y las 12 palabras del cuerpo.

**2. El `.class` que emite este fork no verifica en una JVM real.** `java -Xverify:all` sobre nuestra
`Ver` dice `VerifyError: Expecting a stackmap frame at branch target 24` en `if_icmple`, y el
`javap -v` lo confirma: **cero** atributos `StackMapTable` en nuestro `.class`, dos en el de javac. El
`javap -c -p` del oráculo doble compara las *instrucciones*, no la tabla, y `d8` traga la ausencia
(`rc 0` dexando nuestra `Ver`), así que el producto no se rompió nunca: lo que falta es
**interoperabilidad con una JVM** —el `.class` de este fork solo lo consume su propio dexer—. Va al
plan como **5.23**, y el banco lo señala en cada corrida. *(Cerrado en el §65: la JVM ya verifica y
ejecuta nuestro `.class`.)*

**3. `getstatic`/`putstatic` de una referencia no está medido.** `System.out.println` no pasó de
`apkc dex`: `a static access of type L: solo se midió el entero` (`dexinstr.go:265`). El banco lo
evita escribiendo el resultado por `System.exit`. Va como **5.24** (mide `sget-object`/`sput-object`
y el `Ljava/io/PrintStream;` del pool).

**4. El `if` con `else` pide más temporales de los que cabe.** Con el `else` del fuente de arriba,
`apkc dex` rechazó: `at pc 30, iconst_4: it needs more temporaries than the max stack javap measured`
—la pila no se descarga en el punto de unión de los dos brazos, y el presupuesto de temporales
(`base = max_stack + locales + 1`, párrafo 50) se queda corto. La diana, medida con `dexdump -d` sobre
`d8 --release` del mismo fuente: `registers 3, ins 0, outs 0`, **20 palabras**, cada sentencia fusionada
(`sget v0` + `add-int/lit8 v1, v0, #2` + `sput v1`), la comparación invertida **conservando el
registro del sumando** (`const/16 v2, #int 10` + `if-le v1, v2, +0007`), el brazo `then` dentro del `if`
seguido de su `return-void`, y el `else` colocado **después** de ese retorno. Ojo con el plegado, que
parece una errata y no lo es: `total = total - 4` sale `add-int/lit8 v0, v0, #int -2`, porque `v0` es
el `total` *antes* de sumar los dos —`d8` calcula sobre el registro, no sobre la sentencia—. Va como
**5.25** y es la antesala directa de los bucles (5.10).

Anotación de paso, para que la comparación de registros no sorprenda: nuestro `main` declara **3**
registros donde el de `d8` declara **1** (reutiliza el hueco del `String[]` que nadie lee); es la
diferencia de numeración ya aceptada en el 2G.4, medida otra vez sobre un caso que ahora **corre**.

Suite tras el arreglo del punto 1: **216 PASS + 2 SKIP** nativa (los dos nuevos, del banco:
`TestLlamadaSinArgumentosSaleTresCincoceta` y `TestNuestrasClasesEmitidasSeDexanYLeyen`) y **194 PASS + 24
SKIP** en el hospedador con forma de navegador, con las cuentas de tamaño del dex intactas (2680 bytes:
ninguna sucesión del fixture lleva una llamada sin argumentos) y el gate de `studio/e2e.sh` otra vez en
orden con los mismos apks de la pestaña (10014, 9725 y 10285 bytes).

## 62. Con el banco ejecutando de verdad: estáticos de referencia, sobrecargas y un temporal que se perdía

Seguir el camino del §61 —ejecutar en vez de instalar— destapó cuatro cosas más, y tres eran fallos
reales del emisor y del traductor. El banco (`banco/banco.sh`) ya no compara solo el código de salida:
el fuente de prueba imprime con `System.out.println`, así que compara **el texto y el código de salida**
del mismo programa por los cuatro caminos.

**1. `sget-object` y `sput-object`, medidos.** Sonda `Lanza` con `System.out.println(que)` y
`mensaje = que` (un String estático). En el fichero de `d8`: `sget-object v0, field@0000` → **`6200
0000`** y `sput-object v1, field@0001` → **`6901 0100`**, o sea la misma forma 21c con el hueco de dos
opcodes entre el entero y el de objeto (`sget` 0x60, `wide` 0x61, `object` 0x62; `sput` 0x67, 0x68, 0x69).
`accesoEstatico` acepta ahora los dos prefijos medidos (`I` y `L`) y sigue rechazando el resto por nombre.
Con eso, nuestro dex de `Lanza.di` sale **palabra a palabra igual** al de `d8` (`6200 0000 | 6e20 0000
1000 | 6901 0100 | 0e00`), y en el aparato imprime `uno`/`dos`.

**2. El internado no recorría `getstatic`/`putstatic`.** Con el objeto mío, `apkc dex` cortó con
`a field that was never interned (Ljava/lang/System;.out Ljava/io/PrintStream;)`: `collectCode` seguía
`getfield`/`putfield` e ignoraba los estáticos, así que un campo de **otra** clase nunca entraba en
`field_ids`. Los propios funcionaban por la pasada de `collectClasses`, que interna todos los campos de
la clase. Corregido en `dextabla.go` (mismo `internField`, cuatro nombres en el `case`).

**3. Cada `putstatic` se llevaba un temporal.** Con el `println` del banco, el traductor cortó con
`at pc 34, ldc: it needs more temporaries than the max stack javap measured`. Traza de `toma`/`suelta`
con el `println` de Go a la vista: `accesoEstatico` pedía un registro con `toma` **también al
escribir** y luego lo tiraba (`r = p.pop(in)`), así que cada `putstatic` dejaba un temporal asignado y
perdido; con dos en un método el marco se quedaba sin registros. Corregido: al escribir el registro sale
de la pila y no se pide ninguno. Es el hueco que el plan llamaba 5.25 («temporales en la unión de un
`if`»): la unión no tenía nada que ver.

**4. La sobrecarga se elegía por nombre y aridad.** Ejecutando la sonda: nuestro dex no imprimía nada y
`dexdump` enseñó que la llamada iba a `PrintStream.println:(Z)V` —la primera sobrecarga declarada—.
`javap -v` sobre nuestro `.class` lo confirmó: `Methodref java/io/PrintStream.println:(Z)V` donde javac
escribe `(Ljava/lang/String;)V`. `buscarMetodo` recorría por nombre y número de argumentos; ahora:
1) candidatas por nombre y aridad, deducidas de la jerarquía; 2) con una sola, esa (lo de siempre);
3) con varias, la que coincide **exactamente** con los descriptores de los argumentos; 4) si ninguna es
exacta, la **aplicable más específica** por la cadena de padres y las interfaces del jar (JVMS
15.12.2.5), que es lo que pide `Menu.add("…")` del fixture —`add(CharSequence)` y `add(int)`, y el
String solo cabe en la primera—; 5) si no hay una sola, **rechazo por nombre** con las candidatas y los
tipos a la vista. El `boxing`, los varargs, los arrays y el ensanchamiento de primitivos no están
medidos y caen en el rechazo. Los tipos de los argumentos salen del propio emisor (`e.exp` devuelve el
descriptor al bajar el valor; para las llamadas sin receptor dentro de un `static`, donde hay que
resolver antes de bajar nada, se estiman con `tipoEstimado`).

**Y una cosa que estaba rechazada y no hacía falta: llamar sin receptor desde un `static`.** Los tests
de arriba tropezaron con `llamar di sin receptor desde un metodo estatico no esta medido`; javac escribe
`invokestatic` contra la clase actual, que es la forma 35c ya medida, y ahora se emite así (y se sigue
rechazando si el método no es estático).

**Pruebas permanentes**: `apkc/javaemisor_sobrecarga_test.go` comprueba contra el `android.jar` que
`println(String)` e `println(int)` son las que se internan y que `(Z)V` no aparece, que entre
`h(CharSequence)` y `h(Object)` gana la primera con un String, y que sin candidata aplicable el mensaje
nombra las sobrecargas y los tipos. El banco, de punta a punta: nuestro dex imprime `total 7`/`total 9`
y sale con `rc 9`, **igual que la JVM y que los dos dex de `d8`**, y los tres pasan `dexdump
verificando` con `rc 0`. El dex de `d8` sobre nuestra `Ver` y sobre la de javac miden lo mismo (1404
bytes) pero **no son idénticos**: javac pliega `new StringBuilder(); append("…")` en el constructor
`StringBuilder(String)` —36 palabras y 4 registros— y nosotros escribimos las dos filas del §53 —37
palabras y 3 registros—; el banco lo informa en vez de fallar, porque la forma del código es una
diferencia aceptada y lo que se exige es que ejecute igual.

**Lo que costó del banco mismo**, para no repetirlo: `rish` duplica al terminal lo que imprime la orden
remota cuando la salida es un terminal, así que las capturas van por fichero en el aparato y se bajan en
`base64` (una tubería directa devolvía solo la última línea y el banco comparaba contra nada). Y una
trampa de proceso: el primer gate de este párrafo salió con **un problema** —«el proyecto generado no
compila en nativo»— porque se lanzó con un `apkc-gate` **viejo**, de antes de la resolución nueva;
reconstruidos los dos binarios, el mismo gate queda en orden. Los apks de la pestaña cambian de tamaño
con el código emitido: de 9725 y 10285 bytes a **9720** y **10280**.

Suite: **219 PASS + 2 SKIP** nativa (tres nuevas: dos de sobrecarga y la del rechazo sin candidata) y
**194 PASS + 27 SKIP** en el hospedador, donde las tres nuevas se saltan si no hay `android.jar`
montado; el gate de `studio/e2e.sh`, en orden.

## 63. Los bucles: el `goto` de vuelta medido, y el `.class` del `while` idéntico al de javac

Lo que el plan tenía como el ítem grande de control de flujo (5.10). Medido primero, como siempre:
fuente `while (i < n) { total = total + i; i = i + 1; }` con `static int total`, compilado por javac 21
y dexado por `d8 --release` (fichero de 1380 bytes, `dexdump -d` en el aparato):

```
registers 3, ins 1, outs 0, 12 palabras
0000: const/4 v0, #int 0            ← el local `i`, que `d8` deja en v0
0001: if-ge v0, v2, +000a           ← la condición invertida; v2 es el argumento `n`
0003: sget v1, total ; 0005: add-int/2addr v1, v0 ; 0006: sput v1, total
0008: add-int/lit8 v0, v0, #int 1   ← el paso, plegado a literal
000a: goto 0001                     ← **`28f7`: opcode 0x28 y desplazamiento -9 en el byte alto**
000b: return-void
```

De ahí salen tres cosas. **La rama hacia atrás usa el `goto` 10t con el desplazamiento en el byte alto y
con signo** (el `28f7` de arriba), y la salida un 22t con el desplazamiento en la unidad siguiente; el
traductor ya tenía las dos formas, solo que rechazaba `rel < 0` «porque el reparto de registros en el
destino no está medido». **No hace falta ningún análisis de viveza**: los locales van clavados a su
registro (`guardaLocal`) y en la cabecera del bucle la pila JVM está vacía —lo exige el verificador para
los dos caminos—, así que la vuelta encuentra los mismos registros que dejó la primera pasada. Y **el
cuerpo nuestro es el mismo que el de `d8`** salvo un plegado: nosotros escribimos `const/4 v1, #1` +
`add-int/2addr v0, v1` donde él escribe `add-int/lit8 v0, v0, #1` (la diferencia de forma del §21 y el
§53, que no se toca aquí). El `goto` sale **`28f7` idéntico**.

**El emisor ya escribe el bucle.** `while (cond) cuerpo` se analiza a un nodo propio y se emite con la
forma de javac: `cabecera: <condición invertida> if_… fin ; cuerpo ; goto cabecera ; fin:`. `for
(init; cond; paso)` **se desdobla en el analizador** como `init; while (cond) { cuerpo; paso }`, que es
exactamente lo que escribe javac: el `.class` de nuestro `for` tiene **el mismo `Code` byte a byte** que
el suyo (23 bytes, `iconst_0 1:istore_1 2:iload_1 3:iload_0 4:if_icmpge 22 … 19:goto 2 22:return`), y el
`while` también. El `goto` de la vuelta se resuelve con una función nueva, `resuelveAtras`, porque
`cierra` solo parchea al cerrar y la cabecera se cerró antes del cuerpo; el desplazamiento se comprueba
en 16 bits con signo y lo que no cabe se rechaza por nombre (`goto_w` no está medido).

**Oráculos.** El doble, con la dosis de siempre: `TestElBucleSeDexaIgualQueElDeJavac` compila los dos
fuentes con este fork y exige que el dex sea **byte a byte** el de las clases de javac para el mismo
fuente (dos fixtures nuevos en base64), y además comprueba la forma: `if-ge` (0x35) con +10, el `goto`
`0x28` con **-9**, y `return-void` al final. `TestLaRamaHaciaAtrasSeEscribeComoD8` fija la vuelta en el
traductor (cuerpo sintético, `goto` -3 palabras a la cabecera). `TestDoYTtrySiguenRechazados` deja
escrita la frontera: `do` y `try` siguen saliendo por su nombre, y `i++` también (se nombró en
el portero en vez de dejar el mensaje genérico). *(`switch` entró después, en el §64.)*

**Ejecutado.** El dex de javac's `Bucle` traducido por nosotros imprime `10` y sale con `rc 10` en
`app_process`; el `.class` del `while` escrito por nosotros, dexado por nosotros, también (`10`, `rc 10`),
y el del `for` igual. Y el banco (`banco/banco.sh`) lleva ahora un bucle: `Ver.suma(3)` con su `while`,
de modo que los cuatro caminos imprimen `total 7`, `total 9` y `total 42` y salen con `rc 42` — la JVM,
nuestro dex de punta a punta, `d8` sobre javac y `d8` sobre nuestra `Ver`, los tres dex con `dexdump
verificando rc 0`.

**Frontera, con motivo**: no hay `do-while`, ni `break`/`continue`, ni bucles con valores vivos
distintos de un local, ni `goto/16`; *(y `i++`, que se nombraba aquí, entró en el §67)*; y el `+` de
cadenas en un fuente compilado por **javac 21** sale
como `invokedynamic` y el dexer lo rechaza (plan 5.22), que es la razón de que el `Arranque` del banco
imprima enteros y no cadenas.

Suite: **221 PASS + 2 SKIP** nativa y **194 PASS + 29 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.

## 64. El `switch`: `packed-switch` y `sparse-switch` medidos, y el `<clinit>` que ART exigía

El dexer ya traduce `switch`, con las dos formas y sus payloads medidos en el aparato. Lo que se midió
primero, con javac 21 y `d8`:

- **El `.class`**: `tableswitch` (opcode `0xaa`) con relleno hasta el siguiente múltiplo de cuatro
  **contado desde el principio del `Code`** —la tabla de la sonda está en el pc 1 y sus operandos empiezan
  en el 4, o sea `00 00 00` de relleno— y `lookupswitch` (`0xab`) con la misma regla. Los dos llevan
  `default` primero y, después, `low`/`high` y los destinos, o el número de pares y los pares
  (clave, destino). Cada destino es un desplazamiento **desde el opcode**, con signo.
- **El dex de `d8`**: la tabla densa sale como `packed-switch v0, +0x30` (opcode `0x2b`) y el payload
  —`ident 0x0100`, tamaño, `first_key`, destinos— **al final del cuerpo**, alineado a cuatro bytes y
  detrás de todos los bloques; 20 unidades para ocho claves. La dispersa pequeña **no** sale como
  `sparse-switch`: en `--release` `d8` la **rebaja a una cadena de `if-eq`** (con los cuerpos
  reordenados y `return` en vez de `goto`), y hay que ir a `--debug` para ver `sparse-switch v0, +0x20`
  (`0x2c`) con `ident 0x0200`, tamaño y 26 unidades para seis claves.

**Lo que el formato no dice y el volcado sí: en el payload disperso van primero TODAS las claves y
después TODOS los destinos**, no pares intercalados (26 = 1 + 1 + 6 + 6). Intercalarlos pasó el
`dexdump` —que verifica— y lo cazó ART en ejecución: `VerifyError: Verifier rejected class: invalid
target offset 1000 (end 0x24) in switch payload`, o sea leyó una clave como destino. Y **en DEX, si
ninguna clave casa se sigue por la instrucción siguiente**, así que el `default` del `.class` (un destino
explícito) se escribe como un `goto` de una unidad detrás del switch en vez de reordenar bloques como
hace `d8`.

**Y un defecto que solo ART veía**: nuestro `<clinit>` salía con `access 0x0008 (STATIC)` y el
verificador avisó —«`<clinit>` didn't have expected constructor access flag», «not marked as
constructor»— antes de cortar la clase. Lleva `ACC_CONSTRUCTOR` como los `<init>` (0x10000), y corregido
el fichero corre. Va con `TestElClinitLlevaConstructor`.

**Ejecutado, tres caminos iguales**: `Ramas` (densa de ocho casos y dispersa de tres) llamada desde un
`Arranque` que imprime cada `total`. Nuestro dex en `app_process` da `100`, `105`, `999`, `3`, `9` y
`rc 240`; la JVM con las mismas clases, lo mismo; y el dex de `d8` sobre nuestras clases, lo mismo. Los
tres pasan `dexdump` verificando con `rc 0`. Pruebas permanentes: `TestElSwitchSeDexaComoPackedYSparse`
(las dos formas, los dos idents, las tres claves ascendentes y los destinos detrás de ellas, más el
layout del fichero) y la del `<clinit>`.

**La otra mitad: el `switch` del fuente.** El analizador lee `switch (selector) { case k: … break;
default: … }` con claves literales enteras y sin claves repetidas, y **cada brazo tiene que terminar en
`break` o en `return`** (la caída al siguiente se rechaza por nombre; el último brazo puede acabar sin
`break`). El emisor escribe `tableswitch` o `lookupswitch` con la forma medida: el relleno hasta el
múltiplo de cuatro **contado desde el principio del `Code`** y cada destino un entero de 32 bits con
signo desde el opcode. La elección de forma es de este emisor —`tableswitch` cuando el rango no pasa del
doble de los casos—: el criterio de javac 21 se sondeó y **no es una desigualdad sencilla** (con dos
casos y rango 4 elige `lookupswitch`, con cuatro casos y rango 10 elige `tableswitch`, con seis y rango
15 también `tableswitch`), así que se copia el formato, no la heurística. El resultado, con la switch
densa de ocho casos y la dispersa de tres: el `.class` emitido tiene **el mismo `Code` que el de javac**
(mismos `tableswitch` en el pc 1, mismos destinos 48…112, mismo `default`) y el dex de nuestras clases es
**byte a byte** el de las clases de javac (1432 bytes, `cmp` mudo): `TestElSwitchSeEscribeComoJavac`.
Ejecutado, los tres caminos coinciden: `100`, `105`, `999`, `2`, `9`, `rc 250` en ART con nuestro dex,
en la JVM y en el dex de `d8` sobre nuestras clases, con `dexdump rc 0`. Y el banco lleva ahora un
`switch` además del bucle: los cuatro caminos imprimen `total 7|total 9|total 102` y salen con `rc 102`.

**Frontera**: `do-while`, `break`/`continue` fuera de un `switch`, claves negativas o que no sean
literales, la caída de un caso al siguiente y un `default` que no esté al final (el emisor lo coloca el
último) se rechazan por nombre.

Efecto colateral medido en el gate: el bit nuevo del `<clinit>` cambia los apks de la pestaña de **9720 a
9727** y de **10280 a 10287** bytes (el atributo de acceso del método pasa de un byte a tres en el
`class_data`), y siguen saliendo **igual entrada por entrada** al nativo.

Suite: **225 PASS + 2 SKIP** nativa y **196 PASS + 31 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.





## 65. El `StackMapTable`: la JVM real ya verifica nuestro `.class`, y los marcos coinciden con los de javac

El ítem 5.23 del plan. Hasta aquí todo el camino de ejecución pasaba por `d8` o por ART, y **ninguno de
los dos mira el atributo**: `d8` acepta un `.class` sin marcos y ART nunca ve un `.class`. La JVM sí, y
en el §61 lo dijo con un `Expecting a stackmap frame at branch target` sobre nuestro `.class` en
`java -Xverify:all`. Es el único oráculo que quedaba sin satisfacer.

**Qué escribe javac, medido con `javap -v`** sobre una sonda con siete métodos (`Sonda.java`, compilada
con `javac -g`):

- Un cuerpo recto (`int b = a + 1; return b;`) **no lleva el atributo**.
- Un `if` sin `else` lleva **un** marco, el del destino (pc 12): `frame_type 252 /* append */`,
  `offset_delta 12`, `locals [int]` — el local que asigna el `then` pasa a estar asignado en la unión.
- Un `if/else` lleva dos: `same` en el pc 9 (la rama `else`, que no toca el local) y `append` en el 12
  (`offset_delta 2`), porque por los dos caminos el local quedó asignado.
- El bucle lleva dos: `append` en la cabecera (`offset_delta 4`, los dos locales) y `same` en la salida
  (`offset_delta 14` = 19 − 4 − 1).
- El `switch` lleva uno por destino (`same` en 28, 34 y 40) y uno tras los `goto` (en 43, `append`).
- Con referencias, el marco del cruce dice `class java/lang/String`, no un comodín.

O sea: **marco en cada destino de salto y en la instrucción que sigue a un salto incondicional o a un
`return`**, que es "cada comienzo de bloque básico salvo el primero". Y el `offset_delta` del primer
marco es el pc, no el pc menos algo.

**La forma que rompe la intuición, medida aparte** (`Cab.java`): un bucle cuya cabecera está en el pc 0.
javac escribe un marco **en el pc 0** (`offset_delta 0`, `same`) aunque ese pc ya tenga el marco
implícito, y el local declarado dentro del cuerpo (`int x = 1;`) aparece como `top`: la vuelta del bucle
llega con un entero al hueco que la entrada ve sin asignar. `java -Xverify:all` sobre esa clase no dice
nada (y el programa ni siquiera tiene `main`: la JVM verifica antes de buscarlo), así que **`top` gana en
la unión** y un hueco `top` en el marco no prohíbe que llegue un valor: prohíbe usarlo.

**Lo que hace este emisor** (`apkc/jvmframes.go`). Un intérprete abstracto sobre el cuerpo **ya
parcheado** (las posiciones dependen de los anchos definitivos de los saltos): camina las instrucciones
con la misma tabla del dexer, calcula el estado de cada comienzo de bloque con una cola de trabajo y une
los estados que llegan al mismo pc (`top` gana; dos referencias distintas se unen en `Object`; un entero
y una referencia no se unen y se rechaza por nombre). Los tipos de campos y métodos salen del **pool que
el emisor acaba de escribir**, no de una segunda copia. Cada marco se escribe como `full_frame` —el JVMS
lo permite en cualquier posición— y los `top` del final se recortan; no se copia la compresión de javac
(`same`/`append`) porque lo que el verificador compara es el contenido, no la forma, y una sola forma es
menos superficie para equivocarse. Sin ramas no hay atributo, como en javac. El analizador comprueba
además que el alto de pila que calcula no pase del `max_stack` declarado: si pasara, el `.class` no
verificaría.

**Oráculos.** (1) El barato: `TestLosMarcosSonLosDeJavac` decodifica los dos atributos —el nuestro y el
del fixture de javac— a un estado por pc y exige que **digan lo mismo**: mismos pcs, mismos locales y
misma pila, en el `while`, en las dos formas del `switch`, y en `Cab.f` y `Cab.g` (la cabecera en el pc 0
con y sin local previo). `TestSinRamasNoHayStackMapTable` fija la ausencia en el constructor recto.
(2) El de verdad: `banco/banco.sh` paso 6 ejecuta nuestra `Ver` con el `Arranque` de javac bajo
`java -Xverify:all` y compara **texto y código de salida** con la JVM. Antes decía "VerifyError por
StackMapTable faltante"; ahora dice `jvm con nuestra clase emitida (Xverify:all): rc=102 y salida igual a
la JVM  ok`. Ese `Ver` lleva el `if`, el `while` y el `switch` del banco, o sea marcos con uniones de
enteros y una referencia (`System.out`).

Dos defectos propios salieron escribiéndolo, no del oráculo: el intérprete dimensionaba los locales con
los argumentos del descriptor y rechazaba cualquier local declarado después (`se escribe el hueco 1 y el
marco tiene 1`) —se rellena con `top` hasta el `max_locals`—, y los casos no ramificados del `switch`
caían al `return` de error que sigue al `switch` en vez de continuar el flujo.

No cambia ni un byte de lo que ya estaba medido: el dexer ignora el atributo, así que el dex y los apks
salen iguales. Suite: **227 PASS + 2 SKIP** nativa y **196 PASS + 33 SKIP** en el hospedador (los dos
tests nuevos necesitan el `android.jar` y allí se saltan); gate de `studio/e2e.sh` en orden, con los
apks de siempre (10014, 9727 y 10287 bytes).

**Frontera**: `uninitializedThis`/`uninitialized` no se emiten en ningún marco (los `new` van pegados a
su `<init>` y ningún comienzo de bloque cae en medio, y los constructores llaman a `super` antes de
cualquier rama); un `long`/`double` en un descriptor lo rechaza el propio analizador por nombre (5.8); y
arrays siguen fuera (5.11).

## 66. Arreglos: el `main(String[])` compilado y dexado solo por nosotros corre en ART

El ítem 5.11, el que faltaba para una lista de verdad y para `main(String[])`. Se midió como siempre:
una sonda `Arreglos` con alta (`new int[n]`, `new String[n]`), `.length`, lectura y escritura de un
elemento, y dos métodos con bucle; compilada con `javac -g`, dexada por `d8 --release` y desarmada con
`dexdump -d` del aparato.

**La forma del `.class`**, de `javap -c`: `iconst_3; newarray int`, `iload_0; anewarray #7 // class
java/lang/String`, `aload_0; arraylength`, `aload_0; iload_1; iaload`, `aload_0; iload_1; iload_2;
iastore`, y lo mismo con `aaload`/`aastore` para las referencias. **La forma del dex**, del volcado:

```
array-length v0, v3     2130      12x: destino en el nibble bajo, fuente en el alto
new-array v0, v0, [I    2300 0500 22c: registros en el byte alto, type_id en la unidad siguiente
aget v2, v3, v1         4402 0301 23x: op|AA, después BB|CC<<8
aput v2, v0, v1         4b02 0001 23x, con el valor primero
aget-object · aput-object   0x46 · 0x4d
```

con el tipo `[I` (o `[Ljava/lang/String;`) en `type_ids`. En el `.class` el `anewarray` nombra la clase
del **elemento** (`java/lang/String`), no la del arreglo.

**El emisor ya escribe arreglos.** El analizador acepta `T[]` en declaraciones, parámetros y vueltas
(el tipo se guarda como el texto del fuente con sus corchetes), `new T[n]`, `v[i]` —de lectura y como
destino de una asignación— y `v.length` (`arraylength`). `descriptorDe` escribe `[` delante del
descriptor del elemento, así que `int[]` es `[I`, `String[]` es `[Ljava/lang/String;` y `int[][]` sería
`[[I`. El inicializador `new T[] {…}` y más de una dimensión se rechazan por nombre (piden
`filled-new-array`, que todavía no se ha medido). Trampa que costó un defecto: `pareceDeclaracion`
miraba «un `[` detrás del nombre» y confundía `v[i] = x` con una declaración; ahora solo son
declaración los corchetes **vacíos** seguidos de un nombre.

**El traductor.** `newarray` (primitivo por atype), `anewarray` (clase del elemento), `arraylength`,
`iaload`/`aaload`, `iastore`/`aastore`, con las formas de arriba, y `new-array`/`anewarray` internan su
tipo de arreglo en `collectCode`. El oráculo doble: nuestro dex de nuestra `Arreglos` es **byte a byte**
el de las clases de javac del mismo fuente (diez métodos, `cmp` mudo), o sea que el `Code` emitido es el
de javac y el traductor los baja igual.

**Ejecutar encontró dos defectos que ningún `cmp` veía, y uno era grave.** El primero, del dexer, al
verificar con `dexdump`: una tabla vacía escribía su desplazamiento en la cabecera y ART corta con
`Offset(228) should be zero when size is zero for field-ids`; una clase sin campos jamás lo había
tocado. El segundo lo destapó ART al correr `v[i] = i + 1` y sacar `3` donde la JVM saca `5`: el `iadd`
se escribía como `/2addr` sobre el registro de la **izquierda**, que era `i`… y `i` también estaba en
la pila como índice, así que el índice quedaba pisado y el `aput` escribía `v[i] = i`. Medido en `d8`
con `Vivo`/`Vivo2`, la cuenta entera tiene **tres formas** según quién muera:

```
v[i] = i + 1   d800 0201   add-int/lit8 v0, v2, #1     (22b, resultado en un registro nuevo)
v[i] = i + j   b012        add-int/2addr v2, v1        (la izquierda vive; conmuta, va a la derecha)
v[i] = i / j   9302 0102   div-int v2, v1, v2          (no conmuta: tres registros, 23x)
v[i] = i - j   9102 0102   sub-int v2, v1, v2
v[i] = i % j   9402 0102   rem-int v2, v1, v2
```

y la familia 23x es `0x90` suma, `0x91` resta, `0x92` producto, `0x93` división, `0x94` residuo.
`cuentaEntera` ahora mira la pila simulada: si la izquierda no vive, `/2addr` sobre ella (lo de
siempre); si vive y la operación conmuta y la derecha no vive, `/2addr` sobre la derecha; y si no,
la forma de tres registros en un temporal. `TestLaCuentaNoPisaUnOperandoVivo` fija las palabras de los
dos casos (`1210 b020 004b 0201 000e` y `0091 0302 004b 0201 000e`).

**`move-result` de un entero (0x0a), medido de paso.** El camino "todo nuestro" del banco llamó a un
método que devuelve `I` y el traductor solo tenía `move-result-object`: medido en `Mr.h`, `d8` escribe
`0a00` —la 11x gemela—, y ahora se emite para los cinco enteros; `long`/`double` siguen rechazados
por nombre.

**El hecho que se buscaba: un `main(String[])` que compila y dexa este fork, y corre en el aparato.**
Fuente con `int[] v = new int[4]`, dos bucles, `v.length`, `v[i] = i + 1`, una copia a otro arreglo y
cuatro `System.out.println`. `apkc compile` (34 ms) y `apkc dex` (1076 bytes) y en `app_process`:
`4`, `5`, `4`, `1`, `rc 0`, `dexdump` verificando; y la JVM con `-Xverify:all` sobre las mismas clases
dice lo mismo. Antes de arreglar el alias, ART imprimía `4`, `3`, `4`, `0`: el fallo no da
`VerifyError`, da un número. El banco tiene ahora un quinto camino con esa forma —las dos fuentes
compiladas por nosotros, sin javac ni d8—: `nuestro dex con nuestro main (sin javac ni d8): rc=102 y
salida igual a la JVM  ok`, con la salida `total 7|total 9|total 102|10` de los cinco caminos.

**Los marcos, con arreglos.** El tipo de verificación de un arreglo es `ITEM_Object` con una `Class`
cuyo nombre es el **descriptor** (`class "[I"`, medido en `Arreglos.copia`), y así se escribe; la unión
de dos arreglos distintos da `Object`, que es lo que el verificador acepta. El decodificador del test
tuvo que dejar de usar `internalName` (que convierte `[I` en el tipo null) y leer el nombre del pool
tal cual. `TestLosMarcosDeUnArregloSonLosDeJavac` compara `suma` y `copia` expandidos contra javac.

Suite: **231 PASS + 2 SKIP** nativa y **196 PASS + 37 SKIP** en el hospedador (los cuatro tests nuevos
piden el `android.jar`); gate de `studio/e2e.sh` en orden con los apks de siempre (10014, 9727 y 10287
bytes). El banco: cinco caminos y `jvm con nuestra clase emitida (Xverify:all)` en orden.

**Frontera**: el inicializador `new T[] {…}`, más de una dimensión, los arreglos de `char`/`boolean`
desde el fuente (el traductor sí acepta el atype; el emisor del fuente solo escribe `int`), pasar un
arreglo a un parámetro `Object` (`asignable` no conoce la jerarquía de arreglos; se rechaza por
nombre) y `arraycopy` no están medidos.

## 67. `i++` y `--i`: `iinc` en el `.class`, `add-int/lit8` en el dex

El ítem 5.1, el que se nombraba en el portero desde el §63. Medido con `Inc`, un fuente con `i++` e
`i--` dentro de un bucle, un `++` sobre un campo estático y otro sobre uno de instancia:

```
javac                                   d8 (dexdump -d)
i++    iinc 1, 1                        d800 0001   add-int/lit8 v0, v0, #1
i--    iinc 1, -1                       d800 00ff   add-int/lit8 v0, v0, #-1
total++  getstatic; iconst_1; iadd;     sget; add-int/lit8; sput
         putstatic
propio++ aload_0; dup; getfield;        iget v0, v1; add-int/lit8 v0, v0, #1;
         iconst_1; iadd; putfield       iput v0, v1     (el `dup` desaparece)
```

`iinc` (0x84) lleva el índice del local y una constante de un byte con signo; en DEX es la 22b
`add-int/lit8 vA, vB, #+C` (0xd8), con el destino en el byte alto de la primera unidad y el fuente con
la constante al lado. La forma 22b se conocía del plegado `i + 1` del §63; ahora está medida también
para el decremento.

**El emisor.** El analizador acepta `++` y `--` como tokens propios (antes se leían dos `+`), en
prefijo y en sufijo, y como paso de un `for`. Como sentencia: si el destino es un local se escribe
`iinc` —la forma de javac, no un `iload`/`iconst`/`iadd`/`istore`, que daría el mismo resultado con un
`.class` distinto—; si es un campo se arma `destino = destino ± 1`, que es `getstatic; iconst_1;
iadd/isub; putstatic` o el `aload_0; dup; getfield; …; putfield` de javac. **Como valor** (`x = i++`)
no está medido y se rechaza por nombre: javac tiene que conservar el valor viejo y eso es otra forma.
`+=` y `-=` siguen rechazados.

**Oráculo doble**: el dex de nuestra `Inc` es **byte a byte** el de las clases de javac (sin contar el
`dup`, que en el traductor es una copia de la pila y no una instrucción: la lectura-cuenta-escritura
del campo de instancia sale igual). `TestElIncrementoSeDexaIgualQueElDeJavac` lo fija.

**Ejecutado**: el banco usa ahora `i++` en `Ver.suma` y un `for (int j = 0; …; j++)` en
`Ver.sumaArreglo`, y los cinco caminos siguen dando `total 7|total 9|total 102|10` con `rc 102` y
`dexdump` verificando —incluido el camino "todo nuestro", con las dos fuentes compiladas y dexadas por
este fork—; la JVM con `-Xverify:all` sobre nuestras clases también.

Suite: **232 PASS + 2 SKIP** nativa y **196 PASS + 38 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden con los apks de siempre (10014, 9727 y 10287 bytes).

**Frontera**: `++`/`--` como valor, `+=`/`-=`, y `++` sobre algo que no sea una variable.

## 68. `+=` y `-=`: `iinc` si el literal cabe, lectura-cuenta-escritura si no

La mitad que faltaba del 5.1. Medido con `Mas`, que suma y resta sobre un campo y sobre un local:

```
total += 2     getstatic; iconst_2; iadd; putstatic
i += 2         iinc 0, 2            ← javac pliega el literal en el propio iinc
i -= 3         iinc 0, -3
i += 7         iinc 1, 7            (sobre un local ya cargado)
i *= 3         iload_0; iconst_3; imul; istore_0   ← el producto no se pliega
```

O sea: javac usa `iinc` cuando el operador es `+` o `-` **y** el literal cabe en un byte; para `*=`,
`/=` y para un delta que no cabe, compone la lectura-cuenta-escritura. El analizador reescribe
`x += lit` como el mismo nodo de incremento del §67 (que ya sabe que en un local es `iinc` y en un
campo `destino = destino ± lit`), y deja `x op= expr` como `x = x op expr` cuando el valor no es un
literal entero. `>>=`, `<<=`, `&=` y los demás siguen fuera: el tokenizador ni los arma.

Oráculo doble: el dex de nuestra `Mas` es **byte a byte** el de javac
(`TestElMasIgualSeDexaComoJavac`). El banco imprime además `Ver.alMenosUna(3)` con su `do-while`, así
que los cinco caminos dan `total 7|total 9|total 102|10|3` y `rc 102`.

## 69. El `do-while`: el cuerpo primero y la condición directa

Medido con `Doo`. javac escribe el cuerpo y **después** la condición directa saltando hacia atrás a la
primera instrucción del cuerpo:

```
0: iconst_0 ; 1: istore_1          int i = 0
2: getstatic total ; 5: iload_1 ; 6: iadd ; 7: putstatic
10: iinc 1, 1
13: iload_1 ; 14: iload_0 ; 15: if_icmplt 2   ← `i < n`: salta ATRÁS si se cumple
18: return
```

Es el espejo del `while`, que invierte la condición para salir. `condicionBranca` escribe la forma que
salta cuando la condición **no** se cumple, así que el emisor le pasa la comparación negada
(`i < n` → `i >= n`, cuyo salto es `if_icmplt`). El `StackMapTable` gana un marco en la cabecera: en
`Doo.hasta` es `append` en el pc 2, el primer renglón del cuerpo.

Dos detalles de la `LineNumberTable`, medidos de javac: **no hay fila para el `do`** —la cabecera
lleva la línea del primer renglón del cuerpo— y la condición lleva la línea del `while`
(`line 8: 13`). Con eso, el dex de nuestro `Doo` es **byte a byte** el de javac
(`TestElDoWhileSeDexaComoJavac`); con el `do` anotado de más, la tabla de líneas se apartaba.

Frontera: `do` con una condición que no sea una comparación (`while (true)`) se rechaza por nombre.

Suite tras los dos: **234 PASS + 2 SKIP** nativa y **196 PASS + 40 SKIP** en el hospedador; gate de
`studio/e2e.sh` en orden con los apks de siempre.

## 70. `break` y `continue`: el destino de cada uno, medido

Medido con `Corte`, que corta un `while` con `break` y salta con `continue` en un `for` y en un
`do-while`:

```
while:      break    → goto 24   (después del bucle)
for:        continue → goto 21   (¡el `iinc` del PASO, no la condición!)
do-while:   continue → goto 19   (la condición)
```

Es lo que no se podía componer con el desdoblamiento del `for` en `init; while (cond) { cuerpo; paso }`
del §63: ahí el `continue` habría saltado a la condición, y javac lo manda al paso. Por eso el
analizador tiene ahora un nodo propio, `jpara{cuerpo, cond, paso}`, cuyo `paso` vive **fuera** del
cuerpo: el emisor lo escribe detrás (el `.class` sale igual que con el desdoblamiento, y el dex de
`Para` sigue byte a byte el de javac) y apila el destino de `continue` en la etiqueta que abre justo
antes del paso.

**El emisor.** Dos pilas de contextos: `quiebres` (el fin de cada bucle y de cada `switch`, que es a
donde va `break`) y `continuar` (la cabecera del `while`, la condición del `do-while` y el paso del
`for`). `break` dentro de un brazo de `switch` —también anidado en un `if`— sale del switch porque el
switch apila su fin. Un `continue` hacia una etiqueta ya cerrada (la cabecera del `while`) se parchea
en el acto con `resuelveAtras`; el resto son saltos hacia adelante que `cierra` completa. Fuera de
sitio, los dos se rechazan por nombre.

**Un defecto de camino encontrado al tocarlo**: `anonimasDe`, que decide qué clases anónimas se
emiten, no recorría `if`, `while` ni `switch` —solo bloques, declaraciones y asignaciones—, así que un
`new OnClickListener() { … }` dentro de un `if` no se compilaba. Ahora recorre las cuatro formas.

Oráculo doble: el dex de nuestra `Corte` es **byte a byte** el de javac
(`TestElCorteSeDexaComoJavac`), y `TestRompeYContinuaFueraDeSitio` fija los dos rechazos. El banco
lleva `primerIgual(5)` (un `while` con `break`, 2) y `sinDos(4)` (un `for` con `continue`, 4): los
cinco caminos dan `total 7|total 9|total 102|10|3|2|4` con `rc 102`.

Suite: **236 PASS + 2 SKIP** nativa y **196 PASS + 42 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.

## 71. `try/catch` y `throw`: la tabla de excepciones, y el defecto de locales que destapó ART

El ítem 5.9, el último grande de control de flujo. Medido con `T`, que tiene un `try` que devuelve
desde dentro, un `throw` y un `try` con catch que se ejecuta.

**El `.class`**, de `javap -c -v`:

```
lee:     0 aload_0 ; 1 iload_1 ; 2 iaload ; 3 ireturn
         4 astore_2 ; 5 iconst_m1 ; 6 ireturn
         Exception table: from 0 to 3, target 4, Class java/lang/ArrayIndexOutOfBoundsException
atrapa:  ... 8 goto 14 ; 11 astore_2 ; 12 iconst_2 ; 13 istore_1 ; 14 iload_1 ; 15 ireturn
         Exception table: from 2 to 8, target 11
lanza:   ... 13 athrow ; 14 return
```

Cuatro reglas que dijo el oráculo, no el manual: el `to` de la tabla es **exclusivo**; el handler
empieza con el `astore` de la excepción (el único valor que el verificador deja en la pila); el rango
**excluye un `return`/`goto` final** —no pueden lanzar— pero **incluye el `athrow`** (`T.f` mide
`0..10` con el `athrow` en 9, y `g` mide `0..1` con el `ireturn` en 1); y `catch_type` es una `Class`
con el nombre **interno**, no el descriptor (con `Ljava/lang/X;` javap imprimía
`Class "Ljava/lang/X;"`).

**El dex**, de `dexdump -d`: `try_item` (start, insn_count, handler_off) detrás de las palabras, con un
relleno de una unidad si el cuerpo tiene un número impar, y la `encoded_catch_handler_list` detrás
(`uleb` de tamaño, `sleb` de cuántos tipos —negativo si hay catch-all—, pares tipo/destino). `throw`
es `0x27` (`2701`) y el handler que usa la excepción empieza con `move-exception` (`0x0d`); `d8` lo
omite cuando nadie la usa (en `atrapa` su `astore` desaparece y el handler arranca en el `const/4`).

**El defecto que encontró ART, y era de fondo.** El primer intento corrió y ART cortó con
`VerifyError: int T.atrapa(int): [0x8] register v2 has type Undefined`. La causa no era el try: era el
modelo de locales del traductor. `istore` **fundía** el local con el registro del valor (sin
instrucción), y con dos asignaciones en caminos distintos —`r = 1` en el try y `r = 2` en el catch—
la segunda fusión reescribía el mapa y una de las dos rutas quedaba con un registro sin definir.
Medido en `d8` con `Mv2` y `Mv3`: cuando el valor viene de un local o de un argumento vivo, `d8`
**mueve** (`move v0, v1` = `0110`, `move-object v0, v3` = `0730`). El traductor ahora funde solo la
**primera** escritura de un local que no es argumento y cuando el valor viene de un temporal; el resto
sueltan `move`/`move-object` al registro del local. Los nueve cuerpos del fixture no cambian (sus
locales se escriben una vez o en el mismo registro).

**Los marcos**: el handler es un comienzo de bloque con la excepción encima de la pila y los locales
unidos sobre todo el rango protegido (`same_locals_1_stack_item` en `lee`, `full_frame` en `atrapa`).
El analizador añade esa arista por cada instrucción protegida.

**Ejecutado**: nuestro dex de `T` en `app_process` da `9`, `-1`, `2`, `1` con `rc 0` —lo mismo que la
JVM—, `dexdump` verifica `rc 0`; la JVM con `-Xverify:all` sobre nuestras clases también. El banco
lleva `Ver.leeSeguro` y los cinco caminos dan `total 7|total 9|total 102|10|3|2|4|7|-1` con `rc 102`.
`TestElTrySeDexaComoJavac` compara por método (registros, `tries` y palabras) y no el fichero entero:
la `LineNumberTable` de un `try` sigue la política de este emisor, y javac anota además el `goto` de
salida con la línea de la llave del catch.

De paso entró el **menos de un literal entero** (`return -1;` era `iconst_m1`): el analizador dobla el
signo y el resto de la negación sigue fuera.

Suite: **237 PASS + 2 SKIP** nativa y **196 PASS + 43 SKIP** en el hospedador; gate de `studio/e2e.sh`
en orden.

**Frontera**: `finally`, más de un `catch` por `try`, `catch` de un tipo que no sea clase y la
negación de algo que no sea un literal entero se rechazan por nombre.

## 72. `private`/`protected` y los `access$NNN` de javac, por los dos dexers

**Los flags, con `javap -v`** sobre `Dentro` (`private void oculto()`, `protected int medio()`): un
método privado es `0x0002` y uno protegido `0x0004`, los mismos bits que §49 midió en un campo; no hay
nada más en el `method_info`. Dentro de la propia clase, `oculto()` sale como
`aload_0; invokespecial #13 // Method oculto:()V` —javac llama a un privado con `invokespecial`, no con
`invokevirtual`— y `medio()` como `invokevirtual`.

**La anónima no toca el privado: javac le fabrica métodos.** Sobre `Fuera` (un `private int secreto`,
un `private void oculto()` y una anónima que hace `secreto = secreto + 2; oculto();`), `javap -p` de la
clase exterior añade tres métodos **al final**, después de los declarados y antes del `<clinit>`:
`static int access$002(p.Fuera, int)`, `static int access$000(p.Fuera)` y `static void
access$100(p.Fuera)`, los tres con flags `0x1008` (`ACC_STATIC|ACC_SYNTHETIC`). El nombre es
`access$` + **posición del miembro** × 100 + **sentido** (`0` leer o llamar, `2` escribir), con tres
dígitos: `access$000`/`access$002` son el primer miembro, `access$100` el segundo. La posición es una
sola secuencia para campos y métodos y se asigna por **orden de primera creación**, no de declaración:
con `b` tocado antes que `a`, `b` es el 0 y `a` el 1. Una escritura crea su accesor **antes** de la
lectura del lado derecho (`secreto = secreto + 2` deja `002` antes de `000`), y las anónimas se
procesan antes que las clases anidadas con nombre (con `Interior` declarada antes del método que crea
la anónima, los dos accesores de la anónima salen primero).

**Los cuerpos, todos de `javap -c -p`** y con una sola fila de `LineNumberTable` en la línea de la
**declaración de la clase** (no la del miembro):

| miembro | access$ | cuerpo |
| --- | --- | --- |
| campo de instancia, leer | `000` | `aload_0; getfield; ireturn` |
| campo de instancia, escribir | `002` | `aload_0; iload_1; dup_x1; putfield; ireturn` (devuelve el valor) |
| campo `String`, escribir | `202` | `aload_0; aload_1; dup_x1; putfield; areturn` |
| campo estático, leer | `300` | `getstatic; ireturn` |
| campo estático, escribir | `302` | `iload_0; dup; putstatic; ireturn` |
| método de instancia | `400` | `aload_0; invokespecial; return`/`ireturn` |
| método estático | `500` | `invokestatic; return` |

**Los sitios de llamada, de la anónima:** leer es `aload_0; getfield this$0; invokestatic
access$000:(LExterior;)I`; escribir carga la captura, deja el valor y acaba en
`invokestatic access$002; pop`; un método privado es `aload_0; getfield this$0; invokestatic
access$100` (sin `invokespecial` desde la anónima). Un método **no** privado del exterior se llama con
la captura y `invokevirtual`, y uno estático con `invokestatic` sin receptor.

**En el dex**, de `dexdump -d` sobre `d8`:
- `access$000` es `iget v0, v0` + `return v0` (d8 reutiliza el registro del argumento; nuestro
  asignador usa uno más, `iget v0, v1`, porque el argumento no se declara muerto).
- `access$002` es `5901 0400` (`iput v1, v0`) + `0f01` (`return v1`): el `dup_x1` no deja ninguna
  instrucción; las tres entradas de la pila comparten el registro del valor y `suelta` no lo recicla
  mientras otra entrada viva lo nombre.
- `access$100` es `7010 0f00 0000` (`invoke-direct {v0}`) —un privado de la propia clase es
  `invoke-direct`— y el estático `access$500` es `invoke-static`; el campo estático es `sget`
  (`6000 0800`).

Y el reparto del `class_data` cambió: **un método privado de instancia es un método directo** (junto a
constructores y estáticos). Sin eso `dexdump` corta el fichero con `Direct/virtual method
11(LQ;.lee) not in expected list 0`.

**Dos defectos que solo destapó ejecutar.** (1) `java -Xverify:all` rechazó el primer `.class` con
`ClassFormatError: Arguments can't fit into locals in class file Q`: el accesor escribía
`max_locals=0` aunque su descriptor pidiera argumentos; ahora `max_locals` es el número de huecos del
descriptor (el accesor siempre es `static`, aunque el miembro no lo sea). (2) `dexdump` rechazó el dex
por la lista directa del párrafo anterior.

**Ejecutado.** Sonda `Q`: un `private int n`, un `private static int total`, `sube()`, `suma(int)`,
`lee()` privados y una anónima que hace `n = n + 2; sube(); suma(3); total = total + lee();`.
- Nuestro `.class` en la JVM (`java -Xverify:all`): `3` y `6`, `rc 0`.
- Nuestro dex (compilador + dexer en Go): `dexdump rc 0`, `app_process` imprime `3` y `6`, `rc 0`.
- `d8 --release` sobre **nuestras** clases: `rc 0`, su dex también imprime `3` y `6` con `rc 0`.
- El banco lleva `Ver.privados` (campo de instancia, campo estático y tres métodos privados tocados
  desde una anónima; devuelve `10`) y los cinco caminos dan la misma salida, ahora
  `total 7|total 9|total 102|10|3|2|4|7|-1|10`, y el sexto —la `Ver` nuestra con el `Arranque` de
  javac bajo `-Xverify:all`— `rc=102 y salida igual a la JVM`. `d8` y los caminos de la JVM reciben
  también `Ver$1.class`: la anónima es un fichero aparte y sin él `d8` pide un *nest mate* y la JVM
  muere en `privados`.

**Pruebas permanentes**: `apkc/javaemisor_privada_test.go` compara campo a campo, método a método
(nombres, descriptores, flags y orden) y, para cada cuerpo, la sucesión de instrucciones **con el
miembro resuelto por nombre** contra `javac --release 8 -g` —comparar el `Code` crudo no vale porque
los índices del pool se ordenan distinto—; incluye las dos formas de la frontera.
`apkc/dexprivada_test.go` fija que privados y accesores están en la lista directa y las formas
`iget`/`iput`+`return`/`invoke-direct`/`invoke-static`/`sget`.

**Frontera**: una clase anidada `private` (javac le añade un constructor sintético `(Exterior$1)` y
una clase marcador `Exterior$1`) y una `protected` con nombre y sin `static` (pide la captura
`this$0`) se rechazan por nombre; `protected` en campos y métodos sí entra.

## 73. El inicializador `new T[] {…}`, y una clase llamada `B` que se leía como el primitivo

**La forma de javac**, de `javap -c -p` sobre `A` (`return new int[] {1, 2, 3};`, un `String[]`, un
local y un argumento de llamada): no hay `filled-new-array`; es el largo, el `newarray`/`anewarray` y
por cada elemento `dup`, indice, valor y `iastore`/`aastore`, con el arreglo vivo en la pila:

```
0: iconst_3
1: newarray       int
3: dup
4: iconst_0
5: iconst_1
6: iastore
7: dup
8: iconst_1
9: iconst_2
10: iastore
...
15: areturn
```

El analizador admite `new T[] {a, b, …}` (y `{}` y la coma final); el emisor escribe esa cadena y
rechaza por nombre `new T[]` sin inicializador, más de una dimensión y un elemento de otro tipo.
`d8 --release` sí lo compacta a `filled-new-array {v0, v1}, [I` + `move-result-object` (medido sobre
la sonda `B`), pero el `.class` que emitimos es el de javac y `d8` lo acepta.

**Un estático de tipo arreglo es `sget-object`/`sput-object`**, no `sget`/`sput`: medido en el dex de
`d8` sobre `static int[] v` (`6200 0000` para leer y `6900 0000` para escribir). El traductor solo
aceptaba `I` y `L…`; ahora acepta cualquier descriptor de referencia (`[` incluido).

**El defecto que destapó la sonda llamada `B`**: `descriptorOf` tenía un caso para los primitivos de
una letra y devolvía `B` tal cual; una clase del paquete por defecto llamada `B` (o `I`, `S`…) quedaba
en el dex con el `type_id` `"B"` y `dexdump` cortaba con `Invalid descriptor for class_idx: 'B'`. La
entrada `Class` del pool es siempre un **nombre interno**, nunca un descriptor: `descriptorDeClase`
envuelve en `L…;` salvo que ya empiece por `L` o `[`, y lo usan `refOf` y `classDescriptor`.
`TestUnaClaseLlamadaBNoEsElPrimitivo` lo fija.

**Ejecutado** con la sonda `B` (`static int[] v = new int[] {4, 5}`, `suma(new int[] {1, 2, 3})`,
`nombres().length`):
- JVM con `-Xverify:all`: `6`, `9`, `2`, `rc 0`.
- Nuestro dex (compilador + dexer en Go): `dexdump rc 0`, `app_process` `6`/`9`/`2`, `rc 0`.
- `d8 --release` sobre nuestras clases: `rc 0`, su dex `dexdump rc 0` y `6`/`9`/`2` en ART.

**Pruebas**: `apkc/javaemisor_inicializador_test.go` compara método a método (nombres, flags y la
sucesión de instrucciones con el miembro resuelto por nombre) contra `javac --release 8 -g`, y fija
los tres rechazos; `apkc/classreader_nombre_test.go` fija el nombre de clase de una letra.

**Suite y gates con esto**: **243 PASS + 2 SKIP** nativa (eran 240; dos pruebas del inicializador y
una del nombre de clase), **196 PASS + 49 SKIP** en el hospedador (las tres nuevas se saltan sin
`android.jar` montado), `studio/e2e.sh` en orden y el banco con los cinco caminos y el sexto de
interoperabilidad: salida `total 7|total 9|total 102|10|3|2|4|7|-1|10|6`, `rc 102`, `dexdump rc 0` en
los cuatro dex. El `dexdump` del banco se lee ahora en la **misma** llamada a `rish` que lo ejecuta:
con dos llamadas seguidas el cliente leía el fichero de estado antes de que el aparato lo terminara
de escribir, y el veredicto salía con el `rc` vacío.

## 74. El `long`: dos huecos, dos palabras, la constante `Long` del pool y el `const-wide` del dex

**El pool, con `javap -v` sobre `L`**: una entrada `Long` (tag 5) **ocupa dos posiciones** —`#7 = Long 2l`
y la siguiente es `#9`; el #8 no existe—, así que el escritor reserva un hueco (`agregarDos`) y
`serializar` lo cuenta sin escribir bytes.

**Los huecos de local, con `javap -v -c` sobre `F.conRama(long a, int n)` con un `long x` dentro**:
`locals=5` con `a` en 0-1, `n` en 2 y `x` en 3-4. El `StackMapTable` de javac es un `append` con
`locals = [ long ]`: **una** entrada para las dos mitades. El marco `full_frame` de este emisor hace lo
mismo: la segunda mitad se marca `verifSegundo` en el modelo y no se escribe.

**Las palabras de pila, con `javap -v -c` sobre `L.cuenta(long,long)`**: `stack=6` (dos largos dentro y
uno fuera, dos palabras cada uno). Destapado ejecutando: con `max_stack` contado por *elementos* la JVM
corta `VerifyError: Operand stack overflow … Exceeded max stack size` en el `<clinit>` que empieza con
`ldc2_w`; el conteo es por **palabras** y javac escribe 2 para un solo largo.

**Los opcodes, leídos de `javap -c` sobre `L` y `W` y confirmados al ejecutar**: `lconst_0` 0x09,
`lconst_1` 0x0a, `ldc2_w` 0x14 (dos bytes de índice), `lload` 0x16 / `lload_0..3` 0x1e..0x21, `lstore`
0x37 / `lstore_0..3` 0x3f..0x42, `ladd` 0x61, `lsub` 0x65, `lmul` 0x69, `ldiv` 0x6d, `lrem` 0x71,
`i2l` 0x85, `l2i` 0x88, `lreturn` 0xad. `1L` y `0L` salen como `lconst_1`/`lconst_0` y cualquier otro
por `ldc2_w`, incluso `2L`.

**El ensanchado, con `javap -c` sobre `W`**: `long x = 0` es `lconst_0` (el literal se escribe ancho),
`return n;` con `n` entero es `iload_0; i2l`, y `n + m` con `m` largo es **`iload_0; i2l; lload_1; ladd`**
—el entero se ensancha en su propio sitio, antes de cargar el otro lado—, así que el emisor necesita el
tipo de los dos operandos *antes* de escribirlos: por eso aparece `tipoPrevisto`.

**Lado del dex**, medido con `dexdump -d` sobre `d8 --release` con la misma sonda `L`: `const-wide/16`
(`1600 0200`, dos unidades), `mul-long` 0x9d en forma 23x, `add-long/2addr` 0xbb, `sget-wide` 0x61,
`sput-wide` 0x68, `return-wide` 0x10, `move-result-wide` 0x0b, y en la frame los argumentos largos valen
**dos registros**: `cuenta:(JJ)J` sale con `registers=6, ins=4` y `imprime:(J)V` con
`registers=3, ins=2, outs=3`, con `invoke-static {v0, v1}` para un solo argumento largo. También midió
`d8` que reasigna el par muerto (`mul-long v4, v4, v0` escribe encima del segundo argumento) y que
pliega `1L + 300L` en una sola constante.

**Añadido con dos sondas más** (`V`: conversiones, campo de instancia ancho, constantes de 17 y 5
dígitos; `M`: las cuatro cuentas y una copia): `int-to-long v0, v2` es **0x81** y `long-to-int v1, v0`
**0x84** (12x); `iget-wide` **0x53** y `iput-wide` **0x5a** (22c, el par destino en el nibble bajo);
`const-wide/16` **0x16** (`1600 0100`, dos unidades), `const-wide/32` **0x17** (`1700 7011 0100`, tres)
y `const-wide` entero **0x18** con los ocho bytes en little-endian en cuatro unidades
(`1800 cb04 fb71 1f01` para `1234567890123L`); la familia 23x larga va de **0x9b** (suma) a **0x9f**
(residuo) —`mul-long v4, v4, v0` medido como `9d04 0400`— y la `/2addr` de **0xbb** (suma) a **0xbf**
(residuo), con `div-long/2addr v0, v2` = `be20` y `add-long/2addr v0, v4` = `bb40`. `return-wide` es
**0x10** y `move-result-wide` **0x0b**. La frame cuenta los argumentos **en palabras**: `cuenta:(JJ)J`
sale `registers=6, ins=4`, `imprime:(J)V` `registers=3, ins=2`, `escribeCampo:(LV;J)V` `ins=3`, y una
llamada con un largo es `invoke-static {v0, v1}` (dos registros para un argumento). `d8` además
reutiliza el par del operando muerto (`add-long/2addr v0, v4` encima de una constante) y no emite nada
para `long b = a; return b;` (`return-wide v0` a secas).

**Escrito y ejecutado**. El traductor ya emite esas formas (`tomaPar` da dos registros consecutivos, y
la cuenta ancha usa el mismo reparto de vida que la entera del §66). Con la sonda `L`:

- Nuestro `.class` en la JVM con `-Xverify:all`: `306` y `8`, `rc 0`.
- **Nuestro dex sobre nuestras clases es byte a byte el mismo que nuestro dex sobre las clases de
  `javac`** (1104 bytes, `cmp` mudo) —con el fixture compilado `--release 8 -g:lines,source`; con
  `-g:none` faltaba el `SourceFile` y con `-g` sobraba el `LocalVariableTable`, y las dos cosas cambian
  el fichero—.
- `d8 --release` sobre nuestras clases: `rc 0`, y su dex también da `306` y `8` en ART.
- `dexdump` del nuestro: `rc 0`, y `imprime:(J)V` sale con las mismas seis palabras que `d8`
  (`6200 0100`, `6e30 0600 1002`, `0e00`, `registers 3 / ins 2 / outs 3`). Donde elegimos otra forma
  válida está dicho en la prueba: para `a + b * 2L` nosotros hacemos `mul-long/2addr v4, v0` (`bd04`)
  encima del par muerto y `d8` escribe el `mul-long` de tres registros; y no plegamos `1L + 300L`.

Tres defectos salieron al ejecutar, ninguno visible en los `cmp` anteriores: (1) un `static long` sin
`ConstantValue` se anunciaba en el `encoded_array_item` con `value_null` y ART cortaba con
`unexpected static field initial value type: 'L' vs 'J'` —es la regla 4 del §55 con un campo ancho, y
ahora va con `valueLong` y ocho bytes—; (2) el `parameters_count` del `debug_info_item` se sacaba de
`ArgCount`, que cuenta **palabras**, y con dos `long` declaraba cuatro parámetros: ART avisaba
`invalid stream - problem with parameter iterator … for method long L.cuenta(long, long)`; el conteo es
el de parámetros del descriptor (y `this` ya no estaba dentro, así que se quita la resta que hacía el
§56); (3) una clase llamada `L` —que empieza por la `L` de un descriptor— salía al dex como
`type_id "L"` (`Invalid type descriptor: 'L'`), la misma familia de `B` del §73, y se cierra exigiendo
el `;` de cierre en `descriptorDeClase`.

**Pruebas**: `apkc/javaemisor_largo_test.go` compara método a método contra `javac --release 8`, con
`max_stack` y `max_locals` incluidos, y fija los rechazos de `double`, del arreglo de largos y de la
comparación de largos; `apkc/dexlargo_test.go` exige el dex idéntico entre nuestras clases y las de
javac y las palabras de `d8` en `imprime`; `apkc/classreader_nombre_test.go` cubre ahora `B` y `L`. La
prueba de la frontera del caminante cambió de byte (`0x61` ya está medido; queda `0xd2`).

Suite: **246 PASS + 2 SKIP** nativa y **196 PASS + 52 SKIP** en el hospedador. El banco lleva
`saldo`, `cuentaLarga(2L, 3L)` y `acumula()`, y los cinco caminos dan
`total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306` con `rc 102`.

**Frontera**: `float` y `double` (el léxico ni siquiera arma `1.0`), arreglos de largos (`laload` /
`lastore` sin medir), `lcmp` y las ramas sobre largos, `dup2`/`pop2` y el `access$` de un campo ancho
(pide `dup2_x1` y `pop2`). Un `static final long` con `ConstantValue` tampoco: `classField` guarda el
`Long` pero `buscarCampo` sigue doblando solo enteros.

## 75. El `finally` y los dos `catch` en el mismo `try`: la copia en cada camino y el `any` que re-lanza

**Cuatro sondas** (`Fin`: `finally` solo, `return` en el `try` y varios `catch` con `finally`; `Fin2`: el valor
pendiente, un `finally` que lanza y un `break` dentro del `try`; `Fin3`: un `finally` vacío, un `try` vacío y
el `throw` como última sentencia; `Fin4`: un `void` que termina en el `athrow` del `finally` y un `try`
anidado dentro de un `try` con `finally`), leídas con `javac --release 8 -g:lines,source` y `javap -c -p -l` y
`javap -v`.

**No hay subrutina**: el bloque se **copia** al pie de cada camino que puede caer —el normal y el de cada
`catch`— y el último manejador es el `any`, que guarda la excepción en un local, copia el bloque y hace
`aload` + `athrow`. En `Fin.simple` (`try { a = 1; } finally { a = 2; }`): `0 iconst_1; 1 putstatic; 4 iconst_2;
5 putstatic; 8 goto 18; 11 astore_0; 12 iconst_2; 13 putstatic; 16 aload_0; 17 athrow; 18 getstatic` con
`0..4 → 11 any`. En `Fin.multi` la fila del `any` del cuerpo va **después** de los tipos y **antes** de las de
los cuerpos de los `catch`: `0..7 → 15 AE`, `0..7 → 28 RE`, `0..7 → 42 any`, `15..20 → 42 any`,
`28..34 → 42 any`.

**Los huecos de local**: cada `catch` declara su excepción **en el mismo hueco** (los dos brazos de `multi`
escriben `astore_1`), y el temporal del `any` va en el **más alto que haya visto el método** (`astore_2` en
`multi`; `astore_3` en `Fini.anidado`, donde el `try` interior ya usó el 2). Con `return` pendiente el valor se
guarda antes y la excepción sube un hueco más: `Fin2.retornoConCatch` es valor `1`, excepción del `catch` `1`,
valor del `catch` `2`, `any` `3`.

**Las filas de la `LineNumberTable`**: la copia del camino normal lleva la línea de la **primera sentencia del
`finally`** y el `goto` (o la re-lanza) que la sigue, la de la **llave que lo cierra** —`multi`: `33 en 7` y
`34 en 12`; `simple`: `9 en 4` y `10 en 8`—; la entrada del manejador `any` se anota con la **misma línea de la
sentencia** del `finally`, no con la del `catch` ni la del `try` (`multi`: `33 en 42`, `aload` en `34 en 48`;
`Fin.lanza`: `43 en 8` y `44 en 14`). Sin `finally`, el `goto` del `try/catch` lleva la línea de la **llave que
cierra el último `catch`** —`Fini.dosCatch`: `44 en 7` y `44 en 13`—, regla que el párrafo 71 no había escrito.

**Las salidas que cambian el remate**: un `finally` que lanza (`Fin2.lanzaEnFinally`) no deja `goto` después de
la copia, y en un método `void` javac tampoco escribe el `return` implícito —`Fin4.vacioConThrowFinal` termina
en el `athrow` del manejador, en el pc 20—. Un `finally` **vacío** se suelta entero (`Fin3.vacioFinalmente`:
cuerpo del `try` y nada más, sin copia, sin `goto` y sin fila en la tabla) y un `try` **vacío** con `catch`
tampoco escribe la tabla (`Fin3.sinCuerpo` es `iload_0; ireturn`).

**Dos rangos que van al mismo manejador con el mismo tipo se mezclan**: `Fin3.soloLanzaConCatch` protege
`0..11 → 19 any` (cuerpo y `catch` de una) y `Fini.anidado` escribe `2..13 → 20 any` aunque en el medio haya un
`goto` (pc 7) y el manejador interior (10..13).

**La trampa que queda fuera**: cuando la **última sentencia del cuerpo es un `throw`**, javac escribe un rango
que se pasa a la primera instrucción de su propio manejador —`Fin.lanza`: `0..9 → 8 any` con el `astore` en 8—
y mezcla cuerpo con `catch`. Este emisor lo rechaza por nombre, y también rechaza el `return`/`break`/`continue`
que **sale** de un `try` con `finally` (el valor pendiente: `Fin2.retorno` es `istore_0` antes de la copia e
`iload_0` después; `Fin2.ruptura` guarda el destino del `break` en otro local) y un `catch` con dos tipos
separados por `|`.

**Los marcos del verificador**: cada entrada de manejador lleva **el tipo declarado en el `catch`** sobre la
pila y el `any` lleva `Throwable` —`javap -v` de `Fin.multi`: `same_locals_1_stack_item` con
`ArithmeticException`, `RuntimeException` y `Throwable`, y `same` en la salida (pc 50)—. `apkc/jvmframes.go` ya
unía esos estados desde el párrafo 71; no hizo falta cambiarlo.

**En el dex** no hizo falta escribir nada nuevo: `armaTries` (párrafo 71) ya agrupa por rango las entradas del
mismo `try`, ya escribe el `catch_all` con el tamaño en negativo y ya acepta varios `try_item` por método. Lo
que aporta el fuente son los cuerpos de los `catch` protegidos y los huecos de local del temporal, que suben
`registers`. Nuestro dex de `Fini` (siete métodos, dos con tres manejadores) es **byte a byte** el de javac:
**1452 = 1452 bytes, `cmp` mudo**.

**Ejecutado**: el banco lleva `Ver.finalmente(5)` (camino normal, 22) y `Ver.finalmente(0)` (atrapa la división
por cero, 7); los cinco caminos y la JVM con **nuestra clase emitida** bajo `-Xverify:all` dan
`total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306|22|7` con `rc 102`, y `dexdump` del aparato verifica los
cuatro dex con `rc 0`.

**Pruebas**: `apkc/javaemisor_finally_test.go` — compara método a método contra `javac --release 8
-g:lines,source` las instrucciones, la `exception_table`, la `LineNumberTable`, `max_stack` y los huecos de
local; exige el dex idéntico byte a byte; y fija los cuatro rechazos (valor pendiente del `return`, valor
pendiente del `break`, `throw` último con `finally`, `catch` con `|`). Se fue
`TestElTryDelFuenteRechazaLoNoMedido`, que reclamaba como no medidos los dos casos que este párrafo midió.

Suite: **248 PASS + 2 SKIP** nativa y **196 PASS + 54 SKIP** en el hospedador.

**Frontera**: el valor pendiente que javac guarda cuando un `return`, un `break` o un `continue` **sale** de un
`try` con `finally` (medido en `Fin2`, no escrito); el `throw` como última sentencia del cuerpo de un `try` con
`finally` (el rango se pasa al propio manejador); `catch (A | B e)`; y `try`-with-resources.

## 76. El valor pendiente: un `return` o un `break` que cruza el `finally`

Dos sondas más (`Fin5`: un `return` en una rama del cuerpo y un `break` dentro de un bucle con `finally`;
`Fin6`: el `return` de un `void`, el mismo en una rama, un `finally` anidado y un `return` en la rama `else`),
con `javac --release 8 -g:lines,source` y `javap -c -p -l`.

**El cuerpo deja de ser un rango único**: cada salida abrupta **cierra un trozo** y el siguiente trozo empieza
**después** de la copia, del recargado y del `return` (o del `goto`). Medido en `Fin5.rama`
(`try { if (n > 0) { return 1; } n = n + 1; } finally { a = 2; }`):
`0 iload_0; 1 ifle 12; 4 iconst_1; 5 istore_1; 6 iconst_2; 7 putstatic; 10 iload_1; 11 ireturn;
12 iload_0; 13 iconst_1; 14 iadd; 15 istore_0; 16 iconst_2; 17 putstatic; 20 goto 30` con la tabla
`0..6 → 23 any` y `12..16 → 23 any`. El primer rango termina en 6 —justo después del `istore` que guarda el
valor— y el segundo **empieza en 12**, no en 0: la copia y el `return` del camino primero no están protegidos
(si estuvieran, el `finally` correría dos veces). Lo mismo con el `break` (`Fin5.breakConFinally`:
`6..11 → 29 any` y `18..22 → 29 any`, con el `goto 43` del `break` en 15) y con el `void`
(`Fin6.ramaVoid`: `0..4 → 22 any` y `10..14 → 22 any`).

**El hueco del valor**: `reserva` sin nombre, en el hueco que toca (`Fin5.rama`: `istore_1` con `n` en el 0;
`Fin2.retornoConCatch`: valor del `try` en 1, excepción del `catch` en 1, valor del `catch` en 2 y el temporal
del `any` en 3 —el más alto que vio el método, regla del párrafo 75—). Un `return;` de `void` no deja valor y
no usa hueco: la copia va primero y el `return` después (`Fin6.ramaVoid`: `4 bipush 6; 6 putstatic; 9 return`).

**Las filas de línea**: el `goto` de un `break` que cruza el `finally` lleva **la línea del `break`** (medido
en `Fin5.breakConFinally`: `21 en 15`, y en `Fin2.ruptura`: `54 en 9`), mientras que el `goto` del camino que
cae lleva la de la **llave del `finally`** (párrafo 75). El `return` pendiente marca dos veces su propia
línea: una en la evaluación/`istore` (pc 4 de `rama`) y otra en el `iload` (pc 10). En el `void` no hay fila
para el `return` hasta su instrucción —`Fin6.ramaVoid`: `115 en 4` (la copia) y `111 en 9` (el `return`), sin
fila del `return;` en 4—, así que la copia se escribe **antes** de marcar la línea de la sentencia.

**Un cuerpo que no deja nada protegido escribe en javac un manejador muerto**: `Fin6.soloVoid`
(`try { return; } finally { a = 5; }`) sale con la copia en 0, el `return` en 4, el `astore_0` en 5 y la
**tabla sin filas** —el `walkJvmCode` de este emisor llama la atención sobre el pc inalcanzable, así que el
caso se rechaza por nombre—. Con `catch` y `finally`, javac reparte los tipos por trozos cuando el cuerpo sale
por dos (otro rechazo con nombre).

**Anidado, medido y sin escribir**: `Fin6.anidado` (un `return` dentro de un `try` con `finally` dentro de otro
`try` con `finally`) copia los dos bloques seguidos —`6 bipush 7; 8 putstatic; 11 bipush 8; 13 putstatic;
16 iload_1; 17 ireturn`— y la tabla queda con `0..6 → 26 any`, `0..11 → 42 any` y `18..34 → 42 any`, es decir,
**rangos que se solapan**. En el dex los `try_item` no pueden solaparse, así que hay que repartirlos; este
emisor rechaza la forma por nombre.

**En el dex** no hizo falta tocar nada otra vez: los rangos se agrupan por `(inicio, fin)` y cada uno lleva su
lista de manejadores. El dex de la sonda ampliada (**doce métodos**, con `retorno`, `retornoConCatch`, `rama`,
`ramaVoid` y `breakConFinally`) sale **byte a byte** el de javac: **2052 = 2052 bytes**.

**Ejecutado**: el banco añade `Ver.salidaPendiente(5)` (el `return` pendiente, 10) y `Ver.salidaPendiente(0)`
(la caída, 101); los cinco caminos y la JVM con **nuestra clase** bajo `-Xverify:all` dan
`total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306|22|7|10|101` con `rc 102`, y `dexdump` del aparato verifica
los cuatro dex con `rc 0`.

**Pruebas**: `apkc/javaemisor_finally_test.go` compara ahora doce métodos —instrucciones, `exception_table`,
`LineNumberTable`, `max_stack` y huecos de local—, exige el dex idéntico, y fija siete rechazos: el
`continue` que sale, un `return`/`break` que cruza otro `try` anidado, uno dentro del propio `finally`, el
`throw` último del cuerpo y del `catch`, el cuerpo que sale por dos trozos con `catch`, el cuerpo sin nada
protegido y el `catch (A | B e)`.

Suite: **248 PASS + 2 SKIP** nativa y **196 PASS + 54 SKIP** en el hospedador.

**Frontera**: los dos `finally` anidados con una salida pendiente (los rangos se solapan y el dex exige
repartirlos), el `continue` que sale de un `try` con `finally`, el `catch (A | B e)`, `try`-with-resources, y
lo que seguía del 75: `float`/`double`, las ramas sobre `long` (`lcmp`, `dup2`/`pop2`), los arreglos de largos y
el `ConstantValue` de un `static final long`.

