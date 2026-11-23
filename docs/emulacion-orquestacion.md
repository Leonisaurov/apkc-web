# Coordinación de la orquestación de emulación (2026-11-22)

Este fichero es el tablero de la operación, no una medición. Las mediciones siguen en
`docs/evidencia.md` y los planes en `docs/plan-emulacion.md` y `docs/auditorias.md`.
Se actualiza al integrar cada frente para que la cola y el estado sobrevivan a una compactación.

## Objetivo

Dejar la vista previa y el emulador **completos y conectados**: E1.1 (correcciones de la auditoría)
y E2 a E7 del plan de emulación. Quedan fuera de esta operación el `android.jar`, el diagnóstico D2,
el incremental I3-I5, la deuda general de auditoría y las dependencias Maven. **No se publica, no se
empaqueta `sitio/` y no se hace push a `origin`;** eso solo cuando el usuario lo pida y el producto
esté terminado.

## Entorno común

- Repositorio raíz: `/home/leonisauro/Develop/Go/apkc-web` (rama `web`, por delante de
  `origin/web`; no se reconcilia hacia atrás ni se sube nada).
- `HOME_APKC_WEB=/home/leonisauro`, `ANDROID_HOME=/home/leonisauro/opt/android-sdk`.
- `APKC_ANDROID_JAR=/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar`
  (26.361.808 bytes). Es el oráculo local; no se sirve ni se versiona.
- Prueba base ejecutada en la raíz antes de lanzar nada:
  `APKC_ANDROID_JAR=… tcr -c 2 -m 1536M -r 256M -- go test -count=1 -run 'TestElPreviewNavegaEntreActividadesPropias|TestElPreviewNoAbreUnaActividadNoDeclarada' -v .`
  da `PASS`, `ok github.com/ajinasokan/apkc 0.073s`.
- Los comandos pesados van por `tcr -c 2 -m 1536M -r 256M -- …`. Si un test pide más memoria y
  sale con 137, se sube a `-m 2G` y se ejecuta en solitario, no en paralelo con otros.
- Hay ~2 GiB de RAM libre y ~6,7 GiB de swap libre: como máximo **tres constructores activos**.
  Los tests son enfocados (`go test -run …`), nunca la suite completa ni un gate.

## Reglas para todos los agentes

1. Trabajar **solo** en la ruta absoluta de su worktree; usar rutas absolutas en cada comando.
2. **No ejecutar** `tools/entorno.sh gate`, `test`, `wasm`, `banco`, `sitio` ni `publicar`; no
   lanzar chromium ni `prueba-pestana.js`. La verificación es el test enfocado que dice cada encargo.
3. No tocar `docs/`, `INFORMA-SEGURIDAD.md`, `sitio/` ni el submódulo `studio/`: la documentación la
   escribe el documentador al integrar y los cambios de la página se coordinan desde la raíz.
4. Sin red, sin descargas, sin `git push`, sin `surge`. Commit local con mensaje
   `<type>(<scope>): <summary>` en español cuando el frente esté verde.
5. Prosa y comentarios en español con ortografía completa y cero caracteres de otros alfabetos.
6. Si una API no se puede emular sin red, runtime ajeno o aparato, se rechaza por nombre con aviso
   y se escribe la frontera en el informe; no se finge.

## Worktrees y cola

| Frente | Worktree | PREFIX | Prueba de aceptación | Ola |
| --- | --- | --- | --- | --- |
| E1.1 auditoría (3.1-3.4 y D7) | `/home/leonisauro/.local/share/opencode/worktree/c6f089/emu-e1-audit` | `$HOME/.cache/apkc-web/emu-e1-audit` | pruebas enfocadas del intérprete + `TestElPreviewNavegaEntreActividadesPropias` | 2 |
| E2 listas y adaptadores | `…/emu-e2-listas` | `$HOME/.cache/apkc-web/emu-e2-listas` | `TestElPreviewPintaUnaListaConSuAdaptador` | 1 |
| E3 estado y persistencia | `…/emu-e3-estado` | `$HOME/.cache/apkc-web/emu-e3-estado` | `TestElPreviewConservaElEstadoAlRecrear` | 1 |
| E4 fidelidad visual | `…/emu-e4-visual` | `$HOME/.cache/apkc-web/emu-e4-visual` | `TestElPreviewRespetaVisibilidadTextoColorEImagen` | 1 |
| E5 entrada de usuario | `…/emu-e5-entrada` | `$HOME/.cache/apkc-web/emu-e5-entrada` | `TestElPreviewEscribeYMarcaLoQueSeLePide` | 2 |
| E6 reloj lógico | `…/emu-e6-reloj` | `$HOME/.cache/apkc-web/emu-e6-reloj` | `TestElPreviewAvanzaElRelojLogico` | 2 |
| E7 diálogos y avisos | `…/emu-e7-dialogos` | `$HOME/.cache/apkc-web/emu-e7-dialogos` | `TestElPreviewMuestraUnDialogoPropio` | 2 |

Orden de integración: E1.1, E2, E3, E4, E5, E6, E7. Cada frente se integra en la raíz con su
commit, pasa por `critico` y solo entonces se marca cerrado y se lanza el siguiente de la cola.
El documentador es una sola sesión que se reanuda por `sessionID` para no duplicar trabajo.

## Vista previa local

- Servidor: `APKC_WASM=$HOME/.cache/apkc-web/preview/apkc.wasm ANDROID_JAR=/tmp/androidjar-propio/artifact3/android-stub-34-full.jar STUDIO_SERVIDOR_PORT=8801 node studio/servidor.js`
  → **http://127.0.0.1:8801/**. Da COOP/COEP, sirve el jar generado de AOSP
  (`e2ddfb55…c8827`, el artifact del run `45062792689`) y no publica nada.
- Reconstruir el wasm con los cambios que ya estén en la raíz (o en un worktree concreto):
  `tcr -c 2 -m 2G -r 256M -- sh tools/preview-local.sh [raiz]`. El servidor lee el fichero en cada
  petición, así que no hace falta reiniciarlo; el navegador sí puede servir una copia del service
  worker, así que tras cada reconstrucción hay que recargar con `Ctrl+Mayús+R` y, si sigue saliendo
  código viejo, desregistrar el service worker desde las herramientas del navegador.
- El jar de AOSP es el que eligió el usuario; el de la SDK (`/home/leonisauro/opt/android-sdk/…`)
  sigue siendo solo el oráculo de javac en los tests enfocados.

## Estado

- Preparación: worktrees creados, `PREFIX` en disco, prueba base en verde.
- E1.1 **cerrada** en la raíz como `def0b85` (cherry-pick de `1e3b277`), con sus siete tests
  enfocados en verde y el crítico `ses_df9f4b884ffepDlg2y2VQsIBYZ` en **APROBAR**, sin defectos
  `CONFIRMADO`. Evidencia: §206 de `docs/evidencia.md`.
- E3 **cerrada** como `5cc852a`: `TestElPreviewConservaElEstadoAlRecrear` en verde y el crítico
  `ses_df9ef9e02ffeqSQ1cuPypAdVer` en **APROBAR**. Evidencia: §207.
- E4 **cerrada** como `99cd48f`: `TestElPreviewRespetaVisibilidadTextoColorEImagen` en verde y el
  crítico `ses_df9ef9e02ffeqSQ1cuPypAdVer` en **APROBAR**. H4, la condición del color `0x1d`, queda
  **cerrada** por `762b328`; H5 (`layout_margin` desde `@style`) se fija como **frontera** en §208.
- E2 **cerrada** como `a76422d` (integración manual de `d54fdea`), con
  `TestElPreviewPintaUnaListaConSuAdaptador` en verde y el crítico
  `ses_df9ecde81ffeVOW5GZ5bA820E6` en **APROBAR**, sin defectos `CONFIRMADO`; el merge de cinco
  puntos está verificado y queda una pregunta `INCIERTA` (sonda `--recrear` más lista). Evidencia:
  §209.
- E5 **cerrada** como `2dd70c8` (cherry-pick de `993d51b`, 21 conflictos resueltos): crítico
  `ses_df9c6f81effeRKtmnqwkLjBU4s` en **APROBAR CON CONDICIONES**, cumplida por el builder de E6 en
  `88d9b38` (diez tests juntos y paquete completo en verde). Evidencia: §210.
- E6 **cerrada** como `35d5f79`: 16 pruebas de regresión en verde (`ok … 1.174s`) y crítico
  `ses_df9bd547cffefBHTZrtCZdEv1i` en **APROBAR** en la ronda 3, sin defectos `CONFIRMADO`.
  Evidencia: §212.
- E7 **cerrada**: lógica aprobada por `ses_df9d13306ffehyseNhLIqoaYfh`, `usage` corregido en
  `2dd70c8` y `which` verificado en `969b045` por el QA `ses_df9c1482fffeKi6lTSOz6CBbk0`.
  Evidencia: §211.
- La vista previa actual se reconstruyó con el fix de excepciones (`sha256 48cff1be…`), verificada
  contra el `/apkc.wasm` servido. El fix del inspector fue solo CSS: el wasm no cambió.
- I3 **cerrada** como `d3803bb` tras el **APROBAR** de `ses_df9baefaeffemtyU4YC0gXywOR`; la pregunta
  abierta del classpath queda adscrita a I5, que sigue pendiente con I4. Detalle en
  `docs/plan-incremental-web.md`.
- **Interfaz del emulador:** integrada y verificada en la pestaña real con CDP
  (`http://127.0.0.1:8801/`, `crossOriginIsolated=true`, cero excepciones). Los dos puntos
  `INCIERTO` del crítico y el cierre visual final quedan cerrados:
  - **Última APK:** compilé debug desde la pestaña; `#ultimo-codigo` quedó en `0`, `/sdk/ultimo.apk`
    mide 10.286 B y `/sdk/ultimo.json` registra origen `/proyecto/build/app.apk`, tipo debug y
    SHA-256 `3d4b4382…`. «Usar última APK» pintó 53 nodos **sin compilar**.
  - **Inspector sin brincos:** antes de abrirlo el canvas medía 304×543 en (496,303) y al abrirlo
    296×528; la regla posterior `.marco-previa { position: relative }` anulaba la capa absoluta
    `.marco-vista-lado`. Corregido en `studio/style.css` con una regla más específica (`786e195`,
    puntero `23a827f`). Tras el fix el canvas mide 314×561 en (690,303) antes, apagado, encendido y
    con el inspector abierto: idéntico en los cuatro estados. El botón `#vista-oyentes button`
    pasa de «sin clics» a «1 clic» tras el ciclo apagado/encendido.
  - **Cierre visual final (integrado como `ff31ef6` y `0fce105`; los hashes del worktree `5637944`/`5f4b9ce` se reescribieron al cherry-pickear, puntero `89fc27f`, `studio/` en `0fce105`):**
    consola libre en las dos vistas, asa de arrastre con `pointer-events` y pantalla completa con
    `position: fixed`; acoplar la consola no reescala el canvas (`ajustarCanvas` memoiza `caja|dp`,
    `pausarAjusteCanvas` durante gestos y el `ResizeObserver` no ajusta con el emulador apagado).
    El gate anterior fallaba porque `previa-inspector-cab` tapaba el canvas (`elementFromPoint`
    devolvía la cabecera); se corrigió con `pointer-events: none` en la cabecera y `auto` solo en
    botones y asas (`previa-inspector-asidero`, `vista-lado-asidero`). CDP: apagado, canvas
    `309×712` en libre, acoplada y minimizada, con `elementFromPoint` = `previa-lateral-canvas`;
    encendido desde la última APK, `309×549` en los tres modos y el mismo hit-test; pantalla
    completa en el panel del editor, `505×899` en (708,54) con `full=true` y sin recarga, también
    en la vista Pantalla. El texto de estado sigue fuera del flujo (`previa-estado-oculto`, caja
    1×1) y 400 caracteres no cambiaron el canvas. `node --check` y `node prueba.js` en verde en el
    submódulo.
- **Editor auditado:** `e121270` en el submódulo (cherry-pick de `e600a66`), puntero `6243d9d`;
  F1, C1-C4 y F2 cerrados. `node --check` sin salida y `prueba.js` en `todo en orden`.
- **Clic del contador** (`7b9278c`/`799dafa`): el repintado de `vistas.js` reenvía el mismo
  `ultimoModelo` y ya no borra `pulsos`; verificado de 1 a 5 y el sexto tras cambiar de vista;
  crítico `ses_df953db48ffepwm7yI8MImMgv2` en **APROBAR**. Deuda no bloqueante: doble repintado al
  cambiar de vista.
- **F4** (`99229a4`/`73a59bd`): crítico `ses_df9532943ffeEMhPX85PddyVY3` en **APROBAR** con cuatro
  mutaciones detectadas.
- **Bloqueos y TOCTOU** (`b930078`/`5efc33a`, corrección `ad86af0`/`d86d5e8`): re-auditor
  `ses_df94eb6a7ffeafL8IIklwgqMdX` en **APROBAR** sin condiciones; la carrera real de dos pestañas
  queda para gate.
- **F5** (`d22fe3a`/`36a69c0`): el tecleo durante la carga se rechaza con aviso; crítico
  `ses_df93f2da4ffePx3uHgzYLU2qEM` en **APROBAR** con comprobación visual en gate.
- **OPFS mtime** (`9751433`/`4dd322a`): `statDe` usa el `lastModified` real; crítico
  `ses_df9352c53ffelAkiO5TriVgbyR` en **APROBAR**; deuda preexistente: `writeSync` no actualiza
  `mtimeMs`.
- **D7** (`385f1ed`/`db16376`): seis comprobaciones de navegación en pestaña; crítico
  `ses_df947ea53ffedqhgTYwg0HERax` en **APROBAR** estático; la ejecución real queda para gate.
- **I4** (`67e5f21` y `2dd06f7`): crítico `ses_df9491794ffePi1NK1rABUt0cZ` en **APROBAR** tras
  cerrar los dos `INCIERTO` (cuerpo que crece y caché corrupta); evidencia en §214 y §215.
- **Try/catch y excepciones propias** (`3bbd313`): crítico
  `ses_df93095c5ffeyRt8kl6K1MhO3T` en **APROBAR**. Sus dos `CONFIRMADO` quedaron falsificados por
  ejecución: `MiExcepcion("esp")` → `S:esp` sin avisos, `FalloApp("llano")` → `L:llano`,
  `toString()` → `com.hola.contador.FalloApp: ts`, sin captura
  `Error="…FalloApp: boomcustom"` y `MiAppException` sin regresión. `esThrowableDe` usa
  `m.tipoAcepta("Ljava/lang/Throwable;", descriptor)` y sube por superclases del dex y
  `padresDeExcepcion`; el sufijo es solo respaldo. 11 sondas `ok … 0.123s`, 4 tests del builder
  `ok … 0.070s` y suite completa `ok … 53.576s`.
- **I5 aprobado como medida, no cerrada** (`a1367a1`): el crítico
  `ses_df93c3f9fffefvTzhHzPYyU5lF` cerró los dos `CONFIRMADO` (huella sin fecha y CRC-32 del
  directorio central), con 8 tests en verde. No alcanza los 100 ms, así que no se cierra; la
  medición en pestaña sigue pendiente. La única reparación fue el comentario `SHA-1 → CRC-32`, en
  `c391518`.
- **Apps de ejemplo cerradas** (`6cf7ae2`): crítico `ses_df93069edffes9uiHHQLGh3e2c` en
  **APROBAR**; 5 builds y 5 instalaciones reproducidas en Droidloom, sin `FATAL` y sin binarios
  versionados. «Emulador fiel: no medido» queda como frontera, y los síntomas del compilador están
  marcados como hipótesis desde `bb8be96`.
- **Gate final VERDE** sobre `web` @ `89fc27f` (studio `0fce105`), ejecutado por
  `ses_df9a20cc2ffeLm5MRdXzqCsIwR` con `PREFIX=~/.cache/apkc-web-gate-final3`, puertos
  9081/9581/9082/9083/9582 y `ANDROID_HOME=/home/leonisauro/opt/android-sdk`. `LD_LIBRARY_PATH`
  apuntó a una copia privada de `libsimdutf.so.36`: el chromium 156 del escritorio está enlazado
  contra esa soname; es un parche de entorno ajeno al repo. Salida literal:
  `crear: todo en orden (36 comprobaciones)`, `pestaña: 223 comprobaciones, apk firmada dentro de
  chromium de 10285 bytes y verificada`, `e2e: todo en orden`; `rc=0`. `studio-pestana.log` da
  `223 comprobaciones bien, 0 mal`, cero `FALLA`, sin `sin chromium` ni `sin APKC_WASM`; el único
  aviso no bloqueante es `kotlin: sin kotlin-stdlib por ningun sitio, se omite`. Tres
  comprobaciones que antes fallaban ahora están verdes: el punto del clic cae sobre
  `previa-lateral-canvas`, un clic real sobre `id/pulsar` deja «Pulsos: 1» y `vista-previa.js` se
  sirve sin red desde la caché del service worker. Oráculos externos ejecutados: `apksigner`,
  `zipalign`, `aapt2`, `d8`, `bundletool`, `jarsigner` y `zipcheck.py`. `git status -sb` limpio
  (`ahead 66`, submódulo limpio).
- **Pendiente de gate:** la comprobación visual de F5, la ejecución real de D7, la carrera de dos
  pestañas y la medición de I5 en la pestaña. El gate verde registrado es de `89fc27f`, un árbol
  anterior a estos cierres; el definitivo sigue pendiente.
- **Activas:** ninguna; I5 queda medida y no cerrada. El resto de frentes está cerrado.
- Los worktrees de E2, E3 y E4 se crearon antes de `def0b85`; sus cambios ya están integrados.
