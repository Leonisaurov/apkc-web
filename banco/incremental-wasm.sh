#!/bin/sh
# Medición 3 del incremental web, reproducible desde el repo: compila el test
# apkc/incremental_wasm_test.go a wasm con la toolchain fijada, monta el proyecto grande y el
# android.jar en el hospedador con forma de navegador y ejecuta dos `_build` por modo (plain e inc)
# dentro de un mismo proceso. Sin chromium, sin OPFS, sin red y sin gate.
#
#   uso: sh banco/incremental-wasm.sh
#
# Variables: ANDROID_JAR, NCLASES (200 por defecto, como banco/incremental.sh), TMP (bajo
# $HOME/.cache/apkc-web por defecto). Los temporales nunca van a /tmp, que es tmpfs con cuota.
set -eu

AQUI=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
HOME_REAL=${HOME:-/home/leonisauro}

# Go 1.24.0 fijado (§312). wasm-exec-hash.sh valida versión y SHA de wasm_exec.js antes del cp;
# APKC_GOROOT es el override que el propio verificador documenta para no depender del GOROOT
# ambiental (aquí el Go del sistema es 1.27.4).
GOROOT_FIJADO=${APKC_GOROOT:-$HOME_REAL/sdk/go1.24.0}
if [ ! -x "$GOROOT_FIJADO/bin/go" ]; then
	echo "no hay Go fijado en $GOROOT_FIJADO: instala go1.24.0 o fija APKC_GOROOT" >&2
	exit 1
fi
APKC_GOROOT=$GOROOT_FIJADO
PATH=$GOROOT_FIJADO/bin:$PATH
export APKC_GOROOT PATH

TMP=${TMP:-$HOME_REAL/.cache/apkc-web/incremental-wasm}
mkdir -p "$TMP/gotmp"
export TMP TMPDIR=$TMP GOTMPDIR=$TMP/gotmp
go version

# El jar se monta en /sdk/android.jar, que es la última ruta que prueba rutaDelJar
# (apkc/jarindice_test.go:291-304) y la que no necesita figura de SDK en el fs simulado.
if [ -z "${ANDROID_JAR:-}" ]; then
	for j in "${ANDROID_HOME:-$HOME_REAL/opt/android-sdk}/platforms/android-34/android.jar" \
		"$HOME_REAL/Android/platforms/android-34/android.jar"; do
		[ -f "$j" ] && { ANDROID_JAR=$j; break; }
	done
fi
[ -n "${ANDROID_JAR:-}" ] && [ -f "$ANDROID_JAR" ] || {
	echo "no encuentro android.jar: di ANDROID_JAR=/ruta/android.jar" >&2
	exit 1
}

NCLASES=${NCLASES:-200}
PROY=$(mktemp -d "$TMP/proyecto.XXXXXX")
python3 "$AQUI/banco/gen-proyecto-grande.py" "$PROY" "$NCLASES" >/dev/null

# El hospedador lee ./wasm_exec.js; se copia solo tras pasar el verificador (regla 2: no se versiona).
( cd "$AQUI/shim" && . "$AQUI/tools/wasm-exec-hash.sh" && cp "$WASM_EXEC_ORIGEN" . )

# los preview*.go se materializan desde el submódulo antes de compilar el banco
sh "$AQUI/tools/materializar-emulador.sh" "$AQUI"
( cd "$AQUI/apkc" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$TMP/inc.wasm" . )

# --montar solo acepta ficheros sueltos, así que se enumera el árbol generado. Se guarda la lista
# en un fichero para que el while no corra en una subshell y conserve los argumentos acumulados.
LISTA=$TMP/montajes.txt
find "$PROY" -type f | sort >"$LISTA"
set --
while IFS= read -r f; do
	set -- "$@" --montar "$f:/proyecto/${f#$PROY/}"
done <"$LISTA"
set -- "$@" --montar "$ANDROID_JAR:/sdk/android.jar"

# GOGC y GOMAXPROCS se fijan como en banco/incremental.sh para acotar el presupuesto del proceso;
# el hospedador construye su propio go.env, así que el preset va en el entorno del nodo.
GOGC=50 GOMAXPROCS=2 tcr -c 2 -m 2G -r 400M -- \
	node "$AQUI/shim/navegador-sim.js" "$@" "$TMP/inc.wasm" \
	-test.run=TestBuildDosVecesEnWasmMideEtapas -test.v
