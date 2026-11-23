# inventario-jar

Inventario reproducible de lo que el dexer de este fork aún no traduce de un jar real de
dependencias. No modifica el dexer: pasa cada `.class` por `apkc dex` en su propia invocación y
agrupa los rechazos por tipo de límite.

## Comandos

```sh
# mide `android-support-v4.jar` y escribe informe.md y casos.json aquí al lado
tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py

# comprueba el contador con un jar vacío y un fixture de un solo opcode, y restaura
tcr -c 2 -m 1536M -r 256M -- python3 banco/inventario-jar/inventario.py --falsificar
```

Opciones: `--jar` cambia el artefacto, `--apkc` usa un binario ya construido, `--sin-oraculo` no
arranca d8/dexdump, `--limite N` mide solo las N primeras clases y `--salida` mueve el informe.

## El binario cacheado se recompila cuando cambia `apkc/`

El binario de caché se llama `apkc-<huella del código>`: dos procesos con el mismo `apkc/` lo
comparten y dos con códigos distintos no se lo pisan. La construcción se serializa con un `flock`
por huella y el binario entra con un `rename` atómico; la `.huella` que lo acompaña es la del
código que lo produjo. Con `--apkc` o `APKC=...` se usa el binario externo tal cual, sin
reconstruir; `--apkc-dir` elige qué copia de `apkc/` se compila.

`--falsificar` escribe su detalle en `$HOME/.cache/apkc-web/inventario-jar/falsificacion.md` (o
bajo `$XDG_CACHE_HOME`), nunca en el fichero versionado: la salida lleva hashes de los fixtures y
la ruta del directorio de trabajo. El `falsificacion.md` del repo es un resumen estable sin datos
locales.

La sonda comprueba seis cosas. La reconstrucción se hace sobre una **copia** de `apkc/` en el
directorio de trabajo (nunca escribe en el árbol versionado) y exige `git status --porcelain apkc/`
vacío en cada paso. También exige que dos ejecuciones del mismo jar dejen `informe.md` y
`casos.json` byte a byte iguales, que al terminar no haya cambios en `banco/inventario-jar/` ni en
`apkc/`, y que dos procesos concurrentes sobre el mismo jar terminen con rc 0 y dejen los mismos
bytes. El sexto caso repite la carrera con dos copias distintas de `apkc/` y sin `--apkc`: cada
proceso construye `apkc-<huella>` en su caché aislada y el binario de la copia modificada lleva su
huella. Cada proceso usa un directorio privado `trabajo-<huella>-<aleatorio>`; el informe publica la
forma estable `$CACHE/trabajo-<huella>/d8`, donde `$CACHE` es el directorio de caché del inventario.

## Qué deja

- `informe.md`: tabla de límites con clases y métodos afectados, primer caso con su pc y opcode,
  propuesta de forma DEX y el dex de referencia que escribe `d8` para el mismo método.
- `casos.json`: todos los rechazos con clase, método, pc, opcode y motivo, para que cada agente
  ataque un límite sin volver a correr el jar.
- `falsificacion.md`: resumen estable de los seis casos; el detalle con hashes y rutas va a la
  caché (`$HOME/.cache/apkc-web/inventario-jar/falsificacion.md`).

Los temporales van a `$HOME/.cache/apkc-web/inventario-jar/`, nunca a `/tmp`: esa tmpfs tiene cuota
de usuario y ya ha roto builds de este repo.

## Límite de la medida

Cada clase se dexó sola, así que el conteo de clases es exacto. El de métodos es la primera que
falla dentro de cada clase, porque el ensamblado se detiene ahí: es una cota inferior. Si una clase
tiene dos límites distintos, el informe solo ve el primero.
