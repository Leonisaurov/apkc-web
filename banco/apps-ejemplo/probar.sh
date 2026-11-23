#!/bin/sh
# Instala y arranca las cinco apps en el dispositivo de pruebas por adb, una por una, y deja dicho
# qué pasó con cada una. No borra datos: `install -r` conserva las preferencias.
#
#   sh banco/apps-ejemplo/probar.sh
#
# Variables: SERIE (por defecto 10.177.0.2:5555), APKC_WEB_CACHE.
set -eu
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/apps-ejemplo}
SERIE=${SERIE:-10.177.0.2:5555}

for par in \
	lienzo:com.ejemplo.lienzo \
	notas:com.ejemplo.notas \
	chatbt:com.ejemplo.chatbt \
	tareas:com.ejemplo.tareas \
	cronometro:com.ejemplo.cronometro
do
	app=${par%%:*}
	paquete=${par#*:}
	echo "== $app ($paquete)"
	adb -s "$SERIE" install -r --no-incremental "$CACHE/$app.apk"
	adb -s "$SERIE" shell am start -n "$paquete/.MainActivity"
	sleep 1
	echo "   pid: $(adb -s "$SERIE" shell pidof "$paquete" 2>/dev/null || echo ninguno)"
done
