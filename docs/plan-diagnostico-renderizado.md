# Diagnóstico del renderizado vacío

Fecha: 2026-11-26

Estado: hipótesis corregida por auditoría Modo B; no es medición nueva. Documento de
trabajo: recoge hipótesis y frentes falsables, sin `§` nuevo en `docs/evidencia.md`.
No afirma que ninguna de las cuatro apps funcione ni que muestre su pantalla real.

## 1. Causas comunes (`archivo:línea` verificados por el crítico)

- `setContentView` solo guarda el id (`previewinterprete.go:6181-6187`): no infla el AXML.
- La medición sale del AXML y un contenedor sin hijos medidos colapsa a alto 0
  (`preview.go:3428-3444`, `3450`, `3586`): `medir` decide el tamaño propio de la hoja,
  `medirLineal` reparte pesos y `colocar` escribe el rect con lo que haya medido.
- `anotaTextoDinamico` exige coincidencia exacta de `IDRef` (`preview.go:1448,1453`),
  pero `captura` solo guarda `IDRef` si `obj.id != 0`
  (`previewinterprete.go:1469,1473-1475`). Un texto con id fuera del árbol queda en
  `ejecucion.capturas` y no se pinta.
- No hay resolvedor de `<include>` ni de `FragmentTransaction`: `grep` sobre `preview.go`
  solo da prosa en `:795`; los nodos `include` quedan 32×24 y los
  `FragmentContainerView` 360×640 con 0 hijos.

## 2. Correcciones de la auditoría (para no repetir el error)

- El mecanismo del fix de ApkShare **no** está en `preview.go:1592,1630,1652`.
  Está en `preview.go:1444-1462` + `previewinterprete.go:1469-1485`.
  Las líneas correctas son `1591` (`n.orientacion = "vertical"`), `1630`
  (`return false`) y `1650`/`1653` (`return v.texto`).
- En Termux el raíz **sí mide 360×640**; el alto 0 nace de
  `id/activity_termux_root_relative_layout` (`alto:"0dp"`, `peso:1`) y baja a
  `id/terminal_view` (354×0). No es el raíz quien colapsa.

## 3. Diagnóstico por app (cifras medidas)

- Gestor (`~/.cache/apkc-web/integracion-final/gestor-tres-pulsos.json`): 32 nodos sobre
  `res/R5.xml`; el menú tiene rect 0×0; en tres pulsos aparecen las filas `fotos` y
  `leeme.txt`.
- Termux (`~/.cache/apkc-web/reconcilia-final/termux.json`): 7418 pasos, 13 nodos,
  raíz 360×640 y `id/terminal_view` 354×0.
- NewPipe (`~/.cache/apkc-web/integracion-final/newpipe.json`): 3476 pasos, 6 nodos,
  dos `include` 32×24 y dos `FragmentContainerView` 360×640 con 0 hijos.
- ApkShare (`~/.cache/apkc-web/apkshare-layout/apkshare-post.json`): 6 nodos, 0 avisos,
  `id/apps` con alto 24 y `id/label` con «Cargando paquetes. . .» (texto anotado, que no
  sustituye al placeholder).

## 4. Frentes falsables

1. `<include>` del AXML — **incierto** hasta mostrar el resolvedor; hoy `grep -n -E
   'include|FragmentTransaction' apkc/preview.go` solo da la prosa de `:795`. Criterio:
   los rects de `include` dejan de ser 32×24. Verificación:
   `python3 -c 'import json,os; d=json.load(open(os.environ["HOME"]+"/.cache/apkc-web/integracion-final/newpipe.json")); w=lambda n:[n]+[x for h in n.get("hijos",[]) for x in w(h)]; print([(n.get("id"),n["rect"]) for n in w(d["raiz"]) if n.get("tipo")=="include"])'`
2. Medición de contenedores con hijos que colapsan a 0 — criterio sobre rects: un nodo
   con `hijos` no vacío no puede tener `rect.alto == 0` si el raíz mide 360×640.
   Verificación:
   `python3 -c 'import json,os; d=json.load(open(os.environ["HOME"]+"/.cache/apkc-web/reconcilia-final/termux.json")); w=lambda n:[n]+[x for h in n.get("hijos",[]) for x in w(h)]; print([(n.get("id"),n["rect"],len(n.get("hijos",[]))) for n in w(d["raiz"]) if n["rect"]["alto"]==0 and n.get("hijos")])'`
3. Fragments desde el dex — **incierto** hasta mostrar el despacho de `FragmentTransaction`
   en el intérprete; los `FragmentContainerView` siguen con 0 hijos. Criterio: el nodo
   adquiere hijos medidos sin inventar la pantalla. Verificación:
   `python3 -c 'import json,os; d=json.load(open(os.environ["HOME"]+"/.cache/apkc-web/integracion-final/newpipe.json")); w=lambda n:[n]+[x for h in n.get("hijos",[]) for x in w(h)]; print([n.get("id") for n in w(d["raiz"]) if "FragmentContainerView" in n.get("tipo","") and len(n.get("hijos",[]))==0])'`
4. Visibilidad de la ausencia de adaptador en ApkShare — criterio: `avisos` no vacío o
   `hijos > 1` en `id/apps`, en lugar de un único `id/label` anotado. Verificación:
   `python3 -c 'import json,os; d=json.load(open(os.environ["HOME"]+"/.cache/apkc-web/apkshare-layout/apkshare-post.json")); print(len(d["ejecucion"]["avisos"]),[(n.get("id"),len(n.get("hijos",[]))) for n in [d["raiz"]] if n.get("id")=="id/apps"])'`

## 5. Referencia visual

La referencia visual del oráculo Droidloom está medida por la vía normal
(`droidloomctl launch`). El gestor **abre y pinta**: `Displayed
com.martinmimigames.simplefileexplorer/.MainActivity ... +99ms`, sin excepción en ese
arranque; a los ~7 s pasa a `TO_BACK` (`topResumedActivity=HomeActivity`,
`mDrawState=NO_SURFACE`), así que no queda en primer plano. Su captura no es verificable:
con `disable_secure_windows=1` el `screencap` del display 0 sigue negro y el setting se
restauró a `null`. El `NullPointerException` en `onCreate(SourceFile:6)` pertenecía a un
lanzamiento forzado a `--display 2`, no a esta vía. ApkShare **no abre** por la vía
normal: `VerifyError` del verificador de ART sobre
`android.support.v4.content.FileProvider.buildPath(java.io.File, java.lang.String[])` en
`base.apk!classes2.dex`, con `Process share.apps.util ... has died: fg TOP`; sin evidencia
de clase duplicada. Termux y NewPipe abren y pintan; sus capturas están en
`~/.cache/apkc-web/droidloom-oraculo/`. No se afirma parecido visual con la app real sin
comparación superponible.

| APK | Oráculo Droidloom | Vista previa del emulador |
| --- | --- | --- |
| Gestor | en verificación | 32 nodos, menú 0×0, filas `fotos` y `leeme.txt` en tres pulsos |
| Termux | abre y pinta | 13 nodos, raíz 360×640, `id/terminal_view` 354×0 |
| NewPipe | abre y pinta | 6 nodos, dos `include` 32×24, dos `FragmentContainerView` 0 hijos |
| ApkShare | en verificación | 6 nodos, `id/apps` alto 24, `id/label` anotado |

Sin afirmar parecido entre oráculo y emulador: no hay cotejo de capturas ejecutado.

## 6. No tocar

ROM, permisos, firmas, red, gate, pin del emulador ni APKs versionadas. Ninguno de estos
frentes autoriza a cambiarlos.
