#!/bin/sh
# Empaqueta el sitio estático que se publica: los ficheros versionados de studio/ (submódulo),
# apkc.wasm y wasm_exec.js. No mete el android.jar, keystores ni OPFS: eso vive en el navegador.
#
# uso: APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm sh tools/sitio.sh [destino]
#      tools/entorno.sh sitio [destino]
# por defecto el destino es ./sitio, listo para `cd sitio && surge .`
# sale 0 y deja <destino>/ y <destino>.zip
set -eu

AQUI=$(cd "$(dirname "$0")/.." && pwd)
DESTINO=${1:-$AQUI/sitio}
WASM=${APKC_WASM:?falta APKC_WASM con el binario wasm}
[ -f "$WASM" ] || { printf 'no existe el wasm: %s\n' "$WASM" >&2; exit 1; }
[ -d "$AQUI/studio" ] || { printf 'falta studio/ (submódulo sin inicializar)\n' >&2; exit 1; }

# Surge escribe el dominio en CNAME al publicar. Conservarlo entre reconstrucciones evita que
# cada `tools/entorno.sh sitio` obligue a volver a elegirlo. También se puede fijar de antemano
# con .surge-domain en la raíz del repo (ignorado por git).
CNAME=
if [ -f "$DESTINO/CNAME" ]; then CNAME=$(cat "$DESTINO/CNAME"); fi
if [ -z "$CNAME" ] && [ -f "$AQUI/.surge-domain" ]; then CNAME=$(cat "$AQUI/.surge-domain"); fi

rm -rf "$DESTINO" "${DESTINO}.zip"
mkdir -p "$DESTINO"

# solo lo versionado del submódulo: nada de wasm_exec.js local, apkc.wasm ni perfiles de prueba
if ! git -C "$AQUI/studio" archive --format=tar HEAD | tar -x -C "$DESTINO"; then
  printf 'no se pudo extraer studio/ desde su git\n' >&2
  exit 1
fi

# El paquete publicado es solo la aplicación. Las pruebas, el servidor de desarrollo y la
# documentación del submódulo no viajan; LICENSE y NOTICE sí, porque son obligatorias.
for dev in .gitignore AGENTS.md EDITOR.md README.md e2e.sh package.json servidor.js \
           prueba.js prueba-crear.js prueba-navegador.js prueba-pestana.js prueba-sitio.js \
           servidor-estatico.js; do
  rm -f "$DESTINO/$dev"
done
rm -rf "$DESTINO/.github"

cp "$WASM" "$DESTINO/apkc.wasm"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$DESTINO/wasm_exec.js"
[ -n "$CNAME" ] && printf '%s\n' "$CNAME" > "$DESTINO/CNAME"

# Los hosts sin cabeceras (Surge) necesitan el service worker: se registra con una versión en la
# URL para que un cambio de binario no se quede servido por la caché del navegador. El hash se
# inyecta en el SW, index.html y offline.js; en el árbol de desarrollo el marcador es literal y
# válido como versión.
VERSION=$(cat "$DESTINO/apkc.wasm" "$DESTINO/sw.js" "$DESTINO/index.html" | sha256sum | cut -c1-12)
python3 - "$DESTINO" "$VERSION" <<'PY'
from pathlib import Path
import sys
raiz, version = Path(sys.argv[1]), sys.argv[2]
for nombre in ("index.html", "sw.js", "offline.js"):
    fichero = raiz / nombre
    if fichero.exists():
        fichero.write_text(fichero.read_text().replace("__APKC_VERSION__", version))
PY

# las cabeceras son la mitad del producto: sin COOP/COEP no hay OPFS ni worker
grep -q 'Cross-Origin-Opener-Policy: same-origin' "$DESTINO/_headers" ||
  { printf '_headers no declara COOP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Embedder-Policy: require-corp' "$DESTINO/_headers" ||
  { printf '_headers no declara COEP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Opener-Policy' "$DESTINO/sw.js" ||
  { printf 'el service worker no pone COOP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Embedder-Policy' "$DESTINO/sw.js" ||
  { printf 'el service worker no pone COEP\n' >&2; exit 1; }
[ -f "$DESTINO/index.html" ] || { printf 'el sitio no tiene index.html\n' >&2; exit 1; }
[ -f "$DESTINO/wasm_exec.js" ] || { printf 'falta wasm_exec.js\n' >&2; exit 1; }
if grep -R -q '__APKC_VERSION__' "$DESTINO" 2>/dev/null; then
  printf 'quedó un marcador de versión sin sustituir\n' >&2
  exit 1
fi

python3 - "$DESTINO" "${DESTINO}.zip" <<'PY'
import os, sys, zipfile
raiz, salida = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(salida, "w", zipfile.ZIP_DEFLATED) as z:
    for base, _, ficheros in os.walk(raiz):
        for f in sorted(ficheros):
            ruta = os.path.join(base, f)
            z.write(ruta, os.path.relpath(ruta, raiz))
PY

printf 'sitio: %s (%s ficheros, version %s)\n' "$DESTINO" "$(find "$DESTINO" -type f | wc -l | tr -d ' ')" "$VERSION"
printf 'zip:   %s (%s bytes)\n' "${DESTINO}.zip" "$(wc -c <"${DESTINO}.zip" | tr -d ' ')"
