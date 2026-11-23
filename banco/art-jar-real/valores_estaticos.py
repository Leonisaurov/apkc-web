#!/usr/bin/env python3
# valores_estaticos.py — los encoded_value de class_defs.static_values_off que no casan con el tipo
# del campo.
#
#   python3 banco/art-jar-real/valores_estaticos.py classes2.dex
#
# Para leer el dex del jar desde un APK: `unzip -o app.apk classes2.dex -d /tmp/x`.
#
# ART comprueba `static_values_off` al cargar el fichero y rechaza el dex entero si un valor no
# tiene el tipo del campo («unexpected static field initial value type»). Nuestro dexer escribe el
# valor de cualquier constante entera como VALUE_INT (`apkc/dexarma.go:613`, `case cpInteger`) sin
# mirar el descriptor del campo, y `dexdump` no lo comprueba: este guion lo enseña.
import struct
import sys

# nombre del tipo de encoded_value por su byte bajo
TIPOS = {0x00: "byte", 0x02: "short", 0x03: "char", 0x04: "int", 0x06: "long",
         0x10: "float", 0x11: "double", 0x15: "method_type", 0x16: "method_handle",
         0x17: "string", 0x18: "type", 0x19: "field", 0x1a: "method", 0x1b: "enum",
         0x1c: "array", 0x1d: "annotation", 0x1e: "null", 0x1f: "boolean"}
# lo que el descriptor del campo pide, por su primera letra
ESPERADO = {"Z": "boolean", "B": "byte", "S": "short", "C": "char", "I": "int",
            "J": "long", "F": "float", "D": "double"}
PRIMITIVOS = set(ESPERADO.values())


def uleb(d, p):
    v = 0
    s = 0
    while True:
        b = d[p]
        p += 1
        v |= (b & 0x7f) << s
        if not b & 0x80:
            return v, p
        s += 7


def cadena(d, off):
    _, p = uleb(d, off)
    return d[p:d.index(0, p)].decode("utf-8", "replace")


def main(ruta):
    d = open(ruta, "rb").read()
    if d[:4] != b"dex\n":
        raise SystemExit("%s no es un dex" % ruta)
    n_cadenas, off_cadenas = struct.unpack_from("<II", d, 56)
    n_tipos, off_tipos = struct.unpack_from("<II", d, 64)
    n_campos, off_campos = struct.unpack_from("<II", d, 80)
    n_clases, off_clases = struct.unpack_from("<II", d, 96)
    cadenas = [cadena(d, struct.unpack_from("<I", d, off_cadenas + 4 * i)[0])
               for i in range(n_cadenas)]
    tipos = [cadenas[struct.unpack_from("<I", d, off_tipos + 4 * i)[0]] for i in range(n_tipos)]
    campos = []
    for i in range(n_campos):
        clase, tipo, nombre = struct.unpack_from("<HHI", d, off_campos + 8 * i)
        campos.append((tipos[clase], tipos[tipo], cadenas[nombre]))

    leidos = 0
    malos = []
    for i in range(n_clases):
        (_, _, _, _, _, _, datos, valores) = struct.unpack_from("<IIIIIIII", d,
                                                                off_clases + 32 * i)
        if not datos or not valores:
            continue
        p = datos
        estaticos, p = uleb(d, p)
        _, p = uleb(d, p)
        _, p = uleb(d, p)
        _, p = uleb(d, p)
        indices = []
        idx = 0
        for _ in range(estaticos):
            dif, p = uleb(d, p)
            _, p = uleb(d, p)
            idx += dif
            indices.append(idx)
        q = valores
        n, q = uleb(d, q)
        for j in range(n):
            vf = d[q]
            q += 1
            tipo = vf & 0x1f
            q += 0 if tipo in (0x1e, 0x1f) else int(vf >> 5 & 7) + 1
            if j >= len(indices):
                continue
            clase, tipocampo, nombre = campos[indices[j]]
            leidos += 1
            esperado = ESPERADO.get(tipocampo[0]) if tipocampo else None
            hay = TIPOS.get(tipo)
            if esperado in PRIMITIVOS and esperado != hay:
                malos.append((clase, nombre, tipocampo, hay))

    print("valores estáticos leídos: %d" % leidos)
    print("con un tipo que no casa con el campo: %d" % len(malos))
    for clase, nombre, tipocampo, hay in malos:
        print("  %-72s %-28s campo %-4s valor %s" % (clase, nombre, tipocampo, hay))
    return 0 if not malos else 1


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("uso: valores_estaticos.py <fichero.dex>")
    sys.exit(main(sys.argv[1]))
