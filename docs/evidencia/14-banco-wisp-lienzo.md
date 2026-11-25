# Evidencia — §274-§314, §348

## 274. El banco no es verificable en este escritorio; su mitad local sí

Medido el 2026-11-24 por el QA `ses_df3bea524ffeDQGKHjPG2s75Ny` sobre `HEAD d0610bf` —la sesión
arrancó en `29f6ace` y otro commit llegó con la corrida en marcha—. Este `§` **transfiere** esa
medición: aquí se re-ejecutaron la mitad local y el corte de entorno, con las cifras idénticas; los
cuatro veredictos del aparato siguen sin ejecutarse, allí y aquí.

**El corte.** `tcr -- sh tools/entorno.sh banco` sale con `rc=1` antes de compilar nada, en la
guarda de `banco/banco.sh:62`:

```text
banco: ni rish (Shizuku) ni sudo en el PATH; sin uno de los dos el banco no puede ejecutar dex
```

La guarda busca `rish` (`:31`) y, si no, un `sudo` que valide sin contraseña (`:33`,
`sudo -n true`). En este escritorio no hay `rish`; `/usr/sbin/sudo` existe pero responde
«a password is required» con `rc=1`, así que `RISH` queda vacío y el guion aborta. `pidof zygote64`
sale con `rc=1` —no hay `app_process64` al que entregar el dex— y `adb devices` imprime solo
`List of devices attached`, sin un aparato. El cell no estaba activo y el agente no lo arrancó.

**Segunda limitación, también de entorno.** Aunque hubiera root, el guion no correría aquí sin
adaptarlo: `banco/banco.sh:15` fija `P=/data/data/com.termux/files/usr`, que no existe en un
escritorio, y `:64-67` exige que `TRABAJO` cuelgue de `$P/tmp`, y si no aborta con
`banco: TRABAJO tiene que estar bajo $P/tmp`. No se adaptó el guion: es una decisión de diseño, no
un cierre de este `§`.

**Lo que sí se ejecutó.** Con `apkc` compilado del árbol
(`cd apkc && go build -o ~/.cache/apkc-web/banco/apkc .`, `rc=0`), JDK 21, `build-tools/36.0.0` y
`android-34/android.jar`, los pasos 1, 1b, 2, 3 y 6 del banco —todo lo que no sale del
escritorio, con los dos `d8` del 5 comparados solo por tamaño— dieron esto, con la sonda del QA
repetida aquí sin cambios:

- `jvm (javac)`: `rc=102`, con la salida del programa de prueba
  `total 7|total 9|total 102|10|3|2|4|7|-1|10|6|8|306|22|7|10|101|8|3.0|6|7|10|32|2|21|21|21|9|6|5`.
- `nuestro.dex: 7192 bytes`, `todo.dex: 7192 bytes`, `d8(javac): 7064 bytes` y
  `d8(nuestra Ver): 6920 bytes`.
- `jvm con nuestra clase emitida` (`-Xverify:all`): `rc=102` y salida **idéntica** a la de la JVM.

La tercera línea es la que sostiene la mitad local: los `.class` que emite el compilador pasan la
verificación estricta de la JVM y dan el mismo resultado que javac. Lo que no se midió es lo que
solo el aparato puede decir: si ese mismo código **corre** en ART.

**Veredicto: NO VERIFICABLE, ni verde ni rojo.** Los cuatro `veredicto()` de
`banco/banco.sh:154-157` —`nuestro dex (Go de punta a punta)`, `d8 sobre javac`,
`d8 sobre nuestra Ver` y `nuestro dex con nuestro main (sin javac ni d8)`— no llegaron a
ejecutarse: suben el dex a `/data/local/tmp`, lo corren con `app_process64` y lo verifica
`dexdump`; sin `rish` ni `sudo` no hay ninguno de los tres. El fallo es de entorno, no del
código, y por eso no se escribe como rojo.

**Preguntas abiertas, no cierres.** (1) ¿Se adapta `banco/banco.sh` a un escritorio —`:15` y
`:64-67`— para poder apuntar la mitad del aparato a un emulador? Aquí no se tocó. (2) Mientras no
haya un cell con Termux y `rish` o `sudo`, ¿los cuatro veredictos del aparato quedan fuera del gate,
o se sustituyen por otra medida que no los necesite?

**Reproducción.** El corte se reproduce en cualquier escritorio sin aparato. La mitad local son los
pasos 1, 1b, 2, 3, 5 y 6 de `banco/banco.sh` sin ninguna llamada a `enAparato` (`:54-59`); los seis
van **inlinados** abajo, copiados de la sonda del QA tal como se ejecutó aquí, para que el comando
no dependa de un fichero de `~/.cache/apkc-web/` que la regla 12 permite borrar. `JDK`, `BT` y `JAR`
son las rutas de este escritorio: en otro hay que ajustarlas. Esto es la mitad local, **no** el
banco completo: el banco entero necesita un cell con Termux y root, o Shizuku.

```sh
# 1. el corte: rc 1 en la guarda de :62, antes de compilar
tcr -- sh tools/entorno.sh banco
command -v rish; sudo -n true; pidof zygote64; adb devices

# 2. la mitad local (pasos 1,1b,2,3,5,6 sin enAparato), desde la raíz del repo
sh -c 'cd apkc && go build -o "$HOME/.cache/apkc-web/banco/apkc" .'
AQUI=$(pwd); APKC=$HOME/.cache/apkc-web/banco/apkc
JAR=$HOME/opt/android-sdk/platforms/android-34/android.jar
BT=$HOME/opt/android-sdk/build-tools/36.0.0
JDK=$HOME/opt/jdk21
T=$HOME/.cache/apkc-web/banco-sonda
set -e
rm -rf "$T"
mkdir -p "$T/jc" "$T/scr" "$T/nuestro" "$T/d8" "$T/d8desde" "$T/todo"
"$JDK/bin/javac" -d "$T/scr" "$AQUI/banco/src/p/Ver.java"
"$JDK/bin/javac" -cp "$T/scr" -d "$T/jc" "$AQUI/banco/src/p/Arranque.java"
"$APKC" compile --android-jar "$JAR" -o "$T/nuestro" \
  "$AQUI/banco/src/p/Ver.java" >"$T/compile.log" 2>&1
cp "$T/jc/p/Arranque.class" "$T/nuestro/p/"
"$APKC" compile --android-jar "$JAR" -o "$T/todo" \
  "$AQUI/banco/src/p/Ver.java" "$AQUI/banco/src/p/Arranque.java" >"$T/todo.log" 2>&1
"$APKC" dex -o "$T/todo.dex" "$T/todo" >"$T/tododex.log" 2>&1
"$APKC" dex -o "$T/nuestro.dex" "$T/nuestro" >"$T/dex.log" 2>&1
"$BT/d8" --release --min-api 21 --lib "$JAR" --output "$T/d8" \
  "$T/scr/p/"*.class "$T/jc/p/Arranque.class" >/dev/null 2>&1
"$BT/d8" --release --min-api 21 --output "$T/d8desde" "$T/nuestro/p/"*.class \
  >/dev/null 2>&1
set +e
out=$("$JDK/bin/java" -cp "$T/scr:$T/jc" p.Arranque 2>&1); jvm=$?
set -e
echo "  jvm (javac): rc=$jvm, salida: $(printf '%s' "$out" | tr '\n' '|')"
echo "  nuestro.dex: $(wc -c < "$T/nuestro.dex") bytes; todo.dex: $(wc -c < "$T/todo.dex") bytes"
dn=$(wc -c < "$T/d8/classes.dex"); dnn=$(wc -c < "$T/d8desde/classes.dex")
echo "  d8(javac): $dn bytes; d8(nuestra Ver): $dnn bytes"
mkdir -p "$T/mix/p"
cp "$T/jc/p/Arranque.class" "$T/mix/p/"
cp "$T/nuestro/p/Ver"*.class "$T/nuestro/p/Operable.class" "$T/mix/p/"
set +e
mix=$("$JDK/bin/java" -Xverify:all -cp "$T/mix" p.Arranque 2>&1); mrc=$?
set -e
if [ "$mrc" = "$jvm" ] && [ "$mix" = "$out" ]; then
  echo "  jvm con nuestra clase emitida: rc=$mrc y salida igual a la JVM  ok"
else
  echo "  jvm con nuestra clase emitida: rc=$mrc contra $jvm  FALLO"; exit 1
fi

# 3. con aparato (Termux y root o Shizuku): el banco entero, con los cuatro veredictos
sh -c 'cd apkc && go build -o "$PREFIX/tmp/apkc-gate" .'
tcr -j 1 -q -- sh tools/entorno.sh banco
```

## 275. La constante `String` heredada se dobla con su literal, no como entero (`c56c51f`)

Registrado el 2026-11-24 sobre `f64101e`. Transfiere la medición del builder
`ses_df2a841bfffeaTS26JZz0Fjr33`, con la crítica `ses_df3e5cc0affe6QphBClHbIz3CU`, commit
`c56c51f`, que es ancestro de este árbol (`git diff c56c51f HEAD -- apkc/javaemisor.go
apkc/javaemisor_cadena_heredada_test.go` sale vacío). Aquí solo se ejecutaron los `grep`
del bloque de comandos: sin compilar, sin gate, sin chromium, sin red, sin push y sin
publicación. Las cifras de la suite son del builder y van marcadas como **transferidas, no
versionadas**.

**El síntoma.** La crítica del frente `98dd864` —el que dobló la constante `String` del
classpath en `§265`— confirmó que `Hijo extends Base`, con un `static final String V = "hola"`
en `Base` del classpath y `String dame() { return V; }`, emitía `iconst_0` con descriptor
`Ljava/lang/String;`. El análisis de marcos lo paraba con el literal `pc 1 (areturn): se
esperaba una referencia y hay entero`. Antes de `98dd864` se rechazaba limpio; después, el
mismo nombre suelto pero heredado producía bytecode inválido en vez de un error.

**El criterio: doblar, no rechazar.** El valor sí está disponible: `buscarCampo` recorre la
clase y sus padres (`apkc/javaemisor.go:2077-2078`) y ya lee el `ConstantValue` tag 8 dejándolo
en `campoResuelto.cadena` (`:2148-2156`), porque la rama de `cpString` de `§265` no distingue
si el campo es de la clase actual o de un padre. Lo que faltaba era que el camino del nombre
suelto mirase `esCadena`. El fail-safe queda al revés que antes: un descriptor que no viaja
como entero y no es cadena se rechaza explícitamente (`:4300-4302`, «la constante %s no se
puede doblar con el descriptor %s»), y nunca se emite un `iconst_*` con descriptor de
referencia.

**Los cambios.** `campoDeLaClaseActual` pasa a `(string, bool, error)` (`:4279`) para poder
propagar el rechazo; la rama `cr.esCadena` (`:4285-4294`) emite `e.ldc(e.pool.cadena(cr.cadena))`
y `e.sube()` (`:4291-4292`) y devuelve el descriptor de la cadena. El interruptor de la
falsificación es `pliegaCadenaHeredada` (`:2070-2075`), encendido en el binario que se publica;
apagado revive el defecto —`e.cargaEntero(cr.valor)` con `valor` a cero, que es el `iconst_0`
del síntoma— y solo lo apaga la prueba. El llamador de `:3950` propaga el error como «al leer
%s, %w» (`:3952`).

**El oráculo.** `apkc/javaemisor_cadena_heredada_test.go` compila con `javac 17` un `Base` en
un jar y un `Hijo` que lo extiende, y exige el dex **byte a byte**
(`TestCadenaEstaticaHeredadaContraJavac17`, `:85`); además carga nuestro `Hijo` en la JVM con
`-Xverify:all` desde una sonda (`verificaHeredadaContraLaJVM`, `:160`), comprueba 1 `ldc` de
`shola` y 0 `getstatic` en `dame()` (`:112-127`), que el control `static final int` heredado se
sigue plegando sin `getstatic` (`:130-136`) y que un `static final String` **sin**
`ConstantValue` sale por `getstatic` (`mutable`, `:140-145`).
`TestCadenaEstaticaHeredadaFalsificacion` (`:210`) apaga el interruptor y exige el error
literal del síntoma, precedido de `Hijo.java: `.

**Lo ejecutado por el builder, transferido.** La sonda `-overlay` original pasa con
`heredada v: nuestro 18 176 | javac 18 176` —18 instrucciones y 176 bytes por los dos caminos—.
La regresión acotada tardó `21.820 s`; la suite completa, `ok … 182.423s`, con `go vet` limpio.
Es medición del builder, salida transferida, no versionada; la crítica sí ejecutó los tests
puntuales y la sonda, y aquí no se re-ejecutó ninguno.

**Frontera abierta, no bloqueante.** Un `ConstantValue` tag 8 con índice 0 o con el utf8
ausente en un jar artesanal podría emitir `ldc ""`: `stringValue` (`apkc/classreader.go:566-572`)
devuelve la cadena vacía para un índice que no está, y el emisor (`apkc/javaemisor.go:2154-2156`)
no valida el índice antes de copiarlo. Queda fuera del classpath de confianza del modelo de
amenazas. La constante heredada de una clase **del fuente** sigue rechazándose limpio; el
informe lo atribuye a `TestSondaHeredadaFuente`, que no existe en este árbol
(`grep -rn 'TestSondaHeredadaFuente' .` sin resultados), así que se transfiere sin verificar.

```sh
# los dos `go test` se ejecutan dentro de apkc/
go test -run 'CadenaEstaticaHeredada' -count=1 -v .
grep -n 'pliegaCadenaHeredada\|esCadena' apkc/javaemisor.go
grep -n 'func Test' apkc/javaemisor_cadena_heredada_test.go
# forma general: la sonda vive fuera del repo y entra por overlay
go test -overlay="$SONDA/overlay.json" -run 'CadenaEstaticaHeredada' -count=1 -v .
```

## 276. La constante suelta en un hueco largo se dobla como javac (`6e42499`)

Registrado el 2026-11-24 sobre `e907433`, para el commit `6e42499`. La corrección viene del builder
`ses_df38958e7ffefnS4nQqp3fNnOf`; este `§` **transfiere** su medición: el test del árbol reproduce
la tabla y la falsificación, y las cifras de ApkShare —17 de 21 clases, `16516 B`, APK `305510 B`—
son suyas y se citan sin re-ejecutarse en este árbol. La crítica sigue en curso: no se da por
aprobada.

**La regla de javac 17.0.21.** Con `--release 8 -g:lines,source`, un literal `long` solo usa
`lconst_0`/`lconst_1` para 0 y 1; todo lo demás —`5`, `-1`, `127`, `32767`, `-32768`, `32768`,
`-32769`, `2147483647L`, `5000000000L`— va por `ldc2_w`. El `i2l` no aparece con una constante:
solo con un operando no constante (`iload; i2l`). La tabla completa está en
`apkc/javaemisor_ldc2w_campo_test.go:29-71`; la comprobación, en `TestLaTablaDelLargoEsLaDeJavac17`
(`:77`), que además desarma nuestro `.class` porque el dex no distingue `ldc2_w` de otra carga de
tres bytes.

**La causa.** `AutoResearchDelay` es un `static final int` suelto, no un nombre `Clase.CAMPO`, y no
entraba por `plegarConstante` (`apkc/javaemisor.go:3725`): el camino de `jacceso` exigía una cadena
de tipo. El emisor doblaba el campo a `int` y dejaba la promoción detrás, así que escribía
`sipush 500; i2l` donde javac escribe `ldc2_w 500l`.

**El cambio.** `plegarConstante` resuelve la constante suelta cuando el nombre tiene un solo
segmento, no hay local del método y `tapaLaCaptura` no encuentra un `val$nombre` en la clase ni en
sus exteriores; exige `static final`, entero (`esConstanteEntera`: `I`, `B`, `C`, `S`, sin `Z`) y no
`String` (`javaemisor.go:3868-3882`). La promoción al hueco `long` la hace `expA`, no el plegado. El
interruptor `pliegaCampoSuelto` (`:3921`) revive el defecto para la falsificación;
`esConstanteEntera` está en `:3925` y `tapaLaCaptura` en `:3932`.
`TestFalsificacionDelPlegadoDelCampoSuelto` (`apkc/javaemisor_ldc2w_campo_test.go:162`) comprueba
las dos direcciones: encendido, dex igual a javac; apagado, `sipush i2l` y dex distinto.

**El resultado, transferido.** `MainActivity$3.afterTextChanged` lleva `ldc2_w #42 // long 500l`,
igual que javac. De 16 a **17 de 21** clases byte a byte **en dex por clase** (`apkc dex` de cada
`.class` nuestro y `cmp` contra el de javac 17; los `.class` crudos difieren por el orden del pool).
El dex mide **16516 B** frente a 16504 del
oráculo: la causa valía 4 B —de 16520 a 16516—, no los 8 que se habían estimado, y la brecha
restante es de 12 B. El APK mide `305510 B` y `apksigner` da v1/v2/v3 en `true`. La suite del
builder salió `ok … 168.066s`, con `gofmt` y `go vet` limpios; es salida transferida, no
versionada.

**Las cuatro clases que quedan.** `Editable.toString` (`invokeinterface` donde javac emite
`invokevirtual`) en `$3$1` y `$4`; `astore 4` frente a `astore_3` en `$6`; y en `MainActivity`,
`ldc_w` frente a `ldc` más el reparto de la lambda.

```sh
# con el entorno del repo (javac 17, d8 y android.jar del SDK)
cd apkc && go test -run \
  'TestLaTablaDelLargo|TestElRetardoConstante|TestFalsificacionDelPlegado' -count=1 -v .
# el oráculo de la regla, sin pasar por apkc: la forma que javac escribe en el hueco `long`
# la sonda es `fuenteRetardoConstante` del test, copiada a disco como p/Retardo.java
mkdir -p "$HOME/.cache/apkc-web/ldc2w"
javac --release 8 -g:lines,source -d "$HOME/.cache/apkc-web/ldc2w" p/Retardo.java
javap -c -p "$HOME/.cache/apkc-web/ldc2w/p/Retardo.class"
```

## 277. El shim respeta el modo de creación de `open` (`e907433`)

Registrado el 2026-11-24 sobre `e907433`. Corrección del builder `ses_df29b25fbffeQbKHk2tRB1L4Mq`;
este `§` **transfiere** su medición: la falsificación y los controles quedan reproducibles con los
comandos de abajo, y las cifras se citan sin re-ejecutarse en este árbol. La crítica sigue en curso:
no se da por aprobada.

**El fallo.** `TestArgumentosDeFirmaNoLlevanLaPassword` (`apkc/paquetetemporal_test.go:181`) veía
`0644` donde `os.CreateTemp` pide `0600` (`:207-208`). No era el test: `open()` llamaba a
`nuevoNodo` sin pasar el `modo` y todo nodo nuevo nacía con `0644`. En todo el repo, el único sitio
que compara permisos es esa comprobación (`:207`).

**El cambio.** `shim/navegador-sim.js` define una sola `umask = 0o022` (`:55`), la misma que
devuelve `process.umask` (`:391`); `nuevoNodo(calle, esDir, permiso)` (`:59`) compone el modo como
`(base & 0o170000) | ((permiso & 0o7777) & ~umask)` (`:64`); y `open()` pasa
`modo === undefined ? 0o666 : modo` en la rama `O_CREAT` (`:214`). Un `O_CREAT` sobre un fichero que
ya existe no toca su modo, como en Linux.

**La falsificación.** `shim/prueba-modo.js` compila una sonda fuera del repo y la corre por el
shim: `CreateTemp` sin modo (0600), un `O_CREAT|O_EXCL` con 0600, un `O_CREAT` con 0666 sobre el que
ya existe —debe seguir 0600— y un `WriteFile` con 0644. Con el shim viejo, 3 fallos: `CreateTemp =
0600 — 0644`, `O_CREAT|O_EXCL con 0600 = 0600` y sigue `0600` tras reabrirlo con 0666; con el
arreglo, las 5 comprobaciones en verde. La capa `wasm` baja de 20 a **19** fallos de nivel superior,
sin aborto y sin el fallo de la contraseña.

**Controles.** `shim/prueba-relink.js` 4/4, `shim/prueba-salida.js` 10/10, `create /miapp` con
rc 0 y `node --check` limpio sobre el shim. Son cifras transferidas del builder.

**Deuda abierta.** `mkdir` sigue tirando el modo (`nuevoNodo(a, true)` en `:282`) y ningún test
compara permisos de directorio; no se cierra aquí.

```sh
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" shim/wasm_exec.js
node shim/prueba-modo.js
# la falsificación, contra el shim anterior al arreglo
mkdir -p "$HOME/.cache/apkc-web/falsif"
git show e907433^:shim/navegador-sim.js > "$HOME/.cache/apkc-web/falsif/shim-0644.js"
cp shim/wasm_exec.js "$HOME/.cache/apkc-web/falsif/"
SHIM="$HOME/.cache/apkc-web/falsif/shim-0644.js" node shim/prueba-modo.js
cd apkc && go test -run TestArgumentosDeFirmaNoLlevanLaPassword -count=1 -v .
node --check shim/navegador-sim.js
```

## 278. El fs simulado deja de dejar huérfanos y las pruebas ajenas a la pestaña salen por tag

Registrado el 2026-11-24 sobre `9f29d88`, con dos frentes que van juntos porque el primero es lo
que dejó ver el segundo: la corrección del fs simulado (`2420922`, builder
`ses_df3c42d40ffeqQ5mKEpK7pdwrP`) y la separación por build tags (`f64101e`, builder
`ses_df3bc24abffehGK5ieAcRCpuET`), las dos con crítica APROBAR. Lo ejecutado aquí va marcado como
tal; las cifras del run completo de la suite wasm son del builder y se citan sin re-ejecutar.

**Parte A, el síntoma.** `sh tools/entorno.sh wasm` abortaba en
`TestRecursosDeDependenciaEnElApk`: el `open` de `build/res/layout/deps_fila.xml` no encontraba el
fichero —«No such file or directory»— y `LogF` llama a `os.Exit(1)`, así que solo 118 de 853
pruebas de nivel superior llegaban a correr y el resto del rojo quedaba invisible.

**La causa.** `fsSim.unlink` borraba también nodos de directorio y devolvía `nil`. El
`os.RemoveAll` de `build/res` se daba por hecho al ver el `nil`, dejaba los hijos huérfanos en el
mapa del shim y el `Stat` del huérfano —`build/res/layout`— hacía que `MkdirAll` cortocircuitara
sin recrear la rama; el `open` del recurso recién escrito fallaba con ENOENT.

**El arreglo (`2420922`).** `shim/navegador-sim.js:294-307` —en ese commit, `:283-296`— hace que
`unlink` conteste `EISDIR` a un directorio (`:298-304`), como Linux: `os.Remove` cae a `rmdir`, el
`ENOTEMPTY` de un directorio con hijos hace recorrer a `RemoveAll` y el árbol queda coherente.
`shim/prueba-relink.js` es la sonda: escribe un recurso, vacía `build/res`, rehace
`build/res/layout`, reescribe y relee, y comprueba aparte la semántica de `unlink`/`rmdir`.

**Lo ejecutado aquí (A).** Con el shim del árbol, `node shim/prueba-relink.js` da sus 4
comprobaciones en verde; con el shim de `2420922^` en un directorio aparte da 4 fallos, con
`stat de .../build/res/layout tras RemoveAll -> <nil>` y `(apkc devolvio 2)`. La crítica no vio
regresión en `--montar`/`--exportar` ni en `shim/prueba-salida.js` (10/10), y los 101 SKIP del run
son guards de SDK/javac/d8 que ya existían: el aborto los enmascaraba, no hay `t.Skip` nuevos.

**Después (A).** La suite wasm llega al final: 854 pruebas de nivel superior, 733 PASS, 20 FAIL,
101 SKIP y sin aborto; el rc 1 es de los fallos propios de wasm (exec de javac, `os.Pipe`, re-exec
del binario, permisos 0644, WISP), no del fs.

**Parte B, los cinco movimientos.** `dependencias_paridad_test.go` (`!js`),
`device_paquete_proceso_test.go` (`!js`), `dependencias_dup_proceso_test.go` (`!js`) y
`wisp_endurecimiento_nativo_test.go` (`!js`) dejan de compilarse en el binario wasm;
`wisp_endurecimiento_js_test.go` (`js && wasm`) es la mitad nueva. La doctrina de
`dependencias_test.go:5-8` prohíbe `t.Skip`: se excluye por tag, no se salta, porque un SKIP deja
la suite en verde sin haber medido nada.

**Lo que no se movió, a propósito.** `javac17Exigido` sigue sin tag en
`dependencias_test.go:37-48` porque nueve ficheros sin tag lo usan: al moverlo, `go vet` con
`GOOS=js GOARCH=wasm` daba `undefined: javac17Exigido`. Y la prueba compartida de WISP suelta la
asignación `cfg.Relay = RelaysWISP()[0]`, que era preparativo y no aserción;
`TestAbrirWebSocketWISPExigeConfigValidada` (`wisp_endurecimiento_test.go:268-285`) se queda con
las dos aserciones que se deciden antes de tocar un socket.

**Lo ejecutado aquí (B).** Compilado el binario de pruebas con `GOOS=js GOARCH=wasm` y ejecutado
con `/usr/lib/go/lib/wasm/go_js_wasm_exec`, los cuatro grupos movidos contestan
`testing: warning: no tests to run` seguido de `PASS` (rc 0); el control positivo, las dos pruebas
de WISP sin tag, dan `PASS`
con rc 0; y `GOOS=js GOARCH=wasm CGO_ENABLED=0 go vet .` sale limpio con rc 0. Es la verificación
de la QA `ses_df29342e7ffeozUcSMbYgovw9m`, reproducida aquí.

**La red que ya no se abre.** El test js guarda el `WebSocket` del global, lo pone en
`Undefined()` y lo restaura con `defer` (`wisp_endurecimiento_js_test.go:5-27`), así que no hay
conexión real al relay aunque Node 26 exponga `WebSocket` (undici). Antes del split esa prueba
abría red: una dependencia externa oculta que ahora no existe.

**Deuda abierta, fuera de este frente.** Las nueve pruebas `javaemisor_*` que invocan `javac`
siguen compilándose bajo `js` y merecen el mismo tratamiento por tag.

```sh
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" shim/wasm_exec.js
node shim/prueba-relink.js
mkdir -p "$HOME/.cache/apkc-web/falsif/2420922-prev"
V="$HOME/.cache/apkc-web/falsif/2420922-prev"
git show 2420922^:shim/navegador-sim.js > "$V/navegador-sim.js"
cp shim/wasm_exec.js "$V/"
SHIM="$V/navegador-sim.js" node shim/prueba-relink.js
sh tools/entorno.sh wasm
```

```sh
cd apkc
GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$HOME/.cache/apkc-web/docs278/t.wasm" .
L="$HOME/.cache/apkc-web/docs278/t.wasm"
W=/usr/lib/go/lib/wasm/go_js_wasm_exec
"$W" "$L" -test.v -test.run TestCompilarEnGoDexaIgualQueJavacConUnaDependencia
"$W" "$L" -test.v -test.run TestRunRechazaManifiestoInyectadoAntesDeLaShell
"$W" "$L" -test.v -test.run TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase
"$W" "$L" -test.v -test.run TestAbrirWebSocketWISPConConfigValidaAvisaDeNativo
"$W" "$L" -test.v -test.run TestAbrirWebSocketWISPRechazaLaDireccionSinAbrirRed
"$W" "$L" -test.v -test.run TestAbrirWebSocketWISPExigeConfigValidada
GOOS=js GOARCH=wasm CGO_ENABLED=0 go vet .
```

## 279. La capa wasm llega al final sin aborto y deja 15 fallos ambientales

Registrado el 2026-11-24 sobre `9f29d88`, con la QA `ses_df28bc63effeOwlMQM6jzhJ1Rw`. La suite
wasm ya no muere a mitad: recorre la suite entera y termina con `RC=1` por quince
fallos que no dependen del cambio. Este `§` deja la cifra, la comparación con el rojo anterior y el
`file:line` de cada causa, para que el siguiente run no confunda un ambiente incompleto con una
regresión.

**El run.** Con `TMP`, `PREFIX` y `GOTMPDIR` bajo `$HOME/.cache/apkc-web/` (regla 12, nada en
`/tmp`), `tcr -- sh tools/entorno.sh wasm` sale `RC=1` y deja
`~/.cache/apkc-web/wasm-final.log` (18636 B, 233 líneas). El log no abrevia: hay **15** `--- FAIL`
de nivel superior —más 6 subtests dentro de los dos `Dedupe*`— y ningún `panic` ni `os.Exit`: el
`(apkc devolvio 1)` de la última línea es la propagación del rc del propio test.

**La comparación.** El log previo de 20 fallos (`~/.cache/apkc-web/despues-verboso.txt`) contiene
exactamente estos 15 más cinco que ya no figuran en rojo (no se re-ejecutaron con `-test.v`):
`TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase`,
`TestCompilarEnGoDexaIgualQueJavacConUnaDependencia`,
`TestRunRechazaManifiestoInyectadoAntesDeLaShell`,
`TestArgumentosDeFirmaNoLlevanLaPassword` y
`TestAbrirWebSocketWISPExigeConfigValidada`. Es una reducción estricta: ningún test que antes
pasara está ahora en rojo.

**Lo dirigido.** Ejecutar a mano las dos pruebas de recursos y firma con el `android.jar` y el
layout montados da `--- PASS` en ambas y `RC=0`; en particular ya no aparece el ENOENT de
`build/res/layout/deps_fila.xml` que abortaba el run del `§278`.

**Las causas de los 15.** Once son el oráculo de `javac 17`: `javac17Exigido`
(`dependencias_test.go:38-48`) llama a `buscarJavac17()` y, al no encontrar javac en el hospedador
(`exec.LookPath`/`exec.Command` no funcionan bajo `js`), aborta con `t.Fatalf`. Son
`TestNewDeOtraClaseContraJavac17` (`javaemisor_alcance_test.go:65`), `TestNewConTipoDeJar`
(`:178`), `TestHerenciaDeOtraClaseSubePorSuPadre` (`:348`),
`TestElPostDelayedDeApkShareLlevaSuI2L` (`javaemisor_ancho_argumento_test.go:146`),
`TestGotoWLargoHaciaAtras` (`javaemisor_atras_test.go:95`; su aserción del `goto_w` pasa en `:86`
con el desplazamiento `-36009`), `TestElBooleanNoEnsanchaEnElArgumento`
(`javaemisor_boolean_ensancha_test.go:42`), `TestEmisorReferenciaCualificadaContraJavac17`
(`javaemisor_cualificado_test.go:81`), `TestEmisorImportExplicitoContraJavac17`
(`javaemisor_import_test.go:126`), `TestEmisorImportExplicitoMismoPaqueteContraJavac17` (`:169`),
`TestSobrecargaExteriorContraJavac17` (`javaemisor_receptor_heredado_test.go:170`) y
`TestTipoCualificadoLocalContraJavac17` (`javaemisor_tipo_cualificado_local_test.go:89`).

Dos más son `os.Pipe`, que `js` no implementa: `avisosDurante` (`linker_dedupe_clave_test.go:60-63`)
lo usa para capturar `LogW`, y fallan `TestDedupeDeValuesPorClave`
(`linker_dedupe_clave_test.go:222`, con 3 subtests) y `TestDedupeDeStyleablesPorClave` (`:414`, con
3 subtests). Los dos últimos re-ejecutan el propio binario con `exec.Command(os.Args[0])`
(`multidex_app_test.go:386`): `TestCompilaAppPre21ConDemasiadasClasesAborta` (fallo en `:412`) y
`TestCompilaAppSinUsesSdkConDemasiadasClasesAborta` (`:439`); el hijo no arranca bajo `js` y la
salida sale vacía.

**Veredicto.** Pasa en lo pedido: la capa wasm llega al final sin aborto. El `RC=1` se debe a los 15
fallos ambientales de arriba, ninguno imputable al cambio medido. Queda deuda declarada: `os.Pipe`
en `avisosDurante` y los tests de oráculo `javac`/re-ejecución; la vía es `capturarSalida` con
fichero —ya aplicado en `sign_hostil_test.go`— y build tags —ya aplicados a otros cuatro grupos en
el `§278`—, respectivamente.

**Nota de HEAD.** Durante la medición el `HEAD` avanzó a `b7c86cd` con cambios **solo** en `docs/`;
el código Go medido es idéntico al de `9f29d88`.

```sh
export TMP="$HOME/.cache/apkc-web/tmp279"
export PREFIX="$HOME/.cache/apkc-web/prefix279"
export GOTMPDIR="$HOME/.cache/apkc-web/gotmp"
mkdir -p "$TMP" "$PREFIX" "$GOTMPDIR"
tcr -- sh tools/entorno.sh wasm > "$HOME/.cache/apkc-web/wasm-final.log" 2>&1; echo "RC=$?"
wc -c -l "$HOME/.cache/apkc-web/wasm-final.log"
grep -c '^--- FAIL' "$HOME/.cache/apkc-web/wasm-final.log"
grep -c '^--- FAIL' "$HOME/.cache/apkc-web/despues-verboso.txt"
```

```sh
J="$HOME/opt/android-sdk/platforms/android-34/android.jar"   # la ruta que usó el log
node shim/navegador-sim.js \
  --montar "$J:/android/platforms/android-34/android.jar" \
  --montar apkc/testdata/vista-propia/main.xml:/testdata/vista-propia/main.xml \
  "$HOME/.cache/apkc-web/wasm-final/apkc-test.wasm" -test.v \
  -test.run '^(TestArgumentosDeFirmaNoLlevanLaPassword|TestRecursosDeDependenciaEnElApk)$'
```

## 280. Un método de `Object` sobre interfaz, el hueco del `try` y la captura del dedupe

Registrado el 2026-11-24 sobre `50952c0`. Transfiere tres frentes ya integrados (`e96df44`,
`27f1340` y `50952c0`): el primero llega con crítica emitida
(`ses_df272ffdfffeejda4CdNKbyDT2`, APROBAR) y los otros dos con la crítica en curso. Aquí se
re-ejecutaron solo los cinco tests puntuales de los dos primeros frentes, los dos del dedupe y los
`grep` del bloque de comandos: sin suite, sin build de APK, sin capa wasm, sin gate, sin chromium,
sin red, sin push y sin publicación. Las cifras de suite, de APK y de la capa wasm son del builder
y van marcadas como **transferidas, no versionadas**.

**A. Un método de `Object` sobre un receptor de interfaz es `invokevirtual` (`e96df44`).** La regla,
medida con `javac 17 --release 8 -g:lines,source` y `javap -c`, es que javac solo se queda en la
interfaz cuando el **tipo estático del receptor declara el método él mismo**; si lo hereda, javac
usa `invokevirtual java/lang/Object.toString`. Se verificó sobre seis interfaces, entre ellas
`ConHash`, que declara `hashCode` y sale `invokeinterface`; `ConToString`, que redeclara `toString`
y sale `invokeinterface`; y `HeredaCS extends CharSequence`, que sale `Object`. Antes, el emisor
elegía el opcode por el `ACC_INTERFACE` del receptor y escribía `invokeinterface` también en los
casos heredados.

**Los cambios de A.** `llamada` (`apkc/javaemisor.go:4470`) decide el opcode en `:4748-4764` y
cambia a `invokevirt` con dueño `java/lang/Object` cuando el método está en `Object` y el tipo
estático no lo declara (`:4760`). La tabla de qué declara `Object` está en `metodoDeObject`
(`:1343`), la pregunta de si el tipo lo declara él mismo en `declaraElMismo` (`:1360`), y el
interruptor de la falsificación en `objetoEnInterfazVirtual` (`:1337`). El campo `objeto`
(`:617-620`) se llena en pereza en `:1344-1351` y retrocede sin ruido si el jar no trae
`java/lang/Object` (`err == nil`).

**El test de A.** `apkc/javaemisor_objeto_interfaz_test.go:99`, `:136` y `:156`: dex byte a byte
de la anónima con la captura (`1336 B`); la falsificación da `1512` frente a `1496` del oráculo,
con el primer byte distinto en `8`.

**La medición de A, transferida.** De 17 a **19 de 21** clases byte a byte; APK de `305505 B` con
`apksigner verify --verbose` v1/v2/v3 `true` y `zipalign -c -p 4` OK; suite `ok … 228.560s`. La
crítica solo re-ejecutó los tres tests puntuales y la sonda de la regla.

**Cómo se cuenta.** El recuento es **dex por clase**: se saca el dex de cada `.class` con el
subcomando `dex` de apkc y se compara con `cmp` contra el dex de javac 17 de la misma clase. El
`.class` crudo no vale para contar: además de las diferencias de código, difiere por el orden del
pool. Con esa precisión, las cifras viejas de 12 de 21 eran `.class` crudo, y de 16 de 21 en
adelante son dex por clase.

**B. La excepción del `try` reutiliza el hueco al cerrarlo (`27f1340`).** En `MainActivity$6` el
emisor guardaba la excepción en `astore 4`/`astore 5` y partía la tabla en dos filas (`59 107 122`
y `110 119 122`); ahora usa `astore_3`/`astore 4` y una sola fila `59 119 122`, que es la de
`javac 17`.

**Los cambios de B.** `intenta` (`apkc/javaemisor.go:7881`) abre el ámbito como bloque (`:7932`) y
lo cierra al final (`:8107`). `trozosDelCuerpo` (`:8123`) solo corta el rango si el salto sale de
él, con el interruptor `cortaRangoEnTodoGoto` de `:8114` aplicado en `:8141`. La poda de marcos usa
`muerteAmbito` (`:2570`), que se anota al cerrar el ámbito (`:2990`) y que `podaHuecos` aplica
(`apkc/jvmframes.go:318`, `:1290` y `:1308`); el reparto de huecos queda tras `reutilizaHuecos`
(`:2977`, comprobado en `:2992`).

**El test de B.** `apkc/javaemisor_astore_try_test.go:107` y `:158`: dex byte a byte, más las dos
falsificaciones. Con `reutilizaHuecos=false` el dex mide `1492` frente a `1492` de javac pero
`copia` queda con 6 huecos y `vivos` con 4 (`:176`); con `cortaRangoEnTodoGoto=true` salen dos
filas, `[0,46)` y `[49,57)`, frente a la única de javac (`:192`).

**La medición de B, transferida.** 18 de 21 clases en su worktree, sin el frente A; dex de
`16504 B`, igual al oráculo; APK con v1/v2/v3 `true`; suite `ok … 226.704s`.

**El recuento final, medido.** El auditor `ses_df2710b6affetFQsreOOwjObLc` mide **20 de 21** clases
byte a byte en dex por clase sobre el árbol combinado, con javac 17 compilando contra
`-cp android.jar:android-support-v4.jar`; solo difiere `MainActivity.class`. El dex del build mide
`16504 B`, igual al del oráculo. La interacción de A con B queda así cerrada por medición, no por
analogía.

**C. La captura del dedupe va a fichero, no por tubería (`50952c0`).** `avisosDurante`
(`apkc/linker_dedupe_clave_test.go:62-64`) pasa a `capturarSalida(t, hacer)`
(`apkc/sign_hostil_test.go:91-110`), que desvía `os.Stdout` a un fichero de `t.TempDir()` y lo
restaura con `defer`, también si `hacer` revienta; se retira el import `io`, que era el de
`os.Pipe`. No se relajó ninguna aserción: las cuatro llamadas a `avisosDurante` (tres directas y
la de `enlazaConAvisos`, `:158-170`) y los seis `strings.Contains` sobre el aviso siguen iguales.

**La medición de C, transferida.** En la capa wasm los fallos de nivel superior bajan de 15 a
**13** sin aborto, y los dos `Dedupe` pasan con sus 7 subtests. Quedan 11 por `sin javac 17` y 2
`multidex` que se re-ejecutan con `os.Args[0]` (`apkc/multidex_app_test.go:386`). La suite nativa
da `ok … 210.640s` solo con `JAVAC17` exportado; sin esa variable hay 11 fallos ambientales
previos.

**Lo re-ejecutado aquí.** Los cinco tests de A y B pasan sobre el árbol combinado (`ok … 4.695s`)
y los dos del dedupe también (`ok … 0.629s`, siete subtests), ya con la captura a fichero: el
síntoma de C no reaparece en nativo. No se re-ejecutaron la suite, el APK ni la capa wasm; esas
cifras quedan como transferidas, incluido el recuento de 20 de 21 en dex por clase, que es medición
del auditor y no se repite aquí.

```sh
# A y B, tests puntuales sobre el árbol combinado; PASS en 4.695s
T='TestElToStringSobreUnaInterfaz|TestElToStringDeUnaEditableCapturada|'
T="$T"'TestFalsificacionDelToString|TestElTrySeDexa|TestFalsificacionDelHuecoDelTry'
GOTMPDIR=$HOME/.cache/apkc-web/gotmp tcr -- go test -run "^($T)" -v .
```

```sh
# el dedupe, con la captura a fichero; PASS en 0.629s con siete subtests
GOTMPDIR=$HOME/.cache/apkc-web/gotmp tcr -- \
  go test -run '^(TestDedupeDeValuesPorClave|TestDedupeDeStyleablesPorClave)$' -v .
```

```sh
# los símbolos que sostienen cada afirmación
grep -n 'objetoEnInterfazVirtual\|metodoDeObject\|declaraElMismo' apkc/javaemisor.go
grep -n 'reutilizaHuecos\|cortaRangoEnTodoGoto\|muerteAmbito' apkc/javaemisor.go
grep -n 'podaHuecos' apkc/jvmframes.go
grep -n 'avisosDurante\|capturarSalida' apkc/linker_dedupe_clave_test.go \
  apkc/sign_hostil_test.go
grep -n 'os.Pipe\|"io"' apkc/linker_dedupe_clave_test.go   # sin resultados
```

## 281. Los dos gates WISP pasan con el núcleo inerte y sin contactar relays

Registrado el 2026-11-24 sobre `50952c0`, con la QA `ses_df274db9affe0VViKHbR0zm4op`. Los dos gates
del módulo WISP salen en verde con el módulo **inerte**: no se abre ningún socket hacia un relay
real. Este `§` deja las cifras, las aserciones que lo falsifican y la comprobación de egreso por
separado.

**El gate por defecto.** `tcr -- sh tools/entorno.sh wisp` sale `rc=0` con **25 `ok`** y cierra con
«falsificación: los dos gates rechazan lo que tienen que rechazar». No hay red ni navegador:
`studio/prueba-wisp-gates.mjs` no contiene `fetch(` ni un `new WebSocket(` real —solo el
`socketFalso` de `:44-60`—, y las URL con host son literales de clasificación, no destinos que se
abran. Aserciones citadas del run: «gate por defecto: las URLs del propio origen, `data:` y `blob:`
pasan — 4 locales, cero ajenas»; la falsificación 1 mete `wss://relay-ajeno.example/wisp/` y el modo
externo sin núcleo responde
`{"motivo":"núcleo WISP no disponible en esta compilación","sockets":0,"llamadas":0}`; la
falsificación 2 usa un relay fuera de la lista; y el handshake cubre CONTINUE y DATA.

**El gate externo.** El primer intento con el `chromium` del sistema falló por ABI:
`/usr/lib/chromium/chromium: error while loading shared libraries: libsimdutf.so.36` (el sistema
tiene `.37`; `chromium --version` falla igual, `rc=127`). Es una causa **ambiental preexistente**,
no del cambio. Repetido con
`CHROME_BIN=/usr/bin/brave APKC_WISP_EXTERNO=1 tcr -- sh tools/entorno.sh wisp-externo` sale `rc=0`
con **`237 comprobaciones bien, 0 mal`**, sobre Brave 157.1.99.57 headless, con perfil propio y CDP
en 9345.

**Lo que vio el externo.** `wisp-relays.json trae 6 relays verificados`; sin núcleo falla cerrado
(`sockets:0`, `llamadas:0`, `nuevas:0`); con el módulo activo el `Resolver` falla con su motivo y
`peticiones:0`; el gate de red ve el `wss://sonda-ajena-del-gate.invalid/relay` de la sonda
deliberada, que se crea y se cierra **antes** de resolver DNS (dominio `.invalid`, RFC 6761);
`481 peticiones, 0 websockets, todas locales`; y la cobertura alcanza `1 service_worker, 1 page,
2 worker`.

**El egreso, comprobado aparte.** Un muestreo de `ss -tnp` cada 200 ms dejó
`~/.cache/apkc-web/tmp/conexiones-brave.log` (3522 líneas) y **ninguna IP de los seis relays**
(`wisp.mercurywork.shop`, `terbiumon.top`, `anura.pro`, `lichology.com`, `wisp.waved.site`,
`gointospace.app`) aparece en las conexiones del proceso de la prueba. El bloque de abajo reproduce
ese muestreo, con las diez IP resueltas de esos seis relays en el `grep`, que debe salir vacío.
Matiz honesto: Brave abrió HTTPS propio del navegador (telemetría y actualizaciones) a otras IP,
ajenas a la pestaña y a los relays; el tráfico que mide el CDP son las 481 peticiones locales.

**Estado del entorno.** `APKC_WISP_EXTERNO` no queda exportado: la única asignación es inline en
`tools/entorno.sh:155`, y el guard que sale `rc=2` sin ella está en `:146`. No quedaron procesos
`prueba-pestana` ni `chromium` colgados, y el Brave del usuario sigue intacto.

**Veredicto.** VERDE con semántica de fallo cerrado: el externo pasa **porque el módulo está
inerte**, no porque se haya abierto un socket a un relay real. WISP sigue siendo excepción opt-in y
no está activado por defecto. Desde `§349` sus trust roots se sirven del mismo origen con huella
sha256 fija (`studio/cas-wisp.js`), con `File` local como override; el relay nunca es fuente de
CAs y no se hicieron peticiones nuevas para medirlo.

```sh
tcr -- sh tools/entorno.sh wisp
CHROME_BIN=/usr/bin/brave APKC_WISP_EXTERNO=1 tcr -- sh tools/entorno.sh wisp-externo
```

```sh
# el egreso: se muestrea ss -tnp mientras corre el externo (esta vez sí, el log sale de aquí)
LOG="$HOME/.cache/apkc-web/tmp/conexiones-brave.log"
: > "$LOG"
CHROME_BIN=/usr/bin/brave APKC_WISP_EXTERNO=1 tcr -- sh tools/entorno.sh wisp-externo & GATE=$!
while kill -0 "$GATE" 2>/dev/null; do ss -tnp >> "$LOG"; sleep 0.2; done
wait "$GATE"
wc -l "$LOG"
# el patrón, partido en dos para no pasar de 100 columnas; el grep debe salir vacío
IPS='104\.21\.33\.29|172\.67\.140\.188|51\.81\.36\.242|104\.21\.11\.48|172\.67\.165\.40'
IPS="$IPS"'|104\.21\.34\.165|172\.67\.163\.20|104\.21\.1\.217|172\.67\.128\.22|15\.204\.247\.166'
grep -nE "$IPS" "$LOG"   # debe salir vacío
```

## 282. El implementador de la lambda se emite al cerrar la clase, no en el sitio

Registrado el 2026-11-24 sobre `52eac29` (`4af5740` en este worktree), con la medición del builder
`ses_df2821e86ffeUJp8AeEVvtn4C3`. La crítica `ses_df262b905ffevNGBm3S6zk7wXL` lo **aprobó** con
**20 de 21** clases byte a byte; la única divergente es el `MainActivity` por la fila
`line 159: 155`. La integración del árbol combinado queda al orquestador.

**El defecto era el orden del pool, no su tamaño.** En el `MainActivity` de ApkShare las dos
constantes `Integer` de `R.layout.*` (`2130903043` y `2130837511`) caían en las entradas #268 y #273
y se escribían con `ldc_w`; javac las pone en la #249 y la #254 y escribe `ldc`. No sobraban
entradas —nuestro pool cierra en la #328 y el de javac en la #330, cifra transferida del builder y
no re-ejecutada aquí—: lo que fallaba era el orden. Con
el índice 268, `ldc_w` era la forma **correcta** según el JVMS 6.5: lo incorrecto era que el índice
no fuese el de javac.

**La causa.** El emisor compilaba el implementador `lambda$…` y las constantes de su bootstrap
dentro del método que declara la lambda; javac los genera al cerrar la clase, detrás del código de
todos los métodos. Ahora `lamda` encola el implementador (`apkc/lambda_fuente.go:495`, la cola
`lambdaPendientes`) y `emmitirClase` la vacía al cerrar (`apkc/javaemisor.go:8929`); la fila de
`BootstrapMethods` reserva su índice en el sitio y calcula MethodHandle y MethodType con el pool ya
cerrado (`registraBootstrapDiferido`, `apkc/lambda_fuente.go:564`, y `classBootstrap.pendiente`,
`apkc/lambda.go:31`); `MethodHandles$Lookup` para el `InnerClasses` se anota en ese mismo cierre
(`apkc/javaemisor.go:8943`), que es donde javac la escribe; y el `Object` por defecto de
`super_class` solo entra si el fuente no declara padre (`:8755-8761`). El interruptor de la
falsificación es `implementadorEnElSitio` (`apkc/lambda_fuente.go:490`). Con el arreglo las dos
constantes quedan en la #250 y la #255 (javac #249 y #254), las dos con `ldc`.

**La sonda.** `apkc/javaemisor_ldc_pool_largo_test.go` compila una lambda en un constructor ajeno
(`new Ayuda(s -> usa(s.length()))`) y `k` métodos de relleno, y vigila la forma del primer `ldc` de
`limite`. Con `k=114` el índice cae en #254 y sale `ldc`; con el interruptor encendido vuelve a
`ldc_w` con un índice por encima de 255 —el #274 concreto no lo asevera el test: queda INCIERTO—;
con `k=116` es #258 y toca `ldc_w`. El test asevera la forma y el lado del límite, no el número
exacto. El oráculo es javac 17 `--release 8 -g:lines,source` en los tres casos, y el dex de la
sonda entera coincide byte a byte en el caso arreglado. En este worktree los tres tests pasan:

**Lo que midió el builder (medición transferida, no versionada).** Cuerpo del `.class` de
`MainActivity` idéntico al de javac salvo una fila `line 159: 155`; dex por clase 6108 B frente a
6112; dex de la app 16516 B frente a 16504 del oráculo (brecha de 12 B de otras clases); APK
305509 B con `apksigner verify` v1/v2/v3 en `true`; y la suite completa en `ok … 177.115s`.

**Deuda abierta, no cerrada.** La fila `line 159: 155` de `openAppDetailsDialog` es la única
diferencia de `MainActivity` y no es del pool. Por qué javac anota esa línea no está documentado
como hecho; una hipótesis, no verificada, es que anota la línea del `} else {` en el `goto` cuando
la rama `then` declara una local con lambda. La supuesta diferencia entre `access$000` y `lambda$…`
**no se reproduce** con el oráculo de este repo: javac 17 produce el mismo juego. Deuda sin
reproducir, no hecho.

**Veredicto.** El emisor ya respeta el orden de javac en este caso y la forma del `ldc` la decide el
índice, no el interruptor; lo que resta del `.class` es la tabla de líneas del `if`/`else`.

```sh
# la sonda puntual: los tres casos del límite 255, con el android.jar del SDK de escritorio
sh -c 'cd apkc && \
  APKC_ANDROID_JAR="$HOME/opt/android-sdk/platforms/android-34/android.jar" \
  JAVAC17="$HOME/opt/jdk17/bin/javac" \
  GOTMPDIR="$HOME/.cache/apkc-web/gotmp" \
  tcr go test -run "TestElPoolDeLaLambda|TestElLdcDelPool|TestElInterruptorRestaura" \
  -count=1 -v .'
# salida: los tres PASS con `ok`; el tiempo depende de la máquina
```

```sh
# los símbolos que sostienen cada afirmación
grep -n 'implementadorEnElSitio\|implementadorPendiente\|registraBootstrapDiferido' \
  apkc/lambda_fuente.go
grep -n 'lambdaPendientes = e.lambdaPendientes\[1:\]\|bootstraps\[i\].pendiente' \
  apkc/javaemisor.go
grep -n 'MethodHandles\$Lookup\|superIdx = e.pool.clase("java/lang/Object")' apkc/javaemisor.go
grep -n 'pendiente func' apkc/lambda.go
```

## 283. Los oráculos que exigen javac 17 salen del binario de la pestaña por build tag

Registrado el 2026-11-24 sobre `af7686b` (el commit del builder es `7cb559b`). La crítica
`ses_df25516b9ffeypRdvNY7sfyIXB` aprobó `af7686b` sin hallazgos, así que este `§` deja el veredicto
**APROBAR**. El cambio no toca el emisor: mueve once pruebas que lanzan `javac` a ficheros con
`//go:build !js`, para que la capa wasm deje de contar como fallo lo que en la pestaña no puede
existir —no hay proceso que lanzar—.

**Los once, y dónde viven.** `TestNewDeOtraClaseContraJavac17`, `TestNewConTipoDeJar` y
`TestHerenciaDeOtraClaseSubePorSuPadre` en `apkc/javaemisor_alcance_paridad_test.go:21`, `:134` y
`:215`; `TestElPostDelayedDeApkShareLlevaSuI2L` en `javaemisor_ancho_argumento_paridad_test.go:22`;
`TestGotoWLargoHaciaAtrasContraJavac17` en `javaemisor_atras_paridad_test.go:19`;
`TestElBooleanNoEnsanchaContraJavac17` en `javaemisor_boolean_ensancha_paridad_test.go:20`;
`TestEmisorReferenciaCualificadaContraJavac17` en `javaemisor_cualificado_paridad_test.go:18`;
`TestEmisorImportExplicitoContraJavac17` y `TestEmisorImportExplicitoMismoPaqueteContraJavac17` en
`javaemisor_import_paridad_test.go:20` y `:63`; `TestSobrecargaExteriorContraJavac17` en
`javaemisor_receptor_heredado_paridad_test.go:20`; y `TestTipoCualificadoLocalContraJavac17` en
`javaemisor_tipo_cualificado_local_paridad_test.go:20`. Los ocho ficheros abren con
`//go:build !js`.

**Cero saltos nuevos.** `git diff -U0 7cb559b^ 7cb559b | grep -c "t.Skip"` da `0`: la doctrina de
`apkc/dependencias_test.go:8` —«Estas pruebas **no se saltan**: … un SKIP deja la suite en verde sin
haber medido nada»— se cumple con tag, no con `Skip`.

**Lo que se queda sin tag para no perder cobertura.** `TestNewDeOtraClaseResuelveSuConstructor`
(`apkc/javaemisor_alcance_test.go:48`), `TestElPostDelayedDeApkShareCompila`
(`javaemisor_ancho_argumento_test.go:143`), `TestGotoWLargoHaciaAtras`
(`javaemisor_atras_test.go:50`, la mitad que comprueba `Code` > 32767 y el `goto_w` sin javac),
`TestElBooleanNoEnsanchaEnElArgumento` (`javaemisor_boolean_ensancha_test.go:42`),
`TestEmisorReferenciaCualificada` (`javaemisor_cualificado_test.go:55`),
`TestEmisorImportExplicitoMismoPaqueteGanaAlHomonimo` (`javaemisor_import_test.go:126`),
`TestSobrecargaExteriorResuelvePorElTipo` (`javaemisor_receptor_heredado_test.go:219`) y
`TestTipoCualificadoLocalApuntaAlTipoAnidado` (`javaemisor_tipo_cualificado_local_test.go:89`).

**`javac17Exigido` no se mueve.** Sigue en `apkc/dependencias_test.go:41`, sin tag: lo usan los
ficheros sin tag, y moverlo rompía el build wasm con `undefined: javac17Exigido`.

**Veredicto de la crítica: APROBAR, con su medición.** Cero aserciones relajadas: la comparación
línea a línea de los ocho pares de ficheros —cada `*_paridad_test.go` con su gemelo sin tag— da
`FALTAN 0`. La suite nativa sale `ok … 183.986s`, con `javac17Exigido` fallando por `t.Fatalf` y no
por `t.Skip`. Y la capa wasm se ejecutó entera hasta el final: **2 `--- FAIL` de nivel superior**,
`TestCompilaAppPre21ConDemasiadasClasesAborta` (`apkc/multidex_app_test.go:401`) y
`TestCompilaAppSinUsesSdkConDemasiadasClasesAborta` (`:429`), los dos por la re-ejecución del propio
binario (`exec.Command(os.Args[0])`, `:386`), y **sin** javac, sin dedupe y sin aborto. Son cifras
de la crítica, no re-ejecutadas en esta sesión.

**Lo medido aquí.** `GOOS=js GOARCH=wasm CGO_ENABLED=0 go vet .` y `go vet .` salen `rc=0`;
`git diff --check af7686b^ af7686b` queda limpio; y `gofmt -l` sobre los dieciséis ficheros del
commit no lista ninguno. El único `gofmt` sucio del paquete,
`javaemisor_constantvalue_long_test.go`, es deuda previa y no entra en el commit:
`git diff --name-only 7cb559b^ 7cb559b` no lo nombra.

**Deuda preexistente, no regresión.** `TestReceptorHeredadoEnAnonimaContraJavac17` llama a
`javac17(t)` en `apkc/javaemisor_receptor_heredado_test.go:85` (la prueba abre en `:83`), y otros
ficheros (`comodin`, `cadena_estatica`, `lambda_fuente`, `estatica_cualificada`) hacen lo mismo
(`apkc/javaemisor_argumento_estatico_test.go:57`), que **se salta en silencio** en wasm. Ya estaba
antes de este commit, lo trata otro frente y no se da por cerrado aquí.

```sh
# los ocho ficheros con tag, y la cuenta de saltos nuevos: la segunda línea debe dar 0
grep -n 'go:build !js' apkc/javaemisor_*_paridad_test.go
git diff -U0 7cb559b^ 7cb559b | grep -c "t.Skip"
```

```sh
# las contrapartidas sin tag y el helper que se queda fuera del tag
grep -n 'func Test' apkc/javaemisor_alcance_test.go apkc/javaemisor_atras_test.go
grep -n 'func javac17Exigido' apkc/dependencias_test.go
```

```sh
# el veredicto de la crítica, por capas (JAVAC17 apuntando al javac 17)
JAVAC17=/ruta/al/javac17 tcr -- sh tools/entorno.sh test   # ok … 183.986s
tcr -- sh tools/entorno.sh wasm                            # 2 --- FAIL, ambos multidex
```

```sh
# lo medido en esta sesión: los dos vet en verde, el diff del commit limpio y gofmt
cd apkc
GOOS=js GOARCH=wasm CGO_ENABLED=0 go vet .
go vet .
gofmt -l javaemisor_constantvalue_long_test.go   # deuda previa, fuera del commit
cd ..
git diff --check af7686b^ af7686b
git diff --name-only 7cb559b^ 7cb559b -- 'apkc/*_test.go' | xargs gofmt -l   # sin nombres
```

## 284. La capa wasm deja de saltarse oráculos y queda sin fallos

Registrado el 2026-11-24 sobre `8d93ada`. Dos frentes se cierran en este `§`: `4bbbf080`
(integrado como `8d93ada`) y `8e0b849` (integrado como `7f24a12`). Las cifras de la capa wasm
y de la suite nativa están transferidas de los builders y de las críticas; no se re-ejecutaron
ni la capa wasm ni ninguna suite en esta sesión, que es solo de documentación.

**A. 87 oráculos sin skip silencioso.** Antes, `grep -rn 'javac17(t)' apkc/*_test.go` daba
74 call sites en 33 ficheros; con el cierre transitivo —cinco ayudantes— afectaba a 87 pruebas
en 37 ficheros, todas `SKIP` silencioso bajo wasm. Después: **0** call sites. Los oráculos usan
`javac17Exigido` (`apkc/dependencias_test.go:41`), que termina en `t.Fatalf` y no en `t.Skip`:
sin `javac` la prueba falla en vez de disfrazarse de verde.

**Dónde quedan.** Las 87 pruebas pasan a ficheros `*_paridad_test.go` con `//go:build !js`
en `:1`, y las contrapartidas puras-Go sin tag se conservan. El árbol final tiene 44 ficheros
nuevos con esa cabecera (el último, `javaemisor_else_goto_paridad_test.go`, tras `0d8df36`), 99
`func Test` y 86 call sites de `javac17Exigido`, todos confinados
en ficheros `!js` (las dos líneas restantes del `grep` son la definición y su comentario). El frente
del `else/goto` se integró después con un `javac17(t)` residual en el test nuevo; `d551d21` lo
cambió
a `javac17Exigido`, y `0d8df36` movió ese test a `javaemisor_else_goto_paridad_test.go` (`!js`)
porque su ayudante `compilaYDejaElDexDeJavac` vive en un fichero de paridad: sin eso el binario wasm
no compilaba. Con el cambio, los tres tests pasan y `GOOS=js GOARCH=wasm go vet .` está limpio.

**Lo que midió el builder (transferido).** La capa wasm pasa de 116 `SKIP` a 29, con `PASS`
746 sin cambio y **2 `--- FAIL`**: los dos `multidex`, ya corregidos en `web` por `7f24a12`,
y sin aborto. En nativo, `ok … 173.030s`, 871 `PASS`, 0 `FAIL` y 6 `SKIP` ajenos.

**Veredicto de la crítica de A: APROBADA.** `ses_df1f421a6ffeyiBCvStOUCLWd2` aprobó `8d93ada`
con 265 funciones comparadas en 43 pares y **0 aserciones perdidas ni cambiadas**; el
`grep -c 't.Skip'` del diff da 3 por reubicación de un guard preexistente, no skips nuevos.
`go vet` nativo y wasm limpios; `gofmt -l` solo nombra el preexistente
`javaemisor_constantvalue_long_test.go`.

**B. Los dos últimos fallos, multidex.** `TestCompilaAppPre21ConDemasiadasClasesAborta` y
`TestCompilaAppSinUsesSdkConDemasiadasClasesAborta` venían de `exec.Command(os.Args[0])`
(`apkc/multidex_app_test.go:386`), imposible bajo wasm, donde no hay proceso que relanzar.
Pasan a `apkc/multidex_app_proceso_test.go` (`//go:build !js`) con el cuerpo idéntico. Sus
contrapartidas puras-Go, `TestDexPrimarioPre21QueNoCabeSeRechazaEnGo` (`:341`) y
`TestDexPrimarioSinUsesSdkQueNoCabeSeRechazaEnGo` (`:375`), comprueban los mismos mensajes
(`main-dex`, `65536`, `API 19`/`API 1`, `classes2.dex`) con 0 dex escritos.

**Medición transferida y crítica APROBADA.** El builder dio capa wasm `rc=0` y 0 `FAIL`, y
nativo `ok … 192.981s`, con `vet` y `gofmt` limpios. La crítica
`ses_df1f4df86ffeOiDMvdphcYLcTH` aprobó `7f24a12`: capa wasm
`tcr -r 256M -- sh tools/entorno.sh wasm` → final `PASS`, **0 `--- FAIL`**, sin `t.Skip`
nuevos; nativo `ok … 184.524s` con los dos tests movidos en `PASS`.

**Cierre.** La capa wasm final quedó con 0 `FAIL`, 29 `SKIP` y 805 `PASS`, `rc=0`, medidos por el
QA `ses_df1f12649ffeGtpp1o1HfFooeh` sobre `fe64417` (log
`~/.cache/apkc-web/wasm-cierre-final-v.log`).
Antes de `0d8df36` el binario ni compilaba: el test del `else/goto` llamaba a un ayudante de un
fichero `!js`; movido ese test a `javaemisor_else_goto_paridad_test.go`, el vet wasm está limpio.
Este `§` queda auditado con los dos veredictos de crítica recibidos y el recuento final.

**Deuda por diseño del tag.** Que el hijo real aborta el build solo se prueba en nativo; está
registrada en el mensaje del commit y en los comentarios del fichero, no como medición nueva.

```sh
# cero oráculos con skip silencioso, y el helper con Fatalf
grep -rn 'javac17(t)' apkc/*_test.go                       # 0
grep -n 'func javac17Exigido' apkc/dependencias_test.go     # :41
```

```sh
# 43 cabeceras !js en :1, 99 pruebas y 85 call sites confinados
for f in apkc/*_paridad_test.go; do head -n1 "$f"; done | sort | uniq -c
grep -h 'func Test' apkc/*_paridad_test.go | wc -l
grep -rn 'javac17Exigido' apkc/*_test.go | grep -v 'paridad_test.go' | wc -l   # 0
```

```sh
# el frente multidex: el proceso queda fuera del tag, las contrapartidas dentro
head -n1 apkc/multidex_app_proceso_test.go
grep -n '^func Test' apkc/multidex_app_test.go apkc/multidex_app_proceso_test.go
git show 7f24a12^:apkc/multidex_app_test.go | sed -n '386p'
```

```sh
# capa wasm y suite nativa, tal como las midieron los builders (no re-ejecutadas aquí)
tcr -r 256M -- sh tools/entorno.sh wasm
JAVAC17=/ruta/al/javac17 tcr -- sh tools/entorno.sh test
```

## 285. La página permite activar o desactivar la compilación incremental

Registrado el 2026-11-24 por transferencia del builder `ses_df1d6c613ffeGmQsTnaFzeaRCi`
(commit del submódulo `551a478`, gitlink actualizado por `c324611` en el
superproyecto) y de la crítica
`ses_df1c97e9bffe9Azxx7mOzRggVb`, que APROBÓ sin `CONFIRMADO`. Solo documentación: no se
re-ejecutó nada aquí, sin gate final, chromium, red ni publicación.

**Antes y después.** Antes, `pestana.js:3033-3050` siempre pasaba `--inc`; solo las pruebas
podían pedir `{incremental:false}`. Ahora `index.html:849-850` ofrece la casilla
`#inc-compilar`, marcada por defecto (`checked`), con la nota «Reutiliza las cachés del build
anterior; destildarlo fuerza una compilación completa».

**La regla de prioridad.** `pestana.js:3043-3045` decide con la expresión siguiente:

```js
const inc = opciones.incremental === undefined
  ? (casillaInc ? casillaInc.checked : true)
  : opciones.incremental !== false
```

Manda la opción explícita de las pruebas; si falta, manda la casilla; si no hay casilla en el
DOM, se cae a `true`, que es el comportamiento anterior. El comentario se reescribió: ya no
dice que la página siempre compila con `--inc`.

**Lo que midió la prueba CDP (transferido).** `node --check` limpio en los dos `.js`;
`node prueba.js` → `prueba: todo en orden`, `reglas: 24`; `prueba-pestana.js` con Brave →
**236 comprobaciones bien, 0 mal**. En la sección de `--inc`: 5 builds con `--inc` y 3 sin
él; al destildar se quita `--inc` y la apk sigue válida (`codigo 0`, 7 entradas,
`distintas: []`). El gate de red sigue en cero peticiones ajenas: 469 locales y 0
websockets.

**Veredicto de la crítica: APROBAR, sin `CONFIRMADO`.** La opción explícita manda, el
fallback sin casilla es seguro, el gate de red no se relajó, y los ids y la sintaxis son
correctos; el puntero del submódulo quedó en `551a478` y limpio.

**Memoria del frente.** El Brave headless de la prueba salió solo al terminar: no hubo que
matar procesos ni se tocó el Brave del usuario. El consumo temporal fue de la prueba CDP,
no un defecto del cambio.

```sh
cd studio && node --check pestana.js && node --check prueba-pestana.js
cd studio && node prueba.js
cd studio && tcr -c 4 -m 3G env CHROME_BIN=/usr/bin/brave … node prueba-pestana.js
```

## 286. La última fila de `LineNumberTable` y el `getAdapter` del stub

Registrado el 2026-11-24 por transferencia de dos frentes: `e47948b` (integrado como `7ca4675`)
y `750df74` (integrado como `0809ad6`, la cabecera de este worktree). Las mediciones son de los
builders y de las críticas; no se re-ejecutó aquí ningún build ni suite. Sin gate final, chromium,
red, push ni publicación: solo documentación. La crítica del frente A no pudo localizar el APK ni
confirmar el recuento y la del frente B sigue en curso, así que ninguna se da por aprobada.

**A. Cierre 21/21 de ApkShare (`e47948b` → `7ca4675`).** La fila que faltaba en §282 no era de la
lambda: es un local con rango vivo abierto al cerrar el bloque `then` (`Gen.visitBlock`,
`Code.curCP`/`markStatBegin`). Con un local con valor, javac 17 anota la línea del `} else {`
sobre el `goto`; no la anota con una sola sentencia, con una llamada con lambda de argumento, con
`break`, sin `else` ni con un local sin inicializador. La segunda divergencia era el orden: el
implementador de la lambda va entre los puentes covariantes y los `access$NNN`, no detrás de
estos; sin eso el `code_item` cambiaba de sitio.

**Los cambios (por el builder).** `javaemisor.go:2572` (`ambito.almacena`), `:3132`
(`almacenaEnBloque`), `:8525` (`filaDeCierre`), `:6948` (`branca`), el reorden en `:8978` y el
interruptor en `:2993`; `javafuente.go:233` y `:1993` capturan la línea del `}`. El test
`apkc/javaemisor_else_goto_paridad_test.go` trae 3 pruebas: antes daban `7a8 > line 159: 155`
y después las tablas salen sin diferencias. Se movió a `!js` en `0d8df36` porque su ayudante
vive en un fichero de paridad; `d551d21` corrigió `javac17(t)`.

**Medición del builder, transferida.** 21 de 21 clases byte a byte; `classes.dex` de 16504 B,
igual al oráculo; APK con SHA-1 `5cbdca5d…` verificado con apksigner v1/v2/v3; suite
`ok … 185.994s`. La crítica `ses_df1e219b0ffe8ZcuS0mO6KnGwM` tiene la regla y los tests bien,
pero NO pudo verificar el artefacto ni el recuento: el 21/21 queda como medición del builder,
no como verificado por la crítica (no localizó el APK con ese hash).

**B. `getAdapter` en la pestaña (`750df74` → `0809ad6`).** El usuario, con `--inc` y
`/sdk/android.jar`, recibió `ListView.getAdapter tiene 2 sobrecargas aplicables en la fase
estricta a (()Landroid/widget/ListAdapter; ()LT;) () y ninguna es la mas especifica`, desde
`MainActivity.java:55` (literal de la herramienta, sin acentuar). La causa no era `--inc`: el
stub AOSP que sirve la página declara `AdapterView.getAdapter` como `()LT;`, mientras el SDK
dice `()Landroid/widget/Adapter;`; el rechazo estaba en `javaemisor.go:1491` (`elegirSobrecarga`).

**El oráculo javac 17.** Emite `invokevirtual ListView.getAdapter:()ListAdapter`,
`invokeinterface getItem` y `checkcast App`; sobre `AdapterView<T>` usa la genérica con
`checkcast BaseAdapter`. El arreglo añade `candidato.dueño` (`:1453`), `esVueltaVariable`
(`:1730`), `masEstrictaSobreGenerica` (`:1752`, con `subclaseDe` en `:1799`), el enganche en
`:1637` y el interruptor `desempataGenerica` en `:1650`. Fail-safe: se abstiene si falta dueño,
parentesco o hay más de una concreta. El test `apkc/javaemisor_vuelta_generica_test.go`
(`!js`, 4 pruebas) cubre el byte a byte, el control con `AdapterView<T>`, la falsificación con
el error literal y el fail-safe.

**Medición del builder, transferida.** `build --inc` con el estado descartado y el jar híbrido
da verde: 21 clases, 16520 B y apk `e64fded9…`; con el jar del SDK también verde; suite
`ok … 142.142s`. La crítica de este frente está en curso: no se da por aprobada.

**Deudas abiertas.** El stub AOSP crudo para en `Consumer.andThen` (sin `Signature`,
`lambda_fuente.go:143` y `:176-193`); con una vuelta `LT;` sobre `AdapterView<T>` no se escribe
el `checkcast` al tipo instanciado.

**Fronteras de este `§`.** El 21/21 y las cifras de APK y suite son mediciones del builder: la
crítica A no localizó el APK ni confirmó el recuento y la crítica B está en curso. El stub AOSP
crudo y el `checkcast` sobre `AdapterView<T>` siguen abiertos. No se corrió gate, chromium, red
ni publicación.

```sh
# tests puntuales de los dos frentes (JAVAC17 apunta al javac 17 del oráculo)
cd apkc
TESTS_A='TestLaLineaDelElseVaEnElGotoDelThen'
TESTS_A="$TESTS_A|TestElDexDeMainActivityConElseEsElDeJavac17"
TESTS_A="$TESTS_A|TestLaFilaDelElseSeVeEnElDiffDeJavap"
JAVAC17=/ruta/al/javac17 GOTMPDIR="$HOME/.cache/apkc-web/gotmp" \
  tcr go test -run "$TESTS_A" -count=1 -v .
TESTS_B='TestElDexDeLaVueltaGenericaSinBorradoEsElDeJavac17'
TESTS_B="$TESTS_B|TestElReceptorAdapterViewSigueUsandoLaGenerica"
TESTS_B="$TESTS_B|TestFalsificacionDeLaVueltaGenerica"
TESTS_B="$TESTS_B|TestElDesempateGenericoSeAbstieneSinPrueba"
JAVAC17=/ruta/al/javac17 GOTMPDIR="$HOME/.cache/apkc-web/gotmp" \
  tcr go test -run "$TESTS_B" -count=1 -v .
cd ..
```

```sh
# javap -c del oráculo javac 17: la vuelta y el checkcast
JAVAC17=/ruta/al/javac17
"$JAVAC17" -g:lines,source -d "$TMP/clases-oraculo" "$TMP/fuente/MainActivity.java"
javap -c -p -classpath "$TMP/clases-oraculo" MainActivity \
  | grep -n 'getAdapter\|checkcast'
# build --inc con el estado descartado (caché nueva), tal como lo midió el builder
sh -c 'cd apkc && go build -o "$TMP/apkc-gate" .'
sh -c 'cd "$TMP/app" && "$TMP/apkc-gate" build --inc … --android-jar /sdk/android.jar'
```

```sh
# los símbolos que sostienen cada arreglo y las cabeceras de los tests
grep -n 'filaDeCierre\|almacenaEnBloque\|ambito.almacena' apkc/javaemisor.go
grep -n 'esVueltaVariable\|masEstrictaSobreGenerica' apkc/javaemisor.go
grep -n 'subclaseDe\|desempataGenerica' apkc/javaemisor.go
head -n1 apkc/javaemisor_else_goto_paridad_test.go apkc/javaemisor_vuelta_generica_test.go
```

## 287. Emulador con medidas reales, hot swap sin selector y compilación en un clic

Registrado el 2026-11-24 por transferencia de tres frentes de la pestaña: el emulador contra
Droidloom (builder `ses_df1b21fa4ffeg5Zau1AEaUjKIc`), el hot swap sin selector
(builder `ses_df1ac57c5ffeNSAdj8huY0JjNj`) y la compilación en un clic con API visible
(builder `ses_df1a687e0ffeBjmbbSr4XEcEAS`). Las mediciones son de los builders y de las críticas;
no se re-ejecutó aquí ningún CDP, build ni suite. Sin gate final, chromium, red, push ni
publicación: solo documentación. La temperatura es medición del orquestador con `sensors`/`ps`.

**A. El emulador contra Droidloom (`d84f175` → puntero `6ef4a5e`).** El builder midió en el cell
con `wm size 1920x1080` y `wm density 213` (1,33125 px/dp); la captura
`~/Develop/.tmp/droidloom/contador-real.png` da fondo `#FFFFFF`, texto `#30323A` 14sp
(Large 22sp), botón `#D6D7D7` 88×48 dp y `textAllCaps`. Antes el emulador usaba fondo oscuro,
botón hueco y 64×48; después los ajustes quedan en `vista-previa.js:24`, `:27`, `:122`/`:129`,
`:205`, `:255-260`, `:519`, `:561-574`, `vista-xml.js:29`/`:340` y `layout-editor.js:195`.

La crítica `ses_df1a24e7bffeW5SLRIRCjErwj8` RECHAZÓ una regresión: `vista-previa.js:212` había
cambiado a `Math.max(alto, …)` y con escala 0,5 el alto no escalaba (48 en vez de 24). Se volvió a
`Math.max(1, alto * caja.escala)`, con test en `prueba.js`; `node prueba.js` →
`prueba: todo en orden`. Corregido en `173c6bb`/`3dc1ed0`.

**Deuda del emulador.** Radio de ~2 dp, acento de 1 px, sin barra de estado ni de navegación y sin
densidad 213 real, tema claro fijo; `prueba-navegador.js:862-880` y `:967-970` aún no cubrían
escala <1 ni bordes (las citas `:780-795`/`:927-930` estaban desplazadas).
El CDP no se ejecutó por temperatura.

**B. Hot swap sin selector (`12e065a`/`1547b76`).** Builder `ses_df1ac57c5ffeNSAdj8huY0JjNj`,
crítica `ses_df1a11290ffe170DMklxG0qDu2`: APROBAR con condición CDP. Hay una casilla visible junto
a Compilar, la firma se deriva del `.java` activo, el `classes.dex` del apk recién compilado va a
OPFS y `--intercambiar` actúa en la sesión viva sin reiniciar; el selector avanzado se conserva.
`node --check` ×2 y `node prueba.js` en verde; CDP no ejecutada.

**Deuda del hot swap.** La primera compilación sin sesión aplica el swap en la siguiente, y por
defecto se intercambia la clase entera, no un método.

**C. Compilar en un clic y API visible (`2a5d8fe`).** Builder `ses_df1a687e0ffeBjmbbSr4XEcEAS`,
crítica en curso. `#alternar-panel` abre la pestaña Compilar y enfoca `#compilar-debug`;
`#api-version` y los ocultos suben al formulario de Inicio; el panel queda en orden Consola,
Compilar, Diagnóstico…; la etiqueta dice «Firmar apk o .aab». `node --check` 3/3 y
`node prueba.js` en verde; CDP no ejecutada.

**D. Temperatura (medición del orquestador).** El CPU llegó a 90 °C por dos
`droidloom-applications` colgados 20-22 min y por el CDP de las pruebas. Se terminaron los procesos,
Tctl 41.9 °C, y los seis agentes pasaron a pruebas focalizadas sin procesos en segundo plano.
Medido con `sensors`/`ps`.

**Fronteras de este `§`.** El CDP no se ejecutó en A, B ni C (por temperatura), así que ninguna de
las tres se da por verificada en navegador; la crítica de C sigue en curso y la de B aprobó con
condición CDP. La geometría (radio, acento, recorte) quedó cerrada en §293; siguen abiertas la
densidad de renderizado, la barra de estado y el tema, y las del hot swap (primera compilación,
swap de clase entera). Sin gate, chromium, red, push ni publicación.

```sh
# emulador: medición del cell (captura transferida)
wm size
wm density
# captura: ~/Develop/.tmp/droidloom/contador-real.png
cd studio && node prueba.js
# hot swap: sintaxis de dos .js y prueba focalizada
cd studio && node --check …
cd studio && node prueba.js
# compilación en un clic: sintaxis de tres .js y prueba focalizada
cd studio && node --check …
cd studio && node prueba.js
# ids citados en los tres frentes
grep -rn 'alternar-panel\|compilar-debug\|api-version\|intercambiar' studio/
# temperatura, medición del orquestador
sensors
ps -ef | grep droidloom-applications
```

## 288. Consola acoplable y teclado en los `EditText` del emulador

Registrado el 2026-11-24 por transferencia de dos frentes de la pestaña: la consola acoplable
(builder `ses_df1a71f64ffeK5MEMIR4sBHEhg`, `cee1bc5`/`bf6a688`) y el teclado en los `EditText`
del emulador (builder `ses_df1a60ad7ffeZM2piCQa6kqw7T`, `95dd4b3`/`75cf6c7`). Las mediciones
son de los builders y de las críticas; no se re-ejecutó aquí ningún CDP, build ni suite. Sin
gate final, chromium, red, push ni publicación: solo documentación. La temperatura está medida
en `§287` y no se repite aquí; solo se remite a ella.

**A. Consola acoplable (`cee1bc5`/`bf6a688`).** Antes solo se podía acoplar a izquierda o
derecha y no había tiradores; después hay cuatro bordes (`previa-panel.js:84-104`), ocho
tiradores (`index.html:635-666`, `previa-panel.js:1229`, `:1255`), persistencia en `localStorage`
y botón Soltar. La crítica `ses_df19bfbddffekF81pC8TN5cLmC` RECHAZÓ un fallo de especificidad:
`style.css:700` (`pointer-events:none`, especificidad 0,3,0 por los `:not()`) ganaba a `:702`
(0,2,0), así que ningún tirador recibía el puntero. Se arregló repitiendo la clase en el
selector (`.tirador-previa.tirador-previa`) y fijándolo en `prueba.js`; corregido en
`f252838` (submódulo) y `06583ac` (commit del superproyecto). `node prueba.js` → `prueba: todo en
orden`.

**Deuda de la consola.** Las ventanas ≤760 px ocultan los tiradores por una regla existente;
acoplar y redimensionar en navegador no está verificado (CDP no ejecutada por temperatura).

**B. Teclado en los `EditText` (`95dd4b3`/`75cf6c7`).** Builder
`ses_df1a60ad7ffeZM2piCQa6kqw7T`; crítica `ses_df19b08d1ffePGFWQT6U6OEgpS` APROBAR.
`apkc preview` ya acepta `--escribir id/texto` (`apkc/preview.go:267`, `:319-320`, `:869-897`);
la página añade `editableEnPunto` (`previa-panel.js:475`), `textosEscritos` (`:748`), campo
real (`:749-750`), foco y cierre (`:2749-2762`, `:4876`), y reenvía el guion (`:741-756`). No
muta el documento ni OPFS (contrato `AGENTS.md:188`). `node --check` ×4 y `node prueba.js` sin
fallos.

**Deuda del teclado.** La CDP no se ejecutó; el parser recorta los espacios al principio y al
final del texto; la salida duplicada al acoplar la consola tampoco está aseverada en
`prueba-navegador.js`.

**Fronteras de este `§`.** Ni la consola ni el teclado se dan por verificados en navegador:
la CDP no se ejecutó en A ni en B por temperatura (medida en `§287`). La crítica de A RECHAZÓ
un fallo de especificidad ya corregido; la de B APROBÓ. Siguen abiertas las deudas de la consola
(≤760 px, acoplar/redimensionar sin verificar y salida duplicada sin aseverar) y del teclado (CDP y
espacios recortados). Sin gate, chromium, red, push ni publicación.

```sh
# consola: prueba focalizada
cd studio && node prueba.js
# teclado: sintaxis de cuatro .js y prueba focalizada
cd studio && node --check …
cd studio && node prueba.js
# ids y claves citados en los dos frentes
grep -rn 'tirador-previa\|editableEnPunto\|textosEscritos' studio/
grep -rn -- '--escribir' apkc/
# temperatura: remitida a §287
```

## 289. Los dos avisos del intérprete de `preview`: permiso concedido e `invalidate` sin redibujado

Medido el 2026-11-24 sobre el binario compartido `46a9a792`, con las dos APK reales ya construidas
en `~/.cache/apkc-web/emular/`. Antes, `apkc preview` sobre `chatbt` daba **1 aviso** —
`com.hola.chatbt.MainActivity.checkSelfPermission(Ljava/lang/String;)I no está en el dex ni en los
stubs: se devuelve el cero de I`— y `apkc preview --pulsar id/lienzo` sobre `lienzo` daba **1 aviso**
—`com.hola.lienzo.Lienzo.invalidate()V … se devuelve el cero de V`—. Después de los dos stubs
(`apkc/previewinterprete.go`, casos `checkSelfPermission` e `invalidate`), los mismos comandos dan
**0 avisos** en las dos APK.

`checkSelfPermission` devuelve `int32(0)`, que es `PERMISSION_GRANTED`: el valor que ya entregaba el
cero implícito, así que la rama «concedido» no se mueve. Con el guion
`--escribir id/entrada/hola --pulsar id/enviar` las capturas antes y después son las mismas cinco,
incluida `Bluetooth listo - 0 vinculos - 2 mensajes`. `invalidate` devuelve el cero de `V` sin
captura ni contador: `pintaVistasPropias` (`apkc/previewdibujo.go:102-127`) ejecuta el `onDraw` una
sola vez al construir la pantalla, con una instancia nueva de la vista
(`apkc/previewdibujo.go:133-169`); no hay ciclo de fotogramas ni estado de la instancia que viaje,
así que programar un redibujado sería inventar lo que el modelo no puede medir. Tras el clic en
`id/lienzo` siguen las 17 operaciones y los textos `fotograma 0`, `toques 0`, igual que antes.

Los tests (`apkc/previewstubspermisos_test.go`) comprueban el valor y la ausencia de aviso con
`intentaStub`, y falsifican el aviso con el interruptor `stubsFrameworkActivos=false` por la vía
real de `invoca`. `go test ./...` → `ok github.com/ajinasokan/apkc` en las dos pasadas
(238.447s y 184.087s).

```sh
cd apkc && go build -o ~/.cache/apkc-web/apkc-preview-base .
cd apkc && go build -o ~/.cache/apkc-web/apkc-stubs .
cd apkc && go test -run 'TestStubCheckSelfPermissionConcedido|TestStubInvalidateSinHuella|TestFalsificacionAvisoDeLosDosStubs' -v .
cd apkc && go test ./...
# antes / después, con cada binario
~/.cache/apkc-web/apkc-preview-base preview ~/.cache/apkc-web/emular/chatbt/proyecto/build/app.apk > ~/.cache/apkc-web/chatbt-antes.json
~/.cache/apkc-web/apkc-stubs preview ~/.cache/apkc-web/emular/chatbt/proyecto/build/app.apk > ~/.cache/apkc-web/chatbt-despues.json
python3 -c 'import json,os;f=lambda p:len(json.load(open(os.path.expanduser(p)))["ejecucion"].get("avisos",[]));print(f("~/.cache/apkc-web/chatbt-antes.json"),f("~/.cache/apkc-web/chatbt-despues.json"))'  # 1 0
~/.cache/apkc-web/apkc-preview-base preview --pulsar id/lienzo ~/.cache/apkc-web/emular/lienzo/final/com.hola.lienzo/build/app.apk > ~/.cache/apkc-web/lienzo-antes.json
~/.cache/apkc-web/apkc-stubs preview --pulsar id/lienzo ~/.cache/apkc-web/emular/lienzo/final/com.hola.lienzo/build/app.apk > ~/.cache/apkc-web/lienzo-despues.json
python3 -c 'import json,os;f=lambda p:len(json.load(open(os.path.expanduser(p)))["ejecucion"].get("avisos",[]));print(f("~/.cache/apkc-web/lienzo-antes.json"),f("~/.cache/apkc-web/lienzo-despues.json"))'  # 1 0
# los efectos no cambian: mismo guion de chatbt con ambos binarios
~/.cache/apkc-web/apkc-preview-base preview --escribir id/entrada/hola --pulsar id/enviar ~/.cache/apkc-web/emular/chatbt/proyecto/build/app.apk > ~/.cache/apkc-web/chatbt-g-base.json
~/.cache/apkc-web/apkc-stubs preview --escribir id/entrada/hola --pulsar id/enviar ~/.cache/apkc-web/emular/chatbt/proyecto/build/app.apk > ~/.cache/apkc-web/chatbt-g-stubs.json
python3 - <<'PY'
import json, os
c = lambda p: json.load(open(os.path.expanduser(p)))["ejecucion"]["capturas"]
a, b = c("~/.cache/apkc-web/chatbt-g-base.json"), c("~/.cache/apkc-web/chatbt-g-stubs.json")
print("chatbt capturas:", len(a), len(b), [x.get("texto") for x in a] == [x.get("texto") for x in b])
print("chatbt ultima:", a[-1].get("texto"))
l = lambda p: json.load(open(os.path.expanduser(p)))["raiz"]["dibujo"]["operaciones"]
x, y = l("~/.cache/apkc-web/lienzo-antes.json"), l("~/.cache/apkc-web/lienzo-despues.json")
print("lienzo operaciones:", len(x), len(y), [o.get("texto") for o in x if o.get("operacion") == "texto"])
PY
# chatbt capturas: 5 5 True ; chatbt ultima: Bluetooth listo - 0 vinculos - 2 mensajes
# lienzo operaciones: 17 17 ['fotograma 0', 'toques 0']
```

## Fronteras del bloque, sin cerrar

Ninguna de estas cinco se cierra en `§245`-`§252`; se dejan escritas para que no se reabran por
analogía:

1. **ApkShare no compila entero (superado por `§269`).** Esta fue la frontera en `§253`-`§258`;
   `§263`, `§265`, `§268` y `§269` la cerraron hasta el build completo con `rc=0` y 21 clases.
   En `§253` pasó la 187, la 218 y `AppAdapter`, y se paraba en
   `MainActivity.java:55`, columna 44, con la captura de `apps` en una anónima; el puente `getItem`
   del retorno covariante y su `check-cast` ya cerraron en `ee5a372`. `§256` avanza esa frontera a
   `MainActivity.java:74`, con la captura de `adapter` en una anónima anidada; `§257` la deja en la
   interfaz de la anónima (línea 99) y `§258` fija la parada en `MainActivity.java:97`, con el
   `int`→`long` del `postDelayed`.
2. **Asignación como valor sobre campo o arreglo**, rechazada por nombre, pendiente del
   `dup_x1`/`dup2_x1` (y del `dup_x2` del elemento).
3. **`access$` más lambda en la misma clase** no está medido; el fuente con `access$` difiere igual
   sin lambda, así que no hay defecto atribuible todavía.
4. **Dependencias y runtime viejo:** el AAR de `androidx.multidex`, `Application.attachBaseContext`
   y un Dalvik de API ≤ 20 donde arrancar; hoy el oráculo es el contrato de bytes.
5. **CI remoto y publicación:** los commits nuevos no se empujaron y `apkc-studio.surge.sh` no se
   actualizó; las Actions fijadas por SHA no se miden sin red.

**Adjudicado el 2026-11-24 (`ses_df3cecf21ffekb35M5vY64tX8G`): el cruce del receptor heredado con
concatenación no es defecto.** `usa(getPackageName() + resto)` no falsifica
`estimaMetodoDeExterior` (`apkc/javaemisor.go:5472-5477`, §261): `esConcatenacion` (`:3508-3525`)
tipa el `+` como `String` por el operando derecho, y `tieneCadena` decide con eso sin bajar al
operando izquierdo, así que la estimación no llega a pedir el tipo de `getPackageName()` y
`candidatasSinReceptor` (`:5496`) no interviene. El interruptor es de sonda, no de política, y la
emisión (`:4390-4447`) ya coincide con javac: el camino del receptor implícito resuelve la clase que
encierra, y el `javap` de la sonda lo enseña como
`invokevirtual p/Pantalla.getPackageName:()Ljava/lang/String;` dentro de `Pantalla$1.onClick`. La
frontera falsificable es el control **sin** concatenación, `usa(getPackageName())`, que sí rechaza
en rojo (`apkc/javaemisor_receptor_heredado_test.go:110-134`, la falsificación del §261).

**Lo que midió la sonda (`ses_df3d30ee4ffePBOIEWCOLHvSBu`)**, fuera del repo, en
`~/.cache/apkc-web/auditoria-cruce/`, y metida con `-overlay`: con el interruptor encendido javac 17
acepta el cruce, las instrucciones de `onClick` son idénticas y el dex también tras normalizar solo
el `<init>` de la anónima (§100); con el interruptor apagado la compilación **sigue aceptándose** y
el `onClick` sale byte a byte igual al de la ruta encendida; el control sin concatenación sí
rechaza, con `no se puede estimar el tipo de getPackageName(): hay 0 candidatas…`;
`filtraCandidatasPorTipos=false` también se acepta, porque la sobrecarga no depende de él en este
cruce. Veredicto: **no defecto / condición de test**, y no hubo cambio de código.

**Pregunta abierta.** ¿Se documenta esa distinción —la estimación con interruptor, la emisión ya
convergente con javac— o se extiende el gating a la emisión?

```sh
# forma general: la sonda vive fuera del repo y entra por overlay
cd apkc && go test -overlay=<overlay.json> -run TestAuditoriaCruce -count=1 -v .
```

## 290. `apkc preview --locale`: idioma en los valores, sin cambiar la identidad de los recursos

Medición previa, en `46a9a792`: la APK de `traducido` (`string/saludo`) mostraba `Hello!` en el
modelo de `apkc preview` mientras `aapt2 dump resources` ya veía `(es) "Hola!"`. El lector elegía
la configuración base (`apkc/recursos.go:604`, `:624`), no había flag (`apkc/preview.go:267`) y
`esConfiguracionBase` (`apkc/preview.go:2545`) excluía `values-es`. Ahora `tablaRecursos` lleva
`locale` (`apkc/recursos.go:390`), que el flag asigna tras `leeTabla` (`apkc/preview.go:645`), y
`valorDeId` (`apkc/recursos.go:754`) elige el bloque por `language` en +8/+9 y `country` en +10/+11
(§94, §107): prefiere el país exacto al país vacío y cae a la base si la entrada localizada falta,
está `ausente` o es compleja. No se tocaron `bloqueDeTipo` ni `entradaDeId`, así que los ids, las
rutas y los drawables siguen resolviéndose contra la base; el idioma solo cambia valores. Sin flag,
la cadena vacía conserva el contrato anterior. El binario se construyó con
`tcr -c 3 -m 2G -- go build -o "$HOME/.cache/apkc-web/apkc-preview" .` desde `apkc/`.

Cinco comandos sobre el árbol final, con `APKC="$HOME/.cache/apkc-web/apkc-preview"`,
`APK="$HOME/.cache/apkc-web/tmp/studio-e2e-traducido/com.hola.traducido/build/app.apk"` y
`CONTADOR="$HOME/.cache/apkc-web/tmp/studio-e2e-contador/com.hola.contador/build/app.apk"`. Las
salidas de `preview` se resumen al nodo `id/saludo` (o a los dos nodos del contador) y se omite la
línea de tiempo, que varía entre ejecuciones:

```sh
cd apkc && go test -run TestPreviewLeeValoresDelIdioma -v .
cd apkc && go test ./...
"$APKC" preview --locale es "$APK" | python3 -c 'import json,sys; m=json.load(sys.stdin); n=m["raiz"]["hijos"][0]; print(n["id"], n["texto"])'
"$APKC" preview "$APK" | python3 -c 'import json,sys; m=json.load(sys.stdin); n=m["raiz"]["hijos"][0]; print(n["id"], n["texto"])'
"$APKC" preview --locale es "$CONTADOR" | python3 -c 'import json,sys; m=json.load(sys.stdin); print([(n["id"], n["texto"]) for n in m["raiz"]["hijos"]])'
```

```text
=== RUN   TestPreviewLeeValoresDelIdioma
--- PASS: TestPreviewLeeValoresDelIdioma (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	(cached)
ok  	github.com/ajinasokan/apkc	235.323s
id/saludo Hola!
id/saludo Hello!
[('id/total', 'Pulsos: 0'), ('id/pulsar', 'Pulsar')]
```

`--locale es` lee `values-es` y `--locale es-MX` prefiere `values-es-rMX` (comprobado además por
`TestPreviewLeeValoresDelIdioma` con la sonda escrita por el propio fork). Sin flag el nodo sigue
diciendo `Hello!`, y el contador, que no tiene bloque `es`, sigue funcionando con `--locale es` por
el respaldo a la base. Frontera: el selector de idioma en la pestaña y las regiones compuestas
(junto a idioma, densidad o orientación) quedan fuera de este párrafo; tampoco se repitió aquí el
gate con chromium ni se publicó nada.

## 291. Las diez plantillas compilan y se emulan con `apkc preview`

Medido el 2026-11-24 sobre el generador `buildProject` de `studio/app.js`: las diez plantillas
`hola`, `contador`, `ciclo`, `menu`, `traducido`, `lienzo`, `notas`, `chatbt`, `tareas` y
`cronometro` producen un proyecto que `apkc build --linker go --compiler go` convierte en APK y
que `apkc preview` emula sin `error`. Cuatro no salían tal cual: `chatbt` dejaba el hueco 2 en
`pedirPermisos` (`203fce6`), `notas` el hueco 2 (`0920540`), `tareas` el hueco 3 (`2376185`) y
`lienzo` no tenía layout para `preview` (`d1bd8ab`); además, las cinco plantillas nuevas
(`lienzo`, `notas`, `chatbt`, `tareas`, `cronometro`) no figuraban en `e2e.sh`. Los contratos de
emulación los fijaron `096aee7` (lienzo), `be8ea8c` (cronómetro), `c374a14` (contador), `92b1b6d`
(menú), `c933421` (notas), `1e3d365` (chatbt) y `b5ff4d9` (tareas), todos del submódulo `studio/`.

La receta es la de `studio/e2e.sh:86-116`: sembrar con el propio `buildProject`, compilar con el
compilador Go y rehacer el APK si el generador es más nuevo que él. Cada plantilla elige su guion
en `studio/e2e.sh:288-301`, y el bloque 0d (`studio/e2e.sh:302-333`) comprueba el JSON resultante.

```sh
# studio/e2e.sh:86-116
sembrar_proyecto "$destino" "$paquete" "$plantilla"
#   FIXTURE_DIR/FIXTURE_PKG/FIXTURE_TEMPLATE → node --input-type=module -e
#   import { buildProject } from "./studio/app.js" → escribe p.files y p.dirs
compilar_fixture "$destino"
#   (cd "$destino" && "$APKC" build --linker go --compiler go \
#     --android-jar "$ANDROID_JAR" --sdk-version 34 --codename 14 \
#     --keystore "$HOME/.android/debug.keystore")
fixture_fresco "$destino"   # siembra otra vez si app.js es más nuevo que build/app.apk
# studio/e2e.sh:288-312
guion=$(guion_de "$plantilla")
"$APKC" preview $guion "$destino/build/app.apk" \
  >"$TMP/studio-emular-$plantilla.json" 2>"$TMP/studio-emular-$plantilla.log"
# el JSON debe traer pantalla con ancho y alto > 0, nodos > 0, pasos > 0 y sin error
```

Las huellas leídas en los JSON de `preview`, con el APK recién compilado (se omite la línea de
tiempo, que varía entre ejecuciones):

| plantilla | guion de `preview` | APK (bytes) | huella medida |
| --- | --- | --- | --- |
| `hola` | `--avanzar 1000` | 9205 | `Hello World!` |
| `contador` | `--pulsar id/pulsar` | 10238 | `Pulsos: 1` |
| `ciclo` | `--recrear` | 9401 | `onCreate,onStart` |
| `menu` | `--menu --menu-item menu/0` | 9674 | `Menu pulsado` |
| `traducido` | (ninguno) | 9846 | `Hello!` |
| `lienzo` | (ninguno) | 11563 | 17 operaciones de dibujo |
| `notas` | `--escribir id/editor/TextoDePrueba --pulsar id/guardar` | 12943 | `cuantas:1` |
| `chatbt` | `--escribir id/entrada/hola --pulsar id/enviar` | 13498 | `yo: hola` + `eco: hola` |
| `tareas` | `--escribir id/nueva/Comprar --pulsar id/agregar` | 13581 | `Comprar leche` |
| `cronometro` | `--pulsar id/iniciar --avanzar 1000` | 12667 | `00:01.00` |

```text
lienzo  11563 B  17 operaciones
notas   12943 B  cuantas:1
chatbt  13498 B  yo: hola / eco: hola
tareas  13581 B  Comprar leche
cronometro 12667 B  00:01.00
contador 10238 B  Pulsos: 1
menu    9674 B  Menu pulsado
ciclo   9401 B  onCreate,onStart
traducido 9846 B  Hello!
hola    9205 B  Hello World!
```

**Frontera de `§291`.** `preview` es un modelo aproximado, no ART: la huella prueba que el guion
llega al nodo esperado, no que la app se vea igual en el aparato (eso lo mide `§293`). No se
repitió aquí el gate con chromium ni se publicó nada; los APK salen del directorio de temporales
del gate, no del sitio publicado.

## 292. `e2e.sh` compila y emula las diez plantillas en un solo gate

Los commits `5cfb5b5` y `fb69a51` (submódulo `studio/`) ampliaron el bucle 0b a las diez plantillas
y añadieron el bloque 0d: ya no basta con que `prueba.js` acepte el generador, el gate siembra cada
proyecto, compila su APK con `--linker go --compiler go` y ejecuta `apkc preview` con el guion de
`guion_de`. `fixture_fresco` (`studio/e2e.sh:113-116`) recompila cuando `app.js` es más nuevo que
`build/app.apk`, así que un APK prestado de otra sesión no puede disfrazar el generador vigente.
El bloque 0d (`studio/e2e.sh:316-332`) exige pantalla con dimensiones positivas, al menos un nodo,
pasos de ejecución mayores que cero y ausencia de `error`; imprime el resumen de nodos, pasos y
avisos por plantilla (`studio/e2e.sh:326-327`). `fb69a51` corrigió el guion de `Tareas` para
ejercitar `id/nueva` y `id/agregar`, en vez de un id que ya no existe.

```sh
sh -n studio/e2e.sh
node studio/prueba.js
# desde studio/, con APKC, ANDROID_JAR y BUILD_TOOLS del entorno
sh e2e.sh
```

```text
sh -n: sin salida, rc 0
prueba: todo en orden
```

**Frontera de `§292`.** La comprobación es del guion de gate, no de la página en chromium: el CDP
queda fuera de este párrafo y lo cubre `§293`. Los oráculos de firma y alineación del bucle 0b se
ejecutan sobre los APK sembrados aquí; no se empujó el submódulo ni se publicó el sitio.

## 293. Fidelidad del emulador web frente a la captura real de Droidloom

El commit `ed7db79` (submódulo `studio/`) cierra la deuda que `§287` dejó abierta. La captura real
del cell, con `wm size 1920x1080` y `wm density 213` (1,33125 px/dp), da un relleno gris `#d6d7d7`
en x=178..297 e y=408..460 px (≈119×52 px, ≈90×40 dp) y el área blanca en 480×776 px
(≈360×583 dp), no una pantalla de 360×640 dp. En cambio `apkc preview Contador.apk` entregaba
`Button(148, 308, 64, 48)` con centro en 180 dp: agrandar esa caja a 88 dp sin restituir el centro
desplazaba el botón 12 dp respecto de ART.

Los cambios están en `studio/vista-previa.js:31-32` (`ALTO_BOTON_DP=40` y `RADIO_BOTON_DP=4`),
`:197-226` (`modeloConMinimos` copia el modelo y restituye el centro: el botón queda en
(-12, 28, 88, 40)), `:286-295` (radio de 4 dp) y `:523-531` (`clip(marcoRect)`, para que el quinto
botón de `notas`, en x=359..447 dp, no pinte sobre el editor); en `studio/previa-panel.js:424-426`
(`modeloParaEmulador = modeloConPantalla(modeloConMinimos(modelo))`, mínimos antes de escalar),
`:2316` (limpieza del canvas cuando falla `preview`) y `:3053` (`modeloDelEmulador` usa ese mismo
modelo para el clic); y en `studio/vistas.js:29-34` el panel es el dueño único de `#vista-previa`.

```sh
# medición en el cell (captura transferida)
wm size
wm density
"$APKC" preview "$HOME/.cache/apkc-web/emular/contador/com.hola.contador/build/app.apk"
cd studio && node prueba.js && node prueba-navegador.js
# CDP sobre el wasm del árbol final
cd studio && CHROME_BIN=/usr/bin/brave \
  APKC_WASM="$HOME/.cache/apkc-web/cdp-final/apkc.wasm" node prueba-navegador.js
```

```text
Button(148, 308, 64, 48) → (-12, 28, 88, 40) → (-6, 14, 44, 20) a media pantalla
prueba: todo en orden
navegador: todo en orden
```

**Frontera de `§293`.** El CDP se ejecutó con `CHROME_BIN=/usr/bin/brave` y el `apkc.wasm` de
`~/.cache/apkc-web/cdp-final/`, no sobre el sitio publicado; el modelo sigue siendo aproximado
(sin barra de estado, tema claro fijo) y la densidad 213 se usa para interpretar la captura, no
para renderizar en la pestaña. Sin publicación.

## 294. La sección de Dependencias pasa a primer nivel sobre WISP

El commit `3f2b0e9` del submódulo `studio/` saca Dependencias del panel inferior de Compilar y la
coloca como vista de primer nivel: dos botones `data-vista=deps` (barra y franja), una sección
`<section id="deps">` hermana de las demás y el nombre `deps: "Dependencias"` en `vistas.js`
(`studio/index.html:124`, `:163`, `:1156`, `studio/prueba.js:734-742`). La prueba falsifica que
siga enganchada al panel (`studio/prueba.js:775-777`): no hay `data-panel="deps"` ni
`mostrarPanel("deps")`, los ids no se duplican y `sw.js` queda intacto. En CDP las pestañas salen
`["Inicio","Editor","Pantalla","Dependencias"]`.

Al resolver, el camino es `asegurarDecision()` → `resolverPorWisp()` (`studio/deps.js:512-522`):
si el núcleo WISP no está en esta compilación, se cae al índice local de coordenadas
(`studio/deps-coordenada.js`) y falla cerrado antes de abrir un socket; la falsificación
`studio/prueba-wisp-gates.mjs:163-172` mide `sockets:0, llamadas:0` sin núcleo y el motivo
acordado. El gate de red en CDP sigue sin abrir websockets ajenos al origen.

La compatibilidad con `apkc-deps` se mide sobre la gramática de coordenadas válidas, no sobre la
aceptación ciega: siete casos ejecutados con `SourceFileLoader` sobre
`~/Develop/Python/apkc-deps/apkc-deps` coinciden con `deps-coordenada.js` salvo `g:../a:1.0`.
Python lo acepta y su `relp` produce `g/../a/1.0/../a-1.0.jar`; la página lo rechaza en
`deps-coordenada.js:29`. El BOM y los espacios no son diferencia: ya se normalizan en `:91-97`
con aviso. La decisión `critico` (modo A) conserva el rechazo porque los guardias reales de
escritura están en `deps.js:46-57,156-157,206-224`, no en el parser.

```sh
cd studio && node prueba.js
cd studio && node prueba-navegador.js
cd studio && node prueba-wisp-gates.mjs
```

```text
prueba: todo en orden
navegador: todo en orden
falsificación: los dos gates rechazan lo que tienen que rechazar
sockets:0, llamadas:0
```

```sh
python3 - <<'PY'
from importlib.machinery import SourceFileLoader
m = SourceFileLoader("deps", "/home/leonisauro/Develop/Python/apkc-deps/apkc-deps").load_module()
for c in ["g:a:1.0", "g:a:1.0:cls", "g:a:[1.0]", "g:a:[1.0,2.0]",
          "g:a", "g:a:(,1.0],[1.2,)", "g:../a:1.0"]:
    try:
        print(c, "->", m.parse_coord(c, True))
    except SystemExit as e:
        print(c, "-> SystemExit", e.code)
PY
cd studio && node --input-type=module <<'JS'
import { parsearCoordenada } from "./deps-coordenada.js";
const casos = ["g:a:1.0", "g:a:1.0:cls", "g:a:[1.0]", "g:a:[1.0,2.0]",
  "g:a", "g:a:(,1.0],[1.2,)", "g:../a:1.0"];
for (const c of casos) {
  const r = parsearCoordenada(c, { permitirBom: true });
  console.log(c, "->", r.ok, r.coord || r.motivo);
}
JS
```

```text
seis casos con paridad; g:../a:1.0 -> Coord(group='g', artifact='../a', ...) en Python
g:../a:1.0 -> false "el artefacto «../a» no es válido" en la página
```

**Frontera de `§294`.** La resolución por WISP se mide con un núcleo simulado en node y con el
núcleo incrustado en CDP, nunca contra un relay real: los seis relays y sus latencias son datos
de la lista versionada, no una medición de red. Sin push del submódulo ni publicación.

## Cuadro de las cuatro mediciones nuevas

| § | commit | qué mide |
| --- | --- | --- |
| §291 | `096aee7` … `b5ff4d9` (11; ver §291) | diez plantillas: APK y huella |
| §292 | `5cfb5b5`, `fb69a51` | `e2e.sh` compila y emula las diez plantillas en un gate |
| §293 | `ed7db79` | fidelidad del emulador frente a Droidloom y CDP en verde |
| §294 | `3f2b0e9` (submódulo) | Dependencias como vista de primer nivel sobre WISP |

## 295. Hot swap, puente WISP y gates

### Hot swap

El `40` del hot swap es un **reporte del usuario**, no una medición:
nativo y CDP con el APK restituido dan `4 → swap → 4 → clic → 9`, pero
la crítica (modo A) confirmó que `prueba-navegador.js:767,791-794`
restituía el APK `+1` antes del swap, de modo que esa verde no cubre
la ruta real. Se corrigió la firma automática para derivar
`MainActivity$1.onClick` (commit submódulo `677af8e`); el requisito
abierto es guardar el estado y aplicar el código nuevo sin replay.

```text
nativo y CDP con APK restituido: 4 → swap → 4 → clic → 9
cobertura real: NO demostrada (crítica, modo A)
```

### Puente WISP

Los commits `f841220` (puente en wasm), `d02b2da` (proxy página-worker),
`de92a63` (validación de respuesta), `2146846` (forma del relay), `2146846`
y `6edde93` (destino canónico) se enumeran aquí como referencia; sus
comandos y salidas están en `§296`, no se repiten como medición nueva.

### Gates

```sh
cd studio && node prueba-wisp-gates.mjs
```

```text
falsificación: los dos gates rechazan lo que tienen que rechazar
modo externo: sin núcleo falla con el motivo y sin abrir socket — {"motivo":"núcleo WISP no disponible en esta compilación","sockets":0,"llamadas":0}
```

La cifra `29/29 ok` y el `websockets=0` de la sonda CDP son recuentos
de los logs de QA, no salidas del comando anterior; se citan como
informe, no como medición de este bloque. `tools/entorno.sh wasm` y
la sonda de versión/relay están registrados en `§296`.

**Frontera de `§295`.** Gate final, CI y publicación quedan fuera.
El puente real contra Maven solo se probó con núcleo simulado, no
con un relay externo; el `40` del hot swap sigue sin reproducirse.

## 296. Arreglo del AXML, latch del puente, destino canónico y validación

### Arreglo del AXML

El commit `da9c7de` reserva una celda `""` al final del pool del AXML cuando un atributo de texto
está vacío: sin ella el atributo apuntaba a la posición 0 del mapa, que en un layout es el primer
nombre con id (`orientation`), y la siembra del emulador inventaba texto para los `EditText`.
El fixture se mide byte a byte, el texto vacío se lee vacío y dos `EditText` conservan su `idRef`
distinto y capturan cada escritura por separado.

```sh
cd apkc && go test -v . \
  -run 'Test(EscribeAXMLContraFixture|ElTextoVacioDelEditText|LosDosEditTextConservanSuIdRef)$'
```

```text
=== RUN   TestEscribeAXMLContraFixture
--- PASS: TestEscribeAXMLContraFixture (0.00s)
=== RUN   TestElTextoVacioDelEditText
--- PASS: TestElTextoVacioDelEditText (0.00s)
=== RUN   TestLosDosEditTextConservanSuIdRef
--- PASS: TestLosDosEditTextConservanSuIdRef (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.004s
```

### Latch del puente

El commit `6938ceb` pega el puente del núcleo tras el primer contrato válido: el `activo:false`
de fin de comando ya no lo retira, así que `nucleoDisponible` sigue verdadero en reposo. Un
`obtener` sin instancia viva falla cerrado en el hospedador con su motivo, sin abrir socket.

```sh
cd studio && node prueba.js
```

```text
prueba: todo en orden
```

### Destino canónico

El commit `6edde93` trocea el grupo con barras en `destinoDeCoordenada`, igual que el núcleo
(`ReplaceAll(grupo, ".", "/")`) y no con puntos como la vía del índice; además rechaza los grupos
con algún segmento vacío. La coordenada de Gson da el mismo destino que el núcleo.

```sh
cd studio && node --input-type=module -e '
  import { destinoDeCoordenada } from "./wisp.js";
  console.log(destinoDeCoordenada("com.google.code.gson:gson:2.10.1"));
'
```

```text
/deps/com/google/code/gson/gson/2.10.1/
```

### Validación y contrato

El commit `de92a63` contrasta la respuesta del núcleo antes de pintarla: exige `Uint8Array` no
vacía y dentro del tope de 64 MiB, `sha256` hexadecimal de 64 dígitos en minúscula que coincide
con los bytes recibidos y `ruta` bajo `/deps/` sin `..`, contrabarra, `?` ni `#`. El commit
`f30b5cf` ajusta `DEPS.md` al puente registrado.

```sh
cd studio && node prueba-wisp-gates.mjs
```

```text
falsificación: los dos gates rechazan lo que tienen que rechazar
```

**Frontera de `§296`.** `e2e.sh` no se registra en verde: sigue **en verificación** (`docs/plan.md:60`), con `e2e: 1 problemas` en el gate autorizado por el `TypeError` del bloque `--inc` y `256 comprobaciones bien, 0 mal` (rc=0) en dos ejecuciones focales consecutivas del builder del WISP; el gate completo no se ha repetido (logs `~/.cache/apkc-web/tmp/pestana-focal-final.log` y `~/.cache/apkc-web/tmp/pestana-focal-final2.log`). El puente se
mide con núcleo simulado, nunca contra relays reales. Sin push del submódulo ni publicación.

## 297. Content-Length menor que el cuerpo en el puente WISP

### Arreglo

`getHTTPPuente` pide `Connection: close`, lee hasta el cierre con un `io.LimitReader` de
`topeCuerpoPuente+1` y compara el total de bytes con `Content-Length` antes de devolver nada.
`obtenerConConn` abre una conexión y un `bufio.Reader` nuevos por extensión: ni un rechazo
jar→aar→pom ni un timeout dejan bytes en un lector compartido. No hay token nuevo: el contrato
sigue siendo `apkc-wisp/1` (`puente_wisp.go:32`).

```sh
cd apkc && tcr -c 3 -m 2G -- go test -run TestPuenteWispObtenerConConnSinRespuestaParcial -v .
```

```text
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/sin_200
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/cuerpo_por_encima_del_tope
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/cuerpo_cortado
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/chunked
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/sin_Content-Length
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/estado_inventado
=== RUN   TestPuenteWispObtenerConConnSinRespuestaParcial/CL_3_/_cuerpo_pommas
--- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/sin_200 (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/cuerpo_por_encima_del_tope (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/cuerpo_cortado (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/chunked (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/sin_Content-Length (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/estado_inventado (0.00s)
    --- PASS: TestPuenteWispObtenerConConnSinRespuestaParcial/CL_3_/_cuerpo_pommas (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.004s
```

La suite completa, con las tres pruebas del puente ya corregidas, queda en verde:

```sh
cd apkc && tcr -c 3 -m 2G -- go test ./...
```

```text
ok  	github.com/ajinasokan/apkc	213.082s
```

### Riesgo aceptado

El aviso de `DEPS` es una puerta de UX de la página, no un límite de seguridad. Un script con
`globalThis.apkcWisp` puede invocar `obtener` sin pasar por ese aviso: el global se registra en
`init()` (`puente_wisp_js.go:36`) y el contrato se comprueba en `validarPeticionPuente`
(`puente_wisp.go:62`). El puente rechaza respuestas incoherentes, pero no impide que se pida una
coordenada válida; la política sigue siendo la allowlist de `ConfigWISP.Validar`.

**Frontera de `§297`.** Sin relay real, sin CDP, sin gate y sin publicación: solo `net.Pipe()` y la
suite nativa, además de la compilación `GOOS=js GOARCH=wasm` del lado registrado.

## 298. Cierre del puente de reposo, lienzo, pista y descripciones

```sh
cd apkc && tcr -c 3 -m 2G go test -run 'TestPuenteWisp|TestWispGet' -count=1 -v .
tcr -c 3 -m 2G node shim/prueba-wisp-get.js
cd studio && node prueba.js
```

```text
PASS / ok 0.012s
11 comprobaciones ok; la petición nunca viaja en argv; huella inválida => 0 WebSocket
prueba: todo en orden
```

`wisp-get` entra con los commits `8956450` (fork) y `80fc9d0` (submódulo):
un solo disparo con la petición en `go.env`, salida silenciada y bytes
transferidos como `ArrayBuffer`. La sonda CDP registró 7 peticiones,
ninguna fuera del origen, y 0 WebSocket sin activación.

El lienzo entra con `24b3462` (fork) y `82a93f5` (submódulo). Antes
`getWidth/getHeight/Color.rgb` daban cero y todo era `#00000000`;
después el centro es `(180,320)` y los colores son `#12161e`,
`#46beff`, `#ffb43c`, `#f0f0f0` y `#78f096`.

```sh
cd apkc && tcr -c 4 -m 3G go test -run TestPreviewLienzo -v .
```

```text
--- PASS: TestPreviewLienzo (0.13s)
ok github.com/ajinasokan/apkc 0.139s
```

`notas` entra con `82be596`: el nodo `id/editor` traía `pista:"Escribe
una nota"` y `texto:""`, que el pintor ignoraba. La sonda sobre el
modelo real, sin `textoDinamico`, pasó de `sinGuion:[]` a
`sinGuion:["Escribe una nota"]`. Las descripciones de las diez
plantillas entran con `b11c08d` (selector, `LEEME.txt` y prueba).

**Frontera de `§298`.** La suite Go completa queda pendiente; no se
ejecutó gate final, CI, publicación ni resolución contra un relay real.

## 299. El wasm vuelve a compilar, el corredor se centraliza y el gate destapa el bloque `--inc`

Medido el 2026-11-24 sobre `HEAD 0379182`. Cuatro frentes con evidencia de esta sesión: el binario
wasm, la suite Go completa, el corredor de tareas y el gate autorizado que destapó el bloque `--inc`.

### El import huérfano de `net`

`cff8013`: quitar `"net"` sin uso de `apkc/puente_wisp_js.go` (import huérfano desde `8956450`); el
vigilante de preview llevaba desde 20:05 UTC fallando con
`./puente_wisp_js.go:23:2: "net" imported and not used` y servía un wasm de ocho commits atrás.

```sh
cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .
```

```text
(sin salida, rc 0)
```

### Suite Go completa

```sh
cd apkc && tcr -c 3 -m 2G -r 200M -- go test ./...
```

```text
ok github.com/ajinasokan/apkc 184.017s
```

El guardia de RAM por defecto mató el primer intento con 137; `-r 200M` lo resolvió.

### Corredor

`e516bc2` mete el envoltorio de `tcr` y el `LD_LIBRARY_PATH` de chromium en `tools/gate-local.sh`;
`f5839e1` lo centraliza en `tools/entorno.sh`, con `tools/correr.sh` como punto de entrada único
para seleccionar la tarea.

```sh
sh tools/correr.sh tareas
sh tools/correr.sh corredor
```

```text
TCR=…/corredor/tcr
GUARDIA_MB=300
LD_LIBRARY_PATH=…/shimlib
```

```sh
sh tools/correr.sh wisp
```

```text
falsificación: los dos gates rechazan lo que tienen que rechazar
```

`node --check` limpio.

### Reparto

`bced0a4` reparte 297 `§` en 14 ficheros y convierte `docs/evidencia.md` en índice; verificación con
`awk`-diff rc 0, preámbulo rc 0, 297/297 y `comm` con solo `0` y `78`; regla 13 en `AGENTS.md`, regla
de force y 8 subagentes (`3fb0905`, `0379182`).

### Gate autorizado

```sh
sh tools/gate-local.sh
```

```text
e2e: 1 problemas
FALLO: la pestaña con apkc.wasm
TypeError: Cannot read properties of null (reading 'buffer') en proyecto.js:99 desde leerZip :146, bloque 12c (--inc)
```

Prueba focal posterior: 12c en verde, `256 comprobaciones bien, 0 mal` y rc=0 en dos ejecuciones
consecutivas del builder del WISP (`~/.cache/apkc-web/tmp/pestana-focal-final.log`, `~/.cache/apkc-web/tmp/pestana-focal-final2.log`). El gate
completo no se ha repetido: **no** se da `e2e` por verde.

**Frontera de `§299`.** Sin CDP final, sin segundo gate autorizado, sin push, CI ni publicación.

## 310. ApkShare compila con `--compiler go --linker go` y con `--inc` en nativo y en el hospedador wasm

Evidencia del investigador `ses_df0259c8dffeLhsY3BGfwNGW1M` y del QA `ses_df021ac1bffe2cDvBr6Et6b4cV`,
sobre binarios compilados desde `HEAD 0379182`. Los artefactos y scripts están en
`~/.cache/apkc-web/apkshare-hoy/`; el repo no se tocó.

**Nativo sin `--inc`.**

```text
6 fuentes de Java en Go -> 21 clases
21 clases -> 16504 bytes en build/classes.dex
```

APK `305501 B`; `apksigner verify --verbose` → `Verifies`, v1/v2/v3 `true`.

**Nativo con `--inc`.** Mismo dex byte a byte; segunda vuelta en el mismo directorio:

```text
10.935453ms
reutilización de clases
the package did not change
```

**Wasm con `--inc`.** `21 clases`, `16504 B`, `cmp` del dex contra el nativo sin salida; APK
`305500 B`.

**Wasm sin `--inc` (cuarta celda).** `16504 B`, SHA-256 `69fdb658…82db9` idéntico en los cuatro dex,
`cmp rc=0` contra nativo y wasm con `--inc`, APK `305502 B`, `apksigner verify --verbose` →
`Verifies`, v1/v2/v3 `true`.

**Frontera de `§310`.** La pestaña real con chromium no se re-ejecutó (§285 es transferida), la suite
Go completa y el gate no se corrieron aquí; `TestSondaHeredadaFuente` de §275 sigue sin verificar y
es independiente.

## 312. Go 1.24.0 fijado y wasm_exec.js verificado por hash

Evidencia del builder `ses_df01b4e96ffeVMMLgugTKw2KyN`. El usuario autorizó instalar `go1.24.0`,
quedarse en esa versión y documentarlo.

**Instalación** (tarball oficial, sin sudo, sin tocar el Go del sistema):

```sh
curl -fsSL --max-time 300 https://go.dev/dl/go1.24.0.linux-amd64.tar.gz -o "$HOME/.cache/apkc-web/go1.24.0.linux-amd64.tar.gz"
tar -C "$HOME/sdk" -xzf "$HOME/.cache/apkc-web/go1.24.0.linux-amd64.tar.gz"
mv "$HOME/sdk/go" "$HOME/sdk/go1.24.0"
```

**Versión fijada.**

```sh
GOROOT="$HOME/sdk/go1.24.0" "$HOME/sdk/go1.24.0/bin/go" version
```

```text
go version go1.24.0 linux/amd64
```

`go version` del sistema sigue `go1.27.4-X:nodwarf5`. No se borró nada.

**Hash de `wasm_exec.js`.**

```sh
sha256sum "$HOME/sdk/go1.24.0/lib/wasm/wasm_exec.js"
```

```text
0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14
```

Es byte-idéntico al del Go local (medido con ambos `sha256sum`), 16992 bytes.

```sh
sh tools/wasm-exec-hash.sh
```

```text
wasm_exec: sha256 0c949f… (go go1.24.0) OK
```

rc 0; con versión ajena o JS modificado → rc 1.

```sh
sh tools/entorno.sh -- sh -c 'printf "GOROOT=%s\n" "$GOROOT"; go version'
```

```text
GOROOT=/home/leonisauro/sdk/go1.24.0
go version go1.24.0 linux/amd64
```

(el toolchain se antepone al PATH en `tools/entorno.sh:30,62-69`).

**Decisión.** Permanecer en go1.24.0; no usar el Go del sistema para compilar el repo. Deudas abiertas,
no verdes: `gate.yml:85` (W1 debe sustituir el `cp` directo por el verificador) y `studio/servidor.js`
(cuarto sitio que sirve `wasm_exec.js` desde GOROOT). Sin gate, CI ni publicación.

## 313. Sesión viva de preview: una instancia, sin replay y sin error pegajoso

Evidencia del builder `ses_df0263325ffeds2w253F2b7Qds`, aprobada por el crítico
`ses_defdb3ccfffelZrraWRgzO7x7O` y ya integrada en el árbol principal (`93db6c3`). Archivos nuevos en
`apkc/`: `previewsesion.go`, `previewsesion_js.go`, `previewsesion_nojs.go`, `previewsesion_test.go` y
`sonda-sesion.js`; modificados `main.go`, `preview.go` y `previewinterprete.go`.

**Siete tests Go**, con Go 1.24.0 y `-count=1`: los siete dan `--- PASS` y la suite cierra con
`ok github.com/ajinasokan/apkc 0.190s`. Son `TestLaSesionAplicaSoloLaOrdenNueva`,
`TestLaSesionConservaElModeloAcumulado`, `TestLaSesionAplicaDeltaEnElCaminoReal`,
`TestLaSesionNoLlamaExitPorOrden`, `TestElSegundoDeltaUsaLaBaseActualizada`, `TestLaSionFallaCerrada` y
`TestLaSesionNoPegaElErrorDeUnaOrdenMala`.

**Orden y timeline.** La secuencia medida es `["Pulsos: 0","1","2","3","4","4","9"]`, con cinco
líneas `onClick` (no nueve); `salir=0` y un `apkc:fin-orden` por orden.

**Sonda node.** `APKC_GOROOT=… node apkc/sonda-sesion.js <wasm> <apk>`:

```text
ordenes: 1,2; fin-orden: [1,2]; instancias: 1; timeline: 0,1,2
```

Con `--ocupado` (o `APKC_SESION_OCUPADO=1`) da `TIMEOUT: sin fin-orden para la orden 2`, rc 1.

**El error pegajoso.** `actualizaRes()` limpia `res.Error` (`previewsesion.go:94`) y las dos sendas de
fallo lo reasignan después (`:205` y `:227`): la orden mala publica `error`, la buena siguiente no.

**Contrato de la sesión.** `go.argv=["apkc","preview","--sesion",id,apk]`, `APKC_SESION_ID`,
`apkcSesion.siguiente()`, `setTimeout(...,0)` más un canal para ceder al event loop, y `postMessage` de
`apkc:fin-orden` con id y código, sin `exit` por orden; `os.Exit(2)` queda solo en el wrapper.

**Fallo ajeno.** `TestIndiceDelJarSeReutilizaYSeInvalida` (`jarindice_test.go:151`, 416 frente a 414)
falla igual en árbol limpio; no es atribuible a este cambio.

**Frontera de `§313`.** No se corrió CDP ni gate: el timeline real en navegador y el reclamo del
reporte «40» siguen abiertos. No se da por cerrado el `40`.

## 314. Medición reproducible del incremental en wasm y nativo

Medida el 2026-11-24 por el QA `ses_defb50083ffeZC38H50IiYFd2B` sobre el árbol integrado `718637c`.

**Comandos.**

```sh
cd /home/leonisauro/Develop/Go/apkc-web
PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH GOTMPDIR=/home/leonisauro/.cache/apkc-web/gotmp sh banco/incremental-wasm.sh
REPETICIONES=1 PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH tcr -c 2 -m 2G -r 400M -- sh banco/incremental.sh
```

**Wasm.** `--- PASS: TestBuildDosVecesEnWasmMideEtapas (12.66s)`, `comparación entrada por entrada: 80 entradas, 0 con bytes distintos []`, cuatro builds con etapas: plain frío `aapt2 link 573ms / compile go 1.997s / dex go 814ms / package and sign go 277ms / total 3.782s`; plain caliente `108ms / 1.528s / 308ms / 123ms / 2.151s`, `1866 de 1875 code_item reutilizados`, `copied 76 entries`; inc frío `427ms / 1.891s / 641ms / 211ms / 3.297s` con `79 resource files changed: relinking`; inc caliente `compile go 1.903s / dex go 356ms / package and sign go 172ms / total 2.591s`, `1866 de 1875 reutilizados`. Sin `apkc devolvio 1`, `FAIL`, `error` ni `panic`.

**Nativo.** `r1-plain-frio 993`, `r1-plain-caliente 555`, `r1-inc-frio 808`, `r1-inc-caliente 486`, `r1-incremental.json-bytes 35579`; `dex por clase: la vuelta caliente y el dexado completo son identicos`; `entradas: 80, con bytes distintos: 0`; `Verifies`, v1/v2/v3 `true`, `zipalign: correcto`. Árbol limpio antes y después.

**Frontera de `§314`.** La medición 3 original usaba un parche que hoy está versionado en `shim/navegador-sim.js:285-307` y `shim/prueba-relink.js:14-134` (134 líneas); `/tmp/opencode/inc-web/` ya no existe. La capa wasm completa sigue con fallos ambientales (§279 15/RC=1, §284 0/29/805, `docs/evidencia.md:15-24` 32/38): **no** reclamar capa wasm verde, solo la prueba focal con jar montado. `REPETICIONES=1` deja vacías las medianas de tres repeticiones: es artefacto del guion, no fallo. El shim no es OPFS; sin gate final, chromium, red ni commits.

## 348. Navegación viva entre actividades en la sesión

Medido el 2026-11-24 sobre el árbol integrado en el commit `6496918`
(`fix(sesion): navega entre actividades y publica el recorrido de pantallas`). Quien escribe este
`§` no ejecutó tests, CDP, chromium, gate ni commits: la evidencia de abajo es la que llegó del
árbol ya integrado y del builder de D7.

**Test Go focal.** Con Go 1.24.0, sobre el árbol integrado:

```sh
cd apkc && env PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH GOROOT=/home/leonisauro/sdk/go1.24.0 go test -run TestLaSesionNavegaEntreActividades -count=1 -v .
```

da `--- PASS: TestLaSesionNavegaEntreActividades (0.02s)` y cierra con
`ok github.com/ajinasokan/apkc 0.232s`. El auditor falsó el test: al restaurar `previewsesion.go` a
`HEAD` falla con cinco aserciones (la actividad sigue siendo `MainActivity`, `pantallas:[MainActivity]`
y falta el `id/volver`), así que la prueba sí ejercita el arreglo.

**Qué cambia el código.** `apkc/previewsesion.go` gana `pila` y `pantallas` (`:23-28`);
`estabilizaSesion()` (`:83`) abre `m.pendiente` sin reenviar el guion; `aplicaEvento` (`:164-176`)
lo llama justo después de `entregaEvento`; y `actualizaRes` (`:188`) publica
`res.Pantallas = append([]string(nil), s.pantallas...)`. `finish()` con la pila vacía ya no paniquea.

**CDP del builder de D7 (no ejecutado por quien escribe).** El log
`~/.cache/apkc-web/tmp/cdp-d7.log` cierra con `247 comprobaciones bien, 0 mal` en `:262` y cinco
líneas `ok … (D7)` en `:127-131`. El total es 247 frente a los 258 de corridas anteriores porque
esta corrida no fijó `KS_RELEASE`, `AAB_EJEMPLO` ni `APKC_WISP_EXTERNO`; se registra tal cual, no se
oculta.

**Frontera de `§348`.** El único `FAIL` de `go test ./...` es
`TestIndiceDelJarSeReutilizaYSeInvalida` (`jarindice_test.go:151`, 416 frente a 414): preexistente y
reproducido en `HEAD`, no se presenta como verde. `gofmt -l` está limpio en los dos ficheros tocados
(a nivel global lista cuatro ficheros preexistentes no tocados). El CDP sobre el árbol integrado con
las variables completas queda pendiente de temperatura y autorización; sin CDP, chromium, gate ni
commits por este `§`.
