#!/data/data/com.termux/files/usr/bin/sh
# entorno.sh — un solo sitio para el entorno de apkc-web.
#
# El shell de este aparato arranca con HOME=/home/leonisauro, que no tiene el repo ni el SDK, y con
# PREFIX vacío. Sin esto, `studio/e2e.sh` busca `../apkc` y `~/Develop/Go/apkc-harness/zipcheck.py`
# en sitios que no existen y falla con ENOENT aunque el código esté bien.
#
# Uso:
#   tools/entorno.sh test              # suite nativa de apkc/
#   tools/entorno.sh wasm              # suite en el hospedador con forma de navegador
#   tools/entorno.sh gate              # binarios + studio/e2e.sh (la página de punta a punta)
#   tools/entorno.sh banco             # banco/banco.sh (necesita rish para los caminos del aparato)
#   tools/entorno.sh jar [api]         # ruta del android.jar local o lo instala con sdkmanager (fuente oficial)
#   tools/entorno.sh doctor            # imprime el entorno resuelto
#   tools/entorno.sh env               # exports listos para `eval "$(tools/entorno.sh env)"`
#   tools/entorno.sh -- <comando...>   # ejecuta cualquier comando con el entorno ya puesto
#
# Variables que se respetan si ya vienen puestas: HOME, PREFIX, ANDROID_HOME, APKC, APKC_WASM.
set -eu

RAIZ=$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)

# HOME del repo, no el del shell: apkc-web vive bajo <home>/Develop/Go/apkc-web.
HOME=${HOME_APKC_WEB:-$(CDPATH= cd -P -- "$RAIZ/../../.." && pwd)}
PREFIX=${PREFIX:-/data/data/com.termux/files/usr}
ANDROID_HOME=${ANDROID_HOME:-$HOME/Android}
TMP=${TMP:-$PREFIX/tmp}
export HOME PREFIX ANDROID_HOME TMP

# build-tools más alta que exista; puede no haber ninguna y entonces se deja el PATH como estaba.
BUILD_TOOLS=
if [ -d "$ANDROID_HOME/build-tools" ]; then
  BUILD_TOOLS=$(ls -d "$ANDROID_HOME"/build-tools/* 2>/dev/null | sort -V | tail -1 || true)
fi
JDK=$PREFIX/lib/jvm/java-21-openjdk/bin
# /usr/sbin delante a proposito: el `sudo` de Termux ($PREFIX/bin/sudo) no encuentra un `su` que
# soporte -c, y el del sistema sí. Es el que `banco/banco.sh` usa para app_process64 y dexdump.
PATH_EXTRA=/usr/sbin:$PREFIX/bin
[ -d "$JDK" ] && PATH_EXTRA=$JDK:$PATH_EXTRA
[ -n "$BUILD_TOOLS" ] && PATH_EXTRA=$BUILD_TOOLS:$PATH_EXTRA
PATH=$PATH_EXTRA:$PATH
export PATH

# El android.jar no se versiona; se toma del SDK que ya esté instalado. Esta es la única
# definición de "dónde está el jar" que usan doctor y la tarea jar.
jar_del_sdk() {
  api=${1:-34}
  for j in "$ANDROID_HOME/platforms/android-$api/android.jar" \
           "$HOME/Android/platforms/android-$api/android.jar"; do
    [ -f "$j" ] && { printf '%s\n' "$j"; return 0; }
  done
  return 1
}

# tcr de Termux si está; si no, el comando tal cual.
tcr_o_directo() {
  if command -v tcr >/dev/null 2>&1; then
    tcr -j 1 -q -- "$@"
  else
    "$@"
  fi
}

compila_wasm() {
  ( cd "$RAIZ/apkc" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$TMP/apkc-test.wasm" . )
  ( cd "$RAIZ/shim" && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" . )
}

construye_binarios() {
  ( cd "$RAIZ/apkc" && go build -o "$TMP/apkc-gate" . )
  ( cd "$RAIZ/apkc" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$TMP/apkc-wasm.wasm" . )
}

verifica_entorno() {
  printf 'RAIZ=%s\nHOME=%s\nPREFIX=%s\nANDROID_HOME=%s\nTMP=%s\n' "$RAIZ" "$HOME" "$PREFIX" "$ANDROID_HOME" "$TMP"
  printf 'build-tools=%s\n' "${BUILD_TOOLS:-<ninguna>}"
  printf 'java=%s\n' "$(command -v java || echo '<no>')"
  if j=$(jar_del_sdk 34); then printf 'android.jar=%s\n' "$j"; else printf 'android.jar=<falta; tools/entorno.sh jar>\n'; fi
  printf 'apkc=%s\n' "${APKC:-$TMP/apkc-gate}"
  printf 'apkc-wasm=%s\n' "${APKC_WASM:-$TMP/apkc-wasm.wasm}"
  for f in "$HOME/Android/platforms/android-34/android.jar" \
           "$HOME/Develop/Go/apkc-harness/zipcheck.py"; do
    [ -e "$f" ] && printf 'ok %s\n' "$f" || printf 'FALTA %s\n' "$f"
  done
}

tarea=${1:-doctor}
[ $# -gt 0 ] && shift || true

case "$tarea" in
  test)
    ( cd "$RAIZ/apkc" && tcr_o_directo go test -count=1 ./... )
    ;;
  wasm)
    compila_wasm
    ( cd "$RAIZ/shim" && node navegador-sim.js "$TMP/apkc-test.wasm" )
    ;;
  gate)
    construye_binarios
    ( cd "$RAIZ/studio" && APKC=${APKC:-$TMP/apkc-gate} APKC_WASM=${APKC_WASM:-$TMP/apkc-wasm.wasm} sh e2e.sh )
    ;;
  banco)
    ( cd "$RAIZ" && APKC=${APKC:-$TMP/apkc-gate} sh banco/banco.sh "$@" )
    ;;
  jar)
    api=${1:-34}
    if j=$(jar_del_sdk "$api"); then printf '%s\n' "$j"; exit 0; fi
    sdkmanager=$(command -v sdkmanager 2>/dev/null || true)
    if [ -z "$sdkmanager" ]; then
      for f in "$ANDROID_HOME"/cmdline-tools/*/bin/sdkmanager; do
        [ -x "$f" ] && { sdkmanager=$f; break; }
      done
    fi
    if [ -z "$sdkmanager" ]; then
      printf 'no hay android.jar de android-%s ni sdkmanager en %s\n' "$api" "$ANDROID_HOME" >&2
      printf 'instala cmdline-tools y repite: tools/entorno.sh jar %s\n' "$api" >&2
      exit 2
    fi
    printf 'instalando platforms;android-%s desde el repositorio oficial de Google (sdkmanager)\n' "$api" >&2
    yes | "$sdkmanager" --sdk_root="$ANDROID_HOME" "platforms;android-$api" >&2
    if j=$(jar_del_sdk "$api"); then printf '%s\n' "$j"; else
      printf 'sdkmanager terminó sin dejar %s/platforms/android-%s/android.jar\n' "$ANDROID_HOME" "$api" >&2
      exit 2
    fi
    ;;
  doctor)
    verifica_entorno
    ;;
  env)
    # para `eval "$(tools/entorno.sh env)"` en fish/bash
    printf 'export HOME=%s\n' "$HOME"
    printf 'export PREFIX=%s\n' "$PREFIX"
    printf 'export ANDROID_HOME=%s\n' "$ANDROID_HOME"
    printf 'export TMP=%s\n' "$TMP"
    printf 'export PATH=%s\n' "$PATH"
    ;;
  --)
    exec "$@"
    ;;
  *)
    # comando suelto: `tools/entorno.sh -- go test ./...` o `tools/entorno.sh go env GOROOT`
    exec "$tarea" "$@"
    ;;
esac
