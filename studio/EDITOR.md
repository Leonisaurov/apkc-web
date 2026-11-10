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
|      | arbol del         | fichero  [Guardar] [Archivo v] [Compilar]     |
|      | proyecto          +-----------------------------------------------+
|      |                   | numeros | editor (textarea + pre resaltado)   |
|      |                   +-----------------------------------------------+
|      |                   | Consola | Historial | Compilar | Firmar       |
|      |                   |         (cerrada hasta que hace falta)         |
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

## Diagnóstico

El bloque `#detalle-diagnostico` de Inicio es independiente del editor: un `<textarea
id="salida-apkc">`, los botones `#analizar-salida` y `#limpiar-salida`, el resultado
`#resultado-analisis` y el enlace `#descargar-informe`. `diagnostico.js` llama a `analyze()` y a
`informe()` de `app.js`; no tiene reglas propias ni envía el texto a ningún sitio. Cada regla
reconocida se registra en `localStorage` y el informe se descarga como `informe-apkc.md`.

## Explorador

El arbol vive en la barra lateral, no dentro del area de codigo:

```html
<aside class="lateral">
  <div class="lateral-cab">Explorador
    <span class="lateral-acciones">
      <button id="subir-arbol">...</button>
      <button id="recargar-arbol">...</button>
    </span>
  </div>
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
| `draggable` | solo con puntero fino; en táctil se mueve con **Mover a…** |
| `class="arrastrando"` / `class="soltar"` | realimentación visual del arrastre |

Cada fila tiene tres piezas: la flecha (`.arbol-flecha`), el icono (`.arbol-icono`) y el nombre
(`.nombre`). Los ficheros añaden un `↓` (`.arbol-descargar`) que descarga esa entrada de OPFS sin
abrirla, también si es binaria. Las carpetas se pliegan con `ARBOL.abiertos`, un `Set` que
**empieza vacío**: el árbol arranca cerrado. `#plegar-arbol` lo vacía y `#desplegar-arbol` mete
todas las carpetas del proyecto; `revelarRuta()` abre los ancestros del fichero que se abre o se
mueve. Pulsar una carpeta alterna el `Set` y vuelve a pintar; pulsar un fichero llama a
`abrir(ruta)`.

`#filtro-arbol` filtra por subcadena (sin distinguir mayúsculas) y conserva la cadena de carpetas
del fichero que coincide; mientras hay filtro, las carpetas se pintan abiertas. `Ctrl+P` enfoca el
campo. `build/` y sus hijos llevan la clase `generado`, el color atenuado y la etiqueta
`generado` en la carpeta raíz: se puede abrir, pero se regenera en cada compilación.

`#subir-arbol` pide archivos con `#subir-ficheros` y los escribe en la carpeta del fichero
abierto, o en `/proyecto` si no hay ninguno. En el menú de una carpeta, **Subir archivos aquí** y
**Subir carpeta aquí** fijan el destino; el segundo usa el input `#subir-carpeta` con
`webkitdirectory` y conserva los subdirectorios vía `webkitRelativePath`. `subirSeleccion()`
pregunta una sola vez antes de reemplazar archivos existentes, escribe los bytes directamente en
OPFS y abre el resultado si solo se subió un fichero de texto y no había cambios pendientes.

Con el botón derecho o manteniendo pulsada una fila aparece `#menu-archivo`, un menú con
**Abrir** y **Descargar** (ficheros), **Nuevo fichero aquí**, **Subir archivos aquí** y **Subir
carpeta aquí** (carpetas), **Mover a…**, **Renombrar** y **Borrar**. Renombrar cambia el nombre en
la misma carpeta con `hospedador.renombrar` (host `renombrar` → `fs.rename`, sin copiar bytes) y
actualiza la pestaña si el fichero abierto es el renombrado o está dentro de la carpeta
renombrada. Borrar usa `hospedador.limpiar`; si se borra el fichero abierto, el editor vuelve a
`Sin fichero`.

Mover usa el mismo `fs.rename`: `moverRuta(origen, carpeta)` calcula el destino, rechaza mover
una carpeta dentro de sí misma y remapea `ARBOL.abiertos` y la ruta del fichero abierto. Con
ratón o lápiz las filas son `draggable` y las carpetas aceptan el `drop`; en táctil —donde el
arrastre nativo choca con el menú de mantener pulsado— se usa **Mover a…**, que pide la carpeta
destino y la valida contra el árbol real.

Si `/proyecto` esta vacio, el arbol pinta un estado vacio con un enlace a Inicio. Si OPFS no esta
disponible (por ejemplo, `http` sin COOP/COEP), `refrescarArbol()` captura el error y deja el estado vacio en
vez de reventar la pagina. `#recargar-arbol` vuelve a pedir el arbol sin recargar la pagina.

## Barra del fichero

La fila superior del editor separa la acción principal de las de mantenimiento:

- `#guardar-fichero` es el único botón primario y guarda con `Ctrl+S`.
- `#menu-editor` es un `<details>` con `#nuevo-fichero`, `#subir-ficheros-boton`,
  `#exportar-proyecto`, `#importar-proyecto` (y su `#importar-archivo` oculto),
  `#descargar-fichero`, `#renombrar-fichero` y `#borrar-fichero`. Se cierra al pulsar una acción,
  al hacer clic fuera o con `Escape`; no guarda estado propio.
- `#alternar-panel` se llama **Compilar**: abre `#panel-inferior` en la pestaña Compilar y lo
  cierra si ya está abierto. ``Ctrl + ` `` sigue alternando el panel y conserva la pestaña activa.
  La pestaña de salida se llama **Consola** y lleva el icono de terminal.
- `#descargar-fichero` sigue oculto hasta que hay un fichero abierto; descarga el buffer del
  editor (aunque tenga cambios sin guardar), no solo lo último guardado.

## Pestañas y persistencia

El editor ya no tiene un solo documento. `DOCUMENTOS = { lista, activa }` guarda por pestaña
`{ ruta, texto, sucio, cursor, scroll }`:

- `abrir(ruta)` añade la pestaña si no existe y activa; si ya existe, solo cambia a ella. Cambiar
  de pestaña **no descarta** cambios: cada una conserva lo suyo.
- `activarDocumento()` guarda el texto de la pestaña anterior si está sucia, suelta el de una
  limpia (`texto = null`) y, si hace falta, lee OPFS en ese momento. Así restaurar diez ficheros
  solo lee el que se ve; los demás se cargan al pulsarlos.
- `#pestanas-archivo` se repinta en `renderPestanas()`. La pestaña activa lleva los ids
  `#fichero-abierto` y `#marca-sucio`; las demás usan `.editor-tab-nombre` y `.marca-sucio`. El
  botón `×` cierra (con confirmación solo si hay cambios) y, si era la activa, abre la vecina.
- `PERSISTENCIA = "apkc-studio/editor-v1"` guarda en **sessionStorage** las rutas de las pestañas,
  la activa y `ARBOL.abiertos`: cada ventana tiene su vista y no pisa la de otra. **No guarda
  contenido**: al recargar, un fichero sucio se relee de OPFS; el undo tree es la red de rescate.
- `guardarEnMemoria()` conserva texto sucio, cursor y scroll por pestaña; una pestaña limpia se
  puede reconstruir desde OPFS y no ocupa memoria.
- `activacion` es un contador de turno: si una lectura lenta termina después de que se pulse otra
  pestaña, no pisa el editor.
- `VENTANA` es un id estable en sessionStorage; indexa el undo tree en IndexedDB.

### Undo tree

`historial.js` guarda un árbol por documento. Cada nodo tiene padre, hijos, autor, selección y
scroll; el primer nodo guarda el texto completo y los siguientes un **delta** contra el padre
(`{i, b, s}`), con un **checkpoint** cada 24 niveles o cuando el delta es enorme. Reconstruir un
nodo es subir hasta el checkpoint y aplicar deltas.

- `Ctrl+Z` va al padre; `Ctrl+Shift+Z`/`Ctrl+Y` al último hijo. Si deshaces y escribes, se crea una
  rama nueva y la anterior queda en el árbol.
- Una pausa de tecleo de 500 ms agrupa la edición en un nodo; guardar, cambiar de pestaña, perder
  el foco o cerrar la página vuelca lo pendiente.
- `panel-historial` pinta el árbol con hora, autor (`local`/`remoto`), delta y nodo actual. Un clic
  restaura ese nodo sin borrar los demás. `#historial-deshacer`, `#historial-rehacer` y
  `#historial-limpiar` son las acciones.
- Persistencia: IndexedDB `apkc-studio-historial`, clave `VENTANA|ruta`; se guarda con debounce y
  se carga solo al activar el documento. Límite de 240 nodos y 4 MB; `podar()` quita hojas
  antiguas y `rebase()` re-enraíza una cadena demasiado larga en un checkpoint.
- Un fichero que cambió en OPFS desde otra ventana entra como nodo `remoto`; el editor muestra el
  texto del disco y la rama local queda en el historial.

### Temas

`temas.js` aplica `data-tema` y `data-tema-consola` en `<html>` y guarda la elección en
`localStorage` (preferencia compartida por las ventanas). `style.css` define las variables por
tema: Oscuro, Catppuccin Mocha, Rosé Pine, Tokyo Night y Gruvbox; y paletas ANSI Catppuccin,
Rosé Pine, Gruvbox, Solarized Dark y Dracula. El menú `#menu-apariencia` de la barra de estado
tiene los dos selectores `#tema-ui` y `#tema-consola`. No hay hojas externas.

### Varias ventanas

- `VENTANA` identifica la pestaña del navegador; la vista (tabs, activa, `ARBOL.abiertos`) va en
  `sessionStorage`, no en `localStorage`.
- Cada mutación pasa por `conBloqueo()` (`navigator.locks.request("apkc-studio:/proyecto")`), de
  modo que dos ventanas no interleaven escrituras. Si Web Locks no existe, se ejecuta directo.
- `guardar()` guarda una revisión en `localStorage` (`apkc-studio/rev/<ruta>`), la pone en el
  documento y publica `{tipo:"guardado"}` por `BroadcastChannel`. Si la revisión del disco ya no
  es la del documento, pide confirmación antes de sobrescribir.
- `publicar()` avisa también de `crear`, `subir`, `renombrar`, `mover`, `borrar` y `proyecto`.
- Cada documento guarda `base`: el texto de la última versión de OPFS. Al guardar o recargar se
  actualiza.
- `recibirRemoto()` ignora su propia `VENTANA`, refresca el espejo del worker
  (`hospedador.refrescar`) y compara `base`, el buffer local y el remoto con `fusionar()`:
  - sin cambios locales, recarga el remoto;
  - sin cambios remotos, conserva el local;
  - cambios en líneas distintas, **fusión automática** y el documento queda sucio para revisar;
  - cambios en las mismas líneas, **conflicto** con `#conflicto-aviso`: **Mantener el mío**,
    **Usar el remoto** o **Marcar conflicto** (marcas `<<<<<<<` / `=======` / `>>>>>>>`).
- La fusión es de tres vías por líneas (`fusion.js`), sin CRDT ni servidor. En ficheros de más de
  2000 líneas cae a conflicto explícito. Un borrado remoto conserva las pestañas sucias y marca
  `remotoEliminado`.

### Sin red

`main.js` importa `offline.js`, que registra `sw.js` por http(s). El worker cachea el shell en la
instalación e intenta cachear `apkc.wasm`/`wasm_exec.js`; si fallan, los cachea la primera vez que se piden; no toca `/android-jar`, los
keystores ni OPFS. Las respuestas cacheadas conservan COOP/COEP, así que una recarga offline sigue
siendo `crossOriginIsolated`. `prueba-pestana.js` comprueba la recarga con la red emulada apagada.

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
| ``Ctrl + ` `` | abre o cierra la consola inferior |

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

El **panel inferior** (`#panel-inferior`) es la zona de salida, historial y acciones pesadas.
Empieza cerrado para que el editor ocupe todo el alto; `#alternar-panel` lo abre en **Compilar**
o lo cierra, y ``Ctrl + ` `` conserva la pestaña activa.

```html
<div class="panel-inferior" id="panel-inferior" hidden>
  <div class="panel-tabs">
    <button class="panel-tab activo" data-panel="salida">Consola</button>
    <button class="panel-tab" data-panel="historial">Historial</button>
    <button class="panel-tab" data-panel="compilar">Compilar</button>
    <button class="panel-tab" data-panel="firmar">Firmar otra apk</button>
    <span id="estado-pestana"></span>
    <button id="cerrar-panel">...</button>
  </div>
  <div class="panel-descargas" id="panel-descargas" hidden>...</div>
  <div class="panel-cuerpos">
    <div id="panel-salida" class="panel-vista activa"><pre id="consola-pestana"></pre></div>
    <div id="panel-historial" class="panel-vista">...</div>
    <div id="panel-compilar" class="panel-vista">...</div>
    <div id="panel-firmar" class="panel-vista">...</div>
  </div>
</div>
```

- `mostrarPanel(nombre)` quita `hidden`, marca la pestana `data-panel=nombre` y ensena la vista
  `#panel-nombre`. Si el nombre es `historial`, llama a `renderHistorial()`.
- `#panel-historial` pinta el undo tree del documento activo. `#historial-deshacer`,
  `#historial-rehacer` y `#historial-limpiar` son sus acciones; el menú **Archivo → Historial…**
  y `#abrir-historial` abren esta pestaña.
- `alternarPanel()` abre o cierra el panel; si lo abre, vuelve a la pestana activa.
- `cerrarPanel()` lo esconde y deja `aria-expanded="false"` en el boton `#alternar-panel`.
- `correr(argv)` abre siempre la pestaña Consola, escribe `$ apkc ...`, muestra `#ocupado` y
  `#detener-comando` mientras el worker trabaja, actualiza `#ultimo-codigo` y refresca el arbol al
  terminar. La salida llega por trozos (`hospedador.escucharSalida`), así que se pinta mientras el
  comando corre y la última línea se enseña como fase en `#ocupado`; **Detener** termina el worker
  (`hospedador.cancelar`) y el siguiente comando arranca otro. La
  consola no usa `textContent` directo: `CONSOLA` guarda el texto y `consolaHtml()` convierte los
  códigos ANSI (`\u001b[34m...\u001b[0m`) en `<span class="consola-fg-4">`; las demás secuencias
  CSI se descartan. `enlazarSalidas()` convierte `ruta:línea` en un `<a class="salto">` que
  `saltarALinea()` resuelve abriendo el fichero e `irALinea()` seleccionando esa línea. Así se ven los colores de `apkc` y no los códigos crudos. La paleta de
  `.consola` sale de `--ansi-0..15` y se cambia con `#tema-consola` (Catppuccin por defecto;
  también Rosé Pine, Gruvbox, Solarized Dark y Dracula).
- `#panel-descargas` es una barra encima de las vistas, visible en todas las pestanas. `descargar()`
  y `ofrecerDescarga()` ponen allí los enlaces de `app.apk`, su `.idsig` y los de la firma de una
  apk existente, con nombre y tamaño; así la descarga no se queda escondida en Compilar. El enlace
  `#descargar-fichero` de la fila de pestañas descarga el buffer abierto (aunque no esté guardado).
- `#cargando` es un velo con una rueda (`conCarga(texto, tarea)`) para las operaciones que no son
  compilar pero tardan: leer el apk para el blob, sacar `android.jar` de un `.zip`, abrir un
  fichero grande o descargar el jar del SDK. Solo aparece si la tarea pasa de 120 ms, y se retira
  en `finally`, también cuando falla.
- `#panel-compilar` empieza por las acciones (`#compilar-debug`, `#compilar-apk`,
  `#verificar-apk`, `#descargar-debug-keystore`) y deja el resto en desplegables:
  `#detalle-ajustes` (`#ajustes-minsdk`, `#ajustes-targetsdk`, `#ajustes-versioncode`,
  `#ajustes-versionname`, `#leer-ajustes`, `#guardar-ajustes`, `#estado-ajustes`; avisa si
  targetSdk < 34), `#detalle-sdk`
  (`#abrir-dialogo-sdk`, `#dialogo-sdk`, `#sdk-entiendo`, `#sdk-terminos`, `#sdk-descarga`,
  `#sdk-elegir-archivo`, `#sdk-usar-servidor`, `#sdk-cerrar`, `#traer-android-jar`,
  `#estado-jar-sdk`, `#android-jar`, `#usar-android-jar`, `#estado-jar`), `#detalle-release`
  (`#keystore-compilar`, `#storepass-compilar`, `#keyalias-compilar`) y opciones avanzadas
  (`#api-version`, los ocultos `#sdk-version`/`#codename`, `#v4-compilar`). `#api-version` es el
  selector visible, 34 por defecto; al cambiar sincroniza los dos campos que lee `apkc build`.
  En una página estática, el diálogo exige marcar el acuse antes de habilitar los enlaces a los
  términos y a la descarga oficial de Google; no descarga nada por su cuenta y el editor extrae
  `android-34/android.jar` del `.zip`. El `.jar` suelto también vale. Con `servidor.js`, el botón «Traer el del SDK local» y
  el primer compilar piden `/android-jar` al mismo origen. El jar nunca se versiona en el repo.
- `decirEstadoJar()` marca `#detalle-sdk` con `.jar-listo`, muestra `#sdk-badge` como **listo**,
  cambia `#abrir-dialogo-sdk` a **Cambiar android.jar** y pliega el desplegable cuando el jar ya
  está en OPFS. Así la sección no vuelve a pedir atención en cada visita.
- `#buscar-barra` es del editor: `#buscar-texto`, `#reemplazar-texto`, `#buscar-anterior`,
  `#buscar-siguiente`, `#reemplazar-uno`, `#reemplazar-todos`, `#buscar-cerrar` y
  `#buscar-estado`. `Ctrl+F`/`Ctrl+H` la abren, `Enter`/`Shift+Enter` recorren y `Esc` la cierra;
  reemplazar pasa por `EDITOR.cambiar`, así que entra en el undo tree.
- `Ctrl+G` pide una línea y llama a `irALinea()`; `Ctrl+P` enfoca `#filtro-arbol`.
- `#compilar-debug` no lee el keystore de release: la primera vez llama a
  `apkc debug-keystore`, que escribe `/sdk/debug.keystore` (alias `androiddebugkey`,
  contraseña `android`, `CN=Android Debug`) y lo reutiliza en las siguientes compilaciones.
  `#compilar-apk` sigue siendo el camino release con keystore y contraseña.
- `#crear-clave-release` abre `#dialogo-clave`. El formulario pide alias, CN, organización, país,
  contraseña y confirmación; un aviso obligatorio explica que perder la clave impide actualizar la
  app. Al generar, `apkc create-keystore` escribe `/sdk/release.keystore` (JKS), la página descarga
  `<alias>.jks`, rellena `#storepass-compilar`/`#keyalias-compilar` y pasa al paso **Clave
  creada**, que no deja cerrar hasta marcar que se guardó una copia. No se persiste la contraseña.
  `decirEstadoClave()` deja `#detalle-release` plegado con `.jar-listo`, `#clave-badge` y el botón
  **Cambiar clave de release** cuando el fichero ya existe.
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
- La pestaña del fichero se limita a `22ch` para que Guardar, Archivo y Compilar sigan cabiendo.
- La barra de titulo oculta el estado de la derecha.

## Pruebas

| Comando | Que cubre |
| --- | --- |
| `node studio/prueba.js` | genera el zip, comprueba EOCD, plantillas, reglas, ids de `index.html` (también los de `diagnostico.js`, `temas.js` e `historial.js`), el resaltador puro y el undo tree (ramas, reconstrucción y poda) |
| `node studio/prueba-navegador.js` | `http` sin COOP/COEP: DOM, dos vistas, dos botones de actividad, reflejo del nombre/package, aviso de que no hay OPFS, localStorage, el diagnóstico, los temas y la paleta ANSI |
| `APKC_WASM=... node studio/prueba-crear.js` | por http con OPFS: crear, escribir el proyecto, abrir el manifiesto, comprobar `data-ruta` del arbol, el icono, API 34, el árbol cerrado con sus botones, que `build/` se vea como generado y que el filtro conserve las carpetas, subir un archivo, moverlo de carpeta, exportar/importar el `.zip` (con binario), leer y guardar los ajustes en el manifiesto, varias pestañas, la persistencia tras recargar, el undo tree con ramas/IndexedDB, dos ventanas que se pasan guardados por BroadcastChannel y la fusión/conflicto de la misma línea |
| `APKC_WASM=... APK_EJEMPLO=... KS_EJEMPLO=... ANDROID_JAR=... node studio/prueba-pestana.js` | 83 comprobaciones: editar, guardar, recargar, resaltar, números de línea, scroll de las tres capas, Tab/Shift+Tab/Enter con deshacer, jar del SDK local, `.zip` oficial con sus términos, compilar debug sin keystore, barra de descargas con descarga real, consola con colores ANSI, menú de fichero, renombrar/borrar, pantalla de carga, firmar, compilar release, verificar, recargar sin red, no pedir nada fuera del origen, la salida por trozos, el enlace `ruta:línea`, Detener y la fase en la barra |
| `APKC=... APKC_WASM=... sh studio/e2e.sh` | todo lo anterior más los apk/aab nativos y la plantilla Contador; en la pestaña, 83 comprobaciones |

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

- **Sin dividir el editor.** Hay varias pestañas, pero no dos grupos ni ventanas.
- **Sin buscar ni reemplazar.** `Ctrl+F` es el del navegador, no el del editor.
- **Sin cortar y pegar ni arrastrar en táctil.** Con ratón o lápiz hay arrastrar y soltar entre
  carpetas; en táctil, el menú de la fila ofrece **Mover a…**. Tampoco hay arrastrar y soltar para
  subir archivos: se suben con el botón del explorador o el menú de la carpeta.
- **Sin autosave.** El guardado es explicito (`Ctrl+S` o el boton).
- **Sin plegado de codigo, minimapa, multicursor ni diff.**
- **El resaltado es lexico.** No entiende de tipos, scopes ni errores; no sustituye a un LSP.
- **La indentacion automatica solo mira `{`.** No conoce `switch`, `try` ni el estilo de cada
  fichero.
- **No edita binarios.** El contenido se decodifica como UTF-8; un PNG del arbol no se abre en el
  editor.
- **La consola no es redimensionable con el raton.** Su alto lo fija el CSS (34vh, 42vh en movil).
- **El editor recarga de OPFS, no autosalva.** Al recargar vuelven las pestañas y la activa; un
  cambio sin guardar queda como rama del undo tree, pero el editor muestra el texto del disco.
  `beforeunload` avisa antes.
- **El cursor y el scroll se guardan por pestaña** mientras la sesión vive; la vista (pestañas,
  activa y `ARBOL.abiertos`) va en `sessionStorage` y el undo tree en IndexedDB por ventana.
- **El undo tree tiene límites:** 240 nodos y 4 MB por documento; las ramas más antiguas se podan.
  No hay colaboración en tiempo real ni merge entre ventanas: se avisa y se puede sobrescribir.

Estas fronteras son las de la pagina, no las de `apkc`. Lo que el subconjunto Java acepta o
rechaza esta en el inventario de la Fase 5 de `docs/plan.md`.
