# apkc studio

Una página para crear proyectos de [apkc](../apkc) en el navegador y medir dónde se atasca
quien los usa. Sin dependencias: abre `index.html` con `file://` y funciona.

Qué hace, en el orden en que duele:

1. **Genera el proyecto** (manifest, `MainActivity.java` o `.kt`, layout, styles, carpetas) y lo
   baja como zip. Las plantillas son copia literal de `apkc/template/*`; `prueba.js` compara los
   atributos que apkc escribe con los del repo, porque una tilde dentro de `android:versionCode`
   rompe el XML y `aapt2` lo rechaza en el teléfono.
2. **Dice qué comandos correr**, en el orden en que un desconocido los necesita.
3. **Traduce la salida real de apkc**: 24 reglas, cada una con `lit:` (la cadena que la motivó) y
   `de:` (quién la imprime). Las de apkc se verifican contra el fuente en la propia prueba; una
   regla inventada explicaría un error que nadie imprime, que es peor que no tenerla.
4. **Lista las líneas que no conoce** y lleva contadores de dónde se rompió cada intento. Eso es
   la medida de fricción; el informe en markdown es para traerlo a quien mantiene la herramienta.
   Los logs pegados no salen del navegador: se quedan en `localStorage`.

## Probar

```sh
node prueba.js                 # lo que genera: ascii, plantillas contra el repo, EOCD, reglas
node prueba-navegador.js       # chromium headless por CDP: DOM, clicks, descarga, localStorage
APKC=/ruta/apkc sh e2e.sh      # todo lo anterior + compilar y verificar los dos proyectos
```

`e2e.sh` es el que importa: descomprime el zip de verdad, corre `apkc build` sobre él y exige que
`apksigner verify` (v1 y v2) y `zipcheck.py` del harness acepten el apk. Con `node prueba.js`
solo no basta — encontró seis fallos que el test propio veía verdes: contador del EOCD duplicado,
directorios en modo `000`, `versiónCode` con tilde, un bloque de Java metido dentro de `onCreate`,
`STUDIO_KOTLIN=0` que era verdad en JS (la variante "java" compilaba Kotlin) y un `LEEME.txt` en
`lib/arm64-v8a/` que acababa dentro del apk como si fuera una `.so`.

La variante kotlin necesita `kotlinc` y un `kotlin-stdlib*.jar` por `~/Android`; si no hay, `e2e.sh`
la omite y lo dice.

## Riesgo conocido

`apkc/template/*` puede cambiar y la página no se entera sola: `prueba.js` compara contra el repo
que esté en `APKC_REPO` (por defecto `~/Develop/Go/apkc`), así que la desincronización salta al
correr la prueba, no al usar la página.
