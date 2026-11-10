# Uso de apkc-web

Guía concreta para construir, arrancar y usar este repo. Todos los comandos se ejecutan desde la
raíz del repo. Para la estructura del editor, ver [`studio/EDITOR.md`](studio/EDITOR.md); para las
mediciones, [`docs/evidencia.md`](docs/evidencia.md).

## 1. Entorno

Usa siempre el envoltorio del repo; fija `HOME`, `PREFIX`, `ANDROID_HOME` y `PATH`:

```sh
tools/entorno.sh doctor
```

Las tareas disponibles son `test`, `wasm`, `gate`, `banco`, `jar`, `doctor` y `env`. Para
cualquier comando suelto:

```sh
tools/entorno.sh -- <comando...>
```

## 2. Construir y arrancar

Construye el binario wasm:

```sh
tools/entorno.sh -- sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
```

Arranca el servidor de la página (deja el puerto 8787):

```sh
tools/entorno.sh -- sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" node servidor.js'
```

O en segundo plano:

```sh
tools/entorno.sh -- sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" nohup node servidor.js >"$PREFIX/tmp/studio-server.log" 2>&1 &'
```

Comprueba que responde:

```sh
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8787/index.html
```

Debe imprimir `200`.

## 3. Abrir la página

En Brave Nightly, desde esta sesión:

```sh
termux-open-url 'http://127.0.0.1:8787/'
```

Si no abre, usa el intento explícito:

```sh
am start --user 0 \
  -n com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity \
  -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'
```

No uses `sudo am`, `sudo cmd`, `sudo -u shell cmd`, `input` ni `pm` para abrir la página: no
funcionan en esta sesión. El detalle está en [`apps.md`](apps.md).

## 4. Crear y editar

1. En **Inicio**, escribe el nombre, el package (`com.mi.app`) y, si quieres, un icono PNG.
2. Pulsa **Crear proyecto y editar**. El proyecto se escribe en OPFS y se abre
   `AndroidManifest.xml` en el **Editor**.
3. En el **Explorador**, pulsa un fichero para abrirlo. El árbol empieza **cerrado**; los botones
   **−** y **+** de la cabecera lo contraen o lo despliegan entero. El botón de subir escribe
   archivos en la carpeta del fichero abierto; el menú de una carpeta permite **Subir archivos
   aquí** o **Subir carpeta aquí**, conservando subdirectorios. Para mover, arrastra una fila a
   otra carpeta (ratón o lápiz) o usa **Mover a…** en el menú de mantener pulsado.
4. Edita en el área de código:
   - Cada fichero abierto queda en una **pestaña**; cambia con un clic y ciérrala con `×`. Las
     pestañas, la activa y las carpetas abiertas se recuerdan al recargar.
   - `Ctrl+Z` deshace y `Ctrl+Shift+Z`/`Ctrl+Y` rehace. El panel **Historial** (en la consola)
     muestra el árbol con ramas; un clic restaura una versión sin borrar las demás.
   - `Ctrl+S` o **Guardar** salva el fichero.
   - En **Tema**, en la barra de estado, cambias la interfaz (Catppuccin, Rosé Pine, Tokyo Night,
     Gruvbox), la paleta ANSI de la terminal (Catppuccin, Rosé Pine, Gruvbox, Solarized, Dracula)
     y **Solo iconos en la barra del editor** para dejar Guardar, Archivo y Compilar como iconos.
   - Puedes abrir otra pestaña del navegador con el mismo proyecto: comparten OPFS y se avisan de
     los guardados. Si la otra ventana guardó algo que tienes sucio, se fusionan los cambios que
     no se pisan; si tocan la misma línea aparece una banda de conflicto para elegir **Mantener el
     mío**, **Usar el remoto** o **Marcar conflicto**.
   - El service worker cachea la página; una vez cargada, puedes recargarla sin red (el primer
     acceso y la descarga del `android.jar` sí necesitan conexión).
   - **Archivo** agrupa Nuevo, Subir archivos, Descargar, Renombrar y Borrar.
   - `Tab` / `Shift+Tab` sangran / desangran.
   - `Enter` conserva la sangría y añade un nivel tras `{`.
   - El punto amarillo de la pestaña indica cambios sin guardar.
5. El panel inferior está cerrado por defecto. Ábrelo con **Compilar** o ``Ctrl + ` ``. Sus
   pestañas son **Consola**, **Historial**, **Compilar** y **Firmar otra apk**.

## 5. Compilar, firmar y verificar

En la pestaña **Compilar** del panel:

0. En **Opciones avanzadas** elige la **API de compilación** (34 por defecto; 27-35).
1. Elige el `android.jar` del SDK (API 34) y pulsa **Guardar el android.jar elegido**. Si ya
   estaba cargado, la sección aparece plegada, gris y con la etiqueta **listo**; el botón pasa a
   **Cambiar android.jar**.
2. Elige el keystore (`.p12` o `.jks`), escribe la contraseña y pulsa **Ver certificado**. Si no
   tienes uno, pulsa **Crear clave de release**: se genera en la pestaña, se descarga y el diálogo
   exige confirmar que guardaste una copia fuera del navegador. La contraseña no se almacena.
3. Elige el alias.
4. Pulsa **Compilar debug** (clave de depuración generada en la pestaña) o **Compilar release**
   (el keystore de arriba).
5. Pulsa **Verificar el apk** para leer el veredicto de `apkc verify`.
6. Descarga el apk (y el `.idsig` si se generó) con los enlaces del panel.

En **Firmar otra apk** puedes firmar una apk o un `.aab`, y crear un linaje de rotación
v3.1. Todo ocurre en la pestaña: el fuente, el keystore y el `android.jar` no salen de OPFS.

## 6. Pruebas y gate

```sh
tools/entorno.sh test     # suite nativa de apkc/
tools/entorno.sh wasm     # suite en el hospedador con forma de navegador
tools/entorno.sh gate     # binarios + studio/e2e.sh: la página de punta a punta
tools/entorno.sh banco    # banco de ejecución en el aparato
```

El gate que vale para la página es `tools/entorno.sh gate`. Pruebas sueltas:

```sh
tools/entorno.sh -- sh -c 'cd studio && node prueba.js'
tools/entorno.sh -- sh -c 'cd studio && node prueba-navegador.js'
tools/entorno.sh -- sh -c 'cd studio && APKC_WASM="$PREFIX/tmp/apkc-wasm.wasm" node prueba-crear.js'
```

## 7. Errores típicos

- **El servidor no arranca**: falta `APKC_WASM` o el binario no existe. Repite el paso 2.
- **El puerto 8787 está ocupado**: usa otro, por ejemplo `node servidor.js 8790`, y abre
  `http://127.0.0.1:8790/`.
- **No hay OPFS / el worker no arranca / la página está muda**: los ES modules no cargan por
  `file://`; abre la página por `http://` con `servidor.js`.
- **Compilar pide el `android.jar`**: elígelo en la pestaña **Compilar**; se guarda en OPFS y
  sobrevive a recargar.
- **`am` falla al abrir Brave**: usa `termux-open-url` o el `am start` del paso 3. No uses
  `sudo`/`cmd`/`input`/`pm`.
- **Un guion no encuentra el SDK o el repo**: ejecútalo con `tools/entorno.sh --`; el `HOME` del
  shell no es el de Termux.
- **Rutas temporales**: usa `$PREFIX/tmp`, nunca `/tmp`.

## 8. Documentos relacionados

- [`studio/README.md`](studio/README.md): qué hace la pestaña y cómo se prueba.
- [`studio/EDITOR.md`](studio/EDITOR.md): estructura, ids, atajos y límites del editor.
- [`studio/AGENTS.md`](studio/AGENTS.md): comandos, contratos y trampas al tocar la página.
- [`apps.md`](apps.md): ejecutar apps Android desde Termux/chroot y qué no funciona.
- [`docs/evidencia.md`](docs/evidencia.md): cada medición con su comando.
