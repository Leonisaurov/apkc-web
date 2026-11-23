#!/usr/bin/env python3
"""Reemplaza `classes.dex` de un APK ya construido por otro dex, sin tocar nada más.

Se usa para medir el dexer por separado del resto del empaquetado: el APK del camino de `d8` sale
de `apkc build --compiler javac`, y el del camino propio es el mismo APK con el `classes.dex` que
escribió `apkc dex` sobre los mismos `.class`. La firma anterior deja de valer y el guion vuelve a
alinear y firmar; aquí solo se reescribe el zip.

    uso: reemplaza-dex.py <apk de entrada> <apk de salida> <dex nuevo>
"""
import sys
import zipfile

if len(sys.argv) != 4:
    sys.exit("uso: reemplaza-dex.py <apk de entrada> <apk de salida> <dex nuevo>")

origen, destino, dex = sys.argv[1], sys.argv[2], sys.argv[3]
datos = open(dex, "rb").read()
visto = False
with zipfile.ZipFile(origen) as zin, zipfile.ZipFile(destino, "w", zipfile.ZIP_DEFLATED) as zout:
    for item in zin.infolist():
        cuerpo = zin.read(item.filename)
        if item.filename == "classes.dex":
            cuerpo = datos
            visto = True
        zout.writestr(item, cuerpo)
if not visto:
    sys.exit("el APK de entrada no lleva classes.dex")
print("classes.dex reemplazado por %d bytes" % len(datos))
