# Evidencia — §241-§259

## 241. ApkShare desde el zip: diamante, `for` con dos puntos y comodines

Medido el 2026-11-23 sobre `efbf139` (`fix(java): admite new ArrayList sin constructor`), `074e0bc`
(`fix(java): admite el for Each en la fuente`) y `4c384dd` (`fix(java): admite comodines en argumentos de
tipo`). El origen es el zip real que el usuario compiló desde `~/Downloads/ApkShare-main`: el `check` se
paró en `App.java:32` con `un new ArrayList sin constructor no esta medido`.

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  JAVAC17=/home/leonisauro/opt/jdk17/bin/javac GOMAXPROCS=2 GOGC=50 \
  go test -p=1 -count=1 \
  -run 'TestDiamanteDelNuevoContraJavac17|TestPorCadaDeAppContraJavac17|TestComodinesDeTipoContraJavac17' -v .
```

**Resultado**: los tres frentes en verde; la suite completa del árbol final, `ok … 92.102s`.

**Los tres bloqueos, con la fuente exacta.** `new ArrayList<>()` (`src/share/apps/util/App.java:32`) y
`new ArrayList<Apk>()`: el descriptor sigue siendo el borrado `Ljava/util/ArrayList;` y la construcción
baja a `new-instance` + `invoke-direct <init>` (`efbf139`). El `for (String path :
info.splitSourceDirs)` (`App.java:37`), con arreglo e `Iterable` y con `break`/`continue` dentro
(`074e0bc`). Y `AdapterView<?>` (`src/share/apps/util/MainActivity.java:54`): `?`, `? extends` y `?
super` bajan a `*`, `+cota` y `-cota` en la `Signature`, con el descriptor del borrado (`4c384dd`).

**Oráculo.** `javac` 17 en los tres frentes, y dex **byte a byte** en los casos planos.

**Falsificaciones.** Apagar cada interruptor devuelve el mensaje literal que vio el usuario: `no esta
medido`, `se esperaba ";" y hay ":"` y `el caracter '?' no es de este subconjunto`.

**Fronteras de este `§`.** La anónima con comodín difiere **76 B** del dex de `javac`, así que su oráculo
es solo el `.class`. Siguen abiertos el `final` local de `MainActivity.java:137`, la lambda
`apk -> shareApp(apk)` de `:156` y `App.java:28`/`:35`. No se corrió gate, chromium, red ni publicación
en esa medición; **el gate de `§230` es anterior a estos tres arreglos y no los cubre**, y el repetido en
`§242` sobre el árbol final salió verde.

## 242. El gate final sobre el árbol final, en verde

Medido el 2026-11-23 en la raíz sobre `912a2c4` (`chore(studio): clics del emulador llegan al canvas bajo
el inspector`, con el submódulo en `f2710a1`), autorizado por el usuario con «Preparar y correr gate». Es
el primer gate que cubre `§235`-`§241`: el de `§230` se corrió antes de esos frentes.

**Matiz posterior.** «Árbol final» lo era **en ese momento**: hoy el HEAD lleva más de veinte commits por
delante de `912a2c4`, así que las cifras de este `§` describen ese commit y no el HEAD. El gate que ahora
corre sobre el HEAD se registrará en su propio `§` cuando se mida.

**Preparación.** La capa `test` en verde (`ok … 94.453s`); se borraron los fixtures cacheados del arnés
(`$PREFIX/tmp/studio-e2e-*`, `pestana-*` y `studio-e46*`); `LD_LIBRARY_PATH` al shim de
`libsimdutf.so.36`; y `TCR` apuntando a un wrapper local que ejecuta directo
(`/home/leonisauro/.cache/apkc-web/gate-direct.sh`), porque el guardia de RAM de `tcr` mataba `javac` con
137.

```sh
TCR=/home/leonisauro/.cache/apkc-web/gate-direct.sh \
LD_LIBRARY_PATH=/home/leonisauro/.cache/apkc-web-gate-final/shimlib \
  sh tools/entorno.sh gate
```

**Salida.** `e2e: todo en orden`. Cubre: las plantillas Contador, ciclo, menú y traducido firmadas y
alineadas; la vista previa; crear (36 comprobaciones); la pestaña (229); release con CN=Release y firma
v1+v2+v3; la v4 con su `.idsig`; el `.aab` con su v1 JAR; la rotación v3.1; la pestaña compilando con
`apksigner`, `zipalign`, `zipcheck` y `aapt2`; el proyecto generado **byte a byte** igual al nativo; el
fuente escrito a mano; la app del contador; la firma (bueno, con-v3, bloque-alterado, rotación, EC
P-256/P-384 y SHA-1); y el icono.

**Los dos intentos que no llegaron.** El primero lo mató el guardia de RAM (`java no compila`, `Killed`);
el segundo chocó con una regresión de UI —`previa-detalle-modelo` capturaba los clics del canvas— que
corrigió `f2710a1` en el submódulo, con el puntero `912a2c4` en el padre; la suite de pestaña del agente
quedó en 215/0. La vista previa final es `8b1e471d…`, verificada por HTTP en `127.0.0.1:8801`.

**Fronteras de este `§`.** La capa `wasm` de `tools/entorno.sh` falla por límites conocidos del shim (sin
`android.jar` montado, sin los `testdata` y con permisos de `os.CreateTemp` no POSIX), y **el gate no
ejecuta esa capa**, así que no la cubre. El CI remoto y el push de los commits nuevos siguen pendientes, y
el sitio publicado (`apkc-studio.surge.sh`) no se actualizó.

## 243. Las dos fronteras que quedan: `condy` y API < 21

Medido el 2026-11-23, sin suite, gate ni chromium. Son las dos fronteras que el mapa arrastraba desde
`§235` y el frente de dependencias, y la pregunta era si son **carencia** del fork o límite del oráculo y
del formato.

**`condy`: frontera del formato, no carencia.** El `d8` de build-tools 35.0.1 (R8 8.6.2) rechaza el
fixture `ConstantDynamic` con rc 1 en `min-api` 21, 26 y 34, con `Unsupported dynamic constant (runtime
provided bootstrap method)` y `const-dynamic are not supported at any API level known by the compiler`.
Nuestro rechazo está en `apkc/classreader.go:590` (el `cpDynamic` del `refuseOutsideSubset`, llamado
desde `apkc/classreader.go:187`): DEX no tiene constante dinámica y el emisor Go no la produce. Se
reabriría solo con un `.class` de `jar/` que la use, y entonces el oráculo tendría que ser la JVM o
`app_process`, no `d8`.

**API < 21: el runtime no abre un segundo dex.** Con 70 000 métodos, `d8 --min-api 19` da `Cannot fit
requested classes in a single dex file … Try supplying a main-dex list` y rc 1, mientras que `min-api 21`
sin banderas parte solo en 2 dex. Con `--main-dex-rules "-keep class Big0 { *; }"` a `min-api` 19 sale
rc 0: `classes.dex` con `Big0`..`Big8` (63 000 `method_id`) y `classes2.dex` con `Big9`, y `dexdump -f`
lee los dos. `--main-dex-list` con `.class` está retirado en R8 8.6.2. En el fork no hay banderas de
main-dex (`apkc/build.go:924`), `splitDeps := api >= 21` (`apkc/build.go:282`), el reparto es voraz
(`apkc/reparto.go:90`), no hay `androidx.multidex` en el árbol y `apkc/manifest.go:91-99` no lee
`android:name`. Las vías serían: (1) fijar el main-dex dentro del reparto, (2) importar el AAR desde
disco, (3) enganchar `Application.attachBaseContext`. Lo que bloquea la verificación en runtime es que no
hay dispositivo API ≤ 20: hoy lo medible es el contrato de bytes.

**Fronteras de este `§`.** No se corrió suite Go, gate ni chromium; no hay `.aab` real con `min-api` < 21;
y el desborde se midió con un fixture sintético, porque dexar el `android.jar` agotó la RAM.

**Cita corregida.** `§235` decía `apkc/classreader.go:451` para este rechazo; la línea vigente es
`:590`, y ya está cambiada allí. El comentario de `apkc/indy_condy_test.go:11` sigue nombrando la antigua
y no se toca: la cita buena es la de este `§`.

## 244. Los campos de locales y el null de un arreglo: las dos falsificaciones

Medido el 2026-11-23 sobre `8d25b64`, con el frente en `9422cc9` (`fix(java): resuelve campos de
locales como valor`), los interruptores en `6aa7808` (`refactor(java): interruptores de las dos ramas
del campo de local`) y los tests en `96f5a1f` (`test(java): falsifica los campos de locales y el null
de arreglo`). El crítico aprobó el arreglo con una condición —le faltaban las dos falsificaciones
prometidas—, y este `§` registra esa condición cerrada.

**Las dos ramas del arreglo.** `info.applicationInfo` como argumento (`App.java:28`) se estima desde el
tipo estático del receptor, sin emitir su `aload`/`getfield`; `info.splitSourceDirs == null`
(`App.java:34`) cuenta el `[` como referencia y branquea con `ifnonnull`, igual que javac. Los dos
interruptores son `var` de paquete encendidos por defecto: `resuelveCampoDeLocal`, declarado en
`apkc/javaemisor.go:5298` y leído una sola vez en `:5371`, y `comparaArregloConNulo`, declarado en
`:6305` y leído una sola vez en `:6504`. Encendidos, las dos ramas se comportan como en `9422cc9`;
apagados, cada rama vuelve al rechazo de antes.

**Las citas de línea, sobre el árbol final.** El frente daba `:4467`/`:4503` y `:4955`/`:5065`, que son
las líneas de `96f5a1f`: los cuatro commits posteriores (`2b44c94`, `0ee8200`, `e05b646`, `8d25b64`) las
corrieron a las de arriba, y esas son las que hay que leer hoy.

**Falsificación 1, el campo de la local.** Con `resuelveCampoDeLocal` apagado, el fuente que con la rama
encendida compila se rechaza con el mensaje que vio el usuario, literal:

```
Argumento.java: linea 6: linea 9: linea 10: linea 10, columna 23: al estimar el tipo de info.applicationInfo, el tipo info.applicationInfo no se resuelve: ni en el fuente, ni en su paquete, ni en los imports, ni en java.lang, ni en el classpath (android.jar ni jar/)
```

**Falsificación 2, el null del arreglo.** Con `comparaArregloConNulo` apagado:

```
Nulo.java: linea 5: linea 6: linea 7: comparar con null algo que no es una referencia ([Ljava/lang/String;) no esta medido
```

**Meta-falsificación.** Con los interruptores inertes —las dos condiciones forzadas a `false` en una
copia de `javaemisor.go` montada con `go test -overlay`, sin tocar el árbol— los dos tests de
falsificación fallan, en vez de pasar por el camino del caso verde, con:

```
con la rama apagada el fuente fue aceptado: la falsificación no falsifica
```

**Los casos verdes.** `TestCampoDeLocalComoArgumentoContraJavac17` y
`TestArregloComparadoConNullContraJavac17` comparan el dex **byte a byte** con el de javac 17
(`--release 8 -g:lines,source`) sobre las fuentes de `App.java:28` y `:34`; el caso completo compila
`src/share/apps/util/App.java` y `Apk.java` del zip de ApkShare y compara la clase `App` sola.

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  JAVAC17=/home/leonisauro/opt/jdk17/bin/javac GOMAXPROCS=2 GOGC=50 \
  go test -p=1 -count=1 \
  -run 'TestCampoDeLocalComoArgumentoContraJavac17|TestArregloComparadoConNullContraJavac17|TestCampoDeLocalFalsificacion|TestArregloNuloFalsificacion' -v .
```

**Resultado**: los cuatro en `PASS`, `ok … 1.923s`, con los dos mensajes literales en el log
(`apkc/javaemisor_campo_local_test.go:265` y `:301`). El frente dejó además la corrida enfocada de nueve
tests en la raíz (`ok … 1.883s`) y la suite del árbol (`ok … 98.134s`), y los críticos corrieron tests
enfocados propios (`ok … 2.937s` y `ok … 1.732s`); esas tres cifras no se re-corrieron aquí.

**La re-auditoría.** Sin hallazgos: interruptores reales —declarados y leídos una vez—, encendidos por
defecto sin cambiar lo que el emisor ya escribía; los dos tests devuelven el interruptor antes de
terminar y no usan `t.Parallel`, así que no queda estado sucio para el test siguiente.

**Cita cruzada.** El comentario de `apkc/javaemisor.go:5354` y la cabecera de
`apkc/javaemisor_campo_local_test.go:3` nombran `§244`, que es esta entrada, tras corregirse las líneas
viejas en `c81a0c8` y `131e701`; el frente es este `§244` y el código no se toca.

**Fronteras de este `§`.** No se corrió gate, chromium, red ni publicación: los cuatro tests y la
meta-falsificación son lo único medido en esta entrada, y las tres corridas del frente se citan sin
repetir. No hay medición de runtime en ART para estas dos ramas: el oráculo es javac 17 más el dex byte
a byte.

## 245. El condicional promovido no duplica su frontera de unión

Medido el 2026-11-23 sobre `d315a89` (`fix(dex): no duplica la frontera del condicional promovido`). El
origen es la reconciliación de puntos de unión: un borde de vuelta a una cabecera con **otra altura de
pila** no tiene plan que igualar, así que `reconciliaAntesDeSaltar` no hace `fijaPila` y lo anota como
`fronteraUnion` en vez de abortar el método. La forma larga de un condicional —`if-*` promovido a
`if` invertido más `goto/32`— pasa por esa función **dos veces**, una desde `escribeBranquia` y otra
desde `salto32`; con una altura distinta la segunda veía la misma llegada y el par (origen, destino)
quedaba anotado dos veces.

**El arreglo.** `anotaFrontera` (`apkc/dexunion.go:71`) apunta la llegada solo si no está ya: compara la
estructura entera —`pc`, `destino`, `cabecera` y `borde`—, no solo el par, para no descartar una llegada
del mismo `pc` a **otro** bloque con otra altura, que sí es un camino distinto. Los dos sitios de
anotación pasan por ella: `apkc/dexunion.go:314` (la reconciliación del punto de unión) y
`apkc/dexunion.go:359` (el borde de vuelta). Lo que se escribe en el dex no cambia: la frontera es la
anotación del camino que conserva su reparto, y con una sola copia describe lo mismo que con dos.

**El test.** `TestElCondicionalLargoNoDuplicaLaFrontera` (`apkc/dexunion_test.go:357`) monta el cuerpo
mínimo con la cabecera en `pc 3` (canon de un valor) y el borde de vuelta en `pc 7`, que llega con 0 tras
consumir la comparación, fuerza la promoción del `if_icmplt` con `promover: map[int]byte{7:
promocionCond32}` —para no montar un método de 32 KB— y exige **una** sola frontera para el par
`pc 7 -> pc 3`. El commit documenta que antes del arreglo salían dos. La reproducción es la del propio
test, que no necesita SDK:

```sh
cd apkc && go test -run TestElCondicionalLargoNoDuplicaLaFrontera -v .
```

**Fronteras de este `§`.** No se re-ejecutó el test en esta entrada: la medición que se cita es la del
commit y la que el test deja escrita, no una corrida propia. La deduplicación solo toca la lista de
fronteras anotadas, y el gate de `§242` corrió sobre un árbol posterior (`912a2c4`) que ya lleva este
commit. No se midió en ART un caso que dependa de la lista de fronteras: el oráculo es el test unitario
del traductor.

## 246. El `final` de una declaración local

Medido el 2026-11-23 sobre `9cddc37` (`fix(java): admite final en declaraciones locales`). Lo abrió la
línea 137 de `MainActivity.java` del zip de ApkShare, `final Dialog dialog = new Dialog(this);`, que
paraba el `check` con `"final" no es una expresion que este parser reconozca`.

**Por qué se paraba.** En una declaración local el modificador solo lo veía `pareceDeclaracion`, que lo
saltaba para decidir; el tipo se leía después con `tipoConArgumentos`, que empezaba en la palabra clave
y no la consumía. El guardia de la sentencia (`apkc/javafuente.go:2270`) tampoco dejaba entrar un
`final`: solo abrían tipo un identificador o un nombre de tipo, así que la sentencia caía en la ruta de
la expresión y el parser rechazaba.

**El arreglo.** `tipoArgumento` gana `esFinal` (`apkc/javafuente.go:525`), que consume `final` antes del
tipo dentro de `tipoConArgumentos` (`:1717`); la sentencia acepta `final` como apertura de tipo y pasa
`decl.esFinal` al `jdecl` (`:2288`). El `final` **no viaja al descriptor ni a la `Signature`**: `crudo()`
lo ignora, y `esFinal` solo lo usa la sentencia. El interruptor de la sonda es
`leeFinalEnDeclaracion` (`apkc/javafuente.go:1652`, encendido en `:1656`).

**El caso verde.** `TestFinalEnDeclaracionLocalContraJavac17` (`apkc/javaemisor_final_local_test.go:70`)
compila tres declaraciones locales con `final` —un primitivo con inicializador no literal, un
`ArrayList<>` y un `Dialog` sobre el `this` de la línea 137— y compara `max_stack`/`locals`, la
LineNumberTable y el dex **byte a byte** con javac 17 `--release 8 -g:lines,source`. Además comprueba en
los dos `.class` que no hay `LocalVariableTable` ni `LocalVariableTypeTable` (`:104-115`): con
`-g:lines,source` javac no escribe tabla de variables, así que el `final` no deja huella y la ruta nueva
tampoco puede inventarla.

**La falsificación.** `TestFinalLocalFalsificacion` (`:149`) apaga `leeFinalEnDeclaracion` sobre un
fuente que lleva la línea 137 **verbatim** en la línea 137 (`fuenteMainActivityLinea137`, con las de
relleno como comentarios dentro del método) y con el `Dialog` en la misma unidad, para que el único
motivo posible de fallo sea el `final`. Devuelve el mensaje literal del usuario:

```
src/share/apps/util/MainActivity.java: linea 137: "final" no es una expresion que este parser reconozca
```

**El crítico.** Aprobó sin ningún `CONFIRMADO`: comprobó que `esFinal` no llega al descriptor ni a la
`Signature`, que no rompe parámetros, campos, el `for` con dos puntos ni los comodines, y que la
falsificación devuelve el literal. La suite y el gate de este frente los dio la raíz y no se re-ejecutan
aquí; el gate que lo cubre es el de `§242`, sobre un árbol posterior. El comando de la punta es:

```sh
cd apkc && go test -run 'TestFinalEnDeclaracionLocalContraJavac17|TestFinalLocalFalsificacion' -v .
```

**Fronteras de este `§`.** El oráculo es javac 17 más el dex byte a byte: como `final` no deja huella en
el `.class`, no hay un oráculo de runtime que añada nada. El `final` de un campo y el de un parámetro ya
los leía el bucle de modificadores y no cambian. Pasar la línea 137 no cierra ApkShare: siguen
`MainActivity.java:218` y `AppAdapter.java:21/29/33`.

## 247. La firma de `--intercambiar` con tipos de referencia en el proto

Medido el 2026-11-23 sobre `8d25b64` (`fix(preview): parsea firmas con tipos de referencia en el proto`).
El evento de intercambio se partía buscando el **último** `;` de la firma (`strings.LastIndex(espec,
";")`), con la idea de que el nombre de la clase no lleva `;` y sus barras de paquete quedan dentro. Con
un proto que lleva una referencia —`Lcom/hola/Saludo;/suma(Lcom/hola/Saludo;)I`— el último `;` cae
**dentro del parámetro**, la clase sale truncada y la cola ya no empieza por `/`: la forma con referencia
no se podía usar.

**El arreglo.** `parteFirmaDeIntercambio` (`apkc/preview.go:490`) parte con la gramática del descriptor
DEX: exige que la firma empiece por `L`, cierra la clase en su **primer** `;` (`strings.IndexByte`) y lo
que sigue es o nada —la clase entera—, o `/metodo` sin proto, o `/metodo(proto)retorno`. El proto admite
referencias y arreglos porque sus `;` y sus `[` vienen después del que cerró la clase; el retorno se
valida con `esTipoDeRetorno` (`apkc/preview.go:539`). `eventoDeIntercambio` (`apkc/preview.go:459`) solo
lee el dex y rellena el evento.

**Las tres formas**, con el evento que deja cada una, en `TestLaFirmaDeIntercambioAceptaLasTresFormas`
(`apkc/previewfirma_test.go:42`): `Lcom/hola/Saludo;` (clase entera), `Lcom/hola/Saludo;/suma` (método
sin proto) y `Lcom/hola/Saludo;/suma(Lcom/hola/Saludo;)I` (método con proto). El mismo test comprueba que
`([I[[Lcom/hola/Saludo;)V` se parte sin confundir el troceo y que siete firmas malas se rechazan: sin
descriptor, descriptor sin `;`, `/` sin método, proto sin nombre, proto sin retorno, retorno que no es
descriptor y barra fuera del descriptor. `TestLaFirmaConReferenciaEnElProtoIntercambiaElMetodoVivo`
(`:102`) encadena el parseo real con el swap real: el proto con `;` dentro llega entero a
`intercambiaMetodo` y el cuerpo nuevo responde sobre el objeto vivo con su campo de antes.

**La falsificación** es el troceo anterior: con `Lcom/hola/Saludo;/suma(Lcom/hola/Saludo;)I` la cola que
veía el `LastIndex` era `)I` sin el `/` inicial, y el rechazo es el que el frente registró:

```
tras Lcom/hola/Saludo;/suma(Lcom/hola/Saludo; se esperaba /metodo(proto), no ")I"
```

**El comando y la suite.** La punta se reproduce con:

```sh
cd apkc && go test -run 'TestLaFirmaDeIntercambioAceptaLasTresFormas|TestLaFirmaConReferenciaEnElProtoIntercambiaElMetodoVivo' -v .
```

El constructor dio la suite del árbol en `ok … 103.370s`; no se re-ejecutó aquí.

**Fronteras de este `§`.** Solo se midió el parseo de la firma y su unión con el swap del intérprete: no
hay gate, chromium ni publicación en esta entrada. El comentario de
`studio/prueba-navegador.js:522-524` sigue diciendo que el parser corta por el último `;` y usa por eso
la forma de clase entera; es una nota vieja y la cita buena es este `§` (el código de `studio/` no se
toca).

## 248. El swap por nombre sin proto, y el proto obligatorio si hay sobrecargas

Medido el 2026-11-23 sobre `7200bc8` (`fix(preview): resuelve por nombre el swap sin proto y exige proto
si hay sobrecargas`). La forma `clase/metodo` del flag dejaba `Proto` vacío y `metodoDeClase` comparaba
`descDeProto(ref.proto) != proto` con `proto` vacío, así que **nunca** encontraba el método: el nombre
solo servía si además se escribía el proto.

**El arreglo.** `metodoDeClase` (`apkc/previewhotswap.go:48`) filtra por nombre y trata el proto como
opcional. Tres desenlaces: sin candidatos, el error de siempre con el proto pedido; con **más de uno**,
rechazo que enumera los protos vistos y pide el completo —`… pide el proto completo, como
Saludo.valor()I`, con `strings.Join` de los descriptores—; con uno solo, se toma ese y el
`metodoDalvik` lleva el `descDeProto(ref.proto)` **resuelto**, no el vacío del flag. Con el proto ya
resuelto, la clave `nombre+proto` del intérprete (`apkc/previewinterprete.go:2700`) encuentra el método
y el `invoke` del programa vivo se engancha al cuerpo nuevo.

**La resolución por nombre.** `TestLaFirmaSinProtoResuelvePorNombre` (`apkc/previewfirma_test.go:193`)
deja `valor()` en 102 tras dos `sube()`, intercambia con `Lcom/hola/Saludo;/valor` —sin proto— y
comprueba que el resultado **dice** `()I` y que el programa responde 202: el nombre resolvió y el cuerpo
nuevo se ejecutó sobre el mismo objeto, con su `n`.

**El rechazo con sobrecargas.** `TestLaFirmaSinProtoConSobrecargasPideElProto` (`:267`) usa una clase con
`valor()` y `valor(int)`: el swap sin proto falla y el mensaje lleva `sobrecargado`, `()I`, `(I)I` y
`proto completo`; la sesión **no se toca** —`valor()` sigue en 100— y con `Lcom/hola/Saludo;/valor()I` el
mismo dex entra y deja 200. Que el rechazo no tape un fallo de lectura es la segunda mitad del test.

**El estado.** Cinco tests en verde, `ok … 0.070s`, y la suite del árbol en `ok … 120.206s`, ambas dadas
por el constructor; el crítico lo aprobó formalmente. La punta se reproduce con las cuatro pruebas de
firma, que comparten fichero:

```sh
cd apkc && go test -run 'TestLaFirmaDeIntercambioAceptaLasTresFormas|TestLaFirmaConReferenciaEnElProtoIntercambiaElMetodoVivo|TestLaFirmaSinProtoResuelvePorNombre|TestLaFirmaSinProtoConSobrecargasPideElProto' -v .
```

**Fronteras de este `§`.** La resolución por nombre es del dex **nuevo** del swap: no cambia la forma de
intercambiar la clase entera ni el `--intercambiar` con proto, que ya iban por su ruta. No se re-ejecutó
nada aquí, no hubo gate ni chromium, y el runtime medido es el intérprete del `preview`, no ART.

## 249. La interfaz del intercambio de métodos en el emulador

Medido el 2026-11-23 sobre `9a61a23` (`chore(studio): cierre del intercambio de metodos en el emulador`),
puntero a los dos commits del submódulo `3d79170` (`feat(emulador): interfaz para intercambiar metodos
con un dex`) y `36044e8` (`fix(emulador): registra el fallo del swap y serializa los drops`). Es la capa
de UI del hot swap; el comando (`--intercambiar`) y su parser son `§247` y `§248`.

**Lo que añade la interfaz.** Un campo de firma y un botón por panel —`previa-intercambio-firma` y
`previa-intercambiar`, con sus gemelos `vista-intercambio-firma` y `vista-intercambiar`— más el arrastre
de un `.dex`, que se reconoce por la extensión o por el tipo (`esArchivoDex`, porque en Linux llega como
`application/octet-stream`) y se guarda en una zona aislada con un fichero por swap
(`RUTA_DEX_SOLTADO = "/previa-soltada/intercambio-"`), para que el guion, que se repite entero en cada
invocación, no lea el dex nuevo en los pasos viejos.

**Las puertas antes del intérprete.** `motivoFirmaInvalida` exige el descriptor de la clase y deja el
método como opcional, que es la forma legítima de cambiar la clase entera. `motivoDexInvalido` rechaza
el fichero vacío, el que no empieza por el mágico `dex\n`, el que no llega a la cabecera de 112 bytes, el
que declara una versión que no tiene la forma `0dd` y el que declara más bytes que el fichero; un dex que
pasa el mágico pero está roto por dentro lo rechaza después el propio `preview`, que nombra el defecto.

**El registro.** `textoDeIntercambio` (`studio/previa-panel.js:632`) pinta
`clase/metodo/proto · existía: sí|no`, con los cuatro datos que publica `ejecucion.intercambios`; un
objeto sin clase ni método no se pinta. Ese `existía` es lo que distingue «reemplacé el cuerpo» de
«añadí un método».

**Los dos defectos que cerró `36044e8`.** La guarda del doble drop se toma **antes** de ejecutar, no
dentro de `ejecutaGuion`, así que dos drops seguidos no se pisan el fichero ni el guion; y el `catch`
del transporte llama a `alFallar`, que deja en el registro y en la línea del emulador
`no se intercambió <nombre>: <motivo>` en vez de quedarse en un error mudo.

**La evidencia de la pestaña.** `studio/prueba-navegador.js:525-578` conduce el bloque del swap sobre la
pestaña: presencia de los tres controles, el evento `intercambiar` con su firma dentro del guion, la
ruta `/previa-soltada/intercambio-\d+\.dex` y el registro con clase, método, proto y `existía`
(`:876-883`); y `:893-911` cubre el dex roto, el doble drop y el transporte caído con sus mensajes. El
crítico aprobó con el bloque swap ejecutado sobre wasm real. Este `§` no corrió chromium ni el gate.

**Fronteras de este `§`.** Es una medición de UI sobre el guion del intérprete, no del compilador: un
swap que el intérprete rechace por el cuerpo sale por el mismo registro, y no hay medición de ART. La
nota de `studio/prueba-navegador.js:522-524`, que atribuye al parser el corte por el último `;`, quedó
vieja tras `§247`; el código no se toca.

## 250. Las condiciones cerradas del main-dex pre-21

Medido el 2026-11-23 sobre `b805733` (`test(build): mide el camino d8 pre-21 y el arrastre de la lambda
al dex primario`). Cierra las dos condiciones de bytes que el frente de dependencias dejó abiertas: que
el dex primario lleve las clases fijas **también** cuando el dexado lo hace `javac` + `d8` con
`minSdkVersion` 19, y que la clase sintética de una lambda viaje con su dueña cuando la dueña está fija.
El arranque real en un Dalvik de API ≤ 20, el AAR de `androidx.multidex` y
`Application.attachBaseContext` siguen fuera.

**El camino javac + d8.** `TestCompilaAppPre21PorD8FijaElDexPrimario`
(`apkc/maindex_pre21_test.go:383`) monta el proyecto con `minSdkVersion="19"`, 70 clases de 1000
métodos en `jar/a-grande.jar` y el cargador en `jar/z-multidex.jar` —el nombre del jar decide el orden,
y el cargador va detrás para que la falsificación se vea—, compila por el camino por defecto y comprueba
que `build/maindex.pro` lleva las dos reglas `-keep class grande.Arranque { *; }` y
`-keep class androidx.multidex.MultiDex { *; }`; que `classes.dex` tiene las dos fijas y **no** tiene
`Lgrande/C0;`; que `classes2.dex` lleva el resto; y que `dexdump -f` lee los dos ficheros y ve en cada
uno lo que debe. La corrida del constructor dejó **8 clases en `classes.dex` y 65 en `classes2.dex`**
(es la cifra del `t.Logf` de `:494`; el test no fija cuántas son, solo su pertenencia).

**El arrastre de la lambda.** `arrastraLambdasDeFijas` (`apkc/reparto.go:189`, leído en `:198`) mete la
sintética de una clase fija en el dex primario. `TestRepartoArrastraLaLambdaDeSuClaseFija`
(`apkc/maindex_pre21_test.go:32`) comprueba con el arrastre encendido que
`Lcom/hola/Arranque$$Lambda$1;` encabeza el primer dex junto a su dueña, y con el interruptor apagado
—restaurado por `t.Cleanup`— que la sintética cae en `classes2.dex` mientras la dueña sigue en
`classes.dex`: la falsificación falla si el arrastre no hace nada. `TestRepartoFijaClasesEnElDexPrimario`
(`:77`) hace lo mismo con las fijas en el reparto puro y `TestRepartoFijasQueNoCabenEsUnRechazo`
(`:139`) cubre el caso en que las fijas no caben en el techo de 65 536 referencias.

**El estado.** El crítico aprobó. La punta, con SDK y los oráculos, se reproduce en:

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  go test -run 'TestRepartoArrastraLaLambdaDeSuClaseFija|TestRepartoFijaClasesEnElDexPrimario|TestRepartoFijasQueNoCabenEsUnRechazo|TestCompilaAppPre21FijaElDexPrimario|TestCompilaAppPre21PorD8FijaElDexPrimario' -v .
```

**Fronteras de este `§`.** No se corrió aquí ni el test ni el gate: las cifras 8/65 y el `ok` son del
constructor. El oráculo es de **bytes** (`dexdump`, `apksigner`, `zipalign`), no de ejecución: no hay
dispositivo API ≤ 20 donde arrancar la app, y sin el AAR de `androidx.multidex` el cargador que abre
`classes2.dex` no está en el árbol.

## 251. La lambda de un parámetro en la fuente

Medido el 2026-11-23 sobre `0ee8200` (`fix(java): admite lambdas de un parametro en la fuente`) y
`d0a0492` (`fix(java): numera por clase los implementadores de lambda`). Lo abrió la línea 156 de
`MainActivity.java` del zip de ApkShare, `ApkAdapter adapter = new ApkAdapter(this, apks, apk ->
shareApp(apk));`, que paraba el `check` con `linea 156: ">" no es una expresion que este parser
reconozca`: el `->` no existía como símbolo y la lambda no era una expresión.

**La lambda, sin tipo en la expresión.** El tipo funcional no viaja en la expresión: lo pone el
parámetro formal de la llamada (JLS 15.27.3), así que con una lambda en los argumentos la sobrecarga del
constructor se elige mirando la declaración del fuente —`Consumer<Apk>`, con su `Signature`— y no un
descriptor de la expresión. Se escribe el implementador `lambda$metodo$N` (privado y sintético, de
instancia con `REF_invokeSpecial` cuando captura `this`), un `invokedynamic` con
`LambdaMetafactory.metafactory` que `lambda.go` ya desugra al dex, y el análisis de marcos del emisor
aprende el efecto del `0xba`. El cuerpo nuevo vive en `apkc/lambda_fuente.go`.

**El oráculo de la 156.** `TestLambdaDeUnParametroContraJavac17`
(`apkc/javaemisor_lambda_fuente_test.go:93`) compara con javac 17 `--release 8 -g:lines,source` los
métodos, `max_stack`/`locals` y la LineNumberTable, y el dex **byte a byte**; comprueba que el
implementador es `lambda$openAppDetailsDialog$0` con descriptor `(Lshare/apps/util/Apk;)V`, flags
`0x1002` y una sola fila de `BootstrapMethods`. El `t.Logf` de `:140` deja el tamaño del dex de la
clase en **2 116 B** en las dos partes, que es la cifra que reportó el frente.

**El contador es de la clase.** Dos sobrecargas del mismo nombre con una lambda cada una escribían dos
`method_info` idénticos `lambda$f$0(Ljava/lang/Object;)V`, un ClassFile inválido; lo destapó el crítico
con su propia sonda (`TestCriticoColisionImplementadoresLambda`, que no está en el árbol). La causa era
que `preparaMetodo` reiniciaba `lambdaN` en cada método: ahora el contador vive toda la emisión de la
clase (`apkc/javaemisor.go:2426`, con el porqué en `:2418`) y javac numera igual —con `f(int)`,
`f(String)` y `g(int)` escribe `lambda$f$0`, `lambda$f$1` y `lambda$g$2`, medido con `javap -p`—. El
dex seguía sin coincidir por cuatro bytes y no era el contador: javac escribe los implementadores en el
orden **inverso** al que los crea y el dex copia el orden del ClassFile, así que se emiten en ese orden
(`apkc/javaemisor.go:8600`) y el dex de las dos sobrecargas sale idéntico a javac 17, checksum y firma
incluidos.

**La vuelta que faltaba al comparador.** `comparaMetodos` solo iteraba los métodos de javac, así que un
implementador sobrante pasaba callado; ahora un método que escribimos y javac no es un fallo
(`apkc/javaemisor_generico_test.go:231`) y `TestComparaMetodosVeUnMetodoDeMas` (`:240`) lo falsifica
quitándole un método al de javac. `TestDosSobrecargasNoCompartenImplementadorDeLambda`
(`apkc/javaemisor_lambda_fuente_test.go:180`) exige además que no haya `nombre+descriptor` repetido y
que salgan los dos `lambda$f$0`/`lambda$f$1`.

**El estado.** El crítico rechazó primero y aprobó después de cerrar dos `CONFIRMADO`. La punta:

```sh
cd apkc && go test -run 'TestLambdaDeUnParametroContraJavac17|TestDosSobrecargasNoCompartenImplementadorDeLambda|TestLaLambdaDelFuenteSeDesugra|TestLaLambdaConBloqueSeRechaza|TestComparaMetodosVeUnMetodoDeMas' -v .
```

**Fronteras de este `§`.** Capturar una local, la lambda con cuerpo de bloque y la lambda anidada se
rechazan **por nombre** (`TestLaLambdaConBloqueSeRechaza`, `:283`); cómo se mezclan los implementadores
con los `access$` de una anónima en la misma clase no está medido —hoy salen detrás—, y el frente
comprobó que un fuente con `access$` difiere igual sin lambda, así que no hay defecto atribuible. No se
re-ejecutó nada aquí ni hubo gate. ApkShare pasó la 156 y se paró en la 187, que abre `§252`.

## 252. La asignación como expresión, y la declaración sin inicializador

Medido el 2026-11-23 sobre `91657b4` (`fix(java): admite asignaciones como expresion`) y `0936c32`
(`test(java): mide la asignación anidada y la ancha contra javac`). Lo abrió la línea 187 de
`MainActivity.java`, `while ((length = in.read(buffer)) > 0)`, que paraba el `check` con `linea 187: se
esperaba ")" y hay "="`. Es una asignación como expresión (JLS 15.26.1), el operador de menor precedencia
y asociativo a la derecha.

**El nodo y el emisor.** `jasignExp` (`apkc/javafuente.go:212-223`) guarda destino, valor y posición; el
`=` se lee en `expresion()` después del operando izquierdo, que tiene que ser un acceso o un índice. En
la sentencia y en el paso del `for` el nodo se baja al `jasign` de siempre para no dejar el valor en la
pila, así que lo ya medido no cambia. `asignacionValor` (`apkc/javaemisor.go:7229`) escribe el valor, lo
duplica —`dup`, o `dup2` en un ancho— y lo guarda con `guardaDe` (`:2895`), que es lo que hace javac:
`aload_1; aload_2; invokevirtual read([B)I; dup; istore 4; iconst_0; if_icmple`. Sobre un campo o un
elemento de arreglo se rechaza por nombre, porque el duplicado pide `dup_x1`/`dup_x2` con el receptor
debajo. De paso, una declaración sin inicializador ya no marca fila en la LineNumberTable: `int length;`
no escribe un byte y javac no le pone entrada.

**El anidamiento y el ancho.** `0936c32` cerró las dos condiciones que faltaban. `a = b = c` anida por la
misma ruta y el emisor duplica dos veces el mismo valor (`dup; istore b; dup; istore a`), con el dex
entero byte a byte contra javac 17 (`TestAsignacionAnidadaEnCondicionContraJavac17`,
`apkc/javaemisor_asignacion_valor_test.go:136`). `long x; while ((x = f()) > 0)` lleva `dup2` y `lstore`
y `lcmp` antes de la branquia; para elegir ese camino ancho antes de emitir hacía falta el tipo del
operando, y se añadió `tipoPrevisto` para `jasignExp` (`apkc/javaemisor.go:5954` y la rama en `:6016`),
que devuelve el descriptor del local —sin él el caso se rechazaba con «solo se midieron enteros»— y el
dex sale también byte a byte (`:182`).

**El caso del zip.** `TestAsignacionComoValorContraJavac17` (`:91`) compila una copia mínima con la línea
187 y compara; `TestCopiaDelBloqueDeLaLinea187` (`:210`) compara el bloque con su `try`/`catch`
instrucción a instrucción —opcode a opcode, sin los índices del pool— porque ese dex ya traía 12 bytes de
más en el primer `annotations_directory_item` antes de este cambio, y un control sin asignación en la
condición mide los mismos 12. `TestAsignacionComoValorSoloLocal` (`:310`) fija el rechazo del destino que
no es local.

**La falsificación y el avance.** Con la ruta de `expresion()` apagada en una copia y el binario fuera del
repo, el árbol integrado devuelve el error literal del usuario, `linea 187: se esperaba ")" y hay "="`;
sin apagar nada, ApkShare pasa la 187 y se para en `linea 218: se esperaba ";" y hay "builder"`
(`AlertDialog.Builder builder = ...`). El crítico aprobó.

```sh
cd apkc && go test -run 'TestAsignacionComoValorContraJavac17|TestAsignacionAnidadaEnCondicionContraJavac17|TestAsignacionAnchaComoValorContraJavac17|TestCopiaDelBloqueDeLaLinea187|TestAsignacionComoValorSoloLocal' -v .
```

**Fronteras de este `§`.** La asignación como valor sobre un campo o un elemento de arreglo está
**rechazada por nombre**, no medida: falta el `dup_x1`/`dup2_x1` con el receptor debajo. No se re-ejecutó
nada aquí, no hubo gate ni chromium, y el oráculo es javac 17 más el dex byte a byte (o instrucción a
instrucción en el bloque del zip).

## 253. El cierre de la ronda de auditorías: firmas, cotas, `check`, pestaña y compilador

Medido el 2026-11-23 sobre la tanda que va de `b9ed7ff` a `6157786`, más el commit `4514231` del
submódulo `studio/` (puntero en `7d55e18`). La ronda abrió cinco críticas —compilador, pestaña,
seguridad, dependencias y documentación—; cuatro volvieron `RECHAZAR` y una `APROBAR CON
CONDICIONES`, y de ellas salieron los `CONFIRMADO` que se cierran aquí. Lo que sigue no se
re-ejecutó al escribir este `§`: cada bloque cita el comando del frente que lo midió y el commit que
lo integra, y los que quedaron abiertos se dicen como tales.

**Seguridad (`e837d46`).** Cuatro hallazgos, cada uno con su PoC armada en el test con `archive/zip`
y `bytes` (no se versiona ningún binario, regla 2). El bloque de firmas de 16 B anunciaba un tamaño
que no cubre ni su propia magia: `largo-24` desbordaba y el verificador moría con `panic:
makeslice`, rc=2 y traza. Ahora el tamaño se mira antes de reservar y el veredicto es «el bloque de
firmas no tiene tamaño válido», rc=1, sin pánico; se cubren 16, 20 y 23 bytes y un bloque de 1 TiB
(`apkc/verificar_hostil_test.go:67`). El barrido de la v1 se quedaba con el último `.SF` y el último
`.RSA`, así que un paquete con una firma buena y un segundo `.SF` salía `v1 ok`: ahora dos `.SF` o
dos `.RSA` se rechazan con «varios firmantes v1: no medido» (`apkc/verificar.go:258` y
`verificar_hostil_test.go:96`). Al re-firmar, `carryPlan` (`apkc/sign.go:312`) rechaza un nombre con
barra inicial o `..`, y `../../evil.txt` sale con su nombre (`apkc/sign_hostil_test.go:18`). Y el
descarte de una entrada `META-INF` que parece firma ya no es silencioso: `META-INF/services/x.MF`
avisa por `LogW` (`apkc/sign.go:327`, test `:43`). Además, `verificarApk` convierte cualquier pánico
en un error con el prefijo «paquete hostil:» (`apkc/verificar.go:81`).

**Residuos (`d66eb0d`, `072725b`, `acf6866`).** R2: el `.idsig` y el apk se leían enteros con
`os.ReadFile`, y un fichero de gigabytes agotaba la memoria del worker —un OOM de Go no es un pánico
y el `recover` no lo atrapa—. Ahora los dos tamaños se miran con `os.Stat` y la lectura lleva su
corte con `LimitReader`: 1 GiB para el apk en nativo (`apkc/v4verify.go:31`) y 256 MiB cuando
`runtime.GOARCH == "wasm"` desde `init` (`:39-42`); el tope del `.idsig` se deriva del tamaño del apk
y se aprieta solo (`:53`). R3: `FuzzVerificarApkNoMuere` daba por bueno cualquier error, así que
medía «no se cae el test» y no «no hay pánico»; ahora `falloDelPaquete` falla con el prefijo y una
sonda comprueba en cada corrida que el detector lo ve (`verificar_hostil_test.go:171` y `:226`). R5:
el prefijo vive en una sola fuente, `prefijoPaqueteHostil` (`apkc/verificar.go:81`), y
`TestLaSondaSigueAlPrefijoDeProduccion` (`verificar_hostil_test.go:253`) falsifica un literal
duplicado. R6: en wasm el tope se aplica antes de leer, con un apk disperso de 300 MiB y un `.idsig`
de 80 MiB (`TestVerificarV4RespetaLaCotaDeWasm`, `apkc/v4tope_wasm_test.go:49`); en wasm32/Node se
alcanzan **3 GiB de `heapSys`** con la RAM y el swap de un escritorio (`apkc/v4verify.go:33-38`),
pero los 256 MiB de un renderer móvil **siguen siendo HIPÓTESIS**: no están medidos en Chromium, y
por eso `verificar_hostil_test.go:13-15` deja escrito que la cota no se baja sin esa medición.

**`check` (`50d9213`, `b2973c9`).** Antes, `check` armaba el classpath con el primer `jar/` por orden
léxico y solo marcaba `emisor:true` si el fallo venía del emisor y el pedido ya se había emitido
(`apkc/check.go:416`); una clase definida en dos jars, dos fuentes con la misma clase, o un fuente
que redefinía una clase del jar salían `emisor:true` con cero errores. Ahora
`entradasDelClasspathEn` llama a `claseDuplicadaEntreJars` (`apkc/dependencias.go:77`), el registro
rechaza dos fuentes con la misma clase (`apkc/javaemisor.go:2275`) y contrasta cada nombre registrado
con el índice de los artefactos ya abiertos (`:2284`, `jarDe` en `:483`). El tercer caso se midió con
el jar real `android-support-v4.jar` y `android.support.v4.text.TextUtilsCompat`: sin la guarda,
`emisor:true` y cero errores; con ella, `emisor:false` y un mensaje que nombra clase, fuente y jar.
Tests: `TestCheckAbortaConDosJarsQueDefinenLaMismaClase`,
`TestCheckAbortaConAarYJarQueDefinenLaMismaClase`, `TestCheckRechazaDosFuentesConLaMismaClase` y
`TestCheckAbortaConUnFuenteQueRedefineUnaClaseDelJarReal` (este se salta con un mensaje si el jar no
está).

**Pestaña (`4514231` en `studio/`, puntero `7d55e18`).** Cinco defectos. El worker que no cargaba —un
404, un import roto, COOP/COEP ausente— o que se quedaba mudo dejaba `pedir` sin resolver y el velo
de carga encima: ahora escucha `error` y `messageerror`, cada petición lleva un vigilante de 20 s que
se reinicia con cada mensaje del worker y el fallo se recuerda hasta «Detener» (`studio/pestana.js:46`
y `:96`). El `Toast` contaba emisiones de `ejecucion`, así que un aviso repetido con el mismo texto no
reiniciaba la cuenta y se ocultaba a los 3 s del primero; ahora cuenta `toastFecha` por identidad del
modelo (`studio/previa-panel.js:701` y `:1808-1821`). Un drop con varios ficheros atendía solo a
`files[0]` y el resto desaparecía; ahora se atiende al primero y se dice qué se ignora, con su nombre
(`previa-panel.js:4208-4215`). Los `.catch(() => {})` de refresco, swap, imagen, historial,
diagnóstico y caché pasan por `falloSilencioso` (`studio/pestana.js:32`). Y la lámina de lectura del
inspector no captura el puntero (`studio/style.css:696`): `previa-seccion-consola` volvía a tapar el
clic del canvas que `§242` había destapado.

**Compilador (`b9ed7ff`, `159b6ea`, `7867884`, `3f8325e`).** `ejecutaCuerpo` copiaba cada argumento a
`regs[pos]` sin mirar el marco: con `insSize` 1 y tres argumentos desbordaba el slice y el `recover`
del intérprete lo disfrazaba de «dex malformado». Ahora `colocaArgumentos` cuenta las palabras y las
compara con `insSize` y `registersSize` antes del bucle, y el fallo es un error de contrato de llamada
con método, número de argumentos y palabras, sin pasar por el `recover` (`apkc/previewinterprete.go:2930`
y `:3003`); un `code_item` que escribe fuera del marco sigue siendo dex malformado. La asignación
anidada y la ancha ya se cerraron en `§252` (`0936c32`) y no se re-midieron aquí. El tipo cualificado
de `MainActivity.java:218`, `AlertDialog.Builder builder = new AlertDialog.Builder(this)`, se acepta
en `pareceDeclaracion` contando los segmentos unidos por puntos (`apkc/javafuente.go:2382`). El molde
`(App) apps.getAdapter().getItem(positon)` se estima sin emitir su receptor (`apkc/javaemisor.go:5304`
y `:5842`), que era el rechazo «el tipo de una expresion main.jcast no se puede estimar sin emitirla».
La escritura de un campo acepta un subtipo de referencia sin `checkcast`, como la subida de javac
(`cabeEn`, `:1730`, y el sitio de escritura `:4021`), y `if (llama())` o `if (b)` se branquean con
`ifeq` sin invertir (`condicionBranca`, `:6356`, con `opuestoDeSalto` en `:6335`). Los tres cambios
del emisor comparan el dex con javac 17 `--release 8 -g:lines,source`, byte a byte o instrucción a
instrucción, y las falsificaciones apagan el interruptor y devuelven el error literal del usuario.

**ApkShare avanza, no compila.** El `check --proyecto` del fuente real pasó la 156, la 187, la 218 y
`AppAdapter.java:21/29/33`, y el build se para ahora en `MainActivity.java:55`, **columna 44**: al
leer `apps`, el tipo no se resuelve porque es la captura de una local en posición de receptor dentro
de una anónima (`apkc/javafuente.go:2924`), y ese frente sigue en curso. El puente
`getItem:(I)Ljava/lang/Object;` del retorno covariante y el `check-cast` tras el `List.get` borrado,
que este `§` dejaba fuera, cerraron después en `ee5a372`: con la ruta del cast apagada el
verificador de la JVM rechaza `getView` con `VerifyError`, con la del puente apagada `getItem` sale
una sola vez, y el dex de `AppAdapter` queda byte a byte el de javac 17
`--release 8 -g:lines,source` (`apkc/javaemisor_appadapter_cast_test.go`). Ese `VerifyError` era el
de la JVM; en el aparato lo cerró `§255`, con ART verificando y ejecutando el mismo dex, y allí
quedaron cerrados además el dexer ancho (`f697c73`) y la clase íntegra, `getItemId` incluido, que
verifica y ejecuta. **No compila entero**: el build no se presenta como cierre, y el proyecto entero
sigue difiriendo de javac en el plegado `1|128` de `Apk.java`.

**Docs (`e2fe7f1`, `8bd46da`).** Los nueve defectos de la crítica: I1, I2 e I3 con su estado real de
cerradas; I4 hecha en nativo con el gate pendiente; I5 medida; J4 cerrada con el run `45062792689`
—J5, local, con el sitio pendiente, en el mismo plan (`docs/plan-android-jar-propio.md:4-8` y
`:93-98`)—; la UI del hot swap cerrada en `§249`; la numeración histórica con el hueco del `§78` y el
desorden 80/81 y 213/215, anotada sin renumerar cabeceras; y la fórmula con la letra griega delta —y
sus cifras 35/184— sustituida por `dpc`/`dlinea` (`docs/situacion.md:212`) y **36 comprobaciones en
`crear` y 229 en pestaña** (`docs/situacion.md:33`).

**WISP.** Se descartó como módulo por defecto: rompe la regla 4 y la auditoría de peticiones fuera
del origen, y por CDP no se ven los frames cifrados de un WebSocket. El usuario decidió abrirlo como
**módulo opcional opt-in, con aviso y dos gates**, y la forma elegida es un **módulo con TLS propio
en Go** —`crypto/tls` sobre un `net.Conn` que habla WISP, compilado a `GOOS=js`—, no un cliente en
JS: la investigación midió que JS puro no llega a TLS, porque el relay entrega el flujo en claro y
cualquier relay del camino queda en posición de hacer MITM, y que de 98 relays solo 54 contestan el
handshake. La spec es la **v1.2** (CC-BY-4.0, sin copia en el repo; se cita en
`docs/plan-deps-red.md:47-48`): paquetes de 5 B, un `CONTINUE` inicial y `CLOSE`. **En
implementación, núcleo en curso**: el gate por defecto sigue con **cero peticiones ajenas** y el
módulo solo se activa a mano.

**`deps-index` (`6157786` en el repo, `d70b8e7` en `studio/`).** El generador local
`studio/deps-index-local.mjs` produce un índice mínimo y reproducible **sin red** —coordenada,
fichero, bytes, `sha256`, `licencia` y `fecha`— con un jar sintético o uno real y siempre en un
directorio temporal; el panel muestra la licencia y dice «sin declarar» si falta. El frente corrió
`node studio/prueba.js` y `APKC_WASM=… node studio/prueba-pestana.js` con **216 comprobaciones bien,
0 mal** y **cero peticiones fuera del origen**; la falsificación del 404 quita el índice y el panel lo
dice en vez de inventar datos. El **sitio público no está actualizado** con el índice: falta la
publicación, que queda pendiente de orden (`studio/DEPS.md`, «Estado»).

**Fronteras de este `§`.** Un `.aar` real con `res/` y `lib/` sigue sin empaquetarlos: `classes.jar`
se dexa, pero los recursos y las bibliotecas nativas quedan fuera de la pieza a propósito
(`apkc/dependencias.go:12-14`). El opt-in de WISP queda en implementación, y el push, el CI
remoto y la publicación de `apkc-studio.surge.sh` siguen pendientes de orden. No se re-ejecutó nada
aquí, no hubo gate ni chromium; el oráculo de cada bloque es el que cita: los tests de paquete hostil,
javac 17 con el dex comparado y las comprobaciones de `studio/`.

```sh
cd apkc && go test -run 'TestVerificarRechazaBloqueDeFirmasConTamanoImposible|TestVerificarRechazaDosFirmantesV1|TestSignRechazaNombreQueSaleDelPaquete|TestSignAvisaAlDescartarUnaFirmaEscondida' -v .
cd apkc && go test -run 'TestVerificarV4NoLeeUnIdsigEnorme|TestVerificarV4NoLeeUnApkPorEncimaDelTope|TestTopeV4PorPlataforma|TestLaSondaSigueAlPrefijoDeProduccion|TestElDetectorDePanicosDelFuzz' -v .
cd apkc && go test -run 'TestCheckAbortaConDosJarsQueDefinenLaMismaClase|TestCheckAbortaConAarYJarQueDefinenLaMismaClase|TestCheckRechazaDosFuentesConLaMismaClase|TestCheckAbortaConUnFuenteQueRedefineUnaClaseDelJarReal' -v .
cd apkc && go test -run 'TestLaLlamadaConArgumentosDeMasSeRechazaPorContrato|TestTipoCualificadoLocalContraJavac17|TestMoldeDeArgumentoContraJavac17|TestAppAdapterDeApkShareCompilaContraJavac17|TestLaSubidaDeUnaReferenciaAlEscribirUnCampoDexaComoJavac17' -v .
cd studio && node prueba.js
```

## 254. WISP: la spec, el TLS que no llega en JS y los relays que responden

Medido el 2026-11-23 por el investigador `ses_df473bdbfffelKQp8juy4AI5Jp`. Este `§` transfiere su
salida y los comandos que la reproducen; no se re-ejecutó nada aquí: sin gate, sin chromium y sin
red desde este árbol. WISP es el proxy de sockets TCP/UDP sobre un WebSocket que `§253` deja como
módulo opt-in, no como camino por defecto; lo que sigue registra qué dice la spec, por qué un
cliente en JS puro no sirve para Maven y qué relays contestan.

**La spec, v1.2.** Fuente `github.com/MercuryWorkshop/wisp-protocol`, rama `v1`, **CC-BY-4.0**,
atribuida a **@ading2210**; no hay copia en el repo, como ya decía `§253`. Cada paquete es
`[type u8][streamID u32 LE][payload]` y todo va en **little-endian**. Los tipos:

| tipo | nombre | payload |
| --- | --- | --- |
| `0x01` | `CONNECT` | `[streamType u8 (1 = TCP)][dstPort u16 LE][hostname UTF-8...]` |
| `0x02` | `DATA` | los bytes del flujo |
| `0x03` | `CONTINUE` | `[bufferRemaining u32 LE]`, en **paquetes** |
| `0x04` | `CLOSE` | razón: `0x41`-`0x49` del servidor, `0x81` del cliente |

Dos trampas del contrato: el servidor manda un `CONTINUE` **inicial con `streamID` 0** que hay que
esperar antes de considerarse conectado, y la URL del endpoint **tiene que acabar en `/`**. El
`bufferPackets` de ese `CONTINUE` inicial se observó entre **128 y 2147483647**, así que **no se
asume 128**: el cliente lo lee y se gobierna por él. Todo lo anterior sale de leer los ficheros de
la rama `v1`, sin copia local.

**HTTPS por WISP: no viable con JS puro.** `curl http://repo1.maven.org/maven2/` responde un 301 a
`501-https-required.html`, así que Maven Central hoy exige TLS. Por un WISP real,
`wss://wisp.mercurywork.shop/wisp/`, `example.com:80` devuelve `HTTP/1.1 200 OK`: el passthrough de
TCP funciona. Pero `repo1.maven.org:443` devuelve **HTTP en claro** —414 B y ni un `ServerHello`—:
el flujo TLS viaja tal cual por el WebSocket, así que el relay puede leerlo y sustituirlo, o sea,
queda en posición de MITM. Con JS puro no hay forma de cerrar TLS de extremo a extremo sin que el
relay vea el contenido.

**La vía que queda: `crypto/tls` de Go sobre un `net.Conn` que habla WISP.** El módulo se compila a
`GOOS=js` y Go hace el handshake TLS encima del `net.Conn`, de modo que el relay solo transporta
bytes cifrados. Coste estimado **~2-3 MB más de wasm**, y las CAs hay que **embeberlas**, porque en
`js/wasm` no existe `SystemCertPool`. Es **HIPÓTESIS sin compilar** —no se midió el tamaño real ni
el handshake— y su núcleo está en implementación.

**Relays.** El gist `CompactAIOfficial/bedf67779c2afd8bbd0225328ac210c9` lista **98** entradas; un
handshake WISP real con Node 26 hizo contestar a **54 (27 hosts únicos)**. Las mejores latencias de
ese handshake, en ms:

| endpoint | ms |
| --- | --- |
| `wisp.mercurywork.shop/wisp/` | 400 |
| `terbiumon.top/wisp/` | 430 |
| `anura.pro/wisp/` | 447 |
| `lichology.com/wisp/` | 509 |
| `wisp.waved.site/w/` | 515 |
| `gointospace.app/wisp/` | 519 |

42 fallaron, 1 agotó el tiempo y 1 se anunció como **v2** mandando un `INFO` primero, que este
cliente no espera. Son **pruebas, no auditoría**: nadie revisó quién opera esos relays, y por eso el
`wisp-relays.json` y el aviso al usuario siguen pendientes.

**Licencias.** Spec **CC-BY-4.0**; `wisp-js` **LGPL-3.0**; `wisp-server-python` **AGPL-3.0**;
`epoxy-tls` **sin `LICENSE`**; `libcurl.js` sin verificar. Con un cliente propio solo queda la
atribución **CC-BY** a la spec.

**Fronteras de este `§`.** El módulo se implementó y sus gates pasan (§281, §§294-297). Quedan el
relay real, el push y la publicación; el sondeo está versionado en `wisp-relays.json`. El sitio
público no está actualizado.

```sh
curl http://repo1.maven.org/maven2/
```

## 255. ART verifica y ejecuta el `check-cast` de `AppAdapter`, y destapa el dexer ancho

Medido el 2026-11-23 por el agente de QA `ses_df4582f86ffeMf8Tte2809AvOd` en el cell de Droidloom,
no en la JVM. Este `§` transfiere su salida; aquí solo se releyeron los dex con el `dexdump` del SDK
de escritorio (`~/opt/android-sdk/build-tools/35.0.1/dexdump`) para comprobar offsets, `access` y
`registers`. No se levantó el cell, ni hubo gate, chromium, red, push ni publicación. El sujeto es
el `getItem` de `AppAdapter` que arregló `ee5a372`: el hueco tipado del `List.get` con su
`check-cast` y el puente del retorno covariante. La misma sesión cerró después el dexer ancho
(`f697c73`) y re-midió la clase **íntegra** en el aparato; esas dos correcciones van marcadas abajo.

**El aparato.** `droidloomctl status --json` contestó `{"message":"Droidloom is stopped",...}`; tras
`droidloomctl start`, `adb devices -l` lista
`10.177.0.2:5555 device product:aosp_cf_x86_64_only_phone model:Droidloom device:droidloom`, SDK 37.
`adb root` devuelve `adbd cannot run as root in production builds` y no hay `rish` ni `su`, así que
la sonda corre como uid 2000 con `app_process64` y `ANDROID_DATA=/data/local/tmp/artdata`.
`droidloomctl stop` devolvió el cell a `stopped`.

**Con `check-cast`.** El dex de esta ronda se generó con el binario de `6891693`, que contiene
`ee5a372`, y **sin** `getItemId`, que entonces estorbaba por el defecto que cerró luego `f697c73`;
la medición de referencia, con la clase íntegra, es la de más abajo. En el artefacto,
`p_cast/cast.dex` reproduce el método. `dexdump -d` muestra el puente
`getItem:(I)Ljava/lang/Object;` con
`access: 0x1041 (PUBLIC BRIDGE SYNTHETIC)`, que delega en `getItem:(I)Lshare/apps/util/App;`; en el
método real, el `check-cast` está en el offset `0x6`:

```text
|0005: move-result-object v0
|0006: check-cast v0, Lshare/apps/util/App;
|0008: return-object v0
```

En ART la corrida sale rc=0 y la línea de la sonda es
`CUENTA 2 / CAST_NULO null / CAST_EJECUTADO ClassCastException / PUENTE null / FIN`: el verificador
acepta el dex, el cast se ejecuta, el nulo pasa y un elemento ajeno del `List` revienta con
`ClassCastException` en vez de colarse como `Object`. Es el contrato que javac 17 escribe con su
`checkcast`.

**Sin `check-cast`, la falsificación no es vacua.** El binario se compiló desde una copia de `apkc`
hecha con `git archive HEAD`, con el interruptor apagado
(`apkc-sin-cast/apkc/javaemisor.go:5214`, `var insertaCheckcast = false`; ruta histórica de otro
checkout, no verificable en este árbol, y no se corrigió el número), y el dex
`p_sin_cast/sin.dex` deja el mismo `getItem` sin el `check-cast` del offset `0x6`. ART lo rechaza
antes de ejecutar:

```text
VerifyError: returning 'Reference java.lang.Object', but expected 'Reference: share/apps/util/App'
cannot access instance field java.lang.String App.name from object of type
Reference java.lang.Object
```

La segunda línea va partida para no pasar de 100 columnas. El dex sin el cast no verifica, así que
la corrida con cast sí distingue: el arreglo de `ee5a372` separa verificar de no verificar.

**Veredicto.** `ee5a372` queda cerrado en ART **en lo que arregla**: el `check-cast` del hueco
tipado y el puente del retorno covariante verifican, ejecutan y fallan con `ClassCastException`
cuando entra un elemento ajeno. No se cierra con esto el compilador entero de ApkShare: la frontera
de `MainActivity.java:55` sigue en curso, como decía `§253`.

**El dexer ancho, cerrado por `f697c73`.** Con el `AppAdapter` completo, ART rechazaba `getItemId`
con `wide register index out of range (1+1 >= 2)`. `dexdump` enseñaba el defecto: `registers: 2` e
`int-to-long v1, v1` en el offset `0x0`, con `return-wide v1`; ese `int-to-long` escribía el par
`v1`/`v2`, y el `v2` no existía en un marco de dos registros. Se reproduce con
`sonda-i2l/q/Ancho.java:4-5` del artefacto
(`~/.cache/apkc-web/art-checkcast/sonda-i2l/q/Ancho.java`, `long id(int p) { return p; }`), que se
compiló con javac 17 y se dexó con apkc. El defecto estaba en el `int-to-long` en sitio de
`apkc/dexinstr.go:769` y entró con `6a23468`; el arreglo `f697c73` («fix(dexer): exige que el par
ancho quepa en el marco») reserva el par completo. Los dos dex del mismo método `p.Ancho.id:(I)J`,
con el binario del árbol `f697c73` (13.078.267 B, compilado desde `apkc/`) y con el anterior:

```text
con f697c73: registers: 4
|0000: int-to-long v0, v3
|0001: return-wide v0
pre-fix:     registers: 2
|0000: int-to-long v1, v1
|0001: return-wide v1
```

En ART el fix sale **rc=0** y la sonda imprime `ID 7 / ID_ES_7 / FIN`, sin `VerifyError`. El control
pre-fix, con el binario anterior a `f697c73`, sale **rc=137** y `logcat` registra (partido para no
pasar de 100 columnas):

```text
java.lang.VerifyError: Verifier rejected class p.Ancho: long p.Ancho.id(int):
[0xFFFFFFFF] wide register index out of range (1+1 >= 2)
```

La falsificación no es vacua: solo cambia el dexer —fuente, `android.jar` y `app_process64` son los
mismos— y el mismo método pasa de no verificar a ejecutar.

**La clase íntegra, también en el aparato.** Con el binario de `f697c73`, el `AppAdapter` **sin
retirar nada** pasa el verificador de ART y ejecuta:
`CUENTA 2 / CAST_NULO null / CAST_EJECUTADO ClassCastException / PUENTE null / FIN`. Eso cubre
`getItemId`, el `check-cast` y el puente `getItem:(I)Ljava/lang/Object;`. La ronda que abrió este
`§` midió el `check-cast` sin `getItemId` porque entonces estorbaba; con `f697c73` ya no hay que
retirarlo, así que la corrección de este `§` es que la clase se mide íntegra.

**Fronteras de este `§`.** No se re-ejecutó ART aquí ni hubo gate, chromium, red, push ni
publicación; el `dexdump` local solo confirma el contenido de los artefactos. El dexer ancho queda
**cerrado** por `f697c73`, con su control pre-fix, y el `AppAdapter` íntegro verifica y ejecuta
en el aparato. El argv de `app_process64` viene literal en el bloque de abajo. Lo que sigue abierto
es el compilador entero de ApkShare (`MainActivity.java:55`), como decía `§253`, no este `§`. El
cell se devolvió a `stopped` y los temporales del aparato se borraron.

```sh
droidloomctl status --json
droidloomctl start
adb devices -l
adb root
dexdump -d "$HOME/.cache/apkc-web/art-checkcast/p_cast/cast.dex"
dexdump -d "$HOME/.cache/apkc-web/art-checkcast/p_sin_cast/sin.dex"
dexdump -d "$HOME/.cache/apkc-web/art-checkcast/head-full.dex"
# el dexer ancho, con el binario f697c73
cd ~/.cache/apkc-web/art-ancho-f697c73
./apkc compile --android-jar ~/opt/android-sdk/platforms/android-34/android.jar \
  -o ./clases ./src
./apkc dex -o ./nuevo.dex ./clases
adb push nuevo.dex /data/local/tmp/ancho-nuevo.dex
adb shell 'export ANDROID_DATA=/data/local/tmp/artdata; export ANDROID_ROOT=/system; \
export CLASSPATH=/data/local/tmp/ancho-nuevo.dex; \
/system/bin/app_process64 /data/local/tmp p.Main'
droidloomctl stop
```

## 256. El dex de las 6 clases de ApkShare, byte a byte, y el dedupe de los `styleables`

Registrado el 2026-11-23. Este `§` transfiere la salida de `211754c` y `87fc662` —el dex de las 6
clases— y la de `56fb021` —el dedupe de `styleables`—; aquí no se re-ejecutó nada: sin gate, sin
chromium, sin red, sin push y sin publicación. El sujeto del primer bloque son **6 clases**, no la
app entera: ApkShare **sigue sin compilar completo**, como dice la frontera de abajo.

**El dex de las 6 clases (`211754c`, `87fc662`).** Las 4 fuentes reales de
`~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/src/share/apps/util/` se compilaron con
`apkc compile` + `apkc dex` y su dex se comparó con el oráculo
`javac 17 --release 8 -g:lines,source`. `cmp` no encuentra ni una diferencia y los dos ficheros
pesan **5244 B**:

```sh
cmp final-nuestro.dex final-javac.dex
```

`Apk.<init>` queda con `locals=5` y `registers 9`; antes eran 5240 B, `locals=6` y `registers 13`.
La causa del cambio es que `javaemisor.go` devuelve el hueco de la local al cerrar su bloque
(`ambito`/`picos`, `jbloque`, `intenta`). La falsificación no es vacua: con
`reutilizaHuecos=false` salen los 5240 B, `locals=6` y `registers 13` de antes, y el `code_item`
cambia de longitud, **69 code units frente a 70** entre la falsificación y el árbol corregido.
Durante el desarrollo se rompieron **tres tests de `finally`** y se corrigieron con `picos`: el
manejador `any` bajaba a `astore_0` donde javac escribe `astore_3` (`87fc662` solo actualiza el
comentario del dex de `AppAdapter`).

**El dedupe de `styleables` (`56fb021`).** Cierra el CONFIRMADO de la auditoría de `f2eca37`. Un
`<declare-styleable>` no tiene configuración —ni densidad ni idioma—, así que su **nombre es la
clave**: la prioridad es proyecto(0) > isla(1) > AAR(2) y el desempate va por `origen` y `nombre`
del fichero. Cuando dos fuentes chocan, el `LogW` nombra la clave (`styleable/Vista`) y las dos
fuentes. Antes de este commit el enlazado dejaba `Vista=2` y `avisos=""`; ahora deja `Vista=1`,
**un solo** `int[] Vista=` en `R.java`, y el resultado es determinista también con el orden de
entrada invertido. La falsificación revierte `enlazar.go` y deja **3 FALLO**; con el arreglo, la
suite sale `ok … 188.102s`.

**Fronteras de este `§`.** El byte a byte es de esas 6 clases, **no de la app entera**: ApkShare
sigue sin compilar y el build se para en `MainActivity.java:74`, en la captura de `adapter` dentro
de una anónima anidada; ese frente sigue en curso. El dedupe de `styleables` sí queda cerrado por
`56fb021`, con su falsificación y su suite. No se re-ejecutó nada aquí ni hubo gate, chromium, red,
push ni publicación; el oráculo del dex es javac 17 y el de los recursos, la suite de `apkc`.

## 257. El avance de ApkShare hasta la anónima anidada, y WISP entre el contrato y el núcleo

Registrado el 2026-11-23. Este `§` transfiere la salida de `cdcf9e8`, `0ad4128`, `3794daf`,
`c726329`, `682759e` y `ae6bc0e`; aquí no se re-ejecutó nada: sin gate, sin chromium, sin red, sin
push y sin publicación. **Ninguna frontera de las de abajo se cierra**: ApkShare sigue sin compilar
y el núcleo WISP sigue inerte, fuera del wasm.

**ApkShare: la captura de una local en la anónima anidada (`cdcf9e8`).** La fuente
`MainActivity.java` declara una anónima dentro de otra y captura `adapter` en la exterior. El emisor
baja `val$adapter` a la anónima exterior y deja `this$1`/`this$0` en la interior; el oráculo
`javap` confirma los campos y el dex sale byte a byte salvo el constructor de la anónima. Con eso el
build avanza y se para ahora en `MainActivity.java:99`:

```text
runnable es Ljava/lang/Runnable; y se le asigna un Lshare/apps/util/MainActivity$4$1;
```

`superEInterfaces` (`apkc/javaemisor.go:1824`) devuelve el padre de una clase propia pero deja
`nil` como interfaces: descarta `propia.implementa`, así que `MainActivity$4$1` no consta como
`Runnable` y la asignación se rechaza. Ese frente sigue en curso; **no** se afirma que ApkShare
compile entero.

**Numeración de las anónimas (`0ad4128`).** El contador es por clase exterior y en orden de
creación; una anónima anidada no gasta número del exterior (`Exterior$1$1`, y su hermana `$2`). Los
**10 de 10** nombres coinciden con `javac 17` leídos con `javap`. La falsificación vuelve al
contador viejo y da un dex divergente, así que el arreglo no es vacuo. Queda una deuda medida: una
anónima dentro de una clase anidada **con nombre** sigue rechazada (`apkc/javafuente.go:3049`).

**WISP en la página: las CAs de un `File` local (`3794daf`, `c726329`).** El bundle de CAs se lee
de un `File` del dispositivo, **nunca** por `fetch`: un bundle que llegara por el relay convertiría
al relay en una CA. Se valida la extensión (`.pem`/`.crt`), que el nombre no sea una URL, el tope de
**64 KiB** (`MAX_CAS_BYTES`), que solo haya bloques `CERTIFICATE` y una huella sha256 opcional; y se
pasa al núcleo como `{pem, huella}` por `apkcWispCargarCAs` **antes** de `obtener`
(`studio/wisp.js:204-207`). Las tres falsificaciones fallan como se espera. Un fallo **ajeno** a
esto queda en `prueba-pestana.js`, en la sección de SW/offline con el wasm de fixture.

**El núcleo WISP sigue inerte (`ae6bc0e`).** El arreglo de la cola, los plazos y las CAs está
aprobado, pero el módulo queda **fuera del wasm**: el linker lo elimina al no tener llamadores y
`nucleoDisponible` (`studio/wisp.js:173-176`) falla cerrado sin abrir ningún socket. Antes de
cablearlo, el auditor exigió dos condiciones: rechazar la trama por tamaño **antes de copiar**
(hallazgo A: 48 MiB rechazados dejaban 48 MiB de `TotalAlloc`) y una allowlist con el PEM y su
huella (F).

**La vista previa local se mantiene sola (`682759e`).** `tools/preview-automatica.sh` vigila `HEAD`
y reconstruye `apkc.wasm` de forma atómica (`git archive` del árbol del commit y `mv -f`); el
servidor lee el fichero en cada petición (`studio/servidor.js:264`, `readFileSync`), así que no hay
que reiniciarlo. Se comprobó con dos `HEAD` distintos: `682759e9` y `3794daf4`.

**Fronteras de este `§`.** 1) ApkShare **no** compila: se para en el `Runnable` de
`MainActivity.java:99`. 2) WISP **no** está cableado a la página: faltan los frentes A-E y la
condición F. 3) Quedan **dos gates** por pasar. 4) El sitio publicado no se actualizó. 5) Los
commits no se empujaron y el CI no se midió. Nada de esto se re-ejecutó aquí; los oráculos citados
son los de cada commit (`javap`, el dex de `javac 17` y las pruebas de `studio/`).

## 258. La numeración de anónimas cerrada, la interfaz implícita y el endurecimiento de WISP

Registrado el 2026-11-23. Este `§` transfiere la salida de `0ad4128`, `8af90c2`, `ae6bc0e`,
`09d6de2`, `3794daf`, `c726329` y `36040e3`; aquí no se re-ejecutó nada: sin gate, sin chromium,
sin red, sin push y sin publicación. **ApkShare sigue sin compilar** y el núcleo WISP sigue
**inerte**, fuera del wasm.

**La numeración de las anónimas, con su condición cerrada (`0ad4128`).** `§257` la dejó aprobada
salvo que el crítico no pudo medir ApkShare porque la ruta de fuente que usó era errónea. Con la
ruta real, `src/share/apps/util/MainActivity.java`, el build ya no muestra `MainActivity$4$1` y
avanza hasta la línea 97. Los **10 de 10** nombres siguen coincidiendo con `javac 17` leídos con
`javap`, y la falsificación con el contador viejo da un dex divergente, así que el arreglo no es
vacuo. La condición de `§257` queda cerrada: se mide sobre la fuente que de verdad compila.

**La interfaz implícita de la anónima (`8af90c2`).** Aprobada: la anónima se asigna a su interfaz
aunque el campo sea del exterior. `check` sobre la fuente real ya no emite
`runnable es Ljava/lang/Runnable;` en la línea 99, y el build avanza a `MainActivity.java:97`. La
suite de `Interfaz|Anonima|Acces|Captura` sale `ok … 5.293s`. Queda una pregunta **no bloqueante**:
un `implementa` no resoluble descarta el padre en `asignableEstricto`
(`apkc/javaemisor.go:1751`), y eso solo alcanza a fuentes inválidas.

**ApkShare no compilaba: se paraba en la línea 97 (superado por `§269`).** El build paraba en
`MainActivity.java:97` con
`quedan -1 valores en la pila al cerrar el metodo`. Es un defecto **preexistente** —no lo introduce
`8af90c2`— de pila en el paso `int`→`long` de `handler.postDelayed(runnable, AutoResearchDelay)`
(`MainActivity.java:105`). Se reproduce con una clase mínima `class H { void f(long b){} }` y la
llamada `h.f(n)`, que deja `-1` en la pila al cerrar el método. Aquel era el estado en el momento de
la medición; `§269` registra el build completo con `rc=0` y 21 clases.

**El endurecimiento del núcleo WISP (`ae6bc0e`, `09d6de2`).** Aprobado: la cola baja de
**31 MiB a 16 MiB** con `CLOSE`; los plazos despiertan en **50 ms**; el crédito se topa en
**1024**; las CAs se validan de forma estricta; y la trama se rechaza **antes de copiar**, medido
con 48 MiB rechazados y `TotalAlloc +1736 B`. El módulo sigue **inerte y fuera del wasm**: el
linker lo elimina y `nucleoDisponible` falla cerrado sin abrir ningún socket. La allowlist F se
implementó después en `b7d08ae` y se registra en `§259`.

**Las CAs de la UI (`3794daf`, `c726329`, `36040e3`).** Las CAs se leen de un `File` local, el
orden es CAs→relay y la huella es `sha256(pem entregado)`, con el BOM corregido. Aprobado con
**6 de 6** rechazos. Riesgo vigilado: la UI puede parecer TLS verificado mientras el núcleo no
registra `apkcWispCargarCAs`; hoy falla cerrado.

**Deuda de resources, en corrección.** Un AAR real usa `jni/<abi>/` y **no** `lib/<abi>/` —el
`lib/` era un falso positivo del fixture—; además, el aviso de colisión de `.so` nombraba al
proyecto cuando ganaba otro AAR. Otro frente lo está corrigiendo: **en corrección**, no cerrado.

**Fronteras de este `§`.** 1) ApkShare **no** compila: el `postDelayed` deja `-1` en
`MainActivity.java:97`. 2) WISP **no** está cableado a la página: la allowlist F está hecha
(`§259`), pero falta el cableado del núcleo. 3) Quedan **dos gates** por pasar. 4) El sitio
publicado no se actualizó. 5) Los commits no se empujaron y el CI no se midió. 6) La deuda de
resources se cerró en `3cc7fe7` (`jni/<abi>/` y aviso del ganador real).
Nada de esto se re-ejecutó aquí; los oráculos citados son los de cada commit.

## 259. La allowlist de WISP y el cierre del vigilante de la preview

Registrado el 2026-11-23. Este `§` transfiere la salida de `b7d08ae`, `abf55a7` y `4991d86`; aquí
no se re-ejecutó nada: sin gate, sin chromium, sin red, sin push y sin publicación. La allowlist
queda **aprobada** con notas de endurecimiento **en corrección**, y el núcleo WISP sigue **inerte**,
fuera del wasm.

**La allowlist de relays, destinos y coordenadas (`b7d08ae`).** Cierra la condición F de la
auditoría de seguridad: el módulo validaba la forma de la URL del relay pero no quién era, y
`AbrirTLSWISP` abría cualquier `TCP:puerto` desde la IP del relay. Solo valen las **6** URLs
verificadas por handshake —`wss://wisp.mercurywork.shop/wisp/`, `wss://terbiumon.top/wisp/`,
`wss://anura.pro/wisp/`, `wss://lichology.com/wisp/`, `wss://wisp.waved.site/w/` y
`wss://gointospace.app/wisp/` (`apkc/depswisp.go:63-70`)— y los destinos `repo1.maven.org:443` y
`repo.maven.apache.org:443` (`apkc/depswisp.go:90-93`). Encima del flujo WISP va TLS de punta a
punta: el relay solo ve registros cifrados y no puede leer ni alterar los artefactos, así que la
lista autoriza la **pareja host:puerto** y no el host suelto; el mismo host en un puerto en claro
delataría el tráfico.

**El orden de la comprobación.** `Validar` (`apkc/depswisp.go:275`) pasa forma → allowlist de relay
→ forma de los destinos extra; `validarRelayAutorizado` (`:317`) compara la **URL canónica**
(`relayCanonico`, `:338`), de modo que cambiar la ruta de un relay conocido también lo convierte en
otro relay; `ValidarDestino` (`:373`) rechaza host o puerto fuera de la lista antes del CONNECT;
`ValidarCoordenadaMaven` (`:452`) y `RutaDependenciaMaven` (`:474`) exigen
`grupo:artefacto:version` y una ruta bajo `deps/` sin traversal, con la extensión acotada a
`aar`/`jar`/`pom`; y `AbrirTLSWISP`
(`:517`) valida configuración y destino **antes** del pool de CAs y del CONNECT, que sale por
`AbrirTCPAutorizado` (`apkc/wisp.go:465`).

**Las falsificaciones.** Un relay ajeno pasa toda la forma y solo la allowlist lo rechaza. Sin la
allowlist, el CONNECT sale a `smtp-ajeno.test:25`. Sin validar la coordenada,
`com.example:../../../../../x:1.0` se resuelve en `../../../../../x-1.0.aar`. La suite sale
`ok … 120.029s`; `-race` y el build wasm, verdes. El módulo sigue **inerte**: el linker lo elimina
al no tener llamadores y no hay `js.Global().Set`, así que `nucleoDisponible` falla cerrado.

**La auditoría de la allowlist.** APROBADA, sin ningún bypass, con **6 notas de endurecimiento**:
alias de ruta codificada (`/%77isp/`); `?`/`#` vacíos que hacen `WebSocket.New` lanzar un pánico sin
`recover` y matar el wasm; userinfo `evil@`; IP o loopback en `DestinosExtra`; puerto `0443`; y
`AbrirTCP` cruda como primitiva pública. Las notas están **en corrección**: no se dan por cerradas.

**El vigilante de la preview (`abf55a7`, `4991d86`).** Tras el `mv` atómico revalida `HEAD`
(`tools/preview-automatica.sh:38`); si cambió, no fija el estado. `4991d86` cierra el caso ABA: un
commit y su revert durante el build dejaban `head-construido` con el valor viejo, el ciclo siguiente
no reconstruía y se servía un wasm abandonado indefinidamente; ahora el estado se **vacía** (`:45`)
para reconstruir siempre. El subproceso recibe `APKC_PREVIEW_WASM` y `HOME` (`:34-35`). Verificado
con `estado == HEAD == wasm servido` (`4991d86`, sha `f9943c7b…`), y `studio/servidor.js:264` lee el
fichero en cada petición, así que no hay que reiniciar el servidor.

**Fronteras de este `§`.** 1) ApkShare **no** compila: el `int`→`long` del `postDelayed` se para en
`MainActivity.java:97`. 2) La allowlist está **en corrección**: sus 6 notas no se dan por cerradas.
3) Quedan **dos gates** de WISP por pasar. 4) El sitio publicado no se actualizó. 5) Los commits no
se empujaron y el CI no se midió. Nada de esto se re-ejecutó aquí; los oráculos citados son los de
cada commit.

