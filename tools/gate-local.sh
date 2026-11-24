#!/bin/sh
# Compatibilidad: el corredor real es `tools/correr.sh`, que ya fija el entorno.
# Este alias queda para los mandos guardados que todavía llaman a gate-local.sh.
set -eu

RAIZ=$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)
exec sh "$RAIZ/tools/correr.sh" gate "$@"
