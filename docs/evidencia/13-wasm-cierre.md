# Evidencia — §260-§273

## 260. Los cierres del emisor, el AAR con `jni/` y los dos gates de WISP

Registrado el 2026-11-23. Este `§` transfiere la salida de `8af90c2`, `af73a8a`, `3d74b2e`,
`c0b40e9`, `3cc7fe7`, `b6b11c5`/`50aa19e`, `b7d08ae`, `6a79ff5` y `b64e731`; aquí no se
re-ejecutó nada: sin gate, sin chromium, sin red, sin push y sin publicación. Los cuatro primeros
cierran defectos del emisor que destapó ApkShare; el quinto, la deuda de los AAR; los demás, el
endurecimiento de WISP. **ApkShare sigue sin compilar**: se para en `MainActivity.java:179`, y el
núcleo WISP sigue **inerte**, fuera del wasm.

**La interfaz implícita de la anónima (`8af90c2`).** Cierra la condición de `§258` sobre la fuente
que de verdad compila. `superEInterfaces` (`apkc/javaemisor.go:1816`) ya no descarta
`propia.implementa`: la clase propia devuelve también sus interfaces, así que `MainActivity$4$1`
consta como `Runnable` y la anónima se asigna a su interfaz aunque el campo sea del exterior.
`check` sobre `src/share/apps/util/MainActivity.java` ya no emite el error de la línea 99, y
ApkShare avanza a la 97. Aprobado con las condiciones cerradas: el crítico dio `ok … 5.293s` sobre
`Interfaz|Anonima|Acces|Captura` y confirmó el paso 99→97. El interruptor
`interfacesDeLaPropiaEnAsignable` (`:1814`) es la falsificación: apagado, la clase propia vuelve a
devolver solo el padre (`javaemisor_interfaz_anonima_test.go:110`).

**El `int`→`long` en argumentos (`af73a8a`, corregido en `3d74b2e`).** `necesitaConversionEn`
(`apkc/javaemisor.go:4719`) y `conversionPrimitiva` (`:4808`) emiten `i2l` en el sitio de javac
cuando el parámetro pide un ensanchamiento, en vez de descontar el ancho después de emitir el
argumento: las 15 sondas contra javac 17 coinciden byte a byte, y el `postDelayed` de ApkShare
lleva el `i2l` en el pc 37 (`javaemisor_ancho_argumento_test.go:143`). La app pasa la línea 97. El
auditor encontró un **CONFIRMADO**: `boolean` se trataba como `int` y ensanchaba. `3d74b2e` lo
rechaza con la línea y nombra el tipo y el parámetro (JLS 5.1.2) en `apkc/javaemisor.go:4850`, con
el control de los anchos que sí ensanchan (`javaemisor_boolean_ensancha_test.go:88`) y la
falsificación con el interruptor `booleanEnsanchaEnArgumentos`
(`apkc/javaemisor.go:4764`). Aprobado.

**La llamada encadenada como argumento (`c0b40e9`).** `tipoSinEmitirCompleto`
(`apkc/javaemisor.go:5364`) y `vueltaGenerica` (`:5611`) estiman el tipo de `app.apks.get(0)` sin
emitir el receptor; el genérico (`Apk`) se prefiere al borrado (`Object`). El dex de la sonda y el
de la forma exacta de la línea 163 salen byte a byte contra `javac 17`
(`javaemisor_llamada_argumento_test.go:161` y `:190`). ApkShare pasa la 163 y se para ahora en la
179.

**El AAR con `jni/<abi>/` (`3cc7fe7`).** Un AAR real trae sus nativos en `jni/<abi>/`, no en
`lib/<abi>/`; `nativosDeAars` (`apkc/build.go:1955`) junta las dos rutas y los empaqueta bajo el
nombre canónico `lib/<abi>/<fichero>.so`, el que busca `System.loadLibrary`. Se verificó con
`unzip -l`, `apksigner verify` y `zipalign -c -p 4`. El aviso de colisión (`apkc/build.go:1906`)
nombra ahora al **ganador real** —proyecto, primer AAR o el mismo AAR— en vez de suponer que fue el
proyecto (`:1922` y `:1926`). Las falsificaciones salen en rojo y la suite `ok … 180.810s`
(`aar_real_test.go:425` y `:462`).

**Los dos gates de WISP (`b6b11c5`, que sube el submódulo `studio` a `50aa19e`; `b59d95f` y
`27b379a`).** El gate por defecto escucha
`requestWillBeSent` y `Network.webSocketCreated`, y exige **cero** peticiones y **cero** websockets
ajenos al origen; el modo externo se abre solo con `APKC_WISP_EXTERNO=1`. `tools/entorno.sh` tiene
las tareas `wisp` y `wisp-externo` (`tools/entorno.sh:123-145`): la segunda, sin la variable a 1,
sale con **rc 2** y su propio texto, para que «omitido» no se lea como verde. La clasificación de
`blob:`/`data:` es estricta con el origen incrustado. El resultado son **233/0** comprobaciones. La
auditoría encontró que el claim de las tareas era falso —`rc 127` por el `case` incompleto—, que
corrige `b59d95f`, y que `blob:`/`data:` no eran estrictos, que corrige `27b379a`. Queda abierta,
como otro frente en curso, la pregunta de los WebSockets dentro de workers.

**La allowlist de WISP, cerrada (`b7d08ae`, `6a79ff5`, `b64e731`).** Solo valen los **6** relays
verificados y los destinos `repo1.maven.org:443` y `repo.maven.apache.org:443`
(`apkc/depswisp.go:63-70` y `:90-93`); la coordenada Maven se exige `grupo:artefacto:version` sin
traversal (`:452` y `:474`). `relayCanonico` compara la ruta con `EscapedPath` (`:400`), de modo que
`/%77isp/` no se cuela; se rechazan `#`, `?` y userinfo (`:356` y `:370`), y el host de un destino
extra no puede ser una IPv4 corta, octal, hexadecimal ni entera (`:164`). `abrirTCPCrudo` es privada
(`apkc/wisp.go:424`) y solo `AbrirTCPAutorizado` la usa. Las 6 notas de la auditoría y el último
**CONFIRMADO** quedan cerrados y aprobados. El módulo sigue **inerte**: fuera del wasm, sin puente y
sin llamadores, así que `nucleoDisponible` falla cerrado.

**Fronteras de este `§`.** 1) ApkShare **no** compila: se para en `MainActivity.java:179`, en
`getExternalFilesDir()` sin receptor escrito; ese frente sigue en curso. 2) Falta el cableado del
núcleo WISP a la página. 3) Queda la publicación. 4) No hubo gate final, ni push, ni CI, ni
publicación, por orden del usuario. Nada de esto se re-ejecutó aquí; los oráculos citados son los de
cada commit (`javac 17`, `javap`, `unzip -l`, `apksigner`, `zipalign` y las pruebas de `apkc` y
`studio/`). Los selectores que reproducen las suites citadas:

```sh
cd apkc && go test -run 'Interfaz|Anonima|Acces|Captura' -count=1 .
cd apkc && go test -run 'Ensanchamiento|Boolean|PostDelayed' -count=1 .
cd apkc && go test -run 'LlamadaEncadenada' -count=1 .
cd apkc && go test -run 'AarEnJar|AvisoDeColision|ApkConDosAars' -count=1 .
tools/entorno.sh wisp
APKC_WISP_EXTERNO=1 tools/entorno.sh wisp-externo
```

## 261. El receptor heredado y los cierres finales de WISP

Registrado el 2026-11-23. Este `§` transfiere la salida de `58aaaed`, `b79f8ac` (que sube el
submódulo `studio` de `76c647c` a `ef738c9`) y las auditorías finales de WISP; aquí no se
re-ejecutó nada: sin gate, sin chromium, sin red, sin push y sin publicación. Los cierres de
`3d74b2e` (`boolean` no ensancha), `b59d95f`, `27b379a` y `b64e731` ya quedaron en `§260` y no se
repiten; lo de abajo es el delta posterior y el estado final consolidado.

**El receptor heredado sin receptor escrito (`58aaaed`).** `tipoPrevisto` y `tipoEstimado` solo
miraban la clase actual, así que dentro de la anónima de `Runnable` de `MainActivity.java:179`
`getApplicationContext()` no tenía candidatas y la línea paraba. `candidatasSinReceptor`
(`apkc/javaemisor.go:5441`) busca entonces el nombre en las clases que la encierran, de dentro
afuera (`clasesEnAlcance`, `:8854`), solo dentro de una anónima o una inner no estática (`:5458`),
y sin escribir ninguna instrucción. La vuelta de `getExternalFilesDir(String)` queda en
`Ljava/io/File;` y `File(File, String)` encaja. El oráculo es `javac 17 --release 8` instrucción a
instrucción y el dex byte a byte salvo el constructor de la anónima (§100)
(`javaemisor_receptor_heredado_test.go:81`); la falsificación apaga `estimaMetodoDeExterior`
(`apkc/javaemisor.go:5439`, sonda en `javaemisor_receptor_heredado_test.go:110`) y el control fija
que un nombre inexistente sigue rechazando con el mensaje anterior (`:138`). ApkShare pasa la línea
179 y se para ahora en la columna 103, en `apk.name + ".apk"` como argumento (concatenación,
`joperador`), frente en curso.

**Los dos gates y la allowlist, cierre final (`76c647c`, `ef738c9`, `b79f8ac`).** El gate por
defecto exige cero peticiones y cero websockets ajenos al origen, y el modo externo se abre solo
con `APKC_WISP_EXTERNO=1`. `76c647c` clasifica el origen incrustado en `blob:`/`data:` y `ef738c9`
documenta el **rc 2** de `wisp-externo` en `studio/DEPS.md`; `b79f8ac` actualiza el puntero del
submódulo. `tools/entorno.sh` expone las tareas en `:123-145`: `wisp` corre la falsificación sin
navegador ni red y `wisp-externo`, sin la variable a 1, sale con rc 2 y su propio texto
(`:131-134`). El filtro grueso de `studio/prueba-pestana.js` queda etiquetado como **no
autoritativo**: la regla que manda es la del módulo. Las auditorías finales dan **APROBAR** sin
contradicciones entre `studio/DEPS.md`, `tools/entorno.sh`, el módulo WISP y su prueba; el módulo
sigue **inerte**, fuera del wasm, sin puente y sin llamadores.

**Fronteras de este `§`.** 1) ApkShare **no** compila: `MainActivity.java:179`, columna 103, en
`apk.name + ".apk"` como argumento (concatenación, `joperador`); ese frente sigue en curso. 2) Falta
el cableado del núcleo WISP a la página. 3) Queda la publicación. 4) No hubo gate final, ni push,
ni CI, ni publicación, por orden del usuario. Nada de esto se re-ejecutó aquí; los oráculos citados
son los de cada commit (`javac 17` y las pruebas de `apkc` y `studio/`). Los selectores que
reproducen lo citado:

```sh
cd apkc && go test -run 'ReceptorHeredado|ReceptorInexistente' -count=1 .
tools/entorno.sh wisp
APKC_WISP_EXTERNO=1 tools/entorno.sh wisp-externo
```

## 262. El genérico del local, la sobrecarga por tipo y el gate de red con workers

Registrado el 2026-11-23. Este `§` transfiere la salida de `45ec24a`, `445f7fb` y `ee19eb6` (que
sube el submódulo `studio` de `ef738c9` a `f34c79d`), más las correcciones documentales `c072831`
y `867d3fa`; aquí no se re-ejecutó nada: sin gate, sin chromium, sin red, sin push ni publicación.
Las citas `archivo:línea` corresponden al commit que se indica en cada párrafo y pueden estar
desplazadas en el árbol actual; para localizar el símbolo, busca su nombre (`grep -n`) en vez de la
línea.

**El genérico del local en la llamada encadenada (`45ec24a`).** `c0b40e9` emitía el borrado en
silencio para `void dame(List<Apk> m) { usa(m.get(0)); }`: elegía `usa(Object)` y perdía el
`check-cast`, seis instrucciones (`2a2b03b900100200b60014b1`) frente a las siete de `javac 17`
(`2a2b03b900070200c0000db6000fb1`, con `checkcast 0xc0`). La causa era que
`tipoSinEmitirCompleto` se quedaba con `lv.desc` y tiraba el argumento de tipo, así que
`vueltaGenerica` recibía `nil`. `localVar.generico` (`apkc/javaemisor.go:2447`) guarda el tipo
declarado, y `reservaConGenerico` (`:2772`) lo pone en los dos sitios donde el fuente lo dice —los
parámetros, que lo traen en `p.generico`, y las locales cuyo inicializador lo delata
(`List<Apk> apks = app.apks`)—; `genericoDeLocal` (`:2788`) lo deduce del valor.
`tipoSinEmitirCompleto` (`:5350`) devuelve ese genérico cuando la expresión **es** la local, no solo
cuando sigue un campo de ella, y el fail-safe de `vueltaSinEmitir` (`:5465`) usa
`vueltaConTipoVariable` (`:5509`): si la vuelta declarada es un tipo variable (`List.get` declara
`(I)TE;`) y no hay con qué sustituirlo —una local cuyo tipo no se conoce, un método heredado de otra
clase—, la llamada no se estima y vuelve el rechazo explícito; nunca el borrado.
`propagaGenericoDeLocal` (`:5308`) es el interruptor de la sonda y, apagado, reproduce el defecto
literal del crítico. El oráculo es `javac 17 --release 8 -g:lines,source` y el mismo dexer byte a
byte para `List<Apk>` y para `Map<String,Apk>` —el segundo argumento de tipo, no el primero—
(`javaemisor_llamada_argumento_test.go:365` y `:397`), con los controles de una local sin argumentos
y de `var`, que se rechazan (`:474` y `:517`). Las 9 pruebas del fichero salen en verde, la suite
`ok … 164.971s`, y el crítico aprueba. Quedan como preguntas abiertas no bloqueantes el local
capturado `val$m`, `var`, el `for` de dos puntos, el `try` y el comodín en la vuelta.

**La sobrecarga de una llamada sin receptor (`445f7fb`).** `candidatasSinReceptor`
(`apkc/javaemisor.go:5446`) contaba por nombre y aridad y rechazaba con «hay 2 candidatas con 1
argumentos y el tipo lo decide la sobrecarga» un fuente que javac acepta: `f(String)`/`f(int)` en la
clase exterior y `usa(f("x"))` dentro de la anónima. Ahora `unaCandidataPorTipos` (`:5478`) elige
con `elegirSobrecarga` (`:1435`) —las mismas fases estricta a laxa que `buscarMetodo` (`:1423`), con
las varargs y `masEspecificaDe`— y devuelve la lista original si algún argumento no se puede
estimar o las fases no dejan una sola candidata: el fail-safe es no elegir, nunca la primera.
`filtraCandidatasPorTipos` (`:5470`) es el interruptor de la falsificación, que reproduce el rechazo
previo. `javac 17` elige `f:(Ljava/lang/String;)Ljava/lang/String;` y
`usa:(Ljava/lang/String;)V`, y el `onClick` sale instrucción a instrucción y en dex byte a byte
(`javaemisor_receptor_heredado_test.go:169`, falsificación en `:197`). La suite del commit sale
`ok … 153.997s` —medición del builder, salida transferida, no versionada; la reproduce
`cd apkc && go test -count=1 .` sobre `445f7fb`— y `go vet` queda limpio en nativo y en wasm. Deuda
pactada: la sobrecarga varargs de igual nombre y los argumentos no estimables siguen en el rechazo
conservador.

**El gate de red con workers (`ee19eb6`, `studio` `f34c79d`).** El `Network.enable` iba solo a la
sesión de la página, así que un `WebSocket` abierto en un worker no aparecía en ninguna lista y se
escapaba al gate. `studio/prueba-pestana.js:222` engancha con `Target.setAutoAttach`
(`autoAttach: true`, `waitForDebuggerOnStart: false`, `flatten: true`) —plano y antes de navegar— y
habilita la red en cada sesión `Target.attachedToTarget` (`:198-205`);
`studio/wisp-gates.js:116` suma los eventos con `sessionId` sin filtrar por sesión
(`acumularRed`). La sonda abre en un worker `wss://sonda-ajena-del-gate.invalid/relay` y exige
verlo (`prueba-pestana.js:6341` en `studio` `f34c79d`; el símbolo es
`sonda-ajena-del-gate.invalid`); sin auto-attach (`APKC_GATE_SIN_WORKERS=1`, `:19`) se escapa:
**247 bien, 2 mal**; con él, **249 bien, 0 mal**. Las dos cifras son medición del builder, salida
transferida, no versionada; las reproducen
`APKC_WASM=… APKC_GATE_SIN_WORKERS=1 node studio/prueba-pestana.js` y
`APKC_WASM=… node studio/prueba-pestana.js`. El recuento base es el del árbol con `KS_RELEASE`,
`AAB_EJEMPLO` y el `android.jar` de `ANDROID_JAR`, no el de una corrida recortada. La cobertura se
comprueba aparte (`:6417`: 1 `page`, 1 `service_worker` y 2 `worker` con `Network.enable`), y la
falsificación sin navegador queda en `studio/prueba-wisp-gates.mjs:110`.
Deuda: `waitForDebuggerOnStart:false` deja una carrera teórica entre el enganche del worker y su
primer `WebSocket`, y las listas exactas ahora deduplican por URL porque un mismo recurso llega una
vez por sesión.

**Las correcciones documentales (`c072831`, `867d3fa`).** `c072831` corrige en `§260` la cita del
interruptor `booleanEnsanchaEnArgumentos` (`apkc/javaemisor.go:4716` pasa a `:4715`; en el árbol
actual el interruptor está en `:4764`, `§260`), y `867d3fa` es el `§261`, que ya quedó escrito.

**Fronteras de este `§`.** 1) ApkShare **no** compila: en el momento de la medición se paraba en
`MainActivity.java:179`, columna 103, en `apk.name + ".apk"` como argumento (concatenación,
`joperador`), y `§263` mide ese cierre. Tras `f064e5f` la parada verificada es
`MainActivity.java:198`, columna 50, en `Intent.ACTION_SEND` (constante `String` del `android.jar`)
como argumento del constructor; la midió el builder con el `check --proyecto` del fuente real
(`apkc check --android-jar … --proyecto ~/Downloads/ApkShare-main …/util/MainActivity.java`) y aquí
se transfiere sin re-ejecutarse; otro frente la corrige y no se da por cerrada.
2) WISP sigue **inerte**, fuera del wasm, sin puente y sin llamadores. 3) Falta el cableado del
núcleo WISP a la página. 4) Queda la publicación. 5) No hubo gate final, ni push, ni CI, ni
publicación, por orden del usuario. Nada de esto se re-ejecutó aquí; los oráculos citados son los de
cada commit (`javac 17`, `go vet`, chromium por `prueba-pestana.js` y las pruebas de `apkc` y
`studio/`). Los selectores que reproducen lo citado:

```sh
cd apkc && go test -run 'LlamadaEncadenada' -count=1 .
cd apkc && go test -run 'SobrecargaExterior' -count=1 .
node studio/prueba-wisp-gates.mjs
grep -n 'sonda-ajena-del-gate' studio/prueba-pestana.js
APKC_WASM=… APKC_GATE_SIN_WORKERS=1 node studio/prueba-pestana.js
```

## 263. La concatenación como argumento de `File` (`f064e5f`)

Registrado el 2026-11-23. Este `§` transfiere la salida de `f064e5f`, que añade el caso a
`tipoEstimado` y su prueba; aquí no se re-ejecutó nada: sin gate, sin chromium, sin red, sin push ni
publicación. Las citas `archivo:línea` corresponden a `f064e5f` y pueden estar desplazadas en el
árbol actual; para localizar el símbolo, busca su nombre. El inventario de la Fase 5 registró la
fila 5.51 en `56c31c7`.

**El defecto.** La línea 179 de `src/share/apps/util/MainActivity.java` del zip de ApkShare pasa
`apk.name + ".apk"`, un `joperador` de `+`, como segundo argumento de `File(File,String)`.
`tipoEstimado` no tenía el caso, la sobrecarga no se resolvía y el `check` paraba con el mensaje
literal que leyó el usuario, reproducido por la falsificación del binario:

```
[build] MainActivity.java: linea 175: linea 176: linea 179: linea 179, columna 103: el tipo de una
expresion main.joperador no se puede estimar sin emitirla
```

**La corrección.** `tipoEstimado` (`apkc/javaemisor.go:5910`) añade `case joperador`: si
`estimaConcatEnArgumento` (`:5322`) está encendido, `v.op == "+"` y `esConcatenacion(v)` (`:3485`)
—un lado devuelve cadena (JLS 15.18.1)—, devuelve `Ljava/lang/String;` (`:5917`) sin emitir nada; si
no, el rechazo de siempre (`:5920`). Estimar y emitir coinciden: `exp` manda el nodo a `concatenar`,
que devuelve el mismo descriptor y no cambia el orden de evaluación. `estimaConcatEnArgumento` es el
interruptor de la falsificación y va encendido en el binario que se publica.

**El oráculo.** `TestConcatComoArgumentoContraJavac17`
(`apkc/javaemisor_concat_argumento_test.go:103`) compila las líneas 171-179 reales, con la anónima
que captura `apk`, contra `javac 17 --release 8 -g:lines,source`: `javac` elige `File(File,String)`
(`(Ljava/io/File;Ljava/lang/String;)V`), el `run()V` sale instrucción a instrucción igual y el dex
entero byte a byte salvo `MethodParameters` del constructor de la anónima, que en el momento de la
medición se normalizaba (§100); desde `d123135` el emisor ya no lo escribe y la comparación es byte a
byte sin normalizar.
`TestConcatStringEnteroContraJavac17` (`:141`) fija que `int + String` (`i + ".apk"`) también se
estima como `String` y que el dex sale byte a byte, con el `append(I)` del entero.
`TestConcatComoArgumentoFalsificacion` (`:170`) apaga el interruptor y comprueba que reaparece el
literal «el tipo de una expresion main.joperador no se puede estimar sin emitirla» nombrando el
fuente y la línea (`MainActivity.java: ` y `linea 26` del fuente sintético, que reproduce la 179).
`TestCuentaEnteraEnFileSigueRechazando` (`:198`) fija el control negativo: `1 + 2` en esa posición
no se estima como cadena y se rechaza, y `javac` también rechaza el fuente (`int cannot be converted
to String` o `no suitable constructor`).

**La suite.** `ok github.com/ajinasokan/apkc 174.637s` del commit —medición del builder, salida
transferida, no versionada; la reproduce `cd apkc && go test -count=1 .` sobre `f064e5f`.

**Frontera del momento de la medición (superada por `§265`).** Con `f064e5f` la 179 pasaba y
ApkShare se detenía en `MainActivity.java:198`, columna 50, en `Intent.ACTION_SEND` (constante
`String` del `android.jar`) como argumento del constructor `new Intent(...)`; la midió el builder y
aquí se transfiere sin re-ejecutarse. `§265` cerró esa parada y `§269` registra el build completo
con `rc=0`; las demás fronteras de `§262` (WISP inerte y sin cablear, y la publicación) siguen sin
cambios.

```sh
cd apkc && go test -run 'ConcatComoArgumento|ConcatStringEntero|CuentaEnteraEnFile' -count=1 .
grep -n 'func Test' apkc/javaemisor_concat_argumento_test.go
```

## 264. El aviso «ni PKCS#12 ni JKS» salta también si el keystore no existe

Registrado el 2026-11-23. Transfiere la investigación `ses_df3efa15cffe1yUNMrBMUFbGxM`, que ejecutó
el run de ApkShare; aquí no se re-ejecutó ese run, ni hubo gate, ni chromium, ni red, ni push, ni
publicación. Lo que sí se re-ejecutó es el bloque «Comandos», y las citas `archivo:línea` son de
este árbol (`91744fe`). Lo que solo viene del informe va marcado como **transferido, no
versionado**.

**Qué pregunta el aviso, y qué no.** `resolveSigner` (`apkc/gopackage.go:78`) solo decide con
`--signer=auto`: un backend explícito se devuelve tal cual (`:79-81`), así que `--signer=jvm` y
`--signer=go` nunca llegan a la comprobación. Con `auto`, `keystoreIsReadable`
(`apkc/keystore.go:75`) abre la ruta, lee cuatro bytes con `io.ReadFull` (`:83`) y delega en
`keystoreHeadIsReadable` (`:91-93`): es legible si `isJKSHead` reconoce `FEEDFEED` o si el primer
byte es `0x30` (ASN.1, que cubre PKCS#12). Un JCEKS, un fichero cualquiera o una ruta que no abre
devuelven `false`, y entonces no hay error: `LogD` con el literal de `:87` y `return signerJVM`
(`:86-88`).

**El defecto de diagnóstico.** Que `os.Open` falle (`keystore.go:76-78`) y que el formato sea ajeno
devuelven lo mismo, así que un fichero ausente imprime «is neither a PKCS#12 nor a JKS keystore,
and only the jvm backend reads the rest»: habla de formato sobre algo que no está. El mensaje que
sí nombra la causa existe, `requireKeystore` (`keystore.go:101-105`, «there is no keystore at …»),
pero se llama en la rama que ya va a escribir el paquete (`apkc/build.go:421`, `:427`, `:450`) y en
`apkc/sign.go:71`, es decir **después** de `resolveSigner` (`build.go:212`). El comentario de
`build.go:412-419` declara esa separación a propósito —quien pregunta por la firma es la rama que
firma—, y la comprobación de `resolveSigner` la adelanta sin querer: un build que muera antes, en
recursos o en Java, solo deja el aviso engañoso.

**El orden del build.** `resolveSigner` en `build.go:212` va antes que `compileRes` (`:263`) y que
`compileJava` (`:309`), así que el aviso sale al principio aunque el build muera mucho después en
Java. `--compiler go` no cambia el enrutado; solo cambia quién compila (`build.go:177`, `:306-307`).

**El keystore por defecto no lo dispara.** `doctor.go:162` resuelve `~/.android/debug.keystore`, y
ese fichero empieza por `30`, así que cuenta como legible:

```
$ od -An -tx1 -N8 ~/.android/debug.keystore
 30 82 0a 66 02 01 03 30
$ wc -c < ~/.android/debug.keystore
2666
```

**Dónde sí aparece.** El `grep` de los registros de `~/.cache/apkc-web` solo encuentra el literal
en los fixtures de `TestResolveSigner` (`absent.p12` y `old.jceks`) y en el arnés wasm, cuyo `HOME`
simulado es `/home/usuario` (`shim/navegador-sim.js:372`) y que no monta nada en `~/.android`. El
filtro del informe (`grep -v /home/usuario`) quitaba justo esas líneas, y son la prueba de la
causa: allí la ruta del mensaje es la que resuelve `doctor` y no existe, y aun así se anuncia un
formato. No aparece ningún run nativo con la ruta real.

**El run de ApkShare no dejó paquete.** `~/Downloads/ApkShare-main/build` (mtime 21:43-21:50) no
tiene `.class`, ni `classes.dex`, ni `.apk`: no hay paquete que verificar, y el aviso no bloquea
nada porque el backend elegido (`jvm`) tampoco se llegó a usar. Los tres ceros vienen del informe
(**transferido, no versionado**) y aquí se reprodujeron con `find`: 0, 0 y 0.

**La página.** `studio/app.js:1796-1799` cataloga el literal como aviso de capa `entorno` y
sugiere `--signer=jvm`, coherente con el fallback; el id de la entrada es `aar-not-read`, que no le
pega, pero el `re` y el `lit` sí son los del mensaje. La cita de `studio/` viene del checkout
principal, porque en este worktree el submódulo está sin inicializar.

**Veredicto.** Benigno como decisión —elegir `jvm` es lo correcto cuando el lector Go no conoce el
formato— y engañoso como diagnóstico, porque confunde «no existe» con «es de otro formato». La
corrección está **en curso, otro frente**, y aquí no se da por hecha: separar los dos casos en
`gopackage.go:86-88` y añadir a `TestResolveSigner` (`apkc/keystore_test.go:417`) un caso de ruta
ausente que exija el texto de fichero ausente y no «neither a PKCS#12»; `:439` ya fija que
`absent.p12` no es legible, que es la mitad de la pregunta.

**Incertidumbre.** La ruta exacta que disparó el aviso en el run de ApkShare no quedó guardada
(**INCIERTO**, transferido), y no se ejecutó `apksigner verify` porque no hay APK que verificar.

```sh
od -An -tx1 -N8 ~/.android/debug.keystore
wc -c < ~/.android/debug.keystore
sed -n '70,100p' apkc/gopackage.go
sed -n '60,110p' apkc/keystore.go
sed -n '170,182p;205,218p;258,266p;305,312p;405,455p' apkc/build.go
sed -n '155,168p' apkc/doctor.go
sed -n '410,445p' apkc/keystore_test.go
grep -rn 'is neither a PKCS#12' ~/.cache/apkc-web --include='*.log' | grep -v /home/usuario
grep -rn 'is neither a PKCS#12' ~/.cache/apkc-web --include='*.log' | grep /home/usuario
grep -n 'home/usuario' shim/navegador-sim.js
ls -la ~/Downloads/ApkShare-main/build
find ~/Downloads/ApkShare-main/build -name '*.class' | wc -l
find ~/Downloads/ApkShare-main/build -name '*.dex' | wc -l
find ~/Downloads/ApkShare-main/build -name '*.apk' | wc -l
grep -n 'is neither a PKCS#12' studio/app.js
```

## 265. Las constantes `String` del classpath se doblan como su literal (`98dd864`)

Registrado el 2026-11-23. Transfiere la medición del builder `ses_df3f1d206ffecDlPWv5WN2VDvd`,
commit `98dd864`. Aquí se re-ejecutó solo el bloque «Comandos» —los `grep` y la prueba puntual— y
no la suite: sin gate, sin chromium, sin red, sin push y sin publicación. Las citas
`archivo:línea` son de este árbol (`e142747`, que ya contiene `98dd864`); lo que solo viene del
informe va marcado como **transferido, no versionado**.

**El bloqueo.** `apkc build --linker go --compiler go --sdk-version 34 --android-jar
~/opt/android-sdk/platforms/android-34/android.jar` sobre
`~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/` paraba en la 198, el
`new Intent(Intent.ACTION_SEND)`, con el literal del informe:

```
[build] MainActivity.java: linea 175: linea 176: linea 198: linea 198, columna 50: al estimar el
tipo de Intent.ACTION_SEND, la constante android/content/Intent.ACTION_SEND no es un entero:
doblarla no esta medida
```

**La causa.** `campoResuelto` solo sabía viajar con un entero, `constante` y `valor int32`
(`apkc/javaemisor.go:2052-2053`), y `buscarCampo` (`:2075`) trataba cualquier `ConstantValue`
(`f.HasConst`, `:2139`) como entero: si `f.ConstTag != 3`, rechazaba con el literal «la constante
%s.%s no es un entero: doblarla no esta medida». `cpString` es el tag 8 (`apkc/classfile.go:44`),
así que `Intent.ACTION_SEND` —`ConstantValue` del `android.jar`, medido en `§130`— moría en esa
comprobación antes de estimar o emitir nada.

**La corrección.** El valor viaja en dos campos nuevos, `esCadena` y `cadena`
(`javaemisor.go:2054-2058`), que acompañan al descriptor y no lo sustituyen. `buscarCampo` acepta
ahora `case cpString` (`:2144-2152`): marca `constante`, `esCadena` y copia `f.ConstStr` en
`cadena`; el `default` de `:2153-2154` se queda con el rechazo de lo que no es ni entero ni cadena.
El interruptor `pliegaCadenaEstatica` (`:2066-2071`) es el de la falsificación y va encendido en el
binario que se publica. Los tres consumidores distinguen las dos constantes:

- `plegarConstante` (`:3863`) no pliega una cadena como número: si `cr.esCadena`, devuelve
  `constNum{}, false` y la deja para el acceso estático.
- `accesoEstatico` (`:4056`) emite `ldc` del literal en vez de `getstatic` (`:4081-4086`): `e.ldc`
  de `cr.cadena`, `e.sube()` y el descriptor de vuelta, que es lo que hace `javac` con una
  constante de compilación.
- `descDeAccesoEstatico` (`:8251`) devuelve `cr.desc` para una cadena y `"I"` para un entero
  (`:8269-8273`), así que estimar y emitir siguen de acuerdo sin emitir nada.

**El oráculo.** `TestCadenaEstaticaDelAndroidJarContraJavac17`
(`apkc/javaemisor_cadena_estatica_test.go:39`) compila un fuente con `Intent.ACTION_SEND`,
`Intent.FLAG_GRANT_READ_URI_PERMISSION` y `Build.MODEL` contra `javac 17 --release 8
-g:lines,source` por el mismo dexer, y exige el **dex entero byte a byte**. Además desarma los dos
métodos: `comparte()` tiene 1 `ldc` de `sandroid.intent.action.SEND` y 0 `getstatic` —ACTION_SEND
se dobla—, mientras `modelo()` tiene 1 `getstatic` y 0 `ldc`, porque `Build.MODEL` es `static final
String` **sin** `ConstantValue` y se lee del campo. El `static final int`
(`FLAG_GRANT_READ_URI_PERMISSION`) se sigue plegando como antes.

**La falsificación.** `TestCadenaEstaticaFalsificacion` (`:117`) apaga `pliegaCadenaEstatica` y
reproduce el literal del error. La salida de este worktree encadena las posiciones y acaba en
`linea 8, columna 31: al estimar el tipo de Intent.ACTION_SEND, la constante
android/content/Intent.ACTION_SEND no es un entero: doblarla no esta medida`, precedida de
`Envio.java: linea 6: linea 7: linea 8:`. Que la columna sea la 31 del fuente sintético y no la 50
del real no importa: lo que fija es que el camino vuelve a ser el de la estimación del argumento de
`Intent(String)` y no otro rechazo.

**La suite.** `ok github.com/ajinasokan/apkc 203.024s` y `go vet ./...` limpio, del commit
—medición del builder, salida transferida, no versionada; la reproduce `cd apkc && go test
-count=1 .` sobre `98dd864`. Aquí solo se ejecutó la prueba puntual:
`ok github.com/ajinasokan/apkc 1.167s` con los tres `CadenaEstatica` en `PASS`.

**Frontera histórica, cerrada en `§265`.** Con `98dd864` la 198 pasaba y ApkShare se detenía en
`MainActivity.java:212`, columna 31, en `Intent.createChooser(sharedIntent, "Compartir con")`; la
línea es la que imprime `sed -n '212p'` de la fuente real. La midió el builder y aquí se transfiere
sin re-ejecutar el build. `§265` cerró esa parada y `§269` registra el build completo con `rc=0`.

**Observación del keystore, no un cierre.** Sin `--keystore`, `build --linker go --compiler go` no
llama a `findSDKs` (`apkc/main.go:28-30`, por `sinProcesos`), así que el keystore queda vacío y el
aviso de `§264` aparece; se va al pasar `--keystore ~/.android/debug.keystore`. Es una observación
del builder y no una corrección de este `§`: el diagnóstico sigue siendo el de `§264`.

```sh
grep -n 'esCadena\|pliegaCadenaEstatica' apkc/javaemisor.go
grep -n 'cpString' apkc/classfile.go apkc/javaemisor.go
sed -n '2048,2072p;2138,2156p;3858,3866p;4080,4088p;8265,8275p' apkc/javaemisor.go
grep -n 'func Test' apkc/javaemisor_cadena_estatica_test.go
cd apkc && go test -run 'CadenaEstatica' -v -count=1 .
sed -n '212p' \
  ~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/src/share/apps/util/MainActivity.java
sed -n '24,32p' apkc/main.go
```

## 266. El keystore ausente ya no se anuncia como formato ilegible (`0e65ac1`)

Registrado el 2026-11-23. Transfiere la medición del builder sobre `8eae949`, integrada aquí como
`0e65ac1`; lo que solo viene de ese informe va marcado como **transferido, no versionado**, y las
citas `archivo:línea` son de este árbol. Aquí se re-ejecutó solo la prueba puntual del bloque
«Comandos»: sin suite completa, sin `go vet`, sin binario real, sin gate, sin chromium, sin red,
sin push y sin publicación.

**Lo que decía antes.** Con `--keystore /nonexistent/keystore.p12` el build dejaba primero la línea
del formato (partida aquí para no pasar de 100 columnas; en el registro es una sola):

```
[sign] --signer=auto: /nonexistent/keystore.p12 is neither a PKCS#12 nor a JKS keystore,
and only the jvm backend reads the rest
```

Y después `there is no keystore at …`, que sí nombraba la causa. El fallback a `jvm` era el correcto
y benigno (`§264`), pero el diagnóstico culpaba al formato de un fichero que no existía: el mismo
defecto que `§264` dejó escrito como «en curso, otro frente».

**La corrección.** `keystoreAbsentMessage` (`apkc/keystore.go:101`) es ahora el único texto con que
el paquete dice que un keystore no está; `requireKeystore` (`:125`) lo usa en vez de repetir el
`LogF`, y `keystoreRefusal` (`:110`) elige entre ese texto y el de formato con un `os.Stat`
(`:111`): si la ruta no existe, ausente; si existe y la cabecera no encaja, formato.
`resolveSigner` no cambia de forma: `apkc/gopackage.go:87-90` sigue avisando por `LogD` y
devolviendo `signerJVM`; solo cambia el texto, que pasa a escribirlo `keystoreRefusal`.

**Los tests.** `TestResolveSigner` (`apkc/keystore_test.go:383`) gana dos casos que capturan la
salida real de `resolveSigner` con `capturarSalida` (`:428`), no un helper: `:420` da `absent.p12` y
exige «there is no keystore at» a la vez que prohíbe «neither a PKCS#12»; `:422` da `old.jceks`
—magic `CE CE CE CE`, `:390`— y exige lo contrario. Se conservan los casos anteriores, incluido
`:417`, que solo mira el backend elegido.

**La falsificación.** Reponer el `LogD` viejo en `gopackage.go:90` hace fallar `TestResolveSigner`
en los dos asertos —`:433` para el aviso exigido y `:436` para el prohibido— y el mensaje literal
vuelve a ser el del formato. Medición del builder, **transferida, no versionada**.

**La verificación.** Del builder, el bloque de tests: `go test -run TestResolveSigner -v .` en
`PASS`, la suite completa `ok github.com/ajinasokan/apkc 185.645s`, `go vet .` con rc 0
y `gofmt -l` sin salida —**medición transferida, no versionada**. Aquí se repitió la prueba
puntual:

```
$ cd apkc && go test -run TestResolveSigner -count=1 -v .
=== RUN   TestResolveSigner
--- PASS: TestResolveSigner (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.011s
```

Con el binario real el builder vio el texto nuevo con la ruta ausente, el de formato con `old.jceks`
y ningún aviso con `~/.android/debug.keystore`, donde `auto` elige el backend `go` (**transferido,
no versionado**).

**La deuda de `§264`.** `§264` cerró que el fallback era benigno y dejó la corrección «en curso,
otro frente», pidiendo separar los dos casos en `gopackage.go` y añadir a `TestResolveSigner` un
caso de ruta ausente que exigiera el texto de ausencia. Este `§266`, sobre `0e65ac1`, hace
exactamente esas dos cosas y no reescribe `§264`: lo que allí era defecto de diagnóstico aquí es una
prueba en verde. La observación de `§264` sobre el build sin `--keystore`, que no llama a `findSDKs`
(`apkc/main.go:28-30`), sigue siendo de otro frente y no se toca.

```sh
cd apkc && go test -run TestResolveSigner -count=1 -v .
grep -n 'keystoreAbsentMessage\|keystoreRefusal' apkc/keystore.go apkc/gopackage.go
sed -n '383,438p' apkc/keystore_test.go
git show --stat 0e65ac1
```

## 267. Sin sondeo de SDK, el keystore por defecto queda vacío y el build aborta antes de firmar

Registrado el 2026-11-23 sobre este árbol (`c8dbb67`). La medición es **lectura de código**, no una
ejecución: no se compiló el binario, ni se pasó la suite, ni `go vet`, ni el gate, ni chromium, ni
red, ni push, ni publicación; tampoco se reprodujo el fallo con una línea de comando real. Las citas
`archivo:línea` son de `apkc/` en este árbol y las de `studio/` se comprobaron sobre el pin
`f34c79d`, que es el que declara el submódulo (en este worktree no está inicializado). El hueco que
se describe no se corrige aquí.

**La lista blanca no sondea.** `apkc/main.go:24` enumera los subcomandos que corren sin SDK —`dex`,
`link`, `compile`, `check`, `verify`, `keystore`, `rotate`, `debug-keystore`, `create-keystore`,
`preview`— y no llama a `findSDKs()`. `build` solo lo llama si la línea de comando **no** es el
build sin procesos (`:28-29`), y `default` sondea (`:32`). `sinProcesos` (`apkc/build.go:522`) exige
`--linker go` **y** `--compiler go` (`:540`), el par exacto que usa la pestaña: el único camino que
se queda sin sondeo es justo el que ya estaba pensado para no encontrar SDK.

**El default se asigna en un solo sitio.** `keyStorePath` es un global (`apkc/doctor.go:49`) y el
único sitio que lo escribe es `findSDKs()`, con `os.UserHomeDir()` y `.android/debug.keystore`
(`apkc/doctor.go:162`). Sin ese sondeo la cadena conserva su valor cero, `""`. Lo mismo vale para
`androidJar`, que solo puebla el sondeo.

**La cadena vacía llega hasta la firma.** `set.String("keystore", keyStorePath, …)`
(`apkc/build.go:115`) hereda `""`; `opts()` lo copia a `buildOpts.keyStore` (`:138`) y `:212`
llama a `resolveSigner`. Con `signer=auto` y `!keystoreIsReadable("")`, `resolveSigner`
(`apkc/gopackage.go:78`) escribe el `LogD` de `:90` y devuelve `signerJVM`; con un keystore
legible y API ≥ 18 devuelve `signerGo` (`:96`). El `auto` cambia de backend por el hueco sin decir
que lo que falta es la ruta.

**Y aborta antes de firmar.** `requireKeystore` (`apkc/keystore.go:123`) hace `os.Stat("")`
(`:124`), falla y llama a `LogF` (`apkc/log.go:38-41`, que acaba en `os.Exit(1)`), con el texto de
`keystoreAbsentMessage` (`apkc/keystore.go:101-103`). Con la ruta vacía ese texto queda «there is no
keystore at  - …»: la doble blanca es fea pero inequívoca.

**Ninguna rama lo esquiva.** `apkc/build.go:419-462` pone `requireKeystore` delante de las tres: AAB
(`:421`, con `signAAB` en `:423`), `signerGo` (`:427`) y JVM (`:450`, con `signAPK` en `:452`). No
hay rama que entregue `""` a `signAPK`/`signAAB`, ni `--signer=go` lo evita: el fallo es de puerta y
no de firma, y por eso es benigno para la integridad del paquete.

**El mismo hueco, en el jar.** Sin sondeo, `androidJar` queda `""` salvo `--android-jar`
(`apkc/build.go:128`, `:184-188`); el enlace Go lo exige en `apkc/linker.go:443` y
`opcionesDelProyecto` (`:524-534`) pide `--sdk-version` cuando la ruta no dice la plataforma.

**La pestaña no cae en él.** `studio/pestana.js:3049-3054` arma `build … --keystore <k.ruta>`
siempre; la clave de depuración es `/sdk/debug.keystore` (`:2996`) y `sign` también pasa la ruta
(`:2382-2383`). El wasm no depende de `findSDKs`: el par `--linker go --compiler go` lo saca de
`apkc/main.go:28` y las rutas le llegan por bandera.

**El diagnóstico ya existe; el default sigue vacío.** `0e65ac1`, registrado en `§266`, separó «no
está» de «no se lee» en el mensaje (`keystoreAbsentMessage`, `keystoreRefusal`), y `§264` ya anotó
que el build sin `--keystore` no llama a `findSDKs` (`apkc/main.go:28-30`). Lo que fija este `§267`
es el mecanismo completo: no es que el aviso sea malo, es que el valor por defecto nunca se puebla
fuera del sondeo.

**Veredicto.** Benigno para la firma —no hay ruta que firme con `""`— y fricción de CLI y de e2e: un
guion que invoque `build --linker go --compiler go` sin `--keystore` **ni** `--android-jar` muere en
la puerta. La vía propuesta, **no implementada**, es fijar el default con `os.UserHomeDir()` en
`apkc/main.go:25-30` sin llamar a `findSDKs()`, con su test en `apkc/keystore_test.go`; mientras
tanto el e2e pasa `--keystore ~/.android/debug.keystore --android-jar <jar>`. No se verificó si el
e2e de ApkShare pasa `--keystore`: queda declarado como incertidumbre.

Sin build ni gate: las citas de arriba se reproducen con

```sh
sed -n '24,33p' apkc/main.go
sed -n '518,541p' apkc/build.go
sed -n '157,163p' apkc/doctor.go
sed -n '113,116p;135,140p;184,188p;209,214p;419,462p' apkc/build.go
sed -n '78,97p' apkc/gopackage.go
sed -n '101,127p' apkc/keystore.go
sed -n '36,42p' apkc/log.go
sed -n '440,445p;523,535p' apkc/linker.go
sed -n '2996p;2382,2383p;3049,3054p' studio/pestana.js
git log --oneline -2 0e65ac1
```

## 268. La estática calificada por el nombre de tipo se estima sin emitir el receptor

Registrado el 2026-11-24 sobre `e3a46a1`. La medición es del builder, `d6ae738`, integrada en este
árbol como ese commit; este `§` la **transfiere** y re-ejecuta solo la prueba puntual, no la suite.
Las cifras de suite, `go vet` y build wasm que se citan son salida del builder, no de este registro.

**El error previo.** El build de ApkShare se paraba en `MainActivity.java:212`, columna 31, con
`[build] MainActivity.java: linea 175: linea 176: linea 212: linea 212, columna 31: no se puede
estimar el tipo de createChooser() sin emitir el receptor de la llamada` —posiciones encadenadas por
el envoltorio y el motivo al final—. La línea es la 212 de la fuente real:
`startActivity(Intent.createChooser(sharedIntent, "Compartir con"));`. Para elegir la sobrecarga de
`startActivity(Intent)` hace falta el tipo del argumento, y la estimación lo pedía emitiendo un
receptor que no existe: `Intent` no es local ni campo, sino el nombre de un tipo.

**La ruta.** `vueltaEstaticaCualificada` (`apkc/javaemisor.go:5603`) lee el descriptor de vuelta del
método en el `android.jar` sin escribir ninguna instrucción, y `tipoEstimado` la consulta en `:6030`
en la rama de la llamada con receptor o `super`; ella misma exige que el receptor sea un acceso
(`jacceso`). Repite el guardián de la emisión: el prefijo más largo que resuelve como tipo tiene que
ser el receptor entero (`base == nil`), el primer segmento no puede ser una local
(`e.locales[segs[0]].desc == ""`) y no puede quedar resto (`len(resto) == 0`); la candidata tiene
que ser `static` (`:5636`) y su `Signature` no puede declarar la vuelta con un tipo variable
(`vueltaConTipoVariable`). Si algo no se cumple devuelve «no se estima» y deja la llamada al rechazo
de siempre. El interruptor es `estimaEstaticaCualificada` (`:5348`), encendido en el binario que se
publica.

**La emisión no se tocó.** `exp` ya escribía `invokestatic` contra la clase en la rama de la llamada
estática sobre un nombre de tipo (`:4488-4524`, con el `i2l`/boxing de `argumentosResueltos` cuando
toca) y no hay receptor en la pila, así que la única pieza nueva es la estimación. Los métodos de
instancia siguen por su camino (`:4529-4577`: candidata contra el tipo estático del receptor y
`invokevirtual`/`invokeinterface`/`invokespecial`).

**El oráculo.** `apkc/javaemisor_estatica_cualificada_test.go` compara contra javac 17
(`--release 8 -g:lines,source`) con el mismo dexer:
`TestEstaticaCualificadaComoArgumentoContraJavac17` (`:116`) exige el dex byte a byte y desarma
1 `invokestatic` de `createChooser` y 1 `invokevirtual` de `startActivity`;
`...EnAnonimaContraJavac17` (`:210`) es la línea 212 tal cual dentro de la anónima —instrucciones y
dex salvo el constructor, la diferencia conocida del §100—; `...AnchaContraJavac17` (`:155`) mide el
`i2l` delante del `invokestatic`; y `TestInstanciaCalificadaPorVariableSigueSiendoInvokeVirtual`
(`:295`) es el control: 1 `invokevirtual`, 2 `invokeinterface` y 0 `invokestatic`.

**La falsificación.** `TestEstaticaCualificadaFalsificacion` (`:240`) apaga el interruptor y
reproduce el rechazo de antes —`sin la ruta: Pantalla.java: linea 5: linea 6: linea 7: linea 7,
columna 23: no se puede estimar el tipo de createChooser() sin emitir el receptor de la llamada`—;
`TestEstaticaCualificadaTipoIncorrectoSeRechaza` (`:269`) devuelve el motivo de la candidata —`pc 4
(invokestatic): el argumento 1 de
(Landroid/content/Intent;Ljava/lang/CharSequence;)Landroid/content/Intent; llega entero y el
descriptor pide referencia a java/lang/CharSequence`—, que javac también rechaza.

**El ajuste de otro frente.** `apkc/javaemisor_ancho_argumento_test.go:222-241` mide ahora la guarda
del argumento ancho con **las dos** rutas apagadas (`estimaLlamadaEncadenada` y
`estimaEstaticaCualificada`); sin eso, la variante `cualificado` de su fuente dejaba de rechazar y
el test pasaba a compilar. La variante encendida la fija `...AnchaContraJavac17`.

**Deuda abierta, no arreglada.** Una llamada a un método **estático** a través de una expresión de
instancia (`s.f(...)` con `f` static) no emite el `aload_0; pop` que escribe javac: el camino de
instancia (`:4529-4577`) no mira `accStatic`. No bloquea ApkShare y queda como pregunta abierta.
Tampoco `tipoPrevisto` (`:6054`) estima la estática calificada: su `case jllamada` (`:6182-6184`)
devuelve `false` con `v.base != nil`, lo que solo afectaría a una cuenta que llevara esa llamada
como operando.

**Frontera histórica, cerrada en `§269`.** Con `e3a46a1` ApkShare pasaba la 212 y se paraba en
`MainActivity.java:217`, por el pool de constantes >255 y el `ldc_w` aún no medido. `sed -n '217p'`
devuelve `public void show_loading_dialog(String msg) {`, el método donde caía la siguiente parada;
`§269` registra el `ldc_w` y el build completo con `rc=0`.

**Lo ejecutado aquí.** La prueba puntual, en este worktree: `ok github.com/ajinasokan/apkc 3.673s`
con seis `PASS` —las cinco `...EstaticaCualificada...` y `...InstanciaCalificada...`—, y `gofmt -l`
sin salida sobre los tres ficheros citados. La suite `ok … 177.875s`, `go vet` limpio y
`GOOS=js GOARCH=wasm go build .` OK son del builder y se transfieren sin re-ejecutar.

```sh
cd apkc && go test -run 'EstaticaCualificada|InstanciaCalificada' -count=1 -v .
grep -n 'estimaEstaticaCualificada\|vueltaEstaticaCualificada' apkc/javaemisor.go
sed -n '5343,5349p;5593,5650p;6025,6033p;4488,4524p;4529,4577p' apkc/javaemisor.go
sed -n '222,241p' apkc/javaemisor_ancho_argumento_test.go
grep -n 'func Test' apkc/javaemisor_estatica_cualificada_test.go
sed -n '212p;217p' \
  ~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/src/share/apps/util/MainActivity.java
```

## 269. `ldc_w` cierra la compilación completa de ApkShare

Registrado el 2026-11-24 sobre `570dacb`. La medición es del builder, `87bba91`, integrada en este
árbol como `2ef8b94`; este `§` la **transfiere** y re-ejecuta solo la prueba puntual y las
comprobaciones baratas sobre los artefactos, no la suite ni el build. Las cifras de suite, `gofmt` y
`go vet` que se citan son salida del builder, no de este registro.

**El error previo.** Con el binario de `e3a46a1`, el build se paraba con `[build] MainActivity.java:
linea 35: linea 217: linea 217: el pool de share/apps/util/MainActivity pasa de 255 entradas y ldc_w
no esta medido en este emisor`. La posición es la 217 del fuente —`public void
show_loading_dialog(String msg) {`—, pero el motivo no está en esa línea: el pool de la **clase**
pasó de 255 entradas al añadirse las constantes de `Intent.createChooser`, y el emisor cerraba el
método sin forma larga para `ldc`.

**Las piezas.** `opLdcW = 0x13` (`apkc/javaemisor.go:52`); `ldc(idx)` (`:3006-3017`): hasta 255
`0x12` con un byte, de 256 a 65535 `0x13` con dos —`byte(idx>>8), byte(idx)`— y por encima de
`0xffff` `truncado`; el interruptor `ldcSinFormaLarga` (`:2997`), apagado en el binario y solo para
la falsificación; el comentario de `truncado` (`:2418`); y el rechazo (`:9627`), que ya no dice «no
esta medido» sino «el indice no cabe en el `ldc` que se escribio».

**Sin pasada extra.** El índice es definitivo al emitir porque el pool crece por el final, y las
posiciones que se anotan después —filas de línea, etiquetas, `exception_table`— se toman de
`len(e.codigo)` en ese momento; los saltos se resuelven luego en `resuelveSaltos` (`:2628`).
`max_stack` no cambia: `ldc_w` empuja un valor igual que `ldc`.

**El oráculo.** `apkc/javaemisor_ldcw_escritor_test.go` arma una clase con 200 cadenas y 80 enteros
(`fuentePoolGrande`, `:28-49`) y, tras el límite, pone `conSalto` —rama y `StackMapTable`— y
`conTry` —`exception_table`—. `TestElLdcWEscribeLaFormaLargaYCoincideConJavac17` (`:147`) exige la
premisa sobre el `.class` de javac 17 (`--release 8 -g:lines,source`), la regla del límite sobre
**nuestro** `.class` (`:163-179`: `ldc` solo con índice ≤ 255 y `ldc_w` solo por encima), las pilas
declaradas método a método y el **dex byte a byte**. `TestElPoolCortoSeQuedaEnLdc` (`:209`) es el
control sin `ldc_w`; `TestElLdcEligeLaFormaEnElLimite255` (`:227`) fija `0x12`/`0x13`/`truncado` en
0, 254, 255, 256, 257, `0x7fff`, `0xffff` y `0x10000`; `TestElLdcWNoCambiaElMaxStack` (`:273`) mide
que `ldc`→`ldc_w` alarga el `Code` exactamente un byte.

**La falsificación.** `TestElInterruptorFuerzaLaFormaCorta` (`:257`) enciende `ldcSinFormaLarga` y
el pool grande se rechaza nombrando el pool. Con el binario, la falsificación devuelve el mismo
`file:line` y «el indice no cabe en el `ldc` que se escribio».

**El cierre de ApkShare.** Con el binario de este árbol, sobre
`~/.cache/apkc-web/arraylist-apkshare/ApkShare-main/`, el build termina: 6 fuentes de Java en Go →
21 clases → `build/classes.dex` de **16672 B** → `build/app.apk` de **305577 B**, `rc=0`, `finished
in 97.012316ms`, con el empaquetado y la firma dentro de apkc. **No aparece bloqueo siguiente**: es
la primera vez que ApkShare compila entero. Aquí solo se comprobaron los artefactos —`stat` da los
dos tamaños, 21 `.class` y 6 `.java`—, no se relanzó el build.

**Lo que revisó la crítica.** `ses_df3cbd7c0ffeeGo1IkZxjLAhrA` dio **APROBAR** sin hallazgos
`CONFIRMADO` en contra. Repitió los límites exactos (`0x12` en 255, `0x13 01 00` en 256,
`0x13 ff ff` en 65535 y `truncado` en 65536) con el `Code_length` cuadrado por
`u4be(uint32(len(e.codigo)))` (`:9329`) y la aserción de forma sobre nuestro `.class` (test
`:163-179`), no solo sobre el de javac; y reconstruyó ApkShare **sin caché**
(`mv build/dexcache.bin …bak`, 0 de 63 reutilizados) con el mismo resultado: 21 clases, 16672 B de
dex, `rc=0`. `apksigner verify --verbose` da «Verifies» y v1/v2/v3 en `true`; con
`--v4-signature-file build/app.apk.idsig`, también v4 (medido aquí); `jarsigner -verify` dice «jar
verified».

**Deudas abiertas, no cerradas.** (1) 12 clases anidadas de `MainActivity`/`ApkAdapter$1` no son
byte a byte contra javac **en `.class` crudo** por `StackMapTable` (`full_frame` frente a
`same_frame`), una fila
`line 159: 155` ausente y los `MethodParameters` de las anónimas; no tienen ninguna `ldc_w`, son
preexistentes y no impiden que ApkShare compile (artefactos en `~/.cache/apkc-web/ldcw-w/`).
La compresión de marcos y la `Signature` de la anónima se corrigieron después en `d94be90`
(`apkc/jvmframes.go`, `TestElStackMapDeLaAnidadaEsElDeJavac17`): 12 de 21 clases byte a byte en
`.class` crudo y APK verificado; ese commit no tiene `§` propio y queda aquí como pregunta abierta.
(2)
`apkc/lambda_fuente.go:449-451` conserva un mensaje viejo, que ya solo se dispara con el interruptor
o con un pool de más de 65535. (3) Sin `--keystore` explícito, el paso de firma se queja con la ruta
vacía (§267), aunque el default del flag esté puesto. (4) `ldc_w` junto a un `goto_w` real (más de
32 KB de `Code`) no está medido: la sonda usa rama corta, así que el ensanchado de un salto sobre un
`ldc_w` que ya corrió posiciones no queda ejercitado.

**Lo ejecutado aquí.** La prueba puntual, en este worktree: `ok github.com/ajinasokan/apkc 4.883s`
con seis `PASS` —las cinco del fichero nuevo y `TestElLdcWInternaElTipoYCoincideConD8` (§222), que
entra por el patrón—; el patrón del encargo no casaba con `...Limite255`, así que se añadió.
`gofmt -l` no devuelve nada sobre los dos ficheros citados. La suite `ok … 175.916s`, `gofmt` y
`go vet` limpios son del builder.

```sh
cd apkc && go test -run 'LdcW|PoolCorto|InterruptorFuerza|Limite255' -count=1 -v .
grep -n 'opLdcW\|ldcSinFormaLarga' apkc/javaemisor.go
sed -n '2414,2422p;2580,2625p;2993,3017p;9325,9332p;9623,9630p' apkc/javaemisor.go
sed -n '163,179p;227,252p' apkc/javaemisor_ldcw_escritor_test.go
cd ~/.cache/apkc-web/arraylist-apkshare/ApkShare-main && \
  ~/.cache/apkc-web/ldcw-w/apkc-despues build --linker go --compiler go --sdk-version 34 \
    --android-jar ~/opt/android-sdk/platforms/android-34/android.jar \
    --keystore ~/.android/debug.keystore
stat -c '%s %n' build/classes.dex build/app.apk
find build/classes -name '*.class' | wc -l
apksigner verify --verbose --v4-signature-file build/app.apk.idsig build/app.apk | head -8
jarsigner -verify build/app.apk | grep 'jar verified'
```

## 270. El gate de red suelta los workers con la red ya encendida

Registrado el 2026-11-24 sobre `3380703` (`studio` `54167e6`). La medición es del builder
`ses_df3d9799effePOwHA4oqRVECcH`, aprobada por la crítica `ses_df3c97c3fffe77WoAfvYegHmst`; este `§`
la **transfiere** con sus comandos y **no** re-ejecuta el gate, chromium ni la red. Las cifras de
`prueba-pestana.js` y de la sonda son salida de esas sesiones, no de este registro; aquí solo se
re-ejecutaron las pruebas sin navegador (final del `§`).

**Lo que fijaba `§262` era el estado, no el contrato.** Ese `§` describe
`waitForDebuggerOnStart:false` en `Target.setAutoAttach`; eso era la configuración en el momento de
esa medición, y su propia deuda anotaba «una carrera teórica entre el enganche del worker y su
primer `WebSocket`» (`docs/evidencia.md:11893-11895`). Este `§` mide la carrera, la corrige y cierra
esa deuda para el gate.

**La medición de la carrera, fuera del repo.** La sonda es
`~/.cache/apkc-web/carrera-cdp/sonda-carrera.mjs`: corre el mismo algoritmo que `prueba-pestana.js`
con una sola diferencia conmutable (`SONDA_MODO`, `:8-12`), crea N workers por ronda y cada uno abre
un `wss://` único **en su primer tick** (`:217-222`; `SONDA_RETARDO` existe pero vale 0 por defecto,
`:31-33`). Antes del arreglo (`waitForDebuggerOnStart:false` + `Network.enable` al enganchar), en
4 rondas de 20 workers: **28 vistos de 80, 52 pérdidas (65,0 %)** —`antes-estudio.log`; las rondas
sueltas dan 29/40 (`antes-1.log`) y 41/80 (`antes-2.log`)—. Con el arreglo, 4 rondas: **80/80, 0
pérdidas** (`despues-estudio.log`), y 6 rondas más: **120/120, 0 pérdidas**
(`despues-estudio-2.log`). En las rondas con contador **se enganchan los 20 workers**; lo que se
pierde no es el enganche sino el `webSocketCreated`, porque el script del worker corría antes de que
llegara la respuesta de la habilitación.

**El `setTimeout(400)` de la prueba escondía la carrera, no la evitaba.** Con ese retardo —el que
usa la sonda de `prueba-pestana.js`— y 1 worker por ronda, el builder midió **28/28 en los dos
modos**: el margen temporal tapa el hueco en vez de cerrarlo, así que no sirve como falsificación.
El hueco es de orden: `attachedToTarget → Network.enable` tarda una mediana de 11-15 ms y hasta
48 ms (`antes-2.log`, `despues-estudio-2.log`), mientras el socket sale en el primer tick.

**El cambio (`studio` `54167e6`).** `Target.setAutoAttach` pasa a `waitForDebuggerOnStart:true`
(`studio/prueba-pestana.js:246-253`), plano y antes de navegar: cada target nace pausado. En
`Target.attachedToTarget` (`:198-237`) se encadena, **por sesión**, `Network.enable` y, cuando
responde, `Runtime.runIfWaitingForDebugger` (`:212-236`): primero la red, después soltar el target.
La acumulación sigue sin filtrar por `sessionId` (`:244-245`) y la limpieza sigue en
`Target.detachedFromTarget` (`:239-243`).

**El `service_worker` es la excepción deliberada.** Pausado no contesta `Network.enable` hasta que
arranca; esperarlo lo dejaba congelado y rompía «el service worker queda listo». Se le manda la red
y se le suelta sin esperar, y la respuesta se anota cuando llega (`:219-224`). Una corrida ingenua,
esperándolo como a un `worker`, dio **231 bien, 12 mal** (medición del builder).

**El plazo de 2000 ms.** Alrededor de `Network.enable` hay un `Promise.race` con `esperar(2000)`
(`:226-229`): son 100 veces el máximo medido en la sonda (52 ms, `antes-2.log`) y es red de
seguridad, no margen de la carrera. Si no responde, se suelta el target igual y su sesión cuenta
como «sin red» en la cobertura (`:216-217`, `:231-235`), en vez de colgar la página.

**La cobertura solo dice «sí» con respuesta sin error.** `anotar` (`:213-218`) marca `red: "sí"`
únicamente cuando llega la respuesta y no trae `error`; un plazo vencido o un error dejan
`"no: …"` y la sesión no cuenta como cubierta.

**El gate, con Brave y los fixtures completos.** **249 comprobaciones bien, 0 mal**, y la cobertura
dice `1 page, 1 service_worker, 2 worker` (`gate-despues-2.log`; el contador, `:6449` en el árbol
del builder). La falsificación `APKC_GATE_SIN_WORKERS=1` da **247 bien, 2 mal**: fallan
exactamente la sonda del worker —«no se vio ningún WebSocket del worker: … escapó al gate
(auto-attach apagado a propósito)»— y la cobertura, que se queda en `1 page`
(`gate-sin-workers-2.log`). El gate por defecto sigue en cero ajenos: **488 peticiones, 0
websockets**, todas locales.

**La falsificación de Node.** `studio/prueba-wisp-gates.mjs:145-153` reproduce la regla vieja —mirar
solo los mensajes sin `sessionId`—: deja **1 websocket visto y 0 ajenos**, es decir, el `wss://` del
worker escapa del registro; es justo lo que se quiere falsificar. Con la regla actual el `wss://`
entra y se clasifica ajeno.

**Lo ejecutado aquí, sin navegador.** En la copia del builder sobre este puntero: `node --check
studio/prueba-pestana.js` sin salida; `node studio/prueba-wisp-gates.mjs` termina con
`falsificación: los dos gates rechazan lo que tienen que rechazar`; y `node studio/prueba.js` con
`prueba: todo en orden`. La suite Go no se toca: no hay cambio en `apkc/`.

**Preguntas abiertas, no bloqueantes.** (1) La anotación asíncrona del `service_worker` no tiene
plazo: un `Network.enable` que no conteste nunca dejaría su entrada en `"pendiente"` y la cobertura
la contaría igual; es **INCIERTO sin chromium**. (2) Los recuentos 249/0 y 247/2 se remiten a las
corridas del builder sobre este puntero y no se repiten aquí. (3) El oráculo completo
(`tools/entorno.sh gate`) sigue **sin ejecutar por orden del usuario**; tampoco hubo `apksigner` ni
ART nuevos.

```sh
cd ~/.cache/apkc-web/carrera-cdp
SONDA_MODO=antes   SONDA_RONDAS=4 SONDA_WORKERS=20 CHROME_BIN=/usr/bin/brave \
  node sonda-carrera.mjs                     # antes: 28/80 vistos, 52 pérdidas (65,0 %)
SONDA_MODO=despues SONDA_RONDAS=6 SONDA_WORKERS=20 CHROME_BIN=/usr/bin/brave \
  node sonda-carrera.mjs                     # después: 120/120, 0 pérdidas
# el gate, sobre studio/ con los fixtures, en dos corridas
STUDIO_SERVIDOR_PORT=… STUDIO_PORT=… CHROME_BIN=/usr/bin/brave \
  node studio/prueba-pestana.js              # 249 bien, 0 mal
APKC_GATE_SIN_WORKERS=1 STUDIO_SERVIDOR_PORT=… STUDIO_PORT=… \
  CHROME_BIN=/usr/bin/brave node studio/prueba-pestana.js   # 247 bien, 2 mal
# las que sí se re-ejecutaron aquí, sin navegador
node --check studio/prueba-pestana.js
node studio/prueba-wisp-gates.mjs
node studio/prueba.js
```

## 271. El hospedador de wasm propaga el código de salida sin cortar la salida

Registrado el 2026-11-24 sobre `f07b11c`. La corrección viene del builder
`ses_df3dd2a56ffeOr4xU7Hdu9nQXN`, con las críticas de `ses_df3d6e48fffeBzUhikJmJMhYLq`, y este `§`
la **transfiere**: aquí se re-ejecutó `shim/prueba-salida.js` contra el shim del árbol y contra las
dos copias viejas; las cifras de la recrítica —socket abierto, stderr saturado, `create /miapp`— son
suyas y se citan sin re-ejecutar.

**El síntoma.** Un programa Go que hacía `os.Exit(1)` dentro de `shim/navegador-sim.js` terminaba
con rc 0, y si además quedaba un handle vivo —el binario de tests de apkc dejaba uno— node no
terminaba: se quedaba en `do_epoll_wait` con «(apkc devolvio 1)» ya impreso hasta que alguien lo
mataba con SIGTERM (rc 143; 124 si el que mataba era el `timeout`). La causa era el orden: el exit
falso resolvía `process` en el momento de llamarlo, ya sustituido por `globalThis.process`, así que
solo anotaba el código y no salía nadie.

**La primera corrección (`bb0f622`).** `shim/navegador-sim.js:17-24` captura `procesoReal`,
`salirReal` y un `procReal` con los flujos reales **antes** de sustituir `globalThis.process`;
`salirCon` —`:113-126` en `bb0f622`, ampliado luego a `:113-149`— fija el código y sale por el exit
real, y `exportarYResumir` lo usa al exportar (`:449`) en vez de dejar que el proceso muriera por su
cuenta. Un `go.exit` aparte (`:420-431`) sigue anotando el código y resolviendo `_exitPromise` para
que `wasm_exec` no se quede con un promise sin terminar.

**La segunda corrección (`f07b11c`).** La primera versión esperaba la salida con un temporizador de
1 s que además iba con `unref()`: con un pipe moroso el proceso se vaciaba antes de que stdio
acabara de drenar y se perdía el final. La sonda de 200 KB con lector lento (512 B cada 4 ms) dio
`137216/204810` bytes y sin `(apkc devolvio 1)`. El `write("")` de barrera tampoco servía: node
llama a su callback sin esperar a los trozos anteriores —medido, salía a los 300 ms con dos tercios
del buffer todavía dentro—. El arreglo (`:113-149`) sondea cada 20 ms
`stdout.writableLength + stderr.writableLength` y solo sale cuando **los dos** están a 0; el sondeo
va **sin** `unref()`, con referencia a propósito, y hay un plazo de 120 s para el consumidor muerto
que avisa por `fs.writeSync(2)` —sincrónico, para que el aviso salga aunque el pipe no drene— y
sale igualmente con el código de Go.

**La prueba.** `shim/prueba-salida.js` compila cuatro sondas fuera del repo, en
`~/.cache/apkc-web/prueba-salida/`: una que sale con 1, la misma con un `setInterval` vivo, una que
copia un fichero montado a otro exportado y una que escribe 200 KB con `FIN-200KB` al final. Corre
el shim con `spawnSync` y 20 s de límite (120 s para la lenta) y hace 10 comprobaciones: rc 1 y
aviso del código; rc 1 **sin** SIGTERM con el handle vivo; rc 0 y contenido exportado idéntico;
rc 1, los 200 KB enteros, `FIN-200KB` y el aviso final por un pipe moroso. El caso 4 lanza el lector
**en otro proceso** y le pasa el stdout del shim como stdin: con el lector en el mismo proceso, sus
buffers de 64 KB se quedaban con casi toda la salida y el defecto no se veía. La variable `SHIM=`
permite apuntar el guion a otra copia del hospedador.

**Lo ejecutado aquí.** En este worktree, con `tcr -c 4 -m 3G node shim/prueba-salida.js`: las 10
comprobaciones en verde, con `201067` bytes en el caso 4. La falsificación, con las dos copias
viejas extraídas con `git show`: el shim de `bb0f622` deja **3 fallos** —`133752` bytes,
`FIN-200KB` y el aviso final ausentes—; el shim original (`4e2f8ec`) deja **4 fallos**: rc 0 en el
caso 1, `SIGTERM` en el caso 2 —el cuelgue en `epoll_wait`— y rc 0 otra vez en el caso 4, donde los
200 KB sí llegan porque el `process.exit` falso no esperaba nada. `node --check` sale limpio en
`shim/navegador-sim.js` y en `shim/prueba-salida.js`.

**La recrítica, transferida.** Sobre la sonda original de 200 KB: `204867/204810` bytes,
`FIN-200KB: True`, aviso final `True`, rc 1. Con un consumidor muerto y un socket abierto: rc 1 a
los 120.1 s con el aviso de `fs.writeSync(2)`. Con stderr saturado: aviso entregado a los 120.2 s.
`create /miapp` deja sus 5 ficheros, `node --check` limpio y la regla 5 limpia.

**Frontera viva, no bloqueante.** Un productor que altere `writableLength` cada menos de 120 s con
el consumidor muerto mantendría vivo el plazo indefinidamente; el plazo se midió con pipes, no con
un socket TCP real. Y la capa `wasm` sigue en rojo por causas ajenas a este frente (§ de cabecera):
lo que este `§` garantiza es que el hospedador termine con el código real en vez de colgarse o
truncar la salida, no que la suite pase.

```sh
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" shim/wasm_exec.js
node shim/prueba-salida.js
mkdir -p "$HOME/.cache/apkc-web/falsif/bb0f622" "$HOME/.cache/apkc-web/falsif/original"
git show bb0f622:shim/navegador-sim.js > "$HOME/.cache/apkc-web/falsif/bb0f622/navegador-sim.js"
git show 4e2f8ec:shim/navegador-sim.js > "$HOME/.cache/apkc-web/falsif/original/navegador-sim.js"
cp shim/wasm_exec.js "$HOME/.cache/apkc-web/falsif/bb0f622/"
cp shim/wasm_exec.js "$HOME/.cache/apkc-web/falsif/original/"
SHIM="$HOME/.cache/apkc-web/falsif/bb0f622/navegador-sim.js" node shim/prueba-salida.js
SHIM="$HOME/.cache/apkc-web/falsif/original/navegador-sim.js" node shim/prueba-salida.js
node --check shim/navegador-sim.js
node --check shim/prueba-salida.js
```

## 272. La capa wasm monta el jar y el fixture, y captura sin tubería (`e3c7259`)

Registrado el 2026-11-24 sobre `e907433`. La medición es del builder que dejó `e3c7259`; este `§` la
**transfiere**: cita sus logs (`~/.cache/apkc-web/wasm-antes.log`, `wasm-sin.log`, `wasm-con.log`,
`wasm-despues.log`) y no re-ejecuta la capa. Los rojos y verdes que se dan son de esas corridas.

**El síntoma era el fs simulado, no el código.** `tools/entorno.sh wasm` lanzaba
`shim/navegador-sim.js` sin `--montar`, así que el fs en memoria nacía vacío y las pruebas que abren
el SDK o el fixture morían con ENOENT aunque el emisor estuviera bien. `wasm-antes.log`: **33 `FAIL`
de primer nivel** —25 con `sin android.jar que abrir: exporta APKC_ANDROID_JAR …`, uno con
`open testdata/vista-propia/main.xml: No such file or directory`, **6** líneas de
`pipe: not implemented on js` (4 subtests de `TestAvisoDeColisionDeNativos…`, `TestResolveSigner` y
`TestSignAvisaAlDescartar…`) y cuatro de otra causa: la guarda de `dependenciasDexeables` en un
proceso hijo, el manifiesto inyectado, los permisos del fichero de contraseña y la sonda de WISP—.

**El jar, con la forma del SDK.** `tools/entorno.sh:121-126` monta el android.jar de `jar_del_sdk`
(`:60-67`, la misma definición que usan `doctor` y la tarea `jar`) en
`/android/platforms/android-34/android.jar`, bajo el `ANDROID_HOME` que el hospedador publica. La
forma importa: en `/sdk/android.jar` suelto, `check` y el enlazador no deducen la API del directorio
padre; `wasm-despues.log` lo enseña con cinco rojos de `check` y el aviso
`la ruta del android.jar no dice la version del platform; digala con --sdk-version`; esos cinco
desaparecen al mover el destino. El fixture viaja en el mismo `--montar` (`:128`) y el destino va
absoluto porque `--montar` expande también el destino.

**El fixture y el jar se miden por separado.** `wasm-sin.log`, con el fixture montado y el jar no,
deja **29 rojos** (25 de ellos por el jar); `wasm-con.log`, con los dos, deja **2**. Si falta el jar
el aviso es explícito y no se salta nada: `aviso: sin android.jar; las pruebas contra el SDK
fallarán en wasm` (`:125`).

**La captura, por fichero.** `capturarSalida` (`apkc/sign_hostil_test.go:91-110`) redirigía
`os.Stdout` a `os.Pipe`; `os/pipe_wasm.go` devuelve ENOSYS y la prueba moría con
`pipe: not implemented on js`. Ahora usa `os.CreateTemp(t.TempDir(), "salida-*.log")` (`:94`),
lee el fichero al cerrarlo y restaura `os.Stdout` con `defer` (`:99`): la aserción sobre lo escrito
es la misma y también vale en nativo. Los tres usos (`sign_hostil_test.go:54`,
`aar_real_test.go:379`, `keystore_test.go:428`) dejan de depender de una tubería, y `wasm-con.log`
ya no registra ningún `pipe: not implemented`.

**Lo que queda no es de este cambio.** Con los montajes, los 2 rojos son
`TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase` —la guarda conduce un proceso
hijo y en wasm no hay proceso: la salida llega vacía— y
`TestCompilarEnGoDexaIgualQueJavacConUnaDependencia` (`sin javac 17: exporta JAVAC17 …`). Después
del `§`, el run **aborta** en `TestRecursosDeDependenciaEnElApk` al abrir
`build/res/layout/deps_fila.xml` (`apkc/deps_recursos_test.go:233`): es el fs simulado de otro
frente. `f64101e` separa por build tag las pruebas que no corren en la pestaña y deja la capa en 116
con 0 rojos, con ese aborto abierto; no se verifica aquí.

**Preguntas abiertas.** (1) ¿Sigue el aborto de `TestRecursosDeDependenciaEnElApk` en `e907433`?
**INCIERTO**: no se re-ejecutó la capa. La vía es `sh tools/entorno.sh wasm`, que compila el wasm y
lo corre en el hospedador de Node, sin chromium ni red. (2) `wasm-antes.log` no trae el total de
pruebas, solo los rojos; el 118/116 es del mensaje de `f64101e` y aquí no se usa como medición.

```sh
ANDROID_HOME="$HOME/opt/android-sdk" sh tools/entorno.sh jar 34   # ruta del jar que se monta
sh tools/entorno.sh wasm        # compila el wasm y corre la suite en el hospedador
grep -n 'MONTAJES\|android-34/android.jar\|aviso: sin android.jar' tools/entorno.sh
sed -n '86,110p' apkc/sign_hostil_test.go
sed -n '225,240p' apkc/deps_recursos_test.go
```
## 273. El oráculo javac 17 no escribe `MethodParameters` en las anónimas

Registrado el 2026-11-24 sobre `d0610bf` (corrección `d123135`). Viene del builder
`ses_df3a97d2dffeSWivnPlU52wn50`, con la investigación `ses_df3ac6d4effeRzuYqiLJzx2xxD` y la
crítica `ses_df38ccb99ffeoZv5oEopy5SHn9`. Este `§` **transfiere** el cierre: aquí se re-ejecutaron
las sondas de javac 17.0.21 y 21.0.13, se recontaron los artefactos de
`~/.cache/apkc-web/mp-probe/` y se comprobaron los `file:line`; el build de ApkShare, la suite y
`apksigner` son del builder y se citan sin re-ejecutar.

**El defecto.** Antes de `d123135`, 7 de las 21 clases de ApkShare llevaban `MethodParameters` en
nuestro `javap -v`; javac 17.0.21 —el oráculo del repo, el JDK que fija la CI (§152)— no lo escribe
en **ninguna**. El bloque estaba en `apkc/javaemisor.go:9090-9106` con la condición
`!estatica && !bajoAnonima`: copiaba la medición del párrafo 42, hecha con javac 21.

**La aclaración de javac 21.** javac 21.0.13 sí lo escribe, pero solo en la anónima de un método de
instancia: el `this$0` va como `final mandated` (0x8010), y si además hay capturas de locales, una
segunda fila `final synthetic` (0x1010). Nunca en la anónima de un `static`. La sonda de la crítica
era `static`, de ahí la duda; la causa de las 7 clases era nuestro emisor, no javac 21.

Sonda reproducida aquí (`Sonda$1` es la de instancia con captura, `Sonda$2` la de instancia con solo
`this$0`, `Sonda$3` la de `static` con captura):

- `Sonda$1` `(Lp/Sonda;I)V`: javac 17, sin `MethodParameters` y `Signature: ()V`; javac 21, dos
  filas y `Signature: ()V`.
- `Sonda$2` `(Lp/Sonda;)V`: javac 17, sin `MethodParameters` ni `Signature`; javac 21, una fila
  (`final mandated`) y sin `Signature`.
- `Sonda$3` `(I)V`: javac 17, sin `MethodParameters` y `Signature: ()V`; javac 21, sin
  `MethodParameters` y `Signature: ()V`.

La `Signature: ()V` depende solo de que el constructor reciba capturas de locales: aparece en `$1` y
`$3` con los dos JDK, con `this$0` o sin él, con `static` o sin él, y también sin `--release`
(major 61 con javac 17, compilado aquí). Una constante local plegada no es captura: `Const2.m` usa
`final int k = 5` en un `static` y su anónima sale `()V`, sin `MethodParameters` y sin `Signature`.

**El cambio (`d123135`, ya integrado).** `javaemisor.go:9083-9092` retira el atributo y deja solo la
nota del porqué; se elimina la constante `accMandated`; la `Signature: ()V` queda intacta
(`len(capturas)>0`, `:9093-9101`). Los tests comparan contra el oráculo 17 sin normalizar:
`javaemisor_captura_receptor_test.go:100` (`comparaDexComoJavac17`), `:170` exige
`Parameters=false` en los dos lados conservando `Signature=="()V"`, y `javaemisor_test.go:540`
invierte la aserción. `classreader_javac17_test.go:99,119` añade fixtures major 52; los fixtures
major 65 de `classreader_test.go` (javac 21) se conservan a propósito para las tablas del dexer
contra `d8`.

**Lo recontado aquí.** Sobre los artefactos del probe: 21 clases en cada directorio; con
`MethodParameters` 7 en `clases-antes`, 0 en `clases-despues`, 0 en `clases-final` y 0 en `javac`.
El dex baja de 16668 a 16520 B y el del oráculo mide 16504 (16 B de diferencia); `app.apk` mide
305493 B. Del builder: 16 de las 21 clases dexan byte a byte como javac 17 (las 12 anteriores se
medían sobre `.class` crudo),
`apksigner verify --verbose` con v1/v2/v3 en `true`, suite `ok … 163.880s` y `go vet` limpio.

**Las 5 que quedan** no bloquean la compilación: `access$000` frente a lambda en `MainActivity`,
`sipush`/`i2l` frente a `ldc2_w 500L` en `$3`, `invokeinterface` frente a `invokevirtual` en
`Editable.toString`, `astore` frente a `astore_3` en `$6`, y la clase externa triada. Todas
abiertas, no se adjudican aquí.

**La crítica.** `ses_df38ccb99ffeoZv5oEopy5SHn9` dio **APROBAR con condiciones**. (1) cerrada en
`d0610bf`: los cuatro comentarios obsoletos que decían «se normaliza» —tres tests y `§263`— ya no lo
dicen. (2) este `§` aporta los comandos que reproducen las cifras, que solo constaban en el mensaje
del commit. (3) **no ejecutada**: la falsificación de `len(capturas)>0` (quitar la `Signature` y ver
si algún test la echa en falta); queda como pregunta abierta, no como cierre.

**Pregunta abierta.** ¿Algún caso de javac 17 escribe `Signature` sin capturas de locales? Las
sondas de aquí no lo encuentran: solo la llevan los constructores con `val$x`, y la constante
plegada no cuenta como captura.

```sh
# la reproducción completa (builder; aquí no se relanzó): build de ApkShare y conteo
cd apkc && go build -o "$HOME/.cache/apkc-web/s273/apkc" .
cd ~/.cache/apkc-web/arraylist-apkshare/ApkShare-main && \
  "$HOME/.cache/apkc-web/s273/apkc" build --linker go --compiler go --sdk-version 34 \
    --android-jar ~/opt/android-sdk/platforms/android-34/android.jar \
    --keystore ~/.android/debug.keystore
find build/classes -name '*.class' | wc -l
for f in $(find build/classes -name '*.class'); do
  ~/opt/jdk17/bin/javap -v "$f" | grep -q 'MethodParameters:' && echo "$f"
done

# sondas: instancia con captura ($1), instancia solo con this$0 ($2), static con captura ($3)
~/opt/jdk17/bin/javac --release 8 -g:lines,source -d "$HOME/.cache/apkc-web/s273/p17" \
  "$HOME/.cache/apkc-web/s273/src/p/Sonda.java"
~/opt/jdk21/bin/javac --release 8 -g:lines,source -d "$HOME/.cache/apkc-web/s273/p21" \
  "$HOME/.cache/apkc-web/s273/src/p/Sonda.java"
for f in 'p/Sonda$1.class' 'p/Sonda$2.class' 'p/Sonda$3.class'; do
  ~/opt/jdk17/bin/javap -v "$HOME/.cache/apkc-web/s273/p17/$f" |
    grep -E 'MethodParameters:|Signature:'
  ~/opt/jdk21/bin/javap -v "$HOME/.cache/apkc-web/s273/p21/$f" |
    grep -E 'MethodParameters:|Signature:'
done

# el probe del builder, recontado aquí sobre sus artefactos
cd ~/.cache/apkc-web/mp-probe
for d in clases-antes clases-despues clases-final javac; do
  n=0
  for f in $(find "$d" -name '*.class'); do
    ~/opt/jdk17/bin/javap -v "$f" | grep -q 'MethodParameters:' && n=$((n+1))
  done
  echo "$d: $n de 21"
done
stat -c '%s %n' dex-antes.apkc dex-final.apkc dex-javac.apkc apkshare/build/app.apk
```

