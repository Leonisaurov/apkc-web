# 24 — El emulador pasa a ser submódulo de `studio`

## 356. Cambio de propietario del gitlink, sin mover una línea del intérprete (2026-11-24)

Decisión: la propiedad git de `apkc-emulador` la toma **`studio/`**, que es el producto que publica
la página. El padre conserva su gitlink como **pin espejo**: `banco/` (nueve guiones), el vigilante
y el binario nativo siguen leyendo de ahí; borrarlo obligaría a todos a buscar dentro de un
submódulo anidado sin ganar propiedad. Los dos gitlinks apuntan a `563e991` y no se permite que
diverjan. Sin push, publicación, gate, CDP ni red.

**F0 — precondición.** El worktree asignado arrancó con los dos submódulos sin inicializar (directorios
vacos y `-` en `git submodule status`), así que se materializaron desde los espejos locales ya
configurados, sin red:

```sh
git -c protocol.file.allow=always submodule update --init studio apkc-emulador
# Submodule path 'apkc-emulador': checked out '563e991de1fac3c863e5278483c2ff745d57bbb2'
# Submodule path 'studio': checked out '7d1d09896073773f37d5f1de678d85d5c22dfaa1'
git ls-remote /home/leonisauro/.cache/apkc-web/emulador-origen refs/heads/emulador-modulo
# 563e991de1fac3c863e5278483c2ff745d57bbb2	refs/heads/emulador-modulo
git -C apkc-emulador cat-file -e 563e991de1fac3c863e5278483c2ff745d57bbb2^{commit} && echo OK
# OK
git ls-tree HEAD apkc-emulador studio
# 160000 commit 563e991de1fac3c863e5278483c2ff745d57bbb2	apkc-emulador
# 160000 commit 7d1d09896073773f37d5f1de678d85d5c22dfaa1	studio
```

**F1 — alta dentro de `studio` (commit `0f6d802`, no empujado).** La URL `https` se escribe en
`.gitmodules`; el espejo local queda solo en `.git/config`, no se versiona. El SHA se fija con
`checkout --detach`, sin reescribir la historia pública del emulador:

```sh
git -C studio -c protocol.file.allow=always submodule add -b emulador-modulo \
  --name apkc-emulador /home/leonisauro/.cache/apkc-web/emulador-origen apkc-emulador
git -C studio submodule set-url apkc-emulador https://github.com/Leonisaurov/apkc-emulador.git
git -C studio/apkc-emulador checkout --detach 563e991de1fac3c863e5278483c2ff745d57bbb2
git -C studio add .gitmodules apkc-emulador
git -C studio commit -m 'feat(emulador): el emulador pasa a ser submódulo de studio'
# [detached HEAD 0f6d802] feat(emulador): el emulador pasa a ser submódulo de studio
git -C studio ls-tree HEAD apkc-emulador
# 160000 commit 563e991de1fac3c863e5278483c2ff745d57bbb2	apkc-emulador
git -C studio submodule status
#  563e991de1fac3c863e5278483c2ff745d57bbb2 apkc-emulador (heads/emulador-modulo)
cat studio/.gitmodules
# [submodule "apkc-emulador"]
# 	path = apkc-emulador
# 	url = https://github.com/Leonisaurov/apkc-emulador.git
# 	branch = emulador-modulo
git -C studio status --porcelain
# (sin salida)
ls studio/apkc-emulador/preview*.go | wc -l
# 34
```

**F2 — fuente canónica (commit `586f466`).** `tools/materializar-emulador.sh` resuelve, en este
orden, el argumento `$2`, `APKC_EMULADOR_DIR`, `$RAIZ/studio/apkc-emulador` y
`$RAIZ/apkc-emulador`; el bucle `cp`+`cmp` de 34 ficheros y el fallo si faltase uno siguen intactos
(§351). `preview-local.sh` toma `studio/apkc-emulador` por defecto; `entorno.sh` pasa el override;
`preview-automatica.sh` saca `estudio_sha` del gitlink de `studio`, `subsha` del gitlink anidado y
extrae el contenido con `git -C "$REPO/studio/apkc-emulador" archive "$subsha"` en el temporal,
con caída al camino antiguo del padre si no hay gitlink anidado. Los nueve guiones de `banco/`
no se tocan: el `grep` de §351 sigue dando las nueve llamadas al único resolvedor.

```sh
sh tools/materializar-emulador.sh
# materializados 34 ficheros preview*.go de .../studio-emulador-f0-f4/studio/apkc-emulador en .../studio-emulador-f0-f4/apkc
for f in studio/apkc-emulador/preview*.go; do cmp "$f" "apkc/$(basename "$f")" || exit 1; done
# (sin salida)
APKC_EMULADOR_DIR=$PWD/apkc-emulador sh tools/materializar-emulador.sh
# materializados 34 ficheros preview*.go de .../studio-emulador-f0-f4/apkc-emulador en .../studio-emulador-f0-f4/apkc
sh -n tools/materializar-emulador.sh tools/preview-local.sh tools/entorno.sh tools/preview-automatica.sh
# (sin salida)
grep -R -n 'materializar-emulador' banco/
# banco/incremental.sh:31, incremental-wasm.sh:54, art-verificador/art-verificador.sh:58,
# apps-ejemplo/construir.sh:17, apps-ejemplo/endurecer.sh:29, aab-multidex/aab-multidex.sh:77,
# validar-apk/validar.sh:47, art-jar-real/art-jar-real.sh:85, deps-ejemplo/construir.sh:31
# (siguen pasando un solo argumento: la raíz; el resolvedor elige la fuente)
```

Guardia de gitlinks, ejecutada sobre el árbol final de la fase:

```sh
P=/home/leonisauro/.local/share/opencode/worktree/c6f089/studio-emulador-f0-f4
# padre:apkc-emulador=563e991de1fac3c863e5278483c2ff745d57bbb2
# studio:apkc-emulador=563e991de1fac3c863e5278483c2ff745d57bbb2
# studio:HEAD=0f6d802d5c2b16230e8b8152cff638e3b4356acd
# padre:studio=0f6d802d5c2b16230e8b8152cff638e3b4356acd
# GUARDIA-OK
```

Capa Go completa y capa wasm (regla 8). El primer intento de la suite falló por un PATH propio
que no incluía `java`: `bundletool validate falló: exec: "java": executable file not found in $PATH`
y `apksigner verify falló: exit status 127`. Con el PATH que fija el repo (Go 1.24.0 y JDK 21) y
`GOTMPDIR` en disco:

```sh
env PATH=/home/leonisauro/sdk/go1.24.0/bin:/home/leonisauro/.local/bin:$PATH \
  GOTMPDIR=/home/leonisauro/.cache/apkc-web/gotmp \
  tcr -c 2 -m 2G -r 256M -- go -C apkc test -count=1 ./...
# ok  	github.com/ajinasokan/apkc	248.739s
env HOME=/home/leonisauro PATH=/home/leonisauro/sdk/go1.24.0/bin:/home/leonisauro/.local/bin:$PATH \
  GOTMPDIR=/home/leonisauro/.cache/apkc-web/gotmp GOOS=js GOARCH=wasm CGO_ENABLED=0 \
  tcr -c 2 -m 2G -r 256M -- go -C apkc build -ldflags='-s -w' \
  -o /home/leonisauro/.cache/apkc-web/preview-emulador.wasm .
sha256sum /home/leonisauro/.cache/apkc-web/preview-emulador.wasm
# 1c2f521b1998cce4052f3510d9d76f71c4a257b423be005874ebe230952503f5
node shim/navegador-sim.js \
  --montar /home/leonisauro/.cache/apkc-web/tmp/studio-e2e-contador/com.hola.contador/build/app.apk:/app.apk \
  /home/leonisauro/.cache/apkc-web/preview-emulador.wasm preview /app.apk > salida.json
# [apkc] finished in 58.838016ms
grep -q '"pasos":22' salida.json && ! grep -q 'ejecucion.error' salida.json && echo FALSACION-OK
# FALSACION-OK
cd studio && node prueba.js
# reglas: 24 | ancladas al fuente de apkc o a una salida capturada: 21 | sin captura propia: 3
# prueba: todo en orden
```

**F3 — empaquetado (commit `70e50be`).** `git archive HEAD` del submódulo `studio` no aporta
fuentes del gitlink anidado: crea `apkc-emulador/` como directorio vacío y versiona `.gitmodules`
como fichero. La limpieza quita los dos con `rm -f` y `rm -rf` respectivamente — `rm -f` sobre un
directorio abortaría con `set -eu`. No se publica.

```sh
APKC_WASM=/home/leonisauro/.cache/apkc-web/preview-emulador.wasm sh tools/sitio.sh \
  /home/leonisauro/.cache/apkc-web/sitio-emulador
# wasm_exec: sha256 0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14 (go go1.24.0) OK
# sitio: /home/leonisauro/.cache/apkc-web/sitio-emulador (71 ficheros, version 180ae67c929e)
# zip:   /home/leonisauro/.cache/apkc-web/sitio-emulador.zip (8863829 bytes)
find /home/leonisauro/.cache/apkc-web/sitio-emulador -name '*.go' -o -name '.gitmodules' -o -name 'apkc-emulador'
# (sin salida)
cmp /home/leonisauro/.cache/apkc-web/preview-emulador.wasm \
    /home/leonisauro/.cache/apkc-web/sitio-emulador/apkc.wasm
sha256sum /home/leonisauro/.cache/apkc-web/sitio-emulador/apkc.wasm
# 1c2f521b1998cce4052f3510d9d76f71c4a257b423be005874ebe230952503f5
grep -n -A2 'uses: actions/checkout' .github/workflows/gate.yml
# :26, :45, :155, :218 y :285 → submodules: recursive (cinco, sin editar el workflow)
```

**Frontera y deuda.** `grep -R 'preview.*\.go' sitio/` da tres líneas de prosa (`NOTICE.md`,
dos comentarios de `vista-previa.js`) y una coincidencia binaria en `apkc.wasm`: son menciones
textuales, no ficheros Go. La comprobación correcta es el `find` de arriba, con cero `*.go`.
`git ls-remote` del remoto de GitHub sigue sin ejecutarse (sin red): la CI no verá el gitlink
anidado hasta que el usuario autorice el push de `studio` y del gitlink del padre. `studio`
queda `ahead 3` en local, sin empujar; no hay gate, CDP ni publicación. El vigilante en vivo
no se corrió: su rama nueva se verificó con `sh -n` y con la lectura de los gitlinks, no con un
ciclo real de reconstrucción — eso queda `incierto`.
