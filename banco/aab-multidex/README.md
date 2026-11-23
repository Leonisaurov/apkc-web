# aab-multidex — el `.aab` multi-dex, medido con bundletool

Cierra la última frontera del mapa: un **app bundle con más de un dex** derivado por `bundletool`
a un APK. `apkc build --linker go --compiler go --signer go --aab` arma el bundle sin bundletool
ni aapt2 (`apkc/aab_go.go`); lo que faltaba era comprobarlo con bundletool, que no comparte código
con apkc, y ver el `.aab` ya no con un dex sino con los que deja el reparto del dex de la app
(`reparto.go`, §228).

## Comando

```sh
sh banco/aab-multidex/aab-multidex.sh

# añade el caso de los jars (dex de dependencias en tres buckets): hoy se para en el ensamblador
PROBAR_DEPENDENCIAS=1 sh banco/aab-multidex/aab-multidex.sh
```

Variables: `APKC` (binario; si falta se compila en `$HOME/.cache/apkc-web/aab-multidex`),
`APKC_ANDROID_JAR` o `ANDROID_JAR`, `JAVA_HOME`, `APKC_WEB_CACHE`, `KEYSTORE` y `BUNDLETOOL` (jar o
ejecutable; si falta se busca en el `PATH` y en `$ANDROID_HOME/bundletool`). Los artefactos van a
`$HOME/.cache/apkc-web/aab-multidex`: no se versiona ningún binario (regla 2). No usa serial, ni
`adb`, ni aparato.

Códigos de salida: `0` en verde; `1` si un paso o un oráculo falla; `2` si no hay bundletool — en
ese caso imprime el motivo, el comando de instalación y deja la frontera abierta.

## Qué mide

El caso es el reparto del dex **de la app**: 700 clases de 100 métodos en `src/grande`, 70 000
métodos que no caben en un dex. En el bundle los dos dex quedan en `base/dex/` y el guion exige:

- `base/dex/classes.dex` y `base/dex/classes2.dex`, contiguos y en ese orden (sin `sort` del
  sistema: `sort -V` para que `classes2.dex` vaya antes que un eventual `classes10.dex`);
- `bundletool validate --bundle` con `rc 0` y los dos dex en su informe del módulo `base`;
- `bundletool build-apks` en los modos `default` y `universal`: el `splits/base-master.apk` y el
  `universal.apk` llevan **los mismos** dex que el `.aab` (nombres y conjunto);
- `apksigner verify --verbose` (v1, v2 y v3 en `true`), `zipalign -c -p 4` y `dexdump -f` de cada
  dex derivado, con ningún dex por encima de 65 536 `method_id`;
- el cruce del reparto: un método `grande/Cn.m0` que `classes.dex` **invoca** y `classes2.dex`
  **define**. El par se busca en vez de fijar la clase, porque dónde corta el reparto no lo decide
  el guion.

## Medido el 2026-11-23 (árbol `106e17d`)

Detección y build:

```
apkc        /home/leonisauro/.cache/apkc-web/aab-multidex/apkc
android     /home/leonisauro/opt/android-sdk/platforms/android-34/android.jar
oráculos    /home/leonisauro/opt/android-sdk/build-tools/36.0.0/apksigner · …/zipalign · …/dexdump
bundletool  /home/leonisauro/opt/android-sdk/bundletool/bundletool-all-1.18.3.jar
javac       /home/leonisauro/opt/jdk21/bin
1.18.3
[build] 9 fuentes de Java en Go -> 703 clases
[build] 703 clases -> 3025352 bytes en 2 dex
[build] build/app.aab ensamblado desde el enlazador Go, sin aapt2 ni bundletool
[build] signing aab
-- dex dentro del .aab
   classes.dex classes2.dex 
   nombres y orden ok
```

bundletool como oráculo:

```
== bundletool validate --bundle
App Bundle information
------------
Feature modules:
	Feature module: base
		File: dex/classes.dex
		File: dex/classes2.dex

== bundletool build-apks --mode=default
-- contenido del .apks
   toc.pb
   splits/base-master.apk
   splits/base-master_2.apk
   splits/base-master_3.apk
   standalones/standalone.apk
-- dex de base-master.apk
   classes.dex classes2.dex 
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
-- zipalign -c -p 4
   zipalign ok
-- dexdump de cada dex derivado
   classes2.dex (251112 B, 5756 method_id): Lgrande/C680; Lgrande/C681; Lgrande/C682; Lgrande/C683; 
   classes.dex (2774240 B, 65459 method_id): Lcom/ejemplo/aabmultidex/MainActivity; …; Lgrande/C0; 
-- method_id de cada dex (techo 65 536)
   classes2.dex: 5756
   classes.dex: 65459
-- el cruce entre classes.dex y classes2.dex
   classes.dex invoca grande/C680;.m0 y classes2.dex define Lgrande/C680;
```

El modo `universal` repite el mismo cuadro sobre `universal.apk`: los dos dex, `Verifies`,
`zipalign ok` y el mismo cruce en `C680`. Los `.apks` derivados llevan además los `base-master_2.apk`
y `base-master_3.apk` de configuración (densidad/idioma) sin dex, y el `standalone.apk`: no se
comprueban porque el reparto del bundle no pasa por ellos.

## Lo que sigue abierto: el dex de dependencias en el `.aab`

`PROBAR_DEPENDENCIAS=1` arma el mismo caso de `banco/validar-apk` (veintidós jars, tres buckets) y
el `.aab` **no se puede firmar**:

```
[build] dexed 22 jars in 3 buckets, 3 of them at a time
[build] build/app.aab ensamblado desde el enlazador Go, sin aapt2 ni bundletool
[build] signing aab
[build] no se pudo firmar el .aab: build/app.aab lists base/dex/classes.dex twice, and a signature
        cannot say which of them the platform is meant to read
-- el bundle que dejó (antes de firmar)
   base/dex/classes.dex
   base/dex/classes.dex
   base/dex/classes.dex
   base/dex/classes.dex
```

La causa está acotada: `ensamblarAABDesdeGo` (`apkc/aab_go.go:311`) recorre `build/` **entero** con
`ficherosDeArbol` (`apkc/aab_go.go:369`) y nombra cada entrada con `filepath.Base`
(`apkc/aab_go.go:379`), así que los `classes.dex` que el dexado de dependencias deja en
`build/dexcache/deps-go-…/` colisionan con el `build/classes.dex` de la app. El sitio donde se
sabe el reparto correcto es `packagePlan` (`apkc/build.go:1676`), que numera los dex con
`dexEntryName` a partir de los `depDexes`; el ensamblador no lo recibe: `apkc/aab.go:25` le pasa
solo `"build"`. La firma lo rechaza en `apkc/sign.go:309`. Arreglarlo exige pasar el plan (o los
`depDexes`) a `buildAAB`/`ensamblarAABDesdeGo`, y por eso no se hace aquí: hay frentes abiertos en
`apkc/`.

El caso del dex de la app no pasa por ese camino —`reparto.go` deja `build/classes.dex` y
`build/classes2.dex` en la raíz de `build/` y no hay `dexcache/` con dex porque el proyecto no
tiene jars—, y es el que este guion da por medido.
