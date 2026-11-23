#!/bin/sh
# Sonda de aceptación del frente de dependencias: una app que llama a una clase que no está en
# src/ sino en un jar local. El jar lo construye este guion con el JDK (sin red, sin Maven, sin
# apkc-deps) y la app se compila por los dos caminos:
#
#   nativo  javac con -classpath jar/saludo.jar, y d8 para la app y para la dependencia
#   go      apkc build --linker go --compiler go, que tiene que leer jar/*.jar sin javac ni d8
#
# Después compara el dex de la app de cada camino y comprueba con dexdump que los dos dex
# resuelven e invocan Lsaludo/Saludo;.saludo, y que el apk del camino Go empaqueta la clase.
#
#   sh banco/deps-ejemplo/construir.sh
#
# Variables: APKC (binario), ANDROID_JAR, JAVA_HOME, D8, DEXDUMP, APKC_WEB_CACHE.
# Códigos de salida:
#   0  los dos caminos compilan y la dependencia resuelve
#   3  el compilador Go todavía rechaza jar/ (objetivo del frente, no un fallo oculto)
#   1  cualquier otro fallo
set -eu

AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/deps-fixture}
SALIDA=$CACHE/salida
mkdir -p "$CACHE" "$SALIDA"

# El binario y el android.jar, como en banco/apps-ejemplo: se construyen si faltan.
APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	( cd "$RAIZ/apkc" && CGO_ENABLED=0 go build -o "$APKC" . )
fi
JAR=${ANDROID_JAR:-$(sh "$RAIZ/tools/entorno.sh" jar)}
if [ ! -f "$JAR" ]; then
	echo "falta android.jar: $JAR" >&2
	exit 1
fi
SDK=$(CDPATH= cd -- "$(dirname -- "$(dirname -- "$(dirname -- "$JAR")")")" && pwd)
D8=${D8:-$(ls "$SDK"/build-tools/*/d8 2>/dev/null | sort -V | head -1)}
DEXDUMP=${DEXDUMP:-$(ls "$SDK"/build-tools/*/dexdump 2>/dev/null | sort -V | head -1)}
[ -x "${D8:-}" ] || { echo "no encuentro d8 en $SDK/build-tools" >&2; exit 1; }
[ -x "${DEXDUMP:-}" ] || { echo "no encuentro dexdump en $SDK/build-tools" >&2; exit 1; }

# JDK 17 si está: el d8 de build-tools 34.0.0 no lee los .class de javac 21 (major 65).
JDK_BIN=${JAVA_HOME:+$JAVA_HOME/bin}
if [ -z "${JDK_BIN:-}" ] || [ ! -x "$JDK_BIN/javac" ]; then
	JDK_BIN=
	for j in "$HOME/opt/jdk17" /usr/lib/jvm/java-17-openjdk-amd64 /usr/lib/jvm/java-17-openjdk; do
		if [ -x "$j/bin/javac" ]; then
			JDK_BIN=$j/bin
			break
		fi
	done
fi
[ -n "${JDK_BIN:-}" ] || JDK_BIN=$(dirname "$(command -v javac)")
RELEASE=
if "$JDK_BIN/javac" --release 17 -version >/dev/null 2>&1; then
	RELEASE="--release 17"
fi
echo "JDK   $JDK_BIN ${RELEASE:-(sin --release)}"
echo "d8    $D8"
echo "jar   $JAR"

KS=${KEYSTORE:-$CACHE/debug.keystore}
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

rm -rf "$AQUI/build" "$SALIDA/nativo" "$SALIDA/go"
mkdir -p "$SALIDA/nativo/classes" "$SALIDA/nativo/app" "$SALIDA/nativo/deps" \
	"$SALIDA/dep/classes" "$SALIDA/go/dex" "$AQUI/jar"

# 1. El jar de la dependencia, con el JDK. Es el único artefacto que no se versiona.
echo "== dependencia"
"$JDK_BIN/javac" $RELEASE -d "$SALIDA/dep/classes" "$AQUI/dep/saludo/Saludo.java"
"$JDK_BIN/jar" cf "$AQUI/jar/saludo.jar" -C "$SALIDA/dep/classes" .
echo "   $AQUI/jar/saludo.jar ($(wc -c < "$AQUI/jar/saludo.jar") bytes)"

# 2. Camino nativo: javac resuelve el jar con -classpath y d8 dexa la app y la dependencia.
echo "== nativo (javac + d8)"
# R.java no entra: lo genera el enlazador de apkc y, si quedó de una vuelta anterior, el
# dex nativo cambiaría según el historial del árbol en vez de según el proyecto.
"$JDK_BIN/javac" $RELEASE -classpath "$JAR:$AQUI/jar/saludo.jar" -d "$SALIDA/nativo/classes" \
	$(find "$AQUI/src" -name '*.java' ! -name 'R.java' | sort)
"$D8" --lib "$JAR" --min-api 21 --classpath "$AQUI/jar/saludo.jar" --output "$SALIDA/nativo/app" \
	$(find "$SALIDA/nativo/classes" -name '*.class' | sort)
"$D8" --lib "$JAR" --min-api 21 --output "$SALIDA/nativo/deps" \
	$(find "$SALIDA/dep/classes" -name '*.class' | sort)
echo "   app  $SALIDA/nativo/app/classes.dex ($(wc -c < "$SALIDA/nativo/app/classes.dex") bytes)"
echo "   dep  $SALIDA/nativo/deps/classes.dex ($(wc -c < "$SALIDA/nativo/deps/classes.dex") bytes)"

# 3. Camino Go: el compilador propio lee src/ y jar/*.jar sin levantar javac ni d8.
echo "== go (apkc build --linker go --compiler go)"
JAR_FLAG=
if "$APKC" build --help 2>&1 | grep -q '^  -jar '; then
	JAR_FLAG="--jar $AQUI/jar/saludo.jar"
	echo "   el binario anuncia --jar: se pasa $JAR_FLAG"
else
	echo "   el binario no anuncia --jar: se espera que lea jar/ (el formato de apkc-deps)"
fi
set +e
( cd "$AQUI" && "$APKC" build --linker go --compiler go --android-jar "$JAR" \
	--sdk-version 34 --codename 14 --keystore "$KS" --signer go $JAR_FLAG ) \
	>"$SALIDA/go.log" 2>&1
rc=$?
set -e
if [ "$rc" -ne 0 ]; then
	echo "   el camino Go falló (rc $rc):" >&2
	sed 's/^/   | /' "$SALIDA/go.log" >&2
	if grep -q 'd8 of dependency bucket' "$SALIDA/go.log"; then
		echo "OBJETIVO PENDIENTE: el dex de las dependencias todavía lo escribe d8; esta sonda es su aceptación." >&2
		exit 3
	fi
	exit 1
fi
if [ ! -f "$AQUI/build/app.apk" ]; then
	echo "el camino Go dijo ok pero no dejó build/app.apk" >&2
	exit 1
fi
sed 's/^/   | /' "$SALIDA/go.log"

# 4. El apk tiene que llevar la dependencia, no solo compilar contra ella.
unzip -o -q "$AQUI/build/app.apk" 'classes*.dex' -d "$SALIDA/go/dex"
APP_DEX_GO=
for d in "$SALIDA"/go/dex/classes*.dex; do
	if "$DEXDUMP" -f "$d" | grep -q 'Lcom/ejemplo/deps/MainActivity;'; then
		APP_DEX_GO=$d
	fi
done
[ -n "$APP_DEX_GO" ] || { echo "el apk Go no trae MainActivity en ningún dex" >&2; exit 1; }
if ! "$DEXDUMP" -f "$SALIDA"/go/dex/classes*.dex | grep -q 'Lsaludo/Saludo;'; then
	echo "el apk Go no empaqueta Lsaludo/Saludo;: la dependencia no llegó al dex" >&2
	exit 1
fi

# 5. Los dos caminos tienen que invocar el método del jar, y se documenta si el dex difiere.
for par in "nativo:$SALIDA/nativo/app/classes.dex" "go:$APP_DEX_GO"; do
	camino=${par%%:*}
	dex=${par#*:}
	if ! "$DEXDUMP" -d "$dex" | grep -q 'Lsaludo/Saludo;.saludo'; then
		echo "el dex de $camino no invoca Lsaludo/Saludo;.saludo" >&2
		exit 1
	fi
	echo "   $camino invoca Lsaludo/Saludo;.saludo"
done
echo "== comparación de los dex de la app"
lista_clases() {
	"$DEXDUMP" -f "$1" | sed -n "s/.*Class descriptor  : '\(.*\)'/\1/p" | sort
}
lista_clases "$SALIDA/nativo/app/classes.dex" >"$SALIDA/nativo/clases.txt"
lista_clases "$APP_DEX_GO" >"$SALIDA/go/clases.txt"
if cmp -s "$SALIDA/nativo/app/classes.dex" "$APP_DEX_GO"; then
	echo "   idénticos byte a byte ($(wc -c < "$APP_DEX_GO") bytes)"
else
	echo "   distintos: nativo $(wc -c < "$SALIDA/nativo/app/classes.dex") bytes," \
		"go $(wc -c < "$APP_DEX_GO") bytes"
	cmp "$SALIDA/nativo/app/classes.dex" "$APP_DEX_GO" 2>&1 | head -1 | sed 's/^/   /' || true
	solo_go=$(comm -13 "$SALIDA/nativo/clases.txt" "$SALIDA/go/clases.txt" | tr '\n' ' ')
	solo_nativo=$(comm -23 "$SALIDA/nativo/clases.txt" "$SALIDA/go/clases.txt" | tr '\n' ' ')
	echo "   clases solo en el dex Go (el enlazador de recursos añade R.java): ${solo_go:-(ninguna)}"
	echo "   clases solo en el dex nativo: ${solo_nativo:-(ninguna)}"
	echo "   la diferencia es de emisor y de clases generadas, no de resolución: los dos dex"
	echo "   invocan el jar y el apk Go empaqueta Lsaludo/Saludo; (comprobado con dexdump)."
fi
echo "OK: los dos caminos compilan la app contra jar/saludo.jar"
