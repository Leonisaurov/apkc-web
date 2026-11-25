#!/bin/sh
# Sonda de endurecimiento de las cinco apps de ejemplo. No es un gate: es una sonda reproducible
# que, por cada app, compila con el camino todo-Go, la instala en Droidloom por adb, la arranca,
# ejecuta `apkc preview` con una secuencia corta de órdenes, aplica la misma secuencia al aparato
# tocando por `resource-id`, vuelca `uiautomator dump` de los dos lados y escribe un informe que
# compara texto, vistas y filas, marcando cada diferencia.
#
#   sh banco/apps-ejemplo/endurecer.sh
#
# Variables:
#   SERIE            dispositivo de adb            (por defecto 10.177.0.2:5555)
#   APKC_WEB_CACHE   artefactos y apks             (por defecto $HOME/.cache/apkc-web/apps-mejorar)
#   APKC, ANDROID_JAR, KEYSTORE                    como en construir.sh
#   DERIVA           segundos de la prueba de deriva del cronómetro (0 la salta; por defecto 60)
#
# El informe queda en $APKC_WEB_CACHE/sonda/informe.md y cada pieza por separado al lado.
set -u
AQUI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RAIZ=$(CDPATH= cd -- "$AQUI/../.." && pwd)
CACHE=${APKC_WEB_CACHE:-$HOME/.cache/apkc-web/apps-mejorar}
SALIDA=$CACHE/sonda
SERIE=${SERIE:-10.177.0.2:5555}
DERIVA=${DERIVA:-60}
mkdir -p "$CACHE" "$SALIDA"

APKC=${APKC:-$CACHE/apkc}
if [ ! -x "$APKC" ]; then
	echo "compilando apkc en $APKC"
	sh "$RAIZ/tools/materializar-emulador.sh" "$RAIZ"
	( cd "$RAIZ/apkc" && CGO_ENABLED=0 go build -o "$APKC" . )
fi
JAR=${ANDROID_JAR:-$(sh "$RAIZ/tools/entorno.sh" jar)}
KS=${KEYSTORE:-$CACHE/debug.keystore}
if [ ! -f "$KS" ]; then
	"$APKC" debug-keystore --out "$KS" >/dev/null
fi

INFORME=$SALIDA/informe.md

# La pantalla del emulador se apaga sola y con ella desaparece la ventana de la app (uiautomator
# vuelca entonces la home en negro): se despierta y se pide que no se duerma mientras dura la sonda.
adb -s "$SERIE" shell input keyevent KEYCODE_WAKEUP >/dev/null 2>&1 || true
adb -s "$SERIE" shell svc power stayon true >/dev/null 2>&1 || true
{
	echo "# Sonda de endurecimiento de las apps de ejemplo"
	echo
	echo "- fecha: $(date -u '+%Y-%m-%d %H:%M UTC')"
	echo "- serie: \`$SERIE\` ($(adb -s "$SERIE" shell getprop ro.build.version.release 2>/dev/null | tr -d '\r'))"
	echo "- apkc: \`$APKC\`"
	echo "- la secuencia se aplica dos veces: con \`--pulsar/--escribir/--avanzar\` en \`apkc preview\` y"
	echo "  con \`input tap\`/\`input text\` y espera real en el aparato; los ids del preview salen de la"
	echo "  primera pasada, así que la sonda no depende de números escritos a mano"
	echo
	echo "Cada AVISO es una diferencia entre los dos lados; no se oculta ninguna."
	echo
} >"$INFORME"

# La secuencia de cada app, en un formato que las dos mitades entienden:
#   pulsar:<nombre>          -> --pulsar id/<nombre> | tocar el nodo id/<nombre>
#   escribir:<nombre>=<txt>  -> --escribir id/<nombre>/<txt> | tocar y teclear
#   avanzar:<ms>             -> --avanzar <ms> | esperar de reloj
# Sin espacios en el texto: `input text` los quiere como %s y aquí no hacen falta.
secuencia_de() {
	case $1 in
	lienzo) echo "pulsar:grosor pulsar:grosor pulsar:borrar" ;;
	notas) echo "escribir:editor=uno pulsar:guardar pulsar:nueva escribir:editor=dos pulsar:guardar pulsar:anterior escribir:editor=dos2 pulsar:guardar" ;;
	chatbt) echo "escribir:entrada=hola pulsar:enviar pulsar:enviar pulsar:buscar" ;;
	tareas) echo "escribir:nueva=pan pulsar:agregar escribir:nueva=informe pulsar:trabajo pulsar:agregar pulsar:completar" ;;
	cronometro) echo "pulsar:iniciar avanzar:1200 pulsar:vuelta avanzar:600 pulsar:pausar" ;;
	esac
}

# Qué explica de antemano cada diferencia conocida: va al informe para que un AVISO no se lea como
# un fallo del aparato cuando es una frontera del modelo.
nota_de() {
	case $1 in
	lienzo) echo "frontera: la vista propia no puede ir en el layout (con la etiqueta `<com.ejemplo.lienzo.Lienzo>` el framework aborta al inflar con «You must supply a layout_height attribute» aunque el atributo esté en el binario, y el emisor no resuelve el constructor de la vista escrita en otro fichero), así que la app sigue montando la vista desde código y no tiene layout que modelar: el preview no tiene pantalla que comparar" ;;
	notas) echo "se crean dos notas, se edita la segunda y se comprueba que sobreviven a force-stop + relanzamiento; en el aparato \`input text\` escribe donde está el cursor (el editor acumula) y en el preview \`--escribir\` reemplaza el texto entero: esa diferencia es del instrumento, no de la app" ;;
	chatbt) echo "el intérprete de preview no tiene BluetoothAdapter en sus stubs (devuelve null), así que el estado del adaptador difiere por diseño; los mensajes y el eco sí se comparan" ;;
	tareas) echo "tres arreglos paralelos en SharedPreferences; el filtro se mueve con los botones de categoría y con Filtrar" ;;
	cronometro) echo "el reloj de pared no se finge en el preview: el tiempo difiere y se comparan la forma, los pulsos y las marcas; la deriva se mide aparte contra 60 s de reloj real" ;;
	esac
}

# Expande la secuencia a argumentos de `apkc preview`, con los ids sacados del modelo base.
argumentos_preview() {
	base=$1
	secuencia=$2
	salida=""
	for paso in $secuencia; do
		orden=${paso%%:*}
		resto=${paso#*:}
		case $orden in
		pulsar)
			ident=$(python3 "$AQUI/sonda.py" id "$base" "$resto")
			salida="$salida --pulsar $ident"
			;;
		escribir)
			campo=${resto%%=*}
			texto=${resto#*=}
			ident=$(python3 "$AQUI/sonda.py" id "$base" "$campo")
			salida="$salida --escribir $ident/$texto"
			;;
		avanzar)
			salida="$salida --avanzar $resto"
			;;
		esac
	done
	echo "$salida"
}

# La secuencia se aplica de una vez con las coordenadas de un único volcado: Droidloom retira la
# ventana de la app a los pocos segundos y solo la devuelve con force-stop + start, que borraría el
# estado en memoria. Un volcado por toque llegaba tarde y la sonda se quedaba mirando su home.
aplicar_en_aparato() {
	paquete=$1
	secuencia=$2
	# shellcheck disable=SC2086
	python3 "$AQUI/sonda.py" secuencia "$SERIE" "$paquete" "$SALIDA/$app.inicio.xml" $secuencia ||
		echo "   AVISO: la secuencia no se pudo aplicar entera en el aparato"
}

for par in \
	lienzo:com.ejemplo.lienzo \
	notas:com.ejemplo.notas \
	chatbt:com.ejemplo.chatbt \
	tareas:com.ejemplo.tareas \
	cronometro:com.ejemplo.cronometro
do
	app=${par%%:*}
	paquete=${par#*:}
	# lienzo monta su vista propia desde código y no lleva layout: `apkc preview` no tiene pantalla
	# que modelar y el inflado del AXML con la etiqueta de la vista aborta (medido: «You must supply
	# a layout_height attribute» con el atributo presente). Se deja anotado y se sigue; la app se
	# compila e instala igual, y su comportamiento se prueba a mano en el aparato.
	if [ "$app" = "lienzo" ]; then
		echo "== lienzo (frontera: sin layout que modelar)"
		{
			echo "### lienzo"
			echo
			echo "- frontera medida: con \`<com.ejemplo.lienzo.Lienzo>\` en el layout, el framework aborta al inflar"
			echo "  (\`UnsupportedOperationException: You must supply a layout_height attribute\`) aunque"
			echo "  \`aapt2 dump xmltree\` enseñe el atributo; y \`new Lienzo(this)\` desde otra clase del"
			echo "  proyecto no compila (\`el tipo Context no se resuelve ... android.jar ni jar/\`)"
			echo "- por eso la app sigue montando la vista desde código, con sus mandos dibujados en el Canvas,"
			echo "  y no hay pantalla que comparar en el preview; en el aparato se comprueba a mano"
			echo
		} >>"$INFORME"
		continue
	fi
	echo "== $app"
	(
		cd "$AQUI/$app"
		"$APKC" build --linker go --compiler go --android-jar "$JAR" \
			--sdk-version 34 --codename 14 --keystore "$KS" >"$SALIDA/$app.build.log" 2>&1
	)
	cp "$AQUI/$app/build/app.apk" "$CACHE/$app.apk"
	echo "   build ok: $CACHE/$app.apk ($(wc -c <"$CACHE/$app.apk") bytes)"
	# La copia anterior puede venir firmada con otra clave (otra caché, otro debug.keystore) y
	# `install -r` falla con INSTALL_FAILED_UPDATE_INCOMPATIBLE: se desinstala para partir de cero
	# y, de paso, la sonda no arrastra preferencias de una pasada anterior.
	adb -s "$SERIE" shell input keyevent KEYCODE_BACK >/dev/null 2>&1 || true
	sleep 1
	quitada=$(adb -s "$SERIE" uninstall "$paquete" 2>&1 | tr -d '\r' || true)
	case $quitada in
	*Success*) echo "   copia anterior desinstalada (estado limpio)" ;;
	esac
	instalada=$(adb -s "$SERIE" install -r --no-incremental "$CACHE/$app.apk" 2>&1 | tr -d '\r')
	echo "   instalación: $instalada"
	case $instalada in
	*Success*)
		case $app in
		chatbt)
			adb -s "$SERIE" shell pm grant "$paquete" android.permission.BLUETOOTH_CONNECT >/dev/null 2>&1 || true
			adb -s "$SERIE" shell pm grant "$paquete" android.permission.BLUETOOTH_SCAN >/dev/null 2>&1 || true
			;;
		esac
		adb -s "$SERIE" shell am force-stop "$paquete"
		adb -s "$SERIE" shell am start -n "$paquete/.MainActivity" >/dev/null
		sleep 2
		if ! python3 "$AQUI/sonda.py" volcar "$SERIE" "$paquete" "$SALIDA/$app.arranque.xml" >/dev/null; then
			echo "   la ventana no apareció; se relanza antes de la secuencia"
			adb -s "$SERIE" shell am force-stop "$paquete"
			adb -s "$SERIE" shell am start -n "$paquete/.MainActivity" >/dev/null
			sleep 2
		fi
		;;
	*)
		echo "- AVISO: $app no se pudo instalar: \`$instalada\`; la sonda sigue con las demás" >>"$INFORME"
		continue
		;;
	esac

	if ! "$APKC" preview "$AQUI/$app/build/app.apk" >"$SALIDA/$app.base.json" 2>"$SALIDA/$app.preview.err"; then
		echo "   sin modelo de preview: $(tail -1 "$SALIDA/$app.preview.err")"
		echo "- AVISO: \`apkc preview\` no pudo modelar $app: \`$(tail -1 "$SALIDA/$app.preview.err")\`; solo hay lado del aparato" >>"$INFORME"
		echo "" >>"$INFORME"
		# sin modelo no hay ids que pulsar en el preview: se vuelca el aparato y se sigue
		aplicar_en_aparato "$paquete" "$secuencia"
		python3 "$AQUI/sonda.py" volcar "$SERIE" "$paquete" "$SALIDA/$app.aparato.xml" >"$SALIDA/$app.aparato.txt" ||
			: >"$SALIDA/$app.aparato.txt"
		echo "   aparato: $(wc -l <"$SALIDA/$app.aparato.txt") vistas con id"
		continue
	fi
	secuencia=$(secuencia_de "$app")
	argumentos=$(argumentos_preview "$SALIDA/$app.base.json" "$secuencia")
	# shellcheck disable=SC2086
	"$APKC" preview $argumentos "$AQUI/$app/build/app.apk" >"$SALIDA/$app.preview.json" 2>/dev/null
	echo "   preview con: $secuencia"

	aplicar_en_aparato "$paquete" "$secuencia"
	sleep 1
	if ! python3 "$AQUI/sonda.py" volcar "$SERIE" "$paquete" "$SALIDA/$app.aparato.xml" >"$SALIDA/$app.aparato.txt"; then
		echo "   AVISO: el aparato no enseñó $paquete"
		echo "- AVISO: el aparato no enseñó \`$paquete\`; la comparación se queda sin su lado" >>"$INFORME"
		: >"$SALIDA/$app.aparato.txt"
	fi
	echo "   aparato: $(wc -l <"$SALIDA/$app.aparato.txt") vistas con id"

	# notas y tareas declaran persistencia: se comprueba con el proceso muerto y la actividad nueva
	reinicio=""
	case $app in
	notas | tareas)
		adb -s "$SERIE" shell am force-stop "$paquete"
		sleep 1
		adb -s "$SERIE" shell am start -n "$paquete/.MainActivity" >/dev/null
		sleep 2
		if ! python3 "$AQUI/sonda.py" volcar "$SERIE" "$paquete" "$SALIDA/$app.reinicio.xml" >"$SALIDA/$app.reinicio.txt"; then
			echo "- AVISO: $app no volvió a arrancar tras force-stop" >>"$INFORME"
			: >"$SALIDA/$app.reinicio.txt"
		fi
		reinicio=$SALIDA/$app.reinicio.txt
		echo "   tras force-stop: $(wc -l <"$SALIDA/$app.reinicio.txt") vistas con id"
		;;
	esac

	nota=$(nota_de "$app")
	python3 "$AQUI/sonda.py" comparar "$app" "$nota" \
		"$SALIDA/$app.preview.json" "$SALIDA/$app.aparato.txt" ${reinicio:+"$reinicio"} >>"$INFORME"
done

# Deriva del cronómetro: 60 s de reloj real y se lee el tiempo que enseñó la app al pausar.
if [ "$DERIVA" != "0" ]; then
	echo "== deriva del cronómetro ($DERIVA s de reloj real)"
	paquete=com.ejemplo.cronometro
	adb -s "$SERIE" shell am force-stop "$paquete"
	adb -s "$SERIE" shell am start -n "$paquete/.MainActivity" >/dev/null
	sleep 2
	python3 "$AQUI/sonda.py" tocar "$SERIE" "$paquete" iniciar
	sleep "$DERIVA"
	python3 "$AQUI/sonda.py" tocar "$SERIE" "$paquete" pausar
	python3 "$AQUI/sonda.py" tocar "$SERIE" "$paquete" vuelta
	python3 "$AQUI/sonda.py" volcar "$SERIE" "$paquete" "$SALIDA/deriva.xml" >"$SALIDA/deriva.txt"
	python3 - "$SALIDA/deriva.txt" "$DERIVA" >>"$INFORME" <<'PY'
import sys
esperado = float(sys.argv[2])
vistas = {}
for linea in open(sys.argv[1], encoding="utf-8"):
    partes = linea.rstrip("\n").split("\t")
    if len(partes) >= 3:
        vistas[partes[0]] = partes[2]
tiempo = vistas.get("tiempo", "")
def segundos(texto):
    try:
        minutos, resto = texto.split(":")
        return int(minutos) * 60 + float(resto)
    except Exception:
        return None
medido = segundos(tiempo)
print("### deriva del cronómetro")
print()
print("- esperado: " + ("%.2f" % esperado) + " s de reloj de pared")
if medido is None:
    print("- AVISO: no se pudo leer el tiempo del aparato (`" + tiempo + "`)")
else:
    desvio = medido - esperado
    print("- medido: " + ("%.2f" % medido) + " s (`" + tiempo + "`)")
    print("- desvío: " + ("%+.2f" % desvio) + " s")
    if abs(desvio) > 0.5:
        print("- AVISO: el desvío pasa de medio segundo en " + str(int(esperado)) + " s")
    else:
        print("- ok: el desvío es menor que medio segundo en el minuto (el tick de 50 ms y el reloj monotónico no derivan)")
PY
fi

echo
echo "informe: $INFORME"
