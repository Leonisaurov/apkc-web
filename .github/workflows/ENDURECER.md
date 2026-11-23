# Endurecer los workflows: actions por SHA y bundletool con suma

Los tres workflows usan `uses: owner/action@vN`, que es una etiqueta móvil: quien controle el
repositorio de la action puede reapuntarla a un commit distinto sin que cambie una línea de este
repo. `gate.yml` además descarga `bundletool-all-1.18.3.jar` sin comprobar su suma.

Esto quedó **pendiente por falta de red** en la sesión que cerró la cola de seguridad. Los
comandos de abajo son los que hay que ejecutar cuando haya red; hasta entonces los workflows se
dejan como están.

## Fijar cada action por SHA

`gh api repos/{owner}/{repo}/commits/{ref} --jq .sha` resuelve una etiqueta o una rama al commit
al que apunta hoy. Para las actions que aparecen en los workflows:

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
  sha=$(gh api "repos/$repo/commits/$tag" --jq .sha)
  printf '%s -> %s\n' "$ref" "$sha"
done
```

Después, en cada `uses:` se sustituye la etiqueta por el SHA y se deja la etiqueta como
comentario, que es lo que GitHub recomienda para que un lector sepa qué versión era:

```yaml
- uses: actions/checkout@<sha>  # v4
```

Una etiqueta anotada necesita un paso más: `commits/{ref}` ya devuelve el commit, pero si se usa
`git/tags/{sha}` hay que resolver el objeto `tag` a su `commit` con
`gh api repos/{owner}/{repo}/git/tags/{sha} --jq .object.sha`.

## bundletool con suma

La suma del jar que usa `gate.yml`, medida en local sobre las dos copias disponibles
(`~/opt/android-sdk/bundletool/` y la del respaldo de Termux, idénticas), es:

```
a099cfa1543f55593bc2ed16a70a7c67fe54b1747bb7301f37fdfd6d91028e29  bundletool-all-1.18.3.jar
```

Cuando haya red, el paso de descarga de `gate.yml` debe comprobarla antes de usar el jar:

```sh
echo "a099cfa1543f55593bc2ed16a70a7c67fe54b1747bb7301f37fdfd6d91028e29  $RUNNER_TEMP/bundletool/bundletool-all-1.18.3.jar" | sha256sum -c -
```

Si algún día se sube de versión, la suma se vuelve a medir con `sha256sum` sobre el jar descargado
y se actualiza aquí y en el workflow en el mismo cambio.
