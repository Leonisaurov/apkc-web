# validar-apk — la salida final del build, por los oráculos

Valida que lo que `apkc build --linker go --compiler go --signer go` deja en `build/app.apk` se
puede instalar y abrir: los dex con los nombres y el orden que el runtime espera, la firma que
`apksigner` acepta y el zip alineado. Los oráculos (`apksigner`, `zipalign`, `dexdump`) no
comparten código con apkc, así que no se aprueban a sí mismos.

## Comando

```sh
sh banco/validar-apk/validar.sh

# el techo de 16 bits, con 700 clases de 100 métodos (tarda y pide memoria: es opt-in)
PROBAR_LIMITE=1 sh banco/validar-apk/validar.sh

# la pasada con un jar de verdad (520 clases; valida su APK, §225)
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
`banco/deps-ejemplo`, que ya dexa en verde), y una tercera opcional con otro proyecto:

1. **Un jar**: el APK queda con `classes.dex` (la app) y `classes2.dex` (la dependencia).
2. **Veintidós jars**: el dexado de dependencias parte en tres buckets (`depBuckets` agrupa de diez
   en diez), así que el APK lleva `classes.dex` y `classes2.dex`…`classes4.dex`, en ese orden y sin
   huecos. La app llama a `saludo.Saludo`, que cae en el último dex, y a `dep.D20`, que cae en el
   segundo: los dos métodos se invocan desde `classes.dex` y se definen en otro fichero, que es lo
   que el cargador de dex de Android resuelve por nombre.
3. **El techo de 16 bits** (opt-in, `PROBAR_LIMITE=1`): 700 clases y 70 000 métodos encadenados, que
   no caben en un dex. El dexado reparte el bucket en `classes.dex`…`classes3.dex`; el guion exige
   que ninguno pase de 65 536 `method_id` y que las 700 clases queden repartidas entre los dos dex
   de dependencias, sin perder ni duplicar ninguna (los detalles, en «El techo de 65 536
   referencias»).

En las dos pasadas el guion exige:

- `apksigner verify --verbose` con la v1, la v2 y la v3 en `true`;
- `zipalign -c -p 4`;
- los nombres de los dex, exactos y seguidos desde `classes.dex`;
- que el dex que define la clase **no** sea el que la invoca: si el método del jar cayera en
  `classes.dex`, el APK no probaría el reparto en `classesN.dex` y el guion fallaría;
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

Pasada del techo de 16 bits (`PROBAR_LIMITE=1`, binario del árbol `d4da31c`): el build termina en
verde y deja `classes.dex` 1432 B (la app, 9 `method_id`), `classes2.dex` 2 772 816 B (645 clases,
65 446 `method_id`) y `classes3.dex` 251 112 B (55 clases, 5 756 `method_id`); `Verifies`, v1/v2/v3
`true`, `zipalign ok`, las 700 clases `grande.C0…C699` entre los dos dex de dependencias sin huecos
ni repetidas, y `classes2.dex` invoca `Lgrande/C680;.m0`, que define `classes3.dex`.

Pasada del jar real (`JAR_REAL=…`, binario del árbol `a376330`): `520 clases de 1 dependencias ->
526856 bytes`; `classes.dex` 1668 B (la app) y `classes2.dex` 526 856 B con las 520 clases del jar;
`Verifies`, v1/v2/v3 `true`, `zipalign ok`; `dexdump -d` no da `Failure to verify` en ninguna de
los dos; `classes.dex` invoca `Landroid/support/v4/text/TextUtilsCompat;.htmlEncode` y `classes2.dex`
la define. La corrida con `PROBAR_LIMITE=1` termina con las dos líneas `OK`.

## El techo de 65 536 referencias

Un dex referencia sus métodos, campos, tipos y cadenas con índices de 16 bits: la segunda unidad de
un 21c, un 22c y un 35c es un u16, y el dexer rechaza el índice que no cabe en vez de recortarlo
(`apkc/dexinstr.go`, la guarda de `indice`). El techo es del fichero, no del bucket, así que el
dexer reparte las clases en `classes.dex`, `classes2.dex`, … (`apkc/reparto.go`): mide las tablas
del grupo antes de ensamblarlo y lo cierra cuando la siguiente clase lo sacaría del techo. El
comando que lo reproduce es

```sh
PROBAR_LIMITE=1 sh banco/validar-apk/validar.sh
```

y exige rc 0, `app.apk`, los tres dex sin huecos, la firma v1/v2/v3, `zipalign`, ningún dex por
encima de 65 536 `method_id`, las 700 clases del jar repartidas sin perder ni duplicar ninguna y un
método invocado en un dex de dependencias y definido en el otro. Sin el reparto, el build se para
con `Lgrande/C0;<init>: at pc 1, invokespecial: the method index does not fit a 16-bit index` y no
deja APK.

## El jar real (medido)

- **El jar real dexa.** `android-support-v4.jar` (648 327 B, 520 clases) entra y sale verificado: el
  dexado da `520 clases de 1 dependencias -> 526856 bytes`, el APK lleva `classes.dex` (la app) y
  `classes2.dex` (el jar), `apksigner` da la v1, la v2 y la v3 en `true`, `zipalign` pasa y
  `classes.dex` invoca `Landroid/support/v4/text/TextUtilsCompat;.htmlEncode`, que define
  `classes2.dex`. El comando es

  ```sh
  JAR_REAL=/ruta/android-support-v4.jar sh banco/validar-apk/validar.sh
  ```

  La evidencia está en `docs/evidencia.md` §225 y el inventario del jar
  (`python3 banco/inventario-jar/inventario.py`) da 520 clases y 0 rechazadas. Historial: este pase
  salía con 3 en `goto/16` (cerrado en `295ac58`) y después en un `i2l` con el registro v17 de
  `GestureDetectorCompat$GestureDetectorCompatImplBase.onTouchEvent` (cerrado en `6a23468`); los
  tres síntomas que tapaba el desorden de `try`, en `dd50e71`, `c106f6a` y `64a7da9`. Nada de eso
  es el estado vigente: hoy el pase valida el APK en vez de nombrar un rechazo.

Este guion no corre gate, chromium, red ni publicación.
