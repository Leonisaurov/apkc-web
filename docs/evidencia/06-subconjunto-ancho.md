# Evidencia — §77-§92

## 77. El `float` y el `double` en el `.class`: un hueco contra dos, y los tags 4 y 6 del pool

**El léxico era la frontera real**, no el emisor: el lector de números paraba en los dígitos y las letras
`a`–`f`, así que `2.5` salía como `2`, `.`, `5` («tras el punto se esperaba un nombre y hay "5"»). Se le
añade el punto decimal —punto seguido de dígito, y después lo que ya admitía el bucle (dígitos, letras y el
sufijo `f`/`d`)—. El exponente con signo (`1e-3`) sigue sin armarse, porque el `+`/`-` no entra en el token, y
el hexadecimal con `p` se rechaza con su nombre.

**Los *tags*, medidos en los bytes de `FyD.class` y no en la memoria**: se busca la secuencia del `2.5`
(`40 04 00 00 00 00 00 00`) y el byte anterior es **0x06**; la del `1.5f` (`3F C0 00 00`) va precedida de
**0x04**. O sea `Float` = tag 4 con cuatro bytes y **una** posición, `Double` = tag 6 con ocho y **dos**, como
la `Long` tag 5 del párrafo 74. La tabla del JVMS confirmada a mano: `Integer` es 3 (el que ya usábamos).

**Un hueco contra dos, con `javap -c`**: `resta(float a, float b)` es `fload_0; fload_1` —dos flotantes en
huecos consecutivos— mientras `suma(double a, double b)` es `dload_0; dload_2`, y `por(float a, double b)`
pone `a` en 0 y `b` en 1-2. El `esAncho` del párrafo 74 contaba el `F` como ancho: estaba mal y no se
ejercitaba porque el guardia lo rechazaba; **ancho es `J` y `D`**, el `float` vale un hueco y una palabra.

**Las cuentas y la promoción**, leídas de `FyD`: `fadd` 0x62, `fsub` 0x66, `fmul` 0x6a, `fdiv` 0x6e,
`frem` 0x72 y `dadd` 0x63, `dsub` 0x67, `dmul` 0x6b, `ddiv` 0x6f, `drem` 0x73 —cada familia va justo después
de la entera y la larga, que ya estaban medidas (0x60/0x61, 0x64/0x65, …)—. La conversión va **en el sitio del
operando**: `por` es `fload_0; f2d; dload_1; dmul`, `divide(double a, int b)` es `dload_0; iload_2; i2d; ddiv`
y `campoEstatico` (`dd + ff`) es `getstatic dd; getstatic ff; f2d; dadd`. La promoción de Java es
**doble > flotante > largo > entero**, así que `float + long` da flotante: por eso `tipoPrevisto` devuelve el
tipo promocionado en vez de mirar solamente el `J`.

**Las constantes cortas**: `0.0f` es `fconst_0` (0x0b) y `1.0` es `dconst_1` (0x0f), medido en `deEntero`
(`x + 1.0` sale `dconst_1`); lo demás va por pool —`3.14159` con `ldc2_w`, `1.5f` con `ldc #25`—. El menos se
pliega en la constante: `negativo()` es `ldc2_w #21 // double -2.5d`.

**Retornos, cargas y guardas**: `freturn` 0xae, `dreturn` 0xaf; `fload` 0x17 con `fload_0..3` 0x22..0x25,
`dload` 0x18 con `dload_0..3` 0x26..0x29, `fstore` 0x38 con `fstore_0..3` 0x43..0x46 y `dstore` 0x39 con
`dstore_0..3` 0x47..0x4a. La rejilla de conversiones es `i2l` 0x85, `i2f` 0x86, `i2d` 0x87, `l2i` 0x88,
`l2f` 0x89, `l2d` 0x8a, `f2i` 0x8b, `f2l` 0x8c, `f2d` 0x8d, `d2i` 0x8e, `d2l` 0x8f, `d2f` 0x90 —anclada en los
dos valores medidos del párrafo 74 y confirmada instrucción a instrucción contra `FyD`—. Un cast numérico
**no** es `checkcast`: `(int) d` sale `d2i` y `(float) d` sale `d2f`.

**Los marcos del verificador**: `ITEM_Float` es 2 y `ITEM_Double` 3 (el `ITEM_Long` 4 del párrafo 74 sigue
valiendo), y el doble entra en el marco con **una** entrada para los dos huecos, igual que el largo. La
pila simulada cuenta palabras: el `double` suma dos y el `float` una (`altoPilas`).

**Ejecutado**: un conductor compilado con javac (`Driver`, que suma los trece métodos e imprime) corre
contra **nuestra** `.class` con `java -Xverify:all` y da el mismo texto que contra la de javac: `36.39159` y
`1.5|2.5`, `rc 0` en los dos caminos. Nuestro `FyD.class` son 1181 bytes.

**Frontera por nombre, en aquel momento**: el traductor se paraba en `dload_0` (`outside the measured subset`)
—las formas de dalvik para el flotante y el doble entraron midiendo el fichero de `d8` en el párrafo 80—;
`faload`/`fastore` (un arreglo de `F` o de `D` sale con «un arreglo de F no está medido»); las comparaciones
sobre flotantes o dobles (`>` entre `D` y `D`); los casts a `byte`, `short` y `char`
(`i2b`/`i2c`/`i2s`, sin medir) y el exponente con signo.

**Pruebas**: `apkc/javaemisor_flotante_test.go` — tres pruebas: compara los trece métodos uno a uno contra
`javac --release 8 -g:lines,source` (instrucciones, `max_stack`, huecos de local y filas de línea), exige el
dex idéntico entre nuestras clases y las de javac, y fija tres rechazos. Cayeron dos rechazos que ya no son
ciertos —`double d = 1.0` en la prueba del largo y el del léxico sobre `1.0`— y la prueba del caminante
cambió de byte (0x6a ya es `fmul`; ahora es 0x94, `lcmp`).

Suite: **251 PASS + 2 SKIP** nativa y **196 PASS + 57 SKIP** en el hospedador (las tres pruebas nuevas se
saltan si el hospedador no lleva `android.jar` montado).

**El entorno se rompió con el reset de fábrica del aparato, no el repo.** La sesión pasó a otro usuario de
Android (`u0_a237`, antes `u0_a379`) y los enlaces de `~/Android/build-tools/34.0.0/`
(`aapt`, `aapt2`, `apksigner`, `d8`, `zipalign`) quedaron con el contexto SELinux del usuario anterior:
`ls -Z` del objetivo sano muestra `u:object_r:app_data_file:s0:c237,…`, y sobre el enlace viejo ni
`readlink`, ni `stat`, ni `unlink` —todo `Permission denied`—. Con eso el gate daba `aapt2: permission denied`
y el primer `e2e` de este párrafo salió con dos problemas.

**Reparación medida**: los binarios existen como paquetes de Termux en `$PREFIX/bin/` (`pacman -Q`:
`aapt2 16.0.0.4-4`, `d8 37.0.0-0`) y corren desde ahí —`aapt2 version` → `2.20-android-16.0.0_r4`,
`d8 --version` → `D8 9.2.4-dev`—. Se creó `~/Android/build-tools/34.9.0/` con enlaces **nuevos** a esos
binarios (los enlaces nuevos nacen con el contexto actual y sí funcionan) y `banco/banco.sh` ahora elige la
carpeta de build-tools más alta igual que `studio/e2e.sh`, en vez de una ruta fija. Primer intento con
`35.0.0` y el build falló pidiendo `platforms/android-35/android.jar`: **apkc deriva la platform del número
de build-tools**, y en este SDK solo hay `android-34`; renombrada a `34.9.0`, `apkc doctor` vuelve a resolver
`platforms/android-34/android.jar` ✓. Con eso el gate vuelve a estar **en orden** (el apk del contador,
10287 bytes, verificado por `apksigner`, `zipalign`, `zipcheck.py`, `aapt2 dump` y releído por `d8`).

**Lo que falta en esta sesión**: `rish` desapareció con el reset (`command -v rish` vacío), así que
`banco/banco.sh` no puede correr sus caminos de aparato (`dexdump` y `app_process64` del teléfono). La
evidencia de ejecución del punto flotante, mientras, es la de la JVM real: `-Xverify:all` sobre nuestra clase,
que ya está arriba.

## 79. El `instanceof`, y el oráculo de `d8` sin aparato: `instance-of` es 0x20 en forma 22c

**El `.class`, con `javap -c -p -l` sobre `Ins`**: `static int esCadena(Object x) { if (x instanceof String) { return 1; } return 0; }` es
`0 aload_0; 1 instanceof #7 // class java/lang/String; 4: ifeq 9; 7 iconst_1; 8 ireturn; 9 iconst_0; 10 ireturn`.
Tres cosas: el `instanceof` consume la referencia y **deja un entero** (el alto de pila no cambia), la
condición del `if` es el `ifeq` **sin invertir** —al revés que las seis comparaciones del párrafo 52, que
salen invertidas—, y el indice es de **dos bytes** (`instanceof` = 0xc1, junto al `checkcast` 0xc0 y el
`multianewarray` 0xc2, este ultimo fuera del subconjunto). La filas de línea del `if` son las de la
sentencia (`line 5: 0`, `line 6: 7`, `line 8: 9`) —no hay fila para el `ifeq`—, y el marco del destino es
un `same_locals_1_stack_item`… aqui `same`, porque en el `else` la pila ya está vacía: `javap -v` muestra
`stack=1, locals=2` y un unico marco `same` en el pc 9.

**El dex, medido sobre el fichero de `d8` y no sobre un manual** (sin `rish` hace falta un oráculo local, y
`d8` lo da: escribe el dex, y las **palabras** las leo con nuestro lector `readDex`, que no comparte código
con el traductor porque no lo interpreta —solo corta `code_item`—). `d8 --debug` sobre `Ins.class` deja en
`esCadena` las palabras `32 4 | 56 4 | 4114 15 | 18 15`, y en `esSecuencia` `32 2 | 56 4 | 8210 15 |
12306 15`. Decodificadas con la tabla ya medida en el repo (`check-cast` 0x1f, `array-length` 0x21,
`new-array` 0x23, `if-eqz` 0x38, `const/4` 0x12, `return` 0x0f):

- `32` = **0x20**, el hueco que queda entre `check-cast` y `array-length`, con la forma 22c: el primer
  registro en el nibble bajo del byte alto y el segundo en el alto (`0x0020` → `v0, v0`), y el `type@CCCC`
  en la unidad siguiente (`4` = `Ljava/lang/String;`, `2` = `Ljava/lang/CharSequence;` ✓ los indices
  coinciden con los del `instanceof` del `.class`).
- `4114` = `0x1012` = `const/4 v0, #1` ✓ (el literal en el nibble alto: `0x2012` → `#2` en `esSecuencia` y
  `0x3012` → `#3` ✓ las dos constantes del fuente), y `15` = `0x000f` = `return v0` ✓.

Para fijar el orden de los nibbles hace falta un caso con **destino y origen distintos**, y lo di
`Ins2.f(Object a, Object b)` con `if (b instanceof CharSequence)`: `d8` escribe `4128 2` =
`0x1020` = `instance-of v0, v1, type@2` ✓ —destino v0 (nibble bajo del byte alto), origen v1 (nibble
alto)—. Por eso el traductor pide un **temporal** para el destino en vez de escribir sobre el de la
referencia: la referencia probada puede seguir viva despues de la condicion (`if (b instanceof X) { usa(b); }`).

**Añadido al emisor y al traductor**: el analizador lee `instanceof` con la precedencia de las relacionales
(nivel 4, no asociativo); el emisor escribe `0xc1` con el indice de la clase en el pool (la misma `usar` +
`pool.clase` del `checkcast`, así que la clase probada entra en `InnerClasses`-like como tipo usado);
`condicionBranca` devuelve `opIFEQ` con una palabra consumida; la tabla del caminante lleva
`0xc1: {"instanceof", 2}`; el interprete de marcos saca la referencia y empuja un entero; y el traductor emite
`unidad12x(0x20, temporal, origen)` + el indice de tipo.

**Comparados con javac, los cinco métodos**: `esCadena`, `esSecuencia`, `conArreglo` (el `instanceof` dentro
del bucle sobre `vs[i]`, que es donde el temporal importa), `baja` (el `checkcast` de siempre) y `anidado`.
Las instrucciones, `max_stack`, los huecos de local y las filas de línea salen **iguales**, y el dex que
escribe este fork sobre nuestras clases es **byte a byte** el que escribe sobre las de javac.

**Ejecutado en la JVM real**: un conductor compilado con javac (`InsDriver`, con `new Object[] { "hola", 3, "adios" }`)
corre contra nuestra clase con `-Xverify:all` y da el mismo texto que contra la de javac:
`1|0|2|3|2|cadena|5`, `rc 0` en los dos. **Lo que no se pudo comprobar en esta sesión es el `instance-of` en
ART**, porque `rish` no está: falta el `dexdump` del aparato y el `app_process` del banco.

**Rechazado por nombre, con la forma medida**: la condición negada (`if (!(x instanceof T))`, que en javac
usa el `ifne` y no la leímos), `x instanceof String[]` (el tipo de arreglo en el pool) y `instanceof` sobre
un primitivo (que en Java ni compila, pero el mensaje sale de nuestro portero).

**Pruebas**: `apkc/javaemisor_instof_test.go` — tres pruebas (comparación con javac método a método, dex
idéntico, y los tres rechazos). La prueba del caminante que fijaba `0x6a` como byte no medido ya no podia
usarlo (`fmul`, párrafo 77) y pasó a `0x94`, `lcmp`.

Suite: **254 PASS + 2 SKIP** nativa, **196 PASS + 60 SKIP** en el hospedador y `studio/e2e.sh` **en orden**
(el apk del proyecto generado, verificado por `apksigner`, `zipalign`, `zipcheck.py`, `aapt2 dump` y releído
por `d8`).

**Frontera**: el `instanceof` en ART hasta que vuelva `rish`; `x instanceof T[]`; la forma negada; y lo que
seguía: las ramas sobre `long` (`lcmp`, `dup2`/`pop2`) y los arreglos de largos o flotantes. El punto flotante
en el dex entró después, en el párrafo 80.

## 81. Las comparaciones anchas: `lcmp`, `fcmpl`/`fcmpg`, `dcmpl`/`dcmpg`, medidas las cinco

Dos sondas (`Ram`: `x > 0L`, `x == y` de largos, `x > y` y `x != 0.0` de dobles, `x <= y` de flotantes;
`Ram2`: `x < y` de dobles, `x == y` de dobles, `x >= y` y `x < y` de flotantes, `x < y` de largos), leídas con
`javac --release 8 -g:lines,source`, `javap -c` y —para el dex— `d8 --debug` sobre la clase de javac con las
palabras cortadas por `readDex`, igual que en el párrafo 80.

**En el `.class` la inversión del párrafo 52 se mantiene tal cual**, con el resultado de la comparación en la
pila como un entero:

| fuente | bytecode |
| --- | --- |
| `if (x > 0L)` | `lload_0; lconst_0; lcmp; ifle` |
| `if (x == y)` (largos) | `lload_0; lload_2; lcmp; ifne` |
| `if (x < y)` (largos) | `lload_0; lload_2; lcmp; ifge` |
| `if (x > y)` (dobles) | `dload_0; dload_2; dcmpl; ifle` |
| `if (x != 0.0)` | `dload_0; dconst_0; dcmpl; ifeq` |
| `if (x <= y)` (flotantes) | `fload_0; fload_1; fcmpg; ifgt` |
| `if (x >= y)` (flotantes) | `fload_0; fload_1; fcmpl; iflt` |
| `if (x < y)` (dobles) | `dload_0; dload_2; dcmpg; ifge` |

O sea: **`…l` con las relaciones que javac escribe al derecho (`>`, `>=`, `!=`, `==`) y `…g` con las que
escribe invertidas (`<`, `<=`)** —es la regla de NaN del JVMS 2.9, no una elección de javac: `dcmpl` deja -1
con un NaN y `dcmpg` deja 1, y cada una se combina con la branquia que ya estaba medida—. El `lcmp` no tiene
hermano porque un largo no puede ser NaN.

**En el dex**, las cinco formas y el formato 23x (`op|AA`, `BB|CC<<8`), con la branquia `if-*z` encima del
resultado —d8 no emite `if-eq` contra cero, sino `if-ltz`/`if-gez`/`if-gtz`/`if-lez`, que ya estaban medidos
(0x38..0x3d)—:

| método | palabras | lectura |
| --- | --- | --- |
| `fMayorIgual` (`fcmpl`) | `45 256 \| 58 4` | `0x2D cmp…` → **`fcmpl` = 0x2d**, `if-gez v0, +4` (0x3a) |
| `fMenor` (`fcmpg`) | `46 256 \| 59 4` | **`fcmpg` = 0x2e**, `if-ltz v0, +4` (0x3b) |
| `dIgual` / `doubleMayor` (`dcmpl`) | `47 512 \| 57 4` y `47 512 \| 61 4` | **`dcmpl` = 0x2f**, `if-nez` (0x39) y `if-lez` (0x3d) |
| `dMenor` (`dcmpg`) | `48 512 \| 59 4` | **`dcmpg` = 0x30**, `if-ltz` |
| `iguales` / `mayorQue` / `lMenor` (`lcmp`) | `49 512 \| 57 4`, `… 561 2 \| 573 4 …`, `49 512 \| 59 5` | **`lcmp` = 0x31**, con `if-nez`, `if-lez` y `if-ltz` |

Cada número sale de dos o tres casos independientes y coincide ✓; el bloque es contiguo 0x2d..0x31 y
remata en `if-eq` 0x32, que estaba medido en el párrafo 52. El operando del `cmp-long`/`cmpg-double` es la
**pareja** de cada lado (`512` = `0x0200` → `v0` y `v2`) y el destino es un registro suelto, con lo que un
método `double(double,double)` con `registers=4, ins=4` escribe el resultado encima de un operando muerto,
como ya hacía con las cuentas `/2addr` (párrafo 66).

**Ya escrito** (2026-11-03): el emisor acepta la comparación sobre `J`/`F`/`D` —promociona los dos lados
como en las cuentas, emite `lcmp`/`fcmpl`/`fcmpg`/`dcmpl`/`dcmpg` y encima la branquia contra cero de la
tabla de arriba, sin ahorrarse el literal cero— y el traductor conoce los cinco opcodes. La sonda `Ram` (ocho
métodos) sale con el mismo `Code` que la de javac y su dex **byte a byte** el suyo (1192 bytes); `dexdump -d`
lee `cmpl-double` 0x2f con `if-lez`/`if-eqz`, `cmpg-double` 0x30 con `if-gez`, `cmpl-float` 0x2d con
`if-ltz`, `cmpg-float` 0x2e con `if-gtz` y `cmp-long` 0x31 con `if-nez`/`if-lez`/`if-gez`. Ejecutado en el
aparato por el camino «todo nuestro» (`apkc compile` + `apkc dex`, sin javac ni d8), un driver con las ocho
relaciones imprime `1 2 1 2 …` igual que la JVM, con `dexdump` verificando `rc 0`; el banco lleva
`Ver.compara(2)` (devuelve 6) y los cinco caminos coinciden. `javaemisor_compara_test.go` fija el par
`cmp`+branquia de cada método y la identidad del `.class` y del dex contra javac.

Lo que sigue fuera, con la forma vista: `dup2`/`pop2` (el `access$` de un campo ancho, párrafo 72) y los
arreglos de largos y flotantes.

## 80. Las formas dalvik del `float` y el `double`, medidas sobre el fichero de `d8` —y ya implementadas

Sin `rish` no hay `dexdump` en el aparato, pero el oráculo sigue siendo ajeno: **`d8 --debug` escribe el
dex y las palabras las corta `readDex`**, que no comparte nada con el traductor (no interpreta opcode).
Sonda: `FyD` del párrafo 77, `d8 --debug --min-api 28 --lib platforms/android-34/android.jar` sobre la
clase de `javac` (1728 bytes), y las listas de palabras por método:

| método | palabras (`d8`) | lectura |
| --- | --- | --- |
| `suma(double a, double b)` | `0x20CB 0x0010` | `add-double/2addr v0, v2` + `return-wide v0` |
| `resta(float a, float b)` | `0x10C7 0x000F` | `sub-float/2addr v0, v1` + `return v0` |
| `por(float a, double b)` | `0x2089 0x30CD 0x0010` | `float-to-double v0, v2`, `mul-double/2addr v0, v3`, `return-wide v0` |
| `divide(double a, int b)` | `0x4083 0x02CE 0x0210` | `int-to-double v0, v4`, `div-double/2addr v2, v0`, `return-wide v2` |
| `constante()` (3.14159) | `0x0018 0x866E 0xEFFB 0x21F9 0x4009 0x0010` | `const-wide v0, #+0x400921F9EFFB8669` (los ocho bytes en little-endian, cinco unidades) + `return-wide` |
| `constanteCorta()` (0.0f) | `0x0012 0x000F` | `const/4 v0, #0` + `return v0` —un `0.0f` es un entero de una palabra— |
| `deEntero(int n)` | `0x4083 0x0219 0x3FF0 0x20CB 0x0010` | `int-to-double`, **`const-wide/high16 v2, #+0x3FF0`** (= 1.0, dos unidades), `add-double/2addr v0, v2`, `return-wide v0` |
| `aEntero(double d)` | `0x008A 0x000F` | `double-to-int v0, v0` + `return v0` |
| `aFlotante(double d)` | `0x008C 0x000F` | `double-to-float v0, v0` + `return v0` |
| `aDoble(float f)` | `0x2089 0x0010` | `float-to-double v0, v2` + `return-wide v0` |
| `campoEstatico()` (`dd + ff`) | `0x0061 0x0000 0x0260 0x0001 0x2289 0x20CB 0x0010` | `sget-wide v0, field@0` (el `double`), **`sget v2, field@1`** (el `float` va por la forma entera), `float-to-double v2, v2`, `add-double/2addr v0, v2`, `return-wide v0` |
| `negativo()` (-2.5) | `0x0019 0xC004 0x0010` | `const-wide/high16 v0, #+0xC004` + `return-wide v0` |

**Lo que fija la tabla**, anclado en lo ya medido (`const-wide` 0x18, `const-wide/16` 0x16, `sget-wide` 0x61,
`return-wide` 0x10, `move`/`move-wide` 0x01/0x04, `add-long/2addr` 0xbb y su bloque 0xbb..0xbf, `add-int`
0x90..0x94, `move-result` 0x0a):

- **Conversiones**: el bloque es la rejilla del `.class` menos cuatro —`int-to-long` 0x81 y `long-to-int` 0x84
  (párrafo 74) encajan con `int-to-double` 0x83 (`divide`, `deEntero`), `float-to-double` 0x89 (`por`,
  `aDoble`, `campoEstatico`), `double-to-int` 0x8A (`aEntero`) y `double-to-float` 0x8C (`aFlotante`)—. Los
  seis restantes de ese bloque (`int-to-float` 0x82, `long-to-float` 0x85, `long-to-double` 0x86,
  `float-to-int` 0x87, `float-to-long` 0x88, `double-to-long` 0x8B) salen del mismo paso y quedan por ver.
- **Cuentas `/2addr`**: el bloque de `float` es 0xC6..0xCA (`sub-float/2addr` 0xC7 por `resta`) y el de
  `double` 0xCB..0xCF (`add` 0xCB, `mul` 0xCD, `div` 0xCE por `suma`, `por`, `divide`) —después del de
  `long` 0xBB..0xBF, cinco posiciones cada uno porque no hay bit a bit sobre flotantes. Las formas 23x
  (`add-float` 0xA6.. y `add-double` 0xAB..) se infieren del mismo salto y **no** están vistas aún.
- **Constantes**: `0.0f` se escribe como `const/4 v0, #0` (una palabra), `1.0` y `-2.5` caben en
  `const-wide/high16` (**0x19**, dos unidades: `op|AA` + los dieciseis bits altos) y `3.14159` va por
  `const-wide` 0x18 en cinco unidades. Nuestro traductor no tiene por qué imitar la elección: basta con que
  emita una forma válida; la igualdad que exigimos es entre **nuestro** dex sobre nuestras clases y **nuestro**
  dex sobre las de javac (párrafo 74).
- **Locales y campos**: el `float` usa las formas de una palabra (`sget`/`sput`, `move`, `return`,
  `aget`/`aput`) y el `double` las anchas ya medidas (`sget-wide` 0x61, `move-wide` 0x04, `return-wide` 0x10,
  `iget-wide` 0x53 / `iput-wide` 0x5a). Un local `float` o `double` cargado desde el sitio que le da la
  frame no emite nada, como con el `long` ✓ (en `suma` y `resta` no hay ninguna instrucción de carga).

**Implementado sobre esta medición** (y volviendo a cambiar `TestElFlotanteSeDexaComoJavac` por la
comparación de bytes): el traductor acepta `fload`/`dload` (con sus breves), `fstore`/`dstore`, las cinco
cuentas de cada tipo en forma `/2addr`, las conversiones `i2f`/`i2d`/`f2d`/`d2i`/`d2f`/`f2i` (más las que
conservan palabra), `fconst_*`/`dconst_*`, el `ldc` de una entrada `Float` y el `ldc2_w` de una `Double`,
el `freturn`/`dreturn` y el acceso a campos de `float` por la forma de una palabra (`sget`/`sput`/`iget`/`iput`,
como `d8`) y a los de `double` por la ancha ya medida. **El dex de `FyD` (trece métodos) sale byte a byte el
mismo escrito sobre nuestras clases que sobre las de javac**, y la frontera que queda es concreta: la forma de
**tres registros** de una cuenta ancha de `float`/`double` (ninguna sonda la escribió; se rechaza con el nombre
del opcode), `faload`/`fastore` y `aget-wide`/`aput-wide` (arreglos de flotantes), `l2f`/`l2d`/`f2l`/`d2l`
(inferidas del mismo paso, sin sonda). La comprobación en ART llegó en el párrafo 82, cuando el aparato se
alcanzó con `sudo`.

## 82. El banco en ART sin `rish`: `float`, `double` e `instanceof` ejecutados, y los tres defectos que salieron

Al volver el root al aparato, el banco se alcanza con `sudo` en vez de `rish` (Shizuku desapareció con el
reset; `banco/banco.sh` elige uno u otro con `enAparato`). Con eso, la sonda del banco —que ya llevaba
`finalmente`, `salidaPendiente` y el `long`— ganó lo que faltaba del §79 y del §80 y se pudo **ejecutar**:

```java
static double doble = 2.5;
static float flotante = 1.5f;
public static int flotantes(int n) {
    double d = n * doble + flotante;
    long l = (long) d;
    Object o = "cadena";
    if (o instanceof String) { l = l + 1; }
    if (o instanceof CharSequence) { l = l + 1; }
    return (int) l;
}
public static double dobleCon(int n) { return n * 1.5; }
```

```sh
sh -c 'APKC="$PREFIX/tmp/apkc-banco" sh banco/banco.sh'
# jvm (javac): rc=102, salida: total 7|…|306|22|7|10|101|8|3.0
#   nuestro dex (Go de punta a punta): rc=102 y salida igual a la JVM  ok
#   dex de d8 sobre javac: rc=102 y salida igual a la JVM  ok
#   dex de d8 sobre nuestra Ver: rc=102 y salida igual a la JVM  ok
#   nuestro dex con nuestro main (sin javac ni d8): rc=102 y salida igual a la JVM  ok
# jvm con nuestra clase emitida (Xverify:all): rc=102 y salida igual a la JVM  ok
# banco: en orden
```

Los cuatro dex dan `dexdump` verificando `rc 0`, y el camino «todo nuestro» —las dos fuentes compiladas y
dexadas por este fork— imprime `…|101|8|3.0`: el `8` es `flotantes(2)` (el `double`, el `float`, el `long`,
las dos `instanceof` y las dos sumas) y el `3.0` es `dobleCon(2)`, o sea un `return-wide` leído por
`println(D)V`. Los cinco caminos coinciden.

**Tres defectos, y los tres solo los vio el aparato** (ni `d8`, ni la identidad byte a byte, ni la JVM con
nuestro `.class`):

1. **`unexpected static field initial value type: 'L' vs 'D'`.** El `encoded_value` de un estático tiene que
   ser del tipo del campo. El emisor ya lo hacía para `I` (párrafo 55) y `J` (74), pero un `double`/`float`
   sin `ConstantValue` caía en `valueNull`, y ART cortaba la clase entera antes de ejecutar nada. Se emite el
   cero de la anchura del tipo —`valueDouble | 7<<5` (0x11, ocho bytes) y `valueFloat | 3<<5` (0x10, cuatro)—,
   que el `<clinit>` sobrescribe al correr. De paso, `valorEstatico` ya emite la constante de un
   `static final double`/`float` (el `.class` la trae y el lector guardaba los bits en `ConstLong`/`ConstInt`):
   `dexdump` de nuestra sonda `Fd` lee `FD = 2.5` y `FF = 1.5`. `TestLosEstaticosAnchosLlevanSuTipo` lo fija
   leyendo el `encoded_array_item` con el lector propio.

2. **`instance-of on unexpected class Integer`.** `collectCode` no seguía `instanceof` (ni `checkcast`): la
   clase del `instanceof` solo entraba en `type_ids` si el fichero la volvía a nombrar en otra parte (un
   campo, una llamada, un descriptor). Con `o instanceof CharSequence` en una clase que no la nombra, el
   mapa devolvía el cero de la clave ausente y la instrucción salía `instance-of v5, v4, I` —ART la leía
   como «probar contra `int`» y rechazaba el método—. El test destapó que el fallo era *silencioso*: ahora
   `typeIdxDe` rechaza con nombre («the type_id … was never interned») en las cuatro instrucciones que
   llevan tipo (`new`, `new-array`, `check-cast`, `instance-of`), y el colector interna los dos operandos.
   `TestElTipoDelInstanceOfSeInterna` comprueba que los dos `instance-of` de la sonda apuntan a
   `Ljava/lang/String;` y `Ljava/lang/CharSequence;`; sin el arreglo el test falla con el rechazo nuevo.

3. **`it needs more wide temporaries than the max stack javap measured`, y después `register v16 does not fit
   a four-bit format`.** El marco de `main` se quedaba sin pares: un `double` vivo ocupa DOS registros, la
   pila de javac lo cuenta como dos palabras, pero dos temporales muertos sueltos no forman un par, y el
   reparto pedía siempre dos registros nuevos. Tres cambios, todos medidos por el efecto en el aparato:
   - `tomaPar` busca el **par consecutivo más bajo** de la bolsa libre, no solo las dos últimas entradas
     (con la bolsa `[0 2 3 4 5 6 7]` el par `(2,3)` existía y no se veía);
   - la pasada de medida **no es un rechazo**: `techo` solo lo pone la pasada que escribe las palabras, y sus
     números de registro son provisionales (una base conjetural empujaba `iload_0` a v16 y el propio
     instrumento de medida se caía);
   - la base se busca por **punto fijo**: se repite la medida subiendo la conjetura hasta que el pico cabe
     debajo, porque el plan de la pasada final depende de la base. `main` del banco pedía 11 temporales con
     una conjetura de 9; `Ver.flotantes` acabó con `registers=14` (base 8 y seis huecos) frente a los 6 de
     `d8` —el marco puede ser más grande, lo que no puede es pasar de v15 en un formato de cuatro bits—.
     `TestLaFrameAnchaCabe` exige `registers ≤ 16` en los dos métodos de la sonda.

**Y dos huecos del caminante** que solo aparecen con locales por encima del 3, como el `long` de esta sonda:
`lload` (0x16) y `lstore` (0x37) con el índice de un byte. `javap -c` sobre la sonda los enseña
(`lstore 4`, `lload 4`); sin ellos el caminante paraba en el primer `lstore 4`.
`TestElCaminanteConoceLosAnchosIndexados` fija las dos formas.

Comprobado además: `go test ./...` de punta a punta en el hospedador wasm (PASS) y `studio/e2e.sh` en orden
(41 comprobaciones en la pestaña; el proyecto generado y la app del contador siguen saliendo verificados por
`apksigner`, `zipalign`, `zipcheck`, `aapt2` y releídos por `d8`).

## 83. Los campos anchos privados: `dup2_x1`, `dup2`, `pop2`, y el orden de los `proto_ids` que ART exige

Una clase con `private long largo` y `private double doble` tocados desde una anónima
(`largo = largo + 1; doble = doble + 1.5;`) hace que javac escriba los accesores sintéticos del §72 con
las formas anchas. Medido con `javac --release 8 -g:lines,source` y `javap -c -p`:

```java
static long access$002(D, long);      // escribir
    Code:  0: aload_0  1: lload_1  2: dup2_x1  3: putfield #6  6: lreturn
static long access$000(D);            // leer
    Code:  0: aload_0  1: getfield #6  4: lreturn
static long access$002(long);         // el estático, en `E`
    Code:  0: lload_0  1: dup2  2: putstatic #1  5: lreturn
```

y el llamador anónimo tira el valor que devuelve la escritura con **`pop2`** (0x58):
`invokestatic D.access$002:(LD;J)J; pop2`. En el `.class` las tres formas nuevas son `dup2_x1` (0x5d),
`dup2` (0x5c) y `pop2` (0x58); `getfield`/`putfield` no cambian de opcode, el ancho lo dice el descriptor.

En el dex, `d8 --debug` sobre las mismas clases (`dexdump -d` del aparato) **no escribe nada** para el
`dup2_x1` ni para el `dup2` —las entradas comparten registros, como el `dup_x1` del §72— y el `pop2`
desaparece con la llamada, igual que el `pop` absorbido del §21. El accesor queda en `iput-wide` (0x5a) y
`return-wide` (0x10), y en `run` hay **dos** `move-result-wide` para cuatro `invoke-static`: las dos
lecturas guardan su resultado y las dos escrituras lo tiran.

**Cuatro defectos salieron ejecutándolo**, y ninguno lo veía `javap`:

1. `max_locals` del accesor contaba **argumentos** y no **huecos**: `(LD;J)J` pide tres y el accesor salía
   con `locals=2`, con lo que el traductor ponía `aload_0` en v-1 y el dex se rechazaba con
   `register v does not fit a four-bit format`. `javap -v` del nuestro y del de javac lo enseñan.
2. El número del `access$` se contaba sobre todo el proyecto: con `D` y `E` juntas, la segunda recibía
   `access$200`/`access$202` donde javac pone `access$000`/`access$002`. La posición es **por clase
   exterior**.
3. La contabilidad de pila del accesor no descontaba las dos palabras del `return` ancho (`el access$002
   deja 1 valores en la pila al cerrar`, que el emisor mismo detecta).
4. **El orden de los `proto_ids`**: ART cortaba el fichero con `Out-of-order proto_id arguments`. La regla
   medida es la de `d8` —por tipo de retorno y después por la **lista de parámetros** comparada por sus
   `type_idx`—, no por el shorty: `(LD;J)J` va antes que `(LF;)J` aunque sus shortys sean «JLJ» y «JL», que
   ordenarían al revés. Nuestro fichero ponía `(LF;)J` en medio (`5 (LD;)J`, `6 (LF;)J`, `7 (LD;J)J`) y el
   de `d8` dice `5 (LD;)J`, `6 (LD;J)J`, `7 (LF;)J`.

Con las cuatro correcciones, las cinco clases de la sonda salen con el mismo `javap -c` que las de javac
(solo cambian los números de pool) y su dex, escrito por este fork, verifica (`dexdump`, `rc 0`) y se
ejecuta en el aparato por el camino «todo nuestro» (`apkc compile` + `apkc dex`, sin javac ni d8):

```sh
$PREFIX/tmp/apkc-dup compile --android-jar $HOME/Android/platforms/android-34/android.jar -o $M/nuestro \
    $M/D.java $M/DM.java $M/E.java $M/F.java $M/G.java
$PREFIX/tmp/apkc-dup dex -o $M/todo.dex $M/nuestro/*.class
# /data/local/tmp: dexdump rc 0;  app_process64 p.DM  →  6 4.0   (la JVM dice lo mismo)
```

El banco lleva el caso en `Ver.anchoPrivado` (campo `private long ancho = 5L`, anónima que suma 2, devuelve
7) y los cinco caminos dan `…|6|7` con `rc 102`, incluida la JVM con nuestra clase emitida bajo
`-Xverify:all`. Tests: `javaemisor_privada_ancha_test.go` (el accesor con `iput-wide` y los dos
`move-result-wide` de cuatro llamadas; y el `.class` del emisor contra javac) y `dexprotos_test.go` (el
orden de los protos con el contraejemplo del shorty).

Lo que sigue fuera de este bloque: los arreglos de largos y de flotantes (`laload`/`lastore`,
`faload`/`fastore`, `aget-wide`/`aput-wide`) y el `ConstantValue` de un `static final long`.

## 84. Los arreglos de `long`, `float` y `double`, y el defecto de pares que solo ve ART

Una sonda con seis métodos (`long leeLargo(long[], int)`, `void escribeLargo(long[], int, long)`, y lo
mismo con `float` y `double`) medida con `javac --release 8 -g:lines,source`, `javap -c` y `d8 --debug`
desarmado con `dexdump -d` del aparato:

| fuente | `.class` | dex |
| --- | --- | --- |
| `v[i]` de un `long[]` | `laload` (0x2f) | `aget-wide v0, v2, v3` (**0x45**, 23x) |
| `v[i]` de un `float[]` | `faload` (0x30) | `aget v0, v0, v1` (**0x44**) |
| `v[i]` de un `double[]` | `daload` (0x31) | `aget-wide` 0x45 |
| `v[i] = x` de un `long[]` | `lastore` (0x50) | `aput-wide v2, v0, v1` (**0x4c**) |
| `v[i] = x` de un `float[]` | `fastore` (0x51) | `aput` (**0x4b**) |
| `v[i] = x` de un `double[]` | `dastore` (0x52) | `aput-wide` 0x4c |

El `float` va por las formas de una palabra y el `long`/`double` por las anchas, como en los campos y los
locales. En el `.class` las tres parejas de `*aload`/`*astore` son opcodes propios y el ancho lo dice el
descriptor del arreglo.

**Implementado y ejecutado**: el caminante conoce las seis formas, los marcos consumen y empujan lo que
toca (dos palabras en `laload`/`daload`, una en `faload`), el traductor baja a `aget-wide`/`aput-wide` con
la pareja como destino/valor (y suelta el par), y el emisor ya no rechaza `new long[n]`, `v[i]`, `v[i] = x`
ni `new T[] {…}` de esos tipos: escribe `newarray` con el atype de la tabla, `laload`/`lastore`, etc., y
contabiliza la pila con dos palabras. El camino «todo nuestro» (`apkc compile` + `apkc dex`, sin javac ni
d8) ejecuta en ART un driver con las seis operaciones y da `7 9 3 7 5 9`, lo mismo que la JVM, con
`dexdump` verificando `rc 0`.

**El defecto que destapó ART, y que ningún comparador de bytes veía**:

```
java.lang.VerifyError: ... [0x5F] wide register v7 has type Double (Low Half)/Double (Low Half)
```

El cuerpo era `move-result-wide v7` (el par es `v7,v8`), `int-to-double v8, v0` (el par es `v8,v9`) y
`mul-double/2addr v7, v8`: los dos pares se **solapaban en v8**. La causa estaba en el reparto: la pila
guarda solo el registro BAJO de un par, así que `suelta` liberaba la mitad alta de un par que seguía vivo
(`suelta(r+1)` no la veía en la pila), y `tomaPar` podía crear un par nuevo sobre esa mitad. Se corrigió
con tres piezas: `altos` (la mitad alta de un par vivo no se libera), `sueltaPar` que no devuelve nada
mientras el par siga en la pila, y `ocupado`, que hace que `toma`/`tomaPar` **salten** los registros
vivos al repartir un par nuevo (de dos en dos, para no partir pares).

**Lo que quedó pendiente entonces y se midió después**: el `Code` de la sonda del driver no era idéntico al
de javac porque javac pliega `fa * 2` a la constante `fconst_2` (2.0f) y este emisor escribía
`iconst_2; i2f` —cerrado en el §85—, y faltaban el test permanente de la sonda y el caso en el banco, que
al meterlo destapó dos defectos más (el `baja(3)` del inicializador ancho, §85, y el marco por encima de
v15, §86).

## 85. El literal se promociona **sobre la constante**, antes de elegir su forma

Sonda `Plega` (veinte métodos) medida con `javac --release 8 -g:lines,source Plega.java`, `javap -c -p
Plega.class` y el fixture en base64 dentro de `apkc/javaemisor_plega_test.go`. Para reproducir:

```sh
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/plega" &&
  javac --release 8 -g:lines,source Plega.java && javap -c -p Plega.class && base64 -w0 Plega.class'
sh -c 'cd apkc && go test -count=1 -run Plega .'
```

| fuente | `.class` que escribe javac | qué fija |
| --- | --- | --- |
| `f * 2` | `fload_0; fconst_2; fmul` | el 2 llega siendo flotante: no hay `i2f` |
| `f + 0.0f` / `f + 0` | `fconst_0` | da igual cómo esté escrito el cero |
| `f + 3` / `f * 100` | `ldc` con entrada `Float` (tag 4, **una** posición) | 3.0f y 100.0f no son forma corta |
| `f * -2` | `ldc` con `-2.0f` | el signo entra en la constante: no es `fconst_2; fneg` |
| `-0.0` / `-0.0f` / `0.0 * -1.0` | `ldc2_w` / `ldc` con el cero con signo | `dconst_0` es el cero **positivo**; el signo se mira |
| `d * 2` / `d + 2.0` | `ldc2_w` con `2.0d` | javac **nunca** usa `dconst_2` (0x10 existe, no lo escribe) |
| `d + 1` / `l + 1` / `l + 0` | `dconst_1` / `lconst_1` / `lconst_0` | la forma corta la decide el valor ya promocionado |
| `l + 2` / `l * 3` | `ldc2_w` | no existe `lconst_2` |
| `i + 100` / `i + 3000` / `i + -1` | `bipush` / `sipush` / `iconst_m1` | la regla del §53 sigue entera |
| `2.5f * 2` | una sola `ldc 5.0f`, sin `fmul` | dos literales se pliegan, cada operando entra ya promocionado |
| `1.5f + 2L` / `1.0 + 2.5f` | `ldc 3.5f` / `ldc2_w 3.5d` | la promoción binaria antes de la cuenta |
| `1.0f / 3.0f` | `ldc 0.33333334f` | la cuenta de `float` se hace en precisión de `float32`, no de `float64` |
| `2.5f / 0` / `1.0 / 0` / `1.0 % 0` | `ldc Infinity` / `ldc2_w Infinity` / `ldc2_w NaN` | el cero flotante de divisor **sí** se pliega: es el resultado de IEEE, no una excepción |
| `1 / 0` y `1L / 0` | `iconst_1; iconst_0; idiv` y `lconst_1; lconst_0; ldiv` | la entera y la larga no: la excepción tiene que saltar en ejecución (§53) |
| `2147483647 + 1` | `ldc -2147483648` | el desborde envuelve |
| `d + i` (`i` variable) | `iload_0; i2d` | una variable sí se convierte **en su sitio** (§77), no se pliega |
| `if (f > 2)` | `fconst_2; fcmpl; ifle` | la comparación promociona igual (§81) |
| `float g = 2` / `return 3` de un `double` | `fconst_2` / `ldc2_w 3.0d` | la regla no es solo de las cuentas: rige en asignación, retorno y campo |

Implementado en `apkc/javaemisor.go` con `plegarConstante` (reemplaza a `plegarEntero` del §53, que solo
sabía de enteros), `cargaConstante` (las formas cortas de la tabla) y la promoción dentro de `expA`, que es
por donde ya pasaban la cuenta, la comparación, el retorno, la declaración y la asignación. La lectura de
los literales `j`/`f`/`d` del fuente pasa por el mismo `cargaConstante`, así que `-0.0f` deja de salir con
`fconst_0`.

Oráculos: `TestElLiteralPromocionadoSeEscribeComoJavac` (los veinte cuerpos instrucción a instrucción,
con `max_stack`, huecos de local y filas de línea), `TestElLiteralPromocionadoSeDexaComoJavac` (el dex sale
**byte a byte** igual sobre nuestras clases que sobre las de javac: **1872 bytes**) y
`TestLaAsignacionYElRetornoPromocionanSuLiteral`.

**El defecto que destapó meter el caso en el banco**: `new double[] {1.5, 2.0}`. El inicializador del §73
bajaba **tres** palabras de pila por elemento (`e.baja(3)`: arreglo, índice y valor), correcto para
`int[]` y `String[]`, y un valor ancho son **cuatro**. El emisor respondía `quedan 2 valores en la pila al
cerrar el metodo` y el `.class` no salía. Con `ArreglosAnchos` (diez métodos: las seis operaciones del §84
más `new long[n]`, `new double[] {…}`, `.length` de un `float[]` y una suma con bucle) el `.class` sale con
el mismo `Code`, la misma pila, los mismos huecos y las mismas filas que el de javac, y el dex es **byte a
byte** el de javac: **1496 bytes**, `TestLosArreglosAnchosSeEscribenComoJavac` y
`TestLosArreglosAnchosSeDexanIgualQueLosDeJavac`.

## 86. Un marco por encima de v15: las formas `/from16` que escribe `d8`, medidas sin aparato

El caso nuevo del banco (`Ver.arreglosAnchos`) reventó el camino «todo nuestro» con

```
[dex] Lp/Ver;arreglosAnchos: at pc 0, iload_0: register v22 does not fit a four-bit format
      (the /range or /from16 form is not measured here)
```

Los argumentos van al FINAL de la frame (§57): en cuanto el pico de temporales sube, un hueco se escribe en
v16 o más y las formas 11n, 12x, 21t, 22c y 22s no lo alcanzan. Fallaba igual sobre las clases de `javac`,
así que el defecto era del traductor, no del emisor.

Medición. Sin `rish` (el reset dejó solo `sudo`), el oráculo es `d8 --debug` y el `dexdump -d` del aparato:

```sh
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/dieciseis" &&
  javac --release 8 -g:lines,source D16.java && d8 --debug --min-api 28 --output d8_16 D16.class &&
  base64 -w0 d8_16/classes.dex | sudo sh -c "toybox base64 -d > /data/local/tmp/d16.dex;
  /apex/com.android.art/bin/dexdump -d /data/local/tmp/d16.dex"'
```

Lo que hace `d8` con un registro alto, palabra escrita por palabra:

| situación | palabra | decodificada | qué significa |
| --- | --- | --- | --- |
| leer un argumento alto para una cuenta de cuatro bits | `0202 1200` | `move/from16 v2, v18` | forma 22v: **destino de ocho bits** en la primera palabra, **fuente de dieciséis** en la segunda; el valor baja y de ahí `int-to-double v2, v2` |
| lo mismo con un `long`/`double` | `0501 1300` | `move-wide/from16 v1, v19` | la pareja de destino tiene que caber entera en cuatro bits (techo v14) |
| lo mismo con una referencia | `0801 1300` | `move-object/from16 v1, v19` | y de ahí `iput-object v1, v0, field` |
| una cuenta con un operando alto | `900a 0a10` / `ab02 0210` | `add-int v10, v10, v16` / `add-double v2, v2, v16` | cambia la forma de dos registros por la de **tres**, que lleva ocho bits por campo |
| una llamada con los argumentos altos | `7714 0200 0100` / `7401 0500 1200` | `invoke-static/range {v1..v20}` / `invoke-virtual/range {v18}` | la `/range` toma un registro inicial de 16 bits |
| reasignar un argumento alto | `d804 1001` | `add-int/lit8 v4, v16, #1` | **nunca escribe sobre el alto**: renombra el local a v4 (el `/lit8` sí lee un fuente de ocho bits) |

Corregido en `apkc/dexinstr.go`: `bajaAlto` ata el hueco a un temporal bajo con la 22v medida y
`guardaLocalEn` copia sobre ese bajo. La copia se emite **una vez por hueco**, no por lectura: atar el hueco
es el mismo mecanismo con el que `guardaLocal` ata la primera escritura, así que el registro alto no se
vuelve a nombrar y no hace falta ningún análisis de viveza. Copiar en cada lectura (la primera versión) le
costaba a `Ver.compara` un temporal más por uso y su marco se salía también de v15 —el test
`TestElMarcoSeReintentaCuandoLaPasadaFinalPideMas` lo cazó—. De paso se arregló `unidad23x`, que recortaba
a nibbles el segundo y el tercer registro: los dos campos de esa palabra son de **ocho** bits, como
demuestra el `add-int v10, v10, v16` medido arriba.

Frontera, rechazada por nombre: las formas `/range` de las llamadas y cualquier `/from16` de una cuenta
(`const/16`, `add-int/lit16`) siguen sin emitirse —plan 5.4—; con el `move` abajo basta para lo que hoy
compila el subconjunto, y escribir SOBRE un registro alto no lo hace `d8`, así que no está medido y no se
hace.

Prueba permanente: `apkc/dex16_test.go`, dos fuentes cuyo cuerpo entero es leer un argumento alto y
volverlo, para que el cuerpo salga predecible palabra por palabra —`[move/from16 v0, v17][return v0]` y
`[move-wide/from16 v0, v18][return-wide v0]`—, y que el destino de la copia quede por debajo del techo.
Ejecutada, la prueba es el banco con el caso nuevo (`Ver.arreglosAnchos(2)`, que usa el alta ancha, el
inicializador `new double[] {1.5, 2.0}`, el `u[0] * 2` del §85 y una comparación larga):

```
jvm (javac): rc=102, salida: total 7|…|6|7|10
  nuestro dex (Go de punta a punta): rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  dex de d8 sobre javac:             rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  dex de d8 sobre nuestra Ver:       rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  nuestro dex con nuestro main:      rc=102 y salida igual a la JVM  ok   dexdump verificando: rc 0
  jvm con nuestra clase emitida (Xverify:all): rc=102 y salida igual a la JVM  ok
banco: en orden
```

## 87. El `char`: viaja como entero, se estrecha con `i2c` y tiene opcodes propios en los dos mundos

Sonda `Letras` (veintiún métodos y el `<clinit>`), medida con `javac --release 8 -g:lines,source`,
`javap -c -p` y `javap -v` para el `ConstantValue`; el lado del dex, con `d8 --debug` sobre las mismas
clases y el `dexdump -d` del aparato por `sudo` (tras el reset no queda `rish`):

```sh
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/char/sonda" &&
  javac --release 8 -g:lines,source Letras.java && javap -c -p Letras.class && base64 -w0 Letras.class'
sh -c 'export PATH="$PREFIX/lib/jvm/java-21-openjdk/bin:$PATH"; cd "$PREFIX/tmp/char" &&
  javac --release 8 -g:lines,source Letras.java && d8 --debug --min-api 28 --output d8out Letras.class &&
  base64 -w0 d8out/classes.dex | sudo sh -c "toybox base64 -d > /data/local/tmp/letras.dex;
  /apex/com.android.art/bin/dexdump -d /data/local/tmp/letras.dex"'
sh -c 'cd apkc && go test -count=1 -run "Char|Letras" .'
sh -c 'cd /data/data/com.termux/files/home/Develop/Go/apkc-web &&
  APKC=/data/data/com.termux/files/usr/tmp/apkc-gate sh banco/banco.sh'
```

En el `.class` el `char` **es un `int` en la pila**; lo único propio son el descriptor `C` y cuatro
opcodes:

| fuente | `.class` | qué fija |
| --- | --- | --- |
| `'\n'`, `'\''`, `'\\'`, `'\t'` | `bipush 10 / 39 / 92 / 9` | la forma corta del entero, por valor (§53) |
| `'á'` (UTF-8 suelto) y `'ÿ'` | `sipush 225`, `sipush 255` | el punto de código, no el byte |
| `'\101'` y `'\u0008'` | `bipush 65`, `bipush 8` | el octal y el escape unicode |
| `c + 1` con `char c` | `iload_0; iconst_1; iadd` | **ninguna** conversión: ensanchar `char` a `int` no escribe nada |
| `(char) (a - b)` | `isub; i2c` | estrechar es `i2c` = **0x92** (el 0x91 es `i2b` y el 0x93 `i2s`) |
| `char c = 65;` y `v[0] = 65` | `bipush 65; istore_0` / `bipush 65; castore` | la constante entera entra en el hueco `char` sin conversión |
| `v[i]` / `v[i] = c` | `caload` **0x34** / `castore` **0x55** | las formas de arreglo propias del `char` |
| `new char[n]` | `iload_0; newarray 5` | el atype 5, que ya estaba en la tabla del §84 |
| `switch (c) { case 'a': … }` | `lookupswitch`/`tableswitch` con 97 | la clave `char` es el punto de código |
| `static final char CONSTE = 'Z'` | descriptor `C`, flags `0x0018`, `ConstantValue: int 90` y **sin** `putstatic` | la misma dobladura del `static final int` |
| `fijo = s; return fijo` con `static char fijo` | `putstatic`/`getstatic` con descriptor `C`, y en el `<clinit>` `bipush 65; putstatic` | el campo estático `char` existe |

En el dex las formas son **tipadas**, no la genérica del `int`:

| `.class` | palabra de `d8` | decodificada |
| --- | --- | --- |
| `caload` | `4900 0001` | `aget-char v0, v0, v1` —**0x49**, no el `aget` 0x44 del `int`— |
| `castore` | `5002 0001` | `aput-char v2, v0, v1` —**0x50** |
| `i2c` | `8e00` | `int-to-char v0, v0` —**0x8e**, en la rejilla de conversiones del §80 |
| `getstatic` de `fijo:C` | `6500 0100` | `sget-char v0, LLetras;.fijo:C` —**0x65** |
| `putstatic` de `fijo:C` | `6c00 0100` | `sput-char v0, …` —**0x6c** |
| `newarray 5` | `2300 0700` | `new-array v0, v0, [C` (la forma del §66, con su `type_id`) |

Medido de paso, y **no** implementado: `d8` reescribe el inicializador `new char[] {'a','b'}` como
`fill-array-data` (`2600 0500 0000`) con un payload `0003 0200 0200 0000 6100 6200`, cuyo
`element_width` es **2** —frente al 4 del `float` y al 8 del `double`, leídos en el volcado del §84—, en
lugar de la cadena `dup`/`castore`. Es la misma diferencia de reescritura ya aceptada del §53 y del 2G.4:
el oráculo manda sobre las estructuras, no sobre el cuerpo optimizado, y este fork emite la cadena de javac.

**La elección `tableswitch`/`lookupswitch` queda como diferencia escrita** (era del §64 y se apoya ahora
con datos): sobre una rejilla medida de claves equidistantes, javac escribe tabla en `(3 claves, rango 4)`,
`(4, 8)`, `(5, 10)`, `(6, 18)`, `(6, 19)` y `(8, 24)`, y búsqueda en `(2, 1)`, `(3, 6)`, `(4, 12)`,
`(5, 15)`, `(6, 24)` y `(8, 32)`; este emisor se queda con la tabla cuando `rango + 1 <= 2 · claves`, que
es más conservador —con cuatro claves y rango 8 javac escribe tabla y el emisor, búsqueda—. Las dos formas
son legales; la sonda del test usa claves consecutivas, donde los dos coinciden, y la comparación del `Code`
no depende de ese criterio.

Implementado: el léxico lee el literal con sus escapes (`leerCharJava` y `escapeJava` en
`apkc/javafuente.go`), el emisor emite el `char` como entero con descriptor `C`, estrecha con `i2c`,
conoce `caload`/`castore` y dobla el `ConstantValue`; el caminante y los marcos conocen los tres opcodes
nuevos; y el traductor baja a `aget-char`, `aput-char`, `int-to-char`, `sget-char` y `sput-char`. El
portero sigue rechazando por nombre `i2b`/`i2s` (el `byte` y el `short` no los escribió sonda alguna).

Oráculos: `apkc/javaemisor_char_test.go` —`TestElCharSeEscribeComoJavac` (veintidós cuerpos con pila,
huecos y filas de línea), `TestElCharSeDexaIgualQueLosDeJavac` (el dex sale **byte a byte** el mismo sobre
nuestras clases que sobre las de javac: **2148 bytes**), `TestElCharConstantValueEsEntero` (tag `Integer`,
valor 90) y `TestLasFormasDexDelChar`—, y la ejecución: `Ver.letras(char[], int)` en el banco
(`new char[] {'a','b','c'}`, `'A' + n`, el `(char)`, la comparación y `v[0] = c`) da **32** por los cinco
caminos, con `dexdump` verificando `rc 0` en los cuatro dex.

## 88. El entorno del shell y el banco por `sudo` sin `rish`

Medido el 2026-11-08 tras el reset de fábrica. El shell arranca con `HOME=/home/leonisauro`, que no
tiene `Develop/` ni `Android/`; el repo y el SDK viven bajo `/data/data/com.termux/files/home`. Sin
corregirlo, `studio/e2e.sh` no encuentra `../apkc` ni `~/Develop/Go/apkc-harness/zipcheck.py` y falla
con `ENOENT` aunque el código esté bien:

```sh
sh -c 'HOME=/data/data/com.termux/files/home PREFIX=/data/data/com.termux/files/usr \
  ANDROID_HOME=/data/data/com.termux/files/home/Android \
  APKC=/data/data/com.termux/files/usr/tmp/apkc-gate \
  APKC_WASM=/data/data/com.termux/files/usr/tmp/apkc-wasm.wasm \
  sh studio/e2e.sh'
```

`tools/entorno.sh` fija esas cuatro variables y ofrece `test`, `wasm`, `gate`, `banco` y `doctor`.

Sin `rish`, el banco corre por `sudo`, pero `app_process64` no hereda el entorno de Android y aborta con
`Check failed: !location.empty() BOOTCLASSPATH and DEX2OATBOOTCLASSPATH must not be empty` (rc 134). La
copia que funciona es la de `zygote64`:

```sh
sudo sh -c 'export CLASSPATH=/data/local/tmp/bnuestro.dex; \
  eval "$(tr "\0" "\n" < /proc/$(pidof zygote64)/environ | \
  grep -E "^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH|SYSTEMSERVERCLASSPATH|STANDALONE_SYSTEMSERVER_JARS)=" | \
  sed "s/^/export /")"; /system/bin/app_process64 /data/local/tmp p.Arranque'
```

Da `rc=102` y la misma salida que la JVM; `banco/banco.sh` lo hace solo cuando `RISH=sudo`. El `sudo`
que vale es **`/usr/sbin/sudo`** (el de Termux, `$PREFIX/bin/sudo`, no encuentra un `su` que soporte
`-c`).

## 89. El literal `X.class`: `ldc` de CONSTANT_Class, `getstatic` del primitivo y `if_acmp*`

Sonda `Clase` (diez métodos y el `<clinit>`), medida con `javac --release 8 -g:lines,source`,
`javap -c -p -v`, `d8 --debug`, el `dexdump -d` del aparato y la ejecución por el banco:

```sh
cd /data/data/com.termux/files/usr/tmp/clase2
javac --release 8 -g:lines,source sonda/Clase.java
javap -c -p -v sonda/Clase.class
d8 --debug --min-api 28 --output d8 sonda/Clase.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/Clase.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
d8 --debug --min-api 28 --output d8desde nuestra/Clase.class
cmp d8/classes.dex d8desde/classes.dex
/apex/com.android.art/bin/dexdump -d nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

En el `.class`:

| fuente | forma | qué fija |
| --- | --- | --- |
| `String.class`, `Object.class` | `ldc #7 // class java/lang/String` | `ldc` de una entrada `CONSTANT_Class` (tag 7) |
| `String[].class` | `ldc #9 // class "[Ljava/lang/String;"` | el nombre de la clase es el descriptor del arreglo |
| `int[].class` | `ldc #20 // class "[I"` | también para el arreglo de un primitivo |
| `int.class`, `void.class` | `getstatic java/lang/Integer.TYPE:Ljava/lang/Class;` / `Void.TYPE` | **no** es `ldc`: es el campo `TYPE` del envoltorio |
| `o.getClass() == String.class` | `invokevirtual Object.getClass; ldc String.class; if_acmpne` | `if_acmpne` **0xa6**; `if_acmpeq` **0xa5** es el `!=` |
| `String.class.getName()` | `ldc; invokevirtual Class.getName:()Ljava/lang/String;` | el tipo que sale del literal es `Ljava/lang/Class;` |

En el dex, medido con `dexdump -d` sobre nuestro fichero:

| `.class` | palabra | decodificada |
| --- | --- | --- |
| `ldc` de `String.class` | `1c00 0400` | `const-class v0, Ljava/lang/String;` —**0x1c**, formato 21c |
| `ldc` de `String[].class` | `1c00 0900` | `const-class v0, [Ljava/lang/String;` |
| `ldc` de `int[].class` | `1c00 0800` | `const-class v0, [I` |
| `getstatic Integer.TYPE` | `6200 0100` | `sget-object v0, Ljava/lang/Integer;.TYPE:Ljava/lang/Class;` —**0x62** |
| `if_acmpne` | `3310 0400` | `if-ne v0, v1, +0004` —**0x33** de dos registros; `if_acmpeq` baja a `if-eq` **0x32** |

Los dos dex de este fork —sobre nuestras clases y sobre las de javac— son **1412 bytes** y `cmp` mudo;
`d8 --debug` sobre cualquiera de las dos da **1512 bytes** y los dos ficheros de `d8` son idénticos, con
`rc 0`. La diferencia de tamaño con el nuestro es el bloque de anotaciones y la forma del `debug_info`
ya aceptada en el 2G.4 y el §53, no el cuerpo.

La medición destapó un defecto del lector de clases: `constantPool` solo guardaba `tag[i]` en la rama
`else`, así que una `CONSTANT_Class` (y una `Utf8`) quedaban con etiqueta **0** y el traductor no podía
distinguir el `ldc` de una clase. Corregido en `apkc/classfile.go`; a partir de ahí el `ldc` de clase
llega a `const-class`.

Implementado: el parser reconoce `X.class`, `primitivo.class` y `T[].class` (`literalDeClase` en
`apkc/javafuente.go`); el emisor escribe `ldc` de `CONSTANT_Class` o el `getstatic` del envoltorio y
resuelve `==`/`!=` entre referencias con `if_acmpeq`/`if_acmpne` (`literalDeClase` y
`comparacionDeReferencias` en `apkc/javaemisor.go`); el caminante y los marcos conocen los dos opcodes;
y el traductor baja el `ldc` de clase a `const-class` 0x1c y las dos comparaciones a `if-eq`/`if-ne`
(`apkc/dexinstr.go`).

Oráculos: `apkc/javaemisor_clase_test.go` —`TestLaClaseSeEscribeComoJavac` (los diez cuerpos con pila,
huecos y filas de línea; el índice del pool se normaliza porque javac interna `String` al leer el campo
y este emisor al escribir el `<clinit>`, y el oráculo de bytes es el dex), `TestLaClaseSeDexaIgualQueLosDeJavac`
(el dex byte a byte idéntico: **1412 bytes**) y `TestLasFormasDexDeLaClase` (los opcodes 0x1c y 0x62)—, y
la ejecución: `Ver.clases()` en el banco (el `ldc`, el `getstatic` del primitivo, el arreglo y las dos
comparaciones) devuelve **2** por los cinco caminos, con `dexdump` verificando `rc 0` en los cuatro dex.

## 90. Las llamadas de más de cinco palabras: `invoke-*/range` (3rc)

Sonda `Seis`: un estático con seis enteros, otro con dos `long` (ocho palabras) y un virtual con
receptor y seis enteros (siete palabras). Medida con `javac --release 8 -g:lines,source`, `d8 --debug`,
`dexdump -d` y la ejecución por el banco:

```sh
cd /data/data/com.termux/files/usr/tmp/seis
javac --release 8 -g:lines,source sonda/Seis.java
d8 --debug --min-api 28 --output d8 sonda/Seis.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/Seis.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
d8 --debug --min-api 28 --output d8desde nuestra/Seis.class
cmp d8/classes.dex d8desde/classes.dex
/apex/com.android.art/bin/dexdump -d nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

Las palabras de `d8`, leídas con `dexdump -d`:

| llamada | palabras | decodificada |
| --- | --- | --- |
| estática de seis enteros | `7706 0600 0000` | `invoke-static/range {v0…v5}`: opcode **0x77**, `AA=6` palabras, método `@6`, primer registro `0` |
| estática con dos `long` | `7708 0100 0000` | `AA=8`: los dos pares cuentan dos palabras cada uno |
| virtual de receptor + seis | `7407 0200 0000` | `invoke-virtual/range` **0x74**, `AA=7`; el receptor es el **primer** registro del bloque |

El formato 3rc es: primera unidad `op | AA<<8` (AA = palabras), segunda el índice del método, tercera
el primer registro del bloque contiguo. `d8` usa el bloque que ya tiene; este fork, cuando los
argumentos no son contiguos, pide un bloque nuevo por encima de todo lo vivo (`tomaBloque`), copia
cada valor en orden con `move`/`move-object`/`move-wide` (12x, o 22v `/from16` si el destino pasa de
v15) y emite el 3rc con el receptor el primero. En `llamaInst` eso da `{v6…v12}` en vez del
`{v0…v6}` de `d8`; las dos formas son legales y ART ejecuta la nuestra.

La primera versión puso el receptor al final del bloque y ART la rechazó:
`int p.Ver.seisInstLlama() failed to verify: [0x6] tried to get class from non-reference register v0
(type=BooleanConstant)`. El verificador lee el primer registro como receptor; el orden del bloque es
parte de la instrucción, no un detalle del asignador.

Los dos dex de este fork —sobre nuestras clases y sobre las de javac— son **1032 bytes** y `cmp` mudo;
`d8 --debug` da **1132 bytes** sobre cualquiera de las dos y los dos ficheros de `d8` son idénticos,
con `rc 0`. La diferencia de tamaño es de nuevo el bloque de anotaciones/debug ya aceptado.

Implementado: `invoca` elige 35c o 3rc según las palabras o si un registro no cabe en un nibble;
`invocaRango` escribe el 3rc; `tomaBloque` reserva el bloque y `mueveRegistro` copia con 12x o 22v
(`apkc/dexinstr.go`). El banco añade `seis`, `seisLlama`, `seisInst`, `seisInstLlama`, `seisAncho` y
`seisAnchoLlama`; `java p.Arranque` y los cinco caminos dan `21|21|21`, con `dexdump` verificando
`rc 0` en los cuatro dex.

## 91. Constructores escritos a mano: `super`/`this`, los inicializadores de campo y dos `code_item`

Sonda `Base`/`Ctor` (un `super(x)` explícito, un `this(2)` y un campo con inicializador) y
`Base2`/`Simple` (el `super()` implícito). Medida con `javac --release 8 -g:lines,source`,
`javap -c -p -v`, `d8 --debug`, `dexdump` y el banco:

```sh
cd /data/data/com.termux/files/usr/tmp/ctor
javac --release 8 -g:lines,source sonda/Base.java sonda/Ctor.java
javap -c -p -v sonda/Ctor.class
d8 --debug --min-api 28 --output d8 sonda/Base.class sonda/Ctor.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/Base.java sonda/Ctor.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
/apex/com.android.art/bin/dexdump -f nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

En el `.class`:

| constructor | forma | qué fija |
| --- | --- | --- |
| `Ctor()` con `this(2)` | `aload_0; iconst_2; invokespecial <init>(I)V; return` | delegar **no** baja los inicializadores de campo |
| `Ctor(int)` con `super(x)` | `aload_0; iload_1; invokespecial Base.<init>(I)V; aload_0; iconst_1; putfield a; …` | el `super` va primero y después `a = 1`, en la línea de la declaración del campo |
| `Simple()` sin llamada | `aload_0; invokespecial Base2.<init>(); aload_0; iconst_1; putfield a; …` | el `super()` implícito lleva la línea de la **firma** del constructor |
| descriptor | `()V` / `(I)V`; flags `0x0000` | un constructor vuelve `V`, no el nombre de la clase |

Líneas medidas: `Ctor()` → `line 5: 0`, `line 6: 5`; `Ctor(int)` → `line 8: 0` (super), `line 2: 5`
(inicializador), `line 9: 10`, `line 10: 17`; `Simple()` → `line 3: 0` (super implícito), `line 2: 4`,
`line 4: 9`, `line 5: 14`.

En el dex: `invokespecial` baja a `invoke-direct` 0x70 y cada inicializador es un `iput` con el
`putfield` del `.class`. Nuestro dex sobre nuestras clases y sobre las de javac es **860 bytes** y `cmp`
mudo; `d8 --debug` da **968 bytes** sobre cualquiera de las dos y los dos ficheros de `d8` son
idénticos. `dexdump -f` verifica con `rc 0`.

**El defecto que destapó el banco**: `areaCodigo` y `areaDebug` guardaban sus mapas por
`[clase, nombre]`, sin el descriptor. Con dos `<init>` distintos, los dos métodos apuntaban al mismo
`code_item` (los dos salían con `code=0x9c8`), y `dexdump` cortaba con
`Non-zero padding 3 before section of type 8195 at offset 0x11c8` y ART con
`No pending exception expected: java.lang.ClassNotFoundException: p.Arranque`. El doble oráculo no lo
veía porque las clases de javac colisionaban igual; lo destapó ejecutar el dex en el aparato. La clave
pasa a `[clase, nombre, descriptor]` (`apkc/dexarma.go` y `apkc/dexdebug.go`) y hay una prueba que
exige un `code_item` por método con cuerpo (`TestLosConstructoresSobrecargadosTienenCodeItemPropio`).

Implementado: el parser lee `super(...)` y `this(...)` (`parrafo 91`), `metodoDeClassFile` deja de
rechazar `<init>` y escribe la delegación, el `super()` implícito y los inicializadores en el orden de
javac; `metodo` deja el retorno vacío en un constructor y ya no consume dos veces el nombre. El banco
añade a `Ver` los dos constructores y `sumaCtor()`: `new Ver().sumaCtor()` da **9** por los cinco
caminos, porque `Ver()` delega en `Ver(3)` (`campoCtor = 5`, `otroCtor = 4`).

## 92. `throws`, interfaces propias, métodos abstractos y `invokeinterface`

Sonda `I`/`Impl`/`Uso`/`A`: una interface con una constante y dos métodos sin cuerpo, una clase que la
implementa, una llamada por la interface y un método con uno y con dos `throws`; una clase abstracta
con un método abstracto. Medida con `javac --release 8 -g:lines,source`, `d8 --debug`, `dexdump` y el
banco:

```sh
cd /data/data/com.termux/files/usr/tmp/iface
javac --release 8 -g:lines,source sonda/I.java sonda/Impl.java sonda/Uso.java sonda/A.java
javap -c -p -v sonda/I.class sonda/Impl.class sonda/Uso.class sonda/A.class
d8 --debug --min-api 28 --output d8 sonda/I.class sonda/Impl.class sonda/Uso.class sonda/A.class
/data/data/com.termux/files/usr/tmp/apkc-gate compile \
  --android-jar /data/data/com.termux/files/home/Android/platforms/android-34/android.jar \
  -o nuestra sonda/I.java sonda/Impl.java sonda/Uso.java sonda/A.java
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex nuestra
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o javac.dex sonda
cmp nuestro.dex javac.dex
/apex/com.android.art/bin/dexdump -d nuestro.dex
cd /data/data/com.termux/files/home/Develop/Go/apkc-web && sh banco/banco.sh
```

En el `.class`:

| pieza | forma | qué fija |
| --- | --- | --- |
| interface `I` | flags **0x0601** (`PUBLIC INTERFACE ABSTRACT`), `super java/lang/Object`, sin `<init>` | una interface no lleva constructor ni `ACC_SUPER` |
| campo `I.N` | flags **0x0019** y `ConstantValue: int 3` | en una interface el campo es `public static final` aunque el fuente no lo diga |
| `I.doble`, `I.nada` | flags **0x0401**, **sin** `Code` | los métodos de una interface son `public abstract` |
| `Impl` | `interfaces: 1`, flags **0x0021** | la clase declara la interface y conserva `ACC_SUPER` |
| `A` y `A.x` | clase **0x0421**, método **0x0401** sin `Code` | una clase abstracta conserva `ACC_SUPER` y añade `ACC_ABSTRACT` |
| `Uso.falla` / `falla2` | `Code` y después `Exceptions` con 1 / 2 clases | el atributo cuenta y nombra `java/io/IOException` y `java/lang/IllegalStateException` |
| `Uso.usa` | `invokeinterface #7, 2  // InterfaceMethod I.doble:(I)I` | la llamada se decide por el tipo de la interface, no por la clase del receptor |

En el dex: `invoke-interface` **0x72** en 35c, medido como `7220 0200 0100` (`{v1, v0}`,
`method@2`); los métodos abstractos y la interface no llevan `code_item`; el `Exceptions` no viaja al
dex (es una comprobación de compilación). Nuestro dex sobre nuestras clases y sobre las de javac es
**1172 bytes** y `cmp` mudo; `d8 --debug` da **1504 bytes** sobre cualquiera de las dos y los dos
ficheros de `d8` son idénticos. `dexdump -d` verifica con `rc 0`.

Tres defectos del camino, destapados por la sonda: (a) `throws` no estaba en `palabrasJava`, así que
el tokenizador lo leía como identificador y el cuerpo del método fallaba con «se esperaba `{` y hay
`throws`»; (b) un cuerpo vacío `{ }` se confundía con un método abstracto y se rechazaba; (c)
`accesosDe` no devolvía `ACC_INTERFACE` para una interface del propio fuente, así que `Uso.usa` salía
como `invokevirtual`. Corregidos los tres.

Implementado: el parser lee `throws` (lista en `jmetodo.lanza`), `interface` y su `extends` múltiple,
y marca `abstracto` en el `;` o en la interface (`apkc/javafuente.go`); el emisor escribe los flags, el
`Exceptions`, los campos implícitos de la interface y el `invokeinterface`
(`apkc/javaemisor.go`). Oráculos: `apkc/javaemisor_iface_test.go` (flags, `Code`, `Thrown`,
`interfaces` y dex byte a byte) y el banco, donde `Ver implements Operable` y
`Ver.usa(new Ver())` devuelve **6** y `Ver.conThrows()` **5** por los cinco caminos; `Arranque.main`
declara `throws IOException` y también pasa por nuestro compilador.

