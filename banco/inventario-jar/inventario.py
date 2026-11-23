#!/usr/bin/env python3
"""Inventario de los límites del dexer de este fork sobre un jar real de dependencias.

Pasa cada `.class` del jar por `apkc dex` en su propia invocación, captura el rechazo literal y lo
agrupa por tipo de límite. No necesita que el dex complete: cada clase falla o pasa por separado, y
un fallo no oculta las demás. El dexer no se modifica desde aquí.

Uso:
    python3 banco/inventario-jar/inventario.py                  # mide el jar de android-support-v4
    python3 banco/inventario-jar/inventario.py --falsificar     # comprueba el contador con fixtures
    python3 banco/inventario-jar/inventario.py --sin-oraculo    # sin d8/dexdump

El informe se escribe en `banco/inventario-jar/informe.md` (o en --salida). Los temporales van a
`$HOME/.cache/apkc-web/inventario-jar/`, nunca a /tmp: la cuota de esa tmpfs ya rompió builds.
"""

import argparse
import atexit
import contextlib
import fcntl
import hashlib
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile
from collections import OrderedDict
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[2]
APKC_DIR = RAIZ / "apkc"
CACHE = Path(os.environ.get("XDG_CACHE_HOME", Path.home() / ".cache")) / "apkc-web" / "inventario-jar"
JAR_POR_DEFECTO = Path("/home/leonisauro/Downloads/ApkShare-main/jar/android-support-v4.jar")
ANDROID_JAR_POR_DEFECTO = Path("/home/leonisauro/opt/android-sdk/platforms/android-34/android.jar")

# El rechazo del ensamblado llega como `Lclase;metodo: at pc N, opcode: motivo`; el del recorrido
# del bytecode, como `Lclase;metodo: at pc N, byte 0xNN: motivo`. Los errores que no llegan a un
# método (una clase que no se lee, una lambda sin medir) solo traen la clase.
RE_CON_PC = re.compile(r"(L[^;]+;)([^:\s]+): at pc (\d+), (.*)$")
RE_SIN_PC = re.compile(r"(L[^;]+;)([^:\s]+): (.*)$")


def huella_de_apkc(directorio=None):
    """Huella del código que `go build` lee de apkc/: los `.go` sin pruebas, go.mod/go.sum y los
    ficheros embebidos de `template/`.

    Es una huella de CONTENIDO, no de mtime: cambiar de rama o tocar un fichero sin cambiar sus
    bytes no obliga a recompilar, y un cambio real sí. Se guarda al lado del binario; si no casa, el
    binario cacheado es de otro código y no sirve para medir este HEAD.
    """
    directorio = Path(directorio) if directorio else APKC_DIR
    rutas = []
    for ruta in directorio.rglob("*.go"):
        if ruta.name.endswith("_test.go"):
            continue
        rutas.append(ruta)
    for nombre in ("go.mod", "go.sum"):
        ruta = directorio / nombre
        if ruta.exists():
            rutas.append(ruta)
    plantilla = directorio / "template"
    if plantilla.is_dir():
        rutas.extend(p for p in plantilla.rglob("*") if p.is_file())
    h = hashlib.sha256()
    for ruta in sorted(rutas, key=lambda p: p.relative_to(directorio).as_posix()):
        h.update(ruta.relative_to(directorio).as_posix().encode())
        h.update(b"\0")
        h.update(ruta.read_bytes())
        h.update(b"\0")
    return h.hexdigest()


def construir_apkc(ruta, directorio=APKC_DIR, destino=None):
    """Devuelve el binario de apkc: el que se le pase, el del entorno o uno construido en caché.

    Un `--apkc` o un `APKC=...` externos se usan tal cual, sin reconstruir. El binario de caché se
    llama `apkc-<huella del código>`: dos procesos con el mismo `apkc/` comparten el fichero y dos
    con códigos distintos no se lo pisan, que era la carrera que mezclaba bytes y dejaba una huella
    que no correspondía al binario. La construcción se serializa con un `flock` por huella y el
    resultado entra con un `rename` atómico.
    `directorio` y `destino` existen para la falsificación: así compila una copia en un temporal y
    nunca escribe en el árbol versionado.
    """
    if ruta:
        return str(Path(ruta))
    del_entorno = os.environ.get("APKC")
    if del_entorno:
        return del_entorno
    directorio = Path(directorio)
    CACHE.mkdir(parents=True, exist_ok=True)
    actual = huella_de_apkc(directorio)
    destino = Path(destino) if destino else CACHE / f"apkc-{actual}"
    marca = Path(str(destino) + ".huella")

    def al_dia():
        return (destino.exists() and marca.exists()
                and marca.read_text(encoding="utf-8").strip() == actual)

    if al_dia():
        return str(destino)
    # el cerrojo es por destino: dos fuentes distintas no se esperan entre sí
    with open(str(destino) + ".lock", "a+", encoding="utf-8") as cerrojo:
        fcntl.flock(cerrojo, fcntl.LOCK_EX)
        try:
            # otra corrida pudo construirlo mientras este proceso esperaba
            if al_dia():
                return str(destino)
            print(f"[inventario] construyendo apkc en {destino} (huella {actual[:12]})",
                  file=sys.stderr)
            temporal = Path(str(destino) + f".{os.getpid()}.tmp")
            subprocess.run(["go", "build", "-o", str(temporal), "."], cwd=directorio, check=True)
            os.replace(temporal, destino)
            marca.write_text(actual + "\n", encoding="utf-8")
            return str(destino)
        finally:
            fcntl.flock(cerrojo, fcntl.LOCK_UN)


def extraer_clases(jar, destino):
    """Extrae los `.class` del jar, en orden de nombre, y devuelve la lista de rutas."""
    destino.mkdir(parents=True, exist_ok=True)
    rutas = []
    with zipfile.ZipFile(jar) as z:
        for nombre in sorted(z.namelist()):
            if not nombre.endswith(".class") or nombre.startswith("META-INF/"):
                continue
            if nombre.endswith("/module-info.class") or nombre.endswith("/package-info.class"):
                continue
            if nombre in ("module-info.class", "package-info.class"):
                continue
            ruta = destino / nombre
            ruta.parent.mkdir(parents=True, exist_ok=True)
            ruta.write_bytes(z.read(nombre))
            rutas.append(ruta)
    return rutas


def dexa(apkc, objetivo, salida):
    """Dexa un objetivo (clase o carpeta) y devuelve (código, texto del error)."""
    if salida.exists():
        salida.unlink()
    r = subprocess.run([apkc, "dex", "-o", str(salida), str(objetivo)],
                       capture_output=True, text=True)
    if r.returncode == 0:
        return 0, ""
    texto = (r.stdout + "\n" + r.stderr).strip()
    # las líneas de log llevan color cuando hablan por un terminal; el informe se lee sin ANSI
    texto = re.sub(r"\x1b\[[0-9;]*m", "", texto)
    return r.returncode, texto


def clasifica(mensaje):
    """Convierte el motivo literal en un tipo de límite estable, sin números de registro."""
    if "opcode outside the table" in mensaje or "outside the table this fork measured" in mensaje:
        return "bytecode JVM fuera de la tabla"
    if "a goto of" in mensaje or "goto/16 is not measured" in mensaje:
        return "salto goto fuera de alcance"
    if "was never interned" in mensaje:
        return "tipo no internado por el colector"
    if "no low register is free" in mensaje:
        return "registro bajo ocupado"
    if ("does not fit a four-bit format" in mensaje
            or "does not fit the eight-bit" in mensaje
            or "register plan does not fit" in mensaje):
        return "registro fuera de formato"
    if "field access of type" in mensaje:
        return "acceso a campo de tipo no medido"
    if "needs more temporaries" in mensaje:
        return "presupuesto de temporales"
    if "runs past the end" in mensaje:
        return "bytecode truncado"
    if "no se lee" in mensaje or "cannot read" in mensaje or "not a class file" in mensaje:
        return "clase ilegible"
    if "lambda" in mensaje or "concatenación dinámica" in mensaje or "invokedynamic" in mensaje:
        return "lambda o concatenación dinámica"
    return "otro rechazo del dexer"


def parte_error(texto):
    """Saca clase, método, pc, opcode y motivo de la salida de `apkc dex`.

    Devuelve None si el error no nombra una clase, que entonces se cuenta como fallo del comando
    sin método (por ejemplo, un jar que no abre).
    """
    for linea in reversed([l.strip() for l in texto.splitlines() if l.strip()]):
        # la línea útil es la que empieza por el descriptor del jar de Android, `L…;`
        m = RE_CON_PC.search(linea)
        if m:
            clase, metodo, pc, resto = m.groups()
            if ": " in resto:
                opcode, motivo = resto.split(": ", 1)
            else:
                opcode, motivo = resto, resto
            return {"clase": clase, "metodo": metodo, "pc": int(pc),
                    "opcode": opcode.strip(), "motivo": motivo.strip(), "linea": linea}
        m = RE_SIN_PC.search(linea)
        if m:
            clase, metodo, motivo = m.groups()
            return {"clase": clase, "metodo": metodo, "pc": None,
                    "opcode": "", "motivo": motivo.strip(), "linea": linea}
    return {"clase": "", "metodo": "", "pc": None, "opcode": "", "motivo": texto.strip(),
            "linea": texto.strip().splitlines()[-1] if texto.strip() else ""}


def mide(apkc, rutas, salida, limite=None):
    """Dexa cada clase por separado y agrupa los rechazos. Devuelve el inventario."""
    inv = {
        "clases": min(len(rutas), limite) if limite is not None else len(rutas),
        "fallos": 0,
        "limites": OrderedDict(),
        "opcodes": OrderedDict(),
        "primer_fallo_clase": None,
    }
    for i, ruta in enumerate(rutas):
        if limite is not None and i >= limite:
            break
        _rc, texto = dexa(apkc, ruta, salida)
        if not texto:
            continue
        inv["fallos"] += 1
        caso = parte_error(texto)
        caso["archivo"] = ruta.name
        tipo = clasifica(caso["motivo"])
        if tipo == "bytecode JVM fuera de la tabla":
            m = re.search(r"byte (0x[0-9a-fA-F]{2})", caso["opcode"] + " " + caso["motivo"])
            byte = m.group(1).lower() if m else caso["opcode"]
            caso["byte"] = byte
            inv["opcodes"][byte] = inv["opcodes"].get(byte, 0) + 1
        grupo = inv["limites"].setdefault(tipo, {"casos": [], "clases": set()})
        grupo["casos"].append(caso)
        if caso["clase"]:
            grupo["clases"].add(caso["clase"])
        if inv["primer_fallo_clase"] is None:
            inv["primer_fallo_clase"] = caso
    return inv


def muestra(caso):
    clase = caso.get("clase") or caso.get("archivo", "")
    metodo = caso.get("metodo", "")
    s = clase + (("." + metodo) if metodo else "")
    if caso.get("pc") is not None:
        s += f" pc {caso['pc']}"
    if caso.get("opcode"):
        s += f" · {caso['opcode']}"
    return s


def tabla_limites(inv):
    lineas = ["| Límite | Clases | Métodos | Primer caso |", "| --- | ---: | ---: | --- |"]
    orden = sorted(inv["limites"].items(), key=lambda kv: (-len(kv[1]["casos"]), kv[0]))
    for tipo, grupo in orden:
        lineas.append(f"| {tipo} | {len(grupo['clases'])} | {len(grupo['casos'])} | "
                      f"{muestra(grupo['casos'][0])} |")
    return "\n".join(lineas)


def tabla_opcodes(inv):
    if not inv["opcodes"]:
        return ""
    lineas = ["| Byte JVM | Métodos | Ejemplo |", "| --- | ---: | --- |"]
    for byte in sorted(inv["opcodes"]):
        ejemplo = ""
        for grupo in inv["limites"].values():
            for caso in grupo["casos"]:
                if caso.get("byte") == byte:
                    ejemplo = muestra(caso)
                    break
            if ejemplo:
                break
        lineas.append(f"| {byte} | {inv['opcodes'][byte]} | {ejemplo} |")
    return "\n".join(lineas)


def buscar_d8():
    """Busca d8 y dexdump: los del SDK configurado o los primeros que aparezcan en build-tools.

    d8 y dexdump no siempre viajan juntos: las build-tools 36 del escritorio traen d8 y no
    dexdump, y el dexdump de 35 sirve igual para leer el dex que escribió 36.
    """
    if os.environ.get("APKC_D8"):
        d8 = Path(os.environ["APKC_D8"])
        dexdump = d8.with_name("dexdump")
        if not dexdump.exists():
            dexdump = buscar_dexdump()
        return d8, dexdump
    sdk = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT") or "/home/leonisauro/opt/android-sdk"
    base = Path(sdk) / "build-tools"
    if base.is_dir():
        for version in sorted(base.iterdir(), reverse=True):
            d8 = version / "d8"
            if d8.exists():
                dexdump = version / "dexdump"
                return d8, (dexdump if dexdump.exists() else buscar_dexdump())
    d8 = shutil.which("d8")
    if d8:
        dexdump = shutil.which("dexdump")
        return Path(d8), (Path(dexdump) if dexdump else buscar_dexdump())
    return None, None


def buscar_dexdump():
    """Busca un dexdump en cualquier build-tools del SDK, aunque su d8 no sea el elegido."""
    sdk = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT") or "/home/leonisauro/opt/android-sdk"
    base = Path(sdk) / "build-tools"
    if base.is_dir():
        for version in sorted(base.iterdir(), reverse=True):
            candidato = version / "dexdump"
            if candidato.exists():
                return candidato
    encontrado = shutil.which("dexdump")
    return Path(encontrado) if encontrado else None


def oraculo_d8(jar, trabajo, android_jar, muestras, publicado):
    """Dexa el jar entero con d8 y enseña las instrucciones DEX de los primeros casos.

    `muestras` es una lista de (tipo, clase, método): para cada una se recorta del `dexdump -d`
    del dex de d8 el trozo de ese método, que es el equivalente que propone el dexer de referencia.
    La corrida escribe en `trabajo` (privado del proceso) y el comando se publica con `publicado`
    (la forma estable bajo `$CACHE`).
    """
    d8, dexdump = buscar_d8()
    if d8 is None or not Path(d8).exists():
        return "d8 no está disponible: no se pudo comprobar el jar contra el dexer de referencia.\n"
    salida = trabajo / "d8"
    if salida.exists():
        shutil.rmtree(salida)
    salida.mkdir(parents=True)
    comando = [str(d8), "--min-api", "21", "--lib", str(android_jar), "--output", str(salida), str(jar)]
    visible = list(comando)
    visible[visible.index(str(salida))] = ruta_publicada(publicado / "d8")
    r = subprocess.run(comando, capture_output=True, text=True)
    lineas = ["Comando del oráculo:", "", "```sh", " ".join(visible), "```", ""]
    lineas.append("La corrida usa un directorio privado por proceso; `$CACHE` es el directorio de "
                  "caché del inventario (`$XDG_CACHE_HOME/apkc-web/inventario-jar` o "
                  "`$HOME/.cache/apkc-web/inventario-jar`).")
    lineas.append("")
    if r.returncode != 0:
        lineas.append(f"d8 falló con rc {r.returncode}:")
        lineas.append("```\n" + (r.stdout + r.stderr).strip()[-2000:] + "\n```")
        return "\n".join(lineas) + "\n"
    dex = salida / "classes.dex"
    tam = dex.stat().st_size if dex.exists() else 0
    lineas.append(f"d8 dexa el jar completo con rc 0: `classes.dex` de {tam} bytes.")
    lineas.append("")
    if dexdump is None or not Path(dexdump).exists() or not dex.exists():
        return "\n".join(lineas) + "\n"

    # una sola pasada por el dex grande: se recorta el método de cada muestra en cuanto aparece
    objetivos = {}
    for tipo, clase, metodo in muestras:
        objetivos.setdefault((clase, metodo), tipo)
    recortes = {}
    claves_dex = {}
    clase_actual = ""
    metodo_actual = ""
    capturando = False
    lineas_metodo = 0
    p = subprocess.Popen([str(dexdump), "-d", str(dex)], stdout=subprocess.PIPE,
                         stderr=subprocess.DEVNULL, text=True)
    for linea in p.stdout:
        if linea.strip().startswith("Class descriptor"):
            m = re.search(r"'([^']+)'", linea)
            clase_actual = m.group(1) if m else ""
        m = re.match(r"\s+name\s+: '([^']+)'", linea)
        if m:
            metodo_actual = m.group(1)
            capturando = (clase_actual, metodo_actual) in objetivos
            lineas_metodo = 0
        if not capturando:
            continue
        clave = objetivos.get((clase_actual, metodo_actual))
        if clave is None:
            continue
        # el bloque enseña el principio del método; las palabras clave se buscan en todo el cuerpo
        if lineas_metodo < 25:
            recortes.setdefault(clave, []).append(linea.rstrip())
        lineas_metodo += 1
        for palabra in PALABRAS_DEX.get(clave, []):
            if palabra in linea:
                lista = claves_dex.setdefault(clave, [])
                if len(lista) < 6 and linea.rstrip() not in lista:
                    lista.append(linea.rstrip())
    p.terminate()

    for tipo, clase, metodo in muestras:
        trozo = recortes.get(tipo)
        if not trozo:
            continue
        lineas.append(f"Equivalente DEX de d8 para **{tipo}** — `{clase}.{metodo}`:")
        lineas.append("")
        lineas.append("```")
        lineas.extend(trozo)
        lineas.append("```")
        lineas.append("")
        claves = claves_dex.get(tipo)
        if claves:
            lineas.append("Líneas que d8 escribe para ese límite:")
            lineas.append("")
            lineas.append("```")
            lineas.extend(claves)
            lineas.append("```")
            lineas.append("")
    return "\n".join(lineas) + "\n"


# Las palabras del `dexdump` que enseñan la forma DEX concreta que falta en cada límite.
PALABRAS_DEX = {
    "registro fuera de formato": ["from16"],
    "salto goto fuera de alcance": ["goto/16", "goto/32"],
    "tipo no internado por el colector": ["const-class"],
    "presupuesto de temporales": ["/range"],
}


# Propuesta de equivalente DEX por tipo de límite: qué forma hay que escribir para que el caso que
# hoy se rechaza exista en un dex. d8 lo escribe en el dex del oráculo que acompaña a la tabla.
EQUIVALENTES_DEX = {
    "registro fuera de formato":
        "las formas `/from16` (move-object/from16 0x08, move/from16 0x02, …) y 22x, que ya están "
        "medidas para otras familias; falta aplicarlas a `aload_*`, `iload_*`, `astore_*` y sus "
        "primas.",
    "salto goto fuera de alcance":
        "`goto/16` (0x29, 21t) y `goto/32` (0x2a, 30t) para un salto fuera de ±127 unidades; d8 "
        "los usa en el mismo método del oráculo.",
    "tipo no internado por el colector":
        "`const-class` (0x1c, 21c) con el type_id internado también desde `ldc_w` de una clase; el "
        "colector debe seguir ese operando, no solo `checkcast`/`instanceof`/`new`.",
    "registro bajo ocupado":
        "un plan de registros con desalojo de un local a la pila de temporales; d8 resuelve el "
        "mismo método moviendo el valor a otro registro libre antes del uso.",
    "presupuesto de temporales":
        "subir la base del reparto por encima de `max_stack` de javap cuando el bloque `/range` "
        "necesita más temporales; el bucle de `codeItemConPc` ya sube la base cuando el pico lo "
        "pide, y aquí el límite es el paso de medida.",
    "bytecode JVM fuera de la tabla":
        "añadir la entrada del byte JVM a la tabla y su traducción a la opcode dalvik que d8 "
        "escribe; el byte concreto está en la tabla de opcodes de arriba.",
}


# --- falsificación del contador -------------------------------------------------------------

def u1(b, i):
    return b[i]


def u2(b, i):
    return int.from_bytes(b[i:i + 2], "big")


def u4(b, i):
    return int.from_bytes(b[i:i + 4], "big")


def code_offsets(clase):
    """Devuelve los desplazamientos del primer byte de cada atributo Code de la clase.

    Es un lector mínimo de JVMS 4.1: basta para saltar la constante, los campos y los métodos y
    localizar el cuerpo de los Code, que es donde vive el opcode que se quiere cambiar.
    """
    if u4(clase, 0) != 0xCAFEBABE:
        raise ValueError("no es un .class")
    i = 8
    cp_count = u2(clase, i)
    i += 2
    utf8 = {}
    j = 1
    while j < cp_count:
        etiqueta = u1(clase, i)
        i += 1
        if etiqueta == 1:
            n = u2(clase, i)
            i += 2
            utf8[j] = clase[i:i + n].decode("utf-8", "replace")
            i += n
        elif etiqueta in (7, 8, 16, 19, 20):
            i += 2
        elif etiqueta == 15:
            i += 3
        elif etiqueta in (3, 4, 9, 10, 11, 12, 17, 18):
            i += 4
        elif etiqueta in (5, 6):
            i += 8
            j += 1
        else:
            raise ValueError(f"etiqueta de constante {etiqueta} en {j}")
        j += 1
    i += 6  # access, this, super
    n = u2(clase, i)
    i += 2 + 2 * n
    for _ in range(2):  # campos y métodos comparten la forma
        n = u2(clase, i)
        i += 2
        offsets = []
        for _ in range(n):
            i += 6  # access, name, descriptor
            attrs = u2(clase, i)
            i += 2
            for _ in range(attrs):
                nombre = utf8.get(u2(clase, i), "")
                largo = u4(clase, i + 2)
                cuerpo = i + 6
                if nombre == "Code":
                    offsets.append(cuerpo + 8)  # max_stack, max_locals, code_length y el cuerpo
                i = cuerpo + largo
        if offsets:
            return offsets
    return []


def romper_primera_code(datos):
    """Copia la clase y pone 0xca (`breakpoint`, reservado) en el primer byte del primer Code."""
    copia = bytearray(datos)
    offsets = code_offsets(copia)
    if not offsets:
        raise ValueError("el fixture no tiene ningún atributo Code")
    copia[offsets[0]] = 0xCA
    return bytes(copia)


def decide_construir(ruta="", directorio=None, destino=None):
    """Llama a `construir_apkc` y dice si decidió reconstruir, capturando su aviso de stderr."""
    buf = io.StringIO()
    with contextlib.redirect_stderr(buf):
        if directorio is not None:
            binario = construir_apkc(ruta, directorio, destino)
        else:
            binario = construir_apkc(ruta)
    return binario, "construyendo apkc" in buf.getvalue()


def copia_de_apkc(destino):
    """Copia el código que `go build` lee de `apkc/`, sin `.go` de pruebas ni artefactos."""

    def ignorar(directorio, nombres):
        fuera = set()
        for nombre in nombres:
            ruta = Path(directorio) / nombre
            if ruta.is_dir() and nombre in (".git", "build", "classes", "bin"):
                fuera.add(nombre)
            elif ruta.is_file() and (nombre.endswith("_test.go") or nombre == "apkc"):
                fuera.add(nombre)
        return fuera

    if destino.exists():
        shutil.rmtree(destino)
    shutil.copytree(APKC_DIR, destino, ignore=ignorar)
    return destino


def apkc_versionado_limpio():
    """Dice si `git status --porcelain apkc/` está vacío y devuelve su salida literal."""
    r = subprocess.run(["git", "status", "--porcelain", "apkc/"], cwd=RAIZ,
                       capture_output=True, text=True)
    return r.returncode == 0 and not r.stdout.strip(), r.stdout.strip()


def comprobar_reconstruccion(trabajo):
    """Cambia un `.go` de una COPIA de apkc/ y exige reconstrucción; no toca el árbol versionado.

    La sonda compila una copia en el directorio de trabajo con su propio binario y su propia
    huella. En cada paso se exige que `git status --porcelain apkc/` siga vacío: si alguien vuelve
    a mutar el árbol real, la falsificación lo dice en vez de depender de un `finally`.
    """
    fallos = []
    lineas = ["", "## Reconstrucción por huella", ""]
    copia = copia_de_apkc(trabajo / "apkc-copia")
    archivo = copia / "create.go"
    original = archivo.read_bytes()
    binario_copia = trabajo / "apkc-copia-bin"

    def limpio(momento):
        ok, salida = apkc_versionado_limpio()
        if not ok:
            fallos.append(f"`git status --porcelain apkc/` no está vacío {momento}: {salida}")
        return ok

    limpio("antes de la copia")

    # un binario externo, por --apkc o por APKC, se usa tal cual y no se reconstruye
    externo = str(trabajo / "apkc-externo")
    igual, reconstruyo = decide_construir(externo)
    if igual != externo or reconstruyo:
        fallos.append("--apkc no devolvió el binario externo tal cual")
    else:
        lineas.append("- `--apkc` externo: se usa sin reconstruir.")
    anterior = os.environ.get("APKC")
    os.environ["APKC"] = externo
    try:
        via_entorno, reconstruyo = decide_construir()
    finally:
        if anterior is None:
            os.environ.pop("APKC", None)
        else:
            os.environ["APKC"] = anterior
    if via_entorno != externo or reconstruyo:
        fallos.append("APKC=... no devolvió el binario externo tal cual")
    else:
        lineas.append("- `APKC=...` externo: se usa sin reconstruir.")

    modificado = original + b"\n// huella: prueba de reconstruccion del inventario\n"

    def construir_copia():
        return decide_construir(directorio=copia, destino=binario_copia)

    # primera compilación de la copia: deja su caché al día antes de medir la decisión
    construir_copia()
    limpio("tras la primera compilación de la copia")
    try:
        _, sin_cambio = construir_copia()
        if sin_cambio:
            fallos.append("reconstruyó sin que cambiara ningún .go")
        else:
            lineas.append("- Sin cambios: no reconstruye.")
        limpio("sin cambios")

        # un cambio real de contenido en la copia: un comentario que no altera el código compilado
        archivo.write_bytes(modificado)
        limpio("con la copia cambiada")
        _, con_cambio = construir_copia()
        if not con_cambio:
            fallos.append("no reconstruyó tras cambiar el contenido de create.go")
        else:
            lineas.append("- Con un `.go` cambiado: reconstruye.")
        limpio("tras cambiar la copia")

        archivo.write_bytes(original)
        _, al_restaurar = construir_copia()
        if not al_restaurar:
            fallos.append("no reconstruyó al restaurar el .go original")
        else:
            lineas.append("- Al restaurar el `.go` original: reconstruye una vez.")
        _, tras_restaurar = construir_copia()
        if tras_restaurar:
            fallos.append("reconstruyó otra vez sin cambios tras restaurar")
        else:
            lineas.append("- Después de restaurar, sin cambios: no reconstruye.")
        limpio("al terminar")
    finally:
        # la copia es desechable, pero se deja como estaba por si otra sonda la reutiliza
        if archivo.read_bytes() == modificado:
            archivo.write_bytes(original)
    if not fallos:
        lineas.append("- `git status --porcelain apkc/` vacío durante toda la comprobación: la "
                      "sonda no escribe en el árbol versionado.")
    lineas.append("")
    lineas.append("Veredicto de la reconstrucción: " + ("PASA" if not fallos else "FALLA"))
    for f in fallos:
        lineas.append(f"- {f}")
    return lineas, 0 if not fallos else 1


def comprobar_reproducibilidad(apkc, jar):
    """Cuarto caso: dos ejecuciones seguidas del mismo jar dejan los mismos bytes.

    Se comparan el markdown y el JSON, no los ficheros de disco, para que la prueba no dependa del
    estado del repo; el `git status` limpio se comprueba al regenerar la evidencia de verdad. Lo que
    se persigue es que no quede ninguna ruta aleatoria en el informe.
    """
    fallos = []
    lineas = ["", "## Informe reproducible", ""]
    if not jar.exists():
        return lineas + ["- Falta el jar del fixture; no se pudo medir."], 1

    first = None
    for vuelta in (1, 2):
        trabajo = directorio_de_trabajo(jar)
        try:
            informe, detalle = genera_informe(apkc, jar, trabajo, ANDROID_JAR_POR_DEFECTO, False)
        finally:
            shutil.rmtree(trabajo, ignore_errors=True)
        actual = (informe, detalle)
        if first is None:
            first = actual
        elif actual != first:
            fallos.append(f"la vuelta {vuelta} no dio los mismos bytes que la primera")

    informe, detalle = first
    sha_informe = hashlib.sha256(informe.encode()).hexdigest()
    sha_detalle = hashlib.sha256(detalle.encode()).hexdigest()
    lineas.append(f"- Dos ejecuciones seguidas del mismo jar: `informe.md` idéntico "
                  f"(sha256 `{sha_informe[:16]}…`).")
    lineas.append(f"- `casos.json` idéntico (sha256 `{sha_detalle[:16]}…`).")

    esperado = f"trabajo-{huella_del_jar(jar)}"
    vistos = set(re.findall(r"trabajo-[A-Za-z0-9_]+", informe))
    # el jar del fixture vive dentro del directorio de trabajo del jar real: los dos nombres son
    # huellas de 12 hex. Lo que no puede quedar es un sufijo aleatorio de `mkdtemp`.
    aleatorios = [visto for visto in vistos if not re.fullmatch(r"trabajo-[0-9a-f]{12}", visto)]
    if aleatorios:
        fallos.append(f"el informe nombra directorios de trabajo no derivados de una huella: "
                      f"{sorted(aleatorios)}")
    elif f"{esperado}/d8" not in informe:
        fallos.append(f"el comando del oráculo no usa {esperado}/d8")
    else:
        lineas.append(f"- El oráculo usa `{esperado}/d8`, derivado del jar, no un directorio "
                      "aleatorio.")
    lineas.append("")
    lineas.append("Veredicto de la reproducibilidad: " + ("PASA" if not fallos else "FALLA"))
    for f in fallos:
        lineas.append(f"- {f}")
    return lineas, 0 if not fallos else 1


def comprobar_arbol_limpio():
    """Quinto caso: al terminar, la sonda no ha dejado cambios en el árbol.

    Mira todo `git status --porcelain` y separa lo que la sonda puede tocar (`banco/inventario-jar/`
    y `apkc/`) de cambios ajenos. Los ajenos se enseñan pero no tumban la prueba: el repo puede
    estar en medio de otro trabajo. Los propios sí: era el defecto que reescribía `falsificacion.md`.
    """
    r = subprocess.run(["git", "status", "--porcelain"], cwd=RAIZ, capture_output=True, text=True)
    lineas = ["", "## No escribe en el árbol", ""]
    if r.returncode != 0:
        lineas.append(f"- `git status --porcelain` falló: {r.stderr.strip()}")
        lineas.append("Veredicto: FALLA")
        return lineas, 1
    todas = [l for l in r.stdout.splitlines() if l.strip()]
    propias = [l for l in todas if l[3:].startswith(("banco/inventario-jar/", "apkc/"))]
    ajenas = [l for l in todas if l not in propias]
    if propias:
        lineas.append("- `git status --porcelain` trae cambios de la sonda:")
        lineas.append("```")
        lineas.extend(propias)
        lineas.append("```")
        lineas.append("Veredicto: FALLA")
        return lineas, 1
    lineas.append("- `git status --porcelain banco/inventario-jar/ apkc/`: vacío.")
    if ajenas:
        lineas.append("- Cambios ajenos que la sonda no toca: " + ", ".join(ajenas))
    lineas.append("Veredicto: PASA")
    return lineas, 0


def comprobar_concurrencia(apkc, jar, trabajo):
    """Quinto caso: dos procesos a la vez sobre el mismo jar no comparten ni borran su taller.

    Cada hijo crea su propio `trabajo-<huella>-<aleatorio>`, así que el d8 de uno no puede pisar el
    del otro. Se comparan los informes: si los dos procesos escriben el mismo jar con el mismo
    dexer, tienen que salir los mismos bytes.
    """
    fallos = []
    lineas = ["", "## Dos corridas concurrentes", ""]
    script = RAIZ / "banco" / "inventario-jar" / "inventario.py"
    base = trabajo / "concurrente"
    procesos = []
    for nombre in ("a", "b"):
        destino = base / nombre
        destino.mkdir(parents=True, exist_ok=True)
        cmd = [sys.executable, str(script), "--jar", str(jar), "--apkc", apkc,
               "--salida", str(destino / "informe.md")]
        registro = open(destino / "salida.txt", "w", encoding="utf-8")
        procesos.append((nombre, subprocess.Popen(cmd, cwd=RAIZ, stdout=registro,
                                                  stderr=subprocess.STDOUT), registro))
    codigos = []
    for _, p, registro in procesos:
        codigos.append(p.wait())
        registro.close()

    lineas.append(f"- Dos procesos sobre el mismo jar: rc {codigos[0]} y rc {codigos[1]}.")
    if any(codigo != 0 for codigo in codigos):
        fallos.append(f"alguna corrida concurrente falló: {codigos}")
        for nombre, codigo in zip(("a", "b"), codigos):
            if codigo == 0:
                continue
            salida = (base / nombre / "salida.txt").read_text(encoding="utf-8", errors="replace")
            lineas.append(f"- Proceso {nombre} falló; últimas líneas:")
            lineas.append("```")
            lineas.extend(salida.strip().splitlines()[-8:])
            lineas.append("```")
    else:
        informe_a = (base / "a" / "informe.md").read_bytes()
        informe_b = (base / "b" / "informe.md").read_bytes()
        json_a = (base / "a" / "casos.json").read_bytes()
        json_b = (base / "b" / "casos.json").read_bytes()
        if informe_a != informe_b:
            fallos.append("las dos corridas concurrentes no dejan el mismo informe.md")
        else:
            sha = hashlib.sha256(informe_a).hexdigest()
            lineas.append(f"- `informe.md` idéntico byte a byte (sha256 `{sha[:16]}…`).")
        if json_a != json_b:
            fallos.append("las dos corridas concurrentes no dejan el mismo casos.json")
        else:
            lineas.append(f"- `casos.json` idéntico byte a byte "
                          f"(sha256 `{hashlib.sha256(json_a).hexdigest()[:16]}…`).")
    lineas.append("")
    lineas.append("Veredicto de la concurrencia: " + ("PASA" if not fallos else "FALLA"))
    for f in fallos:
        lineas.append(f"- {f}")
    return lineas, 0 if not fallos else 1


def comprobar_concurrencia_binario(trabajo):
    """Sexto caso: dos procesos con copias distintas de `apkc/` y sin `--apkc` no mezclan binario.

    Cada copia tiene su huella, así que cada una construye `apkc-<huella>` en una caché aislada y
    no hay un destino compartido que se pueda quedar a medias. Se exige que los dos terminen con
    rc 0, que el binario de la copia modificada tenga la huella de esa copia y que los informes
    sigan siendo idénticos.
    """
    fallos = []
    lineas = ["", "## Dos corridas concurrentes del binario", ""]
    jar = trabajo / "fixture-sana.jar"
    if not jar.exists():
        return lineas + ["- Falta el jar del fixture; no se pudo medir."], 1
    limpia = copia_de_apkc(trabajo / "apkc-bin-limpia")
    mod = copia_de_apkc(trabajo / "apkc-bin-mod")
    with open(mod / "create.go", "ab") as f:
        f.write(b"\n// huella: copia modificada para la carrera del binario\n")
    h_limpia = huella_de_apkc(limpia)
    h_mod = huella_de_apkc(mod)
    cache = trabajo / "cache-bin"
    env = os.environ.copy()
    env["XDG_CACHE_HOME"] = str(cache)
    script = RAIZ / "banco" / "inventario-jar" / "inventario.py"
    procesos = []
    for nombre, copia in (("limpia", limpia), ("modificada", mod)):
        destino = trabajo / "bin" / nombre
        destino.mkdir(parents=True, exist_ok=True)
        cmd = [sys.executable, str(script), "--jar", str(jar), "--apkc-dir", str(copia),
               "--salida", str(destino / "informe.md")]
        registro = open(destino / "salida.txt", "w", encoding="utf-8")
        procesos.append((nombre, subprocess.Popen(cmd, cwd=RAIZ, env=env, stdout=registro,
                                                  stderr=subprocess.STDOUT), registro))
    codigos = []
    for _, p, registro in procesos:
        codigos.append(p.wait())
        registro.close()

    lineas.append(f"- Dos procesos con copias distintas y sin `--apkc`: rc {codigos[0]} y "
                  f"rc {codigos[1]}.")
    if any(codigo != 0 for codigo in codigos):
        fallos.append(f"alguna corrida concurrente del binario falló: {codigos}")
        for nombre, codigo in zip(("limpia", "modificada"), codigos):
            if codigo == 0:
                continue
            salida = (trabajo / "bin" / nombre / "salida.txt").read_text(
                encoding="utf-8", errors="replace")
            lineas.append(f"- Proceso {nombre} falló; últimas líneas:")
            lineas.append("```")
            lineas.extend(salida.strip().splitlines()[-8:])
            lineas.append("```")
    else:
        raiz_cache = cache / "apkc-web" / "inventario-jar"
        for nombre, huella in (("limpia", h_limpia), ("modificada", h_mod)):
            binario = raiz_cache / f"apkc-{huella}"
            marca = Path(str(binario) + ".huella")
            if not binario.exists():
                fallos.append(f"no existe el binario de la copia {nombre}: {binario.name}")
            elif not marca.exists() or marca.read_text(encoding="utf-8").strip() != huella:
                fallos.append(f"la huella del binario de la copia {nombre} no es la suya")
            else:
                lineas.append(f"- `apkc-{huella[:12]}` de la copia {nombre} con su huella.")
        informe_a = (trabajo / "bin" / "limpia" / "informe.md").read_bytes()
        informe_b = (trabajo / "bin" / "modificada" / "informe.md").read_bytes()
        json_a = (trabajo / "bin" / "limpia" / "casos.json").read_bytes()
        json_b = (trabajo / "bin" / "modificada" / "casos.json").read_bytes()
        if informe_a != informe_b:
            fallos.append("las dos corridas del binario no dejan el mismo informe.md")
        else:
            lineas.append(f"- `informe.md` idéntico byte a byte "
                          f"(sha256 `{hashlib.sha256(informe_a).hexdigest()[:16]}…`).")
        if json_a != json_b:
            fallos.append("las dos corridas del binario no dejan el mismo casos.json")
        else:
            lineas.append(f"- `casos.json` idéntico byte a byte "
                          f"(sha256 `{hashlib.sha256(json_a).hexdigest()[:16]}…`).")
    lineas.append("")
    lineas.append("Veredicto de la concurrencia del binario: "
                  + ("PASA" if not fallos else "FALLA"))
    for f in fallos:
        lineas.append(f"- {f}")
    return lineas, 0 if not fallos else 1


def comprobar_falsificacion(apkc, trabajo):
    """Comprueba que el contador ve cero sin límites y uno cuando el bytecode no está medido."""
    fallos = []
    javac = shutil.which("javac")
    if javac is None:
        return ["javac no está disponible: no se pudo montar el fixture de un opcode."], 1
    javac = Path(javac)
    fuente = trabajo / "fixture" / "Sano.java"
    fuente.parent.mkdir(parents=True, exist_ok=True)
    fuente.write_text("package fixture;\npublic class Sano {\n"
                      "    public int suma(int a) { return a + 1; }\n}\n")
    clases = trabajo / "fixture" / "classes"
    r = subprocess.run([str(javac), "-d", str(clases), str(fuente)], capture_output=True, text=True)
    if r.returncode != 0:
        return [f"javac falló: {r.stderr.strip()}"], 1
    original = (clases / "fixture" / "Sano.class").read_bytes()

    # jar vacío: cero clases, cero límites
    vacio = trabajo / "vacio.jar"
    with zipfile.ZipFile(vacio, "w") as z:
        z.writestr("LEEME.txt", "sin clases")
    rutas = extraer_clases(vacio, trabajo / "vacio-classes")
    inv = mide(apkc, rutas, trabajo / "out.dex")
    if inv["clases"] != 0 or inv["fallos"] != 0:
        fallos.append(f"el jar vacío cuenta {inv['clases']} clases y {inv['fallos']} fallos")

    # la clase sana, la rota y la restaurada, cada una en su jar
    resultados = {}
    for nombre, datos in (("sana", original), ("rota", romper_primera_code(original)),
                          ("restaurada", original)):
        jar = trabajo / f"fixture-{nombre}.jar"
        with zipfile.ZipFile(jar, "w") as z:
            z.writestr("fixture/Sano.class", datos)
        rutas = extraer_clases(jar, trabajo / f"fixture-{nombre}-classes")
        resultados[nombre] = mide(apkc, rutas, trabajo / "out.dex")

    sanos = sum(len(g["casos"]) for g in resultados["sana"]["limites"].values())
    rotos = sum(len(g["casos"]) for g in resultados["rota"]["limites"].values())
    restaurados = sum(len(g["casos"]) for g in resultados["restaurada"]["limites"].values())
    if rotos != sanos + 1:
        fallos.append(f"la clase rota cuenta {rotos} límites y se esperaban {sanos + 1}")
    if restaurados != sanos:
        fallos.append(f"tras restaurar quedan {restaurados} límites y había {sanos}")
    caso = None
    for grupo in resultados["rota"]["limites"].values():
        for c in grupo["casos"]:
            if c.get("byte") == "0xca":
                caso = c
    if caso is None:
        fallos.append("el caso roto no señala el byte 0xca")

    lineas = [
        "## Contador",
        "",
        f"- Jar vacío: {inv['clases']} clases, {inv['fallos']} fallos.",
        f"- Fixture sano: {sanos} límites.",
        f"- Fixture con el primer opcode cambiado a `0xca`: {rotos} límites"
        + (f", el primero en `{muestra(caso)}`" if caso else "") + ".",
        f"- Fixture restaurado: {restaurados} límites.",
        "",
        "Veredicto del contador: " + ("PASA" if not fallos else "FALLA"),
    ]
    for f in fallos:
        lineas.append(f"- {f}")

    lineas_recon, rc_recon = comprobar_reconstruccion(trabajo)
    lineas.extend(lineas_recon)
    lineas_repro, rc_repro = comprobar_reproducibilidad(apkc, trabajo / "fixture-sana.jar")
    lineas.extend(lineas_repro)
    lineas_conc, rc_conc = comprobar_concurrencia(apkc, trabajo / "fixture-sana.jar", trabajo)
    lineas.extend(lineas_conc)
    lineas_bin, rc_bin = comprobar_concurrencia_binario(trabajo)
    lineas.extend(lineas_bin)
    lineas_arbol, rc_arbol = comprobar_arbol_limpio()
    lineas.extend(lineas_arbol)
    return lineas, 0 if (not fallos and rc_recon == 0 and rc_repro == 0 and rc_conc == 0
                         and rc_bin == 0 and rc_arbol == 0) else 1


def huella_del_jar(jar):
    """SHA-256 del jar, corto: fija el nombre del directorio de trabajo publicado."""
    return hashlib.sha256(Path(jar).read_bytes()).hexdigest()[:12]


def ruta_publicada(ruta):
    """Forma estable de una ruta de caché para el informe: `$CACHE/...`.

    El comando del oráculo se publica con esta forma, no con la ruta privada de la corrida, para
    que dos ejecuciones (y dos máquinas con el mismo `CACHE`) escriban el mismo texto.
    """
    try:
        rel = Path(ruta).resolve().relative_to(CACHE.resolve())
    except ValueError:
        return str(ruta)
    return "$CACHE/" + rel.as_posix()


def directorio_de_trabajo(jar):
    """Directorio de trabajo privado por proceso, con el prefijo estable de la huella del jar.

    Dos corridas concurrentes sobre el mismo jar no pueden compartir ni borrarse el directorio: cada
    una tiene el suyo (`trabajo-<12 hex>-<aleatorio de mkdtemp>`). Lo que se publica en el informe es
    la forma estable `$CACHE/trabajo-<12 hex>/d8`, no esta ruta privada.
    """
    huella = huella_del_jar(jar)
    CACHE.mkdir(parents=True, exist_ok=True)
    return Path(tempfile.mkdtemp(prefix=f"trabajo-{huella}-", dir=str(CACHE)))


def genera_informe(apkc, jar, trabajo, android_jar, sin_oraculo, limite=None):
    """Mide el jar y devuelve (informe markdown, detalle JSON) sin escribir nada.

    Es la misma ruta que usa el comando: la llaman `main` y la prueba de reproducibilidad, así que
    dos ejecuciones seguidas con el mismo jar y el mismo dexer tienen que dar los mismos bytes.
    """
    rutas = extraer_clases(jar, trabajo / "clases")
    inv = mide(apkc, rutas, trabajo / "out.dex", limite)

    # el fallo literal del comando completo: `apkc dex` sobre todas las clases de golpe
    _rc, texto_completo = dexa(apkc, trabajo / "clases", trabajo / "out.dex")

    lineas = []
    lineas.append("### J-bis: inventario de límites restantes")
    lineas.append("")
    lineas.append(f"Jar medido: `{jar}` ({jar.stat().st_size} bytes)")
    lineas.append("")
    lineas.append(f"- Clases recorridas: {inv['clases']}")
    lineas.append(f"- Clases que el dexer rechaza: {inv['fallos']}")
    lineas.append(f"- Tipos de límite: {len(inv['limites'])}")
    lineas.append("")
    if texto_completo:
        lineas.append("Primer fallo de `apkc dex` sobre las clases a la vez:")
        lineas.append("")
        lineas.append("```")
        lineas.append(texto_completo.splitlines()[-1])
        lineas.append("```")
        lineas.append("")
    lineas.append(tabla_limites(inv))
    lineas.append("")
    lineas.append("Cada clase se dexó en su propia invocación de `apkc dex`: el conteo de clases es "
                  "exacto. El de métodos es la primera que falla en cada clase, porque el ensamblado "
                  "se detiene ahí; es una cota inferior, no el total del jar.")
    lineas.append("")
    propuestas = [(tipo, EQUIVALENTES_DEX[tipo]) for tipo in
                  (t for t, _ in sorted(inv["limites"].items(),
                                        key=lambda kv: (-len(kv[1]["casos"]), kv[0]))
                   if t in EQUIVALENTES_DEX)]
    if propuestas:
        lineas.append("#### Equivalente DEX propuesto")
        lineas.append("")
        lineas.append("| Límite | Forma que falta |")
        lineas.append("| --- | --- |")
        for tipo, propuesta in propuestas:
            lineas.append(f"| {tipo} | {propuesta} |")
        lineas.append("")
    if inv["opcodes"]:
        lineas.append("#### Opcodes JVM fuera de la tabla")
        lineas.append("")
        lineas.append(tabla_opcodes(inv))
        lineas.append("")
    if not sin_oraculo:
        lineas.append("#### Oráculo d8/dexdump")
        lineas.append("")
        muestras = []
        for tipo, grupo in sorted(inv["limites"].items(),
                                  key=lambda kv: (-len(kv[1]["casos"]), kv[0])):
            primero = grupo["casos"][0]
            if primero.get("clase") and primero.get("metodo"):
                muestras.append((tipo, primero["clase"], primero["metodo"]))
        lineas.append(oraculo_d8(jar, trabajo, Path(android_jar), muestras,
                                 CACHE / f"trabajo-{huella_del_jar(jar)}"))
    lineas.append("Comando que lo reproduce:")
    lineas.append("")
    lineas.append("```sh")
    lineas.append("tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py")
    lineas.append("```")
    lineas.append("")

    detalle = {tipo: [{k: caso.get(k) for k in ("clase", "metodo", "pc", "opcode", "motivo")}
                      for caso in grupo["casos"]]
               for tipo, grupo in inv["limites"].items()}
    return "\n".join(lineas), json.dumps(detalle, indent=2, ensure_ascii=False) + "\n"


def main():
    p = argparse.ArgumentParser(description="Inventario de límites del dexer sobre un jar real")
    p.add_argument("--jar", default=str(JAR_POR_DEFECTO))
    p.add_argument("--apkc", default="")
    p.add_argument("--apkc-dir", default=str(APKC_DIR),
                   help="código de apkc/ que se compila cuando no se pasa --apkc")
    p.add_argument("--salida", default=str(RAIZ / "banco" / "inventario-jar" / "informe.md"))
    p.add_argument("--android-jar", default=str(ANDROID_JAR_POR_DEFECTO))
    p.add_argument("--sin-oraculo", action="store_true")
    p.add_argument("--falsificar", action="store_true")
    p.add_argument("--limite", type=int, default=None, help="mide solo las N primeras clases")
    args = p.parse_args()

    jar = Path(args.jar)
    if not jar.exists():
        print(f"[inventario] no existe el jar {jar}", file=sys.stderr)
        return 2
    apkc = construir_apkc(args.apkc, directorio=Path(args.apkc_dir))
    CACHE.mkdir(parents=True, exist_ok=True)
    trabajo = directorio_de_trabajo(jar)
    atexit.register(shutil.rmtree, trabajo, ignore_errors=True)
    print(f"[inventario] directorio de trabajo {trabajo}", file=sys.stderr)

    if args.falsificar:
        lineas, rc = comprobar_falsificacion(apkc, trabajo)
        # la salida detallada lleva hashes de los fixtures y la ruta del directorio de trabajo:
        # va a la caché, nunca al fichero versionado, para que la sonda deje el árbol limpio
        destino = CACHE / "falsificacion.md"
        destino.parent.mkdir(parents=True, exist_ok=True)
        destino.write_text("\n".join(lineas) + "\n")
        print("\n".join(lineas))
        print(f"[inventario] escrito {destino}", file=sys.stderr)
        return rc

    informe, detalle = genera_informe(apkc, jar, trabajo, args.android_jar,
                                      args.sin_oraculo, args.limite)

    salida = Path(args.salida)
    salida.parent.mkdir(parents=True, exist_ok=True)
    salida.write_text(informe)
    # el detalle máquina-legible, para que cada agente ataque un límite sin volver a correr el jar
    (salida.parent / "casos.json").write_text(detalle)
    print(informe)
    print(f"[inventario] escrito {salida}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
