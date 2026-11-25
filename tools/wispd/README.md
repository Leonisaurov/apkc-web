# wispd

Relay WISP v1.2 propio, en Go, para la pestaña de apkc-studio. Vive en su propio
módulo (`go.mod`, Go 1.24) para que `github.com/gorilla/websocket` **no entre**
en el wasm de `apkc/`. Es la única dependencia externa; su licencia BSD-2-Clause
está en `LICENSE-gorilla.txt`.

## Arranque

```sh
cd tools/wispd
/home/leonisauro/sdk/go1.24.0/bin/go build -o wispd .
./wispd -addr :443 -cert /ruta/cert.pem -key /ruta/key.pem
```

Sin `-cert`/`-key` el servidor habla `ws://` (para pruebas locales). La URL del
WebSocket termina en `/` (por ejemplo `wss://wisp.dolci-vita.link/wisp/`), y
solo se atienden rutas que terminen en barra. `-origenes` solo **añade** orígenes;
el de producción ya está incluido.

El límite de memoria se fija dentro del proceso con `debug.SetMemoryLimit(512<<20)`
en `main.go:14-18`. Es el único mecanismo: no hace falta exportar `GOMEMLIMIT`.
`TestLimiteMemoriaActivo` comprueba que está activo.

## Banderas

| Bandera | Por defecto | Qué hace |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8800` | dirección de escucha |
| `-origenes` | vacío | orígenes adicionales separados por comas |
| `-cert` / `-key` | vacíos | TLS; si faltan, sirve `ws://` |
| `-credito` | `64` | paquetes del `CONTINUE` inicial (`streamID=0`) |
| `-maxstreams` | `8` | flujos máximos por conexión |
| `-dev` | falso | añade `http://127.0.0.1:8801`, `:8787` y `:8791` |

La allowlist de `Origin` contiene **exactamente** `https://apkc-studio.surge.sh`
en producción. El dominio del servidor (`wisp.dolci-vita.link`) no es un origen
del navegador y no se incluye. Un `Origin` vacío, `null` o ajeno recibe 403.
Es falsificable con `curl`, así que **no es autenticación**.

La allowlist de **destinos** (`destinos.go`) es la que impide el proxy abierto:
solo `repo1.maven.org:443` y `repo.maven.apache.org:443` por TCP, resolvidos y
comprobados antes de dialear: las IP públicas resueltas pasan y las reservadas
se rechazan. Se deniegan explícitamente literales IP, loopback
(`127/8`, `::1`), privadas (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`),
link-local (`169.254/16`, `fe80::/10`) y cualquier otro puerto. Los CONNECT
denegados reciben `CLOSE 0x41` (datos inválidos/reservados) o `0x48` (destino
bloqueado). No hay bandera que permita destinos adicionales.

## Cotas y crédito

Trama máxima de lectura 1 MiB + 5 bytes; cola de 64 paquetes de 16 KiB por flujo
(1 MiB) y 1024 paquetes globales (16 MiB), con contrapresión en el socket;
8 flujos por conexión y 16 conexiones (el exceso recibe `CLOSE 0x49`);
dial de 5 s que termina en `CLOSE 0x43` (o `0x42`/`0x44` según el error);
inactividad de 120 s con ping de 30 s. TCP y UDP (datagramas de hasta 8 KiB)
están soportados.

El `CONTINUE` inicial (`streamID=0`, `-credito`) da crédito al cliente para
enviar DATA; el relay lo renueva antes de que se agote, siempre con valor >0,
porque `apkc/wisp.go:894-916` bloquea el `Write` del cliente en crédito 0. En la
otra dirección, un `CONTINUE` del cliente limita los DATA que el relay le envía;
hasta que llega el primero no hay tope, por compatibilidad con los clientes
Wisp v1 de este repo, que no lo envían. En UDP no hay contabilidad de crédito.

## Motivos de cierre

Tabla verificada el 2026-11-25 en la spec v1.2:
https://github.com/MercuryWorkshop/wisp-protocol/blob/v1/protocol.md
(la propia página la identifica como «Version 1.2» y la licencia es CC-BY-4.0 de
ading2210). Se usa `0x01` sin especificar, `0x02` voluntario, `0x03` error de
red, `0x41` información inválida/dirección reservada, `0x42` host inalcanzable,
`0x43` tiempo de espera, `0x44` conexión rechazada, `0x47` transferencia TCP
agotada, `0x48` destino bloqueado por el proxy y `0x49` conexión limitada.

## Pruebas

```sh
GOTMPDIR=$HOME/.cache/apkc-web/wispd/gotmp go test ./...
go vet ./...
gofmt -l .
```
