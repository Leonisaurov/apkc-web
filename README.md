# apkc-web

La ambición de **compilar una app Android desde el navegador, del lado del cliente, con apkc**.
Este repo es documentación y un experimento que ya funciona; **no es código que cambie
[`../apkc`](../apkc)**. Nada de aquí debe tocar apkc sin que antes un spike diga que el camino
existe, y esa es la razón de separarlo: lo que se mide aquí no puede interferir con la herramienta
que funciona hoy.

## Estado, en una línea

La parte de apkc que **no lanza programas** (escritor de zip con alineado, firma v1+v2, lectura de
keystores PKCS#12 y JKS, generación de proyecto) **ya corre en un hospedador con forma de
navegador**: 88/88 tests y `apkc create` completo, medido el 2026-10-26 ([docs/evidencia.md](docs/evidencia.md)).
Lo que falta no es un flag: son **`aapt2`, `javac` y `d8`**, tres programas externos que `js/wasm` no
puede arrancar porque no tiene `fork/exec`.

## Qué hay aquí

| Fichero | Qué es |
| --- | --- |
| [docs/evidencia.md](docs/evidencia.md) | lo medido, con el comando que lo produce y la fecha |
| [docs/bloqueos.md](docs/bloqueos.md) | las cuatro herramientas externas, qué existe hoy y qué costaría sustituir cada una |
| [docs/plan.md](docs/plan.md) | los spikes, en orden, con el criterio que abandona cada uno |
| [shim/navegador-sim.js](shim/navegador-sim.js) | el hospedador de forma-navegador (un `fs` en memoria con la API de Node) que hace correr `apkc.wasm` sin Node |

## Reglas del repo

- **Sin binarios versionados**: `*.wasm` se ignora; se construyen (`GOOS=js GOARCH=wasm go build`)
  y se apuntan con `sh -c`. Mismo criterio que en apkc.
- **Un claim, una medición.** Una afirmación sobre coste, compatibilidad o formato que no tenga el
  comando que la reprodujo no entra en `docs/evidencia.md`; va a `docs/plan.md` como hipótesis.
- **La prioridad sigue en apkc.** Mientras la herramienta de línea de comandos y su página de
  fricción ([`~/Develop/Web/apkc-studio`](../Web/apkc-studio), que se queda donde está porque mide
  la herramienta real y no esta ambición) no hagan lo que deben, este repo está congelado.

## Empezar por aquí

```sh
sh -c 'cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$PREFIX/tmp/apkc-wasm.wasm" .'
sh -c 'cd shim && node navegador-sim.js "$PREFIX/tmp/apkc-wasm.wasm" create /miapp'
```

El segundo comando imprime el árbol de 17 nodos que apkc escribió **en memoria**, dentro de un
hospedador al que no le diste el `fs` de Node. Eso es la base real de todo lo demás.
