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
`a4d8f43` y `160000 commit a4d8f434988bf55391d5c647613e709e6199f82c	studio`. Es el pin de entonces de
la regla 14; pin vigente `c0e60ca39ee746abb5350fe4d6533de683d7467f`; `996ca07ea35735234713faaa4fc9ee8388720d02`, `9a6ec572fc72dcb374e692d094b627b1960c8efc` y `bf7477e…` quedan como registros históricos fechados.

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
integrador no entregó el gitlink de `studio` ni el padre del commit. El gitlink de `studio` en `ed53d88` es `d0df9e245117b413e3d1bd2f8f9a1db81812d1ed` (`git ls-tree ed53d88 studio`) y su padre es `d4d36cbce1647df1d18a11049a18f58e6f5eb2f0` (`git rev-parse ed53d88^`). `go test -count=1 ./...` → `ok … 162.481s`; 28 pruebas en PASS (el informe del
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

## 380. Reglas relativas del layout del gestor

Medido el 2026-11-25. El commit `d8d09e84f6670af95061d1da73f09bfb56caaa55` se integra en
`9783389`. Las cifras de este `§` son del informe del integrador, transcritas aquí sin re-medir el
código; los rects se verifican sobre el par de artefactos archivados
`~/.cache/apkc-web/gestor-geometria/gestor-despues.json` frente a
`gestor-antes.json`. Los dos commits citados no están en los objetos de este worktree, así que su
identidad queda pendiente de verificar allí. Sin CDP: son rects del JSON, no semejanza visual.

### Comando reproducible

El binario del árbol integrado y el guion del gestor con la ROM y los tres pulsos:

```sh
/home/leonisauro/.cache/apkc-web/integracion-geometria/apkc-integrado preview --rom studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
python3 /home/leonisauro/.cache/apkc-web/integracion-geometria/analiza.py
```

La corrida del binario integrado sobre la ROM de este worktree da `rc 0` y
`3595 None res/R5.xml 37` (pasos, `error`, `layout` y nodos). El analizador sobre los dos JSON
archivados da la comparación siguiente; el comando lee
`~/.cache/apkc-web/integracion-general-f0/gestor-tres-pulsos.json` (antes) y
`~/.cache/apkc-web/integracion-geometria/gestor-tres-pulsos.json` (después), y el par equivalente en
`gestor-geometria/` es byte-idéntico a ambos (`sha256sum`: `6c9ca8e2d56165be4d6820b6423d5c2d0eeedf12e7302854f3aefa57dd6d9bd7` y
`856b6a12ef96fcbb8156595ab069c982ba8ae3bcf1a9f5368647fe562b9ecb52`).

### Rect antes y después

| Campo | Antes (`gestor-antes.json`) | Después (`gestor-despues.json`) |
| --- | --- | --- |
| `id/menu_button` | x 0 | x 265 |
| `id/title` | `(0, 0, 360, 30)` | `(80, 0, 185, 30)` |
| `ScrollView` | `(0, 0, 360, 640)` | `(0, 68, 360, 572)` |

Los seis valores son `x`, `y`, `ancho` y `alto` del `rect` de cada nodo. El analizador cuenta 29
rects cambiados y siete paneles con `id` recolocados (`id/list`, `id/drive_list`, `id/menu_list`,
`id/quick_selection`, `id/single_select_operation`, `id/select_operation` y `id/paste_operation`).
Ningún nodo queda fuera de 360×640: ` == gestor: nodos fuera de 360x640 == 0`. El focal es
`TestGeometriaReglasRelativasDelLayout`, en verde en `gestor-transitorias/tests.log:33`
(`--- PASS: TestGeometriaReglasRelativasDelLayout (0.00s)`).

### Frontera

Son posiciones del modelo en el JSON, no una comprobación visual: sin CDP, gate ni publicación. El
gestor no se da por funcionando.

## 381. Colores del tema del gestor desde el APK

Medido el 2026-11-25. El commit `19b5236d09d50387ac13ddd0580bf6f66cf77ae8` se integra en
`ae9347d`. Se comparan `~/.cache/apkc-web/integracion-tema-setadd/despues-tema/gestor-tres-pulsos.json`
frente a `~/.cache/apkc-web/integracion-geometria/gestor-tres-pulsos.json`. El commit citado no está
en los objetos de este worktree, así que su identidad queda pendiente de verificar allí. No se afirma
fidelidad cromática ni parecido visual: sin CDP.

### Comando reproducible

```sh
/home/leonisauro/.cache/apkc-web/integracion-tema-setadd/apkc-integrado preview --rom studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk
python3 /home/leonisauro/.cache/apkc-web/integracion-tema-setadd/compara.py /home/leonisauro/.cache/apkc-web/integracion-geometria /home/leonisauro/.cache/apkc-web/integracion-tema-setadd/despues-tema
apkanalyzer manifest print /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk | grep theme
```

La corrida del binario integrado da `rc 0` y `3595 None 37` (pasos, `error` y nodos), y su JSON
coincide campo a campo con `despues-tema/gestor-tres-pulsos.json`. El cotejo da `total campos
cambiados = 45`, todos en `color` o `fondo`; el analizador no encuentra ninguna diferencia fuera de
esos campos y de `rect`. En el modelo después: `fondo=#2c2c2c` en la raíz, `id/header` con
`fondo=#3c3c3c`, `color=#ededed` en texto y `fondo=#5f5f5f` en botones. La última orden da
`a:theme="@ref/0x7f050000"` (línea 29 de la salida), que sitúa el tema en el manifiesto del APK.
El focal es `TestTemaColoresPropiosDelGestor`.

### Frontera

Los cuatro colores se leen del APK y se vuelcan al modelo del JSON; la barra se eligió por
heurística geométrica, no semántica. Sin afirmar fidelidad cromática ni parecido visual, sin CDP,
gate ni publicación. El gestor no se da por funcionando.

## 382. Iconos y visibilidad del gestor medidos desde el APK

Medido el 2026-11-25, posterior a `§381`. El informe del integrador atribuye la señal a los commits
`44ce67e` (emulador) y `b0ae4e3` (vista), integrados en `0c15357`, y las aserciones de
`studio/prueba.js` a `e9e6862`. En este árbol se verifican `git rev-parse HEAD` →
`0c153572864c81403d8050d5d6673b49dcee6346`, `git -C studio rev-parse HEAD` →
`239ec270def579b387b02de7daf03b7c8735acfc` y la guardia de la regla 14
`git -C studio rev-parse HEAD:apkc-emulador` → `b89691cf734005844ac757d8babe5fcebf693726`. Los tres
SHAs cortos del informe que no son objetos de este árbol (`44ce67e`, `b0ae4e3`, `e9e6862`) se citan
como procedencia, no como medición re-hecha aquí. Sin CDP, gate ni publicación; no se afirma
parecido visual ni que los iconos se vean.

### Comandos y artefactos

```sh
git rev-parse HEAD
git -C studio rev-parse HEAD
git -C studio rev-parse HEAD:apkc-emulador
/home/leonisauro/.cache/apkc-web/fidelidad-iconos/apkc-fidelidad preview --rom /home/leonisauro/.local/share/opencode/worktree/c6f089/docs-registro2/studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > /home/leonisauro/.cache/apkc-web/fidelidad-iconos/repro-registro2.json 2> /home/leonisauro/.cache/apkc-web/fidelidad-iconos/repro-registro2.err
cmp /home/leonisauro/.cache/apkc-web/fidelidad-iconos/repro-registro2.json /home/leonisauro/.cache/apkc-web/fidelidad-iconos/gestor-tres-pulsos.json
/home/leonisauro/.cache/apkc-web/integracion-lote2/apkc-lote2 preview --rom /home/leonisauro/.local/share/opencode/worktree/c6f089/docs-registro2/studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > /home/leonisauro/.cache/apkc-web/integracion-lote2/repro-registro2.json 2> /home/leonisauro/.cache/apkc-web/integracion-lote2/repro-registro2.err
cmp /home/leonisauro/.cache/apkc-web/integracion-lote2/repro-registro2.json /home/leonisauro/.cache/apkc-web/integracion-lote2/gestor-tres-pulsos.json
sha256sum /home/leonisauro/.cache/apkc-web/fidelidad-iconos/gestor-tres-pulsos.json /home/leonisauro/.cache/apkc-web/integracion-lote2/gestor-tres-pulsos.json
```

Los dos `cmp` dan rc 0: las dos reproducciones son byte-idénticas a los artefactos
`fidelidad-iconos/gestor-tres-pulsos.json` (`0ddd412306a235ff68df4f709cfb11ad5662fc389a80142e1a95f169b0cfa67c`)
e `integracion-lote2/gestor-tres-pulsos.json`
(`371f8fa26015bae4355ce7490b5adecabf3b5608fef1f3cbb7900c6f953eb273`). No se dice que ambos JSON
sean iguales entre sí: solo lo son sus campos de interés.

### Campos medidos

```sh
python3 - <<'PY'
import json
p='/home/leonisauro/.cache/apkc-web/fidelidad-iconos/gestor-tres-pulsos.json'
d=json.load(open(p))
def walk(n,acc):
    acc.append(n)
    for h in n.get('hijos',[]): walk(h,acc)
    return acc
ns=walk(d['raiz'],[])
print('nodos', len(ns))
for n in ns:
    if 'visibilidad' in n or 'recurso' in n:
        print({k:n[k] for k in ('id','texto','recurso','visibilidad','visible') if k in n})
PY
```

Salida: `nodos 37`; `{'texto': 'fotos', 'recurso': '0x7f020003'}`,
`{'texto': 'leeme.txt', 'recurso': '0x7f020002'}` y siete nodos con `visibilidad`: `'gone'` —
`id/drive_list`, `id/menu_list`, `id/quick_selection`, `id/share_selected`,
`id/single_select_operation`, `id/select_operation`, `id/paste_operation`. En los dos artefactos,
`ejecucion.pasos` es 3595, `ejecucion.error` es `None`, `layout` es `res/R5.xml` y `nodos` es 37.
Los rótulos `' <- '` y `' ||| '` siguen siendo **texto literal del APK**: `preview_test.go:1213-1218`
falsifica que se conviertan en drawables, igual que `Loading...` y
`cutting/copying/deleting files...`.

### Focales

```sh
tcr go test -run 'TestRecursoDeFilaDelGestorSaleDelArsc|TestAnotaVisibilidadDeLaEjecucion' -v .
node prueba.js
```

Los dos focales dan `--- PASS` en el materializado temporal
(`~/.cache/apkc-web/registro-setadd/apkc/`, construido copiando `*.go`, `go.mod`, `go.sum` y
`template/` de `apkc/` y los 34 `preview*.go` de `studio/apkc-emulador/`, como prescribe
`tools/materializar-emulador.sh:34-44`). `node prueba.js`, corrido en `studio/`, termina con
`prueba: todo en orden` e
imprime avisos de omisión por falta de los APK de fixtures (`APKC_PREVIEW`, `APKC/APK_MENU`,
`APKC/APK_TAREAS`) y un `incierto` sobre el hit-test real, que solo se podría jurar en chromium.

| Dato | Estado y evidencia |
| --- | --- |
| 3595 pasos, `error=None`, `layout=res/R5.xml`, 37 nodos en los dos artefactos | CONFIRMADO; `cmp` rc 0 y el `python3` de arriba |
| `fotos` → `0x7f020003`, `leeme.txt` → `0x7f020002`, siete paneles `visibilidad: gone` | CONFIRMADO; salida del `python3` de arriba |
| Focales y `node prueba.js` | CONFIRMADO; `--- PASS` y `prueba: todo en orden` |
| Ocho llamadas a `View.setVisibility` con valor 8 | incierto: es cifra del informe del integrador; en el JSON no hay campo que la reproduzca y `grep -n setVisibility /home/leonisauro/.cache/apkc-web/sfm-dexdump.txt` da siete líneas estáticas. No re-medida aquí. |

**Frontera.** Se mide el modelo emitido por `apkc preview`: recurso de las dos filas y visibilidad de
siete paneles. Sin CDP no se afirma que los iconos se vean ni que el resultado sea parecido al APK.

## 383. Vistas transitorias del gestor descartadas por estado medido

Medido el 2026-11-25, posterior a `§382`. El informe del integrador cita los commits `a086bf6` y
`dc8a5cc` (emulador) y `da447ec` y `5af0d12` (vista), integrados en `0c15357`. En este árbol se
verifican los objetos `a086bf6` y `dc8a5cc` en `studio/apkc-emulador` y `da447ec`, `5af0d12` en
`studio`; el pin de la regla 14 es `b89691cf734005844ac757d8babe5fcebf693726`. Sin CDP, gate ni
publicación.

### Comando y artefacto

```sh
/home/leonisauro/.cache/apkc-web/integracion-transitorias/apkc preview --rom /home/leonisauro/.local/share/opencode/worktree/c6f089/docs-registro2/studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 /home/leonisauro/Downloads/com.martinmimigames.simplefileexplorer_20.apk > /home/leonisauro/.cache/apkc-web/integracion-transitorias/repro-registro2.json 2> /home/leonisauro/.cache/apkc-web/integracion-transitorias/repro-registro2.err
cmp /home/leonisauro/.cache/apkc-web/integracion-transitorias/repro-registro2.json /home/leonisauro/.cache/apkc-web/integracion-transitorias/gestor-tres-pulsos.json
python3 /home/leonisauro/.cache/apkc-web/integracion-transitorias/verificar.py /home/leonisauro/.cache/apkc-web/integracion-transitorias/gestor-tres-pulsos.json
sha256sum /home/leonisauro/.cache/apkc-web/integracion-transitorias/gestor-tres-pulsos.json
```

El `cmp` da rc 0 (SHA `7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f`). El
`verificar.py` imprime `nodos=32 pasos=3595 error=None layout='res/R5.xml' interacciones=3`,
`id_list=1`, las dos filas con `recurso='0x7f020003'` y `'0x7f020002'`, y
`transitorios_en_nodos=0 transitorios_en_capturas=6`.

| Corrida | Nodos | `id/list` | `ejecucion` |
| --- | --- | --- | --- |
| Antes (`§382`) | 37 | 7 hijos: `Loading...`, `cutting/copying/deleting files...`, tres vacíos, `fotos`, `leeme.txt` | 3595 pasos, `error=None` |
| Después | **32** | 2 hijos: `fotos`, `leeme.txt` | 3595 pasos, `error=None`, `res/R5.xml` |

El recuento de los 32 nodos lleva `visible` presente en los 32; los siete paneles siguen con
`visibilidad: gone`. Las seis cadenas transitorias **siguen en `ejecucion.capturas`**: son el registro
de `TextView.setText`, no el árbol. No se borra cadena ninguna por su literal.

### Focales y mecanismo

```sh
tcr go test -run 'TestVisibilidadMedidaDeVistaFalsaLosTransitorios|TestCampoVisibleEnElJSONDelNodo|TestRegistraRetiradasSobreviveAlRefresh' -v .
```

Los tres dan `--- PASS` (`ok github.com/ajinasokan/apkc 0.045s` en la corrida conjunta con los
demás focales). `preview_test.go:1251-1276` fija la señal: retirada y `setVisibility` son estado
medido; una vista legítima con el **mismo** texto `Loading...` se publica, y una puesta `gone` se
descarta. `preview_test.go:1280-1298` comprueba que el campo `visible` viaja en el JSON del nodo, y
`preview_test.go:1302-1314` que la retirada del id sintético `0x7ffffffe` sobrevive a una recreación
de la vista. El mecanismo es `visible=false` propagado desde `View.setVisibility` y `removeAllViews`,
no un filtro de cadenas: las claves de búsqueda de `verificar.py:26-29` son `Loading...`, `cutting` y
`copying`.

**Frontera.** Se afirma el recuento del modelo y la procedencia de la señal. Sin CDP no se dice que la
lista dibujada muestre dos filas ni que la navegación esté terminada.

## 384. `Set.add` sin duplicar elementos

Medido el 2026-11-25, posterior a `§383`. El informe del integrador cita el commit `4b0af9e`
(emulador), integrado en `ae9347d`; ambos son objetos verificables en este árbol (`git cat-file -t`
da `commit`). El padre es `0c153572864c81403d8050d5d6673b49dcee6346`. Sin CDP, gate ni publicación.

### Base byte-idéntica y campos sin cambio

```sh
sha256sum /home/leonisauro/.cache/apkc-web/setadd-general/gestor-tres-pulsos.json /home/leonisauro/.cache/apkc-web/integracion-general-f0/gestor-tres-pulsos.json /home/leonisauro/.cache/apkc-web/termux-audio/gestor-tres-pulsos.json
cmp /home/leonisauro/.cache/apkc-web/setadd-general/gestor-tres-pulsos.json /home/leonisauro/.cache/apkc-web/integracion-general-f0/gestor-tres-pulsos.json
python3 /home/leonisauro/.cache/apkc-web/integracion-tema-setadd/compara.py /home/leonisauro/.cache/apkc-web/integracion-tema-setadd/despues-tema /home/leonisauro/.cache/apkc-web/integracion-tema-setadd/despues-setadd
```

Los tres ficheros comparten
`6c9ca8e2d56165be4d6820b6423d5c2d0eeedf12e7302854f3aefa57dd6d9bd7`; los dos `cmp` contra la base
(`integracion-general-f0/` y `termux-audio/`) dan rc 0. `compara.py` dice `IGUAL` para los cinco
JSON de `despues-tema` → `despues-setadd` y `total campos cambiados = 0`; el gestor de
`despues-setadd` tiene 3595 pasos, 37 nodos, `error=None`, `layout=res/R5.xml`. En
`setadd-general/termux.json`, `avisos-recortados: 41` aparece como aviso recortado: no se transcriben
los avisos que faltan.

### Falsación focal

```sh
go test -run TestSetAddDevuelveTrueSoloSiEsNuevo -v .
```

Ejecutado en el materializado temporal `~/.cache/apkc-web/registro-setadd/apkc/`, da:

```
=== RUN   TestSetAddDevuelveTrueSoloSiEsNuevo
--- PASS: TestSetAddDevuelveTrueSoloSiEsNuevo (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.004s
```

`previewstubs_test.go:1013-1016` fija los tres casos sobre `add`: `type` → 1, `type` → 0,
`otro` → 1, con tamaños 1, 1 y 2; es decir `1,0,1`. El mismo test comprueba después que `contains`
lee el mismo respaldo (`previewstubs_test.go:1029-1040`).

| Dato | Estado y evidencia |
| --- | --- |
| 3595 pasos, 37 nodos, `error=None`, cero campos cambiados | CONFIRMADO; `compara.py` y `sha256sum` de arriba |
| `add` → `1,0,1` | CONFIRMADO; `--- PASS` y `previewstubs_test.go:1013-1016` |
| `Set.add` eliminó el error `already registered ... NavKey` | incierto: sin sonda posterior; `~/.cache/apkc-web/condiciones-documentales.md:270` dice «no medido en esta sesión» |
| Delta 6950/7072 pasos en Termux (`setadd-general/` frente a `despues-setadd/`) | medido en ambos JSON; divergencia entre frentes **sin causa atribuida** |

**Frontera.** Se mide la semántica del stub y la ausencia de cambio en los JSON del lote. No se afirma
que `Set.add` arreglara `NavKey` ni que Termux funcione: los dos frentes terminan en errores distintos
y sin CDP no se da por cerrada la app.

## 385. ViewTreeObserver y builders de audio para Termux

Medido el 2026-11-25 sobre el árbol integrado en
`0c153572864c81403d8050d5d6673b49dcee6346`: las mediciones de 7072/7126/7130/7418 se hicieron
sobre el árbol del frente; los cuatro stubs quedaron **ausentes** del pin `8156934e…` y se integraron
después sobre `b7854d6…`, con verificación sobre el blob. Los cuatro commits auditados del submódulo
`studio/apkc-emulador` son `76c60fb60b611cf7b3d8237f13a2240452249768` y
`18baa472efeefbaa56496f278b6f1b8a93a92b2d` («ViewTreeObserver inerte para Termux»),
`dcf10d01e303fb4ea7d6ffc8e59b69042a4049ff` («AudioAttributes.Builder inerte para Termux»),
`fe5363691f1de796fd9dc2231342bcb231790879` («SoundPool.Builder inerte para Termux») y
`1eae1669aacdf4f9c0220437888aaffad5b6f715` («AccessibilityManager inerte para Termux»).
Sin gate, CDP, red, `git push` ni publicación.

### Pasos y retirada de los tres métodos del error

```sh
python3 -c "
import json
r=[('base','/home/leonisauro/.cache/apkc-web/termux-viewtree/base-termux.json'),('viewtree','/home/leonisauro/.cache/apkc-web/termux-viewtree/termux.json'),('audio','/home/leonisauro/.cache/apkc-web/termux-audio/termux.json'),('soundpool','/home/leonisauro/.cache/apkc-web/termux-soundpool/termux.json'),('accesibilidad','/home/leonisauro/.cache/apkc-web/termux-accesibilidad/termux-despues.json')]
for n,p in r:
 e=json.load(open(p))['ejecucion']; s=e.get('error','')
 print(n,e.get('pasos'),'addOnGlobalLayoutListener=', 'addOnGlobalLayoutListener' in s,'setUsage=', 'setUsage' in s,'setAudioAttributes=', 'setAudioAttributes' in s)
"
```
```
base 7072 addOnGlobalLayoutListener= True setUsage= False setAudioAttributes= False
viewtree 7126 addOnGlobalLayoutListener= False setUsage= True setAudioAttributes= False
audio 7130 addOnGlobalLayoutListener= False setUsage= False setAudioAttributes= True
soundpool 7418 addOnGlobalLayoutListener= False setUsage= False setAudioAttributes= False
accesibilidad 7418 addOnGlobalLayoutListener= False setUsage= False setAudioAttributes= False
```

Los errores que se retiran, literales y en el mismo orden:

```sh
python3 -c "
import json
for n,p in [('base','/home/leonisauro/.cache/apkc-web/termux-viewtree/base-termux.json'),('viewtree','/home/leonisauro/.cache/apkc-web/termux-viewtree/termux.json'),('audio','/home/leonisauro/.cache/apkc-web/termux-audio/termux.json')]:
 print(n,json.load(open(p))['ejecucion'].get('error',''))
"
```
```
base en com.termux.app.TermuxActivity.addTermuxActivityRootViewGlobalLayoutListener: se llamó a android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo
viewtree en com.termux.app.terminal.TermuxTerminalSessionClient.onResume: en com.termux.app.terminal.TermuxTerminalSessionClient.getBellSoundPool: se llamó a android.media.AudioAttributes$Builder.setUsage sobre un objeto nulo
audio en com.termux.app.terminal.TermuxTerminalSessionClient.onResume: en com.termux.app.terminal.TermuxTerminalSessionClient.getBellSoundPool: se llamó a android.media.SoundPool$Builder.setAudioAttributes sobre un objeto nulo
```

`ejecucion.error` queda vacío en `soundpool` y en `accesibilidad`. La cuenta es
7072 → 7126 → 7130 → 7418 → 7418. El contrato de las cinco corridas y los avisos, que están
recortados y por eso no se citan como visibles:

```sh
python3 -c "
import json
r=[('base','/home/leonisauro/.cache/apkc-web/termux-viewtree/base-termux.json'),('viewtree','/home/leonisauro/.cache/apkc-web/termux-viewtree/termux.json'),('audio','/home/leonisauro/.cache/apkc-web/termux-audio/termux.json'),('soundpool','/home/leonisauro/.cache/apkc-web/termux-soundpool/termux.json'),('accesibilidad','/home/leonisauro/.cache/apkc-web/termux-accesibilidad/termux-despues.json')]
for n,p in r:
 d=json.load(open(p))
 print(n,'layout=',d.get('layout'),'nodos=',d.get('nodos'),'aproximado=',d.get('aproximado'),[a for a in d['ejecucion']['avisos'] if a.startswith('avisos-recortados')])
"
```
```
base layout= res/layout/activity_termux.xml nodos= 13 aproximado= True ['avisos-recortados: 42']
viewtree layout= res/layout/activity_termux.xml nodos= 13 aproximado= True ['avisos-recortados: 47']
audio layout= res/layout/activity_termux.xml nodos= 13 aproximado= True ['avisos-recortados: 49']
soundpool layout= res/layout/activity_termux.xml nodos= 13 aproximado= True ['avisos-recortados: 54']
accesibilidad layout= res/layout/activity_termux.xml nodos= 13 aproximado= True ['avisos-recortados: 54']
```

### El nodo `TerminalView`

El nodo `com.termux.view.TerminalView` deja de traer una frontera y pasa a una operación de
color. La cuenta de fronteras en el dibujo y su contenido:

```sh
python3 -c "
import json
for n,p in [('base','/home/leonisauro/.cache/apkc-web/termux-viewtree/base-termux.json'),('accesibilidad','/home/leonisauro/.cache/apkc-web/termux-accesibilidad/termux-despues.json')]:
 d=json.load(open(p)); fs=[]; ops=[]
 def walk(x):
  if isinstance(x,dict):
   if 'dibujo' in x:
    if 'frontera' in x['dibujo']: fs.append(x['dibujo']['frontera'])
    if 'operaciones' in x['dibujo']: ops.append(x['dibujo']['operaciones'])
   for v in x.values(): walk(v)
  elif isinstance(x,list):
   for v in x: walk(v)
 walk(d['raiz']); print(n,'fronteras=',len(fs),'operaciones=',ops); print(' ',fs)
"
```
```
base fronteras= 1 operaciones= []
  ['no se pudo construir la vista com.termux.view.TerminalView: se llamó a android.view.accessibility.AccessibilityManager.isEnabled sobre un objeto nulo']
accesibilidad fronteras= 0 operaciones= [[{'operacion': 'color', 'color': '#000000'}]]
  []
```

El color no es inventado: sale del `onDraw` del dex, en `classes14.dump:21712`, que se cita con
su comando:

```sh
sed -n '21712p' /home/leonisauro/.cache/apkc-web/termux-accesibilidad/classes14.dump
```
```
0141dc: 1500 00ff                              |0004: const/high16 v0, #int -16777216 // #ff00
```

### Tests focales

Los quince tests `TestViewTreeObserver*`, `TestAudioAttributes*`, `TestSoundPool*` y
`TestAccesibilidad*` están en `termux-accesibilidad/studio/apkc-emulador/previewstubsframework_test.go`
y se ejecutaron desde `<wt>/apkc`:

```sh
GOTMPDIR=/home/leonisauro/.cache/apkc-web/termux-accesibilidad/gotmp PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH /home/leonisauro/.local/bin/tcr go test -count=1 -v -run '^(TestViewTreeObserverNoNuloLigadoALaVista|TestViewTreeObserverListenerInerteConAviso|TestViewTreeObserverReceptorNuloSigueCortando|TestAudioAttributesBuilderNoNulo|TestAudioAttributesSetterInerteConAviso|TestAudioAttributesReceptorNuloSigueCortando|TestAudioAttributesBuildNoNulo|TestSoundPoolBuilderNoNulo|TestSoundPoolSetterInerteConAviso|TestSoundPoolReceptorNuloSigueCortando|TestSoundPoolBuildNoNulo|TestAccesibilidadServicioNoNulo|TestAccesibilidadIsEnabledCero|TestAccesibilidadOtroServicioCaeAlCero|TestAccesibilidadReceptorNuloSigueCortando)$' . > /home/leonisauro/.cache/apkc-web/termux-accesibilidad/tests-focales.out 2>&1
```
```
rc=0
15
--- PASS: TestAccesibilidadReceptorNuloSigueCortando (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.005s
```

**Frontera.** No se afirma que Termux arranque ni que haya audio. Lo medido es que cuatro
métodos desaparecen del error y que el dibujo del nodo deja de tener frontera; los avisos están
recortados (47, 49 y 54) y no se citan como visibles. Sin CDP, gate ni publicación.

## 386. Observadores vacíos y fusión de `getDeclaredMethods`

Medido el 2026-11-25 sobre los artefactos de
`~/.cache/apkc-web/integracion-fusion/despues/`. Los commits auditados del submódulo
`studio/apkc-emulador` son `7c5ae7042f03acb94ca9fac5e805e99190aef511` y
`ebec52877e863583958a8cb5bd935dce75330a03` (arreglos vacíos),
`8db2ce405979c495035b8eedc1101137a2e2fdeb` y
`74d2ab64bb37f534c3a8da5f259619f05325ecfb` (`forName` y `getDeclaredMethods`), y la fusión
`5c78985931eeb3c84b10f0f5a4946f5cc2a23585` y
`aaa9bf2fbb0978446e8878e9af22bab1d63e9de1` (`getDeclaredMethods` del observador y la
reflexión). Sin gate, CDP, red, `git push` ni publicación.

### Pasos de las tres APK

```sh
python3 -c "
import json
pares=[('fdroid','/home/leonisauro/.cache/apkc-web/integracion-lote2/fdroid.json','/home/leonisauro/.cache/apkc-web/integracion-fusion/despues/fdroid.json'),('librememorygame','/home/leonisauro/.cache/apkc-web/integracion-lote2/librememorygame.json','/home/leonisauro/.cache/apkc-web/integracion-fusion/despues/librememorygame.json'),('newpipe','/home/leonisauro/.cache/apkc-web/integracion-lote2/newpipe.json','/home/leonisauro/.cache/apkc-web/integracion-fusion/despues/newpipe.json')]
for n,a,b in pares:
 da=json.load(open(a)); db=json.load(open(b))
 print(n,'antes',da['ejecucion']['pasos'],'despues',db['ejecucion']['pasos'],'layout',repr(db.get('layout')),'nodos',db.get('nodos'))
"
```
```
fdroid antes 758 despues 2656 layout '' nodos 1
librememorygame antes 1688 despues 7229 layout '' nodos 1
newpipe antes 517 despues 1472 layout 'res/v9.xml' nodos 6
```

Los errores completos de F-Droid y Godot están en
`~/.cache/apkc-web/condiciones-documentales.md:15` y `:25`; sus longitudes y el cotejo de las
dos frases se reproducen con:

```sh
python3 -c "
import json
for n in ('fdroid','librememorygame'):
 p='/home/leonisauro/.cache/apkc-web/integracion-fusion/despues/'+n+'.json'
 e=json.load(open(p))['ejecucion']['error']
 print(n,'len',len(e)); print(e)
"
```
```
fdroid len 457
en org.fdroid.Hilt_MainActivity.<init>: en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.SavedStateHandleSupport.enableSavedStateHandles: en androidx.savedstate.SavedStateRegistry.getSavedStateProvider: en androidx.savedstate.internal.SavedStateRegistryImpl.getSavedStateProvider: se llamó a java.util.Set.iterator sobre un objeto nulo
librememorygame len 312
en org.godotengine.godot.GodotActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.SavedStateHandleSupport.enableSavedStateHandles: se llamó a androidx.savedstate.SavedStateRegistry.registerSavedStateProvider sobre un objeto nulo
```

### La progresión de NewPipe

```sh
python3 -c "
import json
for k,p in [('antes-lote2','/home/leonisauro/.cache/apkc-web/integracion-lote2/newpipe.json'),('arrays-post-previo','/home/leonisauro/.cache/apkc-web/newpipe-arrays/post/newpipe.json'),('fusion','/home/leonisauro/.cache/apkc-web/integracion-fusion/despues/newpipe.json')]:
 d=json.load(open(p)); print('===',k,d['ejecucion'].get('pasos')); print(d['ejecucion'].get('error',''))
"
```
```
=== antes-lote2 517
en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.LifecycleRegistry.addObserver: en androidx.lifecycle.DefaultLifecycleObserverAdapter.<init>: en androidx.lifecycle.ClassesInfoCache.createInfo: array-length sobre un registro que no es arreglo
=== arrays-post-previo 981
en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.LifecycleRegistry.addObserver: en androidx.collection.MutableScatterMap.set: en androidx.collection.MutableScatterMap.findInsertIndex: el registro guarda *main.objetoDalvik y se esperaba un entero
=== fusion 1472
en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.savedstate.internal.SavedStateRegistryImpl.performAttach: en androidx.work.Worker$$ExternalSyntheticLambda0.invoke: se llamó a androidx.lifecycle.Lifecycle.addObserver sobre un objeto nulo
```

El paso intermedio de 981 es una medición previa (`newpipe-arrays/post/newpipe.json`), no una
medición propia de este apartado; el avance que lo produjo se atribuye ya al commit
`d722e4731d651d8e0a26feab4b3ad5df830bcddb` (`and-int/lit8` de un byte). La cifra propia de
este apartado es 1472 pasos y la frontera `Lifecycle.addObserver sobre un objeto nulo`.

### Tests y fusión

Los siete tests de los tres frentes se listan en
`~/.cache/apkc-web/condiciones-documentales.md:68` y se ejecutaron allí mismo; el focal de
arreglos vacíos es `TestClasesReflexionDevuelvenArregloVacio`, con sus dos subtests `getInterfaces`
y `getDeclaredMethods`. El volcado de la fusión conserva 36 líneas `--- PASS`:

```sh
grep -c -- '--- PASS' /home/leonisauro/.cache/apkc-web/fdroid-lifecycle/tests-focales.txt
```
```
36
```

Tras la fusión, cada método tiene un solo `case`:

```sh
f=/home/leonisauro/.local/share/opencode/worktree/c6f089/docs-registro2/studio/apkc-emulador/previewinterprete.go; grep -c 'case "getDeclaredMethods":' "$f"; grep -c 'case "getInterfaces":' "$f"
```
```
1
1
```

**Frontera.** No se afirma que F-Droid, Godot ni NewPipe funcionen: las tres siguen cortando
en una llamada sobre un objeto nulo. Sin gate, CDP ni publicación.

## 387. `getLifecycle` del registry existente para NewPipe

Medido el 2026-11-25. El commit auditado es
`ae218b592f4ccef22587a1eefa9d433572267646` («getLifecycle del registry existente para
NewPipe»); el informe de aprobación da 26 PASS con la suite desde `<wt>/apkc`, cifra
transmitida que no se re-mide aquí. La integración sigue en curso en el checkout principal:
el pin final se fija al cerrar. Sin gate, CDP, red, `git push` ni publicación.

### Pasos y cambio de frontera

```sh
python3 -c "
import json
for n,p in [('fusion','/home/leonisauro/.cache/apkc-web/integracion-fusion/despues/newpipe.json'),('getlifecycle','/home/leonisauro/.cache/apkc-web/newpipe-getlifecycle/newpipe.json'),('lote3','/home/leonisauro/.cache/apkc-web/integracion-lote3/newpipe.json')]:
 d=json.load(open(p)); e=d['ejecucion']['error']
 print(n,d['ejecucion']['pasos'],repr(d.get('layout')),d.get('nodos'),'addObserver=', 'Lifecycle.addObserver' in e,'getSavedStateProvider=', 'getSavedStateProvider' in e)
 print(' ',e)
"
```
```
fusion 1472 'res/v9.xml' 6 addObserver= True getSavedStateProvider= False
  en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.savedstate.internal.SavedStateRegistryImpl.performAttach: en androidx.work.Worker$$ExternalSyntheticLambda0.invoke: se llamó a androidx.lifecycle.Lifecycle.addObserver sobre un objeto nulo
getlifecycle 2151 'res/v9.xml' 6 addObserver= False getSavedStateProvider= True
  en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.ViewModelKt.enableSavedStateHandles: se llamó a androidx.savedstate.SavedStateRegistry.getSavedStateProvider sobre un objeto nulo
lote3 2151 'res/v9.xml' 6 addObserver= False getSavedStateProvider= True
  en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.ViewModelKt.enableSavedStateHandles: se llamó a androidx.savedstate.SavedStateRegistry.getSavedStateProvider sobre un objeto nulo
```

NewPipe pasa de 1472 a 2151 pasos, con `layout=res/v9.xml` y 6 nodos. Desaparece
`addObserver sobre un objeto nulo` y aparece una frontera nueva:
`SavedStateRegistry.getSavedStateProvider sobre un objeto nulo`, desde
`ViewModelKt.enableSavedStateHandles`.

### El stub no sintetiza observadores

El cambio devuelve la referencia que el propio dex guardó en el campo de la actividad; no crea
un `LifecycleRegistry` ni un observador nuevos. Se comprueba en el diff del commit:

```sh
git -C /home/leonisauro/.local/share/opencode/worktree/c6f089/docs-registro2/studio/apkc-emulador show ae218b5 -- previewinterprete.go | grep -A 12 '^+	case "getLifecycle"'
```
```
+	case "getLifecycle":
+		// `LifecycleOwner.getLifecycle()` caía al cero del tipo: el invoke se declara en la interfaz
+		// (o en `SavedStateRegistryOwner` tras el `check-cast` que sobrescribe la clase concreta) y
+		// el receptor es la actividad. Ésa ya tiene en su campo el `LifecycleRegistry` que el propio
+		// dex construyó, así que se devuelve ESA referencia, sin fabricar un objeto ni un
+		// observador: el `addObserver` posterior recibe el registry real. Aviso explícito de
+		// aproximación porque la vista previa no ejecuta el getter del framework. Sin el campo el
+		// stub no casa y la cadena corta en el guardián de receptor nulo de `invoca`.
+		if len(vals) == 0 && obj != nil &&
+			es("Landroidx/lifecycle/LifecycleOwner;", "Landroidx/savedstate/SavedStateRegistryOwner;") {
+			if registry := obj.campos["Landroidx/core/app/ComponentActivity;.lifecycleRegistry"]; registry != nil {
```

### Tests focales

Los tres tests del commit se reejecutaron desde `newpipe-getlifecycle/apkc`:

```sh
GOTMPDIR=/home/leonisauro/.cache/apkc-web/newpipe-getlifecycle/gotmp PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH /home/leonisauro/.local/bin/tcr go test -count=1 -v -run '^(TestGetLifecycleDevuelveElRegistryDelDex|TestGetLifecycleConStubsApagadosReapareceElError|TestGetLifecycleReceptorSinCampoSigueCortando)$' . > /home/leonisauro/.cache/apkc-web/newpipe-getlifecycle/tests-getlifecycle.out 2>&1
```
```
rc=0
3
--- PASS: TestGetLifecycleReceptorSinCampoSigueCortando (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.004s
```

**Frontera.** No se afirma que NewPipe funcione: la corrida corta en una frontera nueva,
`SavedStateRegistry.getSavedStateProvider sobre un objeto nulo`. El pin final queda pendiente
de la integración en curso. Sin CDP, gate ni publicación.

## 388. Registry de saved state del dex para F-Droid, Godot y NewPipe

Medido el 2026-11-25 sobre los artefactos de
`~/.cache/apkc-web/integracion-savedstate/` y `~/.cache/apkc-web/savedstate-registry/`, con las
cinco APK reales de `~/Downloads/`. El commit auditado es
`31ce4566eb0d86d941830f7bdb5ad34b94ff1861` (informe APROBADO), integrado en el submódulo como
`996ca07ea35735234713faaa4fc9ee8388720d02`; pin `008394a…`, padre `83eddb4…`, guardia
`e4eff86…`. Suite `194.716s` y 41 focales PASS, cifras de la corrida de integración transmitidas
y no re-medidas aquí. Sin gate, CDP, red, `git push` ni publicación.

### Cómo se reproducen las cinco corridas

```sh
O=/home/leonisauro/.cache/apkc-web/integracion-savedstate
D=/home/leonisauro/Downloads
$O/apkc preview $D/F-Droid.apk > $O/fdroid.json 2> $O/fdroid.err
$O/apkc preview $D/org.quentin_bettoum.librememorygame_9.apk > $O/godot.json 2> $O/godot.err
$O/apkc preview $D/org.schabi.newpipe_1017.apk > $O/newpipe.json 2> $O/newpipe.err
$O/apkc preview $D/termux-app_v0.118.3+github-debug_armeabi-v7a.apk > $O/termux.json 2> $O/termux.err
$O/apkc preview --rom studio/apkc-emulador/rom.json --permiso android.permission.READ_EXTERNAL_STORAGE --pulsar id/menu_quick_switch --pulsar id/drive_list:0 --pulsar id/list:4 $D/com.martinmimigames.simplefileexplorer_20.apk > $O/gestor-tres-pulsos.json 2> $O/gestor-tres-pulsos.err
```

### Pasos y fronteras

```sh
python3 -c "
import json
a='/home/leonisauro/.cache/apkc-web/savedstate-registry/'
b='/home/leonisauro/.cache/apkc-web/integracion-savedstate/'
for n in ('fdroid','godot','newpipe','termux','gestor-tres-pulsos'):
 x=json.load(open(a+'antes-'+n+'.json')); y=json.load(open(b+n+'.json'))
 print(n, x['ejecucion']['pasos'], '->', y['ejecucion']['pasos'], repr(y.get('layout')), y.get('nodos'), 'aproximado=', y.get('aproximado'))
 print('  antes:', x['ejecucion'].get('error'))
 print('  despues:', y['ejecucion'].get('error'))
"
```
```
fdroid 2656 -> 7272 '' 1 aproximado= True
  antes: en org.fdroid.Hilt_MainActivity.<init>: en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.SavedStateHandleSupport.enableSavedStateHandles: en androidx.savedstate.SavedStateRegistry.getSavedStateProvider: en androidx.savedstate.internal.SavedStateRegistryImpl.getSavedStateProvider: se llamó a java.util.Set.iterator sobre un objeto nulo
  despues: en mu.KotlinLogging.logger: en org.slf4j.LoggerFactory.getLogger: en org.slf4j.LoggerFactory.getILoggerFactory: en org.slf4j.LoggerFactory.getProvider: en org.slf4j.LoggerFactory.performInitialization: en org.slf4j.LoggerFactory.bind: en org.slf4j.LoggerFactory.findServiceProviders: se llamó a java.util.ServiceLoader.iterator sobre un objeto nulo
godot 7229 -> 7279 '' 1 aproximado= True
  antes: en org.godotengine.godot.GodotActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.SavedStateHandleSupport.enableSavedStateHandles: se llamó a androidx.savedstate.SavedStateRegistry.registerSavedStateProvider sobre un objeto nulo
  despues: en org.godotengine.godot.GodotActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.SavedStateHandleSupport.enableSavedStateHandles: se llamó a androidx.lifecycle.Lifecycle.addObserver sobre un objeto nulo
newpipe 2151 -> 3037 'res/v9.xml' 6 aproximado= True
  antes: en androidx.appcompat.app.AppCompatActivity.<init>: en androidx.fragment.app.FragmentActivity.<init>: en androidx.activity.ComponentActivity.<init>: en androidx.lifecycle.ViewModelKt.enableSavedStateHandles: se llamó a androidx.savedstate.SavedStateRegistry.getSavedStateProvider sobre un objeto nulo
  despues: en androidx.core.os.LocaleListCompat.forLanguageTags: array-length sobre un registro que no es arreglo
termux 7072 -> 7072 'res/layout/activity_termux.xml' 13 aproximado= True
  antes: en com.termux.app.TermuxActivity.addTermuxActivityRootViewGlobalLayoutListener: se llamó a android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo
  despues: en com.termux.app.TermuxActivity.addTermuxActivityRootViewGlobalLayoutListener: se llamó a android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo
gestor-tres-pulsos 3595 -> 3595 'res/R5.xml' 32 aproximado= True
  antes: None
  despues: None
```

F-Droid pasa de 2656 a 7272 pasos y corta en `ServiceLoader.iterator` sobre un objeto nulo,
desde `LoggerFactory.findServiceProviders`; Godot de 7229 a 7279 con `Lifecycle.addObserver`
sobre un objeto nulo; NewPipe de 2151 a 3037 con `LocaleListCompat.forLanguageTags`:
`array-length` sobre un registro que no es arreglo. Los tres dan `aproximado=true`; F-Droid y
Godot tienen `layout=''` y 1 nodo, y NewPipe `res/v9.xml` y 6 nodos. Termux sigue en 7072 y el
gestor en 3595 pasos y 32 nodos, sin error.

### Cadenas que desaparecen

```sh
python3 -c "
import json
a='/home/leonisauro/.cache/apkc-web/savedstate-registry/'
b='/home/leonisauro/.cache/apkc-web/integracion-savedstate/'
for n in ('fdroid','godot','newpipe'):
 x=json.dumps(json.load(open(a+'antes-'+n+'.json')),ensure_ascii=False)
 y=json.dumps(json.load(open(b+n+'.json')),ensure_ascii=False)
 for c in ('getSavedStateProvider','registerSavedStateProvider','Set.iterator','sin cuerpo','Ljava/util/Set;'):
  print(n,c,'antes=',c in x,'despues=',c in y)
"
```
```
fdroid getSavedStateProvider antes= True despues= False
fdroid registerSavedStateProvider antes= False despues= False
fdroid Set.iterator antes= True despues= False
fdroid sin cuerpo antes= False despues= False
fdroid Ljava/util/Set; antes= True despues= False
godot getSavedStateProvider antes= False despues= False
godot registerSavedStateProvider antes= True despues= False
godot Set.iterator antes= False despues= False
godot sin cuerpo antes= False despues= False
godot Ljava/util/Set; antes= False despues= False
newpipe getSavedStateProvider antes= True despues= False
newpipe registerSavedStateProvider antes= False despues= False
newpipe Set.iterator antes= False despues= False
newpipe sin cuerpo antes= False despues= False
newpipe Ljava/util/Set; antes= False despues= False
```

Desaparecen `getSavedStateProvider`, `registerSavedStateProvider`, `Set.iterator sobre un objeto
nulo`, `getSavedStateRegistry no tiene cuerpo` y `entrySet … Ljava/util/Set;`. El cotejo sobre
los JSON confirma las cuatro cadenas que en ellos aparecen (`getSavedStateProvider`,
`registerSavedStateProvider`, `Set.iterator` y `Ljava/util/Set;`);
`getSavedStateRegistry no tiene cuerpo` se recoge como procedencia de la auditoría, no re-medida
aquí.

### Atribución: el getter y el mapa, no la rama del dex

El avance se atribuye al getter `getSavedStateRegistry` y a `datosDelMapa` aceptando
`LinkedHashMap`, en `previewinterprete.go:4706-4713` y `:7167-7183` del submódulo; se comprueba
con:

```sh
sed -n '4706,4713p;7170,7183p' studio/apkc-emulador/previewinterprete.go
```
```
	if datos, ok := obj.campos["mapa.datos"].(map[string]any); ok {
		return datos
	}
	// `SavedStateRegistryImpl.<init>` guarda un LinkedHashMap en `keyToProviders` (confirmado con
	// dexdump sobre los dex de F-Droid y NewPipe). Sin reconocerlo, `entrySet` caía al cero de
	// Ljava/util/Set; y el `Set.iterator` siguiente reventaba. Se modela vacío, sin inventar claves.
	if strings.HasSuffix(obj.clase, "Ljava/util/HashMap;") ||
		strings.HasSuffix(obj.clase, "Ljava/util/LinkedHashMap;") ||
	case "getSavedStateRegistry":
		// `SavedStateRegistryOwner.getSavedStateRegistry()` se declara en el dex sin cuerpo: la
		// actividad ya guarda el controlador que el propio dex construyó, con el
		// `savedStateRegistry` escrito por su `<init>`. Se devuelve ESA referencia, sin fabricar un
		// registry ni un proveedor. El aviso dice que es una aproximación porque la vista previa no
		// ejecuta el getter del framework. Sin el campo el stub no casa y el cero llega al
		// `getSavedStateProvider` siguiente, que corta con receptor nulo.
		if stubsFrameworkActivos && len(vals) == 0 && obj != nil &&
			proto == "()Landroidx/savedstate/SavedStateRegistry;" &&
			es("Landroidx/savedstate/SavedStateRegistryOwner;", "Landroidx/activity/ComponentActivity;") {
			if registry := registryDelOwnerSavedState(obj); registry != nil {
				m.avisa("SavedStateRegistryOwner.getSavedStateRegistry: se devuelve el SavedStateRegistry que el dex construyó en el controlador (aproximación)")
				return registry, true, nil
			}
```

**Condición de atribución (no negociable).** El avance viene del getter `getSavedStateRegistry`
y de `datosDelMapa` aceptando `LinkedHashMap` (`previewinterprete.go:4706-4713`, `:7167-7183`),
**no** de `initSavedStateConDex`: esa rama no se ejercitó en las cinco corridas. La función
`initSavedStateConDex` existe en `previewinterprete.go:4736` y se llama en `:4766` y `:7030`, pero
ninguna de las cinco fronteras medidas pasa por ella.

### Control byte a byte

```sh
cmp /home/leonisauro/.cache/apkc-web/integracion-savedstate/termux.json /home/leonisauro/.cache/apkc-web/integracion-lote3-final/termux.json
cmp /home/leonisauro/.cache/apkc-web/integracion-savedstate/gestor-tres-pulsos.json /home/leonisauro/.cache/apkc-web/integracion-lote3-final/gestor-tres-pulsos.json
```
```
rc=0 y sin salida en los dos: Termux 7072 y gestor 3595 pasos y 32 nodos, byte-idénticos a la
línea base de `integracion-lote3-final/`.
```

**Frontera.** No se afirma que F-Droid, Godot ni NewPipe funcionen: las tres nuevas fronteras se
reportan sin forzar, con `aproximado=true`, y sin que cambie el `layout` propio de F-Droid y
Godot. Sin gate, CDP, red ni publicación.

## 389. Desreferencia de recursos para NewPipe

Medido el 2026-11-26 sobre los artefactos de
`/home/leonisauro/.cache/apkc-web/newpipe-referencia/` y la APK real
`/home/leonisauro/Downloads/org.schabi.newpipe_1017.apk`. El commit auditado es `4bbfec9`
(«fix(emulador): desreferencia referencias de recursos para NewPipe»), integrado en el checkout
principal por otro frente con `split` ya en su base; el test de profundidad es `5e53ed5`
(`TestReferenciaProfundidadMaximaConservaElError`). Los 21 focales PASS son cifra transmitida y
no se re-mide aquí. Sin gate, CDP, red, `git push` ni publicación.

### Pasos y frontera

```sh
python3 -c "
import json
r='/home/leonisauro/.cache/apkc-web/newpipe-referencia/'
a=json.load(open(r+'antes-alineado-newpipe.json'))
b=json.load(open(r+'final-newpipe.json'))
for n,d in (('antes',a),('despues',b)):
 e=d['ejecucion']['error']
 print(n,d['ejecucion']['pasos'],repr(d.get('layout')),d.get('nodos'),'aproximado=',d.get('aproximado'))
 print(' error:',e)
 print(' getString(2131951849)=', 'getString(2131951849)' in e, 'la tabla no resuelve ese id como una cadena=', 'la tabla no resuelve ese id como una cadena' in e)
"
```
```
antes 3093 'res/v9.xml' 6 aproximado= True
 error: en okio.Options$Companion.getSelectedThemeKey: getString(2131951849): la tabla no resuelve ese id como una cadena
 getString(2131951849)= True la tabla no resuelve ese id como una cadena= True
despues 3476 'res/v9.xml' 6 aproximado= True
 error: en org.schabi.newpipe.util.ServiceHelper.getSelectedServiceId: en org.schabi.newpipe.util.ServiceHelper.getSelectedService: en us.shandian.giga.util.Utility.getService: se llamó a j$.util.stream.Stream.filter sobre un objeto nulo
 getString(2131951849)= False la tabla no resuelve ese id como una cadena= False
```

NewPipe pasa de 3093 a 3476 pasos, con `layout=res/v9.xml` y 6 nodos. Desaparecen
`getString(2131951849)` y `la tabla no resuelve ese id como una cadena`; aparece una frontera
nueva, `Stream.filter sobre un objeto nulo`, desde `ServiceHelper.getSelectedServiceId`, sin
forzar.

### La cadena de recursos, según aapt2, no en los avisos

El id del error se resuelve contra el `resources.arsc` de la APK con `aapt2`:

```sh
/home/leonisauro/opt/android-sdk/build-tools/35.0.1/aapt2 dump resources /home/leonisauro/Downloads/org.schabi.newpipe_1017.apk 2>/dev/null | grep -A 2 -E 'resource 0x7f1300e9 |resource 0x7f130050 '
```
```
    resource 0x7f130050 string/auto_device_theme_key
      () "auto_device_theme"
    resource 0x7f130051 string/auto_device_theme_title
--
    resource 0x7f1300e9 string/default_theme_value
      () @string/auto_device_theme_key
    resource 0x7f1300ea string/default_video_format_key
```

O sea `0x7f1300e9` es `string/default_theme_value`, que apunta a `@string/auto_device_theme_key`,
cuyo texto es `auto_device_theme`. **Esa cadena no aparece en los avisos de ninguna de las dos
corridas**: se documenta como resolución de `aapt2`, no como texto observado en la vista previa.

```sh
python3 -c "
import json
for f in ('antes-alineado-newpipe.json','final-newpipe.json'):
 s=json.dumps(json.load(open('/home/leonisauro/.cache/apkc-web/newpipe-referencia/'+f)),ensure_ascii=False)
 print(f,'auto_device_theme=', 'auto_device_theme' in s)
"
```
```
antes-alineado-newpipe.json auto_device_theme= False
final-newpipe.json auto_device_theme= False
```

### Control byte a byte

```sh
for n in fdroid gestor godot termux; do cmp /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/antes-$n.json /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/final-$n.json; echo "$n rc=$?"; done
```
```
fdroid rc=0
gestor rc=0
godot rc=0
termux rc=0
```

```sh
sha256sum /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/antes-{fdroid,gestor,godot,termux}.json /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/final-{fdroid,gestor,godot,termux}.json
```
```
dd58c20ec1955779f464a683e0fc97caedffd832f61beac099c162098e329adf  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/antes-fdroid.json
7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/antes-gestor.json
c144806b1214750cb2825c13776fa1db0c3ed1fac39d1ec9208ffddb899e9317  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/antes-godot.json
b55eb52ebe5fa66237c16b372cdbf5c668ca9fd6a8b68e801ce0980b30efa925  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/antes-termux.json
dd58c20ec1955779f464a683e0fc97caedffd832f61beac099c162098e329adf  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/final-fdroid.json
7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/final-gestor.json
c144806b1214750cb2825c13776fa1db0c3ed1fac39d1ec9208ffddb899e9317  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/final-godot.json
b55eb52ebe5fa66237c16b372cdbf5c668ca9fd6a8b68e801ce0980b30efa925  /home/leonisauro/.cache/apkc-web/newpipe-referencia/regresion-alineada/final-termux.json
```

Las cuatro parejas comparten `sha256sum`, así que las copias son byte-idénticas a la línea base
alineada.

**Frontera.** No se afirma que NewPipe funcione: la corrida corta en una frontera nueva,
`Stream.filter sobre un objeto nulo`, con `aproximado=true` y sin forzar. El pin final y los 21
focales quedan pendientes del frente que los integra. Sin gate, CDP, red ni publicación.

## 390. Tema sintético de SplashScreen para Godot

Medido el 2026-11-26 sobre los artefactos de
`/home/leonisauro/.cache/apkc-web/godot-tema-splash/` y la APK real
`/home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk`. El commit auditado es
`e633ae75a64d93456a6598c2b432f94379f4f301`, aún no integrado; el pin final queda pendiente.
Cuatro controles byte-idénticos, verificados aquí. Sin gate, CDP, red, `git push` ni publicación.

### Pasos y frontera

```sh
python3 -c "
import json
r='/home/leonisauro/.cache/apkc-web/godot-tema-splash/'
a=json.load(open(r+'antes.json')); b=json.load(open(r+'despues.json'))
for n,d in (('antes',a),('despues',b)):
 e=d['ejecucion']['error']; av=' '.join(d['ejecucion']['avisos'])
 print(n,d['ejecucion']['pasos'],repr(d.get('layout')),d.get('nodos'),'aproximado=',d.get('aproximado'))
 print(' error:',e)
 print(' resolveAttribute=', 'resolveAttribute' in e, 'aviso cero getTheme=', 'no está en el dex ni en los stubs: se devuelve el cero de Landroid/content/res/Resources\$Theme;' in av, 'tema-sintetico=', 'tema-sintetico: Activity.getTheme' in av)
"
```
```
antes 8980 '' 1 aproximado= True
 error: en androidx.core.splashscreen.SplashScreen.installSplashScreen: en androidx.core.splashscreen.SplashScreen$Companion.installSplashScreen: en androidx.core.splashscreen.SplashScreen.access$install: en androidx.core.splashscreen.SplashScreen.install: en androidx.core.splashscreen.SplashScreen$Impl.install: se llamó a android.content.res.Resources$Theme.resolveAttribute sobre un objeto nulo
 resolveAttribute= True aviso cero getTheme= True tema-sintetico= False
despues 9159 '' 1 aproximado= True
 error: en androidx.activity.EdgeToEdge.enable: en androidx.activity.EdgeToEdge.enable$default: en androidx.activity.EdgeToEdge.enable: se llamó a android.view.Window.getDecorView sobre un objeto nulo
 resolveAttribute= False aviso cero getTheme= False tema-sintetico= True
```

Godot pasa de 8980 a 9159 pasos, con `layout=''` y 1 nodo. Desaparece
`Resources$Theme.resolveAttribute sobre un objeto nulo` y el aviso del cero de `getTheme`; aparece
`tema-sintetico: Activity.getTheme entrega un tema enlazado a la tabla del APK (aproximación)`.
La nueva frontera es `Window.getDecorView sobre un objeto nulo`, en `EdgeToEdge.enable`, sin
forzar. El JSON registra `avisos-recortados: 6`: los seis avisos recortados se citan como tales y
no se reconstruye su texto.

### Ids verificados con aapt2 y rastro retirado

El volcado `aapt2` conservado deja escrito el atributo del tema:

```sh
grep -n 'windowSplashScreenBackground(0x7f030015)' /home/leonisauro/.cache/apkc-web/godot-tema-splash/aapt2.txt
```
```
804:        windowSplashScreenBackground(0x7f030015)=?0x01010031
```

El id `0x01010031` es del framework y no tiene entrada propia en la tabla del APK:

```sh
grep -c 'resource 0x01010031 ' /home/leonisauro/.cache/apkc-web/godot-tema-splash/aapt2.txt
```
```
0
```

Por eso `resolveAttribute` devuelve `false` y no se inventa ningún color; el valor `false` es lo
registrado en el informe auditado y no se re-mide aquí. Los ids de la corrida instrumentada están
en el rastro conservado:

```sh
sed -n '1,3p' /home/leonisauro/.cache/apkc-web/godot-tema-splash/debug.err
```
```
DEPURACION resolveAttribute attr 2130903059 estilo 2131492873 recurso 2131361794 datos 106 tipo 3
DEPURACION resolveAttribute attr 2130903057 estilo 2131492873 recurso 2131034144 datos 73729 tipo 5
DEPURACION resolveAttribute attr 2130903054 estilo 2131492873 recurso 2131492872 datos 2131492872 tipo 1
```

Esa instrumentación está retirada del binario final: el rastro es un registro histórico y **no es
re-medible**. Por lo tanto `resolveAttribute` sobre el APK real no se marca CONFIRMADO; solo se
confirma que la cadena del error desaparece en `despues.json`. Las dos focales del tema quedan en
el registro conservado:

```sh
tail -n 6 /home/leonisauro/.cache/apkc-web/godot-tema-splash/focales.log
```
```
=== RUN   TestTemaSplash
--- PASS: TestTemaSplash (0.00s)
=== RUN   TestResolveAttribute
--- PASS: TestResolveAttribute (0.00s)
PASS
ok  	github.com/ajinasokan/apkc	0.043s
```

### Control byte a byte

```sh
for f in fdroid gestor-tres-pulsos newpipe termux; do cmp /home/leonisauro/.cache/apkc-web/godot-tema-splash/$f.json /home/leonisauro/.cache/apkc-web/integracion-savedstate/$f.json; echo "$f rc=$?"; done
```
```
fdroid rc=0
gestor-tres-pulsos rc=0
newpipe rc=0
termux rc=0
```

```sh
sha256sum /home/leonisauro/.cache/apkc-web/godot-tema-splash/{fdroid,gestor-tres-pulsos,newpipe,termux}.json /home/leonisauro/.cache/apkc-web/integracion-savedstate/{fdroid,gestor-tres-pulsos,newpipe,termux}.json
```
```
dd58c20ec1955779f464a683e0fc97caedffd832f61beac099c162098e329adf  /home/leonisauro/.cache/apkc-web/godot-tema-splash/fdroid.json
7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f  /home/leonisauro/.cache/apkc-web/godot-tema-splash/gestor-tres-pulsos.json
e4725ca1a9d61254cdbba48ddb10c91ef92f8545515948d309a53cc8ce60f948  /home/leonisauro/.cache/apkc-web/godot-tema-splash/newpipe.json
b55eb52ebe5fa66237c16b372cdbf5c668ca9fd6a8b68e801ce0980b30efa925  /home/leonisauro/.cache/apkc-web/godot-tema-splash/termux.json
dd58c20ec1955779f464a683e0fc97caedffd832f61beac099c162098e329adf  /home/leonisauro/.cache/apkc-web/integracion-savedstate/fdroid.json
7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f  /home/leonisauro/.cache/apkc-web/integracion-savedstate/gestor-tres-pulsos.json
e4725ca1a9d61254cdbba48ddb10c91ef92f8545515948d309a53cc8ce60f948  /home/leonisauro/.cache/apkc-web/integracion-savedstate/newpipe.json
b55eb52ebe5fa66237c16b372cdbf5c668ca9fd6a8b68e801ce0980b30efa925  /home/leonisauro/.cache/apkc-web/integracion-savedstate/termux.json
```

Las cuatro parejas comparten `sha256sum` con la línea base de `integracion-savedstate/`.

**Frontera.** No se afirma que el juego funcione: los `.so` ARM quedan fuera y las llamadas
`System.load*` son inertes en la vista previa. La corrida corta en una frontera nueva, sin forzar,
y sin CDP no se afirma parecido visual alguno. El pin final queda pendiente de la integración.
Sin gate, CDP, red ni publicación.

## 391. ServiceLoader desde META-INF/services para F-Droid

Medido el 2026-11-26 sobre
`/home/leonisauro/.cache/apkc-web/fdroid-serviceloader/fdroid.json` y la línea base
`/home/leonisauro/.cache/apkc-web/integracion-savedstate/fdroid.json`, con la APK real
`/home/leonisauro/Downloads/F-Droid.apk`. El commit auditado es
`d163a028129150157645776dc09081b24e6cf296`, aún no integrado; el SHA final queda pendiente al
cerrar. Los 17 focales PASS son cifra transmitida y no se re-mide aquí; cuatro controles
byte-idénticos se verifican aquí. Sin gate, CDP, red, `git push` ni publicación.

### Pasos y frontera

```sh
python3 -c "
import json
a='/home/leonisauro/.cache/apkc-web/integracion-savedstate/fdroid.json'
b='/home/leonisauro/.cache/apkc-web/fdroid-serviceloader/fdroid.json'
for n,p in (('antes',a),('despues',b)):
 d=json.load(open(p)); e=d['ejecucion']['error']; av=' '.join(d['ejecucion']['avisos'])
 print(n,d['ejecucion']['pasos'],repr(d.get('layout')),d.get('nodos'),'aproximado=',d.get('aproximado'))
 print(' error:',e)
 print(' ServiceLoader.iterator=', 'ServiceLoader.iterator' in e, 'aviso load cero=', 'no está en el dex ni en los stubs: se devuelve el cero de Ljava/util/ServiceLoader;' in av)
"
```
```
antes 7272 '' 1 aproximado= True
 error: en mu.KotlinLogging.logger: en org.slf4j.LoggerFactory.getLogger: en org.slf4j.LoggerFactory.getILoggerFactory: en org.slf4j.LoggerFactory.getProvider: en org.slf4j.LoggerFactory.performInitialization: en org.slf4j.LoggerFactory.bind: en org.slf4j.LoggerFactory.findServiceProviders: se llamó a java.util.ServiceLoader.iterator sobre un objeto nulo
 ServiceLoader.iterator= True aviso load cero= True
despues 7530 '' 1 aproximado= True
 error: en mu.KotlinLogging.logger: en org.slf4j.LoggerFactory.getLogger: en org.slf4j.LoggerFactory.getILoggerFactory: en org.slf4j.LoggerFactory.getProvider: en org.slf4j.LoggerFactory.performInitialization: en org.slf4j.LoggerFactory.versionSanityCheck: se llamó a java.lang.String.startsWith sobre un objeto nulo
 ServiceLoader.iterator= False aviso load cero= False
```

F-Droid pasa de 7272 a 7530 pasos, con `layout=''` y 1 nodo. Desaparece
`ServiceLoader.iterator sobre un objeto nulo` y el aviso `load … cero`. La nueva frontera es
`String.startsWith sobre un objeto nulo`, en `versionSanityCheck`, sin forzar.

### El proveedor sale del zip, no se fabrica

El aviso de la corrida dice qué se entregó:

```sh
python3 -c "
import json
d=json.load(open('/home/leonisauro/.cache/apkc-web/fdroid-serviceloader/fdroid.json'))
for a in d['ejecucion']['avisos']:
 if 'ServiceLoader.load' in a: print(repr(a))
"
```
```
'ServiceLoader.load(org.slf4j.spi.SLF4JServiceProvider): 1 proveedor(es) leídos de META-INF/services; se entregan nombres, no instancias (aproximación)'
```

El nombre está escrito en la entrada real del APK:

```sh
unzip -p /home/leonisauro/Downloads/F-Droid.apk META-INF/services/org.slf4j.spi.SLF4JServiceProvider
```
```
org.slf4j.impl.LoggerServiceProvider
```

Se entregan nombres, no instancias: el intérprete no fabrica un proveedor ausente del fichero.

### Condición no negociable: probado solo en Node

El código verificado lo dice literalmente:

```sh
grep -n 'En wasm no hay ficheros del hospedador\|Sin una APK accesible el mapa sigue vacío' /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/verif-apkc/previewinterprete.go
```
```
4582:// En wasm no hay ficheros del hospedador: sin registro, el iterador sale vacío, que es la respuesta
4647:// `apkc preview … app.apk` esa es la APK de la corrida. Sin una APK accesible el mapa sigue vacío,
```

La medición de arriba es del comando nativo de Node, en el que `os.Args` contiene la APK. En la
pestaña, `cargaServiciosDelHost` no halla APK en `os.Args` y el iterador sale vacío para slf4j:
**esto está probado solo en Node** y no se afirma para wasm.

### Control byte a byte

```sh
for f in godot newpipe termux gestor-tres-pulsos; do cmp /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/$f.json /home/leonisauro/.cache/apkc-web/integracion-savedstate/$f.json; echo "$f rc=$?"; done
```
```
godot rc=0
newpipe rc=0
termux rc=0
gestor-tres-pulsos rc=0
```

```sh
sha256sum /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/{godot,newpipe,termux,gestor-tres-pulsos}.json /home/leonisauro/.cache/apkc-web/integracion-savedstate/{godot,newpipe,termux,gestor-tres-pulsos}.json
```
```
1c874f361460f09d2aa8ab365cd7138bef66e90ccc43ba599e4ff712b1bdf4d4  /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/godot.json
e4725ca1a9d61254cdbba48ddb10c91ef92f8545515948d309a53cc8ce60f948  /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/newpipe.json
b55eb52ebe5fa66237c16b372cdbf5c668ca9fd6a8b68e801ce0980b30efa925  /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/termux.json
7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f  /home/leonisauro/.cache/apkc-web/fdroid-serviceloader/gestor-tres-pulsos.json
1c874f361460f09d2aa8ab365cd7138bef66e90ccc43ba599e4ff712b1bdf4d4  /home/leonisauro/.cache/apkc-web/integracion-savedstate/godot.json
e4725ca1a9d61254cdbba48ddb10c91ef92f8545515948d309a53cc8ce60f948  /home/leonisauro/.cache/apkc-web/integracion-savedstate/newpipe.json
b55eb52ebe5fa66237c16b372cdbf5c668ca9fd6a8b68e801ce0980b30efa925  /home/leonisauro/.cache/apkc-web/integracion-savedstate/termux.json
7ebb99a3f787d3c1cf44619a46e95bd52fb2a23eb575116817ae3fd5a7a3b31f  /home/leonisauro/.cache/apkc-web/integracion-savedstate/gestor-tres-pulsos.json
```

Las cuatro parejas comparten `sha256sum` con la línea base de `integracion-savedstate/`.

**Frontera.** No se afirma que F-Droid funcione: la corrida corta en una frontera nueva, sin
forzar. Solo Node ha medido el proveedor real; en la pestaña el iterador sale vacío. El SHA final
queda pendiente de la integración. Sin gate, CDP, red ni publicación.

## 393. Integración final: trimpath, servicios y tema de splash

Medido el 2026-11-26 sobre los artefactos de
`/home/leonisauro/.cache/apkc-web/integracion-final/` y los registros conservados de
`/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/`,
`/home/leonisauro/.cache/apkc-web/godot-tema-splash/` y
`/home/leonisauro/.cache/apkc-web/fdroid-serviceloader/`, con las APK reales
`/home/leonisauro/Downloads/F-Droid.apk` y
`/home/leonisauro/Downloads/org.quentin_bettoum.librememorygame_9.apk`. Los cambios entraron con
`03e2ece` (trimpath) y los gitlinks `d2f9102` y `26282d9`; servicios y splash quedaron dentro del
emulador. Los dos builds del mismo árbol desde temporales distintos, la prueba funcional en el
hospedador de Node y las cinco corridas finales se verifican aquí. La ejecución instrumentada de
`resolveAttribute` y las cifras de focales transmitidas por los frentes previos no se re-miden.
Sin gate, CDP, red, `git push` ni publicación.

### Trimpath: dos temporales, la misma suma

```sh
grep -n 'materializados\|89f8738e\|rc node create\|ficheros en la memoria' /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log
```
```
5:materializados 34 ficheros preview*.go de /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/studio/apkc-emulador en /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc
6:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc.wasm
7:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc.wasm
10:materializados 34 ficheros preview*.go de /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/studio/apkc-emulador en /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc
11:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc.wasm
12:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc.wasm
15:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc.wasm
16:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc.wasm
23:--- ficheros en la memoria simulada: 5
29:rc node create = 0
```

Los builds desde los temporales `A/` y `B/` comparten la suma SHA-256
`89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b`. El hospedador de Node acabó
`create` con `rc node create = 0` y cinco ficheros en la memoria simulada. La corrida anterior,
sobre otro árbol, da una suma distinta con la misma cuenta de ficheros materializados:

```sh
grep -n 'materializados\|89f8738e\|ffa03ac5' /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log
```
```
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:5:materializados 34 ficheros preview*.go de /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/studio/apkc-emulador en /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:6:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc.wasm
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:7:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc.wasm
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:10:materializados 34 ficheros preview*.go de /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/studio/apkc-emulador en /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:11:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc.wasm
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:12:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc.wasm
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:15:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/A/apkc.wasm
/home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/resultado.log:16:89f8738e2b35a555b64b2072c0dad3aff725a92177d4b26d4f292e2a3968007b  /home/leonisauro/.cache/apkc-web/prep-trimpath-remedida/trimpath-remedida.HyGYev/B/apkc.wasm
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:5:materializados 34 ficheros preview*.go de /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/A/studio/apkc-emulador en /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/A/apkc
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:6:ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/A/apkc.wasm
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:7:ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/A/apkc.wasm
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:10:materializados 34 ficheros preview*.go de /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/B/studio/apkc-emulador en /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/B/apkc
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:11:ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/B/apkc.wasm
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:12:ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/B/apkc.wasm
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:15:ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/A/apkc.wasm
/home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log:16:ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/B/apkc.wasm
```

La diferencia de suma entre corridas se atribuye a los 34 ficheros `preview*.go` materializados,
no al flag `-trimpath`: las dos anotan la misma cuenta de materialización y el flag es el mismo en
los dos puntos de compilación. La atribución es la registrada por la integración y no se re-mide
aquí.

### Servicios: el proveedor sale del zip

```sh
grep -n 'registraServiciosDeLaAPKEnPreview' /home/leonisauro/Develop/Go/apkc-web/studio/apkc-emulador/preview.go
```
```
1132:	registraServiciosDeLaAPKEnPreview(zr)
```

```sh
grep -n 'func registraServiciosDeLaAPKEnPreview' /home/leonisauro/Develop/Go/apkc-web/studio/apkc-emulador/previewinterprete.go
```
```
4669:func registraServiciosDeLaAPKEnPreview(zr *zip.Reader) {
```

El envoltorio recibe el lector del zip ya abierto. El nombre que entrega está escrito en la
entrada real de la APK:

```sh
unzip -p /home/leonisauro/Downloads/F-Droid.apk META-INF/services/org.slf4j.spi.SLF4JServiceProvider
```
```
org.slf4j.impl.LoggerServiceProvider
```

Se entregan nombres, no instancias. Con la línea base y la corrida final:

```sh
python3 -c "
import json
pares=(('fdroid','/home/leonisauro/.cache/apkc-web/integracion-savedstate/fdroid.json','/home/leonisauro/.cache/apkc-web/integracion-final/fdroid.json'),('godot','/home/leonisauro/.cache/apkc-web/godot-tema-splash/antes.json','/home/leonisauro/.cache/apkc-web/integracion-final/godot.json'))
for n,a,b in pares:
 x=json.load(open(a)); y=json.load(open(b))
 print(n,'antes',x['ejecucion']['pasos'],'despues',y['ejecucion']['pasos'])
 print(' antes error:',x['ejecucion'].get('error'))
 print(' despues error:',y['ejecucion'].get('error'))
"
```
```
fdroid antes 7272 despues 7530
 antes error: en mu.KotlinLogging.logger: en org.slf4j.LoggerFactory.getLogger: en org.slf4j.LoggerFactory.getILoggerFactory: en org.slf4j.LoggerFactory.getProvider: en org.slf4j.LoggerFactory.performInitialization: en org.slf4j.LoggerFactory.bind: en org.slf4j.LoggerFactory.findServiceProviders: se llamó a java.util.ServiceLoader.iterator sobre un objeto nulo
 despues error: en mu.KotlinLogging.logger: en org.slf4j.LoggerFactory.getLogger: en org.slf4j.LoggerFactory.getILoggerFactory: en org.slf4j.LoggerFactory.getProvider: en org.slf4j.LoggerFactory.performInitialization: en org.slf4j.LoggerFactory.versionSanityCheck: se llamó a java.lang.String.startsWith sobre un objeto nulo
godot antes 8980 despues 9159
 antes error: en androidx.core.splashscreen.SplashScreen.installSplashScreen: en androidx.core.splashscreen.SplashScreen$Companion.installSplashScreen: en androidx.core.splashscreen.SplashScreen.access$install: en androidx.core.splashscreen.SplashScreen.install: en androidx.core.splashscreen.SplashScreen$Impl.install: se llamó a android.content.res.Resources$Theme.resolveAttribute sobre un objeto nulo
 despues error: en androidx.activity.EdgeToEdge.enable: en androidx.activity.EdgeToEdge.enable$default: en androidx.activity.EdgeToEdge.enable: se llamó a android.view.Window.getDecorView sobre un objeto nulo
```

En F-Droid desaparece `ServiceLoader.iterator sobre un objeto nulo` y aparece la nueva frontera
`String.startsWith sobre un objeto nulo` en `versionSanityCheck`, sin forzar. En Godot desaparece
`Resources$Theme.resolveAttribute sobre un objeto nulo` y aparece la nueva frontera
`Window.getDecorView sobre un objeto nulo` en `EdgeToEdge.enable`, sin forzar.

### Splash: id de framework, sin color inventado

El volcado `aapt2` conservado deja escrito el atributo del tema y la ausencia de entrada propia:

```sh
grep -n 'windowSplashScreenBackground(0x7f030015)' /home/leonisauro/.cache/apkc-web/godot-tema-splash/aapt2.txt
```
```
804:        windowSplashScreenBackground(0x7f030015)=?0x01010031
```

```sh
grep -c 'resource 0x01010031 ' /home/leonisauro/.cache/apkc-web/godot-tema-splash/aapt2.txt
```
```
0
```

El id `0x01010031` es del framework, no tiene entrada en la tabla del APK; `resolveAttribute`
devuelve `false` y no se inventa ningún color. El tema sintético y su aviso están en el intérprete:

```sh
grep -n 'func (m \*maquinaDalvik) temaSintetico\|tema-sintetico: Activity.getTheme' /home/leonisauro/Develop/Go/apkc-web/studio/apkc-emulador/previewinterprete.go
```
```
4787:func (m *maquinaDalvik) temaSintetico(act *objetoDalvik) *objetoDalvik {
5705:			m.avisa("tema-sintetico: Activity.getTheme entrega un tema enlazado a la tabla del APK (aproximación)")
```

Los ids de la corrida instrumentada están en el rastro conservado, pero esa instrumentación está
retirada del binario final: `resolveAttribute` sobre la APK real **no es re-medible** y no se marca
CONFIRMADO. Solo se confirma, con el par anterior, que la cadena `resolveAttribute sobre un objeto
nulo` desaparece en la corrida final.

### Las cinco APK finales

```sh
python3 -c "
import json
base='/home/leonisauro/.cache/apkc-web/integracion-final/'
for n in ('fdroid','newpipe','godot','termux','gestor-tres-pulsos'):
 d=json.load(open(base+n+'.json'))
 print(n, 'paquete=',d['paquete'], 'pasos=',d['ejecucion']['pasos'], 'nodos=',d['nodos'], 'aproximado=',d['aproximado'])
 print(' error:', d['ejecucion'].get('error'))
"
```
```
fdroid paquete= org.fdroid.fdroid pasos= 7530 nodos= 1 aproximado= True
 error: en mu.KotlinLogging.logger: en org.slf4j.LoggerFactory.getLogger: en org.slf4j.LoggerFactory.getILoggerFactory: en org.slf4j.LoggerFactory.getProvider: en org.slf4j.LoggerFactory.performInitialization: en org.slf4j.LoggerFactory.versionSanityCheck: se llamó a java.lang.String.startsWith sobre un objeto nulo
newpipe paquete= org.schabi.newpipe pasos= 3476 nodos= 6 aproximado= True
 error: en org.schabi.newpipe.util.ServiceHelper.getSelectedServiceId: en org.schabi.newpipe.util.ServiceHelper.getSelectedService: en us.shandian.giga.util.Utility.getService: se llamó a j$.util.stream.Stream.filter sobre un objeto nulo
godot paquete= org.quentin_bettoum.librememorygame pasos= 9159 nodos= 1 aproximado= True
 error: en androidx.activity.EdgeToEdge.enable: en androidx.activity.EdgeToEdge.enable$default: en androidx.activity.EdgeToEdge.enable: se llamó a android.view.Window.getDecorView sobre un objeto nulo
termux paquete= com.termux pasos= 7072 nodos= 13 aproximado= True
 error: en com.termux.app.TermuxActivity.addTermuxActivityRootViewGlobalLayoutListener: se llamó a android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo
gestor-tres-pulsos paquete= com.martinmimigames.simplefileexplorer pasos= 3595 nodos= 32 aproximado= True
 error: None
```

Las cinco corridas llevan `aproximado=true`. Cuatro acaban en una frontera nueva con `error`; el
gestor acaba sin error.

### La suite completa, en verde sobre el árbol final

La suite completa quedó en verde sobre el árbol final (`8156934e…`): `tcr -r 256M -q -- go test -count=1 ./...` con Go 1.24.0 dio `ok github.com/ajinasokan/apkc 195.144s`, ejecutado por el integrador; esa suite verde no revalidaba los cuatro stubs del §385, ausentes de ese pin, y la suite sobre el árbol que sí los contiene (`b7854d6…`) dio `ok github.com/ajinasokan/apkc 197.081s` con `tcr -r 256M -q -- go test -count=1 ./...`, ejecutado también con Go 1.24.0 y JDK 17. La frontera inerte se conservó eliminando el fallback `os.Args` y no relajando `TestInertesSinCargaNativaNiRedEnElCodigo`; el enganche `preview.go:1132` entrega el `*zip.Reader`. No hay deuda pendiente por `import "os"`.

**Frontera.** Ninguna de las cinco apps se da por funcionando: todas llevan `aproximado=true` y
cuatro acaban con `error`. Sin CDP no se afirma parecido visual alguno; los `.so` ARM quedan fuera
y las llamadas `System.load*` son inertes. La medición de servicios corresponde al comando nativo
de Node; en la pestaña el iterador sale vacío. Sin gate, CDP, red, `git push` ni publicación.

Las seis familias (`ViewTreeObserver`, `AudioAttributes`, `SoundPool`, `AccessibilityManager`, `findInsertIndex` y `and-int/lit8` con `&0xff`) se integraron sobre `b7854d6…` y se verificaron en el blob del pin; `case "split"`=1, `temaSintetico`=3, `case "getLifecycle"`=1, `registraServiciosDelApk`=1 (definición, `grep -c '^func registraServiciosDelApk'`; 4 ocurrencias en total), `cargaServiciosDelHost`=0. Termux 7418 pasos, `error=None`, `aproximado=true`: no se afirma arranque ni audio real.
