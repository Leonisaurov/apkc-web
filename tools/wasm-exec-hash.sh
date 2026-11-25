#!/bin/sh
# wasm-exec-hash.sh — único sitio de verdad para la versión de Go y la suma de wasm_exec.js.
#
# Se invoca antes de cada `cp` que saca wasm_exec.js del GOROOT (entorno.sh y sitio.sh; W1 cubre
# gate.yml). Si la toolchain o el fichero no son los fijados aquí, aborta con el motivo, con el
# mismo patrón que tools/sitio.sh:132-140. Así el JS nunca se copia a ciegas del GOROOT ambiental.
# DEUDA ABIERTA, no verde: studio/servidor.js sirve /wasm_exec.js leyendo del GOROOT en cada
# petición y sigue sin pasar por este verificador. Está fuera del alcance de W2 (no se toca studio/).
#
# Uso:
#   sh tools/wasm-exec-hash.sh                  # valida y escribe la suma en stdout
#   .  tools/wasm-exec-hash.sh                  # además exporta WASM_EXEC_ORIGEN para el cp
#
# No versiona el JS (regla 2) y no añade verificación al cliente (regla 4): solo gobierna a quien
# empaqueta o corre las pruebas en este equipo.
#
# ESTADO (W2, cerrado con medicion real): go1.24.0 se instala fuera del sistema, en
# $HOME/sdk/go1.24.0, con el tarball oficial de go.dev; la suma de su wasm_exec.js se midio con
# sha256sum y consta en SUMA_FIJADA. El unico override de operacion es APKC_GOROOT; en CI, donde
# no existe $HOME/sdk, W1 debe fijarlo al GOROOT que instale setup-go antes de invocar esto.
# APKC_GO_FIJADO y APKC_WASM_EXEC_SHA256 solo surten en falsificaciones, con APKC_PRUEBA=1; sin
# esa marca se ignoran y mandan los valores canonicos de arriba.
set -eu

GO_FIJADO=go1.24.0
SUMA_FIJADA=0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14
if [ "${APKC_PRUEBA:-}" = 1 ]; then
  GO_FIJADO=${APKC_GO_FIJADO:-$GO_FIJADO}
  SUMA_FIJADA=${APKC_WASM_EXEC_SHA256:-$SUMA_FIJADA}
fi
# APKC_GOROOT manda siempre (override deliberado; entorno.sh lo exporta en worktrees). Si no hay
# override, un GOROOT del entorno solo se acepta si su go es exactamente el fijado: así se rechaza
# /usr/lib/go (1.27.4) pero se acepta el que entorno.sh exporta desde el HOME real.
if [ -n "${APKC_GOROOT:-}" ]; then
  GOROOT_FIJADO=$APKC_GOROOT
elif [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/go" ] &&
     [ "$("$GOROOT/bin/go" version | awk '{print $3}')" = "$GO_FIJADO" ]; then
  GOROOT_FIJADO=$GOROOT
else
  GOROOT_FIJADO=$HOME/sdk/go1.24.0
  if [ -n "${GOROOT:-}" ] && [ "$GOROOT" != "$GOROOT_FIJADO" ]; then
    printf 'wasm_exec: GOROOT del entorno %s no es la toolchain fijada %s\n' "$GOROOT" "$GOROOT_FIJADO" >&2
    printf 'use APKC_GOROOT para fijar otro GOROOT deliberadamente\n' >&2
    exit 1
  fi
fi

# Compartido por las dos ramas de aborto: quien vea el motivo debe poder cerrar el hueco sin
# tener que abrir este fichero a buscar los comandos.
comandos_de_registro() {
  printf 'comandos para registrar los dos valores en este fichero:\n' >&2
  printf '  GOTOOLCHAIN=%s go version\n' "$GO_FIJADO" >&2
  printf '  sha256sum "$(GOTOOLCHAIN=%s go env GOROOT)/lib/wasm/wasm_exec.js"\n' "$GO_FIJADO" >&2
}

GOROOT_ACTIVO=$GOROOT_FIJADO

# La version la valida siempre la toolchain instalada: una copia falsificada del JS puede no tener
# bin/go y no debe impedir que se detecte el cambio de suma.
TOOLCHAIN_GO=$GOROOT_FIJADO
if [ ! -x "$TOOLCHAIN_GO/bin/go" ]; then
  TOOLCHAIN_GO=$HOME/sdk/go1.24.0
fi
if [ ! -x "$TOOLCHAIN_GO/bin/go" ]; then
  printf 'wasm_exec: falta la toolchain fijada en %s\n' "$TOOLCHAIN_GO" >&2
  printf 'comando: curl -fsSL https://go.dev/dl/go1.24.0.linux-amd64.tar.gz | tar -C "$HOME/sdk" -xz\n' >&2
  exit 1
fi
VERSION_ACTIVA=$("$TOOLCHAIN_GO/bin/go" version | awk '{print $3}')

if [ "$VERSION_ACTIVA" != "$GO_FIJADO" ]; then
  printf 'wasm_exec: versión de Go esperada %s, hay %s\n' "$GO_FIJADO" "$VERSION_ACTIVA" >&2
  printf 'comando: %s/bin/go version\n' "$TOOLCHAIN_GO" >&2
  comandos_de_registro
  exit 1
fi

if [ -z "$SUMA_FIJADA" ]; then
  printf 'wasm_exec: suma SHA-256 de wasm_exec.js de %s sin registrar\n' "$GO_FIJADO" >&2
  comandos_de_registro
  exit 1
fi

ORIGEN=$GOROOT_ACTIVO/lib/wasm/wasm_exec.js
if [ ! -f "$ORIGEN" ]; then
  printf 'wasm_exec: no existe %s\n' "$ORIGEN" >&2
  printf 'comando: ls "%s"\n' "$ORIGEN" >&2
  exit 1
fi

SUMA_ACTIVA=$(sha256sum "$ORIGEN" | awk '{print $1}')
if [ "$SUMA_ACTIVA" != "$SUMA_FIJADA" ]; then
  printf 'wasm_exec: suma esperada %s, hay %s (%s)\n' "$SUMA_FIJADA" "$SUMA_ACTIVA" "$ORIGEN" >&2
  printf 'comando: sha256sum "%s"\n' "$ORIGEN" >&2
  exit 1
fi

WASM_EXEC_ORIGEN=$ORIGEN
export WASM_EXEC_ORIGEN
printf 'wasm_exec: sha256 %s (go %s) OK\n' "$SUMA_ACTIVA" "$GO_FIJADO"
