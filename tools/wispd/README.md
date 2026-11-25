# wispd

Relay WISP v1.2 propio, en Go, para la pestaña de apkc-studio. Vive en su propio
módulo (`go.mod`, Go 1.24) para que `github.com/gorilla/websocket` **no entre**
en el wasm de `apkc/`. Es la única dependencia externa; su licencia BSD-2-Clause
está en `LICENSE-gorilla.txt`.

## Arranque

```sh
cd tools/wispd
GOMEMLIMIT=512MiB /home/leonisauro/sdk/go1.24.0/bin/go build -o wispd .
GOMEMLIMIT=512MiB ./wispd -addr :443 -cert /ruta/cert.pem -key /ruta/key.pem -origenes https://apkc-studio.surge.sh
```

Sin `-cert`/`-key` el servidor habla `ws://` (para pruebas locales). La URL del
WebSocket termina en `/` (por ejemplo `wss://wisp.dolci-vita.link/wisp/`), y
solo se atienden rutas que terminen en barra.

## Banderas

| Bandera | Por defecto | Qué hace |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8800` | dirección de escucha |
| `-origenes` | vacío | orígenes adicionales separados por comas |
| `-cert` / `-key` | vacíos | TLS; si faltan, sirve `ws://` |
| `-credito` | `64` | paquetes del `CONTINUE` inicial (`streamID=0`) |
| `-maxstreams` | `8` | flujos máximos por conexión |
| `-dev` | falso | añade `http://127.0.0.1:8801`, `:8787` y `:8791` |

La allowlist contiene **exactamente** `https://apkc-studio.surge.sh` en
producción. El dominio del servidor (`wisp.dolci-vita.link`) no es un origen del
navegador y no se incluye. Un `Origin` vacío, `null` o ajeno recibe 403.

## Cotas

Trama máxima de lectura 1 MiB + 5 bytes; cola de 64 paquetes de 16 KiB por flujo
(1 MiB) y 1024 paquetes globales (16 MiB), con contrapresión en el socket;
8 flujos por conexión y 16 conexiones (el exceso recibe `CLOSE 0x48` o `0x49`);
dial de 5 s que termina en `CLOSE 0x43`; inactividad de 120 s con ping de 30 s.
TCP y UDP (datagramas de hasta 8 KiB) están soportados. En la VM usa
`GOMEMLIMIT=512MiB`.

## Pruebas

```sh
GOTMPDIR=$HOME/.cache/apkc-web/wispd/gotmp go test ./...
go vet ./...
gofmt -l .
```
