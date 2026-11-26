# 26 — Construcción reproducible del wasm del preview

## 392. Construcción reproducible del wasm del preview (2026-11-26)

El commit `a8ab3b8c75c4ab9dc095d1dc562323dd03ac02ab` (`fix(preview): compila el wasm con trimpath reproducible`) añade `-trimpath` a los tres puntos que compilan wasm. La auditoría de ese diff se cerró como **APROBADO CON CONDICIONES** y la condición funcional quedó cerrada por una ejecución posterior; el veredicto y su cierre se **transfieren** de la sesión, no se re-miden aquí:

```sh
sqlite3 ~/.local/share/opencode/opencode.db \
  "select seq, substr(data,instr(data,'### Comandos ejecutados'),4500) \
   from session_message \
   where session_id='ses_deee49022ffeE98tk8aapVonRH' and seq=13320;"
# ... **Condición cerrada**: el diff sigue siendo exactamente las tres líneas,
# `git diff --check` da rc 0, el wasm con `-trimpath -ldflags='-s -w'` es funcional
# (`rc=0`, cinco ficheros) y su hash es reproducible e idéntico al de la auditoría
# en los dos caminos de build (`entorno.sh` y `preview-local.sh`).
```

**La causa medida.** El vigilante compilaba cada HEAD en un `mktemp -d` distinto y, sin
`-trimpath`, Go incrustaba en el binario las rutas absolutas de los ficheros `.go`; por eso el
hash del wasm cambiaba entre `274ba82` y `6d69af9` aunque el diff entre esos dos HEAD era solo
documentación. No había cambio funcional. La parte documental se comprueba aquí:

```sh
git diff --stat 274ba82 6d69af9
#  AGENTS.md                        |   2 +-
#  docs/evidencia.md                |   3 +-
#  docs/evidencia/15-apps-reales.md | 155 +++++++++++++++++++++++++++++++++++
#  3 files changed, 158 insertions(+), 2 deletions(-)
git diff --name-only 274ba82 6d69af9
# AGENTS.md
# docs/evidencia.md
# docs/evidencia/15-apps-reales.md
```

La atribución de la causa al cambio de ruta y a la ausencia de `-trimpath` procede del informe
del builder y de la auditoría, no de una re-medición con el binario anterior; se recupera con:

```sh
sqlite3 ~/.local/share/opencode/opencode.db \
  "select seq, substr(data,instr(data,'### Comandos'),4000) \
   from session_message \
   where session_id='ses_deee49022ffeE98tk8aapVonRH' and seq=13058;"
# ... El vigilante compilaba cada HEAD en un `mktemp -d` distinto y, sin `-trimpath`,
# Go incrustaba las rutas absolutas de los `.go` ...
```

**El cambio.** Tres líneas, en los dos caminos que producen wasm; el `go build` nativo de
`apkc-gate` queda fuera porque no genera wasm:

```sh
git show --stat --format='%H%n%s' a8ab3b8c75c4ab9dc095d1dc562323dd03ac02ab
# a8ab3b8c75c4ab9dc095d1dc562323dd03ac02ab
# fix(preview): compila el wasm con trimpath reproducible
#
#  tools/entorno.sh       | 4 ++--
#  tools/preview-local.sh | 2 +-
#  2 files changed, 3 insertions(+), 3 deletions(-)
git show a8ab3b8c75c4ab9dc095d1dc562323dd03ac02ab -- tools/preview-local.sh tools/entorno.sh
# -  ... go test -c -o "$TMP/apkc-test.wasm" .
# +  ... go test -c -trimpath -o "$TMP/apkc-test.wasm" .        (entorno.sh:138)
# -  ... go build -ldflags='-s -w' -o "$TMP/apkc-wasm.wasm" .
# +  ... go build -trimpath -ldflags='-s -w' -o "$TMP/apkc-wasm.wasm" .  (entorno.sh:146)
# -  GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -C "$RAIZ/apkc" -ldflags='-s -w' -o "$TEMPORAL" .
# +  GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -C "$RAIZ/apkc" -trimpath -ldflags='-s -w' -o "$TEMPORAL" .  (preview-local.sh:30)
```

**Dos builds del mismo HEAD desde dos temporales distintos.** El guion
`~/.cache/apkc-web/trimpath-verif.sh` replica el vigilante (`git archive HEAD apkc`, emulador
del gitlink y `preview-local.sh` bajo `tcr`) en dos `mktemp -d`, A y B. Registro literal:

```sh
cat ~/.cache/apkc-web/trimpath-verif.HBmrDG/resultado.log
# --- build A desde /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/A ---
# ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  .../A/apkc.wasm
# --- build B desde /home/leonisauro/.cache/apkc-web/trimpath-verif.HBmrDG/B ---
# ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  .../B/apkc.wasm
# --- comparacion ---
# cmp: byte a byte identicos
sha256sum ~/.cache/apkc-web/trimpath-verif.HBmrDG/A/apkc.wasm ~/.cache/apkc-web/trimpath-verif.HBmrDG/B/apkc.wasm
# ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  .../A/apkc.wasm
# ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  .../B/apkc.wasm
cmp ~/.cache/apkc-web/trimpath-verif.HBmrDG/A/apkc.wasm ~/.cache/apkc-web/trimpath-verif.HBmrDG/B/apkc.wasm; echo "cmp rc=$?"
# cmp rc=0
```

**Rebuild independiente del auditor.** Dos caminos de build distintos (`entorno.sh` y
`preview-local.sh`) sobre la copia aislada dan la misma suma que A y B; la comparación es entre
cuatro ficheros medidos:

```sh
sha256sum ~/.cache/apkc-web/trimpath-funcional/apkc-wasm-pin.wasm ~/.cache/apkc-web/trimpath-funcional/apkc-wasm-preview.wasm
# ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  .../apkc-wasm-pin.wasm
# ffa03ac56f6c23906febf24a16a8dd6a9113853dab3a972b44ff5309412d2dd0  .../apkc-wasm-preview.wasm
```

**Prueba funcional en el hospedador de Node.** El `wasm_exec.js` usado sale del GOROOT de Go
1.24.0 y su suma coincide con la fijada en el repo:

```sh
grep -n SUMA_FIJADA tools/wasm-exec-hash.sh
# 26:SUMA_FIJADA=0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14
sha256sum ~/.cache/apkc-web/trimpath-funcional/shim/wasm_exec.js
# 0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14  .../shim/wasm_exec.js
node shim/navegador-sim.js \
  ~/.cache/apkc-web/trimpath-funcional/apkc-wasm-pin.wasm create /miapp
# [doctor] open /android/build-tools: No such file or directory
# Package name (com.myapp): App name (My App): [apkc] finished in 35.4688ms
# --- ficheros en la memoria simulada: 5
#     /miapp/.gitignore 7 bytes
#     /miapp/AndroidManifest.xml 748 bytes
#     /miapp/res/layout/main.xml 468 bytes
#     /miapp/res/values/styles.xml 423 bytes
#     /miapp/src/com/myapp/MainActivity.java 387 bytes
```

Registro `~/.cache/apkc-web/trimpath-verif.HBmrDG/funcional.log` (primera ejecución, 35.4688
milisegundos). La ejecución de cierre de la condición, con el mismo binario y el mismo
hospedador, se transfiere de la sesión y dio 51.144192 milisegundos:

```sh
sqlite3 ~/.local/share/opencode/opencode.db \
  "select seq, substr(data,instr(data,'51.144192')-700,1500) \
   from session_message \
   where session_id='ses_deee49022ffeE98tk8aapVonRH' and seq=13320;"
# ... `node navegador-sim.js … create /miapp` → `rc=0` ...
# `Package name (com.myapp): App name (My App): [apkc] finished in 51.144192ms`
# `--- ficheros en la memoria simulada: 5` ...
```

El mensaje `[doctor] open /android/build-tools: No such file or directory` es del entorno
simulado, no del wasm; las dos ejecuciones generaron cinco ficheros y terminaron con `rc=0`.

**Fronteras de este §.** No se corrió gate, CDP, chromium, `prueba-pestana.js`,
`prueba-sitio.js` ni publicación, y no se hizo red. Por tanto no se afirma nada sobre la pestaña
real ni sobre el sitio publicado: lo medido aquí es el binario y su ejecución en el hospedador de
Node. El wasm que sirve el vigilante sigue siendo el anterior hasta que el commit se integre en
el checkout principal y el vigilante detecte el nuevo HEAD (procedencia: informe del builder,
mensaje 13058 de la sesión citada).
