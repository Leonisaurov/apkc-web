# Evidencia — §40-§54

## 40. El `.class` que hay que escribir: la plantilla de `apkc create`, medida con `javap -v -p`

Objetivo de la Fase 1G, medido antes de escribir el emisor. El fuente es la plantilla tal cual la genera
`apkc create` (`MainActivity.java`, 17 líneas) y la `R.java` que escribe **nuestro** enlazador de la
Fase 3, con `com.hola.j1g` como paquete:

```sh
$PREFIX/tmp/apkc-gate create $PREFIX/tmp/j1g/proyecto           # plantilla, con las respuestas al prompt
cd $PREFIX/tmp/j1g/proyecto && $PREFIX/tmp/apkc-gate link \
  --android-jar $HOME/Android/platforms/android-34/android.jar --sdk-version 34 --codename 14
$PREFIX/lib/jvm/java-21-openjdk/bin/javac -nowarn -d $PREFIX/tmp/j1g/classes \
  -classpath $HOME/Android/platforms/android-34/android.jar \
  src/com/hola/j1g/R.java src/com/hola/j1g/MainActivity.java     # rc 0
$PREFIX/lib/jvm/java-21-openjdk/bin/javap -J-Xmx256m -v -p -classpath $PREFIX/tmp/j1g/classes \
  com.hola.j1g.MainActivity com.hola.j1g.R 'com.hola.j1g.R$layout'
```

`javac` del JDK 21 del teléfono deja **cuatro** clases: `MainActivity.class` de 445 bytes, `R.class` de
324, `R$layout.class` de 323 y `R$style.class` de 325. Todas con `major version: 65` y
`flags: (0x0021) ACC_PUBLIC, ACC_SUPER` (la `R` añade `ACC_FINAL`, o sea `0x0031`).

El `onCreate` completo, que es el cuerpo que hay que emitir:

```
  public void onCreate(android.os.Bundle);
    descriptor: (Landroid/os/Bundle;)V
    flags: (0x0001) ACC_PUBLIC
    Code:
      stack=2, locals=2, args_size=2
         0: aload_0
         1: aload_1
         2: invokespecial #7                  // Method android/app/Activity.onCreate:(Landroid/os/Bundle;)V
         5: aload_0
         6: ldc           #13                 // int 2130771968
         8: invokevirtual #14                 // Method setContentView:(I)V
        11: return
      LineNumberTable:
        line 14: 0
        line 15: 5
        line 16: 11
```

y el `<init>` que **javac sintetiza** (en el fuente no hay constructor escrito), con su tabla de líneas
apuntando a la línea de la declaración de clase:

```
  public com.hola.j1g.MainActivity();
    Code:
      stack=1, locals=1, args_size=1
         0: aload_0
         1: invokespecial #1                  // Method android/app/Activity."<init>":()V
         4: return
      LineNumberTable:
        line 11: 0
```

Cuatro cosas se leyeron aquí y no en un manual:

- **`setContentView` se resuelve contra la clase actual.** `#14 = Methodref #15.#16` y `#15 = Class
  com/hola/j1g/MainActivity`, aunque el método se declara en `android.app.Activity` y `javap` lo imprime
  sin prefijo de clase. Del camino de superclases se toma la **descripción** (`(I)V`), no el dueño de la
  referencia. `super.onCreate` es lo contrario: `#7 = Methodref #2.#8` con `#2 = Class
  android/app/Activity`, porque ahí el receptor está escrito.
- **`R.layout.main` no es un `getstatic`.** Sale `#13 = Integer 2130771968` (`0x7f010000`) y un `ldc` de
  un byte de índice: javac dobla en el llamador la constante de compilación. En la `R$layout` ese mismo
  valor viaja como atributo del campo —`ConstantValue: int 2130771968`, `flags: (0x0019) ACC_PUBLIC,
  ACC_STATIC, ACC_FINAL`— y **no** deja método de inicialización estática: la clase tiene un solo método,
  el `<init>` sintetizado contra `java/lang/Object`.
- **`@Override` no deja huella.** `attributes: 2` en `MainActivity` (SourceFile e InnerClasses) y ningún
  atributo de anotaciones en los métodos: la anotación se puede descartar sin más. La `R` tiene tres
  atributos porque además lleva `NestMembers` (`com/hola/j1g/R$style`, `com/hola/j1g/R$layout`), y cada
  anidada tiene `NestHost`; eso es de Java 11 y **no viaja en la versión 52** que emite este fork.
- **Las líneas de la `LineNumberTable` son las del fuente**, contadas como las cuenta javac: la 11 es
  `public class MainActivity extends Activity {`, la 14 `super.onCreate(...)`, la 15 `setContentView(...)`
  y la 16 el `}` que cierra el método. El pool de `MainActivity` trae `#11 = Class com/hola/j1g/R$layout`
  y `#27 = Utf8 layout` solo por el `InnerClasses`, no por el código.

Con esto el emisor escribe la **versión 52** (Java 8): los 65 de javac 21 no aportan nada que el dexer o
`d8` necesiten, y 52 es la que este fork ya lee y dexa. El oráculo de la fase es el `javap -c -p` textual
de arriba, más abajo, y el dex que sale de ahí.

## 41. El `.class` escrito en Go: `javap` lo lee igual que el de javac, y el dex sale byte a byte idéntico

`apkc/javaemisor.go` emite las cuatro clases del parrafo 40. Dos oráculos; el segundo es el que cierra la
prueba.

```sh
# vuelco del emisor (prueba auxiliar, borrada despues de medir) y javap sobre lo que escribe
sh -c 'cd apkc && go test -count=1 -timeout 120s -run TestVuelcaParaJavap .'
J=$PREFIX/lib/jvm/java-21-openjdk/bin/javap
for k in MainActivity Rlayout R; do
  $J -J-Xmx256m -c -p -classpath $PREFIX/tmp/j1g/ours    "com.hola.j1g.$k" > a.txt
  $J -J-Xmx256m -c -p -classpath $PREFIX/tmp/j1g/classes "com.hola.j1g.$k" > b.txt
  diff a.txt b.txt
done   # Rlayout es R$layout, con el nombre entre comillas
```

**Primer oráculo**: `javap -c -p` sobre nuestras clases imprime **las mismas líneas de desensamblaje** que
sobre las de javac en las cuatro clases. Las únicas diferencias del `diff` son espacios de alineación de la
columna de comentarios, porque el índice de pool que javac escribe en dos dígitos aqui es de uno.

**Segundo oráculo**, dentro de la suite (`TestEmisorDexaLoQueDexaJavac`): las cuatro clases de javac viajan
como base64 en el test, y el test dexa las nuestras y las de javac con `dexDeClases`. **El dex sale idéntico
byte a byte** (1284 = 1284, cero bytes distintos), incluidas la firma SHA-1 y la `checksum` del encabezado,
que son lo primero que se rompe cuando el contenido difiere. No queda nada que comprobar del significado de
nuestros `.class`: el mismo lector, el mismo dexer y el mismo resultado que produce `javac` en el teléfono.

Tamaños: los nuestros son 467, 298, 304 y 306 bytes frente a los 445, 324, 323 y 325 de javac. La diferencia
es la versión 52 contra 65 (el `NestMembers`/`NestHost` de 8 bytes por clase y las entradas de pool que
javac interna para escribirlos) y nada más: el `Code` es el mismo. *Estas cuatro cifras y el 1284 se midieron
otra vez con `apkc compile` después del §42, que añadió las reglas de mención del `InnerClasses`; el oráculo
del párrafo anterior es el test, que sigue pasando.*

Tres defectos que solo aparecieron comparando con el de fuera, no con un manual:

- **El índice del pool estaba desplazado un puesto.** `agregar` devolvía `len(entradas) + 1` después de
  añadir, o sea que la primera entrada se llamaba 2 a sí misma: cada `CONSTANT_Class` apuntaba a su propio
  índice en vez de a su `Utf8`. Nuestro lector lo pasaba; **javap lo paró con un mensaje exacto**:
  `unexpected entry at #3 -- expected tag 7, found 1`. Con el índice correcto el conteo sigue siendo
  `última posición + 1`.
- **El `<init>` sintetizado va escrito primero.** javac pone el constructor antes que los demás métodos
  (medido: `methods: 2`, el primero `<init>`); emitirlo al revés cambiaba el orden de los `class_data` y
  saltaba el test de igualdad.
- **Doblar una constante no borra la referencia al tipo.** `R.layout.main` sale `ldc int` sin `getstatic`, y
  por eso el emisor no escribía ninguna `Class` de `R$layout`: el `InnerClasses` de MainActivity salía
  vacío. javac sí deja `#11 = Class com/hola/j1g/R$layout` en el pool **solo por el atributo** (medido en el
  §40), así que la mención se registra al resolver la cadena de nombres, aunque el código no la use.

Y dos reglas de formato que conviene tener escritas porque el código las hace a mano: la mágica es
`CA FE BA BE` (no `CAFED00D`, que es como la nombra el estándar), y `ldc` lleva el índice en **un byte** —
a partir de 256 entradas hay que pasar a `ldc_w`, y mientras eso no esté medido el emisor lo nombra
(`el pool de ... pasa de 255 entradas y ldc_w no esta medido`) en vez de truncar el operando.

La suite nativa pasa con **179 tests** (tres nuevos: la plantilla contra `javap`, el dex contra javac y el
portero de rechazos). El portero rechaza con nombre lo que este emisor no escribe: clase anónima (falta el
`this$0`), `if`, `long`/`double`/`float`, una anotación que no sea `@Override` y un tipo que no aparezca en
el fuente, en su paquete, en los imports ni en el `android.jar`.

## 42. La clase anónima: `this$0`, `EnclosingMethod`, `invokeinterface` y el dex idéntico de las seis clases

Lo que pedía la muestra del `studio/` (`menu.add("…").setOnMenuItemClickListener(new MenuItem.OnMenuItem-
ClickListener() { … })` con `Toast.makeText(MainActivity.this, "Clicked!", Toast.LENGTH_SHORT).show()`) se
midió antes de escribirlo, sobre las seis clases que `javac` dejó en `build/classes` del proyecto generado
por la página:

```sh
$J=$(echo $PREFIX)/lib/jvm/java-21-openjdk/bin/javap
S=$(echo $PREFIX)/tmp/studio-e2e-java/com.hola.apkcstudio
$J -J-Xmx256m -v -p -classpath $S/build/classes 'com.hola.apkcstudio.MainActivity$1'
$J -J-Xmx256m -c -p -classpath $S/build/classes com.hola.apkcstudio.MainActivity
```

**La anonima, `javap -v -p`**: `flags: (0x0020) ACC_SUPER` (ni pública ni final), `super_class`
`java/lang/Object` **aunque el fuente dice `new MenuItem.OnMenuItemClickListener()`** — una anonima sobre una
interface extiende Object e *implementa* la interface: `interfaces: 1, fields: 1, methods: 2, attributes: 4`.
El campo es `final com.hola.apkcstudio.MainActivity this$0;` con `descriptor: Lcom/hola/apkcstudio/MainActivity;`
y `flags: (0x1010) ACC_FINAL, ACC_SYNTHETIC`. El constructor es `MainActivity$1(MainActivity)` con
`flags: (0x0000)` y:

```
   0: aload_0
   1: aload_1
   2: putfield      #1   // Field this$0:Lcom/hola/apkcstudio/MainActivity;
   5: aload_0
   6: invokespecial #7   // Method java/lang/Object."<init>":()V
   9: return
  LineNumberTable:
    line 20: 0
  MethodParameters:
    Name        Flags
    <no name>   final mandated
```

o sea: **primero se guarda la captura y después se llama al padre**, la tabla de líneas apunta a la línea del
`new` (la 20), y el parámetro sintético lleva `MethodParameters` con `ACC_FINAL|ACC_MANDATED` (0x8010) y sin
nombre. El método implementado, `onMenuItemClick(MenuItem)`, sale `stack=3, locals=2` con
`aload_0; getfield this$0; ldc "Clicked!"; iconst_0; invokestatic Toast.makeText; invokevirtual Toast.show;
iconst_1; ireturn` en las líneas 22 y 23 — `MainActivity.this` es un `getfield`, `Toast.LENGTH_SHORT` es un
`iconst_0` (constante doblada del jar) y `show()` es `invokevirtual` porque el receptor es una clase.

**El `InnerClasses` tiene tres filas en el exterior y dos en la anonima**, y ahí estaba la parte que un
manual no dice. En `MainActivity`: `layout=R$layout of R`, despues `#28;` (la anonima, **sin exterior y sin
nombre**) y despues `OnMenuItemClickListener=class android/view/MenuItem$OnMenuItemClickListener of
android/view/MenuItem` — en **orden de primer uso**, no alfabetico ni de declaración. Y esa tercera fila
existe aunque la interface no aparezca en ningún opcode: solo está escrita dentro del descriptor de
`setOnMenuItemClickListener`. En la R, las tres anidadas van listas `style, layout, id`, o sea **al revés de
como se declaran**. Los flags de una anidada del framework salen `0x0609` en la fila (`public static`)
mientras su propio ClassFile dice `0x0601`: javac añade `ACC_STATIC` a la fila.

**Resultado con el emisor encima**: `javap -c -p` sobre las seis clases nuestras imprime lo mismo que sobre
las seis de javac, con la única diferencia del **número de pool** (`invokespecial #10` frente a `#1`), que no
es un hecho semántico. Y el oráculo duro, dentro de la suite (`TestEmisorMuestraDexaLoQueDexaJavac`): el dex
de nuestras seis clases es **idéntico byte a byte** al dex de las seis de javac, 2392 = 2392, cero
diferencias. La suite nativa pasa con **182 tests**.

Tres defectos que solo salieron comparando, y cada uno con su síntoma:

- **El descriptor de un constructor sin vuelta.** `new` armaba `(Lcom/hola/apkcstudio/MainActivity;)` y
  olvidaba la `V`. El lector no se quejaba; el dex salía con un `method_id`
  `MainActivity$1.<init>(…)I` y una cadena `L` de más, y el fichero crecía 32 bytes. El formato pide
  `(...)V` siempre.
- **`fields_count` sin contar el campo de captura.** Se escribía el campo pero el conteo salía del AST, que
  no lo tiene. `javap`: `unexpected end of file while reading MainActivity$1`; nuestro lector:
  `class … ends inside needs 720914 bytes at 727 of 881`. El conteo va de lo *emitido*, no de lo declarado.
- **El orden de las `class_defs`.** El dexer conserva el orden de entrada, así que la lista emitida se ordena
  por `interno + ".class"` — el mismo orden de rutas que usa `leeClases` — y por eso `MainActivity$1.class`
  (el `$` es 0x24) va **antes** que `MainActivity.class` (el punto es 0x2e) y la `R` después de sus anidadas.
  Con el orden crudo por nombre el dex medía 4 bytes más y no coincidía ni uno.

Quedan rechazadas con nombre, porque no están medidas: la anonima sobre una clase (`javac` la pone como
`super`, no como interface), una anonima dentro de otra, una anonima creada en un método `static` (no hay
`this` que capturar) o en un inicializador de campo (no hay `EnclosingMethod` que anotar), el `if`, los tipos
de dos huecos y las anotaciones que no sean `@Override`.

## 43. `apkc compile`: el `.class` sin `javac`, medido en el binario y dentro del hospedador con forma de navegador

El emisor de los párrafos 41 y 42 era función de librería; la página necesita un comando. `apkc/compilar.go`
añade `apkc compile --android-jar android.jar [-o build/classes] [carpeta-con-java ...]`, que lee los `.java`
del árbol, resuelve los nombres contra el jar y escribe una clase por fichero con el nombre internal — el
mismo sitio y el mismo nombre de donde los lee `apkc dex`. `compile` entra en la lista de comandos que **no**
sondean el SDK (`case "dex", "link", "compile":`), porque dentro de la pestaña no hay SDK que encontrar.

**Medición nativa**, sobre el proyecto real que genera la página (`studio-e2e-java/com.hola.apkcstudio`):

```sh
$G compile --android-jar $HOME/Android/platforms/android-34/android.jar -o $T/compilado $S/src
#   [compile] 2 fuentes -> 6 clases en .../compilado   finished in 33.588615ms
$G dex -o $T/dex-go.dex   $T/compilado            # 6 classes -> 2392 bytes
$G dex -o $T/dex-javac.dex $S/build/classes       # 6 classes -> 2392 bytes
cmp $T/dex-go.dex $T/dex-javac.dex                # identical
```

Los seis ficheros miden 1034, 881, 312, 318, 320 y 362 bytes frente a los 1036, 900, 331, 337, 339 y 390 de
`javac`; `javap -c -p` sobre ellos imprime **las mismas líneas**, y las únicas diferencias del `diff` en las
seis clases son los **números de pool** (`invokespecial #10` frente a `#1`), que no son un hecho semántico. Y
el dex que sale de los unos es **idéntico byte a byte** al que sale de los otros: 2392 = 2392, `cmp` mudo.

**Medición dentro del hospedador** (`shim/navegador-sim.js`, un `fs` en memoria, sin red y sin proceso), con
el jar montado como un fichero más:

```sh
sh -c 'cd shim && node navegador-sim.js \
  --montar $HOME/Android/platforms/android-34/android.jar:/sdk/android.jar \
  --montar $S/src/com/hola/apkcstudio/MainActivity.java:/proyecto/src/com/hola/apkcstudio/MainActivity.java \
  --montar $S/src/com/hola/apkcstudio/R.java:/proyecto/src/com/hola/apkcstudio/R.java \
  --exportar /proyecto/build/classes/com/hola/apkcstudio/MainActivity.class:$T/wasm-classes/.../MainActivity.class \
  ... (seis exportaciones) ... \
  $PREFIX/tmp/apkc-wasm.wasm compile --android-jar /sdk/android.jar -o /proyecto/build/classes /proyecto/src'
```

**Resultado**: `[compile] 2 fuentes -> 6 clases`, **1,16 s** (leer los 26361808 bytes del jar por el `fs`
simulado es lo que tarda), y las seis clases exportadas son **idénticas byte a byte a las que produce el
binario nativo**; dexadas desde fuera, dan los mismos 2392 bytes que las clases de `javac`. O sea: el paso
Java completo corre dentro de la forma de navegador, con el mismo resultado.

La suite, en ese hospedador: **176 PASS, 0 FAIL, 9 SKIP**, y los **doce** tests de Java (`TestLexico*`,
`TestParsear*`, `TestEmisor*`, `TestCompile*`) **pasan dentro** en vez de saltarse, con el jar montado en
`/sdk/android.jar`; el más caro, `TestCompileEscribeLasClasesDelProyecto`, tarda 2,88 s allí dentro. Nativa:
**184 tests**. Tres tests nuevos por el comando: el camino de disco entero (`fuentesJava` →
`compilarProyectoJava` → `escribirClases` → `leeClases` → `dexDeClases` igual byte a byte que javac), el
rechazo que no escribe nada, y el recorrido que ignora los `.kt` y los `.txt` y nombra una raíz que no existe.

**Otra trampa del hospedador, corregida en el propio comando**: `fuentesJava` averiguaba si la raíz era
fichero o árbol con `os.Stat`, y en el `fs` simulado eso falla para un fichero montado; ahora intenta el
`ReadFile` cuando el nombre acaba en `.java` y si no, camina el árbol, que es lo que funcionaba en el §39.

**Una trampa del hospedador, anotada porque costó un salto de prueba**: dentro del `fs` simulado
`os.Stat("/sdk/android.jar")` responde que no existe, mientras que abrirlo funciona (así lo usó el §39). El
test localizaba el jar con `Stat`, así que se saltaba aunque el jar estuviera montado; ahora lo **abre**
(`abrirBuscador`) y guarda el resultado en un `sync.Once`, que además evita releer los 26 MB en cada prueba.

## 44. `apkc build --linker go --compiler go`: un apk completo sin levantar ningún proceso, también en el hospedador con forma de navegador

Con `compile` y `dex` ya medidos, el cableado dentro de `build` son dos ramas y tres banderas. `--compiler
go` sustituye las dos etapas de código (`compileJava` por `compilarFuentesEnGo`, `bundleJava` por
`dexarClasesEnGo`) y salta `closeApiGap`, que es un remiendo de javac: el emisor lee todos los fuentes del
proyecto juntos, así que no tiene punto ciego. Como dentro de la pestaña no hay SDK que sondear, `build`
acepta ahora `--android-jar`, `--sdk-version` y `--codename` (las mismas tres razones del §39) y `main()`
omite el sondeo cuando la línea de comando pide `--linker go` **y** `--compiler go` (`sinProcesos`).

**El portero** (`revisarCompilerGo`, aislado del `LogF` para poder probarlo) rechaza con nombre lo que el
camino todo-en-Go no mide: fuentes `.kt` en `src`, jars en `jar/` (esas los dexa `d8`) y `--aab`. Cuatro
subpruebas, incluida la de un proyecto que sí está medido y no debe rechazar nada.

**Medición nativa**, en frío (`rm -rf build`) sobre el proyecto que genera la página:

```sh
cd $S && apkc build --linker go --compiler go \
  --android-jar $HOME/Android/platforms/android-34/android.jar --sdk-version 34 --codename 14 \
  --keystore $HOME/.android/debug.keystore
#  [build] linking resources with the Go linker   [aapt2 link] 242ms
#  [build] 2 fuentes de Java en Go -> 6 clases
#  [build] 6 clases -> 2392 bytes en build/classes.dex
#  [build] packaging and signing inside apkc      [timing] total 255ms
```

Sale `build/app.apk` de **9868 bytes**, y los oráculos de siempre lo aprueban: `apksigner verify --verbose`
→ v1 `true`, v2 `true`; `zipalign -c -p 4` → `rc 0`; `aapt2 dump badging` → `package name=com.hola.apkcstudio
versionCode='1' platformBuildVersionName='14' platformBuildVersionCode='34' compileSdkVersion='34'` y
`minSdkVersion:'21'`; `aapt2 dump resources` → `type id id=01 entryCount=1, resource 0x7f010000 id/pulsar`;
`zipcheck.py` → 7 entradas, cada registro de acuerdo consigo mismo. Frente a los 10194 bytes del build con
`javac`+`d8`, la diferencia es la que ya midió el §23: el dex de `d8` trae `debug_info` y anotaciones (2844
frente a nuestros 2392).

**Medición dentro del hospedador**, una sola orden y sin proceso (el proyecto se monta en la raíz porque
`build` trabaja en el directorio de proceso, y el jar y el keystore como ficheros más):

```sh
sh -c 'cd shim && node navegador-sim.js \
  --montar $S/AndroidManifest.xml:/AndroidManifest.xml \
  --montar $S/res/values/styles.xml:/res/values/styles.xml \
  --montar $S/res/layout/main.xml:/res/layout/main.xml \
  --montar $S/src/com/hola/apkcstudio/MainActivity.java:/src/com/hola/apkcstudio/MainActivity.java \
  --montar $HOME/Android/platforms/android-34/android.jar:/android.jar \
  --montar $HOME/.android/debug.keystore:/keystore.p12 \
  --exportar /build/app.apk:$O/app.apk \
  $PREFIX/tmp/apkc-wasm.wasm build --linker go --compiler go --android-jar /android.jar \
    --sdk-version 34 --codename 14 --keystore /keystore.p12'
```

**Resultado**: `app.apk` de **9868 bytes**, las mismas siete entradas que el apk nativo y **los mismos bytes
en cada una** (comprobado extrayendo los dos y `cmp` entrada a entrada: `AndroidManifest.xml`,
`META-INF/ANDROID.RSA`, `META-INF/ANDROID.SF`, `META-INF/MANIFEST.MF`, `classes.dex`,
`res/layout/main.xml`, `resources.arsc`), verificado por `apksigner` (v1 y v2) y `zipalign` también el
fichero del hospedador.

**Y una nota sobre qué NO claimar aquí**: el apk **no es reproducible byte a byte**, ni siquiera dos veces
seguidas en nativo — dos `build` separados por dos segundos difieren en 301 bytes, porque la fecha y hora
que el zip guarda de cada fichero entra en el cálculo de la firma. Por eso la comparación que se escribió
fue entrada por entrada; el `cmp` directo de los dos apks marca el byte 11, que es justo el campo de hora.
Si alguien quiere reproducibilidad hará falta fijar esos tiempos, y eso no está medido todavía.

Suite nativa: **190 tests**, cero fallos. Lo que esta sección todavía no medía era chromium: faltaba que
la página llamara a esa orden en vez de a `sign`. Ese cableado está hecho y medido en el §45.

## 45. La página compila dentro de chromium: el apk entero, y byte a byte igual que el build nativo

El cableado son tres ficheros de la página y nada más de apkc. En `studio/index.html` el bloque 2 gana un
`<details>` con el selector del jar (`#android-jar`), el del keystore (`#keystore-compilar`), la versión de
SDK y su nombre en clave (`#sdk-version`, `#codename`), un aviso de estado (`#estado-jar`) y dos botones:
`#usar-android-jar` guarda el jar en OPFS y `#compilar-apk` llama a `compilarDelTodo()`
(`studio/pestana.js`), que pone el cwd en `/proyecto`, corre `build --linker go --compiler go --android-jar
/sdk/android.jar --sdk-version … --codename … --keystore /sdk/keystore.p12` y, si sale 0, deja el apk en el
ancla `#descargar-apk-compilada`. `studio/servidor.js` sirve `/android-jar` desde la variable `ANDROID_JAR`:
el jar entra por el propio origen de la página, no por un CDN (regla 4).

**Aparecieron dos fallos, y los dos eran del hospedador de la pestaña, no de apkc.**

1. **Carrera de arranque.** Desde el segundo comando en adelante, `hospedador.ejecutar` devolvía `codigo 1`
   con la consola vacía. Se reprodujo con dos órdenes seguidas (`version`, `build`) en vez de con una: la
   página llamaba a `iniciar()` dos veces a la vez, cada una montaba su `fs` y el segundo `postMessage`
   pisaba el primero. `studio/hospedador.js` guarda ahora la promesa de arranque en `arranque` (un solo
   `hacerIniciar()` en vuelo; si falla, se borra para reintentar). Esto reinterpreta la nota de
   «intermitencia de OPFS» del §39: no era intermitencia, era esta carrera.
2. **Montar en un subdirectorio.** `/sdk/android.jar` escrito con `escribir` se leía perfecto desde JS y Go
   contestaba `open /sdk/android.jar: No such file or directory`; el mismo jar en la raíz (`/android.jar`)
   abría. `caminoAbierto` del fragmento de fs exige que **cada tramo** del camino exista como nodo de
   memoria, y `montar` solo creaba los directorios intermedios en OPFS. `studio/fs-fragmento.js` crea ahora
   también los nodos de directorio intermedios.

**Medición dentro de chromium** (conducida por CDP, con el binario `GOOS=js` y el jar real):

```sh
sh -c 'cd studio && APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm \
  APK_EJEMPLO=$PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/build/app.apk \
  KS_EJEMPLO=$HOME/.android/debug.keystore \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar node prueba-pestana.js'
```

- el jar entra por `/android-jar` (origen propio): **26361808 bytes** escritos en OPFS, y releídos por el fs
  de la pestaña dan **los mismos 26361808**, con cabecera `PK\3\4`;
- `compilarDelTodo()` deja `ultimo-codigo` en **`0`**, `finished in 5.08793472s` (`[timing] total 5.022s`,
  de los que `961ms` son empaquetar y firmar) y el apk en `/proyecto/build/app.apk`: **9141 bytes**;
- las **21** comprobaciones de la prueba de pestaña pasan, cero fallos, y las **12 peticiones** registradas
  son todas del propio origen: cero peticiones ajenas. (La prueba creció a **32 comprobaciones** y **18
  peticiones** con las cuatro secciones del §46, y sigue sin salir de su origen.)

**Los oráculos de fuera, sobre el fichero que salió de la pestaña** (`studio/e2e.sh` lo pide solo, bloque 4b):

```sh
$BUILD_TOOLS/apksigner verify --verbose --min-sdk-version 21 $PREFIX/tmp/pestana-apk/pestanera-completa.apk
#  Verified using v1 scheme (JAR signing): true
#  Verified using v2 scheme (APK Signature Scheme v2): true
$BUILD_TOOLS/zipalign -c -p 4 …pestanera-completa.apk           # rc 0
python3 ~/Develop/Go/apkc-harness/zipcheck.py …pestanera-completa.apk
#  7 entries, every record agrees with itself; 1 other stored entry at 4
$BUILD_TOOLS/aapt2 dump badging …pestanera-completa.apk
#  package: name='com.myapp' versionCode='1' versionName='0.1' platformBuildVersionName='14'
#  platformBuildVersionCode='34' compileSdkVersion='34' … minSdkVersion:'21' targetSdkVersion:'33'
```

**Y la comparación que cierra el objetivo**: el mismo proyecto, compilado con el binario nativo
(`apkc create proyecto`, el mismo `src/com/myapp/Editado.java` que escribe la prueba, y el mismo
`build --linker go --compiler go`) da **exactamente 9141 bytes** y **los mismos bytes en cada entrada**.
`unzip -p … | sha256sum` de las cinco entradas que importan, pestaña frente a nativo:

| entrada | sha256 (16 primeros), pestaña = nativo | bytes |
| --- | --- | --- |
| `AndroidManifest.xml` | `ed604df837035a84` | 1908 |
| `resources.arsc` | `b38acea1833e0226` | 788 |
| `res/layout/main.xml` | `0cc057bb56f82857` | 604 |
| `classes.dex` | `907c811250803c58` | 1396 |
| `META-INF/MANIFEST.MF` | `96485e1eb5fc0a7f` | 376 |

Es decir: **el apk que produce el navegador sin levantar un solo proceso no se parece al nativo, es igual**,
entrada por entrada; solo difiere el tiempo del zip (lo del §44, que sigue sin claimar reproducibilidad). El
`classes.dex` es de 1396 bytes y no los 2392 del proyecto del studio porque aquí se compilan dos fuentes
distintos: la plantilla de `apkc create` (`com.myapp`, sin clase anónima) más el `Editado` de la prueba.

Con esto el objetivo del repo está cubierto del lado del cliente: `create`, editar, compilar recursos,
compilar Java, dexar, empaquetar y firmar, todo dentro de la pestaña y sin conexión. Lo que sigue siendo
frontera, y no falla silenciosamente: el emisor de `.class` rechaza por nombre lo que no mide (§41, §42) y
el enlazador rechaza los calificadores de carpeta y los recursos que no sean `values`/`layout` (§38).

## 46. También el proyecto que la página *genera*: la clase anónima, el `@+id` y el `style`, dentro de chromium

El §45 compiló la plantilla de `apkc create`, que es el camino fácil: dos fuentes, `MainActivity` sin clase
anónima y una `R` con dos tipos. Lo que la página **escribe por su cuenta** cuando se activa el botón es el
proyecto del §42: `onCreateOptionsMenu` con `new MenuItem.OnMenuItemClickListener() { ... }`,
`Toast.makeText(MainActivity.this, …)`, `@+id/pulsar` en el layout y `@style/AppTheme` en el manifiesto. Ese
es el que estaba medido con `javap` y con `d8` **en node y en el hospedador con forma de navegador**, pero no
en chromium. Se cableó como comprobación 8 de `studio/prueba-pestana.js`: la propia página llama a
`buildProject(estado)` (el mismo generador del zip), escribe los cinco ficheros en OPFS bajo `/estudio` y
corre el build sin procesos.

```sh
sh -c 'cd studio && APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm \
  APK_EJEMPLO=$PREFIX/tmp/studio-e2e-java/com.hola.apkcstudio/build/app.apk \
  KS_EJEMPLO=$HOME/.android/debug.keystore \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar node prueba-pestana.js'
```

**Dentro de chromium**: cinco ficheros escritos, `codigo 0`, y en la consola de la pestaña las dos líneas que
se esperaban — `[build] 2 fuentes de Java en Go -> 6 clases` y `[build] 6 clases -> 2392 bytes en
build/classes.dex`, **los mismos 2392 bytes que midieron el §42 y el §44** — con `[timing] package and sign
go 788ms`, `total 4.066s` y `finished in 4.067974912s` (en la corrida del gate fueron `546ms` y `3.875s`:
el tiempo cambia, los bytes no). El apk sale de **9832 bytes**. La prueba de pestaña queda en **32
comprobaciones** (llegó a **37** con el verificador del §47), cero fallos, con **18 peticiones** y ninguna
fuera de su origen.

**Y el jar entra sin red, por el camino del usuario (comprobación 9)**: se borra `/sdk/android.jar` (la
página pasa a decir «sin android.jar en esta pestaña todavía»), se pone el fichero real en el
`<input type="file">` con `DOM.setFileInputFiles` de CDP, se pulsa `#usar-android-jar` y el aviso queda en
`hay un android.jar de 26361808 bytes en esta pestaña (/sdk/android.jar)` con **cero peticiones** en ese
tramo; después `compilarDelTodo()` vuelve a dar `codigo 0` y el mismo apk de 9141 bytes del §45. Hasta aquí
el «ni siquiera sin conexión» solo estaba probado por el `fetch` a `/android-jar` del servidor de pruebas;
así lo está por el selector de fichero, que es lo que hace alguien con el avión puesto.

**Y se elige una vez, no una por sesión (comprobación 9b)**: recargada la página con `Page.navigate`, el
aviso sigue diciendo `hay un android.jar de 26361808 bytes en esta pestaña (/sdk/android.jar)`, `hospedador`
lee los 26361808 bytes de OPFS y en todo el tramo de recarga no aparece ni una petición a `/android-jar`
(6 peticiones, todas de assets de la propia página); el build vuelve a salir `codigo 0` con los mismos 9141
bytes del §45.

**Frente al nativo, mismo proyecto mismo camino** (`buildProject` escrito a disco con node y
`apkc build --linker go --compiler go`, 733 ms): también **9832 bytes**, y `unzip -p | sha256sum` entrada por
entrada:

| entrada | sha256 (16 primeros), pestaña = nativo | bytes |
| --- | --- | --- |
| `AndroidManifest.xml` | `8d54e26aa55a9833` | 1932 |
| `classes.dex` | `26da19adb481c95b` | 2392 |
| `resources.arsc` | `f1a468123da12d9e` | 936 |
| `res/layout/main.xml` | `c2a0f09d5610287d` | 784 |
| `META-INF/ANDROID.SF` | `305475898b29d5de` | 500 |
| `META-INF/MANIFEST.MF` | `a469730c65d2a806` | 376 |

Los oráculos de fuera lo aprueban: `apksigner verify --verbose --min-sdk-version 21` → v1 `true` y v2 `true`
(1 firmante); `zipalign -c -p 4` → `rc 0`; `zipcheck.py` → 7 entradas coherentes; `aapt2 dump badging` →
`package: name='com.hola.apkcstudio' versionCode='1' versionName='0.1' platformBuildVersionName='14'
platformBuildVersionCode='34'` con `minSdkVersion:'21'` y `targetSdkVersion:'33'`.

**Y un oráculo que no pasa por nuestro código**: el `classes.dex` que escribió el navegador lo vuelve a leer
`d8` (R8, la herramienta de la cadena oficial) con `rc 0`:

```sh
unzip -qo $PREFIX/tmp/pestana-apk/pestanera-estudio.apk classes.dex -d $PREFIX/tmp/e46-d8
$BUILD_TOOLS/d8 --release --lib $HOME/Android/platforms/android-34/android.jar --min-api 21 \
  --output $PREFIX/tmp/e46-d8 $PREFIX/tmp/e46-d8/classes.dex
#  rc 0, 2076 bytes (en `--release` recorta lo que nadie usa, por eso baja de 2392)
```

Con esto `studio/e2e.sh` pide tres cosas más (bloque 4c): que `d8` lea el dex del navegador, que las seis
entradas comparadas coincidan con el nativo y que el apk del proyecto generado pase `apksigner`, `zipalign`,
`zipcheck` y `aapt2`.

**Señales de que el sistema tendría con qué arrancar, sin teléfono de por medio**: `aapt2 dump badging` del
apk de la pestaña declara el componente lanzable — `launchable-activity: name='com.hola.apkcstudio.MainActivity'
label='Hola apkc'` — y en el `classes.dex` que escribió el navegador están las dos clases que ese componente
necesita, `Lcom/hola/apkcstudio/MainActivity;` y su anónima `Lcom/hola/apkcstudio/MainActivity$1;`, más el
interfaz del framework al que se enlaza la llamada (`Landroid/view/MenuItem$OnMenuItemClickListener;`), que
es justamente `invokeinterface` del §42. Comprobado con `unzip -p … classes.dex | strings`.

**Lo que sigue sin medirse es el hecho 2 del objetivo**: instalar y ver arrancar la
actividad, que necesita un teléfono y no depende de ningún código.

## 47. `apkc verify`: la pestaña dictamina su propio apk, cotejado con `apksigner` caso por caso

Cierra el punto **0.2.2** del plan y es lo único que le faltaba a la página para no tener que pedirle a
nadie que le diga si el apk que ella escribió está bien. `apkc/verificar.go` vuelve a leer el paquete desde
el `fs` y comprueba por su cuenta las dos firmas que esta herramienta escribe: recalcula el digest de cada
entrada contra lo que dice `META-INF/MANIFEST.MF`, digiere el manifiesto sección a sección contra el `.SF`,
verifica el PKCS#7 sobre los bytes exactos del `.SF`, y localiza el bloque de firmas antes del directorio
central para recalcular el digest de las tres secciones — con el campo de desplazamiento del registro final
sustituido por el desplazamiento del bloque, que es la trampa del formato que ya describía el §44 — y
comprobar la RSA sobre los datos firmados con la clave del certificado emparejado.

Lo que no está medido se rechaza con nombre: `DSA`/`EC`, un digest que no sea SHA-256, varios firmantes, o
una clave pública que no sea la del certificado. **Y la cadena de confianza no se valida**: un certificado
de debug es auto-firmado y no hay ancla, así que se informa de él (asunto, serie, caducidad) sin afirmar que
sea digno de confianza, igual que hace `apksigner` cuando dice «Verifies, but not trusted».

**Dos cosas del formato las decidió el oráculo, no yo** — y las dos me hacían marcar mal un apk ajeno:

- el **bloque de firmas no entra en el digest del contenido**: el digest cubre las entradas, el directorio
  central y el registro final con el campo sustituido. Medido: un byte cambiado dentro del par de
  *rellenado* (los ceros que alinean a página) deja `apksigner` en `true/true` y `apkc verify` en `ok/ok`; un
  byte cambiado dentro del **valor del par de la v2** da `DOES NOT VERIFY` en `apksigner` y «la firma RSA
  sobre los datos-firmados no verifica» aquí. La diferencia entre los dos puntos es exactamente esa.
- **un par con id desconocido se ignora y se dice**: añadido un par `0x11223344` al bloque, `apksigner`
  sigue diciendo `v1 true, v2 true` y `apkc verify` dice `ok/ok` más la nota `par desconocido en el bloque de
  firmas: 0x11223344`. Herejarlo como fallo habría hecho que esta orden aprobara menos que el oráculo.

**Cotejo con `apksigner`** — el mismo fichero, los dos verificadores, el mismo código de salida. Las tres
primeras filas y la última las pide `studio/e2e.sh` (bloque 5); las demás se midieron a mano con el mismo
binario:

| caso | `apksigner verify --verbose` | `apkc verify` | salida |
| --- | --- | --- | --- |
| apk compilado y firmado en chromium (§46) | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` | `0` y `0` |
| apk firmado por `apksigner` (v1+v2, sin v3) | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` | `0` y `0` |
| apk firmado por `apksigner` con v1+v2+v3 | v1 `true`, v2 `true`, v3 `true` | v1 `ok`, v2 `ok` + nota «la firma v3 no se verificó aquí» | `0` y `0` |
| byte cambiado en el valor del par de la v2 | `DOES NOT VERIFY` | v1 `ok` (las entradas no se tocaron), v2 mal: «la firma RSA sobre los datos-firmados no verifica» | `1` y `1` |
| byte cambiado en `resources.arsc`, que viaja almacenado | `DOES NOT VERIFY` | v1 mal: «no se puede leer resources.arsc: zip: checksum error», v2 mal: «el digest guardado no es el de este contenido» | `1` y `1` |
| byte cambiado en el par de *rellenado* (los ceros que alinean a página) | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` | `0` y `0` |
| par desconocido `0x11223344` añadido al bloque | v1 `true`, v2 `true` | v1 `ok`, v2 `ok` + nota «par desconocido en el bloque de firmas: 0x11223344» | `0` y `0` |
| paquete sin ninguna firma (fuera los tres `META-INF`) | `DOES NOT VERIFY` · «Missing META-INF/MANIFEST.MF» | v1 mal: «faltan piezas de la firma v1 dentro del paquete», v2 mal: «no hay bloque de firmas antes del directorio central» | `1` y `1` |

Nótese la segunda fila: **el verificador lee una firma que no escribió él**, que es la única prueba de que
no está confirmando sus propios supuestos. Y nótese la de `resources.arsc`: ahí la detención no es nuestra,
es el CRC-32 que el zip lleva por entrada, así que un byte torcido lo detectan tres comprobaciones
independientes —el CRC del archivo, el digest del manifiesto y el digest del bloque.

**Coste, medido en los tres hospedadores**: nativo `finished in 1.064384ms` y `2.301ms` sobre el apk de 9832
bytes del §46; dentro del hospedador con forma de navegador (`navegador-sim.js`, mismo `apkc.wasm`)
`finished in 402.866176ms`; dentro de chromium `finished in 51.294976ms` y `57.554944ms`. Es un orden de
magnitud más rápido que el `build` completo: verificar no compila nada.

**En la página** el botón `#verificar-apk` corre `verify build/app.apk` y el dictamen se lee en
`#estado-verificacion`; `compilarDelTodo()` lo llama solo, al terminar un build bueno, para que nadie tenga
que pulsar dos veces. Las dos comprobaciones nuevas de `studio/prueba-pestana.js` (10 y 10b) valen por la
prueba de fuego: dentro de chromium, sobre el apk que la propia pestaña compiló, el veredicto es `v1 ok` y
`v2 ok`; y sobre una copia con **un byte cambiado dentro de `resources.arsc`** — escrito en OPFS como
`build/mal.apk`, con el byte localizado desde el header local de la entrada — el comando sale con `codigo 1`
y dice `v1 mal` y `v2 mal`. La prueba de pestaña queda en **37 comprobaciones**, cero fallos, y las
peticiones siguen siendo todas de su origen.

**Comandos**:

```sh
# nativo y con el oráculo al lado, sobre el apk que salió de chromium
$PREFIX/tmp/apkc-gate verify $PREFIX/tmp/pestana-apk/pestanera-estudio.apk
$BUILD_TOOLS/apksigner verify --verbose --min-sdk-version 21 $PREFIX/tmp/pestana-apk/pestanera-estudio.apk
# dentro del hospedador con forma de navegador
sh -c 'cd shim && node navegador-sim.js --montar $PREFIX/tmp/pestana-apk/pestanera-estudio.apk:/en.apk \
  $PREFIX/tmp/apkc-wasm.wasm verify /en.apk'
# la prueba de chromium, que además del veredicto bueno comprueba el caso alterado
sh -c 'cd studio && APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm APK_EJEMPLO=… KS_EJEMPLO=… \
  ANDROID_JAR=$HOME/Android/platforms/android-34/android.jar node prueba-pestana.js'
```

**Suite**: cinco tests nuevos en `apkc/verificar_test.go` (bueno, byte cambiado en una entrada almacenada,
sin firma dicho por esquemas, valor del par alterado, par desconocido ignorado y anotado). Nativa: **189
PASS, 1 SKIP, 0 fallos** en 15,4 s. Dentro del hospedador con forma de navegador, con el `android.jar`
montado en `/sdk/android.jar`: **181 PASS, 9 SKIP, 0 fallos**, y los cinco tests del verificador pasan
todos ahí (`TestVerificarApruebaLoQueEstaBienFirmado` en 0,16 s, el más rápido en 0,04 s), lo que quiere
decir que el dictamen corre en `js/wasm` igual que en nativo. La comparación contra `apksigner` vive en
`studio/e2e.sh` (bloque 5) para que un falso «ok» propio no pueda colarse en el gate: `bueno: apksigner si,
apkc verify si · con-v3: apksigner si, apkc verify si · bloque-alterado: apksigner no, apkc verify no`.

Con `verify` dentro del binario, la cadena completa —crear, compilar, dexar, empaquetar, firmar y
**dictaminar**— corre en la pestaña. Lo que sigue sin medirse es el hecho 2 del objetivo: instalar ese apk y
ver la actividad arrancar.


## 48. Lo más cerca de `pm install` que hay sin teléfono: la clase que el manifiesto promete está en el dex del navegador

Un apk puede firmar bien y aun así no arrancar si el componente que declara su manifiesto no existe dentro
del `classes.dex`. Eso es comprobable sin dispositivo, y `studio/e2e.sh` lo pide desde ahora, en los dos
apks que salen de chromium:

```sh
lanzable=$(sed -n "s/^launchable-activity: name='\([^']*\)'.*\$/\1/p" $TMP/studio-pestana-badging.log)
interno=$(printf '%s' "$lanzable" | tr '.' '/')           # com.myapp.MainActivity -> com/myapp/MainActivity
unzip -p $TMP/pestana-apk/pestanera-completa.apk classes.dex | strings | grep -q "L$interno;"
```

**Medido**: para el apk de la plantilla, `aapt2 dump badging` declara `launchable-activity: name='com.myapp.MainActivity'`
y esa clase está en el dex que escribió el navegador (el gate lo avisa: `el apk de la pestaña declara
com.myapp.MainActivity y esa clase está dentro de su dex`). Para el proyecto que la página genera (§46), el
gate exige tres nombres dentro del mismo `classes.dex` de 2392 bytes:

- `Lcom/hola/apkcstudio/MainActivity;` — la actividad,
- `Lcom/hola/apkcstudio/MainActivity$1;` — la clase anónima que emite el fuente, con su `this$0` del §42,
- `Landroid/view/MenuItem$OnMenuItemClickListener;` — la interfaz del framework contra la que va el
  `invokeinterface` del botón, es decir el enlace que ART tendría que resolver al cargarla.

Todo junto, la corrida del gate queda así (`sh e2e.sh`, `rc 0`): variante java 10348 bytes y kotlin 678823
bytes verificadas por `apksigner`/`zipcheck`; la pestaña con **37 comprobaciones**; el apk compilado dentro
de chromium de 9141 bytes aprobado por `apksigner` (v1+v2), `zipalign -c -p 4`, `zipcheck.py` y `aapt2 dump
badging`; el del proyecto generado de 9832 bytes **igual entrada por entrada al nativo** y su dex releído
por `d8` con `rc 0`; y el cotejo del verificador propio contra `apksigner` — `bueno: si/si`,
`con-v3: si/si`, `bloque-alterado: no/no`.

**Otra cosa que sí se puede comprobar por fuera, sin teléfono**: `aapt2 dump xmltree --file AndroidManifest.xml`
sobre el apk que escribió la pestaña imprime, dentro del `<activity>` de la línea 12,
`android:exported(0x01010010)=true` junto al `name=".MainActivity"`, y más abajo los dos
`android:name` del filtro (`android.intent.action.MAIN` y `android.intent.category.LAUNCHER`), con
`targetSdkVersion=33` en el `<uses-sdk>`. El atributo `exported` es el que la plataforma empezó a exigir en
las actividades con filtro de intención a partir de API 31; no lo medimos aquí instalando nada, pero su
ausencia habría sido un motivo conocido de rechazo en el `pm install`, y está. Que lo escriba nuestro
`axmlwriter` saliendo del texto del `studio/app.js` es mérito del enlazador del §37, que copia el atributo
como los demás.

**El límite de esta comprobación, dicho sin adornos**: `strings` sobre el dex prueba que el *tipo* existe y
que la *referencia* a la interfaz del framework está escrita. No prueba que el cargador de ART la resuelva
contra el `framework.jar` del aparato ni que `onCreate` sobreviva a la inflación del layout. Eso último es
exactamente el hecho 2 del objetivo y sigue necesitando un teléfono.
## 49. Escribir de verdad: campos, `static {}`, variables locales y aritmética, medidos contra `javap -v -c -l`

Hasta aquí la pestaña compila lo que **ella genera**. «Escribir o pegar el fuente de una app mínima»
pide algo más: un campo propio, una variable local con su cast, una condición. Esta sección mide qué
emite javac para eso y qué faltaba para emitirlo igual.

**El objetivo, leído de javac 21 con `--release 8`** (la versión 52 que escribe este emisor):

```sh
javac -nowarn --release 8 -cp $HOME/Android/platforms/android-34/android.jar -d cj com/myapp/R.java com/myapp/Campos.java
javap -J-Xmx256m -p -v -l -cp cj com.myapp.Campos
```

Con seis campos declarados así:

```java
private int oculto = 3;  protected int visible = 1;  int paquete = 2;
public int publico7 = 4;  static int shared = 5;  final int fijo = 6;
```

los flags son `0x0002`, `0x0004`, `0x0000`, `0x0001`, `0x0008` y `0x0010`, y **solo `fijo` lleva
`ConstantValue: int 6`** — un `final` de instancia con literal se dobla *y* se baja al `<init>`, que es
la parte que nadie adivina bien. El `<init>` medido:

```
 0: aload_0    1: invokespecial Activity.<init>:()V    4: aload_0  5: iconst_3  6: putfield oculto:I
 9: aload_0   10: iconst_1   11: putfield visible:I   14: … paquete  19: … publico7
24: aload_0   25: bipush 6   27: putfield fijo:I      30: return
LineNumberTable: line 8:0, 9:4, 10:9, 11:14, 12:19, 14:24     stack=2, locals=1
```

una fila de línea **por campo**, apuntando al `aload_0` de cada uno, y el `static int shared = 5` fuera
de ahí: va a un `<clinit>` (`flags 0x0008`, `()V`, `stack=1, locals=0`, una fila `line 13: 0`) que javac
escribe **después de todos los métodos declarados**. En el cuerpo, `View v = findViewById(...)` es
`invokevirtual; astore_2`; `Button b = (Button) v` añade `checkcast #43` antes del `astore_3`; `oculto =
oculto + fijo` es `aload_0; aload_0; getfield; bipush 6; iadd; putfield` — la constante `fijo` llega
**doblada**, no leída; y `shared = shared * 2` no lleva receptor: `getstatic; iconst_2; imul; putstatic`.
`contar(int)` sale `stack=2, locals=3` y su cuerpo es `iload_1; iconst_1; isub; istore_2; iload_2;
iconst_3; irem; istore_2; iload_2; ireturn`, idéntico en las dos compilaciones; `a*b`, `a/b`, `a%b` dan
`imul` `0x68`, `idiv` `0x6c`, `irem` `0x70`.

**Tres defectos reales salió a buscar esta medición**, y los tres estaban en código que ya pasaba sus
pruebas:

1. **`astore` estaba mapeado a la fila de `double`.** `guarda()` devolvía `0x47` para un objeto, que es
   `dstore_0`; el JVMS pone `astore_0` en `0x4b`. Nadie lo había visto porque **el subconjunto medido
   nunca guardaba una variable local de tipo referencia**: la plantilla y la `R` no tienen locales. Con
   `0x47` nuestro `View v = …` salía como `dstore_2`, que además cuenta la ranura de a dos.
2. **La cadena de padres del jar pedía un nombre internal y recibía un descriptor.**
   `classreader.go` guarda `super_class` como `Landroid/widget/TextView;` y `enLaCadena` pasaba ese valor
   tal cual a `claseDelJar`, que indexa por `android/widget/TextView`. Resultado: `b.setText(...)`
   respondía `android/widget/Button: Landroid/widget/TextView; no esta en el android.jar`. Se veía solo
   cuando el método **no** está declarado en la clase del receptor, y aquí sí lo estaba todo: el
   `setContentView` de la plantilla vive en `Activity`, un salto de cadena. Añadido
   `internoDeDescriptor` en los dos recorridos (métodos y campos).
3. **Un `final int` propio no se doblaba.** `buscarCampo` marcaba constante solo si era
   `estatico && final`, y javac dobla también el `final` de instancia con literal (medido: `bipush 6`,
   no `getfield fijo`).

**Estado de la comprobación**: con estos tres arreglos, `javap -p -c -l` sobre nuestra clase y sobre la de
javac es **idéntica línea por línea** (112 líneas, `diff` mudo), y `TestEmisorCamposContraJavap` fija los
flags de los seis campos, el orden `<init> / onCreate / contar / <clinit>`, las alturas de pila y ranuras
`(2,1) (3,4) (2,3) (1,0)` y las seis filas de la `LineNumberTable`. Suite nativa: verde.

**Y lo que todavía no vale, dicho sin adornos**: `apkc dex` traduce dieciséis opcodes medidos
(`jvmMeasured`) y se niega con nombre ante el resto — este fuente le trae `bipush`, `iconst_3`, `iadd`,
`imul`, `irem`, `astore_n`, `getstatic`/`putstatic` y `checkcast`. Mientras el traductor no crezca con su
propia medición contra `d8`, **el avance del emisor no llega al apk**: `build --compiler go` sobre un
proyecto con campos falla en el dexer, no en el compilador. La pestaña sigue compilando hoy lo que genera
la página, que es lo que los §44 a §48 demuestran. El siguiente incremento es ese: la tabla del dexer y su
bajada a instrucciones DEX, con el oráculo de siempre — `d8` sobre los `.class` de javac frente a los
nuestros, byte a byte.

## 50. El traductor DEX aprende las cuentas y los campos: un proyecto con variables propias ya llega al apk

El §49 cerraba diciendo que su avance no llegaba al apk porque `apkc dex` se negaba ante los opcodes nuevos.
Este párrafo los mide y los ingresa; al terminar, un proyecto con campos propios compila de principio a fin
sin `javac` ni `d8`.

**Cómo se midió**, dos lados del mismo puente:

- el **ancho** de cada opcode JVM es la diferencia de desplazamientos que imprime `javap -c` sobre las clases
  de javac (el método del §17, repetido sobre la sonda nueva: `bipush` ocupa dos bytes porque el siguiente
  `pc` está a +2, `astore_2` uno, `getstatic` tres);
- la **forma dalvik** sale de las palabras que `d8` escribió sobre esas mismas clases, leídas con nuestro
  propio lector (`apkc/dexvolcado_test.go`, con `APKC_DEX_FIXTURE`; sin la variable se omite: es un anteojo,
  no una comprobación).

| lo que hace el fuente | palabra de `d8` | lectura |
| --- | --- | --- |
| `a + b` (dos registros) | `0x21b0` | `add-int/2addr v1, v2`: escribe sobre el primer operando |
| `a - b` | `0x21b1` | `sub-int/2addr`, misma familia contigua (add `0xb0`, sub `0xb1`, mul `0xb2`, div `0xb3`, rem `0xb4`) |
| `a << b` | `0x21b8` | otro miembro de la misma tabla, verificado más lejos |
| `x = x + 7` | `0x01d8` + `0x0701` | `add-int/lit8 v1, v1, 7`: en la segunda unidad el nibble bajo es el registro fuente y el byte alto el literal |
| `a * 2`, `a / 4`, `a % 3` | `0x01da`, `0x01db`, `0x01dc` | la familia `/lit8` repite el orden add/mul/div/rem, y eso es lo que confirma la de `/2addr` |
| `hasta - 1` | `0x01d8` + `0xff01` | `d8` lo reescribe a `+ (-1)`: mismo opcode, literal `0xff` |
| `a + 300` | `0x01d0` + `0x012c` | `add-int/lit16`, literal de dieciseis bits en la segunda unidad |
| leer `shared` | `0x0060` + índice | `sget v0, field@N` (21c), **sin receptor** |
| escribir `shared` | `const/16 v0, 9` + `0x0067` + índice | `sput v0, field@N` |
| `private int oculto = 3` en el `<init>` | `0x1059` + índice | `iput v0, v1, field@N`: valor primero, objeto después |
| `(Button) v` | `0x011f` + `type@1` | `check-cast v1, type@N` |
| devolver un objeto | `0x0111` | `return-object v1` |

**Observación abierta, escrita tal cual**: para `a * b` con dos parámetros `d8` escribió `0x0192` +
`0x0201`, una forma de dos unidades que todavía no desciframos (`/2addr` habría sido `0x2192`). Nosotros
emitimos `mul-int/2addr` `0xb2`, la posición que le toca en la familia verificada en los otros cuatro
miembros; la prueba de que eso es legal no es nuestra opinión sino el hecho de que **`d8` lee el dex que
escribimos con `rc 0`** y `apksigner`, `zipalign` y `aapt2` lo aprueban abajo.

**Tres defectos del propio traductor salieron al pasar fuentes nuevas**, y los tres estaban en código con
sus pruebas en verde:

1. **fuga de temporales en los `put`**: `accesoCampo` sacaba valor y objeto de la pila simulada y nunca los
   devolvía a la bolsa, así que un `<init>` con seis campos inicializados pedía seis temporales y reventaba
   contra la pila máxima que midió javap (`iconst_2: it needs more temporaries than the max stack`).
2. **`getstatic` vaciaba la pila**: la ruta de lectura llamaba a `pop` antes de mirar si era lectura, así
   que el primer `getstatic` de un método sin nada en la pila respondía `the JVM stack is empty`.
3. **la base de la primera pasada se quedó corta**: al atar un temporal a un local (`View v = findViewById(…)`)
   ese registro deja de ser reutilizable, y con `base = max_stack` el cuarto temporal chocaba contra el
   borde. La primera pasada usa ahora `max_stack + locales + 1`; la segunda sigue partiendo de lo que gastó
   la primera, así que **las palabras del proyecto del studio no cambian** y su comparación contra `d8`
   (§21, §23) sigue en verde.

**End-to-end**, un proyecto que la página no genera hoy —`private int pulsos = 0`, `static int total = 5`,
`Button boton = (Button) findViewById(R.id.pulsar)`, `pulsos = pulsos + 1`, `total = total * 2`,
`boton.setText("pulsar")`— con `apkc build --linker go --compiler go`:

```
[build] 2 fuentes de Java en Go -> 5 clases
[build] 5 clases -> 1748 bytes en build/classes.dex
[timing] total 834ms        → build/app.apk de 9555 bytes
```

y los oráculos, todos de fuera: `apksigner verify --verbose` → v1 `true` y v2 `true`; `zipalign -c -p 4` →
`rc 0`; `zipcheck.py` → 7 entradas coherentes; `aapt2 dump badging` → `package: name='com.myapp'` y
`launchable-activity: name='com.myapp.MainActivity'`; `d8 --release` sobre el `classes.dex` que escribimos
→ `rc 0`; `apkc verify` sobre el apk → v1 `ok` y v2 `ok`. El mismo proyecto con la cadena de siempre
(`apkc build`, o sea `javac` + `d8`) tarda **10,095 s** y da 9995 bytes con un dex de 2136: los 388 bytes
de diferencia en el dex son `debug_info` y anotaciones, el hueco ya escrito del §23, no una diferencia de
semántica.

**Y la comprobación que cierra el círculo del §41**: sobre la muestra de campos, `apkc dex` de las clases de
javac y `apkc dex` de las nuestras dan **el mismo fichero byte a byte, 1960 bytes** (`cmp` mudo). Suite
nativa: **190 PASS, 2 SKIP, 0 fallos**; en el hospedador con forma de navegador, **182 PASS, 10 SKIP, 0
fallos** (el salto de uno en cada lado es `TestEmisorCamposContraJavap`, que pasa también en `js/wasm`, y
el anteojo del volcado, que se omite sin su variable).

**Lo que sigue rechazado por nombre**, para que nadie crea que esto es un compilador de Java completo: el
`if` (los anchos de las branquias y sus formatos dalvik todavía no están medidos), la concatenación de
cadenas (que en la versión 52 es una cadena de `StringBuilder.append`), arreglos, bucles, `long`/`float`,
anotaciones que no sean `@Override` y cualquier miembro cuyo `private`/`protected` no hayamos leído en un
`javap`.

## 51. El fuente se escribe en la página: campos, cadena de llamadas y cuentas, compilados y verificados en chromium

Lo que prometía el objetivo con las palabras «escribir o pegar el fuente» no era recompilar la plantilla,
sino que alguien cambiara el `.java` desde el editor y siguiera teniendo apk. Con los dos párrafos anteriores
eso ya se puede comprobar dentro del navegador, y `studio/prueba-pestana.js` lo hace (comprobación 11, que
sube la prueba a **39**): se sobrescribe `/proyecto/src/com/myapp/MainActivity.java` con un fuente que la
página no genera,

```java
public class MainActivity extends Activity {
    private int pulsos = 0;
    static int total = 5;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        View raiz = getWindow().getDecorView();
        pulsos = pulsos + 1;
        total = total * 2;
        raiz.setVisibility(0);
        Toast.makeText(this, "cuenta", Toast.LENGTH_SHORT).show();
    }
}
```

se pulsa el mismo camino del botón (`compilarDelTodo()`, o sea `build --linker go --compiler go` con el jar
de OPFS) y sale **un apk de 9459 bytes con `ultimo-codigo` 0**; la pestaña lo verifica sola (`apkc verify` →
v1 `ok`, 4 entradas, y v2 `ok`, 7 entradas) y ninguna de las **18** peticiones sale de su origen.

**Lo mismo, medido fuera de la pestaña sobre ese mismo fichero** (`studio/e2e.sh`, bloque 4d): `apksigner
verify --verbose --min-sdk-version 21` → v1 `true` y v2 `true`; `zipalign -c -p 4` → `rc 0`; `zipcheck.py` →
las entradas coherentes; `aapt2 dump badging` → `launchable-activity: name='com.myapp.MainActivity'`, y en
el `classes.dex` que escribió el navegador están la clase, los dos nombres de campo (`pulsos`, `total`) y
el método del framework al que enlaza la cadena de llamadas (`setVisibility`).

**Y la igualdad con la cadena de siempre, otra vez byte a byte**: el mismo fuente, compilado con `javac
--release 8` y con nuestro emisor, da `javap -p -c -l` **idéntico línea por línea**; `apkc dex` sobre las
clases de javac y sobre las nuestras produce **el mismo fichero de 1656 bytes** (`cmp` mudo), y `d8` lee ese
dex con `rc 0`. En nativo, `apkc compile` tardó 42 ms sobre esos dos fuentes.

Es el cierre del §44 al §50 en una frase: **la página compila un fuente que alguien escribió, y el
resultado es el mismo que daría `javac` + `d8` hasta donde este subconjunto llega**. Lo del teléfono sigue
siendo el hecho 2 del objetivo: instalar y ver la ventana.

## 52. El `if`, medido dos veces: la inversión de javac en el `.class` y las tres formas de rama en el dex

El `if` era la pieza que el §50 dejaba fuera: un fuente con condicionales no se podía escribir en la página.
Se midió en dos tramos, con dos oráculos que no comparten código con este fork —`javap` sobre las clases de
javac 21 y las palabras de `d8`— y con una sonda de doce métodos, en `$PREFIX/tmp/e52/p`:

```java
public int igual(int a, int b) { if (a == b) { return 1; } return 0; }
public int mayorcero(int a) { if (a > 0) { return 2; } return 3; }
public int nulo(Object o) { if (o == null) { return 4; } return 5; }
public int elseCorto(int a) { if (a == 0) { return 6; } else { return 7; } }
public int conElse(int a) { if (a == 0) { shared = 1; } else { shared = 2; } return shared; }
public int plegable(int a) { if (a == 0) { a = 1; } else { a = 2; } return a; }
public int anidadaConElse(int a, int b) { if (a == b) { if (a > 0) { shared = 1; } else { shared = 2; } } else { shared = 3; } return shared; }
public int tresNiveles(int a, int b) { if (a == b) { if (a > 0) { if (b > 0) { return 1; } } } return 2; }
```

**Primera medición — el `.class`.** `javac --release 8` y `apkc compile --android-jar android-34/android.jar`
sobre el mismo fuente, y `javap -p -c -l` a los dos lados: **idéntico línea por línea**, 186 líneas para
`Branquias` (doce métodos con `if`, tres de ellos anidados) y 81 para `Saltos`. Lo que se leyó:

- **javac emite la condición invertida**, y la tabla salió de esas 186 líneas, no del manual:
  `==`→`if_icmpne`, `!=`→`if_icmpeq`, `<`→`if_icmpge`, `>=`→`if_icmplt`, `>`→`if_icmple`, `<=`→`if_icmpgt`;
  contra cero, con una sola palabra en la pila simulada: `==0`→`ifne`, `!=0`→`ifeq`, `<0`→`ifge`,
  `>=0`→`iflt`, `>0`→`ifle`, `<=0`→`ifgt`; contra null: `==null`→`ifnonnull`, `!=null`→`ifnull`.
  `0 == a` no se dobla: javac escribe el literal primero (`iconst_0; iload_1; if_icmpne`).
- **el `goto` solo aparece si el `then` no sale por su cuenta**: `if (a == b) { return 1; } return 0;` no
  lleva ninguno, y `conElse` lleva `8: goto 15`. Un `if` sin `else` cuyo cuerpo sí cae al final tampoco lo
  lleva: el `else` vacío y el final son la misma posición.
- **el desplazamiento se cuenta desde el opcode**, no desde el operando: `2: if_icmpne 7` tiene por operando
  `00 05`. Todas las branquias miden tres bytes, así que el ancho de la tabla de caminantes (`jvmMeasured`)
  es 2 operandos para las quince formas nuevas.
- **la `LineNumberTable` de un método escrito en una línea tiene una sola fila** (`line 12: 0` en
  `anidadaConElse`), aunque el cuerpo tenga cuatro saltos.

**Segunda medición — las palabras de `d8`.** Tres formas de rama nuevas (`22t`, `21t` y `10t`), leídas de sus
ficheros
(`--release --min-api 21 --lib android.jar`) sobre las clases de javac, y verificados contra los nuestros:

| qué | d8 | el nuestro |
| --- | --- | --- |
| `if (a == b)` → `if-ne v1, v2, +4` | `2133 0004` | `4333 0004` |
| `if (a > 0)` → `if-lez v1, +4` | `013d 0004` | `033d 0004` |
| `if (o == null)` → `if-nez v1, +4` | `0139 0004` | `0339 0004` |
| `if (o != null)` → `if-eqz v1, +5` | `0138 0005` | `0338 0005` |
| `goto` → `+2` | `0228` | `0228` |

El `22t` (dos registros) reparte los registros en nibbles igual que el `22c` y lleva el desplazamiento con
signo en la segunda unidad; el `21t` (contra cero y contra null) usa un byte entero para el registro y el
segundo para el desplazamiento; el `10t` es una sola unidad con ocho bits con signo en la mitad alta. El
desplazamiento va **en unidades de código y desde la propia instrucción**, igual que en el JVMS pero con la
unidad de dos bytes. El null es cero para dalvik, así que `ifnull`/`ifnonnull` bajan a `if-eqz`/`if-nez`.

**Dos defectos reales salieron a buscar, y los dos estaban en código que ya pasaba sus tests.**

1. El emisor escribía `13: goto 20` donde javac escribe `13: goto 27`. En la posición 20 no había ninguna
   instrucción antes de otro `goto`: era una etiqueta vacia. javac encadena el salto por encima, y el
   emisor ahora hace lo mismo (`encadenaGotos`, pasada final antes de cerrar el atributo `Code`). Con
   esto el `javap` de los tres métodos anidados sale idéntico; sin esto se habría colado un dex que no es
   el que escribiría javac, que es justo lo que el §49 prometía no volver a hacer.
2. `desmontaPalabras` —el lector con el que se comparan nuestras palabras y las de `d8`— no conocía ni las
   branquias ni `sget`, `sput`, `iput`, `check-cast`, `return-object` (nueve opcodes que el §50 ya escribía)
   ni la familia `/lit8` con su `add-int/lit16` (seis que escribía `d8` y había que leer para comparar). Ahora
   conoce veintinueve mas, repartidos en cuatro casos nuevos de formato (`10t`, `21t`, `22t` y el `22s`).
   Y el `22s` **no** es un `22c`: la palabra de d8 `00d8 0100` es
   `add-int/lit8 v0, v0, 1` —registro de ocho bits en el byte alto de la primera unidad, literal con signo
   en el byte alto de la segunda y `vBB` en el nibble bajo—. La primera versión lo leía como el `22c` e
   imprimía `add-int/lit8 v, v, 256`, un número que ningún literal de ocho bits puede tener: esa absurdez
   fue la señal.

**Cotejo con `d8` sobre los veintiún cuerpos del par** (las dos sondas, instrucciones desmontadas sin
números de registro): **quince salen iguales instrucción por instrucción y con el mismo desplazamiento**,
incluido `tresNiveles` (`if-ne +8 | if-lez +6 | if-lez +4 | const/4 1 | return | const/4 2 | return`, palabra
por palabra la de d8 salvo el registro). Los seis restantes difieren por tres optimizaciones de d8, cada
una con su nombre:

- los dos `<clinit>`: d8 deja `return-void` (pliega la constante estática `shared = 5`); nosotros escribimos
  el `const/4` + `sput` que escribió javac;
- `conElse`, `anidadaConElse`, `enElElse`: d8 fusiona las colas idénticas (un solo `sput` al final, con
  saltos que a veces van hacia atrás); nosotros conservamos la forma de javac, con un `sput` por rama;
- `plegable`: d8 propaga las copias (`istore`+`iload` se convierten en `return`); nosotros no.

Ninguna de las tres es una diferencia de semántica que nos toque decidir: el dex que escribimos **lo relee
`d8` y lo re-dexa con `rc 0`** en los dos casos, y `apkc dex` da **el mismo fichero byte a byte** sobre las
clases de javac que sobre las nuestras (1796 bytes la sonda entera, 1684 el `MainActivity` de la página), que
es la prueba de que el traductor no distingue quién escribió el `.class`.

**Lo que se rechaza por nombre**, porque no lo midió ninguna sonda: una condición que no sea comparación (el
booleano suelto, donde javac usa `ifeq` sobre el valor), el `null` a la izquierda, dos referencias comparadas
entre sí, los tipos de dos huecos en una comparación, una rama **hacia atrás** (un bucle: el reparto de
registros en el punto de fusión no está medido; `d8` lo calcula con análisis de viveza), y un `goto` que pase
de 127 unidades (`goto/16`, 0x29, sin medir).

**Y en la página**: `studio/prueba-pestana.js` sobrescribe el fuente con dos branquias
(`if (pulsos > 2) { total = total * 2; } else { total = total + 1; }` y `if (raiz != null) { raiz.setVisibility(0); }`),
compila entero en chromium con `build --linker go --compiler go` y saca **un apk de 9477 bytes** que la
pestaña verifica sola (v1 `ok`, v2 `ok`) sin hacer ninguna petición fuera de su origen. Fuera de la pestaña,
`studio/e2e.sh` (bloque 4d) le pasa los mismos oráculos de siempre y **el nuevo**: `d8` relee el `classes.dex`
que escribió el navegador con las branquias dentro, `rc 0`.

Tests permanentes: `TestEmisorBranquiasContraJavap` (siete sucesiones con sus destinos, dos de ellas
anidadas), `TestTraduceBranquiasContraLasPalabrasDeD8` (seis cuerpos, con los desplazamientos que escribió
d8) y `TestRechazaUnaRamaHaciaAtras` (el `while` que no se acepta). Suite: **199 PASS + 2 SKIP** nativa y
**181 PASS + 20 SKIP** en el hospedador con forma de navegador.

## 53. Concatenar cadenas y plegar constantes: la versión 52 pega con `StringBuilder`, y `1 / 0` no se dobla

El fuente que alguien escribe en la página necesita decir «llevas 3», no solo decir «cuenta». Con `--release 8`
sobre javac 21, la concatenación **no** es `invokedynamic` (eso llega en la 55): es una cadena de
`StringBuilder`, y el andamiaje que falta en el subconjunto eran tres cosas —la cadena de `append`, el plegado
de constantes y el paréntesis como agrupación—. Sonda en `$PREFIX/tmp/e53/p` (cuatro clases) y comando:

```sh
javac --release 8 -d cj p/*.java                    # el oráculo
apkc compile --android-jar android-34/android.jar -o ours p
for c in Cadena Cero Comp Pliegue; do
  javap -p -c -l cj/p/$c.class   | sed 's/#[0-9]*/#n/g; s/  */ /g' > a.txt
  javap -p -c -l ours/p/$c.class | sed 's/#[0-9]*/#n/g; s/  */ /g' > b.txt
  cmp a.txt b.txt || diff a.txt b.txt
done
```

`javap` sale **idéntico línea por línea** en las cuatro clases: 134 líneas en `Cadena`, 35 en `Cero`, 206 en
`Comp` (la sonda de las branquias, que sigue igual) y 54 en `Pliegue`. Cuatro reglas, todas leídas de esas
líneas:

1. **la cadena de `append` empieza por el primer operando**: `a + "-" + b` es `new StringBuilder; dup;
   invokespecial <init>; aload_1; append(String); ldc "-"; append(String); aload_2; append(String); toString`.
   javac **no** usa el constructor `StringBuilder(String)`, así que el primer `ldc`/`aload` se appendea.
2. **los literales adjacentes se doblan en una sola constante**, y si después del doble no queda más que
   cadena, no hay `StringBuilder`: `"hola" + " " + 3` es `0: ldc "hola 3"; 2: areturn`. El entero se escribe
   en decimal, igual que lo escribiría `String.valueOf`.
3. **la agrupación es la del fuente**: en `1 + 2 + x + "s"` el subárbol `(1+2)+x` no es concatenación, sale
   como cuenta entera (`iconst_3; iload_1; iadd`) y se appendea con `append:(I)`. Por eso el analizador
   necesita el paréntesis: `"s" + (x + 1)` da `ldc "s"; append(String); iload_1; iconst_1; iadd; append(I)`.
4. **el tipo del operando eligió la sobrecarga**, medida la de entero, la de cadena y la de referencia
   (`"v=" + o` sale `append:(Ljava/lang/Object;)`). Lo no medido se rechaza: `append(Z)`, `append(C)`,
   `append(J)` y `append(D)` no los leyimos.

**El plegado de constantes enteras** tiene su propia sonda (`Pliegue`) y un límite que no es obvio:
`1 + 2` sale `iconst_3`, `1 + 2 * 3` sale `bipush 7` (el 7 va en `bipush` porque `iconst` llega al 5),
`7 - 10` sale `bipush -3`, `9 / 2` sale `iconst_4` y `9 % 4` sale `iconst_1`. **Pero `1 / 0` no se dobla**:
javac escribe `iconst_1; iconst_0; idiv`, que es la `ArithmeticException` en tiempo de ejecución y no en la
compilación. Nuestro plegador respeta esa excepción: con divisor cero devuelve «no plegable» y la cuenta se
emite entera, medido en `Cero` (35 líneas iguales).

**Dos piezas que faltaban salieron a la primera de dex**: `areturn` (`0xb0`) estaba en el traductor pero **no**
en la tabla del caminante, así que `apkc dex` se paraba con `byte 0xb0: opcode outside the table this fork
measured` en cuanto el fuente devolvía una cadena; y el analizador leía el `(` de `(x + 1)` como el principio
de un molde. El ancho de `areturn` es el que marca `javap`: `0: ldc …; 2: areturn` con la longitud del código
en 3, o sea un byte y ningún operando.

**Cotejo con `d8` sobre los treinta y nueve cuerpos** (las cuatro clases, desmontadas sin números de
registro): **veinticinco salen iguales instrucción por instrucción, con los mismos ids y los mismos
desplazamientos** —entre ellos `todoLiteral`, que es `const-string; return-object` en los dos ficheros— y
veintisiete si se ignoran los ids. Los doce que se separan tienen cuatro motivos, cada uno con su nombre:

- **`d8` reescribe la cadena de `StringBuilder`** (siete cuerpos, los siete que concatenan algo que no es
  solo literales): usa `invoke-direct <init>:(Ljava/lang/String;)` con la primera constante, y se salta los
  `move-result-object` porque sabe que `append` devuelve `this` — en `conEntero` quedan siete instrucciones y
  quince palabras (`new-instance | const-string | invoke-direct | invoke-virtual | invoke-virtual |
  move-result-object | return-object`) frente a nuestras diez instrucciones y veinte palabras;
- **pliega el `<clinit>`** de la constante estática (`static int total = 5` queda en `return-void`, un cuerpo);
- **mete el operando constante en la forma `/lit8`** (`add-int/lit8 v, v, 3` donde nosotros escribimos
  `const/4 3` y `add-int/2addr`, y `div-int/lit8 v, v, 0` en `1 / 0`), tres cuerpos;
- **compara contra cero con la forma `/z`** (`if-nez v` en `if (0 == a)`, donde javac carga el 0 en una
  palabra y `d8` lo reconoce como el cero), un cuerpo. En esta sonda no aparece la fusión de colas del §52:
  los `if` de `Comp` terminan cada rama en su propio `return`.

Nuestro fichero, con todo, **lo relee `d8` y lo re-dexa con `rc 0`**, y `apkc dex` da **el mismo fichero byte a
byte** sobre las clases de javac que sobre las nuestras: **3240 bytes**, `cmp` mudo. La prueba de que el
traductor no distingue quién escribió el `.class` sigue siendo la misma que en el §50.

**Lo que se rechaza por nombre**: concatenar con una llamada cuyo tipo no se puede estimar sin emitirla
(devolvería el mensaje «la concatenación está medida sobre literales, campos, variables y `this`»), un
`append` de tipo no medido, y todo lo de siempre: arreglos, bucles, `switch`.

Tests permanentes: `TestEmisorCadenaContraJavap` (ocho cuerpos con su sucesión de mnemonicos, sus
desplazamientos y sus destinos) y `TestTraduceLaConcatenacionBajaAStringBuilder` (la cadena baja al dex con
las formas medidas, y el plegado llega ya plegado: `1 / 0` conserva el `div-int/2addr` con su cero). Suite
después de este párrafo: **201 PASS + 2 SKIP** nativa y **181 PASS + 22 SKIP** en el hospedador con forma de
navegador (los dos tests nuevos piden el `android.jar` real a través de `jarDelSDK`, así que en el hospedador
sin montarlo se saltan: esos son los dos SKIP que faltaban).

**Y el proyecto entero, con la concatenación dentro**: la pestaña escribe `Toast.makeText(this, "cuenta " +
pulsos, Toast.LENGTH_SHORT).show()` en el fuente propio de su comprobación 11 y saca **un apk de 9598 bytes**
que verifica sola (v1 `ok`, v2 `ok`) sin salir de su origen; el bloque 4d del gate le pasa los oráculos de
siempre, **exige `cuenta` y `StringBuilder` en el dex** que escribió el navegador y `d8` lo relee con `rc 0`.
Nativo, sobre el proyecto generado por la página con ese mismo fuente (`$PREFIX/tmp/e53/medir-pagina.sh`):
**431 ms y un apk de 10096 bytes** —`apkc build --linker go --compiler go`, sin un solo proceso—, que
`apksigner` aprueba por v1 y v2, `zipalign -c -p 4` aprueba, `zipcheck.py` aprueba, `aapt2 dump badging`
lee como `launchable-activity: name='com.hola.apkcstudio.MainActivity'`, `apkc verify` dictamina `ok` y
`d8` relee su dex con `rc 0` (re-dexado: 2424 bytes frente a los 2736 que escribimos, otra vez por la
reescritura de la cadena).

## 54. La app del contador: anónima, campo del exterior, `if` y concatenación en el mismo cuerpo

El objetivo dice «un APK utilizable», y utilizable quiere decir que algo cambie cuando se pulsa. El fuente
que hace eso cabe en lo medido salvo una pieza: **escribir un campo del exterior desde dentro de la
anónima**. Sonda en `$PREFIX/tmp/e54/p`, con `javac --release 8 -cp android.jar` y `apkc compile` sobre el
mismo fichero:

```java
public class Contador extends Activity {
    int pulsos = 0;
    static int total = 5;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total = total + pulsos;
                if (pulsos > 3) { v.setContentDescription("muchos " + pulsos); }
                else { v.setContentDescription("pulsos " + pulsos); }
            }
        });
    }
}
```

**`javap -p -c -l` sale idéntico línea por línea** en las cinco clases: `Contador` (49 líneas),
`Contador$1` (69), `R` (10), `R$layout` (12) y `R$id` (12). Tres reglas nuevas, todas leídas de esas 69
líneas del `onClick`:

- **el campo del exterior se alcanza por la captura, dos veces**: `pulsos = pulsos + 1` es
  `aload_0; getfield this$0:Lp/Contador;` (el receptor del `putfield`), otra `aload_0; getfield this$0` con
  su `getfield p/Contador.pulsos:I` (la lectura del valor), `iconst_1; iadd`, `putfield p/Contador.pulsos:I`.
  javac **no** duplica el receptor con `dup`: vuelve a cargarlo. `d8` sí comparte la lectura —de ahí que su
  cuerpo tenga un `iget-object` menos que el nuestro—, que es una optimización suya, no una diferencia de
  semántica.
- **el estático del exterior no pasa por la captura**: `total = total + pulsos` es
  `getstatic p/Contador.total:I`, la captura para leer `pulsos`, `iadd` y `putstatic` — las formas del §50,
  ahora desde la anónima.
- **el `if` del §52 y la concatenación del §53 viven dentro del `onClick`**: `if_icmple 73`, dos cadenas de
  `StringBuilder` cerradas con `setContentDescription:(Ljava/lang/CharSequence;)V` (se les pasa un `String`,
  que es lo que ya midió `setText` en el §49) y el `goto 102` del `then` que no sale por su cuenta.

**El campo tiene que ser del paquete, y el `private` se rechaza por nombre.** Con `private int pulsos`, javac
no escribe el `getfield` contra el campo: escribe dos **métodos sintéticos del exterior**, leídos en la primera
corrida de esta sonda —

```
 8: invokestatic  // Method p/Contador.access$000:(Lp/Contador;)I
13: invokestatic  // Method p/Contador.access$002:(Lp/Contador;I)I
16: pop
```

lectura `access$000`, escritura `access$002` **que devuelve el valor** (de ahí el `pop`). Ni el número
(`000`, `002`) ni el lugar que ocupan en el `methods` del exterior están medidos, así que el emisor los
rechaza con ese texto en vez de inventarlos: es el único caso del contador que queda fuera, y se dice con su
motivo.

**Cotejo con `d8`**: sobre los ocho cuerpos de la sonda, **cinco salen iguales** instrucción por instrucción y
con los mismos ids y desplazamientos; los tres que se separan son los tres motivos de siempre —el `<clinit>`
plegado, la cadena de `StringBuilder` reescrita (con su `add-int/lit8` y su `invoke-direct <init>(String)`) y
el `this$0` compartido—. `apkc dex` da **el mismo fichero de 2304 bytes** sobre las clases de javac que sobre
las nuestras, y `d8` relee el nuestro con `rc 0`.

Test permanente: `TestEmisorAnonimaConCampoDelExterior`, con las dos sucesiones completas medidas (`enruta` y
el `onClick`).

**Falta una nota de proyecto**: el layout que escribe `apkc create` no tiene ningún `@+id`, así que un fuente
que haga `R.id.pulsar` se rechaza con «el campo id no esta en com/myapp/R ni en sus padres» —y está bien que
pase, porque el recurso no existe—. Al anadirle el botón (`android:id="@+id/pulsar"`), el enlazador de Go emite
su clase `id` (`public static final class id { public static final int pulsar=0x7f010000; }`) y el fuente
compila entero. Eso es lo que hace la comprobación 12 de `studio/prueba-pestana.js`: escribe el layout y el
`MainActivity` del contador en el proyecto que creó la propia pestaña, y compila dentro de chromium —**41
comprobaciones, 0 fallos, un apk de 10098 bytes** con v1 `ok` y v2 `ok`, y las **18** peticiones todas de su
origen—.

Fuera del navegador, el mismo proyecto (`apkc create` + los dos ficheros escritos a mano,
`$PREFIX/tmp/e54/medir-proyecto.sh` y el `repro` de la sonda): **499 ms y un apk de 10048 bytes** con
`--linker go --compiler go`, `apksigner` por v1 y v2, `zipalign -c -p 4`, `zipcheck.py`, `aapt2 dump badging`
que lee `launchable-activity: name='com.myapp.MainActivity'`, y su `classes.dex` (2724 bytes) **releído por
`d8` con `rc 0`** (re-dexado a 2408). El bloque 4e del gate comprueba además que en el dex del navegador están
`Lcom/myapp/MainActivity$1;`, `Landroid/view/View$OnClickListener;`, `setContentDescription`, `pulsos` y la
cadena que arma el concatenedor.

