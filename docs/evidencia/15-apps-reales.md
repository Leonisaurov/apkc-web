## 354. Apps reales en `~/Downloads`: estado actual

Medido el 2026-11-24 en el worktree `apps-reales-fase-a`, siguiendo el plan aprobado
`…/plan-apps-reales/PLAN-apps-reales.md`. Sin CDP, chromium, gate, red, `git push` ni publicación.
El binario usado es el local `/home/leonisauro/.cache/apkc-web/emulador-fiel/apkc` (2026-11-24 20:06), anterior a
la Fase A (`§355`). Los tres comandos se reejecutaron en esta sesión; las salidas de abajo son propias,
no copiadas del plan.

**Identidad de los cuatro APK.**

```sh
sha256sum /home/leonisauro/Downloads/F-Droid.apk /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_x86_64.apk
```
```
dac55a0d17b1edf7e7286b48cfbd70c7c52fbb32a00af062c68830a437c47959  /home/leonisauro/Downloads/F-Droid.apk
576228432fd261ab3d34b30ea9d5a23b3d53022e18c98f5b5e2e18b885988f05  /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk
89416397b70f9ff67a0044e8abe6ef82487cd48fcf543e2d23e02688cc541cf0  /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
3550e61f4d9eb49b712fd1bd9519dc37085a4d8eb597c57a340f0a64859b7144  /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_x86_64.apk
```
Hay dos Termux (armeabi-v7a y x86_64); se mide el de arm. La variante x86_64 queda registrada por su
SHA, sin `apkc preview` propio: su comportamiento es **`incierto`**.

**F-Droid.**

```sh
/home/leonisauro/.cache/apkc-web/emulador-fiel/apkc preview /home/leonisauro/Downloads/F-Droid.apk
```
```
{"paquete":"org.fdroid.fdroid","version":"2.0.3","actividad":"org.fdroid.IconCalculatorActivity","layout":"res/A1.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":3,"ejecucion":{"metodo":"onCreate","pasos":0,"error":"L_COROUTINE/CoroutineDebuggingKt;: static values: value #0 of type 0x17 is not read by this fork yet"},"raiz":{"tipo":"LinearLayout","estilo":"style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"TextView","id":"id/action_bar_title","idRef":"0x7f08003b","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":32,"alto":24}},{"tipo":"TextView","id":"id/action_bar_subtitle","idRef":"0x7f08003a","ancho":"wrap_content","alto":"wrap_content","margen":{"izq":0,"arr":-3,"der":0,"aba":0},"rect":{"x":0,"y":21,"ancho":32,"alto":24}}]}}
[apkc] finished in 224.016525ms
```
`rc=0`. El error literal es `L_COROUTINE/CoroutineDebuggingKt;: static values: value #0 of type 0x17
is not read by this fork yet`; `pasos:0`, `layout:res/A1.xml`.

**NewPipe.**

```sh
/home/leonisauro/.cache/apkc-web/emulador-fiel/apkc preview /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk
```
```
{"paquete":"org.schabi.newpipe","version":"0.29.3","actividad":"org.schabi.newpipe.MainActivity","layout":"res/A1.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":3,"ejecucion":{"metodo":"onCreate","pasos":0,"error":"Landroidx/compose/foundation/layout/SpacerKt;: 4 static values for 8 static fields"},"raiz":{"tipo":"LinearLayout","estilo":"style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"TextView","id":"id/action_bar_title","idRef":"0x7f0a003d","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":32,"alto":24}},{"tipo":"TextView","id":"id/action_bar_subtitle","idRef":"0x7f0a003c","ancho":"wrap_content","alto":"wrap_content","margen":{"izq":0,"arr":-3,"der":0,"aba":0},"rect":{"x":0,"y":21,"ancho":32,"alto":24}}]}}
[apkc] finished in 335.573296ms
```
`rc=0`. El error literal es `Landroidx/compose/foundation/layout/SpacerKt;: 4 static values for 8
static fields`; `pasos:0`, `layout:res/A1.xml`.

**Termux arm.**

```sh
/home/leonisauro/.cache/apkc-web/emulador-fiel/apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
```
```
{"paquete":"com.termux","version":"0.118.3","actividad":"com.termux.app.TermuxActivity","layout":"res/layout/abc_action_bar_title_item.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":3,"ejecucion":{"pasos":0,"error":"el classes.dex no se deja leer: string #808 claims 18 utf-16 units and holds 20"},"raiz":{"tipo":"LinearLayout","estilo":"style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"TextView","id":"id/action_bar_title","idRef":"0x7f080036","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":32,"alto":24}},{"tipo":"TextView","id":"id/action_bar_subtitle","idRef":"0x7f080035","ancho":"wrap_content","alto":"wrap_content","margen":{"izq":0,"arr":-3,"der":0,"aba":0},"rect":{"x":0,"y":21,"ancho":32,"alto":24}}]}}
[apkc] finished in 203.642511ms
```
`rc=0`. El error literal es `el classes.dex no se deja leer: string #808 claims 18 utf-16 units and
holds 20`; `pasos:0`, `layout:res/layout/abc_action_bar_title_item.xml`. Se reejecutó aquí, así que es
medición propia; la causa byte a byte de la cadena #808 sigue siendo **`incierto`** hasta `§355`.

**Frontera.** Los tres devuelven `pasos:0` y pintan el mismo `LinearLayout` de tres nodos con
`id/action_bar_title` e `id/action_bar_subtitle` (estilo
`style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem`): F-Droid y NewPipe con `layout:res/A1.xml` y
Termux con `res/layout/abc_action_bar_title_item.xml`. **No se afirma que ninguna de las tres apps
funcione**: esa pantalla es falsa, no una aproximación de la app. La actividad de F-Droid
(`org.fdroid.IconCalculatorActivity`) se registra solo como lo que el comando/reporte dice hoy; su
identidad como lanzadora real queda como hipótesis para otra fase. El arreglo del lector se mide en
`§355`.

---

## 355. Fase A: el lector ya traga los dex reales, pero aparece un bloqueo nuevo

Medido el 2026-11-24 en el mismo worktree. Sin gate, CDP, chromium, red, `git push` ni publicación.
Los `preview*.go` se materializaron desde el submódulo con `sh tools/materializar-emulador.sh`
(«materializados 34 ficheros preview*.go»), porque en este checkout viven fuera de `apkc/`.
El cambio está solo en `apkc/dexcode.go` (`anchoValor:53`, `methodsDeCadaClase:356`,
`completaValoresEstaticos:386`, `valoresEstaticos:402`, `leeValorEstatico:422`) y
`apkc/dexreader.go` (`readStringData:262`, `decodeMUTF8:523`, `utf16Len:588`); el escritor de dex no
se tocó.

**Tuplas byte a byte extraídas de los APK.** Guion de extracción:
`/home/leonisauro/.cache/apkc-web/apps-reales-fase-a/extraer.py` (fuera del repo).

```
$ python3 /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/extraer.py
classes.dex: L_COROUTINE/CoroutineDebuggingKt; class_def #1 static_values_off=9178694 class_data_off=8642632 static_fields=1 instance_fields=0 direct=3 virtual=0
  encoded_array: size=1; items:
    #0: tipo=0x17 size=2 bytes=37 0b 7e (offset=9178695)
  array bytes (4): 01 37 0b 7e
classes.dex: Landroidx/compose/foundation/layout/SpacerKt; class_def #990 static_values_off=10657427 class_data_off=10174785 static_fields=8 instance_fields=0 direct=30 virtual=1
  encoded_array: size=4; items:
    #0: tipo=0x1e size=1 bytes=1e (offset=10657428)
    #1: tipo=0x1e size=1 bytes=1e (offset=10657429)
    #2: tipo=0x1e size=1 bytes=1e (offset=10657430)
    #3: tipo=0x04 size=1 bytes=04 0f (offset=10657431)
  array bytes (6): 04 1e 1e 1e 04 0f
/home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk / classes.dex: string_ids[808] off=5945238 declara=18 bytes=46 uleb=12 body=c0 80 7f c2 ad d8 80 d8 9c db 9d dc 8f e0 a3 a2 e1 9a 80 e1 a0 8e e2 80 80 e2 80 a8 e2 81 9f e2 81 a6 e3 80 80 ed a0 80 ef bb bf ef bf b9
```

Dos precisiones medidas: el índice `0b 7e` del valor `0x17` es **little-endian** (32267 → la cadena
`_COROUTINE` del `string_ids`), no uleb128 — el uleb daría 16139, que es otra cadena; coincide con
`codificaBytes` (`apkc/dexanota.go:243`). Y la causa del 18/20 se aisló con un programa aislado que
reproduce `decodeMUTF8`/`utf16Len`: el sustituto suelto `ed a0 80` se emitía como UTF-8 y Go lo
recorría como tres `RuneError`:

```
$ go run /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/check.go
bytes=46 runas=20 utf16Len=20
colita=ed a0 80 ef bb bf ef bf b9
```

Tras convertir los sustitutos sueltos en un solo `U+FFFD`, `utf16Len` da 18. Los arrays y las
anotaciones se recorren recursivamente (`leeValorEstatico:422`), y la cola de valores ausente se
rellena con `VALUE_NULL` (`completaValoresEstaticos:386`).

**Tests nuevos y suite.**

```
$ PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH tcr -j 1 -q -- sh -c 'cd apkc && go test -run "ValoresEstaticosReales|Mutf8Real" -v .'
=== RUN   TestMutf8RealTermux
--- PASS: TestMutf8RealTermux (0.00s)
=== RUN   TestValoresEstaticosRealesFDroid
--- PASS: TestValoresEstaticosRealesFDroid (0.00s)
=== RUN   TestValoresEstaticosRealesNewPipe
--- PASS: TestValoresEstaticosRealesNewPipe (0.00s)
=== RUN   TestValoresEstaticosRealesArrayAnidado
--- PASS: TestValoresEstaticosRealesArrayAnidado (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.011s
```
```
$ PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH tcr -j 1 -q -- sh -c 'cd apkc && go test -count=1 ./...'
ok  	github.com/ajinasokan/apkc	197.931s
```

**Escritor byte a byte.** El binario anterior (`/home/leonisauro/.cache/apkc-web/emulador-fiel/apkc`, 20:06) y el
reconstruido (sha256 `9c57bc54127894df03aa8cef77adfea90ea17dbbe83498f230628f2361c26f47`) dexaron el
mismo `Paridad.java` (constante String e int):

```
$ cmp /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/paridad/antes.dex /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/paridad/despues.dex && echo 'cmp: sin diferencias'
cmp: sin diferencias
$ sha256sum /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/paridad/antes.dex /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/paridad/despues.dex
4e6aa2d8329f85dfd04cddd8bb13d3376d703197003dcfae8a17121b325e1eb5  /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/paridad/antes.dex
4e6aa2d8329f85dfd04cddd8bb13d3376d703197003dcfae8a17121b325e1eb5  /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/paridad/despues.dex
```

**`apkc preview` con el binario reconstruido.** Los tres errores de `§354` desaparecieron y los
tres avanzaron, pero **aparece un error nuevo** en los tres:

```
$ /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/apkc preview /home/leonisauro/Downloads/F-Droid.apk
{"paquete":"org.fdroid.fdroid","version":"2.0.3","actividad":"org.fdroid.IconCalculatorActivity","layout":"res/A1.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":3,"ejecucion":{"actividad":"androidx.activity.ComponentActivity","metodo":"onCreate","pasos":292,"avisos":["java.lang.Object.getClass()Ljava/lang/Class; no está en el dex ni en los stubs: se devuelve el cero de Ljava/lang/Class;","java.lang.Integer.numberOfLeadingZeros(I)I no está en el dex ni en los stubs: se devuelve el cero de I","java.lang.Math.max(II)I no está en el dex ni en los stubs: se devuelve el cero de I","kotlinx.coroutines.flow.StateFlowKt.MutableStateFlow(Ljava/lang/Object;)Lkotlinx/coroutines/flow/MutableStateFlow; no está en el dex ni en los stubs: se devuelve el cero de Lkotlinx/coroutines/flow/MutableStateFlow;","androidx.savedstate.SavedStateRegistryController$Companion.create(Landroidx/savedstate/SavedStateRegistryOwner;)Landroidx/savedstate/SavedStateRegistryController; no está en el dex ni en los stubs: se devuelve el cero de Landroidx/savedstate/SavedStateRegistryController;","android.os.SystemClock.uptimeMillis()J no está en el dex ni en los stubs: se devuelve el cero de J","kotlin.LazyKt__LazyJVMKt.lazy(Lkotlin/jvm/functions/Function0;)Lkotlin/Lazy; no está en el dex ni en los stubs: se devuelve el cero de Lkotlin/Lazy;"],"error":"en androidx.lifecycle.LifecycleRegistry.addObserver: en androidx.lifecycle.LifecycleRegistry.enforceMainThreadIfNeeded: en androidx.lifecycle.LifecycleRegistry_androidKt.isMainThread: en androidx.arch.core.executor.ArchTaskExecutor.getInstance: androidx.arch.core.executor.ArchTaskExecutor.getInstance usa el opcode 0x1d, que el intérprete MVP no decodifica"},"raiz":{"tipo":"LinearLayout","estilo":"style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"TextView","id":"id/action_bar_title","idRef":"0x7f08003b","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":32,"alto":24}},{"tipo":"TextView","id":"id/action_bar_subtitle","idRef":"0x7f08003a","ancho":"wrap_content","alto":"wrap_content","margen":{"izq":0,"arr":-3,"der":0,"aba":0},"rect":{"x":0,"y":21,"ancho":32,"alto":24}}]}}
[apkc] finished in 359.281587ms
```
```
$ /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/apkc preview /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk
{"paquete":"org.schabi.newpipe","version":"0.29.3","actividad":"org.schabi.newpipe.MainActivity","layout":"res/A1.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":3,"ejecucion":{"actividad":"org.schabi.newpipe.MainActivity","metodo":"onCreate","pasos":286,"avisos":["java.lang.Integer.numberOfLeadingZeros(I)I no está en el dex ni en los stubs: se devuelve el cero de I","java.lang.Math.max(II)I no está en el dex ni en los stubs: se devuelve el cero de I","java.lang.Object.getClass()Ljava/lang/Class; no está en el dex ni en los stubs: se devuelve el cero de Ljava/lang/Class;","android.os.SystemClock.COMMON_PSSH_UUID()J no está en el dex ni en los stubs: se devuelve el cero de J"],"error":"en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.LifecycleRegistry.addObserver: en androidx.lifecycle.LifecycleRegistry.enforceMainThreadIfNeeded: en androidx.arch.core.executor.ArchTaskExecutor.getInstance: androidx.arch.core.executor.ArchTaskExecutor.getInstance usa el opcode 0x1d, que el intérprete MVP no decodifica"},"raiz":{"tipo":"LinearLayout","estilo":"style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"TextView","id":"id/action_bar_title","idRef":"0x7f0a003d","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":32,"alto":24}},{"tipo":"TextView","id":"id/action_bar_subtitle","idRef":"0x7f0a003c","ancho":"wrap_content","alto":"wrap_content","margen":{"izq":0,"arr":-3,"der":0,"aba":0},"rect":{"x":0,"y":21,"ancho":32,"alto":24}}]}}
[apkc] finished in 442.195635ms
```
```
$ /home/leonisauro/.cache/apkc-web/apps-reales-fase-a/apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
{"paquete":"com.termux","version":"0.118.3","actividad":"com.termux.app.TermuxActivity","layout":"res/layout/abc_action_bar_title_item.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":3,"ejecucion":{"actividad":"androidx.activity.ComponentActivity","metodo":"onCreate","pasos":207,"error":"en androidx.lifecycle.LifecycleRegistry.addObserver: en androidx.lifecycle.LifecycleRegistry.enforceMainThreadIfNeeded: en androidx.arch.core.executor.ArchTaskExecutor.getInstance: androidx.arch.core.executor.ArchTaskExecutor.getInstance usa el opcode 0x1d, que el intérprete MVP no decodifica"},"raiz":{"tipo":"LinearLayout","estilo":"style/RtlOverlay.Widget.AppCompat.ActionBar.TitleItem","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"TextView","id":"id/action_bar_title","idRef":"0x7f080036","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":32,"alto":24}},{"tipo":"TextView","id":"id/action_bar_subtitle","idRef":"0x7f080035","ancho":"wrap_content","alto":"wrap_content","margen":{"izq":0,"arr":-3,"der":0,"aba":0},"rect":{"x":0,"y":21,"ancho":32,"alto":24}}]}}
[apkc] finished in 385.563567ms
```
`rc=0` en los tres. `pasos`: 292, 286 y 207 (antes 0, 0 y 0). El error literal nuevo, compartido,
es `androidx.arch.core.executor.ArchTaskExecutor.getInstance usa el opcode 0x1d, que el intérprete
MVP no decodifica`.

**Frontera (no verde).** El criterio de Fase A del plan exigía «desaparecen los tres
`ejecucion.error`»; los tres siguen presentes, aunque son otros y mucho más adelante. Lo que queda
medido es que **las tres causas del lector (R1, R2 y R3) están arregladas**: ya no aparece el valor
`0x17`, ni `4 static values for 8 static fields`, ni `string #808 claims 18…`, y el intérprete
corre cientos de pasos. El bloqueo nuevo es del intérprete MVP (opcode `0x1d`), frontera de la fase
de stubs, no de este cambio. Los layouts siguen siendo los falsos de `§354`, así que **ninguna de las
tres apps se da por funcionando**. La causa byte a byte de la cadena #808, que en `§354` era
`incierto`, queda aquí medida. Sin gate, CDP, red, publicación ni `git push`.

---

## 358. Los monitores de `synchronized` dejan de bloquear al intérprete

Medido el 2026-11-24 en el worktree `opcode-1d-monitor` (`…/c6f089/opcode-1d-monitor`), sobre el emulador ya separado como único submódulo de `studio` (§357). Sin gate, CDP, chromium, red, `git push` ni publicación. Los `preview*.go` se materializaron desde el submódulo con `sh tools/materializar-emulador.sh <worktree> <submódulo>` («materializados 34 ficheros preview*.go de … en …/apkc»). El emulador subió a `4117a8f6d4cdbab896e82b6f89b29a2cc00d54f6` (`fix(preview): decodifica los monitores 0x1d y 0x1e`), `studio` actualizó su gitlink en el mismo commit (`4a17f84 chore(emulador): actualiza el gitlink al decodificador de monitores`) y el padre lo recogió: la guardia de la regla 14 pasa de `563e991` a `4117a8f`. El cambio de código está solo en `studio/apkc-emulador/previewinterprete.go:3072-3079` —un único `case 0x1d, 0x1e:` que consume la instrucción 11x con `pc++`— y en el test `previewinterprete_test.go:1553-1676`. El escritor no se tocó: `apkc/dexinstr.go:125-126` (`opMonitorEntra`/`opMonitorSale`) solo se cita en el comentario, y `cmp` entre el `dexinstr.go` del checkout principal y el del worktree da «sin diferencias».

**Oráculo: ART real, no el dexer ni la JVM.** El crítico pidió el valor de ART, no el de nuestro escritor. Oráculo preparado en `$HOME/.cache/apkc-web/monitor-art-oracle/`; AVD x86_64 Android 14 (API 34) con `boot_completed=1`, `release=14`, `fingerprint=google/sdk_gphone64_x86_64/emu64xa:14/UE1A.230829.050/12077443:userdebug/dev-keys`. Comandos literales:

```
$ /home/leonisauro/opt/jdk21/bin/javac --release 8 -g:lines,source -cp src -d classes src/MoniOracle.java
rc=0 (3 avisos por --release 8 obsoleto)
$ /home/leonisauro/opt/android-sdk/build-tools/35.0.1/d8 --release --min-api 21 --lib /home/leonisauro/opt/android-sdk/platforms/android-34/android.jar --output dex src/Moni.class classes/MoniOracle.class
rc=0; dex/classes.dex 1588 bytes
$ /home/leonisauro/opt/android-sdk/build-tools/35.0.1/dexdump -d dex/classes.dex > dexdump.txt
dexdump.txt:39  |0002ec: 1d01 |0000: monitor-enter v1
dexdump.txt:41  |0002f4: 1e01 |0004: monitor-exit v1
dexdump.txt:65  |00031c: 1d00 |0000: monitor-enter v0
dexdump.txt:67  |000324: 1e00 |0004: monitor-exit v0
$ adb -s emulator-5554 push dex/classes.dex /data/local/tmp/moni.dex
$ adb -s emulator-5554 shell 'export CLASSPATH=/data/local/tmp/moni.dex; /system/bin/app_process64 /data/local/tmp MoniOracle'
moni: sinc y sincInst terminaron
rc=0
```

La sonda de tres valores usa `MoniValor` (fuente en `$HOME/.cache/apkc-web/moni-valor-fixture/MoniValor.java`, `.class` de 757 bytes con sha256 `2d2b95da8d59944bdd635af4231f017dfbbb06097561661f5e8756ad68c269c5`, viaja como base64 de 1012 bytes en el test, regla 2). `valor-dexdump.txt:39-44,64-72,91-103` muestra los pares `1d01`/`1e01` en `sincDevuelve`, `sincLanza` y `sincReenvuelve` —este último con cuatro `catch` y `monitor-exit` también en la ruta de excepción—; la ejecución en ART del mismo dex (`moni-valor.out`) da:

```
sincDevuelve=42
sincLanza=java.lang.NumberFormatException: For input string: "x"
sincReenvuelve=42
rc=0
```

Referencia JVM (alternativa, no el oráculo exigido): `/home/leonisauro/opt/jdk21/bin/java -cp src:classes MoniOracle` → `moni: sinc y sincInst terminaron`, `jvm_rc=0`. Por tanto el valor que debe reproducir el intérprete tras decodificar `1d01`/`1e01` es literalmente ese: tres resultados, sin `IllegalMonitorStateException` y sin desbalance de monitores.

**Focal re-ejecutado por quien documenta.**

```
$ env PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH GOTMPDIR=$HOME/.cache/apkc-web/gotmp tcr -j 1 -r 50 -m 1200M -s 2G -q -- sh -c 'cd apkc && go test -count=1 -run TestElInterpreteDecodificaLosMonitoresDeSynchronized -v .'
=== RUN   TestElInterpreteDecodificaLosMonitoresDeSynchronized
    previewinterprete_test.go:1598: guardia: sincDevuelve trae 1 monitor-enter y 2 monitor-exit
    previewinterprete_test.go:1638: sincDevuelve=42
    previewinterprete_test.go:1657: sincLanza="MoniValor.sincLanza: java.lang.NumberFormatException: For input string: \"x\""
    previewinterprete_test.go:1670: sincReenvuelve=42
    previewinterprete_test.go:1675: avisos=0
--- PASS: TestElInterpreteDecodificaLosMonitoresDeSynchronized (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.004s
```

La guardia de `ci.insns` (`previewinterprete_test.go:1586-1598`) es imprescindible: sin los monitores en el dex, un test en verde no probaría nada. El test exige además que `sincLanza` no mencione «balance» y que los tres métodos no dejen avisos (`previewinterprete_test.go:1627-1675`): el delta de `avisos` de la llamada es 0. La frontera queda escrita en el propio `case`: intérprete de un solo hilo, sin `wait`/`notify`, sin contención y sin verificación de que `monitor-enter` y `monitor-exit` formen pareja; se consumen avanzando `pc`, sin estado adicional y sin aviso.

**Paridad del escritor y suite.** Salidas de suite, wasm y pestaña entregadas por el builder; quien documenta solo re-ejecutó el focal de arriba.

```
$ cmp /home/leonisauro/Develop/Go/apkc-web/apkc/dexinstr.go /home/leonisauro/.local/share/opencode/worktree/c6f089/opcode-1d-monitor/apkc/dexinstr.go && echo 'dexinstr.go: cmp sin diferencias'
dexinstr.go: cmp sin diferencias
$ cmp studio/apkc-emulador/previewinterprete.go apkc/previewinterprete.go && echo 'materializado previewinterprete.go: cmp sin diferencias'
materializado previewinterprete.go: cmp sin diferencias
```

Los 34 ficheros `preview*.go` siguen siendo 34 (8 de producción y 26 tests) y `materializar-emulador.sh` comprueba cada copia con `cmp` (`tools/materializar-emulador.sh:32-44`). El único commit del submódulo toca dos ficheros (`previewinterprete.go` +9, `previewinterprete_test.go` +126), ninguno del escritor de dex.

```
$ PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH tcr -j 1 -q -- sh -c 'cd apkc && go test -count=1 ./...'
ok  	github.com/ajinasokan/apkc	208.495s
$ GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o $HOME/.cache/apkc-web/358/apkc-wasm.wasm .
rc=0; 16 499 923 bytes
$ cd studio && node prueba.js
prueba: todo en orden
```

**Tres APK reales, antes (§355) y después.** Binario: `$HOME/.cache/apkc-web/358/apkc`; JSON nuevo: `$HOME/.cache/apkc-web/358/{fdroid,newpipe,termux}.json`; línea base: `$HOME/.cache/apkc-web/monitor-baseline/{fdroid,newpipe,termux}.json`. Los tres dieron `rc=0` y ninguno de los tres JSON nuevos contiene la cadena `0x1d`.

| APK | pasos antes → después | avisos antes → después | error antes (§355) | error después |
| --- | --- | --- | --- | --- |
| F-Droid | 292 → 414 | 7 → 13 | `…ArchTaskExecutor.getInstance usa el opcode 0x1d, que el intérprete MVP no decodifica` | `…androidx.lifecycle.FastSafeIterableMap.putIfAbsent: en androidx.collection.MutableScatterMap.get: aget sobre un registro que no es arreglo` |
| NewPipe | 286 → 355 | 4 → 8 | igual `0x1d` | `…androidx.lifecycle.Lifecycling.getObserverConstructorType: se llamó a java.lang.Class.getCanonicalName sobre un objeto nulo` |
| Termux arm | 207 → 282 | 0 → 4 | igual `0x1d` | `…androidx.lifecycle.Lifecycling.getObserverConstructorType: se llamó a java.util.Map.get sobre un objeto nulo` |

```
$ /home/leonisauro/.cache/apkc-web/358/apkc preview /home/leonisauro/Downloads/F-Droid.apk
$ /home/leonisauro/.cache/apkc-web/358/apkc preview /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk
$ /home/leonisauro/.cache/apkc-web/358/apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
```

**Corrida pareada de avisos.** Los JSON anteriores no eran comparables entre sí (binarios y recetas distintos), así que se repitieron **las dos ramas con la misma receta**. Comandos literales de la corrida (directorio `/home/leonisauro/Develop/Go/apkc-web`; texto según el informe del subagente que la ejecutó; SHA y JSON verificados también aquí):

```
$ sh tools/materializar-emulador.sh
$ PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH GOTMPDIR=$HOME/.cache/apkc-web/avisos-pareados/gotmp tcr -j 1 -r 50 -m 1200M -s 2G -q -- sh -c 'cd apkc && go build -o $HOME/.cache/apkc-web/avisos-pareados/apkc-post .'
$ sha256sum $HOME/.cache/apkc-web/monitor-baseline/apkc $HOME/.cache/apkc-web/avisos-pareados/apkc-post
b8431611439d9b9377bb5f91095d66c7365a782c26ca2dcf9e840e3260126f77  /home/leonisauro/.cache/apkc-web/monitor-baseline/apkc
a9110bfcf49eac2dc9ff817aa44161a15a84bb76c7590d62ad6b1b58f6b1835f  /home/leonisauro/.cache/apkc-web/avisos-pareados/apkc-post
$ go version -m $HOME/.cache/apkc-web/monitor-baseline/apkc
go1.24.0; vcs.revision=b0bdb331ad7ec4414b8f61ce3fccb4c0b334a01c; vcs.modified=false
$ go version -m $HOME/.cache/apkc-web/avisos-pareados/apkc-post
go1.24.0; vcs.revision=010961195ea1ec9ea59a78ffe84c10ec25d5ab25; vcs.modified=false
$ /home/leonisauro/.cache/apkc-web/monitor-baseline/apkc preview /home/leonisauro/Downloads/F-Droid.apk > $HOME/.cache/apkc-web/avisos-pareados/pre_fdroid.json 2>/dev/null
$ /home/leonisauro/.cache/apkc-web/monitor-baseline/apkc preview /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk > $HOME/.cache/apkc-web/avisos-pareados/pre_newpipe.json 2>/dev/null
$ /home/leonisauro/.cache/apkc-web/monitor-baseline/apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk > $HOME/.cache/apkc-web/avisos-pareados/pre_termux.json 2>/dev/null
$ /home/leonisauro/.cache/apkc-web/avisos-pareados/apkc-post preview /home/leonisauro/Downloads/F-Droid.apk > $HOME/.cache/apkc-web/avisos-pareados/post_fdroid.json 2>/dev/null
$ /home/leonisauro/.cache/apkc-web/avisos-pareados/apkc-post preview /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk > $HOME/.cache/apkc-web/avisos-pareados/post_newpipe.json 2>/dev/null
$ /home/leonisauro/.cache/apkc-web/avisos-pareados/apkc-post preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk > $HOME/.cache/apkc-web/avisos-pareados/post_termux.json 2>/dev/null
$ python3 $HOME/.cache/apkc-web/avisos-pareados/comparar.py
$ python3 $HOME/.cache/apkc-web/avisos-pareados/cotejo.py
fdroid: pre_igual=True post_igual=True
newpipe: pre_igual=True post_igual=True
termux: pre_igual=True post_igual=True
```

Los seis `preview` salieron con `rc=0`; el cotejo reproduce exactamente los dos conjuntos anteriores. Rutas de los seis JSON: `$HOME/.cache/apkc-web/avisos-pareados/{pre,post}_{fdroid,newpipe,termux}.json` (junto a `comparar.py`, `cotejo.py`, `gotmp/` y `apkc-post`). Resultado de `comparar.py`, verificado aquí mismo con la misma lectura de los seis JSON:

| APK | pasos pre → post | avisos pre → post | desaparecidos | nuevos |
| --- | --- | --- | --- | --- |
| F-Droid | 292 → 414 | 7 → 13 | 0 | 6 |
| NewPipe | 286 → 355 | 4 → 8 | 0 | 4 |
| Termux arm | 207 → 282 | 0 → 4 | 0 | 4 |

`comunes` = los siete/cuatro/cero avisos del `pre`, idénticos en el `post`. Los nuevos, todos de la familia «no está en el dex ni en los stubs: se devuelve el cero de …»: en F-Droid (6) `Executors.newFixedThreadPool`, `Looper.getThread`, `Thread.currentThread`, `Lifecycling.getObserverConstructorType`, `ClassesInfoCache.getInfo` y `Object.hashCode`; en NewPipe (4) los tres primeros más `HashMap.get`; en Termux (4) los tres primeros más `Object.getClass`. `desaparecidos`: `[]`, `[]`, `[]`.

> Con la misma receta sobre ambos binarios, los avisos del `pre` son subconjunto literal de los del `post` (desaparecidos: 0) y aparecen 6/4/4 avisos nuevos por código que antes era inalcanzable; el `case 0x1d/0x1e` no emite aviso (`previewinterprete.go:3078-3084`, y el test exige delta de `avisos`=0). Por tanto **no** se afirma «sin avisos nuevos» en `§358`: la meta de 0 avisos de esa familia pertenece a Fase D (`§361`, `docs/plan-compatibilidad-apps-pausado.md:37-40,385-387`) y aquí solo se mide que desaparece el opcode.

**Frontera (no verde).** El error compartido del intérprete desaparece y los tres avanzan cientos de pasos más, pero los tres siguen teniendo `ejecucion.error`: ahora son errores del modelo de stubs, muy por delante del punto donde antes caían. Los layouts siguen siendo los falsos de `§354`, ahora **medidos de nuevo** tras `§358` con el binario `vcs.revision=b038662c` (`~/.cache/apkc-web/layouts-post-358/{fdroid,newpipe,termux}.json`): `res/A1.xml` en F-Droid y NewPipe y `res/layout/abc_action_bar_title_item.xml` en Termux, 3 nodos, `LinearLayout` con `id/action_bar_title` e `id/action_bar_subtitle` y `aproximado=true`; los pasos son 414/355/282. La actividad ejecutada sigue siendo `androidx.activity.ComponentActivity` en F-Droid y Termux y `org.schabi.newpipe.MainActivity` en NewPipe, sin que eso identifique la lanzadora real: **ninguna de las tres apps se da por funcionando ni se afirma que muestre su pantalla real**. Quedan pendientes la Fase D (`§361`, familia de avisos a 0), la Fase C (`§359`, lanzadora y layout correctos) y la Fase B (`§360`, multidex), además de E-G, en el orden de `docs/plan-compatibilidad-apps-pausado.md:13`. El cambio cierra solo el opcode `0x1d`/`0x1e`; no toca `dexinstr.go`, no añade estado de monitor y no cambia el despacho de excepciones.

Los criterios ejecutados —test focal y corrida pareada— son la única evidencia de este `§`: aparecen 6/4/4 avisos nuevos de «no está en el dex ni en los stubs» y los tres `ejecucion.error` cambian de causa, sin que se afirme sustitución formal de condición alguna.

---

## 366. Cuarta APK real: línea base anterior a Fase D

Medido el 2026-11-24 (los ficheros de la medición llevan hora 23:48-23:49; el binario declara
`vcs.time=2026-11-25T05:48:32Z`). Sin gate, CDP, chromium, red, `git push` ni publicación.
C, B y `§358` no afectan a este APK: trae un solo `classes.dex`, la actividad lanzadora y el layout
ya son correctos y no hay monitores. La medición es válida como línea base **anterior a Fase D** y
no debe fusionarse con `§361`. Si al medir D con el binario post el SHA del JSON cambia, se re-mide
antes de atribuir nada a D.

**Identidad del APK, del binario y de la salida.**

```sh
sha256sum /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/apkc /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/preview.json
```
```
fc6c9134fd714ef059305eef43ca093880bf0937eb8f3ed750c06899a7162fe0  /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
f241c54b0c372196ad082b05b5909a16e1c74b58824d7f125707e73f7e68c286  /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/apkc
f207c0435930c3d4f5256ac68add74915651f10ec3bdd056d007df7f445741a7  /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/preview.json
```
El APK tiene 38.042 B. Árbol: revisión fuente embebida en el binario (`go version -m`:
`vcs.revision=…`, `vcs.modified=false`); no es el árbol medido en `§365` ni el `HEAD` actual.

```sh
go version -m /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/apkc
```
```
/home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/apkc: go1.24.0
	path	github.com/ajinasokan/apkc
	mod	github.com/ajinasokan/apkc	v0.0.0-20261125054832-f59717b2c6c8
	dep	github.com/fsnotify/fsnotify	v1.10.1	h1:b0/UzAf9yR5rhf3RPm9gf3ehBPpf0oZKIjtpKrx59Ho=
	dep	github.com/hashicorp/go-version	v1.6.0	h1:feTTfFNnjP967rlCxM/I9g701jU+RN74YKx2mOkIeek=
	dep	golang.org/x/sys	v0.13.0	h1:Af8nKPmuFypiUBjVoU9V20FiaFXOcuZI21p0ycVYYGE=
	build	-buildmode=exe
	build	-compiler=gc
	build	CGO_ENABLED=1
	build	CGO_CFLAGS=
	build	CGO_CPPFLAGS=
	build	CGO_CXXFLAGS=
	build	CGO_LDFLAGS=
	build	GOARCH=amd64
	build	GOOS=linux
	build	GOAMD64=v1
	build	vcs=git
	build	vcs.revision=f59717b2c6c82f154332b7fd1958c8dd8faa8844
	build	vcs.time=2026-11-25T05:48:32Z
	build	vcs.modified=false
```
Nota histórica: El binario `b8431611…` se menciona como hipótesis: no hay comando ni salida que
reproduzca su JSON, por lo que **no se afirma** que fuera byte-idéntico (regla 1); queda fuera del
corpus.

**Manifiesto, ficheros y dex.**

```sh
aapt2 dump badging /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
```
```
package: name='com.martinmimigames.simplefileexplorer' versionCode='20' versionName='4.18' platformBuildVersionName='13' platformBuildVersionCode='33' compileSdkVersion='33' compileSdkVersionCodename='13'
minSdkVersion:'1'
targetSdkVersion:'29'
uses-permission: name='android.permission.WRITE_EXTERNAL_STORAGE'
uses-permission: name='android.permission.READ_EXTERNAL_STORAGE'
uses-permission: name='android.permission.MANAGE_EXTERNAL_STORAGE'
uses-permission: name='android.permission.REQUEST_INSTALL_PACKAGES'
application-label:'Little File Explorer'
application-icon-160:'res/yE.png'
application-icon-65535:'res/yE.png'
application: label='Little File Explorer' icon='res/yE.png'
launchable-activity: name='com.martinmimigames.simplefileexplorer.MainActivity'  label='' icon=''
feature-group: label=''
  uses-feature: name='android.hardware.faketouch'
  uses-implied-feature: name='android.hardware.faketouch' reason='default feature for all apps'
main
supports-screens: 'small' 'normal' 'large' 'xlarge'
supports-any-density: 'true'
locales: '--_--'
densities: '160' '65535'
```
```sh
apkanalyzer manifest print /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
```
```
<?xml version="1.0" encoding="utf-8"?>
<manifest
    xmlns:a="http://schemas.android.com/apk/res/android"
    a:versionCode="20"
    a:versionName="4.18"
    a:compileSdkVersion="33"
    a:compileSdkVersionCodename="13"
    package="com.martinmimigames.simplefileexplorer"
    platformBuildVersionCode="33"
    platformBuildVersionName="13">

    <uses-sdk
        a:minSdkVersion="1"
        a:targetSdkVersion="29" />

    <uses-permission
        a:name="android.permission.WRITE_EXTERNAL_STORAGE" />

    <uses-permission
        a:name="android.permission.READ_EXTERNAL_STORAGE" />

    <uses-permission
        a:name="android.permission.MANAGE_EXTERNAL_STORAGE" />

    <uses-permission
        a:name="android.permission.REQUEST_INSTALL_PACKAGES" />

    <application
        a:theme="@ref/0x7f050000"
        a:label="Little File Explorer"
        a:icon="@ref/0x7f020003"
        a:resizeableActivity="true"
        a:requestLegacyExternalStorage="true">

        <provider
            a:name="com.martinmimigames.simplefileexplorer.FileProvider"
            a:exported="false"
            a:authorities="com.martinmimigames.simplefileexplorer.FileProvider"
            a:grantUriPermissions="true" />

        <activity
            a:name="com.martinmimigames.simplefileexplorer.MainActivity"
            a:exported="true">

            <intent-filter>

                <action
                    a:name="android.intent.action.MAIN" />

                <category
                    a:name="android.intent.category.LAUNCHER" />
            </intent-filter>

            <intent-filter>

                <action
                    a:name="android.intent.action.GET_CONTENT" />

                <action
                    a:name="android.intent.action.OPEN_DOCUMENT" />

                <data
                    a:mimeType="*/*" />

                <category
                    a:name="android.intent.category.DEFAULT" />

                <category
                    a:name="android.intent.category.OPENABLE" />
            </intent-filter>
        </activity>
    </application>
</manifest>
```
```sh
unzip -Z1 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
```
```
META-INF/com/android/build/gradle/app-metadata.properties
classes.dex
AndroidManifest.xml
res/Fb.png
res/ML.xml
res/PK.xml
res/R5.xml
res/UI.xml
res/Wl.xml
res/aS.png
res/a_.png
res/kc.png
res/mA.png
res/vD.png
res/yE.png
resources.arsc
META-INF/B0DA40AD.SF
META-INF/B0DA40AD.RSA
META-INF/MANIFEST.MF
```
Un solo `classes.dex`, ningún `.so`, y `res/R5.xml` entre los layouts. El dex se extrajo a
`/home/leonisauro/Develop/.tmp/apk366-eT7e4C/classes.dex` para el oráculo:

```sh
dexdump -f /home/leonisauro/Develop/.tmp/apk366-eT7e4C/classes.dex | head -25
```
```
Processing '/home/leonisauro/Develop/.tmp/apk366-eT7e4C/classes.dex'...
Opened '/home/leonisauro/Develop/.tmp/apk366-eT7e4C/classes.dex', DEX version '035'
DEX file header:
magic               : 'dex\n035\0'
checksum            : 26b0889d
signature           : 9c2b...27d6
file_size           : 35584
header_size         : 112
link_size           : 0
link_off            : 0 (0x000000)
string_ids_size     : 505
string_ids_off      : 112 (0x000070)
type_ids_size       : 148
type_ids_off        : 2132 (0x000854)
proto_ids_size      : 157
proto_ids_off       : 2724 (0x000aa4)
field_ids_size      : 100
field_ids_off       : 4608 (0x001200)
method_ids_size     : 334
method_ids_off      : 5408 (0x001520)
class_defs_size     : 35
class_defs_off      : 8080 (0x001f90)
data_size           : 26384
data_off            : 9200 (0x0023f0)
```
```
$ dexdump -d /home/leonisauro/Develop/.tmp/apk366-eT7e4C/classes.dex | grep -iE '0x1d|opcode|invoke-polymorphic|invoke-custom'
```
Sin salida: ni el opcode `0x1d` ni `invoke-custom`/`invoke-polymorphic` aparecen en el volcado.

**`apkc preview`.**

```sh
/home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/apkc preview /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/preview.json
```
Salida byte a byte (una sola línea, 7484 B, sha256 `f207c0435930c3d4f5256ac68add74915651f10ec3bdd056d007df7f445741a7`):

```
{"paquete":"com.martinmimigames.simplefileexplorer","version":"4.18","actividad":"com.martinmimigames.simplefileexplorer.MainActivity","layout":"res/R5.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},"aproximado":true,"nodos":30,"ejecucion":{"actividad":"com.martinmimigames.simplefileexplorer.MainActivity","metodo":"onCreate","pasos":162,"capturas":[{"origen":"TextView.setText","texto":"Grant"},{"origen":"TextView.setText","texto":"Deny"}],"avisos":["java.util.concurrent.Executors.newCachedThreadPool()Ljava/util/concurrent/ExecutorService; no está en el dex ni en los stubs: se devuelve el cero de Ljava/util/concurrent/ExecutorService;","java.lang.Object.getClass()Ljava/lang/Class; no está en el dex ni en los stubs: se devuelve el cero de Ljava/lang/Class;","android.content.Context.getPackageName()Ljava/lang/String; no está en el dex ni en los stubs: se devuelve el cero de Ljava/lang/String;","android.content.Context.setTheme(I)V no está en el dex ni en los stubs: se devuelve el cero de V","android.app.Dialog.setCancelable(Z)V no está en el dex ni en los stubs: se devuelve el cero de V","android.app.Dialog.setTitle(Ljava/lang/CharSequence;)V no está en el dex ni en los stubs: se devuelve el cero de V","android.view.View.setLayoutParams(Landroid/view/ViewGroup$LayoutParams;)V no está en el dex ni en los stubs: se devuelve el cero de V","android.view.View.setId(I)V no está en el dex ni en los stubs: se devuelve el cero de V","android.view.ViewGroup.addView(Landroid/view/View;)V no está en el dex ni en los stubs: se devuelve el cero de V","android.widget.RelativeLayout$LayoutParams.addRule(II)V no está en el dex ni en los stubs: se devuelve el cero de V","android.app.Dialog.setContentView(Landroid/view/View;)V no está en el dex ni en los stubs: se devuelve el cero de V","android.content.Intent.getAction()Ljava/lang/String; no está en el dex ni en los stubs: se devuelve el cero de Ljava/lang/String;"],"error":"se llamó a java.lang.Object.getClass sobre un objeto nulo","ciclo":["com.martinmimigames.simplefileexplorer.MainActivity.onCreate"]},"raiz":{"tipo":"RelativeLayout","ancho":"match_parent","alto":"match_parent","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"RelativeLayout","id":"id/header","idRef":"0x7f030003","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":68},"hijos":[{"tipo":"Button","id":"id/back_button","idRef":"0x7f030000","texto":" \u003c- ","ancho":"wrap_content","alto":"wrap_content","padding":{"izq":10,"arr":10,"der":10,"aba":10},"tamTexto":"30sp","rect":{"x":0,"y":0,"ancho":80,"alto":68}},{"tipo":"Button","id":"id/menu_button","idRef":"0x7f030006","texto":" ||| ","ancho":"wrap_content","alto":"wrap_content","padding":{"izq":10,"arr":10,"der":10,"aba":10},"tamTexto":"30sp","rect":{"x":0,"y":0,"ancho":95,"alto":68}},{"tipo":"TextView","id":"id/title","idRef":"0x7f030021","ancho":"match_parent","alto":"wrap_content","padding":{"izq":5,"arr":0,"der":5,"aba":0},"tamTexto":"20sp","rect":{"x":0,"y":0,"ancho":360,"alto":30}},{"tipo":"EditText","id":"id/filter","idRef":"0x7f030002","pista":"filter text","estilo":"style/editTextStyle","ancho":"match_parent","alto":"wrap_content","padding":{"izq":0,"arr":0,"der":0,"aba":0},"rect":{"x":0,"y":0,"ancho":360,"alto":24}}]},{"tipo":"ScrollView","ancho":"match_parent","alto":"match_parent","rect":{"x":0,"y":0,"ancho":360,"alto":640},"hijos":[{"tipo":"LinearLayout","id":"id/list","idRef":"0x7f030005","ancho":"match_parent","alto":"wrap_content","padding":{"izq":6,"arr":6,"der":6,"aba":6},"rect":{"x":0,"y":0,"ancho":360,"alto":12}}]},{"tipo":"LinearLayout","id":"id/drive_list","idRef":"0x7f030001","ancho":"match_parent","alto":"wrap_content","padding":{"izq":6,"arr":6,"der":6,"aba":6},"rect":{"x":0,"y":0,"ancho":360,"alto":12}},{"tipo":"LinearLayout","id":"id/menu_list","idRef":"0x7f030008","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":240},"hijos":[{"tipo":"Button","id":"id/menu_toggle_theme","idRef":"0x7f03000c","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":48}},{"tipo":"Button","id":"id/menu_toggle_hidden","idRef":"0x7f03000b","texto":"Toggle hidden","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":48,"ancho":360,"alto":48}},{"tipo":"Button","id":"id/menu_sorter","idRef":"0x7f03000a","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":96,"ancho":360,"alto":48}},{"tipo":"Button","id":"id/menu_create_new_directory","idRef":"0x7f030007","texto":"New folder","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":144,"ancho":360,"alto":48}},{"tipo":"Button","id":"id/menu_quick_switch","idRef":"0x7f030009","texto":"Switch drive menu","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":192,"ancho":360,"alto":48}}]},{"tipo":"LinearLayout","id":"id/quick_selection","idRef":"0x7f030014","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":144},"hijos":[{"tipo":"Button","id":"id/select_all","idRef":"0x7f030017","texto":"Select all","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":360,"alto":48}},{"tipo":"Button","id":"id/invert_select_all","idRef":"0x7f030004","texto":"Invert select all","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":48,"ancho":360,"alto":48}},{"tipo":"Button","id":"id/share_selected","idRef":"0x7f03001d","texto":"Share selected","ancho":"match_parent","alto":"wrap_content","rect":{"x":0,"y":96,"ancho":360,"alto":48}}]},{"tipo":"LinearLayout","id":"id/single_select_operation","idRef":"0x7f03001f","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":152,"alto":48},"hijos":[{"tipo":"Button","id":"id/single_select_rename","idRef":"0x7f030020","texto":"Rename","ancho":"wrap_content","alto":"wrap_content","rect":{"x":0,"y":0,"ancho":64,"alto":48}},{"tipo":"Button","id":"id/single_select_copy_path","idRef":"0x7f03001e","texto":"Copy Path","ancho":"wrap_content","alto":"wrap_content","rect":{"x":64,"y":0,"ancho":88,"alto":48}}]},{"tipo":"LinearLayout","id":"id/select_operation","idRef":"0x7f03001c","ancho":"wrap_content","alto":"wrap_content","padding":{"izq":6,"arr":6,"der":6,"aba":6},"rect":{"x":0,"y":0,"ancho":228,"alto":60},"hijos":[{"tipo":"Button","id":"id/select_cut","idRef":"0x7f03001a","texto":"Cut","ancho":"wrap_content","alto":"wrap_content","rect":{"x":6,"y":6,"ancho":40,"alto":48}},{"tipo":"Button","id":"id/select_copy","idRef":"0x7f030019","texto":"Copy","ancho":"wrap_content","alto":"wrap_content","rect":{"x":46,"y":6,"ancho":48,"alto":48}},{"tipo":"Button","id":"id/select_delete","idRef":"0x7f03001b","texto":"Delete","ancho":"wrap_content","alto":"wrap_content","rect":{"x":94,"y":6,"ancho":64,"alto":48}},{"tipo":"Button","id":"id/select_cancel","idRef":"0x7f030018","texto":"Cancel","ancho":"wrap_content","alto":"wrap_content","rect":{"x":158,"y":6,"ancho":64,"alto":48}}]},{"tipo":"LinearLayout","id":"id/paste_operation","idRef":"0x7f030012","ancho":"wrap_content","alto":"wrap_content","padding":{"izq":6,"arr":6,"der":6,"aba":6},"rect":{"x":0,"y":0,"ancho":132,"alto":60},"hijos":[{"tipo":"Button","id":"id/paste_paste","idRef":"0x7f030013","texto":"Paste","ancho":"wrap_content","alto":"wrap_content","rect":{"x":6,"y":6,"ancho":56,"alto":48}},{"tipo":"Button","id":"id/paste_cancel","idRef":"0x7f030011","texto":"Cancel","ancho":"wrap_content","alto":"wrap_content","rect":{"x":62,"y":6,"ancho":64,"alto":48}}]}]}}
```

`pasos:162`, `avisos` = 12, `error` = «se llamó a java.lang.Object.getClass sobre un objeto nulo»,
`layout` = `res/R5.xml`, `nodos` = 30. Frontera: pre-corrección: 12 avisos y `ejecucion.error`;
`ejecucion.capturas` (`Grant`/`Deny`) son textos capturados de `TextView.setText`, **no capturas
visuales**; no se afirma que la app funciona, no se fusiona con `§361`.

`§361` exige **0 avisos de la familia**; la ausencia de `error` está en el criterio general de
`docs/plan-compatibilidad-apps-pausado.md:37-40` (`plan-...:37-40`). Son dos criterios distintos:
esta línea base no cumple ninguno de los dos y se deja como referencia previa a Fase D, sin
atribuir a D ningún cambio todavía.

---

## 367. Quinta APK real: Libre Memory Game, línea base y clasificación fail-closed

Medido en esta sesión (2026-11-25) sobre
`/home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk`. Sin gate, CDP, chromium,
red, `git push` ni publicación. El binario es
`/home/leonisauro/.cache/apkc-web/bases-un-head/apkc`, no el `HEAD` actual ni el de `§366`; las
salidas quedan en `/home/leonisauro/.cache/apkc-web/quinto-apk/`. Es una línea base nueva: no se
fusiona con `§361` (Fase D) ni con `§366` (cuarta APK).

**Identidad del APK y del binario.**

```sh
sha256sum /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk
stat -c %s /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk
```
```
df04986688906d73ad7680b9e2ffe00773e2e27ae3132169885c273642331236  /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk
64773062
```
```sh
go version -m /home/leonisauro/.cache/apkc-web/bases-un-head/apkc
```
Salida relevante: `vcs.revision=04456d27bc0b25ed7a78851dd9fca8047ac72d4d` y
`vcs.modified=false`.

**Manifiesto y ficheros.**

```sh
aapt2 dump badging /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk
```
Salida relevante: `name='org.quentin_bettoum.librememorygame' versionCode='9' versionName='1.2.0'`,
`minSdkVersion:'24'`, `targetSdkVersion:'35'`,
`launchable-activity: name='com.godot.game.GodotApp'`, `application-isGame` y
`native-code: 'arm64-v8a' 'armeabi-v7a'`.

```sh
unzip -l /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk
```
Salida relevante: `classes.dex` 5.177.092 B; `lib/arm64-v8a/libgodot_android.so` 69.790.576 B;
`lib/armeabi-v7a/libgodot_android.so` 75.199.404 B; `libc++_shared.so` ×2;
`AndroidManifest.xml` 7.500 B y `resources.arsc` 58.776 B.

```sh
apkanalyzer manifest print /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk > /home/leonisauro/.cache/apkc-web/quinto-apk/manifest.xml
wc -l /home/leonisauro/.cache/apkc-web/quinto-apk/manifest.xml
grep -n -E 'GodotApp|org.godotengine.library.version|rendering.method' /home/leonisauro/.cache/apkc-web/quinto-apk/manifest.xml
grep -c AndroidContextProvider /home/leonisauro/.cache/apkc-web/quinto-apk/manifest.xml
```
`rc 0` y 141 líneas en `manifest.xml`. En `:42-65`, `GodotApp` con `MAIN` + `LAUNCHER`; en
`:66-69`, `org.godotengine.library.version = 4.5.1.stable`; en `:132-135`,
`rendering.method = gl_compatibility`. `grep -c AndroidContextProvider` da `0`.

**`apkc preview`.**

```sh
time /home/leonisauro/.cache/apkc-web/bases-un-head/apkc preview /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk > /home/leonisauro/.cache/apkc-web/quinto-apk/preview.json
sha256sum /home/leonisauro/.cache/apkc-web/quinto-apk/preview.json
stat -c %s /home/leonisauro/.cache/apkc-web/quinto-apk/preview.json
```
`rc 0`, `0m0.383s`, stderr `[apkc] finished in 175.945945ms`. `preview.json` tiene 2080 B y sha256
`a46d1b5d9716cf7d23b87150f1ac656ee629569f102ef0c46e112064331c7bfd`. Campos extraídos del JSON:

```
paquete=org.quentin_bettoum.librememorygame
version=1.2.0
actividad=com.godot.game.GodotApp
layout=res/layout/custom_dialog.xml
nodos=2
raiz.tipo=RelativeLayout
aproximado=True
ejecucion.metodo=onCreate
ejecucion.pasos=333
ejecucion.error=… en org.godotengine.godot.GodotActivity.<init>: … Intrinsics.createParameterIsNullExceptionMessage: se llamó a java.lang.Thread.getStackTrace sobre un objeto nulo
```
Los 5 avisos son todos de la familia «no está en el dex ni en los stubs»:
`SavedStateRegistryController$Companion.create`, `SystemClock.uptimeMillis()J`,
`Executors.newFixedThreadPool`, `Looper.getThread()Ljava/lang/Thread;` y
`Thread.currentThread()Ljava/lang/Thread;`. Da `sin-layout-xml: 0` y no aparece
`id/action_bar_title`. Los 2 nodos son `RelativeLayout` + `Button id/dialog_button` con
`texto=' Ok '` (`0x7f07002e`).

**Clasificación honesta y criterio falsable.** El emulador alcanza **solo el envoltorio Java**: la
lógica del juego vive en `libgodot_android.so` (código nativo con render GL), que el intérprete
Dalvik no ejecuta. Decir que «Libre Memory Game pasa» sería fingir cobertura. La medición se registra
como **caso negativo / fail-closed**, con aserciones sobre la evidencia real: `rc 0`, `error` que
contenga `Thread.getStackTrace sobre un objeto nulo`, `nodos=2`,
`layout=res/layout/custom_dialog.xml`, 5 avisos de la familia y sin `id/action_bar_title`, con la
etiqueta explícita «solo envoltorio, sin lógica Godot». Se refuta si al cambiar el envoltorio Java
el JSON no cambia de forma reproducible, o si alguien presenta texto del juego sin ejecutar el
`.so`. Sin gate, CDP, red ni publicación. Sostenido por `§367`.

---

## 359. Fase C: la lanzadora y el layout salen del manifiesto, sin pantalla inventada

Medido el 2026-11-25 sobre el commit integrado `98cedf1` (original `7fb6857`). Sin gate, CDP,
chromium, red, `git push` ni publicación. La actividad y la ruta del layout se leen del manifiesto y
de los recursos del propio APK: no se fija una actividad a mano ni se elige un layout por su nombre.

**Reglas medidas.** `enabled` se decide sobre el valor tipado (`tipoBooleano`), nunca con
`s != "false"`; un alias con `targetActivity` se resuelve a su actividad destino; la prioridad es
`MAIN` + `LAUNCHER` + `DEFAULT`; el orden de elección es `pedido` → `layoutID` → `activity_<Nombre>`;
y cuando no hay layout XML se emite el centinela `sin-layout-xml:` en `avisos`, **sin fallback
alfabético**. Siete tests focales pasan y la suite completa da `460.631s`:

```sh
cd apkc && go test -run 'TestLanzadora|TestSinLayoutXml|TestRutaDelLayout' -v .
cd apkc && go test -count=1 ./...
```
Siete `--- PASS`; `ok github.com/ajinasokan/apkc 460.631s`.

**Oráculo del manifiesto.** Los volcados quedan en `~/.cache/apkc-web/fase-c-evidencia/`:
`fdroid-manifest.txt` (709 líneas) y `fdroid-badging.txt` (118 líneas).

```sh
wc -l ~/.cache/apkc-web/fase-c-evidencia/fdroid-manifest.txt ~/.cache/apkc-web/fase-c-evidencia/fdroid-badging.txt
```
```
709 /home/leonisauro/.cache/apkc-web/fase-c-evidencia/fdroid-manifest.txt
118 /home/leonisauro/.cache/apkc-web/fase-c-evidencia/fdroid-badging.txt
```

**Salidas de `preview`.** JSON en `~/.cache/apkc-web/fase-c/fdroid-preview-final.json`:

```sh
/home/leonisauro/.cache/apkc-web/fase-c/apkc-fase-c preview /home/leonisauro/Downloads/F-Droid.apk > ~/.cache/apkc-web/fase-c/fdroid-preview-final.json
/home/leonisauro/.cache/apkc-web/fase-c/apkc-fase-c preview /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > ~/.cache/apkc-web/fase-c/simplefileexplorer-preview-final.json
```
`actividad=org.fdroid.MainActivity`, `layout=''`, **1 nodo** `FrameLayout` con texto «F-Droid»,
aviso `sin-layout-xml:`; cero apariciones de `IconCalculatorActivity` y de `res/A1.xml` (las dos
respuestas falsas de `§354`). El cuarto APK, en
`~/.cache/apkc-web/fase-c/simplefileexplorer-preview-final.json`: `res/R5.xml`, 30 nodos y recurso
`0x7f040000 layout/activity_main`.

**Frontera.** La lanzadora y el layout ya no se inventan, pero `sin-layout-xml:` declara que F-Droid
no trae un layout XML que el modelo pueda atribuir: ese nodo `FrameLayout` no se da por pantalla real
de la app. Sin CDP, comparación visual en navegador, gate ni publicación.

---

## 360. Fase B: el modelo ejecuta el `onCreate` del segundo dex

Medido el 2026-11-25 sobre el commit integrado `28c22de` (original `9527958`). Sin gate, CDP,
chromium, red, `git push` ni publicación. `dexDelApk` descubre `classes.dex`, `classes2.dex`, … en
orden numérico y rechaza descriptores duplicados; `ejecutaOnCreateConEventos(dxs, …)` recibe todos los
dex y elige el método por su descriptor.

**Criterio.** `ejecucion.actividad == claseCorta(descriptorDeActividad(paquete, nombreActividadC))`,
con el descriptor derivado del manifiesto (el de Fase C, no un valor inyectado) **y**, en el test,
`met.dex == dx[1]`. No se añade ningún campo nuevo al JSON. Cuatro tests focales pasan y la suite da
`210.695s`:

```sh
cd apkc && go test -run 'DescubreTodosLosDexEnOrdenNumerico|ElMetodoEjecutadoEsDelSegundoDex|ElModeloNoRegresaDelSegundoDex|RechazaDescriptorDuplicado' -v .
cd apkc && go test -count=1 ./...
```
Cuatro `--- PASS`; `ok github.com/ajinasokan/apkc 210.695s`. `dexdump` fija las actividades en
`classes2.dex` y `classes20.dex`, así que el criterio no se apoya en el primer dex:

```sh
mkdir -p /home/leonisauro/.cache/apkc-web/auditoria-docs
/home/leonisauro/opt/android-sdk/build-tools/35.0.1/dexdump -f /home/leonisauro/Downloads/F-Droid.apk > /home/leonisauro/.cache/apkc-web/auditoria-docs/fdroid-dexdump-f.txt
/home/leonisauro/opt/android-sdk/build-tools/35.0.1/dexdump -f /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_x86_64.apk > /home/leonisauro/.cache/apkc-web/auditoria-docs/termux-dexdump-f.txt
grep -E "^Opened |Class descriptor  : '(Lorg/fdroid/MainActivity;|Lcom/termux/app/TermuxActivity;)'" /home/leonisauro/.cache/apkc-web/auditoria-docs/fdroid-dexdump-f.txt /home/leonisauro/.cache/apkc-web/auditoria-docs/termux-dexdump-f.txt | grep -B1 "Class descriptor"
```
```
/home/leonisauro/.cache/apkc-web/auditoria-docs/fdroid-dexdump-f.txt:Opened '/home/leonisauro/Downloads/F-Droid.apk:classes2.dex', DEX version '037'
/home/leonisauro/.cache/apkc-web/auditoria-docs/fdroid-dexdump-f.txt:  Class descriptor  : 'Lorg/fdroid/MainActivity;'
--
/home/leonisauro/.cache/apkc-web/auditoria-docs/termux-dexdump-f.txt:Opened '/home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_x86_64.apk:classes20.dex', DEX version '037'
/home/leonisauro/.cache/apkc-web/auditoria-docs/termux-dexdump-f.txt:  Class descriptor  : 'Lcom/termux/app/TermuxActivity;'
```

**Salidas.** Los JSON de la corrida están en `~/.cache/apkc-web/fase-b-evidencia/`
(`F-Droid.json`, `newpipe.json`, `termux.json` y `simplefileexplorer.json`, con sus `.err`). NewPipe y
el cuarto APK no registran regresión frente a `§358` y `§366`.

```sh
ls ~/.cache/apkc-web/fase-b-evidencia/
```

**Frontera.** `Error==""` y las capturas reales en F-Droid y Termux quedan para las fases D y C;
aquí solo se mide que el descriptor elegido y el dex ejecutado son el correcto. Ninguna app se da por
funcionando; sin CDP, gate ni publicación.

---

## 361. Fase D: la familia de avisos de stubs baja a cero en el cuarto APK

Medido el 2026-11-25 sobre el commit integrado `376def6` (original `6fa1d34`). Sin gate, CDP,
chromium, red, `git push` ni publicación. Entran **12 stubs**: `getAction()` devuelve
`android.intent.action.MAIN`, `getClass()` devuelve un `Class` sintético y
`Dialog.setContentView` no toca `m.layoutID` (para no reescribir la ruta leída en Fase C). El guardián
de nulo queda intacto.

**Cuarto APK.** Salida en `~/.cache/apkc-web/fase-d-evidencia/preview-post.json`:

```sh
/home/leonisauro/.cache/apkc-web/fase-d/apkc-post preview /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > /home/leonisauro/.cache/apkc-web/fase-d-evidencia/preview-post.json
sha256sum ~/.cache/apkc-web/apk-nuevo-baseline-final/preview.json ~/.cache/apkc-web/fase-d-evidencia/preview-post.json
```
```
f207c0435930c3d4f5256ac68add74915651f10ec3bdd056d007df7f445741a7  /home/leonisauro/.cache/apkc-web/apk-nuevo-baseline-final/preview.json
28a6e68a772c244eee1d9f5a48a4ddf3722f5e0067cd330ec867b8669268e8c3  /home/leonisauro/.cache/apkc-web/fase-d-evidencia/preview-post.json
```
```sh
python3 -c "import json; d=json.load(open('/home/leonisauro/.cache/apkc-web/fase-d-evidencia/preview-post.json')); e=d['ejecucion']; print('pasos', e['pasos']); print('avisos', sum(1 for a in e['avisos'] if 'no está en el dex ni en los stubs' in a and not a.startswith('sin-layout-xml:'))); print('error', e.get('error')); print('nodos', d['nodos'])"
```
```
pasos 1131
avisos 0
error None
nodos 30
```
`error=None`, avisos de la familia «no está en el dex ni en los stubs» **12 → 0**, `pasos` **162 → 1131**,
`layout=res/R5.xml`, `nodos=30`. La suite da `ok github.com/ajinasokan/apkc 438.144s`:

```sh
cd apkc && go test -count=1 ./...
```

**Otras tres APK, sin regresión.** Los JSON post están en
`~/.cache/apkc-web/fase-d-evidencia/{fdroid,newpipe,termux}-post.json`: F-Droid conserva 13 avisos,
NewPipe 8 y Termux 3 (los valores de `§358`, no una familia nueva).

**Frontera.** Los doce avisos desaparecen solo en el cuarto APK; los otros tres siguen con los suyos y
con `ejecucion.error`. No se afirma que ninguna app funcione; sin CDP, comparación visual, gate ni
publicación.

---

## 362. Fase E: el tema se resuelve por id de atributo, sin campos nuevos

Medido el 2026-11-25 sobre los commits integrados `77dc8e5` y `a2a6c29` (originales `1c22cd2` y
`4c5f514`) en el emulador, y `4d4dbd9` en `studio`. Sin gate, CDP, chromium, red, `git push` ni
publicación. El tema se aplica **por id de atributo** (`textColorPrimary`, `colorBackground` y
`windowBackground`), no por el nombre del estilo, y no se añade ningún campo al JSON. Las filas salen
de `expandeListas` (`preview.go:1019-1071`).

**Falsación y suite.**

```sh
cd apkc && go test -run TestElModeloNoInventaColorSinTema -v .
PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH tcr -r 256M -- sh -c 'cd apkc && go test -count=1 ./...'
cd studio && node prueba.js
```
El test negativo `TestElModeloNoInventaColorSinTema` impide inventar un color cuando no hay tema;
la suite da `ok github.com/ajinasokan/apkc 505.205s` y `node prueba.js` → `prueba: todo en orden`.

**Frontera.** La comparación visual en navegador y la publicación quedan pendientes para Fase G
(`§364`). Sin CDP, gate ni publicación.

---

## 368. Integración de C, B, D y E sobre el emulador y la pestaña

Medido el 2026-11-25. Sin gate, CDP, chromium, red, `git push` ni publicación. Se integran los commits
del emulador C `98cedf1`, B `28c22de`, D `376def6`, E1 `77dc8e5` y E2 `a2a6c29`, y los de `studio`
`4d4dbd9` y `6cf42c1`. Los conflictos se resolvieron conservando los cinco frentes: ninguno se
descartó al fusionar.

**Suite integrada.**

```sh
cd apkc && go test -count=1 ./...
```
`ok github.com/ajinasokan/apkc 196.940s`, `rc 0`, 0 `FAIL`. El recuento de funciones de test no se mide en este `§`: el comando imprime una línea `ok`.

**Cinco JSON de la corrida integrada**, en `~/.cache/apkc-web/integracion/` (junto a
`apkc-integracion`, los `.err` y `gotmp/`):

| APK | Actividad | Layout | Nodos | Estado |
| --- | --- | --- | --- | --- |
| F-Droid (`fdroid.json`) | `org.fdroid.MainActivity` | `''` | 1 | centinela `sin-layout-xml:`; marco con texto «F-Droid» |
| NewPipe (`newpipe.json`) | `org.schabi.newpipe.MainActivity` | `res/v9.xml` | 6 | sin regresión de Fase C |
| Termux (`termux.json`) | `com.termux.app.TermuxActivity` | `res/layout/activity_termux.xml` | 13 | sin regresión de Fase C |
| Gestor (`simplefileexplorer.json`) | `com.martinmimigames.simplefileexplorer.MainActivity` | `res/R5.xml` | 30 | `error=None`, 0 avisos de la familia |
| Libre Memory Game (`librememorygame.json`) | `com.godot.game.GodotApp` | `''` | 1 | mismo `error` de `§367`, 6 avisos, marco «Libre Memory Game» |

```sh
sha256sum ~/.cache/apkc-web/integracion/simplefileexplorer.json
```
```
28a6e68a772c244eee1d9f5a48a4ddf3722f5e0067cd330ec867b8669268e8c3  /home/leonisauro/.cache/apkc-web/integracion/simplefileexplorer.json
```
El SHA del gestor coincide con el de `§361`: la integración no reescribe su medición.

**Frontera.** C, B, D y E están hechos en la capa Go/node y verificados sobre la suite integrada;
la mitad CDP (comparación visual en navegador), `tools/entorno.sh gate` y la publicación siguen sin
ejecutar. Ninguna de las cinco APK se da por funcionando: Libre Memory Game sigue siendo caso
negativo «solo envoltorio, sin lógica Godot» (`§367`) y F-Droid declara `sin-layout-xml:`. El pin
vigente del emulador, actualizado en la integración, es `a2a6c29111783ee09f6e74729c4b303b0dd8242d`
(regla 14 de `AGENTS.md`).

---

## 369. ROM con permisos: el gestor solo ve `/storage/emulated/0` con `READ` concedido

Medido el 2026-11-25 sobre los commits `03b772d` y `49a5726` del emulador, integrados en el lote
`6a207cd` (integración `1e1bb03` en el padre). Sin gate, CDP, chromium, red, `git push` ni
publicación. La ROM es un árbol fijo en memoria: `/sdcard` es solo alias de `/storage/emulated/0`, no
se toca el sistema de ficheros del aparato ni del navegador. `checkSelfPermission` devuelve `-1`
(`PERMISSION_DENIED`) por defecto y `MANAGE_EXTERNAL_STORAGE` queda denegado siempre, aunque se pida
con `--permiso`. La tabla de decisiones de los cinco permisos del corpus está en
`studio/apkc-emulador/ROM-PERMISSOS.md:24-33`.

**Tres SHA del gestor**, sobre `com.martinmimigames.simplefileexplorer_20.apk`:

```sh
sha256sum ~/.cache/apkc-web/rom-permisos/json/verificado-sin-clic.json ~/.cache/apkc-web/rom-permisos/json/verificado-con-clic.json ~/.cache/apkc-web/rom-permisos/json/verificado-con-permiso.json
```
```
28a6e68a772c244eee1d9f5a48a4ddf3722f5e0067cd330ec867b8669268e8c3  /home/leonisauro/.cache/apkc-web/rom-permisos/json/verificado-sin-clic.json
e603739d2643cf3c1819e4346369ea895c747c5e223c89741f42791c98e930fb  /home/leonisauro/.cache/apkc-web/rom-permisos/json/verificado-con-clic.json
88eb9f850be99d02c2b79658a918a37c36f874b28b9e0f99aa0fd4092c6fe831  /home/leonisauro/.cache/apkc-web/rom-permisos/json/verificado-con-permiso.json
```
El de sin clic es idéntico al de `§361` y `§368`: la ROM no reescribe esa medición. Los tres JSON se
obtienen con las tres formas del comando:

```sh
apkc preview --rom … <apk>
apkc preview --rom … --pulsar id/menu_quick_switch <apk>
apkc preview --rom … --permiso android.permission.READ_EXTERNAL_STORAGE <apk>
```

**Avisos de familia de permisos.** Los extrae este comando sobre los mismos JSON:

```sh
python3 -c "import json; b='/home/leonisauro/.cache/apkc-web/rom-permisos/json/'; [(lambda a: print(n, [x for x in a if 'ROM' in x or 'permiso' in x]))(json.load(open(b+n+'.json'))['ejecucion']['avisos']) for n in ['verificado-sin-clic','verificado-con-clic','verificado-con-permiso']]"
```
```
verificado-sin-clic []
verificado-con-clic ['java.io.File.canRead: la ROM deniega el acceso sin android.permission.READ_EXTERNAL_STORAGE concedido sobre la ruta']
verificado-con-permiso []
```

`id/list` gana **2 filas** con `/storage/emulated/0` solo en la tercera corrida (con permiso); en la
del clic el SHA cambia, pero `id/list` sigue con 0 hijos y aparece el aviso de denegación. Los hijos
vienen de `id/drive_list` y se publican en `id/list` como **aproximación documentada**, sin inventar
contenido. `pasos`: 1131 sin clic, 1213 con clic y 1251 con permiso; `error=None` en las tres.

**Frontera.** Los JSON quedan en `~/.cache/apkc-web/rom-permisos/json/`; no se afirma que la app
funcione; la mitad CDP, `tools/entorno.sh gate` y la publicación siguen fuera.

---

## 370. Lote integrado de cuatro frentes sobre `c448442`

Medido el 2026-11-25. Sin gate, CDP, chromium, red, `git push` ni publicación. Sobre `c448442` (red
sin peticiones reales) se integran en el emulador `99a4c3b` (lifecycle e hilos), `a7683be`
(comentario de `getObserverConstructorType`), `157eab5` (`<clinit>` y `fill-array-data`), `9c5ca8d`
(objetos inertes para nativos, WebView y Media3), `1fbaaa1` (retornos de arreglo inerte y avisos
recortados), `5b591ba` (avisos recortados repetidos contados una sola vez), `c3f1e55` (ROM con
permisos) y `6a207cd` (vistas añadidas por código). La integración en el padre es `1e1bb03` y la
guardia del pin se actualizó en `5b3a774`.

**Suite integrada.**

```sh
cd apkc && go test -count=1 ./...
```
```
ok github.com/ajinasokan/apkc 257.828s
```

**Cinco APK, base → lote.** Los JSON de la corrida están en
`~/.cache/apkc-web/integracion-frentes/` (con el binario `apkc-integracion-frentes`, los `.err` y
`gotmp/`):

| APK | `pasos` | avisos |
| --- | --- | --- |
| F-Droid (`fdroid.json`) | 468 → 700 | 12 → 4 |
| NewPipe (`newpipe.json`) | 379 → 508 | 8 → 3 |
| Termux (`termux.json`) | 223 → 284 | con texto de denegación de la ROM |
| Gestor (`simplefileexplorer.json`) | 1131 → 1137 | `error=None` |
| Libre Memory Game (`librememorygame.json`) | 333 → 547 | sin cambio de clasificación |

En Termux los avisos incluyen `java.io.File.exists` e `isFile` con «la ROM deniega el acceso sin
`android.permission.READ_EXTERNAL_STORAGE` concedido sobre la ruta».

**Frontera.** No se afirma que ninguna app funcione: F-Droid y NewPipe conservan su
`ejecucion.error` de `Lifecycling`, Termux el de `Set.iterator` sobre nulo y Libre Memory Game el
caso negativo «solo envoltorio, sin lógica Godot» de `§367`; el gestor es el único con `error=None`.
Sin CDP, comparación visual, gate ni publicación.

---

## 371. Colecciones, cadenas y trazas: la deuda baja sin tocar el modelo

Medido el 2026-11-25 sobre los commits `5676be4`, `8ef0b12` y `ec842d4` del emulador, en el checkout
principal (donde `d924101` integra mapas, cadenas y trazas). Sin gate, CDP, red ni publicación.
Focales en verde, 13 `--- PASS`:

```sh
cd apkc && go test -run 'TestColecciones|TestCadenas|TestStackTrace|TestArraysCopyOfRange|TestIteradorDelSet' -v .
```

**Avisos integrados → deuda**, con JSON en `~/.cache/apkc-web/colecciones/salida-integrada/` y
`~/.cache/apkc-web/colecciones/salida-deuda/`:

| APK | integrada | deuda |
| --- | --- | --- |
| F-Droid (`fdroid.json`) | 4 | 4 (`forName` queda) |
| NewPipe (`newpipe.json`) | 3 | 3 |
| Libre Memory Game (`librememorygame.json`) | 6 | 4 |
| Termux (`termux.json`) | 7 | 6 |
| Gestor (`simplefileexplorer.json`) | 1 | 1 |

`layout` y `nodos` son idénticos en las cinco entre las dos corridas: `''`/1 en F-Droid y Libre
Memory Game, `res/v9.xml`/6 en NewPipe, `res/layout/activity_termux.xml`/13 en Termux y
`res/R5.xml`/30 en el gestor. Queda como frontera el aviso `forName: la reflexión no está en el
subconjunto` (`salida-deuda/fdroid.json`), sin campo JSON nuevo. `copyOfRange` se corrigió contra
java 17: `IllegalArgumentException: 2 > 1` y `ArrayIndexOutOfBoundsException: arraycopy: length -2
is negative` (`previewstubsframework_test.go:1453-1455`).

**Frontera.** Ninguna de las cinco APK se da por funcionando; la mitad CDP, el gate y la publicación
quedan fuera de esta medición.

---

## 372. Recursos y preferencias de Termux: 223 → 3798 pasos, y 4784 en la integración final

Medido el 2026-11-25. La medición original se tomó sobre los commits `af72d22`, `5558e02`,
`4bdebeb`, `7162fdc` y `6366291` de los worktrees, cuando aún no estaban en el checkout principal;
la cadena 223 → 64 → 408 → 503 → 719 → **3798** queda como **registro fechado** de ese momento, no
como el estado del árbol actual. Los cinco commits ya están integrados en el checkout principal con
`688a7ad feat(emulador): integra la correccion de Arrays y los recursos de Termux`, con pin del
emulador `a83ebcd5654475c00bc2f52c134497e64190b646`. La guardia se comprueba con
`git -C studio rev-parse HEAD:apkc-emulador`. Sin gate, CDP, red ni publicación.

**Progreso de Termux (registro fechado)**, con JSON en `~/.cache/apkc-web/recursos/salida/`: 223
(`base-termux-app_v0.118.3+github-debug_x86_64.json`) → 64 (`con-propiedades-termux.json`) → 408
(`debug-termux.json`) → 503 (`contains-debug.json`) → 719 (`estaticos2-termux.json`) → **3798**
(`inmutable2-termux.json`). Desaparecen `Set.iterator` de propiedades, `containsKey`/`contains`,
los conjuntos estáticos nulos e `ImmutableBiMap.entrySet`. En esa corrida quedaba la frontera
`java.io.File.isFile sobre nulo` en `checkForFontAndColors`, en el `ejecucion.error` de
`inmutable2-termux.json`.

**Corrida final sobre el checkout integrado.** El JSON es
`~/.cache/apkc-web/integracion-final/termux.json`:

```sh
python3 -c "import json; d=json.load(open('/home/leonisauro/.cache/apkc-web/integracion-final/termux.json')); print('pasos', d['ejecucion']['pasos']); print('error', d['ejecucion'].get('error'))"
```
```
pasos 4784
error en com.termux.app.TermuxActivity.setTermuxTerminalViewAndClients: en com.termux.app.terminal.TermuxTerminalSessionClient.onCreate: en com.termux.app.terminal.TermuxTerminalSessionClient.checkForFontAndColors: en com.termux.terminal.TerminalColorScheme.updateWith: se llamó a java.util.Set.iterator sobre un objeto nulo
```
De **3798 a 4784 pasos**: la integración `688a7ad` corrige `Arrays.copyOfRange` y los recursos y
preferencias de Termux, y deja una frontera nueva, `Set.iterator` dentro de
`TerminalColorScheme.updateWith`, en lugar del `java.io.File.isFile sobre nulo` anterior.

**Nota del 2026-11-25 (esquema de colores de Termux, posterior a la corrida final).** El frente
avanzó con el cherry-pick `bf7477e7feef577a9ea554120a5b07d824274dde` en
`studio/apkc-emulador`, el gitlink de studio `ed39a58f29f89f445b45f4a0181fcbbb813b774f` y el
padre `84a7b90b84661450601cdd4d3554e86c49b2acd7`; la guardia vigente de la regla 14 da
`git -C studio rev-parse HEAD:apkc-emulador` → `bf7477e…`. La suite integrada
`go test -count=1 ./...` → `ok github.com/ajinasokan/apkc 170.448s`, 0 `FAIL`, y las focales
`ok … 0.041s`. Termux llega a **6599 pasos** (antes 4784) y la nueva frontera es
`ejecucion.error` en `setTerminalToolbarView: se leyó el campo
android.view.ViewGroup$LayoutParams.height de un objeto nulo`; desaparece `Set.iterator` en
`TerminalColorScheme.updateWith`. Las otras cuatro APK siguen byte a byte idénticas a las de
`integracion-final/`. El JSON está en `~/.cache/apkc-web/integracion-color/termux.json` y se
reproduce con `apkc preview` sobre la variante arm de la APK de Termux:

```sh
apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk > ~/.cache/apkc-web/integracion-color/termux.json
```

**Las otras cuatro APK.** En el registro fechado, tres quedan byte a byte idénticas a la base
—F-Droid (`9bd87930…`), NewPipe (`67b498c1…`) y Libre Memory Game (`1b70842e…`), con el mismo SHA
en `base-*.json` y `inmutable2-*.json`— y el gestor solo con el aviso informativo
`preferencias-memoria: getSharedPreferences("com.martinmimigames.simplefileexplorer") usa un
almacén en memoria, sin fichero` (`inmutable2-simplefileexplorer.json`).

**Frontera.** La verificación de cero peticiones fuera del origen queda para Fase G. Los worktrees
ya están integrados en el checkout principal (`688a7ad`); ninguna APK se da por funcionando; sin
CDP, gate ni publicación.

---

## 373. Relay WISP propio como opción configurada, sin activar el módulo

Medido el 2026-11-25 sobre los commits integrados de `studio/` (`ffcc49a`, `64eac8a`, `deda61c`)
y del padre (`18a564b`, `d15cf3c`, `682c6ec`). Sin gate externo, CDP, red ni publicación.

**Dominio y puertos.** `getent hosts wisp.dolci-vita.link` → `163.192.151.24`. El dominio es del
usuario, registrado en Namecheap, y el correo de Let's Encrypt es `leonisaurox@proton.me`. Los
puertos 80 y 443 están abiertos y persistidos en la VM; la sonda con `curl` al dominio devuelve
`Connection refused`, porque todavía no hay ningún servicio escuchando.

```sh
getent hosts wisp.dolci-vita.link
```

**La opción queda configurada, no activa.** Studio `ffcc49a` precarga
`wss://wisp.dolci-vita.link/wisp/` en `#wisp-url` (`studio/index.html:1279`). Studio `64eac8a`
guarda `if (!eleccion.activo()) throw new Error("módulo WISP desactivado…")` en `relayActual`
(`studio/wisp.js:598-601`): el campo puede estar precargado, pero con el módulo apagado no se
devuelve ninguna URL. La falsación es parte de la medición: sin ese guarda, `prueba.js` fallaba
devolviendo la URL apagada (`studio/prueba.js:5882-5886`). Studio `deda61c` hace que el contador
del gate cuente solo `obtener`.

En el padre, `18a564b` y `d15cf3c` introducen la constante `relayPropioWISP`
(`apkc/depswisp.go:77`) y la aceptación por igualdad exacta de la URL en
`validarRelayAutorizado` (`apkc/depswisp.go:389-412`), sin activar
`PermitirRelaysPersonalizados` (`apkc/depswisp.go:390`). Las variantes canónicas —puerto 443
explícito, mayúsculas en el esquema o el host, ruta distinta— se rechazan a propósito: el relay
propio no se normaliza como la lista de terceros. `682c6ec` integra el gitlink del submódulo.

**Gates y pruebas**, cada una con su comando:

```sh
sh tools/correr.sh wisp          # rc 0
sh tools/correr.sh wisp-externo  # sin variable externa: rc 2, «gate externo omitido»
cd studio && node prueba.js      # prueba: todo en orden
cd apkc && go test -run TestRelayPropioAceptadoPorComparacionExacta -v .  # PASS
sh tools/correr.sh test          # ok … 205.713s
```

**Sin activación del módulo.** `#wisp-selector` sigue `hidden` (`studio/index.html:1273`), los
seis relays de terceros siguen en `RelaysWISP()` (`apkc/depswisp.go:63`),
`PermitirRelaysPersonalizados` no se activa y no hay campo nuevo en el JSON del emulador.

**Frontera de la medición original (2026-11-25).** Las cifras de gates de arriba se conservan tal
como se midieron: el relay Go de `tools/wispd/` estaba aprobado para despliegue y no se afirmaba
que WSS funcionara, sin petición externa mientras el módulo estaba apagado, con el gate
`wisp-externo` omitido sin su variable y sin CDP, red ni publicación.

**Nota del 2026-11-25 (fechada, posterior a la medición).** El relay sí se desplegó en la VM
`163.192.151.24` y se verificó en remoto; el despliegue y sus siete resultados están en `§374`. La
frontera actual es otra: el módulo WISP de la página sigue apagado, el relay no está integrado en la
navegación y `prueba-sitio.js`/CDP quedan pendientes de autorización.

---

## 374. Primer relay WISP propio desplegado y verificado en remoto

Nota del 2026-11-25, posterior a `§373`: el relay Go de `tools/wispd/` pasó de aprobado a
desplegado. Las cifras del `§373` se conservan tal como se publicaron; aquí se registra el
despliegue y su verificación en remoto. No se ha corrido gate con variable externa, CDP ni
`prueba-sitio.js`, y no hay publicación: la red se usó solo para desplegar y medir el relay.

### Despliegue

La VM es Oracle, Ubuntu 22.04.5, con IP `163.192.151.24`. Los puertos 80 y 443 están abiertos y
persistidos con iptables: `4 dpt:22 ACCEPT`, `5 dpt:443`, `6 dpt:80` y `7 REJECT`. SSH sigue intacto.

```sh
lsb_release -a
hostname -I
iptables -S
```

El binario subido a `~/wispd` tiene SHA256
`0c7f334b9ae0d1317baf96c7a39b225f064be5331b1b2c9c7be8760ccebbbeed` y está compilado desde
`tools/wispd/`, integrado en `web` en los commits `a7ac8d2`, `a836010` y `3cc6ee7`.

```sh
sha256sum ~/wispd
git show --stat --oneline a7ac8d2 a836010 3cc6ee7
```

El certificado es de Let's Encrypt para `wisp.dolci-vita.link`, con correo `leonisaurox@proton.me`,
`notAfter=2027-02-23` y `certbot.timer` activo (90 días de validez al medir: 2026-11-25 →
2027-02-23 con `date -d`).

```sh
openssl x509 -in /etc/letsencrypt/live/wisp.dolci-vita.link/cert.pem -noout -enddate
systemctl status certbot.timer
echo $(( ( $(date -d 2027-02-23 +%s) - $(date -d 2026-11-25 +%s) ) / 86400 ))
```

El servicio `wispd.service` está `active` y `enabled`: `ExecStart=/home/ubuntu/wispd -addr :443
-cert …/fullchain.pem -key …/privkey.pem`, `Restart=on-failure`, `MemoryMax=640M`, `User=ubuntu` y
`AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_DAC_READ_SEARCH`. La capacidad se conserva por una
decisión consciente: certbot recrea la clave como `root:root 600` y sustituir la capacidad exigiría
un deploy-hook. `debug.SetMemoryLimit(512<<20)` en `main.go` es el único mecanismo de memoria; el
`Environment=GOMEMLIMIT` de la unidad se está retirando.

```sh
systemctl status wispd.service
systemctl cat wispd.service
```

Desde el escritorio, `curl https://wisp.dolci-vita.link/wisp/` → `403` al no llevar `Origin`.
Desde la VM, `curl -Iv` da `SSL certificate verify ok`, TLS 1.3 y ALPN `h2`.

```sh
curl -sS -o /dev/null -w '%{http_code}\n' https://wisp.dolci-vita.link/wisp/
curl -Iv https://wisp.dolci-vita.link/wisp/
```

No hay `-dev`; los seis relays de terceros siguen intactos y el módulo WISP de la página sigue
apagado.

### Verificación Wisp v1 en remoto

La sonda son tests de Go ejecutados con `GOPROXY=off` contra
`wss://wisp.dolci-vita.link/wisp/`, desde una copia de trabajo de `tools/wispd/` bajo caché:
`~/.cache/apkc-web/wisp-verificacion/wispd/`. El fichero `verificacion_remota_test.go` se añadió
solo en esa copia, con los tests `TestRemoto01Handshake` … `TestRemoto06DestinoPermitido` y
`TestRemoto07`; no está versionado y el repositorio sigue intacto. Las tramas que siguen se
transcriben tal como se observaron, sin inventar salidas.

```sh
# desde ~/.cache/apkc-web/wisp-verificacion/wispd/
tcr env GOTOOLCHAIN=local GOMODCACHE=/home/leonisauro/.cache/apkc-web/wispd/modcache GOTMPDIR=/home/leonisauro/.cache/apkc-web/wisp-verificacion/gotmp GOPROXY=off /home/leonisauro/sdk/go1.24.0/bin/go test -v -count=1 -run TestRemoto ./...
# sondeo adicional: el mismo comando con -run TestRemoto07 ./...
```

| # | Sonda | Resultado observado | Lectura |
| --- | --- | --- | --- |
| 1 | primer mensaje del handshake | 9 bytes `030000000040000000` | `tipo=0x03`, `streamID=0`, `bufferRemaining=64` |
| 2 | `Origin: https://mal.example` | `403` | origen ajeno, rechazado |
| 3 | `Origin: null` | `403` | origen `null`, rechazado |
| 4 | CONNECT `127.0.0.1:443` | `040100000041` | `CLOSE`, motivo `0x41`, antes de DATA |
| 5 | CONNECT `repo1.maven.org:80` | `040200000041` | `CLOSE`, motivo `0x41` |
| 6 | CONNECT `example.com:443` | `040700000048` | `CLOSE`, motivo `0x48` |
| 7 | CONNECT `repo1.maven.org:443` | sin `CLOSE` en 6 s | flujo aceptado, sin tráfico HTTP |

Los rechazos de origen son falsificables con `curl`, así que no son autenticación:

```sh
curl -i -H 'Origin: https://mal.example' https://wisp.dolci-vita.link/wisp/  # 403
curl -i -H 'Origin: null' https://wisp.dolci-vita.link/wisp/                 # 403
```

**Discrepancia aceptada.** Un literal IP cierra con `0x41` (información inválida), no con `0x48`
(destino bloqueado). Se acepta como comportamiento válido del relay, no como fallo de seguridad.

**Frontera.** El relay está desplegado y verificado en remoto, pero no se ha integrado en la
navegación de la página: el módulo WISP sigue apagado. `prueba-sitio.js`/CDP siguen pendientes de
autorización; sin gate con variable externa, red adicional ni publicación.

---

## 375. Boolean, opcode `0xd5` y primeras filas reales del gestor

Medido el 2026-11-25, posterior a `§374`. Sobre el pin `bf7477e7feef577a9ea554120a5b07d824274dde`
entraron cuatro cherry-picks en `studio/apkc-emulador`; el primer destino real fue el gestor de
archivos (`com.martinmimigames.simplefileexplorer`). No se ha corrido gate, CDP ni publicación. El
gestor **no se da por funcionando** y no se afirma que aparezcan los cuatro nombres de fichero que se
están siguiendo: aquí solo se registran la actividad, los textos y la frontera que muestra el modelo.

### Cuatro cherry-picks y la guardia vigente

| Commit | Cambio |
| --- | --- |
| `f33bfdb` | Boolean y opcode `0xd5` (`and-int/lit16`) |
| `7213e25` | `listFiles` con filtro para el gestor |
| `6d84fcf` | clic por índice sobre hijos con `onClick` |
| `2a42a84` | `Callable.call` en `FutureTask` |

```sh
git show --stat --oneline f33bfdb 7213e25 6d84fcf 2a42a84   # desde studio/apkc-emulador
git -C studio rev-parse HEAD:apkc-emulador
git rev-parse --short HEAD
git -C studio rev-parse --short HEAD
git ls-tree HEAD studio
```

Salida de las cuatro últimas órdenes: `2a42a846a87df44d66178ca9000aabdc764d9a04`, `c179a0b`,
`a4d8f43` y `160000 commit a4d8f434988bf55391d5c647613e709e6199f82c	studio`. Es el pin vigente de
la regla 14; `bf7477e…` queda como registro histórico fechado.

### Suite y contrato de ficheros

`go test -count=1 ./...` → `ok … 162.890s`, 0 `FAIL`; 11 pruebas focales en PASS. Los siete nombres
que añaden los cuatro commits son `TestAndIntLit16LeeLosNibblesDeD8`,
`TestPreviewBooleanValueOfGuardaElBit`, `TestListFilesConFiltroQueAceptaTodo`,
`TestListFilesFiltraPorExtension`, `TestPulsarHijoContenedorSinId`,
`TestFutureTaskEjecutaCallDiferido` y `TestFutureTaskNoRompeRunnableRun`; el informe del integrador
da el total de 11 focales sin desglosar las otras cuatro, que no se transcriben aquí. La materialización
del emulador conserva los 34 ficheros `preview*.go` byte a byte con `cmp` (regla 14) y
`git diff --check` da rc 0.

```sh
go test -count=1 ./...
go test -count=1 -v -run 'TestAndIntLit16LeeLosNibblesDeD8|TestPreviewBooleanValueOfGuardaElBit|TestListFilesConFiltroQueAceptaTodo|TestListFilesFiltraPorExtension|TestPulsarHijoContenedorSinId|TestFutureTaskEjecutaCallDiferido|TestFutureTaskNoRompeRunnableRun' .
git diff --check
```

### Las cinco APK

Binario: `~/.cache/apkc-web/integracion-gestor-boolean/apkc-integracion-gestor-boolean`. Cada corrida
escribe su JSON en `~/.cache/apkc-web/integracion-gestor-boolean/`:

```sh
apkc preview /home/leonisauro/Downloads/F-Droid.apk > ~/.cache/apkc-web/integracion-gestor-boolean/fdroid.json
apkc preview /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk > ~/.cache/apkc-web/integracion-gestor-boolean/newpipe.json
apkc preview /home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk > ~/.cache/apkc-web/integracion-gestor-boolean/librememorygame.json
apkc preview /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > ~/.cache/apkc-web/integracion-gestor-boolean/simplefileexplorer.json
apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk > ~/.cache/apkc-web/integracion-gestor-boolean/termux.json
```

| APK | JSON | Pasos y nodos medidos |
| --- | --- | --- |
| F-Droid | `fdroid.json` | 758 pasos, 1 nodo, `layout=''`; byte-idéntica a `integracion-color/fdroid.json` |
| NewPipe | `newpipe.json` | 508 pasos, 6 nodos, `res/v9.xml`; byte-idéntica a `integracion-color/newpipe.json` |
| Libre Memory Game | `librememorygame.json` | 1688 pasos, 1 nodo, `layout=''`; byte-idéntica a `integracion-color/librememorygame.json` |
| Gestor de archivos | `simplefileexplorer.json` | 1137 → **1330** pasos y 30 → **33** nodos, `error=None`, `layout=res/R5.xml`; la línea base es `control-simplefileexplorer.json` |
| Termux | `termux.json` | 6599 pasos, 13 nodos, sin regresión; frontera `LayoutParams.height` en `setTerminalToolbarView` |

```sh
cmp ~/.cache/apkc-web/integracion-gestor-boolean/fdroid.json ~/.cache/apkc-web/integracion-color/fdroid.json
cmp ~/.cache/apkc-web/integracion-gestor-boolean/newpipe.json ~/.cache/apkc-web/integracion-color/newpipe.json
cmp ~/.cache/apkc-web/integracion-gestor-boolean/librememorygame.json ~/.cache/apkc-web/integracion-color/librememorygame.json
python3 -c "import json; d=json.load(open('/home/leonisauro/.cache/apkc-web/integracion-gestor-boolean/simplefileexplorer.json')); print(d['ejecucion']['pasos'], d['nodos'], d['ejecucion'].get('error'), d['layout'])"
```

### El gestor: cómo avanza y dónde se queda

Sin clic, el JSON `simplefileexplorer.json` pasa de 1137/30 a **1330/33**, con `error=None` y
`layout=res/R5.xml`. Entre los 17 textos de `ejecucion.capturas` aparecen `Loading...` y
`Access Denied`; son escrituras de `TextView.setText`, no capturas visuales ni una lista de ficheros
confirmada.

Con dos sondas de clic sobre el mismo binario se ve el primer cambio de actividad:

```sh
apkc preview --pulsar id/menu_quick_switch --pulsar id/drive_list:0 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
apkc preview --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
```

La primera entra en `MainActivity.k(File)`; la segunda entra en `Download` y muestra `fotos`, pero
todavía no muestra `leeme.txt`. Los JSON de estas dos sondas finales no quedaron guardados en
`integracion-gestor-boolean/` (pendiente de archivar): las salidas se transcriben del informe del
integrador. Las sondas de cada frente sí dejaron su JSON —`~/.cache/apkc-web/gestor-listfiles/sfm-switch.json`,
`~/.cache/apkc-web/gestor-filtro/final.json` y
`~/.cache/apkc-web/gestor-futuretask/sfm-click-futuretask-download.json`—, que son mediciones de los
frentes por separado, no la corrida final.

**Frontera.** El gestor avanza y cambia de actividad, pero no se da por funcionando: no se afirma que
aparezcan los cuatro nombres que se están siguiendo ni que la navegación esté terminada; `leeme.txt`
sigue sin verse en la sonda final. F-Droid, NewPipe y Libre Memory Game quedan byte-idénticos a
`integracion-color/` y Termux sin regresión. Sin CDP, gate ni publicación.

## 376. Gestor de archivos: navegación y filas reales desde la ROM sembrada

Medido el 2026-11-25, posterior a `§375`. Sobre el pin `2a42a846a87df44d66178ca9000aabdc764d9a04`
entró un cherry-pick nuevo en `studio/apkc-emulador`:
`0c01986d356c6eeb20a20809e5b96c74468d34ae`, del commit `fd1e5ac`, aplicado sobre `2a42a84`. El
gitlink anidado de `studio` es `a7e5ec847b934678aa3ab096a9d8cac077157555` y el padre es `69e798c`;
la guardia de la regla 14 da `0c01986d356c6eeb20a20809e5b96c74468d34ae`. No se ha corrido gate, CDP
ni publicación. El gestor **no se da por funcionando**: aquí solo se registra que muestra dos entradas
de la ROM sembrada.

### Comando, guardia y contrato de ficheros

```sh
git show --stat --oneline 0c01986d356c6eeb20a20809e5b96c74468d34ae
git -C studio rev-parse HEAD:apkc-emulador
git rev-parse --short HEAD
git -C studio rev-parse --short HEAD
git ls-tree HEAD studio
go test -count=1 ./...
git diff --check
```

Salida de la guardia y de los dos `rev-parse --short`: `0c01986d356c6eeb20a20809e5b96c74468d34ae`,
`69e798c` y `a7e5ec8`. `go test -count=1 ./...` → `ok … 197.304s`; 12 pruebas focales en PASS (el
informe del integrador da el total sin desglosar sus nombres, que no se transcriben aquí). La
materialización conserva los 34 ficheros `preview*.go` byte a byte con `cmp` (regla 14) y
`git diff --check` da rc 0.

### Tres pulsos sobre la ROM sembrada

El comando literal, con el permiso concedido y tres pulsos encadenados, es:

```sh
apkc preview --rom studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
```

La corrida escribe su JSON en `~/.cache/apkc-web/integracion-filas/gestor-tres-pulsos.json`:

| Corrida | Pasos | `layout` | `id/list` |
| --- | --- | --- | --- |
| Sin pulsos (`§375`) | 1330 | `res/R5.xml` | 33 nodos, sin las dos entradas de la ROM |
| Tres pulsos | 3596 | `res/R5.xml` | `fotos` y `leeme.txt` |

Las dos dan `error=None`. La línea base sin pulsos es el efecto del lote anterior (`§375`), no una
medición nueva de este `§`.

```sh
python3 -c "import json; d=json.load(open('/home/leonisauro/.cache/apkc-web/integracion-filas/gestor-tres-pulsos.json')); print(d['ejecucion']['pasos'], d['ejecucion'].get('error'), d['layout'])"
```

### Las cuatro APK de control

Las cuatro APK que no son el gestor siguen byte-idénticas a las de `integracion-gestor-boolean/`:
F-Droid, NewPipe, Libre Memory Game y Termux. El cotejo se transcribe del informe del integrador; las
rutas de los JSON de esas cuatro corridas no se transmitieron en él y quedan pendientes de archivar.

**Frontera.** El gestor no se da por funcionando: solo se afirma que, con la ROM sembrada y esos tres
pulsos, `id/list` muestra `fotos` y `leeme.txt`. El icono y la extensión de esas filas quedan sin
resolver: `String.lastIndexOf` sigue abierto. Sin CDP, gate ni publicación.

## 377. Termux: LayoutParams reales y frontera bindService

Medido el 2026-11-25 sobre el árbol final, posterior a `§376`. Sobre el pin
`0c01986d356c6eeb20a20809e5b96c74468d34ae` entró un cherry-pick nuevo en `studio/apkc-emulador`:
`c54e55114568b05947170a879f989e550b6766ee`, del commit `0c17130`, aplicado sobre `0c01986`. El
gitlink anidado de `studio` es `b54a0b4342356657ecafbdf3d7e19293a332a6dc` y el padre es
`acc7d4483339b8d6e3b2dca5ce7113194736fa42`; la guardia de la regla 14 da
`c54e55114568b05947170a879f989e550b6766ee`. El APK es el mismo cuya identidad queda fijada en `§354`
(sha256 `89416397b70f9ff67a0044e8abe6ef82487cd48fcf543e2d23e02688cc541cf0`). Las cifras de este
`§` son del informe del integrador, medidas sobre el árbol final y transcritas aquí sin re-medir.
No se ha corrido gate, CDP ni publicación. Termux **no se da por funcionando**.

### Comando, guardia y contrato de ficheros

```sh
git show --stat --oneline c54e55114568b05947170a879f989e550b6766ee
git -C studio rev-parse HEAD:apkc-emulador
git rev-parse --short HEAD
git -C studio rev-parse --short HEAD
go test -count=1 ./...
git diff --check
```

`go test -count=1 ./...` → `ok … 196.481s`; 17 pruebas focales en PASS (el informe del integrador da
el total sin desglosar sus nombres, que no se transcriben aquí). La materialización conserva los 34
ficheros `preview*.go` byte a byte (regla 14), `git diff --check` da rc 0 y no aparece `DEPURACION`.

### Termux antes y después

El comando literal de la corrida final es:

```sh
apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
```

La corrida escribe su JSON en `~/.cache/apkc-web/integracion-layout/termux.json`:

| Corrida | Pasos | `ejecucion.error` |
| --- | --- | --- |
| Antes (`§372`) | 6599 | `setTerminalToolbarView`: se leyó `android.view.ViewGroup$LayoutParams.height` de un objeto nulo |
| Árbol final (`§377`) | 6944 | `com.termux.app.TermuxActivity.onCreate: java.lang.RuntimeException: bindService() failed` |

En la corrida final, `layout=res/layout/activity_termux.xml`, 13 nodos y 25 avisos
(`avisos-recortados: 40`). El error anterior de `setTerminalToolbarView` desaparece. El aviso de
`getLayoutParams` está recortado: no aparece entre los 25 visibles, así que no se da por medido; el
cambio de error confirma que el stub se ejecutó, no que Termux funcione. No se transcriben aquí otros
campos del JSON que el informe del integrador no entregó.

### Las cuatro APK de control

Las otras cuatro APK —F-Droid, NewPipe, Libre Memory Game y el gestor de tres pulsos— siguen
byte-idénticas a `~/.cache/apkc-web/integracion-filas/*.json`. El cotejo se transcribe del informe del
integrador.

**Frontera.** Termux no se da por funcionando: `onCreate` termina en
`java.lang.RuntimeException: bindService() failed`. Sin CDP, gate ni publicación.

## 378. Termux: integración de `queryBroadcastReceivers` y aviso recortado

Medido el 2026-11-25 sobre el árbol final, posterior a `§377`. La integración `ed53d88`
(`feat(emulador): integra queryBroadcastReceivers para Termux`) deja el pin de la regla 14 en
`f6bc0ba`. El APK es el mismo cuya identidad queda fijada en `§354` (sha256
`89416397b70f9ff67a0044e8abe6ef82487cd48fcf543e2d23e02688cc541cf0`). Las cifras de este
`§` son del informe del integrador, medidas sobre el árbol final y transcritas aquí sin re-medir.
No se ha corrido gate, CDP ni publicación. Termux **no se da por funcionando**.

### Comando, guardia y contrato de ficheros

```sh
git show --stat --oneline ed53d88
git -C studio rev-parse d0df9e245117b413e3d1bd2f8f9a1db81812d1ed:apkc-emulador
git rev-parse --short HEAD
git -C studio rev-parse --short HEAD
cd apkc && go test -count=1 ./...
git diff --check
```

`git show --stat --oneline ed53d88` da `ed53d88 feat(emulador): integra queryBroadcastReceivers
para Termux` y `studio | 2 +-`. La guarda de la regla 14 queda en `f6bc0ba`; el informe del
integrador no entregó el gitlink de `studio` ni el padre del commit, El gitlink de `studio` en `ed53d88` es `d0df9e245117b413e3d1bd2f8f9a1db81812d1ed` (`git ls-tree ed53d88 studio`) y su padre es `d4d36cbce1647df1d18a11049a18f58e6f5eb2f0` (`git rev-parse ed53d88^`). `go test -count=1 ./...` → `ok … 162.481s`; 28 pruebas en PASS (el informe del
integrador da el total sin desglosar sus nombres, que no se transcriben aquí). La
materialización conserva los 34 ficheros `preview*.go` byte a byte (regla 14) y `git diff --check`
da rc 0.

### Termux antes y después

El comando literal de la corrida final es:

```sh
apkc preview /home/leonisauro/Downloads/termux-app_v0.118.3+github-debug_armeabi-v7a.apk
```

La corrida escribe su JSON en `~/.cache/apkc-web/integracion-broadcast/termux.json`:

| Corrida | Pasos | `ejecucion.error` |
| --- | --- | --- |
| Antes (`§377`) | 6950 | `en com.termux.shared.termux.TermuxUtils.sendTermuxOpenedBroadcast: se llamó a java.util.List.iterator sobre un objeto nulo` |
| Árbol final (`§378`) | 7072 | `en com.termux.app.TermuxActivity.addTermuxActivityRootViewGlobalLayoutListener: se llamó a android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo` |

En la corrida final, `layout=res/layout/activity_termux.xml`, 13 nodos y 25 avisos
(`avisos-recortados: 42`). `List.iterator` desaparece del error y lo sustituye
`addOnGlobalLayoutListener` sobre un objeto nulo: eso no implica que Termux arranque. El aviso
`queryBroadcastReceivers-inerte` está recortado, fuera de los 25 visibles, así que no se da por
medido: la preview no avisa al usuario de que ese aviso existe. No se transcriben aquí otros
campos del JSON que el informe del integrador no entregó.

### Frontera

Termux no se da por funcionando: `onCreate` termina en
`en com.termux.app.TermuxActivity.addTermuxActivityRootViewGlobalLayoutListener: se llamó a
android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo`. Sin CDP, gate
ni publicación.

## 379. `and-int/lit8` y `System.arraycopy`: correcciones generales sobre `90d77d7`

Medido el 2026-11-25 sobre el árbol final, posterior a `§378`. El commit
`e0f2f8a6dffa6284a50955c29119be30950fe341` se integra en `90d77d7` y deja el pin de la regla 14
en `d722e47`. Las cifras de este `§` son del informe del integrador, medidas sobre el árbol final
y transcritas aquí sin re-medir. No se ha corrido gate, CDP ni publicación. Ninguna APK se da
por funcionando.

### Comandos y contrato de ficheros

Desde `apkc/`, el comando focal es:

```sh
go test -count=1 -run 'TestAndIntLit8LeeElRegistroDeUnByte|TestArraycopyCopiaYRango' -v .
```

El informe del integrador da 29 pruebas en PASS y la suite en `211.672s`; no desglosa los
nombres de las pruebas ni entrega la salida literal del comando de la suite completa, que
quedan pendientes de archivar. `git diff --check` da rc 0.

### Las cinco APK

Las cinco corridas escriben su JSON en `~/.cache/apkc-web/integracion-general-f0/`:

| APK | `paquete` | Pasos |
| --- | --- | --- |
| Termux | `com.termux` | 7072 |
| NewPipe | `org.schabi.newpipe` | 517 |
| Gestor de archivos | `com.martinmimigames.simplefileexplorer` | 3595 |
| F-Droid | `org.fdroid.fdroid` | 758 |
| Godot | `org.quentin_bettoum.librememorygame` | 1688 |

El JSON de Termux de esta corrida es idéntico al de `integracion-broadcast/termux.json` (`§378`)
en pasos, error, avisos y raíz; difiere solo en `avisos-recortados` (41 frente a 42). No se dice
que sea byte-idéntico: el cotejo se transcribe del informe del integrador y cubre solo esos
campos. La divergencia con el build de 6950 pasos de `integracion-bindservice/termux.json` es
anterior a este commit, no un efecto del mismo.

### Frontera

Ninguna APK se da por funcionando: las cuatro con error registrado en su JSON y el gestor sin
pulsos sobre la ROM. Sin CDP, gate ni publicación.
