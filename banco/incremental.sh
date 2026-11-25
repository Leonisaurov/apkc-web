#!/bin/sh
# Banco del incremental: mide `apkc build --linker go --compiler go`, con y sin `--inc`, sobre el
# mismo proyecto grande (231 fuentes .java y 79 ficheros de recurso, generados por
# banco/gen-proyecto-grande.py), y comprueba que los dos caminos producen el mismo apk entrada por
# entrada. Es la medicion que sostiene docs/plan-incremental-web.md: sin este guion, las cifras de
# alli no serian reproducibles.
#
#   uso: sh banco/incremental.sh                 # camino Go de la pestana (por defecto)
#        COMPILER=javac sh banco/incremental.sh  # referencia: javac + d8 del SDK
#
# Variables: APKC (binario; si no existe se compila desde apkc/), ANDROID_JAR, TRABAJO (directorio
# temporal; se crea con mktemp -d y no se borra al terminar), REPETICIONES (3 por defecto).
#
# Lo que mide cada vuelta: build en frio (sin build/), una edicion de un solo .java, y build en
# caliente. Con `--inc` la vuelta caliente aprovecha el estado de build/incremental.json; sin `--inc`
# reconstruye todo. El resultado va a $TRABAJO/tiempos.tsv y los apks de la ultima repeticion quedan
# alli para compararlos.
set -e
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mkdir -p "${XDG_CACHE_HOME:-$HOME/.cache}"
TRABAJO=${TRABAJO:-$(mktemp -d "${XDG_CACHE_HOME:-$HOME/.cache}/apkc-incremental.XXXXXX")}
REPETICIONES=${REPETICIONES:-3}
COMPILER=${COMPILER:-go}
# El mismo presupuesto de CPU y memoria con el que se midió: dos hilos y un GC que no crece de más.
export GOGC=${GOGC:-50}
export GOMAXPROCS=${GOMAXPROCS:-2}

APKC=${APKC:-$TRABAJO/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	sh "$AQUI/tools/materializar-emulador.sh" "$AQUI"
	( cd "$AQUI/apkc" && CGO_ENABLED=0 go build -o "$APKC" . )
fi

# El android.jar: la bandera --android-jar es obligatoria en este camino porque el compilador Go lo
# necesita para resolver los nombres de android.* y para la tabla de ids del framework.
if [ -z "${ANDROID_JAR:-}" ]; then
	for j in "$HOME/Android/platforms/android-34/android.jar" \
		"${ANDROID_HOME:-/nonexistent}/platforms/android-34/android.jar" \
		"$HOME/opt/android-sdk/platforms/android-34/android.jar"; do
		if [ -f "$j" ]; then ANDROID_JAR=$j; break; fi
	done
fi
[ -n "${ANDROID_JAR:-}" ] && [ -f "$ANDROID_JAR" ] || {
	echo "no encuentro android.jar: di ANDROID_JAR=/ruta/android.jar" >&2
	exit 1
}

# Un keystore de depuracion propio del trabajo: no toca ~/.android y hace la medicion repetible en
# cualquier maquina. Lo escribe el propio apkc, sin keytool.
KS=$TRABAJO/debug.keystore
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

FLAGS="--linker go --compiler $COMPILER --android-jar $ANDROID_JAR --sdk-version 34 --codename 14 --keystore $KS --v4=true"
ahora() { date +%s%N; }

construir() { # arbol, extra, etiqueta
	t0=$(ahora)
	( cd "$1" && "$APKC" build $FLAGS $2 ) >"$TRABAJO/log-$3.txt" 2>&1
	t1=$(ahora)
	ms=$(( (t1 - t0) / 1000000 ))
	printf '%s\t%s\n' "$3" "$ms" | tee -a "$TRABAJO/tiempos.tsv"
}

editar() { # arbol, valor viejo, valor nuevo: un solo fuente cambia de contenido
	sed -i "s/fBase + $2;/fBase + $3;/" "$1/src/com/example/big/Mod037.java"
}

for modo in plain inc; do
	python3 "$AQUI/banco/gen-proyecto-grande.py" "$TRABAJO/proj-$modo" 200 >/dev/null
done

rep=1
while [ "$rep" -le "$REPETICIONES" ]; do
	viejo=$(( (rep - 1) * 100 + 37 ))
	nuevo=$(( rep * 100 + 37 ))
	for modo in plain inc; do
		extra=
		[ "$modo" = inc ] && extra=--inc
		dir=$TRABAJO/proj-$modo
		( cd "$dir" && "$APKC" clean ) >/dev/null 2>&1 || true
		construir "$dir" "$extra" "r$rep-$modo-frio"
		editar "$dir" "$viejo" "$nuevo"
		construir "$dir" "$extra" "r$rep-$modo-caliente"
		if [ "$modo" = inc ] && [ -f "$dir/build/incremental.json" ]; then
			printf 'r%s-incremental.json-bytes\t%s\n' "$rep" "$(wc -c <"$dir/build/incremental.json" | tr -d ' ')" |
				tee -a "$TRABAJO/tiempos.tsv"
		fi
		cp "$dir/build/app.apk" "$TRABAJO/apk-r$rep-$modo.apk"
	done
	rep=$((rep + 1))
done

echo
echo "medianas de la pasada caliente (tres repeticiones):"
for modo in plain inc; do
	mediana=$(grep -- "-$modo-caliente" "$TRABAJO/tiempos.tsv" | cut -f2 | sort -n | sed -n '2p')
	echo "  $modo: ${mediana} ms"
done

# I4: el dex de la vuelta caliente tiene que ser el mismo fichero, byte a byte, que el de un dexado
# completo del mismo arbol. Se guarda el de la ultima vuelta caliente del modo inc y se compara
# contra un build desde cero de ese mismo arbol: si difiere, el ahorro es un cambio de comportamiento
# y no vale nada (criterio de abandono de I4 en docs/plan-incremental-web.md).
cp "$TRABAJO/proj-inc/build/classes.dex" "$TRABAJO/dex-caliente.dex"
( cd "$TRABAJO/proj-inc" && "$APKC" clean ) >/dev/null 2>&1 || true
construir "$TRABAJO/proj-inc" "--inc" "dex-completo"
if cmp "$TRABAJO/dex-caliente.dex" "$TRABAJO/proj-inc/build/classes.dex"; then
	echo "dex por clase: la vuelta caliente y el dexado completo son identicos"
else
	echo "dex por clase: DIFIEREN" >&2
	exit 1
fi

# La comparacion que hace util el numero: si los apks no son iguales entrada por entrada, el ahorro
# es un cambio de comportamiento y no vale nada.
python3 - "$TRABAJO/apk-r$REPETICIONES-plain.apk" "$TRABAJO/apk-r$REPETICIONES-inc.apk" <<'PY'
import hashlib, sys, zipfile
a, b = (zipfile.ZipFile(p) for p in sys.argv[1:3])
na, nb = a.namelist(), b.namelist()
if na != nb:
    sys.exit("los apks no tienen las mismas entradas en el mismo orden")
malas = [n for n in na
         if hashlib.sha256(a.read(n)).digest() != hashlib.sha256(b.read(n)).digest()]
print("entradas: %d, con bytes distintos: %d %s" % (len(na), len(malas), malas[:5] or ""))
sys.exit(1 if malas else 0)
PY

if command -v apksigner >/dev/null 2>&1; then
	apksigner verify --verbose "$TRABAJO/apk-r$REPETICIONES-inc.apk" | sed -n '1,4p'
fi
if command -v zipalign >/dev/null 2>&1; then
	zipalign -c -p 4 "$TRABAJO/apk-r$REPETICIONES-inc.apk" && echo "zipalign: correcto"
fi
echo "todo en $TRABAJO"
