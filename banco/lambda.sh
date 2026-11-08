#!/data/data/com.termux/files/usr/bin/sh
# Banco de las lambdas desugradas (5.22, §110 y §111). `javac --release 8` deja el `invokedynamic`
# y el `lambda$…`; este fork lo convierte en una clase sintética con sus campos de captura. Se
# ejecuta el dex en el aparato con `app_process64` y se exige la misma salida que la JVM para los
# cinco casos: estática sin capturas, captura de un local, captura de `this`, `this::m` y `Tipo::m`.
#
#   uso: sh banco/lambda.sh
#        APKC=$PREFIX/tmp/apkc-gate sh banco/lambda.sh
set -e
P=/data/data/com.termux/files/usr
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
APKC=${APKC:-$P/tmp/apkc-gate}
JDK=$P/lib/jvm/java-21-openjdk
TRABAJO=$P/tmp/banco-lambda
DEXDUMP=/apex/com.android.art/bin/dexdump
RISH=${RISH:-}
if [ -z "$RISH" ]; then
	if command -v rish >/dev/null 2>&1; then
		RISH=rish
	elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
		RISH=sudo
	fi
fi
[ -x "$APKC" ] || { echo "lambda: falta el binario en $APKC (go build -o $APKC . en apkc/)"; exit 1; }
[ -n "$RISH" ] || { echo "lambda: ni rish ni sudo; sin uno de los dos no se puede ejecutar el dex"; exit 1; }

PRELUDIO_APARATO=
if [ "$RISH" = "sudo" ]; then
	zygote=$(sudo sh -c 'pidof zygote64 2>/dev/null || pidof zygote 2>/dev/null' | tr -d '\r')
	if [ -n "$zygote" ]; then
		PRELUDIO_APARATO=$(sudo sh -c "tr '\0' '\n' < /proc/$zygote/environ |
			grep -E '^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH|SYSTEMSERVERCLASSPATH|STANDALONE_SYSTEMSERVER_JARS)=' |
			sed 's/^/export /'")
	fi
fi
enAparato() {
	case $RISH in
	sudo) if [ -n "$PRELUDIO_APARATO" ]; then sudo sh -c "$PRELUDIO_APARATO; $1"; else sudo sh -c "$1"; fi ;;
	*) "$RISH" -c "$1" ;;
	esac
}

rm -rf "$TRABAJO"
mkdir -p "$TRABAJO/src" "$TRABAJO/clases"
cat >"$TRABAJO/src/Lam.java" <<'EOF'
public class Lam {
    interface F { int f(int x); }
    static F uno() { return x -> x + 1; }
    public static void main(String[] a) { System.out.println(uno().f(41)); }
}
EOF
cat >"$TRABAJO/src/LamLoc.java" <<'EOF'
public class LamLoc {
    interface F { int f(int x); }
    static F con(int y) { return x -> x + y; }
    public static void main(String[] a) { System.out.println(con(5).f(37)); }
}
EOF
cat >"$TRABAJO/src/LamThis.java" <<'EOF'
public class LamThis {
    interface F { int f(int x); }
    int y;
    F captura() { return x -> x + y; }
    public static void main(String[] a) { LamThis o = new LamThis(); o.y = 5; System.out.println(o.captura().f(37)); }
}
EOF
cat >"$TRABAJO/src/LamRef.java" <<'EOF'
public class LamRef {
    interface F { int f(int x); }
    int y;
    F ref() { return this::suma; }
    int suma(int x) { return x + y; }
    public static void main(String[] a) { LamRef o = new LamRef(); o.y = 5; System.out.println(o.ref().f(37)); }
}
EOF
cat >"$TRABAJO/src/LamUnb.java" <<'EOF'
public class LamUnb {
    interface F { int f(LamUnb o, int x); }
    int y;
    F ref() { return LamUnb::suma; }
    int suma(int x) { return x + y; }
    public static void main(String[] a) { LamUnb o = new LamUnb(); o.y = 5; System.out.println(o.ref().f(o, 37)); }
}
EOF
"$JDK/bin/javac" --release 8 -g:lines,source -d "$TRABAJO/clases" "$TRABAJO/src"/*.java 2>/dev/null
"$APKC" dex -o "$TRABAJO/lam.dex" "$TRABAJO/clases" >"$TRABAJO/dex.log" 2>&1 || {
	echo "lambda: apkc dex fallo"; tail -3 "$TRABAJO/dex.log"; exit 1; }

base64 -w0 "$TRABAJO/lam.dex" | enAparato "toybox base64 -d > /data/local/tmp/lam.dex"
for c in Lam LamLoc LamThis LamRef LamUnb; do
	set +e
	jvm_out=$("$JDK/bin/java" -cp "$TRABAJO/clases" "$c" 2>&1); jvm=$?
	set -e
	enAparato "export CLASSPATH=/data/local/tmp/lam.dex; /system/bin/app_process64 /data/local/tmp $c > /data/local/tmp/lam-$c.out 2>&1; echo rc=\$? >> /data/local/tmp/lam-$c.out" >/dev/null 2>&1
	enAparato "toybox base64 /data/local/tmp/lam-$c.out" > "$TRABAJO/$c.b64" 2>/dev/null
	sal=$(base64 -d < "$TRABAJO/$c.b64" | tr -d '\r')
	rcs=$(echo "$sal" | sed -n 's/^rc=//p' | tail -1)
	txt=$(printf '%s\n' "$sal" | sed '/^rc=/d')
	echo "  $c: jvm rc=$jvm «$jvm_out» · aparato rc=$rcs «$(printf '%s' "$txt" | tr '\n' '|')»"
	[ "$rcs" = "$jvm" ] && [ "$txt" = "$jvm_out" ] || { printf '%s\n' "$sal" | sed 's/^/      /'; exit 1; }
done
enAparato "$DEXDUMP /data/local/tmp/lam.dex >/dev/null 2>&1; echo \$?" > "$TRABAJO/dv.txt" 2>/dev/null
dv=$(tr -d '\r' < "$TRABAJO/dv.txt")
[ "$dv" = "0" ] || { echo "lambda: dexdump rc=$dv"; exit 1; }
echo "  dexdump verificando: rc 0"
echo "lambda: las cinco lambdas se desugraron y corren igual que la JVM"
