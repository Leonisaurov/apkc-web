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
