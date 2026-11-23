### J-bis: inventario de límites restantes

Jar medido: `/home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar` (648327 bytes)

- Clases recorridas: 520
- Clases que el dexer rechaza: 49
- Tipos de límite: 5

Primer fallo de `apkc dex` sobre las clases a la vez:

```
[dex] Landroid/support/v4/app/BackStackRecord;dump: at pc 0, dump: a goto of -259 code units: the 10t form reaches +-127 and goto/16 is not measured here 
```

| Límite | Clases | Métodos | Primer caso |
| --- | ---: | ---: | --- |
| registro fuera de formato | 37 | 37 | Landroid/support/v4/app/BackStackState;.<init> pc 0 · aload_0 |
| salto goto fuera de alcance | 6 | 6 | Landroid/support/v4/app/BackStackRecord;.dump pc 0 · dump |
| presupuesto de temporales | 3 | 3 | Landroid/support/v4/view/GravityCompat$GravityCompatImplJellybeanMr1;.apply pc 13 · invokestatic |
| registro bajo ocupado | 2 | 2 | Landroid/support/v4/app/NotificationCompatJellybean;.<init> pc 207 · iload |
| tipo no internado por el colector | 1 | 1 | Landroid/support/v4/app/ActionBarDrawerToggleHoneycomb$SetIndicatorInfo;.<init> pc 5 · ldc_w |

Cada clase se dexó en su propia invocación de `apkc dex`: el conteo de clases es exacto. El de métodos es la primera que falla en cada clase, porque el ensamblado se detiene ahí; es una cota inferior, no el total del jar.

#### Equivalente DEX propuesto

| Límite | Forma que falta |
| --- | --- |
| registro fuera de formato | las formas `/from16` (move-object/from16 0x08, move/from16 0x02, …) y 22x, que ya están medidas para otras familias; falta aplicarlas a `aload_*`, `iload_*`, `astore_*` y sus primas. |
| salto goto fuera de alcance | `goto/16` (0x29, 21t) y `goto/32` (0x2a, 30t) para un salto fuera de ±127 unidades; d8 los usa en el mismo método del oráculo. |
| presupuesto de temporales | subir la base del reparto por encima de `max_stack` de javap cuando el bloque `/range` necesita más temporales; el bucle de `codeItemConPc` ya sube la base cuando el pico lo pide, y aquí el límite es el paso de medida. |
| registro bajo ocupado | un plan de registros con desalojo de un local a la pila de temporales; d8 resuelve el mismo método moviendo el valor a otro registro libre antes del uso. |
| tipo no internado por el colector | `const-class` (0x1c, 21c) con el type_id internado también desde `ldc_w` de una clase; el colector debe seguir ese operando, no solo `checkcast`/`instanceof`/`new`. |

#### Oráculo d8/dexdump

Comando del oráculo:

```sh
/home/leonisauro/opt/android-sdk/build-tools/36.0.0/d8 --min-api 21 --lib /home/leonisauro/opt/android-sdk/platforms/android-34/android.jar --output /home/leonisauro/.cache/apkc-web/inventario-jar/trabajo-wonxwn9e/d8 /home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar
```

d8 dexa el jar completo con rc 0: `classes.dex` de 563612 bytes.

Equivalente DEX de d8 para **registro fuera de formato** — `Landroid/support/v4/app/BackStackState;.<init>`:

```
      name          : '<init>'
      type          : '(Landroid/os/Parcel;)V'
      access        : 0x10001 (PUBLIC CONSTRUCTOR)
      code          -
      registers     : 3
      ins           : 2
      outs          : 2
      insns size    : 66 16-bit code units
01e018:                                        |[01e018] android.support.v4.app.BackStackState.<init>:(Landroid/os/Parcel;)V
01e028: 7010 a814 0100                         |0000: invoke-direct {v1}, Ljava/lang/Object;.<init>:()V // method@14a8
01e02e: 6e10 7e01 0200                         |0003: invoke-virtual {v2}, Landroid/os/Parcel;.createIntArray:()[I // method@017e
01e034: 0c00                                   |0006: move-result-object v0
01e036: 5b10 7a00                              |0007: iput-object v0, v1, Landroid/support/v4/app/BackStackState;.mOps:[I // field@007a
01e03a: 6e10 8101 0200                         |0009: invoke-virtual {v2}, Landroid/os/Parcel;.readInt:()I // method@0181
01e040: 0a00                                   |000c: move-result v0
01e042: 5910 7b00                              |000d: iput v0, v1, Landroid/support/v4/app/BackStackState;.mTransition:I // field@007b
01e046: 6e10 8101 0200                         |000f: invoke-virtual {v2}, Landroid/os/Parcel;.readInt:()I // method@0181
01e04c: 0a00                                   |0012: move-result v0
01e04e: 5910 7c00                              |0013: iput v0, v1, Landroid/support/v4/app/BackStackState;.mTransitionStyle:I // field@007c
01e052: 6e10 8301 0200                         |0015: invoke-virtual {v2}, Landroid/os/Parcel;.readString:()Ljava/lang/String; // method@0183
01e058: 0c00                                   |0018: move-result-object v0
01e05a: 5b10 7900                              |0019: iput-object v0, v1, Landroid/support/v4/app/BackStackState;.mName:Ljava/lang/String; // field@0079
01e05e: 6e10 8101 0200                         |001b: invoke-virtual {v2}, Landroid/os/Parcel;.readInt:()I // method@0181
01e064: 0a00                                   |001e: move-result v0
01e066: 5910 7800                              |001f: iput v0, v1, Landroid/support/v4/app/BackStackState;.mIndex:I // field@0078
      name          : '<init>'
      type          : '(Landroid/support/v4/app/FragmentManagerImpl;Landroid/support/v4/app/BackStackRecord;)V'
      access        : 0x10001 (PUBLIC CONSTRUCTOR)
      code          -
      registers     : 11
      ins           : 3
      outs          : 2
      insns size    : 189 16-bit code units
01e0ac:                                        |[01e0ac] android.support.v4.app.BackStackState.<init>:(Landroid/support/v4/app/FragmentManagerImpl;Landroid/support/v4/app/BackStackRecord;)V
01e0bc: 7010 a814 0800                         |0000: invoke-direct {v8}, Ljava/lang/Object;.<init>:()V // method@14a8
01e0c2: 1200                                   |0003: const/4 v0, #int 0 // #0
01e0c4: 54a1 6900                              |0004: iget-object v1, v10, Landroid/support/v4/app/BackStackRecord;.mHead:Landroid/support/v4/app/BackStackRecord$Op; // field@0069
01e0c8: 3801 1000                              |0006: if-eqz v1, 0016 // +0010
01e0cc: 5412 5600                              |0008: iget-object v2, v1, Landroid/support/v4/app/BackStackRecord$Op;.removed:Ljava/util/ArrayList; // field@0056
01e0d0: 3802 0900                              |000a: if-eqz v2, 0013 // +0009
01e0d4: 5412 5600                              |000c: iget-object v2, v1, Landroid/support/v4/app/BackStackRecord$Op;.removed:Ljava/util/ArrayList; // field@0056
01e0d8: 6e10 ea14 0200                         |000e: invoke-virtual {v2}, Ljava/util/ArrayList;.size:()I // method@14ea
01e0de: 0a02                                   |0011: move-result v2
01e0e0: b020                                   |0012: add-int/2addr v0, v2
01e0e2: 5411 5200                              |0013: iget-object v1, v1, Landroid/support/v4/app/BackStackRecord$Op;.next:Landroid/support/v4/app/BackStackRecord$Op; // field@0052
01e0e6: 28f1                                   |0015: goto 0006 // -000f
01e0e8: 52a2 6d00                              |0016: iget v2, v10, Landroid/support/v4/app/BackStackRecord;.mNumOp:I // field@006d
01e0ec: da02 0207                              |0018: mul-int/lit8 v2, v2, #int 7 // #07
01e0f0: b002                                   |001a: add-int/2addr v2, v0
01e0f2: 2322 3903                              |001b: new-array v2, v2, [I // type@0339
```

Equivalente DEX de d8 para **salto goto fuera de alcance** — `Landroid/support/v4/app/BackStackRecord;.dump`:

```
      name          : 'dump'
      type          : '(Ljava/lang/String;Ljava/io/FileDescriptor;Ljava/io/PrintWriter;[Ljava/lang/String;)V'
      access        : 0x0001 (PUBLIC)
      code          -
      registers     : 6
      ins           : 5
      outs          : 4
      insns size    : 5 16-bit code units
01d370:                                        |[01d370] android.support.v4.app.BackStackRecord.dump:(Ljava/lang/String;Ljava/io/FileDescriptor;Ljava/io/PrintWriter;[Ljava/lang/String;)V
01d380: 1210                                   |0000: const/4 v0, #int 1 // #1
01d382: 6e40 3602 2104                         |0001: invoke-virtual {v1, v2, v4, v0}, Landroid/support/v4/app/BackStackRecord;.dump:(Ljava/lang/String;Ljava/io/PrintWriter;Z)V // method@0236
01d388: 0e00                                   |0004: return-void
      catches       : (none)
      positions     :
        0x0000 line=237
        0x0004 line=238
      locals        :
        0x0000 - 0x0005 reg=1 this Landroid/support/v4/app/BackStackRecord;
        0x0000 - 0x0005 reg=2 prefix Ljava/lang/String;
        0x0000 - 0x0005 reg=3 fd Ljava/io/FileDescriptor;
        0x0000 - 0x0005 reg=4 writer Ljava/io/PrintWriter;
        0x0000 - 0x0005 reg=5 args [Ljava/lang/String;

    #13              : (in Landroid/support/v4/app/BackStackRecord;)
      name          : 'dump'
      type          : '(Ljava/lang/String;Ljava/io/PrintWriter;Z)V'
      access        : 0x0001 (PUBLIC)
      code          -
      registers     : 12
      ins           : 4
      outs          : 2
      insns size    : 514 16-bit code units
01d38c:                                        |[01d38c] android.support.v4.app.BackStackRecord.dump:(Ljava/lang/String;Ljava/io/PrintWriter;Z)V
01d39c: 380b da00                              |0000: if-eqz v11, 00da // +00da
01d3a0: 6e20 6e14 9a00                         |0002: invoke-virtual {v10, v9}, Ljava/io/PrintWriter;.print:(Ljava/lang/String;)V // method@146e
01d3a6: 1a00 4711                              |0005: const-string v0, "mName=" // string@1147
01d3aa: 6e20 6e14 0a00                         |0007: invoke-virtual {v10, v0}, Ljava/io/PrintWriter;.print:(Ljava/lang/String;)V // method@146e
01d3b0: 5480 6c00                              |000a: iget-object v0, v8, Landroid/support/v4/app/BackStackRecord;.mName:Ljava/lang/String; // field@006c
01d3b4: 6e20 6e14 0a00                         |000c: invoke-virtual {v10, v0}, Ljava/io/PrintWriter;.print:(Ljava/lang/String;)V // method@146e
01d3ba: 1a00 5f00                              |000f: const-string v0, " mIndex=" // string@005f
01d3be: 6e20 6e14 0a00                         |0011: invoke-virtual {v10, v0}, Ljava/io/PrintWriter;.print:(Ljava/lang/String;)V // method@146e
01d3c4: 5280 6a00                              |0014: iget v0, v8, Landroid/support/v4/app/BackStackRecord;.mIndex:I // field@006a
01d3c8: 6e20 6c14 0a00                         |0016: invoke-virtual {v10, v0}, Ljava/io/PrintWriter;.print:(I)V // method@146c
01d3ce: 1a00 5600                              |0019: const-string v0, " mCommitted=" // string@0056
01d3d2: 6e20 6e14 0a00                         |001b: invoke-virtual {v10, v0}, Ljava/io/PrintWriter;.print:(Ljava/lang/String;)V // method@146e
01d3d8: 5580 6600                              |001e: iget-boolean v0, v8, Landroid/support/v4/app/BackStackRecord;.mCommitted:Z // field@0066
01d3dc: 6e20 7414 0a00                         |0020: invoke-virtual {v10, v0}, Ljava/io/PrintWriter;.println:(Z)V // method@1474
01d3e2: 5280 7100                              |0023: iget v0, v8, Landroid/support/v4/app/BackStackRecord;.mTransition:I // field@0071
01d3e6: 3800 2100                              |0025: if-eqz v0, 0046 // +0021
```

Líneas que d8 escribe para ese límite:

```
01d772: 2900 12ff                              |01eb: goto/16 00fd // -00ee
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

Equivalente DEX de d8 para **registro bajo ocupado** — `Landroid/support/v4/app/NotificationCompatJellybean;.<init>`:

```
      name          : '<init>'
      type          : '(Landroid/content/Context;Landroid/app/Notification;Ljava/lang/CharSequence;Ljava/lang/CharSequence;Ljava/lang/CharSequence;Landroid/widget/RemoteViews;ILandroid/app/PendingIntent;Landroid/app/PendingIntent;Landroid/graphics/Bitmap;IIZZILjava/lang/CharSequence;)V'
      access        : 0x10001 (PUBLIC CONSTRUCTOR)
      code          -
      registers     : 33
      ins           : 17
      outs          : 4
      insns size    : 200 16-bit code units
0290c8:                                        |[0290c8] android.support.v4.app.NotificationCompatJellybean.<init>:(Landroid/content/Context;Landroid/app/Notification;Ljava/lang/CharSequence;Ljava/lang/CharSequence;Ljava/lang/CharSequence;Landroid/widget/RemoteViews;ILandroid/app/PendingIntent;Landroid/app/PendingIntent;Landroid/graphics/Bitmap;IIZZILjava/lang/CharSequence;)V
0290d8: 0800 1200                              |0000: move-object/from16 v0, v18
0290dc: 7601 a814 1000                         |0002: invoke-direct/range {v16}, Ljava/lang/Object;.<init>:()V // method@14a8
0290e2: 2201 0e00                              |0005: new-instance v1, Landroid/app/Notification$Builder; // type@000e
0290e6: 0802 1100                              |0007: move-object/from16 v2, v17
0290ea: 7020 4e00 2100                         |0009: invoke-direct {v1, v2}, Landroid/app/Notification$Builder;.<init>:(Landroid/content/Context;)V // method@004e
0290f0: 5303 0e00                              |000c: iget-wide v3, v0, Landroid/app/Notification;.when:J // field@000e
0290f4: 6e30 6800 3104                         |000e: invoke-virtual {v1, v3, v4}, Landroid/app/Notification$Builder;.setWhen:(J)Landroid/app/Notification$Builder; // method@0068
0290fa: 0c01                                   |0011: move-result-object v1
0290fc: 5203 0600                              |0012: iget v3, v0, Landroid/app/Notification;.icon:I // field@0006
029100: 5204 0700                              |0014: iget v4, v0, Landroid/app/Notification;.iconLevel:I // field@0007
029104: 6e30 6200 3104                         |0016: invoke-virtual {v1, v3, v4}, Landroid/app/Notification$Builder;.setSmallIcon:(II)Landroid/app/Notification$Builder; // method@0062
02910a: 0c01                                   |0019: move-result-object v1
02910c: 5403 0100                              |001a: iget-object v3, v0, Landroid/app/Notification;.contentView:Landroid/widget/RemoteViews; // field@0001
029110: 6e20 5300 3100                         |001c: invoke-virtual {v1, v3}, Landroid/app/Notification$Builder;.setContent:(Landroid/widget/RemoteViews;)Landroid/app/Notification$Builder; // method@0053
029116: 0c01                                   |001f: move-result-object v1
029118: 5403 0c00                              |0020: iget-object v3, v0, Landroid/app/Notification;.tickerText:Ljava/lang/CharSequence; // field@000c
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
