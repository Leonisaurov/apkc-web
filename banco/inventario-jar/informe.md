### J-bis: inventario de límites restantes

Jar medido: `/home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar` (648327 bytes)

- Clases recorridas: 520
- Clases que el dexer rechaza: 22
- Tipos de límite: 4

Primer fallo de `apkc dex` sobre las clases a la vez:

```
[dex] Landroid/support/v4/app/FragmentActivity;onCreateView: at pc 195, getfield: no low register is free to carry a high slot down 
```

| Límite | Clases | Métodos | Primer caso |
| --- | ---: | ---: | --- |
| registro bajo ocupado | 12 | 12 | Landroid/support/v4/app/FragmentActivity;.onCreateView pc 195 · getfield |
| presupuesto de temporales | 5 | 5 | Landroid/support/v4/view/GravityCompat$GravityCompatImplJellybeanMr1;.apply pc 13 · invokestatic |
| registro fuera de formato | 3 | 3 | Landroid/support/v4/util/TimeUtils;.formatDurationLocked pc 340 · iconst_0 |
| tipo no internado por el colector | 2 | 2 | Landroid/support/v4/app/ActionBarDrawerToggleHoneycomb$SetIndicatorInfo;.<init> pc 5 · ldc_w |

Cada clase se dexó en su propia invocación de `apkc dex`: el conteo de clases es exacto. El de métodos es la primera que falla en cada clase, porque el ensamblado se detiene ahí; es una cota inferior, no el total del jar.

#### Equivalente DEX propuesto

| Límite | Forma que falta |
| --- | --- |
| registro bajo ocupado | un plan de registros con desalojo de un local a la pila de temporales; d8 resuelve el mismo método moviendo el valor a otro registro libre antes del uso. |
| presupuesto de temporales | subir la base del reparto por encima de `max_stack` de javap cuando el bloque `/range` necesita más temporales; el bucle de `codeItemConPc` ya sube la base cuando el pico lo pide, y aquí el límite es el paso de medida. |
| registro fuera de formato | las formas `/from16` (move-object/from16 0x08, move/from16 0x02, …) y 22x, que ya están medidas para otras familias; falta aplicarlas a `aload_*`, `iload_*`, `astore_*` y sus primas. |
| tipo no internado por el colector | `const-class` (0x1c, 21c) con el type_id internado también desde `ldc_w` de una clase; el colector debe seguir ese operando, no solo `checkcast`/`instanceof`/`new`. |

#### Oráculo d8/dexdump

Comando del oráculo:

```sh
/home/leonisauro/opt/android-sdk/build-tools/36.0.0/d8 --min-api 21 --lib /home/leonisauro/opt/android-sdk/platforms/android-34/android.jar --output $CACHE/trabajo-f603e4ac3529/d8 /home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar
```

La corrida usa un directorio privado por proceso; `$CACHE` es el directorio de caché del inventario (`$XDG_CACHE_HOME/apkc-web/inventario-jar` o `$HOME/.cache/apkc-web/inventario-jar`).

d8 dexa el jar completo con rc 0: `classes.dex` de 563612 bytes.

Equivalente DEX de d8 para **registro bajo ocupado** — `Landroid/support/v4/app/FragmentActivity;.onCreateView`:

```
      name          : 'onCreateView'
      type          : '(Ljava/lang/String;Landroid/content/Context;Landroid/util/AttributeSet;)Landroid/view/View;'
      access        : 0x0001 (PUBLIC)
      code          -
      registers     : 14
      ins           : 4
      outs          : 4
      insns size    : 370 16-bit code units
01ec3c:                                        |[01ec3c] android.support.v4.app.FragmentActivity.onCreateView:(Ljava/lang/String;Landroid/content/Context;Landroid/util/AttributeSet;)Landroid/view/View;
01ec4c: 1a00 120d                              |0000: const-string v0, "fragment" // string@0d12
01ec50: 6e20 b714 b000                         |0002: invoke-virtual {v0, v11}, Ljava/lang/String;.equals:(Ljava/lang/Object;)Z // method@14b7
01ec56: 0a00                                   |0005: move-result v0
01ec58: 3900 0700                              |0006: if-nez v0, 000d // +0007
01ec5c: 6f40 2000 badc                         |0008: invoke-super {v10, v11, v12, v13}, Landroid/app/Activity;.onCreateView:(Ljava/lang/String;Landroid/content/Context;Landroid/util/AttributeSet;)Landroid/view/View; // method@0020
01ec62: 0c00                                   |000b: move-result-object v0
01ec64: 1100                                   |000c: return-object v0
01ec66: 1a00 0f0b                              |000d: const-string v0, "class" // string@0b0f
01ec6a: 1201                                   |000f: const/4 v1, #int 0 // #0
01ec6c: 7230 1912 1d00                         |0010: invoke-interface {v13, v1, v0}, Landroid/util/AttributeSet;.getAttributeValue:(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String; // method@1219
01ec72: 0c00                                   |0013: move-result-object v0
01ec74: 6202 c700                              |0014: sget-object v2, Landroid/support/v4/app/FragmentActivity$FragmentTag;.Fragment:[I // field@00c7
01ec78: 6e30 9100 dc02                         |0016: invoke-virtual {v12, v13, v2}, Landroid/content/Context;.obtainStyledAttributes:(Landroid/util/AttributeSet;[I)Landroid/content/res/TypedArray; // method@0091
01ec7e: 0c02                                   |0019: move-result-object v2
01ec80: 1203                                   |001a: const/4 v3, #int 0 // #0
01ec82: 3900 0600                              |001b: if-nez v0, 0021 // +0006
```

Equivalente DEX de d8 para **presupuesto de temporales** — `Landroid/support/v4/view/GravityCompat$GravityCompatImplJellybeanMr1;.apply`:

```
      name          : 'apply'
      type          : '(IIILandroid/graphics/Rect;IILandroid/graphics/Rect;I)V'
      access        : 0x0001 (PUBLIC)
      code          -
      registers     : 9
      ins           : 9
      outs          : 8
      insns size    : 4 16-bit code units
0363dc:                                        |[0363dc] android.support.v4.view.GravityCompat$GravityCompatImplJellybeanMr1.apply:(IIILandroid/graphics/Rect;IILandroid/graphics/Rect;I)V
0363ec: 7708 1809 0100                         |0000: invoke-static/range {v1, v2, v3, v4, v5, v6, v7, v8}, Landroid/support/v4/view/GravityCompatJellybeanMr1;.apply:(IIILandroid/graphics/Rect;IILandroid/graphics/Rect;I)V // method@0918
0363f2: 0e00                                   |0003: return-void
      catches       : (none)
      positions     :
        0x0000 line=77
        0x0003 line=79
      locals        :
        0x0000 - 0x0004 reg=0 this Landroid/support/v4/view/GravityCompat$GravityCompatImplJellybeanMr1;
        0x0000 - 0x0004 reg=1 gravity I
        0x0000 - 0x0004 reg=2 w I
        0x0000 - 0x0004 reg=3 h I
        0x0000 - 0x0004 reg=4 container Landroid/graphics/Rect;
        0x0000 - 0x0004 reg=5 xAdj I
        0x0000 - 0x0004 reg=6 yAdj I
        0x0000 - 0x0004 reg=7 outRect Landroid/graphics/Rect;
        0x0000 - 0x0004 reg=8 layoutDirection I
      name          : 'apply'
      type          : '(IIILandroid/graphics/Rect;Landroid/graphics/Rect;I)V'
      access        : 0x0001 (PUBLIC)
      code          -
      registers     : 7
      ins           : 7
      outs          : 6
      insns size    : 4 16-bit code units
0363c4:                                        |[0363c4] android.support.v4.view.GravityCompat$GravityCompatImplJellybeanMr1.apply:(IIILandroid/graphics/Rect;Landroid/graphics/Rect;I)V
0363d4: 7706 1909 0100                         |0000: invoke-static/range {v1, v2, v3, v4, v5, v6}, Landroid/support/v4/view/GravityCompatJellybeanMr1;.apply:(IIILandroid/graphics/Rect;Landroid/graphics/Rect;I)V // method@0919
0363da: 0e00                                   |0003: return-void
      catches       : (none)
      positions     :
        0x0000 line=71
        0x0003 line=72
      locals        :
        0x0000 - 0x0004 reg=0 this Landroid/support/v4/view/GravityCompat$GravityCompatImplJellybeanMr1;
        0x0000 - 0x0004 reg=1 gravity I
        0x0000 - 0x0004 reg=2 w I
        0x0000 - 0x0004 reg=3 h I
        0x0000 - 0x0004 reg=4 container Landroid/graphics/Rect;
        0x0000 - 0x0004 reg=5 outRect Landroid/graphics/Rect;
        0x0000 - 0x0004 reg=6 layoutDirection I

    #2              : (in Landroid/support/v4/view/GravityCompat$GravityCompatImplJellybeanMr1;)
```

Líneas que d8 escribe para ese límite:

```
0363ec: 7708 1809 0100                         |0000: invoke-static/range {v1, v2, v3, v4, v5, v6, v7, v8}, Landroid/support/v4/view/GravityCompatJellybeanMr1;.apply:(IIILandroid/graphics/Rect;IILandroid/graphics/Rect;I)V // method@0918
0363d4: 7706 1909 0100                         |0000: invoke-static/range {v1, v2, v3, v4, v5, v6}, Landroid/support/v4/view/GravityCompatJellybeanMr1;.apply:(IIILandroid/graphics/Rect;Landroid/graphics/Rect;I)V // method@0919
```

Equivalente DEX de d8 para **registro fuera de formato** — `Landroid/support/v4/util/TimeUtils;.formatDurationLocked`:

```
      name          : 'formatDurationLocked'
      type          : '(JI)I'
      access        : 0x000a (PRIVATE STATIC)
      code          -
      registers     : 24
      ins           : 3
      outs          : 6
      insns size    : 273 16-bit code units
034770:                                        |[034770] android.support.v4.util.TimeUtils.formatDurationLocked:(JI)I
034780: 0500 1500                              |0000: move-wide/from16 v0, v21
034784: 0202 1700                              |0002: move/from16 v2, v23
034788: 6203 ff02                              |0004: sget-object v3, Landroid/support/v4/util/TimeUtils;.sFormatStr:[C // field@02ff
03478c: 2133                                   |0006: array-length v3, v3
03478e: 3523 0600                              |0007: if-ge v3, v2, 000d // +0006
034792: 2323 3703                              |0009: new-array v3, v2, [C // type@0337
034796: 6903 ff02                              |000b: sput-object v3, Landroid/support/v4/util/TimeUtils;.sFormatStr:[C // field@02ff
03479a: 6204 ff02                              |000d: sget-object v4, Landroid/support/v4/util/TimeUtils;.sFormatStr:[C // field@02ff
03479e: 1303 2000                              |000f: const/16 v3, #int 32 // #20
0347a2: 1605 0000                              |0011: const-wide/16 v5, #int 0 // #0
0347a6: 3107 0005                              |0013: cmp-long v7, v0, v5
0347aa: 3907 1100                              |0015: if-nez v7, 0026 // +0011
0347ae: 1205                                   |0017: const/4 v5, #int 0 // #0
0347b0: d802 02ff                              |0018: add-int/lit8 v2, v2, #int -1 // #ff
0347b4: 3525 0500                              |001a: if-ge v5, v2, 001f // +0005
0347b8: 5003 0405                              |001c: aput-char v3, v4, v5
```

Líneas que d8 escribe para ese límite:

```
034780: 0500 1500                              |0000: move-wide/from16 v0, v21
034784: 0202 1700                              |0002: move/from16 v2, v23
0348c0: 0203 1000                              |00a0: move/from16 v3, v16
03490c: 0214 0500                              |00c6: move/from16 v20, v5
```

Equivalente DEX de d8 para **tipo no internado por el colector** — `Landroid/support/v4/app/ActionBarDrawerToggleHoneycomb$SetIndicatorInfo;.<init>`:

```
      name          : '<init>'
      type          : '(Landroid/app/Activity;)V'
      access        : 0x10000 (CONSTRUCTOR)
      code          -
      registers     : 9
      ins           : 2
      outs          : 3
      insns size    : 90 16-bit code units
01c0a8:                                        |[01c0a8] android.support.v4.app.ActionBarDrawerToggleHoneycomb$SetIndicatorInfo.<init>:(Landroid/app/Activity;)V
01c0b8: 7010 a814 0700                         |0000: invoke-direct {v7}, Ljava/lang/Object;.<init>:()V // method@14a8
01c0be: 1210                                   |0003: const/4 v0, #int 1 // #1
01c0c0: 1201                                   |0004: const/4 v1, #int 0 // #0
01c0c2: 1c02 0800                              |0005: const-class v2, Landroid/app/ActionBar; // type@0008
01c0c6: 1a03 c115                              |0007: const-string v3, "setHomeAsUpIndicator" // string@15c1
01c0ca: 2304 4d03                              |0009: new-array v4, v0, [Ljava/lang/Class; // type@034d
01c0ce: 1c05 3d00                              |000b: const-class v5, Landroid/graphics/drawable/Drawable; // type@003d
01c0d2: 4d05 0401                              |000d: aput-object v5, v4, v1
01c0d6: 6e30 8514 3204                         |000f: invoke-virtual {v2, v3, v4}, Ljava/lang/Class;.getDeclaredMethod:(Ljava/lang/String;[Ljava/lang/Class;)Ljava/lang/reflect/Method; // method@1485
01c0dc: 0c02                                   |0012: move-result-object v2
01c0de: 5b72 4600                              |0013: iput-object v2, v7, Landroid/support/v4/app/ActionBarDrawerToggleHoneycomb$SetIndicatorInfo;.setHomeAsUpIndicator:Ljava/lang/reflect/Method; // field@0046
01c0e2: 1c02 0800                              |0015: const-class v2, Landroid/app/ActionBar; // type@0008
01c0e6: 1a03 c015                              |0017: const-string v3, "setHomeActionContentDescription" // string@15c0
01c0ea: 2304 4d03                              |0019: new-array v4, v0, [Ljava/lang/Class; // type@034d
01c0ee: 6205 8105                              |001b: sget-object v5, Ljava/lang/Integer;.TYPE:Ljava/lang/Class; // field@0581
01c0f2: 4d05 0401                              |001d: aput-object v5, v4, v1
```

Líneas que d8 escribe para ese límite:

```
01c0c2: 1c02 0800                              |0005: const-class v2, Landroid/app/ActionBar; // type@0008
01c0ce: 1c05 3d00                              |000b: const-class v5, Landroid/graphics/drawable/Drawable; // type@003d
01c0e2: 1c02 0800                              |0015: const-class v2, Landroid/app/ActionBar; // type@0008
```


Comando que lo reproduce:

```sh
tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py
```
