# apkc-web

La ambición de **compilar una app Android desde el navegador, del lado del cliente, con apkc**.
Este repo es documentación y un experimento que ya funciona; **no es código que cambie
[`../apkc`](../apkc)**. Nada de aquí debe tocar apkc sin que antes un spike diga que el camino
existe, y esa es la razón de separarlo: lo que se mide aquí no puede interferir con la herramienta
que funciona hoy.

## Estado, en una línea

El objetivo está cumplido: **https://apkc-studio.surge.sh/** abre en el navegador, crea un proyecto,
compila los recursos y el Java en Go, dexа, empaqueta, firma y verifica **dentro de la pestaña**, sin
servidor, sin SDK en la máquina del visitante, sin red y sin un solo proceso. El sitio se construye
con `sh tools/entorno.sh sitio` y se publica a mano con `cd sitio && surge . publish`. Medido sobre
el host real: 9/9 comprobaciones, apk verificada por `apksigner` y cero peticiones fuera del origen
([docs/evidencia.md](docs/evidencia.md) §155).

Lo que lo hizo posible es que **`aapt2`, `javac` y `d8` dejaron de ser procesos**: hay un enlazador
de recursos, un emisor de `.class` y un dexer escritos en Go dentro del fork [`apkc/`](apkc/), con
el subconjunto medido y lo no medido rechazado por nombre. El muro original (`js/wasm` no tiene
`fork/exec`) y la decisión de 2026-10-27 (sin conexión y sin runtimes de terceros) siguen escritos
en `docs/plan.md`. El dex del proyecto del studio son 2844 bytes y 217 objetos (§13); hoy el binario
es 12,78 MB y se baja como 3,37 MB comprimidos (§155 y §173).

El oráculo final no es el propio editor: el apk que sale de chromium se instala y arranca en un
teléfono real (§55 y §136), el gate completo pasa también en un runner Linux (§152) y en este
escritorio (§154), y `apksigner`, `zipalign`, `aapt2 dump`, `javap`, `d8` y ART verifican las salidas.

## Repositorios

- [`apkc-web`](https://github.com/Leonisaurov/apkc-web): este repo. El fork de apkc, el banco, la
  documentación medida, el gate y el sitio publicado.
- [`apkc-studio`](https://github.com/Leonisaurov/apkc-studio): el editor web, montado aquí como
  **submódulo** en [`studio/`](studio/). Sus cambios se commitean allí y el puntero se actualiza en
  este repo.

```sh
git clone --recurse-submodules https://github.com/Leonisaurov/apkc-web.git
```

## Qué hay aquí

| Fichero | Qué es |
| --- | --- |
| [LICENSE](LICENSE) | MIT del código propio |
| [NOTICE.md](NOTICE.md) | qué es de terceros: el fork `apkc`, Go/`wasm_exec.js`, el SDK de Android y el navegador de las pruebas |
| [AGENTS.md](AGENTS.md) | reglas del repo, entorno de Termux y el gate que hay que pasar antes de dar nada por hecho |
| [USO.md](USO.md) | uso concreto: construir, arrancar, abrir en Brave Nightly, editar, compilar, firmar y pasar el gate |
| [docs/plan.md](docs/plan.md) | el plan detallado: fases, tareas con su criterio de aceptación, y lo que abandona cada una |
| [docs/evidencia.md](docs/evidencia.md) | lo medido, con el comando que lo produce y la fecha |
| [docs/situacion.md](docs/situacion.md) | el recap de lo medido, bullet por bullet: dice qué dejó de estar abierto, pero el original es el `§` de evidencia que cada bullet cita |
| [docs/bloqueos.md](docs/bloqueos.md) | las cuatro herramientas externas, qué existe hoy y qué costaría sustituir cada una |
| [shim/navegador-sim.js](shim/navegador-sim.js) | el hospedador de forma-navegador (un `fs` en memoria con la API de Node) que hace correr `apkc.wasm` sin Node |
| [studio/](studio/) | apkc-studio: genera proyectos de apkc, traduce su salida real y mide dónde se atasca quien lo usa |

## Reglas del repo

- **Sin binarios versionados**: `*.wasm` se ignora; se construyen (`GOOS=js GOARCH=wasm go build`)
  y se apuntan con `sh -c`. Mismo criterio que en apkc.
- **Un claim, una medición.** Una afirmación sobre coste, compatibilidad o formato que no tenga el
  comando que la reprodujo no entra en `docs/evidencia.md`; va a `docs/plan.md` como hipótesis.
- **Todo lo relacionado con la web vive aquí.** La página de fricción viajó de `~/Develop/Web` a
  [`studio/`](studio/) para eso: un solo sitio donde versionar, comprobar y romper lo del navegador.
- **La prioridad sigue en apkc.** Mientras la herramienta de línea de comandos y su página no hagan
  lo que deben, las fases 1 y siguientes de [docs/plan.md](docs/plan.md) no se empiezan.

## Empezar por aquí

La guía concreta, con los comandos en orden, está en [USO.md](USO.md).

```sh
sh -c 'cd apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd shim && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" . && node navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp'
sh -c 'cd studio && APKC=<binario-de-apkc> sh e2e.sh'      # genera, compila de verdad, firma y conduce chromium
```

El segundo comando imprime el árbol de 17 nodos que apkc escribió **en memoria**, dentro de un
hospedador al que no le diste el `fs` de Node. Eso es la base real de todo lo demás.
