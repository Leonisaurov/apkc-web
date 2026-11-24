#!/bin/sh
# gate-local.sh — un solo mando para el gate de punta a punta en un escritorio Linux.
#
# El gate completo necesita tres cosas que no son evidentes y que cada agente vuelve a buscar:
# el HOME del repo (lo fija entorno.sh), un envoltorio de tcr con el guardia de RAM bajado
# (el de 1 GiB mata el gate en este portátil) y la librería de chromium que no está en el PATH.
# Todo eso se resuelve aqui; el agente solo llama a `sh tools/gate-local.sh`.
#
# Uso:
#   sh tools/gate-local.sh        # corre el gate; SOLO cuando el usuario lo pida
#   sh tools/gate-local.sh env    # imprime el entorno resuelto sin correr nada
#
# Se puede sobreescribir SHIMLIB si la librería de chromium vive en otro sitio.
set -eu

RAIZ=$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)
HOME_DIR=${HOME_APKC_WEB:-$(CDPATH= cd -P -- "$RAIZ/../../.." && pwd)}
PREFIX=${PREFIX:-${XDG_CACHE_HOME:-$HOME_DIR/.cache}/apkc-web}
BASE="$PREFIX/gate-local"
mkdir -p "$BASE"

# e2e.sh invoca `"$TCR" -j 1 -q -- <mando>`. Se descartan esos cuatro argumentos para no
# duplicar los de tcr, y se fija el guardia de RAM en 300 MiB: con el de 1 GiB el gate
# muere con 137 en cuanto el compilador de Java aprieta.
WRAP="$BASE/tcr"
cat > "$WRAP" <<'WRAP_FIN'
#!/bin/sh
if [ "${1:-}" = "-j" ] && [ "${3:-}" = "-q" ] && [ "${4:-}" = "--" ]; then
  shift 4
fi
exec tcr -r 300M -j 1 -q -- "$@"
WRAP_FIN
chmod +x "$WRAP"

# El chromium del sistema necesita libsimdutf.so.36, que vive suelto fuera del ldconfig.
# Sin esta variable el debugger no abre y e2e.sh lo reporta como "puerto ocupado".
SHIMLIB=${SHIMLIB:-$HOME_DIR/.cache/apkc-web-gate-final/shimlib}
LD_LIBRARY_PATH=${LD_LIBRARY_PATH:-}
if [ -d "$SHIMLIB" ]; then
  case ":$LD_LIBRARY_PATH:" in
    *":$SHIMLIB:"*) ;;
    *) LD_LIBRARY_PATH=${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$SHIMLIB ;;
  esac
  export LD_LIBRARY_PATH
else
  printf 'aviso: sin %s; chromium podria no abrir el debugger\n' "$SHIMLIB" >&2
fi

if [ "${1:-}" = "env" ]; then
  printf 'RAIZ=%s\nHOME=%s\nTCR=%s\nSHIMLIB=%s\nLD_LIBRARY_PATH=%s\n' \
    "$RAIZ" "$HOME_DIR" "$WRAP" "$SHIMLIB" "${LD_LIBRARY_PATH:-<vacio>}"
  exit 0
fi

printf 'gate: TCR=%s, LD_LIBRARY_PATH=%s\n' "$WRAP" "${LD_LIBRARY_PATH:-<vacio>}"
TCR="$WRAP" exec sh "$RAIZ/tools/entorno.sh" gate
