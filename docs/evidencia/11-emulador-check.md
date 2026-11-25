# Evidencia — §205-§240, §317-§319

## 205. E1 del plan de emulación: ciclo de vida y navegación entre actividades propias

Medido el 2026-11-22 sobre el worktree `plan-emulacion` (rama de trabajo, sin publicar), con el
`android.jar` de `/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar`. Cambia el
intérprete de la vista previa, no el compilador: `apkc/previewinterprete.go` ejecuta `onStart` y
`onResume` después de `onCreate`, llama a `onPause`/`onStop` al cubrir una actividad y a
`onRestart`/`onStart`/`onResume` al volver, y emula `Intent(Context, Class)`, `startActivity` y
`finish()` con una pila de actividades en memoria. Cada actividad conserva su máquina mientras está
cubierta: volver con `finish()` no repite su `onCreate`. Solo se abre una actividad que el
manifiesto declara; fuera de ahí queda un aviso y no se cambia de pantalla. El modelo publica
`ejecucion.pantallas` (recorrido, con la actual al final) y `ejecucion.ciclo` (métodos de ciclo de
vida en orden real). La pestaña no necesita código nuevo: el clic reejecuta la lista completa de
`--pulsar`, como ya hacía.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  go test -count=1 -run 'TestElPreviewNavegaEntreActividadesPropias|TestElPreviewNoAbreUnaActividadNoDeclarada' -v .
```

**Resultado**: las dos pruebas `PASS` (0,04 s y 0,02 s). La primera recorre un APK de dos
actividades: sin clics se ve `res/layout/main.xml`; con `--pulsar id/abrir` se ve
`res/layout/segunda.xml` y la actividad pasa a `com.hola.navega.SegundaActivity`; con
`--pulsar id/abrir --pulsar id/volver` vuelve al `main` con el texto en «Pulsos: 1», no en el
«Pulsos: 0» del layout, y el ciclo exacto es `Main.onCreate, onStart, onResume, onPause, onStop,
Segunda.onCreate, onStart, onResume, onPause, onStop, onDestroy, Main.onRestart, onStart, onResume`.
La segunda prueba deja la segunda actividad fuera del manifiesto: el `startActivity` deja el aviso
«no está declarada en el manifiesto», conserva el layout `main` y el recorrido de una sola pantalla.

```sh
APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar sh tools/entorno.sh test
APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar sh tools/entorno.sh wasm
HOME_APKC_WEB=/home/leonisauro STUDIO_NAV_PORT=9289 STUDIO_SERVIDOR_PORT=9291 \
  STUDIO_PORT=9445 STUDIO_CREAR_PORT=9292 STUDIO_CREAR_CDP=9446 sh tools/entorno.sh gate
```

**Resultado**: suite nativa `ok github.com/ajinasokan/apkc 57.626s`; suite wasm `PASS` con 274
ficheros en la memoria simulada; gate `crear: todo en orden (35 comprobaciones)`,
`pestaña: 204 comprobaciones` y `e2e: todo en orden`, sin «sin chromium» ni «sin APKC_WASM».
`apkc/previewnavega_test.go` es la prueba de aceptación.

**Fronteras de este `§`.** (1) `Intent` implícito, `startActivityForResult`, `launchMode`,
`singleTop` y las transiciones animadas quedan fuera: el primero se rechaza por nombre y los demás
no se emulan. (2) `finish()` desde la única actividad deja un aviso y conserva la pantalla; no se
simula el cierre del proceso. (3) Sigue fuera ejecutar métodos con tabla de excepciones
(`triesSize > 0`), porque el intérprete no la corre. (4) El sitio publicado no se ha empaquetado con
este cambio: `tools/entorno.sh sitio` y `surge . publish` siguen pendientes.

## 206. E1.1: correcciones de la auditoría del emulador, con su crítica

Medido el 2026-11-22 sobre la raíz de `web` con `def0b85` integrado (cherry-pick de `1e3b277` del
worktree `emu-e1-audit`). Es la fase E1.1 de `docs/plan-emulacion.md`: cierra los hallazgos 3.1, 3.2
y 3.4 de `docs/auditorias.md`; 3.3 (`getText` dentro de `onCreate`) ya estaba cerrado por `81d6acf`
y no se repitió aquí. Las líneas de código que se citan son las de `def0b85`; las integraciones
posteriores desplazan la numeración.

**Qué cambió, con su línea.** (1) Una actividad abierta por `startActivity` que no llama a
`setContentView` devuelve error y `estabiliza` avisa y reanuda la anterior, en vez de pintar el
layout `main` de la lanzadora (`apkc-emulador/previewinterprete.go:777`; el resultado se recibe en
`apkc-emulador/preview.go:314` y el layout se elige después en `:336`). (2) Un `setOnClickListener`
registrado en `onCreate` gana al `android:onClick` del XML sobre la misma vista, como en Android: el
enlace declarativo se salta si ya hay un oyente programático (`previewinterprete.go:990`). `pulsa` no
cambia de contrato: sigue valiendo el último oyente registrado, y el programático es el último. (3) El tope de 32 clics se
queda con su prueba y se añade el tope simétrico de 32 pantallas (`pantallasMaximas`,
`previewinterprete.go:47-50`, con el corte en `:639`): antes, dos actividades que se abren la una a
la otra en `onCreate` encadenaban pantallas sin fin; ahora se avisa y queda delante la última que
arrancó.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElInterpreteGetTextDevuelveElTextoActual|TestElPreviewOnClickProgramaticoGanaAlDelLayout|TestElPreviewClicNumeroTreintaYTresNoSeEntrega|TestElPreviewNavegaEntreActividadesPropias|TestElPreviewNoAbreUnaActividadNoDeclarada|TestElPreviewRechazaUnaActividadSinSetContentView|TestElPreviewTopeDeNavegacionCircular' -v .
```

**Resultado**: los siete `PASS` y `ok github.com/ajinasokan/apkc 0.149s`. La capa completa de la
vista previa, tal como la corrió el crítico, da `ok github.com/ajinasokan/apkc 0.435s`; `go vet` y
`gofmt -l` no imprimen nada. El crítico (`ses_df9f4b884ffepDlg2y2VQsIBYZ`) dio **APROBAR**, con los
siete comandos enfocados ejecutados y ningún defecto `CONFIRMADO`.

**Pregunta abierta del crítico (INCIERTA, no bloqueante)**: en un redirect sin layout que vuelve al
frente porque el destino llama a `finish()`, `layoutID==0` podría caer al layout `main`
(`previewinterprete.go:777`; `preview.go:314,336`). El probe de tres actividades que lo decidiría no
se ejecutó. **Matiz del crítico**: las pruebas del tope aseveran contra la constante
`pulsacionesMaximas`, no contra el literal 32; cambiar el valor de la constante no las rompería.

**Fronteras de este `§`.** (1) El gate y chromium no se volvieron a correr para este cierre: la
evidencia son los tests enfocados y la suite de la capa de vista previa. (2) La prueba de pestaña
que ejerza `startActivity` (D7 de `docs/auditorias.md`) sigue pendiente: aquí no hay ninguna nueva.
(3) El sitio publicado no se ha empaquetado con `def0b85`. (4) No hay medición en aparato ni del
orden de ciclo de vida ni del redirect sin layout del párrafo anterior.

## 207. E3: estado, recreación y persistencia local, con su crítica

Medido el 2026-11-22 sobre `a76422d`, con E3 (`5cc852a`), E4 (`99cd48f`) y E2 (`a76422d`) ya
integrados, y revisado por el crítico en la ronda 2 de `ses_df9ef9e02ffeqSQ1cuPypAdVer`.

**Qué emula.** `--pulsar` y `--recrear` comparten un solo acumulador de órdenes: se entregan en el
orden escrito, y `--recrear` es una bandera sin valor (`apkc/preview.go`). La recreación vuelve a
crear la actividad que está delante con el `Bundle` que dejó `onSaveInstanceState`; en la
recreación, el `Bundle` que recibe `onCreate` es no nulo, como en una rotación, y las actividades
cubiertas no se recrean. `SharedPreferences` es un almacén en memoria por nombre de fichero, con su
editor y `apply`/`commit`, sin ficheros fuera del modelo; el Bundle y las preferencias se publican
en `ejecucion.bundle` y `ejecucion.preferencias`. También se corrigió el `if-eqz` de Dalvik: un
registro nulo y el `int32(0)` son el mismo patrón de bits y la comparación los trataba distinto
(`apkc/previewinterprete.go`). La prueba de aceptación es `apkc/previewestado_test.go`.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElPreviewConservaElEstadoAlRecrear' -v .
```

**Resultado**: `--- PASS: TestElPreviewConservaElEstadoAlRecrear (0.05s)` y
`ok github.com/ajinasokan/apkc 0.061s`, sin `SKIP`. La prueba exige el texto «Pulsos: 2» tras dos
clics y una recreación, `bundle.pulsos=2`, `preferencias.cuenta.pulsos=2`, la consola con `pref: 0`
antes y `pref: 2` después, el ciclo exacto `onCreate, onSaveInstanceState, onCreate` y dos pantallas
con la misma actividad; además, que dos ejecuciones den el mismo JSON, que un clic posterior deje
«Pulsos: 3», que la actividad cubierta conserve su único `onCreate` y que la recreada tenga dos.

**Veredicto del crítico**: **APROBAR**. Las hipótesis H2 (E3 sin verificar por falta de
`APKC_ANDROID_JAR`) y H3 (árbol inconsistente por el cherry-pick de E2 en curso) quedan **FALSAS**:
el comando se ejecutó con el jar y `git status --short` estaba limpio, con el `grep` de marcadores
vacío. Nota suya: `gofmt -l` sobre todo `apkc/` devuelve `enlazar.go` y
`javaemisor_constantvalue_long_test.go`, ambos preexistentes (`66426c1` y `3928024`), no
introducidos por E2, E3 ni E4.

**Fronteras de este `§`.** (1) No se corrió el gate ni chromium: la evidencia es el test enfocado.
(2) No hay medición en aparato: el orden de ciclo de vida de la recreación es el que declara el
intérprete, no el observado en ART. (3) Las preferencias duran la ejecución y no tocan ficheros;
`ContentProvider`, `SQLite`, `DataStore` y el cifrado quedan fuera. (4) La recreación solo afecta a
la actividad que está delante. (5) El sitio publicado no se ha empaquetado con `5cc852a`.

## 208. E4: fidelidad visual del modelo, con su crítica

Medido el 2026-11-22 sobre `a76422d`, ampliado con la corrección `762b328`, y revisado por el
crítico en la ronda 2 de `ses_df9ef9e02ffeqSQ1cuPypAdVer`. La prueba de aceptación es
`apkc/previewvisual_test.go`; las líneas citadas son las del árbol de `a76422d` salvo donde se dice
otra cosa.

**Qué emula.** `android:visibility` en la medición y en el JSON: un `gone` no ocupa ni su margen,
un `invisible` conserva su sitio. `textSize` en `sp` y `dp`/`px` con la densidad mdpi del modelo,
`textColor` y `textStyle`, `background` con color propio (`#rrggbb` o `@color/x`) y `ImageView` con
un PNG del APK decodificado con `image/png`, con sus píxeles publicados en el nodo `imagen`.
`gravity` fino se lee también escrito como texto (`center_vertical|end`), y los `style="@style/x"`
del proyecto se resuelven desde `resources.arsc` con su cadena de padres del mismo paquete.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElPreviewRespetaVisibilidadTextoColorEImagen' -v .
```

**Resultado**: `--- PASS: TestElPreviewRespetaVisibilidadTextoColorEImagen (0.00s)` y
`ok github.com/ajinasokan/apkc 0.009s`. El layout de la prueba da cuatro hijos: el `gone` con
rectángulo `{0 0 0 0}`; el `TextView` de 24 sp en `{0 0 76 36}`, con `tamTexto=24sp`,
`color=#336699`, `estiloTexto=bold` y `fondo=#102030` (el color viene de `@color/x`; el estilo
resuelve `textStyle` y `background`); el `ImageView` de 2×2 en `{0 36 2 2}` con cuatro píxeles
`#ff0000ff`, `#00ff00ff`, `#0000ffff` y `#ffffffff`; y el `invisible` en `{280 38 80 24}`, con
`layout_gravity="end"` y `background="#00ff00"`. Lo mismo se comprueba releyendo el JSON.

**Veredicto del crítico**: **APROBAR**; el dictamen original fue APROBAR CON CONDICIONES, y sus dos
condiciones quedan resueltas en este `§`: H4 se cierra con la corrección `762b328` y H5 se fija
como frontera. No hay defecto `CONFIRMADO`. Se aplica la misma nota de `gofmt -l` de §207.

**H4, cerrada (corrección `762b328`).** La sospecha era cierta: `aapt2` escribe
`TYPE_INT_COLOR_RGB8` (`0x1d`), no ARGB8, para un `#rrggbb` de `res/values/colors.xml`, y el
lector solo reconocía `0x1c`. La sonda `ses_df9ea20a2ffeLaFRcu32o5Ddns` se ejecutó en
`/home/leonisauro/.cache/apkc-web/h4-color` con `aapt2` de
`/home/leonisauro/opt/android-sdk/build-tools/35.0.1/aapt2` y el `android.jar` de API 34:

```sh
AAPT2=/home/leonisauro/opt/android-sdk/build-tools/35.0.1/aapt2
JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar
cd /home/leonisauro/.cache/apkc-web/h4-color
$AAPT2 compile --dir res -o compiled.zip
$AAPT2 link -o out.apk -I "$JAR" --manifest AndroidManifest.xml --java gen compiled.zip
$AAPT2 dump resources out.apk | grep -i -C3 probe
```

**Resultado**: `resource 0x7f010000 color/probe` y `() #ff336699`; leído directo el `Res_value` del
`resources.arsc`, `dataType=0x1d data=0xff336699`. `762b328` añade `tipoColorRGB8 = 0x1d` en
`apkc-emulador/preview.go:70` y lo incluye en `colorDeCon` (`:1227`), con la prueba nueva
`TestElColorRGB8DeAapt2SeLeeComoColor` (`previewvisual_test.go:265`), que pasa el `Res_value` de la
sonda (`0x1d`, `0xff336699`) y exige `#336699`. Verificación sobre cinco pruebas:

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElColorRGB8DeAapt2SeLeeComoColor|TestElPreviewRespetaVisibilidadTextoColorEImagen|TestElPreviewPintaUnaListaConSuAdaptador|TestElPreviewConservaElEstadoAlRecrear|TestElPreviewNavegaEntreActividadesPropias' -v .
```

**Resultado**: cinco `--- PASS` y `ok github.com/ajinasokan/apkc 0.101s`.

**H5, frontera decidida (sin código).** `ladosDe` usa los ids de atributo `idVistaPadding*` como
claves de estilo tanto para `padding` como para `layout_margin` (`apkc-emulador/preview.go:1165-1173`); para
leer `layout_margin` desde `@style` harían falta los ids `layout_margin*` del framework, que no
están en el alcance escrito de E4. Queda como frontera honesta: `layout_margin` desde `@style` no
se resuelve, y no se finge que sí.

**Fronteras de este `§`.** (1) No hay oráculo de aparato para la geometría: lo fijado es la cuenta
declarada (alto de texto 1,5 veces el tamaño, 8 dp por carácter, 16 dp de relleno) y los
rectángulos que salen de ella. (2) `RelativeLayout` con reglas, `ConstraintLayout`, fuentes, temas
del sistema, sombras, `drawable` vectoriales y animaciones de vista quedan fuera. (3) `layout_margin`
desde `@style` queda fuera por H5. (4) No se corrió el gate ni chromium, y el sitio publicado no se
ha empaquetado con `762b328`.

## 209. E2: listas con adaptador del proyecto, su merge y su crítica

Medido el 2026-11-22 sobre `a76422d`. E2 nació como `d54fdea` en el worktree `emu-e2-listas` y se
integró con resolución manual de conflictos, coincidiendo con E3 y E4, que tocan `preview.go` y
`previewinterprete.go`. El crítico es `ses_df9ecde81ffeVOW5GZ5bA820E6`, que dictaminó sobre ese
mismo árbol.

**Qué emula.** `ListView` con un `BaseAdapter` o `ArrayAdapter` del proyecto: `setAdapter`,
`getCount`, `getView`, `getItem`, `getItemId`, `notifyDataSetChanged`, `setOnItemClickListener` y
la medición de cada fila con el layout que devuelve `getView`, ejecutando el `getView` del proyecto.
El clic por fila se escribe `--pulsar id/lista:<posición>`. `getChildCount`/`getChildAt` son una
aproximación declarada: no hay reciclado real de vistas.

**Cómo se resolvió el merge.** El crítico comprobó los cinco puntos a mano: (1) `objetoDalvik`
conserva los campos de E3 y los de E2; (2) conviven `almacenPreferencias` y `oyenteItemRegistrado`;
(3) el bucle de órdenes toma `pedido := orden.Valor`, atiende `--recrear`, separa `cortaFila`/
`pulsaFila` y mantiene el tope de 32 de `pulsa`; (4) el bloque final publica `res.Bundle`,
`res.Preferencias` y `res.maquina = m`; (5) el `usage` documenta `--recrear` y
`--pulsar id/lista:1`. Los 15 símbolos nuevos de E2 están en el árbol y no hay defecto
`CONFIRMADO`.

```sh
export APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestElPreviewPintaUnaListaConSuAdaptador' -v .
```

**Resultado**: `--- PASS: TestElPreviewPintaUnaListaConSuAdaptador (0.05s)`. La tanda de la vista
previa en el worktree dejó además las líneas `PASS` de `TestElPreviewPulsaElListenerQueSeLePide`,
`TestElPreviewOnClickProgramaticoGanaAlDelLayout`,
`TestElPreviewConservaElEstadoAlRecrear` y `TestElPreviewRespetaVisibilidadTextoColorEImagen`, con
`ok github.com/ajinasokan/apkc 0.445s`; la tanda de seis sobre la raíz dio `ok … 0.141s`.

**Pregunta abierta (INCIERTA, no bloqueante).** No se ejecutó una sonda combinada `--recrear` más
`--pulsar id/lista:1`; la convivencia de la lista con la recreación se verificó por inspección
(`apkc-emulador/previewinterprete.go:854-860` y `:944`, líneas de `a76422d`), no con una prueba. Queda
pendiente decidir si esa sonda entra en E5 o en la prueba de pestaña.

**Fronteras de este `§`.** (1) Quedan fuera `RecyclerView` con `LayoutManager` real, el reciclado,
`DiffUtil`, `ExpandableListView`, arrastrar filas y las listas de más de 50 filas. (2) También
quedan fuera `super.getCount()`/`super.getView()`, `ArrayAdapter` con `List`,
`registerDataSetObserver`, `getViewTypeCount`/`getItemViewType`, `getFilter`, `areAllItemsEnabled`,
`AdapterView.getItemAtPosition`/`setSelection`/`smoothScrollToPosition` e `inflate(XmlPullParser,…)`.
(3) No se corrió el gate ni chromium, y el sitio publicado no se ha empaquetado con `a76422d`.

## 210. E5: entrada de usuario, cerrada con su medición

Medido el 2026-11-22 sobre `2dd70c8`, cherry-pick de `993d51b` con 21 conflictos resueltos, en un
árbol que ya tenía `f802c0b` (sin topes de clics, capturas ni consola) y `2e3c629`.

**Qué emula.** `--escribir id/texto`, `--marcar id` y `--desmarcar id` entran por un solo
`guionDeEntrada` de `eventoUsuario`, en el mismo orden que los clics y las recreaciones. El modelo
publica `pista` (el `setHint` de un `EditText`) y `marcado` en el nodo y en las capturas. Teclear
avisa al `TextWatcher` del proyecto (`addTextChangedListener`) y marcar al
`OnCheckedChangeListener`; un `RadioGroup` deja una sola opción marcada. Los stubs nuevos son
`isChecked`, `setChecked`, `toggle`, `getHint` y `getEditableText`, y `append(Z)` imprime
`true`/`false` como el framework, no `1`/`0`.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElTextoNoSeCongelaAlLlegarAlTopeDeCapturas|TestElPreviewMuestraUnDialogoPropio|TestElPreviewEscribeYMarcaLoQueSeLePide|TestElGuionDeEntradaNoTieneTope|TestElPreviewConservaElEstadoAlRecrear|TestElPreviewNoTieneTopeDeClics|TestElPreviewPintaUnaListaConSuAdaptador|TestElPreviewNavegaEntreActividadesPropias|TestElPreviewRespetaVisibilidadTextoColorEImagen|TestElColorRGB8DeAapt2SeLeeComoColor' -v .
```

**Resultado**: diez `--- PASS` y `ok github.com/ajinasokan/apkc 0.191s`; el paquete completo,
`ok github.com/ajinasokan/apkc 54.564s`. La prueba de aceptación
`TestElPreviewEscribeYMarcaLoQueSeLePide` teclea «hola mundo», desmarca y vuelve a marcar la
casilla, cambia el radio y pulsa el botón: el `EditText` refleja el texto, el eco queda en
«Enviado: hola mundo», el verde gana el grupo y el rojo queda desmarcado, la captura es de
`usuario.escribir`, la consola trae `tecleo`, `despues`, `acepto=1`, `acepto=2`, `rojo=1` y
`verde=1`, y hay tres marcas publicadas. `TestElGuionDeEntradaNoTieneTope` entrega 160 órdenes
(40 tecleos, 40 marcas, 40 desmarcas y 40 clics) sin aviso de tope, con eco final
«Enviado: hola 40» y la casilla desmarcada.

**Merge.** Los 21 conflictos se resolvieron conservando las dos partes: el guion de entrada convive
con `--pulsar` y `--recrear` en el mismo acumulador de órdenes, y el modelo mantiene `Escrituras`,
`Pulsaciones`, `Marcas`, `Interacciones`, `Bundle` y `Preferencias`. La tanda de diez incluye las
pruebas de los topes retirados y las de E2, E3, E4 y H4 para comprobar que no se pisaron.

**Cierre y condición cumplida.** El crítico `ses_df9c6f81effeRKtmnqwkLjBU4s` dio **APROBAR CON
CONDICIONES**, sin hallazgos confirmados. La condición era repetir los tests de supervivencia de
E1.1, E3, E4, E2 y E7 después de integrar E6; el builder de E6 la cumplió en `88d9b38`: los diez
tests, incluidos `TestElPreviewConservaElEstadoAlRecrear`,
`TestElColorRGB8DeAapt2SeLeeComoColor`, `TestElPreviewPintaUnaListaConSuAdaptador` y
`TestElPreviewMuestraUnDialogoPropio`, pasan juntos (`ok … 0.195s`; paquete completo
`ok … 56.618s`). No hay topes reintroducidos (`grep` vacío) y el `help` de `preview` está corregido.

**Fronteras de este `§`.** (1) Se rechazan por nombre: teclear en algo que no es `EditText`, marcar
algo no marcable (incluido el `RadioGroup`), un id ausente de la tabla y `--escribir` sin la forma
`id/texto`. (2) Sin IME, cursor, selección, autocompletado, arrastre ni portapapeles. (3) Un
`setText` dentro de un `TextWatcher` no vuelve a avisar, para no encadenar avisos. (4) Deuda
apuntada: `String.valueOf(boolean)` sigue dando `1`/`0`. (5) No se corrió el gate ni chromium, y el
sitio publicado no se ha empaquetado con `2dd70c8`.

## 211. E7: diálogos propios y el `which` de sus botones, con su crítica

Medido el 2026-11-22 sobre `969b045`, con la lógica de E7 ya integrada y el defecto cosmético del
`usage` corregido en `2dd70c8`. La lógica la aprobó el crítico
`ses_df9d13306ffehyseNhLIqoaYfh`; la condición `INCIERTA` del `which` la cerró el QA
`ses_df9c1482fffeKi6lTSOz6CBbk0` con el commit `969b045` (`test(preview): verifica el which de los
botones del diálogo`).

**Qué emula.** `AlertDialog.Builder` con título, mensaje y hasta tres botones; `Toast.makeText()`
como línea de consola y aviso efímero; `dismiss()`. Al pulsar un botón se entrega su `which` real
del framework: `-1` el positivo, `-2` el negativo y `-3` el neutro.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElPreviewEntregaElWhichDeCadaBotonDelDialogo|TestElPreviewMuestraUnDialogoPropio' -v .
```

**Resultado**: `--- PASS` en las dos y `ok github.com/ajinasokan/apkc 0.060s`. El diálogo publica
sus tres botones en el orden positivo, negativo y neutro; cada pulsación deja en el `TextView` y en
la consola (`Log.i`) la línea `positivo=-1`, `negativo=-2` o `neutro=-3`, que es lo que el oyente
leyó como `which`. La prueba se falsificó de forma adversativa: mutar el mapa a `+1/+2/+3` o quitar
el botón neutro la hace fallar.

**Fronteras de este `§`.** (1) Diálogos del sistema, `DialogFragment` con `FragmentManager`, menús
contextuales y notificaciones del sistema quedan fuera. (2) No se corrió el gate ni chromium, y el
sitio publicado no se ha empaquetado con `969b045`.

## 212. E6: reloj lógico, cerrado con la aprobación de la ronda 3

Medido el 2026-11-22 sobre `35d5f79` (`fix(preview): desbordamiento en difiere y reloj coherente
con los pasos`); `apkc/` no cambió entre ese commit y `6243d9d`, que solo mueve el puntero de
`studio/`. El crítico `ses_df9bd547cffefBHTZrtCZdEv1i` dio **APROBAR** en la ronda 3.

**Causa raíz.** Los defectos de E6 eran aritmética y estado derivado sin función total: dos sumas
de reloj sin guardia y un presupuesto de pasos global que cortaba la cola y publicaba un destino
que no se había ejecutado. Se corrigió con la guardia de desbordamiento en `difiere`
(`apkc-emulador/previewinterprete.go:674-684`), el presupuesto por tarea (`pasosBase`, `:654`, con el
límite en `:2458-2461`) y el corte solo cuando hay una tarea vencida (`vence:648`). Las dos únicas
sumas de reloj (`avanza:604-607` y `difiere:678`) tienen guardia; las demás son asignaciones.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElRelojNoDesbordaEnDifiere|TestElRelojNoMienteAlAgotarPasos|TestElRelojNoDesborda|TestElRelojAvanzaSinTruncar|TestElRetrasoNegativoNoRetrocedeElReloj|TestElTextoNoSeCongelaAlLlegarAlTopeDeCapturas|TestElPreviewEntregaElWhichDeCadaBotonDelDialogo|TestElPreviewMuestraUnDialogoPropio|TestElGuionDeEntradaNoTieneTope|TestElPreviewConservaElEstadoAlRecrear|TestElInterpreteDecodificaCuentasRamasArreglosYAnchos|TestElPreviewNoTieneTopeDeClics|TestElPreviewPintaUnaListaConSuAdaptador|TestElPreviewAvanzaElRelojLogico|TestElGuionDelPreviewConservaElOrden|TestElColorRGB8DeAapt2SeLeeComoColor' -v .
```

**Resultado**: las 16 pruebas `PASS`, `ok github.com/ajinasokan/apkc 1.174s`. Las tres sondas de la
ronda 3, ejecutadas como pruebas temporales y borradas después, dieron:

| sonda | entrada | salida |
| --- | --- | --- |
| desbordamiento en `difiere` | `postDelayed(MaxInt64)` desde un clic con `reloj=100` | total vacío, `reloj=100`, `tareas=0` y aviso «no cabe» |
| avance largo sin truncar | 100 000 tics, `--avanzar 100000000` | `Tic: 100000`, `reloj=100000000`, `tareas=100000`, sin aviso |
| variante con `AsyncTask` de arranque | `apkDelReloj`, `--avanzar 100000000` | `Tic: 99999`, `reloj=99999000`, aviso de corte: el reloj coincide con el último tic ejecutado y no miente |

Los dos hallazgos de la ronda 2 quedan **CERRADOS** con esas salidas. El crítico enumeró las dos
sumas de reloj y revisó `pasosBase` y el estado compartido por navegación y recreación: **no hay un
tercer defecto `CONFIRMADO`**. Dejó dos observaciones no bloqueantes: tras un corte, un `--avanzar`
posterior mueve el reloj al destino aunque la cola no se drene (con el aviso ya presente), y el
código que corre fuera de tareas conserva un `pasosBase` antiguo, comportamiento previo a este
commit.

**Fronteras de este `§`.** (1) No se corrió el gate, chromium ni la suite completa. (2) Las tres
sondas eran pruebas temporales y no viven en el árbol; lo permanente son las pruebas de reloj del
bloque. (3) No hay medición en aparato: el reloj es lógico. (4) El sitio publicado no se ha
empaquetado con `35d5f79`.

## 214. I4: dexado por clase, sin cambiar un byte del `classes.dex`

Medido el 2026-11-22 en el worktree `inc-i4-i5` sobre `banco/gen-proyecto-grande.py` (231 fuentes,
244 clases, 1875 `code_item`, 195 844 bytes de dex). El ensamblado del dex tiene offsets absolutos
—cambiar una instrucción de una clase mueve todo lo que va detrás—, así que lo que se guarda entre
pasadas no es el fichero sino lo que **no nombra ningún offset**: el `code_item` de cada método
(`debug_info_off` a cero, lo parchea el ensamblado) y su `debug_info_item`. Eso es el 41 % del
tiempo de la etapa: con `-cpuprofile` sobre esas 244 clases, `areaCodigo` acumula 200 de 480
muestras y `collectCode` 60.

La validez de lo guardado es una sola condición, y es lo que hace que el fichero no pueda diferir:
las cinco tablas ordenadas tienen que ser las mismas, porque cada operando traducido es un índice de
`string_ids`, `type_ids`, `proto_ids`, `field_ids` o `method_ids`. `huellaTablas` las resume en un
SHA-1; si no coincide, la caché se tira y la vuelta es un dexado completo. La clave de cada método
es el SHA-1 del fichero `.class` de su clase más su nombre y descriptor: cualquier cambio de la
clase re-traduce todos sus métodos, y las clases sintéticas de las lambdas (sin fichero detrás)
quedan fuera.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 128M -- go test -count=1 \
  -run 'TestDexadoPorClaseReutilizaYNoCambiaUnByte' -v .
# y el criterio del plan, sobre el proyecto grande, reproducible desde el repo:
REPETICIONES=1 APKC=… sh banco/incremental.sh   # imprime «dex por clase: … identicos»
```

**Resultado.** Las tres vueltas de la prueba `PASS` (`ok github.com/ajinasokan/apkc 0.052s`): el
frío no reutiliza nada y deja la caché, la edición de un cuerpo con las tablas iguales reutiliza 6
de 9 `code_item` y su dex es `bytes.Equal` al de un dexado desde cero, y una clase nueva que mueve
las tablas no reutiliza nada y también sale idéntica.

Sobre el proyecto grande (`banco/incremental.sh`, una repetición):

| vuelta | `code_item` reutilizados | `dex go` | `cmp` contra el dexado completo |
| --- | --- | --- | --- |
| frío (sin caché) | 0 de 1875 | 100 ms | — |
| caliente tras editar `Mod037.java` | 1866 de 1875 | 45 ms | idéntico (`cmp` mudo) |
| completo del mismo árbol (`apkc clean` y build) | 0 de 1875 | 100 ms | referencia |

El build entero pasó de 581 a 508 ms en esa tanda, y el apk sigue igual entrada por entrada (80
entradas, 0 con bytes distintos) con `apksigner verify` y `zipalign -c -p 4` correctos.

**Fronteras de este `§`.** (1) No se corrió el gate, chromium, `prueba-pestana.js` ni la suite wasm:
la cifra es nativa. (2) La prueba de identidad es la del repo; el `cmp` sobre el proyecto grande
salió de `banco/incremental.sh` con un `TRABAJO` propio. (3) Lo que queda por recortar en la etapa
—`leeClases` 11 ms, `collectClasses` 13 ms y el ensamblado de las áreas globales— no se tocó: I5 es
el índice del `android.jar` entre pasadas y ataca a la compilación, no al dex.

## 215. I4: los dos `INCIERTO` del crítico, cerrados

Cierra las dos dudas que el crítico de I4 dejó abiertas sobre `829cad6`.

1. **Reutilización con las áreas desplazadas.** La vuelta que ejercitaba la prueba cambiaba un
   literal (`x * 3` → `x * 7`) y no movía un byte de sitio. Se añade una vuelta que **alarga** el
   cuerpo de una clase ya medida (`cuenta` gana tres sentencias, sin una sola cadena, método o campo
   nuevos, así que `huellaTablas` sigue igual): el área de código crece, la depuración, el
   `class_data` y los arrays se desplazan, y sus offsets —`debug_info_off` incluido— tienen que
   recalcularse. Resultado: 6 de 9 `code_item` reutilizados (los de las dos clases intactas), el dex
   pasa de 1196 a 1212 bytes y `bytes.Equal` contra el dexado desde cero del mismo árbol.

2. **Caché corrupta.** `cargaCacheDex` comparaba `p+n` con el tamaño total; con una `n` de cuatro
   gigabytes el `int` de 32 bits del wasm desborda y el guard pasaba por accidente. Ahora se compara
   `n` contra **lo que queda** (`n > len(cuerpo)-p`), el recuento de cada sección tiene su propio
   tope, y el fichero lleva un SHA-1 del cuerpo detrás de la marca: un byte movido en los blobs ya no
   se reutiliza. La escritura va por temporal y `rename`, como `saveTable`. Cinco casos en la
   prueba —truncada, sin cuerpo, marca de otra versión, longitud imposible con el resumen bueno y un
   byte cambiado— caen al dexado completo con aviso, sin pánico, y el dex sale idéntico al de
   referencia.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestDexadoPorClaseReutilizaYNoCambiaUnByte|Test.*Cache' -v .
```

**Resultado**: `--- PASS` en las dos pruebas y en los cinco casos (`ok github.com/ajinasokan/apkc
0.251s`), y la suite entera verde (`ok github.com/ajinasokan/apkc 55.098s`).

**Fronteras de este `§`.** Sigue sin gate, sin chromium y sin wasm: la medición es nativa. El SHA-1
del cuerpo detecta la corrupción, no la repara: el coste de una caché ilegible es el dexado completo.

## 213. El intérprete de la vista previa ejecuta la tabla de excepciones del dex

La tabla que el compilador del fork ya emitía (§71, E1.1) ahora se **ejecuta**. `readCodeItem`
(`apkc/dexcode.go`) lee los `try_item` y la `encoded_catch_handler_list` detrás de las insns —con el
relleno de una unidad cuando el cuerpo tiene un número impar de palabras— y guarda los bytes de la
lista tal cual, porque el `handler_off` de cada `try_item` es un desplazamiento en bytes a ella.
`ejecutaCuerpo` (`apkc/previewinterprete.go`) ya no rechaza `tries_size > 0`: corre el cuerpo y, si el
error es una excepción de la app (`excepcionDalvik`), busca el primer `try_item` cuyo rango cubre la
unidad, recorre sus filas en orden (tipo, dirección) y acepta el `catch_all`, sube por la jerarquía
del dex y por la del framework que hace falta para despachar (`padresDeExcepcion`) y reanuda en el
`handler_pc` con `move-exception` entregando el objeto. Un error que **no** es una excepción —un
opcode sin decodificar, un registro fuera del marco— sigue siendo un fallo duro y no se disfraza de
catch. El `throw` del dex lanza su objeto; `Integer.parseInt` que no parsea lanza
`NumberFormatException`; la división y el resto entero por cero lanzan `ArithmeticException`; y
`Throwable` gana stubs de `getMessage`, `getLocalizedMessage`, `toString` y `printStackTrace` sobre el
mensaje que guarda su constructor. Sin handler, el error sale con el tipo y el mensaje de la
excepción, marcado como fallo de la app y no como dex malformado.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'Test.*Try|Test.*Excepc|Test.*Catch' -v .
```

**Resultado**: `PASS` en `TestElInterpreteEjecutaElTryCatchYDejaElTexto` (un `Integer.parseInt("x")`
lanzado desde un método propio y capturado por `catch (NumberFormatException e)` deja
`capturado: For input string: "x"` en el `TextView`, y la prueba exige **una** sola escritura porque el
`setText` de dentro del `try` no llega a ejecutarse), en
`TestElInterpreteEjecutaElTryYAtrapaLaDivisionPorCero` (`4 / 2` imprime `resultado 2` y `4 / 0`
imprime `fallo -1`, que es el `catch`) y en `TestElInterpretePropagaElThrowNoCapturado` (el error
nombra `IllegalStateException` y `boom`, no hay capturas y `malformado` es falso). La suite entera
queda verde: `ok github.com/ajinasokan/apkc 58.559s`. La prueba que medía la frontera anterior
—`TestElInterpreteRechazaLasExcepcionesPorSuNombre`— se reescribió como la del try con división por
cero: esa frontera ya no existe.

**Fronteras de este `§`.** El `finally` y el multi-catch **no** se han medido: el compilador los emite
como catch-all y como varias filas del mismo handler, y el despacho debería cubrirlos, pero eso no
está probado y no se afirma. Una excepción que cruce de una actividad a otra **sí** ocurre, como
aproximación: la pila `pila` existe (`apkc-emulador/previewinterprete.go:1280,1307`), y si la segunda
actividad lanza, su arranque se degrada a aviso («B no arrancó: …IllegalStateException: boomB») y la
primera reanuda con `Error=""` y `Pantallas=[A]`. Lo midió el crítico
`ses_df93095c5ffeyRt8kl6K1MhO3T` con `go test -run TestCriticoExcepcionCruzaActividad` (prueba
temporal, no está en el árbol). Los tipos del framework fuera de `padresDeExcepcion` se reconocen por el
sufijo `Exception;`/`Error;` para el `throw`, pero un `catch (PadreDesconocido e)` no casaría con su
hijo: la tabla de padres es la que hay, no la de Android entera. Sigue sin gate, sin chromium y sin
wasm: la medición es nativa.

## 216. Dependencias: recursos `res/__deps/` y `lib/<abi>/` en el enlazador

Medido el 2026-11-22 sobre `976b02e` (`feat(apkc): enlaza recursos y lib/ de dependencias`), trabajo
del builder en el worktree `deps-recursos`. La prueba está en `apkc/deps_recursos_test.go` y el
crítico `ses_df915d191ffetJ53F3oGQiKaI1` dio **APROBAR** sin hallazgos confirmados.

**Qué hace.** `apkc/linker.go` fija `carpetaDeps = "__deps"`; `tipoDeRecurso` lee el tipo bajo la
isla y `fuentesDelProyecto` recorre `res/__deps/` además de `res/`, valida una carpeta por
artefacto y tipo y escribe la salida como `res/<tipo>/x.xml`, sin la isla (`:240-242`, `:252` y
`:295`). `apkc/enlazar.go` añade `recursosDeFicheroRepetidos`, que rechaza con el nombre de fichero
cuando app y AAR declaran el mismo recurso de fichero, como el «duplicate value» de `aapt2`
(`:418-430`). `packagePlan` ya metía `lib/**` y no se tocó.

**Pruebas.** La isla se lee; un AAR sintético con `classes.jar`,
`res/values/deps_strings-<isla>.xml`, `res/layout/deps_fila.xml` y `lib/x86/libejemplo.so` con
cabecera ELF32 produce el apk esperado; el duplicado de fichero se rechaza; y el incremental
rehace el paquete al cambiar el `.so`.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestFuentesDelProyectoLeeLaIslaDeDependencias|TestRecursosDeDependenciaEnElApk|TestRecursosDeFicheroRepetidos' -v .
```

**Resultado**: tres `--- PASS` y `ok github.com/ajinasokan/apkc 0.527s`; los cuatro tests de la
raíz que cubren el frente dan `ok … 0.813s` y la suite completa, `ok github.com/ajinasokan/apkc
56.504s`. El crítico atacó la ruta de salida sin isla, el rechazo de duplicados, la entrada
`lib/x86/libejemplo.so`, el rebuild incremental al cambiar el `.so` y la regresión de I3/I4/I5; todo
resistió.

**Fronteras declaradas, no defectos.** (1) Los tipos de recurso no medidos quedan fuera: `drawable`
XML/vectorial, `anim`, `font` y algunos `values`. (2) Dos AAR con la misma clave `values` no se
detectan: solo se rechazan ficheros duplicados. (3) El `AndroidManifest.xml` y el `R.txt` del AAR
quedan fuera del enlace. (4) No se corrió gate, chromium ni wasm: la medición es nativa.

## 217. Dependencias: importador local sin red

Medido el 2026-11-22 sobre `be7b417` y `studio/` en `66c35ab`. El crítico
`ses_df91c58e7ffeuwOqrjEwncRX3Z` dio **APROBAR** sin hallazgos confirmados.

**Qué hace.** La vía local no usa `fetch`, ni Maven, ni proxy: ingiere desde disco u OPFS y deja
`jar/`, `lib/<abi>/`, `res/__deps/<hash>/…` y `deps.importado.json`. El traversal `../` en zip y
AAR se rechaza; los ids y el orden se comprueban bajo `conBloqueo`.

```sh
cd studio && node --check deps.js && node prueba.js
```

**Resultado**: `prueba: todo en orden`. La falsificación del builder invirtió `ficheros.reverse()` y
produjo 4 fallos; restaurado, volvió a verde.

**Notas abiertas, no bloqueantes.** (1) TOCTOU del `confirm`. (2) Atomicidad ante cuota agotada.
(3) El hash de isla FNV no coincide con el `sha1(coord)[:12]` de `apkc-deps`. La fusión de
`res/__deps/` ya quedó cerrada por la Fase K (§216).

**Fronteras de este `§`.** No se corrió gate, chromium, red ni publicación; la verificación es la
suite de `prueba.js` en Node.

## 218. Dependencias: clases de `jar/` en el compilador Go

Medido el 2026-11-22 sobre `5bc1e8c`, con las condiciones cerradas en `8d6ee67`. El crítico
`ses_df91c8423ffeRBUEJpqWc5sSv5` dio **APROBAR CON CONDICIONES**.

**Qué hace.** `apkc compile/build --compiler go` resuelve clases de `jar/*.jar` y del
`classes.jar` de un `.aar`; el dex sale idéntico al de javac 17, el índice es por jar y un cambio de
dependencia invalida el reuso de I3.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestCompilarEnGoResuelveUnaClaseDeJar|TestCompilarEnGoDexaIgualQueJavacConUnaDependencia|TestCompilarEnGoLeeElClassesJarDeUnAar|TestCompilarEnGoInvalidaElReusoSiCambiaUnaDependencia|TestHuellaDelClasspathCubreCadaDependencia|TestIndiceDelJarPorJar|TestPorteroDelCompilerNombraLoQueNoMide' -v .
```

**Resultado**: 7/7 `PASS` y `ok github.com/ajinasokan/apkc 1.772s`. Las tres condiciones quedaron
cerradas por `8d6ee67`: `check` resuelve el classpath del jar (crítico
`ses_df90d3cfaffeThh7Mx1cOC9IVn`, que seguía en curso al medir), el test de cadena A→B→
`android.jar` y el arnés sin `SKIP` silencioso, con `APKC_ANDROID_JAR` fijado.

**Fronteras de este `§`.** El dex de las dependencias con `d8` sigue fuera: es el trabajo de
`deps-dexgo`. No se corrió gate, chromium ni wasm.

## 219. Dependencias: fixture nativo y su rechazo clasificado

Medido el 2026-11-22 sobre `8d99e52` y su corrección `aee59d4`. El crítico
`ses_df911247affeIZ83mX0fq3TTsR` dio **APROBAR**.

```sh
sh banco/deps-ejemplo/construir.sh
```

**Resultado**: el camino nativo compila y dexa la app (1136 bytes) y la dependencia (728 bytes); el
camino Go falla con el error literal `the d8 of dependency bucket 0 failed: exec: no command`, sale
con `rc 3` y escribe `OBJETIVO PENDIENTE: el dex de las dependencias todavía lo escribe d8; esta
sonda es su aceptación`. No hay binarios versionados.

**Fronteras de este `§`.** El camino Go en verde exige `deps-dexgo`; el fixture solo clasifica el
rechazo. Sin gate, chromium ni red.

## 220. Dependencias: dex en Go, sin d8, y caché por bucket

Medido el 2026-11-23 sobre `3bc6c52` (`feat(apkc): dexa dependencias en Go sin d8`), corregido por
`e84a64f` (`fix(apkc): la caché de dex por bucket se escribe en su fichero`). El crítico
`ses_df90847d1ffeaWfOT8dxl9zhB3` retiró con evidencia nueva su `CONFIRMADO` sobre `guarda()` y dio
**APROBAR**.

**Qué hace.** El dexer propio escribe el `classes.dex` de las dependencias sin invocar `d8`. La caché
de cada bucket vive en `build/dexcache/deps-gocache-*.bin`, separada de `build/dexcache.bin` del
proyecto; no quedan `.tmp` y dos buckets con `APKC_JOBS=2` no comparten temporal.

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestLaCacheDelBucketSeEscribeEnSuFicheroYReutiliza|TestDexDepsDosBucketsNoCompartenTemporal|TestBuildGoDexaLaDependenciaSinD8|TestCacheDexCorruptaCaeAlDexadoCompleto' -v .
```

**Resultado**: los cuatro `PASS`, con los cinco subtests de corrupción, y `ok github.com/ajinasokan/apkc
0.064s` en la raíz; la misma tanda en el crítico, `ok … 0.123s`, y la suite completa,
`ok … 50.078s`. `TestBuildGoDexaLaDependenciaSinD8` usa un `d8Path` con marcador que no se ejecuta;
`TestLaCacheDelBucketSeEscribeEnSuFicheroYReutiliza` exige `reutilizados == total` en la segunda
vuelta, y `TestDexDepsDosBucketsNoCompartenTemporal` cubre 12 jars en 2 buckets.

**End-to-end.** `sh banco/deps-ejemplo/construir.sh` sale con **rc 0** y deja
`1 clases de 1 dependencias -> 612 bytes`; los dos dex invocan `Lsaludo/Saludo;.saludo`. Con
`android-support-v4.jar` en el shim, el APK mide 9049 B con `classes.dex` de 1604 y
`classes2.dex` de 612; `apksigner` v2/v3 y `dexdump` del segundo dex, en verde. El wasm se compiló y
la preview se reconstruyó (`sha256 90fc0011…`), verificada contra `/apkc.wasm`.

**Frontera, no defecto.** El dexer propio escribe **un** `classes.dex` por bucket y no parte al
superar 65 536 referencias de método (el límite de índices de 16 bits). Un `invokedynamic` no
medido falla nombrando el jar, la entrada y el sitio. Queda como trabajo aparte. No se corrió gate,
chromium ni publicación.

## 221. Apps de ejemplo: cinco builds reproducibles y la comparación no archivada

Medido el 2026-11-23 sobre `30c2afc` (`feat(banco): apps de ejemplo interactivas y sonda de
endurecimiento`). El crítico `ses_df8d5e78affeJAoal8XzOTzm46` aprobó el commit con una condición
documental: el informe archivado de la sonda no contiene las diferencias que se habían citado.

```sh
sh banco/apps-ejemplo/construir.sh
sh banco/apps-ejemplo/probar.sh
```

**Resultado confirmado.** Las cinco apps compilan con `--linker go --compiler go`; los tamaños de
dex son 4760, 5428, 5940, 6504 y 5320 bytes, con 187-240 ms por build. No se versiona ningún
binario. La sonda apunta solo a `10.177.0.2:5555` y no hace red ni push.

**Fronteras confirmadas.** No inflan: `&&` en una condición, una condición que no es comparación,
`Long.toString`, la concatenación de un `long` y la vista propia de `lienzo`.

**Comparación aparato↔preview: no archivada.** El informe
`$HOME/.cache/apkc-web/apps-mejorar/sonda/informe.md` dice «aparato 0 con id» y «no se pudo leer el
tiempo», así que las diferencias de texto y de tiempo observadas en esa corrida no son medidas y no
se citan. Quedan como objetivos de H3 y hay que repetir la sonda con una ventana sana. El «~6 s» de
retirada de la ventana de Droidloom tampoco tiene `§`; es pregunta abierta.

**Fronteras de este `§`.** No se corrió gate, chromium ni publicación; la comprobación es la del
crítico sobre el árbol de `30c2afc`.

## 222. El colector de la tabla sigue el operando de `ldc_w`

Medido el 2026-11-23 sobre `c129657` (`fix(dex): el colector sigue el operando de ldc_w`), con la
cita del test corregida en `45621e8` (`fix(test): corrige la cita del test de ldc_w`). El crítico
`ses_df8b1e8dcffeNIm0O0VSmlcvGk` aprobó el arreglo y señaló que la medición no tenía `§`: este lo es.

**Qué mide.** El colector de referencias de `dextabla.go` solo seguía `ldc`, de un byte. javac cambia
a `ldc_w` (0x13) cuando el índice del pool pasa de 255, y el tipo de una `CONSTANT_Class` quedaba sin
internar; ART exige su `type_id` en el `const-class` que sale de esa instrucción
(`apkc/dextabla.go:219-228`). La sonda genera 200 métodos con cadenas distintas para forzar un
`ldc_w #407` con un pool de 617 entradas (`apkc/javaemisor_ldcw_test.go:51`).

```sh
cd apkc && APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run TestElLdcWInternaElTipoYCoincideConD8 -v .
```

**Resultado**: `--- PASS: TestElLdcWInternaElTipoYCoincideConD8 (2.07s)` y `ok … 2.078s` en la
raíz; tras integrar la corrección de cita (`45621e8`), `ok … 2.172s`.

**Falsificación.** En una copia temporal (`~/.cache/apkc-web/ldcw-copia/`), sin tocar el árbol,
quitar `case "ldc_w"` de `dextabla.go` da `the type_id Ljava/util/HashMap; was never interned (the
collector does not follow this operand)` y `FAIL`; restaurarlo da `PASS`.

**Oráculo `d8`.** El `d8` de build-tools 36.0.0 sobre el mismo `.class` escribe las palabras
idénticas, `1c00 0400  |0000: const-class v0, Ljava/util/HashMap; // type@0004`, en nuestro dex y en
el suyo; el `javap` del `.class` de javac 17 dice `ldc_w #407 // class java/util/HashMap`.

**Inventario.** Con el dexer de ese árbol: 520 clases, 20 rechazadas y 3 tipos de límite;
desaparecen las 2 clases de «tipo no internado por el colector».

**Fronteras de este `§`.** No se corrió gate, chromium ni publicación; el cambio se limitó a
`dextabla.go` y su test.

## 223. Reparto de un bucket grande en varios dex

Medido el 2026-11-23 sobre `492c3f0` (`feat(build): reparte un bucket grande en varios dex`). Un
bucket de dependencias que no cabe en un `classes.dex` se parte en `classes2.dex`, `classes3.dex`… con
las tablas que mide el propio dexer (`apkc/reparto.go:90`, `apkc/dex.go:124`); un bucket que cabe
sigue siendo un solo dex, byte a byte el de antes.

```sh
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestReparte|TestBucket|TestDexEntry' -v .
PROBAR_LIMITE=1 sh banco/validar-apk/validar.sh   # con el binario del árbol integrado
```

**Falsificación.** Desactivar el reparto (`reparteEnDexes(clases, 1<<30)`, un solo grupo) da
`the method index does not fit a 16-bit index`, rc 1 y **sin `app.apk`**: la segunda unidad de
`21c`/`22c`/`35c` es un `u16` y el `method_id` que no cabe se rechaza. Ese es el estado de antes, no
un defecto nuevo.

**Resultado con reparto.** 700 clases y 70 000 métodos (1 206 648 B): el bucket pasa a **2 dex**, que
salen como `classes2.dex` y `classes3.dex`, y el build sale con rc 0; `app.apk` lleva `classes.dex`,
`classes2.dex` y `classes3.dex`; `apksigner` da `true` en v1, v2 y v3, `zipalign` da `ok`, y
`classes2.dex` invoca `Lgrande/C680;.m0`, que define `classes3.dex`. Dos vueltas dieron **sha1
idénticos**.

**Fronteras de este `§`.** El dex de la **app** (no el de las dependencias) aún no se reparte. El
guion `banco/validar-apk/validar.sh:253` todavía exige el rechazo viejo con `PROBAR_LIMITE=1` y otro
agente está invirtiendo esa comprobación. No se corrió gate, chromium ni publicación.

**Actualizado el 2026-11-23 en `e22f10c`** (`test(banco): valida el reparto de 70 000 métodos en
varios dex`): el guion ya no exige el rechazo, valida el reparto (rc 0,
`classes.dex`/`classes2.dex`/`classes3.dex`, ningún dex por encima de 65 536 `method_id`) y cierra con
el `OK` del techo. Esta nota sustituye a la frase de «Fronteras» sobre `validar.sh:253`; el resto de
este `§` sigue describiendo la medición de `492c3f0`.

## 224. Clase duplicada entre dependencias

Medido el 2026-11-23 sobre `8be5250` (`fix(jar): nombra la clase duplicada entre dependencias`). La
guarda vive en `dependenciasDexeables` (`apkc/dependencias.go:175`), común a los dos dexers, y nombra
la clase y **ambos** jars que la definen, no solo el último.

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestDependenciasDexeables|TestClaseDuplicadaEntreJars' -v .
```

**Resultado**: los 4 tests de la raíz pasan (`ok … 0.022s`).

**Oráculo `d8`.** El `d8` de build-tools 34.0.0 sobre dos jars que definen `dup.D` da
`Type dup.D is defined multiple times: a.jar:dup-D.class, b.jar:dup/D.class` y rc 1. Nuestro mensaje
es `la clase dup.D está definida más de una vez: jar/a.jar y jar/b.jar`.

**Falsificación.** Quitar la guarda hace fallar
`TestDependenciasDexeablesAbortaConDosJarsQueDefinenLaMismaClase` con «el build no abortó…»; con la
guarda, `PASS`.

**Fronteras de este `§`.** `module-info.class` y `package-info.class` de la raíz se excluyen a
propósito (`apkc/dependencias.go:171`): dos jars modulares no colisionan por ellos. No se corrió gate,
chromium ni publicación.

## 225. El frente `jar/` cierra: inventario a cero y el APK del jar real verificado

Medido el 2026-11-23 sobre `6742687` (`fix(java): los throws no van en la Signature`), que lleva
dentro los tres arreglos que destapó el jar real: `dd50e71` (`fix(dex): ordena los try_items por
dirección de inicio`), `c106f6a` (`fix(dex): escribe los encoded_value y el directorio de anotaciones
como el verificador`) y `64a7da9` (`fix(dex): marca la mitad alta del par en conversiones en sitio`).

**El inventario, a cero.**

```sh
python3 banco/inventario-jar/inventario.py
```

**Resultado**: `banco/inventario-jar/informe.md` queda con **520 clases recorridas, 0 rechazadas y 0
tipos de límite**, la tabla de límites vacía y `casos.json` en `{}`. Cada clase se dexó en su propia
invocación de `apkc dex`, así que el conteo de clases es exacto (`banco/inventario-jar/README.md:55-59`).
El oráculo `d8` dexa el jar completo con **rc 0** y deja un `classes.dex` de **563 612 B**
(`banco/inventario-jar/informe.md:24`).

**El APK del jar real, por los oráculos.**

```sh
APKC=<binario del árbol> \
  JAR_REAL=/home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar \
  sh banco/validar-apk/validar.sh
```

**Resultado**: el build `--linker go --compiler go --signer go` dexa las **520 clases** del jar en un
solo dex de **526 856 B** y el APK sale con `classes.dex` (la app) y `classes2.dex` (la dependencia);
`apksigner verify --verbose` da `true` en la v1, la v2 y la v3, `zipalign -c -p 4` da `ok`, y
`dexdump` ve las 520 clases en `classes2.dex` sin un solo `Failure to verify`. El cruce queda medido:
`classes.dex` invoca `Landroid/support/v4/text/TextUtilsCompat;.htmlEncode` y `classes2.dex` lo
define, que es lo que exige `verifica_metodo` (`banco/validar-apk/validar.sh:178`). El guion cierra
con `OK: los dos APK llevan los dex esperados, la firma pasa por los tres esquemas y los oráculos ven
el método del jar`; en esta corrida imprimió además el `OK` del techo de 16 bits (§223), de ahí el
doble `OK`.

**Antes de los tres arreglos.** El jar real no dexaba y el desorden estaba en el fichero que
escribíamos: `dexdump` daba `Out_of_order try_item with start_addr: 61`, y detrás aparecían
`Bad index for encoded_value string` y `Out-of-order field_idx for annotation`. Los tres se
corrigieron en `dd50e71`, `c106f6a` y `64a7da9`. El inventario pasó de 20 rechazadas sobre `492c3f0`
(§222) a 0 sobre el árbol de este `§`; los conteos intermedios de las corridas del frente son
historial, no estado vigente.

**La pestaña.** El wasm se reconstruyó con `sh tools/preview-local.sh` y quedó en
`sha256 8026c54aebfcf6af097a305f5faad55ca0ebb11b827c90ab351a476f82ab3b0e`.

**Fronteras de este `§`.** Siguen fuera de la medida: `invokedynamic`/`condy`; un método que pase de
32 KB y pida `goto/32` (resuelto en `23033fe`, medido en el §226); el dex de la **app**, que aún no se
repartía en este árbol (§223; repartido en `e05213b`, medido en el §228); API < 21; y el `check`
mono-fuente (resuelto en `a4bf224`, medido en el §227). No se corrió gate, chromium ni publicación: la
comprobación es la del inventario y la del guion de los oráculos.

## 226. Promoción a `goto/32` de las ramas largas

Medido el 2026-11-23 sobre `23033fe` (`feat(dex): promueve a goto/32 las ramas largas`). Hasta este
commit el dexer escribía la forma corta y el código se quedaba sin sitio cuando el salto pasaba de
32 767; ahora la rama se promueve a `goto/32` (y el condicional a su forma con `goto/32` detrás).

```sh
tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestDex.*Salto|TestDex.*Goto' -v .   # en apkc/
```

**La sonda.** 4300 sentencias `y = consume(<long>L)`, que empujan el salto por encima del techo de 16
bits y obligan a la forma larga.

**Oráculo `d8` + `dexdump`.** El `d8` de build-tools deja el condicional como un `if-gtz v2,+5`
seguido de `goto/32`, y el salto largo como `goto_w 35011` que se desarma en `goto/32` con
desplazamiento `+40009`. Nuestro dex escribe las mismas formas para las mismas ramas y `dexdump -d`
sobre él da rc 0 (verifica y desarma).

**Falsificaciones.** Dos, y las dos en rojo como se esperaba: devolver `salto32` a `salto16`, y hacer
que el `condicional` ignore `promocionCond32`. Los cuatro tests fallan con `a branch that needs a long
form at pc 3`; restaurado el código, vuelven a verde.

**Suite y wasm.** La suite completa da `ok … 69.558s` y el wasm compila.

**Frontera de este `§`, cerrada en `§235`.** El emisor de `.class` rechazaba un `goto` hacia
atrás de más de 32 767 **bytes** —el arreglo de este commit es del dexer, no del emisor de
`.class`—; ya no:
`resuelveAtras` (`apkc/javaemisor.go:7059-7065`) delega en `resuelveSaltos`, que ensancha a `goto_w`
(`:2619-2622`), y `TestGotoWLargoHaciaAtras` (`apkc/javaemisor_atras_test.go:51`) lo mide con una
vuelta de 36 009 B. No se corrió gate, chromium, red ni publicación.

## 227. `check` compila todos los fuentes del proyecto

Medido el 2026-11-23 sobre `a4bf224` (`fix(check): compila todos los fuentes del proyecto`).

```sh
ANDROID_HOME=/home/leonisauro/opt/android-sdk APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestCheck.*|TestIndiceDelJar' -v .   # en apkc/
```

**Antes.** `check --proyecto P P/src/MainActivity.java` con `Helper.java` en otro fuente del proyecto
daba `el tipo Helper no se resuelve…` y respondía `emisor:false`: el `check` solo compilaba el fichero
pedido.

**Después.** `{"emisor":true,"errores":[]}` y los 4 tests nuevos en verde. La causalidad está
demostrada: en el padre de `a4bf224` esos mismos 4 tests fallan.

**Contrato del informe.** Los errores de otros `.java` del proyecto nunca se convierten en filas del
fichero pedido: van a `aviso`, con el nombre del fichero y la causa. `emisor` queda en `false` si lo
que falló fue el parseo y en `true` si fue la emisión. El contrato de `compile`/`build` sale
byte-idéntico, verificado con el binario anterior.

**Fronteras de este `§`.** Dos ficheros con el mismo nombre en paquetes distintos; un fuente hermano
que no parsea deja el informe sin verificar y con aviso; la latencia de leer N fuentes no se midió con
techo; y el comportamiento en wasm/OPFS real no está verificado, porque no se corrió gate. Tampoco se
tocó red ni publicación.

## 228. El dex de la app también se reparte

Medido el 2026-11-23 sobre `e05213b` (`feat(build): reparte también el dex de la app`). Hasta este
commit `compilar.go` dexaba el proyecto con `dexDeClasesConCache`, en un solo fichero: un proyecto con
más de 65 536 referencias de método propias no compilaba. Ahora `dexarAppEnGo` (`apkc/compilar.go:315`)
llama a `dexDeClasesRepartidas` —la misma etapa que un bucket de dependencias— y escribe `classes.dex`,
`classes2.dex`, … con una caché I4 por grupo y el mismo camino para las clases de `jar/` por debajo de
API 21 (`apkc/compilar.go:304`). Los grupos se escriben de atrás adelante y un `classesN.dex` de una
vuelta anterior que hoy no toca se borra (`apkc/compilar.go:335`).

```sh
tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestDexDeLaAppSeReparteYElViejoCaminoFalla' -v .   # en apkc/
```

**Falsificación.** El mismo conjunto —70 clases de 1000 métodos declarados y un llamador de la última
(`apkc/multidex_app_test.go:58`)— dexado con la etapa vieja falla con `the method index does not fit a
16-bit index`; con el reparto son 2 dex y el registro dice `71 clases -> 794788 bytes en 2 dex`.

**Identidad de las apps pequeñas.** El dex del reparto y el de un solo fichero son **el mismo byte a
byte**, no aparece `classes2.dex`, y el primer grupo sigue dejando `build/dexcache.bin` como antes.

**Build real, por los oráculos.** Una app sintética de 70 clases por 1000 métodos deja `classes.dex` de
**2 425 984 B** y `classes2.dex` de **200 724 B**; `apksigner` da rc 0 en v1, v2 y v3, `zipalign` rc 0 y
`dexdump` rc 0, con `C69.m999` en el último dex.

**Suite y wasm.** La suite completa da `ok … 76.211s` y el wasm compila.

**Deudas.** Dos, ninguna medida como cerrada: los `build/dexcache-gN.bin` que sobran cuando el proyecto
encoge **no se podan** —los borra `apkc clean`, que hace `os.RemoveAll("build")`
(`apkc/build.go:470-472`)—; y el `.aab` multi-dex no está medido con `bundletool` (resuelto en
`0237e23`, medido en el §229).

**Fronteras de este `§`.** No se corrió gate, chromium, red ni publicación.

## 229. El `.aab` multi-dex, derivado por bundletool

Medido el 2026-11-23 sobre `0237e23` (`fix(aab): ensambla los dex del aab desde el plan correcto`) y
`e3a7c97` (`test(banco): el aab con dex de dependencias pasa los oráculos`), con el caso del banco
fijado en `f75b42e` (`fix(banco): fija el caso de dependencias del banco aab`).

**El defecto y el arreglo.** El ensamblador del `.aab` recorría `build/` con `ficherosDeArbol` y
nombraba cada dex por su `filepath.Base`, así que el `classes.dex` que un bucket de dependencias deja
en `build/dexcache/…` caía encima del de la app: el `.aab` listaba `base/dex/classes.dex` dos veces y
la firma lo rechazaba con `build/app.aab lists base/dex/classes.dex twice, and a signature cannot say
which of them the platform is meant to read`. `0237e23` pasa los `depDexes` a `buildAAB`
(`apkc/aab.go:20`) y numera con `dexEntries` (`apkc/build.go:1490`) la lista que recibe
`ensamblarAABDesdeGo` (`apkc/aab_go.go:314`), la misma que empaqueta el APK (`apkc/build.go:1744`):
bundle y APK ya no pueden discrepar.

```sh
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElAABNoRepiteElDexDeLosBuckets' -v .
sh banco/aab-multidex/aab-multidex.sh --dependencias
```

**Falsificación.** `TestElAABNoRepiteElDexDeLosBuckets` arma sobre el árbol real del build un bucket
con su `classes.dex`, distinto del de la app para que se note si se cuela (`apkc/aab_go_test.go:175`), y
reproduce la lista vieja: el árbol da **dos** entradas `classes.dex` y la firma falla con
`base/dex/classes.dex twice`. Con `dexEntries`, el mismo árbol produce `classes.dex` +
`classes2.dex`, y firma y verifica. Es la falsificación del defecto, no la del camino nuevo.

**Caso 1, el dex de la app** (`sh banco/aab-multidex/aab-multidex.sh`, rc 0): 703 clases →
`703 clases -> 3025352 bytes en 2 dex`; el `.aab` lleva `classes.dex` + `classes2.dex` con **65 459** y
**5 756** `method_id`, ninguno por encima de 65 536; `bundletool validate` da rc 0 y `bundletool
build-apks` en `default` y `universal` deriva APK con los mismos dos dex; `apksigner` da `true` en v1,
v2 y v3, `zipalign` da `ok`, y `classes.dex` invoca `grande/C680;.m0`, que define `classes2.dex`.

**Caso 2, con dependencias** (`--dependencias`, rc 0): 22 jars repartidos en tres buckets → el `.aab`
lleva `classes.dex`, `classes2.dex`, `classes3.dex` y `classes4.dex`; `bundletool validate` rc 0 y
`build-apks` en los dos modos con los cuatro; firma v1/v2/v3 `true`, `zipalign ok`, y los tres cruces
quedan medidos entre ficheros: `Lsaludo/Saludo;` y `Ldep/D0;` los define `classes4.dex`, y `Ldep/D20;`
lo define `classes2.dex`, invocados los tres desde `classes.dex`. `PROBAR_DEPENDENCIAS=1` da el mismo
caso en verde, `-h` sale con rc 0 y un argumento desconocido con rc 1.

**El flag, además de la variable.** El caso 2 se activa con `--dependencias` y también con
`PROBAR_DEPENDENCIAS=1` porque el arnés sanea el entorno al invocar `sh <fichero>`: el hijo arranca con
la variable fuera (traza `+ '[' 0 = 1 ']'`) y el flag viaja en `argv`
(`banco/aab-multidex/aab-multidex.sh:56-69`).

**Fronteras de este `§`.** El `bundletool` es el jar local
`$ANDROID_HOME/bundletool/bundletool-all-1.18.3.jar`, sin red; el `.aab` no se probó en un aparato (ni
`bundletool install-apks` ni Play). No se corrió gate, chromium, red ni publicación.

## 230. El gate final, en verde

Medido el 2026-11-23 en la raíz sobre `d456592` (`chore(studio): aísla el navegador de pruebas en
puertos y perfil propios`; el submódulo apunta a `fcd5798`, `fix(prueba): aisla el navegador en puertos
y perfil propios`). El usuario autorizó expresamente esta corrida.

```sh
LD_LIBRARY_PATH=/home/leonisauro/.cache/apkc-web-gate-final/shimlib \
  tcr -c 4 -m 3G -r 512M -- sh tools/entorno.sh gate
```

**Salida.** La última línea es `e2e: todo en orden` y no hay ninguna línea `FALLO`. Lo que prueba esa
corrida:

- **plantillas**: Contador, ciclo, menú y traducido salen firmadas y alineadas;
- **vista previa**: 3 nodos, `res/layout/main.xml`, el modelo `LinearLayout[TextView,Button]`, un clic
  que deja «Pulsos: 1» y 2 líneas de consola;
- **crear**: todo en orden, 36 comprobaciones;
- **pestaña**: 229 comprobaciones, y la APK se firma dentro de chromium (10 285 B) y se verifica;
- **pestaña, release**: keystore propio (CN=Release) con firma v1+v2+v3, la v4 con su `.idsig` de
  5 562 B, el `.aab` con su v1 JAR, y la rotación con linaje v3.1 y v4 de dos firmantes;
- **pestaña, compilando**: APK de 9 252 B verificada por `apksigner`, `zipalign`, `zipcheck` y `aapt2`;
  el dex de la pestaña lleva `com.myapp.MainActivity`, el proyecto generado sale **byte a byte** igual
  al nativo y `d8` lo relee, más el fuente escrito a mano y la app del contador;
- **firma**: los casos bueno, con-v3, bloque-alterado, rotación, EC P-256/P-384 y SHA-1; el icono sale
  con `aapt2` y el PNG literal.

**Lo que costó arrancarlo, y no era del producto.** El primer intento no llegó a correr el gate: el
chromium del sistema pedía `libsimdutf.so.36` y un chromium huérfano seguía escuchando en el puerto 9333
con un OPFS ajeno, así que el arnés hablaba con la corrida anterior. Se corrigió el entorno con el
`LD_LIBRARY_PATH` al shim local (`~/.cache/apkc-web-gate-final/shimlib/libsimdutf.so.36`) y el arnés
pasó a puertos efímeros —`PUERTO` a `0` y el puerto real leído del `DevToolsActivePort`
(`studio/prueba-navegador.js:78-96`)— y a un perfil por PID,
`studio-chrome-perfil-<pid>` (`studio/prueba-navegador.js:55-60`). Ninguno de los dos arreglos toca el
compilador ni la página: son del arnés y del entorno.

**Preview y suite.** La vista previa de ese árbol quedó en `sha256 e1ea319e…`, y la suite Go completa
dio `ok … 72.735s`.

**Fronteras de este `§`.** El gate se corrió **en local**, con el shim de biblioteca, y **no** se corrió
CI en GitHub: un workflow escrito no está verde hasta que el job pasa. El push de `studio/` sigue
pendiente de autorización del usuario, así que este árbol no se ha publicado; seguridad y hot swap
siguen diferidos. No se tocó red ni publicación en esta corrida.

## 231. Genéricos en `extends` y en el campo: los dos bugs del usuario

Medido el 2026-11-23 sobre el árbol integrado, con los tres arreglos que salieron del informe del
usuario: `4ff2d13` (`fix(java): argumentos de tipo en extends e implements`), `979a5b3` (`fix(java):
argumentos de tipo en campos y variables`) y `6742687` (`fix(java): los throws no van en la Signature`).

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  JAVAC17=/home/leonisauro/opt/jdk17/bin/javac \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestCheckClausulaGenericaEnExtends|TestClausulaGenericaContraJavac17|TestCheckCampoGenerico|TestCampoGenericoContraJavac17|TestThrowsFueraDeLaSignatureContraJavac17|TestDirectorioCentralDelJarContraArchiveZip' -v .
```

**Resultado**: 9 tests en verde y `ok … 2.618s`; la capa enfocada de `throws`, campo e índice del jar
dio `ok … 1.577s`. El comando arrastra además `TestDirectorioCentralDelJarContraArchiveZip` —el índice
del jar contra `archive/zip`—, que cae en esa misma capa.

**Caso 1, la cláusula de tipo en `extends`.** `public class ApkAdapter extends ArrayAdapter<Apk> {`
daba `se esperaba "{" y hay "<"` en el `check`; `4ff2d13` consume los argumentos de tipo en `extends` e
`implements`.

**Caso 2, el campo genérico.** `private final List<Apk> apks;` daba `se esperaba un nombre de miembro y
hay "<"`; `979a5b3` los admite en campos y variables.

**Falsificaciones.** Sin consumir el `<` en esos dos sitios, **5 tests** fallan con esos dos mensajes
literales; sin la `Signature` del campo, el test falla y nombra la de `javac` como esperada,
`Ljava/util/List<Lcom/hola/apkcstudio/Apk;>;` (`apkc/javaemisor_campo_generico_test.go:99`). Los dos
oráculos contra `javac` 17 (`TestClausulaGenericaContraJavac17` y `TestCampoGenericoContraJavac17`)
comparan el `.class` byte a byte.

**Tercer hallazgo, del mismo informe.** `firmaDeMetodo` metía los `throws` concretos dentro de la
`Signature` sin el `^` que pide la JVMS 4.7.9.1; `6742687` los deja solo en `Exceptions`, y
`TestThrowsFueraDeLaSignatureContraJavac17` lo comprueba byte a byte contra `javac` 17.

**Contrato que se conserva.** El descriptor sigue siendo el *raw*, sin genéricos; la `Signature`
genérica va medida en su atributo; `check` queda a cero errores; y el `LineNumberTable` no se toca.

**Fronteras de este `§`.** No medidos: los comodines (`?`), los parámetros de tipo declarados en la
clase (`class C<T>`), `new T<...>()` y los `throws` genéricos. El `goto/32` del emisor de `.class` es
otro frente (§226 cierra el del dexer). No se corrió gate, chromium, red ni publicación.

## 232. El marco de recursos se cachea entre pasadas de `check`

Medido el 2026-11-23 sobre `9ad60da` (`perf(check): reutiliza el marco de recursos parseado`). El
`check` inflaba y andaba los bloques del `resources.arsc` del `android.jar` en cada pasada; ahora deja
el marco —ids, valores y formatos— en una caché junto al proyecto y lo relee.

```sh
# en apkc/
ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestMarcoCacheado|TestCheckMarcoCacheadoCorruptoNoFalsifica|TestCheckAprovechaElIndiceDelJar|TestDirectorioCentralDelJarContraArchiveZip' -v .
```

**Resultado.** 5 tests `PASS` en el paquete y `ok … 1.180s`.

**La sonda.** `TestMarcoCacheadoMideElAhorro` (`apkc/marcocache_test.go:187`) da tres vueltas sobre el
mismo jar, con la caché apagada y encendida, y compara además los dos marcos pieza a pieza. Sus
medianas locales: **119,5 ms sin caché frente a 3,4 ms con caché**, medidas con reloj de pared.

**Cifras del builder.** Marco 114,1 → 4,6 ms y `check` con `res/` 125,7 → 9,3 ms en el escritorio; en
wasm sobre OPFS 408 → 11,6 ms y 338 → 36,2 ms; la caché ocupa 512 004 B por jar.

**Falsificación.** La cabecera rechaza marca, huella, tamaño, CRC y fichero cortado; con la caché
corrupta `check` enlaza con **0 errores** y la reescribe sana para la pasada siguiente
(`apkc/marcocache_test.go:260`), y no crea `build/` si no existe.

**Fronteras de este `§`.** Las cifras wasm/OPFS son del informe del builder y no se reejecutaron
aquí. `jarDelFramework` sigue saltándose dos tests en este escritorio, igual que antes de este
commit: es una frontera previa, no una regresión. No se tocó red ni publicación.

## 233. Recursos de AAR, dibujo de vista propia y un APK arrastrado

Medido el 2026-11-23 sobre `ee01c9b` (`feat(preview): recursos de AAR y dibujo de vista propia`) y
`793fc9d` (`feat(preview): carga un apk arrastrado al emulador`).

```sh
# en apkc/
tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'Test.*Preview|Test.*Recurso|Test.*Vista|Test.*Aar' -v .
```

→ `ok … 1.885s`. La suite completa, `go test -vet=off ./...` en `apkc/`, da `ok … 77.143s`, y
`node prueba.js` en `studio/` termina en `prueba: todo en orden`.

**Lo que se mide.** Los recursos que llegan por `.aar` (`anim`, `font` y compañía) entran en la vista
previa; la prioridad del proyecto se respeta por encima de la dependencia; y `<solid>` y `onDraw` de
una vista propia se interpretan. Un APK de terceros arrastrado al emulador,
`com.monuk7735.nope.remote`, sale con un `ejecucion.error` honesto en vez de una vista inventada.

**Falsificaciones del builder.** Sin `anim`/`font`, sin la prioridad del proyecto, sin `<solid>`, sin
`onDraw`, sin el stub de `drawLine`, sin la puerta del zip y sin el lector de bloque complejo, el
test correspondiente falla.

**Mensajes del rechazo.** Un APK sin layout responde `el apk no lleva ningún res/layout/*.xml`
(`apkc-emulador/preview.go:962`) y un zip roto, `el apk no es un zip` (`apkc-emulador/preview.go:465`).

**Fronteras de este `§`.** `onDraw` corre sobre una instancia nueva con un `Canvas` de mentira, no
sobre el árbol dibujado real. `vista-previa.js` todavía no pinta `nodo.dibujo`. Un APK ofuscado puede
elegir el primer layout que encuentre, y `ejecucion.error` aparece cuando el dex usa features que el
intérprete no cubre. No se tocó red ni publicación.

## 234. `check` con varios fuentes: colisiones y hermanos inválidos

Medido el 2026-11-23 sobre `0e440c7` (`fix(check): cubre colisiones de nombres y hermanos inválidos`).
Cierra los tres límites que dejó abiertos `§227`.

```sh
# en apkc/
tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestCheck|TestIndiceDelJar|TestMarcoCacheado' -v .
```

→ `ok … 4.949s`; la suite completa, `ok … 87.977s`.

**Caso 1, el nombre simple repetido.** `TestCheckDosHelperEnPaquetesDistintos`
(`apkc/check_test.go:806`): con `com.hola.apkcstudio.uno.Helper` pedido y
`com.hola.apkcstudio.dos.Helper` en el proyecto, el conjunto de hermanos lleva el del otro paquete y
salta la copia en disco del pedido por su nombre cualificado, no por el simple; el informe queda en
cero errores, `emisor:true` y sin aviso.

**Caso 2, el hermano que no parsea.** `TestCheckHermanoQueNoParseaSeAvisa` (`apkc/check_test.go:850`):
`Roto.java` con un bloque sin cerrar deja `errores` vacío, `emisor:false` y un aviso que nombra el
fichero y la causa, sin atribuirla al fuente pedido.

**Caso 3, el tipo de `jar/` en un hermano.** `TestCheckTipoDeJarLocalEnUnHermano`
(`apkc/check_test.go:932`): el tipo que solo referencia otro fuente del proyecto resuelve porque
`check` arma el mismo classpath que `build`.

**Falsificaciones.** Tres, y caen en estos mensajes literales del test: `con Helper en dos paquetes el
pedido da N errores` (`apkc/check_test.go:812`) y `faltan hermanos` (`:834`); `el fallo de Roto.java
se coló en la lista del pedido` (`:855`) y `se dio por comprobado un fuente cuyo hermano Roto.java no
parsea` (`:858`); `el tipo de jar/ en un hermano da N errores` (`:938`).

**Fronteras de este `§`.** Siguen fuera una referencia cualificada dentro de una expresión
(`apkc/javaemisor.go:3871`), un import de un segundo tipo con el mismo nombre simple
(`apkc/javaemisor.go:956-961`) y un candidato sin `package` con el mismo nombre. No se tocó red ni
publicación.

## 235. `goto_w` en el emisor de `.class`, y el rechazo nombrado de `invokedynamic` y `condy`

Medido el 2026-11-23 sobre `01d681c` (`fix(apkc): escribe goto_w en la vuelta larga del emisor de
.class`) y `764e2f1` (`test(apkc): mide el rechazo de invokedynamic y condy fuera del subconjunto`).

```sh
# en apkc/
tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestGotoWLargoHaciaAtras|TestIndyDeRecordSeRechazaConNombre|TestCondySeRechazaConNombre|TestDexSaltoLargoSePromueveAGoto32|TestDexGotoWLargoSePromueveAGoto32' -v .
```

→ `ok … 4.691s`; la suite completa, `ok … 86.916s`. Los dos tests del dexer que entran en ese filtro
son los de `§226` y vuelven en verde.

**La vuelta larga.** `TestGotoWLargoHaciaAtras` (`apkc/javaemisor_atras_test.go:51`) produce una vuelta
de 36 009 bytes y el salto hacia atrás sale como `goto_w` en el byte 36 011; el `Code` es **byte a
byte** el de `javac` 17 y el dex, idéntico al de `d8` (oráculo del builder con `javap`, `d8` y
`dexdump`; los comandos están en su informe).

**`invokedynamic` de record.** `TestIndyDeRecordSeRechazaConNombre` (`apkc/indy_condy_test.go:53`): el
rechazo nombra el sitio y el bootstrap `ObjectMethods.bootstrap` (`apkc/lambda.go:164`), en vez de caer
en una lambda mal emitida.

**`condy`.** `TestCondySeRechazaConNombre` (`apkc/indy_condy_test.go:78`): se rechaza en
`apkc/classreader.go:590` (la cita original decía `:451`; corregida en §243), y el oráculo `d8` da rc 1,
la misma frontera que `d8`.

**Fronteras de este `§`.** Los bootstraps distintos de `LambdaMetafactory` no se midieron; `condy`
queda rechazado, no soportado; y la API < 21 sigue documentada como fuera del subconjunto. No se tocó
red ni publicación.

**Nota del gate.** El gate de `§230` se corrió **antes** de estos cinco frentes —la caché del marco, los
recursos de AAR y el APK arrastrado, los límites de `check`, `goto_w` y el rechazo de
`invokedynamic`/`condy`—, así que no se puede leer como su verificación; repetido en `§242` sobre el
árbol final, verde.

## 236. La verificación de ART, ampliada a ocho casos

Medido el 2026-11-23 sobre `e1110df` (`fix(dex): escribe las anotaciones del usuario y respeta el marco
del catch`) y `d6e7e99` (`test(banco): verifica la estructura del dex en ART`).

```sh
APKC=<binario del HEAD> CASOS="zchar estaticos tries anota arreglos disper cadenas ifaces" \
  sh banco/art-verificador/art-verificador.sh
```

**Salida**: `OK: zchar estaticos tries anota arreglos disper cadenas ifaces en verde, sin rechazo de
verificación`. Los ocho casos pasan `dexdump -a` igual al de `d8`, `install` con `Success`, `am start`
con `Status: ok`, cinco filtros de `logcat` limpios y `Log.d` idéntico al de `d8` y al `esperado.txt`.

**Los seis defectos que cerró.** (1) las anotaciones de usuario, `RuntimeVisible`/`RuntimeInvisible` y
`AnnotationDefault`; (2) el `VerifyError` dentro de `try` (`protegido` y `bajaAltoGuardar`); (3) el
`wide iinc` (0xc4), que pasa a `add-int/lit16`; (4) el `new-array` con el índice alto; (5) el MUTF-8 con
NUL y subrogados; y (6) los sitios `invokedynamic`, indexados por (método, pc) en vez de por método.

**Falsificación.** Un dex cortado da `FALLO: no aparece el RESULTADO`: la corrida no puede pasar por
ausencia de log. La suite completa dio `ok … 83.959s`.

**Fronteras de este `§`.** Los puntos de unión —el ternario y el booleano de comparación— y `"" + x`
siguen sin medirse. No se tocó red ni publicación.

**Actualizado en §240**: los puntos de unión y la hipótesis de `"" + x` quedaron cerrados por
`eb680b3`/`2940e86`.

## 237. Inyección de shell y «zip bombs»: los dos arreglos de seguridad

Medido el 2026-11-23 sobre `18af2ae` (`fix(apkc): valida package y actividad antes de adb shell`). El
puntero de `studio/` en ese momento fue `6d5c6aa` (`chore(studio): topes de descompresión en el
importador`, submódulo `e5c6dbf`) y después `30d66ea` (`fix(studio): topa la lectura de clases para
autocompletar`); **`6d5c6aa` y su submódulo `e5c6dbf` son estado histórico**: este checkout ya no los
resuelve (`git -C studio show 6d5c6aa` da `unknown revision`). En **esa** corrida el puntero apuntaba a
`30d66ea`; el HEAD de hoy ya está en otro sitio y se mueve con cada integración de `studio/`.

```sh
# en apkc/
tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestValidaPaqueteYActividad|TestRunRechazaManifiestoInyectadoAntesDeLaShell' -v .
# en studio/
node prueba.js
```

→ `ok … 0.028s` y `prueba: todo en orden`.

**Inyección de shell.** El `package` y la actividad del manifiesto se validan **antes** de construir
cualquier guion, en `adb` y en `rish` (`apkc/manifest.go`; la llamada, en `apkc/device.go:328`), así que
un `x; reboot` escrito en el manifiesto ya no llega a la shell.

**«Zip bombs».** Las tres lecturas se cortan por **bytes producidos**, no por bytes comprimidos, y
cancelan en cuanto se pasan del tope: `inflarConTope` (`studio/proyecto.js:121-142`, que es quien
descomprime y llama a `lector.cancel()`), `extraerJarDeZip` (`studio/pestana.js:2727`, que delega en
`inflarConTope` con `MAX_JAR`) y el autocompletado, con 8 MiB por `.class`
(`studio/autocompletado.js:37`). `leerZip` es el que recorre el directorio central
(`studio/proyecto.js:145`) y corta por entrada con `MAX_ENTRADA` (`studio/proyecto.js:175`).

**Falsificaciones.** Quitar la validación deja que `x; reboot` llegue a la shell; y con
`if (false && producido > tope)` la bomba pasa entera y la corrida se va a 159 MB de RSS.

**Fronteras de este `§`.** Siguen abiertas, algunas en curso: el SourceStamp truncado, `verify` con
`ReadAll`, el `storepass` en `argv`, el temporal predecible, la CSP con Trusted Types y las Actions por
tag. No se tocó red ni publicación.

**Actualizado el 2026-11-23 en `d5efe3d`** (`fix: cierra las colas de seguridad de la auditoría`): tres
de esas fronteras se cerraron **después** de la medición, así que esta nota sustituye a las tres y el
resto del `§` sigue describiendo el árbol de `18af2ae`.

```sh
# en apkc/
tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestTemporal|TestEmpaquetar|TestSign|TestResuelve|TestArgumentos|TestSigningKeyAcepta' -v .
sh -n tools/sitio.sh
```

**Resultado**: 16 tests en verde y `ok … 0.322s`; `sh -n tools/sitio.sh` sale limpio; la suite completa,
`ok … 85.180s`, y el wasm compila.

**Qué se cerró.** La **CSP con Trusted Types**: `tools/sitio.sh` añade `trusted-types apkc-studio` a la
cabecera del service worker y al `<meta>`, y falla si falta (`tools/sitio.sh:132-140`). El **temporal
predecible**: `apkc/paquetetemporal.go:20` lo crea con `os.CreateTemp` junto al destino (`.apkc-*`), y
`empaquetar.go` y `sign.go` lo borran si el `rename` no llega (`apkc/empaquetar.go:61-64`,
`apkc/sign.go:195-198`). El **`storepass` en `argv`**: `resolverPassword` acepta `file:<ruta>`
(`apkc/paquetetemporal.go:30`), `apksigner` recibe `--ks-pass file:` (`apkc/apk.go:75`) y el vigilante
`--storepass file:` (`apkc/device.go:565`), con el fichero 0600 borrado tras usarlo.

**Falsificaciones.** Quitar la directiva CSP hace fallar `tools/sitio.sh` con rc 1; volver a
`os.Create(ruta+".tmp")` deja el enlace plantado y escribe en la víctima; y volver a `pass:` devuelve la
contraseña a la línea de órdenes.

**Sigue abierto.** Las Actions por tag, pendientes de red (`.github/workflows/ENDURECER.md`). Lo demás de
esta lista se cerró después en `a9043db`: el SourceStamp truncado, `verify` con `ReadAll` y los otros
`ruta+".tmp"` de `gopackage.go`, `aab.go`, `aab_go.go` y `build.go`, con sus tests.

## 238. Hot swap en el intérprete: identidad, estado y despacho

Medido el 2026-11-23 sobre `e5af669` (`feat(preview): primitiva de hot swap con identidad conservada`).

```sh
# en apkc/
tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestElHotSwapConservaIdentidadYEstadoYCambiaElDespacho|TestElFlagDeHotSwapParteClaseYMetodo|TestSinElEngancheDeHotSwapElMetodoViejoSigueRespondiendo|TestElHotSwapEsAditivoYNoTocaOtrosMetodos' -v .
```

→ `ok … 0.050s`.

**Qué garantiza.** En una sesión viva, la identidad del objeto y su estado se conservan y el despacho
pasa al método nuevo; el reemplazo es **aditivo**, sin tocar otros métodos; y los estáticos se siembran
solo si faltaban. Cada `metodoDalvik` lleva su propio dex para no mezclar los índices de dos ficheros.

**Falsificación.** Sin el enganche, `suma()` devuelve `204` en vez de `404`.

**Suite y wasm.** La suite completa dio `ok … 82.019s` y el wasm compila.

**Fronteras de este `§`.** No se puede cambiar la forma de los campos, y la UI queda pendiente: pasar
los bytes del dex y pintar `ejecucion.intercambios`. No se tocó red ni publicación.

**Nota del gate.** El gate de `§230` se corrió **antes** de estos tres frentes —la verificación ampliada
de ART, los arreglos de seguridad y el hot swap—, así que no se puede leer como su verificación;
repetido en `§242` sobre el árbol final, verde.

## 239. Los records: `invokedynamic` desugrado a métodos directos

Medido el 2026-11-23 sobre `5f04c29` (`feat(dex): desugra los records de invokedynamic`). Hasta este
commit un `record` caía en el rechazo nominado de `§235`; ahora el bootstrap `ObjectMethods.bootstrap` se
desugra en la clase dueña, sin `d8` y sin exigir `min-api` 34.

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  JAVAC17=/home/leonisauro/opt/jdk17/bin/javac GOMAXPROCS=2 GOGC=50 \
  go test -p=1 -count=1 -run 'TestRecordSeDesugra|TestRecordYLambdaEnElMismoDex|TestRecordTagNoSeDuplica|TestIndyDeRecordFueraDeSuClaseSeRechazaConNombre' -v .
```

**Resultado**: los 4 tests en verde y `ok … 0.175s`; la suite completa del builder dio `ok … 100.297s`,
con `gofmt` y `go vet` limpios.

**Los cinco pasos.** (1) `sitioDeRecord` (`apkc/record.go:71`) reconoce el bootstrap y **conserva el
rechazo nominado** para los demás; (2) `$record$getFieldsAsObjects` se genera con boxing por tipo;
(3) `toString`, `hashCode` y `equals` se escriben directos en la clase dueña; (4) el `p.record` queda con
`invoke-direct` + `move-result`; y (5) se añade la `RecordTag` sintética sin pedir `min-api` 34
(`apkc/record.go:29`, `apkc/lambda.go:158`).

**Oráculos.** `javap -v` del bootstrap `ObjectMethods.bootstrap` con sus argumentos (`Class`, `"x;y"` y
un `REF_getField` por componente); `d8 --min-api 26` y `dexdump` sobre el resultado; y ART en Droidloom
(`10.177.0.2:5555`) con `RecordMain`, `MixMain` (diez tipos) y `LocalRec`, con salida **idéntica a la
JVM**.

**Una decisión medida, no copiada.** `hashCode` sigue a la JVM —33 para `Punto(1,2)`— y no a `d8`, que da
160117539 y hace reventar `equals(null)`; `equals` añade guarda de nulo. `getFieldsAsObjects` usa
`new-array` + `aput-object` (22 unidades frente a 17 de `d8`).

**Falsificaciones.** Neutralizar `sitioDeRecord` devuelve el rechazo viejo; quitar la caja del `int` da
`VerifyError`; y cambiar `invoke-direct` por `invoke-virtual`, otro `VerifyError`.

**Fronteras de este `§`.** El multidex con la `RecordTag` en `extras` no está medido. No se corrió gate,
chromium, red ni publicación.

## 240. Los puntos de unión: el reparto de registros reconciliado

Medido el 2026-11-23 sobre `eb680b3` (`fix(dex): reconcilia los registros en los puntos de unión`).

```sh
cd apkc && ANDROID_HOME=/home/leonisauro/opt/android-sdk \
  APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar \
  GOMAXPROCS=2 GOGC=50 go test -p=1 -count=1 -run TestLosPuntosDeUnionIgualanElReparto -v .
```

**Resultado**: `PASS` y `ok … 0.006s`; la suite completa del builder, `ok … 81.058s`.

**El defecto era de correctitud, y silencioso.** `ternario(true, 1, 2)` devolvía **2** en ART (javac da
1): el dexer emitía `if-eqz v0,0003 / goto 0003 / return v2`, con las dos ramas devolviendo `b`, y el
fichero pasaba `dexdump` sin queja. Lo mismo con `long`, `float`, `String` y el ternario invertido; un
lote con `&&`, ternario anidado, bucle y `return` antes de la unión llegó a colgarse (`rc=137`).

**El arreglo.** `apkc/dexunion.go` recorre el cuerpo por pc con `comienzosDe` (`apkc/dexunion.go:362`,
la misma regla que `jvmframes.go:1105-1135`), fija un reparto canónico en cada comienzo y manda los
saltos que llegan con otro reparto a un bloque **fuera de línea** detrás del cuerpo, con
`move`/`move-object`/`move-wide` y `goto/32`; los bordes de vuelta se igualan antes de la branquia, y los
locales no entran porque su registro es estado global (`renom`). El enganche vive en `dexinstr.go`
(`escribeBranquia`, `parchaSaltos`, `cierraCargas`, `escribePads`).

**Oráculos.** `javac -g`, `d8 --min-api 21`, `dexdump -d` y ART en Droidloom (`10.177.0.2:5555`) con
siete sondas —los seis tipos y el `inverso`— y un `switch` de Java 17: antes `33,33,33` y `6,6`;
después `11,22,33` y `5,6`, todo coincidente con `javac` y rc 0 sin `VerifyError`.

**Falsificación.** Desactivar la reconciliación devuelve los seis valores equivocados, con sus registros
y sus unidades.

**`"" + x`, la frontera del §236.** La hipótesis del `nothing to drop` **no se reproduce** con el binario
de HEAD en cuatro tipos: el síntoma era el mismo valor equivocado del punto de unión, y lo cerró esta
misma primitiva.

**Fronteras de este `§`.** Un borde de vuelta que llega a la cabecera de un bucle con **otra altura de
pila** no cambia el plan de registros —no hay registro que inventar ni que tirar sin una instrucción—:
`2940e86` deja de abortar con «stacks of different height» y lo anota como `fronteraUnion` con su
mensaje, en las dos direcciones; el `istore` de la cabecera se funde con el registro del `const/4` y el
bucle sale bien en ART (`new Punto(1,2).hashCode()` da 33). No se corrió gate, chromium, red ni
publicación en esa medición; **el gate de `§230` es anterior a este arreglo y no lo cubre**, y el
repetido en `§242` sobre el árbol final salió verde.

## 317. Apps de ejemplo: firma, alineación y ejecución en Droidloom verificadas

Medido el 2026-11-24 sobre `09a05cf`: el comando 6 viene del informe del investigador
`ses_defe9114affeVMfAoQoVT0LwSY` y los cinco comandos sobre el cell, de la medición del QA
`ses_defe581b2ffeiFhf5cMQod0TJ0`. No se recompilaron los APKs: hay un builder escribiendo en `apkc/`.
Los oráculos del wrapper son el `apksigner` y el `zipalign` de build-tools 36.0.0, con Go fijado en
1.24.0 (§312).

**Comando 6, en verde:**

```sh
sh tools/correr.sh -- sh -c 'for a in lienzo notas chatbt tareas cronometro; do apksigner verify --verbose "$HOME/.cache/apkc-web/apps-ejemplo/$a.apk"; zipalign -c -p 4 "$HOME/.cache/apkc-web/apps-ejemplo/$a.apk"; done'
```

**Resultado**: para los cinco APKs (`lienzo`, `notas`, `chatbt`, `tareas`, `cronometro`), `Verifies`,
v1/v2/v3 `true`, v3.1/v4 `false`, SourceStamp `false`, `Number of signers: 1`, rc=0 en `apksigner`
y rc=0 en `zipalign`. La salida archivada está en
`~/.cache/apkc-web/apps-ejemplo-medicion/cmd6-salida.txt`.

**Arranque del cell.** `droidloomctl status --json` →
`{"message":"Droidloom is stopped","ok":true,"state":"stopped"}`; `droidloomctl start` →
`Droidloom is running; Android is ready`; de nuevo `status --json` →
`{"ok":true,"state":"running","message":"cell u1000 is running from /etc/droidloom/cell.json; Android is ready","host_pid":178471}`.

**GNOME.** `gnome-extensions enable auto-move-windows@…`;
`gsettings set … application-list "[17 entradas droidloom-*.desktop:3]"`; `gsettings get` devuelve las
17; `gnome-extensions info` → `State: ACTIVE`; `num-workspaces=3` y `dynamic-workspaces=false`.

**Dispositivo y apps.** `adb devices -l` →
`10.177.0.2:5555 device product:aosp_cf_x86_64_only_phone model:Droidloom device:droidloom transport_id:1`;
`wm size` → `1920x1080`; `wm density` → `213`; `pm list packages com.ejemplo` → presentes `lienzo`,
`notas`, `chatbt`, `tareas` y `cronometro`; `droidloomctl launch … --user 0 --json` → `"ok": true`
para cada app.

**Cinco comandos, todos medidos en verde.**

1. `screencap -p` → PNG `1920 x 1080`, 8-bit RGBA, `10608 bytes`.
2. `am force-stop com.ejemplo.notas` + `am start -n com.ejemplo.notas/.MainActivity` →
   `Starting: Intent { cmp=com.ejemplo.notas/.MainActivity }`, `pidof` → 1707.
3. Igual para `com.ejemplo.tareas` → `pidof` 1822.
4. `am start -n com.ejemplo.cronometro/.MainActivity` → `pidof` 1869.
5. `bash -c 'set -o pipefail; adb … shell dumpsys bluetooth_manager | head -40; echo ADAPTER_RC=$?'`
   → `Bluetooth Status: State: OFF`, `Bluetooth Service not connected`, `ADAPTER_RC=0` (rc real, sin
   enmascaramiento).

**Cierre.** `droidloomctl stop` → `Droidloom is stopped` y `status --json` → `stopped`. El workspace 3
quedó configurado según la skill `droidloom-launch`.

**Fronteras de este `§`**: fidelidad píxel a píxel aparato↔preview sin medir; el adaptador Bluetooth
está apagado (dato de la medición 5, no fallo); sin gate, chromium ni publicación; `e2e.sh` sigue en
verificación.

## 318. Fase K re-medida sobre el árbol final: recursos de AAR y dedupe de claves

Medido el 2026-11-24 sobre el árbol final (F0, re-medición de Fase K). Se cierran, re-medidas aquí,
las dos fronteras que ya tenían medición en §233 y §256/§280: los tipos de recurso que faltaban en
§216 (`drawable` XML, `anim`, `font` y `values`) y dos AAR con la misma clave `values`/`styleable`.

```sh
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestPreviewRecursosDeAarDrawableAnimYFont|TestPreviewVeElDrawableXmlDeLaDependencia|TestDedupeDeValuesPorClave|TestDedupeDeStyleablesPorClave' -v .
```

**Resultado**: cuatro `--- PASS` (más subtests), `ok github.com/ajinasokan/apkc 1.372s`, 0 saltados.

**Fronteras**: el `R.txt` del AAR sigue sin fusionarse: `R.java` se regenera del enlace (medido con
un campo fantasma que solo está en el `R.txt` del AAR). La fusión del manifiesto se mide en §319 y la
marca de extracción `e3` queda incierta para I3/I4/I5. Sin gate, chromium, red ni publicación: la
medición es nativa.

## 319. Manifiesto del AAR: fusión de hijos, prioridad y rechazo de `tools:`

Medido el 2026-11-24 (F1). El `AndroidManifest.xml` extraído de cada AAR se fusiona con el del
proyecto: se juntan los hijos de `<manifest>` y de `<application>`, con prioridad proyecto > isla >
AAR y un `LogW` cuando colisionan. Una directiva `tools:` se rechaza por su nombre, en vez de
escribir un manifiesto que diga algo que no se aplicó. El `R.txt` no se fusiona: `R.java` se regenera
del enlace.

```sh
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestAarEnJarAportaRecursosYNativos|TestManifiestoDeAar|TestManifiestoDeAarRechazaTools|TestApkConDosAarsLlevaUnSoloSo|TestCompilarEnGoInvalidaElReusoSiCambiaUnaDependencia' -v .
```

**Resultado**: `PASS`, `ok github.com/ajinasokan/apkc 0.764s`, con `apksigner`, `zipalign` y `unzip`
presentes y rc 0.

**Frontera incierta**: `versionDeExtraccionAar = "e3"` queda **incierto** para I3/I4/I5: ningún test
demuestra la invalidación de cachés; no se afirma. Sin gate, chromium, red ni publicación.


## 329. Coste real de `check`: medianas en nativo y en la pestaña por CDP

Medido el 2026-11-24 sobre el frente `d2-d6-diagnostico` **reconciliado en `1a60875`** (incluye
`6496918`). D2 salía de las cifras de D1 (~15 / ~30 / ~200 ms) y de las transferidas de §232 e I5;
aquí se miden con reloj de pared las tres formas que ve el editor. La sonda nativa es
`apkc/check_tiempo_test.go` (nueva, con 11 repeticiones) y la de la pestaña es `studio/sonda-check.js`
(3 repeticiones). No se reutiliza ninguna cifra de §214, §232 ni I5 como propia.

```sh
# wasm para la sonda de pestaña
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/d2/apkc-wasm.wasm" .
# sonda nativa sobre el árbol reconciliado (tres tests, cada uno con su mediana)
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestCheckTiempo|TestMarcoCacheadoMideElAhorro|TestIndiceDelJarMideElAhorro' -v .
# sonda de la pestaña: ejecutada antes de la reconciliación, NO reejecutada aquí
ANDROID_JAR=… APKC_WASM="$PREFIX/tmp/d2/apkc-wasm.wasm" CHROME_BIN=… node studio/sonda-check.js
```

**Salida nativa (literal, árbol reconciliado).** `TestCheckTiempo` imprime sus medianas y `ok
github.com/ajinasokan/apkc 3.518s`; los otros dos tests pasan con sus propias medianas:

```
medida check_sin_proyecto_ms 7.7
medida check_sin_R_enlazando_res_ms 190.3
medida check_con_proyecto_y_R_ms 7.3
resumen: sin proyecto 7.7 ms; sin R enlazando res 190.3 ms; con proyecto y R 7.3 ms
medianas: archive/zip 12.5 ms; lector propio con la caché vacía 5.5 ms; con la caché puesta 2.0 ms; proyecto mínimo 10.7 y 7.7 ms
medianas: sin caché 174.9 ms; con la caché 6.4 ms
```

Segunda ejecución del mismo test sobre el mismo árbol, `ok github.com/ajinasokan/apkc 2.877s`:

```
medida check_sin_proyecto_ms 6.4
medida check_sin_R_enlazando_res_ms 187.5
medida check_con_proyecto_y_R_ms 27.4
resumen: sin proyecto 6.4 ms; sin R enlazando res 187.5 ms; con proyecto y R 27.4 ms
```

El auditor midió **20,0 / 261,6 / 28,5 ms** en el mismo escenario. La cifra de `check con proyecto y
R` se reporta por tanto como **rango 7,3–28,5 ms** (27,4 ms en la segunda corrida aquí), no como un
punto: las cuatro ejecuciones quedan por debajo de los 30 ms del objetivo histórico, pero con un
margen que el ruido del escritorio puede estrechar. La aserción `<30 ms` se retiró por eso (estaba a
1,5 ms del margen) y se sustituyó por una falsificación robusta: el camino con `R` no puede costar
la mitad que enlazar `res/` (`conR < sinR/2`), que sí se cumple (7,3–27,4 frente a 187,5–261,6 ms).
Enlazar `res/` sin `R.java` sigue siendo una orden de magnitud más caro. Las dos últimas líneas de la
primera salida son los tests de §232 e I5 reejecutados aquí: confirman el ahorro del marco
(174,9 → 6,4 ms) y del índice del jar, pero sus cifras históricas siguen siendo transferidas.

**Salida de la pestaña (literal, medida antes de la reconciliación; no reejecutada por la
prohibición expresa de CDP/chromium en la corrección).** La página arrancó aislada y la sonda
salió con rc 0:

```
medida preparacion {"create":0,"jar":26361808,"fuente":387}
medida version_ms [80.00999999977648,163.53999999910593,178.2949999999255]
medida check_sin_proyecto_ms [245.88000000081956,323.785000000149,392.85999999940395]
medida check_sin_R_enlazando_res_ms [710.5850000008941,720.0750000011176,753.7249999996275]
medida caro_dos_checks_esperando_ms 1041
medida caro_dos_checks_cancelando_ms 818
medida cancelacion_disponible true
medida build_deja_R {"codigo":1,"r":399}
medida check_con_proyecto_y_R_ms [218.839999999851,241.5200000014156,252.81000000052154]
medida rapido_dos_checks_esperando_ms 317
medida rapido_dos_checks_cancelando_ms 746
medida diagnostico_saltos {"pasadas0":1,"saltadas0":0,"pasadas1":1,"saltadas1":1,"trasEspacios":"sin errores · Java","pasadas2":2,"corrio":true}
resumen {
  "version": 163.53999999910593,
  "sinProyecto": 323.785000000149,
  "conR": 241.5200000014156,
  "sinR": 720.0750000011176,
  "caroEsperar": 1040.7249999996275,
  "caroCancelar": 818.3700000010431,
  "rapidoEsperar": 317.08500000089407,
  "rapidoCancelar": 745.6199999991804,
  "reps": 3
}
```

**Lo que entra en §332.** `diagnostico_saltos` falsa el salto: `pasadas1 === pasadas0` al añadir solo
espacios y `pasadas2 > pasadas0` al cambiar `R.layout.main` por `R.layout.noexiste`.
`build_deja_R` dejó 399 bytes de `R.java`. La decisión de §332 no se toma de una sola corrida:
las tres mediciones de la sonda de pestaña dieron medianas de `version_ms` 84,6 / 84,4 / 163,5 ms
y de `check_con_proyecto_y_R_ms` 347,9 / 836,5 / 241,5 ms, con razones 24,3 % / 10,1 % / 67,7 %.
Por debajo del tercio en dos y por encima en una: la señal no es estable y se declara **incierta**
en §332 en vez de improvisar una sesión persistente. Esas tres corridas son anteriores a la
reconciliación con `6496918`; no se ha medido de nuevo en la pestaña sobre el árbol reconciliado.

**Fronteras de este `§`.** Once repeticiones por cifra en nativo y tres en la sonda de pestaña
(anteriores a la reconciliación), en un solo escritorio: no se extrapola a un teléfono. `§214`, §232
e I5 se citan como transferidos y en verificación. No se corrió el gate final, no se publicó, no se
tocó red externa y **no se ejecutó CDP/chromium sobre el árbol reconciliado** por la restricción
expresa de la corrección.

## 330. Asignación cualificada: receptor `this` y receptor con tipo

Medido el 2026-11-24 sobre el frente `d2-d6-diagnostico`. D4 tenía el código y los tests escritos
desde antes (`apkc/check_test.go:205-334`, `apkc/javaemisor.go:8526-8542`) pero sin `§` propio: leerlo
no bastaba para marcarlo hecho (regla 11). Se ejecuta su comando sobre el árbol final.

```sh
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestCheckAsignacionCualificada|TestCheckAsignacionPorReceptor|TestCheckAsignacionReceptorSinTipo|TestAsignacionCualificadaContraJavac17' -v .
```

**Resultado literal**: seis `--- PASS` y `ok github.com/ajinasokan/apkc 1.983s`, sin saltados.

```
--- PASS: TestCheckAsignacionCualificadaThis (0.20s)
--- PASS: TestCheckAsignacionCualificadaNombraElCampo (0.19s)
--- PASS: TestCheckAsignacionPorReceptor (0.20s)
--- PASS: TestCheckAsignacionPorReceptorNombraElCampo (0.18s)
--- PASS: TestCheckAsignacionReceptorSinTipo (0.18s)
--- PASS: TestAsignacionCualificadaContraJavac17 (0.99s)
```

`this.filePath = filePath` sale con `"errores":[]` y `emisor:true`; el control `this.noExiste` nombra
el campo y cae en `Apk.java:7` sin el mensaje «mas de un nombre en el destino»; `appInfo.sourceDir =
filePath` da cero errores; y el receptor sin tipo dice «no se conoce el tipo del receptor appInfo».
`TestAsignacionCualificadaContraJavac17` confirma el dex byte a byte contra javac 17, así que la
resolución no movió la `LineNumberTable`.

**Fronteras de este `§`.** Sin gate final, chromium ni publicación. El código residía en `HEAD
4b723ae`: aquí se mide, no se reescribe.

## 331. La columna del parser apunta al token, no a una línea inventada

Medido el 2026-11-24 sobre el frente `d2-d6-diagnostico`. La infraestructura ya estaba en
`apkc/check.go:42-103` (`posicionEnMensaje`, `limpiarMensaje` y `posicion` aceptan `linea N, columna
C:`) y el token de fin de fichero ya guardaba su columna en `apkc/javafuente.go:705-713`; lo que la
descartaba era `nombrePunto` (`:999`), que formateaba solo `linea %d`. Ese sitio usa ahora
`t.donde.sitio()`. Un intento de cambiar también `comer` (`:923`) se revirtió: rompía
`TestPorCadaFalsificacion` y `TestTipoCualificadoLocalFalsificacion`, que comparan dos caminos y no
debían ver columna en un mensaje ajeno a este frente.

```sh
cd apkc && tcr -c 2 -m 1536M -r 256M -- go test -count=1 \
  -run 'TestCheckBloqueSinCerrarApuntaALaUltimaLinea|TestCheckColumnaEnCaracteres' -v .
```

**Resultado literal**: los dos `PASS` y `ok github.com/ajinasokan/apkc 0.228s`. El bloque sin cerrar
pasa de `MainActivity.java:5` a `MainActivity.java:5:C` con `C>0`, y el mensaje sale limpio (sin
`linea 5`). `TestCheckColumnaEnCaracteres` sigue contando la columna en caracteres (runes), con un
emoji delante del token, no en bytes ni en unidades UTF-16.

**Falsificación ejecutada.** Poner la columna del token de fin de fichero a cero
(`pos{linea: finLinea, columna: finColumna * 0}` en `apkc/javafuente.go:713`) rompió el test con
`la posición es "MainActivity.java:5", medida "MainActivity.java:5:C" con C>0` y `la columna es 0 y
el error nació en un token`; al revertirlo volvieron los dos `PASS`. No se tocó la
`LineNumberTable`: el dex se compara byte a byte en §330.

**Fronteras de este `§`.** Los errores del emisor que no nacen de un token siguen sin columna, que es
la forma exigida; no se inventa una columna para ellos. Sin gate final ni publicación.

## 332. Overhead de arranque: decisión incierta, sin sesión persistente

La regla de F4 se fijó antes de medir: si la mediana de `version_ms` es ≥ ⅓ de la de
`check_con_proyecto_y_R_ms`, se implanta una sesión persistente de `check`; si no, se documenta que
el arranque no justifica el cambio y se pide la decisión al usuario, sin cerrar D5. Las tres corridas
de la sonda de pestaña (§329) dieron **24,3 % / 10,1 % / 67,7 %**: dos por debajo del umbral y una
por encima, y todas son **anteriores a la reconciliación con `6496918`/`1a60875`**. La señal no es
estable, así que **la decisión queda `incierta`**: no se implanta `apkc check --sesion`, no se
conecta `studio/diagnostico-vivo.js` a un id de sesión y **D5 no se da por hecha** (regla 11). La vía
siguiente concreta es repetir `node studio/sonda-check.js` con `REPES=7` en una sesión sin otros
gates ni CDP concurrentes; si entonces supera el tercio, se implanta la sesión y se falsifica con
`instanciasWasm === 1`.

La prueba de la pestaña se ejecutó con el wasm de **antes de la reconciliación**:

```sh
APKC_WASM="$PREFIX/tmp/d2/apkc-wasm.wasm" node studio/prueba-pestana.js
```

**Resultado literal de esa corrida**: `240 comprobaciones bien, 3 mal`. Las tres son de D7
(navegación entre actividades: «un clic en id/abrir abre la segunda actividad…», «la segunda pantalla
tiene un rectángulo pulsable para id/volver» y «el recorrido de dos actividades se ve igual…»).
**Se retira la afirmación anterior de que eran «preexistentes»**: `6496918` (`fix(sesion): navega
entre actividades y publica el recorrido de pantallas`, ya integrado en `1a60875`) arregla D7
añadiendo `pantallas` al modelo, y no hay medición suya sobre ese árbol que permita llamarlas
preexistentes ni afirmar que desaparecieron. En aquella corrida pasaron las comprobaciones de este
frente (`diagnostico_saltos` con `pasadas1 === pasadas0` y `pasadas2 > pasadas0`, linter XML
etiquetado `XML`, contextos Java/XML sin cruzarse y `la pestaña no pide nada fuera de su origen (509
peticiones, 0 websockets)`), pero eso no prueba el árbol reconciliado.

**Estado de la reconciliación: `incierto`.** Motivo literal: la corrección impone «Sin
CDP/chromium/red/gate/push/publicación», de modo que **no se reejecutó `prueba-pestana.js` sobre
`1a60875`**. La vía siguiente es ejecutarla con el entorno completo (`KS_RELEASE`, `AAB_EJEMPLO`,
`APKC_WISP_EXTERNO`, `ANDROID_JAR`) en cuanto se autorice CDP; solo entonces se podrá escribir
`pestaña: N comprobaciones, 0 mal` o documentar el fallo que quede.

**Fronteras de este `§`.** No se corrió el gate final, no se publicó, no se tocó red externa ni se
usó chromium en la corrección. D5 permanece **en verificación** a la espera de la mediana estable y
de la decisión del usuario.

## 333. Matriz de cobertura Java/XML con estado medido

Medido el 2026-11-24. La matriz vive en `docs/plan-diagnostico.md`; `studio/prueba.js` imprime una
línea por fila desde los módulos puros y `studio/prueba-pestana.js` comprueba el pintado en chromium.

```sh
# cwd = raíz del worktree
node studio/prueba.js
APKC_WASM="$PREFIX/tmp/d2/apkc-wasm.wasm" node studio/prueba-pestana.js
```

**Salida literal de la matriz en node**, dentro de un `prueba: todo en orden`:

```
matriz 1 Java R.layout y R.id: ok
matriz 2 Java View. y v. sin adivinar: ok
matriz 3 Java this.campo y prefijo Acti: ok
matriz 4 Java comentario y cadena: ok
matriz 5 XML < en layout, values y manifiesto: ok
matriz 6 XML android: y valores enumerados: ok
matriz 7 XML @strin y fila del linter: ok
matriz 8 errores del panel: comprobada en prueba-pestana.js por CDP
```

La matriz se reejecutó sobre el árbol reconciliado (submódulo `a41db75`, con la matriz D6 y el
bombeo del cronómetro) con el mismo `prueba: todo en orden` y las ocho líneas anteriores.

En la corrida CDP **anterior a la reconciliación**, las filas comprobadas incluyeron
`el contexto no se cruza: ni etiquetas XML en .java ni R. en .xml`,
`el linter de XML marca @string/noexiste con archivo:línea:columna y origen XML`
(`{"posicion":"xmlauto.xml:2:27", …}`), `al pasar a .java el error XML desaparece y vuelve el
diagnóstico de Java` y `el diagnóstico en vivo marca el error con la vista previa abierta`; el global
fue `240 comprobaciones bien, 3 mal`. **No se reejecutó sobre `1a60875`**: la corrección prohíbe
CDP/chromium, así que la fila 8 queda `incierto` en el árbol reconciliado (motivo en §332) y no se
afirma que los tres fallos D7 desaparecieron ni que son preexistentes.

**Fronteras de este `§`.** Las filas 1-7 están medidas en node sobre el árbol reconciliado; la 8 queda
**en verificación** por la restricción de CDP. No se anuncia cobertura completa por analogía. Sin
gate final, publicación ni red externa.
