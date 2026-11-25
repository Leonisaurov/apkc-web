# Plan de mejora del diagnóstico de apkc-studio

Orden acordado con el usuario el **2026-11-22**, a raíz de un falso error del diagnóstico en vivo:
al escribir `setContentView(R.layout.main);` en `MainActivity.java`, la pestaña decía *el tipo de
este argumento no esta medido*. La causa y su cierre están en `docs/evidencia.md` §200.

La cola acordada fue **D1 → D2 → D3** (verdad, velocidad, propuesta). El usuario amplió el alcance
el 2026-11-22 con **D4 → D5 → D6**: corregir el diagnóstico de asignaciones cualificadas, llevar la
velocidad y el overhead al mínimo con cifras, y maximizar cobertura y comprensión del autocompletado
en Java y XML. Igual que en `docs/plan-mejoras.md`, esto es un **plan**, no un registro: aquí no
entran hallazgos, y cada ítem se cierra con su sonda, su `§` en `docs/evidencia.md` con el comando
que lo reproduce y su commit.
Ninguna casilla se marca «hecha» sin evidencia propia sobre el árbol final (regla 10 de
`AGENTS.md`).

| # | Ítem | Criterio de aceptación | Cómo se reproduce |
| --- | --- | --- | --- |
| **D1** | **Diagnóstico correcto y accionable** | (a) `setContentView(R.layout.main)` en la plantilla del studio da **cero errores**; (b) todo error lleva `archivo:línea` y, cuando nació en un token, también `columna`; (c) el mensaje dice qué no se resuelve o qué no está medido, en español y sin repetir la posición; (d) el subrayado de `#diagnostico-marcas` cae sobre el token, no sobre la línea | `apkc check --android-jar … --proyecto … MainActivity.java`; `apkc/check_test.go`; `APKC_WASM=… node studio/prueba-pestana.js` |
| **D2** | **Más rápido** | medir el coste real de un `check` en vivo (hoy: debounce de 350 ms en `studio/diagnostico-vivo.js`; el trabajo del worker va aparte) y bajar la latencia sin saturar el worker, **solo con cifras**: reutilizar la instancia de wasm, cachear el marco del `android.jar`, cancelar los `check` obsoletos | la sonda de D2, por escribir: tiempos de `apkc check` nativo con y sin `--proyecto`, y del mismo comando en la pestaña |
| **D3** | **Autocompletado** | identificar de dónde saldrían los ids de recursos (`R.layout.*`, `R.id.*`) y los métodos del jar, y diseñar la fuente de datos **sin red**: qué se lee del `android.jar` del propio origen y qué del árbol del proyecto | la sonda de D3, por escribir: tamaño y tiempo de leer los nombres del jar y del `R.java` del proyecto |
| **D4** | **Asignación cualificada** | `this.filePath = filePath;` en `Apk.java:14` da **cero errores**; el receptor `this` y el campo se resuelven contra la clase y desaparece «una asignacion con mas de un nombre en el destino no esta medida». El dex emitido sigue byte a byte el de javac 17 | `apkc check --android-jar … --proyecto … Apk.java`; `apkc/check_test.go` |
| **D5** | **Velocidad y overhead mínimo** | cerrar D2 con cifras reales en escritorio y pestaña: debounce, cancelación de `check` obsoletos, reutilización del índice I5 del `android.jar`, sin arrancar una instancia nueva de wasm por tecla y sin saturar el worker. Objetivo: `check` nativo <30 ms y respuesta en pestaña medible, no estimada | sonda de D5: tiempos de `apkc check` con y sin `--proyecto`, y del mismo comando en la pestaña |
| **D6** | **Cobertura y comprensión máximas (Java y XML)** | matriz de construcciones Java y de etiquetas/atributos/valores XML con estado medido; autocompletado para `R.`, miembros del jar, `@`, `<` y atributos; cada error nombra qué falta y dónde; cero red | `apkc/check_test.go`, `studio/prueba.js` y `studio/prueba-pestana.js`; la matriz vive en este plan |

## Estado

- **D1 hecha el 2026-11-22** (§200). `check` enlaza la `R` del proyecto en memoria (o usa la que
  dejó el build) y los errores salen con `archivo:línea:columna`. Los tests están en
  `apkc/check_test.go`, `apkc/javaemisor_argumento_estatico_test.go` (byte a byte contra javac 17)
  y en la comprobación del diagnóstico de `studio/prueba-pestana.js`.
- **D2 medida el 2026-11-24 (§329).** La sonda nativa nueva (`apkc/check_tiempo_test.go`) da 8,4 ms
  sin proyecto, 181,6 ms sin `R` enlazando `res/` y 7,6 ms con `R` y `--proyecto`; la sonda CDP
  (`studio/sonda-check.js`) imprime las cinco cifras, el salto de pasadas limpias y las dos formas de
  encadenar checks. Las cifras antiguas de D1 (~15/~30/~200 ms) y las de §232 e I5 quedan como
  transferidas y en verificación, no como resultado de este frente. La conclusión sobre el overhead
  de arranque depende de una mediana inestable y se deja en verificación en §332. Son cifras de un
  escritorio, no de un teléfono.
- **D3 hecha el 2026-11-22** (§201, ampliación final). El editor sugiere recursos del proyecto
  (`R.layout.*`, `R.id.*`) y APIs del `android.jar` sin red: el jar se indexa por trozos
  (1 354 339 de 26 361 808 bytes, 3837 clases, ~50 ms, 0 peticiones), `Tab` acepta y entra en el
  undo tree, `Esc` cierra y las flechas recorren sin disparar otro `check`. El linter de XML
  (`xml-editor.js`) entró en la misma tanda: es local, no lanza `apkc check` y sus filas dicen
  `XML`. El gate final dio `pestaña: 170 comprobaciones` (§202, la cifra de entonces; con el árbol
  de hoy son 192). Fronteras: sin completado tras `v.`, bloques `/* … */` de varias
  líneas no detectados y recursos cacheados hasta `refrescar()`.

### Cola D4-D6 (ampliación del usuario, 2026-11-22)

- **D4 hecha el 2026-11-24 (§330).** `this.filePath = filePath` y `appInfo.sourceDir = filePath`
  salen con cero errores; el control nombra el campo en `Apk.java:7`; el receptor sin tipo dice
  «no se conoce el tipo del receptor appInfo»; el dex sigue byte a byte contra javac 17.
- **D5 en verificación (§331 y §332).** La columna del parser ya apunta al token (§331, con su
  falsación). La decisión de sesión persistente queda **incierta**: las tres medianas de la sonda,
  tomadas antes de reconciliar con `6496918`/`1a60875`, dieron 24,3 % / 10,1 % / 67,7 % frente al
  umbral de ⅓; no se implanta `--sesion` y se pide la mediana estable con `REPES=7` sobre el árbol
  reconciliado antes de decidir. No se cancelan checks obsoletos sin una cifra fija.
- **D6 matriz medida en node, fila 8 en verificación (§333).** Las siete filas de módulos puros están
  en verde en `node studio/prueba.js` sobre el submódulo reconciliado `a41db75` (matriz D6 más
  bombeo del cronómetro). La octava se comprobó en `prueba-pestana.js` **antes** de la reconciliación
  (`240 comprobaciones bien, 3 mal`); `6496918` arregla D7, pero la corrección prohíbe CDP/chromium
  y no se ha reejecutado sobre `1a60875`: no se afirma que los fallos sigan ni que desaparecieron.
  No se anuncia cobertura completa por analogía.

### Matriz D6 (medida en §333)

| # | Lenguaje | Caso | Comando | Estado medido |
| --- | --- | --- | --- | --- |
| 1 | Java | `R.layout.` y `R.id.` sin guardar | `node studio/prueba.js` | `matriz 1 …: ok` (tipos, valores y `@+id` del buffer) |
| 2 | Java | `View.` y `v.` con declaración; sin declaración | `node studio/prueba.js` | `matriz 2 …: ok` (`contexto === null`) |
| 3 | Java | `this.campo` propio y prefijo `Acti` | `node studio/prueba.js` | `matriz 3 …: ok` |
| 4 | Java | cursor en bloque `/* … */` o cadena | `node studio/prueba.js` | `matriz 4 …: ok` (sin contexto) |
| 5 | XML | `<` en layout, manifiesto y `values` | `node studio/prueba.js` | `matriz 5 …: ok` (sin cruzar grupos) |
| 6 | XML | `android:` y valores enumerados | `node studio/prueba.js` | `matriz 6 …: ok` |
| 7 | XML | `@strin` y fila del linter | `node studio/prueba.js` | `matriz 7 …: ok` (`origen:"XML"`, `archivo:línea:columna`) |
| 8 | Errores | cada fila del panel nombra qué falta y dónde | `node studio/prueba-pestana.js` | medido antes de `6496918` (fila propia verde, global `240 bien, 3 mal`); **`incierto`** sobre el árbol reconciliado por la prohibición de CDP |

Orden seguido: **F0 → F1 → F2 → F3 → F4 → F5**, un solo builder y capas secuenciales. Ninguna fila
se marcó hecha sin su comando y su `§` (regla 11).

### Reconciliación (F0, sin medición propia)

Tabla de partida del frente `d2-d6-diagnostico` sobre `HEAD 4b723ae`, puntero del submódulo
`704b409`. `git -c protocol.file.allow=always submodule update --init studio` pobló `studio/` en
ese puntero. Todo lo de abajo está **en verificación**: es código o cifra leída, no una medición
ejecutada sobre este árbol. Aquí no se registra ningún `§`.

| Pieza | Dónde se leyó | Estado a la entrada |
| --- | --- | --- |
| D2, línea base | `docs/plan-diagnostico.md:31-36` (~15 ms suelto, ~30 ms con `R`, ~200 ms enlazando `res/`) | cifras de D1 transferidas, en verificación |
| Marco cacheado | §232 en `docs/evidencia/11-emulador-check.md:1016-1045`; sonda en `apkc/marcocache_test.go:187` | caché implementada; sus medianas son de otra medición, en verificación |
| I5, índice del jar | `docs/plan-incremental-web.md:306-361` | medido por debajo del criterio (82 ms de ahorro frente a 100 ms); se reutiliza como corrección, nunca como cifra de D2 |
| D4, asignación cualificada | tests en `apkc/check_test.go:205-334`; resolución en `apkc/javaemisor.go:8526-8542`; paridad en `apkc/javaemisor_argumento_estatico_paridad_test.go:86` | código y tests presentes, sin `§` propio; en verificación |
| Columna del parser | `apkc/check.go:42-103`, `apkc/javafuente.go:20-37`, `apkc/check_test.go:550-581` | los errores de token ya llevan columna; las sentencias siguen con solo línea; en verificación |
| D5, overhead | `studio/diagnostico-vivo.js:203-259,287-325`; `studio/sonda-check.js:120-262`; `studio/hospedador.js:155-250` | sonda CDP, salto de pasadas limpias y una instancia por comando: sin medición sobre este árbol; en verificación |
| D6, matriz Java/XML | `studio/autocompletado-datos.js`, `studio/xml-editor.js`, `studio/prueba.js`, `studio/prueba-pestana.js` | cobertura implementada, sin matriz medida ni `§`; en verificación |
| Numeración | `docs/evidencia.md:357` en este árbol; resolución del orquestador en `:358` | `§327-328` son J5; este frente usa `§329`-`§333` y declara `§334+` libres |

### Deuda conocida dentro de D1

- **Los errores del parser llevan línea pero no columna.** La columna se toma del token que abrió
  la expresión (`pos.columna`), y las posiciones de sentencia del analizador solo guardan la línea
  (`pos{linea: …}`): darles columna exige tocar cada sitio donde se construye una sentencia y
  comprobar que la `LineNumberTable` no se mueve (los dex se comparan byte a byte). Se deja escrito
  para no fingir que está hecho: en el JSON, `columna` falta y `posicion` sale sin ella.
- **Un `res/` que el enlazador de Go no sabe escribir** (un drawable XML, por ejemplo) deja el
  check sin `R`; el informe lo dice en `recursos` y el error del fuente es el de verdad —«no se
  resuelve R»—, nunca un verde falso. Cuando eso pase, la salida buena es D2/D3: leer los nombres
  del `R.java` que dejó el build de aapt2 o pedirle al usuario que compile.

## Fuera de esta cola

- Un LSP completo o un servidor de lenguaje: la regla 4 del `AGENTS.md` prohíbe runtimes ajenos y
  la pestaña compila del lado del cliente; D3 se queda en sugerencias propias sobre el `android.jar`
  y el árbol del proyecto.
- Diagnóstico de ficheros que no son `.java` (XML de recursos): el panel de Inicio ya tiene las
  reglas de `app.js` sobre la salida de `apkc`, y no es lo mismo que el diagnóstico en vivo.
