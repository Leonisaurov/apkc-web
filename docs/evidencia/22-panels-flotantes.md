## 346. Persistencia versionada y z-order de paneles

Medida el 2026-11-24 en el worktree `panels-flotantes-f0-f7`, sobre el submódulo `studio/`
(commit `12b98f9`), sin chromium, sin Droidloom y sin gate. Cierra el frente F6 del plan
`…/plan-panels-flotantes/PLAN-panels-flotantes.md:43`.

**Comando** (directorio `studio/` del worktree):

```sh
node --check paneles.js && node --check previa-panel.js && node --check prueba.js && node prueba.js
```

**Salida.** Los tres `node --check` salen sin mensaje (código 0). La suite termina con
`reglas: 24 | ancladas al fuente de apkc o a una salida capturada: 21 | sin captura propia: 3`,
`  (medida: 20000 contextos Java en 145 ms y 20000 de XML en 79 ms; 40000 con resultado)` y
`prueba: todo en orden`, código de salida 0, sin `PRUEBA FALLA`. El aviso de node
`ExperimentalWarning: localStorage is not available because --localstorage-file was not provided`
no es un fallo: aquí la persistencia se falsa con funciones puras y con lectura del fuente, no con
un `localStorage` real.

**Qué se escribió.** `studio/paneles.js` añade tres funciones puras: `clavesDePanel(id)` entrega
`apkc-studio/paneles/v1/<id>/{modo,borde,x,y,ancho,alto,z}`; `preferenciaDeEstado(estado)` produce
un objeto serializable sin `NaN`; y `estadoDePreferencia(raw, base, limites)` acepta un objeto o un
texto JSON y devuelve un estado validado sin lanzar. `previa-panel.js` las integra en
`leerFlotante`/`guardarFlotante`: lee primero las claves versionadas, toma de las claves antiguas
(`apkc-studio/previa-inspector-*`, `apkc-studio/previa-lado-*`) solo los campos que faltan y sigue
escribiendo ambas series, así que la migración es de un solo uso y en los dos sentidos. Los campos
con basura y el JSON roto caen al estado base. `traerAlFrente(nombre)` sube `contadorZ` de 0 a 19,
escribe `estado.z` con el reducer puro y fija `--z` en el contenedor; se dispara en el
`pointerdown` del asa de `conectarFlotante`. El CSS de `.previa-inspector[data-modo=…]` pasa de
`z-index: 6` a `z-index: calc(40 + var(--z, 0))`, dentro de la banda 40-59 y por debajo del modo
editar (60) y de los diálogos (80+).

**Falsación.** Copia del submódulo en `/home/leonisauro/Develop/.tmp/panels-f6-kIXubY/` (creado
con `mktemp -d -p ~/Develop/.tmp`), con `falsar.py` que retira un cambio, corre `node prueba.js`
en la copia y restaura el fichero. Comando:

```sh
cd /home/leonisauro/Develop/.tmp/panels-f6-kIXubY
python3 falsar.py
```

Salida, una línea por caso y `resumen: 6/6 falsados`:

```text
FALSADO   | prefijo versionado | rc=1
FALSADO   | integracion en leer/guardarFlotante | rc=1
FALSADO   | JSON corrupto sin lanzar | rc=1
FALSADO   | --z al conectar | rc=1
FALSADO   | contador de z creciente | rc=1
FALSADO   | banda z-index en CSS | rc=1
```

Los cambios que se retiraron, en orden, fueron: `PREFIJO_PANELES` de `v1` a `v0`;
`clavesDePanel(f.id)` a `clavesDePanel(f.tipo)` en las dos funciones de persistencia; el
`try/catch` de `JSON.parse` en `estadoDePreferencia`; la llamada `traerAlFrente(nombre)` del
`pointerdown`; el incremento de `contadorZ`; y el `z-index: calc(40 + var(--z, 0))` del bloque
flotante (no el de `.modulo`). Cada caso devuelve `PRUEBA FALLA` o una excepción, nunca un verde.
La primera ronda dio `5/6` porque la aserción del CSS aceptaba cualquier `calc(40 + var(--z, 0))`
del fichero, incluido el de `.modulo`: se acotó al bloque `.previa-inspector[data-modo="libre"]`
y la segunda ronda dio `6/6`.

**Frontera de `§346`.** No se corrió `getBoundingClientRect()` en `prueba-pestana.js` ni en
`prueba-navegador.js`: ambas mediciones 1:1 del plan (`:46-55`) quedan **en verificación**, porque
exigen chromium por CDP, Tctl ≤43 °C y autorización expresa, y hoy no hay ninguna de las tres. El
z-order medido aquí es el del estado puro, el del fuente y el de la regla CSS; el orden visual
final en la pestaña no se reclama. **Aclaración posterior (decisión del usuario, 2026-11-24):** la
temperatura dejó de ser bloqueo; el motivo vigente para no correr CDP es la falta de autorización
expresa, no la lectura de `Tctl`. El umbral se conserva aquí como texto histórico de la medición.

## 347. Paneles flotantes para todos los módulos

Cierre del frente de los paneles flotantes (F0-F6) sobre
`…/plan-panels-flotantes/PLAN-panels-flotantes.md`. Los seis commits anteriores del submódulo son
`19d92b9` (§340, geometría pura), `e31b073` (§341, los dos defectos de la consola), `08e747b`
(§342, registro `MODULOS`), `c0727b6` (§343, panel inferior), `c2215f2` (§344, explorador y
editor), `b0d16c9` (§345, emulador y teléfono); el de la persistencia es `12b98f9` (§346).

**Comando** (sobre el árbol final del submódulo):

```sh
cd studio
node --check paneles.js && node --check previa-panel.js && node --check prueba.js && node prueba.js
```

**Salida.** La misma que en `§346`: tres `node --check` mudos y `prueba: todo en orden`, código 0,
sin `PRUEBA FALLA`. La suite cubre en un solo sitio la geometría movida a `paneles.js`, el default
340 × 240, la arista de enfrente quieta en los tiradores norte y oeste, los ocho tiradores y los
cuatro botones de acople de las dos capas, el id `panel-inferior` y su `aria-controls`, el
envoltorio sin renombrar ids, la preferencia versionada, la migración, la preferencia corrupta y el
z-order creciente. El envoltorio traslada nodos con `appendChild`: no clona, no cambia ids ni
reescribe `#codigo`, `estadoPestana` ni OPFS.

**Frontera de `§347`.** Quedan **en verificación**, sin ejecutar: (a) la medición 1:1 con
`getBoundingClientRect()` de `prueba-pestana.js` y de `prueba-navegador.js` (plan `:46-55`), que
necesita chromium por CDP con Tctl ≤43 °C y autorización expresa —el usuario retiró después la
temperatura como bloqueo, así que hoy basta la falta de autorización para mantenerla en verificación—;
(b) el oráculo visual de
Droidloom; (c) `tools/entorno.sh gate` (incluido `studio/e2e.sh`); y (d) la publicación en
`apkc-studio.surge.sh`. No se reclama cero peticiones fuera del origen, ni pestaña verificada, ni
sitio publicado por analogía con el `node prueba.js` verde. Tampoco se ha actualizado el puntero
del submódulo en el repo padre ni se ha hecho `push`; el orquestador lo integra.
