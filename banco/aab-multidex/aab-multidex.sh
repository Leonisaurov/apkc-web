#!/bin/sh
# aab-multidex.sh — el .aab multi-dex, medido con bundletool como oráculo.
#
#   sh banco/aab-multidex/aab-multidex.sh
#   sh banco/aab-multidex/aab-multidex.sh --dependencias
#   PROBAR_DEPENDENCIAS=1 sh banco/aab-multidex/aab-multidex.sh
#
# Cierra la última frontera del mapa: un app bundle con más de un dex, derivado por bundletool a
# un APK. `apkc build --linker go --compiler go --signer go --aab` arma el bundle sin bundletool
# ni aapt2 (apkc/aab_go.go); aquí se comprueba con bundletool, que no comparte código con apkc,
# que:
#
#   1. el .aab lleva base/dex/classes.dex…classesN.dex, contiguos y en orden;
#   2. `bundletool validate` da rc 0;
#   3. `bundletool build-apks` deriva el base-master.apk (modo default) y el universal.apk, y los
#      dos llevan los mismos dex que el bundle;
#   4. `apksigner verify`, `zipalign -c -p 4` y `dexdump` del APK derivado pasan, ningún dex pasa
#      de 65 536 `method_id` y un método que classes.dex invoca está definido en otro fichero.
#
# Hay dos casos, y los dos tienen que salir en verde:
#
#   1. el reparto del dex de la app: 700 clases de 100 métodos en `src/grande`, 70 000 métodos que
#      no caben en un dex (es el mismo proyecto con el que el §228 midió el reparto en el APK).
#      Sale con `classes.dex` + `classes2.dex` en `base/dex/`.
#   2. el dex de dependencias, con `--dependencias` o `PROBAR_DEPENDENCIAS=1`: veintidós jars de
#      una clase cada uno, que el dexado reparte en tres buckets. Sale con `classes.dex` y
#      `classes2.dex`…`classes4.dex` (el de la app más los tres de dependencias).
#
# El caso 2 se activa con la variable o con el flag, y el flag existe porque un harness que sanea el
# entorno al invocar `sh <fichero>` deja la variable fuera del hijo: `--dependencias` viaja en argv.
# Estuvo bloqueado hasta `0237e23` («fix(aab): ensambla los dex del aab desde el plan correcto»):
# el ensamblador recorría `build/` entero y los `classes.dex` de `build/dexcache/…` colisionaban
# con el de la app al nombrarlos con `filepath.Base`. Desde ese parche `buildAAB` recibe los
# `depDexes` y numera los dex con `dexEntries`, la misma lista que empaqueta el APK.
#
# Los artefactos van a $HOME/.cache/apkc-web/aab-multidex: no se versiona ningún binario (regla 2).
# Variables: APKC (binario), APKC_ANDROID_JAR o ANDROID_JAR, JAVA_HOME, APKC_WEB_CACHE y BUNDLETOOL
# (jar o ejecutable; si falta se busca en el PATH y en $ANDROID_HOME/bundletool).
#
# No usa serial, ni adb, ni aparato: el oráculo es bundletool en local. Códigos de salida: 0 en
# verde; 1 si un paso o un oráculo falla; 2 si no hay bundletool.
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

# ---------------------------------------------------------------- argumentos
# El flag existe además de la variable: `sh fichero.sh --dependencias` no depende de que el
# entorno llegue al hijo, y en un harness que lo sanea la variable se queda en el padre.
DEPENDENCIAS=${PROBAR_DEPENDENCIAS:-0}
for arg in "$@"; do
	case $arg in
	--dependencias) DEPENDENCIAS=1 ;;
	--sin-dependencias) DEPENDENCIAS=0 ;;
	-h | --help)
		echo "uso: sh banco/aab-multidex/aab-multidex.sh [--dependencias]"
		echo "     PROBAR_DEPENDENCIAS=1 sh banco/aab-multidex/aab-multidex.sh"
		echo "  --dependencias  mide además el .aab con dex de dependencias (cuatro dex)"
		exit 0
		;;
	*) falla "argumento desconocido: $arg (usa --dependencias o -h)" ;;
	esac
done

# ---------------------------------------------------------------- entorno
APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	sh "$RAIZ/tools/materializar-emulador.sh" "$RAIZ"
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
	echo "  sh banco/aab-multidex/aab-multidex.sh --dependencias"
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
if [ "$DEPENDENCIAS" = "1" ]; then
	echo "caso 2      activo (--dependencias o PROBAR_DEPENDENCIAS=1)"
else
	echo "caso 2      inactivo (--dependencias o PROBAR_DEPENDENCIAS=1)"
fi
bundletool version

KS=${KEYSTORE:-$CACHE/debug.keystore}
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

# ---------------------------------------------------------------- utilidades
# lista los dex de un zip como «classes.dex classes2.dex …», en el orden con el que el runtime los
# abre. `sort -V` respeta que classes2.dex va antes que classes10.dex.
nombres_dex() {
	unzip -Z1 "$1" | grep -E "$2" | sed 's#.*/##' | sort -V | tr '\n' ' '
}

# exige que los nombres de los dex de un bundle o apk sean exactamente los esperados: contiguos
# desde classes.dex, sin huecos y sin sorpresas.
exige_dex() {
	obtenidos=$1
	esperados=$2
	que=$3
	echo "-- dex de $que"
	echo "   $obtenidos"
	[ "$obtenidos" = "$esperados" ] ||
		falla "$que: los dex son «$obtenidos» y se esperaban «$esperados»"
	n=1
	for d in $obtenidos; do
		if [ "$n" -eq 1 ]; then
			[ "$d" = "classes.dex" ] || falla "$que: el primer dex es $d y no classes.dex"
		else
			[ "$d" = "classes$n.dex" ] || falla "$que: falta classes$n.dex (hay $d)"
		fi
		n=$((n + 1))
	done
	echo "   nombres y orden ok"
}

# el cruce del caso 1: un método que classes.dex invoca y classes2.dex define. El par se busca en
# vez de fijar la clase, porque dónde corta el reparto no lo decide el guion.
cruce_app() {
	echo "-- el cruce entre classes.dex y classes2.dex"
	"$DEXDUMP" -f "$DEXDERIV/classes2.dex" |
		sed -n "s/.*Class descriptor  : '\(Lgrande\/C[0-9]*;\)'/\1/p" | sort -u >"$SALIDA/define2.txt"
	clase=$(
		"$DEXDUMP" -d "$DEXDERIV/classes.dex" |
			sed -n "s/.*invoke-[a-z/]*.*\(Lgrande\/C[0-9]*;\)\.m0:()I.*/\1/p" |
			grep -F -m1 -f "$SALIDA/define2.txt" || true
	)
	[ -n "$clase" ] ||
		falla "classes.dex no invoca ningún método que defina classes2.dex: el .aab no prueba el reparto"
	echo "   classes.dex invoca ${clase#L}.m0 y classes2.dex define $clase"
}

# el cruce del caso 2: la app invoca lo que definen los dex de dependencias, y ninguna de esas
# clases vive en classes.dex (si viviera, el bundle no probaría el reparto).
cruce_deps() {
	echo "-- el cruce entre classes.dex y los dex de dependencias"
	"$DEXDUMP" -d "$DEXDERIV/classes.dex" | grep -q 'Lsaludo/Saludo;.saludo' ||
		falla "classes.dex no invoca Lsaludo/Saludo;.saludo"
	"$DEXDUMP" -d "$DEXDERIV/classes.dex" | grep -q 'Ldep/D20;.valor' ||
		falla "classes.dex no invoca Ldep/D20;.valor"
	for clase in 'Lsaludo/Saludo;' 'Ldep/D0;' 'Ldep/D20;'; do
		definida=no
		for d in "$DEXDERIV"/classes*.dex; do
			[ "$(basename "$d")" = classes.dex ] && continue
			if "$DEXDUMP" -f "$d" | grep -q "'$clase'"; then
				definida=$(basename "$d")
				break
			fi
		done
		[ "$definida" != no ] || falla "ningún dex de dependencias define $clase"
		echo "   classes.dex invoca $clase lo define $definida"
	done
}

# verifica un APK derivado por bundletool: los mismos dex que el .aab, los oráculos y el cruce.
verifica_apk() {
	apk=$1
	esperados=$2
	cruce=$3
	exige_dex "$(nombres_dex "$apk" '^classes[0-9]*\.dex$')" "$esperados" "$(basename "$apk")"

	echo "-- apksigner verify --verbose"
	"$APKSIGNER" verify --verbose "$apk"
	"$APKSIGNER" verify --verbose "$apk" | grep -q 'Verifies' ||
		falla "$(basename "$apk") no verifica"

	echo "-- zipalign -c -p 4"
	"$ZIPALIGN" -c -p 4 "$apk"
	echo "   zipalign ok"

	DEXDERIV=$SALIDA/dex-derivados
	rm -rf "$DEXDERIV"
	mkdir -p "$DEXDERIV"
	unzip -o -q "$apk" 'classes*.dex' -d "$DEXDERIV"
	echo "-- dexdump de cada dex derivado"
	for d in "$DEXDERIV"/classes*.dex; do
		printf '   %s (%s B, %s method_id): ' "$(basename "$d")" "$(wc -c <"$d")" \
			"$("$DEXDUMP" -f "$d" | sed -n 's/.*method_ids_size *: *\([0-9]*\).*/\1/p' | head -1)"
		"$DEXDUMP" -f "$d" | sed -n "s/.*Class descriptor  : '\(.*\)'/\1/p" | sed '/^$/d' |
			head -4 | tr '\n' ' '
		echo
	done

	# el techo de 16 bits se cumple por fichero, que es la propiedad que se mide
	echo "-- method_id de cada dex (techo 65 536)"
	for d in "$DEXDERIV"/classes*.dex; do
		metodos=$("$DEXDUMP" -f "$d" | sed -n 's/.*method_ids_size *: *\([0-9]*\).*/\1/p' | head -1)
		[ -n "$metodos" ] || falla "dexdump no dice los method_id de $(basename "$d")"
		[ "$metodos" -le 65536 ] ||
			falla "$(basename "$d") lleva $metodos method_id, más de los que caben en un u16"
		echo "   $(basename "$d"): $metodos"
	done

	"$cruce"
}

# el oráculo entero para un .aab: validate, los dos modos de build-apks y verifica_apk sobre el
# base-master y el universal. `tag` separa los artefactos de cada caso.
oracula_aab() {
	aab=$1
	tag=$2
	esperados=$3
	cruce=$4

	echo
	echo "== [$tag] bundletool validate --bundle"
	bundletool validate --bundle "$aab"

	echo
	echo "== [$tag] bundletool build-apks --mode=default"
	apks=$SALIDA/apks-$tag.apks
	bundletool build-apks --bundle "$aab" --output "$apks" \
		--ks "$KS" --ks-pass pass:android --ks-key-alias androiddebugkey \
		--mode=default --overwrite
	rm -rf "$SALIDA/apks-$tag"
	mkdir -p "$SALIDA/apks-$tag"
	unzip -o -q "$apks" -d "$SALIDA/apks-$tag"
	echo "-- contenido del .apks"
	unzip -Z1 "$apks" | sed 's/^/   /'
	base=$(ls "$SALIDA/apks-$tag"/splits/base-master*.apk 2>/dev/null | sort -V | head -1)
	[ -n "${base:-}" ] || falla "bundletool no derivó un base-master.apk para $tag"
	verifica_apk "$base" "$esperados" "$cruce"

	echo
	echo "== [$tag] bundletool build-apks --mode=universal"
	uapks=$SALIDA/universal-$tag.apks
	bundletool build-apks --bundle "$aab" --output "$uapks" \
		--ks "$KS" --ks-pass pass:android --ks-key-alias androiddebugkey \
		--mode=universal --overwrite
	rm -rf "$SALIDA/universal-$tag"
	mkdir -p "$SALIDA/universal-$tag"
	unzip -o -q "$uapks" -d "$SALIDA/universal-$tag"
	[ -f "$SALIDA/universal-$tag/universal.apk" ] ||
		falla "bundletool no derivó universal.apk para $tag"
	verifica_apk "$SALIDA/universal-$tag/universal.apk" "$esperados" "$cruce"
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
[ -f "$P/build/app.aab" ] || falla "$P no dejó build/app.aab"
exige_dex "$(nombres_dex "$P/build/app.aab" '^base/dex/classes[0-9]*\.dex$')" \
	"classes.dex classes2.dex " "el .aab"
oracula_aab "$P/build/app.aab" app "classes.dex classes2.dex " cruce_app

echo
echo "OK caso 1: el .aab multi-dex (classes.dex + classes2.dex) deriva un base-master.apk y un"
echo "universal.apk con los dos dex y los oráculos pasan."

# ================================================================ caso 2: dex de dependencias
# Veintidós jars de una clase cada uno: el dexado de dependencias parte en tres buckets, así que
# el bundle lleva el dex de la app más tres de dependencias. Con `0237e23` esto ya se ensambla y
# se firma; el caso exige lo mismo que el caso 1, no un fallo esperado.
if [ "$DEPENDENCIAS" = "1" ]; then
	echo
	echo "== caso 2: 22 jars en tres buckets, classes.dex + classes2.dex…classes4.dex"
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
	echo "   22 jars de una clase"
	( cd "$D" && "$APKC" build --linker go --compiler go --signer go --aab \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" )
	[ -f "$D/build/app.aab" ] || falla "$D no dejó build/app.aab"
	exige_dex "$(nombres_dex "$D/build/app.aab" '^base/dex/classes[0-9]*\.dex$')" \
		"classes.dex classes2.dex classes3.dex classes4.dex " "el .aab con dependencias"
	oracula_aab "$D/build/app.aab" deps \
		"classes.dex classes2.dex classes3.dex classes4.dex " cruce_deps

	echo
	echo "OK caso 2: el .aab con dex de dependencias deriva un base-master.apk y un universal.apk"
	echo "con los cuatro dex, y la firma y los oráculos pasan."
fi

echo
echo "OK: la frontera del .aab multi-dex está medida con bundletool (caso 2 activo: $DEPENDENCIAS; argv: «${*:-}»)."
