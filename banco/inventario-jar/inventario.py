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


def construir_apkc(ruta):
    """Devuelve el binario de apkc: el que se le pase, el del entorno o uno construido en caché."""
    if ruta:
        return str(Path(ruta))
    del_entorno = os.environ.get("APKC")
    if del_entorno:
        return del_entorno
    destino = CACHE / "apkc"
    if not destino.exists():
        CACHE.mkdir(parents=True, exist_ok=True)
        print(f"[inventario] construyendo apkc en {destino}", file=sys.stderr)
        subprocess.run(["go", "build", "-o", str(destino), "."], cwd=APKC_DIR, check=True)
    return str(destino)


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


def oraculo_d8(jar, trabajo, android_jar, muestras):
    """Dexa el jar entero con d8 y enseña las instrucciones DEX de los primeros casos.

    `muestras` es una lista de (tipo, clase, método): para cada una se recorta del `dexdump -d`
    del dex de d8 el trozo de ese método, que es el equivalente que propone el dexer de referencia.
    """
    d8, dexdump = buscar_d8()
    if d8 is None or not Path(d8).exists():
        return "d8 no está disponible: no se pudo comprobar el jar contra el dexer de referencia.\n"
    salida = trabajo / "d8"
    if salida.exists():
        shutil.rmtree(salida)
    salida.mkdir(parents=True)
    comando = [str(d8), "--min-api", "21", "--lib", str(android_jar), "--output", str(salida), str(jar)]
    r = subprocess.run(comando, capture_output=True, text=True)
    lineas = ["Comando del oráculo:", "", "```sh", " ".join(comando), "```", ""]
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
        "## Falsificación del contador",
        "",
        f"- Jar vacío: {inv['clases']} clases, {inv['fallos']} fallos.",
        f"- Fixture sano: {sanos} límites.",
        f"- Fixture con el primer opcode cambiado a `0xca`: {rotos} límites"
        + (f", el primero en `{muestra(caso)}`" if caso else "") + ".",
        f"- Fixture restaurado: {restaurados} límites.",
        "",
        "Veredicto: " + ("PASA" if not fallos else "FALLA"),
    ]
    for f in fallos:
        lineas.append(f"- {f}")
    return lineas, 0 if not fallos else 1


def main():
    p = argparse.ArgumentParser(description="Inventario de límites del dexer sobre un jar real")
    p.add_argument("--jar", default=str(JAR_POR_DEFECTO))
    p.add_argument("--apkc", default="")
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
    apkc = construir_apkc(args.apkc)
    CACHE.mkdir(parents=True, exist_ok=True)
    trabajo = Path(tempfile.mkdtemp(prefix="trabajo-", dir=str(CACHE)))
    # tempfile ya creó el directorio; los jars y clases van debajo
    print(f"[inventario] temporales en {trabajo}", file=sys.stderr)

    if args.falsificar:
        lineas, rc = comprobar_falsificacion(apkc, trabajo)
        destino = Path(args.salida).parent / "falsificacion.md"
        destino.parent.mkdir(parents=True, exist_ok=True)
        destino.write_text("\n".join(lineas) + "\n")
        print("\n".join(lineas))
        print(f"[inventario] escrito {destino}", file=sys.stderr)
        return rc

    rutas = extraer_clases(jar, trabajo / "clases")
    inv = mide(apkc, rutas, trabajo / "out.dex", args.limite)

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
    if not args.sin_oraculo:
        lineas.append("#### Oráculo d8/dexdump")
        lineas.append("")
        muestras = []
        for tipo, grupo in sorted(inv["limites"].items(),
                                  key=lambda kv: (-len(kv[1]["casos"]), kv[0])):
            primero = grupo["casos"][0]
            if primero.get("clase") and primero.get("metodo"):
                muestras.append((tipo, primero["clase"], primero["metodo"]))
        lineas.append(oraculo_d8(jar, trabajo, Path(args.android_jar), muestras))
    lineas.append("Comando que lo reproduce:")
    lineas.append("")
    lineas.append("```sh")
    lineas.append("tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py")
    lineas.append("```")
    lineas.append("")
    informe = "\n".join(lineas)

    salida = Path(args.salida)
    salida.parent.mkdir(parents=True, exist_ok=True)
    salida.write_text(informe)
    # el detalle máquina-legible, para que cada agente ataque un límite sin volver a correr el jar
    import json
    detalle = {tipo: [{k: caso.get(k) for k in ("clase", "metodo", "pc", "opcode", "motivo")}
                      for caso in grupo["casos"]]
               for tipo, grupo in inv["limites"].items()}
    (salida.parent / "casos.json").write_text(json.dumps(detalle, indent=2, ensure_ascii=False) + "\n")
    print(informe)
    print(f"[inventario] escrito {salida}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
