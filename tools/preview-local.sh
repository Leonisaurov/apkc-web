#!/bin/sh
# vista previa local: construye apkc.wasm desde el árbol que se le pase y lo deja
# donde lo sirve studio/servidor.js. No publica nada y no toca sitio/.
# uso: tcr -c 2 -m 2G -r 256M -- sh tools/preview-local.sh [raiz-del-worktree]
set -eu

RAIZ=${1:-$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)}
DESTINO=${APKC_PREVIEW_WASM:-$HOME/.cache/apkc-web/preview/apkc.wasm}
DIRECTORIO=$(dirname -- "$DESTINO")
mkdir -p "$DIRECTORIO"

TEMPORAL=$DESTINO.tmp.$$
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -C "$RAIZ/apkc" -ldflags='-s -w' -o "$TEMPORAL" .
mv -f "$TEMPORAL" "$DESTINO"
sha256sum "$DESTINO"
