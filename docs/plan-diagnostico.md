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
- **D2 pendiente y sin medir.** Lo único que hay son los números de D1, que son la línea base:
  `check` de un solo fichero cuesta ~15 ms en nativo; con `--proyecto` y la `R` ya escrita en
  `src/`, ~30 ms; si hay que enlazar `res/` porque no hay `R.java`, ~200 ms, y de esos ~170 ms la
  mayor parte es releer y parsear `resources.arsc` del `android.jar` en cada invocación. En la
  pestaña, a eso se le suma el arranque de una instancia de wasm por comando. Son cifras de una
  máquina de escritorio, no de un teléfono: la sonda de D2 tiene que repetirlas en la pestaña.
- **D3 hecha el 2026-11-22** (§201, ampliación final). El editor sugiere recursos del proyecto
  (`R.layout.*`, `R.id.*`) y APIs del `android.jar` sin red: el jar se indexa por trozos
  (1 354 339 de 26 361 808 bytes, 3837 clases, ~50 ms, 0 peticiones), `Tab` acepta y entra en el
  undo tree, `Esc` cierra y las flechas recorren sin disparar otro `check`. El linter de XML
  (`xml-editor.js`) entró en la misma tanda: es local, no lanza `apkc check` y sus filas dicen
  `XML`. El gate final dio `pestaña: 170 comprobaciones` (§202, la cifra de entonces; con el árbol
  de hoy son 192). Fronteras: sin completado tras `v.`, bloques `/* … */` de varias
  líneas no detectados y recursos cacheados hasta `refrescar()`.

### Cola D4-D6 (ampliación del usuario, 2026-11-22)

- **D4, en cola.** El usuario golpeó el error concreto `Apk.java:14`: `this.filePath = filePath;`
  produce «una asignacion con mas de un nombre en el destino no esta medida». El builder del frente
  `jar/` (`deps-check`) está haciendo que `apkc check` vea el classpath completo, pero esa es una
  causa distinta: aquí hay que resolver el receptor `this` y el campo contra la clase en el
  analizador/emisor, sin mover la `LineNumberTable` (los dex se comparan byte a byte).
- **D5, en cola y sin medir.** D2 sigue con solo las cifras de D1 como línea base (~15 ms nativo,
  ~30 ms con `--proyecto`, ~200 ms si hay que enlazar `res/`). La ampliación pide además overhead
  mínimo: no crear una instancia de wasm por tecla, cancelar checks obsoletos y reutilizar I5.
- **D6, en cola.** D3 cubre `R.*` y APIs del jar, y el linter de XML es local; faltan la matriz de
  cobertura, el autocompletado tras `.` y el de atributos XML con su contexto, y mensajes que
  expliquen qué no está medido sin repetir la posición.
- Orden: **D4 → D5 → D6**, un agente por paso en cuanto se libere un hueco de los cuatro actuales.
  Ninguno se marca hecho sin su sonda, su `§` y su commit (regla 10 de `AGENTS.md`).

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
