# El editor de apkc studio

Este documento explica cómo está montada la vista **Editor** de `studio/index.html`: qué pieza
hace qué, qué ids son contrato con las pruebas y qué límites tiene. No es una medición: las
mediciones de la pestaña (compilar, firmar, verificar) están en `docs/evidencia.md`; el resumen de
uso, en `studio/README.md`.

La referencia visual es un editor ligero al estilo Visual Studio Code, **adaptado** a esta página:
barra de actividad, explorador del proyecto, pestaña del fichero, área de código y panel inferior.
No se copia ninguna pieza de VS Code ni se carga ningún editor de terceros.

## Regla de diseño

- **Sin dependencias y sin red.** El editor es un `<textarea>` nativo superpuesto a un `<pre>`
  resaltado. No hay Monaco, CodeMirror, Ace ni CDN. Es la regla 4 del `AGENTS.md`: todo lo que
  compila, empaqueta y firma viaja en el propio origen de la página.
- **Los ids son contrato.** `prueba.js`, `prueba-navegador.js`, `prueba-crear.js`,
  `prueba-pestana.js` y `e2e.sh` buscan ids concretos. Cambiar un id sin cambiar la prueba rompe
  el gate.
- **La vista no guarda estado propio.** El fichero abierto vive en `estadoPestana.ruta`; el
  contenido, en el `<textarea>` y en OPFS. La página no tiene un modelo de documento aparte.

## Mapa de la vista

```text
+--------------------------------------------------------------------------+
| barra de titulo: semaforo, "apkc studio - Mi app", "sin conexion"        |
+------+-------------------+-----------------------------------------------+
| act. | EXPLORADOR        | pestanas de vista: Inicio | Editor            |
|      | COM.MI.APP        +-----------------------------------------------+
|      | arbol del         | pestana del fichero   Nuevo  Guardar  Panel   |
|      | proyecto          +-----------------------------------------------+
|      |                   | numeros | editor (textarea + pre resaltado)   |
|      |                   +-----------------------------------------------+
|      |                   | panel: Salida | Compilar | Firmar             |
|      |                   |         (cerrado hasta que hace falta)        |
+------+-------------------+-----------------------------------------------+
| barra de estado: main | Editor | ultimo codigo - | apkc.wasm | OPFS      |
+--------------------------------------------------------------------------+
```

La cuadricula exterior es `.cuerpo`: 48 px de actividad, 260 px de explorador y el resto para
`.area`. `.area` tiene la fila de pestanas de vista (35 px) y `#vistas`. Cuando la vista activa es
`#pestana`, `vistas.js` pone `#vistas.modo-editor`, que cambia `overflow: auto` por
`overflow: hidden`; así el editor ocupa el alto disponible y no arrastra la pagina.

`#pestana` lleva las clases `vista vista-editor`. Es un `display: flex` en columna con tres filas:
la pestaña del fichero, el editor y el panel inferior. El panel esta `hidden` por defecto; cuando
se abre, el editor se encoge y conserva su propio scroll.

## Vistas

`vistas.js` es el unico fichero que decide que vista se ve:

- `mostrar(vista)` activa la seccion `#crear` o `#pestana`, marca el boton de la barra de
  actividad y la pestana de vista, escribe `document.body.dataset.vista`, cambia
  `#vistas.modo-editor` y actualiza `#estado-general`.
- Los botones `[data-vista]` de la barra de actividad y las pestanas `.pestana-vista` comparten
  el mismo manejador; el hash (`#crear`, `#pestana`) tambien cambia de vista.
- Al arrancar, `iniciarPestana()` (en `pestana.js`) abre el `AndroidManifest.xml` si ya hay un
  proyecto en OPFS y pulsa la vista Editor. Si no hay proyecto, deja Inicio con el formulario.

## Explorador

El arbol vive en la barra lateral, no dentro del area de codigo:

```html
<aside class="lateral">
  <div class="lateral-cab">Explorador <button id="recargar-arbol">...</button></div>
  <div class="lateral-proyecto" id="lateral-proyecto">MI APP</div>
  <div id="arbol-pestana" class="arbol-pestana" role="tree"></div>
  <div class="lateral-pie">/proyecto · OPFS <span id="aislacion">?</span></div>
</aside>
```

`refrescarArbol()` pide `hospedador.arbol()` al worker, filtra las rutas que empiezan por
`/proyecto/` y construye un mapa `padre -> hijos`. Dentro de cada carpeta ordena primero los
directorios y despues los ficheros por `localeCompare`. Cada fila es un `<a>` con:

| Atributo | Para que sirve |
| --- | --- |
| `data-ruta` | ruta completa; `prueba-crear.js` la usa para comprobar el package y el icono |
| `title` | ruta relativa, para verla al pasar el raton |
| `style.paddingLeft` | `8 + profundidad * 12` px; es la jerarquia visual |
| `class="dir"` / `class="archivo"` | icono y color |
| `class="activo"` | fila del fichero abierto |

Cada fila tiene tres piezas: la flecha (`.arbol-flecha`), el icono (`.arbol-icono`) y el nombre
(`.nombre`). Las carpetas se pliegan con `ARBOL.plegados`, un `Set` de rutas que sobrevive a los
refrescos. Pulsar una carpeta alterna el `Set` y vuelve a pintar; pulsar un fichero llama a
`abrir(ruta)`.

Si `/proyecto` esta vacio, el arbol pinta un estado vacio con un enlace a Inicio. Si OPFS no esta
disponible (por ejemplo, `file://`), `refrescarArbol()` captura el error y deja el estado vacio en
vez de reventar la pagina. `#recargar-arbol` vuelve a pedir el arbol sin recargar la pagina.

## El editor de codigo

La pieza central son tres capas superpuestas:

```html
<div class="editor">
  <pre id="numeros" class="numeros"></pre>
  <div class="capas">
    <pre id="resalto" class="resalto" aria-hidden="true"></pre>
    <textarea id="codigo" spellcheck="false" wrap="off"></textarea>
  </div>
</div>
```

- `#codigo` es el unico elemento editable. Tiene el texto transparente
  (`color: transparent; -webkit-text-fill-color: transparent`) y el cursor visible
  (`caret-color: #fff`). El texto real lo pinta `#resalto` debajo.
- `#resalto` recibe el HTML de `resaltar()`. Es `pointer-events: none` para que el raton siga
  llegando al textarea.
- `#numeros` pinta la columna de numeros. Su ancho se calcula en `EDITOR.numeros()` como
  `calc(Nch + 18px)`, donde `N` crece con el numero de lineas y 18 px son los paddings.

Las dos capas comparten las métricas (`font`, `line-height`, `padding`, `tab-size` y
`white-space: pre`). `EDITOR.sincronizar()` copia `scrollTop` y `scrollLeft` del textarea a
`#resalto`, y `scrollTop` a `#numeros`. Las tres capas son contenedores con scroll propio; la
primera versión trasladaba las capas con `transform`, pero el `<pre>` recortaba su contenido al
alto visible y al moverlo hacia arriba no quedaba nada pintado: a partir de la primera pantalla
el editor se veía vacío y parecía que el scroll no funcionaba. Un `ResizeObserver` sobre
`.editor` repite la sincronía cuando el panel inferior cambia el alto. La rueda sobre la columna
de números se reenvía al textarea, y pulsar en el margen devuelve el foco al código.

`EDITOR.pintar()` hace dos cosas: `resaltar(texto, modo)` y `EDITOR.numeros(texto)`. El modo es
`xml` cuando `esXml()` ve un `<?xml`, un `<manifest`, o —quitando BOM, espacios y comentarios—
el texto empieza por una etiqueta; si no, `java`. Esa última condición importa: un fuente Java
con genéricos (`List<String>`) no puede confundirse con XML. El resaltador es léxico, no un
parser:

- Java: palabras clave, anotaciones (`@Override`), números, literales de `char`, cadenas entre
  comillas y comentarios `//` y `/* ... */`.
- XML: comentarios, etiquetas, atributos y valores entre comillas.
- Los literales no cruzan de línea: una comilla sin cerrar no se come el resto del fichero.

`EDITOR.poner(texto)` es la entrada única para cargar un fichero: escribe el valor, deja el
cursor al principio, reinicia el scroll de las tres capas, pinta y marca el documento como
limpio. `EDITOR.texto()` devuelve el valor actual.

### Teclado

| Tecla | Qué hace |
| --- | --- |
| `Tab` | con selección, indenta las líneas; sin selección, inserta 4 espacios en el cursor |
| `Shift+Tab` | con selección, desindenta las líneas; sin selección, quita la sangría de su línea |
| `Enter` | conserva la sangría de la línea; si la línea termina en `{`, añade 4 espacios |
| `Ctrl+S` / `Cmd+S` | guarda el fichero abierto, tenga el foco donde tenga |
| `Ctrl+Enter` / `Cmd+Enter` | repite el último comando de `apkc` |
| ``Ctrl + ` `` | abre o cierra el panel inferior |

Las tres ediciones programáticas (Tab, Shift+Tab y Enter) pasan por `EDITOR.cambiar()`, que usa
`execCommand("insertText")` cuando existe para que entren en el historial nativo de deshacer
(`Ctrl+Z`); si el navegador no lo soporta, cae a `setRangeText` y solo se pierde el deshacer de
esa edición. `e.isComposing` se comprueba antes de tocar Tab/Enter, para no romper la composición
de un IME. La selección del textarea usa un azul con alfa (`rgba(38, 79, 120, .55)`) para que el
texto del `<pre>` se siga leyendo mientras se selecciona. `#estado-cursor` muestra `Ln`/`Col` en
la barra de estado y se actualiza al teclear, al hacer clic y con `selectionchange`.

### Estado del fichero

- `#fichero-abierto` muestra la ruta **relativa** a `/proyecto`; su `title` guarda la ruta
  completa. Si no hay fichero, dice `Sin fichero`.
- `#marca-sucio` es el punto amarillo de la pestaña; `marcarSucio(true)` lo muestra y
  `marcarSucio(false)` lo esconde. Se marca al teclear, indentar, pegar o entrar, y se limpia al
  guardar o al cargar otro fichero. `beforeunload` avisa antes de cerrar la pestaña con cambios.
- `abrir(ruta)` pregunta antes de descartar cambios (`confirmarDescartes`), lee los bytes con
  `hospedador.bytes` (un error del fs se anuncia en la barra, no se convierte en un fichero
  vacío), los decodifica como UTF-8 y llama a `EDITOR.poner`. Si se abre desde el explorador y
  la vista activa no es Editor, salta a Editor.
- `guardar()` pide una ruta si no hay ninguna, escribe los bytes con `hospedador.escribir`,
  actualiza el estado y refresca el árbol. Solo marca limpio si el texto sigue siendo el que se
  escribió: si se tecleó mientras el worker guardaba, el documento sigue sucio.
- `nuevo-fichero` pide una ruta con `nuevoNombre()`. El valor por defecto sale del package del
  formulario (`/proyecto/src/<package>/MainActivity.java`), la ruta se ancla a `/proyecto`, se
  normalizan `.` y `..`, y se avisa si el fichero ya existe. Después crea el fichero vacío en
  OPFS para que el explorador lo muestre, vacía el editor y pone el foco.

Todo el contenido vive en OPFS a traves del worker `hospedador.js`; la pagina no envia el fuente a
ningun servidor.

## Panel inferior

El panel es la zona de salida y de acciones pesadas. Empieza cerrado para que el editor ocupe todo
el alto; se abre cuando hace falta.

```html
<div class="panel-inferior" id="panel-inferior" hidden>
  <div class="panel-tabs">
    <button class="panel-tab activo" data-panel="salida">Salida</button>
    <button class="panel-tab" data-panel="compilar">Compilar</button>
    <button class="panel-tab" data-panel="firmar">Firmar otra apk</button>
    <span id="estado-pestana"></span>
    <button id="cerrar-panel">...</button>
  </div>
  <div class="panel-descargas" id="panel-descargas" hidden>...</div>
  <div class="panel-cuerpos">
    <div id="panel-salida" class="panel-vista activa"><pre id="consola-pestana"></pre></div>
    <div id="panel-compilar" class="panel-vista">...</div>
    <div id="panel-firmar" class="panel-vista">...</div>
  </div>
</div>
```

- `mostrarPanel(nombre)` quita `hidden`, marca la pestana `data-panel=nombre` y ensena la vista
  `#panel-nombre`.
- `alternarPanel()` abre o cierra el panel; si lo abre, vuelve a la pestana activa.
- `cerrarPanel()` lo esconde y deja `aria-expanded="false"` en el boton `#alternar-panel`.
- `correr(argv)` abre siempre la pestana Salida, escribe `$ apkc ...`, muestra `#ocupado`
  mientras el worker trabaja, actualiza `#ultimo-codigo` y refresca el arbol al terminar. La
  consola no usa `textContent` directo: `CONSOLA` guarda el texto y `consolaHtml()` convierte los
  códigos ANSI (`\u001b[34m...\u001b[0m`) en `<span class="consola-fg-4">`; las demás secuencias
  CSI se descartan. Así se ven los colores de `apkc` y no los códigos crudos.
- `#panel-descargas` es una barra encima de las vistas, visible en todas las pestanas. `descargar()`
  y `ofrecerDescarga()` ponen allí los enlaces de `app.apk`, su `.idsig` y los de la firma de una
  apk existente, con nombre y tamaño; así la descarga no se queda escondida en Compilar. El enlace
  `#descargar-fichero` de la fila de pestañas descarga el buffer abierto (aunque no esté guardado).
- `#panel-compilar` empieza por las acciones (`#compilar-debug`, `#compilar-apk`,
  `#verificar-apk`) y deja el resto en tres desplegables: `#detalle-sdk` (términos, descarga
  oficial, `.zip`/`.jar`, `#acepto-sdk`, `#descargar-sdk`, `#traer-android-jar`,
  `#estado-jar-sdk`, `#android-jar`, `#usar-android-jar`, `#estado-jar`), `#detalle-release`
  (`#keystore-compilar`, `#storepass-compilar`, `#keyalias-compilar`) y opciones avanzadas
  (`#sdk-version`, `#codename`, `#v4-compilar`). En una página estática, el enlace a Google solo
  se activa al aceptar los términos y el editor extrae `android-34/android.jar` del `.zip`
  oficial; el `.jar` suelto también vale. Con `servidor.js`, el botón «Traer el del SDK local» y
  el primer compilar piden `/android-jar` al mismo origen. El jar nunca se versiona en el repo.
- `#compilar-debug` no lee el keystore de release: la primera vez llama a
  `apkc debug-keystore`, que escribe `/sdk/debug.keystore` (alias `androiddebugkey`,
  contraseña `android`, `CN=Android Debug`) y lo reutiliza en las siguientes compilaciones.
  `#compilar-apk` sigue siendo el camino release con keystore y contraseña.
- `#panel-firmar` conserva los ids de firma y rotacion: `#keystore`, `#storepass-firmar`,
  `#keyalias-firmar`, `#apk-a-firmar`, `#firmar-aqui`, `#keystore-viejo`, `#storepass-viejo`,
  `#keyalias-viejo`, `#rotation-min-sdk`, `#usar-rotacion`, `#crear-linaje`, `#olvidar-linaje` y
  `#estado-linaje`.

Las acciones concretas de compilar y firmar se documentan en `studio/README.md` y en
`docs/evidencia.md` (§44-§47, §55, §117-§136). Este documento solo cubre la interfaz.

## Barra de estado

La barra azul (`#007acc`, como la de VS Code) muestra `#estado-general` (Inicio o Editor), la rama
`main`, `#estado-cursor` (`Ln`/`Col`), `#ultimo-codigo`, y la firma de la página:
`apkc.wasm · OPFS · UTF-8`. `#estado-pestana` vive en la cabecera del panel; `vistas.js` lo copia a
`#estado-pestana-cab` con un `MutationObserver`, para que el mensaje se vea aunque el panel esté
cerrado. `#ocupado` está en la fila de la pestaña del fichero y solo aparece mientras corre un
comando; `correr()` garantiza que se esconde también cuando el hospedador falla.

## Pantallas estrechas

En `@media (max-width: 760px)`:

- `.cuerpo` pasa a dos columnas (44 px de actividad y el resto) y dos filas (9rem de explorador y
  el resto). La barra de actividad ocupa las dos filas.
- El explorador queda como una tira superior con scroll propio; el pie del explorador se oculta.
- El panel inferior pasa a `42vh`.
- La pestaña del fichero se limita a `22ch` para que los botones Nuevo/Guardar/Panel sigan
  cabiendo.
- La barra de titulo oculta el estado de la derecha.

## Pruebas

| Comando | Que cubre |
| --- | --- |
| `node studio/prueba.js` | genera el zip, comprueba EOCD, plantillas, reglas, ids de `index.html` y ahora tambien el resaltador puro (`resaltar`, `esXml`) |
| `node studio/prueba-navegador.js` | `file://`: DOM, dos vistas, dos botones de actividad, reflejo del nombre/package, aviso de que no hay OPFS, localStorage |
| `APKC_WASM=... node studio/prueba-crear.js` | por http con OPFS: crear, escribir el proyecto, abrir el manifiesto, comprobar `data-ruta` del arbol y el icono |
| `APKC_WASM=... APK_EJEMPLO=... KS_EJEMPLO=... ANDROID_JAR=... node studio/prueba-pestana.js` | 66 comprobaciones: editar, guardar, recargar, resaltar, números de línea, scroll de las tres capas, Tab/Shift+Tab/Enter con deshacer, jar del SDK local, `.zip` oficial con sus términos, compilar debug sin keystore, barra de descargas, consola con colores ANSI, firmar, compilar release, verificar y no pedir nada fuera del origen |
| `APKC=... APKC_WASM=... sh studio/e2e.sh` | todo lo anterior más los apk/aab nativos; en la pestaña son 66 comprobaciones |

El gate completo de la pagina es `e2e.sh`; `prueba-pestana.js` solo no basta para decir que la
pagina funciona. Los comandos de entorno estan en `tools/entorno.sh` (`test`, `wasm`, `gate`).

## Como abrirlo en Brave Nightly

Con `servidor.js` escuchando en `127.0.0.1:8787`, el comando y las trampas del entorno Android
estan en `studio/README.md` y en `docs/evidencia.md` §140. En resumen: se copian
`BOOTCLASSPATH` y `DEX2OATBOOTCLASSPATH` del `environ` de `zygote64` y se lanza
`com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity` con
`android.intent.action.VIEW`. No se usa `cmd activity`: en este aparato falla con
`Failed transaction`.

## Limites conocidos

- **Una sola pestaña de fichero.** No hay varias pestanas de documentos ni un historial de
  editores.
- **Sin buscar ni reemplazar.** `Ctrl+F` es el del navegador, no el del editor.
- **Sin renombrar, borrar ni mover ficheros** desde el arbol. Solo se abren y se pliegan.
- **Sin autosave.** El guardado es explicito (`Ctrl+S` o el boton).
- **Sin plegado de codigo, minimapa, multicursor ni diff.**
- **El resaltado es lexico.** No entiende de tipos, scopes ni errores; no sustituye a un LSP.
- **La indentacion automatica solo mira `{`.** No conoce `switch`, `try` ni el estilo de cada
  fichero.
- **No edita binarios.** El contenido se decodifica como UTF-8; un PNG del arbol no se abre en el
  editor.
- **El panel no es redimensionable con el raton.** Su alto lo fija el CSS (34vh, 42vh en movil).
- **El estado de scroll del arbol se conserva, el del editor no** entre ficheros: `EDITOR.poner`
  vuelve a 0,0 a proposito.

Estas fronteras son las de la pagina, no las de `apkc`. Lo que el subconjunto Java acepta o
rechaza esta en el inventario de la Fase 5 de `docs/plan.md`.
