#!/bin/sh
# Construye las cinco apps de ejemplo con el camino todo-Go (sin aapt2, sin javac, sin d8) y deja
# cada apk en $HOME/.cache/apkc-web/apps-ejemplo. No versiona binarios: build/ está ignorado.
#
#   sh banco/apps-ejemplo/construir.sh
#
# Variables: APKC (binario), ANDROID_JAR, KEYSTORE, APKC_WEB_CACHE.
set -eu
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/apps-ejemplo}
mkdir -p "$CACHE"

APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	sh "$RAIZ/tools/materializar-emulador.sh" "$RAIZ"
	( cd "$RAIZ/apkc" && CGO_ENABLED=0 go build -o "$APKC" . )
fi

JAR=${ANDROID_JAR:-$(sh "$RAIZ/tools/entorno.sh" jar)}
KS=${KEYSTORE:-$CACHE/debug.keystore}
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

for app in lienzo notas chatbt tareas cronometro; do
	echo "== $app"
	( cd "$AQUI/$app" && "$APKC" build --linker go --compiler go \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" )
	cp "$AQUI/$app/build/app.apk" "$CACHE/$app.apk"
	echo "   $CACHE/$app.apk"
done
