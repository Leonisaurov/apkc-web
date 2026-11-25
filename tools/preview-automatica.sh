#!/bin/sh
# Vigilante de la vista previa local: mantiene apkc.wasm sirviendo SIEMPRE el HEAD del repo.
# El servidor (studio/servidor.js) lee el fichero con readFileSync en cada peticion, asi que
# basta con reemplazarlo atomicamente con mv -f (lo que ya hace tools/preview-local.sh).
#
# Uso: sh tools/preview-automatica.sh &
# Estado: $HOME/.cache/apkc-web/preview-automatica/ (hash construido y log).
set -eu

HOME_DIR=${HOME:-/home/leonisauro}
REPO=/home/leonisauro/Develop/Go/apkc-web
DEST="$HOME_DIR/.cache/apkc-web/preview/apkc.wasm"
BASE="$HOME_DIR/.cache/apkc-web/preview-automatica"
ESTADO="$BASE/head-construido"
LOG="$BASE/vigilante.log"
DEBOUNCE=8      # espera a que termine la rafaga de commits
SONDEO=4        # frecuencia de comprobacion de HEAD

mkdir -p "$BASE"
echo "[$(date -u +%FT%TZ)] vigilante iniciado; destino=$DEST" >> "$LOG"

while :; do
  head_actual=$(git -C "$REPO" rev-parse HEAD 2>/dev/null || echo "")
  construido=$(cat "$ESTADO" 2>/dev/null || echo "")
  if [ -n "$head_actual" ] && [ "$head_actual" != "$construido" ]; then
    sleep "$DEBOUNCE"
    head_estable=$(git -C "$REPO" rev-parse HEAD 2>/dev/null || echo "")
    if [ "$head_estable" != "$construido" ] && [ -n "$head_estable" ]; then
      TEMPORAL=$(mktemp -d "$BASE/build.XXXXXX")
      corto=$(echo "$head_estable" | cut -c1-8)
      echo "[$(date -u +%FT%TZ)] HEAD $corto: reconstruyendo wasm" >> "$LOG"
      # El archive de apkc/ no incluye los preview*.go generados: el submódulo se extrae aparte,
      # en el commit exacto que fija el gitlink de HEAD, y preview-local.sh los va a materializar.
      subsha=$(git -C "$REPO" ls-tree "$head_estable" apkc-emulador | awk '{print $3}' 2>>"$LOG")
      mkdir -p "$TEMPORAL/apkc-emulador"
      if git -C "$REPO" archive "$head_estable" apkc -o "$TEMPORAL/apkc.tar" 2>>"$LOG" \
         && tar -x -f "$TEMPORAL/apkc.tar" -C "$TEMPORAL" 2>>"$LOG" \
         && [ -n "$subsha" ] \
         && git -C "$REPO/apkc-emulador" archive "$subsha" -o "$TEMPORAL/emulador.tar" 2>>"$LOG" \
         && tar -x -f "$TEMPORAL/emulador.tar" -C "$TEMPORAL/apkc-emulador" 2>>"$LOG" \
         && APKC_PREVIEW_WASM="$DEST" HOME="$HOME_DIR" \
            tcr -c 2 -m 2G -r 256M -- sh "$REPO/tools/preview-local.sh" "$TEMPORAL" >>"$LOG" 2>&1; then
        # Revalida HEAD tras el mv atomico: si hubo un commit durante el build, NO escribas
        # el estado (así el siguiente ciclo vuelve a construir ese HEAD) y sirve solo el nuevo.
        head_tras=$(git -C "$REPO" rev-parse HEAD 2>/dev/null || echo "")
        if [ "$head_tras" = "$head_estable" ]; then
          printf '%s\n' "$head_estable" > "$ESTADO"
          echo "[$(date -u +%FT%TZ)] HEAD $corto: wasm actualizado" >> "$LOG"
        else
          # Caso ABA: si HEAD volvio al valor que ya estaba en ESTADO, dejarlo tal cual haria que el
          # siguiente ciclo no reconstruya y sirva B para siempre. Vacia el estado: siempre se reconstruye.
          : > "$ESTADO"
          echo "[$(date -u +%FT%TZ)] HEAD $corto: HEAD cambio a $(echo "$head_tras" | cut -c1-8) durante el build; " \
               "wasm servido, estado vaciado para reconstruir" >> "$LOG"
        fi
      else
        echo "[$(date -u +%FT%TZ)] HEAD $corto: FALLO al reconstruir; se reintentara" >> "$LOG"
      fi
      rm -rf "$TEMPORAL"
    fi
  fi
  sleep "$SONDEO"
done
