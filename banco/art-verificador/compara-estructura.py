#!/usr/bin/env python3
"""Compara la estructura de dos volcados de `dexdump -a`: el de `d8` y el nuestro.

No compara instrucciones: los registros los reparte un asignador distinto y esa diferencia está
aceptada. Compara lo que tiene que cuadrar y lo que ART comprueba: las clases con su superclase e
interfaces, cada campo con su tipo, sus accesos y su `static_values` (el `value:` que imprime
`dexdump`), cada método con su descriptor y sus accesos, y cada bloque de anotaciones.

    uso: compara-estructura.py <dump de d8> <dump nuestro> <caso>
"""
import re
import sys

if len(sys.argv) != 4:
    sys.exit("uso: compara-estructura.py <dump de d8> <dump nuestro> <caso>")
_, ruta_d8, ruta_nuestro, caso = sys.argv

RE_CLASE = re.compile(r"^Class #(\d+)\s+-\s*$")
RE_ANOT = re.compile(r"^Class #(\d+)\s+annotations:\s*$")
RE_TARGET = re.compile(r"^Annotations on (class|field #\d+ '[^']*'|method #\d+ '[^']*')\s*:?\s*$")
RE_DESCRIPTOR = re.compile(r"^\s+Class descriptor\s+: '(.*)'\s*$")
RE_SUPER = re.compile(r"^\s+Superclass\s+: '(.*)'\s*$")
RE_INTERFAZ = re.compile(r"^\s+Interfaces\s+(.*)$")
RE_SECCION = re.compile(r"^\s+(Static fields|Instance fields|Direct methods|Virtual methods)\s*-?\s*$")
RE_MIEMBRO = re.compile(r"^\s+#\d+\s*:\s*\(in (.*)\)\s*$")
RE_NOMBRE = re.compile(r"^\s+name\s+: '(.*)'\s*$")
RE_TIPO = re.compile(r"^\s+type\s+: '(.*)'\s*$")
RE_ACCESO = re.compile(r"^\s+access\s+: (0x[0-9a-f]+)\s*(\(.*\))?\s*$")
RE_VALOR = re.compile(r"^\s+value\s+: (.*)\s*$")


def parsea(ruta):
    clases = {}
    anotaciones = {}
    clase_actual = None
    seccion = None
    miembro = None
    clase_anotada = None

    def cierra():
        nonlocal miembro
        if clase_actual is not None and miembro is not None:
            clases[clase_actual]["miembros"].append(
                (seccion, miembro["nombre"], miembro["tipo"], miembro["acceso"], miembro["valor"]))
        miembro = None

    for linea in open(ruta, encoding="utf-8", errors="replace"):
        linea = linea.rstrip("\n")
        m = RE_ANOT.match(linea)
        if m:
            cierra()
            anotaciones.setdefault(int(m.group(1)), [])
            clase_anotada = int(m.group(1))
            continue
        m = RE_TARGET.match(linea)
        if m:
            anotaciones.setdefault(clase_anotada, []).append((m.group(1), []))
            continue
        if clase_anotada is not None and re.match(r"^  VISIBILITY_", linea):
            if not anotaciones[clase_anotada] or not isinstance(anotaciones[clase_anotada][-1], tuple):
                anotaciones[clase_anotada].append(("clase", []))
            anotaciones[clase_anotada][-1][1].append(" ".join(linea.split()))
            continue
        m = RE_CLASE.match(linea)
        if m:
            cierra()
            clase_anotada = None
            clase_actual = int(m.group(1))
            clases[clase_actual] = {"descriptor": "?", "super": "?", "interfaces": ("",),
                                    "miembros": []}
            continue
        if clase_actual is None:
            continue
        m = RE_DESCRIPTOR.match(linea)
        if m:
            clases[clase_actual]["descriptor"] = m.group(1)
            continue
        m = RE_SUPER.match(linea)
        if m:
            clases[clase_actual]["super"] = m.group(1)
            continue
        m = RE_INTERFAZ.match(linea)
        if m:
            resto = m.group(1).strip()
            if resto != "-":
                clases[clase_actual]["interfaces"] += (resto,)
            continue
        m = RE_SECCION.match(linea)
        if m:
            cierra()
            seccion = m.group(1)
            continue
        m = RE_MIEMBRO.match(linea)
        if m:
            cierra()
            miembro = {"nombre": "?", "tipo": "?", "acceso": "?", "valor": ""}
            continue
        if miembro is None:
            continue
        m = RE_NOMBRE.match(linea)
        if m:
            miembro["nombre"] = m.group(1)
            continue
        m = RE_TIPO.match(linea)
        if m:
            miembro["tipo"] = m.group(1)
            continue
        m = RE_ACCESO.match(linea)
        if m:
            miembro["acceso"] = m.group(1)
            continue
        m = RE_VALOR.match(linea)
        if m:
            miembro["valor"] = m.group(1).strip()
            continue
    cierra()

    resumen = {}
    for i, c in clases.items():
        claves = []
        for sec, nombre, tipo, acceso, valor in c["miembros"]:
            # `d8` propaga el `<clinit>` a `static_values_off`; este dexer escribe el cero del tipo y
            # deja que lo sobrescriba el `<clinit>` al correr (está medido en `dexarma.go`: ART no
            # rechaza un cero del tipo correcto). Lo que sí se compara es el tipo y los accesos; el
            # valor solo cuando el campo lleva `ConstantValue`, que es cuando los dos tienen que
            # decir lo mismo.
            if sec == "Static fields" and "FINAL" not in acceso:
                valor = "(lo pone el <clinit>)"
            claves.append("%s %s : %s [%s] = %s" % (sec, nombre, tipo, acceso, valor))
        resumen[c["descriptor"]] = {
            "super": c["super"],
            "interfaces": tuple(sorted(c["interfaces"])),
            "miembros": sorted(claves),
            "anotaciones": sorted("%s :: %s" % (t, " | ".join(v))
                                  for t, v in anotaciones.get(i, [])),
        }
    return resumen


suyo = parsea(ruta_d8)
nuestro = parsea(ruta_nuestro)
fallos = []

for desc in sorted(set(suyo) | set(nuestro)):
    if desc not in suyo:
        fallos.append("clase de más: %s" % desc)
        continue
    if desc not in nuestro:
        fallos.append("clase que falta: %s" % desc)
        continue
    a, b = suyo[desc], nuestro[desc]
    if a["super"] != b["super"]:
        fallos.append("%s: superclase %s contra %s" % (desc, a["super"], b["super"]))
    if a["interfaces"] != b["interfaces"]:
        fallos.append("%s: interfaces %s contra %s" % (desc, a["interfaces"], b["interfaces"]))
    for m in a["miembros"]:
        if m not in b["miembros"]:
            fallos.append("%s: miembro que falta o cambia: %s" % (desc, m))
    for m in b["miembros"]:
        if m not in a["miembros"]:
            fallos.append("%s: miembro de más o distinto: %s" % (desc, m))
    for an in a["anotaciones"]:
        if an not in b["anotaciones"]:
            fallos.append("%s: anotación que falta o cambia: %s" % (desc, an))
    for an in b["anotaciones"]:
        if an not in a["anotaciones"]:
            fallos.append("%s: anotación de más o distinta: %s" % (desc, an))

if fallos:
    print("   %d clases en d8, %d en nuestro" % (len(suyo), len(nuestro)))
    for f in fallos:
        print("   | " + f)
    sys.exit("FALLO: %s: la estructura no coincide con la de d8" % caso)

print("   %d clases, %d miembros y %d bloques de anotaciones iguales que d8" % (
    len(suyo),
    sum(len(c["miembros"]) for c in suyo.values()),
    sum(len(c["anotaciones"]) for c in suyo.values())))
