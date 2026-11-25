## 350. El emulador pasa al submódulo `apkc-emulador/` y el modelo del preview no cambia

Medido el 2026-11-24 en el worktree `emulador-separacion-f1-f4` del plan aprobado
`…/plan-emulador-modulo/PLAN-emulador-modulo.md`. Sin `git push`, sin gate, sin CDP, sin chromium,
sin red y sin publicar. El commit de partida del padre es `097fbd7` (índice 322) y el puntero de
`studio` sigue en `8788604081945ed58314d333fa5b06e97eb2b7b3` (`ahead 1` del commit de ApkShare,
conservado sin empujar por decisión expresa del usuario).

**Historial importado sin perder commits.** El `subtree split` directo se negó por el commit de
reunión del subtree:

```sh
git -C … subtree split --prefix=apkc
# fatal: could not rev-parse split hash a90921488389de1b08fa9c3aeea68b90635ab192 from commit ab4fd902c668f95e760cb7d152d3c412c7ee551f
git -C … subtree split --prefix=apkc --ignore-joins
# 847/847 (846) [0]
# aa78f5fbda6dae426eaeb69ad5a24584ea5a17bd
```

El resultado se exportó con una rama transitoria (`emu-split-export`, borrada justo después) y se
importó en `$HOME/.cache/apkc-web/emulador-origen` sin red. Como `git filter-repo` no está instalado,
la poda se hizo con el `git filter-branch` que sí viene con git:

```sh
git -C …/emulador-origen filter-branch -f \
  --index-filter 'git ls-files -z | grep -zv "^preview" | xargs -0 -r git rm --cached -q --ignore-unmatch' \
  --prune-empty -- importado
# Ref 'refs/heads/importado' was rewritten
git -C …/emulador-origen branch -m importado emulador-modulo
git -C …/emulador-origen rev-list --count --all
# 59
git -C …/emulador-origen ls-tree --name-only HEAD | wc -l
# 34
git -C …/emulador-origen log --oneline -- preview.go | head -3
# d95e2e5 fix(preview): refluye las filas horizontales que desbordan el marco
# daa2565 fix(preview): cuenta el ArrayAdapter vacío y pinta las notas de la sesión viva
# 1006d6a feat(preview): sesión viva de hot swap sin replay ni error pegajoso
```

Los 59 commits son los mismos que en el padre (`git -C … log --oneline -- 'apkc/preview*.go' | wc -l`
→ `59`). `refs/original` y los reflogs se expurgaron; el `origin` del submódulo apunta a
`https://github.com/Leonisaurov/apkc-emulador.git`, sin publicar.

El puntero importado es `563e991de1fac3c863e5278483c2ff745d57bbb2` (`fix(preview): hereda el locale
en la sesion y confirma licencia de CAs`); la salida de `git log --oneline -- preview.go` que se cita
arriba empieza en `d95e2e5` porque ese fichero concreto no cambió en los últimos commits del
historial importado.

**Frontera materializada.** En el padre: `git rm 'apkc/preview*.go'` (34 ficheros),
`git -c protocol.file.allow=always -C … submodule add … apkc-emulador` (gitlink a `563e991`),
`.gitmodules` con la URL https y `.gitignore` con `apkc/preview*.go`. El puntero de `studio` se
comprobó de nuevo en HEAD y en el índice: `8788604081945ed58314d333fa5b06e97eb2b7b3` antes y
después del traslado. Las citas históricas con prefijo `apkc/preview*.go:NNN` se reescribieron a
`apkc-emulador/<fichero>:NNN` (mismo contenido y mismas líneas); las citas desnudas que son salida
literal de un test, como `previewlistas_test.go:537:`, se dejaron intactas para no falsificar lo
medido. El `cmp` de los 34 ficheros entre ambos checkouts dio salida vacía.

**El wasm es byte a byte el de siempre.** Se compiló el árbol separado y un archivo de `HEAD` con
las mismas banderas y la toolchain fijada:

```sh
GOOS=js GOARCH=wasm CGO_ENABLED=0 go1.24.0 go build -ldflags='-s -w' -o …/wasm.wasm .
```

Compilados en directorios distintos los binarios difieren (`differ: byte 42`, 16.248.098 frente a
16.250.018 bytes) porque Go incrusta la ruta de compilación; compilando los dos árboles desde la
**misma** ruta, `cmp` da salida vacía y ambos tienen
`sha256 a00e24effe90aff741a2ae63e5976985ed8dc88880073979bf5b3a707b9c9c43`. El intérprete no se
divisionó en un segundo binario.

**El modelo del hospedador es idéntico.** Con `shim/wasm_exec.js` validado por
`tools/wasm-exec-hash.sh` (`wasm_exec: sha256 0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14
(go go1.24.0) OK`):

```sh
node shim/navegador-sim.js --montar …/studio-e2e-contador/com.hola.contador/build/app.apk:/app.apk \
  …/apkc-wasm.wasm preview /app.apk
# {"paquete":"com.hola.contador","version":"0.1","actividad":".MainActivity",
#  "layout":"res/layout/main.xml","pantalla":{"x":0,"y":0,"ancho":360,"alto":640},
#  "aproximado":true,"nodos":3,"ejecucion":{"actividad":"com.hola.contador.MainActivity",
#  "metodo":"onCreate","pasos":22, …},"raiz":{ … "TextView" "Pulsos: 0", "Button" "Pulsar" … }}
```

La misma línea de JSON sale del wasm construido desde `HEAD` antes de separar (mismo `pasos: 22`,
mismo rect de 360×640 y mismos ids). La salida completa se reproduce con el comando de arriba sobre
el fixture `~/.cache/apkc-web/tmp/studio-e2e-contador/com.hola.contador/build/app.apk`.

**Frontera abierta.** El `apkc.wasm` que sirve la página sigue siendo el del checkout principal: el
vigilante `tools/preview-automatica.sh` reconstruye desde `HEAD`, pero aquí no se ha publicado ni
reconstruido ese destino. `.github/workflows/gate.yml` ya tiene `submodules: recursive` en cinco
checkouts, así que la CI remota **fallará** al querer clonar `apkc-emulador` mientras el submódulo y
el gitlink no se empujen: deuda registrada, no hecha. Sin gate, CDP, red ni publicación en esta
medición.

## 351. Copia gobernada de los 34 ficheros, con detección de deriva

El contrato es de fuentes gobernadas, no de import Go: los campos que cruza el intérprete con el
núcleo no están exportados (`tabla.global.cadenas`, `e.valor.tipo/datos`, `bl.nombre`,
`n.tipo/ns/nombre`, `at.valor`, `d.clases/types/methods`, `it.insns/tries/handlers`), así que un
import exigiría exportar ~50 campos y reescribir más de diez mil líneas. En su lugar
`tools/materializar-emulador.sh` copia los 34 ficheros al construir y comprueba cada copia con
`cmp`:

```sh
sh tools/materializar-emulador.sh
# materializados 34 ficheros preview*.go de …/apkc-emulador en …/apkc
for f in apkc-emulador/preview*.go; do cmp "$f" apkc/"$(basename "$f")" || exit 1; done
# (salida vacía: 34/34 idénticos)
```

El guion cuenta los ficheros y falla si no son exactamente 34 (8 de producción y 26 tests, incluido
`previewsesion_locale_test.go`), de modo que ni un fichero que falte ni uno nuevo pasa desapercibido.
`apkc/preview*.go` está en `.gitignore`: el único sitio editable es el submódulo.

**Cobertura de los caminos de build.** `grep -n 'materializar'` muestra los seis hooks:
`tools/entorno.sh:130-131` (función), `:135` (`compila_wasm`), `:142` (`construye_wasm`), `:148`
(`construye_binarios`) y `:170` (rama `test`); `tools/preview-local.sh:22-27` y
`tools/preview-automatica.sh:32-40` (este último extrae el submódulo en el SHA exacto del gitlink de
HEAD antes de llamar a `preview-local.sh`). El contraste con
`grep -nE 'go (test|build)' tools/entorno.sh` da los cuatro puntos reales —`:136`, `:143`, `:149`,
`:171`—, cada uno precedido de su hook. Fuera de `entorno.sh`, nueve guiones de `banco/` que
compilan `apkc` directamente llevan su propia llamada: `incremental.sh:31`,
`incremental-wasm.sh:54`, `art-verificador/art-verificador.sh:58`,
`apps-ejemplo/construir.sh:17`, `apps-ejemplo/endurecer.sh:29`,
`aab-multidex/aab-multidex.sh:77`, `validar-apk/validar.sh:47`,
`art-jar-real/art-jar-real.sh:85` y `deps-ejemplo/construir.sh:31`. `tools/sitio.sh` y `shim/` no
compilan fuentes Go y no cambian; CI ya tenía `submodules: recursive`.

**Falsación de la deriva.** Sobre un clon local del submódulo, se cambió un byte de `preview.go` y
se comparó con el original gobernado:

```sh
printf 'X' | dd of=…/deriva/preview.go bs=1 seek=120 conv=notrunc status=none
cmp …/deriva/preview.go …/apkc-emulador/preview.go; echo "rc=$?"
# …/deriva/preview.go …/apkc-emulador/preview.go differ: byte 121, line 4
# rc=1
sh tools/materializar-emulador.sh …/deriva-raiz …/deriva
# materializados 34 ficheros preview*.go de …/deriva en …/deriva-raiz/apkc
# bucle cmp posterior: rc=0 tras materializar
```

La copia sigue siempre al origen: editar un fichero generado en `apkc/` se pierde en el siguiente
build. Frontera: el vigilante no se ha corrido en esta sesión; su cobertura queda demostrada por el
código y por la ejecución directa del guion de materialización, no por una reconstrucción en vivo del
servidor.

## 352. La página sigue intacta: suite Go y `prueba.js` en verde sobre el wasm separado

Sin CDP y sin chromium, sobre el árbol con los 34 ficheros materializados y la toolchain fijada
`go1.24.0`:

```sh
tcr -c 2 -m 2G -r 256M -- go test -run TestPreview -v .
# PASS
# ok  	github.com/ajinasokan/apkc	1.337s
tcr -c 2 -m 2G -r 256M -- go test -count=1 ./...
# ok  	github.com/ajinasokan/apkc	260.651s
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -ldflags='-s -w' -o …/apkc-wasm.wasm .
node shim/navegador-sim.js --montar …/app.apk:/app.apk …/apkc-wasm.wasm preview /app.apk
# JSON con "pasos":22 (ídem al del árbol sin separar; §350)
node prueba.js     # desde studio/, checkout 8788604081945ed58314d333fa5b06e97eb2b7b3
# reglas confirmadas contra salidas reales: 9/9
# reglas: 24 | ancladas al fuente de apkc o a una salida capturada: 21 | sin captura propia: 3
# prueba: todo en orden
```

`prueba.js:6722` imprime «prueba: todo en orden» solo cuando `fallos === 0`; no apareció ninguna
línea `FALLO:` ni `PRUEBA FALLA`. El fixture de emulación real se omite cuando no se pasan
`APKC_PREVIEW`/`APK_*`, como en corridas anteriores, y se dice en la propia salida. `studio/` no
importa el submódulo: recibe el mismo `apkc.wasm` de siempre y su puntero no se movió.

**Frontera honesta.** No se ejecutaron `tools/entorno.sh gate`, `studio/prueba-pestana.js` ni
`studio/prueba-sitio.js` (CDP/chromium), ni `surge . publish`, ni `git push` del padre, de
`apkc-emulador` o de `studio` (que sigue `ahead 1`). El wasm servido en la vista previa sigue siendo
el del checkout principal hasta que el vigilante reconstruya; la CI con `submodules: recursive`
seguirá fallando hasta que se autorice el push. Nada de eso se cuenta como verde por analogía con
estas mediciones de capa node/Go (regla 11).
