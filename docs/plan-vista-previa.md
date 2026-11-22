# Plan de vista previa de APK en apkc-studio

**Estado: objetivo próximo, sin implementar.** Fase 0 registrada; fases 1 a 4 pendientes. Este
fichero es un plan, no evidencia: lo que aquí se afirme sobre coste, compatibilidad o formato es una
hipótesis y lleva el comando con el que se comprobará. Cuando haya mediciones reales, van a
`docs/evidencia.md` como un apartado nuevo con su comando, y el estado de las fases a `docs/plan.md`
(regla 7 del `AGENTS.md` de la raíz).

Fecha: 2026-11-22; naturalizado sobre el estado real del repo tras §198. Origen de la petición: el
usuario quiere ver en el editor la app que acaba de compilar, y pregunta si puede basarse en
WineDroid e implementarlo en Go. El objetivo próximo es **este plan**, no las fronteras de §194.

## Por qué no se porta WineDroid

WineDroid demostró con una demo pública que se pueden renderizar APKs en el navegador sin arrancar
un sistema Android. No es el camino de este repo, por tres razones que no son negociables:

1. **Licencia.** WineDroid es GPL-3.0-only. Traducir su código Rust a Go sería una adaptación de la
   obra (GPL 3, apartado 0) y arrastraría un relicenciamiento de todo el sitio, hoy MIT en `LICENSE`
   y en `studio/LICENSE`. Estudiar su comportamiento está permitido; copiar su expresión, no.
2. **Regla 4.** `AGENTS.md` fija «sin conexión y sin runtimes de terceros» por decisión del usuario
   de 2026-10-27. El runtime de WineDroid se compila con Emscripten y trae su propio cargador JS,
   mientras que el studio usa Go wasm con `wasm_exec.js` y un worker propio.
3. **Arquitectura.** Meter un segundo runtime obligaría a ampliar `sw.js`, `tools/sitio.sh` y el
   proceso de construcción con `emcc`, además de un segundo worker para pasar los bytes del APK
   desde OPFS. Todo eso para una licencia que además obligaría a cambiar el `NOTICE.md`.

La base correcta es otra, y es la que ya estaba en casa:

- **NekoDroid es MIT** y contiene las piezas conceptuales: `src/android/dex.rs` (parser DEX 035),
  `src/android/decode.rs`, `src/android/opcodes.rs` y `src/android/interp.rs` (decodificador e
  intérprete Dalvik del subconjunto MVP), `src/android/hle.rs` (stubs de `Log`, `println` y
  `TextView.setText`) y `src/android/axml.rs` (parser de AXML). Se lee como referencia de
  comportamiento y se escribe código Go propio.
- **`apkc/` ya sabe del formato**: `axmlwriter.go` y `arscwriter.go` escriben AXML y `resources.arsc`
  hoy mismo. El lector es el camino inverso y se apoya en ese conocimiento, no en código ajeno.

Los nombres, las tablas de primitivas, los mensajes de error y el orden de arranque de WineDroid no
se reproducen: ahí es donde estaría la expresión protegida.

## Restricciones de partida

- **R1, licencia.** Todo código nuevo es MIT. No se incorpora ni se traduce código de WineDroid.
  Se anotará en `NOTICE.md` que el módulo de vista previa es implementación propia.
- **R2, sin red y sin runtime ajeno.** Cero peticiones fuera del origen; `prueba-pestana.js` ya lo
  comprueba. La vista previa corre dentro de `apkc.wasm`.
- **R3, sin binarios versionados.** `*.wasm` se ignora y se construye con
  `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build` desde `apkc/`.
- **R4, ids contrato.** `prueba.js` exige que todo `getElementById` exista en `index.html`. Ids
  nuevos implican prueba nueva.
- **R5, un claim una medición.** Ninguna cifra de tamaño o tiempo entra en evidencia sin su comando.
- **R6, submódulo.** Lo que se toque en `studio/` se commitea primero en `apkc-studio` y después se
  actualiza el puntero aquí, en el mismo commit que lo que lo necesite.
- **R7, idioma.** Prosa y comentarios en español con ortografía completa y sin caracteres de otros
  alfabetos.
- **R8, binario y headers del sitio actual.** El `apkc.wasm` actual lleva verificación de
  tamaño y SHA-256, carga por rangos de 256 KB si el host trunca (§196), Trusted Types y CSP con meta
  en la primera navegación (§188 y §195). Si el lector y el intérprete aumentan el binario, hay que
  repetir esa medición y comprobar que el fallback por rangos sigue funcionando antes de publicar.

## Hipótesis

- **H1.** Con un lector de AXML escrito en Go, simétrico de `axmlwriter.go`, es posible extraer de
  un APK generado por el propio studio un modelo de layout útil: jerarquía de nodos, tipo de view,
  identificador, texto y atributos de posición y tamaño.
- **H2.** Con un lector mínimo de `resources.arsc`, simétrico de `arscwriter.go`, se pueden resolver
  las referencias `@string` y `@id` que aparecen en ese layout.
- **H3.** Un intérprete Dalvik MVP escrito en Go puede ejecutar `onCreate` de la actividad de la
  plantilla Contador y capturar el texto que la app envía a `TextView.setText`, sin JNI ni `.so`.
- **H4.** El modelo resultante se puede pintar como un esquema en un `<canvas>` nativo del editor,
  reutilizando las variables CSS de tema, sin librerías de terceros.
- **H5.** El coste en tamaño del binario y en tiempo de respuesta del comando permite usarlo tras
  cada compilación sin romper el flujo del editor. Sin cifras nuevas hasta medir. El binario actual
  son **13320296 bytes** y el gzip **3510326 bytes**, con SHA-256 `88aeeb59…b05a419` y
  `db1bf7ca…aa3` respectivamente, medidos en `sitio/wasm-manifest.json` y §198; el tiempo de primera
  carga y de primer build está en §189. Cualquier incremento se medirá con el mismo método y con el
  verificador de publicación (§196).

## Fases

### Fase 0. Registro de la decisión — HECHA

- **T0.1.** Fichero en `docs/plan-vista-previa.md` con hipótesis y restricciones, naturalizado sobre
  el estado real del repo (§198) y con fila propia como objetivo próximo en `docs/plan.md`.
- **Criterio de aceptación.** Cumplido: las cifras del binario son las del manifiesto actual, los
  comandos usan las rutas reales de este entorno y no hay afirmaciones medidas sin evidencia.
- **Abandona.** Portar WineDroid a Go y el botón que abre la demo externa.

### Fase 1. Lectura de recursos en Go, sin interfaz

- **T1.1.** Lector de AXML binario en `apkc/`, simétrico de `axmlwriter.go`, para
  `res/layout/*.xml` y `AndroidManifest.xml`.
- **T1.2.** Lector mínimo de `resources.arsc`, simétrico de `arscwriter.go`, para resolver las
  cadenas y identificadores que use el layout.
- **T1.3.** Comando `apkc preview` que lea el APK generado por el studio y escriba en la salida un
  JSON con el modelo de la vista: nodo, tipo, identificador, texto, hijos y rectángulo aproximado.
- **Criterio de aceptación.** Sobre el APK de la plantilla Contador que produce `e2e.sh`, el JSON
  es válido, las cadenas del layout aparecen resueltas y el comando corre en el hospedador de
  navegador sin levantar ningún proceso. Se ejecuta con el binario que construye `tools/entorno.sh`:
  `node shim/navegador-sim.js --montar "$PREFIX/tmp/app.apk:/in.apk" "$HOME/.cache/apkc-web/tmp/apkc-wasm.wasm" preview /in.apk`
  (en Termux, `HOME` y `PREFIX` son los de `/data/data/com.termux/files`). El mount va antes del
  `.wasm` y antes de la orden, como en la firma de §183.
- **Medición a incorporar.** Tamaño del JSON, número de nodos y tiempo en el hospedador y en
  chromium, con el comando exacto que lo produzca, en un apartado nuevo de `docs/evidencia.md`.
- **Abandona.** Estilos fieles, imágenes, drawables y tema del sistema.

### Fase 2. Intérprete Dalvik MVP en Go

- **T2.1.** Reutilizar el lector DEX 035 que ya existe en el fork (`apkc/dexreader.go`,
  `dextabla.go`, `dexinstr.go`, validado en §20-§23 y §55-§60) en lugar de escribir otro parser. Se
  lee como referencia de comportamiento el parser MIT de NekoDroid; no se copia su código. Si falta
  algún campo para el intérprete, se amplía el lector propio con su test.
- **T2.2.** Decodificador del subconjunto de opcodes MVP y máquina de registros.
- **T2.3.** Stubs de `Log`, `println`, `printInt` y `TextView.setText`, para capturar el texto que
  la actividad escribe en pantalla.
- **T2.4.** Elección de la actividad lanzadora desde el manifiesto. Debe preferir una clase con
  `main` u `onCreate` e ignorar clases anónimas con el signo de dólar, que es el fallo que se
  encontró al probar NekoDroid con un APK real.
- **Criterio de aceptación.** `onCreate` de la plantilla Contador corre dentro de wasm y produce
  texto capturado. Se comprueba primero en node y después en chromium con `prueba-pestana.js`.
- **Abandona.** JNI, bibliotecas `.so`, multidex y cualquier API de framework fuera de los stubs.

### Fase 3. Vista previa en el editor

- **T3.1.** Vista o panel nuevo en `studio/index.html` con un `<canvas>`, tercer botón en
  `nav.actividad` (hoy solo Inicio y Editor, `index.html:114-119`) y su pestaña en `.pestana-vista`.
  Ids nuevos documentados en `studio/EDITOR.md` y verificados en `prueba.js` (R4). El mundo visual
  sigue siendo `Operate`: el canvas usa las variables CSS de tema y no añade animaciones ni una
  firma nueva (véase `studio/DESIGN.md`).
- **T3.2.** `vistas.js` añade la vista nueva y actualiza `document.body.dataset.vista`.
- **T3.3.** `pestana.js` llama a `apkc preview` después de compilar y pinta el modelo en el canvas
  con las variables de tema existentes. El canvas es de solo lectura.
- **T3.4.** `sw.js` sigue cacheando el mismo `apkc.wasm` y `wasm_exec.js`: no hay artefactos nuevos
  si todo el código vive en el binario del fork. Se comprueba que la recarga offline sigue funcionando.
- **Criterio de aceptación.** `prueba-pestana.js` pasa con las comprobaciones nuevas (hoy 87, §197),
  no aparece ninguna petición fuera del origen y el gate completo sigue diciendo «pestaña: N
  comprobaciones» sin los avisos «sin chromium» ni «sin APKC_WASM». La vista nueva no debe romper el
  diagnóstico en vivo ni la paleta `Ctrl+Mayús+P`, que ya están en esa misma prueba.
- **Abandona.** Edición visual del layout, arrastrar views y renderizado de fuentes e imágenes.

### Fase 4. Cierre y registro

- **T4.1.** Apartado nuevo en `docs/evidencia.md` con los comandos y resultados de las fases 1 a 3.
  Fila nueva en el inventario de la fase 5 de `docs/plan.md` si el subconjunto acepta o rechaza
  algo distinto, y recap en `docs/situacion.md`.
- **T4.2.** `e2e.sh` incorpora la comprobación de la vista previa y pasa el gate completo.
- **T4.3.** `NOTICE.md` declara que el módulo de vista previa es código propio MIT y que no se
  incorporó código de WineDroid. Commit en `apkc-studio` y actualización del puntero del submódulo
  aquí.
- **Criterio de aceptación.** Gate verde y árbol limpio en ambos repositorios.

## Riesgos y preguntas abiertas

- `resources.arsc` de la plantilla puede no traer las entradas suficientes para resolver todo el
  layout. Se mide en la fase 1 antes de diseñar el lector completo.
- Añadir un intérprete puede incrementar el binario wasm. No se afirma cuánto hasta repetir la
  medición de §198. El host actual trunca por HTTP/2 y el sitio sobrevive con trozos de 256 KB
  (§196): si el gz crece, hay que volver a comprobar ese camino y el hash publicado antes de dar
  nada por bueno.
- Un esquema con `<canvas>` no sustituye al render de WineDroid. Si el usuario necesita ver fuentes,
  imágenes y estilos reales, la vista previa no llegará y habrá que volver a evaluar un runtime
  externo con su cambio de licencia.
- El alcance se limita a los APK que produce el propio studio: un `classes.dex`, sin `.so` y con las
  llamadas stubbeadas. Otros APK quedan fuera de la hipótesis.

## Lo que no es este plan

No es un emulador de Android, no corre APKs de Play Store y no pretende sustituir a WineDroid ni a
Vetro. Es una forma de ver el resultado del compilador del propio repo sin introducir un runtime de
terceros ni cambiar la licencia del proyecto.
