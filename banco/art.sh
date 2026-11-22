#!/bin/sh
# Subconjunto del banco para un aparato Android alcanzado por `adb` (la Blackview MEGA 3 por WiFi).
# banco/banco.sh está atado a Termux (rish/sudo, /data/data/...), así que aquí se repite lo esencial
# con `adb`: javac y apkc compilan las fuentes de banco/src, los tres dex se empujan a
# /data/local/tmp, app_process64 los ejecuta y dexdump los verifica. La JVM es el oráculo del código
# de salida y del texto.
#
#   uso: APKC=<binario> ANDROID_HOME=<sdk> sh banco/art.sh
#   ADB_SERIAL=<serial> elige el aparato; por defecto la Blackview de 192.168.1.53:46041.
#
# El serial va FIJADO en cada llamada a adb: con el Samsung por USB y la tableta por WiFi conectados
# a la vez, un `adb shell` sin `-s` elegiría el primero y el banco mediría el ART equivocado. La
# marca y el modelo se comprueban antes de compilar. No instala nada y no necesita pantalla.
set -eu
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ANDROID_HOME=${ANDROID_HOME:?falta ANDROID_HOME}
APKC=${APKC:-$AQUI/apkc-gate}
JDK=${JAVA_HOME:-}
JAVAC=${JAVAC:-$JDK/bin/javac}
JAVA=${JAVA:-$JDK/bin/java}
JAR=${ANDROID_JAR:-$ANDROID_HOME/platforms/android-34/android.jar}
BT=${BT:-$(ls -d "$ANDROID_HOME/build-tools"/* 2>/dev/null | sort -V | tail -1)}
ADB=${ADB:-$ANDROID_HOME/platform-tools/adb}
SERIE=${ADB_SERIAL:-192.168.1.53:46041}
TRABAJO=$(mktemp -d "${TMPDIR:-/tmp}/apkc-art.XXXXXX")

# Todas las llamadas llevan `-s "$SERIE"`: nunca se le pide a adb que escoja entre dos aparatos.
adb() { "$ADB" -s "$SERIE" "$@"; }

[ -x "$APKC" ] || { echo "art: falta el binario en $APKC"; exit 1; }
[ -x "$JAVAC" ] || { echo "art: falta javac en $JAVAC"; exit 1; }
[ -f "$JAR" ] || { echo "art: falta android.jar en $JAR"; exit 1; }
[ -x "$BT/d8" ] || { echo "art: falta d8 en $BT/d8"; exit 1; }
[ -x "$ADB" ] || { echo "art: falta adb en $ADB"; exit 1; }
adb wait-for-device
MARCA=$(adb shell getprop ro.product.brand | tr -d '\r')
MODELO=$(adb shell getprop ro.product.model | tr -d '\r')
echo "art: aparato $SERIE, marca=$MARCA, modelo=$MODELO"
if [ "$MARCA" != "Blackview" ] || [ "$MODELO" != "MEGA 3" ]; then
  echo "art: el aparato $SERIE no es la Blackview MEGA 3; se aborta antes de compilar"
  exit 1
fi
[ "$(adb shell getprop sys.boot_completed | tr -d '\r')" = "1" ] ||
  { echo "art: el aparato no terminó de arrancar"; exit 1; }

mkdir -p "$TRABAJO/scr" "$TRABAJO/jc" "$TRABAJO/nuestro" "$TRABAJO/d8" "$TRABAJO/d8desde"
"$JAVAC" -d "$TRABAJO/scr" "$AQUI/banco/src/p/Ver.java"
"$JAVAC" -cp "$TRABAJO/scr" -d "$TRABAJO/jc" "$AQUI/banco/src/p/Arranque.java"
"$APKC" compile --android-jar "$JAR" -o "$TRABAJO/nuestro" "$AQUI/banco/src/p/Ver.java" >"$TRABAJO/compile.log" 2>&1
cp "$TRABAJO/jc/p/Arranque.class" "$TRABAJO/nuestro/p/"
"$APKC" dex -o "$TRABAJO/nuestro.dex" "$TRABAJO/nuestro" >"$TRABAJO/dex.log" 2>&1
# El d8 de build-tools 34.0.0 revienta con un NPE interno sobre nuestros .class; el de 36.0.0 los
# lee. Se guarda el stderr y se muestra literalmente si falla, para no dejar una frontera muda.
"$BT/d8" --release --min-api 21 --lib "$JAR" --output "$TRABAJO/d8" \
  "$TRABAJO/scr/p/"*.class "$TRABAJO/jc/p/Arranque.class" >"$TRABAJO/d8.log" 2>&1 || {
  echo "art: d8 sobre javac fallo"; tail -5 "$TRABAJO/d8.log"; exit 1; }
"$BT/d8" --release --min-api 21 --output "$TRABAJO/d8desde" \
  "$TRABAJO/nuestro/p/"*.class >"$TRABAJO/d8desde.log" 2>&1 || {
  echo "art: d8 sobre nuestra Ver fallo"; tail -5 "$TRABAJO/d8desde.log"; exit 1; }

set +e
java_out=$("$JAVA" -cp "$TRABAJO/scr:$TRABAJO/jc" p.Arranque 2>&1)
jvm=$?
set -e
echo "  jvm (javac): rc=$jvm, salida: $(printf '%s' "$java_out" | tr '\n' '|')"

# Un dex por nombre; `adb push` evita el base64 de banco.sh (allí no hay adb).
veredicto() { # dex local, alias, descripción
  adb push "$1" "/data/local/tmp/$2.dex" >/dev/null 2>&1
  set +e
  adb shell "export CLASSPATH=/data/local/tmp/$2.dex; /system/bin/app_process64 /data/local/tmp p.Arranque > /data/local/tmp/$2.out 2>&1; echo rc=\$?" >/dev/null 2>&1
  sal=$(adb shell "cat /data/local/tmp/$2.out" | tr -d '\r')
  rcdex=$(adb shell "export CLASSPATH=/data/local/tmp/$2.dex; /system/bin/app_process64 /data/local/tmp p.Arranque >/dev/null 2>&1; echo \$?" | tr -d '\r')
  dv=$(adb shell "/apex/com.android.art/bin/dexdump /data/local/tmp/$2.dex >/dev/null 2>&1; echo \$?" | tr -d '\r')
  set -e
  # El fichero del aparato lleva al final la línea `rc=` que añade la propia orden: se recorta
  # antes de comparar con la salida de la JVM, igual que hace banco/banco.sh.
  txt=$(printf '%s\n' "$sal" | sed '/^rc=/d')
  v=$(printf '%s\n' "$sal" | grep -c "VerifyError\|NoClassDefFound" || true)
  if [ "$rcdex" = "$jvm" ] && [ "$txt" = "$java_out" ] && [ "$v" = "0" ] && [ "$dv" = "0" ]; then
    echo "  $3: rc=$rcdex y salida igual a la JVM, dexdump rc=0  ok"
  else
    echo "  $3: rc=$rcdex contra rc=$jvm, salida «$(printf '%s' "$txt" | tr '\n' '|')», dexdump rc=$dv  FALLO"
    exit 1
  fi
}
veredicto "$TRABAJO/nuestro.dex" bnuestro "nuestro dex (Go de punta a punta)"
veredicto "$TRABAJO/d8/classes.dex" bd8 "dex de d8 sobre javac"
veredicto "$TRABAJO/d8desde/classes.dex" bd8desde "dex de d8 sobre nuestra Ver"

echo "art: en orden"
