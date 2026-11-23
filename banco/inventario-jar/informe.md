### J-bis: inventario de límites restantes

Jar medido: `/home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar` (648327 bytes)

- Clases recorridas: 520
- Clases que el dexer rechaza: 0
- Tipos de límite: 0

| Límite | Clases | Métodos | Primer caso |
| --- | ---: | ---: | --- |

Cada clase se dexó en su propia invocación de `apkc dex`: el conteo de clases es exacto. El de métodos es la primera que falla en cada clase, porque el ensamblado se detiene ahí; es una cota inferior, no el total del jar.

#### Oráculo d8/dexdump

Comando del oráculo:

```sh
/home/leonisauro/opt/android-sdk/build-tools/36.0.0/d8 --min-api 21 --lib /home/leonisauro/opt/android-sdk/platforms/android-34/android.jar --output $CACHE/trabajo-f603e4ac3529/d8 /home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar
```

La corrida usa un directorio privado por proceso; `$CACHE` es el directorio de caché del inventario (`$XDG_CACHE_HOME/apkc-web/inventario-jar` o `$HOME/.cache/apkc-web/inventario-jar`).

d8 dexa el jar completo con rc 0: `classes.dex` de 563612 bytes.


Comando que lo reproduce:

```sh
tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py
```
