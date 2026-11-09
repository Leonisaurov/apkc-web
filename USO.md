# Uso de apkc-web

Guía concreta para construir, arrancar y usar este repo. Todos los comandos se ejecutan desde la
raíz del repo. Para la estructura del editor, ver [`studio/EDITOR.md`](studio/EDITOR.md); para las
mediciones, [`docs/evidencia.md`](docs/evidencia.md).

## 1. Entorno

Usa siempre el envoltorio del repo; fija `HOME`, `PREFIX`, `ANDROID_HOME` y `PATH`:

```sh
tools/entorno.sh doctor
```

Las tareas disponibles son `test`, `wasm`, `gate`, `banco` y `doctor`. Para cualquier comando
suelto:

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
3. En el **Explorador**, pulsa un fichero para abrirlo. Las carpetas se pliegan.
4. Edita en el área de código:
   - `Ctrl+S` guarda.
   - `Tab` / `Shift+Tab` sangran / desangran.
   - `Enter` conserva la sangría y añade un nivel tras `{`.
   - El punto amarillo de la pestaña indica cambios sin guardar.
5. El panel inferior está cerrado por defecto. Ábrelo con **Panel** o ``Ctrl + ` ``. Sus pestañas son
   **Salida**, **Compilar** y **Firmar**.

## 5. Compilar, firmar y verificar

En **Panel → Compilar**:

1. Elige el `android.jar` del SDK (API 34) y pulsa **Guardar el android.jar elegido**.
2. Elige el keystore (`.p12` o `.jks`), escribe la contraseña y pulsa **Ver certificado**.
3. Elige el alias.
4. Pulsa **Compilar (apkc build)**.
5. Pulsa **Verificar el apk** para leer el veredicto de `apkc verify`.
6. Descarga el apk (y el `.idsig` si se generó) con los enlaces del panel.

En **Panel → Firmar** puedes firmar otra apk o un `.aab`, y crear un linaje de rotación v3.1. Todo
ocurre en la pestaña: el fuente, el keystore y el `android.jar` no salen de OPFS.

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
- **No hay OPFS / el worker no arranca**: abre la página por `http://` con `servidor.js`, no por
  `file://`.
- **Compilar pide el `android.jar`**: elígelo en **Panel → Compilar**; se guarda en OPFS y
  sobrevive a recargar.
- **`am` falla al abrir Brave**: usa `termux-open-url` o el `am start` del paso 3. No uses
  `sudo`/`cmd`/`input`/`pm`.
- **Un guion no encuentra el SDK o el repo**: ejecútalo con `tools/entorno.sh --`; el `HOME` del
  shell no es el de Termux.
- **Rutas temporales**: usa `$PREFIX/tmp`, nunca `/tmp`.

## 8. Documentos relacionados

- [`studio/README.md`](studio/README.md): qué hace la pestaña y cómo se prueba.
- [`studio/EDITOR.md`](studio/EDITOR.md): estructura, ids, atajos y límites del editor.
- [`apps.md`](apps.md): ejecutar apps Android desde Termux/chroot y qué no funciona.
- [`docs/evidencia.md`](docs/evidencia.md): cada medición con su comando.
