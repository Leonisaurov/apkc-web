# AGENTS.md — apkc-studio

Página estática sin dependencias ni bundler que genera proyectos [apkc](../apkc) y ejecuta
`apkc.wasm` dentro de la pestaña (OPFS + worker). Se publica copiando `studio/`; no hay
`npm install`. El contrato de la herramienta y las mediciones están en el `AGENTS.md` de la raíz.

## Comandos que importan

```sh
# suite rápida: node, sin navegador. Compara las plantillas con el repo apkc.
node prueba.js                          # npm test

# navegador real sin OPFS: vistas, formulario, diagnóstico y localStorage
node prueba-navegador.js                # npm run test:navegador

# con OPFS y wasm: construye el binario y conduce chromium por CDP
sh -c 'cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" node prueba-crear.js
APKC_WASM=… APK_EJEMPLO=… KS_EJEMPLO=… node prueba-pestana.js

# el gate de verdad: descomprime, compila, firma y verifica con oráculos externos
APKC="$PREFIX/tmp/apkc-gate" APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" sh e2e.sh
```

`npm start` es `node servidor.js` y exige `APKC_WASM` y un `wasm_exec.js` en
`$(go env GOROOT)/lib/wasm`; sin uno de los dos sale con código 2. `e2e.sh` llama a `tcr` para
los `apkc build` nativos: tiene que estar en el PATH. No hay lint, typecheck ni formatter; para
una comprobación rápida de sintaxis, `node --check <fichero>`.
`prueba-pestana.js` exige `APKC_WASM`, `APK_EJEMPLO` y `KS_EJEMPLO`; con `KS_RELEASE` y
`AAB_EJEMPLO` añade release, v4, `.aab` y rotación (es lo que le pasa `e2e.sh`). `prueba.js`
compara las plantillas contra `APKC_REPO` (`~/Develop/Go/apkc` por defecto): si el repo no está,
omite la comparación y **no falla**, así que no confundas «reglas: 24» con «plantillas iguales».

Desde la raíz del repo, `tools/entorno.sh gate` construye los binarios y lanza `e2e.sh` con
`HOME`/`PREFIX`/`ANDROID_HOME` correctos (el shell puede arrancar con otro `HOME`). No declares
la página terminada con `prueba.js` ni con `prueba-navegador.js`: solo `e2e.sh` descomprime el
zip y compila con `apksigner`, `zipalign` y `zipcheck.py`. `prueba-pestana.js` cuenta las
peticiones y falla si la página pide algo fuera de su origen.

**`e2e.sh` puede decir «todo en orden» con la pestaña omitida**: sin `chromium-browser`, sin
`APKC_WASM` o sin el apk de fixtures, esas secciones se saltan con un aviso, no fallan. Antes de
dar el editor por probado, comprueba en la salida `pestaña: N comprobaciones` y que no aparezca
`sin chromium` ni `sin APKC_WASM`.

## Mapa y orden de carga

| Fichero | Papel |
| --- | --- |
| `app.js` | **modelo puro**: plantillas, `buildProject`, `makeZip`, `RULES`, `analyze`, `informe`. Sin DOM; `module.exports` para node. |
| `pestana.js` | editor, árbol de OPFS, comandos y el puente `hospedador`; exporta las funciones puras para node. |
| `vistas.js` | cambio de vista, reflejo nombre/package y creación del proyecto desde el formulario. |
| `diagnostico.js` | panel de Inicio que pinta `analyze()`; solo DOM, sin reglas propias. |
| `hospedador.js` | worker de módulo: `fs`/`path`/`process` para Go y una cola de un comando por vez. |
| `fs-fragmento.js` | `fs` con forma de Node sobre OPFS (lo importa el worker, no la página). |
| `servidor.js` | servidor de desarrollo: estáticos + COOP/COEP + `/apkc.wasm` + `/wasm_exec.js` + `/android.jar`. |
| `_headers` | COOP/COEP para hosts estáticos que las respetan (Netlify, Cloudflare Pages). |

Los cuatro scripts de la página son **clásicos, no módulos**, y el orden de `index.html` es
contrato: `app.js` define los globales que usan `vistas.js` y `diagnostico.js`, y `pestana.js`
define `hospedador` antes de que `vistas.js` cree el proyecto. No pongas `"type": "module"` en
`package.json` ni conviertas `app.js`/`pestana.js` a `export`: los tests los cargan con `require`.

`hospedador.js` pide `/wasm_exec.js` y `/apkc.wasm` con ruta **absoluta al origen**. Publica
`studio/` como raíz del sitio, no bajo `/studio/`, o cambia esas dos rutas a la vez.

## Contratos que rompen pruebas

- **Los ids son contrato.** `prueba.js` comprueba que todo `getElementById` de `vistas.js`,
  `pestana.js` y `diagnostico.js` exista en `index.html`; `prueba-crear.js`, `prueba-pestana.js`
  y `e2e.sh` buscan ids concretos. Cambiar uno exige cambiar su prueba.
- **Las reglas de `app.js` no se inventan.** Cada `de: "apkc"` debe existir literalmente en
  `apkc/*.go`; `prueba.js` lo verifica. Una regla sin `lit:` o sin evidencia es un fallo.
- **Las contraseñas se limpian** de los campos después de firmar o compilar, también cuando
  falla; `prueba-pestana.js` lo comprueba.
- **Subir escribe en OPFS, no en un servidor.** El botón del explorador usa la carpeta del
  fichero abierto; el menú de una carpeta fija el destino. La subida de carpeta conserva
  `webkitRelativePath` y descarta los tramos `..`; `prueba-crear.js` conduce el input con
  `DataTransfer`.
- **El árbol empieza cerrado.** `ARBOL.abiertos` vacío significa todo plegado; no lo vuelvas a
  invertir a `plegados`. Mover es `hospedador.renombrar` a través de `moverRuta`, que remapea
  `ARBOL.abiertos` y la ruta del fichero abierto. El arrastre solo se activa con puntero fino;
  en táctil se usa **Mover a…**.
- **La barra del fichero es deliberada:** `#guardar-fichero` es la acción primaria y
  `#menu-editor` (un `<details>`) agrupa Nuevo, Subir, Descargar, Renombrar y Borrar. No vuelvas
  a sacar esos botones sueltos a la barra.
- **Pestañas con carga diferida.** `DOCUMENTOS` guarda `{ruta, texto, sucio, hist}`; una pestaña
  limpia inactiva tiene `texto = null` y se relee de OPFS al activarla. `PERSISTENCIA`
  (`apkc-studio/editor-v1`) va en **sessionStorage** (vista por ventana, no compartida) y guarda
  rutas y `ARBOL.abiertos`, nunca contenido. La activa se restaura al cargar, las demás al
  pulsarlas; `activacion` evita que una lectura lenta pise otra pestaña.
- **Undo tree persistente.** `historial.js` guarda deltas, checkpoints cada 24 niveles y poda
  (240 nodos / 4 MB). Se hidrata por documento al activarlo y se guarda en IndexedDB con clave
  `VENTANA|ruta`; `VENTANA` sale de sessionStorage. No persistas `cache` (se reconstruye). Las
  ramas antiguas se pueden podar, pero nunca la raíz ni los ancestros del nodo actual; si la
  cadena es demasiado larga, `rebase()` re-enraíza en un checkpoint.
- **Temas sin dependencias.** `temas.js` solo pone `data-tema`/`data-tema-consola` en `<html>` y
  `style.css` resuelve las variables. No metas colores nuevos a mano fuera de un bloque de tema;
  la paleta ANSI se cambia con `--ansi-0..15`.
- **API de compilación.** `#api-version` es el selector visible (34 por defecto) y sincroniza los
  ocultos `#sdk-version`/`#codename` que lee `apkc build`; las pruebas siguen usando los ocultos.
- **Nada fuera del origen.** Sin CDN, fuentes, telemetría ni runtimes de terceros. El favicon es
  un `data:` y el informe se descarga como `blob:`.
- **`apkc build` clásico no corre en wasm.** `js` no tiene `fork/exec`; el botón «Probar el build
  clásico» existe para enseñar el muro real (`aapt2 compile never started: pipe: not implemented
  on js`), no para arreglarlo. Lo que sí compila es `build --linker go --compiler go`.

## Trampas de OPFS y wasm ya pagadas

- `navigator.storage` **no existe dentro del worker**: la página baja el mango raíz y se lo pasa
  por `postMessage`. No lo muevas al worker.
- `createSyncAccessHandle()` es **asíncrona**; sus métodos sí son sincrónicos. Sin
  `crossOriginIsolated` no hay handle: hacen falta COOP/COEP y servir por `http(s)`, no `file://`.
- `wasm_exec.js` **no se versiona** (viene del GOROOT). `servidor.js` lo sirve desde
  `$(go env GOROOT)/lib/wasm/`; en un host estático hay que copiarlo junto a `index.html`.
- Un binario de Go en wasm **no se reutiliza**: `hospedador.js` guarda los bytes y crea una
  instancia por comando. Un comando por vez, en cola; no hay tuberías.
- `file://` solo sirve para Inicio y el diagnóstico. El Editor, OPFS y `apkc.wasm` exigen
  `servidor.js` o un host con COOP/COEP.

## Publicar

`_headers` cubre Netlify y Cloudflare Pages. GitHub Pages no permite fijar COOP/COEP, así que
allí el editor no arranca. El `android.jar` y el keystore no se versionan ni se empaquetan: la
página los pide al SDK local (mismo origen) o al selector de fichero, y el paquete oficial de
Google se descarga como navegación del usuario tras aceptar los términos.

## Documentos gemelos

`studio/README.md` (uso y pruebas), `studio/EDITOR.md` (estructura del editor e ids),
`docs/evidencia.md` (cada medición con su comando) y el `AGENTS.md` de la raíz (reglas del repo).
