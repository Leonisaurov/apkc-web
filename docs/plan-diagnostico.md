# Plan de mejora del diagnóstico de apkc-studio

Orden acordado con el usuario el **2026-11-22**, a raíz de un falso error del diagnóstico en vivo:
al escribir `setContentView(R.layout.main);` en `MainActivity.java`, la pestaña decía *el tipo de
este argumento no esta medido*. La causa y su cierre están en `docs/evidencia.md` §200.

La cola es **D1 → D2 → D3** y se cierra en ese orden: primero que el diagnóstico diga la verdad y
señale el sitio, después que llegue antes, y por último que proponga. Igual que en
`docs/plan-mejoras.md`, esto es un **plan**, no un registro: aquí no entran hallazgos, y cada ítem
se cierra con su sonda, su `§` en `docs/evidencia.md` con el comando que lo reproduce y su commit.
Ninguna casilla se marca «hecha» sin evidencia propia sobre el árbol final (regla 10 de
`AGENTS.md`).

| # | Ítem | Criterio de aceptación | Cómo se reproduce |
| --- | --- | --- | --- |
| **D1** | **Diagnóstico correcto y accionable** | (a) `setContentView(R.layout.main)` en la plantilla del studio da **cero errores**; (b) todo error lleva `archivo:línea` y, cuando nació en un token, también `columna`; (c) el mensaje dice qué no se resuelve o qué no está medido, en español y sin repetir la posición; (d) el subrayado de `#diagnostico-marcas` cae sobre el token, no sobre la línea | `apkc check --android-jar … --proyecto … MainActivity.java`; `apkc/check_test.go`; `APKC_WASM=… node studio/prueba-pestana.js` |
| **D2** | **Más rápido** | medir el coste real de un `check` en vivo (hoy: debounce de 350 ms en `studio/diagnostico-vivo.js`; el trabajo del worker va aparte) y bajar la latencia sin saturar el worker, **solo con cifras**: reutilizar la instancia de wasm, cachear el marco del `android.jar`, cancelar los `check` obsoletos | la sonda de D2, por escribir: tiempos de `apkc check` nativo con y sin `--proyecto`, y del mismo comando en la pestaña |
| **D3** | **Autocompletado** | identificar de dónde saldrían los ids de recursos (`R.layout.*`, `R.id.*`) y los métodos del jar, y diseñar la fuente de datos **sin red**: qué se lee del `android.jar` del propio origen y qué del árbol del proyecto | la sonda de D3, por escribir: tamaño y tiempo de leer los nombres del jar y del `R.java` del proyecto |

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
