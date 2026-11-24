#!/bin/sh
# correr.sh — único punto de entrada para correr pruebas en este repo.
#
# Todo el entorno (HOME, PREFIX, SDK, envoltorio de tcr con guardia de RAM y la
# librería suelta de chromium) vive en `tools/entorno.sh`. Aquí solo se elige la
# tarea; no hay variables que copiar a mano en cada llamada.
#
#   sh tools/correr.sh tareas          # lista las tareas
#   sh tools/correr.sh test            # suite Go
#   sh tools/correr.sh wasm            # suite en el hospedador
#   sh tools/correr.sh gate            # SOLO si el usuario lo pide
#   sh tools/correr.sh -- go test ./...  # mando suelto con el entorno puesto
set -eu

RAIZ=$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)
exec sh "$RAIZ/tools/entorno.sh" "$@"
