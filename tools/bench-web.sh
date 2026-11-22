#!/bin/sh
# Mide el sitio empaquetado con chromium y CDP: tiempo hasta crossOriginIsolated, compilación e
# instanciación del wasm, entrada del android.jar en OPFS y primer build. Solo imprime números.
#
#   uso: ANDROID_JAR=<jar> sh tools/bench-web.sh [sitio]
#   opciones: BENCH_CDP (puerto del debugger), BENCH_PUERTO (puerto del servidor),
#             CHROME_BIN (chromium), SITIO_DIR (paquete, también como primer argumento).
set -eu
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SITIO=${1:-${SITIO_DIR:-$AQUI/sitio}}
[ -f "$SITIO/index.html" ] || { echo "bench: no hay index.html en $SITIO" >&2; exit 2; }
[ -n "${ANDROID_JAR:-}" ] || { echo "bench: falta ANDROID_JAR" >&2; exit 2; }
CHROME=${CHROME_BIN:-chromium}
PUERTO=${BENCH_PUERTO:-8793}
CDP=${BENCH_CDP:-9351}
URL_BASE="http://127.0.0.1:$PUERTO/index.html"
PERFIL=$(mktemp -d "${TMPDIR:-/tmp}/apkc-bench-perfil.XXXXXX")

SERVIDOR=
NAVEGADOR=
limpiar() {
  [ -z "$NAVEGADOR" ] || kill "$NAVEGADOR" 2>/dev/null || true
  [ -z "$SERVIDOR" ] || kill "$SERVIDOR" 2>/dev/null || true
}
trap limpiar EXIT HUP INT TERM

SITIO_DIR="$SITIO" node "$AQUI/studio/servidor-estatico.js" "$PUERTO" >/dev/null 2>&1 &
SERVIDOR=$!
for i in $(seq 1 60); do
  if node -e 'fetch(process.argv[1]).then(r => process.exit(r.ok ? 0 : 1)).catch(() => process.exit(1))' "$URL_BASE" 2>/dev/null; then
    break
  fi
  sleep 0.2
done

"$CHROME" --headless=new --no-sandbox --disable-setuid-sandbox --disable-gpu \
  --disable-dev-shm-usage --remote-allow-origins=* --no-first-run --disable-extensions \
  --disable-sync --user-data-dir="$PERFIL" --remote-debugging-port="$CDP" about:blank \
  >/dev/null 2>&1 &
NAVEGADOR=$!

BENCH_CDP="$CDP" BENCH_URL="$URL_BASE" ANDROID_JAR="$ANDROID_JAR" \
  node "$AQUI/tools/bench-web.mjs"
