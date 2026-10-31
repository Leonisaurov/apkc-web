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

    # 4c. el proyecto que la pagina GENERA (clase anonima, @+id y style incluidos), compilado por
    #     la pestaña, frente al mismo proyecto compilado aqui fuera con el mismo camino todo-en-Go
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
fi

if [ "$FALLOS" -gt 0 ]; then echo "e2e: $FALLOS problemas"; exit 1; fi
echo "e2e: todo en orden"