#!/bin/sh
# entorno.sh — un solo sitio para el entorno de apkc-web.
#
# El shell de este aparato arranca con HOME=/home/leonisauro, que no tiene el repo ni el SDK, y con
# PREFIX vacío. Sin esto, `studio/e2e.sh` busca `../apkc` y `~/Develop/Go/apkc-harness/zipcheck.py`
# en sitios que no existen y falla con ENOENT aunque el código esté bien. En un escritorio tampoco
# existe /data/data/com.termux/files/usr: ahí los temporales van a $XDG_CACHE_HOME/apkc-web.
#
# Uso:
#   tools/entorno.sh test              # suite nativa de apkc/
#   tools/entorno.sh wasm              # suite en el hospedador con forma de navegador
#   tools/entorno.sh gate              # binarios + studio/e2e.sh (la página de punta a punta)
#   tools/entorno.sh banco             # banco/banco.sh (necesita rish para los caminos del aparato)
#   tools/entorno.sh wisp              # falsificación de los dos gates WISP, sin red ni navegador
#   tools/entorno.sh wisp-externo      # gate del modo externo; solo con APKC_WISP_EXTERNO=1 (si no, rc 2)
#   tools/entorno.sh jar [api]         # ruta del android.jar local o lo instala con sdkmanager (fuente oficial)
#   tools/entorno.sh sitio             # empaqueta sitio/ listo para subir con surge
#   tools/entorno.sh publicar          # empaqueta, publica en Surge y verifica desde el host
#   tools/entorno.sh doctor            # imprime el entorno resuelto
#   tools/entorno.sh env               # exports listos para `eval "$(tools/entorno.sh env)"`
#   tools/entorno.sh -- <comando...>   # ejecuta cualquier comando con el entorno ya puesto
#
# Variables que se respetan si ya vienen puestas: HOME, PREFIX, ANDROID_HOME, APKC, APKC_WASM.
set -eu

RAIZ=$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)

# HOME del repo, no el del shell: apkc-web vive bajo <home>/Develop/Go/apkc-web. Se guarda el HOME
# real antes de reescribirlo porque la toolchain fijada de Go vive en <home-real>/sdk.
HOME_REAL=$HOME
HOME=${HOME_APKC_WEB:-$(CDPATH= cd -P -- "$RAIZ/../../.." && pwd)}
if [ -z "${PREFIX:-}" ] || [ ! -d "${PREFIX:-/no-existe}" ]; then
  if [ -d /data/data/com.termux/files/usr ]; then
    PREFIX=/data/data/com.termux/files/usr
  else
    PREFIX=${XDG_CACHE_HOME:-$HOME/.cache}/apkc-web
  fi
fi
ANDROID_HOME=${ANDROID_HOME:-$HOME/Android}
TMP=${TMP:-$PREFIX/tmp}
mkdir -p "$TMP"
# apkc-harness es público: zipcheck.py es un oráculo de zip ajeno a apkc, no se versiona aquí.
APKC_HARNESS=${APKC_HARNESS:-$PREFIX/apkc-harness}
export HOME PREFIX ANDROID_HOME TMP APKC_HARNESS

# build-tools más alta que exista; puede no haber ninguna y entonces se deja el PATH como estaba.
BUILD_TOOLS=
if [ -d "$ANDROID_HOME/build-tools" ]; then
  BUILD_TOOLS=$(ls -d "$ANDROID_HOME"/build-tools/* 2>/dev/null | sort -V | tail -1 || true)
fi
JDK=${JAVA_HOME:-$PREFIX/lib/jvm/java-21-openjdk}/bin
# /usr/sbin delante a proposito: el `sudo` de Termux ($PREFIX/bin/sudo) no encuentra un `su` que
# soporte -c, y el del sistema sí. Es el que `banco/banco.sh` usa para app_process64 y dexdump.
PATH_EXTRA=/usr/sbin:$PREFIX/bin
[ -d "$JDK" ] && PATH_EXTRA=$JDK:$PATH_EXTRA
[ -n "$BUILD_TOOLS" ] && PATH_EXTRA=$BUILD_TOOLS:$PATH_EXTRA
PATH=$PATH_EXTRA:$PATH
export PATH

# go1.24.0 es la toolchain fijada por W2: la usan las compilaciones de apkc/ y, a través de
# tools/wasm-exec-hash.sh, los tres cp de wasm_exec.js. No se toca el Go del sistema.
GOROOT=${APKC_GOROOT:-$HOME_REAL/sdk/go1.24.0}
if [ -x "$GOROOT/bin/go" ]; then
  PATH=$GOROOT/bin:$PATH
  # El verificador no debe depender del HOME reescrito en worktrees: comparte el mismo GOROOT.
  APKC_GOROOT=$GOROOT
  export GOROOT APKC_GOROOT PATH
else
  printf 'aviso: no hay toolchain go1.24.0 en %s; wasm-exec-hash.sh abortará\n' "$GOROOT" >&2
fi
unset HOME_REAL

# Corredor de pruebas: el entorno del escritorio se fija aquí una sola vez. Los agentes
# llaman a `tools/entorno.sh <tarea>` (o a `tools/correr.sh <tarea>`) y no tienen que
# acordarse de TCR, del guardia de RAM ni de la librería suelta de chromium.
CORREDOR_BASE=$PREFIX/corredor
mkdir -p "$CORREDOR_BASE"

if [ -z "${TCR:-}" ] && command -v tcr >/dev/null 2>&1; then
  TCR=$CORREDOR_BASE/tcr
  GUARDIA=${CORREDOR_GUARDIA_MB:-300}
  # e2e.sh y tcr_o_directo pasan `-j 1 -q --`; el envoltorio los descarta para no
  # duplicarlos y fija el guardia en 300 MiB (con 1 GiB el gate muere con 137).
  cat > "$TCR" <<ENVOLTORIO
#!/bin/sh
if [ "\${1:-}" = "-j" ] && [ "\${3:-}" = "-q" ] && [ "\${4:-}" = "--" ]; then
  shift 4
fi
exec tcr -r ${GUARDIA}M -j 1 -q -- "\$@"
ENVOLTORIO
  chmod +x "$TCR"
fi
export TCR

# El chromium del sistema necesita libsimdutf.so.36, que no está en el ldconfig. Sin
# esta variable el debugger no abre y e2e.sh lo confunde con un puerto ocupado.
SHIMLIB=${SHIMLIB:-$HOME/.cache/apkc-web-gate-final/shimlib}
LD_LIBRARY_PATH=${LD_LIBRARY_PATH:-}
if [ -d "$SHIMLIB" ]; then
  case ":$LD_LIBRARY_PATH:" in
    *":$SHIMLIB:"*) ;;
    *) LD_LIBRARY_PATH=${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$SHIMLIB ;;
  esac
  export LD_LIBRARY_PATH
fi

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
  if [ -n "${TCR:-}" ]; then
    "$TCR" -j 1 -q -- "$@"
  else
    "$@"
  fi
}

# El emulador vive en el submódulo apkc-emulador/; sus fuentes se materializan en apkc/ antes de
# cualquier compilación. Los cuatro puntos que compilan (test, wasm, gate y sitio/publicar) pasan
# por aquí, así que ningún camino puede quedarse sin los 34 ficheros preview*.go.
materializar_emulador() {
  sh "$RAIZ/tools/materializar-emulador.sh" "$RAIZ"
}

compila_wasm() {
  materializar_emulador
  ( cd "$RAIZ/apkc" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$TMP/apkc-test.wasm" . )
  # La versión de Go y la suma de wasm_exec.js se fijan en un solo sitio; el cp lee de ahí.
  ( cd "$RAIZ/shim" && . "$RAIZ/tools/wasm-exec-hash.sh" && cp "$WASM_EXEC_ORIGEN" . )
}

construye_wasm() {
  materializar_emulador
  ( cd "$RAIZ/apkc" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build \
      -ldflags='-s -w' -o "$TMP/apkc-wasm.wasm" . )
}

construye_binarios() {
  materializar_emulador
  ( cd "$RAIZ/apkc" && go build -o "$TMP/apkc-gate" . )
  construye_wasm
}

verifica_entorno() {
  printf 'RAIZ=%s\nHOME=%s\nPREFIX=%s\nANDROID_HOME=%s\nTMP=%s\n' "$RAIZ" "$HOME" "$PREFIX" "$ANDROID_HOME" "$TMP"
  printf 'build-tools=%s\n' "${BUILD_TOOLS:-<ninguna>}"
  printf 'java=%s\n' "$(command -v java || echo '<no>')"
  if j=$(jar_del_sdk 34); then printf 'android.jar=%s\n' "$j"; else printf 'android.jar=<falta; tools/entorno.sh jar>\n'; fi
  printf 'apkc=%s\n' "${APKC:-$TMP/apkc-gate}"
  printf 'apkc-wasm=%s\n' "${APKC_WASM:-$TMP/apkc-wasm.wasm}"
  [ -d "$ANDROID_HOME/build-tools/34.0.0" ] && printf 'build-tools 34.0.0=%s\n' "$ANDROID_HOME/build-tools/34.0.0" || printf 'FALTA build-tools 34.0.0 en %s\n' "$ANDROID_HOME"
  zipcheck=$APKC_HARNESS/zipcheck.py
  [ -f "$zipcheck" ] && printf 'ok %s\n' "$zipcheck" || printf 'FALTA %s (tools/entorno.sh harness)\n' "$zipcheck"
}

tarea=${1:-doctor}
[ $# -gt 0 ] && shift || true

case "$tarea" in
  test)
    materializar_emulador
    ( cd "$RAIZ/apkc" && tcr_o_directo go test -count=1 ./... )
    ;;
  wasm)
    compila_wasm
    # El fs simulado nace vacío: sin montar el android.jar y el fixture del layout, las pruebas que
    # los abren fallan con ENOENT (`sin android.jar que abrir`, `open testdata/...`) aunque el
    # código esté bien. El jar se monta con la forma del SDK (platforms/android-34/android.jar bajo
    # el ANDROID_HOME que el hospedador ya publica) porque `check` y el enlazador deducen la API del
    # nombre del directorio padre: en /sdk/android.jar suelto piden --sdk-version y fallan. El
    # destino va absoluto porque --montar también expande el destino. El origen sale de jar_del_sdk,
    # la única definición de «dónde está» que comparten doctor y la tarea jar.
    MONTAJES=
    if JAR_WASM=$(jar_del_sdk 34); then
      MONTAJES="--montar $JAR_WASM:/android/platforms/android-34/android.jar"
    else
      printf 'aviso: sin android.jar; las pruebas contra el SDK fallarán en wasm\n' >&2
    fi
    ( cd "$RAIZ/shim" && node navegador-sim.js $MONTAJES \
        --montar "$RAIZ/apkc/testdata/vista-propia/main.xml:/testdata/vista-propia/main.xml" \
        "$TMP/apkc-test.wasm" )
    ;;
  harness)
    if [ ! -f "$APKC_HARNESS/zipcheck.py" ]; then
      printf 'clonando apkc-harness en %s\n' "$APKC_HARNESS" >&2
      git clone --depth 1 https://github.com/Leonisaurov/apkc-harness.git "$APKC_HARNESS" >&2
    fi
    printf '%s\n' "$APKC_HARNESS/zipcheck.py"
    ;;
  wisp)
    # Falsificación de los dos gates del módulo WISP con la regla real (studio/wisp-gates.js):
    # sin navegador y sin red, comprueba que rechazan lo que tienen que rechazar.
    ( cd "$RAIZ" && node studio/prueba-wisp-gates.mjs )
    ;;
  wisp-externo)
    # El gate del modo externo abre relays de terceros: nunca por defecto. Sin la variable a 1 se
    # dice el motivo y se sale 2, para que «omitido» no se confunda con un verde.
    if [ "${APKC_WISP_EXTERNO:-}" != 1 ]; then
      printf 'gate externo omitido: fija APKC_WISP_EXTERNO=1 para abrir relays de terceros\n' >&2
      exit 2
    fi
    # Sin compilar nada nuevo: usa el wasm y el fixture que ya dejó studio/e2e.sh o los que el
    # usuario pase por entorno (las mismas variables que prueba-pestana.js).
    APK_FIXTURE=$(find "$TMP/studio-e2e-java" -name app.apk -path "*/build/*" 2>/dev/null | head -1 || true)
    CHROME=$(command -v chromium || command -v chromium-browser || command -v google-chrome || true)
    ( cd "$RAIZ/studio" && \
      APKC_WISP_EXTERNO=1 CHROME_BIN="${CHROME_BIN:-$CHROME}" \
      APKC_WASM="${APKC_WASM:-$TMP/apkc-wasm.wasm}" \
      APK_EJEMPLO="${APK_EJEMPLO:-$APK_FIXTURE}" \
      KS_EJEMPLO="${KS_EJEMPLO:-$HOME/.android/debug.keystore}" \
      ANDROID_JAR="${ANDROID_JAR:-$(jar_del_sdk 34 || true)}" \
      node prueba-pestana.js )
    ;;
  gate)
    construye_binarios
    # SDK espejo: apkc toma la build-tools mas alta, asi que sin esto un escritorio con 36.0.0
    # pidria android-36 aunque el proyecto mida 34 (tools/sdk34.sh).
    SDK34=$(sh "$RAIZ/tools/sdk34.sh" "$TMP/apkc-sdk34")
    JAVA17=""
    for j in "$HOME/opt/jdk17" /usr/lib/jvm/java-17-openjdk-amd64 /usr/lib/jvm/java-17-openjdk; do
      [ -x "$j/bin/javac" ] && { JAVA17=$j; break; }
    done
    (
      cd "$RAIZ/studio"
      export ANDROID_HOME="$SDK34"
      if [ -n "$JAVA17" ]; then export JAVA_HOME="$JAVA17"; fi
      export APKC=${APKC:-$TMP/apkc-gate} APKC_WASM=${APKC_WASM:-$TMP/apkc-wasm.wasm}
      sh e2e.sh
    )
    ;;
  sitio)
    construye_wasm
    APKC_WASM=${APKC_WASM:-$TMP/apkc-wasm.wasm} sh "$RAIZ/tools/sitio.sh" "${1:-$RAIZ/sitio}"
    ;;
  publicar)
    command -v surge >/dev/null 2>&1 || { printf 'falta el CLI de surge\n' >&2; exit 2; }
    construye_wasm
    APKC_WASM=${APKC_WASM:-$TMP/apkc-wasm.wasm} sh "$RAIZ/tools/sitio.sh" "$RAIZ/sitio"
    DOMINIO=$(cat "$RAIZ/.surge-domain" 2>/dev/null || cat "$RAIZ/sitio/CNAME")
    printf 'publicando en https://%s/\n' "$DOMINIO"
    ( cd "$RAIZ/sitio" && surge . publish )
    # Antes de dar nada por bueno: el gz servido debe ser byte a byte el empaquetado. Si el host
    # devuelve un cuerpo truncado, falla aquí y no se culpa al navegador.
    sh "$RAIZ/tools/verificar-publicacion.sh" "https://$DOMINIO/" "$RAIZ/sitio/apkc.wasm.gz"
    # Verificación sobre el host real, con los mismos oráculos que el gate.
    JAR=$(jar_del_sdk 34 || true)
    APKSIGNER=$ANDROID_HOME/build-tools/34.0.0/apksigner
    [ -x "$APKSIGNER" ] || APKSIGNER=$(command -v apksigner || true)
    CHROME=$(command -v chromium || command -v chromium-browser || command -v google-chrome || true)
    SITIO_URL="https://$DOMINIO/" PREFIX="$PREFIX" CHROME_BIN="$CHROME" \
      ANDROID_JAR="$JAR" APKSIGNER="$APKSIGNER" SITIO_APK="$TMP/sitio-publicado.apk" \
      node "$RAIZ/studio/prueba-sitio.js"
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
  corredor)
    printf 'RAIZ=%s\nHOME=%s\nPREFIX=%s\nTMP=%s\n' "$RAIZ" "$HOME" "$PREFIX" "$TMP"
    printf 'TCR=%s\nGUARDIA_MB=%s\nLD_LIBRARY_PATH=%s\n' \
      "${TCR:-<sin tcr>}" "${CORREDOR_GUARDIA_MB:-300}" "${LD_LIBRARY_PATH:-<vacio>}"
    ;;
  tareas|ayuda)
    cat <<'AYUDA'
Corredor de pruebas de apkc-web. Elige una tarea; el entorno ya está fijado.

  sh tools/correr.sh test              suite Go nativa de apkc/
  sh tools/correr.sh wasm              suite en el hospedador con forma de navegador
  sh tools/correr.sh wisp              falsificación de los gates del módulo WISP
  sh tools/correr.sh wisp-externo      gate externo (solo con APKC_WISP_EXTERNO=1)
  sh tools/correr.sh gate              página de punta a punta: SOLO si el usuario lo pide
  sh tools/correr.sh banco             los cinco caminos de ejecución
  sh tools/correr.sh jar [api]         ruta del android.jar local
  sh tools/correr.sh harness           clona el oráculo de zip si falta
  sh tools/correr.sh sitio             empaqueta el sitio estático
  sh tools/correr.sh publicar          empaqueta, publica y verifica sobre el host
  sh tools/correr.sh doctor            entorno resuelto
  sh tools/correr.sh corredor          envoltorio de tcr y librería de chromium
  sh tools/correr.sh -- <mando>        ejecuta un mando suelto con el entorno puesto

Opciones: CORREDOR_GUARDIA_MB cambia la guardia de RAM de tcr (por defecto 300),
SHIMLIB apunta a otra librería de chromium y TCR=<ruta> sustituye el envoltorio.
AYUDA
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
