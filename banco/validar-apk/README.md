# validar-apk — la salida final del build, por los oráculos

Valida que lo que `apkc build --linker go --compiler go --signer go` deja en `build/app.apk` se
puede instalar y abrir: los dex con los nombres y el orden que el runtime espera, la firma que
`apksigner` acepta y el zip alineado. Los oráculos (`apksigner`, `zipalign`, `dexdump`) no
comparten código con apkc, así que no se aprueban a sí mismos.

## Comando

```sh
sh banco/validar-apk/validar.sh

# el límite de 16 bits, con 700 clases de 100 métodos (tarda y pide memoria: es opt-in)
PROBAR_LIMITE=1 sh banco/validar-apk/validar.sh

# la pasada con un jar de verdad; hoy sale con 3 y dice por qué
JAR_REAL=/ruta/android-support-v4.jar sh banco/validar-apk/validar.sh

# y, si hay aparato, instalar y arrancar el caso multi-dex
SERIE_APARATO=10.177.0.2:5555 sh banco/validar-apk/validar.sh
```

Variables: `APKC` (binario; si falta se compila en `$HOME/.cache/apkc-web/validar-apk`),
`APKC_ANDROID_JAR` o `ANDROID_JAR`, `JAVA_HOME`, `APKC_WEB_CACHE` y `KEYSTORE`. Los artefactos
—proyectos, jars sintéticos, keystore y dex extraídos— van a `$HOME/.cache/apkc-web/validar-apk`:
no se versiona ningún binario (regla 2). Los jars que el guion genera en `jar/` están ignorados.

## Qué mide

Dos pasadas sobre la misma app, con el jar local del fixture (`saludo/Saludo`, el mismo patrón de
`banco/deps-ejemplo`, que ya dexa en verde):

1. **Un jar**: el APK queda con `classes.dex` (la app) y `classes2.dex` (la dependencia).
2. **Veintidós jars**: el dexado de dependencias parte en tres buckets (`depBuckets` agrupa de diez
   en diez), así que el APK lleva `classes.dex` y `classes2.dex`…`classes4.dex`, en ese orden y sin
   huecos. La app llama a `saludo.Saludo`, que cae en el último dex, y a `dep.D20`, que cae en el
   segundo: los dos métodos se invocan desde `classes.dex` y se definen en otro fichero, que es lo
   que el cargador de dex de Android resuelve por nombre.

En las dos pasadas el guion exige:

- `apksigner verify --verbose` con la v1, la v2 y la v3 en `true`;
- `zipalign -c -p 4`;
- los nombres de los dex, exactos y seguidos desde `classes.dex`;
- `dexdump -f` de cada dex (qué clases define cada uno) y `dexdump -d` del dex de la app (la
  invocación del método del jar).

## Medido el 2026-11-23 (árbol `2320838`)

Pasada 1: `1 clases de 1 dependencias -> 860 bytes`, `3 clases -> 1636 bytes en build/classes.dex`;
`classes.dex` 1636 B y `classes2.dex` 860 B; `Verifies`, v1/v2/v3 `true`, `zipalign ok`;
`classes.dex invoca Lsaludo/Saludo;.saludo`, `classes2.dex define Lsaludo/Saludo;`.

Pasada 2: `dexed 22 jars in 3 buckets`, `classes.dex` 1908 B, `classes2.dex` 1396 B,
`classes3.dex` 1400 B y `classes4.dex` 1880 B; las tres dependencias definen las 21 clases
`dep.D0…D20` y `Lsaludo/Saludo;`; `classes.dex` invoca `Lsaludo/Saludo;.saludo` (definido en
`classes4.dex`) y `Ldep/D20;.valor` (definido en `classes2.dex`); v1/v2/v3 `true` y `zipalign ok`.
En el aparato, la app multi-dex arranca y la sonda enseña `text="hola, apkc d20=20"`: el texto sale
del método del segundo y del cuarto dex.

## Fronteras

- **65 536 referencias de método.** Un bucket no parte por su cuenta: con 70 000 métodos el dexer
  rechaza con `Lgrande/C0;<init>: at pc 1, invokespecial: the method index does not fit a 16-bit
  index` (`apkc/dexinstr.go:1258`, la guarda de `indice`) y **no deja apk**, así que no escribe un
  dex con índices recortados. Reparto de un bucket en varios dex es trabajo del dexer, no de este
  guion; el comando que lo reproduce es `PROBAR_LIMITE=1 sh banco/validar-apk/validar.sh`.
- **El jar real todavía no dexa.** `android-support-v4.jar` (648 327 B, 520 clases) se para en
  `Landroid/support/v4/app/BackStackRecord;.dump`: `a goto of -259 code units: the 10t form reaches
  +-127 and goto/16 is not measured here` (`apkc/dexinstr.go:1428`). Es una frontera de opcodes: se
  arregla en `dexinstr.go`, no aquí. `JAR_REAL=… sh banco/validar-apk/validar.sh` imprime ese
  rechazo y sale con 3; cuando el dexer lo acepte, el mismo comando valida el APK del jar real y
  comprueba que `classes.dex` invoca `Landroid/support/v4/text/TextUtilsCompat;.htmlEncode`.

Este guion no corre gate, chromium, red ni publicación.
