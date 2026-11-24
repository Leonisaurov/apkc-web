# Evidencia — §28-§39

## 28. El escritor de tablas de cadenas de recursos, byte a byte contra `aapt2`, y lo que separa
## nuestra tabla de la suya

`apkc/poolrecurso.go` escribe un `RES_STRING_POOL`. La prueba no es un muestreo: se toman los tres
pools del `resources.arsc` fijado (el global en `@12`, UTF-8, 56 bytes; el de nombres de tipo en
`@356`, UTF-16, 80; el de claves en `@436`, UTF-8, 68), se leen con el lector de la 3-B para sacar
las cadenas, y lo que produce el escritor se compara **byte a byte** con los bytes originales. Los
tres salen idénticos.

Lo que hubo que copiar de los bytes, porque el manual no lo deja claro: los offsets del array son
**relativos al inicio de los datos** (no al del trozo), el modo UTF-8 lleva **dos** longitudes
(bytes y unidades utf-16) y el UTF-16 **una**, cada cadena termina con un cero (dos bytes en
UTF-16) y el área de datos se rellena hasta múltiplo de cuatro. Con una cadena de 200 caracteres y
otra del plano suplementario la ida y vuelta por el lector cierra el resto.

Y medida, para que nadie lo descubra escribiendo, la razón por la que la tabla de recursos **no**
puede salir byte a byte con una sola configuración: los dos bloques `id=3` del `AppTheme` del
proyecto difieren en **un byte** de su `ResTable_config` — el `+28` vale `0x00` en el primero y
`0x17` (23) en el segundo — y el bloque con `23` es el que trae el registro extra
(`0x010104e0`, un atributo que nació en API 23). Es decir: `aapt2` **parte los ítems de un estilo
por el nivel de API en que se introdujo cada atributo**, y emite una configuración base y una
`-v23`. Reproducir eso exige la tabla atributo a API del framework, que aquí no hay de dónde sacarla
sin `android.jar` parseado. Así que el plan para 3-C es emisor de **una sola configuración base**
con todos los ítems (válida, instalable, y comprobable con `aapt2 dump resources`), y diferencia
documentada en el número de configuraciones — no en los ids, ni en las cadenas, ni en los valores.

Comprobación:

```sh
sh -c 'cd apkc && go test -run "Pool" -v .'
```

**Resultado**: dos pruebas en verde. Nativa **147 PASS / 0 FAIL**; la misma binaria en `GOOS=js`
en el hospedador de navegador **142 PASS + 6 SKIP, rc 0**.

## 29. El cuerpo del `resources.arsc`, escrito en Go: byte a byte con `aapt2` salvo donde aapt2 abre
## la tabla en dos configuraciones

Primero hubo que cerrar una incógnita que el lector de la 3-B dejaba cruda: **qué significan las
banderas de una entrada**. Para no adivinarla se reconstruyó el objetivo con el `aapt2` de este
teléfono —los mismos bytes del fixture, `sha256 4b2b7ec0…`, comprobado— y se varió el `res/values`,
volcando después cada `ResTable_typeSpec` y cada `ResTable_entry` con el instrumento del §24 (que
hasta hoy no imprimía las banderas del spec; ya las imprime):

```sh
A=$HOME/Android/build-tools/34.0.0/aapt2; J=$HOME/Android/platforms/android-34/android.jar
P=$PREFIX/tmp/flagprobe; rm -rf $P; mkdir -p $P/res/values $P/res/layout
cp $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/{AndroidManifest.xml} $P/
cp -r $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/res/layout $P/res/
cp $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/res/values/styles.xml $P/res/values/
cd $P && $A compile --dir res -o flats.zip
$A link -o out.apk -I $J --manifest AndroidManifest.xml --auto-add-overlay -R flats.zip
unzip -p out.apk resources.arsc | sha256sum          # 4b2b7ec08c98da92… el fixture, idéntico
# variante A: + <item type=id>, <string>, <bool>, <color> y un segundo estilo sin atributo de API 23
# variante B: el AppTheme sin windowLightStatusBar (el atributo que nació en API 23)
# variante C: los mismos recursos con <public name=… type=…/> y un <declare-styleable>
env APKC_ARBOLES="$P/xA/resources.arsc $PREFIX/tmp/flagprobe2/x/resources.arsc" \
  go test -C apkc -run TestArbolDeTroz -v
```

Y la regla, medida sobre las tres variantes (no inferida del `header.h`):

| qué | `ResTable_entry::flags` | `ResTable_typeSpec::entryFlags[i]` |
| --- | --- | --- |
| entrada de tipo `id` (declarada en `values` o con `@+id/` en un layout) | `0x0004` | `0x00000000` |
| entrada de tipo `style` o `attr` (valor complejo) | `0x0001` | `0x00000000` |
| entrada declarada con `<public>` | `0x0002` | `0x40000000` |
| resto (`layout`, `string`, `bool`, `color`) | `0x0000` | `0x00000000` |
| un estilo **partido por API** (el `AppTheme` con un atributo de API 23) | `0x0001` | `0x00000400` |

Tres cosas más salieron de esas variantes, todas antes supuestas y ahora vistas:

- El `u16` del spec en `+10` **es el número de bloques `RES_TABLE_TYPE` que siguen para ese id**: vale
  `1` en `id`, `layout`, `bool`, `color`, `string` y `attr`, y `2` en `style` cuando el estilo se
  parte. En la variante B (mismo estilo, sin el atributo de API 23) vuelve a `1` y la tabla mide 924
  en vez de 1088: **exactamente un bloque de 164 bytes menos**.
- Los tipos van **ordenados alfabéticamente por nombre** y el `id` de tipo es su posición más uno
  (`attr, bool, color, id, layout, string, style` → `01..07`); las claves del pool siguen el mismo
  orden (por tipo y dentro del tipo por nombre), y los registros de un estilo van ordenados por su
  `id` de atributo.
- `lastPublicType` y `lastPublicKey` del paquete valen **0 incluso declarando `<public>`**; el
  `typeIdOffset` también. La cabecera del paquete son 288 bytes: 8 de trozo, 4 de **número** de
  paquete (mide `0x7f`, no `0x7f000000` — los ids de recurso salen de desplazarlo 24), 256 de nombre
  UTF-16 y cinco `u32` detrás.

Con eso `apkc/arscwriter.go` escribe la tabla entera: cabecera, pool global, paquete con sus dos
pools de nombres y, por tipo, un `RES_TABLE_TYPE_SPEC` de 16 + 4·entradas seguido de su
`RES_TABLE_TYPE` de cabecera 84 (20 de campos + la configuración base de 64, cuyo único campo no nulo
es su propio tamaño), con `entriesStart = 84 + 4·entradas` y los offsets relativos a él. Una
configuración, como decidió el §28.

La comparación es byte a byte, con las dos únicas diferencias anunciadas antes de medir:

- Los primeros **752 bytes** —cabecera de tabla, pool global, paquete, los dos pools de nombres, y
  los tipos `id` y `layout` enteros— son idénticos a los del fixture **salvo dos campos**: el
  `file_size` de la tabla (en `+4`) y el del paquete (en `+72`), que cambian porque nosotros
  escribimos una configuración en vez de dos. El test lo que hace es comprobar que el fixture anuncia
  1088 y 1020 en esos dos sitios, sustituirlos por los nuestros, y comparar.
- Después de `@752` el `AppTheme` sale distinto por diseño: `aapt2` emite base (4 registros) + `-v23`
  (5) y nosotros un bloque con los **5** registros en la configuración base. Nuestra tabla mide 936
  bytes = 1088 − 152 (el bloque base del estilo que no escribimos).

Y el oráculo de verdad, que no es nuestro lector: metida nuestra tabla en un zip con forma de apk,
`aapt2 dump resources` la lee y imprime **el mismo listado** que imprime para la tabla de `aapt2`, con
`0x7f010000 id/pulsar`, `0x7f020000 layout/main`, `0x7f030000 style/AppTheme` y el estilo con
`size=5 parent=0x0103012b` y sus cinco atributos (el de la fijación lista 4 en la base y 5 en la
`-v23`). El aviso `no such path exists` que imprime nuestra variante es del zip de prueba —no metió
`res/layout/main.xml`—, no de la tabla.

```sh
A=$HOME/Android/build-tools/34.0.0/aapt2
env APKC_ESCRIBE_ARSBC=$PREFIX/tmp/flagprobe/nuestro.arsc \
  go test -C apkc -run TestEscribeTablaContraFixture .
mkdir -p $PREFIX/tmp/flagprobe/ours && cp $PREFIX/tmp/flagprobe/nuestro.arsc $PREFIX/tmp/flagprobe/ours/resources.arsc
cp $PREFIX/tmp/flagprobe/x/AndroidManifest.xml $PREFIX/tmp/flagprobe/ours/
(cd $PREFIX/tmp/flagprobe/ours && zip -X -0 -q ../nuestro.apk resources.arsc AndroidManifest.xml)
$A dump resources $PREFIX/tmp/flagprobe/nuestro.apk   # y lo mismo sobre out.apk, para cotejar
```

Comprobación del escritor:

```sh
go test -C apkc -run "EscribeTabla" -v .
```

**Resultado**: dos pruebas en verde (la de byte a byte con el fixture y la de los seis rechazos:
clave inexistente, `id` de tipo que no es su lugar, spec sin bandera por entrada, registro con nombre
fuera del framework, nombre de paquete vacío y nombre de paquete de 128 caracteres). Contando solo
pruebas de primer nivel la suite nativa está en **147 PASS / 0 FAIL / 1 SKIP** y con subpruebas en
**149 PASS / 1 SKIP**; la misma binaria en `GOOS=js` dentro del hospedador de navegador da **142 PASS
+ 6 SKIP, rc 0** (144 y 6 contando subpruebas). Queda dicho el modo de contar, porque los párrafos
anteriores usaban el otro y en más de un sitio se leen dos unidades de diferencia.

## 30. El AXML del layout y el del manifiesto, escritos en Go: byte a byte con `aapt2` en los dos

`apkc/axmlwriter.go` escribe los binarios XML. La comparación es la misma que en el §29 pero aquí **no
hay diferencia que declarar**: los 784 bytes del `res/layout/main.xml` y los 2000 del
`AndroidManifest.xml` que produjo el SDK de este teléfono salen **idénticos byte a byte**. Los dos
ficheros quedan fijados en base64 (`apkc/recursosfixture_test.go`, con sus longitudes asertadas) y la
prueba recorre los dos casos.

Lo que hubo que medir, porque el manual no lo dice o miente:

- **El orden del pool**: primero los nombres de atributo **que tienen id de recurso, ordenados por ese
  id** —que es exactamente el orden del `XML_RESOURCE_MAP`— y después el resto de las cadenas
  ordenadas por valor ASCII, con las mayúsculas antes que las minúsculas. En el layout: 6 + 8; en el
  manifiesto: 12 + 19. El orden del fuente no tiene nada que ver: en el fuente el `TextView` declara
  `style` el primero y en el binario sale el último.
- **Los atributos de un elemento salen reordenados**: los de namespace por su id y luego los sin
  namespace por nombre. Medido en los cuatro del `LinearLayout` y en los siete de `<manifest>`.
- **El campo `ns` de un atributo y de un elemento es la posición del URI, no la del prefijo.**
  Medido con un layout de dos namespaces (`tools:` incluido): los trozos de namespace llevan
  prefijo (3) y URI (4), y el atributo apunta a **4**.
- **`idIndex`, `classIndex` y `styleIndex` son índices 1-based** dentro de la lista ya ordenada de
  atributos del propio elemento, y 0 si no hay. Medido: `android:id` el primero del `Button` da 1, y
  `style` el último del `TextView` da 4.
- **El valor crudo solo existe para las cadenas**, y entonces crudo y datos apuntan a la MISMA cadena
  del pool (`text="Hello World!"` → raw 7 y datos 7). En los tipos numérico, de referencia y de enum o
  flags el crudo vale −1. Es el campo que me faltó en el primer intento: salían 2 bytes distintos en
  el layout y 8 en el manifiesto, y los ocho eran siempre los cuatro bytes de `datos` de un valor de
  tipo `0x03`.
- **Los tipos de valor que usan los dos ficheros**, copiados crudos: `0x01` referencia, `0x03` cadena,
  `0x10` entero decimal (con `0xffffffff` para −1 y `0xfffffffe` para −2, que son `match_parent` y
  `wrap_content`), `0x11` entero hexadecimal (`gravity="center"` → 0x11), `0x12` booleano (`true` →
  0xffffffff, `false` → 0).
- **`aapt2` tira los atributos de los namespaces que no conoce.** Probado con `tools:ignore` y
  `tools:visibility` sobre un `FrameLayout`: ni los atributos ni la declaración `xmlns:tools` llegan
  al binario (el fichero de tres namespaces declarados tiene un solo `START_NAMESPACE` y cinco
  cadenas). Así que la decisión de qué se descarta es del que enlaza, no del escritor.
- **Cuatro atributos del manifiesto no están en el fuente**: `compileSdkVersion` (34),
  `compileSdkVersionCodename` ("14"), `platformBuildVersionCode` (34) y `platformBuildVersionName`,
  que sale como **entero** 14 con crudo −1, no como cadena.

Comprobación:

```sh
go test -C apkc -run "EscribeAXML" -v .
# y por fuera, con el oráculo que no comparte código con nosotros:
env APKC_ESCRIBE_AXML=$PREFIX/tmp/axml-out go test -C apkc -run EscribeAXMLContraFixture .
mkdir -p $PREFIX/tmp/axml-out/paquete/res/layout && cd $PREFIX/tmp/axml-out &&
  cp AndroidManifest.xml paquete/ && cp main.xml paquete/res/layout/ &&
  cp $PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/build/resources.arsc paquete/
(cd paquete && zip -X -0 -q ../nuestro.apk AndroidManifest.xml resources.arsc res/layout/main.xml)
A=$HOME/Android/build-tools/34.0.0/aapt2
for f in AndroidManifest.xml res/layout/main.xml; do
  $A dump xmltree --file $f nuestro.apk > n.txt; $A dump xmltree --file $f .../build/app.apk > a.txt
  diff n.txt a.txt
done
```

**Resultado**: dos pruebas en verde (la byte a byte sobre los dos ficheros, con su lectura de vuelta
por el lector de la 3-B, y la de los seis rechazos: atributo de namespace sin id, atributo sin
namespace con id, prefijo no declarado, dos ids para el mismo nombre, documento sin raíz y declaración
de namespace sin URI). El `aapt2 dump xmltree` responde rc 0 sobre nuestro paquete y su volcado es
idéntico al del apk de aapt2 en los dos ficheros. Suite nativa: **149 PASS / 0 FAIL / 1 SKIP** de
primer nivel (151 con subpruebas); la misma binaria en `GOOS=js` en el hospedador de navegador **144
PASS + 6 SKIP, rc 0** (146 con subpruebas).

## 31. La `R` sale de nuestra propia tabla, y el texto es el mismo que escribe `aapt2 link --java`

Paso 3.4: `apkc/r.go` genera `R.java` a partir del modelo que alimenta a `escribeTablaRecursos`, así
que los números de la `R` y los del `resources.arsc` **son la misma cuenta**, no dos programas
contándose el uno al otro.

Medido sobre el proyecto del studio y sobre la variante con siete tipos (`link --java genD`):

- El id es `número de paquete << 24 | id de tipo << 16 | INDICE de la entrada dentro del tipo`. Con
  dos entradas por tipo: `id_bare=0x7f040000` y `pulsar=0x7f040001`; `AppTheme=0x7f070000` y
  `StyBare=0x7f070001`. No es un desplazamiento en bytes: es la posición.
- Las clases anidadas van en el orden de los tipos (alfabético por nombre de tipo, que es el orden en
  que aapt2 los numera) y las entradas en el orden en que se emiten, también alfabético por nombre de
  recurso: con `normal`, `con-guion.y.punto` y `str_simple` los ids 0x7f060000, 0x7f060001 y
  0x7f060002 caen en ese orden.
- El nombre se desfigura a identificador de Java: **`con-guion.y.punto` sale `con_guion_y_punto`** —
  fuera de letras, dígitos, `_` y `$`, todo carácter pasa a `_`.
- El texto, con `cat -A`: cabecera de siete líneas de comentario, línea en blanco, `package X;`,
  línea en blanco, `public final class R {`, **dos espacios** por nivel, `= ` sin espacios alrededor
  (`int pulsar=0x7f010000;`), y el fichero **termina en la llave de cierre sin salto de línea**: 493
  bytes exactos para el proyecto del studio.
- Confirmación cruzada del §29: `aapt2 dump resources` etiqueta **PUBLIC** a las entradas declaradas
  con `<public>` — las que llevaban `0x0002` en `ResTable_entry` — y NO etiqueta a los estilos, que
  llevaban `0x0001`. Así que el bit público es el `0x0002` y el `0x0001` de un valor complejo es otra
  cosa; el escritor no lo interpreta, lo copia.

Comprobación:

```sh
A=$HOME/Android/build-tools/34.0.0/aapt2; J=$HOME/Android/platforms/android-34/android.jar
$A link -o out.apk -I $J --manifest AndroidManifest.xml --auto-add-overlay -R flats.zip \
   --java genjava                       # y comparar genjava/…/R.java con lo que escribe r.go
go test -C apkc -run "TestRLaMismaQueAapt2|TestRDesfiguraNombresYOrdena|TestRRechaza" -v .
```

**Resultado**: tres pruebas en verde — la R nuestra es idéntica byte a byte a los 493 bytes de aapt2,
la de la variante de siete tipos casa con las líneas medidas, y rechaza clave inexistente, tipo sin
nombre, entradas fuera de orden y dos nombres que chocan al desfigurarlos. Suite nativa **152 PASS /
0 FAIL / 1 SKIP** de primer nivel (154 con subpruebas); la binaria `GOOS=js` en el hospedador de
navegador, **rc 0** con 147 PASS + 6 SKIP de primer nivel (149 con subpruebas).

Con esto la Fase 3 tiene escritas **las tres estructuras que produce `aapt2`** —tabla de recursos,
AXML del manifiesto y AXML del layout— y la `R`, todas contra el oráculo: falta 3-E, que es
empaquetarlas en un apk y verificarlo por fuera.

## 32. Un apk hecho enteramente en Go: firma v1 y v2, alineado, y `aapt2 dump badging` lo entiende

Paso 3-E: `apkc/empaquetar.go` reúne los artefactos que ya escriben este fork —el AXML del manifiesto,
el del layout, la tabla de recursos y el `classes.dex` que saca el dexer de la Fase 2 de las seis
clases de javac fijadas— y los mete en el apk con el **mismo** empaquetador y el **mismo** firmante que
usa `apkc build --signer go` y `apkc sign`. No pasa `zipalign` ni pasa `apksigner`: la alineación se
decide entrada a entrada (almacenada a cuatro, nativa a página) y la firma se calcula sobre lo que va
saliendo.

El resultado son **9868 bytes con siete entradas** y un `classes.dex` de 2392, que es el tamaño que el
dexer viene dando desde el §22. Verificación con los oráculos que no comparten código con nosotros:

```sh
go test -C apkc -count=1 -run "TestApkCompiladoSinAapt2|TestEmpaquetaRechazaNombresDeRuta" -v .
# deja el paquete si se le dice dónde escribirlo:
env APKC_ESCRIBE_APK=$PREFIX/tmp/flagprobe/go-built.apk \
  go test -C apkc -count=1 -run TestApkCompiladoSinAapt2 .
BT=$HOME/Android/build-tools/34.0.0; A=$PREFIX/tmp/flagprobe/go-built.apk
$BT/apksigner verify --verbose $A
$BT/zipalign -c -p 4 $A
$PREFIX/lib/jvm/java-21-openjdk/bin/jarsigner -verify $A
python3 ~/Develop/Go/apkc-harness/zipcheck.py $A
$BT/aapt2 dump resources $A
$BT/aapt2 dump xmltree --file AndroidManifest.xml $A
$BT/aapt2 dump xmltree --file res/layout/main.xml $A
$BT/aapt2 dump badging $A
```

**Lo que devolvieron**:

- `apksigner verify --verbose`: `Verifies`, **v1 true y v2 true** (v3 false, que es lo que firma este
  firmante), un firmante con el certificado `CN=Android Debug`.
- `zipalign -c -p 4`: rc 0, sin quejas. `zipcheck.py`: «7 entries, every record agrees with itself;
  0 stored .so at a page boundary, **1 other stored entries at 4**» — la entrada almacenada es
  `resources.arsc`, que es la que el sistema mapea.
- `jarsigner -verify`: `jar verified.`
- `aapt2 dump resources` lee nuestra tabla dentro del paquete (`0x7f010000 id/pulsar`,
  `0x7f020000 layout/main`, el estilo con sus cinco ítems) y `aapt2 dump xmltree` lee los dos AXML.
- `aapt2 dump badging` — que es el que dereferencia el manifiesto contra la tabla — responde
  `package: name='com.hola.apkcstudio' versionCode='1' versionName='0.1'`, `minSdkVersion:'21'`,
  `targetSdkVersion:'33'`, `application: label='Hola apkc'` y
  `launchable-activity: name='com.hola.apkcstudio.MainActivity'`. Si el `@style/AppTheme` del
  manifiesto no apuntara a un id que existe en nuestra tabla, aquí se quejaría.

Por dentro, la prueba también comprueba lo que escribe el zip: `resources.arsc` viaja
`zip.Store` y su dato empieza en múltiplo de cuatro, los dos AXML y la tabla se releen con nuestros
lectores, y el dex conserva la cabecera `dex\n035`.

**Comando de la suite**: nativa **154 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del hospedador de
navegador **150 PASS + 7 SKIP, rc 0** —el apk se salta el caso firmado porque el hospedador no tiene
`~/.android/debug.keystore`, igual que las otras cuatro pruebas que lo necesitan.

Lo que **todavía no** está en Go, dicho para que no se lea como hecho: este apk se ensambla desde los
**modelos** que escribieron las pruebas (los recursos del proyecto del studio ya traducidos a
estructuras). Falta el paso que lee el `res/values/*.xml`, el layout y el manifiesto **en texto** y
produce esos modelos, y la tabla de ids del framework —que hoy la da `android.jar`— para que un
layout cualquiera use atributos que no estén entre los doce del estudio. Con esos dos puentes,
`apkc build` deja de necesitar `aapt2`. Sigue quedando `javac` (Fase 1G) y la página.

## 33. La tabla de ids del framework, leída de `android.jar` con nuestro propio lector

El puente que faltaba para los atributos: `apkc/marco.go` anda el `resources.arsc` del SDK y saca el
mapa `nombre → id` del paquete `android`, que es lo que `aapt2` presta hoy al resolver
`android:layout_width` o `@style/TextAppearance.Large`.

Lo medido sobre el `android.jar` de este teléfono (API 34, `resources.arsc` de 16.390.624 bytes):

- La tabla declara **cuatro paquetes y los cuatro llevan id 1 y se llaman `android`**: el primero mide
  5.355.916 bytes, trae **23 tipos** y **12.877 claves**; los otros tres (4.656, 7.604 y 13.276)
  declaran 255, 223 y 221 tipos. **Los ids que imprime `aapt2 dump resources` son los del primero**;
  leer los siguientes pisaba el mapa con índices de otro espacio, y eso fue exactamente lo que dio la
  primera versión de este lector: 221 tipos y nombres como `?1`. Se corta en el primer paquete con
  id 1.
- El **pool global no se materializa**: 11.009.160 bytes con 152.615 cadenas y **1.307 estilos**
  (`styleCount` distinto de cero, algo que los pools de una app nunca traen). Se salta por su tamaño
  declarado; los dos pools de nombres del paquete sí se leen, y `leePool` devuelve ahora también el
  `styleCount` para poder rechazar en vez de suponer.
- Un tipo del framework **no es un bloque**: son muchos, uno por configuración, y solo uno cubre el
  espacio de entradas que anuncia su `RES_TABLE_TYPE_SPEC` (el de `string` tiene un bloque de 2492
  entradas y luego bloques de 1, 2, 3, 4, 9… con `flags 0x01`). Los índices de un bloque parcial **no
  son el id**; el lector toma el primer bloque cuya cuenta iguale a la del spec.
- Dentro de un bloque denso la entrada ausente se marca con **offset `0xFFFFFFFF`**, no con un bit de
  cabecera: medido en la entrada #616 del tipo `attr`, con `flags 0x00` en el bloque. El índice sigue
  siendo el lugar en el array, así que se salta la entrada y no la numeración.
- El `u16` del spec en `+10` —el que en una app vale el número de configuraciones— vale **0** en el
  framework, y las banderas de sus entradas `attr` valen `0x0003`.

Comprobación contra el oráculo que no comparte código:

```sh
J=$HOME/Android/platforms/android-34/android.jar
$HOME/Android/build-tools/34.0.0/aapt2 dump resources $J > marco.txt   # 240.637 lineas
grep -oE "resource 0x[0-9a-f]{8} (attr|color|style|dimen)/[^ ]+" marco.txt | awk '{print $2, $3}' | sort -u
go test -C apkc -count=1 -run TestMarco -v .
```

**Resultado**: los **28 ids** copiados del volcado de aapt2 coinciden con lo que lee `marco.go` —los
veintidós atributos que usan los ficheros del studio (`attr/theme 0x01010000` …
`attr/compileSdkVersionCodename 0x01010573`), `color/white 0x0106000b`, `color/black 0x0106000c`,
`style/TextAppearance.Large 0x01030042`, `style/Theme.DeviceDefault.Light 0x0103012b` y
`dimen/app_icon_size 0x01050000`—, el paquete da 23 tipos con `attr`, `id`, `style` como tres
primeros (**el orden del framework no es alfabético: es el de `public.xml`**), el tipo `attr` aporta
más de 1500 nombres y un nombre inventado se rechaza. Los rechazos probados: tabla sin paquete id 1
(es nuestra propia tabla de app), primer trozo que no es tabla, `packageCount` mentiroso y fichero
vacío.

Arreglado de paso un **panic real** que encontró el test nuevo: `leeTabla`, `leeAXML` y `leeMarco`
comprobaban el largo en la condición y **dentro del mensaje de error volvían a llamar a `u16re` sobre
el fichero corto**, así que un XML o una tabla de pocos bytes producía `index out of range` en lugar
de un rechazo. Los tres comprueban el tamaño antes de leer, y
`TestLectoresRechazanFicheroCorto` pasa longitudes de 0, 1, 7, 8, 11, 12 y 27 bytes por los cuatro
lectores. Importa porque en la página el fichero lo sube el usuario.

Suite: nativa **157 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del hospedador de navegador **152
PASS + 8 SKIP, rc 0** (el marco se salta allí, porque el hospedador no tiene `android.jar`).

## 34. Cómo define el framework un atributo: los `enum` y `flags` se guardan como recursos de tipo `id`

Para resolver `wrap_content` o `center` sin `aapt2` hace falta saber qué valores admite cada atributo, y
eso está dentro de la misma tabla del jar. Medido sobre `attr/layout_width` (entrada 0x0f4 del tipo
`attr`) y `attr/gravity` (entrada 0x0af):

- El valor de un `attr` es una **entrada compleja de 16 bytes** con `flags 0x0003`, `padre 0x00000000`
  y un recuento, seguida de exactamente `recuento` registros de doce bytes: la misma forma del §27 para
  los estilos.
- **El primer registro no es un valor**: su nombre es `0x01000000` y sus datos son la máscara de formato
  del atributo (`layout_width` lleva `0x00010040`). Se descarta por esa forma, no por posición.
- Los registros de `enum` y `flags` llevan el **nombre referido como recurso de tipo `id`**, no como
  cadena: `wrap_content` = `0x010205af`, `fill_parent` = `0x010202d7`, `center` = `0x01020240`,
  `center_horizontal` = `0x01020243`. `aapt2 dump resources` lo confirma:
  `0x01020240 id/center`, `0x010205af id/wrap_content`.
- Los datos del registro son el valor que hay que escribir, **con el tipo que el framework eligió**:
  `wrap_content` → `tipo 0x10` (entero decimal) `datos 0xfffffffe`; `center` → `tipo 0x11` (entero
  hexadecimal) `datos 0x11`. Es decir: el tipo del `Res_value` no se decide en el enlazador, se copia
  del atributo.

`marco.go` lo expone en dos pasadas, porque el tipo `attr` (id 1) se recorre **antes** que el tipo `id`
(id 2) y su mapa inverso aún está vacío: los registros se acumulan como `itemDeAtributo` y
`resuelveItems()` los traduce al terminar el paquete. La prueba compara cuatro valores concretos contra
el volcado de `aapt2 dump resources` (`layout_width/wrap_content` y `fill_parent`,
`gravity/center`, `center_horizontal` y `center_vertical`) y el `nombreDeId` de `0x0103012b`.

**Resultado**: tres pruebas de marco en verde. Nativa **158 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js`
dentro del hospedador de navegador **152 PASS + 9 SKIP, rc 0**.

## 35. Del `res/values/styles.xml` en texto a la misma tabla y a la misma `R`

Primera mitad del paso 3-G: `apkc/valores.go` lee un `res/values/*.xml` en texto con el decodificador
de Go y produce el modelo `tablaEscribir`. El fuente queda fijado en base64 (423 bytes,
`sha256 a490b9438728f1af…`) —el mismo `styles.xml` con el que aapt2 dejo los ficheros del §24— y la
prueba no muestrea: compara el modelo contra el que escribió a mano `arscwriter_test.go`.

Lo que se comprueba, en tres niveles:

- El modelo sale **igual campo a campo** (`reflect.DeepEqual` en tipos, claves, pool global, y por cada
  entrada su clave, sus banderas, su valor, su padre y sus cinco registros). Las claves salen en el
  orden medido —`pulsar`, `main`, `AppTheme`—, los tipos alfabéticos con id por posición, los registros
  del estilo ordenados por id de atributo (`0x01010054`, `0x01010056`, `0x010102cd`, `0x01010451`,
  `0x010104e0`) y el padre `@android:style/Theme.DeviceDefault.Light` resuelto contra la tabla del jar a
  `0x0103012b`, que es el número que está en los bytes de aapt2.
- Escrito el modelo, **los bytes siguen siendo los del fixture**: 936 bytes y los primeros 752 idénticos
  a los de aapt2 con los dos campos de tamaño sustituidos, la misma comprobación del §29.
- La `R` que sale del mismo modelo **es el texto de aapt2**, el de 493 bytes del §31.

Las formas de los valores no se inventan: `string` es `0x03` con el texto en el pool global, `color`
`0x1d` con el ARGB, `bool` `0x12` con −1 o 0, `integer` `0x10`, `item type="id"` un booleano a cero con
banderas `0x0004` en la entrada, y todo eso se vio en las variantes del §29. Un `<color>` cuyo texto no
es color, un `<bool>` que no dice `true`/`false`, un `<integer>` que no es número, un tipo de recurso no
medido (`dimen`, un `type=` raro), un item de estilo que no sea `android:`, un padre inexistente en el
framework, un `<string>` con un hijo, o una referencia al paquete propio intentando resolverse antes de
numerar la tabla: **los siete se rechazan con nombre y razón**, y eso es la segunda prueba del fichero.

```sh
go test -C apkc -count=1 -run "TestValores" -v .
```

**Resultado**: dos pruebas en verde. Nativa **160 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del
hospedador de navegador **154 PASS + 9 SKIP, rc 0** —esta parte corre allí también porque el marco que
necesita el estilo está reconstruido a mano en la prueba con los ids y valores que §33 y §34 copiaran de
`aapt2 dump resources`, y el hospedador no tiene `android.jar`.

**Lo que queda de 3-G**: la otra mitad, que es la que exige las dos pasadas —leer layouts y manifiesto a
árbol, descubrir los `@+id/…` (que añaden entradas a la tabla), numerar, y entonces escribir los AXML con
las referencias resueltas y los cuatro atributos que el manifiesto gana. Las restricciones están escritas
en `docs/plan.md`.

## 36. El árbol en texto: las líneas que anota aapt2 no son las que da el decodificador de Go

`apkc/arbol.go` convierte el layout y el manifiesto en texto en un árbol con nombres, **líneas**,
prefijos y orden de hijos. Es la pieza que faltaba para la segunda mitad de 3-G, y trae una medición
sobre la propia librería de Go, que es la que decide su forma:

- Sondeado con `encoding/xml` (el programa está en `$PREFIX/tmp/probeXML/sondeo.go`, 24 líneas): en una
  declaración `xmlns:android="…"` el decodificador responde `Space="xmlns"` y `Local="android"`, o sea
  **el prefijo**; en un atributo `android:orientation` responde `Space="http://schemas.android.com/apk/res/android"`,
  o sea **el URI ya resuelto**. El binario quiere el prefijo en el nodo y el URI en el campo `ns`, así
  que se guarda el mapa en las dos direcciones.
- Y la que no se veía venir: **`InputPos()` cae detrás del `>` de la etiqueta de apertura**. En el
  manifiesto del studio `<activity` abre en la línea 13, sus cuatro atributos ocupan hasta la 17, y el
  decodificador informa 17. `aapt2` anota **13** (medido sobre el binario con el instrumento del §24, y
  vuelto a medir en el §30). La línea se busca entonces en el texto con un cursor que solo avanza,
  buscando el `<nombre` siguiente. La prueba `TestArbolDelManifiesto` comprueba las siete líneas del
  binario: manifiesto 1, uses-sdk 6, application 10, **activity 13**, intent-filter 19, action 20,
  category 21.
- El orden de los atributos en el árbol es el del **fuente**, no el del binario: en el `TextView` el
  `style` sin prefijo es el primero leído y aapt2 lo escribe el último. Eso es materia del escritor de
  AXML (que ya ordena como aapt2, §30), y se comprueba aquí para que el lector no lo pierda.

Rechazos probados sobre el propio lector: documento sin raíz, dos raíces, cierre mal anidado, elemento
sin cerrar, texto suelto dentro de un elemento, y un comentario entre dos raíces. La forma buena
(declaración `<?xml …?>`, `xmlns:android` y un hijo) se acepta como control de que los cinco rechazos no
son un error que rechace todo.

```sh
go test -C apkc -count=1 -run "TestArbolDel|TestArbolRechaza" -v .
```

**Resultado**: tres pruebas en verde. Nativa **163 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del
hospedador de navegador **157 PASS + 9 SKIP, rc 0**.

## 37. Enlazado: de los tres ficheros de texto a los tres binarios de aapt2, byte a byte

`apkc/enlazar.go` junta las dos mitades del paso 3-G —`arbol.go` (layout y manifiesto en texto) y
`valores.go` (los `res/values/*.xml`)—, descubre los `@+id/…` que el layout declara, numera la tabla con
ellos y entonces materializa los modelos. Con eso el proyecto del studio se enlaza **sin `aapt2`** y lo
que sale se compara contra los binarios fijados:

| salida | contra lo que escribió aapt2 | resultado |
| --- | --- | --- |
| `res/layout/main.xml` (AXML) | los 784 bytes del fixture | **idéntico byte a byte** |
| `AndroidManifest.xml` (AXML) | los 2000 bytes del fixture | **idéntico byte a byte** |
| `resources.arsc` | los 1088 bytes del fixture | 936 escritos; los **primeros 752 idénticos** salvo los dos campos de tamaño, y la diferencia es la declarada en el §28 (una configuración en vez de dos) |
| `R.java` | los 493 bytes de `link --java` | **idéntico byte a byte** |

Ninguna de las cuatro comparaciones es contra un modelo propio: se compara contra **los ficheros que dejó
la herramienta que este fork quiere sustituir**, fijados en base64.

Dos fallos que encontraron estas pruebas y que valen como medición:

- **`match_parent` no estaba en el marco de la prueba** y caía al último caso de `valorDeAtributo`, la
  cadena: el layout salía con 15 cadenas en vez de 14, 20 bytes de más y **472 bytes distintos**. El §34
  ya había medido que `layout_width` trae cuatro registros y que dos de ellos valen `0xffffffff`
  (`fill_parent` en `0x010202d7` y otro en `0x010203a0`, que es `match_parent`); puesto el item, el layout
  sale exacto. `TestMarcoValoresDeAtributo` exige ahora `match_parent` contra el jar, para que el marco
  escrito a mano no pueda volver a quedarse corto.
- **`platformBuildVersionName` vale 14, no 34.** El primer intento lo copiaba de `compileSdkVersion` y
  daba un solo byte distinto en todo el manifiesto (el `#1368`: `0x22` contra `0x0e`). Los cuatro campos
  que añade el enlace salen de dos números distintos: `compileSdkVersion` y `platformBuildVersionCode` son
  la API (34) y `compileSdkVersionCodename` y `platformBuildVersionName` son la **versión de Android**
  ("14" como texto y 14 como entero, con crudo −1). El enlace deriva el entero del codename y rechaza un
  codename no numérico, en vez de inventarse la forma.

Rechazos probados en el enlazador: manifiesto sin `package`, un atributo `android:` que no esté en la
tabla del framework, una referencia `@style/NoExiste` del paquete propio, una referencia
`@android:style/…` inexistente y una referencia mal formada.

```sh
go test -C apkc -count=1 -run "TestEnlazar|TestValores|TestArbol|TestMarco" -v .
sh -c 'cd studio && APKC=$PREFIX/tmp/apkc-gate APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm sh e2e.sh'
```

**Resultado**: once pruebas en verde en ese conjunto. Nativa **166 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js`
dentro del hospedador de navegador **160 PASS + 9 SKIP, rc 0** —el enlace corre allí también, con el marco
reconstruido a mano sobre los números que §33 y §34 copiaran del jar—. Y el portero del repo, `sh e2e.sh`
con el binario de este fork, sigue diciendo **`e2e: todo en orden`**, con sus 16 comprobaciones de la
pestaña conducida por chromium.

Con esto **la Fase 3 queda cerrada de punta a punta**: texto del proyecto → tabla, dos AXML y `R`, y de ahí
el apk firmado del §32. Lo que queda del objetivo declarado (compilar en la pestaña sin conexión) son dos
piezas grandes: la Fase 1G, que es escribir el `javac` en Go, y el cableado de la página.

## 38. `apkc build --linker go`: un apk real sin ejecutar aapt2, y sus productos comparados

`apkc/linker.go` engancha el enlace de Go en la pipeline de siempre. `--linker` toma `aapt2` (el
default, que es lo que apkc hizo toda su vida) o `go`; con `go`, `compileRes` no ejecuta nada —el `.flat`
solo alimentaba a `aapt2 link`, y eso ya se dijo en el §24— y `bundleRes` escribe los cuatro productos que
el plan del paquete (`packagePlan`) ya sabe tomar: `build/AndroidManifest.xml`, `build/resources.arsc`
(almacenado, porque el sistema lo mapea), `build/res/layout/*.xml` y `src/<paquete>/R.java`. El empaquetado
y la firma no se tocan.

Corrido sobre el proyecto del studio en una copia limpia:

```sh
apkc build --linker go
# [build] linking resources with the Go linker
# [timing] aapt2 link 226ms  <- el enlace, con el nombre del tramo de siempre
# [build] compiling java files / javac 3.77s
# [build] bundling classes / d8 app 4.813s
# [build] packaging and signing inside apkc
```

Contra los productos del proyecto enlazado con aapt2:

| producto | aapt2 | go | resultado |
| --- | --- | --- | --- |
| `build/AndroidManifest.xml` | 2000 | 2000 | **idéntico byte a byte** |
| `build/res/layout/main.xml` | 784 | 784 | **idéntico byte a byte** |
| `src/…/R.java` | 493 | 493 | **idéntico byte a byte** |
| `build/classes.dex` | 2392 | 2392 | **idéntico** (lo produce el dexer del §22, no cambió) |
| `build/resources.arsc` | 1088 | 936 | distinto **por lo declarado en el §28**: una configuración base con los cinco ítems en vez de base + `-v23` |

Y el apk resultante (10194 bytes) pasa los oráculos de siempre:

```sh
apksigner verify --verbose build/app.apk   # Verifies, v1 true, v2 true
zipalign -c -p 4 build/app.apk             # rc 0
python3 ~/Develop/Go/apkc-harness/zipcheck.py build/app.apk
# 7 entries, every record agrees with itself; 1 other stored entries at 4
aapt2 dump badging build/app.apk           # package, versiones, label, launchable-activity
```

**El portero, y un agujero que tapó el test.** `fuentesDelProyecto` rechaza con nombre lo que este enlace
no sabe escribir: `res/drawable` y compañía (piden copiar ficheros y poner entradas con referencia de
fichero), **cualquier calificador** —`res/layout-es`, `drawable-v21`, `values-land`— porque es una segunda
configuración y aquí se emite una sola (§28), subcarpetas dentro de `values` o `layout`, recursos que no
sean `.xml`, un manifiesto con `<permission>` (aapt2 escribe además un `Manifest.java` que aquí no se
produce), un proyecto sin manifiesto, y una `android.jar` cuya versión no esté medida (`opcionesDelProyecto`
saca 34 y "14" de la ruta del platform; API 21 se rechaza en vez de adivinar el codename). El agujero era
precisamente el calificador: la primera versión partía el nombre por el guion y **dejaba pasar
`res/layout-es`**, que habría producido un apk al que le faltaran los recursos de ese idioma sin decir
nada. `TestFuentesDelProyectoRechazaLoQueNoSabeEnlazar` exige ahora los seis rechazos y que cada mensaje
nombre el motivo.

**Resultado**: suite nativa **169 PASS / 0 FAIL / 1 SKIP**; en `GOOS=js` dentro del hospedador de
navegador **163 PASS + 9 SKIP, rc 0**. `javac` y `d8` siguen ejecutándose en este camino: lo que ya no se
ejecuta es `aapt2`.

## 39. `apkc link` dentro del hospedador con forma de navegador: los recursos, compilados del lado del cliente

Tercer comando del fork que no levanta proceso (después de `dex` y de `sign`): `apkc link` hace **solo el
enlace de recursos**, con `--android-jar` apuntando al jar del SDK y `-o` al directorio del proyecto. Se
midió corriéndolo en `shim/navegador-sim.js` —el hospedador con forma de navegador, un `fs` en memoria,
sin red y sin proceso—, con el `android.jar` montado como un fichero más:

```sh
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o $PREFIX/tmp/apkc-link.wasm .'
sh -c 'cd shim && node navegador-sim.js \
  --montar $P/AndroidManifest.xml:/proyecto/AndroidManifest.xml \
  --montar $P/res/values/styles.xml:/proyecto/res/values/styles.xml \
  --montar $P/res/layout/main.xml:/proyecto/res/layout/main.xml \
  --montar $HOME/Android/platforms/android-34/android.jar:/sdk/android.jar \
  --exportar /proyecto/build/AndroidManifest.xml:$O/AndroidManifest.xml \
  --exportar /proyecto/build/resources.arsc:$O/resources.arsc \
  --exportar /proyecto/build/res/layout/main.xml:$O/main.xml \
  --exportar /proyecto/src/com/hola/apkcstudio/R.java:$O/R.java \
  $PREFIX/tmp/apkc-link.wasm link --android-jar /sdk/android.jar --sdk-version 34 -o /proyecto'
```

**Resultado**: `3,22 s` (leer el jar de 26 MB y andar su tabla de 16,39 MB, todo en el hospedador) y los
cuatro productos exportados: **`AndroidManifest.xml` idéntico byte a byte al de aapt2, `main.xml` idéntico,
`R.java` idéntica**, y `resources.arsc` de 936 bytes frente a los 1088 de aapt2 por la única configuración
base declarada en el §28.

Tres cosas que esta corrida obligó a arreglar, y que en el teléfono no habrían aparecido:

- **`main()` sondaba el SDK antes de despachar cualquier orden** (`findSDKs`), y en el hospedador eso
  terminaba el comando con `[doctor] open /android/build-tools: No such file or directory` antes de
  empezar. `link` y `dex`, que son los dos que no levantan proceso, quedan fuera de ese sondeo:
  `default: findSDKs()`.
- **La versión del platform se leía de la ruta del jar** (`platforms/android-34/android.jar`). En la
  pestaña el usuario monta el jar donde quiere y la ruta no informa nada, así que `--sdk-version` y
  `--codename` existen; la ruta sigue siendo el defecto cómodo para `apkc build`. Un `--codename` sin
  `--sdk-version`, o una versión cuyo nombre no esté medido (API 21, por ejemplo), se rechazan con nombre.
- **El orden de argumentos del hospedador**: `navegador-sim.js <wasm> <orden> …`, la orden primero. Llamado
  con la ruta delante, `apkc` interpretaba `/proyecto` como el nombre del comando y no pasaba de ahí.

El control de que los rechazos del nuevo portero no son un error que rechace todo sigue en
`linker_test.go`, y la suite nativa y la del hospedador (las 163 + 9 del §38) no cambiaron con el arreglo.

**Y el portero del studio, con una nota sobre su intermitencia.** `sh e2e.sh` con el binario nuevo dijo
**`e2e: todo en orden`** (16 comprobaciones de la pestaña, apk de 10348 bytes firmado dentro de chromium y
verificado por fuera). En una corrida anterior, con el mismo binario, habia fallado tres: la que
sobrevivía al filtro imprimía

```
--- nodos tras recargar: ["/proyecto:dir","/proyecto/AndroidManifest.xml:0b","/proyecto/res/layout:dir","/proyecto/src:dir"]
```

o sea que al recargar la pestaña el árbol persistido en OPFS estaba **ahi pero con ficheros de cero
bytes**. Eso cambia el diagnóstico que estaba escrito (`create` falla a veces): el fallo no esta en lo que
genera el proyecto, esta en el **persistido** —se consulta el árbol antes de que el contenido este
escrito—, y por eso es intermitente y por eso la comprobacion falla al recargar y no al crear. Queda
anotado con su salida, no como sospecha; reproduce una de cada varias corridas y pasa en la siguiente.

