#!/bin/sh
# Empaqueta el sitio estático que se publica: los ficheros versionados de studio/ (submódulo),
# apkc.wasm y wasm_exec.js. No mete keystores ni OPFS: eso vive en el navegador. El android.jar
# sintético de AOSP se mete solo si se pasa ANDROID_STUB_JAR (F3); el de la SDK nunca se versiona.
#
# uso: APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm sh tools/sitio.sh [destino]
#      ANDROID_STUB_JAR=$PREFIX/androidjar/android-stub-34-full.jar APKC_WASM=… sh tools/sitio.sh
#      tools/entorno.sh sitio [destino]
# por defecto el destino es ./sitio, listo para `cd sitio && surge .`
# sale 0 y deja <destino>/ y <destino>.zip
set -eu

AQUI=$(cd "$(dirname "$0")/.." && pwd)
DESTINO=${1:-$AQUI/sitio}
WASM=${APKC_WASM:?falta APKC_WASM con el binario wasm}
[ -f "$WASM" ] || { printf 'no existe el wasm: %s\n' "$WASM" >&2; exit 1; }
[ -d "$AQUI/studio" ] || { printf 'falta studio/ (submódulo sin inicializar)\n' >&2; exit 1; }

# Surge escribe el dominio en CNAME al publicar. Conservarlo entre reconstrucciones evita que
# cada `tools/entorno.sh sitio` obligue a volver a elegirlo. También se puede fijar de antemano
# con .surge-domain en la raíz del repo (ignorado por git).
CNAME=
if [ -f "$DESTINO/CNAME" ]; then CNAME=$(cat "$DESTINO/CNAME"); fi
if [ -z "$CNAME" ] && [ -f "$AQUI/.surge-domain" ]; then CNAME=$(cat "$AQUI/.surge-domain"); fi

rm -rf "$DESTINO" "${DESTINO}.zip"
mkdir -p "$DESTINO"

# solo lo versionado del submódulo: nada de wasm_exec.js local, apkc.wasm ni perfiles de prueba
if ! git -C "$AQUI/studio" archive --format=tar HEAD | tar -x -C "$DESTINO"; then
  printf 'no se pudo extraer studio/ desde su git\n' >&2
  exit 1
fi

# El paquete publicado es solo la aplicación. Las pruebas, el servidor de desarrollo y la
# documentación del submódulo no viajan; LICENSE y NOTICE sí, porque son obligatorias.
for dev in .gitignore AGENTS.md EDITOR.md README.md e2e.sh package.json servidor.js \
           prueba.js prueba-crear.js prueba-navegador.js prueba-pestana.js prueba-sitio.js \
           prueba-firefox.js servidor-estatico.js; do
  rm -f "$DESTINO/$dev"
done
rm -rf "$DESTINO/.github"
# studio/ posee el gitlink de apkc-emulador: `git archive` versiona .gitmodules como fichero y
# crea apkc-emulador/ como directorio vacío. Ninguna fuente Go viaja en el sitio (regla 4): el
# wasm ya se compila fuera y se sirve como artefacto. El directorio se quita con rm -rf, no con
# rm -f: sobre un directorio, rm -f falla con set -eu y abortaría el empaquetado.
rm -f "$DESTINO/.gitmodules"
rm -rf "$DESTINO/apkc-emulador"

cp "$WASM" "$DESTINO/apkc.wasm"
# El navegador baja la versión comprimida (3,4 MB frente a 12,8 MB) y la descomprime con
# DecompressionStream; el sin comprimir se conserva como respaldo (§173).
gzip -9 -c "$WASM" > "$DESTINO/apkc.wasm.gz"
# Manifiesto de integridad: el worker descarta copias truncadas del wasm comparando tamaño y hash.
python3 - "$WASM" "$DESTINO/apkc.wasm.gz" "$DESTINO/wasm-manifest.json" <<'PY'
from pathlib import Path
import hashlib
import json
import sys

binario = Path(sys.argv[1]).read_bytes()
comprimido = Path(sys.argv[2]).read_bytes()
Path(sys.argv[3]).write_text(json.dumps({
    "bytes": len(binario),
    "sha256": hashlib.sha256(binario).hexdigest(),
    "gzBytes": len(comprimido),
    "gzSha256": hashlib.sha256(comprimido).hexdigest(),
}))
PY
# La versión de Go y la suma de wasm_exec.js se fijan en un solo sitio; el cp lee de ahí.
. "$AQUI/tools/wasm-exec-hash.sh"
cp "$WASM_EXEC_ORIGEN" "$DESTINO/wasm_exec.js"
[ -n "$CNAME" ] && printf '%s\n' "$CNAME" > "$DESTINO/CNAME"

# F2: el índice y los artefactos de dependencias no se versionan (regla 2) y `git archive HEAD` no
# los ve. Si se da DEPS_DIR con un paquete generado fuera del repo, se copia al árbol ya empaquetado
# ANTES de calcular VERSION: así entran en la huella del service worker y un cambio en ellos cambia
# la versión. Sin DEPS_DIR el sitio se construye idéntico salvo la CSP apkc-sw añadida más abajo.
# No se toca git: el destino está ignorado y nada de esto se añade al índice del repo.
if [ -n "${DEPS_DIR:-}" ]; then
  [ -f "$DEPS_DIR/deps-index.json" ] ||
    { printf 'no existe %s/deps-index.json\n' "$DEPS_DIR" >&2; exit 1; }
  [ -d "$DEPS_DIR/deps" ] ||
    { printf 'no existe %s/deps\n' "$DEPS_DIR" >&2; exit 1; }
  # Validar el índice contra los ficheros que describe: tamaño y SHA-256 de cada artefacto.
  python3 - "$DEPS_DIR" <<'PY'
from pathlib import Path
import hashlib
import json
import sys

raiz = Path(sys.argv[1])
indice = json.loads((raiz / "deps-index.json").read_text())
artefactos = indice.get("artefactos")
if not isinstance(artefactos, list) or not artefactos:
    raise SystemExit("deps-index.json sin artefactos")
base = (raiz / "deps").resolve()
for e in artefactos:
    fichero = e.get("fichero", "")
    if not isinstance(fichero, str) or not fichero.startswith("deps/"):
        raise SystemExit("fichero fuera de deps/: " + str(fichero))
    # Contención: ni components '..', ni absolutos, ni barras invertidas, y la resolución real
    # (incluidos symlinks) debe seguir dentro de deps/. Un índice hostil no puede sacar ficheros.
    if ".." in fichero.split("/") or "\\" in fichero or fichero.startswith("/"):
        raise SystemExit("fichero con traversal rechazado: " + fichero)
    destino = (raiz / fichero).resolve()
    if base not in destino.parents:
        raise SystemExit("fichero escapa de deps/: " + fichero)
    datos = destino.read_bytes()
    if len(datos) != e.get("bytes"):
        raise SystemExit("bytes no coinciden: " + fichero)
    if hashlib.sha256(datos).hexdigest() != e.get("sha256"):
        raise SystemExit("sha256 no coincide: " + fichero)
print("deps: " + str(len(artefactos)) + " artefacto(s) validado(s)")
PY
  cp "$DEPS_DIR/deps-index.json" "$DESTINO/deps-index.json"
  cp -R "$DEPS_DIR/deps" "$DESTINO/deps"
fi

# F3: el android.jar sintético de AOSP viaja en el propio origen, sin versionarse (regla 2): se
# copia del ANDROID_STUB_JAR que pase quien empaqueta —el artifact del workflow J4— y se describe
# en un JSON pequeño con su versión, tamaño y SHA-256. Va ANTES de calcular VERSION: el hash del SW
# cubre todo el destino salvo CNAME, así que un jar nuevo cambia la versión y un visitante con caché
# no se queda sin él. Sin la variable el sitio se empaqueta igual que antes y la página cae al SDK.
if [ -n "${ANDROID_STUB_JAR:-}" ]; then
  [ -f "$ANDROID_STUB_JAR" ] || { printf 'no existe el android stub jar: %s\n' "$ANDROID_STUB_JAR" >&2; exit 1; }
  cp "$ANDROID_STUB_JAR" "$DESTINO/android-stub-34.jar"
  python3 - "$DESTINO/android-stub-34.jar" "$DESTINO/android-stub.json" <<'PY'
from pathlib import Path
import hashlib
import json
import sys

binario = Path(sys.argv[1]).read_bytes()
Path(sys.argv[2]).write_text(json.dumps({
    "version": 34,
    "bytes": len(binario),
    "sha256": hashlib.sha256(binario).hexdigest(),
    "tag": "android-14.0.0_r75",
    "origen": "AOSP",
}))
PY
fi

# Los hosts sin cabeceras (Surge) necesitan el service worker: se registra con una versión en la
# URL para que un cambio de binario no se quede servido por la caché del navegador. El hash se
# inyecta en el SW, index.html y offline.js; en el árbol de desarrollo el marcador es literal y
# válido como versión.
VERSION=$(python3 - "$DESTINO" <<'PY'
from pathlib import Path
import hashlib
import sys

raiz = Path(sys.argv[1])
h = hashlib.sha256()
# Todos los ficheros que viajan, no solo el wasm y el SW: si cambia cualquier módulo, la versión
# del service worker cambia y un visitante con caché no se queda con código viejo (§163).
for fichero in sorted(p for p in raiz.rglob("*") if p.is_file() and p.name != "CNAME"):
    h.update(str(fichero.relative_to(raiz)).encode("utf-8"))
    h.update(b"\0")
    h.update(fichero.read_bytes())
    h.update(b"\0")
print(h.hexdigest()[:12])
PY
)
python3 - "$DESTINO" "$VERSION" <<'PY'
from pathlib import Path
import base64
import hashlib
import re
import sys

raiz, version = Path(sys.argv[1]), sys.argv[2]
for nombre in ("index.html", "sw.js", "offline.js"):
    fichero = raiz / nombre
    if fichero.exists():
        fichero.write_text(fichero.read_text().replace("__APKC_VERSION__", version))

# CSP sin 'unsafe-inline': los tres <script> inline de index.html se autorizan por hash SHA-256.
# Los módulos con src van por 'self'; el worker, el wasm y OPFS por sus esquemas de origen.
html = (raiz / "index.html").read_text()
hashes = []
for cuerpo in re.findall(r"<script(?![^>]*\bsrc=)[^>]*>(.*?)</script>", html, re.S):
    hashes.append("'sha256-" + base64.b64encode(
        hashlib.sha256(cuerpo.encode("utf-8")).digest()).decode("ascii") + "'")
csp = ("default-src 'self'; script-src 'self' 'wasm-unsafe-eval' " + " ".join(hashes) +
       # `index.html` crea también la política `apkc-sw` para registrar el service worker desde el
       # <head> (commit 47fc826 del submódulo): sin nombrarla aquí, Chromium bloquea el register y
       # la página nunca consigue crossOriginIsolated en un host sin cabeceras (prueba-sitio.js).
       "; require-trusted-types-for 'script'; trusted-types apkc-studio apkc-sw; " +
       "worker-src 'self'; connect-src 'self'; img-src 'self' data:; " +
       "style-src 'self'; font-src 'self' data:; object-src 'none'; " +
       "form-action 'none'; base-uri 'none'; frame-ancestors 'none'")
sw = raiz / "sw.js"
sw.write_text(sw.read_text().replace("__APKC_CSP__", csp))
# B1: mientras el SW no controla, la primera navegación no tiene cabeceras. Un <meta> con la
# misma política (menos frame-ancestors, que no es válido en meta) cubre esa ventana.
meta_csp = csp.replace("; frame-ancestors 'none'", "")
html = html.replace("<head>", '<head>\n<meta http-equiv="Content-Security-Policy" content="' + meta_csp + '">', 1)
(raiz / "index.html").write_text(html)
print("csp: " + csp)
PY

# las cabeceras son la mitad del producto: sin COOP/COEP no hay OPFS ni worker
grep -q 'Cross-Origin-Opener-Policy: same-origin' "$DESTINO/_headers" ||
  { printf '_headers no declara COOP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Embedder-Policy: require-corp' "$DESTINO/_headers" ||
  { printf '_headers no declara COEP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Opener-Policy' "$DESTINO/sw.js" ||
  { printf 'el service worker no pone COOP\n' >&2; exit 1; }
grep -q 'Cross-Origin-Embedder-Policy' "$DESTINO/sw.js" ||
  { printf 'el service worker no pone COEP\n' >&2; exit 1; }
[ -f "$DESTINO/index.html" ] || { printf 'el sitio no tiene index.html\n' >&2; exit 1; }
[ -f "$DESTINO/wasm_exec.js" ] || { printf 'falta wasm_exec.js\n' >&2; exit 1; }
# La CSP tiene que nombrar la política que crea studio/tt.js. Con require-trusted-types-for y sin
# trusted-types, el navegador solo admite la política por defecto y la página, que escribe con
# createPolicy("apkc-studio"), se queda sin poder tocar el DOM. Las dos mitades van juntas.
grep -q "require-trusted-types-for 'script'" "$DESTINO/index.html" ||
  { printf 'la CSP del <meta> no exige Trusted Types\n' >&2; exit 1; }
grep -q 'trusted-types apkc-studio' "$DESTINO/index.html" ||
  { printf 'la CSP del <meta> no nombra la política trusted-types apkc-studio\n' >&2; exit 1; }
grep -q 'trusted-types apkc-studio' "$DESTINO/sw.js" ||
  { printf 'la CSP del service worker no nombra la política trusted-types apkc-studio\n' >&2; exit 1; }
if grep -R -q -e '__APKC_VERSION__' -e '__APKC_CSP__' "$DESTINO" 2>/dev/null; then
  printf 'quedó un marcador de versión o de CSP sin sustituir\n' >&2
  exit 1
fi

python3 - "$DESTINO" "${DESTINO}.zip" <<'PY'
import os, sys, zipfile
raiz, salida = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(salida, "w", zipfile.ZIP_DEFLATED) as z:
    for base, _, ficheros in os.walk(raiz):
        for f in sorted(ficheros):
            ruta = os.path.join(base, f)
            z.write(ruta, os.path.relpath(ruta, raiz))
PY

printf 'sitio: %s (%s ficheros, version %s)\n' "$DESTINO" "$(find "$DESTINO" -type f | wc -l | tr -d ' ')" "$VERSION"
printf 'zip:   %s (%s bytes)\n' "${DESTINO}.zip" "$(wc -c <"${DESTINO}.zip" | tr -d ' ')"
