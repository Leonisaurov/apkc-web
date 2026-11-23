### J-bis: inventario de límites restantes

Jar medido: `/home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar` (648327 bytes)

- Clases recorridas: 520
- Clases que el dexer rechaza: 6
- Tipos de límite: 1

Primer fallo de `apkc dex` sobre las clases a la vez:

```
[dex] Landroid/support/v4/view/GestureDetectorCompat$GestureDetectorCompatImplBase;onTouchEvent: at pc 505, i2l: register v17 does not fit a four-bit format (the /range or /from16 form is not measured here) 
```

| Límite | Clases | Métodos | Primer caso |
| --- | ---: | ---: | --- |
| registro fuera de formato | 6 | 6 | Landroid/support/v4/view/GestureDetectorCompat$GestureDetectorCompatImplBase;.onTouchEvent pc 505 · i2l |

Cada clase se dexó en su propia invocación de `apkc dex`: el conteo de clases es exacto. El de métodos es la primera que falla en cada clase, porque el ensamblado se detiene ahí; es una cota inferior, no el total del jar.

#### Equivalente DEX propuesto

| Límite | Forma que falta |
| --- | --- |
| registro fuera de formato | las formas `/from16` (move-object/from16 0x08, move/from16 0x02, …) y 22x, que ya están medidas para otras familias; falta aplicarlas a `aload_*`, `iload_*`, `astore_*` y sus primas. |

#### Oráculo d8/dexdump

Comando del oráculo:

```sh
/home/leonisauro/opt/android-sdk/build-tools/36.0.0/d8 --min-api 21 --lib /home/leonisauro/opt/android-sdk/platforms/android-34/android.jar --output $CACHE/trabajo-f603e4ac3529/d8 /home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar
```

La corrida usa un directorio privado por proceso; `$CACHE` es el directorio de caché del inventario (`$XDG_CACHE_HOME/apkc-web/inventario-jar` o `$HOME/.cache/apkc-web/inventario-jar`).

d8 dexa el jar completo con rc 0: `classes.dex` de 563612 bytes.

Equivalente DEX de d8 para **registro fuera de formato** — `Landroid/support/v4/view/GestureDetectorCompat$GestureDetectorCompatImplBase;.onTouchEvent`:

```
      name          : 'onTouchEvent'
      type          : '(Landroid/view/MotionEvent;)Z'
      access        : 0x0001 (PUBLIC)
      code          -
      registers     : 29
      ins           : 2
      outs          : 5
      insns size    : 778 16-bit code units
035944:                                        |[035944] android.support.v4.view.GestureDetectorCompat$GestureDetectorCompatImplBase.onTouchEvent:(Landroid/view/MotionEvent;)Z
035954: 0800 1b00                              |0000: move-object/from16 v0, v27
035958: 0801 1c00                              |0002: move-object/from16 v1, v28
03595c: 6e10 5e12 0100                         |0004: invoke-virtual {v1}, Landroid/view/MotionEvent;.getAction:()I // method@125e
035962: 0a02                                   |0007: move-result v2
035964: 5403 2903                              |0008: iget-object v3, v0, Landroid/support/v4/view/GestureDetectorCompat$GestureDetectorCompatImplBase;.mVelocityTracker:Landroid/view/VelocityTracker; // field@0329
035968: 3903 0800                              |000a: if-nez v3, 0012 // +0008
03596c: 7100 7512 0000                         |000c: invoke-static {}, Landroid/view/VelocityTracker;.obtain:()Landroid/view/VelocityTracker; // method@1275
035972: 0c03                                   |000f: move-result-object v3
035974: 5b03 2903                              |0010: iput-object v3, v0, Landroid/support/v4/view/GestureDetectorCompat$GestureDetectorCompatImplBase;.mVelocityTracker:Landroid/view/VelocityTracker; // field@0329
035978: 5403 2903                              |0012: iget-object v3, v0, Landroid/support/v4/view/GestureDetectorCompat$GestureDetectorCompatImplBase;.mVelocityTracker:Landroid/view/VelocityTracker; // field@0329
03597c: 6e20 6e12 1300                         |0014: invoke-virtual {v3, v1}, Landroid/view/VelocityTracker;.addMovement:(Landroid/view/MotionEvent;)V // method@126e
035982: d523 ff00                              |0017: and-int/lit16 v3, v2, #int 255 // #00ff
035986: 1264                                   |0019: const/4 v4, #int 6 // #6
035988: 3343 0400                              |001a: if-ne v3, v4, 001e // +0004
03598c: 1213                                   |001c: const/4 v3, #int 1 // #1
03598e: 2802                                   |001d: goto 001f // +0002
```

Líneas que d8 escribe para ese límite:

```
035954: 0800 1b00                              |0000: move-object/from16 v0, v27
035958: 0801 1c00                              |0002: move-object/from16 v1, v28
035a04: 0214 0200                              |0058: move/from16 v20, v2
035a08: 0215 0300                              |005a: move/from16 v21, v3
035a0c: 0218 0400                              |005c: move/from16 v24, v4
035a60: 0214 0200                              |0086: move/from16 v20, v2
```


Comando que lo reproduce:

```sh
tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py
```
