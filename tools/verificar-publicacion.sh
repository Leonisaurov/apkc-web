#!/bin/sh
# Comprueba que el binario publicado es byte a byte el que se empaquetó. Usa HTTP/1.1 porque el
# host actual (Surge) trunca respuestas grandes por HTTP/2 desde este entorno; con H1 llega entero.
# Uso: sh tools/verificar-publicacion.sh https://dominio/ /ruta/apkc-wasm.wasm
set -eu
URL=${1:?falta la URL publicada}
WASM=${2:?falta el wasm local}
TMP=$(mktemp -d "${TMPDIR:-/tmp}/apkc-publicado.XXXXXX")
NOMBRE=$(printf '%s' "$URL" | sed 's#^https\?://##; s#/.*##')
curl --http1.1 -fsS -o "$TMP/apkc.wasm.gz" "https://$NOMBRE/apkc.wasm.gz"
LOCAL=$(sha256sum "$WASM.gz" | cut -d ' ' -f 1)
REMOTO=$(sha256sum "$TMP/apkc.wasm.gz" | cut -d ' ' -f 1)
if [ "$LOCAL" != "$REMOTO" ]; then
  printf 'el wasm publicado no coincide con el local: %s contra %s\n' "$REMOTO" "$LOCAL" >&2
  exit 1
fi
printf 'publicación verificada: sha256 %s\n' "$REMOTO"
