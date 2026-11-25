# Evidencia — §334-§339

Medido el 2026-11-24 sobre el worktree aislado
`/home/leonisauro/.local/share/opencode/worktree/c6f089/emulador-fiel-f0-f5`, base `f42b187` y
submódulo `studio/` en `035744f`. Todo el trabajo es de un solo builder secuencial F0 → F1-F4 → F5.
Temporales en `$HOME/.cache/apkc-web/emulador-fiel/`, nunca en `/tmp`. Go fijado 1.24.0 por
`tools/entorno.sh`; el prefijo literal de los comandos Go de abajo es:

```sh
PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH GOTMPDIR=$HOME/.cache/apkc-web/emulador-fiel/gotmp HOME=/home/leonisauro
tcr -c 2 -m 1536M -r 256M -- go …
```

**Frontera común.** No se corrió CDP, chromium, cell ni red: el plan condiciona esa parte (b) a
autorización expresa, que hoy no se dio; la medición original citaba además Tctl ≤43 °C, retirado
como bloqueo el 2026-11-24 por decisión del usuario (`ESTADO-SESION.md` §6). Por eso §337 y §338
tienen su mitad CDP **en verificación**, y el cierre no se da por «hecho» por analogía con
§287/§288/§293/§295/§317.
Tampoco se corrió gate ni publicación, y no se hizo `push`. La crítica final (`critico`) la lanza el
orquestador al integrar; un subagente con `subagent * deny` no delega.

## 334. F0: integra la fidelidad del §311 nunca incorporada

El diff vivía sin commitear en `…/fidelidad-b-311/studio` sobre `42d6787`; los tres ficheros que
toca son idénticos entre `035744f` y `42d6787`, así que aplica limpio sobre la base del plan:

```sh
F=/home/leonisauro/.local/share/opencode/worktree/c6f089/fidelidad-b-311/studio
git -C "$F" diff -- previa-panel.js prueba-navegador.js prueba.js > "$HOME/.cache/apkc-web/emulador-fiel/311.patch"
git -C studio apply --check "$HOME/.cache/apkc-web/emulador-fiel/311.patch"
git -C studio apply "$HOME/.cache/apkc-web/emulador-fiel/311.patch"
```

```text
patch lines: 283
APPLY CHECK OK
rc=0
 studio status: M previa-panel.js, M prueba-navegador.js, M prueba.js
```

Lectura en origen de las cuatro aseveraciones pedidas:

```sh
cd studio
grep -n 'MARCA_FIDELIDAD_311\|numeroSecuencia === 1' previa-panel.js | head
grep -n 'MARCA-FIDELIDAD-311' prueba.js
grep -n 'lienzoInicial\|lienzoFinal\|foco\|espacio\|720\|1280' prueba-navegador.js | head
```

```text
previa-panel.js:1623:  const MARCA_FIDELIDAD_311 = "MARCA-FIDELIDAD-311";
previa-panel.js:1643:        if (!marcada && numeroSecuencia === 1) {
previa-panel.js:1644:          li.dataset.fidelidad = MARCA_FIDELIDAD_311;
prueba.js:1243: if (!/MARCA-FIDELIDAD-311/.test(fuentePreviaEstado) ||
prueba.js:1245:     !/numeroSecuencia === 1/.test(fuentePreviaEstado)) {
prueba-navegador.js:913: // §311: acoplar no puede reescalar el canvas; se mide antes y después de los cuatro bordes.
prueba-navegador.js:920: const lienzoInicial = cajaLienzo();
prueba-navegador.js:947: lienzoInicial, lienzoFinal: cajaLienzo() };
prueba-navegador.js:1163: // §311, escenario de foco: un toque real sobre el rect de un EditText abre el campo; el texto
prueba-navegador.js:1164: // tecleado viaja al guion sin recortar sus espacios y el campo se cierra al perder el foco.
prueba-navegador.js:1708: const estrecho = await sondaEstrecha(720);
prueba-navegador.js:1709: const anchoNormal = await sondaEstrecha(1280);
```

Verificación de hoy (la parte CDP de `prueba-navegador.js` queda en el bloque (b)):

```sh
cd studio && node --check previa-panel.js && node --check prueba.js && node --check prueba-navegador.js && node prueba.js
```

```text
reglas: 24 | ancladas al fuente de apkc o a una salida capturada: 21 | sin captura propia: 3
prueba: todo en orden
rc=0
```

Commit del submódulo: `248bd97 feat(previa): integra la fidelidad del §311 (consola, bordes y teclado)`,
autor `Leonisaurov <leonisaurov@gmail.com>`.

## 335. F1: la lista de notas enseña N filas también en la sesión viva

La falsación se hizo primero, con un test temporal que arrancó una sesión sobre
`$HOME/.cache/apkc-web/apps-ejemplo/notas.apk` y entregó las órdenes una a una. Salida **antes** del
cambio:

```text
orden 0 [--escribir id/editor/Uno] -> rc=0 filas=0 avisos=[el getCount de la lista 0x7f010004 falló: android.widget.ArrayAdapter no tiene getCount()I]
orden 1 [--pulsar id/guardar] -> rc=0 filas=1 avisos=[el getCount de la lista 0x7f010004 falló: android.widget.ArrayAdapter no tiene getCount()I]
...
orden 4 [--pulsar id/guardar] -> rc=0 filas=2 avisos=[el getCount de la lista 0x7f010004 falló: android.widget.ArrayAdapter no tiene getCount()I]
```

Causa confirmada: el `redibujar()` inicial pasa un `String[0]`; en
`apkc-emulador/previewinterprete.go:4176`, `append([]any(nil), arr.datos...)` con cero datos devuelve `nil`,
así que `elementos` queda nulo y `cuentaDelAdaptador` cae en un `getCount` que solo existe como stub.
`apkc/preview.go:1046` buscaba la lista solo por el id hexadecimal formateado.

Corrección en tres puntos: `make([]any, len(datos))` + `copy` (el arreglo vacío deja una lista no
nula), respaldo por `esDe(..., ArrayAdapter)` en `cuentaDelAdaptador` aunque `elementos` esté vacío,
y búsqueda del nodo por hexadecimal **o** decimal y por el texto del id. Caso nuevo y autocontenido
en `apkc/previewlistas_test.go`: `TestElPreviewPintaDosNotas`, que mide 0 filas sin avisos en reposo,
2 filas («Nota 0», «Nota 1») tras dos altas, y las mismas 2 filas en la sesión viva tras un
`--intercambiar` con el mismo dex (hot swap sin replay).

```sh
cd apkc
tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run TestElPreviewPintaDosNotas -v .
```

Salida con la corrección:

```text
=== RUN   TestElPreviewPintaDosNotas
--- PASS: TestElPreviewPintaDosNotas (0.03s)
PASS
ok  	github.com/ajinasokan/apkc	0.03s
rc=0
```

Y la misma prueba **sin** el cambio (`git checkout -- previewinterprete.go preview.go`, test, y
restauración desde copia) falla con la causa:

```text
previewlistas_test.go:537: la sesión dejó avisos: [el getCount de la lista 0x7f010000 falló: android.widget.ArrayAdapter no tiene getCount()I]
previewlistas_test.go:557: el hot swap dejó avisos: [el getCount de la lista 0x7f010000 falló: android.widget.ArrayAdapter no tiene getCount()I]
--- FAIL: TestElPreviewPintaDosNotas (0.02s)
rc_sin_fix=1
```

La sesión viva y el hot swap preexistentes se repitieron después del cambio:

```text
--- PASS: TestElPreviewPintaUnaListaConSuAdaptador (0.02s)
--- PASS: TestLaSesionAplicaSoloLaOrdenNueva (0.02s)
--- PASS: TestLaSesionConservaElModeloAcumulado (0.03s)
--- PASS: TestLaSesionAplicaDeltaEnElCaminoReal (0.04s)
--- PASS: TestElSegundoDeltaUsaLaBaseActualizada (0.05s)
PASS
ok  	github.com/ajinasokan/apkc	0.281s
```

**Rectificación de la hipótesis del plan.** La siembra literal de la hipótesis
(`--escribir id/editor/Uno --pulsar id/guardar --escribir id/editor/Dos --pulsar id/guardar`) produce
**una** fila («Dos»), no dos: la app fija `elegida = cuantas - 1` al guardar y la segunda escritura
edita la nota ya elegida; para dos notas hay que pulsar `id/nueva` entre ambas. Con esa secuencia el
APK real da `sel= 'Nota 2 de 2'` y `lista hijos= 2 ['Uno', 'Dos']`. El test permanente usa el esqueleto
equivalente con `id/pie` como botón de alta, sin depender de un binario en disco.

Commit del superproyecto: `9026a80 fix(preview): cuenta el ArrayAdapter vacío y pinta las notas de la sesión viva`.

## 336. F2: cinco botones dentro de 360 dp, y no-intersección separada

`apkc preview` sobre `notas.apk` ya colocaba los cinco botones dentro del marco, con estos rects:

```text
[('id/anterior', 0, 80, 80), ('id/siguiente', 80, 88, 168), ('id/nueva', 168, 56, 224), ('id/guardar', 224, 72, 296), ('id/borrar', 296, 64, 360)]
```

El desborde lo introducía `modeloConMinimos` al ensanchar cada botón a 88 dp. Se añadió un reflow de
fila en `studio/vista-previa.js` (reduce proporcionalmente anchos y huecos conservando el centro
relativo y sin quitar ningún botón), el mismo criterio en `colocarLineal` de `studio/vista-xml.js` y
una guardia en `colocarHorizontal` de `apkc/preview.go` para que ninguna fila desborde su hueco.

```sh
cd studio && node --check vista-xml.js && node --check vista-previa.js && node --check prueba.js && node prueba.js
```

```text
rectsEscalados (cinco botones en 360 dp): [{"id":"id/anterior","x":0,"ancho":84.25531914893618,"texto":"Anterior"},{"id":"id/siguiente","x":80.42553191489363,"ancho":84.25531914893618,"texto":"Siguiente"},{"id":"id/nueva","x":149.36170212765958,"ancho":84.25531914893618,"texto":"Nueva"},{"id":"id/guardar","x":210.63829787234044,"ancho":84.25531914893618,"texto":"Guardar"},{"id":"id/borrar","x":275.74468085106383,"ancho":84.25531914893618,"texto":"Borrar"}]
paresBotones (huecos de 64 dp): []
prueba: todo en orden
rc=0
```

Los cinco textos siguen intactos (`Anterior`, `Siguiente`, `Nueva`, `Guardar`, `Borrar`): no se podó
contenido. El caso de no-intersección es **separado**, con su propio modelo de huecos de 64 dp; no se
afirma solape en `notas` (sus botones se tocan en un borde). La aseveración de §293 —botón único en
`x=-12, y=28, 88×40`, escalado a `-24, 56, 176×80`— sigue en `prueba.js:888-906`, sin cambios, y
pasa. El conjunto focal de Go vuelve en verde (`PASS`, `ok … 0.281s`, §335).

Commits: submódulo `8969351 fix(previa): refluye la fila de botones para que cinco quepan en 360 dp`;
superproyecto `6ca2341 fix(preview): refluye las filas horizontales que desbordan el marco`.

## 337. F3: capas y recorte

`vista-previa.js:591-594` conserva `ctx.save()/beginPath()/rect(marcoRect)/clip()`. Se añadió
`recortaACaja(rect, caja)` (puro, exportado) y `colocarCampoTexto()` lo usa para que el `<input>` del
EditText no se vea nunca fuera del canvas. Las aseveraciones de node son propias:

```sh
cd studio && node --check vista-previa.js && node --check previa-panel.js && node --check prueba.js && node --check prueba-navegador.js && node prueba.js
```

```text
prueba: todo en orden
rc=0
```

El bloque CDP añade en `prueba-navegador.js` la lectura de `getBoundingClientRect()` del campo y del
canvas y una `comprueba` de contención con margen de 1 px; **no se ejecutó**: queda en verificación,
como la lectura con `Runtime.evaluate` y la captura de Droidloom previstas en el plan. No se afirma
que el `getBoundingClientRect()` real esté en verde.

Commit del submódulo: `a72965d fix(previa): recorta la capa del EditText al canvas y conserva el clip`.

## 338. F4: el EditText usa el tema del aparato

`style.css` cambia `.previa-campo-texto` a `background: #ffffff`, `color: #30323a`,
`caret-color: #30323a` y `border: 1px solid transparent`, y añade
`.previa-campo-texto:focus` para que la regla genérica `input:focus { border-color: var(--foco) }` no
reintroduzca `#007fd4`. Se mantienen `previa-panel.js` (borrado de `textoDinamico`) y
`vista-previa.js:628-635` (subrayado) y `:667,689-697` (texto en reposo).

`prueba-pestana.js` devuelve ahora los cuatro estilos computados, normalizados con
`.replace(/\s/g, "")` antes de compararlos con los literales exactos del plan:

```text
fondo === "rgb(255,255,255)"    color === "rgb(48,50,58)"
caret === "rgb(48,50,58)"       borde === "rgba(0,0,0,0)"
```

Falsación de la aseveración de origen (devolver `color-mix(... var(--superficie2) 88% ...)` y
`var(--texto-fuerte)`, ejecutar y restaurar):

```text
  FALLO: style.css no da al EditText el tema del aparato en reposo
rc_sin_fix=1
=== restaurado ===
prueba: todo en orden
```

La mitad CDP (`getComputedStyle` sobre chromium y cotejo con la captura de Droidloom) **no se corrió**;
§338 queda en verificación para esa parte. Los cuatro literales y la normalización se aseveran hoy en
origen desde `prueba.js`, que da `prueba: todo en orden`.

Commit del submódulo: `4552426 fix(editor): el EditText del emulador usa el tema del aparato`.

## 339. Cierre del frente (estado propio)

Comprobaciones finales sobre el árbol final:

```sh
cd "$W"
PATH=/home/leonisauro/sdk/go1.24.0/bin:$PATH gofmt -l apkc/preview.go apkc/previewinterprete.go apkc/previewlistas_test.go
(cd studio && for f in app.js previa-panel.js vista-previa.js vista-xml.js prueba.js prueba-navegador.js prueba-pestana.js; do node --check "$f"; done && node prueba.js)
(cd apkc && PATH=… tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElPreviewPintaDosNotas|TestElPreviewPintaUnaListaConSuAdaptador|TestLaSesion' -v .)
```

```text
gofmt -l: (sin salida)
app.js: OK / previa-panel.js: OK / vista-previa.js: OK / vista-xml.js: OK /
prueba.js: OK / prueba-navegador.js: OK / prueba-pestana.js: OK
prueba: todo en orden
--- PASS: TestElPreviewPintaDosNotas (0.02s)
--- PASS: TestElPreviewPintaUnaListaConSuAdaptador (0.02s)
--- PASS: TestLaSesionAplicaSoloLaOrdenNueva (0.03s)
--- PASS: TestLaSesionConservaElModeloAcumulado (0.03s)
--- PASS: TestLaSesionAplicaDeltaEnElCaminoReal (0.03s)
--- PASS: TestLaSesionNoLlamaExitPorOrden (0.01s)
--- PASS: TestLaSesionNoPegaElErrorDeUnaOrdenMala (0.01s)
PASS
ok  	github.com/ajinasokan/apkc	0.174s
rc=0
```

`node --check` no se aplica a `style.css` (CSS no es JavaScript): sus reglas se comprueban con las
aseveraciones de origen de `prueba.js` (§338).

La suite Go completa da un único fallo, `TestIndiceDelJarSeReutilizaYSeInvalida`
(`jarindice_test.go:151`, «la prueba necesita el mismo tamaño para probar el contenido: 416 y 414»),
que ya está documentado como fallo en árbol limpio al cierre de §314 y no toca código de preview:

```text
--- FAIL: TestIndiceDelJarSeReutilizaYSeInvalida (0.00s)
    jarindice_test.go:151: la prueba necesita el mismo tamaño para probar el contenido: 416 y 414
FAIL	github.com/ajinasokan/apkc	282.724s
```

Estados y SHAs finales:

```text
git -C "$W/studio" status -sb  → ## HEAD (no branch)
git -C "$W" status -sb         → ## HEAD (no branch)
                                 M studio
git -C "$W" rev-parse HEAD     → 6ca2341f850d3dad064307a7540f7ea35ae0f1cd
git -C "$W/studio" rev-parse HEAD → 45524264569d543ee8a0e8de748cb858658f7ad2
```

El puntero del submódulo se fija en el commit del superproyecto que cierra esta documentación. Los
cinco commits del submódulo (`248bd97`, `8969351`, `a72965d`, `4552426`) viven en el object store del
worktree y el orquestador debe recuperarlos antes de integrar. No hay push, publicación ni gate.
