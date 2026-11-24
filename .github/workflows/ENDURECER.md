# Endurecer los workflows: actions por SHA y bundletool con suma

Los tres workflows usan `uses: owner/action@vN`, que es una etiqueta móvil: quien controle el
repositorio de la action puede reapuntarla a un commit distinto sin que cambie una línea de este
repo. `gate.yml` además descargaba `bundletool-all-1.18.3.jar` sin comprobar su suma.

## Estado (2026-11-23): lo que quedó hecho y lo que sigue pendiente por red

Hecho, sin necesitar red:

- **bundletool con suma**: el paso «oráculo bundletool» de `gate.yml` ejecuta `sha256sum -c -`
  contra la suma medida en local (abajo) y aborta el job si el jar descargado no coincide.
- **Permisos mínimos**: los cinco jobs de `gate.yml` que solo leen el repo (`rapido`, `pesado`,
  `webkit`, `safari`, `art`) declaran `permissions: contents: read`. `publicar` conserva
  `contents: write`. `android-jar.yml` y `deps-maven.yml` ya lo declaraban a nivel de workflow.
- **CSP con `form-action 'none'`** en la política que genera `tools/sitio.sh` (la que se inyecta en
  `studio/sw.js` y en el `<meta>` de `index.html`). El chequeo que aborta si queda el marcador
  `__APKC_CSP__` sin sustituir sigue intacto (`tools/sitio.sh:141-144`).

Pendiente por red: **ninguna `uses:` está fijada a un SHA**. No hay ningún SHA de action en la
documentación local y esta sesión no tuvo red; adivinarlo sería peor que dejar la etiqueta visible.
Cada `uses:` lleva ahora el comentario `# TODO(sha): requiere red; gh api repos/<owner>/<repo>/git/ref/tags/vN --jq .object.sha`.
Para auditarlo, `grep -c "@v[0-9]" .github/workflows/*.yml` debe devolver exactamente las líneas
marcadas con `TODO(sha)` (hoy, 30 en los tres `.yml`).

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
