#!/bin/sh
# sdk34.sh — un SDK espejo con solo lo que el gate necesita.
#
# apkc elige la build-tools mas alta que encuentra y deriva de ahi la platform
# (apkc/doctor.go, latestBuildTools). En un escritorio puede haber varias y eso
# saca al gate del objetivo medido: con build-tools 36 pide android-36. Este
# espejo expone solo build-tools 34.0.0 y platforms/android-34 mediante enlaces,
# sin copiar ni borrar nada del SDK real.
#
# uso: [ANDROID_HOME=/sdk/real] sh tools/sdk34.sh [destino]
# imprime la ruta del espejo.
set -eu

AQUI=$(cd "$(dirname "$0")/.." && pwd)
REAL=${ANDROID_HOME:-$HOME/Android}
DESTINO=${1:-${TMP:-/tmp}/apkc-sdk34}

BT="$REAL/build-tools/34.0.0"
PL="$REAL/platforms/android-34"
[ -d "$BT" ] || { printf 'falta %s (instala build-tools;34.0.0)\n' "$BT" >&2; exit 2; }
[ -f "$PL/android.jar" ] || { printf 'falta %s (instala platforms;android-34)\n' "$PL/android.jar" >&2; exit 2; }

if [ -L "$DESTINO/build-tools/34.0.0" ]; then rm -f "$DESTINO/build-tools/34.0.0"; fi
mkdir -p "$DESTINO/build-tools/34.0.0" "$DESTINO/platforms"
# latestBuildTools() exige un directorio de verdad (IsDir), no un enlace: por eso el directorio
# de la version es real y solo se enlazan sus herramientas una a una.
for herramienta in "$BT"/* "$BT"/.[!.]*; do
  [ -e "$herramienta" ] || continue
  ln -sfn "$herramienta" "$DESTINO/build-tools/34.0.0/$(basename "$herramienta")"
done
ln -sfn "$PL" "$DESTINO/platforms/android-34"
if [ -d "$REAL/platform-tools" ]; then
  ln -sfn "$REAL/platform-tools" "$DESTINO/platform-tools"
fi
printf '%s\n' "$DESTINO"
