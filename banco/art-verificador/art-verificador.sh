#!/bin/sh
# art-verificador.sh — la batería de estructuras del dex, ejecutada por ART y no solo desarmada.
#
#   sh banco/art-verificador/art-verificador.sh
#   FALSIFICAR=1 sh banco/art-verificador/art-verificador.sh   # el mismo guion contra un dex cortado
#   CASOS="zchar anota" sh banco/art-verificador/art-verificador.sh
#
# El defecto de `valorEstatico` (`VALUE_INT` en campos `Z`/`C`) lo destapó ART, no `dexdump`: el
# desensamblador verificaba `rc 0` mientras ART rechazaba el dex entero. Esta batería busca los
# demás del mismo tipo. Por cada caso de `fixtures/`:
#
#   1. `apkc build --compiler javac --linker go` deja el APK del camino de `d8` (javac + d8);
#   2. `apkc dex` sobre los MISMOS `.class` deja el `classes.dex` del camino propio, que se mete en
#      una copia del APK, se alinea y se firma otra vez (`reemplaza-dex.py`);
#   3. `dexdump -a` de los dos ficheros se compara en lo que es estructura: clases, campos (con su
#      `static_values`), métodos, accesos y anotaciones;
#   4. cada APK se instala en el aparato (serial fijado, por defecto Droidloom), se arranca y se
#      miran las líneas de SU proceso: ni `Failure to verify`, ni `Verification rejected`, ni
#      `CLASS_ISPREVERIFIED`, ni `dex2oat`, ni `VerifyError`;
#   5. el `Log.d` de la actividad tiene que traer el mismo texto por los dos caminos, y el de
#      `esperado.txt` si el caso lo declara. El oráculo que manda es ART ejecutando.
#
# Con FALSIFICAR=1 se corta el `classes.dex` del primer caso, se exige que la batería diga FALLO, y
# se restaura: un test que no distingue un dex roto no prueba nada.
#
# Necesita: `adb` con el aparato de `SERIE` conectado, el SDK en `ANDROID_HOME` (build-tools y
# `android-34/android.jar`), javac en `JAVA_HOME`, y `apkc` (si falta, se compila desde el árbol).
# No toca red, ni gate, ni chromium; no instala nada fuera del aparato. Todo lo que escribe va a
# `$APKC_WEB_CACHE/art-verificador` (nada de APKs ni binarios versionados).
set -eu

AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/apkc-web/art-verificador}
SERIE=${SERIE:-10.177.0.2:5555}
ANDROID_HOME=${ANDROID_HOME:-$HOME/opt/android-sdk}
JAR=${APKC_ANDROID_JAR:-$ANDROID_HOME/platforms/android-34/android.jar}
BT=${BT:-$(ls -d "$ANDROID_HOME"/build-tools/* 2>/dev/null | sort -V | tail -1)}
ADB=${ADB:-$ANDROID_HOME/platform-tools/adb}
CASOS=${CASOS:-zchar estaticos tries anota arreglos disper cadenas ifaces}

falla() {
	printf 'FALLO: %s\n' "$*" >&2
	exit 1
}

[ -f "$JAR" ] || falla "falta $JAR (usa APKC_ANDROID_JAR)"
for t in d8 dexdump zipalign apksigner; do
	[ -x "$BT/$t" ] || falla "falta $t en $BT"
done
[ -x "$ADB" ] || falla "falta adb en $ADB"
command -v javac >/dev/null 2>&1 || falla "falta javac (pon JAVA_HOME o el PATH)"

mkdir -p "$CACHE"
APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	if command -v tcr >/dev/null 2>&1; then
		( cd "$RAIZ/apkc" && tcr -c 4 -m 3G go build -o "$APKC" . )
	else
		( cd "$RAIZ/apkc" && go build -o "$APKC" . )
	fi
fi

KS=${KEYSTORE:-$CACHE/debug.keystore}
[ -f "$KS" ] || "$APKC" debug-keystore --out "$KS" >/dev/null

# ---------------------------------------------------------------- el aparato
"$ADB" -s "$SERIE" get-state >/dev/null 2>&1 ||
	falla "$SERIE no está conectado (este banco solo usa ese serial)"
echo "aparato  $SERIE ($("$ADB" -s "$SERIE" shell getprop ro.build.version.release | tr -d '\r') / sdk $("$ADB" -s "$SERIE" shell getprop ro.build.version.sdk | tr -d '\r'))"
echo "apkc     $APKC"
echo "javac    $(command -v javac)"
echo "d8       $BT/d8"

# ---------------------------------------------------------------- construcción
# construye: deja en $CACHE/<caso>/ los dos APK y los dos volcados de dexdump.
construye() { # caso
	caso=$1
	W=$CACHE/$caso
	P=$W/proyecto
	rm -rf "$W"
	mkdir -p "$P"
	cp -R "$AQUI/fixtures/$caso/." "$P/"

	echo
	echo "== $caso: build con d8 (apkc build --compiler javac --linker go)"
	( cd "$P" && "$APKC" build --linker go --compiler javac \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" ) \
		>"$W/build-d8.log" 2>&1 || {
		sed 's/^/   | /' "$W/build-d8.log"
		falla "$caso: el build con javac+d8 no terminó"
	}
	sed 's/^/   | /' "$W/build-d8.log" | grep -E 'timing|finished' || true
	d8apk=$P/build/app.apk
	[ -f "$d8apk" ] || falla "$caso: el build no dejó app.apk"

	echo "== $caso: nuestro dex (apkc dex sobre los .class de javac)"
	"$APKC" dex -o "$W/nuestro.dex" "$P/build/classes" >"$W/dex.log" 2>&1 || {
		sed 's/^/   | /' "$W/dex.log"
		falla "$caso: apkc dex no terminó"
	}
	sed 's/^/   | /' "$W/dex.log"
	[ -f "$W/nuestro.dex" ] || falla "$caso: apkc dex no dejó fichero"

	python3 "$AQUI/reemplaza-dex.py" "$d8apk" "$W/sin-firma.apk" "$W/nuestro.dex" >/dev/null ||
		falla "$caso: no se pudo reemplazar classes.dex"
	"$BT/zipalign" -f -p 4 "$W/sin-firma.apk" "$W/alineado.apk" ||
		falla "$caso: zipalign falló"
	"$BT/apksigner" sign --ks "$KS" --ks-pass pass:android --key-pass pass:android \
		--ks-key-alias androiddebugkey --out "$W/nuestro.apk" "$W/alineado.apk" ||
		falla "$caso: apksigner falló"
	echo "   d8      $d8apk ($(wc -c <"$d8apk") bytes)"
	echo "   nuestro $W/nuestro.apk ($(wc -c <"$W/nuestro.apk") bytes, classes.dex $(wc -c <"$W/nuestro.dex") bytes)"

	# el dexdump del camino propio sale del dex suelto; el de d8, del classes.dex del APK
	unzip -p "$d8apk" classes.dex >"$W/d8.dex" 2>/dev/null ||
		falla "$caso: el APK de d8 no lleva classes.dex"
	"$BT/dexdump" -a "$W/d8.dex" >"$W/d8.dump" 2>&1 ||
		{ sed 's/^/   | /' "$W/d8.dump"; falla "$caso: dexdump rechaza el dex de d8"; }
	"$BT/dexdump" -a "$W/nuestro.dex" >"$W/nuestro.dump" 2>&1 ||
		{ sed 's/^/   | /' "$W/nuestro.dump"; falla "$caso: dexdump rechaza nuestro dex"; }

	echo "== $caso: estructura contra d8 (dexdump -a)"
	python3 "$AQUI/compara-estructura.py" "$W/d8.dump" "$W/nuestro.dump" "$caso" || exit 1
}

# corre: instala, arranca y devuelve la línea `RESULTADO` del proceso, o falla con el motivo.
corre() { # caso, apk, ruta del logcat del proceso
	caso=$1
	apk=$2
	log=$3
	pkg=com.ejemplo.artver.$caso

	"$ADB" -s "$SERIE" uninstall "$pkg" >/dev/null 2>&1 || true
	echo "== $caso: install -r"
	if ! salida=$("$ADB" -s "$SERIE" install --no-incremental "$apk" 2>&1); then
		printf '%s\n' "$salida" | sed 's/^/   | /'
		falla "$caso: la instalación no terminó en Success"
	fi
	printf '%s\n' "$salida" | sed 's/^/   | /'
	case $salida in *Success*) ;; *) falla "$caso: la instalación no dijo Success" ;; esac

	"$ADB" -s "$SERIE" logcat -c >/dev/null 2>&1 || true
	"$ADB" -s "$SERIE" shell am force-stop "$pkg" >/dev/null 2>&1 || true
	echo "== $caso: am start -W"
	arranque=$("$ADB" -s "$SERIE" shell am start -W -n "$pkg/.MainActivity" 2>&1)
	printf '%s\n' "$arranque" | sed 's/^/   | /'
	case $arranque in *"Status: ok"*) ;; *) falla "$caso: la actividad no arrancó" ;; esac
	sleep 3

	pid=$("$ADB" -s "$SERIE" shell pidof "$pkg" 2>/dev/null | tr -d '\r' || true)
	if [ -n "$pid" ]; then
		"$ADB" -s "$SERIE" logcat -d --pid="$pid" >"$log" 2>/dev/null || true
	fi
	if [ ! -s "$log" ]; then
		# el proceso ya murió: se queda lo que lo nombra, que es donde vive el rechazo de ART
		"$ADB" -s "$SERIE" logcat -d >"$log" 2>/dev/null || true
	fi
	echo "   pid ${pid:-ninguno: el proceso ya no está vivo}"

	echo "== $caso: logcat del proceso (filtros)"
	for patron in "Verification rejected" "Failure to verify" "CLASS_ISPREVERIFIED" "dex2oat" "VerifyError"; do
		if grep -q "$patron" "$log"; then
			grep "$patron" "$log" | sed 's/^/   | /'
			falla "$caso: el log del proceso lleva «$patron»"
		fi
		echo "   sin «$patron»"
	done
	# El proceso puede morir después de imprimir (el emulador mata la tarea al instalar la
	# siguiente versión, o el usuario la quita): lo que manda es que el RESULTADO esté en el buffer,
	# y los filtros de arriba ya se aplicaron sobre él.
	if [ -z "$pid" ]; then
		echo "   aviso: el proceso ya no está vivo; se busca el RESULTADO en el buffer entero"
	fi

	echo "== $caso: Log.d de la actividad"
	linea=$(grep -F "RESULTADO " "$log" | tail -1 || true)
	[ -n "$linea" ] || {
		tail -12 "$log" | sed 's/^/   | /'
		falla "$caso: no aparece el RESULTADO en logcat"
	}
	printf '%s\n' "$linea" | sed 's/^/   | /'
	printf '%s\n' "$linea" | sed -n 's/.*RESULTADO //p'
}

compara() { # caso, esperado, obtenido, de donde
	caso=$1
	esperado=$2
	obtenido=$3
	de=$4
	if [ "$esperado" != "$obtenido" ]; then
		echo "   esperado ( $de ): $esperado"
		echo "   obtenido        : $obtenido"
		falla "$caso: lo que imprime ART por $de no coincide"
	fi
	echo "   acuerdo con $de: $obtenido"
}

# ---------------------------------------------------------------- falsificación
if [ "${FALSIFICAR:-0}" = "1" ]; then
	caso=$(printf '%s\n' "$CASOS" | awk '{print $1}')
	construye "$caso"
	pkg=com.ejemplo.artver.$caso
	W=$CACHE/$caso

	echo
	echo "== falsificación: $caso con classes.dex cortado a la mitad"
	cp "$W/nuestro.dex" "$W/nuestro-completo.dex"
	python3 - "$W/nuestro.dex" <<'PY'
import sys
ruta = sys.argv[1]
datos = open(ruta, "rb").read()
# a la mitad: un corte fijo de 4096 no rompe un dex pequeño, y la falsificación pasaba sin cortar nada
open(ruta, "wb").write(datos[:len(datos)//2])
PY
	python3 "$AQUI/reemplaza-dex.py" "$W/proyecto/build/app.apk" "$W/sin-firma.apk" "$W/nuestro.dex" >/dev/null
	"$BT/zipalign" -f -p 4 "$W/sin-firma.apk" "$W/alineado.apk"
	"$BT/apksigner" sign --ks "$KS" --ks-pass pass:android --key-pass pass:android \
		--ks-key-alias androiddebugkey --out "$W/roto.apk" "$W/alineado.apk"
	if ( corre "$caso" "$W/roto.apk" "$W/logcat-roto.txt" >"$W/falsificacion.txt" 2>&1 ); then
		cat "$W/falsificacion.txt"
		falla "el test dio OK con el dex cortado: no falsifica nada"
	fi
	cat "$W/falsificacion.txt"
	echo
	echo "OK: el test dice FALLO con el dex cortado (falsificación detectada)"

	# se restaura el APK del camino propio y se exige otra vez verde
	cp "$W/nuestro-completo.dex" "$W/nuestro.dex"
	python3 "$AQUI/reemplaza-dex.py" "$W/proyecto/build/app.apk" "$W/sin-firma.apk" "$W/nuestro.dex" >/dev/null
	"$BT/zipalign" -f -p 4 "$W/sin-firma.apk" "$W/alineado.apk"
	"$BT/apksigner" sign --ks "$KS" --ks-pass pass:android --key-pass pass:android \
		--ks-key-alias androiddebugkey --out "$W/nuestro.apk" "$W/alineado.apk"
	corre "$caso" "$W/nuestro.apk" "$W/logcat-restaurado.txt" >/dev/null ||
		falla "el APK restaurado sigue en FALLO"
	echo "OK: restaurado, $caso vuelve a ejecutar"
	exit 0
fi

# ---------------------------------------------------------------- la batería
VERDES=""
for caso in $CASOS; do
	[ -d "$AQUI/fixtures/$caso" ] || falla "no existe la fixture $caso"
	construye "$caso"
	W=$CACHE/$caso
	echo
	# La transcripción de cada camino se guarda y se imprime entera: install, am start, cada filtro
	# y el Log.d. La última línea es el texto que imprime la actividad, que es lo que se compara.
	echo "== $caso: camino de d8"
	corre "$caso" "$W/proyecto/build/app.apk" "$W/logcat-d8.txt" >"$W/transcripcion-d8.txt" 2>&1 ||
		{ cat "$W/transcripcion-d8.txt"; falla "$caso: el camino de d8 no pasó"; }
	cat "$W/transcripcion-d8.txt"
	d8txt=$(tail -1 "$W/transcripcion-d8.txt")
	echo "== $caso: camino propio"
	corre "$caso" "$W/nuestro.apk" "$W/logcat-nuestro.txt" >"$W/transcripcion-nuestro.txt" 2>&1 ||
		{ cat "$W/transcripcion-nuestro.txt"; falla "$caso: el camino propio no pasó"; }
	cat "$W/transcripcion-nuestro.txt"
	propiotxt=$(tail -1 "$W/transcripcion-nuestro.txt")
	echo "== $caso: los dos caminos"
	compara "$caso" "$d8txt" "$propiotxt" "d8"
	if [ -f "$AQUI/fixtures/$caso/esperado.txt" ]; then
		esperado=$(cat "$AQUI/fixtures/$caso/esperado.txt")
		compara "$caso" "$esperado" "$propiotxt" "esperado.txt"
	fi
	VERDES="$VERDES
$caso: $propiotxt"
	"$ADB" -s "$SERIE" uninstall "com.ejemplo.artver.$caso" >/dev/null 2>&1 || true
done

echo
echo "== resumen (ART ejecutó los dos caminos y dijeron lo mismo)"
printf '%s\n' "$VERDES" | sed '/^$/d' | sed 's/^/   /'
echo
echo "OK: ${CASOS} en verde, sin rechazo de verificación"
