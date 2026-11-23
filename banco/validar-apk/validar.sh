#!/bin/sh
# Validación de la salida final del build: un APK con dependencias en `jar/`, firmado por el camino
# Go (`--linker go --compiler go --signer go`), comprobado por oráculos que no comparten código con
# apkc.
#
#   sh banco/validar-apk/validar.sh
#
# Qué se mide, en dos pasadas sobre el mismo código de app:
#   1. un jar (jar/saludo.jar): el APK queda con classes.dex (la app) y classes2.dex (la dependencia).
#   2. veintidós jars: el dexado de dependencias parte en tres buckets, así que el APK lleva
#      classes.dex y classes2.dex…classes4.dex, en ese orden y sin huecos.
#
# En las dos, `apksigner verify --verbose`, `zipalign -c -p 4` y `dexdump` son el oráculo; el guion
# comprueba además que la app invoca `Lsaludo/Saludo;.saludo`, que el método del jar está definido en
# uno de los dex de dependencias y que el de la segunda pasada invoca `Ldep/D20;.valor`, la clase que
# solo existe en un jar sintético.
#
# Con PROBAR_LIMITE=1 hace una tercera pasada, de otro proyecto: 700 clases y 70 000 métodos
# encadenados, que no caben en un dex. El dexer reparte el bucket en classes.dex…classes3.dex y el
# guion exige que ninguno pase de 65 536 method_id, que las 700 clases queden repartidas entre los
# dos dex de dependencias sin perder ni duplicar ninguna y que un método se invoque en uno y se
# defina en el otro. Sin el reparto, ese build se paraba en la guarda de los índices de 16 bits.
#
# Los artefactos van a $HOME/.cache/apkc-web/validar-apk: no se versiona ningún binario (regla 2).
# Variables: APKC (binario), APKC_ANDROID_JAR o ANDROID_JAR, JAVA_HOME, APKC_WEB_CACHE y, opcional,
# SERIE_APARATO (p. ej. 10.177.0.2:5555) para instalar y arrancar la app del caso multi-dex.
#
# Códigos de salida: 0 si las pasadas pedidas y sus oráculos salen en verde; 1 en cualquier otro
# caso; 3 si la pasada opcional del jar real todavía se para en una forma que el dexer no mide.
set -eu

AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/validar-apk}
SALIDA=$CACHE/salida
mkdir -p "$CACHE" "$SALIDA" "$AQUI/jar"

falla() {
	echo "FALLO: $*" >&2
	exit 1
}

# El binario y el android.jar, como el resto de banco/: se construyen o se toman del SDK.
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
APKSIGNER=${APKSIGNER:-$(ls "$SDK"/build-tools/*/apksigner 2>/dev/null | sort -V | tail -1)}
ZIPALIGN=${ZIPALIGN:-$(ls "$SDK"/build-tools/*/zipalign 2>/dev/null | sort -V | tail -1)}
DEXDUMP=${DEXDUMP:-$(ls "$SDK"/build-tools/*/dexdump 2>/dev/null | sort -V | tail -1)}
[ -x "${APKSIGNER:-}" ] || falla "no encuentro apksigner en $SDK/build-tools"
[ -x "${ZIPALIGN:-}" ] || falla "no encuentro zipalign en $SDK/build-tools"
[ -x "${DEXDUMP:-}" ] || falla "no encuentro dexdump en $SDK/build-tools"

# JDK: javac solo hace falta para los jars del fixture, y el jar local se compila con --release 17
# cuando ese javac lo acepta.
JDK_BIN=${JAVA_HOME:+$JAVA_HOME/bin}
if [ -z "${JDK_BIN:-}" ] || [ ! -x "$JDK_BIN/javac" ]; then
	JDK_BIN=$(dirname "$(command -v javac)")
fi
RELEASE=
if "$JDK_BIN/javac" --release 17 -version >/dev/null 2>&1; then
	RELEASE="--release 17"
fi
echo "apkc      $APKC"
echo "android   $JAR"
echo "oráculos  $APKSIGNER · $ZIPALIGN · $DEXDUMP"
echo "javac     $JDK_BIN ${RELEASE:-(sin --release)}"

KS=${KEYSTORE:-$CACHE/debug.keystore}
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

# ---------------------------------------------------------------- jars del fixture
echo "== el jar del fixture (saludo/Saludo) y veintiún jars sintéticos"
rm -rf "$SALIDA/dep-clases" "$AQUI/jar"/*.jar
mkdir -p "$SALIDA/dep-clases"
"$JDK_BIN/javac" $RELEASE -d "$SALIDA/dep-clases" "$AQUI/dep/saludo/Saludo.java"
"$JDK_BIN/jar" cf "$AQUI/jar/saludo.jar" -C "$SALIDA/dep-clases" .
echo "   $AQUI/jar/saludo.jar ($(wc -c <"$AQUI/jar/saludo.jar") bytes)"

# Un `.class` por jar: veintidós entradas es lo que hace falta para que depBuckets parta en tres.
mkdir -p "$SALIDA/dep-fuentes/dep"
i=0
while [ "$i" -le 20 ]; do
	printf 'package dep;\n\npublic class D%d {\n    public static int valor() { return %d; }\n}\n' \
		"$i" "$i" >"$SALIDA/dep-fuentes/dep/D$i.java"
	i=$((i + 1))
done
"$JDK_BIN/javac" $RELEASE -d "$SALIDA/dep-clases" "$SALIDA"/dep-fuentes/dep/D*.java
i=0
while [ "$i" -le 20 ]; do
	mkdir -p "$SALIDA/jar-tmp/dep"
	cp "$SALIDA/dep-clases/dep/D$i.class" "$SALIDA/jar-tmp/dep/"
	( cd "$SALIDA/jar-tmp" && "$JDK_BIN/jar" cf "$AQUI/jar/dep$i.jar" dep )
	rm -rf "$SALIDA/jar-tmp"
	i=$((i + 1))
done
echo "   $AQUI/jar/dep0.jar … dep20.jar (una clase cada uno)"

# ---------------------------------------------------------------- utilidades de verificación
# arma un proyecto en el directorio dado: manifiesto, res, y las fuentes del caso
arma_proyecto() {
	dir=$1
	fuentes=$2
	rm -rf "$dir"
	mkdir -p "$dir" "$dir/res/values" "$dir/src/com/ejemplo/validar" "$dir/jar"
	cp "$AQUI/AndroidManifest.xml" "$dir/AndroidManifest.xml"
	cp "$AQUI/res/values/strings.xml" "$dir/res/values/strings.xml"
	cp "$fuentes/com/ejemplo/validar/MainActivity.java" "$dir/src/com/ejemplo/validar/MainActivity.java"
}

# compila un proyecto armado y deja su apk en la ruta que se imprime
construye() {
	dir=$1
	echo "-- apkc build"
	( cd "$dir" && "$APKC" build --linker go --compiler go --signer go \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" )
	[ -f "$dir/build/app.apk" ] || falla "$dir no dejó build/app.apk"
}

# lista los nombres de entrada de los dex del apk, en el orden del zip
lista_dex() {
	unzip -Z1 "$1" | grep -E '^classes[0-9]*\.dex$'
}

# pasa los tres oráculos y comprueba los nombres de los dex y el método del jar
verifica() {
	apk=$1
	nombres_esperados=$2
	echo "-- apksigner verify --verbose"
	"$APKSIGNER" verify --verbose "$apk"
	"$APKSIGNER" verify --verbose "$apk" | grep -q 'Verified using v1 scheme (JAR signing): true' ||
		falla "apksigner no da la v1 por buena"
	"$APKSIGNER" verify --verbose "$apk" | grep -q 'Verified using v2 scheme (APK Signature Scheme v2): true' ||
		falla "apksigner no da la v2 por buena"
	"$APKSIGNER" verify --verbose "$apk" | grep -q 'Verified using v3 scheme (APK Signature Scheme v3): true' ||
		falla "apksigner no da la v3 por buena"

	echo "-- zipalign -c -p 4"
	"$ZIPALIGN" -c -p 4 "$apk"
	echo "   zipalign ok"

	echo "-- nombres de los dex"
	obtenidos=$(lista_dex "$apk" | tr '\n' ' ')
	echo "   $obtenidos"
	[ "$obtenidos" = "$nombres_esperados" ] ||
		falla "los dex son «$obtenidos» y se esperaban «$nombres_esperados»"

	rm -rf "$SALIDA/dex"
	mkdir -p "$SALIDA/dex"
	unzip -o -q "$apk" 'classes*.dex' -d "$SALIDA/dex"
	echo "-- dexdump de cada dex"
	for d in "$SALIDA"/dex/classes*.dex; do
		printf '   %s (%s B): ' "$(basename "$d")" "$(wc -c <"$d")"
		descriptores=$("$DEXDUMP" -f "$d" | sed -n "s/.*Class descriptor  : '\(.*\)'/\1/p")
		# un dex de dependencias puede llevar cientos de clases: se enseñan las primeras y el
		# total, que es lo que hace legible el registro sin dejar de decir qué define cada uno
		printf '%s\n' "$descriptores" | sed '/^$/d' | head -8 | tr '\n' ' '
		total=$(printf '%s\n' "$descriptores" | sed '/^$/d' | wc -l)
		if [ "$total" -gt 8 ]; then printf '… (%d clases)' "$total"; fi
		echo
	done
}

# comprueba que un método se invoca en el dex de la app y que la clase se define en OTRO dex del apk:
# si el dex que invoca fuera también el que define, el apk no probaría el reparto en clasesN.dex y la
# comprobación pasaría por cualquier cosa.
verifica_metodo() {
	metodo=$1      # la forma que imprime dexdump -d: Lsaludo/Saludo;.saludo
	clase=$2       # el descriptor que imprime dexdump -f: Lsaludo/Saludo;
	invoca_en=$3   # el dex de la app
	"$DEXDUMP" -d "$invoca_en" | grep -q "$metodo" ||
		falla "$(basename "$invoca_en") no invoca $metodo"
	echo "   $(basename "$invoca_en") invoca $metodo"
	definida=no
	for d in "$SALIDA"/dex/classes*.dex; do
		if "$DEXDUMP" -f "$d" | grep -q "'$clase'"; then
			definida=$d
			break
		fi
	done
	[ "$definida" != no ] || falla "ningún dex del apk define $clase"
	if [ "$(basename "$definida")" = "$(basename "$invoca_en")" ]; then
		falla "$(basename "$definida") invoca y define $clase: el método del jar tiene que vivir en otro dex del apk, y el dex de la app no prueba el reparto"
	fi
	echo "   $(basename "$definida") define $clase, y es distinto de $(basename "$invoca_en")"
}

# ---------------------------------------------------------------- pasada 1: un jar
echo
echo "== pasada 1: un jar (classes.dex + classes2.dex)"
P1=$CACHE/proyecto-un-jar
arma_proyecto "$P1" "$AQUI/src"
cp "$AQUI/jar/saludo.jar" "$P1/jar/"
construye "$P1"
verifica "$P1/build/app.apk" "classes.dex classes2.dex "
verifica_metodo "Lsaludo/Saludo;.saludo" "Lsaludo/Saludo;" "$SALIDA/dex/classes.dex"

# ---------------------------------------------------------------- pasada 2: veintidós jars
echo
echo "== pasada 2: veintidós jars (classes.dex…classes4.dex)"
P2=$CACHE/proyecto-muchos-jar
arma_proyecto "$P2" "$AQUI/src-multidex"
cp "$AQUI/jar"/*.jar "$P2/jar/"
construye "$P2"
verifica "$P2/build/app.apk" "classes.dex classes2.dex classes3.dex classes4.dex "
verifica_metodo "Lsaludo/Saludo;.saludo" "Lsaludo/Saludo;" "$SALIDA/dex/classes.dex"
verifica_metodo "Ldep/D20;.valor" "Ldep/D20;" "$SALIDA/dex/classes.dex"
definidas=0
for d in "$SALIDA"/dex/classes*.dex; do
	definidas=$((definidas + $("$DEXDUMP" -f "$d" |
		sed -n "s/.*Class descriptor  : '\(Ldep\/D[0-9]*;\)'/\1/p" | sort -u | wc -l)))
done
[ "$definidas" -eq 21 ] || falla "los dex de dependencias definen $definidas clases dep.Dn y hay 21"
echo "   las tres dependencias definen las 21 clases dep.D0…D20"

# ---------------------------------------------------------------- opt-in: el jar real
# La pasada con un jar de verdad (android-support-v4.jar, 520 clases). La frontera de `goto/16` que
# este pase citaba se cerró en `295ac58`, pero el dexer todavía se para más adelante, en una forma
# de registro de cuatro bits (medido: `i2l` con v17 en GestureDetectorCompat; el README lleva el
# mensaje literal). Es una frontera de opcodes y se arregla en dexinstr.go, no aquí: por eso este
# pase sale con 3 en vez de validar. Cuando el binario en uso la acepte, el mismo comando pasa a los
# tres oráculos y comprueba que la app invoca un método del jar real.
#
#   JAR_REAL=/ruta/android-support-v4.jar sh banco/validar-apk/validar.sh
if [ -n "${JAR_REAL:-}" ]; then
	echo
	echo "== jar real: $(basename "$JAR_REAL") ($(wc -c <"$JAR_REAL") bytes)"
	R=$CACHE/real
	rm -rf "$R"
	mkdir -p "$R/jar" "$R/src/com/ejemplo/real" "$R/res/values"
	sed 's/com\.ejemplo\.validar/com.ejemplo.real/' "$AQUI/AndroidManifest.xml" >"$R/AndroidManifest.xml"
	cp "$AQUI/res/values/strings.xml" "$R/res/values/strings.xml"
	cp "$JAR_REAL" "$R/jar/"
	printf 'package com.ejemplo.real;\n\nimport android.app.Activity;\nimport android.os.Bundle;\nimport android.support.v4.text.TextUtilsCompat;\nimport android.widget.TextView;\n\npublic class MainActivity extends Activity {\n    @Override\n    public void onCreate(Bundle estado) {\n        super.onCreate(estado);\n        TextView texto = new TextView(this);\n        texto.setText(TextUtilsCompat.htmlEncode("<b>apkc</b>"));\n        setContentView(texto);\n    }\n}\n' \
		>"$R/src/com/ejemplo/real/MainActivity.java"
	set +e
	( cd "$R" && "$APKC" build --linker go --compiler go --signer go \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" ) \
		>"$R/build.log" 2>&1
	rc=$?
	set -e
	sed 's/^/   | /' "$R/build.log"
	if [ "$rc" -ne 0 ]; then
		echo "   el jar real todavía no dexa (rc $rc): la frontera es del dexer de opcodes" >&2
		exit 3
	fi
	verifica "$R/build/app.apk" "classes.dex classes2.dex "
	verifica_metodo "Landroid/support/v4/text/TextUtilsCompat;.htmlEncode" \
		"Landroid/support/v4/text/TextUtilsCompat;" "$SALIDA/dex/classes.dex"
fi

# ---------------------------------------------------------------- opt-in: el techo de 16 bits
# Un dex referencia sus métodos, campos, tipos y cadenas con índices de 16 bits: la segunda unidad
# de un 21c, un 22c y un 35c es un u16, y el dexer rechaza el índice que no cabe en vez de
# recortarlo (apkc/dexinstr.go, la guarda de `indice`). El techo es del fichero, no del bucket:
# este pase lo fuerza con 700 clases de 100 métodos encadenados (70 000 métodos) y comprueba que
# el dexer reparte el bucket en classes.dex, classes2.dex… en vez de abortar, que no pierde ni
# duplica una clase al cerrar un grupo y que el APK pasa los tres oráculos. El reparto y sus
# pruebas viven en apkc/reparto.go y apkc/reparto_test.go.
if [ "${PROBAR_LIMITE:-0}" = "1" ]; then
	echo
	echo "== techo de 65 536 referencias de método (opt-in: PROBAR_LIMITE=1)"
	L=$CACHE/limite
	rm -rf "$L"
	mkdir -p "$L/src/grande" "$L/clases" "$L/proyecto/jar" "$L/proyecto/src/com/ejemplo/limite" \
		"$L/proyecto/res/values"
	python3 - "$L/src/grande" <<'PY'
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
	"$JDK_BIN/javac" $RELEASE -J-Xmx512m -d "$L/clases" "$L"/src/grande/Parte0.java \
		"$L"/src/grande/Parte1.java "$L"/src/grande/Parte2.java "$L"/src/grande/Parte3.java \
		"$L"/src/grande/Parte4.java "$L"/src/grande/Parte5.java "$L"/src/grande/Parte6.java
	( cd "$L/clases" && "$JDK_BIN/jar" cf "$L/proyecto/jar/grande.jar" grande )
	echo "   $(cd "$L/clases" && find grande -name '*.class' | wc -l) clases, 70 000 métodos, $(wc -c <"$L/proyecto/jar/grande.jar") bytes"
	sed 's/com\.ejemplo\.validar/com.ejemplo.limite/' "$AQUI/AndroidManifest.xml" \
		>"$L/proyecto/AndroidManifest.xml"
	cp "$AQUI/res/values/strings.xml" "$L/proyecto/res/values/strings.xml"
	printf 'package com.ejemplo.limite;\n\nimport android.app.Activity;\nimport android.os.Bundle;\nimport android.widget.TextView;\n\npublic class MainActivity extends Activity {\n    @Override\n    public void onCreate(Bundle estado) {\n        super.onCreate(estado);\n        setContentView(new TextView(this));\n    }\n}\n' \
		>"$L/proyecto/src/com/ejemplo/limite/MainActivity.java"
	set +e
	( cd "$L/proyecto" && "$APKC" build --linker go --compiler go --signer go \
		--android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$KS" ) \
		>"$L/build.log" 2>&1
	rc=$?
	set -e
	sed 's/^/   | /' "$L/build.log"
	[ "$rc" -eq 0 ] ||
		falla "el build con 70 000 métodos salió con $rc y el dexer no repartió el bucket (la guarda de los índices de 16 bits es la de apkc/dexinstr.go)"
	[ -f "$L/proyecto/build/app.apk" ] ||
		falla "el build dijo terminar y no dejó $L/proyecto/build/app.apk"
	verifica "$L/proyecto/build/app.apk" "classes.dex classes2.dex classes3.dex "

	# el techo se cumple en cada fichero, que es la propiedad que se está midiendo y no un efecto
	# colateral: un dex con más method_id de los que caben en un u16 no se puede ejecutar
	echo "-- method_id de cada dex (techo 65 536)"
	for d in "$SALIDA"/dex/classes*.dex; do
		metodos=$("$DEXDUMP" -f "$d" | sed -n 's/.*method_ids_size *: *\([0-9]*\).*/\1/p' | head -1)
		[ -n "$metodos" ] || falla "dexdump no dice los method_id de $(basename "$d")"
		[ "$metodos" -le 65536 ] ||
			falla "$(basename "$d") lleva $metodos method_id, más de los 65 536 que caben en un índice de 16 bits"
		echo "   $(basename "$d"): $metodos"
	done

	# el reparto no pierde ni duplica una clase al cerrar el grupo: las 700 del jar están entre
	# los dos dex de dependencias, cada una una sola vez. La lista de cada uno sirve después para
	# emparejar la invocación con la definición.
	for n in 2 3; do
		"$DEXDUMP" -f "$SALIDA/dex/classes$n.dex" |
			sed -n "s/.*Class descriptor  : '\(Lgrande\/C[0-9]*;\)'/\1/p" | sort >"$L/define$n.txt"
	done
	cat "$L/define2.txt" "$L/define3.txt" | sort >"$L/clases.txt"
	definidas=$(sort -u "$L/clases.txt" | wc -l)
	[ "$definidas" -eq 700 ] ||
		falla "los dex de dependencias definen $definidas clases grande.Cn distintas y hay 700"
	[ "$(wc -l <"$L/clases.txt")" -eq 700 ] ||
		falla "una clase grande.Cn está en los dos dex de dependencias: el reparto la duplicó"
	echo "   los dos dex de dependencias definen las 700 clases grande.C0…C699, sin huecos ni repetidas"

	# la frontera del reparto: un método que un dex de dependencias invoca y el otro define. El
	# par se busca en vez de fijar una clase, porque dónde cierra cada grupo no lo decide el guion
	metodo_cruzado=
	for par in 2:3 3:2; do
		invoca=${par%:*}
		define=${par#*:}
		clase=$(
			"$DEXDUMP" -d "$SALIDA/dex/classes$invoca.dex" |
				sed -n "s/.*invoke-[a-z/]*.*\(Lgrande\/C[0-9]*;\)\.m0:()I.*/\1/p" |
				grep -F -m1 -f "$L/define$define.txt" || true
		)
		[ -n "$clase" ] || continue
		metodo_cruzado="$clase.m0"
		verifica_metodo "$metodo_cruzado" "$clase" "$SALIDA/dex/classes$invoca.dex"
		break
	done
	[ -n "$metodo_cruzado" ] ||
		falla "ningún dex de dependencias invoca un método que defina el otro: el jar no cruzó la frontera del reparto"
	echo "   el bucket de 70 000 métodos se repartió en tres dex y cada uno cabe en los índices de 16 bits"
fi

# ---------------------------------------------------------------- opcional: el aparato
if [ -n "${SERIE_APARATO:-}" ]; then
	echo
	echo "== aparato $SERIE_APARATO: la app multi-dex arranca y llama a la clase del segundo dex"
	adb -s "$SERIE_APARATO" uninstall com.ejemplo.validar >/dev/null 2>&1 || true
	adb -s "$SERIE_APARATO" install "$P2/build/app.apk" >/dev/null
	adb -s "$SERIE_APARATO" shell am start -n com.ejemplo.validar/.MainActivity >/dev/null
	sleep 2
	adb -s "$SERIE_APARATO" shell uiautomator dump /sdcard/validar.xml >/dev/null 2>&1
	texto=$(adb -s "$SERIE_APARATO" shell cat /sdcard/validar.xml | grep -o 'text="[^"]*d20=20[^"]*"' | head -1)
	[ -n "$texto" ] || falla "el aparato no enseñó el texto con d20=20"
	echo "   $texto"
fi

echo
echo "OK: los dos APK llevan los dex esperados, la firma pasa por los tres esquemas y los oráculos ven el método del jar"
if [ "${PROBAR_LIMITE:-0}" = "1" ]; then
	echo "OK: el bucket de 70 000 métodos se repartió en tres dex, ninguno pasa de 65 536 method_id y el APK verifica"
fi
