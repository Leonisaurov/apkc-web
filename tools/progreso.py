#!/usr/bin/env python3
"""Genera el estado del panel de progreso desde docs/plan.md.

Los ítems salen del inventario de la Fase 5 (filas `| 5.x | ... | estado |`), más las
fronteras finas que no tienen fila propia (5.8, 5.9, 5.12, 5.13, recursos y aparato).
El panel hace fetch de este JSON cada dos segundos, así que basta con reescribirlo para
que la pantalla secundaria se actualice sin recargar ni abrir pestañas.

uso: python3 tools/progreso.py [salida.json]
"""

import json
import re
import sys
from pathlib import Path

RAIZ = Path(__file__).resolve().parent.parent
PLAN = RAIZ / "docs" / "plan.md"
ACTUAL = Path("/data/data/com.termux/files/usr/tmp/apkc-progreso/actual.txt")

# Fronteras que el inventario marca como hechas pero el usuario pidió cerrar. Se marcan
# aparte para que el panel enseñe lo que de verdad queda.
EXTRAS = [
    ("5.8a", "cuenta ancha de flotante de tres registros", "hecha (§130)", True),
    ("5.8b", "ConstantValue de un static final long", "hecho (§130)", True),
    ("5.9a", "continue que sale de try/finally", "hecho (§132)", True),
    ("5.9b", "dos finally anidados con salida pendiente", "hecho (§139)", True),
    ("5.9c", "catch (A | B e)", "hecho (§132)", True),
    ("5.12a", "caída al siguiente brazo del switch", "hecha (§129)", True),
    ("5.13a", "ART, instanceof T[] y forma negada", "hecho (§131 y §134)", True),
    ("REC-1", "recursos: más densidades y orientaciones", "hecho (§137)", True),
    ("REC-2", "recursos: format enum/flags", "hecho (§135)", True),
    ("REC-3", "recursos: atributos de <permission>", "hecho (§138)", True),
    ("APARATO", "instalar y arrancar la app de cierre", "instalada y arrancada (§136)", True),
]


def celdas(linea: str) -> list[str]:
    """Separa una fila markdown por | sin romper los \\| de los code spans."""
    partes = re.split(r"(?<!\\)\|", linea.strip())
    return [p.strip().replace("\\|", "|") for p in partes]


def filas_del_inventario(texto: str) -> list[dict]:
    filas = []
    for linea in texto.splitlines():
        if not re.match(r"^\|\s*5\.\d+\s*\|", linea):
            continue
        c = celdas(linea)
        if len(c) < 4:
            continue
        numero = c[1].strip()
        que = re.sub(r"\s+", " ", c[2]).strip()
        # la última celda con contenido es el estado; las demás son la medición
        estado = next((x for x in reversed(c[3:]) if x), "abierto")
        if estado not in ("hecho", "parcial", "M restante", "abierto"):
            # la tabla no siempre pone el estado en la última celda; se busca la palabra
            estado = next((x for x in reversed(c[3:]) if x in ("hecho", "parcial", "M restante", "abierto")), "abierto")
        filas.append({
            "text": f"{numero} · {que}",
            "done": estado == "hecho",
            "detail": f"estado: {estado}",
        })
    return filas


def main() -> int:
    salida = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("/data/data/com.termux/files/usr/tmp/apkc-progreso/tareas.json")
    texto = PLAN.read_text(encoding="utf-8")
    items = filas_del_inventario(texto)
    for numero, texto_item, detalle, hecho in EXTRAS:
        items.append({"text": f"{numero} · {texto_item}", "done": hecho, "detail": detalle})
    actual = ACTUAL.read_text(encoding="utf-8").strip() if ACTUAL.exists() else ""
    salida.parent.mkdir(parents=True, exist_ok=True)
    temporal = salida.with_suffix(salida.suffix + ".tmp")
    temporal.write_text(json.dumps({"actual": actual, "items": items}, ensure_ascii=False, indent=1), encoding="utf-8")
    temporal.replace(salida)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
