#!/data/data/com.termux/files/usr/bin/sh
# punta a punta de la página: lo que apkc-studio genera tiene que compilar, firmar y verificar
# con las herramientas de build-tools, no solo pasar el test de node. Sin esto, la página se
# cree buena porque su propio test usa las mismas suposiciones que el código probado.
#
# uso: APKC=/ruta/apkc sh e2e.sh
# sale 0 con el proyecto java y, si hay kotlinc y una stdlib a mano, con el de kotlin también.

set -eu

AQUI=$(cd "$(dirname "$0")" && pwd)
TMP=${PREFIX:-/data/data/com.termux/files/usr}/tmp
APKC=${APKC:?faltan las ganas y la variable APKC con el binario}
SDK=${ANDROID_HOME:-$HOME/Android}
BUILD_TOOLS=$(ls -d "$SDK"/build-tools/* 2>/dev/null | sort -V | tail -1)
# el android.jar sirve también para la pestaña: `apkc build --linker go --compiler go` compila Java
# contra el SDK dentro de chromium, así que la prueba puede pedir el apk entero del navegador
ANDROID_JAR=${ANDROID_JAR:-$(ls -d "$SDK"/platforms/*/android.jar 2>/dev/null | sort -V | tail -1)}
ZIPCHECK="$HOME/Develop/Go/apkc-harness/zipcheck.py"
FALLOS=0

# El keystore de release de la prueba: se genera uno de usar y tirar con keytool, con contraseña
# y alias conocidos, porque una clave de verdad no se versiona ni se mete en una prueba. Es el
# que la pestaña tiene que abrir con --storepass/--keyalias y enseñar antes de firmar.
KEYTOOL=$(command -v keytool 2>/dev/null || true)
[ -n "$KEYTOOL" ] || KEYTOOL=${PREFIX:-/data/data/com.termux/files/usr}/lib/jvm/java-21-openjdk/bin/keytool
JARSIGNER=$(command -v jarsigner 2>/dev/null || true)
[ -n "$JARSIGNER" ] || JARSIGNER=${PREFIX:-/data/data/com.termux/files/usr}/lib/jvm/java-21-openjdk/bin/jarsigner
KS_RELEASE="$TMP/studio-release.p12"
rm -f "$KS_RELEASE"
if [ -x "$KEYTOOL" ]; then
  "$KEYTOOL" -genkeypair -keystore "$KS_RELEASE" -storetype PKCS12 -storepass saludo \
    -keypass saludo -alias lanzable -keyalg RSA -keysize 2048 -validity 3650 \
    -dname "CN=Release de prueba, O=apkc-web, C=ES" >"$TMP/studio-release-keytool.log" 2>&1 ||
    falla "no se pudo generar el keystore de release de la prueba"
else
  falla "no hay keytool: la prueba del keystore de release no puede correr"
fi

# Un .aab mínimo para la prueba de firma de bundles: es un zip con la forma que ve el
# firmante (BundleConfig.pb y un módulo), no un bundle de bundletool. La firma no depende
# de la tabla de recursos; la construye la fase 5.33.
AAB_EJEMPLO="$TMP/studio-ejemplo.aab"
rm -rf "$TMP/studio-aab-src"
mkdir -p "$TMP/studio-aab-src/base/dex" "$TMP/studio-aab-src/base/manifest"
printf 'BundleConfig' >"$TMP/studio-aab-src/BundleConfig.pb"
printf 'manifest' >"$TMP/studio-aab-src/base/manifest/AndroidManifest.xml"
printf 'dex' >"$TMP/studio-aab-src/base/dex/classes.dex"
(cd "$TMP/studio-aab-src" && python3 -m zipfile -c "$AAB_EJEMPLO" \
  BundleConfig.pb base/dex/classes.dex base/manifest/AndroidManifest.xml) ||
  falla "no se pudo crear el .aab de la prueba"

avisa() { printf '  %s\n' "$*"; }
falla() { FALLOS=$((FALLOS + 1)); avisa "FALLO: $*"; }

# 1. el test propio de la página
if ! node "$AQUI/prueba.js" >"$TMP/studio-prueba.log" 2>&1; then
  falla "prueba.js"; cat "$TMP/studio-prueba.log"
fi

# 2. cada variante: zip -> unzip -> apkc build -> oráculos
variancia() { # $1 nombre, $2 kotlin (0/1)
  nombre=$1; kotlin=$2
  carpeta=$TMP/studio-e2e-$nombre
  rm -rf "$carpeta"; mkdir -p "$carpeta"

  STUDIO_KOTLIN=$kotlin STUDIO_OUT=studio-$nombre.zip node "$AQUI/prueba.js" >/dev/null
  unzip -q "$TMP/studio-$nombre.zip" -d "$carpeta"
  proyecto=$(ls -d "$carpeta"/*/)
  # la variante tiene que ser la que se pide: con STUDIO_KOTLIN="0" por verdad en JS, el test
  # de "java" estaba compilando Kotlin y dando verde sin mirar el camino que decia mirar
  if [ "$kotlin" = 1 ]; then
    find "$proyecto/src" -name 'MainActivity.kt' | grep -q . || falla "$nombre: no hay MainActivity.kt"
  else
    find "$proyecto/src" -name 'MainActivity.java' | grep -q . || falla "$nombre: no hay MainActivity.java"
    find "$proyecto/src" -name '*.kt' | grep -q . && falla "$nombre: tenia .kt siendo la variante java"
  fi
  cd "$proyecto"

  if [ "$kotlin" = 1 ]; then
    stdlib=$(find "$SDK" ~/Develop -name 'kotlin-stdlib-1*.jar' -o -name 'kotlin-stdlib-2*.jar' 2>/dev/null | head -1)
    if [ -z "$stdlib" ]; then avisa "$nombre: sin kotlin-stdlib por ningun sitio, se omite"; return; fi
    cp "$stdlib" jar/
  fi

  if ! tcr -j 1 -q -- "$APKC" build >"$TMP/studio-$nombre-build.log" 2>&1; then
    falla "$nombre no compila"; tail -12 "$TMP/studio-$nombre-build.log"; return
  fi
  [ -f build/app.apk ] || { falla "$nombre: no hay build/app.apk"; return; }

  # los oráculos no comparten código con apkc: si ellos aceptan el apk, existió
  if ! "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 build/app.apk >"$TMP/studio-$nombre-verify.log" 2>&1; then
    falla "$nombre: apksigner no verifica"; head -6 "$TMP/studio-$nombre-verify.log"
  fi
  grep -q 'v1 scheme (JAR signing): true' "$TMP/studio-$nombre-verify.log" || falla "$nombre: sin firma v1"
  grep -q 'v2 scheme (APK Signature Scheme v2): true' "$TMP/studio-$nombre-verify.log" || falla "$nombre: sin firma v2"
  grep -q 'v3 scheme (APK Signature Scheme v3): true' "$TMP/studio-$nombre-verify.log" || falla "$nombre: sin firma v3"
  if ! python3 "$ZIPCHECK" build/app.apk >/dev/null 2>&1; then
    falla "$nombre: zipcheck rechaza el paquete"; python3 "$ZIPCHECK" build/app.apk | head -5
  fi
  if unzip -l build/app.apk | grep -q 'LEEME'; then
    falla "$nombre: un LEEME de la página acabó dentro del apk"
  fi
  avisa "$nombre: apk de $(wc -c <build/app.apk | tr -d ' ') bytes, verificado por apksigner y zipcheck"
  cd "$AQUI"
}

variancia java 0
variancia kotlin 1

# 3. el navegador real, si esta y si no se pidio omitirlo: el DOM, el click de descarga y el
#    localStorage bajo file:// son justo lo que node no puede ejercitar
if [ -z "${STUDIO_NO_BROWSER:-}" ] && command -v chromium-browser >/dev/null 2>&1; then
  if ! node "$AQUI/prueba-navegador.js" >"$TMP/studio-navegador.log" 2>&1; then
    falla "la pagina en chromium"; tail -8 "$TMP/studio-navegador.log"
  else
    avisa "$(grep -m1 '^  analisis' "$TMP/studio-navegador.log")"
  fi
else
  avisa "sin chromium (o STUDIO_NO_BROWSER puesto): la parte de DOM no se probo"
fi

# 4. la pestaña con apkc.wasm dentro de chromium: crea, edita en OPFS, sobrevive a recargar y
#    firma. Solo se prueba si hay binario wasm; sin él esta parte queda avisada, no dada por buena.
APK_FIXTURE=$(find "$TMP/studio-e2e-java" -name app.apk -path "*/build/*" 2>/dev/null | head -1)
if [ -n "${APKC_WASM:-}" ] && [ -f "$APKC_WASM" ] && [ -n "$APK_FIXTURE" ] && command -v chromium-browser >/dev/null 2>&1; then
  if ! APKC_WASM="$APKC_WASM" APK_EJEMPLO="$APK_FIXTURE" KS_EJEMPLO="${KS_EJEMPLO:-$HOME/.android/debug.keystore}" \
       KS_RELEASE="$KS_RELEASE" AAB_EJEMPLO="$AAB_EJEMPLO" \
       ANDROID_JAR="${ANDROID_JAR:-}" node "$AQUI/prueba-pestana.js" >"$TMP/studio-pestana.log" 2>&1; then
    falla "la pestaña con apkc.wasm"; tail -12 "$TMP/studio-pestana.log"
  else
    firmada="$TMP/pestana-apk/pestanera.apk"
    if ! "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$firmada" >"$TMP/studio-pestana-verify.log" 2>&1; then
      falla "la apk firmada en la pestaña no verifica"
    fi
    grep -q 'v1 scheme (JAR signing): true' "$TMP/studio-pestana-verify.log" || falla "apk de pestaña sin v1"
    grep -q 'v2 scheme (APK Signature Scheme v2): true' "$TMP/studio-pestana-verify.log" || falla "apk de pestaña sin v2"
    if ! python3 "$ZIPCHECK" "$firmada" >/dev/null 2>&1; then falla "zipcheck rechaza la apk de la pestaña"; fi
    avisa "pestaña: $(grep -c '^  ok' "$TMP/studio-pestana.log") comprobaciones, apk firmada dentro de chromium de $(wc -c <"$firmada" | tr -d ' ') bytes y verificada"

    # 4a-bis. el apk firmado en la pestaña con el keystore de release: apksigner tiene que leer el
    # certificado que la página enseñó antes de firmar, no el de debug.
    release="$TMP/pestana-apk/pestanera-release.apk"
    if [ -f "$release" ]; then
      if ! "$BUILD_TOOLS/apksigner" verify --verbose --print-certs --min-sdk-version 21 "$release" >"$TMP/studio-pestana-release.log" 2>&1; then
        falla "el apk firmado con el keystore de release no verifica"; head -6 "$TMP/studio-pestana-release.log"
      fi
      grep -q 'CN=Release de prueba' "$TMP/studio-pestana-release.log" ||
        falla "el apk de release no lleva el certificado elegido en la pestaña"
      grep -q 'v3 scheme (APK Signature Scheme v3): true' "$TMP/studio-pestana-release.log" ||
        falla "el apk de release no lleva v3"
      if ! python3 "$ZIPCHECK" "$release" >/dev/null 2>&1; then falla "zipcheck rechaza el apk de release"; fi
      avisa "pestaña release: apk de $(wc -c <"$release" | tr -d ' ') bytes firmado con CN=Release de prueba y verificado por apksigner (v1+v2+v3)"
      idsig="$release.idsig"
      if [ -f "$idsig" ]; then
        if ! "$BUILD_TOOLS/apksigner" verify --v4-signature-file "$idsig" "$release" >"$TMP/studio-pestana-v4.log" 2>&1; then
          falla "apksigner no verifica el .idsig de la pestaña"; tail -4 "$TMP/studio-pestana-v4.log"
        fi
        if "$APKC" verify "$release" >"$TMP/studio-pestana-v4-verify.log" 2>&1; then
          grep -q "v4 (APK Signature Scheme v4): ok" "$TMP/studio-pestana-v4-verify.log" ||
            falla "apkc verify no aprobó la v4 del .idsig"
        else
          falla "apkc verify rechaza el apk con .idsig"; tail -3 "$TMP/studio-pestana-v4-verify.log"
        fi
        avisa "pestaña v4: .idsig de $(wc -c <"$idsig" | tr -d ' ') bytes, apksigner y apkc verify lo aprueban"
      else
        falla "la pestaña no escribió el .idsig de la v4"
      fi
    else
      falla "la pestaña no dejó el apk firmado con el keystore de release"
    fi

    # 4a-ter. el .aab firmado en la pestaña: solo v1, sin bloque v2/v3. El oráculo es jarsigner
    # (que lee el JAR y avisa si el manifiesto no está primero) y el verificador propio.
    aab="$TMP/pestana-apk/pestanera.aab"
    if [ -f "$aab" ]; then
      if ! "$JARSIGNER" -verify "$aab" >"$TMP/studio-pestana-aab.log" 2>&1; then
        falla "jarsigner no verifica el .aab de la pestaña"; tail -4 "$TMP/studio-pestana-aab.log"
      fi
      grep -q "jar verified" "$TMP/studio-pestana-aab.log" ||
        falla "jarsigner no dijo «jar verified» sobre el .aab"
      if "$APKC" verify "$aab" >"$TMP/studio-pestana-aab-verify.log" 2>&1; then
        grep -q "v1 (JAR signing): ok" "$TMP/studio-pestana-aab-verify.log" ||
          falla "apkc verify no aprobó la v1 del .aab"
        grep -q "el paquete es un .aab" "$TMP/studio-pestana-aab-verify.log" ||
          falla "apkc verify no reconoció el .aab como bundle"
      else
        falla "apkc verify rechaza el .aab de la pestaña"; tail -3 "$TMP/studio-pestana-aab-verify.log"
      fi
      unzip -l "$aab" | grep -q "META-INF/MANIFEST.MF" || falla "el .aab no lleva MANIFEST.MF"
      if unzip -p "$aab" | grep -aq "APK Sig Block 42"; then
        falla "el .aab lleva un bloque de firmas de apk"
      fi
      [ ! -f "$aab.idsig" ] || falla "un .aab no lleva .idsig"
      avisa "pestaña .aab: $(wc -c <"$aab" | tr -d ' ') bytes, v1 JAR verificada por jarsigner y apkc verify, sin bloque v2/v3"
    else
      falla "la pestaña no dejó el .aab firmado"
    fi

    # 4a-quater. la rotación hecha desde la página: linaje, v1/v2 vieja, v3/v3.1 nueva y v4 de dos
    # firmantes. apksigner es el oráculo de los cuatro esquemas y del sidecar.
    rotada="$TMP/pestana-apk/pestanera-rotada.apk"
    if [ -f "$rotada" ]; then
      if "$BUILD_TOOLS/apksigner" verify --verbose "$rotada" >"$TMP/studio-pestana-rotada.log" 2>&1; then
        for esquema in "v1 scheme (JAR signing)" "v2 scheme (APK Signature Scheme v2)" \
                       "v3 scheme (APK Signature Scheme v3)" "v3.1 scheme (APK Signature Scheme v3.1)"; do
          grep -q "$esquema: true" "$TMP/studio-pestana-rotada.log" || falla "pestaña rotación: falta $esquema"
        done
      else
        falla "apksigner no verifica la rotación de la pestaña"; head -6 "$TMP/studio-pestana-rotada.log"
      fi
      if "$BUILD_TOOLS/apksigner" verify --v4-signature-file "$rotada.idsig" "$rotada" >"$TMP/studio-pestana-rotada-v4.log" 2>&1; then
        avisa "pestaña rotación: v1/v2 vieja + v3/v3.1 nueva, v4 de dos firmantes, apksigner y apkc verify"
      else
        falla "apksigner no verifica el .idsig de la rotación de la pestaña"; head -6 "$TMP/studio-pestana-rotada-v4.log"
      fi
    else
      falla "la pestaña no dejó el apk de la rotación"
    fi

    # 4b. el apk que la pestaña compiló entera (recursos, Java, dex y firma sin un solo proceso):
    #     los mismos oráculos que se piden a una apk de verdad, por herramientas de fuera
    compilado="$TMP/pestana-apk/pestanera-completa.apk"
    if [ -f "$compilado" ]; then
      if ! "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$compilado" >"$TMP/studio-pestana-completa.log" 2>&1; then
        falla "el apk compilado en la pestaña no verifica"
      fi
      grep -q 'v1 scheme (JAR signing): true' "$TMP/studio-pestana-completa.log" || falla "apk compilado en pestaña sin v1"
      grep -q 'v2 scheme (APK Signature Scheme v2): true' "$TMP/studio-pestana-completa.log" || falla "apk compilado en pestaña sin v2"
      "$BUILD_TOOLS/zipalign" -c -p 4 "$compilado" || falla "zipalign rechaza el apk compilado en la pestaña"
      if ! python3 "$ZIPCHECK" "$compilado" >/dev/null 2>&1; then falla "zipcheck rechaza el apk compilado en la pestaña"; fi
      # y aapt2, que es quien lee la tabla de recursos, tiene que entenderlo
      "$BUILD_TOOLS/aapt2" dump badging "$compilado" >"$TMP/studio-pestana-badging.log" 2>&1 || falla "aapt2 no entiende el apk de la pestaña"
      grep -q "package: name='com.myapp'" "$TMP/studio-pestana-badging.log" || falla "el apk de la pestaña no trae el package que creo"
      for pieza in classes.dex resources.arsc AndroidManifest.xml res/layout/main.xml; do
        unzip -l "$compilado" | grep -q " $pieza\$" || falla "falta $pieza en el apk compilado en la pestaña"
      done
      # y lo que el manifiesto promete con qué arrancar tiene que estar en el dex que escribió el
      # navegador: es lo más cerca del `pm install` que se puede comprobar sin dispositivo
      lanzable=$(sed -n "s/^launchable-activity: name='\([^']*\)'.*\$/\1/p" "$TMP/studio-pestana-badging.log" | head -1)
      [ -n "$lanzable" ] || falla "aapt2 no declaró un componente lanzable en el apk de la pestaña"
      interno=$(printf '%s' "$lanzable" | tr '.' '/')
      unzip -p "$compilado" classes.dex | strings | grep -q "L$interno;" ||
        falla "el dex del navegador no trae la clase que el manifiesto declara lanzable ($interno)"
      avisa "pestaña compilando: apk de $(wc -c <"$compilado" | tr -d ' ') bytes, verificado por apksigner (v1+v2), zipalign, zipcheck y aapt2"
      avisa "el apk de la pestaña declara $lanzable y esa clase está dentro de su dex"
    else
      avisa "sin ANDROID_JAR en el entorno: la pestaña no llego a compilar el proyecto entero"
    fi

    # 4c. el proyecto que la página GENERA (clase anónima, `@+id` y `style` incluidos), compilado por
    #     la pestaña, frente al mismo proyecto compilado aquí fuera con el mismo camino todo-en-Go
    estudio="$TMP/pestana-apk/pestanera-estudio.apk"
    if [ -f "$estudio" ] && [ -n "${ANDROID_JAR:-}" ]; then
      base=$TMP/studio-e46; rm -rf "$base"; mkdir -p "$base"
      AQUI="$AQUI" BASE="$base/proyecto" node -e '
        const fs = require("fs"), path = require("path");
        const a = require(path.join(process.env.AQUI, "app.js"));
        const estado = { name: "Hola apkc", package: "com.hola.apkcstudio", minsdk: "21", targetsdk: "33",
          hello: "Hello World!", button: true, kotlin: false, libs: false, assets: false, native: false,
          buttonLabel: "Press me", toast: "Clicked!" };
        const p = a.buildProject(estado);
        for (const r of Object.keys(p.files)) {
          const d = path.join(process.env.BASE, r);
          fs.mkdirSync(path.dirname(d), { recursive: true }); fs.writeFileSync(d, p.files[r]);
        }'
      proyecto=$base/proyecto
      cd "$proyecto"
      if ! tcr -j 1 -q -- "$APKC" build --linker go --compiler go --android-jar "$ANDROID_JAR" \
           --sdk-version 34 --codename 14 --keystore "${KS_EJEMPLO:-$HOME/.android/debug.keystore}" \
           >"$TMP/studio-e46-build.log" 2>&1; then
        falla "el proyecto generado no compila en nativo"; tail -8 "$TMP/studio-e46-build.log"
      else
        # d8 tiene que leer el dex que salio del navegador: es lo mas cerca de ART que hay sin telefono
        dex=$TMP/studio-e46-dex; rm -rf "$dex"; mkdir -p "$dex"
        unzip -qo "$estudio" classes.dex -d "$dex"
        if ! timeout 200 "$BUILD_TOOLS/d8" --release --lib "$ANDROID_JAR" --min-api 21 \
             --output "$dex" "$dex/classes.dex" >"$TMP/studio-e46-d8.log" 2>&1; then
          falla "d8 no lee el classes.dex que escribio la pestaña"; tail -5 "$TMP/studio-e46-d8.log"
        fi
        for pieza in AndroidManifest.xml resources.arsc res/layout/main.xml classes.dex \
                     META-INF/MANIFEST.MF META-INF/ANDROID.SF; do
          a=$(unzip -p "$estudio" "$pieza" | sha256sum | cut -c1-16)
          b=$(unzip -p build/app.apk "$pieza" | sha256sum | cut -c1-16)
          [ "$a" = "$b" ] || falla "$pieza difiere entre la pestaña ($a) y nativo ($b)"
        done
        "$BUILD_TOOLS/zipalign" -c -p 4 "$estudio" || falla "zipalign rechaza el apk del proyecto generado"
        if ! "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$estudio" >"$TMP/studio-e46-verify.log" 2>&1; then
          falla "el apk del proyecto generado no verifica"
        fi
        grep -q 'v1 scheme (JAR signing): true' "$TMP/studio-e46-verify.log" || falla "apk generado sin v1"
        grep -q 'v2 scheme (APK Signature Scheme v2): true' "$TMP/studio-e46-verify.log" || falla "apk generado sin v2"
        if ! python3 "$ZIPCHECK" "$estudio" >/dev/null 2>&1; then falla "zipcheck rechaza el apk del proyecto generado"; fi
        "$BUILD_TOOLS/aapt2" dump badging "$estudio" >"$TMP/studio-e46-badging.log" 2>&1 || falla "aapt2 no entiende el apk del proyecto generado"
        grep -q "package: name='com.hola.apkcstudio'" "$TMP/studio-e46-badging.log" || falla "el apk generado no trae el package de la pagina"
        # en este proyecto hay clase anonima: la interfaz del framework contra la que enlaza tiene
        # que estar en el dex del navegador, y el metodo que llama a su `show()` tambien
        for nombre in 'Lcom/hola/apkcstudio/MainActivity;' 'Lcom/hola/apkcstudio/MainActivity$1;' \
                      'Landroid/view/MenuItem$OnMenuItemClickListener;'; do
          unzip -p "$estudio" classes.dex | strings | grep -q "$nombre" ||
            falla "el dex del navegador no trae $nombre"
        done
        avisa "pestaña con el proyecto generado: apk de $(wc -c <"$estudio" | tr -d ' ') bytes, igual entrada por entrada al nativo, dex releido por d8"
      fi
      cd "$AQUI"
    fi

    # 4d. el fuente ESCRITO A MANO en la pestaña (campos con modificadores, cadena de llamadas,
    #     cuentas enteras, las branquias del párrafo 52 y la concatenación del 53): los mismos
    #     oráculos que a cualquier apk
    escrita="$TMP/pestana-apk/pestanera-escrita.apk"
    if [ -f "$escrita" ]; then
      if ! "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$escrita" >"$TMP/studio-pestana-escrita.log" 2>&1; then
        falla "el apk del fuente escrito a mano no verifica"
      fi
      grep -q 'v1 scheme (JAR signing): true' "$TMP/studio-pestana-escrita.log" || falla "apk escrito a mano sin v1"
      grep -q 'v2 scheme (APK Signature Scheme v2): true' "$TMP/studio-pestana-escrita.log" || falla "apk escrito a mano sin v2"
      "$BUILD_TOOLS/zipalign" -c -p 4 "$escrita" || falla "zipalign rechaza el apk del fuente escrito a mano"
      python3 "$ZIPCHECK" "$escrita" >/dev/null 2>&1 || falla "zipcheck rechaza el apk del fuente escrito a mano"
      "$BUILD_TOOLS/aapt2" dump badging "$escrita" >"$TMP/studio-pestana-escrita-badging.log" 2>&1 ||
        falla "aapt2 no entiende el apk del fuente escrito a mano"
      lanzable=$(sed -n "s/^launchable-activity: name='\([^']*\)'.*\$/\1/p" "$TMP/studio-pestana-escrita-badging.log" | head -1)
      [ "com.myapp.MainActivity" = "$lanzable" ] ||
        falla "el apk del fuente escrito declara $lanzable y se esperaba com.myapp.MainActivity"
      for nombre in 'Lcom/myapp/MainActivity;' pulsos total setVisibility cuenta StringBuilder; do
        unzip -p "$escrita" classes.dex | strings | grep -q "$nombre" ||
          falla "el dex del fuente escrito no trae $nombre"
      done
      # y el oráculo del párrafo 52: el dex sale de la pestaña CON branquias, y d8 lo relee y lo
      # re-dexa. Un desplazamiento mal parcheado se ve aquí, porque d8 recorre el código antes de
      # convertirlo y un destino fuera del método es un error con nombre
      mkdir -p "$TMP/escrita-dex" "$TMP/escrita-redex"
      unzip -p "$escrita" classes.dex >"$TMP/escrita-dex/classes.dex" ||
        falla "el apk escrito a mano no trae classes.dex"
      "$BUILD_TOOLS/d8" --release --min-api 21 --lib "$ANDROID_JAR" --output "$TMP/escrita-redex" \
        "$TMP/escrita-dex/classes.dex" >"$TMP/studio-escrita-d8.log" 2>&1 ||
        falla "d8 no relee el dex con branquias que salió de la pestaña"
      avisa "fuente escrito a mano: apk de $(wc -c <"$escrita" | tr -d ' ') bytes con $lanzable en su dex, verificado por apksigner, zipalign, zipcheck, aapt2 y releido por d8"
    fi

    # 4e. la APP DEL CONTADOR escrita en la pestaña (parrafo 54): anonima sobre una interfaz del
    #     framework, campo del exterior, `if` y concatenacion en el mismo cuerpo
    contador="$TMP/pestana-apk/pestanera-contador.apk"
    if [ -f "$contador" ]; then
      if ! "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$contador" >"$TMP/studio-pestana-contador.log" 2>&1; then
        falla "el apk de la app del contador no verifica"
      fi
      grep -q 'v1 scheme (JAR signing): true' "$TMP/studio-pestana-contador.log" || falla "apk del contador sin v1"
      grep -q 'v2 scheme (APK Signature Scheme v2): true' "$TMP/studio-pestana-contador.log" || falla "apk del contador sin v2"
      "$BUILD_TOOLS/zipalign" -c -p 4 "$contador" || falla "zipalign rechaza el apk del contador"
      python3 "$ZIPCHECK" "$contador" >/dev/null 2>&1 || falla "zipcheck rechaza el apk del contador"
      "$BUILD_TOOLS/aapt2" dump badging "$contador" >"$TMP/studio-pestana-contador-badging.log" 2>&1 ||
        falla "aapt2 no entiende el apk del contador"
      lanzable=$(sed -n "s/^launchable-activity: name='\([^']*\)'.*\$/\1/p" "$TMP/studio-pestana-contador-badging.log" | head -1)
      [ "com.myapp.MainActivity" = "$lanzable" ] ||
        falla "el apk del contador declara $lanzable y se esperaba com.myapp.MainActivity"
      # la anonima, la interfaz del framework contra la que enlaza, el metodo que llama, el campo del
      # exterior y la cadena que arma el concatenedor tienen que estar en el dex del navegador
      for nombre in 'Lcom/myapp/MainActivity$1;' 'Landroid/view/View$OnClickListener;' setContentDescription pulsos total lleva; do
        unzip -p "$contador" classes.dex | strings | grep -qe "$nombre" ||
          falla "el dex del contador no trae $nombre"
      done
      mkdir -p "$TMP/contador-dex" "$TMP/contador-redex"
      unzip -p "$contador" classes.dex >"$TMP/contador-dex/classes.dex" ||
        falla "el apk del contador no trae classes.dex"
      "$BUILD_TOOLS/d8" --release --min-api 21 --lib "$ANDROID_JAR" --output "$TMP/contador-redex" \
        "$TMP/contador-dex/classes.dex" >"$TMP/studio-contador-d8.log" 2>&1 ||
        falla "d8 no relee el dex del contador que salió de la pestaña"
      avisa "app del contador: apk de $(wc -c <"$contador" | tr -d ' ') bytes con su anonima y su concat en el dex, verificado por apksigner, zipalign, zipcheck, aapt2 y releido por d8"
    fi
  fi
else
  avisa "sin APKC_WASM, sin fixture o sin chromium: la pestaña con wasm no se probo"
fi

# 5. el verificador propio, cotejado con apksigner: el mismo veredicto sobre el mismo fichero. Sin
#    esto, `apkc verify` podría estar aprobando lo que rompe un apk y nadie se enteraría.
cotejo() { # $1 etiqueta, $2 apk, $3 veredicto esperado (si|no)
  etiqueta=$1; apk=$2; esperado=$3
  if [ ! -f "$apk" ]; then avisa "$etiqueta: no hay fichero que cotejar ($apk)"; return; fi
  # los dos `|| rc=$?` son obligatorios: con `set -e` un verify que dice "no" (que es lo que se busca
  # en dos de los tres casos) habría matado el script antes de poder comparar
  rc_apk=0; "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$apk" >"$TMP/studio-cotejo-$etiqueta-oraculo.log" 2>&1 || rc_apk=$?
  rc_nuestro=0; "$APKC" verify "$apk" >"$TMP/studio-cotejo-$etiqueta.log" 2>&1 || rc_nuestro=$?
  oraculo=si; [ "$rc_apk" -ne 0 ] && oraculo=no
  nuestro=si; [ "$rc_nuestro" -ne 0 ] && nuestro=no
  [ "$oraculo" = "$esperado" ] || falla "$etiqueta: apksigner dice $oraculo y se esperaba $esperado"
  [ "$nuestro" = "$esperado" ] || falla "$etiqueta: apkc verify dice $nuestro donde apksigner dice $oraculo"
  avisa "$etiqueta: apksigner $oraculo, apkc verify $nuestro"
}

apk_base=$(find "$TMP/studio-e2e-java" -name app.apk -path "*/build/*" 2>/dev/null | head -1)
if [ -n "${ANDROID_JAR:-}" ] && [ -f "$TMP/pestana-apk/pestanera-estudio.apk" ]; then
  apk_base="$TMP/pestana-apk/pestanera-estudio.apk"
fi
if [ -n "$BUILD_TOOLS" ] && [ -n "$apk_base" ]; then
  cotejo bueno "$apk_base" si

  # apk con la v3 de google encima: los pares que no se conocen se ignoran (y se dicen), no se herejan
  con_v3=$TMP/studio-cotejo-con-v3.apk
  if timeout 200 "$BUILD_TOOLS/apksigner" sign --ks "${KS_EJEMPLO:-$HOME/.android/debug.keystore}" \
       --ks-pass pass:android --v3-signing-enabled true --out "$con_v3" "$apk_base" >"$TMP/studio-cotejo-sign.log" 2>&1; then
    cotejo con-v3 "$con_v3" si
    grep -q "no se verificó aquí" "$TMP/studio-cotejo-con-v3.log" ||
      falla "la v3 presente no se anotó como no verificada en apkc verify"
  else
    avisa "con-v3: apksigner no pudo firmar el cotejo ($(tail -2 "$TMP/studio-cotejo-sign.log" | tr '\n' ' '))"
  fi

  # un byte cambiado dentro de una entrada ALMACENADA: el zip sigue siendo zip y las firmas casan mal
  alterado=$TMP/studio-cotejo-alterado.apk
  python3 - "$apk_base" "$alterado" <<'PY'
import sys, struct
src, dst = sys.argv[1], sys.argv[2]
data = bytearray(open(src, "rb").read())
eocd = data.rfind(b"PK\x05\x06")
cd = struct.unpack_from("<I", data, eocd + 16)[0]
largo = struct.unpack_from("<Q", data, cd - 24)[0]
inicio = cd - largo - 8
par = struct.unpack_from("<Q", data, inicio + 8)[0]
valor = inicio + 8 + 8 + 4
data[valor + (par - 4) // 2] ^= 0x08     # dentro del valor del par de la v2
open(dst, "wb").write(bytes(data))
PY
  cotejo bloque-alterado "$alterado" no

  # rotación de clave de punta a punta: `apkc rotate` hace el linaje, `apkc sign --lineage` pone
  # v1/v2 con la vieja y v3/v3.1 con la nueva, y apksigner tiene que ver los cuatro esquemas y
  # el .idsig de dos firmantes. Sin apksigner en la firma: solo como oráculo.
  rot_old=$TMP/studio-rot-old.p12
  rm -f "$rot_old"
  "$KEYTOOL" -genkeypair -keystore "$rot_old" -storetype PKCS12 -storepass viejito \
    -keypass viejito -alias viejo -keyalg RSA -keysize 2048 -validity 3650 \
    -dname "CN=Rotacion vieja, O=apkc-web, C=ES" >/dev/null 2>&1 ||
    falla "no se pudo generar la clave vieja de la rotación"
  rot_lineage=$TMP/studio-lineage.bin
  if "$APKC" rotate --old-keystore "$rot_old" --old-storepass viejito --old-keyalias viejo \
       --new-keystore "$KS_RELEASE" --new-storepass saludo --new-keyalias lanzable \
       --out "$rot_lineage" >"$TMP/studio-rotate.log" 2>&1; then
    rot_apk=$TMP/studio-rotado.apk
    if "$APKC" sign --keystore "$KS_RELEASE" --storepass saludo --keyalias lanzable \
         --old-keystore "$rot_old" --old-storepass viejito --old-keyalias viejo \
         --lineage "$rot_lineage" --rotation-min-sdk 33 --out "$rot_apk" "$apk_base" \
         >"$TMP/studio-rot-sign.log" 2>&1; then
      if "$BUILD_TOOLS/apksigner" verify --verbose "$rot_apk" >"$TMP/studio-rot-verify.log" 2>&1; then
        for esquema in "v1 scheme (JAR signing)" "v2 scheme (APK Signature Scheme v2)" \
                       "v3 scheme (APK Signature Scheme v3)" "v3.1 scheme (APK Signature Scheme v3.1)"; do
          grep -q "$esquema: true" "$TMP/studio-rot-verify.log" || falla "rotación: falta $esquema"
        done
      else
        falla "apksigner no verifica la rotación"; head -6 "$TMP/studio-rot-verify.log"
      fi
      if "$BUILD_TOOLS/apksigner" verify --v4-signature-file "$rot_apk.idsig" "$rot_apk" >"$TMP/studio-rot-v4.log" 2>&1; then
        "$APKC" verify "$rot_apk" >"$TMP/studio-rot-propio.log" 2>&1 ||
          falla "apkc verify rechaza la rotación"
        grep -q "v4 (APK Signature Scheme v4): ok" "$TMP/studio-rot-propio.log" ||
          falla "apkc verify no aprueba la v4 de dos firmantes"
        avisa "rotación: linaje, v1/v2 vieja + v3/v3.1 nueva, v4 de dos firmantes; apksigner y apkc verify"
      else
        falla "apksigner no verifica el .idsig de la rotación"; head -6 "$TMP/studio-rot-v4.log"
      fi
    else
      falla "apkc sign no firma la rotación"; tail -4 "$TMP/studio-rot-sign.log"
    fi
  else
    falla "apkc rotate no crea el linaje"; tail -4 "$TMP/studio-rotate.log"
  fi
fi

# 5. el icono de res/drawable (5.16): el PNG viaja literal, su entrada de la tabla es un archivo y
#    aapt2, que es quien lee la tabla, lo declara. Es el requisito de cierre «un icono en res/drawable».
prueba_icono() {
  carpeta=$TMP/studio-e2e-icono
  rm -rf "$carpeta"; mkdir -p "$carpeta/res/drawable" "$carpeta/res/mipmap" "$carpeta/res/values" "$carpeta/res/values-es" "$carpeta/res/layout" "$carpeta/src/com/icono"
  cat >"$carpeta/AndroidManifest.xml" <<'XML'
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.icono">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="33"/>
  <application android:label="@string/app_name" android:icon="@drawable/icono" android:roundIcon="@mipmap/ic_launcher">
    <activity android:name=".MainActivity" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.MAIN"/>
        <category android:name="android.intent.category.LAUNCHER"/>
      </intent-filter>
    </activity>
  </application>
</manifest>
XML
  cat >"$carpeta/res/values/strings.xml" <<'XML'
<resources><string name="app_name">Icono</string></resources>
XML
  cat >"$carpeta/res/values-es/strings.xml" <<'XML'
<resources><string name="app_name">Icono ES</string></resources>
XML
  cat >"$carpeta/res/layout/main.xml" <<'XML'
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>
XML
  cat >"$carpeta/src/com/icono/MainActivity.java" <<'JAVA'
package com.icono;
import android.app.Activity;
import android.os.Bundle;
public class MainActivity extends Activity {
    protected void onCreate(Bundle b) {
        super.onCreate(b);
        setContentView(R.layout.main);
    }
}
JAVA
  printf 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' |
    base64 -d >"$carpeta/res/drawable/icono.png"
  cp "$carpeta/res/drawable/icono.png" "$carpeta/res/mipmap/ic_launcher.png"
  if ! ( cd "$carpeta" && "$APKC" build --linker go --compiler go --signer go \
      --android-jar "$ANDROID_JAR" --keystore "${KS_EJEMPLO:-$HOME/.android/debug.keystore}" ) \
      >"$TMP/studio-icono-build.log" 2>&1; then
    falla "el icono no compila"; tail -8 "$TMP/studio-icono-build.log"; return
  fi
  apk=$carpeta/build/app.apk
  "$BUILD_TOOLS/aapt2" dump resources "$apk" >"$TMP/studio-icono-dump.log" 2>&1 ||
    falla "aapt2 no lee la tabla del icono"
  grep -q 'drawable/icono' "$TMP/studio-icono-dump.log" || falla "la tabla no declara drawable/icono"
  grep -q 'res/drawable/icono.png type=PNG' "$TMP/studio-icono-dump.log" || falla "aapt2 no ve el PNG como archivo"
  grep -q 'mipmap/ic_launcher' "$TMP/studio-icono-dump.log" || falla "la tabla no declara mipmap/ic_launcher"
  grep -q 'res/mipmap/ic_launcher.png type=PNG' "$TMP/studio-icono-dump.log" || falla "aapt2 no ve el mipmap como archivo"
  grep -q '(es) "Icono ES"' "$TMP/studio-icono-dump.log" || falla "aapt2 no ve la cadena de values-es"
  unzip -p "$apk" res/drawable/icono.png | cmp - "$carpeta/res/drawable/icono.png" ||
    falla "el PNG del apk no es el del proyecto"
  "$BUILD_TOOLS/apksigner" verify --verbose --min-sdk-version 21 "$apk" >"$TMP/studio-icono-verify.log" 2>&1 ||
    falla "apksigner no verifica el apk con icono"
  grep -q 'v3 scheme (APK Signature Scheme v3): true' "$TMP/studio-icono-verify.log" ||
    falla "el apk con icono no lleva v3"
  "$BUILD_TOOLS/zipalign" -c -p 4 "$apk" >/dev/null 2>&1 || falla "zipalign rechaza el apk con icono"
  avisa "icono: apk de $(wc -c <"$apk" | tr -d ' ') bytes, aapt2 declara drawable/icono y el PNG viaja literal"
}
prueba_icono

if [ "$FALLOS" -gt 0 ]; then echo "e2e: $FALLOS problemas"; exit 1; fi
echo "e2e: todo en orden"