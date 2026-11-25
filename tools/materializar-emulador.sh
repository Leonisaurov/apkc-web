#!/bin/sh
# materializar-emulador.sh — fuentes gobernadas del emulador, no import Go.
#
# El intérprete vive en el único submódulo apkc-emulador/, cuyo gitlink posee studio/ (el
# padre no guarda pin espejo). Los campos que cruza con el núcleo no están exportados
# (tabla.global.cadenas, d.clases, it.insns…), así que meterlo como paquete exigiría
# reescribir más de diez mil líneas. En su lugar este script copia los 34 ficheros `preview*.go`
# a apkc/ justo antes de cada compilación y comprueba con `cmp` que la copia es byte a byte igual.
# apkc/preview*.go está en .gitignore: el único sitio editable es el submódulo.
#
# uso: sh tools/materializar-emulador.sh [raiz-del-worktree] [directorio-del-submodulo]
# El directorio también puede fijarse con APKC_EMULADOR_DIR; si no se da ninguno, se usa
# studio/apkc-emulador. No existe caída al pin apkc-emulador del padre (§357).
set -eu

RAIZ=${1:-$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)}
if [ -n "${2:-}" ]; then
  SUBMODULO=$2
elif [ -n "${APKC_EMULADOR_DIR:-}" ]; then
  SUBMODULO=$APKC_EMULADOR_DIR
else
  SUBMODULO=$RAIZ/studio/apkc-emulador
fi
DESTINO=$RAIZ/apkc

if [ ! -d "$SUBMODULO" ]; then
  printf 'falta el submódulo del emulador en %s (git submodule update --init)\n' "$SUBMODULO" >&2
  exit 2
fi
mkdir -p "$DESTINO"

# El número está fijado a propósito: 8 de producción y 26 tests, incluido
# previewsesion_locale_test.go. Si falta uno o aparece otro, esto falla en vez de compilar a ciegas.
set -- "$SUBMODULO"/preview*.go
if [ "$#" -ne 34 ]; then
  printf 'emulador: esperados 34 ficheros preview*.go en %s, encontrados %s\n' "$SUBMODULO" "$#" >&2
  exit 2
fi

for origen do
  copia=$DESTINO/${origen##*/}
  cp "$origen" "$copia"
  cmp "$origen" "$copia"
done

printf 'materializados %s ficheros preview*.go de %s en %s\n' "$#" "$SUBMODULO" "$DESTINO"
