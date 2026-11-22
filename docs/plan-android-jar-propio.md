# Plan: un `android.jar` propio para la pestaña

Estado: generador implementado y validado en local (2026-11-22). El workflow de
GitHub Actions se ha ejecutado una vez y **ha fallado** por una ruta relativa
(run 45059765189); el arreglo está en el worktree y pendiente de push. No hay
run verde: J4 sigue sin cerrar. Este documento conserva la investigación, el
resultado real de J2 y J3 y el diagnóstico del primer intento de J4.

## Resumen ejecutivo

**Viabilidad: sí, con dos condiciones.** Se puede producir un jar de firmas
propio y redistribuible desde fuentes Apache-2.0 de AOSP (`core/api/current.txt`,
`public-final.xml`, `attrs.xml` y `attrs_manifest.xml`), sin copiar el
`android.jar` de la SDK. La condición primera es no usar el binario de la SDK
como base del artefacto publicado: su licencia prohíbe copiarlo, modificarlo y
crear derivados (cláusula 3.4). La segunda es aceptar que el resultado cubre la
API pública y las plantillas del studio, no todo el framework: no habrá APIs
`@hide`, `@SystemApi`, recursos de producto ni paridad byte a byte con el jar de
Google.

Lo medido en local:

- El `android.jar` de API 34 pesa **26.361.808 bytes**, tiene **5.720 clases**
  y **4.644** de ellas son stubs que lanzan `RuntimeException("Stub!")`. El
  resto son interfaces, anotaciones, clases de constantes y 143 enums con
  `values()`/`valueOf()` generados. No hay implementación reutilizable.
- Las cinco plantillas del studio compilan con **15 clases** del framework. Con
  ese subconjunto, `apkc compile` deja los mismos `.class` y `apkc build
  --linker go --compiler go` deja el mismo `classes.dex` y el mismo
  `resources.arsc` que con el jar completo.
- El jar generado con esas 15 clases y la tabla de recursos completa pesa
  **111.673 bytes**; el jar completo, con las 4.873 clases públicas y la misma
  tabla, pesa **2.829.065 bytes**. El jar de solo las clases (sin recursos)
  medía 65.028 bytes; con la tabla de recursos (301.784 bytes) el jar mínimo
  comprimido pasa a 111.673 bytes.
- El `resources.arsc` es el 62 % del jar de la SDK (16.390.624 bytes) y es lo
  único que `apkc/marco.go` necesita para los ids del framework. La tabla
  propia completa mide **301.784 bytes**, con 3.758 recursos y 1.897 atributos,
  y reproduce los 28 ids de §33.
- Los ids públicos se pueden reconstruir desde `public-final.xml`: **2.924 de
  2.926** ids comprobados coinciden con los del jar de la SDK. Discrepa un
  string de producto (`string/safety_protection_display_text`) y falta uno en
  el volcado.

## 0. Resultado de la implementación (2026-11-22)

El generador está en `tools/androidjar/` (módulo Go propio, sin dependencias) y
el workflow en `.github/workflows/android-jar.yml`. No se ha tocado `apkc/` ni
`studio/`.

Lo que se generó y se midió en local, con la etiqueta `android-14.0.0_r75`:

```sh
go -C tools/androidjar run . \
  -in /tmp/androidjar-propio/aosp -out dist/full -api 34 \
  -tag android-14.0.0_r75 \
  -notice /tmp/androidjar-propio/aosp/NOTICE \
  -license tools/androidjar/LICENSE

go -C tools/androidjar run . \
  -in /tmp/androidjar-propio/aosp -out dist/min -api 34 \
  -tag android-14.0.0_r75 \
  -notice /tmp/androidjar-propio/aosp/NOTICE \
  -license tools/androidjar/LICENSE \
  -only 'android.app.Activity,android.content.Context,android.content.ContextWrapper,android.os.Bundle,android.view.ContextThemeWrapper,android.view.Menu,android.view.MenuItem,android.view.MenuItem.OnMenuItemClickListener,android.view.View,android.view.View.OnClickListener,android.widget.TextView,android.widget.Toast,java.lang.CharSequence,java.lang.Object,java.lang.String'
```

| Jar | Clases | Recursos | `resources.arsc` | Tamaño | SHA-256 |
| --- | --- | --- | --- | --- | --- |
| completo | 4.873 | 3.758 | 301.784 B | 2.829.065 B | `211507a880f02c2736939add2d8d990b5b8abfcbcec883f0c0757c4766e0803a` |
| mínimo | 15 | 3.758 | 301.784 B | 111.673 B | `a77c8ce0fb80b0682dea5c8962dc33e1318e2cb44eb2971925be19a9d4492313` |

El jar completo no contiene `res/` ni `NOTICES/` de la SDK: solo clases
generadas, `resources.arsc`, `NOTICE`, `LICENSE` y (en el directorio de salida)
`PROCEDENCIA.md`.

Validación local, con las cinco plantillas del studio en un proyecto de
prueba:

- `apkc compile --android-jar android-stub-34.jar` deja los mismos `.class` que
  el jar de la SDK: `diff -r` sin salida.
- `apkc build --linker go --compiler go` deja `build/resources.arsc` con SHA-256
  `bb0d10a345812e456330a42c4d0eff1983bce55d71bb9f9d831255cd5c5536d0` y
  `build/classes.dex` con
  `966b43c7394fbba0012304e067e1359935c6cfee7dc741d942a71fbebb99bd92`.
  Los dos coinciden con el build hecho contra el jar de la SDK.
- `HOME=… go test -C apkc -run TestMarco` pasa con el jar generado:
  `TestMarcoDelAndroidJar`, `TestMarcoRechaza` y `TestMarcoValoresDeAtributo`.

**Lo que no se ha verificado**: el primer run real falló (§0.2) y el arreglo
del workflow todavía no se ha empujado; no hay artifact ni hash de CI. El
autocompletado y el `e2e.sh` local sí se probaron con el jar generado (§0.3):
171 comprobaciones y 0 fallos en `prueba-pestana.js`, y `e2e: todo en orden`
usando el `apkc` de `emulador-clics` porque el de este worktree no tiene
`--pulsar`. El sitio publicado sigue sin probar.

### 0.1. Intento de ejecución de J4 (2026-11-22)

El entorno sí tiene `gh` autenticado (`gh auth status`: cuenta `Leonisaurov`,
scopes `repo` y `workflow`) y el remoto `origin` apunta a
`https://github.com/Leonisaurov/apkc-web`. El workflow no se pudo lanzar:

```sh
gh workflow list --repo Leonisaurov/apkc-web --all
# gate  active  412858500
# (no aparece android-jar)

gh workflow run android-jar.yml --repo Leonisaurov/apkc-web --ref web
# HTTP 404: workflow android-jar.yml not found on the default branch

git show origin/web:.github/workflows/android-jar.yml
# fatal: path '.github/workflows/android-jar.yml' exists on disk, but not in 'origin/web'

git ls-remote origin web main
# 8e7b014... refs/heads/main
# 10e566f... refs/heads/web
```

El workflow existe en el `web` local desde `7648c8a`, pero `origin/web` está en
`10e566f` y no lo tiene; el `web` local está 39 commits por delante. GitHub
exige que un workflow `workflow_dispatch` esté en la rama por defecto para
poder lanzarlo por nombre, y el de este repo es `main`, que tampoco lo tiene.

**Resultado: J4 no verificada.** No hay número de run, URL, artifact ni hash de
CI que reportar, y no se escribe «CI verde». Para ejecutarlo hace falta un push
que no se ha hecho porque requiere confirmación explícita del usuario:

```sh
# opción mínima: llevar solo el commit del workflow a origin/web
git push origin 7648c8a:web
# después
gh workflow run android-jar.yml --repo Leonisaurov/apkc-web --ref web
gh run watch --repo Leonisaurov/apkc-web
gh run download --repo Leonisaurov/apkc-web --name android-stub-34
```

La alternativa es llevar el workflow a `main` (rama por defecto), que cambia el
alcance del push. Ninguna de las dos se ha ejecutado.

### 0.2. Primer run real de J4: falló (45059765189)

El push del usuario puso `origin/web` en `b6a1634` y el workflow quedó
disponible. El run se lanzó y falló:

- URL: <https://github.com/Leonisaurov/apkc-web/actions/runs/45059765189>
- Run: `45059765189`; job `generar` (ID 137322843571), 22 s.
- Job `verificar`: no llegó a ejecutarse porque `generar` falló.

Log real:

```text
android-stub-34.jar: 4873 clases, 3758 recursos, 1897 atributos
android-stub-34.jar: 15 clases, 3758 recursos, 1897 atributos
cp: cannot stat 'dist/full/android-stub-34.jar': No such file or directory
##[error]Process completed with exit code 1.
```

Causa: el paso usa `go -C tools/androidjar run .` y `-out dist/full`; `go -C`
cambia el directorio de trabajo al módulo, así que el generador escribió en
`tools/androidjar/dist/full`, no en `dist/full` del repositorio. El generador
en sí hizo su trabajo (4.873 y 15 clases), pero el `cp` posterior no encontró
el fichero.

Al reproducir J5 apareció un segundo defecto del generador: al leer
`libcore-current.txt` antes de que `java.lang.*` estuviera en el mapa, algunos
descriptores quedaban como `LString;` en vez de `Ljava/lang/String;`, y
`PrintStream.println(String)` no resolvía. Se añadió `arreglaDescriptor`, que
reescribe los nombres simples dentro de `L...;` una vez leído todo el fichero.
Eso cambia el artefacto: el jar de este árbol es **211507a…** (completo) y
**a77c8ce…** (mínimo), no los 9103b3a…/5d69f0b… del primer intento ni los
56e45ad…/725d8a2… intermedios. Ninguno de esos hashes es un artifact de CI:
el run falló antes de subirlo.

**Arreglo en el worktree**: el paso ahora calcula `OUT="$GITHUB_WORKSPACE/dist"`
y pasa rutas absolutas a `-out`; el job `verificar` usa
`JAR="$GITHUB_WORKSPACE/dist/android-stub-34-min.jar"` y
`SDK_JAR="${ANDROID_HOME:-$ANDROID_SDK_ROOT}/platforms/android-34/android.jar"`.
El YAML vuelve a pasar PyYAML. Falta el push del commit del arreglo y un run
nuevo; sin ese run, J4 sigue **NO verificada**.

### 0.3. J5 local: autocompletado con el jar generado

Se probó la pestaña con el jar completo (`211507a…`, 2.829.065 B) montado como
`ANDROID_JAR`, en un `PREFIX` propio y puertos `18793`/`19347`:

```sh
PREFIX=/tmp/androidjar-propio/prefix-pestana3 CHROME_BIN=/usr/bin/chromium \
APKC_WASM=/tmp/androidjar-propio/apkc-wasm.wasm \
APK_EJEMPLO=… KS_EJEMPLO=$HOME/.android/debug.keystore \
ANDROID_JAR=/tmp/androidjar-propio/sdk-generated/platforms/android-34/android.jar \
STUDIO_SERVIDOR_PORT=18793 STUDIO_PORT=19347 \
node studio/prueba-pestana.js
```

Resultado real: **171 comprobaciones bien, 0 mal**. Entre ellas:

- `el índice del android.jar se lee de OPFS por trozos y sin una sola petición`
  — 3.372 clases indexadas, 496.700 bytes leídos, 0 peticiones.
- `las clases del android.jar completan en el editor` — `Activity` sale de la
  lista.
- `los método públicos de una clase del jar se leen de su .class` — 599
  métodos de `android.view.View`.
- `el jar generado (<5 MB) carga sus clases y R.layout sigue saliendo del
  proyecto` — opciones `R.color`, `R.id`, `R.layout`, `R.string`, `R.style` y
  estado `recursos del proyecto · 5 tipos`.

Para que ese test sirviera con el jar propio se cambiaron dos cosas en
`studio/prueba-pestana.js`: el tamaño esperado ya no es el literal 26361808 del
SDK, sino `JAR_BYTES` leído del fichero, y la comprobación nueva exige que el
jar cargado pese menos de 5 MB y que `R.layout` siga saliendo de los recursos.

El gate completo también se ejecutó. El `apkc` de este worktree no tiene el
flag `--pulsar` que espera el `studio` submodule en `9f9d5c1`, así que se usó el
binario y el wasm del worktree `emulador-clics` (commit `e524c4d`), sin tocar
`apkc/`:

```sh
PREFIX=/tmp/androidjar-propio/prefix5 \
APKC=/tmp/androidjar-propio/apkc-preview \
APKC_WASM=/tmp/androidjar-propio/apkc-preview-wasm.wasm \
ANDROID_JAR=/tmp/androidjar-propio/sdk-generated4/platforms/android-34/android.jar \
sh studio/e2e.sh
```

Salida real:

```text
vista previa: 3 nodos, res/layout/main.xml, … un clic deja "Pulsos: 1" …
pestaña: 171 comprobaciones, apk firmada dentro de chromium de 10320 bytes y verificada
e2e: todo en orden
```

**Frontera**: la pestaña y el `e2e` local están cerrados con el jar generado,
pero el **sitio publicado** no se ha probado con este jar. J5 queda hecha en su
parte local y pendiente en la de publicación.

## 1. Qué contiene el jar que consume la pestaña

Medición sobre `$ANDROID_HOME/platforms/android-34/android.jar`:

```sh
unzip -l "$ANDROID_HOME/platforms/android-34/android.jar" | tail -3
mkdir -p "$TMP/jarprobe"
unzip -q -o "$ANDROID_HOME/platforms/android-34/android.jar" -d "$TMP/jarprobe"
find "$TMP/jarprobe" -name '*.class' | wc -l
find "$TMP/jarprobe" -name '*.class' -printf '%s\n' | awk '{s+=$1} END {print s}'
grep -rl --include='*.class' 'Stub!' "$TMP/jarprobe" | wc -l
```

| Parte | Tamaño | Qué es |
| --- | --- | --- |
| `resources.arsc` | 16.390.624 B | tabla de recursos del paquete `android`; la lee `apkc/marco.go` |
| `res/` | 16.550.290 B | AXML, PNG y XML de recursos; `apkc` no los usa para compilar |
| clases `.class` | 11.601.133 B | 5.720 entradas; firmas y stubs |
| `NOTICES/` | 456.758 B | avisos de licencias de la SDK |
| `assets/` | 230.050 B | datos del framework; irrelevantes para el studio |
| jar comprimido | 26.361.808 B | 13.968 entradas de zip |

Clases, contadas con un lector propio de atributos `Code`
(`$TMP/classstat.py`, reproducible en el anexo):

- **4.644** clases contienen el literal `Stub!`.
- **4.787** tienen al menos un método con atributo `Code`; de ellas, 4.644 son
  stubs. Las **143** restantes son casi todas enums con `values()` y
  `valueOf()` generados por `javac`.
- **933** no tienen `Code`: interfaces, anotaciones y clases de constantes como
  `android.R`.

Conclusión: el jar de la SDK es un contrato de firmas, no una biblioteca de
implementación. Lo único que aporta al studio es (a) descriptores de clase,
campo y método para resolver nombres y (b) la tabla de ids del framework.

### 1.1. Qué lee de verdad el fork

`apkc/compilar.go` y `apkc/javaemisor.go` resuelven solo las entradas que el
fuente nombra; `abrirBuscador` indexa todos los `.class` del zip, pero no los
lee hasta que hacen falta. `apkc/marco.go` lee `resources.arsc` y no toca las
clases.

Con la plantilla base, un bucle que añade la clase que falta en cada error
(`$TMP/closure.py`) llega a **6 clases**:

```
android/app/Activity
android/content/Context
android/content/ContextWrapper
android/os/Bundle
android/view/ContextThemeWrapper
java/lang/Object
```

Con una actividad que ejercita las cinco plantillas del studio (base, botón,
contador, ciclo y menú), el conjunto suficiente medido es de **15 clases**:

```
android/app/Activity
android/content/Context
android/content/ContextWrapper
android/os/Bundle
android/view/ContextThemeWrapper
android/view/Menu
android/view/MenuItem
android/view/MenuItem$OnMenuItemClickListener
android/view/View
android/view/View$OnClickListener
android/widget/TextView
android/widget/Toast
java/lang/CharSequence
java/lang/Object
java/lang/String
```

No es un conjunto mínimo demostrado: es un **conjunto suficiente** medido. Las
seis primeras salieron del bucle; las nueve restantes se fijaron a partir de los
tipos que usan las plantillas y el bucle no necesitó ninguna más.

### 1.2. Prueba de que el subconjunto no cambia la salida

Con un jar que contiene esas 15 clases y el `resources.arsc` completo de la SDK
(usado solo como entrada de medición, no como artefacto):

```sh
"$TMP/apkc" compile --android-jar "$TMP/sub.jar" -o "$TMP/out-min" src
"$TMP/apkc" compile --android-jar "$ANDROID_HOME/platforms/android-34/android.jar" \
  -o "$TMP/out-full" src
diff -r "$TMP/out-min" "$TMP/out-full"          # sin diferencias

"$TMP/apkc" build --linker go --compiler go \
  --android-jar "$TMP/sub.jar" --sdk-version 34 --codename 14 \
  --keystore "$HOME/.android/debug.keystore"
sha256sum build/resources.arsc build/classes.dex
```

Resultado medido:

- `diff -r` no imprime nada: los `.class` son idénticos.
- `build/resources.arsc` da `bb0d10a345812e456330a42c4d0eff1983bce55d71bb9f9d831255cd5c5536d0`
  y `build/classes.dex` da
  `966b43c7394fbba0012304e067e1359935c6cfee7dc741d942a71fbebb99bd92` con el jar
  recortado y con el completo.
- `TestMarcoDelAndroidJar`, `TestMarcoRechaza` y
  `TestMarcoValoresDeAtributo` pasan con el jar recortado. El test busca el jar
  en `$HOME/Android/platforms/android-34/android.jar`; basta apuntar `HOME` a
  un directorio temporal con esa ruta.

## 2. Licencias: qué se puede redistribuir y qué no

### 2.1. La SDK no es Apache-2.0

Fuente: <https://developer.android.com/studio/terms>, la *Android Software
Development Kit License Agreement*. Cláusula 3.4, texto literal:

> Except to the extent required by applicable third party licenses, you may not
> copy (except for backup purposes), modify, adapt, redistribute, decompile,
> reverse engineer, disassemble, or create derivative works of the SDK or any
> part of the SDK.

La cláusula 3.5 añade que los componentes que estén bajo una licencia open
source se rigen **solo** por esa licencia. La pregunta jurídica fina es si el
`android.jar` empaquetado por Google es un «componente open source» a efectos de
3.5. La respuesta prudente es no: el jar es un producto empaquetado de la SDK y
la propia SDK lo distribuye con su EULA. Por eso este plan **no lo usa como
base** del artefacto publicado.

Comprobación de que el prebuilt de AOSP tampoco es Apache-2.0:

```sh
curl -fsSL 'https://android.googlesource.com/platform/prebuilts/sdk/+/refs/heads/main/NOTICE?format=TEXT' \
  | base64 -d | head -5
```

Devuelve `ANDROID SOFTWARE DEVELOPMENT KIT / Terms and Conditions`, es decir,
la misma licencia de SDK. El directorio `prebuilts/sdk/34/public/` contiene
`android.jar` (comprobado con el listado JSON de Gitiles), pero su `NOTICE` es
la EULA. No es una fuente redistribuible.

Lo mismo vale para `prebuilts/fullsdk/platforms/android-XX/android-stubs-src.jar`:
está en el árbol de fullsdk y arrastra la licencia de la SDK. No se usa.

### 2.2. Fuentes Apache-2.0 que sí sirven

| Fuente | URL | Licencia | Qué aporta |
| --- | --- | --- | --- |
| `frameworks/base/core/api/current.txt` | `https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-14.0.0_r75/core/api/current.txt?format=TEXT` | Apache-2.0 (NOTICE del repo) | firmas de la API pública de `android.*`, constantes incluidas |
| `libcore/api/current.txt` | `https://android.googlesource.com/platform/libcore/+/refs/tags/android-14.0.0_r75/api/current.txt?format=TEXT` | Apache-2.0 (NOTICE de `libcore`) | `java.*`, `javax.*` y `org.*`; sin esto no se resuelven `String`, `Object` ni `CharSequence` |
| `core/res/res/values/public-final.xml` | misma etiqueta, `.../core/res/res/values/public-final.xml` | Apache-2.0 (NOTICE de `frameworks/base`) | nombres e ids públicos de recursos |
| `core/res/res/values/attrs.xml` | misma etiqueta | Apache-2.0 (cabecera del propio fichero) | formatos, enums y flags de los atributos |
| `core/res/res/values/attrs_manifest.xml` | misma etiqueta | Apache-2.0 (misma árbol) | atributos del manifiesto |
| `frameworks/base/NOTICE` y `core/res/NOTICE` | Gitiles | Apache-2.0 | texto de licencia y avisos |

`https://source.android.com/docs/setup/about/licenses` lo confirma: «Apache
License, Version 2.0 (Apache 2.0) is the preferred license for AOSP, and the
majority of Android software is licensed with Apache 2.0».

Cifras medidas de esas fuentes:

- `current.txt`: **4.374.046 bytes**, **62.527 líneas**, 139 paquetes, 18.071
  campos `static final` y 17.857 métodos. Cabecera `// Signature format: 2.0`.
- `libcore-current.txt`: **1.337.830 bytes**, **22.029 líneas** y 79 paquetes;
  es la mitad que aporta `java.lang.String`, `java.lang.Object` y
  `java.lang.CharSequence`. Sin ella, el jar generado no compila ni la
  plantilla base.
- `public-final.xml`: **242.391 bytes**, **3.148** etiquetas `<public>`, de las
  cuales 2.941 llevan `id="0x..."` explícito y 207 no. El parseo XML completo
  agrupa **2.925 entradas en 17 tipos**.
- `attrs.xml`: **589.060 bytes**; `attrs_manifest.xml`: 3.957 líneas.

### 2.3. Metalava y el precedente de Robolectric

Metalava es la herramienta oficial de AOSP para generar stubs desde firmas.
Fuente: <https://android.googlesource.com/platform/tools/metalava/>. Su
`Driver.kt` carga `SignatureFileLoader` cuando la entrada acaba en `.txt` y
`--stubs` escribe los `.java`; el propio código avisa de que generar stubs desde
un `TextCodebase` es experimental. El artefacto publicado en Google Maven,
`com.android.tools.metalava:metalava:1.0.0-alpha16`, declara Apache-2.0 y pesa
1.310.958 bytes.

Precedente de redistribución: `org.robolectric:android-all` publica en Maven
Central un jar construido desde fuentes de AOSP bajo Apache-2.0. Su POM dice:

> While the EULA for the Android SDK restricts distribution of those binaries,
> the source code is licensed under Apache 2.0 which allows compiling binaries
> from source and then distributing those versions.

Ese jar pesa 137.666.235 bytes, contiene 48.226 clases y un `resources.arsc` de
16.424.688 bytes. No se sirve tal cual, pero confirma la tesis: **lo que se
distribuye es un build propio de fuentes Apache-2.0, no el binario de la SDK**.

### 2.4. Qué se puede redistribuir

| Artefacto | ¿Redistribuible? | Motivo |
| --- | --- | --- |
| `android.jar` de la SDK | **No** | SDK License 3.4 |
| `prebuilts/sdk/**/android.jar` | **No** | su `NOTICE` es la SDK License |
| `prebuilts/fullsdk/**/android-stubs-src.jar` | **No** | misma licencia de SDK |
| `current.txt`, `public-final.xml`, `attrs*.xml` de `frameworks/base` | **Sí** | Apache-2.0; conservar `LICENSE` y `NOTICE` |
| `libcore/api/current.txt` | **Sí** | Apache-2.0; es la fuente de `java.*` |
| Stubs generados por nosotros desde esos ficheros | **Sí, con condiciones** | obra propia sobre Apache-2.0; incluir aviso de procedencia y NOTICE |
| `org.robolectric:android-all` | **Sí según su POM** | Apache-2.0 con la nota citada; es un build de AOSP, no el jar de la SDK |

**Pregunta abierta legal**: la cláusula 3.5 permite discutir si recortar el
`android.jar` de la SDK es lícito por ser un componente open source. Este plan
no lo asume. Si el usuario quiere esa vía, hace falta una opinión jurídica
externa; no se sostiene solo con la lectura de las cláusulas.

## 3. Viabilidad y arquitectura del generador

### 3.1. Dos artefactos, no uno

El studio usa el jar para dos cosas distintas y conviene separarlas:

1. **`android-stub-min.jar`**: 15 clases medidas + tabla de recursos pública.
   Es lo que necesita `apkc compile` y `apkc build --linker go` para las
   plantillas. Medido: **111.673 bytes**.
2. **`android-stub-full.jar`**: todas las clases públicas de `current.txt` y
   `libcore-current.txt` sin `Code`, para que el autocompletado siga ofreciendo
   las clases y métodos del framework. Medido: **2.829.065 bytes** y 4.873
   clases. Falta probar el autocompletado de la pestaña (J5).

El jar actual de la SDK se sigue aceptando como fichero del usuario para quien
quiera paridad total. El jar propio pasa a ser el camino por defecto del sitio.

### 3.2. Generador de clases

Ruta recomendada: un escritor de `.class` en Go, dentro de `tools/androidjar/`,
que lea `current.txt` (formato de firma 2.0) y emita:

- `this_class`, `super_class` e interfaces.
- Campos con `ACC_PUBLIC`/`ACC_PROTECTED`/`ACC_STATIC`/`ACC_FINAL` y
  `ConstantValue` para los `static final` primitivos y `String`. Sin esos
  valores, `Toast.LENGTH_SHORT`, `View.VISIBLE` y las constantes de `R` no se
  pueden plegar en el llamador.
- Métodos con descriptor, flags y `throws`. Sin `Code`: se marcan `native` o
  `abstract` para que el verificador no exija cuerpo. `apkc` solo lee el
  descriptor; no ejecuta el stub.
- `Signature` genérica opcional. No hace falta para `apkc`; sí mejora el
  autocompletado.
- Anotaciones opcionales. No hacen falta para compilar; se pueden omitir en la
  variante mínima.

Por qué no `javac` directamente: `libcore-current.txt` incluye `java.lang.*` y
`java.util.*`. Compilar esos stubs con el JDK choca con los paquetes del propio
JDK y obliga a `--patch-module` o a un `bootclasspath` especial. Un escritor
propio evita el problema y permite decidir qué metadatos se emiten. Por eso el
generador lee **dos** ficheros: `libcore-current.txt` primero y `current.txt`
después, compartiendo el mapa de clases para que `CharSequence` se convierta en
`java/lang/CharSequence`.

Ruta alternativa, más rápida de arrancar: tomar `org.robolectric:android-all`
(Apache-2.0), filtrar las clases que aparecen en `current.txt` y quitarles el
atributo `Code`. Es una transformación de bytecode, no una compilación, y evita
el conflicto de `java.*`. El riesgo es que el jar de entrada pesa 137 MB y
arrastra clases de terceros; el filtro por `current.txt` lo reduce. **HIPÓTESIS**:
falta medir cuántas clases quedan y cuánto pesa el resultado.

### 3.3. Generador de `resources.arsc`

`apkc/marco.go` necesita, del paquete `android` con id 1:

- los nombres de tipo y las claves;
- el id de cada entrada (`paquete<<24 | tipo<<16 | índice`);
- para los atributos, la máscara de formato y los items enum/flags con su
  nombre referido a un recurso de tipo `id`;
- nada del pool global de valores, que se salta por tamaño.

Por eso no hace falta el `resources.arsc` de 16 MB: basta una tabla con los
recursos públicos y los valores de los atributos. El generador debe:

1. Parsear `public-final.xml` y asignar ids. Las entradas con `id` explícito lo
   conservan; las 207 sin id reciben el siguiente hueco de su tipo. Esta regla
   se midió contra el jar de la SDK: **2.924 de 2.926 ids comprobados
   coinciden**. Discrepa `string/safety_protection_display_text` (predicho
   `0x01040041`, real `0x01dc0005`, recurso de producto) y falta uno en el
   volcado. Ese string se excluye y se documenta.
2. Parsear `attrs.xml` y `attrs_manifest.xml` para el formato y los items de
   cada atributo.
3. Emitir los 23 nombres de tipo del framework, aunque algunos queden sin
   entradas, para que `marco.tipos` conserve el orden y el test de §33 siga
   siendo válido. `public-final.xml` solo cubre 17 tipos.
4. Escribir el `resources.arsc` directamente o usar `aapt2` con placeholders.
   La ruta `aapt2` ya se probó: `aapt2 compile --dir` + `aapt2 link` con
   `package="android"` y `--stable-ids` produce una tabla binaria; la de solo
   atributos mide **177.224 bytes**. Cuidado: `--static-lib` genera
   `resources.pb`, no `resources.arsc`; hay que enlazar como app regular con
   paquete `android`. Y el fichero de ids debe llevar el paquete y espacios:
   `android:attr/theme = 0x01010000`; sin eso, `aapt2` ignora los ids y los
   reasigna en orden.

**HECHO**: el generador escribe la tabla directamente, sin `aapt2`. La tabla
completa mide **301.784 bytes**, declara 23 nombres de tipo, 3.758 recursos y
1.897 atributos. La ruta `aapt2` con placeholders quedó como alternativa y no
es la que usa el código.

```sh
unzip -p dist/android-stub-34-min.jar resources.arsc | wc -c
# 301784
```

### 3.4. Contenido y licencia del jar publicado

```
android-stub-34/
  android.jar
  LICENSE            Apache License 2.0
  NOTICE             AOSP Copyright (C) 2005-2008 The Android Open Source Project
  PROCEDENCIA.md     etiqueta AOSP, ficheros de entrada y sus sha256
  README.md          qué API cubre, qué no, cómo se regenera
```

El `NOTICE` no se copia del jar de la SDK: se toma el de `frameworks/base`, que
es Apache-2.0. Se añade una nota de cambios («este jar contiene stubs generados
por apkc-web a partir de las firmas públicas de AOSP; no es el android.jar de la
SDK»).

## 4. Workflow de GitHub Actions

`.github/workflows/android-jar.yml` (nombre propuesto). Disparadores:
`workflow_dispatch`, `schedule` mensual y `push` que toque
`tools/androidjar/**`.

### 4.1. Job `generar`

```yaml
generar:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: { go-version: '1.22' }
    - name: Fijar la etiqueta de AOSP
      run: echo "AOSP_TAG=android-14.0.0_r75" >> "$GITHUB_ENV"
    - name: Descargar las fuentes Apache-2.0
      run: |
        base="https://android.googlesource.com/platform/frameworks/base/+/refs/tags/$AOSP_TAG"
        mkdir -p "$RUNNER_TEMP/aosp"
        for f in core/api/current.txt \
                 core/res/res/values/public-final.xml \
                 core/res/res/values/attrs.xml \
                 core/res/res/values/attrs_manifest.xml; do
          curl -fsSL "$base/$f?format=TEXT" | base64 -d > "$RUNNER_TEMP/aosp/$(basename "$f")"
        done
        curl -fsSL "https://android.googlesource.com/platform/libcore/+/refs/tags/$AOSP_TAG/api/current.txt?format=TEXT" \
          | base64 -d > "$RUNNER_TEMP/aosp/libcore-current.txt"
        sha256sum "$RUNNER_TEMP"/aosp/* | tee "$RUNNER_TEMP/aosp/SHA256SUMS"
    - name: Generar el jar
      run: go run ./tools/androidjar -api 34 -in "$RUNNER_TEMP/aosp" -out dist
    - name: Verificar el zip y el hash
      run: |
        unzip -t dist/android-stub-34-min.jar
        unzip -t dist/android-stub-34-full.jar
        sha256sum dist/*.jar > dist/SHA256SUMS
    - uses: actions/upload-artifact@v4
      with:
        name: android-stub-34
        path: dist/
```

El workflow **no descarga la SDK** y **no publica ningún fichero de la SDK**.
Si el job de verificación necesita el jar de Google como oráculo, lo instala en
ese job y lo usa solo para comparar; nunca lo sube como artifact.

### 4.2. Job `verificar`

```yaml
verificar:
  needs: generar
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/download-artifact@v4
      with: { name: android-stub-34, path: dist }
    - uses: actions/setup-java@v4
      with: { distribution: temurin, java-version: '17' }
    - name: Instalar la SDK solo como oráculo
      run: |
        yes | "$ANDROID_HOME/cmdline-tools/latest/bin/sdkmanager" \
          "platforms;android-34" "build-tools;34.0.0"
    - name: Compilar apkc
      run: go build -C apkc -o "$RUNNER_TEMP/apkc" .
    - name: Compilar y construir las cinco plantillas con ambos jars
      run: |
        for plantilla in base boton contador ciclo menu; do
          "$RUNNER_TEMP/apkc" create "$RUNNER_TEMP/$plantilla" <<< $'com.myapp\nMy App'
          # el generador de proyectos del studio aplica la variante; aquí se
          # sustituyen los ficheros con los del studio para cada plantilla
          "$RUNNER_TEMP/apkc" build --linker go --compiler go \
            --android-jar dist/android-stub-34-min.jar \
            --sdk-version 34 --codename 14 \
            --keystore "$HOME/.android/debug.keystore"
        done
    - name: Comparar con el jar de la SDK
      run: |
        diff -r "$RUNNER_TEMP/out-min" "$RUNNER_TEMP/out-sdk"
        cmp "$RUNNER_TEMP/min/resources.arsc" "$RUNNER_TEMP/sdk/resources.arsc"
        cmp "$RUNNER_TEMP/min/classes.dex" "$RUNNER_TEMP/sdk/classes.dex"
    - name: TestMarco con el jar generado
      run: |
        mkdir -p "$RUNNER_TEMP/fakehome/Android/platforms/android-34"
        cp dist/android-stub-34-min.jar \
          "$RUNNER_TEMP/fakehome/Android/platforms/android-34/android.jar"
        HOME="$RUNNER_TEMP/fakehome" go test -C apkc -count=1 -run TestMarco -v .
```

El detalle de «sustituir los ficheros con los del studio» se resuelve en J5
extrayendo las plantillas de `studio/app.js` a un fixture compartido, para no
duplicarlas. En J4 el job puede empezar con la plantilla base y las cuatro
variantes copiadas a mano; el criterio es que las cinco pasen antes de cerrar
la fase.

### 4.3. Publicación y consumo

- El artifact `android-stub-34` incluye `android-stub-34-min.jar`,
  `android-stub-34-full.jar`, `SHA256SUMS`, `LICENSE`, `NOTICE` y
  `PROCEDENCIA.md`.
- Un release manual o etiquetado publica los mismos ficheros. El hash va en el
  cuerpo del release y en `SHA256SUMS`.
- `tools/sitio.sh` copia `android-stub-34-min.jar` a `sitio/android-stub.jar`
  (mismo origen) y el SW lo sirve con COOP/COEP. La página lo pide a
  `/android-stub.jar`; se conserva `/android-jar` para el jar de la SDK elegido
  por el usuario.
- `studio/` es un submódulo: el cambio de la página se commitea primero en
  `apkc-studio` y luego se actualiza el puntero aquí, según la regla 8 de
  `AGENTS.md`.

## 5. Mediciones reales de los jars sintéticos

Todo lo de esta sección se midió en local con `$TMP=/tmp/opencode`. Los jars
sintéticos contienen el `resources.arsc` de la SDK y **no son entregables**;
sirven para aislar qué parte del jar hace falta.

| Artefacto | Contenido | Tamaño | Para qué sirve |
| --- | --- | --- | --- |
| `classes15.jar` | 15 clases, sin recursos | **65.028 B** comprimido; 271.437 B sin comprimir | mide el coste real de las firmas que usan las plantillas |
| `sub.jar` (6 clases) | 6 clases + `resources.arsc` completo | 16.499.055 B (sin comprimir) | primera prueba de `compile` |
| `sub.jar` (15 clases) | 15 clases + `resources.arsc` completo | 16.664.211 B (sin comprimir) | prueba de `compile` y `build` de las cinco plantillas |
| `fw3.apk` | solo atributos, 1.897 entradas, tabla binaria | **177.224 B** de `resources.arsc` | mide el coste de la tabla de atributos; los ids de esta prueba salieron corridos por un `--stable-ids` mal formado |
| `android-all-14.jar` | Robolectric completo | 137.666.235 B | fuente Apache-2.0 de contraste, no entregable tal cual |

Cifras de clases:

- 5.720 clases totales; 4.644 con `Stub!`; 4.787 con `Code`; 143 con `Code` sin
  `Stub!`; 933 sin `Code`.
- `classes15.jar` es el 0,25 % del jar de la SDK. El resto es recursos y clases
  que las plantillas no nombran.

## 6. Fases

Cada fase se cierra con su comando en verde y su párrafo en
`docs/evidencia.md`. Ninguna se marca «hecha» por analogía.

### J1. Medir y registrar el estado actual

- Criterio: están en `docs/evidencia.md` el tamaño del jar, el desglose por
  partes, el conteo de stubs y la lista de 15 clases, cada uno con su comando.
- Reproducción: los comandos de §1 y §1.1.
- Estado: medición hecha en esta investigación; falta el `§` de evidencia.

### J2. Generador de clases

- Criterio: `android-stub-min.jar` contiene las 15 clases y `apkc compile` de
  las cinco plantillas deja los mismos `.class` que con el jar de la SDK.
- Comando: `diff -r "$TMP/out-min" "$TMP/out-full"` sin salida.
- Criterio de rechazo: si el generador necesita `javac` y el `bootclasspath`
  del JDK, se para y se escribe; el objetivo es no depender de una JVM para
  producir el artefacto.
- Riesgo: `current.txt` no incluye `@hide`; una plantilla que use una API
  oculta dejará de compilar. Es una frontera, no un fallo.
- **Estado: hecho en local.** El generador está en `tools/androidjar/`, no usa
  `javac` y produce 4.873 clases. La comparación de las cinco plantillas está
  en §0.

### J3. Generador de `resources.arsc`

- Criterio: `TestMarco` pasa con el jar generado y los 28 ids de §33 coinciden;
  `apkc build --linker go` deja el mismo `resources.arsc` y el mismo
  `classes.dex` que con el jar de la SDK.
- Comando:
  ```sh
  mkdir -p "$TMP/fakehome/Android/platforms/android-34"
  cp "$TMP/android-stub-34-min.jar" \
    "$TMP/fakehome/Android/platforms/android-34/android.jar"
  HOME="$TMP/fakehome" go test -C apkc -count=1 -run TestMarco -v .
  ```
- Criterio de rechazo: si la tabla mínima no reproduce los 23 nombres de tipo
  o los 1.545 atributos, se añaden tipos vacíos y se documenta; no se rebaja el
  test.
- **Estado: hecho en local.** La tabla mide 301.784 bytes, emite 23 tipos,
  1.897 atributos y `TestMarco` pasa. Ver §0.

### J4. Workflow y artifact

- Criterio: el job `generar` produce los dos jars y `SHA256SUMS`; el job
  `verificar` pasa en GitHub Actions sobre el árbol final.
- Comando: `gh run watch` sobre el workflow; descarga del artifact y
  `sha256sum -c`.
- Criterio de rechazo: un workflow escrito no es un workflow verde. Si el job
  no pasa, la fase sigue pendiente.
- **Estado: NO verificada.** El run `45059765189` falló en `generar` por una
  ruta relativa (`cp: cannot stat 'dist/full/android-stub-34.jar'`); ver §0.2.
  El arreglo está en el worktree y necesita push y un run nuevo. No hay
  artifact ni hash de CI.

### J5. Consumo desde el sitio

- Criterio: `tools/sitio.sh` deja `sitio/android-stub.jar`; `studio/` lo pide
  al mismo origen; `prueba-pestana.js` y `prueba-sitio.js` pasan con cero
  peticiones fuera del origen.
- Comando: `SITIO_URL=https://apkc-studio.surge.sh node studio/prueba-sitio.js`.
- Criterio de rechazo: si el SW o el host no sirven el jar con COOP/COEP, la
  página vuelve al selector de fichero y la fase no se cierra.
- **Estado: hecha en local, pendiente en el sitio.** `prueba-pestana.js` pasa
  171/0 y `e2e.sh` termina en `e2e: todo en orden`, con el jar generado como
  `ANDROID_JAR` (§0.3). Se usó el `apkc`/wasm de `emulador-clics` porque el de
  este worktree no tiene `--pulsar`; no se tocó `apkc/`. Falta probar el jar en
  el sitio publicado (`prueba-sitio.js`), que es lo que impide cerrar la fase
  entera.

## 7. Fronteras

- **API 34 (Android 14)** como primera versión. 35 y 36 quedan para después,
  con la misma etiqueta fijada y su propio `current.txt`.
- **Solo API pública.** No se prometen `@hide`, `@SystemApi`, `@TestApi`,
  recursos de producto ni overlays. `string/safety_protection_display_text`
  queda fuera por su id de producto.
- **No hay paridad byte a byte con el jar de Google.** El objetivo es que
  compile y enlace lo que el studio genera; no que sea un reemplazo general.
- **Sin implementación.** Los stubs no ejecutan nada; el `dex` que produce
  `apkc` enlaza contra ellos, pero la implementación real vive en el aparato.
- **Sin binarios de la SDK en el repo ni en el sitio.** El jar de la SDK solo
  se usa en local y en el job de verificación como oráculo, nunca como
  artifact.
- **Autocompletado completo es opcional.** Si J2 solo produce la variante
  mínima, el editor pierde clases del framework; se documenta como regresión
  consciente hasta que exista `android-stub-full.jar`.

## 8. Registro en `docs/evidencia.md`

Cada medición nueva va como `§` con su comando, sin copiar aquí los números.
Propuesta de entradas:

- **§A. Anatomía del `android.jar`**: `unzip -l`, tamaños por parte, 5.720
  clases, 4.644 stubs, 4.787 con `Code`, 143 enums.
- **§B. Subconjunto que consumen las plantillas**: 6 clases base, 15 clases
  suficientes, `diff -r` de `.class` y `sha256sum` de `resources.arsc` y
  `classes.dex`.
- **§C. Licencias**: `NOTICE` de `prebuilts/sdk` es la SDK License; `NOTICE`
  de `frameworks/base` es Apache-2.0; cláusula 3.4 citada.
- **§D. Ids desde `public-final.xml`**: 2.924 de 2.926; la discrepancia de
  `string/safety_protection_display_text`.
- **§E. Tabla de recursos propia**: 301.784 B, 3.758 recursos, 1.897
  atributos, 23 tipos; `TestMarco` en verde.
- **§F. Workflow**: salida del job `generar` y `verificar`, hash del artifact.
  Pendiente de un run real en GitHub.

## 9. Hipótesis y preguntas abiertas

Se distingue lo **HECHO** de lo que sigue siendo **HIPÓTESIS**; cada hipótesis
lleva el comando que la comprobará.

1. **HECHO**: el generador de clases desde `current.txt` + `libcore-current.txt`
   produce las 15 clases con los mismos `.class` que la SDK en las cinco
   plantillas. Comando: `diff -r out-min out-sdk` sin salida.
2. **HECHO**: el `resources.arsc` propio mide 301.784 bytes y reproduce los 28
   ids de §33; `TestMarco` pasa. Comando:
   `unzip -p android-stub-34-min.jar resources.arsc | wc -c` y
   `HOME=… go test -C apkc -run TestMarco`.
3. **HECHO**: el jar completo pesa 2.829.065 bytes. Lo que **no** se ha
   probado es el autocompletado de la pestaña con él: eso es J5.
   Comando pendiente: `studio/prueba-pestana.js`, apartado de autocompletado.
4. **HECHO**: los 23 nombres de tipo se emiten aunque `public-final.xml` solo
   tenga 17; `TestMarcoDelAndroidJar` pasa con el jar generado.
5. **HIPÓTESIS**: Metalava en modo `TextCodebase` genera stubs utilizables.
   Está marcado como experimental en su propio código. Comprobación:
   `metalava --stubs … current.txt` y `javac` de los `.java`.
6. **PREGUNTA ABIERTA LEGAL**: ¿la cláusula 3.5 permite recortar el
   `android.jar` de la SDK por ser un componente open source? Este plan dice
   que no es la vía; no se afirma lo contrario sin dictamen.
7. **PREGUNTA ABIERTA**: ¿la tabla pública de `public-final.xml` cubre todos
   los recursos que un usuario puede referenciar con `@android:`? Los recursos
   no públicos y de producto quedan fuera; falta medir cuántos usos reales hay.
8. **PREGUNTA ABIERTA**: ¿el `NOTICE` de `frameworks/base` es suficiente para
   un jar que también contiene stubs de `java.*` y `org.w3c.*`? Esos paquetes
   están en `current.txt` y en el árbol de AOSP, pero conviene revisar los
   avisos por paquete antes de publicar.

## 10. Anexo: reproducible

Los artefactos de medición viven en `/tmp/androidjar-propio/` (la sesión
anterior usó `/tmp/opencode/`, que se limpió):

- `aosp/`: `current.txt`, `libcore-current.txt`, `public-final.xml`,
  `attrs.xml`, `attrs_manifest.xml` y `NOTICE`, de la etiqueta
  `android-14.0.0_r75`.
- `apkc`: binario de `apkc` compilado desde `apkc/`.
- `proj-base/` y `proj-five/`: proyectos de prueba de la plantilla base y de
  las cinco plantillas.
- `dist-final/full/` y `dist-final/min/`: los dos jars generados, con su
  `PROCEDENCIA.md`.
- `out-five-sdk/` y `final-min-classes/`: salidas de `apkc compile` para el
  `diff -r`.

Nada de esto se versiona: los jars van como artifact de CI, no al repo. El repo
guarda el generador, sus tests, el workflow y este plan.
