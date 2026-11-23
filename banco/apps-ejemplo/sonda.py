#!/usr/bin/env python3
"""Utilidades de la sonda de endurecimiento de las apps de ejemplo.

No es un gate: es la mitad "aparato" y la mitad "comparación" de
`banco/apps-ejemplo/endurecer.sh`. El guion de shell compila, instala y arranca; aquí se toca por
`adb`, se vuelca `uiautomator dump` y se compara eso con el modelo de `apkc preview`.

Subcomandos:
    id <preview.json> <nombre>                  -> idRef de `id/<nombre>` (0x7f010000)
    tocar <serie> <paquete> <nombre>            -> toca el centro del nodo con ese recurso
    escribir <serie> <paquete> <nombre> <texto> -> toca el campo y teclea
    volcar <serie> <paquete> <ruta.xml>         -> dump del aparato y pares id/texto por stdout
    secuencia <serie> <paquete> <ruta.xml> <paso>...
                                                -> vuelca UNA vez y aplica la secuencia por las
                                                   coordenadas de ese volcado (una sola espera de
                                                   uiautomator por app; ver la nota de Droidloom)
    comparar <app> <nota> <preview.json> <aparato.txt> [<aparato2.txt>]
                                                -> sección del informe en markdown

Los códigos de salida: 0 bien, 2 el nodo o el recurso no están (se avisa y se sigue).
"""

import json
import subprocess
import sys
import time
import xml.etree.ElementTree as ET

TIEMPO_DUMP = 3


def falla(texto):
    print("sonda: " + texto, file=sys.stderr)
    return 2


def adb(serie, *args):
    orden = ["adb", "-s", serie, "shell"] + list(args)
    return subprocess.run(orden, capture_output=True, text=True)


def volcar_xml(serie, paquete=None, intentos=12):
    """uiautomator dump con reintentos.

    Dos fallos distintos: «could not get idle state» si la pantalla se mueve, y un volcado que
    todavía enseña otra app porque el arranque no ha terminado. En el segundo caso el XML es válido
    pero no trae nodos del paquete esperado, así que se reintenta hasta que aparece.
    """
    ultimo = ""
    for _ in range(intentos):
        hecho = adb(serie, "uiautomator", "dump", "/sdcard/sonda.xml")
        ultimo = (hecho.stdout or "") + (hecho.stderr or "")
        if "dumped" in ultimo:
            texto = adb(serie, "cat", "/sdcard/sonda.xml").stdout
            if texto.strip().startswith("<?xml"):
                if paquete is None:
                    return texto
                for nodo in nodos(texto):
                    if nodo.attrib.get("package") == paquete:
                        return texto
        time.sleep(1)
    raise RuntimeError("uiautomator dump no enseñó " + str(paquete) + ": " + ultimo.strip()[-200:])


def nodos(xml_texto):
    raiz = ET.fromstring(xml_texto)
    return list(raiz.iter("node"))


def texto_de(nodo):
    texto = nodo.attrib.get("text") or ""
    if texto.strip() == "":
        texto = nodo.attrib.get("hint") or ""
    return texto


def centro(nodo):
    import re

    cajas = re.findall(r"\[(-?\d+),(-?\d+)\]", nodo.attrib.get("bounds", ""))
    if len(cajas) != 2:
        return None
    (x1, y1), (x2, y2) = [(int(a), int(b)) for a, b in cajas]
    return ((x1 + x2) // 2, (y1 + y2) // 2)


def buscar(lista, paquete, nombre):
    buscado = paquete + ":id/" + nombre
    for nodo in lista:
        if nodo.attrib.get("resource-id") == buscado:
            return nodo
    return None


def sub_id(preview, nombre):
    with open(preview, encoding="utf-8") as f:
        datos = json.load(f)

    def recorre(nodo):
        if nodo.get("id") == "id/" + nombre:
            return nodo.get("idRef")
        for hijo in nodo.get("hijos", []):
            encontrado = recorre(hijo)
            if encontrado:
                return encontrado
        return None

    valor = recorre(datos.get("raiz", {}))
    if not valor:
        print("sonda: el preview no tiene id/" + nombre, file=sys.stderr)
        sys.exit(2)
    print(valor)


def sub_tocar(serie, paquete, nombre, texto=None):
    try:
        texto_xml = volcar_xml(serie, paquete)
    except RuntimeError as fallo:
        return falla(str(fallo))
    lista = nodos(texto_xml)
    nodo = buscar(lista, paquete, nombre)
    if nodo is None:
        return falla("no encuentro " + paquete + ":id/" + nombre + " en el aparato")
    punto = centro(nodo)
    if punto is None:
        return falla("el nodo id/" + nombre + " no tiene bounds")
    adb(serie, "input", "tap", str(punto[0]), str(punto[1]))
    if texto is None:
        return 0
    time.sleep(0.4)
    # `input text` no entiende espacios: se piden como %s, que es lo que espera el propio `input`.
    adb(serie, "input", "text", texto.replace(" ", "%s"))
    return 0


def sub_volcar(serie, paquete, ruta):
    try:
        xml_texto = volcar_xml(serie, paquete)
    except RuntimeError as fallo:
        return falla(str(fallo))
    with open(ruta, "w", encoding="utf-8") as f:
        f.write(xml_texto)
    for nodo in nodos(xml_texto):
        ident = nodo.attrib.get("resource-id") or ""
        if not ident.startswith(paquete + ":id/"):
            continue
        nombre = ident.split(":id/", 1)[1]
        clase = (nodo.attrib.get("class") or "").split(".")[-1]
        texto = texto_de(nodo)
        filas = sum(1 for hijo in nodo if (hijo.attrib.get("text") or "").strip() != "")
        print(nombre + "\t" + clase + "\t" + texto.replace("\t", " ") + "\t" + str(filas))
    return 0


def estado_preview(preview):
    with open(preview, encoding="utf-8") as f:
        datos = json.load(f)
    estado = {}

    def recorre(nodo):
        ident = nodo.get("id")
        if ident and ident.startswith("id/"):
            nombre = ident[3:]
            texto = nodo.get("textoDinamico")
            if texto is None:
                texto = nodo.get("texto") or ""
            if texto == "":
                texto = nodo.get("pista") or ""
            filas = len(nodo.get("hijos", []))
            estado[nombre] = (nodo.get("tipo", ""), texto, filas)
        for hijo in nodo.get("hijos", []):
            recorre(hijo)

    recorre(datos.get("raiz", {}))
    ejecucion = datos.get("ejecucion", {})
    total = datos.get("nodos", 0)
    return estado, total, ejecucion.get("avisos") or []


def estado_aparato(ruta):
    with open(ruta, encoding="utf-8") as f:
        lineas = [l.rstrip("\n") for l in f if l.strip()]
    estado = {}
    for linea in lineas:
        partes = linea.split("\t")
        while len(partes) < 4:
            partes.append("")
        estado[partes[0]] = (partes[1], partes[2], int(partes[3] or "0"))
    return estado


def sub_comparar(app, nota, preview, aparato, aparato2=None):
    de_preview, total, avisos = estado_preview(preview)
    del_aparato = estado_aparato(aparato)
    print("### " + app)
    print()
    if nota:
        print("- " + nota)
    print("- vistas: preview " + str(total) + " nodos; aparato " + str(len(del_aparato)) + " con id")
    if avisos:
        print("- avisos del intérprete de preview (no son diferencias con el aparato):")
        for aviso in avisos:
            print("  - `" + aviso + "`")
    print()
    print("| id | preview | aparato | veredicto |")
    print("| --- | --- | --- | --- |")
    diferencias = 0
    for nombre in sorted(set(de_preview) | set(del_aparato)):
        p = de_preview.get(nombre)
        a = del_aparato.get(nombre)
        texto_p = "-" if p is None else (p[0] + ": " + p[1])
        texto_a = "-" if a is None else (a[0] + ": " + a[1])
        veredicto = "ok"
        if p is None:
            veredicto = "AVISO: solo en el aparato"
            diferencias += 1
        elif a is None:
            veredicto = "AVISO: solo en el preview"
            diferencias += 1
        elif p[1] != a[1]:
            # El tema del aparato pinta el texto de un Button en versales (textAllCaps del estilo):
            # es una diferencia de presentación, no de estado, y se marca como tal.
            if (p[0].endswith("Button") or a[0].endswith("Button")) and p[1].lower() == a[1].lower():
                veredicto = "ok (versales del tema en el aparato)"
            else:
                veredicto = "AVISO: texto distinto"
                diferencias += 1
        print("| id/" + nombre + " | `" + texto_p + "` | `" + texto_a + "` | " + veredicto + " |")
        if p is not None and a is not None and p[2] != a[2] and (p[2] or a[2]):
            print("| id/" + nombre + " (filas) | `" + str(p[2]) + "` | `" + str(a[2]) + "` | AVISO: filas distintas |")
            diferencias += 1
    if aparato2:
        print()
        segundo = estado_aparato(aparato2)
        print("Tras `am force-stop` y relanzar, contra el estado anterior al reinicio. Solo cuenta el")
        print("contenido de las listas: el resto de textos es estado de pantalla y se reinicia a propósito.")
        print()
        print("| id | antes | después | veredicto |")
        print("| --- | --- | --- | --- |")
        for nombre in sorted(set(del_aparato) | set(segundo)):
            antes = del_aparato.get(nombre)
            despues = segundo.get(nombre)
            clase = (despues or antes or ("",))[0]
            if clase != "ListView":
                continue
            texto_antes = "-" if antes is None else antes[1]
            texto_despues = "-" if despues is None else despues[1]
            veredicto = "ok"
            if texto_antes != texto_despues:
                veredicto = "AVISO: no sobrevive"
                diferencias += 1
            print("| id/" + nombre + " | `" + texto_antes + "` | `" + texto_despues + "` | " + veredicto + " |")
    print()
    print("Diferencias marcadas: " + str(diferencias))
    print()
    return 0


# La ventana de la app en Droidloom se retira a los pocos segundos y su home ocupa la pantalla:
# volver a levantarla exige force-stop + start, que reinicia el estado en memoria. Por eso la
# secuencia se aplica de una vez, con las coordenadas de un único volcado, y no con un volcado por
# toque: así la app no llega a perder la ventana a mitad de la secuencia.
def sub_secuencia(serie, paquete, xml_salida, pasos):
    try:
        xml_texto = volcar_xml(serie, paquete)
    except RuntimeError as fallo:
        return falla(str(fallo))
    with open(xml_salida, "w", encoding="utf-8") as f:
        f.write(xml_texto)
    lista = nodos(xml_texto)
    for paso in pasos:
        if ":" not in paso:
            return falla("paso sin forma: " + paso)
        orden, resto = paso.split(":", 1)
        if orden == "pulsar":
            nodo = buscar(lista, paquete, resto)
            if nodo is None:
                print("sonda: no encuentro id/" + resto + "; sigo", file=sys.stderr)
                continue
            punto = centro(nodo)
            if punto is None:
                print("sonda: id/" + resto + " sin bounds; sigo", file=sys.stderr)
                continue
            adb(serie, "input", "tap", str(punto[0]), str(punto[1]))
        elif orden == "escribir":
            campo, _, texto = resto.partition("=")
            nodo = buscar(lista, paquete, campo)
            if nodo is None:
                print("sonda: no encuentro id/" + campo + "; sigo", file=sys.stderr)
                continue
            punto = centro(nodo)
            if punto is not None:
                adb(serie, "input", "tap", str(punto[0]), str(punto[1]))
            time.sleep(0.2)
            adb(serie, "input", "text", texto.replace(" ", "%s"))
        elif orden == "avanzar":
            time.sleep(int(resto) / 1000.0)
        else:
            return falla("paso desconocido: " + paso)
    return 0


def principal(argv):
    if len(argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2
    cual = argv[1]
    if cual == "id" and len(argv) == 4:
        return sub_id(argv[2], argv[3]) or 0
    if cual == "tocar" and len(argv) == 5:
        return sub_tocar(argv[2], argv[3], argv[4])
    if cual == "escribir" and len(argv) == 6:
        return sub_tocar(argv[2], argv[3], argv[4], argv[5])
    if cual == "volcar" and len(argv) == 5:
        return sub_volcar(argv[2], argv[3], argv[4])
    if cual == "secuencia" and len(argv) >= 5:
        return sub_secuencia(argv[2], argv[3], argv[4], argv[5:])
    if cual == "comparar" and len(argv) in (6, 7):
        return sub_comparar(argv[2], argv[3], argv[4], argv[5], argv[6] if len(argv) == 7 else None)
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(principal(sys.argv))
