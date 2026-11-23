# aab-multidex — el `.aab` multi-dex, medido con bundletool

Cierra la última frontera del mapa: un **app bundle con más de un dex** derivado por `bundletool`
a un APK. `apkc build --linker go --compiler go --signer go --aab` arma el bundle sin bundletool
ni aapt2 (`apkc/aab_go.go`), y aquí se comprueba con bundletool, que no comparte código con apkc,
en los dos caminos que producen varios dex: el reparto del dex **de la app** (`reparto.go`, §228) y
el de **dependencias**, que `0237e23` («fix(aab): ensambla los dex del aab desde el plan correcto»)
desbloqueó.

## Comando

```sh
# el reparto del dex de la app: classes.dex + classes2.dex
sh banco/aab-multidex/aab-multidex.sh

# y además los jars: classes.dex + classes2.dex…classes4.dex
sh banco/aab-multidex/aab-multidex.sh --dependencias

# la misma activación por entorno, para quien la prefiera
PROBAR_DEPENDENCIAS=1 sh banco/aab-multidex/aab-multidex.sh
```

El caso 2 se activa de las dos formas. El flag existe además de la variable porque un harness que
sanea el entorno al invocar `sh <fichero>` deja `PROBAR_DEPENDENCIAS` fuera del hijo; `--dependencias`
viaja en `argv` y no depende de eso.

Variables: `APKC` (binario; si falta se compila en `$HOME/.cache/apkc-web/aab-multidex`),
`APKC_ANDROID_JAR` o `ANDROID_JAR`, `JAVA_HOME`, `APKC_WEB_CACHE`, `KEYSTORE`, `BUNDLETOOL` (jar o
ejecutable; si falta se busca en el `PATH` y en `$ANDROID_HOME/bundletool`) y `PROBAR_DEPENDENCIAS`.
Los artefactos van a `$HOME/.cache/apkc-web/aab-multidex`: no se versiona ningún binario (regla 2).
No usa serial, ni `adb`, ni aparato.

Códigos de salida: `0` en verde; `1` si un paso o un oráculo falla; `2` si no hay bundletool.

## Qué mide

Los dos casos pasan por el mismo oráculo:

- los nombres de los dex del `.aab` son exactamente los esperados: contiguos desde `classes.dex`,
  sin huecos y en ese orden (`sort -V`, para que `classes2.dex` vaya antes que un eventual
  `classes10.dex`);
- `bundletool validate --bundle` con `rc 0` y los dex en su informe del módulo `base`;
- `bundletool build-apks` en los modos `default` y `universal`: el `splits/base-master.apk` y el
  `universal.apk` llevan **los mismos** dex que el `.aab` (nombres y conjunto);
- `apksigner verify --verbose` (v1, v2 y v3 en `true`), `zipalign -c -p 4` y `dexdump -f` de cada
  dex derivado, con ningún dex por encima de 65 536 `method_id`;
- un cruce real entre ficheros: un método que `classes.dex` **invoca** y que otro dex **define**.

**Caso 1, el dex de la app**: 700 clases de 100 métodos en `src/grande`, 70 000 métodos que no
caben en un dex (el mismo proyecto con el que el §228 midió el reparto en el APK). El bundle sale
con `classes.dex` + `classes2.dex`; el cruce es un método `grande/Cn.m0` que `classes.dex` invoca
y `classes2.dex` define. El par se busca en vez de fijar la clase, porque dónde corta el reparto no
lo decide el guion.

**Caso 2, dex de dependencias** (`--dependencias`): veintidós jars de una clase cada uno (el mismo
artefacto de `banco/validar-apk`), que el dexado reparte en tres buckets. El bundle sale con
`classes.dex` y `classes2.dex`…`classes4.dex`, y el cruce son tres clases —`Lsaludo/Saludo;`,
`Ldep/D0;` y `Ldep/D20;`— que `classes.dex` invoca y que definen otros dex, ninguna en el suyo.

## Medido el 2026-11-23 (`0237e23` para `apkc/`)

Caso 1, `sh banco/aab-multidex/aab-multidex.sh` (`rc 0`, extracto de la salida; el comando imprime
también las líneas de `[timing]`, la cabecera del bundle y el `-- apksigner verify --verbose`):

```
caso 2      inactivo (--dependencias o PROBAR_DEPENDENCIAS=1)
[build] 9 fuentes de Java en Go -> 703 clases
[build] 703 clases -> 3025352 bytes en 2 dex
-- dex de el .aab
   classes.dex classes2.dex 
   nombres y orden ok
== [app] bundletool validate --bundle
Feature module: base
	File: dex/classes.dex
	File: dex/classes2.dex
== [app] bundletool build-apks --mode=default
-- contenido del .apks
   toc.pb
   splits/base-master.apk
   splits/base-master_2.apk
   splits/base-master_3.apk
   standalones/standalone.apk
-- dex de base-master.apk
   classes.dex classes2.dex 
   nombres y orden ok
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
-- zipalign -c -p 4
   zipalign ok
   classes2.dex (251112 B, 5756 method_id): Lgrande/C680; Lgrande/C681; Lgrande/C682; Lgrande/C683; 
   classes.dex (2774240 B, 65459 method_id): Lcom/ejemplo/aabmultidex/MainActivity; …; Lgrande/C0; 
   classes2.dex: 5756
   classes.dex: 65459
-- el cruce entre classes.dex y classes2.dex
   classes.dex invoca grande/C680;.m0 y classes2.dex define Lgrande/C680;
```

En el modo `universal` se repite el mismo cuadro: `classes.dex classes2.dex`, `Verifies`,
`zipalign ok` y el mismo cruce.

Caso 2, `sh banco/aab-multidex/aab-multidex.sh --dependencias` (`rc 0`, extracto de la salida):

```
caso 2      activo (--dependencias o PROBAR_DEPENDENCIAS=1)
== caso 2: 22 jars en tres buckets, classes.dex + classes2.dex…classes4.dex
   22 jars de una clase
[build] dexed 22 jars in 3 buckets, 3 of them at a time
[build] 3 clases -> 1984 bytes en build/classes.dex
[build] build/app.aab ensamblado desde el enlazador Go, sin aapt2 ni bundletool
[build] signing aab
-- dex de el .aab con dependencias
   classes.dex classes2.dex classes3.dex classes4.dex 
   nombres y orden ok
== [deps] bundletool validate --bundle
Feature module: base
	File: dex/classes.dex
	File: dex/classes2.dex
	File: dex/classes3.dex
	File: dex/classes4.dex
== [deps] bundletool build-apks --mode=default
-- dex de base-master.apk
   classes.dex classes2.dex classes3.dex classes4.dex 
   nombres y orden ok
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
-- zipalign -c -p 4
   zipalign ok
   classes2.dex (1396 B, 15 method_id): Ldep/D12; Ldep/D15; Ldep/D18; Ldep/D1; 
   classes3.dex (1400 B, 15 method_id): Ldep/D10; Ldep/D13; Ldep/D16; Ldep/D19; 
   classes4.dex (1876 B, 20 method_id): Ldep/D0; Ldep/D11; Ldep/D14; Ldep/D17; 
   classes.dex (1984 B, 17 method_id): Lcom/ejemplo/aabmultidex/MainActivity; …; R; 
   classes2.dex: 15
   classes3.dex: 15
   classes4.dex: 20
   classes.dex: 17
-- el cruce entre classes.dex y los dex de dependencias
   classes.dex invoca Lsaludo/Saludo; lo define classes4.dex
   classes.dex invoca Ldep/D0; lo define classes4.dex
   classes.dex invoca Ldep/D20; lo define classes2.dex
OK caso 2: el .aab con dex de dependencias deriva un base-master.apk y un universal.apk
con los cuatro dex, y la firma y los oráculos pasan.
```

El modo `universal` repite el cuadro sobre `universal.apk`: los cuatro dex, `Verifies`,
`zipalign ok` y los mismos tres cruces.

`PROBAR_DEPENDENCIAS=1 sh banco/aab-multidex/aab-multidex.sh` da la misma salida con
`caso 2      activo` (medido, `rc 0`). Las dos formas activan exactamente el mismo caso.

## El bloqueo que cerró `0237e23`

Hasta ese parche el caso 2 no llegaba a firmarse:

```
[build] no se pudo firmar el .aab: build/app.aab lists base/dex/classes.dex twice, and a signature
        cannot say which of them the platform is meant to read
```

El ensamblador recorría `build/` entero con `ficherosDeArbol` y nombraba cada dex con
`filepath.Base`, así que los `classes.dex` que el dexado de dependencias deja en
`build/dexcache/…` colisionaban con el de la app. `0237e23` cambia la firma de `buildAAB` y de
`ensamblarAABDesdeGo` para recibir los `depDexes` y numerar los dex con `dexEntries`, la misma
lista que empaqueta el APK. El caso 1 nunca pasó por ese camino: `reparto.go` deja
`build/classes.dex` y `build/classes2.dex` en la raíz de `build/`.
