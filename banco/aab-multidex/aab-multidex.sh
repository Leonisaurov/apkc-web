#!/bin/sh
# aab-multidex.sh — el .aab multi-dex, medido con bundletool como oráculo.
#
#   sh banco/aab-multidex/aab-multidex.sh
#   PROBAR_DEPENDENCIAS=1 sh banco/aab-multidex/aab-multidex.sh   # además, el caso con jars
#
# La frontera que se mide es la última del mapa: un `.aab` con más de un dex, derivado por
# bundletool a un `base.apk`. `apkc build --linker go --compiler go --signer go --aab` arma el
# bundle sin bundletool ni aapt2 (apkc/aab_go.go); aquí se comprueba con bundletool, que no
# comparte código con apkc, que:
#
#   1. el .aab lleva base/dex/classes.dex…classesN.dex, contiguos y en orden;
#   2. `bundletool validate` da rc 0;
#   3. `bundletool build-apks` deriva el base-master.apk (modo default) y el universal.apk, y los
#      dos llevan los mismos dex que el bundle;
#   4. `apksigner verify`, `zipalign -c -p 4` y `dexdump` del APK derivado pasan, y un método que
#      classes.dex invoca está definido en classes2.dex (el reparto se ejerce de verdad).
#
# El caso es el reparto del dex de la app: 700 clases de 100 métodos en `src/grande`, 70 000
# métodos que no caben en un dex. Es el mismo proyecto con el que el §228 midió el reparto en el
# APK; en el bundle los dos dex quedan en `base/dex/` y el número de ficheros no cambia.
#
# `PROBAR_DEPENDENCIAS=1` añade el caso de los jars (veintidós clases de dependencia, tres
# buckets). Ese caso HOY NO SE PUEDE ENSAMBLAR: aab_go.go recorre `build/` entero y mete también
# los `classes.dex` de `build/dexcache/…`, así que el bundle lista `base/dex/classes.dex` dos
# veces y la firma lo rechaza (sign.go:309). El guion lo ejecuta, espera ese fallo y lo reporta
# como defecto abierto con su `file:line`; no lo da por bueno ni lo esconde.
#
# Los artefactos van a $HOME/.cache/apkc-web/aab-multidex: no se versiona ningún binario (regla 2).
# Variables: APKC (binario), APKC_ANDROID_JAR o ANDROID_JAR, JAVA_HOME, APKC_WEB_CACHE y BUNDLETOOL
# (jar o ejecutable; si falta se busca en el PATH y en $ANDROID_HOME/bundletool).
#
# No usa serial, ni adb, ni aparato: el oráculo es bundletool en local. Códigos de salida: 0 en
# verde; 1 si un paso o un oráculo falla; 2 si no hay bundletool (la frontera queda abierta con el
# motivo impreso y el comando que la reproduciría cuando lo haya).
set -eu

AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/aab-multidex}
SALIDA=$CACHE/salida
mkdir -p "$CACHE" "$SALIDA"

falla() {
	echo "FALLO: $*" >&2
	exit 1
}

# ---------------------------------------------------------------- entorno
APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	( cd "$RAIZ/apkc" && CGO_ENABLED=0 go build -o "$APKC" . )
fi

JAR=${APKC_ANDROID_JAR:-${ANDROID_JAR:-}}
if [ -z "$JAR" ]; then
	ANDROID_HOME=${ANDROID_HOME:-$HOME/Android}
	JAR=$(ANDROID_HOME=$ANDROID_HOME sh "$RAIZ/tools/entorno.sh" jar)
fi
[ -f "$JAR" ] || falla "falta android.jar: $JAR"
SDK=$(CDPATH= cd -- "$(dirname -- "$(dirname -- "$(dirname -- "$JAR")")")" && pwd)
ANDROID_HOME=$SDK
APKSIGNER=${APKSIGNER:-$(ls "$SDK"/build-tools/*/apksigner 2>/dev/null | sort -V | tail -1)}
ZIPALIGN=${ZIPALIGN:-$(ls "$SDK"/build-tools/*/zipalign 2>/dev/null | sort -V | tail -1)}
DEXDUMP=${DEXDUMP:-$(ls "$SDK"/build-tools/*/dexdump 2>/dev/null | sort -V | tail -1)}
[ -x "${APKSIGNER:-}" ] || falla "no encuentro apksigner en $SDK/build-tools"
[ -x "${ZIPALIGN:-}" ] || falla "no encuentro zipalign en $SDK/build-tools"
[ -x "${DEXDUMP:-}" ] || falla "no encuentro dexdump en $SDK/build-tools"

JDK_BIN=${JAVA_HOME:+$JAVA_HOME/bin}
if [ -z "${JDK_BIN:-}" ] || [ ! -x "$JDK_BIN/javac" ]; then
	JDK_BIN=$(dirname "$(command -v javac)")
fi

# ---------------------------------------------------------------- bundletool: el oráculo
# El SDK no lo pone en el PATH: se instala aparte, como ejecutable o como jar "all".
BUNDLETOOL=${BUNDLETOOL:-}
if [ -z "$BUNDLETOOL" ] && command -v bundletool >/dev/null 2>&1; then
	BUNDLETOOL=$(command -v bundletool)
fi
if [ -z "$BUNDLETOOL" ]; then
	for j in "$ANDROID_HOME"/bundletool/bundletool-all-*.jar; do
		[ -f "$j" ] && BUNDLETOOL=$j
	done
fi
if [ -z "$BUNDLETOOL" ]; then
	echo "bundletool  AUSENTE (ni en el PATH ni en $ANDROID_HOME/bundletool)"
	echo "La frontera del .aab multi-dex sigue ABIERTA: sin bundletool no hay oráculo que derive el"
	echo "base.apk. Con bundletool instalado este mismo guion la cierra:"
	echo "  sh banco/aab-multidex/aab-multidex.sh"
	echo "Para instalarlo (red, decisión de quien lo ejecute):"
	echo "  curl -O https://github.com/google/bundletool/releases/download/1.18.3/bundletool-all-1.18.3.jar"
	echo "  mkdir -p \"\$ANDROID_HOME/bundletool\" && mv bundletool-all-1.18.3.jar \"\$ANDROID_HOME/bundletool/\""
	exit 2
fi
case $BUNDLETOOL in
*.jar)
	command -v java >/dev/null 2>&1 || falla "bundletool es un jar y no hay java en el PATH"
	bundletool() { java -jar "$BUNDLETOOL" "$@"; }
	;;
*)
	bundletool() { "$BUNDLETOOL" "$@"; }
	;;
esac

echo "apkc        $APKC"
echo "android     $JAR"
echo "oráculos    $APKSIGNER · $ZIPALIGN · $DEXDUMP"
echo "bundletool  $BUNDLETOOL"
echo "javac       $JDK_BIN"
bundletool version

KS=${KEYSTORE:-$CACHE/debug.keystore}
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

# lista los dex de un zip como «classes.dex classes2.dex …», en el orden con el que el runtime los
# abre. `sort -V` respeta que classes2.dex va antes que classes10.dex.
nombres_dex() {
	unzip -Z1 "$1" | grep -E "$2" | sed 's#.*/##' | sort -V | tr '\n' ' '
}

# verifica un APK derivado por bundletool: los mismos dex que el .aab, los tres oráculos y el
# cruce entre ficheros que hace real el reparto.
verifica_apk() {
	apk=$1
	echo "-- dex de $(basename "$apk")"
	obtenidos=$(nombres_dex "$apk" '^classes[0-9]*\.dex$')
	echo "   $obtenidos"
	[ "$obtenidos" = "$DEX_AAB" ] ||
		falla "$(basename "$apk"): los dex son «$obtenidos» y el .aab lleva «$DEX_AAB»"

	echo "-- apksigner verify --verbose"
	"$APKSIGNER" verify --verbose "$apk"
	"$APKSIGNER" verify --verbose "$apk" | grep -q 'Verifies' ||
		falla "$(basename "$apk") no verifica"

	echo "-- zipalign -c -p 4"
	"$ZIPALIGN" -c -p 4 "$apk"
	echo "   zipalign ok"

	rm -rf "$SALIDA/dex-derivados"
	mkdir -p "$SALIDA/dex-derivados"
	unzip -o -q "$apk" 'classes*.dex' -d "$SALIDA/dex-derivados"
	echo "-- dexdump de cada dex derivado"
	for d in "$SALIDA"/dex-derivados/classes*.dex; do
		printf '   %s (%s B, %s method_id): ' "$(basename "$d")" "$(wc -c <"$d")" \
			"$("$DEXDUMP" -f "$d" | sed -n 's/.*method_ids_size *: *\([0-9]*\).*/\1/p' | head -1)"
		"$DEXDUMP" -f "$d" | sed -n "s/.*Class descriptor  : '\(.*\)'/\1/p" | sed '/^$/d' |
			head -4 | tr '\n' ' '
		echo
	done

	# el techo de 16 bits se cumple por fichero, que es la propiedad que se mide
	echo "-- method_id de cada dex (techo 65 536)"
	for d in "$SALIDA"/dex-derivados/classes*.dex; do
		metodos=$("$DEXDUMP" -f "$d" | sed -n 's/.*method_ids_size *: *\([0-9]*\).*/\1/p' | head -1)
		[ -n "$metodos" ] || falla "dexdump no dice los method_id de $(basename "$d")"
		[ "$metodos" -le 65536 ] ||
			falla "$(basename "$d") lleva $metodos method_id, más de los que caben en un u16"
		echo "   $(basename "$d"): $metodos"
	done

	# la frontera del reparto: un método que classes.dex invoca y classes2.dex define. El par se
	# busca en vez de fijar una clase, porque dónde corta el reparto no lo decide el guion.
	echo "-- el cruce entre classes.dex y classes2.dex"
	"$DEXDUMP" -f "$SALIDA/dex-derivados/classes2.dex" |
		sed -n "s/.*Class descriptor  : '\(Lgrande\/C[0-9]*;\)'/\1/p" | sort -u >"$SALIDA/define2.txt"
	clase=$(
		"$DEXDUMP" -d "$SALIDA/dex-derivados/classes.dex" |
			sed -n "s/.*invoke-[a-z/]*.*\(Lgrande\/C[0-9]*;\)\.m0:()I.*/\1/p" |
			grep -F -m1 -f "$SALIDA/define2.txt" || true
	)
	[ -n "$clase" ] ||
		falla "classes.dex no invoca ningún método que defina classes2.dex: el .aab no prueba el reparto"
	echo "   classes.dex invoca ${clase#L}.m0 y classes2.dex define $clase"
}

# ================================================================ caso 1: el dex de la app
echo
echo "== caso 1: 70 000 métodos en las fuentes, repartidos en classes.dex + classes2.dex"
P=$SALIDA/proyecto-app
rm -rf "$P"
mkdir -p "$P/src/grande" "$P/src/com/ejemplo/aabmultidex" "$P/res/values" "$P/jar"
python3 - "$P/src/grande" <<'PY'
import os, sys
raiz = sys.argv[1]
clases = 700
for bloque in range(0, clases, 100):
    lineas = ["package grande;", ""]
    for c in range(bloque, min(bloque + 100, clases)):
        lineas.append("class C%d {" % c)
        for m in range(100):
            if c + 1 < clases:
                # cada método llama al de la clase siguiente: los índices altos se usan de verdad
                lineas.append("  static int m%d() { return C%d.m%d(); }" % (m, c + 1, m))
            else:
                lineas.append("  static int m%d() { return %d; }" % (m, m))
        lineas.append("}")
    with open(os.path.join(raiz, "Parte%d.java" % (bloque // 100)), "w") as fh:
        fh.write("\n".join(lineas) + "\n")
PY
sed 's/com\.ejemplo\.validar/com.ejemplo.aabmultidex/' "$AQUI/AndroidManifest.xml" >"$P/AndroidManifest.xml"
cp "$AQUI/res/values/strings.xml" "$P/res/values/strings.xml"
printf 'package com.ejemplo.aabmultidex;\n\nimport android.app.Activity;\nimport android.os.Bundle;\nimport android.widget.TextView;\nimport grande.C0;\n\npublic class MainActivity extends Activity {\n    @Override\n    public void onCreate(Bundle estado) {\n        super.onCreate(estado);\n        TextView texto = new TextView(this);\n        texto.setText("m0=" + C0.m0());\n        setContentView(texto);\n    }\n}\n' \
	>"$P/src/com/ejemplo/aabmultidex/MainActivity.java"
echo "   700 clases, 70 000 métodos"
( cd "$P" && "$APKC" build --linker go --compiler go --signer go --aab \
	--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" )
AAB=$P/build/app.aab
[ -f "$AAB" ] || falla "$P no dejó $AAB"

echo "-- dex dentro del .aab"
DEX_AAB=$(nombres_dex "$AAB" '^base/dex/classes[0-9]*\.dex$')
echo "   $DEX_AAB"
[ "$(echo "$DEX_AAB" | wc -w)" -ge 2 ] ||
	falla "el .aab lleva un solo dex: el reparto de la app no llegó al bundle"
n=1
for d in $DEX_AAB; do
	if [ "$n" -eq 1 ]; then
		[ "$d" = "classes.dex" ] || falla "el primer dex es $d y no classes.dex"
	else
		[ "$d" = "classes$n.dex" ] || falla "falta classes$n.dex (hay $d)"
	fi
	n=$((n + 1))
done
echo "   nombres y orden ok"

echo
echo "== bundletool validate --bundle"
bundletool validate --bundle "$AAB"

echo
echo "== bundletool build-apks --mode=default"
APKS=$SALIDA/app.apks
bundletool build-apks --bundle "$AAB" --output "$APKS" \
	--ks "$KS" --ks-pass pass:android --ks-key-alias androiddebugkey \
	--mode=default --overwrite
rm -rf "$SALIDA/apks"
mkdir -p "$SALIDA/apks"
unzip -o -q "$APKS" -d "$SALIDA/apks"
echo "-- contenido del .apks"
unzip -Z1 "$APKS" | sed 's/^/   /'
BASE_APK=$(ls "$SALIDA"/apks/splits/base-master*.apk 2>/dev/null | sort -V | head -1)
[ -n "${BASE_APK:-}" ] || falla "bundletool no derivó un base-master.apk"
verifica_apk "$BASE_APK"

echo
echo "== bundletool build-apks --mode=universal"
UAPKS=$SALIDA/universal.apks
bundletool build-apks --bundle "$AAB" --output "$UAPKS" \
	--ks "$KS" --ks-pass pass:android --ks-key-alias androiddebugkey \
	--mode=universal --overwrite
rm -rf "$SALIDA/universal"
mkdir -p "$SALIDA/universal"
unzip -o -q "$UAPKS" -d "$SALIDA/universal"
[ -f "$SALIDA/universal/universal.apk" ] || falla "bundletool no derivó universal.apk"
verifica_apk "$SALIDA/universal/universal.apk"

echo
echo "OK caso 1: el .aab multi-dex (classes.dex + classes2.dex) deriva un base-master.apk y un"
echo "universal.apk con los dos dex y los tres oráculos pasan."

# ================================================================ caso 2: jars (opt-in)
# El mismo mecanismo con dex de dependencias no llega a ensamblarse. Se mide para dejar la causa
# escrita con su file:line en vez de una impresión: el ensamblador recorre build/ entero.
if [ "${PROBAR_DEPENDENCIAS:-0}" = "1" ]; then
	echo
	echo "== caso 2 (PROBAR_DEPENDENCIAS=1): dex de dependencias en tres buckets"
	D=$SALIDA/proyecto-deps
	rm -rf "$D" "$CACHE/jars" "$SALIDA/dep-clases" "$SALIDA/dep-fuentes"
	mkdir -p "$D/src/com/ejemplo/aabmultidex" "$D/res/values" "$D/jar" "$CACHE/jars" "$SALIDA/dep-clases"
	sed 's/com\.ejemplo\.validar/com.ejemplo.aabmultidex/' "$AQUI/AndroidManifest.xml" >"$D/AndroidManifest.xml"
	cp "$AQUI/res/values/strings.xml" "$D/res/values/strings.xml"
	cp "$AQUI/src/com/ejemplo/aabmultidex/MainActivity.java" "$D/src/com/ejemplo/aabmultidex/MainActivity.java"
	cat >"$SALIDA/Saludo.java" <<'EOF'
package saludo;

public class Saludo {
    public static String saludo(String quien) {
        return "hola, " + quien;
    }
}
EOF
	"$JDK_BIN/javac" -d "$SALIDA/dep-clases" "$SALIDA/Saludo.java"
	"$JDK_BIN/jar" cf "$CACHE/jars/saludo.jar" -C "$SALIDA/dep-clases" .
	mkdir -p "$SALIDA/dep-fuentes/dep"
	i=0
	while [ "$i" -le 20 ]; do
		printf 'package dep;\n\npublic class D%d {\n    public static int valor() { return %d; }\n}\n' \
			"$i" "$i" >"$SALIDA/dep-fuentes/dep/D$i.java"
		i=$((i + 1))
	done
	"$JDK_BIN/javac" -d "$SALIDA/dep-clases" "$SALIDA"/dep-fuentes/dep/D*.java
	i=0
	while [ "$i" -le 20 ]; do
		mkdir -p "$SALIDA/jar-tmp/dep"
		cp "$SALIDA/dep-clases/dep/D$i.class" "$SALIDA/jar-tmp/dep/"
		( cd "$SALIDA/jar-tmp" && "$JDK_BIN/jar" cf "$CACHE/jars/dep$i.jar" dep )
		rm -rf "$SALIDA/jar-tmp"
		i=$((i + 1))
	done
	cp "$CACHE"/jars/*.jar "$D/jar/"
	set +e
	( cd "$D" && "$APKC" build --linker go --compiler go --signer go --aab \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" ) \
		>"$SALIDA/deps-build.log" 2>&1
	rc=$?
	set -e
	sed 's/^/   | /' "$SALIDA/deps-build.log"
	[ "$rc" -ne 0 ] ||
		falla "el .aab con dex de dependencias salió en verde: hay que volver a medir el caso 2"
	[ -f "$D/build/app.aab" ] || falla "el build falló sin dejar build/app.aab"
	echo "-- el bundle que dejó (antes de firmar)"
	unzip -Z1 "$D/build/app.aab" | grep '^base/dex/' | sed 's/^/   /'
	echo "-- causa"
	echo "   aab_go.go recorre build/ entero con ficherosDeArbol y nombra cada dex con"
	echo "   filepath.Base: los classes.dex de build/dexcache/… colisionan con el de la app"
	echo "   (apkc/aab_go.go:369-382, ensamblarAABDesdeGo en apkc/aab_go.go:308) y la firma lo"
	echo "   rechaza en apkc/sign.go:309. El caso de dependencias del .aab sigue ABIERTO."
fi

echo
echo "OK: la frontera del .aab multi-dex del dex de la app está medida con bundletool."
