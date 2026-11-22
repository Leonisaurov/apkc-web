#!/bin/sh
# Subconjunto del banco para un emulador Android alcanzado por `adb`. banco/banco.sh está atado a
# Termux (rish/sudo, /data/data/...), así que aquí se repite lo esencial con `adb`: javac y apkc
# compilan las fuentes de banco/src, los tres dex se empujan a /data/local/tmp, app_process64 los
# ejecuta y dexdump los verifica. La JVM es el oráculo del código de salida y del texto.
#
#   uso: APKC=<binario> ANDROID_HOME=<sdk> sh banco/art.sh
#
# No instala nada en el emulador y no necesita pantalla. Lo usa el job informativo `art` de
# .github/workflows/gate.yml; si el runner no da KVM, ese job ni lo intenta.
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
TRABAJO=$(mktemp -d "${TMPDIR:-/tmp}/apkc-art.XXXXXX")

[ -x "$APKC" ] || { echo "art: falta el binario en $APKC"; exit 1; }
[ -x "$JAVAC" ] || { echo "art: falta javac en $JAVAC"; exit 1; }
[ -f "$JAR" ] || { echo "art: falta android.jar en $JAR"; exit 1; }
[ -x "$BT/d8" ] || { echo "art: falta d8 en $BT/d8"; exit 1; }
[ -x "$ADB" ] || { echo "art: falta adb en $ADB"; exit 1; }
"$ADB" wait-for-device
[ "$("$ADB" shell getprop sys.boot_completed | tr -d '\r')" = "1" ] ||
  { echo "art: el emulador no terminó de arrancar"; exit 1; }

mkdir -p "$TRABAJO/scr" "$TRABAJO/jc" "$TRABAJO/nuestro" "$TRABAJO/d8" "$TRABAJO/d8desde"
"$JAVAC" -d "$TRABAJO/scr" "$AQUI/banco/src/p/Ver.java"
"$JAVAC" -cp "$TRABAJO/scr" -d "$TRABAJO/jc" "$AQUI/banco/src/p/Arranque.java"
"$APKC" compile --android-jar "$JAR" -o "$TRABAJO/nuestro" "$AQUI/banco/src/p/Ver.java" >"$TRABAJO/compile.log" 2>&1
cp "$TRABAJO/jc/p/Arranque.class" "$TRABAJO/nuestro/p/"
"$APKC" dex -o "$TRABAJO/nuestro.dex" "$TRABAJO/nuestro" >"$TRABAJO/dex.log" 2>&1
"$BT/d8" --release --min-api 21 --lib "$JAR" --output "$TRABAJO/d8" \
  "$TRABAJO/scr/p/"*.class "$TRABAJO/jc/p/Arranque.class" >/dev/null 2>&1
"$BT/d8" --release --min-api 21 --output "$TRABAJO/d8desde" \
  "$TRABAJO/nuestro/p/"*.class >/dev/null 2>&1

set +e
java_out=$("$JAVA" -cp "$TRABAJO/scr:$TRABAJO/jc" p.Arranque 2>&1)
jvm=$?
set -e
echo "  jvm (javac): rc=$jvm, salida: $(printf '%s' "$java_out" | tr '\n' '|')"

# Un dex por nombre; `adb push` evita el base64 de banco.sh (allí no hay adb).
veredicto() { # dex local, alias, descripción
  "$ADB" push "$1" "/data/local/tmp/$2.dex" >/dev/null
  set +e
  "$ADB" shell "export CLASSPATH=/data/local/tmp/$2.dex; /system/bin/app_process64 /data/local/tmp p.Arranque > /data/local/tmp/$2.out 2>&1; echo rc=\$?" >/dev/null 2>&1
  sal=$("$ADB" shell "cat /data/local/tmp/$2.out" | tr -d '\r')
  rcdex=$("$ADB" shell "export CLASSPATH=/data/local/tmp/$2.dex; /system/bin/app_process64 /data/local/tmp p.Arranque >/dev/null 2>&1; echo \$?" | tr -d '\r')
  dv=$("$ADB" shell "/apex/com.android.art/bin/dexdump /data/local/tmp/$2.dex >/dev/null 2>&1; echo \$?" | tr -d '\r')
  set -e
  txt=$(printf '%s\n' "$sal")
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
