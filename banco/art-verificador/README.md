# art-verificador — la estructura del dex, ejecutada por ART

El defecto de `valorEstatico` (`VALUE_INT` en campos `Z`/`C`) lo destapó **ART**, no `dexdump`:
`dexdump` desarmaba el fichero con `rc 0` mientras el aparato rechazaba el dex entero
(«unexpected static field initial value type: 'I' vs 'Z'»). Este banco busca los demás defectos
estructurales del mismo tipo: **el oráculo que manda es ART ejecutando**, y `dexdump` solo compara
la estructura.

## Comando

```sh
sh banco/art-verificador/art-verificador.sh

# un subconjunto
CASOS="zchar anota" sh banco/art-verificador/art-verificador.sh

# la falsificación: corta el dex del primer caso y exige que la batería diga FALLO
FALSIFICAR=1 sh banco/art-verificador/art-verificador.sh
```

Variables: `SERIE` (por defecto **`10.177.0.2:5555`**, Droidloom; el banco no elige otro serial),
`APKC` (si falta, se compila desde `apkc/` con `tcr` cuando lo hay), `ANDROID_HOME`
(`~/opt/android-sdk`), `APKC_ANDROID_JAR` (`platforms/android-34/android.jar`), `BT` (la
build-tools más alta), `ADB`, `KEYSTORE` y `APKC_WEB_CACHE`. Los artefactos —proyecto, keystore,
APKs, `classes.dex`, volcados y logcat— van a `$HOME/.cache/apkc-web/art-verificador`: **no se
versiona ningún APK ni binario**.

Necesita `adb` con el aparato conectado, `javac` en el `PATH`/`JAVA_HOME`, y `python3`. No toca
red, ni gate, ni chromium, ni instala nada fuera del aparato.

## Qué hace por caso

1. `apkc build --linker go --compiler javac` deja el APK del camino de **`d8`** (javac + d8, el
   oráculo de referencia).
2. `apkc dex` sobre los **mismos** `.class` deja el `classes.dex` del camino propio; `reemplaza-dex.py`
   lo mete en una copia del APK, `zipalign` lo alinea y `apksigner` lo firma otra vez. Así los dos
   APKs solo se diferencian en el dex.
3. `dexdump -a` de los dos ficheros pasa por `compara-estructura.py`: clases, superclase,
   interfaces, cada campo con su tipo, sus accesos y su `static_values`, cada método con su
   descriptor y sus accesos, y cada bloque de anotaciones. Los estáticos **sin** `ConstantValue`
   quedan fuera del valor: `d8` propaga el `<clinit>` y este dexer escribe el cero del tipo, que el
   `<clinit>` sobrescribe al correr (está medido en `apkc/dexarma.go`).
4. Cada APK se instala (`--no-incremental`), se arranca con `am start -W` y se miran las líneas de
   **su** proceso (`logcat --pid`): ni `Verification rejected`, ni `Failure to verify`, ni
   `CLASS_ISPREVERIFIED`, ni `dex2oat`, ni `VerifyError`.
5. El `Log.d` de la actividad tiene que traer el mismo texto por los dos caminos y el de
   `fixtures/<caso>/esperado.txt`. Con la falsificación, un dex cortado a la mitad tiene que hacer
   decir FALLO a la batería; después se restaura y se exige verde otra vez.

## Los ocho casos

| Caso | Qué estructura mide |
| --- | --- |
| `zchar` | el caso ya cerrado de `Z`/`C`: `static final boolean`/`char` y estáticos sin `ConstantValue` |
| `estaticos` | `static final` de los ocho primitivos y `String`, con los bordes de cada ancho (`-128`, `0xffff`, `MIN_VALUE`, `NaN`, `-0.0`) |
| `tries` | `try`/`catch`/`finally`, multi-catch, el valor pendiente del `return` y el `finally` anidado |
| `anota` | anotaciones del usuario con elementos `enum`, entero y `String`, en clase/campo/método, y la `Signature` genérica; se leen por reflexión |
| `arreglos` | un arreglo de cada primitivo, `String[]`, dos dimensiones, `array-length` y `aget`/`aput` |
| `disper` | `switch` con casos dispersos (`sparse-switch`), contiguos (`packed-switch`), `char` fuera de ASCII y clave de cadena |
| `cadenas` | constante de 6000 unidades, NUL (`C0 80` de MUTF-8), acentos y un emoji (par de subrogados) |
| `ifaces` | `invokeinterface` con arreglo de la interfaz, dos implementaciones y una clase abstracta |

## Defectos que este frente cerró

Cada uno se midió primero en Droidloom y se arregló con su prueba:

- **anotaciones del usuario**: no se escribían (solo las seis `Ldalvik/annotation/*`). Ahora se
  leen `RuntimeVisible/InvisibleAnnotations` y `AnnotationDefault`, y se emiten con su visibilidad.
  Formas medidas contra el fichero de `d8`: `VALUE_ENUM` es **un índice de `field_ids`** con su
  anchura en la cabecera (no un par tipo/nombre), el `type_index` de una anotación y el de un enum
  son `Utf8`, el `s` de un `element_value` apunta a `Utf8` (no a `CONSTANT_String`), la cadena
  vacía se interna, y el `AnnotationDefault` es una anotación de sistema en la clase con la
  anotación del propio tipo dentro (`apkc/dexanota.go`, `apkc/classreader.go`).
- **`VerifyError` en `try`**: un local se fusionaba con el temporal de una rama y el registro de
  casa cambiaba para todo lo que venía detrás; en el `catch` se leía un registro indefinido
  (`copy-reference v7<-v57 type=Undefined`). Ahora no se fusiona dentro de un tramo protegido y el
  descenso de un hueco para escribir no copia el valor viejo (`apkc/dexinstr.go`).
- **`wide iinc`** (0xc4): `iinc 1, 1000` de javac no estaba en la tabla del caminante. Se traduce
  a `add-int/lit16` (`d000 e803`), que es lo que emite `d8` (`apkc/jvmopcode.go`,
  `apkc/dexinstr.go`).
- **`new-array` con el tamaño por encima de v15**: el 22c lo reparte en cuatro bits; ahora el
  tamaño se baja a un temporal con `move/from16` en vez de rechazar el método.
- **MUTF-8 del `.class`**: `CONSTANT_Utf8` también es MUTF-8; guardado crudo, el NUL y los
  subrogados se reescribían como U+FFFD (14 unidades y un 65533 donde hay 13 y un 0). Se decodifica
  al leer (`apkc/classfile.go`).
- **sitios `invokedynamic` por pc**: los mapas de concatenación y de lambdas se indexaban solo por
  pc, así que dos métodos con un sitio en el mismo pc compartían entrada; la concatenación de un
  `long` salía como `append:(I)` y ART cortaba con «Long (Low Half) but expected Integer». Ahora la
  clave es (método, pc) (`apkc/lambda.go`).

## Fronteras (lo que este banco no cubre)

- **Los fixtures no usan el operador ternario ni imprimen un `boolean` de una comparación.** El
  traductor mantiene un solo estado por local y por pila en un recorrido lineal: dos ramas que
  dejan el valor en registros distintos no se funden con un `move` en el punto de unión. Medido con
  una versión anterior de `arreglos` y `disper` (`? :` dentro del bucle): ART rechazaba con
  «register vN has type Undefined» o con una mitad de `double` donde esperaba un entero. Arreglarlo
  pide un reparto con puntos de unión de verdad; queda abierto.
- **La concatenación de una expresión con prefijo vacío** (`"" + x`) revienta en el desugrado
  (`the JVM stack is empty: there is nothing to drop`); los fixtures usan `String.valueOf` o un
  prefijo con texto.
- No se comparan instrucciones ni registros: el asignador es otro y esa diferencia está aceptada
  (igual que en `apkc/dexinstr_test.go`).
- El `dexdump` de `compara-estructura.py` solo mira lo que el desensamblador imprime; lo que ART
  comprueba y no se imprime (por ejemplo el tipo de un `encoded_value` de campo estático) lo cubre
  `verificaLayout` en la suite de `apkc/` y, sobre todo, la ejecución.
