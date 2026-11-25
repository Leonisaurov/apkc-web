# 25 — Un solo emulador

## 357. Se retira el pin espejo del padre: `studio/apkc-emulador` es el único (2026-11-24)

Decisión del usuario: «solo quiero que haya un apkc-emulador y es el que está dentro de
apkc-studio». El gitlink `apkc-emulador` del padre se elimina; subsiste solo el gitlink anidado de
`studio`, fijado en `563e991`. No se toca `studio/.gitmodules`, su gitlink ni
`studio/apkc-emulador`. Sin gate, CDP, red, publicación (`surge`) ni cambios en el emulador.

**F0 — precondición.** El worktree asignado arrancó con ambos submódulos sin inicializar
(`-` en `git submodule status`, directorios vacíos). Se materializó `studio` y su anidado desde el
espejo local ya configurado, sin red:

```sh
git -c protocol.file.allow=always submodule update --init --recursive studio
# Cloning into '.../studio'... done.
# Submodule path 'studio': checked out 'e30d3ad329f60694c83a21a0b0c8edeb0939815e'
# Submodule 'apkc-emulador' (https://github.com/Leonisaurov/apkc-emulador.git) registered for path 'studio/apkc-emulador'
# Cloning into '.../studio/apkc-emulador'... done.
# Submodule path 'studio/apkc-emulador': checked out '563e991de1fac3c863e5278483c2ff745d57bbb2'
git -C studio ls-tree HEAD apkc-emulador
# 160000 commit 563e991de1fac3c863e5278483c2ff745d57bbb2	apkc-emulador
git -C studio submodule status
#  563e991de1fac3c863e5278483c2ff745d57bbb2 apkc-emulador (heads/emulador-modulo)
```

**F1 — baja del gitlink.** `git rm` retira el gitlink y la sección `[submodule "apkc-emulador"]`
de `.gitmodules` del padre en el mismo paso; el `AGENTS.md` pierde la fila del mapa y la regla 14
pasa a vigilar solo el gitlink anidado de `studio` (guardia de un solo gitlink, ya sin doble pin).

```sh
git rm -f apkc-emulador
# rm 'apkc-emulador'
cat .gitmodules
# [submodule "studio"]
# 	path = studio
# 	url = https://github.com/Leonisaurov/apkc-studio.git
git status -sb
# ## HEAD (no branch)
# M  .gitmodules
#  M AGENTS.md
# D  apkc-emulador
#  M tools/entorno.sh
#  M tools/materializar-emulador.sh
#  M tools/preview-automatica.sh
git diff --check
# (sin salida)
```

**F2 — resolvedores sin camino alternativo.** `tools/materializar-emulador.sh` resuelve en este
orden el argumento `$2`, `APKC_EMULADOR_DIR` y `$RAIZ/studio/apkc-emulador`; se eliminó la última
rama que caía a `$RAIZ/apkc-emulador`. El bucle `cp`+`cmp` de 34 ficheros y el fallo si falta uno
siguen intactos (§351). `tools/preview-local.sh` ya tomaba `studio/apkc-emulador` por defecto y no
cambia. `tools/entorno.sh` solo pierde el comentario del pin espejo y sigue pasando el override.
`tools/preview-automatica.sh` ya no consulta `git ls-tree … apkc-emulador` del padre: si el gitlink
anidado de `studio` falta, registra el motivo y reintenta en vez de construir desde otro árbol.
Los nueve guiones de `banco/` no se tocan: los nueve pasan un solo argumento (la raíz) al mismo
resolvedor, así que heredan la fuente única sin caer al pin retirado.

```sh
sh -n tools/materializar-emulador.sh tools/preview-local.sh tools/entorno.sh tools/preview-automatica.sh
# (sin salida)
grep -rn '\$RAIZ/apkc-emulador\|\$REPO/apkc-emulador\|\$AQUI/apkc-emulador' tools/ banco/
# (sin coincidencias)
sh tools/materializar-emulador.sh
# materializados 34 ficheros preview*.go de .../quitar-pin-emulador-padre/studio/apkc-emulador en .../quitar-pin-emulador-padre/apkc
n=0; for f in studio/apkc-emulador/preview*.go; do cmp "$f" "apkc/$(basename "$f")" || exit 1; n=$((n+1)); done; echo "cmp idénticos: $n"
# cmp idénticos: 34
APKC_EMULADOR_DIR=$PWD/studio/apkc-emulador sh tools/materializar-emulador.sh
# materializados 34 ficheros preview*.go de .../quitar-pin-emulador-padre/studio/apkc-emulador en .../quitar-pin-emulador-padre/apkc
ls -d apkc-emulador
# (no hay apkc-emulador en el padre)
```

**F3 — capas Go y wasm (regla 8), sobre el árbol con el pin ya retirado.** Toolchain fijada
(§312) y `GOTMPDIR` en disco:

```sh
sh tools/wasm-exec-hash.sh
# wasm_exec: sha256 0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14 (go go1.24.0) OK
env PATH=/home/leonisauro/sdk/go1.24.0/bin:/home/leonisauro/opt/jdk21/bin:$PATH \
  GOTMPDIR=/home/leonisauro/.cache/apkc-web/gotmp \
  tcr -c 2 -m 2G -r 256M -- go -C apkc test -count=1 ./...
# ok  	github.com/ajinasokan/apkc	264.994s
env HOME=/home/leonisauro PATH=/home/leonisauro/sdk/go1.24.0/bin:/home/leonisauro/opt/jdk21/bin:$PATH \
  GOTMPDIR=/home/leonisauro/.cache/apkc-web/gotmp GOOS=js GOARCH=wasm CGO_ENABLED=0 \
  tcr -c 2 -m 2G -r 256M -- go -C apkc build -ldflags='-s -w' \
  -o /home/leonisauro/.cache/apkc-web/preview-emulador.wasm .
# (sin salida)
sha256sum /home/leonisauro/.cache/apkc-web/preview-emulador.wasm
# 8019508fe085eff8d786be0ede54429ed790de47c5b318557f44bba65e9e88c9
node shim/navegador-sim.js \
  --montar /home/leonisauro/.cache/apkc-web/tmp/studio-e2e-contador/com.hola.contador/build/app.apk:/app.apk \
  /home/leonisauro/.cache/apkc-web/preview-emulador.wasm preview /app.apk > salida.json
# [apkc] finished in 60.600832ms
grep -o '"pasos":[0-9]*' salida.json
# "pasos":22
grep -c 'ejecucion.error' salida.json
# 0
```

La pestaña sigue en orden y `studio` no se tocó:

```sh
cd studio && node prueba.js
# reglas: 24 | ancladas al fuente de apkc o a una salida capturada: 21 | sin captura propia: 3
# prueba: todo en orden
git -C studio status -sb
# ## HEAD (no branch)
git -C studio rev-parse HEAD:apkc-emulador
# 563e991de1fac3c863e5278483c2ff745d57bbb2
```

**Frontera y deuda.** No se corrió gate, CDP, `prueba-pestana.js`, `prueba-sitio.js` ni
publicación; el vigilante en vivo sigue sin ciclo real de reconstrucción, así que su rama queda
verificada solo por `sh -n` y por la lectura del gitlink (`incierto`). El único emulador es
`studio/apkc-emulador`; el padre ya no ofrece segundo camino ni guardia de doble gitlink. El push
del padre y, en su caso, el de `studio` se ejecutan autorizados por el usuario al cerrar el frente,
sin tocar el repo `apkc-emulador` (ya empujado).
