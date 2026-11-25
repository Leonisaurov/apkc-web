# Endurecer los workflows: actions por SHA y bundletool con suma

Los tres workflows usan `uses: owner/action@vN`, que es una etiqueta móvil: quien controle el
repositorio de la action puede reapuntarla a un commit distinto sin que cambie una línea de este
repo. `gate.yml` además descargaba `bundletool-all-1.18.3.jar` sin comprobar su suma.

## Estado (2026-11-24): pinning hecho, run pendiente

Hecho:

- **bundletool con suma**: el paso «oráculo bundletool» de `gate.yml` ejecuta `sha256sum -c -`
  contra la suma medida en local (abajo) y aborta el job si el jar descargado no coincide.
- **Permisos mínimos**: los cinco jobs de `gate.yml` que solo leen el repo (`rapido`, `pesado`,
  `webkit`, `safari`, `art`) declaran `permissions: contents: read`. `publicar` conserva
  `contents: write`. `android-jar.yml` y `deps-maven.yml` ya lo declaraban a nivel de workflow.
- **CSP con `form-action 'none'`** en la política que genera `tools/sitio.sh` (la que se inyecta en
  `studio/sw.js` y en el `<meta>` de `index.html`). El chequeo que aborta si queda el marcador
  `__APKC_CSP__` sin sustituir sigue intacto (`tools/sitio.sh:141-144`).
- **Actions fijadas por SHA (fase 1)**: las 30 `uses:` de los tres workflows llevan un SHA de 40 hex
  resuelto con la API de GitHub y conservan la etiqueta legible como comentario (`# v4`, `# v5`,
  `# v1`). Comprobación: `grep -R -c 'uses: .*@[0-9a-f]\{40\}' .github/workflows/*.yml` devuelve
  21, 7 y 2; `grep -R -n '@v[0-9]\|TODO(sha)' .github/workflows/*.yml` no devuelve nada.

Pendiente (fase 2): **el ID del run que valida estos SHA**. Todavía no hay run, así que la columna
«ID de run» de la tabla de abajo dice `pendiente` en las 30 filas. No se rellena con un run
inexistente ni con el número de otro workflow. La política es pin, run y commit de la evidencia:
el pin ya está hecho; cuando haya un run sobre este árbol se registrará en un commit posterior la
tupla `ref|tag|SHA|fecha|ID de run|log` de cada action. Hasta entonces la fila sigue pendiente; un
SHA resuelto por la API no sustituye a la validación del CI.

## Fijar cada action por SHA

`gh api repos/{owner}/{repo}/git/ref/tags/{tag} --jq .object.sha` resuelve una etiqueta al SHA de
su objeto. Las actions que aparecen en los workflows:

```sh
for ref in \
  actions/checkout@v4 \
  actions/setup-go@v5 \
  actions/setup-node@v4 \
  actions/setup-java@v4 \
  actions/cache@v4 \
  actions/upload-artifact@v4 \
  actions/download-artifact@v4 \
  browser-actions/setup-chrome@v1
do
  repo=${ref%@*}
  tag=${ref#*@}
  sha=$(gh api "repos/$repo/git/ref/tags/$tag" --jq .object.sha)
  printf '%s -> %s\n' "$ref" "$sha"
done
```

Después, en cada `uses:` se sustituye la etiqueta por el SHA y se deja la etiqueta como
comentario, que es lo que GitHub recomienda para que un lector sepa qué versión era:

```yaml
- uses: actions/checkout@<sha>  # v4
```

Una etiqueta anotada necesita un paso más: si `git/ref/tags/{tag}` devuelve un objeto `tag`
(comprobable con `--jq .object.type`), su SHA no es el del commit; se resuelve con
`gh api repos/{owner}/{repo}/git/tags/{sha} --jq .object.sha`. `git/ref/tags` ya devuelve el
commit cuando la etiqueta es ligera.

El único caso anotado de estos workflows es `browser-actions/setup-chrome@v1`:

```sh
gh api repos/browser-actions/setup-chrome/git/ref/tags/v1 --jq '.object.sha + " " + .object.type'
# 19ae4b339ee18925ab85cf12c1041150ea4a44c8 tag
gh api repos/browser-actions/setup-chrome/git/tags/19ae4b339ee18925ab85cf12c1041150ea4a44c8 --jq .object.sha
# c785b87e244131f27c9f19c1a33e2ead956ab7ce
```

### Procedencia de los 30 usos

Cada fila corresponde a una línea `uses:`. La fecha es la del commit al que apunta el SHA
(`gh api repos/{owner}/{repo}/git/commits/{sha} --jq .committer.date`). El ID de run se añadirá en
un commit posterior, cuando haya run; mientras tanto la columna dice `pendiente`.

| Dónde | ref | etiqueta | SHA | fecha del commit | ID de run |
| --- | --- | --- | --- | --- | --- |
| `android-jar.yml:25` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `android-jar.yml:27` | `actions/setup-go@v5` | `v5` | `40f1582b2485089dde7abd97c1529aa768e1baff` | 2025-12-15T18:14:51Z | pendiente |
| `android-jar.yml:112` | `actions/upload-artifact@v4` | `v4` | `ea165f8d65b6e75b540449e92b4886f43607fa02` | 2025-03-19T17:34:59Z | pendiente |
| `android-jar.yml:124` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `android-jar.yml:126` | `actions/setup-go@v5` | `v5` | `40f1582b2485089dde7abd97c1529aa768e1baff` | 2025-12-15T18:14:51Z | pendiente |
| `android-jar.yml:130` | `actions/setup-java@v4` | `v4` | `cf277c60eb25467037889841efdb72551f06f6c3` | 2026-08-04T15:48:38Z | pendiente |
| `android-jar.yml:144` | `actions/download-artifact@v4` | `v4` | `d3f86a106a0bac45b974a628896c90dbdf5c8093` | 2025-04-24T16:25:03Z | pendiente |
| `deps-maven.yml:32` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `deps-maven.yml:265` | `actions/upload-artifact@v4` | `v4` | `ea165f8d65b6e75b540449e92b4886f43607fa02` | 2025-03-19T17:34:59Z | pendiente |
| `gate.yml:24` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `gate.yml:27` | `actions/setup-node@v4` | `v4` | `49933ea5288caeca8642d1e84afbd3f7d6820020` | 2025-04-02T19:20:51Z | pendiente |
| `gate.yml:43` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `gate.yml:46` | `actions/setup-go@v5` | `v5` | `40f1582b2485089dde7abd97c1529aa768e1baff` | 2025-12-15T18:14:51Z | pendiente |
| `gate.yml:52` | `actions/setup-node@v4` | `v4` | `49933ea5288caeca8642d1e84afbd3f7d6820020` | 2025-04-02T19:20:51Z | pendiente |
| `gate.yml:55` | `actions/setup-java@v4` | `v4` | `cf277c60eb25467037889841efdb72551f06f6c3` | 2026-08-04T15:48:38Z | pendiente |
| `gate.yml:63` | `actions/cache@v4` | `v4` | `0057852bfaa89a56745cba8c7296529d2fc39830` | 2025-09-24T13:47:33Z | pendiente |
| `gate.yml:79` | `browser-actions/setup-chrome@v1` | `v1` | `c785b87e244131f27c9f19c1a33e2ead956ab7ce` | 2025-01-25T09:35:04Z | pendiente |
| `gate.yml:124` | `actions/upload-artifact@v4` | `v4` | `ea165f8d65b6e75b540449e92b4886f43607fa02` | 2025-03-19T17:34:59Z | pendiente |
| `gate.yml:134` | `actions/upload-artifact@v4` | `v4` | `ea165f8d65b6e75b540449e92b4886f43607fa02` | 2025-03-19T17:34:59Z | pendiente |
| `gate.yml:152` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `gate.yml:155` | `actions/setup-node@v4` | `v4` | `49933ea5288caeca8642d1e84afbd3f7d6820020` | 2025-04-02T19:20:51Z | pendiente |
| `gate.yml:162` | `actions/download-artifact@v4` | `v4` | `d3f86a106a0bac45b974a628896c90dbdf5c8093` | 2025-04-24T16:25:03Z | pendiente |
| `gate.yml:215` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `gate.yml:218` | `actions/setup-node@v4` | `v4` | `49933ea5288caeca8642d1e84afbd3f7d6820020` | 2025-04-02T19:20:51Z | pendiente |
| `gate.yml:221` | `actions/download-artifact@v4` | `v4` | `d3f86a106a0bac45b974a628896c90dbdf5c8093` | 2025-04-24T16:25:03Z | pendiente |
| `gate.yml:272` | `actions/checkout@v4` | `v4` | `11d5960a326750d5838078e36cf38b85af677262` | 2026-07-16T19:43:47Z | pendiente |
| `gate.yml:275` | `actions/setup-go@v5` | `v5` | `40f1582b2485089dde7abd97c1529aa768e1baff` | 2025-12-15T18:14:51Z | pendiente |
| `gate.yml:281` | `actions/setup-java@v4` | `v4` | `cf277c60eb25467037889841efdb72551f06f6c3` | 2026-08-04T15:48:38Z | pendiente |
| `gate.yml:346` | `actions/upload-artifact@v4` | `v4` | `ea165f8d65b6e75b540449e92b4886f43607fa02` | 2025-03-19T17:34:59Z | pendiente |
| `gate.yml:444` | `actions/download-artifact@v4` | `v4` | `d3f86a106a0bac45b974a628896c90dbdf5c8093` | 2025-04-24T16:25:03Z | pendiente |

## bundletool con suma

La suma del jar que usa `gate.yml`, medida en local sobre las dos copias disponibles
(`~/opt/android-sdk/bundletool/` y la del respaldo de Termux, idénticas), es:

```
a099cfa1543f55593bc2ed16a70a7c67fe54b1747bb7301f37fdfd6d91028e29  bundletool-all-1.18.3.jar
```

El paso de descarga de `gate.yml` la comprueba antes de usar el jar:

```sh
echo "a099cfa1543f55593bc2ed16a70a7c67fe54b1747bb7301f37fdfd6d91028e29  $RUNNER_TEMP/bundletool/bundletool-all-1.18.3.jar" | sha256sum -c -
```

Si algún día se sube de versión, la suma se vuelve a medir con `sha256sum` sobre el jar descargado
y se actualiza aquí y en el workflow en el mismo cambio.

## Entorno de compilación fijado

- **Go**: los dos pasos `actions/setup-go` de `gate.yml` (jobs `pesado` y `art`) fijan
  `go-version: '1.24.0'` en lugar de leer `apkc/go.mod`, que solo declara el mínimo `go 1.24`.
  Así un cambio de patch del runner no cambia el binario sin que este repo lo vea.
- **wasm_exec.js**: antes de copiarlo desde el GOROOT, `gate.yml` invoca
  `sh tools/wasm-exec-hash.sh` desde la raíz. El guion lo aporta el frente W2; aquí solo se fija
  el punto de llamada, que queda delante de la copia.
