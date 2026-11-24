# Evidencia — §93-§116

## 93. Recursos binarios: `res/drawable` y `mipmap` con PNG, `roundIcon`

El enlace acepta un PNG (o cualquier binario) en `res/drawable` y `res/mipmap`: el fichero viaja
literal al apk y su entrada de la tabla es un `Res_value` de tipo cadena con la ruta. Medido con
`aapt2 dump resources` y el empaquetado de la página:

```sh
cd /data/data/com.termux/files/usr/tmp/icono/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o app.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources app.apk
unzip -l app.apk
unzip -p app.apk res/drawable/icono.png | sha256sum
sha256sum res/drawable/icono.png
# y el mismo proyecto por nuestro enlace, sin aapt2 ni zipalign:
apkc build --linker go --compiler go --signer go --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
aapt2 dump resources build/app.apk
```

`aapt2 dump resources` del paquete que escribe este fork:

```text
type drawable id=01 entryCount=1
  resource 0x7f010000 drawable/icono
    () (file) res/drawable/icono.png type=PNG
type mipmap id=02 entryCount=1
  resource 0x7f020000 mipmap/ic_launcher
    () (file) res/mipmap/ic_launcher.png type=PNG
type layout id=03 entryCount=1
...
```

El PNG del zip y el del proyecto tienen el mismo `sha256sum`; `roundIcon="@mipmap/ic_launcher"` es un
atributo del manifiesto y no necesita nada especial. El `resources.arsc` de este fork mide 928 bytes y
el de `aapt2` 1060 en el mismo proyecto: la diferencia es la segunda configuración `layout-v1` que
`aapt2` añade a los layouts y este enlace sigue emitiendo una sola (diferencia ya escrita en el §28).
La entrada `drawable` y la `mipmap` son las mismas en los dos.

`studio/e2e.sh` lo comprueba de punta a punta: crea un proyecto con `res/drawable/icono.png`,
`res/mipmap/ic_launcher.png`, `values-es` y `roundIcon`, compila con `--linker go --compiler go`, y
exige que `aapt2 dump resources` declare las dos entradas y el PNG viaje literal; `apksigner` y
`zipalign -c -p 4` aprueban el apk (`icono: apk de 10185 bytes`).

## 94. `values-es`: language, `spec_map`, offset ausente y orden del pool

Dos sondas de dos idiomas, comparadas **byte a byte** con el `resources.arsc` de `aapt2`:

```sh
cd /data/data/com.termux/files/usr/tmp/idioma2/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
# idioma3: la misma comparación con una clave que solo existe en values-es
```

Las dos salen **idénticas**: 728 bytes en `idioma2` (base `app_name`, `otro`; es `app_name`) y 716 en
`idioma3` (base `s1`, `s2`; es `s1`, `s3`). Lo que fija el fichero:

| pieza | valor |
| --- | --- |
| `ResTable_config` de `es` | tamaño 64 y `language[0]='e'`, `language[1]='s'` en los offsets +8 y +9 |
| bloque base | tamaño 64 y el resto a cero |
| `spec_map` | **0x0004** (locale) en la clave que tiene `es`; 0 en la que no |
| clave ausente en `es` | offset **0xffffffff** en el bloque `es`, con `entryCount` igual al de la base |
| clave solo en `es` | `aapt2` la **elimina** (`removing resource … without required default value`) y no entra ni en la tabla ni en el pool |
| orden del pool global | la base ordenada por texto y después cada idioma ordenado por texto (`Icono`, `Uno`, `Icono ES`; `Dos`, `Uno`, `Uno ES`) |

Implementado: `fuentesDelProyecto` acepta `values-xx` de dos letras y guarda el calificador en
`fuenteXML.config`; `tablaDeRecursos` agrupa por tipo y configuración, calcula el `spec_map` 0x0004,
escribe el offset ausente y descarta las claves sin base; `escribeBloqueTipo`/`configDe` escriben la
configuración y el escritor admite varios bloques por tipo (`apkc/linker.go`, `apkc/valores.go`,
`apkc/arscwriter.go`). Prueba: `apkc/valores_es_test.go` compara el arsc de 728 bytes con el fixture
de `aapt2`; la segunda sonda de 716 bytes se midió igual y se comprueba en `studio/e2e.sh`.

**Frontera escrita**: `values-xx` es el único calificador implementado. `layout-land`,
`drawable-hdpi` y `values-<región>` piden las máscaras de orientación/densidad/país, que no están
medidas; se rechazan por nombre. El criterio de cierre «dos idiomas en `res/values`» sí queda cubierto.

## 95. La app del criterio de cierre, compilada y firmada sin `aapt2` ni `javac`

El criterio de la Fase 5 se juntó en un proyecto `cierre`: un `MainActivity` con **bucle sobre un
arreglo**, **`switch`**, **`try`/`catch`**, un icono en `res/drawable` y dos idiomas en
`res/values`/`res/values-es`. Compilado y firmado por el camino todo-Go de la página:

```sh
cd /data/data/com.termux/files/usr/tmp/cierre/proj
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" \
  --keystore "$HOME/.android/debug.keystore"
apksigner verify --verbose --min-sdk-version 21 build/app.apk
zipalign -c -p 4 build/app.apk
aapt2 dump resources build/app.apk
```

El apk mide **9861 bytes**, `apksigner` dice v1 y v2 correctos, `zipalign -c -p 4` sale 0 y `aapt2
dump resources` declara `drawable/icono (file) res/drawable/icono.png type=PNG` y la cadena
`string/app_name` con `()` «Cierre» y `(es)` «Cierre ES». El dex de 2012 bytes trae el bucle, el
`switch` y el `try`/`catch` del fuente.

Lo que **no** está hecho: instalarlo y arrancarlo en el teléfono. Eso exige `rish` o `pm install` por
`sudo`, y es una acción sobre el aparato que se pide al usuario antes de hacerla (memoria del repo:
`feedback-telefono-por-rish`). Hasta entonces la Fase 5 se queda **abierta**: el hecho 2 del objetivo
—instala y arranca en un teléfono real— ya se midió en el §55 con el contador, pero no con esta app de
cierre.

## 96. `string-array`, `integer-array` y `plurals` en un `values`

El enlace de recursos acepta los valores compuestos de un `values`: `string-array`, `integer-array` y
`plurals`, además de `bool`, `color` e `integer` que ya estaban. La tabla de la sonda sale **byte a
byte** la de `aapt2` (1152 bytes):

```sh
cd /data/data/com.termux/files/usr/tmp/tipos/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
aapt2 dump resources build/app.apk
```

`aapt2 dump resources`:

```text
type array id=01 entryCount=2
  resource 0x7f010000 array/nombres
    () (array) size=2
      ["uno", "dos"]
  resource 0x7f010001 array/numeros
    () (array) size=2
      [1, 2]
type plurals id=05 entryCount=1
  resource 0x7f050000 plurals/cuenta
    () (plurals) size=2
      one="uno"
      other="varios"
```

Lo que fija el binario (medido con el lector del §24 y `aapt2 dump`):

| pieza | valor |
| --- | --- |
| entrada de array | compleja, `padre=0`, flags `0x0001`, un registro por elemento |
| nombre del registro | `0x01000000 \| (índice + 1)` (0x01000001, 0x01000002, …) |
| valor de `string-array` | tipo cadena 0x03 con el índice del pool global |
| valor de `integer-array` | tipo entero decimal 0x10 con el número |
| plural | compleja, registros con los ids de cantidad de Android: other 0x01000004, zero 0x01000005, one 0x01000006, two 0x01000007, few 0x01000008, many 0x01000009 |
| orden de los plurales | **el del fuente** (one antes que other en la sonda), no ordenado por id |

Implementado en `apkc/valores.go`: `itemsDeArray`/`itemsDePlural` leen los hijos, y
`tablaDeRecursos` añade los textos de los registros al pool global y los resuelve al escribir.
Prueba: `apkc/valores_tipos_test.go` (fixture de 1152 bytes de `aapt2`).

**Frontera**: `fraction`, `attr` y `styleable` siguen fuera; se rechazan por nombre. `bool`, `color` e
`integer` ya estaban desde el §29.

## 97. `menu`, `xml` y `raw`: AXML de raíz libre y fichero literal

`res/menu` y `res/xml` se enlazan como los layouts (son AXML), y `res/raw` como un fichero binario.
Medido con `aapt2 dump resources` y el empaquetado de la página:

```sh
cd /data/data/com.termux/files/usr/tmp/otros/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources aapt2.apk
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
aapt2 dump resources build/app.apk
unzip -l build/app.apk
```

`aapt2 dump resources` del apk de este fork:

```text
type id id=01 entryCount=1
  resource 0x7f010000 id/accion
type menu id=02 entryCount=1
  resource 0x7f020000 menu/principal
    () (file) res/menu/principal.xml type=XML
type raw id=03 entryCount=1
  resource 0x7f030000 raw/dato
    () (file) res/raw/dato.txt
type string id=04 entryCount=1
  resource 0x7f040000 string/app_name
    () "Otros"
type xml id=05 entryCount=1
  resource 0x7f050000 xml/prefs
    () (file) res/xml/prefs.xml type=XML
```

El `menu` y el `xml` viajan como AXML (`res/menu/principal.xml`, `res/xml/prefs.xml`) y el `raw` viaja
con sus bytes (`res/raw/dato.txt`). `aapt2` añade además `res/menu-v1/principal.xml` por la misma
configuración por API que ya se acepta en el §28; este enlace emite una sola. La tabla y el `R.java`
llevan los tipos `menu`, `raw` y `xml` como cualquier otro recurso.

Implementado en `apkc/linker.go` (se aceptan las carpetas, `menu`/`xml` van a la lista de AXML y
`raw` a los ficheros) y `apkc/valores.go`/`apkc/enlazar.go` (el tipo sale de la ruta). Prueba:
`apkc/linker_otros_test.go` exige los tres tipos, el AXML de los dos XML y los bytes del `raw`.

**Frontera**: el `<permission>` del manifiesto sigue rechazado, porque `aapt2` escribe además un
`Manifest.java` que este enlace no genera.

## 98. La versión de platform: API → versión de Android en la cabecera AXML

La tabla que usa `nombreDeVersionDeAndroid` para `compileSdkVersionCodename` y
`platformBuildVersionName` se midió con `aapt2 link --compile-sdk-version-code`/
`--compile-sdk-version-name` y el volcado de la cabecera:

```sh
cd /data/data/com.termux/files/usr/tmp/tipos/proj
for par in '35 15' '33 13' '32 12L' '31 12' '28 9' '27 8.1'; do
  set -- $par
  aapt2 link -o v$1.apk --manifest AndroidManifest.xml \
    --compile-sdk-version-code "$1" --compile-sdk-version-name "$2" \
    -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
  aapt2 dump xmltree v$1.apk --file AndroidManifest.xml | grep -E 'platformBuildVersion(Name|Code)'
done
```

La tabla implementada es `27→8.1`, `28→9`, `29→10`, `30→11`, `31→12`, `32→12L`, `33→13`, `34→14`
y `35→15`. La 34/14 se había medido en el §37; las demás se comprobaron con el comando de arriba.
Cuando el nombre no es numérico (`8.1`, `12L`), `platformBuildVersionName` se escribe como **cadena**,
que es lo que hace aapt2; con un nombre numérico sigue siendo entero. Cualquier API fuera de la tabla
se rechaza por nombre y se puede pasar con `--codename`. Prueba: `TestVersionesDeAndroidMedidas`.

## 99. Una anónima dentro de un método `static` sin captura

La mitad de 5.14 que no necesita capturar locales: una anónima creada en un método `static` no lleva
`this$0`. Medido con `javac --release 8 -g:lines,source`, `javap -c -p -v` y el doble oráculo:

```sh
cd /data/data/com.termux/files/usr/tmp/cap/javac
javac --release 8 -g:lines,source Captura.java
javap -c -p -v Captura.class 'Captura$1.class'
# y la sonda por el emisor/dexer del fork:
cd /data/data/com.termux/files/home/Develop/Go/apkc-web
tools/entorno.sh -- sh -c 'cd apkc && go test -count=1 -run "AnonimaEnStatic" .'
```

`Captura$1` de javac tiene constructor **`()V`** (`aload_0; invokespecial Object.<init>; return`), **sin**
campo `this$0` y con `EnclosingMethod` apuntando a `Captura.uno`. Nuestra clase y su dex salen byte a
byte los de javac; los tests comparan el `Code` de `<init>`/`run`, las filas de línea y el dex.

## 100. La captura de locales effectively-final: `val$x`, el ctor y `MethodParameters`

La segunda mitad de 5.14: una anónima captura un parámetro/local del método que la crea. Se midieron
los dos casos, la captura en un método `static` y la que además lleva el `this$0` del exterior:

```sh
cd /data/data/com.termux/files/usr/tmp/cap2 && javac --release 8 -g:lines,source Captura.java
javap -c -p -v 'Captura$1.class'
cd /data/data/com.termux/files/usr/tmp/cap3/javac && javac --release 8 -g:lines,source Captura3.java
javap -c -p -v 'Captura3$1.class'
cd /data/data/com.termux/files/home/Develop/Go/apkc-web
tools/entorno.sh -- sh -c 'cd apkc && go test -count=1 -run "Captura" .'
```

| caso | constructor | campo y `MethodParameters` |
| --- | --- | --- |
| `Captura.dos(int x)` (static) | **`(I)V`**; `aload_0; iload_1; putfield val$x; aload_0; invokespecial Object.<init>; return` | `val$x` final synthetic (0x1010); **sin** `MethodParameters` |
| `Captura3.r(int x)` (instancia) | **`(LCaptura3;I)V`**; asigna `this$0` y `val$x` antes del `super` | campos en orden `val$x` y después `this$0`; `MethodParameters` con dos filas: el exterior `final mandated` (0x8010) y `val$x` `final synthetic` (0x1010) |

**Nota del §273 (`d123135`).** La columna de `MethodParameters` de la fila de instancia es la
medición de javac 21. El oráculo del repo es javac 17.0.21, que no escribe el atributo en ninguna
anónima, y el emisor dejó de escribirlo; el constructor, sus parámetros, el orden de los campos y la
`Signature: ()V` de las capturas no cambian.

La referencia `x` dentro de `run` sale `aload_0; getfield val$x` (y `x + y` añade el `getfield` del
campo del exterior). Nuestras clases y sus dex salen byte a byte los de javac en los dos casos; las
pruebas comparan campos, `Code`, filas de línea y dex.

Implementado: el parser registra las locales del método (`jmetodo.locales`), al entrar en la anónima
resuelve un nombre contra ellas y lo convierte en `this.val$nombre`, añade el campo `val$…` y guarda
la captura; el emisor pone los campos en el orden de javac, el constructor con sus parámetros (hoy
sin `MethodParameters`, §273), y `new` pasa las capturas antes de los argumentos
(`apkc/javafuente.go`, `apkc/javaemisor.go`). Frontera: una anónima dentro de otra sigue rechazada
por nombre.

## 101. `Signature` genérica en el dex: el troceado de `d8`

El dexer copia el atributo `Signature` del `.class` a la anotación de sistema
`Ldalvik/annotation/Signature;`, con la firma troceada en un arreglo de cadenas. Medido en una clase
con firma de clase, de campo y de método:

```sh
cd /data/data/com.termux/files/usr/tmp/gen
javac --release 8 -g:lines,source Gen.java
d8 --debug --min-api 28 --output d8 Gen.class
apex/com.android.art/bin/dexdump -a d8/classes.dex
/data/data/com.termux/files/usr/tmp/apkc-gate dex -o nuestro.dex .
/apex/com.android.art/bin/dexdump -a nuestro.dex
```

`dexdump -a` del fichero de este fork, igual que el de `d8`:

```text
Annotations on class
  VISIBILITY_SYSTEM Ldalvik/annotation/Signature; value={ "<T::" "Ljava/lang/CharSequence;" ">" "Ljava/lang/Object;" }
Annotations on field #0 'xs'
  VISIBILITY_SYSTEM Ldalvik/annotation/Signature; value={ "Ljava/util/List<" "Ljava/lang/String;" ">;" }
Annotations on method #1 'id'
  VISIBILITY_SYSTEM Ldalvik/annotation/Signature; value={ "<R:" "Ljava/lang/Object;" ">(TR;)TR;" }
```

La regla de troceado, deducida de los tres casos y de
`()Ljava/util/Map<Ljava/lang/String;Ljava/lang/Integer;>;`:
cada descriptor de clase `L…;` es un trozo; si el descriptor va seguido de `<`, el trozo termina en
`<`; todo lo que queda entre descriptores (`<T::`, `>`, `()`, `>;`) es un trozo. El arreglo se añade
como `value` de la anotación de clase, campo o método.

Implementado: el lector guarda `Signature` de clase, campo y método (`apkc/classreader.go`);
`firmaEnTrozos`/`anotacionDeFirma` y las listas de campos anotados en `apkc/dexanota.go`; el
directorio de anotaciones escribe `fields_size` y los `field_idx` además de clases y métodos.
Prueba: `apkc/dexanota_firma_test.go` (los cuatro troceados y las tres reglas).

## 102. El `<permission>` del manifiesto y su `Manifest.java`

`aapt2` no rechaza un manifiesto con `<permission>`: además del AXML escribe
`<paquete>/Manifest.java` con las constantes. Medido con `aapt2 link --java`:

```sh
cd /data/data/com.termux/files/usr/tmp/perm/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o app.apk --manifest AndroidManifest.xml --java ../java \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
cat ../java/com/perm/Manifest.java
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
cat src/com/perm/Manifest.java
```

El fichero que escribe este fork es el de aapt2, byte a byte salvo la ausencia de salto final (nosotros
también lo omitimos): cabecera `AUTO-GENERATED`, `package com.perm;`, `public final class Manifest`,
`public static final class permission` y una constante por permiso en el orden del manifiesto:

```text
public static final String CAMARA="com.perm.CAMARA";
public static final String RED="com.perm.RED";
```

Implementado: `manifestJavaDe` recorre los `<permission android:name=…>` del árbol del manifiesto y
`enlazarConGo` escribe `src/<paquete>/Manifest.java` junto a la `R.java`; el cambio de contenido entra
en la detección de recompilación. Prueba: `TestManifestJavaDePermissions`. Queda fuera cualquier
atributo de `<permission>` distinto de `android:name`.

## 103. `layout-land` y `drawable-hdpi`: orientación y densidad

Las dos configuraciones que faltaban de 5.17. Se comparó contra `aapt2 link --no-auto-version`, que
es la forma que coincide con la configuración única de este enlace (el `aapt2` por defecto añade
además una copia `layout-v1` de cada layout, ya aceptada en el §28):

```sh
cd /data/data/com.termux/files/usr/tmp/cal/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link --no-auto-version -o noauto.apk --manifest AndroidManifest.xml \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources noauto.apk
unzip -p noauto.apk resources.arsc > noauto.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp noauto.arsc nuestro.arsc
```

Los dos arsc son **1204 bytes idénticos**. Lo que fija:

| configuración | `ResTable_config` | spec |
| --- | --- | --- |
| `layout-land` | orientation `2` en +12 | `0x0080` |
| `drawable-hdpi` | density `240` en +14 y sdkVersion `4` en +24 | `0x0500` |
| `values-es` | language 'e','s' en +8/+9 | `0x0004` |

La ruta de la densidad lleva el sufijo que usa `aapt2`: `res/drawable-hdpi-v4/icono.png`; la de
orientación, `res/layout-land/main.xml`. `aapt2 dump resources` del apk de este fork declara las dos
entradas. Implementado en `apkc/linker.go` (acepta las carpetas y reescribe la ruta de densidad),
`apkc/arscwriter.go` (los bytes de configuración) y `apkc/valores.go` (`mascaraConfig`). Prueba:
`apkc/valores_cal_test.go` (fixture de 1204 bytes).

## 104. Estado de 5.21 (`v3`) y 5.22 (`invokedynamic`)

**5.21 queda en parte.** `apkc verify` no verifica una firma v3, pero la tolera y lo dice: el cotejo
de `studio/e2e.sh` firma el apk base con `--v3-signing-enabled true` y exige que las dos herramientas
lo acepten y que `apkc verify` anote `no se verificó aquí`.

```sh
sh -c 'cd studio && APKC=... APKC_WASM=... sh e2e.sh' | grep con-v3
# con-v3: apksigner si, apkc verify si
grep -n "idParV3\|no se verificó aquí" apkc/verificar.go
```

Lo que falta es **escribir** v3 (y v4): el bloque `APK Signature Scheme v3` y su prueba de rotación
no están implementados. La pestaña sigue firmando con el keystore de depuración; un keystore de
release propio y `--min-sdk` del proyecto tampoco están en el camino de la página.

**5.22 no se implementa aquí.** El lector de `.class` rechaza `invokedynamic` con nombre, y el
compilador de fuente no tiene lambdas ni referencias de método; implementar la desugración de `d8`
(externalizar `lambda$…`, `method_handle`/`method_type` y la tabla de bootstrap) es la pieza XL del
plan y no se ha medido. El rechazo es deliberado:

```sh
grep -n "invokedynamic" apkc/classreader.go
# pool entry %d is an invokedynamic: lambdas and dynamic concatenation are not dexed here
```

Por tanto la Fase 5 no se declara cerrada: quedan `attr`/`styleable` (5.18), las regiones de 5.17,
la firma v3 escrita (5.21) y `invokedynamic` (5.22).

## 105. `fraction`: radix, 1.0 y el bit de `%p`

El `<fraction>` de un `values` sale tipo **0x06** con un entero de 32 bits. Medido con nueve valores
(0, 10, 25, 50, 75, 100, 150, 50p y 100p) y comparado byte a byte con aapt2:

```sh
cd /data/data/com.termux/files/usr/tmp/frac/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
```

El arsc sale **796 bytes idéntico**. La codificación medida:

| valor | datos |
| --- | --- |
| `0%` | `0x00000000` |
| `10%` | `0x0CCCCD30` (radix 3; el valor de 24 bits se redondea hacia arriba) |
| `25%`, `50%`, `75%` | `0x20000030`, `0x40000030`, `0x60000030` |
| `100%` | `0x00000100` (radix 0) |
| `150%` | `0x00C00020` (radix 2) |
| `50%p`, `100%p` | el mismo valor con el bit bajo a 1 (`0x…31` / `0x101`) |

Implementado: `fraccionDeTexto` en `apkc/valores.go` calcula en `float32` (10% solo coincide así),
redondea el valor de 24 bits con `math.Ceil` y conserva el bit de `%p`. Prueba:
`apkc/valores_fraccion_test.go` (fixture de 796 bytes).

## 106. `attr` y `declare-styleable`

`<attr format="…">` produce una entrada compleja del tipo `attr`; `<declare-styleable>` no produce
entrada en `resources.arsc`, solo la clase `styleable` de `R.java`. Medido con
`aapt2 dump resources` y comparado byte a byte:

```sh
cd /data/data/com.termux/files/usr/tmp/attrs/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml --java ../java \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
aapt2 dump resources aapt2.apk
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
```

El arsc sale **596 bytes idéntico**. La entrada de un attr es compleja (`padre=0`, flags `0x0001`,
un registro) con nombre `0x01000000` (`attr/format`) y tipo entero 0x10; los bits del formato medidos
son:

| format | bits |
| --- | --- |
| `string`, `reference`, `integer`, `boolean` | `0x01`, `0x02`, `0x04`, `0x08` |
| `color`, `float`, `dimension`, `fraction` | `0x10`, `0x20`, `0x40`, `0x80` |

El `R.java` de este fork lleva la clase `styleable` con el arreglo de ids y los offsets; el arreglo se
escribe `new int[] {…}` (y sin `final`) porque el emisor de este fork todavía no da `ConstantValue` a
un arreglo, pero Java lo acepta y `R.styleable.Vista`/`Vista_modo` quedan utilizables. Para el arreglo
de ids hizo falta el `const` 0x14 de 31 bits en el dexer (las ids `0x7f010000` no caben en
`const/high16`).

Implementado: `attrDeElemento`/`styleableDeElemento`/`formatoDeAttr` en `apkc/valores.go`, la clase
`styleable` en `apkc/r.go` y `const` 0x14 en `apkc/dexinstr.go`. Prueba:
`apkc/valores_attrs_test.go` (arsc de 596 bytes y el texto de `R.java`). Frontera: `format="enum"` y
`format="flags"` con sus hijos no están medidos.

## 107. `values-es-rMX`: país dentro de la configuración

Una región añade `country` a la configuración del idioma. Medido con `aapt2` y comparado byte a byte:

```sh
cd /data/data/com.termux/files/usr/tmp/region/proj
aapt2 compile --dir res -o compiled.zip
aapt2 link -o aapt2.apk --manifest AndroidManifest.xml \
  -I "$HOME/Android/platforms/android-34/android.jar" compiled.zip
unzip -p aapt2.apk resources.arsc > aapt2.arsc
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" --keystore "$HOME/.android/debug.keystore"
unzip -p build/app.apk resources.arsc > nuestro.arsc
cmp aapt2.arsc nuestro.arsc
```

El arsc de **680 bytes** es idéntico: `language[0]='e'` y `language[1]='s'` en +8/+9, `country[0]='M'` y
`country[1]='X'` en +10/+11; la máscara del spec sigue siendo `0x0004` (locale). Implementado en
`fuentesDelProyecto` (reconoce `values-xx-rYY`) y `configDe` (`apkc/arscwriter.go`). Prueba:
`apkc/valores_region_test.go`.

## 108. APK Signature Scheme v3, escrito y verificado por `apksigner`

El paquete Go escribe ahora el par v3 (`0xf05368c0`) junto al v2. El signer v3 se midió sobre un apk
que firmó `apksigner --v3-signing-enabled true` y después se reprodujo:

```sh
cd /data/data/com.termux/files/usr/tmp/region/proj
apkc build --linker go --compiler go --signer go \
  --android-jar "$HOME/Android/platforms/android-34/android.jar" \
  --keystore "$HOME/.android/debug.keystore"
apksigner verify --verbose --min-sdk-version 21 build/app.apk
```

Salida:

```text
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
```

La forma medida del signer v3:

| campo | valor |
| --- | --- |
| `signed data` | `digests` y `certificates`, **cada uno con su longitud**, y después `minSdk` 24, `maxSdk` 0x7fffffff y la lista de atributos vacía (u32 0) |
| signer | `signed data` con longitud, minSdk/maxSdk crudos, las firmas con longitud y la clave pública con longitud |
| bloque | un par v2 y otro v3 dentro del mismo APK Signing Block |

El defecto del primer intento fue no envolver `digests`/`certs` en su longitud (faltaban 8 bytes) y
poner la lista de atributos **fuera** del signed data: `apksigner` respondía `Invalid SDK Version
parameter(s)` y `No signatures`. Corregidos los dos, verifica v3. Prueba:
`TestElBloqueV3LlevaSuParYSuSigner` (pares y longitudes del signer); `studio/e2e.sh` exige
`v3 scheme … true` en el apk del proyecto y en el del icono.

## 109. `invokedynamic`: el coste medido de la desugración y por qué queda fuera

Sonda `Lam` con una lambda estática, una referencia de método y una lambda que captura `this`:

```sh
cd /data/data/com.termux/files/usr/tmp/lam/sonda
javac --release 8 -g:lines,source Lam.java
cd .. && d8 --debug --min-api 28 --output d8 sonda/Lam.class sonda/Lam\$*.class
/apex/com.android.art/bin/dexdump -d d8/classes.dex
/apex/com.android.art/bin/dexdump -a d8/classes.dex
```

`d8` no deja el `invokedynamic`: **genera tres clases** y dos métodos sintéticos. Lo medido:

| pieza | qué escribe `d8` |
| --- | --- |
| `Lam$0` | clase `PUBLIC FINAL SYNTHETIC`, campo `f$0:LLam;`, constructor `(LLam;)V`, `f(I)I` que llama a `lambda$captura$1$Lam` |
| `Lam$1` | clase para `Lam::doble`, constructor `()V` y `f` que llama a `doble` |
| `Lam$2` | clase para `x -> x + 1`, `f` que llama a `lambda$uno$0` |
| `lambda$uno$0` / `lambda$captura$1$Lam` | métodos sintéticos en `Lam`, el segundo con el sufijo de la captura |
| anotación | `Lcom/android/tools/r8/annotations/LambdaMethod;` con `holder`, `method` y `proto` |
| bootstrap | `LambdaMetafactory.metafactory` con `samMethodType`, el `MethodHandle` de implementación y `instantiatedMethodType`, leídos del atributo `BootstrapMethods` |

Para emitirlo desde este dexer haría falta leer `CONSTANT_InvokeDynamic` (tag 18), el atributo
`BootstrapMethods`, el `MethodHandle` y los `MethodType`; sintetizar una clase por sitio con campos,
constructor, método SAM y anotación; inyectar los `lambda$…` en la clase dueña; y sustituir la
instrucción por `new`/`dup`/capturas/`invokespecial`. Es la pieza XL del plan (5.22), no un ítem de
una sesión; el lector de `.class` la sigue rechazando por nombre:

```text
pool entry %d is an invokedynamic: lambdas and dynamic concatenation are not dexed here
```

No se declara hecha. La Fase 5 queda abierta por esta pieza (y por el keystore de release/v4 de
5.21).


## 110. `invokedynamic` de una lambda estática sin capturas, desugrado y ejecutado

El único caso de 5.22 que se implementa: `x -> x + 1` dentro de un método `static`. `javac
--release 8` deja el sitio con `LambdaMetafactory.metafactory`, el `MethodType` SAM, el
`MethodHandle` del implementador y el `MethodType` instanciado; este fork lee el sitio y lo
convierte en una clase sintética con constructor y método SAM.

```sh
cd /data/data/com.termux/files/usr/tmp/lamok
javac --release 8 -g:lines,source -d clases src/Lam.java
apkc dex -o lamok.dex clases
dexdump -d lamok.dex
sh banco/lambda.sh
```

`dexdump -d` de nuestro fichero:

```text
Class descriptor  : 'LLam$$Lambda$0;'
  name          : '<init>'
000220: 7010 0800 0000  invoke-direct {v0}, Ljava/lang/Object;.<init>:()V
  name          : 'f'
000238: 7110 0400 0200  invoke-static {v2}, LLam;.lambda$uno$0:(I)I
...
  name          : 'uno'
00026c: 2200 0100           new-instance v0, LLam$$Lambda$0;
000270: 7010 0000 0000      invoke-direct {v0}, LLam$$Lambda$0;.<init>:()V
```

`banco/lambda.sh` compila con `javac`, dexa con este fork y ejecuta el `.dex` en el aparato con
`app_process64`; la salida es la misma que la JVM y `dexdump` lo verifica:

```text
  jvm (javac): rc=0, salida: 42
  aparato: rc=0, salida: 42
  dexdump verificando: rc 0
lambda: la lambda estatica sin capturas se desugro y corre igual que la JVM
```

Lo que se mide y lo que se rechaza:

| pieza | qué hace este fork |
| --- | --- |
| `CONSTANT_InvokeDynamic` (tag 18), `BootstrapMethods`, `MethodHandle` (15) y `MethodType` (16) | se leen; el `invokedynamic` deja de rechazarse |
| bootstrap `LambdaMetafactory.metafactory` con 3 argumentos | se acepta si el SAM y el tipo instanciado coinciden (nada de SAM genérico) |
| implementador | `REF_invokeStatic` de la propia clase; a un `private static synthetic` se le quita `private`, como hace `d8` |
| clase sintética | `PUBLIC FINAL SYNTHETIC`, la interfaz SAM, `<init>()V` y el método SAM que llama al implementador |
| sitio en el llamador | `new-instance` + `invoke-direct {v}, <init>()V`; el valor queda en el mismo registro |
| capturas (`(I)`, `(LLam;)`) y `this::m` | se desugran con campos de captura en el §111 |
| referencias sin receptor (`Tipo::m`) | se desugran en el §112 |
| SAM genérico y borrado a una cota | se desugran con el descriptor borrado y `checkcast` en los §113 y §114 |
| `altMetafactory` serializable y con marcadores | se desugra con las interfaces extra en el §115 |
| concatenación dinámica (`makeConcatWithConstants`) | se desugra a `StringBuilder` en el §116 |
| puentes de `altMetafactory` y constantes dinámicas | se rechazan por nombre |

Implementado en `apkc/lambda.go` (expansión, validación y clase sintética), `apkc/classreader.go`
(`BootstrapMethods` y los tags del pool), `apkc/dextabla.go` (internado del sitio),
`apkc/dexinstr.go` (el `new`/`invoke-direct`) y `apkc/jvmopcode.go` (el ancho 0xba). Pruebas:
`apkc/lambda_test.go` (fixture de `javac`, estructura de la clase y del dex) y `banco/lambda.sh`
(ejecución en ART; desde el §111 cubre también las capturas y `this::m`). El compilador de fuente de
este fork sigue sin aceptar lambdas: esto mide el dexer sobre `.class` de `javac`, no la sintaxis.

## 111. Lambdas con capturas: un campo por valor y el receptor de `this::m`

La lambda que usa una local o `this` llega con esos valores como parámetros del descriptor del sitio
(`(I)LLamLoc$F;`, `(LLamThis;)LLamThis$F;`). La clase sintética los guarda en campos, el constructor
los recibe y el método SAM los carga antes de los argumentos, igual que hace `d8` con sus `f$0`
(§109). Se midieron cuatro casos con `javac --release 8` → `apkc dex` → ART:

```sh
sh banco/lambda.sh
```

Salida:

```text
  Lam: jvm rc=0 «42» · aparato rc=0 «42»
  LamLoc: jvm rc=0 «42» · aparato rc=0 «42»
  LamThis: jvm rc=0 «42» · aparato rc=0 «42»
  LamRef: jvm rc=0 «42» · aparato rc=0 «42»
  dexdump verificando: rc 0
lambda: las cuatro lambdas se desugraron y corren igual que la JVM
```

Lo que fija cada caso:

| caso | sitio | sintética | implementador |
| --- | --- | --- | --- |
| `x -> x + 1` (static) | `()LLam$F;` | `()V`, sin campos | `REF_invokeStatic lambda$uno$0(I)I` |
| captura un local `y` | `(I)LLamLoc$F;` | `(I)V`, campo `I` | `REF_invokeStatic lambda$con$0(II)I` (captura primero) |
| captura `this` | `(LLamThis;)LLamThis$F;` | `(LLamThis;)V`, campo `LLamThis;` | `REF_invokeSpecial lambda$captura$0(I)I`; se le quita `private` y el SAM usa `invoke-virtual` |
| `this::suma` | `(LLamRef;)LLamRef$F;` | `(LLamRef;)V`, campo `LLamRef;` | `REF_invokeVirtual LamRef.suma(I)I` |

El orden es el del JVM: el receptor de una referencia de instancia es la **primera** captura, las
demás capturas van en los campos y después los argumentos del SAM. En el llamador, el `invokedynamic`
se cambia por `new-instance` y `invoke-direct {objeto, capturas…}, <init>`, con las capturas que ya
estaban en la pila.

Pruebas: `apkc/lambda_test.go` (los cuatro casos) y `banco/lambda.sh`. La referencia sin receptor se
mide en el §112.

## 112. Referencia a un método de instancia sin receptor (`Tipo::m`)

`LamUnb::suma` no captura nada: el sitio es `()LLamUnb$F;` y el **primer parámetro del SAM** es el
receptor (`(LLamUnb;I)I`). La clase sintética no lleva campos; su método SAM carga todos los
argumentos y llama con `invoke-virtual`, usando el primero como objeto. Medido y ejecutado:

```sh
sh banco/lambda.sh
```

```text
  LamUnb: jvm rc=0 «42» · aparato rc=0 «42»
  dexdump verificando: rc 0
lambda: las cinco lambdas se desugraron y corren igual que la JVM
```

La validación distingue los dos caminos del receptor: si hay capturas, la primera tiene que ser el
dueño (`this::m`); si no hay capturas, el primer parámetro del SAM tiene que ser el dueño
(`Tipo::m`), y los parámetros del implementador son los del SAM menos ese receptor. Prueba:
`TestLaReferenciaSinReceptor` en `apkc/lambda_test.go` y el caso `LamUnb` de `banco/lambda.sh`.

Sigue fuera: nada de lo que `javac` emite para una lambda. El SAM genérico se mide en el §113,
`altMetafactory` en el §115 y la concatenación dinámica de JDK 9 en el §116.

## 113. SAM genérico: descriptor borrado y `checkcast` al tipo instanciado

`Function<String,Integer> f = s -> s.length()` tiene dos tipos en el bootstrap: el **borrado**
`(Ljava/lang/Object;)Ljava/lang/Object;`, que es el que implementa la interfaz, y el **instanciado**
`(Ljava/lang/String;)Ljava/lang/Integer;`, que es el del implementador. La clase sintética declara
el SAM borrado y, antes de llamar, moldea el argumento:

```sh
sh banco/lambda.sh
```

```text
  LamGen: jvm rc=0 «4» · aparato rc=0 «4»
  dexdump verificando: rc 0
lambda: las seis lambdas se desugraron y corren igual que la JVM
```

`dexdump -d` del SAM de la sintética:

```text
  name          : 'apply'
000230: 1f02 0600    check-cast v2, Ljava/lang/String;
000234: 7110 0300 0200 invoke-static {v2}, LLamGen;.lambda$main$0:(Ljava/lang/String;)Ljava/lang/Integer;
```

La validación admite que el parámetro borrado sea `Object` y el instanciado una referencia; el
implementador se comprueba contra el tipo instanciado. En el cuerpo, cada parámetro que difiere se
carga con su tipo del SAM y se moldea con `checkcast` (0xc0) antes de la llamada; la vuelta usa el
tipo del SAM (`Object` acepta el `Integer` del implementador, que ya viene encajado por `javac`).
Prueba: `TestElSamGenericoUsaElDescriptorBorrado` y el caso `LamGen` de `banco/lambda.sh`.

Frontera: el borrado a un tipo que no sea `Object` se mide en el §114; `altMetafactory`
(serializables y marcadores) en el §115 y la concatenación dinámica de JDK 9 en el §116.

## 114. Borrado a una cota (`T extends Number`) y el orden del `annotation_set`

Una interfaz genérica con cota borra a esa cota: `F<T extends Number>` deja el SAM
`(Ljava/lang/Number;)Ljava/lang/String;` y el tipo instanciado `(Ljava/lang/Integer;)…`; la clase
sintética implementa el SAM con `Number` y su cuerpo moldea a `Integer` antes de llamar.

```sh
sh banco/lambda.sh
```

```text
  LamBound: jvm rc=0 «42» · aparato rc=0 «42»
  dexdump verificando: rc 0
lambda: las siete lambdas se desugraron y corren igual que la JVM
```

La validación ya no exige que el borrado sea `Object`: acepta cualquier par de tipos de referencia
en el parámetro y moldea con `checkcast`; los primitivos tienen que coincidir. El constructor y los
campos no cambian respecto del §113.

**Defecto que destapó esta sonda**: el `annotation_set_item` tiene que ir **ordenado por `type_idx`**
y este fork escribía las anotaciones en el orden de las reglas. Una clase con `Signature` y
`EnclosingClass`/`InnerClass` (la interfaz genérica del caso) salía con el conjunto desordenado y
`dexdump` la rechazaba con `Out-of-order entry types: 4 then 2`. Ahora `armaAnotaciones` ordena cada
conjunto por `type_idx` antes de escribirlo (`apkc/dexanota.go`). Sin eso, el dex ni siquiera se
verificaba.

Prueba: `TestElSamConCotaUsaElDescriptorBorrado` (descriptor `Number`, `checkcast` y dex) y el caso
`LamBound` de `banco/lambda.sh`, que pasa por `dexdump` y por `app_process64`.

## 115. `altMetafactory`: lambda serializable y marcadores de una intersección

Cuando el tipo destino no es una interfaz sola, `javac` cambia el bootstrap a
`LambdaMetafactory.altMetafactory`. Los tres primeros argumentos son los de `metafactory` (SAM,
implementador y tipo instanciado) y detrás vienen los `flags`, la cuenta de marcadores con sus
clases y la cuenta de puentes con sus `MethodType`. Los bits medidos son `1` (serializable), `2`
(marcadores) y `4` (puentes).

```sh
P=/data/data/com.termux/files/usr
W=$P/tmp/lamser
cat >"$W/src/LamSer.java" <<'J'
public class LamSer {
    interface R extends Runnable, java.io.Serializable {}
    static R make() { return () -> System.out.println("hola"); }
    public static void main(String[] a) { make().run(); }
}
J
"$P/lib/jvm/java-21-openjdk/bin/javac" --release 8 -g:lines,source -d "$W/clases" "$W/src/LamSer.java"
"$HOME/Android/build-tools/34.9.0/d8" --release --min-api 21 --output "$W/d8" \
  "$W/clases/LamSer.class" "$W/clases/LamSer\$R.class"
/apex/com.android.art/bin/dexdump "$W/d8/classes.dex"
```

`javap -v` del sitio de `LamSer` deja `()V`, el `MethodHandle` del implementador, `()V`, `5` y `0`:
`5` es serializable **y** puentes, y la cuenta de puentes es cero. `d8` no escribe ni `writeReplace`
ni `$deserializeLambda$`: borra el `$deserializeLambda$` que dejó `javac` y añade
`java/io/Serializable` a las interfaces de la clase sintética. El orden que escribe es el del sitio,
luego los marcadores y `Serializable` al final:

```text
  Class descriptor  : 'LLamSer$0;'
  Interfaces        -
    #0              : 'LLamSer$R;'
    #1              : 'Ljava/io/Serializable;'
```

Con una intersección de dos marcadores, `(R & Runnable & Serializable & Cloneable)`, `javac` deja
`flags 7` y los marcadores `R` y `Cloneable`; `d8` los lista en ese orden detrás del tipo del sitio:

```text
    #0              : 'Ljava/lang/Runnable;'
    #1              : 'LLamSer3$R;'
    #2              : 'Ljava/lang/Cloneable;'
    #3              : 'Ljava/io/Serializable;'
```

Este fork hace lo mismo: lee los flags y los marcadores, añade esas interfaces a la clase sintética,
quita `$deserializeLambda$` del dueño y deja el dex sin `writeReplace`, como `d8`. Un `altMetafactory`
con puentes distintos de cero se rechaza por nombre, porque no está medido. La lambda corre en ART
junto a las siete anteriores:

```sh
APKC=$P/tmp/apkc-gate sh banco/lambda.sh
```

```text
  LamSer: jvm rc=0 «hola» · aparato rc=0 «hola»
  LamSerCap: jvm rc=0 «hola7» · aparato rc=0 «hola7»
  LamMarcas: jvm rc=0 «x» · aparato rc=0 «x»
  dexdump verificando: rc 0
lambda: las diez lambdas se desugraron y corren igual que la JVM
```

`LamSerCap` captura un `int` y demuestra que el flag serializable no cambia el campo ni el
constructor; `LamMarcas` es la intersección con dos marcadores. Pruebas:
`TestLaLambdaSerializableAceptaAltMetafactory`, `TestLaLambdaSerializableConCapturas` y
`TestLosMarcadoresDeLaInterseccion` en `apkc/lambda_test.go`, más los tres casos de
`banco/lambda.sh` que pasan por `dexdump` y `app_process64`.

Frontera: la serialización de ida y vuelta no se mide; se mide que el dex se construya y ejecute
como el de `d8`. Los puentes de `altMetafactory` siguen fuera y se rechazan por nombre; la
concatenación dinámica de JDK 9 se mide en el §116.

## 116. `makeConcatWithConstants`: la concatenación dinámica de JDK 9

Desde Java 9, `javac` no escribe `StringBuilder` para `a + b + c`: deja un `invokedynamic` cuyo
bootstrap es `java/lang/invoke/StringConcatFactory.makeConcatWithConstants`. El primer argumento del
bootstrap es una **receta**: cada `\u0001` consume el siguiente argumento dinámico y cada `\u0002` la
siguiente constante de la lista que viene detrás. Con `--min-api 21`, `d8` lo baja otra vez a
`new-instance StringBuilder`, un `append` por trozo y `toString`.

```sh
P=/data/data/com.termux/files/usr
W=$P/tmp/lamser
cat >"$W/src/ConcatB.java" <<'J'
public class ConcatB {
    public static void main(String[] a) {
        int n = a.length;
        String r = "a" + "\u0001" + "b" + "\u0002" + "c" + n;
        System.out.println(r.length() + ":" + r.equals("a\u0001b\u0002c" + n));
    }
}
J
"$P/lib/jvm/java-21-openjdk/bin/javac" --release 11 -g:lines,source -d "$W/clases" "$W/src/ConcatB.java"
"$P/lib/jvm/java-21-openjdk/bin/javap" -v -cp "$W/clases" ConcatB
"$HOME/Android/build-tools/34.9.0/d8" --release --min-api 21 --output "$W/d8catb" "$W/clases/ConcatB.class"
/apex/com.android.art/bin/dexdump -d "$W/d8catb/classes.dex"
```

`javap -v` deja la receta `\u0002\u0001` y una sola constante, `a\u0001b\u0002c`. `\u0002` aparece
justo cuando el texto contiene los marcadores; sin ellos, `javac` dobla el literal dentro de la
propia receta y la lista de constantes queda vacía. `d8` escribe:

```text
0001: new-instance v0, Ljava/lang/StringBuilder;
0003: const-string v1, "a\001b\002c"
0005: invoke-direct {v0, v1}, Ljava/lang/StringBuilder;.<init>:(Ljava/lang/String;)V
0008: invoke-virtual {v0, v5}, Ljava/lang/StringBuilder;.append:(I)Ljava/lang/StringBuilder;
000c: invoke-virtual {v0}, Ljava/lang/StringBuilder;.toString:()Ljava/lang/String;
```

El constructor lleva el primer texto cuando la receta empieza por literal o por constante; si empieza
por `\u0001`, `d8` usa el constructor vacío. Los `append` que elige son los del tipo del argumento:
`(I)`, `(J)`, `(F)`, `(D)`, `(C)`, `(Z)`, `(Ljava/lang/String;)` y, para una referencia cualquiera,
`(Ljava/lang/Object;)`. `javac` convierte antes las referencias y los arreglos con
`String.valueOf`, así que en la práctica el descriptor solo trae `String` y primitivos.

Este fork trocea la receta igual y baja cada sitio a `new-instance` + `<init>()V` + un `append` por
trozo + `toString`, con el constructor vacío siempre y sin encadenar el `move-result-object` de cada
`append`: el comportamiento es el mismo que el de `d8`, aunque la forma no sea idéntica. Está en
`apkc/concat.go`; el sitio se valida en `expandirLambdas` (`apkc/lambda.go`) y se guarda en
`classFull.Concats`, el internador lo sigue en `dextabla.collectCode` y el traductor lo despacha en
`dexinstr.go`. Se rechazan por nombre la receta que no cuadra con el descriptor o con las
constantes, las constantes dinámicas del bootstrap y cualquier otro bootstrap de
`StringConcatFactory`.

La ejecución en ART, junto a las diez lambdas del §115:

```sh
APKC=$P/tmp/apkc-gate sh banco/lambda.sh
```

```text
  Concat: jvm rc=0 «n=0!» · aparato rc=0 «n=0!»
  ConcatB: jvm rc=0 «6:true» · aparato rc=0 «6:true»
  ConcatW: jvm rc=0 «ztruecxj2f1.5d2.5i0ssoobj» · aparato rc=0 «ztruecxj2f1.5d2.5i0ssoobj»
  dexdump verificando: rc 0
lambda: las trece lambdas y concatenaciones se desugraron y corren igual que la JVM
```

`Concat` es la forma básica; `ConcatB` es la receta con `\u0002`; `ConcatW` conserva un argumento
dinámico de cada tipo y comprueba que el dex nombra los siete `append`. Pruebas:
`TestLaConcatenacionConConstantesDelBootstrap` y `TestLaConcatenacionEligeElAppendDeCadaTipo` en
`apkc/concat_test.go`, más los tres casos de `banco/lambda.sh` que pasan por `dexdump` y
`app_process64`.

Frontera: los puentes de `altMetafactory` con cuenta distinta de cero y las constantes dinámicas de
la concatenación siguen fuera y se rechazan por nombre; `javac` no produce ninguna de las dos.

