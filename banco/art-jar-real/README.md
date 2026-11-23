# art-jar-real — el jar real, ejecutado en ART

`banco/validar-apk/` demuestra que el APK de `android-support-v4.jar` pasa `apksigner`, `zipalign` y
`dexdump` (§225). Eso no es ejecución: `dexdump` desarma el fichero. Este banco mide lo que falta,
con un aparato real (**Droidloom**, el serial de red `10.177.0.2:5555`): instala el APK, arranca la
actividad y comprueba que ART ejecuta `Landroid/support/v4/text/TextUtilsCompat;.htmlEncode`, un
método que **solo** existe en `classes2.dex`, el dex que el dexer emitió para el jar. La app no lo
define en ningún sitio.

## Comando

```sh
sh banco/art-jar-real/art-jar-real.sh

# el control que aísla el fallo (no mide nuestro dexer: el dex de dependencias lo hace d8)
CONTROL_D8=1 sh banco/art-jar-real/art-jar-real.sh

# la falsificación: exige que el test diga FALLO con un dex inválido
APK=/ruta/app.apk FALSIFICAR=1 sh banco/art-jar-real/art-jar-real.sh
```

Variables: `SERIE` (por defecto `10.177.0.2:5555`; el guion **solo** usa ese serial, nunca el USB),
`APKC` (binario; si falta se compila desde `apkc/`), `JAR_REAL` (por defecto
`~/Downloads/ApkShare-main/jar/android-support-v4.jar`), `APK`, `ANDROID_HOME`
(`~/opt/android-sdk`), `APKC_WEB_CACHE`. Los artefactos —proyecto, keystore, logcat, APK del
control, APK falsificado— van a `$HOME/.cache/apkc-web/art-jar-real`: no se versiona ningún APK ni
binario.

## Qué exige, con el proceso aislado por `logcat --pid`

1. `install` en `Success` y `am start -W` en `Status: ok`;
2. ni «Verification rejected», ni «Failure to verify», ni «CLASS_ISPREVERIFIED», ni «dex2oat» en las
   líneas del proceso: ART acepta el dex sin rechazarlo ni mandarlo a recompilar;
3. el `Log.d` de la actividad con el resultado exacto de `htmlEncode("<b>apkc & cia</b>")`, que es
   `&lt;b&gt;apkc &amp; cia&lt;/b&gt;`;
4. la UI lo pinta (`uiautomator dump`), no solo el log.

## Resultado de hoy: FALLO, y no es del guion

Árbol `66badcd`, APK construido con `apkc build --linker go --compiler go --signer go`. Instala y
arranca, y ahí se acaba:

```
== install -r
...Performing Streamed Install
Success

== am start -W
Starting: Intent { cmp=com.ejemplo.artjar/.MainActivity }
Status: ok
...
== logcat del proceso (filtros)
   sin «Verification rejected»
   | 11-23 08:43:32.860 85523 19987 W .ejemplo.artjar: Failure to verify dex file '/data/app/~~MXSAXg2zCEEeNq46Ii6W_Q==/com.ejemplo.artjar-RogJluEx0DG8SZnSytSVIw==/base.apk!classes2.dex': unexpected static field initial value type: 'I' vs 'Z'
FALLO: el log del proceso lleva «Failure to verify»
FALLO: el APK del jar real no pasa la prueba
```

Con el proceso muerto antes del `pidof` el guion imprime el final del logcat y el mismo FALLO; en la
misma actividad ART deja `java.lang.NoClassDefFoundError: Failed resolution of:
Landroid/support/v4/text/TextUtilsCompat;` y `ClassNotFoundException`: la clase que la app necesita
no se resuelve del dex que se acaba de rechazar.

### La causa: 17 `encoded_value` con el tipo del campo equivocado

ART comprueba `class_defs.static_values_off` al cargar el fichero y exige que el `encoded_value` de
una constante tenga el tipo del descriptor del campo; si no, rechaza el dex con «Failure to verify
dex file». Nuestro dexer escribe toda constante entera como `VALUE_INT` sin mirar el campo
(`apkc/dexarma.go:613`, `case cpInteger`), y `dexdump` no lo comprueba: el fichero pasa el oráculo
del banco y ART lo rechaza.

```sh
$ unzip -o app.apk classes2.dex -d "$HOME/.cache/apkc-web/art-jar-real/x"
$ python3 banco/art-jar-real/valores_estaticos.py "$HOME/.cache/apkc-web/art-jar-real/x/classes2.dex"
valores estáticos leídos: 509
con un tipo que no casa con el campo: 17
  Landroid/support/v4/app/FragmentManagerImpl;                             DEBUG                        campo Z    valor int
  Landroid/support/v4/app/FragmentManagerImpl;                             HONEYCOMB                    campo Z    valor int
  Landroid/support/v4/app/FragmentPagerAdapter;                            DEBUG                        campo Z    valor int
  Landroid/support/v4/app/FragmentStatePagerAdapter;                       DEBUG                        campo Z    valor int
  Landroid/support/v4/app/LoaderManagerImpl;                               DEBUG                        campo Z    valor int
  Landroid/support/v4/content/AsyncTaskLoader;                             DEBUG                        campo Z    valor int
  Landroid/support/v4/content/LocalBroadcastManager;                       DEBUG                        campo Z    valor int
  Landroid/support/v4/text/BidiFormatter;                                  LRE                          campo C    valor int
  Landroid/support/v4/text/BidiFormatter;                                  LRM                          campo C    valor int
  Landroid/support/v4/text/BidiFormatter;                                  PDF                          campo C    valor int
  Landroid/support/v4/text/BidiFormatter;                                  RLE                          campo C    valor int
  Landroid/support/v4/text/BidiFormatter;                                  RLM                          campo C    valor int
  Landroid/support/v4/util/SimpleArrayMap;                                 DEBUG                        campo Z    valor int
  Landroid/support/v4/view/ViewPager;                                      DEBUG                        campo Z    valor int
  Landroid/support/v4/view/ViewPager;                                      USE_CACHE                    campo Z    valor int
  Landroid/support/v4/widget/DrawerLayout;                                 ALLOW_EDGE_LOCK              campo Z    valor int
  Landroid/support/v4/widget/DrawerLayout;                                 CHILDREN_DISALLOW_INTERCEPT  campo Z    valor int
```

Son doce `boolean` en nueve clases y cinco `char` en `BidiFormatter`: javac da la constante como
entero y el dexer la copia tal cual. `dexdump -d` del mismo fichero sale con `rc 0` y sin una línea
de verificación, que es exactamente el hueco que tapa esta prueba.

## El control con d8: el guion y el fixture están bien

`CONTROL_D8=1` deja el mismo APK, el mismo fixture y el mismo `classes.dex` de la app (el nuestro)
pero sustituye `classes2.dex` por el que produce `d8` sobre el jar. **No mide nuestro dexer**; sirve
para separar «el guion no funciona» de «el dex de dependencias está mal».

```
== control: classes2.dex de d8 (no mide el dexer de apkc)
   d8 dejó classes.dex de 502024 bytes
...
== logcat del proceso (filtros)
   sin «Verification rejected»
   sin «Failure to verify»
   sin «CLASS_ISPREVERIFIED»
   sin «dex2oat»

== Log.d de la actividad (tag artjar)
   esperado: &lt;b&gt;apkc &amp; cia&lt;/b&gt;
   | 11-23 08:43:45.553 85622 20086 D artjar  : htmlEncode(<b>apkc & cia</b>) -> &lt;b&gt;apkc &amp; cia&lt;/b&gt;

== uiautomator dump (la UI lo enseña)
   el TextView lleva el htmlEncode del jar

OK: ART ejecuta el jar real — install Success, am start ok, sin rechazo de verificación,
    Log.d con «&lt;b&gt;apkc &amp; cia&lt;/b&gt;» y la UI lo pinta
```

Con eso quedan probados el fixture, el `Log.d`, el `uiautomator dump`, la instalación y el arranque;
lo único que queda fuera es el dex del jar que emite `apkc`.

## La falsificación: el test dice FALLO con un dex inválido

`FALSIFICAR=1` copia el APK bajo prueba, reemplaza `classes2.dex` por sus primeros 4 096 bytes (el
header promete más, así que el dex es inválido para cualquier lector) y lo vuelve a firmar. El test
tiene que decir FALLO; si dijera OK, el guion saldría con 1.

```
== falsificación: APK con classes2.dex truncado
   $HOME/.cache/apkc-web/art-jar-real/falsificado/app.apk (12749 bytes, classes2.dex 4096 B)
...
Performing Streamed Install
Success
...
== logcat del proceso (filtros)
   sin «Verification rejected»
   sin «Failure to verify»
   sin «CLASS_ISPREVERIFIED»
   sin «dex2oat»
   | 11-23 08:45:27.371 85966 20430 E AndroidRuntime: Caused by: java.lang.ClassNotFoundException: android.support.v4.text.TextUtilsCompat
   | 11-23 08:45:27.389 76963 11440 I ActivityManager: Process com.ejemplo.artjar (pid 85966) has died: prcp TOP
FALLO: el proceso de com.ejemplo.artjar no está vivo después del arranque

OK: el test dice FALLO con el dex inválido (falsificación detectada)

== restaura el APK bajo prueba
...
OK: restaurado, el APK bajo prueba vuelve a arrancar y ejecuta
    android.support.v4.text.TextUtilsCompat.htmlEncode
```

La falsificación se corre contra el APK del control de d8, que sí está en verde: con el APK del jar
real es degenerada, porque ese APK ya sale en FALLO (lo dice el apartado anterior y no distinguiría
nada). Volverá a ser informativa cuando el dex del jar pase.

No se usó un binario anterior a `dd50e71` para generar el dex inválido: la caché tiene muchos
binarios de sesiones anteriores, pero ninguno con la procedencia (commit) anotada, así que no se
puede afirmar que uno sea «el anterior a `dd50e71`». El dex truncado no necesita esa procedencia: es
inválido por construcción.

## Qué queda

- **Arreglar el dexer**: `valorEstatico` (`apkc/dexarma.go:611`) tiene que mirar el descriptor del
  campo y emitir `VALUE_BOOLEAN` (0x1f), `VALUE_BYTE` (0x00), `VALUE_SHORT` (0x02) o `VALUE_CHAR`
  (0x03) para las constantes enteras de esos campos, como hace `d8`. Es un cambio en `apkc/`, que
  este frente no toca. Después, `sh banco/art-jar-real/art-jar-real.sh` debe salir en verde.
- **Ampliar el oráculo**: el inventario y `dexdump` no ven este defecto. La comprobación de
  `valores_estaticos.py` debería entrar en el gate del dex (o en `banco/inventario-jar/`). El
  defecto está en cómo se codifica el valor, no en el jar: cualquier campo `boolean`/`char` con
  constante entera lo dispara (no se ha medido aquí con otro APK, solo se lee del código).
- La fase del jar real sigue **pendiente**: dexdump verifica, ART no ejecuta (§225 queda como está).
