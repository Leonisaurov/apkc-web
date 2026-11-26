#!/bin/sh
# vista previa local: construye apkc.wasm desde el árbol que se le pase y lo deja
# donde lo sirve studio/servidor.js. No publica nada y no toca sitio/.
# uso: tcr -c 2 -m 2G -r 256M -- sh tools/preview-local.sh [raiz-del-worktree]
set -eu

RAIZ=${1:-$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)}
DESTINO=${APKC_PREVIEW_WASM:-$HOME/.cache/apkc-web/preview/apkc.wasm}
DIRECTORIO=$(dirname -- "$DESTINO")
mkdir -p "$DIRECTORIO"

# La toolchain fijada por §312: el vigilante no pasa por entorno.sh, así que hay que
# anteponerla aquí; si no, se compilaría con el go del sistema (1.27.4-X:nodwarf5).
GOROOT=${APKC_GOROOT:-$HOME/sdk/go1.24.0}
if [ -x "$GOROOT/bin/go" ]; then
  PATH=$GOROOT/bin:$PATH
  export GOROOT PATH
else
  printf 'aviso: no hay toolchain go1.24.0 en %s; se usa la del PATH\n' "$GOROOT" >&2
fi

# materializar el emulador antes del build. El script se busca junto a este fichero (no bajo
# $RAIZ) porque el vigilante llama a este guion con un árbol temporal que solo contiene apkc/.
HERRAMIENTAS=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd)
materializar=${APKC_MATERIALIZAR:-$HERRAMIENTAS/materializar-emulador.sh}
SUBMODULO=${APKC_EMULADOR_DIR:-$RAIZ/studio/apkc-emulador}
sh "$materializar" "$RAIZ" "$SUBMODULO"

TEMPORAL=$DESTINO.tmp.$$
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -C "$RAIZ/apkc" -trimpath -ldflags='-s -w' -o "$TEMPORAL" .
mv -f "$TEMPORAL" "$DESTINO"
sha256sum "$DESTINO"
