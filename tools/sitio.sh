#!/data/data/com.termux/files/usr/bin/sh
# Empaqueta el sitio estático que se publica: los ficheros versionados de studio/ (submódulo),
# apkc.wasm y wasm_exec.js. No mete el android.jar, keystores ni OPFS: eso vive en el navegador.
#
# uso: APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm sh tools/sitio.sh [destino]
# sale 0 y deja <destino>/ y <destino>.zip
set -eu

AQUI=$(cd "$(dirname "$0")/.." && pwd)
DESTINO=${1:-${PREFIX:-/data/data/com.termux/files/usr}/tmp/apkc-sitio}
WASM=${APKC_WASM:?falta APKC_WASM con el binario wasm}
[ -f "$WASM" ] || { printf 'no existe el wasm: %s\n' "$WASM" >&2; exit 1; }
[ -d "$AQUI/studio" ] || { printf 'falta studio/ (submódulo sin inicializar)\n' >&2; exit 1; }

rm -rf "$DESTINO" "${DESTINO}.zip"
mkdir -p "$DESTINO"

# solo lo versionado del submódulo: nada de wasm_exec.js local, apkc.wasm ni perfiles de prueba
if ! git -C "$AQUI/studio" archive --format=tar HEAD | tar -x -C "$DESTINO"; then
  printf 'no se pudo extraer studio/ desde su git\n' >&2
  exit 1
fi
cp "$WASM" "$DESTINO/apkc.wasm"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$DESTINO/wasm_exec.js"

# las cabeceras son la mitad del producto: sin COOP/COEP no hay OPFS ni worker
grep -q 'Cross-Origin-Opener-Policy: same-origin' "$DESTINO/_headers" ||
  { printf '_headers no declara COOP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Embedder-Policy: require-corp' "$DESTINO/_headers" ||
  { printf '_headers no declara COEP\n' >&2; exit 1; }
[ -f "$DESTINO/index.html" ] || { printf 'el sitio no tiene index.html\n' >&2; exit 1; }
[ -f "$DESTINO/wasm_exec.js" ] || { printf 'falta wasm_exec.js\n' >&2; exit 1; }

python3 - "$DESTINO" "${DESTINO}.zip" <<'PY'
import os, sys, zipfile
raiz, salida = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(salida, "w", zipfile.ZIP_DEFLATED) as z:
    for base, _, ficheros in os.walk(raiz):
        for f in sorted(ficheros):
            ruta = os.path.join(base, f)
            z.write(ruta, os.path.relpath(ruta, raiz))
PY

printf 'sitio: %s (%s ficheros)\n' "$DESTINO" "$(find "$DESTINO" -type f | wc -l | tr -d ' ')"
printf 'zip:   %s (%s bytes)\n' "${DESTINO}.zip" "$(wc -c <"${DESTINO}.zip" | tr -d ' ')"
