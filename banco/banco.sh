#!/data/data/com.termux/files/usr/bin/sh
# Banco de ejecución del fork. No es un test de bytes ni una comparación de `javap`: **ejecuta** lo
# que este fork compila y lo compara con lo que ejecuta la JVM y con lo que ejecuta `d8` sobre el
# mismo fuente. Sirve para adquirir instrucciones sin montar una app: el `.dex` va a
# /data/local/tmp y se corre con `app_process` por `rish`, sin instalar nada, sin pantalla y sin
# permisos de nada.
#
#   uso: sh banco/banco.sh              # con el binario de siempre
#        APKC=$PREFIX/tmp/apkc-gate sh banco/banco.sh
#
# Tres salidas y un veredicto. El código de salida del proceso es el dato: `p.Arranque` termina con
# `System.exit(total)`, así que cualquier error de emisor, traductor o asignación de registros se ve
# como un rc distinto o como un VerifyError en el stderr.
set -e
P=/data/data/com.termux/files/usr
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
APKC=${APKC:-$P/tmp/apkc-gate}
JAR=${ANDROID_JAR:-$HOME/Android/platforms/android-34/android.jar}
BT=${BT:-$HOME/Android/build-tools/34.0.0}
JDK=$P/lib/jvm/java-21-openjdk
TRABAJO=${TRABAJO:-$P/tmp/banco}
DEXDUMP=/apex/com.android.art/bin/dexdump
RISH=${RISH:-rish}

[ -x "$APKC" ] || { echo "banco: falta el binario en $APKC (go build -o $APKC . en apkc/)"; exit 1; }
command -v "$RISH" >/dev/null || { echo "banco: sin rish, el banco no puede ejecutar dex"; exit 1; }
# el trabajo tiene que quedar bajo el tmp de Termux: el guion lo borra al empezar
case $TRABAJO in
	$P/tmp/*) ;;
	*) echo "banco: TRABAJO tiene que estar bajo $P/tmp (es $TRABAJO)"; exit 1 ;;
esac

rm -rf "$TRABAJO"
mkdir -p "$TRABAJO/jc" "$TRABAJO/scr" "$TRABAJO/nuestro" "$TRABAJO/d8" "$TRABAJO/d8desde"

# 1. las clases de referencia: javac hace las dos, y nuestro emisor hace Ver
"$JDK/bin/javac" -d "$TRABAJO/scr" "$AQUI/banco/src/p/Ver.java"
"$JDK/bin/javac" -cp "$TRABAJO/scr" -d "$TRABAJO/jc" "$AQUI/banco/src/p/Arranque.java"
"$APKC" compile --android-jar "$JAR" -o "$TRABAJO/nuestro" "$AQUI/banco/src/p/Ver.java" >"$TRABAJO/compile.log" 2>&1 || {
	echo "banco: apkc compile fallo"; tail -3 "$TRABAJO/compile.log"; exit 1; }
cp "$TRABAJO/jc/p/Arranque.class" "$TRABAJO/nuestro/p/"

# 2. los tres dex: el nuestro (nuestra Ver + javac Arranque), el de d8 sobre javac, y el de d8
#    sobre nuestra Ver (para separar "el emisor escribe bien" de "el traductor dexa bien")
"$APKC" dex -o "$TRABAJO/nuestro.dex" "$TRABAJO/nuestro" >"$TRABAJO/dex.log" 2>&1 || {
	echo "banco: apkc dex fallo"; tail -3 "$TRABAJO/dex.log"; exit 1; }
"$BT/d8" --release --min-api 21 --lib "$JAR" --output "$TRABAJO/d8" \
	"$TRABAJO/scr/p/Ver.class" "$TRABAJO/jc/p/Arranque.class" >/dev/null 2>&1
"$BT/d8" --release --min-api 21 --output "$TRABAJO/d8desde" \
	"$TRABAJO/nuestro/p/Ver.class" "$TRABAJO/jc/p/Arranque.class" >/dev/null 2>&1

# 3. la JVM, que es el oráculo que no necesita teléfono: código de salida **y texto**
set +e
java_out=$("$JDK/bin/java" -cp "$TRABAJO/scr:$TRABAJO/jc" p.Arranque 2>&1); jvm=$?
set -e
echo "  jvm (javac): rc=$jvm, salida: $(printf '%s' "$java_out" | tr '\n' '|')"

# 4. el aparato ejecuta los tres dex, sin instalar nada
sube() { # nombre -> /data/local/tmp/ nombre .dex
	base64 -w0 "$2" | "$RISH" -c "toybox base64 -d > /data/local/tmp/$1.dex"
}
corre() { # alias -> salida y rc del app_process. La salida se deja en un fichero del aparato y se baja
	# en base64: con una tuberia directa el cliente de rish deja el texto del programa en el terminal y
	# devuelve solo la ultima linea, y el banco compararia contra nada (visto en la primera corrida).
	i=0
	while [ $i -lt 3 ]; do
		"$RISH" -c "export CLASSPATH=/data/local/tmp/$1.dex; /system/bin/app_process64 /data/local/tmp p.Arranque > /data/local/tmp/$1.out 2>&1; echo rc=\$? >> /data/local/tmp/$1.out" >/dev/null 2>&1
		# la bajada se redirige a un fichero local: cuando rish ve un terminal duplica ahi lo que imprime
		"$RISH" -c "toybox base64 /data/local/tmp/$1.out" > "$TRABAJO/corre.b64" 2>/dev/null
		sal=$(base64 -d < "$TRABAJO/corre.b64" | tr -d '\r')
		case $sal in *"rc="*) break ;; esac
		i=$((i + 1))
		sleep 2
	done
	printf '%s\n' "$sal"
}
veredicto() { # alias de dex, descripcion
	s=$1; t=$2
	set +e
	sal=$(corre "$s"); rcs=$(echo "$sal" | sed -n 's/^rc=//p' | tail -1)
	set -e
	txt=$(printf '%s\n' "$sal" | sed '/^rc=/d')
	v=$(echo "$sal" | grep -c "VerifyError\|NoClassDefFound" || true)
	if [ "$rcs" = "$jvm" ] && [ "$txt" = "$java_out" ] && [ "$v" = "0" ]; then
		echo "  $t: rc=$rcs y salida igual a la JVM  ok"
	else
		echo "  $t: rc=$rcs y salida «$(printf '%s' "$txt" | tr '\n' '|')» contra rc=$jvm «$(printf '%s' "$java_out" | tr '\n' '|')»  FALLO"
		printf '%s\n' "$sal" | sed 's/^/      /'
		exit 1
	fi
	"$RISH" -c "$DEXDUMP /data/local/tmp/$s.dex >/dev/null 2>&1; echo \$? > /data/local/tmp/$s.dv" >/dev/null 2>&1
	"$RISH" -c "toybox cat /data/local/tmp/$s.dv" > "$TRABAJO/dv.txt" 2>/dev/null
	dv=$(tr -d '\r' < "$TRABAJO/dv.txt")
	[ "$dv" = "0" ] && echo "    dexdump verificando: rc 0" || { echo "    dexdump rc=$dv"; exit 1; }
}
sube bnuestro "$TRABAJO/nuestro.dex"
sube bd8 "$TRABAJO/d8/classes.dex"
sube bd8desde "$TRABAJO/d8desde/classes.dex"
veredicto bnuestro "nuestro dex (Go de punta a punta)"
veredicto bd8 "dex de d8 sobre javac"
veredicto bd8desde "dex de d8 sobre nuestra Ver"

# 5. el doble oráculo: se informa, no se exige. `d8` sobre nuestra `Ver` y sobre la de javac no salen
#    iguales porque el código de las dos clases no es el mismo (javac pliega `new StringBuilder();
#    append("...")` en el constructor `StringBuilder(String)` y nosotros escribimos las dos filas, §53),
#    y eso es una diferencia de forma ya aceptada. Lo que sí se exige es que las dos *ejecuten* igual,
#    que es lo que comprueban los veredictos de arriba.
if cmp -s "$TRABAJO/d8/classes.dex" "$TRABAJO/d8desde/classes.dex"; then
	echo "  d8 sobre javac y d8 sobre nuestra Ver: $(wc -c < "$TRABAJO/d8/classes.dex" | tr -d ' ') bytes idénticos"
else
	echo "  d8 sobre javac y d8 sobre nuestra Ver: $(wc -c < "$TRABAJO/d8/classes.dex" | tr -d ' ') y $(wc -c < "$TRABAJO/d8desde/classes.dex" | tr -d ' ') bytes (forma distinta: javac pliega el primer append en el constructor, §53)"
fi

# 6. la interoperabilidad que ningun oraculo de bytes daba: se ejecuta la Ver emitida por nosotros con
#    el Arranque de javac, en el arbol de paquetes que exige el classpath, con `-Xverify:all`. El
#    StackMapTable estaba en el plan 5.23 y se cerro en el §65: si vuelve a faltar, la JVM lo dice.
mkdir -p "$TRABAJO/mix/p"
cp "$TRABAJO/jc/p/Arranque.class" "$TRABAJO/mix/p/"
cp "$TRABAJO/nuestro/p/Ver.class" "$TRABAJO/mix/p/"
set +e
mixto=$("$JDK/bin/java" -Xverify:all -cp "$TRABAJO/mix" p.Arranque 2>&1); mezcla=$?
set -e
if echo "$mixto" | grep -q "VerifyError\|StackMapTable\|Expecting a stackmap frame"; then
	echo "  jvm con nuestra clase emitida: VerifyError  FALLO"
	echo "$mixto" | head -3 | sed 's/^/      /'; exit 1
elif [ "$mezcla" != "$jvm" ] || [ "$mixto" != "$java_out" ]; then
	echo "  jvm con nuestra clase emitida: rc=$mezcla (la JVM dice $jvm)  FALLO"
	echo "$mixto" | head -3 | sed 's/^/      /'; exit 1
else
	echo "  jvm con nuestra clase emitida (Xverify:all): rc=$mezcla y salida igual a la JVM  ok"
fi
echo "banco: en orden"
