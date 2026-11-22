# Informe de seguridad de apkc-web

Auditoría del árbol de este worktree (commit `801bfb7` más los cambios de esta rama). Se revisaron
los sumideros HTML y de URL de `studio/`, la política de seguridad, el service worker, los binarios
que se publican y las dependencias del CI. Cada hallazgo lleva el comando que lo reproduce; no se
han incluido claves, contraseñas ni secretos.

## Medio

### M1. Las contraseñas de los keystores se escriben en la consola visible de la página

`correr()` muestra en la consola del editor la línea de comandos completa, y los comandos de firma,
generación y rotación meten la contraseña en `argv`. La consola se guarda en un buffer de 300 000
caracteres (`studio/consola.js:97-102`) y se pinta en el DOM (`studio/consola.js:105-109`), así que
la contraseña queda a la vista de quien mire la pestaña, de una captura de pantalla o de un informe
pegado desde ese texto. No sale del origen, pero no debería mostrarse.

Evidencia (ejecutado):

```
$ grep -n 'CONSOLA.escribir("\$ apkc\|--storepass\|--old-storepass' studio/pestana.js
1843:  CONSOLA.escribir("$ apkc " + argv.join(" ") + "\n");
1913:  const argv = ["sign", "--keystore", k.ruta, "--storepass", k.pass,
1926:      "--old-storepass", passV, "--old-keyalias", aliasV, "--rotation-min-sdk", rotMin);
2157:      "--storepass", pass, "--keyalias", alias,
2226:  const r = await correr(["keystore", "--keystore", cfg.ruta, "--storepass", pass]);
2299:  const r = await correr(["rotate", "--old-keystore", RUTA_VIEJO, "--old-storepass", pass,
2528:    "--storepass", DEBUG_KEY.pass, "--keyalias", DEBUG_KEY.alias]);
2565:    "--keystore", k.ruta, "--storepass", k.pass, "--keyalias", k.alias,
```

Mitigación pendiente (fuera del alcance de esta rama: `studio/` es submódulo): ocultar los
valores de las opciones `--*pass*` antes de `CONSOLA.escribir`, por ejemplo sustituyéndolos por
`***`. El campo se limpia al terminar (`studio/pestana.js:1930-1935`), pero el buffer de consola no.

### M2. El job WebKit de Linux era un falso verde

El job `webkit` instalaba Playwright con `npx -y playwright@latest install`, que deja el paquete en
la caché de npm, no en `node_modules`. Como `studio/prueba-webkit.js:15` hace `import("playwright")`,
la importación fallaba y el guion imprimía «sin playwright: se omite la prueba» y salía con 0: el
job podía estar verde sin haber abierto la página.

Evidencia (ejecutado). Antes del arreglo, `.github/workflows/gate.yml:138` era
`- run: npx -y playwright@latest install webkit --with-deps`; ahora dice:

```
$ grep -n 'playwright' .github/workflows/gate.yml studio/prueba-webkit.js
.github/workflows/gate.yml:139:      # `import("playwright")` de prueba-webkit.js falla y el trabajo se omite solo (falso verde).
.github/workflows/gate.yml:140:      - run: npm install --no-save --no-package-lock playwright@latest
.github/workflows/gate.yml:141:      - run: npx playwright install webkit --with-deps
.github/workflows/gate.yml:171:          npm install --no-save --no-package-lock playwright@latest
.github/workflows/gate.yml:172:          npx playwright install webkit
studio/prueba-webkit.js:15:  ({ webkit: chromium } = await import("playwright"));
studio/prueba-webkit.js:17:  console.log("sin playwright: se omite la prueba de WebKit (solo corre en CI con la dependencia instalada)");
```

Corregido en esta rama: `.github/workflows/gate.yml` instala ahora `playwright` en `node_modules`
antes de `npx playwright install`. El job nuevo `safari` hace lo mismo y además imprime la identidad
del ejecutable (`tools/safari-info.mjs`) antes de dar nada por bueno; sigue siendo informativo
(`continue-on-error`) porque Playwright ejecuta su WebKit, no la app Safari.

## Bajo

### B1. La CSP del sitio publicado solo llega por el service worker

El paquete publicado depende de que `sw.js` inyecte la cabecera `Content-Security-Policy` en cada
respuesta. `studio/_headers` fija COOP/COEP y otras cabeceras, pero no CSP; en hosts que ignoran
`_headers` (Surge) la primera navegación antes de que el SW tome el control no tiene CSP. El SW y
la página se recarga sola, así que la ventana es corta, pero existe.

Evidencia (ejecutado):

```
$ grep -n 'Content-Security-Policy\|trusted-types' studio/_headers studio/sw.js tools/sitio.sh sitio/sw.js
studio/sw.js:18:  "Content-Security-Policy": "__APKC_CSP__",
tools/sitio.sh:91:       "; require-trusted-types-for 'script'; " +
sitio/sw.js:18:  "Content-Security-Policy": "default-src 'self'; script-src 'self' 'wasm-unsafe-eval' 'sha256-ZZPnjUVqVUbfHRJ2rVXwjkO53THGGr0fmaOCBvjPID4=' 'sha256-pismi5g99sypim4Zqr93slUCOaxUVKbhmry9Ger3Xo4='; require-trusted-types-for 'script'; worker-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'",
```

Mitigación pendiente: añadir la CSP también a `studio/_headers` para los hosts que sí la respetan,
y aceptar que en Surge no hay cabeceras de servidor.

### B2. La directiva `trusted-types` no fija el nombre de la política

La CSP generada lleva `require-trusted-types-for 'script'` pero no `trusted-types apkc-studio`. El
nombre de política no queda restringido: cualquier script que se ejecutara en la página podría crear
la suya. Hoy `script-src` solo permite `'self'` y tres hashes, así que no hay una vía de entrada,
pero fijar el nombre cuesta una directiva y cierra la puerta.

Evidencia: la misma salida de B1 (`sitio/sw.js:18`) no contiene `trusted-types apkc-studio`. La
política se crea con ese nombre en `studio/tt.js:5-10`.

### B3. Suministro: `wasm_exec.js` se copia del GOROOT de quien compila

El sitio publicado incluye un fichero JavaScript que no está versionado ni fijado por hash: se copia
del GOROOT de la máquina que empaqueta. Lo mismo pasa en el CI. Son oráculos de CI (no viajan en la
página), pero tampoco llevan checksum: `apkc-harness` se clona por HEAD y `bundletool` se descarga
por versión sin verificar la suma.

Evidencia (ejecutado):

```
$ grep -n 'wasm_exec.js\|apkc-harness\|bundletool' tools/sitio.sh .github/workflows/gate.yml
tools/sitio.sh:3:# apkc.wasm y wasm_exec.js. No mete el android.jar, keystores ni OPFS: eso vive en el navegador.
tools/sitio.sh:27:# solo lo versionado del submódulo: nada de wasm_exec.js local, apkc.wasm ni perfiles de prueba
tools/sitio.sh:46:cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$DESTINO/wasm_exec.js"
tools/sitio.sh:110:[ -f "$DESTINO/wasm_exec.js" ] || { printf 'falta wasm_exec.js\n' >&2; exit 1; }
.github/workflows/gate.yml:79:          cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" studio/wasm_exec.js
.github/workflows/gate.yml:87:      - name: oráculo zipcheck (apkc-harness)
.github/workflows/gate.yml:89:          git clone --depth 1 https://github.com/Leonisaurov/apkc-harness.git "$RUNNER_TEMP/apkc-harness"
.github/workflows/gate.yml:90:          echo "APKC_HARNESS=$RUNNER_TEMP/apkc-harness" >> "$GITHUB_ENV"
.github/workflows/gate.yml:91:      - name: oráculo bundletool
.github/workflows/gate.yml:93:          mkdir -p "$RUNNER_TEMP/bundletool"
.github/workflows/gate.yml:94:          curl -fsSL -o "$RUNNER_TEMP/bundletool/bundletool-all-1.18.3.jar" \
```

Mitigación pendiente: fijar commit de `apkc-harness`, publicar la suma SHA-256 de `bundletool` y
registrar la versión de Go que produjo `wasm_exec.js` en cada artefacto.

### B4. El zip elegido por el usuario se descomprime sin cota de tamaño

`extraerJarDeZip()` toma el tamaño declarado del directorio central, descomprime la entrada con
`DecompressionStream` y solo después compara longitudes. Un fichero elegido a mano con una entrada
`android.jar` muy comprimida puede hacer que el navegador intente materializar mucho más de lo que
ocupó en disco. Es un problema local del usuario que eligió el fichero, no una entrada de red.

Evidencia (ejecutado):

```
$ grep -n 'DecompressionStream("deflate-raw")' studio/pestana.js
2367:        const flujo = new Blob([comprimido]).stream().pipeThrough(new DecompressionStream("deflate-raw"));
```

## Informativo (revisado, sin hallazgo)

- **Sumideros HTML.** Solo dos escrituras con `innerHTML`: `studio/consola.js:108` y
  `studio/pestana.js:222`. Ambas pasan por `htmlConfiable()` (`studio/tt.js:32-34`), que aplica la
  política de Trusted Types, y el contenido se escapa antes con `textoHtml()` (`studio/consola.js:5-7`,
  `studio/editor.js:32`). La política rechaza `<script` y atributos de evento (`studio/tt.js:13`).
  No aparece `eval`, `new Function`, `document.write` ni `insertAdjacentHTML` en `studio/*.js`.
  Comando: `grep -rn 'innerHTML\|outerHTML\|insertAdjacentHTML\|document\.write\|eval(\|new Function' studio --include='*.js'`.
- **URL y mensajes.** `fetch()` apunta a rutas literales del mismo origen (`/apkc.wasm.gz`,
  `/apkc.wasm`, `/android-jar`, `/android-jar-info`) o a la petición interceptada por el SW
  (`studio/sw.js:78,97`). Los enlaces de descarga son `blob:` creados por la propia página y pasan
  por `enlaceConfiable()`. El canal entre pestañas es `BroadcastChannel`, que es del mismo origen
  (`studio/pestana.js:563-564`). No hay `window.open`, `location.assign` ni `importScripts`.
  Comando: `grep -rn 'window\.open\|location\.assign\|importScripts\|fetch(\|createObjectURL' studio --include='*.js'`.
- **Service worker.** Solo intercepta `GET` del mismo origen (`studio/sw.js:67-70`), deja pasar sin
  caché las rutas de jar/keystore (`studio/sw.js:43-46`) y sirve `index.html` como respaldo de la
  navegación (`studio/sw.js:87-89`). No reescribe respuestas de terceros. Comando: `sed -n '43,70p' studio/sw.js`.
- **Dependencias de la página.** `studio/package.json` no declara ninguna dependencia de ejecución;
  no hay CDN, telemetría ni fuentes externas en `sitio/index.html` salvo dos enlaces `<a>` que el
  usuario tendría que pulsar (`sitio/index.html:518,521`). Comando: `grep -n 'https\?://' sitio/index.html`.

## Límites de esta auditoría

No se ha ejecutado un escáner de dependencias ni una prueba de intrusión sobre el host publicado:
el análisis es de código y de las peticiones que la propia prueba de sitio registra. Los hallazgos
M1 y B1-B4 quedan abiertos porque este encargo no permite tocar `studio/`, `tools/sitio.sh` ni el
contenido del sitio; la corrección de M2 ya está aplicada en `.github/workflows/gate.yml`.
