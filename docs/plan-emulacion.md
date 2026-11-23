# Plan de emulación de apps propias en la vista previa

**Estado: plan abierto.** E1 (ciclo de vida y navegación entre actividades propias) está **hecha** y
medida en §205, con sus correcciones de auditoría (E1.1) cerradas en §206; E3 y E4 están
**cerradas** en §207 y §208; E2 está **cerrada** en §209; E5 está **integrada y en verificación**
en §210; E6 y E7 siguen **abiertas**. Este fichero es un plan, no evidencia: lo que aquí se afirme
sobre coste, compatibilidad o formato es una hipótesis y lleva el comando con el que se comprobará.
Las mediciones reales van a `docs/evidencia.md`; el estado de las fases, a la tabla de `docs/plan.md`
cuando la fase se cierre.

Fecha: 2026-11-22. Origen de la petición: el usuario quiere **que emule mucho más**, con prioridad
clara en **diseño de la app, funcionalidad, fidelidad y funcionamiento**. Las APIs externas de
Android o Google (Google Play Services, Firebase, Maps, Ads, permisos del sistema, descargas) quedan
**fuera** por decisión explícita: no se emulan y se rechazan por nombre.

## 1. Estado actual, honesto

Lo que hace hoy el intérprete, con el punto del árbol que lo implementa. Es la línea base sobre la
que se mide cada fase; nada de lo que sigue se da por bueno sin su comando.

### 1.1 Lectura del APK y del layout

- `apkc preview [--layout nombre] [--sin-ejecutar] [--pulsar id]… app.apk` (`apkc/preview.go:178`)
  lee el `AndroidManifest.xml`, el `resources.arsc` y un `res/layout/*.xml` del propio APK con los
  lectores del fork (`leeAXML` y `leeTabla`), y emite **una línea de JSON**: paquete, versión,
  actividad, layout, pantalla, nodos y el árbol con tipo, id, texto, tamaño declarado, `gravity`,
  `padding`, margen, peso y rectángulo (`modeloVista` en `apkc/preview.go:142`).
- La colocación aplica reglas de `LinearLayout` (orientación, `gravity`, `layout_gravity`,
  `layout_weight`, `weightSum`, `padding` y márgenes) y el apilado de `FrameLayout` y del resto de
  `*Layout` (`tipoDeContenedor`, `apkc/preview.go:1170`). `RelativeLayout` se reconoce como
  contenedor apilado, **no** se resuelve su álgebra de reglas.
- El ancho y el alto de una hoja son una estimación: alto de texto 24 dp, alto de botón 48 dp, 8 dp
  por carácter y 16 dp de relleno (`apkc/preview.go:43-50`). La pantalla del modelo es 360×640 dp a
  160 dpi.
- `--layout` elige por nombre de recurso; si la actividad ejecutada llama a `setContentView`, manda
  el id que la app escribe (`rutaDelLayoutDeLaActividad`, `apkc/preview.go:484`).
- Un `android:onClick="metodo"` del layout se enlaza al inflar, como el framework
  (`registraOnClickDeclarativo`, `apkc/previewinterprete.go:710`).

### 1.2 Ejecución Dalvik

- Corre **`main` o `onCreate`** de la actividad lanzadora del manifiesto
  (`ejecutaOnCreatePreparando`, `apkc/previewinterprete.go:521`), incluido el constructor `<init>`.
- `--pulsar` entrega clics **en orden, dentro de la misma ejecución**, hasta 32
  (`pulsacionesMaximas`, `apkc/previewinterprete.go:47`). Un clic cuya vista no tiene oyente devuelve
  error con código 2. El estado que ve cada `onClick` incluye los clics anteriores, y repetir la
  misma lista reproduce la misma pantalla.
- Stubs de framework reconocidos por nombre y firma (`intentaStub`,
  `apkc/previewinterprete.go:1671`): `setContentView`, `findViewById`, `setOnClickListener`,
  `setText`, `getText`, `append` (de `TextView` y de `StringBuilder`), `getString(id)`,
  `getId`, `setTag`/`getTag`, `setTitle`, `Log.*`, `print`/`println`, `String.valueOf`,
  `Integer.parseInt`, `charAt`, `length`, `isEmpty`, `equals`, `toString`.
- Los métodos de `Activity` del ciclo de vida **solo se absorben como stubs** cuando el `invoke` los
  resuelve contra `Landroid/app/Activity;` (`apkc/previewinterprete.go:1704`). Si la app **sobreescribe**
  `onStart`, `onResume`, `finish`, etc., el intérprete **no los llama**: no hay ciclo de vida.
- No hay `startActivity`, ni `Intent` propio, ni pila de actividades: una sola pantalla.
- Lo que no se emula se dice por su nombre y se devuelve el cero del tipo de retorno, sin romper el
  modelo (`rechaza`, `apkc/previewinterprete.go:1659`): `Editable`/`Spannable`, `Html.fromHtml`, la
  reflexión y `getString(id, args)` con formato.
- Un método con tabla de excepciones (`triesSize > 0`) **no se ejecuta**: el intérprete lo rechaza
  (`apkc/previewinterprete.go:792`). El subconjunto Java del compilador sí emite `try`/`catch`, pero
  el intérprete de la vista previa todavía no los corre.
- Límites: 200 000 pasos, 64 capturas, 24 avisos y 48 llamadas anidadas
  (`apkc/previewinterprete.go:39-50`).

### 1.3 Lo que la pestaña ya hace con ese modelo

- El modo **Emulador** pinta el árbol en un `<canvas>` con los colores del tema
  (`pintarModelo`, `studio/vista-previa.js:226`), marca las vistas con oyente y entrega el clic
  reejecutando `apkc preview --pulsar` con **la lista completa de clics** desde el principio
  (`studio/previa-panel.js:945`). Esa repetición es la que hace determinista la navegación futura:
  no hay que inventar un lenguaje nuevo para ir de una pantalla a otra.
- Tres secciones por superficie: Pulsar, Registro y Consola. La consola de la app sale de
  `print`/`println`/`Log.*`, en orden de emisión, sin reloj.

### 1.4 Lo que hoy no existe y este plan ataca

| Falta | Consecuencia para el usuario |
| --- | --- |
| `onStart`/`onResume`/`onPause`/`onStop`/`onDestroy` propios | Una app que pone texto o registra oyentes fuera de `onCreate` se ve incompleta |
| `startActivity` entre actividades propias y `finish()` | No se puede recorrer una app de varias pantallas |
| `ListView`/`RecyclerView` con adaptadores del proyecto | No se ve ninguna lista, que es la mitad de una app real |
| `SharedPreferences`, `Bundle`/`savedInstanceState` | El estado no sobrevive a la rotación ni a cerrar la pantalla |
| `visibility`, `textSize`, colores, estilos e imágenes propias | La pantalla se parece poco al diseño real |
| `EditText`, `CheckBox`, `RadioButton`, `Switch` con estado | No se puede probar un formulario |
| `Handler`/`postDelayed`/`AsyncTask` | No se ve una animación ni un contador temporizado |

## 2. Objetivo

Que la vista previa ejecute **apps propias** (las que produce `apkc-studio`, un `classes.dex` sin
`.so`, sin multidex y con las APIs de Google fuera) y muestre con fidelidad suficiente para juzgar
**diseño, funcionalidad y funcionamiento**: recorrer sus pantallas, ver listas, escribir en campos,
marcar casillas, conservar el estado al recrear la actividad y ver temporizadores deterministas.

No es un emulador de Android ni corre APKs de Play Store. No sustituye a un aparato: sustituye la
pregunta «¿cómo se verá y cómo se comportará mi app?» por una respuesta que se puede repetir en la
pestaña, sin red y sin runtime ajeno.

## 3. Frontera explícita

Se **rechazan por nombre**, con aviso en `ejecucion.avisos` y sin inventar su efecto:

- Google Play Services, Firebase, Google Maps, Ads y cualquier SDK de Google.
- Permisos del sistema y APIs que dependen del usuario o del aparato: cámara, ubicación, contactos,
  SMS, telefonía, Bluetooth, sensores, notificaciones del sistema, `ContentProvider` externo.
- Descargas, red (`HttpURLConnection`, sockets, `WebView`) y cualquier recurso fuera del APK.
- `JNI`, bibliotecas `.so`, multidex y clases cuyo cuerpo no esté en el `classes.dex`.

El comando que mide la frontera es el propio `apkc preview`: una app que llame a una de esas APIs
debe seguir produciendo su modelo con el aviso correspondiente y código de salida 0, o rechazar el
comando si los bytes del dex no son ejecutables. La prueba de aceptación de la frontera es que no se
cuela ninguna llamada de red: `studio/prueba-pestana.js` registra cada petición y falla si sale del
origen.

## 4. Fases, en el orden de valor para el usuario

Cada fase se escribe aquí **antes** de tocar código, se implementa en `apkc/` (y en `studio/` solo
si hay que mostrarla) y se verifica de la punta al gate: `go test -run …` → `tools/entorno.sh test`
→ `tools/entorno.sh wasm` → `tools/entorno.sh gate` al cerrar. `pestaña: N` no puede bajar de 204.

### E1. Ciclo de vida y navegación entre actividades propias — **HECHA**

- **Estado: hecha.** Cerrada el 2026-11-22 con `TestElPreviewNavegaEntreActividadesPropias` y
  `TestElPreviewNoAbreUnaActividadNoDeclarada` en verde, la suite nativa y la de wasm completas y el
  gate con `pestaña: 204 comprobaciones` y `e2e: todo en orden`. Evidencia detallada: §205 de
  `docs/evidencia.md`.

- **Objetivo.** Que una app de dos o tres actividades se pueda recorrer: un botón de la primera
  llama a `startActivity`, se ve la segunda, y `finish()` vuelve a la anterior conservando su
  estado. Se ejecutan `onStart` y `onResume` después de `onCreate`, y al cubrir o cerrar una
  actividad se llaman `onPause`/`onStop`/`onDestroy` si la app los define.
- **Qué se emula.**
  - `Intent(Context, Class)`, `startActivity(Intent)` y `finish()`.
  - Pila de actividades en memoria: cada actividad conserva sus objetos y sus capturas; volver con
    `finish()` no vuelve a ejecutar su `onCreate`.
  - Solo se lanzan actividades **declaradas en el manifiesto**; si no lo está, el aviso lo dice.
  - Cada pantalla aporta su `setContentView` y su layout al modelo, y la actividad actual viaja en
    `modelo.actividad`.
  - `--pulsar` sigue siendo el único mando: la lista de clics se reproduce entera desde el arranque,
    que es lo que hace determinista ir y volver.
- **Criterio de aceptación.**
  - Prueba nueva `TestElPreviewNavegaEntreActividadesPropias`: un APK de dos actividades donde el
    botón de la primera hace `startActivity(new Intent(this, Segunda.class))` y la segunda tiene su
    propio layout. Sin clics se ve la primera; con `--pulsar id/abrir` se ve la segunda, su texto y
    su actividad; con `--pulsar id/abrir --pulsar id/volver` (donde `volver` llama a `finish()`) se
    ve otra vez la primera con el estado que tenía.
  - Comando: `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewNavegaEntreActividadesPropias -v .`
  - La misma navegación es visible en el modo Emulador de la pestaña sin código nuevo: el clic
    reejecuta la lista completa. Lo prueba el gate: `tools/entorno.sh gate`, con
    `pestaña: N` ≥ 204.
- **Abandona.** `Intent` implícito, `startActivityForResult`, tareas, `singleTop`/`launchMode`,
  back stack del sistema y transiciones animadas entre actividades.

### E1.1. Correcciones de la auditoría del emulador — **CERRADA**

- **Estado: cerrada** el 2026-11-22 como `def0b85` (cherry-pick de `1e3b277` del worktree
  `emu-e1-audit`), con los siete tests enfocados en verde, `go vet` y `gofmt -l` sin salida, y el
  crítico `ses_df9f4b884ffepDlg2y2VQsIBYZ` en **APROBAR** sin defectos `CONFIRMADO`. Evidencia:
  §206 de `docs/evidencia.md`.
- **Qué corrigió.** 3.1: una actividad abierta sin `setContentView` da error y aviso, y no cae al
  layout `main` de la lanzadora. 3.2: el `setOnClickListener` programático gana al `android:onClick`
  del XML sin invertir el contrato «último registrado gana» de `pulsa`. 3.4: tope de 32 clics y tope
  simétrico de 32 pantallas, que corta una navegación circular que antes colgaba. 3.3 ya estaba
  cerrado por `81d6acf`.
- **Queda abierto.** La pregunta `INCIERTA` del crítico (redirect sin layout que vuelve por
  `finish()` del destino; `def0b85`: `previewinterprete.go:777`, `preview.go:314,336`) no se ejecutó
  como probe
  de tres actividades, y la prueba de pestaña que ejerza `startActivity` (D7) sigue pendiente en
  `docs/auditorias.md`. Matiz del crítico: los tests del tope aseveran contra `pulsacionesMaximas`,
  no contra el literal 32, así que cambiar el valor no los rompería.

### E2. Listas y adaptadores propios — **CERRADA**

- **Estado: cerrada** el 2026-11-22 como `a76422d` (integración manual de `d54fdea` desde
  `emu-e2-listas`), con `TestElPreviewPintaUnaListaConSuAdaptador` en verde y el crítico
  `ses_df9ecde81ffeVOW5GZ5bA820E6` en **APROBAR**, sin defectos `CONFIRMADO`. Evidencia, merge y
  pregunta abierta: §209 de `docs/evidencia.md`.
- **Objetivo.** Ver una lista real: `ListView` con un `BaseAdapter` o `ArrayAdapter` **del
  proyecto**, su `getView`, el `ViewHolder` y el clic por posición.
- **Qué se emula.** `setAdapter`, `getCount`, `getView`, `getItem`, `getItemId`, `notifyDataSetChanged`,
  `setOnItemClickListener`, `getChildCount`/`getChildAt` si el adaptador los usa, y la medición de
  una fila a partir del layout que devuelve `getView`. Se ejecuta el `getView` del proyecto para
  cada posición visible.
- **Criterio de aceptación.** `TestElPreviewPintaUnaListaConSuAdaptador`: un APK con un `ListView`
  y un `BaseAdapter` de tres filas produce un nodo con tres hijos, cada uno con su texto; pulsar una
  fila entrega `onItemClick` con su posición y el texto cambia. Comando:
  `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewPintaUnaListaConSuAdaptador -v .`.
- **Abandona.** `RecyclerView` con `LayoutManager` real, reciclado de vistas, `DiffUtil`,
  `ExpandableListView`, arrastrar filas y listas de más de 50 elementos.

### E3. Estado y persistencia local — **CERRADA**

- **Estado: cerrada** el 2026-11-22 como `5cc852a`, con
  `TestElPreviewConservaElEstadoAlRecrear` en verde y el crítico
  `ses_df9ef9e02ffeqSQ1cuPypAdVer` en **APROBAR**. Evidencia y comandos: §207 de
  `docs/evidencia.md`.
- **Objetivo.** Que el estado de la app sobreviva a una recreación y a cerrar la pantalla, y que el
  usuario pueda probarlo desde la pestaña.
- **Qué se emula.**
  - `Bundle`: `onSaveInstanceState(Bundle)`, `onRestoreInstanceState(Bundle)`, `put*`/`get*` con
    clave, y `savedInstanceState` no nulo al recrear.
  - Recreación: una orden nueva `--recrear` (o un pulso especial) que mata y vuelve a crear la
    actividad actual con su `Bundle`, como una rotación.
  - `SharedPreferences` propio: `getSharedPreferences`, `getString`/`getInt`/`getBoolean`/`getLong`,
    `edit`, `put*`, `apply`/`commit`, con un almacén en memoria que dura la ejecución y se puede
    volcar a JSON para reproducir. Sin ficheros fuera del modelo.
- **Criterio de aceptación.** `TestElPreviewConservaElEstadoAlRecrear`: un contador que guarda su
  valor en `onSaveInstanceState` y lo restaura en `onCreate` sigue en 2 tras dos clics y una
  recreación; y una preferencia escrita antes de recrear se lee después. Comando:
  `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewConservaElEstadoAlRecrear -v .`.
- **Abandona.** `ContentProvider`, ficheros internos/externos, `SQLite`, `DataStore`, cifrado de
  preferencias y multiusuario.

### E4. Fidelidad visual — **CERRADA**

- **Estado: cerrada** el 2026-11-22 como `99cd48f`, con
  `TestElPreviewRespetaVisibilidadTextoColorEImagen` en verde y el crítico
  `ses_df9ef9e02ffeqSQ1cuPypAdVer` en **APROBAR**. Sus dos condiciones quedaron resueltas: H4
  **cerrada** por `762b328` (el color `0x1d` de `aapt2` se lee como color) y H5 fijada como
  **frontera** (`layout_margin` desde `@style` no se resuelve). Detalle en §208 de
  `docs/evidencia.md`.
- **Objetivo.** Que el dibujo se parezca al diseño: tamaños reales por densidad, gravedad fina,
  visibilidad, tamaños de texto, colores y estilos propios, y una imagen de recurso.
- **Qué se emula.**
  - `android:visibility` (`visible`, `invisible`, `gone`) en la medición y en la pintura.
  - `textSize` en `sp` y `dp`/`px` con la densidad del modelo, y `textColor`/`textStyle`.
  - `background` con color propio (`#rrggbb`, `@color/x` del proyecto) y `ImageView` con un PNG
    propio del APK (`src`, `@drawable/x`), decodificado en Go (sin librerías).
  - `gravity` fino: `center_vertical|end`, `top|start`, `center` en contenedores y en hojas, con la
    misma tabla de bits que ya usa la colocación.
  - Estilos propios (`style="@style/x"`) resueltos desde `resources.arsc` para las propiedades que
    esta fase entienda; los del sistema quedan fuera.
- **Criterio de aceptación.** `TestElPreviewRespetaVisibilidadTextoColorEImagen`: un layout con un
  `TextView` `gone`, un `TextView` con `textSize` de 24 sp y color, y un `ImageView` con un PNG de
  2×2 produce rectángulos y un nodo `imagen` con sus píxeles; el `gone` no ocupa espacio. Comando:
  `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewRespetaVisibilidadTextoColorEImagen -v .`.
- **Abandona.** `RelativeLayout` con reglas, `ConstraintLayout`, fuentes, temas del sistema, sombras,
  `drawable` vectoriales y animaciones de vista.

### E5. Entrada de usuario — **EN VERIFICACIÓN**

- **Estado: integrada en `2dd70c8`, en verificación.** Cherry-pick de `993d51b` con 21 conflictos
  resueltos; la tanda de diez pruebas y el paquete completo están en verde, pero el crítico
  `ses_df9c6f81effeRKtmnqwkLjBU4s` aún no ha dictaminado. Evidencia y fronteras: §210 de
  `docs/evidencia.md`. Se cerrará con el follow-up del crítico.
- **Objetivo.** Probar un formulario: escribir en un `EditText`, marcar una casilla, elegir un
  radio y cambiar un `Switch`, y que la app reaccione por sus oyentes.
- **Qué se emula.**
  - `EditText`: `getText`, `setText`, `getEditableText` (texto plano), `setHint`, y una orden
    `--escribir id/texto` que entrega texto como si el usuario lo hubiera tecleado, disparando
    `TextWatcher`/`onTextChanged` propio si el proyecto lo registra.
  - `CheckBox`/`RadioButton`/`Switch`: `isChecked`, `setChecked`, `setOnCheckedChangeListener`,
    `toggle`, y una orden `--marcar id`/`--desmarcar id`.
  - `RadioGroup` mantiene una sola opción marcada.
- **Criterio de aceptación.** `TestElPreviewEscribeYMarcaLoQueSeLePide`: una app con un `EditText`,
  un `CheckBox` y un `RadioGroup` refleja el texto tecleado y el estado marcado en el modelo y en
  las capturas; `onTextChanged`/`onCheckedChanged` del proyecto corren. Comando:
  `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewEscribeYMarcaLoQueSeLePide -v .`.
- **Abandona.** Teclado en pantalla, IME, selección, autocompletado, arrastrar texto y portapapeles.

### E6. Hilos y temporizadores controlados

- **Objetivo.** Ver un contador o una animación simple sin depender del reloj real: el mismo
  resultado en cada ejecución.
- **Qué se emula.** `Handler.post`/`postDelayed`, `View.post`/`postDelayed`, `runOnUiThread` y
  `AsyncTask` (con su `onPreExecute`/`doInBackground`/`onPostExecute` ejecutados en orden) sobre una
  **cola de tareas con tiempo lógico**: cada tarea lleva su instante en milisegundos y una orden
  `--avanzar N` (o un pulso de reloj) la ejecuta cuando su instante llega. Sin `Thread.sleep`.
- **Criterio de aceptación.** `TestElPreviewAvanzaElRelojLogico`: un contador que se incrementa cada
  1000 ms con `postDelayed` vale 3 tras `--avanzar 3000`, y dos ejecuciones seguidas dan el mismo
  JSON. Comando: `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewAvanzaElRelojLogico -v .`.
- **Abandona.** Hilos reales, `java.util.Timer`, `ValueAnimator`/`ObjectAnimator` con interpoladores,
  corrutinas y cualquier espera de reloj de pared.

### E7. Diálogos y avisos propios (cola posterior)

- **Objetivo.** Que un `AlertDialog` y un `Toast` propios se vean en el modelo como una capa.
- **Qué se emula.** `AlertDialog.Builder` con título, mensaje y botones; `Toast.makeText().show()`
  como línea de consola y aviso efímero; `dismiss()`.
- **Criterio de aceptación.** `TestElPreviewMuestraUnDialogoPropio`: pulsar un botón abre un
  diálogo con su mensaje y su botón; pulsar su botón ejecuta su `OnClickListener` y lo cierra.
  Comando: `cd apkc && APKC_ANDROID_JAR=<jar> go test -run TestElPreviewMuestraUnDialogoPropio -v .`.
- **Abandona.** Diálogos del sistema, `DialogFragment` con `FragmentManager`, menús contextuales y
  notificaciones del sistema.

## 5. Criterio de abandono del plan

El plan **no se cierra por cansancio**. Se abandona una fase si su prueba no se puede escribir sin
red, sin runtime ajeno o sin un aparato, y entonces se entrega el bloqueo con el comando que lo mide
y la siguiente vía concreta. El plan entero se replantea solo si aparece una API que el usuario
necesite y que no se pueda emular sin llamar a un servicio de Google: en ese caso se escribe la
frontera, no se finge.

## 6. Riesgos y preguntas abiertas

- **Tamaño y tiempo del binario.** Cada fase añade código al `apkc.wasm`. Se medirá con el mismo
  método que §198 antes de publicar y se repetirá la comprobación del host que trunca (§196). Sin
  cifras nuevas hasta medir.
- **`try`/`catch`.** Hoy el intérprete no ejecuta métodos con tabla de excepciones. Varias apps
  reales envuelven `parseInt` o las preferencias en `try`; si E3 lo necesita, entra como requisito
  de esa fase, no como excepción silenciosa.
- **Adaptadores.** Un `getView` que infla layouts con `LayoutInflater` exige emular el inflado desde
  el dex; si el adaptador construye la fila a mano (que es lo que permiten las plantillas del
  studio), E2 es directo y el inflado se pospone.
- **Fidelidad de medidas.** El modelo es aproximado por diseño. E4 acerca el dibujo, pero no
  promete píxeles idénticos al aparato: el campo `aproximado` del JSON sigue diciendo la verdad.
