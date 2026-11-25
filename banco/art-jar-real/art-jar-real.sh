#!/bin/sh
# art-jar-real.sh — el APK con el android-support-v4.jar ejecutado en ART real, no solo en dexdump
#
#   sh banco/art-jar-real/art-jar-real.sh
#   FALSIFICAR=1 sh banco/art-jar-real/art-jar-real.sh    # el mismo guion contra un dex inválido
#
# El pase del jar real de `banco/validar-apk/validar.sh` demuestra que `dexdump` acepta las 520
# clases del jar (§225), pero dexdump desarma el fichero: no ejecuta bytecode. Esto mide lo que
# falta: instalar el APK en un aparato real (Droidloom), arrancar la actividad y comprobar que ART
# ejecuta `Landroid/support/v4/text/TextUtilsCompat;.htmlEncode`, que solo existe en
# `classes2.dex`, el dex de la dependencia. La app no define ese método en ningún sitio.
#
# Qué exige, con el proceso de la app aislado por `logcat --pid`:
#   1. `install` termina en Success y `am start -W` en `Status: ok`;
#   2. ni «Verification rejected», ni «Failure to verify», ni «CLASS_ISPREVERIFIED», ni «dex2oat»
#      en las líneas del proceso: ART acepta el dex sin mandarlo a recompilar ni rechazarlo;
#   3. el `Log.d` de la actividad lleva el resultado exacto de
#      htmlEncode("<b>apkc & cia</b>") = "&lt;b&gt;apkc &amp; cia&lt;/b&gt;";
#   4. la UI lo enseña (uiautomator dump), para que la prueba no dependa solo de un log que
#      también podría venir de otro sitio.
#
# Con FALSIFICAR=1 el guion no valida el APK bueno: lo copia, cambia `classes2.dex` por un dex
# truncado (el header promete más bytes de los que hay), lo vuelve a firmar y exige que el mismo
# test diga FALLO. Si el test dijera OK con ese APK, el guion saldría con 1; después reinstala el
# APK bajo prueba.
#
# Con CONTROL_D8=1 cambia `classes2.dex` por el que produce `d8` sobre el mismo jar. No mide
# nuestro dexer: aísla el fallo del fixture y del `classes.dex` de la app cuando el dex de
# dependencias es el que ART rechaza.
#
# Hoy el APK del jar real sale en FALLO: ART no llega a ejecutar nada porque nuestro dexer
# escribe 17 `encoded_value` con tipo `int` en campos `boolean`/`char`. La salida literal y el
# análisis, en README.md.
#
# Solo usa `adb -s $SERIE` (por defecto el serial de red de Droidloom, nunca el USB).
# Variables: SERIE, APKC, JAR_REAL, APK, ANDROID_HOME, APKC_WEB_CACHE, FALSIFICAR.
# No toca red, gate ni chromium. Artefactos en $HOME/.cache/apkc-web/art-jar-real: nada se versiona.
set -eu

AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/art-jar-real}
SERIE=${SERIE:-10.177.0.2:5555}
PAQUETE=com.ejemplo.artjar
ETIQUETA=artjar
ESPERADO='&lt;b&gt;apkc &amp; cia&lt;/b&gt;'
# el mismo texto, escapado otra vez por el volcado XML de uiautomator
ESPERADO_UI='&amp;lt;b&amp;gt;apkc &amp;amp; cia&amp;lt;/b&amp;gt;'
ADB=${ADB:-adb}

falla() {
	printf 'FALLO: %s\n' "$*" >&2
	exit 1
}

mkdir -p "$CACHE"

# ---------------------------------------------------------------- aparato
command -v "$ADB" >/dev/null 2>&1 || falla "no encuentro $ADB"
"$ADB" -s "$SERIE" get-state >/dev/null 2>&1 ||
	falla "$SERIE no está conectado (este guion solo usa ese serial, nunca el USB)"
echo "aparato  $SERIE ($("$ADB" -s "$SERIE" shell getprop ro.build.version.sdk | tr -d '\r') / $("$ADB" -s "$SERIE" shell getprop ro.build.fingerprint | tr -d '\r'))"

# ---------------------------------------------------------------- herramientas
ANDROID_HOME=${ANDROID_HOME:-$HOME/opt/android-sdk}
[ -d "$ANDROID_HOME" ] || ANDROID_HOME=$HOME/Android
JAR_ANDROID=${APKC_ANDROID_JAR:-${ANDROID_JAR:-}}
if [ -z "$JAR_ANDROID" ]; then
	if [ -f "$ANDROID_HOME/platforms/android-34/android.jar" ]; then
		JAR_ANDROID=$ANDROID_HOME/platforms/android-34/android.jar
	else
		JAR_ANDROID=$(ls "$ANDROID_HOME"/platforms/android-*/android.jar 2>/dev/null | sort -V | tail -1)
	fi
fi
[ -f "${JAR_ANDROID:-}" ] || falla "falta android.jar en $ANDROID_HOME/platforms (usa APKC_ANDROID_JAR)"

APKSIGNER=${APKSIGNER:-$(ls "$ANDROID_HOME"/build-tools/*/apksigner 2>/dev/null | sort -V | tail -1)}
[ -x "${APKSIGNER:-}" ] || falla "no encuentro apksigner en $ANDROID_HOME/build-tools"
ZIPALIGN=${ZIPALIGN:-$(ls "$ANDROID_HOME"/build-tools/*/zipalign 2>/dev/null | sort -V | tail -1)}
D8=${D8:-$(ls "$ANDROID_HOME"/build-tools/*/d8 2>/dev/null | sort -V | tail -1)}

APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	sh "$RAIZ/tools/materializar-emulador.sh" "$RAIZ"
	( cd "$RAIZ/apkc" && CGO_ENABLED=0 go build -o "$APKC" . )
fi

JAR_REAL=${JAR_REAL:-$HOME/Downloads/ApkShare-main/jar/android-support-v4.jar}
[ -f "$JAR_REAL" ] || falla "falta el jar real: $JAR_REAL (pásalo en JAR_REAL)"

KS=${KEYSTORE:-$CACHE/debug.keystore}
[ -f "$KS" ] || "$APKC" debug-keystore --out "$KS" >/dev/null

echo "apkc     $APKC"
echo "android  $JAR_ANDROID"
echo "jar      $JAR_REAL ($(wc -c <"$JAR_REAL") bytes)"

# ---------------------------------------------------------------- build
# Un APK se construye una vez y se reutiliza: APK=… se salta el build (la falsificación apunta a
# un APK preparado con otro guion o a mano).
if [ -n "${APK:-}" ]; then
	[ -f "$APK" ] || falla "APK=$APK no existe"
	APK_BUENO=$APK
else
	P=$CACHE/proyecto
	rm -rf "$P"
	mkdir -p "$P/src/com/ejemplo/artjar" "$P/res/values" "$P/jar"
	cp "$AQUI/AndroidManifest.xml" "$P/AndroidManifest.xml"
	cp "$AQUI/res/values/strings.xml" "$P/res/values/strings.xml"
	cp "$AQUI/src/com/ejemplo/artjar/MainActivity.java" "$P/src/com/ejemplo/artjar/MainActivity.java"
	cp "$JAR_REAL" "$P/jar/"
	echo
	echo "== apkc build (linker, compilador y firma en Go)"
	( cd "$P" && "$APKC" build --linker go --compiler go --signer go \
		--android-jar "$JAR_ANDROID" --sdk-version 34 --codename 14 --keystore "$KS" ) \
		>"$CACHE/build.log" 2>&1 || {
		sed 's/^/   | /' "$CACHE/build.log"
		falla "el build del jar real no terminó"
	}
	sed 's/^/   | /' "$CACHE/build.log"
	APK_BUENO=$P/build/app.apk
	[ -f "$APK_BUENO" ] || falla "$P no dejó build/app.apk"
fi
echo "apk      $APK_BUENO ($(wc -c <"$APK_BUENO") bytes)"

# ---------------------------------------------------------------- control con d8
# Este guion mide nuestro dexer. Si ART rechaza el dex que emitió (es el caso hoy: 17 encoded_value
# con tipo `int` en campos boolean/char, ver README), este control aísla la frontera: el mismo APK,
# el mismo fixture y el mismo `classes.dex` de la app (el nuestro), pero `classes2.dex` lo produce
# d8. Si el control está en verde, el fixture, el guion y el dex de la app son correctos y lo único
# que falla es el dexado de dependencias. No sustituye la prueba: no valida nuestro dexer.
if [ "${CONTROL_D8:-0}" = "1" ] && [ -z "${APK:-}" ]; then
	echo
	echo "== control: classes2.dex de d8 (no mide el dexer de apkc)"
	[ -x "${D8:-}" ] || falla "no encuentro d8 en $ANDROID_HOME/build-tools"
	[ -x "${ZIPALIGN:-}" ] || falla "no encuentro zipalign en $ANDROID_HOME/build-tools"
	C=$CACHE/control-d8
	rm -rf "$C"
	mkdir -p "$C"
	JAVA_OPTS=${JAVA_OPTS:--Xmx700m} "$D8" --release --min-api 21 --lib "$JAR_ANDROID" \
		--output "$C" "$JAR_REAL" >"$C/d8.log" 2>&1 || {
		sed 's/^/   | /' "$C/d8.log"
		falla "d8 no dexó el jar"
	}
	echo "   d8 dejó classes.dex de $(wc -c <"$C/classes.dex") bytes"
	python3 - "$APK_BUENO" "$C/app-sin-firma.apk" "$C/classes.dex" <<'PY'
import sys, zipfile

origen, destino, dex = sys.argv[1], sys.argv[2], sys.argv[3]
with zipfile.ZipFile(origen) as zin, zipfile.ZipFile(destino, "w", zipfile.ZIP_DEFLATED) as zout:
    for item in zin.infolist():
        datos = zin.read(item.filename)
        if item.filename == "classes2.dex":
            datos = open(dex, "rb").read()
        zout.writestr(item, datos)
PY
	"$ZIPALIGN" -f -p 4 "$C/app-sin-firma.apk" "$C/app-alineado.apk" ||
		falla "zipalign no alineó el APK del control"
	"$APKSIGNER" sign --ks "$KS" --ks-pass pass:android --key-pass pass:android \
		--ks-key-alias androiddebugkey --out "$C/app.apk" "$C/app-alineado.apk" ||
		falla "apksigner no firmó el APK del control"
	APK_BUENO=$C/app.apk
	echo "apk del control  $APK_BUENO ($(wc -c <"$APK_BUENO") bytes)"
fi

# ---------------------------------------------------------------- el test
# Instala, arranca y comprueba. Devuelve 0 si todo está en verde y 1 con el motivo impreso.
comprueba() {
	apk=$1
	echo
	echo "== install -r"
	if ! salida=$("$ADB" -s "$SERIE" install -r "$apk" 2>&1); then
		printf '%s\n' "$salida"
		# una firma distinta o una instalación previa rota: se desinstala y se reinstala limpio
		echo "   reintento tras desinstalar $PAQUETE"
		"$ADB" -s "$SERIE" uninstall "$PAQUETE" >/dev/null 2>&1 || true
		salida=$("$ADB" -s "$SERIE" install "$apk" 2>&1) || {
			printf '%s\n' "$salida"
			echo "FALLO: la instalación no terminó en Success"
			return 1
		}
	fi
	printf '%s\n' "$salida"
	case $salida in
	*Success*) ;;
	*) echo "FALLO: la instalación no dijo Success"; return 1 ;;
	esac

	"$ADB" -s "$SERIE" logcat -c >/dev/null 2>&1 || true
	"$ADB" -s "$SERIE" shell am force-stop "$PAQUETE" >/dev/null 2>&1 || true

	echo
	echo "== am start -W"
	arranque=$("$ADB" -s "$SERIE" shell am start -W -n "$PAQUETE/.MainActivity" 2>&1)
	printf '%s\n' "$arranque"
	case $arranque in
	*"Status: ok"*) ;;
	*) echo "FALLO: la actividad no arrancó (sin «Status: ok»)"; return 1 ;;
	esac

	sleep 3
	# el buffer entero da contexto también cuando el proceso ya murió (es el caso del jar real:
	# ART rechaza el dex al cargarlo y la actividad no llega a onCreate)
	"$ADB" -s "$SERIE" logcat -d >"$CACHE/logcat.txt" 2>/dev/null || true
	pid=$("$ADB" -s "$SERIE" shell pidof "$PAQUETE" 2>/dev/null | tr -d '\r')
	# el logcat del proceso: aislado por pid; sin proceso, las líneas que lo nombran y las del
	# runtime (la etiqueta del log recorta el nombre del paquete a «.ejemplo.artjar»)
	if [ -n "$pid" ]; then
		"$ADB" -s "$SERIE" logcat -d --pid="$pid" >"$CACHE/logcat-app.txt" 2>/dev/null ||
			cp "$CACHE/logcat.txt" "$CACHE/logcat-app.txt"
	else
		grep -E 'ejemplo\.artjar|AndroidRuntime' "$CACHE/logcat.txt" >"$CACHE/logcat-app.txt" || true
	fi
	echo "   pid ${pid:-ninguno: el proceso ya no está vivo}"

	echo
	echo "== logcat del proceso (filtros)"
	for patron in "Verification rejected" "Failure to verify" "CLASS_ISPREVERIFIED" "dex2oat"; do
		if grep -q "$patron" "$CACHE/logcat-app.txt"; then
			grep "$patron" "$CACHE/logcat-app.txt" | sed 's/^/   | /'
			echo "FALLO: el log del proceso lleva «$patron»"
			return 1
		fi
		echo "   sin «$patron»"
	done

	if [ -z "$pid" ]; then
		tail -15 "$CACHE/logcat-app.txt" | sed 's/^/   | /'
		echo "FALLO: el proceso de $PAQUETE no está vivo después del arranque"
		return 1
	fi

	echo
	echo "== Log.d de la actividad (tag $ETIQUETA)"
	echo "   esperado: $ESPERADO"
	linea=$("$ADB" -s "$SERIE" logcat -d -s "$ETIQUETA":D 2>/dev/null | grep -F "$ESPERADO" | tail -1 || true)
	if [ -z "$linea" ]; then
		"$ADB" -s "$SERIE" logcat -d -s "$ETIQUETA":D | sed 's/^/   | /'
		echo "FALLO: no aparece en logcat el resultado de htmlEncode del jar"
		return 1
	fi
	printf '%s\n' "$linea" | sed 's/^/   | /'

	echo
	echo "== uiautomator dump (la UI lo enseña)"
	"$ADB" -s "$SERIE" shell uiautomator dump /sdcard/artjar.xml >/dev/null 2>&1 || true
	"$ADB" -s "$SERIE" pull /sdcard/artjar.xml "$CACHE/ui.xml" >/dev/null 2>&1 || true
	if ! grep -Fq "$ESPERADO_UI" "$CACHE/ui.xml" 2>/dev/null; then
		sed 's/></>\n</g' "$CACHE/ui.xml" 2>/dev/null | grep -i 'text=' | sed 's/^/   | /' | head -5
		echo "FALLO: la UI no enseña «$ESPERADO_UI»"
		return 1
	fi
	echo "   el TextView lleva el htmlEncode del jar"
	return 0
}

# ---------------------------------------------------------------- falsificación
# Un dex truncado es inválido para cualquier lector de dex: el header de `classes2.dex` promete un
# tamaño que el fichero ya no tiene. Se reescribe el zip del APK bueno con la entrada recortada, se
# alinea y se firma otra vez, porque tocar el zip invalida la firma anterior.
if [ "${FALSIFICAR:-0}" = "1" ]; then
	[ -x "${ZIPALIGN:-}" ] || falla "no encuentro zipalign en $ANDROID_HOME/build-tools"
	echo
	echo "== falsificación: APK con classes2.dex truncado"
	F=$CACHE/falsificado
	rm -rf "$F"
	mkdir -p "$F"
	python3 - "$APK_BUENO" "$F/app-sin-firma.apk" <<'PY'
import sys, zipfile

origen, destino = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(origen) as zin, zipfile.ZipFile(destino, "w", zipfile.ZIP_DEFLATED) as zout:
    for item in zin.infolist():
        datos = zin.read(item.filename)
        if item.filename == "classes2.dex":
            datos = datos[:4096]
        zout.writestr(item, datos)
PY
	"$ZIPALIGN" -f -p 4 "$F/app-sin-firma.apk" "$F/app-alineado.apk" ||
		falla "zipalign no alineó el APK falsificado"
	"$APKSIGNER" sign --ks "$KS" --ks-pass pass:android --key-pass pass:android \
		--ks-key-alias androiddebugkey --out "$F/app.apk" "$F/app-alineado.apk" ||
		falla "apksigner no pudo firmar el APK falsificado"
	echo "   $F/app.apk ($(wc -c <"$F/app.apk") bytes, classes2.dex 4096 B)"
	if comprueba "$F/app.apk" >"$CACHE/falsificacion.txt" 2>&1; then
		cat "$CACHE/falsificacion.txt"
		falla "el test dio OK con el dex inválido: la prueba no falsifica nada"
	fi
	cat "$CACHE/falsificacion.txt"
	echo
	echo "OK: el test dice FALLO con el dex inválido (falsificación detectada)"
	echo
	echo "== restaura el APK bajo prueba"
	if comprueba "$APK_BUENO"; then
		echo
		echo "OK: restaurado, el APK bajo prueba vuelve a arrancar y ejecuta"
		echo "    android.support.v4.text.TextUtilsCompat.htmlEncode"
	else
		echo
		echo "AVISO: el APK bajo prueba sigue en FALLO tras restaurarlo (con el jar real es el"
		echo "       estado de hoy: el dexer escribe 17 encoded_value con el tipo equivocado)"
	fi
	exit 0
fi

comprueba "$APK_BUENO" || falla "el APK del jar real no pasa la prueba"
echo
echo "OK: ART ejecuta el jar real — install Success, am start ok, sin rechazo de verificación,"
echo "    Log.d con «$ESPERADO» y la UI lo pinta"
