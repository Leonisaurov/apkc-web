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
