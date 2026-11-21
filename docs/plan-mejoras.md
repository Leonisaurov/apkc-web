# Plan de mejoras posteriores a la publicación

Orden acordado con el usuario el **2026-11-21**: **P3 → P2 → P1**. La publicación en
`https://apkc-studio.surge.sh/` ya está cerrada (§155) y el gate rojo que dejó el commit `464074a`
se arregló en `4e73ee9` (la prueba de DOM esperaba el aviso de OPFS de antes del service worker).

Este fichero es un **plan**, no un registro: aquí no entran hallazgos. Cada ítem se cierra con su
sonda, su `§` en `docs/evidencia.md` (con el comando que lo reproduce), su fila del inventario de
la Fase 5 actualizada cuando cambia lo que el subconjunto acepta, y su commit. Ninguna casilla se
marca «hecha» por analogía ni sin evidencia propia sobre el árbol final (regla 10 de `AGENTS.md`).

## Bloque A — P3: cerrar cuatro fronteras del subconjunto

| # | Íteme | Qué se mide | Criterio de cierre |
| --- | --- | --- | --- |
| A1 | `++`/`--` **como valor** | prefijo y postfijo, en local y en campo; con `javap -c -p` de javac 21 como oráculo y el dex byte a byte; ejecutar en ART con `banco/` | fila 5.1 actualizada, § nuevo, test en `apkc/` |
| A2 | `>>=`/`<<=`/`&=` (y `\|=`/`^=` si el tokenizador las arma) | primero medir qué escribe javac para cada una; después emitir y comparar `Code` y dex | fila 5.1 actualizada, § nuevo, test en `apkc/` |
| A3 | `protectionLevel` **compuesto** (`signature\|privileged`) en `<permission>` | AXML byte a byte contra `aapt2 link`, con la representación de la bandera compuesta | fila 5.19 actualizada, § nuevo |
| A4 | `nodpi`/`tvdpi` y `sw600dp` | tabla de recursos byte a byte contra `aapt2 link --no-auto-version` para cada calificador | fila 5.17 actualizada, § nuevo |

**Estado (2026-11-21): A1 cerrada en §156** (once formas, dex byte a byte y JVM con `-Xverify:all`),
**A2 cerrada en §157** (`<<=`, `>>=`, `>>>=`, `&=`, `|=`, `^=`; mismos oráculos), **A3 cerrada en §158**
(`protectionLevel` compuesto, AXML byte a byte contra aapt2) y **A4 cerrada en §159** (`nodpi`,
`tvdpi` y `sw600dp`, arsc byte a byte). ART queda pendiente de dispositivo en A1 y A2 y está escrito
como tal. **Bloque A completo.**

Notas de ejecución:

- Estos cuatro viven en el **fork `apkc/` de este repo**: no se toca `../apkc`. Cada pieza entra con
  su commit, su test Go y su gate (`tools/entorno.sh test`, `wasm` y `gate`).
- Si una sonda muestra que el caso exige una estructura no observable (como pasó con el `on-demand`
  de handlers), se escribe la frontera y se deja rechazada por nombre; no se “acepta de paso”.
- `layout-v1` (la copia que aapt2 añade por defecto) queda fuera de A4 salvo que la sonda diga lo
  contrario: es una diferencia de aapt2, no un calificador que el proyecto escriba.

## Bloque B — P2: robustez del sitio y oráculos

| # | Íteme | Criterio de cierre |
| --- | --- | --- |
| B1 | **`zipcheck.py`**: el usuario dice que `apkc-harness` ya está publicado; localizar el repo, tomar el script a un directorio temporal y dejar que `tools/entorno.sh doctor`/`gate` lo encuentren sin versionarlo | el gate deja de decir «se omite ese oráculo» |
| B2 | **`bundletool`** en local, si no está, para el oráculo del `.aab` | el gate valida el bundle sin avisos de omisión, o queda escrito por qué no se puede |
| B3 | **segundo navegador** (Firefox; Safari si hay máquina): SW + `crossOriginIsolated` + OPFS + compilación | `prueba-sitio.js` pasa allí, o se escribe la frontera exacta con el error |
| B4 | **destello de la primera carga**: velo hasta que la página esté aislada | la recarga automática deja de verse como un parpadeo; `prueba-sitio.js` sigue en verde |

**Estado (2026-11-21): Bloque B completo.** B1 y B2 en §160 (el gate ya no omite ni el zip ni el
bundle), B3 en §161 (Firefox 4/4; Safari sin equipo, escrito como frontera) y B4 en §162.

## Bloque C — P1: cierre y pulido

| # | Íteme | Criterio de cierre |
| --- | --- | --- |
| C1 | **`tools/entorno.sh publicar`**: construir `sitio/`, `surge . publish` y verificar con `SITIO_URL` en un solo comando | una sola orden deja el sitio publicado y verificado; imprime URL y hash |
| C2 | **hash de versión del SW sobre todos los ficheros** del sitio, no solo wasm + SW + `index.html` | un cambio en cualquier módulo cambia la versión del SW |
| C3 | **documentación obsoleta**: `docs/situacion.md` dice que la anónima en `static` se rechaza (§99 la cerró); `docs/bloqueos.md` da el desugaring de lambdas como fuera de plan (§110-§116 lo hizo); repasar `README.md` y `docs/plan.md` | ninguna afirmación cerrada sigue escrita como pendiente |
| C4 | **cierre**: panel de progreso si el usuario lo tiene abierto (regla 9), commit y push de este repo y del submódulo | `git status` limpio, `origin/web` alineado y gate en verde |

**Estado (2026-11-21): plan completo.** C1 y C2 en §163 (publicado y verificado en una orden;
versión del SW sobre todo el paquete), C3 corregido con sus `§` y C4 cerrado con el gate local en
verde, el gate de GitHub en verde (run `44950578574`, con `zipcheck` y `bundletool` activos) y los
dos repositorios pusheados.

## Fuera de este plan

Kotlin, multidex, `.aab` con splits por densidad/idioma e interoperabilidad con Gradle siguen
fuera con el motivo escrito en `docs/plan.md`. Meterlos cambiaría la ambición, no la mejoraría.
